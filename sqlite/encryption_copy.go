package sqlite

import (
	"context"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/extension/registry"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
)

// validateCopy checks supported catalog grammar, all B-tree/overflow page
// ownership and freelist membership. It is not an index-to-table content audit.
func (d *DB) validateCopy(ctx context.Context, r pager.SnapshotReader) error {
	t := &Tx{db: d, ctx: ctx, reader: r}
	c, err := t.validateCatalog(ctx)
	if err != nil {
		return err
	}
	free, err := pager.SnapshotFreePages(ctx, r, 0)
	if err != nil {
		return err
	}
	var roots []btree.RootSpec
	for _, tab := range c.tables {
		roots = append(roots, btree.RootSpec{Page: tab.root, Kind: btree.TableTree})
		for _, idx := range tab.indexes {
			roots = append(roots, btree.RootSpec{Page: idx.root, Kind: btree.IndexTree, KeyInfo: btree.KeyInfo{Columns: len(idx.columns)}})
		}
	}
	return btree.CheckOwnership(ctx, r, roots, free)
}
func copyValidator() (*DB, error) {
	r, err := registry.New(grams13.Method())
	if err != nil {
		return nil, err
	}
	opts, err := normalizeConnectionLimits(Options{})
	if err != nil {
		return nil, err
	}
	return &DB{methods: r, opts: opts}, nil
}

// BackupEncrypted emits a portable single encrypted file with fresh DBID,
// epoch and role-separated backup keys. Copying the main manifest alone is not
// a backup. Existing destinations are refused; partial output remains encrypted.
func (d *DB) BackupEncrypted(ctx context.Context, destination string, target EncryptionOptions) (EncryptionInfo, error) {
	if err := checkContext(ctx); err != nil {
		return EncryptionInfo{}, wrap("encrypted backup", err)
	}
	if err := target.validateArchive(); err != nil {
		return EncryptionInfo{}, wrap("encrypted backup", err)
	}
	// Target credentials are acquired before taking engine mutexes.
	p, err := encstore.Prepare(ctx, destination, target.storageOptions(), encryption.OperationBackupTarget, [16]byte{})
	if err != nil {
		return EncryptionInfo{}, wrap("encrypted backup", err)
	}
	defer p.Close()
	d.mu.Lock()
	defer d.mu.Unlock()
	s, err := d.encryptedSession()
	if err != nil {
		return EncryptionInfo{}, wrap("encrypted backup", err)
	}
	if d.tx != nil || len(d.rows) != 0 {
		return EncryptionInfo{}, wrap("encrypted backup", ErrBusy)
	}
	i, err := s.BackupPrepared(ctx, destination, p, d.validateCopy)
	return encryptionInfo(i), wrap("encrypted backup", err)
}

// ExportPlaintext explicitly writes a new ordinary SQLite file, including all
// schema/index/page contents in plaintext. No temporary plaintext file is used.
// Existing destinations are refused. On error any partial named output remains
// at destination and must not be treated as a complete backup.
func (d *DB) ExportPlaintext(ctx context.Context, destination string) (FileInfo, error) {
	if err := checkContext(ctx); err != nil {
		return FileInfo{}, wrap("plaintext export", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	s, err := d.encryptedSession()
	if err != nil {
		return FileInfo{}, wrap("plaintext export", err)
	}
	if d.tx != nil || len(d.rows) != 0 {
		return FileInfo{}, wrap("plaintext export", ErrBusy)
	}
	if err = s.ExportPlaintext(ctx, destination, d.validateCopy); err != nil {
		return FileInfo{}, wrap("plaintext export", err)
	}
	return fileInfo(s.Header()), nil
}

// RestoreEncrypted authenticates a complete portable archive and validates its
// supported logical page ownership before creating an absent destination with
// fresh target database/epoch/file identities. The archive is never modified.
func RestoreEncrypted(ctx context.Context, archivePath, destination string, source, target EncryptionOptions) (_ EncryptionInfo, resultErr error) {
	if err := checkContext(ctx); err != nil {
		return EncryptionInfo{}, wrap("encrypted restore", err)
	}
	if err := source.validateArchive(); err != nil {
		return EncryptionInfo{}, wrap("encrypted restore", err)
	}
	if err := target.validate(); err != nil {
		return EncryptionInfo{}, wrap("encrypted restore", err)
	}
	a, err := pager.OpenEncryptedArchive(ctx, archivePath, source.storageOptions())
	if err != nil {
		return EncryptionInfo{}, wrap("encrypted restore", err)
	}
	defer func() { resultErr = wrap("encrypted restore close", a.CloseWithError(resultErr)) }()
	validator, err := copyValidator()
	if err != nil {
		return EncryptionInfo{}, wrap("encrypted restore", err)
	}
	if err = validator.validateCopy(ctx, a); err != nil {
		return EncryptionInfo{}, wrap("encrypted restore", err)
	}
	p, err := encstore.Prepare(ctx, destination, target.storageOptions(), encryption.OperationRestoreTarget, [16]byte{})
	if err != nil {
		return EncryptionInfo{}, wrap("encrypted restore", err)
	}
	defer p.Close()
	i, err := pager.CreateEncryptedSnapshot(ctx, destination, p, a)
	return encryptionInfo(i), wrap("encrypted restore", err)
}

// ImportEncrypted copies a supported ordinary rollback-mode SQLite snapshot to
// a different absent encrypted destination. It never modifies/deletes the
// plaintext source or erases existing plaintext copies. Native WAL imports are
// deliberately unsupported for this copy path.
func ImportEncrypted(ctx context.Context, sourcePath, destination string, plain Options, target EncryptionOptions) (EncryptionInfo, error) {
	if err := checkContext(ctx); err != nil {
		return EncryptionInfo{}, wrap("encrypted import", err)
	}
	if err := target.validate(); err != nil {
		return EncryptionInfo{}, wrap("encrypted import", err)
	}
	if plain.Encryption != nil || plain.WALExclusive || plain.WALShared || plain.WALPowersafeOverwrite || plain.KeepWALOnClose {
		return EncryptionInfo{}, wrap("encrypted import", fmt.Errorf("%w: import requires ordinary rollback-mode plaintext source", ErrUnsupported))
	}
	plain.ReadOnly = true
	plain.NoCreate = true
	db, err := OpenContext(ctx, sourcePath, plain)
	if err != nil {
		return EncryptionInfo{}, wrap("encrypted import", err)
	}
	defer db.Close()
	r, err := db.openReader(ctx)
	if err != nil {
		return EncryptionInfo{}, wrap("encrypted import", err)
	}
	defer r.Close()
	if err = r.ValidatePlaintextCopySource(); err != nil {
		return EncryptionInfo{}, wrap("encrypted import", err)
	}
	if err = db.validateCopy(ctx, r); err != nil {
		return EncryptionInfo{}, wrap("encrypted import", err)
	}
	p, err := encstore.Prepare(ctx, destination, target.storageOptions(), encryption.OperationCreate, [16]byte{})
	if err != nil {
		return EncryptionInfo{}, wrap("encrypted import", err)
	}
	defer p.Close()
	i, err := pager.CreateEncryptedSnapshot(ctx, destination, p, r)
	return encryptionInfo(i), wrap("encrypted import", err)
}
