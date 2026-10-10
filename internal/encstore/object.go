package encstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

type object struct {
	ns          *namespace
	file        *os.File
	info        os.FileInfo
	path        string
	header      encryption.Header
	reader      *encryption.OpenedFile
	pageOneHash [32]byte
	hash        [32]byte
}

func checkedFile(path string, flags int) (*os.File, os.FileInfo, error) {
	f, err := openFile(path, flags)
	if err != nil {
		return nil, nil, err
	}
	st, err := f.Stat()
	if err == nil {
		err = validateFile(st)
	}
	if err == nil {
		err = validateLinks(f)
	}
	if err == nil {
		var named os.FileInfo
		named, err = os.Lstat(path)
		if err == nil && (!named.Mode().IsRegular() || !os.SameFile(st, named)) {
			err = ErrChanged
		}
	}
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	return f, st, nil
}
func stableFile(f *os.File, path string, info os.FileInfo) error {
	now, err := f.Stat()
	if err != nil {
		return err
	}
	named, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !named.Mode().IsRegular() || !os.SameFile(now, named) || !os.SameFile(info, now) || now.Size() != info.Size() || !now.ModTime().Equal(info.ModTime()) {
		return ErrChanged
	}
	return validateLinks(f)
}
func readExact(f io.ReaderAt, size int, off int64) ([]byte, error) {
	b := make([]byte, size)
	n, err := f.ReadAt(b, off)
	if err != nil || n != len(b) {
		return nil, encryption.ErrFormat
	}
	return b, nil
}
func writeBytes(w io.Writer, b []byte) error {
	n, err := w.Write(b)
	if err != nil {
		return err
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	return nil
}
func idString(id [32]byte) string { return hex.EncodeToString(id[:]) }
func (o *object) close() error {
	if o == nil {
		return nil
	}
	return errors.Join(o.reader.Close(), o.file.Close())
}
func (o *object) geometry() Geometry {
	return Geometry{o.header.Descriptor.PageSize, o.header.Descriptor.PageCount}
}
func (o *object) readPage(ctx context.Context, n uint32) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	g := o.geometry()
	if n == 0 || n > g.PageCount {
		return nil, encryption.ErrIdentity
	}
	if err := o.ns.stable(o.file, o.path, o.info); err != nil {
		return nil, err
	}
	b, err := readExact(o.file, int(g.PageSize)+recordOverhead, pageOffset(g, n))
	if err != nil {
		return nil, err
	}
	p, err := o.reader.Open(encryption.RecordIdentity{Role: encryption.RecordData, Sequence: uint64(n), PageNumber: n}, b)
	if err != nil {
		return nil, err
	}
	if n == lockPage(g.PageSize) && !zero(p) {
		clear(p)
		return nil, encryption.ErrFormat
	}
	if err = ctx.Err(); err != nil {
		clear(p)
		return nil, err
	}
	return p, nil
}

// openObject authenticates the complete bounded object before returning any
// logical page to a pager. Expected child identity/descriptor/hash come only
// from an authenticated manifest. An archive itself is an authentication root.
func openObject(ctx context.Context, path string, master *encryption.MasterKey, limits encryption.Limits, expected encryption.FileIdentity, desc *encryption.FileDescriptor, hash *[32]byte, p1 *[32]byte, spaces ...*namespace) (*object, error) {
	ns := chooseNamespace(spaces)
	f, info, err := ns.checked(path, os.O_RDONLY)
	if err != nil {
		return nil, err
	}
	o := &object{file: f, path: path, info: info, ns: ns}
	ok := false
	defer func() {
		if !ok {
			if o.reader != nil {
				_ = o.reader.Close()
			}
			_ = f.Close()
		}
	}()
	raw, err := readExact(f, encryption.HeaderSize, 0)
	if err != nil {
		return nil, err
	}
	h, err := encryption.DecodeHeader(raw, limits)
	if err != nil {
		return nil, err
	}
	if h.Identity() != expected || (desc != nil && h.Descriptor != *desc) || (h.Descriptor.Role != encryption.FileBase && h.Descriptor.Role != encryption.FileBackup) {
		return nil, encryption.ErrAuthentication
	}
	g := Geometry{h.Descriptor.PageSize, h.Descriptor.PageCount}
	if uint64(info.Size()) != objectSize(g) {
		return nil, encryption.ErrFormat
	}
	meta, err := readExact(f, metaSize+recordOverhead, encryption.HeaderSize)
	if err != nil {
		return nil, err
	}
	r, p, err := encryption.OpenFile(master, expected, raw, meta, limits)
	if err != nil {
		return nil, err
	}
	o.reader = r
	o.header = h
	o.pageOneHash, err = parseMetadata(p, h.Descriptor.Role)
	clear(p)
	if err != nil {
		return nil, err
	}
	if p1 != nil && o.pageOneHash != *p1 {
		return nil, encryption.ErrAuthentication
	}
	prefix := sha256.New()
	_, _ = prefix.Write(raw)
	_, _ = prefix.Write(meta)
	for n := uint32(1); n <= g.PageCount; n++ {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		b, e := readExact(f, int(g.PageSize)+recordOverhead, pageOffset(g, n))
		if e != nil {
			return nil, e
		}
		_, _ = prefix.Write(b)
		page, e := r.Open(encryption.RecordIdentity{Role: encryption.RecordData, Sequence: uint64(n), PageNumber: n}, b)
		if e != nil {
			return nil, e
		}
		valid := true
		if n == 1 && sha256.Sum256(page) != o.pageOneHash {
			valid = false
		}
		if n == lockPage(g.PageSize) && !zero(page) {
			valid = false
		}
		clear(page)
		if !valid {
			return nil, encryption.ErrAuthentication
		}
	}
	var prefixHash [32]byte
	copy(prefixHash[:], prefix.Sum(nil))
	foot, err := readExact(f, footerSize+recordOverhead, int64(objectSize(g))-footerSize-recordOverhead)
	if err != nil {
		return nil, err
	}
	plain, err := r.Open(encryption.RecordIdentity{Role: encryption.RecordFooter, Sequence: uint64(g.PageCount) + 1}, foot)
	if err != nil {
		return nil, err
	}
	valid := string(plain) == string(footer(prefixHash, objectSize(g)))
	clear(plain)
	if !valid {
		return nil, encryption.ErrAuthentication
	}
	_, _ = prefix.Write(foot)
	copy(o.hash[:], prefix.Sum(nil))
	if hash != nil && o.hash != *hash {
		return nil, encryption.ErrAuthentication
	}
	if err = ns.stable(f, path, info); err != nil {
		return nil, err
	}
	ok = true
	return o, nil
}

// writeObject streams bounded page buffers. A new primitive writer supplies a
// fresh identity for every attempt; existing physical files are never resumed.
func writeObject(ctx context.Context, pathForID func([32]byte) string, master *encryption.MasterKey, limits encryption.Limits, desc encryption.FileDescriptor, src PageSource, spaces ...*namespace) (*object, error) {
	ns := chooseNamespace(spaces)
	if err := desc.Validate(limits); err != nil {
		return nil, err
	}
	first, err := src.ReadPage(ctx, 1)
	if err != nil {
		return nil, err
	}
	if len(first) != int(desc.PageSize) {
		clear(first)
		return nil, encryption.ErrFormat
	}
	p1 := sha256.Sum256(first)
	clear(first)
	w, err := encryption.NewFileWriter(master, desc, limits)
	if err != nil {
		return nil, err
	}
	defer w.Close()
	h, err := w.Header()
	if err != nil {
		return nil, err
	}
	path := pathForID(h.FileID)
	f, _, err := ns.checked(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return nil, err
	}
	closed := false
	defer func() {
		if !closed {
			_ = f.Close()
		}
	}()
	digest := sha256.New()
	out := io.MultiWriter(f, digest)
	raw, err := encryption.EncodeHeader(h, limits)
	if err != nil {
		return nil, err
	}
	if err = writeBytes(out, raw); err != nil {
		return nil, err
	}
	b, err := w.Seal(encryption.RecordMetadata, 0, 0, metadata(desc.Role, p1))
	if err != nil {
		return nil, err
	}
	if err = writeBytes(out, b); err != nil {
		return nil, err
	}
	for n := uint32(1); n <= desc.PageCount; n++ {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		var page []byte
		if n == lockPage(desc.PageSize) {
			page = make([]byte, desc.PageSize)
		} else {
			page, err = src.ReadPage(ctx, n)
			if err != nil {
				return nil, err
			}
		}
		if len(page) != int(desc.PageSize) {
			clear(page)
			return nil, encryption.ErrFormat
		}
		if n == 1 && sha256.Sum256(page) != p1 {
			clear(page)
			return nil, ErrChanged
		}
		b, err = w.Seal(encryption.RecordData, n, 0, page)
		clear(page)
		if err != nil {
			return nil, err
		}
		if err = writeBytes(out, b); err != nil {
			return nil, err
		}
	}
	var prefix [32]byte
	copy(prefix[:], digest.Sum(nil))
	g := Geometry{desc.PageSize, desc.PageCount}
	b, err = w.Seal(encryption.RecordFooter, 0, 0, footer(prefix, objectSize(g)))
	if err != nil {
		return nil, err
	}
	if err = writeBytes(out, b); err != nil {
		return nil, err
	}
	if err = w.Finish(); err != nil {
		return nil, err
	}
	if err = f.Sync(); err != nil {
		return nil, err
	}
	err = f.Close()
	closed = true
	if err != nil {
		return nil, err
	}
	var full [32]byte
	copy(full[:], digest.Sum(nil))
	return openObject(ctx, path, master, limits, h.Identity(), &desc, &full, &p1, ns)
}
