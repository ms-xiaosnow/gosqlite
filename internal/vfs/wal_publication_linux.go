//go:build linux

package vfs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const nativeWALPublicationMask = unix.STATX_TYPE | unix.STATX_NLINK | unix.STATX_MTIME | unix.STATX_CTIME | unix.STATX_INO | unix.STATX_SIZE | unix.STATX_MNT_ID_UNIQUE
const nativeWALPublicationResolve = unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_XDEV

type walPublicationLease struct {
	file *File
	fd int // installed before attachment; fd >= 0 is a terminal owned obligation
	attempted bool
}
func newWALPublicationLease(role Role) walPublicationLease {
	return walPublicationLease{file: &File{role: role, writable: role == WAL, replaceable: role == WAL, maps: make(map[*Mapping]struct{})}, fd: -1}
}
func (l *walPublicationLease) attach(path string) error {
	if l.fd < 0 { return ErrPoisoned }
	l.file.raw = os.NewFile(uintptr(l.fd), path)
	if l.file.raw == nil { return ErrPoisoned }
	l.file.path = path
	owners.Lock()
	owners.files[l.file] = struct{}{}
	owners.Unlock()
	l.fd = -1
	return nil
}
func (l *walPublicationLease) retire() (bool, error) {
	// Attachment failure retains the raw number; never close/retry an uncertain
	// possibly reused descriptor. The composite retains directory and main.
	if l.fd >= 0 { return false, ErrPoisoned }
	err := l.file.Close()
	return l.file.RetirementComplete(), err
}

type nativeWALPublication struct {
	main, original, candidate *File
	dir, child walPublicationLease
	mainInfo, originalInfo os.FileInfo
	parent, mainFact, originalFact walPublicationFact
	shmFact walPublicationFact
	base, candidateBase string
	shmPresent, renamed, transferred, sealed bool
	replaceAttempted, shmAttempted, barrierAttempted bool
}
func newNativeWALPublication(main, original *File, mainInfo, originalInfo os.FileInfo) walPublicationOps {
	child := newWALPublicationLease(WAL)
	return &nativeWALPublication{main: main, original: original, candidate: child.file,
		dir: newWALPublicationLease(Directory), child: child, mainInfo: mainInfo, originalInfo: originalInfo}
}
func (n *nativeWALPublication) allowedFiles() []*File {
	return []*File{n.main, n.original, n.candidate, n.dir.file}
}
func nativeWALPublicationError(err error) error {
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EXDEV) {
		return errors.Join(ErrUnsupported, err)
	}
	return err
}
func nativeWALPublicationStatx(fd int, name string, flags int) (walPublicationFact, error) {
	var st unix.Statx_t
	if err := unix.Statx(fd, name, flags, nativeWALPublicationMask, &st); err != nil { return walPublicationFact{}, nativeWALPublicationError(err) }
	if st.Mask&nativeWALPublicationMask != nativeWALPublicationMask { return walPublicationFact{}, ErrUnsupported }
	fact := walPublicationFact{mask: st.Mask, mode: st.Mode, links: st.Nlink, devMajor: st.Dev_major, devMinor: st.Dev_minor,
		inode: st.Ino, mount: st.Mnt_id, size: st.Size, mtimeSec: st.Mtime.Sec, mtimeNsec: st.Mtime.Nsec, ctimeSec: st.Ctime.Sec, ctimeNsec: st.Ctime.Nsec}
	if !fact.complete() { return walPublicationFact{}, ErrUnsupported }
	return fact, nil
}
func nativeWALPublicationHeld(f *File) (walPublicationFact, error) {
	if f.raw == nil { return walPublicationFact{}, ErrClosed }
	return nativeWALPublicationStatx(int(f.raw.Fd()), "", unix.AT_EMPTY_PATH|unix.AT_SYMLINK_NOFOLLOW)
}
func (n *nativeWALPublication) named(base string) (walPublicationFact, error) {
	if n.dir.file.raw == nil { return walPublicationFact{}, ErrClosed }
	return nativeWALPublicationStatx(int(n.dir.file.raw.Fd()), base, unix.AT_SYMLINK_NOFOLLOW)
}
func (n *nativeWALPublication) lockFiles() (func(), error) {
	if n.main == nil || n.original == nil || n.candidate == nil || n.main == n.original || n.main == n.candidate || n.original == n.candidate { return nil, ErrLock }
	n.main.mu.Lock()
	n.original.mu.Lock()
	n.candidate.mu.Lock()
	n.dir.file.mu.Lock()
	return func() { n.dir.file.mu.Unlock(); n.candidate.mu.Unlock(); n.original.mu.Unlock(); n.main.mu.Unlock() }, nil
}
func (n *nativeWALPublication) guard() error {
	if n.sealed { return ErrPoisoned }
	if err := n.main.checkLocked(); err != nil { return err }
	if n.main.role != Database || !n.main.writable || !n.main.shared || !n.main.reserved || !n.main.exclusive || n.main.dms || len(n.main.maps) != 0 { return ErrLock }
	if n.original.role != WAL || !n.original.writable || !n.original.replaceable || n.original.dms || len(n.original.maps) != 0 { return ErrLock }
	if n.mainInfo == nil || n.originalInfo == nil || n.main.parent == nil || n.original.parent == nil ||
		n.original.path != n.main.path+"-wal" || !SameFile(n.main.parent, n.original.parent) { return ErrIdentity }
	return nil
}
func (n *nativeWALPublication) directoryValid() error {
	if n.dir.file.raw == nil || n.dir.file.identity == nil { return ErrClosed }
	fact, err := nativeWALPublicationHeld(n.dir.file)
	if err != nil { return err }
	if !fact.samePhysical(n.parent) || fact.mount != n.parent.mount || fact.mode&walPublicationTypeMask != walPublicationDirectory { return ErrIdentity }
	named, err := statParentPath(filepath.Dir(n.main.path))
	if err != nil { return err }
	if !SameFile(n.dir.file.identity, named) || !SameFile(n.dir.file.identity, n.main.parent) || !SameFile(n.dir.file.identity, n.original.parent) { return ErrIdentity }
	return nil
}
func (n *nativeWALPublication) observe(f *File, base string, expected os.FileInfo) (walPublicationFact, error) {
	if err := f.checkLocked(); err != nil { return walPublicationFact{}, err }
	held, err := nativeWALPublicationHeld(f)
	if err != nil { return walPublicationFact{}, err }
	named, err := n.named(base)
	if err != nil { return walPublicationFact{}, err }
	if err = walPublicationDirectChild(n.parent, held, named); err != nil { return walPublicationFact{}, err }
	info, err := f.raw.Stat()
	if err != nil { return walPublicationFact{}, err }
	if f.identity == nil || !SameFile(info, f.identity) || expected != nil &&
		(!SameFile(info, expected) || info.Size() != expected.Size() || !info.ModTime().Equal(expected.ModTime())) { return walPublicationFact{}, ErrIdentity }
	return held, nil
}
func (n *nativeWALPublication) absent(base string) error {
	_, err := n.named(base)
	if errors.Is(err, unix.ENOENT) { return nil }
	if err != nil { return err }
	return ErrIdentity
}
func (n *nativeWALPublication) inspectSHM(initial bool) error {
	fact, err := n.named(n.base+"-shm")
	if errors.Is(err, unix.ENOENT) {
		if !initial && n.shmPresent { return ErrIdentity }
		return nil
	}
	if err != nil { return err }
	if err = walPublicationDirectChild(n.parent, fact, fact); err != nil { return err }
	if fact.samePhysical(n.mainFact) || fact.samePhysical(n.originalFact) { return ErrIdentity }
	if initial { n.shmPresent, n.shmFact = true, fact; return nil }
	if !n.shmPresent || !fact.stable(n.shmFact) { return ErrIdentity }
	return nil
}
func (n *nativeWALPublication) acquire() error {
	unlock, err := n.lockFiles()
	if err != nil { return err }
	defer unlock()
	if err = n.guard(); err != nil { return err }
	if n.dir.attempted { return ErrPoisoned }
	n.base = filepath.Base(n.main.path)
	if !walPublicationBasename(n.base) { return ErrUnsupported }
	n.dir.attempted = true
	n.dir.fd, err = unix.Open(filepath.Dir(n.main.path), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil { return nativeWALPublicationError(err) }
	if err = n.dir.attach(filepath.Dir(n.main.path)); err != nil { return err }
	n.dir.file.identity, err = n.dir.file.raw.Stat()
	if err != nil { return err }
	if !n.dir.file.identity.IsDir() || !SameFile(n.dir.file.identity, n.main.parent) || !SameFile(n.dir.file.identity, n.original.parent) { return ErrIdentity }
	var fs unix.Statfs_t
	if err = unix.Fstatfs(int(n.dir.file.raw.Fd()), &fs); err != nil { return err }
	// Initial admission signal only; this magic proves neither ext4 features,
	// trustworthy storage/cache behavior nor physical crash durability.
	if fs.Type != unix.EXT4_SUPER_MAGIC { return ErrUnsupported }
	n.parent, err = nativeWALPublicationHeld(n.dir.file)
	if err != nil { return err }
	if err = n.directoryValid(); err != nil { return err }
	n.mainFact, err = n.observe(n.main, n.base, n.mainInfo)
	if err != nil { return err }
	n.originalFact, err = n.observe(n.original, n.base+"-wal", n.originalInfo)
	if err != nil { return err }
	if n.mainFact.samePhysical(n.originalFact) { return ErrIdentity }
	if err = n.absent(n.base+"-journal"); err != nil { return err }
	return n.inspectSHM(true)
}
func (n *nativeWALPublication) sourceStable() error {
	if err := n.guard(); err != nil { return err }
	if err := n.directoryValid(); err != nil { return err }
	main, err := n.observe(n.main, n.base, n.mainInfo)
	if err != nil { return err }
	old, err := n.observe(n.original, n.base+"-wal", n.originalInfo)
	if err != nil { return err }
	if !main.stable(n.mainFact) || !old.stable(n.originalFact) { return ErrIdentity }
	if err = n.absent(n.base+"-journal"); err != nil { return err }
	return n.inspectSHM(false)
}
func (n *nativeWALPublication) candidateValid() (os.FileInfo, error) {
	if n.candidate.role != WAL || !n.candidate.writable || !n.candidate.replaceable || n.candidate.dms || len(n.candidate.maps) != 0 { return nil, ErrLock }
	base := n.candidateBase
	if n.renamed { base = n.base+"-wal" }
	fact, err := n.observe(n.candidate, base, nil)
	if err != nil { return nil, err }
	if err = walPublicationDistinct(n.mainFact, n.originalFact, fact, n.shmFact, n.shmPresent); err != nil { return nil, err }
	return n.candidate.raw.Stat()
}
func (n *nativeWALPublication) create() error {
	unlock, err := n.lockFiles()
	if err != nil { return err }
	defer unlock()
	if n.child.attempted || n.renamed { return ErrPoisoned }
	if err = n.sourceStable(); err != nil { return err }
	n.child.attempted = true
	var random [12]byte
	if _, err = rand.Read(random[:]); err != nil { return err }
	n.candidateBase = n.base+"-wal-publish-"+hex.EncodeToString(random[:])
	for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
		if err = n.absent(n.candidateBase+suffix); err != nil { return err }
	}
	how := unix.OpenHow{Flags: unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK,
		Mode: 0600, Resolve: nativeWALPublicationResolve}
	n.child.fd, err = unix.Openat2(int(n.dir.file.raw.Fd()), n.candidateBase, &how)
	if err != nil { return nativeWALPublicationError(err) }
	if err = n.child.attach(filepath.Join(n.dir.file.path, n.candidateBase)); err != nil { return err }
	n.candidate.identity, err = n.candidate.raw.Stat()
	if err != nil { return err }
	n.candidate.parent = n.dir.file.identity
	info, err := n.candidateValid()
	if err != nil { return err }
	if info.Size() != 0 { return ErrIdentity }
	return nil
}
func (n *nativeWALPublication) readSource(b []byte, off int64) (int, error) {
	unlock, err := n.lockFiles()
	if err != nil { return 0, err }
	defer unlock()
	if err = n.guard(); err != nil { return 0, err }
	if n.renamed || off < 0 || off > n.originalInfo.Size() || int64(len(b)) > n.originalInfo.Size()-off { return 0, ErrIdentity }
	if err = n.original.checkLocked(); err != nil { return 0, err }
	return n.original.raw.ReadAt(b, off)
}
func (n *nativeWALPublication) writeCandidate(b []byte, off int64) (int, error) {
	unlock, err := n.lockFiles()
	if err != nil { return 0, err }
	defer unlock()
	if err = n.guard(); err != nil { return 0, err }
	if n.renamed { return 0, ErrPoisoned }
	if err = n.candidate.checkLocked(); err != nil { return 0, err }
	return n.candidate.raw.WriteAt(b, off)
}
func (n *nativeWALPublication) candidateInfo() (os.FileInfo, error) {
	unlock, err := n.lockFiles()
	if err != nil { return nil, err }
	defer unlock()
	if err = n.guard(); err != nil { return nil, err }
	if err = n.directoryValid(); err != nil { return nil, err }
	return n.candidateValid()
}
func (n *nativeWALPublication) recheck() error {
	unlock, err := n.lockFiles()
	if err != nil { return err }
	defer unlock()
	if n.renamed { return ErrPoisoned }
	if err = n.sourceStable(); err != nil { return err }
	if _, err = n.candidateValid(); err != nil { return err }
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		if err = n.absent(n.candidateBase+suffix); err != nil { return err }
	}
	return nil
}
func (n *nativeWALPublication) syncCandidate() error {
	unlock, err := n.lockFiles()
	if err != nil { return err }
	defer unlock()
	if err = n.guard(); err != nil { return err }
	if n.renamed { return ErrPoisoned }
	return n.candidate.raw.Sync()
}
func (n *nativeWALPublication) replace() (bool, error) {
	unlock, err := n.lockFiles()
	if err != nil { return false, err }
	defer unlock()
	if n.replaceAttempted { return false, ErrPoisoned }
	if err = n.sourceStable(); err != nil { return false, err }
	if _, err = n.candidateValid(); err != nil { return false, err }
	n.replaceAttempted = true
	// This overwrites the checked existing WAL under a cooperative namespace
	// contract; rename is not an inode compare-and-swap against hostile actors.
	err = unix.Renameat2(int(n.dir.file.raw.Fd()), n.candidateBase, int(n.dir.file.raw.Fd()), n.base+"-wal", 0)
	if err != nil { return false, nativeWALPublicationError(err) }
	n.renamed = true
	n.candidate.path = n.original.path
	return true, nil
}
func (n *nativeWALPublication) transfer(main, original *File) (*File, error) {
	// No syscall, status observation, Close, unlock, allocation or group callback.
	// Both candidate and original remain reachable before and after this swap.
	if n.sealed || !n.renamed || n.transferred || main != n.main || original != n.original ||
		n.child.file != n.candidate || n.child.fd >= 0 || n.candidate == nil || n.candidate == original || n.candidate == main { return nil, ErrPoisoned }
	n.child = walPublicationLease{file: original, fd: -1, attempted: true}
	n.transferred = true
	return n.candidate, nil
}
func (n *nativeWALPublication) postPublication() (os.FileInfo, error) {
	unlock, err := n.lockFiles()
	if err != nil { return nil, err }
	defer unlock()
	if err = n.guard(); err != nil { return nil, err }
	if !n.renamed || !n.transferred || n.shmAttempted { return nil, ErrPoisoned }
	if err = n.directoryValid(); err != nil { return nil, err }
	main, err := n.observe(n.main, n.base, n.mainInfo)
	if err != nil { return nil, err }
	if !main.stable(n.mainFact) { return nil, ErrIdentity }
	info, err := n.candidateValid()
	if err != nil { return nil, err }
	if err = n.absent(n.base+"-journal"); err != nil { return nil, err }
	if err = n.absent(n.candidateBase); err != nil { return nil, err }
	if err = n.inspectSHM(false); err != nil { return nil, err }
	n.shmAttempted = true
	if n.shmPresent {
		if err = unix.Unlinkat(int(n.dir.file.raw.Fd()), n.base+"-shm", 0); err != nil { return nil, err }
	}
	if err = n.absent(n.base+"-shm"); err != nil { return nil, err }
	if err = n.absent(n.base+"-journal"); err != nil { return nil, err }
	if _, err = n.candidateValid(); err != nil { return nil, err }
	return info, nil
}
func (n *nativeWALPublication) syncDirectory() error {
	unlock, err := n.lockFiles()
	if err != nil { return err }
	defer unlock()
	if n.sealed || n.barrierAttempted || n.dir.file.raw == nil { return ErrPoisoned }
	n.barrierAttempted = true
	// Do not call sourceStable here: a known replacement unlinks the old WAL,
	// and failed post-observation must still reach this one retained-dir barrier.
	return n.dir.file.raw.Sync()
}
func (n *nativeWALPublication) seal() {
	n.sealed = true
	// The transferred current WAL and borrowed main are deliberately untouched.
	for _, f := range []*File{n.child.file, n.dir.file} {
		f.mu.Lock()
		f.closing = true
		f.mu.Unlock()
	}
}
func (n *nativeWALPublication) closeChild() (bool, error) { return n.child.retire() }
func (n *nativeWALPublication) closeDirectory() (bool, error) { return n.dir.retire() }
