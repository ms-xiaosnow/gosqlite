//go:build !linux || !manual_fault_rollback

package pager

import (
	"os"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// The default complement has no armer, registry, environment or observer.
// Each bridge preserves the concrete original managed owner and original call.
func rollbackCreate(t *Tx, point rollbackIOPoint, path string, flags int) (*vfs.File, error) {
	return openJournal(path, flags)
}
func rollbackRead(t *Tx, point rollbackIOPoint, f *vfs.File, b []byte, off int64) (int, error) {
	return f.ReadAt(b, off)
}
func rollbackWrite(t *Tx, point rollbackIOPoint, f *vfs.File, b []byte, off int64) error {
	return writeFullAt(f, b, off)
}
func rollbackFileSync(t *Tx, point rollbackIOPoint, f *vfs.File) error {
	return f.Sync()
}
func rollbackDirectorySync(t *Tx, point rollbackIOPoint, path string) error {
	return syncDirectory(path)
}
func rollbackUnlink(t *Tx, point rollbackIOPoint, path string) error {
	return os.Remove(path)
}
