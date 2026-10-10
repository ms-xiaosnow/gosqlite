package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestRowIDRangeStorageClassesDirectionsAndCuts(t *testing.T) {
	db, _ := writable(t, sqlite.Options{PageSize: 512})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY, rowid TEXT, v BLOB)")
	ids := []int64{math.MinInt64, -9007199254740993, -42, -2, -1, 0, 1, 2, 42, 9007199254740993, math.MaxInt64}
	for _, id := range ids {
		execute(t, db, "INSERT INTO t VALUES(?,'shadow',X'00FF')", id)
	}
	probes := []any{int64(math.MinInt64), int64(math.MaxInt64), int64(9007199254740993), int64(-9007199254740993), int64(1), float64(1), 1.25, -1.25, math.Nextafter(-9223372036854775808.0, math.Inf(-1)), math.Nextafter(9223372036854775808.0, 0), 9223372036854775808.0, math.Inf(-1), math.Inf(1), "00042", "4.2e1", "-9223372036854775808", "9223372036854775807", "9223372036854775808", nil, math.NaN(), "bad", []byte("42"), []byte{}}
	checks := 0
	for _, col := range []string{"id", "_rowid_", "oid"} {
		for _, op := range []string{"<", "<=", ">", ">="} {
			for _, v := range probes {
				for _, direction := range []string{"ASC", "DESC"} {
					q := "SELECT id,v FROM t WHERE " + col + op + "? ORDER BY id " + direction
					p, s, _ := ordinaryCompare(t, db, q, v)
					if p.Method != "rowid_range" || !strings.Contains(p.Detail(), "order_satisfied=true; sort=false") || s.IndexKeysRead != 0 || s.IndexPageReads != 0 || s.RowIDSeeks > 1 {
						t.Fatal(q, v, p, s)
					}
					q = "SELECT id,v FROM t WHERE ?" + op + col + " ORDER BY id " + direction
					p, _, _ = ordinaryCompare(t, db, q, v)
					if p.Method != "rowid_range" {
						t.Fatal(q, v, p)
					}
					checks += 2
				}
			}
		}
	}
	for _, where := range []string{"id>1 AND id<=42", "id>=1 AND id<42 AND id>2", "id>42 AND id<1", "id>=42 AND id<=42", "id>42 AND id<=42", "id>NULL AND id<1", "id>'text' AND id<1", "id<='text' AND id>=1", "id>=-9223372036854775808 AND id<=9223372036854775807"} {
		for _, tail := range []string{"", " LIMIT 2 OFFSET 1", " ORDER BY id DESC LIMIT 2 OFFSET 1", " ORDER BY v,id DESC"} {
			p, _, _ := ordinaryCompare(t, db, "SELECT id,v FROM t WHERE "+where+tail)
			if p.Method != "rowid_range" {
				t.Fatal(where, p)
			}
		}
	}
	for _, where := range []string{"id<'text'", "id<=X''", "id<1e100", "id>-1e100"} {
		p, _, _ := ordinaryCompare(t, db, "SELECT id FROM t WHERE "+where+" ORDER BY id DESC")
		if p.Access != "SCAN" || !strings.Contains(p.Reason, "bounded=false") {
			t.Fatal("unbounded constraint advertised as search", where, p)
		}
	}
	for _, where := range []string{"id>9223372036854775807", "id<-9223372036854775808", "id>NULL", "id>'text'"} {
		p, stats, _ := ordinaryCompare(t, db, "SELECT id FROM t WHERE "+where+" ORDER BY id DESC")
		if p.Access != "SEARCH" || p.EstimatedCandidates != 0 || stats.RowIDSeeks != 0 || stats.BaseRowFetches != 0 {
			t.Fatal("empty interval performed work", where, p, stats)
		}
	}
	t.Logf("%d native/forced-scan storage-class, side, direction cases", checks)
}

func TestRowIDRangeOrderProofLazyWorkAndBinding(t *testing.T) {
	db, _ := writable(t, sqlite.Options{PageSize: 512})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER,v TEXT)")
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := range 1800 {
		txExecute(t, tx, "INSERT INTO t VALUES(?,?,?)", i-900, i%2, fmt.Sprintf("value %d", i))
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"SELECT id,v FROM t WHERE id>=850 AND id<860 ORDER BY id DESC LIMIT 3 OFFSET 2",
		"SELECT id,v FROM t WHERE id>=850 AND id<860 ORDER BY 1 DESC NULLS FIRST LIMIT 3 OFFSET 2",
		"SELECT id AS rid,v FROM t WHERE id>=850 AND id<860 ORDER BY rid DESC LIMIT 3 OFFSET 2",
		"SELECT id AS rid,v FROM t WHERE rid>=850 AND rid<860 ORDER BY rid DESC LIMIT 3 OFFSET 2",
	} {
		r, err := db.Query(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		before := r.Stats()
		if before.BaseRowFetches != 0 || before.RowIDSeeks != 0 {
			t.Fatal("range eagerly executed", before)
		}
		frozen := r.Plan()
		frozen.Fields[0] = "mutated"
		if r.Plan().Fields[0] != "rowid" {
			t.Fatal("mutable plan")
		}
		r.Close()
		p, s, ss := ordinaryCompare(t, db, q)
		if s.BaseRowFetches != 5 || s.Candidates != 5 || s.RowIDSeeks != 1 || ss.BaseRowFetches != 1800 || !strings.Contains(p.Reason, "direction=reverse") {
			t.Fatal(q, p, s, ss)
		}
		t.Logf("%s: range=%d source rows; scan=%d source rows", q, s.BaseRowFetches, ss.BaseRowFetches)
	}
	for _, q := range []string{"SELECT id,v FROM t ORDER BY id", "SELECT id,v FROM t ORDER BY id DESC NULLS LAST"} {
		p, s, _ := ordinaryCompare(t, db, q+" LIMIT 4")
		if p.Method != "rowid_range" || p.Access != "SCAN" || s.BaseRowFetches != 4 {
			t.Fatal(q, p, s)
		}
	}
	for _, q := range []string{"SELECT k AS id FROM t WHERE _rowid_>850 ORDER BY id DESC", "SELECT id FROM t WHERE id>850 ORDER BY id+0 DESC", "SELECT id FROM t WHERE id>850 ORDER BY id,k", "SELECT abs(id) FROM t WHERE id>850 ORDER BY id DESC"} {
		p, _, _ := ordinaryCompare(t, db, q)
		if strings.Contains(p.Reason, "order_satisfied=true") {
			t.Fatal("unsafe or unproven ordering admitted", q, p)
		}
	}
	for _, q := range []string{"SELECT id FROM t WHERE id>? LIMIT 0", "SELECT id FROM t WHERE id>? ORDER BY id DESC LIMIT 0"} {
		if _, err := db.Query(context.Background(), q); err == nil {
			t.Fatal("missing bound accepted at LIMIT 0")
		}
		if _, err := db.Query(context.Background(), q, struct{}{}); err == nil {
			t.Fatal("invalid bound accepted at LIMIT 0")
		}
		r, err := db.Query(context.Background(), q, 1)
		if err != nil {
			t.Fatal(err)
		}
		if r.Next() || r.Err() != nil || r.Stats().BaseRowFetches != 0 || r.Stats().RowIDSeeks != 0 {
			t.Fatal(r.Err(), r.Stats())
		}
		r.Close()
	}
	r, err := db.Query(context.Background(), "EXPLAIN QUERY PLAN SELECT id FROM t WHERE id>850 ORDER BY id DESC")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if r.Plan().Method != "rowid_range" || r.Stats().RowIDSeeks != 0 || !r.Next() || !strings.Contains(r.Values()[3].(string), "direction=reverse") {
		t.Fatal(r.Plan(), r.Stats(), r.Values())
	}
}

func TestRowIDRangeErrorOrderAndConservativeJoin(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER)")
	execute(t, db, "INSERT INTO t VALUES(1,0),(2,-9223372036854775808),(3,3)")
	for _, q := range []string{
		"SELECT id FROM t WHERE abs(k)>0 AND id>9",
		"SELECT id FROM t WHERE id>9 AND abs(-9223372036854775808)>0",
		"SELECT abs(k) FROM t WHERE id>0 ORDER BY id LIMIT 1",
		"SELECT id FROM t WHERE id>0 ORDER BY abs(k) LIMIT 1",
	} {
		var messages []string
		for _, modifier := range []string{"", " NOT INDEXED"} {
			r, err := db.Query(context.Background(), strings.Replace(q, "FROM t", "FROM t"+modifier, 1))
			if err == nil {
				for r.Next() {
				}
				err = r.Err()
				r.Close()
			}
			if err == nil {
				t.Fatalf("suppressed error: %s", q)
			}
			messages = append(messages, err.Error())
		}
		if messages[0] != messages[1] {
			t.Fatal(q, messages)
		}
	}
	for _, q := range []string{"SELECT a.id,b.id FROM t a LEFT JOIN t b ON b.id>a.id WHERE a.id>1 ORDER BY a.id DESC,b.id", "SELECT a.id,b.id FROM t a JOIN t b ON b.id>a.id WHERE a.id>1 ORDER BY a.id DESC,b.id"} {
		r, err := db.Query(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		if r.Plan().Method == "rowid_range" {
			t.Fatal("range entered relational planner")
		}
		for r.Next() {
		}
		if r.Err() != nil {
			t.Fatal(r.Err())
		}
		r.Close()
	}
}

func TestRowIDRangeTransactionsSnapshotsCancelAndReopen(t *testing.T) {
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
			execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
			execute(t, db, "INSERT INTO t VALUES(1,'one'),(2,'two'),(3,'three'),(4,'four')")
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			txExecute(t, tx, "UPDATE t SET id=8 WHERE id=2")
			got, p, _ := gramResults(t, tx, "SELECT id FROM t WHERE id>1 ORDER BY id DESC")
			if !reflect.DeepEqual(got, [][]any{{int64(8)}, {int64(4)}, {int64(3)}}) || p.Method != "rowid_range" {
				t.Fatal(got, p)
			}
			active, err := tx.Query(ctx, "SELECT id FROM t WHERE id>1 ORDER BY id DESC")
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); !errors.Is(err, sqlite.ErrBusy) {
				t.Fatal("active range must retain lease", err)
			}
			active.Close()
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			ordinaryCompare(t, db, "SELECT id,v FROM t WHERE id>1 ORDER BY id DESC")
			if shared {
				peer, err := sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				r, err := peer.Query(ctx, "SELECT id,v FROM t WHERE id>1 ORDER BY id DESC")
				if err != nil {
					t.Fatal(err)
				}
				if !r.Next() || r.Values()[0] != int64(4) {
					t.Fatal(r.Values(), r.Err())
				}
				execute(t, db, "UPDATE t SET v='changed' WHERE id=3")
				var rest [][]any
				for r.Next() {
					rest = append(rest, r.Values())
				}
				if r.Err() != nil || !reflect.DeepEqual(rest, [][]any{{int64(3), "three"}, {int64(2), "two"}}) {
					t.Fatal(rest, r.Err())
				}
				r.Close()
				peer.Close()
			}
			cancelCtx, cancel := context.WithCancel(ctx)
			r, err := db.Query(cancelCtx, "SELECT id FROM t WHERE id>0 ORDER BY id DESC")
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
			ordinaryCompare(t, db, "SELECT id,v FROM t WHERE id>1 ORDER BY id DESC")
		})
	}
}
