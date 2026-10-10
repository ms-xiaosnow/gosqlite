package vfs

import (
	"errors"
	"testing"
)

// All handles and addresses below are synthetic integers. These tests never
// open a descriptor, map native memory, call a backend, or inject an OS failure.
func TestWindowsMappingRetirementMemoryUnknownCloseIsTerminal(t *testing.T) {
	m := mappingRetirement{handle: 31, base: 4096, length: 32768}
	cause := errors.New("memory mapping-object close uncertainty")
	closes, unmaps := 0, 0
	closeHandle := func(h uintptr) error {
		closes++
		if h != 31 {
			t.Fatal("close did not use the represented handle")
		}
		return cause
	}
	unmapView := func(uintptr) error { unmaps++; return nil }
	first := m.retire(closeHandle, unmapView)
	if !errors.Is(first, cause) || !errors.Is(first, ErrPoisoned) || first != m.closeUnknown {
		t.Fatal("terminal close lost its original poisoned cause", first)
	}
	for i := 0; i < 1000; i++ {
		if err := m.retire(closeHandle, unmapView); err != first {
			t.Fatal("repeat grew/replaced the stored cause or resumed retirement", err)
		}
	}
	if closes != 1 || unmaps != 0 || m.handle != 31 || m.base != 4096 || m.length != 32768 {
		t.Fatal("terminal state retried a numeric handle or discarded a live view", m, closes, unmaps)
	}
}

func TestWindowsMappingRetirementMemoryViewRetryAfterKnownClose(t *testing.T) {
	m := mappingRetirement{handle: 37, base: 8192, length: 65536}
	cause := errors.New("memory represented view still live")
	closes, unmaps := 0, 0
	closeHandle := func(h uintptr) error {
		closes++
		if h != 37 || unmaps != 0 {
			t.Fatal("object close did not precede view retirement")
		}
		return nil
	}
	unmapView := func(base uintptr) error {
		unmaps++
		if base != 8192 || m.handle != 0 || closes != 1 {
			t.Fatal("unmap lost original base or repeated the known-closed handle")
		}
		if unmaps < 3 {
			return cause
		}
		return nil
	}
	for i := 0; i < 2; i++ {
		if err := m.retire(closeHandle, unmapView); err != cause {
			t.Fatal("represented view error lost", err)
		}
		if m.handle != 0 || m.base != 8192 || m.length != 65536 || m.closeUnknown != nil {
			t.Fatal("view retry discarded ownership or invented handle uncertainty", m)
		}
	}
	for i := 0; i < 3; i++ {
		if err := m.retire(closeHandle, unmapView); err != nil {
			t.Fatal(err)
		}
	}
	if closes != 1 || unmaps != 3 || m != (mappingRetirement{}) {
		t.Fatal("successful retirement was not exact and idempotent", m, closes, unmaps)
	}
}

func TestWindowsMappingRetirementMemoryFailedViewAcquisition(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(map[bool]string{false: "known object retirement", true: "terminal object close"}[unknown], func(t *testing.T) {
			// MapView acquisition supplied no base. The exact same retirement
			// transition handles both its cleanup and a later file-close attempt.
			m := mappingRetirement{handle: 41, length: 32768}
			acquisition := errors.New("memory view acquisition rejection")
			closeCause := errors.New("memory acquisition cleanup uncertainty")
			closes, unmaps := 0, 0
			closeHandle := func(h uintptr) error {
				closes++
				if h != 41 {
					t.Fatal("cleanup used a different object")
				}
				if unknown {
					return closeCause
				}
				return nil
			}
			unmapView := func(uintptr) error { unmaps++; return nil }
			var retained []mappingRetirement
			cleanup := m.retire(closeHandle, unmapView)
			if cleanup != nil {
				retained = append(retained, m)
			}
			result := errors.Join(acquisition, cleanup)
			if !errors.Is(result, acquisition) || errors.Is(result, ErrPoisoned) != unknown {
				t.Fatal("rejected view lost its acquisition/cleanup outcome", result)
			}
			if !unknown {
				if len(retained) != 0 || m != (mappingRetirement{}) {
					t.Fatal("known cleanup invented a retained mapping", m)
				}
			} else {
				f := &File{dms: true, shared: true, maps: make(map[*Mapping]struct{})}
				for i := 0; i < 1000; i++ {
					err := f.closeResourcesLocked(func(*nativeMapping) error {
						t.Fatal("rejected view became a published mapping")
						return nil
					}, func() error {
						if err := retained[0].retire(closeHandle, unmapView); err != nil {
							return err
						}
						t.Fatal("terminal acquisition cleanup released its protecting lease")
						return nil
					})
					if err != cleanup || !errors.Is(err, closeCause) || !f.dms || !f.shared {
						t.Fatal("retained acquisition changed its cause or protectors", err)
					}
				}
				if len(retained) != 1 || retained[0] != m || m.closeUnknown != cleanup {
					t.Fatal("terminal acquisition record was changed or discarded")
				}
			}
			if closes != 1 || unmaps != 0 {
				t.Fatal("acquisition cleanup retried object close or unmapped an absent view", closes, unmaps)
			}
		})
	}
}

func TestWindowsMappingRetirementMemoryPublishedViewKeepsProtectors(t *testing.T) {
	f := &File{dms: true, shared: true, reserved: true, exclusive: true, slots: slotLedger{2, 0, 1}, maps: make(map[*Mapping]struct{})}
	m := &Mapping{Bytes: []byte{17, 29}, owner: f}
	sibling := &Mapping{Bytes: []byte{43}, owner: f}
	f.maps[m], f.maps[sibling] = struct{}{}, struct{}{}
	state := mappingRetirement{handle: 47, base: 12288, length: 2}
	cause := errors.New("memory published mapping close uncertainty")
	closes, unmaps, fileCloses := 0, 0, 0
	unmap := func(native *nativeMapping) error {
		if native == &sibling.native {
			return nil // A known sibling can still retire once.
		}
		if native != &m.native {
			t.Fatal("unrecognized mapping owner")
		}
		return state.retire(func(uintptr) error { closes++; return cause }, func(uintptr) error { unmaps++; return nil })
	}
	closeFile := func() error { fileCloses++; return nil }
	// Mapping.Close's serialized inner transition, followed by repeated
	// File.Close resource ordering, all use the same retained native record.
	if err := m.retireLocked(unmap); !errors.Is(err, cause) {
		t.Fatal(err)
	}
	first := state.closeUnknown
	for i := 0; i < 1000; i++ {
		if err := f.closeResourcesLocked(unmap, closeFile); !errors.Is(err, ErrPoisoned) || !errors.Is(err, cause) {
			t.Fatal("file close lost terminal mapping cause", err)
		}
		if len(f.maps) != 1 || len(m.Bytes) != 2 || m.Bytes[0] != 17 || m.Bytes[1] != 29 || sibling.Bytes != nil {
			t.Fatal("published Bytes or the owning mapping ledger changed")
		}
		if _, ok := f.maps[m]; !ok || !f.dms || !f.shared || !f.reserved || !f.exclusive || f.slots != (slotLedger{2, 0, 1}) {
			t.Fatal("terminal mapping lost its file or protecting locks")
		}
	}
	if closes != 1 || unmaps != 0 || fileCloses != 0 || state.closeUnknown != first {
		t.Fatal("repeated owner cleanup touched terminal resources or grew their cause")
	}
}

func TestWindowsMappingRetirementMemorySuccessAndEmptyAreIdempotent(t *testing.T) {
	for _, initial := range []mappingRetirement{{}, {handle: 53, base: 16384, length: 7}} {
		m := initial
		closes, unmaps := 0, 0
		for i := 0; i < 3; i++ {
			if err := m.retire(func(h uintptr) error {
				closes++
				if h != initial.handle || unmaps != 0 {
					t.Fatal("successful retirement reordered object and view")
				}
				return nil
			}, func(base uintptr) error {
				unmaps++
				if base != initial.base || m.handle != 0 {
					t.Fatal("successful unmap lost base or preceded close")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}
		want := 0
		if initial.handle != 0 {
			want = 1
		}
		if m != (mappingRetirement{}) || closes != want || unmaps != want {
			t.Fatal("successful/empty retirement was not idempotent", m, closes, unmaps)
		}
	}
}
