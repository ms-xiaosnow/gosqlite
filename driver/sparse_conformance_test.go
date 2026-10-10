package driver_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	gosqldriver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestDriverSparseBindingOwnershipTransactionsAndPool(t *testing.T) {
	ctx := context.Background()
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared%t", shared), func(t *testing.T) {
			connector, e := gosqldriver.NewConnector(filepath.Join(t.TempDir(), "sparse.db"), sqlite.Options{WALShared: shared})
			if e != nil {
				t.Fatal(e)
			}
			db := sql.OpenDB(connector)
			db.SetMaxOpenConns(1)
			defer db.Close()
			exec := func(q string, args ...any) {
				t.Helper()
				if _, e := db.Exec(q, args...); e != nil {
					t.Fatal(q, e)
				}
			}
			text := "{1:1,999999999:-2,1000000000:1e-45}/1000000000"
			exec("CREATE TABLE items(id INTEGER PRIMARY KEY,s SPARSEVEC(1000000000) DEFAULT '" + text + "',label TEXT)")
			v, e := vector.ParseSparse(text)
			if e != nil {
				t.Fatal(e)
			}
			want, _ := v.MarshalBinary()
			blob := append([]byte(nil), want...)
			stmt, e := db.Prepare("INSERT INTO items VALUES(?,?,?)")
			if e != nil {
				t.Fatal(e)
			}
			for i, value := range []any{v, vector.NullSparseVector{SparseVector: v, Valid: true}, vector.NullSparseVector{}, v.String(), blob, []byte(nil)} {
				if _, e = stmt.Exec(i+1, value, "bound"); e != nil {
					t.Fatal(e)
				}
			}
			if e = stmt.Close(); e != nil {
				t.Fatal(e)
			}
			for i := range blob {
				blob[i] = 0
			}
			exec("INSERT INTO items(id,label) VALUES(7,'default')")
			verify := func(db *sql.DB) {
				t.Helper()
				rows, e := db.Query("SELECT id,s,typeof(s),sparsevec_dims(s),sparsevec_nnz(s) FROM items ORDER BY id")
				if e != nil {
					t.Fatal(e)
				}
				defer rows.Close()
				var n vector.NullSparseVector
				count := 0
				for rows.Next() {
					var id int64
					var typ string
					var dims, nnz sql.NullInt64
					if e = rows.Scan(&id, &n, &typ, &dims, &nnz); e != nil {
						t.Fatal(e)
					}
					count++
					if id == 3 || id == 6 {
						if n.Valid || typ != "null" || dims.Valid || nnz.Valid {
							t.Fatal(id, n, typ, dims, nnz)
						}
					} else {
						b, e := n.SparseVector.MarshalBinary()
						if e != nil || !n.Valid || !bytes.Equal(b, want) || typ != "blob" || dims.Int64 != 1000000000 || nnz.Int64 != 3 {
							t.Fatal(id, n, typ, dims, nnz, e)
						}
						entries := n.SparseVector.Entries()
						entries[0].Value = 9
						b, _ = n.SparseVector.MarshalBinary()
						if !bytes.Equal(b, want) {
							t.Fatal("scan aliases accessor")
						}
					}
				}
				if count != 7 || rows.Err() != nil {
					t.Fatal(count, rows.Err())
				}
			}
			verify(db)
			n := vector.NullSparseVector{SparseVector: v, Valid: true}
			for _, q := range []string{"SELECT vector('[1]')", "SELECT halfvec('[1]')", "SELECT bit_vector('1')", "SELECT 42", "SELECT X'7b7d2f31'"} {
				if e = db.QueryRow(q).Scan(&n); e == nil {
					t.Fatal(q)
				}
				b, _ := n.SparseVector.MarshalBinary()
				if !n.Valid || !bytes.Equal(b, want) {
					t.Fatal("failed scan changed destination")
				}
			}
			tx, e := db.BeginTx(ctx, nil)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback()
			for _, q := range []string{"INSERT INTO items VALUES(8,'{1:8}/1000000000','prior')", "SAVEPOINT sparse", "UPDATE items SET s='{2:4}/1000000000' WHERE id=1"} {
				if _, e = tx.Exec(q); e != nil {
					t.Fatal(e)
				}
			}
			if _, e = tx.Exec("INSERT INTO items VALUES(9,'{1:9}/1000000000','partial'),(10,'{1:NaN}/1000000000','invalid')"); !errors.Is(e, vector.ErrNonFinite) {
				t.Fatal(e)
			}
			retained, e := tx.Query("SELECT id FROM items WHERE id>=8 ORDER BY id")
			if e != nil {
				t.Fatal(e)
			}
			var ids []int64
			for retained.Next() {
				var id int64
				if e = retained.Scan(&id); e != nil {
					t.Fatal(e)
				}
				ids = append(ids, id)
			}
			if e = errors.Join(retained.Err(), retained.Close()); e != nil || len(ids) != 1 || ids[0] != 8 {
				t.Fatal(ids, e)
			}
			for _, q := range []string{"ROLLBACK TO sparse", "RELEASE sparse"} {
				if _, e = tx.Exec(q); e != nil {
					t.Fatal(e)
				}
			}
			if e = tx.Rollback(); e != nil {
				t.Fatal(e)
			}
			verify(db)
			for _, value := range []any{vector.SparseVector{}, "{1:1}/1", 1, []byte(text)} {
				if _, e = db.Exec("UPDATE items SET s=? WHERE id=1", value); e == nil {
					t.Fatal(value)
				}
			}
			for _, finish := range []string{"abandon", "RELEASE root"} {
				conn, e := db.Conn(ctx)
				if e != nil {
					t.Fatal(e)
				}
				for _, q := range []string{"SAVEPOINT root", "INSERT INTO items VALUES(20,'{1:20}/1000000000','pooled')"} {
					if _, e = conn.ExecContext(ctx, q); e != nil {
						t.Fatal(e)
					}
				}
				if finish != "abandon" {
					if _, e = conn.ExecContext(ctx, finish); e != nil {
						t.Fatal(e)
					}
				}
				if e = conn.Close(); e != nil {
					t.Fatal(e)
				}
				e = db.QueryRow("SELECT s FROM items WHERE id=20").Scan(&n)
				if finish == "abandon" {
					if !errors.Is(e, sql.ErrNoRows) {
						t.Fatal(e)
					}
				} else {
					if e != nil || n.SparseVector.String() != "{1:20}/1000000000" {
						t.Fatal(n, e)
					}
					exec("DELETE FROM items WHERE id=20")
				}
			}
			if e = db.Close(); e != nil {
				t.Fatal(e)
			}
			reopened := sql.OpenDB(connector)
			reopened.SetMaxOpenConns(1)
			defer reopened.Close()
			verify(reopened)
		})
	}
}
