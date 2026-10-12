package sqlite

import (
	"context"
	"errors"
	"io"
	"math"
	"testing"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// These helper tests use parsed expressions and synthetic ordinary descriptors,
// not database files, typed codecs, custom callbacks or an external process.
func leftWhereAdmissionFixture(t *testing.T, query string) (*relationalScope, *sqlast.Select, sqlast.Expr, bool) {
	t.Helper()
	statement, err := sqlast.Parse(query)
	if err != nil {
		t.Fatal(err)
	}
	sel, ok := statement.(*sqlast.Select)
	if !ok {
		t.Fatal("fixture is not SELECT")
	}
	budget, err := newRelationalBudget(context.Background(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	s := &relationalScope{budget: budget, aggregates: true, slots: &table{ipk: -1, index: map[string]int{}}}
	left := &table{name: "t0", alias: "t", root: 2, ipk: 0, index: map[string]int{}, columns: []sqlast.ColumnDef{
		{Name: "id", Type: "INTEGER", PrimaryKey: true},
		{Name: "flag", Type: "INTEGER"},
		{Name: "v", Type: "INTEGER"},
		{Name: "txt", Type: "TEXT"},
		{Name: "raw", Type: "BLOB"},
	}}
	right := &table{name: "u", root: 3, ipk: -1, index: map[string]int{}, columns: []sqlast.ColumnDef{{Name: "y", Type: "INTEGER"}}}
	for _, source := range []*table{left, right} {
		s.leaves = append(s.leaves, relationalBinding{table: source, offset: len(s.slots.columns)})
		for i, col := range source.columns {
			source.index[col.Name] = i
			name := slotName(len(s.slots.columns))
			s.slots.index[name] = len(s.slots.columns)
			s.slots.columns = append(s.slots.columns, sqlast.ColumnDef{Name: name, Type: col.Type})
		}
		name := slotName(len(s.slots.columns))
		s.slots.index[name] = len(s.slots.columns)
		s.slots.columns = append(s.slots.columns, sqlast.ColumnDef{Name: name, Type: "INTEGER"})
	}
	aliases := map[string]sqlast.Expr{}
	for _, column := range sel.Columns {
		bound, err := s.bind(column.Expr, nil)
		if err != nil {
			t.Fatal(err)
		}
		if column.Alias != "" {
			aliases[fold(column.Alias)] = bound
		}
	}
	bound, err := s.bind(sel.Where, aliases)
	if err != nil {
		t.Fatal(err)
	}
	promoted := false
	if join, ok := sel.Relation.(*sqlast.JoinRelation); ok && join.Kind == "LEFT" {
		on, err := s.bind(join.On, aliases)
		if err != nil {
			t.Fatal(err)
		}
		_, ordinary, err := s.extractOuterAliasTerms(join.On, on, aliases)
		if err != nil {
			t.Fatal(err)
		}
		promoted = len(ordinary) != 0
	}
	return s, sel, bound, promoted
}

// A borrowed-row model, not a VFS/page/cursor fault hook. Its only allocation
// charge models the row that it owns until Close; no descriptor is opened.
type leftWherePrefilterQuotaSource struct {
	budget                             *relationalBudget
	row                                []any
	held                               int64
	next, closes                       int
	fetchWork                          int64
	fetchFailure                       error
	closeScalar, closeBytes, closeHeld int64
}

func (s *leftWherePrefilterQuotaSource) Next() (int64, []any, error) {
	s.next++
	if s.next != 1 {
		return 0, nil, io.EOF
	}
	s.fetchWork, s.fetchFailure = s.budget.stats.Work, s.budget.failed
	if err := s.budget.reserve(64); err != nil {
		return 0, nil, err
	}
	s.held = 64
	return 0, s.row, nil
}

func (s *leftWherePrefilterQuotaSource) Close() error {
	s.closes++
	// Observe before releasing this model row and before Rows.close resets
	// CurrentBytes. This prevents a terminal zero from masking scalar leakage.
	s.closeScalar, s.closeBytes, s.closeHeld = s.budget.scalarBytes, s.budget.stats.CurrentBytes, s.held
	s.budget.release(s.held)
	s.held, s.row = 0, nil
	return nil
}

func TestLeftWherePrefilterRuntimeQuota(t *testing.T) {
	// These are two fixed runtime boundaries, not MaxWork/MaxBytes=1 Query
	// admission failures. No parser, schema, fixture, compile or native source
	// setup is called by this root. Production quota methods are unchanged.
	for _, tc := range []struct {
		name              string
		work, bytes, peak int64
		want              error
	}{
		{name: "comparison_work_after_scratch", work: 61, bytes: 1 << 20, peak: 732, want: ErrQueryWorkLimit},
		{name: "comparison_memory_after_column_scratch", work: 1 << 20, bytes: 92, peak: 92, want: ErrMemoryLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			budget := &relationalBudget{ctx: ctx, maxWork: tc.work, maxBytes: tc.bytes}
			ctx = sqlast.WithEvalBudget(ctx, budget)
			db := &DB{rows: map[*Rows]struct{}{}}
			tab := &table{ipk: -1, index: map[string]int{slotName(0): 0, slotName(1): 1, slotName(2): 2, slotName(3): 3}, columns: []sqlast.ColumnDef{
				{Name: slotName(0), Type: "INTEGER"}, {Name: slotName(1), Type: "INTEGER"},
				{Name: slotName(2), Type: "INTEGER"}, {Name: slotName(3), Type: "INTEGER"},
			}}
			where := &sqlast.Binary{Op: "=", Left: &sqlast.Column{Name: slotName(0)}, Right: &sqlast.Literal{Value: int64(99)}}
			r := &Rows{db: db, ctx: ctx, table: tab, sel: &sqlast.Select{Where: where}, where: &predicateProgram{residual: []sqlast.Expr{where}}, relational: budget, limit: -1}
			left := &leftWherePrefilterQuotaSource{budget: budget, row: []any{int64(0), int64(1)}}
			rightFactories, releases := 0, 0
			join := &relationalJoin{left: left, leftWidth: 2, rightWidth: 2, kind: "LEFT", rows: r, budget: budget, leftWhere: &predicateProgram{residual: []sqlast.Expr{where}}}
			join.rightFactory = func() (source, error) {
				rightFactories++
				return &constantSource{}, nil
			}
			r.source = join
			r.release = func() error { releases++; return nil }
			db.rows[r] = struct{}{}
			t.Cleanup(func() {
				if err := r.Close(); err != nil {
					t.Error(err)
				}
			})

			// Work: join loop 1 + predicate dispatch 1 + binary Eval 1 +
			// INTEGER column 57 + literal 1 = 61. The column holds 28 bytes.
			// Equality preflight requests 640 bytes then 512 work units.
			// Work case: 64 row + 28 column + 640 comparison = 732 peak,
			// then Step(512) fails. Memory case: the 640-byte request fails
			// at 64+28=92 bytes. Both failures are inside this prefilter.
			if r.Next() || !errors.Is(r.Err(), tc.want) || !errors.Is(budget.failed, tc.want) {
				t.Fatal("missing prefilter quota failure", r.Err(), budget.failed)
			}
			if left.next != 1 || left.fetchWork != 1 || left.fetchFailure != nil || rightFactories != 0 {
				t.Fatal("quota failure did not occur after left fetch and before right factory", left.next, left.fetchWork, left.fetchFailure, rightFactories)
			}
			if budget.stats.Work != 61 || budget.stats.PeakBytes != tc.peak || budget.stats.JoinPairs != 0 || budget.stats.NullExtensions != 0 || budget.stats.PageReads != 0 {
				t.Fatal("unexpected runtime failure stage or join work", budget.stats)
			}
			if left.closes != 1 || left.closeScalar != 0 || left.closeBytes != 64 || left.closeHeld != 64 || left.held != 0 || left.row != nil {
				t.Fatal("prefilter scratch or borrowed row ownership escaped automatic close", left)
			}
			if releases != 1 || !join.closed || join.outer != nil || join.left != nil || join.right != nil {
				t.Fatal("terminal runtime error did not retire join and sole model lease", releases, join)
			}
			leftWherePrefilterTerminal(t, db, r)
			stats := r.RelationalStats()
			if err := r.Close(); err != nil || r.Next() || releases != 1 || left.closes != 1 || stats != r.RelationalStats() {
				t.Fatal("explicit Close repeated automatic retirement", err, releases, left.closes)
			}
		})
	}
}

func TestLeftWherePrefilterAdmission(t *testing.T) {
	const prefix = "SELECT t.id FROM t0 t LEFT JOIN u ON 1 WHERE "
	for _, tc := range []struct {
		name, query string
		want        bool
	}{
		{"direct", prefix + "t.flag=99", true},
		{"commuted", prefix + "99=t.flag", true},
		{"double_equals", prefix + "t.flag==99", true},
		{"two_and", prefix + "t.flag=99 AND t.id=1", true},
		{"three_and", prefix + "t.flag=99 AND t.id=1 AND t.txt='01'", true},
		{"parenthesized_and", prefix + "(t.flag=99 AND (t.id=1 AND t.txt='01'))", true},
		{"hidden_rowid", prefix + "t.rowid=1", true},
		{"physical_ipk", prefix + "t.id=1", true},
		{"direct_alias", "SELECT t.flag AS z FROM t0 t LEFT JOIN u ON 1 WHERE z=99", true},
		{"physical_alias_precedence", "SELECT u.y AS flag FROM t0 t LEFT JOIN u ON 1 WHERE flag=99", true},
		{"null_literal", prefix + "t.flag=NULL", true},
		{"string_literal", prefix + "t.txt='01'", true},
		{"blob_literal", prefix + "t.raw=X'3031'", true},
		{"real_literal", prefix + "t.flag=99.0", true},
		{"positive_numeric", prefix + "t.flag=+99", true},
		{"negative_numeric", prefix + "t.v=-99", true},
		{"negative_endpoint", prefix + "t.v=-9223372036854775808", true},
		{"group_without_having", prefix + "t.flag=99 GROUP BY t.id", true},
		{"right_slot", prefix + "u.y=99", false},
		{"column_column", prefix + "t.flag=t.id", false},
		{"mixed_and", prefix + "t.flag=99 AND u.y=2", false},
		{"inner", "SELECT t.id FROM t0 t INNER JOIN u ON 1 WHERE t.flag=99", false},
		{"cross", "SELECT t.id FROM t0 t CROSS JOIN u WHERE t.flag=99", false},
		{"comma", "SELECT t.id FROM t0 t,u WHERE t.flag=99", false},
		{"three_tables", "SELECT t.id FROM t0 t LEFT JOIN u ON 1 LEFT JOIN u extra ON 1 WHERE t.flag=99", false},
		{"or", prefix + "t.flag=99 OR t.id=1", false},
		{"not", prefix + "NOT (t.flag=99)", false},
		{"is", prefix + "t.flag IS 99", false},
		{"in", prefix + "t.flag IN (99)", false},
		{"between", prefix + "t.flag BETWEEN 99 AND 100", false},
		{"like", prefix + "t.txt LIKE '01'", false},
		{"cast_left", prefix + "CAST(t.flag AS INTEGER)=99", false},
		{"cast_right", prefix + "t.flag=CAST('99' AS INTEGER)", false},
		{"call_left", prefix + "abs(t.flag)=99", false},
		{"call_right", prefix + "t.flag=abs(99)", false},
		{"arithmetic_left", prefix + "t.flag+0=99", false},
		{"arithmetic_right", prefix + "t.flag=98+1", false},
		{"arithmetic_alias", "SELECT t.flag+0 AS z FROM t0 t LEFT JOIN u ON 1 WHERE z=99", false},
		{"function_alias", "SELECT abs(t.flag) AS z FROM t0 t LEFT JOIN u ON 1 WHERE z=99", false},
		{"right_alias", "SELECT u.y AS z FROM t0 t LEFT JOIN u ON 1 WHERE z=99", false},
		{"constant_alias", "SELECT 99 AS z FROM t0 t LEFT JOIN u ON 1 WHERE z=99", false},
		{"bare_constant", prefix + "1", false},
		{"promoted_on_alias", "SELECT t.flag AS z FROM t0 t LEFT JOIN u ON z WHERE t.flag=99", false},
		{"having", prefix + "t.flag=99 GROUP BY t.id HAVING t.id=1", false},
		{"parameter", prefix + "t.flag=?1", false},
		{"signed_parameter", prefix + "t.flag=-?1", false},
		{"signed_column", prefix + "t.flag=-t.id", false},
		{"signed_string", prefix + "t.flag=-'99'", false},
		{"nested_sign", prefix + "t.flag=-(-99)", false},
		{"case", prefix + "t.flag=CASE WHEN 1 THEN 99 ELSE 0 END", false},
		{"no_where", "SELECT t.id FROM t0 t LEFT JOIN u ON 1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, sel, bound, promoted := leftWhereAdmissionFixture(t, tc.query)
			before := s.budget.stats.Work
			got, err := s.leftWherePrefilter(sel, bound, promoted)
			if err != nil || got != tc.want || s.budget.stats.Work <= before {
				t.Fatalf("admission=%v want=%v err=%v work=%d before=%d", got, tc.want, err, s.budget.stats.Work, before)
			}
			if s.budget.scalarBytes != 0 {
				t.Fatal("admission evaluated scalar scratch")
			}
		})
	}
	for _, tc := range []struct {
		name   string
		change func(*relationalScope, *sqlast.Select, *sqlast.Binary)
	}{
		{"missing_private_slot", func(s *relationalScope, _ *sqlast.Select, b *sqlast.Binary) {
			b.Left = &sqlast.Column{Name: "\x00missing"}
		}},
		{"negative_private_slot", func(s *relationalScope, _ *sqlast.Select, b *sqlast.Binary) {
			s.slots.index[b.Left.(*sqlast.Column).Name] = -1
		}},
		{"out_of_range_private_slot", func(s *relationalScope, _ *sqlast.Select, b *sqlast.Binary) {
			s.slots.index[b.Left.(*sqlast.Column).Name] = len(s.slots.columns)
		}},
		{"mismatched_private_slot", func(s *relationalScope, _ *sqlast.Select, b *sqlast.Binary) {
			s.slots.index[b.Left.(*sqlast.Column).Name] = 0
		}},
		{"qualified_private_slot", func(_ *relationalScope, _ *sqlast.Select, b *sqlast.Binary) { b.Left.(*sqlast.Column).Table = "t" }},
		{"nested_right_relation", func(_ *relationalScope, sel *sqlast.Select, _ *sqlast.Binary) {
			j := sel.Relation.(*sqlast.JoinRelation)
			j.Right = &sqlast.JoinRelation{Kind: "LEFT", Left: j.Right, Right: &sqlast.TableRelation{Name: "extra"}}
		}},
		{"typed_left", func(s *relationalScope, _ *sqlast.Select, _ *sqlast.Binary) {
			s.leaves[0].table.columns[4].Type = "VECTOR(2)"
		}},
		{"typed_right", func(s *relationalScope, _ *sqlast.Select, _ *sqlast.Binary) {
			s.leaves[1].table.columns[0].Type = "BIT(2)"
		}},
		{"schema_table", func(s *relationalScope, _ *sqlast.Select, _ *sqlast.Binary) { s.leaves[1].table.root = 1 }},
		{"nonliteral_default", func(s *relationalScope, _ *sqlast.Select, _ *sqlast.Binary) {
			s.leaves[1].table.columns[0].Default = &sqlast.Call{Name: "abs", Args: []sqlast.Expr{&sqlast.Literal{Value: int64(math.MinInt64)}}}
		}},
		{"nonordinary_literal", func(_ *relationalScope, _ *sqlast.Select, b *sqlast.Binary) {
			b.Right = &sqlast.Literal{Value: struct{ X int }{1}}
		}},
		{"nan_literal", func(_ *relationalScope, _ *sqlast.Select, b *sqlast.Binary) {
			b.Right = &sqlast.Literal{Value: math.NaN()}
		}},
		{"infinite_literal", func(_ *relationalScope, _ *sqlast.Select, b *sqlast.Binary) {
			b.Right = &sqlast.Literal{Value: math.Inf(1)}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, sel, bound, _ := leftWhereAdmissionFixture(t, prefix+"t.flag=99")
			tc.change(s, sel, bound.(*sqlast.Binary))
			got, err := s.leftWherePrefilter(sel, bound, false)
			if err != nil || got {
				t.Fatalf("admission=%v err=%v", got, err)
			}
		})
	}
	t.Run("parameter_values_are_irrelevant", func(t *testing.T) {
		// The proof has no Params input. Changing these representative bound
		// values cannot turn the original Parameter node into a SQL literal.
		for _, value := range []any{nil, int64(99), "99", []byte("99"), []float32{1, 2}, struct{ Custom int }{99}} {
			s, sel, bound, _ := leftWhereAdmissionFixture(t, prefix+"t.flag=?1")
			r := &Rows{params: []any{value}}
			got, err := s.leftWherePrefilter(sel, bound, false)
			if err != nil || got || len(r.params) != 1 {
				t.Fatalf("bound type %T: admission=%v err=%v", value, got, err)
			}
		}
	})
	t.Run("literal_defaults_are_structural", func(t *testing.T) {
		s, sel, bound, _ := leftWhereAdmissionFixture(t, prefix+"t.flag=99")
		s.leaves[1].table.columns[0].Default = &sqlast.Unary{Op: "-", Expr: &sqlast.Literal{Value: int64(2)}}
		got, err := s.leftWherePrefilter(sel, bound, false)
		if err != nil || !got || s.budget.scalarBytes != 0 {
			t.Fatal(got, err, s.budget.scalarBytes)
		}
	})
	t.Run("proof_work_limit", func(t *testing.T) {
		s, sel, bound, _ := leftWhereAdmissionFixture(t, prefix+"t.flag=99")
		s.budget.maxWork = s.budget.stats.Work + 1
		if got, err := s.leftWherePrefilter(sel, bound, false); got || !errors.Is(err, ErrQueryWorkLimit) {
			t.Fatal(got, err)
		}
	})
	t.Run("proof_cancellation", func(t *testing.T) {
		s, sel, bound, _ := leftWhereAdmissionFixture(t, prefix+"t.flag=99")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		s.budget.ctx = ctx
		if got, err := s.leftWherePrefilter(sel, bound, false); got || !errors.Is(err, context.Canceled) {
			t.Fatal(got, err)
		}
	})
	t.Run("collate_stays_parser_unsupported", func(t *testing.T) {
		if _, err := sqlast.Parse(prefix + "t.flag COLLATE NOCASE=99"); err == nil {
			t.Fatal("COLLATE unexpectedly entered the bound admission domain")
		}
	})
}
