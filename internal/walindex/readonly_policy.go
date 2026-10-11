package walindex

import (
	"errors"
	"os"
)

// ErrReadOnly reports an operation requiring mutable WAL-index access.
var ErrReadOnly = errors.New("SQLite WAL-index is read-only")

// These descriptor-free policies are shared by the real opener and guards.
// Platform rejection happens before path resolution or native acquisition.
func readonlyIndexOpenProfile(readOnly bool, goos string) error {
	if readOnly && goos != "linux" {
		return ErrUnsupported
	}
	return nil
}

func readonlyIndexOpenFlags(readOnly bool) int {
	if readOnly {
		return os.O_RDONLY
	}
	return os.O_RDWR | os.O_CREATE
}

func readonlyIndexWriteAccess(readOnly bool) error {
	if readOnly {
		return ErrReadOnly
	}
	return nil
}
