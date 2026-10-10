package encstore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

// Independent composition laws: these tests own only Go structs and use integer
// callbacks. They never open files, invoke native handles/locks, inject OS
// failures, or qualify Windows runtime or durability behavior.
func TestWindowsReviewMemoryLockLifetimeComposition(t *testing.T) {
	for state := 0; state < 64; state++ {
		t.Run(fmt.Sprint(state), func(t *testing.T) {
			parent, child := handleLifetime{}, handleLifetime{}
			locks := exactByteLocks{held: [3]uint8{uint8(state & 3), uint8(state >> 2 & 3), uint8(state >> 4 & 3)}}
			if err := parent.addChild(false); err != nil {
				t.Fatal(err)
			}
			parentCloses, childCloses := 0, 0
			parentClose := func() error { parentCloses++; return nil }
			if done, err := parent.retire(func() error { return nil }, parentClose); done || err != nil || parentCloses != 0 {
				t.Fatal("parent escaped child retention", done, err)
			}
			last := int64(3)
			unlock := func(off int64, shared, release bool) error {
				if !release || shared || off > last || locks.held[off] == 0 {
					t.Fatal("not a descending exact outstanding release", off, shared, release, locks.held)
				}
				last = off
				return nil
			}
			done, err := child.retire(func() error { return locks.close(unlock) }, func() error {
				if locks.held != ([3]uint8{}) || parentCloses != 0 {
					t.Fatal("actual child retirement preceded locks or parent pin", locks.held, parentCloses)
				}
				childCloses++
				return nil
			})
			if !done || err != nil || childCloses != 1 {
				t.Fatal("child did not physically retire", done, err)
			}
			if err := parent.dropChild(); err != nil {
				t.Fatal(err)
			}
			if done, err := parent.retire(func() error { return nil }, parentClose); !done || err != nil || parentCloses != 1 {
				t.Fatal("last physical child did not retire parent", done, err)
			}
			if err := child.usable(true); !errors.Is(err, encryption.ErrClosed) {
				t.Fatal("retired child reused", err)
			}
		})
	}
}

func TestWindowsReviewMemoryUnresolvedChildPreservesParent(t *testing.T) {
	// Three stacked intervals require six exact successful release callbacks.
	for position := 1; position <= 6; position++ {
		t.Run(fmt.Sprint(position), func(t *testing.T) {
			parent := handleLifetime{children: 1}
			child := handleLifetime{}
			locks := exactByteLocks{held: [3]uint8{3, 3, 3}}
			marker := errors.New("in-memory unresolved release")
			calls, childCloses, parentCloses := 0, 0, 0
			parentClose := func() error { parentCloses++; return nil }
			if done, err := parent.retire(func() error { return nil }, parentClose); done || err != nil {
				t.Fatal(done, err)
			}
			unlock := func(off int64, shared, release bool) error {
				calls++
				if !release || shared || locks.held[off] == 0 {
					t.Fatal("invalid release", off, locks.held)
				}
				if calls == position {
					return marker
				}
				return nil
			}
			retire := func() (bool, error) {
				return child.retire(func() error { return locks.close(unlock) }, func() error { childCloses++; return nil })
			}
			if done, err := retire(); done || !errors.Is(err, marker) || !errors.Is(err, ErrPoisoned) || childCloses != 0 || parentCloses != 0 || parent.children != 1 {
				t.Fatal("unresolved child released resources", done, err, childCloses, parentCloses)
			}
			before := calls
			if err := locks.change(writerByte, false, false, unlock); !errors.Is(err, ErrPoisoned) || calls != before {
				t.Fatal("ordinary lock operation escaped poison", err)
			}
			if done, err := retire(); !done || err != nil || calls != 7 || childCloses != 1 {
				t.Fatal("cleanup retry did not drain exact remainder", done, err, calls, childCloses)
			}
			if err := parent.dropChild(); err != nil {
				t.Fatal(err)
			}
			if done, err := parent.retire(func() error { return nil }, parentClose); !done || err != nil || parentCloses != 1 {
				t.Fatal("parent did not retire after resolved child", done, err)
			}
		})
	}
}

func TestWindowsReviewMemoryUnknownChildCloseNeverReleasesPin(t *testing.T) {
	parent := handleLifetime{children: 1}
	child := handleLifetime{}
	locks := exactByteLocks{held: [3]uint8{1, 2, 3}}
	marker := errors.New("in-memory uncertain close")
	unlockCalls, closeCalls, parentCloses := 0, 0, 0
	unlock := func(int64, bool, bool) error { unlockCalls++; return nil }
	closeChild := func() error { closeCalls++; return marker }
	if done, err := parent.retire(func() error { return nil }, func() error { parentCloses++; return nil }); done || err != nil {
		t.Fatal(done, err)
	}
	if done, err := child.retire(func() error { return locks.close(unlock) }, closeChild); done || !errors.Is(err, marker) || !errors.Is(err, ErrPoisoned) {
		t.Fatal("unknown close misreported", done, err)
	}
	for range 4 {
		if done, err := child.retire(func() error { return locks.close(unlock) }, closeChild); done || !errors.Is(err, ErrPoisoned) {
			t.Fatal("unknown numeric resource revived", done, err)
		}
		if err := child.addChild(true); !errors.Is(err, ErrPoisoned) {
			t.Fatal("unknown resource admitted child", err)
		}
	}
	if unlockCalls != 4 || closeCalls != 1 || parent.children != 1 || parentCloses != 0 || !child.closeUnknown || child.retired {
		t.Fatal("uncertain resource lost retained parent or retried", unlockCalls, closeCalls, parent, child)
	}
}

func TestWindowsReviewMemoryRetainedProbeClosePermutations(t *testing.T) {
	// All six completion orders for two original children and one relative probe.
	orders := [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}
	for _, order := range orders {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			parent := handleLifetime{children: 2}
			children := [3]handleLifetime{}
			closes := 0
			retireParent := func() (bool, error) {
				return parent.retire(func() error { return nil }, func() error { closes++; return nil })
			}
			if done, err := retireParent(); done || err != nil {
				t.Fatal(done, err)
			}
			if err := parent.addChild(false); !errors.Is(err, encryption.ErrClosed) {
				t.Fatal("public lease reopened", err)
			}
			if err := parent.addChild(true); err != nil || !parent.closed {
				t.Fatal("retained metadata probe changed public close", err)
			}
			for step, index := range order {
				if done, err := children[index].retire(func() error { return nil }, func() error { return nil }); !done || err != nil {
					t.Fatal(done, err)
				}
				if err := parent.dropChild(); err != nil {
					t.Fatal(err)
				}
				if done, err := retireParent(); err != nil || done != (step == 2) || closes != step/2 {
					t.Fatal("physical completion order lost parent", step, done, err, closes)
				}
			}
			if err := parent.addChild(true); !errors.Is(err, encryption.ErrClosed) {
				t.Fatal("last child resurrected retired parent", err)
			}
		})
	}
}
