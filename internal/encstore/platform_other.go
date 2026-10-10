//go:build !linux && !darwin && !(windows && amd64)

package encstore

import "os"

func supported() error                  { return ErrUnsupported }
func validateFile(os.FileInfo) error    { return ErrUnsupported }
func validateLinks(*managedFile) error  { return ErrUnsupported }
func lockFile(*managedFile, bool) error { return ErrUnsupported }
func renameAbsent(string, string) error { return ErrUnsupported }

func nativeOpenDirectory(string) (*managedFile, error) { return nil, ErrUnsupported }

func nativeOpenFileAt(*managedFile, string, int, bool) (*managedFile, error) {
	return nil, ErrUnsupported
}
func stableFileAt(*managedFile, string, *managedFile, os.FileInfo) error { return ErrUnsupported }
func renameFileAt(*managedFile, string, string, bool) error              { return ErrUnsupported }

func rangeLock(*managedFile, int64, bool, bool) error { return ErrUnsupported }
