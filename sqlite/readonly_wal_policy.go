package sqlite

import "fmt"

// This policy is evaluated before public admission or path/native acquisition.
// ReadOnly without WALShared keeps the existing rollback-only default.
func readOnlyWALOptionPolicy(goos string, opts Options) error {
	if !opts.ReadOnly {
		return nil
	}
	if opts.WALExclusive {
		return fmt.Errorf("%w: readonly exclusive WAL is unsupported", ErrUnsupported)
	}
	if opts.WALShared && (goos != "linux" || opts.WALPowersafeOverwrite) {
		return fmt.Errorf("%w: readonly shared WAL requires Linux and no write-storage assertion", ErrUnsupported)
	}
	return nil
}
