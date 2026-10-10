//go:build windows && amd64

package vfs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These are ordinary cooperating native checks. They are compiled on the
// Linux development host, never run through an emulator or claimed as native
// acceptance. No replacement, fault, interruption, or recovery is exercised.
func openWindowsTest(t *testing.T, path string, role Role, flags int) *File {
	t.Helper()
	f, err := Open(path, OpenOptions{Role: role, Flags: flags})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	return f
}
func windowsLockTest(t *testing.T, f *File, lock DBLock) {
	t.Helper()
	if err := f.LockDB(lock); err != nil {
		t.Fatal(err)
	}
}
func windowsAttachTest(t *testing.T, f *File) {
	t.Helper()
	if err := f.AttachDMS(); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsDBLockTransitionsAndProbeRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db")
	writer := openWindowsTest(t, path, Database, os.O_RDWR|os.O_CREATE)
	reader := openWindowsTest(t, path, Database, os.O_RDWR)
	windowsLockTest(t, writer, Shared)
	windowsLockTest(t, reader, Shared)
	windowsLockTest(t, writer, Reserved)
	probe := openWindowsTest(t, path, Database, os.O_RDONLY)
	held, err := probe.ReservedHeld()
	if err != nil || !held {
		t.Fatalf("read-only reserved probe: held=%t err=%v", held, err)
	}
	if err = probe.Close(); err != nil {
		t.Fatal(err)
	}
	if err = reader.LockDB(Reserved); !errors.Is(err, ErrBusy) {
		t.Fatalf("probe close lost writer reservation: %v", err)
	}
	if err = writer.LockDB(Exclusive); !errors.Is(err, ErrBusy) {
		t.Fatalf("exclusive with active peer: %v", err)
	}
	if writer.native.held[winShared] != 1 || writer.native.held[winPending] != 0 || writer.native.held[winReserved] != 2 || writer.native.poisoned {
		t.Fatal("unsuccessful upgrade did not preserve SHARED and RESERVED")
	}
	if err = reader.Close(); err != nil {
		t.Fatal(err)
	}
	windowsLockTest(t, writer, Exclusive)
	if writer.native.held[winShared] != 2 || writer.native.held[winPending] != 2 {
		t.Fatal("exclusive acquisition lacks exact range or PENDING ownership")
	}
	newReader := openWindowsTest(t, path, Database, os.O_RDONLY)
	if err = newReader.LockDB(Shared); !errors.Is(err, ErrBusy) {
		t.Fatalf("new reader entered PENDING gate: %v", err)
	}
	if err = writer.DowngradeDBToShared(); err != nil {
		t.Fatal(err)
	}
	windowsLockTest(t, newReader, Shared)
	if writer.native.held[winReserved] != 2 || writer.native.held[winPending] != 0 || writer.native.held[winShared] != 1 {
		t.Fatal("downgrade discarded reservation or failed to restore shared mode")
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	if held, err = newReader.ReservedHeld(); err != nil || held {
		t.Fatalf("close retained reservation: held=%t err=%v", held, err)
	}
}

func TestWindowsDirectSharedExclusiveTransition(t *testing.T) {
	f := openWindowsTest(t, filepath.Join(t.TempDir(), "db"), Database, os.O_RDWR|os.O_CREATE)
	windowsLockTest(t, f, Shared)
	windowsLockTest(t, f, Exclusive)
	if f.native.held[winPending] != 2 || f.native.held[winShared] != 2 || f.native.held[winReserved] != 0 {
		t.Fatal("direct upgrade did not acquire PENDING independently of RESERVED")
	}
	windowsLockTest(t, f, Exclusive) // Must not stack another Windows lock.
	if err := f.DowngradeDBToShared(); err != nil {
		t.Fatal(err)
	}
	if f.native.held[winPending] != 0 || f.native.held[winShared] != 1 {
		t.Fatal("direct downgrade left extra lock acquisitions")
	}
}

func TestWindowsSHMExactIntervalsRollbackAndUnion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db-shm")
	a := openWindowsTest(t, path, SHM, os.O_RDWR|os.O_CREATE)
	b := openWindowsTest(t, path, SHM, os.O_RDWR)
	windowsAttachTest(t, a)
	windowsAttachTest(t, b)
	if err := a.LockSHM(1, 1, true); err != nil {
		t.Fatal(err)
	}
	if err := b.LockSHM(4, 1, true); err != nil {
		t.Fatal(err)
	}
	if err := a.LockSHM(1, 7, true); !errors.Is(err, ErrBusy) {
		t.Fatalf("multi-slot collision: %v", err)
	}
	for i := 0; i < 8; i++ {
		want := uint8(0)
		if i == 1 {
			want = 2
		}
		if a.native.held[winSlot0+i] != want || a.slots[i] != want {
			t.Fatalf("rollback changed prior ownership: slot=%d", i)
		}
	}
	if err := b.LockSHM(2, 2, true); err != nil {
		t.Fatalf("partial acquisition was not rolled back: %v", err)
	}
	if err := b.UnlockSHM(2, 3); err != nil { // Union of originally distinct intervals.
		t.Fatal(err)
	}
	if err := a.LockSHM(1, 7, true); err != nil {
		t.Fatal(err)
	}
	if err := a.UnlockSHM(2, 3); err != nil { // Strict subrange.
		t.Fatal(err)
	}
	if err := b.LockSHM(2, 3, true); err != nil {
		t.Fatal(err)
	}
	if err := b.LockSHM(1, 1, true); !errors.Is(err, ErrBusy) {
		t.Fatalf("subrange release escaped its interval: %v", err)
	}
	if err := a.UnlockSHM(0, 8); err != nil {
		t.Fatal(err)
	}
	if err := b.UnlockSHM(0, 8); err != nil {
		t.Fatal(err)
	}
	for _, f := range []*File{a, b} {
		if err := f.LockSHM(6, 1, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.LockSHM(6, 1, false); err != nil { // No stacking.
		t.Fatal(err)
	}
	if err := a.LockSHM(6, 1, true); !errors.Is(err, ErrLock) {
		t.Fatalf("held-mode conversion accepted: %v", err)
	}
	if err := a.UnlockSHM(6, 1); err != nil {
		t.Fatal(err)
	}
	if err := b.UnlockSHM(6, 1); err != nil {
		t.Fatal(err)
	}
	if err := b.LockSHM(6, 1, true); err != nil {
		t.Fatalf("same-mode idempotence stacked a physical hold: %v", err)
	}
}

func TestWindowsMappingGranularityAndDMSLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db-shm")
	a := openWindowsTest(t, path, SHM, os.O_RDWR|os.O_CREATE)
	windowsAttachTest(t, a)
	if info, err := a.Stat(); err != nil || info.Size() != 0 {
		t.Fatalf("first Windows attachment must start at zero: info=%v err=%v", info, err)
	}
	if _, err := a.WriteAt([]byte{0}, 32767); err != nil {
		t.Fatal(err)
	}
	first, err := a.MapShared(0, 32768)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Bytes) != 32768 || first.native.length != 32768 || first.native.base == 0 {
		t.Fatal("logical first block was rounded up or lacks original base")
	}
	first.Bytes[160] = 37 // Never access reserved lock-slot bytes 120..127.
	originalBase := first.native.base
	b := openWindowsTest(t, path, SHM, os.O_RDWR)
	windowsAttachTest(t, b)
	if info, err := b.Stat(); err != nil || info.Size() != 32768 {
		t.Fatalf("second local DMS attachment reset live content: info=%v err=%v", info, err)
	}
	peer, err := b.MapShared(0, 32768)
	if err != nil || peer.Bytes[160] != 37 {
		t.Fatalf("first view coherence: err=%v", err)
	}
	if _, err = a.WriteAt([]byte{0}, 65535); err != nil {
		t.Fatal(err)
	}
	second, err := a.MapShared(32768, 32768)
	if err != nil {
		t.Fatal(err)
	}
	_, delta, length, err := mappingSpan(32768, 32768, nativeMapGranularity())
	if err != nil || len(second.Bytes) != 32768 || second.native.length != length || delta+len(second.Bytes) != length || first.native.base != originalBase {
		t.Fatal("mapping moved a live view or lost the logical subspan")
	}
	second.Bytes[160] = 91
	other, err := b.MapShared(32768, 32768)
	if err != nil || other.Bytes[160] != 91 {
		t.Fatalf("second view coherence: err=%v", err)
	}
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	if first.Bytes != nil || second.Bytes != nil || peer.Bytes[160] != 37 || other.Bytes[160] != 91 {
		t.Fatal("closing one lease changed another lease's views")
	}
	c := openWindowsTest(t, path, SHM, os.O_RDWR)
	windowsAttachTest(t, c)
	if info, err := c.Stat(); err != nil || info.Size() != 65536 {
		t.Fatalf("surviving DMS owner was not respected: info=%v err=%v", info, err)
	}
	if err = b.Close(); err != nil {
		t.Fatal(err)
	}
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
	d := openWindowsTest(t, path, SHM, os.O_RDWR)
	windowsAttachTest(t, d)
	if info, err := d.Stat(); err != nil || info.Size() != 0 {
		t.Fatalf("new first attachment did not reset transient index: info=%v err=%v", info, err)
	}
}

func TestWindowsHandleIdentityAndSyncPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db")
	a := openWindowsTest(t, path, Database, os.O_RDWR|os.O_CREATE)
	b := openWindowsTest(t, path, Database, os.O_RDONLY)
	c := openWindowsTest(t, filepath.Join(dir, "other"), Database, os.O_RDWR|os.O_CREATE)
	if a.native.id != b.native.id || a.native.id == c.native.id || a.native.parentID != c.native.parentID || !strings.HasPrefix(a.native.canonical, `\\?\`) {
		t.Fatal("handle identity or retained canonical namespace differs")
	}
	for _, f := range []*File{a, b, c} {
		if err := f.ValidatePath(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.WriteAt([]byte("ordinary native sync"), 0); err != nil {
		t.Fatal(err)
	}
	if err := a.Sync(); err != nil {
		t.Fatal(err)
	}
	// This only checks the explicit no-directory-barrier policy; it is not
	// evidence of durable create/delete/rename namespace ordering.
	if err := SyncDirectory(dir); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsPathPolicyPureValidation(t *testing.T) {
	for _, path := range []string{`C:\db.sqlite`, `c:\dir\名字.sqlite`, `\\?\C:\dir\db.sqlite`, `C:\`} {
		if err := winPath(path); err != nil {
			t.Fatalf("valid drive path %q: %v", path, err)
		}
	}
	for _, path := range []string{`C:\db:stream`, `C:\db::$DATA`, `\\server\share\db`, `\\?\UNC\server\share\db`, `\\.\C:`, `C:db`, `C:\AUX`, `C:\COM1.txt`, `C:\LPT¹`, `C:\dir.\db`, `C:\dir \db`, `C:\..\db`, `C:\x?\db`} {
		if err := winPath(path); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("unsafe path %q accepted: %v", path, err)
		}
	}
}
