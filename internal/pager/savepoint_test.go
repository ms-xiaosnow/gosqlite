package pager

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestSavepointRepeatedRollbackAndStatementUndo(t *testing.T) {
	ctx := context.Background()
	tx := openTxTest(t, createEmpty(t, 512), Options{MaxUndoBytes: 1 << 20})
	root, err := tx.Savepoint(ctx, "root")
	if err != nil {
		t.Fatal(err)
	}
	n, err := tx.AllocatePage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p, err := tx.ReadPage(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	p[0] = 7
	if err = tx.WritePage(ctx, n, p); err != nil {
		t.Fatal(err)
	}
	mark, err := tx.Savepoint(ctx, "same")
	if err != nil {
		t.Fatal(err)
	}
	beforeHeader := tx.Header()
	beforeFree := tx.FreePages()
	if err = tx.Statement(ctx, func() error {
		if e := tx.FreePage(ctx, n); e != nil {
			return e
		}
		if e := tx.BumpSchemaCookie(ctx); e != nil {
			return e
		}
		return errors.New("abort statement")
	}); err == nil {
		t.Fatal("statement succeeded")
	}
	if tx.Header() != beforeHeader || !reflect.DeepEqual(tx.FreePages(), beforeFree) {
		t.Fatal("statement changed header/free pages")
	}
	for cycle := 0; cycle < 3; cycle++ {
		younger, e := tx.Savepoint(ctx, "same")
		if e != nil {
			t.Fatal(e)
		}
		if e = tx.FreePage(ctx, n); e != nil {
			t.Fatal(e)
		}
		if e = tx.BumpSchemaCookie(ctx); e != nil {
			t.Fatal(e)
		}
		if e = tx.RollbackTo(ctx, mark); e != nil {
			t.Fatal(e)
		}
		if e = tx.Release(ctx, younger); !errors.Is(e, ErrMisuse) {
			t.Fatalf("younger survived rollback: %v", e)
		}
		got, e := tx.ReadPage(ctx, n)
		if e != nil || got[0] != 7 || tx.Header() != beforeHeader || !reflect.DeepEqual(tx.FreePages(), beforeFree) {
			t.Fatal("repeated rollback changed original", e)
		}
	}
	if err = tx.Release(ctx, mark); err != nil {
		t.Fatal(err)
	}
	if err = tx.RollbackTo(ctx, root); err != nil {
		t.Fatal(err)
	}
	if tx.Header().PageCount != 1 || len(tx.dirty) != 0 {
		t.Fatalf("extent not restored: %+v", tx.Header())
	}
	if err = tx.Release(ctx, root); err != nil {
		t.Fatal(err)
	}
	if tx.undo.bytes != 0 || len(tx.undo.marks) != 0 {
		t.Fatal("retained released undo", tx.undo)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestSavepointAggregateMemoryAndRelease(t *testing.T) {
	ctx := context.Background()
	tx := openTxTest(t, createEmpty(t, 512), Options{MaxDirtyPages: 2, MaxUndoBytes: 6500})
	a, err := tx.Savepoint(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.AllocatePage(ctx); err != nil {
		t.Fatal(err)
	}
	b, err := tx.Savepoint(ctx, "b")
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Statement(ctx, func() error { return tx.BumpSchemaCookie(ctx) }); err != nil {
		t.Fatal(err)
	}
	c, err := tx.Savepoint(ctx, "c")
	if err != nil {
		t.Fatal(err)
	}
	h := tx.Header()
	ran := false
	if err = tx.Statement(ctx, func() error { ran = true; return nil }); !errors.Is(err, ErrLimit) || ran {
		t.Fatalf("aggregate quota did not reject before callback: %v %t", err, ran)
	}
	if tx.Header() != h {
		t.Fatal("quota changed header")
	}
	if err = tx.Release(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err = tx.Statement(ctx, func() error { return tx.BumpSchemaCookie(ctx) }); err != nil {
		t.Fatal("release did not return quota", err)
	}
	if err = tx.RollbackTo(ctx, b); err != nil {
		t.Fatal(err)
	}
	if tx.Header().SchemaCookie != 0 {
		t.Fatal("mark was overwritten")
	}
	if err = tx.Release(ctx, a); err != nil {
		t.Fatal(err)
	}
	if tx.undo.bytes != 0 {
		t.Fatal(tx.undo.bytes)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if tx.undo.bytes != 0 || len(tx.undo.marks) != 0 {
		t.Fatal("closed transaction retains undo")
	}
}

func TestSavepointMetadataBoundAndUndoDefaults(t *testing.T) {
	ctx := context.Background()
	tx := openTxTest(t, createEmpty(t, 512), Options{MaxUndoBytes: 2500})
	a, err := tx.Savepoint(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Savepoint(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Savepoint(ctx, "c"); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	if err = tx.RollbackTo(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Savepoint(ctx, "c"); err != nil {
		t.Fatal(err)
	}
	for _, size := range []uint32{512, 4096, 65536} {
		o := Options{MaxDirtyPages: math.MaxUint32, MaxFreelistPages: math.MaxUint32}
		limit := UndoLimit(o, size)
		need := uint64(math.MaxUint32)*(uint64(size)+undoPageEntryBytes+undoFreeEntryBytes) + undoMarkBytes
		if limit < need || limit > 1<<49 {
			t.Fatalf("overflow size%d limit%d need%d", size, limit, need)
		}
	}
	if err = ValidateOptions(Options{MaxUndoBytes: -1}); err == nil {
		t.Fatal("negative limit accepted")
	}
}
