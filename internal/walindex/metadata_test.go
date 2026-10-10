package walindex

import (
	"errors"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// Error-value law only, with no native or storage operation.
func TestMetadataPoisonClassificationPreservesCause(t *testing.T) {
	cause := errors.New("abstract metadata retirement")
	err := nativeError(errors.Join(vfs.ErrIdentity, vfs.ErrPoisoned, cause))
	if !errors.Is(err, ErrPoisoned) || !errors.Is(err, vfs.ErrPoisoned) || !errors.Is(err, cause) || !errors.Is(err, vfs.ErrIdentity) || errors.Is(err, ErrBusy) {
		t.Fatalf("metadata poison lost its ownership/cause classification: %v", err)
	}
}
