//go:build !(windows && amd64)

package encstore

import (
	"github.com/ms-xiaosnow/gosqlite/encryption"
	"os"
	"path/filepath"
)

// Keep Linux's existing descriptor-local OFD implementation and I/O unchanged.
// Other backends remain fail-closed until their full lifetime layer exists.
type managedFile = os.File

func canonicalPath(path string) (string, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(p))
	if err != nil {
		return "", err
	}
	p = filepath.Join(dir, filepath.Base(p))
	if info, e := os.Lstat(p); e == nil && !info.Mode().IsRegular() {
		return "", ErrUnsupported
	} else if e != nil && !os.IsNotExist(e) {
		return "", e
	}
	return p, nil
}
func platformPathInfo(path string) (os.FileInfo, error)   { return os.Lstat(path) }
func platformFamilyEntry(name, base string) (bool, error) { return familyEntry(name, base) }
func platformOperation(encryption.Operation, Mode) error  { return supported() }
func platformPreflight(string) error                      { return supported() }
func platformFileCheck(_ *managedFile, path string, info os.FileInfo) error {
	named, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !named.Mode().IsRegular() || !os.SameFile(info, named) {
		return ErrChanged
	}
	return nil
}
func platformDirectoryCheck(_ *managedFile, path string, info os.FileInfo) error {
	named, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !named.IsDir() || !os.SameFile(info, named) {
		return ErrChanged
	}
	return nil
}
func openRootFileAt(dir *managedFile, name string, flags int) (*managedFile, error) {
	return openFileAt(dir, name, flags)
}

func replaceFile(old, new string) error { return os.Rename(old, new) }
