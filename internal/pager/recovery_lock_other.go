//go:build !linux

package pager

import (
	"fmt"
	"os"
)

func recoveryReservedHeld(_ *os.File) (bool, error) {
	return false, fmt.Errorf("%w: rollback recovery locks currently require Linux", ErrUnsupported)
}

func openRecoveryFile(_ string, _ int) (*os.File, error) {
	return nil, fmt.Errorf("%w: rollback recovery currently requires Linux", ErrUnsupported)
}
