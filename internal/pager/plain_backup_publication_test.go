package pager

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

func backupPagerRender(n ast.Node) string {
	var b bytes.Buffer; _ = format.Node(&b, token.NewFileSet(), n); return b.String()
}
func backupPagerFunc(source, name string) (*ast.FuncDecl, error) {
	f, err := parser.ParseFile(token.NewFileSet(), "source.go", source, 0)
	if err != nil { return nil, err }
	for _, d := range f.Decls { if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == name { return fn, nil } }
	return nil, fmt.Errorf("missing %s", name)
}
func backupPagerCall(n ast.Node, name string) []*ast.CallExpr {
	var result []*ast.CallExpr
	ast.Inspect(n, func(node ast.Node) bool { if c, ok := node.(*ast.CallExpr); ok && backupPagerRender(c.Fun) == name { result = append(result, c) }; return true })
	return result
}
func backupPagerGate(fn *ast.FuncDecl, call, before string) error {
	position := -1
	for i, stmt := range fn.Body.List {
		if gate, ok := stmt.(*ast.IfStmt); ok && gate.Init != nil && backupPagerRender(gate.Cond) == "err != nil" && gate.Else == nil && len(gate.Body.List) == 1 {
			if _, returns := gate.Body.List[0].(*ast.ReturnStmt); returns && len(backupPagerCall(gate.Init, call)) == 1 { position = i }
		}
		if len(backupPagerCall(stmt, before)) != 0 { if position < 0 || position >= i { return fmt.Errorf("missing direct %s gate before %s", call, before) }; return nil }
	}
	return fmt.Errorf("missing %s", before)
}
func backupPagerSourceGuard(source map[string]string) error {
	write, err := backupPagerFunc(source["plain_backup.go"], "WritePlaintextBackup"); if err != nil { return err }
	for _, pair := range [][2]string{{"source.ValidateRollbackBackupSource", "publication.OpenBackupDestination"}, {"publication.CreatePrivateCandidate", "copyBackupPages"}, {"copyBackupPages", "publication.SyncData"}, {"publication.SyncData", "publication.PublishNoReplace"}, {"publication.PublishNoReplace", "publication.SyncPublication"}} {
		if err := backupPagerGate(write, pair[0], pair[1]); err != nil { return err }
	}
	begin := backupPagerCall(write, "cleanup.begin")
	if len(begin) != 1 || len(begin[0].Args) != 1 || backupPagerRender(begin[0].Args[0]) != "nil" { return fmt.Errorf("source token may be adopted by destination") }
	for _, forbidden := range []string{"cleanup.observe", "cleanup.adoptLocked", "openWritableDatabase", "backupAbsent", "r.Close"} {
		if len(backupPagerCall(write, forbidden)) != 0 { return fmt.Errorf("forbidden target ownership/path route %s", forbidden) }
	}
	calls := backupPagerCall(write, "publication.OpenBackupDestination")
	if len(calls) != 1 || len(calls[0].Args) != 4 || backupPagerRender(calls[0].Args[0]) != "parent" || backupPagerRender(calls[0].Args[2]) != "sourceIdentity" { return fmt.Errorf("target lacks actual source/parent capability arguments") }
	newOwner, err := backupPagerFunc(source["plain_backup_publication.go"], "newPlainBackupPublication"); if err != nil { return err }
	reserved, created, installed := -1, -1, -1
	for i, stmt := range newOwner.Body.List {
		if len(backupPagerCall(stmt, "plainRetirements.reserve")) == 1 { reserved = i }
		if len(backupPagerCall(stmt, "vfs.NewBackupPublicationOwner")) == 1 { created = i }
		if a, ok := stmt.(*ast.AssignStmt); ok && len(a.Lhs) == 1 && backupPagerRender(a.Lhs[0]) == "cleanup.main" && backupPagerRender(a.Rhs[0]) == "plainBackupPublication{publication: publication}" { installed = i }
	}
	if !(reserved >= 0 && reserved < created && created < installed) { return fmt.Errorf("composite not preinstalled in reserved owner") }
	sourceBegin, err := backupPagerFunc(source["plain_backup_source.go"], "BeginBackupNamespace"); if err != nil { return err }
	if err = backupPagerGate(sourceBegin, "r.cleanup.reserveTemporary", "proof.Acquire"); err != nil { return err }
	installed = -1
	for i, stmt := range sourceBegin.Body.List {
		if a, ok := stmt.(*ast.AssignStmt); ok && len(a.Lhs) == 1 && backupPagerRender(a.Lhs[0]) == "r.cleanup.temporary" && backupPagerRender(a.Rhs[0]) == "plainBackupSource{namespace: proof}" { installed = i }
		if len(backupPagerCall(stmt, "proof.Acquire")) != 0 && (installed < 0 || installed >= i) { return fmt.Errorf("source descriptor acquired before owner installation") }
	}
	public, err := backupPagerFunc(source["../../sqlite/backup.go"], "backupSnapshotLocked"); if err != nil { return err }
	proof, check, copy := -1, -1, -1
	for i, stmt := range public.Body.List {
		if len(backupPagerCall(stmt, "r.BeginBackupNamespace")) != 0 { proof = i }
		if len(backupPagerCall(stmt, "d.checkSnapshot")) != 0 { check = i }
		if len(backupPagerCall(stmt, "pager.WritePlaintextBackup")) != 0 { copy = i }
	}
	if !(proof >= 0 && proof < check && check < copy) { return fmt.Errorf("proof/full Check do not precede target") }
	if err := backupPagerGate(public, "d.checkSnapshot", "pager.WritePlaintextBackup"); err != nil { return err }
	calls = backupPagerCall(public, "pager.WritePlaintextBackup")
	if len(calls) != 1 || len(calls[0].Args) != 4 || backupPagerRender(calls[0].Args[2]) != "r" || backupPagerRender(calls[0].Args[3]) != "proof" { return fmt.Errorf("checked reader/proof substituted") }
	return backupSourceRefusalGuard(source)
}
func backupPagerSourceInputs(t *testing.T) map[string]string {
	t.Helper(); source := make(map[string]string)
	for _, name := range []string{"plain_backup.go", "plain_backup_source.go", "plain_backup_publication.go", "../../sqlite/backup.go", "../vfs/backup_namespace.go", "../vfs/backup_namespace_linux.go", "../vfs/backup_publication.go"} {
		body, err := os.ReadFile(name); if err != nil { t.Fatal(err) }; source[name] = string(body)
	}
	return source
}
func TestBackupSourceModelIndependentOwners(t *testing.T) {
	registry := plainRetirementRegistry{limit: 2}
	g, _ := registry.reserve()
	var events []string
	main := &backupRetirementModel{name: "source-main", trace: &events, finish: true}
	parent := &backupRetirementModel{name: "source-parent", trace: &events}
	g.main, g.temporary = main, parent
	first := g.closeTemporary(nil)
	if !errors.Is(first, ErrPlaintextCleanupPending) || parent.calls != 1 || main.calls != 0 { t.Fatal(first, events) }
	r := &Reader{cleanup: g}
	closed := r.CloseBackupReader()
	if !errors.Is(closed, ErrPlaintextCleanupPending) || parent.calls != 1 || main.calls != 0 { t.Fatal("same-operation repeated retirement", closed, events) }
	parent.finish = true
	if err := RetryPlaintextCleanup(closed); err != nil || parent.calls != 2 || main.calls != 1 || len(registry.groups) != 0 { t.Fatal(err, events) }
	// A live Tx's successful temporary retirement leaves its source main held.
	g, _ = registry.reserve(); events = nil
	main = &backupRetirementModel{name: "source-main", trace: &events, finish: true}
	parent = &backupRetirementModel{name: "source-parent", trace: &events, finish: true}
	g.main, g.temporary = main, parent
	if err := g.closeTemporary(nil); err != nil || main.calls != 0 || g.done { t.Fatal("proof retirement closed pinned Tx", err) }
	if err := g.begin(nil); err != nil { t.Fatal(err) }
}
func TestBackupPublicationSourceOwnerBoundaryMutations(t *testing.T) {
	original := backupPagerSourceInputs(t)
	if err := backupPagerSourceGuard(original); err != nil { t.Fatal(err) }
	for _, mutation := range []struct{file, before, after string}{
		{"plain_backup.go", "cleanup.begin(nil)", "cleanup.begin(resultErr)"},
		{"plain_backup.go", "publication.SyncData()", "publication.SyncPublication()"},
		{"plain_backup_publication.go", "cleanup.main = plainBackupPublication", "cleanup.temporary = plainBackupPublication"},
		{"plain_backup_source.go", "r.cleanup.temporary = plainBackupSource", "r.cleanup.main = plainBackupSource"},
		{"../../sqlite/backup.go", "pager.WritePlaintextBackup(ctx, destination, r, proof)", "pager.WritePlaintextBackup(ctx, destination, r, nil)"},
	} {
		copy := make(map[string]string); for name, body := range original { copy[name] = body }
		if !strings.Contains(copy[mutation.file], mutation.before) { t.Fatal("mutation no longer applies") }
		copy[mutation.file] = strings.Replace(copy[mutation.file], mutation.before, mutation.after, 1)
		if backupPagerSourceGuard(copy) == nil { t.Fatal("unsafe mutation accepted", mutation) }
	}
}

func backupSourceModelGroup(t *testing.T) (*plainRetirement, *backupRetirementModel, *backupRetirementModel) {
	t.Helper()
	registry := &plainRetirementRegistry{limit: 1}
	g, err := registry.reserve()
	if err != nil { t.Fatal(err) }
	trace := []string{}
	main := &backupRetirementModel{name: "source-main", trace: &trace, finish: true}
	parent := &backupRetirementModel{name: "source-parent", trace: &trace, finish: true}
	g.main, g.temporary = main, parent
	return g, main, parent
}
func backupSourceModelRefuse(g *plainRetirement, expected, main plainResource, cause error, benign bool) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.closeFailedBackupSourceLocked(expected, main, cause, benign)
}
func TestBackupSourceModelCleanUnsupportedKeepsReader(t *testing.T) {
	g, main, parent := backupSourceModelGroup(t)
	diagnostic := &os.SyscallError{Syscall: "statx", Err: syscall.ENOSYS}
	cause := errors.Join(ErrUnsupported, diagnostic)
	err := backupSourceModelRefuse(g, parent, main, cause, true)
	var got *os.SyscallError
	if !errors.Is(err, ErrUnsupported) || !errors.Is(err, syscall.ENOSYS) || !errors.As(err, &got) || got != diagnostic || errors.Is(err, ErrPoisoned) || errors.Is(err, ErrPlaintextCleanupPending) { t.Fatal("lost clean refusal diagnostics", err) }
	if parent.calls != 1 || main.calls != 0 || g.main != main || g.temporary != nil || g.temporaryReserved || g.temporaryRequested || g.cause != nil || g.pending.Load() != nil || g.started || g.done || g.check() != nil { t.Fatal("clean refusal disabled the pinned source", err) }
	// A second inert permitted operation is admitted by the actual group state;
	// this does not call Reader.ReadPage or infer native snapshot stability.
	if err := g.reserveTemporary(); err != nil { t.Fatal("subsequent operation refused", err) }
	second := &backupRetirementModel{name: "next-source-scope", trace: main.trace, finish: true}
	g.mu.Lock(); g.temporaryReserved, g.temporary = false, second; g.mu.Unlock()
	if err := g.closeTemporary(nil); err != nil || second.calls != 1 || main.calls != 0 || g.check() != nil { t.Fatal("subsequent inert operation failed", err) }
	if err := g.begin(nil); err != nil || main.calls != 1 || len(g.registry.groups) != 0 { t.Fatal("ordinary final retirement failed", err) }
}
func TestBackupSourceModelUnsafeRefusalKeepsOwnership(t *testing.T) {
	for _, classified := range []bool{false, true} {
		for _, complete := range []bool{false, true} {
			for _, diagnostic := range []bool{false, true} {
				t.Run(fmt.Sprintf("classified=%t/complete=%t/diagnostic=%t", classified, complete, diagnostic), func(t *testing.T) {
					g, main, parent := backupSourceModelGroup(t)
					parent.finish = complete
					closeCause := &os.SyscallError{Syscall: "close", Err: syscall.EIO}
					if diagnostic { parent.err = closeCause }
					err := backupSourceModelRefuse(g, parent, main, ErrUnsupported, classified)
					clean := classified && complete && !diagnostic
					if !errors.Is(err, ErrUnsupported) || errors.Is(err, ErrPoisoned) == clean || errors.Is(err, ErrPlaintextCleanupPending) != !complete || parent.calls != 1 || main.calls != 0 || g.main != main || g.done || g.started { t.Fatal("wrong refusal transition", err) }
					if diagnostic { var got *os.SyscallError; if !errors.Is(err, syscall.EIO) || !errors.As(err, &got) || got != closeCause { t.Fatal("lost first close diagnostic", err) } }
					if (g.check() == nil) != clean { t.Fatal("unsafe state became readable", g.check()) }
					if !complete {
						// A live Tx retry retires only the source parent. Historical
						// poison remains, even after all temporary native ownership ends.
						parent.finish, parent.err = true, nil
						if retry := RetryPlaintextCleanup(err); retry != nil || parent.calls != 2 || main.calls != 0 || g.temporary != nil || g.pending.Load() != nil || !errors.Is(g.check(), ErrPoisoned) { t.Fatal("live retry resumed poisoned Tx", retry) }
					}
					if end := g.begin(nil); main.calls != 1 || !g.done || len(g.registry.groups) != 0 || (!clean && !errors.Is(end, ErrPoisoned)) { t.Fatal("final ownership/diagnostic lost", end) }
				})
			}
		}
	}
	for _, dbClose := range []bool{false, true} {
		g, main, parent := backupSourceModelGroup(t)
		parent.finish = false
		first := backupSourceModelRefuse(g, parent, main, ErrUnsupported, true)
		if !errors.Is(first, ErrPlaintextCleanupPending) || parent.calls != 1 { t.Fatal(first) }
		if dbClose {
			r := &Reader{cleanup: g}
			first = r.CloseBackupReader()
			if !errors.Is(first, ErrPlaintextCleanupPending) || parent.calls != 1 || main.calls != 0 { t.Fatal("DB immediately retried acquisition cleanup", first) }
		}
		parent.finish = true
		if err := RetryPlaintextCleanup(first); err != nil || parent.calls != 2 { t.Fatal(err) }
		if dbClose {
			if main.calls != 1 || !g.done || fmt.Sprint(*main.trace) != "[source-parent source-parent source-main]" { t.Fatal("directory did not protect main", *main.trace) }
		} else if main.calls != 0 || g.done || !errors.Is(g.check(), ErrPoisoned) { t.Fatal("live retry closed or unpoisoned source") }
	}
}

type backupSourceErrorCycle struct{}
func (e *backupSourceErrorCycle) Error() string { return "source error cycle" }
func (e *backupSourceErrorCycle) Unwrap() error { return e }
func TestBackupSourceModelRefusalRejectsUnsafeState(t *testing.T) {
	for _, test := range []string{"prior-poison", "identity", "poison-join", "postpublication-cause", "unknown-nested", "nested-live", "nested-complete", "cyclic-cause", "wrong-temporary", "wrong-main", "pending", "already-requested", "reserved", "wal", "wal-reserved", "wal-requested", "started", "done", "already-consumed", "main-consumed"} {
		t.Run(test, func(t *testing.T) {
			g, main, parent := backupSourceModelGroup(t)
			cause := error(ErrUnsupported)
			expected, originalMain := plainResource(parent), plainResource(main)
			prior := errors.New("prior-failure")
			wantClose := true
			switch test {
			case "prior-poison": g.cause = errors.Join(ErrPoisoned, prior)
			case "identity": cause = errors.Join(cause, vfs.ErrIdentity)
			case "poison-join": cause = errors.Join(cause, ErrPoisoned)
			case "postpublication-cause": cause = errors.Join(cause, ErrCommitUncertain)
			case "unknown-nested": g.nested = []plainResource{plainTerminal{prior}}; wantClose = false
			case "nested-live": g.nested = []plainResource{&backupRetirementModel{name: "nested", trace: main.trace, finish: true}}; wantClose = false
			case "nested-complete": g.nested = []plainResource{&backupRetirementModel{name: "nested", trace: main.trace, complete: true, finish: true}}; wantClose = false
			case "cyclic-cause": cause = errors.Join(cause, &backupSourceErrorCycle{}); wantClose = false
			case "wrong-temporary": expected = &backupRetirementModel{name: "wrong-proof", trace: main.trace, finish: true}; wantClose = false
			case "wrong-main": originalMain = &backupRetirementModel{name: "wrong-main", trace: main.trace, finish: true}; wantClose = false
			case "pending": g.pendingLocked(prior); wantClose = false
			case "already-requested": g.temporaryRequested = true; wantClose = false
			case "reserved": g.temporaryReserved = true; wantClose = false
			case "wal": g.wal = &backupRetirementModel{name: "wal", trace: main.trace, finish: true}; wantClose = false
			case "wal-reserved": g.walReserved = true; wantClose = false
			case "wal-requested": g.walRequested = true; wantClose = false
			case "started": g.started = true; wantClose = false
			case "done": g.done = true; wantClose = false
			case "already-consumed": parent.complete = true; wantClose = false
			case "main-consumed": main.complete = true
			}
			err := backupSourceModelRefuse(g, expected, originalMain, cause, true)
			if !errors.Is(err, ErrPoisoned) || g.check() == nil || main.calls != 0 || g.main != main || (parent.calls == 1) != wantClose { t.Fatal("unsafe source resumed/lost owner", err, parent.calls) }
			if test == "prior-poison" && !errors.Is(err, prior) { t.Fatal("old poison erased", err) }
			if test == "wrong-temporary" && (g.temporary != parent || len(g.nested) != 2 || g.nested[1] != expected) { t.Fatal("substituted owner forgotten") }
			if test == "cyclic-cause" && (len(g.nested) == 0 || !errors.Is(err, ErrPlaintextCleanupPending)) { t.Fatal("unknown graph discarded", err) }
			if (test == "nested-live" || test == "nested-complete") && (len(g.nested) != 2 || g.nested[1].(*backupRetirementModel).calls != 0) { t.Fatal("unrelated nested owner retired") }
			if test == "identity" && !errors.Is(err, vfs.ErrIdentity) { t.Fatal("identity evidence erased", err) }
			// The first-attempt transition returns an error only, never FileInfo;
			// the guarded failed-Acquire caller must return a nil source proof.
		})
	}
}

// These deliberately exact, independently reviewable AST contracts bind the
// models to actual production owner/status inputs and error-return order. A
// legitimate control-flow change must update its reviewed contract explicitly.
// No native function in the canonical text is evaluated by this guard.
func backupSourceRefusalGuard(source map[string]string) error {
	for _, contract := range []struct{ file, name, function string }{
		{"plain_backup_source.go", "BeginBackupNamespace", `func (r *Reader) BeginBackupNamespace() (*vfs.BackupSourceNamespace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lease != nil { return nil, ErrUnsupported }
	if r.closed || r.file == nil { return nil, ErrClosed }
	if err := r.cleanup.check(); err != nil { return nil, err }
	if err := r.cleanup.reserveTemporary(); err != nil { return nil, err }
	proof := vfs.NewBackupSourceNamespace()
	r.cleanup.mu.Lock()
	r.cleanup.temporaryReserved = false
	r.cleanup.temporary = plainBackupSource{namespace: proof}
	r.cleanup.mu.Unlock()
	if err := proof.Acquire(r.file); err != nil {
		// End this live temporary scope once. A later DB-specific close must
		// not repeat an incomplete close in the same Backup operation.
		return nil, r.closeFailedBackupNamespace(proof, err)
	}
	return proof, nil
}`},
		{"plain_backup_source.go", "closeFailedBackupNamespace", `func (r *Reader) closeFailedBackupNamespace(proof *vfs.BackupSourceNamespace, cause error) error {
	cause = nativeFileError(cause)
	benign := proof.BenignAcquisitionRefusal()
	r.cleanup.mu.Lock()
	defer r.cleanup.mu.Unlock()
	return r.cleanup.closeFailedBackupSourceLocked(plainBackupSource{namespace: proof}, plainFile{file: r.file}, cause, benign)
}`},
		{"plain_backup_source.go", "closeFailedBackupSourceLocked", `func (g *plainRetirement) closeFailedBackupSourceLocked(expected, main plainResource, cause error, benign bool) error {
	originalCause, originalPending := g.cause, g.pending.Load()
	originalNested := len(g.nested)
	sameOwner := expected != nil && main != nil && g.temporary == expected && g.main == main
	cause = g.adoptLocked(cause)
	adoptedOwnership := len(g.nested) != originalNested
	// Acquisition diagnostics normally contain no exposed native owners: the
	// namespace facade retains them. If that contract is violated, an adopted
	// owner might alias main; never let generic nested-first retirement guess.
	if len(g.nested) != 0 { g.nested = append([]plainResource{plainTerminal{ErrPoisoned}}, g.nested...) }
	canAttempt := sameOwner && !adoptedOwnership && !g.started && !g.done && !g.temporaryRequested && !g.temporaryReserved && originalPending == nil && originalNested == 0 && g.wal == nil && !g.walReserved && !g.walRequested
	if !canAttempt {
		if sameOwner && len(g.nested) != 0 { g.temporaryRequested = true }
		// A substituted owner or already-ended scope cannot authorize a close.
		// Preserve the unexpected supplied proof too, behind a terminal barrier.
		if expected != nil && g.temporary != expected && g.main != expected {
			g.nested = append(g.nested, plainTerminal{ErrMisuse}, expected)
		}
		g.cause = plainDiagnostic(errors.Join(originalCause, ErrPoisoned, ErrMisuse, cause))
		if g.temporary != nil || len(g.nested) != 0 { return g.pendingLocked(g.cause) }
		return g.cause
	}
	cleanEntry := benign && originalCause == nil && !adoptedOwnership && !main.RetirementComplete() && !expected.RetirementComplete() && errors.Is(cause, ErrUnsupported) && !errors.Is(cause, ErrPoisoned) && !errors.Is(cause, ErrChanged) && !errors.Is(cause, vfs.ErrIdentity) && !errors.Is(cause, vfs.ErrPoisoned) && !errors.Is(cause, ErrCommitUncertain) && !errors.Is(cause, ErrClosed) && !errors.Is(cause, ErrMisuse) && !errors.Is(cause, ErrLimit)
	g.temporaryRequested = true
	retirementErr := g.retireLocked() // g.started is false: source main stays held
	if cleanEntry && retirementErr == nil && g.temporary == nil && len(g.nested) == 0 && !g.temporaryRequested && !g.temporaryReserved && g.main == main && !g.started && !g.done && g.cause == nil && g.pending.Load() == nil {
		return plainDiagnostic(cause)
	}
	g.cause = plainDiagnostic(errors.Join(originalCause, ErrPoisoned, cause, retirementErr))
	if g.temporary != nil || len(g.nested) != 0 { return g.pendingLocked(g.cause) }
	return g.cause
}`},
		{"../vfs/backup_namespace.go", "Acquire", `func (s *BackupSourceNamespace) Acquire(main *File) error {
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
}`},
		{"../vfs/backup_namespace.go", "backupSourcePolicyRefusal", `func backupSourcePolicyRefusal(input error) bool {
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
}`},
		{"../vfs/backup_namespace.go", "recordBenignAcquisitionRefusal", `func (s *BackupSourceNamespace) recordBenignAcquisitionRefusal(edge backupSourceRefusalEdge, cause error, matchedParent bool) {
	if !s.acquisitionAttempted || s.sealed || s.terminal || s.identity.valid { return }
	if edge == backupSourceNameRefusal && cause == ErrUnsupported || edge == backupSourceParentRefusal && matchedParent && backupSourcePolicyRefusal(cause) {
		s.benignAcquisitionRefusal = true
	}
}`},
		{"../vfs/backup_namespace.go", "BenignAcquisitionRefusal", `func (s *BackupSourceNamespace) BenignAcquisitionRefusal() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.benignAcquisitionRefusal && !s.terminal && !s.identity.valid
}`},
		{"../vfs/backup_publication.go", "openDestinationLocked", `func (p *BackupPublication) openDestinationLocked(parent, final string, source BackupSourceIdentity, size int64, candidate string) error {
	if err := p.active(); err != nil { return err }
	if p.opened || !source.valid || !BackupBasename(final) || !BackupBasename(candidate) || size <= 0 { return p.fail(ErrUnsupported) }
	p.opened = true // at most one directory acquisition, including a failed one
	p.final, p.candidate, p.expected = final, candidate, size
	parentIdentity, err := p.ops.openDirectory(parent)
	if err != nil { return p.fail(err) }
	if err = backupDestinationAdmission(source, parentIdentity, p.final, p.candidate); err != nil { return p.fail(err) }
	if err = p.ops.absentFamily(p.final); err != nil { return p.fail(err) }
	if err = p.ops.absentFamily(p.candidate); err != nil { return p.fail(err) }
	return nil
}`},
		{"../vfs/backup_namespace_linux.go", "nativeBackupObserveSource", `func nativeBackupObserveSource(main *File, dir *backupLease, base string) (backupFact, error) {
	if err := nativeBackupContinuity(dir); err != nil { return backupFact{}, err }
	parent, err := nativeBackupHeldFact(dir.file)
	if err != nil { return backupFact{}, err }
	held, err := nativeBackupHeldFact(main)
	if err != nil { return backupFact{}, err }
	named, err := nativeBackupNamedFact(dir, base)
	if err != nil { return backupFact{}, err }
	if err = backupDirectChild(parent, held, named); err != nil { return backupFact{}, err }
	info, err := main.raw.Stat()
	if err != nil { return backupFact{}, err }
	if !SameFile(info, main.identity) || !SameFile(dir.file.identity, main.parent) { return backupFact{}, ErrIdentity }
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		fact, err := nativeBackupNamedFact(dir, base+suffix)
		if errors.Is(err, syscall.ENOENT) { continue }
		if err != nil { return backupFact{}, err }
		if err = backupSourceSidecar(parent, suffix, fact); err != nil { return backupFact{}, err }
	}
	return parent, nil
}`},

	} {
		actual, err := backupPagerFunc(source[contract.file], contract.name)
		if err != nil { return err }
		expected, err := backupPagerFunc("package contract\n"+contract.function, contract.name)
		if err != nil { return err }
		if backupPagerRender(actual) != backupPagerRender(expected) { return fmt.Errorf("%s %s changed its acquisition-only contract", contract.file, contract.name) }
	}
	counts := make(map[string]int)
	factWrites := 0
	for name, body := range source {
		file, err := parser.ParseFile(token.NewFileSet(), name, body, 0)
		if err != nil { return err }
		ast.Inspect(file, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				if selector, ok := call.Fun.(*ast.SelectorExpr); ok { counts[selector.Sel.Name]++ }
			}
			if assignment, ok := node.(*ast.AssignStmt); ok {
				for _, target := range assignment.Lhs {
					if selector, ok := target.(*ast.SelectorExpr); ok && selector.Sel.Name == "benignAcquisitionRefusal" { factWrites++ }
				}
			}
			return true
		})
	}
	for name, want := range map[string]int{"closeFailedBackupNamespace": 1, "closeFailedBackupSourceLocked": 1, "BenignAcquisitionRefusal": 1, "recordBenignAcquisitionRefusal": 2} {
		if counts[name] != want { return fmt.Errorf("%s escaped the sole failed source acquisition edge", name) }
	}
	if factWrites != 1 { return fmt.Errorf("refusal fact was rewritten outside its minting edge") }
	return nil
}
func TestBackupSourceAdmissionSourceMutationCounterexamples(t *testing.T) {
	original := backupPagerSourceInputs(t)
	if err := backupPagerSourceGuard(original); err != nil { t.Fatal(err) }
	for _, mutation := range []struct{ file, before, after string }{
		{"plain_backup_source.go", "benign := proof.BenignAcquisitionRefusal()", "benign := true"},
		{"../vfs/backup_namespace.go", "if err := main.checkLocked(); err != nil { return s.failure(err) }", "if err := main.checkLocked(); err != nil { return err }"},
		{"plain_backup_source.go", "if len(g.nested) != 0 { g.nested = append([]plainResource{plainTerminal{ErrPoisoned}}, g.nested...) }", "_ = adoptedOwnership"},
		{"plain_backup_source.go", "sameOwner && !adoptedOwnership && !g.started", "sameOwner && !g.started"},

		{"plain_backup_source.go", "proof.BenignAcquisitionRefusal()", "vfs.NewBackupSourceNamespace().BenignAcquisitionRefusal()"},
		{"plain_backup_source.go", "closeFailedBackupSourceLocked(plainBackupSource{namespace: proof}, plainFile{file: r.file}, cause, benign)", "closeFailedBackupSourceLocked(plainBackupSource{namespace: nil}, plainFile{file: r.file}, cause, benign)"},
		{"plain_backup_source.go", "plainFile{file: r.file}, cause, benign", "plainFile{file: nil}, cause, benign"},
		{"plain_backup_source.go", "return nil, r.closeFailedBackupNamespace(proof, err)", "return nil, r.cleanup.closeTemporary(nativeFileError(err))"},
		{"plain_backup_source.go", "return nil, r.closeFailedBackupNamespace(proof, err)", "_ = r.closeFailedBackupNamespace(proof, err); return nil, r.cleanup.closeTemporary(nativeFileError(err))"},
		{"plain_backup_source.go", "return nil, r.closeFailedBackupNamespace(proof, err)", "_ = proof.Close(); return nil, r.closeFailedBackupNamespace(proof, err)"},
		{"plain_backup_source.go", "return nil, r.closeFailedBackupNamespace(proof, err)", "_ = r.closeFailedBackupNamespace(proof, err); return proof, nil"},
		{"plain_backup_source.go", "cleanEntry := benign && originalCause == nil && !adoptedOwnership", "cleanEntry := true"},
		{"plain_backup_source.go", "cleanEntry := benign && originalCause == nil && !adoptedOwnership", "cleanEntry := benign"},
		{"plain_backup_source.go", "retirementErr == nil && g.temporary == nil", "g.temporary == nil"},
		{"plain_backup_source.go", "retirementErr == nil && g.temporary == nil", "retirementErr == nil"},
		{"plain_backup_source.go", "g.temporary == expected && g.main == main", "g.temporary != nil && g.main != nil"},
		{"plain_backup_source.go", "cause = g.adoptLocked(cause)", "_ = cause"},
		{"plain_backup_source.go", "g.temporaryRequested = true\n\tretirementErr := g.retireLocked()", "retirementErr := g.retireLocked()\n\tg.temporaryRequested = true"},
		{"plain_backup_source.go", "retirementErr := g.retireLocked()", "g.started = true\n\tretirementErr := g.retireLocked()"},
		{"plain_backup_source.go", "retirementErr := g.retireLocked()", "retirementErr := g.retireLocked(); _ = g.retireLocked()"},
		{"plain_backup_source.go", "originalCause, originalPending := g.cause, g.pending.Load()", "g.cause = nil\n\toriginalCause, originalPending := g.cause, g.pending.Load()"},
		{"plain_backup_source.go", "g.cause = plainDiagnostic(errors.Join(originalCause, ErrPoisoned, cause, retirementErr))", "g.cause = nil"},
		{"plain_backup_source.go", "if g.temporary != nil || len(g.nested) != 0 { return g.pendingLocked(g.cause) }", "if false { return g.pendingLocked(g.cause) }"},
		{"../vfs/backup_namespace.go", "return s.benignAcquisitionRefusal && !s.terminal && !s.identity.valid", "return true"},
		{"../vfs/backup_namespace.go", "edge == backupSourceParentRefusal && matchedParent && backupSourcePolicyRefusal(cause)", "edge == backupSourceParentRefusal && backupSourcePolicyRefusal(cause)"},
		{"../vfs/backup_namespace.go", "cause == ErrUnsupported || edge == backupSourceParentRefusal", "errors.Is(cause, ErrUnsupported) || edge == backupSourceParentRefusal"},
		{"../vfs/backup_namespace.go", "s.dir.file.identity != nil && s.dir.file.identity.IsDir() && SameFile(s.dir.file.identity, main.parent)", "s.dir.file.identity != nil"},
		{"../vfs/backup_namespace.go", "SameFile(s.dir.file.identity, main.parent)", "SameFile(s.dir.file.identity, s.dir.file.identity)"},
		{"../vfs/backup_namespace.go", "s.recordBenignAcquisitionRefusal(backupSourceParentRefusal, err, matchedParent)", "s.recordBenignAcquisitionRefusal(backupSourceParentRefusal, ErrUnsupported, matchedParent)"},
		{"../vfs/backup_namespace.go", "if err != nil { return s.failure(err) }\n\ts.identity =", "if err != nil { s.recordBenignAcquisitionRefusal(backupSourceParentRefusal, err, true); return s.failure(err) }\n\ts.identity ="},
		{"../vfs/backup_namespace.go", "s.sealed = true\n\ts.identity.valid = false", "s.sealed = true\n\ts.benignAcquisitionRefusal = true\n\ts.identity.valid = false"},
		{"plain_backup.go", "if err = publication.PublishNoReplace(); err != nil { return nativeFileError(err) }", "if err = publication.PublishNoReplace(); err != nil { return source.closeFailedBackupNamespace(proof, err) }"},
		{"../vfs/backup_publication.go", "if err = backupDestinationAdmission(source, parentIdentity, p.final, p.candidate); err != nil { return p.fail(err) }", "_ = source; _ = parentIdentity"},
		{"../vfs/backup_publication.go", "if err = backupDestinationAdmission(source, parentIdentity, p.final, p.candidate); err != nil { return p.fail(err) }", "_ = backupDestinationAdmission(source, parentIdentity, p.final, p.candidate)"},
		{"../vfs/backup_publication.go", "backupDestinationAdmission(source, parentIdentity, p.final, p.candidate)", "backupDestinationAdmission(source, source.parent, p.final, p.candidate)"},
		{"../vfs/backup_namespace_linux.go", "if err = backupDirectChild(parent, held, named); err != nil { return backupFact{}, err }", "_ = held; _ = named"},
		{"../vfs/backup_namespace_linux.go", "if err = backupDirectChild(parent, held, named); err != nil { return backupFact{}, err }", "_ = backupDirectChild(parent, held, named)"},
		{"../vfs/backup_namespace_linux.go", "backupDirectChild(parent, held, named)", "backupDirectChild(parent, named, named)"},

	} {
		copy := make(map[string]string)
		for name, body := range original { copy[name] = body }
		if !strings.Contains(copy[mutation.file], mutation.before) { t.Fatal("refusal mutation no longer applies", mutation) }
		copy[mutation.file] = strings.Replace(copy[mutation.file], mutation.before, mutation.after, 1)
		if _, err := parser.ParseFile(token.NewFileSet(), mutation.file, copy[mutation.file], 0); err != nil { t.Fatal("mutation only broke syntax", mutation, err) }
		if backupPagerSourceGuard(copy) == nil { t.Fatal("unsafe source refusal mutation accepted", mutation) }
	}
}

func TestBackupSourceModelNestedAliasesKeepProtectors(t *testing.T) {
	for _, cyclic := range []bool{false, true} {
		g, main, parent := backupSourceModelGroup(t)
		cause := error(ErrUnsupported)
		if cyclic { cause = errors.Join(cause, &backupSourceErrorCycle{}) } else { g.nested = []plainResource{main} }
		first := backupSourceModelRefuse(g, parent, main, cause, true)
		if !errors.Is(first, ErrPlaintextCleanupPending) || main.calls != 0 || parent.calls != 0 || !g.temporaryRequested { t.Fatal("unknown order attempted retirement", first) }
		r := &Reader{cleanup: g}
		closed := r.CloseBackupReader()
		if !errors.Is(closed, ErrPlaintextCleanupPending) || main.calls != 0 || parent.calls != 0 { t.Fatal("DB close bypassed terminal source barrier", closed) }
		if err := RetryPlaintextCleanup(closed); !errors.Is(err, ErrPlaintextCleanupPending) || main.calls != 0 || parent.calls != 0 || g.main != main || g.temporary != parent || g.done { t.Fatal("explicit retry retired possible main alias", err) }
	}
}
