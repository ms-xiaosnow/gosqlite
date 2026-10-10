//go:build oracle

// Package testkit_test contains opt-in differential tests. Production packages
// never import os/exec or delegate SQL execution to the external test oracle.
package testkit_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/internal/buildinfo"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type confResult struct {
	columns []string
	rows    [][]any
}
type confCase struct {
	name, query, oracleQuery string
	args                     []any
	emptyColumns             []string
}

var confVersionOnce sync.Once
var confVersionErr error

func confOracle(t *testing.T) string {
	t.Helper()
	path := os.Getenv("SQLITE_ORACLE")
	if path == "" {
		t.Fatal("oracle acceptance requires an explicit SQLITE_ORACLE path; tests must not silently skip")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("SQLITE_ORACLE must identify the pinned official executable: %v", err)
	}

	confVersionOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, path, "-batch", "-bail", "-noheader", "-list", ":memory:", "SELECT sqlite_version(),sqlite_source_id();").CombinedOutput()
		if err != nil {
			confVersionErr = fmt.Errorf("oracle baseline check: %w: %s", err, out)
			return
		}
		want := buildinfo.SQLiteBaseline + "|" + buildinfo.SQLiteSourceID
		if strings.TrimSpace(string(out)) != want {
			confVersionErr = fmt.Errorf("oracle baseline mismatch: got %q; want %q", strings.TrimSpace(string(out)), want)
		}
	})
	if confVersionErr != nil {
		t.Fatal(confVersionErr)
	}
	return path
}
func confCLI(t *testing.T, path, query string, readonly bool) []byte {
	t.Helper()
	args := []string{"-batch", "-bail", "-quote", "-header"}
	if readonly {
		args = append(args, "-readonly")
	}
	args = append(args, path)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, confOracle(t), args...)
	cmd.Stdin = strings.NewReader(query + "\n")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("official oracle failed for %q: %v: %s", query, err, stderr.String())
	}
	return out
}

// Parse the official shell's quote mode rather than JSON: quote mode preserves
// INTEGER versus REAL, BLOB versus TEXT, empty BLOBs, and duplicate column names.
// The shell truncates NUL-containing TEXT in quote mode, so those fixture values
// are compared separately through hex()/length(), never silently truncated here.
func confQuoted(data []byte) ([][]any, error) {
	var out [][]any
	at := 0
	for at < len(data) {
		var row []any
		for {
			if at >= len(data) {
				return nil, fmt.Errorf("missing value")
			}
			blob := data[at] == 'x' || data[at] == 'X'
			if blob {
				at++
			}
			var value any
			if at < len(data) && data[at] == '\'' {
				at++
				var b []byte
				closed := false
				for at < len(data) {
					c := data[at]
					at++
					if c == '\'' {
						if at < len(data) && data[at] == '\'' {
							at++
							b = append(b, '\'')
							continue
						}
						closed = true
						break
					}
					b = append(b, c)
				}
				if !closed {
					return nil, fmt.Errorf("unterminated quoted value")
				}
				if blob {
					v, err := hex.DecodeString(string(b))
					if err != nil {
						return nil, err
					}
					if v == nil {
						v = []byte{}
					}
					value = v
				} else {
					value = string(b)
				}
			} else {
				if blob {
					return nil, fmt.Errorf("invalid blob")
				}
				start := at
				for at < len(data) && data[at] != ',' && data[at] != '\n' && data[at] != '\r' {
					at++
				}
				s := string(data[start:at])
				if s == "NULL" {
					value = nil
				} else if !strings.ContainsAny(s, ".eE") && s != "Inf" && s != "-Inf" {
					n, err := strconv.ParseInt(s, 10, 64)
					if err != nil {
						return nil, err
					}
					value = n
				} else {
					n, err := strconv.ParseFloat(s, 64)
					if err != nil {
						return nil, err
					}
					value = n
				}
			}
			row = append(row, value)
			if at == len(data) {
				break
			}
			if data[at] == ',' {
				at++
				continue
			}
			if data[at] == '\r' {
				at++
			}
			if at < len(data) && data[at] == '\n' {
				at++
				break
			}
			return nil, fmt.Errorf("unexpected quote output byte at %d", at)
		}
		out = append(out, row)
	}
	return out, nil
}
func confExpected(t *testing.T, path, query string) confResult {
	t.Helper()
	raw := confCLI(t, path, query, true)
	rows, err := confQuoted(raw)
	if err != nil {
		t.Fatalf("parse oracle quote output: %v", err)
	}
	var result confResult
	if len(rows) == 0 {
		return result
	}
	for _, v := range rows[0] {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("non-string oracle column label %T", v)
		}
		result.columns = append(result.columns, s)
	}
	result.rows = rows[1:]
	return result
}
func confNative(t *testing.T, db *sqlite.DB, query string, args ...any) confResult {
	t.Helper()
	rows, err := db.Query(context.Background(), query, args...)
	if err != nil {
		t.Fatalf("native query %q: %v", query, err)
	}
	defer rows.Close()
	result := confResult{columns: rows.Columns()}
	for rows.Next() {
		result.rows = append(result.rows, rows.Values())
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("native iteration %q: %v", query, err)
	}
	return result
}
func confCompare(t *testing.T, want, got confResult) {
	t.Helper()
	if want.columns != nil && !reflect.DeepEqual(want.columns, got.columns) {
		t.Errorf("column names: got %#v; oracle %#v", got.columns, want.columns)
	}
	if len(want.rows) != len(got.rows) {
		t.Fatalf("row count: got %d; oracle %d", len(got.rows), len(want.rows))
	}
	for i := range want.rows {
		if len(want.rows[i]) != len(got.rows[i]) {
			t.Fatalf("row %d width mismatch", i)
		}
		for j, w := range want.rows[i] {
			g := got.rows[i][j]
			if !reflect.DeepEqual(w, g) {
				t.Errorf("row %d column %d: got %s; oracle %s", i, j, confValue(g), confValue(w))
				return
			}
		}
	}
}
func confValue(v any) string {
	s := fmt.Sprintf("%T(%v)", v, v)
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}
func confDigest(t *testing.T, path string) [32]byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(b)
}
func confUnchanged(t *testing.T, path string) func() {
	t.Helper()
	before := confDigest(t, path)
	return func() {
		t.Helper()
		if got := confDigest(t, path); got != before {
			t.Error("read-only tests modified database SHA-256")
		}
		for _, suffix := range []string{"-wal", "-journal", "-shm"} {
			if _, err := os.Stat(path + suffix); !os.IsNotExist(err) {
				t.Errorf("read-only test created sidecar %s: %v", suffix, err)
			}
		}
	}
}
func confRun(t *testing.T, path string, cases []confCase) {
	t.Helper()
	db, err := sqlite.Open(path, sqlite.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("SQL: %s", tc.query)
			q := tc.oracleQuery
			if q == "" {
				q = tc.query
			}
			want := confExpected(t, path, q)
			if want.columns == nil {
				want.columns = tc.emptyColumns
			}
			got := confNative(t, db, tc.query, tc.args...)
			confCompare(t, want, got)
		})
	}
}

func TestOraclePinnedBaseline(t *testing.T) {
	out := string(confCLI(t, ":memory:", "SELECT sqlite_version() AS version;", false))
	if !strings.Contains(out, "'3.54.0'") {
		t.Fatalf("expected pinned official SQLite 3.54.0, got %s", out)
	}
}

func TestOracleReadOnlyFixtures(t *testing.T) {
	confOracle(t)
	cases := []confCase{
		{name: "full_records", query: "SELECT * FROM samples ORDER BY id"},
		{name: "integer_serial_widths", query: "SELECT rowid, value, typeof(value) AS storage FROM numbers ORDER BY rowid"},
		{name: "quoted_unicode_identifiers", query: `SELECT "a""b", "中文" FROM quotes`},
		{name: "rowid_aliases", query: "SELECT id, rowid, _rowid_, oid FROM samples ORDER BY id LIMIT 9"},
		{name: "qualified_star", query: "SELECT s.* FROM samples AS s WHERE id >= 2 ORDER BY id LIMIT 3"},
		{name: "qualified_projection", query: "SELECT s.id, s.label FROM samples s WHERE s.id > 1000 ORDER BY s.id"},
		{name: "names_aliases", query: `SELECT ID, label AS "Mixed Case", id+1, 'it''s', length(label), (score * 2), id AS same, label AS same FROM samples WHERE id=1`},
		{name: "schema", query: "SELECT type, name, tbl_name, rootpage, sql FROM sqlite_schema ORDER BY name"},
		{name: "master_alias", query: "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"},
		{name: "where_null", query: "SELECT id FROM samples WHERE optional IS NULL ORDER BY id"},
		{name: "where_not_null", query: "SELECT id FROM samples WHERE optional IS NOT NULL ORDER BY id LIMIT 11"},
		{name: "three_valued_where", query: "SELECT id FROM samples WHERE (optional = NULL OR id = 1) AND NOT id=2 ORDER BY id"},
		{name: "numeric_affinity", query: "SELECT id FROM samples WHERE id >= '1020' AND id < '2048' ORDER BY id"},
		{name: "real_affinity", query: "SELECT score, typeof(score) FROM samples WHERE id=0 OR id=8 ORDER BY id"},
		{name: "like_one_rune", query: "SELECT id FROM samples WHERE label LIKE '%库%' ORDER BY id"},
		{name: "like_two_runes", query: "SELECT id FROM samples WHERE label LIKE '%数据%' ORDER BY id"},
		{name: "like_three_runes", query: "SELECT id FROM samples WHERE label LIKE '%数据库%' ORDER BY id"},
		{name: "like_ascii_case", query: "SELECT id FROM samples WHERE label LIKE 'ROW-00__' ORDER BY id"},
		{name: "like_unicode_wildcard", query: "SELECT id FROM samples WHERE label LIKE '数据库_é한글%' ORDER BY id"},
		{name: "not_like", query: "SELECT id FROM samples WHERE label NOT LIKE 'row-%' ORDER BY id"},
		{name: "sort_alias", query: "SELECT id AS identifier, score AS value FROM samples ORDER BY value DESC, identifier ASC LIMIT 13"},
		{name: "sort_ordinal", query: "SELECT id, optional FROM samples ORDER BY 2 DESC, 1 ASC LIMIT 13 OFFSET 3"},
		{name: "sort_expression", query: "SELECT id FROM samples WHERE id>0 AND id<20 ORDER BY id%3 DESC, id DESC LIMIT 10"},
		{name: "negative_limit", query: "SELECT value FROM numbers ORDER BY rowid LIMIT -1 OFFSET 20"},
		{name: "negative_offset", query: "SELECT value FROM numbers ORDER BY rowid LIMIT 3 OFFSET -2"},
		{name: "comma_limit", query: "SELECT value FROM numbers ORDER BY rowid LIMIT 20,3"},
		{name: "text_limit", query: "SELECT value FROM numbers ORDER BY rowid LIMIT '3.0'"},
		{name: "zero_limit", query: "SELECT id, label FROM samples LIMIT 0", emptyColumns: []string{"id", "label"}},
		{name: "empty_result", query: "SELECT id, label FROM samples WHERE id > 9223372036854775807", emptyColumns: []string{"id", "label"}},
		{name: "positional_bind", query: "SELECT id, label FROM samples WHERE id>=? AND id<? ORDER BY id LIMIT ?", oracleQuery: "SELECT id, label FROM samples WHERE id>=1020 AND id<2048 ORDER BY id LIMIT 3", args: []any{1020, 2048, 3}},
		{name: "numbered_bind", query: "SELECT ?2 AS second, ?1 AS first, ?2 AS again", oracleQuery: "SELECT 'two' AS second, 1 AS first, 'two' AS again", args: []any{1, "two"}},
		{name: "named_bind", query: "SELECT id FROM samples WHERE id=:id OR id=@other OR id=$third ORDER BY id", oracleQuery: "SELECT id FROM samples WHERE id=1 OR id=2 OR id=3 ORDER BY id", args: []any{sql.Named("third", 3), sql.Named("id", 1), sql.Named("other", 2)}},
		{name: "repeat_named_bind", query: "SELECT :v AS one, :v AS two", oracleQuery: "SELECT x'00FF' AS one, x'00FF' AS two", args: []any{sql.Named("v", []byte{0, 255})}},
		{name: "bound_storage_classes", query: "SELECT typeof(?1) AS n, typeof(?2) AS i, typeof(?3) AS r, typeof(?4) AS t, typeof(?5) AS b", oracleQuery: "SELECT typeof(NULL) AS n, typeof(1) AS i, typeof(1.0) AS r, typeof('x') AS t, typeof(x'') AS b", args: []any{nil, true, 1.0, "x", []byte{}}},
	}
	for _, size := range []int{512, 4096, 65536} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "readonly", fmt.Sprintf("page%d.db", size))
			defer confUnchanged(t, path)()
			confRun(t, path, cases)
		})
	}
}

func TestOracleScalarSemantics(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "readonly", "page4096.db")
	defer confUnchanged(t, path)()
	expressions := []string{
		"NULL, 0, 1, -1, 1.0, -0.0, '', x'', x'00ff80', '数据库🙂'",
		"1+2*3, (1+2)*3, 7/2, 7/2.0, -7%3, 5.5%2, 1/0, 1%0",
		"9223372036854775807+1, -9223372036854775808-1, 9223372036854775807*2, -9223372036854775808/-1",
		"9223372036854775807 = 9223372036854775808.0, 9007199254740993 > 9007199254740992.0, -9223372036854775808 = -9223372036854775808.0",
		"NULL AND 0, NULL AND 1, NULL OR 0, NULL OR 1, NOT NULL, NOT '0abc', NOT '2abc'",
		"NULL IS NULL, NULL IS NOT NULL, 1 IS TRUE, 2 IS TRUE, 0 IS FALSE, NULL IS FALSE, NULL IS TRUE, NULL IS NOT FALSE",
		"'1' = 1, '1' > 1, x'31' > '1', 'abc' || 2, 'a' || NULL",
		"'12abc'+3, '1e2'+1, '  -2.5x'+1, +'12', -'12', ~'12', 1 << 63, -1 >> 63, 1 << -1",
		"1e999, -1e999, 1e999-1e999, 0*1e999, 1e999/1e999",
		"abs(-1), abs(-1.5), abs('12'), abs('abc'), abs(NULL)",
		"length('数据库🙂é'), length(x'00ff80'), length(NULL), lower('ÄABC'), upper('äabc')",
		"typeof(NULL), typeof(1), typeof(1.0), typeof('1'), typeof(x'31'), typeof(1/0)",
		"hex(NULL), hex('数据库'), hex(x'00FF'), hex(123), hex(1.0)",
		"coalesce(NULL, NULL, 'yes'), ifnull(NULL,2), nullif(1,1), nullif('1',1), coalesce(1,abs(-9223372036854775808))",
		"'a_b' LIKE 'a!_b' ESCAPE '!', 'a%b' LIKE 'a!%b' ESCAPE '!', 'A' LIKE 'a', 'Ä' LIKE 'ä', '🙂' LIKE '_', '' LIKE '%', NULL LIKE '%', 'x' NOT LIKE '%'",
		"1_000 + 2, 0xffffffffffffffff, 0x8000000000000000",
	}
	var cases []confCase
	for i, e := range expressions {
		cases = append(cases, confCase{name: fmt.Sprintf("expressions_%02d", i), query: "SELECT " + e})
	}
	confRun(t, path, cases)
}

func confFresh(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ordinary.db")
	integrity := confCLI(t, path, `PRAGMA page_size=512;
CREATE TABLE empty_table(id INTEGER PRIMARY KEY, value TEXT);
CREATE TABLE edge(id INTEGER PRIMARY KEY, n NUMERIC, t TEXT, r REAL, b BLOB);
INSERT INTO edge VALUES(-1,NULL,NULL,NULL,NULL),(0,0,'0',0,x''),(1,1,'1',1,x'31'),(2,1.5,'1.5',1.5,x'0031ff'),(3,'abc','abc','abc','text'),(4,9223372036854775807,'9223372036854775807',9223372036854775807,x'ff');
CREATE TABLE shadow(id INTEGER PRIMARY KEY, rowid TEXT, _rowid_ TEXT, oid TEXT);
INSERT INTO shadow VALUES(7,'public rowid','public _rowid_','public oid');
CREATE TABLE changed(id INTEGER PRIMARY KEY);
INSERT INTO changed VALUES(1);
ALTER TABLE changed ADD COLUMN text_default TEXT DEFAULT 'hello';
ALTER TABLE changed ADD COLUMN numeric_default NUMERIC DEFAULT '12';
ALTER TABLE changed ADD COLUMN real_default REAL DEFAULT 7;
INSERT INTO changed VALUES(2,'world',3,4);
CREATE TABLE boolean_shadow("TRUE" INTEGER, "false" TEXT);
INSERT INTO boolean_shadow VALUES(9,'no');
CREATE TABLE nul_text(t TEXT);
INSERT INTO nul_text VALUES('ab'||char(0)||'cd');
PRAGMA integrity_check;`, false)
	if strings.TrimSpace(string(integrity)) != "'integrity_check'\n'ok'" {
		t.Fatalf("fresh ordinary fixture integrity failed: %s", integrity)
	}
	return path
}
func TestOracleOrdinaryFreshFixtures(t *testing.T) {
	path := confFresh(t)
	defer confUnchanged(t, path)()
	confRun(t, path, []confCase{
		{name: "empty_star", query: "SELECT * FROM empty_table", emptyColumns: []string{"id", "value"}},
		{name: "empty_sorted", query: "SELECT value AS label FROM empty_table ORDER BY label", emptyColumns: []string{"label"}},
		{name: "mixed_storage_classes", query: "SELECT id, n, t, r, b, typeof(n), typeof(t), typeof(r), typeof(b) FROM edge ORDER BY id"},
		{name: "storage_class_sort", query: "SELECT n, b FROM edge ORDER BY n, b"},
		{name: "numeric_compare", query: "SELECT id, n='1', n=1, n=t, t=1, t=1.0, r='1' FROM edge ORDER BY id"},
		{name: "unary_removes_affinity", query: "SELECT id, +n='1', +t=1, +r='1' FROM edge ORDER BY id"},
		{name: "alias_in_order_expression", query: "SELECT -id AS id FROM edge ORDER BY id+0"},
		{name: "alias_simple_order", query: "SELECT -id AS id FROM edge ORDER BY id"},
		{name: "alias_nonconflicting_order", query: "SELECT -id AS negative FROM edge ORDER BY negative+0"},
		{name: "where_alias", query: "SELECT id+1 AS plus_one FROM edge WHERE plus_one=2"},
		{name: "where_alias_shadow", query: "SELECT id+1 AS n FROM edge WHERE n=1 ORDER BY id"},
		{name: "boolean_column_shadow", query: "SELECT TRUE,false,TRUE IS TRUE,false IS FALSE FROM boolean_shadow"},
		{name: "rowid_shadow", query: "SELECT id,rowid,_rowid_,oid FROM shadow"},
		{name: "alter_added_defaults", query: "SELECT *,typeof(numeric_default),typeof(real_default) FROM changed ORDER BY id"},
		{name: "embedded_nul_text", query: "SELECT hex(t),length(t),t LIKE 'ab',t LIKE 'ab%' FROM nul_text"},
	})
}

func TestOracleErrorsAndResources(t *testing.T) {
	path := confFresh(t)
	defer confUnchanged(t, path)()
	db, err := sqlite.Open(path, sqlite.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{"SELECT missing FROM empty_table", "SELECT id FROM empty_table WHERE missing=0", "SELECT id FROM empty_table ORDER BY missing", "SELECT abs() FROM empty_table", "SELECT missing_function(id) FROM empty_table", "SELECT 1 LIMIT NULL", "SELECT 1 LIMIT 1.5", "SELECT 1 ORDER BY 2", "SELECT abs(-9223372036854775808)", "SELECT -0x8000000000000000"} {
		t.Run(q, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, confOracle(t), "-readonly", "-batch", "-bail", path, q)
			if out, err := cmd.CombinedOutput(); err == nil {
				t.Fatalf("error reference unexpectedly succeeded: %s", out)
			}
			r, err := db.Query(context.Background(), q)
			if err == nil {
				defer r.Close()
				for r.Next() {
				}
				err = r.Err()
			}
			if err == nil {
				t.Error("native query unexpectedly succeeded")
			}
		})
	}
	for _, q := range []string{"SELECT DISTINCT id FROM edge", "SELECT id FROM edge UNION SELECT id FROM edge", "SELECT count(*) FROM edge", "SELECT e.id FROM edge e JOIN edge f ON e.id=f.id", "SELECT id FROM edge; SELECT 1", "PRAGMA user_version", "WITH x AS (SELECT 1) SELECT * FROM x"} {
		t.Run("unsupported/"+q, func(t *testing.T) {
			r, err := db.Query(context.Background(), q)
			if r != nil {
				r.Close()
			}
			if !errors.Is(err, sqlite.ErrUnsupported) {
				t.Errorf("want explicit ErrUnsupported, got %v", err)
			}
		})
	}
	for _, q := range []string{"CREATE TABLE denied(id INTEGER)", "INSERT INTO edge(id) VALUES(99)", "UPDATE edge SET n=99", "DELETE FROM edge", "BEGIN"} {
		r, err := db.Query(context.Background(), q)
		if r != nil {
			r.Close()
		}
		if !errors.Is(err, sqlite.ErrReadOnly) {
			t.Errorf("%q: want ErrReadOnly, got %v", q, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := db.Query(ctx, "SELECT 1"); !errors.Is(err, context.Canceled) {
		t.Errorf("pre-canceled query: %v", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	rows, err := db.Query(ctx, "SELECT * FROM edge")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal("expected first row")
	}
	cancel()
	if rows.Next() || !errors.Is(rows.Err(), context.Canceled) {
		t.Errorf("iteration cancellation: %v", rows.Err())
	}
	rows.Close()
	rows.Close()
	rows, err = db.Query(context.Background(), "SELECT x'0102' AS payload")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal("expected BLOB row")
	}
	first := rows.Values()
	first[0].([]byte)[0] = 9
	if rows.Values()[0].([]byte)[0] != 1 {
		t.Error("Values BLOB aliases mutable internal data")
	}
	cols := rows.Columns()
	cols[0] = "changed"
	if rows.Columns()[0] != "payload" {
		t.Error("Columns aliases mutable internal data")
	}
	rows.Close()
	limited, err := sqlite.Open(path, sqlite.Options{ReadOnly: true, MaxSortBytes: 1})
	if err != nil {
		t.Fatal(err)
	}
	r, err := limited.Query(context.Background(), "SELECT * FROM edge ORDER BY id")
	if r != nil {
		r.Close()
	}
	limited.Close()
	if !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Errorf("sort quota: got %v", err)
	}
	rows, err = db.Query(context.Background(), "SELECT * FROM edge")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	db.Close()
	if rows.Next() || !errors.Is(rows.Err(), sqlite.ErrClosed) {
		t.Errorf("iterator after DB.Close: %v", rows.Err())
	}
	rows.Close()
	if _, err = db.Query(context.Background(), "SELECT 1"); !errors.Is(err, sqlite.ErrClosed) {
		t.Errorf("query after DB.Close: %v", err)
	}
}

func TestOracleDatabaseSQLAndConcurrency(t *testing.T) {
	confOracle(t)
	path := filepath.Join("..", "..", "testdata", "readonly", "page512.db")
	defer confUnchanged(t, path)()
	db, err := sql.Open("gosqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	if err = db.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	query := "SELECT id,label,payload,score,optional FROM samples WHERE id>=? AND id<=? ORDER BY id"
	stmt, err := db.PrepareContext(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	want := confExpected(t, path, "SELECT id,label,payload,score,optional FROM samples WHERE id>=0 AND id<=5 ORDER BY id")
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repeat := 0; repeat < 4; repeat++ {
				rows, err := stmt.QueryContext(context.Background(), 0, 5)
				if err != nil {
					t.Error(err)
					return
				}
				columns, err := rows.Columns()
				if err != nil {
					t.Error(err)
					rows.Close()
					return
				}
				got := confResult{columns: columns}
				for rows.Next() {
					v := make([]any, len(columns))
					dest := make([]any, len(columns))
					for i := range v {
						dest[i] = &v[i]
					}
					if err = rows.Scan(dest...); err != nil {
						t.Error(err)
						break
					}
					got.rows = append(got.rows, v)
				}
				if err = rows.Err(); err != nil {
					t.Error(err)
				}
				rows.Close()
				confCompare(t, want, got)
			}
		}()
	}
	wg.Wait()
	var id int64
	if err = db.QueryRowContext(context.Background(), "SELECT id FROM samples WHERE id=:id", sql.Named("id", 29)).Scan(&id); err != nil || id != 29 {
		t.Errorf("driver named parameter: %d %v", id, err)
	}
	if err = db.QueryRowContext(context.Background(), "SELECT id FROM samples WHERE id=17.5").Scan(&id); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("empty driver query: %v", err)
	}
	if _, err = db.ExecContext(context.Background(), "DELETE FROM samples"); !errors.Is(err, sqlite.ErrReadOnly) {
		t.Errorf("driver Exec: %v", err)
	}
	readTx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("read-only transaction: %v", err)
	}
	if _, err = readTx.ExecContext(context.Background(), "DELETE FROM samples"); !errors.Is(err, sqlite.ErrReadOnly) {
		t.Errorf("read-only transaction accepted write: %v", err)
	}
	if err = readTx.Rollback(); err != nil {
		t.Errorf("read-only rollback: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = db.QueryRowContext(ctx, "SELECT 1").Scan(&id); !errors.Is(err, context.Canceled) {
		t.Errorf("driver cancellation: %v", err)
	}
	var normalized any
	if err = db.QueryRowContext(context.Background(), "SELECT ?", math.NaN()).Scan(&normalized); err != nil || normalized != nil {
		t.Errorf("NaN parameter normalization: value=%v err=%v", normalized, err)
	}
}

func TestOracleSeededPredicates(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "readonly", "page4096.db")
	defer confUnchanged(t, path)()
	rng := rand.New(rand.NewSource(3540000))
	ops := []string{"=", "!=", "<", "<=", ">", ">="}
	bounds := []int64{-9223372036854775808, -140737488355329, -32769, -1, 0, 1, 127, 128, 32767, 2147483648, 9007199254740993, 9223372036854775807}
	var cases []confCase
	for i := 0; i < 80; i++ {
		if i%2 == 0 {
			lo, hi := rng.Intn(1200)-100, rng.Intn(1200)-100
			query := fmt.Sprintf("SELECT id, label FROM samples WHERE (id %s %d AND id %s %d) OR (optional IS NULL AND id %% 7 = %d) ORDER BY id DESC LIMIT %d OFFSET %d", ops[rng.Intn(len(ops))], lo, ops[rng.Intn(len(ops))], hi, rng.Intn(7), rng.Intn(20), rng.Intn(10))
			cases = append(cases, confCase{name: fmt.Sprintf("samples_%02d", i), query: query, emptyColumns: []string{"id", "label"}})
		} else {
			a, b := bounds[rng.Intn(len(bounds))], bounds[rng.Intn(len(bounds))]
			op1, op2 := ops[rng.Intn(len(ops))], ops[rng.Intn(len(ops))]
			query := fmt.Sprintf("SELECT value FROM numbers WHERE value %s ?1 OR NOT value %s ?2 ORDER BY value", op1, op2)
			reference := fmt.Sprintf("SELECT value FROM numbers WHERE value %s %d OR NOT value %s %d ORDER BY value", op1, a, op2, b)
			cases = append(cases, confCase{name: fmt.Sprintf("numbers_%02d", i), query: query, oracleQuery: reference, args: []any{a, b}, emptyColumns: []string{"value"}})
		}
	}
	confRun(t, path, cases)
}

func TestOracleSeededScalarExpressions(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "readonly", "page4096.db")
	defer confUnchanged(t, path)()
	rng := rand.New(rand.NewSource(0x354))
	values := []string{"NULL", "0", "1", "-1", "127", "-32769", "2147483648", "9223372036854775807", "-9223372036854775808", "1.5", "-2.25", "1e999", "'12'", "'1e2'", "'12abc'", "'abc'", "''", "x'3132'"}
	ops := []string{"+", "-", "*", "/", "%", "=", "!=", "<", ">=", "AND", "OR", "IS", "IS NOT", "||", "&", "|", "<<", ">>"}
	var cases []confCase
	for group := 0; group < 30; group++ {
		var exprs []string
		for col := 0; col < 10; col++ {
			a, b := values[rng.Intn(len(values))], values[rng.Intn(len(values))]
			op := ops[rng.Intn(len(ops))]
			exprs = append(exprs, fmt.Sprintf("(%s) %s (%s) AS v%d", a, op, b, col))
		}
		cases = append(cases, confCase{name: fmt.Sprintf("scalar_%02d", group), query: "SELECT " + strings.Join(exprs, ", ")})
	}
	confRun(t, path, cases)
}

func TestOracleResourceCloseCycles(t *testing.T) {
	confOracle(t)
	path := filepath.Join("..", "..", "testdata", "readonly", "page4096.db")
	defer confUnchanged(t, path)()
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	countTarget := func() int {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			return -1
		}
		n := 0
		for _, entry := range entries {
			target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
			if err == nil && target == absolute {
				n++
			}
		}
		return n
	}
	before := countTarget()
	for i := 0; i < 100; i++ {
		db, err := sqlite.Open(path, sqlite.Options{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		rows, err := db.Query(context.Background(), "SELECT payload FROM samples WHERE id>0")
		if err != nil {
			db.Close()
			t.Fatal(err)
		}
		if !rows.Next() {
			rows.Close()
			db.Close()
			t.Fatal("expected first row")
		}
		if err = rows.Close(); err != nil {
			t.Error(err)
		}
		if rows.Next() {
			t.Error("Next succeeds after Rows.Close")
		}
		if err = db.Close(); err != nil {
			t.Error(err)
		}
	}
	if after := countTarget(); before >= 0 && after != before {
		t.Errorf("read-only file descriptor leak: before=%d after=%d", before, after)
	}
}
