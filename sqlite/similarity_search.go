package sqlite

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/extension/similarity"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// SimilarityOptions selects exact threshold/Top-K search. MinGram=MaxGram=0
// means unpadded bigrams; otherwise supply an inclusive range within 1..3.
// An empty Metric means Dice. Threshold is inclusive in [0,1]. Limit is required:
// zero returns no hits, and negative or over-budget limits are errors.
//
// Where is one supported SQL expression evaluated against the source table,
// before ranking. Args follow Query's positional/named binding rules. It cannot
// contain ORDER BY, LIMIT, a second statement, or unsupported SQL syntax.
// ForceScan provides an exact reference path with the same normalization.
// A zero Limits uses similarity.DefaultLimits; other limits must be complete.
// MaxCandidates bounds all fetched source rows, including rows rejected by Where.
// MaxTotalInputBytes also charges text from rows rejected by Where.
//
// The MATCH tokenizer configuration comes exclusively from the named index in
// this read snapshot. Changing normalization requires Reindex, never an option
// that silently interprets existing postings using a different configuration.
// SQL scalar ngram_similarity uses its documented default normalization; only
// indexes with that default configuration are directly comparable to it.
type SimilarityOptions struct {
	MinGram, MaxGram int
	Metric           similarity.Metric
	Threshold        float64
	Limit            int
	Where            string
	Args             []any
	ForceScan        bool
	Limits           similarity.Limits
}

// SimilarityPlan describes the actual path of a completed exact search.
// SEARCH reads the complete union of every selected query-gram posting, then
// seeks source rowids and recomputes exact scores. SCAN reads the whole source.
// EMPTY executes no source path (NULL query or zero limit). ConfigHash identifies
// the entire math configuration; TokenizerHash identifies the persisted MATCH
// configuration. Generation changes when the index is rebuilt.
type SimilarityPlan struct {
	QueryPlan
	CandidateMode    similarity.CandidateMode
	QueryTokens      int
	ConfigHash       string
	TokenizerHash    string
	Generation       int64
	MinGram, MaxGram int
	Metric           similarity.Metric
	Threshold        float64
	Limit            int
	Exact            bool
}

// Detail describes similarity access without suggesting the SQL expression
// optimizer performed a rewrite or used an approximate top-K candidate cap.
func (p SimilarityPlan) Detail() string {
	return fmt.Sprintf("%s %s; similarity=%s; grams=%d..%d; threshold=%g; limit=%d; exact=%t; candidates=%s; query_tokens=%d; score_order=DESC,rowid_ASC; reason=%s", p.Access, p.Table, p.Metric, p.MinGram, p.MaxGram, p.Threshold, p.Limit, p.Exact, p.CandidateMode, p.QueryTokens, p.Reason)
}

// SimilarityStats reports actual work, including abandoned posting reads before
// a full-scan fallback. PostingRangesRead counts opened query-gram ranges, not
// B-tree pages. Candidates is nonzero only after a complete posting union.
// FilteredRows counts source rows rejected by the business Where expression;
// ScoredRows excludes those and NULL fields. MatchedRows is before Top-K.
type SimilarityStats struct {
	QueryStats
	PostingRangesRead int64
	FilteredRows      int64
	ScoredRows        int64
	MatchedRows       int64
}

// SimilarityResult is returned only after complete success. Hits are ordered by
// exact float64 score DESC, signed rowid ASC; their integer score cardinalities
// are retained. No partial result is returned on cancellation or a quota error.
type SimilarityResult struct {
	Hits  []similarity.Hit
	Plan  SimilarityPlan
	Stats SimilarityStats
}

// SimilaritySearch searches one field covered by a named persistent grams13
// index. Query uses the same SQL text coercion as indexed values; nil is NULL
// and yields no hits. The operation owns a single snapshot until it completes.
// Native methods use an existing transaction when one is active on this DB.
func (d *DB) SimilaritySearch(ctx context.Context, index, field string, query any, options SimilarityOptions) (SimilarityResult, error) {
	if err := checkContext(ctx); err != nil {
		return SimilarityResult{}, wrap("similarity search", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return SimilarityResult{}, wrap("similarity search", ErrClosed)
	}
	if d.tx != nil {
		if err := d.tx.check(); err != nil {
			return SimilarityResult{}, wrap("similarity search", err)
		}
		if err := d.tx.ensureSnapshotLocked(ctx); err != nil {
			return SimilarityResult{}, wrap("similarity search", err)
		}
		result, err := d.similaritySearch(ctx, d.tx.reader, index, field, query, options)
		return result, wrap("similarity search", err)
	}
	reader, err := d.openReader(ctx)
	if err != nil {
		return SimilarityResult{}, wrap("similarity search", err)
	}
	d.info = fileInfo(reader.Header())
	result, err := d.similaritySearch(ctx, reader, index, field, query, options)
	err = errors.Join(err, reader.Close())
	if err != nil {
		return SimilarityResult{}, wrap("similarity search", err)
	}
	return result, nil
}

func (t *Tx) SimilaritySearch(ctx context.Context, index, field string, query any, options SimilarityOptions) (SimilarityResult, error) {
	if err := checkContext(ctx); err != nil {
		return SimilarityResult{}, wrap("similarity search", err)
	}
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	if err := t.check(); err != nil {
		return SimilarityResult{}, wrap("similarity search", err)
	}
	if err := t.ensureSnapshotLocked(ctx); err != nil {
		return SimilarityResult{}, wrap("similarity search", err)
	}
	result, err := t.db.similaritySearch(ctx, t.reader, index, field, query, options)
	return result, wrap("similarity search", err)
}

func similarityConfig(tokenizer grams13.Config, o SimilarityOptions) (similarity.Config, similarity.Metric, error) {
	c := similarity.DefaultConfig()
	c.Tokenizer = tokenizer
	if o.MinGram != 0 || o.MaxGram != 0 {
		c.MinGram, c.MaxGram = o.MinGram, o.MaxGram
	}
	if o.Limits != (similarity.Limits{}) {
		c.Limits = o.Limits
	}
	if err := c.Validate(); err != nil {
		return c, "", err
	}
	metric := o.Metric
	if metric == "" {
		metric = similarity.Dice
	}
	if _, _, err := similarity.MinimumIntersection(0, 0, o.Threshold, metric); err != nil {
		return c, metric, err
	}
	if o.Limit < 0 {
		return c, metric, fmt.Errorf("%w: negative result limit", similarity.ErrValue)
	}
	if o.Limit > c.Limits.MaxResults {
		return c, metric, fmt.Errorf("%w: results", similarity.ErrLimit)
	}
	return c, metric, nil
}

func (d *DB) similaritySearch(ctx context.Context, reader btree.PageReader, name, fieldName string, query any, options SimilarityOptions) (SimilarityResult, error) {
	c, err := d.loadExtensions(ctx, reader)
	if err != nil {
		return SimilarityResult{}, err
	}
	var idx *extensionIndex
	for _, candidate := range c.indexes {
		if fold(candidate.name) == fold(name) {
			idx = candidate
			break
		}
	}
	if idx == nil {
		return SimilarityResult{}, fmt.Errorf("no such extension index: %s", name)
	}
	if idx.method != grams13.MethodName || idx.methodVersion != grams13.MethodVersion {
		return SimilarityResult{}, fmt.Errorf("%w: similarity requires a grams13 index", ErrUnsupported)
	}
	var field extensionField
	for _, f := range idx.fields {
		if fold(f.name) == fold(fieldName) {
			field = f
			break
		}
	}
	if field.id == 0 {
		return SimilarityResult{}, fmt.Errorf("similarity field %q is not covered by index %q", fieldName, name)
	}
	config, metric, err := similarityConfig(idx.matchConfig, options)
	if err != nil {
		return SimilarityResult{}, err
	}
	if int64(options.Limit) > d.opts.MaxSortBytes/128 {
		return SimilarityResult{}, fmt.Errorf("%w: similarity result memory", ErrMemoryLimit)
	}
	engine, err := similarity.New(config)
	if err != nil {
		return SimilarityResult{}, err
	}
	configHash, _ := config.Hash() // New validated this exact value.
	tokenizerHash, _ := config.Tokenizer.Hash()
	result := SimilarityResult{Plan: SimilarityPlan{
		QueryPlan: QueryPlan{Access: "SCAN", Table: idx.sourceTable, Indexes: []string{idx.name}, Fields: []string{field.name},
			Method: idx.method, MethodVersion: idx.methodVersion, Namespace: string(grams13.MatchConfigV1), EstimatedCandidates: -1, Recheck: true},
		ConfigHash: fmt.Sprintf("%x", configHash), TokenizerHash: fmt.Sprintf("%x", tokenizerHash), Generation: idx.generation,
		MinGram: config.MinGram, MaxGram: config.MaxGram, Metric: metric, Threshold: options.Threshold, Limit: options.Limit, Exact: true,
	}}
	for n := config.MinGram; n <= config.MaxGram; n++ {
		result.Plan.GramLengths = append(result.Plan.GramLengths, n)
	}
	tab, err := d.table(ctx, reader, &sqlast.Select{From: idx.sourceTable})
	if err != nil {
		return SimilarityResult{}, err
	}
	ordinal, err := tab.column("", field.name)
	if err != nil || ordinal < 0 {
		return SimilarityResult{}, fmt.Errorf("%w: indexed source field changed; REINDEX required", ErrReindexRequired)
	}
	filter, err := d.similarityFilter(ctx, reader, tab, options)
	if err != nil {
		return SimilarityResult{}, err
	}
	// Convert application types first, then reject oversized borrowed strings or
	// blobs before parameterValue clones them or LikeText copies blob bytes.
	borrowed, err := driver.DefaultParameterConverter.ConvertValue(query)
	if err != nil {
		return SimilarityResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return SimilarityResult{}, err
	}
	if err := similarityInputSize(borrowed, config.Tokenizer.Limits.MaxInputBytes, config.Limits.MaxTotalInputBytes); err != nil {
		return SimilarityResult{}, err
	}
	value, err := parameterValue(borrowed)
	if err != nil {
		return SimilarityResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return SimilarityResult{}, err
	}
	if value == nil {
		result.Plan.Access, result.Plan.Reason = "EMPTY", "NULL query produces no similarity matches"
		return result, nil
	}
	text, err := sqlast.LikeText(value)
	if err != nil {
		return SimilarityResult{}, err
	}
	if int64(len(text)) > config.Limits.MaxTotalInputBytes {
		return SimilarityResult{}, fmt.Errorf("%w: total similarity input bytes", similarity.ErrLimit)
	}
	q, err := engine.Prepare(ctx, text)
	if err != nil {
		return SimilarityResult{}, err
	}
	result.Plan.QueryTokens = q.Len()
	candidatePlan, err := similarity.PlanCandidates(q.Len(), options.Threshold)
	if err != nil {
		return SimilarityResult{}, err
	}
	result.Plan.CandidateMode, result.Plan.Reason = candidatePlan.Mode, candidatePlan.Reason
	if options.ForceScan {
		result.Plan.CandidateMode, result.Plan.Reason = similarity.FullScan, "ForceScan requested"
	} else if !idx.usable {
		result.Plan.CandidateMode, result.Plan.Reason = similarity.FullScan, idx.fallback
	}
	if options.Limit == 0 {
		result.Plan.Access, result.Plan.CandidateMode = "EMPTY", ""
		result.Plan.Reason = "zero limit; query validated without source reads"
		return result, nil
	}
	var src source
	if result.Plan.CandidateMode == similarity.UnionPostings {
		// A bound limits work, never recall: failure to finish the ENTIRE union
		// discards it and opens the exact full-scan source instead.
		budget := int(min(int64(config.Limits.MaxCandidates), d.opts.MaxSortBytes/128))
		ids, err := similarityUnion(ctx, reader, idx, field, q.Tokens(), budget, &result.Stats)
		if errors.Is(err, ErrMemoryLimit) {
			result.Plan.CandidateMode = similarity.FullScan
			result.Plan.Reason = "posting/candidate budget prevents complete union; exact full scan required"
		} else if err != nil {
			return SimilarityResult{}, err
		} else {
			result.Plan.Access = "SEARCH"
			result.Plan.EstimatedCandidates = int64(len(ids))
			result.Stats.Candidates = int64(len(ids))
			src = &candidateSource{ctx: ctx, reader: reader, root: tab.root, rowids: ids}
		}
	}
	if src == nil {
		src, err = btree.NewCursor(ctx, reader, tab.root)
		if err != nil {
			return SimilarityResult{}, err
		}
	}
	defer src.Close()
	valuesHelper := &writeTable{table: tab}
	usedBytes := int64(len(text))
	scanned, err := engine.Scan(ctx, text, func(ctx context.Context) (similarity.Document, bool, error) {
		if err := ctx.Err(); err != nil {
			return similarity.Document{}, false, err
		}
		id, raw, err := src.Next()
		if err == io.EOF {
			return similarity.Document{}, false, nil
		}
		if err != nil {
			return similarity.Document{}, false, err
		}
		result.Stats.BaseRowFetches++
		if result.Stats.BaseRowFetches > int64(config.Limits.MaxCandidates) {
			return similarity.Document{}, false, fmt.Errorf("%w: source candidates", similarity.ErrLimit)
		}
		values, err := valuesHelper.values(id, raw)
		if err != nil {
			return similarity.Document{}, false, err
		}
		doc := similarity.Document{RowID: id, Null: values[ordinal] == nil}
		if !doc.Null {
			if err := similarityInputSize(values[ordinal], config.Tokenizer.Limits.MaxInputBytes, config.Limits.MaxTotalInputBytes-usedBytes); err != nil {
				return similarity.Document{}, false, err
			}
			doc.Text, err = sqlast.LikeText(values[ordinal])
			if err != nil {
				return similarity.Document{}, false, err
			}
			if int64(len(doc.Text)) > config.Limits.MaxTotalInputBytes-usedBytes {
				return similarity.Document{}, false, fmt.Errorf("%w: total similarity input bytes", similarity.ErrLimit)
			}
			usedBytes += int64(len(doc.Text))
		}
		if filter != nil {
			keep, err := filter(id, values)
			if err != nil {
				return similarity.Document{}, false, err
			}
			if !keep {
				result.Stats.FilteredRows++
				// Still yield the identity to Scan so its cumulative candidate
				// budget and duplicate-row protection include filtered rows.
				doc.Null, doc.Text = true, ""
			}
		}
		return doc, true, nil
	}, similarity.SearchOptions{Metric: metric, Threshold: options.Threshold, Limit: options.Limit})
	if err != nil {
		return SimilarityResult{}, err
	}
	if err := src.Close(); err != nil {
		return SimilarityResult{}, err
	}
	result.Hits = scanned.Hits
	result.Stats.ScoredRows, result.Stats.MatchedRows = int64(scanned.Stats.Scored), int64(scanned.Stats.Matched)
	result.Stats.ReturnedRows = int64(len(result.Hits))
	return result, nil
}

// similarityUnion has no selectivity/probe shortcut. All selected query tokens
// must be visited, even if early postings are empty or the first hit is perfect.
func similarityUnion(ctx context.Context, reader btree.PageReader, idx *extensionIndex, field extensionField, tokens []grams13.Token, budget int, stats *SimilarityStats) ([]int64, error) {
	if budget < 1 || len(tokens) > budget {
		return nil, ErrMemoryLimit
	}
	seen := make(map[int64]struct{})
	var reads int64
	for _, token := range tokens {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		stats.PostingRangesRead++
		rows, visited, err := idx.postings(ctx, reader, field.id, grams13.MatchConfigV1, token, budget-int(reads))
		reads += visited
		stats.PostingKeysRead += visited
		if err != nil {
			return nil, err
		}
		for n, id := range rows {
			if n&1023 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			if _, exists := seen[id]; !exists {
				if len(seen) == budget {
					return nil, ErrMemoryLimit
				}
				seen[id] = struct{}{}
			}
		}
	}
	ids := make([]int64, 0, len(seen))
	for id := range seen {
		if len(ids)&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		ids = append(ids, id)
	}
	var canceled error
	comparisons := 0
	sort.Slice(ids, func(i, j int) bool {
		comparisons++
		if comparisons&1023 == 0 && canceled == nil {
			canceled = ctx.Err()
		}
		return ids[i] < ids[j]
	})
	if canceled != nil {
		return nil, canceled
	}
	return ids, ctx.Err()
}

// Wrapping the expression in parentheses prevents it from adding SELECT
// clauses. Parse+bind preserves exact numbered/named parameter semantics.
func (d *DB) similarityFilter(ctx context.Context, reader btree.PageReader, tab *table, options SimilarityOptions) (func(int64, []any) (bool, error), error) {
	query := "SELECT 1"
	if strings.TrimSpace(options.Where) != "" {
		if _, err := sqlast.ParseExpr(options.Where); err != nil {
			return nil, err
		}
		query += " WHERE (" + options.Where + "\n)"
	}
	stmt, err := sqlast.Parse(query)
	if err != nil {
		return nil, err
	}
	sel := stmt.(*sqlast.Select)
	params, err := bind(sel, options.Args)
	if err != nil {
		return nil, err
	}
	sel.Where = bindBooleans(sel.Where, tab)
	if err := validate(sel.Where, tab, nil); err != nil {
		return nil, err
	}
	if sel.Where == nil {
		return nil, nil
	}
	r := &Rows{ctx: ctx, table: tab, sel: sel, params: params}
	if err := d.prepareGramFunctions(ctx, reader, r); err != nil {
		return nil, err
	}
	return func(id int64, values []any) (bool, error) {
		ec := &sqlast.EvalContext{Context: ctx, Params: params, Functions: r.functions}
		ec.Resolver = func(q, name string) (any, error) {
			i, err := tab.column(q, name)
			if err != nil {
				return nil, err
			}
			if i < 0 {
				return id, nil
			}
			return columnValue(values[i], tab.columns[i].Type)
		}
		ec.Affinity = func(q, name string) string {
			i, err := tab.column(q, name)
			if err != nil {
				return ""
			}
			if i < 0 {
				return "INTEGER"
			}
			return affinity(tab.columns[i].Type)
		}
		value, err := sqlast.Eval(sel.Where, ec)
		if err != nil {
			return false, err
		}
		truth, null := sqlast.Truth(value)
		return truth && !null, nil
	}, nil
}

// Check borrowed variable-length storage before any coercion-owned copy. Numeric
// conversion is bounded by the SQL value representation and checked afterward.
func similarityInputSize(value any, tokenizerBytes int, remainingBytes int64) error {
	var n int
	switch v := value.(type) {
	case string:
		n = len(v)
	case []byte:
		n = len(v)
	}
	if n > tokenizerBytes {
		return fmt.Errorf("%w: similarity input bytes", grams13.ErrLimit)
	}
	if int64(n) > remainingBytes {
		return fmt.Errorf("%w: total similarity input bytes", similarity.ErrLimit)
	}
	return nil
}
