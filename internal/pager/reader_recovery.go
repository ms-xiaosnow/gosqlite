package pager

// rollbackJournalPresence is issued only by the successful journal metadata
// check. It is not evidence that the journal is hot, valid or recoverable.
type rollbackJournalPresence struct{ path string }

func (e *rollbackJournalPresence) Error() string {
	return "SQLite rollback journal exists; explicit recovery policy required"
}
func (e *rollbackJournalPresence) Unwrap() error { return ErrRecoveryRequired }

// retiredReaderJournal is a private, path-bound proof that one failed Reader
// probe had only the trusted presence result and fully retired without any
// diagnostic. No live Reader operation or generic ErrRecoveryRequired issues it.
type retiredReaderJournal struct {
	path  string
	cause error
}

func (e *retiredReaderJournal) Error() string { return e.cause.Error() }
func (e *retiredReaderJournal) Unwrap() error { return e.cause }

// finishReaderProbe preserves the constructor's existing single cleanup attempt.
// Only its verified, diagnostic-free completion can authorize a later recovery.
func finishReaderProbe(cleanup *plainRetirement, path string, cause error) error {
	result := cleanup.begin(cause)
	presence, ok := cause.(*rollbackJournalPresence)
	if !ok || presence == nil || presence.path != path {
		return result
	}
	cleanup.mu.Lock()
	done := cleanup.done
	cleanup.mu.Unlock()
	if !done || !plainSingleCause(result, func(e error) bool {
		p, ok := e.(*rollbackJournalPresence)
		return ok && p == presence
	}) {
		return result
	}
	return &retiredReaderJournal{path: path, cause: result}
}

// ReaderProbeRecoveryReady accepts only the exact private proof for this path.
// Wrapping/joining a proof, even alone, cannot authorize another native action.
// errors.Is/As, arbitrary wrapper behavior, cleanup retry and filesystem access
// are intentionally absent. The proof is only an admission condition: recovery
// must reacquire locks and revalidate the entire journal and database itself.
func ReaderProbeRecoveryReady(cause error, path string) bool {
	proof, ok := cause.(*retiredReaderJournal)
	return ok && proof != nil && proof.path == path
}
