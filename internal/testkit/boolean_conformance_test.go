//go:build oracle

package testkit_test

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/testoracle"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Pinned CLI only, intact ordinary SQL fixtures, normal process completion.
// Both values/storage classes and error presence are compared. Parameters use
// real bindings: substituting a literal would change the behavior being tested.
func TestBooleanOracleContexts(t *testing.T) {
	cli := confOracle(t)
	path := filepath.Join(t.TempDir(), "boolean.db")
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		"CREATE TABLE t(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER)",
		"CREATE TABLE u(y INTEGER)", "CREATE TABLE empty(flag INTEGER,v INTEGER)",
		"CREATE INDEX bool_flag ON t(flag)",
		"INSERT INTO t VALUES(1,0,-9223372036854775808)", "INSERT INTO u VALUES(2)",
	} {
		writeExec(t, db, q)
	}
	const boom = "abs(-9223372036854775808)"
	expressions := []string{}
	for _, flag := range []string{"0", "1", "2", "false", "true", "0.0", "1.0", "+0", "-1", "2147483648", "0_0", "1_0", "0x0", "0x1", "(1 IS NULL)", "(1 NOTNULL)", "(NULL IS NULL)", "(NULL IN ())", "'0'", "'1'", "NULL", "?1", "flag"} {
		for _, op := range []string{"AND", "OR"} {
			expressions = append(expressions, flag+" "+op+" "+boom, boom+" "+op+" "+flag)
		}
	}
	expressions = append(expressions,
		"flag AND abs(v)", "abs(v) AND flag", "flag OR abs(v)", "abs(v) OR flag",
		"(flag AND abs(v))=0", "(flag AND abs(v)) IS FALSE", "(flag AND abs(v)) IS NOT TRUE",
		"NOT (NULL OR "+boom+")", "NOT(NULL AND "+boom+")",
		"(NULL AND "+boom+") IS TRUE", "(NULL AND "+boom+") IS NOT TRUE",
		"flag BETWEEN 1 AND "+boom, "flag NOT BETWEEN 1 AND "+boom,
		"NULL BETWEEN 1 AND "+boom, "NULL NOT BETWEEN 1 AND "+boom,
		"1/0 AND "+boom, "1/0 OR "+boom,
		"0 AND ('x' LIKE 'x' ESCAPE 'ab')", "1 OR ('x' LIKE 'x' ESCAPE 'ab')",
		"flag AND (flag LIKE 'x' ESCAPE 'ab')", "(flag LIKE 'x' ESCAPE 'ab') AND flag",
		"flag=99 AND "+boom, "flag=99 AND (flag LIKE 'x' ESCAPE 'ab')",
		"0 AND missing", "0 AND abs(missing)", "1 OR missing", "0 AND absent()", "1 OR abs()",
		"CASE WHEN flag AND abs(v) THEN 7 ELSE 9 END", "CASE flag WHEN flag AND abs(v) THEN 7 ELSE 9 END",
	)
	templates := []string{
		"SELECT %s FROM t", "SELECT id FROM t WHERE %s", "SELECT id FROM t NOT INDEXED WHERE %s",
		"SELECT CASE WHEN %s THEN 7 ELSE 9 END FROM t",
		"SELECT t.id FROM t JOIN u ON %s", "SELECT t.id,u.y FROM t LEFT JOIN u ON %s",
		"SELECT t.id FROM t,u WHERE %s", "SELECT %s AS z FROM t WHERE z",
	}
	cases := 0
	compare := func(q string, arg any, binding string) {
		t.Helper()
		cmd := testoracle.ForTest(t, cli, "-batch", "-bail", "-noheader", "-quote", "-readonly", path)
		cmd.Stdin = strings.NewReader(".parameter init\n.parameter set ?1 " + binding + "\n" + q + ";\n")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, wantErr := cmd.Output()
		args := []any(nil)
		if strings.Contains(q, "?1") {
			args = []any{arg}
		}
		rows, gotErr := db.Query(context.Background(), q, args...)
		var got [][]any
		if gotErr == nil {
			for rows.Next() {
				got = append(got, rows.Values())
			}
			gotErr = rows.Err()
			rows.Close()
		}
		if (gotErr == nil) != (wantErr == nil) {
			t.Errorf("%s [?1=%s]: native=%v rows=%#v; SQLite=%v %s", q, binding, gotErr, got, wantErr, stderr.String())
			return
		}
		if wantErr != nil {
			for _, kind := range []string{"integer overflow", "no such column", "no such function", "wrong number", "ESCAPE expression"} {
				if strings.Contains(stderr.String(), kind) && !strings.Contains(fmt.Sprint(gotErr), kind) && !(kind == "no such function" && strings.Contains(fmt.Sprint(gotErr), "unsupported SQL: function")) {
					t.Errorf("%s: native error %v; SQLite %s", q, gotErr, stderr.String())
				}
			}
			return
		}
		want, err := confQuoted(output)
		if err != nil {
			t.Fatal(err)
		}
		if len(want) == 0 && len(got) == 0 {
			return
		}
		if !reflect.DeepEqual(want, got) {
			t.Errorf("%s [?1=%s]: got %#v; SQLite %#v", q, binding, got, want)
		}
	}
	for _, expression := range expressions {
		for _, template := range templates {
			q := fmt.Sprintf(template, expression)
			for _, p := range []struct {
				v    any
				text string
			}{{int64(0), "0"}, {int64(1), "1"}, {nil, "NULL"}} {
				if p.text != "0" && !strings.Contains(q, "?1") {
					continue
				}
				compare(q, p.v, p.text)
				cases++
			}
		}
	}
	for _, q := range []string{
		"SELECT flag FROM empty WHERE flag AND " + boom,
		"SELECT flag FROM empty WHERE 0 AND " + boom,
		"SELECT flag FROM empty WHERE " + boom + " AND 0",
		"SELECT " + boom + " FROM empty",
		"SELECT t.id FROM empty JOIN t ON " + boom,
		"SELECT t.id FROM t LEFT JOIN empty ON " + boom,
		"SELECT t.id FROM t LEFT JOIN empty ON 0 AND " + boom,
		"SELECT 0 AS z FROM t WHERE z AND " + boom,
		"SELECT 1 AS z FROM t WHERE " + boom + " OR z",
		"SELECT flag AS z FROM t WHERE z=99 AND z LIKE 'x' ESCAPE 'ab'",
		"SELECT t.flag AS z FROM t,u WHERE z=99 AND z LIKE 'x' ESCAPE 'ab'",
		"SELECT t.id FROM t WHERE " + boom + " LIMIT 0",
		"SELECT t.id FROM t,u WHERE " + boom + " LIMIT 0",
	} {
		compare(q, nil, "NULL")
		cases++
	}
	t.Logf("%d Boolean context/error/value cases checked against pinned SQLite", cases)
}
