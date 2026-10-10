package sqlite

import (
	"context"
	"fmt"
	"io"
	"sort"
	"sync"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

type orderExpr struct {
	expr   sqlast.Expr
	column int
	desc   bool
}
type resultRow struct{ values, keys []any }

// Rows streams a SELECT result unless ORDER BY requires bounded sorting. It
// owns no goroutine. Values returns copies safe to retain after the next row.
// Next, Values, Err and Close are synchronized, but callers must not interleave
// multiple consumers because their logical Next/Values pair is not atomic.
type Rows struct {
	mu                              sync.Mutex
	db                              *DB
	ctx                             context.Context
	table                           *table
	sel                             *sqlast.Select
	params                          []any
	exprs                           []sqlast.Expr
	columns                         []string
	aliases                         map[string]int
	order                           []orderExpr
	source                          source
	limit, offset, emitted, skipped int64
	sorted                          []resultRow
	sortIndex                       int
	current                         []any
	err                             error
	closed                          bool
}

func (r *Rows) Columns() []string { return append([]string(nil), r.columns...) }
func (r *Rows) Values() []any {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == nil {
		return nil
	}
	result := make([]any, len(r.current))
	for i, v := range r.current {
		result[i] = cloneValue(v)
	}
	return result
}
func cloneValue(v any) any {
	if b, ok := v.([]byte); ok {
		return append([]byte{}, b...)
	}
	return v
}
func (r *Rows) Err() error   { r.mu.Lock(); defer r.mu.Unlock(); return r.err }
func (r *Rows) Close() error { r.mu.Lock(); defer r.mu.Unlock(); return r.close() }
func (r *Rows) close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	r.current = nil
	r.sorted = nil
	r.params = nil
	if r.source != nil {
		err := r.source.Close()
		r.source = nil
		return wrap("rows close", err)
	}
	return nil
}
func (r *Rows) fail(err error) bool {
	if err != io.EOF {
		r.err = wrap("next", err)
	}
	r.close()
	return false
}

func (r *Rows) Next() bool {
	r.db.mu.RLock()
	defer r.db.mu.RUnlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return false
	}
	if r.db.closed {
		return r.fail(ErrClosed)
	}
	if err := r.ctx.Err(); err != nil {
		return r.fail(err)
	}
	if r.limit >= 0 && r.emitted >= r.limit {
		return r.fail(io.EOF)
	}
	for {
		var row resultRow
		if len(r.order) > 0 {
			if r.sortIndex >= len(r.sorted) {
				return r.fail(io.EOF)
			}
			row = r.sorted[r.sortIndex]
			r.sorted[r.sortIndex] = resultRow{}
			r.sortIndex++
		} else {
			var err error
			row, _, err = r.fetch()
			if err != nil {
				return r.fail(err)
			}
		}
		if r.skipped < r.offset {
			r.skipped++
			continue
		}
		r.current = row.values
		r.emitted++
		return true
	}
}

func (r *Rows) fetch() (resultRow, int64, error) {
	for {
		if err := r.ctx.Err(); err != nil {
			return resultRow{}, 0, err
		}
		rowid, values, err := r.source.Next()
		if err != nil {
			return resultRow{}, 0, err
		}
		rawCost := valuesSize(values)
		if r.table != nil {
			if len(values) > len(r.table.columns) {
				return resultRow{}, 0, fmt.Errorf("%w: record has too many columns", ErrCorrupt)
			}
			for len(values) < len(r.table.columns) {
				c := r.table.columns[len(values)]
				var v any
				if c.Default != nil {
					if !isConstantDefault(c.Default) {
						return resultRow{}, 0, fmt.Errorf("%w: missing column has a non-literal default", ErrUnsupported)
					}
					v, err = sqlast.Eval(c.Default, &sqlast.EvalContext{})
					if err != nil {
						return resultRow{}, 0, err
					}
					v, err = sqlast.ApplyAffinity(v, affinity(c.Type))
					if err != nil {
						return resultRow{}, 0, err
					}
				}
				values = append(values, v)
			}
			if r.table.ipk >= 0 {
				values[r.table.ipk] = rowid
			}
			for i, c := range r.table.columns {
				if affinity(c.Type) == "REAL" {
					if n, ok := values[i].(int64); ok {
						values[i] = float64(n)
					}
				}
			}
		}
		ec := &sqlast.EvalContext{Params: r.params}
		ec.Resolver = func(q, name string) (any, error) {
			i, err := r.table.column(q, name)
			if err != nil {
				return nil, err
			}
			if i < 0 {
				return rowid, nil
			}
			return values[i], nil
		}
		ec.Affinity = func(q, name string) string {
			i, err := r.table.column(q, name)
			if err != nil {
				return ""
			}
			if i < 0 {
				return "INTEGER"
			}
			return affinity(r.table.columns[i].Type)
		}
		// WHERE accepts result aliases only when no physical column shadows
		// them. Their expressions were already bound against the source.
		physicalResolver, physicalAffinity := ec.Resolver, ec.Affinity
		ec.Resolver = func(q, name string) (any, error) {
			value, err := physicalResolver(q, name)
			if err == nil || q != "" {
				return value, err
			}
			if i, ok := r.aliases[fold(name)]; ok {
				return sqlast.Eval(r.exprs[i], &sqlast.EvalContext{Params: r.params, Resolver: physicalResolver, Affinity: physicalAffinity})
			}
			return nil, err
		}
		ec.Affinity = func(q, name string) string {
			if _, err := r.table.column(q, name); err == nil {
				return physicalAffinity(q, name)
			}
			if q == "" {
				if i, ok := r.aliases[fold(name)]; ok {
					if c, ok := r.exprs[i].(*sqlast.Column); ok {
						return physicalAffinity(c.Table, c.Name)
					}
				}
			}
			return ""
		}
		if r.sel.Where != nil {
			value, err := sqlast.Eval(r.sel.Where, ec)
			if err != nil {
				return resultRow{}, 0, err
			}
			truth, isNull := sqlast.Truth(value)
			if isNull || !truth {
				continue
			}
		}
		row := resultRow{values: make([]any, len(r.exprs))}
		for i, e := range r.exprs {
			row.values[i], err = sqlast.Eval(e, ec)
			if err != nil {
				return resultRow{}, 0, err
			}
		}
		if len(r.order) > 0 {
			// Only a bare ORDER BY alias takes precedence over source
			// columns (resolved to a result position by the planner). Nested
			// expressions bind source columns first, then result aliases.
			ec.Resolver = func(q, name string) (any, error) {
				v, err := physicalResolver(q, name)
				if err == nil || q != "" {
					return v, err
				}
				if i, ok := r.aliases[fold(name)]; ok {
					return row.values[i], nil
				}
				return nil, err
			}

			row.keys = make([]any, len(r.order))
			for i, o := range r.order {
				if o.column >= 0 {
					row.keys[i] = row.values[o.column]
				} else {
					row.keys[i], err = sqlast.Eval(o.expr, ec)
					if err != nil {
						return resultRow{}, 0, err
					}
				}
			}
		}
		// Charge the original record too: derived substrings can retain its backing
		// storage. Over-counting is deliberate, so the quota fails conservatively.
		return row, 192 + 2*(rawCost+valuesSize(row.values)+valuesSize(row.keys)), nil
	}
}

func valuesSize(values []any) int64 {
	size := int64(len(values)) * 16
	for _, v := range values {
		switch x := v.(type) {
		case string:
			size += int64(len(x))
		case []byte:
			size += int64(len(x))
		}
	}
	return size
}

func (r *Rows) materialize() error {
	var used int64
	for {
		row, cost, err := r.fetch()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if cost > r.db.opts.MaxSortBytes-used {
			return ErrMemoryLimit
		}
		used += cost
		r.sorted = append(r.sorted, row)
	}
	if err := r.source.Close(); err != nil {
		return err
	}
	r.source = nil
	var compareErr error
	comparisons := 0
	sort.SliceStable(r.sorted, func(i, j int) bool {
		comparisons++
		if comparisons%1024 == 0 && compareErr == nil {
			compareErr = r.ctx.Err()
		}
		for k, o := range r.order {
			cmp, err := sqlast.Compare(r.sorted[i].keys[k], r.sorted[j].keys[k])
			if err != nil && compareErr == nil {
				compareErr = err
			}
			if cmp != 0 {
				if o.desc {
					return cmp > 0
				}
				return cmp < 0
			}
		}
		return false
	})
	if compareErr != nil {
		return compareErr
	}
	return r.ctx.Err()
}
