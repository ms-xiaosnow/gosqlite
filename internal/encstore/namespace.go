package encstore

import (
	"bytes"
	"os"
	"path/filepath"
)

// namespace pins the selected parent directory. It is not an external database
// identity/freshness oracle; initial path selection and hostile OS actors remain
// outside the cooperative lease and descriptor checks.
type namespace struct {
	group *cleanupGroup
	dir   *managedFile
	info  os.FileInfo
	path  string
	root  string
}

func newNamespace(path string) (*namespace, error) {
	d := filepath.Dir(path)
	f, e := openDirectory(d)
	if e != nil {
		return nil, e
	}
	i, e := f.Stat()
	if e != nil {
		return nil, f.owner.closeAll(e)
	}
	return &namespace{group: f.owner, dir: f, info: i, path: d, root: filepath.Base(path)}, nil
}
func (n *namespace) check() error {
	if n == nil {
		return nil
	}
	if e := n.group.health(); e != nil {
		return e
	}
	return n.group.remember(platformDirectoryCheck(n.dir, n.path, n.info))
}
func (n *namespace) name(path string) (string, error) {
	if filepath.Dir(path) != n.path {
		return "", ErrUnsupported
	}
	return filepath.Base(path), nil
}
func (n *namespace) checked(path string, flags int) (_ *managedFile, _ os.FileInfo, resultErr error) {
	if n != nil {
		defer func() { resultErr = n.group.remember(resultErr) }()
	}
	if n == nil {
		return checkedFile(path, flags)
	}
	name, e := n.name(path)
	if e != nil {
		return nil, nil, e
	}
	var f *managedFile
	if name == n.root {
		f, e = openRootFileAt(n.dir, name, flags)
	} else {
		f, e = openFileAt(n.dir, name, flags)
	}
	if e != nil {
		return nil, nil, e
	}
	i, e := f.Stat()
	if e == nil {
		e = validateFile(i)
	}
	if e == nil {
		e = validateLinks(f)
	}
	if e == nil {
		e = stableFileAt(n.dir, name, f, i)
	}
	if e != nil {
		return nil, nil, closeResult(f, e)
	}
	return f, i, nil
}
func (n *namespace) stable(f *managedFile, path string, i os.FileInfo) error {
	if n == nil {
		return stableFile(f, path, i)
	}
	name, e := n.name(path)
	if e != nil {
		return e
	}
	return n.group.remember(stableFileAt(n.dir, name, f, i))
}
func (n *namespace) sync(path string) error {
	if n == nil {
		return syncDirectory(path)
	}
	if e := n.check(); e != nil {
		return e
	}
	return n.group.remember(n.dir.Sync())
}
func (n *namespace) write(path string, b []byte) error {
	f, _, e := n.checked(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if e != nil {
		return e
	}
	e = writeBytes(f, b)
	if e == nil {
		e = f.Sync()
	}
	return closeResult(f, e)
}
func (n *namespace) retain(path string, b []byte) (resultErr error) {
	e := n.write(path, b)
	if !os.IsExist(e) {
		return e
	}
	f, i, e := n.checked(path, os.O_RDONLY)
	if e != nil {
		return e
	}
	defer func() { resultErr = closeResult(f, resultErr) }()
	if i.Size() != int64(len(b)) {
		return ErrChanged
	}
	a, e := readExact(f, len(b), 0)
	if e != nil {
		return e
	}
	if !bytes.Equal(a, b) {
		return ErrChanged
	}
	return n.stable(f, path, i)
}
func (n *namespace) rename(old, new string, absent bool) error {
	if n == nil {
		if absent {
			return renameAbsent(old, new)
		}
		return replaceFile(old, new)
	}
	a, e := n.name(old)
	if e != nil {
		return e
	}
	b, e := n.name(new)
	if e != nil {
		return e
	}
	return n.group.remember(renameFileAt(n.dir, a, b, absent))
}
func chooseNamespace(ns []*namespace) *namespace {
	if len(ns) > 0 {
		return ns[0]
	}
	return nil
}
