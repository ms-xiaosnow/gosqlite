//go:build oracle

package sqlite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/testoracle"
)

// Source-only candidate: every expectation below is a frozen proposal until a
// separately authorized run. The first case also retains historical evidence;
// that evidence is not a runtime result for this candidate.
//
// Preserve all fourteen original fixture statements, including all three
// secondary indexes. The no-index control is a separate explicit fixture.
const leftWherePrefilterOracleHistoricalSeed = `CREATE TABLE t0(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER,txt TEXT,raw BLOB);
CREATE TABLE t1(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER,txt TEXT,raw BLOB);
CREATE TABLE tn(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER,txt TEXT,raw BLOB);
CREATE TABLE u(y INTEGER);
CREATE TABLE empty(y INTEGER);
CREATE TABLE names(id INTEGER PRIMARY KEY,"true" INTEGER,"false" INTEGER,v INTEGER);
INSERT INTO t0 VALUES(1,0,-9223372036854775808,'01','01');
INSERT INTO t1 VALUES(1,1,-9223372036854775808,'01','01');
INSERT INTO tn VALUES(1,NULL,-9223372036854775808,'01','01');
INSERT INTO u VALUES(2);
INSERT INTO names VALUES(1,0,1,-9223372036854775808);
CREATE INDEX fi0 ON t0(flag);
CREATE INDEX fi1 ON t1(flag);
CREATE INDEX fin ON tn(flag);`

const leftWherePrefilterOracleNoIndexSeed = `CREATE TABLE t0(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER,txt TEXT,raw BLOB);
CREATE TABLE t1(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER,txt TEXT,raw BLOB);
CREATE TABLE tn(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER,txt TEXT,raw BLOB);
CREATE TABLE u(y INTEGER);
CREATE TABLE empty(y INTEGER);
CREATE TABLE names(id INTEGER PRIMARY KEY,"true" INTEGER,"false" INTEGER,v INTEGER);
INSERT INTO t0 VALUES(1,0,-9223372036854775808,'01','01');
INSERT INTO t1 VALUES(1,1,-9223372036854775808,'01','01');
INSERT INTO tn VALUES(1,NULL,-9223372036854775808,'01','01');
INSERT INTO u VALUES(2);
INSERT INTO names VALUES(1,0,1,-9223372036854775808);`

const leftWherePrefilterOracleFactoryContract = "prepend -noinit; capture package CWD; child allowlist LC_ALL=C LANG=C TZ=UTC SQLITE_TMPDIR=TempRoot TMPDIR=TempRoot; TempRoot selects GOTMPDIR, then TMPDIR, then /tmp"

type leftWherePrefilterOracleCase struct {
	name, seed, query, family string
	columns                   []string
	want                      [][]any
}

type leftWherePrefilterOracleResult struct {
	rows   [][]any
	family string
}

// Exactly sixteen literal comparisons, with no generated cases or cross-product.
// Only case 10 adds one healthy INSERT to the intact historical fixture.
func leftWherePrefilterOracleCases() []leftWherePrefilterOracleCase {
	return []leftWherePrefilterOracleCase{
		{
			name: "01_historical_full_fixture", seed: leftWherePrefilterOracleHistoricalSeed,
			query:   "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) AND t.flag WHERE t.flag=99",
			columns: []string{"id", "y"},
		},
		{
			name: "02_historical_without_indexes", seed: leftWherePrefilterOracleNoIndexSeed,
			query:   "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) AND t.flag WHERE t.flag=99",
			columns: []string{"id", "y"},
		},
		{
			name: "03_kept_historical_overflow", seed: leftWherePrefilterOracleHistoricalSeed,
			query:   "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) AND t.flag WHERE t.flag=0",
			columns: []string{"id", "y"}, family: "integer overflow",
		},
		{
			name: "04_commuted_rejecting_equality", seed: leftWherePrefilterOracleHistoricalSeed,
			query:   "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) AND t.flag WHERE 99=t.flag",
			columns: []string{"id", "y"},
		},
		{
			name: "05_two_left_equalities", seed: leftWherePrefilterOracleHistoricalSeed,
			query:   "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) AND t.flag WHERE t.flag=99 AND t.id=1",
			columns: []string{"id", "y"},
		},
		{
			name: "06_direct_left_result_alias", seed: leftWherePrefilterOracleHistoricalSeed,
			query:   "SELECT t.flag AS left_flag,u.y FROM t0 t LEFT JOIN u ON abs(t.v) AND t.flag WHERE left_flag=99",
			columns: []string{"left_flag", "y"},
		},
		{
			name: "07_null_left_nonnull_equality", seed: leftWherePrefilterOracleHistoricalSeed,
			query:   "SELECT t.id,u.y FROM tn t LEFT JOIN u ON abs(t.v) AND t.flag WHERE t.flag=99",
			columns: []string{"id", "y"},
		},
		{
			name: "08_equality_to_null", seed: leftWherePrefilterOracleHistoricalSeed,
			query:   "SELECT t.id,u.y FROM tn t LEFT JOIN u ON abs(t.v) AND t.flag WHERE t.flag=NULL",
			columns: []string{"id", "y"},
		},
		{
			name: "09_kept_false_on_null_extension", seed: leftWherePrefilterOracleHistoricalSeed,
			query:   "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON 0 WHERE t.flag=0",
			columns: []string{"id", "y"}, want: [][]any{{int64(1), nil}},
		},
		{
			name: "10_kept_two_right_matches", seed: leftWherePrefilterOracleHistoricalSeed + "\nINSERT INTO u VALUES(3);",
			query:   "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON 1 WHERE t.flag=0 ORDER BY u.y",
			columns: []string{"id", "y"}, want: [][]any{{int64(1), int64(2)}, {int64(1), int64(3)}},
		},
		{
			name: "11_rejected_empty_right", seed: leftWherePrefilterOracleHistoricalSeed,
			query:   "SELECT t.id,e.y FROM t0 t LEFT JOIN empty e ON abs(t.v) AND t.flag WHERE t.flag=99",
			columns: []string{"id", "y"},
		},
		{
			name: "12_kept_empty_right", seed: leftWherePrefilterOracleHistoricalSeed,
			query:   "SELECT t.id,e.y FROM t0 t LEFT JOIN empty e ON abs(t.v) AND t.flag WHERE t.flag=0",
			columns: []string{"id", "y"}, want: [][]any{{int64(1), nil}},
		},
		{
			name: "13_global_count_rejected", seed: leftWherePrefilterOracleHistoricalSeed,
			query:   "SELECT count(*) AS n FROM t0 t LEFT JOIN u ON abs(t.v) AND t.flag WHERE t.flag=99",
			columns: []string{"n"}, want: [][]any{{int64(0)}},
		},
		{
			name: "14_grouped_rejected_without_having", seed: leftWherePrefilterOracleHistoricalSeed,
			query:   "SELECT t.flag,count(*) AS n FROM t0 t LEFT JOIN u ON abs(t.v) AND t.flag WHERE t.flag=99 GROUP BY t.flag",
			columns: []string{"flag", "n"},
		},
		{
			name: "15_limit_zero_error_on", seed: leftWherePrefilterOracleHistoricalSeed,
			query:   "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON abs(t.v) AND t.flag WHERE t.flag=0 LIMIT 0",
			columns: []string{"id", "y"},
		},
		{
			name: "16_missing_on_column", seed: leftWherePrefilterOracleHistoricalSeed,
			query:   "SELECT t.id,u.y FROM t0 t LEFT JOIN u ON u.missing=1 WHERE t.flag=99",
			columns: []string{"id", "y"}, family: "no such column",
		},
	}
}

// This recognizes only the two reviewed error families. An unknown error is
// never treated as equivalent merely because both implementations failed.
func leftWherePrefilterOracleFamily(err error, detail string) string {
	if err == nil {
		return ""
	}
	for _, family := range []string{"integer overflow", "no such column"} {
		if strings.Contains(detail, family) {
			return family
		}
	}
	return "unclassified"
}

func leftWherePrefilterOfficial(t *testing.T, executable string, tc leftWherePrefilterOracleCase) leftWherePrefilterOracleResult {
	t.Helper()
	arguments := []string{"-batch", "-bail", "-json", "-cmd", ".explain off", "-cmd", ".log stderr", ":memory:"}
	cmd := testoracle.ForTest(t, executable, arguments...)
	input := tc.seed + "\n" + tc.query + ";\n"
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	// Run waits for natural completion. No CommandContext, timeout, signal,
	// Cancel, WaitDelay, kill, retry, or alternative process factory is used.
	err := cmd.Run()
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	family := leftWherePrefilterOracleFamily(err, stderr.String())
	// Submitted factory inputs are expectations, not inspected child state.
	t.Logf("official expected factory executable=%q arguments_after_noinit=%q contract=%q", executable, arguments, leftWherePrefilterOracleFactoryContract)
	t.Logf("official observed SQL=%q stdin=%q natural_exit_code=%d run_error=%v error_family=%q stdout=%q stderr=%q", tc.query, input, exitCode, err, family, stdout.String(), stderr.String())
	if tc.family != "" {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 || exitCode != 1 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "(1) ") || family != tc.family {
			t.Fatalf("official result does not match proposed CodeError/%s", tc.family)
		}
		return leftWherePrefilterOracleResult{family: family}
	}
	if err != nil || exitCode != 0 || stderr.Len() != 0 || family != "" {
		t.Fatal("official expected successful natural exit")
	}
	if strings.TrimSpace(stdout.String()) == "" {
		t.Log("official typed rows: nil (successful empty result)")
		return leftWherePrefilterOracleResult{}
	}
	decoder := json.NewDecoder(&stdout)
	decoder.UseNumber()
	var records []map[string]any
	if err := decoder.Decode(&records); err != nil {
		t.Fatal("official JSON decode", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatal("trailing official output", extra, err)
	}
	var got [][]any
	for _, record := range records {
		if len(record) != len(tc.columns) {
			t.Fatal("official projection width", record, tc.columns)
		}
		row := make([]any, len(tc.columns))
		for i, column := range tc.columns {
			raw, exists := record[column]
			if !exists {
				t.Fatal("official missing projected column", column, record)
			}
			switch value := raw.(type) {
			case nil:
				// A present JSON null is SQL NULL, never an absent field or 0.
				row[i] = nil
			case json.Number:
				integer, err := value.Int64()
				if err != nil {
					t.Fatal("official non-integer numeric storage class", value, err)
				}
				row[i] = integer
			default:
				t.Fatalf("official unexpected storage class %T for %s: %#v", raw, column, raw)
			}
		}
		got = append(got, row)
	}
	t.Logf("official typed rows=%#v", got)
	return leftWherePrefilterOracleResult{rows: got}
}

// Each comparison owns a newly SQL-created, healthy 4096-byte rollback file.
// No shared fixture, raw-file mutation, recovery option, or fault hook is used.
func leftWherePrefilterOracleNative(t *testing.T, tc leftWherePrefilterOracleCase) leftWherePrefilterOracleResult {
	t.Helper()
	path := filepath.Join(t.TempDir(), "left-where-prefilter.db")
	db, err := Open(path, Options{PageSize: 4096})
	if err != nil {
		t.Fatal("native fixture open", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error("native fixture close", err)
		}
	})
	t.Logf("native fixture path=%q SQL=%q", path, tc.seed)
	for _, statement := range strings.Split(tc.seed, ";") {
		statement = strings.TrimSpace(statement)
		if statement != "" {
			if _, err := db.Exec(context.Background(), statement); err != nil {
				t.Fatalf("native fixture SQL=%q error=%v", statement, err)
			}
		}
	}
	info := db.Info()
	if info.PageSize != 4096 || info.ReadVersion != 1 || info.WriteVersion != 1 {
		t.Fatal("native fixture must be ordinary 4096-byte rollback", info)
	}
	rows, queryErr := db.Query(context.Background(), tc.query)
	var got [][]any
	var rowsErr, closeErr error
	if rows != nil {
		t.Cleanup(func() {
			if err := rows.Close(); err != nil {
				t.Error("native rows cleanup", err)
			}
		})
		if queryErr == nil {
			for rows.Next() {
				values := rows.Values()
				if len(values) != len(tc.columns) {
					t.Fatal("native projection width", values, tc.columns)
				}
				got = append(got, values)
			}
			rowsErr = rows.Err()
		}
		closeErr = rows.Close()
	}
	resultErr := queryErr
	if resultErr == nil {
		resultErr = rowsErr
	}
	detail := ""
	if resultErr != nil {
		detail = resultErr.Error()
	}
	family := leftWherePrefilterOracleFamily(resultErr, detail)
	t.Logf("native observed SQL=%q QueryErr=%v RowsErr=%v CloseErr=%v error_family=%q typed_rows=%#v", tc.query, queryErr, rowsErr, closeErr, family, got)
	if closeErr != nil || queryErr != nil && rows != nil || queryErr == nil && rows == nil {
		t.Fatal("native Query/Rows ownership contract", queryErr, rows, closeErr)
	}
	if tc.family == "" {
		if resultErr != nil || family != "" {
			t.Fatal("native expected success", resultErr)
		}
	} else {
		var typed *Error
		if !errors.As(resultErr, &typed) || typed.Code != CodeError || family != tc.family || len(got) != 0 {
			t.Fatalf("native result error=%v rows=%#v; want CodeError/%s", resultErr, got, tc.family)
		}
	}
	return leftWherePrefilterOracleResult{rows: got, family: family}
}

func TestLeftWherePrefilterSQLite354(t *testing.T) {
	executable := os.Getenv("SQLITE_ORACLE")
	if executable == "" {
		t.Fatal("SQLITE_ORACLE must name the pinned SQLite 3.54.0 CLI")
	}
	const identitySQL = "SELECT sqlite_version() || '|' || sqlite_source_id();"
	identityArguments := []string{"-batch", "-bail", ":memory:", identitySQL}
	identityCommand := testoracle.ForTest(t, executable, identityArguments...)
	var stdout, stderr bytes.Buffer
	identityCommand.Stdout, identityCommand.Stderr = &stdout, &stderr
	err := identityCommand.Run()
	exitCode := -1
	if identityCommand.ProcessState != nil {
		exitCode = identityCommand.ProcessState.ExitCode()
	}
	family := leftWherePrefilterOracleFamily(err, stderr.String())
	t.Logf("official identity expected factory executable=%q arguments_after_noinit=%q contract=%q", executable, identityArguments, leftWherePrefilterOracleFactoryContract)
	t.Logf("official identity observed SQL=%q stdin=%q natural_exit_code=%d run_error=%v error_family=%q stdout=%q stderr=%q", identitySQL, "", exitCode, err, family, stdout.String(), stderr.String())
	const identity = "3.54.0|2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3"
	if err != nil || exitCode != 0 || stderr.Len() != 0 || strings.TrimSpace(stdout.String()) != identity {
		t.Fatal("official identity mismatch; no query comparisons started")
	}
	cases := leftWherePrefilterOracleCases()
	if len(cases) != 16 {
		t.Fatal("finite oracle inventory changed", len(cases))
	}
	completed := 0
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			official := leftWherePrefilterOfficial(t, executable, tc)
			native := leftWherePrefilterOracleNative(t, tc)
			if official.family != tc.family || native.family != official.family || !reflect.DeepEqual(official.rows, tc.want) || !reflect.DeepEqual(native.rows, official.rows) {
				t.Fatalf("proposed typed result/error family mismatch: native=%#v official=%#v want_rows=%#v want_family=%q", native, official, tc.want, tc.family)
			}
			completed++
		})
	}
	if completed != 16 {
		t.Fatal("incomplete oracle comparisons", completed)
	}
	t.Log("16 explicit comparisons plus one identity process completed with natural exits; CLI error families do not establish C-API prepare/step phase")
}
