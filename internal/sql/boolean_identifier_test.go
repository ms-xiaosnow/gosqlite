package sql

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// Parser and scalar evaluation only; no files, descriptors, or fault injection.
func TestBooleanIdentifierASTPrecedence(t *testing.T) {
	for _, tc := range []struct {
		text string
		want Expr
	}{
		{"true", &Literal{Value: int64(1), Boolean: true}},
		{"FaLsE", &Literal{Value: int64(0), Boolean: true}},
		{`"true"`, &Column{Name: "true"}},
		{"`false`", &Column{Name: "false"}},
		{"[TRUE]", &Column{Name: "TRUE"}},
		{"'true'", &Literal{Value: "true"}},
		{"NULL", &Literal{}},
		{`"NULL"`, &Column{Name: "NULL"}},
		{"true.false", &Column{Table: "true", Name: "false"}},
		{"FaLsE.TrUe", &Column{Table: "FaLsE", Name: "TrUe"}},
		{`"true".false`, &Column{Table: "true", Name: "false"}},
		{"true.*", &Star{Table: "true"}},
		{"true()", &Call{Name: "true"}},
		{"FaLsE(TRUE, null)", &Call{Name: "false", Args: []Expr{&Literal{Value: int64(1), Boolean: true}, &Literal{}}}},
		{`"true"(false)`, &Call{Name: "true", Args: []Expr{&Literal{Value: int64(0), Boolean: true}}}},
	} {
		got, err := ParseExpr(tc.text)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %#v (%v), want %#v", tc.text, got, err, tc.want)
		}
	}
	calls := 0
	ctx := &EvalContext{Functions: map[string]Function{"true": func(a []any) (any, error) { calls++; return a[0], nil }, "false": func(a []any) (any, error) { calls++; return a[0], nil }}}
	if got := evalText(t, "TrUe(FaLsE(2 IS TRUE))", ctx); got != int64(1) || calls != 2 {
		t.Fatal(got, calls)
	}
	for _, tc := range []struct {
		q    string
		want any
	}{{"2 IS TRUE", int64(1)}, {"2 IS +TRUE", int64(0)}, {"NULL IS FALSE", int64(0)}, {"NULL IS NOT TRUE", int64(1)}, {"CASE WHEN TRUE THEN FALSE ELSE NULL END", int64(0)}} {
		if got := evalText(t, tc.q, nil); !reflect.DeepEqual(got, tc.want) {
			t.Fatal(tc.q, got)
		}
	}
}

func TestBooleanIdentifierGrammar(t *testing.T) {
	for _, q := range []string{
		"SELECT 0 AS true,1 false FROM true false WHERE true OR false ORDER BY false",
		"SELECT true.*,false.true FROM true AS true LEFT JOIN false AS false ON true.false IS false.true",
		"CREATE TABLE true(false INTEGER PRIMARY KEY, true INTEGER DEFAULT FALSE)",
		"CREATE TABLE FaLsE(TrUe TEXT DEFAULT NULL, FaLsE BLOB)",
		"CREATE INDEX false ON true(false,true DESC)",
		"REINDEX false", "INSERT INTO true(false,true) VALUES(?1,TRUE)",
		"UPDATE true SET false=?1,true=FALSE WHERE true IS TRUE", "DELETE FROM false WHERE true IS FALSE",
		"SAVEPOINT true", "ROLLBACK TO false", "RELEASE true",
	} {
		if _, err := Parse(q); err != nil {
			t.Error(q, err)
		}
	}
	s, err := Parse("SELECT ?2 AS TrUe, :false false, ? AS z FROM false true WHERE true.false=?1 ORDER BY TrUe")
	if err != nil {
		t.Fatal(err)
	}
	sel := s.(*Select)
	if sel.From != "false" || sel.Alias != "true" || sel.Columns[0].Alias != "TrUe" || sel.Columns[1].Alias != "false" || s.ParameterCount() != 4 {
		t.Fatalf("%#v", s)
	}
	want := []Parameter{{Index: 1, Name: "?1"}, {Index: 2, Name: "?2"}, {Index: 3, Name: ":false"}, {Index: 4}}
	if !reflect.DeepEqual(s.Parameters(), want) {
		t.Fatalf("parameters: %#v", s.Parameters())
	}
}

func TestBooleanIdentifierRetainedRejections(t *testing.T) {
	// Reserved keywords and unsupported clauses retain structured rejection.
	for _, q := range []string{
		"SELECT 1 AS NULL", "SELECT 1 AS SELECT", "SELECT 1 AS WHERE", "SELECT 1 AS JOIN",
		"CREATE TABLE NULL(x)", "CREATE TABLE true(NULL INTEGER)",
		"SELECT NULL()", "SELECT NULL.true", "SELECT true.NULL",
		"SELECT DISTINCT true FROM false", "SELECT true FROM false GROUP BY true",
		"SELECT true FROM false RIGHT JOIN t ON true", "SELECT true FROM false NATURAL JOIN t",
		"SELECT true FROM false JOIN t USING(true)", "SELECT true FROM main.false",
		"SELECT true OVER()", "SELECT false FILTER(WHERE true)",
		"UPDATE true SET false=1 RETURNING true", "CREATE TABLE true(false INT) STRICT",
	} {
		_, err := Parse(q)
		var u *ErrUnsupported
		if !errors.As(err, &u) {
			t.Errorf("%s: expected structured unsupported, got %v", q, err)
		}
	}
	for _, q := range []string{"SELECT 1 AS", "SELECT true.", "SELECT true(,)", "SELECT false(1,)", "SELECT true FROM false AS", "CREATE TABLE true(true INT,TRUE INT)", "UPDATE true SET true=1,TRUE=2", "INSERT INTO true(true,TRUE) VALUES(1,2)"} {
		if _, err := Parse(q); err == nil {
			t.Error("accepted malformed SQL", q)
		}
	}
	if _, err := ParseExpr("true(" + strings.Repeat("(", maxExpressionDepth) + "0" + strings.Repeat(")", maxExpressionDepth) + ")"); err == nil {
		t.Fatal("depth cap bypassed")
	}
}
