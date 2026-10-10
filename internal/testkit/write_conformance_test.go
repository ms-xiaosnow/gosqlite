//go:build oracle

package testkit_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// These are ordinary transactions against valid synthetic databases. They do
// not inject I/O errors, corrupt files, crash or terminate database processes.
func writeExec(t *testing.T, db *sqlite.DB, query string, args ...any) sqlite.Result {
	t.Helper()
	result, err := db.Exec(context.Background(), query, args...)
	if err != nil {
		t.Fatalf("Exec %q: %v", query, err)
	}
	return result
}
func writeTxExec(t *testing.T, tx *sqlite.Tx, query string, args ...any) sqlite.Result {
	t.Helper()
	result, err := tx.Exec(context.Background(), query, args...)
	if err != nil {
		t.Fatalf("Tx.Exec %q: %v", query, err)
	}
	return result
}
func writeOpen(t *testing.T, path string, size uint32) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(path, sqlite.Options{PageSize: size})
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
func writeQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func writeIntegrity(t *testing.T, path string) {
	t.Helper()
	out := string(confCLI(t, path, "PRAGMA integrity_check;", true))
	if !strings.Contains(out, "'ok'") {
		t.Fatalf("official integrity_check: %s", out)
	}
}
func writeSame(t *testing.T, native *sqlite.DB, oraclePath, query string) {
	t.Helper()
	confCompare(t, confExpected(t, oraclePath, query), confNative(t, native, query))
}

func TestOracleNativeWriteRoundTrip(t *testing.T) {
	confOracle(t)
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "native.db")
			reference := filepath.Join(dir, "reference.db")
			db := writeOpen(t, path, size)
			ddl := []string{"CREATE TABLE items(id INTEGER PRIMARY KEY,title TEXT NOT NULL,payload BLOB,weight REAL,optional NUMERIC)", "CREATE TABLE untouched(id INTEGER PRIMARY KEY,note TEXT)", "INSERT INTO untouched VALUES(7,'preserved')", "CREATE INDEX items_title ON items(title COLLATE BINARY ASC)", "CREATE INDEX items_weight ON items(weight,title)"}
			var expected strings.Builder
			fmt.Fprintf(&expected, "PRAGMA page_size=%d;\n", size)
			for _, s := range ddl {
				writeExec(t, db, s)
				expected.WriteString(s + ";\n")
			}
			tx, err := db.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			for i := 1; i <= 320; i++ {
				title := fmt.Sprintf("row-%04d", i)
				if i%29 == 0 {
					title = "数据库🙂-" + title
				}
				payload := []byte{byte(i), 0, 255}
				if i%73 == 0 {
					payload = bytes.Repeat([]byte{byte(i)}, 9000)
				}
				weight := float64(i) * 0.125
				var optional any
				optSQL := "NULL"
				if i%3 != 0 {
					optional = fmt.Sprint(i)
					optSQL = writeQuote(optional.(string))
				}
				result := writeTxExec(t, tx, "INSERT INTO items(id,title,payload,weight,optional) VALUES(?,?,?,?,?)", int64(i), title, payload, weight, optional)
				if n, e := result.RowsAffected(); e != nil || n != 1 {
					t.Fatalf("affected=%d err=%v", n, e)
				}
				fmt.Fprintf(&expected, "INSERT INTO items VALUES(%d,%s,X'%s',%.3f,%s);\n", i, writeQuote(title), hex.EncodeToString(payload), weight, optSQL)
			}
			huge := strings.Repeat("数", 14000)
			blob := bytes.Repeat([]byte{0, 254, 128, 17}, 8000)
			writeTxExec(t, tx, "INSERT INTO items VALUES(?,?,?,?,?)", int64(1000), huge, blob, 1.25, nil)
			fmt.Fprintf(&expected, "INSERT INTO items VALUES(1000,%s,X'%s',1.25,NULL);\n", writeQuote(huge), hex.EncodeToString(blob))
			if err := tx.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
			confCLI(t, reference, expected.String(), false)
			for _, q := range []string{"SELECT * FROM items ORDER BY id", "SELECT id,title,typeof(weight),typeof(optional) FROM items ORDER BY id", "SELECT * FROM untouched ORDER BY id", "SELECT id,title FROM items ORDER BY title,id", "SELECT id,weight,title FROM items ORDER BY weight,title,id"} {
				writeSame(t, db, reference, q)
			}
			writeIntegrity(t, path)
			// SQLite must read the real native index, rather than recovering rows by a scan.
			q := "SELECT id,title FROM items INDEXED BY items_title ORDER BY title,id"
			confCompare(t, confExpected(t, reference, q), confExpected(t, path, q))
			// Ordinary updates/deletes cause overflow retirement, merges and freelist reuse.
			for _, q := range []string{"UPDATE items SET title='changed',payload=X'00FF',weight=weight+1 WHERE id<100", "DELETE FROM items WHERE id>150 AND id<300", "DELETE FROM items WHERE id=1000"} {
				writeExec(t, db, q)
				confCLI(t, reference, q+";", false)
			}
			tx, err = db.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			var more strings.Builder
			for i := 400; i < 600; i++ {
				title := fmt.Sprintf("reuse-%04d", i)
				writeTxExec(t, tx, "INSERT INTO items VALUES(?,?,?, ?,NULL)", int64(i), title, []byte{9, 8, 7}, float64(i))
				fmt.Fprintf(&more, "INSERT INTO items VALUES(%d,%s,X'090807',%d.0,NULL);\n", i, writeQuote(title), i)
			}
			if err := tx.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
			confCLI(t, reference, more.String(), false)
			writeSame(t, db, reference, "SELECT * FROM items ORDER BY id")
			writeSame(t, db, reference, "SELECT * FROM untouched ORDER BY id")
			writeIntegrity(t, path)
			confCompare(t, confExpected(t, reference, q), confExpected(t, path, q))
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			// The official engine updates a Go-created database through its native indices.
			external := "UPDATE items SET title='official',weight=2.5 WHERE id=320; INSERT INTO items VALUES(777,'from SQLite',X'CAFE',7.0,'42'); DELETE FROM items WHERE id=12;"
			confCLI(t, path, external, false)
			confCLI(t, reference, external, false)
			writeIntegrity(t, path)
			reopened := writeOpen(t, path, size)
			writeSame(t, reopened, reference, "SELECT * FROM items ORDER BY id")
		})
	}
}

func TestOracleExistingDatabaseUpdateAndTransactionRollback(t *testing.T) {
	confOracle(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.db")
	confCLI(t, path, "PRAGMA page_size=512; CREATE TABLE t(id INTEGER PRIMARY KEY,title TEXT NOT NULL DEFAULT 'default',v NUMERIC); CREATE INDEX t_title ON t(title); INSERT INTO t VALUES(1,'first',1),(2,'second',2); CREATE TABLE other(id INTEGER PRIMARY KEY,note TEXT); INSERT INTO other VALUES(99,'untouched'); PRAGMA user_version=87; PRAGMA application_id=123456;", false)
	db := writeOpen(t, path, 512)
	before := confDigest(t, path)
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	writeTxExec(t, tx, "INSERT INTO t VALUES(3,'rollback',3)")
	writeTxExec(t, tx, "UPDATE t SET title='rolled back' WHERE id=1")
	writeTxExec(t, tx, "CREATE TABLE transient(id INTEGER PRIMARY KEY,title TEXT)")
	rows, err := tx.Query(context.Background(), "SELECT title FROM t WHERE id=3")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || rows.Values()[0] != "rollback" {
		t.Fatal("transaction did not read own write")
	}
	rows.Close()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if confDigest(t, path) != before {
		t.Fatal("ordinary overlay rollback changed database bytes")
	}
	tx, err = db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	writeTxExec(t, tx, "INSERT INTO t VALUES(8,'keep',8)")
	if _, err := tx.Exec(context.Background(), "INSERT INTO t VALUES(9,'must disappear',9),(1,'duplicate',1)"); err == nil {
		t.Fatal("duplicate primary key accepted")
	}
	if _, err := tx.Exec(context.Background(), "INSERT INTO t VALUES(10,'must disappear',10),(11,NULL,11)"); err == nil {
		t.Fatal("NOT NULL accepted")
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := confResult{columns: []string{"id", "title"}, rows: [][]any{{int64(1), "first"}, {int64(2), "second"}, {int64(8), "keep"}}}
	confCompare(t, want, confNative(t, db, "SELECT id,title FROM t ORDER BY id"))
	writeExec(t, db, "CREATE TABLE edges(id INTEGER PRIMARY KEY,value TEXT)")
	for _, id := range []int64{math.MinInt64, -1, 0, math.MaxInt64} {
		writeExec(t, db, "INSERT INTO edges VALUES(?,?)", id, fmt.Sprint(id))
	}
	writeIntegrity(t, path)
	out := string(confCLI(t, path, "PRAGMA user_version; PRAGMA application_id; SELECT * FROM other;", true))
	if !strings.Contains(out, "87") || !strings.Contains(out, "123456") || !strings.Contains(out, "'untouched'") {
		t.Fatal(out)
	}
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		if _, err := os.Stat(path + suffix); !os.IsNotExist(err) {
			t.Fatalf("unexpected committed sidecar %s: %v", suffix, err)
		}
	}
}
