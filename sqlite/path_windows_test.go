//go:build windows && amd64

package sqlite

import (
	"errors"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// Error-value tests, compile-only on the Linux host; no native failures.
func TestWindowsCanonicalPathOwnershipClassification(t *testing.T) {
	cause := errors.New("abstract owned metadata close")
	for _, native := range []error{vfs.ErrPoisoned, encstore.ErrPoisoned} {
		err := wrap("open", nativeOpenPathError(errors.Join(native, cause)))
		var public *Error
		if !errors.Is(err, ErrPoisoned) || !errors.Is(err, cause) || !errors.As(err, &public) || public.Code != CodeIOErr {
			t.Fatal("public open lost poisoned metadata classification", err)
		}
	}
}
