package pager

import (
	"errors"
	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
	"testing"
)

type memoryEncryptedRetirement struct {
	calls    int
	done     bool
	cause    error
	received error
}

func (m *memoryEncryptedRetirement) CloseWithError(e error) error {
	m.calls++
	if m.calls == 1 {
		m.received = e
		return errors.Join(encstore.ErrCleanupPending, encstore.ErrPoisoned, m.cause)
	}
	m.done = true
	return nil
}
func (m *memoryEncryptedRetirement) RetirementComplete() bool { return m.done }
func TestEncryptedSessionCloseMemoryOnceAndActiveLease(t *testing.T) {
	closed := false
	calls := 0
	checkpoint := func() error { calls++; return ErrBusy }
	owner := &memoryEncryptedRetirement{cause: errors.New("memory retained owner")}
	if _, e := retireEncryptedSession(&closed, true, false, false, nil, nil, owner, checkpoint); !errors.Is(e, ErrBusy) || closed || owner.calls != 0 || calls != 0 {
		t.Fatal(e)
	}
	done, e := retireEncryptedSession(&closed, false, false, false, nil, nil, owner, checkpoint)
	if done || !closed || !errors.Is(e, encstore.ErrCleanupPending) || !errors.Is(e, ErrPoisoned) || !errors.Is(owner.received, ErrBusy) || calls != 1 {
		t.Fatal(e)
	}
	done, e = retireEncryptedSession(&closed, false, false, false, nil, nil, owner, checkpoint)
	if !done || e != nil || calls != 1 || owner.calls != 2 {
		t.Fatal("repeated checkpoint or lost retirement", e)
	}
}
func TestEncryptedSessionCloseMemorySkipCheckpoint(t *testing.T) {
	for _, which := range []string{"read-only", "keep", "poison", "release-error"} {
		closed := false
		owner := &memoryEncryptedRetirement{}
		var poison, cause error
		if which == "poison" {
			poison = ErrPoisoned
		}
		if which == "release-error" {
			cause = errors.New("memory Rows release")
		}
		_, e := retireEncryptedSession(&closed, false, which == "read-only", which == "keep", poison, cause, owner, func() error { t.Fatal("checkpoint ran", which); return nil })
		if !closed || !errors.Is(e, encstore.ErrCleanupPending) {
			t.Fatal(e)
		}
		if cause != nil && !errors.Is(owner.received, cause) {
			t.Fatal("release cause discarded")
		}
	}
}
func TestEncryptedSessionCloseMemoryActivePreservesOptions(t *testing.T) {
	opts := Options{PageSize: 512, KeepWALOnClose: true, MaxDirtyPages: 17}
	// Empty Store has no native owners. The active-lease path must not touch it.
	s := &EncryptedSession{store: &encstore.Store{}, active: &Tx{}, opts: opts}
	if e := s.Close(); !errors.Is(e, ErrBusy) || s.closed || s.opts != opts || s.store == nil {
		t.Fatal("active BUSY changed session options", e, s.opts)
	}
	s.active = nil
	if e := s.Close(); e != nil || !s.closed || s.store != nil {
		t.Fatal("lease-free close failed", e)
	}
}
