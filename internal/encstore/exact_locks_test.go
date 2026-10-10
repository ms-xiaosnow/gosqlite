package encstore

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

// These tests exercise only the portable ownership ledger with in-memory
// callbacks. No descriptor, file, native lock call or native error is involved.
// They establish bookkeeping laws, not Windows runtime or durability evidence.
type exactMemoryCall struct {
	offset int64
	shared bool
	unlock bool
	before [3]uint8
}

type exactMemoryCallbacks struct {
	ledger *exactByteLocks
	calls  []exactMemoryCall
	failAt int
	cause  error
}

func (m *exactMemoryCallbacks) call(offset int64, shared, unlock bool) error {
	m.calls = append(m.calls, exactMemoryCall{offset, shared, unlock, m.ledger.held})
	if len(m.calls) == m.failAt {
		return m.cause
	}
	return nil
}

func exactMemoryCheck(t *testing.T, m *exactMemoryCallbacks, want []exactMemoryCall, held [3]uint8) {
	t.Helper()
	if !slices.Equal(m.calls, want) {
		t.Fatalf("callback trace = %+v; want %+v", m.calls, want)
	}
	if m.ledger.held != held {
		t.Fatalf("held = %v; want %v", m.ledger.held, held)
	}
}

// Model each successful release independently of the production bit operations.
func exactMemoryReleaseTrace(held [3]uint8, offsets ...int64) ([]exactMemoryCall, [3]uint8) {
	var trace []exactMemoryCall
	for _, offset := range offsets {
		switch held[offset] {
		case 3:
			trace = append(trace, exactMemoryCall{offset: offset, unlock: true, before: held})
			held[offset] = 1
			fallthrough
		case 1, 2:
			trace = append(trace, exactMemoryCall{offset: offset, unlock: true, before: held})
			held[offset] = 0
		}
	}
	return trace, held
}

func TestExactByteLocksMemoryAcquireAndIdempotency(t *testing.T) {
	for offset := lifecycleByte; offset <= captureByte; offset++ {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("byte_%d_shared_%t", offset, shared), func(t *testing.T) {
				initial := [3]uint8{2, 1, 2}
				initial[offset] = 0
				l := exactByteLocks{held: initial}
				m := exactMemoryCallbacks{ledger: &l}
				if err := l.change(offset, shared, false, m.call); err != nil {
					t.Fatal(err)
				}
				want := initial
				want[offset] = 2
				if shared {
					want[offset] = 1
				}
				trace := []exactMemoryCall{{offset, shared, false, initial}}
				exactMemoryCheck(t, &m, trace, want)
				for range 3 {
					if err := l.change(offset, shared, false, m.call); err != nil {
						t.Fatal("same-mode acquisition", err)
					}
				}
				exactMemoryCheck(t, &m, trace, want)
				if l.poisoned != nil {
					t.Fatal("successful acquisition poisoned ownership")
				}
			})
		}
	}
}

func TestExactByteLocksMemorySharedUpgradeRefused(t *testing.T) {
	for offset := lifecycleByte; offset <= captureByte; offset++ {
		l := exactByteLocks{held: [3]uint8{2, 2, 2}}
		l.held[offset] = 1
		initial := l.held
		m := exactMemoryCallbacks{ledger: &l}
		for range 2 {
			err := l.change(offset, false, false, m.call)
			if !errors.Is(err, ErrUnsupported) || errors.Is(err, ErrPoisoned) || errors.Is(err, ErrBusy) {
				t.Fatalf("byte %d upgrade = %v", offset, err)
			}
		}
		exactMemoryCheck(t, &m, nil, initial)
		if l.poisoned != nil {
			t.Fatal("refused upgrade poisoned a retained shared lease")
		}
		if err := l.change(offset, true, false, m.call); err != nil {
			t.Fatal("original shared lease no longer usable", err)
		}
		exactMemoryCheck(t, &m, nil, initial)
	}
}

func TestExactByteLocksMemoryDowngradeStacksBeforeRelease(t *testing.T) {
	for offset := lifecycleByte; offset <= captureByte; offset++ {
		l := exactByteLocks{held: [3]uint8{2, 2, 2}}
		initial := l.held
		stacked, final := initial, initial
		stacked[offset], final[offset] = 3, 1
		m := exactMemoryCallbacks{ledger: &l}
		if err := l.change(offset, true, false, m.call); err != nil {
			t.Fatal(err)
		}
		want := []exactMemoryCall{{offset, true, false, initial}, {offset, false, true, stacked}}
		exactMemoryCheck(t, &m, want, final)
		if l.poisoned != nil {
			t.Fatal("successful downgrade poisoned ownership")
		}
		if err := l.change(offset, true, false, m.call); err != nil {
			t.Fatal("downgraded shared idempotency", err)
		}
		exactMemoryCheck(t, &m, want, final)
	}
}

func TestExactByteLocksMemoryEveryHeldRecordRelease(t *testing.T) {
	for offset := lifecycleByte; offset <= captureByte; offset++ {
		for record := uint8(0); record <= 3; record++ {
			for _, shared := range []bool{false, true} {
				l := exactByteLocks{held: [3]uint8{3, 3, 3}}
				l.held[offset] = record
				want, held := exactMemoryReleaseTrace(l.held, offset)
				m := exactMemoryCallbacks{ledger: &l}
				if err := l.change(offset, shared, true, m.call); err != nil {
					t.Fatal(err)
				}
				exactMemoryCheck(t, &m, want, held)
				if err := l.change(offset, shared, true, m.call); err != nil {
					t.Fatal("repeated release", err)
				}
				exactMemoryCheck(t, &m, want, held)
			}
		}
	}
}

func TestExactByteLocksMemoryAllHeldCombinationsCloseOrder(t *testing.T) {
	for packed := 0; packed < 64; packed++ {
		initial := [3]uint8{uint8(packed & 3), uint8(packed >> 2 & 3), uint8(packed >> 4 & 3)}
		t.Run(fmt.Sprint(initial), func(t *testing.T) {
			l := exactByteLocks{held: initial}
			m := exactMemoryCallbacks{ledger: &l}
			want, held := exactMemoryReleaseTrace(initial, captureByte, writerByte, lifecycleByte)
			if err := l.close(m.call); err != nil {
				t.Fatal(err)
			}
			exactMemoryCheck(t, &m, want, held)
			if held != ([3]uint8{}) || l.poisoned != nil {
				t.Fatal("clean close retained records or poison")
			}
			if err := l.close(m.call); err != nil {
				t.Fatal("repeated close", err)
			}
			exactMemoryCheck(t, &m, want, held)
		})
	}
}

func TestExactByteLocksMemoryAcquisitionErrorOwnership(t *testing.T) {
	modelError := errors.New("memory acquisition result")
	for offset := lifecycleByte; offset <= captureByte; offset++ {
		for _, before := range []uint8{0, 2} {
			for _, cause := range []error{ErrBusy, fmt.Errorf("memory contention: %w", ErrBusy), modelError} {
				l := exactByteLocks{held: [3]uint8{1, 2, 1}}
				l.held[offset] = before
				initial := l.held
				m := exactMemoryCallbacks{ledger: &l, failAt: 1, cause: cause}
				err := l.change(offset, true, false, m.call)
				if !errors.Is(err, cause) {
					t.Fatalf("lost acquisition cause: %v", err)
				}
				exactMemoryCheck(t, &m, []exactMemoryCall{{offset, true, false, initial}}, initial)
				if errors.Is(cause, ErrBusy) {
					if l.poisoned != nil || errors.Is(err, ErrPoisoned) {
						t.Fatal("ordinary acquisition contention poisoned ownership", err)
					}
					m.calls, m.failAt = nil, 0
					if err := l.change(offset, true, false, m.call); err != nil {
						t.Fatal("contention could not be retried", err)
					}
					if l.held[offset] != 1 {
						t.Fatal("successful retry did not retain shared lease")
					}
				} else if !errors.Is(err, ErrPoisoned) || l.poisoned == nil {
					t.Fatal("uncertain acquisition did not poison ownership", err)
				}
			}
		}
	}
}

func TestExactByteLocksMemoryDowngradeReleaseErrorRetainsBoth(t *testing.T) {
	for offset := lifecycleByte; offset <= captureByte; offset++ {
		for _, cause := range []error{ErrBusy, errors.New("memory downgrade release result")} {
			l := exactByteLocks{held: [3]uint8{2, 2, 2}}
			initial, stacked := l.held, l.held
			stacked[offset] = 3
			m := exactMemoryCallbacks{ledger: &l, failAt: 2, cause: cause}
			err := l.change(offset, true, false, m.call)
			if !errors.Is(err, ErrPoisoned) || !errors.Is(err, cause) {
				t.Fatal("failed downgrade cleanup must be poisoned, even for BUSY", err)
			}
			exactMemoryCheck(t, &m, []exactMemoryCall{{offset, true, false, initial}, {offset, false, true, stacked}}, stacked)
			m.calls, m.failAt = nil, 0
			want, held := exactMemoryReleaseTrace(stacked, captureByte, writerByte, lifecycleByte)
			if err := l.close(m.call); err != nil {
				t.Fatal("stacked cleanup retry", err)
			}
			exactMemoryCheck(t, &m, want, held)
			if !errors.Is(l.poisoned, cause) {
				t.Fatal("cleanup cleared ownership poison")
			}
		}
	}
}

func TestExactByteLocksMemoryEveryPartialCloseRetriesExactRemainder(t *testing.T) {
	modelError := errors.New("memory partial cleanup result")
	for packed := 1; packed < 64; packed++ {
		initial := [3]uint8{uint8(packed & 3), uint8(packed >> 2 & 3), uint8(packed >> 4 & 3)}
		complete, _ := exactMemoryReleaseTrace(initial, captureByte, writerByte, lifecycleByte)
		for stop := range complete {
			t.Run(fmt.Sprintf("%v_step_%d", initial, stop+1), func(t *testing.T) {
				l := exactByteLocks{held: initial}
				m := exactMemoryCallbacks{ledger: &l, failAt: stop + 1, cause: modelError}
				err := l.close(m.call)
				if !errors.Is(err, ErrPoisoned) || !errors.Is(err, modelError) {
					t.Fatal("partial close lost poison or original cause", err)
				}
				remaining := complete[stop].before
				exactMemoryCheck(t, &m, complete[:stop+1], remaining)
				m.calls, m.failAt = nil, 0
				for offset := lifecycleByte; offset <= captureByte; offset++ {
					for _, shared := range []bool{false, true} {
						for _, unlock := range []bool{false, true} {
							if got := l.change(offset, shared, unlock, m.call); got != l.poisoned {
								t.Fatal("poison did not block ordinary change", got)
							}
						}
					}
				}
				exactMemoryCheck(t, &m, nil, remaining)
				want, held := exactMemoryReleaseTrace(remaining, captureByte, writerByte, lifecycleByte)
				if err := l.close(m.call); err != nil {
					t.Fatal("remaining cleanup could not be retried", err)
				}
				exactMemoryCheck(t, &m, want, held)
				if !errors.Is(l.poisoned, ErrPoisoned) || !errors.Is(l.poisoned, modelError) {
					t.Fatal("cleanup erased ownership poison")
				}
				if err := l.change(lifecycleByte, true, false, m.call); err != l.poisoned {
					t.Fatal("completed cleanup revived poisoned ownership", err)
				}
				exactMemoryCheck(t, &m, want, held)
				if err := l.close(m.call); err != nil {
					t.Fatal("completed cleanup was not idempotent", err)
				}
				exactMemoryCheck(t, &m, want, held)
			})
		}
	}
}

func TestExactByteLocksMemoryDirectReleaseErrorRetry(t *testing.T) {
	for offset := lifecycleByte; offset <= captureByte; offset++ {
		for record := uint8(1); record <= 3; record++ {
			initial := [3]uint8{2, 2, 2}
			initial[offset] = record
			complete, _ := exactMemoryReleaseTrace(initial, offset)
			for stop := range complete {
				for _, cause := range []error{ErrBusy, errors.New("memory direct release result")} {
					for _, shared := range []bool{false, true} {
						l := exactByteLocks{held: initial}
						m := exactMemoryCallbacks{ledger: &l, failAt: stop + 1, cause: cause}
						if err := l.change(offset, shared, true, m.call); !errors.Is(err, ErrPoisoned) || !errors.Is(err, cause) {
							t.Fatal("direct release lost poison or cause", err)
						}
						remaining := complete[stop].before
						exactMemoryCheck(t, &m, complete[:stop+1], remaining)
						if err := l.change(offset, shared, true, m.call); err != l.poisoned {
							t.Fatal("ordinary release retried poisoned ownership", err)
						}
						exactMemoryCheck(t, &m, complete[:stop+1], remaining)
						m.calls, m.failAt = nil, 0
						want, held := exactMemoryReleaseTrace(remaining, captureByte, writerByte, lifecycleByte)
						if err := l.close(m.call); err != nil {
							t.Fatal("close could not retire remaining direct-release records", err)
						}
						exactMemoryCheck(t, &m, want, held)
					}
				}
			}
		}
	}
}

func TestExactByteLocksMemoryCleanupAccumulatesPoison(t *testing.T) {
	l := exactByteLocks{held: [3]uint8{2, 2, 3}}
	first, second := errors.New("first memory cleanup result"), errors.New("second memory cleanup result")
	m := exactMemoryCallbacks{ledger: &l, failAt: 2, cause: first}
	if err := l.close(m.call); !errors.Is(err, first) || l.held != ([3]uint8{2, 2, 1}) {
		t.Fatal("first partial cleanup", l.held, err)
	}
	m.calls, m.failAt, m.cause = nil, 2, second
	if err := l.close(m.call); !errors.Is(err, first) || !errors.Is(err, second) || !errors.Is(err, ErrPoisoned) || l.held != ([3]uint8{2, 2, 0}) {
		t.Fatal("second partial cleanup forgot prior failure or successful releases", l.held, err)
	}
	m.calls, m.failAt, m.cause = nil, 1, ErrBusy
	if err := l.close(m.call); !errors.Is(err, ErrPoisoned) || !errors.Is(err, ErrBusy) || !errors.Is(err, first) || !errors.Is(err, second) || l.held != ([3]uint8{2, 2, 0}) {
		t.Fatal("cleanup BUSY lost poison precedence", l.held, err)
	}
	want, held := exactMemoryReleaseTrace(l.held, captureByte, writerByte, lifecycleByte)
	m.calls, m.failAt = nil, 0
	if err := l.close(m.call); err != nil {
		t.Fatal(err)
	}
	exactMemoryCheck(t, &m, want, held)
}

func TestExactByteLocksMemoryInvalidOffsetAndStackedAcquisition(t *testing.T) {
	for _, offset := range []int64{-1, 3, -1 << 63, 1<<63 - 1} {
		for _, unlock := range []bool{false, true} {
			l := exactByteLocks{held: [3]uint8{1, 2, 3}}
			m := exactMemoryCallbacks{ledger: &l}
			if err := l.change(offset, false, unlock, m.call); !errors.Is(err, ErrUnsupported) || errors.Is(err, ErrPoisoned) {
				t.Fatalf("invalid offset %d = %v", offset, err)
			}
			exactMemoryCheck(t, &m, nil, [3]uint8{1, 2, 3})
			if l.poisoned != nil {
				t.Fatal("invalid offset poisoned valid records")
			}
		}
	}
	for offset := lifecycleByte; offset <= captureByte; offset++ {
		for _, shared := range []bool{false, true} {
			l := exactByteLocks{held: [3]uint8{3, 3, 3}}
			m := exactMemoryCallbacks{ledger: &l}
			if err := l.change(offset, shared, false, m.call); !errors.Is(err, ErrUnsupported) || !errors.Is(err, ErrPoisoned) {
				t.Fatal("stacked acquisition must reject without another callback", err)
			}
			exactMemoryCheck(t, &m, nil, [3]uint8{3, 3, 3})
			if err := l.change(-1, false, true, m.call); err != l.poisoned {
				t.Fatal("argument validation obscured ownership poison", err)
			}
			exactMemoryCheck(t, &m, nil, [3]uint8{3, 3, 3})
		}
	}
}
