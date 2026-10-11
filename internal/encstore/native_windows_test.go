//go:build windows && amd64

package encstore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

// These tests are compile-only on non-Windows hosts. They exercise ordinary
// cooperating ownership and successful publication, never injected native
// errors, hostile namespace mutation, disk tamper, or interrupted operations.
func TestWindowsEncryptedExactLocksAndProbeLifetime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locks.gse")
	ns, err := newNamespace(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ns.dir.Close()
	a, _, err := ns.checked(path+".gse-lock", os.O_RDWR|os.O_CREATE|os.O_EXCL)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, _, err := ns.checked(path+".gse-lock", os.O_RDWR)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if err = lockFile(a, true); err != nil {
		t.Fatal(err)
	}
	if err = lockFile(b, true); err != nil {
		t.Fatal(err)
	}
	if err = rangeLock(a, writerByte, false, false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		probe, _, err := ns.checked(path+".gse-lock", os.O_RDONLY)
		if err != nil {
			t.Fatal(err)
		}
		if err = probe.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err = rangeLock(b, writerByte, false, false); !errors.Is(err, ErrBusy) {
		t.Fatal("sibling close lost writer", err)
	}
	if err = rangeLock(a, captureByte, true, false); err != nil {
		t.Fatal(err)
	}
	if err = rangeLock(b, captureByte, true, false); err != nil {
		t.Fatal(err)
	}
	if err = rangeLock(a, captureByte, false, true); err != nil {
		t.Fatal(err)
	}
	if err = rangeLock(b, captureByte, false, true); err != nil {
		t.Fatal(err)
	}
	if err = rangeLock(a, captureByte, false, false); err != nil {
		t.Fatal(err)
	}
	if err = rangeLock(b, captureByte, true, false); !errors.Is(err, ErrBusy) {
		t.Fatal("capture gate", err)
	}
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	if err = rangeLock(b, writerByte, false, false); err != nil {
		t.Fatal("writer not released", err)
	}
	if err = b.Close(); err != nil {
		t.Fatal(err)
	}
	c, _, err := ns.checked(path+".gse-lock", os.O_RDWR)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err = lockFile(c, false); err != nil {
		t.Fatal(err)
	}
	if err = rangeLock(c, lifecycleByte, true, false); err != nil {
		t.Fatal("stacked downgrade", err)
	}
	d, _, err := ns.checked(path+".gse-lock", os.O_RDONLY)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err = lockFile(d, true); err != nil {
		t.Fatal("downgraded shared lease", err)
	}
}

func TestWindowsEncryptedStandaloneParentRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.gse")
	file, info, err := checkedFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL)
	if err != nil {
		t.Fatal(err)
	}
	// openFile has released its public directory lease; this child still owns
	// the actual pin and supports relative identity probes until physical close.
	parent := file.parent
	if parent == nil || !parent.closed || parent.raw == nil || parent.children != 1 {
		t.Fatal("directory pin not retained")
	}
	if err = platformDirectoryCheck(parent, parent.path, nil); !errors.Is(err, encryption.ErrClosed) {
		t.Fatal("closed public directory reuse", err)
	}
	if err = stableFile(file, path, info); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	if parent.raw != nil || parent.children != 0 {
		t.Fatal("last child did not retire directory")
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
}
func TestWindowsEncryptedPathPolicyMemory(t *testing.T) {
	for _, path := range []string{`C:\data\valid.gse`, `\\?\C:\data\valid.gse`, `ordinary Ω.gse`} {
		if err := platformInputPath(path); err != nil {
			t.Fatal(path, err)
		}
	}
	for _, path := range []string{`\\server\share\db`, `\\.\C:\db`, `C:db`, `C:\db:stream`, `C:\AUX.gse`, `C:\db.`, `C:\db `, `C:\..\db`, string([]byte{0xff})} {
		if err := platformInputPath(path); !errors.Is(err, ErrUnsupported) {
			t.Fatal("ambiguous path accepted", path, err)
		}
	}
	id := strings.Repeat("a", 64)
	for _, name := range []string{"ORDINARY Ω.GSE", "ORDINARY Ω.GSE.GSE-LOCK", "ORDINARY Ω.GSE.GSE-" + id + ".base"} {
		member, err := platformFamilyEntry(name, "ordinary ω.gse", nil)
		if err != nil || !member {
			t.Fatal("native ordinal family prefix", name, err)
		}
	}
	if _, err := platformFamilyEntry("ORDINARY Ω.GSE.GSE-"+id+".BASE", "ordinary ω.gse", nil); !errors.Is(err, ErrUnsupported) {
		t.Fatal("noncanonical family suffix", err)
	}

	if _, err := canonicalPath(`C:\path-never-opened\` + strings.Repeat("a", 178)); !errors.Is(err, ErrUnsupported) {
		t.Fatal("family envelope preflight", err)
	}

}

func TestWindowsEncryptedSharedRekeyCapability(t *testing.T) {
	// This target-only capability check reads the native version. It does not
	// open a database or replace the portable guarded-conversion law tests.
	if err := platformOperation(encryption.OperationRekeyTarget, WALShared); err != nil {
		t.Fatal("supported Windows target rejected shared rekey", err)
	}
}
