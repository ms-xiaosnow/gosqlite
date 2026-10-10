package sqlite

import "github.com/ms-xiaosnow/gosqlite/internal/pager"

// ErrOpenCleanupPending identifies a failed shared-WAL open that retained native
// resource ownership when its error was produced. Such opening errors also match
// ErrPoisoned; the sentinel alone does not.
// That historical error remains unchanged after a successful RetryOpenCleanup.
var ErrOpenCleanupPending = pager.ErrOpenCleanupPending

// RetryOpenCleanup synchronously retries ordered resource retirement for every
// distinct failed shared-WAL open in openErr, including OpenContext and
// database/sql connector errors combined with errors.Join or ordinary %w wrappers.
// It discovers all groups before cleanup, never sweeps unrelated databases, and
// independently attempts each group before joining any current failures.
//
// Nil or an ordinary error without a retained group is a no-op. A nil result
// means all discovered groups completed cleanup, not that any database opened or
// original failure was fixed. Pending results match ErrOpenCleanupPending and
// ErrPoisoned, preserve original causes, and may themselves be used for retry.
// Calls serialize only for each same group and are idempotent after completion.
//
// Traversal is bounded to 4096 error nodes. An excessive or cyclic wrapper tree
// returns ErrMemoryLimit without attempting cleanup. This refusal cannot retain
// an unsafe wrapper chain; keep original opening errors to retry smaller trees.
//
// No background cleanup or raw native-handle retry is performed. Known-live
// resources can retire on retry; uncertain terminal ownership may require
// permanent retention. Retained groups and in-flight shared-WAL opens share a
// bounded capacity; exhaustion rejects another shared open before its pager
// acquires resources. Original pending errors remain immutable after retirement.
func RetryOpenCleanup(openErr error) error {
	return wrap("open cleanup", pager.RetryOpenCleanup(openErr))
}
