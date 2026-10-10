package driver_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	gosqldriver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// These are ordinary intact-file, SQL-error, transaction and pool lifecycles.
func TestDriverHalfBindingOwnershipTransactionsAndPool(t *testing.T) {
	ctx := context.Background()
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared%t", shared), func(t *testing.T) {
			connector, err := gosqldriver.NewConnector(filepath.Join(t.TempDir(), "half.db"), sqlite.Options{WALShared: shared})
			if err != nil {
				t.Fatal(err)
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
			exec("CREATE TABLE halves(id INTEGER PRIMARY KEY,h HALFVEC(4) DEFAULT '[1.00048828125,-0,5.960464477539063e-8,1.00146484375]',label TEXT)")
			h, err := vector.ParseHalf("[1.00048828125,-0,5.960464477539063e-8,1.00146484375]")
			if err != nil {
				t.Fatal(err)
			}
			want := []byte{'G', 'S', 'H', 0, 1, 2, 1, 0, 0, 0, 0, 4, 0x3c, 0, 0x80, 0, 0, 1, 0x3c, 2}
			blob, _ := h.MarshalBinary()
			if !bytes.Equal(blob, want) {
				t.Fatal(blob)
			}
			stmt, err := db.Prepare("INSERT INTO halves VALUES(?,?,?)")
			if err != nil {
				t.Fatal(err)
			}
			for i, value := range []any{h, vector.NullHalfVector{HalfVector: h, Valid: true}, vector.NullHalfVector{}, h.String(), blob, []byte(nil)} {
				if _, err = stmt.Exec(i+1, value, "bound"); err != nil {
					t.Fatal(err)
				}
			}
			if err = stmt.Close(); err != nil {
				t.Fatal(err)
			}
			for i := range blob {
				blob[i] = 0
			}
			exec("INSERT INTO halves(id,label) VALUES(7,'default')")
			verify := func(db *sql.DB) {
				t.Helper()
				rows, e := db.Query("SELECT id,h,typeof(h),halfvec_dims(h) FROM halves ORDER BY id")
				if e != nil {
					t.Fatal(e)
				}
				defer rows.Close()
				var n vector.NullHalfVector
				count := 0
				for rows.Next() {
					var id int64
					var storage string
					var dims sql.NullInt64
					if e = rows.Scan(&id, &n, &storage, &dims); e != nil {
						t.Fatal(e)
					}
					count++
					if id == 3 || id == 6 {
						if n.Valid || n.HalfVector.Valid() || storage != "null" || dims.Valid {
							t.Fatal(id, n, storage, dims)
						}
					} else {
						b, e := n.HalfVector.MarshalBinary()
						if e != nil || !n.Valid || !bytes.Equal(b, want) || storage != "blob" || dims.Int64 != 4 {
							t.Fatal(id, n, storage, dims, e)
						}
						// Mutating both accessors must not modify a retained scanned value.
						bits, values := n.HalfVector.Bits(), n.HalfVector.Values()
						bits[0], values[0] = 0, 0
						b, _ = n.HalfVector.MarshalBinary()
						if !bytes.Equal(b, want) {
							t.Fatal("scan alias")
						}
					}
				}
				if count != 7 || rows.Err() != nil {
					t.Fatal(count, rows.Err())
				}
			}
			verify(db)
			var n vector.NullHalfVector
			if err = db.QueryRow("SELECT h FROM halves WHERE id=1").Scan(&n); err != nil {
				t.Fatal(err)
			}
			for _, q := range []string{"SELECT vector('[1,2,3,4]')", "SELECT bit_vector('1010')", "SELECT 42", "SELECT x'5b315d'"} {
				if err = db.QueryRow(q).Scan(&n); err == nil {
					t.Fatal("cross-kind scan", q)
				}
				b, _ := n.HalfVector.MarshalBinary()
				if !n.Valid || !bytes.Equal(b, want) {
					t.Fatal("failed scan changed destination", q, n)
				}
			}
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			for _, q := range []string{"INSERT INTO halves VALUES(8,'[8,8,8,8]','prior')", "SAVEPOINT half", "UPDATE halves SET h='[2,2,2,2]' WHERE id=1"} {
				if _, err = tx.Exec(q); err != nil {
					t.Fatal(q, err)
				}
			}
			if _, err = tx.Exec("INSERT INTO halves VALUES(9,'[9,9,9,9]','partial'),(10,'[65520,0,0,0]','overflow')"); !errors.Is(err, vector.ErrNonFinite) {
				t.Fatal(err)
			}
			retained, err := tx.Query("SELECT id FROM halves WHERE id>=8 ORDER BY id")
			if err != nil {
				t.Fatal(err)
			}
			var ids []int64
			for retained.Next() {
				var id int64
				if err = retained.Scan(&id); err != nil {
					t.Fatal(err)
				}
				ids = append(ids, id)
			}
			if err = errors.Join(retained.Err(), retained.Close()); err != nil || !reflect.DeepEqual(ids, []int64{8}) {
				t.Fatal("statement undo", ids, err)
			}
			if _, err = tx.Exec("ROLLBACK TO half"); err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec("RELEASE half"); err != nil {
				t.Fatal(err)
			}
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			verify(db)
			for _, v := range []any{vector.HalfVector{}, "[1]", 1, []byte("[1,2,3,4]")} {
				if _, err = db.Exec("UPDATE halves SET h=? WHERE id=1", v); err == nil {
					t.Fatal("invalid binding accepted", v)
				}
			}
			// Returning a connection with an open SQL savepoint must reset its
			// half writes before the same pooled connection is handed out again.
			for _, finish := range []string{"abandon", "RELEASE root"} {
				conn, e := db.Conn(ctx)
				if e != nil {
					t.Fatal(e)
				}
				for _, q := range []string{"SAVEPOINT root", "INSERT INTO halves VALUES(20,'[20,20,20,20]','pooled')"} {
					if _, e = conn.ExecContext(ctx, q); e != nil {
						t.Fatal(q, e)
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
				e = db.QueryRow("SELECT h FROM halves WHERE id=20").Scan(&n)
				if finish == "abandon" {
					if !errors.Is(e, sql.ErrNoRows) {
						t.Fatal("pool leaked write", e)
					}
				} else {
					if e != nil || !reflect.DeepEqual(n.HalfVector.Values(), []float32{20, 20, 20, 20}) {
						t.Fatal(n, e)
					}
					exec("DELETE FROM halves WHERE id=20")
				}
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			reopened := sql.OpenDB(connector)
			reopened.SetMaxOpenConns(1)
			defer reopened.Close()
			verify(reopened)
		})
	}
}
