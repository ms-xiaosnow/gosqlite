package sqlite

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// An independent PageReader counts actual native calls over a valid fixture.
// Tiny admissions and cancellation occur in memory, without physical faults.
func TestSecondaryRangeIndependentPagesAndAdmissions(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{PageSize: 512})
	ctx := context.Background()
	reviewVectorExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER,suffix INTEGER)")
	reviewVectorExec(t, db, "CREATE INDEX by_k ON t(k,suffix)")
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 1600 {
		if _, err = tx.Exec(ctx, "INSERT INTO t VALUES(?,?,?)", i-800, i/2, 1600-i); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	reader, err := db.openReader(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	tab, err := db.table(ctx, reader, &sqlast.Select{From: "t"})
	if err != nil {
		t.Fatal(err)
	}
	bound := &secondaryInterval{}
	if err = bound.add(">=", int64(798)); err != nil {
		t.Fatal(err)
	}
	if err = bound.add("<", int64(800)); err != nil {
		t.Fatal(err)
	}
	idx, _, err := secondaryReadIndex(ctx, reader, tab, map[int]*secondaryInterval{1: bound})
	if err != nil || idx == nil {
		t.Fatal(idx, err)
	}
	counted := &ordinaryCountingReader{PageReader: reader}
	var stats QueryStats
	ids, err := secondaryIndexRowIDs(ctx, counted, idx, bound, 100, 1<<20, &stats)
	if err != nil || !reflect.DeepEqual(ids, []int64{796, 797, 798, 799}) || stats.IndexKeysRead != 4 || stats.IndexPageReads != int64(counted.reads) {
		t.Fatal(ids, stats, counted.reads, err)
	}
	fullReads := &ordinaryCountingReader{PageReader: reader}
	full, err := btree.NewIndexCursor(ctx, fullReads, idx.root, btree.KeyInfo{Columns: len(idx.columns)})
	if err != nil {
		t.Fatal(err)
	}
	keys := 0
	for {
		_, err := full.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		keys++
	}
	full.Close()
	if keys != 1600 || counted.reads >= fullReads.reads/4 || counted.reads > 6 {
		t.Fatal(counted.reads, fullReads.reads, keys)
	}
	t.Logf("independent PageReader: inequality=%d calls/4 keys versus full index=%d calls/%d keys", counted.reads, fullReads.reads, keys)
	for _, tc := range []struct {
		budget int
		memory int64
	}{{1, 1 << 20}, {100, 1}, {100, 3000}} {
		var stats QueryStats
		got, err := secondaryIndexRowIDs(ctx, reader, idx, bound, tc.budget, tc.memory, &stats)
		if got != nil || !errors.Is(err, errOrdinaryIndexBudget) {
			t.Fatal(tc, got, stats, err)
		}
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	interrupt := &ordinaryCountingReader{PageReader: reader, cancel: cancel, at: 2}
	var canceled QueryStats
	if got, err := secondaryIndexRowIDs(cancelCtx, interrupt, idx, bound, 100, 1<<20, &canceled); got != nil || !errors.Is(err, context.Canceled) || canceled.IndexPageReads != 2 || canceled.RowIDSeeks != 0 {
		t.Fatal(got, canceled, err)
	}
	for _, hook := range []string{"page", "record"} {
		hctx, hcancel := context.WithCancel(ctx)
		defer hcancel()
		h := &secondaryAdmissionHook{PageReader: reader, cancel: hcancel, hook: hook}
		var s QueryStats
		if got, err := secondaryIndexRowIDs(hctx, h, idx, bound, 100, 1<<20, &s); got != nil || !errors.Is(err, context.Canceled) || h.calls != 1 {
			t.Fatal(hook, got, s, err, h.calls)
		}
	}
	// Bound ownership remains with the caller; construction cannot mutate it.
	original := []byte{0, 255}
	v := &secondaryInterval{}
	if err = v.add(">", original); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, []byte{0, 255}) {
		t.Fatal(original)
	}
}

type secondaryAdmissionHook struct {
	btree.PageReader
	cancel context.CancelFunc
	hook   string
	calls  int
}

func (r *secondaryAdmissionHook) BeforePage(int64) error {
	if r.hook == "page" {
		r.calls++
		r.cancel()
	}
	return nil
}
func (r *secondaryAdmissionHook) BeforeRecord(int) error {
	if r.hook == "record" {
		r.calls++
		r.cancel()
	}
	return nil
}

func TestSecondaryRangePreservesPriorRejectionReason(t *testing.T) {
	stmt, err := sqlast.Parse("SELECT k FROM t WHERE k>1 AND k LIKE '%a%'")
	if err != nil {
		t.Fatal(err)
	}
	tab := &table{name: "t", columns: []sqlast.ColumnDef{{Name: "k", Type: "INTEGER"}}, index: map[string]int{"k": 0}, ipk: -1}
	r := &Rows{table: tab, sel: stmt.(*sqlast.Select), queryPlan: QueryPlan{Access: "SCAN", Reason: "earlier planner rejection"}}
	// A nil reader is deliberate: unsafe residual rejection must neither read
	// pages nor overwrite an existing reason before a candidate is attempted.
	if err = (&DB{}).planSecondaryRange(context.Background(), nil, r); err != nil || r.queryPlan.Reason != "earlier planner rejection" {
		t.Fatal(r.queryPlan, err)
	}
	secondaryRangeRejection(r, "secondary budget")
	if r.queryPlan.Reason != "earlier planner rejection; secondary budget" {
		t.Fatal(r.queryPlan)
	}
}
