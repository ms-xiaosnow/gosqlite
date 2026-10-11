package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// All fixtures use ordinary SQL on intact owned files. Only scalar, quota and
// cancellation errors are induced; no physical I/O or publication is faulted.
func TestSecondaryRangeStorageClassesAndAffinities(t *testing.T) {
	db, _ := writable(t, sqlite.Options{PageSize: 512})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k,n NUMERIC,txt TEXT,r REAL,b BLOB,suffix INTEGER)")
	for _, col := range []string{"k", "n", "txt", "r", "b"} {
		execute(t, db, "CREATE INDEX by_"+col+" ON t("+col+" COLLATE BINARY ASC,suffix)")
	}
	values := []any{nil, int64(math.MinInt64), int64(math.MinInt64 + 1), int64(-9007199254740993), int64(-1), int64(0), int64(1), int64(42), int64(9007199254740993), int64(math.MaxInt64), -1.5, 1.5, 42.0, 9223372036854775808.0, math.Inf(-1), math.Inf(1), "", "01", "42", "42.0", "4.2e1", "text", "数据库🙂", "x\x00y", []byte{}, []byte{0, 255}, []byte("42"), strings.Repeat("overflow界", 300)}
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range values {
		for repeat := range 2 {
			txExecute(t, tx, "INSERT INTO t VALUES(?,?,?,?,?,?,?)", int64(2*i+repeat-30), v, v, v, v, v, 60-2*i-repeat)
		}
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	checks := 0
	for _, col := range []string{"k", "n", "txt", "r", "b"} {
		for _, op := range []string{"<", "<=", ">", ">="} {
			for _, v := range values {
				for _, where := range []string{col + op + "?", "?" + op + col} {
					q := "SELECT id,typeof(" + col + ")," + col + " FROM t WHERE " + where + " ORDER BY " + col + ",id DESC"
					p, s, ss := ordinaryCompare(t, db, q, v)
					if p.Method != "btree_range" || p.Access != "SEARCH" || !p.Recheck || s.IndexKeysRead != s.Candidates || s.RowIDSeeks != s.BaseRowFetches || s.BaseRowFetches > ss.BaseRowFetches {
						t.Fatalf("%s %#v: %+v %+v scan=%+v", q, v, p, s, ss)
					}
					if v == nil && (p.EstimatedCandidates != 0 || s.IndexPageReads != 0 || s.Candidates != 0) {
						t.Fatal("NULL bound", p, s)
					}
					checks++
				}
			}
		}
	}
	for _, where := range []string{"k>=1 AND k<=42", "k<=42 AND k>=1", "k>42 AND k<1", "k<1 AND k>42", "k>1 AND k<=1", "k>=1 AND k<=1", "k>NULL AND k<42", "k<42 AND k>NULL", "k>0 AND k>=1 AND k<42 AND k<=2", "k>1 AND (b='42' OR b IS NULL)", "k>+? AND k< -?"} {
		args := []any(nil)
		if strings.Contains(where, "?") {
			args = []any{int64(-2), int64(-42)}
		}
		p, _, _ := ordinaryCompare(t, db, "SELECT id,k FROM t WHERE "+where+" ORDER BY k,id", args...)
		if p.Method != "btree_range" {
			t.Fatal(where, p)
		}
	}
	t.Logf("%d leading inequality affinity/storage-class/operand-side comparisons matched genuine scans", checks)
}

func TestSecondaryRangeWorkOrderingPrecedenceAndBinding(t *testing.T) {
	db, _ := writable(t, sqlite.Options{PageSize: 512})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER,suffix INTEGER,tie INTEGER,rowid TEXT)")
	execute(t, db, "CREATE INDEX by_k ON t(k,suffix)")
	execute(t, db, "CREATE INDEX by_shadow ON t(rowid)")
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := range 1800 {
		txExecute(t, tx, "INSERT INTO t VALUES(?,?,?,0,'shadow')", i-900, i%90, 1800-i)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, tail := range []string{"", " LIMIT 7 OFFSET 3", " ORDER BY tie LIMIT 7 OFFSET 3", " ORDER BY tie DESC LIMIT 7 OFFSET 3", " ORDER BY k DESC NULLS FIRST,suffix ASC,id DESC LIMIT 7 OFFSET 3", " ORDER BY k NULLS LAST,id"} {
		p, s, ss := ordinaryCompare(t, db, "SELECT id,k,suffix FROM t WHERE k>=88 AND k<90"+tail)
		if p.Method != "btree_range" || s.IndexKeysRead != 40 || s.Candidates != 40 || s.IndexPageReads >= 30 || s.BaseRowFetches > 40 || s.BaseRowFetches >= ss.BaseRowFetches || !strings.Contains(p.Reason, "order_satisfied=false") {
			t.Fatal(tail, p, s, ss)
		}
		t.Logf("%q: %d index keys/%d pages/%d source rows versus %d forced-scan rows", tail, s.IndexKeysRead, s.IndexPageReads, s.BaseRowFetches, ss.BaseRowFetches)
	}
	// NOT INDEXED still permits a declared IPK lookup. Keep this case separate
	// from ordinaryCompare's unchanged genuine-scan contract for every range
	// case. Integer id+0 is value-preserving for these bounded fixture keys and
	// deliberately prevents direct-key admission in the scan control.
	for _, probe := range []struct {
		name, query, scan string
		want              [][]any
	}{
		{"ipk_rejecting_residual", "SELECT id FROM t WHERE k>88 AND id=1", "SELECT id FROM t NOT INDEXED WHERE k>88 AND id+0=1", nil},
		{"ipk_matching_residual", "SELECT id FROM t WHERE k>88 AND id=899", "SELECT id FROM t NOT INDEXED WHERE k>88 AND id+0=899", [][]any{{int64(899)}}},
	} {
		t.Run(probe.name, func(t *testing.T) {
			got, p, s := gramResults(t, db, probe.query)
			notIndexed, np, ns := gramResults(t, db, strings.Replace(probe.query, "FROM t", "FROM t NOT INDEXED", 1))
			scan, sp, ss := gramResults(t, db, probe.scan)
			if !reflect.DeepEqual(got, probe.want) || !reflect.DeepEqual(notIndexed, probe.want) || !reflect.DeepEqual(scan, probe.want) {
				t.Fatal("IPK/NOT INDEXED/genuine scan result mismatch", got, notIndexed, scan, probe.want)
			}
			for _, actual := range []struct {
				plan  sqlite.QueryPlan
				stats sqlite.QueryStats
			}{{p, s}, {np, ns}} {
				p, s := actual.plan, actual.stats
				if p.Access != "SEARCH" || p.Method != "rowid" || p.MethodVersion != 1 || !p.Recheck || len(p.Indexes) != 0 || !reflect.DeepEqual(p.Fields, []string{"rowid"}) || p.EstimatedCandidates != 1 {
					t.Fatal("IPK precedence or recheck lost", p)
				}
				if s.IndexKeysRead != 0 || s.IndexPageReads != 0 || s.RowIDSeeks != 1 || s.BaseRowFetches != 1 || s.Candidates != 1 || s.ReturnedRows != int64(len(probe.want)) {
					t.Fatal("IPK attempted work changed", s)
				}
			}
			if sp.Access != "SCAN" || sp.Method != "" || sp.Reason != "NOT INDEXED requested" || ss.IndexKeysRead != 0 || ss.IndexPageReads != 0 || ss.RowIDSeeks != 0 || ss.BaseRowFetches != 1800 || ss.ReturnedRows != int64(len(probe.want)) {
				t.Fatal("independent control is not a genuine complete scan", sp, ss)
			}
			t.Logf("SQL=%q rows=%#v IPK=%+v NOT_INDEXED=%+v full_scan=%+v", probe.query, got, s, ns, ss)
		})
	}
	for _, tc := range []struct{ q, method string }{
		{"SELECT id FROM t WHERE k>88 AND id>0", "rowid_range"},
		{"SELECT id FROM t WHERE k>88 ORDER BY id DESC", "rowid_range"},
		{"SELECT id FROM t WHERE k=89 AND suffix>1000", "btree"},
		{"SELECT k AS renamed,id FROM t WHERE renamed>88 ORDER BY k,id", "btree_range"},
		{"SELECT id FROM t WHERE rowid>'s'", "btree_range"},
		{"SELECT id FROM t WHERE suffix>1000", ""},
		{"SELECT id FROM t WHERE k>88 OR k<1", ""},
		{"SELECT id FROM t WHERE k+0>88", ""},
	} {
		p, _, _ := ordinaryCompare(t, db, tc.q)
		if p.Method != tc.method {
			t.Fatal(tc, p)
		}
	}
	for _, q := range []string{"SELECT id FROM t WHERE k>=? AND k<? LIMIT 0", "EXPLAIN QUERY PLAN SELECT id FROM t WHERE k>=? AND k<?"} {
		r, err := db.Query(context.Background(), q, 88, 90)
		if err != nil {
			t.Fatal(err)
		}
		p := r.Plan()
		if p.Method != "btree_range" || r.Stats().IndexKeysRead != 0 || r.Stats().IndexPageReads != 0 || r.Stats().RowIDSeeks != 0 {
			t.Fatal(p, r.Stats())
		}
		p.Indexes[0] = "changed"
		p.Fields[0] = "changed"
		if r.Plan().Indexes[0] != "by_k" || r.Plan().Fields[0] != "k" {
			t.Fatal("plan ownership", r.Plan())
		}
		before := r.Plan()
		for r.Next() {
		}
		if r.Err() != nil || !reflect.DeepEqual(before, r.Plan()) {
			t.Fatal("plan changed", r.Plan(), r.Err())
		}
		r.Close()
	}
	for _, q := range []string{"SELECT missing FROM t WHERE k>? LIMIT 0", "SELECT id FROM t WHERE k>? LIMIT 0", "SELECT id FROM t WHERE k>1 ORDER BY missing LIMIT 0", "SELECT id FROM t WHERE k>1 LIMIT 'bad'", "SELECT id FROM t WHERE k >", "SELECT id FROM t WHERE k >< 1"} {
		r, err := db.Query(context.Background(), q)
		if err == nil {
			r.Close()
			t.Fatal("binding/syntax error hidden", q)
		}
	}
}

func TestSecondaryRangeBudgetsAndErrorOrder(t *testing.T) {
	db, _ := writable(t, sqlite.Options{PageSize: 512, MaxSortBytes: 16 << 10})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER)")
	execute(t, db, "CREATE INDEX by_k ON t(k)")
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := range 600 {
		txExecute(t, tx, "INSERT INTO t VALUES(?,?)", i, i)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	p, s, ss := ordinaryCompare(t, db, "SELECT id FROM t WHERE k>=0")
	if p.Access != "SCAN" || !strings.Contains(p.Reason, "budget") || s.IndexPageReads == 0 || s.BaseRowFetches != ss.BaseRowFetches {
		t.Fatal("partial range fallback", p, s, ss)
	}
	for _, where := range []string{"k>700 AND abs(-9223372036854775808)>0", "abs(-9223372036854775808)>0 AND k>700", "k>0 AND abs(-9223372036854775808)>0", "k>700 AND id LIKE X'FF'"} {
		var errs []string
		for _, from := range []string{"t", "t NOT INDEXED"} {
			r, err := db.Query(context.Background(), "SELECT id FROM "+from+" WHERE "+where)
			if err == nil {
				if r.Plan().Method == "btree_range" {
					t.Fatal("unsafe residual admitted", where)
				}
				for r.Next() {
				}
				err = r.Err()
				r.Close()
			}
			if err == nil {
				errs = append(errs, "")
			} else {
				errs = append(errs, err.Error())
			}
		}
		if errs[0] != errs[1] {
			t.Fatal(where, errs)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = db.Query(ctx, "SELECT id FROM t WHERE k>599"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestSecondaryRangeTransactionsSnapshotsCancelAndReopen(t *testing.T) {
	ctx := context.Background()
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "range.db")
			opts := sqlite.Options{PageSize: 512, WALShared: shared, KeepWALOnClose: shared}
			db, err := sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { db.Close() }()
			execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER,v TEXT)")
			execute(t, db, "CREATE INDEX by_k ON t(k)")
			execute(t, db, "INSERT INTO t VALUES(1,1,'one'),(2,2,'two'),(3,3,'three'),(4,4,'four')")
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			txExecute(t, tx, "UPDATE t SET k=8 WHERE id=2")
			got, p, _ := gramResults(t, tx, "SELECT id FROM t WHERE k>3")
			if !reflect.DeepEqual(got, [][]any{{int64(2)}, {int64(4)}}) || p.Method != "btree_range" {
				t.Fatal(got, p)
			}
			active, err := tx.Query(ctx, "SELECT id FROM t WHERE k>1")
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); !errors.Is(err, sqlite.ErrBusy) {
				t.Fatal(err)
			}
			active.Close()
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			ordinaryCompare(t, db, "SELECT id,k,v FROM t WHERE k>1 ORDER BY k,id")
			if shared {
				peer, err := sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				r, err := peer.Query(ctx, "SELECT id,k,v FROM t WHERE k>1")
				if err != nil {
					t.Fatal(err)
				}
				if !r.Next() || r.Values()[0] != int64(2) {
					t.Fatal(r.Values(), r.Err())
				}
				execute(t, db, "UPDATE t SET k=0,v='changed' WHERE id=3")
				var rest [][]any
				for r.Next() {
					rest = append(rest, r.Values())
				}
				if r.Err() != nil || !reflect.DeepEqual(rest, [][]any{{int64(3), int64(3), "three"}, {int64(4), int64(4), "four"}}) {
					t.Fatal(rest, r.Err())
				}
				r.Close()
				peer.Close()
			}
			cancelCtx, cancel := context.WithCancel(ctx)
			r, err := db.Query(cancelCtx, "SELECT id FROM t WHERE k>0")
			if err != nil {
				t.Fatal(err)
			}
			if !r.Next() {
				t.Fatal(r.Err())
			}
			cancel()
			if r.Next() || !errors.Is(r.Err(), context.Canceled) {
				t.Fatal(r.Err())
			}
			r.Close()
			if shared {
				if err = db.Checkpoint(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			ordinaryCompare(t, db, "SELECT id,k,v FROM t WHERE k>1 ORDER BY k,id")
		})
	}
}

func TestSecondaryRangeExtensionAndJoinPrecedence(t *testing.T) {
	db, _ := annQueryFixture(t, 80, false)
	execute(t, db, "CREATE INDEX by_active ON items(active)")
	execute(t, db, "CREATE INDEX by_title ON items(title)")
	execute(t, db, "CREATE INDEX title_grams ON items(title) USING grams13")
	_, p, s := gramResults(t, db, "SELECT id FROM items WHERE active>0 ORDER BY embedding <-> '[19,5,1]' LIMIT 2")
	if p.Method != "hnsw" || s.IndexKeysRead != 0 || s.IndexPageReads != 0 || s.RowIDSeeks != 0 {
		t.Fatal("ANN precedence", p, s)
	}
	_, p, s = gramResults(t, db, "SELECT id FROM items WHERE title>'item 1' AND title LIKE '%item 19%'")
	if p.Method != "grams13" || s.IndexKeysRead != 0 {
		t.Fatal("grams precedence", p, s)
	}
	ivf := ivfQueryFixture(t, false)
	execute(t, ivf, "CREATE INDEX by_active ON items(active)")
	_, p, s = gramResults(t, ivf, "SELECT id FROM items WHERE active>0 ORDER BY embedding <-> '[3000,1500,1]' LIMIT 2")
	if p.Method != "ivfflat" || s.IndexKeysRead != 0 || s.IndexPageReads != 0 || s.RowIDSeeks != 0 {
		t.Fatal("IVF precedence", p, s)
	}
	plain, _ := writable(t, sqlite.Options{})
	execute(t, plain, "CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER)")
	execute(t, plain, "CREATE INDEX by_k ON t(k)")
	execute(t, plain, "INSERT INTO t VALUES(1,1),(2,2),(3,3)")
	for _, join := range []string{"JOIN", "LEFT JOIN"} {
		r, err := plain.Query(context.Background(), "SELECT a.id,b.id FROM t a "+join+" t b ON b.k>a.k WHERE a.k>1 ORDER BY a.id,b.id")
		if err != nil {
			t.Fatal(err)
		}
		if r.Plan().Method == "btree_range" {
			t.Fatal("range entered relational planner", r.Plan())
		}
		for r.Next() {
		}
		if r.Err() != nil {
			t.Fatal(r.Err())
		}
		r.Close()
	}
}

func TestSecondaryRangeConcurrentQueries(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k BLOB)")
	execute(t, db, "CREATE INDEX by_k ON t(k)")
	execute(t, db, "INSERT INTO t VALUES(-1,1),(0,2),(1,2),(2,'3'),(3,X'34')")
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 10 {
				r, err := db.Query(context.Background(), "SELECT id FROM t WHERE k>=? AND k<? ORDER BY k,id DESC", int64(1), int64(3))
				if err != nil {
					t.Error(err)
					return
				}
				var ids []int64
				for r.Next() {
					ids = append(ids, r.Values()[0].(int64))
				}
				if err = r.Err(); err != nil {
					t.Error(err)
				}
				if !reflect.DeepEqual(ids, []int64{-1, 1, 0}) || r.Plan().Method != "btree_range" {
					t.Error(ids, r.Plan())
				}
				r.Close()
			}
		})
	}
	wg.Wait()
}
