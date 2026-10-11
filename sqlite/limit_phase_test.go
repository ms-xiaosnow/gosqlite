package sqlite

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// The retained V4c test body below is unchanged. Its four ordinary SQL shapes
// use 12 explicit values: 144 typed bound controls, 20 binding/overflow controls,
// four EXPLAIN controls, and one final reuse query (169 total queries).
func TestPlannerParityV4cLimitErrorOwnership(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY,k)", "CREATE INDEX by_k ON t(k)", "INSERT INTO t VALUES(1,1),(2,1),(3,2)"} {
		reviewVectorExec(t, db, q)
	}
	tx, e := db.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	for _, base := range []string{"SELECT id FROM t", "SELECT id FROM t ORDER BY k,id DESC", "SELECT t.id FROM t LEFT JOIN t AS u ON t.id=u.id", "SELECT count(*) FROM t"} {
		for _, value := range []any{nil, 1.5, math.NaN(), math.Inf(1), math.Inf(-1), float64(math.MinInt64), float64(math.MaxInt64), "bad", "0x1p0", "1_0", "\u00a01", []byte("1")} {
			for _, tail := range []string{" LIMIT ?1", " LIMIT 1 OFFSET ?1"} {
				r, e := tx.Query(context.Background(), base+tail, value)
				var typed *Error
				if r != nil || !errors.As(e, &typed) || typed.Code != CodeMismatch || !errors.Is(e, ErrMismatch) || tx.activeRows != 0 {
					t.Fatal(base, tail, value, r, e, tx.activeRows)
				}
			}
			r, e := tx.Query(context.Background(), base+" LIMIT 0 OFFSET ?1", value)
			if e != nil {
				t.Fatal(base, value, e)
			}
			if r.Next() || r.Err() != nil || r.Stats().BaseRowFetches != 0 {
				t.Fatal(base, value, r.Err(), r.Stats())
			}
			if tx.activeRows != 0 {
				t.Fatal(tx.activeRows)
			}
		}
		for _, tail := range []string{" LIMIT NULL OFFSET missing", " LIMIT abs(-9223372036854775808) OFFSET missing", " LIMIT 0 OFFSET missing"} {
			r, e := tx.Query(context.Background(), base+tail)
			var se *Error
			if r != nil || !errors.As(e, &se) || se.Code != CodeError || errors.Is(e, ErrMismatch) || !strings.Contains(e.Error(), "no such column") || tx.activeRows != 0 {
				t.Fatal(base, tail, r, e, tx.activeRows)
			}
		}
		for _, tail := range []string{" LIMIT abs(-9223372036854775808)", " LIMIT 1 OFFSET abs(-9223372036854775808)"} {
			r, e := tx.Query(context.Background(), base+tail)
			var se *Error
			if r != nil || !errors.As(e, &se) || se.Code != CodeError || errors.Is(e, ErrMismatch) || !strings.Contains(e.Error(), "integer overflow") || tx.activeRows != 0 {
				t.Fatal(base, tail, r, e, tx.activeRows)
			}
		}
		r, e := tx.Query(context.Background(), "EXPLAIN QUERY PLAN "+base+" LIMIT NULL OFFSET abs(-9223372036854775808)")
		if e != nil || !r.Next() || r.Stats().BaseRowFetches != 0 {
			t.Fatal(base, r, e)
		}
		p := r.Plan()
		r.Close()
		if !reflect.DeepEqual(p, r.Plan()) || tx.activeRows != 0 {
			t.Fatal(p, r.Plan(), tx.activeRows)
		}
	}
	r, e := tx.Query(context.Background(), "SELECT id FROM t LIMIT 1")
	if e != nil || !r.Next() || r.Values()[0] != int64(1) {
		t.Fatal(r, e)
	}
	r.Close()
	if tx.activeRows != 0 {
		t.Fatal(tx.activeRows)
	}
}

type limitPhaseQuery func(context.Context, string, ...any) (*Rows, error)

// Both modes use the intact three-row conversion fixture and normal cleanup.
func limitPhaseConnection(t *testing.T, mode string) (*DB, limitPhaseQuery, func(*testing.T)) {
	t.Helper()
	db := limitConversionFixture(t)
	query := limitPhaseQuery(db.Query)
	var tx *Tx
	if mode == "tx" {
		var err error
		tx, err = db.Begin(context.Background(), TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := tx.Rollback(); err != nil {
				t.Error(err)
			}
		})
		query = tx.Query
	}
	check := func(t *testing.T) {
		t.Helper()
		if len(db.rows) != 0 || (tx != nil && tx.activeRows != 0) {
			t.Fatal("query retained a Rows lease")
		}
	}
	return db, query, check
}

func limitPhaseError(t *testing.T, rows *Rows, err error, code Code, part string) {
	t.Helper()
	nonnil := rows != nil
	if nonnil {
		rows.Close()
	}
	var typed *Error
	if nonnil || err == nil || !errors.As(err, &typed) || typed.Code != code || !strings.Contains(err.Error(), part) {
		t.Fatalf("rows nonnil=%t error=%v; want code=%d containing %q", nonnil, err, code, part)
	}
	if code != CodeMismatch && errors.Is(err, ErrMismatch) {
		t.Fatalf("non-conversion error was relabeled mismatch: %v", err)
	}
}

func TestLimitPhaseBindingPrecedence(t *testing.T) {
	// 30 explicit statements, each once through DB and once through Tx.
	cases := []struct{ name, query, part string }{
		{"constant_null_missing", "SELECT 1 LIMIT NULL OFFSET missing", "no such column"},
		{"constant_overflow_missing", "SELECT 1 LIMIT abs(-9223372036854775808) OFFSET missing", "no such column"},
		{"constant_zero_missing", "SELECT 1 LIMIT 0 OFFSET missing", "no such column"},
		{"constant_zero_function", "SELECT 1 LIMIT 0 OFFSET nosuch(1)", "function nosuch"},
		{"constant_zero_arity", "SELECT 1 LIMIT 0 OFFSET abs(1,2)", "wrong number of arguments"},
		{"constant_explain_missing", "EXPLAIN QUERY PLAN SELECT 1 LIMIT NULL OFFSET missing", "no such column"},
		{"constant_explain_function", "EXPLAIN QUERY PLAN SELECT 1 LIMIT NULL OFFSET nosuch(1)", "function nosuch"},
		{"constant_explain_arity", "EXPLAIN QUERY PLAN SELECT 1 LIMIT NULL OFFSET abs(1,2)", "wrong number of arguments"},
		{"constant_limit_missing", "SELECT 1 LIMIT missing OFFSET abs(-9223372036854775808)", "no such column"},
		{"constant_limit_function", "SELECT 1 LIMIT nosuch(1) OFFSET abs(-9223372036854775808)", "function nosuch"},
		{"constant_limit_arity", "SELECT 1 LIMIT abs(1,2) OFFSET abs(-9223372036854775808)", "wrong number of arguments"},
		{"single_null_missing", "SELECT id FROM t LIMIT NULL OFFSET missing", "no such column"},
		{"single_overflow_missing", "SELECT id FROM t LIMIT abs(-9223372036854775808) OFFSET missing", "no such column"},
		{"single_zero_function", "SELECT id FROM t LIMIT 0 OFFSET nosuch(1)", "function nosuch"},
		{"single_zero_arity", "SELECT id FROM t LIMIT 0 OFFSET abs(1,2)", "wrong number of arguments"},
		{"single_bound_not_row_scope", "SELECT id FROM t LIMIT 0 OFFSET id", "no such column"},
		{"single_explain_missing", "EXPLAIN QUERY PLAN SELECT id FROM t LIMIT NULL OFFSET missing", "no such column"},
		{"left_null_missing", "SELECT t.id FROM t LEFT JOIN t AS u ON t.id=u.id LIMIT NULL OFFSET missing", "no such column"},
		{"inner_overflow_missing", "SELECT t.id FROM t JOIN t AS u ON t.id=u.id LIMIT abs(-9223372036854775808) OFFSET missing", "no such column"},
		{"left_on_missing", "SELECT t.id FROM t LEFT JOIN t AS u ON missing=1 LIMIT NULL", "no such column"},
		{"left_on_function", "SELECT t.id FROM t LEFT JOIN t AS u ON nosuch(1)=1 LIMIT NULL", "function nosuch"},
		{"left_on_arity", "SELECT t.id FROM t LEFT JOIN t AS u ON abs(1,2)=1 LIMIT abs(-9223372036854775808)", "wrong number of arguments"},
		{"inner_on_missing", "SELECT t.id FROM t JOIN t AS u ON missing=1 LIMIT NULL", "no such column"},
		{"left_zero_on_missing", "SELECT t.id FROM t LEFT JOIN t AS u ON missing=1 LIMIT 0 OFFSET NULL", "no such column"},
		{"left_explain_on_missing", "EXPLAIN QUERY PLAN SELECT t.id FROM t LEFT JOIN t AS u ON missing=1 LIMIT NULL", "no such column"},
		{"left_forward_reference", "SELECT t.id FROM t LEFT JOIN t AS u ON v.id=t.id LEFT JOIN t AS v ON v.id=u.id LIMIT NULL", "ON references a later FROM relation"},
		{"aggregate_null_missing", "SELECT count(*) FROM t LIMIT NULL OFFSET missing", "no such column"},
		{"aggregate_zero_arity", "SELECT count(*) FROM t LIMIT 0 OFFSET abs(1,2)", "wrong number of arguments"},
		{"grouped_overflow_missing", "SELECT k,count(*) FROM t GROUP BY k LIMIT abs(-9223372036854775808) OFFSET missing", "no such column"},
		{"grouped_explain_function", "EXPLAIN QUERY PLAN SELECT k,count(*) FROM t GROUP BY k LIMIT NULL OFFSET nosuch(1)", "function nosuch"},
	}
	if len(cases) != 30 {
		t.Fatal("binding case count changed")
	}
	for _, mode := range []string{"db", "tx"} {
		t.Run(mode, func(t *testing.T) {
			db, query, check := limitPhaseConnection(t, mode)
			before := db.counters
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					rows, err := query(context.Background(), tc.query)
					limitPhaseError(t, rows, err, CodeError, tc.part)
					check(t)
					if db.counters != before {
						t.Fatal("binding failure changed connection counters")
					}
				})
			}
		})
	}
}

func TestLimitPhaseGroupedAndConnectionPreservation(t *testing.T) {
	// 18 fixed cases per public API. Bounds read the same connection state as
	// projections, and SELECT/error/EXPLAIN must not change those counters.
	cases := []struct {
		name, query string
		want        [][]any
		explain     bool
		noRead      bool
	}{
		{"constant_zero_overflow", "SELECT 1 LIMIT 0 OFFSET abs(-9223372036854775808)", nil, false, true},
		{"single_zero_overflow", "SELECT id FROM t ORDER BY id LIMIT 0 OFFSET abs(-9223372036854775808)", nil, false, true},
		{"join_zero_overflow", "SELECT t.id FROM t LEFT JOIN t AS u ON t.id=u.id LIMIT 0 OFFSET abs(-9223372036854775808)", nil, false, true},
		{"aggregate_zero_overflow", "SELECT count(*) FROM t LIMIT 0 OFFSET abs(-9223372036854775808)", nil, false, true},
		{"grouped_zero_overflow", "SELECT k,count(*) FROM t GROUP BY k HAVING count(*)>0 LIMIT 0 OFFSET abs(-9223372036854775808)", nil, false, true},
		{"constant_explain", "EXPLAIN QUERY PLAN SELECT 1 LIMIT NULL OFFSET abs(-9223372036854775808)", nil, true, true},
		{"single_explain", "EXPLAIN QUERY PLAN SELECT id FROM t ORDER BY id LIMIT NULL OFFSET abs(-9223372036854775808)", nil, true, true},
		{"join_explain", "EXPLAIN QUERY PLAN SELECT t.id FROM t LEFT JOIN t AS u ON t.id=u.id LIMIT NULL OFFSET abs(-9223372036854775808)", nil, true, true},
		{"aggregate_explain", "EXPLAIN QUERY PLAN SELECT count(*) FROM t LIMIT NULL OFFSET abs(-9223372036854775808)", nil, true, true},
		{"grouped_explain", "EXPLAIN QUERY PLAN SELECT k,count(*) FROM t GROUP BY k HAVING count(*)>0 LIMIT NULL OFFSET abs(-9223372036854775808)", nil, true, true},
		{"counter_limit", "SELECT id FROM t ORDER BY id LIMIT changes()-1 OFFSET last_insert_rowid()-2", [][]any{{int64(2)}, {int64(3)}}, false, false},
		{"counter_join", "SELECT t.id FROM t LEFT JOIN t AS u ON t.id=u.id ORDER BY t.id LIMIT total_changes()-2 OFFSET changes()-2", [][]any{{int64(2)}}, false, false},
		{"counter_zero", "SELECT id FROM t LIMIT changes()-total_changes() OFFSET NULL", nil, false, true},
		{"global_discard", "SELECT count(*) FROM t LIMIT 1 OFFSET 1", nil, false, false},
		{"grouped_offset", "SELECT k,count(*) FROM t GROUP BY k ORDER BY k LIMIT 1 OFFSET 1", [][]any{{int64(2), int64(1)}}, false, false},
		{"grouped_having_topk", "SELECT k,count(*) AS n FROM t GROUP BY k HAVING count(*)>0 ORDER BY n DESC,k LIMIT 1", [][]any{{int64(1), int64(2)}}, false, false},
		{"grouped_rowid", "SELECT id,count(*) FROM t GROUP BY id LIMIT 1 OFFSET 1", [][]any{{int64(2), int64(1)}}, false, false},
		{"negative_bounds", "SELECT id FROM t ORDER BY id LIMIT -1 OFFSET -1", [][]any{{int64(1)}, {int64(2)}, {int64(3)}}, false, false},
	}
	if len(cases) != 18 {
		t.Fatal("preservation case count changed")
	}
	for _, mode := range []string{"db", "tx"} {
		t.Run(mode, func(t *testing.T) {
			db, query, check := limitPhaseConnection(t, mode)
			before := db.counters
			if before.Changes != 3 || before.TotalChanges != 3 || before.LastInsertRowID != 3 {
				t.Fatal("unexpected fixture counters", before)
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					rows, err := query(context.Background(), tc.query)
					if err != nil || rows == nil {
						t.Fatalf("Query: rows=%v error=%v", rows, err)
					}
					plan := rows.Plan()
					got := limitConversionCollect(t, rows)
					if tc.explain {
						if len(got) != 1 || len(got[0]) != 4 {
							t.Fatalf("invalid native plan rows: %#v", got)
						}
						if detail, ok := got[0][3].(string); !ok || detail == "" {
							t.Fatal("missing native plan detail")
						}
					} else if !reflect.DeepEqual(got, tc.want) {
						t.Fatalf("got %#v; want %#v", got, tc.want)
					}
					if tc.noRead && (rows.Stats().BaseRowFetches != 0 || rows.RelationalStats().SourceOpens != 0) {
						t.Fatal("suppressed execution read a base row", rows.Stats(), rows.RelationalStats())
					}
					if rows.RelationalStats().CurrentBytes != 0 || !reflect.DeepEqual(plan, rows.Plan()) {
						t.Fatal("closed Rows lost its plan or retained relational memory")
					}
					stats := rows.Stats()
					relational := rows.RelationalStats()
					if rows.Next() || rows.Close() != nil || stats != rows.Stats() || relational != rows.RelationalStats() {
						t.Fatal("closed Rows changed observable statistics")
					}
					check(t)
					if db.counters != before {
						t.Fatal("SELECT or EXPLAIN changed connection counters")
					}
				})
			}
		})
	}
}

func TestLimitPhaseResourceOwnership(t *testing.T) {
	for _, mode := range []string{"db", "tx"} {
		t.Run(mode, func(t *testing.T) {
			db, query, check := limitPhaseConnection(t, mode)
			before := db.counters
			canceled, cancel := context.WithCancel(context.Background())
			cancel()
			deadline, stop := context.WithDeadline(context.Background(), time.Time{})
			defer stop()
			// Six native context/admission failures. No native I/O is injected.
			for _, tc := range []struct {
				name, query string
				ctx         context.Context
				want        error
			}{
				{"canceled_zero", "SELECT 1 LIMIT 0 OFFSET NULL", canceled, context.Canceled},
				{"deadline_explain", "EXPLAIN QUERY PLAN SELECT 1 LIMIT NULL", deadline, context.DeadlineExceeded},
				{"join_zero_work", "SELECT t.id FROM t LEFT JOIN t AS u ON t.id=u.id LIMIT 0 OFFSET NULL", WithRelationalLimits(context.Background(), RelationalLimits{MaxWork: 1}), ErrQueryWorkLimit},
				{"join_explain_work", "EXPLAIN QUERY PLAN SELECT t.id FROM t LEFT JOIN t AS u ON t.id=u.id LIMIT NULL", WithRelationalLimits(context.Background(), RelationalLimits{MaxWork: 1}), ErrQueryWorkLimit},
				{"grouped_zero_memory", "SELECT k,count(*) FROM t GROUP BY k LIMIT 0 OFFSET NULL", WithRelationalLimits(context.Background(), RelationalLimits{MaxBytes: 1}), ErrMemoryLimit},
				{"grouped_explain_memory", "EXPLAIN QUERY PLAN SELECT k,count(*) FROM t GROUP BY k LIMIT NULL", WithRelationalLimits(context.Background(), RelationalLimits{MaxBytes: 1}), ErrMemoryLimit},
			} {
				t.Run(tc.name, func(t *testing.T) {
					rows, err := query(tc.ctx, tc.query)
					if rows != nil {
						rows.Close()
					}
					if rows != nil || !errors.Is(err, tc.want) || errors.Is(err, ErrMismatch) {
						t.Fatalf("rows=%v error=%v; want %v", rows, err, tc.want)
					}
					check(t)
				})
			}
			// Four explicit public parameter-admission failures are not waived.
			for _, tc := range []struct {
				name, query string
				args        []any
			}{
				{"zero_missing_argument", "SELECT 1 LIMIT 0 OFFSET ?1", nil},
				{"explain_missing_argument", "EXPLAIN QUERY PLAN SELECT 1 LIMIT ?1", nil},
				{"zero_extra_argument", "SELECT 1 LIMIT 0", []any{1}},
				{"explain_bad_argument", "EXPLAIN QUERY PLAN SELECT 1 LIMIT ?1", []any{struct{}{}}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					rows, err := query(context.Background(), tc.query, tc.args...)
					limitPhaseError(t, rows, err, CodeError, "")
					check(t)
				})
			}
			rows, err := query(context.Background(), "SELECT id FROM t ORDER BY id LIMIT 1")
			if err != nil {
				t.Fatal(err)
			}
			if got := limitConversionCollect(t, rows); !reflect.DeepEqual(got, [][]any{{int64(1)}}) {
				t.Fatal("post-error reuse", got)
			}
			check(t)
			if db.counters != before {
				t.Fatal("resource or argument errors changed connection counters")
			}
		})
	}
	// Direct name validation must still charge work, even for an unreachable
	// OFFSET. This is a descriptor-free logical budget, not a physical hook.
	t.Run("binding_work", func(t *testing.T) {
		budget := &relationalBudget{ctx: context.Background(), maxWork: 1, maxBytes: 1024}
		err := relationalValidate(&sqlast.Column{Name: "missing"}, nil, budget)
		if !errors.Is(err, ErrQueryWorkLimit) || errors.Is(err, ErrMismatch) {
			t.Fatal(err)
		}
	})
}
