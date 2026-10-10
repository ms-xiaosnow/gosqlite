//go:build oracle

package testkit_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type affinityQuery struct {
	sql, binding string
	params       []any
}

// Quote-mode values preserve INTEGER/REAL and TEXT/BLOB distinctions. Markers
// separate empty results without relying on SQL truth or lossy JSON numbers.
// Every data query returns at least two columns, so a one-column marker cannot
// collide with fixture data. All CLI processes finish normally; no cancellation.
func affinityBatch(t *testing.T, db *sqlite.DB, path string, cases []affinityQuery) {
	t.Helper()
	var script strings.Builder
	script.WriteString(".parameter init\n")
	for i, c := range cases {
		fmt.Fprintf(&script, "SELECT 'affinity-case-%d';\n", i)
		if c.binding != "" {
			// Double-quote the shell argument, retaining the SQL literal's own
			// quotes. This is a real binding, not SQL-text substitution.
			fmt.Fprintf(&script, ".parameter set ?1 \"%s\"\n", c.binding)
		}
		script.WriteString(c.sql + ";\n")
	}
	cmd := exec.Command(confOracle(t), "-batch", "-bail", "-noheader", "-quote", "-readonly", path)
	cmd.Stdin = strings.NewReader(script.String())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("oracle matrix: %v: %s", err, stderr.String())
	}
	decoded, err := confQuoted(out)
	if err != nil {
		t.Fatal(err)
	}
	expected := make([][][]any, len(cases))
	index := -1
	for _, row := range decoded {
		if len(row) == 1 {
			s, ok := row[0].(string)
			if !ok || !strings.HasPrefix(s, "affinity-case-") {
				t.Fatalf("bad marker %#v", row)
			}
			n, err := strconv.Atoi(strings.TrimPrefix(s, "affinity-case-"))
			if err != nil || n != index+1 || n >= len(cases) {
				t.Fatalf("bad marker order %q", s)
			}
			index = n
			continue
		}
		if index < 0 {
			t.Fatal("data before marker")
		}
		expected[index] = append(expected[index], row)
	}
	if index+1 != len(cases) {
		t.Fatalf("oracle returned %d/%d cases", index+1, len(cases))
	}
	failed := 0
	for i, c := range cases {
		rows, err := db.Query(context.Background(), c.sql, c.params...)
		if err != nil {
			t.Errorf("%s params=%#v: %v", c.sql, c.params, err)
			failed++
			continue
		}
		var got [][]any
		for rows.Next() {
			got = append(got, rows.Values())
		}
		err = rows.Err()
		cerr := rows.Close()
		if err != nil || cerr != nil || !reflect.DeepEqual(got, expected[i]) {
			failed++
			t.Errorf("%s params=%#v\nnative=%#v errors=(%v,%v)\nSQLite=%#v", c.sql, c.params, got, err, cerr, expected[i])
		}
	}
	t.Logf("affinity complete-value/class matrix: %d queries, %d mismatches", len(cases), failed)
}

// Intact ordinary SQL-created files, read-only oracle queries, both journal
// choices, indexes and normal Close. No faults, disk changes or process exits.
func TestAffinityComparisonMatrixOracle(t *testing.T) {
	confOracle(t)
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "comparisons.db")
			db, err := sqlite.Open(path, sqlite.Options{WALShared: shared})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			writeExec(t, db, "CREATE TABLE f(id INTEGER PRIMARY KEY,t TEXT,b BLOB,u,n NUMERIC,i INTEGER,r REAL)")
			writeExec(t, db, "CREATE TABLE one(y INTEGER)")
			writeExec(t, db, "INSERT INTO one VALUES(1)")
			for i, v := range []string{"1", "1.0", "'1'", "'1.0'", "'01'", "NULL", "'x'", "X'31'", "0", "-1.5", "'1e0'", "9223372036854775807", "1.25"} {
				writeExec(t, db, fmt.Sprintf("INSERT INTO f VALUES(%d,%s)", i+1, strings.TrimSuffix(strings.Repeat(v+",", 6), ",")))
			}
			// An independent oracle reads native persisted storage classes first.
			values := "id,t,b,u,n,i,r,typeof(t),typeof(b),typeof(u),typeof(n),typeof(i),typeof(r)"
			writeSame(t, db, path, "SELECT "+values+" FROM f ORDER BY id")
			var cases []affinityQuery
			add := func(q string) { cases = append(cases, affinityQuery{sql: q}) }
			cols := []string{"t", "b", "u", "n", "i", "r"}
			rhs := []string{"t", "b", "u", "n", "i", "r", "+t", "+b", "+u", "CAST(b AS TEXT)", "CAST(b AS BLOB)", "CAST(t AS NUMERIC)", "coalesce(b,NULL)", "CASE WHEN 1 THEN u END", "1", "1.0", "'1'", "'1.0'", "NULL", "X'31'"}
			for _, a := range cols {
				for _, b := range rhs {
					var exprs []string
					for _, op := range []string{"=", "!=", "<", "<=", ">", ">=", "IS", "IS NOT"} {
						exprs = append(exprs, a+" "+op+" "+b)
					}
					exprs = append(exprs, a+" BETWEEN "+b+" AND "+b, a+" IN ("+b+")", "CASE "+a+" WHEN "+b+" THEN 1 ELSE 0 END")
					for _, e := range exprs {
						add("SELECT id," + e + ",typeof(" + e + ") FROM f ORDER BY id")
					}
				}
			}
			for _, a := range cols {
				for _, v := range []string{"1", "1.0", "'1'", "'1.0'", "NULL", "X'31'"} {
					for _, b := range cols {
						for _, e := range []string{a + "=" + b, b + "=" + a, a + "!=" + b, a + " IS " + b, a + "=" + "+" + b} {
							add("SELECT " + values + " FROM f WHERE " + a + "=" + v + " AND (" + e + ") ORDER BY id")
						}
					}
				}
			}
			for _, a := range []string{"t", "b", "u", "+t", "+b", "+u", "coalesce(b,NULL)", "CASE WHEN 1 THEN u END", "1.0"} {
				for _, b := range cols {
					add("SELECT id," + a + " AS alias,typeof(" + a + ") FROM f WHERE alias=" + b + " ORDER BY id")
					add("SELECT id," + a + " AS alias,typeof(" + a + ") FROM f WHERE " + b + "=alias ORDER BY id")
				}
			}
			for _, a := range cols {
				for _, b := range []string{"t", "b", "u", "+b"} {
					for _, join := range []string{"JOIN", "LEFT JOIN"} {
						add("SELECT f.id,one.y,t,b,u,typeof(b),typeof(u) FROM f " + join + " one ON " + a + "=" + b + " ORDER BY f.id")
					}
				}
			}
			for _, p := range []struct {
				value   any
				literal string
			}{{int64(1), "1"}, {float64(1), "1.0"}, {"1.0", "'1.0'"}, {[]byte("1"), "X'31'"}, {nil, "NULL"}} {
				for _, a := range cols {
					for _, e := range []string{a + "=?1", a + "=?1 AND " + a + "=t", a + "=?1 AND t=" + a, "t=?1 AND t=" + a} {
						q := "SELECT " + values + ",?1,typeof(?1) FROM f WHERE " + e + " ORDER BY id"
						cases = append(cases, affinityQuery{q, p.literal, []any{p.value}})
					}
				}
			}
			affinityBatch(t, db, path, cases)
			// Repeat predicates/aliases/parameters after index creation. Projection-only
			// expressions were already covered; no planner/index-order compatibility claim.
			for _, c := range cols {
				writeExec(t, db, "CREATE INDEX idx_"+c+" ON f("+c+")")
			}
			var indexed []affinityQuery
			for _, c := range cases {
				if strings.Contains(c.sql, " WHERE ") {
					indexed = append(indexed, c)
				}
			}
			affinityBatch(t, db, path, indexed)
			writeIntegrity(t, path)
		})
	}
}

func TestAffinityComparisonWritesOracle(t *testing.T) {
	confOracle(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "native.db")
	ref := filepath.Join(dir, "reference.db")
	db := writeOpen(t, path, 4096)
	setup := []string{"CREATE TABLE f(id INTEGER PRIMARY KEY,t TEXT,b BLOB,u,changed INTEGER)", "INSERT INTO f VALUES(1,1,1,1,0),(2,1.0,1.0,1.0,0),(3,'1','1','1',0),(4,'1.0','1.0','1.0',0),(5,NULL,NULL,NULL,0)", "CREATE INDEX ft ON f(t)", "CREATE INDEX fb ON f(b)"}
	for _, q := range setup {
		writeExec(t, db, q)
		confCLI(t, ref, q+";", false)
	}
	result := "SELECT id,t,b,u,changed,typeof(t),typeof(b),typeof(u) FROM f ORDER BY id"
	for _, predicate := range []string{"t=b", "b=t", "t=u", "u=t", "t=+b", "b=1 AND t=b", "b=1 AND b!=t", "t IS b", "t BETWEEN b AND b", "t IN(b)"} {
		tx, err := db.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		q := "UPDATE f SET changed=1 WHERE " + predicate
		r := writeTxExec(t, tx, q)
		affected, err := r.RowsAffected()
		if err != nil {
			t.Fatal(err)
		}
		// Commit and restore with ordinary SQL, so the read-only oracle observes the
		// actual native mutations and classes rather than merely unchanged fixtures.
		if err := tx.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
		confCLI(t, ref, q+";", false)
		writeSame(t, db, ref, result)
		nativeChanged := confNative(t, db, "SELECT id,changed FROM f WHERE changed=1 ORDER BY id")
		if int64(len(nativeChanged.rows)) != affected {
			t.Fatalf("affected=%d changed rows=%#v", affected, nativeChanged)
		}
		writeExec(t, db, "UPDATE f SET changed=0")
		confCLI(t, ref, "UPDATE f SET changed=0;", false)
	}
	for _, predicate := range []string{"t=b", "t!=b", "t=u", "t=+u"} {
		tx, err := db.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		q := "DELETE FROM f WHERE " + predicate
		writeTxExec(t, tx, q)
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		writeSame(t, db, ref, result)
	}
	writeIntegrity(t, path)
}
