package encstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

type segment struct {
	file   *managedFile
	info   os.FileInfo
	path   string
	ns     *namespace
	reader *encryption.OpenedFile
	ref    objectRef
	pages  []uint32
}

func (s *segment) close() error {
	if s == nil {
		return nil
	}
	var e error
	if s.reader != nil {
		e = s.reader.Close()
	}
	if s.file != nil {
		e = errors.Join(e, s.file.Close())
	}
	return e
}
func (s *segment) readPage(ctx context.Context, page, ordinal uint32) ([]byte, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if ordinal == 0 || int(ordinal) > len(s.pages) || s.pages[ordinal-1] != page {
		return nil, encryption.ErrIdentity
	}
	if e := s.ns.stable(s.file, s.path, s.info); e != nil {
		return nil, e
	}
	d := s.ref.Descriptor
	b, e := readExact(s.file, int(d.PageSize)+recordOverhead, frameOffset(d.PageSize, d.RecordCount-2, uint64(ordinal)))
	if e != nil {
		return nil, e
	}
	p, e := s.reader.Open(encryption.RecordIdentity{Role: encryption.RecordData, Sequence: uint64(ordinal), PageNumber: page, FrameOrdinal: uint64(ordinal)}, b)
	if e != nil {
		return nil, e
	}
	if page == lockPage(d.PageSize) && !zero(p) {
		clear(p)
		return nil, encryption.ErrAuthentication
	}
	if e = ctx.Err(); e != nil {
		clear(p)
		return nil, e
	}
	return p, nil
}

// verifySegment is a pure ReaderAt seam. Expected reference and predecessor
// are mandatory authenticated authority; plaintext metadata is parsed only
// after OpenFile succeeds. No partial segment or prefix is ever returned.
func verifySegment(ctx context.Context, in io.ReaderAt, extent uint64, master *encryption.MasterKey, epoch encryption.Epoch, o Options, ref objectRef, previous uint32) (*segment, error) {
	l, e := o.WALLimits.normalized()
	if e != nil {
		return nil, e
	}
	d := ref.Descriptor
	if e = d.Validate(o.Limits); e != nil {
		return nil, e
	}
	if d.Role != encryption.FileWAL {
		return nil, encryption.ErrFormat
	}
	frames := d.RecordCount - 2
	if frames > uint64(l.MaxSegmentFrames) || ref.Bytes > l.MaxSegmentBytes || frames*80+uint64(d.PageSize)*4 > l.MaxMemoryBytes {
		return nil, encryption.ErrLimit
	}
	if extent != ref.Bytes || extent != segmentSize(d.PageSize, frames) {
		return nil, encryption.ErrFormat
	}
	metaLen := 128 + 4*frames
	rh := encryption.RecordHeader{Role: encryption.RecordMetadata, PlaintextSize: uint32(metaLen)}
	if e = rh.Validate(o.Limits); e != nil {
		return nil, e
	}
	raw, e := readExact(in, encryption.HeaderSize, 0)
	if e != nil {
		return nil, e
	}
	h, e := encryption.DecodeHeader(raw, o.Limits)
	if e != nil {
		return nil, e
	}
	if h.Descriptor != d || h.Identity() != ref.identity(epoch) {
		return nil, encryption.ErrAuthentication
	}
	meta, e := readExact(in, int(metaLen)+recordOverhead, encryption.HeaderSize)
	if e != nil {
		return nil, e
	}
	reader, plain, e := encryption.OpenFile(master, ref.identity(epoch), raw, meta, o.Limits)
	if e != nil {
		return nil, e
	}
	s := &segment{reader: reader, ref: ref}
	ok := false
	defer func() {
		if !ok {
			s.close()
		}
	}()
	metaHash := sha256.Sum256(plain)
	s.pages, e = parseSegmentMetadata(plain, ref, previous)
	clear(plain)
	if e != nil {
		return nil, e
	}
	digest := sha256.New()
	digest.Write(raw)
	digest.Write(meta)
	for i, n := range s.pages {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		ordinal := uint64(i + 1)
		b, e := readExact(in, int(d.PageSize)+recordOverhead, frameOffset(d.PageSize, frames, ordinal))
		if e != nil {
			return nil, e
		}
		digest.Write(b)
		p, e := reader.Open(encryption.RecordIdentity{Role: encryption.RecordData, Sequence: ordinal, PageNumber: n, FrameOrdinal: ordinal}, b)
		if e != nil {
			return nil, e
		}
		valid := (n != 1 || sha256.Sum256(p) == ref.PageOneHash) && (n != lockPage(d.PageSize) || zero(p))
		clear(p)
		if !valid {
			return nil, encryption.ErrAuthentication
		}
	}
	var prefix [32]byte
	copy(prefix[:], digest.Sum(nil))
	foot, e := readExact(in, walFooterSize+recordOverhead, int64(extent)-walFooterSize-recordOverhead)
	if e != nil {
		return nil, e
	}
	p, e := reader.Open(encryption.RecordIdentity{Role: encryption.RecordFooter, Sequence: frames + 1}, foot)
	if e != nil {
		return nil, e
	}
	valid := bytes.Equal(p, segmentFooter(ref, previous, prefix, metaHash))
	clear(p)
	if !valid {
		return nil, encryption.ErrAuthentication
	}
	digest.Write(foot)
	if !bytes.Equal(digest.Sum(nil), ref.Hash[:]) {
		return nil, encryption.ErrAuthentication
	}
	ok = true
	return s, nil
}
func (s *Store) walPath(id [32]byte) string { return s.path + ".gse-" + idString(id) + ".wal" }
func (s *Store) openSegment(ctx context.Context, master *encryption.MasterKey, epoch encryption.Epoch, o Options, r objectRef, previous uint32) (*segment, error) {
	path := s.walPath(r.FileID)
	f, i, e := s.ns.checked(path, os.O_RDONLY)
	if e != nil {
		return nil, e
	}
	seg, e := verifySegment(ctx, f, uint64(i.Size()), master, epoch, o, r, previous)
	if e != nil {
		f.Close()
		return nil, e
	}
	seg.file, seg.info, seg.path, seg.ns = f, i, path, s.ns
	if e = s.ns.stable(f, path, i); e != nil {
		seg.close()
		return nil, e
	}
	return seg, nil
}
func (s *Store) writeSegment(ctx context.Context, d encryption.FileDescriptor, pages []uint32, src PageSource) (*segment, error) {
	first, e := src.ReadPage(ctx, 1)
	if e != nil {
		return nil, e
	}
	if len(first) != int(d.PageSize) {
		clear(first)
		return nil, encryption.ErrFormat
	}
	p1 := sha256.Sum256(first)
	clear(first)
	w, e := encryption.NewFileWriter(s.master, d, s.opts.Limits)
	if e != nil {
		return nil, e
	}
	defer w.Close()
	h, e := w.Header()
	if e != nil {
		return nil, e
	}
	r := objectRef{Descriptor: d, FileID: h.FileID, Bytes: segmentSize(d.PageSize, uint64(len(pages))), PageOneHash: p1}
	path := s.walPath(h.FileID)
	f, _, e := s.ns.checked(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if e != nil {
		return nil, e
	}
	closed := false
	defer func() {
		if !closed {
			f.Close()
		}
	}()
	digest := sha256.New()
	out := io.MultiWriter(f, digest)
	raw, e := encryption.EncodeHeader(h, s.opts.Limits)
	if e != nil {
		return nil, e
	}
	if e = writeBytes(out, raw); e != nil {
		return nil, e
	}
	meta := segmentMetadata(d, s.Geometry().PageCount, p1, pages)
	metaHash := sha256.Sum256(meta)
	b, e := w.Seal(encryption.RecordMetadata, 0, 0, meta)
	clear(meta)
	if e != nil {
		return nil, e
	}
	if e = writeBytes(out, b); e != nil {
		return nil, e
	}
	for i, n := range pages {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		var page []byte
		if n == lockPage(d.PageSize) {
			page = make([]byte, d.PageSize)
		} else {
			page, e = src.ReadPage(ctx, n)
			if e != nil {
				return nil, e
			}
		}
		if len(page) != int(d.PageSize) || (n == 1 && sha256.Sum256(page) != p1) {
			clear(page)
			return nil, encryption.ErrFormat
		}
		b, e = w.Seal(encryption.RecordData, n, uint64(i+1), page)
		clear(page)
		if e != nil {
			return nil, e
		}
		if e = writeBytes(out, b); e != nil {
			return nil, e
		}
	}
	var prefix [32]byte
	copy(prefix[:], digest.Sum(nil))
	b, e = w.Seal(encryption.RecordFooter, 0, 0, segmentFooter(r, s.Geometry().PageCount, prefix, metaHash))
	if e != nil {
		return nil, e
	}
	if e = writeBytes(out, b); e != nil {
		return nil, e
	}
	if e = w.Finish(); e != nil {
		return nil, e
	}
	if e = f.Sync(); e != nil {
		return nil, e
	}
	e = f.Close()
	closed = true
	if e != nil {
		return nil, e
	}
	copy(r.Hash[:], digest.Sum(nil))
	r.CommitID = r.commitID(h.Epoch)
	return s.openSegment(ctx, s.master, h.Epoch, s.opts, r, s.Geometry().PageCount)
}

// A location has no plaintext and is exactly eight bytes. Source zero means
// authenticated base; source >=1 identifies a selected segment and ordinal.
type location struct{ source, ordinal uint32 }

func buildOverlay(m walManifest, segs []*segment) ([]location, uint32, error) {
	max := m.Base.Descriptor.PageCount
	for _, r := range m.Segments {
		if r.Descriptor.PageCount > max {
			max = r.Descriptor.PageCount
		}
	}
	loc := make([]location, uint64(max)+1)
	ceiling := m.Base.Descriptor.PageCount
	previous := ceiling
	for i, s := range segs {
		final := s.ref.Descriptor.PageCount
		if final < previous {
			clear(loc[uint64(final)+1 : uint64(previous)+1])
			if final < ceiling {
				ceiling = final
			}
		}
		for j, n := range s.pages {
			loc[n] = location{uint32(i + 1), uint32(j + 1)}
		}
		previous = final
	}
	for n := uint32(1); n <= previous; n++ {
		if loc[n].source == 0 && n > ceiling {
			return nil, 0, encryption.ErrFormat
		}
	}
	return loc, ceiling, nil
}
