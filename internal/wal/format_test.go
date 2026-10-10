package wal

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"testing"
)

func testHeader(t testing.TB, magic, pageSize uint32) (Header, []byte) {
	t.Helper()
	h := Header{Magic: magic, Version: Version, PageSize: pageSize,
		CheckpointSequence: 0x01020304, Salt: [2]uint32{0x11223344, 0x55667788}}
	b, err := EncodeHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	h, err = ParseHeader(b)
	if err != nil {
		t.Fatal(err)
	}
	return h, b
}

// Fixed vectors were calculated independently from the published checksum
// definition with 64-bit intermediates reduced modulo 2^32. They are format
// vectors, not a claim of file-level SQLite interoperability.
func TestGoldenFormatBothByteOrders(t *testing.T) {
	for _, tc := range []struct {
		magic uint32
		head  string
		frame string
	}{
		{MagicLittleEndian, "377f0682002de218000002000102030411223344556677881d01272561556720", "000000070000000b1122334455667788b94a5822f7308d5f"},
		{MagicBigEndian, "377f0683002de218000002000102030411223344556677882829011f256a5564", "000000070000000b11223344556677887003317a51ed7a12"},
	} {
		t.Run(fmt.Sprintf("%x", tc.magic), func(t *testing.T) {
			h, hb := testHeader(t, tc.magic, 512)
			if hex.EncodeToString(hb) != tc.head {
				t.Fatalf("header = %x", hb)
			}
			page := make([]byte, 512)
			for i := range page {
				page[i] = byte(i)
			}
			fb, sum, err := EncodeFrame(h, 7, 11, page, h.Checksum)
			if err != nil {
				t.Fatal(err)
			}
			if hex.EncodeToString(fb[:FrameHeaderSize]) != tc.frame || !bytes.Equal(fb[FrameHeaderSize:], page) {
				t.Fatalf("frame header = %x", fb[:FrameHeaderSize])
			}
			f, err := ParseFrame(h, fb[:FrameHeaderSize], page, h.Checksum)
			if err != nil || f.PageNo != 7 || f.DBSize != 11 || f.Checksum != sum || f.Salt != h.Salt {
				t.Fatalf("parse = %+v, %v", f, err)
			}
		})
	}
}

func TestHeaderPageSizesAndExactLengths(t *testing.T) {
	for _, magic := range []uint32{MagicLittleEndian, MagicBigEndian} {
		for ps := uint32(512); ps <= 65536; ps *= 2 {
			h, b := testHeader(t, magic, ps)
			if h.PageSize != ps || binary.BigEndian.Uint32(b[8:]) != ps {
				t.Fatalf("page size %d: %+v %x", ps, h, b)
			}
			page := make([]byte, ps)
			fb, _, err := EncodeFrame(h, 1, 1, page, h.Checksum)
			if err != nil || len(fb) != int(ps)+FrameHeaderSize {
				t.Fatalf("size %d: %v", ps, err)
			}
			if _, err := ParseFrame(h, fb[:FrameHeaderSize], fb[FrameHeaderSize:], h.Checksum); err != nil {
				t.Fatal(err)
			}
		}
	}
	_, b := testHeader(t, MagicLittleEndian, 512)
	for n := 0; n < HeaderSize; n++ {
		if h, err := ParseHeader(b[:n]); !errors.Is(err, ErrFormat) || h != (Header{}) {
			t.Fatalf("header prefix %d accepted: %+v, %v", n, h, err)
		}
	}
	if _, err := ParseHeader(append(b, 0)); !errors.Is(err, ErrFormat) {
		t.Fatal("overlong header accepted")
	}
}

func TestHeaderRejectsEverySingleBitMutation(t *testing.T) {
	for _, magic := range []uint32{MagicLittleEndian, MagicBigEndian} {
		_, b := testHeader(t, magic, 65536)
		for i := range b {
			for bit := byte(1); bit != 0; bit <<= 1 {
				bad := bytes.Clone(b)
				bad[i] ^= bit
				if h, err := ParseHeader(bad); err == nil || h != (Header{}) {
					t.Fatalf("accepted header mutation byte=%d bit=%x", i, bit)
				}
			}
		}
	}
}

func TestInvalidHeaderFields(t *testing.T) {
	h, _ := testHeader(t, MagicLittleEndian, 512)
	for _, pageSize := range []uint32{0, 1, 255, 256, 511, 513, 65535, 65537, 131072, 0xffffffff} {
		bad := h
		bad.PageSize = pageSize
		if _, err := EncodeHeader(bad); !errors.Is(err, ErrFormat) {
			t.Fatalf("page size %d accepted: %v", pageSize, err)
		}
	}
	bad := h
	bad.Version++
	if _, err := EncodeHeader(bad); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported version: %v", err)
	}
	bad = h
	bad.Magic = 0
	if _, err := EncodeHeader(bad); !errors.Is(err, ErrFormat) {
		t.Fatalf("zero magic: %v", err)
	}
	// Zero salts are permitted wire-format values. Randomness is a writer duty.
	h.Salt = [2]uint32{}
	h.Checksum = Checksum{1, 2}
	b, err := EncodeHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseHeader(b)
	if err != nil || parsed.Salt != h.Salt || parsed.Checksum == h.Checksum {
		t.Fatalf("canonical checksum: %+v, %v", parsed, err)
	}
}

func TestRollingChecksumReferenceAndChunking(t *testing.T) {
	rng := rand.New(rand.NewSource(7193))
	for _, magic := range []uint32{MagicLittleEndian, MagicBigEndian} {
		for n := 0; n <= 256; n++ {
			b := make([]byte, n*8)
			rng.Read(b)
			previous := Checksum{rng.Uint32(), rng.Uint32()}
			got, err := RollingChecksum(magic, b, previous)
			want := referenceChecksum(magic, b, previous)
			if err != nil || got != want {
				t.Fatalf("checksum %x len%d: %x want%x err%v", magic, len(b), got, want, err)
			}
			partial := previous
			for off := 0; off < len(b); off += 8 {
				partial, err = RollingChecksum(magic, b[off:off+8], partial)
				if err != nil {
					t.Fatal(err)
				}
			}
			if partial != got {
				t.Fatal("chunked checksum differs")
			}
		}
	}
	seed := Checksum{7, 11}
	for _, n := range []int{1, 4, 7, 9, 12, 15} {
		if got, err := RollingChecksum(MagicLittleEndian, make([]byte, n), seed); !errors.Is(err, ErrFormat) || got != seed {
			t.Fatalf("invalid input length %d: %x %v", n, got, err)
		}
	}
	if got, err := RollingChecksum(0, nil, seed); !errors.Is(err, ErrFormat) || got != seed {
		t.Fatal("invalid magic accepted")
	}
}

// Deliberately independent byte assembly and wider arithmetic for the model.
func referenceChecksum(magic uint32, data []byte, initial Checksum) Checksum {
	s0, s1 := uint64(initial[0]), uint64(initial[1])
	for i := 0; i < len(data); i += 8 {
		var words [2]uint64
		for word := range words {
			for j := 0; j < 4; j++ {
				shift := 8 * j
				if magic == MagicBigEndian {
					shift = 8 * (3 - j)
				}
				words[word] |= uint64(data[i+4*word+j]) << shift
			}
		}
		s0 = (s0 + words[0] + s1) & 0xffffffff
		s1 = (s1 + words[1] + s0) & 0xffffffff
	}
	return Checksum{uint32(s0), uint32(s1)}
}

func TestFrameRejectsEverySingleByteMutation(t *testing.T) {
	for _, magic := range []uint32{MagicLittleEndian, MagicBigEndian} {
		h, _ := testHeader(t, magic, 512)
		page := make([]byte, h.PageSize)
		for i := range page {
			page[i] = byte(i * 17)
		}
		b, _, err := EncodeFrame(h, 3, 5, page, h.Checksum)
		if err != nil {
			t.Fatal(err)
		}
		for i := range b {
			bad := bytes.Clone(b)
			bad[i] ^= 1
			if f, err := ParseFrame(h, bad[:FrameHeaderSize], bad[FrameHeaderSize:], h.Checksum); err == nil || f != (Frame{}) {
				t.Fatalf("accepted mutation magic=%x byte=%d", magic, i)
			}
		}
		bad := bytes.Clone(b)
		bad[8] ^= 1
		if _, err := ParseFrame(h, bad[:FrameHeaderSize], bad[FrameHeaderSize:], h.Checksum); !errors.Is(err, ErrSalt) {
			t.Fatalf("salt error: %v", err)
		}
		bad = bytes.Clone(b)
		bad[FrameHeaderSize] ^= 1
		if _, err := ParseFrame(h, bad[:FrameHeaderSize], bad[FrameHeaderSize:], h.Checksum); !errors.Is(err, ErrChecksum) {
			t.Fatalf("checksum error: %v", err)
		}
	}
}

func TestInvalidFrameFieldsAndNoMutation(t *testing.T) {
	h, _ := testHeader(t, MagicLittleEndian, 512)
	page := bytes.Repeat([]byte{0xa3}, 512)
	original := bytes.Clone(page)
	seed := h.Checksum
	for _, tc := range []struct {
		pageNo, dbSize uint32
		size           int
	}{
		{0, 0, 512}, {0xffffffff, 0, 512}, {0x40000000/512 + 1, 0, 512},
		{1, 0xffffffff, 512}, {1, 0, 511}, {1, 0, 513}, {1, 0, 0},
	} {
		if b, sum, err := EncodeFrame(h, tc.pageNo, tc.dbSize, make([]byte, tc.size), seed); !errors.Is(err, ErrFormat) || b != nil || sum != seed {
			t.Fatalf("invalid fields %+v: %x %x %v", tc, b, sum, err)
		}
	}
	b, sum, err := EncodeFrame(h, 3, 2, page, seed)
	if err != nil || !bytes.Equal(page, original) {
		t.Fatalf("truncating frame: %v", err)
	}
	if _, err := ParseFrame(h, b[:FrameHeaderSize], page, seed); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseFrame(h, b[:FrameHeaderSize], page, sum); !errors.Is(err, ErrChecksum) {
		t.Fatalf("wrong chain seed accepted: %v", err)
	}
	for n := 0; n < FrameHeaderSize; n++ {
		if _, err := ParseFrame(h, b[:n], page, seed); !errors.Is(err, ErrFormat) {
			t.Fatalf("short frame header %d accepted", n)
		}
	}
	if _, err := ParseFrame(h, b[:FrameHeaderSize+1], page, seed); !errors.Is(err, ErrFormat) {
		t.Fatal("overlong frame header accepted")
	}
	if _, err := ParseFrame(h, b[:FrameHeaderSize], page[:511], seed); !errors.Is(err, ErrFormat) {
		t.Fatal("short page accepted")
	}
	if !bytes.Equal(page, original) || h.Checksum != seed {
		t.Fatal("input mutated")
	}
}
