package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// These bounded cases use intact SQL-created files, normal transactions and
// logical constraint errors only. They do not exercise physical fault recovery.
func uniqueConstraint(t *testing.T, db interface {
	Exec(context.Context, string, ...any) (sqlite.Result, error)
}, q string, args ...any) {
	t.Helper()
	r, err := db.Exec(context.Background(), q, args...)
	var detail *sqlite.Error
	if !errors.Is(err, sqlite.ErrConstraint) || !errors.As(err, &detail) || detail.Code != sqlite.CodeConstraint {
		t.Fatalf("%s: want primary constraint error, got %v", q, err)
	}
	if n, err := r.RowsAffected(); err != nil || n != 0 {
		t.Fatalf("failed statement affected=%d err=%v", n, err)
	}
	if n, err := r.LastInsertId(); err != nil || n != 0 {
		t.Fatalf("failed statement insert id=%d err=%v", n, err)
	}
}

func uniqueRows(t *testing.T, db *sqlite.DB, q string, want [][]any, args ...any) {
	t.Helper()
	if got := query(t, db, q, args...); !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: got %#v want %#v", q, got, want)
	}
}

func TestUniqueIndexBuildRollbackAndIfNotExists(t *testing.T) {
	db, _ := writable(t, sqlite.Options{PageSize: 512})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT)")
	for i := 1; i <= 20; i++ {
		execute(t, db, "INSERT INTO t VALUES(?,?)", i, fmt.Sprintf("%03d-", i)+strings.Repeat("build界", 200))
	}
	last := "020-" + strings.Repeat("build界", 200)
	execute(t, db, "INSERT INTO t VALUES(21,?)", last)
	beforeRows := query(t, db, "SELECT id,k FROM t ORDER BY id")
	beforeSchema, err := db.Schema(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	beforeInfo, beforeCheck := db.Info(), checkPass(t, db)
	uniqueConstraint(t, db, "CREATE UNIQUE INDEX rejected ON t(k COLLATE BINARY ASC)")
	// Refresh from a real snapshot: rollback restores the cached Info value,
	// which alone cannot establish that the persisted schema cookie is intact.
	afterSchema, err := db.Schema(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(afterSchema, beforeSchema) {
		t.Fatalf("failed build changed catalog: before=%+v after=%+v", beforeSchema, afterSchema)
	}
	if got := db.Info(); got != beforeInfo {
		t.Fatalf("failed build changed header: before=%+v after=%+v", beforeInfo, got)
	}
	if got := checkPass(t, db); got.Pages != beforeCheck.Pages || got.FreePages != beforeCheck.FreePages || got.Indexes != beforeCheck.Indexes || got.TableRows != beforeCheck.TableRows {
		t.Fatalf("failed build changed allocations/catalog: before=%+v after=%+v", beforeCheck, got)
	}
	uniqueRows(t, db, "SELECT id,k FROM t ORDER BY id", beforeRows)
	uniqueRows(t, db, "SELECT name FROM sqlite_schema WHERE name='rejected'", [][]any{})
	execute(t, db, "DELETE FROM t WHERE id=21")
	execute(t, db, "CREATE UNIQUE INDEX rejected ON t(k COLLATE BINARY ASC)")
	uniqueConstraint(t, db, "INSERT INTO t VALUES(22,?)", last)
	execute(t, db, "CREATE UNIQUE INDEX IF NOT EXISTS rejected ON t(k)")
	uniqueRows(t, db, "SELECT count(*) FROM t", [][]any{{int64(20)}})
	checkPass(t, db)

	execute(t, db, "CREATE TABLE ordinary(id INTEGER PRIMARY KEY,k)")
	execute(t, db, "CREATE INDEX remains_nonunique ON ordinary(k)")
	execute(t, db, "INSERT INTO ordinary VALUES(1,7),(2,7)")
	ordinarySQL := query(t, db, "SELECT sql FROM sqlite_schema WHERE name='remains_nonunique'")
	execute(t, db, "CREATE UNIQUE INDEX IF NOT EXISTS remains_nonunique ON ordinary(k)")
	execute(t, db, "INSERT INTO ordinary VALUES(3,7)")
	uniqueRows(t, db, "SELECT sql FROM sqlite_schema WHERE name='remains_nonunique'", ordinarySQL)
	uniqueRows(t, db, "SELECT count(*) FROM ordinary", [][]any{{int64(3)}})
	checkPass(t, db)
}

func TestUniqueIndexNullCompositeAndStorageClasses(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE composite(id INTEGER PRIMARY KEY,a,b,c)")
	execute(t, db, "CREATE UNIQUE INDEX unique_abc ON composite(a,b,c)")
	execute(t, db, "INSERT INTO composite VALUES(1,1,2,3),(2,1,2,4),(3,1,3,3),(4,2,2,3)")
	uniqueConstraint(t, db, "INSERT INTO composite VALUES(5,1,2,3)")
	for pos := 0; pos < 3; pos++ {
		values := []any{int64(1), int64(2), int64(3)}
		values[pos] = nil
		for repeat := 0; repeat < 2; repeat++ {
			execute(t, db, "INSERT INTO composite VALUES(?,?,?,?)", 10+pos*2+repeat, values[0], values[1], values[2])
		}
	}
	execute(t, db, "INSERT INTO composite VALUES(20,NULL,NULL,NULL),(21,NULL,NULL,NULL)")
	execute(t, db, "UPDATE composite SET a=NULL WHERE id IN (1,2)")
	uniqueRows(t, db, "SELECT count(*) FROM composite", [][]any{{int64(12)}})

	execute(t, db, "CREATE TABLE classes(id INTEGER PRIMARY KEY,k)")
	execute(t, db, "CREATE UNIQUE INDEX unique_class ON classes(k)")
	values := []any{int64(7), "7", []byte("7"), "", []byte{}, "a\x00界", []byte("a\x00界"), "界🙂", nil, nil}
	for i, value := range values {
		execute(t, db, "INSERT INTO classes VALUES(?,?)", i+1, value)
	}
	for _, value := range values[:8] {
		uniqueConstraint(t, db, "INSERT INTO classes VALUES(100,?)", value)
	}
	uniqueConstraint(t, db, "INSERT INTO classes VALUES(100,7.0)")
	uniqueRows(t, db, "SELECT id,typeof(k) FROM classes ORDER BY id", [][]any{
		{int64(1), "integer"}, {int64(2), "text"}, {int64(3), "blob"}, {int64(4), "text"}, {int64(5), "blob"},
		{int64(6), "text"}, {int64(7), "blob"}, {int64(8), "text"}, {int64(9), "null"}, {int64(10), "null"},
	})
	uniqueRows(t, db, "SELECT k FROM classes WHERE id=6", [][]any{{"a\x00界"}})
	uniqueRows(t, db, "SELECT k FROM classes WHERE id=7", [][]any{{[]byte("a\x00界")}})
	checkPass(t, db)
}

func TestUniqueIndexAffinityAndExactNumeric(t *testing.T) {
	for _, tc := range []struct {
		name, affinity, storage string
		first, stored           any
		conflicts               []any
		distinct                any
	}{
		{"numeric", "NUMERIC", "integer", "01", int64(1), []any{int64(1), float64(1), "1.0", "1e0"}, []byte("1")},
		{"integer", "INTEGER", "integer", "01", int64(1), []any{int64(1), float64(1), "1.0"}, "one"},
		{"real", "REAL", "real", "01", float64(1), []any{int64(1), float64(1), "1.0"}, float64(1.25)},
		{"text", "TEXT", "text", int64(1), "1", []any{"1", int64(1)}, "01"},
		{"blob", "BLOB", "integer", int64(1), int64(1), []any{float64(1)}, "1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := writable(t, sqlite.Options{})
			execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k "+tc.affinity+")")
			execute(t, db, "CREATE UNIQUE INDEX unique_k ON t(k)")
			execute(t, db, "INSERT INTO t VALUES(1,?)", tc.first)
			uniqueRows(t, db, "SELECT k,typeof(k) FROM t", [][]any{{tc.stored, tc.storage}})
			for _, value := range tc.conflicts {
				uniqueConstraint(t, db, "INSERT INTO t VALUES(2,?)", value)
			}
			execute(t, db, "INSERT INTO t VALUES(2,?)", tc.distinct)
			uniqueConstraint(t, db, "UPDATE t SET k=? WHERE id=2", tc.first)
			checkPass(t, db)
		})
	}

	db, _ := writable(t, sqlite.Options{PageSize: 512})
	execute(t, db, "CREATE TABLE exact(id INTEGER PRIMARY KEY,k)")
	execute(t, db, "CREATE UNIQUE INDEX unique_exact ON exact(k)")
	values := []any{int64(math.MinInt64), int64(math.MaxInt64), int64(9007199254740991), int64(9007199254740992), int64(9007199254740993), float64(9007199254740994), float64(math.MaxInt64), int64(0)}
	for i, value := range values {
		execute(t, db, "INSERT INTO exact VALUES(?,?)", i+1, value)
	}
	for _, value := range []any{float64(math.MinInt64), float64(9007199254740992), int64(9007199254740994), math.Copysign(0, -1), float64(0)} {
		uniqueConstraint(t, db, "INSERT INTO exact VALUES(100,?)", value)
	}
	uniqueRows(t, db, "SELECT k,typeof(k) FROM exact ORDER BY id", [][]any{
		{int64(math.MinInt64), "integer"}, {int64(math.MaxInt64), "integer"}, {int64(9007199254740991), "integer"},
		{int64(9007199254740992), "integer"}, {int64(9007199254740993), "integer"}, {float64(9007199254740994), "real"},
		{float64(math.MaxInt64), "real"}, {int64(0), "integer"},
	})
	checkPass(t, db)
}

func TestUniqueIndexSignedRowIDSelfExclusion(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT)")
	execute(t, db, "CREATE UNIQUE INDEX unique_k ON t(k)")
	execute(t, db, "CREATE UNIQUE INDEX unique_id ON t(id)")
	ids := []int64{math.MinInt64, -7, -1, 0, 1, 7, math.MaxInt64}
	for _, id := range ids {
		execute(t, db, "INSERT INTO t VALUES(?,?)", id, fmt.Sprint(id))
		execute(t, db, "UPDATE t SET k=k WHERE id=?", id)
		execute(t, db, "UPDATE t SET id=id WHERE id=?", id)
		uniqueConstraint(t, db, "INSERT INTO t VALUES(42,?)", fmt.Sprint(id))
	}
	// Zero is an actual old rowid, never an absent-self sentinel. A rowid
	// change retains its key but must continue to exclude only that same row.
	execute(t, db, "UPDATE t SET id=2 WHERE id=0")
	execute(t, db, "UPDATE t SET id=0 WHERE id=2")
	execute(t, db, "UPDATE t SET id=-8 WHERE id=-7")
	execute(t, db, "UPDATE t SET id=-7 WHERE id=-8")
	uniqueConstraint(t, db, "UPDATE t SET k='0' WHERE id=-7")
	uniqueConstraint(t, db, "UPDATE t SET k='-7' WHERE id=0")
	uniqueRows(t, db, "SELECT id,k FROM t WHERE id IN (-7,0) ORDER BY id", [][]any{{int64(-7), "-7"}, {int64(0), "0"}})
	execute(t, db, "DELETE FROM t WHERE id=0")
	execute(t, db, "INSERT INTO t VALUES(0,'0')")
	checkPass(t, db)
}

func TestUniqueIndexImmediateUpdatesAndCounters(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER,note TEXT)")
	execute(t, db, "CREATE UNIQUE INDEX unique_k ON t(k)")
	execute(t, db, "CREATE INDEX ordinary_note ON t(note)")
	execute(t, db, "INSERT INTO t VALUES(1,1,'one'),(2,2,'two'),(3,3,'three')")
	checkCounters(t, db, 3, 3, 3)
	uniqueConstraint(t, db, "INSERT INTO t VALUES(99,1,'first conflict')")
	checkCounters(t, db, 0, 3, 3)
	uniqueConstraint(t, db, "INSERT INTO t VALUES(10,10,'partial'),(11,2,'later conflict')")
	checkCounters(t, db, 0, 3, 10)
	before := query(t, db, "SELECT id,k,note FROM t ORDER BY id")
	// A hypothetical final set would be unique, but occupied keys conflict
	// immediately while visiting rows; the statement may not defer its checks.
	uniqueConstraint(t, db, "UPDATE t SET k=4-k")
	uniqueRows(t, db, "SELECT id,k,note FROM t ORDER BY id", before)
	uniqueConstraint(t, db, "UPDATE t SET k=CASE WHEN id=1 THEN 4 ELSE 3 END,note='partial' WHERE id<=2")
	uniqueRows(t, db, "SELECT id,k,note FROM t ORDER BY id", before)
	checkCounters(t, db, 0, 3, 10)
	uniqueRows(t, db, "SELECT id FROM t WHERE note='partial' ORDER BY id", [][]any{})
	execute(t, db, "UPDATE t SET k=k,note=note")
	checkCounters(t, db, 3, 6, 10)
	execute(t, db, "DELETE FROM t WHERE id=2")
	execute(t, db, "INSERT INTO t VALUES(20,2,'reused')")
	checkCounters(t, db, 1, 8, 20)
	checkPass(t, db)
}

func TestUniqueIndexTransactionsAndSchemaRollback(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			db, _ := writable(t, sqlite.Options{WALShared: shared})
			execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT)")
			execute(t, db, "INSERT INTO t VALUES(1,'base')")
			execute(t, db, "BEGIN")
			execute(t, db, "INSERT INTO t VALUES(2,'kept')")
			execute(t, db, "SAVEPOINT before_index")
			execute(t, db, "CREATE UNIQUE INDEX unique_k ON t(k)")
			uniqueConstraint(t, db, "INSERT INTO t VALUES(3,'partial'),(4,'base')")
			uniqueRows(t, db, "SELECT id,k FROM t ORDER BY id", [][]any{{int64(1), "base"}, {int64(2), "kept"}})
			checkPass(t, db)
			execute(t, db, "ROLLBACK TO before_index")
			uniqueRows(t, db, "SELECT name FROM sqlite_schema WHERE name='unique_k'", [][]any{})
			execute(t, db, "INSERT INTO t VALUES(3,'base')")
			uniqueConstraint(t, db, "CREATE UNIQUE INDEX unique_k ON t(k)")
			execute(t, db, "DELETE FROM t WHERE id=3")
			execute(t, db, "CREATE UNIQUE INDEX unique_k ON t(k)")
			execute(t, db, "RELEASE before_index")
			execute(t, db, "COMMIT")
			uniqueConstraint(t, db, "INSERT INTO t VALUES(5,'kept')")
			execute(t, db, "BEGIN")
			execute(t, db, "SAVEPOINT rows")
			execute(t, db, "DELETE FROM t WHERE id=1")
			execute(t, db, "INSERT INTO t VALUES(10,'base')")
			execute(t, db, "ROLLBACK TO rows")
			uniqueConstraint(t, db, "INSERT INTO t VALUES(11,'base')")
			execute(t, db, "UPDATE t SET k='temporary' WHERE id=1")
			execute(t, db, "ROLLBACK")
			uniqueConstraint(t, db, "INSERT INTO t VALUES(12,'base')")
			uniqueRows(t, db, "SELECT id,k FROM t ORDER BY id", [][]any{{int64(1), "base"}, {int64(2), "kept"}})
			execute(t, db, "CREATE TABLE ddl(id INTEGER PRIMARY KEY,k)")
			execute(t, db, "INSERT INTO ddl VALUES(1,7)")
			execute(t, db, "BEGIN")
			execute(t, db, "CREATE UNIQUE INDEX rolled_back ON ddl(k)")
			execute(t, db, "ROLLBACK")
			execute(t, db, "INSERT INTO ddl VALUES(2,7)")
			uniqueRows(t, db, "SELECT name FROM sqlite_schema WHERE name='rolled_back'", [][]any{})
			checkPass(t, db)
		})
	}
}

func TestUniqueIndexPageSizesReopenAndPeerVisibility(t *testing.T) {
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("page%d/shared%t", size, shared), func(t *testing.T) {
				opts := sqlite.Options{PageSize: size, WALShared: shared}
				db, path := writable(t, opts)
				execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT,b BLOB)")
				tx, err := db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				for i := 1; i <= 96; i++ {
					txExecute(t, tx, "INSERT INTO t VALUES(?,?,?)", i, fmt.Sprintf("%03d-\x00", i)+strings.Repeat("界", 700), []byte{0, byte(i), 255})
				}
				if err = tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				execute(t, db, "CREATE UNIQUE INDEX unique_kb ON t(k COLLATE BINARY ASC,b)")
				key := "096-\x00" + strings.Repeat("界", 700)
				uniqueConstraint(t, db, "INSERT INTO t VALUES(100,?,?)", key, []byte{0, 96, 255})
				if r := checkPass(t, db); r.Indexes != 1 || r.TableRows != 96 || r.IndexEntries != 96 {
					t.Fatal(r)
				}
				peer, err := sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				defer peer.Close()
				writer, err := db.Begin(ctx, sqlite.TxOptions{Immediate: true})
				if err != nil {
					t.Fatal(err)
				}
				txExecute(t, writer, "INSERT INTO t VALUES(200,'committed',X'00')")
				if _, err = peer.Exec(ctx, "INSERT INTO t VALUES(201,'committed',X'00')"); !errors.Is(err, sqlite.ErrBusy) {
					t.Fatalf("second writer must be BUSY before constraint probing: %v", err)
				}
				if err = writer.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				uniqueConstraint(t, peer, "INSERT INTO t VALUES(201,'committed',X'00')")
				execute(t, peer, "DELETE FROM t WHERE id=200")
				execute(t, db, "INSERT INTO t VALUES(202,'committed',X'00')")
				checkPass(t, peer)
				if err = peer.Close(); err != nil {
					t.Fatal(err)
				}
				if shared {
					if err = db.Checkpoint(ctx); err != nil {
						t.Fatal(err)
					}
				}
				if err = db.Close(); err != nil {
					t.Fatal(err)
				}
				reopened, err := sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				defer reopened.Close()
				uniqueConstraint(t, reopened, "INSERT INTO t VALUES(203,'committed',X'00')")
				uniqueConstraint(t, reopened, "INSERT INTO t VALUES(204,?,?)", key, []byte{0, 96, 255})
				uniqueRows(t, reopened, "SELECT count(*) FROM t", [][]any{{int64(97)}})
				uniqueRows(t, reopened, "SELECT k,b FROM t WHERE id=96", [][]any{{key, []byte{0, 96, 255}}})
				checkPass(t, reopened)
			})
		}
	}
}

func TestUniqueIndexExclusiveWALReopen(t *testing.T) {
	opts := sqlite.Options{PageSize: 4096, WALExclusive: true}
	db, path := writable(t, opts)
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k)")
	execute(t, db, "CREATE UNIQUE INDEX unique_k ON t(k)")
	execute(t, db, "INSERT INTO t VALUES(0,'zero'),(-1,NULL),(-2,NULL)")
	uniqueConstraint(t, db, "INSERT INTO t VALUES(1,'zero')")
	checkPass(t, db)
	if err := db.Checkpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sqlite.Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	uniqueConstraint(t, reopened, "INSERT INTO t VALUES(1,'zero')")
	execute(t, reopened, "UPDATE t SET id=7 WHERE id=0")
	checkPass(t, reopened)
}

func TestUniqueIndexPlannerConservativeFallback(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER)")
	execute(t, db, "CREATE UNIQUE INDEX unique_k ON t(k)")
	execute(t, db, "INSERT INTO t VALUES(1,10),(2,20),(3,30)")
	for _, q := range []string{
		"SELECT id FROM t WHERE k=20 ORDER BY id",
		"SELECT id FROM t WHERE k>=20 ORDER BY id",
	} {
		p, s, _ := ordinaryCompare(t, db, q)
		if p.Access != "SCAN" || s.IndexKeysRead != 0 || s.IndexPageReads != 0 {
			t.Fatalf("constraint-only unique index unexpectedly changed SELECT admission: %+v %+v", p, s)
		}
	}
	execute(t, db, "CREATE INDEX ordinary_k ON t(k)")
	p, _, _ := ordinaryCompare(t, db, "SELECT id FROM t WHERE k=20 ORDER BY id")
	if p.Method != "btree" || !reflect.DeepEqual(p.Indexes, []string{"ordinary_k"}) {
		t.Fatalf("existing ordinary-index plan changed: %+v", p)
	}
	checkPass(t, db)
}

func TestUniqueIndexGramsStatementUndo(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,slug TEXT,body TEXT)")
	execute(t, db, "CREATE UNIQUE INDEX unique_slug ON docs(slug)")
	execute(t, db, "CREATE INDEX body_grams ON docs(body) USING grams13")
	execute(t, db, "INSERT INTO docs VALUES(1,'one','stable alpha'),(2,'two','stable beta')")
	before := query(t, db, "SELECT id,slug,body FROM docs ORDER BY id")
	uniqueConstraint(t, db, "INSERT INTO docs VALUES(3,'three','orphan needle'),(4,'one','conflict needle')")
	uniqueConstraint(t, db, "UPDATE docs SET slug=CASE WHEN id=1 THEN 'new' ELSE 'new' END,body='orphan needle'")
	uniqueRows(t, db, "SELECT id,slug,body FROM docs ORDER BY id", before)
	for _, pattern := range []string{"%orphan%", "%alpha%", "%beta%"} {
		p, _, _ := gramCompare(t, db, "body LIKE ?", pattern)
		if p.Method != "grams13" {
			t.Fatalf("undo check did not use the grams participant: %+v", p)
		}
	}
	if err := db.CheckExtensionIndex(context.Background(), "body_grams"); err != nil {
		t.Fatal(err)
	}
	checkPass(t, db)
}
