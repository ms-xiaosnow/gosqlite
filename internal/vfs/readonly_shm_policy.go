package vfs

import "errors"

// ErrReadOnly rejects write-capable SHM operations before native lock calls.
var ErrReadOnly = errors.New("native shared-memory file is read-only")

// readonlySHMLockAccess is descriptor-free and used by the managed SHM owner.
// Shared acquisition and explicit release remain available to readers.
func readonlySHMLockAccess(writable, exclusive bool) error {
	if !writable && exclusive {
		return ErrReadOnly
	}
	return nil
}
