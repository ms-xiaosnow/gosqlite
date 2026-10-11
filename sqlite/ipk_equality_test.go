package sqlite

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// This finite source candidate has no runtime qualification until the baseline,
// pinned oracle and candidate are separately executed under reviewed selectors.
const ipkEqualitySeed = `CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER);
INSERT INTO t VALUES(1,1);
CREATE TABLE shadowed(id INTEGER PRIMARY KEY,rowid TEXT,k INTEGER);
INSERT INTO shadowed VALUES(1,'shadow',1);`

type ipkEqualityCase struct {
	name, query, family, method string
	args                        []any
	want                        [][]any
	seeks, fetches               int64
	plan                        bool
}

// Exactly 24 intact SQL cases are shared by the normal and tagged oracle roots.
// They are enumerated rather than expanded into a product or random campaign.
func ipkEqualityCases() []ipkEqualityCase {
	return []ipkEqualityCase{
		{name: "missing", query: "SELECT id FROM t WHERE k LIKE 'x' ESCAPE 'ab' AND id=9999", method: "rowid", seeks: 1},
		{name: "missing_not_indexed", query: "SELECT id FROM t NOT INDEXED WHERE k LIKE 'x' ESCAPE 'ab' AND id=9999", method: "rowid", seeks: 1},
		{name: "reversed", query: "SELECT id FROM t WHERE id=9999 AND k LIKE 'x' ESCAPE 'ab'", method: "rowid", seeks: 1},
		{name: "reversed_not_indexed", query: "SELECT id FROM t NOT INDEXED WHERE id=9999 AND k LIKE 'x' ESCAPE 'ab'", method: "rowid", seeks: 1},
		{name: "present", query: "SELECT id FROM t WHERE k LIKE 'x' ESCAPE 'ab' AND id=1", family: "ESCAPE", method: "rowid", seeks: 1, fetches: 1},
		{name: "present_not_indexed", query: "SELECT id FROM t NOT INDEXED WHERE k LIKE 'x' ESCAPE 'ab' AND id=1", family: "ESCAPE", method: "rowid", seeks: 1, fetches: 1},
		{name: "present_reversed", query: "SELECT id FROM t WHERE id=1 AND k LIKE 'x' ESCAPE 'ab'", family: "ESCAPE", method: "rowid", seeks: 1, fetches: 1},
		{name: "present_reject_first", query: "SELECT id FROM t WHERE k<0 AND k LIKE 'x' ESCAPE 'ab' AND id=1", method: "rowid", seeks: 1, fetches: 1},
		{name: "present_error_first", query: "SELECT id FROM t WHERE k LIKE 'x' ESCAPE 'ab' AND k<0 AND id=1", family: "ESCAPE", method: "rowid", seeks: 1, fetches: 1},
		{name: "null_key", query: "SELECT id FROM t WHERE k LIKE 'x' ESCAPE 'ab' AND id=NULL", method: "rowid"},
		{name: "fraction_key", query: "SELECT id FROM t WHERE k LIKE 'x' ESCAPE 'ab' AND id=1.5", method: "rowid"},
		{name: "text_affinity", query: "SELECT id FROM t NOT INDEXED WHERE k LIKE '1' AND id='1.0'", method: "rowid", seeks: 1, fetches: 1, want: [][]any{{int64(1)}}},
		{name: "bound_present", query: "SELECT id FROM t WHERE k LIKE 'x' ESCAPE 'ab' AND id=?1", args: []any{int64(1)}, family: "ESCAPE", method: "rowid", seeks: 1, fetches: 1},
		{name: "operand_reversed", query: "SELECT id FROM t WHERE k LIKE 'x' ESCAPE 'ab' AND 9999=id", method: "rowid", seeks: 1},
		{name: "unary_key", query: "SELECT id FROM t WHERE k LIKE 'x' ESCAPE 'ab' AND id=-(-1)", family: "ESCAPE", method: "rowid", seeks: 1, fetches: 1},
		{name: "double_equals", query: "SELECT id FROM t WHERE k LIKE 'x' ESCAPE 'ab' AND id==9999", method: "rowid", seeks: 1},
		{name: "startup_like", query: "SELECT id FROM t WHERE 'a' LIKE 'x' ESCAPE 'ab' AND id=9999", family: "ESCAPE", method: "rowid"},
		{name: "startup_like_reversed", query: "SELECT id FROM t NOT INDEXED WHERE id=9999 AND 'a' LIKE 'x' ESCAPE 'ab'", family: "ESCAPE", method: "rowid"},
		{name: "startup_overflow_old_path", query: "SELECT id FROM t WHERE id=9999 AND abs(-9223372036854775808)>0", family: "integer overflow"},
		{name: "limit_zero", query: "SELECT id FROM t WHERE k LIKE 'x' ESCAPE 'ab' AND id=1 LIMIT 0", method: "rowid"},
		{name: "explain", query: "EXPLAIN QUERY PLAN SELECT id FROM t NOT INDEXED WHERE k LIKE 'x' ESCAPE 'ab' AND id=1", method: "rowid", plan: true},
		{name: "physical_rowid_shadow", query: "SELECT id FROM shadowed NOT INDEXED WHERE k LIKE 'x' ESCAPE 'ab' AND rowid='missing'", family: "ESCAPE", fetches: 1},
		{name: "unshadowed_rowid_alias", query: "SELECT id FROM shadowed NOT INDEXED WHERE k LIKE 'x' ESCAPE 'ab' AND _rowid_=9999", method: "rowid", seeks: 1},
		{name: "whole_comparison_scan_plan", query: "EXPLAIN QUERY PLAN SELECT id FROM t NOT INDEXED WHERE +(id='1.0')", plan: true},
	}
}

func ipkEqualityFixture(t *testing.T) *DB {
	t.Helper()
	db, _ := reviewVectorDB(t, Options{})
	for _, query := range strings.Split(ipkEqualitySeed, ";") {
		if strings.TrimSpace(query) != "" {
			reviewVectorExec(t, db, query)
		}
	}
	return db
}

func ipkEqualityNative(t *testing.T, db *DB, tc ipkEqualityCase) [][]any {
	t.Helper()
	before := db.counters
	r, err := db.Query(context.Background(), tc.query, tc.args...)
	if err != nil || r == nil {
		t.Fatalf("Query phase: rows=%v error=%v", r, err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	plan := r.Plan()
	access := "SCAN"
	if tc.method != "" {
		access = "SEARCH"
	}
	if r.Stats().RowIDSeeks != 0 || r.Stats().BaseRowFetches != 0 || r.Stats().IndexKeysRead != 0 || r.Stats().IndexPageReads != 0 {
		t.Fatal("unsorted Query performed row access", r.Stats())
	}
	var got [][]any
	for r.Next() {
		got = append(got, r.Values())
	}
	err = r.Err()
	t.Logf("native SQL=%q args=%#v QueryErr=nil RowsErr=%v rows=%#v plan=%+v stats=%+v", tc.query, tc.args, err, got, plan, r.Stats())
	if plan.Access != access || plan.Method != tc.method || tc.method == "rowid" && (!plan.Recheck || !reflect.DeepEqual(plan.Fields, []string{"rowid"}) || len(plan.Indexes) != 0) {
		t.Fatal("unexpected plan", plan)
	}
	if tc.family == "" {
		if err != nil {
			t.Fatal("Rows.Err phase", err)
		}
	} else {
		var typed *Error
		if !errors.As(err, &typed) || typed.Code != CodeError || !strings.Contains(err.Error(), tc.family) || len(got) != 0 {
			t.Fatalf("Rows.Err=%v rows=%#v; want CodeError/%s", err, got, tc.family)
		}
	}
	if tc.plan {
		if len(got) != 1 || len(got[0]) != 4 || got[0][3] != plan.Detail() {
			t.Fatal("EXPLAIN result", got, plan)
		}
	} else if !reflect.DeepEqual(got, tc.want) {
		t.Fatal("result values", got, tc.want)
	}
	stats := r.Stats()
	if stats.RowIDSeeks != tc.seeks || stats.BaseRowFetches != tc.fetches || stats.IndexKeysRead != 0 || stats.IndexPageReads != 0 || stats.ReturnedRows != int64(len(got)) {
		t.Fatal("attempted work", stats, tc.seeks, tc.fetches)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan, r.Plan()) || len(db.rows) != 0 || db.counters != before {
		t.Fatal("plan, query lease or connection counters changed after Close")
	}
	return got
}

func TestIPKEqualityErrorTimingSlice(t *testing.T) {
	db := ipkEqualityFixture(t)
	cases := ipkEqualityCases()
	if len(cases) != 24 {
		t.Fatal("finite case inventory changed", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { ipkEqualityNative(t, db, tc) })
	}
	t.Run("ordered_startup_query_error", func(t *testing.T) {
		before := db.counters
		r, err := db.Query(context.Background(), "SELECT id FROM t WHERE 'a' LIKE 'x' ESCAPE 'ab' AND id=9999 ORDER BY k")
		if r != nil {
			if closeErr := r.Close(); closeErr != nil {
				t.Error(closeErr)
			}
			t.Fatal("ordered startup returned Rows")
		}
		var typed *Error
		if !errors.As(err, &typed) || typed.Code != CodeError || !strings.Contains(err.Error(), "ESCAPE") || len(db.rows) != 0 || db.counters != before {
			t.Fatal("ordered startup must fail in Query and release its owner", err)
		}
	})
	t.Run("close_before_next", func(t *testing.T) {
		r, err := db.Query(context.Background(), "SELECT id FROM t NOT INDEXED WHERE k LIKE '1' AND id=1")
		if err != nil {
			t.Fatal(err)
		}
		plan := r.Plan()
		if r.release == nil || len(db.rows) != 1 {
			t.Fatal("autocommit Rows lost its snapshot owner")
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
		if err := r.Close(); err != nil || r.Next() || r.Err() != nil || len(db.rows) != 0 || r.Stats().RowIDSeeks != 0 || r.Stats().BaseRowFetches != 0 || !reflect.DeepEqual(plan, r.Plan()) {
			t.Fatal("unstarted Close changed result/work/ownership", err, r.Stats())
		}
	})
	t.Run("cancel_before_next", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		r, err := db.Query(ctx, "SELECT id FROM t NOT INDEXED WHERE k LIKE '1' AND id=1")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := r.Close(); err != nil {
				t.Error(err)
			}
		})
		cancel()
		if r.Next() || !errors.Is(r.Err(), context.Canceled) || r.Plan().Method != "rowid" || r.Stats().RowIDSeeks != 0 || r.Stats().BaseRowFetches != 0 || len(db.rows) != 0 {
			t.Fatal(r.Err(), r.Plan(), r.Stats(), len(db.rows))
		}
	})
	t.Run("transaction_owner_and_rollback", func(t *testing.T) {
		ctx := context.Background()
		tx, err := db.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := tx.Rollback(); err != nil {
				t.Error(err)
			}
		})
		if _, err := tx.Exec(ctx, "UPDATE t SET k=2 WHERE id=1"); err != nil {
			t.Fatal(err)
		}
		r, err := tx.Query(ctx, "SELECT k FROM t NOT INDEXED WHERE k LIKE '2' AND id=1")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := r.Close(); err != nil {
				t.Error(err)
			}
		})
		src, ok := r.source.(*ordinaryRowSource)
		if !ok || src.reader != tx.reader || r.tx != tx || r.release != nil || tx.activeRows != 1 {
			t.Fatal("rowid source lost the owning transaction snapshot")
		}
		if err := tx.Commit(ctx); !errors.Is(err, ErrBusy) {
			t.Fatal("active Rows must retain its lease", err)
		}
		if _, err := tx.Exec(ctx, "UPDATE t SET k=3 WHERE id=1"); !errors.Is(err, ErrBusy) {
			t.Fatal("active Rows allowed a write", err)
		}
		if !r.Next() || !reflect.DeepEqual(r.Values(), []any{int64(2)}) || r.Stats().RowIDSeeks != 1 || r.Stats().BaseRowFetches != 1 {
			t.Fatal("transaction read-own-write", r.Values(), r.Err(), r.Stats())
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
		if err := r.Close(); err != nil || tx.activeRows != 0 || len(db.rows) != 0 {
			t.Fatal("Close must release exactly once", err, tx.activeRows)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		got, _, _ := reviewVectorQuery(t, db, "SELECT k FROM t WHERE id=1")
		if !reflect.DeepEqual(got, [][]any{{int64(1)}}) {
			t.Fatal("normal rollback changed original row", got)
		}
	})
}

// EXPLAIN plus NOT INDEXED makes this a descriptor-free plan admission check.
// It uses an existing planner entry point, so the same new test source can be
// overlaid on the preserved baseline for a separately authorized RED comparison.
func ipkEqualityAdmission(t *testing.T, r *Rows) bool {
	t.Helper()
	r.sel.Explain, r.sel.NotIndexed = true, true
	r.queryPlan = QueryPlan{Access: "SCAN"}
	db := &DB{}
	if err := db.planOrdinarySource(context.Background(), nil, r); err != nil {
		t.Fatal(err)
	}
	return r.queryPlan.Method == "rowid"
}

func TestIPKEqualityAdmissionBoundaries(t *testing.T) {
	base := &table{name: "t", columns: []sqlast.ColumnDef{{Name: "id", Type: "INTEGER", PrimaryKey: true}, {Name: "k", Type: "INTEGER"}}, index: map[string]int{"id": 0, "k": 1}, ipk: 0}
	for _, tc := range []struct {
		name, query string
		want        bool
	}{
		{"direct", "SELECT id FROM t WHERE k LIKE 'x' AND id=1", true},
		{"qualified", "SELECT t.id FROM t WHERE k LIKE 'x' AND 1=t.id", true},
		{"result_alias", "SELECT id AS rid FROM t WHERE k LIKE 'x' AND rid=1", true},
		{"physical_wins", "SELECT 42 AS id FROM t WHERE k LIKE 'x' AND id=1", true},
		{"cast_key", "SELECT id FROM t WHERE k LIKE 'x' AND id=CAST(1 AS INTEGER)", false},
		{"cast_column", "SELECT id FROM t WHERE k LIKE 'x' AND CAST(id AS INTEGER)=1", false},
		{"is_key", "SELECT id FROM t WHERE k LIKE 'x' AND id IS 1", false},
		{"is_null", "SELECT id FROM t WHERE k LIKE 'x' AND id IS NULL", false},
		{"like_expr_is", "SELECT id FROM t WHERE id=1 AND (k IS NULL) LIKE '1'", false},
		{"like_pattern_rowid_range", "SELECT id FROM t WHERE id=1 AND k LIKE (id>0)", false},
		{"like_escape_rowid_equality", "SELECT id FROM t WHERE id=1 AND k LIKE 'x' ESCAPE (id=2)", false},
		{"competing", "SELECT id FROM t WHERE k LIKE 'x' AND id=1 AND id=2", false},
		{"competing_is", "SELECT id FROM t WHERE k LIKE 'x' AND id=1 AND id IS 2", false},
		{"or_only", "SELECT id FROM t WHERE k LIKE 'x' AND (id=1 OR id=2)", false},
		{"range_only", "SELECT id FROM t WHERE k LIKE 'x' AND id>1", false},
		{"range_residual", "SELECT id FROM t WHERE k LIKE 'x' AND id=1 AND id>0", false},
		{"function", "SELECT id FROM t WHERE abs(k)>0 AND id=1", false},
		{"function_key", "SELECT id FROM t WHERE k LIKE 'x' AND id=abs(1)", false},
		{"extension", "SELECT id FROM t WHERE l2_distance('bad','[0]')>0 AND id=1", false},
		{"wrapped_comparison", "SELECT id FROM t WHERE +(id=1)", false},
		{"join", "SELECT t.id FROM t JOIN t AS u ON t.id=u.id WHERE t.id=1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stmt, err := sqlast.Parse(tc.query)
			if err != nil {
				t.Fatal(err)
			}
			sel := stmt.(*sqlast.Select)
			r := &Rows{table: base, sel: sel, aliases: map[string]int{}}
			for i, col := range sel.Columns {
				r.exprs = append(r.exprs, col.Expr)
				if col.Alias != "" {
					r.aliases[fold(col.Alias)] = i
				}
			}
			if got := ipkEqualityAdmission(t, r); got != tc.want {
				t.Fatal("narrow admission", got, tc.want)
			}
		})
	}
	stmt, err := sqlast.Parse("SELECT id FROM t WHERE k LIKE 'x' AND id=1")
	if err != nil {
		t.Fatal(err)
	}
	r := &Rows{table: base, sel: stmt.(*sqlast.Select), aliases: map[string]int{}}
	copyTable := *base
	copyTable.ipk = -1
	r.table = &copyTable
	if ipkEqualityAdmission(t, r) {
		t.Fatal("implicit-rowid-only table admitted")
	}
	copyTable = *base
	copyTable.columns = append(append([]sqlast.ColumnDef(nil), base.columns...), sqlast.ColumnDef{Name: "v", Type: "VECTOR(2)"})
	r.table = &copyTable
	if ipkEqualityAdmission(t, r) {
		t.Fatal("unreferenced representation-bearing column admitted")
	}
	r.table = base
	r.vectorRank = &vectorRank{}
	if ipkEqualityAdmission(t, r) {
		t.Fatal("vector ranking admitted")
	}
	r.vectorRank = nil
	r.relational = &relationalBudget{}
	if ipkEqualityAdmission(t, r) {
		t.Fatal("relational source admitted")
	}
	unsafeDefault, err := sqlast.ParseExpr("1+2")
	if err != nil {
		t.Fatal(err)
	}
	copyTable = *base
	copyTable.columns = append([]sqlast.ColumnDef(nil), base.columns...)
	copyTable.columns[1].Default = unsafeDefault
	if ordinarySafeDefaults(context.Background(), &copyTable) {
		t.Fatal("nonliteral default admitted")
	}

	db := ipkEqualityFixture(t)
	for _, query := range []string{"SELECT id FROM t NOT INDEXED WHERE +(id='1.0')", "SELECT id FROM t NOT INDEXED WHERE id IS 1"} {
		got, plan, stats := reviewVectorQuery(t, db, query)
		if !reflect.DeepEqual(got, [][]any{{int64(1)}}) || plan.Access != "SCAN" || stats.RowIDSeeks != 0 || stats.BaseRowFetches != 1 {
			t.Fatal("structural scan/old IS control", query, got, plan, stats)
		}
	}
	got, plan, _ := reviewVectorQuery(t, db, "SELECT count(*) FROM t WHERE id=1")
	if !reflect.DeepEqual(got, [][]any{{int64(1)}}) || plan.Method == "rowid" {
		t.Fatal("aggregate path changed", got, plan)
	}
}
