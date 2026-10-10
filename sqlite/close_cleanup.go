package sqlite

import "github.com/ms-xiaosnow/gosqlite/internal/pager"

// ErrCloseCleanupPending marks a native shared-WAL cleanup result that retained
// managed ownership when returned. Such errors also match ErrPoisoned and carry
// CodeIOErr. It also covers public pre-session canonicalization failures.
// The historical token is immutable, even after successful cleanup.
var ErrCloseCleanupPending = pager.ErrCloseCleanupPending

// RetryCloseCleanup synchronously attempts every distinct close group reachable
// through ordinary wrapped/joined tokens. Discovery and deduplication precede
// cleanup; more than 4096 error nodes (including cyclic wrappers) returns
// ErrMemoryLimit/CodeNoMem without any owner operation. That refusal cannot wrap
// the unsafe input; retain original tokens and retry smaller trees.
//
// It never checkpoints, opens files, replays transactions, retries raw native
// handles, or sweeps other groups. Same-group attempts serialize. Completed
// groups are no-ops. Pending results retain their route and original causes;
// retry history is not accumulated. A retired owner can still report a Close
// error: an error without ErrCloseCleanupPending need not imply live ownership.
// Nil or an error without a cleanup token is a no-op, not a universal cleanup
// guarantee. This API covers native WALShared sessions and their reserved public
// metadata prelude. Failed pager construction still uses RetryOpenCleanup.
func RetryCloseCleanup(err error) error { return wrap("close cleanup", pager.RetryCloseCleanup(err)) }

// PendingCloseCleanups returns a newly allocated snapshot of immutable native
// shared-WAL pending-close/public-metadata tokens, including errors discarded
// by database/sql.
// It performs no cleanup. Live sessions and in-flight opens are excluded. A
// token can already have completed when used; pass it to RetryCloseCleanup for
// current status. Changing this slice does not change ownership or the registry.
//
// At most 1024 native shared-WAL sessions may be opening, live, or pending close
// together. Admission is reserved before native acquisition, without eviction,
// finalizers, or background cleanup. Fully retired groups free capacity; terminal
// uncertainty may retain capacity for process lifetime. The separate 64-group
// failed-pager-open limit and RetryOpenCleanup remain independent; public
// canonicalization failures consume the pre-reserved close group instead.
func PendingCloseCleanups() []error {
	tokens := pager.PendingCloseCleanups()
	for i, token := range tokens {
		tokens[i] = wrap("close cleanup", token)
	}
	return tokens
}
