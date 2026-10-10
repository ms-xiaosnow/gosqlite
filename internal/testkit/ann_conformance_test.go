//go:build oracle

package testkit_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// This matrix uses only valid ordinary SQL and normal close/reopen/checkpoint.
// Official SQLite edits source rows through the exact installed dirty guards.
// No damaged database, interrupted process or physical failure is exercised.
func TestOracleHNSWStorageInterop(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("page%d/shared%t", size, shared), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "ann.db")
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
				writeExec(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3) NOT NULL,active INTEGER)")
				tx, err := db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				for i := 1; i <= 80; i++ {
					writeTxExec(t, tx, "INSERT INTO items VALUES(?,?,?)", i, fmt.Sprintf("[%d,%d,1]", i, i%7), i%2)
				}
				if err = tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				if err = db.CreateHNSWIndex(ctx, "items_hnsw", "items", "embedding"); err != nil {
					t.Fatal(err)
				}
				search := func() sqlite.VectorResult {
					t.Helper()
					r, e := db.VectorSearch(ctx, "items", "embedding", "[20,6,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: "items_hnsw", Limit: 5})
					if e != nil {
						t.Fatal(e)
					}
					return r
				}
				a := search()
				if a.Plan.Access != "SEARCH" || a.Plan.Method != "hnsw" || a.Plan.Exact || a.Stats.ANNNodeReads == 0 {
					t.Fatal(a)
				}
				if err = db.CheckExtensionIndex(ctx, "items_hnsw"); err != nil {
					t.Fatal(err)
				}
				writeSame(t, db, path, "SELECT id,typeof(embedding),hex(embedding),active FROM items ORDER BY id")
				writeIntegrity(t, path)
				if shared {
					if err = db.Checkpoint(ctx); err != nil {
						t.Fatal(err)
					}
				}
				if err = db.Close(); err != nil {
					t.Fatal(err)
				}
				db = nil
				confCLI(t, path, "BEGIN; UPDATE items SET embedding='[20,6,1]' WHERE id=70; INSERT INTO items VALUES(81,'[20,6,1]',1); DELETE FROM items WHERE id=1; COMMIT;", false)
				db, err = sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				dirty := search()
				exact, err := db.VectorSearch(ctx, "items", "embedding", "[20,6,1]", sqlite.VectorSearchOptions{Limit: 5})
				if err != nil {
					t.Fatal(err)
				}
				if dirty.Plan.Access != "SCAN" || !dirty.Plan.Exact || dirty.Plan.ANNFallback == "" || dirty.Stats.ANNNodeReads != 0 || !reflect.DeepEqual(dirty.Hits, exact.Hits) {
					t.Fatal(dirty, exact)
				}
				_, err = db.Exec(ctx, "INSERT INTO items VALUES(82,'[3,4,1]',1)")
				if !errors.Is(err, sqlite.ErrReindexRequired) {
					t.Fatalf("dirty write:%v", err)
				}
				if err = db.Reindex(ctx, "items_hnsw"); err != nil {
					t.Fatal(err)
				}
				rebuilt := search()
				if rebuilt.Plan.Access != "SEARCH" || rebuilt.Plan.ANNGeneration <= a.Plan.ANNGeneration {
					t.Fatal(rebuilt)
				}
				if !reflect.DeepEqual(rebuilt.Hits, exact.Hits) {
					t.Fatalf("rebuilt small-data hits %v exact%v", rebuilt.Hits, exact.Hits)
				}
				if err = db.CheckExtensionIndex(ctx, "items_hnsw"); err != nil {
					t.Fatal(err)
				}
				writeExec(t, db, "UPDATE items SET embedding='[4,4,1]' WHERE id=3")
				writeExec(t, db, "DELETE FROM items WHERE id=2")
				writeExec(t, db, "INSERT INTO items VALUES(2,'[20,6,1]',1)")
				if err = db.CheckExtensionIndex(ctx, "items_hnsw"); err != nil {
					t.Fatal(err)
				}
				if shared {
					if err = db.Checkpoint(ctx); err != nil {
						t.Fatal(err)
					}
				}
				writeSame(t, db, path, "SELECT id,typeof(embedding),hex(embedding),active FROM items ORDER BY id")
				writeIntegrity(t, path)
				if err = db.Close(); err != nil {
					t.Fatal(err)
				}
				db = nil
				db, err = sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				if r := search(); r.Plan.Access != "SEARCH" {
					t.Fatal(r)
				}
				if err = db.CheckExtensionIndex(ctx, "items_hnsw"); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestOracleHNSWLegacyGramsUpgrade(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ann", "legacy-grams.db"))
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != "5dd3237ecaeddb4b94c0019e4a21c7e851996c4438e3dfbe0bae8ec7cd3f92a8" {
		t.Fatalf("fixture hash %s", got)
	}
	path := filepath.Join(t.TempDir(), "legacy.db")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	assertLegacy := func() {
		t.Helper()
		rows, e := db.Query(ctx, "SELECT id FROM legacy_docs WHERE title LIKE '%数据库%' ORDER BY id")
		if e != nil {
			t.Fatal(e)
		}
		var ids []int64
		for rows.Next() {
			ids = append(ids, rows.Values()[0].(int64))
		}
		e = errors.Join(rows.Err(), rows.Close())
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(ids, []int64{1, 3}) || rows.Plan().Access != "SEARCH" || rows.Plan().Method != "grams13" {
			t.Fatal(ids, rows.Plan())
		}
		if e = db.CheckExtensionIndex(ctx, "legacy_grams"); e != nil {
			t.Fatal(e)
		}
	}
	assertLegacy()
	writeExec(t, db, "CREATE TABLE vectors(id INTEGER PRIMARY KEY,v VECTOR(2) NOT NULL)")
	writeExec(t, db, "INSERT INTO vectors VALUES(1,'[1,2]'),(2,'[2,3]'),(3,'[3,4]')")
	if err = db.CreateHNSWIndex(ctx, "vectors_hnsw", "vectors", "v"); err != nil {
		t.Fatal(err)
	}
	assertLegacy()
	if err = db.CheckExtensionIndex(ctx, "vectors_hnsw"); err != nil {
		t.Fatal(err)
	}
	writeSame(t, db, path, "SELECT id,title FROM legacy_docs ORDER BY id")
	writeIntegrity(t, path)
}
