//go:build darwin

package vfs

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ms-xiaosnow/gosqlite/internal/darwinfile"
	"golang.org/x/sys/unix"
)

// Darwin fcntl locks belong to the PROCESS, not a descriptor. Closing ANY
// descriptor of an inode drops ALL of this process's locks on that inode.
// The common darwinfile gate also covers encrypted storage and its probes.
// Consequently the gate covers raw open/registration, every lock transition,
// and every physical close. All library descriptors (including rejected opens,
// read-only probes and sidecars) enter this registry before they can be closed.
// File.mu is acquired before the gate; the gate never acquires any File.mu.
// Data I/O and query evaluation do not hold the gate.
// Unmanaged descriptors/another SQLite library in this process are unsupported.
type darwinIdentity = darwinfile.Identity

type darwinRange struct{ start, length int64 }

var (
	darwinPending  = darwinRange{PendingByte, 1}
	darwinReserved = darwinRange{ReservedByte, 1}
	darwinShared   = darwinRange{SharedFirst, SharedSize}
	darwinDMS      = darwinRange{DMSByte, 1}
)

const (
	darwinRead  = uint8(1)
	darwinWrite = uint8(2)
	// Bound descriptors retained while peers keep POSIX locks. Repeated probes
	// reuse idle descriptors; exhausting different simultaneous leases fails
	// before another raw open, rather than silently growing deferred storage.
	darwinMaxDescriptorsPerInode = darwinfile.MaxPerInode
)

type darwinDescriptor struct {
	raw          *os.File
	node         *darwinNode
	access       int
	role         Role
	idle         bool
	holds        map[darwinRange]uint8
	record       *darwinfile.Record
	admissionErr error
}

type darwinNode struct {
	identity    darwinIdentity
	descriptors map[*os.File]*darwinDescriptor
	// kernel reflects successful transitions only. A failed cleanup poisons
	// the entire inode; represented holds remain available to ordered Close.
	kernel   map[darwinRange]uint8
	poisoned bool
}

type nativeFile struct{ descriptor *darwinDescriptor }

var darwinRegistry = struct {
	*darwinfile.Registry
	nodes       map[darwinIdentity]*darwinNode
	descriptors map[*os.File]*darwinDescriptor
}{Registry: darwinfile.Process, nodes: make(map[darwinIdentity]*darwinNode), descriptors: make(map[*os.File]*darwinDescriptor)}

func darwinID(st *unix.Stat_t) darwinIdentity { return darwinIdentity{Device: st.Dev, Inode: st.Ino} }

func darwinStatFD(fd int, st *unix.Stat_t) error {
	for {
		err := unix.Fstat(fd, st)
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}

func nativeOpen(path string, opts OpenOptions) (*os.File, error) {
	darwinRegistry.Lock()
	defer darwinRegistry.Unlock()

	access := opts.Flags & (os.O_WRONLY | os.O_RDWR)
	// lstat is descriptor-free on Darwin. It is only a cache hint: the
	// descriptor's identity is checked again by the managed Open path.
	var named unix.Stat_t
	if unix.Lstat(path, &named) == nil {
		if n := darwinRegistry.nodes[darwinID(&named)]; n != nil {
			if n.poisoned {
				return nil, ErrPoisoned
			}
			if opts.Flags&os.O_EXCL == 0 && named.Mode&unix.S_IFMT != unix.S_IFLNK {
				for _, d := range n.descriptors {
					if d.idle && d.access == access && d.role == opts.Role {
						if !darwinRegistry.Reuse(d.record) {
							continue
						}
						if opts.Role != Directory {
							if _, err := d.raw.Seek(0, io.SeekStart); err != nil {
								return nil, errors.Join(err, darwinOwnerError(darwinRegistry.Retire(d.record)))
							}
						}
						d.idle = false
						return d.raw, nil
					}
				}
			}
			if len(n.descriptors) >= darwinMaxDescriptorsPerInode {
				return nil, fmt.Errorf("%w: managed inode descriptor limit", ErrUnsupported)
			}
		}
	}
	var hint *darwinIdentity
	if named.Ino != 0 {
		id := darwinID(&named)
		hint = &id
	}
	if err := darwinRegistry.Admit(hint); err != nil {
		return nil, darwinOwnerError(err)
	}

	flags := opts.Flags | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	if opts.Role == Directory {
		flags |= unix.O_DIRECTORY
	}
	fd, err := unix.Open(path, flags, 0600)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	var st unix.Stat_t
	if err = darwinStatFD(fd, &st); err != nil {
		cleanup := darwinRegistry.Quarantine(func() error { return unix.Close(fd) }, nil)
		return nil, errors.Join(ErrPoisoned, err, cleanup)
	}
	id := darwinID(&st)
	n := darwinRegistry.nodes[id]
	if n == nil {
		n = &darwinNode{identity: id, descriptors: make(map[*os.File]*darwinDescriptor), kernel: make(map[darwinRange]uint8)}
		darwinRegistry.nodes[id] = n
	}
	raw := os.NewFile(uintptr(fd), path)
	d := &darwinDescriptor{raw: raw, node: n, access: access, role: opts.Role, holds: make(map[darwinRange]uint8)}
	n.descriptors[raw], darwinRegistry.descriptors[raw] = d, d
	d.record, d.admissionErr = darwinRegistry.Register(id, raw.Close, func(err error) error {
		if err != nil {
			n.poisoned = true
			return nil
		}
		delete(n.descriptors, raw)
		delete(darwinRegistry.descriptors, raw)
		if len(n.descriptors) == 0 {
			delete(darwinRegistry.nodes, id)
		}
		return nil
	})
	// Even a raced pathname must enter the registry before any rejection.
	// Validation below is performed by nativeOpened and common Open.
	return raw, nil
}

func nativeOpened(f *File) error {
	darwinRegistry.Lock()
	defer darwinRegistry.Unlock()
	d := darwinRegistry.descriptors[f.raw]
	if d == nil || d.idle {
		return ErrPoisoned
	}
	f.native.descriptor = d
	if d.admissionErr != nil {
		return darwinOwnerError(d.admissionErr)
	}
	if err := darwinRegistry.Check(d.record); err != nil {
		return darwinOwnerError(err)
	}
	if d.node.poisoned || darwinRegistry.Error(d.node.identity) != nil {
		return ErrPoisoned
	}
	if len(d.node.descriptors) > darwinMaxDescriptorsPerInode {
		return fmt.Errorf("%w: managed inode descriptor limit", ErrUnsupported)
	}
	var fs unix.Statfs_t
	if err := unix.Fstatfs(int(d.raw.Fd()), &fs); err != nil {
		return err
	}
	if fs.Flags&unix.MNT_LOCAL == 0 {
		return fmt.Errorf("%w: Darwin native VFS requires a local filesystem", ErrUnsupported)
	}
	for _, other := range d.node.descriptors {
		if other != d && !other.idle && other.role != d.role {
			return fmt.Errorf("%w: file aliases a different managed role", ErrIdentity)
		}
	}
	return nil
}

func nativeCheck(f *File) error {
	darwinRegistry.Lock()
	defer darwinRegistry.Unlock()
	d := f.native.descriptor
	if d == nil || d.idle || d.node.poisoned {
		return ErrPoisoned
	}
	return darwinOwnerError(darwinRegistry.Check(d.record))
}

func nativeValidatePath(f *File) error {
	var st unix.Stat_t
	if err := darwinStatFD(int(f.raw.Fd()), &st); err != nil {
		return err
	}
	if darwinID(&st) != f.native.descriptor.node.identity {
		return ErrIdentity
	}
	return nil
}

func nativeValidateLinks(f *File) error {
	var st unix.Stat_t
	if err := darwinStatFD(int(f.raw.Fd()), &st); err != nil {
		return err
	}
	if st.Nlink != 1 {
		return ErrLinks
	}
	return nil
}

// Baseline policy is fsync for files and directories, as explicitly declared
// in ADR 0015. F_FULLFSYNC/device power-loss guarantees are not claimed.
func nativeSync(f *File) error            { return f.raw.Sync() }
func nativeReplace(old, new string) error { return os.Rename(old, new) }

func darwinFcntl(d *darwinDescriptor, r darwinRange, mode uint8) error {
	kind := int16(unix.F_UNLCK)
	if mode == darwinRead {
		kind = unix.F_RDLCK
	} else if mode == darwinWrite {
		kind = unix.F_WRLCK
	}
	l := unix.Flock_t{Type: kind, Whence: io.SeekStart, Start: r.start, Len: r.length}
	if err := unix.FcntlFlock(d.raw.Fd(), unix.F_SETLK, &l); err != nil {
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EACCES) {
			return ErrBusy
		}
		if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.ENOTSUP) {
			return fmt.Errorf("%w: Darwin fcntl: %v", ErrUnsupported, err)
		}
		return fmt.Errorf("Darwin fcntl: %w", err)
	}
	return nil
}

// darwinAggregateExcept calculates process ownership excluding a single
// logical connection. It never reads another File's portable cached state.
func darwinAggregateExcept(n *darwinNode, except *darwinDescriptor, r darwinRange) uint8 {
	mode := uint8(0)
	for _, d := range n.descriptors {
		if d != except && d.holds[r] > mode {
			mode = d.holds[r]
		}
	}
	return mode
}

func darwinAcquire(d *darwinDescriptor, r darwinRange, mode uint8) error {
	if d.holds[r] == mode {
		return nil
	}
	other := darwinAggregateExcept(d.node, d, r)
	if other == darwinWrite || mode == darwinWrite && other != 0 {
		return ErrBusy
	}
	if d.node.kernel[r] != mode {
		if err := darwinFcntl(d, r, mode); err != nil {
			return err
		}
		d.node.kernel[r] = mode
		darwinRegistry.SetLocks(d.node.identity, "vfs", true)
	}
	d.holds[r] = mode
	return nil
}

func darwinRelease(d *darwinDescriptor, r darwinRange) error {
	if d.holds[r] == 0 {
		return nil
	}
	remaining := darwinAggregateExcept(d.node, d, r)
	if d.node.kernel[r] != remaining {
		if err := darwinFcntl(d, r, remaining); err != nil {
			return err
		}
		if remaining == 0 {
			delete(d.node.kernel, r)
		} else {
			d.node.kernel[r] = remaining
		}
	}
	delete(d.holds, r)
	darwinRegistry.SetLocks(d.node.identity, "vfs", len(d.node.kernel) != 0)
	return darwinOwnerError(darwinRegistry.Drain(d.node.identity))
}

func darwinPoison(d *darwinDescriptor, errs ...error) error {
	d.node.poisoned = true
	darwinRegistry.Poison(d.node.identity, errors.Join(errs...))
	return errors.Join(append([]error{ErrPoisoned}, errs...)...)
}

func nativeShared(f *File) error {
	darwinRegistry.Lock()
	defer darwinRegistry.Unlock()
	d := f.native.descriptor
	if d.node.poisoned || darwinRegistry.Error(d.node.identity) != nil {
		return ErrPoisoned
	}
	if err := darwinAcquire(d, darwinPending, darwinRead); err != nil {
		return err
	}
	if err := darwinAcquire(d, darwinShared, darwinRead); err != nil {
		if cleanup := darwinRelease(d, darwinPending); cleanup != nil {
			return darwinPoison(d, err, cleanup)
		}
		return err
	}
	if err := darwinRelease(d, darwinPending); err != nil {
		return darwinPoison(d, err)
	}
	return nil
}

func nativeReserved(f *File) error {
	darwinRegistry.Lock()
	defer darwinRegistry.Unlock()
	d := f.native.descriptor
	if d.node.poisoned || darwinRegistry.Error(d.node.identity) != nil {
		return ErrPoisoned
	}
	return darwinAcquire(d, darwinReserved, darwinWrite)
}

func nativeExclusive(f *File) error {
	darwinRegistry.Lock()
	defer darwinRegistry.Unlock()
	d := f.native.descriptor
	if d.node.poisoned || darwinRegistry.Error(d.node.identity) != nil {
		return ErrPoisoned
	}
	if err := darwinAcquire(d, darwinPending, darwinWrite); err != nil {
		return err
	}
	if err := darwinAcquire(d, darwinShared, darwinWrite); err != nil {
		// F_SETLK conversion failure leaves this process's SHARED intact.
		if cleanup := darwinRelease(d, darwinPending); cleanup != nil {
			return darwinPoison(d, err, cleanup)
		}
		return err
	}
	return nil
}

func nativeDowngrade(f *File) error {
	darwinRegistry.Lock()
	defer darwinRegistry.Unlock()
	d := f.native.descriptor
	if d.node.poisoned || darwinRegistry.Error(d.node.identity) != nil {
		return ErrPoisoned
	}
	if err := darwinAcquire(d, darwinShared, darwinRead); err != nil {
		return darwinPoison(d, err)
	}
	if err := darwinRelease(d, darwinPending); err != nil {
		return darwinPoison(d, err)
	}
	return nil
}

func darwinQuery(d *darwinDescriptor, r darwinRange) (int16, error) {
	l := unix.Flock_t{Type: unix.F_WRLCK, Whence: io.SeekStart, Start: r.start, Len: r.length}
	if err := unix.FcntlFlock(d.raw.Fd(), unix.F_GETLK, &l); err != nil {
		return 0, fmt.Errorf("Darwin lock query: %w", err)
	}
	return l.Type, nil
}

func nativeReservedHeld(f *File) (bool, error) {
	darwinRegistry.Lock()
	defer darwinRegistry.Unlock()
	d := f.native.descriptor
	if d.node.poisoned || darwinRegistry.Error(d.node.identity) != nil {
		return false, ErrPoisoned
	}
	// F_GETLK deliberately omits this process's own locks.
	if d.node.kernel[darwinReserved] != 0 {
		return true, nil
	}
	kind, err := darwinQuery(d, darwinReserved)
	return kind != unix.F_UNLCK, err
}

func nativeAttachDMS(f *File) error {
	darwinRegistry.Lock()
	defer darwinRegistry.Unlock()
	d := f.native.descriptor
	if d.node.poisoned || darwinRegistry.Error(d.node.identity) != nil {
		return ErrPoisoned
	}
	if d.node.kernel[darwinDMS] == darwinRead {
		// Additional local Index leases share the live process hold. They must
		// never query their own POSIX lock and reset already mapped storage.
		return darwinAcquire(d, darwinDMS, darwinRead)
	}
	kind, err := darwinQuery(d, darwinDMS)
	if err != nil {
		return err
	}
	initial := false
	switch kind {
	case unix.F_UNLCK:
		if err = darwinAcquire(d, darwinDMS, darwinWrite); err != nil {
			// A lost initialization race is BUSY; do not fall through to read.
			return err
		}
		initial = true
		if err = d.raw.Truncate(3); err != nil {
			return darwinPoison(d, err)
		}
	case unix.F_RDLCK:
	case unix.F_WRLCK:
		return ErrBusy
	default:
		return ErrUnsupported
	}
	err = darwinAcquire(d, darwinDMS, darwinRead)
	if err != nil && initial {
		return darwinPoison(d, err)
	}
	return err
}

func nativeSHMLock(f *File, slot, n int, exclusive, unlock bool) error {
	darwinRegistry.Lock()
	defer darwinRegistry.Unlock()
	d := f.native.descriptor
	if d.node.poisoned || darwinRegistry.Error(d.node.identity) != nil {
		return ErrPoisoned
	}
	mode := darwinRead
	if exclusive {
		mode = darwinWrite
	}
	// Precheck all local conflicts/conversions before touching the kernel.
	if !unlock {
		for i := slot; i < slot+n; i++ {
			r := darwinRange{SHMFirst + int64(i), 1}
			if old := d.holds[r]; old != 0 && old != mode {
				return ErrLock
			}
			other := darwinAggregateExcept(d.node, d, r)
			if other == darwinWrite || exclusive && other != 0 {
				return ErrBusy
			}
		}
	}
	var acquired []darwinRange
	for i := slot; i < slot+n; i++ {
		r := darwinRange{SHMFirst + int64(i), 1}
		if unlock {
			if err := darwinRelease(d, r); err != nil {
				return darwinPoison(d, err)
			}
			continue
		}
		if d.holds[r] == mode {
			continue
		}
		if err := darwinAcquire(d, r, mode); err != nil {
			var cleanup []error
			for j := len(acquired) - 1; j >= 0; j-- {
				if release := darwinRelease(d, acquired[j]); release != nil {
					cleanup = append(cleanup, release)
				}
			}
			if len(cleanup) != 0 {
				return darwinPoison(d, append([]error{err}, cleanup...)...)
			}
			return err
		}
		acquired = append(acquired, r)
	}
	return nil
}

// darwinOwnerError preserves the public VFS error classification.
func darwinOwnerError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, darwinfile.ErrLimit) {
		return errors.Join(ErrUnsupported, err)
	}
	return errors.Join(ErrPoisoned, err)
}

func nativeClose(f *File) error {
	darwinRegistry.Lock()
	defer darwinRegistry.Unlock()
	d := f.native.descriptor
	if d == nil {
		// nativeOpened always associates a registered descriptor before it can
		// report a validation error, so this is an ownership invariant failure.
		return ErrPoisoned
	}
	if d.idle {
		return darwinOwnerError(darwinRegistry.Retire(d.record))
	}
	// All mappings of this owner were already retired by common Close. Release
	// slots before DMS, and main SHARED before PENDING. Other owners' aggregate
	// holds persist regardless of which descriptor originally acquired them.
	ordered := make([]darwinRange, 0, 12)
	for i := 0; i < 8; i++ {
		ordered = append(ordered, darwinRange{SHMFirst + int64(i), 1})
	}
	ordered = append(ordered, darwinDMS, darwinReserved, darwinShared, darwinPending)
	for _, r := range ordered {
		if err := darwinRelease(d, r); err != nil {
			return darwinPoison(d, err)
		}
	}
	d.idle = true
	return darwinOwnerError(darwinRegistry.Retire(d.record))
}

type nativeMapping struct{ view []byte }

func nativeMapGranularity() int64 { return int64(os.Getpagesize()) }
func nativeMap(f *File, off int64, n int) (nativeMapping, []byte, error) {
	b, err := unix.Mmap(int(f.raw.Fd()), off, n, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	return nativeMapping{view: b}, b, err
}
func nativeUnmap(m *nativeMapping) error {
	if err := unix.Munmap(m.view); err != nil {
		return err
	}
	m.view = nil
	return nil
}
