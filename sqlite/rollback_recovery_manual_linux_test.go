//go:build manual_recovery && linux

package sqlite_test

// SOURCE ONLY / NOT_RUN. This bounded public-API harness is excluded from
// ordinary and oracle builds. Never execute it in the restricted environment,
// through CI, or on another executor as a workaround. Independent manual use
// requires both the explicit environment gate and a fresh disposable directory.
// It does not simulate a crash, power loss, short write or Sync failure.
// Every input and working directory is retained, even after PASS, for inspection.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/journal"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestManualPublicRollbackRecovery(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_RECOVERY") != "independently_authorized" {
		t.Fatal("NOT_RUN: public physical recovery requires independent authorization")
	}
	root := os.Getenv("GOSQLITE_MANUAL_RECOVERY_DIR")
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		t.Fatal("GOSQLITE_MANUAL_RECOVERY_DIR must name a fresh absolute clean directory")
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("disposable root must be an existing non-symlink directory: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || resolved != root {
		t.Fatalf("disposable root must have no symlink components: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("disposable root must be empty: entries=%d error=%v", len(entries), err)
	}
	manualPublicRecoveryNoPending(t)
	t.Cleanup(func() {
		for _, pending := range sqlite.PendingPlaintextCleanups() {
			t.Errorf("pending owner after scenario cleanup; retain all fixtures: %v", pending)
		}
	})
	t.Logf("retaining every input and working file under %s; no automatic deletion", root)

	if !t.Run("nil-default-preserves-input", func(t *testing.T) {
		path, _, current, data := manualPublicRecoveryFiles(t, root)
		db, err := sqlite.OpenContext(context.Background(), path, sqlite.Options{NoCreate: true})
		manualPublicRecoveryRefusal(t, db, err, sqlite.ErrRecoveryRequired)
		manualPublicRecoveryEqual(t, path, current)
		manualPublicRecoveryEqual(t, path+"-journal", data)
		manualPublicRecoveryNoPending(t)
	}) {
		return
	}
	if !t.Run("opt-in-restores-and-reopens", func(t *testing.T) {
		path, original, _, _ := manualPublicRecoveryFiles(t, root)
		db, err := sqlite.OpenContext(context.Background(), path, sqlite.Options{
			NoCreate: true, RollbackRecovery: &sqlite.RollbackRecoveryOptions{},
		})
		closeDB := manualPublicRecoveryOwnDB(t, db, err)
		if got := db.Info(); got.PageSize != 512 || got.PageCount != 2 || got.ReadVersion != 1 || got.WriteVersion != 1 {
			t.Fatalf("restored metadata: %+v", got)
		}
		schema, err := db.Schema(context.Background())
		if err != nil || len(schema) != 0 {
			t.Fatalf("restored empty sqlite_schema: entries=%d error=%v", len(schema), err)
		}
		if !closeDB() {
			return
		}
		manualPublicRecoveryEqual(t, path, original)
		manualPublicRecoveryAbsent(t, path+"-journal")
		db, err = sqlite.OpenContext(context.Background(), path, sqlite.Options{ReadOnly: true, NoCreate: true})
		closeDB = manualPublicRecoveryOwnDB(t, db, err)
		if got := db.Info(); got.PageCount != 2 || got.PageSize != 512 {
			t.Fatalf("read-only reopen metadata: %+v", got)
		}
		if !closeDB() {
			return
		}
		manualPublicRecoveryEqual(t, path, original)
		manualPublicRecoveryAbsent(t, path+"-journal")
		manualPublicRecoveryNoPending(t)
	}) {
		return
	}
	if !t.Run("journal-limit-preserves-input", func(t *testing.T) {
		path, _, current, data := manualPublicRecoveryFiles(t, root)
		db, err := sqlite.OpenContext(context.Background(), path, sqlite.Options{
			NoCreate: true, RollbackRecovery: &sqlite.RollbackRecoveryOptions{MaxJournalBytes: 1},
		})
		manualPublicRecoveryRefusal(t, db, err, sqlite.ErrRecoveryRequired, sqlite.ErrMemoryLimit)
		manualPublicRecoveryEqual(t, path, current)
		manualPublicRecoveryEqual(t, path+"-journal", data)
		manualPublicRecoveryNoPending(t)
	}) {
		return
	}
	if !t.Run("no-create-preserves-absence", func(t *testing.T) {
		dir, err := os.MkdirTemp(root, "no-create-")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "absent.db")
		t.Logf("retained isolated directory: %s", dir)
		for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
			manualPublicRecoveryAbsent(t, path+suffix)
		}
		db, err := sqlite.OpenContext(context.Background(), path, sqlite.Options{
			NoCreate: true, RollbackRecovery: &sqlite.RollbackRecoveryOptions{},
		})
		if db != nil {
			closeErr := db.Close()
			t.Fatalf("NoCreate unexpectedly returned a DB: open=%v close=%v", err, closeErr)
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("NoCreate must retain absence cause: %v", err)
		}
		for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
			manualPublicRecoveryAbsent(t, path+suffix)
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 0 {
			t.Fatalf("NoCreate created an unexpected entry: %v %v", entries, err)
		}
		manualPublicRecoveryNoPending(t)
	}) {
		return
	}
}

// This deterministic two-page fixture follows the already documented internal
// recovery fixture: empty sqlite_schema plus an empty freelist trunk. The live
// image has one extra page and different header/trunk bytes; this is prepared
// input, not an interrupted process or a durability simulation.
func manualPublicRecoveryFiles(t *testing.T, root string) (string, []byte, []byte, []byte) {
	t.Helper()
	dir, err := os.MkdirTemp(root, "public-rollback-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("retained isolated directory: %s", dir)
	original := make([]byte, 1024)
	copy(original, "SQLite format 3\x00")
	binary.BigEndian.PutUint16(original[16:], 512)
	original[18], original[19], original[21], original[22], original[23] = 1, 1, 64, 32, 32
	binary.BigEndian.PutUint32(original[24:], 7)
	binary.BigEndian.PutUint32(original[28:], 2)
	binary.BigEndian.PutUint32(original[32:], 2)
	binary.BigEndian.PutUint32(original[36:], 1)
	binary.BigEndian.PutUint32(original[44:], 4)
	binary.BigEndian.PutUint32(original[56:], 1)
	binary.BigEndian.PutUint32(original[92:], 7)
	original[100] = 13
	binary.BigEndian.PutUint16(original[105:], 512)
	copy(original[512:], bytes.Repeat([]byte{0x12}, 512))
	clear(original[512:520])
	current := append(append([]byte(nil), original...), bytes.Repeat([]byte{0x34}, 512)...)
	binary.BigEndian.PutUint32(current[24:], 8)
	binary.BigEndian.PutUint32(current[28:], 3)
	binary.BigEndian.PutUint32(current[92:], 8)
	current[700] ^= 0x7f
	h := journal.Header{Records: 2, Nonce: 0x12345678, OriginalPages: 2, SectorSize: 512, PageSize: 512}
	data, err := journal.EncodeHeader(h, true)
	if err != nil {
		t.Fatal(err)
	}
	for n := uint32(1); n <= 2; n++ {
		record, err := journal.EncodeRecord(h, n, original[(n-1)*512:n*512])
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, record...)
	}
	inputs := map[string][]byte{"original.input.db": original, "current.input.db": current, "journal.input.bin": data}
	hashes := make(map[string]string, len(inputs))
	for name, contents := range inputs {
		manualPublicRecoveryWrite(t, filepath.Join(dir, name), contents)
		hashes[name] = fmt.Sprintf("%x", sha256.Sum256(contents))
	}
	manifest, err := json.MarshalIndent(hashes, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	manifest = append(manifest, '\n')
	manualPublicRecoveryWrite(t, filepath.Join(dir, "input-sha256.json"), manifest)
	inputs["input-sha256.json"] = manifest
	t.Cleanup(func() {
		for _, name := range []string{"original.input.db", "current.input.db", "journal.input.bin", "input-sha256.json"} {
			want := inputs[name]
			got, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil || !bytes.Equal(got, want) {
				t.Errorf("retained input %s: error=%v gotSHA256=%x wantSHA256=%x", name, err, sha256.Sum256(got), sha256.Sum256(want))
			}
		}
	})
	path := filepath.Join(dir, "recovery.db")
	manualPublicRecoveryWrite(t, path, current)
	manualPublicRecoveryWrite(t, path+"-journal", data)
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(f.Sync(), f.Close()); err != nil {
		t.Fatalf("fixture directory publication: %v", err)
	}
	return path, original, current, data
}

func manualPublicRecoveryWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	n, writeErr := f.Write(data)
	if n != len(data) && writeErr == nil {
		writeErr = io.ErrShortWrite
	}
	if err := errors.Join(writeErr, f.Sync(), f.Close()); err != nil {
		t.Fatalf("retain fixture %s: %v", path, err)
	}
}

func manualPublicRecoveryEqual(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("byte assertion %s: error=%v gotSHA256=%x wantSHA256=%x", path, err, sha256.Sum256(got), sha256.Sum256(want))
	}
}

func manualPublicRecoveryAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected absence %s: %v", path, err)
	}
}

func manualPublicRecoveryNoPending(t *testing.T) {
	t.Helper()
	if pending := sqlite.PendingPlaintextCleanups(); len(pending) != 0 {
		t.Fatalf("pending owners retained; do not delete fixtures or retry implicitly: %v", pending)
	}
}

func manualPublicRecoveryRefusal(t *testing.T, db *sqlite.DB, err error, causes ...error) {
	t.Helper()
	if db != nil {
		closeErr := db.Close()
		t.Fatalf("expected refusal returned DB: open=%v close=%v", err, closeErr)
	}
	var diagnostic *sqlite.Error
	if !errors.As(err, &diagnostic) || diagnostic.Code != sqlite.CodeIOErr {
		t.Fatalf("expected public CodeIOErr: %v", err)
	}
	for _, cause := range causes {
		if !errors.Is(err, cause) {
			t.Fatalf("missing public cause %v: %v", cause, err)
		}
	}
}

func manualPublicRecoveryOwnDB(t *testing.T, db *sqlite.DB, err error) func() bool {
	t.Helper()
	if err != nil || db == nil {
		if db != nil {
			err = errors.Join(err, db.Close())
		}
		t.Fatalf("expected public Open success: %v", err)
	}
	closed := false
	var closeErr error
	closeDB := func() bool {
		if closed {
			return closeErr == nil
		}
		closed = true
		closeErr = db.Close()
		if closeErr != nil {
			t.Errorf("Close failed; retain fixtures and diagnostics: %v", closeErr)
		}
		return closeErr == nil
	}
	t.Cleanup(func() { closeDB() })
	return closeDB
}
