package sqlite

import (
	"context"
	"errors"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// TxOptions chooses read-only or immediate-reserved locking. The default is
// a deferred read/write transaction; EXCLUSIVE and nested transactions are not supported.
type TxOptions struct{ ReadOnly, Immediate bool }

// Tx owns one SQLite rollback-journal transaction. Close all Rows before an
// Exec or Commit. A failed statement is rolled back without aborting prior
// statements. A Commit returning ErrBusy may be retried. Rollback is idempotent.
type Tx struct {
	db                 *DB
	ctx                context.Context
	reader             btree.PageReader
	storage            *pager.Tx
	readClose          func() error
	activeRows         int
	ownershipValidated bool
	initialInfo        FileInfo
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
	if d.closed {
		return nil, ErrClosed
	}
	if d.tx != nil {
		return nil, fmt.Errorf("cannot begin a transaction within a transaction")
	}
	if opts.ReadOnly && opts.Immediate {
		return nil, fmt.Errorf("read-only transactions cannot reserve a writer lock")
	}
	tx := &Tx{db: d, ctx: ctx}
	if d.opts.ReadOnly || opts.ReadOnly {
		r, err := d.openReader(ctx)
		if err != nil {
			return nil, err
		}
		tx.reader = r
		tx.readClose = r.Close
	} else {
		pagerOptions := d.pagerOptions()
		// Open already created the database if authorized. A later removal
		// must not silently replace that database with an empty one.
		pagerOptions.Create = false
		var p *pager.Tx
		var err error
		if d.session != nil {
			p, err = d.session.Begin(ctx)
		} else {
			p, err = pager.OpenTx(ctx, d.path, pagerOptions)
		}
		if err != nil {
			return nil, err
		}
		tx.reader = p
		tx.storage = p
		if opts.Immediate {
			if err := p.PromoteWrite(ctx); err != nil {
				p.Rollback()
				return nil, err
			}
		}
	}
	d.tx = tx
	tx.initialInfo = fileInfo(tx.reader.Header())
	d.info = tx.initialInfo
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
	} else {
		err = t.readClose()
	}
	if err != nil {
		return err
	}
	t.db.info = fileInfo(t.reader.Header())
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
	} else {
		err = t.readClose()
	}
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
		switch control.Action {
		case "BEGIN":
			if control.Mode == "EXCLUSIVE" {
				err = fmt.Errorf("%w: BEGIN EXCLUSIVE", ErrUnsupported)
			} else {
				_, err = d.beginLocked(ctx, TxOptions{Immediate: control.Mode == "IMMEDIATE"})
			}
		case "COMMIT":
			if d.tx == nil {
				err = fmt.Errorf("no transaction is active")
			} else {
				err = d.tx.commitLocked(ctx)
			}
		case "ROLLBACK":
			if d.tx == nil {
				err = fmt.Errorf("no transaction is active")
			} else {
				err = d.tx.rollbackLocked()
			}
		}
		return Result{}, wrap("exec", err)
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
	if t.storage == nil {
		return Result{}, wrap("exec", ErrReadOnly)
	}
	if t.activeRows != 0 {
		return Result{}, wrap("exec", fmt.Errorf("%w: close transaction rows before executing a write", ErrBusy))
	}
	var result Result
	err := t.storage.Statement(ctx, func() error {
		catalog, err := t.validateCatalog(ctx)
		if err != nil {
			return err
		}
		if !t.ownershipValidated {
			var roots []btree.RootSpec
			for _, tab := range catalog.tables {
				roots = append(roots, btree.RootSpec{Page: tab.root, Kind: btree.TableTree})
				for _, idx := range tab.indexes {
					roots = append(roots, btree.RootSpec{Page: idx.root, Kind: btree.IndexTree, KeyInfo: btree.KeyInfo{Columns: len(idx.columns)}})
				}
			}
			if err = btree.CheckOwnership(ctx, t.reader, roots, t.storage.FreePages()); err != nil {
				return err
			}
			t.ownershipValidated = true
		}
		if err = t.storage.PromoteWrite(ctx); err != nil {
			return err
		}
		switch s := stmt.(type) {
		case *sqlast.CreateTable:
			err = t.createTable(ctx, catalog, s)
		case *sqlast.CreateIndex:
			err = t.createIndex(ctx, catalog, s)
		case *sqlast.Insert:
			result, err = t.insert(ctx, catalog, s, params)
		case *sqlast.Update:
			result, err = t.update(ctx, catalog, s, params)
		case *sqlast.Delete:
			result, err = t.delete(ctx, catalog, s, params)
		default:
			err = fmt.Errorf("%w: Exec requires CREATE, INSERT, UPDATE or DELETE", ErrUnsupported)
		}
		return err
	})
	if err != nil {
		return Result{}, wrap("exec", err)
	}
	return result, nil
}
