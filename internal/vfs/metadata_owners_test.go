package vfs

import (
	"errors"
	"sync"
	"testing"
)

// Every handle in these tests is a synthetic integer. No native operation,
// os.File, storage, injected OS error, or actual close retry is involved.
func TestMetadataOwnersReserveBeforeCreateAndBound(t *testing.T) {
	r := metadataOwners{limit: 2}
	calls := 0
	create := func() (uintptr, error) {
		calls++
		if len(r.live) != calls {
			t.Fatal("creation began before its ownership slot was reserved")
		}
		return uintptr(calls), nil
	}
	a, err := r.open(create)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.open(create)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.open(func() (uintptr, error) { t.Fatal("opened above bound"); return 3, nil }); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("capacity error lost: %v", err)
	}
	if len(r.live) != 2 || a.handle != 1 || b.handle != 2 || r.check() != nil {
		t.Fatal("capacity pressure changed live ownership or poisoned the gate")
	}
	if err = r.close(a, func(h uintptr) error {
		if h != 1 || len(r.live) != 2 {
			t.Fatal("owner disappeared before actual retirement")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !a.retired || a.handle != 0 || len(r.live) != 1 {
		t.Fatal("successful retirement did not release exactly one slot")
	}
	c, err := r.open(func() (uintptr, error) { return 1, nil }) // Number may legitimately be reused after success.
	if err != nil || c == a {
		t.Fatal("released capacity was not reusable with a distinct owner", err)
	}
	if err = r.close(a, func(uintptr) error { t.Fatal("retired owner closed a reused number"); return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestMetadataOwnersFailedCreateHasNoResource(t *testing.T) {
	r := metadataOwners{limit: 1}
	cause := errors.New("abstract unsuccessful create")
	if o, err := r.open(func() (uintptr, error) {
		if len(r.live) != 1 {
			t.Fatal("failed create had no reserved record")
		}
		return 0, cause
	}); o != nil || !errors.Is(err, cause) || len(r.live) != 0 || r.check() != nil {
		t.Fatal("failed create leaked capacity or invented ownership", o, err)
	}
	if _, err := r.open(func() (uintptr, error) { return 91, nil }); err != nil {
		t.Fatal("failed create consumed the last slot", err)
	}
}

func TestMetadataOwnersUnknownCloseTombstone(t *testing.T) {
	r := metadataOwners{limit: 3}
	a, err := r.open(func() (uintptr, error) { return 11, nil })
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.open(func() (uintptr, error) { return 12, nil })
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("abstract indeterminate retirement")
	closes := 0
	err = r.close(a, func(h uintptr) error {
		closes++
		if h != 11 {
			t.Fatal("wrong owned handle")
		}
		return cause
	})
	if !errors.Is(err, ErrPoisoned) || !errors.Is(err, cause) || !a.unknown || a.retired || a.handle != 11 || len(r.live) != 2 {
		t.Fatal("uncertain resource was not retained intact", err)
	}
	if _, ok := r.live[a]; !ok {
		t.Fatal("uncertain resource lost its strong owner")
	}
	if err = r.close(a, func(uintptr) error { closes++; return nil }); !errors.Is(err, ErrPoisoned) || closes != 1 {
		t.Fatal("uncertain numeric handle was retried", err)
	}
	if _, err = r.open(func() (uintptr, error) { t.Fatal("opened after poison"); return 13, nil }); !errors.Is(err, ErrPoisoned) {
		t.Fatal("poison did not preempt later acquisition", err)
	}
	if err = r.close(b, func(h uintptr) error {
		if h != 12 {
			t.Fatal("known sibling retirement changed target")
		}
		return nil
	}); err != nil || len(r.live) != 1 || !b.retired {
		t.Fatal("known sibling could not retire after poison", err)
	}
	if !errors.Is(r.check(), ErrPoisoned) || !errors.Is(r.check(), cause) {
		t.Fatal("sibling retirement cleared terminal poison")
	}
}

func TestMetadataOwnersConcurrentOrdinaryLifecycle(t *testing.T) {
	r := metadataOwners{limit: 16}
	var next uintptr
	var opens, closes int
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				o, err := r.open(func() (uintptr, error) { next++; opens++; return next, nil })
				if err != nil {
					t.Error(err)
					return
				}
				if err = r.close(o, func(h uintptr) error {
					if h == 0 || h > next {
						t.Error("unowned synthetic handle")
					}
					closes++
					return nil
				}); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
	if opens != 800 || closes != opens || len(r.live) != 0 || r.check() != nil {
		t.Fatal("ordinary lifecycle lost or duplicated ownership", opens, closes, len(r.live))
	}
}

func TestMetadataObservationPublishesOnlyAfterRetirement(t *testing.T) {
	for _, inspectFails := range []bool{false, true} {
		for _, closeFails := range []bool{false, true} {
			r := metadataOwners{limit: 1}
			o, err := r.open(func() (uintptr, error) { return 17, nil })
			if err != nil {
				t.Fatal(err)
			}
			inspectCause := errors.New("abstract observation unavailable")
			closeCause := errors.New("abstract retirement uncertain")
			closeCalls := 0
			value, err := observeMetadata(&r, o, func(h uintptr) (string, error) {
				if h != 17 || len(r.live) != 1 || closeCalls != 0 {
					t.Fatal("metadata observed without its active owner")
				}
				if inspectFails {
					return "must not publish", inspectCause
				}
				return "complete snapshot", nil
			}, func(h uintptr) error {
				closeCalls++
				if h != 17 {
					t.Fatal("closed a different probe")
				}
				if closeFails {
					return closeCause
				}
				return nil
			})
			if closeCalls != 1 || errors.Is(err, inspectCause) != inspectFails || errors.Is(err, closeCause) != closeFails || errors.Is(err, ErrPoisoned) != closeFails {
				t.Fatal("probe composition lost its precise outcome", inspectFails, closeFails, err)
			}
			if inspectFails || closeFails {
				if value != "" || err == nil {
					t.Fatal("uncertain/incomplete snapshot escaped", value, err)
				}
			} else if value != "complete snapshot" || err != nil {
				t.Fatal("success was not published after close", value, err)
			}
			if closeFails != (len(r.live) == 1) || closeFails != o.unknown {
				t.Fatal("unresolved probe lost ownership")
			}
		}
	}
}

func TestMetadataRejectedOpenRetainsParentOnUncertainty(t *testing.T) {
	for _, closeFails := range []bool{false, true} {
		r := metadataOwners{limit: 1}
		parent, err := r.open(func() (uintptr, error) { return 23, nil })
		if err != nil {
			t.Fatal(err)
		}
		cause := errors.New("abstract subsequent file open rejected")
		cleanupCause := errors.New("abstract parent retirement uncertain")
		calls := 0
		err = rejectMetadata(&r, parent, cause, func(h uintptr) error {
			calls++
			if h != 23 {
				t.Fatal("parent cleanup changed target")
			}
			if closeFails {
				return cleanupCause
			}
			return nil
		})
		if calls != 1 || !errors.Is(err, cause) || errors.Is(err, cleanupCause) != closeFails || errors.Is(err, ErrPoisoned) != closeFails {
			t.Fatal("failed-open parent lost cleanup outcome", err)
		}
		if !closeFails && err != cause {
			t.Fatal("successful cleanup wrapped an otherwise intact original error")
		}
		if closeFails != (len(r.live) == 1) || closeFails != parent.unknown {
			t.Fatal("failed-open parent ownership was discarded")
		}
	}
}
