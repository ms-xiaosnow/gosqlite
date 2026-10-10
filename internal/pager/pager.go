package pager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Reader owns a read-only descriptor and a SQLite-compatible shared lock.
// It has no write method, page cache, journal recovery, or WAL interpretation.
// Concurrent ReadPage and Close are safe; each page returned is caller-owned.
type Reader struct {
	mu        sync.RWMutex
	file      *os.File
	path      string
	info      os.FileInfo
	header    Header
	rawHeader [100]byte
}

func Open(path string) (*Reader, error) {
	// Resolve symlinks so sidecar checks apply to the actual database pathname.
	// Multiple hard-linked names remain unsupported by SQLite itself.
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			file.Close()
		}
	}()
	info, err := file.Stat()
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
	if _, err = io.ReadFull(io.NewSectionReader(file, 0, 100), r.rawHeader[:]); err != nil {
		return nil, fmt.Errorf("%w: header: %v", ErrCorrupt, err)
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

func checkSidecars(path string) error {
	for _, suffix := range []string{"-wal", "-journal"} {
		st, err := os.Stat(path + suffix)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect SQLite sidecar: %w", err)
		}
		// Conservative: even an inactive nonempty persistent journal is refused.
		// Proving it is cold belongs to the later recovery implementation.
		if !st.Mode().IsRegular() || st.Size() != 0 {
			return fmt.Errorf("%w: nonempty %s sidecar (recovery unsupported)", ErrUnsupported, suffix)
		}
	}
	return nil
}

func (r *Reader) Header() Header { return r.header }

func (r *Reader) checkStable() error {
	if err := checkSidecars(r.path); err != nil {
		return err
	}
	st, err := r.file.Stat()
	if err != nil {
		return err
	}
	if st.Size() != r.info.Size() || !st.ModTime().Equal(r.info.ModTime()) {
		return ErrChanged
	}
	pathInfo, err := os.Stat(r.path)
	if err != nil {
		return fmt.Errorf("%w: database path: %v", ErrChanged, err)
	}
	if !os.SameFile(st, pathInfo) {
		return ErrChanged
	}
	var raw [100]byte
	if _, err = r.file.ReadAt(raw[:], 0); err != nil {
		return fmt.Errorf("%w: header: %v", ErrChanged, err)
	}
	if !bytes.Equal(raw[:], r.rawHeader[:]) {
		return ErrChanged
	}
	return nil
}

func (r *Reader) ReadPage(ctx context.Context, pageNo uint32) ([]byte, error) {
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
	if _, err := r.file.ReadAt(data, int64(pageNo-1)*int64(r.header.PageSize)); err != nil {
		return nil, fmt.Errorf("%w: page %d: %v", ErrCorrupt, pageNo, err)
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
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	return err
}
