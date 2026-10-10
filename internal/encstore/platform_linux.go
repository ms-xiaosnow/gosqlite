//go:build linux

package encstore

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
)

func supported() error { return nil }
func openFile(path string, flags int) (*os.File, error) {
	return os.OpenFile(path, flags|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0600)
}
func validateFile(info os.FileInfo) error {
	if !info.Mode().IsRegular() {
		return ErrUnsupported
	}
	// os.FileInfo.Sys is syscall.Stat_t; use an independent no-follow stat via
	// descriptor in checkedFile instead of asserting x/sys's distinct Stat_t.
	return nil
}
func validateLinks(f *os.File) error {
	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil {
		return err
	}
	if st.Nlink != 1 {
		return ErrUnsupported
	}
	return nil
}
func lockFile(f *os.File, readOnly bool) error {
	typ := int16(unix.F_WRLCK)
	if readOnly {
		typ = unix.F_RDLCK
	}
	l := unix.Flock_t{Type: typ, Whence: 0, Start: 0, Len: 1}
	err := unix.FcntlFlock(f.Fd(), unix.F_OFD_SETLK, &l)
	if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EACCES) {
		return ErrBusy
	}
	return err
}
func renameAbsent(old, new string) error {
	return unix.Renameat2(unix.AT_FDCWD, old, unix.AT_FDCWD, new, unix.RENAME_NOREPLACE)
}

func openDirectory(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
}

func openFileAt(dir *os.File, name string, flags int) (*os.File, error) {
	fd, e := unix.Openat(int(dir.Fd()), name, flags|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0600)
	if e != nil {
		return nil, e
	}
	return os.NewFile(uintptr(fd), name), nil
}
func stableFileAt(dir *os.File, name string, f *os.File, info os.FileInfo) error {
	now, e := f.Stat()
	if e != nil {
		return e
	}
	var named, held unix.Stat_t
	if e = unix.Fstatat(int(dir.Fd()), name, &named, unix.AT_SYMLINK_NOFOLLOW); e != nil {
		return e
	}
	if e = unix.Fstat(int(f.Fd()), &held); e != nil {
		return e
	}
	if named.Mode&unix.S_IFMT != unix.S_IFREG || named.Dev != held.Dev || named.Ino != held.Ino || !os.SameFile(info, now) || now.Size() != info.Size() || !now.ModTime().Equal(info.ModTime()) {
		return ErrChanged
	}
	return validateLinks(f)
}
func renameFileAt(dir *os.File, old, new string, absent bool) error {
	if absent {
		return unix.Renameat2(int(dir.Fd()), old, int(dir.Fd()), new, unix.RENAME_NOREPLACE)
	}
	return unix.Renameat(int(dir.Fd()), old, int(dir.Fd()), new)
}
