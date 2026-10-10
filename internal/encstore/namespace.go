package encstore

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
)

// namespace pins the selected parent directory. It is not an external database
// identity/freshness oracle; initial path selection and hostile OS actors remain
// outside the cooperative lease and descriptor checks.
type namespace struct {
	dir  *os.File
	info os.FileInfo
	path string
}

func newNamespace(path string) (*namespace, error) {
	d := filepath.Dir(path)
	f, e := openDirectory(d)
	if e != nil {
		return nil, e
	}
	i, e := f.Stat()
	if e != nil {
		f.Close()
		return nil, e
	}
	return &namespace{f, i, d}, nil
}
func (n *namespace) check() error {
	if n == nil {
		return nil
	}
	i, e := os.Lstat(n.path)
	if e != nil {
		return e
	}
	if !i.IsDir() || !os.SameFile(n.info, i) {
		return ErrChanged
	}
	return nil
}
func (n *namespace) name(path string) (string, error) {
	if filepath.Dir(path) != n.path {
		return "", ErrUnsupported
	}
	return filepath.Base(path), nil
}
func (n *namespace) checked(path string, flags int) (*os.File, os.FileInfo, error) {
	if n == nil {
		return checkedFile(path, flags)
	}
	name, e := n.name(path)
	if e != nil {
		return nil, nil, e
	}
	f, e := openFileAt(n.dir, name, flags)
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
		f.Close()
		return nil, nil, e
	}
	return f, i, nil
}
func (n *namespace) stable(f *os.File, path string, i os.FileInfo) error {
	if n == nil {
		return stableFile(f, path, i)
	}
	name, e := n.name(path)
	if e != nil {
		return e
	}
	return stableFileAt(n.dir, name, f, i)
}
func (n *namespace) sync(path string) error {
	if n == nil {
		return syncDirectory(path)
	}
	if e := n.check(); e != nil {
		return e
	}
	return n.dir.Sync()
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
	return errors.Join(e, f.Close())
}
func (n *namespace) retain(path string, b []byte) error {
	e := n.write(path, b)
	if !os.IsExist(e) {
		return e
	}
	f, i, e := n.checked(path, os.O_RDONLY)
	if e != nil {
		return e
	}
	defer f.Close()
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
		return os.Rename(old, new)
	}
	a, e := n.name(old)
	if e != nil {
		return e
	}
	b, e := n.name(new)
	if e != nil {
		return e
	}
	return renameFileAt(n.dir, a, b, absent)
}
func chooseNamespace(ns []*namespace) *namespace {
	if len(ns) > 0 {
		return ns[0]
	}
	return nil
}
