//go:build oracle

package testkit_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Valid-data native/official SQLite interoperability only. Ordinary external
// source SQL marks the index dirty; no physical corruption or recovery occurs.
func TestOracleIVFFlatStorageInterop(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("page%d/shared%t", size, shared), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "ivf.db")
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
				if err = db.CreateIVFFlatIndex(ctx, "items_ivf", "items", "embedding", sqlite.IVFFlatIndexOptions{Lists: 4}); err != nil {
					t.Fatal(err)
				}
				search := func() sqlite.VectorResult {
					t.Helper()
					r, e := db.VectorSearch(ctx, "items", "embedding", "[20,6,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: "items_ivf", Probes: 1, MaxProbes: 3, Limit: 5})
					if e != nil {
						t.Fatal(e)
					}
					return r
				}
				assertScores := func(r sqlite.VectorResult) {
					t.Helper()
					all, e := db.VectorSearch(ctx, "items", "embedding", "[20,6,1]", sqlite.VectorSearchOptions{Limit: 100})
					if e != nil {
						t.Fatal(e)
					}
					truth := map[int64]float64{}
					for _, h := range all.Hits {
						truth[h.RowID] = h.Distance
					}
					for i, h := range r.Hits {
						d, ok := truth[h.RowID]
						if !ok || d != h.Distance {
							t.Fatalf("source exactrescore differs:%v", h)
						}
						if i > 0 && (r.Hits[i-1].Distance > h.Distance || (r.Hits[i-1].Distance == h.Distance && r.Hits[i-1].RowID >= h.RowID)) {
							t.Fatal("unstable ordering", r.Hits)
						}
					}
				}
				a := search()
				if a.Plan.Access != "SEARCH" || a.Plan.Method != "ivfflat" || a.Plan.Exact || a.Stats.ANNCentroidScores != 4 || a.Stats.ANNListsOpened < 1 || a.Stats.ANNMemberKeysRead < 5 || a.Stats.ANNNodeReads != 0 {
					t.Fatal(a)
				}
				assertScores(a)
				if err = db.CheckExtensionIndex(ctx, "items_ivf"); err != nil {
					t.Fatal(err)
				}
				writeSame(t, db, path, "SELECT id,hex(embedding),active FROM items ORDER BY id")
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
				confCLI(t, path, "BEGIN;UPDATE items SET embedding='[20,6,1]' WHERE id=70;INSERT INTO items VALUES(81,'[20,6,1]',1);DELETE FROM items WHERE id=1;COMMIT;", false)
				db, err = sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				dirty := search()
				exact, err := db.VectorSearch(ctx, "items", "embedding", "[20,6,1]", sqlite.VectorSearchOptions{Limit: 5})
				if err != nil {
					t.Fatal(err)
				}
				if dirty.Plan.Access != "SCAN" || !dirty.Plan.Exact || dirty.Plan.ANNFallback == "" || dirty.Stats.ANNCentroidScores != 0 || !reflect.DeepEqual(dirty.Hits, exact.Hits) {
					t.Fatal(dirty, exact)
				}
				_, err = db.Exec(ctx, "INSERT INTO items VALUES(82,'[3,4,1]',1)")
				if !errors.Is(err, sqlite.ErrReindexRequired) {
					t.Fatal(err)
				}
				if err = db.Reindex(ctx, "items_ivf"); err != nil {
					t.Fatal(err)
				}
				rebuilt := search()
				if rebuilt.Plan.Access != "SEARCH" || rebuilt.Plan.ANNGeneration <= a.Plan.ANNGeneration {
					t.Fatal(rebuilt)
				}
				assertScores(rebuilt)
				writeExec(t, db, "UPDATE items SET embedding='[4,4,1]' WHERE id=3")
				writeExec(t, db, "DELETE FROM items WHERE id=2")
				writeExec(t, db, "INSERT INTO items VALUES(2,'[20,6,1]',1)")
				if err = db.CheckExtensionIndex(ctx, "items_ivf"); err != nil {
					t.Fatal(err)
				}
				allProbes, err := db.VectorSearch(ctx, "items", "embedding", "[20,6,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: "items_ivf", Probes: 4, Limit: 5})
				if err != nil {
					t.Fatal(err)
				}
				exact, err = db.VectorSearch(ctx, "items", "embedding", "[20,6,1]", sqlite.VectorSearchOptions{Limit: 5})
				if err != nil {
					t.Fatal(err)
				}
				if !allProbes.Plan.Exact || allProbes.Plan.Access != "SCAN" || !reflect.DeepEqual(allProbes.Hits, exact.Hits) {
					t.Fatal(allProbes, exact)
				}
				if shared {
					if err = db.Checkpoint(ctx); err != nil {
						t.Fatal(err)
					}
				}
				writeSame(t, db, path, "SELECT id,hex(embedding),active FROM items ORDER BY id")
				writeIntegrity(t, path)
				if err = db.Close(); err != nil {
					t.Fatal(err)
				}
				db = nil
				db, err = sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				assertScores(search())
				if err = db.CheckExtensionIndex(ctx, "items_ivf"); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
