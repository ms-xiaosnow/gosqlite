// Package fixturecleanup limits cleanup to completed, successful examples.
// It does not own database handles or credentials.
package fixturecleanup

import (
	"errors"
	"fmt"
)

// Finalize is called inside a deferred closure, registered just after MkdirTemp
// succeeds and before resource defers. completed and result must be evaluated
// inside that closure, so a later close error or incomplete run prevents removal.
// remove is called at most once, only after ordinary completed success. Removing
// files is not secure erasure. A failed removal may already have deleted some
// files, so neither this helper nor its error promises an intact failure fixture.
// Retaining encrypted files does not retain their process-local credentials.
func Finalize(dir string, completed bool, result error, remove func(string) error) error {
	if !completed || result != nil {
		return errors.Join(result, fmt.Errorf("example did not complete successfully; private fixture directory retained at %q", dir))
	}
	if err := remove(dir); err != nil {
		return fmt.Errorf("cleanup failed for private fixture directory %q; any remaining files are retained without retry (removal may be partial): %w", dir, err)
	}
	return nil
}
