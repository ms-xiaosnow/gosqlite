//go:build !(windows && amd64)

package sqlite

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Keep the existing non-Windows canonicalization and encrypted final-link
// policy unchanged. Windows uses owned native metadata instead of these calls.
func canonicalOpenPath(path string, encrypted bool) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if encrypted {
		if info, e := os.Lstat(absolute); e == nil && !info.Mode().IsRegular() {
			return "", ErrUnsupported
		} else if e != nil && !os.IsNotExist(e) {
			return "", e
		}
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent, parentErr := filepath.EvalSymlinks(filepath.Dir(absolute))
		if parentErr != nil {
			return "", parentErr
		}
		canonical = filepath.Join(parent, filepath.Base(absolute))
		// An absent final component may be created; a dangling final symlink
		// is refused rather than treated as an authorized new database.
		if info, statErr := os.Lstat(canonical); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%w: dangling database symlink", ErrUnsupported)
		}
	}
	return canonical, nil
}
