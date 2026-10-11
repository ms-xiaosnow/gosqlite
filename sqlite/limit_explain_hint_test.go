package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// These are pure hint inputs, not calls to the SQL evaluator or binder.
func TestLimitExplainHintIntegerBoundaries(t *testing.T) {
	type noncanonicalInt64 int64
	for _, tc := range []struct {
		name  string
		value any
		want  int64
		known bool
	}{
		{"integer_min", int64(math.MinInt64), math.MinInt64, true},
		{"integer_max", int64(math.MaxInt64), math.MaxInt64, true},
		{"integer_zero", int64(0), 0, true},
		{"integer_positive", int64(3), 3, true},
		{"integer_negative", int64(-3), -3, true},
		{"real_positive", float64(3), 3, true},
		{"real_negative", float64(-3), -3, true},
		{"real_negative_zero", math.Copysign(0, -1), 0, true},
		{"real_lower_neighbor", math.Nextafter(float64(math.MinInt64), 0), -9223372036854774784, true},
		{"real_upper_neighbor", math.Nextafter(9223372036854775808.0, 0), 9223372036854774784, true},
		{"real_min_endpoint", float64(math.MinInt64), 0, false},
		{"real_max_rounded_endpoint", float64(math.MaxInt64), 0, false},
		{"real_below_min", math.Nextafter(float64(math.MinInt64), math.Inf(-1)), 0, false},
		{"real_above_max", math.Nextafter(9223372036854775808.0, math.Inf(1)), 0, false},
		{"real_fraction", 1.5, 0, false},
		{"real_subnormal", math.SmallestNonzeroFloat64, 0, false},
		{"real_nan", math.NaN(), 0, false},
		{"real_positive_infinity", math.Inf(1), 0, false},
		{"real_negative_infinity", math.Inf(-1), 0, false},
		{"null", nil, 0, false},
		{"text_numeric", "3", 0, false},
		{"text_nul_numeric", "3\x00tail", 0, false},
		{"blob_numeric", []byte("3"), 0, false},
		{"raw_bool", true, 0, false},
		{"raw_int", int(3), 0, false},
		{"raw_uint64", uint64(3), 0, false},
		{"raw_float32", float32(3), 0, false},
		{"named_integer", noncanonicalInt64(3), 0, false},
		{"arbitrary_value", struct{}{}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, leaf := range []struct {
				name   string
				expr   sqlast.Expr
				params []any
			}{
				{"literal", &sqlast.Literal{Value: tc.value}, nil},
				{"parameter", &sqlast.Parameter{Index: 2}, []any{int64(99), tc.value}},
			} {
				t.Run(leaf.name, func(t *testing.T) {
					got, known := explainBoundInteger(leaf.expr, leaf.params)
					if got != tc.want || known != tc.known {
						t.Fatalf("got (%d, %t); want (%d, %t)", got, known, tc.want, tc.known)
					}
				})
			}
		})
	}
	minInt := &sqlast.Literal{Value: int64(math.MinInt64)}
	maxInt := &sqlast.Literal{Value: int64(math.MaxInt64)}
	three := &sqlast.Literal{Value: int64(3)}
	for _, tc := range []struct {
		name   string
		expr   sqlast.Expr
		params []any
		want   int64
		known  bool
	}{
		{"absent", nil, nil, 0, false},
		{"boolean_true_literal", &sqlast.Literal{Value: int64(1), Boolean: true}, nil, 1, true},
		{"boolean_false_literal", &sqlast.Literal{Value: int64(0), Boolean: true}, nil, 0, true},
		{"parameter_first", &sqlast.Parameter{Index: 1}, []any{int64(3)}, 3, true},
		{"parameter_named", &sqlast.Parameter{Index: 2, Name: ":bound"}, []any{nil, int64(3)}, 3, true},
		{"parameter_zero", &sqlast.Parameter{Index: 0}, []any{int64(3)}, 0, false},
		{"parameter_negative", &sqlast.Parameter{Index: -1}, []any{int64(3)}, 0, false},
		{"parameter_past_end", &sqlast.Parameter{Index: 2}, []any{int64(3)}, 0, false},
		{"parameter_unbound", &sqlast.Parameter{Index: 1}, nil, 0, false},
		{"unary_plus_min", &sqlast.Unary{Op: "+", Expr: minInt}, nil, math.MinInt64, true},
		{"unary_minus_max", &sqlast.Unary{Op: "-", Expr: maxInt}, nil, -math.MaxInt64, true},
		{"unary_minus_min_overflow", &sqlast.Unary{Op: "-", Expr: minInt}, nil, 0, false},
		{"unary_nested", &sqlast.Unary{Op: "-", Expr: &sqlast.Unary{Op: "-", Expr: maxInt}}, nil, math.MaxInt64, true},
		{"unary_nested_overflow", &sqlast.Unary{Op: "-", Expr: &sqlast.Unary{Op: "-", Expr: minInt}}, nil, 0, false},
		{"unary_parameter", &sqlast.Unary{Op: "-", Expr: &sqlast.Parameter{Index: 1}}, []any{float64(-3)}, 3, true},
		{"unary_text", &sqlast.Unary{Op: "+", Expr: &sqlast.Literal{Value: "3"}}, nil, 0, false},
		{"unary_not", &sqlast.Unary{Op: "NOT", Expr: three}, nil, 0, false},
		{"unary_bitwise", &sqlast.Unary{Op: "~", Expr: three}, nil, 0, false},
		{"call", &sqlast.Call{Name: "abs", Args: []sqlast.Expr{minInt}}, nil, 0, false},
		{"cast", &sqlast.Cast{Expr: three, Type: "INTEGER"}, nil, 0, false},
		{"binary", &sqlast.Binary{Op: "+", Left: three, Right: three}, nil, 0, false},
		{"column", &sqlast.Column{Name: "bound"}, nil, 0, false},
		{"other_form", &sqlast.Star{}, nil, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, known := explainBoundInteger(tc.expr, tc.params)
			if got != tc.want || known != tc.known {
				t.Fatalf("got (%d, %t); want (%d, %t)", got, known, tc.want, tc.known)
			}
		})
	}
}

func TestLimitExplainHintBounds(t *testing.T) {
	literal := func(value any) sqlast.Expr { return &sqlast.Literal{Value: value} }
	cap := int64(vector.DefaultSearchLimits().MaxResults)
	unknown := &sqlast.Call{Name: "abs", Args: []sqlast.Expr{literal(int64(math.MinInt64))}}
	for _, tc := range []struct {
		name          string
		limit, offset sqlast.Expr
		params        []any
		want, skip    int64
	}{
		{"absent", nil, nil, nil, -1, 0},
		{"unlimited", literal(int64(-1)), nil, nil, -1, 0},
		{"integer_min", literal(int64(math.MinInt64)), nil, nil, -1, 0},
		{"unknown_limit", unknown, literal(int64(0)), nil, -1, 0},
		{"text_limit", literal("3"), nil, nil, -1, 0},
		{"null_limit", literal(nil), nil, nil, -1, 0},
		{"zero_no_offset", literal(int64(0)), nil, nil, 0, 0},
		{"zero_unknown_offset", literal(int64(0)), unknown, nil, 0, 0},
		{"zero_unbound_offset", literal(int64(0)), &sqlast.Parameter{Index: 1}, nil, 0, 0},
		{"zero_overflow_offset", literal(int64(0)), literal(int64(math.MaxInt64)), nil, 0, 0},
		{"positive_no_offset", literal(int64(3)), nil, nil, 3, 0},
		{"positive_offset", literal(int64(3)), literal(int64(2)), nil, 3, 2},
		{"real_bounds", literal(float64(3)), literal(float64(2)), nil, 3, 2},
		{"negative_offset", literal(int64(3)), literal(int64(-2)), nil, 3, 0},
		{"minimum_offset", literal(int64(3)), literal(int64(math.MinInt64)), nil, 3, 0},
		{"minimum_real_offset", literal(int64(3)), literal(float64(math.MinInt64)), nil, -1, 0},
		{"unknown_offset", literal(int64(3)), unknown, nil, -1, 0},
		{"text_offset", literal(int64(3)), literal("0"), nil, -1, 0},
		{"null_offset", literal(int64(3)), literal(nil), nil, -1, 0},
		{"unbound_offset", literal(int64(3)), &sqlast.Parameter{Index: 2}, []any{int64(0)}, -1, 0},
		{"canonical_parameters", &sqlast.Parameter{Index: 1}, &sqlast.Parameter{Index: 2}, []any{int64(3), float64(2)}, 3, 2},
		{"cap_limit", literal(cap), nil, nil, cap, 0},
		{"cap_sum", literal(cap - 1), literal(int64(1)), nil, cap - 1, 1},
		{"over_cap_limit", literal(cap + 1), nil, nil, -1, 0},
		{"over_cap_sum", literal(cap), literal(int64(1)), nil, -1, 0},
		{"over_cap_offset", literal(int64(1)), literal(cap), nil, -1, 0},
		{"overflow_limit", literal(int64(math.MaxInt64)), literal(int64(1)), nil, -1, 0},
		{"overflow_offset", literal(int64(1)), literal(int64(math.MaxInt64)), nil, -1, 0},
		{"overflow_both", literal(int64(math.MaxInt64)), literal(int64(math.MaxInt64)), nil, -1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, skip := explainPlanBounds(tc.limit, tc.offset, tc.params)
			if got != tc.want || skip != tc.skip {
				t.Fatalf("got (%d, %d); want (%d, %d)", got, skip, tc.want, tc.skip)
			}
		})
	}
}

// Only valid ordinary fixture writes are used. EXPLAIN reads metadata but must
// never score vectors, traverse ANN candidates, or execute LIMIT/OFFSET calls.
func TestLimitExplainHintPlanPreservation(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "explain.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, table := range []string{"hnsw_items", "ivf_items"} {
		if _, err := db.Exec(ctx, "CREATE TABLE "+table+"(id INTEGER PRIMARY KEY,v VECTOR(2) NOT NULL)"); err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= 16; i++ {
			if _, err := db.Exec(ctx, "INSERT INTO "+table+" VALUES(?,?)", i, fmt.Sprintf("[%d,1]", i)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := db.CreateHNSWIndex(ctx, "explain_hnsw", "hnsw_items", "v"); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateIVFFlatIndex(ctx, "explain_ivf", "ivf_items", "v", IVFFlatIndexOptions{Lists: 2}); err != nil {
		t.Fatal(err)
	}
	for _, shape := range []struct {
		name, from, method, ranking string
	}{
		{"hnsw", "hnsw_items", "hnsw", "ann_exact_rescore"},
		{"ivfflat", "ivf_items", "ivfflat", "ann_exact_rescore"},
		{"exact", "hnsw_items NOT INDEXED", "", "bounded_heap"},
	} {
		t.Run(shape.name, func(t *testing.T) {
			for _, tc := range []struct {
				name, bounds string
				args         []any
				topK         int64
			}{
				{"integer", "LIMIT 3 OFFSET 1", nil, 4},
				{"real", "LIMIT 3.0 OFFSET 1.0", nil, 4},
				{"signed", "LIMIT +3 OFFSET -1", nil, 3},
				{"boolean", "LIMIT TRUE OFFSET FALSE", nil, 1},
				{"bound_integer", "LIMIT ? OFFSET ?", []any{int64(3), int64(1)}, 4},
				{"bound_real", "LIMIT ? OFFSET ?", []any{float64(3), float64(1)}, 4},
				{"admitted_go_types", "LIMIT ? OFFSET ?", []any{int(3), float32(1)}, 4},
				{"admitted_bool", "LIMIT ? OFFSET ?", []any{true, false}, 1},
				{"bound_signed", "LIMIT -? OFFSET +?", []any{int64(-3), int64(1)}, 4},
				{"named_parameters", "LIMIT :take OFFSET :skip", []any{sql.Named("skip", 1), sql.Named("take", 3)}, 4},
			} {
				t.Run(tc.name, func(t *testing.T) {
					query := "EXPLAIN QUERY PLAN SELECT id FROM " + shape.from + " ORDER BY v <-> '[1,1]' " + tc.bounds
					plan := limitExplainHintPlan(t, db, query, tc.args...)
					if plan.VectorRanking != shape.ranking || plan.VectorTopK != tc.topK || plan.Method != shape.method || plan.VectorExact != (shape.name == "exact") {
						t.Fatal(plan)
					}
					if shape.name == "ivfflat" && (plan.ANNTrainingState != "ready" || plan.ANNLists != 2) {
						t.Fatal(plan)
					}
				})
			}
		})
	}
	for _, tc := range []struct {
		name, bounds string
		args         []any
	}{
		{"text_limit", "LIMIT '3'", nil},
		{"text_offset", "LIMIT 3 OFFSET '1'", nil},
		{"bound_text", "LIMIT ?", []any{"3"}},
		{"bound_blob", "LIMIT ?", []any{[]byte("3")}},
		{"null_limit", "LIMIT NULL", nil},
		{"null_offset", "LIMIT 3 OFFSET NULL", nil},
		{"fraction", "LIMIT 1.5", nil},
		{"nonfinite", "LIMIT ?", []any{math.Inf(1)}},
		{"call", "LIMIT abs(-9223372036854775808)", nil},
		{"cast", "LIMIT CAST('3' AS INTEGER)", nil},
		{"arithmetic", "LIMIT 1+2", nil},
		{"unknown_offset", "LIMIT 3 OFFSET abs(-9223372036854775808)", nil},
		{"negative_unlimited", "LIMIT -1", nil},
		{"over_cap", fmt.Sprintf("LIMIT %d", vector.DefaultSearchLimits().MaxResults+1), nil},
		{"over_cap_sum", fmt.Sprintf("LIMIT %d OFFSET 1", vector.DefaultSearchLimits().MaxResults), nil},
		{"overflow_sum", "LIMIT 1 OFFSET 9223372036854775807", nil},
	} {
		t.Run("conservative/"+tc.name, func(t *testing.T) {
			plan := limitExplainHintPlan(t, db, "EXPLAIN QUERY PLAN SELECT id FROM hnsw_items ORDER BY v <-> '[1,1]' "+tc.bounds, tc.args...)
			if plan.VectorRanking != "" || plan.Method == "hnsw" || plan.Method == "ivfflat" {
				t.Fatal(plan)
			}
		})
	}
	t.Run("zero_suppresses_offset", func(t *testing.T) {
		plan := limitExplainHintPlan(t, db, "EXPLAIN QUERY PLAN SELECT id FROM hnsw_items ORDER BY v <-> '[1,1]' LIMIT 0 OFFSET abs(-9223372036854775808)")
		if plan.VectorRanking != "bounded_heap" || plan.VectorTopK != 0 || !plan.VectorExact || plan.Method == "hnsw" {
			t.Fatal(plan)
		}
	})
	t.Run("select_still_converts_numeric_text", func(t *testing.T) {
		rows, err := db.Query(ctx, "SELECT id FROM hnsw_items ORDER BY v <-> '[1,1]' LIMIT '3'")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		if plan := rows.Plan(); plan.Method != "hnsw" || plan.VectorTopK != 3 {
			t.Fatal(plan)
		}
		count := 0
		for rows.Next() {
			count++
		}
		if err := rows.Err(); err != nil || count != 3 {
			t.Fatal(count, err)
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func limitExplainHintPlan(t *testing.T, db *DB, query string, args ...any) QueryPlan {
	t.Helper()
	rows, err := db.Query(context.Background(), query, args...)
	if err != nil {
		t.Fatal(query, err)
	}
	defer rows.Close()
	plan := rows.Plan()
	if rows.limit != -1 || rows.offset != 0 {
		t.Fatal("EXPLAIN result bounds were not reset", rows.limit, rows.offset)
	}
	if !rows.Next() {
		t.Fatal("missing EXPLAIN row", rows.Err())
	}
	values := rows.Values()
	if len(values) != 4 || values[3] != plan.Detail() {
		t.Fatal(values, plan)
	}
	if plan.VectorRanking != "" && !strings.Contains(values[3].(string), "vector_metric=l2") {
		t.Fatal(values)
	}
	if rows.Next() || rows.Err() != nil {
		t.Fatal("unexpected EXPLAIN row or error", rows.Err())
	}
	stats := rows.Stats()
	// IVF may read one state record for eligibility. All traversal, scoring,
	// posting, and base-row counters must remain zero.
	stats.ANNStateReads = 0
	if stats != (QueryStats{ReturnedRows: 1}) {
		t.Fatal("EXPLAIN executed query work", stats)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	return plan
}

// An observation-only scalar budget, with no failure or cancellation hook.
type limitExplainHintBudget struct{ steps, bytes int64 }

func (b *limitExplainHintBudget) Step(n int64) error {
	b.steps += n
	return nil
}

func (b *limitExplainHintBudget) Allocate(n int64) error {
	b.bytes += n
	return nil
}

func TestLimitExplainHintNoEvaluation(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "constant-explain.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, tc := range []struct {
		name, bounds string
		args         []any
	}{
		{"integer", "LIMIT 1", nil},
		{"signed_real", "LIMIT +1.0 OFFSET -1", nil},
		{"parameters", "LIMIT ? OFFSET ?", []any{int64(1), int64(1)}},
		{"null", "LIMIT NULL", nil},
		{"zero", "LIMIT 0 OFFSET abs(-9223372036854775808)", nil},
		{"limit_call", "LIMIT abs(-9223372036854775808)", nil},
		{"offset_call", "LIMIT 1 OFFSET abs(-9223372036854775808)", nil},
		{"connection_calls", "LIMIT changes() OFFSET last_insert_rowid()", nil},
		{"cast", "LIMIT CAST('bad' AS INTEGER)", nil},
		{"arithmetic", "LIMIT 1+2", nil},
		{"over_cap", fmt.Sprintf("LIMIT %d", vector.DefaultSearchLimits().MaxResults+1), nil},
		{"overflow_sum", "LIMIT 9223372036854775807 OFFSET 1", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := db.counters
			budget := &limitExplainHintBudget{}
			ctx := sqlast.WithEvalBudget(context.Background(), budget)
			rows, err := db.Query(ctx, "EXPLAIN QUERY PLAN SELECT 1 "+tc.bounds, tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			if !rows.Next() || len(rows.Values()) != 4 || rows.Next() || rows.Err() != nil {
				t.Fatal("expected one EXPLAIN row", rows.Err())
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}
			if budget.steps != 0 || budget.bytes != 0 || db.counters != before || len(db.rows) != 0 {
				t.Fatal("EXPLAIN executed bounds or retained rows", budget, db.counters, before, len(db.rows))
			}
		})
	}
}
