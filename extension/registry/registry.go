// Package registry provides immutable, per-connection extension index method
// registries. It does not load code from persisted metadata or own transactions.
// The engine supplies the storage and transaction boundary, and invokes the
// registered implementation to derive index keys from an already-coerced value.
package registry

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

var (
	ErrMethod    = errors.New("registry: invalid index method")
	ErrDuplicate = errors.New("registry: duplicate index method")
)

// Key is one logical posting key. Namespace identifies comparison semantics;
// Length is the method's logical token length, not its encoded byte length.
// Method/config identity, source field and row identity are supplied by storage.
type Key struct {
	Namespace string
	Length    int
	Text      string
}

// IndexMethod is an immutable, concurrently safe implementation registered by
// the application. Name and Version describe its persisted semantic identity.
// Name is a canonical lowercase ASCII identifier of at most 64 bytes and
// Version is positive. Distinct versions may coexist under the same name.
//
// Configure must reject unsupported, invalid or noncanonical metadata. On
// success it returns a nonnil immutable Index which does not retain mutable
// aliases to canonical. A registry snapshots descriptors and its own map, but
// cannot clone arbitrary Go implementations: callers must not mutate methods.
type IndexMethod interface {
	Name() string
	Version() int
	Configure(canonical []byte) (Index, error)
}

// Index derives bounded logical keys without reading or committing storage.
// It must be immutable and safe for concurrent calls. Canonical returns a fresh
// copy of the exact accepted configuration. Keys returns distinct keys ordered
// by Namespace, Length, then UTF-8 Text bytes, in a caller-owned slice. It must
// honor cancellation and return no partial keys on error. SQL conversion and
// exact predicate rechecking remain the engine's responsibility.
type Index interface {
	Canonical() []byte
	Keys(context.Context, string) ([]Key, error)
}

type identity struct {
	name    string
	version int
}

// registeredMethod keeps the registered descriptor stable even if the caller
// violates the method immutability contract. Configure still invokes the real
// implementation; descriptor names never select a hard-coded implementation.
type registeredMethod struct {
	identity
	implementation IndexMethod
}

func (m registeredMethod) Name() string { return m.name }
func (m registeredMethod) Version() int { return m.version }
func (m registeredMethod) Configure(canonical []byte) (Index, error) {
	return m.implementation.Configure(canonical)
}

// Registry is immutable after New. Its zero value and a nil *Registry behave as
// empty registries. There is deliberately no global registry or Register method.
type Registry struct {
	methods map[identity]registeredMethod
}

// New validates every descriptor and rejects duplicate (name, version) pairs.
// No caller-owned slice or map is retained. Methods themselves must obey the
// immutable implementation contract on IndexMethod.
func New(methods ...IndexMethod) (*Registry, error) {
	r := &Registry{methods: make(map[identity]registeredMethod, len(methods))}
	for _, method := range methods {
		if nilMethod(method) {
			return nil, fmt.Errorf("%w: nil implementation", ErrMethod)
		}
		id := identity{method.Name(), method.Version()}
		if !validName(id.name) || id.version <= 0 {
			return nil, fmt.Errorf("%w: descriptor %q version %d", ErrMethod, id.name, id.version)
		}
		if _, exists := r.methods[id]; exists {
			return nil, fmt.Errorf("%w: %s version %d", ErrDuplicate, id.name, id.version)
		}
		r.methods[id] = registeredMethod{id, method}
	}
	return r, nil
}

func nilMethod(method IndexMethod) bool {
	if method == nil {
		return true
	}
	// An interface can contain a typed nil pointer (or another nil-able type
	// with methods). Reject it before invoking any descriptor methods.
	v := reflect.ValueOf(method)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func validName(name string) bool {
	if len(name) == 0 || len(name) > 64 || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for i := 1; i < len(name); i++ {
		c := name[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

// Lookup resolves an exact registered name/version. It does not normalize names
// or substitute a different version when persisted metadata is unsupported.
func (r *Registry) Lookup(name string, version int) (IndexMethod, bool) {
	if r == nil {
		return nil, false
	}
	method, ok := r.methods[identity{name, version}]
	if !ok {
		return nil, false
	}
	return method, true
}

// Snapshot returns an independent immutable map suitable for a connection.
// Immutable implementation references are intentionally shared. It returns an
// empty nonnil registry for a nil receiver.
func (r *Registry) Snapshot() *Registry {
	out := &Registry{methods: make(map[identity]registeredMethod)}
	if r != nil {
		for id, method := range r.methods {
			out.methods[id] = method
		}
	}
	return out
}
