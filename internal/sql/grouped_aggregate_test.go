package sql

import (
	"errors"
	"strings"
	"testing"
)

// Pure parser/bounded tree rewrites, no descriptors or database mutations.
func TestGroupedAggregateGrammarAndParameters(t *testing.T) {
	for _, q := range []string{"SELECT x FROM t GROUP BY x", "SELECT true FROM false GROUP BY true", "SELECT sum(x) FROM t GROUP BY x", "SELECT count(*) FROM t HAVING count(*)>0"} {
		s, err := Parse(q)
		if err != nil {
			t.Fatal(q, err)
		}
		if _, ok := s.(*Select); !ok {
			t.Fatal(s)
		}
	}
	stmt, err := Parse("SELECT x+?1 AS k,count(*) FROM a,b WHERE a.id=:id GROUP BY k,?2 HAVING count(*)>?3 ORDER BY k LIMIT ?4 OFFSET ?5")
	if err != nil {
		t.Fatal(err)
	}
	s := stmt.(*Select)
	if len(s.GroupBy) != 2 || s.Having == nil || s.ParameterCount() != 5 {
		t.Fatal(s)
	}
	for _, q := range []string{"SELECT x FROM t GROUP BY ROLLUP(x)", "SELECT x FROM t GROUP BY x WITH ROLLUP", "SELECT DISTINCT x FROM t GROUP BY x", "SELECT * FROM t GROUP BY x", "SELECT sum(DISTINCT x) FROM t GROUP BY x", "SELECT sum(x) OVER() FROM t GROUP BY x", "SELECT x FROM t GROUP BY x HAVING"} {
		if _, err := Parse(q); err == nil {
			t.Fatal("accepted", q)
		}
	}
	_, err = Parse("SELECT 1 GROUP BY " + strings.Repeat("1,", MaxColumns) + "1")
	var unsupported *ErrUnsupported
	if !errors.As(err, &unsupported) {
		t.Fatal(err)
	}
}
func TestGroupedAggregatePrunedRewrite(t *testing.T) {
	e, err := ParseExpr("x+1")
	if err != nil {
		t.Fatal(err)
	}
	visits := 0
	got, err := RewritePrunedChecked(e, func(Expr) error { visits++; return nil }, func(Expr) (Expr, bool, error) { return &Literal{Value: int64(7)}, true, nil })
	if err != nil || visits != 1 || got.(*Literal).Value != int64(7) {
		t.Fatal(got, visits, err)
	}
	stop := errors.New("stop")
	_, err = RewritePrunedChecked(e, func(Expr) error { return stop }, func(Expr) (Expr, bool, error) { t.Fatal("replaced before admission"); return nil, false, nil })
	if !errors.Is(err, stop) {
		t.Fatal(err)
	}
}
