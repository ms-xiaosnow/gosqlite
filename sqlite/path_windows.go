//go:build windows && amd64

package sqlite

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// Public path selection must not call os.Lstat/EvalSymlinks before the managed
// storage layer. Encrypted paths use that backend's exact canonical/provider
// semantics; plain paths retain database-alias and absent-final creation policy.
func canonicalOpenPath(path string, encrypted bool) (string, error) {
	if encrypted {
		p, err := encstore.CanonicalWindowsPath(path)
		return p, nativeOpenPathError(err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	canonical, err := vfs.ResolvePath(absolute)
	if err == nil {
		return canonical, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", nativeOpenPathError(err)
	}
	parent, err := vfs.ResolvePath(filepath.Dir(absolute))
	if err != nil {
		return "", nativeOpenPathError(err)
	}
	canonical = filepath.Join(parent, filepath.Base(absolute))
	// Lstat rejects reparse points through an owned nofollow probe. Report
	// uncertainty as poison; never interpret it as permission to create.
	if info, err := vfs.Lstat(canonical); err == nil {
		if !info.Mode().IsRegular() {
			return "", ErrUnsupported
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", nativeOpenPathError(err)
	}
	return canonical, nil
}

func nativeOpenPathError(err error) error {
	switch {
	case errors.Is(err, vfs.ErrPoisoned), errors.Is(err, encstore.ErrPoisoned):
		return errors.Join(ErrPoisoned, err)
	case errors.Is(err, vfs.ErrUnsupported), errors.Is(err, encstore.ErrUnsupported):
		return errors.Join(ErrUnsupported, err)
	case errors.Is(err, vfs.ErrIdentity), errors.Is(err, encstore.ErrChanged):
		return errors.Join(ErrChanged, err)
	default:
		return err
	}
}
