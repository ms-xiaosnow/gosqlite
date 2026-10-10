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

// SQLite checks only its supported scalar predicates. Native dense ranking has
// tied distances so its rowid tie order can be compared with ordinary ORDER BY
// id. Every fixture uses normal SQL; the pinned readonly CLI exits at EOF.
func TestBooleanRankFilterOracle(t *testing.T) {
	cli := confOracle(t)
	dbpath := filepath.Join(t.TempDir(), "rank_filter.db")
	db, err := sqlite.Open(dbpath, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		"CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2) NOT NULL,active INTEGER,n INTEGER)",
		"INSERT INTO items VALUES(1,'[1,1]',0,-9223372036854775808),(2,'[1,1]',1,7)",
	} {
		writeExec(t, db, q)
	}
	if err := db.CreateHNSWIndex(context.Background(), "knn", "items", "v"); err != nil {
		t.Fatal(err)
	}
	const boom = "abs(-9223372036854775808)"
	cases := 0
	for _, where := range []string{
		"0", "NULL", "1", "?1", "active", "active AND abs(n)",
		"0 AND " + boom, boom + " AND 0", "1 OR " + boom, boom + " OR 1",
		"active AND " + boom, "active=99 AND " + boom,
		"active=1 AND active IS TRUE", "active IN (NULL,1)",
		"active BETWEEN 0 AND 1", "active IS NOT FALSE", "NOT (active OR NULL)",
		"CASE WHEN active AND abs(n) THEN 1 ELSE 0 END",
		"active=99 AND active LIKE 'x' ESCAPE 'ab'",
	} {
		for _, arg := range []struct {
			value any
			sql   string
		}{{int64(0), "0"}, {int64(1), "1"}, {nil, "NULL"}} {
			if !strings.Contains(where, "?1") && arg.sql != "0" {
				continue
			}
			scalar := "SELECT id FROM items NOT INDEXED WHERE " + where + " ORDER BY id LIMIT 2"
			cmd := exec.Command(cli, "-batch", "-bail", "-noheader", "-quote", "-readonly", dbpath)
			cmd.Stdin = strings.NewReader(".parameter init\n.parameter set ?1 " + arg.sql + "\n" + scalar + ";\n")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, wantErr := cmd.Output()
			var want [][]any
			if wantErr == nil {
				want, err = confQuoted(out)
				if err != nil {
					t.Fatal(err)
				}
			}
			args := []any(nil)
			if strings.Contains(where, "?1") {
				args = []any{arg.value}
			}
			check := func(label string, got [][]any, gotErr error) {
				t.Helper()
				if (gotErr != nil) != (wantErr != nil) {
					t.Fatalf("%s WHERE %s [%s]: got %#v, %v; SQLite %v %s", label, where, arg.sql, got, gotErr, wantErr, stderr.String())
				}
				if wantErr != nil {
					for _, kind := range []string{"integer overflow", "ESCAPE expression"} {
						if strings.Contains(stderr.String(), kind) && !strings.Contains(fmt.Sprint(gotErr), kind) {
							t.Fatalf("%s %s: %v; SQLite %s", label, where, gotErr, stderr.String())
						}
					}
					return
				}
				if len(got) == 0 && len(want) == 0 {
					return
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%s WHERE %s [%s]: got %#v; SQLite %#v", label, where, arg.sql, got, want)
				}
			}
			for _, modifier := range []string{"", " NOT INDEXED"} {
				q := "SELECT id FROM items" + modifier + " WHERE " + where + " ORDER BY v <-> '[1,1]' LIMIT 2"
				rows, gotErr := db.Query(context.Background(), q, args...)
				var got [][]any
				if gotErr == nil {
					for rows.Next() {
						got = append(got, rows.Values())
					}
					gotErr = rows.Err()
					rows.Close()
				}
				check("SQL"+modifier, got, gotErr)
			}
			for _, mode := range []sqlite.VectorSearchMode{sqlite.VectorModeExact, sqlite.VectorModeApproximate} {
				result, gotErr := db.VectorSearch(context.Background(), "items", "v", "[1,1]", sqlite.VectorSearchOptions{Mode: mode, Limit: 2, Where: where, Args: args})
				var got [][]any
				for _, hit := range result.Hits {
					got = append(got, []any{hit.RowID})
				}
				check("public "+string(mode), got, gotErr)
			}
			cases++
		}
	}
	t.Logf("%d predicates checked through exact SQL, ANN SQL, exact public and ANN public ranking against SQLite", cases)
}
