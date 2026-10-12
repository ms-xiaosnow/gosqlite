package sql

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestCheckParseOrderedSource(t *testing.T) {
	query := `CREATE TABLE "T"(a CHECK( a < b ) CONSTRAINT "first" CHECK(a IS NOT NULL) UNIQUE,b NUMERIC DEFAULT 4,CONSTRAINT "last" CHECK("T".b >= a),UNIQUE(a,b),CHECK(0 AND a),CHECK(a IN ()))`
	stmt, err := Parse(query)
	if err != nil {
		t.Fatal(err)
	}
	def := stmt.(*CreateTable)
	if def.SQL != query || def.ParameterCount() != 0 || len(def.UniqueConstraints) != 2 || len(def.CheckConstraints) != 5 {
		t.Fatal("lost schema text, order, or mixed constraints", def)
	}
	var names, texts []string
	var named []bool
	for _, check := range def.CheckConstraints {
		names = append(names, check.Name)
		named = append(named, check.Named)
		texts = append(texts, check.Text)
		if query[check.Start:check.End] != check.Text {
			t.Fatal("source span", check)
		}
	}
	if !reflect.DeepEqual(named, []bool{false, true, true, false, false}) || !reflect.DeepEqual(names, []string{"", "first", "last", "", ""}) || !reflect.DeepEqual(texts, []string{" a < b ", "a IS NOT NULL", `"T".b >= a`, "0 AND a", "a IN ()"}) {
		t.Fatal(names, texts)
	}
	if _, ok := def.CheckConstraints[3].Expr.(*Binary); !ok {
		t.Fatal("false-AND source was erased before admission")
	}
	if _, ok := def.CheckConstraints[4].Expr.(*InList); !ok {
		t.Fatal("empty-IN source was erased before admission")
	}
	for _, ordinal := range []int{3, 4} {
		if _, ok := NormalizeCheckSource(def.CheckConstraints[ordinal].Expr).(*Literal); !ok {
			t.Fatal("execution tree did not reproduce parser folding", ordinal)
		}
	}
}

func TestCheckParseRejectsSourceBeforeFolding(t *testing.T) {
	for _, expression := range []string{
		"?", ":x", "0 AND ?", "? IN ()", "CASE WHEN 1 THEN 1 ELSE ? END",
		"abs(a)", "length(a)", "changes()", "random()", "sum(a)",
		"0 AND abs(a)", "abs(a) IN ()", "CASE WHEN 1 THEN 1 ELSE length(a) END",
		"0 AND (a <-> a)", "(a <#> a) IN ()", "(a <=> a) IS NULL",
		"(SELECT 1)", "EXISTS(SELECT 1)", "a IN (SELECT a FROM t)",
		"a COLLATE BINARY", "a.*", "a GLOB 'x'",
		`"".a>0`, `0 AND "".a`, `"".a IN ()`,
		`CASE WHEN 1 THEN 1 ELSE "".a END`,
	} {
		if _, err := Parse("CREATE TABLE t(a CHECK(" + expression + "))"); err == nil {
			t.Fatal("unsupported source erased or accepted", expression)
		}
	}
	for _, query := range []string{
		"CREATE TABLE t(CHECK(1))", "CREATE TABLE t(a,CHECK(1),b)",
		"CREATE TABLE t(a CHECK())", "CREATE TABLE t(a CHECK(1) ON CONFLICT ABORT)",
		"CREATE TABLE t(a,CHECK(1) ON CONFLICT IGNORE)",
		"CREATE TABLE t(a CONSTRAINT c NOT NULL)", "CREATE TABLE t(a CHECK(1)) STRICT",
		"CREATE TABLE t(a CHECK(1)) WITHOUT ROWID", "CREATE TABLE t(a CHECK(1),PRIMARY KEY(a))",
	} {
		if _, err := Parse(query); err == nil {
			t.Fatal("unsupported declaration", query)
		}
	}
	// The qualifier repair is CHECK-scoped. Existing SELECT representation
	// and its broader qualification behavior are not changed by this slice.
	if _, err := Parse(`SELECT "".a FROM t`); err != nil {
		t.Fatal("CHECK-only qualifier guard changed SELECT parsing", err)
	}
	// SELECT retains its existing folds; CHECK mode is scoped to one clause.
	stmt, err := Parse("SELECT missing IN ()")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := stmt.(*Select).Columns[0].Expr.(*Literal); !ok {
		t.Fatal("changed ordinary expression folding")
	}
}

func TestCheckParseSourceBudgets(t *testing.T) {
	for _, count := range []int{MaxCheckConstraints, MaxCheckConstraints + 1} {
		stmt, err := Parse("CREATE TABLE t(a" + strings.Repeat(" CHECK(1)", count) + ")")
		if count > MaxCheckConstraints {
			if err == nil {
				t.Fatal("declaration budget bypass")
			}
		} else if err != nil || len(stmt.(*CreateTable).CheckConstraints) != count {
			t.Fatal("exact declaration bound", err)
		}
	}
	for _, nodes := range []int{MaxCheckNodes, MaxCheckNodes + 1} {
		// IN node + lhs + nodes-2 literal values, all within existing token cap.
		query := "CREATE TABLE t(a CHECK(a IN (" + strings.Repeat("1,", nodes-3) + "1)))"
		stmt, err := Parse(query)
		if nodes > MaxCheckNodes {
			if err == nil {
				t.Fatal("source node budget bypass")
			}
		} else {
			if err != nil {
				t.Fatal("exact node bound", err)
			}
			if got, err := WalkCheckSource(stmt.(*CreateTable).CheckConstraints[0].Expr, nil); err != nil || got != nodes {
				t.Fatal(got, err)
			}
		}
	}
	for _, depth := range []int{128, 129} {
		_, err := Parse("CREATE TABLE t(a CHECK(" + strings.Repeat("NOT ", depth-1) + "a))")
		if (err != nil) != (depth > 128) {
			t.Fatal("depth boundary", depth, err)
		}
	}
	query := "CREATE TABLE t(a CHECK(0 AND a IN (" + strings.Repeat("1,", MaxCheckNodes-3) + "1)))"
	if _, err := Parse(query); err == nil {
		t.Fatal("folded-away source escaped node budget")
	}
}

func TestCheckBranchTruthAndLaziness(t *testing.T) {
	for _, tc := range []struct {
		expr string
		pass bool
	}{
		{"NULL", true}, {"0", false}, {"0.0", false}, {"-0.0", false},
		{"1", true}, {"-0.1", true}, {"0.01", true}, {"''", false}, {"'word'", false},
		{"'1tail'", true}, {"'-0.1tail'", true}, {"'0x1'", false}, {"'0_1'", false},
		{"'1e-2tail'", true}, {"'0e20tail'", false}, {"X'00'", false}, {"X'31'", true},
		{"CAST(X'31007461696C' AS TEXT)", true}, {"0x8000000000000000", true},
		{"9007199254740993", true}, {"NULL AND 0", false}, {"NULL OR 0", true},
		{"NOT NULL", true}, {"CASE WHEN NULL THEN 0 ELSE 1 END", true},
		{"2 BETWEEN 1 AND 3", true}, {"2 NOT BETWEEN 1 AND 3", false},
		{"2 IN (NULL,1)", true}, {"2 IN (1,3)", false}, {"'abc' LIKE 'a%'", true},
		{"NULL OR ('x' LIKE 'x' ESCAPE 'xx')", true},
		{"1.0 OR ('x' LIKE 'x' ESCAPE 'xx')", true},
		{"0.0 AND ('x' LIKE 'x' ESCAPE 'xx')", false},
		{"NOT(NULL AND ('x' LIKE 'x' ESCAPE 'xx'))", true},
	} {
		stmt, err := Parse("SELECT " + tc.expr)
		if err != nil {
			t.Fatal(tc.expr, err)
		}
		got, err := EvalCheck(stmt.(*Select).Columns[0].Expr, &EvalContext{Context: context.Background()})
		if err != nil || got != tc.pass {
			t.Fatal(tc.expr, got, tc.pass, err)
		}
	}
	stmt, err := Parse("SELECT NULL AND ('x' LIKE 'x' ESCAPE 'xx')")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = EvalCheck(stmt.(*Select).Columns[0].Expr, nil); err == nil {
		t.Fatal("required branch error became a passing NULL")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := EvalCheck(&Literal{Value: int64(1)}, &EvalContext{Context: ctx}); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled CHECK passed", err)
	}
}
