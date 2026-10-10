//go:build !darwin && !(windows && amd64)

package vfs

// Linux ownership remains descriptor-local; unsupported backends never open a
// file. These hooks add no lifecycle or syscall change to the Linux backend.
type nativeFile struct{}

func nativeOpened(*File) error       { return nil }
func nativeCheck(*File) error        { return nil }
func nativeValidatePath(*File) error { return nil }
