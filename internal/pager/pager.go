package pager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// Reader owns either a read-only descriptor with a SQLite-compatible SHARED
// lock, or a read-only lease on a WAL session. It has no write
// method, page cache, or rollback-journal recovery.
// Concurrent ReadPage and Close are safe; each page returned is caller-owned.
type Reader struct {
	mu        sync.RWMutex
	file      *vfs.File
	path      string
	info      os.FileInfo
	header    Header
	rawHeader [100]byte
	lease     *Tx
}

func Open(path string) (result *Reader, retErr error) {
	// Resolve symlinks so sidecar checks apply to the actual database pathname.
	// Multiple hard-linked names remain unsupported by SQLite itself.
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	path, err = resolveNativePath(path)
	if err != nil {
		return nil, err
	}
	file, err := openReadOnlyDatabase(path)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			retErr = errors.Join(retErr, closeNativeFile(file))
		}
	}()
	info, err := statNativeFile(file)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: database must be a regular file", ErrUnsupported)
	}
	if err = lockShared(file); err != nil {
		return nil, err
	}
	r := &Reader{file: file, path: path, info: info}
	if err = checkSidecars(path); err != nil {
		return nil, err
	}
	if _, err = io.ReadFull(io.NewSectionReader(nativeReaderAt{file}, 0, 100), r.rawHeader[:]); err != nil {
		return nil, fmt.Errorf("%w: header: %w", ErrCorrupt, err)
	}
	r.header, err = ParseHeader(r.rawHeader[:], info.Size())
	if err != nil {
		return nil, err
	}
	// Check again after acquiring the lock and reading the header. An already
	// reserved writer may create a journal while shared readers remain active.
	if err = r.checkStable(); err != nil {
		return nil, err
	}
	success = true
	return r, nil
}

// checkSidecars is shared by read-only probes and writable snapshots. Until
// recovery has an acceptance-verified implementation, every existing rollback
// journal (including a cold or empty one) fails closed with a distinct error.
func checkSidecars(path string) error {
	for _, suffix := range []string{"-wal", "-journal"} {
		st, err := nativeLstat(path + suffix)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect SQLite sidecar: %w", err)
		}
		if suffix == "-journal" {
			return fmt.Errorf("%w: existing journal; recovery is not yet acceptance-verified", ErrRecoveryRequired)
		}
		if !st.Mode().IsRegular() || st.Size() != 0 {
			return fmt.Errorf("%w: nonempty WAL sidecar", ErrUnsupported)
		}
	}
	return nil
}

func (r *Reader) Header() Header {
	if r.lease != nil {
		return r.lease.Header()
	}
	return r.header
}

func (r *Reader) checkStable() error {
	if err := validateNativePath(r.file); err != nil {
		return err
	}
	if err := checkSidecars(r.path); err != nil {
		return err
	}
	st, err := statNativeFile(r.file)
	if err != nil {
		return err
	}
	if st.Size() != r.info.Size() || !st.ModTime().Equal(r.info.ModTime()) {
		return ErrChanged
	}
	pathInfo, err := nativeLstat(r.path)
	if err != nil {
		return fmt.Errorf("%w: database path: %w", ErrChanged, err)
	}
	if !pathInfo.Mode().IsRegular() || !vfs.SameFile(st, pathInfo) {
		return ErrChanged
	}
	var raw [100]byte
	if _, err = readNativeAt(r.file, raw[:], 0); err != nil {
		return fmt.Errorf("%w: header: %w", ErrChanged, err)
	}
	if !bytes.Equal(raw[:], r.rawHeader[:]) {
		return ErrChanged
	}
	return nil
}

func (r *Reader) ReadPage(ctx context.Context, pageNo uint32) ([]byte, error) {
	if r.lease != nil {
		return r.lease.ReadPage(ctx, pageNo)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.file == nil {
		return nil, ErrClosed
	}
	if pageNo == 0 || pageNo > r.header.PageCount {
		return nil, fmt.Errorf("%w: page %d outside 1..%d", ErrCorrupt, pageNo, r.header.PageCount)
	}
	if err := r.checkStable(); err != nil {
		return nil, err
	}
	data := make([]byte, int(r.header.PageSize))
	if _, err := readNativeAt(r.file, data, int64(pageNo-1)*int64(r.header.PageSize)); err != nil {
		return nil, fmt.Errorf("%w: page %d: %w", ErrCorrupt, pageNo, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := r.checkStable(); err != nil {
		return nil, err
	}
	return data, nil
}

func (r *Reader) Close() error {
	if r.lease != nil {
		return r.lease.Rollback()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return nil
	}
	err := closeNativeFile(r.file)
	r.file = nil
	return err
}
