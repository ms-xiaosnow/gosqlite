package encstore

import (
	"crypto/sha256"
	"encoding/binary"
	"github.com/ms-xiaosnow/gosqlite/encryption"
	"math"
)

type Mode uint8

const (
	Snapshot Mode = iota
	WAL
)

// WALLimits bounds the complete selected graph and operation, independently of
// primitive, transaction-dirty and retained-history limits. Zero uses defaults.
type WALLimits struct {
	MaxSegments       uint32
	MaxFrames         uint64
	MaxSegmentFrames  uint32
	MaxSegmentBytes   uint64
	MaxBytes          uint64
	MaxRootBytes      uint32
	MaxOverlayPages   uint32
	MaxMemoryBytes    uint64
	MaxOpenFiles      uint32
	MaxScanBytes      uint64
	MaxOperationBytes uint64
	MaxReplaySteps    uint64
}

func (l WALLimits) normalized() (WALLimits, error) {
	defaults := WALLimits{128, 1 << 20, 16384, 64 << 20, 256 << 20, 64 << 10, 1 << 20, 64 << 20, 256, 1 << 30, 8 << 30, 8 << 20}
	hard := WALLimits{4094, 8 << 20, 65536, 256 << 20, 1 << 30, encryption.MaxRecordBytes + 552, 1 << 20, 512 << 20, 8192, 70 << 30, 560 << 30, 64 << 20}
	a := []*uint32{&l.MaxSegments, &l.MaxSegmentFrames, &l.MaxRootBytes, &l.MaxOverlayPages, &l.MaxOpenFiles}
	d := []uint32{defaults.MaxSegments, defaults.MaxSegmentFrames, defaults.MaxRootBytes, defaults.MaxOverlayPages, defaults.MaxOpenFiles}
	h := []uint32{hard.MaxSegments, hard.MaxSegmentFrames, hard.MaxRootBytes, hard.MaxOverlayPages, hard.MaxOpenFiles}
	for i, p := range a {
		if *p == 0 {
			*p = d[i]
		}
		if *p > h[i] {
			return l, encryption.ErrLimit
		}
	}
	b := []*uint64{&l.MaxFrames, &l.MaxSegmentBytes, &l.MaxBytes, &l.MaxMemoryBytes, &l.MaxScanBytes, &l.MaxOperationBytes, &l.MaxReplaySteps}
	e := []uint64{defaults.MaxFrames, defaults.MaxSegmentBytes, defaults.MaxBytes, defaults.MaxMemoryBytes, defaults.MaxScanBytes, defaults.MaxOperationBytes, defaults.MaxReplaySteps}
	j := []uint64{hard.MaxFrames, hard.MaxSegmentBytes, hard.MaxBytes, hard.MaxMemoryBytes, hard.MaxScanBytes, hard.MaxOperationBytes, hard.MaxReplaySteps}
	for i, p := range b {
		if *p == 0 {
			*p = e[i]
		}
		if *p > j[i] {
			return l, encryption.ErrLimit
		}
	}
	return l, nil
}

const refSize = 256
const walFixedMeta = 128
const walFooterSize = 208

func walRootSize(n int) uint64                    { return 1064 + uint64(n)*refSize }
func segmentSize(ps uint32, frames uint64) uint64 { return 928 + frames*(uint64(ps)+44) }
func frameOffset(ps uint32, frames, ordinal uint64) int64 {
	return int64(680 + 4*frames + (ordinal-1)*(uint64(ps)+40))
}

type objectRef struct {
	Descriptor                  encryption.FileDescriptor
	FileID                      [32]byte
	Bytes                       uint64
	Hash, PageOneHash, CommitID [32]byte
}

func (r objectRef) encode() []byte {
	b := make([]byte, refSize)
	d := r.Descriptor
	b[0] = byte(d.Role)
	b[1] = 1
	if d.Role == encryption.FileWAL {
		b[1] = 2
	}
	binary.BigEndian.PutUint32(b[4:8], d.PageSize)
	binary.BigEndian.PutUint32(b[8:12], d.PageCount)
	binary.BigEndian.PutUint64(b[16:24], d.RecordCount)
	binary.BigEndian.PutUint64(b[24:32], d.CommitSequence)
	copy(b[32:64], d.TransactionID[:])
	copy(b[64:96], d.ParentCommitID[:])
	copy(b[96:128], r.FileID[:])
	binary.BigEndian.PutUint64(b[128:136], r.Bytes)
	copy(b[136:168], r.Hash[:])
	copy(b[168:200], r.PageOneHash[:])
	copy(b[200:232], r.CommitID[:])
	return b
}
func (r objectRef) identity(e encryption.Epoch) encryption.FileIdentity {
	return encryption.FileIdentity{DatabaseID: e.DatabaseID, EpochID: e.EpochID, FileID: r.FileID, Role: r.Descriptor.Role}
}
func (r objectRef) commitID(e encryption.Epoch) [32]byte {
	h := sha256.New()
	h.Write([]byte("GSECID02"))
	h.Write(e.DatabaseID[:])
	h.Write(e.EpochID[:])
	h.Write(r.encode()[:200])
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
func refFor(o *object) objectRef {
	r := objectRef{Descriptor: o.header.Descriptor, FileID: o.header.FileID, Bytes: objectSize(o.geometry()), Hash: o.hash, PageOneHash: o.pageOneHash}
	r.CommitID = r.commitID(o.header.Epoch)
	return r
}
func decodeRef(b []byte, e encryption.Epoch, limits encryption.Limits) (objectRef, error) {
	var r objectRef
	if len(b) != refSize || !zero(b[2:4]) || !zero(b[12:16]) || !zero(b[232:]) {
		return r, encryption.ErrFormat
	}
	d := &r.Descriptor
	d.Role = encryption.FileRole(b[0])
	if (d.Role != encryption.FileBase || b[1] != 1) && (d.Role != encryption.FileWAL || b[1] != 2) {
		return r, encryption.ErrFormat
	}
	d.PageSize = binary.BigEndian.Uint32(b[4:])
	d.PageCount = binary.BigEndian.Uint32(b[8:])
	d.RecordCount = binary.BigEndian.Uint64(b[16:])
	d.CommitSequence = binary.BigEndian.Uint64(b[24:])
	copy(d.TransactionID[:], b[32:64])
	copy(d.ParentCommitID[:], b[64:96])
	copy(r.FileID[:], b[96:128])
	r.Bytes = binary.BigEndian.Uint64(b[128:])
	copy(r.Hash[:], b[136:168])
	copy(r.PageOneHash[:], b[168:200])
	copy(r.CommitID[:], b[200:232])
	if err := d.Validate(limits); err != nil {
		return r, err
	}
	size := objectSize(Geometry{d.PageSize, d.PageCount})
	if d.Role == encryption.FileWAL {
		size = segmentSize(d.PageSize, d.RecordCount-2)
	}
	if r.FileID == ([32]byte{}) || d.TransactionID == ([32]byte{}) || d.CommitSequence == 0 || r.Bytes != size || r.CommitID != r.commitID(e) {
		return r, encryption.ErrFormat
	}
	return r, nil
}

type walManifest struct {
	Base                                objectRef
	Segments                            []objectRef
	Frames, Bytes                       uint64
	Head, PageOneHash, PreviousRootHash [32]byte
}

func (m walManifest) encode() []byte {
	b := make([]byte, 512+refSize*len(m.Segments))
	copy(b, "GSEMNF02")
	binary.BigEndian.PutUint16(b[8:], 2)
	binary.BigEndian.PutUint16(b[10:], refSize)
	binary.BigEndian.PutUint32(b[16:], 1)
	binary.BigEndian.PutUint32(b[20:], uint32(len(m.Segments)))
	binary.BigEndian.PutUint64(b[24:], m.Frames)
	binary.BigEndian.PutUint64(b[32:], m.Bytes)
	binary.BigEndian.PutUint64(b[40:], uint64(len(b)))
	copy(b[48:80], m.Head[:])
	copy(b[80:112], m.PageOneHash[:])
	copy(b[112:144], m.PreviousRootHash[:])
	copy(b[256:512], m.Base.encode())
	for i, r := range m.Segments {
		copy(b[512+i*refSize:], r.encode())
	}
	h := sha256.New()
	h.Write([]byte("GSESET02"))
	h.Write(b[256:])
	copy(b[144:176], h.Sum(nil))
	return b
}
func decodeWALManifest(b []byte, h encryption.Header, o Options) (walManifest, error) {
	var m walManifest
	l, err := o.WALLimits.normalized()
	if err != nil {
		return m, err
	}
	if len(b) < 512 || string(b[:8]) != "GSEMNF02" || binary.BigEndian.Uint16(b[8:]) != 2 || binary.BigEndian.Uint16(b[10:]) != refSize || !zero(b[12:16]) || binary.BigEndian.Uint32(b[16:]) != 1 || !zero(b[176:256]) {
		return m, encryption.ErrFormat
	}
	n := binary.BigEndian.Uint32(b[20:])
	if n > l.MaxSegments || walRootSize(int(n)) > uint64(l.MaxRootBytes) {
		return m, encryption.ErrLimit
	}
	if uint64(len(b)) != 512+uint64(n)*refSize || binary.BigEndian.Uint64(b[40:]) != uint64(len(b)) {
		return m, encryption.ErrFormat
	}
	digest := sha256.New()
	digest.Write([]byte("GSESET02"))
	digest.Write(b[256:])
	if string(digest.Sum(nil)) != string(b[144:176]) {
		return m, encryption.ErrAuthentication
	}
	m.Frames = binary.BigEndian.Uint64(b[24:])
	m.Bytes = binary.BigEndian.Uint64(b[32:])
	copy(m.Head[:], b[48:80])
	copy(m.PageOneHash[:], b[80:112])
	copy(m.PreviousRootHash[:], b[112:144])
	m.Base, err = decodeRef(b[256:512], h.Epoch, o.Limits)
	if err != nil {
		return m, err
	}
	if m.Base.Descriptor.Role != encryption.FileBase {
		return m, encryption.ErrFormat
	}
	if uint64(n+1)*1024+uint64(len(b))*4 > l.MaxMemoryBytes {
		return m, encryption.ErrLimit
	}
	m.Segments = make([]objectRef, n)
	if m.Base.FileID == h.FileID || (m.Base.Descriptor.CommitSequence == 1) != (m.Base.Descriptor.ParentCommitID == ([32]byte{})) || (h.Descriptor.CommitSequence == 1) != (m.PreviousRootHash == ([32]byte{})) {
		return m, encryption.ErrFormat
	}
	seen := map[[32]byte]bool{m.Base.FileID: true, h.FileID: true}
	prev := m.Base
	var frames, bytes uint64
	for i := range m.Segments {
		r, e := decodeRef(b[512+i*refSize:512+(i+1)*refSize], h.Epoch, o.Limits)
		if e != nil {
			return m, e
		}
		d := r.Descriptor
		if d.Role != encryption.FileWAL || seen[r.FileID] || d.PageSize != prev.Descriptor.PageSize || prev.Descriptor.CommitSequence == math.MaxUint64 || d.CommitSequence != prev.Descriptor.CommitSequence+1 || d.ParentCommitID != prev.CommitID {
			return m, encryption.ErrFormat
		}
		seen[r.FileID] = true
		if d.RecordCount-2 > uint64(l.MaxSegmentFrames) || r.Bytes > l.MaxSegmentBytes {
			return m, encryption.ErrLimit
		}
		frames, e = charge(frames, d.RecordCount-2, l.MaxFrames)
		if e != nil {
			return m, e
		}
		bytes, e = charge(bytes, r.Bytes, l.MaxBytes)
		if e != nil {
			return m, e
		}
		m.Segments[i] = r
		prev = r
	}
	d := prev.Descriptor
	d.Role = encryption.FileRoot
	d.RecordCount = 1
	if h.Descriptor != d || m.Head != prev.CommitID || m.PageOneHash != prev.PageOneHash || frames != m.Frames || bytes != m.Bytes {
		return m, encryption.ErrAuthentication
	}
	if err = graphBudget(m, o, 1); err != nil {
		return m, err
	}
	return m, nil
}

// graphBudget includes both old/new dense overlays, directories, references,
// key-reader/descriptor allowances and root/AEAD scratch, plus replay work.
func graphBudget(m walManifest, o Options, passes uint64, additionalMemory ...uint64) error {
	l, e := o.WALLimits.normalized()
	if e != nil {
		return e
	}
	max := m.Base.Descriptor.PageCount
	steps := uint64(0)
	prev := max
	for _, r := range m.Segments {
		n := r.Descriptor.PageCount
		if n > max {
			max = n
		}
		if n < prev {
			steps += uint64(prev - n)
		}
		prev = n
	}
	// Replay includes dense zero-initialization, final coverage, directory
	// decode/validation, frame iteration and repeated selection validation.
	// Growth cannot hide the final coverage loop behind a tiny initial base.
	steps += 2*uint64(max) + 8*m.Frames + 16*uint64(len(m.Segments)+1)
	if len(m.Segments) > int(l.MaxSegments) || m.Frames > l.MaxFrames || m.Bytes > l.MaxBytes || max > l.MaxOverlayPages || uint64(len(m.Segments))+9 > uint64(l.MaxOpenFiles) || steps > l.MaxReplaySteps {
		return encryption.ErrLimit
	}
	size := walRootSize(len(m.Segments))
	memory := 32768 + uint64(max+1)*16 + m.Frames*8 + uint64(len(m.Segments)+1)*16384 + size*6 + uint64(m.Base.Descriptor.PageSize)*8 + uint64(l.MaxSegmentFrames)*80
	for _, extra := range additionalMemory {
		memory, e = charge(memory, extra, l.MaxMemoryBytes)
		if e != nil {
			return e
		}
	}
	if memory > l.MaxMemoryBytes || size > uint64(l.MaxRootBytes) {
		return encryption.ErrLimit
	}
	scan, e := charge(m.Base.Bytes, m.Bytes, l.MaxScanBytes)
	if e != nil {
		return e
	}
	// Full logical passes also revalidate current root bytes on every page.
	// Charge that read amplification rather than treating each scan as a fresh
	// allowance. Six root buffers and both overlay generations are above.
	headPages := m.Base.Descriptor.PageCount
	if len(m.Segments) > 0 {
		headPages = m.Segments[len(m.Segments)-1].Descriptor.PageCount
	}
	if passes == 0 {
		return nil
	}
	work, e := charge(scan, uint64(headPages)*(size+uint64(m.Base.Descriptor.PageSize)+recordOverhead), l.MaxOperationBytes)
	if e != nil {
		return e
	}
	if passes != 0 && work > l.MaxOperationBytes/passes {
		return encryption.ErrLimit
	}
	return nil
}
func validateDirectory(pages []uint32, previous, final uint32) error {
	if len(pages) == 0 || pages[0] != 1 {
		return encryption.ErrFormat
	}
	var old uint32
	growth := uint64(0)
	for _, n := range pages {
		if n <= old || n > final {
			return encryption.ErrFormat
		}
		if n > previous {
			growth++
		}
		old = n
	}
	if final > previous && growth != uint64(final-previous) {
		return encryption.ErrFormat
	}
	return nil
}
func segmentMetadata(d encryption.FileDescriptor, prev uint32, p1 [32]byte, pages []uint32) []byte {
	b := make([]byte, 128+4*len(pages))
	copy(b, "GSEWAL02")
	binary.BigEndian.PutUint32(b[8:], uint32(len(pages)))
	binary.BigEndian.PutUint32(b[12:], 4)
	binary.BigEndian.PutUint32(b[16:], prev)
	binary.BigEndian.PutUint32(b[20:], d.PageCount)
	binary.BigEndian.PutUint64(b[24:], d.CommitSequence)
	copy(b[32:64], d.TransactionID[:])
	copy(b[64:96], d.ParentCommitID[:])
	copy(b[96:128], p1[:])
	for i, n := range pages {
		binary.BigEndian.PutUint32(b[128+4*i:], n)
	}
	return b
}
func parseSegmentMetadata(b []byte, r objectRef, prev uint32) ([]uint32, error) {
	d := r.Descriptor
	f := d.RecordCount - 2
	if uint64(len(b)) != 128+4*f || string(b[:8]) != "GSEWAL02" || uint64(binary.BigEndian.Uint32(b[8:])) != f || binary.BigEndian.Uint32(b[12:]) != 4 || binary.BigEndian.Uint32(b[16:]) != prev || binary.BigEndian.Uint32(b[20:]) != d.PageCount || binary.BigEndian.Uint64(b[24:]) != d.CommitSequence || string(b[32:64]) != string(d.TransactionID[:]) || string(b[64:96]) != string(d.ParentCommitID[:]) || string(b[96:128]) != string(r.PageOneHash[:]) {
		return nil, encryption.ErrFormat
	}
	pages := make([]uint32, int(f))
	for i := range pages {
		pages[i] = binary.BigEndian.Uint32(b[128+4*i:])
	}
	return pages, validateDirectory(pages, prev, d.PageCount)
}
func segmentFooter(r objectRef, prev uint32, prefix, meta [32]byte) []byte {
	d := r.Descriptor
	b := make([]byte, walFooterSize)
	copy(b, "GSEWEND2")
	copy(b[8:40], prefix[:])
	copy(b[40:72], meta[:])
	copy(b[72:104], r.PageOneHash[:])
	binary.BigEndian.PutUint64(b[104:], r.Bytes)
	binary.BigEndian.PutUint64(b[112:], d.RecordCount-2)
	binary.BigEndian.PutUint32(b[120:], d.PageCount)
	binary.BigEndian.PutUint32(b[124:], prev)
	binary.BigEndian.PutUint64(b[128:], d.CommitSequence)
	copy(b[136:168], d.ParentCommitID[:])
	copy(b[168:200], d.TransactionID[:])
	return b
}
