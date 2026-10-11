package walindex

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

func TestReadOnlyIndexOpenPolicyMemory(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows", "freebsd", ""} {
		for _, readOnly := range []bool{false, true} {
			err := readonlyIndexOpenProfile(readOnly, goos)
			if readOnly && goos != "linux" {
				if !errors.Is(err, ErrUnsupported) {
					t.Fatalf("readOnly=%v GOOS=%q: %v", readOnly, goos, err)
				}
			} else if err != nil {
				t.Fatalf("readOnly=%v GOOS=%q: %v", readOnly, goos, err)
			}
		}
	}
	if got := readonlyIndexOpenFlags(true); got != os.O_RDONLY {
		t.Fatalf("readonly flags=%d", got)
	}
	if got := readonlyIndexOpenFlags(false); got != os.O_RDWR|os.O_CREATE {
		t.Fatalf("writer flags=%d", got)
	}
	if err := readonlyIndexWriteAccess(false); err != nil {
		t.Fatalf("writer capability: %v", err)
	}
	if err := nativeError(vfs.ErrReadOnly); !errors.Is(err, ErrReadOnly) || !errors.Is(err, vfs.ErrReadOnly) {
		t.Fatalf("readonly error identity: %v", err)
	}
}

func TestReadOnlyIndexCapabilityMemory(t *testing.T) {
	operations := []struct {
		name string
		call func(*Index) error
	}{
		{"Publish", func(x *Index) error { return x.Publish(Header{PageSize: 4096}) }},
		{"InitCheckpoint", func(x *Index) error { return x.InitCheckpoint(1) }},
		{"setCheckpoint", func(x *Index) error { return x.setCheckpoint(96, 1) }},
		{"SetBackfill", func(x *Index) error { return x.SetBackfill(1) }},
		{"SetAttempted", func(x *Index) error { return x.SetAttempted(1) }},
		{"SetReadMark", func(x *Index) error { return x.SetReadMark(1, 1) }},
		{"SetReadMark invalid", func(x *Index) error { return x.SetReadMark(-1, 1) }},
		{"Cleanup empty", func(x *Index) error { return x.Cleanup(0) }},
		{"Cleanup prefix", func(x *Index) error { return x.Cleanup(1) }},
		{"Append", func(x *Index) error { return x.Append(1, 1) }},
		{"exclusive slot", func(x *Index) error { return x.Lock(WriteLock, 1, true) }},
		{"exclusive slots", func(x *Index) error { return x.Lock(0, 8, true) }},
		{"extension", func(x *Index) error {
			x.mu.Lock()
			defer x.mu.Unlock()
			_, err := x.blockLocked(0, true)
			return err
		}},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			for _, held := range [][8]uint8{{}, {2, 2, 2, 2, 2, 2, 2, 2}} {
				// No file, native mapping, or I/O callback is constructed. Even
				// fabricated writer ownership cannot override the capability.
				x := &Index{readOnly: true, maxFrames: DefaultMaxFrames, held: held}
				if err := operation.call(x); !errors.Is(err, ErrReadOnly) {
					t.Fatalf("got %v; want ErrReadOnly before any file access", err)
				}
				if x.file != nil || x.maps != nil || x.mappedEnd != 0 || x.held != held || x.closed {
					t.Fatal("denied operation changed descriptor-free owner state")
				}
				x.closed = true
				if err := operation.call(x); !errors.Is(err, ErrClosed) {
					t.Fatalf("closed owner: %v", err)
				}
			}
		})
	}
}

func TestReadOnlyIndexReadReleasePolicyMemory(t *testing.T) {
	x := &Index{readOnly: true, maxFrames: DefaultMaxFrames}
	// A nil owner prevents all native acquisition. These operations reach
	// ordinary ownership validation rather than a read-only capability denial.
	for _, operation := range []struct {
		name string
		call func() error
	}{
		{"shared slot", func() error { return x.Lock(ReadLock0, 1, false) }},
		{"release", func() error { return x.Unlock(ReadLock0, 1) }},
		{"header", func() error { _, err := x.ReadHeader(); return err }},
		{"backfill", func() error { _, err := x.Backfill(); return err }},
		{"attempted", func() error { _, err := x.Attempted(); return err }},
		{"readmark", func() error { _, err := x.ReadMark(1); return err }},
		{"page", func() error { _, err := x.Page(1); return err }},
		{"lookup", func() error { _, err := x.Lookup(1, 1, 1); return err }},
		{"identity", func() error { return x.ValidateIdentity() }},
	} {
		if err := operation.call(); !errors.Is(err, ErrClosed) {
			t.Fatalf("%s nil owner: %v", operation.name, err)
		}
	}
	if err := x.Close(); err != nil || !x.RetirementComplete() {
		t.Fatalf("nil owner close: %v", err)
	}
}

// Source-only guards couple the pure policies to the production effects. They
// do not open a storage file, execute native calls, or parse/execute Go code.
func TestReadOnlyIndexSourceGuard(t *testing.T) {
	reader := readonlyIndexSourceSection(t, "index.go", "func OpenReadOnly(")
	if !strings.Contains(reader, "openIndex(path, DefaultMaxFrames, true)") {
		t.Fatal("readonly opener lost explicit capability")
	}
	writer := readonlyIndexSourceSection(t, "index.go", "func OpenWithLimit(")
	if !strings.Contains(writer, "openIndex(path, maxFrames, false)") {
		t.Fatal("writer opener changed access mode")
	}
	opener := readonlyIndexSourceSection(t, "index.go", "func openIndex(")
	readonlyIndexRequireOrder(t, opener,
		"readonlyIndexOpenProfile(readOnly, runtime.GOOS)",
		"filepath.Abs(path)",
		"vfs.ResolvePath(filepath.Dir(p))",
		"openFile(p, readOnly)",
		"nativeError(f.Close())",
		"readOnly: readOnly",
		"f.AttachDMS()")
	native := readonlyIndexSourceSection(t, "native_file.go", "func openFile(")
	if !strings.Contains(native, "vfs.Open(path, vfs.OpenOptions{Role: vfs.SHM, Flags: readonlyIndexOpenFlags(readOnly)})") {
		t.Fatal("SHM opener bypasses managed owner or access policy")
	}
	guard := readonlyIndexSourceSection(t, "index.go", "func (x *Index) writerLocked(")
	readonlyIndexRequireOrder(t, guard, "readonlyIndexWriteAccess(x.readOnly)", "x.held[WriteLock] != 2")
	for _, name := range []string{"Publish", "InitCheckpoint", "Cleanup", "Append"} {
		body := readonlyIndexSourceSection(t, "index.go", "func (x *Index) "+name+"(")
		readonlyIndexRequireOrder(t, body, "x.writerLocked()", "x.blockLocked(")
	}
	for _, name := range []string{"setCheckpoint", "SetReadMark"} {
		body := readonlyIndexSourceSection(t, "index.go", "func (x *Index) "+name+"(")
		readonlyIndexRequireOrder(t, body, "readonlyIndexWriteAccess(x.readOnly)", "x.blockLocked(", "store32(")
	}
	backfill := readonlyIndexSourceSection(t, "index.go", "func (x *Index) SetBackfill(")
	attempted := readonlyIndexSourceSection(t, "index.go", "func (x *Index) SetAttempted(")
	if !strings.Contains(backfill, "x.setCheckpoint(96, value)") || !strings.Contains(attempted, "x.setCheckpoint(128, value)") {
		t.Fatal("checkpoint wrapper bypasses guarded setter")
	}
	block := readonlyIndexSourceSection(t, "index.go", "func (x *Index) blockLocked(")
	readonlyIndexRequireOrder(t, block,
		"if extend {", "readonlyIndexWriteAccess(x.readOnly)", "x.statLocked()",
		"x.maps[base]", "x.file.WriteAt(", "x.file.MapShared(")
	lock := readonlyIndexSourceSection(t, "index.go", "func (x *Index) Lock(")
	readonlyIndexRequireOrder(t, lock,
		"if exclusive {", "readonlyIndexWriteAccess(x.readOnly)", "x.statLocked()", "x.file.LockSHM(")
	for _, name := range []string{"ReadHeader", "Backfill", "Attempted", "ReadMark", "Page", "Lookup", "Unlock", "Close"} {
		body := readonlyIndexSourceSection(t, "index.go", "func (x *Index) "+name+"(")
		if strings.Contains(body, "readonlyIndexWriteAccess") || strings.Contains(body, "x.writerLocked()") {
			t.Fatalf("%s incorrectly requires writable capability", name)
		}
	}
}

func readonlyIndexSourceSection(t *testing.T, path, signature string) string {
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

func readonlyIndexRequireOrder(t *testing.T, source string, tokens ...string) {
	t.Helper()
	for _, token := range tokens {
		at := strings.Index(source, token)
		if at < 0 {
			t.Fatalf("source sequence missing %q", token)
		}
		source = source[at+len(token):]
	}
}
