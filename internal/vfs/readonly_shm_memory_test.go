package vfs

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestReadOnlySHMLockCapabilityMemory(t *testing.T) {
	for _, writable := range []bool{false, true} {
		for _, exclusive := range []bool{false, true} {
			err := readonlySHMLockAccess(writable, exclusive)
			if !writable && exclusive {
				if !errors.Is(err, ErrReadOnly) {
					t.Fatalf("readonly exclusive: %v", err)
				}
			} else if err != nil {
				t.Fatalf("writable=%v exclusive=%v: %v", writable, exclusive, err)
			}
		}
	}
}

// These guards read only source text. They never create a database, acquire a
// descriptor for storage, or invoke a native lock or mapping operation.
func TestReadOnlySHMSourceGuard(t *testing.T) {
	lock := readonlySHMSourceSection(t, "locks.go", "func (f *File) LockSHM(")
	readonlySHMRequireOrder(t, lock,
		"readonlySHMLockAccess(f.writable, exclusive)",
		"f.slots.prepare(slot, n, exclusive)",
		"nativeSHMLock(f, slot, n, exclusive, false)",
		"f.slots.assign(slot, n, mode)")
	unlock := readonlySHMSourceSection(t, "locks.go", "func (f *File) UnlockSHM(")
	if strings.Contains(unlock, "readonlySHMLockAccess") {
		t.Fatal("read-only capability must not prevent release")
	}
	dms := readonlySHMSourceSection(t, "native_linux.go", "func nativeAttachDMS(")
	readonlySHMRequireOrder(t, dms,
		"syscall.FcntlFlock(f.raw.Fd(), 36, &l)",
		"readonlySHMDMSDecision(f.writable, l.Type)",
		"if err != nil {\n\t\treturn err\n\t}",
		"if initialWriter {",
		"ofdLock(f, syscall.F_WRLCK, DMSByte, 1)",
		"f.raw.Truncate(3)",
		"err = ofdLock(f, syscall.F_RDLCK, DMSByte, 1)")
	mapping := readonlySHMSourceSection(t, "native_linux.go", "func nativeMap(")
	if !strings.Contains(mapping, "readonlySHMProtection(f.writable), syscall.MAP_SHARED") {
		t.Fatal("mapping must use managed access capability")
	}
	if strings.Contains(mapping, "syscall.PROT_WRITE") {
		t.Fatal("mapping bypasses read-only protection policy")
	}
	owner := readonlySHMSourceSection(t, "mapping.go", "func (f *File) MapShared(")
	readonlySHMRequireOrder(t, owner,
		"f.role != SHM || !f.dms",
		"nativeMap(f, base, length)",
		"owner: f",
		"f.maps[m] = struct{}{}")
}

func readonlySHMSourceSection(t *testing.T, path, signature string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	start := strings.Index(s, signature)
	if start < 0 {
		t.Fatalf("%s: missing %s", path, signature)
	}
	s = s[start:]
	if end := strings.Index(s[1:], "\nfunc "); end >= 0 {
		s = s[:end+1]
	}
	return s
}

func readonlySHMRequireOrder(t *testing.T, source string, tokens ...string) {
	t.Helper()
	for _, token := range tokens {
		at := strings.Index(source, token)
		if at < 0 {
			t.Fatalf("source sequence missing %q", token)
		}
		source = source[at+len(token):]
	}
}
