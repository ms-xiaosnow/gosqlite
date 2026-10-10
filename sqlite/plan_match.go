package sqlite

import (
	"context"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// grams13_match(index_name,value,needle) uses this query snapshot's persisted
// MATCH_CONFIG_V1 config. It is normalized substring containment, not FTS syntax.
// Dirty postings disable candidate filtering but do not change exact matching.
func (d *DB) prepareGramFunctions(ctx context.Context, reader btree.PageReader, r *Rows) error {
	needed := false
	inspect := func(e sqlast.Expr) {
		if e != nil {
			_ = sqlast.Walk(e, func(e sqlast.Expr) error {
				if c, ok := e.(*sqlast.Call); ok && fold(c.Name) == "grams13_match" {
					needed = true
				}
				return nil
			})
		}
	}
	for _, e := range r.exprs {
		inspect(e)
	}
	inspect(r.sel.Where)
	for _, o := range r.order {
		inspect(o.expr)
	}
	if !needed {
		return nil
	}
	catalog, err := d.loadExtensions(ctx, reader)
	if err != nil {
		return err
	}
	tokenizers := map[string]*grams13.Tokenizer{}
	for _, idx := range catalog.indexes {
		if idx.method != grams13.MethodName {
			continue
		}
		tokenizer, err := grams13.NewTokenizer(idx.matchConfig)
		if err != nil {
			continue
		} // A call naming an invalid config fails explicitly below.
		tokenizers[fold(idx.name)] = tokenizer
	}
	r.functions = map[string]sqlast.Function{"grams13_match": func(args []any) (any, error) {
		if len(args) != 3 {
			return nil, fmt.Errorf("wrong number of arguments to grams13_match()")
		}
		if args[0] == nil || args[1] == nil || args[2] == nil {
			return nil, nil
		}
		name, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("grams13_match index name must be text")
		}
		tokenizer := tokenizers[fold(name)]
		if tokenizer == nil {
			return nil, fmt.Errorf("%w: unknown or unusable grams13 matching configuration %q", ErrUnsupported, name)
		}
		value, err := sqlast.LikeText(args[1])
		if err != nil {
			return nil, err
		}
		needle, err := sqlast.LikeText(args[2])
		if err != nil {
			return nil, err
		}
		found, err := tokenizer.Contains(ctx, value, needle)
		if err != nil {
			return nil, err
		}
		if found {
			return int64(1), nil
		}
		return int64(0), nil
	}}
	return nil
}

func (p *gramPlanner) match(call *sqlast.Call) (*gramCandidatePlan, string, error) {
	if len(call.Args) != 3 {
		return nil, "invalid grams13_match arity", nil
	}
	// No column resolver is installed, so row-dependent arguments are rejected
	// without evaluating them against a fabricated row.
	nameValue, err := sqlast.Eval(call.Args[0], &sqlast.EvalContext{Params: p.rows.params})
	name, ok := nameValue.(string)
	if err != nil || !ok {
		return nil, "dynamic or NULL grams13_match index", nil
	}
	needleValue, err := sqlast.Eval(call.Args[2], &sqlast.EvalContext{Params: p.rows.params})
	if err != nil || needleValue == nil {
		return nil, "dynamic or NULL grams13_match needle", nil
	}
	needle, err := sqlast.LikeText(needleValue)
	if err != nil {
		return nil, "unsupported grams13_match needle encoding", nil
	}
	column, ok := call.Args[1].(*sqlast.Column)
	if !ok {
		return nil, "grams13_match value is not an indexed column", nil
	}
	ordinal, err := p.rows.table.column(column.Table, column.Name)
	if err != nil {
		return nil, "grams13_match value is not a physical column", nil
	}
	if ordinal < 0 {
		ordinal = p.rows.table.ipk
	}
	for _, idx := range p.indexes {
		if fold(idx.name) != fold(name) {
			continue
		}
		if !idx.usable {
			return nil, idx.fallback, nil
		}
		tokenizer, err := grams13.NewTokenizer(idx.matchConfig)
		if err != nil {
			return nil, "unsupported grams13 matching configuration", nil
		}
		normalized, err := tokenizer.Normalize(p.ctx, needle)
		if err != nil {
			if p.ctx.Err() != nil {
				return nil, "", p.ctx.Err()
			}
			return nil, "grams13_match normalization budget or policy", nil
		}
		if normalized == "" {
			return nil, "empty normalized grams13_match needle requires scan", nil
		}
		// A normalized containment match necessarily contains every contiguous gram
		// of the normalized needle, including decompositions/fold expansions.
		tokens := literalTokens([]string{normalized})
		for _, field := range idx.fields {
			if field.ordinal == ordinal {
				return p.selectTokens(idx, field, tokens, grams13.MatchConfigV1)
			}
		}
		return nil, "grams13_match column not covered by named index", nil
	}
	if p.unavailable != "" {
		return nil, p.unavailable, nil
	}
	return nil, "grams13_match index does not cover source table", nil
}
