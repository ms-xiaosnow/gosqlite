package vfs

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
)

// These controls only read and parse source. Win32 calls below are inert string
// literals, never native calls, generated compilable fixtures, or I/O callbacks.
// Full-function AST contracts bind the actual arguments, nesting, error returns
// and statement ordering. A decorative helper call or constant cannot satisfy
// them. Positions, comments and parser object metadata have no authority.
//
// The byte pins are separate change detectors. Counterexamples invoke only the
// structural checker, so they cannot pass merely because a digest changed.
// These are bounded source regression controls, not native/runtime evidence or
// a general proof of Windows semantics. Changes require fresh source review.
type windowsReadOnlySourceContract struct {
	group    string
	path     string
	name     string
	function string
}

var windowsReadOnlySourceContracts = []windowsReadOnlySourceContract{
	{"dms", "readonly_shm_windows_policy.go", "readonlyWindowsDMSDecision", `func readonlyWindowsDMSDecision(writable bool, observation windowsDMSObservation) (initialize bool, err error) {
	switch observation {
	case windowsDMSLocalAttached, windowsDMSProbeContended:
		return false, nil
	case windowsDMSProbeAcquired:
		if writable {
			return true, nil
		}
		return false, ErrUnsupported
	default:
		return false, ErrUnsupported
	}
}`},
	{"dms", "native_windows.go", "nativeAttachDMS", `func nativeAttachDMS(f *File) error {
	winRegistry.Lock()
	defer winRegistry.Unlock()
	node := f.native.node
	if node == nil || node.role != SHM {
		return ErrLock
	}
	observation := windowsDMSLocalAttached
	if node.attached == 0 {
		err := winAcquire(f, winDMS, true)
		if err == nil {
			observation = windowsDMSProbeAcquired
		} else if errors.Is(err, ErrPoisoned) || !errors.Is(err, ErrBusy) {
			return err
		} else {
			observation = windowsDMSProbeContended
		}
	}
	initialize, err := readonlyWindowsDMSDecision(f.writable, observation)
	if err != nil {
		if observation == windowsDMSProbeAcquired {
			// Refusal still owns the successful exclusive probe. A failed
			// release retains its exact ledger entry and poison cause.
			return errors.Join(err, winRelease(f, winDMS))
		}
		return err
	}
	if initialize {
		// Only a writable acquired probe can initialize. There is no
		// fallback header erasure after an arbitrary truncation failure.
		if err = f.raw.Truncate(0); err != nil {
			return winPoison(f, err)
		}
		if err = winRelease(f, winDMS); err != nil {
			return err
		}
	}
	if err := winAcquire(f, winDMS, false); err != nil {
		return err
	}
	node.attached++
	f.native.attached = true
	return nil
}`},
	{"dms", "native_windows.go", "winAcquire", `func winAcquire(f *File, which int, exclusive bool) error {
	mode := uint8(1)
	if exclusive {
		mode = 2
	}
	if f.native.held[which] != 0 {
		if f.native.held[which] == mode {
			return nil
		}
		return ErrLock
	}
	off, n := winInterval(which)
	o := windows.Overlapped{Offset: uint32(off), OffsetHigh: uint32(uint64(off) >> 32)}
	flags := uint32(windows.LOCKFILE_FAIL_IMMEDIATELY)
	if exclusive {
		flags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}
	err := windows.LockFileEx(windows.Handle(f.raw.Fd()), flags, 0, uint32(n), uint32(uint64(n)>>32), &o)
	if err != nil {
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return ErrBusy
		}
		if errors.Is(err, windows.ERROR_IO_PENDING) {
			return winPoison(f, fmt.Errorf("unexpected asynchronous LockFileEx result: %w", err))
		}
		return fmt.Errorf("LockFileEx: %w", err)
	}
	f.native.held[which] = mode
	return nil
}`},
	{"dms", "native_windows.go", "winRelease", `func winRelease(f *File, which int) error {
	if f.native.held[which] == 0 {
		return nil
	}
	off, n := winInterval(which)
	o := windows.Overlapped{Offset: uint32(off), OffsetHigh: uint32(uint64(off) >> 32)}
	if err := windows.UnlockFileEx(windows.Handle(f.raw.Fd()), 0, uint32(n), uint32(uint64(n)>>32), &o); err != nil {
		return winPoison(f, fmt.Errorf("UnlockFileEx: %w", err))
	}
	f.native.held[which] = 0
	return nil
}`},
	{"dms", "native_windows.go", "winPoison", `func winPoison(f *File, err error) error {
	f.native.poisoned = true
	return errors.Join(ErrPoisoned, err)
}`},
	{"dms", "native_windows.go", "nativeCheck", `func nativeCheck(f *File) error {
	if err := winMetadataOwners.check(); err != nil {
		return winPoison(f, err)
	}
	if f.native.poisoned || f.native.closeUnknown || f.native.rawClosed {
		return ErrPoisoned
	}
	return nil
}`},
	{"dms", "locks.go", "AttachDMS", `func (f *File) AttachDMS() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return err
	}
	if f.role != SHM {
		return ErrLock
	}
	if f.dms {
		return nil
	}
	if err := nativeAttachDMS(f); err != nil {
		return f.lockResultLocked(err)
	}
	f.dms = true
	return nil
}`},
	{"dms", "locks.go", "lockResultLocked", `func (f *File) lockResultLocked(err error) error {
	if errors.Is(err, ErrPoisoned) {
		f.closing = true
	}
	return err
}`},
	{"mapping", "native_windows.go", "nativeMap", `func nativeMap(f *File, off int64, n int) (nativeMapping, []byte, error) {
	var empty nativeMapping
	g := nativeMapGranularity()
	if off < 0 || n <= 0 || g == 0 || off%g != 0 {
		return empty, nil, ErrUnsupported
	}
	protection := uint32(windows.PAGE_READONLY)
	access := uint32(windows.FILE_MAP_READ)
	if f.writable {
		protection = windows.PAGE_READWRITE
		access |= windows.FILE_MAP_WRITE
	}
	// Zero maximum size means current backing size, so CreateFileMapping
	// cannot extend storage merely to satisfy allocation-granularity rounding.
	h, err := windows.CreateFileMapping(windows.Handle(f.raw.Fd()), nil, protection, 0, 0, nil)
	if err != nil {
		return empty, nil, err
	}
	m := nativeMapping{mappingRetirement: mappingRetirement{handle: uintptr(h), length: n}, owner: f}
	base, err := windows.MapViewOfFile(h, access, uint32(uint64(off)>>32), uint32(off), uintptr(n))
	if err != nil {
		if closeErr := nativeUnmap(&m); closeErr != nil {
			f.native.retained = append(f.native.retained, m)
			return empty, nil, errors.Join(err, closeErr)
		}
		return empty, nil, err
	}
	m.base = base
	return m, unsafe.Slice((*byte)(unsafe.Pointer(base)), n), nil
}`},
	{"mapping", "native_windows.go", "nativeMapGranularity", `func nativeMapGranularity() int64 {
	winMappingSystem.Do(func() {
		proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetSystemInfo")
		if err := proc.Find(); err != nil {
			return
		}
		var info winSystemInfo
		proc.Call(uintptr(unsafe.Pointer(&info))) // Return and last-error are undefined.
		g, p := info.allocationGranularity, info.pageSize
		if g == 0 || p == 0 || g&(g-1) != 0 || p&(p-1) != 0 || g < p || g%p != 0 {
			return
		}
		winMappingSystem.granularity = int64(g)
	})
	return winMappingSystem.granularity
}`},
	{"mapping", "native_windows.go", "nativeUnmap", `func nativeUnmap(m *nativeMapping) error {
	// Views retain a mapping object reference independently of its HANDLE.
	// Closing the object handle first keeps Bytes live if either stage fails;
	// DMS and the file handle remain held until the view is actually unmapped.
	// Both published views and rejected MapViewOfFile acquisition use this
	// gate. An indeterminate object close is never retried with a numeric
	// HANDLE that may now designate some unrelated object.
	if err := m.mappingRetirement.retire(winCloseMappingHandle, windows.UnmapViewOfFile); err != nil {
		return winPoison(m.owner, err)
	}
	return nil
}`},
	{"mapping", "native_windows.go", "winCloseMappingHandle", `func winCloseMappingHandle(handle uintptr) error {
	return windows.CloseHandle(windows.Handle(handle))
}`},
	{"mapping", "native_windows.go", "nativeClose", `func nativeClose(f *File) error {
	if f.native.closeUnknown {
		return ErrPoisoned // os.File.Close cannot safely be repeated after an error.
	}
	for len(f.native.retained) != 0 {
		i := len(f.native.retained) - 1
		if err := nativeUnmap(&f.native.retained[i]); err != nil {
			return winPoison(f, err)
		}
		f.native.retained = f.native.retained[:i]
	}
	winRegistry.Lock()
	defer winRegistry.Unlock()
	if !f.native.rawClosed {
		// Exact one-byte slot acquisitions, then DB protection, then DMS.
		for i := winIntervals - 1; i >= winSlot0; i-- {
			if err := winRelease(f, i); err != nil {
				return err
			}
		}
		for _, i := range []int{winShared, winReserved, winProbe, winPending, winDMS} {
			if err := winRelease(f, i); err != nil {
				return err
			}
		}
		if f.native.attached {
			f.native.node.attached--
			f.native.attached = false
		}
		if err := f.raw.Close(); err != nil {
			f.native.closeUnknown = true
			return winPoison(f, err)
		}
		f.native.rawClosed = true
	}
	if f.native.parent != nil {
		if err := winCloseMetadata(f.native.parent); err != nil {
			return winPoison(f, err)
		}
		f.native.parent = nil
	}
	if node := f.native.node; node != nil {
		node.leases--
		if node.leases == 0 {
			delete(winRegistry.nodes, f.native.id)
		}
		f.native.node = nil
	}
	return nil
}`},
	{"mapping", "mapping.go", "mappingSpan", `func mappingSpan(off int64, n int, granularity int64) (base int64, delta, length int, err error) {
	if off < 0 || n <= 0 || granularity <= 0 || granularity&(granularity-1) != 0 || off > math.MaxInt64-int64(n) {
		return 0, 0, 0, ErrUnsupported
	}
	base = off / granularity * granularity
	d := off - base
	if d > int64(math.MaxInt)-int64(n) {
		return 0, 0, 0, ErrUnsupported
	}
	return base, int(d), int(d) + n, nil
}`},
	{"mapping", "mapping.go", "MapShared", `func (f *File) MapShared(off int64, n int) (*Mapping, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return nil, err
	}
	if f.role != SHM || !f.dms {
		return nil, ErrLock
	}
	base, delta, length, err := mappingSpan(off, n, nativeMapGranularity())
	if err != nil {
		return nil, err
	}
	info, err := f.raw.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() < off+int64(n) {
		return nil, ErrIdentity
	}
	mapped, bytes, err := nativeMap(f, base, length)
	if err != nil {
		return nil, err
	}
	m := &Mapping{Bytes: bytes[delta : delta+n], owner: f, native: mapped}
	f.maps[m] = struct{}{}
	return m, nil
}`},
	{"mapping", "mapping.go", "closeLocked", `func (m *Mapping) closeLocked() error { return m.retireLocked(nativeUnmap) }`},
	{"mapping", "mapping.go", "retireLocked", `func (m *Mapping) retireLocked(unmap func(*nativeMapping) error) error {
	if m.Bytes == nil {
		return nil
	}
	if err := unmap(&m.native); err != nil {
		return err
	}
	m.Bytes = nil
	delete(m.owner.maps, m)
	return nil
}`},
	{"mapping", "mapping.go", "Close", `func (m *Mapping) Close() error {
	m.owner.mu.Lock()
	defer m.owner.mu.Unlock()
	return m.closeLocked()
}`},
	{"mapping", "mapping_retirement.go", "retire", `func (m *mappingRetirement) retire(closeHandle, unmapView func(uintptr) error) error {
	if m.closeUnknown != nil {
		return m.closeUnknown
	}
	// Closing the object before the view preserves the exposed Bytes if close
	// fails. A view independently retains the object after a successful close.
	if m.handle != 0 {
		if err := closeHandle(m.handle); err != nil {
			m.closeUnknown = errors.Join(ErrPoisoned, err)
			return m.closeUnknown
		}
		m.handle = 0
	}
	if m.base != 0 {
		if err := unmapView(m.base); err != nil {
			return err // The represented view can be retried; its handle cannot.
		}
		m.base = 0
	}
	m.length = 0
	return nil
}`},
	{"mapping", "file.go", "Close", `func (f *File) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.raw == nil {
		return nil
	}
	f.closing = true
	err := f.closeResourcesLocked(nativeUnmap, func() error { return nativeClose(f) })
	if len(f.maps) != 0 || errors.Is(err, ErrPoisoned) {
		// Nested constructors may return nil plus this error. Carry the
		// managed retry obligation to their parent instead of losing it.
		return retainedFileCleanup(f, err)
	}
	f.raw = nil
	owners.Lock()
	delete(owners.files, f)
	owners.Unlock()
	return err
}`},
	{"mapping", "file.go", "closeResourcesLocked", `func (f *File) closeResourcesLocked(unmap func(*nativeMapping) error, closeFile func() error) error {
	var errs []error
	for m := range f.maps {
		if err := m.retireLocked(unmap); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return errors.Join(append([]error{ErrPoisoned}, errs...)...)
	}
	return closeFile()
}`},
}

// Pins cover every source file that supplies a structural contract. They do not
// update or weaken any pre-existing source guard or manual/oracle exception.
var windowsReadOnlySourcePins = map[string]string{
	"file.go":                        "5ae510838d69e48ae5b417143987e1c50bb7f413f237d8a92f06660c71e87aae",
	"locks.go":                       "0a94a3019553f5e3a6d2c2f1a6dedb3c076e25039d9dd83c13fd4d8736aad80c",
	"mapping.go":                     "f4e243e005f634ddec968ce9885a3bca4db6e998b072a128d0f44116f107f437",
	"mapping_retirement.go":          "9791a8be8ceb3c2c7fff85aba80b31c9f1334b96ea4070dda45b5c44765d464d",
	"native_windows.go":              "0713ca03e4d83a251c433d094a72acf639db8c1456c854859d6fe6c1f54f45c7",
	"readonly_shm_windows_policy.go": "a479f0f546a91348cfdd6327e9e6aad6403a088a83611fc43eacae8bca4f2dac",
}

func windowsReadOnlySourceInputs(t *testing.T) map[string]string {
	t.Helper()
	inputs := make(map[string]string)
	for path := range windowsReadOnlySourcePins {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		inputs[path] = string(raw)
	}
	return inputs
}

func windowsReadOnlySourceFunction(raw, path, name string) (*ast.FuncDecl, *token.FileSet, error) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, raw, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, err
	}
	var found *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name {
			continue
		}
		if found != nil {
			return nil, nil, fmt.Errorf("%s: duplicate function %s", path, name)
		}
		found = fn
	}
	if found == nil {
		return nil, nil, fmt.Errorf("%s: missing function %s", path, name)
	}
	return found, set, nil
}

func windowsReadOnlySourceShape(fn *ast.FuncDecl) (string, error) {
	var out bytes.Buffer
	position := reflect.TypeOf(token.Pos(0))
	err := ast.Fprint(&out, token.NewFileSet(), fn, func(name string, value reflect.Value) bool {
		return value.Type() != position && name != "Obj" && name != "Doc" && name != "Comment"
	})
	return out.String(), err
}

func windowsReadOnlySourceStructure(inputs map[string]string, group string) error {
	for _, contract := range windowsReadOnlySourceContracts {
		if group != "" && contract.group != group {
			continue
		}
		raw, ok := inputs[contract.path]
		if !ok {
			return fmt.Errorf("missing source %s", contract.path)
		}
		actual, _, err := windowsReadOnlySourceFunction(raw, contract.path, contract.name)
		if err != nil {
			return err
		}
		expected, _, err := windowsReadOnlySourceFunction("package vfs\n"+contract.function, "contract.go", contract.name)
		if err != nil {
			return fmt.Errorf("invalid contract %s: %w", contract.name, err)
		}
		a, err := windowsReadOnlySourceShape(actual)
		if err != nil {
			return err
		}
		b, err := windowsReadOnlySourceShape(expected)
		if err != nil {
			return err
		}
		if a != b {
			return fmt.Errorf("%s: %s control flow or arguments changed", contract.path, contract.name)
		}
	}
	return nil
}

func windowsReadOnlyCheckSourcePins(t *testing.T, inputs map[string]string) {
	t.Helper()
	for path, want := range windowsReadOnlySourcePins {
		if got := fmt.Sprintf("%x", sha256.Sum256([]byte(inputs[path]))); got != want {
			t.Errorf("%s source changed: %s; review the complete boundary before changing its new pin", path, got)
		}
	}
}

func TestWindowsReadOnlyDMSProductionSourceBoundary(t *testing.T) {
	inputs := windowsReadOnlySourceInputs(t)
	windowsReadOnlyCheckSourcePins(t, inputs)
	if err := windowsReadOnlySourceStructure(inputs, "dms"); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsReadOnlyMappingProductionSourceBoundary(t *testing.T) {
	inputs := windowsReadOnlySourceInputs(t)
	windowsReadOnlyCheckSourcePins(t, inputs)
	if err := windowsReadOnlySourceStructure(inputs, "mapping"); err != nil {
		t.Fatal(err)
	}
}

func windowsReadOnlyMutateSource(t *testing.T, raw, path, name, old, replacement string) string {
	t.Helper()
	fn, set, err := windowsReadOnlySourceFunction(raw, path, name)
	if err != nil {
		t.Fatal(err)
	}
	begin, end := set.Position(fn.Pos()).Offset, set.Position(fn.End()).Offset
	body := raw[begin:end]
	if strings.Count(body, old) != 1 {
		t.Fatalf("%s: mutation anchor must occur exactly once in %s", path, name)
	}
	return raw[:begin] + strings.Replace(body, old, replacement, 1) + raw[end:]
}

func TestWindowsReadOnlySourceMutationCounterexamples(t *testing.T) {
	inputs := windowsReadOnlySourceInputs(t)
	// Establish the real baseline through the same no-pin checker first.
	if err := windowsReadOnlySourceStructure(inputs, ""); err != nil {
		t.Fatal(err)
	}
	mutants := []struct {
		name        string
		path        string
		function    string
		old         string
		replacement string
	}{
		{"remove-helper", "native_windows.go", "nativeAttachDMS", "initialize, err := readonlyWindowsDMSDecision(f.writable, observation)", "initialize, err := f.writable, error(nil)"},
		{"force-writable-helper", "native_windows.go", "nativeAttachDMS", "readonlyWindowsDMSDecision(f.writable, observation)", "readonlyWindowsDMSDecision(true, observation)"},
		{"feed-invented-observation", "native_windows.go", "nativeAttachDMS", "readonlyWindowsDMSDecision(f.writable, observation)", "readonlyWindowsDMSDecision(f.writable, windowsDMSLocalAttached)"},
		{"ignore-helper-refusal", "native_windows.go", "nativeAttachDMS", "initialize, err := readonlyWindowsDMSDecision(f.writable, observation)\n\tif err != nil {", "initialize, err := readonlyWindowsDMSDecision(f.writable, observation)\n\tif false {"},
		{"invert-initialize", "native_windows.go", "nativeAttachDMS", "if initialize {", "if !initialize {"},
		{"arbitrary-error-as-contention", "native_windows.go", "nativeAttachDMS", "errors.Is(err, ErrPoisoned) || !errors.Is(err, ErrBusy)", "false"},
		{"busy-over-poison", "native_windows.go", "nativeAttachDMS", "errors.Is(err, ErrPoisoned) || !errors.Is(err, ErrBusy)", "!errors.Is(err, ErrBusy)"},
		{"discard-original-probe-error", "native_windows.go", "nativeAttachDMS", "} else if errors.Is(err, ErrPoisoned) || !errors.Is(err, ErrBusy) {\n\t\t\treturn err", "} else if errors.Is(err, ErrPoisoned) || !errors.Is(err, ErrBusy) {\n\t\t\treturn ErrBusy"},
		{"omit-cold-release", "native_windows.go", "nativeAttachDMS", "return errors.Join(err, winRelease(f, winDMS))", "return err"},
		{"discard-cold-release-error", "native_windows.go", "nativeAttachDMS", "return errors.Join(err, winRelease(f, winDMS))", "winRelease(f, winDMS)\n\t\t\treturn err"},
		{"release-cold-twice", "native_windows.go", "nativeAttachDMS", "return errors.Join(err, winRelease(f, winDMS))", "winRelease(f, winDMS)\n\t\t\treturn errors.Join(err, winRelease(f, winDMS))"},
		{"truncate-before-refusal", "native_windows.go", "nativeAttachDMS", "initialize, err := readonlyWindowsDMSDecision(f.writable, observation)", "f.raw.Truncate(0)\n\tinitialize, err := readonlyWindowsDMSDecision(f.writable, observation)"},
		{"discard-init-release-error", "native_windows.go", "nativeAttachDMS", "if err = winRelease(f, winDMS); err != nil {\n\t\t\treturn err\n\t\t}", "winRelease(f, winDMS)"},
		{"publish-before-shared-success", "native_windows.go", "nativeAttachDMS", "if err := winAcquire(f, winDMS, false); err != nil {", "node.attached++\n\tf.native.attached = true\n\tif err := winAcquire(f, winDMS, false); err != nil {"},
		{"skip-local-owner-shared", "native_windows.go", "nativeAttachDMS", "if err := winAcquire(f, winDMS, false); err != nil {\n\t\treturn err\n\t}", "if observation != windowsDMSLocalAttached {\n\t\tif err := winAcquire(f, winDMS, false); err != nil {\n\t\t\treturn err\n\t\t}\n\t}"},
		{"ignore-shared-error", "native_windows.go", "nativeAttachDMS", "if err := winAcquire(f, winDMS, false); err != nil {\n\t\treturn err\n\t}", "winAcquire(f, winDMS, false)"},
		{"omit-registry-lock", "native_windows.go", "nativeAttachDMS", "winRegistry.Lock()", ""},
		{"readonly-cold-allowed", "readonly_shm_windows_policy.go", "readonlyWindowsDMSDecision", "if writable {", "if !writable {"},
		{"unknown-observation-allowed", "readonly_shm_windows_policy.go", "readonlyWindowsDMSDecision", "default:\n\t\treturn false, ErrUnsupported", "default:\n\t\treturn false, nil"},
		{"native-protection-bypass", "native_windows.go", "nativeMap", "nil, protection, 0, 0, nil", "nil, windows.PAGE_READWRITE, 0, 0, nil"},
		{"native-access-bypass", "native_windows.go", "nativeMap", "windows.MapViewOfFile(h, access,", "windows.MapViewOfFile(h, windows.FILE_MAP_READ|windows.FILE_MAP_WRITE,"},
		{"writable-default-protection", "native_windows.go", "nativeMap", "uint32(windows.PAGE_READONLY)", "uint32(windows.PAGE_READWRITE)"},
		{"writable-default-access", "native_windows.go", "nativeMap", "uint32(windows.FILE_MAP_READ)", "uint32(windows.FILE_MAP_READ|windows.FILE_MAP_WRITE)"},
		{"invert-mapping-capability", "native_windows.go", "nativeMap", "if f.writable {", "if !f.writable {"},
		{"grow-maximum-mapping-size", "native_windows.go", "nativeMap", "nil, protection, 0, 0, nil", "nil, protection, 0, uint32(n), nil"},
		{"drop-failed-view-owner", "native_windows.go", "nativeMap", "f.native.retained = append(f.native.retained, m)", ""},
		{"drop-map-owner", "native_windows.go", "nativeMap", "length: n}, owner: f", "length: n}, owner: nil"},
		{"forget-original-view-base", "native_windows.go", "nativeMap", "m.base = base", "m.base = base+uintptr(n)"},
		{"skip-existing-extent-check", "mapping.go", "MapShared", "if info.Size() < off+int64(n) {", "if false {"},
		{"drop-managed-registration", "mapping.go", "MapShared", "f.maps[m] = struct{}{}", ""},
		{"retry-unknown-object-close", "mapping_retirement.go", "retire", "if m.closeUnknown != nil {\n\t\treturn m.closeUnknown\n\t}", ""},
		{"retry-unknown-file-close", "native_windows.go", "nativeClose", "if f.native.closeUnknown {\n\t\treturn ErrPoisoned // os.File.Close cannot safely be repeated after an error.\n\t}", ""},
		{"clear-failed-release-ledger", "native_windows.go", "winRelease", "return winPoison(f, fmt.Errorf(\"UnlockFileEx: %w\", err))", "f.native.held[which] = 0\n\t\treturn winPoison(f, fmt.Errorf(\"UnlockFileEx: %w\", err))"},
		{"hide-native-unmap-poison", "native_windows.go", "nativeUnmap", "return winPoison(m.owner, err)", "return err"},
		{"publish-managed-before-native", "locks.go", "AttachDMS", "if err := nativeAttachDMS(f); err != nil {", "f.dms = true\n\tif err := nativeAttachDMS(f); err != nil {"},
		{"retire-file-before-views", "file.go", "closeResourcesLocked", "var errs []error", "closeFile()\n\tvar errs []error"},
	}
	for _, mutant := range mutants {
		t.Run(mutant.name, func(t *testing.T) {
			changed := make(map[string]string, len(inputs))
			for path, raw := range inputs {
				changed[path] = raw
			}
			changed[mutant.path] = windowsReadOnlyMutateSource(t, changed[mutant.path], mutant.path, mutant.function, mutant.old, mutant.replacement)
			// Require a valid inert AST: parser failure is not a killed mutant.
			if _, _, err := windowsReadOnlySourceFunction(changed[mutant.path], mutant.path, mutant.function); err != nil {
				t.Fatalf("malformed counterexample: %v", err)
			}
			if err := windowsReadOnlySourceStructure(changed, ""); err == nil {
				t.Fatal("structural checker accepted the counterexample without consulting any digest")
			}
		})
	}
}
