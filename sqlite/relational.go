package sqlite

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

type relationalFactory func() (source, error)

// Leaf and join values are borrowed until their next Next/Close. The only
// snapshot owner is Rows. A right-hand restart only creates a new cursor on the
// original PageReader, with the original context and cumulative root budget.
type relationalLeaf struct {
	ctx      context.Context
	base     btree.PageReader
	binding  relationalBinding
	budget   *relationalBudget
	stats    *QueryStats
	reader   *relationalReader
	cursor   source
	reverse  bool
	rowBytes int64
	closed   bool
}

func (s *relationalLeaf) Next() (int64, []any, error) {
	if s.closed {
		return 0, nil, io.EOF
	}
	if err := s.budget.step(1); err != nil {
		return 0, nil, err
	}
	if s.cursor == nil {
		s.reader = &relationalReader{base: s.base, budget: s.budget}
		beforeRecord := func(payload int) error {
			// A one-byte serial-type can yield a 16-byte interface value. This
			// includes decode slices, payload copies, default/IPK expansion and the
			// current normalized tuple before any of those allocations occur.
			if err := s.budget.step(int64(payload)); err != nil {
				return err
			}
			cost := int64(payload)*32 + int64(len(s.binding.table.columns)+1)*32 + 512
			if err := s.budget.reserve(cost); err != nil {
				return err
			}
			s.rowBytes += cost
			return nil
		}
		var c source
		var err error
		if s.reverse {
			c, err = btree.NewRowIDRangeCursor(s.ctx, &relationalDirectionalReader{relationalReader: s.reader, beforeRecord: beforeRecord}, s.binding.table.root, nil, nil, true, btree.RangeLimits{})
		} else {
			c, err = btree.NewBudgetCursor(s.ctx, s.reader, s.binding.table.root, beforeRecord)
		}
		if err != nil {
			return 0, nil, err
		}
		s.cursor = c
		s.budget.stats.SourceOpens++
	}
	s.budget.release(s.rowBytes)
	s.rowBytes = 0
	rowid, values, err := s.cursor.Next()
	if err != nil {
		return 0, nil, err
	}
	s.stats.BaseRowFetches++
	s.budget.stats.SourceRows++
	t := s.binding.table
	if len(values) > len(t.columns) {
		return 0, nil, fmt.Errorf("%w: record has too many columns", ErrCorrupt)
	}
	// Type-name normalization is byte-dependent even for ordinary columns.
	for _, c := range t.columns {
		if err := s.Step(1 + 8*int64(len(c.Type))); err != nil {
			return 0, nil, err
		}
		if err := s.Allocate(4 * int64(len(c.Type))); err != nil {
			return 0, nil, err
		}
	}
	out := make([]any, len(t.columns)+1)
	copy(out, values)
	for i := len(values); i < len(t.columns); i++ {
		c := t.columns[i]
		if c.Default != nil {
			if err := sqlast.Walk(c.Default, func(e sqlast.Expr) error { return s.Step(relationalNodeWork(e)) }); err != nil {
				return 0, nil, err
			}
			if !isConstantDefault(c.Default) {
				return 0, nil, fmt.Errorf("%w: missing column has a non-literal default", ErrUnsupported)
			}
			defaultCtx := sqlast.WithEvalBudget(s.ctx, s)
			v, err := sqlast.Eval(c.Default, &sqlast.EvalContext{Context: defaultCtx})
			if err != nil {
				return 0, nil, err
			}
			// The normalized default remains part of this leaf row, not ON scratch.
			if err := s.Step(valuesSize([]any{v}) * 128); err != nil {
				return 0, nil, err
			}
			if err := s.Allocate(valuesSize([]any{v})*128 + 4096); err != nil {
				return 0, nil, err
			}
			v, err = missingColumnValue(v, c.Type)
			if err != nil {
				return 0, nil, err
			}
			out[i] = v
		}
	}
	if t.ipk >= 0 {
		out[t.ipk] = rowid
	}
	for i, c := range t.columns {
		if affinity(c.Type) == "REAL" {
			if n, ok := out[i].(int64); ok {
				out[i] = float64(n)
			}
		}
	}
	out[len(t.columns)] = rowid
	return 0, out, nil
}
func (s *relationalLeaf) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	var err error
	if s.cursor != nil {
		err = s.cursor.Close()
		s.cursor = nil
	}
	if s.reader != nil {
		s.reader.close()
		s.reader = nil
	}
	s.budget.release(s.rowBytes)
	s.rowBytes = 0
	return err
}

type relationalJoin struct {
	left, right           source
	rightFactory          relationalFactory
	leftWidth, rightWidth int
	kind                  string
	on                    *predicateProgram
	rows                  *Rows
	budget                *relationalBudget
	outer                 []any
	matched               bool
	outputBytes           int64
	closed                bool
}

func (s *relationalJoin) Next() (int64, []any, error) {
	if s.closed {
		return 0, nil, io.EOF
	}
	s.budget.release(s.outputBytes)
	s.outputBytes = 0
	for {
		if err := s.budget.step(1); err != nil {
			return 0, nil, err
		}
		if s.outer == nil {
			_, row, err := s.left.Next()
			if err != nil {
				return 0, nil, err
			}
			s.outer = row
			s.matched = false
			s.right, err = s.rightFactory()
			if err != nil {
				return 0, nil, err
			}
		}
		_, inner, err := s.right.Next()
		if err == io.EOF {
			if e := s.right.Close(); e != nil {
				return 0, nil, e
			}
			s.right = nil
			if s.kind == "LEFT" && !s.matched {
				out, e := s.tuple(nil)
				if e != nil {
					return 0, nil, e
				}
				s.outer = nil
				s.budget.stats.NullExtensions++
				return 0, out, nil
			}
			s.outer = nil
			continue
		}
		if err != nil {
			return 0, nil, err
		}
		s.budget.stats.JoinPairs++
		out, err := s.tuple(inner)
		if err != nil {
			return 0, nil, err
		}
		keep := true
		if s.on != nil {
			match, e := s.rows.relationalPredicate(s.on, out)
			if e != nil {
				return 0, nil, e
			}
			keep = match
		}
		if keep {
			s.matched = true
			return 0, out, nil
		}
		s.budget.release(s.outputBytes)
		s.outputBytes = 0
	}
}
func (s *relationalJoin) tuple(inner []any) ([]any, error) {
	cost := int64(s.leftWidth+s.rightWidth)*16 + 64
	if err := s.budget.reserve(cost); err != nil {
		return nil, err
	}
	s.outputBytes = cost
	out := make([]any, s.leftWidth+s.rightWidth)
	copy(out, s.outer)
	copy(out[s.leftWidth:], inner)
	return out, nil
}
func (s *relationalJoin) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	var err error
	if s.left != nil {
		err = s.left.Close()
		s.left = nil
	}
	if s.right != nil {
		err = errors.Join(err, s.right.Close())
		s.right = nil
	}
	s.outer = nil
	s.budget.release(s.outputBytes)
	s.outputBytes = 0
	return err
}

func (r *Rows) relationalPredicate(p *predicateProgram, values []any) (bool, error) {
	defer r.relational.clearScalar()
	ec := &sqlast.EvalContext{Context: r.ctx, Params: r.params, Functions: r.functions, DeclaredType: declaredTypeFor(r.table)}
	ec.Resolver = func(q, name string) (any, error) {
		i, err := r.table.column(q, name)
		if err != nil {
			return nil, err
		}
		if i < 0 || i >= len(values) {
			return nil, fmt.Errorf("%w: bound slot outside join scope", ErrUnsupported)
		}
		return r.relationalColumnValue(values[i], r.table.columns[i].Type)
	}
	ec.Affinity = func(q, name string) string {
		i, err := r.table.column(q, name)
		if err != nil || i < 0 {
			return ""
		}
		return affinity(r.table.columns[i].Type)
	}
	return p.evaluate(ec)
}

// Validation of a referenced typed value is lazy, but charged before its codec
// allocates scratch. Unreferenced and unselected typed cells remain untouched.
func (r *Rows) relationalColumnValue(value any, declared string) (any, error) {
	if value != nil && sqlast.IsVectorDeclaration(declared) {
		cost := valuesSize([]any{value})*128 + 512
		if err := r.relational.Allocate(cost); err != nil {
			return nil, err
		}
		if err := r.relational.step(valuesSize([]any{value})); err != nil {
			return nil, err
		}
	}
	return columnValue(value, declared)
}

// Defaults may produce retained bytes; keep their reservations with the leaf
// row rather than clearing them with the next ON/projection scratch scope.
func (s *relationalLeaf) Step(n int64) error { return s.budget.step(n) }
func (s *relationalLeaf) Allocate(n int64) error {
	if err := s.budget.reserve(n); err != nil {
		return err
	}
	s.rowBytes += n
	return nil
}

// Preserve the leaf's per-record preflight when the existing bounded rowid
// cursor wraps its reader. Page reservations still belong to the root budget.
type relationalDirectionalReader struct {
	*relationalReader
	beforeRecord func(int) error
}

func (r *relationalDirectionalReader) BeforeRecord(n int) error { return r.beforeRecord(n) }
