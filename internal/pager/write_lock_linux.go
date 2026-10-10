//go:build linux

package pager

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func writeLock(f *os.File, typ int16, start, length int64) error {
	l := syscall.Flock_t{Type: typ, Whence: 0, Start: start, Len: length}
	err := syscall.FcntlFlock(f.Fd(), 37, &l) // Linux F_OFD_SETLK.
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EACCES) {
		return ErrBusy
	}
	return fmt.Errorf("SQLite file lock: %w", err)
}
func lockReserved(f *os.File) error { return writeLock(f, syscall.F_WRLCK, 0x40000001, 1) }
func lockExclusive(f *os.File) error {
	if err := writeLock(f, syscall.F_WRLCK, 0x40000000, 1); err != nil {
		return err
	}
	if err := writeLock(f, syscall.F_WRLCK, 0x40000002, 510); err != nil {
		if cleanup := writeLock(f, syscall.F_UNLCK, 0x40000000, 1); cleanup != nil {
			return fmt.Errorf("%w: exclusive acquisition: %v; pending unlock: %v", ErrPoisoned, err, cleanup)
		}
		return err
	}
	return nil
}
func validateWritableFile(info os.FileInfo) error {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || s.Nlink != 1 {
		return fmt.Errorf("%w: writable database must have exactly one hard link", ErrUnsupported)
	}
	return nil
}
func unlockExclusive(f *os.File) error {
	if err := writeLock(f, syscall.F_RDLCK, 0x40000002, 510); err != nil {
		return err
	}
	return writeLock(f, syscall.F_UNLCK, 0x40000000, 1)
}

// Symlinks are resolved before opening. Refuse a changed or dangling final
// symlink rather than creating sidecars beside the wrong database pathname.
func openWritableDatabase(path string, flags int) (*os.File, error) {
	return os.OpenFile(path, flags|syscall.O_NOFOLLOW, 0600)
}
