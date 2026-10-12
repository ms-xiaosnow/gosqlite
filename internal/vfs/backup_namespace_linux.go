//go:build linux

package vfs

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

const nativeBackupStatxMask = unix.STATX_TYPE | unix.STATX_NLINK | unix.STATX_INO | unix.STATX_SIZE | unix.STATX_MNT_ID_UNIQUE
const nativeBackupResolve = unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_XDEV

func nativeBackupCapabilityError(err error) error {
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EXDEV) {
		return errors.Join(ErrUnsupported, err)
	}
	return err
}
func nativeBackupAttach(l *backupLease, name string) error {
	// l.fd was already installed by the syscall assignment into its owner.
	if l.fd < 0 { return ErrPoisoned }
	l.file.raw = os.NewFile(uintptr(l.fd), name)
	if l.file.raw == nil { return ErrPoisoned }
	l.file.path = name
	owners.Lock()
	owners.files[l.file] = struct{}{}
	owners.Unlock()
	l.fd = -1 // sole ownership has moved into the already-retained File
	return nil
}
func nativeBackupFilesystem(l *backupLease) error {
	var st unix.Statfs_t
	if err := unix.Fstatfs(int(l.file.raw.Fd()), &st); err != nil { return err }
	// An admission signal only: 0xef53 is shared by ext-family variants. This
	// does not establish ext4 version/features, local-storage trust or durability.
	if st.Type != unix.EXT4_SUPER_MAGIC { return fmt.Errorf("%w: backup requires the initial ext-family filesystem profile", ErrUnsupported) }
	return nil
}
func nativeBackupOpenDirectory(l *backupLease, path string) error {
	if l.attempted { return ErrPoisoned }
	l.attempted = true
	var err error
	l.fd, err = unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil { return nativeBackupCapabilityError(err) }
	if err = nativeBackupAttach(l, path); err != nil { return err }
	l.file.identity, err = l.file.raw.Stat()
	if err != nil { return err }
	if !l.file.identity.IsDir() { return ErrUnsupported }
	if err = nativeBackupFilesystem(l); err != nil { return err }
	_, err = nativeBackupHeldFact(l.file)
	return err
}
func nativeBackupStatx(fd int, name string, flags int) (backupFact, error) {
	var st unix.Statx_t
	if err := unix.Statx(fd, name, flags, nativeBackupStatxMask, &st); err != nil { return backupFact{}, nativeBackupCapabilityError(err) }
	if st.Mask&nativeBackupStatxMask != nativeBackupStatxMask { return backupFact{}, ErrUnsupported }
	return backupFact{mask: st.Mask, mode: st.Mode, links: st.Nlink, devMajor: st.Dev_major, devMinor: st.Dev_minor, inode: st.Ino, mount: st.Mnt_id, size: st.Size}, nil
}
func nativeBackupHeldFact(f *File) (backupFact, error) {
	if f.raw == nil { return backupFact{}, ErrClosed }
	return nativeBackupStatx(int(f.raw.Fd()), "", unix.AT_EMPTY_PATH|unix.AT_SYMLINK_NOFOLLOW)
}
func nativeBackupNamedFact(dir *backupLease, base string) (backupFact, error) {
	// All callers supply either a validated main or its fixed sidecar suffix.
	return nativeBackupStatx(int(dir.file.raw.Fd()), base, unix.AT_SYMLINK_NOFOLLOW)
}
func nativeBackupContinuity(l *backupLease) error {
	if l.file.raw == nil || l.file.identity == nil { return ErrClosed }
	held, err := l.file.raw.Stat()
	if err != nil { return err }
	named, err := statParentPath(l.file.path)
	if err != nil { return errors.Join(ErrIdentity, err) }
	if !SameFile(held, l.file.identity) || !SameFile(held, named) { return ErrIdentity }
	return nil
}
func nativeBackupObserveSource(main *File, dir *backupLease, base string) (backupFact, error) {
	if err := nativeBackupContinuity(dir); err != nil { return backupFact{}, err }
	parent, err := nativeBackupHeldFact(dir.file)
	if err != nil { return backupFact{}, err }
	held, err := nativeBackupHeldFact(main)
	if err != nil { return backupFact{}, err }
	named, err := nativeBackupNamedFact(dir, base)
	if err != nil { return backupFact{}, err }
	if err = backupDirectChild(parent, held, named); err != nil { return backupFact{}, err }
	info, err := main.raw.Stat()
	if err != nil { return backupFact{}, err }
	if !SameFile(info, main.identity) || !SameFile(dir.file.identity, main.parent) { return backupFact{}, ErrIdentity }
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		fact, err := nativeBackupNamedFact(dir, base+suffix)
		if errors.Is(err, syscall.ENOENT) { continue }
		if err != nil { return backupFact{}, err }
		if err = backupSourceSidecar(parent, suffix, fact); err != nil { return backupFact{}, err }
	}
	return parent, nil
}
