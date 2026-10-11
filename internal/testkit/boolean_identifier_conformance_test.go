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

// Read-only pinned SQLite oracle on an intact native SQL-created file. No child
// cancellation or process signaling; every invocation waits for normal EOF.
func TestBooleanIdentifierOracleMatrix(t *testing.T) {
	cli := confOracle(t)
	path := filepath.Join(t.TempDir(), "boolean-identifiers.db")
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE t(id INTEGER PRIMARY KEY,v INTEGER)`,
		`INSERT INTO t VALUES(1,-9223372036854775808),(2,0),(3,2)`,
		`CREATE TABLE one(y INTEGER)`, `INSERT INTO one VALUES(7)`,
		`CREATE TABLE true(id INTEGER PRIMARY KEY,TrUe INTEGER,FaLsE INTEGER,"NULL" TEXT)`,
		`INSERT INTO true VALUES(1,0,1,'n'),(2,2,0,NULL),(3,NULL,NULL,'x')`,
		`CREATE TABLE false(id INTEGER PRIMARY KEY,true INTEGER,false INTEGER)`,
		`INSERT INTO false VALUES(1,7,0),(2,0,8)`,
		`CREATE INDEX truth_ix ON true(true,false)`,
	} {
		writeExec(t, db, q)
	}
	cases := 0
	compare := func(q string) {
		t.Helper()
		cases++
		cmd := testoracle.ForTest(t, cli, "-batch", "-bail", "-quote", "-header", "-readonly", path)
		cmd.Stdin = strings.NewReader(q + ";\n")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, wantErr := cmd.Output()
		rows, gotErr := db.Query(context.Background(), q)
		var got confResult
		if gotErr == nil {
			got.columns = rows.Columns()
			for rows.Next() {
				got.rows = append(got.rows, rows.Values())
			}
			gotErr = rows.Err()
			rows.Close()
		}
		if (gotErr == nil) != (wantErr == nil) {
			t.Errorf("%s: native %v %#v; SQLite %v %s", q, gotErr, got, wantErr, stderr.String())
			return
		}
		if wantErr != nil {
			expected := stderr.String()
			actual := fmt.Sprint(gotErr)
			for _, kind := range []string{"integer overflow", "no such column", "ambiguous column name", "no such function"} {
				if strings.Contains(expected, kind) && !strings.Contains(actual, kind) && !(kind == "no such function" && strings.Contains(actual, "unsupported SQL: function")) {
					t.Errorf("%s: native error %v; SQLite %s", q, gotErr, expected)
				}
			}
			return
		}
		raw, err := confQuoted(output)
		if err != nil {
			t.Fatal(err)
		}
		var want confResult
		if len(raw) > 0 {
			for _, c := range raw[0] {
				want.columns = append(want.columns, c.(string))
			}
			want.rows = raw[1:]
		}
		if want.columns != nil && !reflect.DeepEqual(got.columns, want.columns) {
			t.Errorf("%s: labels %#v; SQLite %#v", q, got.columns, want.columns)
		}
		if len(want.rows) == 0 && len(got.rows) == 0 {
			return
		}
		if !reflect.DeepEqual(got.rows, want.rows) {
			t.Errorf("%s: native %#v; SQLite %#v", q, got.rows, want.rows)
		}
	}
	for _, name := range []string{"true", "false"} {
		for _, alias := range []string{name, strings.ToUpper(name), `"` + name + `"`, "[" + name + "]", "`" + name + "`"} {
			for _, value := range []string{"0", "1", "2", "NULL", "-id"} {
				for _, as := range []string{" AS ", " "} {
					result := value + as + alias
					for _, source := range []string{"t", "true"} {
						for _, condition := range []string{name, "NOT " + name, name + " IS TRUE", name + " IS FALSE", name + " IS NULL", name + " IS +TRUE"} {
							compare("SELECT id," + result + " FROM " + source + " WHERE " + condition + " ORDER BY id")
						}
						compare("SELECT id," + result + " FROM " + source + " ORDER BY " + name + ",id")
						compare("SELECT id," + result + " FROM " + source + " ORDER BY +" + name + ",id")
					}
					for _, join := range []string{" JOIN ", " LEFT JOIN "} {
						compare("SELECT t.id," + result + ",one.y FROM t" + join + "one ON " + name + " ORDER BY t.id")
						compare("SELECT true.id," + result + ",one.y FROM true" + join + "one ON " + name + " ORDER BY true.id")
					}
				}
			}
		}
	}
	for _, q := range []string{
		`SELECT 0 AS true FROM t WHERE true AND abs(t.v)`,
		`SELECT 1 AS false FROM t WHERE false OR abs(t.v)`,
		`SELECT 0 AS true,true,1 AS false,false,NULL`,
		`SELECT NULL AS true FROM t WHERE true OR abs(v)`,
		`SELECT true,false,"true",[false],"NULL" FROM true ORDER BY id`,
		`SELECT true.true,false.false FROM true AS true JOIN false AS false ON true.id=false.id ORDER BY true.id`,
		`SELECT true.*,false.* FROM true true LEFT JOIN false false ON true.id=false.id ORDER BY true.id`,
		`SELECT true.true,true.false FROM true AS true NOT INDEXED WHERE true.true IS FALSE ORDER BY true.id`,
		`SELECT true,false FROM true,false LIMIT 0`,
		`SELECT true FROM true,false LIMIT 0`,
		`SELECT true.true FROM false`,
		`SELECT true(),false(NULL)`, `SELECT TrUe(1) FROM true LIMIT 0`,
		`SELECT false(abs(v)) FROM t WHERE 0`,
		`SELECT TRUE IS TRUE,FALSE IS FALSE,NULL IS TRUE,NULL IS FALSE,2 IS TRUE,2 IS +TRUE`,
		`SELECT CASE true WHEN false THEN NULL ELSE true END FROM true ORDER BY id`,
		`SELECT CASE WHEN true THEN false ELSE NULL END FROM true ORDER BY id`,
		`SELECT true BETWEEN false AND 2,true IN(false,NULL) FROM true ORDER BY id`,
		`SELECT typeof(TRUE),typeof(FALSE),typeof(NULL),typeof('TRUE')`,
		`SELECT 'true' AS true FROM one WHERE true`,
		`SELECT 0 AS "NULL",NULL FROM one WHERE NULL IS NULL`,
	} {
		compare(q)
	}
	t.Logf("%d Boolean identifier values/classes/labels/error cases checked against pinned SQLite", cases)
}

func TestBooleanIdentifierOracleDML(t *testing.T) {
	cli := confOracle(t)
	path := filepath.Join(t.TempDir(), "boolean-dml.db")
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	script := []string{
		`CREATE TABLE true(id INTEGER PRIMARY KEY,true INTEGER DEFAULT TRUE,false INTEGER DEFAULT FALSE,"NULL" TEXT)`,
		`CREATE TABLE false(id INTEGER PRIMARY KEY,true INTEGER,false INTEGER)`,
		`CREATE INDEX truth ON true(true,false)`,
		`INSERT INTO true(id) VALUES(1),(2)`,
		`INSERT INTO true VALUES(3,NULL,2,'NULL')`,
		`INSERT INTO false(id,true,false) VALUES(1,TRUE,FALSE)`,
		`UPDATE true SET true=false+2,false=true WHERE id=2`,
		`DELETE FROM true WHERE false=TRUE AND id=2`,
	}
	for _, q := range script {
		writeExec(t, db, q)
	}
	query := `SELECT id,true,false,"NULL",typeof(true),typeof(false) FROM true ORDER BY id`
	cmd := testoracle.ForTest(t, cli, "-batch", "-bail", "-quote", "-header", ":memory:", strings.Join(script, ";")+";"+query+";")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oracle DML %v: %s", err, out)
	}
	raw, err := confQuoted(out)
	if err != nil {
		t.Fatal(err)
	}
	var want confResult
	for _, v := range raw[0] {
		want.columns = append(want.columns, v.(string))
	}
	want.rows = raw[1:]
	confCompare(t, want, confNative(t, db, query))
	confCompare(t, confExpected(t, path, query), confNative(t, db, query))
	if out := strings.TrimSpace(string(confCLI(t, path, "PRAGMA integrity_check;", true))); out != "'integrity_check'\n'ok'" {
		t.Fatalf("integrity_check: %s", out)
	}
}
