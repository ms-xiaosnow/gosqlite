package vfs

import (
	"os"
	"path/filepath"
)

// NamespaceIdentity is a pathname-family observation from a managed lease.
// Parent is the identity captured when this File was opened, not a fresh lookup
// of a possibly unrelated parent. It conveys no descriptor or ownership transfer.
// It is not a capability for descriptor-relative creation in that directory.
type NamespaceIdentity struct {
	Base   string
	Parent os.FileInfo
}

// Namespace validates continuity with the original file and parent identity
// before returning their naming metadata. The caller must adopt any returned
// metadata/retirement error into the same owner that protects this File. No
// additional native descriptor is opened here beyond validatePathLocked's
// existing platform-managed probes. The original lease remains held throughout.
func (f *File) Namespace() (NamespaceIdentity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.validatePathLocked(); err != nil {
		return NamespaceIdentity{}, err
	}
	if f.parent == nil {
		return NamespaceIdentity{}, ErrIdentity
	}
	return NamespaceIdentity{Base: filepath.Base(f.path), Parent: f.parent}, nil
}
