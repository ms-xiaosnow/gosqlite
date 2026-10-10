package encstore

import (
	"context"
	"errors"
	"os"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

// WriteBackup creates one self-contained authenticated archive. Existing
// destinations are never replaced; any partial encrypted output is retained.
func WriteBackup(ctx context.Context, destination string, target *Prepared, g Geometry, src PageSource) (_ Info, resultErr error) {
	if target.Options.Mode != Snapshot {
		return Info{}, ErrUnsupported
	}
	path, err := canonical(destination)
	if err != nil {
		return Info{}, err
	}
	if err = supported(); err != nil {
		return Info{}, err
	}
	d := encryption.FileDescriptor{Role: encryption.FileBackup, PageSize: g.PageSize, PageCount: g.PageCount, RecordCount: uint64(g.PageCount) + 2, CommitSequence: 1}
	if err = d.Validate(target.Options.Limits); err != nil {
		return Info{}, err
	}
	_, maxBytes, _, err := target.Options.retentionLimits()
	if err != nil {
		return Info{}, err
	}
	if _, err = charge(0, objectSize(g), maxBytes); err != nil {
		return Info{}, err
	}
	l, err := target.Options.WALLimits.normalized()
	if err != nil {
		return Info{}, err
	}
	if target.writerIDs >= encryption.MaxFilesPerMaster || (target.writerIDs+1)*128 > l.MaxMemoryBytes {
		return Info{}, encryption.ErrLimit
	}
	target.writerIDs++
	o, err := writeObject(ctx, func([32]byte) string { return path }, target.Master, target.Options.Limits, d, src)
	if err != nil {
		return Info{}, err
	}
	defer func() { resultErr = o.closeWithError(resultErr) }()
	if err = syncDirectory(path); err != nil {
		return Info{}, errors.Join(ErrUncertain, err)
	}
	return Info{PageSize: g.PageSize, PageCount: g.PageCount, DatabaseID: o.header.Epoch.DatabaseID, CommitSequence: 1}, nil
}

type Archive struct {
	obj     *object
	master  *encryption.MasterKey
	cleanup *cleanupGroup
}

func OpenBackup(ctx context.Context, path string, options Options) (*Archive, error) {
	if options.Mode != Snapshot {
		return nil, ErrUnsupported
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	path, err := canonical(path)
	if err != nil {
		return nil, err
	}
	f, _, err := checkedFile(path, os.O_RDONLY)
	if err != nil {
		return nil, err
	}
	raw, err := readExact(f, encryption.HeaderSize, 0)
	err = closeResult(f, err)
	if err != nil {
		return nil, err
	}
	h, err := encryption.DecodeHeader(raw, options.Limits)
	if err != nil {
		return nil, err
	}
	if h.Descriptor.Role != encryption.FileBackup || h.Epoch.Kind != options.CredentialKind {
		return nil, encryption.ErrAuthentication
	}
	master, err := encryption.DeriveMaster(ctx, options.KeyProvider, encryption.KeyRequest{Operation: encryption.OperationOpen, Path: path}, h.Epoch, options.KDFPolicy, options.DerivationBudget)
	if err != nil {
		return nil, err
	}
	o, err := openObject(ctx, path, master, options.Limits, h.Identity(), &h.Descriptor, nil, nil)
	if err != nil {
		_ = master.Close()
		return nil, err
	}
	return &Archive{obj: o, master: master, cleanup: o.file.owner}, nil
}
func (a *Archive) Geometry() Geometry {
	if a.obj == nil {
		return Geometry{}
	}
	return a.obj.geometry()
}
func (a *Archive) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	if a.obj == nil {
		return nil, encryption.ErrClosed
	}
	return a.obj.readPage(ctx, n)
}
func (a *Archive) Close() error { return a.CloseWithError(nil) }
func (a *Archive) CloseWithError(cause error) error {
	if a.obj == nil && a.cleanup != nil {
		return a.cleanup.retry()
	}
	var err error
	if a.cleanup != nil {
		cause = a.cleanup.remember(cause)
	}
	if a.obj != nil {
		err = a.obj.closeWithError(cause)
		cause = nil
		a.obj = nil
	}
	if a.master != nil {
		err = errors.Join(err, a.master.Close())
		a.master = nil
	}
	if a.cleanup != nil {
		return a.cleanup.closeAll(errors.Join(cause, err))
	}
	return errors.Join(cause, err)
}

// CreateFrom makes an absent destination from a validated pinned source. The
// caller must validate logical SQLite structure before calling this routine.
func CreateFrom(ctx context.Context, destination string, target *Prepared, g Geometry, src PageSource) (_ Info, resultErr error) {
	if err := ctx.Err(); err != nil {
		return Info{}, err
	}
	d := encryption.FileDescriptor{Role: encryption.FileBase, PageSize: g.PageSize, PageCount: g.PageCount, RecordCount: uint64(g.PageCount) + 2}
	if err := d.Validate(target.Options.Limits); err != nil {
		return Info{}, err
	}
	path, err := canonical(destination)
	if err != nil {
		return Info{}, err
	}
	if err = publicationBudget(ctx, path, target.Options, g, 1, false); err != nil {
		return Info{}, err
	}
	s, absent, err := acquire(path, false, true, target.Options.Mode)
	if err != nil {
		return Info{}, err
	}
	defer func() { resultErr = s.CloseWithError(resultErr) }()
	if !absent {
		return Info{}, os.ErrExist
	}
	s.opts = target.Options
	if err = s.Publish(ctx, g, src, target); err != nil {
		return Info{}, err
	}
	return s.Info(), nil
}

// ExportPlaintext is the sole explicit confidentiality-downgrade sink. No
// staging file is used. A partial target, if any, is deliberately left at the
// named destination; success requires file and directory Sync.
func ExportPlaintext(ctx context.Context, destination string, g Geometry, src PageSource) (resultErr error) {
	path, err := canonical(destination)
	if err != nil {
		return err
	}
	if err = supported(); err != nil {
		return err
	}
	f, _, err := checkedFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			resultErr = closeResult(f, resultErr)
		}
	}()
	for n := uint32(1); n <= g.PageCount; n++ {
		if err = ctx.Err(); err != nil {
			return err
		}
		var p []byte
		if n == lockPage(g.PageSize) {
			p = make([]byte, g.PageSize)
		} else {
			p, err = src.ReadPage(ctx, n)
			if err != nil {
				return err
			}
		}
		if len(p) != int(g.PageSize) {
			clear(p)
			return encryption.ErrFormat
		}
		err = writeBytes(f, p)
		clear(p)
		if err != nil {
			return err
		}
	}
	if err = f.Sync(); err != nil {
		return err
	}
	err = f.Close()
	closed = true
	if err != nil {
		return err
	}
	if err = syncDirectory(path); err != nil {
		return errors.Join(ErrUncertain, err)
	}
	return nil
}
