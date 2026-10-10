package pager

import (
	"context"
	"encoding/binary"

	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
)

// SnapshotReader is the logical page contract shared by copy/verification APIs.
type SnapshotReader interface {
	Header() Header
	ReadPage(context.Context, uint32) ([]byte, error)
}

// SnapshotFreePages validates freelist membership without obtaining a write
// lease or creating a plaintext transaction/journal.
func SnapshotFreePages(ctx context.Context, r SnapshotReader, max uint32) ([]uint32, error) {
	if max == 0 {
		max = 1 << 20
	}
	h := r.Header()
	p, err := r.ReadPage(ctx, 1)
	if err != nil {
		return nil, err
	}
	defer clear(p)
	count, head := binary.BigEndian.Uint32(p[36:]), binary.BigEndian.Uint32(p[32:])
	if count > max {
		return nil, ErrLimit
	}
	seen := make(map[uint32]struct{})
	add := func(n uint32) error {
		if n < 2 || n > h.PageCount || n == lockPage(h.PageSize) {
			return ErrCorrupt
		}
		if _, ok := seen[n]; ok {
			return ErrCorrupt
		}
		if uint32(len(seen)) >= count {
			return ErrCorrupt
		}
		seen[n] = struct{}{}
		return nil
	}
	for head != 0 {
		if err = add(head); err != nil {
			return nil, err
		}
		p, err := r.ReadPage(ctx, head)
		if err != nil {
			return nil, err
		}
		next, n := binary.BigEndian.Uint32(p), binary.BigEndian.Uint32(p[4:])
		if n > h.UsableSize/4-2 {
			clear(p)
			return nil, ErrCorrupt
		}
		for i := uint32(0); i < n; i++ {
			if err = add(binary.BigEndian.Uint32(p[8+4*i:])); err != nil {
				clear(p)
				return nil, err
			}
		}
		clear(p)
		head = next
	}
	if uint32(len(seen)) != count {
		return nil, ErrCorrupt
	}
	pages := make([]uint32, 0, len(seen))
	for n := range seen {
		pages = append(pages, n)
	}
	return pages, nil
}

type EncryptedArchive struct {
	archive *encstore.Archive
	header  Header
}

func OpenEncryptedArchive(ctx context.Context, path string, options encstore.Options) (*EncryptedArchive, error) {
	a, err := encstore.OpenBackup(ctx, path, options)
	if err != nil {
		return nil, encryptedError(err)
	}
	p, err := a.ReadPage(ctx, 1)
	if err != nil {
		_ = a.Close()
		return nil, err
	}
	g := a.Geometry()
	h, err := ParseHeader(p, int64(g.PageSize)*int64(g.PageCount))
	clear(p)
	if err == nil && (h.PageSize != g.PageSize || h.PageCount != g.PageCount) {
		err = ErrCorrupt
	}
	if err == nil {
		err = writableHeader(h)
	}
	if err != nil {
		_ = a.Close()
		return nil, err
	}
	return &EncryptedArchive{archive: a, header: h}, nil
}
func (a *EncryptedArchive) Header() Header { return a.header }
func (a *EncryptedArchive) ReadPage(c context.Context, n uint32) ([]byte, error) {
	return a.archive.ReadPage(c, n)
}
func (a *EncryptedArchive) Close() error { return a.archive.Close() }
func CreateEncryptedSnapshot(ctx context.Context, destination string, target *encstore.Prepared, r SnapshotReader) (encstore.Info, error) {
	h := r.Header()
	i, e := encstore.CreateFrom(ctx, destination, target, encstore.Geometry{PageSize: h.PageSize, PageCount: h.PageCount}, r)
	return i, encryptedError(e)
}
func (s *EncryptedSession) BackupPrepared(ctx context.Context, destination string, target *encstore.Prepared) (encstore.Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkLocked(); err != nil {
		return encstore.Info{}, err
	}
	if s.active != nil {
		return encstore.Info{}, ErrBusy
	}
	if err := s.store.Verify(ctx); err != nil {
		return encstore.Info{}, encryptedError(err)
	}
	i, e := encstore.WriteBackup(ctx, destination, target, s.store.Geometry(), s.store)
	return i, encryptedError(e)
}
func (s *EncryptedSession) ExportPlaintext(ctx context.Context, destination string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkLocked(); err != nil {
		return err
	}
	if s.active != nil {
		return ErrBusy
	}
	if err := s.store.Verify(ctx); err != nil {
		return encryptedError(err)
	}
	return encryptedError(encstore.ExportPlaintext(ctx, destination, s.store.Geometry(), s.store))
}

// ValidatePlaintextCopySource narrows ordinary read-only alias behavior for
// explicit copy import: the actual held source inode must have one hard link.
// This performs no mutation or native journal recovery.
func (r *Reader) ValidatePlaintextCopySource() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.lease != nil {
		return ErrUnsupported
	}
	if r.file == nil {
		return ErrClosed
	}
	if err := r.checkStable(); err != nil {
		return err
	}
	info, err := r.file.Stat()
	if err != nil {
		return err
	}
	return validateWritableFile(info)
}
