//go:build oracle

package testkit_test

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Ordinary native/official SQL only. No physical fault or recovery workflow.
func TestOracleOrdinaryIndexSelect(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("page%d/shared%t", size, shared), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "ordinary.db")
				opts := sqlite.Options{PageSize: size, WALShared: shared, KeepWALOnClose: shared}
				db, err := sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if db != nil {
						if err := db.Close(); err != nil {
							t.Error(err)
						}
					}
				}()
				writeExec(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,bucket INTEGER,title TEXT,payload BLOB,weight REAL)")
				writeExec(t, db, "CREATE INDEX items_bucket_title ON items(bucket,title)")
				tx, err := db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				for i := 1; i <= 320; i++ {
					var bucket any = int64(i % 11)
					if i%31 == 0 {
						bucket = nil
					}
					writeTxExec(t, tx, "INSERT INTO items VALUES(?,?,?,?,?)", i, bucket, fmt.Sprintf("组-%d", i%7), []byte{byte(i), 0, 255}, float64(i)/8)
				}
				if err = tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				collect := func(query string) ([][]any, sqlite.QueryPlan, sqlite.QueryStats) {
					t.Helper()
					rows, e := db.Query(ctx, query)
					if e != nil {
						t.Fatal(e)
					}
					var values [][]any
					for rows.Next() {
						values = append(values, rows.Values())
					}
					p, s := rows.Plan(), rows.Stats()
					if e = rows.Err(); e != nil {
						t.Fatal(e)
					}
					if e = rows.Close(); e != nil {
						t.Fatal(e)
					}
					return values, p, s
				}
				check := func() {
					t.Helper()
					query := "SELECT id,title,hex(payload),typeof(weight) FROM items WHERE bucket=3 AND title='组-3' ORDER BY id"
					got, p, s := collect(query)
					exact, ep, es := collect(strings.Replace(query, "FROM items ", "FROM items NOT INDEXED ", 1))
					if !reflect.DeepEqual(got, exact) {
						t.Fatalf("index/scan mismatch: %v / %v", got, exact)
					}
					if len(got) == 0 || p.Access != "SEARCH" || p.Method != "btree" || ep.Access != "SCAN" || s.IndexKeysRead == 0 || s.RowIDSeeks == 0 || s.BaseRowFetches >= es.BaseRowFetches {
						t.Fatalf("not a real reduced index path: plan=%+v stats=%+v exact=%+v/%+v", p, s, ep, es)
					}
					writeSame(t, db, path, query)
					for _, q := range []string{
						"SELECT id,title FROM items WHERE id=42",
						"SELECT id,title FROM items WHERE bucket='3' ORDER BY id",
						"SELECT id,title FROM items WHERE bucket IS NULL ORDER BY id",
						"SELECT id,title FROM items WHERE bucket=NULL ORDER BY id",
						"SELECT id,title FROM items WHERE title='组-3' ORDER BY id",
					} {
						writeSame(t, db, path, q)
					}
					_, rp, rs := collect("SELECT id,title FROM items WHERE id=42")
					if rp.Access != "SEARCH" || rp.Method != "rowid" || rs.RowIDSeeks != 1 || rs.BaseRowFetches != 1 {
						t.Fatal(rp, rs)
					}
					// Native deterministic rowid order is retained for an unordered LIMIT.
					q := "SELECT id,title FROM items WHERE bucket=3 LIMIT 5 OFFSET 2"
					a, _, _ := collect(q)
					b, _, _ := collect(strings.Replace(q, "FROM items ", "FROM items NOT INDEXED ", 1))
					if !reflect.DeepEqual(a, b) {
						t.Fatal(a, b)
					}
					writeIntegrity(t, path)
				}
				check()
				tx, err = db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				writeTxExec(t, tx, "UPDATE items SET bucket=3,title='组-3' WHERE id=1")
				writeTxExec(t, tx, "DELETE FROM items WHERE id=80")
				rows, err := tx.Query(ctx, "SELECT id FROM items WHERE bucket=3 AND title='组-3' ORDER BY id")
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for rows.Next() {
					if rows.Values()[0] == int64(1) {
						found = true
					}
				}
				if err = rows.Err(); err != nil {
					t.Fatal(err)
				}
				if err = rows.Close(); err != nil {
					t.Fatal(err)
				}
				if !found {
					t.Fatal("index did not see transaction's own update")
				}
				if err = tx.Rollback(); err != nil {
					t.Fatal(err)
				}
				check()
				if shared {
					if err = db.Checkpoint(ctx); err != nil {
						t.Fatal(err)
					}
				}
				if err = db.Close(); err != nil {
					t.Fatal(err)
				}
				db = nil
				confCLI(t, path, "BEGIN;UPDATE items SET bucket=3,title='组-3' WHERE id=1;INSERT INTO items VALUES(321,3,'组-3',X'00FF',1.5);DELETE FROM items WHERE id=80;COMMIT;", false)
				db, err = sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				check()
				writeExec(t, db, "UPDATE items SET bucket=4 WHERE id=321")
				check()
			})
		}
	}
}

func TestOracleOrdinaryIndexIgnoresUnsupportedPeer(t *testing.T) {
	confOracle(t)
	path := filepath.Join(t.TempDir(), "peer.db")
	confCLI(t, path, "CREATE TABLE items(id INTEGER PRIMARY KEY,bucket INTEGER,title TEXT);CREATE INDEX usable ON items(bucket);CREATE INDEX unsupported_expression ON items(lower(title));CREATE INDEX unsupported_desc ON items(title DESC);INSERT INTO items VALUES(1,3,'A'),(2,4,'B'),(3,3,'C');", false)
	db, err := sqlite.Open(path, sqlite.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	writeSame(t, db, path, "SELECT id,title FROM items WHERE bucket=3 ORDER BY id")
	rows, err := db.Query(context.Background(), "SELECT id FROM items WHERE bucket=3 ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
	}
	p := rows.Plan()
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	if p.Access != "SEARCH" || p.Method != "btree" {
		t.Fatal(p)
	}
}

// Mixed numeric comparison boundaries and overflow/NUL keys are ordinary values,
// independently compared with the pinned official engine on the same valid file.
func TestOracleOrdinaryIndexNumericAndOverflowKeys(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "boundaries.db")
	db := writeOpen(t, path, 512)
	writeExec(t, db, "CREATE TABLE values_table(id INTEGER PRIMARY KEY,n NUMERIC,txt TEXT)")
	writeExec(t, db, "CREATE INDEX values_n ON values_table(n)")
	writeExec(t, db, "CREATE INDEX values_txt ON values_table(txt)")
	numbers := []any{int64(math.MinInt64), float64(math.MinInt64), int64(math.MaxInt64), float64(math.MaxInt64), int64(9007199254740991), int64(9007199254740992), int64(9007199254740993), float64(9007199254740992), float64(9007199254740994), int64(0), math.SmallestNonzeroFloat64, math.MaxFloat64, "3", "abc", []byte{'3'}, nil}
	needle := "head\x00" + strings.Repeat("索引", 2500) + "tail"
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i, n := range numbers {
		value := fmt.Sprint(i)
		if i%5 == 0 {
			value = needle
		}
		writeTxExec(t, tx, "INSERT INTO values_table VALUES(?,?,?)", i+1, n, value)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		value   any
		literal string
	}{
		{int64(math.MinInt64), "-9223372036854775808"}, {int64(math.MaxInt64), "9223372036854775807"},
		{float64(math.MaxInt64), "9.223372036854776e18"}, {int64(9007199254740993), "9007199254740993"},
		{float64(9007199254740992), "9007199254740992.0"}, {"9007199254740993", "'9007199254740993'"},
		{"3", "'3'"}, {[]byte{'3'}, "X'33'"}, {"abc", "'abc'"}, {nil, "NULL"},
	}
	for _, check := range checks {
		for _, operator := range []string{"=", "IS"} {
			q := "SELECT id,typeof(n) FROM values_table WHERE n " + operator + " ? ORDER BY id"
			expected := confExpected(t, path, "SELECT id,typeof(n) FROM values_table WHERE n "+operator+" "+check.literal+" ORDER BY id")
			confCompare(t, expected, confNative(t, db, q, check.value))
			confCompare(t, expected, confNative(t, db, strings.Replace(q, "FROM values_table ", "FROM values_table NOT INDEXED ", 1), check.value))
		}
	}
	q := "SELECT id,hex(txt) FROM values_table WHERE txt=? ORDER BY id"
	expected := confExpected(t, path, fmt.Sprintf("SELECT id,hex(txt) FROM values_table WHERE txt=CAST(X'%x' AS TEXT) ORDER BY id", []byte(needle)))
	confCompare(t, expected, confNative(t, db, q, needle))
	rows, err := db.Query(ctx, q, needle)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
	}
	p, stats := rows.Plan(), rows.Stats()
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	if p.Method != "btree" || stats.IndexKeysRead != 4 || stats.BaseRowFetches != 4 {
		t.Fatal(p, stats)
	}
	writeIntegrity(t, path)
}
