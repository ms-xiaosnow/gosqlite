package sqlite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// These fixtures use ordinary SQL on intact databases. Cancellation below is
// during read iteration, never physical commit or native I/O failure injection.
func TestSQLExpressionFeaturesDMLBindingDefaults(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE e(id INTEGER PRIMARY KEY, n INTEGER, t TEXT, \"true\" INTEGER, d INTEGER DEFAULT (CASE WHEN 2 BETWEEN 1 AND 3 THEN CAST('7' AS INTEGER) ELSE 0 END))")
	reviewVectorExec(t, db, "INSERT INTO e(id,n,t,\"true\") VALUES(1,12,'12',0),(2,NULL,'2',1),(3,2,'junk',NULL)")
	gotEmpty, _, _ := reviewVectorQuery(t, db, "SELECT absent IN (), absent NOT IN ()")
	if !reflect.DeepEqual(gotEmpty, [][]any{{int64(0), int64(1)}}) {
		t.Fatal(gotEmpty)
	}
	q := "SELECT ALL id, CASE TRUE WHEN 0 THEN 'physical' WHEN 1 THEN 'one' ELSE 'null' END, CAST(t AS INTEGER), n IN ('12','2'), n BETWEEN 1 AND '12', d FROM e ORDER BY id"
	got, _, _ := reviewVectorQuery(t, db, q)
	want := [][]any{{int64(1), "physical", int64(12), int64(1), int64(1), int64(7)}, {int64(2), "one", int64(2), nil, nil, int64(7)}, {int64(3), "null", int64(0), int64(1), int64(1), int64(7)}}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	// Alias affinity follows the expression's CAST, with physical-column
	// precedence unchanged. CASE itself intentionally has no affinity.
	got, _, _ = reviewVectorQuery(t, db, "SELECT CAST(t AS INTEGER) AS a FROM e WHERE a='12'")
	if !reflect.DeepEqual(got, [][]any{{int64(12)}}) {
		t.Fatal(got)
	}
	reviewVectorExec(t, db, "UPDATE e SET n=CASE WHEN id IN (1,3) THEN CAST(t AS NUMERIC) ELSE n END WHERE id BETWEEN 1 AND 3")
	reviewVectorExec(t, db, "DELETE FROM e WHERE CASE id WHEN 2 THEN 1 ELSE 0 END")
	got, _, _ = reviewVectorQuery(t, db, "SELECT id,n FROM e ORDER BY id")
	if !reflect.DeepEqual(got, [][]any{{int64(1), int64(12)}, {int64(3), int64(0)}}) {
		t.Fatal(got)
	}
	// Dead CASE arms still undergo name/function validation before any rows.
	for _, q := range []string{"SELECT CASE WHEN 1 THEN 7 ELSE absent END FROM e WHERE 0", "SELECT 1 FROM e WHERE id IN(CASE WHEN 1 THEN 1 ELSE absent END)", "SELECT 1 FROM e ORDER BY CAST(absent AS INTEGER)", "SELECT 1 FROM e LIMIT CASE WHEN 1 THEN 1 ELSE absent END", "SELECT CASE WHEN 1 THEN 1 ELSE absent_func() END FROM e", "SELECT absent_func() IN ()", "SELECT abs(absent) IN ()", "SELECT (absent LIKE 'x') IN ()"} {
		rows, err := db.Query(context.Background(), q)
		if err == nil {
			rows.Close()
			t.Fatal("unvalidated node", q)
		}
	}
	for _, q := range []string{"CREATE TABLE bad(n INTEGER DEFAULT (CASE WHEN 1 THEN 1 ELSE absent END))", "CREATE TABLE bad(n INTEGER DEFAULT (1 IN(?)))", "CREATE TABLE bad(n INTEGER DEFAULT (CAST(abs(1) AS INTEGER)))"} {
		if _, err := db.Exec(context.Background(), q); err == nil {
			t.Fatal("invalid default", q)
		}
	}
	// Immutable binding checks every new node without changing the parsed tree.
	stmt, err := sqlast.Parse("SELECT CASE TRUE WHEN 0 THEN CAST(TRUE AS INTEGER) ELSE TRUE BETWEEN 0 AND TRUE IN(FALSE) END FROM e")
	if err != nil {
		t.Fatal(err)
	}
	original := stmt.(*sqlast.Select).Columns[0].Expr
	tab := &table{index: map[string]int{"true": 0, "false": 1}}
	bound := bindBooleans(original, tab)
	literalCount, columnCount := 0, 0
	sqlast.Walk(original, func(e sqlast.Expr) error {
		if x, ok := e.(*sqlast.Literal); ok && x.Boolean {
			literalCount++
		}
		return nil
	})
	sqlast.Walk(bound, func(e sqlast.Expr) error {
		if _, ok := e.(*sqlast.Column); ok {
			columnCount++
		}
		return nil
	})
	if literalCount != 5 || columnCount != 5 {
		t.Fatal(literalCount, columnCount)
	}
}

func TestSQLExpressionFeaturesNullOrdering(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE e(id INTEGER PRIMARY KEY,n INTEGER,v VECTOR(1))")
	reviewVectorExec(t, db, "INSERT INTO e VALUES(1,NULL,NULL),(2,3,'[3]'),(3,1,'[1]'),(4,NULL,NULL),(5,1,'[1]')")
	cases := []struct {
		order string
		ids   []int64
	}{{"ASC NULLS FIRST", []int64{1, 4, 3, 5, 2}}, {"ASC NULLS LAST", []int64{3, 5, 2, 1, 4}}, {"DESC NULLS FIRST", []int64{1, 4, 2, 3, 5}}, {"DESC NULLS LAST", []int64{2, 3, 5, 1, 4}}, {"NULLS LAST", []int64{3, 5, 2, 1, 4}}}
	for _, c := range cases {
		for _, key := range []string{"n", "CASE WHEN n IS NULL THEN NULL ELSE n END", "v <-> '[0]'"} {
			q := "SELECT id," + key + " AS distance FROM e ORDER BY distance " + c.order + " LIMIT 5"
			got, p, _ := reviewVectorQuery(t, db, q)
			var ids []int64
			for _, row := range got {
				ids = append(ids, row[0].(int64))
			}
			if !reflect.DeepEqual(ids, c.ids) {
				t.Fatal(q, ids, c.ids)
			}
			if key == "v <-> '[0]'" && c.order == "ASC NULLS FIRST" && p.VectorRanking != "bounded_heap" {
				t.Fatal("lost existing heap", p)
			}
			if key == "v <-> '[0]'" && c.order == "ASC NULLS LAST" && p.VectorRanking == "bounded_heap" {
				t.Fatal("unsupported NULL ordering used heap", p)
			}
		}
	}
	small, _ := reviewVectorDB(t, Options{MaxSortBytes: 100})
	reviewVectorExec(t, small, "CREATE TABLE e(id INTEGER PRIMARY KEY,n INTEGER)")
	reviewVectorExec(t, small, "INSERT INTO e VALUES(1,NULL),(2,3),(3,1)")
	if rows, err := small.Query(context.Background(), "SELECT id FROM e ORDER BY n NULLS LAST"); !errors.Is(err, ErrMemoryLimit) {
		if rows != nil {
			rows.Close()
		}
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	rows, err := db.Query(ctx, "SELECT CASE WHEN id IN(1,2,3) THEN CAST(n AS TEXT) ELSE 'later' END FROM e")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	cancel()
	if rows.Next() || !errors.Is(rows.Err(), context.Canceled) {
		t.Fatal(rows.Err())
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	reviewVectorExec(t, db, "INSERT INTO e VALUES(6,2,'[2]')")
}

func TestSQLExpressionFeaturesVectorStorageIdentity(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE e(id INTEGER PRIMARY KEY,v VECTOR(3),h HALFVEC(3),b BIT(3),s SPARSEVEC(3))")
	reviewVectorExec(t, db, "INSERT INTO e VALUES(1,'[3,0,-4]','[3,0,-4]','101','{1:3,3:-4}/3')")
	for _, c := range []struct{ column, kind, toText string }{{"v", "vector", "vector_to_text"}, {"h", "halfvec", "halfvec_to_text"}, {"b", "bit", "bit_to_text"}, {"s", "sparsevec", "sparsevec_to_text"}} {
		q := fmt.Sprintf("SELECT %s, CASE WHEN 1 THEN %s END, CAST(%s AS BLOB), hex(CAST(%s AS TEXT)), hex(%s), %s(CASE 1 WHEN 1 THEN CAST(%s AS BLOB) END) FROM e", c.column, c.column, c.column, c.column, c.column, c.toText, c.column)
		got, _, _ := reviewVectorQuery(t, db, q)
		row := got[0]
		if !bytes.Equal(row[0].([]byte), row[1].([]byte)) || !bytes.Equal(row[0].([]byte), row[2].([]byte)) || row[3] != row[4] {
			t.Fatal(q, row)
		}
		bad := fmt.Sprintf("SELECT vector_dims(CASE WHEN 1 THEN %s ELSE v END) FROM e", c.column)
		if c.kind != "vector" {
			rows, err := db.Query(context.Background(), bad)
			if err == nil {
				if rows.Next() || rows.Err() == nil {
					rows.Close()
					t.Fatal("mixed identity accepted", bad)
				}
				rows.Close()
			}
		}
	}
	got, _, _ := reviewVectorQuery(t, db, "SELECT CAST(v AS INTEGER),CAST(h AS REAL),CAST(b AS NUMERIC),CAST(s AS VECTOR(3)) FROM e")
	if !reflect.DeepEqual(got, [][]any{{int64(0), float64(0), int64(0), int64(0)}}) {
		t.Fatal(got)
	}
}

func TestSQLExpressionFeaturesConstantInAliases(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE e(id INTEGER PRIMARY KEY,n INTEGER)")
	reviewVectorExec(t, db, "INSERT INTO e VALUES(1,1)")
	for _, q := range []string{
		"SELECT 1 IN(1,2,abs(-9223372036854775808)) FROM e",
		"SELECT 1 AS a FROM e WHERE 1 IN(a,2,abs(-9223372036854775808))",
		"SELECT CAST(1 AS INTEGER) AS a FROM e WHERE 1 IN(a,2,abs(-9223372036854775808))",
	} {
		rows, err := db.Query(context.Background(), q)
		if err == nil {
			for rows.Next() {
			}
			err = rows.Err()
			rows.Close()
		}
		if err == nil {
			t.Fatal("missing eager constant RHS error", q)
		}
	}
	for _, q := range []string{
		"SELECT 1 IN(1,n,abs(-9223372036854775808)) FROM e",
		"SELECT n AS a FROM e WHERE 1 IN(a,2,abs(-9223372036854775808))",
		"SELECT 7 AS n FROM e WHERE 1 IN(n,2,abs(-9223372036854775808))",
	} {
		got, _, _ := reviewVectorQuery(t, db, q)
		if len(got) != 1 {
			t.Fatal(q, got)
		}
	}
}
