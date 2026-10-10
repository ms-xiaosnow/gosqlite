//go:build oracle

package testkit_test

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Bound parameters are real bindings in each engine. Oracle processes use only
// readonly SQL or temporary parameter storage and complete normally.
func TestBooleanAliasOnOracleProvenance(t *testing.T) {
	cli := confOracle(t)
	path := filepath.Join(t.TempDir(), "alias-on.db")
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE lhs(id INTEGER PRIMARY KEY,flag INTEGER,v INTEGER)`,
		`INSERT INTO lhs VALUES(1,0,-9223372036854775808),(2,1,1),(3,NULL,2)`,
		`CREATE TABLE rhs(y INTEGER)`, `INSERT INTO rhs VALUES(0),(1),(NULL)`,
		`CREATE TABLE empty(id INTEGER,flag INTEGER,v INTEGER)`,
		`CREATE TABLE physical(id INTEGER,true INTEGER,false INTEGER,z INTEGER)`,
		`INSERT INTO physical VALUES(1,0,1,0),(2,NULL,0,1)`,
	} {
		writeExec(t, db, q)
	}
	cases := 0
	compare := func(q string, arg any, binding string) {
		t.Helper()
		cases++
		cmd := exec.Command(cli, "-batch", "-bail", "-quote", "-noheader", "-readonly", path)
		cmd.Stdin = strings.NewReader(".parameter init\n.parameter set ?1 " + binding + "\n" + q + ";\n")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, wantErr := cmd.Output()
		var args []any
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
			t.Errorf("%s [?1=%s]: native %#v %v; SQLite %s %v", q, binding, got, gotErr, stderr.String(), wantErr)
			return
		}
		if wantErr != nil {
			for _, kind := range []string{"integer overflow", "no such column", "ambiguous column name"} {
				if strings.Contains(stderr.String(), kind) && !strings.Contains(fmt.Sprint(gotErr), kind) {
					t.Errorf("%s: %v vs %s", q, gotErr, stderr.String())
				}
			}
			return
		}
		want, err := confQuoted(out)
		if err != nil {
			t.Fatal(err)
		}
		if len(want) == 0 && len(got) == 0 {
			return
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s [?1=%s]: native %#v; SQLite %#v", q, binding, got, want)
		}
	}
	for _, source := range []string{"lhs", "empty"} {
		for _, alias := range []string{"z", "true", "false", "\"NULL\""} {
			for _, value := range []string{"0", "NULL", "TRUE", "FALSE", "?1", "flag", "rhs.y", "rhs.y IS NULL", "flag OR 1", "abs(-9223372036854775808)"} {
				for _, on := range []string{alias, "(" + alias + ")", "+" + alias, alias + "=0", alias + " OR 0", alias + " AND 1", "1 AND " + alias} {
					q := "SELECT l.id," + value + " AS " + alias + ",rhs.y FROM " + source + " l LEFT JOIN rhs ON " + on + " ORDER BY l.id,rhs.y"
					if value == "?1" {
						for _, p := range []struct {
							v any
							s string
						}{{int64(0), "0"}, {int64(1), "1"}, {nil, "NULL"}, {"0", "\"'0'\""}} {
							compare(q, p.v, p.s)
						}
					} else {
						compare(q, nil, "NULL")
					}
				}
			}
		}
	}
	for _, q := range []string{
		`SELECT l.id,FALSE,rhs.y FROM lhs l LEFT JOIN rhs ON FALSE ORDER BY l.id,rhs.y`,
		`SELECT l.id,NULL,rhs.y FROM lhs l LEFT JOIN rhs ON NULL ORDER BY l.id,rhs.y`,
		`SELECT l.id,TRUE,rhs.y FROM lhs l LEFT JOIN rhs ON TRUE ORDER BY l.id,rhs.y`,
		`SELECT l.id,0 AS z,rhs.y FROM lhs l LEFT JOIN rhs ON FALSE ORDER BY l.id,rhs.y`,
		`SELECT l.id,0 AS z,rhs.y FROM lhs l LEFT JOIN rhs ON z AND abs(-9223372036854775808)`,
		`SELECT l.id,0 AS z,rhs.y FROM lhs l LEFT JOIN rhs ON abs(-9223372036854775808) AND z`,
		`SELECT l.id,0 AS z,abs(-9223372036854775808) FROM lhs l LEFT JOIN rhs ON z`,
		`SELECT l.id,abs(-9223372036854775808) AS z FROM lhs l LEFT JOIN rhs ON z LIMIT 0`,
		`SELECT l.id,abs(-9223372036854775808) AS z FROM lhs l LEFT JOIN rhs ON z WHERE 0`,
		`SELECT l.id,0.0 AND abs(v) AS z,rhs.y FROM lhs l LEFT JOIN rhs ON z ORDER BY l.id,rhs.y`,
		`SELECT l.id,abs(v) OR 1 AS z,rhs.y FROM lhs l LEFT JOIN rhs ON z ORDER BY l.id,rhs.y`,
		`SELECT l.id,0 AS z,0 AS z,rhs.y FROM lhs l LEFT JOIN rhs ON z`,
		`SELECT l.id,FALSE,1 AS false,rhs.y FROM lhs l LEFT JOIN rhs ON false ORDER BY l.id,rhs.y`,
		`SELECT l.id,TRUE,0 AS true,rhs.y FROM lhs l LEFT JOIN rhs ON true ORDER BY l.id,rhs.y`,
		`SELECT p.id,1 AS true,0 AS false,1 AS z,rhs.y FROM physical p LEFT JOIN rhs ON true ORDER BY p.id,rhs.y`,
		`SELECT p.id,1 AS true,0 AS false,1 AS z,rhs.y FROM physical p LEFT JOIN rhs ON false ORDER BY p.id,rhs.y`,
		`SELECT p.id,1 AS true,0 AS false,1 AS z,rhs.y FROM physical p LEFT JOIN rhs ON z ORDER BY p.id,rhs.y`,
		`SELECT p.id,1 AS z,rhs.y FROM physical p LEFT JOIN rhs ON p.z ORDER BY p.id,rhs.y`,
		`SELECT l.id,0 AS z,a.y,b.y FROM lhs l LEFT JOIN rhs a ON abs(-9223372036854775808) LEFT JOIN rhs b ON z`,
		`SELECT l.id,0 AS z,a.y,b.y FROM lhs l LEFT JOIN rhs a ON z LEFT JOIN rhs b ON abs(-9223372036854775808)`,
	} {
		compare(q, nil, "NULL")
	}
	t.Logf("%d alias-root/operator/physical/parameter/empty-source/error provenance cases checked", cases)
}
