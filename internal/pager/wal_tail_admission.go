package pager

import (
	"errors"
	"github.com/ms-xiaosnow/gosqlite/internal/wal"
)

// Nil/default and unsupported profiles refuse tails before mutation. The
// explicit Linux capability is invoked only after prospective paired-byte
// admission; the intact-WAL normalizer is not an alternative tail publisher.
// Source presence is not native qualification: see the current-baseline ADR.
var errWALTailPublicationUnavailable = errors.New("exclusive WAL tail requires retained-directory committed-prefix publication capability")

// The predicate only classifies scanner evidence. It does not grant file,
// namespace or ownership authority. Clean scans and all old-salt, incomplete,
// invalid, header, I/O, quota and cancellation errors are excluded here.
func exclusiveWALPrefixAllowed(prefix wal.CommittedPrefix, scanError error) bool {
	return scanError != nil && prefix.Allows(scanError, wal.TailUncommitted)
}
