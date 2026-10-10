package encstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

// Store is serialized by its pager session. A separate stable lock inode is
// retained throughout its lifetime. Historical encrypted objects/roots are
// retained, never automatically selected or garbage-collected.
type selection struct {
	wal         *walManifest
	segments    []*segment
	overlay     []location
	baseCeiling uint32
	root        *managedFile // legacy modes only; shared roots own authenticated bytes
	rootInfo    os.FileInfo
	rootBytes   []byte
	header      encryption.Header
	base        *object
}

type Store struct {
	selection
	writerIDs          uint64
	ns                 *namespace
	path               string
	opts               Options
	readOnly           bool
	lock               *managedFile
	lockInfo           os.FileInfo
	master             *encryption.MasterKey
	poisoned           error
	lifecycleExclusive bool
	writerOwned        bool
	gateOwned          bool
}

func canonical(path string) (string, error) {
	return canonicalPath(path)
}

// rejectPlainSidecars is a read-only preflight, repeated under the stable
// lease. Known incompatible plaintext sidecars must not strand a new family
// merely because its lock inode was created before detecting the conflict.
func rejectPlainSidecars(path string) error {
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		if _, err := platformPathInfo(path + suffix); err == nil {
			return ErrUnsupported
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func acquire(path string, readOnly, create bool, mode Mode) (*Store, bool, error) {
	if err := supported(); err != nil {
		return nil, false, err
	}
	path, err := canonical(path)
	if err != nil {
		return nil, false, err
	}
	_, err = platformPathInfo(path)
	absent := os.IsNotExist(err)
	if err != nil && !absent {
		return nil, false, err
	}
	if absent && (!create || readOnly) {
		return nil, false, os.ErrNotExist
	}
	flags := os.O_RDWR
	if readOnly {
		flags = os.O_RDONLY
	}
	if absent && create {
		flags |= os.O_CREATE | os.O_EXCL
	}
	if err = rejectPlainSidecars(path); err != nil {
		return nil, false, err
	}
	ns, err := newNamespace(path)
	if err != nil {
		return nil, false, err
	}
	kept := false
	defer func() {
		if !kept {
			_ = ns.dir.Close()
		}
	}()
	f, info, err := ns.checked(path+".gse-lock", flags)
	if err != nil {
		return nil, false, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = f.Close()
		}
	}()
	if info.Size() != 0 {
		return nil, false, ErrUnsupported
	}
	if err = lockFile(f, readOnly || (mode == WALShared && !absent)); err != nil {
		return nil, false, err
	}
	if err = ns.stable(f, path+".gse-lock", info); err != nil {
		return nil, false, err
	}
	// Recheck the root only after acquiring the stable lease.
	_, err = platformPathInfo(path)
	absent = os.IsNotExist(err)
	if err != nil && !absent {
		return nil, false, err
	}
	if absent && (!create || readOnly) {
		return nil, false, os.ErrNotExist
	}
	if err = rejectPlainSidecars(path); err != nil {
		return nil, false, err
	}
	if err = ns.check(); err != nil {
		return nil, false, err
	}
	ok = true
	kept = true
	return &Store{path: path, readOnly: readOnly, lock: f, lockInfo: info, ns: ns, lifecycleExclusive: !readOnly && (mode != WALShared || absent)}, absent, nil
}

func Open(ctx context.Context, path string, o Options, readOnly, create bool, initial []byte) (*Store, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := o.Validate(); err != nil {
		return nil, err
	}
	if err := supported(); err != nil {
		return nil, err
	}
	// Preflight brand-new geometry and obtain credentials before creating even
	// the stable lock inode. A failed provider must not strand a new family.
	path, err := canonical(path)
	if err != nil {
		return nil, err
	}
	var initialTarget *Prepared
	_, probeErr := platformPathInfo(path)
	if os.IsNotExist(probeErr) && create && !readOnly {
		desc := encryption.FileDescriptor{Role: encryption.FileBase, PageSize: uint32(len(initial)), PageCount: 1, RecordCount: 3}
		if err = desc.Validate(o.Limits); err != nil {
			return nil, err
		}
		if err = publicationBudget(ctx, path, o, Geometry{uint32(len(initial)), 1}, 1, false); err != nil {
			return nil, err
		}
		initialTarget, err = Prepare(ctx, path, o, encryption.OperationCreate, [16]byte{})
		if err != nil {
			return nil, err
		}
		defer initialTarget.Close()
	} else if probeErr != nil && !os.IsNotExist(probeErr) {
		return nil, probeErr
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	s, absent, err := acquire(path, readOnly, initialTarget != nil, o.Mode)
	if err != nil {
		return nil, err
	}
	s.opts = o
	ok := false
	defer func() {
		if !ok {
			_ = s.Close()
		}
	}()
	if absent {
		src := SourceFunc(func(_ context.Context, n uint32) ([]byte, error) {
			if n != 1 {
				return nil, encryption.ErrIdentity
			}
			return bytes.Clone(initial), nil
		})
		if err = s.Publish(ctx, Geometry{uint32(len(initial)), 1}, src, initialTarget); err != nil {
			return nil, err
		}
	} else {
		raw, e := s.captureRoot()
		if e != nil {
			return nil, e
		}

		h, e := encryption.DecodeHeader(raw[:encryption.HeaderSize], o.Limits)
		if e != nil {
			return nil, e
		}
		if h.Descriptor.Role != encryption.FileRoot || h.Epoch.Kind != o.CredentialKind {
			return nil, encryption.ErrAuthentication
		}
		m, e := encryption.DeriveMaster(ctx, o.KeyProvider, encryption.KeyRequest{Operation: encryption.OperationOpen, Path: s.path}, h.Epoch, o.KDFPolicy, o.DerivationBudget)
		if e != nil {
			return nil, e
		}
		s.master = m
		s.header = h
		s.rootBytes = raw
		if e = s.openSelection(ctx); e != nil {
			return nil, e
		}
	}
	if err = s.Check(); err != nil {
		return nil, err
	}
	if o.Mode == WALShared && s.lifecycleExclusive {
		if err = s.downgradeLifecycle(); err != nil {
			return nil, err
		}
	}
	ok = true
	return s, nil
}
func openRoot(raw []byte, m *encryption.MasterKey, limits encryption.Limits) (manifest, error) {
	if len(raw) != rootSize {
		return manifest{}, encryption.ErrFormat
	}
	h, err := encryption.DecodeHeader(raw[:encryption.HeaderSize], limits)
	if err != nil {
		return manifest{}, err
	}
	if h.Descriptor.Role != encryption.FileRoot {
		return manifest{}, encryption.ErrFormat
	}
	r, p, err := encryption.OpenFile(m, h.Identity(), raw[:encryption.HeaderSize], raw[encryption.HeaderSize:], limits)
	if err != nil {
		return manifest{}, err
	}
	defer r.Close()
	defer clear(p)
	return decodeManifest(p)
}
func (s *Store) basePath(id [32]byte) string { return s.path + ".gse-" + idString(id) + ".base" }
func (s *Store) Geometry() Geometry {
	return Geometry{s.header.Descriptor.PageSize, s.header.Descriptor.PageCount}
}
func (s *Store) DatabaseID() [16]byte { return s.header.Epoch.DatabaseID }
func (s *Store) Check() error {
	if s.poisoned != nil {
		return s.poisoned
	}
	if s.lock == nil {
		return encryption.ErrClosed
	}
	if err := s.ns.check(); err != nil {
		return err
	}
	if err := s.ns.stable(s.lock, s.path+".gse-lock", s.lockInfo); err != nil {
		return err
	}
	if s.opts.Mode == WALShared && len(s.rootBytes) != 0 {
		return s.checkSelected()
	}
	if s.root == nil {
		if _, err := platformPathInfo(s.path); !os.IsNotExist(err) {
			return ErrChanged
		}
		return nil
	}
	if err := s.ns.stable(s.root, s.path, s.rootInfo); err != nil {
		return err
	}
	raw, err := readExact(s.root, len(s.rootBytes), 0)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, s.rootBytes) {
		return encryption.ErrAuthentication
	}
	return s.ns.stable(s.base.file, s.base.path, s.base.info)
}
func (s *Store) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	if err := s.Check(); err != nil {
		return nil, err
	}
	if n == 0 || n > s.Geometry().PageCount {
		return nil, encryption.ErrIdentity
	}
	if s.wal != nil {
		loc := s.overlay[n]
		if loc.source != 0 {
			return s.segments[loc.source-1].readPage(ctx, n, loc.ordinal)
		}
		if n > s.baseCeiling {
			return nil, encryption.ErrFormat
		}
	}
	return s.base.readPage(ctx, n)
}
func (s *Store) Verify(ctx context.Context) error {
	if err := s.Check(); err != nil {
		return err
	}
	if s.wal != nil {
		return s.verifyWAL(ctx)
	}
	m, err := openRoot(s.rootBytes, s.master, s.opts.Limits)
	if err != nil {
		return err
	}
	desc := s.base.header.Descriptor
	o, err := openObject(ctx, s.base.path, s.master, s.opts.Limits, s.base.header.Identity(), &desc, &m.BaseHash, &m.PageOneHash, s.ns)
	if err != nil {
		return err
	}
	return o.close()
}
func syncDirectory(path string) error {
	f, err := openDirectory(filepath.Dir(path))
	if err != nil {
		return err
	}
	return errors.Join(f.Sync(), f.Close())
}
func (s *Store) Close() error {
	var err error
	err = s.closeSelection()
	if s.master != nil {
		err = errors.Join(err, s.master.Close())
		s.master = nil
	}
	if s.lock != nil {
		err = errors.Join(err, s.lock.Close())
		s.lock = nil
	}
	if s.ns != nil {
		err = errors.Join(err, s.ns.dir.Close())
		s.ns = nil
	}
	return errors.Join(s.poisoned, err)
}

type Info struct {
	Mode                Mode
	StorageLayout       uint16
	WALSegments         uint32
	WALFrames, WALBytes uint64
	HeadCommitID        [32]byte
	PageSize, PageCount uint32
	DatabaseID          [16]byte
	CommitSequence      uint64
	RetainsHistory      bool
}

func (s *Store) Info() Info {
	i := Info{Mode: s.opts.Mode, StorageLayout: 1, PageSize: s.header.Descriptor.PageSize, PageCount: s.header.Descriptor.PageCount, DatabaseID: s.header.Epoch.DatabaseID, CommitSequence: s.header.Descriptor.CommitSequence, RetainsHistory: true}
	if s.wal != nil {
		i.StorageLayout = 2
		if s.opts.Mode == WALShared {
			i.StorageLayout = 3
		}
		i.WALSegments = uint32(len(s.segments))
		i.WALFrames = s.wal.Frames
		i.WALBytes = s.wal.Bytes
		i.HeadCommitID = s.wal.Head
	} else {
		i.HeadCommitID = sha256.Sum256(s.rootBytes)
	}
	return i
}
