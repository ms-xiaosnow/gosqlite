//go:build windows && amd64

package encstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"
	"unsafe"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"golang.org/x/sys/windows"
)

// The amd64 ABI and native filesystem contract are deliberately explicit.
// These are independent from the plain SQLite VFS. No mapping is used here.
type encryptedWinID struct {
	volume uint64
	file   [16]byte
}
type encryptedWinStandard struct {
	allocation, size    int64
	links               uint32
	deleting, directory byte
	_                   [2]byte
}
type encryptedWinTag struct{ attributes, tag uint32 }
type encryptedWinRename struct {
	flags uint32
	root  windows.Handle
	bytes uint32
	name  [260]uint16 // accepted basename is bounded to 255 UTF-16 code units
}

var (
	_ [24]byte = [unsafe.Sizeof(encryptedWinID{})]byte{}
	_ [24]byte = [unsafe.Sizeof(encryptedWinStandard{})]byte{}
	_ [20]byte = [unsafe.Offsetof(encryptedWinStandard{}.deleting)]byte{}
	_ [21]byte = [unsafe.Offsetof(encryptedWinStandard{}.directory)]byte{}
	_ [8]byte  = [unsafe.Sizeof(encryptedWinTag{})]byte{}
	_ [8]byte  = [unsafe.Offsetof(encryptedWinRename{}.root)]byte{}
	_ [16]byte = [unsafe.Offsetof(encryptedWinRename{}.bytes)]byte{}
	_ [20]byte = [unsafe.Offsetof(encryptedWinRename{}.name)]byte{}
)

// managedFile never exposes its raw handle. Parent references outlive all child
// leases, including unreachable poisoned children. Global retention prevents an
// os.File finalizer from releasing unresolved native locks or directory pins.
type managedFile struct {
	managedOwnership
	mu                     sync.Mutex
	raw                    *os.File
	path                   string
	id                     encryptedWinID
	directory, replaceable bool
	parent                 *managedFile
	handleLifetime
	locks exactByteLocks
}

var encryptedWinOwners = struct {
	sync.Mutex
	files   map[*managedFile]struct{}
	opening int
}{files: make(map[*managedFile]struct{})}

const encryptedWinMaxHandles = 16384

func supported() error {
	major, _, build := windows.RtlGetNtVersionNumbers()
	if major < 10 || major == 10 && build < 16299 {
		return fmt.Errorf("%w: encrypted Windows backend requires Windows 10 1709 or later", ErrUnsupported)
	}
	return nil
}
func platformOperation(op encryption.Operation, mode Mode) error {
	if err := supported(); err != nil {
		return err
	}
	if op == encryption.OperationRekeyTarget && mode == WALShared {
		return fmt.Errorf("%w: shared encrypted rekey has no atomic Windows lifecycle upgrade", ErrUnsupported)
	}
	return nil
}
func platformPreflight(path string) error {
	if err := supported(); err != nil {
		return err
	}
	d, err := openDirectory(filepath.Dir(path))
	if err != nil {
		return err
	}
	return d.Close()
}
func encryptedWinComponent(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "\\/:\x00*?<>|\"") || strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") {
		return ErrUnsupported
	}
	for _, c := range name {
		if c < 32 {
			return ErrUnsupported
		}
	}
	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CONIN$" || base == "CONOUT$" ||
		len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9' ||
		base == "COM¹" || base == "COM²" || base == "COM³" || base == "LPT¹" || base == "LPT²" || base == "LPT³" {
		return ErrUnsupported
	}
	u, err := windows.UTF16FromString(name)
	if err != nil {
		return err
	}
	if len(u)-1 > 255 {
		return ErrUnsupported
	}
	return nil
}
func platformInputPath(path string) error {
	if path == "" || !utf8.ValidString(path) {
		return ErrUnsupported
	}
	p := strings.ReplaceAll(path, "/", `\`)
	if strings.HasPrefix(p, `\\?\`) {
		p = p[4:]
	}
	// Absolute local drive paths and ordinary relative names only. Reject raw
	// UNC/device/drive-relative spelling before filepath.Abs can normalize it.
	if strings.HasPrefix(p, `\`) {
		return ErrUnsupported
	}
	if len(p) >= 2 && p[1] == ':' {
		if len(p) < 3 || p[2] != '\\' || !(p[0] >= 'A' && p[0] <= 'Z' || p[0] >= 'a' && p[0] <= 'z') {
			return ErrUnsupported
		}
		p = p[3:]
	}
	for _, part := range strings.Split(p, `\`) {
		if part == "" {
			continue
		}
		if err := encryptedWinComponent(part); err != nil {
			return err
		}
	}
	return nil
}
func encryptedWinAbsolute(path string) error {
	if err := platformInputPath(path); err != nil {
		return err
	}
	p := strings.TrimPrefix(strings.ReplaceAll(path, "/", `\`), `\\?\`)
	if len(p) < 3 || p[1] != ':' || p[2] != '\\' {
		return ErrUnsupported
	}
	return nil
}
func encryptedWinError(op, path string, err error) error {
	if err == nil {
		return nil
	}
	if status, ok := err.(windows.NTStatus); ok {
		err = status.Errno()
	}
	return &os.PathError{Op: op, Path: path, Err: err}
}
func encryptedWinInspect(h windows.Handle, directory bool) (id encryptedWinID, err error) {
	kind, err := windows.GetFileType(h)
	if err != nil {
		return id, err
	}
	if kind != windows.FILE_TYPE_DISK {
		return id, ErrUnsupported
	}
	var tag encryptedWinTag
	if err = windows.GetFileInformationByHandleEx(h, windows.FileAttributeTagInfo, (*byte)(unsafe.Pointer(&tag)), uint32(unsafe.Sizeof(tag))); err != nil {
		return id, err
	}
	if tag.attributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DEVICE|windows.FILE_ATTRIBUTE_OFFLINE|windows.FILE_ATTRIBUTE_RECALL_ON_OPEN|windows.FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS) != 0 {
		return id, ErrUnsupported
	}
	var info encryptedWinStandard
	if err = windows.GetFileInformationByHandleEx(h, windows.FileStandardInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		return id, err
	}
	if info.deleting != 0 || (info.directory != 0) != directory || !directory && info.links != 1 {
		return id, ErrUnsupported
	}
	err = windows.GetFileInformationByHandleEx(h, windows.FileIdInfo, (*byte)(unsafe.Pointer(&id)), uint32(unsafe.Sizeof(id)))
	return
}
func encryptedWinFinalPath(h windows.Handle) (string, error) {
	for size := uint32(512); size <= 32768; {
		buf := make([]uint16, size)
		n, err := windows.GetFinalPathNameByHandle(h, &buf[0], size, 0)
		if err != nil {
			return "", err
		}
		if n < size {
			p := windows.UTF16ToString(buf[:n])
			if err = encryptedWinAbsolute(p); err != nil {
				return "", err
			}
			return p, nil
		}
		size = n + 1
	}
	return "", ErrUnsupported
}
func encryptedWinLocal(h windows.Handle) error {
	var fs [64]uint16
	if err := windows.GetVolumeInformationByHandle(h, nil, 0, nil, nil, nil, &fs[0], uint32(len(fs))); err != nil {
		return err
	}
	if windows.UTF16ToString(fs[:]) != "NTFS" {
		return fmt.Errorf("%w: encrypted Windows backend requires NTFS", ErrUnsupported)
	}
	path, err := encryptedWinFinalPath(h)
	if err != nil {
		return err
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var root [32768]uint16
	if err = windows.GetVolumePathName(p, &root[0], uint32(len(root))); err != nil {
		return err
	}
	if windows.GetDriveType(&root[0]) != windows.DRIVE_FIXED {
		return ErrUnsupported
	}
	return nil
}

// Reserve BEFORE an OS open. Unknown close state never decrements the bound.
func encryptedWinReserve() error {
	encryptedWinOwners.Lock()
	defer encryptedWinOwners.Unlock()
	if len(encryptedWinOwners.files)+encryptedWinOwners.opening >= encryptedWinMaxHandles {
		return fmt.Errorf("%w: Windows encrypted handle capacity", encryption.ErrLimit)
	}
	encryptedWinOwners.opening++
	return nil
}
func encryptedWinAdopt(h windows.Handle, path string, directory, replaceable bool, parent *managedFile) *managedFile {
	f := &managedFile{raw: os.NewFile(uintptr(h), path), path: path, directory: directory, replaceable: replaceable, parent: parent}
	encryptedWinOwners.Lock()
	encryptedWinOwners.opening--
	encryptedWinOwners.files[f] = struct{}{}
	encryptedWinOwners.Unlock()
	return f
}
func encryptedWinCancelReserve() {
	encryptedWinOwners.Lock()
	encryptedWinOwners.opening--
	encryptedWinOwners.Unlock()
}
func encryptedWinValidateOpened(f *managedFile) (*managedFile, error) {
	var err error
	f.id, err = encryptedWinInspect(windows.Handle(f.raw.Fd()), f.directory)
	if err == nil && f.directory {
		err = encryptedWinLocal(windows.Handle(f.raw.Fd()))
	}
	if err != nil {
		return f, err
	}
	return f, nil
}
func nativeOpenDirectory(path string) (*managedFile, error) {
	if err := supported(); err != nil {
		return nil, err
	}
	if err := encryptedWinAbsolute(path); err != nil {
		return nil, err
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	if err = encryptedWinReserve(); err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.FILE_LIST_DIRECTORY|windows.FILE_TRAVERSE|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		encryptedWinCancelReserve()
		return nil, encryptedWinError("open encrypted directory", path, err)
	}
	return encryptedWinValidateOpened(encryptedWinAdopt(h, path, true, false, nil))
}
func encryptedWinOpenAt(dir *managedFile, name string, flags int, root, metadata, rename bool) (*managedFile, error) {
	return scopedNativeOpen(dir, func() (*managedFile, error) {
		return encryptedWinOpenAtPolicy(dir, name, flags, root, metadata, rename, false)
	})
}
func encryptedWinOpenAtPolicy(dir *managedFile, name string, flags int, root, metadata, rename, retained bool) (*managedFile, error) {
	directory := name == "."
	if directory {
		if flags != os.O_RDONLY || root || rename {
			return nil, ErrUnsupported
		}
	} else if err := encryptedWinComponent(name); err != nil {
		return nil, err
	}
	if flags & ^(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_EXCL) != 0 || flags&(os.O_WRONLY|os.O_RDWR) == os.O_WRONLY|os.O_RDWR {
		return nil, ErrUnsupported
	}
	dir.mu.Lock()
	if err := dir.checkParentLocked(retained); err != nil {
		dir.mu.Unlock()
		return nil, err
	}
	if !dir.directory {
		dir.mu.Unlock()
		return nil, ErrUnsupported
	}
	// NT opens the RootDirectory itself with an empty relative ObjectName.
	// Match pinned Go's OBJECT_ATTRIBUTES.init for the portable "." request.
	ntName := name
	if directory {
		ntName = ""
	}
	objectName, err := windows.NewNTUnicodeString(ntName)
	if err != nil {
		dir.mu.Unlock()
		return nil, err
	}
	// There is exactly one checked basename (or an empty self-directory
	// name), so FILE_OPEN_REPARSE_POINT covers every component resolved here.
	// Do not depend on version-dependent OBJ_DONT_REPARSE support or retry
	// with weaker flags. Inspect the opened handle before any data access.
	oa := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(dir.raw.Fd()), ObjectName: objectName, Attributes: windows.OBJ_CASE_INSENSITIVE}
	oa.Length = uint32(unsafe.Sizeof(oa))
	access := uint32(windows.FILE_GENERIC_READ)
	if flags&(os.O_WRONLY|os.O_RDWR) != 0 {
		access |= windows.FILE_GENERIC_WRITE
	}
	if metadata {
		access = windows.FILE_READ_ATTRIBUTES | windows.SYNCHRONIZE
	}
	if rename {
		access |= windows.DELETE
	}
	options := uint32(windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_OPEN_REPARSE_POINT)
	if directory {
		options |= windows.FILE_DIRECTORY_FILE
	} else {
		options |= windows.FILE_NON_DIRECTORY_FILE
	}
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE)
	if root || metadata || rename {
		share |= windows.FILE_SHARE_DELETE
	}
	disposition := uint32(windows.FILE_OPEN)
	if flags&os.O_CREATE != 0 {
		disposition = windows.FILE_OPEN_IF
		if flags&os.O_EXCL != 0 {
			disposition = windows.FILE_CREATE
		}
	}
	if err = dir.addChild(retained); err != nil {
		dir.mu.Unlock()
		return nil, err
	}
	if err = encryptedWinReserve(); err != nil {
		_ = dir.dropChild()
		dir.mu.Unlock()
		return nil, err
	}
	var h windows.Handle
	err = windows.NtCreateFile(&h, access, &oa, &windows.IO_STATUS_BLOCK{}, nil, windows.FILE_ATTRIBUTE_NORMAL, share, disposition, options, 0, 0)
	runtime.KeepAlive(objectName)
	if err != nil {
		encryptedWinCancelReserve()
		_ = dir.dropChild()
		dir.mu.Unlock()
		return nil, encryptedWinError("open encrypted relative", name, err)
	}
	f := encryptedWinAdopt(h, filepath.Join(dir.path, name), directory, root, dir)
	dir.mu.Unlock()
	return encryptedWinValidateOpened(f)
}
func nativeOpenFileAt(dir *managedFile, name string, flags int, root bool) (*managedFile, error) {
	return encryptedWinOpenAtPolicy(dir, name, flags, root, false, false, false)
}
func validateFile(info os.FileInfo) error {
	if !info.Mode().IsRegular() {
		return ErrUnsupported
	}
	return nil
}
func validateLinks(f *managedFile) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return err
	}
	id, err := encryptedWinInspect(windows.Handle(f.raw.Fd()), false)
	if err == nil && id != f.id {
		return ErrChanged
	}
	return err
}
func platformDirectoryCheck(f *managedFile, path string, _ os.FileInfo) error {
	// Independent named observation detects ancestor retargeting. The returned
	// probe is managed too; generic os.Lstat/Go lazy file-ID opens are not used.
	f.mu.Lock()
	err := f.checkLocked()
	f.mu.Unlock()
	if err != nil {
		return err
	}
	probe, err := openDirectory(path)
	if err != nil {
		return err
	}
	f.mu.Lock()
	if err = f.checkLocked(); err == nil && probe.id != f.id {
		err = ErrChanged
	}
	f.mu.Unlock()
	return closeResult(probe, err)
}

func platformFileCheck(f *managedFile, path string, info os.FileInfo) error {
	f.mu.Lock()
	parent := f.parent
	err := f.checkLocked()
	f.mu.Unlock()
	if err != nil {
		return err
	}
	if parent == nil || filepath.Dir(path) != parent.path {
		return ErrChanged
	}
	return stableFileAt(parent, filepath.Base(path), f, info)
}
func stableFileAt(dir *managedFile, name string, f *managedFile, info os.FileInfo) error {
	// A metadata-only probe shares delete access even for a publication root.
	// This opens the name relative to the exact retained directory, not a rebuilt
	// absolute string or a case-folded identity approximation.
	probe, err := encryptedWinMetadataAt(dir, name)
	if err != nil {
		return err
	}
	f.mu.Lock()
	if err = f.checkLocked(); err == nil {
		var now os.FileInfo
		now, err = f.raw.Stat()
		if err == nil && (probe.id != f.id || !os.SameFile(info, now) || now.Size() != info.Size() || !now.ModTime().Equal(info.ModTime())) {
			err = ErrChanged
		}
		if err == nil {
			var id encryptedWinID
			id, err = encryptedWinInspect(windows.Handle(f.raw.Fd()), false)
			if err == nil && id != f.id {
				err = ErrChanged
			}
		}
	}
	f.mu.Unlock()
	return closeResult(probe, err)
}

// A child can continue using its pinned parent after the parent's public
// lease closes. The child count and actual handle are checked under one mutex;
// the public closed bit is never temporarily cleared.
func encryptedWinMetadataAt(dir *managedFile, name string) (*managedFile, error) {
	return scopedNativeOpen(dir, func() (*managedFile, error) {
		return encryptedWinOpenAtPolicy(dir, name, os.O_RDONLY, false, true, false, true)
	})
}
func lockFile(f *managedFile, readOnly bool) error {
	return rangeLock(f, lifecycleByte, readOnly, false)
}
func rangeLock(f *managedFile, offset int64, shared, unlock bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return err
	}
	if f.directory || !strings.HasSuffix(f.path, ".gse-lock") {
		return ErrUnsupported
	}
	return f.locks.change(offset, shared, unlock, f.nativeLock)
}
func (f *managedFile) nativeLock(offset int64, shared, unlock bool) error {
	ov := windows.Overlapped{Offset: uint32(offset), OffsetHigh: uint32(uint64(offset) >> 32)}
	h := windows.Handle(f.raw.Fd())
	if unlock {
		return windows.UnlockFileEx(h, 0, 1, 0, &ov)
	}
	flags := uint32(windows.LOCKFILE_FAIL_IMMEDIATELY)
	if !shared {
		flags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}
	err := windows.LockFileEx(h, flags, 0, 1, 0, &ov)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return ErrBusy
	}
	return err
}
func renameAbsent(old, new string) error {
	if filepath.Dir(old) != filepath.Dir(new) {
		return ErrUnsupported
	}
	d, err := openDirectory(filepath.Dir(old))
	if err != nil {
		return err
	}
	return closeResult(d, renameFileAt(d, filepath.Base(old), filepath.Base(new), true))
}
func renameFileAt(dir *managedFile, old, new string, absent bool) error {
	if old == new {
		return ErrUnsupported
	}
	if err := encryptedWinComponent(old); err != nil {
		return err
	}
	if err := encryptedWinComponent(new); err != nil {
		return err
	}
	source, err := encryptedWinOpenAt(dir, old, os.O_RDONLY, false, false, true)
	if err != nil {
		return err
	}
	// The caller has already authenticated this candidate and owns the required
	// lifecycle/capture gate. Recheck handle/name identity before cutover.
	info, err := source.Stat()
	if err == nil {
		err = stableFileAt(dir, old, source, info)
	}
	if err != nil {
		return closeResult(source, err)
	}
	name, err := windows.UTF16FromString(new)
	if err != nil {
		return closeResult(source, err)
	}
	ri := encryptedWinRename{bytes: uint32((len(name) - 1) * 2)}
	copy(ri.name[:], name)
	if !absent {
		ri.flags = windows.FILE_RENAME_REPLACE_IF_EXISTS | windows.FILE_RENAME_POSIX_SEMANTICS
	}
	dir.mu.Lock()
	if err = dir.checkLocked(); err == nil {
		source.mu.Lock()
		ri.root = windows.Handle(dir.raw.Fd())
		// FileRenameInformationEx = 65 per FILE_INFORMATION_CLASS. No fallback to
		// path-based MoveFileEx, deletion, copy, or weaker legacy replacement.
		err = windows.NtSetInformationFile(windows.Handle(source.raw.Fd()), &windows.IO_STATUS_BLOCK{}, (*byte)(unsafe.Pointer(&ri)), uint32(unsafe.Sizeof(ri)), 65)
		source.mu.Unlock()
	}
	dir.mu.Unlock()
	if err == nil {
		var dest *managedFile
		dest, err = encryptedWinMetadataAt(dir, new)
		if err == nil {
			if dest.id != source.id {
				err = ErrChanged
			}
			err = closeResult(dest, err)
		}
	}
	return closeResult(source, encryptedWinError("rename encrypted relative", new, err))
}

func replaceFile(old, new string) error {
	if filepath.Dir(old) != filepath.Dir(new) {
		return ErrUnsupported
	}
	d, err := openDirectory(filepath.Dir(old))
	if err != nil {
		return err
	}
	return closeResult(d, renameFileAt(d, filepath.Base(old), filepath.Base(new), false))
}

// Preserve os.IsNotExist/IsExist classification when cleanup itself succeeded;
// those legacy helpers do not unwrap an errors.Join node.
func encryptedWinJoin(err, cleanup error) error {
	if cleanup == nil {
		return err
	}
	if err == nil {
		return cleanup
	}
	return errors.Join(err, cleanup)
}
func canonicalPath(path string) (string, error) {
	if err := platformInputPath(path); err != nil {
		return "", err
	}
	p, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	// Reserve the longest future generated member, including retained roots,
	// before any native open, key provider or stable-lock creation.
	if err = windowsFamilyNameBound(filepath.Base(p)); err != nil {
		return "", err
	}
	d, err := openDirectory(filepath.Dir(p))
	if err != nil {
		return "", err
	}
	d.mu.Lock()
	parent, err := encryptedWinFinalPath(windows.Handle(d.raw.Fd()))
	d.mu.Unlock()
	err = closeResult(d, err)
	if err != nil {
		return "", err
	}
	p = filepath.Join(parent, filepath.Base(p))
	if _, err = platformPathInfo(p); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return p, nil
}
func platformPathInfo(path string) (os.FileInfo, error) {
	d, err := openDirectory(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	f, err := encryptedWinMetadataAt(d, filepath.Base(path))
	if err != nil {
		return nil, closeResult(d, err)
	}
	info, err := f.Stat()
	err = closeResult(f, err)
	err = closeResult(d, err)
	if err != nil {
		return nil, err
	}
	return info, nil
}

var encryptedWinCompareOrdinal = windows.NewLazySystemDLL("kernel32.dll").NewProc("CompareStringOrdinal")

// Use Windows' ordinal uppercase table for accounting names. This is never an
// inode identity test: those always compare native handle IDs. The generated
// family suffix/hex grammar remains strict after normalizing only the prefix.
func encryptedWinFoldPrefix(name, prefix string) (string, bool, error) {
	n, err := windows.UTF16FromString(name)
	if err != nil {
		return "", false, err
	}
	p, err := windows.UTF16FromString(prefix)
	if err != nil {
		return "", false, err
	}
	count := len(p) - 1
	if count <= 0 {
		return "", false, ErrUnsupported
	}
	if len(n)-1 < count {
		return "", false, nil
	}
	if err = encryptedWinCompareOrdinal.Find(); err != nil {
		return "", false, err
	}
	result, _, nativeErr := encryptedWinCompareOrdinal.Call(uintptr(unsafe.Pointer(&n[0])), uintptr(count), uintptr(unsafe.Pointer(&p[0])), uintptr(count), 1)
	runtime.KeepAlive(n)
	runtime.KeepAlive(p)
	if result == 0 {
		return "", false, errors.Join(ErrUnsupported, nativeErr)
	}
	if result != 2 {
		return "", false, nil
	} // CSTR_EQUAL
	return windows.UTF16ToString(n[count : len(n)-1]), true, nil
}
func platformFamilyEntry(name, base string) (bool, error) {
	return foldedFamilyEntry(name, base, encryptedWinFoldPrefix)
}
