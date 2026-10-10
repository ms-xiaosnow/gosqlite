//go:build oracle

package testkit_test

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// All files remain intact. Coverage uses ordinary native/official SQL, normal
// rollback, checkpoint and close/reopen. No physical fault/recovery injection.
func TestOracleBitCanonicalStorageInterop(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	for _, page := range []uint32{512, 4096, 65536} {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("page%d/shared%t", page, shared), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "bits.db")
				opts := sqlite.Options{PageSize: page, WALShared: shared, KeepWALOnClose: shared}
				db, err := sqlite.Open(path, opts)
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
				writeExec(t, db, "CREATE TABLE bits(id INTEGER PRIMARY KEY,b BIT(9),v VECTOR(2),active INTEGER)")
				writeExec(t, db, "INSERT INTO bits VALUES(1,'001010001','[1,2]',1),(2,NULL,NULL,0)")
				v, err := vector.ParseBit("100000001")
				if err != nil {
					t.Fatal(err)
				}
				binary, _ := v.MarshalBinary()
				writeExec(t, db, "INSERT INTO bits VALUES(3,?,'[3,4]',1)", v)
				writeSame(t, db, path, "SELECT id,typeof(b),hex(b),hex(v),active FROM bits ORDER BY id")
				writeSame(t, db, path, "SELECT ~1,1&3,1|4,1<<3,8>>2,1+2&4,5%2")
				tx, err := db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				writeTxExec(t, tx, "UPDATE bits SET b='111111111' WHERE id=1")
				if err = tx.Rollback(); err != nil {
					t.Fatal(err)
				}
				writeSame(t, db, path, "SELECT id,hex(b) FROM bits ORDER BY id")
				if shared {
					if err = db.Checkpoint(ctx); err != nil {
						t.Fatal(err)
					}
				}
				writeIntegrity(t, path)
				if err = db.Close(); err != nil {
					t.Fatal(err)
				}
				db = nil
				// The external writer supplies a canonical GSB BLOB, preserving width and
				// bypassing ordinary SQLite's NUMERIC-affinity conversion of bit text.
				confCLI(t, path, "BEGIN; UPDATE bits SET b=x'"+hex.EncodeToString(binary)+"' WHERE id=1; INSERT INTO bits VALUES(8,x'"+hex.EncodeToString(binary)+"','[8,9]',1); COMMIT;", false)
				db, err = sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				rows, err := db.Query(ctx, "SELECT id,bit_to_text(b),bit_dims(b),b <~> ?,b <%> ? FROM bits WHERE active=1 ORDER BY id", v, v)
				if err != nil {
					t.Fatal(err)
				}
				var got [][]any
				for rows.Next() {
					got = append(got, rows.Values())
				}
				if err = errors.Join(rows.Err(), rows.Close()); err != nil {
					t.Fatal(err)
				}
				want := [][]any{{int64(1), v.String(), int64(9), float64(0), float64(0)}, {int64(3), v.String(), int64(9), float64(0), float64(0)}, {int64(8), v.String(), int64(9), float64(0), float64(0)}}
				if !reflect.DeepEqual(got, want) {
					t.Fatal(got, want)
				}
				search, err := db.BitSearch(ctx, "bits", "b", v, sqlite.BitSearchOptions{Metric: vector.Hamming, Limit: 2, Where: "active=1"})
				if err != nil || !reflect.DeepEqual(search.Hits, []vector.Hit{{RowID: 1}, {RowID: 3}}) {
					t.Fatal(search, err)
				}
				writeExec(t, db, "UPDATE bits SET b='000000000' WHERE id=1")
				writeSame(t, db, path, "SELECT id,typeof(b),hex(b) FROM bits ORDER BY id")
				if shared {
					if err = db.Checkpoint(ctx); err != nil {
						t.Fatal(err)
					}
				}
				if err = db.Close(); err != nil {
					t.Fatal(err)
				}
				db = nil
				db, err = sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				writeSame(t, db, path, "SELECT id,typeof(b),hex(b) FROM bits ORDER BY id")
				writeIntegrity(t, path)
			})
		}
	}
}

func TestOracleCreatedBitTypeAndNumericAffinityBoundary(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "official-bits.db")
	v, _ := vector.ParseBit("001")
	blob, _ := v.MarshalBinary()
	confCLI(t, path, "PRAGMA page_size=512; CREATE TABLE bits(id INTEGER PRIMARY KEY,b BIT(3)); CREATE INDEX bits_b ON bits(b); INSERT INTO bits VALUES(1,x'"+hex.EncodeToString(blob)+"'),(2,'001'),(3,NULL);", false)
	// '001' is legally converted to integer 1. This is a normal type-compatibility
	// case in a valid file, not byte corruption or an invalid database structure.
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r, err := db.Query(ctx, "SELECT id,bit_to_text(b),b <~> '001' FROM bits WHERE id=1")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Next() || !reflect.DeepEqual(r.Values(), []any{int64(1), "001", float64(0)}) {
		t.Fatal(r.Values(), r.Err())
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	// Excluding that row without referencing its bit column remains valid.
	writeSame(t, db, path, "SELECT id FROM bits ORDER BY id")
	for _, query := range []string{"SELECT b FROM bits WHERE id=2", "SELECT id FROM bits WHERE b=1", "SELECT id FROM bits ORDER BY b <~> '001' LIMIT 1"} {
		rows, e := db.Query(ctx, query)
		if e == nil {
			for rows.Next() {
			}
			e = errors.Join(rows.Err(), rows.Close())
		}
		if !errors.Is(e, sqlite.ErrMismatch) {
			t.Fatal(query, e)
		}
	}
	if got, err := db.BitSearch(ctx, "bits", "b", "001", sqlite.BitSearchOptions{Limit: 1}); !errors.Is(err, sqlite.ErrMismatch) || !reflect.DeepEqual(got, sqlite.BitResult{}) {
		t.Fatal(got, err)
	}
	// An explicit ordinary native rewrite restores a known width; no automatic
	// repair is attempted while reading the external numeric value.
	writeExec(t, db, "UPDATE bits SET b='001' WHERE id=2")
	writeSame(t, db, path, "SELECT id,typeof(b),hex(b) FROM bits ORDER BY id")
	writeIntegrity(t, path)
}

func TestOracleBitAddedColumnDefaultPreservesNumericAffinity(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	for _, blobDefault := range []bool{false, true} {
		t.Run(fmt.Sprintf("blob%t", blobDefault), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "defaults.db")
			v, _ := vector.ParseBit("001")
			b, _ := v.MarshalBinary()
			defaultSQL := "'001'"
			if blobDefault {
				defaultSQL = "x'" + hex.EncodeToString(b) + "'"
			}
			confCLI(t, path, "CREATE TABLE bits(id INTEGER PRIMARY KEY); INSERT INTO bits VALUES(1); ALTER TABLE bits ADD COLUMN b BIT(3) DEFAULT "+defaultSQL+";", false)
			db, err := sqlite.Open(path, sqlite.Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			writeSame(t, db, path, "SELECT id FROM bits")
			if blobDefault {
				writeSame(t, db, path, "SELECT id,typeof(b),hex(b) FROM bits")
				got, err := db.BitSearch(ctx, "bits", "b", "001", sqlite.BitSearchOptions{Limit: 1})
				if err != nil || !reflect.DeepEqual(got.Hits, []vector.Hit{{RowID: 1}}) {
					t.Fatal(got, err)
				}
			} else {
				r, err := db.Query(ctx, "SELECT b FROM bits")
				if err == nil {
					for r.Next() {
					}
					err = errors.Join(r.Err(), r.Close())
				}
				if !errors.Is(err, sqlite.ErrMismatch) {
					t.Fatal("numeric default silently reinterpreted", err)
				}
				if got, err := db.BitSearch(ctx, "bits", "b", "001", sqlite.BitSearchOptions{Limit: 1}); !errors.Is(err, sqlite.ErrMismatch) || !reflect.DeepEqual(got, sqlite.BitResult{}) {
					t.Fatal(got, err)
				}
			}
			writeExec(t, db, "UPDATE bits SET b='001' WHERE id=1")
			writeSame(t, db, path, "SELECT id,typeof(b),hex(b) FROM bits")
			writeIntegrity(t, path)
		})
	}
}
