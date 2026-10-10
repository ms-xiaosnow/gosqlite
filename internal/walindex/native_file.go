package walindex

import (
	"errors"
	"fmt"
	"os"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

func nativeError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, vfs.ErrPoisoned):
		return fmt.Errorf("%w: %w", ErrPoisoned, err)
	case errors.Is(err, vfs.ErrBusy):
		return fmt.Errorf("%w: %w", ErrBusy, err)
	case errors.Is(err, vfs.ErrLinks):
		return fmt.Errorf("%w: %w", ErrIdentity, err)
	case errors.Is(err, vfs.ErrUnsupported):
		return fmt.Errorf("%w: %w", ErrUnsupported, err)
	case errors.Is(err, vfs.ErrIdentity):
		return fmt.Errorf("%w: %w", ErrIdentity, err)
	case errors.Is(err, vfs.ErrClosed):
		return fmt.Errorf("%w: %w", ErrClosed, err)
	case errors.Is(err, vfs.ErrLock):
		return fmt.Errorf("%w: %w", ErrLock, err)
	default:
		return err
	}
}
func openFile(path string) (*vfs.File, error) {
	f, err := vfs.Open(path, vfs.OpenOptions{Role: vfs.SHM, Flags: os.O_RDWR | os.O_CREATE})
	return f, nativeError(err)
}
