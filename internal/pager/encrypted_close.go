package pager

import "errors"

// Only the actual encrypted Store implements this in production. Tests exercise
// orchestration with descriptor-free owners, without injecting native failures.
type encryptedRetirementOwner interface {
	CloseWithError(error) error
	RetirementComplete() bool
}

func retireEncryptedSession(closed *bool, active, readOnly, keepWAL bool, poisoned, cause error, owner encryptedRetirementOwner, checkpoint func() error) (bool, error) {
	if active {
		return false, ErrBusy
	}
	var checkpointErr error
	if !*closed {
		*closed = true
		if !readOnly && !keepWAL && poisoned == nil && cause == nil {
			checkpointErr = checkpoint()
		}
	}
	err := encryptedError(owner.CloseWithError(errors.Join(cause, poisoned, checkpointErr)))
	return owner.RetirementComplete(), err
}
