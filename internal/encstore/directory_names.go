package encstore

import (
	"bytes"
	"encoding/binary"
	"os"
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

func darwinFamilyEntry(name, base string) (bool, error) {
	// APFS preserves names while looking up canonically equivalent Unicode.
	// Refuse all non-ASCII siblings, not only requested root names: a preserved
	// Kelvin-sign prefix can otherwise alias ASCII K and evade family accounting.
	for i := range len(name) {
		if name[i] >= 128 {
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
