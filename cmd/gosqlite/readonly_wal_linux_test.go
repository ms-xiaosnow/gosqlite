//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestCLIReadOnlyWALQueryInspectCheck(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cli-readonly.db")
	keeper, err := sqlite.Open(path, sqlite.Options{WALShared: true, KeepWALOnClose: true})
	if err != nil { t.Fatal(err) }
	defer keeper.Close()
	for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT)", "CREATE INDEX idx ON t(k)", "INSERT INTO t VALUES(1,'数据库')", "CREATE INDEX grams ON t(k) USING grams13"} {
		if _, err := keeper.Exec(context.Background(), q); err != nil { t.Fatal(err) }
	}
	before := cliCheckFamily(t, dir)
	for _, tc := range []struct{ args []string; want string }{
		{[]string{"query", "--wal-readonly", path, "SELECT k FROM t WHERE id=1"}, "数据库"},
		{[]string{"inspect", "--wal-readonly", path}, "metadata inspection only"},
		{[]string{"inspect", "--extensions", "--wal-readonly", path}, "extension_indexes"},
		{[]string{"check", "--wal-readonly", path}, sqlite.CheckScope},
		{[]string{"check", "--wal-readonly", path, "grams"}, "consistency only"},
	} {
		var out bytes.Buffer
		if err := run(tc.args, &out); err != nil || !strings.Contains(out.String(), tc.want) { t.Fatal(tc.args[0], out.String(), err) }
	}
	var out bytes.Buffer
	if err := run([]string{"query", "--wal-readonly", path, "UPDATE t SET k='forbidden'"}, &out); !errors.Is(err, sqlite.ErrReadOnly) || out.Len() != 0 { t.Fatal(err, out.String()) }
	if err := run([]string{"query", "--wal-readonly", path, "SELECT missing FROM t"}, &out); err == nil { t.Fatal("invalid query accepted") }
	if !reflect.DeepEqual(before, cliCheckFamily(t, dir)) { t.Fatal("quiescent readonly CLI changed family names or bytes") }
	if err := keeper.Close(); err != nil { t.Fatal(err) }
}
