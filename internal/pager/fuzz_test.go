package pager

import (
	"encoding/binary"
	"errors"
	"testing"
)

// FuzzParseHeader works entirely on byte slices; it never opens a file or
// mutates a database. The physical length is a scalar parser input.
func FuzzParseHeader(f *testing.F) {
	for _, size := range []uint32{512, 4096, 65536} {
		p := make([]byte, 100)
		copy(p, []byte("SQLite format 3\x00"))
		diskSize := uint16(size)
		if size == 65536 {
			diskSize = 1
		}
		binary.BigEndian.PutUint16(p[16:], diskSize)
		p[18], p[19] = 1, 1
		p[21], p[22], p[23] = 64, 32, 32
		binary.BigEndian.PutUint32(p[28:], 1)
		binary.BigEndian.PutUint32(p[44:], 4)
		binary.BigEndian.PutUint32(p[56:], 1)
		f.Add(p, int64(size))
		wal := append([]byte(nil), p...)
		wal[18], wal[19] = 2, 2
		f.Add(wal, int64(size))
		old := append([]byte(nil), p...)
		binary.BigEndian.PutUint32(old[28:], 0)
		f.Add(old, int64(size))
	}
	f.Add([]byte{}, int64(0))
	f.Add([]byte("SQLite format 3\x00"), int64(-1))
	f.Fuzz(func(t *testing.T, p []byte, size int64) {
		h, err := ParseHeader(p, size)
		if err != nil {
			if !errors.Is(err, ErrCorrupt) && !errors.Is(err, ErrUnsupported) {
				t.Fatalf("unexpected error: %v", err)
			}
			return
		}
		if len(p) < 100 || h.PageSize < 512 || h.PageSize > 65536 || h.PageSize&(h.PageSize-1) != 0 || h.UsableSize < 480 || h.UsableSize > h.PageSize {
			t.Fatalf("invalid accepted header: %+v", h)
		}
		if size < 100 || size%int64(h.PageSize) != 0 || h.PageCount == 0 || uint64(h.PageCount) > uint64(size/int64(h.PageSize)) {
			t.Fatalf("invalid accepted extent: %+v size=%d", h, size)
		}
		if h.ReadVersion != 1 || h.WriteVersion != 1 || (h.TextEncoding != 1 && !(h.TextEncoding == 0 && h.SchemaFormat == 0)) {
			t.Fatalf("unsupported format accepted: %+v", h)
		}
		again, err := ParseHeader(p[:100], size)
		if err != nil || again != h {
			t.Fatalf("trailing bytes change header interpretation: %+v %v", again, err)
		}
	})
}
