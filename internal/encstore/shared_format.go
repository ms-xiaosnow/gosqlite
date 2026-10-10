package encstore

import (
	"bytes"
	"encoding/binary"
	"math"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

// Layout 3 freezes the same immutable child graph encoding as layout 2, but
// authenticates a distinct coordination protocol. The legacy decoder is never
// widened: it continues to reject layout 3 bytes.
const sharedCoordinationOFD3 = 1

func (m walManifest) encodeMode(mode Mode) []byte {
	b := m.encode()
	if mode == WALShared {
		copy(b, "GSEMNF03")
		binary.BigEndian.PutUint16(b[8:], 3)
		b[176] = sharedCoordinationOFD3
	}
	return b
}
func decodeStorageWAL(b []byte, h encryption.Header, o Options) (walManifest, error) {
	if o.Mode == WAL {
		return decodeWALManifest(b, h, o)
	}
	if o.Mode != WALShared {
		return walManifest{}, ErrUnsupported
	}
	if len(b) < 512 || string(b[:8]) != "GSEMNF03" || binary.BigEndian.Uint16(b[8:]) != 3 || b[176] != sharedCoordinationOFD3 || !zero(b[177:256]) {
		return walManifest{}, encryption.ErrFormat
	}
	// The root extent guard bounds this conversion scratch before allocation.
	if e := rootExtent(int64(encryption.HeaderSize+recordOverhead+len(b)), o); e != nil {
		return walManifest{}, e
	}
	legacy := bytes.Clone(b)
	defer clear(legacy)
	copy(legacy, "GSEMNF02")
	binary.BigEndian.PutUint16(legacy[8:], 2)
	legacy[176] = 0
	return decodeWALManifest(legacy, h, o)
}

// selectionMemory charges one complete retained graph, including its root
// evidence, dense overlay, directories, refs, derived readers and object state.
func selectionMemory(m walManifest) uint64 {
	max := m.Base.Descriptor.PageCount
	for _, r := range m.Segments {
		if r.Descriptor.PageCount > max {
			max = r.Descriptor.PageCount
		}
	}
	if m.Frames > math.MaxUint64/8 {
		return math.MaxUint64
	}
	total := uint64(32768)
	for _, n := range []uint64{(uint64(max) + 1) * 16, m.Frames * 8, uint64(len(m.Segments)+1) * 16384, walRootSize(len(m.Segments)) * 6, uint64(m.Base.Descriptor.PageSize) * 8} {
		if n > math.MaxUint64-total {
			return math.MaxUint64
		}
		total += n
	}
	return total
}

// combinedGraphBudget admits the entire old and candidate graph, rather than
// just two overlays. Both remain owned until the candidate is fully verified.
func combinedGraphBudget(old *walManifest, next walManifest, o Options, ids uint64) error {
	if ids > encryption.MaxFilesPerMaster*2 {
		return encryption.ErrLimit
	}
	if old == nil {
		return graphBudget(next, o, 1, ids*128)
	}
	l, e := o.WALLimits.normalized()
	if e != nil {
		return e
	}
	if uint64(len(old.Segments))+uint64(len(next.Segments))+10 > uint64(l.MaxOpenFiles) {
		return encryption.ErrLimit
	}
	if e = graphBudget(next, o, 1, selectionMemory(*old), ids*128); e != nil {
		return e
	}
	// The newly captured graph is authenticated in full. Old graph state is
	// retained, not rescanned, so only the new graph contributes admission I/O.
	return nil
}
