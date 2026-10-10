package sqlite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
)

// Only ordinary statements on intact owned fixtures, normal transaction
// lifetime/cancellation, and descriptor-free arithmetic/stream laws below.
func relationalFixture(t *testing.T, options Options) (*DB, string) {
	db, path := reviewVectorDB(t, options)
	for _, q := range []string{
		"CREATE TABLE a(id INTEGER PRIMARY KEY,n TEXT,x INTEGER)",
		"CREATE TABLE b(id INTEGER PRIMARY KEY,n TEXT,x INTEGER)",
		"INSERT INTO a VALUES(1,'one',1),(2,'two',2),(3,'three',NULL)",
		"INSERT INTO b VALUES(11,'first',1),(12,'second',1),(13,'third',3)",
	} {
		reviewVectorExec(t, db, q)
	}
	return db, path
}
func TestRelationalJoinStreamingAndNulls(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	cases := []struct {
		q    string
		want [][]any
	}{
		{"SELECT a.id,b.id FROM a JOIN b ON a.x=b.x", [][]any{{int64(1), int64(11)}, {int64(1), int64(12)}}},
		{"SELECT a.id,b.id FROM a LEFT JOIN b ON a.x=b.x ORDER BY a.id,b.id", [][]any{{int64(1), int64(11)}, {int64(1), int64(12)}, {int64(2), nil}, {int64(3), nil}}},
		{"SELECT a.id,b.id FROM a LEFT JOIN b ON a.x=b.x AND b.id=12 ORDER BY a.id", [][]any{{int64(1), int64(12)}, {int64(2), nil}, {int64(3), nil}}},
		{"SELECT a.id,b.id FROM a LEFT JOIN b ON a.x=b.x WHERE b.id=12", [][]any{{int64(1), int64(12)}}},
		{"SELECT a.rowid,b._rowid_ FROM a CROSS JOIN b LIMIT 2 OFFSET 7", [][]any{{int64(3), int64(12)}, {int64(3), int64(13)}}},
		{"SELECT a.id,b.id FROM a,b WHERE a.id=2 AND b.id=13", [][]any{{int64(2), int64(13)}}},
		{"SELECT a.id,b.id,c.id FROM a LEFT JOIN b ON a.x=b.x LEFT JOIN b c ON c.id=b.id WHERE a.id IN(1,2) ORDER BY 1,2", [][]any{{int64(1), int64(11), int64(11)}, {int64(1), int64(12), int64(12)}, {int64(2), nil, nil}}},
	}
	for _, tc := range cases {
		got, plan, stats := reviewVectorQuery(t, db, tc.q)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatal(tc.q, got, tc.want)
		}
		if plan.Method != "nested_loop" || plan.Access != "JOIN" || stats.BaseRowFetches == 0 {
			t.Fatal(plan, stats)
		}
	}
	rows, err := db.Query(context.Background(), "SELECT a.id,b.id FROM a CROSS JOIN b LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	if rows.RelationalStats().SourceOpens != 0 {
		t.Fatal("query eagerly opened leaf")
	}
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	stats := rows.RelationalStats()
	if stats.SourceOpens != 2 || stats.SourceRows != 2 || stats.JoinPairs != 1 {
		t.Fatal(stats)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if rows.RelationalStats().CurrentBytes != 0 {
		t.Fatal(rows.RelationalStats())
	}
}
func TestRelationalBindingAmbiguityAndLazyErrors(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	for _, q := range []string{
		"SELECT id FROM a JOIN b ON 1", "SELECT rowid FROM a,b", "SELECT a.id AS id FROM a,b WHERE id=1",
		"SELECT a.id FROM a AS x,b", "SELECT a.* FROM a AS x,b", "SELECT x.id FROM a x,b x",
		"SELECT 1 FROM a LEFT JOIN b ON missing=1", "SELECT CASE WHEN 1 THEN 1 ELSE missing END FROM a,b WHERE 0",
	} {
		rows, err := db.Query(context.Background(), q)
		if err == nil {
			rows.Close()
			t.Fatal("accepted", q)
		}
	}
	cases := []struct {
		q    string
		want [][]any
	}{
		{"SELECT a.id+10 AS chosen,b.id FROM a JOIN b ON chosen=b.id ORDER BY 1,2", [][]any{{int64(11), int64(11)}, {int64(12), int64(12)}, {int64(13), int64(13)}}},
		{"SELECT CASE WHEN a.id=1 THEN a.n ELSE 'skip' END FROM a,b WHERE a.id=1 LIMIT 1", [][]any{{"one"}}},
		{"SELECT CASE WHEN 1 THEN 7 ELSE abs(-9223372036854775808) END FROM a,b LIMIT 1", [][]any{{int64(7)}}},
	}
	for _, tc := range cases {
		got, _, _ := reviewVectorQuery(t, db, tc.q)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatal(tc.q, got)
		}
	}
	rows, err := db.Query(context.Background(), "SELECT CASE WHEN b.id=11 THEN 7 ELSE abs(-9223372036854775808) END FROM a,b")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	if rows.Next() || rows.Err() == nil {
		t.Fatal("missing lazy source expression error", rows.Err())
	}
	if rows.RelationalStats().CurrentBytes != 0 {
		t.Fatal(rows.RelationalStats())
	}
	// Projection aliases take precedence in bare ORDER BY, not nested clauses.
	got, _, _ := reviewVectorQuery(t, db, "SELECT -a.id AS id FROM a,b WHERE b.id=11 ORDER BY id")
	if !reflect.DeepEqual(got, [][]any{{int64(-3)}, {int64(-2)}, {int64(-1)}}) {
		t.Fatal(got)
	}
}
func TestRelationalSharedBudgetsCancellationAndExplain(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	q := "SELECT a.id,b.id FROM a,b"
	limited := WithRelationalLimits(context.Background(), RelationalLimits{MaxWork: 30})
	rows, err := db.Query(limited, q)
	if err == nil {
		for rows.Next() {
		}
		err = rows.Err()
		if rows.RelationalStats().CurrentBytes != 0 {
			t.Fatal(rows.RelationalStats())
		}
	}
	if !errors.Is(err, ErrQueryWorkLimit) {
		t.Fatal(err)
	}
	limited = WithRelationalLimits(context.Background(), RelationalLimits{MaxBytes: 32})
	if rows, err = db.Query(limited, q); !errors.Is(err, ErrMemoryLimit) {
		if rows != nil {
			rows.Close()
		}
		t.Fatal(err)
	}
	limited = WithRelationalLimits(context.Background(), RelationalLimits{MaxWork: -1})
	if rows, err = db.Query(limited, q); err == nil {
		rows.Close()
		t.Fatal("negative work accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	rows, err = db.Query(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	cancel()
	if rows.Next() || !errors.Is(rows.Err(), context.Canceled) {
		t.Fatal(rows.Err())
	}
	if rows.RelationalStats().CurrentBytes != 0 {
		t.Fatal(rows.RelationalStats())
	}
	rows, err = db.Query(context.Background(), "EXPLAIN QUERY PLAN "+q)
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	if !strings.Contains(rows.Values()[3].(string), "nested-loop") {
		t.Fatal(rows.Values())
	}
	if rows.RelationalStats().SourceRows != 0 || rows.RelationalStats().SourceOpens != 0 {
		t.Fatal(rows.RelationalStats())
	}
	rows.Close()
	// Enough source memory, but insufficient aggregate sort memory. Failure
	// happens eagerly and releases all child streams and the single root lease.
	reviewVectorExec(t, db, "CREATE TABLE wide(id INTEGER PRIMARY KEY,t TEXT)")
	for i := 1; i <= 8; i++ {
		reviewVectorExec(t, db, "INSERT INTO wide VALUES(?,?)", i, strings.Repeat("x", 2000))
	}
	limited = WithRelationalLimits(context.Background(), RelationalLimits{MaxBytes: 250000})
	rows, err = db.Query(limited, "SELECT a.t,b.t FROM wide a,wide b ORDER BY a.id,b.id")
	if !errors.Is(err, ErrMemoryLimit) {
		if rows != nil {
			rows.Close()
		}
		t.Fatal("unbounded sort", err)
	}
	reviewVectorQuery(t, db, "SELECT a.id FROM a,b LIMIT 1")
}
func TestRelationalSingleSnapshotAndTxOwnership(t *testing.T) {
	db, path := relationalFixture(t, Options{WALShared: true})
	writer, err := Open(path, Options{WALShared: true})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	rows, err := db.Query(context.Background(), "SELECT a.id,b.id FROM a,b")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	reviewVectorExec(t, writer, "INSERT INTO b VALUES(14,'after-snapshot',4)")
	count := 1
	for rows.Next() {
		count++
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 9 {
		t.Fatal("inner restart used a fresh snapshot", count)
	}
	got, _, _ := reviewVectorQuery(t, db, "SELECT a.id,b.id FROM a,b")
	if len(got) != 12 {
		t.Fatal(len(got))
	}
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	rows, err = tx.Query(context.Background(), "SELECT a.id,b.id FROM a,b")
	if err != nil {
		t.Fatal(err)
	}
	if tx.activeRows != 1 {
		t.Fatal("child rows acquired transaction leases", tx.activeRows)
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	if tx.activeRows != 0 {
		t.Fatal(tx.activeRows)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = db.Checkpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func TestRelationalBudgetArithmeticAndScalarPreflight(t *testing.T) {
	b, err := newRelationalBudget(context.Background(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err = b.reserve(math.MaxInt64); !errors.Is(err, ErrMemoryLimit) {
		t.Fatal(err)
	}
	if err = b.reserve(-1); !errors.Is(err, ErrMemoryLimit) {
		t.Fatal(err)
	}
	if err = b.step(1); !errors.Is(err, ErrMemoryLimit) {
		t.Fatal("first quota failure was not terminal", err)
	}
	b, err = newRelationalBudget(context.Background(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err = b.step(math.MaxInt64); !errors.Is(err, ErrQueryWorkLimit) {
		t.Fatal(err)
	}
	if b.stats.Work != 0 || b.stats.CurrentBytes != 0 {
		t.Fatal(b.stats)
	}
	db, _ := relationalFixture(t, Options{})
	// Nested hex calls amplify an ordinary string. The scalar preflight must
	// reject before the exponentially larger result is allocated.
	expr := "a.n"
	for range 20 {
		expr = "hex(" + expr + ")"
	}
	ctx := WithRelationalLimits(context.Background(), RelationalLimits{MaxBytes: 200000})
	rows, err := db.Query(ctx, "SELECT "+expr+" FROM a,b LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	if rows.Next() || !errors.Is(rows.Err(), ErrMemoryLimit) {
		t.Fatal(rows.Err())
	}
	if rows.RelationalStats().PeakBytes > 200000 {
		t.Fatal(rows.RelationalStats())
	}
	t.Log(fmt.Sprintf("peak bounded at %d bytes", rows.RelationalStats().PeakBytes))
}

func TestRelationalRecordPreflightAndWildcardBounds(t *testing.T) {
	db, _ := relationalFixture(t, Options{PageSize: 512})
	reviewVectorExec(t, db, "CREATE TABLE big(id INTEGER PRIMARY KEY,payload TEXT)")
	reviewVectorExec(t, db, "INSERT INTO big VALUES(1,?)", strings.Repeat("z", 8192))
	ctx := WithRelationalLimits(context.Background(), RelationalLimits{MaxBytes: 150000})
	rows, err := db.Query(ctx, "SELECT x.id FROM big x,b y LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	before := rows.RelationalStats().PageReads
	if rows.Next() || !errors.Is(rows.Err(), ErrMemoryLimit) {
		t.Fatal(rows.Err())
	}
	stats := rows.RelationalStats()
	if stats.SourceRows != 0 || stats.PageReads-before != 1 || stats.PeakBytes > 150000 || stats.CurrentBytes != 0 {
		t.Fatal("payload preflight occurred after row/overflow allocation", stats, before)
	}
	// Wildcard expansion is bounded before appending millions of columns.
	q := "SELECT " + strings.Repeat("a.*,", 1000) + "a.* FROM a,b"
	if rows, err = db.Query(context.Background(), q); !errors.Is(err, ErrUnsupported) {
		if rows != nil {
			rows.Close()
		}
		t.Fatal(err)
	}
}

func TestRelationalTypedColumnsAndGramsRemainExact(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE typed(id INTEGER PRIMARY KEY,v VECTOR(3),h HALFVEC(3),bits BIT(3),s SPARSEVEC(3),text TEXT)")
	reviewVectorExec(t, db, "INSERT INTO typed VALUES(1,'[3,0,-4]','[3,0,-4]','101','{1:3,3:-4}/3','数据库')")
	reviewVectorExec(t, db, "CREATE INDEX typed_text ON typed(text) USING grams13")
	for _, tc := range []struct{ col, fn string }{{"v", "vector_dims"}, {"h", "halfvec_dims"}, {"bits", "bit_dims"}, {"s", "sparsevec_dims"}} {
		q := fmt.Sprintf("SELECT %s(CASE WHEN a.id=1 THEN t.%s END) AS dims FROM a JOIN typed t ON a.id=t.id WHERE dims=3", tc.fn, tc.col)
		got, p, stats := reviewVectorQuery(t, db, q)
		if !reflect.DeepEqual(got, [][]any{{int64(3)}}) || p.Method != "nested_loop" || stats.ANNDistanceEvaluations != 0 {
			t.Fatal(q, got, p, stats)
		}
	}
	for _, q := range []string{
		"SELECT a.id FROM a JOIN typed t ON a.id=t.id WHERE grams13_match('typed_text',t.text,'数据')",
		"SELECT a.id FROM a JOIN typed t ON grams13_match('typed_text',t.text,'数据') AND a.id=t.id",
	} {
		got, _, _ := reviewVectorQuery(t, db, q)
		if !reflect.DeepEqual(got, [][]any{{int64(1)}}) {
			t.Fatal(q, got)
		}
	}
	got, p, _ := reviewVectorQuery(t, db, "SELECT a.id,t.v <-> '[0,0,0]' AS distance FROM a CROSS JOIN typed t ORDER BY distance,a.id")
	if len(got) != 3 || got[0][1] != float64(5) || p.VectorRanking != "" || p.Method != "nested_loop" {
		t.Fatal(got, p)
	}
	for _, q := range []string{
		"SELECT vector_dims(CASE WHEN 1 THEN t.h ELSE t.v END) FROM a JOIN typed t ON a.id=t.id",
		"SELECT vector_dims(t.h) FROM a JOIN typed t ON a.id=t.id",
	} {
		rows, err := db.Query(context.Background(), q)
		if err != nil {
			continue
		}
		if rows.Next() || rows.Err() == nil {
			t.Fatal("typed identity lost", q)
		}
		rows.Close()
	}
}

func TestRelationalLikeScratchAndNullLaziness(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	// A long ASCII pattern costs a rune slice plus a grown term slice even
	// against an empty input. Its preflight must reject under the shared cap.
	ctx := WithRelationalLimits(context.Background(), RelationalLimits{MaxBytes: 180000, MaxWork: 1000000})
	rows, err := db.Query(ctx, "SELECT '' LIKE ? FROM a,b LIMIT 1", strings.Repeat("a", 40000))
	if err != nil {
		t.Fatal(err)
	}
	if rows.Next() || !errors.Is(rows.Err(), ErrMemoryLimit) {
		t.Fatal(rows.Err())
	}
	if rows.RelationalStats().PeakBytes > 180000 {
		t.Fatal(rows.RelationalStats())
	}
	// NULL propagation allocates no dense-conversion scratch; the unsupported
	// value in the other operand is not decoded or scored.
	rows, err = db.Query(ctx, "SELECT vector_from_sparsevec(NULL),NULL <-> 'invalid',NULL LIKE ? FROM a,b LIMIT 1", strings.Repeat("a", 1000))
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || !reflect.DeepEqual(rows.Values(), []any{nil, nil, nil}) {
		t.Fatal(rows.Values(), rows.Err())
	}
	rows.Close()
}

func TestRelationalBooleanNamesAndOrderOrdinals(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	reviewVectorExec(t, db, `CREATE TABLE boolnames(id INTEGER PRIMARY KEY,"true" INTEGER,"false" INTEGER)`)
	reviewVectorExec(t, db, `INSERT INTO boolnames VALUES(1,9,2),(2,3,8),(3,6,5)`)
	for _, from := range []string{"boolnames a", "boolnames a,b WHERE b.id=11"} {
		for _, tc := range []struct {
			order string
			want  [][]any
		}{
			{"true", [][]any{{int64(2)}, {int64(3)}, {int64(1)}}},
			{"false", [][]any{{int64(1)}, {int64(3)}, {int64(2)}}},
			{"+true", [][]any{{int64(2)}, {int64(3)}, {int64(1)}}},
			{"(true) DESC", [][]any{{int64(1)}, {int64(3)}, {int64(2)}}},
		} {
			got, _, _ := reviewVectorQuery(t, db, "SELECT a.id FROM "+from+" ORDER BY "+tc.order)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatal(from, tc.order, got)
			}
		}
	}
	for _, from := range []string{"a", "a,b WHERE b.id=11"} {
		for _, key := range []string{"TRUE", "FALSE", "+TRUE", "-FALSE"} {
			got, _, _ := reviewVectorQuery(t, db, "SELECT -a.id FROM "+from+" ORDER BY "+key)
			if !reflect.DeepEqual(got, [][]any{{int64(-1)}, {int64(-2)}, {int64(-3)}}) {
				t.Fatal(from, key, got)
			}
		}
		for _, key := range []string{"true", "false"} {
			got, _, _ := reviewVectorQuery(t, db, "SELECT -a.id AS \""+key+"\" FROM "+from+" ORDER BY "+key)
			if !reflect.DeepEqual(got, [][]any{{int64(-3)}, {int64(-2)}, {int64(-1)}}) {
				t.Fatal(from, key, got)
			}
		}
	}
}

type relationalLifetimeSource struct {
	observe func()
	closes  int
}

func (s *relationalLifetimeSource) Next() (int64, []any, error) { s.observe(); return 0, nil, io.EOF }
func (s *relationalLifetimeSource) Close() error                { s.closes++; return nil }

func TestRelationalPreviousResultAndSortWorkAccounting(t *testing.T) {
	b, err := newRelationalBudget(context.Background(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	r := &Rows{db: &DB{rows: map[*Rows]struct{}{}}, ctx: context.Background(), relational: b, limit: -1, current: []any{strings.Repeat("x", 4096)}}
	b.resultBytes = valuesSize(r.current)
	if err = b.reserve(b.resultBytes); err != nil {
		t.Fatal(err)
	}
	source := &relationalLifetimeSource{observe: func() {
		if r.current != nil || b.resultBytes != 0 {
			t.Fatal("previous generated result retained after releasing charge", b.resultBytes)
		}
	}}
	r.source = source
	if r.Next() || r.Err() != nil {
		t.Fatal(r.Err())
	}
	r.Close()
	if source.closes != 1 {
		t.Fatal(source.closes)
	}
	b, err = newRelationalBudget(WithRelationalLimits(context.Background(), RelationalLimits{MaxWork: 100}), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	r.relational = b
	r.order = []orderExpr{{}}
	_, err = r.compareRelationalRows(resultRow{keys: []any{strings.Repeat("a", 1000)}}, resultRow{keys: []any{strings.Repeat("a", 1000)}})
	if !errors.Is(err, ErrQueryWorkLimit) {
		t.Fatal("long common-prefix comparison escaped work budget", err)
	}
}

func TestRelationalPlanningBudgetIncludesAliasExpansion(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	large := "a.id" + strings.Repeat("+a.id", 80)
	refs := strings.Repeat("chosen+", 80) + "chosen"
	q := "SELECT " + large + " AS chosen FROM a,b WHERE " + refs + " LIMIT 0"
	rows, err := db.Query(WithRelationalLimits(context.Background(), RelationalLimits{MaxWork: 3000}), q)
	if !errors.Is(err, ErrQueryWorkLimit) {
		if rows != nil {
			rows.Close()
		}
		t.Fatal("LIMIT0 bypassed alias planning work", err)
	}
	for _, clause := range []string{"WHERE ?", "JOIN b ON ?"} {
		q := "SELECT a.id FROM a,b " + clause
		if strings.HasPrefix(clause, "JOIN") {
			q = "SELECT a.id FROM a " + clause
		}
		rows, err = db.Query(WithRelationalLimits(context.Background(), RelationalLimits{MaxWork: 3000}), q, strings.Repeat("0", 10000))
		if err == nil {
			for rows.Next() {
			}
			err = rows.Err()
		}
		if !errors.Is(err, ErrQueryWorkLimit) {
			t.Fatal(q, err)
		}
	}
}
