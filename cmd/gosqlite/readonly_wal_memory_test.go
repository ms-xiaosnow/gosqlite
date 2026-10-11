package main

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestCLIReadOnlyWALArgumentsMemory(t *testing.T) {
	flags, tail, err := parseReadOnlyWALFlags([]string{"--wal-readonly", "db", "SELECT 1"}, false)
	if err != nil || len(tail) != 2 { t.Fatal(flags, tail, err) }
	opts, err := flags.queryOptions(true)
	if err != nil || !opts.ReadOnly || !opts.NoCreate || !opts.WALShared || opts.WALExclusive { t.Fatal(opts, err) }
	if _, err := flags.queryOptions(false); !errors.Is(err, sqlite.ErrReadOnly) { t.Fatal(err) }
	for _, args := range [][]string{
		{"--wal-readonly", "--wal-readonly", "db"},
		{"--wal-shared", "--wal-readonly", "db"},
		{"--wal-readonly", "--wal-exclusive", "db"},
		{"--key-provider", "env-hex:KEY", "db"},
	} {
		if _, _, err := parseReadOnlyWALFlags(args, false); err == nil { t.Fatal("accepted conflicting flags", args) }
	}
	for _, args := range [][]string{{"--wal-readonly", "--extensions", "db"}, {"--extensions", "--wal-readonly", "db"}} {
		flags, tail, err := parseReadOnlyWALFlags(args, true)
		if err != nil || len(tail) != 1 || !flags.extensions || !flags.walReadOnly { t.Fatal(flags, tail, err) }
	}
	for _, args := range [][]string{{"--wal-readonly", "db"}, {"--wal-readonly", "db", "grams"}} {
		o, err := parseCheck(args)
		if err != nil || !o.walReadOnly || o.walShared || o.walExclusive { t.Fatal(o, err) }
	}
	for _, args := range [][]string{
		{"--wal-readonly", "--wal-shared", "db"},
		{"--wal-readonly", "db", "--key-provider", "env-hex:KEY"},
		{"--wal-readonly", "db", "--mode", "shared"},
		{"--wal-shared", "db"},
		{"--wal-exclusive", "db"},
	} {
		if _, err := parseCheck(args); err == nil { t.Fatal("accepted conflicting/full writable check", args) }
	}
	// DML must reject before the supplied never-opened path reaches Open.
	if err := runContext(context.Background(), []string{"query", "--wal-readonly", "never-opened", "CREATE TABLE t(id INTEGER PRIMARY KEY)"}, io.Discard); !errors.Is(err, sqlite.ErrReadOnly) { t.Fatal(err) }
}
