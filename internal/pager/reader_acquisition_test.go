package pager

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// This checks constructor source only, without opening a database or injecting
// scheduling/native-operation hooks. The ordinary workload below supplies live
// cooperating-reader/writer evidence, not deterministic race reproduction.
func TestReaderAcquisitionSourceSnapshotAfterShared(t *testing.T) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "pager.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var body []ast.Stmt
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "OpenAdmitted" {
			body = fn.Body.List
		}
	}
	if body == nil {
		t.Fatal("OpenAdmitted source missing")
	}
	text := func(node ast.Node) string {
		var out bytes.Buffer
		if err := format.Node(&out, set, node); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	index := func(want string) int {
		t.Helper()
		found := -1
		for i, stmt := range body {
			if text(stmt) == want {
				if found >= 0 {
					t.Fatalf("duplicate constructor step %q", want)
				}
				found = i
			}
		}
		if found < 0 {
			t.Fatalf("missing constructor step %q", want)
		}
		return found
	}
	guard := "if err != nil {\n\treturn nil, err\n}"
	open := index("file, err := openReadOnlyDatabase(path)")
	own := index("cleanup.main = plainFile{file}")
	initial := index("info, err := statNativeFile(file)")
	regular := index("if !info.Mode().IsRegular() {\n\treturn nil, fmt.Errorf(\"%w: database must be a regular file\", ErrUnsupported)\n}")
	lock := index("if err = lockShared(file); err != nil {\n\treturn nil, err\n}")
	snapshot := index("info, err = statNativeFile(file)")
	reader := index("r := &Reader{file: file, path: path, info: info, cleanup: cleanup}")
	parse := index("r.header, err = ParseHeader(r.rawHeader[:], info.Size())")
	stable := index("if err = r.checkStable(); err != nil {\n\treturn nil, err\n}")
	if own != open+2 || text(body[open+1]) != guard || initial != own+1 ||
		text(body[initial+1]) != guard || regular != initial+2 || lock != regular+1 ||
		snapshot != lock+1 || text(body[snapshot+1]) != guard || reader != snapshot+2 ||
		parse <= reader || text(body[parse+1]) != guard || stable <= parse {
		t.Fatal("reader ownership, regular-file check, SHARED snapshot or validation ordering changed")
	}
	// The original retirement owner still handles every unsuccessful constructor
	// exit, including the added metadata acquisition. No raw native close path.
	index("defer func() {\n\tif result == nil {\n\t\tretErr = finishReaderProbe(cleanup, path, retErr)\n\t}\n}()")
}

// The writer changes only valid schema-cookie metadata through normal pager
// transactions. Independent readers acquire SHARED repeatedly and verify a
// pinned page twice while commits/intentional rollbacks contend normally.
// There are no native hooks, replaced files, injected faults or forced exits.
func TestReaderAcquisitionOrdinaryCooperatingWriter(t *testing.T) {
	path := createEmpty(t, 4096)
	ctx := context.Background()
	const commits = 128
	deadline := time.Now().Add(20 * time.Second)
	stop := make(chan struct{})
	type outcome struct {
		reads int
		err   error
	}
	results := make(chan outcome, 2)
	var started sync.WaitGroup
	started.Add(2)
	for range 2 {
		go func() {
			started.Done()
			out := outcome{}
			defer func() { results <- out }()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if time.Now().After(deadline) {
					out.err = errors.New("ordinary reader exceeded cooperative deadline")
					return
				}
				r, err := Open(path)
				if readerAcquisitionBusy(err) {
					time.Sleep(100 * time.Microsecond)
					continue
				}
				if err != nil {
					out.err = fmt.Errorf("reader open: %w", err)
					return
				}
				err = func() (retErr error) {
					defer func() { retErr = errors.Join(retErr, r.Close()) }()
					h := r.Header()
					if h.SchemaCookie > commits || h.ChangeCounter != h.SchemaCookie+1 || h.PageCount != 1 {
						return fmt.Errorf("reader header inconsistent: %+v", h)
					}
					first, err := r.ReadPage(ctx, 1)
					if err != nil {
						return err
					}
					runtime.Gosched()
					second, err := r.ReadPage(ctx, 1)
					if err != nil {
						return err
					}
					if !bytes.Equal(first, second) || binary.BigEndian.Uint32(first[40:]) != h.SchemaCookie || binary.BigEndian.Uint32(first[24:]) != h.ChangeCounter {
						return errors.New("reader lost its pinned committed generation")
					}
					return nil
				}()
				if err != nil {
					out.err = fmt.Errorf("reader snapshot: %w", err)
					return
				}
				out.reads++
				time.Sleep(100 * time.Microsecond)
			}
		}()
	}
	started.Wait()
	writeErr := func() error {
		for generation := uint32(0); generation < commits; generation++ {
			// Each fourth committed generation also has an intentional rollback.
			if generation%4 == 0 {
				tx, err := OpenTx(ctx, path, Options{})
				if err != nil {
					return err
				}
				err = tx.BumpSchemaCookie(ctx)
				if err = errors.Join(err, tx.Rollback()); err != nil {
					return err
				}
			}
			tx, err := OpenTx(ctx, path, Options{})
			if err != nil {
				return err
			}
			err = func() (retErr error) {
				defer func() { retErr = errors.Join(retErr, tx.Close()) }()
				if tx.Header().SchemaCookie != generation {
					return fmt.Errorf("writer generation %d, want %d", tx.Header().SchemaCookie, generation)
				}
				if err := tx.BumpSchemaCookie(ctx); err != nil {
					return err
				}
				for {
					if time.Now().After(deadline) {
						return errors.New("ordinary writer exceeded cooperative deadline")
					}
					err := tx.Commit(ctx)
					if !readerAcquisitionBusy(err) {
						return err
					}
					time.Sleep(100 * time.Microsecond)
				}
			}()
			if err != nil {
				return err
			}
		}
		return nil
	}()
	close(stop)
	for range 2 {
		result := <-results // every worker closes its owned Reader before returning
		if result.err != nil {
			t.Error(result.err)
		}
		if result.reads == 0 {
			t.Error("reader acquired no successful snapshot")
		}
	}
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Header().SchemaCookie; got != commits {
		t.Errorf("final generation = %d, want %d", got, commits)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
}

// Accept only an error tree made exclusively of known BUSY leaves. An unknown
// joined close/ownership/metadata diagnostic must fail this ordinary workload.
func readerAcquisitionBusy(err error) bool {
	if err == nil {
		return false
	}
	pending := []error{err}
	for nodes := 0; len(pending) != 0; nodes++ {
		if nodes == 64 {
			return false
		}
		e := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if e == nil {
			return false
		}
		switch u := e.(type) {
		case interface{ Unwrap() []error }:
			children := u.Unwrap()
			if len(children) == 0 || len(children)+len(pending) > 64 {
				return false
			}
			pending = append(pending, children...)
		case interface{ Unwrap() error }:
			pending = append(pending, u.Unwrap())
		default:
			if e != ErrBusy && e != vfs.ErrBusy {
				return false
			}
		}
	}
	return true
}

func TestReaderAcquisitionMemoryBusyClassification(t *testing.T) {
	busy := fmt.Errorf("%w: %w", ErrBusy, vfs.ErrBusy)
	if !readerAcquisitionBusy(errors.Join(busy)) {
		t.Fatal("ordinary BUSY rejected")
	}
	for _, err := range []error{nil, ErrChanged, errors.Join(busy, ErrChanged), errors.Join(busy, ErrPlaintextCleanupPending), errors.Join(busy, errors.New("memory cleanup diagnostic"))} {
		if readerAcquisitionBusy(err) {
			t.Fatal("unexpected diagnostic accepted as ordinary BUSY", err)
		}
	}
}
