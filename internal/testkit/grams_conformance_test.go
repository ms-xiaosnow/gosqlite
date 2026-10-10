//go:build oracle

package testkit_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// This suite exercises only ordinary valid SQL, transaction rollback, normal
// connection close/reopen and cooperative official SQLite writes. It does not
// corrupt files, inject physical faults or interrupt a database process.
func assertGramAccess(t *testing.T, db *sqlite.DB, access string) {
	t.Helper()
	checkErr := db.CheckExtensionIndex(context.Background(), "docs_grams")
	if access == "SEARCH" && checkErr != nil {
		t.Fatal(checkErr)
	}
	if access == "SCAN" && !errors.Is(checkErr, sqlite.ErrReindexRequired) {
		t.Fatalf("dirty consistency check: %v", checkErr)
	}
	for _, needle := range []string{"库", "数据", "数据库"} {
		rows, err := db.Query(context.Background(), "SELECT id FROM docs WHERE title LIKE ? ORDER BY id", "%"+needle+"%")
		if err != nil {
			t.Fatal(err)
		}
		plan := rows.Plan()
		for rows.Next() {
		}
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		stats := rows.Stats()
		if err = rows.Close(); err != nil {
			t.Fatal(err)
		}
		if plan.Access != access {
			t.Fatalf("%q expected %s, got %+v", needle, access, plan)
		}
		if access == "SEARCH" {
			if !plan.Recheck || len(plan.Indexes) == 0 || stats.PostingKeysRead == 0 || stats.BaseRowFetches >= 48 {
				t.Fatalf("not a selective rechecked posting-index plan: %+v %+v", plan, stats)
			}
		} else if plan.Reason == "" {
			t.Fatal("fallback did not explain its reason")
		}
	}
	match, err := db.Query(context.Background(), "SELECT id FROM docs WHERE grams13_match('docs_grams',title,'数据库') ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	plan := match.Plan()
	var got []int64
	for match.Next() {
		got = append(got, match.Values()[0].(int64))
	}
	if err = errors.Join(match.Err(), match.Close()); err != nil {
		t.Fatal(err)
	}
	if plan.Access != access {
		t.Fatalf("configured MATCH expected %s got %+v", access, plan)
	}
	scan, err := db.Query(context.Background(), "SELECT id FROM docs NOT INDEXED WHERE grams13_match('docs_grams',title,'数据库') ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	var want []int64
	for scan.Next() {
		want = append(want, scan.Values()[0].(int64))
	}
	if err = errors.Join(scan.Err(), scan.Close()); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("MATCH mismatch got%v want%v", got, want)
	}

}

func TestOraclePersistentGramsInterop(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("page%d/shared%t", size, shared), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "grams.db")
				opts := sqlite.Options{PageSize: size, WALShared: shared, KeepWALOnClose: shared}
				db, err := sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if db != nil {
						if e := db.Close(); e != nil {
							t.Error(e)
						}
					}
				}()
				writeExec(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,title TEXT,body TEXT)")
				writeExec(t, db, "CREATE TABLE preserved(id INTEGER PRIMARY KEY,note TEXT)")
				writeExec(t, db, "INSERT INTO preserved VALUES(7,'unrelated table')")
				tx, err := db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				for i := int64(1); i <= 48; i++ {
					title := fmt.Sprintf("ordinary-%02d", i)
					body := "常见正文"
					switch i {
					case 3:
						title = "数据库🙂"
					case 9:
						title = "e\u0301 カナ 한글"
						body = "数据库搜索"
					case 21:
						title = "literal%_escape"
						body = "emoji👩‍💻"
					}
					writeTxExec(t, tx, "INSERT INTO docs VALUES(?,?,?)", i, title, body)
				}
				if err = tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				if err = db.CreateGramsIndex(ctx, "docs_grams", "docs", []string{"title", "body"}); err != nil {
					t.Fatal(err)
				}
				queries := []string{
					"SELECT id,title FROM docs WHERE title LIKE '%库%' ORDER BY id",
					"SELECT id,title FROM docs WHERE title LIKE '%数据%' ORDER BY id",
					"SELECT id,title FROM docs WHERE title LIKE '%数据库%' ORDER BY id",
					"SELECT id,title FROM docs WHERE title LIKE '%数据库%' OR body LIKE '%数据库%' ORDER BY id",
					"SELECT id,title FROM docs WHERE title LIKE '%e%' AND body LIKE '%常见%' ORDER BY id",
					"SELECT id,title FROM docs WHERE title LIKE '%!%%' ESCAPE '!' ORDER BY id",
				}
				for _, q := range queries {
					writeSame(t, db, path, q)
				}
				assertGramAccess(t, db, "SEARCH")
				writeIntegrity(t, path)
				entries, err := db.Schema(ctx)
				if err != nil {
					t.Fatal(err)
				}
				guard := ""
				for _, entry := range entries {
					if entry.Type == "trigger" && entry.TableName == "docs" {
						guard = entry.Name
					}
					if entry.Type == "table" || entry.Type == "index" {
						if strings.Contains(strings.ToLower(entry.SQL), "using grams13") {
							t.Fatalf("extension SQL leaked into SQLite schema: %s", entry.SQL)
						}
					}
				}
				if guard == "" {
					t.Fatal("no external-writer invalidation guard")
				}
				// Normal external writes invalidate the index within their own
				// SQLite transaction. Reads must remain correct before rebuild.
				if err = db.Close(); err != nil {
					t.Fatal(err)
				}
				db = nil
				confCLI(t, path, "BEGIN; UPDATE docs SET title='外部数据库' WHERE id=3; INSERT INTO docs VALUES(99,'新增数据库','external'); DELETE FROM docs WHERE id=9; COMMIT;", false)
				db, err = sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				for _, q := range queries {
					writeSame(t, db, path, q)
				}
				assertGramAccess(t, db, "SCAN")
				if _, err = db.Exec(ctx, "UPDATE docs SET body='must require rebuild' WHERE id=3"); err == nil {
					t.Fatal("native source write accepted dirty external index")
				}
				if err = db.Reindex(ctx, "docs_grams"); err != nil {
					t.Fatal(err)
				}
				assertGramAccess(t, db, "SEARCH")
				for _, q := range queries {
					writeSame(t, db, path, q)
				}
				// Ordinary rollback preserves both the source row and every
				// posting/frequency change produced inside this transaction.
				tx, err = db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				writeTxExec(t, tx, "UPDATE docs SET id=100,title='撤销数据库' WHERE id=3")
				writeTxExec(t, tx, "DELETE FROM docs WHERE id=99")
				if err = tx.Rollback(); err != nil {
					t.Fatal(err)
				}
				for _, q := range queries {
					writeSame(t, db, path, q)
				}
				writeExec(t, db, "UPDATE docs SET id=101,title='native数据库' WHERE id=3")
				writeExec(t, db, "DELETE FROM docs WHERE id=99")
				writeExec(t, db, "INSERT INTO docs VALUES(103,'native新增数据库','new')")
				for _, q := range queries {
					writeSame(t, db, path, q)
				}
				assertGramAccess(t, db, "SEARCH")
				if shared {
					if err = db.Checkpoint(ctx); err != nil {
						t.Fatal(err)
					}
				}
				writeIntegrity(t, path)
				writeSame(t, db, path, "SELECT * FROM preserved")
				if err = db.Close(); err != nil {
					t.Fatal(err)
				}
				db = nil
				// A legitimate external schema edit removes a guard. Native
				// reads must not trust stale ownership, and REINDEX repairs
				// only its exact generated guard definition.
				confCLI(t, path, "DROP TRIGGER \""+strings.ReplaceAll(guard, "\"", "\"\"")+"\";", false)
				db, err = sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				assertGramAccess(t, db, "SCAN")
				for _, q := range queries {
					writeSame(t, db, path, q)
				}
				if err = db.Reindex(ctx, "docs_grams"); err != nil {
					t.Fatal(err)
				}
				assertGramAccess(t, db, "SEARCH")
				for _, q := range queries {
					writeSame(t, db, path, q)
				}
				writeIntegrity(t, path)
			})
		}
	}
}

// VACUUM is a normal SQLite operation that may renumber implicit rowids without
// firing DML guards. The persisted schema epoch must invalidate old postings.
func TestOracleGramsVacuumEpoch(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vacuum.db")
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if db != nil {
			if e := db.Close(); e != nil {
				t.Error(e)
			}
		}
	}()
	writeExec(t, db, "CREATE TABLE docs(title TEXT)")
	writeExec(t, db, "INSERT INTO docs(rowid,title) VALUES(10,'plain'),(20,'数据库'),(40,'other'),(100,'last')")
	if err = db.CreateGramsIndex(ctx, "docs_grams", "docs", []string{"title"}); err != nil {
		t.Fatal(err)
	}
	if err = db.CheckExtensionIndex(ctx, "docs_grams"); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db = nil
	confCLI(t, path, "VACUUM;", false)
	db, err = sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	q := "SELECT rowid,title FROM docs WHERE title LIKE '%数据库%' ORDER BY rowid"
	writeSame(t, db, path, q)
	rows, err := db.Query(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	plan := rows.Plan()
	for rows.Next() {
	}
	err = errors.Join(rows.Err(), rows.Close())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Access != "SCAN" || !strings.Contains(plan.Reason, "schema") {
		t.Fatalf("VACUUM failed to invalidate: %+v", plan)
	}
	if err = db.CheckExtensionIndex(ctx, "docs_grams"); !errors.Is(err, sqlite.ErrReindexRequired) {
		t.Fatalf("check after VACUUM: %v", err)
	}
	if _, err = db.Exec(ctx, "UPDATE docs SET title='blocked' WHERE rowid=20"); !errors.Is(err, sqlite.ErrReindexRequired) {
		t.Fatalf("write after VACUUM: %v", err)
	}
	if err = db.Reindex(ctx, "docs_grams"); err != nil {
		t.Fatal(err)
	}
	if err = db.CheckExtensionIndex(ctx, "docs_grams"); err != nil {
		t.Fatal(err)
	}
	writeSame(t, db, path, q)
	writeIntegrity(t, path)
}
