//go:build windows && amd64

package vfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// This backend is deliberately restricted to local fixed NTFS volumes. Native
// runtime acceptance is separate from compiling these Win32 bindings.
// FILE_ID_INFO contains a volume serial plus the entire 128-bit file ID.
type winIdentity struct {
	volume uint64
	file   [16]byte
}
type winStandardInfo struct {
	allocationSize int64
	endOfFile      int64
	links          uint32
	deletePending  byte
	directory      byte
	_              [2]byte
}
type winAttributeTagInfo struct{ attributes, tag uint32 }

// Reviewed amd64 SYSTEM_INFO ABI. GetSystemInfo returns void, not BOOL.
type winSystemInfo struct {
	architecture, reserved            uint16
	pageSize                          uint32
	minimumAddress, maximumAddress    uintptr
	activeProcessorMask               uintptr
	processors, processorType         uint32
	allocationGranularity             uint32
	processorLevel, processorRevision uint16
}

var (
	_ [24]byte = [unsafe.Sizeof(winIdentity{})]byte{}
	_ [24]byte = [unsafe.Sizeof(winStandardInfo{})]byte{}
	_ [20]byte = [unsafe.Offsetof(winStandardInfo{}.deletePending)]byte{}
	_ [21]byte = [unsafe.Offsetof(winStandardInfo{}.directory)]byte{}
	_ [8]byte  = [unsafe.Sizeof(winAttributeTagInfo{})]byte{}
	_ [48]byte = [unsafe.Sizeof(winSystemInfo{})]byte{}
	_ [40]byte = [unsafe.Offsetof(winSystemInfo{}.allocationGranularity)]byte{}
)

const (
	winPending = iota
	winReserved
	winShared
	winDMS
	winProbe
	winSlot0
	winIntervals = winSlot0 + 8
)

type winNode struct {
	role     Role
	leases   int
	attached int
}

type nativeFile struct {
	id, parentID winIdentity
	canonical    string
	parentHandle windows.Handle
	node         *winNode
	// Each entry is one exact successful LockFileEx interval: 0 absent,
	// 1 shared, 2 exclusive. The probe entry is only transient.
	held         [winIntervals]uint8
	attached     bool
	poisoned     bool
	rawClosed    bool
	closeUnknown bool
	// A failed view creation can still own its mapping object if closing that
	// object failed. Such resources retire before DMS/descriptor release.
	retained []nativeMapping
}

// Files retain separate locking handles even in one process. The registry
// coordinates only same-inode roles and first-local-attachment DMS reset.
var winRegistry = struct {
	sync.Mutex
	nodes   map[winIdentity]*winNode
	pending map[*os.File]nativeFile
}{nodes: make(map[winIdentity]*winNode), pending: make(map[*os.File]nativeFile)}

// winPath accepts only unambiguous absolute drive paths, with an optional
// documented extended-path prefix. Reject ADS and DOS device spellings before
// CreateFile can interpret them as a different object type.
func winPath(path string) error {
	p := strings.ReplaceAll(path, "/", `\`)
	if strings.HasPrefix(p, `\\?\`) {
		p = p[4:]
	}
	if len(p) < 3 || !(p[0] >= 'A' && p[0] <= 'Z' || p[0] >= 'a' && p[0] <= 'z') || p[1] != ':' || p[2] != '\\' {
		return fmt.Errorf("%w: only local absolute drive paths are supported", ErrUnsupported)
	}
	for _, part := range strings.Split(p[3:], `\`) {
		if part == "" {
			continue
		}
		if strings.ContainsAny(part, ":\x00*?<>|\"") || strings.HasSuffix(part, " ") || strings.HasSuffix(part, ".") || part == "." || part == ".." {
			return fmt.Errorf("%w: ambiguous path or alternate data stream", ErrUnsupported)
		}
		name := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if name == "CON" || name == "PRN" || name == "AUX" || name == "NUL" || name == "CONIN$" || name == "CONOUT$" ||
			len(name) == 4 && (strings.HasPrefix(name, "COM") || strings.HasPrefix(name, "LPT")) && name[3] >= '0' && name[3] <= '9' ||
			name == "COM¹" || name == "COM²" || name == "COM³" || name == "LPT¹" || name == "LPT²" || name == "LPT³" {
			return fmt.Errorf("%w: DOS device pathname", ErrUnsupported)
		}
		for _, c := range part {
			if c < 32 {
				return ErrUnsupported
			}
		}
	}
	return nil
}

func winCreate(path string, access, disposition uint32, directory, follow, replaceable bool) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE)
	if replaceable {
		share |= windows.FILE_SHARE_DELETE
	}
	attrs := uint32(windows.FILE_ATTRIBUTE_NORMAL)
	if directory {
		attrs |= windows.FILE_FLAG_BACKUP_SEMANTICS
	}
	if !follow {
		attrs |= windows.FILE_FLAG_OPEN_REPARSE_POINT
	}
	// No inheritable security attributes, OVERLAPPED mode, DELETE_ON_CLOSE,
	// truncation disposition, or direct/uncached I/O is requested.
	h, err := windows.CreateFile(p, access, share, nil, disposition, attrs, 0)
	if err != nil {
		return 0, &os.PathError{Op: "CreateFile", Path: path, Err: err}
	}
	return h, nil
}

func winID(h windows.Handle) (id winIdentity, err error) {
	err = windows.GetFileInformationByHandleEx(h, windows.FileIdInfo, (*byte)(unsafe.Pointer(&id)), uint32(unsafe.Sizeof(id)))
	return
}
func winStandard(h windows.Handle) (info winStandardInfo, err error) {
	err = windows.GetFileInformationByHandleEx(h, windows.FileStandardInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	return
}
func winInspect(h windows.Handle, directory bool) (winIdentity, error) {
	var zero winIdentity
	kind, err := windows.GetFileType(h)
	if err != nil {
		return zero, err
	}
	if kind != windows.FILE_TYPE_DISK {
		return zero, ErrUnsupported
	}
	var tag winAttributeTagInfo
	if err = windows.GetFileInformationByHandleEx(h, windows.FileAttributeTagInfo, (*byte)(unsafe.Pointer(&tag)), uint32(unsafe.Sizeof(tag))); err != nil {
		return zero, err
	}
	if tag.attributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DEVICE|windows.FILE_ATTRIBUTE_OFFLINE|windows.FILE_ATTRIBUTE_RECALL_ON_OPEN|windows.FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS) != 0 {
		return zero, fmt.Errorf("%w: reparse, device, offline, or recalled storage", ErrUnsupported)
	}
	info, err := winStandard(h)
	if err != nil {
		return zero, err
	}
	if (info.directory != 0) != directory || info.deletePending != 0 {
		return zero, ErrUnsupported
	}
	return winID(h)
}
func winFinalPath(h windows.Handle) (string, error) {
	for size := uint32(512); size <= 32768; {
		buf := make([]uint16, size)
		n, err := windows.GetFinalPathNameByHandle(h, &buf[0], size, 0)
		if err != nil {
			return "", err
		}
		if n < size {
			path := windows.UTF16ToString(buf[:n])
			if err = winPath(path); err != nil {
				return "", err
			}
			return path, nil
		}
		size = n + 1
	}
	return "", ErrUnsupported
}
func winLocalNTFS(h windows.Handle, canonical string) error {
	var fsName [64]uint16
	var serial, maximumComponent, flags uint32
	if err := windows.GetVolumeInformationByHandle(h, nil, 0, &serial, &maximumComponent, &flags, &fsName[0], uint32(len(fsName))); err != nil {
		return err
	}
	if windows.UTF16ToString(fsName[:]) != "NTFS" {
		return fmt.Errorf("%w: native Windows VFS requires NTFS", ErrUnsupported)
	}
	p, err := windows.UTF16PtrFromString(canonical)
	if err != nil {
		return err
	}
	var root [32768]uint16
	if err = windows.GetVolumePathName(p, &root[0], uint32(len(root))); err != nil {
		return err
	}
	if windows.GetDriveType(&root[0]) != windows.DRIVE_FIXED {
		return fmt.Errorf("%w: native Windows VFS requires local fixed storage", ErrUnsupported)
	}
	return nil
}

func nativeOpen(path string, opts OpenOptions) (*os.File, error) {
	if err := winPath(path); err != nil {
		return nil, err
	}
	// Resolve only the directory alias, through a handle. Retaining this parent
	// without delete sharing stabilizes its namespace while the lease is live.
	parentPath := filepath.Dir(path)
	parent, err := winCreate(parentPath, windows.FILE_READ_ATTRIBUTES, windows.OPEN_EXISTING, true, true, false)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*os.File, error) { return nil, errors.Join(err, windows.CloseHandle(parent)) }
	parentID, err := winInspect(parent, true)
	if err != nil {
		return fail(err)
	}
	canonicalParent, err := winFinalPath(parent)
	if err != nil {
		return fail(err)
	}
	if err = winLocalNTFS(parent, canonicalParent); err != nil {
		return fail(err)
	}
	canonical := filepath.Join(canonicalParent, filepath.Base(path))
	if filepath.Clean(path) == parentPath {
		canonical = canonicalParent // A drive root used as Directory.
	}
	access := uint32(windows.GENERIC_READ)
	if opts.Flags&(os.O_WRONLY|os.O_RDWR) != 0 {
		access |= windows.GENERIC_WRITE
	}
	disposition := uint32(windows.OPEN_EXISTING)
	if opts.Flags&os.O_CREATE != 0 {
		disposition = windows.OPEN_ALWAYS
		if opts.Flags&os.O_EXCL != 0 {
			disposition = windows.CREATE_NEW
		}
	}
	if opts.Role == Directory {
		if opts.Flags != 0 {
			return fail(ErrUnsupported)
		}
		access = windows.FILE_READ_ATTRIBUTES
	}
	h, err := winCreate(canonical, access, disposition, opts.Role == Directory, false, opts.Replaceable)
	if err != nil {
		return fail(err)
	}
	raw := os.NewFile(uintptr(h), canonical)
	winRegistry.Lock()
	winRegistry.pending[raw] = nativeFile{canonical: canonical, parentHandle: parent, parentID: parentID}
	winRegistry.Unlock()
	return raw, nil
}
func nativeOpened(f *File) error {
	winRegistry.Lock()
	initial, ok := winRegistry.pending[f.raw]
	delete(winRegistry.pending, f.raw)
	winRegistry.Unlock()
	if !ok {
		return ErrPoisoned
	}
	f.native = initial
	h := windows.Handle(f.raw.Fd())
	id, err := winInspect(h, f.role == Directory)
	if err != nil {
		return err
	}
	f.native.id = id
	canonical, err := winFinalPath(h)
	if err != nil {
		return err
	}
	if err = winLocalNTFS(h, canonical); err != nil {
		return err
	}
	f.native.canonical = canonical
	winRegistry.Lock()
	defer winRegistry.Unlock()
	node := winRegistry.nodes[id]
	if node != nil && node.role != f.role {
		return fmt.Errorf("%w: one file used for different storage roles", ErrIdentity)
	}
	if node == nil {
		node = &winNode{role: f.role}
		winRegistry.nodes[id] = node
	}
	node.leases++
	f.native.node = node
	return nil
}
func nativeCheck(f *File) error {
	if f.native.poisoned || f.native.closeUnknown || f.native.rawClosed {
		return ErrPoisoned
	}
	return nil
}
func winPoison(f *File, err error) error {
	f.native.poisoned = true
	return errors.Join(ErrPoisoned, err)
}
func nativeValidateLinks(f *File) error {
	info, err := winStandard(windows.Handle(f.raw.Fd()))
	if err != nil {
		return err
	}
	if info.links != 1 || info.deletePending != 0 || info.directory != 0 {
		return ErrLinks
	}
	return nil
}
func nativeValidatePath(f *File) error {
	id, err := winInspect(windows.Handle(f.raw.Fd()), f.role == Directory)
	if err != nil {
		return err
	}
	if id != f.native.id {
		return ErrIdentity
	}
	parentID, err := winInspect(f.native.parentHandle, true)
	if err != nil || parentID != f.native.parentID {
		return errors.Join(ErrIdentity, err)
	}
	// Metadata-only probes use compatible sharing and reject final reparse
	// points. Parent aliases are allowed only if their resolved identity agrees.
	for _, target := range []struct {
		path              string
		directory, follow bool
		want              winIdentity
	}{
		{f.path, f.role == Directory, false, f.native.id},
		{filepath.Dir(f.path), true, true, f.native.parentID},
	} {
		h, e := winCreate(target.path, windows.FILE_READ_ATTRIBUTES, windows.OPEN_EXISTING, target.directory, target.follow, f.replaceable && !target.directory)
		if e != nil {
			return errors.Join(ErrIdentity, e)
		}
		actual, inspectErr := winInspect(h, target.directory)
		closeErr := windows.CloseHandle(h)
		if inspectErr != nil || closeErr != nil || actual != target.want {
			return errors.Join(ErrIdentity, inspectErr, closeErr)
		}
	}
	return nil
}

func nativeSync(f *File) error {
	if f.role == Directory {
		// Deliberately absent, as with SQLite win32's syncDir policy. This is
		// not a successful Unix-style namespace durability barrier. See the
		// Windows capability declaration; interruption durability is unqualified.
		return nil
	}
	return windows.FlushFileBuffers(windows.Handle(f.raw.Fd()))
}

func winInterval(which int) (int64, int64) {
	switch which {
	case winPending:
		return PendingByte, 1
	case winReserved, winProbe:
		return ReservedByte, 1
	case winShared:
		return SharedFirst, SharedSize
	case winDMS:
		return DMSByte, 1
	default:
		return SHMFirst + int64(which-winSlot0), 1
	}
}
func winAcquire(f *File, which int, exclusive bool) error {
	mode := uint8(1)
	if exclusive {
		mode = 2
	}
	if f.native.held[which] != 0 {
		if f.native.held[which] == mode {
			return nil
		}
		return ErrLock
	}
	off, n := winInterval(which)
	o := windows.Overlapped{Offset: uint32(off), OffsetHigh: uint32(uint64(off) >> 32)}
	flags := uint32(windows.LOCKFILE_FAIL_IMMEDIATELY)
	if exclusive {
		flags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}
	err := windows.LockFileEx(windows.Handle(f.raw.Fd()), flags, 0, uint32(n), uint32(uint64(n)>>32), &o)
	if err != nil {
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return ErrBusy
		}
		if errors.Is(err, windows.ERROR_IO_PENDING) {
			return winPoison(f, fmt.Errorf("unexpected asynchronous LockFileEx result: %w", err))
		}
		return fmt.Errorf("LockFileEx: %w", err)
	}
	f.native.held[which] = mode
	return nil
}
func winRelease(f *File, which int) error {
	if f.native.held[which] == 0 {
		return nil
	}
	off, n := winInterval(which)
	o := windows.Overlapped{Offset: uint32(off), OffsetHigh: uint32(uint64(off) >> 32)}
	if err := windows.UnlockFileEx(windows.Handle(f.raw.Fd()), 0, uint32(n), uint32(uint64(n)>>32), &o); err != nil {
		return winPoison(f, fmt.Errorf("UnlockFileEx: %w", err))
	}
	f.native.held[which] = 0
	return nil
}
func nativeShared(f *File) error {
	if err := winAcquire(f, winPending, false); err != nil {
		return err
	}
	err := winAcquire(f, winShared, false)
	if cleanup := winRelease(f, winPending); cleanup != nil {
		return winPoison(f, errors.Join(err, cleanup))
	}
	return err
}
func nativeReserved(f *File) error { return winAcquire(f, winReserved, true) }
func nativeExclusive(f *File) error {
	// Also required for direct SHARED -> EXCLUSIVE WAL-mode transitions.
	if err := winAcquire(f, winPending, true); err != nil {
		return err
	}
	if err := winRelease(f, winShared); err != nil {
		return err
	}
	if err := winAcquire(f, winShared, true); err != nil {
		// Restore snapshot protection before dropping the writer gate. BUSY
		// is retryable only after BOTH restoration and cleanup succeed.
		if restore := winAcquire(f, winShared, false); restore != nil {
			return winPoison(f, errors.Join(err, restore))
		}
		if cleanup := winRelease(f, winPending); cleanup != nil {
			return winPoison(f, errors.Join(err, cleanup))
		}
		return err
	}
	return nil
}
func nativeDowngrade(f *File) error {
	if f.native.held[winPending] != 2 || f.native.held[winShared] != 2 {
		return winPoison(f, ErrLock)
	}
	if err := winRelease(f, winShared); err != nil {
		return err
	}
	if err := winAcquire(f, winShared, false); err != nil {
		return winPoison(f, err)
	}
	return winRelease(f, winPending) // Keep this owner's RESERVED, if any.
}
func nativeReservedHeld(f *File) (bool, error) {
	if f.native.held[winReserved] != 0 {
		return true, nil
	}
	// A shared probe is legal even on a read-only database handle.
	if err := winAcquire(f, winProbe, false); err != nil {
		if errors.Is(err, ErrBusy) {
			return true, nil
		}
		return false, err
	}
	return false, winRelease(f, winProbe)
}
func nativeAttachDMS(f *File) error {
	winRegistry.Lock()
	defer winRegistry.Unlock()
	node := f.native.node
	if node == nil || node.role != SHM {
		return ErrLock
	}
	if node.attached == 0 {
		err := winAcquire(f, winDMS, true)
		if err == nil {
			// The Windows handshake resets to zero. There is intentionally no
			// fallback header erasure after an arbitrary truncation failure.
			if err = f.raw.Truncate(0); err != nil {
				return winPoison(f, err)
			}
			if err = winRelease(f, winDMS); err != nil {
				return err
			}
		} else if !errors.Is(err, ErrBusy) {
			return err
		}
	}
	if err := winAcquire(f, winDMS, false); err != nil {
		return err
	}
	node.attached++
	f.native.attached = true
	return nil
}
func nativeSHMLock(f *File, slot, n int, exclusive, unlock bool) error {
	if !validSlots(slot, n) || !unlock && !exclusive && n != 1 {
		return ErrLock
	}
	if unlock {
		for i := slot; i < slot+n; i++ {
			if err := winRelease(f, winSlot0+i); err != nil {
				return err
			}
		}
		return nil
	}
	mode := uint8(1)
	if exclusive {
		mode = 2
	}
	for i := slot; i < slot+n; i++ {
		if held := f.native.held[winSlot0+i]; held != 0 && held != mode {
			return ErrLock
		}
	}
	var newly [8]int
	count := 0
	for i := slot; i < slot+n; i++ {
		which := winSlot0 + i
		if f.native.held[which] != 0 {
			continue
		}
		if err := winAcquire(f, which, exclusive); err != nil {
			var rollback []error
			for j := count - 1; j >= 0; j-- {
				if e := winRelease(f, newly[j]); e != nil {
					rollback = append(rollback, e)
				}
			}
			if len(rollback) != 0 {
				return winPoison(f, errors.Join(append(rollback, err)...))
			}
			return err
		}
		newly[count] = which
		count++
	}
	return nil
}

func nativeClose(f *File) error {
	if f.native.closeUnknown {
		return ErrPoisoned // os.File.Close cannot safely be repeated after an error.
	}
	for len(f.native.retained) != 0 {
		i := len(f.native.retained) - 1
		if err := nativeUnmap(&f.native.retained[i]); err != nil {
			return winPoison(f, err)
		}
		f.native.retained = f.native.retained[:i]
	}
	winRegistry.Lock()
	defer winRegistry.Unlock()
	if !f.native.rawClosed {
		// Exact one-byte slot acquisitions, then DB protection, then DMS.
		for i := winIntervals - 1; i >= winSlot0; i-- {
			if err := winRelease(f, i); err != nil {
				return err
			}
		}
		for _, i := range []int{winShared, winReserved, winProbe, winPending, winDMS} {
			if err := winRelease(f, i); err != nil {
				return err
			}
		}
		if f.native.attached {
			f.native.node.attached--
			f.native.attached = false
		}
		if err := f.raw.Close(); err != nil {
			f.native.closeUnknown = true
			return winPoison(f, err)
		}
		f.native.rawClosed = true
	}
	if f.native.parentHandle != 0 {
		if err := windows.CloseHandle(f.native.parentHandle); err != nil {
			return winPoison(f, err)
		}
		f.native.parentHandle = 0
	}
	if node := f.native.node; node != nil {
		node.leases--
		if node.leases == 0 {
			delete(winRegistry.nodes, f.native.id)
		}
		f.native.node = nil
	}
	return nil
}

func nativeReplace(old, new string) error {
	// ReplaceWAL already checks EXCLUSIVE ownership, both Replaceable flags,
	// identities, and the common directory. No cross-volume copy fallback.
	if err := winPath(old); err != nil {
		return err
	}
	if err := winPath(new); err != nil {
		return err
	}
	from, err := windows.UTF16PtrFromString(old)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(new)
	if err != nil {
		return err
	}
	if err := windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH); err != nil {
		// No caller may keep using a possibly uncertain publication result.
		// The portable gate retains and poisons every participating lease.
		return errors.Join(ErrPoisoned, err)
	}
	return nil
}

type nativeMapping struct {
	base   uintptr
	length int
	handle windows.Handle
	owner  *File
}

var winMappingSystem = struct {
	sync.Once
	granularity int64
}{}

func nativeMapGranularity() int64 {
	winMappingSystem.Do(func() {
		proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetSystemInfo")
		if err := proc.Find(); err != nil {
			return
		}
		var info winSystemInfo
		proc.Call(uintptr(unsafe.Pointer(&info))) // Return and last-error are undefined.
		g, p := info.allocationGranularity, info.pageSize
		if g == 0 || p == 0 || g&(g-1) != 0 || p&(p-1) != 0 || g < p || g%p != 0 {
			return
		}
		winMappingSystem.granularity = int64(g)
	})
	return winMappingSystem.granularity
}
func nativeMap(f *File, off int64, n int) (nativeMapping, []byte, error) {
	var empty nativeMapping
	g := nativeMapGranularity()
	if off < 0 || n <= 0 || g == 0 || off%g != 0 {
		return empty, nil, ErrUnsupported
	}
	// Zero maximum size means current backing size, so CreateFileMapping
	// cannot extend storage merely to satisfy allocation-granularity rounding.
	h, err := windows.CreateFileMapping(windows.Handle(f.raw.Fd()), nil, windows.PAGE_READWRITE, 0, 0, nil)
	if err != nil {
		return empty, nil, err
	}
	m := nativeMapping{handle: h, length: n, owner: f}
	base, err := windows.MapViewOfFile(h, windows.FILE_MAP_READ|windows.FILE_MAP_WRITE, uint32(uint64(off)>>32), uint32(off), uintptr(n))
	if err != nil {
		if closeErr := windows.CloseHandle(h); closeErr != nil {
			f.native.retained = append(f.native.retained, m)
			return empty, nil, winPoison(f, errors.Join(err, closeErr))
		}
		return empty, nil, err
	}
	m.base = base
	return m, unsafe.Slice((*byte)(unsafe.Pointer(base)), n), nil
}
func nativeUnmap(m *nativeMapping) error {
	// Views retain a mapping object reference independently of its HANDLE.
	// Closing the object handle first keeps Bytes live if either stage fails;
	// DMS and the file handle remain held until the view is actually unmapped.
	if m.handle != 0 {
		if err := windows.CloseHandle(m.handle); err != nil {
			return winPoison(m.owner, err)
		}
		m.handle = 0
	}
	if m.base != 0 {
		if err := windows.UnmapViewOfFile(m.base); err != nil {
			return winPoison(m.owner, err)
		}
		m.base = 0
	}
	m.length = 0
	return nil
}
