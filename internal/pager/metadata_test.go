package pager

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
	"github.com/ms-xiaosnow/gosqlite/internal/wal"
)

// Error-value law only: no real file, native failure, or cleanup is executed.
func TestMetadataPoisonClassificationPreservesCause(t *testing.T) {
	cause := errors.New("abstract metadata retirement")
	native := errors.Join(vfs.ErrIdentity, vfs.ErrPoisoned, cause)
	err := nativeFileError(native)
	if !errors.Is(err, ErrPoisoned) || !errors.Is(err, vfs.ErrPoisoned) || !errors.Is(err, cause) || !errors.Is(err, vfs.ErrIdentity) || errors.Is(err, ErrBusy) {
		t.Fatalf("metadata poison collapsed into retryable pathname/lock error: %v", err)
	}
}

// These are synthetic memory-only I/O interfaces, never wrappers around files.
// They exercise the production error adapters, not native physical failures.
type metadataReaderModel struct{ cause error }

func (r metadataReaderModel) ReadAt([]byte, int64) (int, error) { return 0, r.cause }

type metadataCloserModel struct {
	cause error
	calls int
}

func (c *metadataCloserModel) Close() error { c.calls++; return c.cause }

func TestMetadataPoisonThroughGenericReadAndCloseModels(t *testing.T) {
	cause := errors.New("abstract owned metadata uncertainty")
	modelErr := errors.Join(vfs.ErrPoisoned, cause)
	r := nativeReaderAt{metadataReaderModel{modelErr}}
	var header [100]byte
	_, sectionErr := io.ReadFull(io.NewSectionReader(r, 0, 100), header[:])
	_, scanErr := wal.Scan(context.Background(), r, wal.HeaderSize, wal.Limits{})
	closer := &metadataCloserModel{cause: modelErr}
	closeErr := closeNativeFile(closer)
	for _, err := range []error{sectionErr, scanErr, closeErr} {
		if !errors.Is(err, ErrPoisoned) || !errors.Is(err, vfs.ErrPoisoned) || !errors.Is(err, cause) || errors.Is(err, ErrBusy) {
			t.Fatal("generic consumer lost ownership/cause classification", err)
		}
	}
	if closer.calls != 1 {
		t.Fatal("close adapter repeated the close operation")
	}
}
