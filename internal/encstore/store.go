package encstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"math"
	"os"
	"path/filepath"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

// Store is serialized by its pager session. A separate stable lock inode is
// retained throughout its lifetime. Historical encrypted objects/roots are
// retained, never automatically selected or garbage-collected.
type Store struct {
	path      string
	opts      Options
	readOnly  bool
	lock      *os.File
	lockInfo  os.FileInfo
	root      *os.File
	rootInfo  os.FileInfo
	rootBytes []byte
	header    encryption.Header
	master    *encryption.MasterKey
	base      *object
	poisoned  error
}

func canonical(path string) (string, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(p))
	if err != nil {
		return "", err
	}
	p = filepath.Join(dir, filepath.Base(p))
	if info, e := os.Lstat(p); e == nil && !info.Mode().IsRegular() {
		return "", ErrUnsupported
	} else if e != nil && !os.IsNotExist(e) {
		return "", e
	}
	return p, nil
}

// rejectPlainSidecars is a read-only preflight, repeated under the stable
// lease. Known incompatible plaintext sidecars must not strand a new family
// merely because its lock inode was created before detecting the conflict.
func rejectPlainSidecars(path string) error {
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		if _, err := os.Lstat(path + suffix); err == nil {
			return ErrUnsupported
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func acquire(path string, readOnly, create bool) (*Store, bool, error) {
	if err := supported(); err != nil {
		return nil, false, err
	}
	path, err := canonical(path)
	if err != nil {
		return nil, false, err
	}
	_, err = os.Lstat(path)
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
	f, info, err := checkedFile(path+".gse-lock", flags)
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
	if err = lockFile(f, readOnly); err != nil {
		return nil, false, err
	}
	if err = stableFile(f, path+".gse-lock", info); err != nil {
		return nil, false, err
	}
	// Recheck the root only after acquiring the stable lease.
	_, err = os.Lstat(path)
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
	ok = true
	return &Store{path: path, readOnly: readOnly, lock: f, lockInfo: info}, absent, nil
}

func Open(ctx context.Context, path string, o Options, readOnly, create bool, initial []byte) (*Store, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := o.Validate(); err != nil {
		return nil, err
	}
	// Preflight brand-new geometry and obtain credentials before creating even
	// the stable lock inode. A failed provider must not strand a new family.
	path, err := canonical(path)
	if err != nil {
		return nil, err
	}
	var initialTarget *Prepared
	_, probeErr := os.Lstat(path)
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
	s, absent, err := acquire(path, readOnly, initialTarget != nil)
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
		f, info, e := checkedFile(s.path, os.O_RDONLY)
		if e != nil {
			return nil, e
		}
		s.root = f
		s.rootInfo = info
		if info.Size() != rootSize {
			return nil, encryption.ErrFormat
		}
		raw, e := readExact(f, rootSize, 0)
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
		manifest, e := openRoot(raw, m, o.Limits)
		if e != nil {
			return nil, e
		}
		desc := h.Descriptor
		desc.Role = encryption.FileBase
		desc.RecordCount = uint64(desc.PageCount) + 2
		if manifest.BaseBytes != objectSize(Geometry{desc.PageSize, desc.PageCount}) {
			return nil, encryption.ErrFormat
		}
		expected := encryption.FileIdentity{DatabaseID: h.Epoch.DatabaseID, EpochID: h.Epoch.EpochID, FileID: manifest.BaseID, Role: encryption.FileBase}
		obj, e := openObject(ctx, s.basePath(manifest.BaseID), m, o.Limits, expected, &desc, &manifest.BaseHash, &manifest.PageOneHash)
		if e != nil {
			return nil, e
		}
		s.base = obj
		s.header = h
		s.rootBytes = raw
	}
	if err = s.Check(); err != nil {
		return nil, err
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
	if err := stableFile(s.lock, s.path+".gse-lock", s.lockInfo); err != nil {
		return err
	}
	if s.root == nil {
		if _, err := os.Lstat(s.path); !os.IsNotExist(err) {
			return ErrChanged
		}
		return nil
	}
	if err := stableFile(s.root, s.path, s.rootInfo); err != nil {
		return err
	}
	raw, err := readExact(s.root, rootSize, 0)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, s.rootBytes) {
		return encryption.ErrAuthentication
	}
	return stableFile(s.base.file, s.base.path, s.base.info)
}
func (s *Store) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	if err := s.Check(); err != nil {
		return nil, err
	}
	return s.base.readPage(ctx, n)
}
func (s *Store) Verify(ctx context.Context) error {
	if err := s.Check(); err != nil {
		return err
	}
	m, err := openRoot(s.rootBytes, s.master, s.opts.Limits)
	if err != nil {
		return err
	}
	desc := s.base.header.Descriptor
	o, err := openObject(ctx, s.base.path, s.master, s.opts.Limits, s.base.header.Identity(), &desc, &m.BaseHash, &m.PageOneHash)
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
func writeSmall(path string, raw []byte) error {
	f, _, err := checkedFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return err
	}
	err = writeBytes(f, raw)
	if err == nil {
		err = f.Sync()
	}
	return errors.Join(err, f.Close())
}
func retain(path string, raw []byte) error {
	err := writeSmall(path, raw)
	if !os.IsExist(err) {
		return err
	}
	f, info, e := checkedFile(path, os.O_RDONLY)
	if e != nil {
		return e
	}
	defer f.Close()
	if info.Size() != int64(len(raw)) {
		return ErrChanged
	}
	b, e := readExact(f, len(raw), 0)
	if e != nil {
		return e
	}
	if !bytes.Equal(b, raw) {
		return ErrChanged
	}
	return nil
}

// Publish creates and verifies new immutable objects before replacing the only
// authoritative root. Errors after the rename attempt poison this session and
// preserve both generations. No crash/fault durability qualification is implied.
func (s *Store) Publish(ctx context.Context, g Geometry, src PageSource, target *Prepared) error {
	if s.readOnly {
		return ErrReadOnly
	}
	if err := s.Check(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	master, opts := s.master, s.opts
	if target != nil {
		master, opts = target.Master, target.Options
	}
	if master == nil {
		return encryption.ErrClosed
	}
	geometryDescriptor := encryption.FileDescriptor{Role: encryption.FileBase, PageSize: g.PageSize, PageCount: g.PageCount, RecordCount: uint64(g.PageCount) + 2}
	if err := geometryDescriptor.Validate(opts.Limits); err != nil {
		return err
	}
	seq := s.header.Descriptor.CommitSequence
	if seq == math.MaxUint64 {
		return encryption.ErrLimit
	}
	seq++
	if err := publicationBudget(ctx, s.path, opts, g, seq, s.root != nil); err != nil {
		return err
	}
	var txid, parent [32]byte
	if _, err := rand.Read(txid[:]); err != nil {
		return encryption.ErrRandomness
	}
	if s.root != nil {
		parent = sha256.Sum256(s.rootBytes)
	}
	desc := encryption.FileDescriptor{Role: encryption.FileBase, PageSize: g.PageSize, PageCount: g.PageCount, RecordCount: uint64(g.PageCount) + 2, CommitSequence: seq, TransactionID: txid, ParentCommitID: parent}
	obj, err := writeObject(ctx, s.basePath, master, opts.Limits, desc, src)
	if err != nil {
		return err
	}
	adopted := false
	defer func() {
		if !adopted {
			_ = obj.close()
		}
	}()
	if err = syncDirectory(s.path); err != nil {
		return err
	}
	m := manifest{BaseID: obj.header.FileID, BaseBytes: objectSize(g), BaseHash: obj.hash, PageOneHash: obj.pageOneHash}
	desc.Role = encryption.FileRoot
	desc.RecordCount = 1
	w, err := encryption.NewFileWriter(master, desc, opts.Limits)
	if err != nil {
		return err
	}
	defer w.Close()
	h, err := w.Header()
	if err != nil {
		return err
	}
	raw, err := encryption.EncodeHeader(h, opts.Limits)
	if err != nil {
		return err
	}
	record, err := w.Seal(encryption.RecordMetadata, 0, 0, m.encode())
	if err != nil {
		return err
	}
	raw = append(raw, record...)
	if err = w.Finish(); err != nil {
		return err
	}
	pending := s.path + ".gse-" + idString(h.FileID) + ".pending"
	if err = writeSmall(pending, raw); err != nil {
		return err
	}
	check, info, err := checkedFile(pending, os.O_RDONLY)
	if err != nil {
		return err
	}
	verified, err := readExact(check, rootSize, 0)
	if err == nil && info.Size() != rootSize {
		err = encryption.ErrFormat
	}
	if err == nil && !bytes.Equal(verified, raw) {
		err = ErrChanged
	}
	if err == nil {
		_, err = openRoot(verified, master, opts.Limits)
	}
	err = errors.Join(err, check.Close())
	if err != nil {
		return err
	}
	if s.root != nil {
		if err = retain(s.path+".gse-"+idString(s.header.FileID)+".retained", s.rootBytes); err != nil {
			return err
		}
		if err = syncDirectory(s.path); err != nil {
			return err
		}
	}
	if err = s.Check(); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	uncertain := func(e error) error { s.poisoned = errors.Join(ErrUncertain, e); return s.poisoned }
	// From this point cancellation cannot abandon a namespace publication.
	if s.root == nil {
		err = renameAbsent(pending, s.path)
	} else {
		err = os.Rename(pending, s.path)
	}
	if err != nil {
		return uncertain(err)
	}
	if err = syncDirectory(s.path); err != nil {
		return uncertain(err)
	}
	root, rootInfo, err := checkedFile(s.path, os.O_RDONLY)
	if err != nil {
		return uncertain(err)
	}
	final, err := readExact(root, rootSize, 0)
	if err != nil || rootInfo.Size() != rootSize || !bytes.Equal(final, raw) {
		_ = root.Close()
		if err == nil {
			err = ErrChanged
		}
		return uncertain(err)
	}
	oldRoot, oldBase, oldMaster := s.root, s.base, s.master
	s.root, s.rootInfo, s.rootBytes, s.header, s.base, s.master, s.opts = root, rootInfo, raw, h, obj, master, opts
	adopted = true
	if target != nil {
		target.Master = nil
	}
	// Closing read-only obsolete descriptors does not undo a durable commit.
	if oldRoot != nil {
		_ = oldRoot.Close()
	}
	if oldBase != nil {
		_ = oldBase.close()
	}
	if oldMaster != nil && oldMaster != master {
		_ = oldMaster.Close()
	}
	return nil
}
func (s *Store) Close() error {
	var err error
	if s.root != nil {
		err = errors.Join(err, s.root.Close())
		s.root = nil
	}
	if s.base != nil {
		err = errors.Join(err, s.base.close())
		s.base = nil
	}
	if s.master != nil {
		err = errors.Join(err, s.master.Close())
		s.master = nil
	}
	if s.lock != nil {
		err = errors.Join(err, s.lock.Close())
		s.lock = nil
	}
	return errors.Join(s.poisoned, err)
}

type Info struct {
	PageSize, PageCount uint32
	DatabaseID          [16]byte
	CommitSequence      uint64
	RetainsHistory      bool
}

func (s *Store) Info() Info {
	return Info{PageSize: s.header.Descriptor.PageSize, PageCount: s.header.Descriptor.PageCount, DatabaseID: s.header.Epoch.DatabaseID, CommitSequence: s.header.Descriptor.CommitSequence, RetainsHistory: true}
}
