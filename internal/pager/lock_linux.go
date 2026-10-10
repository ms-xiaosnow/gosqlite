//go:build linux

package pager

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// Linux open-file-description locks conflict with SQLite POSIX locks, but
// unlike POSIX process-scoped locks survive closes of other Reader handles.
// Shared locking follows SQLite: briefly lock PENDING_BYTE, acquire the full
// shared range, then release PENDING_BYTE. This blocks writers' EXCLUSIVE lock.
func lockShared(f *os.File) error {
	const ofdSetLK = 37 // Linux F_OFD_SETLK, introduced in Linux 3.15.
	set := func(typ int16, start, length int64) error {
		lock := syscall.Flock_t{Type: typ, Whence: 0, Start: start, Len: length}
		return syscall.FcntlFlock(f.Fd(), ofdSetLK, &lock)
	}
	classify := func(err error) error {
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EACCES) {
			return fmt.Errorf("%w: shared lock", ErrBusy)
		}
		return fmt.Errorf("%w: native SQLite-compatible shared lock: %v", ErrUnsupported, err)
	}
	if err := set(syscall.F_RDLCK, 0x40000000, 1); err != nil {
		return classify(err)
	}
	if err := set(syscall.F_RDLCK, 0x40000002, 510); err != nil {
		_ = set(syscall.F_UNLCK, 0x40000000, 1)
		return classify(err)
	}
	if err := set(syscall.F_UNLCK, 0x40000000, 1); err != nil {
		return classify(err)
	}
	return nil
}
