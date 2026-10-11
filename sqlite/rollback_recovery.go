package sqlite

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/ms-xiaosnow/gosqlite/internal/journal"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
)

// RollbackRecoveryOptions opts Open into one bounded rollback-journal recovery
// attempt on Linux. This is experimental source-only recovery: physical replay
// and failure behavior have NOT passed the acceptance gate. Nil in Options keeps
// existing fail-closed behavior. Only writable plaintext rollback opens support
// this policy; it does not apply to later Query/Begin/Exec calls or WAL recovery.
//
// The supported subset uses published finite-count DELETE journal segments,
// matching geometry, unique original pages and no super/master journal. Cold,
// empty, malformed, ambiguous and unsupported journals are preserved. No-sync
// count sentinels and legacy unspecified page sizes remain unsupported.
// Limits bound parsing, not total process RSS. Allow roughly twice journal bytes
// plus O(MaxRecords) metadata. Zero fields select the defaults below.
type RollbackRecoveryOptions struct {
	MaxJournalBytes  uint64 // default 64 MiB
	MaxRecords       uint32 // default 65536, cumulative across segments
	MaxDatabaseBytes uint64 // default 1 TiB original database extent
}

func (o RollbackRecoveryOptions) pagerOptions() pager.RecoveryOptions {
	return pager.RecoveryOptions{Limits: journal.ReplayLimits{
		MaxJournalBytes: o.MaxJournalBytes, MaxRecords: o.MaxRecords,
		MaxDatabaseBytes: o.MaxDatabaseBytes,
	}}
}

// Descriptor-free policy validation precedes admission, path resolution and all
// native acquisition. A read-only flag is never silently upgraded to writable.
func validateRollbackRecovery(enabled, readOnly, walMode, encrypted bool, platform string) error {
	if !enabled {
		return nil
	}
	if readOnly {
		return fmt.Errorf("%w: rollback recovery requires writable access", ErrReadOnly)
	}
	if walMode || encrypted {
		return fmt.Errorf("%w: rollback recovery requires plaintext rollback mode", ErrUnsupported)
	}
	if platform != "linux" {
		return fmt.Errorf("%w: rollback recovery currently requires Linux", ErrUnsupported)
	}
	return nil
}

func snapshotRollbackRecovery(opts Options) (Options, error) {
	if err := validateRollbackRecovery(opts.RollbackRecovery != nil, opts.ReadOnly,
		opts.WALExclusive || opts.WALShared || opts.WALPowersafeOverwrite || opts.KeepWALOnClose,
		opts.Encryption != nil, runtime.GOOS); err != nil {
		return opts, err
	}
	if opts.RollbackRecovery != nil {
		copy := *opts.RollbackRecovery
		opts.RollbackRecovery = &copy
	}
	return opts, nil
}

// recoverRollbackOpen is the only production bridge to physical replay. It
// never retries cleanup or recovery. The proof belongs to the exact canonical
// path and exists only after the first probe completely retired without error.
func recoverRollbackOpen(ctx context.Context, path string, initial error, opts RollbackRecoveryOptions) (FileInfo, error) {
	if !pager.ReaderProbeRecoveryReady(initial, path) {
		return FileInfo{}, initial
	}
	return runRollbackRecovery(ctx, initial, func() error {
		_, err := pager.RecoverRollback(ctx, path, opts.pagerOptions())
		return err
	}, func() (FileInfo, error) {
		// Open reserves a fresh root. The first probe's admission was consumed;
		// never reuse it, recurse through public Open, or create on this retry.
		r, err := pager.Open(path)
		if err != nil {
			return FileInfo{}, err
		}
		info := fileInfo(r.Header())
		if err = r.Close(); err != nil {
			return FileInfo{}, err
		}
		return info, nil
	})
}

// runRollbackRecovery is a descriptor-free sequencing law, with callbacks
// supplied only by the private bridge above. Tests supply memory-only callbacks;
// neither the real recovery bridge nor enabled public Open runs in those tests.
// Exact bound: one recovery and, only after success, one fresh probe. Any error
// retains the original probe cause and every later cleanup/ownership diagnostic.
func runRollbackRecovery(ctx context.Context, initial error, recover func() error, probe func() (FileInfo, error)) (FileInfo, error) {
	if err := checkContext(ctx); err != nil {
		return FileInfo{}, errors.Join(initial, err)
	}
	if err := recover(); err != nil {
		return FileInfo{}, errors.Join(initial, err)
	}
	if err := checkContext(ctx); err != nil {
		return FileInfo{}, errors.Join(initial, err)
	}
	info, err := probe()
	if err != nil {
		return FileInfo{}, errors.Join(initial, err)
	}
	return info, nil
}
