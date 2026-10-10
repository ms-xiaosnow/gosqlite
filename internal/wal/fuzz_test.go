package wal

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"
)

// FuzzScan operates only on in-memory byte strings. It does not attempt disk
// recovery, file mutation, process crashes, or production-engine delegation.
func FuzzScan(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("not a WAL"))
	for _, magic := range []uint32{MagicLittleEndian, MagicBigEndian} {
		l := newMemoryLog(f, magic, 512)
		f.Add(bytes.Clone(l.bytes))
		l.append(f, 1, 1, 0)
		f.Add(bytes.Clone(l.bytes))
		l.append(f, 1, 0, 7)
		f.Add(bytes.Clone(l.bytes))
		l.append(f, 2, 2, 19)
		f.Add(bytes.Clone(l.bytes))
		f.Add(bytes.Clone(l.bytes[:len(l.bytes)-1]))
		l.bytes[HeaderSize+FrameHeaderSize+1] ^= 1
		f.Add(bytes.Clone(l.bytes))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		s, err := Scan(context.Background(), bytes.NewReader(b), int64(len(b)), Limits{
			MaxWALBytes: 1 << 20, MaxFrames: 2048, MaxIndexEntries: 1024,
		})
		if err != nil && !errors.Is(err, ErrTail) {
			if s.Complete {
				t.Fatal("failed scan marked complete")
			}
			return
		}
		if s.EndOffset < HeaderSize || s.EndOffset > int64(len(b)) || s.ValidEndOffset < s.EndOffset || s.ValidFrameCount < s.FrameCount {
			t.Fatalf("invalid snapshot extents: %+v", s)
		}
		if err == nil && (!s.Complete || s.Tail != TailClean || s.EndOffset != int64(len(b))) {
			t.Fatalf("successful scan not clean: %+v", s)
		}
		if err != nil && (s.Complete || s.Tail == TailClean) {
			t.Fatalf("tail error not classified: %+v %v", s, err)
		}
		// The returned committed prefix must independently validate and
		// reconstruct exactly the same committed page index and checksum.
		prefix, perr := Scan(context.Background(), bytes.NewReader(b), s.EndOffset, Limits{
			MaxWALBytes: 1 << 20, MaxFrames: 2048, MaxIndexEntries: 1024,
		})
		if perr != nil || !prefix.Complete || prefix.DBSize != s.DBSize || prefix.Checksum != s.Checksum || prefix.FrameCount != s.FrameCount || prefix.IndexWork != s.IndexWork || !reflect.DeepEqual(prefix.Pages, s.Pages) {
			t.Fatalf("invalid committed prefix: %+v %v; original %+v %v", prefix, perr, s, err)
		}
		for pageNo, ref := range s.Pages {
			if pageNo == 0 || pageNo > s.DBSize || ref.Frame == 0 || ref.Frame > s.FrameCount || ref.Offset < HeaderSize+FrameHeaderSize || ref.Offset+int64(s.Header.PageSize) > s.EndOffset {
				t.Fatalf("out-of-snapshot page %d: %+v in %+v", pageNo, ref, s)
			}
		}
	})
}

func FuzzFormatRoundTrip(f *testing.F) {
	f.Add([]byte{0, 0, 1, 2, 3, 4, 5, 6})
	f.Add([]byte{1, 7, 255, 255, 1, 0, 127, 128})
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) < 2 {
			return
		}
		magic := MagicLittleEndian
		if input[0]&1 != 0 {
			magic = MagicBigEndian
		}
		pageSize := uint32(512) << (input[1] % 8)
		h, hb := testHeader(t, magic, pageSize)
		sum := h.Checksum
		page := make([]byte, pageSize)
		for i := range page {
			page[i] = input[i%len(input)]
		}
		log := bytes.Clone(hb)
		count := min(len(input), 16)
		for i := 0; i < count; i++ {
			pageNo := uint32(input[i]) + 1
			dbSize := uint32(0)
			if i == count-1 || input[i]&1 != 0 {
				dbSize = pageNo
			}
			encoded, next, err := EncodeFrame(h, pageNo, dbSize, page, sum)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := ParseFrame(h, encoded[:FrameHeaderSize], encoded[FrameHeaderSize:], sum)
			if err != nil || parsed.PageNo != pageNo || parsed.DBSize != dbSize || parsed.Checksum != next || parsed.Salt != h.Salt || !bytes.Equal(encoded[FrameHeaderSize:], page) {
				t.Fatalf("frame roundtrip %+v, %v", parsed, err)
			}
			sum = next
			log = append(log, encoded...)
		}
		s, err := scanMemory(log, Limits{})
		if err != nil || !s.Complete || s.FrameCount != uint32(count) || s.Checksum != sum || s.EndOffset != int64(len(log)) {
			t.Fatalf("encoded frames do not scan: %+v, %v", s, err)
		}
	})
}
