package sqlite_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// All files here are healthy, SQL-created fixtures. These are bounded logical
// constraint/transaction cases, not physical-fault or recovery qualification.
func TestAutomaticUniqueOrderedMetadataAndCoalescing(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	if _, err := db.Schema(ctx); err != nil {
		t.Fatal(err)
	}
	cookie := db.Info().SchemaCookie
	execute(t, db, `CREATE TABLE "Mixed界"(
		id INTEGER PRIMARY KEY CONSTRAINT "id unique" UNIQUE,
		"A" TEXT COLLATE BINARY CONSTRAINT singleton UNIQUE UNIQUE,
		b BLOB,
		CONSTRAINT redundant UNIQUE(a),
		CONSTRAINT pair UNIQUE(a COLLATE BINARY ASC,b),
		UNIQUE("A",b),UNIQUE(b,a),UNIQUE(a,a))`)
	schema, err := db.Schema(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if db.Info().SchemaCookie != cookie+1 {
		t.Fatalf("compound CREATE cookie=%d want %d", db.Info().SchemaCookie, cookie+1)
	}
	if len(schema) != 6 {
		t.Fatalf("one table plus five coalesced indexes: %+v", schema)
	}
	seen := map[string]sqlite.SchemaEntry{}
	roots := map[uint32]bool{}
	for _, entry := range schema {
		if entry.RootPage < 2 || roots[entry.RootPage] {
			t.Fatalf("missing/shared physical root: %+v", entry)
		}
		roots[entry.RootPage] = true
		seen[entry.Name] = entry
	}
	for i := 1; i <= 5; i++ {
		name := fmt.Sprintf("sqlite_autoindex_Mixed界_%d", i)
		entry, ok := seen[name]
		if !ok || entry.Type != "index" || entry.TableName != "Mixed界" || entry.SQL != "" || !entry.SQLNull {
			t.Fatalf("constraint index %s: %+v present=%t", name, entry, ok)
		}
	}
	if entry := seen["Mixed界"]; entry.Type != "table" || entry.SQL == "" || entry.SQLNull {
		t.Fatalf("table SQL metadata: %+v", entry)
	}
	inspection, err := db.Inspect(ctx, sqlite.InspectOptions{})
	if err != nil || !reflect.DeepEqual(inspection.Schema, schema) {
		t.Fatalf("Inspect and Schema disagree: %+v %v", inspection, err)
	}
	uniqueRows(t, db, "SELECT name,sql IS NULL,typeof(sql) FROM sqlite_schema WHERE type='index' ORDER BY name", [][]any{
		{"sqlite_autoindex_Mixed界_1", int64(1), "null"},
		{"sqlite_autoindex_Mixed界_2", int64(1), "null"},
		{"sqlite_autoindex_Mixed界_3", int64(1), "null"},
		{"sqlite_autoindex_Mixed界_4", int64(1), "null"},
		{"sqlite_autoindex_Mixed界_5", int64(1), "null"},
	})
	// Explicit indexes have independent roots and never coalesce with owned ones.
	execute(t, db, `CREATE UNIQUE INDEX explicit_a ON "Mixed界"(a)`)
	execute(t, db, `CREATE INDEX ordinary_b ON "Mixed界"(b)`)
	execute(t, db, `INSERT INTO "Mixed界" VALUES(0,'zero',X'00'),(-7,NULL,X'00'),(-8,NULL,X'00')`)
	uniqueConstraint(t, db, `INSERT INTO "Mixed界" VALUES(1,'zero',X'01')`)
	uniqueRows(t, db, "SELECT sql IS NULL,typeof(sql) FROM sqlite_schema WHERE name='explicit_a'", [][]any{{int64(0), "text"}})
	if r := checkPass(t, db); r.Indexes != 7 || r.TableRows != 3 || r.IndexEntries != 21 {
		t.Fatal(r)
	}
}

func TestAutomaticUniquePublicSchemaJSONCompatibility(t *testing.T) {
	for _, tc := range []struct {
		entry sqlite.SchemaEntry
		want  string
	}{
		{sqlite.SchemaEntry{Type: "index", Name: "i", TableName: "t", SQL: "CREATE INDEX i ON t(k)", RootPage: 3}, `{"Type":"index","Name":"i","TableName":"t","SQL":"CREATE INDEX i ON t(k)","RootPage":3}`},
		{sqlite.SchemaEntry{Type: "index", Name: "sqlite_autoindex_t_1", TableName: "t", RootPage: 4, SQLNull: true}, `{"Type":"index","Name":"sqlite_autoindex_t_1","TableName":"t","SQL":"","RootPage":4,"SQLNull":true}`},
	} {
		got, err := json.Marshal(tc.entry)
		if err != nil || string(got) != tc.want {
			t.Fatalf("SchemaEntry JSON=%s want=%s err=%v", got, tc.want, err)
		}
	}
}

func TestAutomaticUniqueCompositeNullAndStorageClasses(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE composite(id INTEGER PRIMARY KEY,a,b,c,CONSTRAINT triple UNIQUE(a,b,c),UNIQUE(c,b,a))")
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
	execute(t, db, "CREATE TABLE classes(id INTEGER PRIMARY KEY,k UNIQUE,UNIQUE(k,k))")
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
	if r := checkPass(t, db); r.Indexes != 4 || r.TableRows != 22 || r.IndexEntries != 44 {
		t.Fatal(r)
	}
}

func TestAutomaticUniqueAffinityAndExactNumeric(t *testing.T) {
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
			execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k "+tc.affinity+" CONSTRAINT uq_key UNIQUE)")
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
	execute(t, db, "CREATE TABLE exact(id INTEGER PRIMARY KEY,k,UNIQUE(k))")
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

func TestAutomaticUniqueSignedIPKAndDefaults(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY UNIQUE,k TEXT UNIQUE,UNIQUE(id,k))")
	for _, id := range []int64{math.MinInt64, -7, -1, 0, 1, 7, math.MaxInt64} {
		execute(t, db, "INSERT INTO t VALUES(?,?)", id, fmt.Sprint(id))
		execute(t, db, "UPDATE t SET k=k,id=id WHERE id=?", id)
		uniqueConstraint(t, db, "INSERT INTO t VALUES(42,?)", fmt.Sprint(id))
	}
	execute(t, db, "UPDATE t SET id=2 WHERE id=0")
	execute(t, db, "UPDATE t SET id=0 WHERE id=2")
	execute(t, db, "UPDATE t SET id=-8 WHERE id=-7")
	execute(t, db, "UPDATE t SET id=-7 WHERE id=-8")
	uniqueConstraint(t, db, "UPDATE t SET k='0' WHERE id=-7")
	uniqueConstraint(t, db, "UPDATE t SET k='-7' WHERE id=0")
	execute(t, db, "DELETE FROM t WHERE id=0")
	execute(t, db, "INSERT INTO t VALUES(0,'0')")
	uniqueRows(t, db, "SELECT id,k FROM t WHERE id IN (-7,0) ORDER BY id", [][]any{{int64(-7), "-7"}, {int64(0), "0"}})
	if r := checkPass(t, db); r.Indexes != 3 || r.IndexEntries != 21 {
		t.Fatal(r)
	}
	execute(t, db, "CREATE TABLE defaults(id INTEGER PRIMARY KEY,k TEXT DEFAULT 'default' UNIQUE,n NUMERIC DEFAULT '01',UNIQUE(n,k))")
	execute(t, db, "INSERT INTO defaults DEFAULT VALUES")
	uniqueConstraint(t, db, "INSERT INTO defaults DEFAULT VALUES")
	execute(t, db, "INSERT INTO defaults(k) VALUES(NULL),(NULL)")
	uniqueRows(t, db, "SELECT id,k,n,typeof(n) FROM defaults ORDER BY id", [][]any{
		{int64(1), "default", int64(1), "integer"}, {int64(2), nil, int64(1), "integer"}, {int64(3), nil, int64(1), "integer"},
	})
	checkPass(t, db)
}

func TestAutomaticUniqueImmediateAbortCountersAndCoexistence(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER UNIQUE,note TEXT,UNIQUE(k,note))")
	execute(t, db, "CREATE UNIQUE INDEX explicit_k ON t(k)")
	execute(t, db, "CREATE INDEX ordinary_note ON t(note)")
	execute(t, db, "INSERT INTO t VALUES(1,1,'one'),(2,2,'two'),(3,3,'three')")
	checkCounters(t, db, 3, 3, 3)
	uniqueConstraint(t, db, "INSERT INTO t VALUES(99,1,'first conflict')")
	checkCounters(t, db, 0, 3, 3)
	uniqueConstraint(t, db, "INSERT INTO t VALUES(10,10,'partial'),(11,2,'later conflict')")
	checkCounters(t, db, 0, 3, 10)
	before := query(t, db, "SELECT id,k,note FROM t ORDER BY id")
	uniqueConstraint(t, db, "UPDATE t SET k=4-k")
	uniqueConstraint(t, db, "UPDATE t SET k=CASE WHEN id=1 THEN 4 ELSE 3 END,note='partial' WHERE id<=2")
	uniqueRows(t, db, "SELECT id,k,note FROM t ORDER BY id", before)
	uniqueRows(t, db, "SELECT id FROM t WHERE note='partial' ORDER BY id", [][]any{})
	checkCounters(t, db, 0, 3, 10)
	execute(t, db, "UPDATE t SET k=k,note=note")
	checkCounters(t, db, 3, 6, 10)
	execute(t, db, "DELETE FROM t WHERE id=2")
	execute(t, db, "INSERT INTO t VALUES(20,2,'reused')")
	checkCounters(t, db, 1, 8, 20)
	if r := checkPass(t, db); r.Indexes != 4 || r.IndexEntries != 12 {
		t.Fatal(r)
	}
}

func TestAutomaticUniqueTransactionsDDLUndoAndIfNotExists(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			db, _ := writable(t, sqlite.Options{WALShared: shared})
			execute(t, db, "CREATE TABLE base(id INTEGER PRIMARY KEY,k UNIQUE)")
			execute(t, db, "INSERT INTO base VALUES(1,'base')")
			execute(t, db, "BEGIN")
			execute(t, db, "INSERT INTO base VALUES(2,'kept')")
			execute(t, db, "SAVEPOINT ddl")
			execute(t, db, "CREATE TABLE transient(id INTEGER PRIMARY KEY UNIQUE,k UNIQUE,UNIQUE(id,k))")
			execute(t, db, "INSERT INTO transient VALUES(0,'one')")
			uniqueConstraint(t, db, "INSERT INTO transient VALUES(3,'partial'),(4,'one')")
			uniqueRows(t, db, "SELECT id,k FROM transient", [][]any{{int64(0), "one"}})
			checkPass(t, db)
			execute(t, db, "ROLLBACK TO ddl")
			uniqueRows(t, db, "SELECT name FROM sqlite_schema WHERE tbl_name='transient'", [][]any{})
			// Reusing the name with a different definition must not retain descriptors.
			execute(t, db, "CREATE TABLE transient(id INTEGER PRIMARY KEY,k)")
			execute(t, db, "INSERT INTO transient VALUES(1,'same'),(2,'same')")
			execute(t, db, "RELEASE ddl")
			execute(t, db, "COMMIT")
			uniqueConstraint(t, db, "INSERT INTO base VALUES(3,'kept')")
			execute(t, db, "BEGIN")
			execute(t, db, "CREATE TABLE rolled_back(k UNIQUE)")
			execute(t, db, "DELETE FROM base WHERE id=1")
			execute(t, db, "INSERT INTO base VALUES(10,'base')")
			execute(t, db, "ROLLBACK")
			uniqueRows(t, db, "SELECT name FROM sqlite_schema WHERE tbl_name='rolled_back'", [][]any{})
			uniqueRows(t, db, "SELECT id,k FROM base ORDER BY id", [][]any{{int64(1), "base"}, {int64(2), "kept"}})
			uniqueConstraint(t, db, "INSERT INTO base VALUES(11,'base')")
			execute(t, db, "SAVEPOINT outer_savepoint")
			execute(t, db, "CREATE TABLE released(k UNIQUE)")
			execute(t, db, "INSERT INTO released VALUES('committed')")
			execute(t, db, "RELEASE outer_savepoint")
			uniqueConstraint(t, db, "INSERT INTO released VALUES('committed')")
			before, err := db.Schema(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			info, checked := db.Info(), checkPass(t, db)
			execute(t, db, "CREATE TABLE IF NOT EXISTS transient(id INTEGER PRIMARY KEY UNIQUE,k UNIQUE)")
			execute(t, db, "CREATE TABLE IF NOT EXISTS base(id INTEGER PRIMARY KEY,k,UNIQUE(id,k))")
			after, err := db.Schema(context.Background())
			if err != nil || !reflect.DeepEqual(before, after) || db.Info() != info {
				t.Fatalf("IF NOT EXISTS changed schema/header: %+v %+v %v", before, after, err)
			}
			if got := checkPass(t, db); got != checked {
				t.Fatalf("IF NOT EXISTS changed check result: %+v %+v", checked, got)
			}
			execute(t, db, "INSERT INTO transient VALUES(3,'same')")
			checkPass(t, db)
		})
	}
}

func TestAutomaticUniqueRejectedPreparationAndDDLDoNotMutate(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k UNIQUE)")
	execute(t, db, "INSERT INTO t VALUES(1,'base')")
	before, err := db.Schema(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	info, checked := db.Info(), checkPass(t, db)
	// Unknown columns and expression syntax are logical preparation errors;
	// their diagnostics need not carry the unsupported-feature code.
	for _, q := range []string{"CREATE TABLE bad(a,UNIQUE(missing))", "CREATE TABLE bad(a,UNIQUE(a+1))"} {
		if _, err := db.Exec(context.Background(), q); err == nil {
			t.Fatal("invalid UNIQUE declaration accepted", q)
		}
	}
	for _, q := range []string{
		"CREATE TABLE bad(a UNIQUE,UNIQUE(a DESC))",
		"CREATE TABLE bad(a UNIQUE ON CONFLICT ABORT)",
		"CREATE TABLE bad(a,UNIQUE(a COLLATE NOCASE))",
		"CREATE TABLE bad(a UNIQUE) WITHOUT ROWID",
		"CREATE TABLE bad(a TEXT PRIMARY KEY)",
		"CREATE TABLE bad(a INTEGER PRIMARY KEY DESC)",
		"CREATE TABLE bad(a,PRIMARY KEY(a))",
		"DROP INDEX sqlite_autoindex_t_1",
		"DROP INDEX IF EXISTS sqlite_autoindex_t_1",
		"DROP TABLE t",
		"ALTER TABLE t RENAME TO renamed",
		"ALTER TABLE t ADD COLUMN ordinary TEXT DEFAULT 'literal'",
	} {
		if _, err := db.Exec(context.Background(), q); !errors.Is(err, sqlite.ErrUnsupported) {
			t.Fatalf("%s: want unsupported, got %v", q, err)
		}
		after, err := db.Schema(context.Background())
		if err != nil || !reflect.DeepEqual(before, after) || db.Info() != info {
			t.Fatalf("rejection changed schema/header: %s: %+v %v", q, after, err)
		}
		if got := checkPass(t, db); got != checked {
			t.Fatalf("rejection changed allocation/ownership: %s: %+v %+v", q, checked, got)
		}
	}
	uniqueRows(t, db, "SELECT id,k FROM t", [][]any{{int64(1), "base"}})
	uniqueConstraint(t, db, "INSERT INTO t VALUES(2,'base')")
}

func TestAutomaticUniqueScalarKeysWithTypedColumns(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	for i, declaration := range []string{"VECTOR(2)", "HALFVEC(2)", "BIT(9)", "SPARSEVEC(4)"} {
		name := fmt.Sprintf("typed%d", i)
		execute(t, db, "CREATE TABLE "+name+"(id INTEGER PRIMARY KEY,slug TEXT UNIQUE,v "+declaration+")")
		execute(t, db, "INSERT INTO "+name+"(id,slug) VALUES(1,'one'),(2,NULL),(3,NULL)")
		uniqueConstraint(t, db, "INSERT INTO "+name+"(id,slug) VALUES(4,'one')")
		for _, q := range []string{
			"CREATE TABLE rejected(v " + declaration + " UNIQUE)",
			"CREATE TABLE rejected(id INTEGER PRIMARY KEY,k,v " + declaration + ",UNIQUE(k,v))",
		} {
			if _, err := db.Exec(context.Background(), q); !errors.Is(err, sqlite.ErrUnsupported) {
				t.Fatal(q, err)
			}
			uniqueRows(t, db, "SELECT name FROM sqlite_schema WHERE tbl_name='rejected'", [][]any{})
		}
	}
	checkPass(t, db)
}

func TestAutomaticUniquePageSizesReopenAndReadOnlyBytes(t *testing.T) {
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("page%d/shared%t", size, shared), func(t *testing.T) {
				opts := sqlite.Options{PageSize: size, WALShared: shared}
				db, path := writable(t, opts)
				execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY UNIQUE,k TEXT,b BLOB,UNIQUE(k COLLATE BINARY ASC,b))")
				execute(t, db, "CREATE INDEX ordinary_b ON t(b)")
				tx, err := db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				for i := 1; i <= 96; i++ {
					txExecute(t, tx, "INSERT INTO t VALUES(?,?,?)", i, fmt.Sprintf("%03d-\x00", i)+strings.Repeat("界", 700), []byte{0, byte(i), 255})
				}
				// This key exceeds the index local-payload bound even at 65536.
				huge := "overflow-" + strings.Repeat("界", 14000)
				txExecute(t, tx, "INSERT INTO t VALUES(100,?,X'AA')", huge)
				if r, err := tx.Check(ctx); err != nil || r.Indexes != 3 || r.TableRows != 97 || r.IndexEntries != 291 {
					t.Fatal(r, err)
				}
				if err = tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				uniqueConstraint(t, db, "INSERT INTO t VALUES(101,?,X'AA')", huge)
				execute(t, db, "UPDATE t SET id=-100 WHERE id=100")
				execute(t, db, "DELETE FROM t WHERE id>=20 AND id<=40")
				execute(t, db, "INSERT INTO t VALUES(200,'reused',X'00')")
				checkPass(t, db)
				if shared {
					if err = db.Checkpoint(ctx); err != nil {
						t.Fatal(err)
					}
				}
				if err = db.Close(); err != nil {
					t.Fatal(err)
				}
				reopened := checkOpen(t, path, opts)
				uniqueConstraint(t, reopened, "INSERT INTO t VALUES(201,?,X'AA')", huge)
				execute(t, reopened, "UPDATE t SET k='after reopen' WHERE id=200")
				execute(t, reopened, "INSERT INTO t VALUES(202,'reused',X'00')")
				uniqueRows(t, reopened, "SELECT k FROM t WHERE id=-100", [][]any{{huge}})
				if r := checkPass(t, reopened); r.TableRows != 78 || r.IndexEntries != 234 {
					t.Fatal(r)
				}
				if shared {
					if err = reopened.Checkpoint(ctx); err != nil {
						t.Fatal(err)
					}
				} else if err = reopened.Close(); err != nil {
					t.Fatal(err)
				}
				// Shared read-only requires this initialized keeper to stay live
				// and idle until the reader closes and the family bytes match.
				before := checkFamily(t, filepath.Dir(path))
				ro := checkOpen(t, path, sqlite.Options{ReadOnly: true, NoCreate: true, WALShared: shared})
				uniqueRows(t, ro, "SELECT count(*) FROM t", [][]any{{int64(78)}})
				if _, err = ro.Schema(ctx); err != nil {
					t.Fatal(err)
				}
				if _, err = ro.Inspect(ctx, sqlite.InspectOptions{}); err != nil {
					t.Fatal(err)
				}
				checkPass(t, ro)
				if err = ro.Close(); err != nil {
					t.Fatal(err)
				}
				if after := checkFamily(t, filepath.Dir(path)); !reflect.DeepEqual(before, after) {
					t.Fatalf("read-only SELECT/Schema/Inspect/Check changed file family: before=%v after=%v", before, after)
				}
				if shared {
					if err = reopened.Close(); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestAutomaticUniquePlannerAndGramsUndo(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER UNIQUE)")
	execute(t, db, "INSERT INTO t VALUES(1,10),(2,20),(3,30)")
	for _, q := range []string{"SELECT id FROM t WHERE k=20 ORDER BY id", "SELECT id FROM t WHERE k>=20 ORDER BY id"} {
		p, s, _ := ordinaryCompare(t, db, q)
		if p.Access != "SCAN" || s.IndexKeysRead != 0 || s.IndexPageReads != 0 {
			t.Fatalf("constraint-only index changed SELECT admission: %+v %+v", p, s)
		}
	}
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,slug TEXT UNIQUE,body TEXT)")
	execute(t, db, "CREATE INDEX body_grams ON docs(body) USING grams13")
	execute(t, db, "INSERT INTO docs VALUES(1,'one','stable alpha'),(2,'two','stable beta')")
	before := query(t, db, "SELECT id,slug,body FROM docs ORDER BY id")
	uniqueConstraint(t, db, "INSERT INTO docs VALUES(3,'three','orphan needle'),(4,'one','conflict needle')")
	uniqueConstraint(t, db, "UPDATE docs SET slug='same',body='orphan needle'")
	uniqueRows(t, db, "SELECT id,slug,body FROM docs ORDER BY id", before)
	for _, pattern := range []string{"%orphan%", "%alpha%", "%beta%"} {
		p, _, _ := gramCompare(t, db, "body LIKE ?", pattern)
		if p.Method != "grams13" {
			t.Fatal(p)
		}
	}
	if err := db.CheckExtensionIndex(context.Background(), "body_grams"); err != nil {
		t.Fatal(err)
	}
	checkPass(t, db)
}
