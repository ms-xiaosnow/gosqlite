package vfs

import "math"

// Mapping owns the entire native view while exposing only its logical span.
// Bytes is invalid after a successful Close. Callers serialize mapped access
// with Close; the file owner keeps DMS held while any view remains live.
type Mapping struct {
	Bytes  []byte
	owner  *File
	native nativeMapping
}

// mappingSpan accepts logical offsets independently of native granularity.
// In particular, a 32 KiB SQLite block need not extend to a 64 KiB view unit.
func mappingSpan(off int64, n int, granularity int64) (base int64, delta, length int, err error) {
	if off < 0 || n <= 0 || granularity <= 0 || granularity&(granularity-1) != 0 || off > math.MaxInt64-int64(n) {
		return 0, 0, 0, ErrUnsupported
	}
	base = off / granularity * granularity
	d := off - base
	if d > int64(math.MaxInt)-int64(n) {
		return 0, 0, 0, ErrUnsupported
	}
	return base, int(d), int(d) + n, nil
}
func (f *File) MapShared(off int64, n int) (*Mapping, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return nil, err
	}
	if f.role != SHM || !f.dms {
		return nil, ErrLock
	}
	base, delta, length, err := mappingSpan(off, n, nativeMapGranularity())
	if err != nil {
		return nil, err
	}
	info, err := f.raw.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() < off+int64(n) {
		return nil, ErrIdentity
	}
	mapped, bytes, err := nativeMap(f, base, length)
	if err != nil {
		return nil, err
	}
	m := &Mapping{Bytes: bytes[delta : delta+n], owner: f, native: mapped}
	f.maps[m] = struct{}{}
	return m, nil
}
func (m *Mapping) closeLocked() error { return m.retireLocked(nativeUnmap) }

// retireLocked also admits a memory-only backend model in ownership tests.
// Production always supplies nativeUnmap, never an injectable I/O hook.
func (m *Mapping) retireLocked(unmap func(*nativeMapping) error) error {
	if m.Bytes == nil {
		return nil
	}
	if err := unmap(&m.native); err != nil {
		return err
	}
	m.Bytes = nil
	delete(m.owner.maps, m)
	return nil
}
func (m *Mapping) Close() error {
	m.owner.mu.Lock()
	defer m.owner.mu.Unlock()
	return m.closeLocked()
}
