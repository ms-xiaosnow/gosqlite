package sqlite

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// These controls concern conversion of an already evaluated bound. They do not
// assert LIMIT-zero suppression, EXPLAIN execution, or bound-name ordering.
type limitConversionCase struct {
	name  string
	value any
	want  int64
	valid bool
}

func limitConversionCases() []limitConversionCase {
	return []limitConversionCase{
		{"integer_min", int64(math.MinInt64), math.MinInt64, true},
		{"integer_max", int64(math.MaxInt64), math.MaxInt64, true},
		{"integer_one", int64(1), 1, true},
		{"integer_negative", int64(-1), -1, true},
		{"integer_zero", int64(0), 0, true},
		{"go_int", int(1), 1, true},
		{"go_uint32", uint32(1), 1, true},
		{"go_bool", true, 1, true},
		{"go_float32", float32(1), 1, true},
		{"real_one", float64(1), 1, true},
		{"real_negative_zero", math.Copysign(0, -1), 0, true},
		{"real_lower_neighbor", math.Nextafter(float64(math.MinInt64), 0), -9223372036854774784, true},
		{"real_upper_neighbor", math.Nextafter(9223372036854775808.0, 0), 9223372036854774784, true},
		{"text_min", "-9223372036854775808", math.MinInt64, true},
		{"text_max", "9223372036854775807", math.MaxInt64, true},
		{"text_exact_large_integer", "9007199254740993", 9007199254740993, true},
		{"text_ascii_space", " \t\n\r\v\f+1 \t\n\r\v\f", 1, true},
		{"text_leading_zero", "0001", 1, true},
		{"text_decimal", "1.0", 1, true},
		{"text_trailing_point", "1.", 1, true},
		{"text_leading_point", ".1e1", 1, true},
		{"text_exponent", "+1e+0", 1, true},
		{"text_negative_exponent", "10E-1", 1, true},
		{"text_underflow", "1e-999", 0, true},
		{"text_negative_underflow", "-1e-999", 0, true},
		{"nul_one", "1\x00junk", 1, true},
		{"nul_exponent", "1e0\x00junk", 1, true},
		{"nul_whitespace", " \t1\r\x00junk", 1, true},
		{"nul_precision_round", "9007199254740993\x00", 9007199254740992, true},
		{"nul_underflow", "1e-999\x00tail", 0, true},
		{"null", nil, 0, false},
		{"blob_one", []byte("1"), 0, false},
		{"blob_empty", []byte{}, 0, false},
		{"real_fraction", 1.5, 0, false},
		{"real_subnormal", math.SmallestNonzeroFloat64, 0, false},
		{"real_nan", math.NaN(), 0, false},
		{"real_positive_infinity", math.Inf(1), 0, false},
		{"real_negative_infinity", math.Inf(-1), 0, false},
		{"real_min_endpoint", float64(math.MinInt64), 0, false},
		{"real_max_rounded_endpoint", float64(math.MaxInt64), 0, false},
		{"text_min_real_endpoint", "-9223372036854775808.0", 0, false},
		{"text_below_min", "-9223372036854775809", 0, false},
		{"text_above_max", "9223372036854775808", 0, false},
		{"text_overflow", "1e999", 0, false},
		{"text_fraction", "1.25", 0, false},
		{"text_empty", "", 0, false},
		{"text_spaces", " \t", 0, false},
		{"text_sign", "+", 0, false},
		{"text_dot", ".", 0, false},
		{"text_incomplete_exponent", "1e+", 0, false},
		{"text_suffix", "1junk", 0, false},
		{"text_hex", "0x1p0", 0, false},
		{"text_underscore", "1_0", 0, false},
		{"text_unicode_leading_space", "\u00a01", 0, false},
		{"text_unicode_trailing_space", "1\u2003", 0, false},
		{"text_nan", "NaN", 0, false},
		{"text_infinity", "Inf", 0, false},
		{"nul_empty_prefix", "\x001", 0, false},
		{"nul_bad_prefix", "bad\x001", 0, false},
		{"nul_fraction", "1.5\x00", 0, false},
		{"nul_hex", "0x1p0\x00", 0, false},
		{"nul_min_endpoint", "-9223372036854775808\x00", 0, false},
		{"nul_max_endpoint", "9223372036854775807\x00", 0, false},
	}
}

func TestLimitConversionTypes(t *testing.T) {
	for _, tc := range limitConversionCases() {
		t.Run(tc.name, func(t *testing.T) {
			got, err := limitValue(context.Background(), &sqlast.Parameter{Index: 1}, []any{tc.value})
			if tc.valid {
				if err != nil || got != tc.want {
					t.Fatalf("value=%#v: got %d, %v; want %d", tc.value, got, err, tc.want)
				}
			} else if got != 0 || !errors.Is(err, ErrMismatch) {
				t.Fatalf("value=%#v: got %d, %v; want zero and ErrMismatch", tc.value, got, err)
			}
		})
	}
}

// A pure scalar quota observer: no I/O hooks or physical-failure injection.
type limitConversionBudget struct {
	steps, allocated     int64
	failStep             int64
	stepErr, allocateErr error
}

func (b *limitConversionBudget) Step(n int64) error {
	b.steps++
	if b.steps == b.failStep {
		return b.stepErr
	}
	return nil
}
func (b *limitConversionBudget) Allocate(n int64) error {
	b.allocated += n
	return b.allocateErr
}

func TestLimitConversionErrorOwnership(t *testing.T) {
	expr := &sqlast.Parameter{Index: 1}
	for _, tc := range []struct {
		name   string
		budget limitConversionBudget
		want   error
	}{
		{"eval_work", limitConversionBudget{failStep: 1, stepErr: ErrQueryWorkLimit}, ErrQueryWorkLimit},
		{"preflight_work", limitConversionBudget{failStep: 2, stepErr: ErrQueryWorkLimit}, ErrQueryWorkLimit},
		{"preflight_memory", limitConversionBudget{allocateErr: ErrMemoryLimit}, ErrMemoryLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := tc.budget
			ctx := sqlast.WithEvalBudget(context.Background(), &b)
			_, err := limitValue(ctx, expr, []any{"bad"})
			if err != tc.want || errors.Is(err, ErrMismatch) {
				t.Fatalf("got %v; want unchanged %v", err, tc.want)
			}
		})
	}
	t.Run("full_nul_value_preflight", func(t *testing.T) {
		value := "1\x00" + strings.Repeat("suffix", 32)
		b := &limitConversionBudget{allocateErr: ErrMemoryLimit}
		_, err := limitValue(sqlast.WithEvalBudget(context.Background(), b), expr, []any{value})
		if err != ErrMemoryLimit || b.allocated != 128+8*int64(len(value)) || b.steps != 1 {
			t.Fatalf("full-value preflight lost: err=%v allocated=%d steps=%d", err, b.allocated, b.steps)
		}
	})
	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := limitValue(ctx, expr, []any{nil})
		if err != context.Canceled || errors.Is(err, ErrMismatch) {
			t.Fatal(err)
		}
	})
	t.Run("deadline", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Time{})
		defer cancel()
		_, err := limitValue(ctx, expr, []any{nil})
		if err != context.DeadlineExceeded || errors.Is(err, ErrMismatch) {
			t.Fatal(err)
		}
	})
	for _, tc := range []struct {
		name   string
		expr   sqlast.Expr
		params []any
		part   string
	}{
		{"unbound", expr, nil, "unbound"},
		{"unsupported_value", expr, []any{struct{}{}}, "unsupported SQL value type"},
		{"overflow", &sqlast.Call{Name: "abs", Args: []sqlast.Expr{&sqlast.Literal{Value: int64(math.MinInt64)}}}, nil, "integer overflow"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := limitValue(context.Background(), tc.expr, tc.params)
			if err == nil || errors.Is(err, ErrMismatch) || !strings.Contains(err.Error(), tc.part) {
				t.Fatal(err)
			}
		})
	}
}

const limitConversionSeed = "CREATE TABLE t(id INTEGER PRIMARY KEY,k); INSERT INTO t VALUES(1,1),(2,1),(3,2);"

func limitConversionFixture(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "conversion.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, q := range strings.Split(limitConversionSeed, ";") {
		if strings.TrimSpace(q) == "" {
			continue
		}
		if _, err := db.Exec(context.Background(), q); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

type limitConversionShape struct {
	name, query string
	rows        [][]any
}

func limitConversionShapes() []limitConversionShape {
	return []limitConversionShape{
		{"single", "SELECT id FROM t ORDER BY id", [][]any{{int64(1)}, {int64(2)}, {int64(3)}}},
		{"join", "SELECT t.id FROM t LEFT JOIN t AS u ON t.id=u.id ORDER BY t.id", [][]any{{int64(1)}, {int64(2)}, {int64(3)}}},
		{"aggregate", "SELECT count(*) FROM t", [][]any{{int64(3)}}},
		{"grouped", "SELECT k,count(*) FROM t GROUP BY k ORDER BY k", [][]any{{int64(1), int64(2)}, {int64(2), int64(1)}}},
	}
}

func limitConversionCollect(t *testing.T, rows *Rows) [][]any {
	t.Helper()
	defer rows.Close()
	var got [][]any
	for rows.Next() {
		got = append(got, rows.Values())
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	return got
}

func limitConversionExpected(base [][]any, bound int64, offset bool) [][]any {
	start, end := 0, len(base)
	if offset {
		if bound > int64(end) {
			start = end
		} else if bound > 0 {
			start = int(bound)
		}
		end = min(end, start+1)
	} else if bound >= 0 && bound < int64(end) {
		end = int(bound)
	}
	if start == end {
		return nil
	}
	return base[start:end]
}

func TestLimitConversionPublicSQL(t *testing.T) {
	for _, mode := range []string{"db", "tx"} {
		t.Run(mode, func(t *testing.T) {
			db := limitConversionFixture(t)
			query := db.Query
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
			for _, shape := range limitConversionShapes() {
				for _, offset := range []bool{false, true} {
					clause := "limit"
					suffix := " LIMIT ?1"
					if offset {
						clause, suffix = "offset", " LIMIT 1 OFFSET ?1"
					}
					for _, tc := range limitConversionCases() {
						// The public slice intentionally has no zero-valued valid bounds.
						if tc.valid && tc.want == 0 {
							continue
						}
						t.Run(shape.name+"/"+clause+"/"+tc.name, func(t *testing.T) {
							rows, err := query(context.Background(), shape.query+suffix, tc.value)
							if tc.valid {
								if err != nil || rows == nil {
									t.Fatalf("rows=%v err=%v", rows, err)
								}
								got := limitConversionCollect(t, rows)
								want := limitConversionExpected(shape.rows, tc.want, offset)
								if !reflect.DeepEqual(got, want) {
									t.Fatalf("got %#v; want %#v", got, want)
								}
							} else {
								var typed *Error
								nonnil := rows != nil
								if nonnil {
									if e := rows.Close(); e != nil {
										t.Error(e)
									}
								}
								if nonnil || !errors.Is(err, ErrMismatch) || !errors.As(err, &typed) || typed.Code != CodeMismatch {
									t.Errorf("rows nonnil=%t err=%v; want nil Rows, ErrMismatch and code 20", nonnil, err)
								}
							}
							if len(db.rows) != 0 || tx != nil && tx.activeRows != 0 {
								t.Fatalf("leaked rows: db=%d tx=%v", len(db.rows), tx)
							}
						})
					}
					t.Run(shape.name+"/"+clause+"/overflow", func(t *testing.T) {
						rows, err := query(context.Background(), shape.query+strings.ReplaceAll(suffix, "?1", "abs(-9223372036854775808)"))
						var typed *Error
						nonnil := rows != nil
						if nonnil {
							rows.Close()
						}
						if nonnil || !errors.As(err, &typed) || typed.Code != CodeError || errors.Is(err, ErrMismatch) || !strings.Contains(err.Error(), "integer overflow") {
							t.Fatalf("rows nonnil=%t err=%v", nonnil, err)
						}
						if len(db.rows) != 0 || tx != nil && tx.activeRows != 0 {
							t.Fatal("overflow leaked Rows lease")
						}
					})
				}
			}
			// A final successful public query also checks that refusals left the owner usable.
			rows, err := query(context.Background(), "SELECT id FROM t ORDER BY id LIMIT 1 OFFSET 1")
			if err != nil {
				t.Fatal(err)
			}
			if got := limitConversionCollect(t, rows); !reflect.DeepEqual(got, [][]any{{int64(2)}}) {
				t.Fatal(got)
			}
			if len(db.rows) != 0 || tx != nil && tx.activeRows != 0 {
				t.Fatal("positive control leaked Rows lease")
			}
		})
	}
}
