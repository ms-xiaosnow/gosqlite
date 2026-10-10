//go:build !linux

package pager

import (
	"fmt"
	"os"
)

func lockShared(_ *os.File) error {
	return fmt.Errorf("%w: read-only native shared locks currently require Linux", ErrUnsupported)
}
