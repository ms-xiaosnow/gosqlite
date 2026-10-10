package vfs

import "errors"

// CleanupOwner retains only a managed owner, never a raw native handle. Values
// are obtained from CleanupOwners after an unsuccessful nested acquisition or
// retirement. Close obeys that managed owner's represented lifetime; a terminal
// obligation has no callable raw close and always reports its original cause.
type CleanupOwner interface{ Close() error }

type cleanupError struct {
	cause error
	file  *File // nil means explicitly unresolved, without a safe retry route
}

func (e *cleanupError) Error() string { return e.cause.Error() }
func (e *cleanupError) Unwrap() error { return e.cause }
func (e *cleanupError) Close() error {
	if e.file == nil {
		return e.cause
	}
	return e.file.Close()
}

// CleanupComplete reports explicit retirement for error-carried owners. Unknown
// implementations fail closed; a diagnostic error is never ownership evidence.
func CleanupComplete(owner CleanupOwner) bool {
	if o, ok := owner.(interface{ RetirementComplete() bool }); ok {
		return o.RetirementComplete()
	}
	return false
}
func (e *cleanupError) RetirementComplete() bool {
	return e.file != nil && e.file.RetirementComplete()
}

func retainedFileCleanup(f *File, cause error) error {
	return &cleanupError{file: f, cause: cause}
}
func terminalCleanup(cause error) *cleanupError { return &cleanupError{cause: cause} }

// A backend can reject nativeOpen after retaining a resource before a common
// File exists (for example quarantined Darwin descriptors). Without a returned
// managed owner, this layer must not invent a safe cleanup call or forget the
// obligation. Preserve any more precise lower-layer obligation already present.
func unmanagedOpenError(err error) error {
	if errors.Is(err, ErrPoisoned) && len(CleanupOwners(err)) == 0 {
		return terminalCleanup(err)
	}
	return err
}

// CleanupOwners finds every retained managed/terminal obligation in a wrapped
// or joined error. It deduplicates the same File and terminal marker, and places
// nested obligations first. Collecting owners never retries them or acquires a
// native lock. Callers must retain protecting parent resources until all of the
// returned owners have conclusively retired. The input is an internal error tree,
// not a traversal of the process-wide owner registry.
func CleanupOwners(err error) []CleanupOwner {
	type identity struct {
		file     *File
		terminal *cleanupError
	}
	type node struct {
		owner CleanupOwner
		deps  []*node
		state uint8
	}
	nodes := make(map[identity]*node)
	seen := make(map[*cleanupError]bool)
	var order []*node
	var collect func(error) []*node
	collect = func(err error) []*node {
		if err == nil {
			return nil
		}
		switch e := err.(type) {
		case *cleanupError:
			id := identity{file: e.file}
			if e.file == nil {
				id.terminal = e
			}
			n := nodes[id]
			if n == nil {
				n = &node{owner: e}
				nodes[id] = n
				order = append(order, n)
			}
			if !seen[e] {
				seen[e] = true
				for _, child := range collect(e.cause) {
					if child != n {
						n.deps = append(n.deps, child)
					}
				}
			}
			return []*node{n}
		case interface{ Unwrap() []error }:
			var children []*node
			for _, child := range e.Unwrap() {
				children = append(children, collect(child)...)
			}
			return children
		case interface{ Unwrap() error }:
			return collect(e.Unwrap())
		}
		return nil
	}
	collect(err)
	var out []CleanupOwner
	var emit func(*node) bool
	emit = func(n *node) bool {
		if n.state == 1 {
			return false
		}
		if n.state == 2 {
			return true
		}
		n.state = 1
		for _, child := range n.deps {
			if !emit(child) {
				return false
			}
		}
		n.state = 2
		out = append(out, n.owner)
		return true
	}
	for _, n := range order {
		if !emit(n) {
			// An inconsistent dependency graph cannot justify any retirement.
			// Preserve all concrete owners behind an explicit terminal barrier.
			out = []CleanupOwner{terminalCleanup(errors.Join(ErrPoisoned, err, errors.New("native cleanup ownership dependency cycle")))}
			for _, n := range order {
				out = append(out, n.owner)
			}
			return out
		}
	}
	return out
}
