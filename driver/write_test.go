package driver

import (
	"context"
	"database/sql"
	sqldriver "database/sql/driver"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func writableDatabase(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "writable.db")
	db, err := sql.Open("gosqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db, path
}

func mustExec(t *testing.T, db interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, query string, args ...any) sql.Result {
	t.Helper()
	result, err := db.ExecContext(context.Background(), query, args...)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return result
}

func checkAffected(t *testing.T, result sql.Result, want int64) {
	t.Helper()
	got, err := result.RowsAffected()
	if err != nil || got != want {
		t.Fatalf("RowsAffected = %d, %v; want %d", got, err, want)
	}
}

func TestWritableDatabaseSQLCRUDAndReopen(t *testing.T) {
	db, path := writableDatabase(t)
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	mustExec(t, db, "CREATE TABLE items (id INTEGER PRIMARY KEY, title TEXT NOT NULL, payload BLOB, score REAL, optional TEXT)")
	mustExec(t, db, "CREATE INDEX items_title ON items(title)")
	stmt, err := db.PrepareContext(ctx, "INSERT INTO items(title,payload,score,optional) VALUES (:title,:payload,:score,:optional)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	result, err := stmt.ExecContext(ctx, sql.Named("score", 1.5), sql.Named("title", "数据库🙂"), sql.Named("optional", nil), sql.Named("payload", []byte{0, 1, 255}))
	if err != nil {
		t.Fatal(err)
	}
	checkAffected(t, result, 1)
	id, err := result.LastInsertId()
	if err != nil || id != 1 {
		t.Fatalf("LastInsertId = %d, %v", id, err)
	}
	result = mustExec(t, db, "INSERT INTO items(title,payload,score) VALUES (?,?,?),(?,?,?)", "second", []byte{}, 2, "third", nil, 3)
	checkAffected(t, result, 2)
	if id, err := result.LastInsertId(); err != nil || id != 3 {
		t.Fatalf("multi-row LastInsertId = %d, %v", id, err)
	}
	checkAffected(t, mustExec(t, db, "UPDATE items SET score=score+?,title=? WHERE id=?", 0.5, "updated", 2), 1)
	checkAffected(t, mustExec(t, db, "DELETE FROM items WHERE id=?", 3), 1)
	checkAffected(t, mustExec(t, db, "UPDATE items SET title=? WHERE id=?", "missing", 999), 0)
	if err := stmt.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sql.Open("gosqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	rows, err := reopened.QueryContext(ctx, "SELECT id,title,payload,score,optional FROM items ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id int64
		var title string
		var payload []byte
		var score float64
		var optional sql.NullString
		if err := rows.Scan(&id, &title, &payload, &score, &optional); err != nil {
			t.Fatal(err)
		}
		count++
		if id != int64(count) || optional.Valid {
			t.Fatalf("id=%d optional=%v", id, optional)
		}
		if id == 1 && (title != "数据库🙂" || !reflect.DeepEqual(payload, []byte{0, 1, 255}) || score != 1.5) {
			t.Fatalf("first row: %q %v %v", title, payload, score)
		}
		if id == 2 && (title != "updated" || payload == nil || len(payload) != 0 || score != 2.5) {
			t.Fatalf("second row: %q %v %v", title, payload, score)
		}
	}
	if count != 2 || rows.Err() != nil {
		t.Fatalf("count=%d err=%v", count, rows.Err())
	}
}

func TestWritableDatabaseSQLTransactions(t *testing.T) {
	db, _ := writableDatabase(t)
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	mustExec(t, db, "CREATE TABLE items (id INTEGER PRIMARY KEY, title TEXT)")
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, tx, "INSERT INTO items VALUES (?,?)", 1, "committed")
	stmt, err := tx.PrepareContext(ctx, "UPDATE items SET title=:title WHERE id=:id")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stmt.ExecContext(ctx, sql.Named("id", 1), sql.Named("title", "visible")); err != nil {
		t.Fatal(err)
	}
	var title string
	if err := tx.QueryRowContext(ctx, "SELECT title FROM items WHERE id=1").Scan(&title); err != nil || title != "visible" {
		t.Fatalf("read own write: %q, %v", title, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM items"); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("after commit: %v", err)
	}
	if err := db.QueryRowContext(ctx, "SELECT title FROM items WHERE id=1").Scan(&title); err != nil || title != "visible" {
		t.Fatalf("committed: %q, %v", title, err)
	}
	tx, err = db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, tx, "UPDATE items SET title='discarded' WHERE id=1")
	mustExec(t, tx, "INSERT INTO items VALUES (2,'discarded')")
	mustExec(t, tx, "CREATE TABLE abandoned (id INTEGER PRIMARY KEY)")
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT title FROM items WHERE id=1").Scan(&title); err != nil || title != "visible" {
		t.Fatalf("rolled back update: %q, %v", title, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT title FROM items WHERE id=2").Scan(&title); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("rolled back insert: %v", err)
	}
	if err := db.QueryRowContext(ctx, "SELECT name FROM sqlite_schema WHERE name='abandoned'").Scan(&title); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("rolled back DDL: %v", err)
	}
}

func TestWritableReadOnlyTransactionsAndIsolation(t *testing.T) {
	db, _ := writableDatabase(t)
	mustExec(t, db, "CREATE TABLE items (id INTEGER PRIMARY KEY)")
	mustExec(t, db, "INSERT INTO items VALUES (1)")
	ctx := context.Background()
	for _, level := range []sql.IsolationLevel{sql.LevelReadUncommitted, sql.LevelReadCommitted, sql.LevelRepeatableRead, sql.LevelSnapshot, sql.LevelLinearizable, sql.IsolationLevel(100)} {
		if tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: level}); !errors.Is(err, sqlite.ErrUnsupported) {
			if tx != nil {
				tx.Rollback()
			}
			t.Fatalf("isolation %d: %v", level, err)
		}
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM items").Scan(&id); err != nil || id != 1 {
		t.Fatalf("read-only read: %d, %v", id, err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO items VALUES (2)"); !errors.Is(err, sqlite.ErrReadOnly) {
		t.Fatalf("read-only write: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, "INSERT INTO items VALUES (2)")
}

func TestWritableExplicitSQLTransactionsAndPoolReset(t *testing.T) {
	db, _ := writableDatabase(t)
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	mustExec(t, db, "CREATE TABLE items (id INTEGER PRIMARY KEY)")
	c, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, c, "BEGIN IMMEDIATE")
	mustExec(t, c, "INSERT INTO items VALUES (1)")
	var id int64
	if err := c.QueryRowContext(ctx, "SELECT id FROM items").Scan(&id); err != nil || id != 1 {
		t.Fatalf("explicit read own write: %d, %v", id, err)
	}
	if _, err := c.ExecContext(ctx, "BEGIN"); err == nil {
		t.Fatal("nested BEGIN accepted")
	}
	mustExec(t, c, "ROLLBACK")
	mustExec(t, c, "BEGIN")
	mustExec(t, c, "INSERT INTO items VALUES (2)")
	mustExec(t, c, "COMMIT")
	mustExec(t, c, "BEGIN")
	mustExec(t, c, "INSERT INTO items VALUES (3)")
	// Returning an unfinished SQL transaction must not leak it to the next
	// pool borrower, nor accidentally commit it with that borrower's writes.
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT id FROM items WHERE id=3").Scan(&id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("abandoned pooled transaction: %v", err)
	}
	if err := db.QueryRowContext(ctx, "SELECT id FROM items WHERE id=2").Scan(&id); err != nil || id != 2 {
		t.Fatalf("explicit commit: %d, %v", id, err)
	}
	mustExec(t, db, "INSERT INTO items VALUES (4)")
}

func TestWritableContextCancellation(t *testing.T) {
	db, _ := writableDatabase(t)
	db.SetMaxOpenConns(1)
	mustExec(t, db, "CREATE TABLE items (id INTEGER PRIMARY KEY)")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := db.ExecContext(ctx, "INSERT INTO items VALUES (1)"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Exec: %v", err)
	}
	if _, err := db.BeginTx(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled BeginTx: %v", err)
	}
	stmt, err := db.Prepare("INSERT INTO items VALUES (?)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.ExecContext(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled prepared Exec: %v", err)
	}
	txCtx, txCancel := context.WithCancel(context.Background())
	tx, err := db.BeginTx(txCtx, nil)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, tx, "INSERT INTO items VALUES (2)")
	txCancel()
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		t.Fatal(err)
	}
	var id int64
	if err := db.QueryRow("SELECT id FROM items").Scan(&id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("canceled transaction persisted: %d, %v", id, err)
	}
	mustExec(t, db, "INSERT INTO items VALUES (3)")
}

func TestWritableCommitBusyRollsBackForPool(t *testing.T) {
	db, path := writableDatabase(t)
	db.SetMaxOpenConns(1)
	mustExec(t, db, "CREATE TABLE items (id INTEGER PRIMARY KEY)")
	mustExec(t, db, "INSERT INTO items VALUES (1)")
	reader, err := sql.Open("gosqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	readTx, err := reader.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer readTx.Rollback()
	var id int64
	if err := readTx.QueryRow("SELECT id FROM items WHERE id=1").Scan(&id); err != nil || id != 1 {
		t.Fatalf("read snapshot: %d, %v", id, err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	mustExec(t, tx, "INSERT INTO items VALUES (2)")
	// An ordinary live reader prevents the rollback-mode exclusive commit
	// lock. database/sql cannot retry this Tx, so the adapter must roll it back.
	if err := tx.Commit(); !errors.Is(err, sqlite.ErrBusy) {
		t.Fatalf("commit with a live reader: %v", err)
	}
	if err := readTx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT id FROM items WHERE id=2").Scan(&id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("failed commit persisted: %d, %v", id, err)
	}
	mustExec(t, db, "INSERT INTO items VALUES (3)")
}

func TestWritablePooledConcurrentInserts(t *testing.T) {
	db, _ := writableDatabase(t)
	// The connection pool serializes these writes. Multi-connection writer
	// contention has explicit SQLITE_BUSY semantics, rather than hidden retries.
	db.SetMaxOpenConns(1)
	mustExec(t, db, "CREATE TABLE items (id INTEGER PRIMARY KEY, title TEXT)")
	const count = 12
	var wg sync.WaitGroup
	for n := 0; n < count; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if _, err := db.Exec("INSERT INTO items VALUES (?,?)", n+1, fmt.Sprintf("row %d", n+1)); err != nil {
				t.Errorf("insert %d: %v", n, err)
			}
		}(n)
	}
	wg.Wait()
	rows, err := db.Query("SELECT id FROM items ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
		var id int64
		if err := rows.Scan(&id); err != nil || id != int64(n) {
			t.Fatalf("id=%d, err=%v, want=%d", id, err, n)
		}
	}
	if n != count || rows.Err() != nil {
		t.Fatalf("rows=%d, err=%v", n, rows.Err())
	}
}

func TestWritableDSNModes(t *testing.T) {
	for _, mode := range []string{"ro", "rw"} {
		t.Run(mode+"_missing", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "missing.db")
			db, err := sql.Open("gosqlite", "file:"+path+"?mode="+mode)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := db.Ping(); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("missing %s: %v", mode, err)
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("%s created missing database: %v", mode, err)
			}
		})
	}
	for _, mode := range []string{"", "rwc"} {
		t.Run(mode+"_create", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "created.db")
			dsn := path
			if mode != "" {
				dsn = "file:" + path + "?mode=" + mode
			}
			db, err := sql.Open("gosqlite", dsn)
			if err != nil {
				t.Fatal(err)
			}
			mustExec(t, db, "CREATE TABLE items (id INTEGER PRIMARY KEY)")
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = sql.Open("gosqlite", "file:"+path+"?mode=rw")
			if err != nil {
				t.Fatal(err)
			}
			mustExec(t, db, "INSERT INTO items VALUES (1)")
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = sql.Open("gosqlite", "file:"+path+"?mode=ro")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec("INSERT INTO items VALUES (2)"); !errors.Is(err, sqlite.ErrReadOnly) {
				t.Fatalf("read-only Exec: %v", err)
			}
			var id int64
			if err := db.QueryRow("SELECT id FROM items").Scan(&id); err != nil || id != 1 {
				t.Fatalf("read-only Query: %d, %v", id, err)
			}
		})
	}
}

func TestConnectorModesNeverRelaxOptions(t *testing.T) {
	for _, tc := range []struct {
		dsn      string
		opts     sqlite.Options
		readOnly bool
		noCreate bool
	}{
		{"plain.db", sqlite.Options{}, false, false},
		{"file:a.db?mode=ro", sqlite.Options{}, true, false},
		{"file:a.db?mode=rw", sqlite.Options{}, false, true},
		{"file:a.db?mode=rwc", sqlite.Options{}, false, false},
		{"file:a.db?mode=rwc", sqlite.Options{ReadOnly: true}, true, false},
		{"file:a.db?mode=rw", sqlite.Options{ReadOnly: true}, true, true},
		{"file:a.db?mode=rwc", sqlite.Options{NoCreate: true}, false, true},
	} {
		connector, err := NewConnector(tc.dsn, tc.opts)
		if err != nil || connector.opts.ReadOnly != tc.readOnly || connector.opts.NoCreate != tc.noCreate {
			t.Fatalf("%s: connector=%+v err=%v", tc.dsn, connector, err)
		}
	}
	if _, err := NewConnector("plain.db", sqlite.Options{MaxSortBytes: -1}); err == nil {
		t.Fatal("negative sort budget accepted")
	}
}

func TestWritableDriverLegacyStatementAndClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "direct.db")
	connection, err := (&Driver{}).Open(path)
	if err != nil {
		t.Fatal(err)
	}
	c := connection.(*conn)
	defer c.Close()
	if _, err := c.ExecContext(context.Background(), "CREATE TABLE items (id INTEGER PRIMARY KEY)", nil); err != nil {
		t.Fatal(err)
	}
	s, err := c.Prepare("INSERT INTO items VALUES (?)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Exec([]sqldriver.Value{int64(1)}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Exec([]sqldriver.Value{int64(2)}); !errors.Is(err, sqlite.ErrClosed) {
		t.Fatalf("closed statement Exec: %v", err)
	}
	tx, err := c.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	txCtx, cancel := context.WithCancel(context.Background())
	tx, err = c.BeginTx(txCtx, sqldriver.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.ExecContext(context.Background(), "INSERT INTO items VALUES (2)", nil); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := tx.Commit(); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled driver Commit: %v", err)
	}
	if _, err := c.ExecContext(context.Background(), "INSERT INTO items VALUES (2)", nil); err != nil {
		t.Fatalf("connection after canceled Commit: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if c.IsValid() {
		t.Fatal("closed connection is valid")
	}
	if _, err := c.Prepare("SELECT 1"); !errors.Is(err, sqlite.ErrClosed) {
		t.Fatalf("closed connection Prepare: %v", err)
	}
	if err := c.ResetSession(context.Background()); !errors.Is(err, sqldriver.ErrBadConn) {
		t.Fatalf("closed connection ResetSession: %v", err)
	}
}
