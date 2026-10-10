package pager

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
)

// EncryptedSession uses immutable authenticated full snapshots. Writable
// connections retain an exclusive lease on a separate stable lock inode;
// read-only connections share that inode. One local Tx/Reader lease is allowed.
type EncryptedSession struct {
	mu       sync.Mutex
	store    *encstore.Store
	header   Header
	opts     Options
	readOnly bool
	active   *Tx
	poisoned error
}

func encryptedError(err error) error {
	switch {
	case errors.Is(err, encstore.ErrBusy):
		return errors.Join(ErrBusy, err)
	case errors.Is(err, encstore.ErrChanged):
		return errors.Join(ErrChanged, err)
	case errors.Is(err, encstore.ErrUnsupported):
		return errors.Join(ErrUnsupported, err)
	case errors.Is(err, encstore.ErrUncertain):
		return errors.Join(ErrPoisoned, ErrCommitUncertain, err)
	case errors.Is(err, encryption.ErrLimit):
		return errors.Join(ErrLimit, err)
	default:
		return err
	}
}
func OpenEncryptedSession(ctx context.Context, path string, opts Options, cfg encstore.Options, readOnly bool) (*EncryptedSession, error) {
	if opts.KeepWALOnClose || opts.WALPowersafeOverwrite {
		return nil, ErrUnsupported
	}
	opts, err := normalizedOptions(opts)
	if err != nil {
		return nil, err
	}
	st, err := encstore.Open(ctx, path, cfg, readOnly, opts.Create, emptyDatabasePage(opts.PageSize))
	if err != nil {
		return nil, encryptedError(err)
	}
	p, err := st.ReadPage(ctx, 1)
	if err != nil {
		_ = st.Close()
		return nil, encryptedError(err)
	}
	g := st.Geometry()
	h, err := ParseHeader(p, int64(g.PageSize)*int64(g.PageCount))
	clear(p)
	if err == nil && (h.PageCount != g.PageCount || h.PageSize != g.PageSize) {
		err = ErrCorrupt
	}
	if err == nil {
		err = writableHeader(h)
	}
	if err != nil {
		_ = st.Close()
		return nil, err
	}
	return &EncryptedSession{store: st, header: h, opts: opts, readOnly: readOnly}, nil
}
func (s *EncryptedSession) Header() Header { s.mu.Lock(); defer s.mu.Unlock(); return s.header }
func (s *EncryptedSession) DatabaseID() [16]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil {
		return [16]byte{}
	}
	return s.store.DatabaseID()
}
func (s *EncryptedSession) checkLocked() error {
	if s.poisoned != nil {
		return s.poisoned
	}
	if s.store == nil {
		return ErrClosed
	}
	return encryptedError(s.store.Check())
}
func (s *EncryptedSession) checkStable() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checkLocked()
}
func (s *EncryptedSession) Begin(ctx context.Context) (*Tx, error) { return s.begin(ctx, false) }
func (s *EncryptedSession) Reader(ctx context.Context) (*Reader, error) {
	t, e := s.begin(ctx, true)
	if e != nil {
		return nil, e
	}
	return &Reader{lease: t}, nil
}
func (s *EncryptedSession) begin(ctx context.Context, readOnly bool) (*Tx, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	if err := s.checkLocked(); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	if s.active != nil {
		s.mu.Unlock()
		return nil, ErrBusy
	}
	if s.readOnly && !readOnly {
		s.mu.Unlock()
		return nil, ErrMisuse
	}
	p, err := s.store.ReadPage(ctx, 1)
	if err != nil {
		s.mu.Unlock()
		return nil, encryptedError(err)
	}
	t := &Tx{encrypted: s, header: s.header, originalPages: s.header.PageCount, opts: s.opts, readOnly: readOnly, reserved: !readOnly, dirty: make(map[uint32][]byte), free: make(map[uint32]struct{})}
	copy(t.originalHeader[:], p[:100])
	clear(p)
	s.active = t
	s.mu.Unlock()
	if !readOnly {
		if err = t.loadFreelist(ctx); err != nil {
			_ = t.Rollback()
			return nil, err
		}
	}
	return t, nil
}
func (s *EncryptedSession) readPage(ctx context.Context, n uint32) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkLocked(); err != nil {
		return nil, err
	}
	p, err := s.store.ReadPage(ctx, n)
	return p, encryptedError(err)
}
func (s *EncryptedSession) release(t *Tx) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == t {
		s.active = nil
	}
}
func (t *Tx) clearEncrypted() {
	for _, p := range t.dirty {
		clear(p)
	}
	t.dirty = nil
	t.free = nil
	t.encrypted = nil
}
func (t *Tx) commitEncryptedLocked(ctx context.Context) error {
	s := t.encrypted
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkLocked(); err != nil {
		return err
	}
	if s.active != t {
		return ErrMisuse
	}
	if len(t.dirty) == 0 {
		s.active = nil
		t.clearEncrypted()
		return nil
	}
	if t.readOnly {
		return ErrMisuse
	}
	first, ok := t.dirty[1]
	if !ok {
		return ErrMisuse
	}
	p1 := append([]byte(nil), first...)
	defer clear(p1)
	h := t.header
	h.ChangeCounter++
	h.VersionValidFor = h.ChangeCounter
	binary.BigEndian.PutUint32(p1[24:], h.ChangeCounter)
	binary.BigEndian.PutUint32(p1[28:], h.PageCount)
	binary.BigEndian.PutUint32(p1[92:], h.VersionValidFor)
	src := encstore.SourceFunc(func(c context.Context, n uint32) ([]byte, error) {
		if n == 1 {
			return append([]byte(nil), p1...), nil
		}
		if p, ok := t.dirty[n]; ok {
			return append([]byte(nil), p...), nil
		}
		return s.store.ReadPage(c, n)
	})
	err := encryptedError(s.store.Publish(ctx, encstore.Geometry{PageSize: h.PageSize, PageCount: h.PageCount}, src, nil))
	if err != nil {
		if errors.Is(err, ErrCommitUncertain) {
			s.poisoned = err
			t.poisoned = err
			s.active = nil
			t.clearEncrypted()
		}
		return err
	}
	s.header = h
	t.header = h
	s.active = nil
	t.clearEncrypted()
	return nil
}
func (s *EncryptedSession) Checkpoint(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkLocked(); err != nil {
		return err
	}
	if s.active != nil {
		return ErrBusy
	}
	return ctx.Err()
}
func (s *EncryptedSession) Verify(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkLocked(); err != nil {
		return err
	}
	if s.active != nil {
		return ErrBusy
	}
	return encryptedError(s.store.Verify(ctx))
}
func (s *EncryptedSession) RekeyPrepared(ctx context.Context, target *encstore.Prepared) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkLocked(); err != nil {
		return err
	}
	if s.active != nil {
		return ErrBusy
	}
	if s.readOnly {
		return ErrMisuse
	}
	err := encryptedError(s.store.Publish(ctx, s.store.Geometry(), s.store, target))
	if errors.Is(err, ErrCommitUncertain) {
		s.poisoned = err
	}
	return err
}
func (s *EncryptedSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil {
		return ErrBusy
	}
	if s.store == nil {
		return s.poisoned
	}
	err := encryptedError(s.store.Close())
	s.store = nil
	return errors.Join(s.poisoned, err)
}

func (s *EncryptedSession) EncryptionInfo() encstore.Info {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil {
		return encstore.Info{}
	}
	return s.store.Info()
}
