package sqlite

import (
	"context"
	"errors"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
)

// Backup is a partial, unqualified Linux/ext-family plaintext backup API for
// a checked ordinary rollback snapshot. It preserves the final file/CLI --out
// meaning, every page byte, and Check's resource limits. Required Openat2,
// statx unique mount-ID fields and no-replace rename have no fallback. The
// ext-family magic is an admission signal, not proof of ext4 or crash durability.
// Other filesystems (including tmpfs/overlay) and non-Linux are unsupported.
//
// A retained existing trusted/private parent contains the random private stage.
// It is EXCLUSIVE-locked before copying, file-synced before no-replace publication,
// then directory-synced and natively retired before main's protector is closed.
// An ordinary SQLite client of the exact final main may race: its earlier main
// creation makes publication refuse, or it opens the complete already-locked
// published inode. This does not atomically reserve the four family names.
//
// Required contract: stable source/target namespaces, disjoint ordinary SQLite
// families, no private-stage access, no entry substitution/unlink/remount, no
// direct sidecar mutation or sidecar-as-independent-main, and compatible locks
// and storage. Permission bits cannot prove these premises. Main names use
// bounded ASCII letters/digits/dot/underscore/hyphen, without sidecar suffixes.
// Source journals/SHM reject; WAL may only be an empty regular direct child.
// Held-main/parent/relative-entry mount facts reject direct per-file bind aliases,
// but do not discover all hidden aliases elsewhere. Full native qualification,
// broader filesystem support and general-capability release remain pending.
//
// Encryption, native WAL, active DB Tx/Rows and read-write Tx are unsupported.
// Use a read-only Tx's Backup for its pinned snapshot. Failures retain candidate
// or final artifacts; after mutation begins ErrCommitUncertain is conservative.
// Never retry the backup automatically. ErrPlaintextCleanupPending permits only
// explicit represented resource retirement, never copy/Sync/rename/recovery.
func (d *DB) Backup(ctx context.Context, destination string) (result FileInfo, resultErr error) {
	if err := checkContext(ctx); err != nil {
		return FileInfo{}, wrap("backup", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.backupProfileLocked(); err != nil {
		return FileInfo{}, wrap("backup", err)
	}
	if d.tx != nil || len(d.rows) != 0 {
		return FileInfo{}, wrap("backup", ErrBusy)
	}
	r, err := d.openReader(ctx)
	if err != nil {
		return FileInfo{}, wrap("backup", err)
	}
	defer func() {
		closeErr := r.CloseBackupReader()
		if closeErr != nil && result.PageCount != 0 {
			closeErr = errors.Join(ErrCommitUncertain, closeErr)
		}
		resultErr = wrapCleanup("backup", resultErr, closeErr)
		if resultErr != nil {
			result = FileInfo{}
		}
	}()
	return d.backupSnapshotLocked(ctx, destination, r)
}

// Backup copies the pinned snapshot of an explicitly read-only transaction.
// A lazy transaction pins its snapshot on this call. The transaction remains
// open, and its caller remains responsible for Commit/Rollback and any cleanup.
// Read-write transactions, encryption, native WAL and active Rows are refused
// before destination access. No uncommitted write overlay is ever exported.
func (t *Tx) Backup(ctx context.Context, destination string) (FileInfo, error) {
	if err := checkContext(ctx); err != nil {
		return FileInfo{}, wrap("backup", err)
	}
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	if err := t.check(); err != nil {
		return FileInfo{}, wrap("backup", err)
	}
	if err := t.db.backupProfileLocked(); err != nil {
		return FileInfo{}, wrap("backup", err)
	}
	if !t.readOnly {
		return FileInfo{}, wrap("backup", fmt.Errorf("%w: backup requires a read-only transaction", ErrUnsupported))
	}
	if t.activeRows != 0 {
		return FileInfo{}, wrap("backup", ErrBusy)
	}
	if err := t.ensureSnapshotLocked(ctx); err != nil {
		return FileInfo{}, wrap("backup", err)
	}
	r, ok := t.reader.(*pager.Reader)
	if !ok {
		return FileInfo{}, wrap("backup", ErrUnsupported)
	}
	copyContext, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(t.ctx, cancel)
	defer cancel()
	defer stop()
	if err := t.ctx.Err(); err != nil {
		return FileInfo{}, wrap("backup", err)
	}
	return t.db.backupSnapshotLocked(copyContext, destination, r)
}

func (d *DB) backupProfileLocked() error {
	if d.closed {
		return ErrClosed
	}
	if d.session != nil || d.opts.Encryption != nil || d.opts.WALExclusive || d.opts.WALShared {
		return fmt.Errorf("%w: backup requires ordinary rollback plaintext storage", ErrUnsupported)
	}
	return pager.PlaintextBackupPlatform()
}

func (d *DB) backupSnapshotLocked(ctx context.Context, destination string, r *pager.Reader) (result FileInfo, resultErr error) {
	if err := r.ValidateRollbackBackupSource(); err != nil { return FileInfo{}, wrap("backup", err) }
	proof, err := r.BeginBackupNamespace()
	if err != nil { return FileInfo{}, wrap("backup", err) }
	defer func() {
		closeErr := r.EndBackupNamespace(proof)
		if closeErr != nil && result.PageCount != 0 { closeErr = errors.Join(ErrCommitUncertain, closeErr) }
		resultErr = wrapCleanup("backup", resultErr, closeErr)
		if resultErr != nil { result = FileInfo{} }
	}()
	o, err := checkOptions(nil)
	if err != nil { return FileInfo{}, wrap("backup", err) }
	if _, err = d.checkSnapshot(ctx, r, o); err != nil { return FileInfo{}, wrap("backup", err) }
	if err = pager.WritePlaintextBackup(ctx, destination, r, proof); err != nil { return FileInfo{}, wrap("backup", err) }
	return fileInfo(r.Header()), nil
}
