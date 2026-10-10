// Package sqlite exposes a native, pure-Go SQLite-format database engine.
// Unsupported SQL and file features fail closed. Production code never invokes
// or links SQLite itself.
package sqlite

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/extension/registry"
	"github.com/ms-xiaosnow/gosqlite/extension/similarity"
	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

var (
	ErrAuthentication   = encryption.ErrAuthentication
	ErrUnsupported      = errors.New("unsupported SQLite feature")
	ErrReadOnly         = errors.New("SQLite database is read-only")
	ErrClosed           = errors.New("SQLite database is closed")
	ErrMemoryLimit      = errors.New("SQLite query memory limit exceeded")
	ErrCorrupt          = pager.ErrCorrupt
	ErrBusy             = pager.ErrBusy
	ErrBusySnapshot     = pager.ErrBusySnapshot
	ErrChanged          = pager.ErrChanged
	ErrFull             = errors.New("SQLite database is full")
	ErrMismatch         = errors.New("SQLite datatype mismatch")
	ErrConstraint       = errors.New("SQLite constraint failed")
	ErrTxDone           = errors.New("SQLite transaction has ended")
	ErrRecoveryRequired = pager.ErrRecoveryRequired
	ErrCommitUncertain  = pager.ErrCommitUncertain
	ErrPoisoned         = pager.ErrPoisoned
)

// Code is a SQLite primary result code. Unsupported features use CodeError.
type Code int

const (
	CodeNotADB     Code = 26
	CodeOK         Code = 0
	CodeError      Code = 1
	CodeBusy       Code = 5
	CodeNoMem      Code = 7
	CodeReadOnly   Code = 8
	CodeInterrupt  Code = 9
	CodeIOErr      Code = 10
	CodeCorrupt    Code = 11
	CodeFull       Code = 13
	CodeMismatch   Code = 20
	CodeConstraint Code = 19
	CodeMisuse     Code = 21
)

// Error adds an operation and stable primary code, preserving errors.Is/As.
type Error struct {
	Code Code
	Op   string
	Err  error
}

func (e *Error) Error() string { return fmt.Sprintf("sqlite %s: %v", e.Op, e.Err) }
func (e *Error) Unwrap() error { return e.Err }
func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	var unsupported *sqlast.ErrUnsupported
	if errors.Is(err, record.ErrCorrupt) || errors.Is(err, ann.ErrCorrupt) || errors.Is(err, encryption.ErrFormat) || errors.Is(err, encryption.ErrIdentity) {
		err = fmt.Errorf("%w: %w", ErrCorrupt, err)
	}
	if errors.Is(err, record.ErrLimit) || errors.Is(err, pager.ErrLimit) || errors.Is(err, grams13.ErrLimit) || errors.Is(err, similarity.ErrLimit) || errors.Is(err, vector.ErrLimit) || errors.Is(err, ann.ErrLimit) || errors.Is(err, encryption.ErrLimit) {
		err = fmt.Errorf("%w: %w", ErrMemoryLimit, err)
	}
	if errors.As(err, &unsupported) || errors.Is(err, pager.ErrUnsupported) || errors.Is(err, record.ErrUnsupported) || errors.Is(err, encryption.ErrUnsupported) || errors.Is(err, encryption.ErrUnsupportedConfiguration) {
		err = fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	if errors.Is(err, btree.ErrDuplicate) {
		err = fmt.Errorf("%w: %w", ErrConstraint, err)
	}
	code := CodeError
	switch {
	case errors.Is(err, ErrCommitUncertain) || errors.Is(err, ErrPoisoned) || errors.Is(err, ErrRecoveryRequired):
		code = CodeIOErr
	case errors.Is(err, ErrFull):
		code = CodeFull
	case errors.Is(err, ErrMismatch):
		code = CodeMismatch
	case errors.Is(err, ErrBusy):
		code = CodeBusy
	case errors.Is(err, ErrMemoryLimit):
		code = CodeNoMem
	case errors.Is(err, ErrReadOnly):
		code = CodeReadOnly
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		code = CodeInterrupt
	case errors.Is(err, encryption.ErrAuthentication):
		code = CodeCorrupt
		if op == "open" {
			code = CodeNotADB
		}
	case errors.Is(err, ErrCorrupt):
		code = CodeCorrupt
	case errors.Is(err, ErrConstraint) || errors.Is(err, btree.ErrDuplicate):
		code = CodeConstraint
	case errors.Is(err, ErrClosed) || errors.Is(err, ErrTxDone) || errors.Is(err, pager.ErrClosed):
		code = CodeMisuse
	}
	return &Error{Code: code, Op: op, Err: err}
}

// Options configures a connection. Zero-value options open read/write and
// create an absent database. ReadOnly never creates or modifies files.
type Options struct {
	// Encryption explicitly selects the independent authenticated snapshot container.
	// Nil preserves the ordinary plaintext engine. Native WAL options are incompatible.
	Encryption *EncryptionOptions
	// IndexMethods selects an immutable snapshot of registered native index
	// methods. Nil installs the built-in grams13 method. Stored metadata never
	// loads code or changes this connection registry.
	IndexMethods *registry.Registry
	ReadOnly     bool
	NoCreate     bool
	// WALExclusive enables the restricted native WAL mode. It retains an
	// EXCLUSIVE database lock for the connection lifetime and allows one active
	// query or transaction lease. It requires writable access even for reads.
	WALExclusive bool
	// WALShared requests native shared-memory WAL coordination. Independent
	// connections may retain read snapshots while one writer commits.
	WALShared bool
	// WALPowersafeOverwrite is an explicit storage assertion for shared WAL:
	// writes cannot damage bytes outside their requested range. Default false
	// keeps conservative append/checkpoint gates. It is not autodetection.
	WALPowersafeOverwrite bool
	// KeepWALOnClose retains normally committed, synced WAL data without an
	// implicit checkpoint. Only meaningful in a WAL mode; default Close
	// attempts a checkpoint. WALExclusive then removes sidecars; WALShared
	// retains their inodes. This is not a crash-safety guarantee.
	KeepWALOnClose bool
	// MaxSortBytes bounds retained ORDER BY rows and UPDATE/DELETE rowid lists.
	// Zero selects 64 MiB. Sorting never spills plaintext to disk.
	MaxSortBytes int64
	// MaxTransactionPages bounds the pager's dirty-page transaction overlay.
	// Zero selects 4096 pages. Exceeding a quota rolls back the statement.
	MaxTransactionPages uint32
	// MaxTransactionUndoBytes bounds all retained statement/savepoint page
	// images and metadata together. Zero derives room for one worst-case
	// statement from the page and freelist quotas. Undo never spills to disk.
	MaxTransactionUndoBytes int64
	// PageSize applies only to newly created files. Zero selects 4096.
	PageSize uint32
	// SectorSize is the storage-sector assumption for rollback-journal
	// protection, not autodetection. Zero selects 4096 bytes. Set it no
	// smaller than the actual storage sector (power of two, 512..65536).
	SectorSize uint32
}

// FileInfo describes validated on-disk metadata.
type FileInfo struct {
	PageSize, UsableSize, PageCount, SchemaCookie, SchemaFormat, TextEncoding uint32
	ReadVersion, WriteVersion                                                 byte
}

// SchemaEntry is an entry from sqlite_schema. SQL is empty for internal indexes.
type SchemaEntry struct {
	Type, Name, TableName, SQL string
	RootPage                   uint32
}

// DB is a connection. Idle rollback-mode connections hold no shared lock.
// WALExclusive connections retain an exclusive lock for their full lifetime. Each Query owns
// its snapshot until Rows.Close or exhaustion; explicit transactions own one
// snapshot and overlay until Commit/Rollback. Concurrent methods are serialized
// per connection. Independent DB handles can be used concurrently.
type DB struct {
	mu      sync.RWMutex
	path    string
	info    FileInfo
	opts    Options
	closed  bool
	tx      *Tx
	rows    map[*Rows]struct{}
	session pageSession
	methods *registry.Registry
}

// Open opens a SQLite-format file. An absent file is created unless ReadOnly
// or NoCreate is set. Existing unsupported schemas are readable where possible,
// but are validated conservatively before any SQL mutation.
func Open(path string, opts Options) (*DB, error) {
	return OpenContext(context.Background(), path, opts)
}

// OpenContext opens a connection while honoring cancellation before physical
// publication. Once a mode-switch commit starts I/O it must finish or report
// an uncertain/poisoned result rather than abandon partially published data.
func OpenContext(ctx context.Context, path string, opts Options) (*DB, error) {
	if err := checkContext(ctx); err != nil {
		return nil, wrap("open", err)
	}
	if opts.Encryption != nil {
		cfg := *opts.Encryption
		opts.Encryption = &cfg
		if err := cfg.validate(); err != nil {
			return nil, wrap("open", err)
		}
		if opts.WALExclusive || opts.WALShared || opts.WALPowersafeOverwrite || (opts.KeepWALOnClose && cfg.Mode != EncryptionWAL && cfg.Mode != EncryptionWALShared) {
			return nil, wrap("open", fmt.Errorf("%w: encrypted storage rejects incompatible native WAL options", ErrUnsupported))
		}
	}
	if opts.WALExclusive && opts.WALShared {
		return nil, wrap("open", fmt.Errorf("WALExclusive and WALShared are mutually exclusive"))
	}
	if opts.WALPowersafeOverwrite && !opts.WALShared {
		return nil, wrap("open", fmt.Errorf("WALPowersafeOverwrite requires WALShared"))
	}
	if opts.ReadOnly && (opts.WALExclusive || opts.WALShared) {
		return nil, wrap("open", fmt.Errorf("%w: native WAL modes currently require writable access", ErrUnsupported))
	}
	if opts.KeepWALOnClose && !opts.WALExclusive && !opts.WALShared && (opts.Encryption == nil || (opts.Encryption.Mode != EncryptionWAL && opts.Encryption.Mode != EncryptionWALShared)) {
		return nil, wrap("open", fmt.Errorf("KeepWALOnClose requires a WAL mode"))
	}
	opts, err := normalizeConnectionLimits(opts)
	if err != nil {
		return nil, wrap("open", err)
	}
	// A connection must keep naming the same path across working-directory
	// changes and symlink retargeting while idle between snapshots.
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, wrap("open", err)
	}
	if opts.Encryption != nil {
		if info, e := os.Lstat(absolute); e == nil && !info.Mode().IsRegular() {
			return nil, wrap("open", ErrUnsupported)
		} else if e != nil && !os.IsNotExist(e) {
			return nil, wrap("open", e)
		}
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, wrap("open", err)
		}
		parent, parentErr := filepath.EvalSymlinks(filepath.Dir(absolute))
		if parentErr != nil {
			return nil, wrap("open", parentErr)
		}
		canonical = filepath.Join(parent, filepath.Base(absolute))
		// An absent final component may be created; a dangling final symlink
		// is refused rather than treating it as an authorized new database.
		if info, statErr := os.Lstat(canonical); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return nil, wrap("open", fmt.Errorf("%w: dangling database symlink", ErrUnsupported))
		}
	}
	path = canonical
	methods := opts.IndexMethods
	if methods == nil {
		methods, err = registry.New(grams13.Method())
		if err != nil {
			return nil, wrap("open index registry", err)
		}
	} else {
		methods = methods.Snapshot()
	}
	opts.IndexMethods = methods
	d := &DB{path: path, opts: opts, rows: make(map[*Rows]struct{}), methods: methods}
	if !opts.ReadOnly {
		if err := pager.ValidateOptions(d.pagerOptions()); err != nil {
			return nil, wrap("open", err)
		}
	}
	if opts.Encryption != nil {
		session, e := pager.OpenEncryptedSession(ctx, path, d.pagerOptions(), opts.Encryption.storageOptions(), opts.ReadOnly)
		if e != nil {
			return nil, wrap("open", e)
		}
		d.session = session
		d.info = fileInfo(session.Header())
		return d, nil
	}
	if opts.WALExclusive || opts.WALShared {
		var session pageSession
		var err error
		if opts.WALShared {
			session, err = pager.OpenSharedSession(ctx, path, d.pagerOptions())
		} else {
			session, err = pager.OpenSession(ctx, path, d.pagerOptions())
		}
		if err != nil {
			return nil, wrap("open", err)
		}
		d.session = session
		d.info = fileInfo(session.Header())
		return d, nil
	}
	// Prefer a read-only probe for existing files, so opening a handle does not
	// acquire a writer lock or rewrite an unchanged database header.
	r, err := pager.Open(path)
	if err == nil {
		d.info = fileInfo(r.Header())
		err = r.Close()
	} else if !opts.ReadOnly && !opts.NoCreate && errors.Is(err, os.ErrNotExist) {
		var p *pager.Tx
		p, err = pager.OpenTx(ctx, path, d.pagerOptions())
		if err == nil {
			d.info = fileInfo(p.Header())
			err = p.Commit(ctx)
			if err != nil {
				_ = p.Rollback()
			}
		}
	}
	if err != nil {
		return nil, wrap("open", err)
	}
	return d, nil
}
func fileInfo(h pager.Header) FileInfo {
	return FileInfo{PageSize: h.PageSize, UsableSize: h.UsableSize, PageCount: h.PageCount, SchemaCookie: h.SchemaCookie, SchemaFormat: h.SchemaFormat, TextEncoding: h.TextEncoding, ReadVersion: h.ReadVersion, WriteVersion: h.WriteVersion}
}
func (d *DB) pagerOptions() pager.Options {
	return pager.Options{Create: !d.opts.NoCreate && !d.opts.ReadOnly, PageSize: d.opts.PageSize, MaxDirtyPages: d.opts.MaxTransactionPages, MaxUndoBytes: d.opts.MaxTransactionUndoBytes, SectorSize: d.opts.SectorSize, KeepWALOnClose: d.opts.KeepWALOnClose, WALPowersafeOverwrite: d.opts.WALPowersafeOverwrite}
}

// Info returns the most recently observed header metadata.
func (d *DB) Info() FileInfo { d.mu.RLock(); defer d.mu.RUnlock(); return d.info }

// Schema returns the current catalog, including an explicit transaction's DDL.
func (d *DB) Schema(ctx context.Context) ([]SchemaEntry, error) {
	if err := checkContext(ctx); err != nil {
		return nil, wrap("schema", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil, wrap("schema", ErrClosed)
	}
	var reader btree.PageReader
	var close func() error
	if d.tx != nil {
		if err := d.tx.check(); err != nil {
			return nil, wrap("schema", err)
		}
		if err := d.tx.ensureSnapshotLocked(ctx); err != nil {
			return nil, wrap("schema", err)
		}
		reader = d.tx.reader
	} else {
		r, err := d.openReader(ctx)
		if err != nil {
			return nil, wrap("schema", err)
		}
		reader = r
		close = r.Close
	}
	if close != nil {
		defer close()
	}
	entries, err := btree.ReadSchema(ctx, reader)
	if err != nil {
		return nil, wrap("schema", err)
	}
	d.info = fileInfo(reader.Header())
	result := make([]SchemaEntry, len(entries))
	for i, e := range entries {
		result[i] = SchemaEntry{e.Type, e.Name, e.TableName, e.SQL, e.RootPage}
	}
	return result, nil
}

// Close rolls back an open transaction and releases every owned snapshot.
// Open iterators subsequently report ErrClosed. Close is idempotent.
func (d *DB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	var err error
	for r := range d.rows {
		r.mu.Lock()
		r.err = wrap("next", ErrClosed)
		err = errors.Join(err, r.close())
		r.mu.Unlock()
	}
	if d.tx != nil {
		err = errors.Join(err, d.tx.rollbackLocked())
	}
	if d.session != nil {
		err = errors.Join(err, d.session.Close())
	}
	return wrap("close", err)
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("nil context")
	}
	return ctx.Err()
}

// Query executes one SELECT. Parameters may be positional values or
// database/sql.Named arguments. Call Rows.Close even on early exit.
func (d *DB) Query(ctx context.Context, query string, args ...any) (*Rows, error) {
	if err := checkContext(ctx); err != nil {
		return nil, wrap("query", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil, wrap("query", ErrClosed)
	}
	if d.tx != nil {
		return d.tx.queryLocked(ctx, query, args)
	}
	stmt, err := sqlast.Parse(query)
	if err != nil {
		return nil, wrap("query", err)
	}
	sel, ok := stmt.(*sqlast.Select)
	if !ok {
		return nil, wrap("query", ErrReadOnly)
	}
	params, err := bind(stmt, args)
	if err != nil {
		return nil, wrap("bind", err)
	}
	reader, err := d.openReader(ctx)
	if err != nil {
		return nil, wrap("query", err)
	}
	rows, err := d.plan(ctx, reader, sel, params)
	if err != nil {
		reader.Close()
		return nil, wrap("query", err)
	}
	d.info = fileInfo(reader.Header())
	rows.release = reader.Close
	d.rows[rows] = struct{}{}
	return rows, nil
}

// ASCII folding matches SQLite identifier handling; Unicode case folding does
// not, for example, make distinct non-ASCII identifiers interchangeable.
func fold(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// openReader is called under the connection lock. A WAL session reader is a
// lease on its existing descriptor; opening another descriptor would conflict
// with that session's retained EXCLUSIVE lock.
func (d *DB) openReader(ctx context.Context) (*pager.Reader, error) {
	if d.session != nil {
		return d.session.Reader(ctx)
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	return pager.Open(d.path)
}

// Checkpoint copies committed WAL images into the main file and durably ends
// that WAL generation. It requires a WAL mode and no active local query/Tx lease.
// Normal-success tests do not establish interrupted-checkpoint durability.
func (d *DB) Checkpoint(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return wrap("checkpoint", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return wrap("checkpoint", ErrClosed)
	}
	if d.session == nil {
		return wrap("checkpoint", fmt.Errorf("%w: checkpoint requires a WAL mode", ErrUnsupported))
	}
	if d.tx != nil || len(d.rows) != 0 {
		return wrap("checkpoint", ErrBusy)
	}
	err := d.session.Checkpoint(ctx)
	if err == nil {
		d.info = fileInfo(d.session.Header())
	}
	return wrap("checkpoint", err)
}

// pageSession lets public connections share lifetime/lease semantics without
// treating exclusive locking as shared-WAL coordination.
type pageSession interface {
	Header() pager.Header
	Begin(context.Context) (*pager.Tx, error)
	Reader(context.Context) (*pager.Reader, error)
	Checkpoint(context.Context) error
	Close() error
}

// normalizeConnectionLimits is shared by opening and standalone copy validators
// so a restored catalog receives the same bounded defaults as a connection.
func normalizeConnectionLimits(opts Options) (Options, error) {
	if opts.MaxTransactionUndoBytes < 0 {
		return opts, fmt.Errorf("MaxTransactionUndoBytes must not be negative")
	}
	if opts.MaxSortBytes < 0 {
		return opts, fmt.Errorf("MaxSortBytes must not be negative")
	}
	if opts.MaxSortBytes == 0 {
		opts.MaxSortBytes = 64 << 20
	}
	if opts.MaxTransactionPages == 0 {
		opts.MaxTransactionPages = 4096
	}
	return opts, nil
}
