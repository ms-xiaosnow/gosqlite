// Package sqlite exposes a native, pure-Go, read-only SQLite file reader.
// This milestone intentionally refuses writes, transactions, WAL, and SQL
// outside its documented subset. It never invokes or links SQLite itself.
package sqlite

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

var (
	ErrUnsupported = errors.New("unsupported SQLite feature")
	ErrReadOnly    = errors.New("SQLite database is read-only")
	ErrClosed      = errors.New("SQLite database is closed")
	ErrMemoryLimit = errors.New("SQLite query memory limit exceeded")
	ErrCorrupt     = pager.ErrCorrupt
	ErrBusy        = pager.ErrBusy
	ErrChanged     = pager.ErrChanged
)

// Code is a SQLite primary result code. Unsupported features use CodeError.
type Code int

const (
	CodeOK        Code = 0
	CodeError     Code = 1
	CodeBusy      Code = 5
	CodeNoMem     Code = 7
	CodeReadOnly  Code = 8
	CodeInterrupt Code = 9
	CodeCorrupt   Code = 11
	CodeMisuse    Code = 21
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
	if errors.Is(err, record.ErrLimit) {
		err = fmt.Errorf("%w: %w", ErrMemoryLimit, err)
	}
	if errors.As(err, &unsupported) || errors.Is(err, pager.ErrUnsupported) || errors.Is(err, record.ErrUnsupported) {
		err = fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	code := CodeError
	switch {
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
	case errors.Is(err, ErrClosed) || errors.Is(err, pager.ErrClosed):
		code = CodeMisuse
	}
	return &Error{Code: code, Op: op, Err: err}
}

// Options configures a connection. Zero-value options open read-only too:
// write support cannot be enabled in this milestone.
type Options struct {
	ReadOnly bool
	// MaxSortBytes bounds the accounted retained ORDER BY row/key memory.
	// Zero selects 64 MiB. Negative values are invalid. Sorting never spills.
	MaxSortBytes int64
}

// FileInfo describes validated on-disk metadata.
type FileInfo struct{ PageSize, UsableSize, PageCount, SchemaCookie, SchemaFormat, TextEncoding uint32 }

// SchemaEntry is an entry from sqlite_schema. SQL is empty for internal indexes.
type SchemaEntry struct {
	Type, Name, TableName, SQL string
	RootPage                   uint32
}

// DB owns a read-only descriptor and its shared lock until Close. Query and
// Schema may be called concurrently. A Rows iterator is not a shared cursor.
type DB struct {
	mu     sync.RWMutex
	reader *pager.Reader
	info   FileInfo
	opts   Options
	closed bool
}

// Open opens an existing SQLite database without creating or modifying files.
func Open(path string, opts Options) (*DB, error) {
	if opts.MaxSortBytes < 0 {
		return nil, wrap("open", fmt.Errorf("MaxSortBytes must not be negative"))
	}
	if opts.MaxSortBytes == 0 {
		opts.MaxSortBytes = 64 << 20
	}
	r, err := pager.Open(path)
	if err != nil {
		return nil, wrap("open", err)
	}
	h := r.Header()
	return &DB{reader: r, opts: opts, info: FileInfo{h.PageSize, h.UsableSize, h.PageCount, h.SchemaCookie, h.SchemaFormat, h.TextEncoding}}, nil
}

// Info returns immutable metadata from the database header.
func (d *DB) Info() FileInfo { return d.info }

// Schema returns the database catalog without interpreting unsupported tables.
func (d *DB) Schema(ctx context.Context) ([]SchemaEntry, error) {
	if ctx == nil {
		return nil, wrap("schema", fmt.Errorf("nil context"))
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return nil, wrap("schema", ErrClosed)
	}
	entries, err := btree.ReadSchema(ctx, d.reader)
	if err != nil {
		return nil, wrap("schema", err)
	}
	result := make([]SchemaEntry, len(entries))
	for i, e := range entries {
		result[i] = SchemaEntry{e.Type, e.Name, e.TableName, e.SQL, e.RootPage}
	}
	return result, nil
}

// Close releases the descriptor and shared lock. It is idempotent. Iterators
// still open after Close fail on their next Next call and should be closed.
func (d *DB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	return wrap("close", d.reader.Close())
}

// Query executes one supported SELECT. Parameters may be ordinary positional
// values or database/sql.Named arguments. Call Rows.Close even on early exit.
func (d *DB) Query(ctx context.Context, query string, args ...any) (*Rows, error) {
	if ctx == nil {
		return nil, wrap("query", fmt.Errorf("nil context"))
	}
	if err := ctx.Err(); err != nil {
		return nil, wrap("query", err)
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return nil, wrap("query", ErrClosed)
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
	rows, err := d.plan(ctx, sel, params)
	if err != nil {
		return nil, wrap("query", err)
	}
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
