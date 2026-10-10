package driver_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	gosqldriver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Prepared statement reuse uses ordinary SQL and normal resource retirement.
func TestDriverBooleanAliasOnPreparedReuse(t *testing.T) {
	c, err := gosqldriver.NewConnector(filepath.Join(t.TempDir(), "alias-on.db"), sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(c)
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, q := range []string{`CREATE TABLE l(id INTEGER)`, `INSERT INTO l VALUES(1),(2)`, `CREATE TABLE u(y INTEGER)`, `INSERT INTO u VALUES(7)`} {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	stmt, err := db.Prepare(`SELECT l.id,?1 AS TrUe,u.y FROM l LEFT JOIN u ON true ORDER BY l.id`)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	for _, v := range []any{int64(0), int64(1), nil, int64(1), int64(0)} {
		rows, err := stmt.Query(v)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for rows.Next() {
			var id, alias, y int64
			if err = rows.Scan(&id, &alias, &y); err != nil {
				t.Fatal(err)
			}
			n++
			if id != int64(n) || alias != 1 || y != 7 {
				t.Fatal(id, alias, y)
			}
		}
		err = rows.Err()
		rows.Close()
		want := 0
		if v == int64(1) {
			want = 2
		}
		if err != nil || n != want {
			t.Fatal(v, n, err)
		}
	}
}
