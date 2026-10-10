package sqlite

import (
	"context"
	"errors"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
	"reflect"
	"strings"
	"testing"
)

// Intact ordinary SQL and normal query/transaction/cancellation lifetimes only.
func TestGroupedAggregateSemantics(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	for _, tc := range []struct {
		q    string
		want [][]any
	}{
		{"SELECT x,count(*),sum(id) FROM a GROUP BY x", [][]any{{nil, int64(1), int64(3)}, {int64(1), int64(1), int64(1)}, {int64(2), int64(1), int64(2)}}},
		{"SELECT a.x,count(*),sum(b.id) FROM a LEFT JOIN b ON a.x=b.x GROUP BY a.x HAVING count(*)>1", [][]any{{int64(1), int64(2), int64(23)}}},
		{"SELECT x+1 AS k,count(*) AS n FROM a GROUP BY k ORDER BY k DESC LIMIT 1 OFFSET 1", [][]any{{int64(2), int64(1)}}},
		{"SELECT x FROM a GROUP BY 1 ORDER BY 1 NULLS LAST", [][]any{{int64(1)}, {int64(2)}, {nil}}},
		{"SELECT count(*) FROM a HAVING count(*)=3", [][]any{{int64(3)}}},
		{"SELECT count(*) FROM a HAVING count(*)=4", nil},
		{"SELECT count(*) FROM a WHERE 0 HAVING count(*)=0", [][]any{{int64(0)}}},
		{"SELECT x,count(*) FROM a WHERE 0 GROUP BY x", nil},
		{"SELECT 7,count(*) GROUP BY 1", [][]any{{int64(7), int64(1)}}},
		{"SELECT x,count(*) FROM a GROUP BY x HAVING x IS NULL", [][]any{{nil, int64(1)}}},
		{"SELECT x,count(*) FROM a GROUP BY x ORDER BY sum(id) DESC", [][]any{{nil, int64(1)}, {int64(2), int64(1)}, {int64(1), int64(1)}}},
	} {
		got, _, _ := reviewVectorQuery(t, db, tc.q)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: got %#v want %#v", tc.q, got, tc.want)
		}
	}
}
func TestGroupedAggregateBinding(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	for _, tc := range []struct{ q, part string }{
		{"SELECT count(*) FROM a GROUP BY missing LIMIT 0", "no such column"},
		{"SELECT count(*) FROM a GROUP BY count(*) LIMIT 0", "misuse of aggregate"},
		{"SELECT count(*) FROM a GROUP BY 1 LIMIT 0", "misuse of aggregate"},
		{"SELECT x FROM a HAVING count(*)>0", "non-aggregate"},
		{"SELECT x,count(*) FROM a GROUP BY n", "unsupported"},
		{"SELECT x,count(*) FROM a GROUP BY x HAVING n='one'", "unsupported"},
		{"SELECT x,count(*) FROM a GROUP BY 0", "GROUP BY position"},
		{"SELECT x,count(*) FROM a GROUP BY x HAVING sum(sum(id))>0", "misuse of aggregate"},
	} {
		r, err := db.Query(context.Background(), tc.q)
		if r != nil {
			r.Close()
		}
		if err == nil || !strings.Contains(err.Error(), tc.part) {
			t.Fatal(tc.q, err)
		}
	}
}
func TestGroupedAggregateLimitsAndLifetime(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	q := "SELECT CAST(n AS BLOB),count(*) FROM a GROUP BY CAST(n AS BLOB)"
	r, err := db.Query(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	planned := r.RelationalStats().Work
	if r.RelationalStats().SourceOpens != 0 {
		t.Fatal("eager input")
	}
	if !r.Next() {
		t.Fatal(r.Err())
	}
	saved := r.Values()
	saved[0].([]byte)[0] = 'X'
	if string(r.Values()[0].([]byte)) != "one" {
		t.Fatal("aliased key")
	}
	for r.Next() {
	}
	if r.Err() != nil || r.RelationalStats().CurrentBytes != 0 {
		t.Fatal(r.Err(), r.RelationalStats())
	}
	r, err = db.Query(WithRelationalLimits(context.Background(), RelationalLimits{MaxWork: planned + 10}), q)
	if err != nil {
		t.Fatal(err)
	}
	if r.Next() || !errors.Is(r.Err(), ErrQueryWorkLimit) || r.RelationalStats().CurrentBytes != 0 {
		t.Fatal(r.Err(), r.RelationalStats())
	}
	ctx, cancel := context.WithCancel(context.Background())
	r, err = db.Query(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if r.Next() || !errors.Is(r.Err(), context.Canceled) || r.RelationalStats().CurrentBytes != 0 {
		t.Fatal(r.Err(), r.RelationalStats())
	}
	for _, q := range []string{"SELECT x,count(*) FROM a GROUP BY x LIMIT 0", "EXPLAIN QUERY PLAN SELECT x,count(*) FROM a GROUP BY x HAVING count(*)>0"} {
		r, err = db.Query(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		for r.Next() {
		}
		if r.Err() != nil || r.RelationalStats().SourceOpens != 0 {
			t.Fatal(q, r.Err(), r.RelationalStats())
		}
	}
}

func TestGroupedAggregateAtomicBudgetAndAdmission(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	for _, suffix := range []string{"", " ORDER BY x DESC"} {
		q := "SELECT x,hex(CAST(n AS BLOB)),count(*) FROM a GROUP BY x,n" + suffix
		r, err := db.Query(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Next() {
			t.Fatal(r.Err())
		}
		stats := r.RelationalStats()
		r.Close()
		for _, limits := range []RelationalLimits{{MaxWork: stats.Work - 1}, {MaxBytes: stats.PeakBytes - 1}} {
			r, err = db.Query(WithRelationalLimits(context.Background(), limits), q)
			if err == nil {
				if r.Next() {
					t.Fatal("quota published partial grouped answer", limits, r.Values())
				}
				err = r.Err()
				if r.RelationalStats().CurrentBytes != 0 {
					t.Fatal(r.RelationalStats())
				}
			}
			if !errors.Is(err, ErrMemoryLimit) && !errors.Is(err, ErrQueryWorkLimit) {
				t.Fatal(limits, err)
			}
		}
	}
	// Descriptor-free admission: no borrowed long key is retained on refusal.
	b, _ := newRelationalBudget(WithRelationalLimits(context.Background(), RelationalLimits{MaxBytes: 100}), 1<<20)
	s := &groupedAggregateSource{budget: b}
	if _, err := s.group([]any{strings.Repeat("x", 1000)}); !errors.Is(err, ErrMemoryLimit) || len(s.groups) != 0 {
		t.Fatal(err, s.groups)
	}
}
func TestGroupedAggregateSharedSnapshotAndTxLease(t *testing.T) {
	ctx := context.Background()
	db, path := relationalFixture(t, Options{WALShared: true})
	writer, err := Open(path, Options{WALShared: true})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	r, err := db.Query(ctx, "SELECT a.x,count(*) FROM a,b GROUP BY a.x")
	if err != nil {
		t.Fatal(err)
	}
	reviewVectorExec(t, writer, "INSERT INTO b VALUES(14,'later',4)")
	var got [][]any
	for r.Next() {
		got = append(got, r.Values())
	}
	if r.Err() != nil || !reflect.DeepEqual(got, [][]any{{nil, int64(3)}, {int64(1), int64(3)}, {int64(2), int64(3)}}) {
		t.Fatal(got, r.Err())
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO a VALUES(4,'own write',2)"); err != nil {
		t.Fatal(err)
	}
	r, err = tx.Query(ctx, "SELECT x,count(*) FROM a GROUP BY x")
	if err != nil {
		t.Fatal(err)
	}
	if tx.activeRows != 1 {
		t.Fatal(tx.activeRows)
	}
	if !r.Next() {
		t.Fatal(r.Err())
	}
	if err = tx.Commit(ctx); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	got = nil
	got = append(got, r.Values())
	for r.Next() {
		got = append(got, r.Values())
	}
	if r.Err() != nil || !reflect.DeepEqual(got, [][]any{{nil, int64(1)}, {int64(1), int64(1)}, {int64(2), int64(2)}}) {
		t.Fatal(got, r.Err())
	}
	if tx.activeRows != 0 {
		t.Fatal(tx.activeRows)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = db.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestGroupedAggregateKeyStateAdmission(t *testing.T) {
	b, _ := newRelationalBudget(context.Background(), 64<<20)
	s := &groupedAggregateSource{budget: b, template: []sqlast.Aggregate{{Name: "count"}}}
	for i := int64(511); i >= 0; i-- {
		g, err := s.group([]any{i})
		if err != nil {
			t.Fatal(err)
		}
		if err = g.states[0].Step(int64(1)); err != nil {
			t.Fatal(err)
		}
	}
	for i, g := range s.groups {
		if g.keys[0] != int64(i) {
			t.Fatal(i, g.keys)
		}
	}
	for i := range 512 {
		g, err := s.group([]any{float64(i)})
		if err != nil {
			t.Fatal(err)
		}
		if err = g.states[0].Step(int64(1)); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.groups) != 512 {
		t.Fatal(len(s.groups))
	}
	for _, g := range s.groups {
		v, err := g.states[0].Result()
		if err != nil || v != int64(2) {
			t.Fatal(v, err)
		}
	}
	if err := s.Close(); err != nil || b.stats.CurrentBytes != 0 {
		t.Fatal(err, b.stats)
	}
}
func TestGroupedAggregateOffsetAndScalarFailureAtomicity(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	for _, q := range []string{
		"SELECT id,count(*)+abs(-9223372036854775808) FROM a GROUP BY id LIMIT 1 OFFSET 9",
		"SELECT id,CASE WHEN id=1 THEN abs(-9223372036854775808) ELSE count(*) END FROM a GROUP BY id ORDER BY id LIMIT 1 OFFSET 1",
	} {
		r, err := db.Query(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		for r.Next() {
		}
		if r.Err() != nil {
			t.Fatal(q, r.Err())
		}
	}
	r, err := db.Query(context.Background(), "SELECT id,CASE WHEN id=2 THEN abs(-9223372036854775808) ELSE count(*) END FROM a GROUP BY id")
	if err != nil {
		t.Fatal(err)
	}
	if r.Next() || r.Err() == nil || !strings.Contains(r.Err().Error(), "integer overflow") || r.RelationalStats().CurrentBytes != 0 {
		t.Fatal(r.Values(), r.Err(), r.RelationalStats())
	}
}

func TestGroupedAggregateTopKAdmissionAndFailures(t *testing.T) {
	// Descriptor-free candidate admission. A worse key skips ordinary scalar
	// projection errors, but key evaluation and resource failures are immediate.
	makeRows := func(ctx context.Context) *Rows {
		b, _ := newRelationalBudget(ctx, 1<<20)
		return &Rows{ctx: sqlast.WithEvalBudget(ctx, b), relational: b, table: &table{ipk: -1, index: map[string]int{"k": 0}, columns: []sqlast.ColumnDef{{Name: "k"}}},
			exprs: []sqlast.Expr{&sqlast.Call{Name: "abs", Args: []sqlast.Expr{&sqlast.Literal{Value: int64(-9223372036854775808)}}}},
			order: []orderExpr{{expr: &sqlast.Column{Name: "k"}, column: -1}}, sorted: []resultRow{{keys: []any{int64(1)}}}}
	}
	r := makeRows(context.Background())
	if _, _, _, keep, err := r.groupedOrderedCandidate(2, []any{int64(2)}, 1); err != nil || keep {
		t.Fatal(keep, err)
	}
	if r.relational.stats.CurrentBytes != 0 {
		t.Fatal(r.relational.stats)
	}
	for _, want := range []error{ErrMemoryLimit, ErrQueryWorkLimit, context.Canceled} {
		r = makeRows(context.Background())
		r.order[0].expr = &sqlast.Call{Name: "abs", Args: []sqlast.Expr{&sqlast.Column{Name: "k"}}}
		r.functions = map[string]sqlast.Function{"abs": func([]any) (any, error) { return nil, want }}
		if _, _, _, keep, err := r.groupedOrderedCandidate(2, []any{int64(2)}, 1); !errors.Is(err, want) || keep {
			t.Fatal(want, keep, err)
		}
	}
	r = makeRows(context.Background())
	r.relational.maxBytes = 1
	if _, _, _, keep, err := r.groupedOrderedCandidate(2, []any{int64(2)}, 1); !errors.Is(err, ErrMemoryLimit) || keep {
		t.Fatal(keep, err)
	}
	r = makeRows(context.Background())
	r.relational.maxWork = 1
	if _, _, _, keep, err := r.groupedOrderedCandidate(2, []any{int64(2)}, 1); !errors.Is(err, ErrQueryWorkLimit) || keep {
		t.Fatal(keep, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r = makeRows(ctx)
	cancel()
	if _, _, _, keep, err := r.groupedOrderedCandidate(2, []any{int64(2)}, 1); !errors.Is(err, context.Canceled) || keep {
		t.Fatal(keep, err)
	}
}

func TestGroupedAggregateRowidLookaheadAndTopK(t *testing.T) {
	for _, shared := range []bool{false, true} {
		db, _ := relationalFixture(t, Options{WALShared: shared})
		for index, q := range []string{
			"SELECT id,count(*) FROM a WHERE CASE WHEN id=3 THEN abs(-9223372036854775808) ELSE 1 END GROUP BY id LIMIT 1",
			"SELECT id,count(*) FROM a WHERE CASE WHEN id=1 THEN abs(-9223372036854775808) ELSE 1 END GROUP BY id ORDER BY id DESC LIMIT 1",
			"SELECT id,CASE WHEN id=3 THEN abs(-9223372036854775808) ELSE count(*) END FROM a GROUP BY id ORDER BY id+0 LIMIT 1 OFFSET 1",
		} {
			r, err := db.Query(context.Background(), q)
			if err != nil {
				t.Fatal(q, err)
			}
			var got [][]any
			for r.Next() {
				got = append(got, r.Values())
			}
			if r.Err() != nil || len(got) != 1 || r.RelationalStats().CurrentBytes != 0 {
				t.Fatal(q, got, r.Err(), r.RelationalStats())
			}
			if r.RelationalStats().SourceOpens != 1 {
				t.Fatal("additional snapshot/source", r.RelationalStats())
			}
			if index < 2 && (r.RelationalStats().SourceRows != 2 || r.Plan().Method != "grouped_rowid") {
				t.Fatal("missing proved stream/lookahead", r.RelationalStats(), r.Plan())
			}
		}
		r, err := db.Query(context.Background(), "SELECT id,count(*) FROM a WHERE CASE WHEN id=2 THEN abs(-9223372036854775808) ELSE 1 END GROUP BY id LIMIT 1")
		if err != nil {
			t.Fatal(err)
		}
		if r.Next() || r.Err() == nil || !strings.Contains(r.Err().Error(), "integer overflow") || r.RelationalStats().CurrentBytes != 0 {
			t.Fatal(r.Values(), r.Err(), r.RelationalStats())
		}
	}
}
