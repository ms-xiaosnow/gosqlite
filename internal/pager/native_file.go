package pager

import (
	"errors"
	"fmt"
	"os"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

func nativeFileError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, vfs.ErrPoisoned):
		return fmt.Errorf("%w: %w", ErrPoisoned, err)
	case errors.Is(err, vfs.ErrBusy):
		return fmt.Errorf("%w: %w", ErrBusy, err)
	case errors.Is(err, vfs.ErrUnsupported):
		return fmt.Errorf("%w: %w", ErrUnsupported, err)
	case errors.Is(err, vfs.ErrIdentity):
		return fmt.Errorf("%w: %w", ErrChanged, err)
	case errors.Is(err, vfs.ErrClosed):
		return fmt.Errorf("%w: %w", ErrClosed, err)
	case errors.Is(err, vfs.ErrLock):
		return fmt.Errorf("%w: %w", ErrMisuse, err)
	default:
		return err
	}
}
func openNativeFile(path string, opts vfs.OpenOptions) (*vfs.File, error) {
	f, err := vfs.Open(path, opts)
	return f, nativeFileError(err)
}
func openWritableDatabase(path string, flags int) (*vfs.File, error) {
	return openNativeFile(path, vfs.OpenOptions{Role: vfs.Database, Flags: flags})
}
func openWALFile(path string, flags int, replaceable bool) (*vfs.File, error) {
	return openNativeFile(path, vfs.OpenOptions{Role: vfs.WAL, Flags: flags, Replaceable: replaceable})
}
func openRecoveryFile(path string, flags int) (*vfs.File, error) {
	return openWritableDatabase(path, flags)
}
func lockShared(f *vfs.File) error      { return nativeFileError(f.LockDB(vfs.Shared)) }
func lockReserved(f *vfs.File) error    { return nativeFileError(f.LockDB(vfs.Reserved)) }
func lockExclusive(f *vfs.File) error   { return nativeFileError(f.LockDB(vfs.Exclusive)) }
func unlockExclusive(f *vfs.File) error { return nativeFileError(f.DowngradeDBToShared()) }
func recoveryReservedHeld(f *vfs.File) (bool, error) {
	held, err := f.ReservedHeld()
	return held, nativeFileError(err)
}
func validateWritableFile(f *vfs.File) error { return nativeFileError(f.ValidateSingleLink()) }
func validateNativePath(f *vfs.File) error   { return nativeFileError(f.ValidatePath()) }
func openJournal(path string, flags int) (*vfs.File, error) {
	return openNativeFile(path, vfs.OpenOptions{Role: vfs.Journal, Flags: flags})
}
func openReadOnlyDatabase(path string) (*vfs.File, error) {
	return openNativeFile(path, vfs.OpenOptions{Role: vfs.Database, Flags: os.O_RDONLY})
}
