//go:build linux

package walindex

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
)

func openTestIndex(t *testing.T, path string) *Index {
	t.Helper()
	x, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := x.Close(); err != nil {
			t.Error(err)
		}
	})
	return x
}
func lockTest(t *testing.T, x *Index, slot, n int, exclusive bool) {
	t.Helper()
	if err := x.Lock(slot, n, exclusive); err != nil {
		t.Fatal(err)
	}
}
func initTest(t *testing.T, x *Index, mx uint32) {
	t.Helper()
	lockTest(t, x, 0, 8, true)
	if err := x.InitCheckpoint(mx); err != nil {
		t.Fatal(err)
	}
	if err := x.Unlock(1, 7); err != nil {
		t.Fatal(err)
	}
}

func TestDMSNormalLifecycleAndHeaderPublication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db-shm")
	x := openTestIndex(t, path)
	info, err := os.Stat(path)
	if err != nil || info.Size() != 3 {
		t.Fatalf("first-opener size: %v %v", info, err)
	}
	if _, err = x.ReadHeader(); !errors.Is(err, ErrUninitialized) {
		t.Fatalf("empty header: %v", err)
	}
	if err = x.Publish(Header{PageSize: 4096}); !errors.Is(err, ErrLock) {
		t.Fatalf("unlocked publication: %v", err)
	}
	initTest(t, x, 0)
	h := Header{Version: Version, PageSize: 4096, Change: 3, Unused: 4}
	if err = x.Publish(h); err != nil {
		t.Fatal(err)
	}
	canonical, err := x.ReadHeader()
	if err != nil {
		t.Fatal(err)
	}
	y := openTestIndex(t, path)
	got, err := y.ReadHeader()
	if err != nil || got != canonical {
		t.Fatalf("second opener changed header: %+v %v", got, err)
	}
	if err = y.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err = x.ReadHeader(); err != nil || got != canonical {
		t.Fatalf("other close changed index: %+v %v", got, err)
	}
	if err = x.Close(); err != nil {
		t.Fatal(err)
	}
	z := openTestIndex(t, path)
	info, err = os.Stat(path)
	if err != nil || info.Size() != 3 {
		t.Fatalf("clean reopen did not reinitialize: %v %v", info, err)
	}
	if _, err = z.ReadHeader(); !errors.Is(err, ErrUninitialized) {
		t.Fatalf("reopened header: %v", err)
	}
}

func TestDMSEarlierExclusiveOpenerReturnsBusy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db-shm")
	first := openTestIndex(t, path)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	native, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer native.Close()
	l := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: 0, Start: 128, Len: 1}
	if err = syscall.FcntlFlock(native.Fd(), syscall.F_SETLK, &l); err != nil {
		t.Fatal(err)
	}
	if got, err := Open(path); !errors.Is(err, ErrBusy) {
		if got != nil {
			_ = got.Close()
		}
		t.Fatalf("concurrent first opener: %v", err)
	}
	info, err := native.Stat()
	if err != nil || info.Size() != 3 {
		t.Fatalf("busy opener changed size: %v %v", info, err)
	}
}

func TestRealHashBlocksSnapshotAndCleanup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db-shm")
	x := openTestIndex(t, path)
	initTest(t, x, 0)
	const frames = 8300
	pageFor := func(f uint32) uint32 { return 1 + (f%127)*8192 } // all collide
	for f := uint32(1); f <= frames; f++ {
		if err := x.Append(f, pageFor(f)); err != nil {
			t.Fatalf("frame %d: %v", f, err)
		}
	}
	if err := x.Publish(Header{PageSize: 4096, MaxFrame: frames, DBSize: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	y := openTestIndex(t, path)
	lockTest(t, y, ReadLock0+1, 1, false)
	for _, window := range [][2]uint32{{1, 4062}, {1, 4063}, {1, 8158}, {1, 8159}, {3900, 4200}, {8130, 8300}} {
		expected := map[uint32]uint32{}
		for f := window[0]; f <= window[1]; f++ {
			expected[pageFor(f)] = f
		}
		for page, want := range expected {
			got, err := y.Lookup(page, window[0], window[1])
			if err != nil || got != want {
				t.Fatalf("lookup %d [%d,%d] got %d want%d: %v", page, window[0], window[1], got, want, err)
			}
		}
	}
	for _, f := range []uint32{1, 4062, 4063, 8158, 8159, 8300} {
		got, err := y.Page(f)
		if err != nil || got != pageFor(f) {
			t.Fatalf("page %d got%d: %v", f, got, err)
		}
	}
	if got, err := y.Lookup(17, 1, frames); err != nil || got != 0 {
		t.Fatalf("missing page: %d %v", got, err)
	}
	for f := uint32(frames + 1); f <= frames+20; f++ {
		if err := x.Append(f, pageFor(1)); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := y.Lookup(pageFor(1), 1, frames); err != nil || got > frames {
		t.Fatalf("snapshot included new frame %d: %v", got, err)
	}
	if err := x.Cleanup(frames); err != nil {
		t.Fatal(err)
	}
	if err := x.Append(frames+1, 42); err != nil {
		t.Fatal(err)
	}
	if got, err := y.Lookup(42, 1, frames); err != nil || got != 0 {
		t.Fatalf("unpublished frame escaped bounds: %d %v", got, err)
	}
	if got, err := y.Lookup(42, 1, frames+1); err != nil || got != frames+1 {
		t.Fatalf("replacement uncommitted entry: %d %v", got, err)
	}
}

func TestCheckpointMetadataAndLimits(t *testing.T) {
	x := openTestIndex(t, filepath.Join(t.TempDir(), "db-shm"))
	lockTest(t, x, 0, 1, true)
	if err := x.InitCheckpoint(8); !errors.Is(err, ErrLock) {
		t.Fatalf("init without exclusive marks: %v", err)
	}
	lockTest(t, x, 1, 7, true)
	if err := x.InitCheckpoint(8); err != nil {
		t.Fatal(err)
	}
	if v, err := x.Backfill(); err != nil || v != 0 {
		t.Fatalf("backfill %d %v", v, err)
	}
	if v, err := x.Attempted(); err != nil || v != 8 {
		t.Fatalf("attempted %d %v", v, err)
	}
	for mark := 0; mark < ReaderCount; mark++ {
		want := UnusedReadMark
		if mark == 0 {
			want = 0
		}
		if mark == 1 {
			want = 8
		}
		v, err := x.ReadMark(mark)
		if err != nil || v != want {
			t.Fatalf("mark%d %d want%d %v", mark, v, want, err)
		}
	}
	if err := x.SetBackfill(3); err != nil {
		t.Fatal(err)
	}
	if err := x.SetAttempted(7); err != nil {
		t.Fatal(err)
	}
	if err := x.SetReadMark(2, 6); err != nil {
		t.Fatal(err)
	}
	if err := x.Unlock(0, 8); err != nil {
		t.Fatal(err)
	}
	if err := x.SetBackfill(4); !errors.Is(err, ErrLock) {
		t.Fatalf("unlocked backfill: %v", err)
	}
	if err := x.SetReadMark(2, 7); !errors.Is(err, ErrLock) {
		t.Fatalf("unlocked mark: %v", err)
	}
	lockTest(t, x, CheckpointLock, 1, true)
	if err := x.SetBackfill(4); err != nil {
		t.Fatal(err)
	}
	lockTest(t, x, 0, 1, true)
	if err := x.Append(DefaultMaxFrames+1, 1); !errors.Is(err, ErrLimit) {
		t.Fatalf("append limit: %v", err)
	}
	if err := x.Publish(Header{PageSize: 4096, MaxFrame: DefaultMaxFrames + 1, DBSize: 1}); !errors.Is(err, ErrLimit) {
		t.Fatalf("header limit: %v", err)
	}
	if _, err := x.Lookup(1, 1, DefaultMaxFrames+1); !errors.Is(err, ErrLimit) {
		t.Fatalf("lookup limit: %v", err)
	}
}

func TestConcurrentHeaderPublication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db-shm")
	x := openTestIndex(t, path)
	initTest(t, x, 0)
	if err := x.Publish(Header{PageSize: 4096}); err != nil {
		t.Fatal(err)
	}
	y := openTestIndex(t, path)
	var wg sync.WaitGroup
	failures := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := uint32(1); i <= 2000; i++ {
			h := Header{Change: i, PageSize: 4096, Unused: i ^ 0xa5a5a5a5}
			if err := x.Publish(h); err != nil {
				failures <- err
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 4000; i++ {
			h, err := y.ReadHeader()
			if errors.Is(err, ErrHeader) {
				continue
			}
			if err != nil {
				failures <- err
				return
			}
			if h.Change != 0 && h.Unused != h.Change^0xa5a5a5a5 {
				failures <- fmt.Errorf("torn accepted header %+v", h)
				return
			}
		}
	}()
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	h, err := y.ReadHeader()
	if err != nil || h.Change != 2000 {
		t.Fatalf("final header %+v: %v", h, err)
	}
}

func TestNativeHashAndPageMappingBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db-shm")
	x := openTestIndex(t, path)
	initTest(t, x, 0)
	if err := x.Publish(Header{PageSize: 4096, Change: 19}); err != nil {
		t.Fatal(err)
	}
	before, err := x.ReadHeader()
	if err != nil {
		t.Fatal(err)
	}
	if err = x.Append(1, 1); err != nil {
		t.Fatal(err)
	}
	if err = x.Append(2, 8193); err != nil {
		t.Fatal(err)
	}
	after, err := x.ReadHeader()
	if err != nil || before != after {
		t.Fatalf("first block clear damaged header: %+v %v", after, err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var bytes [BlockSize]byte
	if _, err = file.ReadAt(bytes[:], 0); err != nil {
		t.Fatal(err)
	}
	if binary.NativeEndian.Uint32(bytes[136:]) != 1 || binary.NativeEndian.Uint32(bytes[140:]) != 8193 {
		t.Fatal("incorrect native page-map bytes")
	}
	// Both pages collide at key383. These halfwords straddle a 32-bit-word
	// boundary, so test both high and low halves independently of loadHash.
	if binary.NativeEndian.Uint16(bytes[16384+383*2:]) != 1 || binary.NativeEndian.Uint16(bytes[16384+384*2:]) != 2 {
		t.Fatal("incorrect native uint16 hash slots")
	}
	if binary.NativeEndian.Uint16(bytes[16384+382*2:]) != 0 || binary.NativeEndian.Uint16(bytes[16384+385*2:]) != 0 {
		t.Fatal("hash publication changed an adjacent halfword")
	}
}

func TestPortableSHMLockRequiresExplicitRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db-shm")
	x, y := openTestIndex(t, path), openTestIndex(t, path)
	lockTest(t, x, ReadLock0+1, 1, false)
	lockTest(t, x, ReadLock0+1, 1, false)
	if err := x.Lock(ReadLock0+1, 1, true); !errors.Is(err, ErrLock) {
		t.Fatalf("conversion accepted: %v", err)
	}
	if err := y.Lock(ReadLock0+1, 1, true); !errors.Is(err, ErrBusy) {
		t.Fatalf("rejected conversion released snapshot: %v", err)
	}
	if err := x.Unlock(ReadLock0+1, 1); err != nil {
		t.Fatal(err)
	}
	lockTest(t, y, ReadLock0+1, 1, true)
	if err := y.Unlock(ReadLock0+1, 1); err != nil {
		t.Fatal(err)
	}
	lockTest(t, x, ReadLock0+1, 1, true)
	if err := x.Lock(ReadLock0+1, 1, false); !errors.Is(err, ErrLock) {
		t.Fatalf("downgrade accepted: %v", err)
	}
}
