//go:build darwin

package encstore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/darwinfile"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/fileview"
	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// Compile-only on this Linux executor. These histories use intact ordinary
// files and successful native operations; no injected OS failure, tamper,
// namespace mutation race, subprocess, interrupted publication or recovery.
func TestDarwinEncryptedAggregateLocksAndProbeReuse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ordinary.gse")
	ns, e := newNamespace(path)
	if e != nil {
		t.Fatal(e)
	}
	defer ns.dir.Close()
	a, _, e := ns.checked(path+".gse-lock", os.O_RDWR|os.O_CREATE|os.O_EXCL)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	b, _, e := ns.checked(path+".gse-lock", os.O_RDWR)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	if e = lockFile(a, true); e != nil {
		t.Fatal(e)
	}
	if e = lockFile(b, true); e != nil {
		t.Fatal(e)
	}
	if e = rangeLock(a, writerByte, false, false); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 1000; i++ {
		probe, _, e := checkedFile(path+".gse-lock", os.O_RDONLY)
		if e != nil {
			t.Fatal(e)
		}
		if e = probe.Close(); e != nil {
			t.Fatal(e)
		}
	}
	darwinfile.Process.Lock()
	count := len(a.descriptor.node.descriptors)
	kernel := a.descriptor.node.locks.kernel
	darwinfile.Process.Unlock()
	if count != 3 || kernel != [3]uint8{1, 2, 0} {
		t.Fatal("unbounded probes or lost process lock", count, kernel)
	}
	if e = rangeLock(b, writerByte, false, false); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	if e = lockFile(a, false); !errors.Is(e, ErrBusy) {
		t.Fatal("rekey ignored local lease", e)
	}
	if e = b.Close(); e != nil {
		t.Fatal(e)
	}
	if e = lockFile(a, false); e != nil {
		t.Fatal("quiescent atomic upgrade", e)
	}
	if e = lockFile(a, true); e != nil {
		t.Fatal("atomic downgrade", e)
	}
	if e = a.Close(); e != nil {
		t.Fatal(e)
	}
	darwinfile.Process.Lock()
	left := len(a.descriptor.node.descriptors)
	darwinfile.Process.Unlock()
	if left != 0 {
		t.Fatal("last lock did not drain all deferred descriptors", left)
	}
}
func TestDarwinEncryptedPlainProbeCloseCoordination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cross-domain.gse-lock")
	a, _, e := checkedFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if e = lockFile(a, true); e != nil {
		t.Fatal(e)
	}
	if e = rangeLock(a, writerByte, false, false); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 32; i++ {
		if _, e := fileview.ReadFile(path); e != nil {
			t.Fatal(e)
		}
		probe, e := vfs.Open(path, vfs.OpenOptions{Role: vfs.Database, Flags: os.O_RDONLY})
		if e != nil {
			t.Fatal(e)
		}
		if e = probe.Close(); e != nil {
			t.Fatal(e)
		}
	}
	b, _, e := checkedFile(path, os.O_RDWR)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	if e = rangeLock(b, writerByte, false, false); !errors.Is(e, ErrBusy) {
		t.Fatal("plain probe erased encrypted writer", e)
	}
	if e = b.Close(); e != nil {
		t.Fatal(e)
	}
	if e = a.Close(); e != nil {
		t.Fatal(e)
	}
	// Reverse direction: encrypted managed probes must respect plain process holds.
	plain, e := vfs.Open(path, vfs.OpenOptions{Role: vfs.Database, Flags: os.O_RDWR})
	if e != nil {
		t.Fatal(e)
	}
	defer plain.Close()
	if e = plain.LockDB(vfs.Shared); e != nil {
		t.Fatal(e)
	}
	if e = plain.LockDB(vfs.Reserved); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 32; i++ {
		probe, _, e := checkedFile(path, os.O_RDONLY)
		if e != nil {
			t.Fatal(e)
		}
		if e = probe.Close(); e != nil {
			t.Fatal(e)
		}
	}
	peer, e := vfs.Open(path, vfs.OpenOptions{Role: vfs.Database, Flags: os.O_RDWR})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	if e = peer.LockDB(vfs.Shared); e != nil {
		t.Fatal(e)
	}
	if e = peer.LockDB(vfs.Reserved); !errors.Is(e, vfs.ErrBusy) {
		t.Fatal("encrypted probe erased plain reservation", e)
	}
}

func TestDarwinEncryptedPathPolicyMemory(t *testing.T) {
	for _, name := range []string{"ordinary.gse", "Mixed Case.gse", strings.Repeat("a", 177)} {
		if e := encryptedDarwinTarget(name); e != nil {
			t.Fatal(name, e)
		}
	}
	for _, name := range []string{"", ".", "..", "nested/path", "unicode-Ω.gse", strings.Repeat("a", 178)} {
		if e := encryptedDarwinTarget(name); !errors.Is(e, ErrUnsupported) {
			t.Fatal(name, e)
		}
	}
	for _, name := range []string{"FAMILY.GSE", "FAMILY.GSE.gse-lock", "FAMILY.GSE.gse-" + strings.Repeat("a", 64) + ".retained"} {
		ok, e := platformFamilyEntry(name, "family.gse", nil)
		if e != nil || !ok {
			t.Fatal(name, ok, e)
		}
	}
	if _, e := platformFamilyEntry("FAMILY.GSE.gse-"+strings.Repeat("A", 64)+".retained", "family.gse", nil); !errors.Is(e, ErrUnsupported) {
		t.Fatal("noncanonical ID suffix", e)
	}
}
