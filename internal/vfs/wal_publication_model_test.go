package vfs

import (
	"bytes"
	"errors"
	"io"
	"os"
	"reflect"
	"syscall"
	"testing"
	"time"
)

// These are inert integer/byte/event models of the actual production facade.
// File values are identity markers only; no File method or native factory runs.
type walPublicationModelInfo struct{ size int64 }
func (i walPublicationModelInfo) Name() string { return "memory" }
func (i walPublicationModelInfo) Size() int64 { return i.size }
func (walPublicationModelInfo) Mode() os.FileMode { return 0600 }
func (walPublicationModelInfo) ModTime() time.Time { return time.Time{} }
func (walPublicationModelInfo) IsDir() bool { return false }
func (walPublicationModelInfo) Sys() any { return nil }

type walPublicationModel struct {
	events []string
	fail map[string]error
	source, candidate []byte
	main, old, next, directory *File
	known, transferred bool
	childDone, directoryDone bool
	shortWrite bool
}
func newWALPublicationModel() (*WALPublication, *walPublicationModel) {
	m := &walPublicationModel{fail: make(map[string]error), source: bytes.Repeat([]byte{7}, 64),
		main: &File{}, old: &File{}, next: &File{}, directory: &File{}, known: true, childDone: true, directoryDone: true}
	return newWALPublicationWithOps(m), m
}
func (m *walPublicationModel) event(name string) error { m.events = append(m.events, name); return m.fail[name] }
func (m *walPublicationModel) allowedFiles() []*File { return []*File{m.main, m.old, m.next, m.directory} }
func (m *walPublicationModel) acquire() error { return m.event("acquire") }
func (m *walPublicationModel) create() error { return m.event("create") }
func (m *walPublicationModel) readSource(b []byte, off int64) (int, error) {
	if err := m.event("read"); err != nil { return 0, err }
	return bytes.NewReader(m.source).ReadAt(b, off)
}
func (m *walPublicationModel) writeCandidate(b []byte, off int64) (int, error) {
	err := m.event("write")
	n := len(b)
	if m.shortWrite { n /= 2 }
	if end := int(off)+n; end > len(m.candidate) { m.candidate = append(m.candidate, make([]byte, end-len(m.candidate))...) }
	copy(m.candidate[int(off):], b[:n])
	return n, err
}
func (m *walPublicationModel) candidateInfo() (os.FileInfo, error) {
	if err := m.event("info"); err != nil { return nil, err }
	return walPublicationModelInfo{int64(len(m.candidate))}, nil
}
func (m *walPublicationModel) recheck() error { return m.event("recheck") }
func (m *walPublicationModel) syncCandidate() error { return m.event("candidate-sync") }
func (m *walPublicationModel) replace() (bool, error) { return m.known, m.event("replace") }
func (m *walPublicationModel) transfer(main, old *File) (*File, error) {
	if main != m.main || old != m.old || m.transferred { return nil, ErrIdentity }
	if err := m.event("transfer"); err != nil { return nil, err }
	m.transferred = true
	return m.next, nil
}
func (m *walPublicationModel) postPublication() (os.FileInfo, error) {
	if err := m.event("post-shm"); err != nil { return nil, err }
	return walPublicationModelInfo{int64(len(m.candidate))}, nil
}
func (m *walPublicationModel) syncDirectory() error { return m.event("directory-sync") }
func (m *walPublicationModel) seal() { _ = m.event("seal") }
func (m *walPublicationModel) closeChild() (bool, error) { return m.childDone, m.event("child-close") }
func (m *walPublicationModel) closeDirectory() (bool, error) { return m.directoryDone, m.event("directory-close") }
func walPublicationModelPrepared(t *testing.T, p *WALPublication) {
	t.Helper()
	if err := p.Acquire(); err != nil { t.Fatal(err) }
	if err := p.CreateCandidate(32); err != nil { t.Fatal(err) }
	b := make([]byte, 32)
	if _, err := p.Source().ReadAt(b, 0); err != nil { t.Fatal(err) }
	if _, err := p.WriteAt(b, 0); err != nil { t.Fatal(err) }
	if err := p.Prepare(32); err != nil { t.Fatal(err) }
}
func walPublicationModelCount(events []string, name string) int {
	n := 0
	for _, e := range events { if e == name { n++ } }
	return n
}
func TestWALPublicationMemorySuccessOrdering(t *testing.T) {
	p, m := newWALPublicationModel()
	original := append([]byte(nil), m.source...)
	walPublicationModelPrepared(t, p)
	if p.Durable() { t.Fatal("premature durable state") }
	if known, err := p.Replace(); !known || err != nil { t.Fatal(known, err) }
	if p.Durable() { t.Fatal("rename exposed clean state") }
	next, err := p.TransferWAL(m.main, m.old)
	if err != nil || next != m.next || !m.transferred { t.Fatal("lost owner", err) }
	if _, err = p.FinishPublication(); err != nil || !p.Durable() { t.Fatal(err) }
	if err = p.Close(); err != nil || !p.RetirementComplete() { t.Fatal(err) }
	if err = p.Close(); err != nil { t.Fatal(err) }
	want := []string{"acquire", "create", "info", "read", "write", "info", "recheck", "candidate-sync", "recheck", "replace", "transfer", "post-shm", "directory-sync", "seal", "child-close", "directory-close"}
	if !reflect.DeepEqual(m.events, want) || !bytes.Equal(m.source, original) || !bytes.Equal(m.candidate, original[:32]) { t.Fatal(m.events) }
	if _, err := p.WriteAt([]byte{1}, 0); !errors.Is(err, ErrPoisoned) { t.Fatal(err) }
}
func TestWALPublicationMemoryOneShotFailures(t *testing.T) {
	cause := errors.New("inert publication diagnostic")
	for _, phase := range []string{"acquire", "create", "info", "read", "write", "recheck", "candidate-sync", "replace", "transfer", "post-shm", "directory-sync"} {
		t.Run(phase, func(t *testing.T) {
			p, m := newWALPublicationModel()
			m.fail[phase] = cause
			var err error
			if err = p.Acquire(); err == nil { err = p.CreateCandidate(32) }
			if err == nil { _, err = p.Source().ReadAt(make([]byte, 32), 0) }
			if err == nil { _, err = p.WriteAt(make([]byte, 32), 0) }
			if err == nil { err = p.Prepare(32) }
			attempted := false
			if err == nil {
				attempted = true
				if phase == "replace" { m.known = false }
				known, replacement := p.Replace()
				err = replacement
				if known { _, transfer := p.TransferWAL(m.main, m.old); err = errors.Join(err, transfer) }
				_, barrier := p.FinishPublication()
				err = errors.Join(err, barrier)
			}
			if !errors.Is(err, cause) || p.Durable() { t.Fatal("failure erased or published", err) }
			if attempted && walPublicationModelCount(m.events, "directory-sync") != 1 { t.Fatal("missing protective barrier", m.events) }
			_, _ = p.Replace()
			_, _ = p.FinishPublication()
			if walPublicationModelCount(m.events, "replace") > 1 || walPublicationModelCount(m.events, "directory-sync") > 1 { t.Fatal("native phase retried") }
			_ = p.Close()
			if walPublicationModelCount(m.events, "child-close") != 1 || walPublicationModelCount(m.events, "directory-close") != 1 { t.Fatal(m.events) }
		})
	}
}
func TestWALPublicationMemoryCloseOutcomeMatrix(t *testing.T) {
	cause := errors.New("inert close diagnostic")
	for child := 0; child < 4; child++ {
		for directory := 0; directory < 4; directory++ {
			p, m := newWALPublicationModel()
			m.childDone, m.directoryDone = child < 2, directory < 2
			if child&1 != 0 { m.fail["child-close"] = cause }
			if directory&1 != 0 { m.fail["directory-close"] = cause }
			err := p.Close()
			if p.RetirementComplete() != (child < 2 && directory < 2) { t.Fatal(child, directory, err) }
			if child&1 != 0 || child < 2 && directory&1 != 0 {
				if !errors.Is(err, cause) { t.Fatal("consumed diagnostic lost", err) }
			}
			_ = p.Close()
			_ = p.Close()
			if walPublicationModelCount(m.events, "child-close") != 1 { t.Fatal("unknown child close retried") }
			wantDir := 0
			if child < 2 { wantDir = 1 }
			if walPublicationModelCount(m.events, "directory-close") != wantDir { t.Fatal("directory passed incomplete child", m.events) }
		}
	}
}
func TestWALPublicationMemoryBoundsAndTransfer(t *testing.T) {
	p, m := newWALPublicationModel()
	if _, ok := p.Source().(io.WriterAt); ok { t.Fatal("source writable") }
	if err := p.Acquire(); err != nil { t.Fatal(err) }
	if err := p.CreateCandidate(32); err != nil { t.Fatal(err) }
	if _, err := p.WriteAt(make([]byte, 33), 0); !errors.Is(err, ErrPoisoned) { t.Fatal(err) }
	if err := p.Prepare(32); !errors.Is(err, ErrIdentity) { t.Fatal("short candidate admitted", err) }
	if walPublicationModelCount(m.events, "candidate-sync") != 0 { t.Fatal("short candidate synced") }
	p, m = newWALPublicationModel()
	if err := p.Acquire(); err != nil { t.Fatal(err) }
	if err := p.CreateCandidate(32); err != nil { t.Fatal(err) }
	m.shortWrite = true
	if _, err := p.WriteAt(make([]byte, 32), 0); !errors.Is(err, io.ErrShortWrite) { t.Fatal("short nil write lost", err) }
	if err := p.Prepare(32); !errors.Is(err, ErrPoisoned) || walPublicationModelCount(m.events, "candidate-sync") != 0 { t.Fatal("failed write remained usable", err) }
	p, m = newWALPublicationModel()
	walPublicationModelPrepared(t, p)
	if _, err := p.TransferWAL(m.main, m.old); !errors.Is(err, ErrPoisoned) { t.Fatal("transfer before rename") }
	_, _ = p.Replace()
	if _, err := p.TransferWAL(m.old, m.main); !errors.Is(err, ErrIdentity) { t.Fatal(err) }
	_, _ = p.FinishPublication()
	if p.Durable() || walPublicationModelCount(m.events, "directory-sync") != 1 { t.Fatal(m.events) }
}

type walPublicationCycle struct{}
func (*walPublicationCycle) Error() string { return "inert cycle" }
func (e *walPublicationCycle) Unwrap() error { return e }
func TestWALPublicationMemoryDiagnosticOwnership(t *testing.T) {
	cause := errors.New("inert owner diagnostic")
	for _, kind := range []string{"known", "foreign", "cycle", "over-budget", "borrowed"} {
		p, m := newWALPublicationModel()
		var input error
		switch kind {
		case "known": input = &os.PathError{Op: "memory", Path: "inert", Err: retainedFileCleanup(m.next, cause)}
		case "borrowed": input = retainedFileCleanup(m.old, cause)
		case "foreign": input = retainedFileCleanup(&File{}, cause)
		case "cycle": input = &walPublicationCycle{}
		case "over-budget":
			errs := make([]error, 129)
			for i := range errs { errs[i] = cause }
			input = errors.Join(errs...)
		}
		m.fail["acquire"] = input
		err := p.Acquire()
		owners, discovery := CleanupOwnersBounded(err, 256)
		if discovery != nil || len(owners) != 0 { t.Fatal("interior owner escaped", kind, discovery) }
		if kind == "known" || kind == "borrowed" {
			if !errors.Is(err, cause) { t.Fatal("safe cause lost") }
			if kind == "known" { var path *os.PathError; if !errors.As(err, &path) || path.Path != "inert" { t.Fatal("safe As lost") } }
			_ = p.Close()
			if !p.RetirementComplete() { t.Fatal(kind) }
		} else {
			_ = p.Close()
			if p.RetirementComplete() || walPublicationModelCount(m.events, "child-close") != 0 { t.Fatal("foreign/cyclic graph released protector", kind) }
		}
	}
}
func TestWALPublicationMemoryNamespaceFacts(t *testing.T) {
	parent := walPublicationFact{mask: walPublicationRequiredMask, mode: walPublicationDirectory, inode: 1, mount: 10, devMajor: 8, devMinor: 1}
	child := walPublicationFact{mask: walPublicationRequiredMask, mode: walPublicationRegular, links: 1, inode: 2, mount: 10, devMajor: 8, devMinor: 1}
	if err := walPublicationDirectChild(parent, child, child); err != nil { t.Fatal(err) }
	for _, mutate := range []func(*walPublicationFact){
		func(f *walPublicationFact) { f.mask &^= 0x4000 }, func(f *walPublicationFact) { f.mount = 0 },
		func(f *walPublicationFact) { f.mount++ }, func(f *walPublicationFact) { f.links = 2 },
		func(f *walPublicationFact) { f.devMinor++ }, func(f *walPublicationFact) { f.mode = 0120000 },
	} {
		bad := child
		mutate(&bad)
		if err := walPublicationDirectChild(parent, bad, bad); err == nil { t.Fatal("unsafe fact admitted", bad) }
	}
	if !child.samePhysical(walPublicationFact{devMajor: 8, devMinor: 1, inode: 2, mount: 99}) { t.Fatal("mount hid physical alias") }
	for _, name := range []string{"", ".", "..", "db-wal", "DB-SHM", "db-journal", "a/b", "a\\b", "数据库", string(bytes.Repeat([]byte{'x'}, 201))} {
		if walPublicationBasename(name) { t.Fatal("unsafe basename", name) }
	}
	if !walPublicationBasename("ordinary.db") { t.Fatal("bounded name refused") }
	main, original, candidate, shm := child, child, child, child
	main.inode, original.inode, candidate.inode, shm.inode = 2, 3, 4, 5
	if err := walPublicationDistinct(main, original, candidate, shm, true); err != nil { t.Fatal(err) }
	for _, alias := range []walPublicationFact{main, original, shm} {
		alias.mount++ // a different mount never excuses a physical alias
		if err := walPublicationDistinct(main, original, alias, shm, true); !errors.Is(err, ErrIdentity) { t.Fatal("candidate physical alias admitted", err) }
	}
	if err := walPublicationDistinct(main, main, candidate, shm, false); !errors.Is(err, ErrIdentity) { t.Fatal("main/WAL physical alias") }
	p, model := newWALPublicationModel()
	if err := p.Acquire(); err != nil { t.Fatal(err) }
	model.fail["create"] = os.ErrExist
	if err := p.CreateCandidate(32); !errors.Is(err, os.ErrExist) { t.Fatal(err) }
	if err := p.CreateCandidate(32); !errors.Is(err, ErrPoisoned) || walPublicationModelCount(model.events, "create") != 1 { t.Fatal("collision retried", err) }
}

// The hook and nil-wrapper graphs are inert; counters prove that diagnostics do
// not format or dispatch unknown hooks. Only the production facade owns them.
type walPublicationHookModel struct { errorCalls, unwrapCalls, hookCalls int }
func (e *walPublicationHookModel) Error() string { e.errorCalls++; return "inert unknown hook" }
func (e *walPublicationHookModel) Unwrap() error { e.unwrapCalls++; return syscall.EIO }
type walPublicationIsModel struct { *walPublicationHookModel }
func (e *walPublicationIsModel) Is(error) bool { e.hookCalls++; return false }
type walPublicationAsModel struct { *walPublicationHookModel }
func (e *walPublicationAsModel) As(any) bool { e.hookCalls++; return false }

type walPublicationNilWrapperModel struct { errorCalls, unwrapCalls int }
func (e *walPublicationNilWrapperModel) Error() string { e.errorCalls++; return "inert nil wrapper" }
func (e *walPublicationNilWrapperModel) Unwrap() error { e.unwrapCalls++; return nil }
type walPublicationNilChildrenModel struct { children []error }
func (*walPublicationNilChildrenModel) Error() string { panic("nil-child graph must remain opaque") }
func (e *walPublicationNilChildrenModel) Unwrap() []error { return e.children }

func walPublicationModelTerminalGraph(t *testing.T, p *WALPublication, m *walPublicationModel, input, diagnostic error) {
	t.Helper()
	if !p.terminal || !p.failed || p.unsafe != input { t.Fatal("original terminal graph not retained opaquely") }
	if diagnostic == nil || diagnostic.Error() == "" || !errors.Is(diagnostic, ErrPoisoned) { t.Fatal("missing safe terminal diagnostic") }
	owners, discovery := CleanupOwnersBounded(diagnostic, 256)
	if discovery != nil || len(owners) != 0 { t.Fatal("interior cleanup owner escaped") }
	for i := 0; i < 2; i++ {
		if err := p.Close(); !errors.Is(err, ErrPoisoned) { t.Fatal("terminal close lost barrier") }
		if p.RetirementComplete() || p.complete || p.childComplete || p.directoryComplete || p.unsafe != input { t.Fatal("terminal graph released protector") }
	}
	if walPublicationModelCount(m.events, "seal") != 1 || walPublicationModelCount(m.events, "child-close") != 0 || walPublicationModelCount(m.events, "directory-close") != 0 { t.Fatal("terminal child or directory retired", m.events) }
}

func TestWALPublicationMemoryUnknownHooksBeforeUnwrap(t *testing.T) {
	for _, kind := range []string{"Is", "As"} {
		for _, sibling := range []bool{false, true} {
			p, m := newWALPublicationModel()
			hooks := &walPublicationHookModel{}
			var input error = &walPublicationIsModel{hooks}
			if kind == "As" { input = &walPublicationAsModel{hooks} }
			if sibling { input = errors.Join(syscall.EIO, input) }
			m.fail["acquire"] = input
			diagnostic := p.Acquire()
			walPublicationModelTerminalGraph(t, p, m, input, diagnostic)
			if errors.Is(diagnostic, syscall.EIO) != sibling { t.Fatal("unknown child traversed or safe errno sibling lost") }
			var path *os.PathError
			if errors.As(diagnostic, &path) { t.Fatal("unknown As supplied an OS error") }
			if hooks.errorCalls != 0 || hooks.unwrapCalls != 0 || hooks.hookCalls != 0 { t.Fatal("unknown error hook invoked", kind) }
		}
	}
}

func TestWALPublicationMemoryUnsafeBranchPreservesErrno(t *testing.T) {
	for _, foreignFirst := range []bool{false, true} {
		p, m := newWALPublicationModel()
		foreign := retainedFileCleanup(&File{}, errors.New("inert foreign owner"))
		safe := &os.PathError{Op: "memory", Path: "inert", Err: syscall.ENOSPC}
		input := errors.Join(syscall.EIO, foreign, safe)
		if foreignFirst { input = errors.Join(foreign, syscall.EIO, safe) }
		m.fail["acquire"] = input
		diagnostic := p.Acquire()
		walPublicationModelTerminalGraph(t, p, m, input, diagnostic)
		if !errors.Is(diagnostic, syscall.EIO) || !errors.Is(diagnostic, syscall.ENOSPC) { t.Fatal("safe errno sibling lost") }
		var path *os.PathError
		if !errors.As(diagnostic, &path) || path.Path != "inert" || path.Err != syscall.ENOSPC { t.Fatal("safe OS diagnostic lost") }
	}
}

func TestWALPublicationMemoryNilWrapperFailsClosed(t *testing.T) {
	for _, kind := range []string{"single", "empty", "nil-child", "path", "nested-empty", "typed-nil-path"} {
		p, m := newWALPublicationModel()
		wrapper := &walPublicationNilWrapperModel{}
		var input error = wrapper
		switch kind {
		case "empty": input = &walPublicationNilChildrenModel{}
		case "nil-child": input = &walPublicationNilChildrenModel{[]error{nil}}
		case "path": input = &os.PathError{Op: "memory", Path: "inert"}
		case "nested-empty": input = &os.PathError{Op: "memory", Path: "inert", Err: &walPublicationNilChildrenModel{}}
		case "typed-nil-path": input = (*os.PathError)(nil)
		}
		m.fail["acquire"] = input
		diagnostic := p.Acquire()
		walPublicationModelTerminalGraph(t, p, m, input, diagnostic)
		if errors.Is(diagnostic, syscall.EIO) { t.Fatal("empty graph invented errno") }
		if wrapper.errorCalls != 0 || kind == "single" && wrapper.unwrapCalls != 1 { t.Fatal("nil wrapper was formatted or retraversed") }
	}
}
