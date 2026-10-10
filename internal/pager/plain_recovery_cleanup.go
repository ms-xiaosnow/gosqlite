package pager

import "errors"

// finishPlainRecovery ends only ownership. The bounded registry retains no
// replay plan, page bytes, recovery options, result or application callback.
// A failed explicit child close must not be attempted again by this operation's
// defer; subsequent token/snapshot retry performs managed retirement only.
func finishPlainRecovery(cleanup *plainRetirement, cause error, touched, removed bool) error {
	var failureClass error
	if touched {
		if removed {
			failureClass = errors.Join(ErrPoisoned, ErrCommitUncertain)
		} else {
			failureClass = errors.Join(ErrPoisoned, ErrRecoveryRequired)
		}
		if cause != nil {
			cause = errors.Join(failureClass, cause)
		}
	}
	return cleanup.beginClass(cause, failureClass, !cleanup.childClosePending())
}
