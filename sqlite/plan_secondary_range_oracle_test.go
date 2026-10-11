//go:build oracle

package sqlite_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/testoracle"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// This bounded regression keeps the original 1800-row healthy fixture and the
// exact failing SQL. There are nine normal CLI invocations: one identity probe,
// six result queries, and two plans. No process is timed out, killed, or retried.
func TestSecondaryRangeIPKPrecedencePinnedOracle(t *testing.T) {
	executable := os.Getenv("SQLITE_ORACLE")
	if executable == "" {
		t.Fatal("SQLITE_ORACLE must name the pinned SQLite 3.54.0 CLI")
	}
	completed := 0
	run := func(t *testing.T, input string, arguments ...string) []byte {
		t.Helper()
		cmd := testoracle.ForTest(t, executable, arguments...)
		cmd.Stdin = strings.NewReader(input)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		completed++
		t.Logf("official expected factory executable=%q arguments_after_noinit=%q contract=%q", executable, arguments, "prepend -noinit; capture package CWD; child allowlist LC_ALL=C LANG=C TZ=UTC SQLITE_TMPDIR=TempRoot TMPDIR=TempRoot; TempRoot selects GOTMPDIR, then TMPDIR, then /tmp")
		t.Logf("official observed stdin=%q exit=%v stdout=%q stderr=%q", input, err, stdout.String(), stderr.String())
		if err != nil || stderr.Len() != 0 {
			t.Fatalf("official normal completion: exit=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
		}
		return stdout.Bytes()
	}
	identity := run(t, "", "-batch", "-bail", ":memory:", "SELECT sqlite_version() || '|' || sqlite_source_id();")
	const pin = "3.54.0|2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3"
	if strings.TrimSpace(string(identity)) != pin {
		t.Fatalf("official identity=%q", identity)
	}

	db, _ := writable(t, sqlite.Options{PageSize: 512})
	var seed strings.Builder
	seed.WriteString("PRAGMA page_size=512;\n")
	for _, query := range []string{
		"CREATE TABLE t(id INTEGER PRIMARY KEY,k INTEGER,suffix INTEGER,tie INTEGER,rowid TEXT)",
		"CREATE INDEX by_k ON t(k,suffix)",
		"CREATE INDEX by_shadow ON t(rowid)",
	} {
		execute(t, db, query)
		seed.WriteString(query + ";\n")
	}
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	seed.WriteString("BEGIN;\n")
	for i := range 1800 {
		txExecute(t, tx, "INSERT INTO t VALUES(?,?,?,0,'shadow')", i-900, i%90, 1800-i)
		fmt.Fprintf(&seed, "INSERT INTO t VALUES(%d,%d,%d,0,'shadow');\n", i-900, i%90, 1800-i)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	seed.WriteString("COMMIT;\n")
	cases := []struct {
		name, query, method string
		want                [][]any
		plan                bool
	}{
		{"original_reject", "SELECT id FROM t WHERE k>88 AND id=1", "rowid", nil, false},
		{"not_indexed_reject", "SELECT id FROM t NOT INDEXED WHERE k>88 AND id=1", "rowid", nil, false},
		{"full_scan_reject", "SELECT id FROM t NOT INDEXED WHERE k>88 AND id+0=1", "", nil, false},
		{"original_match", "SELECT id FROM t WHERE k>88 AND id=899", "rowid", [][]any{{int64(899)}}, false},
		{"not_indexed_match", "SELECT id FROM t NOT INDEXED WHERE k>88 AND id=899", "rowid", [][]any{{int64(899)}}, false},
		{"full_scan_match", "SELECT id FROM t NOT INDEXED WHERE k>88 AND id+0=899", "", [][]any{{int64(899)}}, false},
		{"not_indexed_plan", "EXPLAIN QUERY PLAN SELECT id FROM t NOT INDEXED WHERE k>88 AND id=1", "rowid", nil, true},
		{"full_scan_plan", "EXPLAIN QUERY PLAN SELECT id FROM t NOT INDEXED WHERE k>88 AND id+0=1", "", nil, true},
	}
	if len(cases) != 8 {
		t.Fatal("finite query inventory changed", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output := run(t, seed.String()+tc.query+";\n", "-batch", "-bail", "-json", "-cmd", ".explain off", ":memory:")
			var records []map[string]any
			if len(output) != 0 {
				decoder := json.NewDecoder(bytes.NewReader(output))
				decoder.UseNumber()
				if err := decoder.Decode(&records); err != nil {
					t.Fatal(err)
				}
				var extra any
				if err := decoder.Decode(&extra); err != io.EOF {
					t.Fatal("trailing official output", extra, err)
				}
			}
			var official [][]any
			if tc.plan {
				if len(records) != 1 || len(records[0]) != 4 {
					t.Fatal("official plan shape", records)
				}
				for _, key := range []string{"id", "parent", "notused"} {
					n, ok := records[0][key].(json.Number)
					if !ok {
						t.Fatal("official plan metadata", key, records)
					}
					if _, err := n.Int64(); err != nil {
						t.Fatal(err)
					}
				}
				detail, ok := records[0]["detail"].(string)
				if !ok || tc.method == "rowid" && !strings.HasPrefix(detail, "SEARCH t USING INTEGER PRIMARY KEY") || tc.method == "" && detail != "SCAN t" {
					t.Fatal("official access path", detail)
				}
			} else {
				for _, record := range records {
					if len(record) != 1 {
						t.Fatal("official result shape", record)
					}
					n, ok := record["id"].(json.Number)
					if !ok {
						t.Fatal("official result type", record)
					}
					id, err := n.Int64()
					if err != nil {
						t.Fatal(err)
					}
					official = append(official, []any{id})
				}
				if !reflect.DeepEqual(official, tc.want) {
					t.Fatal("official typed result", official, tc.want)
				}
			}
			got, plan, stats := gramResults(t, db, tc.query)
			access := "SCAN"
			if tc.method == "rowid" {
				access = "SEARCH"
			}
			if plan.Access != access || plan.Method != tc.method || len(plan.Indexes) != 0 || tc.method == "rowid" && (!plan.Recheck || !reflect.DeepEqual(plan.Fields, []string{"rowid"})) {
				t.Fatal("native access path", plan)
			}
			seeks, fetches := int64(1), int64(1)
			if tc.method == "" {
				seeks, fetches = 0, 1800
			}
			if tc.plan {
				seeks, fetches = 0, 0
				if len(got) != 1 || len(got[0]) != 4 || got[0][3] != plan.Detail() {
					t.Fatal("native explain shape", got, plan)
				}
			} else if !reflect.DeepEqual(got, official) {
				t.Fatal("native/official typed result", got, official)
			}
			if stats.IndexKeysRead != 0 || stats.IndexPageReads != 0 || stats.RowIDSeeks != seeks || stats.BaseRowFetches != fetches || stats.ReturnedRows != int64(len(got)) {
				t.Fatal("native attempted work", stats, seeks, fetches)
			}
			t.Logf("native SQL=%q rows=%#v plan=%+v stats=%+v", tc.query, got, plan, stats)
		})
	}
	if completed != 9 {
		t.Fatal("finite official process count changed", completed)
	}
	t.Log("9 official normal process completions: one identity, six results, two plans")
}
