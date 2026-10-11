//go:build oracle

package testkit_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/ms-xiaosnow/gosqlite/internal/testoracle"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Historical independent matrix retained unchanged, except the test name.
// Intact ordinary SQL databases and normally exiting readonly CLI only.
// Value-focused constant propagation independently exercises all affinities,
// aliases, transitivity, contradictory constraints and projection isolation.
func TestAffinityInheritedMatrixOracle(t *testing.T) {
	cli := confOracle(t)
	path := filepath.Join(t.TempDir(), "affinity.db")
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		"CREATE TABLE vals(id INTEGER PRIMARY KEY,n NUMERIC,i INTEGER,r REAL,t TEXT,b BLOB,u)",
		"CREATE TABLE one(y INTEGER)", "INSERT INTO one VALUES(1)",
		"INSERT INTO vals VALUES(1,'01','01','01','01','01','01'),(2,1.0,1.0,1.0,1.0,1.0,1.0),(3,NULL,NULL,NULL,NULL,NULL,NULL),(4,'x','x','x','x','x','x'),(5,x'31',x'31',x'31',x'31',x'31',x'31')",
	} {
		writeExec(t, db, q)
	}
	cases, failed := 0, 0
	compare := func(q string) {
		t.Helper()
		cases++
		cmd := testoracle.ForTest(t, cli, "-batch", "-bail", "-noheader", "-quote", "-readonly", path, q)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, werr := cmd.Output()
		rows, gerr := db.Query(context.Background(), q)
		var got [][]any
		if gerr == nil {
			for rows.Next() {
				got = append(got, rows.Values())
			}
			gerr = rows.Err()
			rows.Close()
		}
		if (gerr == nil) != (werr == nil) {
			failed++
			t.Errorf("SQL %s\nnative=%v %#v\nSQLite=%v %s", q, gerr, got, werr, stderr.String())
			return
		}
		if werr != nil {
			for _, kind := range []string{"integer overflow", "ESCAPE expression", "no such column", "wrong number"} {
				if strings.Contains(stderr.String(), kind) && !strings.Contains(fmt.Sprint(gerr), kind) {
					failed++
					t.Errorf("SQL %s: native=%v SQLite=%s", q, gerr, stderr.String())
				}
			}
			return
		}
		want, e := confQuoted(out)
		if e != nil {
			t.Fatal(e)
		}
		if len(want) == 0 && len(got) == 0 {
			return
		}
		if !reflect.DeepEqual(got, want) {
			failed++
			t.Errorf("SQL %s\nnative=%#v\nSQLite=%#v", q, got, want)
		}
	}
	for _, column := range []string{"n", "i", "r", "t", "b", "u"} {
		for _, value := range []string{"1", "1.0", "'01'", "'1'", "x'31'", "NULL", "0", "'x'"} {
			for _, probe := range []string{"typeof(C)='integer'", "typeof(C)='real'", "typeof(C)='text'", "C LIKE '1'", "C LIKE '01'", "C LIKE '1.0'", "hex(C)='31'", "C=t", "t=C", "C=b", "b=C", "C=+t", "+t=C", "C IS t", "t IS C", "C != t", "C NOT BETWEEN 0 AND 2", "C IN (t,b)", "CASE WHEN {c} THEN 1 ELSE 0 END"} {
				if strings.Contains(probe, "{c}") {
					probe = strings.ReplaceAll(probe, "{c}", column)
				} else {
					probe = strings.ReplaceAll(probe, "C", column)
				}
				where := column + "=" + value + " AND (" + probe + ")"
				for _, from := range []string{"vals", "vals,one"} {
					compare("SELECT id," + column + ",typeof(" + column + ") FROM " + from + " WHERE " + where + " ORDER BY id")
				}
			}
			for _, where := range []string{column + "=" + value + " AND " + column + "=1", column + "=1 AND " + column + "=" + value,
				column + "=" + value + " AND n=" + column + " AND t=n", column + "=" + value + " AND n=" + column + " AND n LIKE '01'"} {
				compare("SELECT id," + column + " FROM vals WHERE " + where + " ORDER BY id")
			}
			for _, where := range []string{"alias=" + value + " AND alias LIKE '01'", "alias=" + value + " AND alias=" + column, "alias=" + value + " AND " + column + " LIKE '01'"} {
				compare("SELECT id," + column + " AS alias FROM vals WHERE " + where + " ORDER BY id")
			}
		}
	}
	t.Logf("independent affinity: %d cases, %d mismatches", cases, failed)
}
