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

type ordinaryCountingReader struct {
	btree.PageReader
	reads  int
	cancel context.CancelFunc
	at     int
}

func (r *ordinaryCountingReader) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	r.reads++
	if r.cancel != nil && r.reads == r.at {
		r.cancel()
	}
	return r.PageReader.ReadPage(ctx, n)
}

// This uses a real, valid native database with an independently instrumented
// PageReader. It proves the prefix cursor does not enumerate preceding keys or
// read their pages; no fault/corruption/recovery experiment is involved.
func TestOrdinaryIndexIndependentRangePageEvidence(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{PageSize: 512})
	reviewVectorExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER,suffix INTEGER)")
	reviewVectorExec(t, db, "CREATE INDEX by_k ON t(k,suffix)")
	ctx := context.Background()
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 1600 {
		if _, err := tx.Exec(ctx, "INSERT INTO t VALUES(?,?,?)", i, i/2, 1600-i); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
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
	idx, _, err := ordinaryReadIndex(ctx, reader, tab, map[int]any{1: int64(799)})
	if err != nil || idx == nil {
		t.Fatal(idx, err)
	}
	counted := &ordinaryCountingReader{PageReader: reader}
	var stats QueryStats
	ids, err := ordinaryIndexRowIDs(ctx, counted, idx, 100, 1<<20, &stats)
	if err != nil || !reflect.DeepEqual(ids, []int64{1598, 1599}) || stats.IndexKeysRead != 2 || stats.IndexPageReads != int64(counted.reads) {
		t.Fatal(ids, stats, counted.reads, err)
	}
	scanReads := &ordinaryCountingReader{PageReader: reader}
	full, err := btree.NewIndexCursor(ctx, scanReads, idx.root, btree.KeyInfo{Columns: len(idx.columns)})
	if err != nil {
		t.Fatal(err)
	}
	var keys int
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
	if keys != 1600 || counted.reads >= scanReads.reads/4 || counted.reads > 6 {
		t.Fatalf("prefix did not skip unrelated index pages: prefix reads=%d, full index reads=%d, full keys=%d", counted.reads, scanReads.reads, keys)
	}
	t.Logf("independent PageReader: prefix=%d reads/2 keys vs full index=%d reads/%d keys", counted.reads, scanReads.reads, keys)

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	interrupt := &ordinaryCountingReader{PageReader: reader, cancel: cancel, at: 2}
	var cancelled QueryStats
	if ids, err := ordinaryIndexRowIDs(cancelCtx, interrupt, idx, 100, 1<<20, &cancelled); ids != nil || !errors.Is(err, context.Canceled) || cancelled.IndexPageReads != 2 || cancelled.RowIDSeeks != 0 {
		t.Fatal("cancellation during range descent must propagate, not become scan fallback", ids, cancelled, err)
	}
}

func TestOrdinaryIndexTotalPredicateProof(t *testing.T) {
	tab := &table{name: "t", columns: []sqlast.ColumnDef{{Name: "id", Type: "INTEGER", PrimaryKey: true}, {Name: "k", Type: "TEXT"}, {Name: "v", Type: "VECTOR(2)"}}, index: map[string]int{"id": 0, "k": 1, "v": 2}, ipk: 0}
	for _, tc := range []struct {
		where string
		safe  bool
	}{
		{"id=42 AND k>'a'", true},
		{"id=42 AND (k='a' OR k IS NULL)", true},
		{"id=42 AND +k/0 IS NULL", true},
		{"id=42 AND k LIKE '%a%'", false},
		{"id=42 AND abs(k)>0", false},
		{"id=42 AND v IS NULL", false},
		{"id=42 AND (v <-> '[0,0]')=0", false},
	} {
		stmt, err := sqlast.Parse("SELECT id FROM t WHERE " + tc.where)
		if err != nil {
			t.Fatal(err)
		}
		r := &Rows{table: tab, aliases: map[string]int{}}
		if got := ordinarySafeWhere(r, stmt.(*sqlast.Select).Where); got != tc.safe {
			t.Fatal(tc.where, got, tc.safe)
		}
	}
	for _, tc := range []struct {
		decl, literal string
		want          bool
	}{
		{"TEXT", "'ordinary'", true},
		{"NUMERIC", "42", true},
		{"INTEGER", "1+2", false},
		{"VECTOR(2)", "'[0,1]'", true},
		{"VECTOR(2)", "'bad'", false},
	} {
		stmt, err := sqlast.Parse("CREATE TABLE t(k " + tc.decl + " DEFAULT (" + tc.literal + "))")
		if err != nil {
			t.Fatal(err)
		}
		columns := stmt.(*sqlast.CreateTable).Columns
		if got := ordinarySafeDefaults(context.Background(), &table{columns: columns}); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}
