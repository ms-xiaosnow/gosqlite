//go:build oracle

package sqlite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// Fixed ordinary SQL only. The official CLI always completes normally; this
// helper has no cancellation, timeout, signal, extension, or observer dependency.
func limitConversionOracle(t *testing.T, executable, query string) ([][]any, bool) {
	t.Helper()
	cmd := exec.Command(executable, "-batch", "-bail", "-json", "-cmd", ".log stderr", ":memory:")
	cmd.Stdin = strings.NewReader(limitConversionSeed + "\n" + query + ";\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.HasPrefix(stderr.String(), "(20) ") || !strings.Contains(stderr.String(), "datatype mismatch") || stdout.Len() != 0 {
			t.Fatalf("oracle %s: exit=%v stdout=%q stderr=%q", query, err, stdout.String(), stderr.String())
		}
		return nil, true
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected oracle stderr: %s", stderr.String())
	}
	if stdout.Len() == 0 {
		return nil, false
	}
	decoder := json.NewDecoder(&stdout)
	decoder.UseNumber()
	var records []map[string]json.Number
	if err := decoder.Decode(&records); err != nil {
		t.Fatal(err)
	}
	var rows [][]any
	for _, record := range records {
		var row []any
		for _, key := range []string{"v", "n"} {
			if raw, ok := record[key]; ok {
				n, err := raw.Int64()
				if err != nil {
					t.Fatal(err)
				}
				row = append(row, n)
			}
		}
		if len(row) != len(record) {
			t.Fatalf("unexpected oracle record: %#v", record)
		}
		rows = append(rows, row)
	}
	return rows, false
}

func TestLimitConversionPinnedOracle(t *testing.T) {
	executable := os.Getenv("SQLITE_ORACLE")
	if executable == "" {
		t.Fatal("SQLITE_ORACLE must name the pinned SQLite 3.54.0 CLI")
	}
	version, err := exec.Command(executable, "-batch", ":memory:", "SELECT sqlite_version() || '|' || sqlite_source_id();").CombinedOutput()
	const wantVersion = "3.54.0|2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3"
	if err != nil || strings.TrimSpace(string(version)) != wantVersion {
		t.Fatalf("oracle identity=%q err=%v", version, err)
	}
	db := limitConversionFixture(t)
	expressions := []string{
		"1", "-1", "9223372036854775807", "-9223372036854775808", "1.0",
		"9223372036854774784.0", "-9223372036854774784.0",
		"'1'", "'1.0'", "'.1e1'", "'1.'", "'1e+0'", "'10E-1'",
		"'9007199254740993'", "'9223372036854775807'", "'-9223372036854775808'",
		"CAST(X'20090a0d0b0c3120090a0d0b0c' AS TEXT)",
		"CAST(X'31007461696c' AS TEXT)", "CAST(X'3930303731393932353437343039393300' AS TEXT)",
		"NULL", "X'31'", "1.5", "1e999", "-1e999", "5e-324",
		"9223372036854775808.0", "-9223372036854775808.0",
		"'9223372036854775808'", "'-9223372036854775808.0'", "'-9223372036854775809'",
		"'0x1p0'", "'1_0'", "'1e+'", "'1junk'", "''", "'NaN'", "'Inf'",
		"CAST(X'c2a031' AS TEXT)", "CAST(X'3100' AS TEXT)",
		"CAST(X'00' AS TEXT)", "CAST(X'312e3500' AS TEXT)",
		"CAST(X'3932323333373230333638353437373538303700' AS TEXT)",
		"CAST(X'2d3932323333373230333638353437373538303800' AS TEXT)",
	}
	shapes := []string{
		"SELECT id AS v FROM t ORDER BY id",
		"SELECT t.id AS v FROM t LEFT JOIN t AS u ON t.id=u.id ORDER BY t.id",
		"SELECT count(*) AS v FROM t",
		"SELECT k AS v,count(*) AS n FROM t GROUP BY k ORDER BY k",
	}
	comparisons := 0
	for si, base := range shapes {
		for _, offset := range []bool{false, true} {
			for ei, expression := range expressions {
				t.Run(fmt.Sprintf("shape%d/offset%t/expression%02d", si, offset, ei), func(t *testing.T) {
					suffix := " LIMIT " + expression
					if offset {
						suffix = " LIMIT 1 OFFSET " + expression
					}
					q := base + suffix
					want, mismatch := limitConversionOracle(t, executable, q)
					rows, err := db.Query(context.Background(), q)
					if mismatch {
						var typed *Error
						nonnil := rows != nil
						if nonnil {
							rows.Close()
						}
						if nonnil || !errors.Is(err, ErrMismatch) || !errors.As(err, &typed) || typed.Code != CodeMismatch {
							t.Fatalf("%s: rows nonnil=%t error=%v; oracle datatype mismatch (20)", q, nonnil, err)
						}
					} else {
						if err != nil || rows == nil {
							t.Fatalf("%s: rows=%v error=%v; oracle success", q, rows, err)
						}
						got := limitConversionCollect(t, rows)
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("%s: got %#v; oracle %#v", q, got, want)
						}
					}
					if len(db.rows) != 0 {
						t.Fatal("Rows lease leaked")
					}
					comparisons++
				})
			}
		}
	}
	t.Logf("fixed comparisons completed: %d", comparisons)
}
