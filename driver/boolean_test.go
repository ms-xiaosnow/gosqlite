package driver_test

import (
	"context"
	"database/sql"
	"errors"
	gosqldriver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"path/filepath"
	"strings"
	"testing"
)

// Only normal SQL, prepared bindings, rollback and cancellation on intact files.
func TestDriverBooleanPreparedAndTransactions(t *testing.T) {
	connector, err := gosqldriver.NewConnector(filepath.Join(t.TempDir(), "boolean.db"), sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(connector)
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY,flag INTEGER)", "CREATE TABLE u(y INTEGER)", "INSERT INTO t VALUES(1,0)", "INSERT INTO u VALUES(2)"} {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	stmt, err := db.Prepare("SELECT t.id FROM t,u WHERE ?1 AND abs(-9223372036854775808)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	for _, v := range []any{int64(0), int64(1), nil, int64(0)} {
		rows, err := stmt.Query(v)
		if err == nil {
			if rows.Next() {
				t.Fatal("unexpected result")
			}
			err = rows.Err()
			rows.Close()
		}
		if (err != nil) != (v == int64(1)) {
			t.Fatalf("bound %v: %v", v, err)
		}
		if err != nil && !strings.Contains(err.Error(), "integer overflow") {
			t.Fatal(err)
		}
	}
	var value int64
	if err = db.QueryRow("SELECT CASE WHEN flag AND abs(-9223372036854775808) THEN 7 ELSE 9 END FROM t").Scan(&value); err != nil || value != 9 {
		t.Fatal(value, err)
	}
	if err = db.QueryRow("SELECT flag AND abs(-9223372036854775808) FROM t").Scan(&value); err == nil {
		t.Fatal("scalar unexpectedly lazy")
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("UPDATE t SET flag=1 WHERE 0 AND abs(-9223372036854775808)"); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("DELETE FROM t WHERE 0 AND abs(-9223372036854775808)"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT flag FROM t").Scan(&value); err != nil || value != 0 {
		t.Fatal(value, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = stmt.QueryContext(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
