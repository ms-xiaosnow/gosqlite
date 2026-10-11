//go:build oracle

package sqlite

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/testoracle"
)

// CLI results/error families are observed here, never C-API prepare/bind/step
// phases. Each fixed query uses a fresh :memory: database and normal completion.
// ForTest owns the current -noinit, allowlisted child environment and package-CWD
// contract. This test does not change the parent environment or temporary root.
func ipkEqualityOracle(t *testing.T, executable string, tc ipkEqualityCase) [][]any {
	t.Helper()
	cmd := testoracle.ForTest(t, executable, "-batch", "-bail", "-json", "-cmd", ".explain off", "-cmd", ".log stderr", ":memory:")
	input := ipkEqualitySeed + "\n"
	if len(tc.args) != 0 {
		// This single real binding is intentionally present: an unbound NULL
		// would suppress the expected matching-row ESCAPE error.
		if len(tc.args) != 1 || tc.args[0] != int64(1) {
			t.Fatal("unreviewed oracle binding", tc.args)
		}
		input += ".parameter init\n.parameter set ?1 1\n"
	}
	cmd.Stdin = strings.NewReader(input + tc.query + ";\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	// These are immutable submitted arguments and pinned factory expectations,
	// not observations of the child environment, working directory or argv.
	t.Logf("official expected factory executable=%q arguments_after_noinit=%q contract=%q", executable, []string{"-batch", "-bail", "-json", "-cmd", ".explain off", "-cmd", ".log stderr", ":memory:"}, "prepend -noinit; capture package CWD; child allowlist LC_ALL=C LANG=C TZ=UTC SQLITE_TMPDIR=TempRoot TMPDIR=TempRoot; TempRoot selects GOTMPDIR, then TMPDIR, then /tmp")
	t.Logf("official observed SQL=%q stdin=%q exit=%v stdout=%q stderr=%q", tc.query, input+tc.query+";\n", err, stdout.String(), stderr.String())
	if tc.family != "" {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "(1) ") || !strings.Contains(stderr.String(), tc.family) {
			t.Fatalf("official query=%s exit=%v stdout=%q stderr=%q; want SQL error/%s", tc.query, err, stdout.String(), stderr.String(), tc.family)
		}
		return nil
	}
	if err != nil || stderr.Len() != 0 {
		t.Fatalf("official query=%s exit=%v stdout=%q stderr=%q", tc.query, err, stdout.String(), stderr.String())
	}
	if stdout.Len() == 0 {
		if tc.plan {
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
		t.Fatal("trailing official output", extra, err)
	}
	if tc.plan {
		if len(records) != 1 || len(records[0]) != 4 {
			t.Fatal("official plan shape", records)
		}
		for _, key := range []string{"id", "parent", "notused"} {
			n, ok := records[0][key].(json.Number)
			if !ok {
				t.Fatal("official plan field", key, records)
			}
			if _, err := n.Int64(); err != nil {
				t.Fatal(err)
			}
		}
		detail, ok := records[0]["detail"].(string)
		if !ok {
			t.Fatal("official plan detail", records)
		}
		if tc.method == "rowid" {
			if !strings.HasPrefix(detail, "SEARCH t USING INTEGER PRIMARY KEY") {
				t.Fatal("official IPK plan", detail)
			}
		} else if detail != "SCAN t" {
			t.Fatal("whole-comparison control did not retain an official SCAN", detail)
		}
		return nil
	}
	var rows [][]any
	for _, record := range records {
		if len(record) != 1 {
			t.Fatal("official projection shape", record)
		}
		n, ok := record["id"].(json.Number)
		if !ok {
			t.Fatal("official non-integer id", record)
		}
		id, err := n.Int64()
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, []any{id})
	}
	return rows
}

func TestIPKEqualityErrorTimingPinnedOracle(t *testing.T) {
	executable := os.Getenv("SQLITE_ORACLE")
	if executable == "" {
		t.Fatal("SQLITE_ORACLE must name the pinned SQLite 3.54.0 CLI")
	}
	identityCommand := testoracle.ForTest(t, executable, "-batch", "-bail", ":memory:", "SELECT sqlite_version() || '|' || sqlite_source_id();")
	version, err := identityCommand.CombinedOutput()
	// Expected construction inputs are distinct from the observed process result.
	t.Logf("official identity expected factory executable=%q arguments_after_noinit=%q contract=%q", executable, []string{"-batch", "-bail", ":memory:", "SELECT sqlite_version() || '|' || sqlite_source_id();"}, "prepend -noinit; capture package CWD; child allowlist LC_ALL=C LANG=C TZ=UTC SQLITE_TMPDIR=TempRoot TMPDIR=TempRoot; TempRoot selects GOTMPDIR, then TMPDIR, then /tmp")
	t.Logf("official identity observed combined_output=%q exit=%v", version, err)
	const identity = "3.54.0|2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3"
	if err != nil || strings.TrimSpace(string(version)) != identity {
		t.Fatalf("official identity=%q error=%v", version, err)
	}
	cases := ipkEqualityCases()
	if len(cases) != 24 {
		t.Fatal("finite oracle case inventory changed", len(cases))
	}
	db := ipkEqualityFixture(t)
	completed := 0
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			official := ipkEqualityOracle(t, executable, tc)
			native := ipkEqualityNative(t, db, tc)
			if !tc.plan && (!reflect.DeepEqual(official, tc.want) || !reflect.DeepEqual(native, official)) {
				t.Fatal("typed result difference", native, official, tc.want)
			}
			completed++
		})
	}
	if completed != 24 {
		t.Fatal("incomplete oracle comparisons", completed)
	}
	t.Logf("24 explicit SQL cases plus one pin identity check completed")
}
