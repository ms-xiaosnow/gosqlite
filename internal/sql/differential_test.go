//go:build oracle

package sql

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ms-xiaosnow/gosqlite/internal/buildinfo"
	"io"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// The official CLI is an opt-in test oracle only. Production has no dependency
// on it. Set SQLITE_ORACLE to the independently built official SQLite executable.
func TestDiffSQLiteExpressions(t *testing.T) {
	oracle := os.Getenv("SQLITE_ORACLE")
	if oracle == "" {
		t.Fatal("SQLITE_ORACLE must identify the pinned official executable")
	}
	expressions := []string{"9223372036854775807=9223372036854775808.0", "-9223372036854775808=-9223372036854775808.0", "9223372036854775807+1", "-9223372036854775808/-1", "abs(-2.5)", "abs('12')", "abs('abc')", "abs(X'3132')", "'1e2'%7", "'1e2'|0", "~'1e2'", "NULL IS FALSE", "NULL IS NOT TRUE", "2 IS TRUE", "2 IS +TRUE", "1=2<3", "NOT 1=2", "0xFFFFFFFFFFFFFFFF", "1_2_3+1", "'12foo'+0", "'1e0'+0", "length('a中🦀')", "lower('AbÉ')", "hex(1.2345678901234567)", "hex(1e16)", "hex(0.1)", "hex(1e-05)", "hex(-0.0)", "X'61' LIKE 'A'", "1e999-1e999", "coalesce(NULL,1,2)", "nullif(1,1)", "ifnull(NULL,3)"}
	r := rand.New(rand.NewSource(3540000))
	atoms := []string{"NULL", "0", "1", "2", "-3", "1.25", "-0.5", "9223372036854775807", "-9223372036854775808", "'2'", "'junk'", "'12.5foo'", "'1e2'", "'1.'", "'1e'", "'1e+'", "'1.e'", "' .5'", "'0x12'", "X'32'", "'中'"}
	ops := []string{"+", "-", "*", "/", "%", "=", "!=", "<", "<=", ">", ">=", "IS", "IS NOT", "AND", "OR", "&", "|", "<<", ">>", "||"}
	var randomExpr func(int) string
	randomExpr = func(d int) string {
		if d == 0 || r.Intn(3) == 0 {
			return atoms[r.Intn(len(atoms))]
		}
		return "(" + randomExpr(d-1) + " " + ops[r.Intn(len(ops))] + " " + randomExpr(d-1) + ")"
	}
	for i := 0; i < 4000; i++ {
		f := math.Float64frombits(r.Uint64())
		if !math.IsNaN(f) && !math.IsInf(f, 0) {
			expressions = append(expressions, "hex("+strconv.FormatFloat(f, 'e', -1, 64)+")")
		}
	}
	for _, f := range []float64{math.SmallestNonzeroFloat64, math.Float64frombits(0x0010000000000000), math.MaxFloat64, 0.1, 0.3, 1e16, 1e17, 111111111111111.125, 49.47} {
		expressions = append(expressions, "hex("+strconv.FormatFloat(f, 'e', -1, 64)+")")
	}
	for i := 0; i < 600; i++ {
		expressions = append(expressions, randomExpr(3))
	}
	alphabet := []rune("abAB%_中é🦀!")
	q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	randomText := func() string {
		n := r.Intn(9)
		var b strings.Builder
		for j := 0; j < n; j++ {
			b.WriteRune(alphabet[r.Intn(len(alphabet))])
		}
		return b.String()
	}
	for i := 0; i < 500; i++ {
		s := q(randomText()) + " LIKE " + q(randomText())
		if i%2 == 0 {
			s += " ESCAPE '!'"
		}
		expressions = append(expressions, s)
	}
	runDiff(t, oracle, expressions, "", nil)
}
func TestDiffSQLiteAffinity(t *testing.T) {
	oracle := os.Getenv("SQLITE_ORACLE")
	if oracle == "" {
		t.Fatal("SQLITE_ORACLE must identify the pinned official executable")
	}
	ctx := &EvalContext{Row: map[string]any{"i": int64(12), "t": "12", "r": float64(12), "b": []byte("12")}, Affinity: func(_, name string) string {
		switch name {
		case "i":
			return "INTEGER"
		case "t":
			return "TEXT"
		case "r":
			return "REAL"
		}
		return "BLOB"
	}}
	exprs := []string{"i='12'", "i='12.0'", "i='012'", "i=' 12 '", "i='12junk'", "t=12", "t=12.0", "r='12'", "i=t", "i=b", "t=b", "+i='12'", "i=+t", "t<'13'", "b>X'30'", "i IS '12'", "i IS NOT '12'", "i IS TRUE", "r+t", "t||r", "hex(r)"}
	runDiff(t, oracle, exprs, "CREATE TABLE tab(i INTEGER,t TEXT,r REAL,b BLOB); INSERT INTO tab VALUES(12,'12',12,X'3132');", ctx)
}
func runDiff(t *testing.T, oracle string, exprs []string, prefix string, ctx *EvalContext) {
	t.Helper()
	version, err := exec.Command(oracle, "-batch", "-bail", ":memory:", "SELECT sqlite_version() || '|' || sqlite_source_id();").CombinedOutput()
	if err != nil || strings.TrimSpace(string(version)) != buildinfo.SQLiteBaseline+"|"+buildinfo.SQLiteSourceID {
		t.Fatalf("oracle version/source mismatch: %s (%v)", version, err)
	}
	var script strings.Builder
	script.WriteString(prefix)
	for i, e := range exprs {
		from := ""
		if prefix != "" {
			from = " FROM tab"
		}
		fmt.Fprintf(&script, "SELECT %d AS id, typeof((%s)) AS t, CASE WHEN typeof((%s)) IN ('text','blob') THEN hex((%s)) ELSE quote((%s)) END AS v%s;\n", i, e, e, e, e, from)
	}
	cmd := exec.Command(oracle, "-json", ":memory:")
	cmd.Stdin = strings.NewReader(script.String())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("oracle: %v: %s", err, stderr.String())
	}
	decoder := json.NewDecoder(bytes.NewReader(out))
	count := 0
	for {
		var rows []struct {
			ID    int    `json:"id"`
			Type  string `json:"t"`
			Value string `json:"v"`
		}
		err := decoder.Decode(&rows)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			count++
			e, err := ParseExpr(exprs[row.ID])
			if err != nil {
				t.Fatalf("parse %s: %v", exprs[row.ID], err)
			}
			got, err := Eval(e, ctx)
			if err != nil {
				t.Fatalf("eval %s: %v", exprs[row.ID], err)
			}
			if err := sameOracle(got, row.Type, row.Value); err != nil {
				t.Errorf("%s => %T(%v), SQLite %s(%s): %v", exprs[row.ID], got, got, row.Type, row.Value, err)
			}
		}
	}
	if count != len(exprs) {
		t.Fatalf("oracle returned %d/%d rows", count, len(exprs))
	}
	t.Logf("%d scalar/LIKE cases matched official SQLite", count)
}
func sameOracle(got any, kind, repr string) error {
	switch kind {
	case "null":
		if got == nil {
			return nil
		}
	case "integer":
		n, e := strconv.ParseInt(repr, 10, 64)
		if e != nil {
			return e
		}
		if v, ok := got.(int64); ok && v == n {
			return nil
		}
	case "real":
		f, e := strconv.ParseFloat(repr, 64)
		if e != nil && !math.IsInf(f, 0) {
			return e
		}
		if v, ok := got.(float64); ok && (v == f || math.IsInf(v, 1) && math.IsInf(f, 1) || math.IsInf(v, -1) && math.IsInf(f, -1)) {
			return nil
		}
	case "text":
		b, e := hex.DecodeString(repr)
		if e != nil {
			return e
		}
		if v, ok := got.(string); ok && v == string(b) {
			return nil
		}
	case "blob":
		b, e := hex.DecodeString(repr)
		if e != nil {
			return e
		}
		if v, ok := got.([]byte); ok && bytes.Equal(v, b) {
			return nil
		}
	}
	return fmt.Errorf("value or storage class mismatch")
}
