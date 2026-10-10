package driver

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func database(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("gosqlite", "file:"+filepath.Join("..", "testdata", "readonly", "page4096.db")+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}
func TestDatabaseSQLRowsAndPrepared(t *testing.T) {
	db := database(t)
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	stmt, err := db.PrepareContext(ctx, "SELECT id,label,payload,score,optional FROM samples WHERE id>=:lower AND id<:upper ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	rows, err := stmt.QueryContext(ctx, sql.Named("upper", 4), sql.Named("lower", 1))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(columns, []string{"id", "label", "payload", "score", "optional"}) {
		t.Fatal(columns)
	}
	count := 0
	for rows.Next() {
		var id int64
		var label string
		var blob []byte
		var score float64
		var optional sql.NullString
		if err := rows.Scan(&id, &label, &blob, &score, &optional); err != nil {
			t.Fatal(err)
		}
		count++
		if id != int64(count) || score != float64(count)*0.125 || len(blob) != 4 {
			t.Fatalf("row %d", id)
		}
	}
	if count != 3 || rows.Err() != nil {
		t.Fatalf("count=%d err=%v", count, rows.Err())
	}
	var answer int64
	if err := db.QueryRowContext(ctx, "SELECT ? + ?", 20, 22).Scan(&answer); err != nil || answer != 42 {
		t.Fatalf("answer=%d err=%v", answer, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT id FROM samples WHERE id=?", -1).Scan(&answer); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM samples"); !errors.Is(err, sqlite.ErrReadOnly) {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRowContext(ctx, "SELECT id FROM samples WHERE id=1").Scan(&answer); err != nil || answer != 1 {
		t.Fatalf("read-only transaction: answer=%d err=%v", answer, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	write, err := db.PrepareContext(ctx, "INSERT INTO samples(id) VALUES(?)")
	if err != nil {
		t.Fatal(err)
	}
	defer write.Close()
	if _, err := write.ExecContext(ctx, 10000); !errors.Is(err, sqlite.ErrReadOnly) {
		t.Fatal(err)
	}
}
func TestConnectorAndContext(t *testing.T) {
	connector, err := NewConnector("file:"+filepath.Join("..", "testdata", "readonly", "page4096.db")+"?mode=ro", sqlite.Options{MaxSortBytes: 1})
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(connector)
	defer db.Close()
	if _, err := db.Query("SELECT id FROM samples ORDER BY id"); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	rows, err := db.QueryContext(ctx, "SELECT id FROM samples")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	cancel()
	for rows.Next() {
	}
	if !errors.Is(rows.Err(), context.Canceled) {
		t.Fatal(rows.Err())
	}
	rows.Close()
	if _, err := db.QueryContext(ctx, "SELECT 1"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestDSNValidationNoCredentialDisclosure(t *testing.T) {
	accepted := map[string]string{"plain.db": "plain.db", "file:relative.db?mode=ro": "relative.db", "file:relative.db?mode=rw": "relative.db", "file:relative.db?mode=rwc": "relative.db", "file:/tmp/a%20b.db": "/tmp/a b.db", "file:///tmp/a.db?mode=ro": "/tmp/a.db", "file:a%3Fb.db": "a?b.db"}
	for dsn, want := range accepted {
		got, err := parseDSN(dsn)
		if err != nil || got != want {
			t.Fatalf("%q => %q, %v", dsn, got, err)
		}
	}
	for _, dsn := range []string{"", ":memory:", "file::memory:", "file:db?mode=memory", "file:db?mode=", "file:db?mode=ro&mode=ro", "file:db?mode=rw&mode=rwc", "file:db?key=SECRET", "file://user:SECRET@host/db", "file://localhost/db", "postgres://user:SECRET@host/db", "file:db#SECRET", "file:db?unknown=SECRET", "file:db?mode=ro;key=SECRET", "file:bad%zz", "file:"} {
		_, err := parseDSN(dsn)
		if err == nil {
			t.Fatalf("accepted %q", dsn)
		}
		if strings.Contains(err.Error(), "SECRET") {
			t.Fatal("credential leaked")
		}
	}
}
func TestPooledConcurrentQueries(t *testing.T) {
	db := database(t)
	db.SetMaxOpenConns(4)
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			var id int64
			err := db.QueryRow("SELECT id FROM samples WHERE id=?", n+1).Scan(&id)
			if err != nil || id != int64(n+1) {
				t.Errorf("id %d err %v", id, err)
			}
		}(n)
	}
	wg.Wait()
}
