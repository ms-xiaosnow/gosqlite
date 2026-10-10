package sqlite

import (
	"context"
	"errors"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// TxOptions chooses read-only, immediate-reserved, or exclusive locking. The
// default is a genuinely deferred transaction: its first database access pins
// the snapshot. Immediate and Exclusive are mutually exclusive.
type TxOptions struct{ ReadOnly, Immediate, Exclusive bool }

// Tx owns a logical transaction and, after its first database access, one pinned
// rollback/WAL/encrypted snapshot. Close its Rows before writes, savepoint
// controls or Commit. Full Rollback invalidates Rows and is idempotent. Failed
// statements preserve earlier work. Native Commit returning ErrBusy is retryable.
// Nested BEGIN is rejected; SAVEPOINT controls are accepted through Exec.
type Tx struct {
	db                 *DB
	ctx                context.Context
	reader             btree.PageReader
	storage            *pager.Tx
	readClose          func() error
	activeRows         int
	ownershipValidated bool
	initialInfo        FileInfo
	readOnly           bool
	savepointTxn       bool
	savepoints         []transactionSavepoint
	markBytes          uint64
	done               bool
}

// Result describes the last successful statement; DDL reports zero changes.
type Result struct{ affected, lastID int64 }

func (r Result) RowsAffected() (int64, error) { return r.affected, nil }
func (r Result) LastInsertId() (int64, error) { return r.lastID, nil }
func (r Result) LastInsertID() (int64, error) { return r.lastID, nil }

func (d *DB) Begin(ctx context.Context, options ...TxOptions) (*Tx, error) {
	if err := checkContext(ctx); err != nil {
		return nil, wrap("begin", err)
	}
	if len(options) > 1 {
		return nil, wrap("begin", fmt.Errorf("at most one TxOptions is accepted"))
	}
	var opts TxOptions
	if len(options) == 1 {
		opts = options[0]
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	tx, err := d.beginLocked(ctx, opts)
	return tx, wrap("begin", err)
}
func (d *DB) beginLocked(ctx context.Context, opts TxOptions) (*Tx, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if d.closed {
		return nil, ErrClosed
	}
	if d.tx != nil {
		return nil, fmt.Errorf("cannot begin a transaction within a transaction")
	}
	if opts.Immediate && opts.Exclusive {
		return nil, fmt.Errorf("Immediate and Exclusive are mutually exclusive")
	}
	if (d.opts.ReadOnly || opts.ReadOnly) && (opts.Immediate || opts.Exclusive) {
		return nil, ErrReadOnly
	}
	// Session readers own the sole local lease. Preserve the documented safe
	// active-Rows restriction without acquiring a snapshot for deferred BEGIN.
	if d.session != nil && len(d.rows) != 0 {
		return nil, ErrBusy
	}
	tx := &Tx{db: d, ctx: ctx, readOnly: d.opts.ReadOnly || opts.ReadOnly, initialInfo: d.info}
	if opts.Immediate || opts.Exclusive {
		if session, ok := d.session.(*pager.EncryptedSession); ok {
			p, err := session.BeginImmediate(ctx)
			if err != nil {
				return nil, err
			}
			tx.reader, tx.storage = p, p
			tx.initialInfo = fileInfo(p.Header())
			d.info = tx.initialInfo
		}
		if err := tx.ensureSnapshotLocked(ctx); err != nil {
			return nil, err
		}
		var err error
		if opts.Exclusive {
			err = tx.storage.Exclusive(ctx)
		} else {
			err = tx.storage.PromoteWrite(ctx)
		}
		if err != nil {
			return nil, errors.Join(err, tx.storage.Rollback())
		}
	}
	d.tx = tx
	return tx, nil
}

func (t *Tx) check() error {
	if t.db.closed {
		return ErrClosed
	}
	if t.done {
		return ErrTxDone
	}
	return t.ctx.Err()
}
func (t *Tx) Query(ctx context.Context, query string, args ...any) (*Rows, error) {
	if err := checkContext(ctx); err != nil {
		return nil, wrap("query", err)
	}
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	return t.queryLocked(ctx, query, args)
}
func (t *Tx) queryLocked(ctx context.Context, query string, args []any) (*Rows, error) {
	if err := t.check(); err != nil {
		return nil, wrap("query", err)
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
	if selectReadsDatabase(sel) {
		if err := t.ensureSnapshotLocked(ctx); err != nil {
			return nil, wrap("query", err)
		}
	}
	rows, err := t.db.plan(ctx, t.reader, sel, params)
	if err != nil {
		return nil, wrap("query", err)
	}
	rows.tx = t
	t.activeRows++
	t.db.rows[rows] = struct{}{}
	return rows, nil
}
func (t *Tx) Commit(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return wrap("commit", err)
	}
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	return wrap("commit", t.commitLocked(ctx))
}
func (t *Tx) commitLocked(ctx context.Context) error {
	if err := t.check(); err != nil {
		return err
	}
	if t.activeRows != 0 {
		return fmt.Errorf("%w: close transaction rows before commit", ErrBusy)
	}
	var err error
	if t.storage != nil {
		err = t.storage.Commit(ctx)
	} else if t.readClose != nil {
		err = t.readClose()
	}
	if err != nil {
		return err
	}
	if t.reader != nil {
		t.db.info = fileInfo(t.reader.Header())
	}
	t.savepoints = nil
	t.markBytes = 0
	t.done = true
	if t.db.tx == t {
		t.db.tx = nil
	}
	return nil
}
func (t *Tx) Rollback() error {
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	return wrap("rollback", t.rollbackLocked())
}
func (t *Tx) rollbackLocked() error {
	if t.done {
		return nil
	}
	for r := range t.db.rows {
		if r.tx == t {
			r.mu.Lock()
			r.err = wrap("next", ErrTxDone)
			_ = r.close()
			r.mu.Unlock()
		}
	}
	var err error
	if t.storage != nil {
		err = t.storage.Rollback()
	} else if t.readClose != nil {
		err = t.readClose()
	}
	t.savepoints = nil
	t.markBytes = 0
	t.done = true
	t.db.info = t.initialInfo
	if t.db.tx == t {
		t.db.tx = nil
	}
	return err
}
func (t *Tx) Close() error { return t.Rollback() }

// Exec executes one DDL, DML, or transaction-control statement. Outside an
// explicit transaction a statement is committed atomically before returning.
func (d *DB) Exec(ctx context.Context, query string, args ...any) (Result, error) {
	if err := checkContext(ctx); err != nil {
		return Result{}, wrap("exec", err)
	}
	stmt, err := sqlast.Parse(query)
	if err != nil {
		return Result{}, wrap("exec", err)
	}
	params, err := bind(stmt, args)
	if err != nil {
		return Result{}, wrap("bind", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return Result{}, wrap("exec", ErrClosed)
	}
	if control, ok := stmt.(*sqlast.Transaction); ok {
		return Result{}, wrap("exec", d.controlLocked(ctx, control))
	}
	if d.tx != nil {
		return d.tx.execLocked(ctx, stmt, params)
	}
	if d.opts.ReadOnly {
		return Result{}, wrap("exec", ErrReadOnly)
	}
	t, err := d.beginLocked(ctx, TxOptions{})
	if err != nil {
		return Result{}, wrap("exec", err)
	}
	result, err := t.execLocked(ctx, stmt, params)
	if err == nil {
		err = wrap("commit", t.commitLocked(ctx))
	}
	if err != nil {
		err = errors.Join(err, t.rollbackLocked())
		return Result{}, err
	}
	return result, nil
}
func (t *Tx) Exec(ctx context.Context, query string, args ...any) (Result, error) {
	if err := checkContext(ctx); err != nil {
		return Result{}, wrap("exec", err)
	}
	stmt, err := sqlast.Parse(query)
	if err != nil {
		return Result{}, wrap("exec", err)
	}
	params, err := bind(stmt, args)
	if err != nil {
		return Result{}, wrap("bind", err)
	}
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	return t.execLocked(ctx, stmt, params)
}
func (t *Tx) execLocked(ctx context.Context, stmt sqlast.Statement, params []any) (Result, error) {
	if err := t.check(); err != nil {
		return Result{}, wrap("exec", err)
	}
	if control, ok := stmt.(*sqlast.Transaction); ok {
		return Result{}, wrap("exec", t.db.controlLocked(ctx, control))
	}
	if t.readOnly {
		return Result{}, wrap("exec", ErrReadOnly)
	}
	if t.activeRows != 0 {
		return Result{}, wrap("exec", fmt.Errorf("%w: close transaction rows before executing a write", ErrBusy))
	}
	if err := t.ensureSnapshotLocked(ctx); err != nil {
		return Result{}, wrap("exec", err)
	}
	var result Result
	err := t.storage.Statement(ctx, func() error {
		catalog, err := t.validateCatalog(ctx)
		if err != nil {
			return err
		}
		if err = t.checkWriteOwnership(ctx, catalog); err != nil {
			return err
		}
		if err = t.storage.PromoteWrite(ctx); err != nil {
			return err
		}
		beforeSchema := t.reader.Header().SchemaCookie
		switch s := stmt.(type) {
		case *sqlast.CreateTable:
			err = t.createTable(ctx, catalog, s)
		case *sqlast.CreateIndex:
			if s.Method != "" {
				err = t.createSQLMethodIndex(ctx, catalog, s)
			} else {
				err = t.createIndex(ctx, catalog, s)
			}
		case *sqlast.Reindex:
			err = t.reindexExtension(ctx, catalog, s.Name)
		case *sqlast.Insert:
			result, err = t.insert(ctx, catalog, s, params)
		case *sqlast.Update:
			result, err = t.update(ctx, catalog, s, params)
		case *sqlast.Delete:
			result, err = t.delete(ctx, catalog, s, params)
		default:
			err = fmt.Errorf("%w: Exec requires CREATE, INSERT, UPDATE or DELETE", ErrUnsupported)
		}
		if err != nil {
			return err
		}
		return t.refreshExtensionSchemaCookies(ctx, catalog, beforeSchema)
	})
	if err != nil {
		return Result{}, wrap("exec", err)
	}
	return result, nil
}
