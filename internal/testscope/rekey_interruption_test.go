package main

// Seven finite AST/source roots. Source strings are parsed as inert syntax;
// no scenario, compiler, process, database, descriptor or fault is invoked.
import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func rekeyGuardFixture(t *testing.T, path string) string {
	t.Helper()
	source, ok := rekeyGuardShapes[path]
	if !ok || source == "" {
		t.Fatal("missing source fixture", path)
	}
	return source
}
func rekeyGuardReject(t *testing.T, path, source string) {
	t.Helper()
	oldPin, hadPin := rekeyGuardPins[path]
	oldShape, hadShape := rekeyGuardShapes[path]
	// Deliberately repin both supplemental identities. Rejection must come
	// from the independent semantic rule, never a stale digest/snapshot.
	rekeyGuardPins[path] = fmt.Sprintf("%x", sha256.Sum256([]byte(source)))
	rekeyGuardShapes[path] = source
	defer func() {
		if hadPin {
			rekeyGuardPins[path] = oldPin
		} else {
			delete(rekeyGuardPins, path)
		}
		if hadShape {
			rekeyGuardShapes[path] = oldShape
		} else {
			delete(rekeyGuardShapes, path)
		}
	}()
	got, err := inspectRekeyInterruptionSource(path, []byte(source))
	if err != nil {
		t.Fatal("mutant must remain valid inert Go syntax", err)
	}
	if len(got) == 0 {
		t.Fatal("repinned semantic mutant admitted", path)
	}
	for _, finding := range got {
		if strings.Contains(finding, "HOLD") || strings.Contains(finding, "declaration") {
			t.Fatal("mutation rejected only through supplemental identity", finding)
		}
	}
}
func rekeyGuardMutant(t *testing.T, path, from, to string) {
	t.Helper()
	source := rekeyGuardFixture(t, path)
	if !strings.Contains(source, from) {
		t.Fatal("stale exact mutant anchor", path, from)
	}
	rekeyGuardReject(t, path, strings.Replace(source, from, to, 1))
}
func rekeyGuardPositive(t *testing.T, path string) {
	t.Helper()
	got, err := inspectRekeyInterruptionStructure(path, []byte(rekeyGuardFixture(t, path)))
	if err != nil || len(got) != 0 {
		t.Fatalf("finite source semantic control %s: %v %v", path, got, err)
	}
}

func rekeyGuardOracleProbe(t *testing.T, path, source string, allowed bool) {
	t.Helper()
	oldPin, hadPin := rekeyGuardPins[path]
	oldShape, hadShape := rekeyGuardShapes[path]
	rekeyGuardPins[path] = fmt.Sprintf("%x", sha256.Sum256([]byte(source)))
	rekeyGuardShapes[path] = source
	defer func() {
		if hadPin {
			rekeyGuardPins[path] = oldPin
		} else {
			delete(rekeyGuardPins, path)
		}
		if hadShape {
			rekeyGuardShapes[path] = oldShape
		} else {
			delete(rekeyGuardShapes, path)
		}
	}()
	got, err := inspectOracleSource(path, []byte(source))
	if err != nil || (len(got) == 0) != allowed {
		t.Fatal("exact startup admission", path, got, err)
	}
}

func TestRekeyGuardInventoryGate(t *testing.T) {
	for _, path := range []string{rekeyGuardNative, rekeyGuardBridge, rekeyGuardModel, rekeyGuardModelTest, rekeyGuardVocabulary} {
		rekeyGuardPositive(t, path)
	}
	rekeyGuardMutant(t, rekeyGuardNative, rekeyGuardTag, "//go:build linux || manual_rekey_interruption")
	rekeyGuardReject(t, "sqlite/copied_rekey_test.go", rekeyGuardFixture(t, rekeyGuardNative))
	rekeyGuardReject(t, "internal/testkit/rekeyinterruption/extra.go", "package rekeyinterruption\nfunc extra() {}\n")
	rekeyGuardMutant(t, rekeyGuardModel, `"crypto/sha256"`, `"crypto/sha256"; "os"`)
	rekeyGuardMutant(t, rekeyGuardModel, "type Hash = [32]byte", "type Hash = [32]byte\ntype Effect func()")
	rekeyGuardMutant(t, rekeyGuardModel, "PayloadHash Hash", "PayloadHash [16]byte")
	rekeyGuardMutant(t, rekeyGuardNative,
		`if os.Getenv("GOSQLITE_MANUAL_REKEY_INTERRUPTION") != "independently_authorized"`,
		`if os.Getenv("GOSQLITE_MANUAL_REKEY_INTERRUPTION") == "independently_authorized"`)
	rekeyGuardMutant(t, rekeyGuardNative, `r := manualRekeyAdmit(t, "writer")`, `_ = os.Getenv("GOFLAGS"); r := manualRekeyAdmit(t, "writer")`)
}

func TestRekeyGuardPublicationOrder(t *testing.T) {
	rekeyGuardPositive(t, rekeyGuardPublish)
	rekeyGuardMutant(t, rekeyGuardPublish,
		`rekeyPublicationObserve(ctx, rekeyBeforeRootRename, s.path, s.rootBytes, raw)`,
		`rekeyPublicationObserve(ctx, rekeyAfterRootRename, s.path, s.rootBytes, raw)`)
	rekeyGuardMutant(t, rekeyGuardPublish,
		`rekeyPublicationObserve(ctx, rekeyAfterRootDirSync, s.path, s.rootBytes, raw)`, "")
	rekeyGuardMutant(t, rekeyGuardPublish,
		`rekeyPublicationObserve(ctx, rekeyAfterRootRename, s.path, s.rootBytes, raw)`,
		`if false { rekeyPublicationObserve(ctx, rekeyAfterRootRename, s.path, s.rootBytes, raw) }`)
	rekeyGuardMutant(t, rekeyGuardPublish,
		`attempted = true`, `attempted = false`)
	rekeyGuardMutant(t, rekeyGuardPublish,
		`rekeyPublicationObserve(ctx, rekeyBeforeRootRename, s.path, s.rootBytes, raw)`,
		`alias := rekeyPublicationObserve; alias(ctx, rekeyBeforeRootRename, s.path, s.rootBytes, raw)`)
}

func TestRekeyGuardObserverIsolation(t *testing.T) {
	rekeyGuardMutant(t, rekeyGuardBridge,
		`scope, ok := ctx.Value(manualRekeyContextKey{}).(*manualRekeyScope)`,
		`oldRoot[0] = 0; scope, ok := ctx.Value(manualRekeyContextKey{}).(*manualRekeyScope)`)
	rekeyGuardMutant(t, rekeyGuardBridge,
		`scope.mu.Unlock()`, `scope.mu.Lock()`)
	rekeyGuardMutant(t, rekeyGuardBridge,
		`grant, err := scope.channel.Exchange("publication", uint8(point), observation, [32]byte{})`,
		`clear(candidateRoot); grant, err := scope.channel.Exchange("publication", uint8(point), observation, [32]byte{})`)
	rekeyGuardMutant(t, rekeyGuardBridge,
		`control := os.NewFile(3, "rekey-control")`, `control := os.NewFile(8, "rekey-control")`)
	rekeyGuardMutant(t, rekeyGuardBridge,
		`os.Exit(ManualRekeyExitCode)`, `os.Exit(0)`)
	rekeyGuardMutant(t, rekeyGuardModel,
		`func (Result) Pass() bool              { return false }`, `func (Result) Pass() bool              { return true }`)
}

func TestRekeyGuardPublicReturnACK(t *testing.T) {
	rekeyGuardMutant(t, rekeyGuardNative, `capturedRekeyErr := rekeyErr`, `capturedRekeyErr := error(nil)`)
	rekeyGuardMutant(t, rekeyGuardNative, `capturedRekeyErr := rekeyErr`, `capturedRekeyErr := rekeyErr; rekeyErr = nil`)
	rekeyGuardMutant(t, rekeyGuardNative,
		`event.Kind == "public-nil" && event == child.last`, `event.Kind == "publication" && event == child.last`)
	rekeyGuardMutant(t, rekeyGuardNative,
		`"control/parent-ack.frame"), raw, true`, `"control/parent-ack.frame"), raw, false`)
	rekeyGuardMutant(t, rekeyGuardNative,
		`os.O_WRONLY|os.O_CREATE|os.O_EXCL`, `os.O_WRONLY|os.O_CREATE`)
	rekeyGuardMutant(t, rekeyGuardBridge,
		`scope.seen != 3`, `scope.seen != 2`)
}

func TestRekeyGuardProcessOwnership(t *testing.T) {
	rekeyGuardMutant(t, rekeyGuardNative,
		`go func() { err := command.Wait(); child.wait <- manualRekeyWait{State: command.ProcessState, Err: err} }()`,
		`if true { go func() { err := command.Wait(); child.wait <- manualRekeyWait{State: command.ProcessState, Err: err} }() }`)
	source := rekeyGuardFixture(t, rekeyGuardNative)
	rekeyGuardOracleProbe(t, rekeyGuardNative, source, true)
	rekeyGuardOracleProbe(t, "sqlite/copied_image_test.go", source, false)
	rekeyGuardOracleProbe(t, rekeyGuardNative, strings.Replace(source, `exec.Command(r.admission.BinaryPath, manualRekeyFlags(role)...)`, `exec.Command("/bin/sh", manualRekeyFlags(role)...)`, 1), false)
	rekeyGuardMutant(t, rekeyGuardNative,
		`exec.Command(r.admission.BinaryPath, manualRekeyFlags(role)...)`, `exec.Command("/bin/sh", manualRekeyFlags(role)...)`)
	rekeyGuardMutant(t, rekeyGuardNative,
		`command.ExtraFiles = []*os.File{controlRead, eventWrite}`, `command.ExtraFiles = []*os.File{eventWrite, controlRead}`)
	rekeyGuardMutant(t, rekeyGuardNative,
		`err := command.Wait(); child.wait <-`, `err := error(nil); child.wait <-`)
	rekeyGuardMutant(t, rekeyGuardNative,
		`child.live, child.unknown = true, true`, `child.waited, child.unknown = true, false`)
	rekeyGuardMutant(t, rekeyGuardNative,
		`command := exec.Command(r.admission.BinaryPath, manualRekeyFlags(role)...)`,
		`command := exec.Command(r.admission.BinaryPath, manualRekeyFlags(role)...); command.Process.Kill()`)
	rekeyGuardMutant(t, rekeyGuardNative,
		`r.require(writer.waited && !writer.live && !writer.unknown`, `r.require(true || writer.waited && !writer.live && !writer.unknown`)
}

func TestRekeyGuardProbeLedger(t *testing.T) {
	rekeyGuardMutant(t, rekeyGuardNative,
		`sqlite.Options{ReadOnly: true, NoCreate: true, Encryption: &options}`, `sqlite.Options{ReadOnly: false, NoCreate: true, Encryption: &options}`)
	rekeyGuardMutant(t, rekeyGuardNative,
		`detail.Code != sqlite.CodeNotADB`, `detail.Code != sqlite.CodeIO`)
	rekeyGuardMutant(t, rekeyGuardNative,
		`p.label, p.path, p.operation = "NEW", r.path("probe-new")`, `p.label, p.path, p.operation = "OLD", r.path("probe-new")`)
	rekeyGuardMutant(t, rekeyGuardNative,
		`"int64", 3, "string", "red", "string", "three"`, `"int64", 3, "string", "red", "string", "wrong"`)
	rekeyGuardMutant(t, rekeyGuardNative,
		`s.IndexPageReads <= 0`, `s.IndexPageReads < 0`)
	rekeyGuardMutant(t, rekeyGuardNative,
		`q.CloseCalls != 1`, `q.CloseCalls > 1`)
	rekeyGuardMutant(t, rekeyGuardNative,
		`MaxPages: 3, MaxPageReads: 512`, `MaxPages: 3, MaxPageReads: 0`)
}

func TestRekeyGuardIntegrationDefault(t *testing.T) {
	rekeyGuardPositive(t, rekeyGuardDefault)
	rekeyGuardPositive(t, rekeyGuardMain)
	for _, path := range []string{rekeyGuardModel, rekeyGuardModelTest} {
		got, err := inspectRollbackFaultSQLSource(path, []byte(rekeyGuardFixture(t, path)))
		if err != nil || len(got) != 0 {
			t.Fatal("independent rekey value types must preserve rollback model boundary", path, got, err)
		}
	}
	rekeyGuardMutant(t, rekeyGuardDefault, rekeyGuardDefaultTag, "//go:build linux && !manual_rekey_interruption")
	rekeyGuardMutant(t, rekeyGuardDefault,
		`func rekeyPublicationObserve(context.Context, rekeyPublicationPoint, string, []byte, []byte) {}`,
		`func rekeyPublicationObserve(context.Context, rekeyPublicationPoint, string, []byte, []byte) { panic("effect") }`)
	rekeyGuardMutant(t, rekeyGuardMain,
		`rekeyContracts, rekeyContractErr := inspectRekeyInterruptionContracts(".")`,
		`rekeyContracts, rekeyContractErr := inspectEncryptedRootNegativeContracts(".")`)
	rekeyGuardMutant(t, rekeyGuardMain,
		`findings = append(findings, rekey...)`, `findings = rekey`)
	rekeyGuardMutant(t, rekeyGuardMain,
		`findings = append(findings, rekey...)`, `findings = nil`)
	rekeyGuardMutant(t, rekeyGuardMain,
		`findings = append(findings, ordinary...)`, `findings = ordinary`)
	rekeyGuardMutant(t, rekeyGuardMain,
		`restart, restartErr := inspectRollbackRestartSource(path, raw)`,
		`restart, restartErr := inspectRekeyInterruptionSource(path, raw)`)
	rekeyGuardMutant(t, rekeyGuardMain,
		`rootNegative, rootNegativeErr := inspectEncryptedRootNegativeSource(path, raw)`,
		`rootNegative, rootNegativeErr := inspectRekeyInterruptionSource(path, raw)`)
	// Ordinary restricted-reference enforcement and the closed A5 plain
	// contract are still invoked independently. A manual tag grants nothing.
	bad := "//go:build linux && manual_rekey_interruption\npackage sqlite_test\nimport \"os\"\nfunc bad() { os.Exit(86) }\n"
	got, err := inspectSource("sqlite/copied_manual_test.go", []byte(bad))
	if err != nil || len(got) == 0 {
		t.Fatal("existing ordinary restricted reference lost", got, err)
	}
	old, err := inspectRollbackRestartSource("sqlite/copied_restart_test.go", []byte("package sqlite\nfunc x() { TestManualRollbackRestartWriter(nil) }\n"))
	if err != nil || len(old) == 0 {
		t.Fatal("closed A5 capability escaped", old, err)
	}
}
