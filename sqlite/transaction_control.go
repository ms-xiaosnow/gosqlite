package sqlite

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

type transactionSavepoint struct {
	name string
	id   uint64
}

// Autocommit reports authoritative connection state, including SQL savepoints
// and native transactions. An outer RELEASE changes it only after commit succeeds.
func (d *DB) Autocommit() bool { d.mu.RLock(); defer d.mu.RUnlock(); return d.tx == nil }

// ensureSnapshotLocked is the single first-access path for SQL and native APIs.
// BEGIN/empty COMMIT/SAVEPOINT and constant SELECT never call it. Database rows
// and catalog-dependent grams13_match calls do; the lease remains pinned until
// the transaction ends, including after ROLLBACK TO its oldest savepoint.
func (t *Tx) ensureSnapshotLocked(ctx context.Context) error {
	if err := t.check(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if t.reader != nil {
		return nil
	}
	d := t.db
	if t.readOnly {
		r, err := d.openReader(ctx)
		if err != nil {
			return err
		}
		t.reader, t.readClose = r, r.Close
	} else {
		opts := d.pagerOptions()
		opts.Create = false
		var p *pager.Tx
		var err error
		if d.session != nil {
			p, err = d.session.Begin(ctx)
		} else {
			p, err = pager.OpenTx(ctx, d.path, opts)
		}
		if err != nil {
			return err
		}
		// Empty logical marks all refer to this first observed state. Only install
		// the snapshot after every mark has passed the shared aggregate quota.
		ids := make([]uint64, len(t.savepoints))
		for i, mark := range t.savepoints {
			ids[i], err = p.Savepoint(ctx, mark.name)
			if err != nil {
				return errors.Join(err, p.Rollback())
			}
		}
		for i := range ids {
			t.savepoints[i].id = ids[i]
		}
		t.reader, t.storage = p, p
	}
	t.initialInfo = fileInfo(t.reader.Header())
	d.info = t.initialInfo
	return nil
}

func selectReadsDatabase(sel *sqlast.Select) bool {
	if sel.From != "" {
		return true
	}
	reads := false
	inspect := func(e sqlast.Expr) {
		if e != nil {
			_ = sqlast.Walk(e, func(e sqlast.Expr) error {
				if call, ok := e.(*sqlast.Call); ok && fold(call.Name) == "grams13_match" {
					reads = true
				}
				return nil
			})
		}
	}
	for _, column := range sel.Columns {
		inspect(column.Expr)
	}
	inspect(sel.Where)
	for _, order := range sel.OrderBy {
		inspect(order.Expr)
	}
	return reads
}

func (d *DB) controlLocked(ctx context.Context, control *sqlast.Transaction) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch control.Action {
	case "BEGIN":
		_, err := d.beginLocked(ctx, TxOptions{Immediate: control.Mode == "IMMEDIATE", Exclusive: control.Mode == "EXCLUSIVE"})
		return err
	case "SAVEPOINT":
		created := d.tx == nil
		if created {
			if _, err := d.beginLocked(ctx, TxOptions{}); err != nil {
				return err
			}
			d.tx.savepointTxn = true
		}
		err := d.tx.savepointLocked(ctx, control.Name)
		if err != nil && created {
			_ = d.tx.rollbackLocked()
		}
		return err
	case "COMMIT", "ROLLBACK":
		if d.tx == nil {
			return fmt.Errorf("no transaction is active")
		}
		if control.Action == "COMMIT" {
			return d.tx.commitLocked(ctx)
		}
		return d.tx.rollbackLocked()
	case "RELEASE", "ROLLBACK TO":
		if d.tx == nil {
			return fmt.Errorf("no such savepoint: %s", control.Name)
		}
		return d.tx.savepointControlLocked(ctx, control)
	default:
		return fmt.Errorf("%w: transaction control %q", ErrUnsupported, control.Action)
	}
}
func (t *Tx) savepointLocked(ctx context.Context, name string) error {
	if err := t.check(); err != nil {
		return err
	}
	if t.activeRows != 0 {
		return fmt.Errorf("%w: close transaction rows before savepoint control", ErrBusy)
	}
	charge := pager.SavepointMetadataBytes(name)
	limit := pager.UndoLimit(t.db.pagerOptions(), t.db.info.PageSize)
	if charge > limit || t.markBytes > limit-charge {
		return fmt.Errorf("%w: aggregate transaction undo memory", pager.ErrLimit)
	}
	var id uint64
	if t.storage != nil {
		var err error
		id, err = t.storage.Savepoint(ctx, name)
		if err != nil {
			return err
		}
	}
	t.savepoints = append(t.savepoints, transactionSavepoint{name: strings.Clone(name), id: id})
	t.markBytes += charge
	return nil
}
func (t *Tx) discardSavepointsLocked(first int) {
	for _, m := range t.savepoints[first:] {
		t.markBytes -= pager.SavepointMetadataBytes(m.name)
	}
	clear(t.savepoints[first:])
	if first == 0 {
		t.savepoints = nil
	} else {
		kept := make([]transactionSavepoint, first)
		copy(kept, t.savepoints[:first])
		t.savepoints = kept
	}
}
func (t *Tx) savepointControlLocked(ctx context.Context, control *sqlast.Transaction) error {
	if err := t.check(); err != nil {
		return err
	}
	name := fold(control.Name)
	i := len(t.savepoints) - 1
	for ; i >= 0; i-- {
		if fold(t.savepoints[i].name) == name {
			break
		}
	}
	if i < 0 {
		return fmt.Errorf("no such savepoint: %s", control.Name)
	}
	if t.activeRows != 0 {
		return fmt.Errorf("%w: close transaction rows before savepoint control", ErrBusy)
	}
	if control.Action == "RELEASE" {
		if i == 0 && t.savepointTxn {
			return t.commitLocked(ctx)
		}
		if t.storage != nil {
			if err := t.storage.Release(ctx, t.savepoints[i].id); err != nil {
				return err
			}
		}
		t.discardSavepointsLocked(i)
		return nil
	}
	if t.storage != nil {
		if err := t.storage.RollbackTo(ctx, t.savepoints[i].id); err != nil {
			return err
		}
	}
	t.discardSavepointsLocked(i + 1)
	t.ownershipValidated = false
	if t.reader != nil {
		t.db.info = fileInfo(t.reader.Header())
	}
	return nil
}
