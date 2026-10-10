//go:build oracle

package testkit_test

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"path/filepath"
	"reflect"
	"testing"
)

// Official SQLite is only a valid-file storage-class/format oracle here.
// All records are produced by ordinary SQL; no file mutation or recovery.
func TestOracleSparseCanonicalTextAndMaximumSupportInterop(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	for _, page := range []uint32{512, 4096, 65536} {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("page%d/shared%t", page, shared), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "sparse.db")
				opts := sqlite.Options{PageSize: page, WALShared: shared, KeepWALOnClose: shared}
				db, e := sqlite.Open(path, opts)
				if e != nil {
					t.Fatal(e)
				}
				defer func() {
					if db != nil {
						if e := db.Close(); e != nil {
							t.Error(e)
						}
					}
				}()
				text := "{1:3,1000000000:-2}/1000000000"
				v, e := vector.ParseSparse(text)
				if e != nil {
					t.Fatal(e)
				}
				binary, _ := v.MarshalBinary()
				writeExec(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,s SPARSEVEC(1000000000) DEFAULT '"+text+"',active INTEGER)")
				writeExec(t, db, "INSERT INTO items(id,active) VALUES(1,1)")
				writeExec(t, db, "INSERT INTO items VALUES(2,NULL,0)")
				writeExec(t, db, "CREATE TABLE wide(id INTEGER PRIMARY KEY,s SPARSEVEC(1000000000))")
				entries := make([]vector.SparseEntry, 16000)
				for i := range entries {
					entries[i] = vector.SparseEntry{Index: uint32(i * 60000), Value: float32(i%7 + 1)}
				}
				wide, e := vector.NewSparse(1000000000, entries)
				if e != nil {
					t.Fatal(e)
				}
				writeExec(t, db, "INSERT INTO wide VALUES(1,?)", wide)
				writeSame(t, db, path, "SELECT id,typeof(s),hex(s),active FROM items ORDER BY id")
				writeSame(t, db, path, "SELECT id,typeof(s),length(s),hex(s) FROM wide")
				tx, e := db.Begin(ctx)
				if e != nil {
					t.Fatal(e)
				}
				writeTxExec(t, tx, "UPDATE items SET s='{2:4}/1000000000' WHERE id=1")
				if e = tx.Rollback(); e != nil {
					t.Fatal(e)
				}
				if shared {
					if e = db.Checkpoint(ctx); e != nil {
						t.Fatal(e)
					}
				}
				writeIntegrity(t, path)
				if e = db.Close(); e != nil {
					t.Fatal(e)
				}
				db = nil
				confCLI(t, path, "BEGIN;INSERT INTO items VALUES(3,'"+text+"',1);INSERT INTO items VALUES(4,x'"+hex.EncodeToString(binary)+"',1);COMMIT;", false)
				db, e = sqlite.Open(path, opts)
				if e != nil {
					t.Fatal(e)
				}
				writeSame(t, db, path, "SELECT id,typeof(s),hex(s) FROM items ORDER BY id")
				writeSame(t, db, path, "SELECT id FROM items WHERE s='"+text+"' ORDER BY id")
				for _, expr := range []string{"s <-> ?", "? <-> s", "l2_distance(s,?)", "l2_distance(?,s)", "coalesce(s,NULL) <-> ?", "+s <-> ?", "nullif(s,NULL) <-> ?"} {
					rows, e := db.Query(ctx, "SELECT id,"+expr+" AS distance FROM items WHERE active=1 ORDER BY distance LIMIT 3", text)
					if e != nil {
						t.Fatal(expr, e)
					}
					var got [][]any
					for rows.Next() {
						got = append(got, rows.Values())
					}
					if e = errors.Join(rows.Err(), rows.Close()); e != nil {
						t.Fatal(e)
					}
					if !reflect.DeepEqual(got, [][]any{{int64(1), float64(0)}, {int64(3), float64(0)}, {int64(4), float64(0)}}) {
						t.Fatal(expr, got)
					}
				}
				rows, e := db.Query(ctx, "SELECT s AS renamed,sparsevec_to_text(s),typeof(s) FROM items WHERE id=3 AND l2_distance(renamed,?)=0", text)
				if e != nil {
					t.Fatal(e)
				}
				if !rows.Next() || rows.Values()[1] != text || rows.Values()[2] != "text" {
					t.Fatal(rows.Values(), rows.Err())
				}
				rows.Close()
				result, e := db.SparseSearch(ctx, "items", "s", text, sqlite.SparseSearchOptions{Limit: 3, Where: "active=1"})
				if e != nil || !reflect.DeepEqual(result.Hits, []vector.Hit{{RowID: 1}, {RowID: 3}, {RowID: 4}}) {
					t.Fatal(result, e)
				}
				writeExec(t, db, "UPDATE items SET s=s WHERE id=3")
				writeSame(t, db, path, "SELECT id,typeof(s),hex(s) FROM items ORDER BY id")
				if shared {
					if e = db.Checkpoint(ctx); e != nil {
						t.Fatal(e)
					}
				}
				if e = db.Close(); e != nil {
					t.Fatal(e)
				}
				db = nil
				db, e = sqlite.Open(path, opts)
				if e != nil {
					t.Fatal(e)
				}
				writeSame(t, db, path, "SELECT id,typeof(s),hex(s) FROM items ORDER BY id")
				writeSame(t, db, path, "SELECT id,length(s),hex(s) FROM wide")
				writeIntegrity(t, path)
			})
		}
	}
}

func TestOracleSparseExternalDefaultsAndUnknownSchema(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	for _, defaultSQL := range []string{"'{1:1}/1000000000'", "x'47535300010401003b9aca0000000001000000003f800000'", "'1'"} {
		t.Run(defaultSQL, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "external-sparse.db")
			confCLI(t, path, "CREATE TABLE items(id INTEGER PRIMARY KEY,active INTEGER);INSERT INTO items VALUES(1,1);ALTER TABLE items ADD COLUMN s SPARSEVEC(1000000000) DEFAULT "+defaultSQL+";CREATE INDEX s_key ON items(s);", false)
			db, e := sqlite.Open(path, sqlite.Options{})
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			writeSame(t, db, path, "SELECT id,active FROM items")
			if defaultSQL != "'1'" {
				writeSame(t, db, path, "SELECT id,typeof(s),hex(s) FROM items")
				got, e := db.SparseSearch(ctx, "items", "s", "{1:1}/1000000000", sqlite.SparseSearchOptions{Limit: 1})
				if e != nil || got.Hits[0].Distance != 0 {
					t.Fatal(got, e)
				}
			} else {
				for _, q := range []string{"SELECT s FROM items", "SELECT id FROM items WHERE s=2", "SELECT id FROM items ORDER BY s <-> '{1:1}/1000000000' LIMIT 1", "SELECT id FROM items ORDER BY l2_distance(s,'{1:1}/1000000000'),id LIMIT 1"} {
					rows, e := db.Query(ctx, q)
					if e == nil {
						for rows.Next() {
						}
						e = errors.Join(rows.Err(), rows.Close())
					}
					if !errors.Is(e, sqlite.ErrMismatch) {
						t.Fatal(q, e)
					}
				}
				got, e := db.SparseSearch(ctx, "items", "s", "{1:1}/1000000000", sqlite.SparseSearchOptions{Limit: 1, Where: "active=0"})
				if e != nil || got.Stats.FilteredRows != 1 {
					t.Fatal(got, e)
				}
			}
			writeExec(t, db, "UPDATE items SET s='{1:1}/1000000000' WHERE id=1")
			writeSame(t, db, path, "SELECT id,typeof(s),hex(s) FROM items")
			writeIntegrity(t, path)
		})
	}
	path := filepath.Join(t.TempDir(), "unknown-schema.db")
	confCLI(t, path, "CREATE TABLE unsupported(id INTEGER PRIMARY KEY,s SPARSEVEC(1000000001));INSERT INTO unsupported VALUES(1,'{} /1000000001');", false)
	db, e := sqlite.Open(path, sqlite.Options{})
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if rows, e := db.Query(ctx, "SELECT s FROM unsupported"); e == nil {
		rows.Close()
		t.Fatal("unknown schema was inferred")
	}
}
