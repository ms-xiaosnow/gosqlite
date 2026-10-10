package driver_test

import (
	"context"
	"database/sql"
	_ "github.com/ms-xiaosnow/gosqlite/driver"
	"path/filepath"
	"strings"
	"testing"
)

func TestDriverPersistentGrams(t *testing.T) {
	db, err := sql.Open("gosqlite", filepath.Join(t.TempDir(), "grams.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, q := range []string{"CREATE TABLE docs(id INTEGER PRIMARY KEY,title TEXT)", "INSERT INTO docs VALUES(1,'数据库'),(2,'other'),(3,'plain'),(4,'more'),(5,'none')", "CREATE INDEX docs_grams ON docs(title) USING grams13"} {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	stmt, err := db.Prepare("SELECT id FROM docs WHERE title LIKE ? ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	var id int64
	if err = stmt.QueryRow("%数据%").Scan(&id); err != nil || id != 1 {
		t.Fatalf("id=%d error=%v", id, err)
	}
	var detail string
	var a, b, c int64
	if err = db.QueryRow("EXPLAIN QUERY PLAN SELECT id FROM docs WHERE title LIKE '%库%'").Scan(&a, &b, &c, &detail); err != nil || !strings.Contains(detail, "SEARCH") {
		t.Fatalf("plan %s error=%v", detail, err)
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("UPDATE docs SET title='changed' WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = stmt.QueryRow("%数据库%").Scan(&id); err != nil || id != 1 {
		t.Fatalf("rollback id=%d error=%v", id, err)
	}
	if _, err = db.Exec("REINDEX docs_grams"); err != nil {
		t.Fatal(err)
	}
}
