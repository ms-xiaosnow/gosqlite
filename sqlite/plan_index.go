package sqlite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// ordinaryIndex is read-only metadata, deliberately independent of the write
// catalog's whole-database validation. Unsupported indexes do not make a table
// unreadable. This descriptor proves only nonunique BINARY ASC physical keys.
type ordinaryIndex struct {
	name    string
	root    uint32
	columns []int
	prefix  []any
}

const maxOrdinaryIndexCandidates = 100000
const maxOrdinaryIndexDefinitions = 256
const maxOrdinaryIndexPageReads = 8192

var errOrdinaryIndexBudget = errors.New("ordinary index candidate/work budget")

// planOrdinarySource runs after ANN eligibility, retaining ANN then grams
// precedence. It replaces only an untouched scan. Candidate rowids are sorted
// before source fetches: index suffix order must not change unordered LIMIT or
// the stable tie order of the existing sorter. No covering/order-elision claim
// is made, and the original WHERE is always evaluated again.
func (d *DB) planOrdinarySource(ctx context.Context, reader btree.PageReader, r *Rows) error {
	if r.table == nil || r.sel.NotIndexed || r.sel.Where == nil || r.queryPlan.Access != "SCAN" || (r.vectorRank != nil && r.vectorRank.ann != nil) {
		return nil
	}
	constraints := make(map[int]any)
	ordinaryEqualities(ctx, r, r.sel.Where, constraints)
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(constraints) == 0 {
		return nil
	}
	// Unrelated grams/MATCH rejection reasons remain intact when there is no
	// ordinary equality to consider.
	// The scalar evaluator eagerly evaluates AND. Filtering on one conjunct
	// would hide errors in another on excluded rows unless the entire WHERE is
	// total over ordinary storage classes. Keep error-capable functions, LIKE,
	// typed-vector interpretation and unknown expression shapes on their old
	// path. Projection errors retain the same matching rows and rowid order.
	if !ordinarySafeWhere(r, r.sel.Where) || !ordinarySafeDefaults(ctx, r.table) {
		r.queryPlan.Reason = "ordinary index requires an error-safe residual WHERE and defaults"
		return nil
	}
	if value, ok := constraints[-1]; ok {
		id, possible := ordinaryRowID(value)
		p := r.queryPlan
		p.Access, p.Method, p.MethodVersion = "SEARCH", "rowid", 1
		p.Indexes, p.Fields = nil, []string{"rowid"}
		p.EstimatedCandidates, p.Recheck = 1, true
		p.Reason = "rowid equality seek; ANN and grams retain precedence"
		var ids []int64
		if possible {
			ids = []int64{id}
		} else {
			p.EstimatedCandidates = 0
			p.Reason = "rowid equality is impossible after comparison affinity"
		}
		if !r.sel.Explain {
			if err := r.source.Close(); err != nil {
				return err
			}
			r.source = &ordinaryRowSource{ctx: ctx, reader: reader, root: r.table.root, rowids: ids, stats: &r.queryStats, missingOK: true}
			r.queryStats.Candidates = int64(len(ids))
		}
		r.queryPlan = p
		return nil
	}
	idx, why, err := ordinaryReadIndex(ctx, reader, r.table, constraints)
	if err != nil {
		return err
	}
	if idx == nil {
		if why != "" {
			r.queryPlan.Reason = why
		}
		return nil
	}
	// Prefix copies, retained rowids and cursor work each have conservative
	// bounds. The B-tree additionally enforces payload/depth/visited limits.
	prefixBytes := 2 * valuesSize(idx.prefix)
	budget := int(min(int64(maxOrdinaryIndexCandidates), (d.opts.MaxSortBytes-prefixBytes)/64))
	if prefixBytes > d.opts.MaxSortBytes/2 || budget < 1 {
		r.queryPlan.Reason = "ordinary index prefix/candidate memory budget"
		return nil
	}
	p := r.queryPlan
	p.Access, p.Method, p.MethodVersion = "SEARCH", "btree", 1
	p.Indexes, p.Fields = []string{idx.name}, nil
	for _, ordinal := range idx.columns[:len(idx.prefix)] {
		p.Fields = append(p.Fields, r.table.columns[ordinal].Name)
	}
	p.EstimatedCandidates, p.Recheck = -1, true // No cardinality statistics yet.
	p.Reason = "BINARY ASC leading equality prefix; rowid-ordered candidates; ANN and grams retain precedence"
	if r.sel.Explain {
		r.queryPlan = p
		return nil
	}
	ids, err := ordinaryIndexRowIDs(ctx, reader, idx, budget, d.opts.MaxSortBytes-prefixBytes, &r.queryStats)
	if errors.Is(err, errOrdinaryIndexBudget) || errors.Is(err, record.ErrLimit) || errors.Is(err, record.ErrUnsupported) {
		r.queryPlan.Reason = "ordinary index candidate/work/storage budget exceeded; complete scan retained"
		return nil
	}
	if err != nil {
		return err
	}
	if err := r.source.Close(); err != nil {
		return err
	}
	r.source = &ordinaryRowSource{ctx: ctx, reader: reader, root: r.table.root, rowids: ids, stats: &r.queryStats}
	r.queryStats.Candidates = int64(len(ids))
	r.queryPlan = p
	return nil
}

// Resolve physical slots first, matching WHERE's alias and rowid-shadow rules.
// Only a direct physical-column result alias is admitted in this first slice.
func ordinaryColumn(r *Rows, c *sqlast.Column) (int, bool) {
	i, err := r.table.column(c.Table, c.Name)
	if err != nil && c.Table == "" {
		if alias, ok := r.aliases[fold(c.Name)]; ok {
			if physical, ok := r.exprs[alias].(*sqlast.Column); ok {
				i, err = r.table.column(physical.Table, physical.Name)
			}
		}
	}
	if err != nil {
		return 0, false
	}
	if i == r.table.ipk {
		i = -1
	}
	return i, true
}

func ordinarySafeWhere(r *Rows, e sqlast.Expr) bool {
	switch x := e.(type) {
	case *sqlast.Literal, *sqlast.Parameter:
		return true
	case *sqlast.Column:
		i, ok := ordinaryColumn(r, x)
		if !ok || i < 0 {
			return ok
		}
		_, vector := sqlast.VectorDimensions(r.table.columns[i].Type)
		return !vector
	case *sqlast.Unary:
		switch x.Op {
		case "+", "-", "~", "NOT":
			return ordinarySafeWhere(r, x.Expr)
		}
	case *sqlast.Binary:
		switch x.Op {
		case "AND", "OR", "=", "==", "!=", "<>", "<", "<=", ">", ">=", "IS", "IS NOT", "+", "-", "*", "/", "%", "&", "|", "<<", ">>", "||":
			return ordinarySafeWhere(r, x.Left) && ordinarySafeWhere(r, x.Right)
		}
	}
	return false
}

func ordinarySafeDefaults(ctx context.Context, t *table) bool {
	for _, col := range t.columns {
		if col.Default == nil {
			continue
		}
		if !isConstantDefault(col.Default) {
			return false
		}
		v, err := sqlast.Eval(col.Default, &sqlast.EvalContext{Context: ctx})
		if err != nil {
			return false
		}
		if _, err = storedColumnValue(v, col.Type); err != nil {
			return false
		}
	}
	return true
}

func ordinaryConstant(e sqlast.Expr) bool {
	switch x := e.(type) {
	case *sqlast.Literal, *sqlast.Parameter:
		return true
	case *sqlast.Unary:
		return (x.Op == "+" || x.Op == "-") && ordinaryConstant(x.Expr)
	}
	return false
}

func ordinaryEqualities(ctx context.Context, r *Rows, e sqlast.Expr, out map[int]any) {
	x, ok := e.(*sqlast.Binary)
	if !ok {
		return
	}
	if x.Op == "AND" {
		ordinaryEqualities(ctx, r, x.Left, out)
		ordinaryEqualities(ctx, r, x.Right, out)
		return
	}
	if x.Op != "=" && x.Op != "==" && x.Op != "IS" {
		return
	}
	if lit, ok := x.Right.(*sqlast.Literal); x.Op == "IS" && ok && lit.Boolean {
		return // IS TRUE/FALSE tests truth, not equality to 1/0.
	}
	col, ok := x.Left.(*sqlast.Column)
	constant := x.Right
	if !ok {
		col, ok = x.Right.(*sqlast.Column)
		constant = x.Left
	}
	if !ok || !ordinaryConstant(constant) {
		return
	}
	ordinal, ok := ordinaryColumn(r, col)
	if !ok {
		return
	}
	if _, exists := out[ordinal]; exists {
		return // The first equality is sufficient; the others stay residuals.
	}
	value, err := sqlast.Eval(constant, &sqlast.EvalContext{Context: ctx, Params: r.params})
	if err != nil {
		return
	}
	aff := "INTEGER"
	if ordinal >= 0 {
		aff = affinity(r.table.columns[ordinal].Type)
	}
	// Comparison affinity is NUMERIC for all three numeric column affinities,
	// not storage REAL coercion. This preserves exact int64/REAL comparisons.
	if aff == "INTEGER" || aff == "REAL" {
		aff = "NUMERIC"
	}
	value, err = sqlast.ApplyAffinity(value, aff)
	if err == nil {
		out[ordinal] = value
	}
}

func ordinaryRowID(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case float64:
		if x >= -9223372036854775808.0 && x < 9223372036854775808.0 && math.Trunc(x) == x {
			return int64(x), true
		}
	}
	return 0, false
}

func ordinaryReadIndex(ctx context.Context, reader btree.PageReader, t *table, constraints map[int]any) (*ordinaryIndex, string, error) {
	entries, err := btree.ReadSchema(ctx, reader)
	if err != nil {
		return nil, "", err
	}
	var best *ordinaryIndex
	why := "no supported BINARY ASC leading equality-prefix index"
	examined := 0
	for _, entry := range entries {
		if entry.Type != "index" || fold(entry.TableName) != fold(t.name) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		if examined == maxOrdinaryIndexDefinitions {
			why = "ordinary index definition planning budget"
			break
		}
		examined++
		stmt, err := sqlast.Parse(entry.SQL)
		if err != nil {
			continue // Includes autoindexes, unique, expression and collation forms.
		}
		def, ok := stmt.(*sqlast.CreateIndex)
		if !ok || def.Method != "" || len(def.Columns) == 0 {
			continue
		}
		if fold(def.Name) != fold(entry.Name) || fold(def.Table) != fold(t.name) || entry.RootPage < 2 || entry.RootPage == t.root {
			return nil, "", fmt.Errorf("%w: ordinary index schema mismatch", ErrCorrupt)
		}
		idx := &ordinaryIndex{name: entry.Name, root: entry.RootPage}
		usable := true
		for i, name := range def.Columns {
			ordinal, found := t.index[fold(name)]
			if !found || (i < len(def.Desc) && def.Desc[i]) {
				usable = false
				break
			}
			if _, typed := sqlast.VectorDimensions(t.columns[ordinal].Type); typed {
				usable = false
				break
			}
			idx.columns = append(idx.columns, ordinal)
		}
		if !usable {
			continue
		}
		for _, ordinal := range idx.columns {
			if ordinal == t.ipk {
				ordinal = -1
			}
			value, found := constraints[ordinal]
			if !found {
				break
			}
			idx.prefix = append(idx.prefix, value)
		}
		if len(idx.prefix) > 0 && (best == nil || len(idx.prefix) > len(best.prefix) || (len(idx.prefix) == len(best.prefix) && fold(idx.name) < fold(best.name))) {
			best = idx
		}
	}
	return best, why, nil
}

// Page reads count actual range-cursor calls (including repeated probes), not
// schema reads or source-row seeks. Conservative work bounds force a complete
// fallback before a source row has been emitted; cancellation/errors propagate.
type ordinaryIndexReader struct {
	btree.PageReader
	stats *QueryStats
	left  int64
}

func (r *ordinaryIndexReader) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.left <= 0 {
		return nil, errOrdinaryIndexBudget
	}
	r.left--
	r.stats.IndexPageReads++
	return r.PageReader.ReadPage(ctx, n)
}

func ordinaryIndexRowIDs(ctx context.Context, reader btree.PageReader, idx *ordinaryIndex, budget int, memory int64, stats *QueryStats) ([]int64, error) {
	work := min(int64(maxOrdinaryIndexPageReads), memory/(int64(reader.Header().PageSize)*4+512))
	bounded := &ordinaryIndexReader{PageReader: reader, stats: stats, left: work}
	cursor, err := btree.NewIndexRangeCursor(ctx, bounded, idx.root, btree.KeyInfo{Columns: len(idx.columns)}, nil, idx.prefix)
	if err != nil {
		return nil, err
	}
	defer cursor.Close()
	var ids []int64
	for {
		key, err := cursor.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		stats.IndexKeysRead++
		if len(ids) == budget || 2*valuesSize(key) > memory {
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
			return nil, fmt.Errorf("%w: ordinary index repeats a source rowid", ErrCorrupt)
		}
	}
	return ids, ctx.Err()
}

type ordinaryRowSource struct {
	ctx       context.Context
	reader    btree.PageReader
	root      uint32
	rowids    []int64
	pos       int
	stats     *QueryStats
	missingOK bool
}

func (s *ordinaryRowSource) Next() (int64, []any, error) {
	if err := s.ctx.Err(); err != nil {
		return 0, nil, err
	}
	if s.pos == len(s.rowids) {
		return 0, nil, io.EOF
	}
	id := s.rowids[s.pos]
	s.pos++
	s.stats.RowIDSeeks++
	values, found, err := btree.SeekTable(s.ctx, s.reader, s.root, id)
	if err != nil {
		return 0, nil, err
	}
	if !found {
		if s.missingOK {
			return 0, nil, io.EOF
		}
		return 0, nil, fmt.Errorf("%w: ordinary index refers to missing source row", ErrCorrupt)
	}
	return id, values, nil
}
func (s *ordinaryRowSource) Close() error { s.rowids = nil; s.pos = 0; return nil }
