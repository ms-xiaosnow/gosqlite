package encryption

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
	"math/rand/v2"
	"testing"
)

func fixedHeader() Header {
	h := Header{Major: FormatMajor, Minor: FormatMinor, Suite: SuiteAES256GCM, Epoch: fixedEpoch(), NoncePrefix: [4]byte{0xa1, 0xa2, 0xa3, 0xa4}, Descriptor: FileDescriptor{Role: FileBase, PageSize: 512, PageCount: 2, RecordCount: 4, CommitSequence: 7}}
	for i := range h.FileID {
		h.FileID[i], h.Descriptor.TransactionID[i], h.Descriptor.ParentCommitID[i] = byte(i+81), byte(i+113), byte(i+145)
	}
	return h
}

func TestExperimentalHeaderVector(t *testing.T) {
	// This pins this EXPERIMENTAL revision for review; it is not a declaration
	// that the container has been audited, integrated, or frozen for release.
	// Expected bytes independently constructed with Python struct.pack_into.
	want := unhex(t, "4753514c454e43310001000000010200000000000201000000000000000000000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f305152535455565758595a5b5c5d5e5f606162636465666768696a6b6c6d6e6f7000000000000000000000000000000000000000000000000000000000000000003132333435363738393a3b3c3d3e3f404142434445464748494a4b4c4d4e4f50a1a2a3a4000002000000000200000000000000000000000400000000000000077172737475767778797a7b7c7d7e7f808182838485868788898a8b8c8d8e8f909192939495969798999a9b9c9d9e9fa0a1a2a3a4a5a6a7a8a9aaabacadaeafb0")
	want = append(want, make([]byte, HeaderSize-len(want))...)
	h := fixedHeader()
	encoded, err := EncodeHeader(h, Limits{})
	if err != nil || !bytes.Equal(encoded, want) {
		t.Fatal("experimental header literal vector mismatch")
	}
	digest := sha256.Sum256(encoded)
	if hex.EncodeToString(digest[:]) != "0d047b4fc9675ebfe53c98efd71c2c5a03fb9c4fbd61bc7b3812a11975908adf" {
		t.Fatal("experimental header digest mismatch")
	}
	decoded, err := DecodeHeader(encoded, Limits{})
	if err != nil || decoded != h {
		t.Fatal("header roundtrip mismatch")
	}
	reader := bytes.NewReader(append(bytes.Clone(encoded), 0x7f))
	fromReader, err := ReadHeader(reader, Limits{})
	if err != nil || fromReader != h || reader.Len() != 1 {
		t.Fatal("header stream framing mismatch")
	}
	encoded[80] ^= 1
	if decoded.FileID == ([32]byte{}) || decoded != h {
		t.Fatal("decoded header aliased input buffer")
	}
}

func TestStrictHeaderBounds(t *testing.T) {
	h := fixedHeader()
	encoded, _ := EncodeHeader(h, Limits{})
	for n := range HeaderSize {
		if _, err := DecodeHeader(encoded[:n], Limits{}); err == nil {
			t.Fatalf("truncated header accepted at %d", n)
		}
	}
	if _, err := DecodeHeader(append(bytes.Clone(encoded), 0), Limits{}); err == nil {
		t.Fatal("overlong header accepted")
	}
	for _, offset := range append([]int{188, 189, 190, 191}, integerRange(272, HeaderSize)...) {
		mutated := bytes.Clone(encoded)
		mutated[offset] = 1
		if _, err := DecodeHeader(mutated, Limits{}); !errors.Is(err, ErrFormat) {
			t.Fatalf("reserved header byte accepted at %d", offset)
		}
	}
	for _, mutate := range []func(*Header){
		func(h *Header) { h.Major++ }, func(h *Header) { h.Minor++ }, func(h *Header) { h.Suite++ },
		func(h *Header) { h.Flags = 1 }, func(h *Header) { h.Descriptor.Role = 255 }, func(h *Header) { h.Epoch.Kind = 255 },
		func(h *Header) { h.FileID = [32]byte{} }, func(h *Header) { h.Epoch.DatabaseID = [16]byte{} },
		func(h *Header) { h.Epoch.RootSalt = [32]byte{} }, func(h *Header) { h.Epoch.EpochID = [32]byte{} },
		func(h *Header) { h.Epoch.KDFSalt[0] = 1 }, func(h *Header) { h.Epoch.KDF = DefaultArgon2Params() },
		func(h *Header) { h.Descriptor.PageSize = 513 }, func(h *Header) { h.Descriptor.PageSize = 1 },
		func(h *Header) { h.Descriptor.PageCount = 0 }, func(h *Header) { h.Descriptor.PageCount = math.MaxUint32 },
		func(h *Header) { h.Descriptor.RecordCount = math.MaxUint64 }, func(h *Header) { h.Descriptor.RecordCount = 0 },
		func(h *Header) { h.Descriptor.RecordCount-- },
	} {
		candidate := h
		mutate(&candidate)
		if _, err := EncodeHeader(candidate, Limits{}); err == nil {
			t.Fatal("invalid header accepted")
		}
	}
	for _, limits := range []Limits{
		{MaxPages: 1}, {MaxRecords: 3}, {MaxLogicalBytes: 1023}, {MaxRecordBytes: 511},
		{MaxPages: MaxPages + 1}, {MaxRecords: MaxRecords + 1}, {MaxLogicalBytes: MaxLogicalBytes + 1}, {MaxRecordBytes: MaxRecordBytes + 1},
	} {
		if _, err := DecodeHeader(encoded, limits); !errors.Is(err, ErrLimit) {
			t.Fatal("local/hard limit not enforced")
		}
	}
	password := h
	password.Epoch.Kind, password.Epoch.KDF, password.Epoch.KDFSalt[0] = PasswordArgon2id, DefaultArgon2Params(), 1
	pb, err := EncodeHeader(password, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := DecodeHeader(pb, Limits{}); err != nil || got != password {
		t.Fatal("password bootstrap roundtrip failed")
	}
	pb[22] = 0x10
	if _, err := DecodeHeader(pb, Limits{}); !errors.Is(err, ErrUnsupported) {
		t.Fatal("unknown Argon2 version accepted")
	}
	encoded[22] = ArgonVersion
	if _, err := DecodeHeader(encoded, Limits{}); !errors.Is(err, ErrUnsupported) {
		t.Fatal("raw mode accepted nonzero Argon2 version")
	}
}

func integerRange(start, end int) []int {
	var out []int
	for n := start; n < end; n++ {
		out = append(out, n)
	}
	return out
}

func TestRecordHeaderAndReaderBounds(t *testing.T) {
	h := RecordHeader{Role: RecordData, Sequence: 7, PlaintextSize: 4096}
	want := unhex(t, "475345520200000000000000000000070000100000000000")
	b, err := EncodeRecordHeader(h, Limits{})
	if err != nil || !bytes.Equal(b, want) {
		t.Fatal("record header literal vector mismatch")
	}
	if got, err := DecodeRecordHeader(b, Limits{}); err != nil || got != h {
		t.Fatal("record header roundtrip failed")
	}
	for n := range RecordHeaderSize {
		if _, err := DecodeRecordHeader(b[:n], Limits{}); err == nil {
			t.Fatal("truncated record header accepted")
		}
	}
	for _, offset := range []int{5, 6, 7, 20, 21, 22, 23} {
		changed := bytes.Clone(b)
		changed[offset] = 1
		if _, err := DecodeRecordHeader(changed, Limits{}); err == nil {
			t.Fatal("noncanonical record padding accepted")
		}
	}
	for _, candidate := range []RecordHeader{
		{Role: 0}, {Role: 4}, {Role: RecordMetadata, Sequence: math.MaxUint64},
		{Role: RecordMetadata, PlaintextSize: math.MaxUint32}, {Role: RecordData, PlaintextSize: 0},
		{Role: RecordData, PlaintextSize: 513}, {Role: RecordData, PlaintextSize: MaxPageSize + 1},
	} {
		if _, err := EncodeRecordHeader(candidate, Limits{}); err == nil {
			t.Fatal("invalid record header accepted")
		}
	}
	// Hostile length is rejected after the fixed prefix, without reading or
	// allocating the purported 4GiB body. This reader is entirely in memory.
	huge := bytes.Clone(b)
	binary.BigEndian.PutUint32(huge[16:20], math.MaxUint32)
	r := bytes.NewReader(append(huge, 1, 2, 3))
	if got, err := ReadRecord(r, Limits{}); !errors.Is(err, ErrLimit) || got != nil || r.Len() != 3 {
		t.Fatal("unbounded body read attempted")
	}
	valid := append(bytes.Clone(b), make([]byte, int(h.PlaintextSize)+TagSize)...)
	stream := bytes.NewReader(append(bytes.Clone(valid), 1))
	if got, err := ReadRecord(stream, Limits{}); err != nil || !bytes.Equal(got, valid) || stream.Len() != 1 {
		t.Fatal("one-record framing failed")
	}
	for _, size := range []int{0, 23, 24, len(valid) - 1} {
		if got, err := ReadRecord(bytes.NewReader(valid[:size]), Limits{}); err == nil || got != nil {
			t.Fatal("partial record returned")
		}
	}
}

func TestMalformedInputsBounded(t *testing.T) {
	// Deterministic in-process parser exercise: no subprocess, database path,
	// filesystem mutation, or expensive password derivation is used.
	rng := rand.New(rand.NewPCG(9, 17))
	for i := 0; i < 5000; i++ {
		b := make([]byte, rng.IntN(HeaderSize+100))
		for j := range b {
			b[j] = byte(rng.Uint32())
		}
		_, _ = DecodeHeader(b, Limits{})
		_, _ = ReadHeader(bytes.NewReader(b), Limits{})
		_, _ = DecodeRecordHeader(b, Limits{})
		_, _ = ReadRecord(bytes.NewReader(b), Limits{MaxRecordBytes: 4096})
	}
}

func FuzzHeaderCodec(f *testing.F) {
	valid, _ := EncodeHeader(fixedHeader(), Limits{})
	f.Add(valid)
	f.Add([]byte{})
	f.Add([]byte(Magic))
	f.Fuzz(func(t *testing.T, b []byte) {
		h, err := DecodeHeader(b, Limits{})
		if err != nil {
			return
		}
		roundtrip, err := EncodeHeader(h, Limits{})
		if err != nil || !bytes.Equal(roundtrip, b) {
			t.Fatal("accepted header is not canonical")
		}
	})
}

func FuzzRecordFraming(f *testing.F) {
	valid, _ := EncodeRecordHeader(RecordHeader{Role: RecordMetadata, PlaintextSize: 1}, Limits{})
	valid = append(valid, make([]byte, 1+TagSize)...)
	f.Add(valid)
	f.Add([]byte{})
	f.Add(bytes.Repeat([]byte{255}, RecordHeaderSize))
	f.Fuzz(func(t *testing.T, b []byte) {
		out, err := ReadRecord(bytes.NewReader(b), Limits{MaxRecordBytes: 4096})
		if err != nil {
			if out != nil {
				t.Fatal("failure exposed partial record")
			}
			return
		}
		if len(out) > RecordHeaderSize+4096+TagSize || len(out) > len(b) || !bytes.Equal(out, b[:len(out)]) {
			t.Fatal("reader violated its length bounds")
		}
	})
}
