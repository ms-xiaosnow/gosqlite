//go:build oracle

package testkit_test

import (
	"context"
	"errors"
	"github.com/ms-xiaosnow/gosqlite/internal/testoracle"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReviewPreparationOracleDefaultRuntimeAndInvalid(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "native.db")
	a, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	official := filepath.Join(t.TempDir(), "official.db")
	x, y := startCounterLiveOracle(t, official), startCounterLiveOracle(t, official)
	both := func(db *sqlite.DB, p *counterLiveOracle, q string) {
		t.Helper()
		if _, e := db.Exec(ctx, q); e != nil {
			t.Fatal(q, e)
		}
		p.run(q, false)
	}
	both(a, x, "CREATE TABLE t(id INTEGER PRIMARY KEY,v NOT NULL,true,false)")
	both(a, x, "INSERT INTO t VALUES(1,10,1,0),(2,20,0,1)")
	cases := []struct {
		q        string
		code     sqlite.Code
		category string
	}{
		{"INSERT INTO t DEFAULT VALUES", sqlite.CodeBusy, "database is locked"},
		{"INSERT INTO t(id,v) VALUES('bad',1)", sqlite.CodeBusy, "database is locked"},
		{"UPDATE t SET v=1/0", sqlite.CodeBusy, "database is locked"},
		{"UPDATE t SET v=CASE WHEN true THEN 7 ELSE false END WHERE rowid=1", sqlite.CodeBusy, "database is locked"},
		{"DELETE FROM t WHERE _rowid_=2 OR oid=1", sqlite.CodeBusy, "database is locked"},
		{"UPDATE t SET v=CASE WHEN 0 THEN missing ELSE v END", sqlite.CodeError, "no such column"},
		{"UPDATE t SET v=sum(v)", sqlite.CodeError, "misuse of aggregate"},
		{"INSERT INTO t(v) VALUES(total_changes(1))", sqlite.CodeError, "wrong number of arguments"},
	}
	for _, tc := range cases {
		both(a, x, "UPDATE t SET v=v")
		both(b, y, "BEGIN EXCLUSIVE")
		_, e := a.Exec(ctx, tc.q)
		var detail *sqlite.Error
		if !errors.As(e, &detail) || detail.Code != tc.code {
			t.Fatal(tc.q, e)
		}
		cachedOracleError(t, x, tc.q, tc.category)
		if rows, e := a.Query(ctx, "SELECT * FROM t"); e == nil {
			rows.Close()
			t.Fatal("cache authorized data read")
		} else if !errors.Is(e, sqlite.ErrBusy) {
			t.Fatal(e)
		}
		both(b, y, "ROLLBACK")
		native := confNative(t, a, connectionOracleSelect)
		want, e := confQuoted(x.run(connectionOracleSelect, false))
		if e != nil {
			t.Fatal(e)
		}
		labels := make([]any, len(native.columns))
		for i, v := range native.columns {
			labels[i] = v
		}
		got := append([][]any{labels}, native.rows...)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s native=%#v official=%#v", tc.q, got, want)
		}
	}
	confCompare(t, confNative(t, a, "SELECT id,v,true,false FROM t ORDER BY id"), confExpected(t, official, "SELECT id,v,true,false FROM t ORDER BY id"))
}

func TestReviewPreparationUnsupportedCatalogCannotWrite(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "native.db")
	a, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY,v)", "INSERT INTO t VALUES(1,10),(2,20)"} {
		if _, err = a.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	// Ordinary official SQL introduces a well-formed unsupported view; no raw writes.
	cmd := testoracle.ForTest(t, confOracle(t), "-batch", path)
	cmd.Stdin = strings.NewReader("CREATE VIEW view_t AS SELECT * FROM t;\n")
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(string(out), e)
	}
	b, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if _, err = b.Exec(ctx, "BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Exec(ctx, "UPDATE t SET v=99"); !errors.Is(err, sqlite.ErrBusy) {
		t.Fatal(err)
	}
	if _, err = b.Exec(ctx, "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Exec(ctx, "UPDATE t SET v=99"); !errors.Is(err, sqlite.ErrUnsupported) {
		t.Fatal("fresh snapshot skipped unsupported catalog", err)
	}
	got := confNative(t, a, "SELECT sum(v) FROM t")
	if !reflect.DeepEqual(got.rows, [][]any{{int64(30)}}) {
		t.Fatal("cache broadened write authority", got.rows)
	}
	if _, err = b.Exec(ctx, "BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Exec(ctx, "DELETE FROM missing"); !errors.Is(err, sqlite.ErrBusy) {
		t.Fatal("excluded catalog should decline cache", err)
	}
	if _, err = b.Exec(ctx, "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
}
