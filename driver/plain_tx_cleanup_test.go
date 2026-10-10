package driver

import (
	"errors"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"testing"
)

func TestPlainTxCleanupMemoryDriverCodeAndInvalidation(t *testing.T) {
	operation := &sqlite.Error{Code: sqlite.CodeBusy, Op: "commit", Err: sqlite.ErrBusy}
	cleanup := &sqlite.Error{Code: sqlite.CodeIOErr, Op: "rollback", Err: errors.Join(sqlite.ErrPlaintextCleanupPending, sqlite.ErrPoisoned)}
	e := joinTransactionCleanup(operation, cleanup)
	var detail *sqlite.Error
	if !errors.Is(e, sqlite.ErrBusy) || !errors.Is(e, sqlite.ErrPlaintextCleanupPending) || !errors.As(e, &detail) || detail.Code != sqlite.CodeIOErr {
		t.Fatal(e)
	}
	conn := &conn{db: &sqlite.DB{}} // closed with no session, files, Rows or transaction
	if !conn.IsValid() {
		t.Fatal("new model invalid")
	}
	if e = conn.Close(); e != nil || conn.IsValid() {
		t.Fatal(e)
	}
	if e = conn.Close(); e != nil {
		t.Fatal(e)
	}
}

func TestPlainTxCleanupMemoryExecErrorInvalidation(t *testing.T) {
	busy := &sqlite.Error{Code: sqlite.CodeBusy, Op: "commit", Err: sqlite.ErrBusy}
	for _, poison := range []bool{false, true} {
		c := &conn{db: &sqlite.DB{}}
		var cause error = busy
		if poison {
			cause = errors.Join(cause, sqlite.ErrPoisoned, sqlite.ErrPlaintextCleanupPending)
		}
		e := c.executionError(cause)
		var detail *sqlite.Error
		if !errors.Is(e, sqlite.ErrBusy) || !errors.As(e, &detail) || c.IsValid() == poison {
			t.Fatal(e)
		}
		if poison && (detail.Code != sqlite.CodeIOErr || !errors.Is(e, sqlite.ErrPlaintextCleanupPending)) {
			t.Fatal(e)
		}
		if !poison && e != cause {
			t.Fatal("ordinary error changed", e)
		}
	}
}
