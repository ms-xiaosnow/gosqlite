package sqlite

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// Intact owned fixtures and ordinary query/transaction/cancellation lifetimes.
func TestGlobalAggregatePullAndSemantics(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	for _, tc := range []struct {
		q    string
		want [][]any
	}{
		{"SELECT count(*),count(x),sum(x),total(x),avg(x),min(x),max(x) FROM a", [][]any{{int64(3), int64(2), int64(3), float64(3), float64(1.5), int64(1), int64(2)}}},
		{"SELECT count(*),count(x),sum(x),total(x),avg(x),min(x),max(x) FROM a WHERE 0", [][]any{{int64(0), int64(0), nil, float64(0), nil, nil, nil}}},
		{"SELECT count(*),sum(a.x),count(b.id),max(b.n) FROM a LEFT JOIN b ON a.x=b.x", [][]any{{int64(4), int64(4), int64(2), "second"}}},
		{"SELECT count(*),count(NULL),sum(2),total(2),avg(2),min('x'),max(x'01')", [][]any{{int64(1), int64(0), int64(2), float64(2), float64(2), "x", []byte{1}}}},
		{"SELECT typeof(sum(x)),sum(x)+1,coalesce(avg(x),0) FROM a", [][]any{{"integer", int64(4), float64(1.5)}}},
		{"SELECT count(abs(-9223372036854775808)),sum(abs(-9223372036854775808)) FROM a WHERE 0", [][]any{{int64(0), nil}}},
		{"SELECT count(*) AS n FROM a ORDER BY n", [][]any{{int64(3)}}},
		{"SELECT count(*) FROM a ORDER BY 1 DESC LIMIT 1", [][]any{{int64(3)}}},
	} {
		got, plan, stats := reviewVectorQuery(t, db, tc.q)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatal(tc.q, got, tc.want)
		}
		if plan.Access != "AGGREGATE" || plan.Method != "global" || stats.ReturnedRows != 1 {
			t.Fatal(plan, stats)
		}
	}
	r, err := db.Query(context.Background(), "SELECT count(*) AS n FROM a")
	if err != nil {
		t.Fatal(err)
	}
	if r.Stats().BaseRowFetches != 0 || r.RelationalStats().SourceOpens != 0 {
		t.Fatal("not pull", r.Stats(), r.RelationalStats())
	}
	if !r.Next() || r.Values()[0] != int64(3) || r.Stats().BaseRowFetches != 3 {
		t.Fatal(r.Values(), r.Err(), r.Stats())
	}
	if r.Next() || r.Err() != nil {
		t.Fatal(r.Err())
	}
	if r.RelationalStats().CurrentBytes != 0 {
		t.Fatal(r.RelationalStats())
	}
	for _, q := range []string{"SELECT count(*) FROM a LIMIT 0", "SELECT count(*) FROM a LIMIT 1 OFFSET 1", "SELECT count(*) FROM a LIMIT 0 OFFSET 1"} {
		r, err := db.Query(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		if r.Next() || r.Err() != nil {
			t.Fatal(q, r.Values(), r.Err())
		}
		if strings.Contains(q, "LIMIT 0") && r.Stats().BaseRowFetches != 0 {
			t.Fatal(r.Stats())
		}
	}
}
func TestGlobalAggregateBindingAndBoundaries(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	for _, tc := range []struct{ q, category string }{
		{"SELECT sum(missing) FROM a LIMIT 0", "no such column"},
		{"SELECT sum(abs()) FROM a WHERE 0 LIMIT 0", "wrong number"},
		{"SELECT sum(sum(x)) FROM a LIMIT 0", "misuse of aggregate"},
		{"SELECT count(*) FROM a WHERE sum(x)>0 LIMIT 0", "misuse of aggregate"},
		{"SELECT count(*) AS cnt FROM a WHERE cnt>0 LIMIT 0", "misuse of aggregate"},
		{"SELECT count(*) FROM a,b ON count(*)>0 LIMIT 0", "misuse of aggregate"},
		{"SELECT count(*) FROM a LIMIT sum(1)", "misuse of aggregate"},
		{"SELECT count(1,2) FROM a LIMIT 0", "wrong number"},
		{"SELECT x,count(*) FROM a LIMIT 0", "unsupported"},
		{"SELECT min(x,1) FROM a LIMIT 0", "unsupported"},
		{"SELECT sum(x) FROM a ORDER BY sum(x)", "unsupported"},
	} {
		r, err := db.Query(context.Background(), tc.q)
		if r != nil {
			r.Close()
		}
		if err == nil || !strings.Contains(err.Error(), tc.category) {
			t.Fatal(tc.q, err)
		}
	}
	for _, q := range []string{"SELECT sum(abs(-9223372036854775808)) FROM a", "SELECT count(*)+abs(-9223372036854775808) FROM a WHERE 0"} {
		r, err := db.Query(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		if r.Next() || r.Err() == nil || !strings.Contains(r.Err().Error(), "integer overflow") {
			t.Fatal(q, r.Err())
		}
		if r.RelationalStats().CurrentBytes != 0 {
			t.Fatal(r.RelationalStats())
		}
	}
	r, err := db.Query(context.Background(), "SELECT sum(x+?) AS total FROM a WHERE x>? ORDER BY total LIMIT ?", int64(10), int64(0), int64(1))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if !r.Next() || r.Values()[0] != int64(23) {
		t.Fatal(r.Values(), r.Err())
	}
}
func TestGlobalAggregateLimitsAndCancel(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	ctx, cancel := context.WithCancel(context.Background())
	r, err := db.Query(ctx, "SELECT count(*) FROM a,b")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if r.Next() || !errors.Is(r.Err(), context.Canceled) || r.RelationalStats().CurrentBytes != 0 {
		t.Fatal(r.Err(), r.RelationalStats())
	}
	for _, lim := range []RelationalLimits{{MaxWork: 1}, {MaxBytes: 1}, {MaxWork: -1}} {
		r, err := db.Query(WithRelationalLimits(context.Background(), lim), "SELECT count(*) FROM a LIMIT 0")
		if r != nil {
			r.Close()
		}
		if err == nil {
			t.Fatal(lim)
		}
	}
	r, err = db.Query(context.Background(), "SELECT count(*) FROM a,b")
	if err != nil {
		t.Fatal(err)
	}
	planned := r.RelationalStats().Work
	r.Close()
	r, err = db.Query(WithRelationalLimits(context.Background(), RelationalLimits{MaxWork: planned + 10}), "SELECT count(*) FROM a,b")
	if err != nil {
		t.Fatal(err)
	}
	if r.Next() || !errors.Is(r.Err(), ErrQueryWorkLimit) || r.RelationalStats().CurrentBytes != 0 {
		t.Fatal(r.Err(), r.RelationalStats())
	}
	r, err = db.Query(context.Background(), "EXPLAIN QUERY PLAN SELECT count(*) FROM a,b")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Next() || !strings.Contains(r.Values()[3].(string), "AGGREGATE") || r.RelationalStats().SourceOpens != 0 {
		t.Fatal(r.Values(), r.Err())
	}
	r.Close()
}

func TestGlobalAggregateSharedSnapshotAndTxLease(t *testing.T) {
	ctx := context.Background()
	db, path := relationalFixture(t, Options{WALShared: true})
	writer, err := Open(path, Options{WALShared: true})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	rows, err := db.Query(ctx, "SELECT count(*),sum(a.id) FROM a,b")
	if err != nil {
		t.Fatal(err)
	}
	// The root has acquired its read snapshot, but no aggregate leaf opened yet.
	reviewVectorExec(t, writer, "INSERT INTO b VALUES(14,'after snapshot',4)")
	if !rows.Next() || !reflect.DeepEqual(rows.Values(), []any{int64(9), int64(18)}) {
		t.Fatal(rows.Values(), rows.Err())
	}
	saved := rows.Values()
	if rows.Next() || rows.Err() != nil {
		t.Fatal(rows.Err())
	}
	if !reflect.DeepEqual(saved, []any{int64(9), int64(18)}) {
		t.Fatal(saved)
	}
	got, _, _ := reviewVectorQuery(t, db, "SELECT count(*),sum(a.id) FROM a,b")
	if !reflect.DeepEqual(got, [][]any{{int64(12), int64(24)}}) {
		t.Fatal(got)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rows, err = tx.Query(ctx, "SELECT count(*) FROM a,b")
	if err != nil {
		t.Fatal(err)
	}
	if tx.activeRows != 1 {
		t.Fatal("extra aggregate input lease", tx.activeRows)
	}
	if err := tx.Commit(ctx); !errors.Is(err, ErrBusy) {
		t.Fatal("active root must pin transaction", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if tx.activeRows != 0 {
		t.Fatal(tx.activeRows)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	reviewVectorExec(t, db, "BEGIN")
	reviewVectorExec(t, db, "INSERT INTO a VALUES(4,'transaction',4)")
	got, _, _ = reviewVectorQuery(t, db, "SELECT count(*),sum(x) FROM a")
	if !reflect.DeepEqual(got, [][]any{{int64(4), int64(7)}}) {
		t.Fatal(got)
	}
	reviewVectorExec(t, db, "ROLLBACK")
	got, _, _ = reviewVectorQuery(t, db, "SELECT count(*),sum(x) FROM a")
	if !reflect.DeepEqual(got, [][]any{{int64(3), int64(3)}}) {
		t.Fatal(got)
	}
	if err := db.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestGlobalAggregateTypedBoundaryAndValueLifetime(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	for _, fn := range []string{"count", "sum", "total", "avg", "min", "max"} {
		q := "SELECT " + fn + "(vector('bad')) FROM a"
		r, err := db.Query(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		if r.Next() || r.Err() == nil {
			t.Fatal(q, r.Values(), r.Err())
		}
		r, err = db.Query(context.Background(), q+" WHERE 0")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Next() || r.Err() != nil {
			t.Fatal(q, r.Err())
		}
		r.Close()
	}
	gotValid, _, _ := reviewVectorQuery(t, db, "SELECT count(vector('[1,2]')),sum(vector('[1,2]')),typeof(min(vector('[1,2]'))),vector_to_text(max(vector('[1,2]'))) FROM a")
	if !reflect.DeepEqual(gotValid, [][]any{{int64(3), float64(0), "blob", "[1,2]"}}) {
		t.Fatal(gotValid)
	}
	r, err := db.Query(context.Background(), "SELECT max(CAST(n AS BLOB)),min(n) FROM a")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Next() {
		t.Fatal(r.Err())
	}
	got := r.Values()
	if !reflect.DeepEqual(got, []any{[]byte("two"), "one"}) {
		t.Fatal(got)
	}
	got[0].([]byte)[0] = 'x'
	if string(r.Values()[0].([]byte)) != "two" {
		t.Fatal("result aliases retained state")
	}
	r.Close()
	if string(got[0].([]byte)) != "xwo" {
		t.Fatal(got)
	}
}

func TestGlobalAggregateOffsetProjectionSuppression(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	r, err := db.Query(context.Background(), "SELECT count(*)+abs(-9223372036854775808) FROM a LIMIT 1 OFFSET 1")
	if err != nil {
		t.Fatal(err)
	}
	if r.Next() || r.Err() != nil {
		t.Fatal(r.Values(), r.Err())
	}
	if r.Stats().BaseRowFetches != 3 {
		t.Fatal("OFFSET skipped aggregate input", r.Stats())
	}
}

func TestGlobalAggregateBoundsBindingAndExecution(t *testing.T) {
	db, _ := relationalFixture(t, Options{})
	for _, q := range []string{"SELECT count(*) FROM a LIMIT 0 OFFSET NULL", "SELECT count(*) FROM a LIMIT 0 OFFSET 1.5", "SELECT count(*) FROM a LIMIT 0 OFFSET abs(-9223372036854775808)"} {
		r, err := db.Query(context.Background(), q)
		if err != nil {
			t.Fatal(q, err)
		}
		if r.Next() || r.Err() != nil || r.Stats().BaseRowFetches != 0 {
			t.Fatal(q, r.Err(), r.Stats())
		}
	}
	r, err := db.Query(context.Background(), "SELECT count(*) FROM a LIMIT ? OFFSET ?", 0, "not an integer")
	if err != nil {
		t.Fatal(err)
	}
	if r.Next() || r.Err() != nil {
		t.Fatal(r.Err())
	}
	for _, q := range []string{"EXPLAIN QUERY PLAN SELECT count(*) FROM a LIMIT NULL", "EXPLAIN QUERY PLAN SELECT count(*) FROM a LIMIT abs(-9223372036854775808) OFFSET 1.5"} {
		r, err := db.Query(context.Background(), q)
		if err != nil {
			t.Fatal(q, err)
		}
		if !r.Next() || r.Err() != nil || r.RelationalStats().SourceOpens != 0 || r.Stats().BaseRowFetches != 0 {
			t.Fatal(q, r.Err(), r.RelationalStats())
		}
		r.Close()
	}
	for _, tc := range []struct{ q, category string }{
		{"SELECT count(*) FROM a LIMIT NULL OFFSET missing", "no such column"},
		{"SELECT count(*) FROM a LIMIT abs(-9223372036854775808) OFFSET abs()", "wrong number of arguments"},
		{"SELECT count(*) FROM a LIMIT 0 OFFSET count(*)", "misuse of aggregate"},
		{"EXPLAIN QUERY PLAN SELECT count(*) FROM a LIMIT NULL OFFSET missing", "no such column"},
	} {
		r, err := db.Query(context.Background(), tc.q)
		if r != nil {
			r.Close()
		}
		if err == nil || !strings.Contains(err.Error(), tc.category) {
			t.Fatal(tc.q, err)
		}
	}
}
