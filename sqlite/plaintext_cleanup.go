package sqlite

import "github.com/ms-xiaosnow/gosqlite/internal/pager"

// ErrPlaintextCleanupPending marks an immutable historical cleanup token for an
// covered public open, rollback Reader, standalone Tx or exclusive-WAL Session. Tokens match
// ErrPoisoned and carry CodeIOErr.
// A close diagnostic without this marker can describe fully consumed ownership.
var ErrPlaintextCleanupPending = pager.ErrPlaintextCleanupPending

// RetryPlaintextCleanup deliberately attempts all distinct ordinary rollback
// Reader/Tx/exclusive-Session groups in wrapped/joined errors. Discovery precedes effects and accepts
// at most 4096 error nodes. Excessive/cyclic trees return ErrMemoryLimit without
// cleanup or wrapping the unsafe tree; retain tokens and retry smaller trees.
//
// A live group retries only failed nested probes or close-requested child files,
// never an active main file or an unrequested live journal/WAL. After logical close,
// child obligations retire before the protecting main file.
// Retries do not open paths, replay transactions, or retry numeric native handles.
// Terminal ownership can remain pending indefinitely. Completed groups and errors
// without tokens are no-ops, not proof every subsystem is clean. This increment
// does not cover WALShared or encryption; those retain their separate APIs.
func RetryPlaintextCleanup(err error) error {
	return wrap("plaintext cleanup", pager.RetryPlaintextCleanup(err))
}

// PendingPlaintextCleanups returns a new snapshot of immutable pending rollback
// Reader/Tx/exclusive-Session tokens, including failed opens and errors discarded
// by database/sql.
// Enumeration performs no cleanup; ordinary live/opening groups are excluded.
// At most 1024 opening/live/pending groups are admitted before native acquisition.
// Public rollback/exclusive opens reserve before canonicalization and transfer
// that same root into their Reader probe or Session; early errors are adopted.
// Each pre-reserves main, current WAL and one temporary child slot (journal, WAL
// candidate or superseded WAL). Unused slots contain no owners.
// There is no eviction, finalizer, automatic sweep, or wait on another group at
// admission. Changing the returned slice does not change registry ownership.
func PendingPlaintextCleanups() []error {
	tokens := pager.PendingPlaintextCleanups()
	for i, e := range tokens {
		tokens[i] = wrap("plaintext cleanup", e)
	}
	return tokens
}

// Only conclusive initial absence can authorize creation. Missing-path errors
// after acquisition, additional diagnostics and pending ownership fail closed.
func plainProbeMissing(err error) bool {
	return pager.ReaderProbeAbsent(err)
}
