package sqlite

import (
	"context"
	"errors"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
	"runtime"
	"testing"
)

// Descriptor-free admission checks, using caller-owned BLOB/Valuer inputs.
func TestGlobalAggregateParameterAdmission(t *testing.T) {
	for _, q := range []string{"SELECT count(?)", "SELECT min(?) FROM a", "SELECT max(?) FROM a,b"} {
		stmt, err := sqlast.Parse(q)
		if err != nil {
			t.Fatal(err)
		}
		data := make([]byte, 8<<20)
		for _, convert := range []bool{false, true} {
			calls := 0
			var arg any = data
			if convert {
				arg = relationalBlobValuer{data, &calls}
			}
			db := &DB{opts: Options{MaxSortBytes: 64 << 20}}
			ctx := WithRelationalLimits(context.Background(), RelationalLimits{MaxBytes: 1 << 20})
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			_, _, err := db.bindSelect(ctx, stmt.(*sqlast.Select), []any{arg})
			runtime.ReadMemStats(&after)
			if !errors.Is(err, ErrMemoryLimit) {
				t.Fatal(q, err)
			}
			if after.TotalAlloc-before.TotalAlloc >= uint64(len(data))/2 {
				t.Fatal("parameter cloned before admission", q, after.TotalAlloc-before.TotalAlloc)
			}
			if convert && calls != 1 {
				t.Fatal(calls)
			}
		}
	}
}

// Synthetic pull input is memory-only; cancellation is the ordinary statement
// context, not a native I/O/close failure or database mutation.
type aggregateCancelInput struct {
	next, closed int
	cancel       context.CancelFunc
}

func (s *aggregateCancelInput) Next() (int64, []any, error) {
	s.next++
	if s.next == 3 {
		s.cancel()
	}
	return 0, []any{int64(s.next)}, nil
}
func (s *aggregateCancelInput) Close() error { s.closed++; return nil }
func TestGlobalAggregateMidPullCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b, err := newRelationalBudget(ctx, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	ctx = sqlast.WithEvalBudget(ctx, b)
	src := &aggregateCancelInput{cancel: cancel}
	child := &Rows{ctx: ctx, table: &table{columns: []sqlast.ColumnDef{{Name: "x"}}, index: map[string]int{"x": 0}, ipk: -1}, source: src, exprs: []sqlast.Expr{&sqlast.Column{Name: "x"}}, relational: b}
	op := &aggregateSource{child: child, states: []sqlast.Aggregate{{Name: "sum"}}, retained: make([]int64, 1), budget: b}
	if _, _, err := op.Next(); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if src.next != 3 {
		t.Fatal(src.next)
	}
	if err := op.Close(); err != nil {
		t.Fatal(err)
	}
	if err := op.Close(); err != nil {
		t.Fatal(err)
	}
	if src.closed != 1 {
		t.Fatal(src.closed)
	}
}
func TestGlobalAggregateMinMaxCloneAdmission(t *testing.T) {
	data := make([]byte, 8<<20)
	for _, fn := range []string{"min", "max"} {
		b, err := newRelationalBudget(context.Background(), 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		ctx := sqlast.WithEvalBudget(context.Background(), b)
		op := &aggregateSource{child: &Rows{ctx: ctx}, states: []sqlast.Aggregate{{Name: fn}}, retained: make([]int64, 1), budget: b}
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		err = op.step(0, data)
		runtime.ReadMemStats(&after)
		if !errors.Is(err, ErrMemoryLimit) || op.states[0].RetainedValue() != nil {
			t.Fatal(fn, err)
		}
		if after.TotalAlloc-before.TotalAlloc >= uint64(len(data))/2 {
			t.Fatal("MIN/MAX cloned before admission", after.TotalAlloc-before.TotalAlloc)
		}
	}
}
