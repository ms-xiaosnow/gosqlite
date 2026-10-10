package encstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"math"
	"os"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

func (s *Store) nextDescriptor(g Geometry, role encryption.FileRole, records uint64) (encryption.FileDescriptor, error) {
	d := encryption.FileDescriptor{Role: role, PageSize: g.PageSize, PageCount: g.PageCount, RecordCount: records, CommitSequence: s.header.Descriptor.CommitSequence}
	if d.CommitSequence == math.MaxUint64 {
		return d, encryption.ErrLimit
	}
	d.CommitSequence++
	if _, e := rand.Read(d.TransactionID[:]); e != nil {
		return d, encryption.ErrRandomness
	}
	if d.TransactionID == ([32]byte{}) {
		return d, encryption.ErrRandomness
	}
	if len(s.rootBytes) != 0 {
		d.ParentCommitID = s.HeadCommitID()
	}
	return d, nil
}
func (s *Store) prepublish(ctx context.Context) error {
	if s.opts.Mode == WALShared && !s.writerOwned && !s.lifecycleExclusive {
		return ErrUnsupported
	}
	if s.readOnly {
		return ErrReadOnly
	}
	if e := s.Check(); e != nil {
		return e
	}
	return ctx.Err()
}

// Publish materializes a complete logical head. Snapshot commits use this;
// WAL transactions exclusively use PublishWAL. Checkpoint/rekey/upgrade are
// explicit base publications and never change the logical page-one counters.
func (s *Store) Publish(ctx context.Context, g Geometry, src PageSource, target *Prepared) error {
	return s.publishBase(ctx, g, src, target, s.opts.Mode)
}
func (s *Store) publishBase(ctx context.Context, g Geometry, src PageSource, target *Prepared, mode Mode) error {
	if e := s.prepublish(ctx); e != nil {
		return e
	}
	master, opts := s.master, s.opts
	if target != nil {
		master, opts = target.Master, target.Options
		if len(s.rootBytes) != 0 && opts.Mode != s.opts.Mode {
			return ErrUnsupported
		}
	}
	opts.Mode = mode
	if master == nil {
		return encryption.ErrClosed
	}
	d, e := s.nextDescriptor(g, encryption.FileBase, uint64(g.PageCount)+2)
	if e != nil {
		return e
	}
	if e = d.Validate(opts.Limits); e != nil {
		return e
	}
	if e = basePlanBudget(opts, g); e != nil {
		return e
	}
	ids := s.writerIDs
	if target != nil {
		ids = target.writerIDs
	}
	if ids > encryption.MaxFilesPerMaster-2 {
		return encryption.ErrLimit
	}
	peakIDs := ids + 2
	if target != nil && master != s.master {
		peakIDs += s.writerIDs
	}
	l, e := opts.WALLimits.normalized()
	if e != nil {
		return e
	}
	if peakIDs*128 > l.MaxMemoryBytes {
		return encryption.ErrLimit
	}
	rootBytes := uint64(rootSize)
	if mode.IsWAL() {
		rootBytes = walRootSize(0)
		stub := walManifest{Base: objectRef{Descriptor: d, Bytes: objectSize(g)}}
		if e = combinedGraphBudget(s.retainedGraph(), stub, opts, peakIDs); e != nil {
			return e
		}
	}
	if s.wal != nil {
		if e = graphBudget(*s.wal, opts, 16, peakIDs*128); e != nil {
			return e
		}
	}
	if e = plannedBudget(ctx, s.path, s.ns, opts, d.CommitSequence, objectSize(g), rootBytes, uint64(len(s.rootBytes)), len(s.rootBytes) != 0); e != nil {
		return e
	}
	if target != nil && len(s.rootBytes) != 0 {
		if e = s.Verify(ctx); e != nil {
			return e
		}
	}
	// Reserve both object/root identities before constructing either writer.
	// A failed attempt keeps the conservative reservation until master Close.
	if target != nil {
		target.writerIDs += 2
	} else {
		s.writerIDs += 2
	}
	obj, e := writeObject(ctx, s.basePath, master, opts.Limits, d, src, s.ns)
	if e != nil {
		return e
	}
	adopted := false
	defer func() {
		if !adopted {
			obj.close()
		}
	}()
	if e = s.ns.sync(s.path); e != nil {
		return e
	}
	var payload []byte
	var wm *walManifest
	var loc []location
	var ceiling uint32
	if mode.IsWAL() {
		ref := refFor(obj)
		m := walManifest{Base: ref, Head: ref.CommitID, PageOneHash: ref.PageOneHash}
		if len(s.rootBytes) != 0 {
			m.PreviousRootHash = sha256.Sum256(s.rootBytes)
		}
		wm = &m
		payload = m.encodeMode(mode)
		loc, ceiling, e = buildOverlay(m, nil)
		if e != nil {
			return e
		}
	} else {
		m := manifest{BaseID: obj.header.FileID, BaseBytes: objectSize(g), BaseHash: obj.hash, PageOneHash: obj.pageOneHash}
		payload = m.encode()
	}
	oldBase, oldSegments, oldMaster := s.base, s.segments, s.master
	if e = s.publishRoot(ctx, master, opts, d, payload, func() error { return obj.ns.stable(obj.file, obj.path, obj.info) }); e != nil {
		return e
	}
	if target != nil {
		s.writerIDs = target.writerIDs
	}
	s.base, s.wal, s.segments, s.overlay, s.baseCeiling, s.master, s.opts = obj, wm, nil, loc, ceiling, master, opts
	adopted = true
	if target != nil {
		target.Master = nil
	}
	var closeErr error
	if oldBase != nil {
		closeErr = errors.Join(closeErr, oldBase.close())
	}
	for _, seg := range oldSegments {
		closeErr = errors.Join(closeErr, seg.close())
	}
	if oldMaster != nil && oldMaster != master {
		closeErr = errors.Join(closeErr, oldMaster.Close())
	}
	return s.poisonOwnership(closeErr, true)
}

func (s *Store) PublishWAL(ctx context.Context, expected [32]byte, g Geometry, pages []uint32, src PageSource) error {
	if e := s.prepublish(ctx); e != nil {
		return e
	}
	if s.wal == nil || !s.opts.Mode.IsWAL() {
		return ErrUnsupported
	}
	if expected != s.wal.Head {
		return ErrChanged
	}
	if g.PageSize != s.Geometry().PageSize {
		return encryption.ErrFormat
	}
	l, e := s.opts.WALLimits.normalized()
	if e != nil {
		return e
	}
	if uint64(len(pages)) > uint64(l.MaxSegmentFrames) {
		return encryption.ErrLimit
	}
	if e = validateDirectory(pages, s.Geometry().PageCount, g.PageCount); e != nil {
		return e
	}
	d, e := s.nextDescriptor(g, encryption.FileWAL, uint64(len(pages))+2)
	if e != nil {
		return e
	}
	if e = d.Validate(s.opts.Limits); e != nil {
		return e
	}
	if e = (encryption.RecordHeader{Role: encryption.RecordMetadata, PlaintextSize: uint32(128 + 4*len(pages))}).Validate(s.opts.Limits); e != nil {
		return e
	}
	// All candidate allocations, active selection, scratch, I/O passes, retained
	// inventory and future directory entries are reserved before creating a file.
	size := segmentSize(g.PageSize, uint64(len(pages)))
	if size > l.MaxSegmentBytes {
		return encryption.ErrLimit
	}
	m := *s.wal
	if len(m.Segments)+1 > int(l.MaxSegments) {
		return encryption.ErrLimit
	}
	m.Frames, e = charge(m.Frames, uint64(len(pages)), l.MaxFrames)
	if e != nil {
		return e
	}
	m.Bytes, e = charge(m.Bytes, size, l.MaxBytes)
	if e != nil {
		return e
	}
	// Charge retained graph plus metadata scratch/ledger before cloning refs.
	if e = rootBytesBudget(selectionMemory(*s.wal), walRootSize(len(m.Segments)+1), s.writerIDs+2, s.opts); e != nil {
		return e
	}
	// Bounded descriptor list is allocated only after its configured count cap.
	m.Segments = append(append([]objectRef(nil), m.Segments...), objectRef{Descriptor: d, Bytes: size})
	if e = combinedGraphBudget(s.wal, m, s.opts, s.writerIDs+2); e != nil {
		return e
	}
	work := uint64(0)
	for _, amount := range []uint64{size, size, size, 8 * uint64(len(s.rootBytes)), 8 * walRootSize(len(m.Segments))} {
		work, e = charge(work, amount, l.MaxOperationBytes)
		if e != nil {
			return e
		}
	}
	if e = (encryption.RecordHeader{Role: encryption.RecordMetadata, PlaintextSize: uint32(512 + 256*len(m.Segments))}).Validate(s.opts.Limits); e != nil {
		return e
	}
	if e = plannedBudget(ctx, s.path, s.ns, s.opts, d.CommitSequence, size, walRootSize(len(m.Segments)), uint64(len(s.rootBytes)), true); e != nil {
		return e
	}
	if s.writerIDs > encryption.MaxFilesPerMaster-2 {
		return encryption.ErrLimit
	}
	s.writerIDs += 2
	seg, e := s.writeSegment(ctx, d, pages, src)
	if e != nil {
		return e
	}
	adopted := false
	defer func() {
		if !adopted {
			seg.close()
		}
	}()
	if e = s.ns.sync(s.path); e != nil {
		return e
	}
	m.Segments[len(m.Segments)-1] = seg.ref
	m.Head, m.PageOneHash, m.PreviousRootHash = seg.ref.CommitID, seg.ref.PageOneHash, sha256.Sum256(s.rootBytes)
	segs := append(append([]*segment(nil), s.segments...), seg)
	loc, ceiling, e := buildOverlay(m, segs)
	if e != nil {
		return e
	}
	if e = s.publishRoot(ctx, s.master, s.opts, d, m.encodeMode(s.opts.Mode), func() error { return seg.ns.stable(seg.file, seg.path, seg.info) }); e != nil {
		return e
	}
	s.wal, s.segments, s.overlay, s.baseCeiling = &m, segs, loc, ceiling
	adopted = true
	return nil
}

// publishRoot is the single authority cutover. From rename attempt onward,
// every failure is uncertain and poisons the handle. No recovery/promotion or
// fallback to retained roots is attempted. Obsolete descriptor close cannot
// undo a durably published root or turn success into SQL rollback.
func (s *Store) publishRoot(ctx context.Context, master *encryption.MasterKey, opts Options, d encryption.FileDescriptor, payload []byte, candidateStable func() error) (err error) {
	defer clear(payload)
	d.Role = encryption.FileRoot
	d.RecordCount = 1
	w, e := encryption.NewFileWriter(master, d, opts.Limits)
	if e != nil {
		return e
	}
	defer w.Close()
	h, e := w.Header()
	if e != nil {
		return e
	}
	raw, e := encryption.EncodeHeader(h, opts.Limits)
	if e != nil {
		return e
	}
	record, e := w.Seal(encryption.RecordMetadata, 0, 0, payload)
	if e != nil {
		return e
	}
	raw = append(raw, record...)
	if e = w.Finish(); e != nil {
		return e
	}
	// Parse authenticated candidate bytes before the first root-file write.
	if e = verifyRootBytes(raw, master, opts); e != nil {
		return e
	}
	pending := s.path + ".gse-" + idString(h.FileID) + ".pending"
	if e = s.ns.write(pending, raw); e != nil {
		return e
	}
	check, info, e := s.ns.checked(pending, os.O_RDONLY)
	if e != nil {
		return e
	}
	verified, e := readExact(check, len(raw), 0)
	if e == nil && (info.Size() != int64(len(raw)) || !bytes.Equal(verified, raw)) {
		e = ErrChanged
	}
	if e == nil {
		e = verifyRootBytes(verified, master, opts)
	}
	if e == nil {
		e = s.ns.stable(check, pending, info)
	}
	e = errors.Join(e, check.Close())
	if e != nil {
		return e
	}
	if len(s.rootBytes) != 0 {
		if e = s.ns.retain(s.path+".gse-"+idString(s.header.FileID)+".retained", s.rootBytes); e != nil {
			return e
		}
	}
	if e = s.ns.sync(s.path); e != nil {
		return e
	}
	// Only final authority verification and cutover hold the short exclusive
	// capture gate. Preparing/syncing immutable candidates does not block readers.
	attempted := false
	if opts.Mode == WALShared {
		if e = s.enterGate(false); e != nil {
			return e
		}
		defer func() { err = errors.Join(err, s.leaveGate(attempted)) }()
	}
	if e = s.checkAuthority(); e != nil {
		return e
	}
	if e = s.checkSelected(); e != nil {
		return e
	}
	if e = candidateStable(); e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	uncertain := func(e error) error { s.poisoned = errors.Join(ErrUncertain, e); return s.poisoned }
	attempted = true
	if e = s.ns.rename(pending, s.path, len(s.rootBytes) == 0); e != nil {
		return uncertain(e)
	}
	if e = s.ns.sync(s.path); e != nil {
		return uncertain(e)
	}
	root, ri, e := s.ns.checked(s.path, os.O_RDONLY)
	if e != nil {
		return uncertain(e)
	}
	final, e := readExact(root, len(raw), 0)
	if e == nil && (ri.Size() != int64(len(raw)) || !bytes.Equal(final, raw)) {
		e = ErrChanged
	}
	if e == nil {
		e = s.ns.stable(root, s.path, ri)
	}
	if e != nil {
		root.Close()
		return uncertain(e)
	}
	if e = s.checkOwner(); e == nil {
		e = s.checkSelected()
	}
	if e == nil {
		e = candidateStable()
	}
	if e != nil {
		root.Close()
		return uncertain(e)
	}
	old := s.root
	if opts.Mode == WALShared {
		if e = root.Close(); e != nil {
			return uncertain(e)
		}
		root = nil
	}
	clear(s.rootBytes)
	s.root, s.rootInfo, s.rootBytes, s.header = root, ri, raw, h
	if old != nil {
		if e = old.Close(); e != nil {
			return uncertain(e)
		}
	}
	return nil
}
func verifyRootBytes(raw []byte, master *encryption.MasterKey, o Options) error {
	h, p, e := rootPayload(raw, master, o)
	if e != nil {
		return e
	}
	defer clear(p)
	if o.Mode.IsWAL() {
		_, e = decodeStorageWAL(p, h, o)
	} else {
		_, e = decodeManifest(p)
	}
	return e
}
func (s *Store) Checkpoint(ctx context.Context) (err error) {
	if s.opts.Mode == WALShared {
		if err = s.BeginMaintenance(ctx, false); err != nil {
			return err
		}
		before := s.Authority()
		defer func() { err = errors.Join(err, s.EndMaintenance(s.Authority() != before)) }()
	}
	if e := s.Check(); e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	if s.wal == nil || len(s.segments) == 0 {
		return nil
	}
	if s.readOnly {
		return ErrReadOnly
	}
	if e := s.CopyBudget(); e != nil {
		return e
	}
	if e := s.Verify(ctx); e != nil {
		return e
	}
	return s.Publish(ctx, s.Geometry(), s, nil)
}
func (s *Store) UpgradeWAL(ctx context.Context) error {
	if s.opts.Mode != Snapshot || s.wal != nil {
		return ErrUnsupported
	}
	if e := basePlanBudget(Options{WALLimits: s.opts.WALLimits, Limits: s.opts.Limits, Mode: WAL}, s.Geometry()); e != nil {
		return e
	}
	if e := s.Verify(ctx); e != nil {
		return e
	}
	return s.publishBase(ctx, s.Geometry(), s, nil, WAL)
}

// CopyBudget reserves all repeated logical/source/output passes as one bounded
// operation before the caller's catalog validation or destination publication.
// The conservative 16-pass envelope includes per-page authority revalidation.
func (s *Store) CopyBudget() error {
	if s.wal != nil {
		return graphBudget(*s.wal, s.opts, 16, s.writerIDs*128)
	}
	return nil
}
func basePlanBudget(o Options, g Geometry) error {
	payload := uint32(manifestSize)
	if o.Mode.IsWAL() {
		payload = 512
	}
	if e := (encryption.RecordHeader{Role: encryption.RecordMetadata, PlaintextSize: payload}).Validate(o.Limits); e != nil {
		return e
	}
	if e := rootExtent(int64(encryption.HeaderSize+recordOverhead+payload), o); e != nil {
		return e
	}
	if o.Mode.IsWAL() {
		m := walManifest{Base: objectRef{Descriptor: encryption.FileDescriptor{PageSize: g.PageSize, PageCount: g.PageCount}, Bytes: objectSize(g)}}
		return graphBudget(m, o, 4)
	}
	return nil
}

func (s *Store) checkSelected() error {
	if s.base != nil {
		if e := s.base.ns.stable(s.base.file, s.base.path, s.base.info); e != nil {
			return e
		}
	}
	for _, seg := range s.segments {
		if e := seg.ns.stable(seg.file, seg.path, seg.info); e != nil {
			return e
		}
	}
	return nil
}

// DirtyPagesBudget runs before the pager allocates its immutable sorted list.
func (s *Store) DirtyPagesBudget(count uint64) error {
	l, e := s.opts.WALLimits.normalized()
	if e != nil {
		return e
	}
	if count == 0 || count > uint64(l.MaxSegmentFrames) || count*4 > l.MaxMemoryBytes {
		return encryption.ErrLimit
	}
	return nil
}
