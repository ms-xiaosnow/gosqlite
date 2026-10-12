package rollbackfault

import "testing"

// SOURCE ONLY: these tests have not been compiled or executed. All fixtures are
// finite values. They create no database, owner, descriptor, process or recovery
// input, and a passing test would validate only this receipt contract.
func onePageSpec(action Action, targetIndex int) PlanSpec {
	spec := PlanSpec{
		Identity:  Identity{Suite: "suite-v1", Source: "source-v1", Tool: "tool-v1", Binary: "binary-v1", Manifest: "manifest-v1", Trial: "trial-1", Scope: "scope-1", Profile: PlainRollback4096},
		MainOwner: 11, JournalOwner: 12, DirectoryOwner: 13, CleanupOwner: 14,
		OriginalPages: 1, DirtyPages: 1, PageSize: 4096, SectorSize: 4096,
		Action: action, ExpectedCommit: CommitSucceeded,
		Calls: [MaxCalls]Call{
			{Point: JournalCreate, Occurrence: 1, Role: Journal, Owner: 12},
			{Point: JournalInvalid, Occurrence: 1, Role: Journal, Owner: 12, Length: 4096},
			{Point: DBOriginalRead, Occurrence: 1, Role: Database, Owner: 11, Length: 4096},
			{Point: JournalRecord, Occurrence: 1, Role: Journal, Owner: 12, Offset: 4096, Length: 4104},
			{Point: JournalBodySync, Occurrence: 1, Role: Journal, Owner: 12},
			{Point: JournalValidHeader, Occurrence: 1, Role: Journal, Owner: 12, Length: 12},
			{Point: JournalHeaderSync, Occurrence: 1, Role: Journal, Owner: 12},
			{Point: PublishDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},
			{Point: DBPage, Occurrence: 1, Role: Database, Owner: 11, Length: 4096},
			{Point: DBSync, Occurrence: 1, Role: Database, Owner: 11},
			{Point: JournalUnlink, Occurrence: 1, Role: Journal, Owner: 12},
			{Point: FinalDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},
		},
		CallCount: 12,
	}
	spec.Target = spec.Calls[targetIndex]
	if action == Pass {
		return spec
	}
	spec.ExpectedCommit = CommitFailed
	if targetIndex >= 8 && targetIndex <= 10 {
		spec.ExpectedCommit = CommitRecoveryRequired
	}
	if targetIndex == 11 {
		spec.ExpectedCommit = CommitUncertain
	}
	if action == PrefixShortWrite {
		spec.Prefix = 7
	}
	spec.CallCount = uint8(targetIndex + 1)
	for i := targetIndex + 1; i < MaxCalls; i++ {
		spec.Calls[i] = Call{}
	}
	if targetIndex > 0 && targetIndex < 8 {
		spec.Calls[spec.CallCount] = Call{Point: CleanupUnlink, Occurrence: 1, Role: Journal, Owner: 12}
		spec.CallCount++
		spec.Calls[spec.CallCount] = Call{Point: CleanupDirectorySync, Occurrence: 1, Role: Directory, Owner: 13}
		spec.CallCount++
	}
	return spec
}

func fixtureEvents(spec PlanSpec) []Event {
	events := []Event{{Kind: RunStarted}}
	targetError := None
	for i := 0; i < int(spec.CallCount); i++ {
		call, action := spec.Calls[i], Pass
		if call == spec.Target {
			action = spec.Action
		}
		result := Result{Executed: true, NativeN: call.Length, ReturnedN: call.Length, Effect: EffectUnknown}
		if call.Point == DBOriginalRead {
			result.Effect = EffectNone
		}
		if writePoint(call.Point) {
			result.Effect = EffectPossible
		}
		switch action {
		case FailBefore:
			result = Result{ReturnedError: Injected, NormalizedError: Injected, Effect: EffectNone}
		case MisplaceIndexPageMinus64:
			result = Result{Executed: true, NativeN: 4096, ReturnedN: 4096, ReturnedError: Injected, NormalizedError: Injected, Effect: EffectPossible}
		case PrefixShortWrite:
			result = Result{Executed: true, NativeN: spec.Prefix, ReturnedN: spec.Prefix, NormalizedError: ShortWrite, Effect: EffectPrefix}
			if spec.Prefix == 0 {
				result.Effect = EffectNone
			}
			if spec.PrefixError {
				result.ReturnedError, result.NormalizedError = Injected, Injected
			}
		}
		if call == spec.Target {
			targetError = result.NormalizedError
		}
		events = append(events, Event{Kind: Intent, Call: call, Action: action}, Event{Kind: Returned, Call: call, Action: action, Result: result})
	}
	commit := CommitResult{Class: spec.ExpectedCommit, Logical: LogicalClosed, TargetError: targetError}
	owners := OwnersResult{MainOwner: 11, JournalOwner: 12, CleanupOwner: 14, Main: OwnerRetired, Journal: OwnerRetired, Cleanup: OwnerRetired}
	if spec.Action != Pass {
		commit.Error, commit.ErrorToken = Underlying, 900
	}
	if spec.Target.Point == JournalCreate && spec.Action == FailBefore {
		commit.Logical = LogicalOpen
		owners.Journal = OwnerNotAcquired
	}
	events = append(events, Event{Kind: CommitReturned, Commit: commit}, Event{Kind: OwnersObserved, Owners: owners}, Event{Kind: ValidationObserved}, Event{Kind: Terminal})
	renumber(events, spec.Identity)
	return events
}

func renumber(events []Event, identity Identity) {
	for i := range events {
		events[i].Identity, events[i].Ordinal = identity, uint8(i+1)
	}
}

func observe(t *testing.T, spec PlanSpec, events []Event) Observation {
	t.Helper()
	plan, problem := NewPlan(spec)
	if problem != NoProblem {
		t.Fatalf("fixture plan refused: %v", problem)
	}
	receipt, problem := NewReceipt(plan)
	if problem != NoProblem {
		t.Fatalf("fixture receipt refused: %v", problem)
	}
	for _, event := range events {
		if receipt.Append(event) != NoProblem {
			break
		}
	}
	return receipt.Seal()
}

func mustNotQualify(t *testing.T, observation Observation) {
	t.Helper()
	if observation.Outcome == AdapterObserved || observation.PhysicalValidated() || observation.Pass() {
		t.Fatalf("invalid evidence qualified: %+v", observation)
	}
}

func TestRollbackFaultFiniteActionsAreAdapterObservationOnly(t *testing.T) {
	for index := 0; index < 12; index++ {
		for _, action := range []Action{Pass, FailBefore, PrefixShortWrite} {
			if action == PrefixShortWrite && index != 1 && index != 3 && index != 5 && index != 8 {
				continue
			}
			spec := onePageSpec(action, index)
			observation := observe(t, spec, fixtureEvents(spec))
			if observation.Outcome != AdapterObserved || observation.TargetHits != 1 || observation.Problem != NoProblem {
				t.Fatalf("point %v action %v: %+v", spec.Target.Point, action, observation)
			}
			if observation.Pass() || observation.PhysicalValidated() {
				t.Fatal("adapter evidence became physical acceptance")
			}
		}
	}
	for _, prefix := range []int{0, 1, 4095} {
		for _, injectError := range []bool{false, true} {
			spec := onePageSpec(PrefixShortWrite, 8)
			spec.Prefix, spec.PrefixError = prefix, injectError
			if observation := observe(t, spec, fixtureEvents(spec)); observation.Outcome != AdapterObserved || observation.Pass() {
				t.Fatalf("prefix %d error %v: %+v", prefix, injectError, observation)
			}
		}
	}
}

func TestRollbackFaultPlanRejectsMalformedFrozenSelectors(t *testing.T) {
	for _, name := range []string{"suite", "source", "tool", "binary", "manifest", "trial", "zero-trial", "scope", "profile", "whitespace", "long-id", "point", "occurrence", "role", "offset", "length", "owner", "duplicate-owner", "zero-owner", "page-size", "sector-size", "zero-originals", "five-dirty", "action", "prefix-read", "prefix-negative", "prefix-full", "prefix-on-pass", "cleanup-target", "commit-success", "missing-call", "duplicated-call", "reordered-call", "hidden-tail", "zero-call-count", "oversized-call-count"} {
		spec := onePageSpec(FailBefore, 8)
		switch name {
		case "suite":
			spec.Identity.Suite = ""
		case "source":
			spec.Identity.Source = ""
		case "tool":
			spec.Identity.Tool = ""
		case "binary":
			spec.Identity.Binary = ""
		case "manifest":
			spec.Identity.Manifest = ""
		case "trial":
			spec.Identity.Trial = ""
		case "zero-trial":
			spec.Identity.Trial = "0"
		case "scope":
			spec.Identity.Scope = ""
		case "profile":
			spec.Identity.Profile = Profile(9)
		case "whitespace":
			spec.Identity.Scope = "scope 1"
		case "long-id":
			for len(spec.Identity.Trial) <= MaxIDLength {
				spec.Identity.Trial += "x"
			}
		case "point":
			spec.Target.Point = PointUnknown
		case "occurrence":
			spec.Target.Occurrence = 0
		case "role":
			spec.Target.Role = Journal
		case "offset":
			spec.Target.Offset = 4096
		case "length":
			spec.Target.Length--
		case "owner":
			spec.Target.Owner = 99
		case "duplicate-owner":
			spec.DirectoryOwner = spec.MainOwner
		case "zero-owner":
			spec.CleanupOwner = 0
		case "page-size":
			spec.PageSize = 8192
		case "sector-size":
			spec.SectorSize = 512
		case "zero-originals":
			spec.OriginalPages = 0
		case "five-dirty":
			spec.DirtyPages = 5
		case "action":
			spec.Action = Action(99)
		case "prefix-read":
			spec = onePageSpec(PrefixShortWrite, 2)
		case "prefix-negative":
			spec.Action, spec.Prefix = PrefixShortWrite, -1
		case "prefix-full":
			spec.Action, spec.Prefix = PrefixShortWrite, spec.Target.Length
		case "prefix-on-pass":
			spec.Prefix = 1
		case "cleanup-target":
			spec.Target.Point = CleanupUnlink
		case "commit-success":
			spec.ExpectedCommit = CommitSucceeded
		case "missing-call":
			spec.Calls[3] = spec.Calls[4]
		case "duplicated-call":
			spec.Calls[4] = spec.Calls[3]
		case "reordered-call":
			spec.Calls[2], spec.Calls[3] = spec.Calls[3], spec.Calls[2]
		case "hidden-tail":
			spec.Calls[spec.CallCount] = spec.Calls[0]
		case "zero-call-count":
			spec.CallCount = 0
		case "oversized-call-count":
			spec.CallCount = MaxCalls + 1
		}
		if _, problem := NewPlan(spec); problem == NoProblem {
			t.Fatalf("malformed %s selector admitted", name)
		}
	}
}

func TestRollbackFaultEveryTruncationAndOutOfOrderEventFailsClosed(t *testing.T) {
	spec := onePageSpec(FailBefore, 8)
	events := fixtureEvents(spec)
	for end := 0; end < len(events); end++ {
		mustNotQualify(t, observe(t, spec, events[:end]))
	}
	for index := range events {
		// Renumbering defeats a mere ordinal/checksum guard; structural omissions
		// and duplicates must still fail against the complete finite call path.
		missing := append([]Event{}, events[:index]...)
		missing = append(missing, events[index+1:]...)
		renumber(missing, spec.Identity)
		mustNotQualify(t, observe(t, spec, missing))
		duplicate := append([]Event{}, events[:index]...)
		duplicate = append(duplicate, events[index])
		duplicate = append(duplicate, events[index:]...)
		renumber(duplicate, spec.Identity)
		mustNotQualify(t, observe(t, spec, duplicate))
		if index+1 < len(events) {
			reordered := append([]Event{}, events...)
			reordered[index], reordered[index+1] = reordered[index+1], reordered[index]
			renumber(reordered, spec.Identity)
			mustNotQualify(t, observe(t, spec, reordered))
		}
	}
}

func TestRollbackFaultStaleIdentityAndMismatchedCallFailClosed(t *testing.T) {
	for _, name := range []string{"suite", "source", "tool", "binary", "manifest", "trial", "scope", "profile", "ordinal", "role", "owner", "offset", "length", "occurrence", "not-hit", "wrong-action", "hidden-payload", "native-return-missing"} {
		spec := onePageSpec(FailBefore, 8)
		events := fixtureEvents(spec)
		index := 1 + 2*8
		event := &events[index]
		switch name {
		case "suite":
			event.Identity.Suite = "old-suite"
		case "source":
			event.Identity.Source = "old-source"
		case "tool":
			event.Identity.Tool = "old-tool"
		case "binary":
			event.Identity.Binary = "old-binary"
		case "manifest":
			event.Identity.Manifest = "old-manifest"
		case "trial":
			event.Identity.Trial = "old-trial"
		case "scope":
			event.Identity.Scope = "old-scope"
		case "profile":
			event.Identity.Profile = ProfileUnknown
		case "ordinal":
			event.Ordinal++
		case "role":
			event.Call.Role = Journal
		case "owner":
			event.Call.Owner = 12
		case "offset":
			event.Call.Offset = 4096
		case "length":
			event.Call.Length--
		case "occurrence":
			event.Call.Occurrence++
		case "not-hit":
			event.Call.Point = DBSync
		case "wrong-action":
			event.Action = Pass
		case "hidden-payload":
			event.Owners.Main = OwnerRetired
		case "native-return-missing":
			events[index+1].Kind = CommitReturned
		}
		mustNotQualify(t, observe(t, spec, events))
	}
}

func TestRollbackFaultUnderlyingErrorNeverBecomesExpectedInjection(t *testing.T) {
	for _, mutation := range []string{"preserved", "masked-native", "masked-exposed", "masked-normalized", "wrong-token", "missing-token", "commit-cause-masked"} {
		spec := onePageSpec(PrefixShortWrite, 8)
		events := fixtureEvents(spec)
		result := &events[2+2*8].Result
		*result = Result{Executed: true, NativeN: 3, NativeError: Underlying, NativeErrorToken: 71, ReturnedN: 3, ReturnedError: Underlying, ReturnedErrorToken: 71, NormalizedError: Underlying, NormalizedErrorToken: 71, Effect: EffectUnknown}
		commit := &events[len(events)-4].Commit
		commit.TargetError, commit.TargetErrorToken = Underlying, 71
		switch mutation {
		case "masked-native":
			result.NativeError, result.NativeErrorToken = None, 0
		case "masked-exposed":
			result.ReturnedError, result.ReturnedErrorToken = Injected, 0
		case "masked-normalized":
			result.NormalizedError, result.NormalizedErrorToken = ShortWrite, 0
		case "wrong-token":
			result.NormalizedErrorToken = 72
		case "missing-token":
			result.NativeErrorToken = 0
		case "commit-cause-masked":
			commit.TargetError, commit.TargetErrorToken = Injected, 0
		}
		observation := observe(t, spec, events)
		mustNotQualify(t, observation)
		if mutation == "preserved" && (observation.Outcome != ObservationRefused || observation.Problem != ProblemUnderlyingFailure) {
			t.Fatalf("preserved original error not distinguished from contract invalidity: %+v", observation)
		}
	}
}

func TestRollbackFaultOwnersAndLogicalClosureAreIndependent(t *testing.T) {
	for _, state := range []OwnerState{OwnerUnknown, OwnerLive, OwnerPending} {
		for owner := 0; owner < 3; owner++ {
			spec := onePageSpec(FailBefore, 8)
			events := fixtureEvents(spec)
			observed := &events[len(events)-3].Owners
			switch owner {
			case 0:
				observed.Main = state
			case 1:
				observed.Journal = state
			case 2:
				observed.Cleanup = state
			}
			observation := observe(t, spec, events)
			mustNotQualify(t, observation)
			if observation.Problem != ProblemOwnersUnresolved {
				t.Fatalf("logical close incorrectly resolved owner %d state %v: %+v", owner, state, observation)
			}
		}
	}
	for _, mutation := range []string{"wrong-main", "wrong-journal", "wrong-cleanup", "journal-gone", "main-not-acquired", "cleanup-not-acquired", "logical-unknown", "logical-open"} {
		spec := onePageSpec(FailBefore, 8)
		events := fixtureEvents(spec)
		owners := &events[len(events)-3].Owners
		switch mutation {
		case "wrong-main":
			owners.MainOwner++
		case "wrong-journal":
			owners.JournalOwner++
		case "wrong-cleanup":
			owners.CleanupOwner++
		case "journal-gone":
			owners.Journal = OwnerNotAcquired
		case "main-not-acquired":
			owners.Main = OwnerNotAcquired
		case "cleanup-not-acquired":
			owners.Cleanup = OwnerNotAcquired
		case "logical-unknown":
			events[len(events)-4].Commit.Logical = LogicalUnknown
		case "logical-open":
			events[len(events)-4].Commit.Logical = LogicalOpen
		}
		mustNotQualify(t, observe(t, spec, events))
	}
}

func TestRollbackFaultUncertainAndUnqualifiedCommitNeverPass(t *testing.T) {
	for _, class := range []CommitClass{CommitUnknown, CommitSucceeded, CommitFailed, CommitUncertain, CommitCleanupPending, CommitUnsupported, CommitRefused, CommitSkipped} {
		spec := onePageSpec(FailBefore, 8)
		events := fixtureEvents(spec)
		events[len(events)-4].Commit.Class = class
		if class == CommitSucceeded {
			events[len(events)-4].Commit.Error, events[len(events)-4].Commit.ErrorToken = None, 0
		}
		mustNotQualify(t, observe(t, spec, events))
	}
	// DB write fail-before still requires recovery in the production protocol,
	// despite this individual operation reporting no native execution or effect.
	db := onePageSpec(FailBefore, 8)
	if db.ExpectedCommit != CommitRecoveryRequired {
		t.Fatal("fixture lost conservative databaseTouched classification")
	}
	final := onePageSpec(FailBefore, 11)
	events := fixtureEvents(final)
	observation := observe(t, final, events)
	if observation.Outcome != AdapterObserved || observation.PhysicalValidated() || observation.Pass() {
		t.Fatalf("uncertain commit was upgraded to physical success: %+v", observation)
	}
	events[len(events)-4].Commit.Class = CommitSucceeded
	events[len(events)-4].Commit.Error, events[len(events)-4].Commit.ErrorToken = None, 0
	mustNotQualify(t, observe(t, final, events))
}

func TestRollbackFaultPhysicalEvidenceCannotBeManufactured(t *testing.T) {
	spec := onePageSpec(Pass, 8)
	for _, claim := range []Validation{1, 2, 3, 255} {
		events := fixtureEvents(spec)
		events[len(events)-2].Validation = claim
		mustNotQualify(t, observe(t, spec, events))
	}
	// No recovery, old/new ledger, index verification, or acknowledged-commit
	// ledger can be attached through this model. Even a fabricated outcome value
	// has no method that grants physical acceptance.
	for _, outcome := range []Outcome{ObservationInvalid, ObservationIncomplete, ObservationRefused, AdapterObserved, Outcome(255)} {
		observation := Observation{Outcome: outcome}
		if observation.Pass() || observation.PhysicalValidated() {
			t.Fatal("manufactured physical validation")
		}
	}
}

func TestRollbackFaultValueCopiesAndStickyTerminal(t *testing.T) {
	spec := onePageSpec(FailBefore, 8)
	plan, problem := NewPlan(spec)
	if problem != NoProblem {
		t.Fatal(problem)
	}
	spec.Calls[0].Owner = 999
	copySpec := plan.Spec()
	copySpec.Identity.Trial, copySpec.Calls[0].Owner = "altered", 998
	if plan.Spec().Calls[0].Owner != 12 || plan.Spec().Identity.Trial != "trial-1" {
		t.Fatal("plan retains mutable caller storage")
	}
	receipt, problem := NewReceipt(plan)
	if problem != NoProblem {
		t.Fatal(problem)
	}
	events := fixtureEvents(plan.Spec())
	for _, event := range events {
		if problem := receipt.Append(event); problem != NoProblem {
			t.Fatal(problem)
		}
	}
	snapshot, count := receipt.Events()
	snapshot[0].Identity.Scope = "old-scope"
	second, secondCount := receipt.Events()
	if count != secondCount || second[0].Identity.Scope != "scope-1" {
		t.Fatal("receipt snapshot aliases internal evidence")
	}
	first := receipt.Seal()
	if problem := receipt.Append(events[0]); problem != ProblemSealed {
		t.Fatal(problem)
	}
	if again := receipt.Seal(); again != first {
		t.Fatal("sealing changed terminal receipt")
	}
	bad, _ := NewReceipt(plan)
	malformed := events[0]
	malformed.Ordinal = 9
	if bad.Append(malformed) != ProblemOrdinal || bad.Append(events[0]) != ProblemOrdinal {
		t.Fatal("invalid receipt was silently reset as a new trial")
	}
	mustNotQualify(t, bad.Seal())
	if _, problem := NewReceipt(Plan{}); problem != ProblemUnadmittedPlan {
		t.Fatal(problem)
	}
	var zero Receipt
	if problem := zero.Append(events[0]); problem != ProblemUnadmittedPlan {
		t.Fatal(problem)
	}
	mustNotQualify(t, zero.Seal())
}

func TestRollbackFaultBoundedStorageCannotOverflowOrBecomeNewTrial(t *testing.T) {
	spec := onePageSpec(FailBefore, 8)
	plan, problem := NewPlan(spec)
	if problem != NoProblem {
		t.Fatal(problem)
	}
	receipt, _ := NewReceipt(plan)
	// The admitted finite path normally rejects long streams before this bound.
	// This same-package source test addresses the fixed-array overflow guard
	// independently without executing any adapter, fixture, or native operation.
	receipt.count = MaxEvents
	if receipt.Append(Event{}) != ProblemEventLimit {
		t.Fatal("event bound not enforced")
	}
	if receipt.count != MaxEvents {
		t.Fatal("overflow wrapped ordinal counter")
	}
	mustNotQualify(t, receipt.Seal())
	complete, _ := NewReceipt(plan)
	for _, event := range fixtureEvents(spec) {
		if complete.Append(event) != NoProblem {
			t.Fatal("fixture rejected")
		}
	}
	extra := Event{Identity: spec.Identity, Ordinal: complete.count + 1, Kind: RunStarted}
	if complete.Append(extra) != ProblemEventOrder {
		t.Fatal("second trial admitted after terminal")
	}
	mustNotQualify(t, complete.Seal())
}

func TestRollbackFaultFourPageManifestUsesPerPointOccurrences(t *testing.T) {
	spec := onePageSpec(Pass, 8)
	spec.OriginalPages, spec.DirtyPages, spec.CallCount = 4, 4, 21
	spec.Calls = [MaxCalls]Call{
		{Point: JournalCreate, Occurrence: 1, Role: Journal, Owner: 12},
		{Point: JournalInvalid, Occurrence: 1, Role: Journal, Owner: 12, Length: 4096},
		{Point: DBOriginalRead, Occurrence: 1, Role: Database, Owner: 11, Length: 4096},
		{Point: JournalRecord, Occurrence: 1, Role: Journal, Owner: 12, Offset: 4096, Length: 4104},
		{Point: DBOriginalRead, Occurrence: 2, Role: Database, Owner: 11, Offset: 4096, Length: 4096},
		{Point: JournalRecord, Occurrence: 2, Role: Journal, Owner: 12, Offset: 8200, Length: 4104},
		{Point: DBOriginalRead, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192, Length: 4096},
		{Point: JournalRecord, Occurrence: 3, Role: Journal, Owner: 12, Offset: 12304, Length: 4104},
		{Point: DBOriginalRead, Occurrence: 4, Role: Database, Owner: 11, Offset: 12288, Length: 4096},
		{Point: JournalRecord, Occurrence: 4, Role: Journal, Owner: 12, Offset: 16408, Length: 4104},
		{Point: JournalBodySync, Occurrence: 1, Role: Journal, Owner: 12},
		{Point: JournalValidHeader, Occurrence: 1, Role: Journal, Owner: 12, Length: 12},
		{Point: JournalHeaderSync, Occurrence: 1, Role: Journal, Owner: 12},
		{Point: PublishDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},
		{Point: DBPage, Occurrence: 1, Role: Database, Owner: 11, Length: 4096},
		{Point: DBPage, Occurrence: 2, Role: Database, Owner: 11, Offset: 4096, Length: 4096},
		{Point: DBPage, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192, Length: 4096},
		{Point: DBPage, Occurrence: 4, Role: Database, Owner: 11, Offset: 12288, Length: 4096},
		{Point: DBSync, Occurrence: 1, Role: Database, Owner: 11},
		{Point: JournalUnlink, Occurrence: 1, Role: Journal, Owner: 12},
		{Point: FinalDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},
	}
	spec.Target = spec.Calls[17]
	observation := observe(t, spec, fixtureEvents(spec))
	if observation.Outcome != AdapterObserved || observation.EventCount != 47 || observation.TargetHits != 1 {
		t.Fatalf("four-page finite trace rejected: %+v", observation)
	}
	for _, mutation := range []string{"repeat-page", "misaligned-page", "outside-bound", "wrong-record-offset", "wrong-record-length", "global-ordinal", "fewer-originals", "fewer-dirty"} {
		changed := spec
		switch mutation {
		case "repeat-page":
			changed.Calls[15].Offset = 0
		case "misaligned-page":
			changed.Calls[15].Offset++
		case "outside-bound":
			changed.Calls[17].Offset = 4 * 4096
		case "wrong-record-offset":
			changed.Calls[5].Offset = 8192
		case "wrong-record-length":
			changed.Calls[5].Length = 4096
		case "global-ordinal":
			changed.Calls[5].Occurrence = 6
		case "fewer-originals":
			changed.OriginalPages = 3
		case "fewer-dirty":
			changed.DirtyPages = 3
		}
		if _, problem := NewPlan(changed); problem == NoProblem {
			t.Fatalf("invalid multi-page %s accepted", mutation)
		}
	}
	spec.Action, spec.Prefix, spec.ExpectedCommit, spec.CallCount = PrefixShortWrite, 4095, CommitRecoveryRequired, 18
	for i := 18; i < MaxCalls; i++ {
		spec.Calls[i] = Call{}
	}
	if observation := observe(t, spec, fixtureEvents(spec)); observation.Outcome != AdapterObserved || observation.TargetHits != 1 {
		t.Fatalf("fourth occurrence prefix receipt rejected: %+v", observation)
	}
}

func TestRollbackFaultResultNormalizationAndEffectClaimsFailClosed(t *testing.T) {
	for _, mutation := range []string{"executed-fail-before", "native-fail-before", "fail-before-effect", "negative-native-count", "excess-native-count", "wrong-exposed-count", "masked-short-write", "wrong-prefix-count", "prefix-effect-none", "prefix-effect-unknown", "fake-native-injected", "synthetic-token", "pass-write-effect-none", "read-short-nil", "read-fake-effect", "create-fake-no-effect", "native-write-fake-no-effect"} {
		spec := onePageSpec(PrefixShortWrite, 8)
		if mutation == "executed-fail-before" || mutation == "native-fail-before" || mutation == "fail-before-effect" {
			spec = onePageSpec(FailBefore, 8)
		}
		events := fixtureEvents(spec)
		result := &events[18].Result
		switch mutation {
		case "executed-fail-before":
			result.Executed = true
		case "native-fail-before":
			result.NativeN = 1
		case "fail-before-effect":
			result.Effect = EffectPrefix
		case "negative-native-count":
			result.NativeN = -1
		case "excess-native-count":
			result.NativeN = 4097
		case "wrong-exposed-count":
			result.ReturnedN++
		case "masked-short-write":
			result.NormalizedError = None
		case "wrong-prefix-count":
			result.NativeN, result.ReturnedN = 6, 6
		case "prefix-effect-none":
			result.Effect = EffectNone
		case "prefix-effect-unknown":
			result.Effect = EffectUnknown
		case "fake-native-injected":
			result.NativeError = Injected
		case "synthetic-token":
			result.NormalizedErrorToken = 99
		case "pass-write-effect-none":
			events[4].Result.Effect = EffectNone
		case "read-short-nil":
			events[6].Result.NativeN, events[6].Result.ReturnedN = 4095, 4095
		case "read-fake-effect":
			events[6].Result.Effect = EffectPrefix
		case "create-fake-no-effect":
			events[2].Result.Effect = EffectNone
		case "native-write-fake-no-effect":
			*result = Result{Executed: true, NativeN: 3, NativeError: Underlying, NativeErrorToken: 77, ReturnedN: 3, ReturnedError: Underlying, ReturnedErrorToken: 77, NormalizedError: Underlying, NormalizedErrorToken: 77, Effect: EffectNone}
			events[len(events)-4].Commit.TargetError, events[len(events)-4].Commit.TargetErrorToken = Underlying, 77
		}
		mustNotQualify(t, observe(t, spec, events))
	}
}

func TestRollbackFaultAcknowledgementAndRecoveryLedgersRemainUnproven(t *testing.T) {
	for _, action := range []Action{Pass, FailBefore} {
		spec := onePageSpec(action, 8)
		observation := observe(t, spec, fixtureEvents(spec))
		if observation.Outcome != AdapterObserved {
			t.Fatal("fixture observation malformed")
		}
		// This complete trace supplies no independent old/new ledger, ordinary
		// index or extension-index check, or durable acknowledgement history.
		// Neither success nor recovery-required can certify those missing facts.
		if observation.Pass() || observation.PhysicalValidated() {
			t.Fatal("absent recovery/ledger/index/acknowledgement evidence treated as PASS")
		}
	}
}

func TestRollbackFaultCleanupIsCompleteAndObservationOnly(t *testing.T) {
	spec := onePageSpec(FailBefore, 6)
	for _, mutation := range []string{"second-fault-unlink", "second-fault-directory", "missing-cleanup-pair", "reversed-cleanup", "wrong-cleanup-owner", "spontaneous-cleanup-error"} {
		events := fixtureEvents(spec)
		unlinkIntent := 1 + 2*(int(spec.CallCount)-2)
		directoryIntent := unlinkIntent + 2
		switch mutation {
		case "second-fault-unlink":
			events[unlinkIntent].Action = FailBefore
		case "second-fault-directory":
			events[directoryIntent].Action = FailBefore
		case "missing-cleanup-pair":
			events = append(events[:unlinkIntent], events[directoryIntent:]...)
			renumber(events, spec.Identity)
		case "reversed-cleanup":
			events[unlinkIntent].Call, events[directoryIntent].Call = events[directoryIntent].Call, events[unlinkIntent].Call
		case "wrong-cleanup-owner":
			events[directoryIntent].Call.Owner = spec.CleanupOwner
		case "spontaneous-cleanup-error":
			events[directoryIntent+1].Result = Result{Executed: true, NativeError: Underlying, NativeErrorToken: 55, ReturnedError: Underlying, ReturnedErrorToken: 55, NormalizedError: Underlying, NormalizedErrorToken: 55, Effect: EffectUnknown}
		}
		mustNotQualify(t, observe(t, spec, events))
	}
	for _, mutation := range []string{"omit-cleanup", "omit-directory", "third-cleanup"} {
		changed := spec
		switch mutation {
		case "omit-cleanup":
			changed.Calls[changed.CallCount-2], changed.Calls[changed.CallCount-1] = Call{}, Call{}
			changed.CallCount -= 2
		case "omit-directory":
			changed.Calls[changed.CallCount-1] = Call{}
			changed.CallCount--
		case "third-cleanup":
			changed.Calls[changed.CallCount] = changed.Calls[changed.CallCount-1]
			changed.CallCount++
		}
		if _, problem := NewPlan(changed); problem == NoProblem {
			t.Fatalf("malformed cleanup inventory %s admitted", mutation)
		}
	}
}

// RF-01 SOURCE ONLY: these regressions describe bounded retained evidence. They
// do not measure heap allocation or establish native or physical qualification.
func TestRollbackFaultEveryIdentityFieldRejectsOversizedRetention(t *testing.T) {
	for _, size := range []int{MaxIDLength + 1, 1 << 20} {
		payload := make([]byte, size)
		for i := range payload {
			payload[i] = 'x'
		}
		value := string(payload)
		for _, field := range []string{"suite", "source", "tool", "binary", "manifest", "trial", "scope"} {
			spec := onePageSpec(FailBefore, 8)
			plan, problem := NewPlan(spec)
			if problem != NoProblem {
				t.Fatal(problem)
			}
			badSpec := spec
			switch field {
			case "suite":
				badSpec.Identity.Suite = value
			case "source":
				badSpec.Identity.Source = value
			case "tool":
				badSpec.Identity.Tool = value
			case "binary":
				badSpec.Identity.Binary = value
			case "manifest":
				badSpec.Identity.Manifest = value
			case "trial":
				badSpec.Identity.Trial = value
			case "scope":
				badSpec.Identity.Scope = value
			}
			if rejected, problem := NewPlan(badSpec); problem != ProblemIdentity || rejected != (Plan{}) {
				t.Fatalf("oversized %s length %d retained in plan: problem %v", field, size, problem)
			}
			receipt, problem := NewReceipt(plan)
			if problem != NoProblem {
				t.Fatal(problem)
			}
			bad := Event{
				Identity: badSpec.Identity, Ordinal: 251, Kind: Returned,
				Call: spec.Target, Action: PrefixShortWrite,
				Result:     Result{Executed: true, NativeN: 99, NativeError: Underlying, NativeErrorToken: 77, ReturnedN: 98, ReturnedError: Underlying, ReturnedErrorToken: 78, NormalizedError: Underlying, NormalizedErrorToken: 79, Effect: EffectPossible},
				Commit:     CommitResult{Class: CommitSucceeded, Error: Underlying, ErrorToken: 80, TargetError: Underlying, TargetErrorToken: 81, Logical: LogicalClosed},
				Owners:     OwnersResult{MainOwner: 91, JournalOwner: 92, CleanupOwner: 93, Main: OwnerRetired, Journal: OwnerRetired, Cleanup: OwnerRetired},
				Validation: 255, Rejection: ProblemEventOrder, RejectedAt: 250,
			}
			if problem := receipt.Append(bad); problem != ProblemIdentity {
				t.Fatalf("oversized %s length %d lost identity precedence: %v", field, size, problem)
			}
			want := [MaxEvents]Event{{Ordinal: 251, Kind: EventIdentityRejected, Rejection: ProblemIdentity, RejectedAt: 1}}
			snapshot, count := receipt.Events()
			if count != 1 || snapshot != want {
				t.Fatalf("oversized %s length %d retained payload or lost original ordinal/actual slot", field, size)
			}
			if receipt.next != RunStarted || receipt.callIndex != 0 || receipt.targetHits != 0 || receipt.targetResult != (Result{}) {
				t.Fatalf("oversized %s length %d advanced accepted receipt state", field, size)
			}
			observation := receipt.Seal()
			mustNotQualify(t, observation)
			if observation.Outcome != ObservationInvalid || observation.Problem != ProblemIdentity || observation.EventCount != 1 || observation.TargetHits != 0 {
				t.Fatalf("oversized %s length %d lost terminal failure evidence: %+v", field, size, observation)
			}
		}
	}
}

func TestRollbackFaultIdentityRejectionKeepsFirstMarkerAndDetachedSnapshot(t *testing.T) {
	payload := make([]byte, 1<<20)
	for i := range payload {
		payload[i] = 'x'
	}
	for _, prior := range []int{0, 1} {
		spec := onePageSpec(FailBefore, 8)
		plan, problem := NewPlan(spec)
		if problem != NoProblem {
			t.Fatal(problem)
		}
		receipt, problem := NewReceipt(plan)
		if problem != NoProblem {
			t.Fatal(problem)
		}
		events := fixtureEvents(spec)
		want := [MaxEvents]Event{}
		for i := 0; i < prior; i++ {
			if problem := receipt.Append(events[i]); problem != NoProblem {
				t.Fatal(problem)
			}
			want[i] = events[i]
		}
		bad := events[prior]
		bad.Identity.Scope, bad.Ordinal = string(payload), 251
		if problem := receipt.Append(bad); problem != ProblemIdentity {
			t.Fatalf("identity rejection after %d valid events lost precedence over wrong ordinal: %v", prior, problem)
		}
		want[prior] = Event{Ordinal: 251, Kind: EventIdentityRejected, Rejection: ProblemIdentity, RejectedAt: uint8(prior + 1)}
		snapshot, count := receipt.Events()
		if count != uint8(prior+1) || snapshot != want {
			t.Fatalf("first rejection after %d valid events lost prefix, original ordinal, actual slot, or zero payload", prior)
		}
		// Mutating both an earlier event and the marker in a returned snapshot
		// must leave the original finite failure evidence untouched.
		snapshot[prior] = events[prior]
		snapshot[0].Identity.Scope = "changed-snapshot"
		second, secondCount := receipt.Events()
		if secondCount != count || second != want {
			t.Fatal("rejected receipt snapshot aliases internal evidence")
		}
		bad.Identity.Trial, bad.Ordinal = string(payload), 19
		for _, retry := range []Event{events[prior], bad, events[0]} {
			if problem := receipt.Append(retry); problem != ProblemIdentity {
				t.Fatalf("retry after identity failure changed sticky problem: %v", problem)
			}
			retained, retainedCount := receipt.Events()
			if retainedCount != count || retained != want {
				t.Fatal("retry replaced or extended the first rejection evidence")
			}
		}
		observation := receipt.Seal()
		mustNotQualify(t, observation)
		if observation.Outcome != ObservationInvalid || observation.Problem != ProblemIdentity || observation.EventCount != count || observation.TargetHits != 0 {
			t.Fatalf("sticky identity failure lost terminal evidence: %+v", observation)
		}
		if again := receipt.Seal(); again != observation {
			t.Fatal("repeated seal changed identity rejection")
		}
		if problem := receipt.Append(events[prior]); problem != ProblemSealed {
			t.Fatal(problem)
		}
		retained, retainedCount := receipt.Events()
		if retainedCount != count || retained != want {
			t.Fatal("sealed receipt changed first rejection evidence")
		}
	}
}

func TestRollbackFaultStaleIdentityAndBadProfileUseBoundedMarkers(t *testing.T) {
	for _, mutation := range []string{"stale-trial", "stale-scope", "unknown-profile", "unsupported-profile"} {
		spec := onePageSpec(FailBefore, 8)
		plan, problem := NewPlan(spec)
		if problem != NoProblem {
			t.Fatal(problem)
		}
		receipt, problem := NewReceipt(plan)
		if problem != NoProblem {
			t.Fatal(problem)
		}
		bad := fixtureEvents(spec)[0]
		switch mutation {
		case "stale-trial":
			bad.Identity.Trial = "old-trial"
		case "stale-scope":
			bad.Identity.Scope = "old-scope"
		case "unknown-profile":
			bad.Identity.Profile = ProfileUnknown
		case "unsupported-profile":
			bad.Identity.Profile = Profile(255)
		}
		if problem := receipt.Append(bad); problem != ProblemIdentity {
			t.Fatalf("%s did not reject identity before retention: %v", mutation, problem)
		}
		want := [MaxEvents]Event{{Ordinal: 1, Kind: EventIdentityRejected, Rejection: ProblemIdentity, RejectedAt: 1}}
		snapshot, count := receipt.Events()
		if count != 1 || snapshot != want {
			t.Fatalf("%s did not retain only a bounded identity marker", mutation)
		}
		observation := receipt.Seal()
		mustNotQualify(t, observation)
		if observation.Problem != ProblemIdentity || observation.EventCount != 1 {
			t.Fatalf("%s lost identity rejection evidence: %+v", mutation, observation)
		}
	}
}

func TestRollbackFaultRejectionMarkerAndMetadataCannotQualify(t *testing.T) {
	spec := onePageSpec(Pass, 8)
	for _, annotated := range []bool{false, true} {
		events := fixtureEvents(spec)
		events[0].Kind = EventIdentityRejected
		if annotated {
			events[0].Rejection, events[0].RejectedAt = ProblemIdentity, 1
		}
		observation := observe(t, spec, events)
		mustNotQualify(t, observation)
		if observation.Problem != ProblemEventOrder || observation.EventCount != 1 {
			t.Fatalf("forged marker became an admissible event: %+v", observation)
		}
	}
	// Every normal event kind must keep both marker-only fields zero, including
	// an otherwise complete terminal event at the end of an admitted trace.
	for index := range fixtureEvents(spec) {
		for _, field := range []string{"rejection", "rejected-at", "both"} {
			events := fixtureEvents(spec)
			if field != "rejected-at" {
				events[index].Rejection = ProblemIdentity
			}
			if field != "rejection" {
				events[index].RejectedAt = uint8(index + 1)
			}
			observation := observe(t, spec, events)
			mustNotQualify(t, observation)
			if observation.Problem != ProblemEventPayload || observation.EventCount != uint8(index+1) {
				t.Fatalf("normal kind %v with %s marker metadata qualified: %+v", events[index].Kind, field, observation)
			}
		}
	}
}

func TestRollbackFaultLargeBackedValidSubstringsPreserveAdmittedValues(t *testing.T) {
	spec := onePageSpec(Pass, 8)
	want := spec
	values := [7]string{spec.Identity.Suite, spec.Identity.Source, spec.Identity.Tool, spec.Identity.Binary, spec.Identity.Manifest, spec.Identity.Trial, spec.Identity.Scope}
	for i, value := range values {
		payload := make([]byte, 1<<20)
		for j := range payload {
			payload[j] = 'x'
		}
		copy(payload, value)
		backing := string(payload)
		values[i] = backing[:len(value)]
	}
	spec.Identity = Identity{Suite: values[0], Source: values[1], Tool: values[2], Binary: values[3], Manifest: values[4], Trial: values[5], Scope: values[6], Profile: PlainRollback4096}
	plan, problem := NewPlan(spec)
	if problem != NoProblem {
		t.Fatal(problem)
	}
	if plan.Spec() != want {
		t.Fatal("valid substring admission changed frozen values")
	}
	receipt, problem := NewReceipt(plan)
	if problem != NoProblem {
		t.Fatal(problem)
	}
	// Event identities also arrive as short substrings of large source strings.
	// Value checks exercise the admitted path; the explicit bounded copy in the
	// implementation, not this source test, addresses backing-storage retention.
	events := fixtureEvents(spec)
	spec.Identity = Identity{}
	for i := range values {
		values[i] = "changed-source-value"
	}
	if plan.Spec() != want {
		t.Fatal("caller reassignment changed frozen plan")
	}
	for _, event := range events {
		if problem := receipt.Append(event); problem != NoProblem {
			t.Fatal(problem)
		}
	}
	snapshot, count := receipt.Events()
	if count != uint8(len(events)) {
		t.Fatal("valid substring receipt lost events")
	}
	for i := 0; i < int(count); i++ {
		if snapshot[i] != events[i] || snapshot[i].Identity != want.Identity || snapshot[i].Rejection != NoProblem || snapshot[i].RejectedAt != 0 {
			t.Fatalf("valid substring event %d lost canonical admitted values", i)
		}
	}
	observation := receipt.Seal()
	if observation.Outcome != AdapterObserved || observation.Problem != NoProblem || observation.TargetHits != 1 || observation.EventCount != count {
		t.Fatalf("valid substring receipt was refused: %+v", observation)
	}
	if observation.Pass() || observation.PhysicalValidated() {
		t.Fatal("valid substring receipt became physical PASS")
	}
}

// Independent literal plan: this fixture does not call the SQL plan builder.
func misplacedIndexSpec() PlanSpec {
	spec := PlanSpec{
		Identity:  onePageSpec(Pass, 8).Identity,
		MainOwner: 11, JournalOwner: 12, DirectoryOwner: 13, CleanupOwner: 14,
		OriginalPages: 3, DirtyPages: 3, PageSize: 4096, SectorSize: 4096,
		Action: MisplaceIndexPageMinus64, ExpectedCommit: CommitRecoveryRequired, CallCount: 15,
		Calls: [MaxCalls]Call{
			{Point: JournalCreate, Occurrence: 1, Role: Journal, Owner: 12},
			{Point: JournalInvalid, Occurrence: 1, Role: Journal, Owner: 12, Length: 4096},
			{Point: DBOriginalRead, Occurrence: 1, Role: Database, Owner: 11, Offset: 0, Length: 4096},
			{Point: JournalRecord, Occurrence: 1, Role: Journal, Owner: 12, Offset: 4096, Length: 4104},
			{Point: DBOriginalRead, Occurrence: 2, Role: Database, Owner: 11, Offset: 4096, Length: 4096},
			{Point: JournalRecord, Occurrence: 2, Role: Journal, Owner: 12, Offset: 8200, Length: 4104},
			{Point: DBOriginalRead, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192, Length: 4096},
			{Point: JournalRecord, Occurrence: 3, Role: Journal, Owner: 12, Offset: 12304, Length: 4104},
			{Point: JournalBodySync, Occurrence: 1, Role: Journal, Owner: 12},
			{Point: JournalValidHeader, Occurrence: 1, Role: Journal, Owner: 12, Length: 12},
			{Point: JournalHeaderSync, Occurrence: 1, Role: Journal, Owner: 12},
			{Point: PublishDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},
			{Point: DBPage, Occurrence: 1, Role: Database, Owner: 11, Offset: 0, Length: 4096},
			{Point: DBPage, Occurrence: 2, Role: Database, Owner: 11, Offset: 4096, Length: 4096},
			{Point: DBPage, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192, Length: 4096},
		},
	}
	spec.Target = spec.Calls[14]
	return spec
}

// The only additional action is a fixed actual placement, not a caller offset.
func TestRollbackFaultMisplacementAdmissionIsExact(t *testing.T) {
	plan, problem := NewPlan(misplacedIndexSpec())
	if problem != NoProblem {
		t.Fatal(problem)
	}
	spec := plan.Spec()
	if spec.Action != MisplaceIndexPageMinus64 || spec.CallCount != 15 || spec.Target != (Call{Point: DBPage, Occurrence: 3, Role: Database, Offset: 8192, Length: 4096, Owner: 11}) || spec.Prefix != 0 || spec.PrefixError || spec.ExpectedCommit != CommitRecoveryRequired {
		t.Fatal("fixed misplacement plan differs", spec)
	}
	observation := observe(t, spec, fixtureEvents(spec))
	if observation.Outcome != AdapterObserved || observation.EventCount != 35 || observation.TargetHits != 1 || observation.Problem != NoProblem || observation.Pass() || observation.PhysicalValidated() {
		t.Fatal("one complete misplaced-write trace must remain nonphysical", observation)
	}
	for _, name := range []string{"occurrence", "journal-role", "directory-role", "owner", "offset-minus64", "offset-plus64", "offset-zero", "length-prefix", "length-short", "dirty-two", "dirty-four", "original-two", "original-four", "page-size", "sector-size", "prefix", "prefix-error", "call-count", "success", "uncertain", "original-read-inventory", "dirty-write-inventory"} {
		bad := spec
		switch name {
		case "occurrence":
			bad.Target.Occurrence = 2
		case "journal-role":
			bad.Target.Role = Journal
		case "directory-role":
			bad.Target.Role = Directory
		case "owner":
			bad.Target.Owner = bad.JournalOwner
		case "offset-minus64":
			bad.Target.Offset = 8128
		case "offset-plus64":
			bad.Target.Offset = 8256
		case "offset-zero":
			bad.Target.Offset = 0
		case "length-prefix":
			bad.Target.Length = 64
		case "length-short":
			bad.Target.Length = 4095
		case "dirty-two":
			bad.DirtyPages = 2
		case "dirty-four":
			bad.DirtyPages = 4
		case "original-two":
			bad.OriginalPages = 2
		case "original-four":
			bad.OriginalPages = 4
		case "page-size":
			bad.PageSize = 8192
		case "sector-size":
			bad.SectorSize = 512
		case "prefix":
			bad.Prefix = 64
		case "prefix-error":
			bad.PrefixError = true
		case "call-count":
			bad.CallCount = 16
		case "success":
			bad.ExpectedCommit = CommitSucceeded
		case "uncertain":
			bad.ExpectedCommit = CommitUncertain
		case "original-read-inventory":
			bad.Calls[4].Offset = 8192
		case "dirty-write-inventory":
			bad.Calls[13].Offset = 8192
		}
		if _, problem := NewPlan(bad); problem == NoProblem {
			t.Fatal("misplacement selector accepted", name)
		}
	}
	for value := 0; value <= 255; value++ {
		bad := spec
		bad.Target.Point = Point(value)
		if bad.Target.Point != DBPage {
			if _, problem := NewPlan(bad); problem == NoProblem {
				t.Fatal("misplacement point accepted", value)
			}
		}
		bad = spec
		bad.Target.Role = Role(value)
		if bad.Target.Role != Database {
			if _, problem := NewPlan(bad); problem == NoProblem {
				t.Fatal("misplacement role accepted", value)
			}
		}
		bad = spec
		bad.Target.Occurrence = uint8(value)
		if value != 3 {
			if _, problem := NewPlan(bad); problem == NoProblem {
				t.Fatal("misplacement occurrence accepted", value)
			}
		}
		bad = spec
		bad.Action = Action(value)
		if value > int(MisplaceIndexPageMinus64) {
			if _, problem := NewPlan(bad); problem == NoProblem {
				t.Fatal("future action accepted", value)
			}
		}
	}
	for i := 0; i < 12; i++ {
		if _, problem := NewPlan(onePageSpec(MisplaceIndexPageMinus64, i)); problem == NoProblem {
			t.Fatal("one-page action accepted", i)
		}
	}
}

func TestRollbackFaultMisplacementNativeResultsKeepOriginalError(t *testing.T) {
	plan, problem := NewPlan(misplacedIndexSpec())
	if problem != NoProblem {
		t.Fatal(problem)
	}
	spec := plan.Spec()
	for _, name := range []string{"success-exposed", "prefix", "short-nil", "negative-native", "oversized-native", "unexecuted", "wrong-effect", "short-write", "normalized-nil", "returned-token", "requested-offset", "masked-native-error", "masked-native-token", "success-commit", "uncertain-commit", "missing-target", "logical-open", "pending-owner", "unknown-owner"} {
		events := fixtureEvents(spec)
		result := &events[30].Result
		switch name {
		case "success-exposed":
			result.ReturnedError, result.NormalizedError = None, None
		case "prefix":
			result.NativeN, result.ReturnedN = 64, 64
		case "short-nil":
			result.NativeN, result.ReturnedN, result.ReturnedError, result.NormalizedError = 4095, 4095, None, ShortWrite
		case "negative-native":
			result.NativeN = -1
		case "oversized-native":
			result.NativeN = 4097
		case "unexecuted":
			result.Executed = false
		case "wrong-effect":
			result.Effect = EffectPrefix
		case "short-write":
			result.ReturnedError, result.NormalizedError = ShortWrite, ShortWrite
		case "normalized-nil":
			result.NormalizedError = None
		case "returned-token":
			result.ReturnedErrorToken = 71
		case "requested-offset":
			events[30].Call.Offset = 8128
		case "masked-native-error":
			result.NativeError, result.NativeErrorToken, result.Effect = Underlying, 71, EffectUnknown
		case "masked-native-token":
			result.NativeError, result.NativeErrorToken, result.Effect = Underlying, 71, EffectUnknown
			result.ReturnedError, result.ReturnedErrorToken = Underlying, 72
			result.NormalizedError, result.NormalizedErrorToken = Underlying, 72
		case "success-commit":
			events[31].Commit.Class = CommitSucceeded
		case "uncertain-commit":
			events[31].Commit.Class = CommitUncertain
		case "missing-target":
			events[31].Commit.TargetError = None
		case "logical-open":
			events[31].Commit.Logical = LogicalOpen
		case "pending-owner":
			events[32].Owners.Cleanup = OwnerPending
		case "unknown-owner":
			events[32].Owners.Journal = OwnerUnknown
		}
		mustNotQualify(t, observe(t, spec, events))
	}
	for _, count := range []int{0, 64, 4095, 4096} {
		events := fixtureEvents(spec)
		events[30].Result = Result{Executed: true, NativeN: count, NativeError: Underlying, NativeErrorToken: 71, ReturnedN: count, ReturnedError: Underlying, ReturnedErrorToken: 71, NormalizedError: Underlying, NormalizedErrorToken: 71, Effect: EffectUnknown}
		events[31].Commit.TargetError, events[31].Commit.TargetErrorToken = Underlying, 71
		observation := observe(t, spec, events)
		if observation.Outcome != ObservationRefused || observation.Problem != ProblemUnderlyingFailure {
			t.Fatal("original native error must remain refused evidence", count, observation)
		}
	}
}
