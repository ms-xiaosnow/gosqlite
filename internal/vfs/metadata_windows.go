//go:build windows && amd64

package vfs

import (
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
)

// One reserved slot per retained parent or in-flight metadata probe. No live
// owner is evicted at the bound, and uncertain closes block all new Windows
// plain-storage work. Ordinary explicit Close can still retire known resources.
var winMetadataOwners = metadataOwners{limit: 16384}

func winOpenMetadata(path string, follow, replaceable bool) (*metadataOwner, error) {
	if err := winPath(path); err != nil {
		return nil, err
	}
	return winMetadataOwners.open(func() (uintptr, error) {
		// BACKUP_SEMANTICS allows either a file or a directory; inspect the
		// acquired handle instead of statting the pathname to choose a type.
		h, err := winCreate(path, windows.FILE_READ_ATTRIBUTES, windows.OPEN_EXISTING, true, follow, replaceable)
		return uintptr(h), err
	})
}
func winCloseMetadata(o *metadataOwner) error {
	return winMetadataOwners.close(o, winCloseMetadataHandle)
}

func winCloseMetadataHandle(h uintptr) error { return windows.CloseHandle(windows.Handle(h)) }

type winFileInfo struct {
	name string
	id   winIdentity
	data windows.ByHandleFileInformation
}

func (i *winFileInfo) Name() string { return i.name }
func (i *winFileInfo) Size() int64  { return int64(i.data.FileSizeHigh)<<32 | int64(i.data.FileSizeLow) }
func (i *winFileInfo) Mode() os.FileMode {
	mode := os.FileMode(0666)
	if i.data.FileAttributes&windows.FILE_ATTRIBUTE_READONLY != 0 {
		mode = 0444
	}
	if i.data.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		mode |= os.ModeDir | 0111
	}
	return mode
}
func (i *winFileInfo) ModTime() time.Time { return time.Unix(0, i.data.LastWriteTime.Nanoseconds()) }
func (i *winFileInfo) IsDir() bool        { return i.Mode().IsDir() }
func (i *winFileInfo) Sys() any           { return nil } // No HANDLE or lazy path escapes.

// winSnapshot observes only an already-owned handle. FILE_ID_INFO supplies the
// complete 128-bit ID; ByHandleFileInformation supplies size/time/attributes,
// never its narrower identity. No os.File wrapper/finalizer is created here.
func winSnapshot(h windows.Handle, path string) (os.FileInfo, error) {
	var data windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &data); err != nil {
		return nil, err
	}
	id, err := winInspect(h, data.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0)
	if err != nil {
		return nil, err
	}
	return &winFileInfo{name: filepath.Base(path), id: id, data: data}, nil
}

func winStatPath(path string, follow bool) (os.FileInfo, error) {
	info, _, err := winStatPathForCreate(path, follow)
	return info, err
}

func winStatPathForCreate(path string, follow bool) (os.FileInfo, bool, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return nil, false, err
	}
	// Temporary metadata probes permit delete sharing so they are compatible
	// with explicitly replaceable WAL leases. The actual file and retained
	// parent keep their stricter sharing policy throughout this observation.
	o, err := winOpenMetadata(p, follow, true)
	return observeMetadataForCreate(&winMetadataOwners, o, err, func(h uintptr) (os.FileInfo, error) {
		return winSnapshot(windows.Handle(h), p)
	}, winCloseMetadataHandle)
}

// Lstat rejects final reparse points under the Windows plain-storage policy.
// Unlike os.Lstat/SameFile, it does not leave a hidden lazy pathname open.
func Lstat(path string) (os.FileInfo, error)          { return winStatPath(path, false) }
func statParentPath(path string) (os.FileInfo, error) { return winStatPath(path, true) }
func SameFile(a, b os.FileInfo) bool {
	x, ok := a.(*winFileInfo)
	y, ok2 := b.(*winFileInfo)
	return ok && ok2 && x != nil && y != nil && x.id == y.id
}
func nativeStat(f *File) (os.FileInfo, error) {
	return winSnapshot(windows.Handle(f.raw.Fd()), f.path)
}
func nativeParentInfo(f *File) (os.FileInfo, error) {
	if f.native.parent == nil {
		return nil, ErrPoisoned
	}
	return winSnapshot(windows.Handle(f.native.parent.handle), filepath.Dir(f.path))
}

// ResolvePath uses an owned metadata handle instead of filepath.EvalSymlinks,
// whose Windows implementation can open hidden metadata handles. It follows
// aliases only while choosing the canonical file family; Open still inspects
// its final component without following and retains its checked parent.
func ResolvePath(path string) (string, error) {
	resolved, _, err := ResolvePathForCreate(path)
	return resolved, err
}

// ResolvePathForCreate reports absence only when the first metadata open
// acquired no handle. All post-acquisition inspection errors fail closed.
func ResolvePathForCreate(path string) (string, bool, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return "", false, err
	}
	o, err := winOpenMetadata(p, true, true)
	return observeMetadataForCreate(&winMetadataOwners, o, err, func(raw uintptr) (string, error) {
		h := windows.Handle(raw)
		if _, err := winSnapshot(h, p); err != nil {
			return "", err
		}
		resolved, err := winFinalPath(h)
		if err != nil {
			return "", err
		}
		if err := winLocalNTFS(h, resolved); err != nil {
			return "", err
		}
		return resolved, nil
	}, winCloseMetadataHandle)
}

// LstatForCreate applies the same no-handle provenance to the nofollow probe.
func LstatForCreate(path string) (os.FileInfo, bool, error) {
	return winStatPathForCreate(path, false)
}
