package encstore

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

// These fabricated byte buffers never reach a filesystem or native syscall.
func directoryMemoryRecord(name string, ino uint64) []byte {
	n := (darwinDirNameOffset + len(name) + 1 + 7) &^ 7
	raw := make([]byte, n)
	binary.LittleEndian.PutUint64(raw, ino)
	binary.LittleEndian.PutUint16(raw[16:], uint16(n))
	binary.LittleEndian.PutUint16(raw[18:], uint16(len(name)))
	raw[20] = 8
	copy(raw[21:], name)
	return raw
}
func TestDarwinDirectoryMemoryWireNamesAndBoundaries(t *testing.T) {
	for _, name := range []string{"a", "ordinary.gse", "Ω-parent", string(bytes.Repeat([]byte{'x'}, 1023))} {
		raw := directoryMemoryRecord(name, 1)
		e, n, skip, err := nextDarwinDirectoryName(raw)
		if err != nil || e.Name() != name || n != len(raw) || skip || e.Type() != 0 {
			t.Fatal(name, n, skip, err)
		}
	}
	for _, name := range []string{".", ".."} {
		_, _, skip, e := nextDarwinDirectoryName(directoryMemoryRecord(name, 1))
		if e != nil || !skip {
			t.Fatal(name, e)
		}
	}
	if _, _, skip, e := nextDarwinDirectoryName(directoryMemoryRecord("", 0)); e != nil || !skip {
		t.Fatal("zero inode", e)
	}
}
func TestDarwinDirectoryMemoryRejectsMalformedRecord(t *testing.T) {
	good := directoryMemoryRecord("ordinary.gse", 1)
	for n := 0; n < len(good); n++ {
		if _, _, _, e := nextDarwinDirectoryName(good[:n]); e == nil {
			t.Fatal("truncated record", n)
		}
	}
	for _, change := range []func([]byte){
		func(b []byte) { binary.LittleEndian.PutUint16(b[16:], 0) },
		func(b []byte) { binary.LittleEndian.PutUint16(b[16:], 23) },
		func(b []byte) { binary.LittleEndian.PutUint16(b[18:], 1024) },
		func(b []byte) { binary.LittleEndian.PutUint16(b[18:], 0) },
		func(b []byte) { b[darwinDirNameOffset] = 0 },
		func(b []byte) { b[darwinDirNameOffset] = '/' },
		func(b []byte) { b[darwinDirNameOffset+len("ordinary.gse")] = 1 },
	} {
		b := bytes.Clone(good)
		change(b)
		if _, _, _, e := nextDarwinDirectoryName(b); e == nil {
			t.Fatal("malformed record accepted")
		}
	}
}
func TestDarwinDirectoryMemoryReturnedExtentOnly(t *testing.T) {
	a, b := directoryMemoryRecord("first", 1), directoryMemoryRecord("second", 2)
	returned := append(bytes.Clone(a), b...)
	buffer := append(bytes.Clone(returned), 0xde, 0xad, 0xbe, 0xef)
	cursor := 0
	var names []string
	for cursor < len(returned) {
		entry, n, skip, e := nextDarwinDirectoryName(buffer[cursor:len(returned)])
		if e != nil {
			t.Fatal(e)
		}
		cursor += n
		if !skip {
			names = append(names, entry.Name())
		}
	}
	if cursor != len(returned) || len(names) != 2 || names[0] != "first" || names[1] != "second" {
		t.Fatal(names, cursor)
	}
}

func TestDarwinDirectoryMemoryASCIIInventoryPolicy(t *testing.T) {
	for _, name := range []string{"K.gse", "k.GSE.gse-lock", "K.gse.gse-" + strings.Repeat("a", 64) + ".retained"} {
		member, e := darwinFamilyEntry(name, "k.gse")
		if e != nil || !member {
			t.Fatal(name, member, e)
		}
	}
	for _, name := range []string{"K.gse", "K.gse.gse-" + strings.Repeat("a", 64) + ".retained"} {
		if member, e := darwinFamilyEntry(name, "k.gse"); !errors.Is(e, ErrUnsupported) || member {
			t.Fatal("Unicode accounting alias", name, member, e)
		}
	}
	if member, e := darwinFamilyEntry("unrelated.txt", "k.gse"); e != nil || member {
		t.Fatal(member, e)
	}
	if member, e := darwinFamilyEntry("unrelated-Ω.txt", "k.gse"); e != nil || member {
		t.Fatal(member, e)
	}
}
