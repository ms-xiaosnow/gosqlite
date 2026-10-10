//go:build linux

package vfs

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func openNativeSHMTest(t *testing.T, path string) *File {
	t.Helper()
	f, err := Open(path, OpenOptions{Role: SHM, Flags: os.O_RDWR | os.O_CREATE})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	if err = f.AttachDMS(); err != nil {
		t.Fatal(err)
	}
	return f
}
func lockOFDTest(t *testing.T, f *File, slot, n int, exclusive bool) {
	t.Helper()
	if err := nativeSHMLock(f, slot, n, exclusive, false); err != nil {
		t.Fatal(err)
	}
}

func TestIndependentOFDLocksAndPOSIXInterop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db-shm")
	x := openNativeSHMTest(t, path)
	y := openNativeSHMTest(t, path)
	lockOFDTest(t, x, 0, 1, true)
	if err := nativeSHMLock(y, 0, 1, true, false); !errors.Is(err, ErrBusy) {
		t.Fatalf("second writer: %v", err)
	}
	// Closing an unrelated descriptor must not drop x's open-description locks.
	extra, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = extra.Close(); err != nil {
		t.Fatal(err)
	}
	if err = nativeSHMLock(y, 0, 1, true, false); !errors.Is(err, ErrBusy) {
		t.Fatalf("writer released by unrelated close: %v", err)
	}
	if err = nativeSHMLock(x, 0, 1, false, true); err != nil {
		t.Fatal(err)
	}
	lockOFDTest(t, x, 4, 1, false)
	lockOFDTest(t, y, 4, 1, false)
	if err = nativeSHMLock(y, 4, 1, true, false); !errors.Is(err, ErrBusy) {
		t.Fatalf("exclusive read mark: %v", err)
	}
	if err = nativeSHMLock(x, 4, 1, false, true); err != nil {
		t.Fatal(err)
	}
	lockOFDTest(t, y, 4, 1, true)
	if err = nativeSHMLock(y, 4, 1, false, true); err != nil {
		t.Fatal(err)
	}
	native, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer native.Close()
	l := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: 0, Start: 120, Len: 1}
	if err = syscall.FcntlFlock(native.Fd(), syscall.F_SETLK, &l); err != nil {
		t.Fatal(err)
	}
	if err = nativeSHMLock(x, 0, 1, true, false); !errors.Is(err, ErrBusy) {
		t.Fatalf("OFD did not conflict with POSIX: %v", err)
	}
	l.Type = syscall.F_UNLCK
	if err = syscall.FcntlFlock(native.Fd(), syscall.F_SETLK, &l); err != nil {
		t.Fatal(err)
	}
	lockOFDTest(t, x, 0, 1, true)
	l.Type = syscall.F_WRLCK
	if err = syscall.FcntlFlock(native.Fd(), syscall.F_SETLK, &l); !errors.Is(err, syscall.EAGAIN) && !errors.Is(err, syscall.EACCES) {
		t.Fatalf("POSIX did not conflict with OFD: %v", err)
	}
	l.Type = syscall.F_WRLCK
	l.Start = 128
	if err = syscall.FcntlFlock(native.Fd(), syscall.F_SETLK, &l); !errors.Is(err, syscall.EAGAIN) && !errors.Is(err, syscall.EACCES) {
		t.Fatalf("DMS not held shared: %v", err)
	}
}

func TestOFDSubrangeAndUnionUnlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db-shm")
	x, y := openNativeSHMTest(t, path), openNativeSHMTest(t, path)
	lockOFDTest(t, x, 0, 8, true)
	if err := nativeSHMLock(x, 1, 7, false, true); err != nil {
		t.Fatal(err)
	}
	if err := nativeSHMLock(y, 0, 1, true, false); !errors.Is(err, ErrBusy) {
		t.Fatalf("subrange released slot zero: %v", err)
	}
	lockOFDTest(t, y, 1, 7, true)
	if err := nativeSHMLock(y, 1, 7, false, true); err != nil {
		t.Fatal(err)
	}
	lockOFDTest(t, x, 1, 3, true)
	lockOFDTest(t, x, 4, 4, true)
	if err := nativeSHMLock(x, 0, 8, false, true); err != nil {
		t.Fatal(err)
	}
	lockOFDTest(t, y, 0, 8, true)
}

func TestManagedDBProbeRetainsReservedAndSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ordinary.db")
	writer, err := Open(path, OpenOptions{Role: Database, Flags: os.O_RDWR | os.O_CREATE})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if err = writer.LockDB(Shared); err != nil {
		t.Fatal(err)
	}
	if err = writer.LockDB(Reserved); err != nil {
		t.Fatal(err)
	}
	probe, err := Open(path, OpenOptions{Role: Database})
	if err != nil {
		t.Fatal(err)
	}
	if err = probe.LockDB(Shared); err != nil {
		t.Fatal(err)
	}
	if err = probe.Close(); err != nil {
		t.Fatal(err)
	}
	peer, err := Open(path, OpenOptions{Role: Database, Flags: os.O_RDWR})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	if err = peer.LockDB(Shared); err != nil {
		t.Fatal(err)
	}
	if err = peer.LockDB(Reserved); !errors.Is(err, ErrBusy) {
		t.Fatalf("probe released reservation: %v", err)
	}
	if held, err := peer.ReservedHeld(); err != nil || !held {
		t.Fatalf("reserved probe: %t %v", held, err)
	}
	if err = writer.LockDB(Exclusive); !errors.Is(err, ErrBusy) {
		t.Fatalf("exclusive with peer reader: %v", err)
	}
	// The failed upgrade must leave SHARED and release temporary PENDING.
	another, err := Open(path, OpenOptions{Role: Database})
	if err != nil {
		t.Fatal(err)
	}
	if err = another.LockDB(Shared); err != nil {
		t.Fatal(err)
	}
	if err = another.Close(); err != nil {
		t.Fatal(err)
	}
	if err = peer.Close(); err != nil {
		t.Fatal(err)
	}
	if err = writer.LockDB(Exclusive); err != nil {
		t.Fatal(err)
	}
	if err = writer.DowngradeDBToShared(); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestManagedMappingLogicalSpanAndClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ordinary-shm")
	f := openNativeSHMTest(t, path)
	if err := f.LockSHM(0, 1, true); err != nil {
		t.Fatal(err)
	}
	// Normal backing allocation, without truncating or relocating live views.
	if _, err := f.WriteAt([]byte{0}, 32767); err != nil {
		t.Fatal(err)
	}
	a, err := f.MapShared(0, 32768)
	if err != nil {
		t.Fatal(err)
	}
	a.Bytes[137] = 42
	if _, err = f.WriteAt([]byte{0}, 65535); err != nil {
		t.Fatal(err)
	}
	b, err := f.MapShared(32768, 32768)
	if err != nil {
		t.Fatal(err)
	}
	b.Bytes[0] = 17
	if a.Bytes[137] != 42 {
		t.Fatal("growth relocated live mapping")
	}
	peer := openNativeSHMTest(t, path)
	c, err := peer.MapShared(0, 32768)
	if err != nil {
		t.Fatal(err)
	}
	if c.Bytes[137] != 42 {
		t.Fatal("mapped views are not shared")
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if a.Bytes != nil || b.Bytes != nil {
		t.Fatal("closed owner retained live bytes")
	}
	if c.Bytes[137] != 42 {
		t.Fatal("peer close invalidated mapping")
	}
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
}
