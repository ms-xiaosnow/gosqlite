//go:build linux

package vfs

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type nativeBackupPublication struct {
	dir, main backupLease
	name string
	parent backupFact
	sealed, childUsesDirectory bool
}
func newNativeBackupPublication() backupPublicationIO {
	return &nativeBackupPublication{dir: newBackupLease(Directory), main: newBackupLease(Database)}
}
func (n *nativeBackupPublication) ownedFiles() []*File { return []*File{n.dir.file, n.main.file} }
func (n *nativeBackupPublication) openDirectory(path string) (backupFact, error) {
	if n.sealed { return backupFact{}, ErrPoisoned }
	if err := nativeBackupOpenDirectory(&n.dir, path); err != nil { return backupFact{}, err }
	fact, err := nativeBackupHeldFact(n.dir.file)
	if err != nil { return backupFact{}, err }
	n.parent = fact
	return fact, nil
}
func (n *nativeBackupPublication) directoryValid() error {
	if n.sealed || n.dir.file.raw == nil { return ErrPoisoned }
	if err := nativeBackupContinuity(&n.dir); err != nil { return err }
	fact, err := nativeBackupHeldFact(n.dir.file)
	if err != nil { return err }
	if !fact.samePhysical(n.parent) || fact.mount != n.parent.mount || fact.mode&backupTypeMask != backupDirectory { return ErrIdentity }
	return nil
}
func (n *nativeBackupPublication) absentFamily(base string) error {
	if !BackupBasename(base) { return ErrUnsupported }
	if err := n.directoryValid(); err != nil { return err }
	for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
		_, err := nativeBackupNamedFact(&n.dir, base+suffix)
		if errors.Is(err, unix.ENOENT) { continue }
		if err != nil { return err }
		return os.ErrExist
	}
	return nil
}
func (n *nativeBackupPublication) createCandidate(base string) error {
	if !BackupBasename(base) || n.main.attempted { return ErrUnsupported }
	if err := n.directoryValid(); err != nil { return err }
	n.main.attempted = true
	n.childUsesDirectory = true // reserved before acquisition, not after validation
	n.name = base
	how := unix.OpenHow{Flags: unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, Mode: 0600, Resolve: nativeBackupResolve}
	var err error
	n.main.fd, err = unix.Openat2(int(n.dir.file.raw.Fd()), base, &how)
	if err != nil { return nativeBackupCapabilityError(err) }
	if err = nativeBackupAttach(&n.main, filepath.Join(n.dir.file.path, base)); err != nil { return err }
	n.main.file.identity, err = n.main.file.raw.Stat()
	if err != nil { return err }
	_, err = n.validateCandidate()
	return err
}
func (n *nativeBackupPublication) validateCandidate() (uint64, error) {
	if n.sealed || !n.childUsesDirectory || n.main.file.raw == nil { return 0, ErrPoisoned }
	if err := n.directoryValid(); err != nil { return 0, err }
	held, err := nativeBackupHeldFact(n.main.file)
	if err != nil { return 0, err }
	named, err := nativeBackupNamedFact(&n.dir, n.name)
	if err != nil { return 0, err }
	if err = backupDirectChild(n.parent, held, named); err != nil { return 0, err }
	info, err := n.main.file.raw.Stat()
	if err != nil { return 0, err }
	if !SameFile(info, n.main.file.identity) { return 0, ErrIdentity }
	return held.size, nil
}
func (n *nativeBackupPublication) lockCandidate() error {
	if n.sealed { return ErrPoisoned }
	if _, err := n.validateCandidate(); err != nil { return err }
	for _, lock := range []DBLock{Shared, Reserved, Exclusive} {
		if err := n.main.file.LockDB(lock); err != nil { return err }
	}
	return nil
}
func (n *nativeBackupPublication) writeCandidate(page []byte, offset int64) (int, error) {
	if n.sealed || !n.main.file.exclusive { return 0, ErrPoisoned }
	return n.main.file.WriteAt(page, offset)
}
func (n *nativeBackupPublication) syncCandidate() error {
	if n.sealed || !n.main.file.exclusive { return ErrPoisoned }
	return n.main.file.Sync()
}
func (n *nativeBackupPublication) publish(candidate, final string) error {
	if n.sealed || !n.childUsesDirectory || !n.main.file.exclusive || candidate != n.name || !BackupBasename(final) { return ErrPoisoned }
	if err := unix.Renameat2(int(n.dir.file.raw.Fd()), candidate, int(n.dir.file.raw.Fd()), final, unix.RENAME_NOREPLACE); err != nil { return nativeBackupCapabilityError(err) }
	// The same open inode and lock survive rename; old stage name remains in
	// the parent state for diagnostics. Never rename back after a later failure.
	n.name = final
	n.main.file.path = filepath.Join(n.dir.file.path, final)
	return nil
}
func (n *nativeBackupPublication) validatePublished() error {
	if n.sealed || !n.main.file.exclusive { return ErrPoisoned }
	// Even if post-publication observation fails, attempt the single required
	// directory Sync. The error cannot roll publication back or claim success.
	_, validation := n.validateCandidate()
	var family error
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		_, err := nativeBackupNamedFact(&n.dir, n.name+suffix)
		if errors.Is(err, unix.ENOENT) { continue }
		if err == nil { err = os.ErrExist }
		family = errors.Join(family, err)
	}
	return errors.Join(validation, family)
}
func (n *nativeBackupPublication) syncDirectory() error {
	if n.sealed || !n.main.file.exclusive { return ErrPoisoned }
	return n.dir.file.Sync()
}
func (n *nativeBackupPublication) seal() error {
	if n.sealed { return nil }
	// All native methods are serialized by BackupPublication.mu. These seals
	// take only File.mu, perform no I/O, and do not release a protecting lock.
	n.sealed = true
	n.main.seal()
	n.dir.seal()
	n.childUsesDirectory = false
	return nil
}
func (n *nativeBackupPublication) closeDirectory() error {
	if !n.sealed || n.childUsesDirectory { return ErrPoisoned }
	return n.dir.close()
}
func (n *nativeBackupPublication) directoryComplete() bool { return n.dir.complete() }
func (n *nativeBackupPublication) closeMain() error {
	if !n.sealed || n.childUsesDirectory || !n.dir.complete() { return ErrPoisoned }
	return n.main.close()
}
func (n *nativeBackupPublication) mainComplete() bool { return n.main.complete() }
