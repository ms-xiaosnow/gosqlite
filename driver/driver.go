// Package driver registers the "gosqlite" database/sql driver. Plain paths and
// file: URIs open writable databases, creating them when needed. Use mode=ro for
// read-only access or mode=rw to require an existing database. SQL transactions
// must use a pinned sql.Conn or sql.Tx; pooled connections are reset before reuse.
package driver

import (
	"context"
	"database/sql"
	sqldriver "database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func init() { sql.Register("gosqlite", &Driver{}) }

// Driver is the registered database/sql adapter. Use NewConnector to configure
// options without placing configuration or secrets in a DSN.
type Driver struct{}

func (*Driver) Open(dsn string) (sqldriver.Conn, error) {
	c, err := NewConnector(dsn, sqlite.Options{})
	if err != nil {
		return nil, err
	}
	return c.Connect(context.Background())
}
func (*Driver) OpenConnector(dsn string) (sqldriver.Connector, error) {
	return NewConnector(dsn, sqlite.Options{})
}

// Connector creates independently owned connections. WALExclusive users must
// set sql.DB.SetMaxOpenConns(1): a second connection cannot share its file lock.
type Connector struct {
	path string
	opts sqlite.Options
}

// NewConnector accepts plain filesystem paths or file:path?mode=ro|rw|rwc.
// The default mode is rwc. URI modes only restrict supplied options: mode=ro
// forces ReadOnly, mode=rw forces NoCreate, and neither rw nor rwc overrides a
// ReadOnly or NoCreate option. Authorities, fragments, credentials and unknown
// options fail closed; DSN values are never echoed in validation errors.
func NewConnector(dsn string, opts sqlite.Options) (*Connector, error) {
	path, mode, err := parseDSNMode(dsn)
	if err != nil {
		return nil, err
	}
	if opts.MaxSortBytes < 0 {
		return nil, fmt.Errorf("gosqlite: MaxSortBytes must not be negative")
	}
	if mode == "ro" {
		opts.ReadOnly = true
	}
	if mode == "rw" {
		opts.NoCreate = true
	}
	if opts.Encryption != nil {
		cfg := *opts.Encryption
		opts.Encryption = &cfg
	}
	return &Connector{path: path, opts: opts}, nil
}
func (c *Connector) Driver() sqldriver.Driver { return &Driver{} }
func (c *Connector) Connect(ctx context.Context) (sqldriver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	db, err := sqlite.OpenContext(ctx, c.path, c.opts)
	if err != nil {
		return nil, err
	}
	// The Connector context governs dialing. Once OpenContext succeeds, return
	// its owned connection even if cancellation races with this return. A late
	// check used to discard both the connection and its Close error.
	return &conn{db: db}, nil
}
func parseDSN(dsn string) (string, error) {
	path, _, err := parseDSNMode(dsn)
	return path, err
}
func parseDSNMode(dsn string) (string, string, error) {
	fail := func() (string, string, error) {
		return "", "", fmt.Errorf("gosqlite: unsupported DSN; use a path or file:path?mode=ro, rw, or rwc")
	}
	if dsn == "" || strings.ContainsRune(dsn, 0) || dsn == ":memory:" {
		return fail()
	}
	if !strings.HasPrefix(dsn, "file:") {
		if strings.Contains(dsn, "://") || strings.ContainsAny(dsn, "?#") {
			return fail()
		}
		return dsn, "rwc", nil
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme != "file" || u.User != nil || u.Host != "" || u.Fragment != "" {
		return fail()
	}
	options, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return fail()
	}
	mode := "rwc"
	for key, values := range options {
		if key != "mode" || len(values) != 1 {
			return fail()
		}
		switch values[0] {
		case "ro", "rw", "rwc":
			mode = values[0]
		default:
			return fail()
		}
	}
	path := u.Path
	if u.Opaque != "" {
		path, err = url.PathUnescape(u.Opaque)
		if err != nil {
			return fail()
		}
	}
	if path == "" || path == ":memory:" || strings.ContainsRune(path, 0) {
		return fail()
	}
	return path, mode, nil
}

type conn struct {
	db     *sqlite.DB
	closed atomic.Bool
}

func (c *conn) Close() error {
	c.closed.Store(true)
	return c.db.Close()
}
func (c *conn) IsValid() bool { return !c.closed.Load() }
func (c *conn) ResetSession(ctx context.Context) error {
	if c.closed.Load() {
		return sqldriver.ErrBadConn
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !c.db.Autocommit() {
		// SQL BEGIN or SAVEPOINT on a pinned sql.Conn may be abandoned when returned
		// to the pool. Never expose that transaction to the next borrower.
		if _, err := c.db.Exec(ctx, "ROLLBACK"); err != nil {
			c.Close()
			return sqldriver.ErrBadConn
		}
	}
	return nil
}
func (c *conn) Prepare(query string) (sqldriver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}
func (c *conn) PrepareContext(ctx context.Context, query string) (sqldriver.Stmt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.closed.Load() {
		return nil, apiError("prepare", sqlite.ErrClosed)
	}
	_, err := sqlast.Parse(query)
	if err != nil {
		return nil, apiError("prepare", err)
	}
	return &stmt{conn: c, query: query}, nil
}
func (c *conn) Begin() (sqldriver.Tx, error) {
	return c.BeginTx(context.Background(), sqldriver.TxOptions{})
}
func (c *conn) BeginTx(ctx context.Context, opts sqldriver.TxOptions) (sqldriver.Tx, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Other isolation levels must not silently receive different semantics.
	if opts.Isolation != sqldriver.IsolationLevel(sql.LevelDefault) && opts.Isolation != sqldriver.IsolationLevel(sql.LevelSerializable) {
		return nil, apiError("begin", fmt.Errorf("%w: transaction isolation level %d", sqlite.ErrUnsupported, opts.Isolation))
	}
	t, err := c.db.Begin(ctx, sqlite.TxOptions{ReadOnly: opts.ReadOnly})
	if err != nil {
		return nil, err
	}
	return &transaction{conn: c, inner: t, ctx: ctx}, nil
}
func (c *conn) Ping(ctx context.Context) error { _, err := c.db.Schema(ctx); return err }
func (c *conn) QueryContext(ctx context.Context, query string, args []sqldriver.NamedValue) (sqldriver.Rows, error) {
	values := namedArguments(args)
	r, err := c.db.Query(ctx, query, values...)
	if err != nil {
		return nil, err
	}
	return &rows{inner: r}, nil
}
func (c *conn) ExecContext(ctx context.Context, query string, args []sqldriver.NamedValue) (sqldriver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result, err := c.db.Exec(ctx, query, namedArguments(args)...)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type transaction struct {
	conn  *conn
	inner *sqlite.Tx
	ctx   context.Context
}

func (t *transaction) Commit() error {
	err := t.inner.Commit(t.ctx)
	if err != nil {
		// database/sql cannot retry Commit: it considers this Tx finished even
		// on an error. Release its uncommitted native transaction before reuse.
		if rollbackErr := t.inner.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sqlite.ErrClosed) {
			t.conn.Close()
			err = errors.Join(err, rollbackErr)
		}
	}
	return err
}
func (t *transaction) Rollback() error {
	err := t.inner.Rollback()
	if err != nil {
		t.conn.Close()
	}
	return err
}
func namedArguments(args []sqldriver.NamedValue) []any {
	values := make([]any, len(args))
	for i, arg := range args {
		if arg.Name != "" {
			values[i] = sql.Named(arg.Name, arg.Value)
		} else {
			values[i] = arg.Value
		}
	}
	return values
}

type stmt struct {
	mu     sync.RWMutex
	conn   *conn
	query  string
	closed bool
}

func (s *stmt) Close() error { s.mu.Lock(); defer s.mu.Unlock(); s.closed = true; return nil }

// -1 leaves named-slot and ?NNN gap validation to the native binder.
func (*stmt) NumInput() int { return -1 }
func (s *stmt) Query(args []sqldriver.Value) (sqldriver.Rows, error) {
	named := make([]sqldriver.NamedValue, len(args))
	for i, v := range args {
		named[i] = sqldriver.NamedValue{Ordinal: i + 1, Value: v}
	}
	return s.QueryContext(context.Background(), named)
}
func (s *stmt) QueryContext(ctx context.Context, args []sqldriver.NamedValue) (sqldriver.Rows, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, apiError("statement", sqlite.ErrClosed)
	}
	return s.conn.QueryContext(ctx, s.query, args)
}
func (s *stmt) Exec(args []sqldriver.Value) (sqldriver.Result, error) {
	named := make([]sqldriver.NamedValue, len(args))
	for i, v := range args {
		named[i] = sqldriver.NamedValue{Ordinal: i + 1, Value: v}
	}
	return s.ExecContext(context.Background(), named)
}
func (s *stmt) ExecContext(ctx context.Context, args []sqldriver.NamedValue) (sqldriver.Result, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, apiError("statement", sqlite.ErrClosed)
	}
	return s.conn.ExecContext(ctx, s.query, args)
}

type rows struct{ inner *sqlite.Rows }

func (r *rows) Columns() []string { return r.inner.Columns() }
func (r *rows) Close() error      { return r.inner.Close() }
func (r *rows) Next(dest []sqldriver.Value) error {
	if !r.inner.Next() {
		if err := r.inner.Err(); err != nil {
			return err
		}
		return io.EOF
	}
	values := r.inner.Values()
	if len(dest) != len(values) {
		return fmt.Errorf("gosqlite: result column count mismatch")
	}
	for i, v := range values {
		dest[i] = v
	}
	return nil
}

var (
	_ sqldriver.DriverContext      = (*Driver)(nil)
	_ sqldriver.Connector          = (*Connector)(nil)
	_ sqldriver.Conn               = (*conn)(nil)
	_ sqldriver.QueryerContext     = (*conn)(nil)
	_ sqldriver.ExecerContext      = (*conn)(nil)
	_ sqldriver.ConnPrepareContext = (*conn)(nil)
	_ sqldriver.ConnBeginTx        = (*conn)(nil)
	_ sqldriver.Pinger             = (*conn)(nil)
	_ sqldriver.SessionResetter    = (*conn)(nil)
	_ sqldriver.Validator          = (*conn)(nil)
	_ sqldriver.Tx                 = (*transaction)(nil)
	_ sqldriver.StmtQueryContext   = (*stmt)(nil)
	_ sqldriver.StmtExecContext    = (*stmt)(nil)
	_ sqldriver.Rows               = (*rows)(nil)
)

func apiError(op string, err error) error {
	code := sqlite.CodeError
	var unsupported *sqlast.ErrUnsupported
	if errors.As(err, &unsupported) {
		err = fmt.Errorf("%w: %w", sqlite.ErrUnsupported, err)
	}
	if errors.Is(err, sqlite.ErrReadOnly) {
		code = sqlite.CodeReadOnly
	}
	if errors.Is(err, sqlite.ErrClosed) {
		code = sqlite.CodeMisuse
	}
	return &sqlite.Error{Code: code, Op: op, Err: err}
}
