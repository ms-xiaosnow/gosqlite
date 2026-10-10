package encstore

import (
	"errors"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/darwinfile"
)

func TestProcessByteLocksMemoryOwnersAndAtomicConversion(t *testing.T) {
	var l processByteLocks
	var a, b processByteOwner
	l.attach(&a)
	l.attach(&b)
	var calls [][3]int64
	call := func(off int64, shared, unlock bool) error {
		s, u := int64(0), int64(0)
		if shared {
			s = 1
		}
		if unlock {
			u = 1
		}
		calls = append(calls, [3]int64{off, s, u})
		return nil
	}
	for _, o := range []*processByteOwner{&a, &b} {
		if e := l.change(o, 0, true, false, call); e != nil {
			t.Fatal(e)
		}
	}
	if len(calls) != 1 {
		t.Fatal("local shared acquisition touched process lock twice")
	}
	if e := l.change(&a, 0, false, false, call); !errors.Is(e, ErrBusy) || len(calls) != 1 {
		t.Fatal("upgrade ignored sibling", e)
	}
	if e := l.close(&b, call); e != nil {
		t.Fatal(e)
	}
	if len(calls) != 1 || l.kernel[0] != 1 {
		t.Fatal("sibling close dropped shared")
	}
	if e := l.change(&a, 0, false, false, call); e != nil {
		t.Fatal(e)
	}
	if e := l.change(&a, 0, true, false, call); e != nil {
		t.Fatal(e)
	}
	if len(calls) != 3 || calls[1] != [3]int64{0, 0, 0} || calls[2] != [3]int64{0, 1, 0} {
		t.Fatal("conversion introduced unlock gap", calls)
	}
	if e := l.close(&a, call); e != nil {
		t.Fatal(e)
	}
	if l.any() || len(l.owners) != 0 || calls[3] != [3]int64{0, 0, 1} {
		t.Fatal("last release", calls)
	}
}
func TestProcessByteLocksMemoryBusyPreservesLedger(t *testing.T) {
	var l processByteLocks
	var a processByteOwner
	l.attach(&a)
	ok := func(int64, bool, bool) error { return nil }
	if e := l.change(&a, 0, true, false, ok); e != nil {
		t.Fatal(e)
	}
	if e := l.change(&a, 0, false, false, func(int64, bool, bool) error { return ErrBusy }); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	if l.kernel[0] != 1 || a.held[0] != 1 || l.poisoned != nil {
		t.Fatal("busy altered atomic conversion")
	}
}
func TestProcessByteLocksMemoryThreeByteIndependence(t *testing.T) {
	var l processByteLocks
	var a, b processByteOwner
	l.attach(&a)
	l.attach(&b)
	call := func(int64, bool, bool) error { return nil }
	for _, o := range []*processByteOwner{&a, &b} {
		if e := l.change(o, 0, true, false, call); e != nil {
			t.Fatal(e)
		}
	}
	if e := l.change(&a, 1, false, false, call); e != nil {
		t.Fatal(e)
	}
	if e := l.change(&b, 2, true, false, call); e != nil {
		t.Fatal(e)
	}
	if e := l.change(&b, 1, false, false, call); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	if e := l.change(&a, 2, false, false, call); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	if l.kernel != [3]uint8{1, 2, 1} {
		t.Fatal(l.kernel)
	}
}
func TestProcessByteLocksMemoryCleanupPoisonAndRetry(t *testing.T) {
	for fail := 0; fail < 3; fail++ {
		var l processByteLocks
		var a processByteOwner
		l.attach(&a)
		ok := func(int64, bool, bool) error { return nil }
		for off := int64(0); off < 3; off++ {
			if e := l.change(&a, off, false, false, ok); e != nil {
				t.Fatal(e)
			}
		}
		calls := 0
		fault := errors.New("synthetic unlock failure")
		e := l.close(&a, func(int64, bool, bool) error {
			at := calls
			calls++
			if at == fail {
				return fault
			}
			return nil
		})
		if !errors.Is(e, ErrPoisoned) || !errors.Is(e, fault) {
			t.Fatal(e)
		}
		for off := 0; off < 3; off++ {
			want := uint8(2)
			if off > 2-fail {
				want = 0
			}
			if a.held[off] != want || l.kernel[off] != want {
				t.Fatal("partial ledger lost", fail, off)
			}
		}
		if e = l.change(&a, 1, false, false, ok); !errors.Is(e, ErrPoisoned) {
			t.Fatal("poison reuse", e)
		}
		if e = l.close(&a, ok); e != nil || l.any() {
			t.Fatal("represented cleanup retry", e)
		}
	}
}
func TestProcessByteLocksMemoryExhaustiveLocalConflict(t *testing.T) {
	for byteOffset := int64(0); byteOffset < 3; byteOffset++ {
		for peer := uint8(0); peer < 3; peer++ {
			for want := uint8(1); want < 3; want++ {
				var l processByteLocks
				var a, b processByteOwner
				l.attach(&a)
				l.attach(&b)
				b.held[byteOffset] = peer
				l.kernel[byteOffset] = peer
				calls := 0
				e := l.change(&a, byteOffset, want == 1, false, func(int64, bool, bool) error { calls++; return nil })
				conflict := peer == 2 || want == 2 && peer != 0
				if conflict {
					if !errors.Is(e, ErrBusy) || calls != 0 || a.held[byteOffset] != 0 {
						t.Fatal(peer, want, e)
					}
				} else if e != nil || a.held[byteOffset] != want || l.kernel[byteOffset] != want {
					t.Fatal(peer, want, e)
				}
			}
		}
	}
}
func TestProcessByteLocksMemoryCrossDomainCloseComposition(t *testing.T) {
	r := darwinfile.New()
	id := darwinfile.Identity{Device: 1, Inode: 2}
	closed := 0
	probe, _ := r.Register(id, func() error { closed++; return nil }, nil)
	var l processByteLocks
	var a, b processByteOwner
	l.attach(&a)
	l.attach(&b)
	call := func(int64, bool, bool) error { return nil }
	for _, o := range []*processByteOwner{&a, &b} {
		if e := l.change(o, 0, true, false, call); e != nil {
			t.Fatal(e)
		}
	}
	r.SetLocks(id, "encstore", l.any())
	r.SetLocks(id, "vfs", true)
	if e := r.Retire(probe); e != nil {
		t.Fatal(e)
	}
	for _, o := range []*processByteOwner{&a, &b} {
		if e := l.close(o, call); e != nil {
			t.Fatal(e)
		}
		r.SetLocks(id, "encstore", l.any())
		if e := r.Drain(id); e != nil {
			t.Fatal(e)
		}
	}
	if closed != 0 {
		t.Fatal("encrypted last-close ignored plain locks")
	}
	r.SetLocks(id, "vfs", false)
	if e := r.Drain(id); e != nil || closed != 1 {
		t.Fatal(e, closed)
	}
}

func TestProcessByteLocksMemoryEveryHeldArrayAndPartialClose(t *testing.T) {
	for encoded := 0; encoded < 27; encoded++ {
		var state [3]uint8
		value := encoded
		held := 0
		for i := range state {
			state[i] = uint8(value % 3)
			value /= 3
			if state[i] != 0 {
				held++
			}
		}
		for fail := -1; fail < held; fail++ {
			var l processByteLocks
			a := processByteOwner{held: state}
			l.attach(&a)
			l.kernel = state
			calls := 0
			fault := errors.New("synthetic close position")
			e := l.close(&a, func(off int64, shared, unlock bool) error {
				if shared || !unlock {
					t.Fatal("cleanup changed mode instead of release")
				}
				position := calls
				calls++
				if position == fail {
					return fault
				}
				return nil
			})
			if fail < 0 {
				if e != nil || l.any() || len(l.owners) != 0 || calls != held {
					t.Fatal(encoded, fail, e, calls)
				}
				continue
			}
			if !errors.Is(e, ErrPoisoned) || !errors.Is(e, fault) || calls != fail+1 {
				t.Fatal(encoded, fail, e, calls)
			}
			remaining := state
			successful := 0
			for i := 2; i >= 0; i-- {
				if remaining[i] != 0 {
					if successful == fail {
						break
					}
					remaining[i] = 0
					successful++
				}
			}
			if a.held != remaining || l.kernel != remaining {
				t.Fatal("partial close ledger", state, fail, a.held, l.kernel, remaining)
			}
		}
	}
}
