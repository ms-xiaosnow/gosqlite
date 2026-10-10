// Package driver registers the "gosqlite" database/sql driver. Connections are
// strictly read-only; transaction and Exec APIs return explicit errors.
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

// Connector creates independently owned read-only connections.
type Connector struct {
	path string
	opts sqlite.Options
}

// NewConnector accepts plain filesystem paths or file:path?mode=ro. URI
// authorities, fragments, credentials, other modes and unknown options fail
// closed; DSN values are never echoed in validation errors.
func NewConnector(dsn string, opts sqlite.Options) (*Connector, error) {
	path, err := parseDSN(dsn)
	if err != nil {
		return nil, err
	}
	if opts.MaxSortBytes < 0 {
		return nil, fmt.Errorf("gosqlite: MaxSortBytes must not be negative")
	}
	return &Connector{path: path, opts: opts}, nil
}
func (c *Connector) Driver() sqldriver.Driver { return &Driver{} }
func (c *Connector) Connect(ctx context.Context) (sqldriver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	db, err := sqlite.Open(c.path, c.opts)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		db.Close()
		return nil, err
	}
	return &conn{db: db}, nil
}
func parseDSN(dsn string) (string, error) {
	fail := func() (string, error) {
		return "", fmt.Errorf("gosqlite: unsupported DSN; use a path or file:path?mode=ro")
	}
	if dsn == "" || strings.ContainsRune(dsn, 0) || dsn == ":memory:" {
		return fail()
	}
	if !strings.HasPrefix(dsn, "file:") {
		if strings.Contains(dsn, "://") || strings.ContainsAny(dsn, "?#") {
			return fail()
		}
		return dsn, nil
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme != "file" || u.User != nil || u.Host != "" || u.Fragment != "" {
		return fail()
	}
	options, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return fail()
	}
	for key, values := range options {
		if key != "mode" || len(values) != 1 || values[0] != "ro" {
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
	return path, nil
}

type conn struct{ db *sqlite.DB }

func (c *conn) Close() error { return c.db.Close() }
func (c *conn) Prepare(query string) (sqldriver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}
func (c *conn) PrepareContext(ctx context.Context, query string) (sqldriver.Stmt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ast, err := sqlast.Parse(query)
	if err != nil {
		return nil, apiError("prepare", err)
	}
	if _, ok := ast.(*sqlast.Select); !ok {
		return nil, apiError("prepare", sqlite.ErrReadOnly)
	}
	return &stmt{conn: c, query: query}, nil
}
func (*conn) Begin() (sqldriver.Tx, error) {
	return nil, apiError("begin", fmt.Errorf("%w: transactions", sqlite.ErrUnsupported))
}
func (*conn) BeginTx(ctx context.Context, _ sqldriver.TxOptions) (sqldriver.Tx, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, apiError("begin", fmt.Errorf("%w: transactions", sqlite.ErrUnsupported))
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
func (*conn) ExecContext(ctx context.Context, _ string, _ []sqldriver.NamedValue) (sqldriver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, apiError("exec", sqlite.ErrReadOnly)
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
	return nil, apiError("exec", sqlite.ErrReadOnly)
}
func (s *stmt) ExecContext(ctx context.Context, _ []sqldriver.NamedValue) (sqldriver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, apiError("exec", sqlite.ErrReadOnly)
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
