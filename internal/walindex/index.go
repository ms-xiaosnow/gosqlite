package walindex

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// Index owns a distinct open file description and its lifetime DMS shared
// lock. Methods serialize local access, including Close, but callers own
// transaction-level locking. A slot acquisition is idempotent, not recursive;
// changing a held slot's mode requires an explicit Unlock first.
// Cooperating SQLite clients must not truncate/unlink a live SHM file. Identity
// and size checks reject a detected replacement/shrink, but cannot make mmap
// safe against a malicious process truncating it concurrently with a load.
type Index struct {
	mu        sync.Mutex
	file      *vfs.File
	path      string
	identity  os.FileInfo
	maps      map[int64]*vfs.Mapping
	mapSize   int
	mappedEnd int64
	maxFrames uint32
	held      [8]uint8 // 0=unlocked, 1=shared, 2=exclusive
	closed    bool
}

func Open(path string) (*Index, error) { return OpenWithLimit(path, DefaultMaxFrames) }

// OpenWithLimit takes a SHM pathname (normally databasePath+"-shm"), not a
// database pathname. Zero selects DefaultMaxFrames. It acquires the DMS lock
// using SQLite's first-opener protocol without extending or mapping the file.
func OpenWithLimit(path string, maxFrames uint32) (result *Index, retErr error) {
	if maxFrames == 0 {
		maxFrames = DefaultMaxFrames
	}
	p, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	// Resolve directory aliases only. A final-component symlink is forbidden.
	dir, err := vfs.ResolvePath(filepath.Dir(p))
	if err != nil {
		return nil, nativeError(err)
	}
	p = filepath.Join(dir, filepath.Base(p))
	f, err := openFile(p)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			retErr = errors.Join(retErr, nativeError(f.Close()))
		}
	}()
	unit := BlockSize
	if size := os.Getpagesize(); size > unit {
		unit = size
	}
	if unit%BlockSize != 0 || unit&(unit-1) != 0 {
		return nil, ErrUnsupported
	}
	x := &Index{file: f, path: p, maps: make(map[int64]*vfs.Mapping), mapSize: unit, maxFrames: maxFrames}
	info, err := x.statLocked()
	if err != nil {
		return nil, err
	}
	x.identity = info
	if err = nativeError(f.AttachDMS()); err != nil {
		return nil, err
	}
	if _, err = x.statLocked(); err != nil {
		return nil, err
	}
	ok = true
	return x, nil
}

func (x *Index) statLocked() (os.FileInfo, error) {
	if x.closed || x.file == nil {
		return nil, ErrClosed
	}
	if err := nativeError(x.file.ValidatePath()); err != nil {
		return nil, err
	}
	info, err := x.file.Stat()
	if err != nil {
		return nil, nativeError(err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: sidecar is not a regular file", ErrIdentity)
	}
	if err = nativeError(x.file.ValidateSingleLink()); err != nil {
		return nil, err
	}
	named, err := vfs.Lstat(x.path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIdentity, nativeError(err))
	}
	if named.Mode()&os.ModeSymlink != 0 || !vfs.SameFile(info, named) || (x.identity != nil && !vfs.SameFile(info, x.identity)) {
		return nil, fmt.Errorf("%w: sidecar pathname no longer names the opened file", ErrIdentity)
	}
	if info.Size() < x.mappedEnd {
		return nil, fmt.Errorf("%w: mapped sidecar shrank", ErrIdentity)
	}
	return info, nil
}

func (x *Index) ValidateIdentity() error {
	x.mu.Lock()
	defer x.mu.Unlock()
	_, err := x.statLocked()
	return err
}

func (x *Index) Close() error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.file == nil {
		return nil
	}
	x.closed = true
	// The owner retains all failed native mappings and DMS on an unmap error.
	// Retrying Close is permitted; reads and further lock acquisitions are not.
	err := x.file.Close()
	if x.file.RetirementComplete() {
		x.maps = nil
		x.file = nil
	}
	return nativeError(err)
}

// RetirementComplete distinguishes a retired owner with a diagnostic error
// from retained views, DMS, or terminal file ownership.
func (x *Index) RetirementComplete() bool {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.file == nil
}

func validRange(slot, n int) bool { return slot >= 0 && n > 0 && slot < 8 && n <= 8-slot }
func (x *Index) Lock(slot, n int, exclusive bool) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if !validRange(slot, n) || (!exclusive && n != 1) {
		return fmt.Errorf("%w: invalid lock range", ErrLock)
	}
	if _, err := x.statLocked(); err != nil {
		return err
	}
	if err := nativeError(x.file.LockSHM(slot, n, exclusive)); err != nil {
		return err
	}
	mode := uint8(1)
	if exclusive {
		mode = 2
	}
	for i := slot; i < slot+n; i++ {
		x.held[i] = mode
	}
	return nil
}
func (x *Index) Unlock(slot, n int) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if !validRange(slot, n) {
		return fmt.Errorf("%w: invalid unlock range", ErrLock)
	}
	// Unlock remains possible after a detected pathname replacement or shrink.
	if x.closed || x.file == nil {
		return ErrClosed
	}
	if err := nativeError(x.file.UnlockSHM(slot, n)); err != nil {
		return err
	}
	for i := slot; i < slot+n; i++ {
		x.held[i] = 0
	}
	return nil
}
func (x *Index) writerLocked() error {
	if x.closed {
		return ErrClosed
	}
	if x.held[WriteLock] != 2 {
		return ErrLock
	}
	return nil
}
func (x *Index) checkFrame(frame uint32) error {
	if frame == 0 || frame > x.maxFrames {
		return fmt.Errorf("%w: frame %d", ErrLimit, frame)
	}
	return nil
}

// blockLocked maps an existing unit or allocates it only under this handle's
// writer lock. Page-tail writes allocate backing storage before mmap. We never
// relocate live mappings; the local mutex prevents Close racing any access.
func (x *Index) blockLocked(block uint32, extend bool) ([]byte, error) {
	if block > frameBlock(x.maxFrames) {
		return nil, ErrLimit
	}
	info, err := x.statLocked()
	if err != nil {
		return nil, err
	}
	off := int64(block) * BlockSize
	base := off / int64(x.mapSize) * int64(x.mapSize)
	end := base + int64(x.mapSize)
	if b, ok := x.maps[base]; ok {
		delta := int(off - base)
		return b.Bytes[delta : delta+BlockSize], nil
	}
	if info.Size() < end {
		if !extend {
			return nil, ErrUninitialized
		}
		if err = x.writerLocked(); err != nil {
			return nil, err
		}
		page := int64(os.Getpagesize())
		var zero [1]byte
		for tail := (info.Size()/page+1)*page - 1; tail < end; tail += page {
			n, e := x.file.WriteAt(zero[:], tail)
			if e != nil {
				return nil, fmt.Errorf("extend WAL-index: %w", nativeError(e))
			}
			if n != 1 {
				return nil, fmt.Errorf("extend WAL-index: %w", io.ErrShortWrite)
			}
		}
		if info, err = x.statLocked(); err != nil {
			return nil, err
		}
		if info.Size() < end {
			return nil, fmt.Errorf("%w: sidecar shrank during extension", ErrIdentity)
		}
	}
	b, err := x.file.MapShared(base, x.mapSize)
	if err != nil {
		return nil, fmt.Errorf("map WAL-index: %w", nativeError(err))
	}
	x.maps[base] = b
	if end > x.mappedEnd {
		x.mappedEnd = end
	}
	delta := int(off - base)
	return b.Bytes[delta : delta+BlockSize], nil
}

func load32(b []byte, off int) uint32     { return atomic.LoadUint32((*uint32)(unsafe.Pointer(&b[off]))) }
func store32(b []byte, off int, v uint32) { atomic.StoreUint32((*uint32)(unsafe.Pointer(&b[off])), v) }

var nativeLittleEndian = binary.NativeEndian.Uint16([]byte{1, 0}) == 1

func hashWord(slot int) (off int, shift uint) {
	off = BlockSize/2 + (slot&^1)*2
	shift = uint(slot&1) * 16
	if !nativeLittleEndian {
		shift = 16 - shift
	}
	return
}
func loadHash(b []byte, slot int) uint32 {
	off, shift := hashWord(slot)
	return (load32(b, off) >> shift) & 0xffff
}
func storeHash(b []byte, slot int, value uint32) {
	off, shift := hashWord(slot)
	p := (*uint32)(unsafe.Pointer(&b[off]))
	mask := uint32(0xffff) << shift
	for {
		old := atomic.LoadUint32(p)
		next := (old &^ mask) | (value << shift)
		if atomic.CompareAndSwapUint32(p, old, next) {
			return
		}
	}
}

func (x *Index) ReadHeader() (Header, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	b, err := x.blockLocked(0, false)
	if err != nil {
		return Header{}, err
	}
	var copies [2][HeaderSize]byte
	// Readers copy 0 then 1. Sequentially consistent atomic loads supply the
	// intervening memory barrier; writers publish the copies in reverse order.
	for copyNo := 0; copyNo < 2; copyNo++ {
		for off := 0; off < HeaderSize; off += 4 {
			binary.NativeEndian.PutUint32(copies[copyNo][off:], load32(b, copyNo*HeaderSize+off))
		}
	}
	if copies[0] != copies[1] {
		return Header{}, fmt.Errorf("%w: header changed while reading", ErrHeader)
	}
	h, err := ParseHeader(copies[0][:])
	if err != nil {
		return Header{}, err
	}
	if h.MaxFrame > x.maxFrames {
		return Header{}, ErrLimit
	}
	return h, nil
}

// Publish installs a committed header in SQLite's second-then-first order.
// WAL content and hash entries must already be complete and durable as required
// by the caller's synchronization policy. No SHM fsync is needed or performed.
func (x *Index) Publish(h Header) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if err := x.writerLocked(); err != nil {
		return err
	}
	if h.MaxFrame > x.maxFrames {
		return ErrLimit
	}
	h.Version = Version
	encoded, err := EncodeHeader(h)
	if err != nil {
		return err
	}
	b, err := x.blockLocked(0, true)
	if err != nil {
		return err
	}
	for copyNo := 1; copyNo >= 0; copyNo-- {
		for off := 0; off < HeaderSize; off += 4 {
			store32(b, copyNo*HeaderSize+off, binary.NativeEndian.Uint32(encoded[off:]))
		}
	}
	return nil
}

// InitCheckpoint initializes recovery checkpoint metadata. The caller must
// hold all eight slots exclusively; this conservative requirement excludes any
// live snapshot while its read marks are being replaced. Lock bytes 120..127
// are never read or written as data.
func (x *Index) InitCheckpoint(mx uint32) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if err := x.writerLocked(); err != nil {
		return err
	}
	if mx > x.maxFrames {
		return ErrLimit
	}
	for _, mode := range x.held {
		if mode != 2 {
			return ErrLock
		}
	}
	b, err := x.blockLocked(0, true)
	if err != nil {
		return err
	}
	store32(b, 96, 0)
	store32(b, 128, mx)
	store32(b, 132, 0)
	store32(b, 100, 0)
	for mark := 1; mark < ReaderCount; mark++ {
		v := UnusedReadMark
		if mark == 1 && mx > 0 {
			v = mx
		}
		store32(b, 100+mark*4, v)
	}
	return nil
}
func (x *Index) value(off int) (uint32, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	b, err := x.blockLocked(0, false)
	if err != nil {
		return 0, err
	}
	v := load32(b, off)
	isReadMark := off >= 100 && off < 120
	if v > x.maxFrames && !(isReadMark && v == UnusedReadMark) {
		return 0, ErrLimit
	}
	return v, nil
}
func (x *Index) Backfill() (uint32, error)  { return x.value(96) }
func (x *Index) Attempted() (uint32, error) { return x.value(128) }
func (x *Index) ReadMark(mark int) (uint32, error) {
	if mark < 0 || mark >= ReaderCount {
		return 0, ErrLock
	}
	return x.value(100 + mark*4)
}
func (x *Index) setCheckpoint(off int, value uint32) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.closed {
		return ErrClosed
	}
	if x.held[CheckpointLock] != 2 && x.held[WriteLock] != 2 {
		return ErrLock
	}
	if value > x.maxFrames {
		return ErrLimit
	}
	b, err := x.blockLocked(0, false)
	if err != nil {
		return err
	}
	store32(b, off, value)
	return nil
}

// SetBackfill requires the checkpoint lock, or the writer lock for a reset.
// The caller must enforce monotonicity and the committed-header upper bound.
func (x *Index) SetBackfill(value uint32) error  { return x.setCheckpoint(96, value) }
func (x *Index) SetAttempted(value uint32) error { return x.setCheckpoint(128, value) }
func (x *Index) SetReadMark(mark int, value uint32) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.closed {
		return ErrClosed
	}
	if mark < 0 || mark >= ReaderCount || x.held[ReadLock0+mark] != 2 {
		return ErrLock
	}
	if mark == 0 && value != 0 {
		return ErrHeader
	}
	if value > x.maxFrames && value != UnusedReadMark {
		return ErrLimit
	}
	b, err := x.blockLocked(0, false)
	if err != nil {
		return err
	}
	store32(b, 100+mark*4, value)
	return nil
}

func clearBlock(b []byte, block uint32) {
	start := 0
	if block == 0 {
		start = FirstHeaderSize
	}
	for off := start; off < BlockSize; off += 4 {
		store32(b, off, 0)
	}
}
func cleanupBlock(b []byte, block, limit uint32) {
	for slot := 0; slot < HashSlots; slot++ {
		if loadHash(b, slot) > limit {
			storeHash(b, slot, 0)
		}
	}
	for relative := limit + 1; relative <= blockCapacity(block); relative++ {
		store32(b, pageOffset(block, relative), 0)
	}
}

// Cleanup discards indexed entries after mx in its final block under the
// writer lock. Later blocks are cleared when Append reaches their first frame.
// The caller must ensure no live reader relies on frames being discarded.
func (x *Index) Cleanup(mx uint32) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if err := x.writerLocked(); err != nil {
		return err
	}
	if mx == 0 {
		_, err := x.statLocked()
		return err
	}
	if err := x.checkFrame(mx); err != nil {
		return err
	}
	block := frameBlock(mx)
	b, err := x.blockLocked(block, false)
	if err != nil {
		return err
	}
	cleanupBlock(b, block, mx-blockBase(block))
	return nil
}

// Append adds a 1-based frame and its database page to the real SQLite hash
// index. Frames must be appended sequentially. It does not publish MaxFrame.
// Starting a block clears that block. Reusing an occupied frame discards its
// uncommitted suffix, so caller locking must exclude snapshots using it.
func (x *Index) Append(frame, page uint32) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if err := x.writerLocked(); err != nil {
		return err
	}
	if err := x.checkFrame(frame); err != nil {
		return err
	}
	if page == 0 || page > 0xfffffffe {
		return ErrHeader
	}
	block := frameBlock(frame)
	relative := frame - blockBase(block)
	b, err := x.blockLocked(block, true)
	if err != nil {
		return err
	}
	if relative == 1 {
		clearBlock(b, block)
	} else if load32(b, pageOffset(block, relative-1)) == 0 {
		return fmt.Errorf("%w: frame append gap", ErrHeader)
	}
	off := pageOffset(block, relative)
	if load32(b, off) != 0 {
		cleanupBlock(b, block, relative-1)
	}
	key := int((page * 383) & (HashSlots - 1))
	for collisions := 0; collisions < HashSlots; collisions++ {
		if loadHash(b, key) == 0 {
			store32(b, off, page)
			storeHash(b, key, relative)
			return nil
		}
		key = (key + 1) & (HashSlots - 1)
	}
	return fmt.Errorf("%w: full hash table", ErrHeader)
}

func (x *Index) Page(frame uint32) (uint32, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	if err := x.checkFrame(frame); err != nil {
		return 0, err
	}
	block := frameBlock(frame)
	b, err := x.blockLocked(block, false)
	if err != nil {
		return 0, err
	}
	page := load32(b, pageOffset(block, frame-blockBase(block)))
	if page == 0 || page > 0xfffffffe {
		return 0, fmt.Errorf("%w: missing or invalid frame entry", ErrHeader)
	}
	return page, nil
}

// Lookup returns the newest frame for page in the inclusive snapshot bounds,
// or zero if none exists. A reader lock must protect this snapshot from reset.
// Appended newer entries are ignored, including ones in an existing block.
func (x *Index) Lookup(page, minFrame, maxFrame uint32) (uint32, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.closed {
		return 0, ErrClosed
	}
	if page == 0 || page > 0xfffffffe {
		return 0, ErrHeader
	}
	if maxFrame > x.maxFrames {
		return 0, ErrLimit
	}
	if minFrame == 0 {
		minFrame = 1
	}
	if maxFrame == 0 || minFrame > maxFrame {
		return 0, nil
	}
	minimum := frameBlock(minFrame)
	for block := frameBlock(maxFrame); ; block-- {
		b, err := x.blockLocked(block, false)
		if err != nil {
			return 0, err
		}
		base := blockBase(block)
		key := int((page * 383) & (HashSlots - 1))
		best := uint32(0)
		ended := false
		for collisions := 0; collisions < HashSlots; collisions++ {
			relative := loadHash(b, key)
			if relative == 0 {
				ended = true
				break
			}
			if relative > blockCapacity(block) {
				return 0, fmt.Errorf("%w: hash frame outside block", ErrHeader)
			}
			frame := base + relative
			if frame >= minFrame && frame <= maxFrame && frame > best && load32(b, pageOffset(block, relative)) == page {
				best = frame
			}
			key = (key + 1) & (HashSlots - 1)
		}
		if !ended {
			return 0, fmt.Errorf("%w: unterminated hash chain", ErrHeader)
		}
		if best != 0 {
			return best, nil
		}
		if block == minimum {
			break
		}
	}
	return 0, nil
}
