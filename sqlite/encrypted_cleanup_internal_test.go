package sqlite

import (
	"errors"
	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
	"testing"
)

func TestEncryptedCleanupMemoryPublicClassification(t *testing.T) {
	original := errors.New("memory original cause")
	e := wrap("open", errors.Join(encstore.ErrCleanupPending, encstore.ErrPoisoned, ErrBusy, original))
	var detail *Error
	if !errors.Is(e, ErrEncryptedCleanupPending) || !errors.Is(e, ErrPoisoned) || !errors.Is(e, original) || !errors.As(e, &detail) || detail.Code != CodeIOErr {
		t.Fatal(e)
	}
	e = wrap("test", ErrEncryptedCleanupPending)
	if errors.Is(e, ErrPoisoned) {
		t.Fatal("bare marker inferred poison")
	}
	if e = RetryEncryptedCleanup(nil); e != nil {
		t.Fatal(e)
	}
	if e = RetryEncryptedCleanup(errors.New("ordinary error")); e != nil {
		t.Fatal(e)
	}
}
