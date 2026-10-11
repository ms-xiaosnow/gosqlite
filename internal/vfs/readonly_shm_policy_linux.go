//go:build linux

package vfs

import "syscall"

// readonlySHMDMSDecision is the descriptor-free admission decision after a
// successful native DMS probe. Only writers may initialize an unheld DMS;
// readers require a live shared holder. Exclusive peers remain contention.
func readonlySHMDMSDecision(writable bool, peer int16) (initialize bool, err error) {
	switch peer {
	case syscall.F_UNLCK:
		if !writable {
			return false, ErrUnsupported
		}
		return true, nil
	case syscall.F_RDLCK:
		return false, nil
	case syscall.F_WRLCK:
		return false, ErrBusy
	default:
		return false, ErrUnsupported
	}
}

func readonlySHMProtection(writable bool) int {
	protection := syscall.PROT_READ
	if writable {
		protection |= syscall.PROT_WRITE
	}
	return protection
}
