//go:build oracle

package testkit_test

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/similarity"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Only valid-data SQL and normal process completion are used. Official SQLite
// verifies legal storage and ordinary external invalidation; similarity scores
// are independently tested mathematically, not attributed to SQLite builtins.
func TestOracleSimilarityDirtyIndexFallback(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "similarity.db")
			openOptions := sqlite.Options{WALShared: shared, KeepWALOnClose: shared}
			db, err := sqlite.Open(path, openOptions)
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
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for i := int64(1); i <= 24; i++ {
				title := fmt.Sprintf("plain-%02d", i)
				body := "ordinary"
				if i == 1 {
					title = "数据库查询"
					body = "数据库"
				}
				if i == 2 {
					title = "数据检索"
					body = "数据库搜索"
				}
				writeTxExec(t, tx, "INSERT INTO docs VALUES(?,?,?)", i, title, body)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err = db.CreateGramsIndex(ctx, "docs_grams", "docs", []string{"title", "body"}); err != nil {
				t.Fatal(err)
			}
			options := sqlite.SimilarityOptions{MinGram: 2, MaxGram: 2, Metric: similarity.Dice, Threshold: 0.1, Limit: 10}
			compare := func(access string) {
				t.Helper()
				indexed, err := db.SimilaritySearch(ctx, "docs_grams", "title", "数据库", options)
				if err != nil {
					t.Fatal(err)
				}
				force := options
				force.ForceScan = true
				scanned, err := db.SimilaritySearch(ctx, "docs_grams", "title", "数据库", force)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(indexed.Hits, scanned.Hits) {
					t.Fatalf("candidate recall got%+v want%+v", indexed.Hits, scanned.Hits)
				}
				if indexed.Plan.Access != access || !indexed.Plan.Exact {
					t.Fatalf("plan: %+v", indexed.Plan)
				}
				if access == "SEARCH" && (indexed.Stats.PostingKeysRead == 0 || indexed.Stats.BaseRowFetches >= 24) {
					t.Fatalf("not selective: %+v", indexed.Stats)
				}
				if access == "SCAN" && indexed.Plan.Reason == "" {
					t.Fatal("missing fallback reason")
				}
			}
			compare("SEARCH")
			writeIntegrity(t, path)
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			db = nil
			confCLI(t, path, "BEGIN; UPDATE docs SET title='unrelated' WHERE id=1; INSERT INTO docs VALUES(99,'数据库','external'); DELETE FROM docs WHERE id=2; COMMIT;", false)
			db, err = sqlite.Open(path, openOptions)
			if err != nil {
				t.Fatal(err)
			}
			compare("SCAN")
			if err = db.Reindex(ctx, "docs_grams"); err != nil {
				t.Fatal(err)
			}
			compare("SEARCH")
			options.Threshold = 0
			compare("SCAN") // Zero-score ties cannot be dropped by a postings union.
			options.Threshold = 0.1
			options.Where = "id >= ?"
			options.Args = []any{int64(90)}
			compare("SEARCH")
			if shared {
				if err = db.Checkpoint(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if err = db.CheckExtensionIndex(ctx, "docs_grams"); err != nil {
				t.Fatal(err)
			}
			writeIntegrity(t, path)
		})
	}
}
