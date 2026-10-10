package pager

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"
)

// This review exercises only in-memory accounting. No synthetic header or
// synthetic freelist in this test is ever attached to a file or published.
func TestReviewSavepointExactAggregateAccountingAndIDOverflow(t *testing.T) {
	const size = uint32(65536)
	tx := &Tx{
		header: Header{PageSize: size},
		dirty:  map[uint32][]byte{1: make([]byte, size), 2: make([]byte, size)},
		free:   map[uint32]struct{}{7: {}, 8: {}, 9: {}},
	}
	name := "bounded-name"
	charge := SavepointMetadataBytes(name) + 2*(uint64(size)+undoPageEntryBytes) + 3*undoFreeEntryBytes
	tx.opts.MaxUndoBytes = int64(2 * charge)
	first, err := tx.newUndoMarkLocked(name)
	if err != nil || first.charge != charge || tx.undo.bytes != charge {
		t.Fatalf("first charge: mark=%+v bytes=%d err=%v", first, tx.undo.bytes, err)
	}
	second, err := tx.newUndoMarkLocked(name)
	if err != nil || second.id != first.id+1 || tx.undo.bytes != 2*charge {
		t.Fatalf("exact limit rejected: bytes=%d err=%v", tx.undo.bytes, err)
	}
	if _, err = tx.newUndoMarkLocked(""); !errors.Is(err, ErrLimit) {
		t.Fatalf("aggregate quota accepted a third mark: %v", err)
	}
	if tx.undo.next != second.id || len(tx.undo.marks) != 2 || tx.undo.bytes != 2*charge {
		t.Fatal("rejected mark changed undo state")
	}
	delete(tx.dirty, 2)
	delete(tx.free, 8)
	if len(first.dirty) != 2 || len(first.free) != 3 {
		t.Fatal("mark maps alias the current maps")
	}
	tx.discardUndoLocked(1)
	if tx.undo.bytes != charge || len(tx.undo.marks) != 1 || cap(tx.undo.marks) != 1 {
		t.Fatalf("release retains excess accounting or backing stack: %+v", tx.undo)
	}
	tx.undo.next = math.MaxUint64
	if _, err = tx.newUndoMarkLocked(""); !errors.Is(err, ErrLimit) {
		t.Fatalf("mark id overflow accepted: %v", err)
	}
	if tx.undo.next != math.MaxUint64 || tx.undo.bytes != charge || len(tx.undo.marks) != 1 {
		t.Fatal("id overflow changed undo state")
	}
	tx.discardUndoLocked(0)
	if tx.undo.marks != nil || tx.undo.bytes != 0 {
		t.Fatal("all released marks remain retained")
	}
}

// The fixture is created normally. All subsequent candidate page/header
// changes stay in memory and the transaction finishes through normal rollback.
func TestReviewSavepointStatementCancellationPanicAndControlPreserveMarks(t *testing.T) {
	ctx := context.Background()
	tx := openTxTest(t, createEmpty(t, 512), Options{MaxUndoBytes: 1 << 20})
	if err := tx.BumpSchemaCookie(ctx); err != nil {
		t.Fatal(err)
	}
	mark, err := tx.Savepoint(ctx, "retained")
	if err != nil {
		t.Fatal(err)
	}
	before := tx.Header()
	page, err := tx.ReadPage(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	charge := tx.undo.bytes
	assertRestored := func() {
		t.Helper()
		got, err := tx.ReadPage(ctx, 1)
		if err != nil || !bytes.Equal(got, page) || tx.Header() != before {
			t.Fatalf("statement failed to restore entry image: %v", err)
		}
		if tx.statement || tx.undo.bytes != charge || len(tx.undo.marks) != 1 || tx.undo.marks[0].id != mark {
			t.Fatalf("statement damaged retained savepoint: %+v", tx.undo)
		}
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	err = tx.Statement(cancelCtx, func() error {
		if err := tx.BumpSchemaCookie(cancelCtx); err != nil {
			return err
		}
		cancel()
		return nil
	})
	cancel()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled callback result: %v", err)
	}
	assertRestored()
	func() {
		defer func() {
			if got := recover(); got != "review panic" {
				t.Fatalf("panic was changed or swallowed: %v", got)
			}
		}()
		_ = tx.Statement(ctx, func() error {
			if err := tx.BumpSchemaCookie(ctx); err != nil {
				t.Fatal(err)
			}
			panic("review panic")
		})
	}()
	assertRestored()
	err = tx.Statement(ctx, func() error {
		if err := tx.BumpSchemaCookie(ctx); err != nil {
			return err
		}
		for _, control := range []func() error{
			func() error { _, err := tx.Savepoint(ctx, "nested"); return err },
			func() error { return tx.Release(ctx, mark) },
			func() error { return tx.RollbackTo(ctx, mark) },
			func() error { return tx.Commit(ctx) },
			tx.Rollback,
		} {
			if err := control(); !errors.Is(err, ErrMisuse) {
				t.Fatalf("statement control was allowed: %v", err)
			}
		}
		return errors.New("review statement abort")
	})
	if err == nil {
		t.Fatal("callback error was lost")
	}
	assertRestored()
	if err := tx.BumpSchemaCookie(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.RollbackTo(ctx, mark); err != nil {
		t.Fatal(err)
	}
	assertRestored()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if tx.undo.bytes != 0 || tx.undo.marks != nil {
		t.Fatal("normal rollback retained undo state")
	}
}
