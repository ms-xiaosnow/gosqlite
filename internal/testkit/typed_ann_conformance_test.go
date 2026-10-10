//go:build oracle

package testkit_test

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Valid SQL only, ordinary closes/checkpoints and external dirty guards. The
// official CLI is exclusively a test oracle; no physical mutation or recovery.
func TestOracleTypedANNStorageAndDirtyGuards(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	for _, page := range []uint32{512, 4096, 65536} {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("page%d/shared%t", page, shared), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "typed.db")
				opts := sqlite.Options{PageSize: page, WALShared: shared, KeepWALOnClose: shared}
				db, e := sqlite.Open(path, opts)
				if e != nil {
					t.Fatal(e)
				}
				defer func() {
					if db != nil {
						db.Close()
					}
				}()
				writeExec(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,h HALFVEC(2) NOT NULL,b BIT(16) NOT NULL,s SPARSEVEC(1000000000) NOT NULL)")
				for i := 1; i <= 24; i++ {
					writeExec(t, db, "INSERT INTO items VALUES(?,?,?,?)", i, fmt.Sprintf("[%d,1]", i), fmt.Sprintf("%016b", i), fmt.Sprintf("{1:%d,1000000000:1}/1000000000", i))
				}
				for _, field := range []string{"h", "b", "s"} {
					if e = db.CreateHNSWIndex(ctx, field+"_hnsw", "items", field); e != nil {
						t.Fatal(e)
					}
				}
				for _, field := range []string{"h", "b"} {
					if e = db.CreateIVFFlatIndex(ctx, field+"_ivf", "items", field, sqlite.IVFFlatIndexOptions{Lists: 2}); e != nil {
						t.Fatal(e)
					}
				}
				names := []string{"h_hnsw", "b_hnsw", "s_hnsw", "h_ivf", "b_ivf"}
				for _, name := range names {
					if e = db.CheckExtensionIndex(ctx, name); e != nil {
						t.Fatal(e)
					}
				}
				for _, q := range []string{"SELECT id,typeof(h),hex(h),typeof(b),hex(b),typeof(s),hex(s) FROM items ORDER BY id", "SELECT index_id,node_id,hex(original_vector),hex(adjacency) FROM __gosqlite_ext_ann_nodes ORDER BY id", "SELECT index_id,list_id,hex(centroid),member_count FROM __gosqlite_ext_ann_centroids ORDER BY id"} {
					writeSame(t, db, path, q)
				}
				writeIntegrity(t, path)
				if shared {
					if e = db.Checkpoint(ctx); e != nil {
						t.Fatal(e)
					}
				}
				if e = db.Close(); e != nil {
					t.Fatal(e)
				}
				db = nil
				half, e := vector.ParseHalf("[0.5,1]")
				if e != nil {
					t.Fatal(e)
				}
				encoded, e := half.MarshalBinary()
				if e != nil {
					t.Fatal(e)
				}
				confCLI(t, path, "BEGIN;UPDATE items SET h=X'"+hex.EncodeToString(encoded)+"' WHERE id=24;COMMIT;", false)
				db, e = sqlite.Open(path, opts)
				if e != nil {
					t.Fatal(e)
				}
				dirty, e := db.HalfSearch(ctx, "items", "h", "[0.5,1]", sqlite.HalfSearchOptions{Mode: sqlite.VectorModeApproximate, Index: "h_hnsw", Limit: 1})
				if e != nil || !dirty.Plan.Exact || dirty.Plan.ANNFallback == "" || dirty.Stats.ANNNodeReads != 0 || dirty.Hits[0].RowID != 24 {
					t.Fatal(dirty, e)
				}
				if _, e = db.Exec(ctx, "UPDATE items SET h='[7,1]' WHERE id=1"); !errors.Is(e, sqlite.ErrReindexRequired) {
					t.Fatal(e)
				}
				for _, name := range names {
					if e = db.Reindex(ctx, name); e != nil {
						t.Fatal(e)
					}
					if e = db.CheckExtensionIndex(ctx, name); e != nil {
						t.Fatal(e)
					}
				}
				current, e := db.HalfSearch(ctx, "items", "h", "[0.5,1]", sqlite.HalfSearchOptions{Mode: sqlite.VectorModeApproximate, Index: "h_hnsw", Limit: 1})
				if e != nil || current.Plan.Exact || current.Plan.MethodVersion != 2 || current.Stats.ANNNodeReads == 0 || current.Hits[0].RowID != 24 {
					t.Fatal(current, e)
				}
				writeExec(t, db, "BEGIN")
				writeExec(t, db, "DELETE FROM items WHERE id=2")
				writeExec(t, db, "ROLLBACK")
				if shared {
					if e = db.Checkpoint(ctx); e != nil {
						t.Fatal(e)
					}
				}
				writeSame(t, db, path, "SELECT id,hex(h),hex(b),hex(s) FROM items ORDER BY id")
				writeIntegrity(t, path)
			})
		}
	}
}

func TestOracleTypedANNSparseTextEntryAccounting(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sparse-text.db")
	// Ordinary official SQL preserves legal TEXT with explicit zeros; native
	// indexing keeps its canonical sparse value while source work counts input.
	confCLI(t, path, "CREATE TABLE items(id INTEGER PRIMARY KEY,s SPARSEVEC(1000000000) NOT NULL); INSERT INTO items VALUES(1,'{1:0,2:0,3:0}/1000000000');", false)
	db, e := sqlite.Open(path, sqlite.Options{})
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = db.CreateHNSWIndex(ctx, "idx", "items", "s"); e != nil {
		t.Fatal(e)
	}
	query := "{4:0,5:0}/1000000000"
	r, e := db.SparseSearch(ctx, "items", "s", query, sqlite.SparseSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 1})
	if e != nil || r.Plan.Exact || r.Stats.ScoredNonzeros != 7 || r.Stats.ANNWork.ComponentWork != 0 {
		t.Fatal(r, e)
	}
	exact, e := db.SparseSearch(ctx, "items", "s", query, sqlite.SparseSearchOptions{Limit: 1})
	if e != nil || exact.Stats.ScoredNonzeros != r.Stats.ScoredNonzeros {
		t.Fatal(exact, e)
	}
	for _, hint := range []string{"", " NOT INDEXED"} {
		rows, e := db.Query(ctx, "SELECT id FROM items"+hint+" ORDER BY s <-> ? LIMIT 1", query)
		if e != nil {
			t.Fatal(e)
		}
		for rows.Next() {
		}
		e = rows.Err()
		stats := rows.Stats()
		work := rows.ANNWork()
		rows.Close()
		if e != nil || stats.VectorRankNonzeros != 7 || work.ComponentWork != 0 {
			t.Fatal(hint, stats, work, e)
		}
	}
	if e = db.CheckExtensionIndex(ctx, "idx"); e != nil {
		t.Fatal(e)
	}
	writeIntegrity(t, path)
}
