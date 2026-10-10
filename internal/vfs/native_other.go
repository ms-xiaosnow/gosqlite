//go:build !linux

package vfs

import "os"

// Fail closed until real platform file ownership, locks, and mapping backends
// are implemented. Cross-compiling this file is not native database support.
func nativeOpen(string, OpenOptions) (*os.File, error) { return nil, ErrUnsupported }
func nativeValidateLinks(*File) error                  { return ErrUnsupported }
func nativeSync(*File) error                           { return ErrUnsupported }
func nativeClose(*File) error                          { return ErrUnsupported }
func nativeReplace(string, string) error               { return ErrUnsupported }
func nativeShared(*File) error                         { return ErrUnsupported }
func nativeReserved(*File) error                       { return ErrUnsupported }
func nativeExclusive(*File) error                      { return ErrUnsupported }
func nativeDowngrade(*File) error                      { return ErrUnsupported }
func nativeReservedHeld(*File) (bool, error)           { return false, ErrUnsupported }
func nativeAttachDMS(*File) error                      { return ErrUnsupported }
func nativeSHMLock(*File, int, int, bool, bool) error  { return ErrUnsupported }

type nativeMapping struct{}

func nativeMapGranularity() int64 { return 0 }
func nativeMap(*File, int64, int) (nativeMapping, []byte, error) {
	return nativeMapping{}, nil, ErrUnsupported
}
func nativeUnmap(*nativeMapping) error { return ErrUnsupported }
