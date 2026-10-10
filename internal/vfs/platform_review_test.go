package vfs

import (
	"errors"
	"math"
	"testing"
)

// These tests exercise only portable arithmetic, ledgers, and retirement
// callbacks. No file is opened, no native backend is called, and no OS failure
// is injected. They do not establish Darwin or Windows runtime acceptance.
func TestPlatformReviewMappingSpanConservation(t *testing.T) {
	for _, granularity := range []int64{1, 4096, 16384, 65536, 1 << 30} {
		for _, off := range []int64{0, 1, 4095, 16384, 32768, 65535, 65536, 98304, math.MaxInt64 - 65536} {
			for _, length := range []int{1, 7, 4096, 32768, 65536} {
				base, delta, nativeLength, err := mappingSpan(off, length, granularity)
				if err != nil {
					t.Fatalf("valid span (%d,%d,%d): %v", off, length, granularity, err)
				}
				if base < 0 || base%granularity != 0 || delta < 0 || int64(delta) >= granularity || nativeLength < length {
					t.Fatalf("invalid aligned span (%d,%d,%d): (%d,%d,%d)", off, length, granularity, base, delta, nativeLength)
				}
				if base+int64(delta) != off || nativeLength-delta != length || base+int64(nativeLength) != off+int64(length) {
					t.Fatalf("span lost bytes or implicitly enlarged backing extent: (%d,%d,%d)", base, delta, nativeLength)
				}
			}
		}
	}
	for _, tc := range []struct {
		off, granularity int64
		length           int
	}{
		{-1, 65536, 1}, {0, 65536, -1}, {0, 65536, 0},
		{0, 0, 1}, {0, -65536, 1}, {0, 65535, 1},
		{math.MaxInt64, 65536, 1}, {math.MaxInt64 - 32767, 65536, 32768},
		{1, 65536, math.MaxInt},
	} {
		if _, _, _, err := mappingSpan(tc.off, tc.length, tc.granularity); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("invalid/overflowing span accepted: %+v: %v", tc, err)
		}
	}
}

func TestPlatformReviewSlotTransitionsExhaustive(t *testing.T) {
	// All 3^8 ownership states, every legal interval, and both requested modes.
	// prepare must reject every conversion before mutating any existing hold.
	const states = 6561
	for encoded := 0; encoded < states; encoded++ {
		var initial slotLedger
		for remaining, slot := encoded, 0; slot < len(initial); slot++ {
			initial[slot] = uint8(remaining % 3)
			remaining /= 3
		}
		for start := 0; start < len(initial); start++ {
			for count := 1; count <= len(initial)-start; count++ {
				for _, exclusive := range []bool{false, true} {
					state := initial
					wantMode := uint8(1)
					if exclusive {
						wantMode = 2
					}
					invalid, wantMissing := !exclusive && count != 1, false
					for i := start; i < start+count; i++ {
						invalid = invalid || initial[i] != 0 && initial[i] != wantMode
						wantMissing = wantMissing || initial[i] == 0
					}
					mode, missing, err := state.prepare(start, count, exclusive)
					if state != initial {
						t.Fatal("preflight changed ownership", initial, state)
					}
					if invalid {
						if !errors.Is(err, ErrLock) {
							t.Fatalf("invalid conversion accepted: %v [%d,%d) exclusive=%t", initial, start, start+count, exclusive)
						}
						continue
					}
					if err != nil || mode != wantMode || missing != wantMissing {
						t.Fatalf("wrong acquisition plan: %v [%d,%d) exclusive=%t: %d,%t,%v", initial, start, start+count, exclusive, mode, missing, err)
					}
					state.assign(start, count, mode)
					if _, again, err := state.prepare(start, count, exclusive); err != nil || again {
						t.Fatal("same-mode acquisition is not idempotent", state, err)
					}
					for i := range state {
						want := initial[i]
						if i >= start && i < start+count {
							want = wantMode
						}
						if state[i] != want {
							t.Fatal("acquisition escaped its exact interval", state)
						}
					}
				}
			}
		}
	}
}

func TestPlatformReviewRetirementPermutationsMemoryModel(t *testing.T) {
	// Each mask specifies which of four abstract views remains owned. This is
	// a callback-level resource model, not failed mmap/unmap/Close execution.
	for retainedMask := 0; retainedMask < 16; retainedMask++ {
		f := &File{dms: true, shared: true, maps: make(map[*Mapping]struct{})}
		var views [4]*Mapping
		var calls [4]int
		for i := range views {
			views[i] = &Mapping{Bytes: []byte{byte(i)}, owner: f}
			f.maps[views[i]] = struct{}{}
		}
		pending := errors.New("abstract resource still owned")
		closeCalls := 0
		closeOwner := func() error {
			closeCalls++
			if len(f.maps) != 0 || !f.dms || !f.shared {
				t.Fatal("owner retirement preceded a view or protective lock")
			}
			return nil
		}
		err := f.closeResourcesLocked(func(native *nativeMapping) error {
			for i, view := range views {
				if native == &view.native {
					calls[i]++
					if retainedMask&(1<<i) != 0 {
						return pending
					}
					return nil
				}
			}
			t.Fatal("unknown abstract view")
			return nil
		}, closeOwner)
		if retainedMask == 0 {
			if err != nil || closeCalls != 1 {
				t.Fatalf("complete retirement: %v, close calls=%d", err, closeCalls)
			}
		} else if !errors.Is(err, ErrPoisoned) || !errors.Is(err, pending) || closeCalls != 0 {
			t.Fatalf("partial retirement released owner: %v, close calls=%d", err, closeCalls)
		}
		for i, view := range views {
			_, owned := f.maps[view]
			wantOwned := retainedMask&(1<<i) != 0
			if calls[i] != 1 || owned != wantOwned || (view.Bytes != nil) != wantOwned {
				t.Fatal("retirement lost or repeated a view", retainedMask, i, calls[i])
			}
		}
		if retainedMask != 0 {
			if err := f.closeResourcesLocked(func(*nativeMapping) error { return nil }, closeOwner); err != nil || closeCalls != 1 || len(f.maps) != 0 {
				t.Fatalf("represented remainder could not retire: %v", err)
			}
		}
	}
}

func TestPlatformReviewClosedOwnerPreemptsCachedHolds(t *testing.T) {
	// A nil raw owner must stop even stale successful cached lock bits before
	// reaching nativeCheck or any OS method, on every target backend.
	f := &File{role: Database, shared: true, reserved: true, exclusive: true, dms: true}
	checks := []func() error{
		func() error { return f.LockDB(Shared) },
		func() error { return f.LockDB(Reserved) },
		func() error { return f.LockDB(Exclusive) },
		f.DowngradeDBToShared,
		func() error { _, err := f.ReservedHeld(); return err },
		f.AttachDMS,
		func() error { return f.LockSHM(0, 1, true) },
		func() error { return f.UnlockSHM(0, 1) },
		func() error { _, err := f.MapShared(0, 32768); return err },
		f.ValidatePath,
		f.ValidateSingleLink,
	}
	for i, check := range checks {
		if err := check(); !errors.Is(err, ErrClosed) {
			t.Fatalf("closed owner escaped entry check %d: %v", i, err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal("closed owner retirement was not idempotent", err)
	}
}
