//go:build linux

package encstore

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func supported() error { return nil }
func validateFile(info os.FileInfo) error {
	if !info.Mode().IsRegular() {
		return ErrUnsupported
	}
	return nil
}
func validateLinks(f *managedFile) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(false); err != nil {
		return err
	}
	var st unix.Stat_t
	if err := unix.Fstat(int(f.raw.Fd()), &st); err != nil {
		return err
	}
	if st.Nlink != 1 {
		return ErrUnsupported
	}
	return nil
}
func lockFile(f *managedFile, readOnly bool) error { return rangeLock(f, 0, readOnly, false) }
func rangeLock(f *managedFile, offset int64, shared, unlock bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(false); err != nil {
		return err
	}
	typ := int16(unix.F_WRLCK)
	if shared {
		typ = unix.F_RDLCK
	}
	if unlock {
		typ = unix.F_UNLCK
	}
	l := unix.Flock_t{Type: typ, Whence: 0, Start: offset, Len: 1}
	err := unix.FcntlFlock(f.raw.Fd(), unix.F_OFD_SETLK, &l)
	if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EACCES) {
		return ErrBusy
	}
	return err
}
func renameAbsent(old, new string) error {
	return unix.Renameat2(unix.AT_FDCWD, old, unix.AT_FDCWD, new, unix.RENAME_NOREPLACE)
}

func nativeOpenDirectory(path string) (*managedFile, error) {
	raw, err := os.OpenFile(path, os.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	return &managedFile{raw: raw, directory: true}, nil
}

func nativeOpenFileAt(dir *managedFile, name string, flags int, _ bool) (*managedFile, error) {
	dir.mu.Lock()
	defer dir.mu.Unlock()
	if err := dir.checkLocked(false); err != nil {
		return nil, err
	}
	if !dir.directory {
		return nil, ErrUnsupported
	}
	fd, err := unix.Openat(int(dir.raw.Fd()), name, flags|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	dir.children++
	return &managedFile{raw: os.NewFile(uintptr(fd), name), parent: dir, directory: name == "." && flags == os.O_RDONLY}, nil
}
func stableFileAt(dir *managedFile, name string, f *managedFile, info os.FileInfo) error {
	// Native operations hold the actual leases, never an unprotected Fd value.
	// The child-before-parent order matches close's detached pin release.
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(false); err != nil {
		return err
	}
	if dir == f {
		return ErrUnsupported
	}
	dir.mu.Lock()
	defer dir.mu.Unlock()
	if err := dir.checkLocked(true); err != nil {
		return err
	}
	now, err := f.raw.Stat()
	if err != nil {
		return err
	}
	var named, held unix.Stat_t
	if err = unix.Fstatat(int(dir.raw.Fd()), name, &named, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if err = unix.Fstat(int(f.raw.Fd()), &held); err != nil {
		return err
	}
	if named.Mode&unix.S_IFMT != unix.S_IFREG || named.Dev != held.Dev || named.Ino != held.Ino || !os.SameFile(info, now) || now.Size() != info.Size() || !now.ModTime().Equal(info.ModTime()) {
		return ErrChanged
	}
	if held.Nlink != 1 {
		return ErrUnsupported
	}
	return nil
}
func renameFileAt(dir *managedFile, old, new string, absent bool) error {
	dir.mu.Lock()
	defer dir.mu.Unlock()
	if err := dir.checkLocked(false); err != nil {
		return err
	}
	if absent {
		return unix.Renameat2(int(dir.raw.Fd()), old, int(dir.raw.Fd()), new, unix.RENAME_NOREPLACE)
	}
	return unix.Renameat(int(dir.raw.Fd()), old, int(dir.raw.Fd()), new)
}
