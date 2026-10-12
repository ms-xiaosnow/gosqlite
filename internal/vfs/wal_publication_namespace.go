package vfs

import (
	"errors"
	"os"
	"strings"
	"syscall"
)

// These are required returned fields, including STATX_MNT_ID_UNIQUE rather
// than the recyclable old mount identifier. Device/inode are checked separately.
const walPublicationRequiredMask uint32 = 0x0001 | 0x0004 | 0x0040 | 0x0080 | 0x0100 | 0x0200 | 0x4000
const walPublicationTypeMask uint16 = 0170000
const walPublicationRegular uint16 = 0100000
const walPublicationDirectory uint16 = 0040000

type walPublicationFact struct {
	mask uint32
	mode uint16
	links uint32
	devMajor, devMinor uint32
	inode, mount, size uint64
	mtimeSec, ctimeSec int64
	mtimeNsec, ctimeNsec uint32
}

func (f walPublicationFact) complete() bool {
	return f.mask&walPublicationRequiredMask == walPublicationRequiredMask && f.inode != 0 && f.mount != 0
}
func (f walPublicationFact) samePhysical(g walPublicationFact) bool {
	return f.devMajor == g.devMajor && f.devMinor == g.devMinor && f.inode == g.inode
}
func (f walPublicationFact) stable(g walPublicationFact) bool {
	return f.samePhysical(g) && f.mount == g.mount && f.mode == g.mode && f.links == g.links && f.size == g.size &&
		f.mtimeSec == g.mtimeSec && f.mtimeNsec == g.mtimeNsec && f.ctimeSec == g.ctimeSec && f.ctimeNsec == g.ctimeNsec
}
func walPublicationDirectChild(parent, held, named walPublicationFact) error {
	if !parent.complete() || !held.complete() || !named.complete() {
		return ErrUnsupported
	}
	if parent.mode&walPublicationTypeMask != walPublicationDirectory || held.mode&walPublicationTypeMask != walPublicationRegular ||
		named.mode&walPublicationTypeMask != walPublicationRegular || held.links != 1 || named.links != 1 {
		return ErrUnsupported
	}
	if !held.samePhysical(named) || held.mount != named.mount || held.mount != parent.mount ||
		held.devMajor != parent.devMajor || held.devMinor != parent.devMinor {
		return ErrIdentity
	}
	return nil
}
func walPublicationDistinct(main, original, candidate, shm walPublicationFact, hasSHM bool) error {
	if main.samePhysical(original) || main.samePhysical(candidate) || original.samePhysical(candidate) ||
		hasSHM && (shm.samePhysical(main) || shm.samePhysical(original) || shm.samePhysical(candidate)) { return ErrIdentity }
	return nil
}
func walPublicationBasename(name string) bool {
	if name == "" || name == "." || name == ".." || len(name) > 200 { return false }
	for i := range name {
		b := name[i]
		if !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '.' || b == '_' || b == '-') { return false }
	}
	n := strings.ToLower(name)
	return !strings.HasSuffix(n, "-journal") && !strings.HasSuffix(n, "-wal") && !strings.HasSuffix(n, "-shm")
}

// No Unwrap: generic parent adopters must not detach an interior File from its
// retained directory. The safe tree has no cleanup owners or custom traversal.
type walPublicationDiagnostic struct{ safe error }
func (e *walPublicationDiagnostic) Error() string {
	if e == nil || e.safe == nil { return ErrPoisoned.Error() }
	return e.safe.Error()
}
func (e *walPublicationDiagnostic) Is(target error) bool { return errors.Is(e.safe, target) }
func (e *walPublicationDiagnostic) As(target any) bool { return errors.As(e.safe, target) }

// Sanitize once under a fixed budget. Unknown/foreign owners and custom error
// hooks become terminal barriers; the composite retains their original graph
// opaquely. Unknown hooks are rejected before any generic Unwrap; they must
// never run, even when a wrapper offers a seemingly safe child. A bad branch
// cannot erase its independently sanitized siblings. Empty wrappers fail closed.
// Safe errno and standard OS error Is/As causes remain available.
func walPublicationError(input error, allowed []*File) (error, bool) {
	if input == nil { return nil, false }
	count, bad := 0, false
	var clean func(error) error
	clean = func(err error) error {
		count++
		if count > 128 { bad = true; return ErrCleanupLimit }
		if err == nil { bad = true; return ErrPoisoned }
		switch e := err.(type) {
		case *cleanupError:
			if e == nil { bad = true; return ErrPoisoned }
			known := false
			for _, f := range allowed { known = known || e.file != nil && e.file == f }
			if !known { bad = true; return ErrPoisoned }
			return clean(e.cause)
		case CleanupOwner:
			bad = true
			return ErrPoisoned
		case *os.PathError:
			if e == nil { bad = true; return ErrPoisoned }
			return &os.PathError{Op: e.Op, Path: e.Path, Err: clean(e.Err)}
		case *os.LinkError:
			if e == nil { bad = true; return ErrPoisoned }
			return &os.LinkError{Op: e.Op, Old: e.Old, New: e.New, Err: clean(e.Err)}
		case *os.SyscallError:
			if e == nil { bad = true; return ErrPoisoned }
			return &os.SyscallError{Syscall: e.Syscall, Err: clean(e.Err)}
		case syscall.Errno:
			return e
		case interface{ As(any) bool }:
			bad = true
			return ErrPoisoned
		case interface{ Is(error) bool }:
			bad = true
			return ErrPoisoned
		case interface{ Unwrap() []error }:
			children := e.Unwrap()
			if len(children) == 0 { bad = true; return ErrPoisoned }
			if len(children) > 128-count { bad = true; return ErrCleanupLimit }
			var safe []error
			for _, child := range children { safe = append(safe, clean(child)) }
			return errors.Join(safe...)
		case interface{ Unwrap() error }:
			return clean(e.Unwrap())
		default:
			return err
		}
	}
	safe := clean(input)
	if safe == nil { bad = true; safe = ErrPoisoned }
	if bad { safe = errors.Join(ErrPoisoned, safe) }
	return &walPublicationDiagnostic{safe: safe}, bad
}
