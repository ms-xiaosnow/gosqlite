package sqlite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// QueryPlan describes the actual source access path. SEARCH means covering
// posting B-tree ranges followed by individual source-table rowid seeks. Every
// candidate still evaluates the original WHERE. EXPLAIN does not execute it.
type QueryPlan struct {
	// Vector ranking is separate from source access: a heap is never an index.
	VectorMetric        vector.MetricKind
	VectorColumn        string
	VectorDimensions    int
	VectorTopK          int64
	VectorExact         bool
	VectorRanking       string
	Access              string
	Table               string
	Indexes             []string
	Method              string
	MethodVersion       int
	Fields              []string
	Namespace           string
	GramLengths         []int
	EstimatedCandidates int64
	Recheck             bool
	Reason              string
}

// QueryStats reports work performed so far, including eager ORDER BY work.
// PostingKeysRead counts matching covering posting records returned by ranges,
// not page I/O, seek comparisons or a posting-table scan.
// Candidates is the distinct rowid set before original-WHERE verification.
// BaseRowFetches counts actual source rows read; ReturnedRows excludes OFFSET.
// FrequencyLookups counts planning probes and also applies to EXPLAIN.
type QueryStats struct {
	// These count only the recognized exact vector ranking stage. NULL ranks
	// are separate; components charge its query once and scored candidates.
	VectorRankedRows     int64
	VectorFilteredRows   int64
	VectorNullRows       int64
	VectorRankComponents int64
	FrequencyLookups     int64
	PostingKeysRead      int64
	Candidates           int64
	BaseRowFetches       int64
	ReturnedRows         int64
}

func (r *Rows) Plan() QueryPlan {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.queryPlan
	p.Indexes = append([]string(nil), p.Indexes...)
	p.Fields = append([]string(nil), p.Fields...)
	p.GramLengths = append([]int(nil), p.GramLengths...)
	return p
}
func (r *Rows) Stats() QueryStats { r.mu.Lock(); defer r.mu.Unlock(); return r.queryStats }

func (p QueryPlan) Detail() string {
	var detail string
	if p.Access != "SEARCH" {
		detail = fmt.Sprintf("%s %s; reason=%s", p.Access, p.Table, p.Reason)
	} else {
		detail = fmt.Sprintf("SEARCH %s USING %s %s v%d; fields=%s; namespace=%s; grams=%v; estimated_candidates=%d; original_where_recheck=true; covering_posting_ranges=true; content_rowid_seek=true", p.Table, strings.Join(p.Indexes, ","), p.Method, p.MethodVersion, strings.Join(p.Fields, ","), p.Namespace, p.GramLengths, p.EstimatedCandidates)
	}
	if p.VectorRanking != "" {
		detail += fmt.Sprintf("; vector_ranking=%s; vector_metric=%s; vector_field=%s; dimensions=%d; retained_limit_plus_offset=%d; exact=%t; distance_order=ASC,rowid_ASC; nulls=FIRST", p.VectorRanking, p.VectorMetric, p.VectorColumn, p.VectorDimensions, p.VectorTopK, p.VectorExact)
	}
	return detail
}

type gramLookup struct {
	index     *extensionIndex
	field     extensionField
	token     grams13.Token
	frequency int64
	namespace grams13.Namespace
}
type gramCandidatePlan struct {
	kind     string
	lookups  []gramLookup
	children []*gramCandidatePlan
	estimate int64
}
type gramPlanner struct {
	ctx         context.Context
	reader      btree.PageReader
	rows        *Rows
	indexes     []*extensionIndex
	budget      int
	probes      int
	unavailable string
}

func (d *DB) planSource(ctx context.Context, reader btree.PageReader, r *Rows) error {
	r.queryPlan = QueryPlan{Access: "SCAN", Reason: "no safe indexed predicate"}
	if r.table == nil {
		r.queryPlan.Access, r.queryPlan.Reason = "CONSTANT", "no source table"
		if !r.sel.Explain {
			r.source = &constantSource{}
		}
		return nil
	}
	r.queryPlan.Table = r.table.name
	var cp *gramCandidatePlan
	if r.sel.NotIndexed {
		r.queryPlan.Reason = "NOT INDEXED requested"
	} else if r.sel.Where != nil {
		indexes, why, err := d.gramIndexes(ctx, reader, r.table)
		if err != nil {
			return err
		}
		budget := int(min(int64(100000), d.opts.MaxSortBytes/64))
		gp := &gramPlanner{ctx: ctx, reader: reader, rows: r, indexes: indexes, budget: budget, unavailable: why}
		if budget < 1 {
			r.queryPlan.Reason = "candidate memory budget"
		} else {
			cp, why, err = gp.predicate(r.sel.Where)
			if err != nil {
				return err
			}
			if cp == nil {
				r.queryPlan.Reason = why
			}
		}
		if cp != nil {
			r.queryPlan = QueryPlan{Access: "SEARCH", Table: r.table.name, Method: "grams13", MethodVersion: 1, EstimatedCandidates: cp.estimate, Recheck: true}
			describeGramPlan(cp, &r.queryPlan)
			if !r.sel.Explain {
				ids, err := gp.execute(cp)
				if errors.Is(err, ErrMemoryLimit) {
					cp = nil
					r.queryPlan = QueryPlan{Access: "SCAN", Table: r.table.name, Reason: "posting/candidate memory budget exceeded"}
				} else if err != nil {
					return err
				} else {
					r.queryStats.Candidates = int64(len(ids))
					r.source = &candidateSource{ctx: ctx, reader: reader, root: r.table.root, rowids: ids}
				}
			}
		}
	}
	if cp == nil && !r.sel.Explain {
		var err error
		r.source, err = btree.NewCursor(ctx, reader, r.table.root)
		return err
	}
	return nil
}

func describeGramPlan(p *gramCandidatePlan, out *QueryPlan) {
	for _, l := range p.lookups {
		if !containsString(strings.Split(out.Namespace, ","), string(l.namespace)) {
			if out.Namespace != "" {
				out.Namespace += ","
			}
			out.Namespace += string(l.namespace)
		}
		if !containsString(out.Indexes, l.index.name) {
			out.Indexes = append(out.Indexes, l.index.name)
		}
		if !containsString(out.Fields, l.field.name) {
			out.Fields = append(out.Fields, l.field.name)
		}
		found := false
		for _, n := range out.GramLengths {
			if n == int(l.token.Length) {
				found = true
			}
		}
		if !found {
			out.GramLengths = append(out.GramLengths, int(l.token.Length))
		}
	}
	for _, c := range p.children {
		describeGramPlan(c, out)
	}
	sort.Ints(out.GramLengths)
}
func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func (p *gramPlanner) predicate(e sqlast.Expr) (*gramCandidatePlan, string, error) {
	if err := p.ctx.Err(); err != nil {
		return nil, "", err
	}
	switch x := e.(type) {
	case *sqlast.Binary:
		if x.Op != "AND" && x.Op != "OR" {
			break
		}
		a, wa, err := p.predicate(x.Left)
		if err != nil {
			return nil, "", err
		}
		b, wb, err := p.predicate(x.Right)
		if err != nil {
			return nil, "", err
		}
		if x.Op == "AND" {
			if a == nil {
				return b, wa, nil
			}
			if b == nil {
				return a, wb, nil
			}
			return &gramCandidatePlan{kind: "AND", children: []*gramCandidatePlan{a, b}, estimate: min(a.estimate, b.estimate)}, "", nil
		}
		if a == nil || b == nil {
			return nil, "OR requires safe candidates for every branch: " + wa + " " + wb, nil
		}
		if a.estimate > int64(p.budget)-b.estimate {
			return nil, "OR candidate memory budget", nil
		}
		return &gramCandidatePlan{kind: "OR", children: []*gramCandidatePlan{a, b}, estimate: a.estimate + b.estimate}, "", nil
	case *sqlast.Like:
		if x.Not {
			return nil, "NOT LIKE requires scan", nil
		}
		return p.like(x)
	case *sqlast.Call:
		if fold(x.Name) == "grams13_match" {
			return p.match(x)
		}
	case *sqlast.Unary:
		if x.Op == "NOT" {
			return nil, "NOT predicate requires scan", nil
		}
	}
	return nil, "predicate has no safe gram filter", nil
}

func (p *gramPlanner) like(x *sqlast.Like) (*gramCandidatePlan, string, error) {
	column, ok := x.Expr.(*sqlast.Column)
	if !ok {
		return nil, "LIKE operand is not an indexed column", nil
	}
	ordinal, err := p.rows.table.column(column.Table, column.Name)
	if err != nil && column.Table == "" {
		if i, found := p.rows.aliases[fold(column.Name)]; found {
			if alias, yes := p.rows.exprs[i].(*sqlast.Column); yes {
				ordinal, err = p.rows.table.column(alias.Table, alias.Name)
			}
		}
	}
	if err != nil {
		return nil, "LIKE operand is not a physical column", nil
	}
	if ordinal < 0 {
		ordinal = p.rows.table.ipk
	}
	pattern, why, ok := constantLikePatternContext(p.ctx, x, p.rows.params)
	if p.ctx.Err() != nil {
		return nil, "", p.ctx.Err()
	}
	if !ok {
		return nil, why, nil
	}
	tokens := literalTokens(pattern.runs)
	if len(tokens) == 0 {
		return nil, "LIKE pattern has no necessary literal grams", nil
	}
	var best *gramCandidatePlan
	reason := p.unavailable
	for _, idx := range p.indexes {
		if !idx.usable {
			if reason == "" {
				reason = idx.fallback
			}
			continue
		}
		for _, field := range idx.fields {
			if field.ordinal != ordinal {
				continue
			}
			candidate, why, err := p.selectTokens(idx, field, tokens, grams13.LikeRawV1)
			if err != nil {
				return nil, "", err
			}
			if candidate != nil && (best == nil || candidate.estimate < best.estimate) {
				best = candidate
			}
			if why != "" {
				reason = why
			}
		}
	}
	if best != nil {
		return best, "", nil
	}
	if reason == "" {
		reason = "no grams13 index for LIKE column"
	}
	return nil, reason, nil
}

func (p *gramPlanner) selectTokens(idx *extensionIndex, field extensionField, tokens []grams13.Token, namespace grams13.Namespace) (*gramCandidatePlan, string, error) {
	var lookups []gramLookup
	for _, token := range tokens {
		if p.probes == 128 {
			break
		}
		p.probes++
		p.rows.queryStats.FrequencyLookups++
		f, err := idx.frequency(p.ctx, p.reader, field.id, namespace, token)
		if err != nil {
			return nil, "", err
		}
		if f < 0 {
			return nil, "", fmt.Errorf("%w: negative gram frequency", ErrCorrupt)
		}
		lookups = append(lookups, gramLookup{index: idx, field: field, token: token, frequency: f, namespace: namespace})
		if f == 0 {
			break
		}
	}
	if len(lookups) == 0 {
		return nil, "frequency probe budget", nil
	}
	sort.SliceStable(lookups, func(i, j int) bool { return lookups[i].frequency < lookups[j].frequency })
	f := lookups[0].frequency
	if f > int64(p.budget) {
		return nil, "posting frequency exceeds memory budget", nil
	}
	if f > 0 && f >= idx.documentCount-idx.documentCount/4 {
		return nil, "high posting frequency favors scan", nil
	}
	chosen := lookups[:1]
	cost := f
	for i := 1; i < len(lookups) && len(chosen) < 4 && f != 0; i++ {
		next := lookups[i].frequency
		if cost > int64(p.budget)-next || next >= idx.documentCount-idx.documentCount/4 {
			break
		}
		chosen = append(chosen, lookups[i])
		cost += next
	}
	return &gramCandidatePlan{kind: "GRAMS", lookups: chosen, estimate: f}, "", nil
}

type likeLiterals struct{ runs []string }

// Only row-independent builtin expressions are evaluated during planning.
// Errors merely disable filtering, leaving original evaluator/error semantics.
func constantLikePattern(x *sqlast.Like, params []any) (likeLiterals, string, bool) {
	return constantLikePatternContext(nil, x, params)
}

// Context-free proof callers can fold ordinary builtins, but bounded extension
// work must never run during planning without the real operation context.
func constantLikePatternContext(ctx context.Context, x *sqlast.Like, params []any) (likeLiterals, string, bool) {
	constant := func(e sqlast.Expr) bool {
		if e == nil {
			return true
		}
		return sqlast.Walk(e, func(v sqlast.Expr) error {
			switch call := v.(type) {
			case *sqlast.Call:
				if ctx == nil {
					switch fold(call.Name) {
					case "ngram_similarity", "bigram_similarity", "levenshtein":
						return fmt.Errorf("similarity requires planning context")
					}
				}
			case *sqlast.Column, *sqlast.Star:
				return fmt.Errorf("dynamic")
			}
			return nil
		}) == nil
	}
	if !constant(x.Pattern) || !constant(x.Escape) {
		return likeLiterals{}, "dynamic LIKE pattern/ESCAPE", false
	}
	ec := &sqlast.EvalContext{Context: ctx, Params: params}
	v, err := sqlast.Eval(x.Pattern, ec)
	if err != nil || v == nil {
		return likeLiterals{}, "NULL or unevaluable LIKE pattern", false
	}
	pattern, err := sqlast.LikeText(v)
	if err != nil || len(pattern) > sqlast.MaxLikePatternBytes {
		return likeLiterals{}, "unsupported LIKE pattern encoding/budget", false
	}
	var escape *rune
	if x.Escape != nil {
		v, err = sqlast.Eval(x.Escape, ec)
		if err != nil || v == nil {
			return likeLiterals{}, "NULL or unevaluable LIKE ESCAPE", false
		}
		text, err := sqlast.LikeText(v)
		if err != nil {
			return likeLiterals{}, "unsupported LIKE ESCAPE encoding", false
		}
		if n := strings.IndexByte(text, 0); n >= 0 {
			text = text[:n]
		}
		if utf8.RuneCountInString(text) != 1 {
			return likeLiterals{}, "invalid LIKE ESCAPE", false
		}
		e, _ := utf8.DecodeRuneInString(text)
		escape = &e
	}
	if n := strings.IndexByte(pattern, 0); n >= 0 {
		pattern = pattern[:n]
	}
	var runs []string
	var run strings.Builder
	flush := func() {
		if run.Len() > 0 {
			runs = append(runs, run.String())
			run.Reset()
		}
	}
	rr := []rune(pattern)
	for i := 0; i < len(rr); i++ {
		c := rr[i]
		if escape != nil && c == *escape {
			i++
			if i >= len(rr) {
				return likeLiterals{}, "trailing LIKE ESCAPE", false
			}
			c = rr[i]
		} else if c == '%' || c == '_' {
			flush()
			continue
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		run.WriteRune(c)
	}
	flush()
	return likeLiterals{runs}, "", true
}

func literalTokens(runs []string) []grams13.Token {
	var result []grams13.Token
	seen := map[grams13.Token]bool{}
	for _, run := range runs {
		rr := []rune(run)
		n := min(3, len(rr))
		for i := 0; n > 0 && i+n <= len(rr); i++ {
			token := grams13.Token{Length: uint8(n), Text: string(rr[i : i+n])}
			if !seen[token] {
				seen[token] = true
				result = append(result, token)
				if len(result) == 64 {
					return result
				}
			}
		}
	}
	return result
}

func (p *gramPlanner) execute(plan *gramCandidatePlan) ([]int64, error) {
	if err := p.ctx.Err(); err != nil {
		return nil, err
	}
	var result []int64
	if len(plan.lookups) != 0 {
		for i, l := range plan.lookups {
			remaining := p.budget - int(p.rows.queryStats.PostingKeysRead)
			if remaining < 0 {
				return nil, ErrMemoryLimit
			}
			ids, reads, err := l.index.postings(p.ctx, p.reader, l.field.id, l.namespace, l.token, remaining)
			p.rows.queryStats.PostingKeysRead += reads
			if err != nil {
				return nil, err
			}
			if i == 0 {
				result = ids
			} else {
				result = intersectRowids(result, ids)
			}
			if len(result) == 0 {
				break
			}
		}
		return result, nil
	}
	for i, c := range plan.children {
		ids, err := p.execute(c)
		if err != nil {
			return nil, err
		}
		if i == 0 {
			result = ids
		} else if plan.kind == "AND" {
			result = intersectRowids(result, ids)
		} else {
			result, err = unionRowids(result, ids, p.budget)
			if err != nil {
				return nil, err
			}
		}
		if plan.kind == "AND" && len(result) == 0 {
			break
		}
	}
	return result, nil
}
func intersectRowids(a, b []int64) []int64 {
	out := a[:0]
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] < b[j] {
			i++
		} else if a[i] > b[j] {
			j++
		} else {
			out = append(out, a[i])
			i++
			j++
		}
	}
	return out
}
func unionRowids(a, b []int64, limit int) ([]int64, error) {
	out := make([]int64, 0, min(limit, len(a)+len(b)))
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		var v int64
		if j == len(b) || (i < len(a) && a[i] < b[j]) {
			v = a[i]
			i++
		} else if i == len(a) || b[j] < a[i] {
			v = b[j]
			j++
		} else {
			v = a[i]
			i++
			j++
		}
		if len(out) == limit {
			return nil, ErrMemoryLimit
		}
		out = append(out, v)
	}
	return out, nil
}

type candidateSource struct {
	ctx    context.Context
	reader btree.PageReader
	root   uint32
	rowids []int64
	pos    int
}

func (s *candidateSource) Next() (int64, []any, error) {
	if s.pos == len(s.rowids) {
		return 0, nil, io.EOF
	}
	rowid := s.rowids[s.pos]
	s.pos++
	values, found, err := btree.SeekTable(s.ctx, s.reader, s.root, rowid)
	if err != nil {
		return 0, nil, err
	}
	if !found {
		return 0, nil, fmt.Errorf("%w: gram posting refers to missing content row", ErrCorrupt)
	}
	return rowid, values, nil
}
func (s *candidateSource) Close() error { s.rowids = nil; s.pos = 0; return nil }
