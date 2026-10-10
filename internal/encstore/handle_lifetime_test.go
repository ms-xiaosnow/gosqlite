package encstore

import (
	"errors"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

// All retirement callbacks below change only integers/slices in memory.
// These tests do not own any native file, directory, or lock handle.
func TestEncryptedHandleLifetimeMemoryParentPins(t *testing.T) {
	var l handleLifetime
	for i := 0; i < 3; i++ {
		if err := l.addChild(false); err != nil {
			t.Fatal(err)
		}
	}
	unlocks, closes := 0, 0
	unlock := func() error { unlocks++; return nil }
	closeHandle := func() error { closes++; return nil }
	if done, err := l.retire(unlock, closeHandle); done || err != nil || unlocks != 0 || closes != 0 {
		t.Fatal("parent retired under children", done, err)
	}
	if err := l.usable(false); !errors.Is(err, encryption.ErrClosed) {
		t.Fatal("public lease reused", err)
	}
	if err := l.addChild(false); !errors.Is(err, encryption.ErrClosed) {
		t.Fatal("new public child admitted", err)
	}
	if err := l.addChild(true); err != nil {
		t.Fatal("retained child metadata probe refused", err)
	}
	for l.children > 0 {
		if err := l.dropChild(); err != nil {
			t.Fatal(err)
		}
		if done, err := l.retire(unlock, closeHandle); err != nil || done != (l.children == 0) {
			t.Fatal("retirement boundary", done, err)
		}
	}
	if unlocks != 1 || closes != 1 || !l.retired {
		t.Fatal("actual handle not retired exactly once", unlocks, closes, l)
	}
	if done, err := l.retire(unlock, closeHandle); !done || err != nil || unlocks != 1 || closes != 1 {
		t.Fatal("duplicate actual close", done, err)
	}
	if err := l.usable(true); !errors.Is(err, encryption.ErrClosed) {
		t.Fatal("retired parent reopened", err)
	}
	if err := l.dropChild(); !errors.Is(err, ErrPoisoned) {
		t.Fatal("negative child count accepted", err)
	}
}
func TestEncryptedHandleLifetimeMemoryUnresolvedLocks(t *testing.T) {
	l := handleLifetime{}
	calls, closes := 0, 0
	marker := errors.New("memory lock cleanup marker")
	unlock := func() error {
		calls++
		if calls == 1 {
			return marker
		}
		return nil
	}
	closeHandle := func() error { closes++; return nil }
	if done, err := l.retire(unlock, closeHandle); done || !errors.Is(err, ErrPoisoned) || !errors.Is(err, marker) || closes != 0 || l.closeUnknown {
		t.Fatal("failed cleanup lost actual handle", done, err, l)
	}
	if err := l.usable(false); !errors.Is(err, encryption.ErrClosed) {
		t.Fatal("closing handle usable", err)
	}
	if done, err := l.retire(unlock, closeHandle); !done || err != nil || calls != 2 || closes != 1 {
		t.Fatal("safe cleanup retry", done, err, l)
	}
}
func TestEncryptedHandleLifetimeMemoryUnknownCloseIsTerminal(t *testing.T) {
	l := handleLifetime{}
	calls := 0
	marker := errors.New("memory close marker")
	closeHandle := func() error { calls++; return marker }
	if done, err := l.retire(func() error { return nil }, closeHandle); done || !errors.Is(err, ErrPoisoned) || !errors.Is(err, marker) || !l.closeUnknown || l.retired {
		t.Fatal("unknown close misrepresented", done, err, l)
	}
	for i := 0; i < 3; i++ {
		if done, err := l.retire(func() error { t.Fatal("lock retried after terminal close"); return nil }, closeHandle); done || !errors.Is(err, ErrPoisoned) || calls != 1 {
			t.Fatal("numeric handle close retried", done, err, calls)
		}
		if err := l.usable(true); !errors.Is(err, ErrPoisoned) {
			t.Fatal("uncertain parent reused", err)
		}
	}
}
