package main

// Eight closed inert AST/source roots. These parse source strings only; they
// never import the manual bridge, create a fixture, launch a child or checkpoint.
import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func checkpointGuardFixture(t *testing.T, path string) string {
	t.Helper()
	source, ok := checkpointGuardTemplates[path]
	if !ok || source == "" {
		t.Fatal("missing frozen source fixture", path)
	}
	return source
}
func checkpointGuardPositive(t *testing.T, path string) {
	t.Helper()
	got, err := inspectCheckpointRestartStructure(path, []byte(checkpointGuardFixture(t, path)))
	if err != nil || len(got) != 0 {
		t.Fatal("finite semantic control", path, got, err)
	}
}
func checkpointGuardReject(t *testing.T, path, source string) {
	t.Helper()
	pin, hadPin := checkpointGuardPins[path]
	// Refresh the byte identity deliberately. The frozen AST obligations,
	// concrete callsites and closed declarations must reject this mutation.
	checkpointGuardPins[path] = fmt.Sprintf("%x", sha256.Sum256([]byte(source)))
	defer func() {
		if hadPin {
			checkpointGuardPins[path] = pin
		} else {
			delete(checkpointGuardPins, path)
		}
	}()
	got, err := inspectCheckpointRestartSource(path, []byte(source))
	if err != nil {
		t.Fatal("mutant must remain valid inert syntax", path, err)
	}
	if len(got) == 0 {
		t.Fatal("repinned semantic mutant admitted", path)
	}
	for _, finding := range got {
		if strings.Contains(finding, "HOLD") {
			t.Fatal("stale identity must not reject semantic mutant", finding)
		}
	}
}
func checkpointGuardMutant(t *testing.T, path, from, to string) {
	t.Helper()
	source := checkpointGuardFixture(t, path)
	if !strings.Contains(source, from) {
		t.Fatal("stale mutant anchor", path, from)
	}
	checkpointGuardReject(t, path, strings.Replace(source, from, to, 1))
}
func checkpointGuardOracleProbe(t *testing.T, path, source string, allowed bool) {
	t.Helper()
	pin, hadPin := checkpointGuardPins[path]
	checkpointGuardPins[path] = fmt.Sprintf("%x", sha256.Sum256([]byte(source)))
	defer func() {
		if hadPin {
			checkpointGuardPins[path] = pin
		} else {
			delete(checkpointGuardPins, path)
		}
	}()
	got, err := inspectOracleSource(path, []byte(source))
	if err != nil || (len(got) == 0) != allowed {
		t.Fatal("exact checkpoint startup admission", path, got, err)
	}
}

func TestCheckpointRestartGuardInventoryAndGate(t *testing.T) {
	for path := range checkpointGuardTemplates {
		checkpointGuardPositive(t, path)
	}
	checkpointGuardMutant(t, checkpointGuardNative, checkpointGuardTag, "//go:build linux || manual_checkpoint_restart")
	checkpointGuardMutant(t, checkpointGuardBridge, checkpointGuardTag, "//go:build manual_checkpoint_restart")
	checkpointGuardReject(t, "sqlite/copied_checkpoint_test.go", checkpointGuardFixture(t, checkpointGuardNative))
	checkpointGuardReject(t, "internal/testkit/checkpointrestart/extra.go", "package checkpointrestart\nfunc extra() {}\n")
	checkpointGuardReject(t, "sqlite/other_manual_test.go", "//go:build linux && manual_checkpoint_restart\n\npackage sqlite_test\n")
	checkpointGuardReject(t, "internal/unrelated/copied.go", "package unrelated\nimport cp \"github.com/ms-xiaosnow/gosqlite/internal/testkit/checkpointrestart\"\nvar _ = cp.ManualContext\n")
	checkpointGuardMutant(t, checkpointGuardModel, `"crypto/sha256"`, `"crypto/sha256"; "os"`)
	checkpointGuardMutant(t, checkpointGuardModel, `package checkpointrestart`, "//go:linkname Validate runtime.gopanic\npackage checkpointrestart")
	checkpointGuardMutant(t, checkpointGuardNative, `os.Getenv("GOSQLITE_MANUAL_CHECKPOINT_RESTART") != "independently_authorized"`, `os.Getenv("GOSQLITE_MANUAL_CHECKPOINT_RESTART") == "independently_authorized"`)
}

func TestCheckpointRestartGuardProductionOrder(t *testing.T) {
	for _, path := range []string{checkpointGuardExclusive, checkpointGuardShared, checkpointGuardPublish} {
		checkpointGuardPositive(t, path)
	}
	checkpointGuardMutant(t, checkpointGuardExclusive, `checkpointObserve(ctx, checkpointExclusiveCopy,`, `checkpointObserve(ctx, checkpointExclusiveRetired,`)
	checkpointGuardMutant(t, checkpointGuardShared, `checkpointObserve(ctx, checkpointSharedCopy,`, `checkpointObserve(ctx, checkpointExclusiveCopy,`)
	checkpointGuardMutant(t, checkpointGuardShared, `s.index.SetBackfill(safe)`, `s.index.SetBackfill(backfill)`)
	checkpointGuardMutant(t, checkpointGuardExclusive,
		`if err := writeFullAt(s.file, p, int64(n-1)*int64(s.header.PageSize)); err != nil {`,
		`if err := s.file.Sync(); err != nil {`)
	checkpointGuardMutant(t, checkpointGuardPublish, `checkpointPublicationObserve(ctx, checkpointBeforeRootRename,`, `checkpointPublicationObserve(ctx, checkpointAfterRootRename,`)
	checkpointGuardMutant(t, checkpointGuardPublish, `rekeyPublicationObserve(ctx, rekeyAfterRootRename, s.path, s.rootBytes, raw)`, `if false { rekeyPublicationObserve(ctx, rekeyAfterRootRename, s.path, s.rootBytes, raw) }`)
	checkpointGuardMutant(t, checkpointGuardPublish, `attempted = true`, `attempted = false`)
	// Move a real namespace operation across an otherwise unchanged observer.
	checkpointGuardMutant(t, checkpointGuardPublish,
		`checkpointPublicationObserve(ctx, checkpointAfterRootRename, s.path, s.rootBytes, raw)`,
		`if e = s.ns.sync(s.path); e != nil { return uncertain(e) }; checkpointPublicationObserve(ctx, checkpointAfterRootRename, s.path, s.rootBytes, raw)`)
}

func TestCheckpointRestartGuardObserverPurpose(t *testing.T) {
	checkpointGuardMutant(t, checkpointGuardBridge, `channel.binding.Role != "writer"`, `channel.binding.Role != "probe"`)
	checkpointGuardMutant(t, checkpointGuardBridge, `channel.sequence != 2`, `channel.sequence != 1`)
	checkpointGuardMutant(t, checkpointGuardBridge, `path == scope.path`, `path != scope.path`)
	checkpointGuardMutant(t, checkpointGuardBridge, `scope.copies == page.Selected`, `scope.copies <= page.Selected`)
	checkpointGuardMutant(t, checkpointGuardBridge, `scope.mu.Unlock()`, `scope.mu.Lock()`)
	checkpointGuardMutant(t, checkpointGuardBridge, `b.Epoch == a.Epoch`, `b.Epoch != a.Epoch`)
	checkpointGuardMutant(t, checkpointGuardBridge, `KDFSalt: h.Epoch.KDFSalt`, `KDFSalt: h.Epoch.RootSalt`)
	checkpointGuardMutant(t, checkpointGuardBridge, `b.Descriptor.ParentCommitID == head.AuthenticatedHeadCommitID`, `b.Descriptor.ParentCommitID == head.OldRootHash`)
	checkpointGuardMutant(t, checkpointGuardBridge, `os.NewFile(3, "checkpoint-control")`, `os.NewFile(8, "checkpoint-control")`)
	checkpointGuardMutant(t, checkpointGuardBridge, `os.Exit(ManualExitCode)`, `os.Exit(0)`)
	checkpointGuardMutant(t, checkpointGuardRootAdapter, `uint8(point)+4`, `uint8(point)+3`)
	checkpointGuardMutant(t, checkpointGuardRootAdapter, `if !checkpointrestart.ManualArmed(ctx) {
		return
	}`, `oldRoot[0] = 0; if !checkpointrestart.ManualArmed(ctx) { return }`)
}

func TestCheckpointRestartGuardPublicEntryAndCommitReceipt(t *testing.T) {
	checkpointGuardMutant(t, checkpointGuardNative, `checkpointErr := db.Checkpoint(ctx)`, `checkpointErr := db.Checkpoint(context.Background())`)
	checkpointGuardMutant(t, checkpointGuardNative, `capturedCheckpointErr := checkpointErr`, `capturedCheckpointErr := error(nil)`)
	checkpointGuardMutant(t, checkpointGuardNative, `capturedCommitErr := commitErr`, `capturedCommitErr := error(nil)`)
	checkpointGuardMutant(t, checkpointGuardNative, `commitReturnedNil := capturedCommitErr == nil`, `commitReturnedNil := true`)
	checkpointGuardMutant(t, checkpointGuardNative, `"control/committed-head.json"), raw, true`, `"control/committed-head.json"), raw, false`)
	checkpointGuardMutant(t, checkpointGuardNative, `os.O_WRONLY|os.O_CREATE|os.O_EXCL`, `os.O_WRONLY|os.O_CREATE`)
	checkpointGuardMutant(t, checkpointGuardNative, `ctx, err := checkpointrestart.ManualContext`, `_, _ = db.Exec(context.Background(), "SELECT 1"); ctx, err := checkpointrestart.ManualContext`)
	checkpointGuardMutant(t, checkpointGuardBridge, `scope.target == 0`, `scope.target != 0`)
	checkpointGuardMutant(t, checkpointGuardNative, `PublicCheckpointReturned: publicReturned`, `PublicCheckpointReturned: true`)
}

func TestCheckpointRestartGuardProcessOwnership(t *testing.T) {
	source := checkpointGuardFixture(t, checkpointGuardNative)
	checkpointGuardOracleProbe(t, checkpointGuardNative, source, true)
	checkpointGuardOracleProbe(t, "sqlite/copied_checkpoint_test.go", source, false)
	checkpointGuardOracleProbe(t, checkpointGuardNative, strings.Replace(source, `exec.Command(r.admission.BinaryPath, manualCheckpointFlags(role)...)`, `exec.Command("/bin/sh", manualCheckpointFlags(role)...)`, 1), false)
	checkpointGuardMutant(t, checkpointGuardNative, `"-test.timeout=0"`, `"-test.timeout=30s"`)
	checkpointGuardMutant(t, checkpointGuardNative, `"GOFLAGS="`, `"GOFLAGS=-tags=manual_checkpoint_restart"`)
	checkpointGuardMutant(t, checkpointGuardNative, `command.ExtraFiles = []*os.File{controlRead, eventWrite}`, `command.ExtraFiles = []*os.File{eventWrite, controlRead}`)
	checkpointGuardMutant(t, checkpointGuardNative, `err := command.Wait()
			child.wait <-`, `err := error(nil); child.wait <-`)
	checkpointGuardMutant(t, checkpointGuardNative, `child.terminalEOF = n == 0 && endErr == io.EOF`, `child.terminalEOF = true`)
	checkpointGuardMutant(t, checkpointGuardNative, `result := <-child.wait`, `result := manualCheckpointWait{}`)
	checkpointGuardMutant(t, checkpointGuardNative, `os.Open("/proc/self/exe")`, `os.Open(r.admission.BinaryPath)`)
	checkpointGuardMutant(t, checkpointGuardNative, `command := exec.Command(r.admission.BinaryPath, manualCheckpointFlags(role)...)`, `command := exec.Command(r.admission.BinaryPath, manualCheckpointFlags(role)...); command.Process.Kill()`)
	checkpointGuardMutant(t, checkpointGuardNative, `o.Attempts++`, `o.Attempts = 0`)
}

func TestCheckpointRestartGuardFullExtensionLedger(t *testing.T) {
	checkpointGuardPositive(t, checkpointGuardLedger)
	checkpointGuardMutant(t, checkpointGuardLedger, `SELECT id,body,active FROM docs ORDER BY id`, `SELECT id,body,active FROM docs ORDER BY id LIMIT 1`)
	checkpointGuardMutant(t, checkpointGuardLedger, `len(postings) != 96`, `len(postings)!=95`)
	checkpointGuardMutant(t, checkpointGuardLedger, `len(members) != 128`, `len(members)!=32`)
	checkpointGuardMutant(t, checkpointGuardLedger, `c.TableRows != 979`, `c.TableRows!=315`)
	checkpointGuardMutant(t, checkpointGuardLedger, `c.IndexEntries != 789`, `c.IndexEntries!=788`)
	checkpointGuardMutant(t, checkpointGuardLedger, `p.ANNProbes != 1`, `p.ANNProbes!=4`)
	checkpointGuardMutant(t, checkpointGuardNative, `ForceScan: i == 1`, `ForceScan: true`)
	checkpointGuardMutant(t, checkpointGuardNative, `stream.Receipt.CloseCalls, stream.Receipt.CloseNil = uint8(owner.Attempts), err == nil`, `stream.Receipt.CloseCalls, stream.Receipt.CloseNil = 1, true`)
	checkpointGuardMutant(t, checkpointGuardNative, `err := db.CheckExtensionIndex(context.Background(), name)`, `err := error(nil)`)
	checkpointGuardMutant(t, checkpointGuardNative, `MaxPages: 4096, MaxPageReads: 1 << 20`, `MaxPages: 40960, MaxPageReads: 1<<24`)
}

func TestCheckpointRestartGuardPrivateCopiesAndProbe(t *testing.T) {
	checkpointGuardMutant(t, checkpointGuardNative, `header.Epoch != fixtureEpoch`, `header.Epoch == fixtureEpoch`)
	checkpointGuardMutant(t, checkpointGuardNative, `expected != uint64(len(raw))`, `expected > uint64(len(raw))`)
	checkpointGuardMutant(t, checkpointGuardNative, `MaxGenerations: 16, MaxRetainedBytes: 64 << 20, MaxDirectoryEntries: 64`, `MaxGenerations: 160, MaxRetainedBytes: 640<<20, MaxDirectoryEntries: 640`)
	checkpointGuardMutant(t, checkpointGuardNative, `providers := r.providerResults()`, `providers := []manualCheckpointProviderResult{}`)
	checkpointGuardMutant(t, checkpointGuardNative, `case ".wal":
						wantRole = encryption.FileWAL`, `case ".wal": wantRole = encryption.FileBase`)
	checkpointGuardMutant(t, checkpointGuardNative, `header.Descriptor.Role != wantRole`, `header.Descriptor.Role == wantRole`)
	checkpointGuardMutant(t, checkpointGuardNative, `s.Nlink == 1`, `s.Nlink <= 2`)
	checkpointGuardMutant(t, checkpointGuardNative, `writer.waited && writer.terminalEOF && !writer.unknown`, `true`)
	checkpointGuardMutant(t, checkpointGuardNative, `r.options("probe-open")`, `r.options("writer-open")`)
	checkpointGuardMutant(t, checkpointGuardNative, `o.ReadOnly = purpose == "probe-open"`, `o.ReadOnly = false`)
	checkpointGuardMutant(t, checkpointGuardNative, `r.path("probe"), r.options("probe-open")`, `r.path("original"), r.options("probe-open")`)
	checkpointGuardMutant(t, checkpointGuardNative, `"same-kernel-controlled-process-exit; no-power-loss-qualification"`, `"physical-PASS"`)
	checkpointGuardMutant(t, checkpointGuardNative, `PhysicalValidated: false`, `PhysicalValidated: true`)
}

func TestCheckpointRestartGuardDefaultAndIntegration(t *testing.T) {
	for _, path := range []string{checkpointGuardPagerDefault, checkpointGuardRootDefault, checkpointGuardMain, checkpointGuardOracle} {
		checkpointGuardPositive(t, path)
	}
	checkpointGuardMutant(t, checkpointGuardPagerDefault, checkpointGuardDefaultTag, "//go:build !manual_checkpoint_restart")
	checkpointGuardMutant(t, checkpointGuardRootDefault, `[]byte, []byte) {
}`, `[]byte, []byte) { panic("native") }`)
	checkpointGuardMutant(t, checkpointGuardMain, `findings = append(findings, checkpoint...)`, `findings = nil`)
	checkpointGuardMutant(t, checkpointGuardMain, `return checkpointErr`, `return nil`)
	checkpointGuardMutant(t, checkpointGuardMain, `fmt.Fprintln(os.Stderr, checkpointContractErr)`, `_ = checkpointContractErr`)
	checkpointGuardMutant(t, checkpointGuardMain, `rekey, rekeyErr := inspectRekeyInterruptionSource(path, raw)`, `rekey, rekeyErr := inspectCheckpointRestartSource(path, raw)`)
	checkpointGuardMutant(t, checkpointGuardOracle, `checkpointGuardProcessReference(path, sel, parents, set, raw)`, `true`)
	// The ordinary inspectSource collector and its A5 exclusions are unchanged;
	// its two local non-ellipsis appends must not be called a main reset.
	if got := inspectReadOnlyWALManualWiring([]byte(checkpointGuardFixture(t, checkpointGuardMain))); len(got) != 0 {
		t.Fatal("original policies", got)
	}
}
