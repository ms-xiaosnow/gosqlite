package sqlite

import (
	"context"
	"errors"
	"fmt"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
	"reflect"
	"strings"
	"testing"
)

// All files are intact normal SQL fixtures. Typed-invalid cells below are
// descriptor-free evaluator inputs, never bytes written into a database.
func TestBooleanQueryContextsAndPreflight(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER)", "CREATE TABLE u(y INTEGER)", "CREATE TABLE empty(flag INTEGER)", "INSERT INTO t VALUES(1,0,-9223372036854775808)", "INSERT INTO u VALUES(2)", "CREATE INDEX bool_flag ON t(flag)"} {
		reviewVectorExec(t, db, q)
	}
	const boom = "abs(-9223372036854775808)"
	for _, tc := range []struct {
		q       string
		count   int
		bad     bool
		noFetch bool
	}{
		{"SELECT t.id FROM t,u WHERE 0 AND " + boom, 0, false, true},
		{"SELECT t.id FROM t JOIN u ON 0 AND " + boom, 0, false, true},
		{"SELECT t.id FROM t,u WHERE 1 OR " + boom, 1, false, false},
		{"SELECT t.id FROM t,u WHERE " + boom + " OR 1", 1, false, false},
		{"SELECT id FROM t WHERE flag AND " + boom, 0, true, true},
		{"SELECT flag FROM empty WHERE " + boom, 0, true, true},
		{"SELECT t.id FROM empty JOIN t ON " + boom, 0, true, true},
		{"SELECT t.id FROM t LEFT JOIN empty ON " + boom, 1, false, false},
		{"SELECT id FROM t WHERE flag AND abs(v)", 0, false, false},
		{"SELECT flag AND abs(v) FROM t", 0, true, false},
		{"SELECT 0.0 AND " + boom + " FROM t", 0, true, false},
		{"SELECT CASE WHEN flag AND abs(v) THEN 7 ELSE 9 END FROM t", 1, false, false},
		{"SELECT id FROM t WHERE " + boom + " AND 0", 0, true, true},
		{"SELECT id FROM t WHERE " + boom + " LIMIT 0", 0, false, true},
		{"SELECT flag AS z FROM t WHERE z=99 AND z LIKE 'x' ESCAPE 'ab'", 0, true, true},
		{"SELECT 0 AS z FROM t WHERE z AND " + boom, 0, false, true},
		{"SELECT t.id,u.y FROM t LEFT JOIN u ON " + boom + " OR 1", 0, true, false},
		{"SELECT t.id,u.y FROM t LEFT JOIN u ON 1 OR " + boom, 1, false, false},
	} {
		r, err := db.Query(context.Background(), tc.q)
		if err != nil {
			t.Fatal(tc.q, err)
		}
		n := 0
		for r.Next() {
			n++
		}
		err = r.Err()
		if (err != nil) != tc.bad || n != tc.count {
			t.Errorf("%s: %d rows, %v", tc.q, n, err)
		}
		if tc.noFetch && r.Stats().BaseRowFetches != 0 {
			t.Errorf("preflight fetched rows for %s: %+v", tc.q, r.Stats())
		}
		r.Close()
	}
	// Name/arity/parameter validation precedes runtime branch suppression.
	for _, q := range []string{"SELECT 0 AND abs(missing) FROM t", "SELECT 1 OR missing FROM t", "SELECT 0 AND abs() FROM t", "SELECT 0 AND absent() FROM t", "SELECT 0 AND ?1 FROM t"} {
		r, err := db.Query(context.Background(), q)
		if err == nil {
			r.Close()
			t.Fatal("validation bypassed", q)
		}
	}
	// The sole historical parser exception is false AND a function-free tree.
	got, _, _ := reviewVectorQuery(t, db, "SELECT 0 AND missing FROM t")
	if !reflect.DeepEqual(got, [][]any{{int64(0)}}) {
		t.Fatal(got)
	}
	// Ordered queries retain eager startup-error reporting.
	r, err := db.Query(context.Background(), "SELECT id FROM t WHERE "+boom+" ORDER BY id")
	if err == nil {
		r.Close()
		t.Fatal("ordered startup error disappeared")
	}
	ctx, cancel := context.WithCancel(context.Background())
	r, err = db.Query(ctx, "SELECT id FROM t WHERE 0 AND "+boom)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if r.Next() || !errors.Is(r.Err(), context.Canceled) {
		t.Fatal(r.Err())
	}
	r.Close()
}

func TestBooleanLazyTypedCellsAndProjection(t *testing.T) {
	tab := &table{name: "t", ipk: -1, columns: []sqlast.ColumnDef{{Name: "flag", Type: "INTEGER"}, {Name: "v", Type: "VECTOR(2)"}}, index: map[string]int{"flag": 0, "v": 1}}
	for _, tc := range []struct {
		where     string
		keep, bad bool
	}{
		{"flag AND v", false, false}, {"flag OR v", false, true}, {"1 OR v", true, false}, {"0 AND v", false, false},
		{"flag AND l2_distance(v,'[0,0]')>0", false, false}, {"l2_distance(v,'[0,0]')>0 AND flag", false, true},
	} {
		e, err := sqlast.ParseExpr(tc.where)
		if err != nil {
			t.Fatal(err)
		}
		r := &Rows{ctx: context.Background(), table: tab, sel: &sqlast.Select{Where: e}, exprs: []sqlast.Expr{&sqlast.Literal{Value: int64(7)}}, aliases: map[string]int{}}
		if err = r.prepareWhere(); err != nil {
			t.Fatal(err)
		}
		// No descriptor/source exists in this evaluator-only regression.
		if err = r.startPredicates(); err != nil {
			if tc.keep || tc.bad {
				t.Fatal(tc.where, err)
			}
			continue
		}
		_, _, keep, err := r.evaluateSourceRow(1, []any{int64(0), "invalid vector text"})
		if (err != nil) != tc.bad || err == nil && keep != tc.keep {
			t.Fatal(tc.where, keep, err)
		}
	}
}

func TestBooleanConservativeIndexErrorBoundary(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER)", "INSERT INTO t VALUES(1,1)", "CREATE INDEX bool_k ON t(k)"} {
		reviewVectorExec(t, db, q)
	}
	// Candidate correction of the retained original SQL. Baseline source bytes
	// remain separate evidence; execution/oracle qualification is still required.
	for _, modifier := range []string{"", " NOT INDEXED"} {
		r, err := db.Query(context.Background(), "SELECT id FROM t"+modifier+" WHERE k LIKE 'x' ESCAPE 'ab' AND id=9999")
		if err != nil {
			t.Fatal(err)
		}
		if r.Plan().Access != "SEARCH" || r.Plan().Method != "rowid" || !r.Plan().Recheck {
			t.Fatal("missing IPK equality seek", r.Plan())
		}
		if r.Next() || r.Err() != nil || r.Stats().RowIDSeeks != 1 || r.Stats().BaseRowFetches != 0 {
			t.Fatal(r.Err(), r.Stats())
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
		// Keep the original reversed-conjunct result assertion.
		got, _, _ := reviewVectorQuery(t, db, "SELECT id FROM t"+modifier+" WHERE id=9999 AND k LIKE 'x' ESCAPE 'ab'")
		if len(got) != 0 {
			t.Fatal(got)
		}
		// The same residual still errors after a matching source row is fetched.
		r, err = db.Query(context.Background(), "SELECT id FROM t"+modifier+" WHERE k LIKE 'x' ESCAPE 'ab' AND id=1")
		if err != nil {
			t.Fatal(err)
		}
		if r.Next() || r.Err() == nil || !strings.Contains(r.Err().Error(), "ESCAPE") || r.Stats().RowIDSeeks != 1 || r.Stats().BaseRowFetches != 1 {
			t.Fatal(r.Err(), r.Stats())
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBooleanDMLPredicatesAndRollback(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER)")
	reviewVectorExec(t, db, "INSERT INTO t VALUES(1,0,-9223372036854775808)")
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for _, q := range []string{"UPDATE t SET flag=1 WHERE 0 AND abs(-9223372036854775808)", "DELETE FROM t WHERE flag AND abs(v)"} {
		if _, err := tx.Exec(context.Background(), q); err != nil {
			t.Fatal(q, err)
		}
	}
	if _, err := tx.Exec(context.Background(), "UPDATE t SET flag=1 WHERE 1 OR abs(-9223372036854775808)"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	got, _, _ := reviewVectorQuery(t, db, "SELECT flag FROM t")
	if !reflect.DeepEqual(got, [][]any{{int64(0)}}) {
		t.Fatal(got)
	}
}

func TestBooleanTransitiveConstantExpansionIsBounded(t *testing.T) {
	// Descriptor-free planner table, with a linear SQL tree whose naive fixed
	// substitution proof would expand exponentially. No database file is used.
	columns := []sqlast.ColumnDef{}
	index := map[string]int{}
	var terms []string
	for i := 0; i < 24; i++ {
		name := fmt.Sprintf("n%d", i)
		index[name] = i
		columns = append(columns, sqlast.ColumnDef{Name: name, Type: "INTEGER"})
		value := "1"
		if i > 0 {
			value = fmt.Sprintf("n%d+n%d", i-1, i-1)
		}
		terms = append(terms, name+"="+value)
	}
	e, err := sqlast.ParseExpr(strings.Join(terms, " AND "))
	if err != nil {
		t.Fatal(err)
	}
	r := &Rows{ctx: context.Background(), table: &table{name: "t", ipk: -1, columns: columns, index: index}, sel: &sqlast.Select{Where: e}, aliases: map[string]int{}}
	if err = r.prepareWhere(); !errors.Is(err, ErrQueryWorkLimit) {
		t.Fatal("unbounded expanded predicate accepted", err)
	}
	// The same shape must also stop under the owning relational work budget,
	// rather than doing an uncharged recursive constancy proof.
	budget, err := newRelationalBudget(WithRelationalLimits(context.Background(), RelationalLimits{MaxWork: 5000}), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	r.relational = budget
	r.ctx = sqlast.WithEvalBudget(context.Background(), budget)
	if err = r.fixPredicateConstants([]sqlast.Expr{e}); !errors.Is(err, ErrQueryWorkLimit) {
		t.Fatal("planning work quota escaped", err)
	}
}
