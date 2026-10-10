package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var b bytes.Buffer
	if err := run([]string{"version"}, &b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "SQLite behavior baseline 3.54.0") {
		t.Fatal(b.String())
	}
}
func TestUnsupported(t *testing.T) {
	var b bytes.Buffer
	if run([]string{"query"}, &b) == nil {
		t.Fatal("unimplemented command accepted")
	}
}

func TestSQLWriteAndQuery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cli.db")
	var b bytes.Buffer
	statements := []string{
		"CREATE TABLE items(id INTEGER PRIMARY KEY, title TEXT NOT NULL)",
		"CREATE INDEX items_title ON items(title COLLATE BINARY)",
		"INSERT INTO items VALUES (1,'数据库'),(2,'other')",
		"UPDATE items SET title='updated' WHERE id=2",
		"DELETE FROM items WHERE id=2",
	}
	for _, q := range statements {
		b.Reset()
		if err := run([]string{"query", path, q}, &b); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		if !strings.Contains(b.String(), "rows_affected") {
			t.Fatal(b.String())
		}
	}
	b.Reset()
	if err := run([]string{"query", path, "SELECT id,title FROM items ORDER BY id"}, &b); err != nil {
		t.Fatal(err)
	}
	if b.String() != "{\"columns\":[\"id\",\"title\"]}\n{\"row\":[1,\"数据库\"]}\n" {
		t.Fatal(b.String())
	}
	if err := run([]string{"inspect", path}, &b); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"-journal", "-wal"} {
		if _, err := os.Stat(path + suffix); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("sidecar %s: %v", suffix, err)
		}
	}
}
func TestSQLRejectedBeforeCreation(t *testing.T) {
	for _, q := range []string{"BEGIN", "ROLLBACK", "CREATE VIEW v AS SELECT 1", "SELECT 1; SELECT 2"} {
		path := filepath.Join(t.TempDir(), "absent.db")
		var b bytes.Buffer
		if err := run([]string{"query", path, q}, &b); err == nil {
			t.Fatalf("accepted %q", q)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("created for %q: %v", q, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(t.TempDir(), "absent.db")
	var b bytes.Buffer
	if err := runContext(ctx, []string{"query", path, "CREATE TABLE t(id INTEGER)"}, &b); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancel created: %v", err)
	}
}

func TestCLIExclusiveWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal-cli.db")
	var b bytes.Buffer
	for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY, title TEXT)", "INSERT INTO t VALUES(1,'WAL')", "SELECT id,title FROM t"} {
		b.Reset()
		if err := run([]string{"query", "--wal-exclusive", path, q}, &b); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if !strings.Contains(b.String(), `{"row":[1,"WAL"]}`) {
		t.Fatal(b.String())
	}
}

func TestCLISharedWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared-cli.db")
	var b bytes.Buffer
	for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY,title TEXT)", "INSERT INTO t VALUES(1,'shared')", "SELECT title FROM t"} {
		b.Reset()
		if err := run([]string{"query", "--wal-shared", path, q}, &b); err != nil {
			t.Fatal(q, err)
		}
	}
	if !strings.Contains(b.String(), `{"row":["shared"]}`) {
		t.Fatal(b.String())
	}
}
