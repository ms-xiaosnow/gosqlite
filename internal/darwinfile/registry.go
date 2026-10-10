// Package darwinfile coordinates physical descriptor retirement across the
// plain and encrypted Darwin backends. It contains no database lock protocol.
// Its state laws are portable so they can be tested without native failures.
package darwinfile

import (
	"errors"
	"fmt"
	"sync"
)

const (
	MaxPerInode    = 256
	MaxDescriptors = 16384
)

var (
	ErrPoisoned = errors.New("Darwin descriptor ownership is uncertain")
	ErrLimit    = errors.New("Darwin managed descriptor limit")
)

type Identity struct {
	Device int32
	Inode  uint64
}

type inode struct {
	records map[*Record]struct{}
	locks   map[string]bool
	poison  error
}

// Record retains the physical close function and therefore the actual native
// resource. Retired logical owners may be reused only before a close attempt.
// Neither callbacks nor the retained resource may escape to application code.
type Record struct {
	id                    Identity
	close                 func() error
	after                 func(error) error
	idle, attempted, done bool
	err                   error
}

// Registry's mutex is the common lifecycle gate. Every method requires it held,
// including admission, registration, successful lock-ledger changes and drains.
// Close hooks may update only gate-owned state; they must not acquire a File
// mutex or invoke user code. I/O owners acquire their own mutex before this gate.
type Registry struct {
	sync.Mutex
	nodes    map[Identity]*inode
	records  map[*Record]struct{}
	unknown  []*Record
	overflow int
}

var Process = New()

func New() *Registry {
	return &Registry{nodes: make(map[Identity]*inode), records: make(map[*Record]struct{})}
}
func (r *Registry) node(id Identity) *inode {
	n := r.nodes[id]
	if n == nil {
		n = &inode{records: make(map[*Record]struct{}), locks: make(map[string]bool)}
		r.nodes[id] = n
	}
	return n
}
func (r *Registry) Error(id Identity) error {
	if n := r.nodes[id]; n != nil {
		return n.poison
	}
	return nil
}
func (r *Registry) Poison(id Identity, err error) error {
	n := r.node(id)
	// Retain only the first failure. Repeated represented-lock retirement may
	// fail indefinitely; accumulating every retry would make retained state
	// unbounded. Report a later operation's own cause without storing that join.
	if n.poison == nil {
		n.poison = errors.Join(ErrPoisoned, err)
		return n.poison
	}
	if err == nil {
		return n.poison
	}
	return errors.Join(n.poison, err)
}

// Admit uses a descriptor-free pathname hint when available. Register MUST still
// precede validation: a raced open may discover an already full or poisoned inode.
func (r *Registry) Admit(hint *Identity) error {
	if len(r.unknown) != 0 {
		return ErrPoisoned
	}
	if len(r.records)+len(r.unknown) >= MaxDescriptors {
		return ErrLimit
	}
	if r.overflow != 0 {
		return ErrLimit
	}
	if hint != nil {
		if err := r.Error(*hint); err != nil {
			return err
		}
		if n := r.nodes[*hint]; n != nil && len(n.records) >= MaxPerInode {
			return ErrLimit
		}
	}
	return nil
}
func (r *Registry) Register(id Identity, close func() error, after func(error) error) (*Record, error) {
	rec := &Record{id: id, close: close, after: after}
	n := r.node(id)
	n.records[rec] = struct{}{}
	if len(n.records) == MaxPerInode+1 {
		r.overflow++
	}
	r.records[rec] = struct{}{}
	if n.poison != nil {
		return rec, n.poison
	}
	if len(n.records) > MaxPerInode || len(r.records) > MaxDescriptors {
		return rec, ErrLimit
	}
	return rec, nil
}
func (r *Registry) Check(rec *Record) error {
	if rec == nil {
		return ErrPoisoned
	}
	if rec.err != nil {
		return rec.err
	}
	if rec.idle || rec.attempted || rec.done {
		return ErrPoisoned
	}
	return r.Error(rec.id)
}
func (r *Registry) Reuse(rec *Record) bool {
	if rec == nil || !rec.idle || rec.attempted || rec.done || r.Error(rec.id) != nil {
		return false
	}
	rec.idle = false
	return true
}
func (r *Registry) SetLocks(id Identity, domain string, held bool) {
	n := r.node(id)
	if held {
		n.locks[domain] = true
	} else {
		delete(n.locks, domain)
	}
	r.dropEmpty(id, n)
}
func (r *Registry) Locked(id Identity) bool { n := r.nodes[id]; return n != nil && len(n.locks) != 0 }
func (r *Registry) anyLocked() bool {
	for _, n := range r.nodes {
		if len(n.locks) != 0 {
			return true
		}
	}
	return false
}
func (r *Registry) dropEmpty(id Identity, n *inode) {
	if len(n.records) == 0 && len(n.locks) == 0 {
		delete(r.nodes, id)
	}
}

// Retire marks only logical closure. Physical closure waits for every domain's
// process locks, including locks acquired through another descriptor.
func (r *Registry) Retire(rec *Record) error {
	if rec == nil {
		return ErrPoisoned
	}
	if rec.attempted || rec.done {
		return rec.err
	}
	rec.idle = true
	return r.Drain(rec.id)
}
func (r *Registry) Drain(id Identity) error {
	var errs []error
	n := r.nodes[id]
	if n != nil && len(n.locks) == 0 {
		// Hooks may retire a pinned parent. Snapshot eligible records before calling
		// them, and mark attempted before a native call to prevent reentrant retry.
		eligible := make([]*Record, 0, len(n.records))
		for rec := range n.records {
			if rec.idle && !rec.attempted {
				eligible = append(eligible, rec)
			}
		}
		for _, rec := range eligible {
			if rec.attempted {
				continue
			}
			rec.attempted = true
			if err := rec.close(); err != nil {
				rec.err = r.Poison(id, err)
				errs = append(errs, rec.err)
			} else {
				rec.done = true
				if len(n.records) == MaxPerInode+1 {
					r.overflow--
				}
				delete(n.records, rec)
				delete(r.records, rec)
			}
			if err := runAfter(rec); err != nil {
				// A same-inode parent hook may have drained the previous node.
				n = r.node(id)
				rec.err = r.Poison(id, errors.Join(rec.err, err))
				// Native retirement already consumed the descriptor. Retain a bounded
				// tombstone for uncertain pin bookkeeping, never a retryable raw number.
				if _, exists := n.records[rec]; !exists {
					n.records[rec] = struct{}{}
					r.records[rec] = struct{}{}
					if len(n.records) == MaxPerInode+1 {
						r.overflow++
					}
				}
				errs = append(errs, rec.err)
			}
		}
		r.dropEmpty(id, n)
	}
	if err := r.DrainUnknown(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// Quarantine receives a raw descriptor with no os.File/finalizer when fstat
// could not establish its inode. Further physical opens stop until it retires.
func (r *Registry) Quarantine(close func() error, after func(error) error) error {
	rec := &Record{close: close, after: after, idle: true}
	r.unknown = append(r.unknown, rec)
	return r.DrainUnknown()
}
func (r *Registry) DrainUnknown() error {
	if len(r.unknown) == 0 {
		return nil
	}
	if r.anyLocked() {
		return nil
	}
	var keep []*Record
	var errs []error
	for _, rec := range r.unknown {
		if !rec.attempted {
			rec.attempted = true
			if err := rec.close(); err != nil {
				rec.err = errors.Join(ErrPoisoned, err)
			} else {
				rec.done = true
			}
			if err := runAfter(rec); err != nil {
				rec.err = errors.Join(ErrPoisoned, rec.err, err)
				rec.done = false
				errs = append(errs, err)
			}
		}
		if !rec.done {
			keep = append(keep, rec)
			errs = append(errs, rec.err)
		}
	}
	r.unknown = keep
	return errors.Join(errs...)
}

// Hooks are private gate-owned bookkeeping. A programming panic cannot make a
// native close retryable or skip the remaining independent retirements.
func runAfter(rec *Record) (err error) {
	if rec.after == nil {
		return nil
	}
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%w: descriptor retirement hook: %v", ErrPoisoned, p)
		}
	}()
	return rec.after(rec.err)
}
