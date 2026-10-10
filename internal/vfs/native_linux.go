//go:build linux

package vfs

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func nativeOpen(path string, opts OpenOptions) (*os.File, error) {
	flags := opts.Flags | syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC
	if opts.Role == Directory {
		flags |= syscall.O_DIRECTORY
	}
	return os.OpenFile(path, flags, 0600)
}
func nativeValidateLinks(f *File) error {
	info, err := f.raw.Stat()
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Nlink != 1 {
		return ErrLinks
	}
	return nil
}
func nativeSync(f *File) error            { return f.raw.Sync() }
func nativeClose(f *File) error           { return f.raw.Close() }
func nativeReplace(old, new string) error { return os.Rename(old, new) }

// OFD locks remain descriptor-local and conflict with SQLite's POSIX locks.
// Neither a sibling lease nor an unrelated close can release these locks.
func ofdLock(f *File, kind int16, start, length int64) error {
	l := syscall.Flock_t{Type: kind, Whence: 0, Start: start, Len: length}
	if err := syscall.FcntlFlock(f.raw.Fd(), 37, &l); err != nil {
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EACCES) {
			return ErrBusy
		}
		if errors.Is(err, syscall.ENOSYS) {
			return fmt.Errorf("%w: OFD lock: %v", ErrUnsupported, err)
		}
		return fmt.Errorf("SQLite OFD lock: %w", err)
	}
	return nil
}
func nativeShared(f *File) error {
	if err := ofdLock(f, syscall.F_RDLCK, PendingByte, 1); err != nil {
		return err
	}
	if err := ofdLock(f, syscall.F_RDLCK, SharedFirst, SharedSize); err != nil {
		if cleanup := ofdLock(f, syscall.F_UNLCK, PendingByte, 1); cleanup != nil {
			return errors.Join(ErrPoisoned, err, cleanup)
		}
		return err
	}
	if err := ofdLock(f, syscall.F_UNLCK, PendingByte, 1); err != nil {
		return errors.Join(ErrPoisoned, err)
	}
	return nil
}
func nativeReserved(f *File) error { return ofdLock(f, syscall.F_WRLCK, ReservedByte, 1) }
func nativeExclusive(f *File) error {
	if err := ofdLock(f, syscall.F_WRLCK, PendingByte, 1); err != nil {
		return err
	}
	if err := ofdLock(f, syscall.F_WRLCK, SharedFirst, SharedSize); err != nil {
		if cleanup := ofdLock(f, syscall.F_UNLCK, PendingByte, 1); cleanup != nil {
			return errors.Join(ErrPoisoned, err, cleanup)
		}
		return err
	}
	return nil
}
func nativeDowngrade(f *File) error {
	if err := ofdLock(f, syscall.F_RDLCK, SharedFirst, SharedSize); err != nil {
		return errors.Join(ErrPoisoned, err)
	}
	if err := ofdLock(f, syscall.F_UNLCK, PendingByte, 1); err != nil {
		return errors.Join(ErrPoisoned, err)
	}
	return nil
}
func nativeReservedHeld(f *File) (bool, error) {
	l := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: 0, Start: ReservedByte, Len: 1}
	if err := syscall.FcntlFlock(f.raw.Fd(), 36, &l); err != nil {
		return false, fmt.Errorf("reserved-lock probe: %w", err)
	}
	return l.Type != syscall.F_UNLCK, nil
}
func nativeAttachDMS(f *File) error {
	l := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: 0, Start: DMSByte, Len: 1}
	if err := syscall.FcntlFlock(f.raw.Fd(), 36, &l); err != nil {
		return fmt.Errorf("DMS probe: %w", err)
	}
	initialWriter := false
	switch l.Type {
	case syscall.F_UNLCK:
		if err := ofdLock(f, syscall.F_WRLCK, DMSByte, 1); err != nil {
			return err
		}
		initialWriter = true
		if err := f.raw.Truncate(3); err != nil {
			return errors.Join(ErrPoisoned, fmt.Errorf("initialize WAL-index: %w", err))
		}
	case syscall.F_RDLCK:
	case syscall.F_WRLCK:
		return ErrBusy
	default:
		return ErrUnsupported
	}
	err := ofdLock(f, syscall.F_RDLCK, DMSByte, 1)
	if err != nil && initialWriter {
		return errors.Join(ErrPoisoned, err)
	}
	return err
}
func nativeSHMLock(f *File, slot, n int, exclusive, unlock bool) error {
	kind := int16(syscall.F_RDLCK)
	if exclusive {
		kind = syscall.F_WRLCK
	}
	if unlock {
		kind = syscall.F_UNLCK
	}
	return ofdLock(f, kind, SHMFirst+int64(slot), int64(n))
}

type nativeMapping struct{ view []byte }

func nativeMapGranularity() int64 { return int64(os.Getpagesize()) }
func nativeMap(f *File, off int64, n int) (nativeMapping, []byte, error) {
	b, err := syscall.Mmap(int(f.raw.Fd()), off, n, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	return nativeMapping{view: b}, b, err
}
func nativeUnmap(m *nativeMapping) error {
	if err := syscall.Munmap(m.view); err != nil {
		return err
	}
	m.view = nil
	return nil
}
