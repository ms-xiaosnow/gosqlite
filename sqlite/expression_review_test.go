package sqlite

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"testing"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// Only ordinary CREATE/INSERT/query/UPDATE/rollback on intact temp databases.
// reviewVectorDB/Exec/Query are the source-audited normal lifecycle helpers.
func TestExpressionReviewPhysicalUntypedAliasIdentity(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE e(id INTEGER PRIMARY KEY,x,h HALFVEC(3))")
	reviewVectorExec(t, db, "INSERT INTO e VALUES(1,'[3,0,-4]','[3,0,-4]')")
	for _, q := range []string{
		"SELECT CASE WHEN 1 THEN h END AS x FROM e WHERE vector_dims(x)=3",
		"SELECT CAST(h AS BLOB) AS x FROM e WHERE vector_dims(x)=3",
		"SELECT CASE WHEN 1 THEN h END AS x FROM e ORDER BY vector_dims(x) NULLS LAST",
	} {
		rows, err := db.Query(context.Background(), q)
		if err != nil {
			t.Errorf("%s: %v", q, err)
			continue
		}
		if !rows.Next() {
			t.Errorf("%s: physical x must be dense text; got %v", q, rows.Err())
		}
		rows.Close()
	}
}

func TestExpressionReviewBoundTreeIsolation(t *testing.T) {
	stmt, err := sqlast.Parse("SELECT CASE TRUE WHEN FALSE THEN CAST(TRUE AS TEXT) ELSE TRUE IN(FALSE) END")
	if err != nil {
		t.Fatal(err)
	}
	e := stmt.(*sqlast.Select).Columns[0].Expr
	trueOnly := &table{index: map[string]int{"true": 0}}
	falseOnly := &table{index: map[string]int{"false": 0}}
	a, b := bindBooleans(e, trueOnly), bindBooleans(e, falseOnly)
	counts := func(e sqlast.Expr) (literal int, columns []string) {
		sqlast.Walk(e, func(e sqlast.Expr) error {
			switch x := e.(type) {
			case *sqlast.Literal:
				if x.Boolean {
					literal++
				}
			case *sqlast.Column:
				columns = append(columns, x.Name)
			}
			return nil
		})
		return
	}
	l, c := counts(e)
	if l != 5 || len(c) != 0 {
		t.Fatal(l, c)
	}
	l, c = counts(a)
	if l != 2 || !reflect.DeepEqual(c, []string{"true", "true", "true"}) {
		t.Fatal(l, c)
	}
	l, c = counts(b)
	if l != 3 || !reflect.DeepEqual(c, []string{"false", "false"}) {
		t.Fatal(l, c)
	}
}

func TestExpressionReviewStatementUndoAndDefaults(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE e(id INTEGER PRIMARY KEY,n INTEGER DEFAULT (CAST(CASE WHEN 1 IN(2,NULL) THEN 9 ELSE '12' END AS INTEGER)))")
	reviewVectorExec(t, db, "INSERT INTO e(id) VALUES(1),(2)")
	reviewVectorExec(t, db, "BEGIN")
	reviewVectorExec(t, db, "UPDATE e SET n=13 WHERE id=1")
	if _, err := db.Exec(context.Background(), "UPDATE e SET n=CASE WHEN id=1 THEN 99 ELSE abs(-9223372036854775808) END"); err == nil {
		t.Fatal("missing scalar error")
	}
	got, _, _ := reviewVectorQuery(t, db, "SELECT id,n FROM e ORDER BY id")
	if !reflect.DeepEqual(got, [][]any{{int64(1), int64(13)}, {int64(2), int64(12)}}) {
		t.Fatal(got)
	}
	reviewVectorExec(t, db, "ROLLBACK")
	got, _, _ = reviewVectorQuery(t, db, "SELECT n FROM e ORDER BY id")
	if !reflect.DeepEqual(got, [][]any{{int64(12)}, {int64(12)}}) {
		t.Fatal(got)
	}
}

func TestExpressionReviewRepresentationAliasFamilies(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE e(id INTEGER PRIMARY KEY,v VECTOR(3),h HALFVEC(3),b BIT(3),s SPARSEVEC(3))")
	reviewVectorExec(t, db, "INSERT INTO e VALUES(1,'[3,0,-4]','[3,0,-4]','101','{1:3,3:-4}/3')")
	for _, c := range []struct{ col, fn string }{{"v", "vector_dims"}, {"h", "halfvec_dims"}, {"b", "bit_dims"}, {"s", "sparsevec_dims"}} {
		q := fmt.Sprintf("SELECT %s,CASE WHEN id=1 THEN CAST(%s AS BLOB) END AS chosen FROM e WHERE %s(chosen)=3 ORDER BY CASE WHEN %s(chosen)=3 THEN id ELSE 0 END NULLS LAST", c.col, c.col, c.fn, c.fn)
		got, _, _ := reviewVectorQuery(t, db, q)
		if len(got) != 1 || !bytes.Equal(got[0][0].([]byte), got[0][1].([]byte)) {
			t.Fatal(q, got)
		}
	}
	for _, other := range []string{"h", "b", "s"} {
		q := fmt.Sprintf("SELECT CASE WHEN 1 THEN v ELSE %s END AS chosen FROM e WHERE vector_dims(chosen)=3", other)
		rows, err := db.Query(context.Background(), q)
		if err != nil {
			continue
		}
		if rows.Next() || rows.Err() == nil {
			t.Errorf("mixed representation alias accepted: %s", q)
		}
		rows.Close()
	}
}
