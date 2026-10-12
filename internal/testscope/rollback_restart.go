package main

// SOURCE ONLY / REVIEW HOLD / NOT_RUN. This scanner is a lexical prevention
// boundary for the finite Linux plain-rollback restart facility. It is not a
// native runner, a type checker, source-to-binary proof or physical acceptance.
import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const rollbackRestartBridge = "internal/pager/rollback_restart_manual_linux.go"
const rollbackRestartStub = "internal/pager/rollback_restart_stub_linux.go"
const rollbackRestartNative = "sqlite/rollback_restart_manual_linux_test.go"
const rollbackRestartModel = "internal/testkit/rollbackrestart/contract.go"
const rollbackRestartModelTest = "internal/testkit/rollbackrestart/contract_test.go"
const rollbackRestartGuard = "internal/testscope/rollback_restart.go"
const rollbackRestartGuardTest = "internal/testscope/rollback_restart_test.go"
const rollbackRestartImport = "github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackrestart"
const rollbackRestartTag = "//go:build linux && manual_fault_rollback && manual_restart_rollback"
const rollbackRestartStubTag = "//go:build linux && manual_fault_rollback && !manual_restart_rollback"
const rollbackRestartControllerRoot = "TestManualRollbackRestartController"
const rollbackRestartWriterRoot = "TestManualRollbackRestartWriter"
const rollbackRestartRecoveryRoot = "TestManualRollbackRestartRecovery"
const rollbackRestartMainReview = "HOLD_PENDING_INDEPENDENT_RESTART_WHOLE_MAIN_REVIEW"

// Explicit proposed byte identities. These do not release any earlier reviewed
// main.go pin. The unchanged inspectSource body retains its independently
// released SQL pin; the new whole-main still requires independent review.
var rollbackRestartPins = map[string]string{
	rollbackRestartBridge:    "580c1726e059ed36c86fbaafbe491ec8062f527e9b7db3ef4d45cb501b1f4625",
	rollbackRestartStub:      "d7f3ed454dfc97782d9a69959ce17591e567b6b616e24465d34562d73d42520f",
	rollbackRestartNative:    "27213b134a472b99ebef395f02fd96f31a105375bf9d1b98d6eff603da05d896",
	rollbackRestartModel:     "176bef11727fe7f58dacf73167e73e57a6f1f2f71dcd47eb27e90fb19fc2b7ac",
	rollbackRestartModelTest: "e2573c872f5d8229e56739f336f4771c5c7edfc7744339baddcc3383ca425273",
}

// Full declaration snapshots are supplemental; semantic counterexamples repin
// both these snapshots and byte identities and must still be rejected.
var rollbackRestartShapes = map[string]string{
	rollbackRestartBridge:    "//go:build linux && manual_fault_rollback && manual_restart_rollback\n\npackage pager\n\n// SOURCE ONLY / REVIEW HOLD. This bridge has one concrete pipe transport and\n// one lifetime scope. It is not a callback, VFS replacement, or command runner.\nimport (\n\t\"encoding/binary\"\n\t\"errors\"\n\t\"fmt\"\n\t\"io\"\n\t\"os\"\n\t\"strconv\"\n\t\"sync\"\n\n\t\"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault\"\n\t\"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackrestart\"\n\t\"github.com/ms-xiaosnow/gosqlite/internal/vfs\"\n)\n\ntype ManualRollbackRestartChannel struct {\n\tplan         rollbackrestart.Plan\n\trole         rollbackrestart.RoleIdentity\n\tsequence     uint8\n\tcontrol      *os.File\n\tevents       *os.File\n\tcontrolClose uint8\n\teventsClose  uint8\n\tproblem      error\n\tframeCount   uint8\n\tready        bool\n\tcomplete     bool\n}\n\ntype manualRollbackRestartScope struct {\n\tfault          *rollbackFaultScope\n\tchannel        *ManualRollbackRestartChannel\n\tcount          uint8\n\ttarget         bool\n\tcommitCaptured bool\n\tcommitError    error\n}\n\nvar rollbackRestartRegistry struct {\n\tmu     sync.Mutex\n\tused   bool\n\tactive *manualRollbackRestartScope\n}\n\n// Adoption is fixed to the controller-created ExtraFiles mapping: fd3 is the\n// child control reader; fd4 is the child event writer. Arbitrary files/FDs and\n// callbacks are never accepted. All closure is explicit, checked and once.\nfunc NewManualRollbackRestartChannel(plan rollbackrestart.Plan, role rollbackrestart.Role) (*ManualRollbackRestartChannel, error) {\n\tif os.Getenv(\"GOSQLITE_MANUAL_FAULT_ROLLBACK\") != \"independently_authorized\" || os.Getenv(\"GOSQLITE_MANUAL_RESTART_ROLLBACK\") != \"independently_authorized\" {\n\t\treturn nil, errRollbackFaultGate\n\t}\n\tspec := plan.Spec()\n\tverified, problem := rollbackrestart.NewPlan(spec)\n\tif problem != rollbackrestart.ProblemNone || verified.Spec() != spec {\n\t\treturn nil, errRollbackFaultScope\n\t}\n\tvar identity rollbackrestart.RoleIdentity\n\tswitch role {\n\tcase rollbackrestart.Writer:\n\t\tidentity = spec.Writer\n\tcase rollbackrestart.Recovery:\n\t\tidentity = spec.Recovery\n\tdefault:\n\t\treturn nil, errRollbackFaultScope\n\t}\n\tif os.Getenv(\"GOSQLITE_RESTART_ROLE\") != fmt.Sprint(uint8(role)) {\n\t\treturn nil, errRollbackFaultScope\n\t}\n\tsequence, err := strconv.ParseUint(os.Getenv(\"GOSQLITE_RESTART_SEQUENCE\"), 10, 8)\n\tif err != nil || sequence == 0 || sequence >= rollbackrestart.MaxFrames {\n\t\treturn nil, errRollbackFaultScope\n\t}\n\tchannel := &ManualRollbackRestartChannel{plan: plan, role: identity, sequence: uint8(sequence)}\n\tchannel.control = os.NewFile(3, \"restart-parent-control\")\n\tchannel.events = os.NewFile(4, \"restart-child-events\")\n\tif channel.control == nil || channel.events == nil {\n\t\treturn channel, errRollbackFaultScope\n\t}\n\tcontrolInfo, controlErr := channel.control.Stat()\n\teventInfo, eventErr := channel.events.Stat()\n\tif controlErr != nil || eventErr != nil || controlInfo.Mode()&os.ModeNamedPipe == 0 || eventInfo.Mode()&os.ModeNamedPipe == 0 || vfs.SameFile(controlInfo, eventInfo) {\n\t\treturn channel, errors.Join(errRollbackFaultScope, controlErr, eventErr)\n\t}\n\treturn channel, nil\n}\n\nfunc (c *ManualRollbackRestartChannel) frame(kind rollbackrestart.FrameKind) rollbackrestart.Frame {\n\treturn rollbackrestart.Frame{Schema: rollbackrestart.SchemaVersion, Binding: c.plan.Spec().Binding, Sender: c.role, Sequence: c.sequence + 1, Kind: kind}\n}\nfunc (c *ManualRollbackRestartChannel) send(frame rollbackrestart.Frame) error {\n\tif c.problem != nil || c.complete || c.frameCount >= rollbackrestart.MaxFrames || frame.Sequence != c.sequence+1 || frame.Sender != c.role || frame.Binding != c.plan.Spec().Binding {\n\t\treturn errRollbackFaultScope\n\t}\n\traw, problem := rollbackrestart.EncodeFrame(frame)\n\tif problem != rollbackrestart.ProblemNone {\n\t\tc.problem = errRollbackFaultScope\n\t\treturn c.problem\n\t}\n\tn, err := c.events.Write(raw)\n\tif err == nil && n != len(raw) {\n\t\terr = io.ErrShortWrite\n\t}\n\tif err != nil {\n\t\tc.problem = err\n\t\treturn err\n\t}\n\tc.sequence, c.frameCount = frame.Sequence, c.frameCount+1\n\treturn nil\n}\nfunc (c *ManualRollbackRestartChannel) receive() (rollbackrestart.Frame, error) {\n\tif c.problem != nil || c.complete || c.frameCount >= rollbackrestart.MaxFrames {\n\t\treturn rollbackrestart.Frame{}, errRollbackFaultScope\n\t}\n\tvar size [4]byte\n\tif _, err := io.ReadFull(c.control, size[:]); err != nil {\n\t\tc.problem = err\n\t\treturn rollbackrestart.Frame{}, err\n\t}\n\tcount := binary.BigEndian.Uint32(size[:])\n\tif count == 0 || count > rollbackrestart.MaxFrameBytes {\n\t\tc.problem = errRollbackFaultScope\n\t\treturn rollbackrestart.Frame{}, c.problem\n\t}\n\traw := make([]byte, 4+int(count))\n\tcopy(raw, size[:])\n\tif _, err := io.ReadFull(c.control, raw[4:]); err != nil {\n\t\tc.problem = err\n\t\treturn rollbackrestart.Frame{}, err\n\t}\n\tframe, problem := rollbackrestart.DecodeFrame(raw)\n\tif problem != rollbackrestart.ProblemNone || frame.Binding != c.plan.Spec().Binding || frame.Sender != c.plan.Spec().Controller || frame.Sequence != c.sequence+1 {\n\t\tc.problem = errRollbackFaultScope\n\t\treturn rollbackrestart.Frame{}, c.problem\n\t}\n\tc.sequence, c.frameCount = frame.Sequence, c.frameCount+1\n\treturn frame, nil\n}\n\nfunc (c *ManualRollbackRestartChannel) Ready(actualImage rollbackrestart.FileIdentity, imageHash rollbackrestart.Digest) error {\n\tif c.ready || actualImage != c.plan.Spec().Image || imageHash != c.plan.Spec().Binding.Binary {\n\t\treturn errRollbackFaultScope\n\t}\n\tkind := rollbackrestart.ChildReadyBound\n\tif c.role.Role == rollbackrestart.Recovery {\n\t\tkind = rollbackrestart.RecoveryReadyBound\n\t}\n\tframe := c.frame(kind)\n\tframe.Process = rollbackrestart.ProcessEvidence{PID: int64(os.Getpid()), Handle: uint64(c.role.Role - 1), Image: actualImage, ImageHash: imageHash, ActualImage: true, State: rollbackrestart.ProcessRunning}\n\tif err := c.send(frame); err != nil {\n\t\treturn err\n\t}\n\tgrant, err := c.receive()\n\tif err != nil {\n\t\treturn err\n\t}\n\texpected := rollbackrestart.ArmGranted\n\tif c.role.Role == rollbackrestart.Recovery {\n\t\texpected = rollbackrestart.RecoveryReadbackGranted\n\t}\n\tif grant.Kind != expected {\n\t\treturn errRollbackFaultScope\n\t}\n\tc.ready = true\n\treturn nil\n}\n\nfunc ArmManualRollbackRestart(tx *Tx, plan rollbackrestart.Plan, channel *ManualRollbackRestartChannel) (*manualRollbackRestartScope, error) {\n\tif os.Getenv(\"GOSQLITE_MANUAL_FAULT_ROLLBACK\") != \"independently_authorized\" || os.Getenv(\"GOSQLITE_MANUAL_RESTART_ROLLBACK\") != \"independently_authorized\" {\n\t\treturn nil, errRollbackFaultGate\n\t}\n\tif channel == nil || !channel.ready || channel.role.Role != rollbackrestart.Writer || channel.plan.Spec() != plan.Spec() || tx == nil {\n\t\treturn nil, errRollbackFaultScope\n\t}\n\tfaultPlan, problem := rollbackfault.SQLPlan(plan.Spec().AdapterIdentity, rollbackfault.SQLControl)\n\tif problem != rollbackfault.NoProblem {\n\t\treturn nil, errRollbackFaultScope\n\t}\n\t// Reuse the reviewed armer's exact three-page, ordinary rollback and one\n\t// healthy independent Tx restrictions. Its fixed action is Pass only.\n\toldScope, err := ArmManualRollbackSQLFault(tx, faultPlan)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tscope := &manualRollbackRestartScope{fault: oldScope.scope, channel: channel}\n\trollbackRestartRegistry.mu.Lock()\n\tif rollbackRestartRegistry.used || rollbackRestartRegistry.active != nil {\n\t\trollbackRestartRegistry.mu.Unlock()\n\t\treturn nil, errRollbackFaultScope\n\t}\n\trollbackRestartRegistry.used, rollbackRestartRegistry.active = true, scope\n\trollbackRestartRegistry.mu.Unlock()\n\tscope.fault.mu.Lock()\n\tevents, count := scope.fault.receipt.Events()\n\tproblemErr := scope.fault.problem\n\tscope.fault.mu.Unlock()\n\tif problemErr != nil || count != 1 || events[0].Kind != rollbackfault.RunStarted {\n\t\treturn nil, errRollbackFaultScope\n\t}\n\tframe := channel.frame(rollbackrestart.TransactionArmed)\n\tframe.Adapter, frame.Ticket = events[0], plan.Spec().Ticket\n\tif err := channel.send(frame); err != nil {\n\t\treturn nil, err\n\t}\n\tscope.count = 1\n\treturn scope, nil\n}\n\nfunc rollbackRestartBefore(ticket rollbackFaultTicket) {\n\trollbackRestartObserve(ticket, rollbackfault.Intent)\n}\nfunc rollbackRestartAfter(ticket rollbackFaultTicket) {\n\trollbackRestartObserve(ticket, rollbackfault.Returned)\n}\n\n// Called only after the core has released the adapter mutex. The original Tx\n// mutex remains held by Commit intentionally. Never call a Tx-locking snapshot\n// or any native owner method here; copy only the already recorded scalar event.\nfunc rollbackRestartObserve(ticket rollbackFaultTicket, kind rollbackfault.EventKind) {\n\trollbackRestartRegistry.mu.Lock()\n\tscope := rollbackRestartRegistry.active\n\trollbackRestartRegistry.mu.Unlock()\n\tif scope == nil {\n\t\treturn\n\t}\n\tif ticket.scope == nil || ticket.scope != scope.fault || ticket.action != rollbackfault.Pass || scope.target || scope.commitCaptured {\n\t\trollbackRestartAbort()\n\t}\n\tfault := scope.fault\n\tfault.mu.Lock()\n\tevents, count := fault.receipt.Events()\n\tproblemErr := fault.problem\n\tdiagnostics := fault.diagnostics\n\tfault.mu.Unlock()\n\tif problemErr != nil || count != scope.count+1 || count == 0 {\n\t\trollbackRestartDiagnostic(problemErr, diagnostics)\n\t\trollbackRestartAbort()\n\t}\n\tevent := events[count-1]\n\tif event.Kind != kind || event.Call != ticket.call || event.Identity != scope.channel.plan.Spec().AdapterIdentity || event.Action != rollbackfault.Pass {\n\t\trollbackRestartAbort()\n\t}\n\tframe := scope.channel.frame(rollbackrestart.AdapterObserved)\n\tframe.Adapter, frame.Ticket = event, scope.channel.plan.Spec().Ticket\n\tif err := scope.channel.send(frame); err != nil {\n\t\trollbackRestartTransport(err)\n\t\trollbackRestartDiagnostic(problemErr, diagnostics)\n\t\trollbackRestartAbort()\n\t}\n\tscope.count = count\n\t// Native failure remains an actual failed observation, never an expected\n\t// target. Abort code differs from 86 and parent retains the event prefix.\n\tif kind == rollbackfault.Returned && (!event.Result.Executed || event.Result.NativeError != rollbackfault.None || event.Result.ReturnedError != rollbackfault.None || event.Result.NormalizedError != rollbackfault.None || event.Result.NativeN != event.Call.Length || event.Result.ReturnedN != event.Call.Length) {\n\t\trollbackRestartDiagnostic(problemErr, diagnostics)\n\t\trollbackRestartAbort()\n\t}\n\twhich := scope.channel.plan.Spec().Binding.Case\n\ttarget := (which == rollbackrestart.BeforeJCreate && kind == rollbackfault.Intent && ticket.call.Point == rollbackfault.JournalCreate && ticket.call.Occurrence == 1) || (which == rollbackrestart.AfterTablePage && kind == rollbackfault.Returned && ticket.call.Point == rollbackfault.DBPage && ticket.call.Occurrence == 2 && ticket.call.Offset == 4096 && ticket.call.Length == 4096) || (which == rollbackrestart.AfterJUnlink && kind == rollbackfault.Returned && ticket.call.Point == rollbackfault.JournalUnlink && ticket.call.Occurrence == 1)\n\tif !target {\n\t\treturn\n\t}\n\tscope.target = true\n\treached := scope.channel.frame(rollbackrestart.TargetReached)\n\treached.Adapter, reached.Ticket = event, scope.channel.plan.Spec().Ticket\n\tif err := scope.channel.send(reached); err != nil {\n\t\trollbackRestartTransport(err)\n\t\trollbackRestartDiagnostic(problemErr, diagnostics)\n\t\trollbackRestartAbort()\n\t}\n\ttargetHash, problem := rollbackrestart.FrameDigest(reached)\n\tif problem != rollbackrestart.ProblemNone {\n\t\trollbackRestartTransport(errRollbackFaultScope)\n\t\trollbackRestartDiagnostic(problemErr, diagnostics)\n\t\trollbackRestartAbort()\n\t}\n\tgrant, err := scope.channel.receive()\n\tif err != nil || grant.Kind != rollbackrestart.ExitGranted || grant.Digest != targetHash || grant.Ack != (rollbackrestart.AckEvidence{}) {\n\t\trollbackRestartTransport(errors.Join(errRollbackFaultScope, err))\n\t\trollbackRestartDiagnostic(problemErr, diagnostics)\n\t\trollbackRestartAbort()\n\t}\n\t// No return, panic, defer, Finish, Disarm, DB.Close or rollback happens at\n\t// this precise original-call boundary. This source has never been run.\n\tos.Exit(86)\n}\n\ntype ManualRollbackRestartCommit struct {\n\tEvidence    rollbackrestart.CommitEvidence\n\tCommitError error\n\tProblem     error\n}\n\n// The public caller's next statement passes the untouched Commit error here.\n// A crash prefix never reaches this function; no terminal facts are invented.\nfunc CaptureManualRollbackRestartCommit(scope *manualRollbackRestartScope, commitErr error, done bool, detached bool, activeRows int) ManualRollbackRestartCommit {\n\tout := ManualRollbackRestartCommit{CommitError: commitErr}\n\tif scope == nil || scope.target || scope.commitCaptured {\n\t\tout.Problem = errRollbackFaultScope\n\t\treturn out\n\t}\n\tscope.commitCaptured, scope.commitError = true, commitErr\n\tfault := scope.fault\n\tfault.tx.mu.Lock()\n\tfault.mu.Lock()\n\tevents, count := fault.receipt.Events()\n\thealthy := fault.problem == nil && !fault.inFlight && !fault.finalized && count == 37 && scope.count == count && fault.tx.file == nil && fault.tx.encrypted == nil\n\tfault.mu.Unlock()\n\tfault.tx.mu.Unlock()\n\tif !healthy || !done || !detached || activeRows != 0 || commitErr != nil || events[count-1].Kind != rollbackfault.Returned || events[count-1].Call.Point != rollbackfault.FinalDirectorySync {\n\t\tout.Problem = errRollbackFaultScope\n\t\treturn out\n\t}\n\tif !fault.main.RetirementComplete() || fault.journal == nil || !fault.journal.RetirementComplete() {\n\t\tout.Problem = errRollbackFaultScope\n\t\treturn out\n\t}\n\tstate := ManualRollbackSQLState(nil)\n\tif state.Groups != 0 || state.Pending != 0 {\n\t\tout.Problem = errRollbackFaultScope\n\t\treturn out\n\t}\n\tout.Evidence = rollbackrestart.CommitEvidence{Calls: 1, Value: rollbackrestart.CommitNil, CapturedAdjacent: true, CapturedUnchanged: true, PublicDone: done, PublicDetached: detached, ActiveRows: uint32(activeRows), Main: rollbackfault.OwnerRetired, Journal: rollbackfault.OwnerRetired, Cleanup: rollbackfault.OwnerRetired}\n\treturn out\n}\n\nfunc (c *ManualRollbackRestartChannel) CommitReturned(captured ManualRollbackRestartCommit) error {\n\tif !c.ready || c.role.Role != rollbackrestart.Writer || captured.Problem != nil || captured.CommitError != nil || captured.Evidence.Value != rollbackrestart.CommitNil {\n\t\treturn errRollbackFaultScope\n\t}\n\tframe := c.frame(rollbackrestart.PublicCommitReturned)\n\tframe.Commit = captured.Evidence\n\tcommitHash, problem := rollbackrestart.FrameDigest(frame)\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn errRollbackFaultScope\n\t}\n\tif err := c.send(frame); err != nil {\n\t\treturn err\n\t}\n\tgrant, err := c.receive()\n\tif err != nil {\n\t\treturn err\n\t}\n\tif c.plan.Spec().Binding.Case == rollbackrestart.Control {\n\t\tif grant.Kind != rollbackrestart.NormalFinishGranted || grant.Digest != commitHash {\n\t\t\treturn errRollbackFaultScope\n\t\t}\n\t\treturn nil\n\t}\n\tif c.plan.Spec().Binding.Case != rollbackrestart.AcknowledgedExit || grant.Kind != rollbackrestart.AcknowledgementDurable || grant.Ack.CommitSequence != frame.Sequence || grant.Ack.CommitFrameSHA256 != commitHash {\n\t\treturn errRollbackFaultScope\n\t}\n\tack := grant.Ack\n\tif ack.SHA256 == \"\" || !ack.Exclusive || ack.Bytes <= 0 || ack.Written != ack.Bytes || ack.WriteStep != 1 || ack.FileSyncStep != 2 || ack.FileCloseStep != 3 || ack.DirectorySyncStep != 4 || ack.DirectoryCloseStep != 5 || ack.FileCloseCalls != 1 || ack.DirectoryCloseCalls != 1 || ack.ErrorToken != 0 {\n\t\treturn errRollbackFaultScope\n\t}\n\texit, err := c.receive()\n\tif err != nil || exit.Kind != rollbackrestart.ExitGranted || exit.Ack != ack || exit.Digest != ack.SHA256 {\n\t\treturn errRollbackFaultScope\n\t}\n\tos.Exit(86)\n\treturn errRollbackFaultScope\n}\n\nfunc CompleteManualRollbackRestartControl(scope *manualRollbackRestartScope) error {\n\tif scope == nil || !scope.commitCaptured || scope.commitError != nil || scope.target || scope.channel.plan.Spec().Binding.Case != rollbackrestart.Control {\n\t\treturn errRollbackFaultScope\n\t}\n\treport := finishRollbackFault(scope.fault, scope.commitError)\n\tif report.Problem != nil || report.CommitError != nil || report.Observation.Outcome != rollbackfault.AdapterObserved {\n\t\treturn errRollbackFaultScope\n\t}\n\treturn disarmRollbackFault(scope.fault)\n}\nfunc (c *ManualRollbackRestartChannel) Finished(digest rollbackrestart.Digest) error {\n\tkind := rollbackrestart.WriterFinished\n\tif c.role.Role == rollbackrestart.Recovery {\n\t\tkind = rollbackrestart.ReadbackResult\n\t}\n\tif c.role.Role == rollbackrestart.Writer && c.plan.Spec().Binding.Case != rollbackrestart.Control {\n\t\treturn errRollbackFaultScope\n\t}\n\tframe := c.frame(kind)\n\tframe.Digest = digest\n\tif err := c.send(frame); err != nil {\n\t\treturn err\n\t}\n\tc.complete = true\n\treturn c.CloseNormal()\n}\nfunc (c *ManualRollbackRestartChannel) CloseNormal() error {\n\tif c == nil || c.controlClose != 0 || c.eventsClose != 0 {\n\t\treturn errRollbackFaultScope\n\t}\n\tc.controlClose++\n\tcontrolErr := c.control.Close()\n\tc.eventsClose++\n\teventErr := c.events.Close()\n\treturn errors.Join(controlErr, eventErr)\n}\n\n// Diagnostics preserve original error text alongside its event token; they are\n// bounded, independent from protocol and can never grant a successful exit.\nfunc rollbackRestartDiagnostic(problem error, diagnostics [rollbackfault.MaxEvents]error) {\n\tif problem != nil {\n\t\tfmt.Fprintln(os.Stderr, \"ADAPTER_PROBLEM:\", problem)\n\t}\n\tfor ordinal, diagnostic := range diagnostics {\n\t\tif diagnostic == nil {\n\t\t\tcontinue\n\t\t}\n\t\ttext := diagnostic.Error()\n\t\tif len(text) > 384 {\n\t\t\ttext = text[:384] + \" [TRUNCATED_DIAGNOSTIC_FAILURE]\"\n\t\t}\n\t\tfmt.Fprintf(os.Stderr, \"ORIGINAL_ERROR_TOKEN=%d text=%s\\n\", ordinal+1, text)\n\t}\n}\nfunc rollbackRestartTransport(err error) {\n\ttext := fmt.Sprint(err)\n\tif len(text) > 384 {\n\t\ttext = text[:384] + \" [TRUNCATED_TRANSPORT_FAILURE]\"\n\t}\n\tfmt.Fprintln(os.Stderr, \"ORIGINAL_TRANSPORT_ERROR:\", text)\n}\nfunc rollbackRestartAbort() { os.Exit(87) }\n",
	rollbackRestartStub:      "//go:build linux && manual_fault_rollback && !manual_restart_rollback\n\npackage pager\n\n// The fault-only selection contains no restart state, environment, IPC or exit.\nfunc rollbackRestartBefore(ticket rollbackFaultTicket) {}\nfunc rollbackRestartAfter(ticket rollbackFaultTicket)  {}\n",
	rollbackRestartNative:    "//go:build linux && manual_fault_rollback && manual_restart_rollback\n\npackage sqlite\n\n// SOURCE ONLY / REVIEW HOLD. All native actions described here are NOT_RUN.\n// Exactly one controller case and two fresh, self-image-bound children. No\n// command shell, signal, automatic kill, cleanup retry, or original recovery.\nimport (\n\t\"bytes\"\n\t\"context\"\n\t\"crypto/rand\"\n\t\"crypto/sha256\"\n\t\"encoding/binary\"\n\t\"encoding/hex\"\n\t\"encoding/json\"\n\t\"errors\"\n\t\"fmt\"\n\t\"io\"\n\t\"os\"\n\t\"os/exec\"\n\t\"path/filepath\"\n\t\"strings\"\n\t\"sync\"\n\t\"syscall\"\n\t\"testing\"\n\t\"time\"\n\n\t\"github.com/ms-xiaosnow/gosqlite/internal/pager\"\n\t\"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault\"\n\t\"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackrestart\"\n)\n\nconst manualRestartLimit = 65536\nconst manualRestartWait = 30 * time.Second\n\ntype manualRestartBinding struct {\n\tSchema             string\n\tCase               string\n\tTrial              string\n\tSourceTree         string\n\tSourceManifest     string\n\tSourceManifestPath string\n\tTool               string\n\tToolPath           string\n\tBinary             string\n\tBinaryPath         string\n\tCompileReceipt     string\n\tCompileReceiptPath string\n\tFilesystem         string\n\tNativeDeclaration  string\n}\n\ntype manualRestartConfig struct {\n\tsql              manualRollbackSQLConfig\n\tbinding          manualRestartBinding\n\trawBinding       []byte\n\troot             string\n\tfamily           [5]manualRollbackSQLConfig\n\tplan             rollbackrestart.Plan\n\trole             rollbackrestart.Role\n\tselfInfo         os.FileInfo\n\tbaseline         [12288]byte\n\tretained         [24]manualRestartArtifact\n\tretainedCount    int\n\tactive           *manualRestartChild\n\tchildren         [2]*manualRestartChild\n\treceipt          *rollbackrestart.Receipt\n\tevidence         *rollbackrestart.ScenarioEvidence\n\tdurableAck       rollbackrestart.AckEvidence\n\tfailureAttempted bool\n}\n\ntype manualRestartArtifact struct {\n\tPath   string\n\tDevice uint64\n\tInode  uint64\n\tBytes  int64\n\tHash   string\n\tExists bool\n}\n\n// Limited diagnostics remain separate from protocol. Exhaustion makes Wait\n// fail, never counts as an expected child exit and never kills the child.\ntype manualRestartDiagnostic struct {\n\tmu       sync.Mutex\n\tdata     [manualRestartLimit]byte\n\tcount    int\n\toverflow bool\n}\n\nfunc (d *manualRestartDiagnostic) Write(p []byte) (int, error) {\n\td.mu.Lock()\n\tdefer d.mu.Unlock()\n\tavailable := len(d.data) - d.count\n\tif len(p) > available {\n\t\tcopy(d.data[d.count:], p[:available])\n\t\td.count += available\n\t\td.overflow = true\n\t\treturn available, io.ErrShortWrite\n\t}\n\tcopy(d.data[d.count:], p)\n\td.count += len(p)\n\treturn len(p), nil\n}\nfunc (d *manualRestartDiagnostic) snapshot() ([]byte, bool) {\n\td.mu.Lock()\n\tdefer d.mu.Unlock()\n\treturn append([]byte(nil), d.data[:d.count]...), d.overflow\n}\n\ntype manualRestartWaitResult struct {\n\tstate *os.ProcessState\n\terr   error\n}\ntype manualRestartChild struct {\n\tcommand      *exec.Cmd\n\trole         rollbackrestart.Role\n\tidentity     rollbackrestart.RoleIdentity\n\tpid          int\n\tcontrols     *os.File\n\tevents       *os.File\n\tcontrolOwner int\n\teventOwner   int\n\tstdout       manualRestartDiagnostic\n\tstderr       manualRestartDiagnostic\n\twait         chan manualRestartWaitResult\n\twaited       bool\n\ttimedOut     bool\n\tactual       manualRestartWaitResult\n\tframes       [rollbackrestart.MaxFrames]rollbackrestart.Frame\n\tframeCount   uint8\n\trawFrames    [rollbackrestart.MaxFrames][]byte\n\trawCount     uint8\n\tlastGrant    rollbackrestart.Frame\n\tgrantBytes   int\n\tgrantWritten int\n\tgrantError   string\n\tsequence     uint8\n}\n\nfunc manualRestartDigest(raw []byte) rollbackrestart.Digest {\n\tsum := sha256.Sum256(raw)\n\treturn rollbackrestart.Digest(hex.EncodeToString(sum[:]))\n}\nfunc manualRestartID(info os.FileInfo) (rollbackrestart.FileIdentity, error) {\n\tif info == nil {\n\t\treturn rollbackrestart.FileIdentity{}, fmt.Errorf(\"missing actual stat\")\n\t}\n\tstat, ok := info.Sys().(*syscall.Stat_t)\n\tif !ok || stat.Dev == 0 || stat.Ino == 0 {\n\t\treturn rollbackrestart.FileIdentity{}, fmt.Errorf(\"native dev/inode unavailable\")\n\t}\n\treturn rollbackrestart.FileIdentity{Device: uint64(stat.Dev), Inode: stat.Ino}, nil\n}\nfunc manualRestartRegular(info os.FileInfo) error {\n\tif info == nil || !info.Mode().IsRegular() {\n\t\treturn fmt.Errorf(\"regular file required\")\n\t}\n\tstat, ok := info.Sys().(*syscall.Stat_t)\n\tif !ok || stat.Nlink != 1 || stat.Uid != uint32(os.Getuid()) {\n\t\treturn fmt.Errorf(\"single-link owner-controlled file required\")\n\t}\n\treturn nil\n}\nfunc manualRestartRead(c *manualRestartConfig, path string, bound int64) ([]byte, os.FileInfo, error) {\n\tif !filepath.IsAbs(path) || filepath.Clean(path) != path {\n\t\treturn nil, nil, fmt.Errorf(\"absolute clean input path required\")\n\t}\n\tcurrent := string(filepath.Separator)\n\tfor _, component := range strings.Split(strings.TrimPrefix(filepath.Dir(path), \"/\"), \"/\") {\n\t\tif component == \"\" {\n\t\t\tcontinue\n\t\t}\n\t\tcurrent = filepath.Join(current, component)\n\t\tinfo, err := os.Lstat(current)\n\t\tif err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {\n\t\t\treturn nil, nil, fmt.Errorf(\"input ancestor unverified: %s: %v\", current, err)\n\t\t}\n\t}\n\traw, info, err := manualRollbackSQLRead(&c.sql.ledger, path, bound)\n\tif err != nil {\n\t\treturn nil, nil, err\n\t}\n\tif err := manualRestartRegular(info); err != nil {\n\t\treturn nil, nil, err\n\t}\n\treturn raw, info, nil\n}\nfunc manualRestartDecode(raw []byte, value any) error {\n\tif len(raw) == 0 || len(raw) > 4<<20 {\n\t\treturn fmt.Errorf(\"bounded canonical JSON required\")\n\t}\n\tif err := manualRollbackSQLDecode(raw, value); err != nil {\n\t\treturn err\n\t}\n\tcanonical, err := json.Marshal(value)\n\tif err != nil || !bytes.Equal(raw, canonical) {\n\t\treturn fmt.Errorf(\"noncanonical, duplicate, omitted, or unknown JSON fields: %v\", err)\n\t}\n\treturn nil\n}\nfunc manualRestartFlags(role rollbackrestart.Role) ([]string, error) {\n\troot := \"\"\n\tswitch role {\n\tcase rollbackrestart.Controller:\n\t\troot = \"^TestManualRollbackRestartController$\"\n\tcase rollbackrestart.Writer:\n\t\troot = \"^TestManualRollbackRestartWriter$\"\n\tcase rollbackrestart.Recovery:\n\t\troot = \"^TestManualRollbackRestartRecovery$\"\n\tdefault:\n\t\treturn nil, fmt.Errorf(\"unknown role\")\n\t}\n\treturn []string{\"-test.run=\" + root, \"-test.count=1\", \"-test.shuffle=off\", \"-test.parallel=1\", \"-test.timeout=0\", \"-test.cpu=1\", \"-test.v=true\"}, nil\n}\nfunc manualRestartCheckFlags(role rollbackrestart.Role) error {\n\twant, err := manualRestartFlags(role)\n\tif err != nil || len(os.Args) != len(want)+1 {\n\t\treturn fmt.Errorf(\"exact test root and seven flags required\")\n\t}\n\tfor i := range want {\n\t\tif os.Args[i+1] != want[i] {\n\t\t\treturn fmt.Errorf(\"unexpected flag or flag order\")\n\t\t}\n\t}\n\treturn nil\n}\n\n// /proc/self/exe is the one explicit procfs exception to the no-symlink input\n// policy. Hash the opened running image, then compare fstat before and after;\n// argv[0], PATH and a replacement executable with the same name are insufficient.\nfunc manualRestartImage(c *manualRestartConfig) error {\n\ti, err := c.sql.ledger.reserve()\n\tif err != nil {\n\t\treturn err\n\t}\n\timage, openErr := os.Open(\"/proc/self/exe\")\n\tc.sql.ledger.owners[i] = manualRollbackSQLOwner{file: image, path: \"/proc/self/exe\"}\n\tif openErr != nil {\n\t\treturn errors.Join(openErr, c.sql.ledger.close(i))\n\t}\n\tbefore, statErr := image.Stat()\n\tif statErr != nil || before.Size() < 1 || before.Size() > 128<<20 || !before.Mode().IsRegular() {\n\t\treturn errors.Join(fmt.Errorf(\"running image unverified\"), statErr, c.sql.ledger.close(i))\n\t}\n\traw, readErr := io.ReadAll(io.LimitReader(image, (128<<20)+1))\n\tafter, afterErr := image.Stat()\n\tcloseErr := c.sql.ledger.close(i)\n\tif err := errors.Join(readErr, afterErr, closeErr); err != nil {\n\t\treturn err\n\t}\n\tif int64(len(raw)) != before.Size() || !os.SameFile(before, after) || before.Size() != after.Size() || manualRestartDigest(raw) != rollbackrestart.Digest(c.binding.Binary) {\n\t\treturn fmt.Errorf(\"actual running image changed or digest mismatch\")\n\t}\n\tif !filepath.IsAbs(c.binding.BinaryPath) || filepath.Clean(c.binding.BinaryPath) != c.binding.BinaryPath {\n\t\treturn fmt.Errorf(\"absolute reviewed binary path required\")\n\t}\n\ttarget, targetInfo, err := manualRestartRead(c, c.binding.BinaryPath, 128<<20)\n\tif err != nil || !os.SameFile(before, targetInfo) || manualRestartDigest(target) != rollbackrestart.Digest(c.binding.Binary) {\n\t\treturn fmt.Errorf(\"spawn target differs from running image: %v\", err)\n\t}\n\tc.selfInfo = before\n\treturn nil\n}\n\nfunc manualRestartAdmit(c *manualRestartConfig, role rollbackrestart.Role) error {\n\t// Must dominate flags, paths, stat, descriptors, owners, nonce and process work.\n\tif os.Getenv(\"GOSQLITE_MANUAL_FAULT_ROLLBACK\") != \"independently_authorized\" || os.Getenv(\"GOSQLITE_MANUAL_RESTART_ROLLBACK\") != \"independently_authorized\" {\n\t\treturn fmt.Errorf(\"ADMISSION_BLOCKED: independent fault and restart authorization required\")\n\t}\n\tif err := manualRestartCheckFlags(role); err != nil {\n\t\treturn err\n\t}\n\tif os.Getenv(\"GOSQLITE_RESTART_PRIVATE_ROOT_CONFIRMED\") != \"private_stable_namespace_retained\" || os.Getenv(\"GOSQLITE_RESTART_ROLE\") != fmt.Sprint(uint8(role)) {\n\t\treturn fmt.Errorf(\"role and private retained namespace confirmation required\")\n\t}\n\tc.role = role\n\tc.root = os.Getenv(\"GOSQLITE_RESTART_ROOT\")\n\trootInfo, err := manualRollbackSQLNamespace(c.root)\n\tif err != nil {\n\t\treturn err\n\t}\n\tc.sql.root, c.sql.rootInfo = c.root, rootInfo\n\tvar filesystem syscall.Statfs_t\n\tif err := syscall.Statfs(c.root, &filesystem); err != nil {\n\t\treturn fmt.Errorf(\"native filesystem identity unavailable: %w\", err)\n\t}\n\traw, _, err := manualRestartRead(c, os.Getenv(\"GOSQLITE_RESTART_BINDING\"), 65536)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif manualRestartDigest(raw) != rollbackrestart.Digest(os.Getenv(\"GOSQLITE_RESTART_BINDING_SHA256\")) {\n\t\treturn fmt.Errorf(\"reviewed binding digest mismatch\")\n\t}\n\tif err := manualRestartDecode(raw, &c.binding); err != nil {\n\t\treturn err\n\t}\n\tc.rawBinding = raw\n\tif c.binding.Filesystem != fmt.Sprintf(\"linux-fstype-%x\", uint64(filesystem.Type)) {\n\t\treturn fmt.Errorf(\"declared filesystem does not match native root statfs\")\n\t}\n\twhich, ok := rollbackrestart.ParseCase(c.binding.Case)\n\tif !ok || c.binding.Schema != \"rollback-restart-binding-v1\" || len(c.binding.Trial) < 1 || len(c.binding.Trial) > 64 || strings.ContainsAny(c.binding.Trial, \" /\\\\\\t\\r\\n\") || c.binding.NativeDeclaration != \"linux-native-process-exit-no-powerloss-4096-retain-all\" || c.binding.Filesystem == \"\" || len(c.binding.Filesystem) > 64 {\n\t\treturn fmt.Errorf(\"fixed case, trial and explicit native declaration required\")\n\t}\n\tmanifest, _, err := manualRestartRead(c, c.binding.SourceManifestPath, 4<<20)\n\tif err != nil || manualRestartDigest(manifest) != rollbackrestart.Digest(c.binding.SourceManifest) {\n\t\treturn fmt.Errorf(\"source manifest mismatch: %v\", err)\n\t}\n\tvar inventory struct {\n\t\tFiles map[string]struct {\n\t\t\tBytes   int64  `json:\"bytes\"`\n\t\t\tGitBlob string `json:\"git_blob\"`\n\t\t\tGitMode string `json:\"git_mode\"`\n\t\t\tSHA256  string `json:\"sha256\"`\n\t\t} `json:\"files\"`\n\t\tTree string `json:\"tree\"`\n\t}\n\tif err := manualRollbackSQLDecode(manifest, &inventory); err != nil {\n\t\treturn err\n\t}\n\tif inventory.Tree != c.binding.SourceTree || len(inventory.Tree) != 40 || len(inventory.Files) < 1 || len(inventory.Files) > 4096 {\n\t\treturn fmt.Errorf(\"complete finite source inventory required\")\n\t}\n\tfor path, entry := range inventory.Files {\n\t\tif filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || strings.HasPrefix(path, \"../\") || entry.Bytes < 0 || (entry.GitMode != \"100644\" && entry.GitMode != \"100755\") || len(entry.GitBlob) != 40 {\n\t\t\treturn fmt.Errorf(\"invalid source inventory entry\")\n\t\t}\n\t\tif _, err := manualRollbackSQLHash(entry.SHA256); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\tcompiled, _, err := manualRestartRead(c, c.binding.CompileReceiptPath, 65536)\n\tif err != nil || manualRestartDigest(compiled) != rollbackrestart.Digest(c.binding.CompileReceipt) {\n\t\treturn fmt.Errorf(\"compile receipt mismatch: %v\", err)\n\t}\n\tvar receipt manualRollbackSQLCompile\n\tif err := manualRestartDecode(compiled, &receipt); err != nil {\n\t\treturn err\n\t}\n\tif receipt.Schema != \"rollback-restart-compile-v1\" || receipt.Source != c.binding.SourceManifest || receipt.SourceTree != c.binding.SourceTree || receipt.Tool != c.binding.Tool || receipt.Binary != c.binding.Binary || receipt.Manifest != c.binding.SourceManifest || receipt.Package != \"./sqlite\" || receipt.Tags != \"manual_fault_rollback,manual_restart_rollback\" || !receipt.CompileOnly || int(receipt.Files) != len(inventory.Files) {\n\t\treturn fmt.Errorf(\"reviewed source-to-binary receipt does not bind this restart image\")\n\t}\n\ttool, _, err := manualRestartRead(c, c.binding.ToolPath, 128<<20)\n\tif err != nil || manualRestartDigest(tool) != rollbackrestart.Digest(c.binding.Tool) {\n\t\treturn fmt.Errorf(\"tool image digest mismatch: %v\", err)\n\t}\n\tif err := manualRestartImage(c); err != nil {\n\t\treturn err\n\t}\n\tbaseline, err := manualRollbackSQLBaseline()\n\tif err != nil {\n\t\treturn err\n\t}\n\tc.baseline = baseline\n\trootID, err := manualRestartID(rootInfo)\n\tif err != nil {\n\t\treturn err\n\t}\n\tbinding := rollbackrestart.Binding{Suite: \"rollback-restart-v1\", Profile: rollbackfault.PlainRollback4096, Case: which, Trial: c.binding.Trial, Source: c.binding.SourceTree, Manifest: rollbackrestart.Digest(c.binding.SourceManifest), Tool: rollbackrestart.Digest(c.binding.Tool), Compile: rollbackrestart.Digest(c.binding.CompileReceipt), Binary: rollbackrestart.Digest(c.binding.Binary), Native: rollbackrestart.NativeIdentity{OS: \"linux\", Filesystem: c.binding.Filesystem, SectorSize: 4096, PageSize: 4096}, Root: rootID, Baseline: manualRestartDigest(baseline[:])}\n\tif role == rollbackrestart.Controller {\n\t\timageID, err := manualRestartID(c.selfInfo)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tspec := rollbackrestart.PlanSpec{Binding: binding, Ticket: 1, Image: imageID}\n\t\tidentities := []*rollbackrestart.RoleIdentity{&spec.Controller, &spec.Writer, &spec.Recovery}\n\t\tfor n, identity := range identities {\n\t\t\tvar nonce [32]byte\n\t\t\tif count, err := rand.Read(nonce[:]); err != nil || count != len(nonce) {\n\t\t\t\treturn fmt.Errorf(\"parent nonce generation failed: %v\", err)\n\t\t\t}\n\t\t\t*identity = rollbackrestart.RoleIdentity{Role: rollbackrestart.Role(n + 1), Generation: uint64(n + 1), Challenge: rollbackrestart.Digest(hex.EncodeToString(nonce[:]))}\n\t\t}\n\t\tspec.AdapterIdentity = manualRestartSQLIdentity(binding, spec.Writer)\n\t\tplan, problem := rollbackrestart.NewPlan(spec)\n\t\tif problem != rollbackrestart.NoProblem {\n\t\t\treturn fmt.Errorf(\"restart plan admission: %v\", problem)\n\t\t}\n\t\tc.plan = plan\n\t} else {\n\t\trawPlan, _, err := manualRestartRead(c, filepath.Join(c.root, \"control\", \"plan.json\"), 65536)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tvar spec rollbackrestart.PlanSpec\n\t\tif err := manualRestartDecode(rawPlan, &spec); err != nil {\n\t\t\treturn err\n\t\t}\n\t\tif spec.Binding != binding || manualRestartDigest(rawPlan) != rollbackrestart.Digest(os.Getenv(\"GOSQLITE_RESTART_PLAN_SHA256\")) {\n\t\t\treturn fmt.Errorf(\"child plan/root/source/baseline identity mismatch\")\n\t\t}\n\t\tc.plan, _ = rollbackrestart.NewPlan(spec)\n\t\tif c.plan.Spec() != spec {\n\t\t\treturn fmt.Errorf(\"child plan refused\")\n\t\t}\n\t}\n\treturn c.sql.stable()\n}\n\nfunc manualRestartSQLIdentity(binding rollbackrestart.Binding, role rollbackrestart.RoleIdentity) rollbackfault.Identity {\n\treturn rollbackfault.Identity{Suite: \"rollback-restart-v1\", Source: binding.Source, Tool: string(binding.Tool), Binary: string(binding.Binary), Manifest: string(binding.Manifest), Trial: binding.Trial, Scope: fmt.Sprintf(\"restart-role-%d-generation-%d\", role.Role, role.Generation), Profile: rollbackfault.PlainRollback4096}\n}\n\nfunc manualRestartFamilies(c *manualRestartConfig, create bool) error {\n\tnames := [5]string{\"inputs\", \"trial\", \"evidence\", \"replay\", \"control\"}\n\tif create {\n\t\towner, err := c.sql.ledger.reserve()\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tdirectory, openErr := os.Open(c.root)\n\t\tc.sql.ledger.owners[owner] = manualRollbackSQLOwner{file: directory, path: c.root}\n\t\tif openErr != nil {\n\t\t\treturn errors.Join(openErr, c.sql.ledger.close(owner))\n\t\t}\n\t\tactual, statErr := directory.Stat()\n\t\tif statErr != nil || !os.SameFile(actual, c.sql.rootInfo) {\n\t\t\treturn errors.Join(fmt.Errorf(\"root owner identity changed\"), statErr, c.sql.ledger.close(owner))\n\t\t}\n\t\tentries, readErr := directory.ReadDir(1)\n\t\tcloseErr := c.sql.ledger.close(owner)\n\t\tif readErr != io.EOF || len(entries) != 0 || closeErr != nil {\n\t\t\treturn errors.Join(fmt.Errorf(\"controller root must be empty\"), readErr, closeErr)\n\t\t}\n\t}\n\tfor i, name := range names {\n\t\tpath := filepath.Join(c.root, name)\n\t\tif create {\n\t\t\tif err := os.Mkdir(path, 0700); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n\t\t}\n\t\tinfo, err := manualRollbackSQLNamespace(path)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tc.family[i].root, c.family[i].rootInfo = path, info\n\t}\n\treturn c.sql.stable()\n}\nfunc manualRestartSyncDirectory(c *manualRestartConfig, family int) error {\n\tif family < 0 || family >= len(c.family) {\n\t\treturn fmt.Errorf(\"closed family required\")\n\t}\n\tf := &c.family[family]\n\tif err := f.stable(); err != nil {\n\t\treturn err\n\t}\n\ti, err := c.sql.ledger.reserve()\n\tif err != nil {\n\t\treturn err\n\t}\n\tdir, openErr := os.Open(f.root)\n\tc.sql.ledger.owners[i] = manualRollbackSQLOwner{file: dir, path: f.root}\n\tif openErr != nil {\n\t\treturn errors.Join(openErr, c.sql.ledger.close(i))\n\t}\n\tactual, statErr := dir.Stat()\n\tif statErr != nil || !os.SameFile(actual, f.rootInfo) {\n\t\treturn errors.Join(fmt.Errorf(\"directory changed\"), statErr, c.sql.ledger.close(i))\n\t}\n\tsyncErr := dir.Sync()\n\tcloseErr := c.sql.ledger.close(i)\n\treturn errors.Join(syncErr, closeErr, f.stable())\n}\nfunc manualRestartCreate(c *manualRestartConfig, family int, name string, raw []byte) error {\n\tallowed := map[string]bool{\n\t\t\"0/baseline.input.db\": true, \"0/binding.input.json\": true,\n\t\t\"1/trial.db\": true, \"2/post-exit.input.db\": true, \"2/post-exit-journal.input.bin\": true,\n\t\t\"3/replay.db\": true, \"3/replay.db-journal\": true,\n\t\t\"4/plan.json\": true, \"4/ack.json\": true, \"4/baseline-sql.json\": true,\n\t\t\"4/readback.json\": true, \"4/result.json\": true, \"4/final-manifest.json\": true, \"4/failure.json\": true,\n\t\t\"4/failure-wait.json\": true, \"4/failure-stdout.json\": true, \"4/failure-stderr.json\": true, \"4/failure-protocol.json\": true,\n\t\t\"4/writer-stdout.json\": true, \"4/writer-stderr.json\": true,\n\t\t\"4/recovery-stdout.json\": true, \"4/recovery-stderr.json\": true,\n\t\t\"4/writer-protocol.json\": true, \"4/recovery-protocol.json\": true,\n\t}\n\tif !allowed[fmt.Sprintf(\"%d/%s\", family, name)] {\n\t\treturn fmt.Errorf(\"unreviewed artifact name\")\n\t}\n\tif len(raw) == 0 {\n\t\treturn fmt.Errorf(\"empty artifact body refused; diagnostics use explicit JSON envelopes\")\n\t}\n\tinfo, err := c.family[family].create(name, raw, false)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartRegular(info); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartSyncDirectory(c, family); err != nil {\n\t\treturn err\n\t}\n\tif family == 4 && (name == \"writer-stdout.json\" || name == \"writer-stderr.json\" || name == \"writer-protocol.json\" || name == \"recovery-stdout.json\" || name == \"recovery-stderr.json\" || name == \"recovery-protocol.json\" || name == \"readback.json\" || name == \"result.json\" || name == \"final-manifest.json\") {\n\t\treturn manualRestartRetain(c, \"control/\"+name, 1<<20)\n\t}\n\treturn nil\n}\nfunc manualRestartArtifactAt(c *manualRestartConfig, relative string, bound int64) (manualRestartArtifact, []byte, error) {\n\tout := manualRestartArtifact{Path: relative}\n\tpath := filepath.Join(c.root, relative)\n\tif filepath.IsAbs(relative) || filepath.Clean(relative) != relative || strings.HasPrefix(relative, \"../\") {\n\t\treturn out, nil, fmt.Errorf(\"fixed relative artifact required\")\n\t}\n\traw, info, err := manualRestartRead(c, path, bound)\n\tif errors.Is(err, os.ErrNotExist) {\n\t\treturn out, nil, nil\n\t}\n\tif err != nil {\n\t\treturn out, nil, err\n\t}\n\tid, err := manualRestartID(info)\n\tif err != nil {\n\t\treturn out, nil, err\n\t}\n\tout.Device, out.Inode, out.Bytes, out.Hash, out.Exists = id.Device, id.Inode, int64(len(raw)), string(manualRestartDigest(raw)), true\n\treturn out, raw, nil\n}\nfunc manualRestartRetain(c *manualRestartConfig, relative string, bound int64) error {\n\tstamp, _, err := manualRestartArtifactAt(c, relative, bound)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif c.retainedCount == len(c.retained) {\n\t\treturn fmt.Errorf(\"artifact inventory full\")\n\t}\n\tfor i := 0; i < c.retainedCount; i++ {\n\t\tprevious := c.retained[i]\n\t\tif previous.Path == relative || (stamp.Exists && previous.Exists && stamp.Device == previous.Device && stamp.Inode == previous.Inode) {\n\t\t\treturn fmt.Errorf(\"duplicate or aliased original/copy\")\n\t\t}\n\t}\n\tc.retained[c.retainedCount] = stamp\n\tc.retainedCount++\n\treturn nil\n}\nfunc manualRestartUnchanged(c *manualRestartConfig) error {\n\tif err := c.sql.stable(); err != nil {\n\t\treturn err\n\t}\n\tfor i := 0; i < len(c.family); i++ {\n\t\tif err := c.family[i].stable(); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\tfor i := 0; i < c.retainedCount; i++ {\n\t\tprevious := c.retained[i]\n\t\tstamp, _, err := manualRestartArtifactAt(c, previous.Path, 1<<20)\n\t\tif err != nil || stamp != previous {\n\t\t\treturn fmt.Errorf(\"ARTIFACT_CHANGED: %s: %v\", previous.Path, err)\n\t\t}\n\t}\n\treturn nil\n}\n\nfunc manualRestartAppend(c *manualRestartConfig, receipt *rollbackrestart.Receipt, frame rollbackrestart.Frame) error {\n\tif problem := receipt.Append(frame); problem != rollbackrestart.ProblemNone {\n\t\treturn fmt.Errorf(\"PROTOCOL_INVALID: frame %d kind %d: %v\", frame.Sequence, frame.Kind, problem)\n\t}\n\treturn nil\n}\nfunc manualRestartParentFrame(c *manualRestartConfig, receipt *rollbackrestart.Receipt, kind rollbackrestart.FrameKind) rollbackrestart.Frame {\n\t_, count := receipt.Frames()\n\treturn rollbackrestart.Frame{Schema: rollbackrestart.SchemaVersion, Binding: c.plan.Spec().Binding, Sender: c.plan.Spec().Controller, Sequence: count + 1, Kind: kind}\n}\nfunc manualRestartPipe(c *manualRestartConfig) (*os.File, *os.File, int, int, error) {\n\treadOwner, err := c.sql.ledger.reserve()\n\tif err != nil {\n\t\treturn nil, nil, 0, 0, err\n\t}\n\twriteOwner, err := c.sql.ledger.reserve()\n\tif err != nil {\n\t\treturn nil, nil, 0, 0, err\n\t}\n\tread, write, pipeErr := os.Pipe()\n\tc.sql.ledger.owners[readOwner] = manualRollbackSQLOwner{file: read, path: \"owned restart pipe reader\"}\n\tc.sql.ledger.owners[writeOwner] = manualRollbackSQLOwner{file: write, path: \"owned restart pipe writer\"}\n\tif pipeErr != nil {\n\t\treturn nil, nil, readOwner, writeOwner, pipeErr\n\t}\n\treturn read, write, readOwner, writeOwner, nil\n}\n\nfunc manualRestartSpawn(c *manualRestartConfig, receipt *rollbackrestart.Receipt, role rollbackrestart.Role) (*manualRestartChild, error) {\n\tif c.role != rollbackrestart.Controller || (role != rollbackrestart.Writer && role != rollbackrestart.Recovery) || !manualRollbackSQLRegistryEmpty() || !c.sql.ledger.retired() {\n\t\treturn nil, fmt.Errorf(\"parent role and no live owners required before spawn\")\n\t}\n\tslot := int(role) - int(rollbackrestart.Writer)\n\tif c.children[slot] != nil {\n\t\treturn nil, fmt.Errorf(\"one actual process per fixed role; no retry\")\n\t}\n\tif role == rollbackrestart.Recovery && (c.children[0] == nil || !c.children[0].waited || c.children[0].actual.state == nil || !c.children[0].actual.state.Exited()) {\n\t\treturn nil, fmt.Errorf(\"recovery cannot spawn before actual writer Wait\")\n\t}\n\tfor i := range c.family {\n\t\tif !c.family[i].ledger.retired() {\n\t\t\treturn nil, fmt.Errorf(\"family owner live before spawn\")\n\t\t}\n\t}\n\tif err := manualRestartUnchanged(c); err != nil {\n\t\treturn nil, err\n\t}\n\tif err := manualRestartImage(c); err != nil {\n\t\treturn nil, err\n\t}\n\targs, err := manualRestartFlags(role)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\trawPlan, _, err := manualRestartRead(c, filepath.Join(c.root, \"control\", \"plan.json\"), 65536)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\t_, count := receipt.Frames()\n\tchild := &manualRestartChild{role: role, wait: make(chan manualRestartWaitResult, 1), sequence: count + 1}\n\tc.children[slot] = child\n\tchild.identity = c.plan.Spec().Writer\n\tif role == rollbackrestart.Recovery {\n\t\tchild.identity = c.plan.Spec().Recovery\n\t}\n\tcontrolRead, controlWrite, controlReadOwner, controlWriteOwner, err := manualRestartPipe(c)\n\tif err != nil {\n\t\treturn child, err\n\t}\n\teventRead, eventWrite, eventReadOwner, eventWriteOwner, err := manualRestartPipe(c)\n\tif err != nil {\n\t\treturn child, err\n\t}\n\tchild.controls, child.events = controlWrite, eventRead\n\tchild.controlOwner, child.eventOwner = controlWriteOwner, eventReadOwner\n\t// The only executable is the exact verified running image. No inherited\n\t// PATH, GOFLAGS, testing flags, role authorization or environment is used.\n\tcommand := exec.Command(c.binding.BinaryPath, args...)\n\tcommand.Dir = c.root\n\tcommand.Env = []string{\n\t\t\"GOSQLITE_MANUAL_FAULT_ROLLBACK=independently_authorized\",\n\t\t\"GOSQLITE_MANUAL_RESTART_ROLLBACK=independently_authorized\",\n\t\t\"GOSQLITE_RESTART_PRIVATE_ROOT_CONFIRMED=private_stable_namespace_retained\",\n\t\t\"GOSQLITE_RESTART_ROLE=\" + fmt.Sprint(uint8(role)),\n\t\t\"GOSQLITE_RESTART_ROOT=\" + c.root,\n\t\t\"GOSQLITE_RESTART_BINDING=\" + filepath.Join(c.root, \"inputs\", \"binding.input.json\"),\n\t\t\"GOSQLITE_RESTART_BINDING_SHA256=\" + string(manualRestartDigest(c.rawBinding)),\n\t\t\"GOSQLITE_RESTART_PLAN_SHA256=\" + string(manualRestartDigest(rawPlan)),\n\t\t\"GOSQLITE_RESTART_SEQUENCE=\" + fmt.Sprint(count+1),\n\t}\n\tcommand.ExtraFiles = []*os.File{controlRead, eventWrite}\n\tcommand.Stdout, command.Stderr = &child.stdout, &child.stderr\n\tchild.command = command\n\tc.active = child\n\tif err := command.Start(); err != nil {\n\t\treturn child, fmt.Errorf(\"spawn did not establish an owned child: %w\", err)\n\t}\n\tchild.pid = command.Process.Pid\n\t// Start proves ownership of this process object; message PIDs never select\n\t// a process. The goroutine does exactly one actual Wait and retains state.\n\tgo func() {\n\t\twaitErr := command.Wait()\n\t\tchild.wait <- manualRestartWaitResult{state: command.ProcessState, err: waitErr}\n\t}()\n\tcloseErr := errors.Join(c.sql.ledger.close(controlReadOwner), c.sql.ledger.close(eventWriteOwner))\n\tif closeErr != nil {\n\t\treturn child, closeErr\n\t}\n\tif err := child.events.SetReadDeadline(time.Now().Add(manualRestartWait)); err != nil {\n\t\treturn child, err\n\t}\n\tif err := child.controls.SetWriteDeadline(time.Now().Add(manualRestartWait)); err != nil {\n\t\treturn child, err\n\t}\n\tkind := rollbackrestart.WriterSpawned\n\tif role == rollbackrestart.Recovery {\n\t\tkind = rollbackrestart.RecoverySpawned\n\t}\n\tframe := manualRestartParentFrame(c, receipt, kind)\n\tframe.Process = rollbackrestart.ProcessEvidence{PID: int64(child.pid), Handle: uint64(role - 1), Image: c.plan.Spec().Image, ImageHash: c.plan.Spec().Binding.Binary, SpawnObserved: true, State: rollbackrestart.ProcessRunning}\n\tif err := manualRestartAppend(c, receipt, frame); err != nil {\n\t\treturn child, err\n\t}\n\treturn child, nil\n}\n\nfunc manualRestartReadFrame(c *manualRestartConfig, child *manualRestartChild, receipt *rollbackrestart.Receipt) (rollbackrestart.Frame, error) {\n\tif child == nil || child.pid == 0 || child.waited || child.frameCount >= rollbackrestart.MaxFrames {\n\t\treturn rollbackrestart.Frame{}, fmt.Errorf(\"owned live child and bounded protocol required\")\n\t}\n\tvar header [4]byte\n\tif n, err := io.ReadFull(child.events, header[:]); err != nil {\n\t\tif child.rawCount < rollbackrestart.MaxFrames {\n\t\t\tchild.rawFrames[child.rawCount] = append([]byte(nil), header[:n]...)\n\t\t\tchild.rawCount++\n\t\t}\n\t\treturn rollbackrestart.Frame{}, err\n\t}\n\tcount := binary.BigEndian.Uint32(header[:])\n\tif count == 0 || count > rollbackrestart.MaxFrameBytes {\n\t\tif child.rawCount < rollbackrestart.MaxFrames {\n\t\t\tchild.rawFrames[child.rawCount] = append([]byte(nil), header[:]...)\n\t\t\tchild.rawCount++\n\t\t}\n\t\treturn rollbackrestart.Frame{}, fmt.Errorf(\"oversized/empty protocol frame\")\n\t}\n\traw := make([]byte, 4+int(count))\n\tcopy(raw, header[:])\n\tn, readErr := io.ReadFull(child.events, raw[4:])\n\tif child.rawCount >= rollbackrestart.MaxFrames {\n\t\treturn rollbackrestart.Frame{}, fmt.Errorf(\"raw frame limit\")\n\t}\n\tchild.rawFrames[child.rawCount] = append([]byte(nil), raw[:4+n]...)\n\tchild.rawCount++\n\tif readErr != nil {\n\t\treturn rollbackrestart.Frame{}, readErr\n\t}\n\tframe, problem := rollbackrestart.DecodeFrame(raw)\n\tif problem != rollbackrestart.ProblemNone || frame.Sender != child.identity || frame.Binding != c.plan.Spec().Binding || frame.Sequence != child.sequence+1 {\n\t\treturn frame, fmt.Errorf(\"wrong, stale, duplicate or unbound child frame: %v\", problem)\n\t}\n\t// Ready's PID is cross-checked against the actual object returned by Start.\n\tif (frame.Kind == rollbackrestart.ChildReadyBound || frame.Kind == rollbackrestart.RecoveryReadyBound) && frame.Process.PID != int64(child.command.Process.Pid) {\n\t\treturn frame, fmt.Errorf(\"child-specified PID does not match parent process object\")\n\t}\n\tchild.frames[child.frameCount], child.frameCount = frame, child.frameCount+1\n\tchild.sequence = frame.Sequence\n\tif err := manualRestartAppend(c, receipt, frame); err != nil {\n\t\treturn frame, err\n\t}\n\treturn frame, nil\n}\nfunc manualRestartSendFrame(c *manualRestartConfig, child *manualRestartChild, receipt *rollbackrestart.Receipt, frame rollbackrestart.Frame) error {\n\tif child == nil || child.pid == 0 || child.waited || frame.Sequence != child.sequence+1 {\n\t\treturn fmt.Errorf(\"owned child grant ordering\")\n\t}\n\tpreview := *receipt\n\tif err := manualRestartAppend(c, &preview, frame); err != nil {\n\t\treturn err\n\t}\n\traw, problem := rollbackrestart.EncodeFrame(frame)\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn fmt.Errorf(\"grant encoding: %v\", problem)\n\t}\n\tchild.lastGrant, child.grantBytes = frame, len(raw)\n\tcount, err := child.controls.Write(raw)\n\tchild.grantWritten = count\n\tif err == nil && count != len(raw) {\n\t\terr = io.ErrShortWrite\n\t}\n\tif err != nil {\n\t\tchild.grantError = err.Error()\n\t\treturn err\n\t}\n\tchild.grantError = \"\"\n\t*receipt = preview\n\tchild.sequence = frame.Sequence\n\treturn nil\n}\n\n// A deadline is diagnostic only. The actual process object, channels and file\n// owners remain strongly held here until the actual Wait returns. No signal,\n// Kill, cleanup, retry, recovery or invented exit is used to end this wait.\nfunc manualRestartAwait(t *testing.T, c *manualRestartConfig, child *manualRestartChild) manualRestartWaitResult {\n\tif child.waited {\n\t\treturn child.actual\n\t}\n\ttimer := time.NewTimer(manualRestartWait)\n\tselect {\n\tcase actual := <-child.wait:\n\t\ttimer.Stop()\n\t\tchild.waited, child.actual = true, actual\n\t\treturn actual\n\tcase <-timer.C:\n\t\tchild.timedOut = true\n\t\tmanualRestartPersistFailure(t, c, fmt.Errorf(\"CHILD_LIVE/WAIT_UNKNOWN: actual Wait exceeded observation deadline\"))\n\t\tt.Logf(\"CHILD_LIVE/WAIT_UNKNOWN: retained process object, generation %d, pid %d; no signal, cleanup or recovery\", child.identity.Generation, child.pid)\n\t\tactual := <-child.wait\n\t\tchild.waited, child.actual = true, actual\n\t\treturn actual\n\t}\n}\nfunc manualRestartWaitChild(t *testing.T, c *manualRestartConfig, child *manualRestartChild, receipt *rollbackrestart.Receipt) error {\n\tactual := manualRestartAwait(t, c, child)\n\tif child.timedOut {\n\t\treturn fmt.Errorf(\"WAIT_UNKNOWN: observation deadline exceeded; actual eventual Wait retained without successful scenario\")\n\t}\n\tif actual.state == nil || actual.state.Pid() != child.pid {\n\t\treturn fmt.Errorf(\"WAIT_UNKNOWN: no actual matching process state: %v\", actual.err)\n\t}\n\tcode := 0\n\tif child.role == rollbackrestart.Writer && c.plan.Spec().Binding.Case != rollbackrestart.Control {\n\t\tcode = rollbackrestart.ExpectedExitCode\n\t}\n\tif !actual.state.Exited() || actual.state.ExitCode() != code {\n\t\treturn fmt.Errorf(\"CHILD_EXIT_UNEXPECTED: %v: %v\", actual.state, actual.err)\n\t}\n\tif code == 0 && actual.err != nil {\n\t\treturn fmt.Errorf(\"natural child exit lacked complete Wait: %w\", actual.err)\n\t}\n\tif code == rollbackrestart.ExpectedExitCode {\n\t\tvar exitError *exec.ExitError\n\t\tif !errors.As(actual.err, &exitError) || exitError.ProcessState != actual.state {\n\t\t\treturn fmt.Errorf(\"expected exit lacks actual ExitError provenance\")\n\t\t}\n\t}\n\t// With actual exit established, EOF must follow the exact closed protocol.\n\t// Any extra byte, duplicate terminal, truncation, or diagnostic overflow fails.\n\tif err := child.events.SetReadDeadline(time.Now().Add(manualRestartWait)); err != nil {\n\t\treturn err\n\t}\n\tvar trailing [1]byte\n\tn, tailErr := child.events.Read(trailing[:])\n\tif n != 0 && child.rawCount < rollbackrestart.MaxFrames {\n\t\tchild.rawFrames[child.rawCount] = append([]byte(nil), trailing[:n]...)\n\t\tchild.rawCount++\n\t}\n\tif n != 0 || tailErr != io.EOF {\n\t\treturn fmt.Errorf(\"protocol trailing bytes or missing EOF: %v\", tailErr)\n\t}\n\tstdout, outOverflow := child.stdout.snapshot()\n\tstderr, errOverflow := child.stderr.snapshot()\n\tif outOverflow || errOverflow {\n\t\treturn fmt.Errorf(\"bounded diagnostic overflow; no successful terminal\")\n\t}\n\tname := \"writer\"\n\tif child.role == rollbackrestart.Recovery {\n\t\tname = \"recovery\"\n\t}\n\tstdoutJSON, err := json.Marshal(struct {\n\t\tRaw      []byte\n\t\tOverflow bool\n\t}{stdout, outOverflow})\n\tif err != nil {\n\t\treturn err\n\t}\n\tstderrJSON, err := json.Marshal(struct {\n\t\tRaw      []byte\n\t\tOverflow bool\n\t}{stderr, errOverflow})\n\tif err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 4, name+\"-stdout.json\", stdoutJSON); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 4, name+\"-stderr.json\", stderrJSON); err != nil {\n\t\treturn err\n\t}\n\ttranscript, err := json.Marshal(child.frames[:child.frameCount])\n\tif err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 4, name+\"-protocol.json\", transcript); err != nil {\n\t\treturn err\n\t}\n\tif err := errors.Join(c.sql.ledger.close(child.eventOwner), c.sql.ledger.close(child.controlOwner)); err != nil {\n\t\treturn err\n\t}\n\tkind := rollbackrestart.WriterWaitObserved\n\tif child.role == rollbackrestart.Recovery {\n\t\tkind = rollbackrestart.RecoveryWaitObserved\n\t}\n\tframe := manualRestartParentFrame(c, receipt, kind)\n\tframe.Process = rollbackrestart.ProcessEvidence{PID: int64(child.pid), Handle: uint64(child.role - 1), Image: c.plan.Spec().Image, ImageHash: c.plan.Spec().Binding.Binary, WaitObserved: true, State: rollbackrestart.ProcessExited, ExitCode: code}\n\treturn manualRestartAppend(c, receipt, frame)\n}\n\nfunc manualRestartAck(c *manualRestartConfig, commit rollbackrestart.Frame) (rollbackrestart.AckEvidence, error) {\n\tout := rollbackrestart.AckEvidence{}\n\tif c.role != rollbackrestart.Controller || c.plan.Spec().Binding.Case != rollbackrestart.AcknowledgedExit || commit.Kind != rollbackrestart.PublicCommitReturned || commit.Commit.Value != rollbackrestart.CommitNil || commit.Sender != c.plan.Spec().Writer {\n\t\treturn out, fmt.Errorf(\"ACK_FAILED: exact public nil writer receipt required\")\n\t}\n\thash, problem := rollbackrestart.FrameDigest(commit)\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn out, fmt.Errorf(\"ACK_FAILED: original frame unavailable\")\n\t}\n\traw, problem := rollbackrestart.EncodeFrame(commit)\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn out, fmt.Errorf(\"ACK_FAILED: original frame encoding\")\n\t}\n\traw = raw[4:]\n\t// This is the unaltered canonical COMMIT_OK JSON body including every identity\n\t// and sequence, written exclusively then file-Sync/Close and dir-Sync/Close.\n\tif err := manualRestartCreate(c, 4, \"ack.json\", raw); err != nil {\n\t\treturn out, fmt.Errorf(\"ACK_UNCERTAIN: %w\", err)\n\t}\n\tstamp, ack, err := manualRestartArtifactAt(c, \"control/ack.json\", rollbackrestart.MaxFrameBytes+4)\n\tif err != nil || !stamp.Exists || !bytes.Equal(ack, raw) {\n\t\treturn out, fmt.Errorf(\"ACK_UNCERTAIN: acknowledgement changed: %v\", err)\n\t}\n\tout = rollbackrestart.AckEvidence{CommitSequence: commit.Sequence, CommitFrameSHA256: hash, SHA256: manualRestartDigest(raw), File: rollbackrestart.FileIdentity{Device: stamp.Device, Inode: stamp.Inode}, Bytes: stamp.Bytes, Written: int64(len(raw)), Exclusive: true, WriteStep: 1, FileSyncStep: 2, FileCloseStep: 3, DirectorySyncStep: 4, DirectoryCloseStep: 5, FileCloseCalls: 1, DirectoryCloseCalls: 1}\n\tif err := manualRestartRetain(c, \"control/ack.json\", rollbackrestart.MaxFrameBytes+4); err != nil {\n\t\treturn out, err\n\t}\n\tc.durableAck = out\n\treturn out, nil\n}\n\nfunc manualRestartFreshRegistry() bool {\n\tstate := pager.ManualRollbackSQLState(nil)\n\treturn state.Groups == 0 && state.Pending == 0 && !state.AdapterActive && !state.AdapterUsed\n}\nfunc manualRestartWriter(c *manualRestartConfig, channel *pager.ManualRollbackRestartChannel) error {\n\tif c.role != rollbackrestart.Writer || !manualRestartFreshRegistry() {\n\t\treturn fmt.Errorf(\"fresh writer registry required\")\n\t}\n\tctx := context.Background()\n\tdb, owner, err := manualRollbackSQLOpen(&c.sql, filepath.Join(c.root, \"trial\", \"trial.db\"), false)\n\tif err != nil {\n\t\treturn err\n\t}\n\ttx, err := db.Begin(ctx, TxOptions{Immediate: true})\n\tif err != nil {\n\t\treturn err\n\t}\n\tfor _, statement := range []string{\"UPDATE t SET k='green',v='ONE' WHERE id=1\", \"DELETE FROM t WHERE id=2\", \"INSERT INTO t(id,k,v) VALUES(4,'blue','four')\"} {\n\t\tresult, err := tx.Exec(ctx, statement)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tcount, err := result.RowsAffected()\n\t\tif err != nil || count != 1 {\n\t\t\treturn fmt.Errorf(\"DML affected %d: %v\", count, err)\n\t\t}\n\t}\n\tif tx.activeRows != 0 || len(db.rows) != 0 {\n\t\treturn fmt.Errorf(\"writer has live Rows\")\n\t}\n\tscope, err := pager.ArmManualRollbackRestart(tx.storage, c.plan, channel)\n\tif err != nil {\n\t\treturn err\n\t}\n\tcommitErr := tx.Commit(ctx)\n\tcaptured := pager.CaptureManualRollbackRestartCommit(scope, commitErr, tx.done, db.tx == nil, tx.activeRows)\n\tif captured.CommitError != commitErr || captured.Problem != nil || commitErr != nil {\n\t\treturn fmt.Errorf(\"COMMIT_UNCERTAIN: original=%v capture=%v\", commitErr, captured.Problem)\n\t}\n\tif err := channel.CommitReturned(captured); err != nil {\n\t\treturn err\n\t}\n\t// Acknowledged-exit cannot return from CommitReturned successfully. Only\n\t// natural control is allowed to Finish/Disarm and explicitly close its DB.\n\tif c.plan.Spec().Binding.Case != rollbackrestart.Control {\n\t\treturn fmt.Errorf(\"target was not reached\")\n\t}\n\tif err := pager.CompleteManualRollbackRestartControl(scope); err != nil {\n\t\treturn err\n\t}\n\tif err := c.sql.ledger.close(owner); err != nil {\n\t\treturn err\n\t}\n\tif !c.sql.ledger.retired() || !manualRollbackSQLRegistryEmpty() {\n\t\treturn fmt.Errorf(\"OWNER_PENDING: normal writer control\")\n\t}\n\treturn channel.Finished(\"\")\n}\n\ntype manualRestartReadbackResult struct {\n\tBinding    rollbackrestart.Binding\n\tGeneration uint64\n\tSnapshot   rollbackfault.SQLSnapshotEvidence\n\tRecovery   rollbackrestart.RecoveryEvidence\n}\n\nfunc manualRestartRecovery(c *manualRestartConfig, channel *pager.ManualRollbackRestartChannel) error {\n\tif c.role != rollbackrestart.Recovery || !manualRestartFreshRegistry() {\n\t\treturn fmt.Errorf(\"fresh recovery registry required\")\n\t}\n\tctx := context.Background()\n\tresult := manualRestartReadbackResult{Binding: c.plan.Spec().Binding, Generation: c.plan.Spec().Recovery.Generation}\n\tresult.Recovery = rollbackrestart.RecoveryEvidence{Generation: result.Generation, ReadOnly: true, NoCreate: true, OpenCalls: 1, CheckMaxPages: 3, CheckMaxPageReads: 512, NoRetry: true}\n\treplayPath := filepath.Join(c.root, \"replay\", \"replay.db\")\n\tjournalStamp, _, err := manualRestartArtifactAt(c, \"replay/replay.db-journal\", 16408)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif c.plan.Spec().Binding.Case == rollbackrestart.AfterTablePage {\n\t\tif !journalStamp.Exists || journalStamp.Bytes != 16408 {\n\t\t\treturn fmt.Errorf(\"exact replay journal required\")\n\t\t}\n\t\tresult.Recovery.JournalPresentBefore = true\n\t\tresult.Recovery.Calls = 1\n\t\tresult.Recovery.Limits = rollbackfault.SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}\n\t\trecovered, recoveryErr := manualRollbackSQLRecoverCopy(ctx, replayPath)\n\t\tif recoveryErr != nil {\n\t\t\treturn fmt.Errorf(\"RECOVERY_FAILED: %w\", recoveryErr)\n\t\t}\n\t\tresult.Recovery.Replayed, result.Recovery.ReplayedPages, result.Recovery.OriginalPages = recovered.Recovered, recovered.RestoredPages, recovered.OriginalPages\n\t\tabsent, _, err := manualRestartArtifactAt(c, \"replay/replay.db-journal\", 16408)\n\t\tif err != nil || absent.Exists {\n\t\t\treturn fmt.Errorf(\"replay journal survived recovery: %v\", err)\n\t\t}\n\t} else if journalStamp.Exists {\n\t\treturn fmt.Errorf(\"unexpected journal; no cleanup/recovery is allowed for this case\")\n\t}\n\tidentity := c.plan.Spec().AdapterIdentity\n\tsnapshot, err := manualRollbackSQLReadback(&c.sql, ctx, replayPath, identity)\n\tif err != nil {\n\t\treturn fmt.Errorf(\"READBACK_FAILED: %w\", err)\n\t}\n\tif problem := rollbackfault.ValidateSQLSnapshot(snapshot, identity, rollbackrestart.ExpectedSQLState(c.plan.Spec().Binding.Case)); problem != rollbackfault.SQLProblemNone {\n\t\treturn fmt.Errorf(\"complete SQL/index result differs from frozen expected state: %v\", problem)\n\t}\n\tresult.Snapshot = snapshot\n\tresult.Recovery.OwnersRetired = c.sql.ledger.retired() && manualRollbackSQLRegistryEmpty()\n\tif !result.Recovery.OwnersRetired {\n\t\treturn fmt.Errorf(\"OWNER_PENDING: recovery readback\")\n\t}\n\traw, err := json.Marshal(result)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 4, \"readback.json\", raw); err != nil {\n\t\treturn err\n\t}\n\tdigest, problem := rollbackrestart.SnapshotDigest(snapshot)\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn fmt.Errorf(\"snapshot digest refused: %v\", problem)\n\t}\n\treturn channel.Finished(digest)\n}\n\nfunc manualRestartArtifactEvidence(c *manualRestartConfig, evidence *rollbackrestart.ScenarioEvidence, after bool) error {\n\tfor i := 0; i < int(rollbackrestart.ArtifactCount); i++ {\n\t\trole := rollbackrestart.ArtifactRole(i)\n\t\tartifact, raw, err := manualRestartArtifactAt(c, rollbackrestart.ArtifactName(role), 1<<20)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tstamp := rollbackrestart.ArtifactStamp{}\n\t\tif artifact.Exists {\n\t\t\tstamp = rollbackrestart.ArtifactStamp{Path: artifact.Path, File: rollbackrestart.FileIdentity{Device: artifact.Device, Inode: artifact.Inode}, Exists: true, Regular: true, Links: 1, Bytes: artifact.Bytes, SHA256: rollbackrestart.Digest(artifact.Hash)}\n\t\t\tif role == rollbackrestart.BaselineArtifact || role == rollbackrestart.TrialArtifact || role == rollbackrestart.PostExitArtifact || role == rollbackrestart.ReplayArtifact {\n\t\t\t\tif len(raw) != 12288 {\n\t\t\t\t\treturn fmt.Errorf(\"three-page artifact required\")\n\t\t\t\t}\n\t\t\t\tfor page := 0; page < 3; page++ {\n\t\t\t\t\tstamp.Pages[page] = manualRestartDigest(raw[page*4096 : (page+1)*4096])\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t\tentry := &evidence.Artifacts[i]\n\t\tentry.Role = role\n\t\tif after {\n\t\t\tentry.After = stamp\n\t\t} else {\n\t\t\tentry.Before = stamp\n\t\t\tif stamp.Exists && (role == rollbackrestart.PostExitArtifact || role == rollbackrestart.PostExitJournalArtifact || role == rollbackrestart.ReplayArtifact || role == rollbackrestart.ReplayJournalArtifact) {\n\t\t\t\tentry.ExclusiveCopy, entry.CompleteRead, entry.CompleteWrite = true, true, true\n\t\t\t\tentry.FileSynced, entry.FileClosedOnce, entry.DirectorySynced, entry.DirectoryClosedOnce = true, true, true, true\n\t\t\t}\n\t\t}\n\t}\n\treturn nil\n}\nfunc manualRestartFreeze(c *manualRestartConfig, evidence *rollbackrestart.ScenarioEvidence) error {\n\tif c.active == nil || c.active.role != rollbackrestart.Writer || !c.active.waited || c.active.actual.state == nil || !c.active.actual.state.Exited() {\n\t\treturn fmt.Errorf(\"actual writer Wait must precede all original reads/copies\")\n\t}\n\ttrial, raw, err := manualRestartArtifactAt(c, \"trial/trial.db\", 12288)\n\tif err != nil || !trial.Exists || len(raw) != 12288 {\n\t\treturn fmt.Errorf(\"invalid retained trial: %v\", err)\n\t}\n\tjournal, journalRaw, err := manualRestartArtifactAt(c, \"trial/trial.db-journal\", 16408)\n\tif err != nil {\n\t\treturn err\n\t}\n\twhich := c.plan.Spec().Binding.Case\n\tif which == rollbackrestart.BeforeJCreate {\n\t\tif journal.Exists || !bytes.Equal(raw, c.baseline[:]) {\n\t\t\treturn fmt.Errorf(\"pre-create exit must retain exact OLD and no journal\")\n\t\t}\n\t} else if which == rollbackrestart.AfterTablePage {\n\t\tif !journal.Exists || len(journalRaw) != 16408 || bytes.Equal(raw[:4096], c.baseline[:4096]) || bytes.Equal(raw[4096:8192], c.baseline[4096:8192]) || !bytes.Equal(raw[8192:], c.baseline[8192:]) {\n\t\t\treturn fmt.Errorf(\"exact two-written-page prefix and intact journal required\")\n\t\t}\n\t} else if journal.Exists || bytes.Equal(raw, c.baseline[:]) {\n\t\treturn fmt.Errorf(\"completed database bytes and absent journal required\")\n\t}\n\tif err := manualRestartRetain(c, \"trial/trial.db\", 12288); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartRetain(c, \"trial/trial.db-journal\", 16408); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 2, \"post-exit.input.db\", raw); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 3, \"replay.db\", raw); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartRetain(c, \"evidence/post-exit.input.db\", 12288); err != nil {\n\t\treturn err\n\t}\n\tif journal.Exists {\n\t\tif err := manualRestartCreate(c, 2, \"post-exit-journal.input.bin\", journalRaw); err != nil {\n\t\t\treturn err\n\t\t}\n\t\tif err := manualRestartCreate(c, 3, \"replay.db-journal\", journalRaw); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\tif err := manualRestartRetain(c, \"evidence/post-exit-journal.input.bin\", 16408); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartArtifactEvidence(c, evidence, false); err != nil {\n\t\treturn err\n\t}\n\t// Distinct inodes are checked before permitting the recovery process,\n\t// including the mutable replay copies that are not in the frozen ledger.\n\tfor i, a := range evidence.Artifacts {\n\t\tif !a.Before.Exists {\n\t\t\tcontinue\n\t\t}\n\t\tfor j := 0; j < i; j++ {\n\t\t\tif evidence.Artifacts[j].Before.Exists && a.Before.File == evidence.Artifacts[j].Before.File {\n\t\t\t\treturn fmt.Errorf(\"copy aliased an original or another artifact\")\n\t\t\t}\n\t\t}\n\t}\n\treturn manualRestartUnchanged(c)\n}\n\nfunc manualRestartDriveWriter(t *testing.T, c *manualRestartConfig, receipt *rollbackrestart.Receipt) error {\n\tchild, err := manualRestartSpawn(c, receipt, rollbackrestart.Writer)\n\tif err != nil {\n\t\treturn err\n\t}\n\tready, err := manualRestartReadFrame(c, child, receipt)\n\tif err != nil || ready.Kind != rollbackrestart.ChildReadyBound {\n\t\treturn fmt.Errorf(\"writer ready failed: %v\", err)\n\t}\n\tarm := manualRestartParentFrame(c, receipt, rollbackrestart.ArmGranted)\n\tif err := manualRestartSendFrame(c, child, receipt, arm); err != nil {\n\t\treturn err\n\t}\n\tfor {\n\t\tframe, err := manualRestartReadFrame(c, child, receipt)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tswitch frame.Kind {\n\t\tcase rollbackrestart.TransactionArmed, rollbackrestart.AdapterObserved:\n\t\t\tcontinue\n\t\tcase rollbackrestart.TargetReached:\n\t\t\ttarget, problem := rollbackrestart.FrameDigest(frame)\n\t\t\tif problem != rollbackrestart.ProblemNone {\n\t\t\t\treturn fmt.Errorf(\"target digest refused\")\n\t\t\t}\n\t\t\texit := manualRestartParentFrame(c, receipt, rollbackrestart.ExitGranted)\n\t\t\texit.Digest = target\n\t\t\tif err := manualRestartSendFrame(c, child, receipt, exit); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n\t\t\treturn manualRestartWaitChild(t, c, child, receipt)\n\t\tcase rollbackrestart.PublicCommitReturned:\n\t\t\tif c.plan.Spec().Binding.Case == rollbackrestart.AcknowledgedExit {\n\t\t\t\tack, err := manualRestartAck(c, frame)\n\t\t\t\tif err != nil {\n\t\t\t\t\treturn err\n\t\t\t\t}\n\t\t\t\tdurable := manualRestartParentFrame(c, receipt, rollbackrestart.AcknowledgementDurable)\n\t\t\t\tdurable.Ack = ack\n\t\t\t\tif err := manualRestartSendFrame(c, child, receipt, durable); err != nil {\n\t\t\t\t\treturn err\n\t\t\t\t}\n\t\t\t\texit := manualRestartParentFrame(c, receipt, rollbackrestart.ExitGranted)\n\t\t\t\texit.Ack, exit.Digest = ack, ack.SHA256\n\t\t\t\tif err := manualRestartSendFrame(c, child, receipt, exit); err != nil {\n\t\t\t\t\treturn err\n\t\t\t\t}\n\t\t\t\treturn manualRestartWaitChild(t, c, child, receipt)\n\t\t\t}\n\t\t\tif c.plan.Spec().Binding.Case != rollbackrestart.Control {\n\t\t\t\treturn fmt.Errorf(\"unacknowledged target was not reached\")\n\t\t\t}\n\t\t\tgrant := manualRestartParentFrame(c, receipt, rollbackrestart.NormalFinishGranted)\n\t\t\tgrant.Digest, _ = rollbackrestart.FrameDigest(frame)\n\t\t\tif err := manualRestartSendFrame(c, child, receipt, grant); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n\t\tcase rollbackrestart.WriterFinished:\n\t\t\treturn manualRestartWaitChild(t, c, child, receipt)\n\t\tdefault:\n\t\t\treturn fmt.Errorf(\"closed writer protocol rejected kind %d\", frame.Kind)\n\t\t}\n\t}\n}\nfunc manualRestartDriveRecovery(t *testing.T, c *manualRestartConfig, receipt *rollbackrestart.Receipt, evidence *rollbackrestart.ScenarioEvidence) error {\n\tchild, err := manualRestartSpawn(c, receipt, rollbackrestart.Recovery)\n\tif err != nil {\n\t\treturn err\n\t}\n\tready, err := manualRestartReadFrame(c, child, receipt)\n\tif err != nil || ready.Kind != rollbackrestart.RecoveryReadyBound {\n\t\treturn fmt.Errorf(\"recovery ready failed: %v\", err)\n\t}\n\tgrant := manualRestartParentFrame(c, receipt, rollbackrestart.RecoveryReadbackGranted)\n\tif err := manualRestartSendFrame(c, child, receipt, grant); err != nil {\n\t\treturn err\n\t}\n\tresult, err := manualRestartReadFrame(c, child, receipt)\n\tif err != nil || result.Kind != rollbackrestart.ReadbackResult {\n\t\treturn fmt.Errorf(\"readback terminal missing: %v\", err)\n\t}\n\tif err := manualRestartWaitChild(t, c, child, receipt); err != nil {\n\t\treturn err\n\t}\n\traw, _, err := manualRestartRead(c, filepath.Join(c.root, \"control\", \"readback.json\"), 65536)\n\tif err != nil {\n\t\treturn err\n\t}\n\tvar readback manualRestartReadbackResult\n\tif err := manualRestartDecode(raw, &readback); err != nil {\n\t\treturn err\n\t}\n\tif readback.Binding != c.plan.Spec().Binding || readback.Generation != c.plan.Spec().Recovery.Generation {\n\t\treturn fmt.Errorf(\"stale/wrong readback binding\")\n\t}\n\tdigest, problem := rollbackrestart.SnapshotDigest(readback.Snapshot)\n\tif problem != rollbackrestart.ProblemNone || digest != result.Digest {\n\t\treturn fmt.Errorf(\"readback artifact differs from child's exact result\")\n\t}\n\tevidence.After, evidence.Recovery = readback.Snapshot, readback.Recovery\n\tif err := manualRestartRetain(c, \"control/readback.json\", 65536); err != nil {\n\t\treturn err\n\t}\n\treturn manualRestartUnchanged(c)\n}\n\nfunc manualRestartController(t *testing.T, c *manualRestartConfig) error {\n\tif c.role != rollbackrestart.Controller || !manualRollbackSQLRegistryEmpty() || !c.sql.ledger.retired() {\n\t\treturn fmt.Errorf(\"parent must have no existing database owners\")\n\t}\n\tif err := manualRestartFamilies(c, true); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 0, \"binding.input.json\", c.rawBinding); err != nil {\n\t\treturn err\n\t}\n\tplanRaw, err := json.Marshal(c.plan.Spec())\n\tif err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 4, \"plan.json\", planRaw); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartRetain(c, \"inputs/binding.input.json\", 65536); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartRetain(c, \"control/plan.json\", 65536); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 0, \"baseline.input.db\", c.baseline[:]); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartRetain(c, \"inputs/baseline.input.db\", 12288); err != nil {\n\t\treturn err\n\t}\n\tbefore, err := manualRollbackSQLReadback(&c.sql, context.Background(), filepath.Join(c.root, \"inputs\", \"baseline.input.db\"), c.plan.Spec().AdapterIdentity)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif problem := rollbackfault.ValidateSQLSnapshot(before, c.plan.Spec().AdapterIdentity, rollbackfault.SQLOld); problem != rollbackfault.SQLProblemNone {\n\t\treturn fmt.Errorf(\"baseline SQL/index not literal OLD: %v\", problem)\n\t}\n\tbeforeRaw, err := json.Marshal(before)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 4, \"baseline-sql.json\", beforeRaw); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartRetain(c, \"control/baseline-sql.json\", 65536); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 1, \"trial.db\", c.baseline[:]); err != nil {\n\t\treturn err\n\t}\n\treceipt, problem := rollbackrestart.NewReceipt(c.plan)\n\tc.receipt = &receipt\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn fmt.Errorf(\"receipt admission: %v\", problem)\n\t}\n\tparent := manualRestartParentFrame(c, &receipt, rollbackrestart.ParentAdmitted)\n\tparent.Process = rollbackrestart.ProcessEvidence{PID: int64(os.Getpid()), Image: c.plan.Spec().Image, ImageHash: c.plan.Spec().Binding.Binary, ActualImage: true, State: rollbackrestart.ProcessRunning}\n\tif err := manualRestartAppend(c, &receipt, parent); err != nil {\n\t\treturn err\n\t}\n\tbaseline := manualRestartParentFrame(c, &receipt, rollbackrestart.BaselineCheckedAndClosed)\n\tbaseline.Digest, problem = rollbackrestart.SnapshotDigest(before)\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn fmt.Errorf(\"baseline snapshot digest\")\n\t}\n\tif err := manualRestartAppend(c, &receipt, baseline); err != nil {\n\t\treturn err\n\t}\n\tevidence := rollbackrestart.ScenarioEvidence{Binding: c.plan.Spec().Binding, Before: before, BeforeGeneration: c.plan.Spec().Controller.Generation, AfterGeneration: c.plan.Spec().Recovery.Generation}\n\tc.evidence = &evidence\n\tif err := manualRestartDriveWriter(t, c, &receipt); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartFreeze(c, &evidence); err != nil {\n\t\treturn err\n\t}\n\tfrozen := manualRestartParentFrame(c, &receipt, rollbackrestart.OriginalsFrozen)\n\tfrozen.Digest, problem = rollbackrestart.ArtifactManifestDigest(evidence.Artifacts, false)\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn fmt.Errorf(\"original artifact manifest refused\")\n\t}\n\tif err := manualRestartAppend(c, &receipt, frozen); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartDriveRecovery(t, c, &receipt, &evidence); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartArtifactEvidence(c, &evidence, true); err != nil {\n\t\treturn err\n\t}\n\tchecked := manualRestartParentFrame(c, &receipt, rollbackrestart.ArtifactsRechecked)\n\tchecked.Digest, problem = rollbackrestart.ArtifactManifestDigest(evidence.Artifacts, true)\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn fmt.Errorf(\"post-readback artifact manifest refused\")\n\t}\n\tif err := manualRestartAppend(c, &receipt, checked); err != nil {\n\t\treturn err\n\t}\n\tcomplete := manualRestartParentFrame(c, &receipt, rollbackrestart.ScenarioEvidenceComplete)\n\tif err := manualRestartAppend(c, &receipt, complete); err != nil {\n\t\treturn err\n\t}\n\tevidence.Lifecycle = rollbackrestart.WriterLifecycle{OpenCalls: 1, BeginCalls: 1, DMLCalls: 3, ArmCalls: 1, CommitCalls: 1, Main: rollbackrestart.AbandonedByExpectedProcessExit, Journal: rollbackrestart.AbandonedByExpectedProcessExit, Cleanup: rollbackrestart.AbandonedByExpectedProcessExit}\n\tswitch c.plan.Spec().Binding.Case {\n\tcase rollbackrestart.BeforeJCreate:\n\t\tevidence.Lifecycle.Journal = rollbackrestart.NotAcquired\n\tcase rollbackrestart.AfterJUnlink:\n\t\tevidence.Lifecycle.Journal = rollbackrestart.RetiredNormally\n\tcase rollbackrestart.Control, rollbackrestart.AcknowledgedExit:\n\t\tevidence.Lifecycle.PublicReturned = true\n\t\tevidence.Lifecycle.Main, evidence.Lifecycle.Journal, evidence.Lifecycle.Cleanup = rollbackrestart.RetiredNormally, rollbackrestart.RetiredNormally, rollbackrestart.RetiredNormally\n\t\tif c.plan.Spec().Binding.Case == rollbackrestart.Control {\n\t\t\tevidence.Lifecycle.FinishCalls, evidence.Lifecycle.DisarmCalls, evidence.Lifecycle.DBCloseCalls = 1, 1, 1\n\t\t}\n\t}\n\tif !c.sql.ledger.retired() || !manualRollbackSQLRegistryEmpty() {\n\t\treturn fmt.Errorf(\"parent owner ledger incomplete\")\n\t}\n\tfor i := range c.family {\n\t\tif !c.family[i].ledger.retired() {\n\t\t\treturn fmt.Errorf(\"artifact owner ledger incomplete\")\n\t\t}\n\t}\n\tevidence.Runtime = rollbackrestart.RuntimeEvidence{EnvFirst: true, ExactRoleSelectors: true, FixedAllowlistEnvironment: true, CountOne: true, ParallelOne: true, CPUOne: true, ShuffleOff: true, TimeoutZero: true, PrivateRoot0700: true, NoSymlinkAncestors: true, RootUnchanged: true, TwoDedicatedPipes: true, PipeOwnersRecorded: true, UnusedPipeEndsClosedOnce: true, BoundedDiagnostics: true, NoInheritedDatabaseOwners: true, ParentOwnersClosedBeforeSpawn: true, FreshRegistryEachChild: true, OnlyParentSpawns: true, NoSignalOrKill: true, NoRetries: true, OriginalsNeverSyncedAfterExit: true, ArtifactsRetained: true, AdapterSameTransaction: true, AdapterSameScope: true, AdapterSameOwners: true, ObserverOutsidePrivateMutex: true, OriginalTxLockHeldAtCut: true, NoCallbackNativeReplacement: true}\n\tobservation := rollbackrestart.Observe(c.plan, receipt, evidence)\n\tif !observation.EvidenceConsistent || !observation.ScenarioObserved || observation.PhysicalValidated() || observation.Pass() {\n\t\treturn fmt.Errorf(\"scenario evidence rejected: %+v\", observation)\n\t}\n\tframes, count := receipt.Frames()\n\toutput := struct {\n\t\tStatus      string\n\t\tPlan        rollbackrestart.PlanSpec\n\t\tFrames      []rollbackrestart.Frame\n\t\tEvidence    rollbackrestart.ScenarioEvidence\n\t\tObservation rollbackrestart.Observation\n\t}{\"SCENARIO_EVIDENCE_COMPLETE_NOT_POWERLOSS_ACCEPTANCE\", c.plan.Spec(), frames[:count], evidence, observation}\n\traw, err := json.Marshal(output)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 4, \"result.json\", raw); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartUnchanged(c); err != nil {\n\t\treturn err\n\t}\n\t// The final manifest includes every retained input and control artifact,\n\t// including result.json. Its own separately measured identity is held and\n\t// rechecked in memory; a manifest cannot recursively contain its own hash.\n\tfinalManifest, err := json.Marshal(c.retained[:c.retainedCount])\n\tif err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 4, \"final-manifest.json\", finalManifest); err != nil {\n\t\treturn err\n\t}\n\treturn manualRestartUnchanged(c)\n}\n\n// Failure receipts never overwrite success, erase originals or cause a retry.\n// A protocol abort permission is a failure-only frame, not a process signal.\n// If it cannot be sent or the child stays in native I/O, retain the actual\n// handle and wait; no recovery may begin on this path.\ntype manualRestartFailureRecord struct {\n\tStatus          string\n\tError           string\n\tPlan            rollbackrestart.PlanSpec\n\tRecordedFrames  []rollbackrestart.Frame\n\tProtocolProblem rollbackrestart.Problem\n\tLastGrant       rollbackrestart.Frame\n\tGrantBytes      int\n\tGrantWritten    int\n\tGrantError      string\n\tRawChildPrefix  [][]byte\n\tCommit          rollbackrestart.CommitValue\n\tDurableAck      rollbackrestart.AckEvidence\n\tSQLState        rollbackfault.SQLState\n\tGeneration      uint64\n\tPID             int\n\tWaitObserved    bool\n\tExitCode        int\n}\n\nfunc manualRestartFailureSnapshot(c *manualRestartConfig, cause error) manualRestartFailureRecord {\n\tchild := c.active\n\tdetail := manualRestartFailureRecord{Status: \"INCOMPLETE_RETAINED\", Error: fmt.Sprint(cause), Plan: c.plan.Spec(), ExitCode: -1}\n\tif len(detail.Error) > 4096 {\n\t\tdetail.Error = detail.Error[:4096]\n\t}\n\tif c.receipt != nil {\n\t\tframes, count := c.receipt.Frames()\n\t\tdetail.RecordedFrames = append([]rollbackrestart.Frame(nil), frames[:count]...)\n\t\tfrozenReceipt := *c.receipt\n\t\tobservation := frozenReceipt.Seal()\n\t\tdetail.Commit, detail.ProtocolProblem = observation.Commit, observation.Problem\n\t}\n\tdetail.DurableAck = c.durableAck\n\tif c.evidence != nil && c.evidence.After.Owner != 0 && rollbackfault.ValidateSQLSnapshot(c.evidence.After, c.plan.Spec().AdapterIdentity, rollbackrestart.ExpectedSQLState(c.plan.Spec().Binding.Case)) == rollbackfault.SQLProblemNone {\n\t\tdetail.SQLState = rollbackrestart.ExpectedSQLState(c.plan.Spec().Binding.Case)\n\t}\n\tif child != nil && child.pid != 0 {\n\t\tdetail.Generation, detail.PID, detail.WaitObserved = child.identity.Generation, child.pid, child.waited && child.actual.state != nil\n\t\tdetail.LastGrant, detail.GrantBytes, detail.GrantWritten, detail.GrantError = child.lastGrant, child.grantBytes, child.grantWritten, child.grantError\n\t\tdetail.RawChildPrefix = append([][]byte(nil), child.rawFrames[:child.rawCount]...)\n\t\tif !child.waited {\n\t\t\tdetail.Status = \"CHILD_LIVE_WAIT_UNKNOWN_RETAINED\"\n\t\t} else if child.actual.state != nil {\n\t\t\tdetail.ExitCode = child.actual.state.ExitCode()\n\t\t}\n\t}\n\treturn detail\n}\n\n// Shared by explicit failure and the normal-Wait timeout path. It never waits,\n// retries, signals, closes a DB, or overwrites an earlier failed receipt.\nfunc manualRestartPersistFailure(t *testing.T, c *manualRestartConfig, cause error) {\n\tif c.failureAttempted {\n\t\treturn\n\t}\n\tc.failureAttempted = true\n\tdetail := manualRestartFailureSnapshot(c, cause)\n\tif c.family[4].root != \"\" {\n\t\traw, err := json.Marshal(detail)\n\t\tif err == nil {\n\t\t\terr = manualRestartCreate(c, 4, \"failure.json\", raw)\n\t\t}\n\t\tif err != nil {\n\t\t\tt.Logf(\"failure evidence write incomplete: %v; originals retained\", err)\n\t\t}\n\t}\n}\n\nfunc manualRestartFailure(t *testing.T, c *manualRestartConfig, cause error) {\n\tchild := c.active\n\tmanualRestartPersistFailure(t, c, cause)\n\tdetail := manualRestartFailureSnapshot(c, cause)\n\tif child == nil || child.pid == 0 {\n\t\treturn\n\t}\n\tif !child.waited {\n\t\tfailure := rollbackrestart.Frame{Schema: rollbackrestart.SchemaVersion, Binding: c.plan.Spec().Binding, Sender: c.plan.Spec().Controller, Sequence: child.sequence + 1, Kind: rollbackrestart.Failure, Diagnostic: rollbackrestart.ProtocolInvalid}\n\t\traw, problem := rollbackrestart.EncodeFrame(failure)\n\t\tif problem == rollbackrestart.ProblemNone {\n\t\t\tn, err := child.controls.Write(raw)\n\t\t\tif err != nil || n != len(raw) {\n\t\t\t\tt.Logf(\"failure-only control send incomplete: %v; keeping actual child\", err)\n\t\t\t}\n\t\t}\n\t\tactual := manualRestartAwait(t, c, child)\n\t\tdetail.WaitObserved = actual.state != nil\n\t\tif actual.state != nil {\n\t\t\tdetail.ExitCode = actual.state.ExitCode()\n\t\t}\n\t}\n\t// The eventual real Wait gets a separate file; it cannot overwrite or\n\t// promote the original failure to successful scenario evidence.\n\tdetail.Status = \"FAILED_ACTUAL_WAIT_RETAINED\"\n\traw, err := json.Marshal(detail)\n\tif err == nil {\n\t\terr = manualRestartCreate(c, 4, \"failure-wait.json\", raw)\n\t}\n\tif err != nil {\n\t\tt.Logf(\"actual Wait evidence write incomplete: %v\", err)\n\t}\n\tstdout, outOverflow := child.stdout.snapshot()\n\tstderr, errOverflow := child.stderr.snapshot()\n\tstdoutJSON, outErr := json.Marshal(struct {\n\t\tRaw      []byte\n\t\tOverflow bool\n\t}{stdout, outOverflow})\n\tstderrJSON, errErr := json.Marshal(struct {\n\t\tRaw      []byte\n\t\tOverflow bool\n\t}{stderr, errOverflow})\n\tif outErr == nil {\n\t\toutErr = manualRestartCreate(c, 4, \"failure-stdout.json\", stdoutJSON)\n\t}\n\tif errErr == nil {\n\t\terrErr = manualRestartCreate(c, 4, \"failure-stderr.json\", stderrJSON)\n\t}\n\tif err := errors.Join(outErr, errErr); err != nil {\n\t\tt.Logf(\"diagnostic retention: %v\", err)\n\t}\n\traw, err = json.Marshal(child.rawFrames[:child.rawCount])\n\tif err == nil {\n\t\terr = manualRestartCreate(c, 4, \"failure-protocol.json\", raw)\n\t}\n\tif err != nil {\n\t\tt.Logf(\"raw prefix retention: %v\", err)\n\t}\n}\n\nfunc TestManualRollbackRestartController(t *testing.T) {\n\tif os.Getenv(\"GOSQLITE_MANUAL_FAULT_ROLLBACK\") != \"independently_authorized\" || os.Getenv(\"GOSQLITE_MANUAL_RESTART_ROLLBACK\") != \"independently_authorized\" {\n\t\tt.Fatal(\"NOT_RUN: separate manual restart authorization required\")\n\t}\n\tvar config manualRestartConfig\n\tif err := manualRestartAdmit(&config, rollbackrestart.Controller); err != nil {\n\t\tt.Fatalf(\"ADMISSION_BLOCKED: %v\", err)\n\t}\n\tif err := manualRestartController(t, &config); err != nil {\n\t\tmanualRestartFailure(t, &config, err)\n\t\tt.Fatalf(\"INCOMPLETE: %v; all artifacts retained\", err)\n\t}\n\tt.Log(\"SCENARIO_EVIDENCE_COMPLETE: process-exit scenario only; PhysicalValidated=false; Pass=false; all artifacts retained\")\n}\nfunc TestManualRollbackRestartWriter(t *testing.T) {\n\tif os.Getenv(\"GOSQLITE_MANUAL_FAULT_ROLLBACK\") != \"independently_authorized\" || os.Getenv(\"GOSQLITE_MANUAL_RESTART_ROLLBACK\") != \"independently_authorized\" {\n\t\tt.Fatal(\"NOT_RUN: separate manual restart authorization required\")\n\t}\n\tvar config manualRestartConfig\n\tif err := manualRestartAdmit(&config, rollbackrestart.Writer); err != nil {\n\t\tt.Fatalf(\"ADMISSION_BLOCKED: %v\", err)\n\t}\n\tif err := manualRestartFamilies(&config, false); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tchannel, err := pager.NewManualRollbackRestartChannel(config.plan, rollbackrestart.Writer)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\timage, err := manualRestartID(config.selfInfo)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif err := channel.Ready(image, config.plan.Spec().Binding.Binary); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif err := manualRestartWriter(&config, channel); err != nil {\n\t\tt.Fatalf(\"INCOMPLETE: %v; no cleanup retry\", err)\n\t}\n}\nfunc TestManualRollbackRestartRecovery(t *testing.T) {\n\tif os.Getenv(\"GOSQLITE_MANUAL_FAULT_ROLLBACK\") != \"independently_authorized\" || os.Getenv(\"GOSQLITE_MANUAL_RESTART_ROLLBACK\") != \"independently_authorized\" {\n\t\tt.Fatal(\"NOT_RUN: separate manual restart authorization required\")\n\t}\n\tvar config manualRestartConfig\n\tif err := manualRestartAdmit(&config, rollbackrestart.Recovery); err != nil {\n\t\tt.Fatalf(\"ADMISSION_BLOCKED: %v\", err)\n\t}\n\tif err := manualRestartFamilies(&config, false); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tchannel, err := pager.NewManualRollbackRestartChannel(config.plan, rollbackrestart.Recovery)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\timage, err := manualRestartID(config.selfInfo)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif err := channel.Ready(image, config.plan.Spec().Binding.Binary); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif err := manualRestartRecovery(&config, channel); err != nil {\n\t\tt.Fatalf(\"INCOMPLETE: %v; no recovery retry\", err)\n\t}\n}\n",
	rollbackRestartModel:     "// Package rollbackrestart is a bounded, descriptor-free value contract for five\n// process-exit scenarios. Consistent caller-supplied values are not physical\n// evidence. This package never opens, closes, syncs, spawns, waits, or recovers.\npackage rollbackrestart\n\nimport (\n\t\"bytes\"\n\t\"crypto/sha256\"\n\t\"encoding/binary\"\n\t\"encoding/hex\"\n\t\"encoding/json\"\n\n\t\"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault\"\n)\n\nconst (\n\tSchemaVersion            uint16 = 1\n\tSuite                           = \"rollback-restart-v1\"\n\tMaxFrames                       = 64\n\tMaxFrameBytes                   = 4096\n\tMaxAdapterEvents                = 128\n\tMaxSnapshotBytes                = 32768\n\tMaxArtifactManifestBytes        = 16384\n\tMaxScenarioFrames               = 54\n\tMaxIDLength                     = 128\n\tExpectedExitCode                = 86\n\tAbortExitCode                   = 87\n\tCallCount                       = 18\n)\n\ntype Case uint8\n\nconst (\n\tCaseUnknown Case = iota\n\tControl\n\tBeforeJCreate\n\tAfterTablePage\n\tAfterJUnlink\n\tAcknowledgedExit\n)\n\nfunc Cases() [5]Case {\n\treturn [5]Case{Control, BeforeJCreate, AfterTablePage, AfterJUnlink, AcknowledgedExit}\n}\nfunc CaseName(c Case) string {\n\tswitch c {\n\tcase Control:\n\t\treturn \"control\"\n\tcase BeforeJCreate:\n\t\treturn \"before-jcreate\"\n\tcase AfterTablePage:\n\t\treturn \"after-table-page\"\n\tcase AfterJUnlink:\n\t\treturn \"after-junlink\"\n\tcase AcknowledgedExit:\n\t\treturn \"acknowledged-exit\"\n\tdefault:\n\t\treturn \"\"\n\t}\n}\nfunc ParseCase(s string) (Case, bool) {\n\tfor _, c := range Cases() {\n\t\tif CaseName(c) == s {\n\t\t\treturn c, true\n\t\t}\n\t}\n\treturn CaseUnknown, false\n}\n\n// Digest is exactly 64 lowercase hexadecimal bytes when present. It is a label\n// unless a named pure digest helper derives it from a bounded value below.\ntype Digest string\ntype FileIdentity struct{ Device, Inode uint64 }\ntype Role uint8\n\nconst (\n\tRoleUnknown Role = iota\n\tController\n\tWriter\n\tRecovery\n)\n\ntype RoleIdentity struct {\n\tRole       Role\n\tGeneration uint64\n\tChallenge  Digest\n}\ntype NativeIdentity struct {\n\tOS         string\n\tFilesystem string\n\tSectorSize uint32\n\tPageSize   uint32\n}\ntype Binding struct {\n\tSuite    string\n\tProfile  rollbackfault.Profile\n\tCase     Case\n\tTrial    string\n\tSource   string\n\tManifest Digest\n\tTool     Digest\n\tCompile  Digest\n\tBinary   Digest\n\tNative   NativeIdentity\n\tRoot     FileIdentity\n\tBaseline Digest\n}\ntype PlanSpec struct {\n\tBinding         Binding\n\tController      RoleIdentity\n\tWriter          RoleIdentity\n\tRecovery        RoleIdentity\n\tAdapterIdentity rollbackfault.Identity\n\tTicket          uint64\n\tImage           FileIdentity\n}\ntype Plan struct {\n\tspec  PlanSpec\n\tvalid bool\n}\n\nfunc (p Plan) Spec() PlanSpec { return p.spec }\n\n// FrameKind is closed. Parent-only observations cannot be replaced by an\n// identically worded child report or by output on stdout/stderr.\ntype FrameKind uint8\n\nconst (\n\tFrameUnknown FrameKind = iota\n\tParentAdmitted\n\tBaselineCheckedAndClosed\n\tWriterSpawned\n\tChildReadyBound\n\tArmGranted\n\tTransactionArmed\n\tAdapterObserved\n\tTargetReached\n\tPublicCommitReturned\n\tAcknowledgementDurable\n\tExitGranted\n\tNormalFinishGranted\n\tWriterFinished\n\tWriterWaitObserved\n\tOriginalsFrozen\n\tRecoverySpawned\n\tRecoveryReadyBound\n\tRecoveryReadbackGranted\n\tReadbackResult\n\tRecoveryWaitObserved\n\tArtifactsRechecked\n\tScenarioEvidenceComplete\n\tFailure\n)\n\ntype ProcessState uint8\n\nconst (\n\tProcessUnknown ProcessState = iota\n\tProcessRunning\n\tProcessExited\n\tProcessSignaled\n)\n\n// Handle is a parent's process-object ordinal, never an externally supplied PID\n// capability. ActualImage attests a /proc/self/exe observation, not argv[0].\ntype ProcessEvidence struct {\n\tPID           int64\n\tHandle        uint64\n\tImage         FileIdentity\n\tImageHash     Digest\n\tActualImage   bool\n\tSpawnObserved bool\n\tWaitObserved  bool\n\tState         ProcessState\n\tExitCode      int\n\tSignal        int\n\tTimedOut      bool\n}\ntype CommitValue uint8\n\nconst (\n\tCommitUnknown CommitValue = iota\n\tCommitNil\n\tCommitError\n\tCommitUncertain\n)\n\ntype CommitEvidence struct {\n\tCalls             uint8\n\tValue             CommitValue\n\tCapturedAdjacent  bool\n\tCapturedUnchanged bool\n\tPublicDone        bool\n\tPublicDetached    bool\n\tActiveRows        uint32\n\tMain              rollbackfault.OwnerState\n\tJournal           rollbackfault.OwnerState\n\tCleanup           rollbackfault.OwnerState\n}\n\n// Ack is a receipt for one immutable O_EXCL file in the separate control\n// directory. Sync and close ordering is frozen by the numbered steps. Numbers\n// are sequence labels, not proof that the native operations were performed.\ntype AckEvidence struct {\n\tCommitSequence      uint8\n\tCommitFrameSHA256   Digest\n\tSHA256              Digest\n\tFile                FileIdentity\n\tBytes               int64\n\tWritten             int64\n\tExclusive           bool\n\tWriteStep           uint8\n\tFileSyncStep        uint8\n\tFileCloseStep       uint8\n\tDirectorySyncStep   uint8\n\tDirectoryCloseStep  uint8\n\tFileCloseCalls      uint8\n\tDirectoryCloseCalls uint8\n\tErrorToken          uint64\n}\ntype Diagnostic uint8\n\nconst (\n\tDiagnosticNone Diagnostic = iota\n\tNotRun\n\tAdmissionBlocked\n\tProtocolInvalid\n\tTargetNotReached\n\tUnderlyingIOFailed\n\tCommitUncertainDiagnostic\n\tAckFailed\n\tAckUncertain\n\tChildExitUnexpected\n\tChildLive\n\tWaitUnknown\n\tRecoveryFailed\n\tReadbackFailed\n\tOwnerPending\n\tOwnerUnknown\n\tArtifactChanged\n\tEvidenceComplete\n)\n\n// Each encoded Frame contains exactly this schema, with no extension fields or\n// arbitrary payload. Sequence is global across the two one-way child pipes and\n// the parent's private observations. At most 36 AdapterObserved frames occur.\ntype Frame struct {\n\tSchema     uint16\n\tBinding    Binding\n\tSender     RoleIdentity\n\tSequence   uint8\n\tKind       FrameKind\n\tTicket     uint64              `json:\",omitzero\"`\n\tAdapter    rollbackfault.Event `json:\",omitzero\"`\n\tProcess    ProcessEvidence     `json:\",omitzero\"`\n\tCommit     CommitEvidence      `json:\",omitzero\"`\n\tAck        AckEvidence         `json:\",omitzero\"`\n\tDigest     Digest              `json:\",omitzero\"`\n\tDiagnostic Diagnostic          `json:\",omitzero\"`\n}\ntype Problem uint8\n\nconst (\n\tProblemNone Problem = iota\n\tProblemPlan\n\tProblemIdentity\n\tProblemSchema\n\tProblemEncoding\n\tProblemFrameLimit\n\tProblemSequence\n\tProblemOrder\n\tProblemPayload\n\tProblemAdapter\n\tProblemTarget\n\tProblemUnderlyingIO\n\tProblemCommit\n\tProblemAck\n\tProblemProcess\n\tProblemIncomplete\n\tProblemSealed\n\tProblemLifecycle\n\tProblemArtifacts\n\tProblemRecovery\n\tProblemSQL\n\tProblemPhysicalClaim\n)\nconst NoProblem = ProblemNone\n\n// Observation is deliberately not accepted as an input by Observe. Forging one\n// cannot substitute for replaying the private Receipt's admitted frame values.\ntype Observation struct {\n\tEvidenceConsistent bool\n\tScenarioObserved   bool\n\tStatus             Diagnostic\n\tProblem            Problem\n\tFrameCount         uint8\n\tAdapterEventCount  uint8\n\tCommit             CommitValue\n\tAcknowledged       bool\n\tWriterWaited       bool\n\tRecoveryWaited     bool\n\tSQLState           rollbackfault.SQLState\n}\n\nfunc (Observation) PhysicalValidated() bool { return false }\nfunc (Observation) Pass() bool              { return false }\n\ntype OwnerDisposition uint8\n\nconst (\n\tDispositionUnknown OwnerDisposition = iota\n\tRetiredNormally\n\tNotAcquired\n\tAbandonedByExpectedProcessExit\n)\n\ntype WriterLifecycle struct {\n\tOpenCalls      uint8\n\tBeginCalls     uint8\n\tDMLCalls       uint8\n\tArmCalls       uint8\n\tCommitCalls    uint8\n\tPublicReturned bool\n\tFinishCalls    uint8\n\tDisarmCalls    uint8\n\tRollbackCalls  uint8\n\tDBCloseCalls   uint8\n\tMain           OwnerDisposition\n\tJournal        OwnerDisposition\n\tCleanup        OwnerDisposition\n}\ntype RecoveryEvidence struct {\n\tGeneration           uint64\n\tCalls                uint8\n\tLimits               rollbackfault.SQLRecoveryLimits\n\tError                rollbackfault.ErrorKind\n\tErrorToken           uint64\n\tJournalPresentBefore bool\n\tJournalPresentAfter  bool\n\tReplayed             bool\n\tReplayedPages        uint32\n\tOriginalPages        uint32\n\tOwnersRetired        bool\n\tReadOnly             bool\n\tNoCreate             bool\n\tOpenCalls            uint8\n\tCheckMaxPages        uint32\n\tCheckMaxPageReads    uint32\n\tNoRetry              bool\n}\ntype ArtifactRole uint8\n\nconst (\n\tBaselineArtifact ArtifactRole = iota\n\tTrialArtifact\n\tTrialJournalArtifact\n\tPostExitArtifact\n\tPostExitJournalArtifact\n\tReplayArtifact\n\tReplayJournalArtifact\n\tAcknowledgementArtifact\n\tArtifactCount\n)\n\ntype ArtifactStamp struct {\n\tPath    string\n\tFile    FileIdentity\n\tExists  bool\n\tRegular bool\n\tLinks   uint64\n\tBytes   int64\n\tSHA256  Digest\n\tPages   [3]Digest\n}\ntype ArtifactEvidence struct {\n\tRole                ArtifactRole\n\tBefore              ArtifactStamp\n\tAfter               ArtifactStamp\n\tExclusiveCopy       bool\n\tCompleteRead        bool\n\tCompleteWrite       bool\n\tFileSynced          bool\n\tFileClosedOnce      bool\n\tDirectorySynced     bool\n\tDirectoryClosedOnce bool\n}\n\n// RuntimeEvidence is conjunctive. None of these independent requirements is\n// inferred from a successful SQL result, exit code, or missing diagnostics.\ntype RuntimeEvidence struct {\n\tEnvFirst                      bool\n\tExactRoleSelectors            bool\n\tFixedAllowlistEnvironment     bool\n\tCountOne                      bool\n\tParallelOne                   bool\n\tCPUOne                        bool\n\tShuffleOff                    bool\n\tTimeoutZero                   bool\n\tPrivateRoot0700               bool\n\tNoSymlinkAncestors            bool\n\tRootUnchanged                 bool\n\tTwoDedicatedPipes             bool\n\tPipeOwnersRecorded            bool\n\tUnusedPipeEndsClosedOnce      bool\n\tBoundedDiagnostics            bool\n\tNoInheritedDatabaseOwners     bool\n\tParentOwnersClosedBeforeSpawn bool\n\tFreshRegistryEachChild        bool\n\tOnlyParentSpawns              bool\n\tNoSignalOrKill                bool\n\tNoRetries                     bool\n\tOriginalsNeverSyncedAfterExit bool\n\tArtifactsRetained             bool\n\tAdapterSameTransaction        bool\n\tAdapterSameScope              bool\n\tAdapterSameOwners             bool\n\tObserverOutsidePrivateMutex   bool\n\tOriginalTxLockHeldAtCut       bool\n\tNoCallbackNativeReplacement   bool\n}\ntype ScenarioEvidence struct {\n\tBinding          Binding\n\tBefore           rollbackfault.SQLSnapshotEvidence\n\tAfter            rollbackfault.SQLSnapshotEvidence\n\tBeforeGeneration uint64\n\tAfterGeneration  uint64\n\tLifecycle        WriterLifecycle\n\tRecovery         RecoveryEvidence\n\tArtifacts        [ArtifactCount]ArtifactEvidence\n\tRuntime          RuntimeEvidence\n\tPhysicalClaim    bool\n}\n\n// Native transport writes all returned bytes once with checked short writes.\n// It must read the 4-byte length first, reject it before allocating if outside\n// 1..MaxFrameBytes, then read exactly that many bytes. This pure decoder takes\n// the complete prefix+body value, rejects truncation and trailing bytes, and\n// rejects every alternate JSON encoding (including duplicates and unknowns).\nfunc EncodeFrame(f Frame) ([]byte, Problem) {\n\tif problem := ValidateFrame(f); problem != ProblemNone {\n\t\treturn nil, problem\n\t}\n\tbody, err := json.Marshal(f)\n\tif err != nil || len(body) == 0 || len(body) > MaxFrameBytes {\n\t\treturn nil, ProblemEncoding\n\t}\n\tout := make([]byte, 4+len(body))\n\tbinary.BigEndian.PutUint32(out[:4], uint32(len(body)))\n\tcopy(out[4:], body)\n\treturn out, ProblemNone\n}\nfunc DecodeFrame(raw []byte) (Frame, Problem) {\n\tif len(raw) < 5 || len(raw) > MaxFrameBytes+4 {\n\t\treturn Frame{}, ProblemEncoding\n\t}\n\tn := binary.BigEndian.Uint32(raw[:4])\n\tif n == 0 || n > MaxFrameBytes || int(n) != len(raw)-4 {\n\t\treturn Frame{}, ProblemEncoding\n\t}\n\tvar f Frame\n\tdecoder := json.NewDecoder(bytes.NewReader(raw[4:]))\n\tdecoder.DisallowUnknownFields()\n\tif err := decoder.Decode(&f); err != nil {\n\t\treturn Frame{}, ProblemEncoding\n\t}\n\tencoded, problem := EncodeFrame(f)\n\tif problem != ProblemNone || !bytes.Equal(encoded, raw) {\n\t\treturn Frame{}, ProblemEncoding\n\t}\n\treturn f, ProblemNone\n}\nfunc FrameDigest(f Frame) (Digest, Problem) {\n\tencoded, problem := EncodeFrame(f)\n\tif problem != ProblemNone {\n\t\treturn \"\", problem\n\t}\n\treturn digestBytes(encoded), ProblemNone\n}\nfunc digestBytes(raw []byte) Digest {\n\tsum := sha256.Sum256(raw)\n\treturn Digest(hex.EncodeToString(sum[:]))\n}\n\nfunc validDigest(d Digest) bool {\n\tif len(d) != 64 {\n\t\treturn false\n\t}\n\tnonzero := false\n\tfor _, c := range d {\n\t\tif !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {\n\t\t\treturn false\n\t\t}\n\t\tif c != '0' {\n\t\t\tnonzero = true\n\t\t}\n\t}\n\treturn nonzero\n}\nfunc validID(s string) bool {\n\tif len(s) == 0 || len(s) > MaxIDLength || s == \"0\" {\n\t\treturn false\n\t}\n\tfor _, c := range s {\n\t\tif c < '!' || c > '~' {\n\t\t\treturn false\n\t\t}\n\t}\n\treturn true\n}\nfunc validFile(f FileIdentity) bool { return f.Device != 0 && f.Inode != 0 }\nfunc validBinding(b Binding) bool {\n\treturn b.Suite == Suite && b.Profile == rollbackfault.PlainRollback4096 && CaseName(b.Case) != \"\" && validID(b.Trial) && validID(b.Source) && validDigest(b.Manifest) && validDigest(b.Tool) && validDigest(b.Compile) && validDigest(b.Binary) && validDigest(b.Baseline) && b.Native.OS == \"linux\" && validID(b.Native.Filesystem) && len(b.Native.Filesystem) <= 32 && b.Native.PageSize == 4096 && b.Native.SectorSize == 4096 && validFile(b.Root)\n}\nfunc validRole(r RoleIdentity) bool {\n\treturn r.Role >= Controller && r.Role <= Recovery && r.Generation != 0 && validDigest(r.Challenge)\n}\nfunc validAdapterIdentity(id rollbackfault.Identity) bool {\n\treturn validID(id.Suite) && validID(id.Source) && validID(id.Tool) && validID(id.Binary) && validID(id.Manifest) && validID(id.Trial) && validID(id.Scope) && id.Profile == rollbackfault.PlainRollback4096\n}\nfunc copyText(s string) string {\n\tvar buf [MaxIDLength]byte\n\tn := copy(buf[:], s)\n\treturn string(buf[:n])\n}\nfunc copyDigest(d Digest) Digest { return Digest(copyText(string(d))) }\nfunc copyBinding(b Binding) Binding {\n\tb.Suite, b.Trial, b.Source = copyText(b.Suite), copyText(b.Trial), copyText(b.Source)\n\tb.Manifest, b.Tool, b.Compile, b.Binary, b.Baseline = copyDigest(b.Manifest), copyDigest(b.Tool), copyDigest(b.Compile), copyDigest(b.Binary), copyDigest(b.Baseline)\n\tb.Native.OS, b.Native.Filesystem = copyText(b.Native.OS), copyText(b.Native.Filesystem)\n\treturn b\n}\nfunc NewPlan(s PlanSpec) (Plan, Problem) {\n\tif !validBinding(s.Binding) || !validRole(s.Controller) || !validRole(s.Writer) || !validRole(s.Recovery) || s.Controller.Role != Controller || s.Writer.Role != Writer || s.Recovery.Role != Recovery || s.Ticket == 0 || !validFile(s.Image) {\n\t\treturn Plan{}, ProblemPlan\n\t}\n\troles := [3]RoleIdentity{s.Controller, s.Writer, s.Recovery}\n\tfor i, role := range roles {\n\t\tfor j := 0; j < i; j++ {\n\t\t\tif role.Generation == roles[j].Generation || role.Challenge == roles[j].Challenge {\n\t\t\t\treturn Plan{}, ProblemIdentity\n\t\t\t}\n\t\t}\n\t}\n\tb, id := s.Binding, s.AdapterIdentity\n\tif !validAdapterIdentity(id) || id.Suite != b.Suite || id.Source != b.Source || id.Tool != string(b.Tool) || id.Binary != string(b.Binary) || id.Manifest != string(b.Manifest) || id.Trial != b.Trial || id.Profile != b.Profile {\n\t\treturn Plan{}, ProblemIdentity\n\t}\n\ts.Binding = copyBinding(b)\n\ts.Controller.Challenge, s.Writer.Challenge, s.Recovery.Challenge = copyDigest(s.Controller.Challenge), copyDigest(s.Writer.Challenge), copyDigest(s.Recovery.Challenge)\n\tid.Suite, id.Source, id.Tool, id.Binary = copyText(id.Suite), copyText(id.Source), copyText(id.Tool), copyText(id.Binary)\n\tid.Manifest, id.Trial, id.Scope = copyText(id.Manifest), copyText(id.Trial), copyText(id.Scope)\n\ts.AdapterIdentity = id\n\treturn Plan{spec: s, valid: true}, ProblemNone\n}\n\n// Target's call number is one-based in the original 18-call table. It is not\n// the adapter ordinal, which also includes RunStarted and every Returned.\nfunc Target(c Case) (uint8, rollbackfault.EventKind) {\n\tswitch c {\n\tcase BeforeJCreate:\n\t\treturn 1, rollbackfault.Intent\n\tcase AfterTablePage:\n\t\treturn 14, rollbackfault.Returned\n\tcase AfterJUnlink:\n\t\treturn 17, rollbackfault.Returned\n\tdefault:\n\t\treturn 0, rollbackfault.EventUnknown\n\t}\n}\nfunc ExpectedCalls() [CallCount]rollbackfault.Call {\n\treturn [CallCount]rollbackfault.Call{\n\t\t{Point: rollbackfault.JournalCreate, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},\n\t\t{Point: rollbackfault.JournalInvalid, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12, Length: 4096},\n\t\t{Point: rollbackfault.DBOriginalRead, Occurrence: 1, Role: rollbackfault.Database, Owner: 11, Length: 4096},\n\t\t{Point: rollbackfault.JournalRecord, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12, Offset: 4096, Length: 4104},\n\t\t{Point: rollbackfault.DBOriginalRead, Occurrence: 2, Role: rollbackfault.Database, Owner: 11, Offset: 4096, Length: 4096},\n\t\t{Point: rollbackfault.JournalRecord, Occurrence: 2, Role: rollbackfault.Journal, Owner: 12, Offset: 8200, Length: 4104},\n\t\t{Point: rollbackfault.DBOriginalRead, Occurrence: 3, Role: rollbackfault.Database, Owner: 11, Offset: 8192, Length: 4096},\n\t\t{Point: rollbackfault.JournalRecord, Occurrence: 3, Role: rollbackfault.Journal, Owner: 12, Offset: 12304, Length: 4104},\n\t\t{Point: rollbackfault.JournalBodySync, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},\n\t\t{Point: rollbackfault.JournalValidHeader, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12, Length: 12},\n\t\t{Point: rollbackfault.JournalHeaderSync, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},\n\t\t{Point: rollbackfault.PublishDirectorySync, Occurrence: 1, Role: rollbackfault.Directory, Owner: 13},\n\t\t{Point: rollbackfault.DBPage, Occurrence: 1, Role: rollbackfault.Database, Owner: 11, Length: 4096},\n\t\t{Point: rollbackfault.DBPage, Occurrence: 2, Role: rollbackfault.Database, Owner: 11, Offset: 4096, Length: 4096},\n\t\t{Point: rollbackfault.DBPage, Occurrence: 3, Role: rollbackfault.Database, Owner: 11, Offset: 8192, Length: 4096},\n\t\t{Point: rollbackfault.DBSync, Occurrence: 1, Role: rollbackfault.Database, Owner: 11},\n\t\t{Point: rollbackfault.JournalUnlink, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},\n\t\t{Point: rollbackfault.FinalDirectorySync, Occurrence: 1, Role: rollbackfault.Directory, Owner: 13},\n\t}\n}\nfunc ExpectedAdapterEvents(c Case) uint8 {\n\tswitch c {\n\tcase BeforeJCreate:\n\t\treturn 2\n\tcase AfterTablePage:\n\t\treturn 29\n\tcase AfterJUnlink:\n\t\treturn 35\n\tcase Control, AcknowledgedExit:\n\t\treturn 37\n\tdefault:\n\t\treturn 0\n\t}\n}\nfunc ExpectedSQLState(c Case) rollbackfault.SQLState {\n\tswitch c {\n\tcase BeforeJCreate, AfterTablePage:\n\t\treturn rollbackfault.SQLOld\n\tcase Control, AfterJUnlink, AcknowledgedExit:\n\t\treturn rollbackfault.SQLNew\n\tdefault:\n\t\treturn rollbackfault.SQLStateUnknown\n\t}\n}\n\nfunc ValidateFrame(f Frame) Problem {\n\tif f.Schema != SchemaVersion {\n\t\treturn ProblemSchema\n\t}\n\tif !validBinding(f.Binding) || !validRole(f.Sender) {\n\t\treturn ProblemIdentity\n\t}\n\tif f.Sequence == 0 || int(f.Sequence) > MaxFrames {\n\t\treturn ProblemSequence\n\t}\n\tif f.Kind <= FrameUnknown || f.Kind > Failure {\n\t\treturn ProblemPayload\n\t}\n\tif f.Digest != \"\" && !validDigest(f.Digest) {\n\t\treturn ProblemPayload\n\t}\n\tif f.Process.ImageHash != \"\" && !validDigest(f.Process.ImageHash) {\n\t\treturn ProblemPayload\n\t}\n\tif f.Ack.SHA256 != \"\" && !validDigest(f.Ack.SHA256) {\n\t\treturn ProblemPayload\n\t}\n\tif f.Ack.CommitFrameSHA256 != \"\" && !validDigest(f.Ack.CommitFrameSHA256) {\n\t\treturn ProblemPayload\n\t}\n\tif f.Adapter != (rollbackfault.Event{}) && !validAdapterIdentity(f.Adapter.Identity) {\n\t\treturn ProblemIdentity\n\t}\n\tif f.Process.State > ProcessSignaled || f.Process.Signal < 0 || f.Process.Signal > 64 || f.Process.ExitCode < 0 || f.Process.ExitCode > 255 || f.Commit.Value > CommitUncertain || f.Diagnostic > EvidenceComplete {\n\t\treturn ProblemPayload\n\t}\n\tif f.Commit.Main > rollbackfault.OwnerNotAcquired || f.Commit.Journal > rollbackfault.OwnerNotAcquired || f.Commit.Cleanup > rollbackfault.OwnerNotAcquired {\n\t\treturn ProblemPayload\n\t}\n\tif f.Adapter != (rollbackfault.Event{}) {\n\t\ta := f.Adapter\n\t\tif a.Kind != rollbackfault.RunStarted && a.Kind != rollbackfault.Intent && a.Kind != rollbackfault.Returned {\n\t\t\treturn ProblemPayload\n\t\t}\n\t\tif a.Action != rollbackfault.Pass || a.Call.Point > rollbackfault.CleanupDirectorySync || a.Call.Role > rollbackfault.Directory || a.Result.NativeError > rollbackfault.Underlying || a.Result.ReturnedError > rollbackfault.Underlying || a.Result.NormalizedError > rollbackfault.Underlying || a.Result.Effect > rollbackfault.EffectPossible || a.Commit != (rollbackfault.CommitResult{}) || a.Owners != (rollbackfault.OwnersResult{}) || a.Validation != rollbackfault.ValidationAbsent || a.Rejection != rollbackfault.NoProblem || a.RejectedAt != 0 {\n\t\t\treturn ProblemPayload\n\t\t}\n\t}\n\tkind := f.Kind\n\tf.Schema, f.Binding, f.Sender, f.Sequence, f.Kind = 0, Binding{}, RoleIdentity{}, 0, FrameUnknown\n\tswitch kind {\n\tcase ParentAdmitted, WriterSpawned, ChildReadyBound, WriterWaitObserved, RecoverySpawned, RecoveryReadyBound, RecoveryWaitObserved:\n\t\tf.Process = ProcessEvidence{}\n\tcase BaselineCheckedAndClosed, OriginalsFrozen, ReadbackResult, ArtifactsRechecked, NormalFinishGranted:\n\t\tf.Digest = \"\"\n\tcase TransactionArmed, AdapterObserved, TargetReached:\n\t\tf.Adapter, f.Ticket = rollbackfault.Event{}, 0\n\tcase PublicCommitReturned:\n\t\tf.Commit = CommitEvidence{}\n\tcase AcknowledgementDurable:\n\t\tf.Ack = AckEvidence{}\n\tcase ExitGranted:\n\t\tf.Ack, f.Digest = AckEvidence{}, \"\"\n\tcase Failure:\n\t\tif f.Diagnostic == DiagnosticNone || f.Diagnostic == EvidenceComplete {\n\t\t\treturn ProblemPayload\n\t\t}\n\t\tf.Diagnostic = DiagnosticNone\n\t}\n\tif f != (Frame{}) {\n\t\treturn ProblemPayload\n\t}\n\treturn ProblemNone\n}\n\ntype Receipt struct {\n\tplan            Plan\n\tframes          [MaxFrames]Frame\n\tcount           uint8\n\tnext            FrameKind\n\tproblem         Problem\n\tstatus          Diagnostic\n\tadapterCount    uint8\n\tlastAdapter     rollbackfault.Event\n\ttargetHash      Digest\n\tcommitHash      Digest\n\tcommitBodyHash  Digest\n\tcommitBodyBytes int64\n\tcommitSequence  uint8\n\tack             AckEvidence\n\tparent          ProcessEvidence\n\twriter          ProcessEvidence\n\trecovery        ProcessEvidence\n\tbaselineDigest  Digest\n\tfrozenDigest    Digest\n\treadbackDigest  Digest\n\tcheckedDigest   Digest\n\twriterWaited    bool\n\trecoveryWaited  bool\n\tsealed          bool\n}\n\nfunc NewReceipt(p Plan) (Receipt, Problem) {\n\tif !p.valid {\n\t\treturn Receipt{}, ProblemPlan\n\t}\n\treturn Receipt{plan: p, next: ParentAdmitted}, ProblemNone\n}\nfunc (r Receipt) Frames() ([MaxFrames]Frame, uint8) { return r.frames, r.count }\nfunc (r *Receipt) Append(f Frame) Problem {\n\tif r.sealed {\n\t\treturn ProblemSealed\n\t}\n\tif r.problem != ProblemNone {\n\t\treturn r.problem\n\t}\n\tif !r.plan.valid {\n\t\tr.problem = ProblemPlan\n\t\treturn r.problem\n\t}\n\tif int(r.count) >= MaxFrames {\n\t\tr.problem = ProblemFrameLimit\n\t\treturn r.problem\n\t}\n\tif problem := ValidateFrame(f); problem != ProblemNone {\n\t\tr.problem, r.status = problem, ProtocolInvalid\n\t\treturn problem\n\t}\n\tif f.Binding != r.plan.spec.Binding || f.Sender != r.sender(f.Kind) {\n\t\tr.problem, r.status = ProblemIdentity, ProtocolInvalid\n\t\treturn r.problem\n\t}\n\tif f.Sequence != r.count+1 {\n\t\tr.problem, r.status = ProblemSequence, ProtocolInvalid\n\t\treturn r.problem\n\t}\n\t// Freeze admitted strings instead of retaining an input substring's backing\n\t// storage. All remaining variable fields have already been bounded above.\n\tf.Binding, f.Sender = r.plan.spec.Binding, r.sender(f.Kind)\n\tif f.Adapter != (rollbackfault.Event{}) {\n\t\tif f.Adapter.Identity != r.plan.spec.AdapterIdentity {\n\t\t\tr.problem, r.status = ProblemIdentity, ProtocolInvalid\n\t\t\treturn r.problem\n\t\t}\n\t\tf.Adapter.Identity = r.plan.spec.AdapterIdentity\n\t}\n\tf.Digest, f.Process.ImageHash = copyDigest(f.Digest), copyDigest(f.Process.ImageHash)\n\tf.Ack.SHA256, f.Ack.CommitFrameSHA256 = copyDigest(f.Ack.SHA256), copyDigest(f.Ack.CommitFrameSHA256)\n\tr.frames[r.count], r.count = f, r.count+1\n\tr.problem = r.accept(f)\n\tif r.problem != ProblemNone && r.status == DiagnosticNone {\n\t\tr.status = ProtocolInvalid\n\t}\n\treturn r.problem\n}\nfunc (r Receipt) sender(kind FrameKind) RoleIdentity {\n\tswitch kind {\n\tcase ChildReadyBound, TransactionArmed, AdapterObserved, TargetReached, PublicCommitReturned, WriterFinished:\n\t\treturn r.plan.spec.Writer\n\tcase RecoveryReadyBound, ReadbackResult:\n\t\treturn r.plan.spec.Recovery\n\tdefault:\n\t\treturn r.plan.spec.Controller\n\t}\n}\nfunc (r *Receipt) accept(f Frame) Problem {\n\tif f.Kind == Failure {\n\t\tif r.next == FrameUnknown {\n\t\t\treturn ProblemOrder\n\t\t}\n\t\tr.status, r.next = f.Diagnostic, FrameUnknown\n\t\treturn ProblemIncomplete\n\t}\n\tif f.Kind != r.next || r.next == FrameUnknown {\n\t\treturn ProblemOrder\n\t}\n\tspec, which := r.plan.spec, r.plan.spec.Binding.Case\n\tswitch f.Kind {\n\tcase ParentAdmitted:\n\t\tif !validSelf(f.Process, spec) || f.Process.Handle != 0 {\n\t\t\treturn ProblemProcess\n\t\t}\n\t\tr.parent, r.next = f.Process, BaselineCheckedAndClosed\n\tcase BaselineCheckedAndClosed:\n\t\tif !validDigest(f.Digest) {\n\t\t\treturn ProblemPayload\n\t\t}\n\t\tr.baselineDigest, r.next = f.Digest, WriterSpawned\n\tcase WriterSpawned:\n\t\tif !validSpawn(f.Process, spec) || f.Process.PID == r.parent.PID {\n\t\t\treturn ProblemProcess\n\t\t}\n\t\tr.writer, r.next = f.Process, ChildReadyBound\n\tcase ChildReadyBound:\n\t\tif !validReady(f.Process, r.writer, spec) {\n\t\t\treturn ProblemProcess\n\t\t}\n\t\tr.next = ArmGranted\n\tcase ArmGranted:\n\t\tr.next = TransactionArmed\n\tcase TransactionArmed:\n\t\twant := rollbackfault.Event{Identity: spec.AdapterIdentity, Ordinal: 1, Kind: rollbackfault.RunStarted}\n\t\tif f.Ticket != spec.Ticket || f.Adapter != want {\n\t\t\treturn ProblemAdapter\n\t\t}\n\t\tr.adapterCount, r.next = 1, AdapterObserved\n\tcase AdapterObserved:\n\t\tif f.Ticket != spec.Ticket || r.adapterCount < 1 || r.adapterCount >= 37 {\n\t\t\treturn ProblemAdapter\n\t\t}\n\t\tindex := (r.adapterCount - 1) / 2\n\t\tkind := rollbackfault.Intent\n\t\tif r.adapterCount%2 == 0 {\n\t\t\tkind = rollbackfault.Returned\n\t\t}\n\t\tcalls := ExpectedCalls()\n\t\twant := rollbackfault.Event{Identity: spec.AdapterIdentity, Ordinal: r.adapterCount + 1, Kind: kind, Call: calls[index]}\n\t\tif kind == rollbackfault.Returned {\n\t\t\twant.Result = successfulResult(want.Call)\n\t\t\tif f.Adapter.Result != want.Result {\n\t\t\t\tr.status = UnderlyingIOFailed\n\t\t\t\treturn ProblemUnderlyingIO\n\t\t\t}\n\t\t}\n\t\tif f.Adapter != want {\n\t\t\treturn ProblemAdapter\n\t\t}\n\t\tr.adapterCount++\n\t\tr.lastAdapter = f.Adapter\n\t\ttargetCall, targetKind := Target(which)\n\t\tif targetCall == index+1 && targetKind == kind {\n\t\t\tr.next = TargetReached\n\t\t} else if r.adapterCount == 37 {\n\t\t\tr.next = PublicCommitReturned\n\t\t}\n\tcase TargetReached:\n\t\tif f.Ticket != spec.Ticket || f.Adapter != r.lastAdapter || r.adapterCount != ExpectedAdapterEvents(which) {\n\t\t\treturn ProblemTarget\n\t\t}\n\t\tvar problem Problem\n\t\tr.targetHash, problem = FrameDigest(f)\n\t\tif problem != ProblemNone {\n\t\t\treturn problem\n\t\t}\n\t\tr.next = ExitGranted\n\tcase PublicCommitReturned:\n\t\twant := CommitEvidence{Calls: 1, Value: CommitNil, CapturedAdjacent: true, CapturedUnchanged: true, PublicDone: true, PublicDetached: true, Main: rollbackfault.OwnerRetired, Journal: rollbackfault.OwnerRetired, Cleanup: rollbackfault.OwnerRetired}\n\t\tif f.Commit != want || (which != Control && which != AcknowledgedExit) {\n\t\t\tr.status = CommitUncertainDiagnostic\n\t\t\treturn ProblemCommit\n\t\t}\n\t\tvar problem Problem\n\t\tr.commitHash, problem = FrameDigest(f)\n\t\tif problem != ProblemNone {\n\t\t\treturn problem\n\t\t}\n\t\traw, encodeProblem := EncodeFrame(f)\n\t\tif encodeProblem != ProblemNone {\n\t\t\treturn encodeProblem\n\t\t}\n\t\tr.commitBodyHash, r.commitBodyBytes = digestBytes(raw[4:]), int64(len(raw)-4)\n\t\tr.commitSequence = f.Sequence\n\t\tr.next = NormalFinishGranted\n\t\tif which == AcknowledgedExit {\n\t\t\tr.next = AcknowledgementDurable\n\t\t}\n\tcase AcknowledgementDurable:\n\t\tif !validAck(f.Ack, r.commitSequence, r.commitHash, r.commitBodyHash, r.commitBodyBytes) {\n\t\t\tr.status = AckUncertain\n\t\t\treturn ProblemAck\n\t\t}\n\t\tr.ack, r.next = f.Ack, ExitGranted\n\tcase ExitGranted:\n\t\tif which == AcknowledgedExit {\n\t\t\tif f.Ack != r.ack || f.Digest != r.ack.SHA256 || !validDigest(f.Digest) {\n\t\t\t\treturn ProblemAck\n\t\t\t}\n\t\t} else if which == Control || f.Ack != (AckEvidence{}) || f.Digest != r.targetHash || !validDigest(f.Digest) {\n\t\t\treturn ProblemTarget\n\t\t}\n\t\tr.next = WriterWaitObserved\n\tcase NormalFinishGranted:\n\t\tif which != Control || f.Digest != r.commitHash {\n\t\t\treturn ProblemCommit\n\t\t}\n\t\tr.next = WriterFinished\n\tcase WriterFinished:\n\t\tr.next = WriterWaitObserved\n\tcase WriterWaitObserved:\n\t\tcode := ExpectedExitCode\n\t\tif which == Control {\n\t\t\tcode = 0\n\t\t}\n\t\tif !validWait(f.Process, r.writer, code) {\n\t\t\tr.status = waitStatus(f.Process)\n\t\t\treturn ProblemProcess\n\t\t}\n\t\tr.writerWaited, r.next = true, OriginalsFrozen\n\tcase OriginalsFrozen:\n\t\tif !r.writerWaited || !validDigest(f.Digest) {\n\t\t\treturn ProblemProcess\n\t\t}\n\t\tr.frozenDigest, r.next = f.Digest, RecoverySpawned\n\tcase RecoverySpawned:\n\t\tif !r.writerWaited || !validSpawn(f.Process, spec) || f.Process.Handle == r.writer.Handle || f.Process.PID == r.parent.PID {\n\t\t\treturn ProblemProcess\n\t\t}\n\t\tr.recovery, r.next = f.Process, RecoveryReadyBound\n\tcase RecoveryReadyBound:\n\t\tif !validReady(f.Process, r.recovery, spec) {\n\t\t\treturn ProblemProcess\n\t\t}\n\t\tr.next = RecoveryReadbackGranted\n\tcase RecoveryReadbackGranted:\n\t\tr.next = ReadbackResult\n\tcase ReadbackResult:\n\t\tif !validDigest(f.Digest) {\n\t\t\treturn ProblemPayload\n\t\t}\n\t\tr.readbackDigest, r.next = f.Digest, RecoveryWaitObserved\n\tcase RecoveryWaitObserved:\n\t\tif !validWait(f.Process, r.recovery, 0) {\n\t\t\tr.status = waitStatus(f.Process)\n\t\t\treturn ProblemProcess\n\t\t}\n\t\tr.recoveryWaited, r.next = true, ArtifactsRechecked\n\tcase ArtifactsRechecked:\n\t\tif !r.recoveryWaited || !validDigest(f.Digest) {\n\t\t\treturn ProblemProcess\n\t\t}\n\t\tr.checkedDigest, r.next = f.Digest, ScenarioEvidenceComplete\n\tcase ScenarioEvidenceComplete:\n\t\tif !r.writerWaited || !r.recoveryWaited || r.adapterCount != ExpectedAdapterEvents(which) {\n\t\t\treturn ProblemIncomplete\n\t\t}\n\t\tr.next = FrameUnknown\n\tdefault:\n\t\treturn ProblemOrder\n\t}\n\treturn ProblemNone\n}\nfunc successfulResult(call rollbackfault.Call) rollbackfault.Result {\n\teffect := rollbackfault.EffectUnknown\n\tswitch call.Point {\n\tcase rollbackfault.DBOriginalRead:\n\t\teffect = rollbackfault.EffectNone\n\tcase rollbackfault.JournalInvalid, rollbackfault.JournalRecord, rollbackfault.JournalValidHeader, rollbackfault.DBPage:\n\t\teffect = rollbackfault.EffectPossible\n\t}\n\treturn rollbackfault.Result{Executed: true, NativeN: call.Length, ReturnedN: call.Length, Effect: effect}\n}\nfunc validSelf(p ProcessEvidence, spec PlanSpec) bool {\n\treturn p.PID > 0 && p.Image == spec.Image && p.ImageHash == spec.Binding.Binary && p.ActualImage && !p.SpawnObserved && !p.WaitObserved && p.State == ProcessRunning && p.ExitCode == 0 && p.Signal == 0 && !p.TimedOut\n}\nfunc validSpawn(p ProcessEvidence, spec PlanSpec) bool {\n\treturn p.PID > 0 && p.Handle != 0 && p.Image == spec.Image && p.ImageHash == spec.Binding.Binary && !p.ActualImage && p.SpawnObserved && !p.WaitObserved && p.State == ProcessRunning && p.ExitCode == 0 && p.Signal == 0 && !p.TimedOut\n}\nfunc validReady(p, spawned ProcessEvidence, spec PlanSpec) bool {\n\treturn validSelf(p, spec) && p.PID == spawned.PID && p.Handle == spawned.Handle\n}\nfunc validWait(p, spawned ProcessEvidence, code int) bool {\n\twant := spawned\n\twant.ActualImage, want.SpawnObserved, want.WaitObserved = false, false, true\n\twant.State, want.ExitCode = ProcessExited, code\n\treturn p == want\n}\nfunc waitStatus(p ProcessEvidence) Diagnostic {\n\tif p.State == ProcessRunning {\n\t\treturn ChildLive\n\t}\n\tif !p.WaitObserved || p.State == ProcessUnknown {\n\t\treturn WaitUnknown\n\t}\n\treturn ChildExitUnexpected\n}\nfunc validAck(a AckEvidence, sequence uint8, raw, body Digest, bodyBytes int64) bool {\n\treturn sequence != 0 && a.CommitSequence == sequence && a.CommitFrameSHA256 == raw && validDigest(raw) && a.SHA256 == body && validDigest(body) && validFile(a.File) && a.Bytes == bodyBytes && a.Bytes > 0 && a.Bytes <= MaxFrameBytes && a.Written == a.Bytes && a.Exclusive && a.WriteStep == 1 && a.FileSyncStep == 2 && a.FileCloseStep == 3 && a.DirectorySyncStep == 4 && a.DirectoryCloseStep == 5 && a.FileCloseCalls == 1 && a.DirectoryCloseCalls == 1 && a.ErrorToken == 0\n}\nfunc (r *Receipt) Seal() Observation {\n\tr.sealed = true\n\to := Observation{Problem: r.problem, Status: r.status, FrameCount: r.count, AdapterEventCount: r.adapterCount, Acknowledged: r.ack != (AckEvidence{}), WriterWaited: r.writerWaited, RecoveryWaited: r.recoveryWaited}\n\tif r.commitSequence != 0 {\n\t\to.Commit = CommitNil\n\t}\n\tif !r.plan.valid {\n\t\to.Problem = ProblemPlan\n\t\treturn o\n\t}\n\tif r.problem != ProblemNone {\n\t\treturn o\n\t}\n\tif r.next != FrameUnknown {\n\t\to.Problem = ProblemIncomplete\n\t\tif r.count == 0 {\n\t\t\to.Status = NotRun\n\t\t}\n\t\treturn o\n\t}\n\to.EvidenceConsistent = true\n\t// SQL and artifact qualification deliberately require Observe as well.\n\treturn o\n}\n\n// SnapshotDigest hashes only a complete, already qualified literal OLD or NEW\n// SQL snapshot. The framing digest is separate from the database-byte digest.\nfunc SnapshotDigest(s rollbackfault.SQLSnapshotEvidence) (Digest, Problem) {\n\tif rollbackfault.ValidateSQLSnapshot(s, s.Identity, rollbackfault.SQLOld) != rollbackfault.SQLProblemNone && rollbackfault.ValidateSQLSnapshot(s, s.Identity, rollbackfault.SQLNew) != rollbackfault.SQLProblemNone {\n\t\treturn \"\", ProblemSQL\n\t}\n\traw, err := json.Marshal(s)\n\tif err != nil || len(raw) > MaxSnapshotBytes {\n\t\treturn \"\", ProblemEncoding\n\t}\n\treturn digestBytes(raw), ProblemNone\n}\nfunc ArtifactName(role ArtifactRole) string {\n\tswitch role {\n\tcase BaselineArtifact:\n\t\treturn \"inputs/baseline.input.db\"\n\tcase TrialArtifact:\n\t\treturn \"trial/trial.db\"\n\tcase TrialJournalArtifact:\n\t\treturn \"trial/trial.db-journal\"\n\tcase PostExitArtifact:\n\t\treturn \"evidence/post-exit.input.db\"\n\tcase PostExitJournalArtifact:\n\t\treturn \"evidence/post-exit-journal.input.bin\"\n\tcase ReplayArtifact:\n\t\treturn \"replay/replay.db\"\n\tcase ReplayJournalArtifact:\n\t\treturn \"replay/replay.db-journal\"\n\tcase AcknowledgementArtifact:\n\t\treturn \"control/ack.json\"\n\tdefault:\n\t\treturn \"\"\n\t}\n}\nfunc ArtifactManifestDigest(artifacts [ArtifactCount]ArtifactEvidence, after bool) (Digest, Problem) {\n\tvar stamps [ArtifactCount]ArtifactStamp\n\tfor i, a := range artifacts {\n\t\tif a.Role != ArtifactRole(i) {\n\t\t\treturn \"\", ProblemArtifacts\n\t\t}\n\t\ts := a.Before\n\t\tif after {\n\t\t\ts = a.After\n\t\t}\n\t\tif s != (ArtifactStamp{}) && !validStamp(s, ArtifactRole(i)) {\n\t\t\treturn \"\", ProblemArtifacts\n\t\t}\n\t\tstamps[i] = s\n\t}\n\traw, err := json.Marshal(stamps)\n\tif err != nil || len(raw) > MaxArtifactManifestBytes {\n\t\treturn \"\", ProblemEncoding\n\t}\n\treturn digestBytes(raw), ProblemNone\n}\nfunc validStamp(s ArtifactStamp, role ArtifactRole) bool {\n\tif s.Path != ArtifactName(role) || !s.Exists || !s.Regular || s.Links != 1 || !validFile(s.File) || !validDigest(s.SHA256) {\n\t\treturn false\n\t}\n\tif role == AcknowledgementArtifact {\n\t\treturn s.Bytes > 0 && s.Bytes <= MaxFrameBytes && s.Pages == ([3]Digest{})\n\t}\n\tif role == TrialJournalArtifact || role == PostExitJournalArtifact || role == ReplayJournalArtifact {\n\t\treturn s.Bytes == 16408 && s.Pages == ([3]Digest{})\n\t}\n\tif s.Bytes != 12288 {\n\t\treturn false\n\t}\n\tfor _, page := range s.Pages {\n\t\tif !validDigest(page) {\n\t\t\treturn false\n\t\t}\n\t}\n\treturn true\n}\nfunc sameContent(a, b ArtifactStamp) bool {\n\treturn a.Exists && b.Exists && a.Bytes == b.Bytes && a.SHA256 == b.SHA256 && a.Pages == b.Pages\n}\nfunc copiedArtifact(a ArtifactEvidence) bool {\n\treturn a.ExclusiveCopy && a.CompleteRead && a.CompleteWrite && a.FileSynced && a.FileClosedOnce && a.DirectorySynced && a.DirectoryClosedOnce\n}\nfunc hasCopyClaims(a ArtifactEvidence) bool {\n\treturn a.ExclusiveCopy || a.CompleteRead || a.CompleteWrite || a.FileSynced || a.FileClosedOnce || a.DirectorySynced || a.DirectoryClosedOnce\n}\nfunc validateArtifacts(p Plan, r Receipt, e ScenarioEvidence) bool {\n\twhich := p.spec.Binding.Case\n\tartifacts := e.Artifacts\n\tfor i, a := range artifacts {\n\t\trole := ArtifactRole(i)\n\t\tif a.Role != role {\n\t\t\treturn false\n\t\t}\n\t\tjournal := role == TrialJournalArtifact || role == PostExitJournalArtifact || role == ReplayJournalArtifact\n\t\tpresent := !journal || which == AfterTablePage\n\t\tif role == AcknowledgementArtifact {\n\t\t\tpresent = which == AcknowledgedExit\n\t\t}\n\t\tif !present {\n\t\t\tif a != (ArtifactEvidence{Role: role}) {\n\t\t\t\treturn false\n\t\t\t}\n\t\t\tcontinue\n\t\t}\n\t\tif !validStamp(a.Before, role) || a.Before.File == p.spec.Binding.Root || a.Before.File == p.spec.Image {\n\t\t\treturn false\n\t\t}\n\t\tif role == ReplayJournalArtifact {\n\t\t\tif a.After != (ArtifactStamp{}) {\n\t\t\t\treturn false\n\t\t\t}\n\t\t} else {\n\t\t\tif !validStamp(a.After, role) || a.Before.File != a.After.File {\n\t\t\t\treturn false\n\t\t\t}\n\t\t\tif role != ReplayArtifact && a.Before != a.After {\n\t\t\t\treturn false\n\t\t\t}\n\t\t\tif role == ReplayArtifact && which != AfterTablePage && a.Before != a.After {\n\t\t\t\treturn false\n\t\t\t}\n\t\t}\n\t\tcopyRole := role == PostExitArtifact || role == PostExitJournalArtifact || role == ReplayArtifact || role == ReplayJournalArtifact\n\t\tif copyRole {\n\t\t\tif !copiedArtifact(a) {\n\t\t\t\treturn false\n\t\t\t}\n\t\t} else if hasCopyClaims(a) {\n\t\t\treturn false\n\t\t}\n\t\tfor j := 0; j < i; j++ {\n\t\t\tother := artifacts[j]\n\t\t\tif other.Before.Exists && other.Before.File == a.Before.File {\n\t\t\t\treturn false\n\t\t\t}\n\t\t\tif other.After.Exists && a.After.Exists && other.After.File == a.After.File {\n\t\t\t\treturn false\n\t\t\t}\n\t\t}\n\t}\n\tbaseline, trial := artifacts[BaselineArtifact].Before, artifacts[TrialArtifact].Before\n\tif baseline.SHA256 != p.spec.Binding.Baseline || !sameContent(trial, artifacts[PostExitArtifact].Before) || !sameContent(trial, artifacts[ReplayArtifact].Before) {\n\t\treturn false\n\t}\n\tif which == BeforeJCreate {\n\t\tif !sameContent(trial, baseline) {\n\t\t\treturn false\n\t\t}\n\t} else {\n\t\tif trial.SHA256 == baseline.SHA256 || trial.Pages[0] == baseline.Pages[0] || trial.Pages[1] == baseline.Pages[1] {\n\t\t\treturn false\n\t\t}\n\t\tif which == AfterTablePage {\n\t\t\tif trial.Pages[2] != baseline.Pages[2] {\n\t\t\t\treturn false\n\t\t\t}\n\t\t} else if trial.Pages[2] == baseline.Pages[2] {\n\t\t\treturn false\n\t\t}\n\t}\n\tif which == AfterTablePage {\n\t\tif !sameContent(artifacts[ReplayArtifact].After, baseline) || !sameContent(artifacts[TrialJournalArtifact].Before, artifacts[PostExitJournalArtifact].Before) || !sameContent(artifacts[TrialJournalArtifact].Before, artifacts[ReplayJournalArtifact].Before) {\n\t\t\treturn false\n\t\t}\n\t}\n\tif which == AcknowledgedExit {\n\t\ta := artifacts[AcknowledgementArtifact].Before\n\t\tif a.File != r.ack.File || a.SHA256 != r.ack.SHA256 || a.Bytes != r.ack.Bytes {\n\t\t\treturn false\n\t\t}\n\t}\n\tbefore, problem := ArtifactManifestDigest(artifacts, false)\n\tif problem != ProblemNone || before != r.frozenDigest {\n\t\treturn false\n\t}\n\tafter, problem := ArtifactManifestDigest(artifacts, true)\n\treturn problem == ProblemNone && after == r.checkedDigest\n}\nfunc validLifecycle(which Case, got WriterLifecycle) bool {\n\twant := WriterLifecycle{OpenCalls: 1, BeginCalls: 1, DMLCalls: 3, ArmCalls: 1, CommitCalls: 1, Main: AbandonedByExpectedProcessExit, Journal: AbandonedByExpectedProcessExit, Cleanup: AbandonedByExpectedProcessExit}\n\tswitch which {\n\tcase BeforeJCreate:\n\t\twant.Journal = NotAcquired\n\tcase AfterJUnlink:\n\t\twant.Journal = RetiredNormally\n\tcase Control, AcknowledgedExit:\n\t\twant.PublicReturned, want.Main, want.Journal, want.Cleanup = true, RetiredNormally, RetiredNormally, RetiredNormally\n\t\tif which == Control {\n\t\t\twant.FinishCalls, want.DisarmCalls, want.DBCloseCalls = 1, 1, 1\n\t\t}\n\t}\n\treturn got == want\n}\nfunc validRecovery(p Plan, got RecoveryEvidence) bool {\n\twant := RecoveryEvidence{Generation: p.spec.Recovery.Generation, OwnersRetired: true, ReadOnly: true, NoCreate: true, OpenCalls: 1, CheckMaxPages: 3, CheckMaxPageReads: 512, NoRetry: true}\n\tif p.spec.Binding.Case == AfterTablePage {\n\t\twant.Calls = 1\n\t\twant.Limits = rollbackfault.SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}\n\t\twant.JournalPresentBefore, want.Replayed, want.ReplayedPages, want.OriginalPages = true, true, 3, 3\n\t}\n\treturn got == want\n}\nfunc validRuntime(got RuntimeEvidence) bool {\n\treturn got == (RuntimeEvidence{\n\t\tEnvFirst: true, ExactRoleSelectors: true, FixedAllowlistEnvironment: true,\n\t\tCountOne: true, ParallelOne: true, CPUOne: true, ShuffleOff: true, TimeoutZero: true,\n\t\tPrivateRoot0700: true, NoSymlinkAncestors: true, RootUnchanged: true,\n\t\tTwoDedicatedPipes: true, PipeOwnersRecorded: true, UnusedPipeEndsClosedOnce: true,\n\t\tBoundedDiagnostics: true, NoInheritedDatabaseOwners: true, ParentOwnersClosedBeforeSpawn: true,\n\t\tFreshRegistryEachChild: true, OnlyParentSpawns: true, NoSignalOrKill: true,\n\t\tNoRetries: true, OriginalsNeverSyncedAfterExit: true, ArtifactsRetained: true,\n\t\tAdapterSameTransaction: true, AdapterSameScope: true, AdapterSameOwners: true,\n\t\tObserverOutsidePrivateMutex: true, OriginalTxLockHeldAtCut: true, NoCallbackNativeReplacement: true,\n\t})\n}\n\n// Observe replays the private raw frames, then independently checks generations,\n// exact OLD/NEW SQL snapshots, lifecycle, copy identity and retained artifacts.\n// The old rollbackfault.ObserveSQLLedger terminal model is intentionally unused.\nfunc Observe(p Plan, supplied Receipt, e ScenarioEvidence) Observation {\n\tbad := func(problem Problem, status Diagnostic) Observation {\n\t\treturn Observation{Problem: problem, Status: status}\n\t}\n\tif e.PhysicalClaim {\n\t\treturn bad(ProblemPhysicalClaim, ProtocolInvalid)\n\t}\n\tif !p.valid || !supplied.plan.valid || p.spec != supplied.plan.spec || supplied.count > MaxFrames {\n\t\treturn bad(ProblemPlan, ProtocolInvalid)\n\t}\n\tr, problem := NewReceipt(p)\n\tif problem != ProblemNone {\n\t\treturn bad(problem, ProtocolInvalid)\n\t}\n\tfor i := uint8(0); i < supplied.count; i++ {\n\t\tif r.Append(supplied.frames[i]) != ProblemNone {\n\t\t\tbreak\n\t\t}\n\t}\n\to := r.Seal()\n\t// Rejected identity or sequence frames may not be retained; their sticky\n\t// rejection still must not disappear when accepted-prefix values replay.\n\tif supplied.problem != ProblemNone {\n\t\to.Problem, o.Status, o.EvidenceConsistent = supplied.problem, supplied.status, false\n\t\treturn o\n\t}\n\tif !o.EvidenceConsistent {\n\t\treturn o\n\t}\n\tif e.Binding != p.spec.Binding || e.BeforeGeneration != p.spec.Controller.Generation || e.AfterGeneration != p.spec.Recovery.Generation {\n\t\to.Problem, o.Status = ProblemIdentity, ProtocolInvalid\n\t\to.EvidenceConsistent = false\n\t\treturn o\n\t}\n\tif !validLifecycle(p.spec.Binding.Case, e.Lifecycle) || !validRuntime(e.Runtime) {\n\t\to.Problem, o.Status = ProblemLifecycle, OwnerUnknown\n\t\to.EvidenceConsistent = false\n\t\treturn o\n\t}\n\tif !validRecovery(p, e.Recovery) {\n\t\to.Problem, o.Status = ProblemRecovery, RecoveryFailed\n\t\to.EvidenceConsistent = false\n\t\treturn o\n\t}\n\tif !validateArtifacts(p, r, e) {\n\t\to.Problem, o.Status = ProblemArtifacts, ArtifactChanged\n\t\to.EvidenceConsistent = false\n\t\treturn o\n\t}\n\tstate := ExpectedSQLState(p.spec.Binding.Case)\n\tif rollbackfault.ValidateSQLSnapshot(e.Before, p.spec.AdapterIdentity, rollbackfault.SQLOld) != rollbackfault.SQLProblemNone || rollbackfault.ValidateSQLSnapshot(e.After, p.spec.AdapterIdentity, state) != rollbackfault.SQLProblemNone {\n\t\to.Problem, o.Status = ProblemSQL, ReadbackFailed\n\t\to.EvidenceConsistent = false\n\t\treturn o\n\t}\n\tbefore, problem := SnapshotDigest(e.Before)\n\tif problem != ProblemNone || before != r.baselineDigest {\n\t\to.Problem, o.Status = ProblemSQL, ReadbackFailed\n\t\to.EvidenceConsistent = false\n\t\treturn o\n\t}\n\tafter, problem := SnapshotDigest(e.After)\n\tif problem != ProblemNone || after != r.readbackDigest {\n\t\to.Problem, o.Status = ProblemSQL, ReadbackFailed\n\t\to.EvidenceConsistent = false\n\t\treturn o\n\t}\n\t// Local reader/iterator ordinals may legitimately recur across generations;\n\t// no global-integer inequality is substituted for the fresh-process proof.\n\to.ScenarioObserved, o.SQLState, o.Status = true, state, EvidenceComplete\n\treturn o\n}\n\n// ScenarioRun contains private receipts, not a caller-forgeable Observation.\n// Each native controller invocation still admits exactly one of the five cases.\n// This optional pure aggregator rejects missing/duplicate cases and trial/root\n// reuse instead of claiming that one successful case covers the whole suite.\ntype ScenarioRun struct {\n\tPlan     Plan\n\tReceipt  Receipt\n\tEvidence ScenarioEvidence\n}\ntype SuiteObservation struct {\n\tEvidenceConsistent bool\n\tProblem            Problem\n\tCases              [5]Observation\n}\n\nfunc (SuiteObservation) PhysicalValidated() bool { return false }\nfunc (SuiteObservation) Pass() bool              { return false }\nfunc ObserveSuite(runs [5]ScenarioRun) SuiteObservation {\n\tout := SuiteObservation{}\n\tcases := Cases()\n\tfor i, run := range runs {\n\t\tif !run.Plan.valid || run.Plan.spec.Binding.Case != cases[i] {\n\t\t\tout.Problem = ProblemPlan\n\t\t\treturn out\n\t\t}\n\t\tb := run.Plan.spec.Binding\n\t\tfor j := 0; j < i; j++ {\n\t\t\tprior := runs[j].Plan.spec.Binding\n\t\t\tif b.Trial == prior.Trial || b.Root == prior.Root {\n\t\t\t\tout.Problem = ProblemIdentity\n\t\t\t\treturn out\n\t\t\t}\n\t\t\tcommon, expected := b, prior\n\t\t\tcommon.Trial, common.Root, common.Case = \"\", FileIdentity{}, CaseUnknown\n\t\t\texpected.Trial, expected.Root, expected.Case = \"\", FileIdentity{}, CaseUnknown\n\t\t\tif common != expected || run.Plan.spec.Image != runs[j].Plan.spec.Image {\n\t\t\t\tout.Problem = ProblemIdentity\n\t\t\t\treturn out\n\t\t\t}\n\t\t\troles := [3]RoleIdentity{run.Plan.spec.Controller, run.Plan.spec.Writer, run.Plan.spec.Recovery}\n\t\t\tprevious := [3]RoleIdentity{runs[j].Plan.spec.Controller, runs[j].Plan.spec.Writer, runs[j].Plan.spec.Recovery}\n\t\t\tfor _, role := range roles {\n\t\t\t\tfor _, other := range previous {\n\t\t\t\t\tif role.Challenge == other.Challenge {\n\t\t\t\t\t\tout.Problem = ProblemIdentity\n\t\t\t\t\t\treturn out\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}\n\t\t\tfor _, artifact := range run.Evidence.Artifacts {\n\t\t\t\tif !artifact.Before.Exists {\n\t\t\t\t\tcontinue\n\t\t\t\t}\n\t\t\t\tfor _, other := range runs[j].Evidence.Artifacts {\n\t\t\t\t\tif other.Before.Exists && artifact.Before.File == other.Before.File {\n\t\t\t\t\t\tout.Problem = ProblemArtifacts\n\t\t\t\t\t\treturn out\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t\tout.Cases[i] = Observe(run.Plan, run.Receipt, run.Evidence)\n\t\tif !out.Cases[i].ScenarioObserved || !out.Cases[i].EvidenceConsistent || out.Cases[i].Problem != ProblemNone {\n\t\t\tout.Problem = out.Cases[i].Problem\n\t\t\tif out.Problem == ProblemNone {\n\t\t\t\tout.Problem = ProblemIncomplete\n\t\t\t}\n\t\t\treturn out\n\t\t}\n\t}\n\tout.EvidenceConsistent = true\n\treturn out\n}\n",
	rollbackRestartModelTest: "package rollbackrestart\n\nimport (\n\t\"bytes\"\n\t\"encoding/binary\"\n\t\"strings\"\n\t\"testing\"\n\n\t\"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault\"\n)\n\n// SOURCE ONLY. These descriptor-free synthetic values make no SQL, fixture,\n// process, native owner, journal, recovery, or physical durability observation.\nfunc testDigest(label string) Digest { return digestBytes([]byte(\"synthetic:\" + label)) }\nfunc testPlan(t *testing.T, which Case) Plan {\n\tt.Helper()\n\tb := Binding{Suite: Suite, Profile: rollbackfault.PlainRollback4096, Case: which, Trial: \"trial-\" + CaseName(which), Source: \"frozen-source\", Manifest: testDigest(\"manifest\"), Tool: testDigest(\"tool\"), Compile: testDigest(\"compile\"), Binary: testDigest(\"binary\"), Native: NativeIdentity{OS: \"linux\", Filesystem: \"qualified-filesystem\", SectorSize: 4096, PageSize: 4096}, Root: FileIdentity{Device: 1, Inode: 100 + uint64(which)}, Baseline: testDigest(\"old-database\")}\n\tid := rollbackfault.Identity{Suite: b.Suite, Source: b.Source, Tool: string(b.Tool), Binary: string(b.Binary), Manifest: string(b.Manifest), Trial: b.Trial, Scope: \"writer-scope\", Profile: b.Profile}\n\tspec := PlanSpec{Binding: b, Controller: RoleIdentity{Role: Controller, Generation: 1, Challenge: testDigest(b.Trial + \"-controller\")}, Writer: RoleIdentity{Role: Writer, Generation: 2, Challenge: testDigest(b.Trial + \"-writer\")}, Recovery: RoleIdentity{Role: Recovery, Generation: 3, Challenge: testDigest(b.Trial + \"-recovery\")}, AdapterIdentity: id, Ticket: 1, Image: FileIdentity{Device: 1, Inode: 2}}\n\tplan, problem := NewPlan(spec)\n\tif problem != ProblemNone {\n\t\tt.Fatalf(\"synthetic plan rejected: %v\", problem)\n\t}\n\treturn plan\n}\nfunc literalSnapshot(id rollbackfault.Identity, state rollbackfault.SQLState) rollbackfault.SQLSnapshotEvidence {\n\trows := [3]rollbackfault.SQLRow{\n\t\t{IDType: rollbackfault.SQLInteger, ID: 1, KeyType: rollbackfault.SQLText, Key: \"red\", ValueType: rollbackfault.SQLText, Value: \"one\"},\n\t\t{IDType: rollbackfault.SQLInteger, ID: 2, KeyType: rollbackfault.SQLText, Key: \"blue\", ValueType: rollbackfault.SQLText, Value: \"two\"},\n\t\t{IDType: rollbackfault.SQLInteger, ID: 3, KeyType: rollbackfault.SQLText, Key: \"red\", ValueType: rollbackfault.SQLText, Value: \"three\"},\n\t}\n\tif state == rollbackfault.SQLNew {\n\t\trows = [3]rollbackfault.SQLRow{\n\t\t\t{IDType: rollbackfault.SQLInteger, ID: 1, KeyType: rollbackfault.SQLText, Key: \"green\", ValueType: rollbackfault.SQLText, Value: \"ONE\"},\n\t\t\t{IDType: rollbackfault.SQLInteger, ID: 3, KeyType: rollbackfault.SQLText, Key: \"red\", ValueType: rollbackfault.SQLText, Value: \"three\"},\n\t\t\t{IDType: rollbackfault.SQLInteger, ID: 4, KeyType: rollbackfault.SQLText, Key: \"blue\", ValueType: rollbackfault.SQLText, Value: \"four\"},\n\t\t}\n\t}\n\ttexts := [7]string{\n\t\t\"SELECT id,k,v FROM t NOT INDEXED ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t WHERE k='blue' ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t NOT INDEXED WHERE k='blue' ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t WHERE k='red' ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t NOT INDEXED WHERE k='red' ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t WHERE k='green' ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t NOT INDEXED WHERE k='green' ORDER BY id\",\n\t}\n\tkeys := [7]string{\"\", \"blue\", \"blue\", \"red\", \"red\", \"green\", \"green\"}\n\tsnapshot := rollbackfault.SQLSnapshotEvidence{Identity: id, Owner: 11, CloseCalls: 1, OwnerState: rollbackfault.OwnerRetired, Check: rollbackfault.SQLCheckEvidence{Identity: id, Owner: 11, Calls: 1, Pages: 3, Tables: 1, Indexes: 1, TableRows: 3, IndexEntries: 3}}\n\tfor i := 0; i < 7; i++ {\n\t\tq := rollbackfault.SQLQueryEvidence{Identity: id, Query: rollbackfault.SQLQuery(i), Owner: 11, Iterator: uint64(i + 1), SQL: texts[i], EOF: true, ErrCalled: true, CloseCalls: 1, Plan: rollbackfault.SQLQueryPlan{Access: \"SCAN\", Table: \"t\", Reason: \"NOT INDEXED requested\"}, Work: rollbackfault.SQLQueryWork{BaseRowFetches: 3}}\n\t\tfor _, row := range rows {\n\t\t\tif i == 0 || row.Key == keys[i] {\n\t\t\t\tq.Rows[q.RowCount] = row\n\t\t\t\tq.RowCount++\n\t\t\t}\n\t\t}\n\t\tn := int64(q.RowCount)\n\t\tq.Work.ReturnedRows = n\n\t\tif i == 1 || i == 3 || i == 5 {\n\t\t\tq.Plan = rollbackfault.SQLQueryPlan{Access: \"SEARCH\", Table: \"t\", Method: \"btree\", Index: \"by_k\", IndexCount: 1, Field: \"k\", FieldCount: 1, Recheck: true}\n\t\t\tq.Work = rollbackfault.SQLQueryWork{IndexKeysRead: n, IndexPageReads: 1, RowIDSeeks: n, Candidates: n, BaseRowFetches: n, ReturnedRows: n}\n\t\t}\n\t\tsnapshot.Queries[i] = q\n\t}\n\treturn snapshot\n}\nfunc testEvidence(p Plan) ScenarioEvidence {\n\ts := p.Spec()\n\tstate := rollbackfault.SQLNew\n\tif s.Binding.Case == BeforeJCreate || s.Binding.Case == AfterTablePage {\n\t\tstate = rollbackfault.SQLOld\n\t}\n\te := ScenarioEvidence{\n\t\tBinding: s.Binding, Before: literalSnapshot(s.AdapterIdentity, rollbackfault.SQLOld), After: literalSnapshot(s.AdapterIdentity, state), BeforeGeneration: s.Controller.Generation, AfterGeneration: s.Recovery.Generation,\n\t\tLifecycle: WriterLifecycle{OpenCalls: 1, BeginCalls: 1, DMLCalls: 3, ArmCalls: 1, CommitCalls: 1, Main: AbandonedByExpectedProcessExit, Journal: AbandonedByExpectedProcessExit, Cleanup: AbandonedByExpectedProcessExit},\n\t\tRecovery:  RecoveryEvidence{Generation: s.Recovery.Generation, OwnersRetired: true, ReadOnly: true, NoCreate: true, OpenCalls: 1, CheckMaxPages: 3, CheckMaxPageReads: 512, NoRetry: true},\n\t\tRuntime:   RuntimeEvidence{EnvFirst: true, ExactRoleSelectors: true, FixedAllowlistEnvironment: true, CountOne: true, ParallelOne: true, CPUOne: true, ShuffleOff: true, TimeoutZero: true, PrivateRoot0700: true, NoSymlinkAncestors: true, RootUnchanged: true, TwoDedicatedPipes: true, PipeOwnersRecorded: true, UnusedPipeEndsClosedOnce: true, BoundedDiagnostics: true, NoInheritedDatabaseOwners: true, ParentOwnersClosedBeforeSpawn: true, FreshRegistryEachChild: true, OnlyParentSpawns: true, NoSignalOrKill: true, NoRetries: true, OriginalsNeverSyncedAfterExit: true, ArtifactsRetained: true, AdapterSameTransaction: true, AdapterSameScope: true, AdapterSameOwners: true, ObserverOutsidePrivateMutex: true, OriginalTxLockHeldAtCut: true, NoCallbackNativeReplacement: true},\n\t}\n\tswitch s.Binding.Case {\n\tcase BeforeJCreate:\n\t\te.Lifecycle.Journal = NotAcquired\n\tcase AfterJUnlink:\n\t\te.Lifecycle.Journal = RetiredNormally\n\tcase Control, AcknowledgedExit:\n\t\te.Lifecycle.PublicReturned, e.Lifecycle.Main, e.Lifecycle.Journal, e.Lifecycle.Cleanup = true, RetiredNormally, RetiredNormally, RetiredNormally\n\t\tif s.Binding.Case == Control {\n\t\t\te.Lifecycle.FinishCalls, e.Lifecycle.DisarmCalls, e.Lifecycle.DBCloseCalls = 1, 1, 1\n\t\t}\n\t}\n\tif s.Binding.Case == AfterTablePage {\n\t\te.Recovery.Calls, e.Recovery.JournalPresentBefore, e.Recovery.Replayed, e.Recovery.ReplayedPages, e.Recovery.OriginalPages = 1, true, true, 3, 3\n\t\te.Recovery.Limits = rollbackfault.SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}\n\t}\n\toldPages := [3]Digest{testDigest(\"old-schema\"), testDigest(\"old-table\"), testDigest(\"old-index\")}\n\tnewPages := [3]Digest{testDigest(\"new-schema\"), testDigest(\"new-table\"), testDigest(\"new-index\")}\n\ttrialHash, trialPages := testDigest(\"new-database\"), newPages\n\tif s.Binding.Case == BeforeJCreate {\n\t\ttrialHash, trialPages = s.Binding.Baseline, oldPages\n\t}\n\tif s.Binding.Case == AfterTablePage {\n\t\ttrialHash, trialPages[2] = testDigest(\"mixed-database\"), oldPages[2]\n\t}\n\tfor i := range e.Artifacts {\n\t\trole := ArtifactRole(i)\n\t\ta := ArtifactEvidence{Role: role}\n\t\tjournal := role == TrialJournalArtifact || role == PostExitJournalArtifact || role == ReplayJournalArtifact\n\t\tif role == AcknowledgementArtifact || (journal && s.Binding.Case != AfterTablePage) {\n\t\t\te.Artifacts[i] = a\n\t\t\tcontinue\n\t\t}\n\t\tstamp := ArtifactStamp{Path: ArtifactName(role), File: FileIdentity{Device: 1, Inode: 1000 + 100*uint64(s.Binding.Case) + uint64(i)}, Exists: true, Regular: true, Links: 1, Bytes: 12288, SHA256: trialHash, Pages: trialPages}\n\t\tif role == BaselineArtifact {\n\t\t\tstamp.SHA256, stamp.Pages = s.Binding.Baseline, oldPages\n\t\t}\n\t\tif journal {\n\t\t\tstamp.Bytes, stamp.SHA256, stamp.Pages = 16408, testDigest(\"complete-hot-journal\"), [3]Digest{}\n\t\t}\n\t\ta.Before, a.After = stamp, stamp\n\t\tif role == ReplayArtifact && s.Binding.Case == AfterTablePage {\n\t\t\ta.After.SHA256, a.After.Pages = s.Binding.Baseline, oldPages\n\t\t}\n\t\tif role == ReplayJournalArtifact {\n\t\t\ta.After = ArtifactStamp{}\n\t\t}\n\t\tif role == PostExitArtifact || role == PostExitJournalArtifact || role == ReplayArtifact || role == ReplayJournalArtifact {\n\t\t\ta.ExclusiveCopy, a.CompleteRead, a.CompleteWrite, a.FileSynced, a.FileClosedOnce, a.DirectorySynced, a.DirectoryClosedOnce = true, true, true, true, true, true, true\n\t\t}\n\t\te.Artifacts[i] = a\n\t}\n\treturn e\n}\nfunc literalCalls() [18]rollbackfault.Call {\n\treturn [18]rollbackfault.Call{\n\t\t{Point: rollbackfault.JournalCreate, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},\n\t\t{Point: rollbackfault.JournalInvalid, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12, Length: 4096},\n\t\t{Point: rollbackfault.DBOriginalRead, Occurrence: 1, Role: rollbackfault.Database, Owner: 11, Length: 4096},\n\t\t{Point: rollbackfault.JournalRecord, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12, Offset: 4096, Length: 4104},\n\t\t{Point: rollbackfault.DBOriginalRead, Occurrence: 2, Role: rollbackfault.Database, Owner: 11, Offset: 4096, Length: 4096},\n\t\t{Point: rollbackfault.JournalRecord, Occurrence: 2, Role: rollbackfault.Journal, Owner: 12, Offset: 8200, Length: 4104},\n\t\t{Point: rollbackfault.DBOriginalRead, Occurrence: 3, Role: rollbackfault.Database, Owner: 11, Offset: 8192, Length: 4096},\n\t\t{Point: rollbackfault.JournalRecord, Occurrence: 3, Role: rollbackfault.Journal, Owner: 12, Offset: 12304, Length: 4104},\n\t\t{Point: rollbackfault.JournalBodySync, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},\n\t\t{Point: rollbackfault.JournalValidHeader, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12, Length: 12},\n\t\t{Point: rollbackfault.JournalHeaderSync, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},\n\t\t{Point: rollbackfault.PublishDirectorySync, Occurrence: 1, Role: rollbackfault.Directory, Owner: 13},\n\t\t{Point: rollbackfault.DBPage, Occurrence: 1, Role: rollbackfault.Database, Owner: 11, Length: 4096},\n\t\t{Point: rollbackfault.DBPage, Occurrence: 2, Role: rollbackfault.Database, Owner: 11, Offset: 4096, Length: 4096},\n\t\t{Point: rollbackfault.DBPage, Occurrence: 3, Role: rollbackfault.Database, Owner: 11, Offset: 8192, Length: 4096},\n\t\t{Point: rollbackfault.DBSync, Occurrence: 1, Role: rollbackfault.Database, Owner: 11},\n\t\t{Point: rollbackfault.JournalUnlink, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},\n\t\t{Point: rollbackfault.FinalDirectorySync, Occurrence: 1, Role: rollbackfault.Directory, Owner: 13},\n\t}\n}\nfunc mustSnapshotDigest(t *testing.T, s rollbackfault.SQLSnapshotEvidence) Digest {\n\tt.Helper()\n\td, p := SnapshotDigest(s)\n\tif p != ProblemNone {\n\t\tt.Fatalf(\"snapshot: %v\", p)\n\t}\n\treturn d\n}\nfunc mustManifestDigest(t *testing.T, e ScenarioEvidence, after bool) Digest {\n\tt.Helper()\n\td, p := ArtifactManifestDigest(e.Artifacts, after)\n\tif p != ProblemNone {\n\t\tt.Fatalf(\"manifest: %v\", p)\n\t}\n\treturn d\n}\nfunc mustFrameDigest(t *testing.T, f Frame) Digest {\n\tt.Helper()\n\td, p := FrameDigest(f)\n\tif p != ProblemNone {\n\t\tt.Fatalf(\"frame: %v\", p)\n\t}\n\treturn d\n}\nfunc testFrames(t *testing.T, p Plan, e *ScenarioEvidence) []Frame {\n\tt.Helper()\n\ts := p.Spec()\n\tvar frames []Frame\n\tadd := func(kind FrameKind, role RoleIdentity) *Frame {\n\t\tframes = append(frames, Frame{Schema: SchemaVersion, Binding: s.Binding, Sender: role, Sequence: uint8(len(frames) + 1), Kind: kind})\n\t\treturn &frames[len(frames)-1]\n\t}\n\tparent := ProcessEvidence{PID: 1000, Image: s.Image, ImageHash: s.Binding.Binary, ActualImage: true, State: ProcessRunning}\n\twriter := ProcessEvidence{PID: 1001, Handle: 1, Image: s.Image, ImageHash: s.Binding.Binary, SpawnObserved: true, State: ProcessRunning}\n\trecovery := ProcessEvidence{PID: 1002, Handle: 2, Image: s.Image, ImageHash: s.Binding.Binary, SpawnObserved: true, State: ProcessRunning}\n\tadd(ParentAdmitted, s.Controller).Process = parent\n\tadd(BaselineCheckedAndClosed, s.Controller).Digest = mustSnapshotDigest(t, e.Before)\n\tadd(WriterSpawned, s.Controller).Process = writer\n\tready := writer\n\tready.SpawnObserved, ready.ActualImage = false, true\n\tadd(ChildReadyBound, s.Writer).Process = ready\n\tadd(ArmGranted, s.Controller)\n\tarmed := add(TransactionArmed, s.Writer)\n\tarmed.Ticket, armed.Adapter = s.Ticket, rollbackfault.Event{Identity: s.AdapterIdentity, Ordinal: 1, Kind: rollbackfault.RunStarted}\n\tordinal := uint8(1)\n\tvar last rollbackfault.Event\n\ttargetCall, targetKind := 0, rollbackfault.EventUnknown\n\tswitch s.Binding.Case {\n\tcase BeforeJCreate:\n\t\ttargetCall, targetKind = 1, rollbackfault.Intent\n\tcase AfterTablePage:\n\t\ttargetCall, targetKind = 14, rollbackfault.Returned\n\tcase AfterJUnlink:\n\t\ttargetCall, targetKind = 17, rollbackfault.Returned\n\t}\n\tstopped := false\n\tfor i, call := range literalCalls() {\n\t\tfor _, kind := range [2]rollbackfault.EventKind{rollbackfault.Intent, rollbackfault.Returned} {\n\t\t\tordinal++\n\t\t\tevent := rollbackfault.Event{Identity: s.AdapterIdentity, Ordinal: ordinal, Kind: kind, Call: call}\n\t\t\tif kind == rollbackfault.Returned {\n\t\t\t\teffect := rollbackfault.EffectUnknown\n\t\t\t\tif call.Point == rollbackfault.DBOriginalRead {\n\t\t\t\t\teffect = rollbackfault.EffectNone\n\t\t\t\t}\n\t\t\t\tif call.Point == rollbackfault.JournalInvalid || call.Point == rollbackfault.JournalRecord || call.Point == rollbackfault.JournalValidHeader || call.Point == rollbackfault.DBPage {\n\t\t\t\t\teffect = rollbackfault.EffectPossible\n\t\t\t\t}\n\t\t\t\tevent.Result = rollbackfault.Result{Executed: true, NativeN: call.Length, ReturnedN: call.Length, Effect: effect}\n\t\t\t}\n\t\t\tf := add(AdapterObserved, s.Writer)\n\t\t\tf.Adapter, f.Ticket = event, s.Ticket\n\t\t\tlast = event\n\t\t\tif i+1 == targetCall && kind == targetKind {\n\t\t\t\tstopped = true\n\t\t\t\tbreak\n\t\t\t}\n\t\t}\n\t\tif stopped {\n\t\t\tbreak\n\t\t}\n\t}\n\tif targetCall != 0 {\n\t\treached := add(TargetReached, s.Writer)\n\t\treached.Adapter, reached.Ticket = last, s.Ticket\n\t\tdigest := mustFrameDigest(t, *reached)\n\t\tadd(ExitGranted, s.Controller).Digest = digest\n\t} else {\n\t\tcommit := add(PublicCommitReturned, s.Writer)\n\t\tcommit.Commit = CommitEvidence{Calls: 1, Value: CommitNil, CapturedAdjacent: true, CapturedUnchanged: true, PublicDone: true, PublicDetached: true, Main: rollbackfault.OwnerRetired, Journal: rollbackfault.OwnerRetired, Cleanup: rollbackfault.OwnerRetired}\n\t\tcommitFrame := *commit\n\t\tcommitHash := mustFrameDigest(t, commitFrame)\n\t\tif s.Binding.Case == Control {\n\t\t\tadd(NormalFinishGranted, s.Controller).Digest = commitHash\n\t\t\tadd(WriterFinished, s.Writer)\n\t\t} else {\n\t\t\traw, problem := EncodeFrame(commitFrame)\n\t\t\tif problem != ProblemNone {\n\t\t\t\tt.Fatal(problem)\n\t\t\t}\n\t\t\tack := AckEvidence{CommitSequence: commitFrame.Sequence, CommitFrameSHA256: commitHash, SHA256: digestBytes(raw[4:]), File: FileIdentity{Device: 1, Inode: 9000}, Bytes: int64(len(raw) - 4), Written: int64(len(raw) - 4), Exclusive: true, WriteStep: 1, FileSyncStep: 2, FileCloseStep: 3, DirectorySyncStep: 4, DirectoryCloseStep: 5, FileCloseCalls: 1, DirectoryCloseCalls: 1}\n\t\t\tadd(AcknowledgementDurable, s.Controller).Ack = ack\n\t\t\texit := add(ExitGranted, s.Controller)\n\t\t\texit.Ack, exit.Digest = ack, ack.SHA256\n\t\t\tstamp := ArtifactStamp{Path: ArtifactName(AcknowledgementArtifact), File: ack.File, Exists: true, Regular: true, Links: 1, Bytes: ack.Bytes, SHA256: ack.SHA256}\n\t\t\te.Artifacts[AcknowledgementArtifact] = ArtifactEvidence{Role: AcknowledgementArtifact, Before: stamp, After: stamp}\n\t\t}\n\t}\n\twaited := writer\n\twaited.SpawnObserved, waited.WaitObserved, waited.State, waited.ExitCode = false, true, ProcessExited, 86\n\tif s.Binding.Case == Control {\n\t\twaited.ExitCode = 0\n\t}\n\tadd(WriterWaitObserved, s.Controller).Process = waited\n\tadd(OriginalsFrozen, s.Controller).Digest = mustManifestDigest(t, *e, false)\n\tadd(RecoverySpawned, s.Controller).Process = recovery\n\tready = recovery\n\tready.SpawnObserved, ready.ActualImage = false, true\n\tadd(RecoveryReadyBound, s.Recovery).Process = ready\n\tadd(RecoveryReadbackGranted, s.Controller)\n\tadd(ReadbackResult, s.Recovery).Digest = mustSnapshotDigest(t, e.After)\n\twaited = recovery\n\twaited.SpawnObserved, waited.WaitObserved, waited.State = false, true, ProcessExited\n\tadd(RecoveryWaitObserved, s.Controller).Process = waited\n\tadd(ArtifactsRechecked, s.Controller).Digest = mustManifestDigest(t, *e, true)\n\tadd(ScenarioEvidenceComplete, s.Controller)\n\treturn frames\n}\nfunc replay(p Plan, frames []Frame) Receipt {\n\tr, _ := NewReceipt(p)\n\tfor _, frame := range frames {\n\t\tif r.Append(frame) != ProblemNone {\n\t\t\tbreak\n\t\t}\n\t}\n\treturn r\n}\nfunc fixture(t *testing.T, which Case) (Plan, Receipt, ScenarioEvidence, []Frame) {\n\tt.Helper()\n\tp := testPlan(t, which)\n\te := testEvidence(p)\n\tframes := testFrames(t, p, &e)\n\treturn p, replay(p, frames), e, frames\n}\nfunc requireRejected(t *testing.T, o Observation) {\n\tt.Helper()\n\tif o.ScenarioObserved || o.EvidenceConsistent || o.Problem == ProblemNone || o.Pass() || o.PhysicalValidated() {\n\t\tt.Fatalf(\"false complete observation: %+v\", o)\n\t}\n}\nfunc frameIndex(frames []Frame, kind FrameKind) int {\n\tfor i, f := range frames {\n\t\tif f.Kind == kind {\n\t\t\treturn i\n\t\t}\n\t}\n\treturn -1\n}\nfunc renumber(frames []Frame) {\n\tfor i := range frames {\n\t\tframes[i].Sequence = uint8(i + 1)\n\t}\n}\n\nfunc TestRestartExactlyFiveCasesAndEighteenCalls(t *testing.T) {\n\twant := [5]string{\"control\", \"before-jcreate\", \"after-table-page\", \"after-junlink\", \"acknowledged-exit\"}\n\tcounts := [5]uint8{37, 2, 29, 35, 37}\n\tfor i, c := range Cases() {\n\t\tif CaseName(c) != want[i] || ExpectedAdapterEvents(c) != counts[i] {\n\t\t\tt.Fatalf(\"case inventory %d\", i)\n\t\t}\n\t\tparsed, ok := ParseCase(want[i])\n\t\tif !ok || parsed != c {\n\t\t\tt.Fatal(\"case roundtrip\")\n\t\t}\n\t}\n\tfor _, name := range []string{\"\", \"CONTROL\", \"after-index-page\", \"after-jcreate\", \"all\", \"control,before-jcreate\", \"acknowledged-exit/again\"} {\n\t\tif _, ok := ParseCase(name); ok {\n\t\t\tt.Fatalf(\"unexpected case %q\", name)\n\t\t}\n\t}\n\tif ExpectedCalls() != literalCalls() {\n\t\tt.Fatal(\"exact 18-call path changed\")\n\t}\n\tif call, phase := Target(BeforeJCreate); call != 1 || phase != rollbackfault.Intent {\n\t\tt.Fatal(\"before target changed\")\n\t}\n\tif call, phase := Target(AfterTablePage); call != 14 || phase != rollbackfault.Returned {\n\t\tt.Fatal(\"table target changed\")\n\t}\n\tif call, phase := Target(AfterJUnlink); call != 17 || phase != rollbackfault.Returned {\n\t\tt.Fatal(\"unlink target changed\")\n\t}\n}\nfunc TestRestartFiveCompleteValueTranscripts(t *testing.T) {\n\tfor _, which := range Cases() {\n\t\tt.Run(CaseName(which), func(t *testing.T) {\n\t\t\tp, receipt, e, frames := fixture(t, which)\n\t\t\tif len(frames) > MaxFrames {\n\t\t\t\tt.Fatal(\"frozen frame budget insufficient\")\n\t\t\t}\n\t\t\tfor _, f := range frames {\n\t\t\t\traw, problem := EncodeFrame(f)\n\t\t\t\tif problem != ProblemNone {\n\t\t\t\t\tt.Fatalf(\"encode %d: %v\", f.Kind, problem)\n\t\t\t\t}\n\t\t\t\tif len(raw)-4 > MaxFrameBytes {\n\t\t\t\t\tt.Fatal(\"frame budget\")\n\t\t\t\t}\n\t\t\t\tdecoded, problem := DecodeFrame(raw)\n\t\t\t\tif problem != ProblemNone || decoded != f {\n\t\t\t\t\tt.Fatalf(\"roundtrip %d: %v\", f.Kind, problem)\n\t\t\t\t}\n\t\t\t}\n\t\t\to := Observe(p, receipt, e)\n\t\t\tif !o.EvidenceConsistent || !o.ScenarioObserved || o.Status != EvidenceComplete || o.Problem != ProblemNone || o.SQLState != ExpectedSQLState(which) || !o.WriterWaited || !o.RecoveryWaited || o.Pass() || o.PhysicalValidated() {\n\t\t\t\tt.Fatalf(\"synthetic consistency: %+v\", o)\n\t\t\t}\n\t\t\tif o.Acknowledged != (which == AcknowledgedExit) {\n\t\t\t\tt.Fatal(\"ack inferred from wrong fact\")\n\t\t\t}\n\t\t})\n\t}\n}\nfunc TestRestartNewGenerationCanReuseLocalOrdinalsAndPID(t *testing.T) {\n\tp, _, e, frames := fixture(t, AfterTablePage)\n\tif e.Before.Owner != e.After.Owner || e.Before.Queries[0].Iterator != e.After.Queries[0].Iterator {\n\t\tt.Fatal(\"fixture must repeat local ordinals\")\n\t}\n\twriterPID := frames[frameIndex(frames, WriterSpawned)].Process.PID\n\tfor i := range frames {\n\t\tif frames[i].Kind == RecoverySpawned || frames[i].Kind == RecoveryReadyBound || frames[i].Kind == RecoveryWaitObserved {\n\t\t\tframes[i].Process.PID = writerPID\n\t\t}\n\t}\n\to := Observe(p, replay(p, frames), e)\n\tif !o.ScenarioObserved || o.Problem != ProblemNone {\n\t\tt.Fatalf(\"generation identity replaced by PID/local integer inequality: %+v\", o)\n\t}\n}\nfunc TestRestartPlanIdentityAndClosedValues(t *testing.T) {\n\tp := testPlan(t, Control)\n\tmutants := []struct {\n\t\tname   string\n\t\tmutate func(*PlanSpec)\n\t}{\n\t\t{\"no-case\", func(s *PlanSpec) { s.Binding.Case = CaseUnknown }},\n\t\t{\"sixth-case\", func(s *PlanSpec) { s.Binding.Case = Case(6) }},\n\t\t{\"same-process-generation\", func(s *PlanSpec) { s.Recovery.Generation = s.Writer.Generation }},\n\t\t{\"same-challenge\", func(s *PlanSpec) { s.Recovery.Challenge = s.Writer.Challenge }},\n\t\t{\"zero-challenge\", func(s *PlanSpec) { s.Writer.Challenge = \"\" }},\n\t\t{\"all-zero-challenge\", func(s *PlanSpec) { s.Writer.Challenge = Digest(strings.Repeat(\"0\", 64)) }},\n\t\t{\"foreign-source\", func(s *PlanSpec) { s.Binding.Source = \"other-source\" }},\n\t\t{\"foreign-tool\", func(s *PlanSpec) { s.Binding.Tool = testDigest(\"other-tool\") }},\n\t\t{\"foreign-binary\", func(s *PlanSpec) { s.Binding.Binary = testDigest(\"other-binary\") }},\n\t\t{\"foreign-manifest\", func(s *PlanSpec) { s.Binding.Manifest = testDigest(\"other-manifest\") }},\n\t\t{\"missing-compile-receipt\", func(s *PlanSpec) { s.Binding.Compile = \"\" }},\n\t\t{\"unknown-profile\", func(s *PlanSpec) { s.Binding.Profile = rollbackfault.ProfileUnknown }},\n\t\t{\"wrong-os\", func(s *PlanSpec) { s.Binding.Native.OS = \"darwin\" }},\n\t\t{\"wrong-sector\", func(s *PlanSpec) { s.Binding.Native.SectorSize = 512 }},\n\t\t{\"root-missing\", func(s *PlanSpec) { s.Binding.Root.Inode = 0 }},\n\t\t{\"image-missing\", func(s *PlanSpec) { s.Image.Inode = 0 }},\n\t\t{\"ticket-missing\", func(s *PlanSpec) { s.Ticket = 0 }},\n\t\t{\"scope-unbounded\", func(s *PlanSpec) { s.AdapterIdentity.Scope = strings.Repeat(\"x\", MaxIDLength+1) }},\n\t\t{\"upper-case-hash\", func(s *PlanSpec) { s.Binding.Compile = Digest(strings.Repeat(\"A\", 64)) }},\n\t}\n\tfor _, mutant := range mutants {\n\t\tt.Run(mutant.name, func(t *testing.T) {\n\t\t\ts := p.Spec()\n\t\t\tmutant.mutate(&s)\n\t\t\tif _, problem := NewPlan(s); problem == ProblemNone {\n\t\t\t\tt.Fatal(\"admitted invalid plan\")\n\t\t\t}\n\t\t})\n\t}\n\tif _, problem := NewReceipt(Plan{}); problem == ProblemNone {\n\t\tt.Fatal(\"zero plan admitted\")\n\t}\n}\nfunc lengthPrefix(body []byte) []byte {\n\tout := make([]byte, len(body)+4)\n\tbinary.BigEndian.PutUint32(out[:4], uint32(len(body)))\n\tcopy(out[4:], body)\n\treturn out\n}\nfunc TestRestartCanonicalBoundedFraming(t *testing.T) {\n\t_, _, _, frames := fixture(t, Control)\n\traw, problem := EncodeFrame(frames[0])\n\tif problem != ProblemNone {\n\t\tt.Fatal(problem)\n\t}\n\tbody := raw[4:]\n\tbad := [][]byte{\n\t\tnil, raw[:3], raw[:len(raw)-1], append(append([]byte{}, raw...), 0),\n\t\t{0, 0, 0, 0}, lengthPrefix(bytes.Repeat([]byte{' '}, MaxFrameBytes+1)),\n\t\tlengthPrefix(append([]byte(\" \"), body...)),\n\t\tlengthPrefix(append(append([]byte{}, body...), []byte(\"{}\")...)),\n\t\tlengthPrefix(append([]byte(`{\"Unknown\":true,`), body[1:]...)),\n\t\tlengthPrefix(append([]byte(`{\"Schema\":1,`), body[1:]...)),\n\t\tlengthPrefix(bytes.Replace(body, []byte(`\"Schema\":1`), []byte(`\"Schema\":2`), 1)),\n\t\tlengthPrefix(bytes.Replace(body, []byte(`\"Role\":1`), []byte(`\"Role\":99`), 1)),\n\t}\n\tfor i, value := range bad {\n\t\tif _, problem := DecodeFrame(value); problem == ProblemNone {\n\t\t\tt.Fatalf(\"invalid encoding %d accepted\", i)\n\t\t}\n\t}\n\tp := testPlan(t, Control)\n\ts := p.Spec()\n\ts.Binding.Source, s.AdapterIdentity.Source = strings.Repeat(\"s\", MaxIDLength), strings.Repeat(\"s\", MaxIDLength)\n\ts.Binding.Trial, s.AdapterIdentity.Trial = strings.Repeat(\"t\", MaxIDLength), strings.Repeat(\"t\", MaxIDLength)\n\ts.AdapterIdentity.Scope = strings.Repeat(\"q\", MaxIDLength)\n\tp, problem = NewPlan(s)\n\tif problem != ProblemNone {\n\t\tt.Fatal(problem)\n\t}\n\tcall := literalCalls()[13]\n\tf := Frame{Schema: SchemaVersion, Binding: p.Spec().Binding, Sender: p.Spec().Writer, Sequence: 64, Kind: AdapterObserved, Ticket: 1, Adapter: rollbackfault.Event{Identity: p.Spec().AdapterIdentity, Ordinal: 29, Kind: rollbackfault.Returned, Call: call, Result: rollbackfault.Result{Executed: true, NativeN: 4096, ReturnedN: 4096, Effect: rollbackfault.EffectPossible}}}\n\traw, problem = EncodeFrame(f)\n\tif problem != ProblemNone || len(raw) > MaxFrameBytes+4 {\n\t\tt.Fatalf(\"maximal admitted identity exceeds frozen frame budget: %d %v\", len(raw), problem)\n\t}\n}\nfunc TestRestartRejectsProtocolAndAdapterFalsePositives(t *testing.T) {\n\tmutants := []struct {\n\t\tname   string\n\t\twhich  Case\n\t\tkind   FrameKind\n\t\tmutate func(*Frame)\n\t}{\n\t\t{\"wrong-sequence\", BeforeJCreate, TargetReached, func(f *Frame) { f.Sequence++ }},\n\t\t{\"old-trial\", BeforeJCreate, TargetReached, func(f *Frame) { f.Binding.Trial = \"old-trial\" }},\n\t\t{\"wrong-role\", BeforeJCreate, TargetReached, func(f *Frame) { f.Sender.Role = Recovery }},\n\t\t{\"old-generation\", BeforeJCreate, TargetReached, func(f *Frame) { f.Sender.Generation-- }},\n\t\t{\"wrong-challenge\", BeforeJCreate, TargetReached, func(f *Frame) { f.Sender.Challenge = testDigest(\"wrong\") }},\n\t\t{\"wrong-root\", BeforeJCreate, TargetReached, func(f *Frame) { f.Binding.Root.Inode++ }},\n\t\t{\"wrong-baseline\", BeforeJCreate, TargetReached, func(f *Frame) { f.Binding.Baseline = testDigest(\"wrong\") }},\n\t\t{\"wrong-compile\", BeforeJCreate, TargetReached, func(f *Frame) { f.Binding.Compile = testDigest(\"wrong\") }},\n\t\t{\"wrong-source\", BeforeJCreate, TargetReached, func(f *Frame) { f.Binding.Source = \"other\" }},\n\t\t{\"wrong-binary\", BeforeJCreate, TargetReached, func(f *Frame) { f.Binding.Binary = testDigest(\"wrong\") }},\n\t\t{\"wrong-native\", BeforeJCreate, TargetReached, func(f *Frame) { f.Binding.Native.Filesystem = \"different\" }},\n\t\t{\"same-scope-required\", AfterTablePage, TargetReached, func(f *Frame) { f.Adapter.Identity.Scope = \"other-scope\" }},\n\t\t{\"same-ticket-required\", AfterTablePage, TargetReached, func(f *Frame) { f.Ticket++ }},\n\t\t{\"before-is-not-returned\", BeforeJCreate, AdapterObserved, func(f *Frame) { f.Adapter.Kind = rollbackfault.Returned }},\n\t\t{\"wrong-owner\", BeforeJCreate, AdapterObserved, func(f *Frame) { f.Adapter.Call.Owner++ }},\n\t\t{\"wrong-occurrence\", BeforeJCreate, AdapterObserved, func(f *Frame) { f.Adapter.Call.Occurrence++ }},\n\t\t{\"wrong-offset\", AfterTablePage, TargetReached, func(f *Frame) { f.Adapter.Call.Offset = 8192 }},\n\t\t{\"wrong-length\", AfterTablePage, TargetReached, func(f *Frame) { f.Adapter.Call.Length = 64 }},\n\t\t{\"fake-native-count\", AfterTablePage, TargetReached, func(f *Frame) { f.Adapter.Result.NativeN = 64 }},\n\t\t{\"injected-action\", BeforeJCreate, AdapterObserved, func(f *Frame) { f.Adapter.Action = rollbackfault.FailBefore }},\n\t\t{\"fake-finish\", BeforeJCreate, AdapterObserved, func(f *Frame) { f.Adapter.Kind = rollbackfault.Terminal }},\n\t\t{\"fake-commit-event\", BeforeJCreate, AdapterObserved, func(f *Frame) { f.Adapter.Commit.Class = rollbackfault.CommitSucceeded }},\n\t\t{\"normal-exit-is-not-crash\", AfterJUnlink, WriterWaitObserved, func(f *Frame) { f.Process.ExitCode = 0 }},\n\t\t{\"abort-is-not-target\", AfterJUnlink, WriterWaitObserved, func(f *Frame) { f.Process.ExitCode = AbortExitCode }},\n\t\t{\"signal-is-not-target\", AfterJUnlink, WriterWaitObserved, func(f *Frame) { f.Process.State, f.Process.Signal = ProcessSignaled, 9 }},\n\t\t{\"timeout-is-not-wait\", AfterJUnlink, WriterWaitObserved, func(f *Frame) { f.Process.TimedOut = true }},\n\t\t{\"live-is-not-exit\", AfterJUnlink, WriterWaitObserved, func(f *Frame) { f.Process.State = ProcessRunning }},\n\t\t{\"missing-actual-wait\", AfterJUnlink, WriterWaitObserved, func(f *Frame) { f.Process.WaitObserved = false }},\n\t\t{\"child-chosen-pid\", BeforeJCreate, ChildReadyBound, func(f *Frame) { f.Process.PID++ }},\n\t\t{\"child-claims-spawn\", BeforeJCreate, ChildReadyBound, func(f *Frame) { f.Process.SpawnObserved = true }},\n\t\t{\"argv-is-not-image\", BeforeJCreate, ChildReadyBound, func(f *Frame) { f.Process.ActualImage = false }},\n\t\t{\"foreign-executing-image\", BeforeJCreate, ChildReadyBound, func(f *Frame) { f.Process.Image.Inode++ }},\n\t\t{\"reopen-is-not-new-process\", BeforeJCreate, RecoverySpawned, func(f *Frame) { f.Process.Handle = 1 }},\n\t\t{\"recovery-exit86-not-success\", BeforeJCreate, RecoveryWaitObserved, func(f *Frame) { f.Process.ExitCode = 86 }},\n\t\t{\"wrong-exit-grant\", BeforeJCreate, ExitGranted, func(f *Frame) { f.Digest = testDigest(\"unbound-permission\") }},\n\t}\n\tfor _, mutant := range mutants {\n\t\tt.Run(mutant.name, func(t *testing.T) {\n\t\t\tp, _, e, frames := fixture(t, mutant.which)\n\t\t\tindex := frameIndex(frames, mutant.kind)\n\t\t\tif index < 0 {\n\t\t\t\tt.Fatal(\"missing mutation target\")\n\t\t\t}\n\t\t\tmutant.mutate(&frames[index])\n\t\t\trequireRejected(t, Observe(p, replay(p, frames), e))\n\t\t})\n\t}\n\tfor _, kind := range []FrameKind{TargetReached, ExitGranted, WriterWaitObserved, OriginalsFrozen, RecoveryReadyBound, RecoveryReadbackGranted, ReadbackResult, RecoveryWaitObserved, ScenarioEvidenceComplete} {\n\t\tt.Run(\"missing-causal-step\", func(t *testing.T) {\n\t\t\tp, _, e, frames := fixture(t, AfterTablePage)\n\t\t\ti := frameIndex(frames, kind)\n\t\t\tframes = append(frames[:i], frames[i+1:]...)\n\t\t\trenumber(frames)\n\t\t\trequireRejected(t, Observe(p, replay(p, frames), e))\n\t\t})\n\t}\n\tp, _, e, frames := fixture(t, AfterTablePage)\n\ti := frameIndex(frames, TargetReached)\n\tduplicated := append([]Frame{}, frames[:i+1]...)\n\tduplicated = append(duplicated, frames[i])\n\tduplicated = append(duplicated, frames[i+1:]...)\n\trenumber(duplicated)\n\trequireRejected(t, Observe(p, replay(p, duplicated), e))\n\textra := append([]Frame{}, frames[:i]...)\n\tforbidden := frames[i-1]\n\tforbidden.Adapter.Ordinal++\n\tforbidden.Adapter.Kind = rollbackfault.Intent\n\tforbidden.Adapter.Call = literalCalls()[14]\n\tforbidden.Adapter.Result = rollbackfault.Result{}\n\textra = append(extra, forbidden)\n\textra = append(extra, frames[i:]...)\n\trenumber(extra)\n\trequireRejected(t, Observe(p, replay(p, extra), e))\n}\n\nfunc TestRestartAcknowledgementRequiresExactCausalDurability(t *testing.T) {\n\tmutants := []struct {\n\t\tname   string\n\t\tmutate func(*AckEvidence)\n\t}{\n\t\t{\"wrong-public-sequence\", func(a *AckEvidence) { a.CommitSequence-- }},\n\t\t{\"different-commit-frame\", func(a *AckEvidence) { a.CommitFrameSHA256 = testDigest(\"old-commit\") }},\n\t\t{\"different-ack-content\", func(a *AckEvidence) { a.SHA256 = testDigest(\"old-ack\") }},\n\t\t{\"not-exclusive\", func(a *AckEvidence) { a.Exclusive = false }},\n\t\t{\"short-write\", func(a *AckEvidence) { a.Written-- }},\n\t\t{\"wrong-length\", func(a *AckEvidence) { a.Bytes++; a.Written++ }},\n\t\t{\"missing-file-sync\", func(a *AckEvidence) { a.FileSyncStep = 0 }},\n\t\t{\"missing-file-close\", func(a *AckEvidence) { a.FileCloseCalls = 0 }},\n\t\t{\"double-file-close\", func(a *AckEvidence) { a.FileCloseCalls = 2 }},\n\t\t{\"missing-directory-sync\", func(a *AckEvidence) { a.DirectorySyncStep = 0 }},\n\t\t{\"missing-directory-close\", func(a *AckEvidence) { a.DirectoryCloseCalls = 0 }},\n\t\t{\"directory-before-file\", func(a *AckEvidence) { a.DirectorySyncStep, a.FileSyncStep = 2, 4 }},\n\t\t{\"close-before-sync\", func(a *AckEvidence) { a.FileCloseStep, a.FileSyncStep = 2, 3 }},\n\t\t{\"sync-error\", func(a *AckEvidence) { a.ErrorToken = 1 }},\n\t}\n\tfor _, mutant := range mutants {\n\t\tt.Run(mutant.name, func(t *testing.T) {\n\t\t\tp, _, e, frames := fixture(t, AcknowledgedExit)\n\t\t\tmutant.mutate(&frames[frameIndex(frames, AcknowledgementDurable)].Ack)\n\t\t\trequireRejected(t, Observe(p, replay(p, frames), e))\n\t\t})\n\t}\n\tfor _, value := range []CommitValue{CommitUnknown, CommitError, CommitUncertain} {\n\t\tp, _, e, frames := fixture(t, AcknowledgedExit)\n\t\tframes[frameIndex(frames, PublicCommitReturned)].Commit.Value = value\n\t\trequireRejected(t, Observe(p, replay(p, frames), e))\n\t}\n\tfor _, kind := range []FrameKind{PublicCommitReturned, AcknowledgementDurable} {\n\t\tp, _, e, frames := fixture(t, AcknowledgedExit)\n\t\ti := frameIndex(frames, kind)\n\t\tframes = append(frames[:i], frames[i+1:]...)\n\t\trenumber(frames)\n\t\trequireRejected(t, Observe(p, replay(p, frames), e))\n\t}\n\tp, _, e, frames := fixture(t, AcknowledgedExit)\n\tcommit, ack := frameIndex(frames, PublicCommitReturned), frameIndex(frames, AcknowledgementDurable)\n\tframes[commit], frames[ack] = frames[ack], frames[commit]\n\trenumber(frames)\n\trequireRejected(t, Observe(p, replay(p, frames), e))\n\tp, _, e, frames = fixture(t, AfterJUnlink)\n\t// A missing journal and successful DB Sync cannot introduce an ACK fact.\n\tframes[frameIndex(frames, ExitGranted)].Ack = AckEvidence{SHA256: testDigest(\"made-up\")}\n\trequireRejected(t, Observe(p, replay(p, frames), e))\n}\nfunc TestRestartIncompleteLivenessAndStickyFailure(t *testing.T) {\n\tfor _, which := range Cases() {\n\t\tp, _, e, frames := fixture(t, which)\n\t\tfor n := 0; n < len(frames); n++ {\n\t\t\tr := replay(p, frames[:n])\n\t\t\to := Observe(p, r, e)\n\t\t\trequireRejected(t, o)\n\t\t\tif o.SQLState != rollbackfault.SQLStateUnknown {\n\t\t\t\tt.Fatal(\"partial transcript invented SQL state\")\n\t\t\t}\n\t\t\tif n == 0 && o.Status != NotRun {\n\t\t\t\tt.Fatal(\"empty receipt must be NOT_RUN\")\n\t\t\t}\n\t\t}\n\t}\n\tp, _, e, frames := fixture(t, AcknowledgedExit)\n\tcommit := frameIndex(frames, PublicCommitReturned)\n\tr := replay(p, frames[:commit+1])\n\to := Observe(p, r, e)\n\tif o.Commit != CommitNil || o.Acknowledged || o.WriterWaited || o.SQLState != rollbackfault.SQLStateUnknown {\n\t\tt.Fatalf(\"pipe COMMIT_OK became acknowledgement/readback: %+v\", o)\n\t}\n\tfor _, diagnostic := range []Diagnostic{ChildLive, WaitUnknown, AckFailed, AckUncertain, OwnerUnknown, OwnerPending, TargetNotReached} {\n\t\tr = replay(p, frames[:commit+1])\n\t\tfailure := Frame{Schema: SchemaVersion, Binding: p.Spec().Binding, Sender: p.Spec().Controller, Sequence: uint8(commit + 2), Kind: Failure, Diagnostic: diagnostic}\n\t\tif r.Append(failure) == ProblemNone {\n\t\t\tt.Fatal(\"failure accepted as complete\")\n\t\t}\n\t\tcount := r.count\n\t\tif r.Append(frames[commit+1]) != ProblemIncomplete || r.count != count {\n\t\t\tt.Fatal(\"failure was overwritten by later success\")\n\t\t}\n\t\to = Observe(p, r, e)\n\t\trequireRejected(t, o)\n\t\tif o.Status != diagnostic || o.Acknowledged || o.WriterWaited {\n\t\t\tt.Fatalf(\"lost exact failure or inferred later fact: %+v\", o)\n\t\t}\n\t}\n\tr = replay(p, frames[:commit+1])\n\t_ = r.Seal()\n\tif r.Append(frames[commit+1]) != ProblemSealed {\n\t\tt.Fatal(\"sealed incomplete receipt repaired\")\n\t}\n\tforged := Observation{EvidenceConsistent: true, ScenarioObserved: true, Status: EvidenceComplete, Acknowledged: true, WriterWaited: true, RecoveryWaited: true}\n\tif forged.PhysicalValidated() || forged.Pass() {\n\t\tt.Fatal(\"exported observation acquired physical authority\")\n\t}\n\trequireRejected(t, Observe(p, Receipt{}, e))\n}\nfunc rebindManifests(frames []Frame, e ScenarioEvidence) {\n\tif d, p := ArtifactManifestDigest(e.Artifacts, false); p == ProblemNone {\n\t\tframes[frameIndex(frames, OriginalsFrozen)].Digest = d\n\t}\n\tif d, p := ArtifactManifestDigest(e.Artifacts, true); p == ProblemNone {\n\t\tframes[frameIndex(frames, ArtifactsRechecked)].Digest = d\n\t}\n}\nfunc TestRestartRejectsOwnerRecoveryAndArtifactFalsePositives(t *testing.T) {\n\tmutants := []struct {\n\t\tname   string\n\t\twhich  Case\n\t\tmutate func(*ScenarioEvidence)\n\t}{\n\t\t{\"fake-crash-Finish\", AfterTablePage, func(e *ScenarioEvidence) { e.Lifecycle.FinishCalls = 1 }},\n\t\t{\"fake-crash-Disarm\", AfterTablePage, func(e *ScenarioEvidence) { e.Lifecycle.DisarmCalls = 1 }},\n\t\t{\"fake-crash-Commit-return\", AfterTablePage, func(e *ScenarioEvidence) { e.Lifecycle.PublicReturned = true }},\n\t\t{\"fake-crash-retirement\", AfterTablePage, func(e *ScenarioEvidence) { e.Lifecycle.Main = RetiredNormally }},\n\t\t{\"crash-DB-Close\", AfterJUnlink, func(e *ScenarioEvidence) { e.Lifecycle.DBCloseCalls = 1 }},\n\t\t{\"crash-Rollback\", BeforeJCreate, func(e *ScenarioEvidence) { e.Lifecycle.RollbackCalls = 1 }},\n\t\t{\"retry-Commit\", AfterTablePage, func(e *ScenarioEvidence) { e.Lifecycle.CommitCalls = 2 }},\n\t\t{\"normal-owner-pending\", Control, func(e *ScenarioEvidence) { e.Lifecycle.Main = DispositionUnknown }},\n\t\t{\"ack-DB-Close\", AcknowledgedExit, func(e *ScenarioEvidence) { e.Lifecycle.DBCloseCalls = 1 }},\n\t\t{\"same-process-readback\", AfterTablePage, func(e *ScenarioEvidence) { e.AfterGeneration = 2 }},\n\t\t{\"baseline-not-closed-before-spawn\", Control, func(e *ScenarioEvidence) { e.Runtime.ParentOwnersClosedBeforeSpawn = false }},\n\t\t{\"stdout-PASS-only\", Control, func(e *ScenarioEvidence) { e.Runtime.TwoDedicatedPipes = false }},\n\t\t{\"truncated-diagnostics\", Control, func(e *ScenarioEvidence) { e.Runtime.BoundedDiagnostics = false }},\n\t\t{\"empty-selector\", Control, func(e *ScenarioEvidence) { e.Runtime.ExactRoleSelectors = false }},\n\t\t{\"automatic-kill\", Control, func(e *ScenarioEvidence) { e.Runtime.NoSignalOrKill = false }},\n\t\t{\"same-process-registry\", AfterTablePage, func(e *ScenarioEvidence) { e.Runtime.FreshRegistryEachChild = false }},\n\t\t{\"observer-inside-mutex\", AfterTablePage, func(e *ScenarioEvidence) { e.Runtime.ObserverOutsidePrivateMutex = false }},\n\t\t{\"different-Tx\", AfterTablePage, func(e *ScenarioEvidence) { e.Runtime.AdapterSameTransaction = false }},\n\t\t{\"trial-extra-Sync\", AfterJUnlink, func(e *ScenarioEvidence) { e.Runtime.OriginalsNeverSyncedAfterExit = false }},\n\t\t{\"recovery-twice\", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.Calls = 2 }},\n\t\t{\"no-recovery\", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.Calls = 0 }},\n\t\t{\"control-recovery\", Control, func(e *ScenarioEvidence) { e.Recovery.Calls = 1 }},\n\t\t{\"unlink-recovery\", AfterJUnlink, func(e *ScenarioEvidence) { e.Recovery.Calls = 1 }},\n\t\t{\"larger-replay-budget\", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.Limits.MaxRecords = 4 }},\n\t\t{\"wrong-replayed-extent\", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.OriginalPages = 4 }},\n\t\t{\"writer-recovery-generation\", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.Generation = 2 }},\n\t\t{\"readwrite-reopen\", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.ReadOnly = false }},\n\t\t{\"unbounded-Check\", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.CheckMaxPageReads = 513 }},\n\t\t{\"recovery-retry\", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.NoRetry = false }},\n\t\t{\"recovery-error-masked\", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.Error, e.Recovery.ErrorToken = rollbackfault.Underlying, 1 }},\n\t\t{\"original-overwritten\", AfterTablePage, func(e *ScenarioEvidence) {\n\t\t\te.Artifacts[TrialArtifact].After.SHA256 = e.Artifacts[BaselineArtifact].Before.SHA256\n\t\t}},\n\t\t{\"original-recovered\", AfterTablePage, func(e *ScenarioEvidence) { e.Artifacts[TrialJournalArtifact].After = ArtifactStamp{} }},\n\t\t{\"journal-omitted\", AfterTablePage, func(e *ScenarioEvidence) {\n\t\t\te.Artifacts[PostExitJournalArtifact] = ArtifactEvidence{Role: PostExitJournalArtifact}\n\t\t}},\n\t\t{\"wrong-journal-size\", AfterTablePage, func(e *ScenarioEvidence) { e.Artifacts[TrialJournalArtifact].Before.Bytes-- }},\n\t\t{\"wrong-copy-hash\", AfterTablePage, func(e *ScenarioEvidence) {\n\t\t\te.Artifacts[PostExitArtifact].Before.SHA256 = testDigest(\"wrong\")\n\t\t\te.Artifacts[PostExitArtifact].After = e.Artifacts[PostExitArtifact].Before\n\t\t}},\n\t\t{\"hardlink-copy\", AfterTablePage, func(e *ScenarioEvidence) {\n\t\t\te.Artifacts[ReplayArtifact].Before.Links = 2\n\t\t\te.Artifacts[ReplayArtifact].After.Links = 2\n\t\t}},\n\t\t{\"inode-alias\", AfterTablePage, func(e *ScenarioEvidence) {\n\t\t\te.Artifacts[ReplayArtifact].Before.File = e.Artifacts[TrialArtifact].Before.File\n\t\t\te.Artifacts[ReplayArtifact].After.File = e.Artifacts[TrialArtifact].Before.File\n\t\t}},\n\t\t{\"nonexclusive-copy\", AfterTablePage, func(e *ScenarioEvidence) { e.Artifacts[ReplayArtifact].ExclusiveCopy = false }},\n\t\t{\"copy-short-write\", AfterTablePage, func(e *ScenarioEvidence) { e.Artifacts[ReplayArtifact].CompleteWrite = false }},\n\t\t{\"copy-directory-not-synced\", AfterTablePage, func(e *ScenarioEvidence) { e.Artifacts[ReplayArtifact].DirectorySynced = false }},\n\t\t{\"trial-new-at-before-create\", BeforeJCreate, func(e *ScenarioEvidence) {\n\t\t\tfor _, i := range []ArtifactRole{TrialArtifact, PostExitArtifact, ReplayArtifact} {\n\t\t\t\te.Artifacts[i].Before.SHA256 = testDigest(\"different\")\n\t\t\t\te.Artifacts[i].After = e.Artifacts[i].Before\n\t\t\t}\n\t\t}},\n\t\t{\"index-already-new-at-table-cut\", AfterTablePage, func(e *ScenarioEvidence) {\n\t\t\tfor _, i := range []ArtifactRole{TrialArtifact, PostExitArtifact, ReplayArtifact} {\n\t\t\t\te.Artifacts[i].Before.Pages[2] = testDigest(\"new-index\")\n\t\t\t\tif i != ReplayArtifact {\n\t\t\t\t\te.Artifacts[i].After = e.Artifacts[i].Before\n\t\t\t\t}\n\t\t\t}\n\t\t}},\n\t\t{\"mixed-after-replay\", AfterTablePage, func(e *ScenarioEvidence) { e.Artifacts[ReplayArtifact].After = e.Artifacts[ReplayArtifact].Before }},\n\t\t{\"ack-artifact-replaced\", AcknowledgedExit, func(e *ScenarioEvidence) {\n\t\t\te.Artifacts[AcknowledgementArtifact].Before.SHA256 = testDigest(\"other\")\n\t\t\te.Artifacts[AcknowledgementArtifact].After = e.Artifacts[AcknowledgementArtifact].Before\n\t\t}},\n\t\t{\"wrong-relative-path\", AfterTablePage, func(e *ScenarioEvidence) { e.Artifacts[ReplayArtifact].Before.Path = \"trial/trial.db\" }},\n\t\t{\"symlink-path\", AfterTablePage, func(e *ScenarioEvidence) { e.Runtime.NoSymlinkAncestors = false }},\n\t\t{\"physical-claim\", Control, func(e *ScenarioEvidence) { e.PhysicalClaim = true }},\n\t}\n\tfor _, mutant := range mutants {\n\t\tt.Run(mutant.name, func(t *testing.T) {\n\t\t\tp, _, e, frames := fixture(t, mutant.which)\n\t\t\tmutant.mutate(&e)\n\t\t\trebindManifests(frames, e)\n\t\t\trequireRejected(t, Observe(p, replay(p, frames), e))\n\t\t})\n\t}\n}\nfunc TestRestartExactSQLSnapshotFalsePositives(t *testing.T) {\n\tmutants := []struct {\n\t\tname   string\n\t\tmutate func(*rollbackfault.SQLSnapshotEvidence)\n\t}{\n\t\t{\"same-count-wrong-value\", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[0].Rows[0].Value = \"one\" }},\n\t\t{\"wrong-type\", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[0].Rows[0].IDType = rollbackfault.SQLText }},\n\t\t{\"wrong-order\", func(s *rollbackfault.SQLSnapshotEvidence) {\n\t\t\ts.Queries[0].Rows[0], s.Queries[0].Rows[1] = s.Queries[0].Rows[1], s.Queries[0].Rows[0]\n\t\t}},\n\t\t{\"not-exhausted\", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[0].EOF = false }},\n\t\t{\"Err-not-read\", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[0].ErrCalled = false }},\n\t\t{\"iterator-close-missing\", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[0].CloseCalls = 0 }},\n\t\t{\"owner-close-missing\", func(s *rollbackfault.SQLSnapshotEvidence) { s.CloseCalls = 0 }},\n\t\t{\"owner-pending\", func(s *rollbackfault.SQLSnapshotEvidence) { s.OwnerState = rollbackfault.OwnerPending }},\n\t\t{\"index-is-scan\", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[1].Plan.Access = \"SCAN\" }},\n\t\t{\"wrong-index\", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[1].Plan.Index = \"other\" }},\n\t\t{\"wrong-index-field\", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[1].Plan.Field = \"v\" }},\n\t\t{\"no-index-page\", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[1].Work.IndexPageReads = 0 }},\n\t\t{\"wrong-base-fetches\", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[1].Work.BaseRowFetches = 0 }},\n\t\t{\"Check-ignored\", func(s *rollbackfault.SQLSnapshotEvidence) { s.Check.IndexEntries = 2 }},\n\t\t{\"duplicate-iterator\", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[1].Iterator = s.Queries[0].Iterator }},\n\t}\n\tfor _, mutant := range mutants {\n\t\tt.Run(mutant.name, func(t *testing.T) {\n\t\t\tp, receipt, e, _ := fixture(t, AcknowledgedExit)\n\t\t\tmutant.mutate(&e.After)\n\t\t\trequireRejected(t, Observe(p, receipt, e))\n\t\t})\n\t}\n\tfor _, which := range []Case{Control, AcknowledgedExit, AfterJUnlink} {\n\t\tp, _, e, frames := fixture(t, which)\n\t\te.After = literalSnapshot(p.Spec().AdapterIdentity, rollbackfault.SQLOld)\n\t\tframes[frameIndex(frames, ReadbackResult)].Digest = mustSnapshotDigest(t, e.After)\n\t\trequireRejected(t, Observe(p, replay(p, frames), e))\n\t}\n\tp, receipt, e, _ := fixture(t, AfterTablePage)\n\te.After.Queries[5].Work.IndexPageReads = 0 // OLD green is empty but must perform a real index probe.\n\trequireRejected(t, Observe(p, receipt, e))\n\tp, receipt, e, _ = fixture(t, Control)\n\t// Matching scan and index wrong rows still cannot replace literal NEW.\n\te.After.Queries[0].Rows[0].Value = \"wrong\"\n\te.After.Queries[5].Rows[0].Value = \"wrong\"\n\te.After.Queries[6].Rows[0].Value = \"wrong\"\n\trequireRejected(t, Observe(p, receipt, e))\n}\nfunc TestRestartCompleteSuiteRejectsMissingDuplicateAndReuse(t *testing.T) {\n\tvar runs [5]ScenarioRun\n\tfor i, which := range Cases() {\n\t\tp, receipt, e, _ := fixture(t, which)\n\t\truns[i] = ScenarioRun{Plan: p, Receipt: receipt, Evidence: e}\n\t}\n\to := ObserveSuite(runs)\n\tif !o.EvidenceConsistent || o.Problem != ProblemNone || o.PhysicalValidated() || o.Pass() {\n\t\tt.Fatalf(\"five-value suite: %+v\", o)\n\t}\n\tmissing := runs\n\tmissing[4] = ScenarioRun{}\n\tif ObserveSuite(missing).EvidenceConsistent {\n\t\tt.Fatal(\"missing case admitted\")\n\t}\n\tduplicate := runs\n\tduplicate[4] = duplicate[3]\n\tif ObserveSuite(duplicate).EvidenceConsistent {\n\t\tt.Fatal(\"duplicate case admitted\")\n\t}\n\treused := runs\n\treused[1].Plan.spec.Binding.Root = reused[0].Plan.spec.Binding.Root\n\tif ObserveSuite(reused).EvidenceConsistent {\n\t\tt.Fatal(\"trial root reused\")\n\t}\n\tforged := SuiteObservation{EvidenceConsistent: true}\n\tif forged.PhysicalValidated() || forged.Pass() {\n\t\tt.Fatal(\"suite manufactured physical authority\")\n\t}\n}\n\nfunc TestRestartUnderlyingResultAndIdentityFailuresRemainRaw(t *testing.T) {\n\tp, _, e, frames := fixture(t, AfterTablePage)\n\tindex := -1\n\tfor i, f := range frames {\n\t\tif f.Kind == AdapterObserved && f.Adapter.Kind == rollbackfault.Returned && f.Adapter.Call.Point == rollbackfault.DBPage {\n\t\t\tindex = i\n\t\t\tbreak\n\t\t}\n\t}\n\tif index < 0 {\n\t\tt.Fatal(\"missing actual-return mutation point\")\n\t}\n\tfailed := &frames[index].Adapter.Result\n\tfailed.NativeN, failed.ReturnedN = 64, 64\n\tfailed.NativeError, failed.ReturnedError, failed.NormalizedError = rollbackfault.Underlying, rollbackfault.Underlying, rollbackfault.Underlying\n\tfailed.NativeErrorToken, failed.ReturnedErrorToken, failed.NormalizedErrorToken = 91, 91, 91\n\tfailed.Effect = rollbackfault.EffectUnknown\n\tr := replay(p, frames)\n\tkept, count := r.Frames()\n\tif int(count) != index+1 || kept[index].Adapter.Result != *failed {\n\t\tt.Fatal(\"original failed native result lost or replaced\")\n\t}\n\to := Observe(p, r, e)\n\trequireRejected(t, o)\n\tif o.Status != UnderlyingIOFailed || o.Commit != CommitUnknown || o.Acknowledged || o.WriterWaited || o.SQLState != rollbackfault.SQLStateUnknown {\n\t\tt.Fatalf(\"native error inferred unavailable facts: %+v\", o)\n\t}\n\tp, _, e, frames = fixture(t, BeforeJCreate)\n\tindex = frameIndex(frames, TargetReached)\n\tframes[index].Binding.Trial = strings.Repeat(\"x\", MaxIDLength+1)\n\tr = replay(p, frames)\n\tkept, count = r.Frames()\n\tif int(count) != index {\n\t\tt.Fatal(\"unbounded rejected identity retained\")\n\t}\n\tfor i := uint8(0); i < count; i++ {\n\t\tif kept[i].Binding != p.Spec().Binding {\n\t\t\tt.Fatal(\"untrusted binding retained\")\n\t\t}\n\t}\n\trequireRejected(t, Observe(p, r, e))\n}\nfunc TestRestartFrozenProtocolBudgetAndDetachedReceipt(t *testing.T) {\n\twant := [5]int{54, 18, 45, 51, 54}\n\tfor i, which := range Cases() {\n\t\tp, r, e, frames := fixture(t, which)\n\t\tif len(frames) != want[i] || len(frames) > MaxScenarioFrames || MaxScenarioFrames > MaxFrames || ExpectedAdapterEvents(which) > MaxAdapterEvents {\n\t\t\tt.Fatalf(\"protocol budget changed: %s %d\", CaseName(which), len(frames))\n\t\t}\n\t\tcopyFrames, count := r.Frames()\n\t\tcopyFrames[0].Binding.Trial = \"detached-mutation\"\n\t\tif count != uint8(len(frames)) || !Observe(p, r, e).ScenarioObserved {\n\t\t\tt.Fatal(\"receipt exposed mutable state\")\n\t\t}\n\t}\n}\n",
}

func restartGuardOwned(path string) bool {
	return path == rollbackRestartBridge || path == rollbackRestartStub || path == rollbackRestartNative || path == rollbackRestartModel || path == rollbackRestartModelTest
}
func restartGuardManual(path string) bool {
	return path == rollbackRestartBridge || path == rollbackRestartStub || path == rollbackRestartNative
}
// Only the exact restart bridge and native source share the fault gate with
// recovery. The stub has no environment consumer and receives no admission.
func restartGuardRecoveryManualPinned(path string, raw []byte) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	return (path == rollbackRestartBridge || path == rollbackRestartNative) && strings.SplitN(string(raw), "\n", 2)[0] == rollbackRestartTag && fmt.Sprintf("%x", sha256.Sum256(raw)) == rollbackRestartPins[path]
}
func restartGuardFaultImport(path string, imp *ast.ImportSpec) bool {
	return imp.Name == nil && (path == rollbackRestartBridge || path == rollbackRestartNative || path == rollbackRestartModel || path == rollbackRestartModelTest)
}
func restartGuardRoot(name string) bool {
	return name == rollbackRestartControllerRoot || name == rollbackRestartWriterRoot || name == rollbackRestartRecoveryRoot
}
func restartGuardAPI(name string) bool {
	switch name {
	case "NewManualRollbackRestartChannel", "ArmManualRollbackRestart", "CaptureManualRollbackRestartCommit", "CompleteManualRollbackRestartControl":
		return true
	}
	return false
}
func restartGuardProtected(name string) bool {
	return restartGuardAPI(name) || strings.HasPrefix(name, "rollbackRestart") || strings.HasPrefix(name, "manualRestart") || strings.HasPrefix(name, "manualRollbackRestart") || strings.HasPrefix(name, "ManualRollbackRestart") || restartGuardRoot(name)
}

func restartGuardTokens(raw string) string {
	// The parser establishes statement boundaries; inserted versus explicit
	// semicolons are formatting, while every literal/token stays significant.
	return strings.ReplaceAll(rollbackFaultShapeTokens(raw), ";:\";\"\n", "")
}

// Statements are compared as Go tokens so spaces inside string literals remain
// significant. A sequence must occur directly in its supplied block, never in
// a dead branch, a closure, or a comment.
func restartGuardSequence(block *ast.BlockStmt, set *token.FileSet, raw []byte, parts ...string) int {
	if block == nil || len(parts) == 0 {
		return -1
	}
	for i := 0; i+len(parts) <= len(block.List); i++ {
		match := true
		for j, part := range parts {
			if restartGuardTokens(oracleNodeText(set, raw, block.List[i+j])) != restartGuardTokens(part) {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
func restartGuardExactBody(path string, functions map[string]*ast.FuncDecl, set *token.FileSet, raw []byte, name, expected, diagnostic string) []string {
	fn := functions[name]
	if fn == nil || fn.Body == nil || restartGuardTokens(oracleNodeText(set, raw, fn.Body)) != restartGuardTokens(expected) {
		return []string{path + ": rollback restart contract: " + diagnostic + ": " + name}
	}
	return nil
}
func restartGuardImportBound(file *ast.File, name, importPath string) bool {
	count := 0
	for _, imp := range file.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return false
		}
		local := filepath.Base(value)
		if imp.Name != nil {
			local = imp.Name.Name
		}
		if local == name {
			if value != importPath || imp.Name != nil {
				return false
			}
			count++
		}
	}
	return count == 1
}

// A reference key contains path, enclosing top-level function, and the exact
// direct call. No aliases, method values, copied helpers or tag-wide exception
// are represented here. The inventories below are authored with the facility.
func restartGuardCallKey(path string, id *ast.Ident, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte) string {
	call := walPublicationDirectCall(id, parents)
	fn := oracleEnclosingFunction(id, parents)
	if call == nil || fn == nil || fn.Recv != nil {
		return ""
	}
	return path + "|" + fn.Name.Name + "|" + rollbackFaultText(set, raw, call)
}

var restartGuardOldCalls = map[string]bool{
	rollbackRestartBridge + "|CompleteManualRollbackRestartControl|finishRollbackFault(scope.fault, scope.commitError)": true,
	rollbackRestartBridge + "|CompleteManualRollbackRestartControl|disarmRollbackFault(scope.fault)":                    true,
}
var restartGuardSQLCalls = map[string]bool{
	rollbackRestartBridge + "|ArmManualRollbackRestart|rollbackfault.SQLPlan(plan.Spec().AdapterIdentity, rollbackfault.SQLControl)":                                                                         true,
	rollbackRestartBridge + "|ArmManualRollbackRestart|ArmManualRollbackSQLFault(tx, faultPlan)":                                                                                                             true,
	rollbackRestartBridge + "|CaptureManualRollbackRestartCommit|ManualRollbackSQLState(nil)":                                                                                                                true,
	rollbackRestartNative + "|manualRestartRead|manualRollbackSQLRead(&c.sql.ledger, path, bound)":                                                                                                           true,
	rollbackRestartNative + "|manualRestartDecode|manualRollbackSQLDecode(raw, value)":                                                                                                                       true,
	rollbackRestartNative + "|manualRestartAdmit|manualRollbackSQLNamespace(c.root)":                                                                                                                         true,
	rollbackRestartNative + "|manualRestartAdmit|manualRollbackSQLDecode(manifest, &inventory)":                                                                                                              true,
	rollbackRestartNative + "|manualRestartAdmit|manualRollbackSQLHash(entry.SHA256)":                                                                                                                        true,
	rollbackRestartNative + "|manualRestartAdmit|manualRollbackSQLBaseline()":                                                                                                                                true,
	rollbackRestartNative + "|manualRestartFamilies|manualRollbackSQLNamespace(path)":                                                                                                                        true,
	rollbackRestartModel + "|Observe|rollbackfault.ValidateSQLSnapshot(e.Before, p.spec.AdapterIdentity, rollbackfault.SQLOld)":                                                                              true,
	rollbackRestartModel + "|Observe|rollbackfault.ValidateSQLSnapshot(e.After, p.spec.AdapterIdentity, state)":                                                                                              true,
	rollbackRestartModel + "|SnapshotDigest|rollbackfault.ValidateSQLSnapshot(s, s.Identity, rollbackfault.SQLOld)":                                                                                          true,
	rollbackRestartModel + "|SnapshotDigest|rollbackfault.ValidateSQLSnapshot(s, s.Identity, rollbackfault.SQLNew)":                                                                                          true,
	rollbackRestartNative + "|manualRestartSpawn|manualRollbackSQLRegistryEmpty()":                                                                                                                           true,
	rollbackRestartNative + "|manualRestartFreshRegistry|pager.ManualRollbackSQLState(nil)":                                                                                                                  true,
	rollbackRestartNative + "|manualRestartWriter|manualRollbackSQLRegistryEmpty()":                                                                                                                          true,
	rollbackRestartNative + "|manualRestartWriter|manualRollbackSQLOpen(&c.sql, filepath.Join(c.root, \"trial\", \"trial.db\"), false)":                                                                      true,
	rollbackRestartNative + "|manualRestartRecovery|manualRollbackSQLRegistryEmpty()":                                                                                                                        true,
	rollbackRestartNative + "|manualRestartRecovery|manualRollbackSQLRecoverCopy(ctx, replayPath)":                                                                                                           true,
	rollbackRestartNative + "|manualRestartRecovery|manualRollbackSQLReadback(&c.sql, ctx, replayPath, identity)":                                                                                            true,
	rollbackRestartNative + "|manualRestartRecovery|rollbackfault.ValidateSQLSnapshot(snapshot, identity, rollbackrestart.ExpectedSQLState(c.plan.Spec().Binding.Case))":                                     true,
	rollbackRestartNative + "|manualRestartController|manualRollbackSQLRegistryEmpty()":                                                                                                                      true,
	rollbackRestartNative + "|manualRestartController|manualRollbackSQLReadback(&c.sql, context.Background(), filepath.Join(c.root, \"inputs\", \"baseline.input.db\"), c.plan.Spec().AdapterIdentity)":      true,
	rollbackRestartNative + "|manualRestartController|rollbackfault.ValidateSQLSnapshot(before, c.plan.Spec().AdapterIdentity, rollbackfault.SQLOld)":                                                        true,
	rollbackRestartNative + "|manualRestartFailureSnapshot|rollbackfault.ValidateSQLSnapshot(c.evidence.After, c.plan.Spec().AdapterIdentity, rollbackrestart.ExpectedSQLState(c.plan.Spec().Binding.Case))": true,
}
var restartGuardOldPrivateContexts = map[string]bool{
	rollbackRestartBridge + "|type manualRollbackRestartScope|fault *rollbackFaultScope": true,
	rollbackRestartBridge + "|func rollbackRestartBefore|ticket rollbackFaultTicket":     true,
	rollbackRestartBridge + "|func rollbackRestartAfter|ticket rollbackFaultTicket":      true,
	rollbackRestartBridge + "|func rollbackRestartObserve|ticket rollbackFaultTicket":    true,
}
var restartGuardSQLContexts = map[string]bool{
	rollbackRestartNative + "|type manualRestartConfig|sql manualRollbackSQLConfig":                                       true,
	rollbackRestartNative + "|type manualRestartConfig|family [5]manualRollbackSQLConfig":                                 true,
	rollbackRestartNative + "|manualRestartImage|manualRollbackSQLOwner{file: image, path: \"/proc/self/exe\"}":           true,
	rollbackRestartNative + "|manualRestartSyncDirectory|manualRollbackSQLOwner{file: dir, path: f.root}":                 true,
	rollbackRestartNative + "|manualRestartFamilies|manualRollbackSQLOwner{file: directory, path: c.root}":                true,
	rollbackRestartNative + "|func manualRestartAdmit|receipt manualRollbackSQLCompile":                                   true,
	rollbackRestartNative + "|manualRestartPipe|manualRollbackSQLOwner{file: read, path: \"owned restart pipe reader\"}":  true,
	rollbackRestartNative + "|manualRestartPipe|manualRollbackSQLOwner{file: write, path: \"owned restart pipe writer\"}": true,
}
var restartGuardNativeCalls = map[string]bool{
	rollbackRestartBridge + "|rollbackRestartObserve|os.Exit(86)":                             true,
	rollbackRestartBridge + "|CommitReturned|os.Exit(86)":                                     true,
	rollbackRestartBridge + "|rollbackRestartAbort|os.Exit(87)":                               true,
	rollbackRestartNative + "|manualRestartSpawn|exec.Command(c.binding.BinaryPath, args...)": true,
	rollbackRestartNative + "|manualRestartSpawn|command.Start()":                             true,
}

func restartGuardOldReference(path string, id *ast.Ident, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte) bool {
	return restartGuardOldCalls[restartGuardCallKey(path, id, parents, set, raw)]
}
func restartGuardSQLReference(path string, id *ast.Ident, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte) bool {
	if id.Name == "ObserveSQLLedger" {
		return false
	}
	if restartGuardSQLCalls[restartGuardCallKey(path, id, parents, set, raw)] {
		return true
	}
	return restartGuardSQLContexts[restartGuardContextKey(path, id, parents, set, raw)]
}

func restartGuardContextKey(path string, id *ast.Ident, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte) string {
	// A nearest exact field/type/value context is separate from direct calls.
	// No bare path or function name can authorize an ownership reference.
	for node := ast.Node(id); node != nil; node = parents[node] {
		switch value := node.(type) {
		case *ast.Field:
			for owner := parents[value]; owner != nil; owner = parents[owner] {
				if typ, ok := owner.(*ast.TypeSpec); ok {
					return path + "|type " + typ.Name.Name + "|" + rollbackFaultText(set, raw, value)
				}
				if fn, ok := owner.(*ast.FuncDecl); ok {
					return path + "|func " + fn.Name.Name + "|" + rollbackFaultText(set, raw, value)
				}
			}
			return ""
		case *ast.ValueSpec:
			if fn := oracleEnclosingFunction(value, parents); fn != nil {
				return path + "|func " + fn.Name.Name + "|" + rollbackFaultText(set, raw, value)
			}
			return ""
		case *ast.CompositeLit:
			fn := oracleEnclosingFunction(value, parents)
			if fn != nil {
				return path + "|" + fn.Name.Name + "|" + rollbackFaultText(set, raw, value)
			}
			return ""
		case *ast.FuncDecl:
			return ""
		}
	}
	return ""
}
func restartGuardOldPrivate(path string, id *ast.Ident, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte) bool {
	if path == rollbackRestartStub && id.Name == "rollbackFaultTicket" {
		for node := ast.Node(id); node != nil; node = parents[node] {
			if fn, ok := node.(*ast.FuncDecl); ok {
				if fn.Recv != nil || (fn.Name.Name != "rollbackRestartBefore" && fn.Name.Name != "rollbackRestartAfter") {
					return false
				}
				return restartGuardTokens(oracleNodeText(set, raw, fn)) == restartGuardTokens("func "+fn.Name.Name+"(ticket rollbackFaultTicket) {}")
			}
		}
	}
	if path == rollbackRestartBridge && (id.Name == "errRollbackFaultGate" || id.Name == "errRollbackFaultScope") {
		fn := oracleEnclosingFunction(id, parents)
		if fn == nil {
			return false
		}
		for node := parents[id]; node != nil && node != fn; node = parents[node] {
			switch node.(type) {
			case *ast.CallExpr:
				if restartGuardErrorContexts[fn.Name.Name+"|"+rollbackFaultText(set, raw, node)] {
					return true
				}
			case *ast.ReturnStmt, *ast.AssignStmt:
				return restartGuardErrorContexts[fn.Name.Name+"|"+rollbackFaultText(set, raw, node)]
			}
		}
		return false
	}
	return restartGuardOldCalls[restartGuardCallKey(path, id, parents, set, raw)] || restartGuardOldPrivateContexts[restartGuardContextKey(path, id, parents, set, raw)]
}

var restartGuardErrorContexts = map[string]bool{
	"NewManualRollbackRestartChannel|return nil, errRollbackFaultGate":                                         true,
	"NewManualRollbackRestartChannel|return nil, errRollbackFaultScope":                                        true,
	"NewManualRollbackRestartChannel|return channel, errRollbackFaultScope":                                    true,
	"NewManualRollbackRestartChannel|return channel, errors.Join(errRollbackFaultScope, controlErr, eventErr)": true,
	"rollbackRestartObserve|rollbackRestartTransport(errRollbackFaultScope)":                                   true,
	"rollbackRestartObserve|errors.Join(errRollbackFaultScope, err)":                                           true,
	"send|return errRollbackFaultScope":                                                                        true,
	"send|c.problem = errRollbackFaultScope":                                                                   true,
	"receive|return rollbackrestart.Frame{}, errRollbackFaultScope":                                            true,
	"receive|c.problem = errRollbackFaultScope":                                                                true,
	"Ready|return errRollbackFaultScope":                                                                       true,
	"ArmManualRollbackRestart|return nil, errRollbackFaultGate":                                                true,
	"ArmManualRollbackRestart|return nil, errRollbackFaultScope":                                               true,
	"CaptureManualRollbackRestartCommit|out.Problem = errRollbackFaultScope":                                   true,
	"CommitReturned|return errRollbackFaultScope":                                                              true,
	"CompleteManualRollbackRestartControl|return errRollbackFaultScope":                                        true,
	"Finished|return errRollbackFaultScope":                                                                    true,
	"CloseNormal|return errRollbackFaultScope":                                                                 true,
}

func inspectRollbackRestartSource(path string, raw []byte) ([]string, error) {
	path = filepath.ToSlash(filepath.Clean(path))
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, raw, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var findings []string
	report := func(message string) { findings = append(findings, path+": rollback restart contract: "+message) }
	owned, manual := restartGuardOwned(path), restartGuardManual(path)
	pure := path == rollbackRestartModel || path == rollbackRestartModelTest
	first := strings.SplitN(string(raw), "\n", 2)[0]
	if strings.HasPrefix(path, "internal/pager/rollback_restart") || strings.HasPrefix(path, "sqlite/rollback_restart") || strings.HasPrefix(path, "internal/testkit/rollbackrestart/") {
		if !owned {
			report("unreviewed restart source inventory member")
		}
	}
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if (strings.HasPrefix(comment.Text, "//go:build") || strings.HasPrefix(comment.Text, "// +build")) && strings.Contains(comment.Text, "manual_restart_rollback") && !manual {
				report("restart tag outside exact three-file selection")
			}
		}
	}
	if owned {
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != rollbackRestartPins[path] {
			report("proposed restart byte pin changed or pending")
		}
		if shape, ok := rollbackRestartShapes[path]; ok {
			findings = append(findings, rollbackFaultDeclarationShape(path, raw, set, file, shape)...)
		} else {
			report("restart declaration snapshot pending")
		}
		if manual {
			wanted := rollbackRestartTag
			if path == rollbackRestartStub {
				wanted = rollbackRestartStubTag
			}
			if first != wanted || strings.Count(string(raw), "//go:build") != 1 || strings.Contains(string(raw), "// +build") {
				report("exact restart AND build gate and fault-only complement required")
			}
		} else if strings.Contains(string(raw), "//go:build") || strings.Contains(string(raw), "// +build") {
			report("portable restart model cannot add build constraints")
		}
		wanted := "rollbackrestart"
		if path == rollbackRestartNative {
			wanted = "sqlite"
		}
		if path == rollbackRestartBridge || path == rollbackRestartStub {
			wanted = "pager"
		}
		if file.Name.Name != wanted {
			report("exact restart package identity required")
		}
	}
	imports := map[string]string{}
	for _, imp := range file.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(value)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		imports[name] = value
		if value == rollbackRestartImport && (imp.Name != nil || path != rollbackRestartBridge && path != rollbackRestartNative) {
			report("restart model import outside exact unaliased consumers")
		}
		if owned && imp.Name != nil {
			report("restart aliased/dot/blank import forbidden")
		}
		if owned && (value == "unsafe" || value == "reflect" || strings.HasPrefix(value, "net") || value == "golang.org/x/sys/unix") {
			report("restart native/reflection/network import forbidden")
		}
		if pure {
			allowed := value == rollbackFaultImport || value == "bytes" || value == "crypto/sha256" || value == "encoding/binary" || value == "encoding/hex" || value == "encoding/json" || path == rollbackRestartModelTest && (value == "testing" || value == "strings")
			if !allowed {
				report("restart model imports must be descriptor-free encoding/SQL values and testing only")
			}
		}
		if path == rollbackRestartStub {
			report("fault-only restart stub cannot import anything")
		}
		if owned && value == "os/exec" && path != rollbackRestartNative {
			report("process creation import belongs only to exact controller source")
		}
		if manual {
			allowed := ""
			if path == rollbackRestartBridge {
				allowed = "encoding/binary errors fmt io os strconv sync github.com/ms-xiaosnow/gosqlite/internal/vfs " + rollbackFaultImport + " " + rollbackRestartImport
			}
			if path == rollbackRestartNative {
				allowed = "bytes context crypto/rand crypto/sha256 encoding/binary encoding/hex encoding/json errors fmt io os os/exec path/filepath strings sync syscall testing time github.com/ms-xiaosnow/gosqlite/internal/pager " + rollbackFaultImport + " " + rollbackRestartImport
			}
			if !strings.Contains(" "+allowed+" ", " "+value+" ") {
				report("manual restart import outside finite reviewed closure")
			}
		}
	}
	parents := oracleParents(file)
	functions := readOnlyWALManualFunctions(file)
	roots := map[string]int{}
	ioCounts := map[string]int{}
	effectCounts := map[string]int{}
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if owned && (fn.Name.Name == "init" || fn.Name.Name == "TestMain" || fn.Body == nil) {
			report("restart initializer/TestMain/external implementation forbidden")
		}
		entry := strings.HasPrefix(fn.Name.Name, "Test") || strings.HasPrefix(fn.Name.Name, "Benchmark") || strings.HasPrefix(fn.Name.Name, "Fuzz") || strings.HasPrefix(fn.Name.Name, "Example")
		if manual && entry {
			if path != rollbackRestartNative || !restartGuardRoot(fn.Name.Name) || fn.Recv != nil {
				report("exact three restart role roots only")
			}
			roots[fn.Name.Name]++
		}
	}
	if path == rollbackRestartNative {
		if len(roots) != 3 {
			report("exact three restart role roots only")
		}
		for _, name := range []string{rollbackRestartControllerRoot, rollbackRestartWriterRoot, rollbackRestartRecoveryRoot} {
			if roots[name] != 1 {
				report("missing/duplicate restart role root " + name)
			}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok {
			if restartGuardProtected(id.Name) && !owned && path != rollbackRestartGuard && path != rollbackRestartGuardTest {
				if !restartGuardWrapperReference(path, id, parents, set, raw) {
					report("restart declaration/reference outside exact source closure")
				}
			}
		}
		if call, ok := node.(*ast.CallExpr); ok {
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok && (selector.Sel.Name == "Getenv" || selector.Sel.Name == "LookupEnv") {
				fn := oracleEnclosingFunction(call, parents)
				for _, argument := range call.Args {
					if literal, ok := argument.(*ast.BasicLit); ok && literal.Kind == token.STRING {
						value, unquoteErr := strconv.Unquote(literal.Value)
						if unquoteErr == nil && (value == "GOSQLITE_MANUAL_RESTART_ROLLBACK" || strings.HasPrefix(value, "GOSQLITE_RESTART_")) {
							if fn == nil || !restartGuardEnvReference(path, fn.Name.Name, rollbackFaultText(set, raw, call)) {
								report("restart environment consumer outside exact rejecting admission")
							}
						}
					}
				}
			}
		}
		if !owned {
			return true
		}
		if id, ok := node.(*ast.Ident); ok {
			if path == rollbackRestartNative && restartGuardEffectHelper(id.Name) {
				declaration, isDeclaration := parents[id].(*ast.FuncDecl)
				if !isDeclaration || declaration.Name != id {
					key := restartGuardCallKey(path, id, parents, set, raw)
					if restartGuardEffectCalls[key] == 0 {
						report("restart effect helper outside exact direct consumer: " + id.Name)
					} else {
						effectCounts[key]++
					}
				}
			}
			if restartGuardAPI(id.Name) {
				declaration, isDeclaration := parents[id].(*ast.FuncDecl)
				if !isDeclaration || declaration.Name != id || path != rollbackRestartBridge || declaration.Recv != nil {
					if !restartGuardAPICalls[restartGuardCallKey(path, id, parents, set, raw)] {
						report("restart API outside exact role/path/direct call; aliases forbidden")
					}
				}
			}
			if fn := functions[id.Name]; fn != nil && fn.Recv == nil && restartGuardProtected(id.Name) {
				declaration, isDeclaration := parents[id].(*ast.FuncDecl)
				if (!isDeclaration || declaration.Name != id) && rollbackFaultDirectReference(id, parents) == nil {
					report("restart helper function value/alias/callback forbidden")
				}
			}
		}
		switch node.(type) {
		case *ast.GoStmt, *ast.SendStmt:
			if !restartGuardWaitClosure(path, node, parents, set, raw) {
				report("restart goroutine/channel execution forbidden")
			}
		case *ast.FuncLit:
			if manual && !restartGuardWaitClosure(path, node, parents, set, raw) {
				report("restart callback/closure execution forbidden")
			}
		}
		if gen, ok := node.(*ast.GenDecl); ok && gen.Tok == token.VAR && (pure || path == rollbackRestartStub || path == rollbackRestartNative) {
			if _, top := parents[gen].(*ast.File); top {
				report("restart package state forbidden outside exact pager scope")
			}
		}
		if typ, ok := node.(*ast.TypeSpec); ok && typ.Assign.IsValid() {
			report("restart ownership/type aliases forbidden")
		}
		if manual {
			if typ, ok := node.(*ast.FuncType); ok {
				fn, declaration := parents[typ].(*ast.FuncDecl)
				if (!declaration || fn.Type != typ) && !restartGuardWaitClosure(path, node, parents, set, raw) {
					report("manual restart function/callback type outside exact declarations forbidden")
				}
			}
			if _, ok := node.(*ast.InterfaceType); ok {
				report("manual restart dynamic interface capability forbidden")
			}
			if typ, ok := node.(*ast.ChanType); ok {
				allowed := false
				if path == rollbackRestartNative {
					if field, ok := parents[typ].(*ast.Field); ok && len(field.Names) == 1 && field.Names[0].Name == "wait" && rollbackFaultText(set, raw, typ) == "chan manualRestartWaitResult" {
						for parent := parents[field]; parent != nil; parent = parents[parent] {
							if owner, ok := parent.(*ast.TypeSpec); ok {
								allowed = owner.Name.Name == "manualRestartChild"
								break
							}
						}
					}
					if call, ok := parents[typ].(*ast.CallExpr); ok && rollbackFaultText(set, raw, call) == "make(chan manualRestartWaitResult, 1)" {
						fn := oracleEnclosingFunction(call, parents)
						allowed = fn != nil && fn.Name.Name == "manualRestartSpawn"
					}
				}
				if !allowed {
					report("manual restart channel outside single actual Wait owner forbidden")
				}
			}
			if id, ok := node.(*ast.Ident); ok && id.Name == "any" {
				field, isField := parents[id].(*ast.Field)
				fn := oracleEnclosingFunction(id, parents)
				if path != rollbackRestartNative || !isField || len(field.Names) != 1 || field.Names[0].Name != "value" || field.Type != id || fn == nil || fn.Name.Name != "manualRestartDecode" {
					report("manual restart any capability is only the exact canonical JSON parameter")
				}
			}
		}
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		call, called := parents[sel].(*ast.CallExpr)
		called = called && call.Fun == sel
		fn := oracleEnclosingFunction(sel, parents)
		if manual {
			key := ""
			if called && fn != nil {
				key = path + "|" + fn.Name.Name + "|" + rollbackFaultText(set, raw, call)
			}
			if restartGuardIOSelector(sel.Sel.Name) {
				if !called || restartGuardIOCalls[key] == 0 {
					report("restart I/O outside exact file/function/call triplet: " + sel.Sel.Name)
				} else {
					ioCounts[key]++
				}
			}
			if qualifier, ok := sel.X.(*ast.Ident); ok {
				allowed := ""
				switch imports[qualifier.Name] {
				case "os":
					allowed = "Args ErrNotExist File FileInfo Getenv Getpid Getuid Lstat Mkdir ModeSymlink Open Pipe ProcessState SameFile Exit ModeNamedPipe NewFile Stderr"
				case "io":
					allowed = "EOF ErrShortWrite LimitReader ReadAll ReadFull"
				case "syscall":
					allowed = "Stat_t Statfs Statfs_t"
				case "crypto/rand":
					allowed = "Read"
				case "github.com/ms-xiaosnow/gosqlite/internal/vfs":
					allowed = "SameFile"
				}
				if allowed != "" && !strings.Contains(" "+allowed+" ", " "+sel.Sel.Name+" ") {
					report("restart system capability outside closed package selector inventory")
				}
			}
		}
		if qualifier, ok := sel.X.(*ast.Ident); ok && (imports[qualifier.Name] == "os" || imports[qualifier.Name] == "syscall") {
			if sel.Sel.Name == "StartProcess" || sel.Sel.Name == "ForkExec" || sel.Sel.Name == "Exec" {
				report("low-level restart process bypass forbidden")
			}
		}
		switch sel.Sel.Name {
		case "CommandContext", "Kill", "Remove", "RemoveAll", "Rename", "Link", "Symlink", "Chmod", "Truncate", "RetryPlaintextCleanup", "RetryCleanup", "Reset", "Setenv", "Unsetenv", "Environ", "Chdir", "TempDir", "MkdirTemp", "CreateTemp", "Parallel", "Skip", "Skipf", "SkipNow":
			report("restart forbidden ownership/process/test control reference " + sel.Sel.Name)
		case "Signal":
			if !restartGuardSignalField(path, sel, parents, set, raw, file) {
				report("restart forbidden ownership/process/test control reference Signal")
			}
		case "Run":
			if manual {
				report("restart subtest or hidden native execution forbidden")
			}
		case "Rollback":
			if manual {
				report("restart cannot rollback or compensate a crash prefix")
			}
		case "Commit":
			if manual && !restartGuardCommitField(path, sel, parents, set, raw, file) && (!called || fn == nil || path != rollbackRestartNative || fn.Name.Name != "manualRestartWriter" || rollbackFaultText(set, raw, call) != "tx.Commit(ctx)") {
				report("restart public Commit outside unique writer call; method values forbidden")
			}
		case "Close", "Sync", "Open", "OpenFile", "NewFile", "Pipe", "Mkdir", "ReadDir", "RetirementComplete":
			if manual && (!called || fn == nil || !restartGuardOwnerCalls[path+"|"+fn.Name.Name+"|"+rollbackFaultText(set, raw, call)]) {
				report("restart owner/descriptor action outside exact once-only boundary: " + sel.Sel.Name)
			}
		case "Exit", "Command", "Start", "Wait", "RecoverRollback", "Fd", "SyscallConn", "RawConn":
			if !called || (fn == nil || !restartGuardNativeCalls[path+"|"+fn.Name.Name+"|"+rollbackFaultText(set, raw, call)]) && !restartGuardWaitClosure(path, node, parents, set, raw) {
				report("restart native action outside exact role/function/call triplet: " + sel.Sel.Name)
			}
		}
		if pure {
			switch sel.Sel.Name {
			case "Open", "OpenFile", "Create", "ReadFile", "WriteFile", "Close", "Sync", "Begin", "Getenv", "LookupEnv", "Rollback", "Commit":
				if called || sel.Sel.Name != "Commit" {
					report("non-memory restart model reference " + sel.Sel.Name)
				}
			}
		}
		return true
	})
	for key, want := range restartGuardEffectCalls {
		if strings.HasPrefix(key, path+"|") && effectCounts[key] != want {
			report("restart effect helper exact call count changed: " + key)
		}
	}
	for key, want := range restartGuardIOCalls {
		if strings.HasPrefix(key, path+"|") && ioCounts[key] != want {
			report("restart exact I/O call count changed: " + key)
		}
	}
	if path == rollbackRestartStub {
		findings = append(findings, restartGuardStubSemantics(path, raw, set, file)...)
	}
	if path == rollbackRestartBridge {
		findings = append(findings, restartGuardBridgeSemantics(path, raw, set, file, functions)...)
	}
	if path == rollbackRestartNative {
		findings = append(findings, restartGuardNativeSemantics(path, raw, set, file, functions)...)
	}
	if path == rollbackRestartModel {
		findings = append(findings, restartGuardModelSemantics(path, raw, set, file, functions)...)
	}
	if path == rollbackFaultManual {
		findings = append(findings, restartGuardAdapterSemantics(path, raw, set, file, functions)...)
	}
	if path == "internal/testscope/main.go" {
		findings = append(findings, inspectRollbackRestartWiring(raw)...)
	}
	return findings, nil
}

// Signal is also an integer in the descriptor-free model. These five
// exceptions bind a concrete type chain and exact comparison/table context;
// no call, retained method value, alias or arbitrary non-call use is admitted.
// The test source relies on the same-package model declarations, whose two
// concrete fields are independently checked whenever the model is scanned.
func restartGuardSignalField(path string, sel *ast.SelectorExpr, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte, file *ast.File) bool {
	if sel.Sel.Name != "Signal" || file.Name.Name != "rollbackrestart" {
		return false
	}
	var fn *ast.FuncDecl
	for node := ast.Node(sel); node != nil; node = parents[node] {
		if declaration, ok := node.(*ast.FuncDecl); ok {
			fn = declaration
			break
		}
	}
	if fn == nil || fn.Recv != nil || fn.Body == nil || fn.Type.TypeParams != nil {
		return false
	}
	if path == rollbackRestartModel {
		if !restartGuardSignalModelTypes(file, set, raw) {
			return false
		}
		comparison, ok := parents[sel].(*ast.BinaryExpr)
		if !ok || comparison.X != sel {
			return false
		}
		if fn.Name.Name == "validSelf" || fn.Name.Name == "validSpawn" {
			expected := restartGuardSignalSelfDeclaration
			if fn.Name.Name == "validSpawn" {
				expected = restartGuardSignalSpawnDeclaration
			}
			return rollbackFaultText(set, raw, sel) == "p.Signal" && comparison.Op == token.EQL && rollbackFaultText(set, raw, comparison.Y) == "0" && restartGuardTokens(oracleNodeText(set, raw, fn)) == restartGuardTokens(expected)
		}
		if fn.Name.Name != "ValidateFrame" || rollbackFaultText(set, raw, fn.Type.Params) != "(f Frame)" || fn.Type.Results == nil || rollbackFaultText(set, raw, fn.Type.Results) != "Problem" || rollbackFaultText(set, raw, sel) != "f.Process.Signal" || (comparison.Op != token.LSS || rollbackFaultText(set, raw, comparison.Y) != "0") && (comparison.Op != token.GTR || rollbackFaultText(set, raw, comparison.Y) != "64") {
			return false
		}
		// The one unchanged range check is directly in the typed function,
		// so a nested/shadowed receiver, added call or moved check cannot match.
		var conditional *ast.IfStmt
		for node := ast.Node(comparison); node != nil && node != fn; node = parents[node] {
			if value, ok := node.(*ast.IfStmt); ok {
				conditional = value
				break
			}
		}
		if conditional == nil || parents[conditional] != fn.Body || restartGuardTokens(oracleNodeText(set, raw, conditional)) != restartGuardTokens(restartGuardSignalRangeStatement) {
			return false
		}
		matches := 0
		for _, statement := range fn.Body.List {
			if restartGuardTokens(oracleNodeText(set, raw, statement)) == restartGuardTokens(restartGuardSignalRangeStatement) {
				matches++
			}
		}
		return matches == 1
	}
	if path != rollbackRestartModelTest || fn.Name.Name != "TestRestartRejectsProtocolAndAdapterFalsePositives" || rollbackFaultText(set, raw, fn.Type.Params) != "(t *testing.T)" || fn.Type.Results != nil || !restartGuardImportBound(file, "testing", "testing") || rollbackFaultText(set, raw, sel) != "f.Process.Signal" {
		return false
	}
	// The test file cannot replace the model's concrete names. Local
	// shadows cannot precede the table because it is the first statement.
	for _, declaration := range file.Decls {
		if gen, ok := declaration.(*ast.GenDecl); ok {
			for _, spec := range gen.Specs {
				if typ, ok := spec.(*ast.TypeSpec); ok && (typ.Name.Name == "Frame" || typ.Name.Name == "ProcessEvidence" || typ.Name.Name == "int") {
					return false
				}
			}
		}
	}
	assignment, ok := parents[sel].(*ast.AssignStmt)
	if !ok || assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 2 || len(assignment.Rhs) != 2 || assignment.Lhs[1] != sel || rollbackFaultText(set, raw, assignment) != "f.Process.State, f.Process.Signal = ProcessSignaled, 9" {
		return false
	}
	body, ok := parents[assignment].(*ast.BlockStmt)
	if !ok || len(body.List) != 1 || body.List[0] != assignment {
		return false
	}
	literal, ok := parents[body].(*ast.FuncLit)
	if !ok || literal.Body != body || rollbackFaultText(set, raw, literal.Type) != "func(f *Frame)" {
		return false
	}
	row, ok := parents[literal].(*ast.CompositeLit)
	if !ok || row.Type != nil || len(row.Elts) != 4 || row.Elts[3] != literal || restartGuardTokens(oracleNodeText(set, raw, row)) != restartGuardTokens(restartGuardSignalTestRow) {
		return false
	}
	table, ok := parents[row].(*ast.CompositeLit)
	if !ok || restartGuardTokens(oracleNodeText(set, raw, table.Type)) != restartGuardTokens(`[]struct { name string; which Case; kind FrameKind; mutate func(*Frame) }`) {
		return false
	}
	binding, ok := parents[table].(*ast.AssignStmt)
	if !ok || binding.Tok != token.DEFINE || len(binding.Lhs) != 1 || len(binding.Rhs) != 1 || rollbackFaultText(set, raw, binding.Lhs[0]) != "mutants" || binding.Rhs[0] != table || parents[binding] != fn.Body || len(fn.Body.List) == 0 || fn.Body.List[0] != binding {
		return false
	}
	matches := 0
	for _, entry := range table.Elts {
		if restartGuardTokens(oracleNodeText(set, raw, entry)) == restartGuardTokens(restartGuardSignalTestRow) {
			matches++
		}
	}
	return matches == 1
}

func restartGuardSignalModelTypes(file *ast.File, set *token.FileSet, raw []byte) bool {
	frames, processes, processFields := 0, 0, 0
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			typ, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if typ.Name.Name == "int" {
				return false
			}
			if typ.Name.Name == "ProcessEvidence" {
				processes++
				if typ.Assign.IsValid() || restartGuardTokens(oracleNodeText(set, raw, typ)) != restartGuardTokens(restartGuardSignalProcessType) {
					return false
				}
			}
			if typ.Name.Name == "Frame" {
				frames++
				record, ok := typ.Type.(*ast.StructType)
				if !ok || typ.Assign.IsValid() || typ.TypeParams != nil {
					return false
				}
				for _, field := range record.Fields.List {
					for _, name := range field.Names {
						if name.Name == "Process" {
							processFields++
							concrete, ok := field.Type.(*ast.Ident)
							if !ok || len(field.Names) != 1 || concrete.Name != "ProcessEvidence" {
								return false
							}
						}
					}
				}
			}
		}
	}
	return frames == 1 && processes == 1 && processFields == 1
}

const restartGuardSignalProcessType = `ProcessEvidence struct {
	PID int64
	Handle uint64
	Image FileIdentity
	ImageHash Digest
	ActualImage bool
	SpawnObserved bool
	WaitObserved bool
	State ProcessState
	ExitCode int
	Signal int
	TimedOut bool
}`
const restartGuardSignalSelfDeclaration = `func validSelf(p ProcessEvidence, spec PlanSpec) bool {
	return p.PID > 0 && p.Image == spec.Image && p.ImageHash == spec.Binding.Binary && p.ActualImage && !p.SpawnObserved && !p.WaitObserved && p.State == ProcessRunning && p.ExitCode == 0 && p.Signal == 0 && !p.TimedOut
}`
const restartGuardSignalSpawnDeclaration = `func validSpawn(p ProcessEvidence, spec PlanSpec) bool {
	return p.PID > 0 && p.Handle != 0 && p.Image == spec.Image && p.ImageHash == spec.Binding.Binary && !p.ActualImage && p.SpawnObserved && !p.WaitObserved && p.State == ProcessRunning && p.ExitCode == 0 && p.Signal == 0 && !p.TimedOut
}`
const restartGuardSignalRangeStatement = `if f.Process.State > ProcessSignaled || f.Process.Signal < 0 || f.Process.Signal > 64 || f.Process.ExitCode < 0 || f.Process.ExitCode > 255 || f.Commit.Value > CommitUncertain || f.Diagnostic > EvidenceComplete {
		return ProblemPayload
	}`
const restartGuardSignalTestRow = `{"signal-is-not-target", AfterJUnlink, WriterWaitObserved, func(f *Frame) { f.Process.State, f.Process.Signal = ProcessSignaled, 9 }}`

// These four non-call selectors carry descriptor-free receipt values, not a
// public Tx method. Each exception is bound to its exact file, function, use
// and independently closed declaration, never a name-only or non-call rule.
// Native declarations and the existing closed ownership/model type contracts
// fix Frame.Commit to CommitEvidence and both failure fields to CommitValue.
// Supplemental byte/shape repinning cannot change these independent decisions.
func restartGuardCommitField(path string, sel *ast.SelectorExpr, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte, file *ast.File) bool {
	if sel.Sel.Name != "Commit" || !restartGuardImportBound(file, "rollbackrestart", rollbackRestartImport) {
		return false
	}
	fn := oracleEnclosingFunction(sel, parents)
	if fn == nil {
		return false
	}
	field := rollbackFaultText(set, raw, sel)
	if path == rollbackRestartNative && file.Name.Name == "sqlite" && fn.Recv == nil {
		// These unchanged independent declarations bind commit to Frame,
		// detail to manualRestartFailureRecord, and observation to the
		// Observation returned by sealing a copied concrete Receipt.
		if fn.Name.Name != "manualRestartAck" && fn.Name.Name != "manualRestartFailureSnapshot" {
			return false
		}
		expected, ok := restartGuardLaunchInputDeclaration(fn.Name.Name)
		if !ok || restartGuardTokens(oracleNodeText(set, raw, fn)) != restartGuardTokens(expected) {
			return false
		}
		if fn.Name.Name == "manualRestartAck" && field == "commit.Commit" {
			value, ok := parents[sel].(*ast.SelectorExpr)
			if !ok || value.X != sel || value.Sel.Name != "Value" {
				return false
			}
			comparison, ok := parents[value].(*ast.BinaryExpr)
			return ok && comparison.X == value && comparison.Op == token.NEQ && rollbackFaultText(set, raw, comparison.Y) == "rollbackrestart.CommitNil"
		}
		if fn.Name.Name == "manualRestartFailureSnapshot" {
			assignment, ok := parents[sel].(*ast.AssignStmt)
			if !ok || assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 2 || len(assignment.Rhs) != 2 || rollbackFaultText(set, raw, assignment) != "detail.Commit, detail.ProtocolProblem = observation.Commit, observation.Problem" {
				return false
			}
			return field == "detail.Commit" && assignment.Lhs[0] == sel || field == "observation.Commit" && assignment.Rhs[0] == sel
		}
		return false
	}
	if path != rollbackRestartBridge || file.Name.Name != "pager" || fn.Name.Name != "CommitReturned" || field != "frame.Commit" {
		return false
	}
	assignment, ok := parents[sel].(*ast.AssignStmt)
	if !ok || assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 || assignment.Lhs[0] != sel || rollbackFaultText(set, raw, assignment) != "frame.Commit = captured.Evidence" {
		return false
	}
	// Fix the concrete channel receiver, captured scalar record parameter,
	// fresh Frame origin and immediate assignment before hashing/sending.
	// No alias, shadow, method value, call, duplicate or relocated use matches.
	if restartGuardTokens(oracleNodeText(set, raw, fn)) != restartGuardTokens(restartGuardCommitReturnedDeclaration) {
		return false
	}
	frames := 0
	for _, declaration := range file.Decls {
		factory, ok := declaration.(*ast.FuncDecl)
		if !ok || factory.Name.Name != "frame" {
			continue
		}
		frames++
		if restartGuardTokens(oracleNodeText(set, raw, factory)) != restartGuardTokens(`func (c *ManualRollbackRestartChannel) frame(kind rollbackrestart.FrameKind) rollbackrestart.Frame {
	return rollbackrestart.Frame{Schema: rollbackrestart.SchemaVersion, Binding: c.plan.Spec().Binding, Sender: c.role, Sequence: c.sequence + 1, Kind: kind}
}`) {
			return false
		}
	}
	return frames == 1
}

const restartGuardCommitReturnedDeclaration = `func (c *ManualRollbackRestartChannel) CommitReturned(captured ManualRollbackRestartCommit) error {
	if !c.ready || c.role.Role != rollbackrestart.Writer || captured.Problem != nil || captured.CommitError != nil || captured.Evidence.Value != rollbackrestart.CommitNil {
		return errRollbackFaultScope
	}
	frame := c.frame(rollbackrestart.PublicCommitReturned)
	frame.Commit = captured.Evidence
	commitHash, problem := rollbackrestart.FrameDigest(frame)
	if problem != rollbackrestart.ProblemNone {
		return errRollbackFaultScope
	}
	if err := c.send(frame); err != nil {
		return err
	}
	grant, err := c.receive()
	if err != nil {
		return err
	}
	if c.plan.Spec().Binding.Case == rollbackrestart.Control {
		if grant.Kind != rollbackrestart.NormalFinishGranted || grant.Digest != commitHash {
			return errRollbackFaultScope
		}
		return nil
	}
	if c.plan.Spec().Binding.Case != rollbackrestart.AcknowledgedExit || grant.Kind != rollbackrestart.AcknowledgementDurable || grant.Ack.CommitSequence != frame.Sequence || grant.Ack.CommitFrameSHA256 != commitHash {
		return errRollbackFaultScope
	}
	ack := grant.Ack
	if ack.SHA256 == "" || !ack.Exclusive || ack.Bytes <= 0 || ack.Written != ack.Bytes || ack.WriteStep != 1 || ack.FileSyncStep != 2 || ack.FileCloseStep != 3 || ack.DirectorySyncStep != 4 || ack.DirectoryCloseStep != 5 || ack.FileCloseCalls != 1 || ack.DirectoryCloseCalls != 1 || ack.ErrorToken != 0 {
		return errRollbackFaultScope
	}
	exit, err := c.receive()
	if err != nil || exit.Kind != rollbackrestart.ExitGranted || exit.Ack != ack || exit.Digest != ack.SHA256 {
		return errRollbackFaultScope
	}
	os.Exit(86)
	return errRollbackFaultScope
}`

// Every raw I/O/read/write/deadline reference is a direct call at one named
// consumer. Counts reject duplicate calls even if supplemental snapshots are
// both repinned. Higher-level artifact I/O remains in the old exact SQL ledger.
func restartGuardIOSelector(name string) bool {
	switch name {
	case "create", "close", "reserve", "Begin", "Exec", "Read", "Write", "ReadAt", "WriteAt", "ReadFile", "WriteFile", "ReadFull", "ReadAll", "ReadFrom", "WriteTo", "ReadByte", "ReadRune", "ReadString", "WriteString", "WriteByte", "WriteRune", "Copy", "CopyN", "CopyBuffer", "LimitReader", "Seek", "Stat", "Lstat", "Statfs", "Fstat", "SameFile", "SetDeadline", "SetReadDeadline", "SetWriteDeadline", "Fprint", "Fprintf", "Fprintln", "Print", "Printf", "Println":
		return true
	}
	return false
}

var restartGuardIOCalls = map[string]int{
	rollbackRestartNative + `|manualRestartCreate|c.family[family].create(name, raw, false)`:                                          1,
	rollbackRestartNative + `|manualRestartImage|c.sql.ledger.reserve()`:                                                              1,
	rollbackRestartNative + `|manualRestartImage|c.sql.ledger.close(i)`:                                                               3,
	rollbackRestartNative + `|manualRestartFamilies|c.sql.ledger.reserve()`:                                                           1,
	rollbackRestartNative + `|manualRestartFamilies|c.sql.ledger.close(owner)`:                                                        3,
	rollbackRestartNative + `|manualRestartSyncDirectory|c.sql.ledger.reserve()`:                                                      1,
	rollbackRestartNative + `|manualRestartSyncDirectory|c.sql.ledger.close(i)`:                                                       3,
	rollbackRestartNative + `|manualRestartPipe|c.sql.ledger.reserve()`:                                                               2,
	rollbackRestartNative + `|manualRestartSpawn|c.sql.ledger.close(controlReadOwner)`:                                                1,
	rollbackRestartNative + `|manualRestartSpawn|c.sql.ledger.close(eventWriteOwner)`:                                                 1,
	rollbackRestartNative + `|manualRestartWaitChild|c.sql.ledger.close(child.eventOwner)`:                                            1,
	rollbackRestartNative + `|manualRestartWaitChild|c.sql.ledger.close(child.controlOwner)`:                                          1,
	rollbackRestartNative + `|manualRestartWriter|c.sql.ledger.close(owner)`:                                                          1,
	rollbackRestartNative + `|manualRestartWriter|db.Begin(ctx, TxOptions{Immediate: true})`:                                          1,
	rollbackRestartNative + `|manualRestartWriter|tx.Exec(ctx, statement)`:                                                            1,
	rollbackRestartBridge + `|NewManualRollbackRestartChannel|channel.control.Stat()`:                                                 1,
	rollbackRestartBridge + `|NewManualRollbackRestartChannel|channel.events.Stat()`:                                                  1,
	rollbackRestartBridge + `|NewManualRollbackRestartChannel|vfs.SameFile(controlInfo, eventInfo)`:                                   1,
	rollbackRestartBridge + `|send|c.events.Write(raw)`:                                                                               1,
	rollbackRestartBridge + `|receive|io.ReadFull(c.control, size[:])`:                                                                1,
	rollbackRestartBridge + `|receive|io.ReadFull(c.control, raw[4:])`:                                                                1,
	rollbackRestartBridge + `|rollbackRestartDiagnostic|fmt.Fprintln(os.Stderr, "ADAPTER_PROBLEM:", problem)`:                         1,
	rollbackRestartBridge + `|rollbackRestartDiagnostic|fmt.Fprintf(os.Stderr, "ORIGINAL_ERROR_TOKEN=%d text=%s\n", ordinal+1, text)`: 1,
	rollbackRestartBridge + `|rollbackRestartTransport|fmt.Fprintln(os.Stderr, "ORIGINAL_TRANSPORT_ERROR:", text)`:                    1,
	rollbackRestartNative + `|manualRestartRead|os.Lstat(current)`:                                                                    1,
	rollbackRestartNative + `|manualRestartImage|image.Stat()`:                                                                        2,
	rollbackRestartNative + `|manualRestartImage|io.ReadAll(io.LimitReader(image, (128<<20)+1))`:                                      1,
	rollbackRestartNative + `|manualRestartImage|io.LimitReader(image, (128<<20)+1)`:                                                  1,
	rollbackRestartNative + `|manualRestartImage|os.SameFile(before, after)`:                                                          1,
	rollbackRestartNative + `|manualRestartImage|os.SameFile(before, targetInfo)`:                                                     1,
	rollbackRestartNative + `|manualRestartAdmit|syscall.Statfs(c.root, &filesystem)`:                                                 1,
	rollbackRestartNative + `|manualRestartAdmit|rand.Read(nonce[:])`:                                                                 1,
	rollbackRestartNative + `|manualRestartFamilies|directory.Stat()`:                                                                 1,
	rollbackRestartNative + `|manualRestartFamilies|os.SameFile(actual, c.sql.rootInfo)`:                                              1,
	rollbackRestartNative + `|manualRestartSyncDirectory|dir.Stat()`:                                                                  1,
	rollbackRestartNative + `|manualRestartSyncDirectory|os.SameFile(actual, f.rootInfo)`:                                             1,
	rollbackRestartNative + `|manualRestartSpawn|child.events.SetReadDeadline(time.Now().Add(manualRestartWait))`:                     1,
	rollbackRestartNative + `|manualRestartSpawn|child.controls.SetWriteDeadline(time.Now().Add(manualRestartWait))`:                  1,
	rollbackRestartNative + `|manualRestartReadFrame|io.ReadFull(child.events, header[:])`:                                            1,
	rollbackRestartNative + `|manualRestartReadFrame|io.ReadFull(child.events, raw[4:])`:                                              1,
	rollbackRestartNative + `|manualRestartSendFrame|child.controls.Write(raw)`:                                                       1,
	rollbackRestartNative + `|manualRestartWaitChild|child.events.SetReadDeadline(time.Now().Add(manualRestartWait))`:                 1,
	rollbackRestartNative + `|manualRestartWaitChild|child.events.Read(trailing[:])`:                                                  1,
	rollbackRestartNative + `|manualRestartFailure|child.controls.Write(raw)`:                                                         1,
}

// Reviewed callee bodies do not grant arbitrary new callers. In particular,
// a trial-family directory Sync cannot be inserted through an old helper.
func restartGuardEffectHelper(name string) bool {
	switch name {
	case "manualRestartRead", "manualRestartImage", "manualRestartFamilies", "manualRestartSyncDirectory", "manualRestartCreate", "manualRestartArtifactAt", "manualRestartRetain", "manualRestartUnchanged", "manualRestartPipe":
		return true
	}
	return false
}

var restartGuardEffectCalls = map[string]int{
	rollbackRestartNative + `|manualRestartImage|manualRestartRead(c, c.binding.BinaryPath, 128<<20)`:                                    1,
	rollbackRestartNative + `|manualRestartAdmit|manualRestartRead(c, os.Getenv("GOSQLITE_RESTART_BINDING"), 65536)`:                     1,
	rollbackRestartNative + `|manualRestartAdmit|manualRestartRead(c, c.binding.SourceManifestPath, 4<<20)`:                              1,
	rollbackRestartNative + `|manualRestartAdmit|manualRestartRead(c, c.binding.CompileReceiptPath, 65536)`:                              1,
	rollbackRestartNative + `|manualRestartAdmit|manualRestartRead(c, c.binding.ToolPath, 128<<20)`:                                      1,
	rollbackRestartNative + `|manualRestartAdmit|manualRestartImage(c)`:                                                                  1,
	rollbackRestartNative + `|manualRestartAdmit|manualRestartRead(c, filepath.Join(c.root, "control", "plan.json"), 65536)`:             1,
	rollbackRestartNative + `|manualRestartCreate|manualRestartSyncDirectory(c, family)`:                                                 1,
	rollbackRestartNative + `|manualRestartCreate|manualRestartRetain(c, "control/"+name, 1<<20)`:                                        1,
	rollbackRestartNative + `|manualRestartArtifactAt|manualRestartRead(c, path, bound)`:                                                 1,
	rollbackRestartNative + `|manualRestartRetain|manualRestartArtifactAt(c, relative, bound)`:                                           1,
	rollbackRestartNative + `|manualRestartUnchanged|manualRestartArtifactAt(c, previous.Path, 1<<20)`:                                   1,
	rollbackRestartNative + `|manualRestartSpawn|manualRestartUnchanged(c)`:                                                              1,
	rollbackRestartNative + `|manualRestartSpawn|manualRestartImage(c)`:                                                                  1,
	rollbackRestartNative + `|manualRestartSpawn|manualRestartRead(c, filepath.Join(c.root, "control", "plan.json"), 65536)`:             1,
	rollbackRestartNative + `|manualRestartSpawn|manualRestartPipe(c)`:                                                                   2,
	rollbackRestartNative + `|manualRestartWaitChild|manualRestartCreate(c, 4, name+"-stdout.json", stdoutJSON)`:                         1,
	rollbackRestartNative + `|manualRestartWaitChild|manualRestartCreate(c, 4, name+"-stderr.json", stderrJSON)`:                         1,
	rollbackRestartNative + `|manualRestartWaitChild|manualRestartCreate(c, 4, name+"-protocol.json", transcript)`:                       1,
	rollbackRestartNative + `|manualRestartAck|manualRestartCreate(c, 4, "ack.json", raw)`:                                               1,
	rollbackRestartNative + `|manualRestartAck|manualRestartArtifactAt(c, "control/ack.json", rollbackrestart.MaxFrameBytes+4)`:          1,
	rollbackRestartNative + `|manualRestartAck|manualRestartRetain(c, "control/ack.json", rollbackrestart.MaxFrameBytes+4)`:              1,
	rollbackRestartNative + `|manualRestartRecovery|manualRestartArtifactAt(c, "replay/replay.db-journal", 16408)`:                       2,
	rollbackRestartNative + `|manualRestartRecovery|manualRestartCreate(c, 4, "readback.json", raw)`:                                     1,
	rollbackRestartNative + `|manualRestartArtifactEvidence|manualRestartArtifactAt(c, rollbackrestart.ArtifactName(role), 1<<20)`:       1,
	rollbackRestartNative + `|manualRestartFreeze|manualRestartArtifactAt(c, "trial/trial.db", 12288)`:                                   1,
	rollbackRestartNative + `|manualRestartFreeze|manualRestartArtifactAt(c, "trial/trial.db-journal", 16408)`:                           1,
	rollbackRestartNative + `|manualRestartFreeze|manualRestartRetain(c, "trial/trial.db", 12288)`:                                       1,
	rollbackRestartNative + `|manualRestartFreeze|manualRestartRetain(c, "trial/trial.db-journal", 16408)`:                               1,
	rollbackRestartNative + `|manualRestartFreeze|manualRestartCreate(c, 2, "post-exit.input.db", raw)`:                                  1,
	rollbackRestartNative + `|manualRestartFreeze|manualRestartCreate(c, 3, "replay.db", raw)`:                                           1,
	rollbackRestartNative + `|manualRestartFreeze|manualRestartRetain(c, "evidence/post-exit.input.db", 12288)`:                          1,
	rollbackRestartNative + `|manualRestartFreeze|manualRestartCreate(c, 2, "post-exit-journal.input.bin", journalRaw)`:                  1,
	rollbackRestartNative + `|manualRestartFreeze|manualRestartCreate(c, 3, "replay.db-journal", journalRaw)`:                            1,
	rollbackRestartNative + `|manualRestartFreeze|manualRestartRetain(c, "evidence/post-exit-journal.input.bin", 16408)`:                 1,
	rollbackRestartNative + `|manualRestartFreeze|manualRestartUnchanged(c)`:                                                             1,
	rollbackRestartNative + `|manualRestartDriveRecovery|manualRestartRead(c, filepath.Join(c.root, "control", "readback.json"), 65536)`: 1,
	rollbackRestartNative + `|manualRestartDriveRecovery|manualRestartRetain(c, "control/readback.json", 65536)`:                         1,
	rollbackRestartNative + `|manualRestartDriveRecovery|manualRestartUnchanged(c)`:                                                      1,
	rollbackRestartNative + `|manualRestartController|manualRestartFamilies(c, true)`:                                                    1,
	rollbackRestartNative + `|manualRestartController|manualRestartCreate(c, 0, "binding.input.json", c.rawBinding)`:                     1,
	rollbackRestartNative + `|manualRestartController|manualRestartCreate(c, 4, "plan.json", planRaw)`:                                   1,
	rollbackRestartNative + `|manualRestartController|manualRestartRetain(c, "inputs/binding.input.json", 65536)`:                        1,
	rollbackRestartNative + `|manualRestartController|manualRestartRetain(c, "control/plan.json", 65536)`:                                1,
	rollbackRestartNative + `|manualRestartController|manualRestartCreate(c, 0, "baseline.input.db", c.baseline[:])`:                     1,
	rollbackRestartNative + `|manualRestartController|manualRestartRetain(c, "inputs/baseline.input.db", 12288)`:                         1,
	rollbackRestartNative + `|manualRestartController|manualRestartCreate(c, 4, "baseline-sql.json", beforeRaw)`:                         1,
	rollbackRestartNative + `|manualRestartController|manualRestartRetain(c, "control/baseline-sql.json", 65536)`:                        1,
	rollbackRestartNative + `|manualRestartController|manualRestartCreate(c, 1, "trial.db", c.baseline[:])`:                              1,
	rollbackRestartNative + `|manualRestartController|manualRestartCreate(c, 4, "result.json", raw)`:                                     1,
	rollbackRestartNative + `|manualRestartController|manualRestartUnchanged(c)`:                                                         2,
	rollbackRestartNative + `|manualRestartController|manualRestartCreate(c, 4, "final-manifest.json", finalManifest)`:                   1,
	rollbackRestartNative + `|manualRestartPersistFailure|manualRestartCreate(c, 4, "failure.json", raw)`:                                1,
	rollbackRestartNative + `|manualRestartFailure|manualRestartCreate(c, 4, "failure-wait.json", raw)`:                                  1,
	rollbackRestartNative + `|manualRestartFailure|manualRestartCreate(c, 4, "failure-stdout.json", stdoutJSON)`:                         1,
	rollbackRestartNative + `|manualRestartFailure|manualRestartCreate(c, 4, "failure-stderr.json", stderrJSON)`:                         1,
	rollbackRestartNative + `|manualRestartFailure|manualRestartCreate(c, 4, "failure-protocol.json", raw)`:                              1,
	rollbackRestartNative + `|TestManualRollbackRestartWriter|manualRestartFamilies(&config, false)`:                                     1,
	rollbackRestartNative + `|TestManualRollbackRestartRecovery|manualRestartFamilies(&config, false)`:                                   1,
}

var restartGuardOwnerCalls = map[string]bool{
	rollbackRestartBridge + "|NewManualRollbackRestartChannel|os.NewFile(3, \"restart-parent-control\")": true,
	rollbackRestartBridge + "|NewManualRollbackRestartChannel|os.NewFile(4, \"restart-child-events\")":   true,
	rollbackRestartBridge + "|CloseNormal|c.control.Close()":                                             true,
	rollbackRestartBridge + "|CloseNormal|c.events.Close()":                                              true,
	rollbackRestartBridge + "|CaptureManualRollbackRestartCommit|fault.main.RetirementComplete()":        true,
	rollbackRestartBridge + "|CaptureManualRollbackRestartCommit|fault.journal.RetirementComplete()":     true,
	rollbackRestartNative + "|manualRestartImage|os.Open(\"/proc/self/exe\")":                            true,
	rollbackRestartNative + "|manualRestartFamilies|os.Open(c.root)":                                     true,
	rollbackRestartNative + "|manualRestartFamilies|directory.ReadDir(1)":                                true,
	rollbackRestartNative + "|manualRestartFamilies|os.Mkdir(path, 0700)":                                true,
	rollbackRestartNative + "|manualRestartSyncDirectory|os.Open(f.root)":                                true,
	rollbackRestartNative + "|manualRestartSyncDirectory|dir.Sync()":                                     true,
	rollbackRestartNative + "|manualRestartPipe|os.Pipe()":                                               true,
}

var restartGuardAPICalls = map[string]bool{
	rollbackRestartNative + "|TestManualRollbackRestartWriter|pager.NewManualRollbackRestartChannel(config.plan, rollbackrestart.Writer)":           true,
	rollbackRestartNative + "|TestManualRollbackRestartRecovery|pager.NewManualRollbackRestartChannel(config.plan, rollbackrestart.Recovery)":       true,
	rollbackRestartNative + "|manualRestartWriter|pager.ArmManualRollbackRestart(tx.storage, c.plan, channel)":                                      true,
	rollbackRestartNative + "|manualRestartWriter|pager.CaptureManualRollbackRestartCommit(scope, commitErr, tx.done, db.tx == nil, tx.activeRows)": true,
	rollbackRestartNative + "|manualRestartWriter|pager.CompleteManualRollbackRestartControl(scope)":                                                true,
}

func restartGuardEnvReference(path, name, call string) bool {
	if path == rollbackRestartNative {
		if restartGuardRoot(name) {
			return call == `os.Getenv("GOSQLITE_MANUAL_RESTART_ROLLBACK")`
		}
		if name != "manualRestartAdmit" {
			return false
		}
		switch call {
		case `os.Getenv("GOSQLITE_MANUAL_RESTART_ROLLBACK")`, `os.Getenv("GOSQLITE_RESTART_PRIVATE_ROOT_CONFIRMED")`, `os.Getenv("GOSQLITE_RESTART_ROLE")`, `os.Getenv("GOSQLITE_RESTART_ROOT")`, `os.Getenv("GOSQLITE_RESTART_BINDING")`, `os.Getenv("GOSQLITE_RESTART_BINDING_SHA256")`, `os.Getenv("GOSQLITE_RESTART_PLAN_SHA256")`:
			return true
		}
	}
	if path == rollbackRestartBridge {
		if name == "ArmManualRollbackRestart" {
			return call == `os.Getenv("GOSQLITE_MANUAL_RESTART_ROLLBACK")`
		}
		if name == "NewManualRollbackRestartChannel" {
			return call == `os.Getenv("GOSQLITE_MANUAL_RESTART_ROLLBACK")` || call == `os.Getenv("GOSQLITE_RESTART_ROLE")` || call == `os.Getenv("GOSQLITE_RESTART_SEQUENCE")`
		}
	}
	return false
}

// This is the sole narrow addition to the older startup guard. The two prior
// CLI rules are unchanged. Matching a name/tag is insufficient: the complete
// native bytes, declarations and independent role/argv semantics must all pass.
func restartGuardProcessReference(path string, selector *ast.SelectorExpr, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte) bool {
	if filepath.ToSlash(filepath.Clean(path)) != rollbackRestartNative || selector.Sel.Name != "Command" || strings.SplitN(string(raw), "\n", 2)[0] != rollbackRestartTag {
		return false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok || qualifier.Name != "exec" {
		return false
	}
	if restartGuardCallKey(rollbackRestartNative, selector.Sel, parents, set, raw) != rollbackRestartNative+"|manualRestartSpawn|exec.Command(c.binding.BinaryPath, args...)" {
		return false
	}
	var file *ast.File
	for node := ast.Node(selector); node != nil; node = parents[node] {
		if found, ok := node.(*ast.File); ok {
			file = found
			break
		}
	}
	if file == nil || !restartGuardImportBound(file, "exec", "os/exec") || readOnlyWALManualCountCall(file, "exec.Command") != 1 {
		return false
	}
	findings, err := inspectRollbackRestartSource(rollbackRestartNative, raw)
	return err == nil && len(findings) == 0
}

func restartGuardWrapperReference(path string, id *ast.Ident, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte) bool {
	if path != rollbackFaultManual {
		return false
	}
	key := restartGuardCallKey(path, id, parents, set, raw)
	return key == rollbackFaultManual+"|rollbackFaultBefore|rollbackRestartBefore(ticket)" || key == rollbackFaultManual+"|rollbackFaultAfter|rollbackRestartAfter(ticket)"
}

func restartGuardWaitClosure(path string, node ast.Node, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte) bool {
	if path != rollbackRestartNative {
		return false
	}
	var goroutine *ast.GoStmt
	for current := node; current != nil; current = parents[current] {
		if found, ok := current.(*ast.GoStmt); ok {
			goroutine = found
		}
		if fn, ok := current.(*ast.FuncDecl); ok {
			if goroutine == nil || fn.Name.Name != "manualRestartSpawn" || fn.Recv != nil {
				return false
			}
			want := `go func() { waitErr := command.Wait(); child.wait <- manualRestartWaitResult{state: command.ProcessState, err: waitErr} }()`
			return restartGuardTokens(oracleNodeText(set, raw, goroutine)) == restartGuardTokens(want)
		}
	}
	return false
}

func restartGuardStubSemantics(path string, raw []byte, set *token.FileSet, file *ast.File) []string {
	var findings []string
	report := func(message string) { findings = append(findings, path+": rollback restart contract: "+message) }
	if len(file.Imports) != 0 || len(file.Decls) != 2 {
		report("fault-only stub must be exactly two import-free empty observers")
	}
	counts := map[string]int{}
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok {
			report("fault-only stub cannot contain declarations or state")
			continue
		}
		counts[fn.Name.Name]++
		want := "func " + fn.Name.Name + "(ticket rollbackFaultTicket) {}"
		if (fn.Name.Name != "rollbackRestartBefore" && fn.Name.Name != "rollbackRestartAfter") || restartGuardTokens(oracleNodeText(set, raw, fn)) != restartGuardTokens(want) {
			report("fault-only stub must be exactly two import-free empty observers")
		}
	}
	if counts["rollbackRestartBefore"] != 1 || counts["rollbackRestartAfter"] != 1 {
		report("fault-only stub observer inventory changed")
	}
	return findings
}

// The original core statements remain separately frozen. Wrapper completion
// proves its core's deferred private unlock has run before observer entry.
func restartGuardAdapterSemantics(path string, raw []byte, set *token.FileSet, file *ast.File, functions map[string]*ast.FuncDecl) []string {
	var findings []string
	for _, entry := range []struct{ name, body string }{
		{"rollbackFaultBefore", `{ ticket := rollbackFaultBeforeCore(t, point, f, path, off, length); rollbackRestartBefore(ticket); return ticket }`},
		{"rollbackFaultAfter", `{ rollbackFaultAfterCore(ticket, executed, nativeN, nativeErr, returnedN, returnedErr, normalizedErr, effect); rollbackRestartAfter(ticket) }`},
	} {
		findings = append(findings, restartGuardExactBody(path, functions, set, raw, entry.name, entry.body, "original core must finish before fixed observer outside private mutex")...)
	}
	for name, body := range restartGuardAdapterCoreBodies {
		findings = append(findings, restartGuardExactBody(path, functions, set, raw, name, body, "original adapter core body must remain unchanged")...)
	}
	return findings
}

var restartGuardAdapterCoreBodies = map[string]string{
	"rollbackFaultBeforeCore": "{\n\ts := rollbackFaultFor(t)\n\tif s == nil {\n\t\treturn rollbackFaultTicket{action: rollbackfault.Pass}\n\t}\n\ts.mu.Lock()\n\tdefer s.mu.Unlock()\n\tp, index := rollbackFaultPoint(point)\n\tif index < 0 || s.finalized || s.inFlight || s.counts[index] >= 4 {\n\t\ts.problem = errRollbackFaultScope\n\t\treturn rollbackFaultTicket{action: rollbackfault.Pass}\n\t}\n\ts.counts[index]++\n\tspec := s.plan.Spec()\n\tcall := rollbackfault.Call{Point: p, Occurrence: s.counts[index], Offset: off, Length: length}\n\tvalid := t.file == s.main && t.cleanup == s.cleanup && t.session == nil && t.shared == nil && t.encrypted == nil && !t.keepFile && t.borrowedCleanup == nil\n\tswitch point {\n\tcase rbDBOriginalRead, rbDBPage, rbDBSync:\n\t\tcall.Role, call.Owner = rollbackfault.Database, spec.MainOwner\n\t\tvalid = valid && f == s.main && path == \"\"\n\tcase rbPublishDirSync, rbFinalDirSync, rbCleanupDirSync:\n\t\tcall.Role, call.Owner = rollbackfault.Directory, spec.DirectoryOwner\n\t\tvalid = valid && f == nil && path == t.path\n\tcase rbJCreate, rbJUnlink, rbCleanupUnlink:\n\t\tcall.Role, call.Owner = rollbackfault.Journal, spec.JournalOwner\n\t\tvalid = valid && f == nil && path == t.path+\"-journal\"\n\tdefault:\n\t\tcall.Role, call.Owner = rollbackfault.Journal, spec.JournalOwner\n\t\tvalid = valid && f != nil && f == s.journal && path == \"\"\n\t}\n\tif !valid {\n\t\ts.problem = errRollbackFaultScope\n\t}\n\taction := rollbackfault.Pass\n\tif valid && !s.firstError && s.problem == nil && call == spec.Target && point != rbCleanupUnlink && point != rbCleanupDirSync {\n\t\taction = spec.Action\n\t}\n\ts.inFlight = true\n\ts.append(rollbackfault.Event{Kind: rollbackfault.Intent, Call: call, Action: action})\n\tif s.problem != nil {\n\t\taction = rollbackfault.Pass\n\t}\n\treturn rollbackFaultTicket{scope: s, call: call, action: action}\n}",
	"rollbackFaultAfterCore":  "{\n\ts := ticket.scope\n\tif s == nil {\n\t\treturn\n\t}\n\ts.mu.Lock()\n\tdefer s.mu.Unlock()\n\tresult := rollbackfault.Result{Executed: executed, NativeN: nativeN, ReturnedN: returnedN, Effect: effect}\n\tif nativeErr != nil {\n\t\tresult.NativeError = rollbackfault.Underlying\n\t\tresult.NativeErrorToken = s.diagnostic(nativeErr)\n\t}\n\tif returnedErr != nil {\n\t\tswitch {\n\t\tcase nativeErr != nil && returnedErr == nativeErr:\n\t\t\tresult.ReturnedError, result.ReturnedErrorToken = rollbackfault.Underlying, result.NativeErrorToken\n\t\tcase returnedErr == errRollbackFaultInjected:\n\t\t\tresult.ReturnedError = rollbackfault.Injected\n\t\tdefault:\n\t\t\ts.problem = errRollbackFaultScope\n\t\t}\n\t}\n\tif normalizedErr != nil {\n\t\tswitch {\n\t\tcase nativeErr != nil && normalizedErr == nativeErr:\n\t\t\tresult.NormalizedError, result.NormalizedErrorToken = rollbackfault.Underlying, result.NativeErrorToken\n\t\tcase normalizedErr == errRollbackFaultInjected:\n\t\t\tresult.NormalizedError = rollbackfault.Injected\n\t\tcase normalizedErr == io.ErrShortWrite:\n\t\t\tresult.NormalizedError = rollbackfault.ShortWrite\n\t\tdefault:\n\t\t\ts.problem = errRollbackFaultScope\n\t\t}\n\t}\n\tif ticket.call == s.plan.Spec().Target {\n\t\ts.targetError = normalizedErr\n\t\ts.targetErrorToken = result.NormalizedErrorToken\n\t}\n\tif normalizedErr != nil || returnedErr != nil || nativeErr != nil {\n\t\ts.firstError = true\n\t}\n\ts.inFlight = false\n\ts.append(rollbackfault.Event{Kind: rollbackfault.Returned, Call: ticket.call, Action: ticket.action, Result: result})\n}",
}

// Native/model decisions below are independent of supplemental whole-source
// snapshots. Their finite inventories will remain review-held with this change.
func restartGuardRequire(path string, raw []byte, set *token.FileSet, functions map[string]*ast.FuncDecl, name, diagnostic string, parts ...string) []string {
	fn := functions[name]
	if fn == nil || fn.Body == nil {
		return []string{path + ": rollback restart contract: " + diagnostic + ": missing " + name}
	}
	actual := restartGuardTokens(oracleNodeText(set, raw, fn.Body))
	for _, part := range parts {
		// Independent token decisions, never comments or whitespace-sensitive
		// string erasure. Dominating gates/order use structural sequences too.
		if !strings.Contains(actual, restartGuardTokens(part)) {
			return []string{path + ": rollback restart contract: " + diagnostic + ": missing decision in " + name}
		}
	}
	return nil
}
func restartGuardFunctionInventory(path string, file *ast.File, names string) []string {
	wanted := map[string]bool{}
	for _, name := range strings.Fields(names) {
		wanted[name] = true
	}
	counts := map[string]int{}
	var findings []string
	for _, declaration := range file.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok {
			counts[fn.Name.Name]++
			if !wanted[fn.Name.Name] {
				findings = append(findings, path+": rollback restart contract: closed function inventory has extra "+fn.Name.Name)
			}
		}
	}
	for name := range wanted {
		if counts[name] != 1 {
			findings = append(findings, path+": rollback restart contract: closed function inventory missing/duplicate "+name)
		}
	}
	return findings
}

func restartGuardBridgeSemantics(path string, raw []byte, set *token.FileSet, file *ast.File, functions map[string]*ast.FuncDecl) []string {
	findings := restartGuardFunctionInventory(path, file, "NewManualRollbackRestartChannel frame send receive Ready ArmManualRollbackRestart rollbackRestartBefore rollbackRestartAfter rollbackRestartObserve CaptureManualRollbackRestartCommit CommitReturned CompleteManualRollbackRestartControl Finished CloseNormal rollbackRestartDiagnostic rollbackRestartTransport rollbackRestartAbort")
	report := func(message string) { findings = append(findings, path+": rollback restart contract: "+message) }
	require := func(name, diagnostic string, parts ...string) {
		findings = append(findings, restartGuardRequire(path, raw, set, functions, name, diagnostic, parts...)...)
	}
	typeBodies := map[string]string{
		"ManualRollbackRestartChannel": `ManualRollbackRestartChannel struct { plan rollbackrestart.Plan; role rollbackrestart.RoleIdentity; sequence uint8; control *os.File; events *os.File; controlClose uint8; eventsClose uint8; problem error; frameCount uint8; ready bool; complete bool }`,
		"manualRollbackRestartScope":   `manualRollbackRestartScope struct { fault *rollbackFaultScope; channel *ManualRollbackRestartChannel; count uint8; target bool; commitCaptured bool; commitError error }`,
		"ManualRollbackRestartCommit":  `ManualRollbackRestartCommit struct { Evidence rollbackrestart.CommitEvidence; CommitError error; Problem error }`,
	}
	stateCount := 0
	for _, declaration := range file.Decls {
		if gen, ok := declaration.(*ast.GenDecl); ok {
			if gen.Tok == token.VAR {
				stateCount++
				if restartGuardTokens(oracleNodeText(set, raw, gen)) != restartGuardTokens(`var rollbackRestartRegistry struct { mu sync.Mutex; used bool; active *manualRollbackRestartScope }`) {
					report("bridge package state must be one lifetime concrete registry")
				}
			}
			for _, spec := range gen.Specs {
				if typ, ok := spec.(*ast.TypeSpec); ok {
					expected, exists := typeBodies[typ.Name.Name]
					if !exists || restartGuardTokens(oracleNodeText(set, raw, typ)) != restartGuardTokens(expected) {
						report("bridge closed ownership type changed or callback/descriptor added")
					}
				}
			}
		}
	}
	if stateCount != 1 {
		report("bridge package state must be one lifetime concrete registry")
	}
	gate := `if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" || os.Getenv("GOSQLITE_MANUAL_RESTART_ROLLBACK") != "independently_authorized" { return nil, errRollbackFaultGate }`
	for _, name := range []string{"NewManualRollbackRestartChannel", "ArmManualRollbackRestart"} {
		if restartGuardSequence(readOnlyWALManualBody(functions, name), set, raw, gate) != 0 {
			report("bridge first action must reject both exact independent environment gates")
		}
	}
	require("NewManualRollbackRestartChannel", "fixed child role and two concrete inherited pipe owners", `verified, problem := rollbackrestart.NewPlan(spec)`, `if problem != rollbackrestart.ProblemNone || verified.Spec() != spec { return nil, errRollbackFaultScope }`, `case rollbackrestart.Writer: identity = spec.Writer`, `case rollbackrestart.Recovery: identity = spec.Recovery`, `default: return nil, errRollbackFaultScope`, `if os.Getenv("GOSQLITE_RESTART_ROLE") != fmt.Sprint(uint8(role)) { return nil, errRollbackFaultScope }`, `sequence, err := strconv.ParseUint(os.Getenv("GOSQLITE_RESTART_SEQUENCE"), 10, 8)`, `if err != nil || sequence == 0 || sequence >= rollbackrestart.MaxFrames { return nil, errRollbackFaultScope }`, `channel.control = os.NewFile(3, "restart-parent-control")`, `channel.events = os.NewFile(4, "restart-child-events")`, `if channel.control == nil || channel.events == nil { return channel, errRollbackFaultScope }`, `if controlErr != nil || eventErr != nil || controlInfo.Mode()&os.ModeNamedPipe == 0 || eventInfo.Mode()&os.ModeNamedPipe == 0 || vfs.SameFile(controlInfo, eventInfo) { return channel, errors.Join(errRollbackFaultScope, controlErr, eventErr) }`)
	require("send", "bounded sequenced exact full-frame write", `if c.problem != nil || c.complete || c.frameCount >= rollbackrestart.MaxFrames || frame.Sequence != c.sequence+1 || frame.Sender != c.role || frame.Binding != c.plan.Spec().Binding { return errRollbackFaultScope }`, `raw, problem := rollbackrestart.EncodeFrame(frame)`, `n, err := c.events.Write(raw)`, `if err == nil && n != len(raw) { err = io.ErrShortWrite }`, `if err != nil { c.problem = err; return err }`, `c.sequence, c.frameCount = frame.Sequence, c.frameCount+1`)
	require("receive", "bounded complete length-prefix read and strict parent identity", `if c.problem != nil || c.complete || c.frameCount >= rollbackrestart.MaxFrames { return rollbackrestart.Frame{}, errRollbackFaultScope }`, `var size [4]byte`, `if _, err := io.ReadFull(c.control, size[:]); err != nil { c.problem = err; return rollbackrestart.Frame{}, err }`, `count := binary.BigEndian.Uint32(size[:])`, `if count == 0 || count > rollbackrestart.MaxFrameBytes { c.problem = errRollbackFaultScope; return rollbackrestart.Frame{}, c.problem }`, `raw := make([]byte, 4+int(count))`, `copy(raw, size[:])`, `if _, err := io.ReadFull(c.control, raw[4:]); err != nil { c.problem = err; return rollbackrestart.Frame{}, err }`, `frame, problem := rollbackrestart.DecodeFrame(raw)`, `if problem != rollbackrestart.ProblemNone || frame.Binding != c.plan.Spec().Binding || frame.Sender != c.plan.Spec().Controller || frame.Sequence != c.sequence+1 { c.problem = errRollbackFaultScope; return rollbackrestart.Frame{}, c.problem }`, `c.sequence, c.frameCount = frame.Sequence, c.frameCount+1`)
	require("Ready", "child ready binds actual image and role-specific parent grant", `if c.ready || actualImage != c.plan.Spec().Image || imageHash != c.plan.Spec().Binding.Binary { return errRollbackFaultScope }`, `if c.role.Role == rollbackrestart.Recovery { kind = rollbackrestart.RecoveryReadyBound }`, `PID: int64(os.Getpid()), Handle: uint64(c.role.Role-1), Image: actualImage, ImageHash: imageHash, ActualImage: true, State: rollbackrestart.ProcessRunning`, `grant, err := c.receive()`, `if c.role.Role == rollbackrestart.Recovery { expected = rollbackrestart.RecoveryReadbackGranted }`, `if grant.Kind != expected { return errRollbackFaultScope }`, `c.ready = true`)
	require("ArmManualRollbackRestart", "same ready writer plan and fixed successful three-page SQL armer", `if channel == nil || !channel.ready || channel.role.Role != rollbackrestart.Writer || channel.plan.Spec() != plan.Spec() || tx == nil { return nil, errRollbackFaultScope }`, `faultPlan, problem := rollbackfault.SQLPlan(plan.Spec().AdapterIdentity, rollbackfault.SQLControl)`, `oldScope, err := ArmManualRollbackSQLFault(tx, faultPlan)`, `scope := &manualRollbackRestartScope{fault: oldScope.scope, channel: channel}`, `if rollbackRestartRegistry.used || rollbackRestartRegistry.active != nil { rollbackRestartRegistry.mu.Unlock(); return nil, errRollbackFaultScope }`, `rollbackRestartRegistry.used, rollbackRestartRegistry.active = true, scope`, `if problemErr != nil || count != 1 || events[0].Kind != rollbackfault.RunStarted { return nil, errRollbackFaultScope }`, `frame.Adapter, frame.Ticket = events[0], plan.Spec().Ticket`, `scope.count = 1`)
	for _, entry := range []struct{ name, body string }{
		{"rollbackRestartBefore", `{ rollbackRestartObserve(ticket, rollbackfault.Intent) }`},
		{"rollbackRestartAfter", `{ rollbackRestartObserve(ticket, rollbackfault.Returned) }`},
		{"rollbackRestartAbort", `{ os.Exit(87) }`},
		{"CloseNormal", `{ if c == nil || c.controlClose != 0 || c.eventsClose != 0 { return errRollbackFaultScope }; c.controlClose++; controlErr := c.control.Close(); c.eventsClose++; eventErr := c.events.Close(); return errors.Join(controlErr, eventErr) }`},
		{"CompleteManualRollbackRestartControl", `{ if scope == nil || !scope.commitCaptured || scope.commitError != nil || scope.target || scope.channel.plan.Spec().Binding.Case != rollbackrestart.Control { return errRollbackFaultScope }; report := finishRollbackFault(scope.fault, scope.commitError); if report.Problem != nil || report.CommitError != nil || report.Observation.Outcome != rollbackfault.AdapterObserved { return errRollbackFaultScope }; return disarmRollbackFault(scope.fault) }`},
	} {
		findings = append(findings, restartGuardExactBody(path, functions, set, raw, entry.name, entry.body, "closed observer/abort/normal-control lifecycle")...)
	}
	observe := readOnlyWALManualBody(functions, "rollbackRestartObserve")
	if restartGuardSequence(observe, set, raw, `rollbackRestartRegistry.mu.Lock()`, `scope := rollbackRestartRegistry.active`, `rollbackRestartRegistry.mu.Unlock()`, `if scope == nil { return }`, `if ticket.scope == nil || ticket.scope != scope.fault || ticket.action != rollbackfault.Pass || scope.target || scope.commitCaptured { rollbackRestartAbort() }`, `fault := scope.fault`, `fault.mu.Lock()`, `events, count := fault.receipt.Events()`, `problemErr := fault.problem`, `diagnostics := fault.diagnostics`, `fault.mu.Unlock()`) != 0 {
		report("observer same-ticket scalar snapshot must release both private mutexes before IPC")
	}
	require("rollbackRestartObserve", "exact one fresh event, no target after I/O failure", `if problemErr != nil || count != scope.count+1 || count == 0 { rollbackRestartDiagnostic(problemErr, diagnostics); rollbackRestartAbort() }`, `event := events[count-1]`, `if event.Kind != kind || event.Call != ticket.call || event.Identity != scope.channel.plan.Spec().AdapterIdentity || event.Action != rollbackfault.Pass { rollbackRestartAbort() }`, `frame.Adapter, frame.Ticket = event, scope.channel.plan.Spec().Ticket`, `if kind == rollbackfault.Returned && (!event.Result.Executed || event.Result.NativeError != rollbackfault.None || event.Result.ReturnedError != rollbackfault.None || event.Result.NormalizedError != rollbackfault.None || event.Result.NativeN != event.Call.Length || event.Result.ReturnedN != event.Call.Length) { rollbackRestartDiagnostic(problemErr, diagnostics); rollbackRestartAbort() }`)
	require("rollbackRestartObserve", "exact owned target phase occurrence extent before next original call", `target := (which == rollbackrestart.BeforeJCreate && kind == rollbackfault.Intent && ticket.call.Point == rollbackfault.JournalCreate && ticket.call.Occurrence == 1) || (which == rollbackrestart.AfterTablePage && kind == rollbackfault.Returned && ticket.call.Point == rollbackfault.DBPage && ticket.call.Occurrence == 2 && ticket.call.Offset == 4096 && ticket.call.Length == 4096) || (which == rollbackrestart.AfterJUnlink && kind == rollbackfault.Returned && ticket.call.Point == rollbackfault.JournalUnlink && ticket.call.Occurrence == 1)`, `if !target { return }`, `scope.target = true`, `reached.Adapter, reached.Ticket = event, scope.channel.plan.Spec().Ticket`)
	if restartGuardSequence(observe, set, raw, `targetHash, problem := rollbackrestart.FrameDigest(reached)`, `if problem != rollbackrestart.ProblemNone { rollbackRestartTransport(errRollbackFaultScope); rollbackRestartDiagnostic(problemErr, diagnostics); rollbackRestartAbort() }`, `grant, err := scope.channel.receive()`, `if err != nil || grant.Kind != rollbackrestart.ExitGranted || grant.Digest != targetHash || grant.Ack != (rollbackrestart.AckEvidence{}) { rollbackRestartTransport(errors.Join(errRollbackFaultScope, err)); rollbackRestartDiagnostic(problemErr, diagnostics); rollbackRestartAbort() }`, `os.Exit(86)`) < 0 {
		report("exit86 requires the exact target frame and bound parent EXIT, with no cleanup afterward")
	}
	require("CaptureManualRollbackRestartCommit", "adjacent unchanged public Commit nil and real normal owner retirement", `out := ManualRollbackRestartCommit{CommitError: commitErr}`, `if scope == nil || scope.target || scope.commitCaptured { out.Problem = errRollbackFaultScope; return out }`, `scope.commitCaptured, scope.commitError = true, commitErr`, `healthy := fault.problem == nil && !fault.inFlight && !fault.finalized && count == 37 && scope.count == count && fault.tx.file == nil && fault.tx.encrypted == nil`, `if !healthy || !done || !detached || activeRows != 0 || commitErr != nil || events[count-1].Kind != rollbackfault.Returned || events[count-1].Call.Point != rollbackfault.FinalDirectorySync { out.Problem = errRollbackFaultScope; return out }`, `if !fault.main.RetirementComplete() || fault.journal == nil || !fault.journal.RetirementComplete() { out.Problem = errRollbackFaultScope; return out }`, `state := ManualRollbackSQLState(nil)`, `if state.Groups != 0 || state.Pending != 0 { out.Problem = errRollbackFaultScope; return out }`, `CapturedAdjacent: true, CapturedUnchanged: true`)
	require("CommitReturned", "public nil frame precedes durable exact ack receipt", `if !c.ready || c.role.Role != rollbackrestart.Writer || captured.Problem != nil || captured.CommitError != nil || captured.Evidence.Value != rollbackrestart.CommitNil { return errRollbackFaultScope }`, `frame := c.frame(rollbackrestart.PublicCommitReturned)`, `frame.Commit = captured.Evidence`, `commitHash, problem := rollbackrestart.FrameDigest(frame)`, `if c.plan.Spec().Binding.Case == rollbackrestart.Control { if grant.Kind != rollbackrestart.NormalFinishGranted || grant.Digest != commitHash { return errRollbackFaultScope }; return nil }`, `if c.plan.Spec().Binding.Case != rollbackrestart.AcknowledgedExit || grant.Kind != rollbackrestart.AcknowledgementDurable || grant.Ack.CommitSequence != frame.Sequence || grant.Ack.CommitFrameSHA256 != commitHash { return errRollbackFaultScope }`, `if ack.SHA256 == "" || !ack.Exclusive || ack.Bytes <= 0 || ack.Written != ack.Bytes || ack.WriteStep != 1 || ack.FileSyncStep != 2 || ack.FileCloseStep != 3 || ack.DirectorySyncStep != 4 || ack.DirectoryCloseStep != 5 || ack.FileCloseCalls != 1 || ack.DirectoryCloseCalls != 1 || ack.ErrorToken != 0 { return errRollbackFaultScope }`)
	if restartGuardSequence(readOnlyWALManualBody(functions, "CommitReturned"), set, raw, `exit, err := c.receive()`, `if err != nil || exit.Kind != rollbackrestart.ExitGranted || exit.Ack != ack || exit.Digest != ack.SHA256 { return errRollbackFaultScope }`, `os.Exit(86)`, `return errRollbackFaultScope`) < 0 {
		report("acknowledged exit86 requires durable parent ack then bound EXIT")
	}
	require("Finished", "only normal control/recovery can close their protocol owners", `if c.role.Role == rollbackrestart.Recovery { kind = rollbackrestart.ReadbackResult }`, `if c.role.Role == rollbackrestart.Writer && c.plan.Spec().Binding.Case != rollbackrestart.Control { return errRollbackFaultScope }`, `if err := c.send(frame); err != nil { return err }`, `c.complete = true`, `return c.CloseNormal()`)
	require("rollbackRestartTransport", "bounded original transport failure diagnostics", `text := fmt.Sprint(err)`, `if len(text) > 384 { text = text[:384]+" [TRUNCATED_TRANSPORT_FAILURE]" }`, `fmt.Fprintln(os.Stderr, "ORIGINAL_TRANSPORT_ERROR:", text)`)
	require("rollbackRestartObserve", "transport failure preserves original error before abort", `if err := scope.channel.send(frame); err != nil { rollbackRestartTransport(err); rollbackRestartDiagnostic(problemErr, diagnostics); rollbackRestartAbort() }`, `if err := scope.channel.send(reached); err != nil { rollbackRestartTransport(err); rollbackRestartDiagnostic(problemErr, diagnostics); rollbackRestartAbort() }`)
	// Concrete native operations remain at these exact sites even when the
	// supplemental byte and declaration snapshots are deliberately repinned.
	parents := oracleParents(file)
	ast.Inspect(file, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			fn := oracleEnclosingFunction(call, parents)
			if fn == nil {
				report("bridge call outside named runtime entry")
				return true
			}
			name := readOnlyWALManualName(call.Fun)
			if name == "ManualRollbackSQLState" && fn.Name.Name != "CaptureManualRollbackRestartCommit" {
				report("no Tx-locking snapshot in original-Commit observer")
			}
			switch name {
			case "os.NewFile":
				if fn.Name.Name != "NewManualRollbackRestartChannel" {
					report("pipe adoption outside exact child channel gate")
				}
			case "c.events.Write":
				if fn.Name.Name != "send" {
					report("IPC write outside concrete bounded sender")
				}
			case "io.ReadFull":
				if fn.Name.Name != "receive" {
					report("IPC read outside concrete bounded receiver")
				}
			case "c.control.Close", "c.events.Close":
				if fn.Name.Name != "CloseNormal" {
					report("pipe owner close outside once-only normal closure")
				}
			case "fault.main.RetirementComplete", "fault.journal.RetirementComplete":
				if fn.Name.Name != "CaptureManualRollbackRestartCommit" {
					report("native owner observation outside post-Commit capture")
				}
			}
		}
		return true
	})
	for name, want := range map[string]int{"os.NewFile": 2, "os.Exit": 3, "c.events.Write": 1, "io.ReadFull": 2, "c.control.Close": 1, "c.events.Close": 1, "finishRollbackFault": 1, "disarmRollbackFault": 1, "ArmManualRollbackSQLFault": 1, "ManualRollbackSQLState": 1} {
		if readOnlyWALManualCountCall(file, name) != want {
			report("bridge fixed native/old-lifecycle call count changed: " + name)
		}
	}
	return findings
}

// This independent command-capability contract is not a supplemental source
// pin/shape. The complete reviewed spawn declaration fixes construction, every
// launch field assignment, ordering, sole Start/Wait and object publication.
// Repinning rollbackRestartPins and rollbackRestartShapes cannot change it.
const restartGuardCommandSpawnDeclaration = `func manualRestartSpawn(c *manualRestartConfig, receipt *rollbackrestart.Receipt, role rollbackrestart.Role) (*manualRestartChild, error) {
	if c.role != rollbackrestart.Controller || (role != rollbackrestart.Writer && role != rollbackrestart.Recovery) || !manualRollbackSQLRegistryEmpty() || !c.sql.ledger.retired() {
		return nil, fmt.Errorf("parent role and no live owners required before spawn")
	}
	slot := int(role) - int(rollbackrestart.Writer)
	if c.children[slot] != nil {
		return nil, fmt.Errorf("one actual process per fixed role; no retry")
	}
	if role == rollbackrestart.Recovery && (c.children[0] == nil || !c.children[0].waited || c.children[0].actual.state == nil || !c.children[0].actual.state.Exited()) {
		return nil, fmt.Errorf("recovery cannot spawn before actual writer Wait")
	}
	for i := range c.family {
		if !c.family[i].ledger.retired() {
			return nil, fmt.Errorf("family owner live before spawn")
		}
	}
	if err := manualRestartUnchanged(c); err != nil {
		return nil, err
	}
	if err := manualRestartImage(c); err != nil {
		return nil, err
	}
	args, err := manualRestartFlags(role)
	if err != nil {
		return nil, err
	}
	rawPlan, _, err := manualRestartRead(c, filepath.Join(c.root, "control", "plan.json"), 65536)
	if err != nil {
		return nil, err
	}
	_, count := receipt.Frames()
	child := &manualRestartChild{role: role, wait: make(chan manualRestartWaitResult, 1), sequence: count + 1}
	c.children[slot] = child
	child.identity = c.plan.Spec().Writer
	if role == rollbackrestart.Recovery {
		child.identity = c.plan.Spec().Recovery
	}
	controlRead, controlWrite, controlReadOwner, controlWriteOwner, err := manualRestartPipe(c)
	if err != nil {
		return child, err
	}
	eventRead, eventWrite, eventReadOwner, eventWriteOwner, err := manualRestartPipe(c)
	if err != nil {
		return child, err
	}
	child.controls, child.events = controlWrite, eventRead
	child.controlOwner, child.eventOwner = controlWriteOwner, eventReadOwner
	// The only executable is the exact verified running image. No inherited
	// PATH, GOFLAGS, testing flags, role authorization or environment is used.
	command := exec.Command(c.binding.BinaryPath, args...)
	command.Dir = c.root
	command.Env = []string{
		"GOSQLITE_MANUAL_FAULT_ROLLBACK=independently_authorized",
		"GOSQLITE_MANUAL_RESTART_ROLLBACK=independently_authorized",
		"GOSQLITE_RESTART_PRIVATE_ROOT_CONFIRMED=private_stable_namespace_retained",
		"GOSQLITE_RESTART_ROLE=" + fmt.Sprint(uint8(role)),
		"GOSQLITE_RESTART_ROOT=" + c.root,
		"GOSQLITE_RESTART_BINDING=" + filepath.Join(c.root, "inputs", "binding.input.json"),
		"GOSQLITE_RESTART_BINDING_SHA256=" + string(manualRestartDigest(c.rawBinding)),
		"GOSQLITE_RESTART_PLAN_SHA256=" + string(manualRestartDigest(rawPlan)),
		"GOSQLITE_RESTART_SEQUENCE=" + fmt.Sprint(count+1),
	}
	command.ExtraFiles = []*os.File{controlRead, eventWrite}
	command.Stdout, command.Stderr = &child.stdout, &child.stderr
	child.command = command
	c.active = child
	if err := command.Start(); err != nil {
		return child, fmt.Errorf("spawn did not establish an owned child: %w", err)
	}
	child.pid = command.Process.Pid
	// Start proves ownership of this process object; message PIDs never select
	// a process. The goroutine does exactly one actual Wait and retains state.
	go func() {
		waitErr := command.Wait()
		child.wait <- manualRestartWaitResult{state: command.ProcessState, err: waitErr}
	}()
	closeErr := errors.Join(c.sql.ledger.close(controlReadOwner), c.sql.ledger.close(eventWriteOwner))
	if closeErr != nil {
		return child, closeErr
	}
	if err := child.events.SetReadDeadline(time.Now().Add(manualRestartWait)); err != nil {
		return child, err
	}
	if err := child.controls.SetWriteDeadline(time.Now().Add(manualRestartWait)); err != nil {
		return child, err
	}
	kind := rollbackrestart.WriterSpawned
	if role == rollbackrestart.Recovery {
		kind = rollbackrestart.RecoverySpawned
	}
	frame := manualRestartParentFrame(c, receipt, kind)
	frame.Process = rollbackrestart.ProcessEvidence{PID: int64(child.pid), Handle: uint64(role - 1), Image: c.plan.Spec().Image, ImageHash: c.plan.Spec().Binding.Binary, SpawnObserved: true, State: rollbackrestart.ProcessRunning}
	if err := manualRestartAppend(c, receipt, frame); err != nil {
		return child, err
	}
	return child, nil
}`

// Outside that exact declaration, the stored command capability has only one
// read: the complete direct PID-check statement. No object/field address,
// alias, assignment, method value or argument escape is admitted. The eight
// closed native types and no-reflection/interface rules remain prerequisites.
const restartGuardCommandPIDStatement = `if (frame.Kind == rollbackrestart.ChildReadyBound || frame.Kind == rollbackrestart.RecoveryReadyBound) && frame.Process.PID != int64(child.command.Process.Pid) {
		return frame, fmt.Errorf("child-specified PID does not match parent process object")
	}`

func restartGuardCommandClosure(path string, raw []byte, set *token.FileSet, file *ast.File, functions map[string]*ast.FuncDecl) []string {
	var findings []string
	report := func(message string) { findings = append(findings, path+": rollback restart contract: "+message) }
	spawn := functions["manualRestartSpawn"]
	exactSpawn := spawn != nil && restartGuardTokens(oracleNodeText(set, raw, spawn)) == restartGuardTokens(restartGuardCommandSpawnDeclaration)
	if !exactSpawn {
		report("command capability exact spawn declaration/order changed")
	}
	parents := oracleParents(file)
	commandField, commandType, commandRead := 0, 0, 0
	exactField := func(field *ast.Field) bool {
		if field == nil || len(field.Names) != 1 || field.Names[0].Name != "command" || rollbackFaultText(set, raw, field) != "command *exec.Cmd" {
			return false
		}
		fields, ok := parents[field].(*ast.FieldList)
		if !ok {
			return false
		}
		structure, ok := parents[fields].(*ast.StructType)
		if !ok {
			return false
		}
		owner, ok := parents[structure].(*ast.TypeSpec)
		return ok && owner.Name.Name == "manualRestartChild" && owner.Type == structure
	}
	ast.Inspect(file, func(node ast.Node) bool {
		if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "Cmd" {
			qualifier, bound := selector.X.(*ast.Ident)
			if bound && qualifier.Name == "exec" {
				pointer, isPointer := parents[selector].(*ast.StarExpr)
				var field *ast.Field
				if isPointer && pointer.X == selector {
					field, _ = parents[pointer].(*ast.Field)
				}
				if exactField(field) && field.Type == pointer {
					commandType++
				} else {
					report("command capability type outside sole concrete owner field")
				}
			}
		}
		id, ok := node.(*ast.Ident)
		if !ok || id.Name != "command" {
			return true
		}
		if field, ok := parents[id].(*ast.Field); ok && exactField(field) && field.Names[0] == id {
			commandField++
			return true
		}
		// The one Wait goroutine belongs to the exact frozen declaration;
		// the general enclosing-function helper intentionally stops at literals.
		if exactSpawn {
			for owner := parents[id]; owner != nil; owner = parents[owner] {
				if declaration, ok := owner.(*ast.FuncDecl); ok {
					if declaration == spawn {
						return true
					}
					break
				}
			}
		}
		fn := oracleEnclosingFunction(id, parents)
		if fn != nil && fn.Name.Name == "manualRestartReadFrame" {
			selector, isSelector := parents[id].(*ast.SelectorExpr)
			if isSelector && selector.Sel == id && rollbackFaultText(set, raw, selector) == "child.command" {
				for owner := parents[selector]; owner != nil && owner != fn; owner = parents[owner] {
					statement, isStatement := owner.(*ast.IfStmt)
					if isStatement && parents[statement] == fn.Body && restartGuardTokens(oracleNodeText(set, raw, statement)) == restartGuardTokens(restartGuardCommandPIDStatement) {
						commandRead++
						return true
					}
				}
			}
		}
		report("command capability alias/field/use outside exact spawn and PID read")
		return true
	})
	if commandField != 1 || commandType != 1 || commandRead != 1 {
		report("command capability exact owner/type/read inventory changed")
	}
	return findings
}

// Finite construction-input closure, independent of both supplemental source
// snapshots. These are the actual three role entries, every existing mutable
// config/child holder, and their value/read/decode/image/pipe dependencies.
// A complete declaration fixes permitted initialization and decoder addresses,
// field/aggregate writes, slice/address passes, return objects and owner order.
// It does not claim a policy for arbitrary Go programs. The three other native
// declarations are bounded diagnostics and the already exact FreshRegistry;
// they cannot obtain this closure's holders under the type/reference rules.
func restartGuardLaunchInputDeclaration(name string) (string, bool) {
	switch name {
	case "manualRestartDigest":
		return "func manualRestartDigest(raw []byte) rollbackrestart.Digest {\n\tsum := sha256.Sum256(raw)\n\treturn rollbackrestart.Digest(hex.EncodeToString(sum[:]))\n}", true
	case "manualRestartID":
		return "func manualRestartID(info os.FileInfo) (rollbackrestart.FileIdentity, error) {\n\tif info == nil {\n\t\treturn rollbackrestart.FileIdentity{}, fmt.Errorf(\"missing actual stat\")\n\t}\n\tstat, ok := info.Sys().(*syscall.Stat_t)\n\tif !ok || stat.Dev == 0 || stat.Ino == 0 {\n\t\treturn rollbackrestart.FileIdentity{}, fmt.Errorf(\"native dev/inode unavailable\")\n\t}\n\treturn rollbackrestart.FileIdentity{Device: uint64(stat.Dev), Inode: stat.Ino}, nil\n}", true
	case "manualRestartRegular":
		return "func manualRestartRegular(info os.FileInfo) error {\n\tif info == nil || !info.Mode().IsRegular() {\n\t\treturn fmt.Errorf(\"regular file required\")\n\t}\n\tstat, ok := info.Sys().(*syscall.Stat_t)\n\tif !ok || stat.Nlink != 1 || stat.Uid != uint32(os.Getuid()) {\n\t\treturn fmt.Errorf(\"single-link owner-controlled file required\")\n\t}\n\treturn nil\n}", true
	case "manualRestartRead":
		return "func manualRestartRead(c *manualRestartConfig, path string, bound int64) ([]byte, os.FileInfo, error) {\n\tif !filepath.IsAbs(path) || filepath.Clean(path) != path {\n\t\treturn nil, nil, fmt.Errorf(\"absolute clean input path required\")\n\t}\n\tcurrent := string(filepath.Separator)\n\tfor _, component := range strings.Split(strings.TrimPrefix(filepath.Dir(path), \"/\"), \"/\") {\n\t\tif component == \"\" {\n\t\t\tcontinue\n\t\t}\n\t\tcurrent = filepath.Join(current, component)\n\t\tinfo, err := os.Lstat(current)\n\t\tif err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {\n\t\t\treturn nil, nil, fmt.Errorf(\"input ancestor unverified: %s: %v\", current, err)\n\t\t}\n\t}\n\traw, info, err := manualRollbackSQLRead(&c.sql.ledger, path, bound)\n\tif err != nil {\n\t\treturn nil, nil, err\n\t}\n\tif err := manualRestartRegular(info); err != nil {\n\t\treturn nil, nil, err\n\t}\n\treturn raw, info, nil\n}", true
	case "manualRestartDecode":
		return "func manualRestartDecode(raw []byte, value any) error {\n\tif len(raw) == 0 || len(raw) > 4<<20 {\n\t\treturn fmt.Errorf(\"bounded canonical JSON required\")\n\t}\n\tif err := manualRollbackSQLDecode(raw, value); err != nil {\n\t\treturn err\n\t}\n\tcanonical, err := json.Marshal(value)\n\tif err != nil || !bytes.Equal(raw, canonical) {\n\t\treturn fmt.Errorf(\"noncanonical, duplicate, omitted, or unknown JSON fields: %v\", err)\n\t}\n\treturn nil\n}", true
	case "manualRestartFlags":
		return "func manualRestartFlags(role rollbackrestart.Role) ([]string, error) {\n\troot := \"\"\n\tswitch role {\n\tcase rollbackrestart.Controller:\n\t\troot = \"^TestManualRollbackRestartController$\"\n\tcase rollbackrestart.Writer:\n\t\troot = \"^TestManualRollbackRestartWriter$\"\n\tcase rollbackrestart.Recovery:\n\t\troot = \"^TestManualRollbackRestartRecovery$\"\n\tdefault:\n\t\treturn nil, fmt.Errorf(\"unknown role\")\n\t}\n\treturn []string{\"-test.run=\" + root, \"-test.count=1\", \"-test.shuffle=off\", \"-test.parallel=1\", \"-test.timeout=0\", \"-test.cpu=1\", \"-test.v=true\"}, nil\n}", true
	case "manualRestartCheckFlags":
		return "func manualRestartCheckFlags(role rollbackrestart.Role) error {\n\twant, err := manualRestartFlags(role)\n\tif err != nil || len(os.Args) != len(want)+1 {\n\t\treturn fmt.Errorf(\"exact test root and seven flags required\")\n\t}\n\tfor i := range want {\n\t\tif os.Args[i+1] != want[i] {\n\t\t\treturn fmt.Errorf(\"unexpected flag or flag order\")\n\t\t}\n\t}\n\treturn nil\n}", true
	case "manualRestartImage":
		return "func manualRestartImage(c *manualRestartConfig) error {\n\ti, err := c.sql.ledger.reserve()\n\tif err != nil {\n\t\treturn err\n\t}\n\timage, openErr := os.Open(\"/proc/self/exe\")\n\tc.sql.ledger.owners[i] = manualRollbackSQLOwner{file: image, path: \"/proc/self/exe\"}\n\tif openErr != nil {\n\t\treturn errors.Join(openErr, c.sql.ledger.close(i))\n\t}\n\tbefore, statErr := image.Stat()\n\tif statErr != nil || before.Size() < 1 || before.Size() > 128<<20 || !before.Mode().IsRegular() {\n\t\treturn errors.Join(fmt.Errorf(\"running image unverified\"), statErr, c.sql.ledger.close(i))\n\t}\n\traw, readErr := io.ReadAll(io.LimitReader(image, (128<<20)+1))\n\tafter, afterErr := image.Stat()\n\tcloseErr := c.sql.ledger.close(i)\n\tif err := errors.Join(readErr, afterErr, closeErr); err != nil {\n\t\treturn err\n\t}\n\tif int64(len(raw)) != before.Size() || !os.SameFile(before, after) || before.Size() != after.Size() || manualRestartDigest(raw) != rollbackrestart.Digest(c.binding.Binary) {\n\t\treturn fmt.Errorf(\"actual running image changed or digest mismatch\")\n\t}\n\tif !filepath.IsAbs(c.binding.BinaryPath) || filepath.Clean(c.binding.BinaryPath) != c.binding.BinaryPath {\n\t\treturn fmt.Errorf(\"absolute reviewed binary path required\")\n\t}\n\ttarget, targetInfo, err := manualRestartRead(c, c.binding.BinaryPath, 128<<20)\n\tif err != nil || !os.SameFile(before, targetInfo) || manualRestartDigest(target) != rollbackrestart.Digest(c.binding.Binary) {\n\t\treturn fmt.Errorf(\"spawn target differs from running image: %v\", err)\n\t}\n\tc.selfInfo = before\n\treturn nil\n}", true
	case "manualRestartAdmit":
		return "func manualRestartAdmit(c *manualRestartConfig, role rollbackrestart.Role) error {\n\t// Must dominate flags, paths, stat, descriptors, owners, nonce and process work.\n\tif os.Getenv(\"GOSQLITE_MANUAL_FAULT_ROLLBACK\") != \"independently_authorized\" || os.Getenv(\"GOSQLITE_MANUAL_RESTART_ROLLBACK\") != \"independently_authorized\" {\n\t\treturn fmt.Errorf(\"ADMISSION_BLOCKED: independent fault and restart authorization required\")\n\t}\n\tif err := manualRestartCheckFlags(role); err != nil {\n\t\treturn err\n\t}\n\tif os.Getenv(\"GOSQLITE_RESTART_PRIVATE_ROOT_CONFIRMED\") != \"private_stable_namespace_retained\" || os.Getenv(\"GOSQLITE_RESTART_ROLE\") != fmt.Sprint(uint8(role)) {\n\t\treturn fmt.Errorf(\"role and private retained namespace confirmation required\")\n\t}\n\tc.role = role\n\tc.root = os.Getenv(\"GOSQLITE_RESTART_ROOT\")\n\trootInfo, err := manualRollbackSQLNamespace(c.root)\n\tif err != nil {\n\t\treturn err\n\t}\n\tc.sql.root, c.sql.rootInfo = c.root, rootInfo\n\tvar filesystem syscall.Statfs_t\n\tif err := syscall.Statfs(c.root, &filesystem); err != nil {\n\t\treturn fmt.Errorf(\"native filesystem identity unavailable: %w\", err)\n\t}\n\traw, _, err := manualRestartRead(c, os.Getenv(\"GOSQLITE_RESTART_BINDING\"), 65536)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif manualRestartDigest(raw) != rollbackrestart.Digest(os.Getenv(\"GOSQLITE_RESTART_BINDING_SHA256\")) {\n\t\treturn fmt.Errorf(\"reviewed binding digest mismatch\")\n\t}\n\tif err := manualRestartDecode(raw, &c.binding); err != nil {\n\t\treturn err\n\t}\n\tc.rawBinding = raw\n\tif c.binding.Filesystem != fmt.Sprintf(\"linux-fstype-%x\", uint64(filesystem.Type)) {\n\t\treturn fmt.Errorf(\"declared filesystem does not match native root statfs\")\n\t}\n\twhich, ok := rollbackrestart.ParseCase(c.binding.Case)\n\tif !ok || c.binding.Schema != \"rollback-restart-binding-v1\" || len(c.binding.Trial) < 1 || len(c.binding.Trial) > 64 || strings.ContainsAny(c.binding.Trial, \" /\\\\\\t\\r\\n\") || c.binding.NativeDeclaration != \"linux-native-process-exit-no-powerloss-4096-retain-all\" || c.binding.Filesystem == \"\" || len(c.binding.Filesystem) > 64 {\n\t\treturn fmt.Errorf(\"fixed case, trial and explicit native declaration required\")\n\t}\n\tmanifest, _, err := manualRestartRead(c, c.binding.SourceManifestPath, 4<<20)\n\tif err != nil || manualRestartDigest(manifest) != rollbackrestart.Digest(c.binding.SourceManifest) {\n\t\treturn fmt.Errorf(\"source manifest mismatch: %v\", err)\n\t}\n\tvar inventory struct {\n\t\tFiles map[string]struct {\n\t\t\tBytes   int64  `json:\"bytes\"`\n\t\t\tGitBlob string `json:\"git_blob\"`\n\t\t\tGitMode string `json:\"git_mode\"`\n\t\t\tSHA256  string `json:\"sha256\"`\n\t\t} `json:\"files\"`\n\t\tTree string `json:\"tree\"`\n\t}\n\tif err := manualRollbackSQLDecode(manifest, &inventory); err != nil {\n\t\treturn err\n\t}\n\tif inventory.Tree != c.binding.SourceTree || len(inventory.Tree) != 40 || len(inventory.Files) < 1 || len(inventory.Files) > 4096 {\n\t\treturn fmt.Errorf(\"complete finite source inventory required\")\n\t}\n\tfor path, entry := range inventory.Files {\n\t\tif filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || strings.HasPrefix(path, \"../\") || entry.Bytes < 0 || (entry.GitMode != \"100644\" && entry.GitMode != \"100755\") || len(entry.GitBlob) != 40 {\n\t\t\treturn fmt.Errorf(\"invalid source inventory entry\")\n\t\t}\n\t\tif _, err := manualRollbackSQLHash(entry.SHA256); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\tcompiled, _, err := manualRestartRead(c, c.binding.CompileReceiptPath, 65536)\n\tif err != nil || manualRestartDigest(compiled) != rollbackrestart.Digest(c.binding.CompileReceipt) {\n\t\treturn fmt.Errorf(\"compile receipt mismatch: %v\", err)\n\t}\n\tvar receipt manualRollbackSQLCompile\n\tif err := manualRestartDecode(compiled, &receipt); err != nil {\n\t\treturn err\n\t}\n\tif receipt.Schema != \"rollback-restart-compile-v1\" || receipt.Source != c.binding.SourceManifest || receipt.SourceTree != c.binding.SourceTree || receipt.Tool != c.binding.Tool || receipt.Binary != c.binding.Binary || receipt.Manifest != c.binding.SourceManifest || receipt.Package != \"./sqlite\" || receipt.Tags != \"manual_fault_rollback,manual_restart_rollback\" || !receipt.CompileOnly || int(receipt.Files) != len(inventory.Files) {\n\t\treturn fmt.Errorf(\"reviewed source-to-binary receipt does not bind this restart image\")\n\t}\n\ttool, _, err := manualRestartRead(c, c.binding.ToolPath, 128<<20)\n\tif err != nil || manualRestartDigest(tool) != rollbackrestart.Digest(c.binding.Tool) {\n\t\treturn fmt.Errorf(\"tool image digest mismatch: %v\", err)\n\t}\n\tif err := manualRestartImage(c); err != nil {\n\t\treturn err\n\t}\n\tbaseline, err := manualRollbackSQLBaseline()\n\tif err != nil {\n\t\treturn err\n\t}\n\tc.baseline = baseline\n\trootID, err := manualRestartID(rootInfo)\n\tif err != nil {\n\t\treturn err\n\t}\n\tbinding := rollbackrestart.Binding{Suite: \"rollback-restart-v1\", Profile: rollbackfault.PlainRollback4096, Case: which, Trial: c.binding.Trial, Source: c.binding.SourceTree, Manifest: rollbackrestart.Digest(c.binding.SourceManifest), Tool: rollbackrestart.Digest(c.binding.Tool), Compile: rollbackrestart.Digest(c.binding.CompileReceipt), Binary: rollbackrestart.Digest(c.binding.Binary), Native: rollbackrestart.NativeIdentity{OS: \"linux\", Filesystem: c.binding.Filesystem, SectorSize: 4096, PageSize: 4096}, Root: rootID, Baseline: manualRestartDigest(baseline[:])}\n\tif role == rollbackrestart.Controller {\n\t\timageID, err := manualRestartID(c.selfInfo)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tspec := rollbackrestart.PlanSpec{Binding: binding, Ticket: 1, Image: imageID}\n\t\tidentities := []*rollbackrestart.RoleIdentity{&spec.Controller, &spec.Writer, &spec.Recovery}\n\t\tfor n, identity := range identities {\n\t\t\tvar nonce [32]byte\n\t\t\tif count, err := rand.Read(nonce[:]); err != nil || count != len(nonce) {\n\t\t\t\treturn fmt.Errorf(\"parent nonce generation failed: %v\", err)\n\t\t\t}\n\t\t\t*identity = rollbackrestart.RoleIdentity{Role: rollbackrestart.Role(n + 1), Generation: uint64(n + 1), Challenge: rollbackrestart.Digest(hex.EncodeToString(nonce[:]))}\n\t\t}\n\t\tspec.AdapterIdentity = manualRestartSQLIdentity(binding, spec.Writer)\n\t\tplan, problem := rollbackrestart.NewPlan(spec)\n\t\tif problem != rollbackrestart.NoProblem {\n\t\t\treturn fmt.Errorf(\"restart plan admission: %v\", problem)\n\t\t}\n\t\tc.plan = plan\n\t} else {\n\t\trawPlan, _, err := manualRestartRead(c, filepath.Join(c.root, \"control\", \"plan.json\"), 65536)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tvar spec rollbackrestart.PlanSpec\n\t\tif err := manualRestartDecode(rawPlan, &spec); err != nil {\n\t\t\treturn err\n\t\t}\n\t\tif spec.Binding != binding || manualRestartDigest(rawPlan) != rollbackrestart.Digest(os.Getenv(\"GOSQLITE_RESTART_PLAN_SHA256\")) {\n\t\t\treturn fmt.Errorf(\"child plan/root/source/baseline identity mismatch\")\n\t\t}\n\t\tc.plan, _ = rollbackrestart.NewPlan(spec)\n\t\tif c.plan.Spec() != spec {\n\t\t\treturn fmt.Errorf(\"child plan refused\")\n\t\t}\n\t}\n\treturn c.sql.stable()\n}", true
	case "manualRestartSQLIdentity":
		return "func manualRestartSQLIdentity(binding rollbackrestart.Binding, role rollbackrestart.RoleIdentity) rollbackfault.Identity {\n\treturn rollbackfault.Identity{Suite: \"rollback-restart-v1\", Source: binding.Source, Tool: string(binding.Tool), Binary: string(binding.Binary), Manifest: string(binding.Manifest), Trial: binding.Trial, Scope: fmt.Sprintf(\"restart-role-%d-generation-%d\", role.Role, role.Generation), Profile: rollbackfault.PlainRollback4096}\n}", true
	case "manualRestartFamilies":
		return "func manualRestartFamilies(c *manualRestartConfig, create bool) error {\n\tnames := [5]string{\"inputs\", \"trial\", \"evidence\", \"replay\", \"control\"}\n\tif create {\n\t\towner, err := c.sql.ledger.reserve()\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tdirectory, openErr := os.Open(c.root)\n\t\tc.sql.ledger.owners[owner] = manualRollbackSQLOwner{file: directory, path: c.root}\n\t\tif openErr != nil {\n\t\t\treturn errors.Join(openErr, c.sql.ledger.close(owner))\n\t\t}\n\t\tactual, statErr := directory.Stat()\n\t\tif statErr != nil || !os.SameFile(actual, c.sql.rootInfo) {\n\t\t\treturn errors.Join(fmt.Errorf(\"root owner identity changed\"), statErr, c.sql.ledger.close(owner))\n\t\t}\n\t\tentries, readErr := directory.ReadDir(1)\n\t\tcloseErr := c.sql.ledger.close(owner)\n\t\tif readErr != io.EOF || len(entries) != 0 || closeErr != nil {\n\t\t\treturn errors.Join(fmt.Errorf(\"controller root must be empty\"), readErr, closeErr)\n\t\t}\n\t}\n\tfor i, name := range names {\n\t\tpath := filepath.Join(c.root, name)\n\t\tif create {\n\t\t\tif err := os.Mkdir(path, 0700); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n\t\t}\n\t\tinfo, err := manualRollbackSQLNamespace(path)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tc.family[i].root, c.family[i].rootInfo = path, info\n\t}\n\treturn c.sql.stable()\n}", true
	case "manualRestartSyncDirectory":
		return "func manualRestartSyncDirectory(c *manualRestartConfig, family int) error {\n\tif family < 0 || family >= len(c.family) {\n\t\treturn fmt.Errorf(\"closed family required\")\n\t}\n\tf := &c.family[family]\n\tif err := f.stable(); err != nil {\n\t\treturn err\n\t}\n\ti, err := c.sql.ledger.reserve()\n\tif err != nil {\n\t\treturn err\n\t}\n\tdir, openErr := os.Open(f.root)\n\tc.sql.ledger.owners[i] = manualRollbackSQLOwner{file: dir, path: f.root}\n\tif openErr != nil {\n\t\treturn errors.Join(openErr, c.sql.ledger.close(i))\n\t}\n\tactual, statErr := dir.Stat()\n\tif statErr != nil || !os.SameFile(actual, f.rootInfo) {\n\t\treturn errors.Join(fmt.Errorf(\"directory changed\"), statErr, c.sql.ledger.close(i))\n\t}\n\tsyncErr := dir.Sync()\n\tcloseErr := c.sql.ledger.close(i)\n\treturn errors.Join(syncErr, closeErr, f.stable())\n}", true
	case "manualRestartCreate":
		return "func manualRestartCreate(c *manualRestartConfig, family int, name string, raw []byte) error {\n\tallowed := map[string]bool{\n\t\t\"0/baseline.input.db\": true, \"0/binding.input.json\": true,\n\t\t\"1/trial.db\": true, \"2/post-exit.input.db\": true, \"2/post-exit-journal.input.bin\": true,\n\t\t\"3/replay.db\": true, \"3/replay.db-journal\": true,\n\t\t\"4/plan.json\": true, \"4/ack.json\": true, \"4/baseline-sql.json\": true,\n\t\t\"4/readback.json\": true, \"4/result.json\": true, \"4/final-manifest.json\": true, \"4/failure.json\": true,\n\t\t\"4/failure-wait.json\": true, \"4/failure-stdout.json\": true, \"4/failure-stderr.json\": true, \"4/failure-protocol.json\": true,\n\t\t\"4/writer-stdout.json\": true, \"4/writer-stderr.json\": true,\n\t\t\"4/recovery-stdout.json\": true, \"4/recovery-stderr.json\": true,\n\t\t\"4/writer-protocol.json\": true, \"4/recovery-protocol.json\": true,\n\t}\n\tif !allowed[fmt.Sprintf(\"%d/%s\", family, name)] {\n\t\treturn fmt.Errorf(\"unreviewed artifact name\")\n\t}\n\tif len(raw) == 0 {\n\t\treturn fmt.Errorf(\"empty artifact body refused; diagnostics use explicit JSON envelopes\")\n\t}\n\tinfo, err := c.family[family].create(name, raw, false)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartRegular(info); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartSyncDirectory(c, family); err != nil {\n\t\treturn err\n\t}\n\tif family == 4 && (name == \"writer-stdout.json\" || name == \"writer-stderr.json\" || name == \"writer-protocol.json\" || name == \"recovery-stdout.json\" || name == \"recovery-stderr.json\" || name == \"recovery-protocol.json\" || name == \"readback.json\" || name == \"result.json\" || name == \"final-manifest.json\") {\n\t\treturn manualRestartRetain(c, \"control/\"+name, 1<<20)\n\t}\n\treturn nil\n}", true
	case "manualRestartArtifactAt":
		return "func manualRestartArtifactAt(c *manualRestartConfig, relative string, bound int64) (manualRestartArtifact, []byte, error) {\n\tout := manualRestartArtifact{Path: relative}\n\tpath := filepath.Join(c.root, relative)\n\tif filepath.IsAbs(relative) || filepath.Clean(relative) != relative || strings.HasPrefix(relative, \"../\") {\n\t\treturn out, nil, fmt.Errorf(\"fixed relative artifact required\")\n\t}\n\traw, info, err := manualRestartRead(c, path, bound)\n\tif errors.Is(err, os.ErrNotExist) {\n\t\treturn out, nil, nil\n\t}\n\tif err != nil {\n\t\treturn out, nil, err\n\t}\n\tid, err := manualRestartID(info)\n\tif err != nil {\n\t\treturn out, nil, err\n\t}\n\tout.Device, out.Inode, out.Bytes, out.Hash, out.Exists = id.Device, id.Inode, int64(len(raw)), string(manualRestartDigest(raw)), true\n\treturn out, raw, nil\n}", true
	case "manualRestartRetain":
		return "func manualRestartRetain(c *manualRestartConfig, relative string, bound int64) error {\n\tstamp, _, err := manualRestartArtifactAt(c, relative, bound)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif c.retainedCount == len(c.retained) {\n\t\treturn fmt.Errorf(\"artifact inventory full\")\n\t}\n\tfor i := 0; i < c.retainedCount; i++ {\n\t\tprevious := c.retained[i]\n\t\tif previous.Path == relative || (stamp.Exists && previous.Exists && stamp.Device == previous.Device && stamp.Inode == previous.Inode) {\n\t\t\treturn fmt.Errorf(\"duplicate or aliased original/copy\")\n\t\t}\n\t}\n\tc.retained[c.retainedCount] = stamp\n\tc.retainedCount++\n\treturn nil\n}", true
	case "manualRestartUnchanged":
		return "func manualRestartUnchanged(c *manualRestartConfig) error {\n\tif err := c.sql.stable(); err != nil {\n\t\treturn err\n\t}\n\tfor i := 0; i < len(c.family); i++ {\n\t\tif err := c.family[i].stable(); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\tfor i := 0; i < c.retainedCount; i++ {\n\t\tprevious := c.retained[i]\n\t\tstamp, _, err := manualRestartArtifactAt(c, previous.Path, 1<<20)\n\t\tif err != nil || stamp != previous {\n\t\t\treturn fmt.Errorf(\"ARTIFACT_CHANGED: %s: %v\", previous.Path, err)\n\t\t}\n\t}\n\treturn nil\n}", true
	case "manualRestartAppend":
		return "func manualRestartAppend(c *manualRestartConfig, receipt *rollbackrestart.Receipt, frame rollbackrestart.Frame) error {\n\tif problem := receipt.Append(frame); problem != rollbackrestart.ProblemNone {\n\t\treturn fmt.Errorf(\"PROTOCOL_INVALID: frame %d kind %d: %v\", frame.Sequence, frame.Kind, problem)\n\t}\n\treturn nil\n}", true
	case "manualRestartParentFrame":
		return "func manualRestartParentFrame(c *manualRestartConfig, receipt *rollbackrestart.Receipt, kind rollbackrestart.FrameKind) rollbackrestart.Frame {\n\t_, count := receipt.Frames()\n\treturn rollbackrestart.Frame{Schema: rollbackrestart.SchemaVersion, Binding: c.plan.Spec().Binding, Sender: c.plan.Spec().Controller, Sequence: count + 1, Kind: kind}\n}", true
	case "manualRestartPipe":
		return "func manualRestartPipe(c *manualRestartConfig) (*os.File, *os.File, int, int, error) {\n\treadOwner, err := c.sql.ledger.reserve()\n\tif err != nil {\n\t\treturn nil, nil, 0, 0, err\n\t}\n\twriteOwner, err := c.sql.ledger.reserve()\n\tif err != nil {\n\t\treturn nil, nil, 0, 0, err\n\t}\n\tread, write, pipeErr := os.Pipe()\n\tc.sql.ledger.owners[readOwner] = manualRollbackSQLOwner{file: read, path: \"owned restart pipe reader\"}\n\tc.sql.ledger.owners[writeOwner] = manualRollbackSQLOwner{file: write, path: \"owned restart pipe writer\"}\n\tif pipeErr != nil {\n\t\treturn nil, nil, readOwner, writeOwner, pipeErr\n\t}\n\treturn read, write, readOwner, writeOwner, nil\n}", true
	case "manualRestartSpawn":
		return restartGuardCommandSpawnDeclaration, true
	case "manualRestartReadFrame":
		return "func manualRestartReadFrame(c *manualRestartConfig, child *manualRestartChild, receipt *rollbackrestart.Receipt) (rollbackrestart.Frame, error) {\n\tif child == nil || child.pid == 0 || child.waited || child.frameCount >= rollbackrestart.MaxFrames {\n\t\treturn rollbackrestart.Frame{}, fmt.Errorf(\"owned live child and bounded protocol required\")\n\t}\n\tvar header [4]byte\n\tif n, err := io.ReadFull(child.events, header[:]); err != nil {\n\t\tif child.rawCount < rollbackrestart.MaxFrames {\n\t\t\tchild.rawFrames[child.rawCount] = append([]byte(nil), header[:n]...)\n\t\t\tchild.rawCount++\n\t\t}\n\t\treturn rollbackrestart.Frame{}, err\n\t}\n\tcount := binary.BigEndian.Uint32(header[:])\n\tif count == 0 || count > rollbackrestart.MaxFrameBytes {\n\t\tif child.rawCount < rollbackrestart.MaxFrames {\n\t\t\tchild.rawFrames[child.rawCount] = append([]byte(nil), header[:]...)\n\t\t\tchild.rawCount++\n\t\t}\n\t\treturn rollbackrestart.Frame{}, fmt.Errorf(\"oversized/empty protocol frame\")\n\t}\n\traw := make([]byte, 4+int(count))\n\tcopy(raw, header[:])\n\tn, readErr := io.ReadFull(child.events, raw[4:])\n\tif child.rawCount >= rollbackrestart.MaxFrames {\n\t\treturn rollbackrestart.Frame{}, fmt.Errorf(\"raw frame limit\")\n\t}\n\tchild.rawFrames[child.rawCount] = append([]byte(nil), raw[:4+n]...)\n\tchild.rawCount++\n\tif readErr != nil {\n\t\treturn rollbackrestart.Frame{}, readErr\n\t}\n\tframe, problem := rollbackrestart.DecodeFrame(raw)\n\tif problem != rollbackrestart.ProblemNone || frame.Sender != child.identity || frame.Binding != c.plan.Spec().Binding || frame.Sequence != child.sequence+1 {\n\t\treturn frame, fmt.Errorf(\"wrong, stale, duplicate or unbound child frame: %v\", problem)\n\t}\n\t// Ready's PID is cross-checked against the actual object returned by Start.\n\tif (frame.Kind == rollbackrestart.ChildReadyBound || frame.Kind == rollbackrestart.RecoveryReadyBound) && frame.Process.PID != int64(child.command.Process.Pid) {\n\t\treturn frame, fmt.Errorf(\"child-specified PID does not match parent process object\")\n\t}\n\tchild.frames[child.frameCount], child.frameCount = frame, child.frameCount+1\n\tchild.sequence = frame.Sequence\n\tif err := manualRestartAppend(c, receipt, frame); err != nil {\n\t\treturn frame, err\n\t}\n\treturn frame, nil\n}", true
	case "manualRestartSendFrame":
		return "func manualRestartSendFrame(c *manualRestartConfig, child *manualRestartChild, receipt *rollbackrestart.Receipt, frame rollbackrestart.Frame) error {\n\tif child == nil || child.pid == 0 || child.waited || frame.Sequence != child.sequence+1 {\n\t\treturn fmt.Errorf(\"owned child grant ordering\")\n\t}\n\tpreview := *receipt\n\tif err := manualRestartAppend(c, &preview, frame); err != nil {\n\t\treturn err\n\t}\n\traw, problem := rollbackrestart.EncodeFrame(frame)\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn fmt.Errorf(\"grant encoding: %v\", problem)\n\t}\n\tchild.lastGrant, child.grantBytes = frame, len(raw)\n\tcount, err := child.controls.Write(raw)\n\tchild.grantWritten = count\n\tif err == nil && count != len(raw) {\n\t\terr = io.ErrShortWrite\n\t}\n\tif err != nil {\n\t\tchild.grantError = err.Error()\n\t\treturn err\n\t}\n\tchild.grantError = \"\"\n\t*receipt = preview\n\tchild.sequence = frame.Sequence\n\treturn nil\n}", true
	case "manualRestartAwait":
		return "func manualRestartAwait(t *testing.T, c *manualRestartConfig, child *manualRestartChild) manualRestartWaitResult {\n\tif child.waited {\n\t\treturn child.actual\n\t}\n\ttimer := time.NewTimer(manualRestartWait)\n\tselect {\n\tcase actual := <-child.wait:\n\t\ttimer.Stop()\n\t\tchild.waited, child.actual = true, actual\n\t\treturn actual\n\tcase <-timer.C:\n\t\tchild.timedOut = true\n\t\tmanualRestartPersistFailure(t, c, fmt.Errorf(\"CHILD_LIVE/WAIT_UNKNOWN: actual Wait exceeded observation deadline\"))\n\t\tt.Logf(\"CHILD_LIVE/WAIT_UNKNOWN: retained process object, generation %d, pid %d; no signal, cleanup or recovery\", child.identity.Generation, child.pid)\n\t\tactual := <-child.wait\n\t\tchild.waited, child.actual = true, actual\n\t\treturn actual\n\t}\n}", true
	case "manualRestartWaitChild":
		return "func manualRestartWaitChild(t *testing.T, c *manualRestartConfig, child *manualRestartChild, receipt *rollbackrestart.Receipt) error {\n\tactual := manualRestartAwait(t, c, child)\n\tif child.timedOut {\n\t\treturn fmt.Errorf(\"WAIT_UNKNOWN: observation deadline exceeded; actual eventual Wait retained without successful scenario\")\n\t}\n\tif actual.state == nil || actual.state.Pid() != child.pid {\n\t\treturn fmt.Errorf(\"WAIT_UNKNOWN: no actual matching process state: %v\", actual.err)\n\t}\n\tcode := 0\n\tif child.role == rollbackrestart.Writer && c.plan.Spec().Binding.Case != rollbackrestart.Control {\n\t\tcode = rollbackrestart.ExpectedExitCode\n\t}\n\tif !actual.state.Exited() || actual.state.ExitCode() != code {\n\t\treturn fmt.Errorf(\"CHILD_EXIT_UNEXPECTED: %v: %v\", actual.state, actual.err)\n\t}\n\tif code == 0 && actual.err != nil {\n\t\treturn fmt.Errorf(\"natural child exit lacked complete Wait: %w\", actual.err)\n\t}\n\tif code == rollbackrestart.ExpectedExitCode {\n\t\tvar exitError *exec.ExitError\n\t\tif !errors.As(actual.err, &exitError) || exitError.ProcessState != actual.state {\n\t\t\treturn fmt.Errorf(\"expected exit lacks actual ExitError provenance\")\n\t\t}\n\t}\n\t// With actual exit established, EOF must follow the exact closed protocol.\n\t// Any extra byte, duplicate terminal, truncation, or diagnostic overflow fails.\n\tif err := child.events.SetReadDeadline(time.Now().Add(manualRestartWait)); err != nil {\n\t\treturn err\n\t}\n\tvar trailing [1]byte\n\tn, tailErr := child.events.Read(trailing[:])\n\tif n != 0 && child.rawCount < rollbackrestart.MaxFrames {\n\t\tchild.rawFrames[child.rawCount] = append([]byte(nil), trailing[:n]...)\n\t\tchild.rawCount++\n\t}\n\tif n != 0 || tailErr != io.EOF {\n\t\treturn fmt.Errorf(\"protocol trailing bytes or missing EOF: %v\", tailErr)\n\t}\n\tstdout, outOverflow := child.stdout.snapshot()\n\tstderr, errOverflow := child.stderr.snapshot()\n\tif outOverflow || errOverflow {\n\t\treturn fmt.Errorf(\"bounded diagnostic overflow; no successful terminal\")\n\t}\n\tname := \"writer\"\n\tif child.role == rollbackrestart.Recovery {\n\t\tname = \"recovery\"\n\t}\n\tstdoutJSON, err := json.Marshal(struct {\n\t\tRaw      []byte\n\t\tOverflow bool\n\t}{stdout, outOverflow})\n\tif err != nil {\n\t\treturn err\n\t}\n\tstderrJSON, err := json.Marshal(struct {\n\t\tRaw      []byte\n\t\tOverflow bool\n\t}{stderr, errOverflow})\n\tif err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 4, name+\"-stdout.json\", stdoutJSON); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 4, name+\"-stderr.json\", stderrJSON); err != nil {\n\t\treturn err\n\t}\n\ttranscript, err := json.Marshal(child.frames[:child.frameCount])\n\tif err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 4, name+\"-protocol.json\", transcript); err != nil {\n\t\treturn err\n\t}\n\tif err := errors.Join(c.sql.ledger.close(child.eventOwner), c.sql.ledger.close(child.controlOwner)); err != nil {\n\t\treturn err\n\t}\n\tkind := rollbackrestart.WriterWaitObserved\n\tif child.role == rollbackrestart.Recovery {\n\t\tkind = rollbackrestart.RecoveryWaitObserved\n\t}\n\tframe := manualRestartParentFrame(c, receipt, kind)\n\tframe.Process = rollbackrestart.ProcessEvidence{PID: int64(child.pid), Handle: uint64(child.role - 1), Image: c.plan.Spec().Image, ImageHash: c.plan.Spec().Binding.Binary, WaitObserved: true, State: rollbackrestart.ProcessExited, ExitCode: code}\n\treturn manualRestartAppend(c, receipt, frame)\n}", true
	case "manualRestartAck":
		return "func manualRestartAck(c *manualRestartConfig, commit rollbackrestart.Frame) (rollbackrestart.AckEvidence, error) {\n\tout := rollbackrestart.AckEvidence{}\n\tif c.role != rollbackrestart.Controller || c.plan.Spec().Binding.Case != rollbackrestart.AcknowledgedExit || commit.Kind != rollbackrestart.PublicCommitReturned || commit.Commit.Value != rollbackrestart.CommitNil || commit.Sender != c.plan.Spec().Writer {\n\t\treturn out, fmt.Errorf(\"ACK_FAILED: exact public nil writer receipt required\")\n\t}\n\thash, problem := rollbackrestart.FrameDigest(commit)\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn out, fmt.Errorf(\"ACK_FAILED: original frame unavailable\")\n\t}\n\traw, problem := rollbackrestart.EncodeFrame(commit)\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn out, fmt.Errorf(\"ACK_FAILED: original frame encoding\")\n\t}\n\traw = raw[4:]\n\t// This is the unaltered canonical COMMIT_OK JSON body including every identity\n\t// and sequence, written exclusively then file-Sync/Close and dir-Sync/Close.\n\tif err := manualRestartCreate(c, 4, \"ack.json\", raw); err != nil {\n\t\treturn out, fmt.Errorf(\"ACK_UNCERTAIN: %w\", err)\n\t}\n\tstamp, ack, err := manualRestartArtifactAt(c, \"control/ack.json\", rollbackrestart.MaxFrameBytes+4)\n\tif err != nil || !stamp.Exists || !bytes.Equal(ack, raw) {\n\t\treturn out, fmt.Errorf(\"ACK_UNCERTAIN: acknowledgement changed: %v\", err)\n\t}\n\tout = rollbackrestart.AckEvidence{CommitSequence: commit.Sequence, CommitFrameSHA256: hash, SHA256: manualRestartDigest(raw), File: rollbackrestart.FileIdentity{Device: stamp.Device, Inode: stamp.Inode}, Bytes: stamp.Bytes, Written: int64(len(raw)), Exclusive: true, WriteStep: 1, FileSyncStep: 2, FileCloseStep: 3, DirectorySyncStep: 4, DirectoryCloseStep: 5, FileCloseCalls: 1, DirectoryCloseCalls: 1}\n\tif err := manualRestartRetain(c, \"control/ack.json\", rollbackrestart.MaxFrameBytes+4); err != nil {\n\t\treturn out, err\n\t}\n\tc.durableAck = out\n\treturn out, nil\n}", true
	case "manualRestartWriter":
		return "func manualRestartWriter(c *manualRestartConfig, channel *pager.ManualRollbackRestartChannel) error {\n\tif c.role != rollbackrestart.Writer || !manualRestartFreshRegistry() {\n\t\treturn fmt.Errorf(\"fresh writer registry required\")\n\t}\n\tctx := context.Background()\n\tdb, owner, err := manualRollbackSQLOpen(&c.sql, filepath.Join(c.root, \"trial\", \"trial.db\"), false)\n\tif err != nil {\n\t\treturn err\n\t}\n\ttx, err := db.Begin(ctx, TxOptions{Immediate: true})\n\tif err != nil {\n\t\treturn err\n\t}\n\tfor _, statement := range []string{\"UPDATE t SET k='green',v='ONE' WHERE id=1\", \"DELETE FROM t WHERE id=2\", \"INSERT INTO t(id,k,v) VALUES(4,'blue','four')\"} {\n\t\tresult, err := tx.Exec(ctx, statement)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tcount, err := result.RowsAffected()\n\t\tif err != nil || count != 1 {\n\t\t\treturn fmt.Errorf(\"DML affected %d: %v\", count, err)\n\t\t}\n\t}\n\tif tx.activeRows != 0 || len(db.rows) != 0 {\n\t\treturn fmt.Errorf(\"writer has live Rows\")\n\t}\n\tscope, err := pager.ArmManualRollbackRestart(tx.storage, c.plan, channel)\n\tif err != nil {\n\t\treturn err\n\t}\n\tcommitErr := tx.Commit(ctx)\n\tcaptured := pager.CaptureManualRollbackRestartCommit(scope, commitErr, tx.done, db.tx == nil, tx.activeRows)\n\tif captured.CommitError != commitErr || captured.Problem != nil || commitErr != nil {\n\t\treturn fmt.Errorf(\"COMMIT_UNCERTAIN: original=%v capture=%v\", commitErr, captured.Problem)\n\t}\n\tif err := channel.CommitReturned(captured); err != nil {\n\t\treturn err\n\t}\n\t// Acknowledged-exit cannot return from CommitReturned successfully. Only\n\t// natural control is allowed to Finish/Disarm and explicitly close its DB.\n\tif c.plan.Spec().Binding.Case != rollbackrestart.Control {\n\t\treturn fmt.Errorf(\"target was not reached\")\n\t}\n\tif err := pager.CompleteManualRollbackRestartControl(scope); err != nil {\n\t\treturn err\n\t}\n\tif err := c.sql.ledger.close(owner); err != nil {\n\t\treturn err\n\t}\n\tif !c.sql.ledger.retired() || !manualRollbackSQLRegistryEmpty() {\n\t\treturn fmt.Errorf(\"OWNER_PENDING: normal writer control\")\n\t}\n\treturn channel.Finished(\"\")\n}", true
	case "manualRestartRecovery":
		return "func manualRestartRecovery(c *manualRestartConfig, channel *pager.ManualRollbackRestartChannel) error {\n\tif c.role != rollbackrestart.Recovery || !manualRestartFreshRegistry() {\n\t\treturn fmt.Errorf(\"fresh recovery registry required\")\n\t}\n\tctx := context.Background()\n\tresult := manualRestartReadbackResult{Binding: c.plan.Spec().Binding, Generation: c.plan.Spec().Recovery.Generation}\n\tresult.Recovery = rollbackrestart.RecoveryEvidence{Generation: result.Generation, ReadOnly: true, NoCreate: true, OpenCalls: 1, CheckMaxPages: 3, CheckMaxPageReads: 512, NoRetry: true}\n\treplayPath := filepath.Join(c.root, \"replay\", \"replay.db\")\n\tjournalStamp, _, err := manualRestartArtifactAt(c, \"replay/replay.db-journal\", 16408)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif c.plan.Spec().Binding.Case == rollbackrestart.AfterTablePage {\n\t\tif !journalStamp.Exists || journalStamp.Bytes != 16408 {\n\t\t\treturn fmt.Errorf(\"exact replay journal required\")\n\t\t}\n\t\tresult.Recovery.JournalPresentBefore = true\n\t\tresult.Recovery.Calls = 1\n\t\tresult.Recovery.Limits = rollbackfault.SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}\n\t\trecovered, recoveryErr := manualRollbackSQLRecoverCopy(ctx, replayPath)\n\t\tif recoveryErr != nil {\n\t\t\treturn fmt.Errorf(\"RECOVERY_FAILED: %w\", recoveryErr)\n\t\t}\n\t\tresult.Recovery.Replayed, result.Recovery.ReplayedPages, result.Recovery.OriginalPages = recovered.Recovered, recovered.RestoredPages, recovered.OriginalPages\n\t\tabsent, _, err := manualRestartArtifactAt(c, \"replay/replay.db-journal\", 16408)\n\t\tif err != nil || absent.Exists {\n\t\t\treturn fmt.Errorf(\"replay journal survived recovery: %v\", err)\n\t\t}\n\t} else if journalStamp.Exists {\n\t\treturn fmt.Errorf(\"unexpected journal; no cleanup/recovery is allowed for this case\")\n\t}\n\tidentity := c.plan.Spec().AdapterIdentity\n\tsnapshot, err := manualRollbackSQLReadback(&c.sql, ctx, replayPath, identity)\n\tif err != nil {\n\t\treturn fmt.Errorf(\"READBACK_FAILED: %w\", err)\n\t}\n\tif problem := rollbackfault.ValidateSQLSnapshot(snapshot, identity, rollbackrestart.ExpectedSQLState(c.plan.Spec().Binding.Case)); problem != rollbackfault.SQLProblemNone {\n\t\treturn fmt.Errorf(\"complete SQL/index result differs from frozen expected state: %v\", problem)\n\t}\n\tresult.Snapshot = snapshot\n\tresult.Recovery.OwnersRetired = c.sql.ledger.retired() && manualRollbackSQLRegistryEmpty()\n\tif !result.Recovery.OwnersRetired {\n\t\treturn fmt.Errorf(\"OWNER_PENDING: recovery readback\")\n\t}\n\traw, err := json.Marshal(result)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 4, \"readback.json\", raw); err != nil {\n\t\treturn err\n\t}\n\tdigest, problem := rollbackrestart.SnapshotDigest(snapshot)\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn fmt.Errorf(\"snapshot digest refused: %v\", problem)\n\t}\n\treturn channel.Finished(digest)\n}", true
	case "manualRestartArtifactEvidence":
		return "func manualRestartArtifactEvidence(c *manualRestartConfig, evidence *rollbackrestart.ScenarioEvidence, after bool) error {\n\tfor i := 0; i < int(rollbackrestart.ArtifactCount); i++ {\n\t\trole := rollbackrestart.ArtifactRole(i)\n\t\tartifact, raw, err := manualRestartArtifactAt(c, rollbackrestart.ArtifactName(role), 1<<20)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tstamp := rollbackrestart.ArtifactStamp{}\n\t\tif artifact.Exists {\n\t\t\tstamp = rollbackrestart.ArtifactStamp{Path: artifact.Path, File: rollbackrestart.FileIdentity{Device: artifact.Device, Inode: artifact.Inode}, Exists: true, Regular: true, Links: 1, Bytes: artifact.Bytes, SHA256: rollbackrestart.Digest(artifact.Hash)}\n\t\t\tif role == rollbackrestart.BaselineArtifact || role == rollbackrestart.TrialArtifact || role == rollbackrestart.PostExitArtifact || role == rollbackrestart.ReplayArtifact {\n\t\t\t\tif len(raw) != 12288 {\n\t\t\t\t\treturn fmt.Errorf(\"three-page artifact required\")\n\t\t\t\t}\n\t\t\t\tfor page := 0; page < 3; page++ {\n\t\t\t\t\tstamp.Pages[page] = manualRestartDigest(raw[page*4096 : (page+1)*4096])\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t\tentry := &evidence.Artifacts[i]\n\t\tentry.Role = role\n\t\tif after {\n\t\t\tentry.After = stamp\n\t\t} else {\n\t\t\tentry.Before = stamp\n\t\t\tif stamp.Exists && (role == rollbackrestart.PostExitArtifact || role == rollbackrestart.PostExitJournalArtifact || role == rollbackrestart.ReplayArtifact || role == rollbackrestart.ReplayJournalArtifact) {\n\t\t\t\tentry.ExclusiveCopy, entry.CompleteRead, entry.CompleteWrite = true, true, true\n\t\t\t\tentry.FileSynced, entry.FileClosedOnce, entry.DirectorySynced, entry.DirectoryClosedOnce = true, true, true, true\n\t\t\t}\n\t\t}\n\t}\n\treturn nil\n}", true
	case "manualRestartFreeze":
		return "func manualRestartFreeze(c *manualRestartConfig, evidence *rollbackrestart.ScenarioEvidence) error {\n\tif c.active == nil || c.active.role != rollbackrestart.Writer || !c.active.waited || c.active.actual.state == nil || !c.active.actual.state.Exited() {\n\t\treturn fmt.Errorf(\"actual writer Wait must precede all original reads/copies\")\n\t}\n\ttrial, raw, err := manualRestartArtifactAt(c, \"trial/trial.db\", 12288)\n\tif err != nil || !trial.Exists || len(raw) != 12288 {\n\t\treturn fmt.Errorf(\"invalid retained trial: %v\", err)\n\t}\n\tjournal, journalRaw, err := manualRestartArtifactAt(c, \"trial/trial.db-journal\", 16408)\n\tif err != nil {\n\t\treturn err\n\t}\n\twhich := c.plan.Spec().Binding.Case\n\tif which == rollbackrestart.BeforeJCreate {\n\t\tif journal.Exists || !bytes.Equal(raw, c.baseline[:]) {\n\t\t\treturn fmt.Errorf(\"pre-create exit must retain exact OLD and no journal\")\n\t\t}\n\t} else if which == rollbackrestart.AfterTablePage {\n\t\tif !journal.Exists || len(journalRaw) != 16408 || bytes.Equal(raw[:4096], c.baseline[:4096]) || bytes.Equal(raw[4096:8192], c.baseline[4096:8192]) || !bytes.Equal(raw[8192:], c.baseline[8192:]) {\n\t\t\treturn fmt.Errorf(\"exact two-written-page prefix and intact journal required\")\n\t\t}\n\t} else if journal.Exists || bytes.Equal(raw, c.baseline[:]) {\n\t\treturn fmt.Errorf(\"completed database bytes and absent journal required\")\n\t}\n\tif err := manualRestartRetain(c, \"trial/trial.db\", 12288); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartRetain(c, \"trial/trial.db-journal\", 16408); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 2, \"post-exit.input.db\", raw); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 3, \"replay.db\", raw); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartRetain(c, \"evidence/post-exit.input.db\", 12288); err != nil {\n\t\treturn err\n\t}\n\tif journal.Exists {\n\t\tif err := manualRestartCreate(c, 2, \"post-exit-journal.input.bin\", journalRaw); err != nil {\n\t\t\treturn err\n\t\t}\n\t\tif err := manualRestartCreate(c, 3, \"replay.db-journal\", journalRaw); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\tif err := manualRestartRetain(c, \"evidence/post-exit-journal.input.bin\", 16408); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartArtifactEvidence(c, evidence, false); err != nil {\n\t\treturn err\n\t}\n\t// Distinct inodes are checked before permitting the recovery process,\n\t// including the mutable replay copies that are not in the frozen ledger.\n\tfor i, a := range evidence.Artifacts {\n\t\tif !a.Before.Exists {\n\t\t\tcontinue\n\t\t}\n\t\tfor j := 0; j < i; j++ {\n\t\t\tif evidence.Artifacts[j].Before.Exists && a.Before.File == evidence.Artifacts[j].Before.File {\n\t\t\t\treturn fmt.Errorf(\"copy aliased an original or another artifact\")\n\t\t\t}\n\t\t}\n\t}\n\treturn manualRestartUnchanged(c)\n}", true
	case "manualRestartDriveWriter":
		return "func manualRestartDriveWriter(t *testing.T, c *manualRestartConfig, receipt *rollbackrestart.Receipt) error {\n\tchild, err := manualRestartSpawn(c, receipt, rollbackrestart.Writer)\n\tif err != nil {\n\t\treturn err\n\t}\n\tready, err := manualRestartReadFrame(c, child, receipt)\n\tif err != nil || ready.Kind != rollbackrestart.ChildReadyBound {\n\t\treturn fmt.Errorf(\"writer ready failed: %v\", err)\n\t}\n\tarm := manualRestartParentFrame(c, receipt, rollbackrestart.ArmGranted)\n\tif err := manualRestartSendFrame(c, child, receipt, arm); err != nil {\n\t\treturn err\n\t}\n\tfor {\n\t\tframe, err := manualRestartReadFrame(c, child, receipt)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tswitch frame.Kind {\n\t\tcase rollbackrestart.TransactionArmed, rollbackrestart.AdapterObserved:\n\t\t\tcontinue\n\t\tcase rollbackrestart.TargetReached:\n\t\t\ttarget, problem := rollbackrestart.FrameDigest(frame)\n\t\t\tif problem != rollbackrestart.ProblemNone {\n\t\t\t\treturn fmt.Errorf(\"target digest refused\")\n\t\t\t}\n\t\t\texit := manualRestartParentFrame(c, receipt, rollbackrestart.ExitGranted)\n\t\t\texit.Digest = target\n\t\t\tif err := manualRestartSendFrame(c, child, receipt, exit); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n\t\t\treturn manualRestartWaitChild(t, c, child, receipt)\n\t\tcase rollbackrestart.PublicCommitReturned:\n\t\t\tif c.plan.Spec().Binding.Case == rollbackrestart.AcknowledgedExit {\n\t\t\t\tack, err := manualRestartAck(c, frame)\n\t\t\t\tif err != nil {\n\t\t\t\t\treturn err\n\t\t\t\t}\n\t\t\t\tdurable := manualRestartParentFrame(c, receipt, rollbackrestart.AcknowledgementDurable)\n\t\t\t\tdurable.Ack = ack\n\t\t\t\tif err := manualRestartSendFrame(c, child, receipt, durable); err != nil {\n\t\t\t\t\treturn err\n\t\t\t\t}\n\t\t\t\texit := manualRestartParentFrame(c, receipt, rollbackrestart.ExitGranted)\n\t\t\t\texit.Ack, exit.Digest = ack, ack.SHA256\n\t\t\t\tif err := manualRestartSendFrame(c, child, receipt, exit); err != nil {\n\t\t\t\t\treturn err\n\t\t\t\t}\n\t\t\t\treturn manualRestartWaitChild(t, c, child, receipt)\n\t\t\t}\n\t\t\tif c.plan.Spec().Binding.Case != rollbackrestart.Control {\n\t\t\t\treturn fmt.Errorf(\"unacknowledged target was not reached\")\n\t\t\t}\n\t\t\tgrant := manualRestartParentFrame(c, receipt, rollbackrestart.NormalFinishGranted)\n\t\t\tgrant.Digest, _ = rollbackrestart.FrameDigest(frame)\n\t\t\tif err := manualRestartSendFrame(c, child, receipt, grant); err != nil {\n\t\t\t\treturn err\n\t\t\t}\n\t\tcase rollbackrestart.WriterFinished:\n\t\t\treturn manualRestartWaitChild(t, c, child, receipt)\n\t\tdefault:\n\t\t\treturn fmt.Errorf(\"closed writer protocol rejected kind %d\", frame.Kind)\n\t\t}\n\t}\n}", true
	case "manualRestartDriveRecovery":
		return "func manualRestartDriveRecovery(t *testing.T, c *manualRestartConfig, receipt *rollbackrestart.Receipt, evidence *rollbackrestart.ScenarioEvidence) error {\n\tchild, err := manualRestartSpawn(c, receipt, rollbackrestart.Recovery)\n\tif err != nil {\n\t\treturn err\n\t}\n\tready, err := manualRestartReadFrame(c, child, receipt)\n\tif err != nil || ready.Kind != rollbackrestart.RecoveryReadyBound {\n\t\treturn fmt.Errorf(\"recovery ready failed: %v\", err)\n\t}\n\tgrant := manualRestartParentFrame(c, receipt, rollbackrestart.RecoveryReadbackGranted)\n\tif err := manualRestartSendFrame(c, child, receipt, grant); err != nil {\n\t\treturn err\n\t}\n\tresult, err := manualRestartReadFrame(c, child, receipt)\n\tif err != nil || result.Kind != rollbackrestart.ReadbackResult {\n\t\treturn fmt.Errorf(\"readback terminal missing: %v\", err)\n\t}\n\tif err := manualRestartWaitChild(t, c, child, receipt); err != nil {\n\t\treturn err\n\t}\n\traw, _, err := manualRestartRead(c, filepath.Join(c.root, \"control\", \"readback.json\"), 65536)\n\tif err != nil {\n\t\treturn err\n\t}\n\tvar readback manualRestartReadbackResult\n\tif err := manualRestartDecode(raw, &readback); err != nil {\n\t\treturn err\n\t}\n\tif readback.Binding != c.plan.Spec().Binding || readback.Generation != c.plan.Spec().Recovery.Generation {\n\t\treturn fmt.Errorf(\"stale/wrong readback binding\")\n\t}\n\tdigest, problem := rollbackrestart.SnapshotDigest(readback.Snapshot)\n\tif problem != rollbackrestart.ProblemNone || digest != result.Digest {\n\t\treturn fmt.Errorf(\"readback artifact differs from child's exact result\")\n\t}\n\tevidence.After, evidence.Recovery = readback.Snapshot, readback.Recovery\n\tif err := manualRestartRetain(c, \"control/readback.json\", 65536); err != nil {\n\t\treturn err\n\t}\n\treturn manualRestartUnchanged(c)\n}", true
	case "manualRestartController":
		return "func manualRestartController(t *testing.T, c *manualRestartConfig) error {\n\tif c.role != rollbackrestart.Controller || !manualRollbackSQLRegistryEmpty() || !c.sql.ledger.retired() {\n\t\treturn fmt.Errorf(\"parent must have no existing database owners\")\n\t}\n\tif err := manualRestartFamilies(c, true); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 0, \"binding.input.json\", c.rawBinding); err != nil {\n\t\treturn err\n\t}\n\tplanRaw, err := json.Marshal(c.plan.Spec())\n\tif err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 4, \"plan.json\", planRaw); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartRetain(c, \"inputs/binding.input.json\", 65536); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartRetain(c, \"control/plan.json\", 65536); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 0, \"baseline.input.db\", c.baseline[:]); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartRetain(c, \"inputs/baseline.input.db\", 12288); err != nil {\n\t\treturn err\n\t}\n\tbefore, err := manualRollbackSQLReadback(&c.sql, context.Background(), filepath.Join(c.root, \"inputs\", \"baseline.input.db\"), c.plan.Spec().AdapterIdentity)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif problem := rollbackfault.ValidateSQLSnapshot(before, c.plan.Spec().AdapterIdentity, rollbackfault.SQLOld); problem != rollbackfault.SQLProblemNone {\n\t\treturn fmt.Errorf(\"baseline SQL/index not literal OLD: %v\", problem)\n\t}\n\tbeforeRaw, err := json.Marshal(before)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 4, \"baseline-sql.json\", beforeRaw); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartRetain(c, \"control/baseline-sql.json\", 65536); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 1, \"trial.db\", c.baseline[:]); err != nil {\n\t\treturn err\n\t}\n\treceipt, problem := rollbackrestart.NewReceipt(c.plan)\n\tc.receipt = &receipt\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn fmt.Errorf(\"receipt admission: %v\", problem)\n\t}\n\tparent := manualRestartParentFrame(c, &receipt, rollbackrestart.ParentAdmitted)\n\tparent.Process = rollbackrestart.ProcessEvidence{PID: int64(os.Getpid()), Image: c.plan.Spec().Image, ImageHash: c.plan.Spec().Binding.Binary, ActualImage: true, State: rollbackrestart.ProcessRunning}\n\tif err := manualRestartAppend(c, &receipt, parent); err != nil {\n\t\treturn err\n\t}\n\tbaseline := manualRestartParentFrame(c, &receipt, rollbackrestart.BaselineCheckedAndClosed)\n\tbaseline.Digest, problem = rollbackrestart.SnapshotDigest(before)\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn fmt.Errorf(\"baseline snapshot digest\")\n\t}\n\tif err := manualRestartAppend(c, &receipt, baseline); err != nil {\n\t\treturn err\n\t}\n\tevidence := rollbackrestart.ScenarioEvidence{Binding: c.plan.Spec().Binding, Before: before, BeforeGeneration: c.plan.Spec().Controller.Generation, AfterGeneration: c.plan.Spec().Recovery.Generation}\n\tc.evidence = &evidence\n\tif err := manualRestartDriveWriter(t, c, &receipt); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartFreeze(c, &evidence); err != nil {\n\t\treturn err\n\t}\n\tfrozen := manualRestartParentFrame(c, &receipt, rollbackrestart.OriginalsFrozen)\n\tfrozen.Digest, problem = rollbackrestart.ArtifactManifestDigest(evidence.Artifacts, false)\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn fmt.Errorf(\"original artifact manifest refused\")\n\t}\n\tif err := manualRestartAppend(c, &receipt, frozen); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartDriveRecovery(t, c, &receipt, &evidence); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartArtifactEvidence(c, &evidence, true); err != nil {\n\t\treturn err\n\t}\n\tchecked := manualRestartParentFrame(c, &receipt, rollbackrestart.ArtifactsRechecked)\n\tchecked.Digest, problem = rollbackrestart.ArtifactManifestDigest(evidence.Artifacts, true)\n\tif problem != rollbackrestart.ProblemNone {\n\t\treturn fmt.Errorf(\"post-readback artifact manifest refused\")\n\t}\n\tif err := manualRestartAppend(c, &receipt, checked); err != nil {\n\t\treturn err\n\t}\n\tcomplete := manualRestartParentFrame(c, &receipt, rollbackrestart.ScenarioEvidenceComplete)\n\tif err := manualRestartAppend(c, &receipt, complete); err != nil {\n\t\treturn err\n\t}\n\tevidence.Lifecycle = rollbackrestart.WriterLifecycle{OpenCalls: 1, BeginCalls: 1, DMLCalls: 3, ArmCalls: 1, CommitCalls: 1, Main: rollbackrestart.AbandonedByExpectedProcessExit, Journal: rollbackrestart.AbandonedByExpectedProcessExit, Cleanup: rollbackrestart.AbandonedByExpectedProcessExit}\n\tswitch c.plan.Spec().Binding.Case {\n\tcase rollbackrestart.BeforeJCreate:\n\t\tevidence.Lifecycle.Journal = rollbackrestart.NotAcquired\n\tcase rollbackrestart.AfterJUnlink:\n\t\tevidence.Lifecycle.Journal = rollbackrestart.RetiredNormally\n\tcase rollbackrestart.Control, rollbackrestart.AcknowledgedExit:\n\t\tevidence.Lifecycle.PublicReturned = true\n\t\tevidence.Lifecycle.Main, evidence.Lifecycle.Journal, evidence.Lifecycle.Cleanup = rollbackrestart.RetiredNormally, rollbackrestart.RetiredNormally, rollbackrestart.RetiredNormally\n\t\tif c.plan.Spec().Binding.Case == rollbackrestart.Control {\n\t\t\tevidence.Lifecycle.FinishCalls, evidence.Lifecycle.DisarmCalls, evidence.Lifecycle.DBCloseCalls = 1, 1, 1\n\t\t}\n\t}\n\tif !c.sql.ledger.retired() || !manualRollbackSQLRegistryEmpty() {\n\t\treturn fmt.Errorf(\"parent owner ledger incomplete\")\n\t}\n\tfor i := range c.family {\n\t\tif !c.family[i].ledger.retired() {\n\t\t\treturn fmt.Errorf(\"artifact owner ledger incomplete\")\n\t\t}\n\t}\n\tevidence.Runtime = rollbackrestart.RuntimeEvidence{EnvFirst: true, ExactRoleSelectors: true, FixedAllowlistEnvironment: true, CountOne: true, ParallelOne: true, CPUOne: true, ShuffleOff: true, TimeoutZero: true, PrivateRoot0700: true, NoSymlinkAncestors: true, RootUnchanged: true, TwoDedicatedPipes: true, PipeOwnersRecorded: true, UnusedPipeEndsClosedOnce: true, BoundedDiagnostics: true, NoInheritedDatabaseOwners: true, ParentOwnersClosedBeforeSpawn: true, FreshRegistryEachChild: true, OnlyParentSpawns: true, NoSignalOrKill: true, NoRetries: true, OriginalsNeverSyncedAfterExit: true, ArtifactsRetained: true, AdapterSameTransaction: true, AdapterSameScope: true, AdapterSameOwners: true, ObserverOutsidePrivateMutex: true, OriginalTxLockHeldAtCut: true, NoCallbackNativeReplacement: true}\n\tobservation := rollbackrestart.Observe(c.plan, receipt, evidence)\n\tif !observation.EvidenceConsistent || !observation.ScenarioObserved || observation.PhysicalValidated() || observation.Pass() {\n\t\treturn fmt.Errorf(\"scenario evidence rejected: %+v\", observation)\n\t}\n\tframes, count := receipt.Frames()\n\toutput := struct {\n\t\tStatus      string\n\t\tPlan        rollbackrestart.PlanSpec\n\t\tFrames      []rollbackrestart.Frame\n\t\tEvidence    rollbackrestart.ScenarioEvidence\n\t\tObservation rollbackrestart.Observation\n\t}{\"SCENARIO_EVIDENCE_COMPLETE_NOT_POWERLOSS_ACCEPTANCE\", c.plan.Spec(), frames[:count], evidence, observation}\n\traw, err := json.Marshal(output)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 4, \"result.json\", raw); err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartUnchanged(c); err != nil {\n\t\treturn err\n\t}\n\t// The final manifest includes every retained input and control artifact,\n\t// including result.json. Its own separately measured identity is held and\n\t// rechecked in memory; a manifest cannot recursively contain its own hash.\n\tfinalManifest, err := json.Marshal(c.retained[:c.retainedCount])\n\tif err != nil {\n\t\treturn err\n\t}\n\tif err := manualRestartCreate(c, 4, \"final-manifest.json\", finalManifest); err != nil {\n\t\treturn err\n\t}\n\treturn manualRestartUnchanged(c)\n}", true
	case "manualRestartFailureSnapshot":
		return "func manualRestartFailureSnapshot(c *manualRestartConfig, cause error) manualRestartFailureRecord {\n\tchild := c.active\n\tdetail := manualRestartFailureRecord{Status: \"INCOMPLETE_RETAINED\", Error: fmt.Sprint(cause), Plan: c.plan.Spec(), ExitCode: -1}\n\tif len(detail.Error) > 4096 {\n\t\tdetail.Error = detail.Error[:4096]\n\t}\n\tif c.receipt != nil {\n\t\tframes, count := c.receipt.Frames()\n\t\tdetail.RecordedFrames = append([]rollbackrestart.Frame(nil), frames[:count]...)\n\t\tfrozenReceipt := *c.receipt\n\t\tobservation := frozenReceipt.Seal()\n\t\tdetail.Commit, detail.ProtocolProblem = observation.Commit, observation.Problem\n\t}\n\tdetail.DurableAck = c.durableAck\n\tif c.evidence != nil && c.evidence.After.Owner != 0 && rollbackfault.ValidateSQLSnapshot(c.evidence.After, c.plan.Spec().AdapterIdentity, rollbackrestart.ExpectedSQLState(c.plan.Spec().Binding.Case)) == rollbackfault.SQLProblemNone {\n\t\tdetail.SQLState = rollbackrestart.ExpectedSQLState(c.plan.Spec().Binding.Case)\n\t}\n\tif child != nil && child.pid != 0 {\n\t\tdetail.Generation, detail.PID, detail.WaitObserved = child.identity.Generation, child.pid, child.waited && child.actual.state != nil\n\t\tdetail.LastGrant, detail.GrantBytes, detail.GrantWritten, detail.GrantError = child.lastGrant, child.grantBytes, child.grantWritten, child.grantError\n\t\tdetail.RawChildPrefix = append([][]byte(nil), child.rawFrames[:child.rawCount]...)\n\t\tif !child.waited {\n\t\t\tdetail.Status = \"CHILD_LIVE_WAIT_UNKNOWN_RETAINED\"\n\t\t} else if child.actual.state != nil {\n\t\t\tdetail.ExitCode = child.actual.state.ExitCode()\n\t\t}\n\t}\n\treturn detail\n}", true
	case "manualRestartPersistFailure":
		return "func manualRestartPersistFailure(t *testing.T, c *manualRestartConfig, cause error) {\n\tif c.failureAttempted {\n\t\treturn\n\t}\n\tc.failureAttempted = true\n\tdetail := manualRestartFailureSnapshot(c, cause)\n\tif c.family[4].root != \"\" {\n\t\traw, err := json.Marshal(detail)\n\t\tif err == nil {\n\t\t\terr = manualRestartCreate(c, 4, \"failure.json\", raw)\n\t\t}\n\t\tif err != nil {\n\t\t\tt.Logf(\"failure evidence write incomplete: %v; originals retained\", err)\n\t\t}\n\t}\n}", true
	case "manualRestartFailure":
		return "func manualRestartFailure(t *testing.T, c *manualRestartConfig, cause error) {\n\tchild := c.active\n\tmanualRestartPersistFailure(t, c, cause)\n\tdetail := manualRestartFailureSnapshot(c, cause)\n\tif child == nil || child.pid == 0 {\n\t\treturn\n\t}\n\tif !child.waited {\n\t\tfailure := rollbackrestart.Frame{Schema: rollbackrestart.SchemaVersion, Binding: c.plan.Spec().Binding, Sender: c.plan.Spec().Controller, Sequence: child.sequence + 1, Kind: rollbackrestart.Failure, Diagnostic: rollbackrestart.ProtocolInvalid}\n\t\traw, problem := rollbackrestart.EncodeFrame(failure)\n\t\tif problem == rollbackrestart.ProblemNone {\n\t\t\tn, err := child.controls.Write(raw)\n\t\t\tif err != nil || n != len(raw) {\n\t\t\t\tt.Logf(\"failure-only control send incomplete: %v; keeping actual child\", err)\n\t\t\t}\n\t\t}\n\t\tactual := manualRestartAwait(t, c, child)\n\t\tdetail.WaitObserved = actual.state != nil\n\t\tif actual.state != nil {\n\t\t\tdetail.ExitCode = actual.state.ExitCode()\n\t\t}\n\t}\n\t// The eventual real Wait gets a separate file; it cannot overwrite or\n\t// promote the original failure to successful scenario evidence.\n\tdetail.Status = \"FAILED_ACTUAL_WAIT_RETAINED\"\n\traw, err := json.Marshal(detail)\n\tif err == nil {\n\t\terr = manualRestartCreate(c, 4, \"failure-wait.json\", raw)\n\t}\n\tif err != nil {\n\t\tt.Logf(\"actual Wait evidence write incomplete: %v\", err)\n\t}\n\tstdout, outOverflow := child.stdout.snapshot()\n\tstderr, errOverflow := child.stderr.snapshot()\n\tstdoutJSON, outErr := json.Marshal(struct {\n\t\tRaw      []byte\n\t\tOverflow bool\n\t}{stdout, outOverflow})\n\tstderrJSON, errErr := json.Marshal(struct {\n\t\tRaw      []byte\n\t\tOverflow bool\n\t}{stderr, errOverflow})\n\tif outErr == nil {\n\t\toutErr = manualRestartCreate(c, 4, \"failure-stdout.json\", stdoutJSON)\n\t}\n\tif errErr == nil {\n\t\terrErr = manualRestartCreate(c, 4, \"failure-stderr.json\", stderrJSON)\n\t}\n\tif err := errors.Join(outErr, errErr); err != nil {\n\t\tt.Logf(\"diagnostic retention: %v\", err)\n\t}\n\traw, err = json.Marshal(child.rawFrames[:child.rawCount])\n\tif err == nil {\n\t\terr = manualRestartCreate(c, 4, \"failure-protocol.json\", raw)\n\t}\n\tif err != nil {\n\t\tt.Logf(\"raw prefix retention: %v\", err)\n\t}\n}", true
	case "TestManualRollbackRestartController":
		return "func TestManualRollbackRestartController(t *testing.T) {\n\tif os.Getenv(\"GOSQLITE_MANUAL_FAULT_ROLLBACK\") != \"independently_authorized\" || os.Getenv(\"GOSQLITE_MANUAL_RESTART_ROLLBACK\") != \"independently_authorized\" {\n\t\tt.Fatal(\"NOT_RUN: separate manual restart authorization required\")\n\t}\n\tvar config manualRestartConfig\n\tif err := manualRestartAdmit(&config, rollbackrestart.Controller); err != nil {\n\t\tt.Fatalf(\"ADMISSION_BLOCKED: %v\", err)\n\t}\n\tif err := manualRestartController(t, &config); err != nil {\n\t\tmanualRestartFailure(t, &config, err)\n\t\tt.Fatalf(\"INCOMPLETE: %v; all artifacts retained\", err)\n\t}\n\tt.Log(\"SCENARIO_EVIDENCE_COMPLETE: process-exit scenario only; PhysicalValidated=false; Pass=false; all artifacts retained\")\n}", true
	case "TestManualRollbackRestartWriter":
		return "func TestManualRollbackRestartWriter(t *testing.T) {\n\tif os.Getenv(\"GOSQLITE_MANUAL_FAULT_ROLLBACK\") != \"independently_authorized\" || os.Getenv(\"GOSQLITE_MANUAL_RESTART_ROLLBACK\") != \"independently_authorized\" {\n\t\tt.Fatal(\"NOT_RUN: separate manual restart authorization required\")\n\t}\n\tvar config manualRestartConfig\n\tif err := manualRestartAdmit(&config, rollbackrestart.Writer); err != nil {\n\t\tt.Fatalf(\"ADMISSION_BLOCKED: %v\", err)\n\t}\n\tif err := manualRestartFamilies(&config, false); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tchannel, err := pager.NewManualRollbackRestartChannel(config.plan, rollbackrestart.Writer)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\timage, err := manualRestartID(config.selfInfo)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif err := channel.Ready(image, config.plan.Spec().Binding.Binary); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif err := manualRestartWriter(&config, channel); err != nil {\n\t\tt.Fatalf(\"INCOMPLETE: %v; no cleanup retry\", err)\n\t}\n}", true
	case "TestManualRollbackRestartRecovery":
		return "func TestManualRollbackRestartRecovery(t *testing.T) {\n\tif os.Getenv(\"GOSQLITE_MANUAL_FAULT_ROLLBACK\") != \"independently_authorized\" || os.Getenv(\"GOSQLITE_MANUAL_RESTART_ROLLBACK\") != \"independently_authorized\" {\n\t\tt.Fatal(\"NOT_RUN: separate manual restart authorization required\")\n\t}\n\tvar config manualRestartConfig\n\tif err := manualRestartAdmit(&config, rollbackrestart.Recovery); err != nil {\n\t\tt.Fatalf(\"ADMISSION_BLOCKED: %v\", err)\n\t}\n\tif err := manualRestartFamilies(&config, false); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tchannel, err := pager.NewManualRollbackRestartChannel(config.plan, rollbackrestart.Recovery)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\timage, err := manualRestartID(config.selfInfo)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif err := channel.Ready(image, config.plan.Spec().Binding.Binary); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif err := manualRestartRecovery(&config, channel); err != nil {\n\t\tt.Fatalf(\"INCOMPLETE: %v; no recovery retry\", err)\n\t}\n}", true
	}
	return "", false
}

func restartGuardLaunchInputClosure(path string, raw []byte, set *token.FileSet, file *ast.File) []string {
	var findings []string
	report := func(message string) { findings = append(findings, path+": rollback restart contract: "+message) }
	exact := map[*ast.FuncDecl]bool{}
	counts := map[string]int{}
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		expected, owned := restartGuardLaunchInputDeclaration(fn.Name.Name)
		if !owned {
			continue
		}
		counts[fn.Name.Name]++
		exact[fn] = restartGuardTokens(oracleNodeText(set, raw, fn)) == restartGuardTokens(expected)
		if !exact[fn] {
			report("launch input complete declaration/effect/return changed: " + fn.Name.Name)
		}
	}
	for _, name := range strings.Fields("manualRestartDigest manualRestartID manualRestartRegular manualRestartRead manualRestartDecode manualRestartFlags manualRestartCheckFlags manualRestartImage manualRestartAdmit manualRestartSQLIdentity manualRestartFamilies manualRestartSyncDirectory manualRestartCreate manualRestartArtifactAt manualRestartRetain manualRestartUnchanged manualRestartAppend manualRestartParentFrame manualRestartPipe manualRestartSpawn manualRestartReadFrame manualRestartSendFrame manualRestartAwait manualRestartWaitChild manualRestartAck manualRestartWriter manualRestartRecovery manualRestartArtifactEvidence manualRestartFreeze manualRestartDriveWriter manualRestartDriveRecovery manualRestartController manualRestartFailureSnapshot manualRestartPersistFailure manualRestartFailure TestManualRollbackRestartController TestManualRollbackRestartWriter TestManualRollbackRestartRecovery") {
		if counts[name] != 1 {
			report("launch input exact declaration inventory changed: " + name)
		}
	}
	// A new holder parameter/local/composite/alias in another existing method
	// cannot evade the fixed set by using a different receiver name. The fixed
	// eight type declarations are separately checked in NativeSemantics.
	parents := oracleParents(file)
	ast.Inspect(file, func(node ast.Node) bool {
		id, ok := node.(*ast.Ident)
		if !ok || (id.Name != "manualRestartConfig" && id.Name != "manualRestartBinding" && id.Name != "manualRestartChild") {
			return true
		}
		for owner := parents[id]; owner != nil; owner = parents[owner] {
			if fn, ok := owner.(*ast.FuncDecl); ok {
				if !exact[fn] {
					report("launch input holder reference outside exact capability closure")
				}
				return true
			}
			if typ, ok := owner.(*ast.TypeSpec); ok {
				if typ.Name.Name != "manualRestartConfig" && typ.Name.Name != "manualRestartBinding" && typ.Name.Name != "manualRestartChild" {
					report("launch input holder/type escape outside exact ownership declarations")
				}
				return true
			}
		}
		report("launch input holder reference outside exact declaration")
		return true
	})
	return findings
}

func restartGuardNativeSemantics(path string, raw []byte, set *token.FileSet, file *ast.File, functions map[string]*ast.FuncDecl) []string {
	findings := restartGuardFunctionInventory(path, file, "Write snapshot manualRestartDigest manualRestartID manualRestartRegular manualRestartRead manualRestartDecode manualRestartFlags manualRestartCheckFlags manualRestartImage manualRestartAdmit manualRestartSQLIdentity manualRestartFamilies manualRestartSyncDirectory manualRestartCreate manualRestartArtifactAt manualRestartRetain manualRestartUnchanged manualRestartAppend manualRestartParentFrame manualRestartPipe manualRestartSpawn manualRestartReadFrame manualRestartSendFrame manualRestartAwait manualRestartWaitChild manualRestartAck manualRestartFreshRegistry manualRestartWriter manualRestartRecovery manualRestartArtifactEvidence manualRestartFreeze manualRestartDriveWriter manualRestartDriveRecovery manualRestartController manualRestartFailureSnapshot manualRestartPersistFailure manualRestartFailure TestManualRollbackRestartController TestManualRollbackRestartWriter TestManualRollbackRestartRecovery")
	report := func(message string) { findings = append(findings, path+": rollback restart contract: "+message) }
	require := func(name, diagnostic string, parts ...string) {
		findings = append(findings, restartGuardRequire(path, raw, set, functions, name, diagnostic, parts...)...)
	}
	typeBodies := map[string]string{
		"manualRestartBinding": `manualRestartBinding struct {
    Schema string
    Case string
    Trial string
    SourceTree string
    SourceManifest string
    SourceManifestPath string
    Tool string
    ToolPath string
    Binary string
    BinaryPath string
    CompileReceipt string
    CompileReceiptPath string
    Filesystem string
    NativeDeclaration string
}`,
		"manualRestartConfig": `manualRestartConfig struct {
    sql manualRollbackSQLConfig
    binding manualRestartBinding
    rawBinding []byte
    root string
    family [5]manualRollbackSQLConfig
    plan rollbackrestart.Plan
    role rollbackrestart.Role
    selfInfo os.FileInfo
    baseline [12288]byte
    retained [24]manualRestartArtifact
    retainedCount int
    active *manualRestartChild
    children [2]*manualRestartChild
    receipt *rollbackrestart.Receipt
    evidence *rollbackrestart.ScenarioEvidence
    durableAck rollbackrestart.AckEvidence
    failureAttempted bool
}`,
		"manualRestartArtifact": `manualRestartArtifact struct {
    Path string
    Device uint64
    Inode uint64
    Bytes int64
    Hash string
    Exists bool
}`,
		"manualRestartDiagnostic": `manualRestartDiagnostic struct {
    mu sync.Mutex
    data [manualRestartLimit]byte
    count int
    overflow bool
}`,
		"manualRestartWaitResult": `manualRestartWaitResult struct { state *os.ProcessState; err error }`,
		"manualRestartChild": `manualRestartChild struct {
    command *exec.Cmd
    role rollbackrestart.Role
    identity rollbackrestart.RoleIdentity
    pid int
    controls *os.File
    events *os.File
    controlOwner int
    eventOwner int
    stdout manualRestartDiagnostic
    stderr manualRestartDiagnostic
    wait chan manualRestartWaitResult
    waited bool
    timedOut bool
    actual manualRestartWaitResult
    frames [rollbackrestart.MaxFrames]rollbackrestart.Frame
    frameCount uint8
    rawFrames [rollbackrestart.MaxFrames][]byte
    rawCount uint8
    lastGrant rollbackrestart.Frame
    grantBytes int
    grantWritten int
    grantError string
    sequence uint8
}`,
		"manualRestartReadbackResult": `manualRestartReadbackResult struct {
    Binding rollbackrestart.Binding
    Generation uint64
    Snapshot rollbackfault.SQLSnapshotEvidence
    Recovery rollbackrestart.RecoveryEvidence
}`,
		"manualRestartFailureRecord": `manualRestartFailureRecord struct {
        Status string
        Error string
        Plan rollbackrestart.PlanSpec
        RecordedFrames []rollbackrestart.Frame
        ProtocolProblem rollbackrestart.Problem
        LastGrant rollbackrestart.Frame
        GrantBytes int
        GrantWritten int
        GrantError string
        RawChildPrefix [][]byte
        Commit rollbackrestart.CommitValue
        DurableAck rollbackrestart.AckEvidence
        SQLState rollbackfault.SQLState
        Generation uint64
        PID int
        WaitObserved bool
        ExitCode int
}`,
	}
	typeCounts := map[string]int{}
	ast.Inspect(file, func(node ast.Node) bool {
		if typ, ok := node.(*ast.TypeSpec); ok {
			typeCounts[typ.Name.Name]++
			expected, exists := typeBodies[typ.Name.Name]
			if !exists || restartGuardTokens(oracleNodeText(set, raw, typ)) != restartGuardTokens(expected) {
				report("native closed ownership type changed or callback/interface/descriptor added")
			}
		}
		return true
	})
	for name := range typeBodies {
		if typeCounts[name] != 1 {
			report("native closed ownership type missing/duplicate: " + name)
		}
	}
	rootGate := `if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" || os.Getenv("GOSQLITE_MANUAL_RESTART_ROLLBACK") != "independently_authorized" { t.Fatal("NOT_RUN: separate manual restart authorization required") }`
	for _, role := range []struct{ root, name string }{{rollbackRestartControllerRoot, "Controller"}, {rollbackRestartWriterRoot, "Writer"}, {rollbackRestartRecoveryRoot, "Recovery"}} {
		if restartGuardSequence(readOnlyWALManualBody(functions, role.root), set, raw, rootGate, `var config manualRestartConfig`, `if err := manualRestartAdmit(&config, rollbackrestart.`+role.name+`); err != nil { t.Fatalf("ADMISSION_BLOCKED: %v", err) }`) != 0 {
			report("root first action must reject both gates then admit exactly its fixed role")
		}
	}
	gate := `if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" || os.Getenv("GOSQLITE_MANUAL_RESTART_ROLLBACK") != "independently_authorized" { return fmt.Errorf("ADMISSION_BLOCKED: independent fault and restart authorization required") }`
	if restartGuardSequence(readOnlyWALManualBody(functions, "manualRestartAdmit"), set, raw, gate, `if err := manualRestartCheckFlags(role); err != nil { return err }`) != 0 {
		report("admission gates must dominate every flag/path/owner/image/process action")
	}
	findings = append(findings, restartGuardExactBody(path, functions, set, raw, "manualRestartFlags", `{ root := ""; switch role { case rollbackrestart.Controller: root = "^TestManualRollbackRestartController$"; case rollbackrestart.Writer: root = "^TestManualRollbackRestartWriter$"; case rollbackrestart.Recovery: root = "^TestManualRollbackRestartRecovery$"; default: return nil, fmt.Errorf("unknown role") }; return []string{"-test.run="+root, "-test.count=1", "-test.shuffle=off", "-test.parallel=1", "-test.timeout=0", "-test.cpu=1", "-test.v=true"}, nil }`, "exact independent single-role test selector and seven fixed flags")...)
	findings = append(findings, restartGuardExactBody(path, functions, set, raw, "manualRestartCheckFlags", `{ want, err := manualRestartFlags(role); if err != nil || len(os.Args) != len(want)+1 { return fmt.Errorf("exact test root and seven flags required") }; for i := range want { if os.Args[i+1] != want[i] { return fmt.Errorf("unexpected flag or flag order") } }; return nil }`, "reject extra/repeated/union/parallel/shuffle/timeout arguments")...)
	require("manualRestartAdmit", "complete source compile tool binding and declared native filesystem", `if os.Getenv("GOSQLITE_RESTART_PRIVATE_ROOT_CONFIRMED") != "private_stable_namespace_retained" || os.Getenv("GOSQLITE_RESTART_ROLE") != fmt.Sprint(uint8(role)) { return fmt.Errorf("role and private retained namespace confirmation required") }`, `rootInfo, err := manualRollbackSQLNamespace(c.root)`, `if manualRestartDigest(raw) != rollbackrestart.Digest(os.Getenv("GOSQLITE_RESTART_BINDING_SHA256")) { return fmt.Errorf("reviewed binding digest mismatch") }`, `if c.binding.Filesystem != fmt.Sprintf("linux-fstype-%x", uint64(filesystem.Type)) { return fmt.Errorf("declared filesystem does not match native root statfs") }`, `if err := syscall.Statfs(c.root, &filesystem); err != nil { return fmt.Errorf("native filesystem identity unavailable: %w", err) }`, `if inventory.Tree != c.binding.SourceTree || len(inventory.Tree) != 40 || len(inventory.Files) < 1 || len(inventory.Files) > 4096 { return fmt.Errorf("complete finite source inventory required") }`, `if err != nil || manualRestartDigest(manifest) != rollbackrestart.Digest(c.binding.SourceManifest)`, `if err != nil || manualRestartDigest(compiled) != rollbackrestart.Digest(c.binding.CompileReceipt)`, `receipt.Package != "./sqlite" || receipt.Tags != "manual_fault_rollback,manual_restart_rollback" || !receipt.CompileOnly || int(receipt.Files) != len(inventory.Files)`, `if err != nil || manualRestartDigest(tool) != rollbackrestart.Digest(c.binding.Tool)`, `if err := manualRestartImage(c); err != nil { return err }`, `spec := rollbackrestart.PlanSpec{Binding: binding, Ticket: 1, Image: imageID}`, `if spec.Binding != binding || manualRestartDigest(rawPlan) != rollbackrestart.Digest(os.Getenv("GOSQLITE_RESTART_PLAN_SHA256"))`)
	require("manualRestartImage", "running /proc/self/exe fstat hash then exact same spawn target", `image, openErr := os.Open("/proc/self/exe")`, `c.sql.ledger.owners[i] = manualRollbackSQLOwner{file: image, path: "/proc/self/exe"}`, `before, statErr := image.Stat()`, `if statErr != nil || before.Size() < 1 || before.Size() > 128<<20 || !before.Mode().IsRegular()`, `raw, readErr := io.ReadAll(io.LimitReader(image, (128<<20)+1))`, `after, afterErr := image.Stat()`, `closeErr := c.sql.ledger.close(i)`, `if int64(len(raw)) != before.Size() || !os.SameFile(before, after) || before.Size() != after.Size() || manualRestartDigest(raw) != rollbackrestart.Digest(c.binding.Binary)`, `if !filepath.IsAbs(c.binding.BinaryPath) || filepath.Clean(c.binding.BinaryPath) != c.binding.BinaryPath`, `target, targetInfo, err := manualRestartRead(c, c.binding.BinaryPath, 128<<20)`, `if err != nil || !os.SameFile(before, targetInfo) || manualRestartDigest(target) != rollbackrestart.Digest(c.binding.Binary)`)
	require("manualRestartDecode", "bounded strict canonical JSON rejects duplicate unknown and trailing data", `if len(raw) == 0 || len(raw) > 4<<20`, `if err := manualRollbackSQLDecode(raw, value); err != nil { return err }`, `canonical, err := json.Marshal(value)`, `if err != nil || !bytes.Equal(raw, canonical)`)
	require("manualRestartRead", "bounded source/artifact inputs reject symlink ancestors", `if !filepath.IsAbs(path) || filepath.Clean(path) != path`, `info, err := os.Lstat(current)`, `if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0`, `raw, info, err := manualRollbackSQLRead(&c.sql.ledger, path, bound)`, `if err := manualRestartRegular(info); err != nil { return nil, nil, err }`)
	require("manualRestartRegular", "only owner-controlled regular single-link files", `if info == nil || !info.Mode().IsRegular()`, `stat, ok := info.Sys().(*syscall.Stat_t)`, `if !ok || stat.Nlink != 1 || stat.Uid != uint32(os.Getuid())`)
	require("manualRestartSpawn", "parent only one actual process per role after closed owners", `if c.role != rollbackrestart.Controller || (role != rollbackrestart.Writer && role != rollbackrestart.Recovery) || !manualRollbackSQLRegistryEmpty() || !c.sql.ledger.retired()`, `if c.children[slot] != nil { return nil, fmt.Errorf("one actual process per fixed role; no retry") }`, `if role == rollbackrestart.Recovery && (c.children[0] == nil || !c.children[0].waited || c.children[0].actual.state == nil || !c.children[0].actual.state.Exited())`, `for i := range c.family { if !c.family[i].ledger.retired() { return nil, fmt.Errorf("family owner live before spawn") } }`, `if err := manualRestartImage(c); err != nil { return nil, err }`, `args, err := manualRestartFlags(role)`, `c.children[slot] = child`, `command := exec.Command(c.binding.BinaryPath, args...)`, `command.Dir = c.root`, `command.ExtraFiles = []*os.File{controlRead, eventWrite}`, `command.Stdout, command.Stderr = &child.stdout, &child.stderr`, `child.command = command`, `c.active = child`, `child.pid = command.Process.Pid`, `closeErr := errors.Join(c.sql.ledger.close(controlReadOwner), c.sql.ledger.close(eventWriteOwner))`)
	require("manualRestartSpawn", "fixed allowlist child environment never inherited PATH/flags/authorization", `command.Env = []string{
        "GOSQLITE_MANUAL_FAULT_ROLLBACK=independently_authorized",
        "GOSQLITE_MANUAL_RESTART_ROLLBACK=independently_authorized",
        "GOSQLITE_RESTART_PRIVATE_ROOT_CONFIRMED=private_stable_namespace_retained",
        "GOSQLITE_RESTART_ROLE="+fmt.Sprint(uint8(role)),
        "GOSQLITE_RESTART_ROOT="+c.root,
        "GOSQLITE_RESTART_BINDING="+filepath.Join(c.root, "inputs", "binding.input.json"),
        "GOSQLITE_RESTART_BINDING_SHA256="+string(manualRestartDigest(c.rawBinding)),
        "GOSQLITE_RESTART_PLAN_SHA256="+string(manualRestartDigest(rawPlan)),
        "GOSQLITE_RESTART_SEQUENCE="+fmt.Sprint(count+1),
    }`)
	require("manualRestartPipe", "two exclusively parent-created directions and immediate owner ledger", `readOwner, err := c.sql.ledger.reserve()`, `writeOwner, err := c.sql.ledger.reserve()`, `read, write, pipeErr := os.Pipe()`, `c.sql.ledger.owners[readOwner] = manualRollbackSQLOwner{file: read, path: "owned restart pipe reader"}`, `c.sql.ledger.owners[writeOwner] = manualRollbackSQLOwner{file: write, path: "owned restart pipe writer"}`)
	require("manualRestartAwait", "timeout retains actual child Wait and never invents terminal", `if child.waited { return child.actual }`, `case actual := <-child.wait:`, `case <-timer.C:`, `child.timedOut = true`, `actual := <-child.wait`, `child.waited, child.actual = true, actual`)
	require("manualRestartWaitChild", "exact observed exit state and diagnostic/trailing-frame completeness", `actual := manualRestartAwait(t, c, child)`, `if child.timedOut`, `if actual.state == nil || actual.state.Pid() != child.pid`, `if child.role == rollbackrestart.Writer && c.plan.Spec().Binding.Case != rollbackrestart.Control { code = rollbackrestart.ExpectedExitCode }`, `if !actual.state.Exited() || actual.state.ExitCode() != code`, `if code == 0 && actual.err != nil`, `if !errors.As(actual.err, &exitError) || exitError.ProcessState != actual.state`, `if n != 0 || tailErr != io.EOF`, `if outOverflow || errOverflow`, `WaitObserved: true, State: rollbackrestart.ProcessExited, ExitCode: code`)
	require("manualRestartReadFrame", "bounded complete event read with retained raw failures and owned child identity", `if child == nil || child.pid == 0 || child.waited || child.frameCount >= rollbackrestart.MaxFrames`, `count := binary.BigEndian.Uint32(header[:])`, `if count == 0 || count > rollbackrestart.MaxFrameBytes`, `raw := make([]byte, 4+int(count))`, `n, readErr := io.ReadFull(child.events, raw[4:])`, `child.rawFrames[child.rawCount] = append([]byte(nil), raw[:4+n]...)`, `frame, problem := rollbackrestart.DecodeFrame(raw)`, `if problem != rollbackrestart.ProblemNone || frame.Sender != child.identity || frame.Binding != c.plan.Spec().Binding || frame.Sequence != child.sequence+1`, `if (frame.Kind == rollbackrestart.ChildReadyBound || frame.Kind == rollbackrestart.RecoveryReadyBound) && frame.Process.PID != int64(child.command.Process.Pid)`, `if err := manualRestartAppend(c, receipt, frame); err != nil { return frame, err }`)
	require("manualRestartSendFrame", "grant validates preview and records actual complete write before receipt publication", `if child == nil || child.pid == 0 || child.waited || frame.Sequence != child.sequence+1`, `preview := *receipt`, `if err := manualRestartAppend(c, &preview, frame); err != nil { return err }`, `raw, problem := rollbackrestart.EncodeFrame(frame)`, `child.lastGrant, child.grantBytes = frame, len(raw)`, `count, err := child.controls.Write(raw)`, `child.grantWritten = count`, `if err == nil && count != len(raw) { err = io.ErrShortWrite }`, `if err != nil { child.grantError = err.Error(); return err }`, `child.grantError = ""`, `*receipt = preview`, `child.sequence = frame.Sequence`)
	if restartGuardSequence(readOnlyWALManualBody(functions, "manualRestartSendFrame"), set, raw, `count, err := child.controls.Write(raw)`, `child.grantWritten = count`, `if err == nil && count != len(raw) { err = io.ErrShortWrite }`, `if err != nil { child.grantError = err.Error(); return err }`, `child.grantError = ""`, `*receipt = preview`, `child.sequence = frame.Sequence`, `return nil`) < 0 {
		report("unsent or partial grant must never enter parent receipt")
	}
	require("manualRestartAck", "only parent owns exclusive raw COMMIT_OK persistence before EXIT", `if c.role != rollbackrestart.Controller || c.plan.Spec().Binding.Case != rollbackrestart.AcknowledgedExit || commit.Kind != rollbackrestart.PublicCommitReturned || commit.Commit.Value != rollbackrestart.CommitNil || commit.Sender != c.plan.Spec().Writer`, `hash, problem := rollbackrestart.FrameDigest(commit)`, `raw, problem := rollbackrestart.EncodeFrame(commit)`, `raw = raw[4:]`, `if err := manualRestartCreate(c, 4, "ack.json", raw); err != nil { return out, fmt.Errorf("ACK_UNCERTAIN: %w", err) }`, `if err != nil || !stamp.Exists || !bytes.Equal(ack, raw)`, `CommitSequence: commit.Sequence, CommitFrameSHA256: hash, SHA256: manualRestartDigest(raw)`, `Exclusive: true, WriteStep: 1, FileSyncStep: 2, FileCloseStep: 3, DirectorySyncStep: 4, DirectoryCloseStep: 5, FileCloseCalls: 1, DirectoryCloseCalls: 1`, `if err := manualRestartRetain(c, "control/ack.json", rollbackrestart.MaxFrameBytes+4); err != nil { return out, err }`, `c.durableAck = out`)
	require("manualRestartCreate", "fixed O_EXCL artifact helper then separate directory sync", `if !allowed[fmt.Sprintf("%d/%s", family, name)]`, `info, err := c.family[family].create(name, raw, false)`, `if err != nil { return err }`, `if err := manualRestartRegular(info); err != nil { return err }`, `if err := manualRestartSyncDirectory(c, family); err != nil { return err }`, `name == "readback.json" || name == "result.json" || name == "final-manifest.json"`, `return manualRestartRetain(c, "control/"+name, 1<<20)`)
	if restartGuardSequence(readOnlyWALManualBody(functions, "manualRestartSyncDirectory"), set, raw, `syncErr := dir.Sync()`, `closeErr := c.sql.ledger.close(i)`, `return errors.Join(syncErr, closeErr, f.stable())`) < 0 {
		report("ack/artifact directory Sync and exactly-once Close must both succeed")
	}
	findings = append(findings, restartGuardExactBody(path, functions, set, raw, "manualRestartFreshRegistry", `{ state := pager.ManualRollbackSQLState(nil); return state.Groups == 0 && state.Pending == 0 && !state.AdapterActive && !state.AdapterUsed }`, "fresh process must have unused, inactive adapter and no owners")...)
	writer := readOnlyWALManualBody(functions, "manualRestartWriter")
	if restartGuardSequence(writer, set, raw, `commitErr := tx.Commit(ctx)`, `captured := pager.CaptureManualRollbackRestartCommit(scope, commitErr, tx.done, db.tx == nil, tx.activeRows)`) < 0 {
		report("one public Commit must be immediately followed by unchanged capture")
	}
	require("manualRestartWriter", "fixed writer transaction and normal-only owner retirement", `if c.role != rollbackrestart.Writer || !manualRestartFreshRegistry()`, `db, owner, err := manualRollbackSQLOpen(&c.sql, filepath.Join(c.root, "trial", "trial.db"), false)`, `tx, err := db.Begin(ctx, TxOptions{Immediate: true})`, `[]string{"UPDATE t SET k='green',v='ONE' WHERE id=1", "DELETE FROM t WHERE id=2", "INSERT INTO t(id,k,v) VALUES(4,'blue','four')"}`, `if err != nil || count != 1`, `if tx.activeRows != 0 || len(db.rows) != 0`, `scope, err := pager.ArmManualRollbackRestart(tx.storage, c.plan, channel)`, `if captured.CommitError != commitErr || captured.Problem != nil || commitErr != nil`, `if c.plan.Spec().Binding.Case != rollbackrestart.Control { return fmt.Errorf("target was not reached") }`, `if err := pager.CompleteManualRollbackRestartControl(scope); err != nil { return err }`, `if err := c.sql.ledger.close(owner); err != nil { return err }`, `return channel.Finished("")`)
	require("manualRestartRecovery", "fresh role one bounded replay copy then frozen SQL/index readback", `if c.role != rollbackrestart.Recovery || !manualRestartFreshRegistry()`, `replayPath := filepath.Join(c.root, "replay", "replay.db")`, `if c.plan.Spec().Binding.Case == rollbackrestart.AfterTablePage`, `if !journalStamp.Exists || journalStamp.Bytes != 16408`, `result.Recovery.Calls = 1`, `result.Recovery.Limits = rollbackfault.SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}`, `recovered, recoveryErr := manualRollbackSQLRecoverCopy(ctx, replayPath)`, `if recoveryErr != nil { return fmt.Errorf("RECOVERY_FAILED: %w", recoveryErr) }`, `snapshot, err := manualRollbackSQLReadback(&c.sql, ctx, replayPath, identity)`, `rollbackfault.ValidateSQLSnapshot(snapshot, identity, rollbackrestart.ExpectedSQLState(c.plan.Spec().Binding.Case))`, `result.Recovery.OwnersRetired = c.sql.ledger.retired() && manualRollbackSQLRegistryEmpty()`, `if err := manualRestartCreate(c, 4, "readback.json", raw); err != nil { return err }`, `return channel.Finished(digest)`)
	require("manualRestartFreeze", "actual writer Wait before preserving exact originals and distinct replay", `if c.active == nil || c.active.role != rollbackrestart.Writer || !c.active.waited || c.active.actual.state == nil || !c.active.actual.state.Exited()`, `trial, raw, err := manualRestartArtifactAt(c, "trial/trial.db", 12288)`, `journal, journalRaw, err := manualRestartArtifactAt(c, "trial/trial.db-journal", 16408)`, `if journal.Exists || !bytes.Equal(raw, c.baseline[:])`, `if !journal.Exists || len(journalRaw) != 16408 || bytes.Equal(raw[:4096], c.baseline[:4096]) || bytes.Equal(raw[4096:8192], c.baseline[4096:8192]) || !bytes.Equal(raw[8192:], c.baseline[8192:])`, `manualRestartCreate(c, 2, "post-exit.input.db", raw)`, `manualRestartCreate(c, 3, "replay.db", raw)`, `manualRestartCreate(c, 2, "post-exit-journal.input.bin", journalRaw)`, `manualRestartCreate(c, 3, "replay.db-journal", journalRaw)`, `if evidence.Artifacts[j].Before.Exists && a.Before.File == evidence.Artifacts[j].Before.File`, `return manualRestartUnchanged(c)`)
	controller := readOnlyWALManualBody(functions, "manualRestartController")
	if restartGuardSequence(controller, set, raw, `if err := manualRestartDriveWriter(t, c, &receipt); err != nil { return err }`, `if err := manualRestartFreeze(c, &evidence); err != nil { return err }`) < 0 {
		report("writer complete/Wait must immediately precede original freeze")
	}
	require("manualRestartController", "closed baseline then writer Wait then independent recovery and model replay", `before, err := manualRollbackSQLReadback(&c.sql, context.Background(), filepath.Join(c.root, "inputs", "baseline.input.db"), c.plan.Spec().AdapterIdentity)`, `rollbackfault.ValidateSQLSnapshot(before, c.plan.Spec().AdapterIdentity, rollbackfault.SQLOld)`, `BeforeGeneration: c.plan.Spec().Controller.Generation, AfterGeneration: c.plan.Spec().Recovery.Generation`, `if err := manualRestartDriveRecovery(t, c, &receipt, &evidence); err != nil { return err }`, `if err := manualRestartArtifactEvidence(c, &evidence, true); err != nil { return err }`, `observation := rollbackrestart.Observe(c.plan, receipt, evidence)`, `if !observation.EvidenceConsistent || !observation.ScenarioObserved || observation.PhysicalValidated() || observation.Pass()`, `if err := manualRestartCreate(c, 4, "result.json", raw); err != nil { return err }`, `return manualRestartUnchanged(c)`)
	require("manualRestartDriveWriter", "target or durable ack bound before EXIT and actual Wait", `child, err := manualRestartSpawn(c, receipt, rollbackrestart.Writer)`, `case rollbackrestart.TargetReached:`, `exit.Digest = target`, `ack, err := manualRestartAck(c, frame)`, `if err != nil { return err }`, `durable.Ack = ack`, `if err := manualRestartSendFrame(c, child, receipt, durable); err != nil { return err }`, `exit.Ack, exit.Digest = ack, ack.SHA256`, `return manualRestartWaitChild(t, c, child, receipt)`)
	require("manualRestartDriveRecovery", "new recovery role result bound to actual natural Wait", `child, err := manualRestartSpawn(c, receipt, rollbackrestart.Recovery)`, `if err != nil || ready.Kind != rollbackrestart.RecoveryReadyBound`, `if err != nil || result.Kind != rollbackrestart.ReadbackResult`, `if err := manualRestartWaitChild(t, c, child, receipt); err != nil { return err }`, `if readback.Binding != c.plan.Spec().Binding || readback.Generation != c.plan.Spec().Recovery.Generation`, `if problem != rollbackrestart.ProblemNone || digest != result.Digest`, `return manualRestartUnchanged(c)`)
	require("manualRestartFailureSnapshot", "retain raw parent prefix and independent Commit ACK SQL Wait knowledge", `Status: "INCOMPLETE_RETAINED"`, `detail.RecordedFrames = append([]rollbackrestart.Frame(nil), frames[:count]...)`, `frozenReceipt := *c.receipt`, `observation := frozenReceipt.Seal()`, `detail.Commit, detail.ProtocolProblem = observation.Commit, observation.Problem`, `detail.DurableAck = c.durableAck`, `rollbackfault.ValidateSQLSnapshot(c.evidence.After, c.plan.Spec().AdapterIdentity, rollbackrestart.ExpectedSQLState(c.plan.Spec().Binding.Case))`, `detail.LastGrant, detail.GrantBytes, detail.GrantWritten, detail.GrantError = child.lastGrant, child.grantBytes, child.grantWritten, child.grantError`, `detail.RawChildPrefix = append([][]byte(nil), child.rawFrames[:child.rawCount]...)`, `if !child.waited { detail.Status = "CHILD_LIVE_WAIT_UNKNOWN_RETAINED" }`)
	require("manualRestartPersistFailure", "one immutable failure receipt without wait or retry", `if c.failureAttempted { return }`, `c.failureAttempted = true`, `detail := manualRestartFailureSnapshot(c, cause)`, `manualRestartCreate(c, 4, "failure.json", raw)`)
	if restartGuardSequence(readOnlyWALManualBody(functions, "manualRestartPersistFailure"), set, raw, `if c.failureAttempted { return }`, `c.failureAttempted = true`, `detail := manualRestartFailureSnapshot(c, cause)`) != 0 {
		report("failure snapshot must be attempted once before any waiting")
	}
	require("manualRestartAwait", "deadline persists uncertain prefix before unbounded actual Wait", `child.timedOut = true`, `manualRestartPersistFailure(t, c, fmt.Errorf("CHILD_LIVE/WAIT_UNKNOWN: actual Wait exceeded observation deadline"))`, `actual := <-child.wait`)
	require("manualRestartFailure", "retain first failure before indefinite actual Wait and never recovery", `manualRestartPersistFailure(t, c, cause)`, `detail := manualRestartFailureSnapshot(c, cause)`, `Kind: rollbackrestart.Failure, Diagnostic: rollbackrestart.ProtocolInvalid`, `actual := manualRestartAwait(t, c, child)`, `detail.Status = "FAILED_ACTUAL_WAIT_RETAINED"`, `manualRestartCreate(c, 4, "failure-wait.json", raw)`, `json.Marshal(child.rawFrames[:child.rawCount])`)
	require("manualRestartController", "actual receipt and evidence retained for failure knowledge", `c.receipt = &receipt`, `c.evidence = &evidence`)
	require("manualRestartController", "recheck complete retained control/result/final manifests", `finalManifest, err := json.Marshal(c.retained[:c.retainedCount])`, `if err := manualRestartCreate(c, 4, "final-manifest.json", finalManifest); err != nil { return err }`)
	require("manualRestartWaitChild", "retain observed trailing bytes before protocol rejection", `if n != 0 && child.rawCount < rollbackrestart.MaxFrames { child.rawFrames[child.rawCount] = append([]byte(nil), trailing[:n]...); child.rawCount++ }`)
	findings = append(findings, restartGuardCommandClosure(path, raw, set, file, functions)...)
	findings = append(findings, restartGuardLaunchInputClosure(path, raw, set, file)...)
	// Role-specific direct-call closure is independent of whole-file shapes.
	parents := oracleParents(file)
	for _, name := range []string{"manualRestartSpawn", "manualRestartAck", "manualRestartWriter", "manualRestartRecovery", "manualRestartController", "manualRestartDriveWriter", "manualRestartDriveRecovery", "manualRestartFreeze", "manualRestartAdmit", "manualRestartFailure"} {
		ast.Inspect(file, func(node ast.Node) bool {
			id, ok := node.(*ast.Ident)
			if !ok || id.Name != name {
				return true
			}
			if fn, ok := parents[id].(*ast.FuncDecl); ok && fn.Name == id {
				return true
			}
			if !restartGuardRoleCalls[restartGuardCallKey(path, id, parents, set, raw)] {
				report("role-specific helper outside exact direct consumer: " + name)
			}
			return true
		})
	}
	for name, want := range map[string]int{"exec.Command": 1, "command.Start": 1, "command.Wait": 1, "os.Pipe": 1, "manualRestartPipe": 2, "tx.Commit": 1, "pager.ArmManualRollbackRestart": 1, "pager.CaptureManualRollbackRestartCommit": 1, "pager.CompleteManualRollbackRestartControl": 1, "manualRollbackSQLRecoverCopy": 1, "manualRollbackSQLReadback": 2, "manualRollbackSQLBaseline": 1, "manualRestartSpawn": 2, "manualRestartAck": 1, "manualRestartAdmit": 3, "pager.NewManualRollbackRestartChannel": 2} {
		if readOnlyWALManualCountCall(file, name) != want {
			report("finite native/helper call count changed: " + name)
		}
	}
	receiptAssignments := 0
	if sender := functions["manualRestartSendFrame"]; sender != nil {
		ast.Inspect(sender.Body, func(node ast.Node) bool {
			if assignment, ok := node.(*ast.AssignStmt); ok {
				for _, left := range assignment.Lhs {
					if rollbackFaultText(set, raw, left) == "*receipt" {
						receiptAssignments++
					}
				}
			}
			return true
		})
	}
	if receiptAssignments != 1 {
		report("unsent or partial grant must never enter parent receipt")
	}
	ast.Inspect(file, func(node ast.Node) bool {
		if deferred, ok := node.(*ast.DeferStmt); ok {
			if rollbackFaultText(set, raw, deferred.Call) != "d.mu.Unlock()" {
				report("native deferred cleanup/retry forbidden")
			}
		}
		return true
	})
	return findings
}

var restartGuardRoleCalls = map[string]bool{
	rollbackRestartNative + "|manualRestartDriveWriter|manualRestartSpawn(c, receipt, rollbackrestart.Writer)":             true,
	rollbackRestartNative + "|manualRestartDriveRecovery|manualRestartSpawn(c, receipt, rollbackrestart.Recovery)":         true,
	rollbackRestartNative + "|manualRestartDriveWriter|manualRestartAck(c, frame)":                                         true,
	rollbackRestartNative + "|TestManualRollbackRestartWriter|manualRestartWriter(&config, channel)":                       true,
	rollbackRestartNative + "|TestManualRollbackRestartRecovery|manualRestartRecovery(&config, channel)":                   true,
	rollbackRestartNative + "|TestManualRollbackRestartController|manualRestartController(t, &config)":                     true,
	rollbackRestartNative + "|manualRestartController|manualRestartDriveWriter(t, c, &receipt)":                            true,
	rollbackRestartNative + "|manualRestartController|manualRestartDriveRecovery(t, c, &receipt, &evidence)":               true,
	rollbackRestartNative + "|manualRestartController|manualRestartFreeze(c, &evidence)":                                   true,
	rollbackRestartNative + "|TestManualRollbackRestartController|manualRestartAdmit(&config, rollbackrestart.Controller)": true,
	rollbackRestartNative + "|TestManualRollbackRestartWriter|manualRestartAdmit(&config, rollbackrestart.Writer)":         true,
	rollbackRestartNative + "|TestManualRollbackRestartRecovery|manualRestartAdmit(&config, rollbackrestart.Recovery)":     true,
	rollbackRestartNative + "|TestManualRollbackRestartController|manualRestartFailure(t, &config, err)":                   true,
}

func restartGuardModelSemantics(path string, raw []byte, set *token.FileSet, file *ast.File, functions map[string]*ast.FuncDecl) []string {
	var findings []string
	report := func(message string) { findings = append(findings, path+": rollback restart contract: "+message) }
	if !restartGuardSignalModelTypes(file, set, raw) {
		report("restart Signal concrete model type chain changed")
	}
	require := func(name, diagnostic string, parts ...string) {
		findings = append(findings, restartGuardRequire(path, raw, set, functions, name, diagnostic, parts...)...)
	}
	var scalar func(ast.Expr) bool
	scalar = func(expression ast.Expr) bool {
		switch value := expression.(type) {
		case *ast.Ident:
			return value.Name != "any" && value.Name != "error" && value.Name != "uintptr" && value.Name != "unsafe"
		case *ast.SelectorExpr:
			qualifier, ok := value.X.(*ast.Ident)
			if !ok || qualifier.Name != "rollbackfault" {
				return false
			}
			return restartGuardImportBound(file, "rollbackfault", rollbackFaultImport)
		case *ast.ArrayType:
			return value.Len != nil && scalar(value.Elt)
		case *ast.StructType:
			for _, field := range value.Fields.List {
				if !scalar(field.Type) {
					return false
				}
			}
			return true
		}
		return false
	}
	constants := map[string]string{"SchemaVersion": "1", "Suite": `"rollback-restart-v1"`, "MaxFrames": "64", "MaxFrameBytes": "4096", "MaxAdapterEvents": "128", "MaxIDLength": "128", "ExpectedExitCode": "86", "AbortExitCode": "87", "CallCount": "18"}
	seen := map[string]int{}
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			if typ, ok := spec.(*ast.TypeSpec); ok && (typ.Assign.IsValid() || !scalar(typ.Type)) {
				report("restart model types must be descriptor-free fixed scalar/SQL-value records")
			}
			if value, ok := spec.(*ast.ValueSpec); ok && gen.Tok == token.CONST {
				for _, name := range value.Names {
					if want, exists := constants[name.Name]; exists {
						seen[name.Name]++
						if len(value.Names) != 1 || len(value.Values) != 1 || rollbackFaultText(set, raw, value.Values[0]) != want {
							report("fixed restart budget/schema/exit constant changed: " + name.Name)
						}
					}
				}
			}
		}
	}
	for name := range constants {
		if seen[name] != 1 {
			report("fixed restart budget/schema/exit constant missing: " + name)
		}
	}
	physical := map[string]int{}
	for _, declaration := range file.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok && (fn.Name.Name == "PhysicalValidated" || fn.Name.Name == "Pass") {
			accepted := false
			for _, receiver := range []string{"Observation", "SuiteObservation"} {
				if restartGuardTokens(oracleNodeText(set, raw, fn)) == restartGuardTokens("func ("+receiver+") "+fn.Name.Name+"() bool { return false }") {
					physical[receiver+"."+fn.Name.Name]++
					accepted = true
				}
			}
			if !accepted {
				report("restart observation cannot confer physical validation or PASS")
			}
		}
	}
	for _, receiver := range []string{"Observation", "SuiteObservation"} {
		for _, name := range []string{"PhysicalValidated", "Pass"} {
			if physical[receiver+"."+name] != 1 {
				report("restart observation cannot confer physical validation or PASS")
			}
		}
	}
	for _, entry := range []struct{ name, body, diagnostic string }{
		{"Cases", `{ return [5]Case{Control, BeforeJCreate, AfterTablePage, AfterJUnlink, AcknowledgedExit} }`, "exact five independent scenarios"},
		{"Target", `{ switch c { case BeforeJCreate: return 1, rollbackfault.Intent; case AfterTablePage: return 14, rollbackfault.Returned; case AfterJUnlink: return 17, rollbackfault.Returned; default: return 0, rollbackfault.EventUnknown } }`, "exact target call/phase catalogue"},
		{"ExpectedAdapterEvents", `{ switch c { case BeforeJCreate: return 2; case AfterTablePage: return 29; case AfterJUnlink: return 35; case Control, AcknowledgedExit: return 37; default: return 0 } }`, "finite adapter prefix length"},
		{"ExpectedSQLState", `{ switch c { case BeforeJCreate, AfterTablePage: return rollbackfault.SQLOld; case Control, AfterJUnlink, AcknowledgedExit: return rollbackfault.SQLNew; default: return rollbackfault.SQLStateUnknown } }`, "exact complete OLD/NEW expectations"},
		{"validWait", `{ want := spawned; want.ActualImage, want.SpawnObserved, want.WaitObserved = false, false, true; want.State, want.ExitCode = ProcessExited, code; return p == want }`, "actual matching process object natural/86 Wait only"},
		{"validAck", `{ return sequence != 0 && a.CommitSequence == sequence && a.CommitFrameSHA256 == raw && validDigest(raw) && a.SHA256 == body && validDigest(body) && validFile(a.File) && a.Bytes == bodyBytes && a.Bytes > 0 && a.Bytes <= MaxFrameBytes && a.Written == a.Bytes && a.Exclusive && a.WriteStep == 1 && a.FileSyncStep == 2 && a.FileCloseStep == 3 && a.DirectorySyncStep == 4 && a.DirectoryCloseStep == 5 && a.FileCloseCalls == 1 && a.DirectoryCloseCalls == 1 && a.ErrorToken == 0 }`, "ack raw commit/body hash and complete exclusive Sync/Close chain"},
		{"validLifecycle", `{ want := WriterLifecycle{OpenCalls: 1, BeginCalls: 1, DMLCalls: 3, ArmCalls: 1, CommitCalls: 1, Main: AbandonedByExpectedProcessExit, Journal: AbandonedByExpectedProcessExit, Cleanup: AbandonedByExpectedProcessExit}; switch which { case BeforeJCreate: want.Journal = NotAcquired; case AfterJUnlink: want.Journal = RetiredNormally; case Control, AcknowledgedExit: want.PublicReturned, want.Main, want.Journal, want.Cleanup = true, RetiredNormally, RetiredNormally, RetiredNormally; if which == Control { want.FinishCalls, want.DisarmCalls, want.DBCloseCalls = 1, 1, 1 } }; return got == want }`, "crash abandonment is never normal Finish/Disarm/Close"},
		{"validRecovery", `{ want := RecoveryEvidence{Generation: p.spec.Recovery.Generation, OwnersRetired: true, ReadOnly: true, NoCreate: true, OpenCalls: 1, CheckMaxPages: 3, CheckMaxPageReads: 512, NoRetry: true}; if p.spec.Binding.Case == AfterTablePage { want.Calls = 1; want.Limits = rollbackfault.SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}; want.JournalPresentBefore, want.Replayed, want.ReplayedPages, want.OriginalPages = true, true, 3, 3 }; return got == want }`, "only hot-journal scenario gets one bounded copy recovery"},
	} {
		findings = append(findings, restartGuardExactBody(path, functions, set, raw, entry.name, entry.body, entry.diagnostic)...)
	}
	findings = append(findings, restartGuardExactBody(path, functions, set, raw, "ExpectedCalls", `{
        return [CallCount]rollbackfault.Call{
            {Point: rollbackfault.JournalCreate, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},
            {Point: rollbackfault.JournalInvalid, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12, Length: 4096},
            {Point: rollbackfault.DBOriginalRead, Occurrence: 1, Role: rollbackfault.Database, Owner: 11, Length: 4096},
            {Point: rollbackfault.JournalRecord, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12, Offset: 4096, Length: 4104},
            {Point: rollbackfault.DBOriginalRead, Occurrence: 2, Role: rollbackfault.Database, Owner: 11, Offset: 4096, Length: 4096},
            {Point: rollbackfault.JournalRecord, Occurrence: 2, Role: rollbackfault.Journal, Owner: 12, Offset: 8200, Length: 4104},
            {Point: rollbackfault.DBOriginalRead, Occurrence: 3, Role: rollbackfault.Database, Owner: 11, Offset: 8192, Length: 4096},
            {Point: rollbackfault.JournalRecord, Occurrence: 3, Role: rollbackfault.Journal, Owner: 12, Offset: 12304, Length: 4104},
            {Point: rollbackfault.JournalBodySync, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},
            {Point: rollbackfault.JournalValidHeader, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12, Length: 12},
            {Point: rollbackfault.JournalHeaderSync, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},
            {Point: rollbackfault.PublishDirectorySync, Occurrence: 1, Role: rollbackfault.Directory, Owner: 13},
            {Point: rollbackfault.DBPage, Occurrence: 1, Role: rollbackfault.Database, Owner: 11, Length: 4096},
            {Point: rollbackfault.DBPage, Occurrence: 2, Role: rollbackfault.Database, Owner: 11, Offset: 4096, Length: 4096},
            {Point: rollbackfault.DBPage, Occurrence: 3, Role: rollbackfault.Database, Owner: 11, Offset: 8192, Length: 4096},
            {Point: rollbackfault.DBSync, Occurrence: 1, Role: rollbackfault.Database, Owner: 11},
            {Point: rollbackfault.JournalUnlink, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},
            {Point: rollbackfault.FinalDirectorySync, Occurrence: 1, Role: rollbackfault.Directory, Owner: 13},
        }
    }`, "exact eighteen original calls bind point occurrence role owner offset length")...)
	require("EncodeFrame", "bounded validated canonical length-prefix encoding", `if problem := ValidateFrame(f); problem != ProblemNone { return nil, problem }`, `body, err := json.Marshal(f)`, `if err != nil || len(body) == 0 || len(body) > MaxFrameBytes { return nil, ProblemEncoding }`, `out := make([]byte, 4+len(body))`, `binary.BigEndian.PutUint32(out[:4], uint32(len(body)))`, `copy(out[4:], body)`)
	require("DecodeFrame", "reject truncated oversized unknown duplicate or trailing frame data", `if len(raw) < 5 || len(raw) > MaxFrameBytes+4 { return Frame{}, ProblemEncoding }`, `n := binary.BigEndian.Uint32(raw[:4])`, `if n == 0 || n > MaxFrameBytes || int(n) != len(raw)-4 { return Frame{}, ProblemEncoding }`, `decoder.DisallowUnknownFields()`, `if err := decoder.Decode(&f); err != nil { return Frame{}, ProblemEncoding }`, `encoded, problem := EncodeFrame(f)`, `if problem != ProblemNone || !bytes.Equal(encoded, raw) { return Frame{}, ProblemEncoding }`)
	require("NewPlan", "every provenance role generation challenge and actual image bound", `if !validBinding(s.Binding) || !validRole(s.Controller) || !validRole(s.Writer) || !validRole(s.Recovery) || s.Controller.Role != Controller || s.Writer.Role != Writer || s.Recovery.Role != Recovery || s.Ticket == 0 || !validFile(s.Image) { return Plan{}, ProblemPlan }`, `if role.Generation == roles[j].Generation || role.Challenge == roles[j].Challenge { return Plan{}, ProblemIdentity }`, `if !validAdapterIdentity(id) || id.Suite != b.Suite || id.Source != b.Source || id.Tool != string(b.Tool) || id.Binary != string(b.Binary) || id.Manifest != string(b.Manifest) || id.Trial != b.Trial || id.Profile != b.Profile { return Plan{}, ProblemIdentity }`)
	require("ValidateFrame", "closed payload schema rejects synthetic terminal adapter events", `if f.Schema != SchemaVersion { return ProblemSchema }`, `if f.Sequence == 0 || int(f.Sequence) > MaxFrames { return ProblemSequence }`, `if f.Kind <= FrameUnknown || f.Kind > Failure { return ProblemPayload }`, `if a.Kind != rollbackfault.RunStarted && a.Kind != rollbackfault.Intent && a.Kind != rollbackfault.Returned { return ProblemPayload }`, `a.Commit != (rollbackfault.CommitResult{}) || a.Owners != (rollbackfault.OwnersResult{}) || a.Validation != rollbackfault.ValidationAbsent`, `f.Schema, f.Binding, f.Sender, f.Sequence, f.Kind = 0, Binding{}, RoleIdentity{}, 0, FrameUnknown`, `if f != (Frame{}) { return ProblemPayload }`)
	require("accept", "linear same-ticket exact adapter prefix and no post-target operation", `if f.Kind != r.next || r.next == FrameUnknown { return ProblemOrder }`, `if f.Ticket != spec.Ticket || f.Adapter != want { return ProblemAdapter }`, `if f.Ticket != spec.Ticket || r.adapterCount < 1 || r.adapterCount >= 37 { return ProblemAdapter }`, `calls := ExpectedCalls()`, `if f.Adapter.Result != want.Result { r.status = UnderlyingIOFailed; return ProblemUnderlyingIO }`, `if f.Adapter != want { return ProblemAdapter }`, `if targetCall == index+1 && targetKind == kind { r.next = TargetReached } else if r.adapterCount == 37 { r.next = PublicCommitReturned }`, `if f.Ticket != spec.Ticket || f.Adapter != r.lastAdapter || r.adapterCount != ExpectedAdapterEvents(which) { return ProblemTarget }`)
	require("accept", "public nil precedes raw-frame durable acknowledgement and exact EXIT", `if f.Commit != want || (which != Control && which != AcknowledgedExit) { r.status = CommitUncertainDiagnostic; return ProblemCommit }`, `r.commitBodyHash, r.commitBodyBytes = digestBytes(raw[4:]), int64(len(raw)-4)`, `if !validAck(f.Ack, r.commitSequence, r.commitHash, r.commitBodyHash, r.commitBodyBytes) { r.status = AckUncertain; return ProblemAck }`, `if f.Ack != r.ack || f.Digest != r.ack.SHA256 || !validDigest(f.Digest) { return ProblemAck }`, `else if which == Control || f.Ack != (AckEvidence{}) || f.Digest != r.targetHash || !validDigest(f.Digest) { return ProblemTarget }`, `r.next = WriterWaitObserved`)
	require("accept", "actual writer Wait before originals and fresh recovery generation", `if !validWait(f.Process, r.writer, code) { r.status = waitStatus(f.Process); return ProblemProcess }`, `if !r.writerWaited || !validDigest(f.Digest) { return ProblemProcess }`, `if !r.writerWaited || !validSpawn(f.Process, spec) || f.Process.Handle == r.writer.Handle || f.Process.PID == r.parent.PID { return ProblemProcess }`, `if !validWait(f.Process, r.recovery, 0) { r.status = waitStatus(f.Process); return ProblemProcess }`, `if !r.writerWaited || !r.recoveryWaited || r.adapterCount != ExpectedAdapterEvents(which) { return ProblemIncomplete }`)
	require("validateArtifacts", "immutable originals unaliased complete copies and retained ack", `if !validStamp(a.Before, role) || a.Before.File == p.spec.Binding.Root || a.Before.File == p.spec.Image { return false }`, `if role != ReplayArtifact && a.Before != a.After { return false }`, `if role == ReplayArtifact && which != AfterTablePage && a.Before != a.After { return false }`, `if other.Before.Exists && other.Before.File == a.Before.File { return false }`, `if baseline.SHA256 != p.spec.Binding.Baseline || !sameContent(trial, artifacts[PostExitArtifact].Before) || !sameContent(trial, artifacts[ReplayArtifact].Before) { return false }`, `if trial.Pages[2] != baseline.Pages[2] { return false }`, `if a.File != r.ack.File || a.SHA256 != r.ack.SHA256 || a.Bytes != r.ack.Bytes { return false }`, `return problem == ProblemNone && after == r.checkedDigest`)
	require("Observe", "replay raw receipt then conjunctive SQL/index artifact lifecycle evidence", `if e.PhysicalClaim { return bad(ProblemPhysicalClaim, ProtocolInvalid) }`, `if !p.valid || !supplied.plan.valid || p.spec != supplied.plan.spec || supplied.count > MaxFrames { return bad(ProblemPlan, ProtocolInvalid) }`, `for i := uint8(0); i < supplied.count; i++ { if r.Append(supplied.frames[i]) != ProblemNone { break } }`, `o := r.Seal()`, `if supplied.problem != ProblemNone`, `if e.Binding != p.spec.Binding || e.BeforeGeneration != p.spec.Controller.Generation || e.AfterGeneration != p.spec.Recovery.Generation`, `if !validLifecycle(p.spec.Binding.Case, e.Lifecycle) || !validRuntime(e.Runtime)`, `if !validRecovery(p, e.Recovery)`, `if !validateArtifacts(p, r, e)`, `rollbackfault.ValidateSQLSnapshot(e.Before, p.spec.AdapterIdentity, rollbackfault.SQLOld)`, `rollbackfault.ValidateSQLSnapshot(e.After, p.spec.AdapterIdentity, state)`, `if problem != ProblemNone || before != r.baselineDigest`, `if problem != ProblemNone || after != r.readbackDigest`)
	return findings
}

func inspectRollbackRestartWiring(raw []byte) []string {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "internal/testscope/main.go", raw, 0)
	if err != nil {
		return []string{"restart scanner wiring parse failure"}
	}
	functions := readOnlyWALManualFunctions(file)
	body := readOnlyWALManualBody(functions, "main")
	var walk *ast.BlockStmt
	if body != nil {
		ast.Inspect(body, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok && readOnlyWALManualName(call.Fun) == "filepath.WalkDir" && len(call.Args) == 2 {
				if fn, ok := call.Args[1].(*ast.FuncLit); ok {
					walk = fn.Body
				}
			}
			return true
		})
	}
	var findings []string
	report := func(message string) { findings = append(findings, "restart scanner wiring: "+message) }
	index := restartGuardSequence(walk, set, raw, `restart, restartErr := inspectRollbackRestartSource(path, raw)`, `if restartErr != nil { return restartErr }`, `findings = append(findings, restart...)`)
	if index < 0 {
		report("direct all-source hook must propagate errors and findings")
	}
	if walk != nil {
		for i, statement := range walk.List {
			if conditional, ok := statement.(*ast.IfStmt); ok && (readOnlyWALManualCompact(conditional.Cond) == `strings.HasSuffix(path,"_test.go")` || readOnlyWALManualCompact(conditional.Cond) == "oracleCurrentGoPath(path)") && index >= i {
				report("restart scanner must precede existing platform/test filters")
			}
		}
	}
	if restartGuardSequence(body, set, raw, `restartContracts, restartContractErr := inspectRollbackRestartContracts(".")`, `if restartContractErr != nil { fmt.Fprintln(os.Stderr, restartContractErr); failed = true }`, `for _, finding := range restartContracts { fmt.Fprintln(os.Stderr, finding); failed = true }`) < 0 {
		report("restart contract errors and findings must reach failure bit")
	}
	for _, name := range []string{"inspectRollbackRestartSource", "inspectRollbackRestartContracts", "inspectRollbackFaultSQLSource", "inspectRollbackFaultSQLContracts", "inspectSource", "inspectBackupManualSource", "inspectBackupManualContracts", "inspectWALPublicationSource", "inspectWALPublicationContracts", "inspectOracleSource", "inspectOracleContracts", "inspectDarwinInventoryManualContract", "inspectFormatUpgradeSource", "inspectFormatUpgradeContracts", "inspectRollbackFaultSource", "inspectRollbackFaultContracts", "inspectReadOnlyWALManualSource", "inspectReadOnlyWALManualContracts"} {
		if readOnlyWALManualCountCall(body, name) != 1 {
			report("missing/duplicate prior or restart scanner hook: " + name)
		}
	}
	findings = append(findings, readOnlyWALManualPreviousWiring(body, walk)...)
	ordinary := functions["inspectSource"]
	if ordinary == nil || fmt.Sprintf("%x", sha256.Sum256([]byte(oracleNodeText(set, raw, ordinary)))) != readOnlyWALManualPriorOrdinarySHA256 {
		report("previous reviewed ordinary policy body changed")
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != walPublicationFrozenSources["internal/testscope/main.go"] {
		report(rollbackRestartMainReview)
	}
	return findings
}

func inspectRollbackRestartContracts(root string) ([]string, error) {
	var findings []string
	found := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		items, err := inspectRollbackRestartSource(relative, raw)
		if err != nil {
			return err
		}
		findings = append(findings, items...)
		if restartGuardOwned(relative) {
			found[relative] = true
		}
		return nil
	})
	if err != nil {
		return findings, err
	}
	for path := range rollbackRestartPins {
		if !found[path] {
			findings = append(findings, path+": missing exact restart inventory source")
		}
	}
	return findings, nil
}
