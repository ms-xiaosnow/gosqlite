package encstore

import (
	"context"
	"crypto/sha256"
	"github.com/ms-xiaosnow/gosqlite/encryption"
)

func rootExtent(size int64, o Options) error {
	l, e := o.WALLimits.normalized()
	if e != nil {
		return e
	}
	if size < rootSize || uint64(size) > uint64(l.MaxRootBytes) || uint64(size)*4 > l.MaxMemoryBytes {
		return encryption.ErrLimit
	}
	return nil
}

// Root payload magic is interpreted only after complete AEAD authentication.
func rootPayload(raw []byte, m *encryption.MasterKey, o Options) (encryption.Header, []byte, error) {
	var h encryption.Header
	if e := rootExtent(int64(len(raw)), o); e != nil {
		return h, nil, e
	}
	h, e := encryption.DecodeHeader(raw[:encryption.HeaderSize], o.Limits)
	if e != nil {
		return h, nil, e
	}
	if h.Descriptor.Role != encryption.FileRoot {
		return h, nil, encryption.ErrFormat
	}
	r, p, e := encryption.OpenFile(m, h.Identity(), raw[:encryption.HeaderSize], raw[encryption.HeaderSize:], o.Limits)
	if e != nil {
		return h, nil, e
	}
	r.Close()
	return h, p, nil
}
func (s *Store) openSelection(ctx context.Context) error {
	h, p, e := rootPayload(s.rootBytes, s.master, s.opts)
	if e != nil {
		return e
	}
	defer clear(p)
	if len(p) >= 8 && string(p[:8]) == "GSEMNF02" {
		if s.opts.Mode != WAL {
			return ErrUnsupported
		}
		m, e := decodeWALManifest(p, h, s.opts)
		if e != nil {
			return e
		}
		r := m.Base
		obj, e := openObject(ctx, s.basePath(r.FileID), s.master, s.opts.Limits, r.identity(h.Epoch), &r.Descriptor, &r.Hash, &r.PageOneHash, s.ns)
		if e != nil {
			return e
		}
		s.base = obj
		s.wal = &m
		prev := r.Descriptor.PageCount
		for _, ref := range m.Segments {
			seg, e := s.openSegment(ctx, s.master, h.Epoch, s.opts, ref, prev)
			if e != nil {
				return e
			}
			s.segments = append(s.segments, seg)
			prev = ref.Descriptor.PageCount
		}
		s.overlay, s.baseCeiling, e = buildOverlay(m, s.segments)
		if e != nil {
			return e
		}
		page, e := s.ReadPage(ctx, 1)
		if e != nil {
			return e
		}
		valid := sha256.Sum256(page) == m.PageOneHash
		clear(page)
		if !valid {
			return encryption.ErrAuthentication
		}
		return nil
	}
	if s.opts.Mode != Snapshot {
		return ErrUnsupported
	}
	m, e := decodeManifest(p)
	if e != nil {
		return e
	}
	d := h.Descriptor
	d.Role = encryption.FileBase
	d.RecordCount = uint64(d.PageCount) + 2
	if m.BaseBytes != objectSize(Geometry{d.PageSize, d.PageCount}) {
		return encryption.ErrFormat
	}
	expected := encryption.FileIdentity{DatabaseID: h.Epoch.DatabaseID, EpochID: h.Epoch.EpochID, FileID: m.BaseID, Role: encryption.FileBase}
	s.base, e = openObject(ctx, s.basePath(m.BaseID), s.master, s.opts.Limits, expected, &d, &m.BaseHash, &m.PageOneHash, s.ns)
	return e
}
func (s *Store) verifyWAL(ctx context.Context) error {
	h, p, e := rootPayload(s.rootBytes, s.master, s.opts)
	if e != nil {
		return e
	}
	defer clear(p)
	m, e := decodeWALManifest(p, h, s.opts)
	if e != nil {
		return e
	}
	if e = graphBudget(m, s.opts, 1, s.writerIDs*128); e != nil {
		return e
	}
	r := m.Base
	o, e := openObject(ctx, s.basePath(r.FileID), s.master, s.opts.Limits, r.identity(h.Epoch), &r.Descriptor, &r.Hash, &r.PageOneHash, s.ns)
	if e != nil {
		return e
	}
	if e = o.close(); e != nil {
		return e
	}
	previous := r.Descriptor.PageCount
	for _, r := range m.Segments {
		seg, e := s.openSegment(ctx, s.master, h.Epoch, s.opts, r, previous)
		if e != nil {
			return e
		}
		if e = seg.close(); e != nil {
			return e
		}
		previous = r.Descriptor.PageCount
	}
	return nil
}
func (s *Store) Mode() Mode { return s.opts.Mode }
func (s *Store) HeadCommitID() [32]byte {
	if s.wal != nil {
		return s.wal.Head
	}
	return sha256.Sum256(s.rootBytes)
}
