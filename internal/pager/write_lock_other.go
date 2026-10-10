//go:build !linux

package pager

import (
	"fmt"
	"os"
)

func lockReserved(_ *os.File) error {
	return fmt.Errorf("%w: native write locks currently require Linux", ErrUnsupported)
}
func lockExclusive(_ *os.File) error {
	return fmt.Errorf("%w: native write locks currently require Linux", ErrUnsupported)
}
func validateWritableFile(_ os.FileInfo) error {
	return fmt.Errorf("%w: native writes currently require Linux", ErrUnsupported)
}
func unlockExclusive(_ *os.File) error {
	return fmt.Errorf("%w: native write locks currently require Linux", ErrUnsupported)
}

func openWritableDatabase(path string, flags int) (*os.File, error) {
	return nil, fmt.Errorf("%w: native writes currently require Linux", ErrUnsupported)
}
