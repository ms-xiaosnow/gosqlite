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
