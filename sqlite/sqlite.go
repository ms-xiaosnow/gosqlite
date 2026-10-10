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

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

var (
	ErrUnsupported      = errors.New("unsupported SQLite feature")
	ErrReadOnly         = errors.New("SQLite database is read-only")
	ErrClosed           = errors.New("SQLite database is closed")
	ErrMemoryLimit      = errors.New("SQLite query memory limit exceeded")
	ErrCorrupt          = pager.ErrCorrupt
	ErrBusy             = pager.ErrBusy
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
	if errors.Is(err, record.ErrCorrupt) {
		err = fmt.Errorf("%w: %w", ErrCorrupt, err)
	}
	if errors.Is(err, record.ErrLimit) || errors.Is(err, pager.ErrLimit) {
		err = fmt.Errorf("%w: %w", ErrMemoryLimit, err)
	}
	if errors.As(err, &unsupported) || errors.Is(err, pager.ErrUnsupported) || errors.Is(err, record.ErrUnsupported) {
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
	ReadOnly bool
	NoCreate bool
	// MaxSortBytes bounds retained ORDER BY rows and UPDATE/DELETE rowid lists.
	// Zero selects 64 MiB. Sorting never spills plaintext to disk.
	MaxSortBytes int64
	// MaxTransactionPages bounds the pager's dirty-page transaction overlay.
	// Zero selects 4096 pages. Exceeding a quota rolls back the statement.
	MaxTransactionPages uint32
	// PageSize applies only to newly created files. Zero selects 4096.
	PageSize uint32
	// SectorSize is the storage-sector assumption for rollback-journal
	// protection, not autodetection. Zero selects 4096 bytes. Set it no
	// smaller than the actual storage sector (power of two, 512..65536).
	SectorSize uint32
}

// FileInfo describes validated on-disk metadata.
type FileInfo struct{ PageSize, UsableSize, PageCount, SchemaCookie, SchemaFormat, TextEncoding uint32 }

// SchemaEntry is an entry from sqlite_schema. SQL is empty for internal indexes.
type SchemaEntry struct {
	Type, Name, TableName, SQL string
	RootPage                   uint32
}

// DB is a connection. Idle connections hold no shared lock. Each Query owns
// its snapshot until Rows.Close or exhaustion; explicit transactions own one
// snapshot and overlay until Commit/Rollback. Concurrent methods are serialized
// per connection. Independent DB handles can be used concurrently.
type DB struct {
	mu     sync.RWMutex
	path   string
	info   FileInfo
	opts   Options
	closed bool
	tx     *Tx
	rows   map[*Rows]struct{}
}

// Open opens a SQLite-format file. An absent file is created unless ReadOnly
// or NoCreate is set. Existing unsupported schemas are readable where possible,
// but are validated conservatively before any SQL mutation.
func Open(path string, opts Options) (*DB, error) {
	if opts.MaxSortBytes < 0 {
		return nil, wrap("open", fmt.Errorf("MaxSortBytes must not be negative"))
	}
	if opts.MaxSortBytes == 0 {
		opts.MaxSortBytes = 64 << 20
	}
	if opts.MaxTransactionPages == 0 {
		opts.MaxTransactionPages = 4096
	}
	// A connection must keep naming the same path across working-directory
	// changes and symlink retargeting while idle between snapshots.
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, wrap("open", err)
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
	d := &DB{path: path, opts: opts, rows: make(map[*Rows]struct{})}
	if !opts.ReadOnly {
		if err := pager.ValidateOptions(d.pagerOptions()); err != nil {
			return nil, wrap("open", err)
		}
	}
	// Prefer a read-only probe for existing files, so opening a handle does not
	// acquire a writer lock or rewrite an unchanged database header.
	r, err := pager.Open(path)
	if err == nil {
		d.info = fileInfo(r.Header())
		err = r.Close()
	} else if !opts.ReadOnly && !opts.NoCreate && errors.Is(err, os.ErrNotExist) {
		var p *pager.Tx
		p, err = pager.OpenTx(context.Background(), path, d.pagerOptions())
		if err == nil {
			d.info = fileInfo(p.Header())
			err = p.Commit(context.Background())
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
	return FileInfo{h.PageSize, h.UsableSize, h.PageCount, h.SchemaCookie, h.SchemaFormat, h.TextEncoding}
}
func (d *DB) pagerOptions() pager.Options {
	return pager.Options{Create: !d.opts.NoCreate && !d.opts.ReadOnly, PageSize: d.opts.PageSize, MaxDirtyPages: d.opts.MaxTransactionPages, SectorSize: d.opts.SectorSize}
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
		reader = d.tx.reader
	} else {
		r, err := pager.Open(d.path)
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
	reader, err := pager.Open(d.path)
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
