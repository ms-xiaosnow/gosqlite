//go:build oracle

package testkit_test

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Only pinned official SQLite 3.54.0 and ordinary SQL on intact files. Processes
// end normally. No crash, file mutation, physical-fault or recovery experiment.
func TestOracleSecondaryRangeSQL(t *testing.T) {
	confOracle(t)
	probes := []struct {
		v       any
		literal string
	}{
		{nil, "NULL"}, {int64(math.MinInt64), "-9223372036854775808"}, {int64(math.MinInt64 + 1), "-9223372036854775807"},
		{int64(-9007199254740993), "-9007199254740993"}, {int64(-1), "-1"}, {int64(0), "0"}, {int64(1), "1"}, {int64(42), "42"},
		{int64(9007199254740993), "9007199254740993"}, {int64(math.MaxInt64), "9223372036854775807"},
		{-9223372036854775808.0, "-9223372036854775808.0"}, {9223372036854775808.0, "9223372036854775808.0"},
		{-1.5, "-1.5"}, {1.5, "1.5"}, {42.0, "42.0"}, {9007199254740992.0, "9007199254740992.0"},
		{"", "''"}, {"01", "'01'"}, {"42.0", "'42.0'"}, {"4.2e1", "'4.2e1'"}, {"text", "'text'"}, {"数据库🙂", "'数据库🙂'"},
		{"x\x00y", "CAST(X'780079' AS TEXT)"}, {[]byte{}, "X''"}, {[]byte{0, 255}, "X'00FF'"}, {[]byte("42"), "X'3432'"},
	}
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "range.db")
			db := writeOpen(t, path, size)
			writeExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k,n NUMERIC,txt TEXT,r REAL,b BLOB,suffix INTEGER)")
			for _, col := range []string{"k", "n", "txt", "r", "b"} {
				writeExec(t, db, "CREATE INDEX by_"+col+" ON t("+col+" COLLATE BINARY ASC,suffix)")
			}
			tx, err := db.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			for i, p := range probes {
				for repeat := range 2 {
					writeTxExec(t, tx, "INSERT INTO t VALUES(?,?,?,?,?,?,?)", 2*i+repeat-30, p.v, p.v, p.v, p.v, p.v, 60-2*i-repeat)
				}
			}
			if err = tx.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
			checks := 0
			for _, col := range []string{"k", "n", "txt", "r", "b"} {
				for _, op := range []string{"<", "<=", ">", ">="} {
					for _, p := range probes {
						// Preserve complete NUL-containing text through hex and expose the
						// original SQL storage class separately; other values stay unmodified.
						selectSQL := "SELECT id,typeof(" + col + "),CASE WHEN typeof(" + col + ")='text' THEN hex(" + col + ") ELSE " + col + " END FROM t WHERE "
						for _, left := range []bool{false, true} {
							where := col + op + "?"
							oracleWhere := col + op + p.literal
							if left {
								where = "?" + op + col
								oracleWhere = p.literal + op + col
							}
							tail := " ORDER BY " + col + " DESC NULLS LAST,id ASC"
							q := selectSQL + where + tail
							t.Logf("case %d: page=%d %s %#v", checks, size, q, p.v)
							want := confExpected(t, path, selectSQL+oracleWhere+tail)
							confCompare(t, want, confNative(t, db, q, p.v))
							confCompare(t, want, confNative(t, db, strings.Replace(q, "FROM t ", "FROM t NOT INDEXED ", 1), p.v))
							r, err := db.Query(context.Background(), q, p.v)
							if err != nil {
								t.Fatal(err)
							}
							if r.Plan().Method != "btree_range" {
								t.Fatal(q, r.Plan())
							}
							r.Close()
							checks++
						}
					}
				}
			}
			for _, where := range []string{"k>=1 AND k<=42", "k<=42 AND k>=1", "k>42 AND k<1", "k<1 AND k>42", "k>1 AND k<=1", "k>=1 AND k<=1", "k>NULL AND k<42", "k<42 AND k>NULL", "b>1 AND b<='42.0'", "txt>1 AND txt<'text'"} {
				q := "SELECT id,typeof(k),hex(k) FROM t WHERE " + where + " ORDER BY k,id DESC LIMIT 9 OFFSET 1"
				want := confExpected(t, path, q)
				confCompare(t, want, confNative(t, db, q))
				confCompare(t, want, confNative(t, db, strings.Replace(q, "FROM t ", "FROM t NOT INDEXED ", 1)))
				checks++
			}
			for _, p := range []struct {
				v    any
				bind string
			}{{int64(1), "1"}, {1.5, "1.5"}, {"01", "\"'01'\""}, {[]byte("42"), "X'3432'"}, {nil, "NULL"}, {"x\x00y", "\"CAST(X'780079' AS TEXT)\""}} {
				q := "SELECT id,typeof(k),hex(k) FROM t WHERE k>?1 ORDER BY k,id DESC LIMIT 9"
				cmd := exec.Command(confOracle(t), "-batch", "-bail", "-quote", "-header", "-readonly", path)
				cmd.Stdin = strings.NewReader(".parameter init\n.parameter set ?1 " + p.bind + "\n" + q + ";\n")
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatal(err, string(out))
				}
				parsed, err := confQuoted(out)
				if err != nil {
					t.Fatal(err)
				}
				want := confResult{}
				if len(parsed) > 0 {
					want.rows = parsed[1:]
					for _, v := range parsed[0] {
						want.columns = append(want.columns, v.(string))
					}
				}
				confCompare(t, want, confNative(t, db, q, p.v))
				checks++
			}
			for _, q := range []string{"SELECT abs(-9223372036854775808) FROM t WHERE k>0 ORDER BY k,id", "SELECT id FROM t WHERE k>0 AND abs(-9223372036854775808)>0", "SELECT missing FROM t WHERE k>0 LIMIT 0", "SELECT id FROM t WHERE k>0 ORDER BY missing LIMIT 0", "SELECT id FROM t WHERE k >", "SELECT id FROM t WHERE k >< 1"} {
				cmd := exec.Command(confOracle(t), "-batch", "-bail", "-readonly", path, q)
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				_, oracleErr := cmd.Output()
				if oracleErr == nil {
					t.Fatal("expected oracle error", q)
				}
				r, nativeErr := db.Query(context.Background(), q)
				if nativeErr == nil {
					for r.Next() {
					}
					nativeErr = r.Err()
					r.Close()
				}
				if nativeErr == nil {
					t.Fatal("missed oracle error", q, stderr.String())
				}
				for _, kind := range []string{"integer overflow", "no such column"} {
					if strings.Contains(stderr.String(), kind) && !strings.Contains(nativeErr.Error(), kind) {
						t.Fatal(q, nativeErr, stderr.String())
					}
				}
				checks++
			}
			writeIntegrity(t, path)
			t.Logf("%d exact values/types/order/bound-side/parameter/error official comparisons at page size %d", checks, size)
		})
	}
}

func TestOracleSecondaryRangeUnsupportedPeersAndLifecycle(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "range.db")
			opts := sqlite.Options{PageSize: 512, WALShared: shared, KeepWALOnClose: shared}
			db, err := sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if db != nil {
					db.Close()
				}
			}()
			writeExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT,suffix INTEGER)")
			writeExec(t, db, "CREATE INDEX by_k ON t(k,suffix)")
			needle := "head\x00" + strings.Repeat("界", 3000)
			for _, row := range []struct {
				id int64
				k  string
			}{{math.MinInt64, "a"}, {-2, needle}, {-1, needle + "a"}, {0, "z"}, {1, needle + "b"}, {math.MaxInt64, needle}} {
				writeExec(t, db, "INSERT INTO t VALUES(?,?,0)", row.id, row.k)
			}
			q := "SELECT id,hex(k),typeof(k) FROM t WHERE k>=? AND k<? ORDER BY k DESC,id ASC"
			want := confExpected(t, path, fmt.Sprintf("SELECT id,hex(k),typeof(k) FROM t WHERE k>=CAST(X'%x' AS TEXT) AND k<CAST(X'%x' AS TEXT) ORDER BY k DESC,id ASC", []byte(needle), []byte(needle+"z")))
			confCompare(t, want, confNative(t, db, q, needle, needle+"z"))
			confCompare(t, want, confNative(t, db, strings.Replace(q, "FROM t ", "FROM t NOT INDEXED ", 1), needle, needle+"z"))
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			writeTxExec(t, tx, "UPDATE t SET k='outside' WHERE id=-2")
			r, err := tx.Query(ctx, "SELECT id FROM t WHERE k>?", needle)
			if err != nil {
				t.Fatal(err)
			}
			if r.Plan().Method != "btree_range" {
				t.Fatal(r.Plan())
			}
			for r.Next() {
			}
			if r.Err() != nil {
				t.Fatal(r.Err())
			}
			r.Close()
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			confCompare(t, want, confNative(t, db, q, needle, needle+"z"))
			if shared {
				if err = db.Checkpoint(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			db = nil
			// Official ordinary SQL creates unsupported peer definitions. Read-only
			// catalog admission must skip each peer while retaining the usable index.
			// Native read-only WAL is outside the existing API. Switch modes by
			// ordinary official SQL after all shared owners have closed.
			confCLI(t, path, "PRAGMA journal_mode=DELETE;CREATE INDEX peer_desc ON t(k DESC);CREATE INDEX peer_expr ON t(lower(k));CREATE INDEX peer_nocase ON t(k COLLATE NOCASE);CREATE INDEX peer_suffix_desc ON t(k,suffix DESC);UPDATE t SET suffix=3 WHERE id=1;", false)
			db, err = sqlite.Open(path, sqlite.Options{ReadOnly: true})
			if err != nil {
				t.Fatal(err)
			}
			r, err = db.Query(ctx, q, needle, needle+"z")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(r.Plan().Indexes, []string{"by_k"}) || r.Plan().Method != "btree_range" {
				t.Fatal(r.Plan())
			}
			r.Close()
			confCompare(t, want, confNative(t, db, q, needle, needle+"z"))
			writeIntegrity(t, path)
		})
	}
}
