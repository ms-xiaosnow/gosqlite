package sqlite

import (
	"context"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
)

type EncryptionMode uint8

// EncryptionSnapshot writes a full immutable authenticated snapshot per changed
// transaction. It is intentionally not native SQLite WAL or SEE ciphertext.
const EncryptionSnapshot EncryptionMode = 0

// EncryptionOptions selects experimental encrypted storage explicitly. Raw keys
// and passwords are borrowed from a code-configured provider, never a DSN/SQL
// value. The option struct is copied; providers and derivation budgets are shared
// trusted application code and must be concurrency-safe if reused.
type EncryptionOptions struct {
	KeyProvider      encryption.KeyProvider
	CredentialKind   encryption.CredentialKind
	CreateKDF        encryption.Argon2Params
	KDFPolicy        encryption.KDFPolicy
	DerivationBudget *encryption.DerivationBudget
	Limits           encryption.Limits
	Mode             EncryptionMode
	// Retained ciphertext and abandoned candidates are charged before publication.
	// Defaults: 256 successful generations, 1 GiB family bytes, 16384 directory entries.
	// Backup/restore to a fresh family resets generation history without deleting it.
	MaxGenerations   uint64
	MaxRetainedBytes uint64
	// MaxDirectoryEntries limits scan work, including unrelated names. A commit
	// may add entries; later writes refuse if the next inventory exceeds this cap.
	MaxDirectoryEntries uint32
}

func (EncryptionOptions) Format(s fmt.State, verb rune) {
	_, _ = s.Write([]byte("sqlite.EncryptionOptions{redacted}"))
}
func (o EncryptionOptions) storageOptions() encstore.Options {
	return encstore.Options{KeyProvider: o.KeyProvider, CredentialKind: o.CredentialKind, CreateKDF: o.CreateKDF, KDFPolicy: o.KDFPolicy, DerivationBudget: o.DerivationBudget, Limits: o.Limits, MaxGenerations: o.MaxGenerations, MaxRetainedBytes: o.MaxRetainedBytes, MaxDirectoryEntries: o.MaxDirectoryEntries}
}
func (o EncryptionOptions) validate() error {
	if o.Mode != EncryptionSnapshot {
		return ErrUnsupported
	}
	return o.storageOptions().Validate()
}

// EncryptionInfo describes a fully authenticated physical snapshot. It does not
// assert a global SQL/index integrity audit or protection against whole-snapshot
// rollback. RetainsHistory means prior ciphertext remains on disk and remains
// decryptable with its prior credential; no secure deletion is performed.
type EncryptionInfo struct {
	PageSize, PageCount uint32
	DatabaseID          [16]byte
	CommitSequence      uint64
	RetainsHistory      bool
}

func encryptionInfo(i encstore.Info) EncryptionInfo {
	return EncryptionInfo{PageSize: i.PageSize, PageCount: i.PageCount, DatabaseID: i.DatabaseID, CommitSequence: i.CommitSequence, RetainsHistory: i.RetainsHistory}
}
func (d *DB) encryptedSession() (*pager.EncryptedSession, error) {
	if d.closed {
		return nil, ErrClosed
	}
	s, ok := d.session.(*pager.EncryptedSession)
	if !ok {
		return nil, ErrUnsupported
	}
	return s, nil
}
func (d *DB) VerifyEncrypted(ctx context.Context) (EncryptionInfo, error) {
	if err := checkContext(ctx); err != nil {
		return EncryptionInfo{}, wrap("verify encrypted", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	s, err := d.encryptedSession()
	if err != nil {
		return EncryptionInfo{}, wrap("verify encrypted", err)
	}
	if d.tx != nil || len(d.rows) != 0 {
		return EncryptionInfo{}, wrap("verify encrypted", ErrBusy)
	}
	if err = s.Verify(ctx); err != nil {
		return EncryptionInfo{}, wrap("verify encrypted", err)
	}
	return encryptionInfo(s.EncryptionInfo()), nil
}

// Rekey streams every logical page under fresh target epoch/file keys and then
// publishes a verified new root. Prior encrypted generations are retained.
// No Rows/Tx may be active. Provider callbacks run outside engine mutexes.
func (d *DB) Rekey(ctx context.Context, target EncryptionOptions) error {
	if err := checkContext(ctx); err != nil {
		return wrap("rekey", err)
	}
	if err := target.validate(); err != nil {
		return wrap("rekey", err)
	}
	d.mu.Lock()
	s, err := d.encryptedSession()
	if err == nil && d.opts.ReadOnly {
		err = ErrReadOnly
	}
	if err == nil && (d.tx != nil || len(d.rows) != 0) {
		err = ErrBusy
	}
	var id [16]byte
	if err == nil {
		id = s.DatabaseID()
	}
	path := d.path
	d.mu.Unlock()
	if err != nil {
		return wrap("rekey", err)
	}
	prepared, err := encstore.Prepare(ctx, path, target.storageOptions(), encryption.OperationRekeyTarget, id)
	if err != nil {
		return wrap("rekey", err)
	}
	defer prepared.Close()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return wrap("rekey", ErrClosed)
	}
	if d.tx != nil || len(d.rows) != 0 {
		return wrap("rekey", ErrBusy)
	}
	if err = d.validateEncryptedCopyLocked(ctx, s); err != nil {
		return wrap("rekey", err)
	}
	if err = s.RekeyPrepared(ctx, prepared); err != nil {
		return wrap("rekey", err)
	}
	cfg := target
	d.opts.Encryption = &cfg
	return nil
}
