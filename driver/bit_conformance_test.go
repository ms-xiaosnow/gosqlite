package driver_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	gosqldriver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestDriverBitBindingsTransactionsAndReopen(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "rollback", true: "shared"}[shared], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bits.db")
			connector, err := gosqldriver.NewConnector(path, sqlite.Options{WALShared: shared})
			if err != nil {
				t.Fatal(err)
			}
			db := sql.OpenDB(connector)
			db.SetMaxOpenConns(1)
			defer db.Close()
			if _, err = db.Exec("CREATE TABLE bits(id INTEGER PRIMARY KEY,b BIT(9),label TEXT)"); err != nil {
				t.Fatal(err)
			}
			v, err := vector.ParseBit("001010001")
			if err != nil {
				t.Fatal(err)
			}
			blob, _ := v.MarshalBinary()
			stmt, err := db.Prepare("INSERT INTO bits VALUES(?,?,?)")
			if err != nil {
				t.Fatal(err)
			}
			for i, value := range []any{v, vector.NullBitVector{BitVector: v, Valid: true}, vector.NullBitVector{}, "001010001", blob, []byte(nil)} {
				if _, err = stmt.Exec(i+1, value, "source"); err != nil {
					t.Fatal(err)
				}
			}
			if err = stmt.Close(); err != nil {
				t.Fatal(err)
			}
			for i := range blob {
				blob[i] = 0
			}
			verify := func(db *sql.DB) {
				rows, err := db.Query("SELECT id,b,bit_to_text(b),bit_dims(b),typeof(b) FROM bits ORDER BY id")
				if err != nil {
					t.Fatal(err)
				}
				defer rows.Close()
				var got vector.NullBitVector
				count := 0
				for rows.Next() {
					var id int64
					var text sql.NullString
					var dims sql.NullInt64
					var storage string
					if err = rows.Scan(&id, &got, &text, &dims, &storage); err != nil {
						t.Fatal(err)
					}
					count++
					if id == 3 || id == 6 {
						if got.Valid || got.BitVector.Valid() || text.Valid || dims.Valid || storage != "null" {
							t.Fatal(id, got, text, dims, storage)
						}
					} else if !got.Valid || got.BitVector.String() != v.String() || text.String != v.String() || dims.Int64 != 9 || storage != "blob" {
						t.Fatal(id, got, text, dims, storage)
					}
				}
				if count != 6 || rows.Err() != nil {
					t.Fatal(count, rows.Err())
				}
			}
			verify(db)
			tx, err := db.BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec("INSERT INTO bits VALUES(7,'111111111','prior')"); err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec("INSERT INTO bits VALUES(8,'111111111','partial'),(9,'11','invalid')"); !errors.Is(err, vector.ErrDimensions) {
				t.Fatal(err)
			}
			retained, err := tx.Query("SELECT id FROM bits WHERE id>=7 ORDER BY id")
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
			if err = retained.Err(); err != nil {
				t.Fatal(err)
			}
			if err = retained.Close(); err != nil {
				t.Fatal(err)
			}
			if len(ids) != 1 || ids[0] != 7 {
				t.Fatal("statement rollback lost prior row or retained partial row", ids)
			}
			var id int64
			if err = tx.QueryRow("SELECT id FROM bits ORDER BY b <~> '111111111' DESC,id DESC LIMIT 1").Scan(&id); err != nil || id != 5 {
				t.Fatal(id, err)
			}
			if _, err = tx.Exec("UPDATE bits SET b='000000000' WHERE id=1"); err != nil {
				t.Fatal(err)
			}
			var text string
			if err = tx.QueryRow("SELECT bit_to_text(b) FROM bits WHERE id=1").Scan(&text); err != nil || text != "000000000" {
				t.Fatal(text, err)
			}
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			verify(db)
			for _, value := range []any{vector.BitVector{}, "11", 101} {
				if _, err = db.Exec("UPDATE bits SET b=? WHERE id=1", value); err == nil {
					t.Fatal("invalid binding accepted")
				}
			}
			var distance float64
			if err = db.QueryRow("SELECT b <~> ? FROM bits WHERE id=1", v).Scan(&distance); err != nil || distance != 0 {
				t.Fatal(distance, err)
			}
			if err = db.QueryRow("SELECT b <%> ? FROM bits WHERE id=1", v).Scan(&distance); err != nil || distance != 0 {
				t.Fatal(distance, err)
			}
			var got vector.NullBitVector
			if err = db.QueryRow("SELECT b FROM bits WHERE id=1").Scan(&got); err != nil {
				t.Fatal(err)
			}
			prior, _ := got.BitVector.MarshalBinary()
			if err = db.QueryRow("SELECT vector('[1]')").Scan(&got); err == nil {
				t.Fatal("cross-representation scan accepted")
			}
			after, _ := got.BitVector.MarshalBinary()
			if !got.Valid || !bytes.Equal(prior, after) {
				t.Fatal("failed scan changed destination")
			}
			if err = db.QueryRow("SELECT binary_quantize(subvector('[-1,0,1,2]',2,3))").Scan(&got); err != nil || !got.Valid || got.BitVector.String() != "011" {
				t.Fatal(got, err)
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
