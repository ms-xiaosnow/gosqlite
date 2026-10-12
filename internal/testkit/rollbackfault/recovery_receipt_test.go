package rollbackfault

import "testing"

// SOURCE ONLY / NOT RUN. These are synthetic descriptor-free consistency
// vectors. Their invented hashes, inode numbers and scalar owner IDs are not
// measured files, build provenance, a database fixture, or physical evidence.
// None of these tests constructs a database or invokes Commit or recovery.
func recoveryFixture(t *testing.T, selected RecoveryCase) (RecoverySpec, RecoveryEvidence) {
	t.Helper()
	action, target := Pass, 8
	if selected == RecoveryPagePrefix64 {
		action = PrefixShortWrite
	}
	if selected == RecoveryDBSyncBefore {
		action, target = FailBefore, 9
	}
	spec := RecoverySpec{
		Case: selected, Plan: onePageSpec(action, target),
		SourceHash: [32]byte{1}, ToolHash: [32]byte{2}, BinaryHash: [32]byte{3},
		ManifestHash: [32]byte{4}, CompileReceiptHash: [32]byte{5},
		OldPageHash: [32]byte{6}, NewPageHash: [32]byte{7}, PartialPageHash: [32]byte{8},
	}
	if selected == RecoveryPagePrefix64 {
		spec.Plan.Prefix = 64
	}
	spec.Plan.Identity.Source = recoveryHashLabel(spec.SourceHash)
	spec.Plan.Identity.Tool = recoveryHashLabel(spec.ToolHash)
	spec.Plan.Identity.Binary = recoveryHashLabel(spec.BinaryHash)
	spec.Plan.Identity.Manifest = recoveryHashLabel(spec.ManifestHash)
	events := fixtureEvents(spec.Plan)
	evidence := RecoveryEvidence{
		Identity: spec.Plan.Identity, Case: selected, SelectedCases: 1,
		ArmCalls: 1, CommitCalls: 1, FinishCalls: 1, DisarmCalls: 1,
		AdapterPlan: spec.Plan, AdapterEventCount: uint8(len(events)),
		AdapterObservation: observe(t, spec.Plan, events),
		Provenance: RecoveryProvenance{
			SourceHash: spec.SourceHash, ToolHash: spec.ToolHash, BinaryHash: spec.BinaryHash,
			ManifestHash: spec.ManifestHash, CompileReceiptHash: spec.CompileReceiptHash,
			CompileSourceHash: spec.SourceHash, CompileToolHash: spec.ToolHash,
			CompileBinaryHash: spec.BinaryHash, CompileManifestHash: spec.ManifestHash,
			SourceFiles: 17, ManifestFiles: 17, CompileFiles: 17,
		},
		Recovery: RecoveryCallEvidence{RegistryChecks: 2},
		Reader: RecoveryReadEvidence{
			Owner: 102, ReadCalls: 1, CloseCalls: 1, PageNumber: 1,
			BytesRead: 4096, ComparedBytes: 4096, MatchingBytes: 4096,
			ExtentBefore: 4096, ExtentAfter: 4096, PageHash: spec.OldPageHash,
			Header: RecoveryPageHeader{PageSize: 4096, DatabasePages: 1, ChangeCounter: 7, SchemaCookie: 9, VersionValidFor: 7, Encoding: 1, ReadVersion: 1, WriteVersion: 1},
		},
		Owners: [MaxRecoveryOwners]RecoveryOwnerEvidence{
			{ID: 101, Kind: RecoveryFileOwner, OpenCalls: 1, CloseCalls: 1, State: OwnerRetired},
			{ID: 102, Kind: RecoveryReaderOwner, OpenCalls: 1, CloseCalls: 1, State: OwnerRetired},
		},
		OwnerCount: 2, StageCount: RecoveryStageCount,
	}
	copy(evidence.AdapterEvents[:], events)
	evidence.CommitReturn = events[1+2*int(spec.Plan.CallCount)].Commit
	if selected != RecoveryControl {
		evidence.CommitDiagnosticHash = [32]byte{9}
		evidence.Recovery = RecoveryCallEvidence{Calls: 1, Recovered: true, RestoredPages: 1, OriginalPages: 1, MaxJournalBytes: 8200, MaxDatabaseBytes: 4096, MaxRecords: 1, RegistryChecks: 2}
	} else {
		evidence.Reader.PageHash = spec.NewPageHash
		evidence.Reader.Header.ChangeCounter = 8
		evidence.Reader.Header.SchemaCookie = 10
		evidence.Reader.Header.VersionValidFor = 8
	}
	post := spec.NewPageHash
	if selected == RecoveryPagePrefix64 {
		post = spec.PartialPageHash
	}
	for i := range evidence.Files {
		role := RecoveryFileRole(i + 1)
		file := RecoveryFileEvidence{Role: role}
		journal := role == RecoveryTrialJournal || role == RecoveryRetainedJournal || role == RecoveryReplayJournal
		if !journal || selected != RecoveryControl {
			file.Before = RecoveryFileSnapshot{Present: true, Device: 1, Inode: uint64(201 + i), Size: 4096, Hash: post}
			if journal {
				file.Before.Size, file.Before.Hash = 8200, [32]byte{10}
			}
			if role == RecoveryBaseline {
				file.Before.Hash = spec.OldPageHash
			}
			if role == RecoveryExpectedNew {
				file.Before.Hash = spec.NewPageHash
			}
			file.After = file.Before
			if role == RecoveryReplayDatabase {
				file.After.Hash = evidence.Reader.PageHash
			}
			if role == RecoveryReplayJournal {
				file.After = RecoveryFileSnapshot{}
			}
		}
		evidence.Files[i] = file
	}
	for i := range evidence.Stages {
		evidence.Stages[i] = RecoveryStage(i + 1)
	}
	recoveryRehash(t, &evidence)
	return spec, evidence
}

func recoveryRehash(t *testing.T, evidence *RecoveryEvidence) {
	t.Helper()
	digest, problem := RecoveryAdapterHash(evidence.AdapterPlan, evidence.AdapterEvents, evidence.AdapterEventCount)
	if problem != RecoveryProblemNone {
		t.Fatalf("bounded synthetic transcript cannot be hashed: %v", problem)
	}
	evidence.AdapterHash = digest
}

func recoveryMustReject(t *testing.T, spec RecoverySpec, evidence RecoveryEvidence, problem RecoveryProblem) {
	t.Helper()
	observation := ValidateRecoveryEvidence(spec, evidence)
	if observation.Outcome == ScenarioObserved || observation.Pass() || observation.PhysicalValidated() || observation.Problem == RecoveryProblemNone {
		t.Fatalf("false scenario/physical qualification: %+v", observation)
	}
	if problem != RecoveryProblemNone && observation.Problem != problem {
		t.Fatalf("wrong rejection: got %v want %v", observation.Problem, problem)
	}
}

func TestRollbackRecoverySyntheticConsistencyNeverPhysicalPass(t *testing.T) {
	for _, selected := range [3]RecoveryCase{RecoveryControl, RecoveryPagePrefix64, RecoveryDBSyncBefore} {
		spec, evidence := recoveryFixture(t, selected)
		observation := ValidateRecoveryEvidence(spec, evidence)
		if observation.Outcome != ScenarioObserved || observation.Problem != RecoveryProblemNone || observation.Adapter.Outcome != AdapterObserved || observation.Case != selected {
			t.Fatalf("synthetic case %v rejected: %+v", selected, observation)
		}
		if observation.Pass() || observation.PhysicalValidated() || observation.Adapter.Pass() || observation.Adapter.PhysicalValidated() {
			t.Fatal("synthetic input consistency acquired physical authority")
		}
	}
	// Even manually constructed exported outcome fields cannot change methods.
	if (RecoveryObservation{Outcome: ScenarioObserved}).Pass() || (RecoveryObservation{Outcome: ScenarioObserved}).PhysicalValidated() {
		t.Fatal("exported outcome bypassed permanently absent physical status")
	}
	recoveryMustReject(t, RecoverySpec{}, RecoveryEvidence{}, RecoveryProblemCase)
	// A source inventory may itself be the manifest. Equal provenance digests
	// are legal; old/new/partial page images must nevertheless be distinct.
	spec, evidence := recoveryFixture(t, RecoveryControl)
	spec.ManifestHash = spec.SourceHash
	spec.Plan.Identity.Manifest = spec.Plan.Identity.Source
	evidence.Identity = spec.Plan.Identity
	evidence.AdapterPlan = spec.Plan
	evidence.Provenance.ManifestHash = spec.ManifestHash
	evidence.Provenance.CompileManifestHash = spec.ManifestHash
	for i := 0; i < int(evidence.AdapterEventCount); i++ {
		evidence.AdapterEvents[i].Identity = spec.Plan.Identity
	}
	recoveryRehash(t, &evidence)
	if observation := ValidateRecoveryEvidence(spec, evidence); observation.Outcome != ScenarioObserved || observation.Pass() || observation.PhysicalValidated() {
		t.Fatalf("shared inventory digest changed consistency-only semantics: %+v", observation)
	}
}

func TestRollbackRecoveryRejectsCasePlanAndProvenanceFalsePass(t *testing.T) {
	for _, name := range [...]string{"unknown-case", "foreign-case", "empty-selection", "multiple-selection", "mismatched-case", "target-occurrence", "target-offset", "target-length", "target-point", "prefix-63", "prefix-error", "dirty-two", "original-two", "source-zero", "tool-zero", "binary-zero", "manifest-zero", "compile-zero", "old-zero", "new-zero", "partial-zero", "old-new-equal", "old-partial-equal", "new-partial-equal", "identity-label", "stale-trial", "stale-scope", "unbounded-id", "stale-source", "stale-tool", "stale-binary", "stale-manifest", "missing-compile", "unbound-compile-source", "unbound-compile-tool", "unbound-compile-binary", "unbound-compile-manifest", "zero-source-count", "too-many-sources", "manifest-count", "compile-count"} {
		spec, evidence := recoveryFixture(t, RecoveryPagePrefix64)
		problem := RecoveryProblemSpec
		switch name {
		case "unknown-case":
			spec.Case, problem = RecoveryCaseUnknown, RecoveryProblemCase
		case "foreign-case":
			spec.Case, problem = RecoveryCase(255), RecoveryProblemCase
		case "empty-selection":
			evidence.SelectedCases, problem = 0, RecoveryProblemCase
		case "multiple-selection":
			evidence.SelectedCases, problem = 2, RecoveryProblemCase
		case "mismatched-case":
			evidence.Case, problem = RecoveryControl, RecoveryProblemCase
		case "target-occurrence":
			spec.Plan.Target.Occurrence = 2
		case "target-offset":
			spec.Plan.Target.Offset = 4096
		case "target-length":
			spec.Plan.Target.Length = 64
		case "target-point":
			spec.Plan.Target.Point = JournalUnlink
		case "prefix-63":
			spec.Plan.Prefix = 63
		case "prefix-error":
			spec.Plan.PrefixError = true
		case "dirty-two":
			spec.Plan.DirtyPages = 2
		case "original-two":
			spec.Plan.OriginalPages = 2
		case "source-zero":
			spec.SourceHash = [32]byte{}
		case "tool-zero":
			spec.ToolHash = [32]byte{}
		case "binary-zero":
			spec.BinaryHash = [32]byte{}
		case "manifest-zero":
			spec.ManifestHash = [32]byte{}
		case "compile-zero":
			spec.CompileReceiptHash = [32]byte{}
		case "old-zero":
			spec.OldPageHash = [32]byte{}
		case "new-zero":
			spec.NewPageHash = [32]byte{}
		case "partial-zero":
			spec.PartialPageHash = [32]byte{}
		case "old-new-equal":
			spec.NewPageHash = spec.OldPageHash
		case "old-partial-equal":
			spec.PartialPageHash = spec.OldPageHash
		case "new-partial-equal":
			spec.PartialPageHash = spec.NewPageHash
		case "identity-label":
			spec.Plan.Identity.Source, problem = "unverified-source-name", RecoveryProblemIdentity
		case "stale-trial":
			evidence.Identity.Trial, problem = "old-trial", RecoveryProblemIdentity
		case "stale-scope":
			evidence.Identity.Scope, problem = "old-scope", RecoveryProblemIdentity
		case "unbounded-id":
			var oversized [MaxIDLength + 1]byte
			for i := range oversized {
				oversized[i] = 'a'
			}
			evidence.Identity.Scope, problem = string(oversized[:]), RecoveryProblemIdentity
		case "stale-source":
			evidence.Provenance.SourceHash, problem = [32]byte{99}, RecoveryProblemProvenance
		case "stale-tool":
			evidence.Provenance.ToolHash, problem = [32]byte{99}, RecoveryProblemProvenance
		case "stale-binary":
			evidence.Provenance.BinaryHash, problem = [32]byte{99}, RecoveryProblemProvenance
		case "stale-manifest":
			evidence.Provenance.ManifestHash, problem = [32]byte{99}, RecoveryProblemProvenance
		case "missing-compile":
			evidence.Provenance.CompileReceiptHash, problem = [32]byte{}, RecoveryProblemProvenance
		case "unbound-compile-source":
			evidence.Provenance.CompileSourceHash, problem = [32]byte{99}, RecoveryProblemProvenance
		case "unbound-compile-tool":
			evidence.Provenance.CompileToolHash, problem = [32]byte{99}, RecoveryProblemProvenance
		case "unbound-compile-binary":
			evidence.Provenance.CompileBinaryHash, problem = [32]byte{99}, RecoveryProblemProvenance
		case "unbound-compile-manifest":
			evidence.Provenance.CompileManifestHash, problem = [32]byte{99}, RecoveryProblemProvenance
		case "zero-source-count":
			evidence.Provenance.SourceFiles, problem = 0, RecoveryProblemProvenance
		case "too-many-sources":
			evidence.Provenance.SourceFiles, problem = MaxRecoverySourceFiles+1, RecoveryProblemProvenance
		case "manifest-count":
			evidence.Provenance.ManifestFiles, problem = 16, RecoveryProblemProvenance
		case "compile-count":
			evidence.Provenance.CompileFiles, problem = 16, RecoveryProblemProvenance
		}
		t.Run(name, func(t *testing.T) { recoveryMustReject(t, spec, evidence, problem) })
	}
}

func TestRollbackRecoveryRejectsCommitAndAdapterFalsePass(t *testing.T) {
	for _, name := range [...]string{"arm-zero", "arm-two", "commit-zero", "commit-two", "finish-zero", "finish-two", "disarm-zero", "disarm-two", "disarm-error", "disarm-token", "native-extra-error", "replaced-return", "masked-short-write", "masked-injected", "missing-diagnostic", "stale-plan", "adapter-unknown", "adapter-refused", "adapter-incomplete", "adapter-invalid", "adapter-zero-hits", "adapter-two-hits", "adapter-wrong-count", "adapter-wrong-hash", "adapter-zero-hash", "adapter-hidden-tail", "adapter-overflow", "adapter-zero-count"} {
		selected := RecoveryPagePrefix64
		if name == "masked-injected" {
			selected = RecoveryDBSyncBefore
		}
		spec, evidence := recoveryFixture(t, selected)
		problem := RecoveryProblemAdapter
		switch name {
		case "arm-zero":
			evidence.ArmCalls, problem = 0, RecoveryProblemCalls
		case "arm-two":
			evidence.ArmCalls, problem = 2, RecoveryProblemCalls
		case "commit-zero":
			evidence.CommitCalls, problem = 0, RecoveryProblemCalls
		case "commit-two":
			evidence.CommitCalls, problem = 2, RecoveryProblemCalls
		case "finish-zero":
			evidence.FinishCalls, problem = 0, RecoveryProblemCalls
		case "finish-two":
			evidence.FinishCalls, problem = 2, RecoveryProblemCalls
		case "disarm-zero":
			evidence.DisarmCalls, problem = 0, RecoveryProblemCalls
		case "disarm-two":
			evidence.DisarmCalls, problem = 2, RecoveryProblemCalls
		case "disarm-error":
			evidence.DisarmError, problem = Underlying, RecoveryProblemCalls
		case "disarm-token":
			evidence.DisarmErrorToken, problem = 81, RecoveryProblemCalls
		case "native-extra-error":
			evidence.UnexpectedErrorCount, problem = 1, RecoveryProblemCalls
		case "replaced-return":
			evidence.CommitReturn.ErrorToken, problem = 901, RecoveryProblemCommit
		case "masked-short-write", "masked-injected":
			evidence.CommitReturn.TargetError, problem = None, RecoveryProblemCommit
		case "missing-diagnostic":
			evidence.CommitDiagnosticHash, problem = [32]byte{}, RecoveryProblemCommit
		case "stale-plan":
			evidence.AdapterPlan.Identity.Trial = "old-trial"
		case "adapter-unknown":
			evidence.AdapterObservation = Observation{}
		case "adapter-refused":
			evidence.AdapterObservation.Outcome = ObservationRefused
		case "adapter-incomplete":
			evidence.AdapterObservation.Outcome = ObservationIncomplete
		case "adapter-invalid":
			evidence.AdapterObservation.Outcome = ObservationInvalid
		case "adapter-zero-hits":
			evidence.AdapterObservation.TargetHits = 0
		case "adapter-two-hits":
			evidence.AdapterObservation.TargetHits = 2
		case "adapter-wrong-count":
			evidence.AdapterObservation.EventCount--
		case "adapter-wrong-hash":
			evidence.AdapterHash, problem = [32]byte{88}, RecoveryProblemAdapterHash
		case "adapter-zero-hash":
			evidence.AdapterHash, problem = [32]byte{}, RecoveryProblemAdapterHash
		case "adapter-hidden-tail":
			evidence.AdapterEvents[evidence.AdapterEventCount] = evidence.AdapterEvents[0]
		case "adapter-overflow":
			evidence.AdapterEventCount = MaxEvents + 1
		case "adapter-zero-count":
			evidence.AdapterEventCount = 0
		}
		t.Run(name, func(t *testing.T) { recoveryMustReject(t, spec, evidence, problem) })
	}
	for _, class := range [...]CommitClass{CommitUnknown, CommitFailed, CommitUncertain, CommitCleanupPending, CommitUnsupported, CommitRefused, CommitSkipped} {
		spec, evidence := recoveryFixture(t, RecoveryPagePrefix64)
		index := 1 + 2*int(spec.Plan.CallCount)
		evidence.AdapterEvents[index].Commit.Class = class
		evidence.CommitReturn.Class = class
		recoveryRehash(t, &evidence)
		recoveryMustReject(t, spec, evidence, RecoveryProblemAdapter)
	}
}

func TestRollbackRecoveryNestedTranscriptEveryTruncationOmissionDuplicateAndOrder(t *testing.T) {
	spec, original := recoveryFixture(t, RecoveryPagePrefix64)
	count := int(original.AdapterEventCount)
	for end := 0; end < count; end++ {
		evidence := original
		evidence.AdapterEventCount = uint8(end)
		for i := end; i < MaxEvents; i++ {
			evidence.AdapterEvents[i] = Event{}
		}
		if end > 0 {
			recoveryRehash(t, &evidence)
		}
		recoveryMustReject(t, spec, evidence, RecoveryProblemAdapter)
	}
	for index := 0; index < count; index++ {
		for mode := 0; mode < 3; mode++ {
			if mode == 2 && index+1 == count {
				continue
			}
			evidence := original
			switch mode {
			case 0:
				for i := index; i < count-1; i++ {
					evidence.AdapterEvents[i] = original.AdapterEvents[i+1]
				}
				evidence.AdapterEvents[count-1], evidence.AdapterEventCount = Event{}, uint8(count-1)
			case 1:
				for i := count; i > index; i-- {
					evidence.AdapterEvents[i] = original.AdapterEvents[i-1]
				}
				evidence.AdapterEventCount++
			case 2:
				evidence.AdapterEvents[index], evidence.AdapterEvents[index+1] = evidence.AdapterEvents[index+1], evidence.AdapterEvents[index]
			}
			for i := 0; i < int(evidence.AdapterEventCount); i++ {
				evidence.AdapterEvents[i].Ordinal = uint8(i + 1)
			}
			// Recompute hashes and ordinals: rejection must be semantic, never
			// just a stale checksum or an unchanged ordinal mismatch.
			recoveryRehash(t, &evidence)
			recoveryMustReject(t, spec, evidence, RecoveryProblemAdapter)
		}
	}
	for _, name := range [...]string{"stale-source", "stale-binary", "stale-trial", "target-duplicate", "short-native-63", "short-native-error", "masked-original-cause", "owner-unknown", "owner-live", "owner-pending", "validation-claim"} {
		evidence := original
		target := 2 + 2*8
		commit := 1 + 2*int(spec.Plan.CallCount)
		switch name {
		case "stale-source":
			evidence.AdapterEvents[1].Identity.Source = "old-source"
		case "stale-binary":
			evidence.AdapterEvents[1].Identity.Binary = "old-binary"
		case "stale-trial":
			evidence.AdapterEvents[1].Identity.Trial = "old-trial"
		case "target-duplicate":
			evidence.AdapterEvents[1].Call = spec.Plan.Target
		case "short-native-63":
			evidence.AdapterEvents[target].Result.NativeN = 63
		case "short-native-error":
			evidence.AdapterEvents[target].Result.NativeError = Underlying
		case "masked-original-cause":
			evidence.AdapterEvents[commit].Commit.TargetError = None
		case "owner-unknown":
			evidence.AdapterEvents[commit+1].Owners.Main = OwnerUnknown
		case "owner-live":
			evidence.AdapterEvents[commit+1].Owners.Main = OwnerLive
		case "owner-pending":
			evidence.AdapterEvents[commit+1].Owners.Main = OwnerPending
		case "validation-claim":
			evidence.AdapterEvents[commit+2].Validation = Validation(1)
		}
		recoveryRehash(t, &evidence)
		t.Run(name, func(t *testing.T) { recoveryMustReject(t, spec, evidence, RecoveryProblemAdapter) })
	}
}

func TestRollbackRecoveryRejectsRetainedInputAndReplayFalsePass(t *testing.T) {
	for _, name := range [...]string{"wrong-role", "source-inode-copy", "retained-inode-copy", "journal-inode-copy", "missing-baseline", "zero-inode", "zero-device", "wrong-db-size", "wrong-journal-size", "zero-hash", "baseline-mutated", "expected-mutated", "trial-mutated", "trial-journal-consumed", "retained-db-mutated", "retained-journal-consumed", "retained-journal-mismatch", "replay-source-mismatch", "replay-journal-mismatch", "replay-replaced-inode", "replay-journal-remains", "replay-not-restored", "replay-wrong-extent", "control-journal-present", "control-absence-with-payload", "control-restored-old"} {
		selected := RecoveryPagePrefix64
		if name == "control-journal-present" || name == "control-absence-with-payload" || name == "control-restored-old" {
			selected = RecoveryControl
		}
		spec, evidence := recoveryFixture(t, selected)
		problem := RecoveryProblemFiles
		switch name {
		case "wrong-role":
			evidence.Files[0].Role = RecoveryTrialDatabase
		case "source-inode-copy":
			evidence.Files[6].Before.Inode, evidence.Files[6].After.Inode, problem = evidence.Files[2].Before.Inode, evidence.Files[2].Before.Inode, RecoveryProblemAliases
		case "retained-inode-copy":
			evidence.Files[4].Before.Inode, evidence.Files[4].After.Inode, problem = evidence.Files[2].Before.Inode, evidence.Files[2].Before.Inode, RecoveryProblemAliases
		case "journal-inode-copy":
			evidence.Files[7].Before.Inode, problem = evidence.Files[3].Before.Inode, RecoveryProblemAliases
		case "missing-baseline":
			evidence.Files[0].Before = RecoveryFileSnapshot{}
		case "zero-inode":
			evidence.Files[0].Before.Inode = 0
		case "zero-device":
			evidence.Files[0].Before.Device = 0
		case "wrong-db-size":
			evidence.Files[2].Before.Size = 8192
		case "wrong-journal-size":
			evidence.Files[3].Before.Size = 8199
		case "zero-hash":
			evidence.Files[3].Before.Hash = [32]byte{}
		case "baseline-mutated":
			evidence.Files[0].After.Hash = spec.NewPageHash
		case "expected-mutated":
			evidence.Files[1].After.Hash = spec.OldPageHash
		case "trial-mutated":
			evidence.Files[2].After.Hash = spec.OldPageHash
		case "trial-journal-consumed":
			evidence.Files[3].After = RecoveryFileSnapshot{}
		case "retained-db-mutated":
			evidence.Files[4].After.Hash = spec.OldPageHash
		case "retained-journal-consumed":
			evidence.Files[5].Before, evidence.Files[5].After = RecoveryFileSnapshot{}, RecoveryFileSnapshot{}
		case "retained-journal-mismatch":
			evidence.Files[5].Before.Hash, evidence.Files[5].After.Hash = [32]byte{33}, [32]byte{33}
		case "replay-source-mismatch":
			evidence.Files[6].Before.Hash = spec.OldPageHash
		case "replay-journal-mismatch":
			evidence.Files[7].Before.Hash = [32]byte{33}
		case "replay-replaced-inode":
			evidence.Files[6].After.Inode = 999
		case "replay-journal-remains":
			evidence.Files[7].After = evidence.Files[7].Before
		case "replay-not-restored":
			evidence.Files[6].After.Hash = spec.PartialPageHash
		case "replay-wrong-extent":
			evidence.Files[6].After.Size = 8192
		case "control-journal-present":
			evidence.Files[3].Before, evidence.Files[3].After = evidence.Files[2].Before, evidence.Files[2].After
		case "control-absence-with-payload":
			evidence.Files[3].Before.Hash = [32]byte{33}
		case "control-restored-old":
			evidence.Files[6].After.Hash = spec.OldPageHash
		}
		t.Run(name, func(t *testing.T) { recoveryMustReject(t, spec, evidence, problem) })
	}
}

func TestRollbackRecoveryRejectsRecoveryReadbackOwnerAndTerminalFalsePass(t *testing.T) {
	for _, name := range [...]string{"no-recovery-but-old-page", "recovery-retried", "not-recovered", "wrong-restored-count", "wrong-original-count", "recovery-error", "recovery-token", "journal-limit", "database-limit", "record-limit", "registry-unchecked", "registry-once", "registry-before-nonempty", "registry-after-nonempty", "pending-before", "pending-after", "control-fake-recovery", "control-fake-result", "reader-zero", "reader-two", "header-only-comparison", "nonheader-byte-wrong", "wrong-page-hash", "read-before-extent", "read-after-extent", "header-cookie", "header-counter", "header-version", "header-encoding", "header-pages", "header-page-size", "header-read-version", "header-write-version", "reader-page-number", "reader-short", "reader-error", "reader-close-error", "reader-double-close", "reader-unclosed", "owner-missing", "owner-overflow", "owner-duplicate", "owner-unknown", "owner-live", "owner-pending", "owner-unacquired", "file-close-failed", "file-double-close", "file-unclosed", "file-double-open", "file-unknown-kind", "owner-hidden-tail", "reader-owner-mismatch", "two-reader-owners", "no-reader-owner", "terminal-truncated", "terminal-overflow", "terminal-duplicate", "terminal-reordered", "terminal-unknown"} {
		selected := RecoveryPagePrefix64
		if name == "control-fake-recovery" || name == "control-fake-result" {
			selected = RecoveryControl
		}
		spec, evidence := recoveryFixture(t, selected)
		problem := RecoveryProblemRecovery
		switch name {
		case "no-recovery-but-old-page":
			evidence.Recovery.Calls = 0
		case "recovery-retried":
			evidence.Recovery.Calls = 2
		case "not-recovered":
			evidence.Recovery.Recovered = false
		case "wrong-restored-count":
			evidence.Recovery.RestoredPages = 0
		case "wrong-original-count":
			evidence.Recovery.OriginalPages = 2
		case "recovery-error":
			evidence.Recovery.Error = Underlying
		case "recovery-token":
			evidence.Recovery.ErrorToken = 77
		case "journal-limit":
			evidence.Recovery.MaxJournalBytes++
		case "database-limit":
			evidence.Recovery.MaxDatabaseBytes++
		case "record-limit":
			evidence.Recovery.MaxRecords++
		case "registry-unchecked":
			evidence.Recovery.RegistryChecks = 0
		case "registry-once":
			evidence.Recovery.RegistryChecks = 1
		case "registry-before-nonempty":
			evidence.Recovery.RegistryBefore.PlainGroups = 1
		case "registry-after-nonempty":
			evidence.Recovery.RegistryAfter.PlainGroups = 1
		case "pending-before":
			evidence.Recovery.RegistryBefore.Pending = 1
		case "pending-after":
			evidence.Recovery.RegistryAfter.Pending = 1
		case "control-fake-recovery":
			evidence.Recovery.Calls = 1
		case "control-fake-result":
			evidence.Recovery.Recovered = true
		case "reader-zero":
			evidence.Reader.ReadCalls, problem = 0, RecoveryProblemReadback
		case "reader-two":
			evidence.Reader.ReadCalls, problem = 2, RecoveryProblemReadback
		case "header-only-comparison":
			evidence.Reader.ComparedBytes, problem = 100, RecoveryProblemReadback
		case "nonheader-byte-wrong":
			evidence.Reader.MatchingBytes, problem = 4095, RecoveryProblemReadback
		case "wrong-page-hash":
			evidence.Reader.PageHash, problem = [32]byte{33}, RecoveryProblemReadback
		case "read-before-extent":
			evidence.Reader.ExtentBefore, problem = 8192, RecoveryProblemReadback
		case "read-after-extent":
			evidence.Reader.ExtentAfter, problem = 8192, RecoveryProblemReadback
		case "header-cookie":
			evidence.Reader.Header.SchemaCookie, problem = 10, RecoveryProblemReadback
		case "header-counter":
			evidence.Reader.Header.ChangeCounter, problem = 8, RecoveryProblemReadback
		case "header-version":
			evidence.Reader.Header.VersionValidFor, problem = 8, RecoveryProblemReadback
		case "header-encoding":
			evidence.Reader.Header.Encoding, problem = 2, RecoveryProblemReadback
		case "header-pages":
			evidence.Reader.Header.DatabasePages, problem = 2, RecoveryProblemReadback
		case "header-page-size":
			evidence.Reader.Header.PageSize, problem = 512, RecoveryProblemReadback
		case "header-read-version":
			evidence.Reader.Header.ReadVersion, problem = 2, RecoveryProblemReadback
		case "header-write-version":
			evidence.Reader.Header.WriteVersion, problem = 2, RecoveryProblemReadback
		case "reader-page-number":
			evidence.Reader.PageNumber, problem = 2, RecoveryProblemReadback
		case "reader-short":
			evidence.Reader.BytesRead, problem = 4095, RecoveryProblemReadback
		case "reader-error":
			evidence.Reader.Error, problem = Underlying, RecoveryProblemReadback
		case "reader-close-error":
			evidence.Reader.CloseError, problem = Underlying, RecoveryProblemReadback
		case "reader-double-close":
			evidence.Reader.CloseCalls, problem = 2, RecoveryProblemReadback
		case "reader-unclosed":
			evidence.Reader.CloseCalls, problem = 0, RecoveryProblemReadback
		case "owner-missing":
			evidence.OwnerCount, problem = 0, RecoveryProblemOwners
		case "owner-overflow":
			evidence.OwnerCount, problem = MaxRecoveryOwners+1, RecoveryProblemOwners
		case "owner-duplicate":
			evidence.Owners[0].ID, problem = evidence.Owners[1].ID, RecoveryProblemOwners
		case "owner-unknown":
			evidence.Owners[0].State, problem = OwnerUnknown, RecoveryProblemOwners
		case "owner-live":
			evidence.Owners[0].State, problem = OwnerLive, RecoveryProblemOwners
		case "owner-pending":
			evidence.Owners[0].State, problem = OwnerPending, RecoveryProblemOwners
		case "owner-unacquired":
			evidence.Owners[0].State, problem = OwnerNotAcquired, RecoveryProblemOwners
		case "file-close-failed":
			evidence.Owners[0].CloseError, problem = Underlying, RecoveryProblemOwners
		case "file-double-close":
			evidence.Owners[0].CloseCalls, problem = 2, RecoveryProblemOwners
		case "file-unclosed":
			evidence.Owners[0].CloseCalls, problem = 0, RecoveryProblemOwners
		case "file-double-open":
			evidence.Owners[0].OpenCalls, problem = 2, RecoveryProblemOwners
		case "file-unknown-kind":
			evidence.Owners[0].Kind, problem = RecoveryOwnerUnknown, RecoveryProblemOwners
		case "owner-hidden-tail":
			evidence.Owners[2], problem = evidence.Owners[0], RecoveryProblemOwners
		case "reader-owner-mismatch":
			evidence.Owners[1].ID, problem = 999, RecoveryProblemOwners
		case "two-reader-owners":
			evidence.Owners[0].Kind, problem = RecoveryReaderOwner, RecoveryProblemOwners
		case "no-reader-owner":
			evidence.Owners[1].Kind, problem = RecoveryFileOwner, RecoveryProblemOwners
		case "terminal-truncated":
			evidence.StageCount, problem = RecoveryStageCount-1, RecoveryProblemTerminal
		case "terminal-overflow":
			evidence.StageCount, problem = RecoveryStageCount+1, RecoveryProblemTerminal
		case "terminal-duplicate":
			evidence.Stages[8], problem = RecoveryReaderClosed, RecoveryProblemTerminal
		case "terminal-reordered":
			evidence.Stages[1], evidence.Stages[2], problem = evidence.Stages[2], evidence.Stages[1], RecoveryProblemTerminal
		case "terminal-unknown":
			evidence.Stages[9], problem = RecoveryStageUnknown, RecoveryProblemTerminal
		}
		t.Run(name, func(t *testing.T) { recoveryMustReject(t, spec, evidence, problem) })
	}
	spec, complete := recoveryFixture(t, RecoveryDBSyncBefore)
	for end := 0; end < RecoveryStageCount; end++ {
		evidence := complete
		evidence.StageCount = uint8(end)
		for i := end; i < RecoveryStageCount; i++ {
			evidence.Stages[i] = RecoveryStageUnknown
		}
		recoveryMustReject(t, spec, evidence, RecoveryProblemTerminal)
	}
	for index := 0; index < RecoveryStageCount; index++ {
		evidence := complete
		evidence.Stages[index] = RecoveryStageUnknown
		recoveryMustReject(t, spec, evidence, RecoveryProblemTerminal)
		if index+1 < RecoveryStageCount {
			evidence = complete
			evidence.Stages[index], evidence.Stages[index+1] = evidence.Stages[index+1], evidence.Stages[index]
			recoveryMustReject(t, spec, evidence, RecoveryProblemTerminal)
		}
	}
}

func TestRollbackRecoveryHashBindsFullNestedPlanAndEventPayload(t *testing.T) {
	_, original := recoveryFixture(t, RecoveryPagePrefix64)
	for _, name := range [...]string{"plan-owner", "event-identity", "event-ordinal", "event-kind", "event-call", "event-action", "event-native", "event-commit", "event-owners", "event-validation", "event-rejection", "event-rejected-at"} {
		evidence := original
		switch name {
		case "plan-owner":
			evidence.AdapterPlan.CleanupOwner++
		case "event-identity":
			evidence.AdapterEvents[0].Identity.Scope = "another-scope"
		case "event-ordinal":
			evidence.AdapterEvents[0].Ordinal++
		case "event-kind":
			evidence.AdapterEvents[0].Kind = Terminal
		case "event-call":
			evidence.AdapterEvents[0].Call.Owner++
		case "event-action":
			evidence.AdapterEvents[0].Action = FailBefore
		case "event-native":
			evidence.AdapterEvents[0].Result.NativeN++
		case "event-commit":
			evidence.AdapterEvents[0].Commit.TargetError = Injected
		case "event-owners":
			evidence.AdapterEvents[0].Owners.Main = OwnerRetired
		case "event-validation":
			evidence.AdapterEvents[0].Validation = Validation(1)
		case "event-rejection":
			evidence.AdapterEvents[0].Rejection = ProblemIdentity
		case "event-rejected-at":
			evidence.AdapterEvents[0].RejectedAt++
		}
		digest, problem := RecoveryAdapterHash(evidence.AdapterPlan, evidence.AdapterEvents, evidence.AdapterEventCount)
		if problem != RecoveryProblemNone || digest == original.AdapterHash {
			t.Fatalf("hash did not bind %s: problem=%v", name, problem)
		}
	}
	var invalid [MaxEvents]Event
	if _, problem := RecoveryAdapterHash(original.AdapterPlan, invalid, 1); problem != RecoveryProblemIdentity {
		t.Fatalf("unbounded/unadmitted identity accepted: %v", problem)
	}
	if _, problem := RecoveryAdapterHash(original.AdapterPlan, original.AdapterEvents, 0); problem == RecoveryProblemNone {
		t.Fatal("zero transcript hash accepted")
	}
	if _, problem := RecoveryAdapterHash(original.AdapterPlan, original.AdapterEvents, MaxEvents+1); problem == RecoveryProblemNone {
		t.Fatal("oversized transcript hash accepted")
	}
	invalid = original.AdapterEvents
	invalid[original.AdapterEventCount] = invalid[0]
	if _, problem := RecoveryAdapterHash(original.AdapterPlan, invalid, original.AdapterEventCount); problem == RecoveryProblemNone {
		t.Fatal("hidden transcript tail ignored")
	}
	var oversized [MaxIDLength + 1]byte
	for i := range oversized {
		oversized[i] = 'x'
	}
	invalid = original.AdapterEvents
	invalid[0].Identity.Source = string(oversized[:])
	if _, problem := RecoveryAdapterHash(original.AdapterPlan, invalid, original.AdapterEventCount); problem != RecoveryProblemIdentity {
		t.Fatalf("unbounded identity reached the hash encoder: %v", problem)
	}
}
