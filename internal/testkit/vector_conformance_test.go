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
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Only ordinary SQL, statement/transaction rollback, normal close/reopen and
// checkpoint are exercised. No damaged files, physical faults or interrupted
// process/recovery workflows are created or executed.
func TestOracleDenseVectorStorageInterop(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("page%d/shared%t", size, shared), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "vectors.db")
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
				writeExec(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,label TEXT,embedding VECTOR(3))")
				writeExec(t, db, "CREATE TABLE preserved(id INTEGER PRIMARY KEY,note TEXT)")
				writeExec(t, db, "INSERT INTO preserved VALUES(7,'unchanged')")
				signed, err := vector.New([]float32{math.Float32frombits(1 << 31), math.SmallestNonzeroFloat32, -math.SmallestNonzeroFloat32})
				if err != nil {
					t.Fatal(err)
				}
				binary, err := signed.MarshalBinary()
				if err != nil {
					t.Fatal(err)
				}
				writeExec(t, db, "INSERT INTO items VALUES(1,'literal','[1,2,3]'),(2,'nullable',NULL)")
				writeExec(t, db, "INSERT INTO items VALUES(?,?,?)", int64(3), "bits", signed)
				writeSame(t, db, path, "SELECT id,label,typeof(embedding),hex(embedding) FROM items ORDER BY id")
				writeSame(t, db, path, "SELECT id,note FROM preserved")
				entries, err := db.Schema(ctx)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, e := range entries {
					if e.Name == "items" {
						found = strings.Contains(strings.ToUpper(e.SQL), "VECTOR(3)")
					}
				}
				if !found {
					t.Fatal("declared VECTOR(3) missing from legal schema")
				}
				rows, err := db.Query(ctx, "SELECT embedding FROM items WHERE id=3")
				if err != nil {
					t.Fatal(err)
				}
				if !rows.Next() {
					t.Fatalf("missing vector: %v", rows.Err())
				}
				got, ok := rows.Values()[0].([]byte)
				if !ok || !reflect.DeepEqual(got, binary) {
					t.Fatalf("float32 BLOB bits changed: %x want%x", got, binary)
				}
				if err = rows.Close(); err != nil {
					t.Fatal(err)
				}
				tx, err := db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				writeTxExec(t, tx, "UPDATE items SET embedding='[4,5,6]' WHERE id=1")
				if err = tx.Rollback(); err != nil {
					t.Fatal(err)
				}
				writeSame(t, db, path, "SELECT id,hex(embedding) FROM items ORDER BY id")
				writeIntegrity(t, path)
				if err = db.Close(); err != nil {
					t.Fatal(err)
				}
				db = nil
				// SQLite accepts the legal declaration and ordinary text/BLOB writes.
				// Native vector interpretation accepts only the documented value encodings.
				confCLI(t, path, "BEGIN; UPDATE items SET embedding='[2,3,4]' WHERE id=1; INSERT INTO items VALUES(8,'external',x'"+hex.EncodeToString(binary)+"'); COMMIT;", false)
				db, err = sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				q, err := db.Query(ctx, "SELECT id,vector_dims(embedding),l2_distance(embedding,'[2,3,4]') FROM items WHERE id=1")
				if err != nil {
					t.Fatal(err)
				}
				if !q.Next() {
					t.Fatalf("external vector row: %v", q.Err())
				}
				values := q.Values()
				if len(values) != 3 || values[0] != int64(1) || values[1] != int64(3) || values[2] != float64(0) {
					t.Fatalf("external text vector result: %#v", values)
				}
				if err = q.Close(); err != nil {
					t.Fatal(err)
				}
				writeExec(t, db, "UPDATE items SET embedding=? WHERE id=1", signed)
				writeSame(t, db, path, "SELECT id,label,typeof(embedding),hex(embedding) FROM items ORDER BY id")
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
				writeSame(t, db, path, "SELECT id,hex(embedding) FROM items ORDER BY id")
				writeSame(t, db, path, "SELECT id,note FROM preserved")
				writeIntegrity(t, path)
			})
		}
	}
}

func TestOracleCreatedDenseVectorDeclaration(t *testing.T) {
	confOracle(t)
	path := filepath.Join(t.TempDir(), "official-vectors.db")
	confCLI(t, path, "PRAGMA page_size=512; CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3)); INSERT INTO items VALUES(1,'[1,2,3]'),(2,NULL);", false)
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(context.Background(), "SELECT id,vector_dims(embedding),l1_distance(embedding,'[2,3,4]') FROM items ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	var result [][]any
	for rows.Next() {
		result = append(result, append([]any(nil), rows.Values()...))
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	want := [][]any{{int64(1), int64(3), float64(3)}, {int64(2), nil, nil}}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("official typed values got%#v want%#v", result, want)
	}
	writeExec(t, db, "UPDATE items SET embedding='[2,3,4]' WHERE id=1")
	writeSame(t, db, path, "SELECT id,typeof(embedding),hex(embedding) FROM items ORDER BY id")
	writeIntegrity(t, path)
}

// VECTOR(3) is an affinity declaration to official SQLite, which legally stores
// '[1,2]' as TEXT. This is a normal SQL type-compatibility negative case, not a
// damaged SQLite file or physical fault/recovery test.
func TestOracleVectorTypedReadErrorParity(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "typed-values.db")
	confCLI(t, path, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3)); INSERT INTO items VALUES(1,'[1,2]'); PRAGMA integrity_check;", false)
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		"SELECT id FROM items ORDER BY embedding <-> NULL LIMIT 1",
		"SELECT id FROM items ORDER BY embedding <-> NULL,id LIMIT 1",
		"SELECT embedding <-> NULL FROM items",
	} {
		rows, e := db.Query(ctx, q)
		if e == nil {
			for rows.Next() {
			}
			e = rows.Err()
			closeErr := rows.Close()
			e = errors.Join(e, closeErr)
		}
		if !errors.Is(e, sqlite.ErrMismatch) || !errors.Is(e, vector.ErrDimensions) {
			t.Fatalf("typed NULL-query error mismatch for %q: %v", q, e)
		}
	}
	// A predicate that does not reference the vector can exclude the row first.
	for _, q := range []string{
		"SELECT id FROM items WHERE id<0 ORDER BY embedding <-> NULL LIMIT 1",
		"SELECT id FROM items WHERE id<0 ORDER BY embedding <-> NULL,id LIMIT 1",
	} {
		rows, e := db.Query(ctx, q)
		if e != nil {
			t.Fatal(e)
		}
		if rows.Next() {
			t.Fatal("excluded row returned")
		}
		if e = errors.Join(rows.Err(), rows.Close()); e != nil {
			t.Fatalf("excluded vector unexpectedly interpreted: %v", e)
		}
	}
	// The well-formed SQLite file stays readable and a valid replacement is a
	// supported ordinary correction; no recovery routine is invoked.
	writeExec(t, db, "UPDATE items SET embedding='[1,2,3]' WHERE id=1")
	writeSame(t, db, path, "SELECT id,typeof(embedding),hex(embedding) FROM items")
	writeIntegrity(t, path)
}
