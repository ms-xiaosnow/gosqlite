//go:build linux

package walindex

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func openFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
}
func validateLinks(info os.FileInfo) error {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Nlink != 1 {
		return fmt.Errorf("%w: sidecar must have exactly one hard link", ErrIdentity)
	}
	return nil
}
func fileLock(f *os.File, start int64, n int, kind int16) error {
	lock := syscall.Flock_t{Type: kind, Whence: 0, Start: start, Len: int64(n)}
	if err := syscall.FcntlFlock(f.Fd(), 37, &lock); err != nil {
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EACCES) {
			return ErrBusy
		}
		if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOSYS) {
			return fmt.Errorf("%w: OFD lock: %v", ErrUnsupported, err)
		}
		return fmt.Errorf("WAL-index OFD lock: %w", err)
	}
	return nil
}
func attachDMS(f *os.File) error {
	// A failed exclusive probe must never fall through to a shared acquisition:
	// the first opener may still be truncating. SQLite retries the entire open.
	lock := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: 0, Start: 128, Len: 1}
	if err := syscall.FcntlFlock(f.Fd(), 36, &lock); err != nil {
		return fmt.Errorf("%w: DMS probe: %v", ErrUnsupported, err)
	}
	switch lock.Type {
	case syscall.F_UNLCK:
		if err := fileLock(f, 128, 1, syscall.F_WRLCK); err != nil {
			return err
		}
		if err := f.Truncate(3); err != nil {
			return fmt.Errorf("initialize WAL-index: %w", err)
		}
	case syscall.F_RDLCK:
	case syscall.F_WRLCK:
		return ErrBusy
	default:
		return fmt.Errorf("%w: DMS lock type", ErrUnsupported)
	}
	return fileLock(f, 128, 1, syscall.F_RDLCK)
}
func setRangeLock(f *os.File, slot, n int, exclusive, unlock bool) error {
	kind := int16(syscall.F_RDLCK)
	if exclusive {
		kind = syscall.F_WRLCK
	}
	if unlock {
		kind = syscall.F_UNLCK
	}
	return fileLock(f, int64(120+slot), n, kind)
}
func mapFile(f *os.File, off int64, size int) ([]byte, error) {
	return syscall.Mmap(int(f.Fd()), off, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
}
func unmapFile(b []byte) error { return syscall.Munmap(b) }
