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
const (
	EncryptionSnapshot EncryptionMode = 0
	// EncryptionWAL writes immutable final-dirty-page transaction segments. It
	// currently holds a lifetime-exclusive writer lease; readers cannot coexist.
	EncryptionWAL EncryptionMode = 1
	// EncryptionWALShared uses layout 3 with simultaneous immutable readers and one writer.
	EncryptionWALShared EncryptionMode = 2
)

// EncryptionWALLimits separately bounds active replay and peak storage-adapter
// resources; transaction and retained-history quotas remain additive. Zero
// fields select documented defaults.
type EncryptionWALLimits struct {
	MaxSegments       uint32
	MaxFrames         uint64
	MaxSegmentFrames  uint32
	MaxSegmentBytes   uint64
	MaxBytes          uint64
	MaxRootBytes      uint32
	MaxOverlayPages   uint32
	MaxMemoryBytes    uint64
	MaxOpenFiles      uint32
	MaxScanBytes      uint64
	MaxOperationBytes uint64
	MaxReplaySteps    uint64
}

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
	WALLimits        EncryptionWALLimits
	// Retained ciphertext and abandoned candidates are charged before publication.
	// Defaults: 256 successful generations, 1 GiB family bytes, 16384 directory entries.
	// Backup/restore to a fresh family resets generation history without deleting it.
	MaxGenerations   uint64
	MaxRetainedBytes uint64
	// MaxDirectoryEntries limits scanned entries, including unrelated names.
	// Publication also reserves its own known upcoming entries.
	MaxDirectoryEntries uint32
}

func (EncryptionOptions) Format(s fmt.State, verb rune) {
	_, _ = s.Write([]byte("sqlite.EncryptionOptions{redacted}"))
}
func (o EncryptionOptions) storageOptions() encstore.Options {
	return encstore.Options{Mode: encstore.Mode(o.Mode), WALLimits: encstore.WALLimits(o.WALLimits), KeyProvider: o.KeyProvider, CredentialKind: o.CredentialKind, CreateKDF: o.CreateKDF, KDFPolicy: o.KDFPolicy, DerivationBudget: o.DerivationBudget, Limits: o.Limits, MaxGenerations: o.MaxGenerations, MaxRetainedBytes: o.MaxRetainedBytes, MaxDirectoryEntries: o.MaxDirectoryEntries}
}
func (o EncryptionOptions) validate() error {
	if o.Mode != EncryptionSnapshot && o.Mode != EncryptionWAL && o.Mode != EncryptionWALShared {
		return ErrUnsupported
	}
	return o.storageOptions().Validate()
}

// EncryptionInfo describes a fully authenticated selected logical head. It does not
// assert a global SQL/index integrity audit or protection against whole-snapshot
// rollback. RetainsHistory means prior ciphertext remains on disk and remains
// decryptable with its prior credential; no secure deletion is performed.
type EncryptionInfo struct {
	Mode                EncryptionMode
	StorageLayout       uint16
	WALSegments         uint32
	WALFrames, WALBytes uint64
	HeadCommitID        [32]byte
	PageSize, PageCount uint32
	DatabaseID          [16]byte
	CommitSequence      uint64
	RetainsHistory      bool
}

func encryptionInfo(i encstore.Info) EncryptionInfo {
	return EncryptionInfo{Mode: EncryptionMode(i.Mode), StorageLayout: i.StorageLayout, WALSegments: i.WALSegments, WALFrames: i.WALFrames, WALBytes: i.WALBytes, HeadCommitID: i.HeadCommitID, PageSize: i.PageSize, PageCount: i.PageCount, DatabaseID: i.DatabaseID, CommitSequence: i.CommitSequence, RetainsHistory: i.RetainsHistory}
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
	d.info = fileInfo(s.Header())
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
	if err == nil && target.Mode != d.opts.Encryption.Mode {
		err = ErrUnsupported
	}
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
	if err = s.RekeyPrepared(ctx, prepared, d.validateCopy); err != nil {
		return wrap("rekey", err)
	}
	cfg := target
	d.opts.Encryption = &cfg
	return nil
}

// UpgradeEncryptedWAL explicitly materializes a snapshot family into layout 2.
// It requires a writable quiescent connection and preserves old ciphertext.
// Reopen with EncryptionWAL; mode selection never silently changes on Open.
func (d *DB) UpgradeEncryptedWAL(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return wrap("upgrade encrypted WAL", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	s, err := d.encryptedSession()
	if err != nil {
		return wrap("upgrade encrypted WAL", err)
	}
	if d.opts.ReadOnly {
		return wrap("upgrade encrypted WAL", ErrReadOnly)
	}
	if d.tx != nil || len(d.rows) != 0 {
		return wrap("upgrade encrypted WAL", ErrBusy)
	}
	if d.opts.Encryption.Mode != EncryptionSnapshot {
		return wrap("upgrade encrypted WAL", ErrUnsupported)
	}
	if err = s.UpgradeWAL(ctx, d.validateCopy); err != nil {
		return wrap("upgrade encrypted WAL", err)
	}
	cfg := *d.opts.Encryption
	cfg.Mode = EncryptionWAL
	d.opts.Encryption = &cfg
	return nil
}

func (o EncryptionOptions) validateArchive() error {
	if o.Mode != EncryptionSnapshot {
		return ErrUnsupported
	}
	return o.validate()
}

// UpgradeEncryptedWALShared explicitly publishes layout 3 from a quiescent
// snapshot or exclusive WAL connection. It preserves identity and ciphertext
// history, then permits simultaneous readers and a single admitted writer.
func (d *DB) UpgradeEncryptedWALShared(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return wrap("upgrade encrypted shared WAL", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	s, err := d.encryptedSession()
	if err != nil {
		return wrap("upgrade encrypted shared WAL", err)
	}
	if d.opts.ReadOnly {
		return wrap("upgrade encrypted shared WAL", ErrReadOnly)
	}
	if d.tx != nil || len(d.rows) != 0 {
		return wrap("upgrade encrypted shared WAL", ErrBusy)
	}
	if d.opts.Encryption.Mode == EncryptionWALShared {
		return wrap("upgrade encrypted shared WAL", ErrUnsupported)
	}
	if err = s.UpgradeShared(ctx, d.validateCopy); err != nil {
		return wrap("upgrade encrypted shared WAL", err)
	}
	cfg := *d.opts.Encryption
	cfg.Mode = EncryptionWALShared
	d.opts.Encryption = &cfg
	return nil
}
