//go:build oracle

package testkit_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Ordinary cooperative SQLite writes exercise planner diagnostics without
// mutating internal metadata or constructing damaged database images.
func TestOracleGramsMatchExactFallbackReason(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "match-reason.db")
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if db != nil {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
	writeExec(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,title TEXT)")
	writeExec(t, db, "INSERT INTO docs VALUES(1,'数据库'),(2,'other'),(3,'third'),(4,'fourth'),(5,'fifth'),(6,'sixth'),(7,'seventh'),(8,'eighth')")
	writeExec(t, db, "CREATE INDEX docs_grams ON docs(title) USING grams13")
	for _, step := range []struct{ sql, reason string }{
		{"UPDATE docs SET title='数据库新行' WHERE id=2;", "external writer marked index dirty; REINDEX required"},
		{"CREATE TABLE unrelated(id INTEGER PRIMARY KEY);", "schema epoch changed; REINDEX required"},
	} {
		if err = db.Close(); err != nil {
			t.Fatal(err)
		}
		db = nil
		confCLI(t, path, step.sql, false)
		db, err = sqlite.Open(path, sqlite.Options{})
		if err != nil {
			t.Fatal(err)
		}
		collect := func(query string) ([]int64, sqlite.QueryPlan) {
			t.Helper()
			rows, err := db.Query(ctx, query)
			if err != nil {
				t.Fatal(err)
			}
			var ids []int64
			for rows.Next() {
				ids = append(ids, rows.Values()[0].(int64))
			}
			plan := rows.Plan()
			if err = errors.Join(rows.Err(), rows.Close()); err != nil {
				t.Fatal(err)
			}
			return ids, plan
		}
		got, plan := collect("SELECT id FROM docs WHERE grams13_match('docs_grams',title,'数据库') ORDER BY id")
		want, _ := collect("SELECT id FROM docs NOT INDEXED WHERE grams13_match('docs_grams',title,'数据库') ORDER BY id")
		if plan.Access != "SCAN" || plan.Reason != step.reason {
			t.Fatalf("after %q: expected SCAN reason %q; got %+v", step.sql, step.reason, plan)
		}
		if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(got, []int64{1, 2}) {
			t.Fatalf("after %q: indexed=%v forced-scan=%v", step.sql, got, want)
		}
		if err = db.Reindex(ctx, "docs_grams"); err != nil {
			t.Fatal(err)
		}
		_, plan = collect("SELECT id FROM docs WHERE grams13_match('docs_grams',title,'数据库') ORDER BY id")
		if plan.Access != "SEARCH" {
			t.Fatalf("rebuilt plan=%+v", plan)
		}
	}
}
