package encstore

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

// Every callback and competing owner here is inert memory. No file, descriptor,
// native lock, failure injection, process or publication is used by these laws.
func TestWindowsLifecycleMemoryAdmissionTrace(t *testing.T) {
	for _, shared := range []bool{false, true} {
		l := exactByteLocks{}
		m := exactMemoryCallbacks{ledger: &l}
		kind := uint8(2)
		if shared {
			kind = 1
		}
		if err := l.admitLifecycle(shared, m.call); err != nil {
			t.Fatal(err)
		}
		exactMemoryCheck(t, &m, []exactMemoryCall{
			{captureByte, false, false, [3]uint8{}},
			{lifecycleByte, shared, false, [3]uint8{0, 0, 2}},
			{captureByte, false, true, [3]uint8{kind, 0, 2}},
		}, [3]uint8{kind, 0, 0})
	}
}

func TestWindowsLifecycleMemoryUpgradeTrace(t *testing.T) {
	l := exactByteLocks{held: [3]uint8{1, 2, 0}}
	m := exactMemoryCallbacks{ledger: &l}
	if err := l.upgradeLifecycle(m.call); err != nil {
		t.Fatal(err)
	}
	exactMemoryCheck(t, &m, []exactMemoryCall{
		{captureByte, false, false, [3]uint8{1, 2, 0}},
		{lifecycleByte, false, true, [3]uint8{1, 2, 2}},
		{lifecycleByte, false, false, [3]uint8{0, 2, 2}},
		{captureByte, false, true, [3]uint8{2, 2, 2}},
	}, [3]uint8{2, 2, 0})
	m.calls = nil
	if err := l.change(lifecycleByte, true, false, m.call); err != nil {
		t.Fatal(err)
	}
	exactMemoryCheck(t, &m, []exactMemoryCall{
		{lifecycleByte, true, false, [3]uint8{2, 2, 0}},
		{lifecycleByte, false, true, [3]uint8{3, 2, 0}},
	}, [3]uint8{1, 2, 0})
}

func TestWindowsLifecycleMemoryAdmissionErrors(t *testing.T) {
	unexpected := errors.New("inert admission result")
	for _, shared := range []bool{false, true} {
		kind := uint8(2)
		if shared {
			kind = 1
		}
		for _, tc := range []struct {
			at     int
			cause  error
			held   [3]uint8
			poison bool
			calls  int
		}{
			{1, ErrBusy, [3]uint8{}, false, 1}, {1, unexpected, [3]uint8{}, true, 1},
			{2, ErrBusy, [3]uint8{}, false, 3}, {2, unexpected, [3]uint8{0, 0, 2}, true, 2},
			{3, ErrBusy, [3]uint8{kind, 0, 2}, true, 3}, {3, unexpected, [3]uint8{kind, 0, 2}, true, 3},
		} {
			t.Run(fmt.Sprintf("shared_%t_step_%d_%v", shared, tc.at, tc.cause), func(t *testing.T) {
				l := exactByteLocks{}
				m := exactMemoryCallbacks{ledger: &l, failAt: tc.at, cause: tc.cause}
				err := l.admitLifecycle(shared, m.call)
				if !errors.Is(err, tc.cause) || errors.Is(err, ErrPoisoned) != tc.poison || l.held != tc.held || len(m.calls) != tc.calls {
					t.Fatalf("result=%v held=%v calls=%v", err, l.held, m.calls)
				}
				assertWindowsLifecycleMemoryRetirement(t, &l, &m, tc.poison)
			})
		}
	}
}

func TestWindowsLifecycleMemoryUpgradeErrors(t *testing.T) {
	unexpected := errors.New("inert upgrade result")
	for _, tc := range []struct {
		at     int
		cause  error
		held   [3]uint8
		poison bool
		calls  int
	}{
		{1, ErrBusy, [3]uint8{1, 2, 0}, false, 1}, {1, unexpected, [3]uint8{1, 2, 0}, true, 1},
		{2, ErrBusy, [3]uint8{1, 2, 2}, true, 2}, {2, unexpected, [3]uint8{1, 2, 2}, true, 2},
		{3, ErrBusy, [3]uint8{1, 2, 0}, false, 5}, {3, unexpected, [3]uint8{0, 2, 2}, true, 3},
		{4, ErrBusy, [3]uint8{2, 2, 2}, true, 4}, {4, unexpected, [3]uint8{2, 2, 2}, true, 4},
	} {
		t.Run(fmt.Sprintf("step_%d_%v", tc.at, tc.cause), func(t *testing.T) {
			l := exactByteLocks{held: [3]uint8{1, 2, 0}}
			m := exactMemoryCallbacks{ledger: &l, failAt: tc.at, cause: tc.cause}
			err := l.upgradeLifecycle(m.call)
			if !errors.Is(err, tc.cause) || errors.Is(err, ErrPoisoned) != tc.poison || l.held != tc.held || len(m.calls) != tc.calls {
				t.Fatalf("result=%v held=%v calls=%v", err, l.held, m.calls)
			}
			assertWindowsLifecycleMemoryRetirement(t, &l, &m, tc.poison)
		})
	}
}

func assertWindowsLifecycleMemoryRetirement(t *testing.T, l *exactByteLocks, m *exactMemoryCallbacks, poison bool) {
	t.Helper()
	m.calls = nil
	m.failAt = 0
	if poison {
		if err := l.admitLifecycle(true, m.call); !errors.Is(err, ErrPoisoned) {
			t.Fatal("poison allowed admission", err)
		}
		if err := l.upgradeLifecycle(m.call); !errors.Is(err, ErrPoisoned) {
			t.Fatal("poison allowed upgrade", err)
		}
		if len(m.calls) != 0 {
			t.Fatal("poison performed ordinary callback")
		}
	}
	want, held := exactMemoryReleaseTrace(l.held, captureByte, writerByte, lifecycleByte)
	if err := l.close(m.call); err != nil {
		t.Fatal(err)
	}
	exactMemoryCheck(t, m, want, held)
	if (l.poisoned != nil) != poison {
		t.Fatal("retirement changed poison")
	}
}

func TestWindowsLifecycleMemoryRestorationErrorsRetainProtectors(t *testing.T) {
	for _, at := range []int{4, 5} {
		for _, cause := range []error{ErrBusy, errors.New("inert restoration/cleanup result")} {
			l := exactByteLocks{held: [3]uint8{1, 2, 0}}
			m := exactMemoryCallbacks{ledger: &l, failAt: at, cause: cause}
			call := func(off int64, shared, unlock bool) error {
				err := m.call(off, shared, unlock)
				if len(m.calls) == 3 {
					return ErrBusy
				}
				return err
			}
			err := l.upgradeLifecycle(call)
			want := [3]uint8{0, 2, 2}
			if at == 5 {
				want[0] = 1
			}
			if !errors.Is(err, ErrBusy) || !errors.Is(err, cause) || !errors.Is(err, ErrPoisoned) || l.held != want || len(m.calls) != at {
				t.Fatalf("at %d result=%v held=%v calls=%v", at, err, l.held, m.calls)
			}
			assertWindowsLifecycleMemoryRetirement(t, &l, &m, true)
		}
	}
}

func TestWindowsLifecycleMemoryAdmissionAndUpgradePreconditions(t *testing.T) {
	for packed := 0; packed < 64; packed++ {
		held := [3]uint8{uint8(packed & 3), uint8((packed >> 2) & 3), uint8((packed >> 4) & 3)}
		if held != ([3]uint8{}) {
			l := exactByteLocks{held: held}
			m := exactMemoryCallbacks{ledger: &l}
			if err := l.admitLifecycle(true, m.call); !errors.Is(err, ErrUnsupported) || l.poisoned != nil {
				t.Fatal(held, err)
			}
			exactMemoryCheck(t, &m, nil, held)
		}
		if held != ([3]uint8{1, 2, 0}) {
			l := exactByteLocks{held: held}
			m := exactMemoryCallbacks{ledger: &l}
			if err := l.upgradeLifecycle(m.call); !errors.Is(err, ErrUnsupported) || l.poisoned != nil {
				t.Fatal(held, err)
			}
			exactMemoryCheck(t, &m, nil, held)
		}
	}
}

// This separate little kernel follows the documented interval rules, not the
// production ledger bit operations. Three owners permit deterministic insertion
// of a newer admission participant or an older byte-0-only lifecycle participant.
// It is a model of the contract, never evidence that Windows actually obeyed it.
type windowsLifecycleMemoryKernel struct{ shared, exclusive [3][3]bool }

func (k *windowsLifecycleMemoryKernel) call(owner int, off int64, shared, unlock bool) error {
	if unlock {
		if k.exclusive[owner][off] {
			k.exclusive[owner][off] = false
			return nil
		}
		if k.shared[owner][off] {
			k.shared[owner][off] = false
			return nil
		}
		return errors.New("model unlock without exact hold")
	}
	for other := range k.shared {
		if shared {
			if other != owner && k.exclusive[other][off] {
				return ErrBusy
			}
		} else if k.shared[other][off] || k.exclusive[other][off] {
			return ErrBusy
		}
	}
	if shared {
		k.shared[owner][off] = true
	} else {
		k.exclusive[owner][off] = true
	}
	return nil
}
func (k *windowsLifecycleMemoryKernel) callback(owner int) byteLockCall {
	return func(off int64, shared, unlock bool) error { return k.call(owner, off, shared, unlock) }
}
func (k *windowsLifecycleMemoryKernel) assert(t *testing.T, owner int, l *exactByteLocks) {
	t.Helper()
	for off := range l.held {
		if (l.held[off]&1 != 0) != k.shared[owner][off] || (l.held[off]&2 != 0) != k.exclusive[owner][off] {
			t.Fatalf("owner %d ledger=%v kernel=%+v", owner, l.held, k)
		}
	}
}

func TestWindowsLifecycleMemoryCompetingAdmissionsAtEveryGuardedStep(t *testing.T) {
	for insert := 2; insert <= 4; insert++ {
		for _, shared := range []bool{false, true} {
			k := windowsLifecycleMemoryKernel{}
			k.shared[0][0] = true
			k.exclusive[0][1] = true
			a := exactByteLocks{held: [3]uint8{1, 2, 0}}
			b := exactByteLocks{}
			step := 0
			err := a.upgradeLifecycle(func(off int64, s, u bool) error {
				step++
				if step == insert {
					if e := b.admitLifecycle(shared, k.callback(1)); !errors.Is(e, ErrBusy) {
						t.Fatalf("step %d competing admission=%v", step, e)
					}
					k.assert(t, 1, &b)
					if b.held != ([3]uint8{}) || b.poisoned != nil {
						t.Fatal("competing admission retained ownership")
					}
				}
				return k.call(0, off, s, u)
			})
			if err != nil {
				t.Fatal(err)
			}
			k.assert(t, 0, &a)
		}
	}
}

func TestWindowsLifecycleMemoryExistingReadersAndCapture(t *testing.T) {
	for _, capture := range []bool{false, true} {
		k := windowsLifecycleMemoryKernel{}
		k.shared[0][0] = true
		k.exclusive[0][1] = true
		k.shared[1][0] = true
		k.shared[1][2] = capture
		l := exactByteLocks{held: [3]uint8{1, 2, 0}}
		if err := l.upgradeLifecycle(k.callback(0)); !errors.Is(err, ErrBusy) || errors.Is(err, ErrPoisoned) {
			t.Fatal(err)
		}
		k.assert(t, 0, &l)
		if l.held != ([3]uint8{1, 2, 0}) {
			t.Fatal("ordinary BUSY lost original shared lease", l.held)
		}
		if err := k.call(2, writerByte, false, false); !errors.Is(err, ErrBusy) {
			t.Fatal("writer protection lost", err)
		}
	}
}

func TestWindowsLifecycleMemoryLegacyAdmissionDuringGap(t *testing.T) {
	for _, shared := range []bool{false, true} {
		k := windowsLifecycleMemoryKernel{}
		k.shared[0][0] = true
		k.exclusive[0][1] = true
		l := exactByteLocks{held: [3]uint8{1, 2, 0}}
		step := 0
		err := l.upgradeLifecycle(func(off int64, s, u bool) error {
			step++
			if step == 3 {
				if e := k.call(1, lifecycleByte, shared, false); e != nil {
					t.Fatal(e)
				}
			}
			return k.call(0, off, s, u)
		})
		k.assert(t, 0, &l)
		if !errors.Is(err, ErrBusy) || errors.Is(err, ErrPoisoned) == shared {
			t.Fatal("legacy result", shared, err)
		}
		if shared {
			if l.held != ([3]uint8{1, 2, 0}) {
				t.Fatal(l.held)
			}
		} else {
			if l.held != ([3]uint8{0, 2, 2}) {
				t.Fatal("restoration failure released protectors", l.held)
			}
			if e := l.change(writerByte, false, true, k.callback(0)); !errors.Is(e, ErrPoisoned) {
				t.Fatal("writer escaped poison", e)
			}
		}
	}
}

// Model the mixed-version interval independently of disk. The source control
// separately requires the real Store to authenticate and compare the named
// bytes before maintenance returns; this generation model is not a filesystem.
func TestWindowsLifecycleMemoryLegacyTransientOwnerAuthority(t *testing.T) {
	for _, changed := range []bool{false, true} {
		k := windowsLifecycleMemoryKernel{}
		k.shared[0][0], k.exclusive[0][1] = true, true
		l := exactByteLocks{held: [3]uint8{1, 2, 0}}
		selected := []byte("authenticated root identity A")
		named := bytes.Clone(selected)
		step := 0
		err := l.upgradeLifecycle(func(off int64, shared, unlock bool) error {
			step++
			if step == 3 {
				if e := k.call(1, lifecycleByte, false, false); e != nil {
					t.Fatal(e)
				}
				if changed {
					named = []byte("authenticated root identity B")
				}
				if e := k.call(1, lifecycleByte, false, true); e != nil {
					t.Fatal(e)
				}
			}
			return k.call(0, off, shared, unlock)
		})
		if err != nil {
			t.Fatal(err)
		}
		k.assert(t, 0, &l)
		// Exclusive acquisition alone does not prove that selected authority is fresh.
		authorized := bytes.Equal(selected, named)
		if authorized == changed || string(selected) != "authenticated root identity A" {
			t.Fatal("generation/selection law")
		}
		if l.held != ([3]uint8{2, 2, 0}) {
			t.Fatal("missing protectors during authority check", l.held)
		}
	}
}
