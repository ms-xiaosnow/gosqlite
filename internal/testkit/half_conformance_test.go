//go:build oracle

package testkit_test

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// The official shell is strictly a test-side ordinary SQLite format oracle.
// Every file remains intact; no physical failure or recovery is exercised.
func TestOracleHalfCanonicalStorageTextAndOverflowInterop(t *testing.T) {
	confOracle(t) // Requires the pinned version AND source identity.
	ctx := context.Background()
	for _, page := range []uint32{512, 4096, 65536} {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("page%d/shared%t", page, shared), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "half.db")
				opts := sqlite.Options{PageSize: page, WALShared: shared, KeepWALOnClose: shared}
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
				writeExec(t, db, "CREATE TABLE halves(id INTEGER PRIMARY KEY,h HALFVEC(3) DEFAULT '[1.0001,-0,5.960464477539063e-8]',d VECTOR(3),b BIT(3),active INTEGER)")
				writeExec(t, db, "INSERT INTO halves(id,d,b,active) VALUES(1,'[1.0001,-0,5.960464477539063e-8]','101',1)")
				writeExec(t, db, "INSERT INTO halves VALUES(2,NULL,NULL,NULL,0)")
				writeExec(t, db, "CREATE TABLE wide(id INTEGER PRIMARY KEY,h HALFVEC(16000))")
				bits := make([]uint16, 16000)
				for i := range bits {
					bits[i] = uint16(i%1024) | 0x3000
				}
				bits[0], bits[15999] = 0x8000, 1
				wide, e := vector.NewHalfBits(bits)
				if e != nil {
					t.Fatal(e)
				}
				writeExec(t, db, "INSERT INTO wide VALUES(1,?)", wide)
				writeSame(t, db, path, "SELECT id,typeof(h),hex(h),hex(d),hex(b),active FROM halves ORDER BY id")
				writeSame(t, db, path, "SELECT id,typeof(h),length(h),hex(h) FROM wide")
				rows, e := db.Query(ctx, "SELECT h FROM wide")
				if e != nil {
					t.Fatal(e)
				}
				if !rows.Next() {
					t.Fatal(rows.Err())
				}
				got, e := vector.DecodeHalf(rows.Values()[0].([]byte))
				if e != nil || !reflect.DeepEqual(got.Bits(), bits) {
					t.Fatal("wide overflow-page round trip", e)
				}
				if e = rows.Close(); e != nil {
					t.Fatal(e)
				}
				tx, e := db.Begin(ctx)
				if e != nil {
					t.Fatal(e)
				}
				writeTxExec(t, tx, "UPDATE halves SET h='[2,3,4]' WHERE id=1")
				if e = tx.Rollback(); e != nil {
					t.Fatal(e)
				}
				writeSame(t, db, path, "SELECT id,hex(h) FROM halves ORDER BY id")
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
				canonical := []byte{'G', 'S', 'H', 0, 1, 2, 1, 0, 0, 0, 0, 3, 0x3c, 0, 0x80, 0, 0, 1}
				// Both legal external storage classes survive unchanged on reads.
				confCLI(t, path, "BEGIN; INSERT INTO halves VALUES(3,'[1.0001,-0,5.960464477539063e-8]',NULL,NULL,1); INSERT INTO halves VALUES(4,x'"+hex.EncodeToString(canonical)+"',NULL,NULL,1); COMMIT;", false)
				db, err = sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				writeSame(t, db, path, "SELECT id,typeof(h),hex(h),hex(d),hex(b) FROM halves ORDER BY id")
				// Ordinary equality is storage equality, not pgvector equality:
				// identical half coordinates in TEXT and BLOB remain distinct.
				writeSame(t, db, path, "SELECT id FROM halves WHERE h='[1.0001,-0,5.960464477539063e-8]' ORDER BY id")
				for _, expr := range []string{"h <-> '[1,0,5.960464477539063e-8]'", "l2_distance(h,'[1,0,5.960464477539063e-8]')", "'[1,0,5.960464477539063e-8]' <-> h", "l2_distance('[1,0,5.960464477539063e-8]',h)"} {
					r, e := db.Query(ctx, "SELECT id,"+expr+" AS distance FROM halves WHERE active=1 ORDER BY distance LIMIT 3")
					if e != nil {
						t.Fatal(expr, e)
					}
					var result [][]any
					for r.Next() {
						result = append(result, r.Values())
					}
					if e = errors.Join(r.Err(), r.Close()); e != nil {
						t.Fatal(e)
					}
					if !reflect.DeepEqual(result, [][]any{{int64(1), float64(0)}, {int64(3), float64(0)}, {int64(4), float64(0)}}) {
						t.Fatal(expr, result)
					}
				}
				r, e := db.Query(ctx, "SELECT h AS renamed,l2_distance(h,'[1,0,5.960464477539063e-8]'),halfvec_to_text(subvector(h,1,1)),bit_to_text(binary_quantize(h)),typeof(h) FROM halves WHERE id=3 AND l2_distance(renamed,'[1,0,5.960464477539063e-8]')=0")
				if e != nil {
					t.Fatal(e)
				}
				if !r.Next() {
					t.Fatal(r.Err())
				}
				v := r.Values()
				if v[1] != float64(0) || v[2] != "[1]" || v[3] != "101" || v[4] != "text" {
					t.Fatal(v)
				}
				if e = r.Close(); e != nil {
					t.Fatal(e)
				}
				search, e := db.HalfSearch(ctx, "halves", "h", "[1,0,5.960464477539063e-8]", sqlite.HalfSearchOptions{Limit: 3, Where: "active=1"})
				if e != nil || !reflect.DeepEqual(search.Hits, []vector.Hit{{RowID: 1}, {RowID: 3}, {RowID: 4}}) {
					t.Fatal(search, e)
				}
				// Dense and BIT identities/math are unaffected by half use.
				r, e = db.Query(ctx, "SELECT d <-> '[1,0,5.960464477539063e-8]',b <~> '001',typeof(d),typeof(b) FROM halves WHERE id=1")
				if e != nil {
					t.Fatal(e)
				}
				if !r.Next() {
					t.Fatal(r.Err())
				}
				v = r.Values()
				if math.Abs(v[0].(float64)-float64(float32(1.0001)-1)) > 1e-15 || v[1] != float64(1) || v[2] != "blob" || v[3] != "blob" {
					t.Fatal(v)
				}
				r.Close()
				writeExec(t, db, "UPDATE halves SET h=h WHERE id=3")
				writeSame(t, db, path, "SELECT id,typeof(h),hex(h) FROM halves ORDER BY id")
				if shared {
					if e = db.Checkpoint(ctx); e != nil {
						t.Fatal(e)
					}
				}
				if e = db.Close(); e != nil {
					t.Fatal(e)
				}
				db = nil
				db, err = sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				writeSame(t, db, path, "SELECT id,typeof(h),hex(h) FROM halves ORDER BY id")
				writeSame(t, db, path, "SELECT id,hex(h) FROM wide")
				writeIntegrity(t, path)
			})
		}
	}
}

func TestOracleHalfExternalNumericAffinityAndAddedDefaults(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	for _, defaultSQL := range []string{"'[1.0001]'", "x'4753480001020100000000013c00'", "'1'"} {
		t.Run(defaultSQL, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "official-half.db")
			confCLI(t, path, "CREATE TABLE halves(id INTEGER PRIMARY KEY,active INTEGER); INSERT INTO halves VALUES(1,1); ALTER TABLE halves ADD COLUMN h HALFVEC(1) DEFAULT "+defaultSQL+"; CREATE INDEX half_key ON halves(h);", false)
			db, e := sqlite.Open(path, sqlite.Options{})
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			writeSame(t, db, path, "SELECT id,active FROM halves")
			if defaultSQL != "'1'" {
				writeSame(t, db, path, "SELECT id,typeof(h),hex(h) FROM halves")
				r, e := db.HalfSearch(ctx, "halves", "h", "[1]", sqlite.HalfSearchOptions{Limit: 1})
				if e != nil || !reflect.DeepEqual(r.Hits, []vector.Hit{{RowID: 1}}) {
					t.Fatal(r, e)
				}
			} else {
				for _, q := range []string{"SELECT h FROM halves", "SELECT id FROM halves WHERE h=2", "SELECT id FROM halves ORDER BY h <-> '[1]' LIMIT 1", "SELECT id FROM halves ORDER BY l2_distance(h,'[1]'),id LIMIT 1"} {
					r, e := db.Query(ctx, q)
					if e == nil {
						for r.Next() {
						}
						e = errors.Join(r.Err(), r.Close())
					}
					if !errors.Is(e, sqlite.ErrMismatch) {
						t.Fatal(q, e)
					}
				}
				r, e := db.HalfSearch(ctx, "halves", "h", "[1]", sqlite.HalfSearchOptions{Limit: 1})
				if !errors.Is(e, sqlite.ErrMismatch) || !reflect.DeepEqual(r, sqlite.HalfResult{}) {
					t.Fatal(r, e)
				}
				r, e = db.HalfSearch(ctx, "halves", "h", "[1]", sqlite.HalfSearchOptions{Limit: 1, Where: "active=0"})
				if e != nil || len(r.Hits) != 0 || r.Stats.FilteredRows != 1 {
					t.Fatal(r, e)
				}
			}
			writeExec(t, db, "UPDATE halves SET h='[1.0001]' WHERE id=1")
			writeSame(t, db, path, "SELECT id,typeof(h),hex(h) FROM halves")
			writeIntegrity(t, path)
		})
	}
}

func TestOracleHalfExternalTextRepresentationIdentity(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "typed-text.db")
	confCLI(t, path, "CREATE TABLE values_table(id INTEGER PRIMARY KEY,h HALFVEC(2),d VECTOR(2),label TEXT); INSERT INTO values_table VALUES(1,'[1.0001,2]','[1.0001,2]','[1.0001,2]');", false)
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, query := range []string{
		"SELECT h <-> d FROM values_table",
		"SELECT l2_distance(h,d) FROM values_table",
		"SELECT l2_distance(coalesce(h,h),d) FROM values_table",
		"SELECT halfvec(d) FROM values_table",
		"SELECT vector(h) FROM values_table",
		"SELECT halfvec_from_vector(h) FROM values_table",
		"SELECT vector_from_halfvec(d) FROM values_table",
		"SELECT halfvec_norm(d) FROM values_table",
		"SELECT vector_dims(h) FROM values_table",
		"SELECT h AS renamed FROM values_table WHERE vector_to_text(renamed)=label",
	} {
		r, e := db.Query(ctx, query)
		if e == nil {
			for r.Next() {
			}
			e = errors.Join(r.Err(), r.Close())
		}
		if e == nil {
			t.Fatal("external TEXT lost known representation", query)
		}
	}
	for _, query := range []string{
		"UPDATE values_table SET h=d",
		"UPDATE values_table SET d=h",
		"UPDATE values_table SET h=subvector(d,1,2)",
		"UPDATE values_table SET d=coalesce(h,h)",
	} {
		if _, e := db.Exec(ctx, query); e == nil {
			t.Fatal("implicit cross-kind assignment", query)
		}
	}
	// Every failed statement preserves the legal, externally authored TEXT.
	writeSame(t, db, path, "SELECT id,typeof(h),hex(h),typeof(d),hex(d) FROM values_table")
	r, err := db.Query(ctx, "SELECT l2_distance(h,'[1,2]'),l2_distance(d,'[1,2]'),l2_distance(label,'[1,2]'),l2_distance(coalesce(h,h),'[1,2]'),halfvec_to_text(halfvec_from_vector(d)),vector_to_text(vector_from_halfvec(h)) FROM values_table")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Next() {
		t.Fatal(r.Err())
	}
	wantDense := float64(float32(1.0001) - 1)
	if !reflect.DeepEqual(r.Values(), []any{float64(0), wantDense, wantDense, float64(0), "[1,2]", "[1,2]"}) {
		t.Fatal(r.Values())
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	writeExec(t, db, "UPDATE values_table SET h=halfvec_from_vector(d)")
	writeExec(t, db, "UPDATE values_table SET d=vector_from_halfvec(h)")
	writeSame(t, db, path, "SELECT id,typeof(h),hex(h),typeof(d),hex(d) FROM values_table")
	writeIntegrity(t, path)
}

func TestOracleHalfExternalLegalStorageClassBoundary(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legal-values.db")
	// Official SQLite permits every value below in its NUMERIC-affinity column.
	// These are legal SQL values in an intact file, not damaged storage bytes.
	confCLI(t, path, "CREATE TABLE halves(id INTEGER PRIMARY KEY,h HALFVEC(2)); CREATE INDEX half_key ON halves(h); INSERT INTO halves VALUES(1,'[1.0001,2]'),(2,x'4753480001020100000000023c004000'),(3,1),(4,1.25),(5,x'5b312c325d'),(6,x'4753560001010100000000023f80000040000000');", false)
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	writeSame(t, db, path, "SELECT id FROM halves ORDER BY id")
	writeSame(t, db, path, "SELECT id,typeof(h),hex(h) FROM halves WHERE id<=2 ORDER BY id")
	good, err := db.HalfSearch(ctx, "halves", "h", "[1,2]", sqlite.HalfSearchOptions{Limit: 2, Where: "id<=2"})
	if err != nil || !reflect.DeepEqual(good.Hits, []vector.Hit{{RowID: 1}, {RowID: 2}}) || good.Stats.FilteredRows != 4 {
		t.Fatal(good, err)
	}
	for _, id := range []int{3, 4, 5, 6} {
		for _, expr := range []string{"h", "halfvec_to_text(h)", "h <-> '[1,2]'"} {
			r, e := db.Query(ctx, fmt.Sprintf("SELECT %s FROM halves WHERE id=%d", expr, id))
			if e == nil {
				for r.Next() {
				}
				e = errors.Join(r.Err(), r.Close())
			}
			if !errors.Is(e, sqlite.ErrMismatch) {
				t.Fatal(id, expr, e)
			}
		}
	}
	failed, err := db.HalfSearch(ctx, "halves", "h", "[1,2]", sqlite.HalfSearchOptions{Limit: 1})
	if !errors.Is(err, sqlite.ErrMismatch) || !reflect.DeepEqual(failed, sqlite.HalfResult{}) {
		t.Fatal(failed, err)
	}
	writeExec(t, db, "UPDATE halves SET h='[1.0001,2]' WHERE id>=3")
	writeSame(t, db, path, "SELECT id,typeof(h),hex(h) FROM halves ORDER BY id")
	writeIntegrity(t, path)
}
