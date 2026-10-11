//go:build darwin

package encstore

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/darwinfile"
	"golang.org/x/sys/unix"
)

func supported() error                                   { return nil }
func platformOperation(encryption.Operation, Mode) error { return nil }
func platformPreflight(path string) error {
	d, e := openDirectory(filepath.Dir(path))
	if e != nil {
		return e
	}
	return d.Close()
}
func encryptedDarwinID(st *unix.Stat_t) darwinfile.Identity {
	return darwinfile.Identity{Device: st.Dev, Inode: st.Ino}
}
func encryptedDarwinStat(fd int, st *unix.Stat_t) error {
	for {
		e := unix.Fstat(fd, st)
		if !errors.Is(e, unix.EINTR) {
			return e
		}
	}
}
func encryptedDarwinComponent(name string) error {
	if name == "" || name == "." || name == ".." || len(name) > 255 || strings.ContainsAny(name, "/\x00") {
		return ErrUnsupported
	}
	return nil
}

// Root basenames are deliberately ASCII until Apple filesystem Unicode
// equivalence is independently implemented. Parent directory paths may be UTF-8.
func encryptedDarwinTarget(name string) error {
	if e := encryptedDarwinComponent(name); e != nil {
		return e
	}
	if len(name) > 177 {
		return ErrUnsupported
	}
	for _, c := range name {
		if c < 32 || c > 126 {
			return ErrUnsupported
		}
	}
	return nil
}
func canonicalPath(path string) (string, error) {
	p, e := filepath.Abs(path)
	if e != nil {
		return "", e
	}
	if e = encryptedDarwinTarget(filepath.Base(p)); e != nil {
		return "", e
	}
	dir, e := filepath.EvalSymlinks(filepath.Dir(p))
	if e != nil {
		return "", e
	}
	p = filepath.Join(dir, filepath.Base(p))
	if i, e := os.Lstat(p); e == nil && !i.Mode().IsRegular() {
		return "", ErrUnsupported
	} else if e != nil && !os.IsNotExist(e) {
		return "", e
	}
	return p, nil
}
func platformPathInfo(path string) (os.FileInfo, error) { return os.Lstat(path) }
func platformFamilyEntry(name, base string, dir *managedFile) (bool, error) {
	member, err := darwinFamilyEntry(name, base)
	if err != nil || member {
		return member, err
	}
	for i := range len(name) {
		if name[i] >= 128 {
			// Only this newly allowed Unicode nonmember needs a filesystem proof.
			// Inspect the actual managed enumeration descriptor, never a path or
			// an independently opened probe. ASCII local-filesystem policy is unchanged.
			if dir == nil {
				return false, ErrUnsupported
			}
			dir.mu.Lock()
			defer dir.mu.Unlock()
			raw, e := dir.rawLocked()
			if e != nil {
				return false, e
			}
			if !dir.descriptor.directory {
				return false, ErrUnsupported
			}
			var fs unix.Statfs_t
			if e = unix.Fstatfs(int(raw.Fd()), &fs); e != nil {
				return false, e
			}
			if !darwinUnicodeInventoryFilesystem(fs.Fstypename, fs.Fssubtype, fs.Flags&unix.MNT_LOCAL != 0) {
				return false, ErrUnsupported
			}
			return false, nil
		}
	}
	return false, nil
}

func validateFile(i os.FileInfo) error {
	if !i.Mode().IsRegular() {
		return ErrUnsupported
	}
	return nil
}
func encryptedDarwinInspect(d *encryptedDarwinDescriptor) error {
	var st unix.Stat_t
	if e := encryptedDarwinStat(int(d.raw.Fd()), &st); e != nil {
		return e
	}
	if encryptedDarwinID(&st) != d.node.id {
		return ErrChanged
	}
	kind := uint16(unix.S_IFREG)
	if d.directory {
		kind = unix.S_IFDIR
	}
	if st.Mode&unix.S_IFMT != kind || !d.directory && st.Nlink != 1 {
		return ErrUnsupported
	}
	var fs unix.Statfs_t
	if e := unix.Fstatfs(int(d.raw.Fd()), &fs); e != nil {
		return e
	}
	if fs.Flags&unix.MNT_LOCAL == 0 {
		return ErrUnsupported
	}
	return nil
}
func encryptedDarwinDropParent(parent *managedFile) error {
	if parent == nil {
		return nil
	}
	if parent.children <= 0 {
		return managedRetirementError(parent, ErrPoisoned)
	}
	parent.children--
	if parent.closed {
		if err := parent.closeHeld(); err != nil {
			return managedRetirementError(parent, err)
		}
	}
	if parent.retirementCompleteHeld() {
		managedRetirementComplete(parent)
	}
	return nil
}

// openHeld is called with the common gate held across open/fstat/register and
// validation. No rejected descriptor can bypass cross-domain close protection.
func encryptedDarwinOpenHeld(parent *managedFile, path, name string, flags int, directory bool) (*managedFile, error) {
	if flags & ^(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_EXCL) != 0 || flags&(os.O_WRONLY|os.O_RDWR) == os.O_WRONLY|os.O_RDWR {
		return nil, ErrUnsupported
	}
	dirfd := unix.AT_FDCWD
	lookup := path
	if parent != nil {
		if e := parent.checkHeld(false); e != nil {
			return nil, e
		}
		if !parent.descriptor.directory {
			return nil, ErrUnsupported
		}
		dirfd = int(parent.descriptor.raw.Fd())
		lookup = name
	}
	access := flags & (os.O_WRONLY | os.O_RDWR)
	var hint *darwinfile.Identity
	var named unix.Stat_t
	if unix.Fstatat(dirfd, lookup, &named, unix.AT_SYMLINK_NOFOLLOW) == nil {
		id := encryptedDarwinID(&named)
		hint = &id
		if n := encryptedDarwinNodes[id]; n != nil && flags&os.O_EXCL == 0 && !directory && named.Mode&unix.S_IFMT == unix.S_IFREG {
			for d := range n.descriptors {
				if !d.idle || d.access != access || d.directory != directory {
					continue
				}
				if d.parent != parent && (d.parent == nil || parent == nil || d.parent.descriptor.node.id != parent.descriptor.node.id) {
					continue
				}
				if e := darwinfile.Process.Error(id); e != nil {
					return nil, encryptedDarwinError(e)
				}
				// Reset sequential I/O only after checking record usability. Position
				// belongs to the deferred descriptor, which has no active logical owner.
				if !darwinfile.Process.Reuse(d.record) {
					continue
				}
				f := &managedFile{descriptor: d}
				d.idle = false
				d.lease = f
				d.path = path
				n.locks.attach(&f.locks)
				if d.parent != parent {
					previous := d.parent
					parent.children++
					d.parent = parent
					if e := encryptedDarwinDropParent(previous); e != nil {
						return f, e
					}
				}
				// Reuse already acquired a new lease. Return that owner on every
				// validation failure so the common boundary can register it first.
				if _, e := d.raw.Seek(0, io.SeekStart); e != nil {
					return f, e
				}
				if e := encryptedDarwinInspect(d); e != nil {
					return f, e
				}
				return f, nil
			}
		}
	}
	if e := darwinfile.Process.Admit(hint); e != nil {
		return nil, encryptedDarwinError(e)
	}
	nativeFlags := flags | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
	if directory {
		nativeFlags |= unix.O_DIRECTORY
	}
	fd, e := unix.Openat(dirfd, lookup, nativeFlags, 0600)
	if e != nil {
		return nil, &os.PathError{Op: "openat", Path: path, Err: e}
	}
	if parent != nil {
		parent.children++
	}
	var st unix.Stat_t
	if e = encryptedDarwinStat(fd, &st); e != nil {
		retired := false
		cleanup := darwinfile.Process.Quarantine(func() error { return unix.Close(fd) }, func(err error) error {
			if err == nil {
				err = encryptedDarwinDropParent(parent)
				retired = err == nil
				return err
			}
			return nil
		})
		failure := errors.Join(ErrPoisoned, e, cleanup)
		if !retired {
			// The process registry owns this unidentifiable resource. This
			// terminal marker never exposes or retries the numeric descriptor.
			failure = nativeOwnershipPending(failure)
		}
		return nil, failure
	}
	id := encryptedDarwinID(&st)
	n := encryptedDarwinNodes[id]
	if n == nil {
		n = &encryptedDarwinNode{id: id, descriptors: make(map[*encryptedDarwinDescriptor]struct{})}
		encryptedDarwinNodes[id] = n
	}
	d := &encryptedDarwinDescriptor{raw: os.NewFile(uintptr(fd), path), node: n, access: access, directory: directory, parent: parent, path: path}
	f := &managedFile{descriptor: d}
	d.lease = f
	n.descriptors[d] = struct{}{}
	n.locks.attach(&f.locks)
	d.record, e = darwinfile.Process.Register(id, d.raw.Close, func(err error) error {
		if err != nil {
			d.lease.closeErr = managedTerminalRetirementError(d.lease, encryptedDarwinError(err))
			return d.lease.closeErr
		}
		d.raw = nil
		delete(n.descriptors, d)
		if len(n.descriptors) == 0 {
			delete(encryptedDarwinNodes, id)
		}
		if err = encryptedDarwinDropParent(d.parent); err != nil {
			d.lease.closeErr = err
		}
		d.parent = nil
		if err == nil && d.lease.retirementCompleteHeld() {
			managedRetirementComplete(d.lease)
		}
		return err
	})
	if e == nil {
		e = encryptedDarwinInspect(d)
	}
	if e != nil {
		return f, e
	}
	return f, nil
}
func nativeOpenDirectory(path string) (*managedFile, error) {
	darwinfile.Process.Lock()
	defer darwinfile.Process.Unlock()
	return encryptedDarwinOpenHeld(nil, path, "", os.O_RDONLY, true)
}
func nativeOpenFileAt(dir *managedFile, name string, flags int, _ bool) (*managedFile, error) {
	directory := name == "." && flags == os.O_RDONLY
	if !directory {
		if e := encryptedDarwinComponent(name); e != nil {
			return nil, e
		}
	}
	dir.mu.Lock()
	defer dir.mu.Unlock()
	darwinfile.Process.Lock()
	defer darwinfile.Process.Unlock()
	if e := dir.checkHeld(false); e != nil {
		return nil, e
	}
	return encryptedDarwinOpenHeld(dir, filepath.Join(dir.descriptor.path, name), name, flags, directory)
}
func validateLinks(f *managedFile) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	darwinfile.Process.Lock()
	defer darwinfile.Process.Unlock()
	if e := f.checkHeld(false); e != nil {
		return e
	}
	return encryptedDarwinInspect(f.descriptor)
}
func encryptedDarwinDirectoryHeld(dir *managedFile, path string, info os.FileInfo, retained bool) error {
	if e := dir.checkHeld(retained); e != nil {
		return e
	}
	if !dir.descriptor.directory {
		return ErrUnsupported
	}
	if e := encryptedDarwinInspect(dir.descriptor); e != nil {
		return e
	}
	var named unix.Stat_t
	if e := unix.Lstat(path, &named); e != nil {
		return e
	}
	if named.Mode&unix.S_IFMT != unix.S_IFDIR || encryptedDarwinID(&named) != dir.descriptor.node.id {
		return ErrChanged
	}
	if info != nil {
		now, e := dir.descriptor.raw.Stat()
		if e != nil {
			return e
		}
		if !os.SameFile(now, info) {
			return ErrChanged
		}
	}
	return nil
}
func platformDirectoryCheck(dir *managedFile, path string, info os.FileInfo) error {
	dir.mu.Lock()
	defer dir.mu.Unlock()
	darwinfile.Process.Lock()
	defer darwinfile.Process.Unlock()
	return encryptedDarwinDirectoryHeld(dir, path, info, false)
}
func encryptedDarwinStableHeld(dir *managedFile, name string, f *managedFile, info os.FileInfo) error {
	if e := f.checkHeld(false); e != nil {
		return e
	}
	if e := encryptedDarwinDirectoryHeld(dir, dir.descriptor.path, nil, true); e != nil {
		return e
	}
	if e := encryptedDarwinInspect(f.descriptor); e != nil {
		return e
	}
	now, e := f.descriptor.raw.Stat()
	if e != nil {
		return e
	}
	var named unix.Stat_t
	if e = unix.Fstatat(int(dir.descriptor.raw.Fd()), name, &named, unix.AT_SYMLINK_NOFOLLOW); e != nil {
		return e
	}
	if named.Mode&unix.S_IFMT != unix.S_IFREG || named.Nlink != 1 || encryptedDarwinID(&named) != f.descriptor.node.id || !os.SameFile(info, now) || info.Size() != now.Size() || !info.ModTime().Equal(now.ModTime()) {
		return ErrChanged
	}
	return nil
}
func stableFileAt(dir *managedFile, name string, f *managedFile, info os.FileInfo) error {
	if e := encryptedDarwinComponent(name); e != nil {
		return e
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	darwinfile.Process.Lock()
	defer darwinfile.Process.Unlock()
	return encryptedDarwinStableHeld(dir, name, f, info)
}
func platformFileCheck(f *managedFile, path string, info os.FileInfo) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	darwinfile.Process.Lock()
	defer darwinfile.Process.Unlock()
	if e := f.checkHeld(false); e != nil {
		return e
	}
	d := f.descriptor
	if d.parent == nil || filepath.Dir(path) != d.parent.descriptor.path {
		return ErrChanged
	}
	return encryptedDarwinStableHeld(d.parent, filepath.Base(path), f, info)
}
func (d *encryptedDarwinDescriptor) nativeLock(offset int64, shared, unlock bool) error {
	kind := int16(unix.F_WRLCK)
	if shared {
		kind = unix.F_RDLCK
	}
	if unlock {
		kind = unix.F_UNLCK
	}
	l := unix.Flock_t{Type: kind, Whence: io.SeekStart, Start: offset, Len: 1}
	e := unix.FcntlFlock(d.raw.Fd(), unix.F_SETLK, &l)
	if errors.Is(e, unix.EAGAIN) || errors.Is(e, unix.EACCES) {
		return ErrBusy
	}
	return e
}
func lockFile(f *managedFile, readOnly bool) error {
	return rangeLock(f, lifecycleByte, readOnly, false)
}
func rangeLock(f *managedFile, offset int64, shared, unlock bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	darwinfile.Process.Lock()
	defer darwinfile.Process.Unlock()
	if e := f.checkHeld(false); e != nil {
		return e
	}
	d := f.descriptor
	e := d.node.locks.change(&f.locks, offset, shared, unlock, d.nativeLock)
	darwinfile.Process.SetLocks(d.node.id, "encstore", d.node.locks.any())
	if errors.Is(e, ErrPoisoned) {
		darwinfile.Process.Poison(d.node.id, e)
	}
	if cleanup := darwinfile.Process.Drain(d.node.id); cleanup != nil {
		e = errors.Join(e, encryptedDarwinError(cleanup))
	}
	return e
}

// Publication is relative to the retained directory. Initial publication uses
// native RENAME_EXCL; there is no check-then-rename or hard-link fallback.
func renameFileAt(dir *managedFile, old, new string, absent bool) (resultErr error) {
	if e := encryptedDarwinComponent(old); e != nil {
		return e
	}
	if e := encryptedDarwinComponent(new); e != nil {
		return e
	}
	source, e := openFileAt(dir, old, os.O_RDONLY)
	if e != nil {
		return e
	}
	defer func() { resultErr = closeResult(source, resultErr) }()
	source.mu.Lock()
	defer source.mu.Unlock()
	darwinfile.Process.Lock()
	defer darwinfile.Process.Unlock()
	if e = source.checkHeld(false); e != nil {
		return e
	}
	info, e := source.descriptor.raw.Stat()
	if e != nil {
		return e
	}
	if e = encryptedDarwinStableHeld(dir, old, source, info); e != nil {
		return e
	}
	fd := int(dir.descriptor.raw.Fd())
	if absent {
		e = unix.RenameatxNp(fd, old, fd, new, unix.RENAME_EXCL)
	} else {
		e = unix.Renameat(fd, old, fd, new)
	}
	if e != nil {
		return &os.PathError{Op: "renameat", Path: new, Err: e}
	}
	var st unix.Stat_t
	if e = unix.Fstatat(fd, new, &st, unix.AT_SYMLINK_NOFOLLOW); e != nil {
		return e
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG || st.Nlink != 1 || encryptedDarwinID(&st) != source.descriptor.node.id {
		return ErrChanged
	}
	return nil
}
func encryptedDarwinRename(old, new string, absent bool) error {
	if filepath.Dir(old) != filepath.Dir(new) {
		return ErrUnsupported
	}
	d, e := openDirectory(filepath.Dir(old))
	if e != nil {
		return e
	}
	return closeResult(d, renameFileAt(d, filepath.Base(old), filepath.Base(new), absent))
}
func renameAbsent(old, new string) error { return encryptedDarwinRename(old, new, true) }
func replaceFile(old, new string) error  { return encryptedDarwinRename(old, new, false) }
