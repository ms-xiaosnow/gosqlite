package sqlite

import (
	"context"
	"fmt"
	"io"
	"math"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// rowIDInterval is a conjunction of cuts in signed integer space. SQL NULL
// and nonnumeric storage classes are resolved before constructing a cursor;
// physical ordering alone is not SQL's three-valued comparison semantics.
type rowIDInterval struct {
	lower, upper *btree.RowIDBound
	seen, empty  bool
}

func (b *rowIDInterval) add(op string, v any) {
	b.seen = true
	if v == nil {
		b.empty = true
		return
	}
	var id int64
	switch x := v.(type) {
	case int64:
		id = x
	case float64:
		if math.IsNaN(x) {
			b.empty = true
			return
		}
		// These comparisons are against exactly representable powers of two.
		// Never cast +2^63 to int64 or round an int64 endpoint through REAL.
		if x < -9223372036854775808.0 {
			b.empty = b.empty || op == "<" || op == "<="
			return
		}
		if x >= 9223372036854775808.0 {
			b.empty = b.empty || op == ">" || op == ">="
			return
		}
		if op == ">=" || op == "<" {
			id = int64(math.Ceil(x))
		} else {
			id = int64(math.Floor(x))
		}
	default:
		// After NUMERIC comparison affinity, remaining TEXT/BLOB sort after
		// every rowid. Parameters/literals have already been validated.
		b.empty = b.empty || op == ">" || op == ">="
		return
	}
	cut := &btree.RowIDBound{Value: id, Inclusive: op == ">=" || op == "<="}
	if op == ">" || op == ">=" {
		if b.lower == nil || id > b.lower.Value || id == b.lower.Value && !cut.Inclusive {
			b.lower = cut
		}
	} else if b.upper == nil || id < b.upper.Value || id == b.upper.Value && !cut.Inclusive {
		b.upper = cut
	}
}

func rowIDBounds(ctx context.Context, r *Rows, e sqlast.Expr, out *rowIDInterval) {
	x, ok := e.(*sqlast.Binary)
	if !ok {
		return
	}
	if x.Op == "AND" {
		rowIDBounds(ctx, r, x.Left, out)
		rowIDBounds(ctx, r, x.Right, out)
		return
	}
	op := x.Op
	if op != "<" && op != "<=" && op != ">" && op != ">=" {
		return
	}
	c, ok := x.Left.(*sqlast.Column)
	constant := x.Right
	if !ok {
		c, ok = x.Right.(*sqlast.Column)
		constant = x.Left
		switch op {
		case "<":
			op = ">"
		case "<=":
			op = ">="
		case ">":
			op = "<"
		case ">=":
			op = "<="
		}
	}
	if !ok || !ordinaryConstant(constant) {
		return
	}
	ordinal, ok := ordinaryColumn(r, c)
	if !ok || ordinal != -1 {
		return
	}
	v, err := sqlast.Eval(constant, &sqlast.EvalContext{Context: ctx, Params: r.params})
	if err != nil {
		return // Preserve the ordinary evaluator's error order.
	}
	v, err = sqlast.ApplyAffinity(v, "NUMERIC")
	if err == nil {
		out.add(op, v)
	}
}

// rowIDOrder proves one unique, non-NULL physical rowid ordering. Result
// positions/aliases have already been resolved with ORDER BY precedence.
// Reject error-capable projections: eliding an eager sort must not suppress
// an error in a later row merely because LIMIT stops the new stream early.
func rowIDOrder(r *Rows) (satisfied, reverse bool) {
	if len(r.order) != 1 || r.vectorRank != nil {
		return false, false
	}
	o := r.order[0]
	e := o.expr
	if o.column >= 0 {
		e = r.exprs[o.column]
	}
	c, ok := e.(*sqlast.Column)
	if !ok {
		return false, false
	}
	i, err := r.table.column(c.Table, c.Name)
	if err != nil || i != -1 && i != r.table.ipk {
		return false, false
	}
	for _, e := range r.exprs {
		if !ordinarySafeWhere(r, e) {
			return false, false
		}
	}
	return true, o.desc
}

// planRowIDRange changes only an untouched single-table scan. Equality, ANN
// and grams access retain their existing precedence, and JOIN/LEFT JOIN never
// enter this planner. Every candidate still runs the original WHERE program.
func (d *DB) planRowIDRange(ctx context.Context, reader btree.PageReader, r *Rows) error {
	if r.table == nil || r.relational != nil || r.sel.NotIndexed || r.queryPlan.Access != "SCAN" || r.vectorRank != nil {
		return nil
	}
	var bounds rowIDInterval
	rowIDBounds(ctx, r, r.sel.Where, &bounds)
	if err := ctx.Err(); err != nil {
		return err
	}
	ordered, reverse := rowIDOrder(r)
	if !bounds.seen && !ordered {
		return nil
	}
	if r.sel.Where != nil && !ordinarySafeWhere(r, r.sel.Where) || !ordinarySafeDefaults(ctx, r.table) {
		// A rejected order-only optimization leaves the selected scan and its
		// earlier grams/index fallback explanation untouched. An actual range
		// rejection adds context without erasing that preceding explanation.
		if bounds.seen {
			r.queryPlan.Reason += "; rowid range requires an error-safe residual WHERE and defaults"
		}
		return ctx.Err()
	}
	if bounds.lower != nil && bounds.lower.Value == math.MaxInt64 && !bounds.lower.Inclusive || bounds.upper != nil && bounds.upper.Value == math.MinInt64 && !bounds.upper.Inclusive {
		bounds.empty = true
	}
	if bounds.empty {
		bounds.lower = &btree.RowIDBound{Value: math.MaxInt64}
		bounds.upper = &btree.RowIDBound{Value: math.MaxInt64, Inclusive: true}
	}
	if bounds.lower != nil && bounds.upper != nil && (bounds.lower.Value > bounds.upper.Value || bounds.lower.Value == bounds.upper.Value && (!bounds.lower.Inclusive || !bounds.upper.Inclusive)) {
		bounds.empty = true
	}
	p := r.queryPlan
	p.Method, p.MethodVersion, p.Fields = "rowid_range", 1, []string{"rowid"}
	p.Indexes, p.Recheck, p.EstimatedCandidates = nil, true, -1
	bounded := bounds.lower != nil || bounds.upper != nil || bounds.empty
	if bounded {
		p.Access = "SEARCH"
	}
	if bounds.empty {
		p.EstimatedCandidates = 0
	}
	direction := "forward"
	if reverse {
		direction = "reverse"
	}
	p.Reason = fmt.Sprintf("native rowid interval; direction=%s; bounded=%t; order_satisfied=%t; sort=%t; covering=false", direction, bounded, ordered, len(r.order) > 0 && !ordered)
	if !r.sel.Explain {
		if err := r.source.Close(); err != nil {
			return err
		}
		r.source = &rowIDRangeSource{ctx: ctx, reader: reader, root: r.table.root, bounds: bounds, reverse: reverse, stats: &r.queryStats}
		if ordered {
			r.order = nil // The unique non-NULL key needs neither sort nor tie work.
		}
	}
	r.queryPlan = p
	return nil
}

// Open lazily: EXPLAIN and LIMIT 0 perform no range I/O. The snapshot belongs
// to Rows/Tx, as for the original source. Finite native cursor limits remain
// cumulative, are not reset between Next calls, and failures never mean EOF.
type rowIDRangeSource struct {
	ctx      context.Context
	reader   btree.PageReader
	root     uint32
	bounds   rowIDInterval
	reverse  bool
	stats    *QueryStats
	limits   btree.RangeLimits
	cursor   *btree.RowIDRangeCursor
	terminal error
}

func (s *rowIDRangeSource) Next() (int64, []any, error) {
	if s.terminal != nil {
		return 0, nil, s.terminal
	}
	if err := s.ctx.Err(); err != nil {
		s.terminal = err
		return 0, nil, err
	}
	if s.cursor == nil {
		if !s.bounds.empty {
			s.stats.RowIDSeeks++
		}
		var err error
		s.cursor, err = btree.NewRowIDRangeCursor(s.ctx, s.reader, s.root, s.bounds.lower, s.bounds.upper, s.reverse, s.limits)
		if err != nil {
			s.terminal = err
			return 0, nil, err
		}
	}
	id, values, err := s.cursor.Next()
	if err == nil {
		s.stats.Candidates++
	} else {
		s.terminal = err
	}
	return id, values, err
}

func (s *rowIDRangeSource) Close() error {
	s.terminal = io.EOF
	if s.cursor != nil {
		return s.cursor.Close()
	}
	return nil
}
