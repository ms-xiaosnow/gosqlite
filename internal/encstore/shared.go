package encstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"github.com/ms-xiaosnow/gosqlite/encryption"
	"os"
)

const (
	lifecycleByte int64 = 0
	writerByte    int64 = 1
	captureByte   int64 = 2
)

func (s *Store) poisonOwnership(err error, uncertain bool) error {
	if err == nil {
		return nil
	}
	s.poisoned = errors.Join(s.poisoned, ErrPoisoned, err)
	if uncertain {
		s.poisoned = errors.Join(ErrUncertain, s.poisoned)
	}
	return s.poisoned
}
func (s *Store) enterGate(shared bool) error {
	if s.gateOwned {
		return ErrUnsupported
	}
	if e := rangeLock(s.lock, captureByte, shared, false); e != nil {
		return e
	}
	s.gateOwned = true
	return nil
}
func (s *Store) leaveGate(uncertain bool) error {
	if !s.gateOwned {
		return nil
	}
	e := rangeLock(s.lock, captureByte, false, true)
	if e == nil {
		s.gateOwned = false
	}
	return s.poisonOwnership(e, uncertain)
}
func (s *Store) downgradeLifecycle() error {
	if !s.lifecycleExclusive {
		return nil
	}
	e := rangeLock(s.lock, lifecycleByte, true, false)
	if e == nil {
		s.lifecycleExclusive = false
	}
	return s.poisonOwnership(e, true)
}

// captureRoot owns only the short capture gate. Provider/KDF and whole-graph
// authentication run after it is released. Shared snapshots never retain the
// supersedable named root descriptor.
func (s *Store) captureRoot() (raw []byte, err error) {
	shared := s.opts.Mode == WALShared
	if shared {
		if err = s.enterGate(true); err != nil {
			return nil, err
		}
		defer func() { err = errors.Join(err, s.leaveGate(false)) }()
	}
	if err = s.ns.check(); err != nil {
		return nil, err
	}
	if err = s.ns.stable(s.lock, s.path+".gse-lock", s.lockInfo); err != nil {
		return nil, err
	}
	f, info, err := s.ns.checked(s.path, os.O_RDONLY)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			err = errors.Join(err, s.poisonOwnership(f.Close(), false))
		}
	}()
	if err = rootExtent(info.Size(), s.opts); err != nil {
		return nil, err
	}
	var oldMemory uint64
	if old := s.retainedGraph(); old != nil {
		oldMemory = selectionMemory(*old)
	}
	if e := rootBytesBudget(oldMemory, uint64(info.Size()), s.writerIDs, s.opts); e != nil {
		return nil, e
	}
	raw, err = readExact(f, int(info.Size()), 0)
	if err != nil {
		return nil, err
	}
	if err = s.ns.stable(f, s.path, info); err != nil {
		return nil, err
	}
	if !shared {
		s.root, s.rootInfo = f, info
		keep = true
	}
	return raw, nil
}

func (s *Store) closeSelection() error {
	var err error
	for _, seg := range s.segments {
		err = errors.Join(err, seg.close())
	}
	if s.base != nil {
		err = errors.Join(err, s.base.close())
	}
	if s.root != nil {
		err = errors.Join(err, s.root.Close())
	}
	clear(s.rootBytes)
	s.selection = selection{}
	return err
}

// buildSelection is isolated from the retained selection and borrows only the
// owner namespace, lock and master. Cleanup never closes those owner resources.
func (s *Store) buildSelection(ctx context.Context, raw []byte) (*Store, error) {
	var oldMemory uint64
	if old := s.retainedGraph(); old != nil {
		oldMemory = selectionMemory(*old)
	}
	if e := rootBytesBudget(oldMemory, uint64(len(raw)), s.writerIDs, s.opts); e != nil {
		return nil, e
	}
	h, p, e := rootPayload(raw, s.master, s.opts)
	if e != nil {
		return nil, e
	}
	defer clear(p)
	m, e := decodeStorageWAL(p, h, s.opts)
	if e != nil {
		return nil, e
	}
	if e = combinedGraphBudget(s.wal, m, s.opts, s.writerIDs); e != nil {
		return nil, e
	}
	c := &Store{selection: selection{rootBytes: raw, header: h}, ns: s.ns, path: s.path, opts: s.opts, readOnly: s.readOnly, lock: s.lock, lockInfo: s.lockInfo, master: s.master}
	if e = c.openWALSelection(ctx, m); e != nil {
		return nil, errors.Join(e, s.poisonOwnership(c.closeSelection(), false))
	}
	return c, nil
}

// Refresh requires local quiescence. Failure leaves the prior selection owned
// and usable; adoption occurs only after full authentication and peak admission.
func (s *Store) Refresh(ctx context.Context) error {
	if s.opts.Mode != WALShared {
		return s.Check()
	}
	if e := s.Check(); e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	raw, e := s.captureRoot()
	if e != nil {
		return e
	}
	c, e := s.buildSelection(ctx, raw)
	if e != nil {
		clear(raw)
		return e
	}
	old := s.selection
	s.selection = c.selection
	c.selection = old
	if e = c.closeSelection(); e != nil {
		return s.poisonOwnership(e, false)
	}
	return nil
}

func (s *Store) Authority() [32]byte { return sha256.Sum256(s.rootBytes) }

// AcquireWriter reserves first. A non-nil expectation denotes an exposed read
// snapshot: stale promotion never replaces it, even on successful admission.
func (s *Store) AcquireWriter(ctx context.Context, expected *[32]byte) (err error) {
	if s.opts.Mode != WALShared {
		return s.Check()
	}
	if err = s.Check(); err != nil {
		return err
	}
	if s.readOnly {
		return ErrReadOnly
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if s.writerOwned {
		return ErrUnsupported
	}
	if err = rangeLock(s.lock, writerByte, false, false); err != nil {
		return err
	}
	s.writerOwned = true
	defer func() {
		if err != nil {
			err = errors.Join(err, s.ReleaseWriter(false))
		}
	}()
	if expected == nil {
		return s.Refresh(ctx)
	}
	raw, err := s.captureRoot()
	if err != nil {
		return err
	}
	c, err := s.buildSelection(ctx, raw)
	if err != nil {
		clear(raw)
		return err
	}
	equal := c.Authority() == *expected
	err = c.closeSelection()
	if err != nil {
		return s.poisonOwnership(err, false)
	}
	if !equal {
		return ErrBusySnapshot
	}
	return nil
}
func (s *Store) ReleaseWriter(published bool) error {
	if !s.writerOwned {
		return nil
	}
	e := rangeLock(s.lock, writerByte, false, true)
	if e == nil {
		s.writerOwned = false
	}
	return s.poisonOwnership(e, published)
}

// checkAuthority runs with byte 1 owned and byte 2 exclusive at cutover. It
// authenticates current named bytes; snapshot Check deliberately does not.
func (s *Store) checkAuthority() (err error) {
	if s.opts.Mode != WALShared {
		return s.Check()
	}
	if !s.writerOwned && !s.lifecycleExclusive {
		return ErrUnsupported
	}
	if err = s.checkOwner(); err != nil {
		return err
	}
	if len(s.rootBytes) == 0 {
		if _, e := os.Lstat(s.path); !os.IsNotExist(e) {
			return ErrChanged
		}
		return nil
	}
	f, info, e := s.ns.checked(s.path, os.O_RDONLY)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, s.poisonOwnership(f.Close(), false)) }()
	if e = rootExtent(info.Size(), s.opts); e != nil {
		return e
	}
	raw, e := readExact(f, int(info.Size()), 0)
	if e != nil {
		return e
	}
	defer clear(raw)
	if e = verifyRootBytes(raw, s.master, s.opts); e != nil {
		return e
	}
	if !bytes.Equal(raw, s.rootBytes) {
		return ErrChanged
	}
	return s.ns.stable(f, s.path, info)
}

// BeginMaintenance acquires the latest authority before any copy validation.
// Rekey additionally converts byte 0 atomically; BUSY retains the SHARED lease.
func (s *Store) BeginMaintenance(ctx context.Context, rekey bool) (err error) {
	if s.opts.Mode != WALShared {
		return s.Check()
	}
	if err = s.AcquireWriter(ctx, nil); err != nil {
		return err
	}
	if rekey {
		if err = rangeLock(s.lock, lifecycleByte, false, false); err != nil {
			return errors.Join(err, s.ReleaseWriter(false))
		}
		s.lifecycleExclusive = true
	}
	return nil
}
func (s *Store) EndMaintenance(published bool) error {
	var err error
	if s.opts.Mode == WALShared && s.lifecycleExclusive {
		err = s.downgradeLifecycle()
	}
	return errors.Join(err, s.ReleaseWriter(published))
}

// UpgradeShared retains the original exclusive lifecycle lock through verified
// layout-3 publication, then atomically downgrades it. No old object is removed.
func (s *Store) UpgradeShared(ctx context.Context) (err error) {
	if s.opts.Mode == WALShared || s.readOnly || !s.lifecycleExclusive {
		return ErrUnsupported
	}
	if err = s.CopyBudget(); err != nil {
		return err
	}
	if err = s.Verify(ctx); err != nil {
		return err
	}
	if err = s.publishBase(ctx, s.Geometry(), s, nil, WALShared); err != nil {
		return err
	}
	return s.downgradeLifecycle()
}

func (s *Store) retainedGraph() *walManifest {
	if s.wal != nil {
		return s.wal
	}
	if s.base == nil {
		return nil
	}
	return &walManifest{Base: objectRef{Descriptor: s.base.header.Descriptor, Bytes: objectSize(s.Geometry())}}
}

// rootBytesBudget reserves simultaneously retained root evidence and bounded
// scratch before graph metadata allocation. No root refresh resets the ledger.
func rootBytesBudget(old uint64, next uint64, ids uint64, o Options) error {
	l, e := o.WALLimits.normalized()
	if e != nil {
		return e
	}
	if ids > encryption.MaxFilesPerMaster*2 {
		return encryption.ErrLimit
	}
	// Twelve root extents cover captured bytes, AEAD input/output, the layout
	// conversion, refs, ID-validation map and metadata codec scratch together.
	if next > l.MaxMemoryBytes/12 {
		return encryption.ErrLimit
	}
	n, e := charge(old, next*12, l.MaxMemoryBytes)
	if e != nil {
		return e
	}
	_, e = charge(n, ids*128, l.MaxMemoryBytes)
	return e
}

// checkOwner is separate from both immutable selection and current-root
// authority. Recheck it at cutover, after candidate preparation, and after
// publication without accepting replaced/detached directory or lock objects.
func (s *Store) checkOwner() error {
	if s.lock == nil {
		return encryption.ErrClosed
	}
	if e := s.ns.check(); e != nil {
		return e
	}
	return s.ns.stable(s.lock, s.path+".gse-lock", s.lockInfo)
}
