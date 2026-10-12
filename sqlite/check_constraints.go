package sqlite

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// These are private CHECK quotas, independent of relational Query limits.
// One owner spans schema compilation and every selected row/declaration in a
// statement or full Check. Work never resets; expression scratch is released
// only after its Boolean result is discarded. Metadata stays charged.
const maxCheckWork int64 = 1 << 24
const maxCheckMetadata int64 = 16 << 20

type checkBudgetKey struct{}
type checkBudget struct {
	ctx      context.Context
	work     int64
	maxWork  int64
	metadata int64
	scratch  int64
	maxBytes int64
}

func withCheckBudget(ctx context.Context, maxBytes int64) (context.Context, *checkBudget) {
	if existing, ok := ctx.Value(checkBudgetKey{}).(*checkBudget); ok {
		return ctx, existing
	}
	if maxBytes <= 0 {
		maxBytes = 64 << 20
	}
	b := &checkBudget{ctx: ctx, maxWork: maxCheckWork, maxBytes: maxBytes}
	return context.WithValue(ctx, checkBudgetKey{}, b), b
}
func (b *checkBudget) Step(n int64) error {
	if err := b.ctx.Err(); err != nil {
		return err
	}
	if n < 0 || n > b.maxWork-b.work {
		return fmt.Errorf("%w: CHECK cumulative work budget", ErrMemoryLimit)
	}
	b.work += n
	return nil
}
func (b *checkBudget) Allocate(n int64) error {
	if err := b.ctx.Err(); err != nil {
		return err
	}
	if n < 0 || n > b.maxBytes-b.metadata-b.scratch {
		return fmt.Errorf("%w: CHECK scalar scratch budget", ErrMemoryLimit)
	}
	b.scratch += n
	return nil
}
func (b *checkBudget) retain(n int64) error {
	if err := b.Step(1); err != nil {
		return err
	}
	if n < 0 || n > maxCheckMetadata-b.metadata || n > b.maxBytes-b.metadata-b.scratch {
		return fmt.Errorf("%w: CHECK retained metadata budget", ErrMemoryLimit)
	}
	b.metadata += n
	return nil
}

// preflightCheckDescriptor runs before allocating the table name map, visiting
// DEFAULT through the general validator/evaluator, or deriving UNIQUE indexes.
// It is used only by CHECK-bearing tables; non-CHECK descriptor behavior stays
// unchanged. Binding and SQL constraint/evaluation order remain in their usual
// later phases. This pass only admits shape, bounded metadata and resources.
func preflightCheckDescriptor(ctx context.Context, def *sqlast.CreateTable) (context.Context, *checkBudget, error) {
	ctx, budget := withCheckBudget(ctx, 0)
	step := func(n int64) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return budget.Step(n)
	}
	fail := func(feature string) (context.Context, *checkBudget, error) {
		return ctx, budget, fmt.Errorf("%w: CHECK descriptor %s", ErrUnsupported, feature)
	}
	if err := step(1); err != nil {
		return ctx, budget, err
	}
	if len(def.Columns) == 0 || len(def.Columns) > sqlast.MaxColumns || len(def.CheckConstraints) > sqlast.MaxCheckConstraints || len(def.UniqueConstraints) > sqlast.MaxUniqueConstraints {
		return fail("declaration count budget")
	}
	if len(def.Name) > sqlast.MaxSQLBytes || len(def.SQL) > sqlast.MaxSQLBytes {
		return fail("name or SQL byte budget")
	}
	if err := step(8*int64(len(def.Name)) + int64(len(def.SQL))); err != nil {
		return ctx, budget, err
	}
	if err := budget.retain(int64(len(def.Name)) + int64(len(def.SQL)) + int64(len(def.Columns))*256); err != nil {
		return ctx, budget, err
	}
	// Complete all column string checks before any default is traversed.
	for _, column := range def.Columns {
		if err := step(1); err != nil {
			return ctx, budget, err
		}
		if len(column.Name) > sqlast.MaxSQLBytes {
			return fail("column name byte budget")
		}
		if !checkCanonicalColumnType(column.Type) {
			return fail("column type is not parser-canonical")
		}
		n := int64(len(column.Name)) + int64(len(column.Type))
		if err := step(16 * n); err != nil {
			return ctx, budget, err
		}
		if err := budget.retain(8 * n); err != nil {
			return ctx, budget, err
		}
	}
	// Bound shape walks before any rewrite/name lookup/evaluation. Later
	// source binding remains responsible for resolving every raw reference.
	nodeCharge := func(e sqlast.Expr) error {
		if err := step(1); err != nil {
			return err
		}
		n := int64(0)
		switch x := e.(type) {
		case *sqlast.Literal:
			switch value := x.Value.(type) {
			case string:
				n = int64(len(value))
			case []byte:
				n = int64(len(value))
			}
		case *sqlast.Column:
			if len(x.Table) > sqlast.MaxSQLBytes || len(x.Name) > sqlast.MaxSQLBytes {
				return fmt.Errorf("%w: CHECK descriptor reference byte budget", ErrUnsupported)
			}
			n = int64(len(x.Table)) + int64(len(x.Name))
		case *sqlast.Cast:
			n = int64(len(x.Type))
		}
		if n > sqlast.MaxSQLBytes {
			return fmt.Errorf("%w: CHECK descriptor scalar byte budget", ErrUnsupported)
		}
		if err := step(8 * n); err != nil {
			return err
		}
		return budget.retain(64 + 4*n)
	}
	shapeError := func(err error) error {
		if _, unsupported := err.(*sqlast.ErrUnsupported); unsupported {
			return fmt.Errorf("%w: CHECK descriptor shape: %w", ErrUnsupported, err)
		}
		return err
	}
	checkNodes := 0
	for _, check := range def.CheckConstraints {
		if err := step(1); err != nil {
			return ctx, budget, err
		}
		if len(check.Name) > sqlast.MaxSQLBytes || len(check.Text) > sqlast.MaxSQLBytes {
			return fail("CHECK name or source byte budget")
		}
		n := int64(len(check.Name)) + int64(len(check.Text))
		if err := step(n); err != nil {
			return ctx, budget, err
		}
		if err := budget.retain(128 + n); err != nil {
			return ctx, budget, err
		}
		if def.SQL != "" && (check.Start < 0 || check.End < check.Start || check.End > len(def.SQL) || def.SQL[check.Start:check.End] != check.Text) {
			return fail("CHECK source span mismatch")
		}
		nodes, err := sqlast.WalkCheckSource(check.Expr, nodeCharge)
		if err != nil {
			return ctx, budget, shapeError(err)
		}
		if nodes > sqlast.MaxCheckNodes-checkNodes {
			return fail("CHECK total source node budget")
		}
		checkNodes += nodes
	}
	defaultNodes := 0
	for _, column := range def.Columns {
		if column.Default == nil {
			continue
		}
		nodes, err := sqlast.WalkCheckDefaultSource(column.Default, nodeCharge)
		if err != nil {
			return ctx, budget, shapeError(err)
		}
		if nodes > sqlast.MaxTokens-defaultNodes {
			return fail("DEFAULT total source node budget")
		}
		defaultNodes += nodes
	}
	uniqueTerms := 0
	for _, unique := range def.UniqueConstraints {
		if err := step(1); err != nil {
			return ctx, budget, err
		}
		if len(unique.Name) > sqlast.MaxSQLBytes || len(unique.Columns) == 0 || len(unique.Columns) > sqlast.MaxColumns || len(unique.Columns) > sqlast.MaxUniqueTerms-uniqueTerms {
			return fail("UNIQUE declaration budget")
		}
		uniqueTerms += len(unique.Columns)
		if err := step(int64(len(unique.Name))); err != nil {
			return ctx, budget, err
		}
		if err := budget.retain(int64(len(unique.Name)) + 64); err != nil {
			return ctx, budget, err
		}
		for _, name := range unique.Columns {
			if len(name) > sqlast.MaxSQLBytes {
				return fail("UNIQUE column name byte budget")
			}
			if err := step(8 * int64(len(name))); err != nil {
				return ctx, budget, err
			}
			if err := budget.retain(32 + 4*int64(len(name))); err != nil {
				return ctx, budget, err
			}
		}
	}
	return ctx, budget, nil
}

// The CREATE parser emits these canonical spellings only. A length guard runs
// before prefix/scalar parsing, so no arbitrary manual type can make each
// IN-list comparison scan an unbounded declared-type string. This does not
// restrict the separate ordinary SQL CAST typename grammar.
func checkCanonicalColumnType(typ string) bool {
	if len(typ) > len("SPARSEVEC(1000000000)") {
		return false
	}
	switch typ {
	case "", "INTEGER", "INT", "TEXT", "REAL", "BLOB", "NUMERIC":
		return true
	}
	for _, rule := range []struct {
		prefix string
		max    int
	}{{"VECTOR(", 16000}, {"HALFVEC(", 16000}, {"BIT(", 64000}, {"SPARSEVEC(", 1000000000}} {
		if strings.HasPrefix(typ, rule.prefix) && strings.HasSuffix(typ, ")") {
			digits := typ[len(rule.prefix) : len(typ)-1]
			n, err := strconv.Atoi(digits)
			return err == nil && n > 0 && n <= rule.max && strconv.Itoa(n) == digits
		}
	}
	return false
}

type compiledCheck struct {
	label        string
	expr         sqlast.Expr
	dependencies []int // Sorted ordinals; -1 denotes hidden rowid/IPK.
}

func compileCheckConstraints(ctx context.Context, def *sqlast.CreateTable, tab *writeTable) ([]compiledCheck, error) {
	if len(def.CheckConstraints) == 0 {
		return nil, nil
	}
	if len(def.CheckConstraints) > sqlast.MaxCheckConstraints || len(def.Columns) > sqlast.MaxColumns || len(def.SQL) > sqlast.MaxSQLBytes || len(def.Name) > sqlast.MaxSQLBytes {
		return nil, fmt.Errorf("%w: CHECK declaration or source budget", ErrUnsupported)
	}
	if extensionReserved(def.Name) {
		return nil, fmt.Errorf("%w: CHECK on extension-owned tables", ErrUnsupported)
	}
	_, budget := withCheckBudget(ctx, 0)
	// Retain the original SQL only once, plus conservative immutable-copy,
	// dependency/map and slice-growth accounting for each source node below.
	if err := budget.Step(1 + int64(len(def.SQL)) + 8*int64(len(def.Name))); err != nil {
		return nil, err
	}
	if err := budget.retain(int64(len(def.SQL)) + int64(len(def.CheckConstraints))*256); err != nil {
		return nil, err
	}
	for _, column := range tab.columns {
		if len(column.Name) > sqlast.MaxSQLBytes || len(column.Type) > sqlast.MaxSQLBytes {
			return nil, fmt.Errorf("%w: CHECK column metadata budget", ErrUnsupported)
		}
		if err := budget.Step(1 + 8*int64(len(column.Type))); err != nil {
			return nil, err
		}
	}
	checks := make([]compiledCheck, 0, len(def.CheckConstraints))
	totalNodes := 0
	for _, declaration := range def.CheckConstraints {
		if err := budget.Step(1); err != nil {
			return nil, err
		}
		if len(declaration.Name) > sqlast.MaxSQLBytes || len(declaration.Text) > sqlast.MaxSQLBytes {
			return nil, fmt.Errorf("%w: CHECK label budget", ErrUnsupported)
		}
		if err := budget.Step(1 + int64(len(declaration.Name)) + int64(len(declaration.Text))); err != nil {
			return nil, err
		}
		if def.SQL != "" && (declaration.Start < 0 || declaration.End < declaration.Start || declaration.End > len(def.SQL) || def.SQL[declaration.Start:declaration.End] != declaration.Text) {
			return nil, fmt.Errorf("%w: CHECK source span mismatch", ErrUnsupported)
		}
		if err := budget.retain(int64(len(declaration.Name)) + int64(len(declaration.Text))); err != nil {
			return nil, err
		}
		nodes, err := sqlast.WalkCheckSource(declaration.Expr, func(e sqlast.Expr) error {
			bytes := int64(512) // Raw/copy/rewrite nodes, child storage and sparse dependencies.
			work := int64(32)   // Admission plus bounded immutable rewrite passes.
			switch x := e.(type) {
			case *sqlast.Literal:
				// TRUE/FALSE are source references when a real column shadows
				// them, even if a later empty-IN/false-AND fold erases the node.
				if x.Boolean {
					name := "false"
					if x.Value == int64(1) {
						name = "true"
					}
					if i, exists := tab.index[name]; exists {
						n := int64(len(tab.columns[i].Type))
						if err := budget.Step(1 + 16*n); err != nil {
							return err
						}
						if err := budget.retain(8 * n); err != nil {
							return err
						}
						if sqlast.IsVectorDeclaration(tab.columns[i].Type) {
							return fmt.Errorf("%w: CHECK source references a typed-vector Boolean column", ErrUnsupported)
						}
					}
				}
				switch value := x.Value.(type) {
				case string:
					if len(value) > sqlast.MaxSQLBytes {
						return fmt.Errorf("%w: CHECK literal budget", ErrUnsupported)
					}
					bytes += 4 * int64(len(value))
				case []byte:
					if len(value) > sqlast.MaxSQLBytes {
						return fmt.Errorf("%w: CHECK literal budget", ErrUnsupported)
					}
					bytes += 4 * int64(len(value))
				}
			case *sqlast.Column:
				if len(x.Table) > sqlast.MaxSQLBytes || len(x.Name) > sqlast.MaxSQLBytes {
					return fmt.Errorf("%w: CHECK reference budget", ErrUnsupported)
				}
				bytes += 16 * (int64(len(x.Table)) + int64(len(x.Name)) + int64(len(tab.name)))
				work += bytes
				// Charge before case-folding/hashing potentially large names.
				if err := budget.Step(work); err != nil {
					return err
				}
				work = 0
				if err := budget.retain(bytes); err != nil {
					return err
				}
				bytes = 0
				i, err := tab.column(x.Table, x.Name)
				if err != nil {
					return fmt.Errorf("%w: CHECK source: %v", ErrUnsupported, err)
				}
				if i >= 0 {
					n := int64(len(tab.columns[i].Type))
					if err := budget.Step(1 + 16*n); err != nil {
						return err
					}
					if err := budget.retain(8 * n); err != nil {
						return err
					}
					if sqlast.IsVectorDeclaration(tab.columns[i].Type) {
						return fmt.Errorf("%w: CHECK references a typed-vector column", ErrUnsupported)
					}
				}
			case *sqlast.Cast:
				bytes += 4 * int64(len(x.Type))
				work += 8 * int64(len(x.Type))
			}
			if err := budget.Step(work + bytes); err != nil {
				return err
			}
			return budget.retain(bytes)
		})
		if err != nil {
			if _, unsupported := err.(*sqlast.ErrUnsupported); unsupported {
				return nil, fmt.Errorf("%w: CHECK declaration: %w", ErrUnsupported, err)
			}
			return nil, err
		}
		if nodes > sqlast.MaxCheckNodes-totalNodes {
			return nil, fmt.Errorf("%w: total CHECK source node budget", ErrUnsupported)
		}
		totalNodes += nodes
		expr := bindBooleans(sqlast.NormalizeCheckSource(declaration.Expr), tab.table)
		dependencies := map[int]bool{}
		var bindErr error
		// Replace admitted column spellings with bounded private ordinal names.
		// This avoids repeatedly hashing long SQL identifiers in every row/IN
		// comparison. Source AST and schema text are never changed.
		expr = sqlast.Rewrite(expr, func(e sqlast.Expr) sqlast.Expr {
			if err := budget.Step(1); err != nil {
				bindErr = err
				return e
			}
			if column, ok := e.(*sqlast.Column); ok {
				i, err := tab.column(column.Table, column.Name)
				if err != nil {
					bindErr = err
					return e
				}
				if i >= 0 && sqlast.IsVectorDeclaration(tab.columns[i].Type) {
					bindErr = fmt.Errorf("%w: CHECK references a typed-vector Boolean column", ErrUnsupported)
					return e
				}
				if i == tab.ipk {
					i = -1
				}
				dependencies[i] = true
				return &sqlast.Column{Name: strconv.Itoa(i)}
			}
			return e
		})
		if bindErr != nil {
			return nil, bindErr
		}
		label := sqlast.CheckSourceLabel(declaration)
		if !declaration.Named && declaration.Name == "" {
			label = strings.TrimSpace(label)
		}
		check := compiledCheck{label: label, expr: expr}
		for column := range dependencies {
			check.dependencies = append(check.dependencies, column)
		}
		sort.Ints(check.dependencies)
		checks = append(checks, check)
	}
	return checks, nil
}

// selectChecks is once per UPDATE, not once per row. Assignment membership is
// what matters, including SET a=a and assigning an unchanged rowid value.
func (tab *writeTable) selectChecks(ctx context.Context, targets []int) ([]compiledCheck, error) {
	if len(tab.checks) == 0 {
		return nil, ctx.Err()
	}
	_, budget := withCheckBudget(ctx, 0)
	if err := budget.retain(int64(len(targets))*32 + int64(len(tab.checks))*64); err != nil {
		return nil, err
	}
	assigned := make(map[int]bool, len(targets))
	for _, target := range targets {
		if err := budget.Step(1); err != nil {
			return nil, err
		}
		assigned[target] = true
	}
	var selected []compiledCheck
	for _, check := range tab.checks {
		if err := budget.Step(1); err != nil {
			return nil, err
		}
		for _, dependency := range check.dependencies {
			if err := budget.Step(1); err != nil {
				return nil, err
			}
			if assigned[dependency] {
				selected = append(selected, check)
				break
			}
		}
	}
	return selected, nil
}

func (tab *writeTable) checkEvalContext(ctx context.Context, rowid int64, values []any) *sqlast.EvalContext {
	ordinal := func(q, name string) (int, error) {
		i, err := strconv.Atoi(name)
		if q != "" || err != nil || i < -1 || i >= len(tab.columns) || len(values) != len(tab.columns) {
			return 0, fmt.Errorf("%w: CHECK bound column", ErrCorrupt)
		}
		return i, nil
	}
	typ := func(q, name string) string {
		i, err := ordinal(q, name)
		if err != nil {
			return ""
		}
		if i < 0 {
			return "INTEGER"
		}
		return tab.columns[i].Type
	}
	// Same current-row values and affinity as evalContext. No parameter,
	// registry or connection-state function can enter this admitted tree.
	return &sqlast.EvalContext{Context: ctx, DeclaredType: typ, Resolver: func(q, name string) (any, error) {
		i, err := ordinal(q, name)
		if err != nil {
			return nil, err
		}
		if i < 0 {
			return rowid, nil
		}
		return values[i], nil
	}, Affinity: func(q, name string) string { return affinity(typ(q, name)) }}
}

func (tab *writeTable) evaluateChecks(ctx context.Context, checks []compiledCheck, rowid int64, values []any, violation error) error {
	if len(checks) == 0 {
		return ctx.Err()
	}
	ctx, budget := withCheckBudget(ctx, 0)
	ec := tab.checkEvalContext(sqlast.WithEvalBudget(ctx, budget), rowid, values)
	for _, check := range checks {
		if err := budget.Step(1); err != nil {
			return err
		}
		pass, err := sqlast.EvalCheck(check.expr, ec)
		budget.scratch = 0 // The scalar result is a bool; no operand escapes.
		if err != nil {
			return err
		}
		if !pass {
			return fmt.Errorf("%w: CHECK constraint failed: %s", violation, check.label)
		}
	}
	return nil
}
