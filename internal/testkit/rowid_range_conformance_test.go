//go:build oracle

package testkit_test

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/testoracle"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Pinned official CLI only, normal process completion, ordinary correct-key
// SQL on intact databases. No physical fault, file mutation or recovery path.
func TestOracleRowIDRangeSQL(t *testing.T) {
	confOracle(t)
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "rowid.db")
			db, err := sqlite.Open(path, sqlite.Options{PageSize: size})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			writeExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,rowid TEXT,v BLOB,r REAL)")
			for _, id := range []string{"-9223372036854775808", "-9007199254740993", "-42", "-2", "-1", "0", "1", "2", "42", "9007199254740993", "9223372036854775807"} {
				writeExec(t, db, "INSERT INTO t VALUES("+id+",'shadow',X'00FF',1.5)")
			}
			cases := 0
			for _, bound := range []string{"-9223372036854775808", "9223372036854775807", "-9223372036854775808.0", "9223372036854775808.0", "-1e100", "1e100", "-9007199254740993", "9007199254740993", "-1.25", "1.25", "-0.0", "0", "'00042'", "'4.2e1'", "'-9223372036854775808'", "'9223372036854775807'", "'9223372036854775808'", "'bad'", "NULL", "X''", "X'3432'"} {
				for _, op := range []string{"<", "<=", ">", ">="} {
					for _, direction := range []string{"ASC", "DESC"} {
						q := "SELECT id,v,r FROM t WHERE id" + op + bound + " ORDER BY id " + direction
						writeSame(t, db, path, q)
						cases++
						q = "SELECT id,v,r FROM t WHERE " + bound + op + "_rowid_ ORDER BY id " + direction + " LIMIT 3 OFFSET 1"
						writeSame(t, db, path, q)
						cases++
					}
				}
			}
			for _, q := range []string{
				"SELECT id,v,r FROM t WHERE id>=-42 AND id<42 ORDER BY id DESC",
				"SELECT id,v,r FROM t WHERE id>42 AND id<1 ORDER BY id DESC",
				"SELECT id,v,r FROM t WHERE id>=42 AND id<=42 ORDER BY id DESC",
				"SELECT id,v,r FROM t WHERE id>NULL AND id<1 ORDER BY id DESC",
				"SELECT id AS rid,v,r FROM t WHERE rid>=1 ORDER BY rid DESC NULLS FIRST LIMIT 3 OFFSET 1",
				"SELECT id,v,r FROM t ORDER BY 1 DESC NULLS LAST LIMIT 3",
				"SELECT rowid AS id,v,r FROM t WHERE _rowid_>1 ORDER BY id DESC",
				"SELECT id,v,r FROM t WHERE id>0 ORDER BY id+0 DESC LIMIT 3",
				"SELECT id,v,r FROM t WHERE id>1 AND id<42 LIMIT 0",
			} {
				writeSame(t, db, path, q)
				cases++
			}
			// Real bound parameters: preserve INTEGER/TEXT/BLOB/NULL rather than
			// using substitution to accidentally change expression affinity.
			for _, p := range []struct {
				v    any
				bind string
			}{{int64(1), "1"}, {1.25, "1.25"}, {"00042", "\"'00042'\""}, {[]byte("42"), "X'3432'"}, {nil, "NULL"}} {
				q := "SELECT id,v,r FROM t WHERE id>?1 ORDER BY id DESC LIMIT 3"
				cmd := testoracle.ForTest(t, confOracle(t), "-batch", "-bail", "-quote", "-header", "-readonly", path)
				cmd.Stdin = strings.NewReader(".parameter init\n.parameter set ?1 " + p.bind + "\n" + q + ";\n")
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatal(err, string(out))
				}
				parsed, err := confQuoted(out)
				if err != nil {
					t.Fatal(err)
				}
				want := confResult{rows: parsed[1:]}
				for _, v := range parsed[0] {
					want.columns = append(want.columns, v.(string))
				}
				confCompare(t, want, confNative(t, db, q, p.v))
				cases++
			}
			for _, q := range []string{"SELECT abs(id) FROM t WHERE id<=-9223372036854775808 ORDER BY id DESC LIMIT 1", "SELECT id FROM t WHERE id>0 AND abs(-9223372036854775808)>0 ORDER BY id", "SELECT id FROM t WHERE id>0 ORDER BY abs(-9223372036854775808)", "SELECT missing FROM t WHERE id>0 ORDER BY id LIMIT 0"} {
				cmd := testoracle.ForTest(t, confOracle(t), "-batch", "-bail", "-readonly", path, q)
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				_, oracleErr := cmd.Output()
				if oracleErr == nil {
					t.Fatal("expected oracle error", q)
				}
				rows, nativeErr := db.Query(context.Background(), q)
				if nativeErr == nil {
					for rows.Next() {
					}
					nativeErr = rows.Err()
					rows.Close()
				}
				if nativeErr == nil {
					t.Fatal("native missed oracle error", q, stderr.String())
				}
				for _, kind := range []string{"integer overflow", "no such column"} {
					if strings.Contains(stderr.String(), kind) && !strings.Contains(nativeErr.Error(), kind) {
						t.Fatal(q, nativeErr, stderr.String())
					}
				}
				cases++
			}
			writeIntegrity(t, path)
			t.Logf("%d exact values/types/order/error CLI comparisons", cases)
		})
	}
}
