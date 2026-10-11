package pager

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/wal"
)

// These are byte-only policies and AST assertions. They never call OpenSession,
// native recovery, replacement, truncation, checkpoint or any injected I/O.
func TestWALTailAdmissionMemoryUsesExactProvenance(t *testing.T) {
	h := wal.Header{Magic: wal.MagicLittleEndian, Version: wal.Version, PageSize: 512}
	raw, err := wal.EncodeHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	h, err = wal.ParseHeader(raw)
	if err != nil {
		t.Fatal(err)
	}
	frame, sum, err := wal.EncodeFrame(h, 1, 1, bytes.Repeat([]byte{1}, 512), h.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	pending, _, err := wal.EncodeFrame(h, 1, 0, bytes.Repeat([]byte{2}, 512), sum)
	if err != nil {
		t.Fatal(err)
	}
	full := append(append(raw, frame...), pending...)
	prefix, scanErr := wal.ScanPrefix(context.Background(), bytes.NewReader(full), int64(len(full)), wal.Limits{})
	if !exclusiveWALPrefixAllowed(prefix, scanErr) || !sharedWALPrefixAllowed(prefix, scanErr) {
		t.Fatal("valid prefix rejected", scanErr)
	}
	for _, e := range []error{nil, wal.ErrTail, errors.Join(scanErr), errors.Join(scanErr, io.ErrClosedPipe), errors.Join(scanErr, wal.ErrUnsupported), errors.Join(scanErr, ErrPoisoned)} {
		if exclusiveWALPrefixAllowed(prefix, e) || sharedWALPrefixAllowed(prefix, e) {
			t.Fatal("joined or substituted error admitted", e)
		}
	}
	for _, b := range [][]byte{full[:len(full)-1], append(append([]byte(nil), full...), 0)} {
		p, e := wal.ScanPrefix(context.Background(), bytes.NewReader(b), int64(len(b)), wal.Limits{})
		if exclusiveWALPrefixAllowed(p, e) || sharedWALPrefixAllowed(p, e) {
			t.Fatal("partial suffix admitted", e)
		}
	}
	if exclusiveWALPrefixAllowed(wal.CommittedPrefix{}, scanErr) || sharedWALPrefixAllowed(wal.CommittedPrefix{}, nil) {
		t.Fatal("fabricated prefix admitted")
	}
}

func TestWALTailAdmissionSourceRemainsBeforeMutation(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "wal.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var open *ast.FuncDecl
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "openWALLocked" {
			open = fn
		}
	}
	if open == nil {
		t.Fatal("missing exclusive scanner")
	}
	found := false
	ast.Inspect(open, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == "errWALTailPublicationUnavailable" {
			found = true
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				for _, mutation := range []string{"WriteAt", "WriteCandidate", "Truncate", "Remove", "Rename", "ReplaceWAL", "CreateWALCandidate", "normalizeAppendWALLocked"} {
					if sel.Sel.Name == mutation {
						t.Fatal("exclusive tail admission gained mutation without capability", mutation)
					}
				}
			}
		}
		return true
	})
	if !found {
		t.Fatal("missing explicit native publication capability gate")
	}
	f, err = parser.ParseFile(token.NewFileSet(), "shared_index.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "sharedWALPrefixAllowed" {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && (sel.Sel.Name == "Is" || sel.Sel.Name == "As") {
				t.Fatal("tail whitelist returned")
			}
			return true
		})
	}
	if !strings.Contains(errWALTailPublicationUnavailable.Error(), "retained-directory") {
		t.Fatal("capability gap obscured")
	}
}
