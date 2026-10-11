package pager

import (
	"errors"
	"github.com/ms-xiaosnow/gosqlite/internal/wal"
)

// This is a real native capability gap, not an unrun acceptance test. Current
// vfs.ReplaceWAL uses path-based publication and does not retain a directory
// owner through candidate creation, replacement and publication sync. Until a
// publisher enforces the contract in docs/decisions/0017-wal-tail-prefix.md,
// every exclusive tail remains rejected before any mutation or SHM removal.
var errWALTailPublicationUnavailable = errors.New("exclusive WAL tail requires retained-directory committed-prefix publication capability")

// The predicate only classifies scanner evidence. It does not grant file,
// namespace or ownership authority. Clean scans and all old-salt, incomplete,
// invalid, header, I/O, quota and cancellation errors are excluded here.
func exclusiveWALPrefixAllowed(prefix wal.CommittedPrefix, scanError error) bool {
	return scanError != nil && prefix.Allows(scanError, wal.TailUncommitted)
}
