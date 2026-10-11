package encstore

import (
	"bytes"
	"encoding/binary"
	"os"
	"unicode/utf8"
)

// Darwin's 64-bit direntry wire offsets, independently checked against Apple's
// bsd/sys/dirent.h and the pinned x/sys Dirent on both supported architectures.
// This parser has no descriptor, syscall, or injected native error dependency.
const darwinDirNameOffset = 21
const darwinDirBufferBytes = 64 << 10

type directoryName struct {
	name string
	kind uint8
}

func (d directoryName) Name() string { return d.name }
func (d directoryName) IsDir() bool  { return d.kind == 4 }
func (d directoryName) Type() os.FileMode {
	switch d.kind {
	case 1:
		return os.ModeNamedPipe
	case 2:
		return os.ModeDevice | os.ModeCharDevice
	case 4:
		return os.ModeDir
	case 6:
		return os.ModeDevice
	case 8:
		return 0
	case 10:
		return os.ModeSymlink
	case 12:
		return os.ModeSocket
	default:
		return os.ModeIrregular
	}
}

// Inventory consumes only names and reopens each selected member through its
// checked namespace. No DirEntry lazily retains or opens an unowned descriptor.
func (d directoryName) Info() (os.FileInfo, error) { return nil, ErrUnsupported }
func nextDarwinDirectoryName(raw []byte) (entry directoryName, used int, skip bool, err error) {
	if len(raw) < darwinDirNameOffset+1 {
		return entry, 0, false, ErrChanged
	}
	used = int(binary.LittleEndian.Uint16(raw[16:18]))
	length := int(binary.LittleEndian.Uint16(raw[18:20]))
	if used < darwinDirNameOffset+1 || used > len(raw) || used%4 != 0 || length > 1023 || length >= used-darwinDirNameOffset {
		return entry, 0, false, ErrChanged
	}
	name := raw[darwinDirNameOffset : darwinDirNameOffset+length]
	if raw[darwinDirNameOffset+length] != 0 || bytes.IndexByte(name, 0) >= 0 || bytes.IndexByte(name, '/') >= 0 {
		return entry, 0, false, ErrChanged
	}
	if binary.LittleEndian.Uint64(raw[:8]) == 0 {
		return entry, used, true, nil
	}
	if length == 0 {
		return entry, 0, false, ErrChanged
	}
	entry = directoryName{name: string(name), kind: raw[20]}
	return entry, used, entry.name == "." || entry.name == "..", nil
}

// Only the Apple filename-equivalence domain proved below may use the new
// negative classification. HFS subtype 128 is legacy Mac OS Standard, not HFS+;
// unknown/reserved subtypes and malformed native type names remain unsupported.
func darwinUnicodeInventoryFilesystem(name [16]byte, subtype uint32, local bool) bool {
	end := bytes.IndexByte(name[:], 0)
	if !local || end < 0 {
		return false
	}
	switch string(name[:end]) {
	case "apfs":
		return true
	case "hfs":
		return subtype <= 3
	default:
		return false
	}
}

// darwinASCIIBaseMismatch proves only a negative: a printable ASCII mismatch
// inside the requested ASCII basename, before any non-ASCII/control byte, cannot
// be repaired by later canonical decomposition, combining marks or ignorables.
// It never compares Unicode, skips over it, or treats a matching prefix as proof
// of membership. See docs/darwin-unicode-inventory.md for the Apple-source proof.
func darwinASCIIBaseMismatch(name, base string) bool {
	// Do not turn a future Unicode-target expansion into an omission of reverse
	// aliases (for example an ASCII sibling of a Kelvin-sign root).
	for i := range len(base) {
		if base[i] < 32 || base[i] > 126 {
			return false
		}
	}
	for i := 0; i < len(name) && i < len(base); i++ {
		a, b := name[i], base[i]
		if a < 32 || a > 126 || b < 32 || b > 126 {
			return false
		}
		if a >= 'A' && a <= 'Z' {
			a += 32
		}
		if b >= 'A' && b <= 'Z' {
			b += 32
		}
		if a != b {
			return true
		}
	}
	return false
}

// darwinFamilyEntry is the name-only part of Apple-domain classification. A
// non-ASCII nonmember result is provisional until platformFamilyEntry verifies
// the actual enumeration descriptor's filesystem with Fstatfs.
func darwinFamilyEntry(name, base string) (bool, error) {
	// The target policy still requires an ASCII basename. A non-ASCII sibling
	// can be omitted only after a pre-Unicode ASCII mismatch proves it cannot
	// share that basename. All ambiguous Unicode names remain unsupported, even
	// if they look unrelated; accepting them as possible roots could hide an
	// unknown family suffix. Invalid UTF-8 retains the previous fail-closed rule.
	for i := range len(name) {
		if name[i] >= 128 {
			if utf8.ValidString(name) && darwinASCIIBaseMismatch(name, base) {
				return false, nil
			}
			return false, ErrUnsupported
		}
	}
	return foldedFamilyEntry(name, base, func(s, p string) (string, bool, error) {
		if len(s) < len(p) {
			return "", false, nil
		}
		for i := range len(p) {
			a, b := s[i], p[i]
			if a >= 'A' && a <= 'Z' {
				a += 32
			}
			if b >= 'A' && b <= 'Z' {
				b += 32
			}
			if a != b {
				return "", false, nil
			}
		}
		return s[len(p):], true, nil
	})
}
