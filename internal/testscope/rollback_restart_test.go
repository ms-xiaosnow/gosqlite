package main

// These fixtures inspect source text and descriptor-free syntax only. Merely
// authoring them does not run Go, compile the native selection, spawn a child,
// create a database, call Commit/recovery, or qualify any physical evidence.
import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func restartGuardSources(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for path := range rollbackRestartPins {
		raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		out[path] = raw
	}
	return out
}
func restartGuardMustFind(t *testing.T, findings []string, err error, diagnostic string) {
	t.Helper()
	if err != nil || !strings.Contains(strings.Join(findings, "\n"), diagnostic) {
		t.Fatal("source-only semantic rejection missing", diagnostic, findings, err)
	}
}
func restartGuardRepinned(t *testing.T, path string, original []byte, old, replacement, diagnostic string) {
	t.Helper()
	if strings.Count(string(original), old) == 0 {
		t.Fatal("source-only mutation anchor missing", path, old)
	}
	raw := []byte(strings.Replace(string(original), old, replacement, 1))
	pin, shape := rollbackRestartPins[path], rollbackRestartShapes[path]
	rollbackRestartPins[path] = fmt.Sprintf("%x", sha256.Sum256(raw))
	rollbackRestartShapes[path] = string(raw)
	t.Cleanup(func() { rollbackRestartPins[path], rollbackRestartShapes[path] = pin, shape })
	findings, err := inspectRollbackRestartSource(path, raw)
	restartGuardMustFind(t, findings, err, diagnostic)
	joined := strings.Join(findings, "\n")
	if strings.Contains(joined, "byte pin changed") || strings.Contains(joined, "source binding/body/order changed") {
		t.Fatal("counterexample relied on supplemental source snapshots", findings)
	}
}

func TestRollbackRestartOwnedSourceContracts(t *testing.T) {
	sources := restartGuardSources(t)
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	// Deterministic diagnostics without hiding a later source behind the
	// first failure. Insertion sort avoids changing the import inventory.
	for i := 1; i < len(paths); i++ {
		for j := i; j > 0 && paths[j] < paths[j-1]; j-- {
			paths[j], paths[j-1] = paths[j-1], paths[j]
		}
	}
	for _, path := range paths {
		raw := sources[path]
		findings, err := inspectRollbackRestartSource(path, raw)
		if err != nil || len(findings) != 0 {
			t.Error(path, findings, err)
		}
		oldFindings, oldErr := inspectRollbackFaultSource(path, raw)
		if oldErr != nil || len(oldFindings) != 0 {
			t.Error("old adapter constraints changed", path, oldFindings, oldErr)
		}
		sqlFindings, sqlErr := inspectRollbackFaultSQLSource(path, raw)
		if sqlErr != nil || len(sqlFindings) != 0 {
			t.Error("old SQL constraints changed", path, sqlFindings, sqlErr)
		}
		if path == rollbackRestartNative || path == rollbackRestartModelTest {
			ordinary, ordinaryErr := inspectSource(path, raw)
			if ordinaryErr != nil || len(ordinary) != 0 {
				t.Error("ordinary policy cannot gain a whole-file exemption", path, ordinary, ordinaryErr)
			}
		}
	}
	// The real ordinary entry must reject the unchanged scalar row when copied.
	ordinary, ordinaryErr := inspectSource("copied/contract_test.go", sources[rollbackRestartModelTest])
	restartGuardMustFind(t, ordinary, ordinaryErr, "restricted reference Signal")
}

func TestRollbackRestartDoubleRepinnedSelectionAndStubCounterexamples(t *testing.T) {
	sources := restartGuardSources(t)
	for _, variant := range []struct{ name, path, old, replacement, finding string }{
		{"native OR", rollbackRestartNative, rollbackRestartTag, "//go:build linux || manual_fault_rollback || manual_restart_rollback", "exact restart AND build"},
		{"bridge fault only", rollbackRestartBridge, rollbackRestartTag, rollbackFaultManualTag, "exact restart AND build"},
		{"stub selected with restart", rollbackRestartStub, rollbackRestartStubTag, rollbackRestartTag, "exact restart AND build"},
		{"stub state", rollbackRestartStub, "package pager", "package pager\nvar hidden = 1", "fault-only stub"},
		{"stub environment", rollbackRestartStub, "package pager", "package pager\nimport \"os\"", "fault-only restart stub cannot import"},
		{"stub body", rollbackRestartStub, "func rollbackRestartBefore(ticket rollbackFaultTicket) {}", "func rollbackRestartBefore(ticket rollbackFaultTicket) { hidden() }", "fault-only stub must be exactly"},
		{"stub alternate ticket", rollbackRestartStub, "func rollbackRestartAfter(ticket rollbackFaultTicket)  {}", "func rollbackRestartAfter(ticket *rollbackFaultTicket) {}", "fault-only stub must be exactly"},
		{"model native import", rollbackRestartModel, "package rollbackrestart", "package rollbackrestart\nimport \"os\"", "descriptor-free"},
		{"model state", rollbackRestartModel, ")\n\nconst (", ")\n\nvar shared Plan\n\nconst (", "package state forbidden"},
		{"model physical PASS", rollbackRestartModel, "func (Observation) Pass() bool              { return false }", "func (Observation) Pass() bool { return true }", "physical validation or PASS"},
		{"model physical validation", rollbackRestartModel, "func (Observation) PhysicalValidated() bool { return false }", "func (Observation) PhysicalValidated() bool { return true }", "physical validation or PASS"},
		// All 44 Signal mutants repin both supplemental snapshots and must
		// fail independently; the original table rows remain above unchanged.
		{"native Signal call", rollbackRestartNative, `sum := sha256.Sum256(raw)`, `process.Signal(signal); sum := sha256.Sum256(raw)`, "control reference Signal"},
		{"native parenthesized Signal call", rollbackRestartNative, `sum := sha256.Sum256(raw)`, `(process.Signal)(signal); sum := sha256.Sum256(raw)`, "control reference Signal"},
		{"native retained Signal function", rollbackRestartNative, `sum := sha256.Sum256(raw)`, `send := process.Signal; send(signal); sum := sha256.Sum256(raw)`, "control reference Signal"},
		{"native Signal receiver alias", rollbackRestartNative, `sum := sha256.Sum256(raw)`, `alias := process; alias.Signal(signal); sum := sha256.Sum256(raw)`, "control reference Signal"},
		{"model Signal field called", rollbackRestartModel, `f.Process.Signal < 0`, `f.Process.Signal(9) < 0`, "control reference Signal"},
		{"model Signal parenthesized call", rollbackRestartModel, `f.Process.Signal < 0`, `(f.Process.Signal)(9) < 0`, "control reference Signal"},
		{"model Signal retained field", rollbackRestartModel, `func ValidateFrame(f Frame) Problem {`, `func ValidateFrame(f Frame) Problem { retained := f.Process.Signal; _ = retained;`, "control reference Signal"},
		{"model Signal receiver alias", rollbackRestartModel, `func ValidateFrame(f Frame) Problem {`, `func ValidateFrame(f Frame) Problem { alias := f; _ = alias.Process.Signal;`, "control reference Signal"},
		{"model Signal wrong receiver", rollbackRestartModel, `f.Process.Signal < 0`, `other.Process.Signal < 0`, "control reference Signal"},
		{"model Signal wrong parameter type", rollbackRestartModel, `func ValidateFrame(f Frame) Problem {`, `func ValidateFrame(f alternateFrame) Problem {`, "control reference Signal"},
		{"model Signal generic type shadow", rollbackRestartModel, `func ValidateFrame(f Frame) Problem {`, `func ValidateFrame[Frame any](f Frame) Problem {`, "control reference Signal"},
		{"model Signal function field type", rollbackRestartModel, `Signal        int`, `Signal        func(int) int`, "concrete model type chain"},
		{"model Signal pointer field type", rollbackRestartModel, `Signal        int`, `Signal        *int`, "concrete model type chain"},
		{"model Signal alternate field type", rollbackRestartModel, `Signal        int`, `Signal        uint8`, "concrete model type chain"},
		{"model Signal process pointer", rollbackRestartModel, `Process    ProcessEvidence`, `Process    *ProcessEvidence`, "concrete model type chain"},
		{"model Signal process alternate type", rollbackRestartModel, `Process    ProcessEvidence`, `Process    alternateProcess`, "concrete model type chain"},
		{"model Signal process alias", rollbackRestartModel, `type ProcessEvidence struct {`, `type ProcessEvidence = struct {`, "concrete model type chain"},
		{"model Signal Frame alias", rollbackRestartModel, `type Frame struct {`, `type Frame = struct {`, "concrete model type chain"},
		{"model Signal Frame generic shadow", rollbackRestartModel, `type Frame struct {`, `type Frame[ProcessEvidence any] struct {`, "concrete model type chain"},
		{"model Signal shadowed integer type", rollbackRestartModel, `type ProcessEvidence struct {`, `type int uint8; type ProcessEvidence struct {`, "concrete model type chain"},
		{"model Signal check inverted", rollbackRestartModel, `f.Process.Signal < 0`, `f.Process.Signal >= 0`, "control reference Signal"},
		{"model Signal check limit changed", rollbackRestartModel, `f.Process.Signal > 64`, `f.Process.Signal > 65`, "control reference Signal"},
		{"model Signal check duplicated", rollbackRestartModel, restartGuardSignalRangeStatement, restartGuardSignalRangeStatement + "; " + restartGuardSignalRangeStatement, "control reference Signal"},
		{"model Signal check nested", rollbackRestartModel, restartGuardSignalRangeStatement, "{ " + restartGuardSignalRangeStatement + " }", "control reference Signal"},
		{"model Signal moved function", rollbackRestartModel, `func ValidateFrame(f Frame) Problem {`, `func alternateValidateFrame(f Frame) Problem {`, "control reference Signal"},
		{"self Signal receiver alias", rollbackRestartModel, `func validSelf(p ProcessEvidence, spec PlanSpec) bool {`, `func validSelf(original ProcessEvidence, spec PlanSpec) bool { p := original;`, "control reference Signal"},
		{"self Signal parameter type", rollbackRestartModel, `func validSelf(p ProcessEvidence, spec PlanSpec) bool {`, `func validSelf(p alternateProcess, spec PlanSpec) bool {`, "control reference Signal"},
		{"spawn Signal parameter type", rollbackRestartModel, `func validSpawn(p ProcessEvidence, spec PlanSpec) bool {`, `func validSpawn(p alternateProcess, spec PlanSpec) bool {`, "control reference Signal"},
		{"test Signal field called", rollbackRestartModelTest, `f.Process.State, f.Process.Signal = ProcessSignaled, 9`, `f.Process.Signal(9)`, "control reference Signal"},
		{"test Signal parenthesized call", rollbackRestartModelTest, `f.Process.State, f.Process.Signal = ProcessSignaled, 9`, `(f.Process.Signal)(9)`, "control reference Signal"},
		{"test Signal retained function", rollbackRestartModelTest, `f.Process.State, f.Process.Signal = ProcessSignaled, 9`, `send := f.Process.Signal; send(9)`, "control reference Signal"},
		{"test Signal receiver alias", rollbackRestartModelTest, `f.Process.State, f.Process.Signal = ProcessSignaled, 9`, `alias := f; alias.Process.State, alias.Process.Signal = ProcessSignaled, 9`, "control reference Signal"},
		{"test Signal wrong receiver", rollbackRestartModelTest, `f.Process.State, f.Process.Signal = ProcessSignaled, 9`, `f.Process.State, other.Process.Signal = ProcessSignaled, 9`, "control reference Signal"},
		{"test Signal wrong closure type", rollbackRestartModelTest, restartGuardSignalTestRow, `{"signal-is-not-target", AfterJUnlink, WriterWaitObserved, func(f *alternateFrame) { f.Process.State, f.Process.Signal = ProcessSignaled, 9 }}`, "control reference Signal"},
		{"test Signal wrong table type", rollbackRestartModelTest, `mutate func(*Frame)`, `mutate func(*alternateFrame)`, "control reference Signal"},
		{"test Signal wrong row name", rollbackRestartModelTest, `"signal-is-not-target"`, `"unrelated-row"`, "control reference Signal"},
		{"test Signal wrong case", rollbackRestartModelTest, restartGuardSignalTestRow, `{"signal-is-not-target", BeforeJCreate, WriterWaitObserved, func(f *Frame) { f.Process.State, f.Process.Signal = ProcessSignaled, 9 }}`, "control reference Signal"},
		{"test Signal wrong frame kind", rollbackRestartModelTest, restartGuardSignalTestRow, `{"signal-is-not-target", AfterJUnlink, TargetReached, func(f *Frame) { f.Process.State, f.Process.Signal = ProcessSignaled, 9 }}`, "control reference Signal"},
		{"test Signal standalone write", rollbackRestartModelTest, `f.Process.State, f.Process.Signal = ProcessSignaled, 9`, `f.Process.Signal = 9`, "control reference Signal"},
		{"test Signal duplicate row", rollbackRestartModelTest, restartGuardSignalTestRow, restartGuardSignalTestRow + ", " + restartGuardSignalTestRow, "control reference Signal"},
		{"test Signal nested assignment", rollbackRestartModelTest, `f.Process.State, f.Process.Signal = ProcessSignaled, 9`, `{ f.Process.State, f.Process.Signal = ProcessSignaled, 9 }`, "control reference Signal"},
		{"test Signal moved function", rollbackRestartModelTest, `func TestRestartRejectsProtocolAndAdapterFalsePositives(t *testing.T) {`, `func TestAlternateSignalUse(t *testing.T) {`, "control reference Signal"},
		{"test Signal generic type shadow", rollbackRestartModelTest, `func TestRestartRejectsProtocolAndAdapterFalsePositives(t *testing.T) {`, `func TestRestartRejectsProtocolAndAdapterFalsePositives[Frame any](t *testing.T) {`, "control reference Signal"},
		{"test Signal preceding shadow", rollbackRestartModelTest, `func TestRestartRejectsProtocolAndAdapterFalsePositives(t *testing.T) {`, `func TestRestartRejectsProtocolAndAdapterFalsePositives(t *testing.T) { type Frame struct { Process alternateProcess };`, "control reference Signal"},
		{"test Signal model type replacement", rollbackRestartModelTest, `func TestRestartRejectsProtocolAndAdapterFalsePositives(t *testing.T) {`, `type Frame struct { Process alternateProcess }; func TestRestartRejectsProtocolAndAdapterFalsePositives(t *testing.T) {`, "control reference Signal"},
	} {
		t.Run(variant.name, func(t *testing.T) {
			restartGuardRepinned(t, variant.path, sources[variant.path], variant.old, variant.replacement, variant.finding)
			if variant.finding == "control reference Signal" && (variant.path == rollbackRestartModelTest || variant.path == rollbackRestartNative) {
				// Calls, method values, aliases and moved/duplicate rows must also
				// fail the ordinary entry, independently of the restart subguard.
				raw := []byte(strings.Replace(string(sources[variant.path]), variant.old, variant.replacement, 1))
				ordinary, ordinaryErr := inspectSource(variant.path, raw)
				restartGuardMustFind(t, ordinary, ordinaryErr, "restricted reference Signal")
			}
		})
	}
}

func TestRollbackRestartOutsideConsumersAndAliasesRejected(t *testing.T) {
	for _, variant := range []struct{ path, source, finding string }{
		{"internal/pager/rogue.go", `package pager; var observer = rollbackRestartBefore`, "restart declaration/reference outside exact"},
		{"sqlite/rogue_test.go", `package sqlite; func TestRogue(){ pager.ArmManualRollbackRestart(tx.storage,plan,channel) }`, "restart declaration/reference outside exact"},
		{"sqlite/rogue_test.go", `package sqlite; var capture = pager.CaptureManualRollbackRestartCommit`, "restart declaration/reference outside exact"},
		{"internal/pager/rogue.go", `package pager; var finish = CompleteManualRollbackRestartControl`, "restart declaration/reference outside exact"},
		{"sqlite/rogue_test.go", rollbackFaultManualTag + "\npackage sqlite; func TestRogue(){ pager.NewManualRollbackRestartChannel(plan,role) }", "restart declaration/reference outside exact"},
		{"internal/pager/rogue_linux.go", rollbackFaultManualTag + "\npackage pager; var arm = ArmManualRollbackRestart", "restart declaration/reference outside exact"},
		{"internal/pager/rogue.go", `package pager; func rollbackRestartBefore(ticket rollbackFaultTicket){}`, "restart declaration/reference outside exact"},
		{"sqlite/rogue_test.go", `package sqlite; func TestRogue(){ manualRestartAdmit(nil,role) }`, "restart declaration/reference outside exact"},
		{"sqlite/rogue_test.go", rollbackRestartTag + "\npackage sqlite; func TestManualRollbackRestartWriter(t *testing.T) {}", "restart tag outside exact"},
		{"internal/pager/rollback_restart_copied.go", `package pager; func rogue(){}`, "unreviewed restart source inventory"},
		{"internal/testkit/rollbackrestart/extra.go", `package rollbackrestart; var arbitrary = 1`, "unreviewed restart source inventory"},
		{"sqlite/rogue.go", "package sqlite\nimport \"" + rollbackRestartImport + "\"", "restart model import outside exact"},
		{rollbackRestartNative, "package sqlite\nimport rr \"" + rollbackRestartImport + "\"", "restart model import outside exact"},
		{rollbackRestartBridge, rollbackRestartTag + "\npackage pager\nimport \"os\"\nfunc rogue(){ os.Exit(86) }", "native action outside exact role"},
		{rollbackRestartNative, rollbackRestartTag + "\npackage sqlite\nimport \"os/exec\"\nfunc rogue(){ exec.Command(\"sh\",\"-c\",code) }", "native action outside exact role"},
	} {
		findings, err := inspectRollbackRestartSource(variant.path, []byte(variant.source))
		restartGuardMustFind(t, findings, err, variant.finding)
	}
	// Relocated manual sources retain a protected tag/name and must be
	// rejected by this checker alone. Relocated pure sources are not manual
	// capabilities: their exact package inventory and external consumers are
	// checked above; the composed older rollbackfault guard owns their import.
	for path, raw := range restartGuardSources(t) {
		if !restartGuardManual(path) {
			continue
		}
		findings, err := inspectRollbackRestartSource("copied/"+filepath.Base(path), raw)
		if err != nil || len(findings) == 0 {
			t.Fatal("whole copied restart source admitted", path, findings, err)
		}
	}
}

func TestRollbackRestartOldCoreAndMutexWrapperStayExact(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", rollbackFaultManual))
	if err != nil {
		t.Fatal(err)
	}
	findings, err := inspectRollbackRestartSource(rollbackFaultManual, raw)
	if err != nil || len(findings) != 0 {
		t.Fatal(findings, err)
	}
	for _, variant := range []struct{ old, replacement, finding string }{
		{"ticket := rollbackFaultBeforeCore(t, point, f, path, off, length)", "ticket := rollbackFaultBeforeCore(t, point, nil, path, off, length)", "original core must finish"},
		{"rollbackRestartBefore(ticket)", "go rollbackRestartBefore(ticket)", "original core must finish"},
		{"rollbackRestartAfter(ticket)", "rollbackRestartAfter(rollbackFaultTicket{})", "original core must finish"},
		{"rollbackFaultAfterCore(ticket, executed, nativeN, nativeErr, returnedN, returnedErr, normalizedErr, effect)", "rollbackFaultAfterCore(ticket, executed, nativeN, nil, returnedN, returnedErr, normalizedErr, effect)", "original core must finish"},
		{"s.inFlight = true", "s.inFlight = false", "original adapter core body"},
		{"s.inFlight = false\n\ts.append(rollbackfault.Event{Kind: rollbackfault.Returned", "s.inFlight = true\n\ts.append(rollbackfault.Event{Kind: rollbackfault.Returned", "original adapter core body"},
		{"\tdefer s.mu.Unlock()\n\tp, index := rollbackFaultPoint(point)", "\tdefer s.mu.Unlock()\n\trollbackRestartBefore(rollbackFaultTicket{}); p, index := rollbackFaultPoint(point)", "original adapter core body"},
	} {
		if !strings.Contains(string(raw), variant.old) {
			t.Fatal("core mutation anchor missing", variant.old)
		}
		changed := []byte(strings.Replace(string(raw), variant.old, variant.replacement, 1))
		observed, parseErr := inspectRollbackRestartSource(rollbackFaultManual, changed)
		restartGuardMustFind(t, observed, parseErr, variant.finding)
	}
}

func TestRollbackRestartWiringPreservesAllPoliciesAndReviewHold(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	// The independent whole-main review releases only these exact source bytes.
	// The exact scalar forwarding is reviewed too; unreviewed main bytes HOLD.
	const reviewedWholeMain = "6dbab48ff374858bfafe578f0fca99f5176ced7b6bdc008bec04ce6a6d1d0eb2"
	const priorOrdinaryBody = "98341345e43f39b39dbc78eaf49a916a0701f3d7f2f383d4949b9a3ed439d199"
	if walPublicationFrozenSources["internal/testscope/main.go"] != reviewedWholeMain || formatUpgradePins["internal/testscope/main.go"] != reviewedWholeMain || readOnlyWALManualPriorOrdinarySHA256 != priorOrdinaryBody {
		t.Fatal("whole-main/body guard pins differ from independently reviewed literals")
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != reviewedWholeMain {
		t.Fatal("main.go bytes differ from independently reviewed literal")
	}
	if findings := inspectRollbackRestartWiring(raw); len(findings) != 0 {
		t.Fatal("independently reviewed restart wiring rejected", findings)
	}
	if findings := inspectWALPublicationFrozenSource("internal/testscope/main.go", raw); len(findings) != 0 {
		t.Fatal("independently reviewed whole-main source rejected", findings)
	}
	unreviewed := append(append([]byte(nil), raw...), []byte("\n// unreviewed restart main.go change\n")...)
	if findings := inspectRollbackRestartWiring(unreviewed); len(findings) != 1 || !strings.Contains(findings[0], rollbackRestartMainReview) {
		t.Fatal("unreviewed main.go comment must retain restart HOLD", findings)
	}
	if findings := inspectWALPublicationFrozenSource("internal/testscope/main.go", unreviewed); len(findings) == 0 {
		t.Fatal("unreviewed main.go comment accepted by existing whole-main guard")
	}
	for _, variant := range []struct{ old, replacement, finding string }{
		{"findings = append(findings, restart...)", "_ = restart", "direct all-source hook"},
		{"return restartErr", "return nil", "direct all-source hook"},
		{"fmt.Fprintln(os.Stderr, restartContractErr)", "fmt.Fprintln(os.Stderr, nil)", "contract errors and findings"},
		{"for _, finding := range restartContracts", "for _, finding := range []string{}", "contract errors and findings"},
		{"findings = append(findings, rollbackSQL...)", "_ = rollbackSQL", "previous guard wiring weakened"},
		{"return rollbackSQLErr", "return nil", "previous guard wiring weakened"},
		{"findings = append(findings, ordinary...)", "_ = ordinary", "previous guard wiring weakened"},
		{"inspectRollbackRestartSource(path, raw)", "inspectRollbackFaultSource(path, raw)", "missing/duplicate prior or restart"},
	} {
		if !strings.Contains(string(raw), variant.old) {
			t.Fatal("wiring mutation anchor missing", variant.old)
		}
		changed := []byte(strings.Replace(string(raw), variant.old, variant.replacement, 1))
		restartGuardMustFind(t, inspectRollbackRestartWiring(changed), nil, variant.finding)
	}
}

func TestRollbackRestartDoubleRepinnedNativeProtocolCounterexamples(t *testing.T) {
	sources := restartGuardSources(t)
	for _, variant := range []struct{ name, path, old, replacement, finding string }{
		{"extra trial directory Sync", rollbackRestartNative, `sum := sha256.Sum256(raw)`, `manualRestartSyncDirectory(c, 1); sum := sha256.Sum256(raw)`, "effect helper outside exact direct consumer"},
		{"duplicate approved directory Sync", rollbackRestartNative, `if err := manualRestartSyncDirectory(c, family);`, `manualRestartSyncDirectory(c, family); if err := manualRestartSyncDirectory(c, family);`, "effect helper exact call count"},
		{"wrong directory family", rollbackRestartNative, `manualRestartSyncDirectory(c, family)`, `manualRestartSyncDirectory(c, 1)`, "effect helper outside exact direct consumer"},
		{"extra fixed artifact helper", rollbackRestartNative, `sum := sha256.Sum256(raw)`, `manualRestartCreate(c, 1, "trial.db", raw); sum := sha256.Sum256(raw)`, "effect helper outside exact direct consumer"},
		{"extra SQL config create", rollbackRestartNative, `sum := sha256.Sum256(raw)`, `c.sql.create("arbitrary", raw, false); sum := sha256.Sum256(raw)`, "I/O outside exact file/function/call"},
		{"duplicate approved SQL create", rollbackRestartNative, `info, err := c.family[family].create(name, raw, false)`, `c.family[family].create(name, raw, false); info, err := c.family[family].create(name, raw, false)`, "exact I/O call count"},
		{"extra SQL ledger close", rollbackRestartNative, `sum := sha256.Sum256(raw)`, `c.sql.ledger.close(1); sum := sha256.Sum256(raw)`, "I/O outside exact file/function/call"},
		{"SQL ledger close value", rollbackRestartNative, `closeErr := c.sql.ledger.close(i)`, `closeOwner := c.sql.ledger.close; closeErr := closeOwner(i)`, "I/O outside exact file/function/call"},
		{"extra transaction write", rollbackRestartNative, `result, err := tx.Exec(ctx, statement)`, `tx.Exec(ctx, "DELETE FROM t"); result, err := tx.Exec(ctx, statement)`, "I/O outside exact file/function/call"},
		{"native callback field", rollbackRestartNative, `type manualRestartConfig struct {`, `type manualRestartConfig struct { callback func()`, "native closed ownership type"},
		{"native any field", rollbackRestartNative, `type manualRestartConfig struct {`, `type manualRestartConfig struct { executor any`, "native closed ownership type"},
		{"native interface field", rollbackRestartNative, `type manualRestartConfig struct {`, `type manualRestartConfig struct { executor interface { Run() }`, "native closed ownership type"},
		{"native alternate channel", rollbackRestartNative, `type manualRestartConfig struct {`, `type manualRestartConfig struct { done chan error`, "single actual Wait owner"},
		{"native function local", rollbackRestartNative, `sum := sha256.Sum256(raw)`, `var callback func(); callback(); sum := sha256.Sum256(raw)`, "function/callback type"},
		{"native arbitrary raw write", rollbackRestartNative, `sum := sha256.Sum256(raw)`, `os.WriteFile("arbitrary", raw, 0600); sum := sha256.Sum256(raw)`, "I/O outside exact file/function/call"},
		{"native arbitrary owner write", rollbackRestartNative, `sum := sha256.Sum256(raw)`, `someFile.Write(raw); sum := sha256.Sum256(raw)`, "I/O outside exact file/function/call"},
		{"native arbitrary owner read", rollbackRestartNative, `sum := sha256.Sum256(raw)`, `someFile.Read(raw); sum := sha256.Sum256(raw)`, "I/O outside exact file/function/call"},
		{"native duplicate grant write", rollbackRestartNative, `count, err := child.controls.Write(raw)`, `child.controls.Write(raw); count, err := child.controls.Write(raw)`, "exact I/O call count"},
		{"native raw write method value", rollbackRestartNative, `count, err := child.controls.Write(raw)`, `write := child.controls.Write; count, err := write(raw)`, "I/O outside exact file/function/call"},
		{"native alternate syscall", rollbackRestartNative, `sum := sha256.Sum256(raw)`, `syscall.Pwrite(9, raw, 0); sum := sha256.Sum256(raw)`, "closed package selector inventory"},
		{"bridge arbitrary raw write", rollbackRestartBridge, `n, err := c.events.Write(raw)`, `os.WriteFile("arbitrary", raw, 0600); n, err := c.events.Write(raw)`, "I/O outside exact file/function/call"},
		{"bridge duplicate event write", rollbackRestartBridge, `n, err := c.events.Write(raw)`, `c.events.Write(raw); n, err := c.events.Write(raw)`, "exact I/O call count"},
		{"bridge metadata owner bypass", rollbackRestartBridge, `vfs.SameFile(controlInfo, eventInfo)`, `os.SameFile(controlInfo, eventInfo)`, "I/O outside exact file/function/call"},
		{"bridge metadata wrong wrapper", rollbackRestartBridge, `vfs.SameFile(controlInfo, eventInfo)`, `rollbackfault.SameFile(controlInfo, eventInfo)`, "I/O outside exact file/function/call"},
		{"env effect first", rollbackRestartNative, `if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK")`, `_ = os.Getpid(); if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK")`, "admission gates must dominate"},
		{"writer role swapped", rollbackRestartNative, `manualRestartAdmit(&config, rollbackrestart.Writer)`, `manualRestartAdmit(&config, rollbackrestart.Controller)`, "root first action"},
		{"selector union", rollbackRestartNative, `"^TestManualRollbackRestartWriter$"`, `"TestManualRollbackRestartWriter|TestOther"`, "single-role test selector"},
		{"count retry", rollbackRestartNative, `"-test.count=1"`, `"-test.count=2"`, "single-role test selector"},
		{"timeout kill window", rollbackRestartNative, `"-test.timeout=0"`, `"-test.timeout=30s"`, "single-role test selector"},
		{"extra argv accepted", rollbackRestartNative, `len(os.Args) != len(want)+1`, `len(os.Args) < len(want)+1`, "reject extra/repeated"},
		{"image argv substitution", rollbackRestartNative, `os.Open("/proc/self/exe")`, `os.Open(os.Args[0])`, "running /proc/self/exe"},
		{"image samefile omitted", rollbackRestartNative, `!os.SameFile(before, targetInfo)`, `false`, "running /proc/self/exe"},
		{"spawn foreign binary", rollbackRestartNative, `exec.Command(c.binding.BinaryPath, args...)`, `exec.Command(otherBinary, args...)`, "native action outside exact role"},
		{"spawn command context", rollbackRestartNative, `exec.Command(c.binding.BinaryPath, args...)`, `exec.CommandContext(ctx, c.binding.BinaryPath, args...)`, "forbidden ownership/process"},
		{"spawn shell", rollbackRestartNative, `exec.Command(c.binding.BinaryPath, args...)`, `exec.Command("sh", "-c", c.binding.BinaryPath)`, "native action outside exact role"},
		{"database descriptor inherited", rollbackRestartNative, `command.ExtraFiles = []*os.File{controlRead, eventWrite}`, `command.ExtraFiles = []*os.File{controlRead, eventWrite, databaseOwner}`, "parent only one actual process"},
		{"second role launch", rollbackRestartNative, `if c.children[slot] != nil`, `if false`, "parent only one actual process"},
		{"premature recovery", rollbackRestartNative, `!c.children[0].waited`, `false`, "parent only one actual process"},
		{"callback Wait", rollbackRestartNative, `waitErr := command.Wait()`, `wait := command.Wait; waitErr := wait()`, "callback/closure execution"},
		{"exit zero accepted", rollbackRestartNative, `code = rollbackrestart.ExpectedExitCode`, `code = 0`, "exact observed exit state"},
		{"signal accepted", rollbackRestartNative, `!actual.state.Exited() || actual.state.ExitCode() != code`, `actual.state.ExitCode() != code`, "exact observed exit state"},
		{"no terminal EOF", rollbackRestartNative, `n != 0 || tailErr != io.EOF`, `n != 0`, "exact observed exit state"},
		{"self child PID", rollbackRestartNative, `frame.Process.PID != int64(child.command.Process.Pid)`, `frame.Process.PID != frame.Process.PID`, "owned child identity"},
		{"unbounded receive", rollbackRestartNative, `count > rollbackrestart.MaxFrameBytes`, `count > 1<<24`, "bounded complete event read"},
		{"stale generation admitted", rollbackRestartNative, `frame.Sender != child.identity || `, ``, "owned child identity"},
		{"grant sequence replay", rollbackRestartNative, `frame.Sequence != child.sequence+1`, `frame.Sequence < child.sequence+1`, "owned child identity"},
		{"ack without Commit nil", rollbackRestartNative, `commit.Commit.Value != rollbackrestart.CommitNil`, `false`, "only parent owns exclusive raw"},
		{"ack digest rewritten", rollbackRestartNative, `SHA256: manualRestartDigest(raw)`, `SHA256: hash`, "only parent owns exclusive raw"},
		{"fake ack directory close", rollbackRestartNative, `DirectoryCloseStep: 5`, `DirectoryCloseStep: 0`, "only parent owns exclusive raw"},
		{"public Commit error substituted", rollbackRestartNative, `pager.CaptureManualRollbackRestartCommit(scope, commitErr, tx.done, db.tx == nil, tx.activeRows)`, `pager.CaptureManualRollbackRestartCommit(scope, nil, tx.done, db.tx == nil, tx.activeRows)`, "immediately followed by unchanged capture"},
		{"public Commit capture delayed", rollbackRestartNative, `captured := pager.CaptureManualRollbackRestartCommit(`, `_ = commitErr; captured := pager.CaptureManualRollbackRestartCommit(`, "immediately followed by unchanged capture"},
		{"method value Commit", rollbackRestartNative, `commitErr := tx.Commit(ctx)`, `commit := tx.Commit; commitErr := commit(ctx)`, "public Commit outside unique writer"},
		// Scalar receipt exceptions are tested in their actual native/bridge
		// contexts, with both supplemental snapshots repinned by the helper.
		{"wrong public Commit receiver", rollbackRestartNative, `commitErr := tx.Commit(ctx)`, `commitErr := other.Commit(ctx)`, "public Commit outside unique writer"},
		{"parenthesized public Commit value", rollbackRestartNative, `commitErr := tx.Commit(ctx)`, `commitErr := (tx.Commit)(ctx)`, "public Commit outside unique writer"},
		{"duplicate public Commit", rollbackRestartNative, `commitErr := tx.Commit(ctx)`, `tx.Commit(ctx); commitErr := tx.Commit(ctx)`, "finite native/helper call count changed: tx.Commit"},
		{"ACK Commit field called", rollbackRestartNative, `commit.Commit.Value`, `commit.Commit().Value`, "public Commit outside unique writer"},
		{"ACK Commit field parenthesized call", rollbackRestartNative, `commit.Commit.Value`, `(commit.Commit)().Value`, "public Commit outside unique writer"},
		{"ACK Commit field retained", rollbackRestartNative, `out := rollbackrestart.AckEvidence{}`, `out := rollbackrestart.AckEvidence{}; retained := commit.Commit; _ = retained`, "public Commit outside unique writer"},
		{"ACK Commit wrong receiver", rollbackRestartNative, `commit.Commit.Value`, `other.Commit.Value`, "public Commit outside unique writer"},
		{"ACK Commit receiver alias", rollbackRestartNative, `out := rollbackrestart.AckEvidence{}`, `out := rollbackrestart.AckEvidence{}; alias := commit; _ = alias.Commit.Value`, "public Commit outside unique writer"},
		{"ACK Commit parameter type", rollbackRestartNative, `commit rollbackrestart.Frame`, `commit *Tx`, "public Commit outside unique writer"},
		{"ACK Commit scalar assignment", rollbackRestartNative, `out := rollbackrestart.AckEvidence{}`, `out := rollbackrestart.AckEvidence{}; commit.Commit = rollbackrestart.CommitEvidence{}`, "public Commit outside unique writer"},
		{"ACK Commit comparison inverted", rollbackRestartNative, `commit.Commit.Value != rollbackrestart.CommitNil`, `commit.Commit.Value == rollbackrestart.CommitNil`, "public Commit outside unique writer"},
		{"failure Commit source called", rollbackRestartNative, `observation.Commit, observation.Problem`, `observation.Commit(), observation.Problem`, "public Commit outside unique writer"},
		{"failure Commit source retained", rollbackRestartNative, `observation := frozenReceipt.Seal()`, `observation := frozenReceipt.Seal(); retained := observation.Commit; _ = retained`, "public Commit outside unique writer"},
		{"failure Commit source alias", rollbackRestartNative, `observation := frozenReceipt.Seal()`, `original := frozenReceipt.Seal(); observation := original`, "public Commit outside unique writer"},
		{"failure Commit source wrong receiver", rollbackRestartNative, `observation.Commit, observation.Problem`, `other.Commit, observation.Problem`, "public Commit outside unique writer"},
		{"failure Commit destination wrong receiver", rollbackRestartNative, `detail.Commit, detail.ProtocolProblem`, `other.Commit, detail.ProtocolProblem`, "public Commit outside unique writer"},
		{"failure Commit assignment reversed", rollbackRestartNative, `detail.Commit, detail.ProtocolProblem = observation.Commit, observation.Problem`, `observation.Commit, observation.Problem = detail.Commit, detail.ProtocolProblem`, "public Commit outside unique writer"},
		{"failure Commit function field", rollbackRestartNative, `Commit          rollbackrestart.CommitValue`, `Commit          func() rollbackrestart.CommitValue`, "native closed ownership type"},
		{"failure Commit pointer field", rollbackRestartNative, `Commit          rollbackrestart.CommitValue`, `Commit          *rollbackrestart.CommitValue`, "native closed ownership type"},
		{"failure Commit moved to digest", rollbackRestartNative, `sum := sha256.Sum256(raw)`, `var detail manualRestartFailureRecord; var observation rollbackrestart.Observation; detail.Commit, detail.ProtocolProblem = observation.Commit, observation.Problem; sum := sha256.Sum256(raw)`, "public Commit outside unique writer"},
		{"bridge Commit source called", rollbackRestartBridge, `frame.Commit = captured.Evidence`, `frame.Commit = captured.Evidence; frame.Commit()`, "public Commit outside unique writer"},
		{"bridge Commit retained", rollbackRestartBridge, `frame.Commit = captured.Evidence`, `frame.Commit = captured.Evidence; retained := frame.Commit; _ = retained`, "public Commit outside unique writer"},
		{"bridge Commit wrong receiver", rollbackRestartBridge, `frame.Commit = captured.Evidence`, `other.Commit = captured.Evidence`, "public Commit outside unique writer"},
		{"bridge Commit receiver alias", rollbackRestartBridge, `frame := c.frame(rollbackrestart.PublicCommitReturned)`, `original := c.frame(rollbackrestart.PublicCommitReturned); frame := original`, "public Commit outside unique writer"},
		{"bridge Commit parameter type", rollbackRestartBridge, `CommitReturned(captured ManualRollbackRestartCommit)`, `CommitReturned(captured *ManualRollbackRestartCommit)`, "public Commit outside unique writer"},
		{"bridge Commit reversed assignment", rollbackRestartBridge, `frame.Commit = captured.Evidence`, `captured.Evidence = frame.Commit`, "public Commit outside unique writer"},
		{"bridge Commit duplicate assignment", rollbackRestartBridge, `frame.Commit = captured.Evidence`, `frame.Commit = captured.Evidence; frame.Commit = captured.Evidence`, "public Commit outside unique writer"},
		{"bridge Commit alternate frame origin", rollbackRestartBridge, `frame(kind rollbackrestart.FrameKind) rollbackrestart.Frame`, `frame(kind rollbackrestart.FrameKind) alternateFrame`, "public Commit outside unique writer"},
		{"bridge Commit captured function field", rollbackRestartBridge, `Evidence    rollbackrestart.CommitEvidence`, `Evidence    func() rollbackrestart.CommitEvidence`, "bridge closed ownership type"},
		{"bridge Commit moved to observer", rollbackRestartBridge, `event := events[count-1]`, `var frame rollbackrestart.Frame; var captured ManualRollbackRestartCommit; frame.Commit = captured.Evidence; event := events[count-1]`, "public Commit outside unique writer"},
		{"native Commit model import alias", rollbackRestartNative, `"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackrestart"`, `rollbackrestart "github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackrestart"`, "public Commit outside unique writer"},
		{"native Commit foreign model import", rollbackRestartNative, `"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackrestart"`, `"example.invalid/rollbackrestart"`, "public Commit outside unique writer"},
		{"bridge Commit model import alias", rollbackRestartBridge, `"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackrestart"`, `rollbackrestart "github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackrestart"`, "public Commit outside unique writer"},
		{"bridge Commit foreign model import", rollbackRestartBridge, `"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackrestart"`, `"example.invalid/rollbackrestart"`, "public Commit outside unique writer"},
		{"bridge Commit channel receiver type", rollbackRestartBridge, `(c *ManualRollbackRestartChannel) CommitReturned`, `(c *alternateChannel) CommitReturned`, "public Commit outside unique writer"},
		{"failure Commit destination type", rollbackRestartNative, `detail := manualRestartFailureRecord{`, `detail := alternateRecord{`, "public Commit outside unique writer"},
		{"failure Commit manufactured observation", rollbackRestartNative, `observation := frozenReceipt.Seal()`, `observation := rollbackrestart.Observation{}`, "public Commit outside unique writer"},
		{"ACK Commit receiver shadow", rollbackRestartNative, `out := rollbackrestart.AckEvidence{}`, `out := rollbackrestart.AckEvidence{}; { var commit rollbackrestart.Frame; _ = commit.Commit.Value }`, "public Commit outside unique writer"},
		{"writer reuses adapter", rollbackRestartNative, `state.Groups == 0 && state.Pending == 0 && !state.AdapterActive && !state.AdapterUsed`, `state.Groups == 0 && state.Pending == 0`, "fresh process must have unused"},
		{"recovery original family", rollbackRestartNative, `filepath.Join(c.root, "replay", "replay.db")`, `filepath.Join(c.root, "trial", "trial.db")`, "one bounded replay copy"},
		{"recovery all cases", rollbackRestartNative, `if c.plan.Spec().Binding.Case == rollbackrestart.AfterTablePage {`, `if true {`, "one bounded replay copy"},
		{"recovery extra records", rollbackRestartNative, `MaxRecords: 3`, `MaxRecords: 4`, "one bounded replay copy"},
		{"recovery wrong SQL expectation", rollbackRestartNative, `rollbackrestart.ExpectedSQLState(c.plan.Spec().Binding.Case)`, `rollbackfault.SQLNew`, "one bounded replay copy"},
		{"original freeze before Wait", rollbackRestartNative, `!c.active.waited || `, ``, "actual writer Wait before preserving"},
		{"copy alias tolerated", rollbackRestartNative, `a.Before.File == evidence.Artifacts[j].Before.File`, `false`, "actual writer Wait before preserving"},
		{"failure loses parent prefix", rollbackRestartNative, `detail.RecordedFrames = append([]rollbackrestart.Frame(nil), frames[:count]...)`, `detail.RecordedFrames = nil`, "raw parent prefix and independent"},
		{"failure invents ACK", rollbackRestartNative, `detail.DurableAck = c.durableAck`, `detail.DurableAck = rollbackrestart.AckEvidence{Exclusive: true}`, "raw parent prefix and independent"},
		{"timeout no persisted knowledge", rollbackRestartNative, `manualRestartPersistFailure(t, c, fmt.Errorf("CHILD_LIVE/WAIT_UNKNOWN: actual Wait exceeded observation deadline"))`, `t.Log("timeout")`, "deadline persists uncertain prefix"},
		{"failure retry", rollbackRestartNative, "if c.failureAttempted {\n\t\treturn\n\t}", `if false { return }`, "failure snapshot must be attempted once"},
		{"grant receipt published before write", rollbackRestartNative, `preview := *receipt`, `preview := *receipt; *receipt = preview`, "unsent or partial grant"},
		{"grant receipt published on short write", rollbackRestartNative, "if err != nil {\n\t\tchild.grantError = err.Error()\n\t\treturn err\n\t}", `if err != nil { child.grantError = err.Error() }`, "unsent or partial grant"},
		{"result absent from final manifest", rollbackRestartNative, `name == "result.json" || `, ``, "fixed O_EXCL artifact helper"},
		{"final manifest not rechecked", rollbackRestartNative, `name == "final-manifest.json"`, `name == "unreviewed.json"`, "fixed O_EXCL artifact helper"},
		{"trailing bytes discarded", rollbackRestartNative, `child.rawFrames[child.rawCount] = append([]byte(nil), trailing[:n]...)`, `child.rawFrames[child.rawCount] = nil`, "retain observed trailing bytes"},
		{"transport diagnostic omitted", rollbackRestartBridge, "rollbackRestartTransport(err)\n\t\trollbackRestartDiagnostic(problemErr, diagnostics)\n\t\trollbackRestartAbort()", `rollbackRestartDiagnostic(problemErr, diagnostics); rollbackRestartAbort()`, "transport failure preserves original"},
		{"bridge gate weakened", rollbackRestartBridge, `os.Getenv("GOSQLITE_MANUAL_RESTART_ROLLBACK") != "independently_authorized"`, `false`, "bridge first action"},
		{"arbitrary inherited descriptor", rollbackRestartBridge, `os.NewFile(3, "restart-parent-control")`, `os.NewFile(9, "restart-parent-control")`, "fixed child role and two concrete"},
		{"restart fault action", rollbackRestartBridge, `rollbackfault.SQLControl)`, `rollbackfault.SQLDBSyncBefore)`, "fixed successful three-page SQL armer"},
		{"scope mismatch ignored", rollbackRestartBridge, `ticket.scope != scope.fault || `, ``, "observer same-ticket scalar"},
		{"observer while locked", rollbackRestartBridge, "diagnostics := fault.diagnostics\n\tfault.mu.Unlock()", "diagnostics := fault.diagnostics\n\tscope.channel.send(scope.channel.frame(rollbackrestart.AdapterObserved))\n\tfault.mu.Unlock()", "observer same-ticket scalar"},
		{"target wrong phase", rollbackRestartBridge, `which == rollbackrestart.BeforeJCreate && kind == rollbackfault.Intent`, `which == rollbackrestart.BeforeJCreate && kind == rollbackfault.Returned`, "exact owned target phase"},
		{"target wrong offset", rollbackRestartBridge, `ticket.call.Offset == 4096`, `ticket.call.Offset == 8192`, "exact owned target phase"},
		{"target wrong occurrence", rollbackRestartBridge, `ticket.call.Occurrence == 2`, `ticket.call.Occurrence == 3`, "exact owned target phase"},
		{"underlying error passes", rollbackRestartBridge, `event.Result.NativeError != rollbackfault.None || `, ``, "no target after I/O failure"},
		{"unbound EXIT", rollbackRestartBridge, `grant.Digest != targetHash || `, ``, "exit86 requires the exact target"},
		{"wrong crash code", rollbackRestartBridge, `os.Exit(86)`, `os.Exit(1)`, "native action outside exact role"},
		{"ack close omitted", rollbackRestartBridge, `ack.DirectoryCloseCalls != 1`, `false`, "public nil frame precedes durable"},
		{"normal finish crash prefix", rollbackRestartBridge, `scope.channel.plan.Spec().Binding.Case != rollbackrestart.Control`, `false`, "normal-control lifecycle"},
		{"bridge callback retained", rollbackRestartBridge, `type manualRollbackRestartScope struct {`, `type manualRollbackRestartScope struct { callback func()`, "closed ownership type"},
	} {
		t.Run(variant.name, func(t *testing.T) {
			restartGuardRepinned(t, variant.path, sources[variant.path], variant.old, variant.replacement, variant.finding)
		})
	}
	t.Run("inherited environment", func(t *testing.T) {
		restartGuardRepinned(t, rollbackRestartNative, sources[rollbackRestartNative], `command.ExtraFiles = []*os.File{controlRead, eventWrite}`, `command.Env = os.Environ(); command.ExtraFiles = []*os.File{controlRead, eventWrite}`, "forbidden ownership/process")
	})
}

func TestRollbackRestartDoubleRepinnedModelCausalCounterexamples(t *testing.T) {
	raw := restartGuardSources(t)[rollbackRestartModel]
	for _, variant := range []struct{ name, old, replacement, finding string }{
		{"sixth case", `[5]Case{Control, BeforeJCreate, AfterTablePage, AfterJUnlink, AcknowledgedExit}`, `[5]Case{Control, Control, AfterTablePage, AfterJUnlink, AcknowledgedExit}`, "exact five independent"},
		{"wrong real page owner", `{Point: rollbackfault.DBPage, Occurrence: 2, Role: rollbackfault.Database, Owner: 11, Offset: 4096, Length: 4096}`, `{Point: rollbackfault.DBPage, Occurrence: 2, Role: rollbackfault.Database, Owner: 12, Offset: 4096, Length: 4096}`, "eighteen original calls"},
		{"wrong target call", "case AfterTablePage:\n\t\treturn 14, rollbackfault.Returned", `case AfterTablePage: return 15, rollbackfault.Returned`, "target call/phase"},
		{"wrong old-new expectation", "case BeforeJCreate, AfterTablePage:\n\t\treturn rollbackfault.SQLOld", `case BeforeJCreate, AfterTablePage: return rollbackfault.SQLNew`, "complete OLD/NEW"},
		{"frame budget expansion", "MaxFrames                       = 64", `MaxFrames = 255`, "fixed restart budget"},
		{"frame trailing accepted", `int(n) != len(raw)-4`, `int(n) > len(raw)-4`, "truncated oversized unknown duplicate"},
		{"canonical check omitted", `!bytes.Equal(encoded, raw)`, `false`, "truncated oversized unknown duplicate"},
		{"foreign generation", `role.Generation == roles[j].Generation || `, ``, "provenance role generation challenge"},
		{"old ticket replay", `f.Ticket != spec.Ticket || f.Adapter != want`, `f.Adapter != want`, "same-ticket exact adapter prefix"},
		{"post-target operation", `r.next = TargetReached`, `r.next = AdapterObserved`, "same-ticket exact adapter prefix"},
		{"ack without dir sync", `a.DirectorySyncStep == 4`, `a.DirectorySyncStep == 0`, "complete exclusive Sync/Close"},
		{"ack unrelated bytes", `a.SHA256 == body && `, ``, "ack raw commit/body hash"},
		{"Wait signal", `want.State, want.ExitCode = ProcessExited, code`, `want.State, want.ExitCode = ProcessSignaled, code`, "matching process object"},
		{"invent crash finish", `ArmCalls: 1, CommitCalls: 1, Main: AbandonedByExpectedProcessExit`, `ArmCalls: 1, CommitCalls: 1, FinishCalls: 1, Main: AbandonedByExpectedProcessExit`, "crash abandonment"},
		{"recovery retry", `want.Calls = 1`, `want.Calls = 2`, "one bounded copy recovery"},
		{"recovery larger journal", `MaxJournalBytes: 16408`, `MaxJournalBytes: 32768`, "one bounded copy recovery"},
		{"alias original", `other.Before.File == a.Before.File`, `false`, "immutable originals unaliased"},
		{"forged receipt observation", `o := r.Seal()`, `o := Observation{EvidenceConsistent: true}`, "replay raw receipt"},
		{"physical claim ignored", `if e.PhysicalClaim`, `if false`, "replay raw receipt"},
		{"suite physical PASS", "func (SuiteObservation) Pass() bool              { return false }", `func (SuiteObservation) Pass() bool { return true }`, "physical validation or PASS"},
		{"model callback", `type PlanSpec struct {`, `type PlanSpec struct { callback func()`, "descriptor-free fixed scalar"},
	} {
		t.Run(variant.name, func(t *testing.T) {
			restartGuardRepinned(t, rollbackRestartModel, raw, variant.old, variant.replacement, variant.finding)
		})
	}
}

func TestRollbackRestartOracleExceptionRequiresCompletePinnedNativeClosure(t *testing.T) {
	raw := restartGuardSources(t)[rollbackRestartNative]
	findings, err := inspectOracleSource(rollbackRestartNative, raw)
	if err != nil || len(findings) != 0 {
		t.Fatal("exact complete restart controller source rejected", findings, err)
	}
	for _, variant := range []struct{ name, path, old, replacement string }{
		{"copied path", "sqlite/copied_restart_linux_test.go", "", ""},
		{"OR gate", rollbackRestartNative, rollbackRestartTag, "//go:build linux || manual_fault_rollback || manual_restart_rollback"},
		{"same-name alias import", rollbackRestartNative, `"os/exec"`, `exec "os/exec"`},
		{"renamed alias import", rollbackRestartNative, `"os/exec"`, `run "os/exec"`},
		{"function value", rollbackRestartNative, `command := exec.Command(c.binding.BinaryPath, args...)`, `launch := exec.Command; command := launch(c.binding.BinaryPath, args...)`},
		{"shell", rollbackRestartNative, `exec.Command(c.binding.BinaryPath, args...)`, `exec.Command("sh", "-c", c.binding.BinaryPath)`},
		{"foreign args", rollbackRestartNative, `exec.Command(c.binding.BinaryPath, args...)`, `exec.Command(c.binding.BinaryPath, otherArgs...)`},
		{"foreign image", rollbackRestartNative, `exec.Command(c.binding.BinaryPath, args...)`, `exec.Command(otherBinary, args...)`},
		{"extra native call", rollbackRestartNative, `command := exec.Command(c.binding.BinaryPath, args...)`, `_ = exec.Command(c.binding.BinaryPath, args...); command := exec.Command(c.binding.BinaryPath, args...)`},
		{"copied helper", rollbackRestartNative, `func manualRestartSpawn(`, `func copiedSpawn(`},
		{"unreviewed comment", rollbackRestartNative, `package sqlite`, `package sqlite // unreviewed`},
	} {
		t.Run(variant.name, func(t *testing.T) {
			changed := raw
			if variant.old != "" {
				if !strings.Contains(string(raw), variant.old) {
					t.Fatal("oracle source mutation anchor missing", variant.old)
				}
				changed = []byte(strings.Replace(string(raw), variant.old, variant.replacement, 1))
				if variant.name == "renamed alias import" {
					changed = []byte(strings.ReplaceAll(string(changed), "exec.", "run."))
				}
			}
			if variant.name != "unreviewed comment" && variant.path == rollbackRestartNative {
				pin, shape := rollbackRestartPins[variant.path], rollbackRestartShapes[variant.path]
				rollbackRestartPins[variant.path] = fmt.Sprintf("%x", sha256.Sum256(changed))
				rollbackRestartShapes[variant.path] = string(changed)
				t.Cleanup(func() { rollbackRestartPins[variant.path], rollbackRestartShapes[variant.path] = pin, shape })
			}
			found, parseErr := inspectOracleSource(variant.path, changed)
			if parseErr != nil || len(found) == 0 {
				t.Fatal("old startup boundary silently widened", variant.name, found, parseErr)
			}
		})
	}
}

// Every mutant repins both supplemental source identities. The independent
// closed declaration/capability rule must reject it, and the old oracle guard
// must consequently refuse the constructor exception without changing its rules.
func TestRollbackRestartDoubleRepinnedCommandCapabilityClosure(t *testing.T) {
	raw := restartGuardSources(t)[rollbackRestartNative]
	start := `if err := command.Start(); err != nil`
	pid := restartGuardCommandPIDStatement
	for _, variant := range []struct{ name, old, replacement string }{
		{"post-constructor Path", start, `command.Path = "/bin/sh"; ` + start},
		{"post-constructor Args", start, `command.Args = []string{"sh", "-c", "true"}; ` + start},
		{"post-constructor shell Path and Args", start, `command.Path = "/bin/sh"; command.Args = []string{"sh", "-c", "true"}; ` + start},
		{"post-constructor Env", start, `command.Env = []string{"PATH=/tmp"}; ` + start},
		{"post-constructor Dir", start, `command.Dir = "/tmp"; ` + start},
		{"post-constructor ExtraFiles", start, `command.ExtraFiles = []*os.File{eventWrite, controlRead}; ` + start},
		{"post-constructor stdout", start, `command.Stdout = nil; ` + start},
		{"post-constructor stderr", start, `command.Stderr = nil; ` + start},
		{"command object alias", start, `launch := command; launch.Path = "/bin/sh"; ` + start},
		{"command object aggregate alias", start, `launch := *command; launch.Path = "/bin/sh"; *command = launch; ` + start},
		{"Path field address alias", start, `target := &command.Path; *target = "/bin/sh"; ` + start},
		{"Args field address alias", start, `arguments := &command.Args; *arguments = []string{"sh", "-c", "true"}; ` + start},
		{"Env slice alias", start, `environment := command.Env; environment[0] = "PATH=/tmp"; ` + start},
		{"ExtraFiles slice alias", start, `files := command.ExtraFiles; files[0] = eventWrite; ` + start},
		{"outside-spawn command Path", pid, `child.command.Path = "/bin/sh"; ` + pid},
		{"outside-spawn command Args", pid, `child.command.Args = []string{"sh", "-c", "true"}; ` + pid},
		{"outside-spawn command Env", pid, `child.command.Env = []string{"PATH=/tmp"}; ` + pid},
		{"outside-spawn command ExtraFiles", pid, `child.command.ExtraFiles = nil; ` + pid},
		{"outside-spawn object alias", pid, `launch := child.command; launch.Path = "/bin/sh"; ` + pid},
		{"outside-spawn field alias", pid, `arguments := &child.command.Args; *arguments = []string{"sh", "-c", "true"}; ` + pid},
		{"outside-spawn child alias", pid, `borrowed := child; borrowed.command.Path = "/bin/sh"; ` + pid},
		{"outside-spawn indexed owner", pid, `c.children[0].command.Path = "/bin/sh"; ` + pid},
		{"outside-spawn active owner", pid, `c.active.command.Env = []string{"PATH=/tmp"}; ` + pid},
		{"outside-spawn command replacement", pid, `*child.command = exec.Cmd{Path: "/bin/sh"}; ` + pid},
		{"outside-spawn Cmd type alias", pid, `type hiddenCommand = exec.Cmd; ` + pid},
		{"outside-spawn duplicate permitted read", pid, pid + "; " + pid},
	} {
		t.Run(variant.name, func(t *testing.T) {
			if strings.Count(string(raw), variant.old) != 1 {
				t.Fatal("command mutation requires one exact anchor", variant.name)
			}
			changed := []byte(strings.Replace(string(raw), variant.old, variant.replacement, 1))
			pin, shape := rollbackRestartPins[rollbackRestartNative], rollbackRestartShapes[rollbackRestartNative]
			rollbackRestartPins[rollbackRestartNative] = fmt.Sprintf("%x", sha256.Sum256(changed))
			rollbackRestartShapes[rollbackRestartNative] = string(changed)
			t.Cleanup(func() {
				rollbackRestartPins[rollbackRestartNative], rollbackRestartShapes[rollbackRestartNative] = pin, shape
			})
			findings, err := inspectRollbackRestartSource(rollbackRestartNative, changed)
			restartGuardMustFind(t, findings, err, "command capability")
			joined := strings.Join(findings, "\n")
			if strings.Contains(joined, "byte pin changed") || strings.Contains(joined, "source binding/body/order changed") {
				t.Fatal("command counterexample relied on supplemental source snapshots", findings)
			}
			startup, startupErr := inspectOracleSource(rollbackRestartNative, changed)
			if startupErr != nil || len(startup) == 0 {
				t.Fatal("mutable command escaped old startup boundary", variant.name, startup, startupErr)
			}
		})
	}
}

// These are the finite constructor inputs actually used after image admission,
// including the same mutable config pointer passed through read/pipe helpers.
// Whole aggregates and address/slice aliases are substantive counterexamples,
// not a generic mutation catalog or a claim about arbitrary Go programs.
func TestRollbackRestartDoubleRepinnedLaunchInputClosure(t *testing.T) {
	raw := restartGuardSources(t)[rollbackRestartNative]
	positive, positiveErr := inspectRollbackRestartSource(rollbackRestartNative, raw)
	if positiveErr != nil || len(positive) != 0 {
		t.Fatal("unchanged launch input closure rejected", positive, positiveErr)
	}
	ordinary, ordinaryErr := inspectOracleSource(rollbackRestartNative, raw)
	if ordinaryErr != nil || len(ordinary) != 0 {
		t.Fatal("unchanged launch input oracle exception rejected", ordinary, ordinaryErr)
	}
	pipe := `return read, write, readOwner, writeOwner, nil`
	read := `return raw, info, nil`
	diagnostic := `func (d *manualRestartDiagnostic) Write(p []byte) (int, error) {`
	for _, variant := range []struct{ name, old, replacement string }{
		{"pipe direct executable rewrite", pipe, `c.binding.BinaryPath = "/bin/sh"; ` + pipe},
		{"pipe whole binding replacement", pipe, `c.binding = manualRestartBinding{BinaryPath: "/bin/sh"}; ` + pipe},
		{"pipe whole config copy replacement", pipe, `next := *c; next.binding.BinaryPath = "/bin/sh"; *c = next; ` + pipe},
		{"pipe config pointer alias", pipe, `next := c; next.binding.BinaryPath = "/bin/sh"; ` + pipe},
		{"pipe binding address alias", pipe, `binding := &c.binding; binding.BinaryPath = "/bin/sh"; ` + pipe},
		{"pipe path field address alias", pipe, `target := &c.binding.BinaryPath; *target = "/bin/sh"; ` + pipe},
		{"pipe root Dir and environment rewrite", pipe, `c.root = "/tmp"; ` + pipe},
		{"pipe root address alias", pipe, `root := &c.root; *root = "/tmp"; ` + pipe},
		{"pipe rawBinding slice alias", pipe, `binding := c.rawBinding; binding[0] ^= 1; ` + pipe},
		{"pipe rawBinding indexed rewrite", pipe, `c.rawBinding[0] ^= 1; ` + pipe},
		{"pipe rawBinding address alias", pipe, `binding := &c.rawBinding; *binding = []byte("changed"); ` + pipe},
		{"pipe rawBinding aggregate replacement", pipe, `c.rawBinding = []byte("changed"); ` + pipe},
		{"pipe plan aggregate replacement", pipe, `c.plan = rollbackrestart.Plan{}; ` + pipe},
		{"pipe returned directions swapped", pipe, `return write, read, readOwner, writeOwner, nil`},
		{"pipe returned owners swapped", pipe, `return read, write, writeOwner, readOwner, nil`},
		{"pipe owner relabel before return", pipe, `readOwner, writeOwner = writeOwner, readOwner; ` + pipe},
		{"read helper executable rewrite", read, `c.binding.BinaryPath = "/bin/sh"; ` + read},
		{"read returned plan bytes alias", read, `planBytes := raw; planBytes[0] ^= 1; ` + read},
		{"digest mutates input slice", `sum := sha256.Sum256(raw)`, `raw[0] ^= 1; sum := sha256.Sum256(raw)`},
		{"digest returns unrelated bytes", `return rollbackrestart.Digest(hex.EncodeToString(sum[:]))`, `return rollbackrestart.Digest(hex.EncodeToString(raw))`},
		{"image rewrites verified executable", `c.selfInfo = before`, `c.selfInfo = before; c.binding.BinaryPath = "/bin/sh"`},
		{"admitted decode destination redirected", `manualRestartDecode(raw, &c.binding)`, `manualRestartDecode(raw, &manualRestartBinding{})`},
		{"admitted rawBinding slice changes", `c.rawBinding = raw`, `c.rawBinding = raw; raw[0] ^= 1`},
		{"flags return role substitution", "case rollbackrestart.Controller:\n\t\troot = \"^TestManualRollbackRestartController$\"", `case rollbackrestart.Controller: root = "^TestManualRollbackRestartWriter$"`},
		{"spawn plan digest replaced", "\"GOSQLITE_RESTART_PLAN_SHA256=\" + string(manualRestartDigest(rawPlan))", `"GOSQLITE_RESTART_PLAN_SHA256="+string(manualRestartDigest(c.rawBinding))`},
		{"spawn role environment replaced", "\"GOSQLITE_RESTART_ROLE=\" + fmt.Sprint(uint8(role))", `"GOSQLITE_RESTART_ROLE="+fmt.Sprint(uint8(rollbackrestart.Writer))`},
		{"spawn sequence environment replaced", "\"GOSQLITE_RESTART_SEQUENCE=\" + fmt.Sprint(count+1)", `"GOSQLITE_RESTART_SEQUENCE="+fmt.Sprint(count+2)`},
		{"spawn stored pipe directions swapped", `child.controls, child.events = controlWrite, eventRead`, `child.controls, child.events = eventRead, controlWrite`},
		{"spawn stored pipe owners swapped", `child.controlOwner, child.eventOwner = controlWriteOwner, eventReadOwner`, `child.controlOwner, child.eventOwner = eventReadOwner, controlWriteOwner`},
		{"outside-spawn whole child replacement", restartGuardCommandPIDStatement, `next := *child; next.controlOwner, next.eventOwner = next.eventOwner, next.controlOwner; *child = next; ` + restartGuardCommandPIDStatement},
		{"outside-spawn child pointer alias", restartGuardCommandPIDStatement, `next := child; next.controls, next.events = next.events, next.controls; ` + restartGuardCommandPIDStatement},
		{"outside closure config holder", diagnostic, diagnostic + ` var config manualRestartConfig; config.root = "/tmp"; _ = config;`},
		{"outside closure binding holder", diagnostic, diagnostic + ` var binding manualRestartBinding; binding.BinaryPath = "/bin/sh"; _ = binding;`},
		{"outside closure child holder", diagnostic, diagnostic + ` var child manualRestartChild; _ = child;`},
	} {
		t.Run(variant.name, func(t *testing.T) {
			if strings.Count(string(raw), variant.old) != 1 {
				t.Fatal("launch-input mutation requires one exact anchor", variant.name)
			}
			changed := []byte(strings.Replace(string(raw), variant.old, variant.replacement, 1))
			if string(changed) == string(raw) {
				t.Fatal("launch-input mutation must change source", variant.name)
			}
			pin, shape := rollbackRestartPins[rollbackRestartNative], rollbackRestartShapes[rollbackRestartNative]
			rollbackRestartPins[rollbackRestartNative] = fmt.Sprintf("%x", sha256.Sum256(changed))
			rollbackRestartShapes[rollbackRestartNative] = string(changed)
			t.Cleanup(func() {
				rollbackRestartPins[rollbackRestartNative], rollbackRestartShapes[rollbackRestartNative] = pin, shape
			})
			findings, err := inspectRollbackRestartSource(rollbackRestartNative, changed)
			restartGuardMustFind(t, findings, err, "launch input")
			joined := strings.Join(findings, "\n")
			if strings.Contains(joined, "byte pin changed") || strings.Contains(joined, "source binding/body/order changed") {
				t.Fatal("launch input counterexample relied on supplemental source snapshots", findings)
			}
			startup, startupErr := inspectOracleSource(rollbackRestartNative, changed)
			if startupErr != nil || len(startup) == 0 {
				t.Fatal("mutable launch input escaped old startup boundary", variant.name, startup, startupErr)
			}
		})
	}
}
