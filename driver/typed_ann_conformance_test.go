package driver_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	gosqldriver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Only ordinary intact file transactions, snapshots, and close/reopen.
func TestDriverTypedANNPreparedSavepointReopen(t *testing.T) {
	for _, shared := range []bool{false, true} {
		for _, kind := range []string{"halfvec", "bit", "sparsevec"} {
			for _, method := range []string{"hnsw", "ivfflat"} {
				if kind == "sparsevec" && method == "ivfflat" {
					continue
				}
				t.Run(fmt.Sprintf("%s/%s/shared%t", kind, method, shared), func(t *testing.T) {
					ctx := context.Background()
					path := filepath.Join(t.TempDir(), "typed.db")
					open := func() *sql.DB {
						connector, e := gosqldriver.NewConnector(path, sqlite.Options{WALShared: shared})
						if e != nil {
							t.Fatal(e)
						}
						db := sql.OpenDB(connector)
						db.SetMaxOpenConns(1)
						return db
					}
					db := open()
					defer func() { db.Close() }()
					exec := func(q string, args ...any) {
						t.Helper()
						if _, e := db.ExecContext(ctx, q, args...); e != nil {
							t.Fatal(q, e)
						}
					}
					declaration, op, opclass := "HALFVEC(2)", "<->", "halfvec_l2_ops"
					if kind == "bit" {
						declaration, op, opclass = "BIT(16)", "<~>", "bit_hamming_ops"
					}
					if kind == "sparsevec" {
						declaration, opclass = "SPARSEVEC(1000000000)", "sparsevec_l2_ops"
					}
					value := func(i int) any {
						switch kind {
						case "halfvec":
							v, e := vector.NewHalf([]float32{float32(i), 1})
							if e != nil {
								t.Fatal(e)
							}
							return v
						case "bit":
							v, e := vector.ParseBit(fmt.Sprintf("%016b", i))
							if e != nil {
								t.Fatal(e)
							}
							return v
						default:
							v, e := vector.NewSparse(1000000000, []vector.SparseEntry{{Index: 0, Value: float32(i)}, {Index: 999999999, Value: 1}})
							if e != nil {
								t.Fatal(e)
							}
							return v
						}
					}
					exec("CREATE TABLE items(id INTEGER PRIMARY KEY,v " + declaration + " NOT NULL)")
					insert, e := db.PrepareContext(ctx, "INSERT INTO items VALUES(?,?)")
					if e != nil {
						t.Fatal(e)
					}
					for i := 1; i <= 32; i++ {
						if _, e = insert.ExecContext(ctx, i, value(i)); e != nil {
							t.Fatal(e)
						}
					}
					insert.Close()
					options := "opclass='" + opclass + "'"
					if method == "ivfflat" {
						options += ",lists=2"
					}
					exec("CREATE INDEX typed_idx ON items(v) USING " + method + " WITH(" + options + ")")
					query := "SELECT id,v " + op + " ? AS distance FROM items ORDER BY distance LIMIT ?"
					verify := func() {
						t.Helper()
						stmt, e := db.PrepareContext(ctx, query)
						if e != nil {
							t.Fatal(e)
						}
						defer stmt.Close()
						rows, e := stmt.QueryContext(ctx, value(1), 3)
						if e != nil {
							t.Fatal(e)
						}
						defer rows.Close()
						count := 0
						for rows.Next() {
							var id int64
							var d float64
							if e = rows.Scan(&id, &d); e != nil {
								t.Fatal(e)
							}
							if count == 0 && (id != 1 || d != 0) {
								t.Fatal(id, d)
							}
							count++
						}
						if rows.Err() != nil || count != 3 {
							t.Fatal(count, rows.Err())
						}
						rows.Close()
						plan, e := db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+query, value(1), 3)
						if e != nil {
							t.Fatal(e)
						}
						defer plan.Close()
						seen := false
						for plan.Next() {
							var a, b, c int64
							var detail string
							if e = plan.Scan(&a, &b, &c, &detail); e != nil {
								t.Fatal(e)
							}
							if strings.Contains(detail, "USING typed_idx "+method+" v2") && strings.Contains(detail, "approximate_recall=true") {
								seen = true
							}
						}
						if !seen || plan.Err() != nil {
							t.Fatal("missing real typed ANN plan", plan.Err())
						}
					}
					verify()
					tx, e := db.BeginTx(ctx, nil)
					if e != nil {
						t.Fatal(e)
					}
					for _, q := range []string{"SAVEPOINT owned", "DELETE FROM items WHERE id=1", "ROLLBACK TO owned", "RELEASE owned"} {
						if _, e = tx.ExecContext(ctx, q); e != nil {
							t.Fatal(e)
						}
					}
					if _, e = tx.ExecContext(ctx, "UPDATE items SET v=? WHERE id=2", value(30)); e != nil {
						t.Fatal(e)
					}
					if e = tx.Rollback(); e != nil {
						t.Fatal(e)
					}
					verify()
					exec("REINDEX typed_idx")
					if e = db.Close(); e != nil {
						t.Fatal(e)
					}
					db = open()
					verify()
				})
			}
		}
	}
}
