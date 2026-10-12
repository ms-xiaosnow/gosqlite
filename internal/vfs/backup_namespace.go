package vfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

// The first backup profile requires these actual statx returned fields. Device
// numbers have no separate availability bit in statx. Unique mount IDs must not
// be substituted with recyclable STATX_MNT_ID values or zero-filled fields.
const backupRequiredMask uint32 = 0x0001 | 0x0004 | 0x0100 | 0x0200 | 0x4000
const backupRegular uint16 = 0100000
const backupDirectory uint16 = 0040000
const backupTypeMask uint16 = 0170000
const backupMaximumBase = 200

type backupFact struct {
	mask uint32
	mode uint16
	links uint32
	devMajor, devMinor uint32
	inode, mount, size uint64
}

func (f backupFact) complete() bool { return f.mask&backupRequiredMask == backupRequiredMask }
func (f backupFact) samePhysical(g backupFact) bool {
	return f.devMajor == g.devMajor && f.devMinor == g.devMinor && f.inode == g.inode
}
func backupDirectChild(parent, held, named backupFact) error {
	if !parent.complete() || !held.complete() || !named.complete() {
		return fmt.Errorf("%w: backup requires all unique-mount statx fields", ErrUnsupported)
	}
	if parent.mode&backupTypeMask != backupDirectory || held.mode&backupTypeMask != backupRegular || named.mode&backupTypeMask != backupRegular || held.links != 1 || named.links != 1 {
		return ErrUnsupported
	}
	if !held.samePhysical(named) || held.mount != named.mount || held.mount != parent.mount || held.devMajor != parent.devMajor || held.devMinor != parent.devMinor {
		return ErrIdentity
	}
	return nil
}

func backupSourceSidecar(parent backupFact, suffix string, fact backupFact) error {
	if suffix != "-wal" || fact.size != 0 { return ErrUnsupported }
	return backupDirectChild(parent, fact, fact)
}

// BackupBasename admits a bounded ordinary SQLite main name, not a sidecar name.
// ASCII folding deliberately rejects case-only aliases even on case-sensitive
// storage. Hidden aliases elsewhere are not globally discoverable by this API.
func BackupBasename(name string) bool {
	if name == "" || name == "." || name == ".." || len(name) > backupMaximumBase {
		return false
	}
	for i := 0; i < len(name); i++ {
		b := name[i]
		if !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '.' || b == '_' || b == '-') {
			return false
		}
	}
	lower := strings.ToLower(name)
	return !strings.HasSuffix(lower, "-journal") && !strings.HasSuffix(lower, "-wal") && !strings.HasSuffix(lower, "-shm")
}
func backupFamiliesDisjoint(a, b string) bool {
	for _, x := range []string{"", "-journal", "-wal", "-shm"} {
		for _, y := range []string{"", "-journal", "-wal", "-shm"} {
			if strings.EqualFold(a+x, b+y) { return false }
		}
	}
	return true
}

// BackupSourceIdentity is an inert snapshot minted by a live source namespace.
// It conveys neither a source descriptor nor any source cleanup authority.
type BackupSourceIdentity struct {
	parent backupFact
	base string
	valid bool
}

// backupLease has a reserved sole-owner slot before any syscall returns a
// descriptor. fd >= 0 means attachment itself did not finish; it is terminal
// rather than a raw-descriptor retry route. File stays private to this protocol.
type backupLease struct {
	file *File
	fd int
	attempted bool
}
func newBackupLease(role Role) backupLease {
	return backupLease{file: &File{role: role, writable: role == Database, maps: make(map[*Mapping]struct{})}, fd: -1}
}
func (l *backupLease) complete() bool { return l.fd < 0 && l.file.RetirementComplete() }
func (l *backupLease) close() error {
	if l.fd >= 0 { return ErrPoisoned }
	return l.file.Close()
}
func (l *backupLease) seal() {
	l.file.mu.Lock()
	l.file.closing = true
	l.file.mu.Unlock()
}

// backupDiagnostic deliberately has no Unwrap. Interior cleanup tokens cannot
// escape to a generic adopter and retire a protector behind its composite.
// Is/As operate on a bounded sanitized diagnostic tree containing no owners.
type backupDiagnostic struct{ safe error }
func (e *backupDiagnostic) Error() string { return e.safe.Error() }
func (e *backupDiagnostic) Is(target error) bool { return errors.Is(e.safe, target) }
func (e *backupDiagnostic) As(target any) bool { return errors.As(e.safe, target) }

func backupError(input error, allowed ...*File) (error, bool) {
	if input == nil { return nil, false }
	count := 0
	bad := false
	var clean func(error) error
	clean = func(err error) error {
		count++
		if count > 128 { bad = true; return ErrCleanupLimit }
		if err == nil { return nil }
		switch e := err.(type) {
		case *cleanupError:
			known := false
			for _, f := range allowed { known = known || e.file != nil && e.file == f }
			if !known { bad = true; return ErrPoisoned }
			return clean(e.cause)
		case CleanupOwner:
			bad = true
			return ErrPoisoned
		case *os.PathError:
			return &os.PathError{Op: e.Op, Path: e.Path, Err: clean(e.Err)}
		case *os.LinkError:
			return &os.LinkError{Op: e.Op, Old: e.Old, New: e.New, Err: clean(e.Err)}
		case *os.SyscallError:
			return &os.SyscallError{Syscall: e.Syscall, Err: clean(e.Err)}
		case interface{ Unwrap() []error }:
			children := e.Unwrap()
			if len(children) > 128-count { bad = true; return ErrCleanupLimit }
			out := make([]error, 0, len(children))
			for _, child := range children { out = append(out, clean(child)) }
			return errors.Join(out...)
		case interface{ Unwrap() error }:
			return clean(e.Unwrap())
		case interface{ As(any) bool }:
			bad = true
			return ErrPoisoned
		case interface{ Is(error) bool }:
			if _, ok := err.(syscall.Errno); !ok { bad = true; return ErrPoisoned }
			return err
		default:
			return err
		}
	}
	safe := clean(input)
	if bad { safe = errors.Join(ErrPoisoned, ErrCleanupLimit) }
	return &backupDiagnostic{safe: safe}, bad
}

// BackupSourceNamespace owns only the temporary retained source directory. The
// Reader owns the already-open main; none of its descriptors or cleanup tokens
// enter the destination owner. Lock order: Reader -> namespace -> main -> dir.
// NewBackupSourceNamespace is memory-only and must be installed before Acquire.
type BackupSourceNamespace struct {
	mu sync.Mutex
	dir backupLease
	identity BackupSourceIdentity
	sealed, terminal bool
	acquisitionAttempted, benignAcquisitionRefusal bool
	unsafe error
}
func NewBackupSourceNamespace() *BackupSourceNamespace {
	return &BackupSourceNamespace{dir: newBackupLease(Directory)}
}
func (s *BackupSourceNamespace) failure(err error) error {
	diagnostic, terminal := backupError(err, s.dir.file)
	if terminal { s.terminal = true; if s.unsafe == nil { s.unsafe = err } }
	return diagnostic
}
// A refusal fact is minted only at the two read-only acquisition edges below.
// It is not an error classifier for topology, observation or publication errors.
type backupSourceRefusalEdge uint8
const (
	backupSourceUnknownRefusal backupSourceRefusalEdge = iota
	backupSourceNameRefusal
	backupSourceParentRefusal
)

// Only the concrete capability/policy leaves produced by directory acquisition
// qualify. Unsupported joined with identity, poison, ownership or an unrelated
// diagnostic never qualifies. Traversal is bounded and invokes no Is/As hook.
func backupSourcePolicyRefusal(input error) bool {
	stack := []error{input}
	seenUnsupported, count := false, 0
	for len(stack) != 0 {
		count++
		if count > 128 { return false }
		err := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if err == ErrUnsupported { seenUnsupported = true; continue }
		switch e := err.(type) {
		case syscall.Errno:
			if e != syscall.ENOSYS && e != syscall.EINVAL && e != syscall.EOPNOTSUPP && e != syscall.EXDEV { return false }
		case CleanupOwner:
			return false
		case interface{ Is(error) bool }:
			return false
		case interface{ As(any) bool }:
			return false
		case interface{ Unwrap() []error }:
			children := e.Unwrap()
			if len(children) > 128-count-len(stack) { return false }
			stack = append(stack, children...)
		case interface{ Unwrap() error }:
			stack = append(stack, e.Unwrap())
		default:
			return false
		}
	}
	return seenUnsupported
}
func (s *BackupSourceNamespace) recordBenignAcquisitionRefusal(edge backupSourceRefusalEdge, cause error, matchedParent bool) {
	if !s.acquisitionAttempted || s.sealed || s.terminal || s.identity.valid { return }
	if edge == backupSourceNameRefusal && cause == ErrUnsupported || edge == backupSourceParentRefusal && matchedParent && backupSourcePolicyRefusal(cause) {
		s.benignAcquisitionRefusal = true
	}
}
// BenignAcquisitionRefusal is an inert historical admission fact, not cleanup
// permission. The owner must independently establish first-attempt retirement.
func (s *BackupSourceNamespace) BenignAcquisitionRefusal() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.benignAcquisitionRefusal && !s.terminal && !s.identity.valid
}
func (s *BackupSourceNamespace) Acquire(main *File) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sealed || s.terminal || s.acquisitionAttempted || s.dir.attempted || main == nil { return ErrPoisoned }
	s.acquisitionAttempted = true
	main.mu.Lock()
	defer main.mu.Unlock()
	if err := main.checkLocked(); err != nil { return s.failure(err) }
	if main.role != Database || !main.shared || main.parent == nil { return ErrUnsupported }
	if !BackupBasename(filepath.Base(main.path)) {
		s.recordBenignAcquisitionRefusal(backupSourceNameRefusal, ErrUnsupported, false)
		return ErrUnsupported
	}
	if err := nativeBackupOpenDirectory(&s.dir, filepath.Dir(main.path)); err != nil {
		matchedParent := s.dir.fd < 0 && s.dir.file.identity != nil && s.dir.file.identity.IsDir() && SameFile(s.dir.file.identity, main.parent)
		diagnostic := s.failure(err)
		s.recordBenignAcquisitionRefusal(backupSourceParentRefusal, err, matchedParent)
		return diagnostic
	}
	if !SameFile(s.dir.file.identity, main.parent) { return ErrIdentity }
	parent, err := nativeBackupObserveSource(main, &s.dir, filepath.Base(main.path))
	if err != nil { return s.failure(err) }
	s.identity = BackupSourceIdentity{parent: parent, base: filepath.Base(main.path), valid: true}
	return nil
}
func (s *BackupSourceNamespace) ObserveDirectSourceFamily(main *File) (BackupSourceIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sealed || s.terminal || !s.identity.valid || main == nil { return BackupSourceIdentity{}, ErrPoisoned }
	main.mu.Lock()
	defer main.mu.Unlock()
	if err := main.checkLocked(); err != nil { return BackupSourceIdentity{}, err }
	parent, err := nativeBackupObserveSource(main, &s.dir, s.identity.base)
	if err != nil { return BackupSourceIdentity{}, s.failure(err) }
	if !parent.samePhysical(s.identity.parent) || parent.mount != s.identity.parent.mount { return BackupSourceIdentity{}, ErrIdentity }
	return s.identity, nil
}
func (s *BackupSourceNamespace) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sealed = true
	s.identity.valid = false
	s.dir.seal()
	if s.terminal { return ErrPoisoned }
	return s.failure(s.dir.close())
}
func (s *BackupSourceNamespace) RetirementComplete() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sealed && !s.terminal && s.dir.complete()
}
