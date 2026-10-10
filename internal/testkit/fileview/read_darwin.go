//go:build darwin

package fileview

import (
	"errors"
	"io"
	"os"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// ReadFile joins the same descriptor-close registry as native plain/encrypted
// storage. A raw os.ReadFile of a live .gse-lock would remove sibling fcntl
// locks on close, making an otherwise ordinary inspection test unsafe on Darwin.
// Fixture members inspected by the encrypted tests are regular file snapshots;
// this probe takes no SQLite database lock and never modifies bytes.
func ReadFile(path string) ([]byte, error) {
	f, err := vfs.Open(path, vfs.OpenOptions{Role: vfs.Database, Flags: os.O_RDONLY})
	if err != nil {
		return nil, err
	}
	b, readErr := io.ReadAll(f)
	return b, errors.Join(readErr, f.Close())
}
