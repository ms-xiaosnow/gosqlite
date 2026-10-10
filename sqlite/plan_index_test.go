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

func ordinaryCompare(t *testing.T, db gramQuerier, sql string, args ...any) (sqlite.QueryPlan, sqlite.QueryStats, sqlite.QueryStats) {
	t.Helper()
	got, p, s := gramResults(t, db, sql, args...)
	forced := strings.Replace(sql, "FROM t", "FROM t NOT INDEXED", 1)
	want, sp, ss := gramResults(t, db, forced, args...)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ordinary/scan mismatch %s %#v\nindexed=%#v\nscan=%#v\nplan=%+v", sql, args, got, want, p)
	}
	if sp.Access != "SCAN" || sp.Reason != "NOT INDEXED requested" || ss.IndexKeysRead != 0 || ss.RowIDSeeks != 0 {
		t.Fatalf("NOT INDEXED was not a genuine scan: %+v %+v", sp, ss)
	}
	return p, s, ss
}

func TestOrdinaryIndexStorageClassesAndAffinities(t *testing.T) {
	db, _ := writable(t, sqlite.Options{PageSize: 512})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k,n NUMERIC,txt TEXT,r REAL,b BLOB)")
	for _, sql := range []string{"CREATE INDEX by_k ON t(k)", "CREATE INDEX by_n ON t(n)", "CREATE INDEX by_txt ON t(txt COLLATE BINARY ASC)", "CREATE INDEX by_r ON t(r)", "CREATE INDEX by_b ON t(b)", "CREATE INDEX by_composite ON t(k,n,txt)"} {
		execute(t, db, sql)
	}
	values := []any{nil, int64(0), int64(1), int64(-1), int64(42), int64(9007199254740993), int64(math.MaxInt64), int64(math.MinInt64), 1.5, 42.0, math.Inf(1), math.Inf(-1), "", "01", "42", "42.0", "4.2e1", "text", "数据库🙂", "x\x00y", []byte{}, []byte{0, 255}, []byte("42"), strings.Repeat("overflow界", 300)}
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range values {
		for repeat := range 2 {
			txExecute(t, tx, "INSERT INTO t VALUES(?,?,?,?,?,?)", int64(2*i+repeat-30), v, v, v, v, v)
		}
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	checks := 0
	for _, col := range []string{"k", "n", "txt", "r", "b"} {
		for _, op := range []string{"=", "IS"} {
			for _, v := range values {
				q := "SELECT id,typeof(" + col + ")," + col + " FROM t WHERE " + col + " " + op + " ? ORDER BY id DESC"
				p, s, ss := ordinaryCompare(t, db, q, v)
				if p.Access != "SEARCH" || p.Method != "btree" || !p.Recheck || s.BaseRowFetches > ss.BaseRowFetches || s.RowIDSeeks != s.BaseRowFetches || s.IndexKeysRead != s.Candidates {
					t.Fatalf("%s %#v: %+v %+v scan=%+v", q, v, p, s, ss)
				}
				checks++
			}
		}
	}
	for _, q := range []string{"SELECT id FROM t WHERE k=? AND n=? ORDER BY id", "SELECT id FROM t WHERE ?=k AND ? IS n ORDER BY id", "SELECT id FROM t WHERE k=? AND n=? AND txt='42' ORDER BY id", "SELECT k AS renamed,id FROM t WHERE renamed=? AND n=? ORDER BY id"} {
		p, _, _ := ordinaryCompare(t, db, q, int64(42), "42.0")
		if p.Method != "btree" || len(p.Fields) < 2 || !reflect.DeepEqual(p.Indexes, []string{"by_composite"}) {
			t.Fatal(q, p)
		}
	}
	t.Logf("%d storage-class/affinity equality and IS probes matched independently forced scans", checks)
}

func TestOrdinaryIndexRowIDAndShadowedNames(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT,rowid TEXT)")
	execute(t, db, "CREATE INDEX by_shadow ON t(rowid)")
	ids := []int64{math.MinInt64, -42, -1, 0, 1, 42, 9007199254740993, math.MaxInt64}
	for _, id := range ids {
		execute(t, db, "INSERT INTO t VALUES(?,?,'shadow')", id, fmt.Sprint(id))
	}
	probes := []any{int64(math.MinInt64), int64(math.MaxInt64), int64(9007199254740993), int64(42), float64(42), "00042", "4.2e1", "42.0", "-9223372036854775808", "9223372036854775807", float64(-9223372036854775808.0), 9223372036854775808.0, nil, []byte("42"), "bad", 1.5, math.Inf(1)}
	for _, col := range []string{"id", "_rowid_", "oid"} {
		for _, arg := range probes {
			for _, where := range []string{col + "=?", "?=" + col, col + " IS ?"} {
				p, s, _ := ordinaryCompare(t, db, "SELECT id,k FROM t WHERE "+where, arg)
				if p.Method != "rowid" || p.Access != "SEARCH" || s.BaseRowFetches > 1 || s.RowIDSeeks > 1 || s.IndexKeysRead != 0 {
					t.Fatal(where, arg, p, s)
				}
			}
		}
	}
	for _, q := range []string{"SELECT id FROM t WHERE id IS TRUE", "SELECT id FROM t WHERE id IS FALSE", "SELECT id FROM t WHERE id>0", "SELECT id FROM t WHERE id=1 OR id=42"} {
		p, _, _ := ordinaryCompare(t, db, q)
		if p.Access != "SCAN" {
			t.Fatal(q, p)
		}
	}
	p, s, _ := ordinaryCompare(t, db, "SELECT id FROM t WHERE rowid='shadow'")
	if p.Method != "btree" || s.BaseRowFetches != int64(len(ids)) {
		t.Fatal("physical rowid shadow", p, s)
	}
	p, _, _ = ordinaryCompare(t, db, "SELECT id AS rid FROM t WHERE rid=42")
	if p.Method != "rowid" {
		t.Fatal("simple WHERE alias was not bound to physical slot", p)
	}
	p, _, _ = ordinaryCompare(t, db, "SELECT 42 AS id FROM t WHERE id=1")
	if p.Method != "rowid" {
		t.Fatal("physical column should shadow result alias", p)
	}
}

func TestOrdinaryIndexPrefixWorkAndDeterministicLimit(t *testing.T) {
	db, _ := writable(t, sqlite.Options{PageSize: 512})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER,suffix INTEGER,tie INTEGER)")
	execute(t, db, "CREATE INDEX by_prefix ON t(k,suffix)")
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := range 1800 {
		txExecute(t, tx, "INSERT INTO t VALUES(?,?,?,0)", i-900, i%90, 1800-i)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, tail := range []string{"", " LIMIT 7 OFFSET 3", " ORDER BY tie LIMIT 7 OFFSET 3", " ORDER BY tie DESC LIMIT 7 OFFSET 3", " ORDER BY suffix,id LIMIT 7 OFFSET 3", " ORDER BY id DESC LIMIT 7 OFFSET 3"} {
		p, s, ss := ordinaryCompare(t, db, "SELECT id,suffix FROM t WHERE k=89"+tail)
		if p.Method != "btree" || s.IndexKeysRead != 20 || s.Candidates != 20 || s.BaseRowFetches > 20 || s.BaseRowFetches >= ss.BaseRowFetches || s.IndexPageReads >= 50 {
			t.Fatalf("%s: %+v %+v scan=%+v", tail, p, s, ss)
		}
		if !strings.Contains(p.Detail(), "covering=false; order_satisfied=false") {
			t.Fatal(p.Detail())
		}
		t.Logf("tail=%q matching keys=%d range page reads=%d source fetches=%d forced-scan fetches=%d", tail, s.IndexKeysRead, s.IndexPageReads, s.BaseRowFetches, ss.BaseRowFetches)
	}
	p, s, _ := ordinaryCompare(t, db, "SELECT id FROM t WHERE k=89 AND suffix>1000 ORDER BY id")
	if p.Method != "btree" || s.IndexKeysRead != 20 || s.ReturnedRows >= s.BaseRowFetches {
		t.Fatal("residual inequality must be rechecked", p, s)
	}
	r, err := db.Query(context.Background(), "EXPLAIN QUERY PLAN SELECT id FROM t WHERE k=89")
	if err != nil {
		t.Fatal(err)
	}
	if r.Plan().Method != "btree" || r.Stats().IndexKeysRead != 0 || r.Stats().IndexPageReads != 0 || r.Stats().RowIDSeeks != 0 || r.Stats().BaseRowFetches != 0 {
		t.Fatal(r.Plan(), r.Stats())
	}
	if !r.Next() || !strings.Contains(r.Values()[3].(string), "equality_prefix=k") {
		t.Fatal(r.Values(), r.Err())
	}
	r.Close()
}

func TestOrdinaryIndexBudgetsCancellationAndResidualErrors(t *testing.T) {
	db, _ := writable(t, sqlite.Options{PageSize: 512, MaxSortBytes: 16 << 10})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER)")
	execute(t, db, "CREATE INDEX by_k ON t(k)")
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := range 600 {
		txExecute(t, tx, "INSERT INTO t VALUES(?,1)", i)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	p, s, ss := ordinaryCompare(t, db, "SELECT id FROM t WHERE k=1")
	if p.Access != "SCAN" || !strings.Contains(p.Reason, "budget") || s.IndexKeysRead == 0 || s.IndexPageReads == 0 || s.RowIDSeeks != 0 || s.BaseRowFetches != ss.BaseRowFetches || s.ReturnedRows != 600 {
		t.Fatal("budget must retain attempted work and return a complete scan", p, s, ss)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r, err := db.Query(ctx, "SELECT id FROM t WHERE id=10")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if r.Next() || !errors.Is(r.Err(), context.Canceled) || r.Stats().RowIDSeeks != 0 {
		t.Fatal(r.Err(), r.Stats())
	}
	for _, where := range []string{"id=9999 AND abs(-9223372036854775808)>0", "id=9999 AND l2_distance('bad','[0]')>0", "id=9999 AND k LIKE 'x' ESCAPE 'ab'"} {
		var messages []string
		for _, modifier := range []string{"", " NOT INDEXED"} {
			r, err := db.Query(context.Background(), "SELECT id FROM t"+modifier+" WHERE "+where)
			if err == nil {
				if modifier == "" && r.Plan().Access != "SCAN" {
					t.Fatal("error-capable residual incorrectly filtered", where, r.Plan())
				}
				for r.Next() {
				}
				err = r.Err()
				r.Close()
			}
			if err == nil {
				t.Fatalf("error suppressed for %s", where)
			}
			messages = append(messages, err.Error())
		}
		if messages[0] != messages[1] {
			t.Fatal(where, messages)
		}
	}
}

func TestOrdinaryIndexTransactionsRollbackReopenAndSnapshot(t *testing.T) {
	ctx := context.Background()
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared%t", shared), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ordinary.db")
			opts := sqlite.Options{PageSize: 512, WALShared: shared, KeepWALOnClose: shared}
			db, err := sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { db.Close() }()
			execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT,suffix INTEGER)")
			execute(t, db, "CREATE INDEX by_k ON t(k,suffix)")
			execute(t, db, "INSERT INTO t VALUES(1,'old',3),(2,'old',2),(3,'old',1),(4,'other',0)")
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			txExecute(t, tx, "UPDATE t SET id=-10,k='new' WHERE id=1")
			txExecute(t, tx, "DELETE FROM t WHERE id=2")
			txExecute(t, tx, "INSERT INTO t VALUES(2,'new',7)")
			p, s, _ := ordinaryCompare(t, tx, "SELECT id,k,suffix FROM t WHERE k='new' ORDER BY id")
			if p.Method != "btree" || s.ReturnedRows != 2 {
				t.Fatal(p, s)
			}
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			_, s, _ = ordinaryCompare(t, db, "SELECT id FROM t WHERE k='old' ORDER BY id")
			if s.ReturnedRows != 3 {
				t.Fatal(s)
			}
			if shared {
				peer, err := sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				r, err := peer.Query(ctx, "SELECT id,k FROM t WHERE k='old'")
				if err != nil {
					t.Fatal(err)
				}
				if !r.Next() || r.Values()[0] != int64(1) {
					t.Fatal(r.Values(), r.Err())
				}
				execute(t, db, "UPDATE t SET k='new' WHERE id=2")
				var old [][]any
				for r.Next() {
					old = append(old, r.Values())
				}
				if r.Err() != nil || !reflect.DeepEqual(old, [][]any{{int64(2), "old"}, {int64(3), "old"}}) {
					t.Fatal("index and base rows must use the same snapshot", old, r.Err())
				}
				r.Close()
				peer.Close()
			} else {
				execute(t, db, "UPDATE t SET k='new' WHERE id=2")
			}
			execute(t, db, "UPDATE t SET id=-10,k='new' WHERE id=1")
			execute(t, db, "DELETE FROM t WHERE id=3")
			execute(t, db, "INSERT INTO t VALUES(3,'new',10)")
			if shared {
				if err := db.Checkpoint(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			p, s, _ = ordinaryCompare(t, db, "SELECT id,k,suffix FROM t WHERE k='new' ORDER BY id")
			if p.Method != "btree" || s.ReturnedRows != 3 || s.BaseRowFetches != 3 {
				t.Fatal(p, s)
			}
		})
	}
}

func TestOrdinaryIndexGramsANNPrecedenceAndExactVector(t *testing.T) {
	ctx := context.Background()
	db, _ := annQueryFixture(t, 80, false)
	execute(t, db, "CREATE INDEX by_active ON items(active)")
	execute(t, db, "CREATE INDEX by_title ON items(title)")
	execute(t, db, "CREATE INDEX title_grams ON items(title) USING grams13")
	_, p, s := gramResults(t, db, "SELECT id FROM items WHERE active=1 ORDER BY embedding <-> '[19,5,1]' LIMIT 2")
	if p.Method != "hnsw" || s.IndexKeysRead != 0 || s.IndexPageReads != 0 || s.RowIDSeeks != 0 {
		t.Fatal("ordinary filtering must not replace or precede eligible ANN", p, s)
	}
	_, p, s = gramResults(t, db, "SELECT id FROM items WHERE title='item 19' AND title LIKE '%item 19%'")
	if p.Method != "grams13" || s.IndexKeysRead != 0 {
		t.Fatal("grams precedence", p, s)
	}
	_, p, s = gramResults(t, db, "SELECT id FROM items WHERE active=1 ORDER BY embedding <+> '[19,5,1]' LIMIT 2")
	if p.Method != "btree" || !p.VectorExact || p.VectorRanking != "bounded_heap" || s.BaseRowFetches != 4 || s.VectorRankedRows != 4 {
		t.Fatal("exact vector ranking should consume ordinary candidates", p, s)
	}
	ivf := ivfQueryFixture(t, false)
	execute(t, ivf, "CREATE INDEX by_active ON items(active)")
	_, p, s = gramResults(t, ivf, "SELECT id FROM items WHERE active=1 ORDER BY embedding <-> '[3000,1500,1]' LIMIT 2")
	if p.Method != "ivfflat" || s.IndexKeysRead != 0 || s.IndexPageReads != 0 || s.RowIDSeeks != 0 {
		t.Fatal("ordinary filtering must not replace or precede eligible IVF", p, s)
	}
	for _, index := range []string{"items_hnsw", "title_grams"} {
		if err := db.CheckExtensionIndex(ctx, index); err != nil {
			t.Fatal(err)
		}
	}
	if err := ivf.CheckExtensionIndex(ctx, "items_ivf"); err != nil {
		t.Fatal(err)
	}
}

func TestOrdinaryIndexConcurrentQueries(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER)")
	execute(t, db, "CREATE INDEX by_k ON t(k)")
	execute(t, db, "INSERT INTO t VALUES(1,10),(2,20),(3,10)")
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 10 {
				r, err := db.Query(context.Background(), "SELECT id FROM t WHERE k=10 ORDER BY id")
				if err != nil {
					t.Error(err)
					return
				}
				var got []int64
				for r.Next() {
					got = append(got, r.Values()[0].(int64))
				}
				if r.Err() != nil || !reflect.DeepEqual(got, []int64{1, 3}) || r.Plan().Method != "btree" || r.Stats().RowIDSeeks != 2 {
					t.Error(got, r.Err(), r.Plan(), r.Stats())
				}
				r.Close()
			}
		})
	}
	wg.Wait()
}

func TestOrdinaryIndexSlotBindingAndUnsupportedShapes(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER,n INTEGER,\"TRUE\" INTEGER)")
	execute(t, db, "CREATE INDEX by_prefix ON t(k,n)")
	execute(t, db, "CREATE INDEX by_n ON t(n)")
	execute(t, db, "INSERT INTO t VALUES(1,10,20,1),(2,20,10,2),(3,10,10,0)")
	for _, tc := range []struct {
		sql, forced string
		field       string
	}{
		{"SELECT x.id FROM t AS x WHERE x.k=10 ORDER BY x.id", "SELECT x.id FROM t AS x NOT INDEXED WHERE x.k=10 ORDER BY x.id", "k"},
		{"SELECT k AS n,id FROM t WHERE n=10 ORDER BY id", "SELECT k AS n,id FROM t NOT INDEXED WHERE n=10 ORDER BY id", "n"},
		{"SELECT id FROM t WHERE k=10 AND TRUE=1 ORDER BY id", "SELECT id FROM t NOT INDEXED WHERE k=10 AND TRUE=1 ORDER BY id", "k"},
		{"SELECT id FROM t WHERE (k=10 OR k=20) AND n=10 ORDER BY id", "SELECT id FROM t NOT INDEXED WHERE (k=10 OR k=20) AND n=10 ORDER BY id", "n"},
	} {
		got, p, _ := gramResults(t, db, tc.sql)
		want, _, _ := gramResults(t, db, tc.forced)
		if !reflect.DeepEqual(got, want) || p.Method != "btree" || len(p.Fields) == 0 || p.Fields[0] != tc.field {
			t.Fatal(tc.sql, got, want, p)
		}
	}
	for _, sql := range []string{"SELECT id FROM t WHERE +k=10", "SELECT id FROM t WHERE k+0=10", "SELECT id FROM t WHERE k=n", "SELECT id FROM t WHERE k>10", "SELECT id FROM t WHERE k=10 OR k=20", "SELECT id FROM t WHERE k=abs(10)", "SELECT id+0 AS renamed FROM t WHERE renamed=1"} {
		p, _, _ := ordinaryCompare(t, db, sql)
		if p.Access != "SCAN" {
			t.Fatal("unsupported extraction must keep a genuine scan", sql, p)
		}
	}
}
