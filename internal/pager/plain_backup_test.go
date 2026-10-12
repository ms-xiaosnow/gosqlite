package pager

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"testing"
)

// All negative cases in this file operate only on memory or source ASTs.
// AST cases read source files; no database descriptor, native failure hook,
// failing OS operation or recovery path is exercised.
func TestPlainBackupMemoryHeaderGeometry(t *testing.T) {
	for _, size := range []uint32{512, 4096, 65536} {
		p := emptyDatabasePage(size)
		h, err := ParseHeader(p, int64(size))
		if err != nil || validateBackupHeader(h, p, int64(size)) != nil {
			t.Fatal(h, err)
		}
		before := bytes.Clone(p)
		if err := validateBackupHeader(h, p, int64(size)); err != nil || !bytes.Equal(p, before) {
			t.Fatal("header validation modified source", err)
		}
	}
	for _, tc := range []struct {
		name string
		edit func([]byte)
		size int64
		want error
	}{
		{"wal", func(p []byte) { p[18], p[19] = 2, 2 }, 512, ErrUnsupported},
		{"reserved", func(p []byte) { p[20] = 1 }, 512, ErrUnsupported},
		{"legacy-schema", func(p []byte) { binary.BigEndian.PutUint32(p[44:], 3) }, 512, ErrUnsupported},
		{"auto-vacuum", func(p []byte) { binary.BigEndian.PutUint32(p[52:], 1) }, 512, ErrUnsupported},
		{"encoding", func(p []byte) { binary.BigEndian.PutUint32(p[56:], 2) }, 512, ErrUnsupported},
		{"trailing-pages", func(p []byte) {}, 1024, ErrUnsupported},
		{"unaligned", func(p []byte) {}, 513, ErrCorrupt},
		{"excess-count", func(p []byte) { binary.BigEndian.PutUint32(p[28:], 2) }, 512, ErrCorrupt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := emptyDatabasePage(512)
			tc.edit(p)
			h, _ := ParseHeader(p, tc.size)
			if err := validateBackupHeader(h, p, tc.size); !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
		})
	}
	// A stale count uses the intact physical extent, preserving all raw fields.
	p := emptyDatabasePage(512)
	binary.BigEndian.PutUint32(p[24:], 7)
	binary.BigEndian.PutUint32(p[92:], 6)
	binary.BigEndian.PutUint32(p[28:], 99)
	h, err := ParseHeader(p, 1024)
	before := bytes.Clone(p)
	if err != nil || h.PageCount != 2 || validateBackupHeader(h, p, 1024) != nil || !bytes.Equal(before, p) {
		t.Fatal("stale header was truncated, normalized or refused", h, err)
	}
	h.PageCount++
	if err := validateBackupHeader(h, p, 1024); !errors.Is(err, ErrChanged) {
		t.Fatal(err)
	}
}

type backupMemorySource struct {
	header   Header
	pages    [][]byte
	readErr  error
	calls    int
	returned [][]byte
}

func (s *backupMemorySource) Header() Header { return s.header }
func (s *backupMemorySource) ReadPage(_ context.Context, n uint32) ([]byte, error) {
	s.calls++
	if s.readErr != nil {
		return nil, s.readErr
	}
	p := bytes.Clone(s.pages[n-1])
	s.returned = append(s.returned, p)
	return p, nil
}
func TestPlainBackupMemoryPageCopy(t *testing.T) {
	p, q := bytes.Repeat([]byte{3}, 512), bytes.Repeat([]byte{7}, 512)
	h := Header{PageSize: 512, PageCount: 2}
	s := &backupMemorySource{header: h, pages: [][]byte{p, q}}
	var output []byte
	err := copyBackupPages(context.Background(), h, s, func(p []byte, offset int64) error {
		if offset != int64(len(output)) {
			t.Fatal("wrong page offset")
		}
		output = append(output, p...)
		return nil
	})
	if err != nil || !bytes.Equal(output, append(bytes.Clone(p), q...)) || s.calls != 2 {
		t.Fatal(err)
	}
	for _, p := range s.returned {
		if !bytes.Equal(p, make([]byte, 512)) {
			t.Fatal("copy page retained after write")
		}
	}
	cause := errors.New("memory source error")
	for _, mode := range []string{"read", "page-length", "write", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			s := &backupMemorySource{header: h, pages: [][]byte{p, q}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := cause
			if mode == "read" {
				s.readErr = cause
			}
			if mode == "page-length" {
				s.pages = [][]byte{p[:511]}
				want = io.ErrUnexpectedEOF
			}
			if mode == "write" {
				want = io.ErrShortWrite
			}
			if mode == "cancel" {
				want = context.Canceled
			}
			writes := 0
			err := copyBackupPages(ctx, h, s, func([]byte, int64) error {
				writes++
				if mode == "cancel" {
					cancel()
				}
				if mode == "write" {
					return io.ErrShortWrite
				}
				return nil
			})
			if !errors.Is(err, want) || s.calls != 1 || writes > 1 {
				t.Fatal(err, s.calls, writes)
			}
		})
	}
}

type backupRetirementModel struct {
	name             string
	trace            *[]string
	complete, finish bool
	err              error
	calls            int
}

func (m *backupRetirementModel) RetirementComplete() bool { return m.complete }
func (m *backupRetirementModel) Close() error {
	m.calls++
	*m.trace = append(*m.trace, m.name)
	m.complete = m.finish
	return m.err
}
func TestPlainBackupMemoryIndependentErrorOwnership(t *testing.T) {
	// Source and destination have independent protectors. A nested/wrapped
	// source token must remain discoverable without being adopted or retired by
	// destination cleanup. Explicit caller retry alone may revisit that owner.
	registry := plainRetirementRegistry{limit: 3}
	var trace []string
	src, _ := registry.reserve()
	source := &backupRetirementModel{name: "source", trace: &trace, err: errors.New("source-retirement")}
	src.main = source
	sourceErr := src.begin(errors.New("source-operation"))
	dst, _ := registry.reserve()
	main := &backupRetirementModel{name: "main", trace: &trace, finish: true}
	directory := &backupRetirementModel{name: "directory", trace: &trace, finish: true, err: errors.New("consumed-directory-close")}
	dst.main, dst.temporary = main, directory
	wrapped := fmt.Errorf("source read: %w", errors.Join(sourceErr, io.ErrUnexpectedEOF))
	result := finishPlaintextBackup(wrapped, dst.begin(nil), true)
	for _, target := range []error{ErrCommitUncertain, ErrPlaintextCleanupPending, ErrPoisoned, source.err, directory.err, io.ErrUnexpectedEOF} {
		if !errors.Is(result, target) {
			t.Fatal("lost composed cause", target, result)
		}
	}
	if source.calls != 1 || main.calls != 1 || directory.calls != 1 || fmt.Sprint(trace) != "[source directory main]" || !dst.done {
		t.Fatal(trace)
	}
	source.finish, source.err = true, nil
	if err := RetryPlaintextCleanup(fmt.Errorf("public: %w", errors.Join(result, result))); err != nil || source.calls != 2 || main.calls != 1 {
		t.Fatal(err, trace)
	}
	if !errors.Is(result, ErrPlaintextCleanupPending) {
		t.Fatal("historical result changed")
	}
	if len(registry.groups) != 0 {
		t.Fatal("completed independent owners retained")
	}
}
func TestPlainBackupMemoryDirectoryProtectsMain(t *testing.T) {
	registry := plainRetirementRegistry{limit: 1}
	g, _ := registry.reserve()
	var trace []string
	main := &backupRetirementModel{name: "main", trace: &trace, finish: true}
	directory := &backupRetirementModel{name: "directory", trace: &trace}
	g.main, g.temporary = main, directory
	err := finishPlaintextBackup(nil, g.begin(nil), true)
	if !errors.Is(err, ErrPlaintextCleanupPending) || !errors.Is(err, ErrCommitUncertain) || main.calls != 0 || directory.calls != 1 {
		t.Fatal(err, trace)
	}
	directory.finish = true
	if err = RetryPlaintextCleanup(err); err != nil || fmt.Sprint(trace) != "[directory directory main]" || len(registry.groups) != 0 {
		t.Fatal(err, trace)
	}
	if err = finishPlaintextBackup(io.EOF, nil, false); !errors.Is(err, io.EOF) || errors.Is(err, ErrCommitUncertain) {
		t.Fatal(err)
	}
	if finishPlaintextBackup(nil, nil, true) != nil {
		t.Fatal("success became uncertain")
	}
}
func TestPlainBackupSourceOwnershipBoundary(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "plain_backup.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	begins, sourceValidation, sourceReads := 0, 0, 0
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name == "begin" {
			begins++
			id, ok := call.Args[0].(*ast.Ident)
			if !ok || id.Name != "nil" {
				t.Fatal("destination cleanup adopted cross-owner operation error")
			}
		}
		if sel.Sel.Name == "ValidateRollbackBackupSource" {
			sourceValidation++
		}
		if sel.Sel.Name == "ReadPage" {
			sourceReads++
		}
		return true
	})
	if begins != 1 || sourceValidation != 3 || sourceReads != 1 {
		t.Fatal(begins, sourceValidation, sourceReads)
	}
}
