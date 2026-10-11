//go:build oracle && linux

package testkit_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type readonlyLiveMember struct {
	Size int64
	Hash [32]byte
}

// Capture only this test's three small, complete files. The official peer is
// idle during each before/after interval; lock state and atime are not asserted.
func readonlyLiveFamily(t *testing.T, dir string) map[string]readonlyLiveMember {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"live.db", "live.db-shm", "live.db-wal"}
	if len(entries) != len(want) {
		t.Fatalf("family count: got %d, want %d", len(entries), len(want))
	}
	result := make(map[string]readonlyLiveMember, len(want))
	for i, entry := range entries {
		if entry.Name() != want[i] {
			t.Fatalf("family entry: got %q, want %q", entry.Name(), want[i])
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("family metadata %q: %v", entry.Name(), err)
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		result[entry.Name()] = readonlyLiveMember{Size: info.Size(), Hash: sha256.Sum256(data)}
	}
	return result
}

// Next may retire its lease internally and retain a close error in Err even
// when a later public Close is idempotent. Preserve that uncertainty first.
func readonlyLiveRowsErr(rows *sqlite.Rows, retirementUnknown *bool) error {
	err := rows.Err()
	if err != nil {
		*retirementUnknown = true
	}
	return err
}

func readonlyLiveValue(t *testing.T, q interface {
	Query(context.Context, string, ...any) (*sqlite.Rows, error)
}, retirementUnknown *bool) string {
	t.Helper()
	rows, err := q.Query(context.Background(), "SELECT text FROM t WHERE id=1")
	if err != nil {
		*retirementUnknown = true
		t.Fatal(err)
	}
	defer func() {
		if *retirementUnknown {
			return
		}
		if err := rows.Close(); err != nil {
			*retirementUnknown = true
			t.Fatalf("value Rows retirement (not retried): %v", err)
		}
	}()
	if !rows.Next() {
		t.Fatalf("missing row: %v", readonlyLiveRowsErr(rows, retirementUnknown))
	}
	values := rows.Values()
	if len(values) != 1 {
		t.Fatalf("unexpected column count: %d; error: %v", len(values), readonlyLiveRowsErr(rows, retirementUnknown))
	}
	value, ok := values[0].(string)
	if !ok || rows.Next() || readonlyLiveRowsErr(rows, retirementUnknown) != nil {
		t.Fatalf("unexpected row or iteration error: %v", readonlyLiveRowsErr(rows, retirementUnknown))
	}
	return value
}

// This is a normal live peer, not recovery or a permission/fault test. The
// existing isolated oracle factory controls its startup. It exits normally via
// .quit/EOF; this test does not signal, kill, truncate, rename, or damage files.
func TestOracleLiveWriterReadOnlyWALSnapshots(t *testing.T) {
	ctx := context.Background()
	for _, pageSize := range []uint32{512, 4096, 65536} {
		for _, keep := range []bool{false, true} {
			if !t.Run(fmt.Sprintf("%d-keep-%v", pageSize, keep), func(t *testing.T) {
				// Match the existing Go/factory precedence without changing any
				// environment variable or calling a launch-factory internal API.
				tempRoot := os.Getenv("GOTMPDIR")
				if tempRoot == "" {
					tempRoot = os.Getenv("TMPDIR")
				}
				if tempRoot == "" {
					tempRoot = "/tmp"
				}
				if !filepath.IsAbs(tempRoot) || filepath.Clean(tempRoot) != tempRoot || strings.ContainsAny(tempRoot, "\\\x00") || (tempRoot != "/tmp" && !strings.HasPrefix(tempRoot, "/tmp/")) {
					t.Fatalf("unsupported existing temporary root: %q", tempRoot)
				}
				dir, err := os.MkdirTemp(tempRoot, "gosqlite-readonly-live-")
				if err != nil {
					t.Fatal(err)
				}
				// Retain success and failure fixtures. In particular, an uncertain
				// retirement must never be followed by automatic directory removal.
				t.Logf("retained ordinary interoperability fixture: %s", dir)
				retirementUnknown := false
				path := filepath.Join(dir, "live.db")
				peer := startLiveOracle(t, path)
				liveWant(t, peer, fmt.Sprintf("PRAGMA page_size=%d; PRAGMA journal_mode=WAL;", pageSize), "wal")
				liveWant(t, peer, "PRAGMA page_size;", fmt.Sprint(pageSize))
				peer.run("CREATE TABLE t(id INTEGER PRIMARY KEY,text TEXT); INSERT INTO t VALUES(1,'old'),(2,'stable');")
				liveWant(t, peer, "BEGIN; SELECT text FROM t WHERE id=1;", "old")

				before := readonlyLiveFamily(t, dir)
				db, err := sqlite.Open(path, sqlite.Options{ReadOnly: true, WALShared: true, KeepWALOnClose: keep})
				if err != nil {
					t.Fatal(err)
				}
				closed := false
				closeDB := func() {
					if closed || retirementUnknown {
						return
					}
					closed = true
					if err := db.Close(); err != nil {
						retirementUnknown = true
						t.Fatalf("read-only close (not retried): %v", err)
					}
				}
				defer closeDB()
				defer func() {
					if retirementUnknown {
						t.Logf("retirement unknown; parent native owners may remain; fixture retained: %s", dir)
					}
				}()
				if got := db.Info().PageSize; got != pageSize {
					t.Fatalf("native page size: got %d, want %d", got, pageSize)
				}
				if got := readonlyLiveValue(t, db, &retirementUnknown); got != "old" {
					t.Fatalf("initial snapshot: %q", got)
				}
				if _, err := db.Exec(ctx, "UPDATE t SET text='forbidden' WHERE id=1"); !errors.Is(err, sqlite.ErrReadOnly) {
					t.Fatalf("read-only write refusal: %v", err)
				}
				if err := db.Checkpoint(ctx); !errors.Is(err, sqlite.ErrReadOnly) {
					t.Fatalf("read-only checkpoint refusal: %v", err)
				}
				if !reflect.DeepEqual(before, readonlyLiveFamily(t, dir)) {
					t.Fatal("quiescent read-only open/query/refusal changed the three files")
				}

				tx, err := db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
				if err != nil {
					t.Fatal(err)
				}
				txEnded := false
				defer func() {
					if !txEnded && !retirementUnknown {
						txEnded = true
						if err := tx.Rollback(); err != nil {
							retirementUnknown = true
							t.Errorf("read transaction retirement (not retried): %v", err)
						}
					}
				}()
				if got := readonlyLiveValue(t, tx, &retirementUnknown); got != "old" {
					t.Fatalf("transaction baseline: %q", got)
				}
				liveWant(t, peer, "COMMIT; UPDATE t SET text='newer' WHERE id=1; BEGIN; SELECT text FROM t WHERE id=1;", "newer")
				if got := readonlyLiveValue(t, tx, &retirementUnknown); got != "old" {
					t.Fatalf("transaction lost pinned snapshot: %q", got)
				}
				txEnded = true
				if err := tx.Commit(ctx); err != nil {
					retirementUnknown = true
					t.Fatal(err)
				}
				if got := readonlyLiveValue(t, db, &retirementUnknown); got != "newer" {
					t.Fatalf("next snapshot did not advance: %q", got)
				}

				rows, err := db.Query(ctx, "SELECT id,text FROM t ORDER BY id")
				if err != nil {
					retirementUnknown = true
					t.Fatal(err)
				}
				rowsClosed := false
				closeRows := func() {
					if !rowsClosed && !retirementUnknown {
						rowsClosed = true
						if err := rows.Close(); err != nil {
							retirementUnknown = true
							t.Fatalf("rows retirement (not retried): %v", err)
						}
					}
				}
				defer closeRows()
				liveWant(t, peer, "COMMIT; UPDATE t SET text='latest' WHERE id=1; BEGIN; SELECT text FROM t WHERE id=1;", "latest")
				if !rows.Next() {
					t.Fatalf("Rows missing first snapshot row: %v", readonlyLiveRowsErr(rows, &retirementUnknown))
				}
				if values := rows.Values(); len(values) != 2 || values[1] != "newer" {
					t.Fatalf("Rows lost pinned snapshot: %v", readonlyLiveRowsErr(rows, &retirementUnknown))
				}
				if !rows.Next() {
					t.Fatalf("Rows missing stable row: %v", readonlyLiveRowsErr(rows, &retirementUnknown))
				}
				if values := rows.Values(); len(values) != 2 || values[1] != "stable" {
					t.Fatalf("Rows content changed: %v", readonlyLiveRowsErr(rows, &retirementUnknown))
				}
				if rows.Next() || readonlyLiveRowsErr(rows, &retirementUnknown) != nil {
					t.Fatalf("Rows extra row or retirement error: %v", readonlyLiveRowsErr(rows, &retirementUnknown))
				}
				closeRows()
				if got := readonlyLiveValue(t, db, &retirementUnknown); got != "latest" {
					t.Fatalf("snapshot after Rows did not advance: %q", got)
				}
				beforeClose := readonlyLiveFamily(t, dir)
				closeDB()
				if !reflect.DeepEqual(beforeClose, readonlyLiveFamily(t, dir)) {
					t.Fatal("quiescent read-only close changed the three files")
				}
				liveWant(t, peer, "SELECT text FROM t WHERE id=1;", "latest")
				liveWant(t, peer, "PRAGMA integrity_check;", "ok")
				peer.run("COMMIT;")
				peer.close()
			}) {
				// A failed subcase does not authorize launching another peer.
				return
			}
		}
	}
}
