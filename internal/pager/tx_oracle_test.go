//go:build oracle

package pager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/buildinfo"
	"github.com/ms-xiaosnow/gosqlite/internal/testoracle"
)

// This oracle is test-only. No crash, fault, forced exit, corrupted fixture,
// recovery, or I/O-failure workflow is run by these ordinary transaction tests.
func pagerOracle(t *testing.T, path, sql string) string {
	t.Helper()
	exe := os.Getenv("SQLITE_ORACLE")
	if exe == "" {
		t.Fatal("oracle acceptance requires explicit SQLITE_ORACLE")
	}
	cmd := testoracle.ForTest(t, exe, "-batch", "-bail", "-noheader", "-list", path)
	cmd.Stdin = strings.NewReader(sql + "\n")
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("official oracle %q: %v: %s", sql, err, b)
	}
	return strings.TrimSpace(string(b))
}
func verifyPagerOracle(t *testing.T) {
	t.Helper()
	got := pagerOracle(t, ":memory:", "SELECT sqlite_version(),sqlite_source_id();")
	want := buildinfo.SQLiteBaseline + "|" + buildinfo.SQLiteSourceID
	if got != want {
		t.Fatalf("oracle mismatch: %q want %q", got, want)
	}
}

func TestTxOracleNativeEmptyAndFreelistIntegrity(t *testing.T) {
	txPlatform(t)
	verifyPagerOracle(t)
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := createEmpty(t, size)
			if got := pagerOracle(t, path, "PRAGMA integrity_check; SELECT count(*) FROM sqlite_schema;"); got != "ok\n0" {
				t.Fatal(got)
			}
			tx := openTxTest(t, path, Options{})
			for i := 0; i < 150; i++ {
				if _, err := tx.AllocatePage(ctx); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 150; i++ {
				if err := tx.FreePage(ctx, uint32(i+2)); err != nil {
					t.Fatal(err)
				}
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if got := pagerOracle(t, path, "PRAGMA integrity_check; PRAGMA freelist_count;"); got != "ok\n150" {
				t.Fatal(got)
			}
			// Official SQLite reuses pages originally allocated/freed by Go.
			got := pagerOracle(t, path, "CREATE TABLE ordinary(id INTEGER PRIMARY KEY, v TEXT); INSERT INTO ordinary VALUES(1,'数据库'),(2,'after native commit'); PRAGMA integrity_check; SELECT id,v FROM ordinary ORDER BY id;")
			if got != "ok\n1|数据库\n2|after native commit" {
				t.Fatal(got)
			}
			tx = openTxTest(t, path, Options{})
			if err := tx.BumpSchemaCookie(ctx); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if got := pagerOracle(t, path, "PRAGMA integrity_check; SELECT count(*) FROM ordinary;"); got != "ok\n2" {
				t.Fatal(got)
			}
		})
	}
}

func TestTxOracleReuseOfficialFreelistPreservesRowsAndIndex(t *testing.T) {
	txPlatform(t)
	verifyPagerOracle(t)
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "official.db")
			pagerOracle(t, path, fmt.Sprintf("PRAGMA page_size=%d; PRAGMA journal_mode=DELETE; CREATE TABLE data(id INTEGER PRIMARY KEY, v TEXT); CREATE INDEX data_v ON data(v); WITH RECURSIVE n(x) AS(VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<200) INSERT INTO data SELECT x,printf('%%04d',x)||printf('%%01000d',x) FROM n; DELETE FROM data WHERE id%%10!=0;", size))
			expected := pagerOracle(t, path, "PRAGMA integrity_check; SELECT id,length(v),substr(v,1,4) FROM data ORDER BY id;")
			tx := openTxTest(t, path, Options{})
			free := len(tx.FreePages())
			if free == 0 {
				t.Fatal("oracle did not produce a freelist")
			}
			const limit = 30
			if free > limit {
				free = limit
			}
			nums := make([]uint32, free)
			originalPages := tx.Header().PageCount
			for i := range nums {
				n, err := tx.AllocatePage(ctx)
				if err != nil {
					t.Fatal(err)
				}
				nums[i] = n
			}
			if tx.Header().PageCount != originalPages {
				t.Fatal("allocation failed to reuse official freelist")
			}
			for _, n := range nums {
				if err := tx.FreePage(ctx, n); err != nil {
					t.Fatal(err)
				}
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			got := pagerOracle(t, path, "PRAGMA integrity_check; SELECT id,length(v),substr(v,1,4) FROM data ORDER BY id;")
			if got != expected {
				t.Fatalf("rows/index changed: got %s want %s", got, expected)
			}
		})
	}
}

func TestTxOraclePOSIXAndNativeOFDLocksConflict(t *testing.T) {
	txPlatform(t)
	verifyPagerOracle(t)
	ctx := context.Background()
	path := createEmpty(t, 4096)
	tx := openTxTest(t, path, Options{})
	busy := func(sql string) {
		t.Helper()
		cmd := testoracle.ForTest(t, os.Getenv("SQLITE_ORACLE"), "-batch", "-bail", path, sql)
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "database is locked") {
			t.Fatalf("expected official BUSY for %s: %v %s", sql, err, out)
		}
	}
	// Official process opens/closes normally. These are ordinary lock-contention
	// transactions, not termination/crash/recovery tests.
	busy("BEGIN EXCLUSIVE; COMMIT;")
	if err := tx.PromoteWrite(ctx); err != nil {
		t.Fatal(err)
	}
	busy("BEGIN IMMEDIATE; COMMIT;")
	if got := pagerOracle(t, path, "SELECT count(*) FROM sqlite_schema;"); got != "0" {
		t.Fatal(got)
	}
	if err := tx.Close(); err != nil {
		t.Fatal(err)
	}
	if got := pagerOracle(t, path, "BEGIN EXCLUSIVE; COMMIT; PRAGMA integrity_check;"); got != "ok" {
		t.Fatal(got)
	}
	noJournal(t, path)
}

// A read-only header/inspection probe is a managed lease even though it never
// writes. The independent official process must still see the sibling writer.
func TestTxOracleManagedProbeRetainsReservation(t *testing.T) {
	txPlatform(t)
	verifyPagerOracle(t)
	ctx := context.Background()
	path := createEmpty(t, 4096)
	tx := openTxTest(t, path, Options{})
	if err := tx.PromoteWrite(ctx); err != nil {
		t.Fatal(err)
	}
	probe, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = probe.ReadPage(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if err = probe.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := testoracle.ForTest(t, os.Getenv("SQLITE_ORACLE"), "-batch", "-bail", path, "BEGIN IMMEDIATE; COMMIT;")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "database is locked") {
		t.Fatalf("probe released native reservation: %v %s", err, out)
	}
	if got := pagerOracle(t, path, "SELECT count(*) FROM sqlite_schema;"); got != "0" {
		t.Fatal(got)
	}
	if err = tx.Close(); err != nil {
		t.Fatal(err)
	}
	if got := pagerOracle(t, path, "BEGIN IMMEDIATE; COMMIT; PRAGMA integrity_check;"); got != "ok" {
		t.Fatal(got)
	}
}
