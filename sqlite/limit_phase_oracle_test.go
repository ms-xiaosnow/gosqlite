//go:build oracle

package sqlite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/testoracle"
)

// This helper runs only fixed ordinary SQL in a fresh :memory: database. Every
// invocation disables external user initialization. There is no C observer,
// extension load, file database, process signal, randomized input, or campaign.
// Error families concern the CLI result, not C-API bind/prepare/step timing.
func limitPhaseOracle(t *testing.T, executable, query, family string) [][]any {
	t.Helper()
	cmd := testoracle.ForTest(t, executable, "-noinit", "-batch", "-bail", "-json", "-cmd", ".explain off", "-cmd", ".log stderr", ":memory:")
	cmd.Stdin = strings.NewReader(limitConversionSeed + "\n" + query + ";\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code, _, oraclePart := limitPhaseFamily(t, family)
	if code != CodeOK {
		var exit *exec.ExitError
		prefix := fmt.Sprintf("(%d) ", code)
		if !errors.As(err, &exit) || exit.ExitCode() != 1 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), prefix) || !strings.Contains(stderr.String(), oraclePart) {
			t.Fatalf("oracle %s: exit=%v stdout=%q stderr=%q; want code=%d family=%s", query, err, stdout.String(), stderr.String(), code, family)
		}
		return nil
	}
	if err != nil || stderr.Len() != 0 {
		t.Fatalf("oracle %s: exit=%v stdout=%q stderr=%q", query, err, stdout.String(), stderr.String())
	}
	if stdout.Len() == 0 {
		if family == "plan" {
			t.Fatal("official EXPLAIN returned no plan")
		}
		return nil
	}
	decoder := json.NewDecoder(&stdout)
	decoder.UseNumber()
	var records []map[string]any
	if err := decoder.Decode(&records); err != nil {
		t.Fatal(err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("unexpected trailing oracle output: value=%#v error=%v", extra, err)
	}
	if family == "plan" {
		if len(records) == 0 {
			t.Fatal("official EXPLAIN returned an empty plan")
		}
		for _, record := range records {
			if len(record) != 4 {
				t.Fatalf("unexpected plan fields: %#v", record)
			}
			for _, key := range []string{"id", "parent", "notused"} {
				n, ok := record[key].(json.Number)
				if !ok {
					t.Fatalf("non-numeric plan field %s: %#v", key, record)
				}
				if _, err := n.Int64(); err != nil {
					t.Fatal(err)
				}
			}
			if detail, ok := record["detail"].(string); !ok || detail == "" {
				t.Fatalf("missing official plan detail: %#v", record)
			}
		}
		return nil
	}
	var rows [][]any
	for _, record := range records {
		var row []any
		for _, key := range []string{"v", "n"} {
			if raw, ok := record[key]; ok {
				n, ok := raw.(json.Number)
				if !ok {
					t.Fatalf("non-integer result: %#v", record)
				}
				v, err := n.Int64()
				if err != nil {
					t.Fatal(err)
				}
				row = append(row, v)
			}
		}
		if len(row) != len(record) || len(row) == 0 {
			t.Fatalf("unexpected oracle fields: %#v", record)
		}
		rows = append(rows, row)
	}
	return rows
}

func limitPhaseFamily(t *testing.T, family string) (Code, string, string) {
	t.Helper()
	switch family {
	case "rows", "plan":
		return CodeOK, "", ""
	case "column":
		return CodeError, "no such column", "no such column"
	case "function":
		return CodeError, "function nosuch", "no such function: nosuch"
	case "arity":
		return CodeError, "wrong number of arguments", "wrong number of arguments"
	case "overflow":
		return CodeError, "integer overflow", "integer overflow"
	case "mismatch":
		return CodeMismatch, "LIMIT/OFFSET requires an integer", "datatype mismatch"
	default:
		t.Fatalf("unknown expected error family %q", family)
		return CodeError, "", ""
	}
}

func TestLimitPhasePinnedOracle(t *testing.T) {
	executable := os.Getenv("SQLITE_ORACLE")
	if executable == "" {
		t.Fatal("SQLITE_ORACLE must name the pinned SQLite 3.54.0 CLI")
	}
	version, err := testoracle.ForTest(t, executable, "-noinit", "-batch", "-bail", ":memory:", "SELECT sqlite_version() || '|' || sqlite_source_id();").CombinedOutput()
	const wantVersion = "3.54.0|2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3"
	if err != nil || strings.TrimSpace(string(version)) != wantVersion {
		t.Fatalf("oracle identity=%q err=%v", version, err)
	}
	// Forty-eight literal cases, not a Cartesian or randomized campaign.
	cases := []struct {
		name, query, family string
		want                [][]any
	}{
		{"constant_zero_null", "SELECT 1 AS v LIMIT 0 OFFSET NULL", "rows", nil},
		{"constant_zero_blob", "SELECT 1 AS v LIMIT 0 OFFSET X'31'", "rows", nil},
		{"constant_zero_fraction", "SELECT 1 AS v LIMIT 0 OFFSET 1.5", "rows", nil},
		{"constant_zero_text", "SELECT 1 AS v LIMIT 0 OFFSET 'bad'", "rows", nil},
		{"constant_zero_overflow", "SELECT 1 AS v LIMIT 0 OFFSET abs(-9223372036854775808)", "rows", nil},
		{"single_zero_null", "SELECT id AS v FROM t ORDER BY id LIMIT 0 OFFSET NULL", "rows", nil},
		{"single_zero_blob", "SELECT id AS v FROM t ORDER BY id LIMIT 0 OFFSET X'31'", "rows", nil},
		{"single_zero_fraction", "SELECT id AS v FROM t ORDER BY id LIMIT 0 OFFSET 1.5", "rows", nil},
		{"single_zero_text", "SELECT id AS v FROM t ORDER BY id LIMIT 0 OFFSET 'bad'", "rows", nil},
		{"single_zero_overflow", "SELECT id AS v FROM t ORDER BY id LIMIT 0 OFFSET abs(-9223372036854775808)", "rows", nil},
		{"join_zero_null", "SELECT t.id AS v FROM t LEFT JOIN t AS u ON t.id=u.id ORDER BY t.id LIMIT 0 OFFSET NULL", "rows", nil},
		{"join_zero_blob", "SELECT t.id AS v FROM t LEFT JOIN t AS u ON t.id=u.id ORDER BY t.id LIMIT 0 OFFSET X'31'", "rows", nil},
		{"join_zero_fraction", "SELECT t.id AS v FROM t LEFT JOIN t AS u ON t.id=u.id ORDER BY t.id LIMIT 0 OFFSET 1.5", "rows", nil},
		{"join_zero_text", "SELECT t.id AS v FROM t LEFT JOIN t AS u ON t.id=u.id ORDER BY t.id LIMIT 0 OFFSET 'bad'", "rows", nil},
		{"join_zero_overflow", "SELECT t.id AS v FROM t LEFT JOIN t AS u ON t.id=u.id ORDER BY t.id LIMIT 0 OFFSET abs(-9223372036854775808)", "rows", nil},
		{"aggregate_zero_null", "SELECT count(*) AS v FROM t LIMIT 0 OFFSET NULL", "rows", nil},
		{"aggregate_zero_blob", "SELECT count(*) AS v FROM t LIMIT 0 OFFSET X'31'", "rows", nil},
		{"aggregate_zero_fraction", "SELECT count(*) AS v FROM t LIMIT 0 OFFSET 1.5", "rows", nil},
		{"aggregate_zero_text", "SELECT count(*) AS v FROM t LIMIT 0 OFFSET 'bad'", "rows", nil},
		{"aggregate_zero_overflow", "SELECT count(*) AS v FROM t LIMIT 0 OFFSET abs(-9223372036854775808)", "rows", nil},
		{"grouped_zero_null", "SELECT k AS v,count(*) AS n FROM t GROUP BY k ORDER BY k LIMIT 0 OFFSET NULL", "rows", nil},
		{"grouped_zero_blob", "SELECT k AS v,count(*) AS n FROM t GROUP BY k ORDER BY k LIMIT 0 OFFSET X'31'", "rows", nil},
		{"grouped_zero_fraction", "SELECT k AS v,count(*) AS n FROM t GROUP BY k ORDER BY k LIMIT 0 OFFSET 1.5", "rows", nil},
		{"grouped_zero_text", "SELECT k AS v,count(*) AS n FROM t GROUP BY k ORDER BY k LIMIT 0 OFFSET 'bad'", "rows", nil},
		{"grouped_zero_overflow", "SELECT k AS v,count(*) AS n FROM t GROUP BY k ORDER BY k LIMIT 0 OFFSET abs(-9223372036854775808)", "rows", nil},
		{"constant_explain", "EXPLAIN QUERY PLAN SELECT 1 AS v LIMIT NULL OFFSET abs(-9223372036854775808)", "plan", nil},
		{"single_explain", "EXPLAIN QUERY PLAN SELECT id AS v FROM t ORDER BY id LIMIT NULL OFFSET abs(-9223372036854775808)", "plan", nil},
		{"join_explain", "EXPLAIN QUERY PLAN SELECT t.id AS v FROM t LEFT JOIN t AS u ON t.id=u.id ORDER BY t.id LIMIT NULL OFFSET abs(-9223372036854775808)", "plan", nil},
		{"aggregate_explain", "EXPLAIN QUERY PLAN SELECT count(*) AS v FROM t LIMIT NULL OFFSET abs(-9223372036854775808)", "plan", nil},
		{"grouped_explain", "EXPLAIN QUERY PLAN SELECT k AS v,count(*) AS n FROM t GROUP BY k ORDER BY k LIMIT NULL OFFSET abs(-9223372036854775808)", "plan", nil},
		{"zero_column", "SELECT 1 AS v LIMIT 0 OFFSET missing", "column", nil},
		{"zero_function", "SELECT 1 AS v LIMIT 0 OFFSET nosuch(1)", "function", nil},
		{"zero_arity", "SELECT 1 AS v LIMIT 0 OFFSET abs(1,2)", "arity", nil},
		{"null_column", "SELECT id AS v FROM t LIMIT NULL OFFSET missing", "column", nil},
		{"overflow_column", "SELECT id AS v FROM t LIMIT abs(-9223372036854775808) OFFSET missing", "column", nil},
		{"on_column", "SELECT t.id AS v FROM t LEFT JOIN t AS u ON missing=1 LIMIT NULL", "column", nil},
		{"on_function", "SELECT t.id AS v FROM t LEFT JOIN t AS u ON nosuch(1)=1 LIMIT NULL", "function", nil},
		{"limit_column", "SELECT 1 AS v LIMIT missing OFFSET abs(-9223372036854775808)", "column", nil},
		{"explain_arity", "EXPLAIN QUERY PLAN SELECT 1 AS v LIMIT NULL OFFSET abs(1,2)", "arity", nil},
		{"grouped_column", "SELECT k AS v,count(*) AS n FROM t GROUP BY k LIMIT NULL OFFSET missing", "column", nil},
		{"constant_negative", "SELECT 1 AS v LIMIT -1 OFFSET -1", "rows", [][]any{{int64(1)}}},
		{"single_offset", "SELECT id AS v FROM t ORDER BY id LIMIT 1 OFFSET 1", "rows", [][]any{{int64(2)}}},
		{"join_offset", "SELECT t.id AS v FROM t LEFT JOIN t AS u ON t.id=u.id ORDER BY t.id LIMIT 2 OFFSET 1", "rows", [][]any{{int64(2)}, {int64(3)}}},
		{"global_discard", "SELECT count(*) AS v FROM t LIMIT 1 OFFSET 1", "rows", nil},
		{"grouped_offset", "SELECT k AS v,count(*) AS n FROM t GROUP BY k ORDER BY k LIMIT 1 OFFSET 1", "rows", [][]any{{int64(2), int64(1)}}},
		{"limit_mismatch", "SELECT 1 AS v LIMIT NULL OFFSET 0", "mismatch", nil},
		{"limit_overflow", "SELECT 1 AS v LIMIT abs(-9223372036854775808) OFFSET 0", "overflow", nil},
		{"offset_overflow", "SELECT 1 AS v LIMIT 1 OFFSET abs(-9223372036854775808)", "overflow", nil},
	}
	if len(cases) != 48 {
		t.Fatal("oracle case count changed")
	}
	db := limitConversionFixture(t)
	before := db.counters
	comparisons := 0
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			oracle := limitPhaseOracle(t, executable, tc.query, tc.family)
			code, nativePart, _ := limitPhaseFamily(t, tc.family)
			rows, err := db.Query(context.Background(), tc.query)
			if code != CodeOK {
				limitPhaseError(t, rows, err, code, nativePart)
				if code == CodeMismatch && !errors.Is(err, ErrMismatch) {
					t.Fatal("mismatch lost its native error identity", err)
				}
			} else {
				if err != nil || rows == nil {
					t.Fatalf("native rows=%v error=%v; official success", rows, err)
				}
				plan := rows.Plan()
				got := limitConversionCollect(t, rows)
				if tc.family == "plan" {
					// Plan shape and successful preparation are checked separately;
					// native node IDs/details are not compared to official text.
					if len(got) != 1 || len(got[0]) != 4 {
						t.Fatalf("invalid native plan: %#v", got)
					}
					if detail, ok := got[0][3].(string); !ok || detail == "" {
						t.Fatal("missing native plan detail")
					}
				} else if !reflect.DeepEqual(oracle, tc.want) || !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("%s: native=%#v official=%#v want=%#v", tc.query, got, oracle, tc.want)
				}
				if (tc.family == "plan" || strings.Contains(tc.name, "_zero_")) && (rows.Stats().BaseRowFetches != 0 || rows.RelationalStats().SourceOpens != 0) {
					t.Fatal("suppressed native execution fetched rows", rows.Stats(), rows.RelationalStats())
				}
				if !reflect.DeepEqual(plan, rows.Plan()) || rows.RelationalStats().CurrentBytes != 0 {
					t.Fatal("native plan lifetime or resource ownership changed")
				}
			}
			if len(db.rows) != 0 || db.counters != before {
				t.Fatal("native query leaked a Rows lease or changed connection counters")
			}
			comparisons++
		})
	}
	if comparisons != 48 {
		t.Fatalf("completed %d comparisons; want 48", comparisons)
	}
	t.Logf("fixed phase comparisons completed: %d", comparisons)
}
