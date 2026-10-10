package encstore

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

func TestSnapshotManifestCanonicalMemory(t *testing.T) {
	m := manifest{BaseBytes: 0x0102030405060708}
	for i := range m.BaseID {
		m.BaseID[i] = byte(i + 1)
		m.BaseHash[i] = byte(i + 33)
		m.PageOneHash[i] = byte(i + 65)
	}
	const golden = "4753454d4e4630310102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f2001020304050607082122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f6000000000000000000000000000000000"
	if hex.EncodeToString(m.encode()) != golden {
		t.Fatal("manifest encoding changed")
	}
	got, err := decodeManifest(m.encode())
	if err != nil || got != m {
		t.Fatal(got, err)
	}
	for _, off := range []int{0, 7, 112, 127} {
		b := m.encode()
		b[off] ^= 1
		if _, err = decodeManifest(b); err == nil {
			t.Fatal("bad canonical manifest accepted", off)
		}
	}
	for _, b := range [][]byte{m.encode()[:127], append(m.encode(), 0)} {
		if _, err = decodeManifest(b); err == nil {
			t.Fatal("nonexact extent accepted")
		}
	}
	for _, role := range []encryption.FileRole{encryption.FileBase, encryption.FileBackup} {
		b := metadata(role, m.PageOneHash)
		h, err := parseMetadata(b, role)
		if err != nil || h != m.PageOneHash {
			t.Fatal(err)
		}
		b[63] = 1
		if _, err = parseMetadata(b, role); err == nil {
			t.Fatal("metadata padding accepted")
		}
	}
	if bytes.Equal(metadata(encryption.FileBase, m.PageOneHash), metadata(encryption.FileBackup, m.PageOneHash)) {
		t.Fatal("base/backup metadata ambiguity")
	}
}
func TestInventoryAccountingMemory(t *testing.T) {
	for _, tc := range []struct {
		total, amount, max uint64
		good               bool
	}{{0, 0, 0, true}, {5, 7, 12, true}, {5, 8, 12, false}, {13, 0, 12, false}, {math.MaxUint64, 1, math.MaxUint64, false}} {
		_, err := charge(tc.total, tc.amount, tc.max)
		if (err == nil) != tc.good {
			t.Fatal(tc, err)
		}
	}
	base := "private.gse"
	id := string(bytes.Repeat([]byte{'a'}, 64))
	for _, suffix := range []string{".base", ".retained", ".pending"} {
		ok, err := familyEntry(base+".gse-"+id+suffix, base)
		if !ok || err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{base + ".gse-short.base", base + ".gse-" + id + ".wal", base + ".gse-" + id + ".unknown"} {
		if _, err := familyEntry(name, base); err == nil {
			t.Fatal("unknown family member accepted")
		}
	}
	if _, _, _, err := (Options{MaxGenerations: HardMaxGenerations + 1}).retentionLimits(); !errors.Is(err, encryption.ErrLimit) {
		t.Fatal(err)
	}
}
