package sqlite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// secondaryInterval contains whole-field cuts on the first physical index key.
// SQL NULL comparisons are empty, not physical NULL endpoints. Non-NULL upper
// bounds also need a strict NULL lower cut: NULL is never true for < or <=.
// Constants have no expression affinity; the column's comparison affinity has
// already been applied. No integer-to-REAL conversion is used to form a cut.
type secondaryInterval struct {
	lower, upper *btree.IndexBound
	empty        bool
}

func (b *secondaryInterval) add(op string, value any) error {
	if value == nil {
		b.empty = true
		return nil
	}
	cut := &btree.IndexBound{Key: []any{value}, Inclusive: op == ">=" || op == "<="}
	if op == ">" || op == ">=" {
		cmp := 1
		var err error
		if b.lower != nil {
			cmp, err = sqlast.Compare(value, b.lower.Key[0])
		}
		if err != nil {
			return err
		}
		if cmp > 0 || cmp == 0 && !cut.Inclusive {
			b.lower = cut
		}
	} else {
		cmp := -1
		var err error
		if b.upper != nil {
			cmp, err = sqlast.Compare(value, b.upper.Key[0])
		}
		if err != nil {
			return err
		}
		if cmp < 0 || cmp == 0 && !cut.Inclusive {
			b.upper = cut
		}
	}
	if b.lower == nil {
		b.lower = &btree.IndexBound{Key: []any{nil}}
	}
	if b.upper != nil {
		cmp, err := sqlast.Compare(b.lower.Key[0], b.upper.Key[0])
		if err != nil {
			return err
		}
		b.empty = b.empty || cmp > 0 || cmp == 0 && (!b.lower.Inclusive || !b.upper.Inclusive)
	}
	return nil
}

func secondaryBounds(ctx context.Context, r *Rows, e sqlast.Expr, out map[int]*secondaryInterval) {
	x, ok := e.(*sqlast.Binary)
	if !ok {
		return
	}
	if x.Op == "AND" {
		secondaryBounds(ctx, r, x.Left, out)
		secondaryBounds(ctx, r, x.Right, out)
		return
	}
	op := x.Op
	if op != "<" && op != "<=" && op != ">" && op != ">=" {
		return
	}
	col, ok := x.Left.(*sqlast.Column)
	constant := x.Right
	if !ok {
		col, ok = x.Right.(*sqlast.Column)
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
	ordinal, ok := ordinaryColumn(r, col)
	if !ok || ordinal < 0 || sqlast.IsVectorDeclaration(r.table.columns[ordinal].Type) {
		return
	}
	v, err := sqlast.Eval(constant, &sqlast.EvalContext{Context: ctx, Params: r.params})
	if err != nil {
		return // Binding and the existing evaluator retain error precedence.
	}
	aff := affinity(r.table.columns[ordinal].Type)
	if aff == "INTEGER" || aff == "REAL" {
		aff = "NUMERIC"
	}
	v, err = sqlast.ApplyAffinity(v, aff)
	if err != nil {
		return
	}
	b := out[ordinal]
	if b == nil {
		b = &secondaryInterval{}
	}
	if b.add(op, v) == nil {
		out[ordinal] = b
	}
}

// Equality, rowid (including proven rowid ordering), grams and ANN planners
// have already run. Do not replace any of those selected sources. This narrow
// increment only admits a leading inequality, never an equality-prefix suffix,
// expression key, DESC index, vector rank, JOIN probe or ordering claim.
func (d *DB) planSecondaryRange(ctx context.Context, reader btree.PageReader, r *Rows) error {
	if r.table == nil || r.relational != nil || r.sel.NotIndexed || r.sel.Where == nil || r.queryPlan.Access != "SCAN" || r.queryPlan.Method == "rowid_range" || r.vectorRank != nil {
		return nil
	}
	bounds := make(map[int]*secondaryInterval)
	secondaryBounds(ctx, r, r.sel.Where, bounds)
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(bounds) == 0 {
		return nil
	}
	if !ordinarySafeWhere(r, r.sel.Where) || !ordinarySafeDefaults(ctx, r.table) {
		// No candidate was attempted. Preserve the earlier planner's specific
		// fallback reason (for example a grams high-frequency rejection).
		return ctx.Err()
	}
	idx, bound, err := secondaryReadIndex(ctx, reader, r.table, bounds)
	if err != nil || idx == nil {
		return err
	}
	// Admit endpoint copies and candidate capacity before opening a cursor.
	// Keep half the configured workspace for cursor records/parser admissions;
	// this is conservative workspace accounting, not a connection-wide heap cap.
	boundBytes := int64(256)
	for _, edge := range []*btree.IndexBound{bound.lower, bound.upper} {
		if edge != nil {
			boundBytes += 2 * valuesSize(edge.Key)
		}
	}
	memory := d.opts.MaxSortBytes / 2
	budget := int(min(int64(maxOrdinaryIndexCandidates), (memory-boundBytes)/64))
	if budget < 1 {
		secondaryRangeRejection(r, "secondary range endpoint/candidate memory budget; complete scan retained")
		return nil
	}
	p := r.queryPlan
	p.Access, p.Method, p.MethodVersion = "SEARCH", "btree_range", 1
	p.Indexes, p.Fields = []string{idx.name}, []string{r.table.columns[idx.columns[0]].Name}
	p.EstimatedCandidates, p.Recheck = -1, true
	if bound.empty {
		p.EstimatedCandidates = 0
	}
	p.Reason = fmt.Sprintf("native BINARY ASC leading-key interval; direction=forward; rowid-ordered candidates; order_satisfied=false; sort=%t; covering=false", len(r.order) > 0)
	if r.sel.Explain || r.limit == 0 {
		r.queryPlan = p // Prebinding is complete; these paths never open the range.
		return nil
	}
	ids, err := secondaryIndexRowIDs(ctx, reader, idx, bound, budget, memory, &r.queryStats)
	if errors.Is(err, errOrdinaryIndexBudget) || errors.Is(err, record.ErrLimit) || errors.Is(err, record.ErrUnsupported) {
		secondaryRangeRejection(r, "secondary range candidate/work/storage budget exceeded; complete scan retained")
		return nil
	}
	if err != nil {
		return err
	}
	if err = r.source.Close(); err != nil {
		return err
	}
	r.source = &ordinaryRowSource{ctx: ctx, reader: reader, root: r.table.root, rowids: ids, stats: &r.queryStats}
	r.queryStats.Candidates = int64(len(ids))
	r.queryPlan = p
	return nil
}

func secondaryRangeRejection(r *Rows, why string) {
	if r.queryPlan.Reason != "" {
		r.queryPlan.Reason += "; "
	}
	r.queryPlan.Reason += why
}

// Keep the reviewed equality catalog admission untouched. The same limited
// physical descriptor grammar proves every indexed field BINARY ASC, including
// suffix fields even though only the first field supplies the interval.
func secondaryReadIndex(ctx context.Context, reader btree.PageReader, t *table, bounds map[int]*secondaryInterval) (*ordinaryIndex, *secondaryInterval, error) {
	entries, err := btree.ReadSchema(ctx, reader)
	if err != nil {
		return nil, nil, err
	}
	var best *ordinaryIndex
	var bound *secondaryInterval
	examined := 0
	for _, entry := range entries {
		if entry.Type != "index" || fold(entry.TableName) != fold(t.name) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if examined == maxOrdinaryIndexDefinitions {
			break
		}
		examined++
		stmt, err := sqlast.Parse(entry.SQL)
		if err != nil {
			continue
		}
		def, ok := stmt.(*sqlast.CreateIndex)
		if !ok || def.Method != "" || len(def.Columns) == 0 {
			continue
		}
		if fold(def.Name) != fold(entry.Name) || fold(def.Table) != fold(t.name) || entry.RootPage < 2 || entry.RootPage == t.root {
			return nil, nil, fmt.Errorf("%w: secondary range index schema mismatch", ErrCorrupt)
		}
		idx := &ordinaryIndex{name: entry.Name, root: entry.RootPage}
		for i, name := range def.Columns {
			ordinal, found := t.index[fold(name)]
			if !found || i < len(def.Desc) && def.Desc[i] || sqlast.IsVectorDeclaration(t.columns[ordinal].Type) {
				break
			}
			idx.columns = append(idx.columns, ordinal)
		}
		if len(idx.columns) != len(def.Columns) || bounds[idx.columns[0]] == nil {
			continue
		}
		if best == nil || fold(idx.name) < fold(best.name) {
			best, bound = idx, bounds[idx.columns[0]]
		}
	}
	return best, bound, ctx.Err()
}

// These admission hooks run before payload copies and page-parser allocations,
// and preserve any tighter upstream policy. The bounded cursor separately caps
// cumulative page/work/record/payload admissions, including seek probes.
type secondaryRangeReader struct {
	btree.PageReader
	stats  *QueryStats
	ctx    context.Context
	memory int64
}

func (r *secondaryRangeReader) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.stats.IndexPageReads++
	return r.PageReader.ReadPage(ctx, n)
}
func (r *secondaryRangeReader) BeforeRecord(n int) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if n < 0 || r.memory < 512 || int64(n) > (r.memory-512)/128 {
		return errOrdinaryIndexBudget
	}
	if prior, ok := r.PageReader.(interface{ BeforeRecord(int) error }); ok {
		if err := prior.BeforeRecord(n); err != nil {
			return err
		}
	}
	return r.ctx.Err()
}
func (r *secondaryRangeReader) BeforePage(n int64) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if n < 0 || n > r.memory {
		return errOrdinaryIndexBudget
	}
	if prior, ok := r.PageReader.(interface{ BeforePage(int64) error }); ok {
		if err := prior.BeforePage(n); err != nil {
			return err
		}
	}
	return r.ctx.Err()
}

func secondaryIndexRowIDs(ctx context.Context, reader btree.PageReader, idx *ordinaryIndex, bounds *secondaryInterval, budget int, memory int64, stats *QueryStats) ([]int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if bounds.empty {
		return nil, nil
	}
	pages := min(int64(maxOrdinaryIndexPageReads), memory/(int64(reader.Header().PageSize)*4+512))
	if budget < 1 || budget > maxOrdinaryIndexCandidates || int64(budget) > memory/64 || pages < 1 {
		return nil, errOrdinaryIndexBudget
	}
	bounded := &secondaryRangeReader{PageReader: reader, stats: stats, ctx: ctx, memory: memory}
	cursor, err := btree.NewBoundedIndexCursor(ctx, bounded, idx.root, btree.KeyInfo{Columns: len(idx.columns)}, bounds.lower, bounds.upper, false, btree.RangeLimits{PageReads: pages})
	if err != nil {
		return nil, err
	}
	defer cursor.Close()
	// Capacity was admitted before range I/O, and never grows beyond budget.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, budget)
	for {
		key, err := cursor.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		stats.IndexKeysRead++
		if len(ids) == budget {
			return nil, errOrdinaryIndexBudget
		}
		ids = append(ids, key[len(key)-1].(int64))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i := 1; i < len(ids); i++ {
		if ids[i] == ids[i-1] {
			return nil, fmt.Errorf("%w: secondary range repeats a source rowid", ErrCorrupt)
		}
	}
	return ids, ctx.Err()
}
