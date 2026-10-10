//go:build oracle

package testkit_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Only intact SQL-created fixtures and ordinary completed CLI reads. No
// corruption, missing selected files, process termination or injected I/O.
func aggregateOracleFixture(t *testing.T, shared bool) (*sqlite.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "aggregates.db")
	confCLI(t, path, `CREATE TABLE a(id INTEGER PRIMARY KEY,x,n TEXT,r REAL);
INSERT INTO a VALUES(1,1,'one',1),(2,2,'two',2),(3,NULL,'three',NULL),(4,'3.0','four',4),(5,x'34','five',5),(6,'word','six',6);
CREATE TABLE b(id INTEGER PRIMARY KEY,x,n TEXT);
INSERT INTO b VALUES(11,1,'first'),(12,1,'second'),(13,3,'third');
CREATE TABLE empty_source(x);
CREATE TABLE null_source(x);
INSERT INTO null_source VALUES(NULL),(NULL);
CREATE TABLE mixed(x);
INSERT INTO mixed VALUES(NULL),(7),(2.5),('3'),('4.0'),('5e0'),('word'),('123abc'),(x'3132'),(x''),(''),(' 7 '),('0x10'),('1_0');
CREATE TABLE overpos(x); INSERT INTO overpos VALUES(9223372036854775807),(1);
CREATE TABLE overneg(x); INSERT INTO overneg VALUES(-9223372036854775808),(-1);
CREATE TABLE extremes(x); INSERT INTO extremes VALUES(9223372036854775807),(-9223372036854775808),(1);
CREATE TABLE latefloat(x); INSERT INTO latefloat VALUES(9223372036854775807),(1),(0.0);
CREATE TABLE compensated(x); INSERT INTO compensated VALUES(1e16),(1.0),(-1e16);
CREATE TABLE realoverflow(x); INSERT INTO realoverflow VALUES(1e308),(1e308);
CREATE TABLE opposed(x); INSERT INTO opposed VALUES(9e999),(-9e999);
CREATE TABLE texts(x); INSERT INTO texts VALUES('a'),('Z'),('é'),(''),(NULL);
CREATE TABLE blobs(x); INSERT INTO blobs VALUES(x'00ff'),(x''),(x'ff'),(NULL);
CREATE TABLE ties(x); INSERT INTO ties VALUES(1.0),(1),(NULL);
CREATE TABLE boolnames("true" INTEGER,"false" INTEGER); INSERT INTO boolnames VALUES(8,4),(3,9);`, false)
	db, err := sqlite.Open(path, sqlite.Options{WALShared: shared, PageSize: 512})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db, path
}
func TestGlobalAggregateOracleValuesAndClasses(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared_%t", shared), func(t *testing.T) {
			db, path := aggregateOracleFixture(t, shared)
			cases := []confCase{
				{query: "SELECT count(*) FROM a"}, {query: "SELECT sum(x) FROM a"},
				{query: "SELECT count(*),count(NULL),sum(2),total(2),avg(2),min('x'),max(x'01')"},
				{query: "SELECT count(),count(1),count(NULL) FROM a"},
				{query: "SELECT count(*) AS n FROM a WHERE n>0 LIMIT 0"},
				{query: "SELECT count(*) AS cnt FROM a ORDER BY cnt DESC"},
				{query: "SELECT sum(x) AS amount,total(x) AS amount FROM a ORDER BY amount"},
				{query: "SELECT sum(x) AS \"true\" FROM a ORDER BY true"},
				{query: "SELECT sum(true),sum(false),min(true),max(false) FROM boolnames"},
				{query: "SELECT count(*),sum(a.x),count(b.id),max(b.n) FROM a LEFT JOIN b ON a.x=b.x"},
				{query: "SELECT count(*),sum(a.x),count(b.id) FROM a LEFT JOIN b ON 0"},
				{query: "SELECT count(*),sum(a.x),count(b.id) FROM a,b WHERE a.x=b.x"},
				{query: "SELECT count(*),sum(a.x),count(b.id) FROM a,b WHERE 0"},
				{query: "SELECT count(*),sum(b.x) FROM empty_source a LEFT JOIN b ON 1"},
				{query: "SELECT count(abs(-9223372036854775808)),sum(abs(-9223372036854775808)) FROM a WHERE 0"},
				{query: "SELECT count(abs(-9223372036854775808)),sum(abs(-9223372036854775808)) FROM empty_source"},
				{query: "SELECT count(abs(-9223372036854775808)) FROM a LIMIT 0"},
				{query: "SELECT CASE WHEN 0 THEN sum(x) ELSE 9 END,coalesce(avg(x),0),typeof(sum(x)),sum(x)+1 FROM a"},
				{query: "SELECT sum(x)+abs(-9223372036854775808) FROM a LIMIT 0"},
				{query: "SELECT sum(x) AS amount FROM a ORDER BY 1 DESC NULLS LAST LIMIT 1 OFFSET 0"},
				{query: "SELECT sum(x) AS amount FROM a LIMIT 1 OFFSET 1"},
				{query: "SELECT sum(x+?) AS amount FROM a WHERE id>? ORDER BY amount LIMIT ?", oracleQuery: "SELECT sum(x+10) AS amount FROM a WHERE id>1 ORDER BY amount LIMIT 1", args: []any{10, 1, 1}},
			}

			for _, table := range []string{"a", "empty_source", "null_source", "mixed", "extremes", "latefloat", "compensated", "realoverflow", "opposed", "texts", "blobs", "ties"} {
				var cols []string
				for _, fn := range []string{"count", "sum", "total", "avg", "min", "max"} {
					cols = append(cols, fn+"(x)", "typeof("+fn+"(x))")
				}
				cases = append(cases, confCase{query: "SELECT " + strings.Join(cols, ",") + " FROM " + table})
			}
			for _, table := range []string{"overpos", "overneg"} {
				cases = append(cases, confCase{query: "SELECT total(x),avg(x),min(x),max(x),typeof(total(x)),typeof(avg(x)) FROM " + table})
			}
			for _, q := range []string{
				"SELECT sum(CASE WHEN id=1 THEN '1' ELSE '2' END),typeof(sum(CASE WHEN id=1 THEN '1' ELSE '2' END)) FROM a",
				"SELECT sum(CASE WHEN id=1 THEN 9223372036854775807 WHEN id=2 THEN 1 ELSE 'word' END) FROM a",
				"SELECT sum(CASE WHEN id=1 THEN 9223372036854775807 WHEN id=2 THEN 1 ELSE x'30' END) FROM a",
				"SELECT min(CAST(x AS TEXT)),max(CAST(x AS BLOB)),typeof(min(CAST(x AS TEXT))) FROM a",
				"SELECT count(*) AS n,1 AS keep FROM a WHERE keep ORDER BY n",
				"SELECT count(*) FROM a,b ON 0 AND abs(-9223372036854775808)",
			} {
				cases = append(cases, confCase{query: q})
			}
			for _, tc := range cases {
				t.Run(tc.query, func(t *testing.T) {
					q := tc.oracleQuery
					if q == "" {
						q = tc.query
					}
					want := confExpected(t, path, q)
					got := confNative(t, db, tc.query, tc.args...)
					confCompare(t, want, got)
				})
			}
		})
	}
}
func TestGlobalAggregateOracleErrors(t *testing.T) {
	db, path := aggregateOracleFixture(t, false)
	for _, tc := range []struct{ q, category string }{
		{"SELECT sum(x) FROM overpos", "integer overflow"}, {"SELECT sum(x) FROM overneg", "integer overflow"},
		{"SELECT sum(abs(-9223372036854775808)) FROM a", "integer overflow"},
		{"SELECT count(*)+abs(-9223372036854775808) FROM empty_source", "integer overflow"},
		{"SELECT count(*) FROM a WHERE abs(-9223372036854775808)", "integer overflow"},
		{"SELECT sum(abs(-9223372036854775808)) FROM a WHERE 0 OR abs(-9223372036854775808)", "integer overflow"},
		{"SELECT sum(missing) FROM a LIMIT 0", "no such column"},
		{"SELECT sum(abs()) FROM empty_source LIMIT 0", "wrong number of arguments"},
		{"SELECT count(1,2) FROM a LIMIT 0", "wrong number of arguments"},
		{"SELECT sum(sum(x)) FROM a LIMIT 0", "misuse of aggregate"},
		{"SELECT count(*) FROM a WHERE sum(x)>0 LIMIT 0", "misuse of aggregate"},
		{"SELECT count(*) AS cnt FROM a WHERE cnt>0 LIMIT 0", "misuse of aggregate"},
		{"SELECT count(*) FROM a,b ON count(*)>0 LIMIT 0", "misuse of aggregate"},
		{"SELECT count(*) FROM a LIMIT sum(1)", "misuse of aggregate"},
	} {
		t.Run(tc.q, func(t *testing.T) {
			out, err := exec.Command(confOracle(t), "-readonly", "-batch", "-bail", path, tc.q).CombinedOutput()
			if err == nil || !strings.Contains(string(out), tc.category) {
				t.Fatalf("oracle want %s, got %v: %s", tc.category, err, out)
			}
			r, err := db.Query(context.Background(), tc.q)
			if err == nil {
				for r.Next() {
				}
				err = r.Err()
				r.Close()
			}
			var detail *sqlite.Error
			if !errors.As(err, &detail) || detail.Code != sqlite.CodeError || !strings.Contains(err.Error(), tc.category) {
				t.Fatalf("native want SQLITE_ERROR/%s: %v", tc.category, err)
			}
		})
	}
}

// Exact formerly unsupported SQL is compared positively to the pinned oracle,
// with complete labels, values and storage classes, before retiring negatives.
func TestGlobalAggregateOraclePromotedCapabilities(t *testing.T) {
	confRun(t, filepath.Join("..", "..", "testdata", "readonly", "page4096.db"), []confCase{{name: "samples_count", query: "SELECT count(*) FROM samples"}})
	confRun(t, confFresh(t), []confCase{{name: "edge_count", query: "SELECT count(*) FROM edge"}})
	path := filepath.Join(t.TempDir(), "promoted.db")
	confCLI(t, path, "CREATE TABLE t(x); INSERT INTO t VALUES(NULL),(1),(2);", false)
	confRun(t, path, []confCase{{name: "t_count", query: "SELECT count(*) FROM t"}, {name: "t_sum", query: "SELECT sum(x) FROM t"}})
}

func TestGlobalAggregateOracleTypedColumnBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "typed.db")
	// Ordinary SQL may store values that violate this engine's extra VECTOR
	// declaration contract. This is not an invalid database or physical mutation.
	confCLI(t, path, "CREATE TABLE typed(id INTEGER PRIMARY KEY,v VECTOR(2)); INSERT INTO typed VALUES(1,'[1,2]'),(2,7);", false)
	db, err := sqlite.Open(path, sqlite.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	confCompare(t, confExpected(t, path, "SELECT count(*) FROM typed"), confNative(t, db, "SELECT count(*) FROM typed"))
	for _, fn := range []string{"count", "sum", "total", "avg", "min", "max"} {
		q := "SELECT " + fn + "(v) FROM typed"
		r, err := db.Query(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		if r.Next() || !errors.Is(r.Err(), sqlite.ErrMismatch) {
			t.Fatal(q, r.Values(), r.Err())
		}
		r.Close()
		q += " WHERE id=1"
		confCompare(t, confExpected(t, path, q), confNative(t, db, q))
	}
}

// Deterministic mixed-class expression sequences exercise real arithmetic and
// result classes. Only SQL CASE/literals and ordinary read-only oracle calls.
func TestGlobalAggregateOracleSeededSequences(t *testing.T) {
	db, path := aggregateOracleFixture(t, false)
	random := rand.New(rand.NewSource(3540000))
	atoms := []string{"NULL", "0", "1", "-1", "7", "2.5", "-0.25", "1e16", "-1e16", "'3'", "'4.0'", "'5e0'", "' 6 '", "'12suffix'", "'1e'", "'1.e'", "'1e+'", "'word'", "'0x10'", "x'3132'", "x''", "x'00ff'"}
	for trial := 0; trial < 96; trial++ {
		input := "CASE id"
		for id := 1; id <= 6; id++ {
			input += fmt.Sprintf(" WHEN %d THEN %s", id, atoms[random.Intn(len(atoms))])
		}
		input += " END"
		var columns []string
		for _, fn := range []string{"count", "sum", "total", "avg", "min", "max"} {
			call := fn + "(" + input + ")"
			columns = append(columns, call, "typeof("+call+")")
		}
		q := "SELECT " + strings.Join(columns, ",") + " FROM a"
		t.Run(fmt.Sprint(trial), func(t *testing.T) { confCompare(t, confExpected(t, path, q), confNative(t, db, q)) })
	}
}

func TestGlobalAggregateOracleOffsetErrorSuppression(t *testing.T) {
	db, path := aggregateOracleFixture(t, false)
	for _, q := range []string{
		"SELECT count(*)+abs(-9223372036854775808) FROM a LIMIT 1 OFFSET 1",
		"SELECT count(*)+abs(-9223372036854775808) AS cnt FROM a ORDER BY cnt LIMIT -1 OFFSET 2",
		"SELECT count(*)+abs(-9223372036854775808) FROM empty_source LIMIT 1 OFFSET 1",
	} {
		t.Run(q, func(t *testing.T) { confCompare(t, confExpected(t, path, q), confNative(t, db, q)) })
	}
	for _, q := range []string{
		"SELECT sum(x) FROM overpos LIMIT 1 OFFSET 1",
		"SELECT sum(abs(-9223372036854775808)) FROM a LIMIT 1 OFFSET 1",
	} {
		t.Run(q, func(t *testing.T) {
			out, err := exec.Command(confOracle(t), "-readonly", "-batch", "-bail", path, q).CombinedOutput()
			if err == nil || !strings.Contains(string(out), "integer overflow") {
				t.Fatal(err, string(out))
			}
			r, err := db.Query(context.Background(), q)
			if err == nil {
				for r.Next() {
				}
				err = r.Err()
				r.Close()
			}
			if err == nil || !strings.Contains(err.Error(), "integer overflow") {
				t.Fatal(err)
			}
		})
	}
}

func TestGlobalAggregateOracleNULNumericStorageClasses(t *testing.T) {
	db, path := aggregateOracleFixture(t, false)
	for _, atom := range []string{"CAST(x'310032' AS TEXT)", "CAST(x'0031' AS TEXT)", "CAST(x'312e300078' AS TEXT)", "CAST(x'316162630030' AS TEXT)", "CAST(x'2031200020' AS TEXT)", "x'310032'", "x'0031'", "x'312e300078'"} {
		var cols []string
		for _, fn := range []string{"count", "sum", "total", "avg"} {
			expr := fn + "(" + atom + ")"
			cols = append(cols, expr, "typeof("+expr+")")
		}
		q := "SELECT " + strings.Join(cols, ",")
		t.Run(atom, func(t *testing.T) { confCompare(t, confExpected(t, path, q), confNative(t, db, q)) })
	}
	// Integer-valued NUL-terminated TEXT does not clear prior integer overflow.
	q := "SELECT sum(CASE WHEN id=1 THEN 9223372036854775807 WHEN id=2 THEN 1 ELSE CAST(x'300031' AS TEXT) END) FROM a"
	out, err := exec.Command(confOracle(t), "-batch", "-bail", "-readonly", path, q).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "integer overflow") {
		t.Fatal(err, string(out))
	}
	r, err := db.Query(context.Background(), q)
	if err == nil {
		for r.Next() {
		}
		err = r.Err()
		r.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "integer overflow") {
		t.Fatal(err)
	}
}
