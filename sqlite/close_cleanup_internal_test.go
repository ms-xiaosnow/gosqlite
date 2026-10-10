package sqlite

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
)

// A descriptor-free pageSession tests DB orchestration only. Managed native
// retirement and retained-token laws are tested separately in pager.
type memoryCloseSession struct {
	calls   int
	first   error
	pending bool
}

func (*memoryCloseSession) Header() pager.Header                          { return pager.Header{} }
func (*memoryCloseSession) Begin(context.Context) (*pager.Tx, error)      { panic("unused") }
func (*memoryCloseSession) Reader(context.Context) (*pager.Reader, error) { panic("unused") }
func (*memoryCloseSession) Checkpoint(context.Context) error {
	panic("Close must not checkpoint through DB")
}
func (s *memoryCloseSession) Close() error { return s.CloseWithError(nil) }
func (s *memoryCloseSession) CloseWithError(err error) error {
	s.calls++
	if s.calls == 1 {
		s.first = err
	}
	if s.pending {
		s.pending = false
		return errors.Join(ErrCloseCleanupPending, ErrPoisoned, err)
	}
	return nil
}
func TestCloseCleanupMemoryDBInvalidatesOnceAndRetries(t *testing.T) {
	session := &memoryCloseSession{pending: true}
	d := &DB{session: session, rows: make(map[*Rows]struct{})}
	rowCause, txCause := errors.New("memory row release"), errors.New("memory transaction release")
	rowCalls, txCalls := 0, 0
	r := &Rows{db: d, release: func() error { rowCalls++; return rowCause }}
	d.rows[r] = struct{}{}
	tx := &Tx{db: d, readClose: func() error { txCalls++; return txCause }}
	d.tx = tx
	err := d.Close()
	var detail *Error
	if !errors.Is(err, ErrCloseCleanupPending) || !errors.Is(err, ErrPoisoned) || !errors.As(err, &detail) || detail.Code != CodeIOErr {
		t.Fatal(err)
	}
	if !errors.Is(session.first, rowCause) || !errors.Is(session.first, txCause) {
		t.Fatal("nested release error lost", session.first)
	}
	if rowCalls != 1 || txCalls != 1 || !tx.done || !r.closed || !d.closed || d.tx != nil || len(d.rows) != 0 || !errors.Is(r.Err(), ErrClosed) {
		t.Fatal("logical closure incomplete")
	}
	if _, e := d.Query(context.Background(), "SELECT 1"); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e := d.Close(); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if rowCalls != 1 || txCalls != 1 || session.calls != 25 {
		t.Fatal("logical cleanup repeated or retirement not reached")
	}
}

func TestCloseCleanupMemoryPublicAdmissionBeforePath(t *testing.T) {
	var admissions []*pager.SharedCloseAdmission
	defer func() {
		for _, a := range admissions {
			a.Cancel()
		}
	}()
	for i := 0; i < 1024; i++ {
		a, err := pager.ReserveSharedClose()
		if err != nil {
			t.Fatal("unexpected preexisting admission", i, err)
		}
		admissions = append(admissions, a)
	}
	// The NUL path would fail canonicalization; capacity must reject first.
	db, err := Open("\x00", Options{WALShared: true})
	var detail *Error
	if db != nil || !errors.Is(err, ErrMemoryLimit) || !errors.As(err, &detail) || detail.Code != CodeNoMem {
		t.Fatal("native metadata preceded admission", err)
	}
	if len(PendingCloseCleanups()) != 0 {
		t.Fatal("live admission exposed as pending close")
	}
}
