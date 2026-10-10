package sql

import (
	"errors"
	"strings"
	"testing"
)

// Pure parser input only; no storage or external processes.
func TestRelationalParserTreeAndParameters(t *testing.T) {
	stmt, err := Parse("SELECT x.*, y.n AS label FROM a AS x NOT INDEXED LEFT OUTER JOIN b y ON y.id=x.id AND y.n=?1 CROSS JOIN c z ON z.id=?2 WHERE x.id>?3 ORDER BY label LIMIT ?4")
	if err != nil {
		t.Fatal(err)
	}
	s := stmt.(*Select)
	if s.Relation == nil || s.From != "" || s.ParameterCount() != 4 {
		t.Fatal(s)
	}
	j := s.Relation.(*JoinRelation)
	if j.Kind != "CROSS" {
		t.Fatal(j)
	}
	l := j.Left.(*JoinRelation)
	if l.Kind != "LEFT" || !l.Left.(*TableRelation).NotIndexed {
		t.Fatal(l)
	}
	one, err := Parse("SELECT * FROM a x NOT INDEXED")
	if err != nil {
		t.Fatal(err)
	}
	a := one.(*Select)
	if a.Relation != nil || a.From != "a" || a.Alias != "x" || !a.NotIndexed {
		t.Fatal(a)
	}
}
func TestRelationalParserFailClosed(t *testing.T) {
	for _, q := range []string{
		"SELECT * FROM a NATURAL JOIN b", "SELECT * FROM a JOIN b USING(id)",
		"SELECT * FROM a RIGHT JOIN b ON 1", "SELECT * FROM a FULL JOIN b ON 1",
		"SELECT * FROM (a JOIN b ON 1)", "SELECT * FROM (SELECT 1) x", "SELECT * FROM main.a JOIN b ON 1",
		"SELECT * FROM a INDEXED BY ai JOIN b ON 1", "SELECT * FROM a JOIN b ON 1 GROUP BY a.id",
		"SELECT * FROM a JOIN b ON 1 UNION SELECT 1", "SELECT * FROM a JOIN b ON EXISTS(SELECT 1)",
	} {
		if _, err := Parse(q); err == nil {
			t.Fatal("accepted", q)
		}
	}
	q := "SELECT 1 FROM a" + strings.Repeat(" CROSS JOIN a", MaxRelations)
	var unsupported *ErrUnsupported
	if _, err := Parse(q); !errors.As(err, &unsupported) {
		t.Fatal(err)
	}
}
