package walindex

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func testHeader() Header {
	return Header{Version: Version, Unused: 0x01020304, Change: 91, BigEndianChecksum: true, PageSize: 65536,
		MaxFrame: 4180, DBSize: 10000, FrameChecksum: [2]uint32{0x12345678, 0x90abcdef}, Salt: [8]byte{1, 2, 3, 4, 5, 6, 7, 8}}
}

func TestHeaderCodecNativeLayout(t *testing.T) {
	h := testHeader()
	b, err := EncodeHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if binary.NativeEndian.Uint32(b[:]) != Version || binary.NativeEndian.Uint32(b[4:]) != h.Unused || b[12] != 1 || b[13] != 1 || binary.NativeEndian.Uint16(b[14:]) != 1 {
		t.Fatalf("incorrect native layout: %x", b)
	}
	if !bytes.Equal(b[32:40], h.Salt[:]) {
		t.Fatal("WAL salt bytes changed byte order")
	}
	got, err := ParseHeader(b[:])
	if err != nil {
		t.Fatal(err)
	}
	h.Checksum = got.Checksum
	if got != h {
		t.Fatalf("header got %+v want %+v", got, h)
	}
	b2, err := EncodeHeader(got)
	if err != nil || b2 != b {
		t.Fatalf("round trip failed: %v", err)
	}
}

func TestHeaderMalformedBytesOnly(t *testing.T) {
	original, err := EncodeHeader(testHeader())
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < HeaderSize; n++ {
		if _, err := ParseHeader(original[:n]); err == nil {
			t.Fatalf("accepted length %d", n)
		}
	}
	long := append(append([]byte(nil), original[:]...), 0)
	if _, err := ParseHeader(long); err == nil {
		t.Fatal("accepted extra bytes")
	}
	for pos := range original {
		b := original
		b[pos] ^= 0x80
		if _, err := ParseHeader(b[:]); err == nil {
			t.Fatalf("accepted changed byte %d", pos)
		}
	}
	var zero [HeaderSize]byte
	if _, err := ParseHeader(zero[:]); !errors.Is(err, ErrUninitialized) {
		t.Fatalf("zero header: %v", err)
	}
	for _, h := range []Header{
		{Version: Version, PageSize: 513},
		{Version: Version, MaxFrame: 1, DBSize: 1},
		{Version: Version, PageSize: 4096, MaxFrame: 1},
		{Version: Version, PageSize: 4096, DBSize: 0xffffffff},
		{Version: Version + 1, PageSize: 4096},
	} {
		if _, err := EncodeHeader(h); err == nil {
			t.Fatalf("accepted malformed in-memory header %+v", h)
		}
	}
}

func TestHeaderEmptyAndPageSizes(t *testing.T) {
	for _, ps := range []uint32{0, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536} {
		h := Header{Version: Version, PageSize: ps}
		b, err := EncodeHeader(h)
		if err != nil {
			t.Fatalf("page size %d: %v", ps, err)
		}
		got, err := ParseHeader(b[:])
		if err != nil || got.PageSize != ps || got.MaxFrame != 0 {
			t.Fatalf("page size %d got %+v: %v", ps, got, err)
		}
	}
}

func TestFrameBlockBoundaries(t *testing.T) {
	for _, tt := range []struct{ frame, block, relative uint32 }{
		{1, 0, 1}, {4062, 0, 4062}, {4063, 1, 1}, {8158, 1, 4096}, {8159, 2, 1}, {12254, 2, 4096}, {12255, 3, 1},
		{0xfffffffe, 1048576, 32},
	} {
		block := frameBlock(tt.frame)
		relative := tt.frame - blockBase(block)
		if block != tt.block || relative != tt.relative {
			t.Fatalf("frame %d got block%d relative%d", tt.frame, block, relative)
		}
		off := pageOffset(block, relative)
		if off < 0 || off >= BlockSize/2 || off%4 != 0 {
			t.Fatalf("frame %d offset %d", tt.frame, off)
		}
	}
}
