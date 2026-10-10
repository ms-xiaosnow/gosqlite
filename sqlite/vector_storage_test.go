package sqlite_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Construct expected storage without using the implementation's encoder.
func nativeVectorReferenceBlob(values []float32) []byte {
	out := make([]byte, 12+4*len(values))
	copy(out, []byte{'G', 'S', 'V', 0, 1, 1, 1, 0})
	binary.BigEndian.PutUint32(out[8:12], uint32(len(values)))
	for i, value := range values {
		binary.BigEndian.PutUint32(out[12+4*i:], math.Float32bits(value))
	}
	return out
}

func assertStoredNativeVector(t *testing.T, value any, expected []float32) {
	t.Helper()
	blob, ok := value.([]byte)
	if !ok {
		t.Fatalf("vector was not stored as BLOB: %T", value)
	}
	want := nativeVectorReferenceBlob(expected)
	if !bytes.Equal(blob, want) {
		t.Fatalf("vector storage differs from independently encoded IEEE bits: got %x want %x", blob, want)
	}
	decoded, err := vector.Decode(blob)
	if err != nil {
		t.Fatal(err)
	}
	values := decoded.Values()
	if len(values) != len(expected) {
		t.Fatalf("decoded dimensions: got %d want %d", len(values), len(expected))
	}
	for i, f := range expected {
		if math.Float32bits(values[i]) != math.Float32bits(f) {
			t.Fatalf("component %d: got %08x want %08x", i, math.Float32bits(values[i]), math.Float32bits(f))
		}
	}
}

func TestVectorStorageCanonicalBitsAndOrdinaryReopen(t *testing.T) {
	values := []float32{
		math.Float32frombits(0x80000000), 0,
		math.SmallestNonzeroFloat32, -math.SmallestNonzeroFloat32,
		math.Float32frombits(0x007fffff), math.Float32frombits(0x00800000),
		math.MaxFloat32, -math.MaxFloat32, 1.5,
	}
	for _, wal := range []bool{false, true} {
		for _, pageSize := range []uint32{512, 4096} {
			t.Run(fmt.Sprintf("wal=%t/page=%d", wal, pageSize), func(t *testing.T) {
				opts := sqlite.Options{WALShared: wal, PageSize: pageSize}
				db, path := writable(t, opts)
				execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding vEcToR ( 9 ))")
				v := mustNativeVector(t, values...)
				input := nativeVectorReferenceBlob(values)
				execute(t, db, "INSERT INTO items VALUES(1,?),(2,?),(3,?),(4,NULL),(5,?),(6,?)", v.String(), v, input, vector.NullVector{}, []byte(nil))
				// Binding and row retrieval must not retain mutable caller buffers.
				for i := range input {
					input[i] = 0
				}
				verify := func(db *sqlite.DB) {
					t.Helper()
					rows := query(t, db, "SELECT id,embedding,typeof(embedding) FROM items ORDER BY id")
					if len(rows) != 6 {
						t.Fatalf("stored row count: %d", len(rows))
					}
					for i, row := range rows {
						if i < 3 {
							assertStoredNativeVector(t, row[1], values)
							if row[2] != "blob" {
								t.Fatalf("wrong canonical class: %+v", row)
							}
						} else if row[1] != nil || row[2] != "null" {
							t.Fatalf("NULL became a vector: %+v", row)
						}
					}
					rows[0][1].([]byte)[12] ^= 0xff
					assertStoredNativeVector(t, query(t, db, "SELECT embedding FROM items WHERE id=1")[0][0], values)
				}
				verify(db)
				// Text UPDATE takes the same canonical path, preserving signed zero.
				execute(t, db, "UPDATE items SET embedding=? WHERE id=2", v.String())
				if wal {
					if err := db.Checkpoint(context.Background()); err != nil {
						t.Fatal(err)
					}
				}
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
				reopened, err := sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				defer reopened.Close()
				verify(reopened)
				// The schema's dimension survives ordinary close/open too.
				if _, err := reopened.Exec(context.Background(), "UPDATE items SET embedding='[1,2,3]' WHERE id=1"); !errors.Is(err, vector.ErrDimensions) {
					t.Fatalf("reopened VECTOR(n) lost dimensions: %v", err)
				}
				verify(reopened)
			})
		}
	}
}

func TestVectorStorageLargeDenseOverflowAndExactSearch(t *testing.T) {
	// Exact dense vectors are not restricted by the separate 2000-dimension
	// ANN limit. A 16000-component BLOB also exercises normal overflow pages.
	values := make([]float32, vector.MaxDimensions)
	for i := range values {
		values[i] = float32(i%31-15) / 4
	}
	values[0], values[1] = math.Float32frombits(0x80000000), math.SmallestNonzeroFloat32
	db, path := writable(t, sqlite.Options{PageSize: 512})
	execute(t, db, fmt.Sprintf("CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(%d))", len(values)))
	v := mustNativeVector(t, values...)
	execute(t, db, "INSERT INTO items VALUES(1,?),(2,?)", v, v.String())
	for _, row := range query(t, db, "SELECT embedding FROM items ORDER BY id") {
		assertStoredNativeVector(t, row[0], values)
	}
	got := searchNativeVector(t, db, v, sqlite.VectorSearchOptions{Limit: 2})
	assertNativeVectorHits(t, got.Hits, []vector.Hit{{RowID: 1}, {RowID: 2}})
	if got.Plan.Dimensions != vector.MaxDimensions || got.Stats.ScoredComponents != 3*vector.MaxDimensions {
		t.Fatalf("large exact vector accounting: %+v", got)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for _, row := range query(t, reopened, "SELECT embedding FROM items ORDER BY id") {
		assertStoredNativeVector(t, row[0], values)
	}
	assertNativeVectorHits(t, searchNativeVector(t, reopened, v, sqlite.VectorSearchOptions{Limit: 2}).Hits, got.Hits)
}

func TestVectorStorageDefaultsNullAndNotNull(t *testing.T) {
	db, path := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3) DEFAULT '[1,-0,1e-45]',nullable VECTOR(3) DEFAULT NULL)")
	execute(t, db, "INSERT INTO items DEFAULT VALUES")
	rows := query(t, db, "SELECT embedding,nullable FROM items")
	assertStoredNativeVector(t, rows[0][0], []float32{1, math.Float32frombits(0x80000000), math.SmallestNonzeroFloat32})
	if rows[0][1] != nil {
		t.Fatal("NULL default did not stay NULL")
	}
	execute(t, db, "INSERT INTO items(id,embedding) VALUES(2,NULL)")
	if row := query(t, db, "SELECT embedding FROM items WHERE id=2")[0]; row[0] != nil {
		t.Fatal("explicit NULL was replaced by default", row)
	}
	execute(t, db, "CREATE TABLE required(id INTEGER PRIMARY KEY,embedding VECTOR(3) NOT NULL DEFAULT '[1,2,3]')")
	execute(t, db, "INSERT INTO required DEFAULT VALUES")
	for _, statement := range []string{"INSERT INTO required VALUES(2,NULL)", "UPDATE required SET embedding=NULL"} {
		if _, err := db.Exec(context.Background(), statement); !errors.Is(err, sqlite.ErrConstraint) {
			t.Fatalf("NOT NULL lost: %s: %v", statement, err)
		}
	}
	assertStoredNativeVector(t, query(t, db, "SELECT embedding FROM required")[0][0], []float32{1, 2, 3})
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	execute(t, reopened, "INSERT INTO items DEFAULT VALUES")
	rows = query(t, reopened, "SELECT embedding FROM items WHERE id=3")
	assertStoredNativeVector(t, rows[0][0], []float32{1, math.Float32frombits(0x80000000), math.SmallestNonzeroFloat32})
}

func TestVectorStorageRejectsInvalidDeclarationsAndDefaults(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	for _, declaration := range []string{
		"VECTOR", "VECTOR()", "VECTOR(0)", "VECTOR(-1)", "VECTOR(16001)", "VECTOR(1.5)", "VECTOR(3,4)",
		"VECTOR(3) DEFAULT '[1,2]'", "VECTOR(3) DEFAULT '[NaN,2,3]'", "VECTOR(3) DEFAULT '[]'",
		"VECTOR(3) DEFAULT 1", "VECTOR(3) DEFAULT X'5b312c322c335d'",
	} {
		_, err := db.Exec(context.Background(), "CREATE TABLE bad(id INTEGER PRIMARY KEY,embedding "+declaration+")")
		if err == nil {
			t.Fatalf("accepted invalid declaration/default: %s", declaration)
		}
		if got := query(t, db, "SELECT name FROM sqlite_schema WHERE name='bad'"); len(got) != 0 {
			t.Fatalf("failed DDL left schema entry: %s: %+v", declaration, got)
		}
	}
	// Every failed declaration leaves the same name available to valid DDL.
	execute(t, db, "CREATE TABLE bad(id INTEGER PRIMARY KEY,embedding VECTOR(3) DEFAULT '[1,2,3]')")
	execute(t, db, "INSERT INTO bad DEFAULT VALUES")
	assertStoredNativeVector(t, query(t, db, "SELECT embedding FROM bad")[0][0], []float32{1, 2, 3})
}

func TestVectorStorageInvalidInputsDoNotChangeRows(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3),label TEXT)")
	execute(t, db, "CREATE INDEX by_embedding ON items(embedding)")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,2,3]','kept')")
	validBlob := nativeVectorReferenceBlob([]float32{1, 2, 3})
	badVersion := append([]byte(nil), validBlob...)
	badVersion[4] = 2
	badFlags := append([]byte(nil), validBlob...)
	badFlags[7] = 1
	badOrder := append([]byte(nil), validBlob...)
	badOrder[6] = 2
	nonfinite := append([]byte(nil), validBlob...)
	binary.BigEndian.PutUint32(nonfinite[12:16], 0x7f800000)
	invalid := []any{
		"[]", "[1,2]", "[1,2,3,4]", "[1,2,3,]", "[NaN,2,3]", "[1,Inf,3]", "[1e100,2,3]",
		"[0x1p0,2,3]", "[1_0,2,3]", "['1',2,3]", "[1,2,3] trailing", "[1,2,3]\x00",
		int64(1), true, []float32{1, 2, 3}, vector.Vector{}, vector.NullVector{Valid: true},
		[]byte{}, []byte("[1,2,3]"), validBlob[:len(validBlob)-1], append(append([]byte(nil), validBlob...), 0),
		badVersion, badFlags, badOrder, nonfinite, nativeVectorReferenceBlob([]float32{1, 2}),
	}
	before := query(t, db, "SELECT * FROM items ORDER BY id")
	for i, value := range invalid {
		// Only API input is malformed; no database bytes are ever corrupted.
		for _, statement := range []string{
			"INSERT INTO items VALUES(2,'[3,2,1]','partial'),(3,?,'invalid')",
			"UPDATE items SET label='changed',embedding=? WHERE id=1",
		} {
			if _, err := db.Exec(context.Background(), statement, value); err == nil {
				t.Fatalf("input %d (%T) accepted: %s", i, value, statement)
			}
			if after := query(t, db, "SELECT * FROM items ORDER BY id"); !reflect.DeepEqual(after, before) {
				t.Fatalf("input %d left partial statement: got %+v want %+v", i, after, before)
			}
		}
	}
	// Rejection leaves both regular writes and the BLOB's ordinary B-tree
	// index maintainable; this does not claim an ANN/vector index exists.
	execute(t, db, "UPDATE items SET embedding=? WHERE id=1", mustNativeVector(t, 3, 2, 1))
	assertStoredNativeVector(t, query(t, db, "SELECT embedding FROM items")[0][0], []float32{3, 2, 1})
}

func TestVectorStorageLateUpdateFailureIsStatementAtomic(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3),proposed TEXT,label TEXT)")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,2,3]','[4,5,6]','first'),(2,'[2,3,4]','[1,2]','second')")
	before := query(t, db, "SELECT * FROM items ORDER BY id")
	if _, err := db.Exec(ctx, "UPDATE items SET embedding=proposed,label='changed'"); !errors.Is(err, vector.ErrDimensions) || !errors.Is(err, sqlite.ErrMismatch) {
		t.Fatalf("late dimension failure: %v", err)
	}
	if after := query(t, db, "SELECT * FROM items ORDER BY id"); !reflect.DeepEqual(after, before) {
		t.Fatalf("late UPDATE failure left earlier row changed: %+v", after)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	txExecute(t, tx, "INSERT INTO items VALUES(3,'[3,4,5]','[6,7,8]','earlier successful statement')")
	if _, err := tx.Exec(ctx, "UPDATE items SET embedding=proposed,label='changed'"); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO items VALUES(4,'[4,5,6]','[7,8,9]','partial'),(5,'[9]','[1,2,3]','bad')"); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
	// The statement rolls back, while the preceding successful statement stays.
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	after := query(t, db, "SELECT * FROM items ORDER BY id")
	if len(after) != 3 || !reflect.DeepEqual(after[:2], before) || after[2][3] != "earlier successful statement" {
		t.Fatalf("statement failure changed transaction history: %+v", after)
	}
	assertStoredNativeVector(t, after[2][1], []float32{3, 4, 5})
}
