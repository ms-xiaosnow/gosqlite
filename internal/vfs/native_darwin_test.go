//go:build darwin

package vfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// These are ordinary intact-file tests for a future approved native host.
// Cross-compiling them does not establish Darwin runtime acceptance. No raw
// descriptor of a managed inode is opened/closed by these tests.
func openDarwinTest(t *testing.T, path string, role Role, flags int) *File {
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

func TestDarwinManagedProbesKeepInodeLocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ordinary.db")
	writer := openDarwinTest(t, path, Database, os.O_RDWR|os.O_CREATE)
	if err := writer.LockDB(Shared); err != nil {
		t.Fatal(err)
	}
	if err := writer.LockDB(Reserved); err != nil {
		t.Fatal(err)
	}
	// A long-lived writer must not accumulate one fd for each short probe.
	for i := 0; i < 1000; i++ {
		probe := openDarwinTest(t, path, Database, os.O_RDONLY)
		if err := probe.LockDB(Shared); err != nil {
			t.Fatal(err)
		}
		if err := probe.Close(); err != nil {
			t.Fatal(err)
		}
	}
	darwinRegistry.Lock()
	n := writer.native.descriptor.node
	count := len(n.descriptors)
	reserved := n.kernel[darwinReserved]
	darwinRegistry.Unlock()
	if count != 2 || reserved != darwinWrite {
		t.Fatalf("probe cache/aggregate changed: descriptors=%d reserved=%d", count, reserved)
	}
	peer := openDarwinTest(t, path, Database, os.O_RDWR)
	if err := peer.LockDB(Shared); err != nil {
		t.Fatal(err)
	}
	if held, err := peer.ReservedHeld(); err != nil || !held {
		t.Fatalf("same-process reservation query: %t %v", held, err)
	}
	if err := peer.LockDB(Reserved); !errors.Is(err, ErrBusy) {
		t.Fatalf("second local reservation: %v", err)
	}
	if err := writer.LockDB(Exclusive); !errors.Is(err, ErrBusy) {
		t.Fatalf("upgrade while a peer reads: %v", err)
	}
	later := openDarwinTest(t, path, Database, os.O_RDONLY)
	if err := later.LockDB(Shared); err != nil {
		t.Fatalf("failed upgrade retained temporary PENDING: %v", err)
	}
	if err := later.Close(); err != nil {
		t.Fatal(err)
	}
	if err := peer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writer.LockDB(Exclusive); err != nil {
		t.Fatal(err)
	}
	if err := writer.DowngradeDBToShared(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	darwinRegistry.Lock()
	remaining := len(n.descriptors)
	_, exists := darwinRegistry.nodes[n.identity]
	darwinRegistry.Unlock()
	if remaining != 0 || exists {
		t.Fatal("last owner failed to drain deferred physical descriptors")
	}
}

func TestDarwinSharedDMSSlotsAndMappingLifetime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ordinary.db-shm")
	a := openDarwinTest(t, path, SHM, os.O_RDWR|os.O_CREATE)
	if err := a.AttachDMS(); err != nil {
		t.Fatal(err)
	}
	if err := a.LockSHM(0, 1, true); err != nil {
		t.Fatal(err)
	}
	if _, err := a.WriteAt([]byte{0}, 32767); err != nil {
		t.Fatal(err)
	}
	m, err := a.MapShared(0, 32768)
	if err != nil {
		t.Fatal(err)
	}
	m.Bytes[136] = 23
	b := openDarwinTest(t, path, SHM, os.O_RDWR|os.O_CREATE)
	if err := b.AttachDMS(); err != nil {
		t.Fatal(err)
	}
	if info, err := b.Stat(); err != nil || info.Size() != 32768 {
		t.Fatalf("local attach reset live mapped index: %v %v", info, err)
	}
	other, err := b.MapShared(0, 32768)
	if err != nil || other.Bytes[136] != 23 {
		t.Fatalf("shared mapped bytes: %v", err)
	}
	if err := b.LockSHM(0, 1, true); !errors.Is(err, ErrBusy) {
		t.Fatalf("concurrent WRITE: %v", err)
	}
	if err := a.UnlockSHM(0, 1); err != nil {
		t.Fatal(err)
	}
	if err := a.LockSHM(4, 1, false); err != nil {
		t.Fatal(err)
	}
	if err := b.LockSHM(4, 1, false); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if m.Bytes != nil || other.Bytes[136] != 23 {
		t.Fatal("owner close violated shared view lifetime")
	}
	if err := b.UnlockSHM(4, 1); err != nil {
		t.Fatal(err)
	}
	if err := b.LockSHM(1, 7, true); err != nil {
		t.Fatal(err)
	}
	if err := b.UnlockSHM(2, 2); err != nil {
		t.Fatal(err)
	}
	if err := b.UnlockSHM(1, 7); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil || other.Bytes != nil {
		t.Fatalf("mapping retirement: %v", err)
	}
}

func TestDarwinAggregateOwnershipMemoryModel(t *testing.T) {
	// No descriptor, file, native operation, or synthetic native error. This
	// exercises the exact aggregate helper used before process fcntl calls.
	n := &darwinNode{descriptors: make(map[*os.File]*darwinDescriptor)}
	a, b := &os.File{}, &os.File{}
	x := &darwinDescriptor{holds: map[darwinRange]uint8{darwinShared: darwinRead, darwinReserved: darwinWrite}}
	y := &darwinDescriptor{holds: map[darwinRange]uint8{darwinShared: darwinRead}}
	n.descriptors[a], n.descriptors[b] = x, y
	if darwinAggregateExcept(n, x, darwinShared) != darwinRead || darwinAggregateExcept(n, y, darwinReserved) != darwinWrite {
		t.Fatal("aggregate omitted a sibling's holds")
	}
	delete(y.holds, darwinShared)
	if darwinAggregateExcept(n, x, darwinShared) != 0 || darwinAggregateExcept(n, y, darwinShared) != darwinRead {
		t.Fatal("releasing one owner changed its sibling")
	}
}
