package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
	"github.com/ms-xiaosnow/gosqlite/internal/walindex"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Only transparent standard wrappers and the public SQLite diagnostic envelope
// are understood. Arbitrary Error/Is/As/Unwrap methods are never executed.
var (
	terminalErrorType = reflect.TypeOf(errors.New("terminal diagnostic"))
	singleWrapType    = reflect.TypeOf(fmt.Errorf("context: %w", context.Canceled))
	multiWrapType     = reflect.TypeOf(fmt.Errorf("%w: %w", context.Canceled, context.DeadlineExceeded))
	joinType          = reflect.TypeOf(errors.Join(context.Canceled, context.DeadlineExceeded))
)

const maxOutcomeErrorNodes = 128
const (
	busyLeaf uint8 = 1 << iota
	canceledLeaf
)

type outcome uint8

const (
	outcomeSuccess outcome = iota
	outcomeBusy
	outcomeCanceled
	outcomeFailed
)

// errorOutcome discovers the whole accepted error graph in one bounded pass.
// Every branch must terminate in exact known sentinels. Unknown diagnostics,
// cleanup markers, unsafe SQLite codes, cycles and excessive graphs fail closed.
// Known BUSY plus cancellation is canceled, so commit cannot retry it.
func errorOutcome(err error) outcome {
	if err == nil {
		return outcomeSuccess
	}
	remaining := maxOutcomeErrorNodes
	var visit func(error) uint8
	visit = func(node error) uint8 {
		if remaining == 0 || node == nil {
			return 0
		}
		remaining--
		switch node {
		case sqlite.ErrBusy, vfs.ErrBusy, encstore.ErrBusy, walindex.ErrBusy:
			return busyLeaf
		case context.Canceled, context.DeadlineExceeded:
			return canceledLeaf
		}
		if e, ok := node.(*sqlite.Error); ok {
			if e == nil || (e.Code != sqlite.CodeBusy && e.Code != sqlite.CodeInterrupt) {
				return 0
			}
			leaves := visit(e.Err)
			if e.Code == sqlite.CodeBusy && leaves&busyLeaf == 0 {
				return 0
			}
			if e.Code == sqlite.CodeInterrupt && leaves != canceledLeaf {
				return 0
			}
			return leaves
		}
		typ := reflect.TypeOf(node)
		if typ != singleWrapType && typ != multiWrapType && typ != joinType {
			return 0
		}
		if reflect.ValueOf(node).IsNil() {
			return 0
		}
		if typ == singleWrapType {
			return visit(node.(interface{ Unwrap() error }).Unwrap())
		}
		children := node.(interface{ Unwrap() []error }).Unwrap()
		if len(children) == 0 || len(children) > remaining {
			return 0
		}
		var leaves uint8
		for _, child := range children {
			part := visit(child)
			if part == 0 {
				return 0
			}
			leaves |= part
		}
		return leaves
	}
	switch leaves := visit(err); {
	case leaves == busyLeaf:
		return outcomeBusy
	case leaves&canceledLeaf != 0:
		return outcomeCanceled
	default:
		return outcomeFailed
	}
}
func ordinaryBusy(err error) bool { return errorOutcome(err) == outcomeBusy }
func classify(c *Counts, err error) bool {
	switch errorOutcome(err) {
	case outcomeSuccess:
		c.Success++
	case outcomeBusy:
		c.Busy++
	case outcomeCanceled:
		c.Canceled++
	default:
		c.Failed++
		return true
	}
	return false
}

// Only a wholly understood unsupported graph may be labeled skipped. An
// unrelated joined diagnostic is a failure, not a platform skip.
func benignUnsupported(err error) bool {
	remaining := maxOutcomeErrorNodes
	var visit func(error) bool
	visit = func(node error) bool {
		if node == nil || remaining == 0 {
			return false
		}
		remaining--
		switch node {
		case sqlite.ErrUnsupported, pager.ErrUnsupported, vfs.ErrUnsupported, walindex.ErrUnsupported, encstore.ErrUnsupported, encryption.ErrUnsupported, encryption.ErrUnsupportedConfiguration:
			return true
		}
		if e, ok := node.(*sqlite.Error); ok {
			return e != nil && e.Code == sqlite.CodeError && visit(e.Err)
		}
		typ := reflect.TypeOf(node)
		if typ != singleWrapType && typ != multiWrapType && typ != joinType {
			return false
		}
		if reflect.ValueOf(node).IsNil() {
			return false
		}
		if typ == singleWrapType {
			return visit(node.(interface{ Unwrap() error }).Unwrap())
		}
		children := node.(interface{ Unwrap() []error }).Unwrap()
		if len(children) == 0 || len(children) > remaining {
			return false
		}
		for _, child := range children {
			if !visit(child) {
				return false
			}
		}
		return true
	}
	return visit(err)
}
