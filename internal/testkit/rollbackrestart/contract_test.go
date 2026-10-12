package rollbackrestart

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"
)

// SOURCE ONLY. These descriptor-free synthetic values make no SQL, fixture,
// process, native owner, journal, recovery, or physical durability observation.
func testDigest(label string) Digest { return digestBytes([]byte("synthetic:" + label)) }
func testPlan(t *testing.T, which Case) Plan {
	t.Helper()
	b := Binding{Suite: Suite, Profile: rollbackfault.PlainRollback4096, Case: which, Trial: "trial-" + CaseName(which), Source: "frozen-source", Manifest: testDigest("manifest"), Tool: testDigest("tool"), Compile: testDigest("compile"), Binary: testDigest("binary"), Native: NativeIdentity{OS: "linux", Filesystem: "qualified-filesystem", SectorSize: 4096, PageSize: 4096}, Root: FileIdentity{Device: 1, Inode: 100 + uint64(which)}, Baseline: testDigest("old-database")}
	id := rollbackfault.Identity{Suite: b.Suite, Source: b.Source, Tool: string(b.Tool), Binary: string(b.Binary), Manifest: string(b.Manifest), Trial: b.Trial, Scope: "writer-scope", Profile: b.Profile}
	spec := PlanSpec{Binding: b, Controller: RoleIdentity{Role: Controller, Generation: 1, Challenge: testDigest(b.Trial + "-controller")}, Writer: RoleIdentity{Role: Writer, Generation: 2, Challenge: testDigest(b.Trial + "-writer")}, Recovery: RoleIdentity{Role: Recovery, Generation: 3, Challenge: testDigest(b.Trial + "-recovery")}, AdapterIdentity: id, Ticket: 1, Image: FileIdentity{Device: 1, Inode: 2}}
	plan, problem := NewPlan(spec)
	if problem != ProblemNone {
		t.Fatalf("synthetic plan rejected: %v", problem)
	}
	return plan
}
func literalSnapshot(id rollbackfault.Identity, state rollbackfault.SQLState) rollbackfault.SQLSnapshotEvidence {
	rows := [3]rollbackfault.SQLRow{
		{IDType: rollbackfault.SQLInteger, ID: 1, KeyType: rollbackfault.SQLText, Key: "red", ValueType: rollbackfault.SQLText, Value: "one"},
		{IDType: rollbackfault.SQLInteger, ID: 2, KeyType: rollbackfault.SQLText, Key: "blue", ValueType: rollbackfault.SQLText, Value: "two"},
		{IDType: rollbackfault.SQLInteger, ID: 3, KeyType: rollbackfault.SQLText, Key: "red", ValueType: rollbackfault.SQLText, Value: "three"},
	}
	if state == rollbackfault.SQLNew {
		rows = [3]rollbackfault.SQLRow{
			{IDType: rollbackfault.SQLInteger, ID: 1, KeyType: rollbackfault.SQLText, Key: "green", ValueType: rollbackfault.SQLText, Value: "ONE"},
			{IDType: rollbackfault.SQLInteger, ID: 3, KeyType: rollbackfault.SQLText, Key: "red", ValueType: rollbackfault.SQLText, Value: "three"},
			{IDType: rollbackfault.SQLInteger, ID: 4, KeyType: rollbackfault.SQLText, Key: "blue", ValueType: rollbackfault.SQLText, Value: "four"},
		}
	}
	texts := [7]string{
		"SELECT id,k,v FROM t NOT INDEXED ORDER BY id",
		"SELECT id,k,v FROM t WHERE k='blue' ORDER BY id",
		"SELECT id,k,v FROM t NOT INDEXED WHERE k='blue' ORDER BY id",
		"SELECT id,k,v FROM t WHERE k='red' ORDER BY id",
		"SELECT id,k,v FROM t NOT INDEXED WHERE k='red' ORDER BY id",
		"SELECT id,k,v FROM t WHERE k='green' ORDER BY id",
		"SELECT id,k,v FROM t NOT INDEXED WHERE k='green' ORDER BY id",
	}
	keys := [7]string{"", "blue", "blue", "red", "red", "green", "green"}
	snapshot := rollbackfault.SQLSnapshotEvidence{Identity: id, Owner: 11, CloseCalls: 1, OwnerState: rollbackfault.OwnerRetired, Check: rollbackfault.SQLCheckEvidence{Identity: id, Owner: 11, Calls: 1, Pages: 3, Tables: 1, Indexes: 1, TableRows: 3, IndexEntries: 3}}
	for i := 0; i < 7; i++ {
		q := rollbackfault.SQLQueryEvidence{Identity: id, Query: rollbackfault.SQLQuery(i), Owner: 11, Iterator: uint64(i + 1), SQL: texts[i], EOF: true, ErrCalled: true, CloseCalls: 1, Plan: rollbackfault.SQLQueryPlan{Access: "SCAN", Table: "t", Reason: "NOT INDEXED requested"}, Work: rollbackfault.SQLQueryWork{BaseRowFetches: 3}}
		for _, row := range rows {
			if i == 0 || row.Key == keys[i] {
				q.Rows[q.RowCount] = row
				q.RowCount++
			}
		}
		n := int64(q.RowCount)
		q.Work.ReturnedRows = n
		if i == 1 || i == 3 || i == 5 {
			q.Plan = rollbackfault.SQLQueryPlan{Access: "SEARCH", Table: "t", Method: "btree", Index: "by_k", IndexCount: 1, Field: "k", FieldCount: 1, Recheck: true}
			q.Work = rollbackfault.SQLQueryWork{IndexKeysRead: n, IndexPageReads: 1, RowIDSeeks: n, Candidates: n, BaseRowFetches: n, ReturnedRows: n}
		}
		snapshot.Queries[i] = q
	}
	return snapshot
}
func testEvidence(p Plan) ScenarioEvidence {
	s := p.Spec()
	state := rollbackfault.SQLNew
	if s.Binding.Case == BeforeJCreate || s.Binding.Case == AfterTablePage {
		state = rollbackfault.SQLOld
	}
	e := ScenarioEvidence{
		Binding: s.Binding, Before: literalSnapshot(s.AdapterIdentity, rollbackfault.SQLOld), After: literalSnapshot(s.AdapterIdentity, state), BeforeGeneration: s.Controller.Generation, AfterGeneration: s.Recovery.Generation,
		Lifecycle: WriterLifecycle{OpenCalls: 1, BeginCalls: 1, DMLCalls: 3, ArmCalls: 1, CommitCalls: 1, Main: AbandonedByExpectedProcessExit, Journal: AbandonedByExpectedProcessExit, Cleanup: AbandonedByExpectedProcessExit},
		Recovery:  RecoveryEvidence{Generation: s.Recovery.Generation, OwnersRetired: true, ReadOnly: true, NoCreate: true, OpenCalls: 1, CheckMaxPages: 3, CheckMaxPageReads: 512, NoRetry: true},
		Runtime:   RuntimeEvidence{EnvFirst: true, ExactRoleSelectors: true, FixedAllowlistEnvironment: true, CountOne: true, ParallelOne: true, CPUOne: true, ShuffleOff: true, TimeoutZero: true, PrivateRoot0700: true, NoSymlinkAncestors: true, RootUnchanged: true, TwoDedicatedPipes: true, PipeOwnersRecorded: true, UnusedPipeEndsClosedOnce: true, BoundedDiagnostics: true, NoInheritedDatabaseOwners: true, ParentOwnersClosedBeforeSpawn: true, FreshRegistryEachChild: true, OnlyParentSpawns: true, NoSignalOrKill: true, NoRetries: true, OriginalsNeverSyncedAfterExit: true, ArtifactsRetained: true, AdapterSameTransaction: true, AdapterSameScope: true, AdapterSameOwners: true, ObserverOutsidePrivateMutex: true, OriginalTxLockHeldAtCut: true, NoCallbackNativeReplacement: true},
	}
	switch s.Binding.Case {
	case BeforeJCreate:
		e.Lifecycle.Journal = NotAcquired
	case AfterJUnlink:
		e.Lifecycle.Journal = RetiredNormally
	case Control, AcknowledgedExit:
		e.Lifecycle.PublicReturned, e.Lifecycle.Main, e.Lifecycle.Journal, e.Lifecycle.Cleanup = true, RetiredNormally, RetiredNormally, RetiredNormally
		if s.Binding.Case == Control {
			e.Lifecycle.FinishCalls, e.Lifecycle.DisarmCalls, e.Lifecycle.DBCloseCalls = 1, 1, 1
		}
	}
	if s.Binding.Case == AfterTablePage {
		e.Recovery.Calls, e.Recovery.JournalPresentBefore, e.Recovery.Replayed, e.Recovery.ReplayedPages, e.Recovery.OriginalPages = 1, true, true, 3, 3
		e.Recovery.Limits = rollbackfault.SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}
	}
	oldPages := [3]Digest{testDigest("old-schema"), testDigest("old-table"), testDigest("old-index")}
	newPages := [3]Digest{testDigest("new-schema"), testDigest("new-table"), testDigest("new-index")}
	trialHash, trialPages := testDigest("new-database"), newPages
	if s.Binding.Case == BeforeJCreate {
		trialHash, trialPages = s.Binding.Baseline, oldPages
	}
	if s.Binding.Case == AfterTablePage {
		trialHash, trialPages[2] = testDigest("mixed-database"), oldPages[2]
	}
	for i := range e.Artifacts {
		role := ArtifactRole(i)
		a := ArtifactEvidence{Role: role}
		journal := role == TrialJournalArtifact || role == PostExitJournalArtifact || role == ReplayJournalArtifact
		if role == AcknowledgementArtifact || (journal && s.Binding.Case != AfterTablePage) {
			e.Artifacts[i] = a
			continue
		}
		stamp := ArtifactStamp{Path: ArtifactName(role), File: FileIdentity{Device: 1, Inode: 1000 + 100*uint64(s.Binding.Case) + uint64(i)}, Exists: true, Regular: true, Links: 1, Bytes: 12288, SHA256: trialHash, Pages: trialPages}
		if role == BaselineArtifact {
			stamp.SHA256, stamp.Pages = s.Binding.Baseline, oldPages
		}
		if journal {
			stamp.Bytes, stamp.SHA256, stamp.Pages = 16408, testDigest("complete-hot-journal"), [3]Digest{}
		}
		a.Before, a.After = stamp, stamp
		if role == ReplayArtifact && s.Binding.Case == AfterTablePage {
			a.After.SHA256, a.After.Pages = s.Binding.Baseline, oldPages
		}
		if role == ReplayJournalArtifact {
			a.After = ArtifactStamp{}
		}
		if role == PostExitArtifact || role == PostExitJournalArtifact || role == ReplayArtifact || role == ReplayJournalArtifact {
			a.ExclusiveCopy, a.CompleteRead, a.CompleteWrite, a.FileSynced, a.FileClosedOnce, a.DirectorySynced, a.DirectoryClosedOnce = true, true, true, true, true, true, true
		}
		e.Artifacts[i] = a
	}
	return e
}
func literalCalls() [18]rollbackfault.Call {
	return [18]rollbackfault.Call{
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
}
func mustSnapshotDigest(t *testing.T, s rollbackfault.SQLSnapshotEvidence) Digest {
	t.Helper()
	d, p := SnapshotDigest(s)
	if p != ProblemNone {
		t.Fatalf("snapshot: %v", p)
	}
	return d
}
func mustManifestDigest(t *testing.T, e ScenarioEvidence, after bool) Digest {
	t.Helper()
	d, p := ArtifactManifestDigest(e.Artifacts, after)
	if p != ProblemNone {
		t.Fatalf("manifest: %v", p)
	}
	return d
}
func mustFrameDigest(t *testing.T, f Frame) Digest {
	t.Helper()
	d, p := FrameDigest(f)
	if p != ProblemNone {
		t.Fatalf("frame: %v", p)
	}
	return d
}
func testFrames(t *testing.T, p Plan, e *ScenarioEvidence) []Frame {
	t.Helper()
	s := p.Spec()
	var frames []Frame
	add := func(kind FrameKind, role RoleIdentity) *Frame {
		frames = append(frames, Frame{Schema: SchemaVersion, Binding: s.Binding, Sender: role, Sequence: uint8(len(frames) + 1), Kind: kind})
		return &frames[len(frames)-1]
	}
	parent := ProcessEvidence{PID: 1000, Image: s.Image, ImageHash: s.Binding.Binary, ActualImage: true, State: ProcessRunning}
	writer := ProcessEvidence{PID: 1001, Handle: 1, Image: s.Image, ImageHash: s.Binding.Binary, SpawnObserved: true, State: ProcessRunning}
	recovery := ProcessEvidence{PID: 1002, Handle: 2, Image: s.Image, ImageHash: s.Binding.Binary, SpawnObserved: true, State: ProcessRunning}
	add(ParentAdmitted, s.Controller).Process = parent
	add(BaselineCheckedAndClosed, s.Controller).Digest = mustSnapshotDigest(t, e.Before)
	add(WriterSpawned, s.Controller).Process = writer
	ready := writer
	ready.SpawnObserved, ready.ActualImage = false, true
	add(ChildReadyBound, s.Writer).Process = ready
	add(ArmGranted, s.Controller)
	armed := add(TransactionArmed, s.Writer)
	armed.Ticket, armed.Adapter = s.Ticket, rollbackfault.Event{Identity: s.AdapterIdentity, Ordinal: 1, Kind: rollbackfault.RunStarted}
	ordinal := uint8(1)
	var last rollbackfault.Event
	targetCall, targetKind := 0, rollbackfault.EventUnknown
	switch s.Binding.Case {
	case BeforeJCreate:
		targetCall, targetKind = 1, rollbackfault.Intent
	case AfterTablePage:
		targetCall, targetKind = 14, rollbackfault.Returned
	case AfterJUnlink:
		targetCall, targetKind = 17, rollbackfault.Returned
	}
	stopped := false
	for i, call := range literalCalls() {
		for _, kind := range [2]rollbackfault.EventKind{rollbackfault.Intent, rollbackfault.Returned} {
			ordinal++
			event := rollbackfault.Event{Identity: s.AdapterIdentity, Ordinal: ordinal, Kind: kind, Call: call}
			if kind == rollbackfault.Returned {
				effect := rollbackfault.EffectUnknown
				if call.Point == rollbackfault.DBOriginalRead {
					effect = rollbackfault.EffectNone
				}
				if call.Point == rollbackfault.JournalInvalid || call.Point == rollbackfault.JournalRecord || call.Point == rollbackfault.JournalValidHeader || call.Point == rollbackfault.DBPage {
					effect = rollbackfault.EffectPossible
				}
				event.Result = rollbackfault.Result{Executed: true, NativeN: call.Length, ReturnedN: call.Length, Effect: effect}
			}
			f := add(AdapterObserved, s.Writer)
			f.Adapter, f.Ticket = event, s.Ticket
			last = event
			if i+1 == targetCall && kind == targetKind {
				stopped = true
				break
			}
		}
		if stopped {
			break
		}
	}
	if targetCall != 0 {
		reached := add(TargetReached, s.Writer)
		reached.Adapter, reached.Ticket = last, s.Ticket
		digest := mustFrameDigest(t, *reached)
		add(ExitGranted, s.Controller).Digest = digest
	} else {
		commit := add(PublicCommitReturned, s.Writer)
		commit.Commit = CommitEvidence{Calls: 1, Value: CommitNil, CapturedAdjacent: true, CapturedUnchanged: true, PublicDone: true, PublicDetached: true, Main: rollbackfault.OwnerRetired, Journal: rollbackfault.OwnerRetired, Cleanup: rollbackfault.OwnerRetired}
		commitFrame := *commit
		commitHash := mustFrameDigest(t, commitFrame)
		if s.Binding.Case == Control {
			add(NormalFinishGranted, s.Controller).Digest = commitHash
			add(WriterFinished, s.Writer)
		} else {
			raw, problem := EncodeFrame(commitFrame)
			if problem != ProblemNone {
				t.Fatal(problem)
			}
			ack := AckEvidence{CommitSequence: commitFrame.Sequence, CommitFrameSHA256: commitHash, SHA256: digestBytes(raw[4:]), File: FileIdentity{Device: 1, Inode: 9000}, Bytes: int64(len(raw) - 4), Written: int64(len(raw) - 4), Exclusive: true, WriteStep: 1, FileSyncStep: 2, FileCloseStep: 3, DirectorySyncStep: 4, DirectoryCloseStep: 5, FileCloseCalls: 1, DirectoryCloseCalls: 1}
			add(AcknowledgementDurable, s.Controller).Ack = ack
			exit := add(ExitGranted, s.Controller)
			exit.Ack, exit.Digest = ack, ack.SHA256
			stamp := ArtifactStamp{Path: ArtifactName(AcknowledgementArtifact), File: ack.File, Exists: true, Regular: true, Links: 1, Bytes: ack.Bytes, SHA256: ack.SHA256}
			e.Artifacts[AcknowledgementArtifact] = ArtifactEvidence{Role: AcknowledgementArtifact, Before: stamp, After: stamp}
		}
	}
	waited := writer
	waited.SpawnObserved, waited.WaitObserved, waited.State, waited.ExitCode = false, true, ProcessExited, 86
	if s.Binding.Case == Control {
		waited.ExitCode = 0
	}
	add(WriterWaitObserved, s.Controller).Process = waited
	add(OriginalsFrozen, s.Controller).Digest = mustManifestDigest(t, *e, false)
	add(RecoverySpawned, s.Controller).Process = recovery
	ready = recovery
	ready.SpawnObserved, ready.ActualImage = false, true
	add(RecoveryReadyBound, s.Recovery).Process = ready
	add(RecoveryReadbackGranted, s.Controller)
	add(ReadbackResult, s.Recovery).Digest = mustSnapshotDigest(t, e.After)
	waited = recovery
	waited.SpawnObserved, waited.WaitObserved, waited.State = false, true, ProcessExited
	add(RecoveryWaitObserved, s.Controller).Process = waited
	add(ArtifactsRechecked, s.Controller).Digest = mustManifestDigest(t, *e, true)
	add(ScenarioEvidenceComplete, s.Controller)
	return frames
}
func replay(p Plan, frames []Frame) Receipt {
	r, _ := NewReceipt(p)
	for _, frame := range frames {
		if r.Append(frame) != ProblemNone {
			break
		}
	}
	return r
}
func fixture(t *testing.T, which Case) (Plan, Receipt, ScenarioEvidence, []Frame) {
	t.Helper()
	p := testPlan(t, which)
	e := testEvidence(p)
	frames := testFrames(t, p, &e)
	return p, replay(p, frames), e, frames
}
func requireRejected(t *testing.T, o Observation) {
	t.Helper()
	if o.ScenarioObserved || o.EvidenceConsistent || o.Problem == ProblemNone || o.Pass() || o.PhysicalValidated() {
		t.Fatalf("false complete observation: %+v", o)
	}
}
func frameIndex(frames []Frame, kind FrameKind) int {
	for i, f := range frames {
		if f.Kind == kind {
			return i
		}
	}
	return -1
}
func renumber(frames []Frame) {
	for i := range frames {
		frames[i].Sequence = uint8(i + 1)
	}
}

func TestRestartExactlyFiveCasesAndEighteenCalls(t *testing.T) {
	want := [5]string{"control", "before-jcreate", "after-table-page", "after-junlink", "acknowledged-exit"}
	counts := [5]uint8{37, 2, 29, 35, 37}
	for i, c := range Cases() {
		if CaseName(c) != want[i] || ExpectedAdapterEvents(c) != counts[i] {
			t.Fatalf("case inventory %d", i)
		}
		parsed, ok := ParseCase(want[i])
		if !ok || parsed != c {
			t.Fatal("case roundtrip")
		}
	}
	for _, name := range []string{"", "CONTROL", "after-index-page", "after-jcreate", "all", "control,before-jcreate", "acknowledged-exit/again"} {
		if _, ok := ParseCase(name); ok {
			t.Fatalf("unexpected case %q", name)
		}
	}
	if ExpectedCalls() != literalCalls() {
		t.Fatal("exact 18-call path changed")
	}
	if call, phase := Target(BeforeJCreate); call != 1 || phase != rollbackfault.Intent {
		t.Fatal("before target changed")
	}
	if call, phase := Target(AfterTablePage); call != 14 || phase != rollbackfault.Returned {
		t.Fatal("table target changed")
	}
	if call, phase := Target(AfterJUnlink); call != 17 || phase != rollbackfault.Returned {
		t.Fatal("unlink target changed")
	}
}
func TestRestartFiveCompleteValueTranscripts(t *testing.T) {
	for _, which := range Cases() {
		t.Run(CaseName(which), func(t *testing.T) {
			p, receipt, e, frames := fixture(t, which)
			if len(frames) > MaxFrames {
				t.Fatal("frozen frame budget insufficient")
			}
			for _, f := range frames {
				raw, problem := EncodeFrame(f)
				if problem != ProblemNone {
					t.Fatalf("encode %d: %v", f.Kind, problem)
				}
				if len(raw)-4 > MaxFrameBytes {
					t.Fatal("frame budget")
				}
				decoded, problem := DecodeFrame(raw)
				if problem != ProblemNone || decoded != f {
					t.Fatalf("roundtrip %d: %v", f.Kind, problem)
				}
			}
			o := Observe(p, receipt, e)
			if !o.EvidenceConsistent || !o.ScenarioObserved || o.Status != EvidenceComplete || o.Problem != ProblemNone || o.SQLState != ExpectedSQLState(which) || !o.WriterWaited || !o.RecoveryWaited || o.Pass() || o.PhysicalValidated() {
				t.Fatalf("synthetic consistency: %+v", o)
			}
			if o.Acknowledged != (which == AcknowledgedExit) {
				t.Fatal("ack inferred from wrong fact")
			}
		})
	}
}
func TestRestartNewGenerationCanReuseLocalOrdinalsAndPID(t *testing.T) {
	p, _, e, frames := fixture(t, AfterTablePage)
	if e.Before.Owner != e.After.Owner || e.Before.Queries[0].Iterator != e.After.Queries[0].Iterator {
		t.Fatal("fixture must repeat local ordinals")
	}
	writerPID := frames[frameIndex(frames, WriterSpawned)].Process.PID
	for i := range frames {
		if frames[i].Kind == RecoverySpawned || frames[i].Kind == RecoveryReadyBound || frames[i].Kind == RecoveryWaitObserved {
			frames[i].Process.PID = writerPID
		}
	}
	o := Observe(p, replay(p, frames), e)
	if !o.ScenarioObserved || o.Problem != ProblemNone {
		t.Fatalf("generation identity replaced by PID/local integer inequality: %+v", o)
	}
}
func TestRestartPlanIdentityAndClosedValues(t *testing.T) {
	p := testPlan(t, Control)
	mutants := []struct {
		name   string
		mutate func(*PlanSpec)
	}{
		{"no-case", func(s *PlanSpec) { s.Binding.Case = CaseUnknown }},
		{"sixth-case", func(s *PlanSpec) { s.Binding.Case = Case(6) }},
		{"same-process-generation", func(s *PlanSpec) { s.Recovery.Generation = s.Writer.Generation }},
		{"same-challenge", func(s *PlanSpec) { s.Recovery.Challenge = s.Writer.Challenge }},
		{"zero-challenge", func(s *PlanSpec) { s.Writer.Challenge = "" }},
		{"all-zero-challenge", func(s *PlanSpec) { s.Writer.Challenge = Digest(strings.Repeat("0", 64)) }},
		{"foreign-source", func(s *PlanSpec) { s.Binding.Source = "other-source" }},
		{"foreign-tool", func(s *PlanSpec) { s.Binding.Tool = testDigest("other-tool") }},
		{"foreign-binary", func(s *PlanSpec) { s.Binding.Binary = testDigest("other-binary") }},
		{"foreign-manifest", func(s *PlanSpec) { s.Binding.Manifest = testDigest("other-manifest") }},
		{"missing-compile-receipt", func(s *PlanSpec) { s.Binding.Compile = "" }},
		{"unknown-profile", func(s *PlanSpec) { s.Binding.Profile = rollbackfault.ProfileUnknown }},
		{"wrong-os", func(s *PlanSpec) { s.Binding.Native.OS = "darwin" }},
		{"wrong-sector", func(s *PlanSpec) { s.Binding.Native.SectorSize = 512 }},
		{"root-missing", func(s *PlanSpec) { s.Binding.Root.Inode = 0 }},
		{"image-missing", func(s *PlanSpec) { s.Image.Inode = 0 }},
		{"ticket-missing", func(s *PlanSpec) { s.Ticket = 0 }},
		{"scope-unbounded", func(s *PlanSpec) { s.AdapterIdentity.Scope = strings.Repeat("x", MaxIDLength+1) }},
		{"upper-case-hash", func(s *PlanSpec) { s.Binding.Compile = Digest(strings.Repeat("A", 64)) }},
	}
	for _, mutant := range mutants {
		t.Run(mutant.name, func(t *testing.T) {
			s := p.Spec()
			mutant.mutate(&s)
			if _, problem := NewPlan(s); problem == ProblemNone {
				t.Fatal("admitted invalid plan")
			}
		})
	}
	if _, problem := NewReceipt(Plan{}); problem == ProblemNone {
		t.Fatal("zero plan admitted")
	}
}
func lengthPrefix(body []byte) []byte {
	out := make([]byte, len(body)+4)
	binary.BigEndian.PutUint32(out[:4], uint32(len(body)))
	copy(out[4:], body)
	return out
}
func TestRestartCanonicalBoundedFraming(t *testing.T) {
	_, _, _, frames := fixture(t, Control)
	raw, problem := EncodeFrame(frames[0])
	if problem != ProblemNone {
		t.Fatal(problem)
	}
	body := raw[4:]
	bad := [][]byte{
		nil, raw[:3], raw[:len(raw)-1], append(append([]byte{}, raw...), 0),
		{0, 0, 0, 0}, lengthPrefix(bytes.Repeat([]byte{' '}, MaxFrameBytes+1)),
		lengthPrefix(append([]byte(" "), body...)),
		lengthPrefix(append(append([]byte{}, body...), []byte("{}")...)),
		lengthPrefix(append([]byte(`{"Unknown":true,`), body[1:]...)),
		lengthPrefix(append([]byte(`{"Schema":1,`), body[1:]...)),
		lengthPrefix(bytes.Replace(body, []byte(`"Schema":1`), []byte(`"Schema":2`), 1)),
		lengthPrefix(bytes.Replace(body, []byte(`"Role":1`), []byte(`"Role":99`), 1)),
	}
	for i, value := range bad {
		if _, problem := DecodeFrame(value); problem == ProblemNone {
			t.Fatalf("invalid encoding %d accepted", i)
		}
	}
	p := testPlan(t, Control)
	s := p.Spec()
	s.Binding.Source, s.AdapterIdentity.Source = strings.Repeat("s", MaxIDLength), strings.Repeat("s", MaxIDLength)
	s.Binding.Trial, s.AdapterIdentity.Trial = strings.Repeat("t", MaxIDLength), strings.Repeat("t", MaxIDLength)
	s.AdapterIdentity.Scope = strings.Repeat("q", MaxIDLength)
	p, problem = NewPlan(s)
	if problem != ProblemNone {
		t.Fatal(problem)
	}
	call := literalCalls()[13]
	f := Frame{Schema: SchemaVersion, Binding: p.Spec().Binding, Sender: p.Spec().Writer, Sequence: 64, Kind: AdapterObserved, Ticket: 1, Adapter: rollbackfault.Event{Identity: p.Spec().AdapterIdentity, Ordinal: 29, Kind: rollbackfault.Returned, Call: call, Result: rollbackfault.Result{Executed: true, NativeN: 4096, ReturnedN: 4096, Effect: rollbackfault.EffectPossible}}}
	raw, problem = EncodeFrame(f)
	if problem != ProblemNone || len(raw) > MaxFrameBytes+4 {
		t.Fatalf("maximal admitted identity exceeds frozen frame budget: %d %v", len(raw), problem)
	}
}
func TestRestartRejectsProtocolAndAdapterFalsePositives(t *testing.T) {
	mutants := []struct {
		name   string
		which  Case
		kind   FrameKind
		mutate func(*Frame)
	}{
		{"wrong-sequence", BeforeJCreate, TargetReached, func(f *Frame) { f.Sequence++ }},
		{"old-trial", BeforeJCreate, TargetReached, func(f *Frame) { f.Binding.Trial = "old-trial" }},
		{"wrong-role", BeforeJCreate, TargetReached, func(f *Frame) { f.Sender.Role = Recovery }},
		{"old-generation", BeforeJCreate, TargetReached, func(f *Frame) { f.Sender.Generation-- }},
		{"wrong-challenge", BeforeJCreate, TargetReached, func(f *Frame) { f.Sender.Challenge = testDigest("wrong") }},
		{"wrong-root", BeforeJCreate, TargetReached, func(f *Frame) { f.Binding.Root.Inode++ }},
		{"wrong-baseline", BeforeJCreate, TargetReached, func(f *Frame) { f.Binding.Baseline = testDigest("wrong") }},
		{"wrong-compile", BeforeJCreate, TargetReached, func(f *Frame) { f.Binding.Compile = testDigest("wrong") }},
		{"wrong-source", BeforeJCreate, TargetReached, func(f *Frame) { f.Binding.Source = "other" }},
		{"wrong-binary", BeforeJCreate, TargetReached, func(f *Frame) { f.Binding.Binary = testDigest("wrong") }},
		{"wrong-native", BeforeJCreate, TargetReached, func(f *Frame) { f.Binding.Native.Filesystem = "different" }},
		{"same-scope-required", AfterTablePage, TargetReached, func(f *Frame) { f.Adapter.Identity.Scope = "other-scope" }},
		{"same-ticket-required", AfterTablePage, TargetReached, func(f *Frame) { f.Ticket++ }},
		{"before-is-not-returned", BeforeJCreate, AdapterObserved, func(f *Frame) { f.Adapter.Kind = rollbackfault.Returned }},
		{"wrong-owner", BeforeJCreate, AdapterObserved, func(f *Frame) { f.Adapter.Call.Owner++ }},
		{"wrong-occurrence", BeforeJCreate, AdapterObserved, func(f *Frame) { f.Adapter.Call.Occurrence++ }},
		{"wrong-offset", AfterTablePage, TargetReached, func(f *Frame) { f.Adapter.Call.Offset = 8192 }},
		{"wrong-length", AfterTablePage, TargetReached, func(f *Frame) { f.Adapter.Call.Length = 64 }},
		{"fake-native-count", AfterTablePage, TargetReached, func(f *Frame) { f.Adapter.Result.NativeN = 64 }},
		{"injected-action", BeforeJCreate, AdapterObserved, func(f *Frame) { f.Adapter.Action = rollbackfault.FailBefore }},
		{"fake-finish", BeforeJCreate, AdapterObserved, func(f *Frame) { f.Adapter.Kind = rollbackfault.Terminal }},
		{"fake-commit-event", BeforeJCreate, AdapterObserved, func(f *Frame) { f.Adapter.Commit.Class = rollbackfault.CommitSucceeded }},
		{"normal-exit-is-not-crash", AfterJUnlink, WriterWaitObserved, func(f *Frame) { f.Process.ExitCode = 0 }},
		{"abort-is-not-target", AfterJUnlink, WriterWaitObserved, func(f *Frame) { f.Process.ExitCode = AbortExitCode }},
		{"signal-is-not-target", AfterJUnlink, WriterWaitObserved, func(f *Frame) { f.Process.State, f.Process.Signal = ProcessSignaled, 9 }},
		{"timeout-is-not-wait", AfterJUnlink, WriterWaitObserved, func(f *Frame) { f.Process.TimedOut = true }},
		{"live-is-not-exit", AfterJUnlink, WriterWaitObserved, func(f *Frame) { f.Process.State = ProcessRunning }},
		{"missing-actual-wait", AfterJUnlink, WriterWaitObserved, func(f *Frame) { f.Process.WaitObserved = false }},
		{"child-chosen-pid", BeforeJCreate, ChildReadyBound, func(f *Frame) { f.Process.PID++ }},
		{"child-claims-spawn", BeforeJCreate, ChildReadyBound, func(f *Frame) { f.Process.SpawnObserved = true }},
		{"argv-is-not-image", BeforeJCreate, ChildReadyBound, func(f *Frame) { f.Process.ActualImage = false }},
		{"foreign-executing-image", BeforeJCreate, ChildReadyBound, func(f *Frame) { f.Process.Image.Inode++ }},
		{"reopen-is-not-new-process", BeforeJCreate, RecoverySpawned, func(f *Frame) { f.Process.Handle = 1 }},
		{"recovery-exit86-not-success", BeforeJCreate, RecoveryWaitObserved, func(f *Frame) { f.Process.ExitCode = 86 }},
		{"wrong-exit-grant", BeforeJCreate, ExitGranted, func(f *Frame) { f.Digest = testDigest("unbound-permission") }},
	}
	for _, mutant := range mutants {
		t.Run(mutant.name, func(t *testing.T) {
			p, _, e, frames := fixture(t, mutant.which)
			index := frameIndex(frames, mutant.kind)
			if index < 0 {
				t.Fatal("missing mutation target")
			}
			mutant.mutate(&frames[index])
			requireRejected(t, Observe(p, replay(p, frames), e))
		})
	}
	for _, kind := range []FrameKind{TargetReached, ExitGranted, WriterWaitObserved, OriginalsFrozen, RecoveryReadyBound, RecoveryReadbackGranted, ReadbackResult, RecoveryWaitObserved, ScenarioEvidenceComplete} {
		t.Run("missing-causal-step", func(t *testing.T) {
			p, _, e, frames := fixture(t, AfterTablePage)
			i := frameIndex(frames, kind)
			frames = append(frames[:i], frames[i+1:]...)
			renumber(frames)
			requireRejected(t, Observe(p, replay(p, frames), e))
		})
	}
	p, _, e, frames := fixture(t, AfterTablePage)
	i := frameIndex(frames, TargetReached)
	duplicated := append([]Frame{}, frames[:i+1]...)
	duplicated = append(duplicated, frames[i])
	duplicated = append(duplicated, frames[i+1:]...)
	renumber(duplicated)
	requireRejected(t, Observe(p, replay(p, duplicated), e))
	extra := append([]Frame{}, frames[:i]...)
	forbidden := frames[i-1]
	forbidden.Adapter.Ordinal++
	forbidden.Adapter.Kind = rollbackfault.Intent
	forbidden.Adapter.Call = literalCalls()[14]
	forbidden.Adapter.Result = rollbackfault.Result{}
	extra = append(extra, forbidden)
	extra = append(extra, frames[i:]...)
	renumber(extra)
	requireRejected(t, Observe(p, replay(p, extra), e))
}

func TestRestartAcknowledgementRequiresExactCausalDurability(t *testing.T) {
	mutants := []struct {
		name   string
		mutate func(*AckEvidence)
	}{
		{"wrong-public-sequence", func(a *AckEvidence) { a.CommitSequence-- }},
		{"different-commit-frame", func(a *AckEvidence) { a.CommitFrameSHA256 = testDigest("old-commit") }},
		{"different-ack-content", func(a *AckEvidence) { a.SHA256 = testDigest("old-ack") }},
		{"not-exclusive", func(a *AckEvidence) { a.Exclusive = false }},
		{"short-write", func(a *AckEvidence) { a.Written-- }},
		{"wrong-length", func(a *AckEvidence) { a.Bytes++; a.Written++ }},
		{"missing-file-sync", func(a *AckEvidence) { a.FileSyncStep = 0 }},
		{"missing-file-close", func(a *AckEvidence) { a.FileCloseCalls = 0 }},
		{"double-file-close", func(a *AckEvidence) { a.FileCloseCalls = 2 }},
		{"missing-directory-sync", func(a *AckEvidence) { a.DirectorySyncStep = 0 }},
		{"missing-directory-close", func(a *AckEvidence) { a.DirectoryCloseCalls = 0 }},
		{"directory-before-file", func(a *AckEvidence) { a.DirectorySyncStep, a.FileSyncStep = 2, 4 }},
		{"close-before-sync", func(a *AckEvidence) { a.FileCloseStep, a.FileSyncStep = 2, 3 }},
		{"sync-error", func(a *AckEvidence) { a.ErrorToken = 1 }},
	}
	for _, mutant := range mutants {
		t.Run(mutant.name, func(t *testing.T) {
			p, _, e, frames := fixture(t, AcknowledgedExit)
			mutant.mutate(&frames[frameIndex(frames, AcknowledgementDurable)].Ack)
			requireRejected(t, Observe(p, replay(p, frames), e))
		})
	}
	for _, value := range []CommitValue{CommitUnknown, CommitError, CommitUncertain} {
		p, _, e, frames := fixture(t, AcknowledgedExit)
		frames[frameIndex(frames, PublicCommitReturned)].Commit.Value = value
		requireRejected(t, Observe(p, replay(p, frames), e))
	}
	for _, kind := range []FrameKind{PublicCommitReturned, AcknowledgementDurable} {
		p, _, e, frames := fixture(t, AcknowledgedExit)
		i := frameIndex(frames, kind)
		frames = append(frames[:i], frames[i+1:]...)
		renumber(frames)
		requireRejected(t, Observe(p, replay(p, frames), e))
	}
	p, _, e, frames := fixture(t, AcknowledgedExit)
	commit, ack := frameIndex(frames, PublicCommitReturned), frameIndex(frames, AcknowledgementDurable)
	frames[commit], frames[ack] = frames[ack], frames[commit]
	renumber(frames)
	requireRejected(t, Observe(p, replay(p, frames), e))
	p, _, e, frames = fixture(t, AfterJUnlink)
	// A missing journal and successful DB Sync cannot introduce an ACK fact.
	frames[frameIndex(frames, ExitGranted)].Ack = AckEvidence{SHA256: testDigest("made-up")}
	requireRejected(t, Observe(p, replay(p, frames), e))
}
func TestRestartIncompleteLivenessAndStickyFailure(t *testing.T) {
	for _, which := range Cases() {
		p, _, e, frames := fixture(t, which)
		for n := 0; n < len(frames); n++ {
			r := replay(p, frames[:n])
			o := Observe(p, r, e)
			requireRejected(t, o)
			if o.SQLState != rollbackfault.SQLStateUnknown {
				t.Fatal("partial transcript invented SQL state")
			}
			if n == 0 && o.Status != NotRun {
				t.Fatal("empty receipt must be NOT_RUN")
			}
		}
	}
	p, _, e, frames := fixture(t, AcknowledgedExit)
	commit := frameIndex(frames, PublicCommitReturned)
	r := replay(p, frames[:commit+1])
	o := Observe(p, r, e)
	if o.Commit != CommitNil || o.Acknowledged || o.WriterWaited || o.SQLState != rollbackfault.SQLStateUnknown {
		t.Fatalf("pipe COMMIT_OK became acknowledgement/readback: %+v", o)
	}
	for _, diagnostic := range []Diagnostic{ChildLive, WaitUnknown, AckFailed, AckUncertain, OwnerUnknown, OwnerPending, TargetNotReached} {
		r = replay(p, frames[:commit+1])
		failure := Frame{Schema: SchemaVersion, Binding: p.Spec().Binding, Sender: p.Spec().Controller, Sequence: uint8(commit + 2), Kind: Failure, Diagnostic: diagnostic}
		if r.Append(failure) == ProblemNone {
			t.Fatal("failure accepted as complete")
		}
		count := r.count
		if r.Append(frames[commit+1]) != ProblemIncomplete || r.count != count {
			t.Fatal("failure was overwritten by later success")
		}
		o = Observe(p, r, e)
		requireRejected(t, o)
		if o.Status != diagnostic || o.Acknowledged || o.WriterWaited {
			t.Fatalf("lost exact failure or inferred later fact: %+v", o)
		}
	}
	r = replay(p, frames[:commit+1])
	_ = r.Seal()
	if r.Append(frames[commit+1]) != ProblemSealed {
		t.Fatal("sealed incomplete receipt repaired")
	}
	forged := Observation{EvidenceConsistent: true, ScenarioObserved: true, Status: EvidenceComplete, Acknowledged: true, WriterWaited: true, RecoveryWaited: true}
	if forged.PhysicalValidated() || forged.Pass() {
		t.Fatal("exported observation acquired physical authority")
	}
	requireRejected(t, Observe(p, Receipt{}, e))
}
func rebindManifests(frames []Frame, e ScenarioEvidence) {
	if d, p := ArtifactManifestDigest(e.Artifacts, false); p == ProblemNone {
		frames[frameIndex(frames, OriginalsFrozen)].Digest = d
	}
	if d, p := ArtifactManifestDigest(e.Artifacts, true); p == ProblemNone {
		frames[frameIndex(frames, ArtifactsRechecked)].Digest = d
	}
}
func TestRestartRejectsOwnerRecoveryAndArtifactFalsePositives(t *testing.T) {
	mutants := []struct {
		name   string
		which  Case
		mutate func(*ScenarioEvidence)
	}{
		{"fake-crash-Finish", AfterTablePage, func(e *ScenarioEvidence) { e.Lifecycle.FinishCalls = 1 }},
		{"fake-crash-Disarm", AfterTablePage, func(e *ScenarioEvidence) { e.Lifecycle.DisarmCalls = 1 }},
		{"fake-crash-Commit-return", AfterTablePage, func(e *ScenarioEvidence) { e.Lifecycle.PublicReturned = true }},
		{"fake-crash-retirement", AfterTablePage, func(e *ScenarioEvidence) { e.Lifecycle.Main = RetiredNormally }},
		{"crash-DB-Close", AfterJUnlink, func(e *ScenarioEvidence) { e.Lifecycle.DBCloseCalls = 1 }},
		{"crash-Rollback", BeforeJCreate, func(e *ScenarioEvidence) { e.Lifecycle.RollbackCalls = 1 }},
		{"retry-Commit", AfterTablePage, func(e *ScenarioEvidence) { e.Lifecycle.CommitCalls = 2 }},
		{"normal-owner-pending", Control, func(e *ScenarioEvidence) { e.Lifecycle.Main = DispositionUnknown }},
		{"ack-DB-Close", AcknowledgedExit, func(e *ScenarioEvidence) { e.Lifecycle.DBCloseCalls = 1 }},
		{"same-process-readback", AfterTablePage, func(e *ScenarioEvidence) { e.AfterGeneration = 2 }},
		{"baseline-not-closed-before-spawn", Control, func(e *ScenarioEvidence) { e.Runtime.ParentOwnersClosedBeforeSpawn = false }},
		{"stdout-PASS-only", Control, func(e *ScenarioEvidence) { e.Runtime.TwoDedicatedPipes = false }},
		{"truncated-diagnostics", Control, func(e *ScenarioEvidence) { e.Runtime.BoundedDiagnostics = false }},
		{"empty-selector", Control, func(e *ScenarioEvidence) { e.Runtime.ExactRoleSelectors = false }},
		{"automatic-kill", Control, func(e *ScenarioEvidence) { e.Runtime.NoSignalOrKill = false }},
		{"same-process-registry", AfterTablePage, func(e *ScenarioEvidence) { e.Runtime.FreshRegistryEachChild = false }},
		{"observer-inside-mutex", AfterTablePage, func(e *ScenarioEvidence) { e.Runtime.ObserverOutsidePrivateMutex = false }},
		{"different-Tx", AfterTablePage, func(e *ScenarioEvidence) { e.Runtime.AdapterSameTransaction = false }},
		{"trial-extra-Sync", AfterJUnlink, func(e *ScenarioEvidence) { e.Runtime.OriginalsNeverSyncedAfterExit = false }},
		{"recovery-twice", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.Calls = 2 }},
		{"no-recovery", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.Calls = 0 }},
		{"control-recovery", Control, func(e *ScenarioEvidence) { e.Recovery.Calls = 1 }},
		{"unlink-recovery", AfterJUnlink, func(e *ScenarioEvidence) { e.Recovery.Calls = 1 }},
		{"larger-replay-budget", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.Limits.MaxRecords = 4 }},
		{"wrong-replayed-extent", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.OriginalPages = 4 }},
		{"writer-recovery-generation", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.Generation = 2 }},
		{"readwrite-reopen", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.ReadOnly = false }},
		{"unbounded-Check", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.CheckMaxPageReads = 513 }},
		{"recovery-retry", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.NoRetry = false }},
		{"recovery-error-masked", AfterTablePage, func(e *ScenarioEvidence) { e.Recovery.Error, e.Recovery.ErrorToken = rollbackfault.Underlying, 1 }},
		{"original-overwritten", AfterTablePage, func(e *ScenarioEvidence) {
			e.Artifacts[TrialArtifact].After.SHA256 = e.Artifacts[BaselineArtifact].Before.SHA256
		}},
		{"original-recovered", AfterTablePage, func(e *ScenarioEvidence) { e.Artifacts[TrialJournalArtifact].After = ArtifactStamp{} }},
		{"journal-omitted", AfterTablePage, func(e *ScenarioEvidence) {
			e.Artifacts[PostExitJournalArtifact] = ArtifactEvidence{Role: PostExitJournalArtifact}
		}},
		{"wrong-journal-size", AfterTablePage, func(e *ScenarioEvidence) { e.Artifacts[TrialJournalArtifact].Before.Bytes-- }},
		{"wrong-copy-hash", AfterTablePage, func(e *ScenarioEvidence) {
			e.Artifacts[PostExitArtifact].Before.SHA256 = testDigest("wrong")
			e.Artifacts[PostExitArtifact].After = e.Artifacts[PostExitArtifact].Before
		}},
		{"hardlink-copy", AfterTablePage, func(e *ScenarioEvidence) {
			e.Artifacts[ReplayArtifact].Before.Links = 2
			e.Artifacts[ReplayArtifact].After.Links = 2
		}},
		{"inode-alias", AfterTablePage, func(e *ScenarioEvidence) {
			e.Artifacts[ReplayArtifact].Before.File = e.Artifacts[TrialArtifact].Before.File
			e.Artifacts[ReplayArtifact].After.File = e.Artifacts[TrialArtifact].Before.File
		}},
		{"nonexclusive-copy", AfterTablePage, func(e *ScenarioEvidence) { e.Artifacts[ReplayArtifact].ExclusiveCopy = false }},
		{"copy-short-write", AfterTablePage, func(e *ScenarioEvidence) { e.Artifacts[ReplayArtifact].CompleteWrite = false }},
		{"copy-directory-not-synced", AfterTablePage, func(e *ScenarioEvidence) { e.Artifacts[ReplayArtifact].DirectorySynced = false }},
		{"trial-new-at-before-create", BeforeJCreate, func(e *ScenarioEvidence) {
			for _, i := range []ArtifactRole{TrialArtifact, PostExitArtifact, ReplayArtifact} {
				e.Artifacts[i].Before.SHA256 = testDigest("different")
				e.Artifacts[i].After = e.Artifacts[i].Before
			}
		}},
		{"index-already-new-at-table-cut", AfterTablePage, func(e *ScenarioEvidence) {
			for _, i := range []ArtifactRole{TrialArtifact, PostExitArtifact, ReplayArtifact} {
				e.Artifacts[i].Before.Pages[2] = testDigest("new-index")
				if i != ReplayArtifact {
					e.Artifacts[i].After = e.Artifacts[i].Before
				}
			}
		}},
		{"mixed-after-replay", AfterTablePage, func(e *ScenarioEvidence) { e.Artifacts[ReplayArtifact].After = e.Artifacts[ReplayArtifact].Before }},
		{"ack-artifact-replaced", AcknowledgedExit, func(e *ScenarioEvidence) {
			e.Artifacts[AcknowledgementArtifact].Before.SHA256 = testDigest("other")
			e.Artifacts[AcknowledgementArtifact].After = e.Artifacts[AcknowledgementArtifact].Before
		}},
		{"wrong-relative-path", AfterTablePage, func(e *ScenarioEvidence) { e.Artifacts[ReplayArtifact].Before.Path = "trial/trial.db" }},
		{"symlink-path", AfterTablePage, func(e *ScenarioEvidence) { e.Runtime.NoSymlinkAncestors = false }},
		{"physical-claim", Control, func(e *ScenarioEvidence) { e.PhysicalClaim = true }},
	}
	for _, mutant := range mutants {
		t.Run(mutant.name, func(t *testing.T) {
			p, _, e, frames := fixture(t, mutant.which)
			mutant.mutate(&e)
			rebindManifests(frames, e)
			requireRejected(t, Observe(p, replay(p, frames), e))
		})
	}
}
func TestRestartExactSQLSnapshotFalsePositives(t *testing.T) {
	mutants := []struct {
		name   string
		mutate func(*rollbackfault.SQLSnapshotEvidence)
	}{
		{"same-count-wrong-value", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[0].Rows[0].Value = "one" }},
		{"wrong-type", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[0].Rows[0].IDType = rollbackfault.SQLText }},
		{"wrong-order", func(s *rollbackfault.SQLSnapshotEvidence) {
			s.Queries[0].Rows[0], s.Queries[0].Rows[1] = s.Queries[0].Rows[1], s.Queries[0].Rows[0]
		}},
		{"not-exhausted", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[0].EOF = false }},
		{"Err-not-read", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[0].ErrCalled = false }},
		{"iterator-close-missing", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[0].CloseCalls = 0 }},
		{"owner-close-missing", func(s *rollbackfault.SQLSnapshotEvidence) { s.CloseCalls = 0 }},
		{"owner-pending", func(s *rollbackfault.SQLSnapshotEvidence) { s.OwnerState = rollbackfault.OwnerPending }},
		{"index-is-scan", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[1].Plan.Access = "SCAN" }},
		{"wrong-index", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[1].Plan.Index = "other" }},
		{"wrong-index-field", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[1].Plan.Field = "v" }},
		{"no-index-page", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[1].Work.IndexPageReads = 0 }},
		{"wrong-base-fetches", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[1].Work.BaseRowFetches = 0 }},
		{"Check-ignored", func(s *rollbackfault.SQLSnapshotEvidence) { s.Check.IndexEntries = 2 }},
		{"duplicate-iterator", func(s *rollbackfault.SQLSnapshotEvidence) { s.Queries[1].Iterator = s.Queries[0].Iterator }},
	}
	for _, mutant := range mutants {
		t.Run(mutant.name, func(t *testing.T) {
			p, receipt, e, _ := fixture(t, AcknowledgedExit)
			mutant.mutate(&e.After)
			requireRejected(t, Observe(p, receipt, e))
		})
	}
	for _, which := range []Case{Control, AcknowledgedExit, AfterJUnlink} {
		p, _, e, frames := fixture(t, which)
		e.After = literalSnapshot(p.Spec().AdapterIdentity, rollbackfault.SQLOld)
		frames[frameIndex(frames, ReadbackResult)].Digest = mustSnapshotDigest(t, e.After)
		requireRejected(t, Observe(p, replay(p, frames), e))
	}
	p, receipt, e, _ := fixture(t, AfterTablePage)
	e.After.Queries[5].Work.IndexPageReads = 0 // OLD green is empty but must perform a real index probe.
	requireRejected(t, Observe(p, receipt, e))
	p, receipt, e, _ = fixture(t, Control)
	// Matching scan and index wrong rows still cannot replace literal NEW.
	e.After.Queries[0].Rows[0].Value = "wrong"
	e.After.Queries[5].Rows[0].Value = "wrong"
	e.After.Queries[6].Rows[0].Value = "wrong"
	requireRejected(t, Observe(p, receipt, e))
}
func TestRestartCompleteSuiteRejectsMissingDuplicateAndReuse(t *testing.T) {
	var runs [5]ScenarioRun
	for i, which := range Cases() {
		p, receipt, e, _ := fixture(t, which)
		runs[i] = ScenarioRun{Plan: p, Receipt: receipt, Evidence: e}
	}
	o := ObserveSuite(runs)
	if !o.EvidenceConsistent || o.Problem != ProblemNone || o.PhysicalValidated() || o.Pass() {
		t.Fatalf("five-value suite: %+v", o)
	}
	missing := runs
	missing[4] = ScenarioRun{}
	if ObserveSuite(missing).EvidenceConsistent {
		t.Fatal("missing case admitted")
	}
	duplicate := runs
	duplicate[4] = duplicate[3]
	if ObserveSuite(duplicate).EvidenceConsistent {
		t.Fatal("duplicate case admitted")
	}
	reused := runs
	reused[1].Plan.spec.Binding.Root = reused[0].Plan.spec.Binding.Root
	if ObserveSuite(reused).EvidenceConsistent {
		t.Fatal("trial root reused")
	}
	forged := SuiteObservation{EvidenceConsistent: true}
	if forged.PhysicalValidated() || forged.Pass() {
		t.Fatal("suite manufactured physical authority")
	}
}

func TestRestartUnderlyingResultAndIdentityFailuresRemainRaw(t *testing.T) {
	p, _, e, frames := fixture(t, AfterTablePage)
	index := -1
	for i, f := range frames {
		if f.Kind == AdapterObserved && f.Adapter.Kind == rollbackfault.Returned && f.Adapter.Call.Point == rollbackfault.DBPage {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatal("missing actual-return mutation point")
	}
	failed := &frames[index].Adapter.Result
	failed.NativeN, failed.ReturnedN = 64, 64
	failed.NativeError, failed.ReturnedError, failed.NormalizedError = rollbackfault.Underlying, rollbackfault.Underlying, rollbackfault.Underlying
	failed.NativeErrorToken, failed.ReturnedErrorToken, failed.NormalizedErrorToken = 91, 91, 91
	failed.Effect = rollbackfault.EffectUnknown
	r := replay(p, frames)
	kept, count := r.Frames()
	if int(count) != index+1 || kept[index].Adapter.Result != *failed {
		t.Fatal("original failed native result lost or replaced")
	}
	o := Observe(p, r, e)
	requireRejected(t, o)
	if o.Status != UnderlyingIOFailed || o.Commit != CommitUnknown || o.Acknowledged || o.WriterWaited || o.SQLState != rollbackfault.SQLStateUnknown {
		t.Fatalf("native error inferred unavailable facts: %+v", o)
	}
	p, _, e, frames = fixture(t, BeforeJCreate)
	index = frameIndex(frames, TargetReached)
	frames[index].Binding.Trial = strings.Repeat("x", MaxIDLength+1)
	r = replay(p, frames)
	kept, count = r.Frames()
	if int(count) != index {
		t.Fatal("unbounded rejected identity retained")
	}
	for i := uint8(0); i < count; i++ {
		if kept[i].Binding != p.Spec().Binding {
			t.Fatal("untrusted binding retained")
		}
	}
	requireRejected(t, Observe(p, r, e))
}
func TestRestartFrozenProtocolBudgetAndDetachedReceipt(t *testing.T) {
	want := [5]int{54, 18, 45, 51, 54}
	for i, which := range Cases() {
		p, r, e, frames := fixture(t, which)
		if len(frames) != want[i] || len(frames) > MaxScenarioFrames || MaxScenarioFrames > MaxFrames || ExpectedAdapterEvents(which) > MaxAdapterEvents {
			t.Fatalf("protocol budget changed: %s %d", CaseName(which), len(frames))
		}
		copyFrames, count := r.Frames()
		copyFrames[0].Binding.Trial = "detached-mutation"
		if count != uint8(len(frames)) || !Observe(p, r, e).ScenarioObserved {
			t.Fatal("receipt exposed mutable state")
		}
	}
}
