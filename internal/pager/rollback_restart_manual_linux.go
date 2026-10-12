//go:build linux && manual_fault_rollback && manual_restart_rollback

package pager

// SOURCE ONLY / REVIEW HOLD. This bridge has one concrete pipe transport and
// one lifetime scope. It is not a callback, VFS replacement, or command runner.
import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"

	"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackrestart"
	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

type ManualRollbackRestartChannel struct {
	plan         rollbackrestart.Plan
	role         rollbackrestart.RoleIdentity
	sequence     uint8
	control      *os.File
	events       *os.File
	controlClose uint8
	eventsClose  uint8
	problem      error
	frameCount   uint8
	ready        bool
	complete     bool
}

type manualRollbackRestartScope struct {
	fault          *rollbackFaultScope
	channel        *ManualRollbackRestartChannel
	count          uint8
	target         bool
	commitCaptured bool
	commitError    error
}

var rollbackRestartRegistry struct {
	mu     sync.Mutex
	used   bool
	active *manualRollbackRestartScope
}

// Adoption is fixed to the controller-created ExtraFiles mapping: fd3 is the
// child control reader; fd4 is the child event writer. Arbitrary files/FDs and
// callbacks are never accepted. All closure is explicit, checked and once.
func NewManualRollbackRestartChannel(plan rollbackrestart.Plan, role rollbackrestart.Role) (*ManualRollbackRestartChannel, error) {
	if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" || os.Getenv("GOSQLITE_MANUAL_RESTART_ROLLBACK") != "independently_authorized" {
		return nil, errRollbackFaultGate
	}
	spec := plan.Spec()
	verified, problem := rollbackrestart.NewPlan(spec)
	if problem != rollbackrestart.ProblemNone || verified.Spec() != spec {
		return nil, errRollbackFaultScope
	}
	var identity rollbackrestart.RoleIdentity
	switch role {
	case rollbackrestart.Writer:
		identity = spec.Writer
	case rollbackrestart.Recovery:
		identity = spec.Recovery
	default:
		return nil, errRollbackFaultScope
	}
	if os.Getenv("GOSQLITE_RESTART_ROLE") != fmt.Sprint(uint8(role)) {
		return nil, errRollbackFaultScope
	}
	sequence, err := strconv.ParseUint(os.Getenv("GOSQLITE_RESTART_SEQUENCE"), 10, 8)
	if err != nil || sequence == 0 || sequence >= rollbackrestart.MaxFrames {
		return nil, errRollbackFaultScope
	}
	channel := &ManualRollbackRestartChannel{plan: plan, role: identity, sequence: uint8(sequence)}
	channel.control = os.NewFile(3, "restart-parent-control")
	channel.events = os.NewFile(4, "restart-child-events")
	if channel.control == nil || channel.events == nil {
		return channel, errRollbackFaultScope
	}
	controlInfo, controlErr := channel.control.Stat()
	eventInfo, eventErr := channel.events.Stat()
	if controlErr != nil || eventErr != nil || controlInfo.Mode()&os.ModeNamedPipe == 0 || eventInfo.Mode()&os.ModeNamedPipe == 0 || vfs.SameFile(controlInfo, eventInfo) {
		return channel, errors.Join(errRollbackFaultScope, controlErr, eventErr)
	}
	return channel, nil
}

func (c *ManualRollbackRestartChannel) frame(kind rollbackrestart.FrameKind) rollbackrestart.Frame {
	return rollbackrestart.Frame{Schema: rollbackrestart.SchemaVersion, Binding: c.plan.Spec().Binding, Sender: c.role, Sequence: c.sequence + 1, Kind: kind}
}
func (c *ManualRollbackRestartChannel) send(frame rollbackrestart.Frame) error {
	if c.problem != nil || c.complete || c.frameCount >= rollbackrestart.MaxFrames || frame.Sequence != c.sequence+1 || frame.Sender != c.role || frame.Binding != c.plan.Spec().Binding {
		return errRollbackFaultScope
	}
	raw, problem := rollbackrestart.EncodeFrame(frame)
	if problem != rollbackrestart.ProblemNone {
		c.problem = errRollbackFaultScope
		return c.problem
	}
	n, err := c.events.Write(raw)
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	if err != nil {
		c.problem = err
		return err
	}
	c.sequence, c.frameCount = frame.Sequence, c.frameCount+1
	return nil
}
func (c *ManualRollbackRestartChannel) receive() (rollbackrestart.Frame, error) {
	if c.problem != nil || c.complete || c.frameCount >= rollbackrestart.MaxFrames {
		return rollbackrestart.Frame{}, errRollbackFaultScope
	}
	var size [4]byte
	if _, err := io.ReadFull(c.control, size[:]); err != nil {
		c.problem = err
		return rollbackrestart.Frame{}, err
	}
	count := binary.BigEndian.Uint32(size[:])
	if count == 0 || count > rollbackrestart.MaxFrameBytes {
		c.problem = errRollbackFaultScope
		return rollbackrestart.Frame{}, c.problem
	}
	raw := make([]byte, 4+int(count))
	copy(raw, size[:])
	if _, err := io.ReadFull(c.control, raw[4:]); err != nil {
		c.problem = err
		return rollbackrestart.Frame{}, err
	}
	frame, problem := rollbackrestart.DecodeFrame(raw)
	if problem != rollbackrestart.ProblemNone || frame.Binding != c.plan.Spec().Binding || frame.Sender != c.plan.Spec().Controller || frame.Sequence != c.sequence+1 {
		c.problem = errRollbackFaultScope
		return rollbackrestart.Frame{}, c.problem
	}
	c.sequence, c.frameCount = frame.Sequence, c.frameCount+1
	return frame, nil
}

func (c *ManualRollbackRestartChannel) Ready(actualImage rollbackrestart.FileIdentity, imageHash rollbackrestart.Digest) error {
	if c.ready || actualImage != c.plan.Spec().Image || imageHash != c.plan.Spec().Binding.Binary {
		return errRollbackFaultScope
	}
	kind := rollbackrestart.ChildReadyBound
	if c.role.Role == rollbackrestart.Recovery {
		kind = rollbackrestart.RecoveryReadyBound
	}
	frame := c.frame(kind)
	frame.Process = rollbackrestart.ProcessEvidence{PID: int64(os.Getpid()), Handle: uint64(c.role.Role - 1), Image: actualImage, ImageHash: imageHash, ActualImage: true, State: rollbackrestart.ProcessRunning}
	if err := c.send(frame); err != nil {
		return err
	}
	grant, err := c.receive()
	if err != nil {
		return err
	}
	expected := rollbackrestart.ArmGranted
	if c.role.Role == rollbackrestart.Recovery {
		expected = rollbackrestart.RecoveryReadbackGranted
	}
	if grant.Kind != expected {
		return errRollbackFaultScope
	}
	c.ready = true
	return nil
}

func ArmManualRollbackRestart(tx *Tx, plan rollbackrestart.Plan, channel *ManualRollbackRestartChannel) (*manualRollbackRestartScope, error) {
	if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" || os.Getenv("GOSQLITE_MANUAL_RESTART_ROLLBACK") != "independently_authorized" {
		return nil, errRollbackFaultGate
	}
	if channel == nil || !channel.ready || channel.role.Role != rollbackrestart.Writer || channel.plan.Spec() != plan.Spec() || tx == nil {
		return nil, errRollbackFaultScope
	}
	faultPlan, problem := rollbackfault.SQLPlan(plan.Spec().AdapterIdentity, rollbackfault.SQLControl)
	if problem != rollbackfault.NoProblem {
		return nil, errRollbackFaultScope
	}
	// Reuse the reviewed armer's exact three-page, ordinary rollback and one
	// healthy independent Tx restrictions. Its fixed action is Pass only.
	oldScope, err := ArmManualRollbackSQLFault(tx, faultPlan)
	if err != nil {
		return nil, err
	}
	scope := &manualRollbackRestartScope{fault: oldScope.scope, channel: channel}
	rollbackRestartRegistry.mu.Lock()
	if rollbackRestartRegistry.used || rollbackRestartRegistry.active != nil {
		rollbackRestartRegistry.mu.Unlock()
		return nil, errRollbackFaultScope
	}
	rollbackRestartRegistry.used, rollbackRestartRegistry.active = true, scope
	rollbackRestartRegistry.mu.Unlock()
	scope.fault.mu.Lock()
	events, count := scope.fault.receipt.Events()
	problemErr := scope.fault.problem
	scope.fault.mu.Unlock()
	if problemErr != nil || count != 1 || events[0].Kind != rollbackfault.RunStarted {
		return nil, errRollbackFaultScope
	}
	frame := channel.frame(rollbackrestart.TransactionArmed)
	frame.Adapter, frame.Ticket = events[0], plan.Spec().Ticket
	if err := channel.send(frame); err != nil {
		return nil, err
	}
	scope.count = 1
	return scope, nil
}

func rollbackRestartBefore(ticket rollbackFaultTicket) {
	rollbackRestartObserve(ticket, rollbackfault.Intent)
}
func rollbackRestartAfter(ticket rollbackFaultTicket) {
	rollbackRestartObserve(ticket, rollbackfault.Returned)
}

// Called only after the core has released the adapter mutex. The original Tx
// mutex remains held by Commit intentionally. Never call a Tx-locking snapshot
// or any native owner method here; copy only the already recorded scalar event.
func rollbackRestartObserve(ticket rollbackFaultTicket, kind rollbackfault.EventKind) {
	rollbackRestartRegistry.mu.Lock()
	scope := rollbackRestartRegistry.active
	rollbackRestartRegistry.mu.Unlock()
	if scope == nil {
		return
	}
	if ticket.scope == nil || ticket.scope != scope.fault || ticket.action != rollbackfault.Pass || scope.target || scope.commitCaptured {
		rollbackRestartAbort()
	}
	fault := scope.fault
	fault.mu.Lock()
	events, count := fault.receipt.Events()
	problemErr := fault.problem
	diagnostics := fault.diagnostics
	fault.mu.Unlock()
	if problemErr != nil || count != scope.count+1 || count == 0 {
		rollbackRestartDiagnostic(problemErr, diagnostics)
		rollbackRestartAbort()
	}
	event := events[count-1]
	if event.Kind != kind || event.Call != ticket.call || event.Identity != scope.channel.plan.Spec().AdapterIdentity || event.Action != rollbackfault.Pass {
		rollbackRestartAbort()
	}
	frame := scope.channel.frame(rollbackrestart.AdapterObserved)
	frame.Adapter, frame.Ticket = event, scope.channel.plan.Spec().Ticket
	if err := scope.channel.send(frame); err != nil {
		rollbackRestartTransport(err)
		rollbackRestartDiagnostic(problemErr, diagnostics)
		rollbackRestartAbort()
	}
	scope.count = count
	// Native failure remains an actual failed observation, never an expected
	// target. Abort code differs from 86 and parent retains the event prefix.
	if kind == rollbackfault.Returned && (!event.Result.Executed || event.Result.NativeError != rollbackfault.None || event.Result.ReturnedError != rollbackfault.None || event.Result.NormalizedError != rollbackfault.None || event.Result.NativeN != event.Call.Length || event.Result.ReturnedN != event.Call.Length) {
		rollbackRestartDiagnostic(problemErr, diagnostics)
		rollbackRestartAbort()
	}
	which := scope.channel.plan.Spec().Binding.Case
	target := (which == rollbackrestart.BeforeJCreate && kind == rollbackfault.Intent && ticket.call.Point == rollbackfault.JournalCreate && ticket.call.Occurrence == 1) || (which == rollbackrestart.AfterTablePage && kind == rollbackfault.Returned && ticket.call.Point == rollbackfault.DBPage && ticket.call.Occurrence == 2 && ticket.call.Offset == 4096 && ticket.call.Length == 4096) || (which == rollbackrestart.AfterJUnlink && kind == rollbackfault.Returned && ticket.call.Point == rollbackfault.JournalUnlink && ticket.call.Occurrence == 1)
	if !target {
		return
	}
	scope.target = true
	reached := scope.channel.frame(rollbackrestart.TargetReached)
	reached.Adapter, reached.Ticket = event, scope.channel.plan.Spec().Ticket
	if err := scope.channel.send(reached); err != nil {
		rollbackRestartTransport(err)
		rollbackRestartDiagnostic(problemErr, diagnostics)
		rollbackRestartAbort()
	}
	targetHash, problem := rollbackrestart.FrameDigest(reached)
	if problem != rollbackrestart.ProblemNone {
		rollbackRestartTransport(errRollbackFaultScope)
		rollbackRestartDiagnostic(problemErr, diagnostics)
		rollbackRestartAbort()
	}
	grant, err := scope.channel.receive()
	if err != nil || grant.Kind != rollbackrestart.ExitGranted || grant.Digest != targetHash || grant.Ack != (rollbackrestart.AckEvidence{}) {
		rollbackRestartTransport(errors.Join(errRollbackFaultScope, err))
		rollbackRestartDiagnostic(problemErr, diagnostics)
		rollbackRestartAbort()
	}
	// No return, panic, defer, Finish, Disarm, DB.Close or rollback happens at
	// this precise original-call boundary. This source has never been run.
	os.Exit(86)
}

type ManualRollbackRestartCommit struct {
	Evidence    rollbackrestart.CommitEvidence
	CommitError error
	Problem     error
}

// The public caller's next statement passes the untouched Commit error here.
// A crash prefix never reaches this function; no terminal facts are invented.
func CaptureManualRollbackRestartCommit(scope *manualRollbackRestartScope, commitErr error, done bool, detached bool, activeRows int) ManualRollbackRestartCommit {
	out := ManualRollbackRestartCommit{CommitError: commitErr}
	if scope == nil || scope.target || scope.commitCaptured {
		out.Problem = errRollbackFaultScope
		return out
	}
	scope.commitCaptured, scope.commitError = true, commitErr
	fault := scope.fault
	fault.tx.mu.Lock()
	fault.mu.Lock()
	events, count := fault.receipt.Events()
	healthy := fault.problem == nil && !fault.inFlight && !fault.finalized && count == 37 && scope.count == count && fault.tx.file == nil && fault.tx.encrypted == nil
	fault.mu.Unlock()
	fault.tx.mu.Unlock()
	if !healthy || !done || !detached || activeRows != 0 || commitErr != nil || events[count-1].Kind != rollbackfault.Returned || events[count-1].Call.Point != rollbackfault.FinalDirectorySync {
		out.Problem = errRollbackFaultScope
		return out
	}
	if !fault.main.RetirementComplete() || fault.journal == nil || !fault.journal.RetirementComplete() {
		out.Problem = errRollbackFaultScope
		return out
	}
	state := ManualRollbackSQLState(nil)
	if state.Groups != 0 || state.Pending != 0 {
		out.Problem = errRollbackFaultScope
		return out
	}
	out.Evidence = rollbackrestart.CommitEvidence{Calls: 1, Value: rollbackrestart.CommitNil, CapturedAdjacent: true, CapturedUnchanged: true, PublicDone: done, PublicDetached: detached, ActiveRows: uint32(activeRows), Main: rollbackfault.OwnerRetired, Journal: rollbackfault.OwnerRetired, Cleanup: rollbackfault.OwnerRetired}
	return out
}

func (c *ManualRollbackRestartChannel) CommitReturned(captured ManualRollbackRestartCommit) error {
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
}

func CompleteManualRollbackRestartControl(scope *manualRollbackRestartScope) error {
	if scope == nil || !scope.commitCaptured || scope.commitError != nil || scope.target || scope.channel.plan.Spec().Binding.Case != rollbackrestart.Control {
		return errRollbackFaultScope
	}
	report := finishRollbackFault(scope.fault, scope.commitError)
	if report.Problem != nil || report.CommitError != nil || report.Observation.Outcome != rollbackfault.AdapterObserved {
		return errRollbackFaultScope
	}
	return disarmRollbackFault(scope.fault)
}
func (c *ManualRollbackRestartChannel) Finished(digest rollbackrestart.Digest) error {
	kind := rollbackrestart.WriterFinished
	if c.role.Role == rollbackrestart.Recovery {
		kind = rollbackrestart.ReadbackResult
	}
	if c.role.Role == rollbackrestart.Writer && c.plan.Spec().Binding.Case != rollbackrestart.Control {
		return errRollbackFaultScope
	}
	frame := c.frame(kind)
	frame.Digest = digest
	if err := c.send(frame); err != nil {
		return err
	}
	c.complete = true
	return c.CloseNormal()
}
func (c *ManualRollbackRestartChannel) CloseNormal() error {
	if c == nil || c.controlClose != 0 || c.eventsClose != 0 {
		return errRollbackFaultScope
	}
	c.controlClose++
	controlErr := c.control.Close()
	c.eventsClose++
	eventErr := c.events.Close()
	return errors.Join(controlErr, eventErr)
}

// Diagnostics preserve original error text alongside its event token; they are
// bounded, independent from protocol and can never grant a successful exit.
func rollbackRestartDiagnostic(problem error, diagnostics [rollbackfault.MaxEvents]error) {
	if problem != nil {
		fmt.Fprintln(os.Stderr, "ADAPTER_PROBLEM:", problem)
	}
	for ordinal, diagnostic := range diagnostics {
		if diagnostic == nil {
			continue
		}
		text := diagnostic.Error()
		if len(text) > 384 {
			text = text[:384] + " [TRUNCATED_DIAGNOSTIC_FAILURE]"
		}
		fmt.Fprintf(os.Stderr, "ORIGINAL_ERROR_TOKEN=%d text=%s\n", ordinal+1, text)
	}
}
func rollbackRestartTransport(err error) {
	text := fmt.Sprint(err)
	if len(text) > 384 {
		text = text[:384] + " [TRUNCATED_TRANSPORT_FAILURE]"
	}
	fmt.Fprintln(os.Stderr, "ORIGINAL_TRANSPORT_ERROR:", text)
}
func rollbackRestartAbort() { os.Exit(87) }
