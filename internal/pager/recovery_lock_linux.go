//go:build linux

package pager

import (
	"fmt"
	"os"
	"syscall"
)

// recoveryReservedHeld queries a conflicting RESERVED byte lock without ever
// taking RESERVED ourselves. OFD_GETLK observes both POSIX SQLite locks and
// other OFD descriptions, including descriptions in this same process.
func recoveryReservedHeld(f *os.File) (bool, error) {
	const ofdGetLK = 36 // Linux F_OFD_GETLK, introduced in Linux 3.15.
	lock := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: 0, Start: 0x40000001, Len: 1}
	if err := syscall.FcntlFlock(f.Fd(), ofdGetLK, &lock); err != nil {
		return false, fmt.Errorf("%w: native reserved-lock probe: %v", ErrUnsupported, err)
	}
	return lock.Type != syscall.F_UNLCK, nil
}

func openRecoveryFile(path string, flags int) (*os.File, error) {
	// NONBLOCK avoids hanging on a replaced FIFO before the regular-file
	// check. NOFOLLOW prevents a last-component sidecar symlink redirect.
	return os.OpenFile(path, flags|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
}
