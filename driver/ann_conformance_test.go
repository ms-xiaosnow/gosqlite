package driver_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	gosqldriver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestDriverHNSWPreparedBindingsAndRollback(t *testing.T) {
	ctx := context.Background()
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared%t", shared), func(t *testing.T) {
			connector, err := gosqldriver.NewConnector(filepath.Join(t.TempDir(), "ann.db"), sqlite.Options{WALShared: shared})
			if err != nil {
				t.Fatal(err)
			}
			db := sql.OpenDB(connector)
			defer db.Close()
			db.SetMaxOpenConns(1)
			if _, err = db.ExecContext(ctx, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2) NOT NULL)"); err != nil {
				t.Fatal(err)
			}
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			for i := 1; i <= 64; i++ {
				if _, err = tx.ExecContext(ctx, "INSERT INTO items VALUES(?,?)", i, fmt.Sprintf("[%d,1]", i)); err != nil {
					t.Fatal(err)
				}
			}
			if err = tx.Commit(); err != nil {
				t.Fatal(err)
			}
			if _, err = db.ExecContext(ctx, "CREATE INDEX items_hnsw ON items(v) USING hnsw WITH(m=16,ef_construction=64)"); err != nil {
				t.Fatal(err)
			}
			q, err := vector.New([]float32{20, 1})
			if err != nil {
				t.Fatal(err)
			}
			stmt, err := db.PrepareContext(ctx, "SELECT id,l2_distance(v,?) AS distance FROM items ORDER BY distance LIMIT ?")
			if err != nil {
				t.Fatal(err)
			}
			defer stmt.Close()
			read := func(rows *sql.Rows, err error) []int64 {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
				defer rows.Close()
				var ids []int64
				for rows.Next() {
					var id int64
					var d float64
					if e := rows.Scan(&id, &d); e != nil {
						t.Fatal(e)
					}
					ids = append(ids, id)
				}
				if e := rows.Err(); e != nil {
					t.Fatal(e)
				}
				return ids
			}
			if got := read(stmt.QueryContext(ctx, q, 3)); !reflect.DeepEqual(got, []int64{20, 19, 21}) {
				t.Fatal(got)
			}
			var detail string
			var node, parent, unused int64
			if err = db.QueryRowContext(ctx, "EXPLAIN QUERY PLAN SELECT id FROM items ORDER BY v <-> ? LIMIT 3", q).Scan(&node, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(detail, "USING items_hnsw hnsw v1") || !strings.Contains(detail, "exact=false") {
				t.Fatal(detail)
			}
			tx, err = db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tx.ExecContext(ctx, "UPDATE items SET v='[20,1]' WHERE id=1"); err != nil {
				t.Fatal(err)
			}
			if got := read(tx.QueryContext(ctx, "SELECT id,v <-> ? AS distance FROM items ORDER BY distance LIMIT 1", q)); !reflect.DeepEqual(got, []int64{1}) {
				t.Fatal(got)
			}
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			if got := read(stmt.QueryContext(ctx, q, 3)); !reflect.DeepEqual(got, []int64{20, 19, 21}) {
				t.Fatal(got)
			}
			if _, err = db.ExecContext(ctx, "REINDEX items_hnsw"); err != nil {
				t.Fatal(err)
			}
			if got := read(stmt.QueryContext(ctx, q, 3)); !reflect.DeepEqual(got, []int64{20, 19, 21}) {
				t.Fatal(got)
			}
		})
	}
}
