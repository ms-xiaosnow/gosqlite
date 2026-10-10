package vfs

import "errors"

// ErrCleanupLimit means an error graph could not be safely discovered in the
// caller's budget. The refusal never wraps that possibly cyclic input.
var ErrCleanupLimit = errors.New("native cleanup error discovery limit exceeded")

// CleanupOwnersBounded is a single-pass, bounded form of CleanupOwners for
// scopes that can receive caller-provided error implementations. It invokes
// each encountered wrapper's Unwrap only during this traversal; a separate
// validation pass would not constrain a stateful wrapper's second result.
// On refusal, no ownership operation occurs and the caller must retain input
// opaquely with all its protectors. Allocations inside Unwrap remain caller code.
func CleanupOwnersBounded(input error, limit int) ([]CleanupOwner, error) {
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
	count := 0
	var refusal error
	var collect func(error) []*node
	collect = func(err error) []*node {
		if refusal != nil {
			return nil
		}
		if count >= limit {
			refusal = ErrCleanupLimit
			return nil
		}
		count++
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
			children := e.Unwrap()
			if len(children) > limit-count {
				refusal = ErrCleanupLimit
				return nil
			}
			var out []*node
			for _, child := range children {
				out = append(out, collect(child)...)
				if refusal != nil {
					return nil
				}
			}
			return out
		case interface{ Unwrap() error }:
			return collect(e.Unwrap())
		}
		return nil
	}
	collect(input)
	if refusal != nil {
		return nil, refusal
	}
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
			out = []CleanupOwner{terminalCleanup(errors.Join(ErrPoisoned, input, errors.New("native cleanup ownership dependency cycle")))}
			for _, n := range order {
				out = append(out, n.owner)
			}
			return out, nil
		}
	}
	return out, nil
}
