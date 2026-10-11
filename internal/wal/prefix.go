package wal

import (
	"context"
	"fmt"
	"io"
	"maps"
)

// tailError is created only at a scanner-established frame boundary. It is
// deliberately private: ErrTail matching alone is never admission evidence.
// In particular an I/O error wrapping/joining ErrTail is not a tailError.
type tailError struct {
	status TailStatus
	offset int64
	cause  error
}

func (e *tailError) Error() string {
	if e.cause == nil {
		return fmt.Sprintf("%v: %s at %d", ErrTail, e.status, e.offset)
	}
	return fmt.Sprintf("%v: %s at %d: %v", ErrTail, e.status, e.offset, e.cause)
}
func (e *tailError) Unwrap() []error {
	if e.cause == nil {
		return []error{ErrTail}
	}
	return []error{ErrTail, e.cause}
}

// CommittedPrefix carries scanner-owned provenance, not permission to mutate
// or publish a WAL. The zero value is invalid. Its private snapshot cannot be
// changed by a caller. The paired database and WAL must remain locked and
// immutable throughout scanning and use. Identity, mode, family, durability,
// and publication policy belong to the pager.
type CommittedPrefix struct{ proof *prefixProof }
type prefixProof struct {
	snapshot  Snapshot
	scanError error
}

// ScanPrefix is Scan with a sealed committed-prefix result. Only a successful
// scan or the scanner's exact typed tail result can produce provenance. Header,
// quota, cancellation and ReaderAt failures return an invalid zero prefix even
// when their error graph contains ErrTail or an earlier scanner tail error.
func ScanPrefix(ctx context.Context, r io.ReaderAt, size int64, limits Limits) (CommittedPrefix, error) {
	s, err := Scan(ctx, r, size, limits)
	if err != nil {
		if _, ok := err.(*tailError); !ok {
			return CommittedPrefix{}, err
		}
	}
	return CommittedPrefix{&prefixProof{snapshot: s, scanError: err}}, err
}

// Allows requires the exact returned error and one explicitly supported tail.
// It never searches an error graph: wrapping or joining unrelated failures,
// including a single-element Join, cannot manufacture admission. A clean scan
// is accepted with nil only. The caller must not discard a later operation's
// error to reuse the original pair.
func (p CommittedPrefix) Allows(scanError error, tails ...TailStatus) bool {
	if p.proof == nil || scanError != p.proof.scanError {
		return false
	}
	s := p.proof.snapshot
	if scanError == nil {
		return s.Complete && s.Tail == TailClean
	}
	for _, status := range tails {
		if status != TailClean && status == s.Tail {
			return true
		}
	}
	return false
}

// Snapshot returns a detached copy for a policy-admitted reader. Complete and
// Tail keep their original diagnostic meanings; this does not make an invalid
// suffix appendable. One additional bounded committed-page map is allocated.
func (p CommittedPrefix) Snapshot() Snapshot {
	if p.proof == nil {
		return Snapshot{}
	}
	s := p.proof.snapshot
	s.Pages = maps.Clone(s.Pages)
	return s
}
