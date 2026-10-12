//go:build darwin && manual_darwin_inventory

package encstore

// SOURCE-ONLY candidate. This test has never been compiled or run. It creates
// ordinary tiny metadata samples, not encrypted database-format fixtures.
// See docs/testing/darwin-inventory-manual-zh.md before separate authorization.
import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/darwinfile"
	"golang.org/x/sys/unix"
)

type manualDarwinInventoryOwner struct {
	name string
	file *managedFile
	parent *manualDarwinInventoryOwner
	attempted, retired bool
}

type manualDarwinInventoryLedger struct {
	owners []*manualDarwinInventoryOwner
	diagnostics []error
	fixture string
	blocked bool
}

// A failed run keeps strong references as well as the production registry's
// ownership tokens until process exit. No finalizer, retry, or deletion exists.
var manualDarwinInventoryRetained *manualDarwinInventoryLedger

func (l *manualDarwinInventoryLedger) add(name string, f *managedFile, parent *manualDarwinInventoryOwner) *manualDarwinInventoryOwner {
	if f == nil {
		return nil
	}
	o := &manualDarwinInventoryOwner{name: name, file: f, parent: parent}
	l.owners = append(l.owners, o)
	return o
}

func (l *manualDarwinInventoryLedger) record(err error) {
	if err != nil {
		l.diagnostics = append(l.diagnostics, err)
	}
}

// Read ownership facts only. This never calls RetryCleanup/closeAll/Drain.
// Unknown constructor or inventory-owned leases and dependency barriers retain
// all enclosing owners. The sole selected test must start with an empty registry.
func (l *manualDarwinInventoryLedger) known(t *testing.T) bool {
	encryptedCleanups.mu.Lock()
	groups := make([]*cleanupGroup, 0, len(encryptedCleanups.groups))
	for g := range encryptedCleanups.groups {
		groups = append(groups, g)
	}
	leases := encryptedCleanups.leases
	encryptedCleanups.mu.Unlock()
	unknown, barriers := 0, 0
	for _, g := range groups {
		g.mu.Lock()
		if g.terminal || len(g.dependencies) != 0 || g.callbackTerminal.Load() {
			barriers++
		}
		for resource := range g.files {
			found := false
			for _, o := range l.owners {
				if resource == o.file {
					found = true
					break
				}
			}
			if !found {
				unknown++
			}
		}
		g.mu.Unlock()
	}
	if unknown != 0 || barriers != 0 {
		l.blocked = true
		t.Errorf("OWNERSHIP unresolved: groups=%d leases=%d unknown=%d barriers=%d; retained, no retry", len(groups), leases, unknown, barriers)
		return false
	}
	return !l.blocked
}

func (l *manualDarwinInventoryLedger) retire(t *testing.T, o *manualDarwinInventoryOwner) bool {
	if o == nil || o.retired {
		return true
	}
	if o.attempted || !l.known(t) {
		return false
	}
	for _, child := range l.owners {
		if child.parent == o && !child.retired {
			t.Errorf("RETIRE %s retained: child %s not retired", o.name, child.name)
			return false
		}
	}
	o.file.mu.Lock()
	darwinfile.Process.Lock()
	children := o.file.children
	darwinfile.Process.Unlock()
	o.file.mu.Unlock()
	if children != 0 {
		l.blocked = true
		t.Errorf("RETIRE %s retained: native children=%d", o.name, children)
		return false
	}
	o.attempted = true // Record consuming attempt before Close, never retry it.
	err := o.file.Close()
	l.record(err)
	complete := o.file.RetirementComplete()
	o.file.mu.Lock()
	darwinfile.Process.Lock()
	physical := o.file.descriptor != nil && o.file.descriptor.raw == nil && o.file.children == 0
	darwinfile.Process.Unlock()
	o.file.mu.Unlock()
	o.retired = complete && physical
	t.Logf("RETIRE %s attempted=true complete=%t physical=%t error=%v", o.name, complete, physical, err)
	if err != nil || !o.retired {
		t.Errorf("RETIRE %s incomplete or diagnostic; no second Close: %v", o.name, err)
	}
	if !o.retired {
		l.blocked = true
	}
	return err == nil && o.retired
}

func (l *manualDarwinInventoryLedger) finish(t *testing.T) {
	for i := len(l.owners)-1; i >= 0; i-- {
		l.retire(t, l.owners[i])
	}
	left := 0
	for _, o := range l.owners {
		if !o.retired {
			left++
			t.Logf("RETAINED_OWNER name=%s attempted=%t", o.name, o.attempted)
		}
	}
	encryptedCleanups.mu.Lock()
	groups, leases := len(encryptedCleanups.groups), encryptedCleanups.leases
	encryptedCleanups.mu.Unlock()
	t.Logf("RETAINED_FIXTURE path=%q owners=%d unresolved=%d registry_groups=%d registry_leases=%d diagnostics=%d", l.fixture, len(l.owners), left, groups, leases, len(l.diagnostics))
	if left != 0 || groups != 0 || leases != 0 || l.blocked {
		t.Error("ASSERT retirement-complete FAIL; retain source, logs, fixture, and unresolved ownership; no automatic retry")
		return
	}
	t.Log("ASSERT retirement-complete PASS")
}

// Caller supplies the root. Even a symlink in its ancestry is rejected.
// The /private spelling is therefore required for macOS /tmp or /var aliases.
func manualDarwinInventoryRoot(path string) (unix.Stat_t, error) {
	var st unix.Stat_t
	if path == "" || len(path) > 1024 || !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
		return st, ErrUnsupported
	}
	part := "/"
	for _, component := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		part = filepath.Join(part, component)
		if err := unix.Lstat(part, &st); err != nil {
			return st, err
		}
		if st.Mode&unix.S_IFMT != unix.S_IFDIR {
			return st, ErrUnsupported
		}
	}
	if st.Uid != uint32(unix.Geteuid()) || st.Mode&07777 != 0700 {
		return st, ErrUnsupported
	}
	return st, nil
}

func manualDarwinInventoryIdentity(f *managedFile) (unix.Stat_t, unix.Statfs_t, error) {
	var st unix.Stat_t
	var fs unix.Statfs_t
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, err := f.rawLocked()
	if err != nil {
		return st, fs, err
	}
	if err = unix.Fstat(int(raw.Fd()), &st); err != nil {
		return st, fs, err
	}
	err = unix.Fstatfs(int(raw.Fd()), &fs)
	return st, fs, err
}

func manualDarwinInventoryMkdir(root *managedFile, name string) error {
	root.mu.Lock()
	defer root.mu.Unlock()
	raw, err := root.rawLocked()
	if err != nil {
		return err
	}
	// One exclusive mkdirat attempt. EEXIST is fatal, with no retry or fallback.
	return unix.Mkdirat(int(raw.Fd()), name, 0700)
}

func manualDarwinInventoryChildIdentity(root *managedFile, name string, child unix.Stat_t) error {
	root.mu.Lock()
	defer root.mu.Unlock()
	raw, err := root.rawLocked()
	if err != nil {
		return err
	}
	var named unix.Stat_t
	if err = unix.Fstatat(int(raw.Fd()), name, &named, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if named.Dev != child.Dev || named.Ino != child.Ino || named.Mode&unix.S_IFMT != unix.S_IFDIR {
		return ErrChanged
	}
	return nil
}

func manualDarwinInventorySample(t *testing.T, f *managedFile, name, payload string) {
	i, err := f.Stat()
	if err != nil || !i.Mode().IsRegular() || i.Mode().Perm() != 0600 || i.Size() != int64(len(payload)) {
		t.Fatalf("sample %s metadata mismatch: %v", name, err)
	}
	if err = validateLinks(f); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, len(payload))
	n, err := f.ReadAt(b, 0)
	if err != nil || n != len(b) || sha256.Sum256(b) != sha256.Sum256([]byte(payload)) {
		t.Fatalf("sample %s bytes changed: count=%d error=%v", name, n, err)
	}
	t.Logf("SAMPLE name=%q bytes=%d sha256=%x", name, i.Size(), sha256.Sum256(b))
}

func TestManualDarwinUnicodeInventory(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_DARWIN_INVENTORY") != "1" {
		t.Fatal("explicit GOSQLITE_MANUAL_DARWIN_INVENTORY=1 is required; no native fixture action")
	}
	rootPath := os.Getenv("GOSQLITE_MANUAL_DARWIN_INVENTORY_ROOT")
	if rootPath == "" || !filepath.IsAbs(rootPath) {
		t.Fatal("an explicit absolute caller-owned isolated root is required")
	}
	if runtime.GOOS != "darwin" || runtime.Version() != "go1.27.2" || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		t.Fatal("requires native Darwin amd64/arm64 and the reviewed Go 1.27.2 toolchain")
	}
	source := os.Getenv("GOSQLITE_MANUAL_DARWIN_INVENTORY_SOURCE_TREE")
	if len(source) != 40 || strings.Trim(source, "0123456789abcdef") != "" {
		t.Fatal("externally verified reviewed source tree identity is required")
	}
	if manualDarwinInventoryRetained != nil {
		t.Fatal("one invocation per process only; previous ledger is retained")
	}
	// Prove a pristine registry before adopting a ledger or installing cleanup.
	// In particular, an empty scope or a stray reserved lease must not pass.
	encryptedCleanups.mu.Lock()
	initialGroups, initialLeases := len(encryptedCleanups.groups), encryptedCleanups.leases
	encryptedCleanups.mu.Unlock()
	if initialGroups != 0 || initialLeases != 0 {
		t.Fatalf("initial registry must be empty: groups=%d leases=%d; existing ownership untouched", initialGroups, initialLeases)
	}
	l := &manualDarwinInventoryLedger{}
	manualDarwinInventoryRetained = l
	defer l.finish(t)
	if !l.known(t) {
		t.Fatal("pre-existing ownership blocks this isolated test")
	}
	namedRoot, err := manualDarwinInventoryRoot(rootPath)
	if err != nil {
		t.Fatalf("root must exist, be caller-owned 0700, and have no symlink components: %v", err)
	}
	root, err := openDirectory(rootPath)
	rootOwner := l.add("caller-root", root, nil)
	l.record(err)
	if err != nil {
		t.Fatal(err)
	}
	rootStat, rootFS, err := manualDarwinInventoryIdentity(root)
	if err != nil || rootStat.Dev != namedRoot.Dev || rootStat.Ino != namedRoot.Ino || rootStat.Uid != uint32(unix.Geteuid()) || rootStat.Mode&07777 != 0700 || rootStat.Mode&unix.S_IFMT != unix.S_IFDIR {
		t.Fatalf("retained root identity/ownership mismatch: %v", err)
	}
	if !darwinUnicodeInventoryFilesystem(rootFS.Fstypename, rootFS.Fssubtype, rootFS.Flags&unix.MNT_LOCAL != 0) {
		t.Fatal("root filesystem is not local APFS or local HFS subtype 0 through 3; no fallback")
	}
	for _, key := range []string{"kern.osrelease", "kern.osversion", "hw.machine"} {
		value, err := unix.Sysctl(key)
		if err != nil {
			t.Fatalf("native identity %s: %v", key, err)
		}
		t.Logf("NATIVE %s=%q", key, value)
	}
	build, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("build identity unavailable")
	}
	t.Logf("BUILD go=%s os=%s arch=%s path=%q source_tree_external=%s", runtime.Version(), runtime.GOOS, runtime.GOARCH, build.Path, source)
	for _, dep := range build.Deps {
		if dep.Path == "golang.org/x/crypto" || dep.Path == "golang.org/x/sys" || dep.Path == "golang.org/x/text" {
			t.Logf("BUILD_DEP path=%s version=%s sum=%s", dep.Path, dep.Version, dep.Sum)
		}
	}
	t.Logf("FILESYSTEM root_device=%d root_inode=%d type_bytes=%v subtype=%d local=%t fsid=%v", rootStat.Dev, rootStat.Ino, rootFS.Fstypename, rootFS.Fssubtype, rootFS.Flags&unix.MNT_LOCAL != 0, rootFS.Fsid)
	t.Log("ASSERT prerequisites PASS")
	childName := fmt.Sprintf("inventory-%d-%d", os.Getpid(), time.Now().UnixNano())
	l.fixture = filepath.Join(rootPath, childName)
	t.Logf("FIXTURE planned_retained_path=%q", l.fixture)
	if err = platformDirectoryCheck(root, rootPath, nil); err != nil {
		t.Fatal(err)
	}
	if err = manualDarwinInventoryMkdir(root, childName); err != nil {
		t.Fatalf("exclusive private child creation failed; preserve possible fixture: %v", err)
	}
	dir, err := openDirectory(l.fixture)
	dirOwner := l.add("fixture-directory", dir, rootOwner)
	l.record(err)
	if err != nil {
		t.Fatal(err)
	}
	childStat, childFS, err := manualDarwinInventoryIdentity(dir)
	if err != nil || childStat.Uid != uint32(unix.Geteuid()) || childStat.Mode&07777 != 0700 || childFS.Fsid != rootFS.Fsid || !darwinUnicodeInventoryFilesystem(childFS.Fstypename, childFS.Fssubtype, childFS.Flags&unix.MNT_LOCAL != 0) {
		t.Fatalf("new child's exact private filesystem/ownership qualification failed: %v", err)
	}
	if err = manualDarwinInventoryChildIdentity(root, childName, childStat); err != nil {
		t.Fatal(err)
	}
	info, err := dir.Stat()
	if err != nil {
		t.Fatal(err)
	}
	const base = "k.gse"
	path := filepath.Join(l.fixture, base)
	ns := &namespace{group: dir.owner, dir: dir, info: info, path: l.fixture, root: base}
	samples := []struct{name, payload string; member bool}{
		{base, "R01", true},
		{base + ".gse-lock", "L001", true},
		{base + ".gse-" + strings.Repeat("a", 64) + ".base", "G0001", true},
		{"notes-Ω.txt", "N000001", false},
	}
	for _, sample := range samples {
		f, err := openFileAt(dir, sample.name, os.O_RDWR|os.O_CREATE|os.O_EXCL)
		o := l.add("create:"+sample.name, f, dirOwner)
		l.record(err)
		if err != nil {
			t.Fatal(err)
		}
		n, err := f.Write([]byte(sample.payload))
		if err != nil || n != len(sample.payload) {
			t.Fatalf("exclusive sample write: n=%d error=%v", n, err)
		}
		manualDarwinInventorySample(t, f, sample.name, sample.payload)
		if !l.retire(t, o) {
			t.Fatal("sample retirement not proven")
		}
	}
	member, err := platformFamilyEntry("notes-Ω.txt", base, nil)
	if member || !errors.Is(err, ErrUnsupported) {
		t.Fatalf("nil-owner refusal: member=%t error=%v", member, err)
	}
	t.Log("ASSERT nil-owner-refusal PASS")
	enumerator, err := openFileAt(dir, ".", os.O_RDONLY)
	enumeratorOwner := l.add("retained-enumerator", enumerator, dirOwner)
	l.record(err)
	if err != nil {
		t.Fatal(err)
	}
	enumeratorStat, enumeratorFS, err := manualDarwinInventoryIdentity(enumerator)
	if err != nil || enumeratorStat.Dev != childStat.Dev || enumeratorStat.Ino != childStat.Ino || enumeratorFS.Fsid != childFS.Fsid {
		t.Fatalf("retained enumeration owner identity mismatch: %v", err)
	}
	t.Logf("ENUMERATOR device=%d inode=%d type_bytes=%v subtype=%d local=%t fsid=%v", enumeratorStat.Dev, enumeratorStat.Ino, enumeratorFS.Fstypename, enumeratorFS.Fssubtype, enumeratorFS.Flags&unix.MNT_LOCAL != 0, enumeratorFS.Fsid)
	seen := make(map[string]bool)
	sawEOF := false
	for batchNumber := 0; batchNumber < 2; batchNumber++ {
		batch, readErr := enumerator.ReadDir(5)
		if readErr != nil && readErr != io.EOF {
			t.Fatal(readErr)
		}
		for _, entry := range batch {
			matched := false
			for _, sample := range samples {
				if entry.Name() != sample.name {
					continue
				}
				member, err := platformFamilyEntry(entry.Name(), base, enumerator)
				if err != nil || member != sample.member || seen[entry.Name()] {
					t.Fatalf("actual retained enumeration owner classification: name=%q member=%t error=%v", entry.Name(), member, err)
				}
				seen[entry.Name()] = true
				matched = true
			}
			if !matched {
				t.Fatalf("unexpected entry %q; fixture retained", entry.Name())
			}
		}
		if readErr == io.EOF {
			sawEOF = true
			break
		}
	}
	if len(seen) != len(samples) || !sawEOF {
		t.Fatalf("retained enumeration coverage: got=%d want=%d", len(seen), len(samples))
	}
	if !l.retire(t, enumeratorOwner) {
		t.Fatal("enumerator retirement not proven")
	}
	t.Log("ASSERT retained-enumerator-classification PASS")
	for _, route := range []struct{name string; ns *namespace}{{"namespace", ns}, {"selfopen", nil}} {
		for _, check := range []struct{name string; bytes uint64; entries uint32; limit bool}{
			{"exact-bytes", 12, 4, false},
			{"byte-limit", 11, 4, true},
			{"entry-limit", 12, 3, true},
		} {
			total, err := inventory(context.Background(), path, check.bytes, check.entries, route.ns)
			l.record(err)
			if !l.known(t) {
				t.Fatal("inventory ownership unresolved")
			}
			if check.limit {
				if total != 0 || !errors.Is(err, encryption.ErrLimit) || errors.Is(err, ErrCleanupPending) || errors.Is(err, ErrPoisoned) {
					t.Fatalf("%s-%s: total=%d error=%v", route.name, check.name, total, err)
				}
			} else if err != nil || total != 12 {
				t.Fatalf("%s-%s: total=%d want=12 error=%v", route.name, check.name, total, err)
			}
			t.Logf("ASSERT %s-%s PASS total=%d", route.name, check.name, total)
		}
	}
	for _, sample := range samples {
		f, _, err := ns.checked(filepath.Join(l.fixture, sample.name), os.O_RDONLY)
		o := l.add("verify:"+sample.name, f, dirOwner)
		l.record(err)
		if err != nil {
			t.Fatal(err)
		}
		manualDarwinInventorySample(t, f, sample.name, sample.payload)
		if !l.retire(t, o) {
			t.Fatal("verification owner retirement not proven")
		}
	}
	if err = platformDirectoryCheck(root, rootPath, nil); err != nil {
		t.Fatal(err)
	}
	t.Log("ASSERT samples-unchanged PASS")
}
