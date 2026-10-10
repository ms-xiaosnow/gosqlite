package journal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestSQLiteRollbackFormatInMemory(t *testing.T) {
	h := Header{Records: 2, Nonce: 0x01020304, OriginalPages: 7, SectorSize: 4096, PageSize: 512}
	before, err := EncodeHeader(h, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before[:12], make([]byte, 12)) {
		t.Fatal("early header is valid")
	}
	b, err := EncodeHeader(h, true)
	if err != nil {
		t.Fatal(err)
	}
	prefix := []byte{0xd9, 0xd5, 0x05, 0xf9, 0x20, 0xa1, 0x63, 0xd7, 0, 0, 0, 2, 1, 2, 3, 4, 0, 0, 0, 7, 0, 0, 16, 0, 0, 0, 2, 0}
	if len(b) != 4096 || !bytes.Equal(b[:28], prefix) {
		t.Fatal("SQLite header layout mismatch")
	}
	parsed, err := ParseHeader(b)
	if err != nil || parsed != h {
		t.Fatalf("%+v %v", parsed, err)
	}
	p := make([]byte, 512)
	p[112] = 7
	p[312] = 9
	p[511] = 99
	rec, err := EncodeRecord(h, 3, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec) != 520 || binary.BigEndian.Uint32(rec) != 3 || binary.BigEndian.Uint32(rec[516:]) != h.Nonce+16 {
		t.Fatal("SQLite record/checksum layout mismatch")
	}
	n, q, err := ParseRecord(h, rec)
	if err != nil || n != 3 || !bytes.Equal(p, q) {
		t.Fatal(n, err)
	}
	q[0] = 1
	if rec[4] != 0 {
		t.Fatal("parser result aliases input")
	}
}

// Malformed parser inputs remain byte slices; no database or journal file is
// created, changed, recovered, truncated, or subjected to a fault.
func TestRejectMalformedJournalBytesInMemory(t *testing.T) {
	h := Header{Records: 1, Nonce: 19, OriginalPages: 2, SectorSize: 512, PageSize: 512}
	b, _ := EncodeHeader(h, true)
	for i := 0; i < len(b); i++ {
		if _, e := ParseHeader(b[:i]); !errors.Is(e, ErrFormat) {
			t.Fatalf("short header %d: %v", i, e)
		}
	}
	bad := append([]byte(nil), b...)
	bad[0] = 0
	if _, e := ParseHeader(bad); !errors.Is(e, ErrFormat) {
		t.Fatal(e)
	}
	page := make([]byte, 512)
	rec, _ := EncodeRecord(h, 2, page)
	rec[4+312]++
	if _, _, e := ParseRecord(h, rec); !errors.Is(e, ErrFormat) {
		t.Fatal(e)
	}
	if _, e := EncodeRecord(h, 0, page); !errors.Is(e, ErrFormat) {
		t.Fatal(e)
	}
	if _, e := EncodeRecord(h, 3, page); !errors.Is(e, ErrFormat) {
		t.Fatal(e)
	}
	if _, e := EncodeRecord(h, 2, page[:10]); !errors.Is(e, ErrFormat) {
		t.Fatal(e)
	}
}
