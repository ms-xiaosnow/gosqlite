//go:build oracle

package btree

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
)

// The CLI is test-only. It creates intact ordinary databases using legal SQL.
// No file faults, file-byte mutation, process termination or crash paths run.
func boundedOracleSQL(t *testing.T, cli, path, sql string) string {
	t.Helper()
	cmd := exec.Command(cli, "-batch", "-bail", path)
	cmd.Stdin = strings.NewReader(sql)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("SQLite 3.54 oracle: %v\n%s", err, out)
	}
	return strings.TrimSuffix(string(out), "\n")
}
func boundedLiteral(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		if math.IsNaN(x) {
			return "NULL"
		}
		if math.IsInf(x, 1) {
			return "1e999"
		}
		if math.IsInf(x, -1) {
			return "-1e999"
		}
		s := strconv.FormatFloat(x, 'g', -1, 64)
		if !strings.ContainsAny(s, ".eE") {
			s += ".0"
		}
		return s
	case string:
		return "CAST(X'" + hex.EncodeToString([]byte(x)) + "' AS TEXT)"
	case []byte:
		return "X'" + hex.EncodeToString(x) + "'"
	}
	panic("invalid oracle literal")
}

// A NULL-safe CASE implements SQLite B-tree prefix comparison independently
// using the official engine's own IS and storage-class < operators. Row-value
// SQL comparison is unsuitable here because its NULL result is three-valued.
func boundedOracleCompare(key []any) string {
	expr := "0"
	names := []string{"a", "b", "rowid"}
	for i := len(key) - 1; i >= 0; i-- {
		col, v := names[i], boundedLiteral(key[i])
		expr = "CASE WHEN " + col + " IS " + v + " THEN (" + expr + ") WHEN " + col + " IS NULL THEN -1 WHEN " + v + " IS NULL THEN 1 WHEN " + col + " < " + v + " THEN -1 ELSE 1 END"
	}
	return "(" + expr + ")"
}
func boundedOraclePredicate(lo, hi *IndexBound) string {
	var parts []string
	if lo != nil {
		op := ">"
		if lo.Inclusive {
			op = ">="
		}
		parts = append(parts, boundedOracleCompare(lo.Key)+op+"0")
	}
	if hi != nil {
		op := "<"
		if hi.Inclusive {
			op = "<="
		}
		parts = append(parts, boundedOracleCompare(hi.Key)+op+"0")
	}
	if len(parts) == 0 {
		return "1"
	}
	return strings.Join(parts, " AND ")
}
func boundedIDs(keys [][]any) string {
	ids := make([]string, len(keys))
	for i, key := range keys {
		ids[i] = strconv.FormatInt(key[len(key)-1].(int64), 10)
	}
	return strings.Join(ids, ",")
}

func TestBoundedSQLite354Oracle(t *testing.T) {
	cli := os.Getenv("SQLITE_ORACLE")
	if cli == "" {
		t.Fatal("oracle acceptance requires explicit SQLITE_ORACLE pointing to official SQLite 3.54 CLI")
	}
	version := boundedOracleSQL(t, cli, ":memory:", "SELECT sqlite_version(); SELECT sqlite_source_id();")
	if !strings.HasPrefix(version, "3.54.0\n") {
		t.Fatalf("wrong oracle: %s", version)
	}
	t.Log("oracle", version)
	values := []any{nil, math.Inf(-1), int64(math.MinInt64), int64(-9007199254740993), -0.5, int64(0), float64(0), int64(1), float64(9007199254740992), int64(9007199254740993), int64(math.MaxInt64), math.Inf(1), "", "a", "a\x00", "数据库", []byte{}, []byte{0}, []byte{255}, strings.Repeat("overflow\x00", 160)}
	var keys [][]any
	var insert strings.Builder
	for i := 0; i < 1800; i++ {
		id := int64(i - 900)
		if i == 0 {
			id = math.MinInt64
		}
		if i == 1799 {
			id = math.MaxInt64
		}
		a := values[i%len(values)]
		b := []any{nil, int64(i % 5), float64(i % 5), fmt.Sprintf("b%02d", i%11), []byte{byte(i % 13)}}[i%5]
		keys = append(keys, []any{a, b, id})
		fmt.Fprintf(&insert, "INSERT INTO t(rowid,a,b) VALUES(%d,%s,%s);\n", id, boundedLiteral(a), boundedLiteral(b))
	}
	type interval struct {
		lo, hi  *IndexBound
		reverse bool
	}
	cases := []interval{{nil, nil, false}, {nil, nil, true}}
	random := rand.New(rand.NewSource(3540000))
	bounds := []*IndexBound{nil}
	for _, v := range values {
		for _, inc := range []bool{false, true} {
			bounds = append(bounds, &IndexBound{[]any{v}, inc})
		}
	}
	for i := 0; i < 180; i++ {
		key := keys[random.Intn(len(keys))]
		n := 1 + random.Intn(3)
		bounds = append(bounds, &IndexBound{key[:n], random.Intn(2) == 0})
	}
	for n := 0; n < 140; n++ {
		lo, hi := bounds[random.Intn(len(bounds))], bounds[random.Intn(len(bounds))]
		cases = append(cases, interval{lo, hi, false}, interval{lo, hi, true})
	}
	for _, size := range []int{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ordinary.db")
			boundedOracleSQL(t, cli, path, fmt.Sprintf("PRAGMA page_size=%d; CREATE TABLE t(a,b); CREATE INDEX ix ON t(a,b); BEGIN;\n%sCOMMIT;", size, insert.String()))
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			before := sha256.Sum256(raw)
			var query strings.Builder
			for _, tc := range cases {
				order := "a,b,rowid"
				if tc.reverse {
					order = "a DESC,b DESC,rowid DESC"
				}
				fmt.Fprintf(&query, "SELECT coalesce(group_concat(rowid,','),'') FROM (SELECT rowid FROM t WHERE %s ORDER BY %s);\n", boundedOraclePredicate(tc.lo, tc.hi), order)
			}
			expected := strings.Split(boundedOracleSQL(t, cli, path, query.String()), "\n")
			if len(expected) != len(cases) {
				t.Fatal("oracle rows", len(expected), len(cases))
			}
			r, err := pager.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			indexRoot, tableRoot := schemaRoot(t, r, "ix"), schemaRoot(t, r, "t")
			for n, tc := range cases {
				tracked := &boundedCountReader{PageReader: r}
				c, err := NewBoundedIndexCursor(context.Background(), tracked, indexRoot, KeyInfo{2}, tc.lo, tc.hi, tc.reverse, RangeLimits{})
				if err != nil {
					t.Fatalf("case %d: %v", n, err)
				}
				got := boundedIDs(collectBoundedIndex(t, c))
				if got != expected[n] {
					t.Fatalf("case %d reverse=%v lower=%#v upper=%#v got %s want %s", n, tc.reverse, tc.lo, tc.hi, got, expected[n])
				}
				if c.Stats().PageReads != int64(len(tracked.reads)) {
					t.Fatal("unreported read", c.Stats(), len(tracked.reads))
				}
			}
			// Measured selective traversal: same file, exact reader calls, no cache
			// substitutions. Pick a full key near the end for each direction.
			full := &boundedCountReader{PageReader: r}
			scan, err := NewIndexCursor(context.Background(), full, indexRoot, KeyInfo{2})
			if err != nil {
				t.Fatal(err)
			}
			var ordered [][]any
			for {
				key, err := scan.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				ordered = append(ordered, key)
			}
			scan.Close()
			for _, rev := range []bool{false, true} {
				at := len(ordered) - 7
				if rev {
					at = 6
				}
				bound := &IndexBound{ordered[at], true}
				tracked := &boundedCountReader{PageReader: r}
				c, err := NewBoundedIndexCursor(context.Background(), tracked, indexRoot, KeyInfo{2}, bound, bound, rev, RangeLimits{})
				if err != nil {
					t.Fatal(err)
				}
				got := collectBoundedIndex(t, c)
				if !reflect.DeepEqual(got, ordered[at:at+1]) {
					t.Fatal("selective singleton")
				}
				if len(tracked.reads) >= len(full.reads) {
					t.Fatalf("range read %d complete scan %d", len(tracked.reads), len(full.reads))
				}
				t.Logf("index page_size=%d reverse=%v singleton_reads=%d full_scan_reads=%d exact_trace=%v", size, rev, len(tracked.reads), len(full.reads), tracked.reads)
			}
			tableFull := &boundedCountReader{PageReader: r}
			if err := Scan(context.Background(), tableFull, tableRoot, func(int64, []any) error { return nil }); err != nil {
				t.Fatal(err)
			}
			for _, rev := range []bool{false, true} {
				tracked := &boundedCountReader{PageReader: r}
				lo, hi := &RowIDBound{int64(887), true}, &RowIDBound{int64(889), true}
				c, err := NewRowIDRangeCursor(context.Background(), tracked, tableRoot, lo, hi, rev, RangeLimits{})
				if err != nil {
					t.Fatal(err)
				}
				var got []int64
				for {
					id, _, err := c.Next()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					got = append(got, id)
				}
				c.Close()
				want := []int64{887, 888, 889}
				if rev {
					want = []int64{889, 888, 887}
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatal(got, want)
				}
				if c.Stats().PageReads != int64(len(tracked.reads)) || len(tracked.reads) >= len(tableFull.reads) {
					t.Fatal(c.Stats(), tracked.reads, len(tableFull.reads))
				}
				t.Logf("rowid page_size=%d reverse=%v three_row_reads=%d full_scan_reads=%d exact_trace=%v", size, rev, len(tracked.reads), len(tableFull.reads), tracked.reads)
			}
			if err = r.Close(); err != nil {
				t.Fatal(err)
			}
			raw, err = os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if sha256.Sum256(raw) != before {
				t.Fatal("range changed database bytes")
			}
			// Reopen normally and read an extreme key after all prior cursors close.
			r, err = pager.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			c, err := NewRowIDRangeCursor(context.Background(), r, tableRoot, &RowIDBound{math.MaxInt64, true}, nil, true, RangeLimits{})
			if err != nil {
				t.Fatal(err)
			}
			id, _, err := c.Next()
			if err != nil || id != math.MaxInt64 {
				t.Fatal(id, err)
			}
			c.Close()
			if err = r.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
