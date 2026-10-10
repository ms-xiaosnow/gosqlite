package pager

import (
	"context"
	"errors"

	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
)

// encryptedSnapshotView borrows an operation's immutable selection while the
// session mutex and local operation ownership are retained. Its page methods
// deliberately do not reenter Session.mu or call a Tx.
type encryptedSnapshotView struct {
	store  *encstore.Store
	header Header
}

func (r encryptedSnapshotView) Header() Header { return r.header }
func (r encryptedSnapshotView) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	p, e := r.store.ReadPage(ctx, n)
	return p, encryptedError(e)
}

type SnapshotValidator func(context.Context, SnapshotReader) error

func (s *EncryptedSession) selectedHeader(ctx context.Context) (Header, error) {
	p, e := s.store.ReadPage(ctx, 1)
	if e != nil {
		return Header{}, encryptedError(e)
	}
	defer clear(p)
	g := s.store.Geometry()
	h, e := ParseHeader(p, int64(g.PageSize)*int64(g.PageCount))
	if e == nil && (h.PageSize != g.PageSize || h.PageCount != g.PageCount) {
		e = ErrCorrupt
	}
	if e == nil {
		e = writableHeader(h)
	}
	return h, e
}

// operationLocked captures one authority for budget, physical/logical
// validation, output pages and returned metadata. Maintenance takes writer
// ownership before capture; rekey additionally requires global quiescence.
func (s *EncryptedSession) operationLocked(ctx context.Context, write, rekey bool, validate SnapshotValidator, action func(encryptedSnapshotView) error) (err error) {
	if err = s.checkLocked(); err != nil {
		return err
	}
	if s.active != nil {
		return ErrBusy
	}
	if write && s.readOnly {
		return ErrMisuse
	}
	if write {
		if err = encryptedError(s.store.BeginMaintenance(ctx, rekey)); err != nil {
			return err
		}
		before := s.store.Authority()
		defer func() {
			err = errors.Join(err, encryptedError(s.store.EndMaintenance(s.store.Authority() != before)))
			if errors.Is(err, ErrPoisoned) {
				s.poisoned = err
			}
		}()
	} else if err = encryptedError(s.store.Refresh(ctx)); err != nil {
		return err
	}
	if err = encryptedError(s.store.CopyBudget()); err != nil {
		return err
	}
	if err = encryptedError(s.store.Verify(ctx)); err != nil {
		return err
	}
	h, err := s.selectedHeader(ctx)
	if err != nil {
		return err
	}
	s.header = h
	view := encryptedSnapshotView{s.store, h}
	if validate != nil {
		if err = validate(ctx, view); err != nil {
			return err
		}
	}
	return action(view)
}
