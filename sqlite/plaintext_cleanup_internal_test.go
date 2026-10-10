package sqlite

import (
	"errors"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"os"
	"testing"
)

func TestPlaintextCleanupMemoryPublicClassification(t *testing.T) {
	original := errors.New("memory cause")
	e := wrap("close", errors.Join(pager.ErrPlaintextCleanupPending, pager.ErrPoisoned, ErrBusy, original))
	var detail *Error
	if !errors.Is(e, ErrPlaintextCleanupPending) || !errors.Is(e, ErrPoisoned) || !errors.Is(e, original) || !errors.As(e, &detail) || detail.Code != CodeIOErr {
		t.Fatal(e)
	}
	if e = wrap("sentinel", ErrPlaintextCleanupPending); errors.Is(e, ErrPoisoned) {
		t.Fatal("bare sentinel inferred poison")
	}
	if e = RetryPlaintextCleanup(nil); e != nil {
		t.Fatal(e)
	}
	if e = RetryPlaintextCleanup(errors.New("ordinary")); e != nil {
		t.Fatal(e)
	}
}
func TestPlaintextCleanupMemoryRowsFailPreservesCleanup(t *testing.T) {
	operation, cleanup := errors.New("memory operation"), errors.New("memory cleanup")
	db := &DB{rows: make(map[*Rows]struct{})}
	calls := 0
	r := &Rows{db: db, release: func() error { calls++; return cleanup }}
	db.rows[r] = struct{}{}
	if r.fail(operation) || !errors.Is(r.Err(), operation) || !errors.Is(r.Err(), cleanup) || calls != 1 || !r.closed || r.release != nil {
		t.Fatal(r.Err())
	}
	if e := r.Close(); e != nil || calls != 1 {
		t.Fatal(e)
	}
}

func TestPlaintextCleanupMemoryComposedCodePrecedence(t *testing.T) {
	operation := wrap("original operation", ErrBusy)
	cleanup := wrap("cleanup", errors.Join(pager.ErrPlaintextCleanupPending, pager.ErrPoisoned))
	for _, op := range []string{"schema", "encrypted import"} {
		e := wrapCleanup(op, operation, cleanup)
		var detail *Error
		if !errors.Is(e, ErrBusy) || !errors.Is(e, ErrPlaintextCleanupPending) || !errors.Is(e, ErrPoisoned) || !errors.As(e, &detail) || detail.Code != CodeIOErr {
			t.Fatal(op, e, detail)
		}
		if got := wrapCleanup(op, operation, nil); got != operation {
			t.Fatal("nil cleanup changed operation")
		}
	}
	db := &DB{rows: make(map[*Rows]struct{})}
	r := &Rows{db: db, release: func() error { return cleanup }}
	db.rows[r] = struct{}{}
	if r.fail(operation) {
		t.Fatal("Rows failure returned true")
	}
	var detail *Error
	if !errors.Is(r.Err(), ErrBusy) || !errors.Is(r.Err(), ErrPlaintextCleanupPending) || !errors.As(r.Err(), &detail) || detail.Code != CodeIOErr {
		t.Fatal(r.Err(), detail)
	}
}

func TestPlaintextCleanupMemoryPoisonedProbeCannotCreate(t *testing.T) {
	for _, cause := range []error{os.ErrNotExist, errors.Join(os.ErrNotExist, ErrChanged), errors.Join(os.ErrNotExist, errors.New("consumed cleanup diagnostic")), errors.Join(os.ErrNotExist, ErrPoisoned), wrap("open", errors.Join(os.ErrNotExist, ErrPlaintextCleanupPending, ErrPoisoned)), ErrBusy, nil} {
		if plainProbeMissing(cause) {
			t.Fatal("owned/ordinary nonabsence authorized create", cause)
		}
	}
}
