package pager

import (
	"errors"
	"fmt"
	"os"

	"github.com/ms-xiaosnow/gosqlite/internal/walindex"
)

var ErrReadOnly = errors.New("pager session is read-only")

// These descriptor-free decisions are used by the native session itself.
func sharedReadOnlyOpenPolicy(goos string, readOnly bool, opts Options) error {
	if !readOnly {
		return nil
	}
	if goos != "linux" || opts.Create || opts.WALPowersafeOverwrite {
		return fmt.Errorf("%w: readonly shared WAL requires Linux, no create, and no write-storage assertion", ErrUnsupported)
	}
	return nil
}
func sharedMainOpenFlags(readOnly, create bool) int {
	if readOnly {
		return os.O_RDONLY
	}
	flags := os.O_RDWR
	if create {
		flags |= os.O_CREATE
	}
	return flags
}
func sharedWALOpenFlags(readOnly bool) int {
	return sharedMainOpenFlags(readOnly, true)
}
func sharedCloseMayCheckpoint(readOnly, keep bool) bool { return !readOnly && !keep }
func sharedReadMarkPreferred(maxFrame uint32, chosen int, mark, candidate uint32) bool {
	return candidate != walindex.UnusedReadMark && candidate <= maxFrame && (chosen == 0 || candidate >= mark)
}
func sharedReadSnapshotStable(captured, current walindex.Header, mark, currentMark, backfill uint32) bool {
	return current == captured && currentMark == mark && backfill <= captured.MaxFrame
}

func sharedReadOnlyMarkError(readOnly bool, chosen int) error {
	if readOnly && chosen == 0 {
		return fmt.Errorf("%w: readonly WAL requires an existing reusable readmark", ErrUnsupported)
	}
	return nil
}
