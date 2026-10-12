package pager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// ValidateRollbackBackupSource admits only a pinned ordinary rollback reader.
// The exact physical extent is required: copying a smaller logical extent could
// silently discard trailing pages. Stale SQLite page-count fields are accepted
// only through ParseHeader's documented physical-extent rule and are preserved.
func (r *Reader) ValidateRollbackBackupSource() (retErr error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lease != nil {
		return ErrUnsupported
	}
	if r.closed || r.file == nil {
		return ErrClosed
	}
	if err := r.cleanup.check(); err != nil {
		return err
	}
	defer func() { retErr = r.cleanup.observe(retErr) }()
	if err := r.checkStable(); err != nil {
		return err
	}
	if err := validateWritableFile(r.file); err != nil {
		return err
	}
	// A rollback backup has no SHM authority. In particular a directory at
	// this reserved name must not become an accepted destination parent.
	_, absent, err := nativeLstatForCreate(r.path + "-shm")
	if err != nil && !absent {
		return err
	}
	if !absent {
		return fmt.Errorf("%w: rollback backup refuses an existing SHM sidecar", ErrUnsupported)
	}
	return validateBackupHeader(r.header, r.rawHeader[:], r.info.Size())
}

func validateBackupHeader(h Header, raw []byte, size int64) error {
	parsed, err := ParseHeader(raw, size)
	if err != nil {
		return err
	}
	if parsed != h {
		return fmt.Errorf("%w: backup source header changed", ErrChanged)
	}
	if h.ReservedBytes != 0 || h.UsableSize != h.PageSize || h.SchemaFormat != 0 && h.SchemaFormat != 4 {
		return fmt.Errorf("%w: backup requires unreserved schema format 4", ErrUnsupported)
	}
	if int64(h.PageSize)*int64(h.PageCount) != size {
		return fmt.Errorf("%w: backup physical extent differs from logical extent", ErrUnsupported)
	}
	return nil
}

// WritePlaintextBackup copies a fully checked pinned source into a private
// already-locked candidate, then atomically publishes that same complete inode.
// The source proof belongs to Reader.cleanup; target cleanup never adopts it.
// The caller must keep the proof through full Check, this copy, and source-only
// proof retirement. No final entry is overwritten, removed or recovered.
func WritePlaintextBackup(ctx context.Context, destination string, source *Reader, proof *vfs.BackupSourceNamespace) (resultErr error) {
	if err := ctx.Err(); err != nil { return err }
	if err := PlaintextBackupPlatform(); err != nil { return err }
	if err := source.ValidateRollbackBackupSource(); err != nil { return err }
	sourceIdentity, err := source.ObserveBackupNamespace(proof)
	if err != nil { return err }
	if destination == "" { return ErrMisuse }
	path, err := filepath.Abs(destination)
	if err != nil { return err }
	if !vfs.BackupBasename(filepath.Base(path)) { return ErrUnsupported }
	parent, err := resolveNativePath(filepath.Dir(path))
	if err != nil { return err }
	cleanup, publication, err := newPlainBackupPublication()
	if err != nil { return err }
	defer func() {
		// nil is deliberate: resultErr can contain a SOURCE ownership token.
		// Only the pre-installed composite owns destination acquisition/close.
		retirement := cleanup.begin(nil)
		if resultErr == nil && retirement == nil {
			resultErr = nativeFileError(publication.SuccessfulPublication())
		}
		resultErr = finishPlaintextBackup(resultErr, retirement, publication.Status().MutationAttempted)
	}()
	h := source.Header()
	if err = publication.OpenBackupDestination(parent, filepath.Base(path), sourceIdentity, int64(h.PageCount)*int64(h.PageSize)); err != nil { return nativeFileError(err) }
	if err = source.ValidateRollbackBackupSource(); err != nil { return err }
	if _, err = source.ObserveBackupNamespace(proof); err != nil { return err }
	if err = ctx.Err(); err != nil { return err }
	if err = publication.CreatePrivateCandidate(); err != nil { return nativeFileError(err) }
	if err = copyBackupPages(ctx, h, source, func(page []byte, offset int64) error {
		return nativeFileError(publication.WritePage(page, offset))
	}); err != nil { return err }
	if err = source.ValidateRollbackBackupSource(); err != nil { return err }
	if _, err = source.ObserveBackupNamespace(proof); err != nil { return err }
	if err = ctx.Err(); err != nil { return err }
	// No cancellation after this boundary. Each Sync/publication is attempted
	// once; cleanup never repeats any copy, source, rename or Sync operation.
	if err = publication.SyncData(); err != nil { return nativeFileError(err) }
	if err = publication.PublishNoReplace(); err != nil { return nativeFileError(err) }
	if err = publication.SyncPublication(); err != nil { return nativeFileError(err) }
	return nil
}

// copyBackupPages holds at most one caller-owned page. It copies all bytes,
// including free/lock-byte pages and header fields; no format normalization or
// plaintext staging occurs. The writer callback never retains the page.
func copyBackupPages(ctx context.Context, h Header, source SnapshotReader, write func([]byte, int64) error) error {
	if h.PageSize < 512 || h.PageSize > 65536 || h.PageSize&(h.PageSize-1) != 0 || h.PageCount == 0 || h.PageCount > 0xfffffffe {
		return ErrCorrupt
	}
	for n := uint32(1); n <= h.PageCount; n++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		p, err := source.ReadPage(ctx, n)
		if err != nil {
			clear(p)
			return err
		}
		if len(p) != int(h.PageSize) {
			clear(p)
			return io.ErrUnexpectedEOF
		}
		err = write(p, int64(n-1)*int64(h.PageSize))
		clear(p)
		if err != nil {
			return err
		}
	}
	return nil
}

func finishPlaintextBackup(operation, retirement error, attemptedCreate bool) error {
	err := errors.Join(operation, retirement)
	if err != nil && attemptedCreate {
		return errors.Join(ErrCommitUncertain, err)
	}
	return err
}
