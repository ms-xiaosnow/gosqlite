package sqlite

import (
	"errors"
	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
)

// ErrEncryptedCleanupPending marks an immutable historical token for retained
// encrypted managed ownership. Errors carrying a cleanup token also match
// ErrPoisoned and use CodeIOErr; the bare sentinel alone does not. A close
// diagnostic without this marker can describe a fully consumed owner.
var ErrEncryptedCleanupPending = encstore.ErrCleanupPending

// RetryEncryptedCleanup synchronously attempts all distinct encrypted cleanup
// scopes in wrapped/joined errors. Discovery precedes effects and is limited to
// 4096 error nodes. Oversized/cyclic trees return ErrMemoryLimit; keep original
// tokens and retry smaller trees. Old tokens remain historical after completion.
//
// A token from a live connection retires only already-close-requested temporary
// owners. It never closes selected live files, checkpoints, reacquires keys,
// reopens paths, replays transactions, or retries a numeric native handle. A
// whole-scope close preserves child -> lifecycle -> directory dependency order.
// A terminal native state may remain pending indefinitely. Completed scopes and
// errors without tokens are no-ops, not proof that every subsystem is clean.
func RetryEncryptedCleanup(err error) error {
	return wrap("encrypted cleanup", classifyEncryptedCleanup(encstore.RetryCleanup(err)))
}

// PendingEncryptedCleanups returns a new snapshot of encrypted pending tokens,
// including failed constructors and errors discarded by database/sql. It performs
// no cleanup. The process admits at most 1024 opening/live/pending scopes and
// 16384 common managed leases before native acquisition; the platform's physical
// limits also apply. Resources are never evicted, finalized, or swept by retry.
// Source-only exceptional-state corrections are not native failure qualification.
func PendingEncryptedCleanups() []error {
	tokens := encstore.PendingCleanups()
	for i, token := range tokens {
		tokens[i] = wrap("encrypted cleanup", classifyEncryptedCleanup(token))
	}
	return tokens
}

func classifyEncryptedCleanup(err error) error {
	if errors.Is(err, encstore.ErrPoisoned) {
		return errors.Join(ErrPoisoned, err)
	}
	return err
}
