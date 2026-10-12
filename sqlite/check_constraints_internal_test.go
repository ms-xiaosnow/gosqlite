package sqlite

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// All invalid rows/catalogs and boundary probes here are descriptor-free memory
// models. No fixture in this file is written through a native file or VFS.
func TestCheckBindingAndDependenciesMemory(t *testing.T) {
	def := automaticTableDefinition(t, `CREATE TABLE t(id INTEGER PRIMARY KEY,a,b,rowid TEXT,"true",CHECK(a>0),CHECK(id>0),CHECK(rowid LIKE 'r%'),CHECK(true>0),CHECK(0 AND a),CHECK(a IN ()),CHECK(1 OR b>0),CHECK(_rowid_>0))`)
	ctx, budget := withCheckBudget(context.Background(), 1<<20)
	tab, err := writableTableContext(ctx, def, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]int{{1}, {-1}, {3}, {4}, nil, nil, {2}, {-1}}
	if len(tab.checks) != len(want) || len(tab.indexes) != 0 {
		t.Fatal("CHECK changed schema roots", tab)
	}
	for i, check := range tab.checks {
		if !reflect.DeepEqual(check.dependencies, want[i]) {
			t.Fatal("dependency uses source or execution-time folding", i, check.dependencies, want[i])
		}
	}
	values := []any{int64(5), int64(-1), int64(-1), "row", int64(2)}
	selected, err := tab.selectChecks(ctx, []int{2})
	if err != nil || len(selected) != 1 {
		t.Fatal(selected, err)
	}
	if err = tab.evaluateChecks(ctx, selected, 5, values, ErrConstraint); err != nil {
		t.Fatal("unrelated UPDATE rechecked invalid a/constant checks", err)
	}
	selected, err = tab.selectChecks(ctx, []int{1})
	if err != nil || len(selected) != 1 {
		t.Fatal(selected, err)
	}
	if err = tab.evaluateChecks(ctx, selected, 5, values, ErrConstraint); !errors.Is(err, ErrConstraint) {
		t.Fatal("SET a=a did not check a", err)
	}
	selected, err = tab.selectChecks(ctx, []int{-1})
	if err != nil || len(selected) != 2 {
		t.Fatal("IPK and hidden rowid dependency", selected, err)
	}
	if err = tab.evaluateChecks(ctx, selected, -1, values, ErrConstraint); !errors.Is(err, ErrConstraint) {
		t.Fatal("rowid change bypassed check", err)
	}
	if err = tab.evaluateChecks(ctx, tab.checks, 5, values, ErrCorrupt); !errors.Is(err, ErrCorrupt) {
		t.Fatal("full primary-scan validation accepted an invalid logical row", err)
	}
	if budget.work == 0 || budget.metadata == 0 || budget.scratch != 0 {
		t.Fatal("quota owner accounting", budget)
	}
	// Empty physical names and numeric-looking spellings must not collide
	// with the compiler's private ordinal names or hidden rowid sentinel.
	quoted := automaticTableDefinition(t, `CREATE TABLE names("","0","-1",a,CHECK("">0),CHECK("0">1),CHECK("-1">2),CHECK(names.a>3),CHECK(rowid>0))`)
	bound, err := writableTable(quoted, 2)
	if err != nil {
		t.Fatal(err)
	}
	quotedValues := []any{int64(1), int64(2), int64(3), int64(4)}
	for i, check := range bound.checks {
		want := i
		if i == 4 {
			want = -1
		}
		if !reflect.DeepEqual(check.dependencies, []int{want}) {
			t.Fatal("physical spelling collided with ordinal binding", i, check.dependencies)
		}
	}
	if err := bound.evaluateChecks(context.Background(), bound.checks, 1, quotedValues, ErrConstraint); err != nil {
		t.Fatal(err)
	}
	for i := range quotedValues {
		badValues := append([]any(nil), quotedValues...)
		badValues[i] = int64(0)
		if err := bound.evaluateChecks(context.Background(), bound.checks[i:i+1], 1, badValues, ErrConstraint); !errors.Is(err, ErrConstraint) {
			t.Fatal("wrong physical column was enforced", i, err)
		}
	}
	// Source identifiers and leaf mutation cannot alter compiled authority.
	source := def.CheckConstraints[0].Expr.(*sqlast.Binary)
	source.Left.(*sqlast.Column).Name = "b"
	source.Right.(*sqlast.Literal).Value = int64(-100)
	values[1] = int64(-2)
	if err := tab.evaluateChecks(ctx, tab.checks[:1], 5, values, ErrConstraint); !errors.Is(err, ErrConstraint) {
		t.Fatal("compiled expression shared mutable source leaves", err)
	}
}

func TestCheckSemanticSourceAdmissionMemory(t *testing.T) {
	for _, expression := range []string{"missing", "other.a", "0 AND missing", "missing IN ()", "0 AND other.a", "CASE WHEN 1 THEN 1 ELSE missing END"} {
		def := automaticTableDefinition(t, "CREATE TABLE t(a CHECK("+expression+"))")
		if _, err := writableTable(def, 0); !errors.Is(err, ErrUnsupported) {
			t.Fatal("unresolved source was erased", expression, err)
		}
	}
	for _, typ := range []string{"VECTOR(2)", "HALFVEC(2)", "BIT(2)", "SPARSEVEC(2)"} {
		for _, expression := range []string{"v IS NULL", "0 AND v", "v IN ()", "CASE WHEN 1 THEN 1 ELSE v END"} {
			def := automaticTableDefinition(t, "CREATE TABLE t(v "+typ+",CHECK("+expression+"))")
			if _, err := writableTable(def, 0); !errors.Is(err, ErrUnsupported) {
				t.Fatal("typed-vector CHECK reference admitted", typ, expression, err)
			}
		}
		for _, expression := range []string{"0 AND true", "true IN ()", "0 AND false"} {
			def := automaticTableDefinition(t, "CREATE TABLE t(\"true\" "+typ+",\"false\" "+typ+",CHECK("+expression+"))")
			if _, err := writableTable(def, 0); !errors.Is(err, ErrUnsupported) {
				t.Fatal("folded Boolean source escaped vector exclusion", expression, err)
			}
		}
		if _, err := writableTable(automaticTableDefinition(t, "CREATE TABLE t(a CHECK(a>0),v "+typ+")"), 0); err != nil {
			t.Fatal("unrelated vector column blocked ordinary CHECK", err)
		}
	}
	for _, query := range []string{
		"CREATE TABLE t(a CHECK(b>a),b)", "CREATE TABLE t(a CHECK(t.a>0))",
		`CREATE TABLE t("" CHECK("">0))`, `CREATE TABLE t(a CONSTRAINT "" CHECK(a>0))`,
		"CREATE TABLE t(a CHECK(rowid>0))", "CREATE TABLE t(a CHECK(0))",
		"CREATE TABLE t(\"true\",\"false\",CHECK(true IS false))",
	} {
		if _, err := writableTable(automaticTableDefinition(t, query), 0); err != nil {
			t.Fatal("valid declaration evaluated or rejected at CREATE", query, err)
		}
	}
}

func TestCheckManualASTAdmissionMemory(t *testing.T) {
	cycle := &sqlast.Unary{Op: "NOT"}
	cycle.Expr = cycle
	for _, expr := range []sqlast.Expr{
		nil, (*sqlast.Literal)(nil), cycle, &sqlast.Parameter{Index: 1},
		&sqlast.Call{Name: "abs", Args: []sqlast.Expr{&sqlast.Literal{Value: int64(1)}}},
		&sqlast.Binary{Op: "<->", Left: &sqlast.Literal{}, Right: &sqlast.Literal{}},
		&sqlast.Binary{Op: "+", Left: &sqlast.Literal{}},
		&sqlast.InList{Expr: &sqlast.Literal{}, Values: []sqlast.Expr{nil}},
		&sqlast.Case{Arms: []sqlast.CaseArm{{When: &sqlast.Literal{Value: int64(1)}}}},
	} {
		def := &sqlast.CreateTable{Name: "t", Columns: []sqlast.ColumnDef{{Name: "a"}}, CheckConstraints: []sqlast.CheckConstraint{{Expr: expr}}}
		if _, err := writableTable(def, 0); !errors.Is(err, ErrUnsupported) {
			t.Fatal("manual AST bypassed admission", expr, err)
		}
	}
	def := automaticTableDefinition(t, "CREATE TABLE t(a CHECK(1))")
	def.CheckConstraints[0].Start = -1
	if _, err := writableTable(def, 0); !errors.Is(err, ErrUnsupported) {
		t.Fatal("bad source span", err)
	}
	def = &sqlast.CreateTable{Name: "t", Columns: []sqlast.ColumnDef{{Name: "a"}}, CheckConstraints: make([]sqlast.CheckConstraint, sqlast.MaxCheckConstraints+1)}
	if _, err := writableTable(def, 0); !errors.Is(err, ErrUnsupported) {
		t.Fatal("manual count bound", err)
	}
	def = automaticTableDefinition(t, "CREATE TABLE t(a CHECK(X'31'))")
	tab, err := writableTable(def, 2)
	if err != nil {
		t.Fatal(err)
	}
	def.CheckConstraints[0].Expr.(*sqlast.Literal).Value.([]byte)[0] = '0'
	if err := tab.evaluateChecks(context.Background(), tab.checks, 1, []any{nil}, ErrConstraint); err != nil {
		t.Fatal("compiled literal BLOB shared mutable source bytes", err)
	}
}

func TestCheckQuotaAndCancellationMemory(t *testing.T) {
	def := automaticTableDefinition(t, "CREATE TABLE t(a CHECK(a))")
	ctx, budget := withCheckBudget(context.Background(), 1<<20)
	tab, err := writableTableContext(ctx, def, 2)
	if err != nil {
		t.Fatal(err)
	}
	retained := budget.metadata
	tiny, _ := withCheckBudget(context.Background(), retained-1)
	if _, err := writableTableContext(tiny, def, 2); !errors.Is(err, ErrMemoryLimit) {
		t.Fatal("metadata one-under budget", err)
	}
	exact, exactBudget := withCheckBudget(context.Background(), retained)
	if _, err := writableTableContext(exact, def, 2); err != nil || exactBudget.metadata != retained {
		t.Fatal("exact metadata bound", exactBudget, err)
	}
	before := budget.work
	if err = tab.evaluateChecks(ctx, tab.checks, 1, []any{int64(1)}, ErrConstraint); err != nil {
		t.Fatal(err)
	}
	cost := budget.work - before
	budget.maxWork = budget.work + 2*cost
	for i := 0; i < 2; i++ {
		if err = tab.evaluateChecks(ctx, tab.checks, 1, []any{int64(1)}, ErrConstraint); err != nil {
			t.Fatal("exact cumulative work", i, err)
		}
	}
	if err = tab.evaluateChecks(ctx, tab.checks, 1, []any{int64(1)}, ErrConstraint); !errors.Is(err, ErrMemoryLimit) {
		t.Fatal("work reset between rows", err)
	}
	if budget.scratch != 0 || budget.metadata != retained {
		t.Fatal("expression scratch escaped or metadata reset", budget)
	}
	ctx, budget = withCheckBudget(context.Background(), retained+1024)
	if err = budget.retain(retained); err != nil {
		t.Fatal(err)
	}
	if err = tab.evaluateChecks(ctx, tab.checks, 1, []any{strings.Repeat("1", 1024)}, ErrConstraint); !errors.Is(err, ErrMemoryLimit) {
		t.Fatal("numeric text scratch preflight was bypassed", err)
	}
	if err = budget.Allocate(math.MaxInt64); !errors.Is(err, ErrMemoryLimit) {
		t.Fatal("scratch overflow", err)
	}
	if err = budget.Step(math.MaxInt64); !errors.Is(err, ErrMemoryLimit) {
		t.Fatal("work overflow", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = writableTableContext(canceled, def, 2); !errors.Is(err, context.Canceled) {
		t.Fatal("metadata ignored cancellation", err)
	}
	if err = tab.evaluateChecks(canceled, tab.checks, 1, []any{int64(1)}, ErrConstraint); !errors.Is(err, context.Canceled) {
		t.Fatal("row CHECK ignored cancellation", err)
	}
}

func scalarCheckMemoryFixture(t *testing.T, value int64, indexes int) (*DB, *checkMemory) {
	t.Helper()
	d, err := copyValidator()
	if err != nil {
		t.Fatal(err)
	}
	m := newCheckMemory()
	ctx := context.Background()
	root, err := btree.CreateTree(ctx, m, btree.TableTree)
	if err != nil {
		t.Fatal(err)
	}
	if err = btree.InsertTable(ctx, m, 1, 1, []any{"table", "t", "t", int64(root), "CREATE TABLE t(id INTEGER PRIMARY KEY,a CHECK(a>0))"}); err != nil {
		t.Fatal(err)
	}
	if err = btree.InsertTable(ctx, m, root, 1, []any{nil, value}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < indexes; i++ {
		name := "idx" + string(rune('a'+i))
		index, err := btree.CreateTree(ctx, m, btree.IndexTree)
		if err != nil {
			t.Fatal(err)
		}
		if err = btree.InsertTable(ctx, m, 1, int64(i+2), []any{"index", name, "t", int64(index), "CREATE INDEX " + name + " ON t(a)"}); err != nil {
			t.Fatal(err)
		}
		if err = btree.InsertIndex(ctx, m, index, btree.KeyInfo{Columns: 1}, []any{value, int64(1)}); err != nil {
			t.Fatal(err)
		}
	}
	return d, m
}

func TestCheckPrimaryScanOnceAndInvalidRowsMemory(t *testing.T) {
	var work int64
	for _, indexes := range []int{0, 2} {
		d, m := scalarCheckMemoryFixture(t, 1, indexes)
		before := uniqueMemoryPages(m)
		ctx, budget := withCheckBudget(context.Background(), 1<<20)
		opts, err := checkOptions(nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := d.checkSnapshot(ctx, m, opts)
		if err != nil || got.TableRows != 1 || got.IndexEntries != uint64(indexes) {
			t.Fatal("valid CHECK file model", got, err)
		}
		if indexes == 0 {
			work = budget.work
		} else if budget.work != work {
			t.Fatal("CHECK evaluation repeated per index", work, budget.work)
		}
		if !reflect.DeepEqual(before, m.pages) {
			t.Fatal("Check mutated its read-only source")
		}
	}
	d, m := scalarCheckMemoryFixture(t, -1, 2)
	before := uniqueMemoryPages(m)
	got, err := checkMemoryRun(d, m, CheckOptions{})
	if !errors.Is(err, ErrCorrupt) || got != (CheckResult{}) || !reflect.DeepEqual(before, m.pages) {
		t.Fatal("invalid logical row passed or was repaired", got, err)
	}
}

func TestCheckCatalogQuotaAndPreallocationMemory(t *testing.T) {
	ctx, budget := withCheckBudget(context.Background(), 1<<20)
	def := automaticTableDefinition(t, "CREATE TABLE t(a CHECK(a>0))")
	tab, err := writableTableContext(ctx, def, 2)
	if err != nil {
		t.Fatal(err)
	}
	perTable := budget.metadata
	entries := []btree.SchemaEntry{
		{Type: "table", Name: "t", TableName: "t", RootPage: 2, SQL: "CREATE TABLE t(a CHECK(a>0))"},
		{Type: "table", Name: "u", TableName: "u", RootPage: 3, SQL: "CREATE TABLE u(a CHECK(a>0))"},
	}
	for _, allowance := range []int64{2*perTable - 1, 2 * perTable} {
		limited, _ := withCheckBudget(context.Background(), allowance)
		_, err := buildWriteCatalog(limited, entries, &extensionCatalog{})
		if allowance < 2*perTable {
			if !errors.Is(err, ErrMemoryLimit) {
				t.Fatal("metadata owner reset between tables", err)
			}
		} else if err != nil {
			t.Fatal("exact aggregate metadata budget", err)
		}
	}
	// Reading row values alone remains legal, even for an invalid logical row.
	if values, err := tab.values(1, []any{int64(-1)}); err != nil || !reflect.DeepEqual(values, []any{int64(-1)}) {
		t.Fatal("ordinary read imposed CHECK", values, err)
	}
	// No storage/reader is supplied. Reaching root allocation or schema reads
	// before semantic admission would panic instead of returning Unsupported.
	catalog := &writeCatalog{tables: map[string]*writeTable{}, names: map[string]string{}}
	bad := automaticTableDefinition(t, "CREATE TABLE bad(a CHECK(0 AND unknown))")
	if err := (&Tx{}).createTable(context.Background(), catalog, bad); !errors.Is(err, ErrUnsupported) {
		t.Fatal("unsupported source reached allocation", err)
	}
	if len(catalog.tables) != 0 || len(catalog.names) != 0 {
		t.Fatal("rejected CHECK changed catalog")
	}
	// The selection cost is charged even when no assigned column intersects.
	budget.maxWork = budget.work
	if _, err := tab.selectChecks(ctx, nil); !errors.Is(err, ErrMemoryLimit) {
		t.Fatal("skipped checks bypassed cumulative work", err)
	}
}

func manualCheckDescriptor() *sqlast.CreateTable {
	return &sqlast.CreateTable{Name: "t", Columns: []sqlast.ColumnDef{{Name: "a", Type: "NUMERIC"}}, CheckConstraints: []sqlast.CheckConstraint{{Expr: &sqlast.Literal{Value: int64(1)}}}}
}

func TestCheckDescriptorPreflightOrderMemory(t *testing.T) {
	// A typed nil would panic in the former general default validator. These
	// inert inputs therefore distinguish early admission from late rejection
	// without running an expensive expression or touching any storage.
	poison := (*sqlast.Call)(nil)
	for _, tc := range []struct {
		name   string
		change func(*sqlast.CreateTable)
		label  string
	}{
		{"column count", func(d *sqlast.CreateTable) {
			d.Columns = make([]sqlast.ColumnDef, sqlast.MaxColumns+1)
			d.Columns[0].Default = poison
		}, "declaration count"},
		{"check count", func(d *sqlast.CreateTable) {
			d.CheckConstraints = make([]sqlast.CheckConstraint, sqlast.MaxCheckConstraints+1)
		}, "declaration count"},
		{"unique count", func(d *sqlast.CreateTable) {
			d.UniqueConstraints = make([]sqlast.UniqueConstraint, sqlast.MaxUniqueConstraints+1)
		}, "declaration count"},
		{"table name", func(d *sqlast.CreateTable) { d.Name = strings.Repeat("n", sqlast.MaxSQLBytes+1) }, "name or SQL"},
		{"SQL bytes", func(d *sqlast.CreateTable) { d.SQL = strings.Repeat("x", sqlast.MaxSQLBytes+1) }, "name or SQL"},
		{"column name", func(d *sqlast.CreateTable) { d.Columns[0].Name = strings.Repeat("n", sqlast.MaxSQLBytes+1) }, "column name"},
		{"column type", func(d *sqlast.CreateTable) { d.Columns[0].Type = strings.Repeat("q", 64) }, "parser-canonical"},
		{"check name", func(d *sqlast.CreateTable) { d.CheckConstraints[0].Name = strings.Repeat("n", sqlast.MaxSQLBytes+1) }, "CHECK name"},
		{"check span", func(d *sqlast.CreateTable) { d.SQL = "x"; d.CheckConstraints[0].Start = -1 }, "source span"},
		{"check shape", func(d *sqlast.CreateTable) { d.CheckConstraints[0].Expr = (*sqlast.Literal)(nil) }, "descriptor shape"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			def := manualCheckDescriptor()
			def.Columns[0].Default = poison
			tc.change(def)
			_, err := writableTableContext(context.Background(), def, 0)
			if !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), tc.label) {
				t.Fatal("descriptor work preceded its admission guard", err)
			}
		})
	}
	def := manualCheckDescriptor()
	def.Columns[0].Default = poison
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := writableTableContext(ctx, def, 0); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation reached default/map processing", err)
	}
	parent, _ := withCheckBudget(context.Background(), 1<<20)
	derived, stop := context.WithCancel(parent)
	stop()
	if _, err := writableTableContext(derived, def, 0); !errors.Is(err, context.Canceled) {
		t.Fatal("derived cancellation bypassed existing owner", err)
	}
	ctx, owner := withCheckBudget(context.Background(), 1<<20)
	owner.maxWork = 0
	if _, err := writableTableContext(ctx, def, 0); !errors.Is(err, ErrMemoryLimit) {
		t.Fatal("exhausted work reached default/map processing", err)
	}
	ctx, _ = withCheckBudget(context.Background(), 32)
	if _, err := writableTableContext(ctx, def, 0); !errors.Is(err, ErrMemoryLimit) {
		t.Fatal("metadata reservation occurred after map/default work", err)
	}
	cycle := &sqlast.Unary{Op: "+"}
	cycle.Expr = cycle
	def = manualCheckDescriptor()
	def.Columns[0].Default = cycle
	if _, err := writableTable(def, 0); !errors.Is(err, ErrUnsupported) {
		t.Fatal("cyclic DEFAULT escaped early shape admission", err)
	}
	// Valid parser-produced defaults, including constant expression trees,
	// still reach the existing default validator and affinity semantics.
	def = automaticTableDefinition(t, "CREATE TABLE t(a NUMERIC DEFAULT (CASE WHEN 1 THEN 2 ELSE 3 END),b TEXT DEFAULT ('x'||'y'),CHECK(a>0))")
	if _, err := writableTable(def, 0); err != nil {
		t.Fatal("valid DEFAULT was narrowed", err)
	}
}

func TestCheckBoundedColumnTypeINMemory(t *testing.T) {
	for _, typ := range []string{"", "INTEGER", "INT", "TEXT", "REAL", "BLOB", "NUMERIC", "VECTOR(1)", "VECTOR(16000)", "HALFVEC(16000)", "BIT(64000)", "SPARSEVEC(1000000000)"} {
		def := manualCheckDescriptor()
		def.Columns[0].Type = typ
		if _, err := writableTable(def, 0); err != nil {
			t.Fatal("canonical unrelated column type rejected", typ, err)
		}
	}
	for _, typ := range []string{"numeric", "VARCHAR", "VECTOR(0)", "VECTOR(01)", "VECTOR(+1)", "VECTOR(16001)", "HALFVEC(16001)", "BIT(64001)", "SPARSEVEC(1000000001)", "VECTOR (2)"} {
		def := manualCheckDescriptor()
		def.Columns[0].Type = typ
		if _, err := writableTable(def, 0); !errors.Is(err, ErrUnsupported) {
			t.Fatal("manual type spelling bypassed parser profile", typ, err)
		}
	}
	one := &sqlast.Literal{Value: int64(1)}
	list := &sqlast.InList{Expr: &sqlast.Column{Name: "a"}, Values: make([]sqlast.Expr, sqlast.MaxCheckNodes-2)}
	for i := range list.Values {
		list.Values[i] = one
	}
	def := manualCheckDescriptor()
	def.CheckConstraints[0].Expr = list
	def.Columns[0].Type = strings.Repeat("q", 65536)
	ctx, owner := withCheckBudget(context.Background(), 1<<20)
	owner.maxWork = 16
	if _, err := writableTableContext(ctx, def, 0); !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), "parser-canonical") || owner.work > 16 {
		t.Fatal("long type reached large-IN/default/name processing", owner.work, err)
	}
	// The same exact-node expression with a canonical type is admissible;
	// a small logical runtime allowance stops it without scanning long types.
	def.Columns[0].Type = "NUMERIC"
	ctx, owner = withCheckBudget(context.Background(), 64<<20)
	tab, err := writableTableContext(ctx, def, 2)
	if err != nil {
		t.Fatal("canonical exact-node IN", err)
	}
	def.Columns[0].Type = strings.Repeat("q", 65536)
	if tab.columns[0].Type != "NUMERIC" {
		t.Fatal("admitted type was mutable through caller's descriptor")
	}
	owner.maxWork = owner.work + 128
	if err := tab.evaluateChecks(ctx, tab.checks, 1, []any{int64(2)}, ErrConstraint); !errors.Is(err, ErrMemoryLimit) || owner.scratch != 0 {
		t.Fatal("bounded IN failed to honor cumulative scalar work", err)
	}
}
