package vfs

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
)

// These finite source contracts inspect the real production functions, not a
// substitute native model. Complete AST shapes freeze the capability argument,
// refusal dominance, exact lock calls, poisoning and actual Mmap arguments.
// Formatting/comments/positions are ignored. This is deliberately narrower
// than a general call-graph proof; any intended change requires source review.
// Source text is the only file input. No storage descriptor, native function,
// subprocess, environment gate, test fixture or package init is introduced.
func darwinReadonlyASTShape(node ast.Node) (string, error) {
	var out bytes.Buffer
	position := reflect.TypeOf(token.Pos(0))
	err := ast.Fprint(&out, nil, node, func(name string, value reflect.Value) bool {
		return value.Type() != position && name != "Obj" && name != "Scope" && name != "Unresolved" && name != "Doc" && name != "Comment" && name != "Comments"
	})
	return out.String(), err
}

func darwinReadonlyParse(path, source string) (*ast.File, error) {
	return parser.ParseFile(token.NewFileSet(), path, source, parser.SkipObjectResolution)
}

func darwinReadonlyFunction(file *ast.File, name string) (*ast.FuncDecl, error) {
	var found *ast.FuncDecl
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if ok && fn.Name.Name == name {
			if found != nil {
				return nil, fmt.Errorf("duplicate function %s", name)
			}
			found = fn
		}
	}
	if found == nil || found.Body == nil {
		return nil, fmt.Errorf("missing body for %s", name)
	}
	return found, nil
}

func darwinReadonlyCheckFunction(source, name, contract string) error {
	actualFile, err := darwinReadonlyParse("actual.go", source)
	if err != nil {
		return err
	}
	expectedFile, err := darwinReadonlyParse("contract.go", "package vfs\n"+contract)
	if err != nil {
		return err
	}
	actual, err := darwinReadonlyFunction(actualFile, name)
	if err != nil {
		return err
	}
	expected, err := darwinReadonlyFunction(expectedFile, name)
	if err != nil {
		return err
	}
	actualShape, err := darwinReadonlyASTShape(actual)
	if err != nil {
		return err
	}
	expectedShape, err := darwinReadonlyASTShape(expected)
	if err != nil {
		return err
	}
	if actualShape != expectedShape {
		return fmt.Errorf("%s: production AST no longer matches the reviewed control-flow contract", name)
	}
	return nil
}

func darwinReadonlyCheckPolicy(source string) error {
	actual, err := darwinReadonlyParse("readonly_darwin_dms_policy.go", source)
	if err != nil {
		return err
	}
	expected, err := darwinReadonlyParse("contract.go", darwinReadonlyPolicyContract)
	if err != nil {
		return err
	}
	actualShape, err := darwinReadonlyASTShape(actual)
	if err != nil {
		return err
	}
	expectedShape, err := darwinReadonlyASTShape(expected)
	if err != nil {
		return err
	}
	if actualShape != expectedShape {
		return fmt.Errorf("policy must retain its complete typed, descriptor-free, fail-closed AST")
	}
	return nil
}

func darwinReadonlyReadSource(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestDarwinReadOnlyDMSProductionSourceBoundary(t *testing.T) {
	if err := darwinReadonlyCheckPolicy(darwinReadonlyReadSource(t, "readonly_darwin_dms_policy.go")); err != nil {
		t.Fatal(err)
	}
	source := darwinReadonlyReadSource(t, "native_darwin.go")
	for _, contract := range []struct{ name, source string }{
		{"nativeAttachDMS", darwinReadonlyAttachContract},
		{"darwinQuery", darwinReadonlyQueryContract},
		{"darwinAcquire", darwinReadonlyAcquireContract},
		{"darwinRelease", darwinReadonlyReleaseContract},
		{"darwinPoison", darwinReadonlyPoisonContract},
	} {
		if err := darwinReadonlyCheckFunction(source, contract.name, contract.source); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDarwinReadOnlyMappingProductionSourceBoundary(t *testing.T) {
	for _, contract := range []struct{ path, name, source string }{
		{"native_darwin.go", "nativeMap", darwinReadonlyMapContract},
		{"native_darwin.go", "nativeUnmap", darwinReadonlyUnmapContract},
		{"native_darwin.go", "nativeClose", darwinReadonlyNativeCloseContract},
		{"mapping.go", "mappingSpan", darwinReadonlySpanContract},
		{"mapping.go", "MapShared", darwinReadonlyMapSharedContract},
		{"mapping.go", "retireLocked", darwinReadonlyRetireContract},
		{"file.go", "Close", darwinReadonlyCloseContract},
		{"file.go", "closeResourcesLocked", darwinReadonlyResourcesContract},
	} {
		if err := darwinReadonlyCheckFunction(darwinReadonlyReadSource(t, contract.path), contract.name, contract.source); err != nil {
			t.Fatal(err)
		}
	}
}

// Each mutant changes a copy of the actual production source in memory. It is
// parsed to ensure it is syntactically valid, then rejected by the same boundary
// used above. No mutant is written, compiled, evaluated or invoked.
func TestDarwinReadOnlySourceMutationCounterexamples(t *testing.T) {
	native := darwinReadonlyReadSource(t, "native_darwin.go")
	policy := darwinReadonlyReadSource(t, "readonly_darwin_dms_policy.go")
	if err := darwinReadonlyCheckFunction(native, "nativeAttachDMS", darwinReadonlyAttachContract); err != nil {
		t.Fatal(err)
	}
	if err := darwinReadonlyCheckFunction(native, "nativeMap", darwinReadonlyMapContract); err != nil {
		t.Fatal(err)
	}
	if err := darwinReadonlyCheckPolicy(policy); err != nil {
		t.Fatal(err)
	}
	for _, mutant := range []struct {
		name, function, before, after string
	}{
		{"remove decision", "nativeAttachDMS", "initial, err := readonlyDarwinDMSDecision(f.writable, peer)", "initial := true"},
		{"invert capability", "nativeAttachDMS", "readonlyDarwinDMSDecision(f.writable, peer)", "readonlyDarwinDMSDecision(!f.writable, peer)"},
		{"substitute writable", "nativeAttachDMS", "readonlyDarwinDMSDecision(f.writable, peer)", "readonlyDarwinDMSDecision(true, peer)"},
		{"ignore refusal", "nativeAttachDMS", "initial, err := readonlyDarwinDMSDecision(f.writable, peer)\n\tif err != nil {\n\t\treturn err\n\t}", "initial, err := readonlyDarwinDMSDecision(f.writable, peer)\n\tif err != nil {\n\t\terr = nil\n\t}"},
		{"unknown becomes unheld", "nativeAttachDMS", "peer := darwinDMSUnknown", "peer := darwinDMSUnheld"},
		{"shared becomes unheld", "nativeAttachDMS", "peer = darwinDMSShared", "peer = darwinDMSUnheld"},
		{"exclusive becomes shared", "nativeAttachDMS", "peer = darwinDMSExclusive", "peer = darwinDMSShared"},
		{"bypass local registry", "nativeAttachDMS", "if d.node.kernel[darwinDMS] == darwinRead {", "if false {"},
		{"drop registry lock", "nativeAttachDMS", "darwinRegistry.Lock()", "_ = 0"},
		{"query before local hold", "nativeAttachDMS", "if d.node.kernel[darwinDMS] == darwinRead {", "_, _ = darwinQuery(d, darwinDMS)\n\tif d.node.kernel[darwinDMS] == darwinRead {"},
		{"lost initialization race falls through", "nativeAttachDMS", "// A lost initialization race is BUSY; do not fall through to read.\n\t\t\treturn err", "// Inert counterexample only.\n\t\t\terr = nil"},
		{"gate after truncate", "nativeAttachDMS", "initial, err := readonlyDarwinDMSDecision(f.writable, peer)", "_ = d.raw.Truncate(3)\n\tinitial, err := readonlyDarwinDMSDecision(f.writable, peer)"},
		{"ignore initialization decision", "nativeAttachDMS", "if initial {", "if true {"},
		{"invert initialization decision", "nativeAttachDMS", "if initial {", "if !initial {"},
		{"lose truncate poison", "nativeAttachDMS", "if err = d.raw.Truncate(3); err != nil {\n\t\t\treturn darwinPoison(d, err)", "if err = d.raw.Truncate(3); err != nil {\n\t\t\treturn err"},
		{"lose downgrade poison", "nativeAttachDMS", "if err != nil && initial {\n\t\treturn darwinPoison(d, err)", "if err != nil && initial {\n\t\treturn err"},
		{"alternate exclusive path", "nativeAttachDMS", "peer := darwinDMSUnknown", "_ = darwinAcquire(d, darwinDMS, darwinWrite)\n\tpeer := darwinDMSUnknown"},
		{"force write base", "nativeMap", "protection := unix.PROT_READ", "protection := unix.PROT_READ | unix.PROT_WRITE"},
		{"invert mapping capability", "nativeMap", "if f.writable {", "if !f.writable {"},
		{"ignore computed protection", "nativeMap", "off, n, protection, unix.MAP_SHARED", "off, n, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED"},
		{"alternate mapping before gate", "nativeMap", "protection := unix.PROT_READ", "_, _ = unix.Mmap(int(f.raw.Fd()), off, n, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)\n\tprotection := unix.PROT_READ"},
		{"mapping grows backing file", "nativeMap", "protection := unix.PROT_READ", "_ = f.raw.Truncate(off + int64(n))\n\tprotection := unix.PROT_READ"},
		{"private mapping", "nativeMap", "unix.MAP_SHARED", "unix.MAP_PRIVATE"},
	} {
		t.Run(mutant.name, func(t *testing.T) {
			fileSet := token.NewFileSet()
			parsed, err := parser.ParseFile(fileSet, "native_darwin.go", native, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			function, err := darwinReadonlyFunction(parsed, mutant.function)
			if err != nil {
				t.Fatal(err)
			}
			start, end := fileSet.Position(function.Pos()).Offset, fileSet.Position(function.End()).Offset
			body := native[start:end]
			if strings.Count(body, mutant.before) != 1 {
				t.Fatalf("mutant must change one real production expression: %q", mutant.before)
			}
			changed := native[:start] + strings.Replace(body, mutant.before, mutant.after, 1) + native[end:]
			if _, err := darwinReadonlyParse("inert.go", changed); err != nil {
				t.Fatalf("counterexample must remain syntactically valid: %v", err)
			}
			contract := darwinReadonlyAttachContract
			if mutant.function == "nativeMap" {
				contract = darwinReadonlyMapContract
			}
			if err := darwinReadonlyCheckFunction(changed, mutant.function, contract); err == nil {
				t.Fatal("actual production boundary accepted inert counterexample")
			}
		})
	}
	for _, mutant := range []struct{ name, before, after string }{
		{"readonly initializes", "if !writable {", "if writable {"},
		{"zero authorizes initialization", "default:\n\t\treturn false, ErrUnsupported", "default:\n\t\treturn true, nil"},
		{"unheld shared equivalence", "case darwinDMSShared:", "case darwinDMSUnknown, darwinDMSShared:"},
		{"policy gets initialization side effect", "package vfs", "package vfs\nfunc init() { panic(\"inert source only\") }"},
	} {
		t.Run(mutant.name, func(t *testing.T) {
			if strings.Count(policy, mutant.before) != 1 {
				t.Fatal("policy mutation anchor is not unique")
			}
			changed := strings.Replace(policy, mutant.before, mutant.after, 1)
			if _, err := darwinReadonlyParse("inert.go", changed); err != nil {
				t.Fatalf("counterexample must remain syntactically valid: %v", err)
			}
			if err := darwinReadonlyCheckPolicy(changed); err == nil {
				t.Fatal("complete policy boundary accepted inert counterexample")
			}
		})
	}
}

// Readable contract ASTs make every accepted branch and argument reviewable.
// They are finite source assertions, not runtime acceptance or native evidence.
const darwinReadonlyAttachContract = `func nativeAttachDMS(f *File) error {
	darwinRegistry.Lock()
	defer darwinRegistry.Unlock()
	d := f.native.descriptor
	if d.node.poisoned || darwinRegistry.Error(d.node.identity) != nil {
		return ErrPoisoned
	}
	if d.node.kernel[darwinDMS] == darwinRead {
		// Additional local Index leases share the live process hold. They must
		// never query their own POSIX lock and reset already mapped storage.
		return darwinAcquire(d, darwinDMS, darwinRead)
	}
	kind, err := darwinQuery(d, darwinDMS)
	if err != nil {
		return err
	}
	peer := darwinDMSUnknown
	switch kind {
	case unix.F_UNLCK:
		peer = darwinDMSUnheld
	case unix.F_RDLCK:
		peer = darwinDMSShared
	case unix.F_WRLCK:
		peer = darwinDMSExclusive
	}
	initial, err := readonlyDarwinDMSDecision(f.writable, peer)
	if err != nil {
		return err
	}
	if initial {
		if err = darwinAcquire(d, darwinDMS, darwinWrite); err != nil {
			// A lost initialization race is BUSY; do not fall through to read.
			return err
		}
		if err = d.raw.Truncate(3); err != nil {
			return darwinPoison(d, err)
		}
	}
	err = darwinAcquire(d, darwinDMS, darwinRead)
	if err != nil && initial {
		return darwinPoison(d, err)
	}
	return err
}`

const darwinReadonlyQueryContract = `func darwinQuery(d *darwinDescriptor, r darwinRange) (int16, error) {
	l := unix.Flock_t{Type: unix.F_WRLCK, Whence: io.SeekStart, Start: r.start, Len: r.length}
	if err := unix.FcntlFlock(d.raw.Fd(), unix.F_GETLK, &l); err != nil {
		return 0, fmt.Errorf("Darwin lock query: %w", err)
	}
	return l.Type, nil
}`

const darwinReadonlyAcquireContract = `func darwinAcquire(d *darwinDescriptor, r darwinRange, mode uint8) error {
	if d.holds[r] == mode {
		return nil
	}
	other := darwinAggregateExcept(d.node, d, r)
	if other == darwinWrite || mode == darwinWrite && other != 0 {
		return ErrBusy
	}
	if d.node.kernel[r] != mode {
		if err := darwinFcntl(d, r, mode); err != nil {
			return err
		}
		d.node.kernel[r] = mode
		darwinRegistry.SetLocks(d.node.identity, "vfs", true)
	}
	d.holds[r] = mode
	return nil
}`

const darwinReadonlyReleaseContract = `func darwinRelease(d *darwinDescriptor, r darwinRange) error {
	if d.holds[r] == 0 {
		return nil
	}
	remaining := darwinAggregateExcept(d.node, d, r)
	if d.node.kernel[r] != remaining {
		if err := darwinFcntl(d, r, remaining); err != nil {
			return err
		}
		if remaining == 0 {
			delete(d.node.kernel, r)
		} else {
			d.node.kernel[r] = remaining
		}
	}
	delete(d.holds, r)
	darwinRegistry.SetLocks(d.node.identity, "vfs", len(d.node.kernel) != 0)
	return darwinOwnerError(darwinRegistry.Drain(d.node.identity))
}`

const darwinReadonlyPoisonContract = `func darwinPoison(d *darwinDescriptor, errs ...error) error {
	d.node.poisoned = true
	darwinRegistry.Poison(d.node.identity, errors.Join(errs...))
	return errors.Join(append([]error{ErrPoisoned}, errs...)...)
}`

const darwinReadonlyMapContract = `func nativeMap(f *File, off int64, n int) (nativeMapping, []byte, error) {
	protection := unix.PROT_READ
	if f.writable {
		protection |= unix.PROT_WRITE
	}
	b, err := unix.Mmap(int(f.raw.Fd()), off, n, protection, unix.MAP_SHARED)
	return nativeMapping{view: b}, b, err
}`

const darwinReadonlyUnmapContract = `func nativeUnmap(m *nativeMapping) error {
	if err := unix.Munmap(m.view); err != nil {
		return err
	}
	m.view = nil
	return nil
}`

const darwinReadonlyNativeCloseContract = `func nativeClose(f *File) error {
	darwinRegistry.Lock()
	defer darwinRegistry.Unlock()
	d := f.native.descriptor
	if d == nil {
		// nativeOpened always associates a registered descriptor before it can
		// report a validation error, so this is an ownership invariant failure.
		return ErrPoisoned
	}
	if d.idle {
		return darwinOwnerError(darwinRegistry.Retire(d.record))
	}
	// All mappings of this owner were already retired by common Close. Release
	// slots before DMS, and main SHARED before PENDING. Other owners' aggregate
	// holds persist regardless of which descriptor originally acquired them.
	ordered := make([]darwinRange, 0, 12)
	for i := 0; i < 8; i++ {
		ordered = append(ordered, darwinRange{SHMFirst + int64(i), 1})
	}
	ordered = append(ordered, darwinDMS, darwinReserved, darwinShared, darwinPending)
	for _, r := range ordered {
		if err := darwinRelease(d, r); err != nil {
			return darwinPoison(d, err)
		}
	}
	d.idle = true
	return darwinOwnerError(darwinRegistry.Retire(d.record))
}`

const darwinReadonlySpanContract = `func mappingSpan(off int64, n int, granularity int64) (base int64, delta, length int, err error) {
	if off < 0 || n <= 0 || granularity <= 0 || granularity&(granularity-1) != 0 || off > math.MaxInt64-int64(n) {
		return 0, 0, 0, ErrUnsupported
	}
	base = off / granularity * granularity
	d := off - base
	if d > int64(math.MaxInt)-int64(n) {
		return 0, 0, 0, ErrUnsupported
	}
	return base, int(d), int(d) + n, nil
}`

const darwinReadonlyMapSharedContract = `func (f *File) MapShared(off int64, n int) (*Mapping, error) {
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
}`

const darwinReadonlyRetireContract = `func (m *Mapping) retireLocked(unmap func(*nativeMapping) error) error {
	if m.Bytes == nil {
		return nil
	}
	if err := unmap(&m.native); err != nil {
		return err
	}
	m.Bytes = nil
	delete(m.owner.maps, m)
	return nil
}`

const darwinReadonlyCloseContract = `func (f *File) Close() error {
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
}`

const darwinReadonlyResourcesContract = `func (f *File) closeResourcesLocked(unmap func(*nativeMapping) error, closeFile func() error) error {
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
}`

const darwinReadonlyPolicyContract = `package vfs

// darwinDMSPeer describes a successful external F_GETLK observation. The zero
// value is deliberately unknown, never evidence that initialization is safe.
// Keep this policy portable: it owns no descriptor and performs no native I/O.
type darwinDMSPeer uint8

const (
	darwinDMSUnknown darwinDMSPeer = iota
	darwinDMSUnheld
	darwinDMSShared
	darwinDMSExclusive
)

// readonlyDarwinDMSDecision is consumed by nativeAttachDMS before its exclusive
// acquisition or truncate. The native process registry handles a tracked local
// shared hold first because F_GETLK does not report this process's own locks.
// Shared admission does not initialize or prove immutability. Public read-only
// shared WAL remains Linux-only; this is internal Darwin backend groundwork.
func readonlyDarwinDMSDecision(writable bool, peer darwinDMSPeer) (initialize bool, err error) {
	switch peer {
	case darwinDMSUnheld:
		if !writable {
			return false, ErrUnsupported
		}
		return true, nil
	case darwinDMSShared:
		return false, nil
	case darwinDMSExclusive:
		return false, ErrBusy
	default:
		return false, ErrUnsupported
	}
}
`
