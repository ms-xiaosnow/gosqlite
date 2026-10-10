package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
)

func TestPlainTxCleanupMemoryCommitLogicalInvalidation(t *testing.T) {
	db := &DB{rows: make(map[*Rows]struct{})}
	cleanup := errors.Join(pager.ErrPlaintextCleanupPending, pager.ErrPoisoned)
	calls := 0
	tx := &Tx{db: db, ctx: context.Background(), readClose: func() error { calls++; return cleanup }}
	db.tx = tx
	e := tx.Commit(context.Background())
	var detail *Error
	if !errors.Is(e, ErrPlaintextCleanupPending) || !errors.As(e, &detail) || detail.Code != CodeIOErr || !tx.done || db.tx != nil || calls != 1 {
		t.Fatal(e)
	}
	if e = tx.Rollback(); e != nil || calls != 1 {
		t.Fatal(e)
	}
	if _, e = tx.Query(context.Background(), "SELECT 1"); !errors.Is(e, ErrTxDone) {
		t.Fatal(e)
	}
}
func TestPlainTxCleanupMemoryRollbackPreservesRowsCause(t *testing.T) {
	db := &DB{rows: make(map[*Rows]struct{})}
	child, storage := errors.New("memory rows cleanup"), errors.Join(pager.ErrPlaintextCleanupPending, pager.ErrPoisoned)
	tx := &Tx{db: db, ctx: context.Background(), activeRows: 1, readClose: func() error { return storage }}
	db.tx = tx
	row := &Rows{db: db, tx: tx, release: func() error { return child }}
	db.rows[row] = struct{}{}
	e := tx.Rollback()
	var detail *Error
	if !errors.Is(e, child) || !errors.Is(e, ErrPlaintextCleanupPending) || !errors.As(e, &detail) || detail.Code != CodeIOErr || !row.closed || !errors.Is(row.Err(), ErrTxDone) || !tx.done || db.tx != nil {
		t.Fatal(e)
	}
	if e = tx.Rollback(); e != nil {
		t.Fatal(e)
	}
}

func TestPlainTxCleanupMemoryAutomaticRollbackCodePrecedence(t *testing.T) {
	db := &DB{rows: make(map[*Rows]struct{})}
	tx := &Tx{db: db, ctx: context.Background(), readClose: func() error { return errors.Join(pager.ErrPlaintextCleanupPending, pager.ErrPoisoned) }}
	db.tx = tx
	e := tx.rollbackAfterExecLocked(wrap("commit", ErrBusy))
	var detail *Error
	if !errors.Is(e, ErrBusy) || !errors.Is(e, ErrPlaintextCleanupPending) || !errors.As(e, &detail) || detail.Code != CodeIOErr || !tx.done || db.tx != nil {
		t.Fatal(e)
	}
}
