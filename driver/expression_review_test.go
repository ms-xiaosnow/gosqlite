package driver_test

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"

	gosqldriver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Independent, ordinary database/sql prepared bindings and normal close/reopen.
// No failed physical I/O, interruption, recovery or mutation of a database file.
func TestDriverExpressionReviewBindOwnershipReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "expression-review.db")
	open := func() *sql.DB {
		c, err := gosqldriver.NewConnector(path, sqlite.Options{})
		if err != nil {
			t.Fatal(err)
		}
		db := sql.OpenDB(c)
		db.SetMaxOpenConns(1)
		return db
	}
	db := open()
	defer func() {
		if db != nil {
			db.Close()
		}
	}()
	if _, err := db.Exec("CREATE TABLE e(id INTEGER PRIMARY KEY,\"true\" INTEGER,\"false\" INTEGER,n INTEGER DEFAULT (CAST(CASE 2 WHEN 1 THEN 0 WHEN 2 THEN '7' END AS INTEGER)))"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO e(id,\"true\",\"false\") VALUES(1,2,3)"); err != nil {
		t.Fatal(err)
	}
	q, err := db.Prepare("SELECT CASE TRUE WHEN 2 THEN CAST(? AS BLOB) ELSE NULL END, TRUE BETWEEN 1 AND FALSE, n FROM e WHERE id IN(?,?)")
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		input := []byte("before")
		rows, err := q.Query(input, 1, 2)
		if err != nil {
			t.Fatal(err)
		}
		copy(input, []byte("after!"))
		if !rows.Next() {
			t.Fatal(rows.Err())
		}
		var got []byte
		var yes, n int64
		if err = rows.Scan(&got, &yes, &n); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, []byte("before")) || yes != 1 || n != 7 {
			t.Fatal(got, yes, n)
		}
		if rows.Next() || rows.Err() != nil {
			t.Fatal(rows.Err())
		}
		if err = rows.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err = q.Close(); err != nil {
		t.Fatal(err)
	}
	var empty, chosen int64
	if err = db.QueryRow("SELECT ?3 IN(),CASE ?2 WHEN 2 THEN CAST(?1 AS INTEGER) ELSE 0 END", "12", 2, []byte("unused")).Scan(&empty, &chosen); err != nil || empty != 0 || chosen != 12 {
		t.Fatal(empty, chosen, err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db = open()
	if _, err = db.Exec("INSERT INTO e(id) VALUES(2)"); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err = db.QueryRow("SELECT ALL n FROM e WHERE id=2 ORDER BY n DESC NULLS FIRST").Scan(&n); err != nil || n != 7 {
		t.Fatal(n, err)
	}
}
