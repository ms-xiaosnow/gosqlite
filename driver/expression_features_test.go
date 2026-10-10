package driver_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"

	gosqldriver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestDriverExpressionFeaturesPreparedTransactions(t *testing.T) {
	connector, err := gosqldriver.NewConnector(filepath.Join(t.TempDir(), "expressions.db"), sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(connector)
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err = db.Exec("CREATE TABLE e(id INTEGER PRIMARY KEY,n INTEGER)"); err != nil {
		t.Fatal(err)
	}
	stmt, err := db.Prepare("INSERT INTO e VALUES(?,CASE WHEN ? IN(1,2) THEN CAST(? AS INTEGER) ELSE NULL END)")
	if err != nil {
		t.Fatal(err)
	}
	for i, arg := range []any{"12", "3", "junk"} {
		if _, err = stmt.Exec(i+1, i+1, arg); err != nil {
			t.Fatal(err)
		}
	}
	if err = stmt.Close(); err != nil {
		t.Fatal(err)
	}
	query, err := db.Prepare("SELECT ALL id,CASE n WHEN ? THEN 'match' ELSE 'other' END FROM e WHERE id BETWEEN ? AND ? ORDER BY n NULLS LAST,id")
	if err != nil {
		t.Fatal(err)
	}
	defer query.Close()
	for range 2 {
		rows, err := query.Query("12", 1, 3)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for rows.Next() {
			var id int
			var label string
			if err = rows.Scan(&id, &label); err != nil {
				t.Fatal(err)
			}
			got = append(got, label)
		}
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		if !reflect.DeepEqual(got, []string{"other", "match", "other"}) {
			t.Fatal(got)
		}
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("UPDATE e SET n=CASE WHEN id IN(1,2) THEN n+1 ELSE n END"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = db.QueryRow("SELECT CAST(n AS INTEGER) FROM e WHERE id=1").Scan(&n); err != nil || n != 12 {
		t.Fatal(n, err)
	}
}
