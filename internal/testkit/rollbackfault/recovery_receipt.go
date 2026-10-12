package rollbackfault

import "crypto/sha256"

// This file models bounded caller-supplied evidence, not physical execution.
// Hash agreement is input consistency. It neither authenticates a build receipt
// nor proves that a caller measured a file, performed recovery, or retired an
// owner. Independent source/build review and authorized physical evidence remain
// separate requirements. There are no descriptors, error interfaces, callbacks,
// environment reads, clocks, goroutines, or I/O in this model.
const (
	MaxRecoveryOwners      = 96
	MaxRecoverySourceFiles = 4096
	RecoveryStageCount     = 10
)

type RecoveryCase uint8

const (
	RecoveryCaseUnknown RecoveryCase = iota
	RecoveryControl
	RecoveryPagePrefix64
	RecoveryDBSyncBefore
)

// RecoverySpec is the frozen one-case expectation. The first four hash fields
// also bind the corresponding lowercase hexadecimal Plan.Identity labels.
// CompileReceiptHash identifies separately retained source-to-binary evidence;
// a self-reported binary digest alone is deliberately insufficient.
type RecoverySpec struct {
	Case                                                               RecoveryCase
	Plan                                                               PlanSpec
	SourceHash, ToolHash, BinaryHash, ManifestHash, CompileReceiptHash [32]byte
	OldPageHash, NewPageHash, PartialPageHash                          [32]byte
}

type RecoveryProvenance struct {
	SourceHash, ToolHash, BinaryHash, ManifestHash, CompileReceiptHash         [32]byte
	CompileSourceHash, CompileToolHash, CompileBinaryHash, CompileManifestHash [32]byte
	SourceFiles, ManifestFiles, CompileFiles                                   uint16
}

type RecoveryFileRole uint8

const (
	RecoveryFileUnknown RecoveryFileRole = iota
	RecoveryBaseline
	RecoveryExpectedNew
	RecoveryTrialDatabase
	RecoveryTrialJournal
	RecoveryRetainedDatabase
	RecoveryRetainedJournal
	RecoveryReplayDatabase
	RecoveryReplayJournal
)

// Missing snapshots must be entirely zero. Before is the retained/pre-recovery
// checkpoint, including the post-Commit trial and fresh replay copy. After is
// the final recheck. Device/Inode are caller-observed identities, not handles.
type RecoveryFileSnapshot struct {
	Present       bool
	Device, Inode uint64
	Size          int64
	Hash          [32]byte
}

type RecoveryFileEvidence struct {
	Role          RecoveryFileRole
	Before, After RecoveryFileSnapshot
}

// Registry evidence is deliberately aggregate. The production recovery API
// does not expose an independent descriptor-owner token, so none is invented.
type RecoveryRegistrySnapshot struct {
	PlainGroups, Pending uint16
}

type RecoveryCallEvidence struct {
	Calls                             uint8
	Error                             ErrorKind
	ErrorToken                        uint64
	Recovered                         bool
	RestoredPages, OriginalPages      uint32
	MaxJournalBytes, MaxDatabaseBytes uint64
	MaxRecords                        uint32
	RegistryChecks                    uint8
	RegistryBefore, RegistryAfter     RecoveryRegistrySnapshot
}

type RecoveryPageHeader struct {
	PageSize, DatabasePages, ChangeCounter, SchemaCookie, VersionValidFor uint32
	Encoding, ReadVersion, WriteVersion                                   uint8
}

type RecoveryReadEvidence struct {
	Owner                                               uint64
	ReadCalls, CloseCalls                               uint8
	Error, CloseError                                   ErrorKind
	ErrorToken, CloseErrorToken                         uint64
	PageNumber, BytesRead, ComparedBytes, MatchingBytes uint32
	ExtentBefore, ExtentAfter                           int64
	PageHash                                            [32]byte
	Header                                              RecoveryPageHeader
}

type RecoveryOwnerKind uint8

const (
	RecoveryOwnerUnknown RecoveryOwnerKind = iota
	RecoveryFileOwner
	RecoveryReaderOwner
)

// These bounded scalar IDs identify the caller's preparation/readback ledger.
// They do not pretend to be recovery's undisclosed descriptor-owner IDs.
type RecoveryOwnerEvidence struct {
	ID                    uint64
	Kind                  RecoveryOwnerKind
	OpenCalls, CloseCalls uint8
	CloseError            ErrorKind
	CloseErrorToken       uint64
	State                 OwnerState
}

type RecoveryStage uint8

const (
	RecoveryStageUnknown RecoveryStage = iota
	RecoveryAdmitted
	RecoveryCommitReturned
	RecoveryAdapterSealed
	RecoveryAdapterDisarmed
	RecoveryInputsRetained
	RecoveryReplayCopied
	RecoveryChecked
	RecoveryReaderClosed
	RecoveryInputsRechecked
	RecoveryTerminal
)

// The nested adapter transcript is complete: its plan and every event are
// replayed through the original contract, rather than trusting an outcome flag.
// CommitReturn and its diagnostic hash refer to the original Commit return.
// Native caller source must separately bind that return directly to finish.
type RecoveryEvidence struct {
	Identity                                                       Identity
	Case                                                           RecoveryCase
	SelectedCases, ArmCalls, CommitCalls, FinishCalls, DisarmCalls uint8
	DisarmError                                                    ErrorKind
	DisarmErrorToken                                               uint64
	CommitReturn                                                   CommitResult
	CommitDiagnosticHash                                           [32]byte
	UnexpectedErrorCount                                           uint8
	AdapterPlan                                                    PlanSpec
	AdapterEvents                                                  [MaxEvents]Event
	AdapterEventCount                                              uint8
	AdapterHash                                                    [32]byte
	AdapterObservation                                             Observation
	Provenance                                                     RecoveryProvenance
	Files                                                          [8]RecoveryFileEvidence
	Recovery                                                       RecoveryCallEvidence
	Reader                                                         RecoveryReadEvidence
	Owners                                                         [MaxRecoveryOwners]RecoveryOwnerEvidence
	OwnerCount                                                     uint8
	Stages                                                         [RecoveryStageCount]RecoveryStage
	StageCount                                                     uint8
}

type RecoveryProblem uint8

const (
	RecoveryProblemNone RecoveryProblem = iota
	RecoveryProblemCase
	RecoveryProblemSpec
	RecoveryProblemIdentity
	RecoveryProblemProvenance
	RecoveryProblemCalls
	RecoveryProblemAdapter
	RecoveryProblemAdapterHash
	RecoveryProblemCommit
	RecoveryProblemFiles
	RecoveryProblemAliases
	RecoveryProblemRecovery
	RecoveryProblemReadback
	RecoveryProblemOwners
	RecoveryProblemTerminal
)

type RecoveryOutcome uint8

const (
	RecoveryInvalid RecoveryOutcome = iota
	// ScenarioObserved means only that supplied finite evidence is consistent.
	// It is never an acceptance, durability, physical, or all-three-cases PASS.
	ScenarioObserved
)

type RecoveryObservation struct {
	Outcome RecoveryOutcome
	Problem RecoveryProblem
	Case    RecoveryCase
	Adapter Observation
}

func (RecoveryObservation) PhysicalValidated() bool { return false }
func (RecoveryObservation) Pass() bool              { return false }

func ValidateRecoveryEvidence(spec RecoverySpec, evidence RecoveryEvidence) RecoveryObservation {
	observation := RecoveryObservation{Case: spec.Case}
	if problem := validRecoverySpec(spec); problem != RecoveryProblemNone {
		observation.Problem = problem
		return observation
	}
	if evidence.Case != spec.Case || evidence.SelectedCases != 1 {
		observation.Problem = RecoveryProblemCase
		return observation
	}
	if !validIdentity(evidence.Identity) || evidence.Identity != spec.Plan.Identity {
		observation.Problem = RecoveryProblemIdentity
		return observation
	}
	if !validRecoveryProvenance(spec, evidence.Provenance) {
		observation.Problem = RecoveryProblemProvenance
		return observation
	}
	if evidence.ArmCalls != 1 || evidence.CommitCalls != 1 || evidence.FinishCalls != 1 || evidence.DisarmCalls != 1 || evidence.DisarmError != None || evidence.DisarmErrorToken != 0 || evidence.UnexpectedErrorCount != 0 {
		observation.Problem = RecoveryProblemCalls
		return observation
	}
	if evidence.AdapterPlan != spec.Plan || evidence.AdapterEventCount == 0 || int(evidence.AdapterEventCount) > MaxEvents {
		observation.Problem = RecoveryProblemAdapter
		return observation
	}
	plan, planProblem := NewPlan(evidence.AdapterPlan)
	if planProblem != NoProblem {
		observation.Problem = RecoveryProblemAdapter
		return observation
	}
	receipt, receiptProblem := NewReceipt(plan)
	if receiptProblem != NoProblem {
		observation.Problem = RecoveryProblemAdapter
		return observation
	}
	for i := 0; i < int(evidence.AdapterEventCount); i++ {
		if receipt.Append(evidence.AdapterEvents[i]) != NoProblem {
			observation.Problem = RecoveryProblemAdapter
			return observation
		}
	}
	for i := int(evidence.AdapterEventCount); i < MaxEvents; i++ {
		if evidence.AdapterEvents[i] != (Event{}) {
			observation.Problem = RecoveryProblemAdapter
			return observation
		}
	}
	observation.Adapter = receipt.Seal()
	if observation.Adapter.Outcome != AdapterObserved || observation.Adapter.Problem != NoProblem || observation.Adapter.TargetHits != 1 || observation.Adapter != evidence.AdapterObservation {
		observation.Problem = RecoveryProblemAdapter
		return observation
	}
	digest, hashProblem := RecoveryAdapterHash(evidence.AdapterPlan, evidence.AdapterEvents, evidence.AdapterEventCount)
	if hashProblem != RecoveryProblemNone || digest != evidence.AdapterHash || digest == ([32]byte{}) {
		observation.Problem = RecoveryProblemAdapterHash
		return observation
	}
	commitIndex := 1 + 2*int(spec.Plan.CallCount)
	if commitIndex >= int(evidence.AdapterEventCount) || evidence.AdapterEvents[commitIndex].Kind != CommitReturned || evidence.CommitReturn != evidence.AdapterEvents[commitIndex].Commit || evidence.CommitReturn.Class != spec.Plan.ExpectedCommit || evidence.CommitReturn.Logical != LogicalClosed {
		observation.Problem = RecoveryProblemCommit
		return observation
	}
	if spec.Case == RecoveryControl {
		if evidence.CommitReturn.Error != None || evidence.CommitReturn.ErrorToken != 0 || evidence.CommitReturn.TargetError != None || evidence.CommitReturn.TargetErrorToken != 0 || evidence.CommitDiagnosticHash != ([32]byte{}) {
			observation.Problem = RecoveryProblemCommit
			return observation
		}
	} else {
		cause := Injected
		if spec.Case == RecoveryPagePrefix64 {
			cause = ShortWrite
		}
		if evidence.CommitReturn.Error != Underlying || evidence.CommitReturn.ErrorToken == 0 || evidence.CommitReturn.TargetError != cause || evidence.CommitReturn.TargetErrorToken != 0 || evidence.CommitDiagnosticHash == ([32]byte{}) {
			observation.Problem = RecoveryProblemCommit
			return observation
		}
	}
	if problem := validRecoveryFiles(spec, evidence.Files); problem != RecoveryProblemNone {
		observation.Problem = problem
		return observation
	}
	if !validRecoveryCall(spec.Case, evidence.Recovery) {
		observation.Problem = RecoveryProblemRecovery
		return observation
	}
	if !validRecoveryRead(spec, evidence.Reader) {
		observation.Problem = RecoveryProblemReadback
		return observation
	}
	if !validRecoveryOwners(evidence) {
		observation.Problem = RecoveryProblemOwners
		return observation
	}
	if evidence.StageCount != RecoveryStageCount {
		observation.Problem = RecoveryProblemTerminal
		return observation
	}
	for i, stage := range evidence.Stages {
		if stage != RecoveryStage(i+1) {
			observation.Problem = RecoveryProblemTerminal
			return observation
		}
	}
	observation.Outcome = ScenarioObserved
	return observation
}

func validRecoverySpec(spec RecoverySpec) RecoveryProblem {
	if spec.Case < RecoveryControl || spec.Case > RecoveryDBSyncBefore {
		return RecoveryProblemCase
	}
	if _, problem := NewPlan(spec.Plan); problem != NoProblem {
		return RecoveryProblemSpec
	}
	if spec.Plan.OriginalPages != 1 || spec.Plan.DirtyPages != 1 || spec.Plan.Target.Occurrence != 1 || spec.Plan.Target.Offset != 0 || spec.Plan.Target.Role != Database || spec.Plan.Target.Owner != spec.Plan.MainOwner {
		return RecoveryProblemSpec
	}
	point, action, length, prefix := DBPage, Pass, 4096, 0
	switch spec.Case {
	case RecoveryPagePrefix64:
		action, prefix = PrefixShortWrite, 64
	case RecoveryDBSyncBefore:
		point, action, length = DBSync, FailBefore, 0
	}
	if spec.Plan.Target.Point != point || spec.Plan.Action != action || spec.Plan.Target.Length != length || spec.Plan.Prefix != prefix || spec.Plan.PrefixError {
		return RecoveryProblemSpec
	}
	hashes := [8][32]byte{spec.SourceHash, spec.ToolHash, spec.BinaryHash, spec.ManifestHash, spec.CompileReceiptHash, spec.OldPageHash, spec.NewPageHash, spec.PartialPageHash}
	for _, hash := range hashes {
		if hash == ([32]byte{}) {
			return RecoveryProblemSpec
		}
	}
	if spec.OldPageHash == spec.NewPageHash || spec.OldPageHash == spec.PartialPageHash || spec.NewPageHash == spec.PartialPageHash {
		return RecoveryProblemSpec
	}
	if spec.Plan.Identity.Source != recoveryHashLabel(spec.SourceHash) || spec.Plan.Identity.Tool != recoveryHashLabel(spec.ToolHash) || spec.Plan.Identity.Binary != recoveryHashLabel(spec.BinaryHash) || spec.Plan.Identity.Manifest != recoveryHashLabel(spec.ManifestHash) {
		return RecoveryProblemIdentity
	}
	return RecoveryProblemNone
}

func validRecoveryProvenance(spec RecoverySpec, p RecoveryProvenance) bool {
	return p.SourceHash == spec.SourceHash && p.ToolHash == spec.ToolHash && p.BinaryHash == spec.BinaryHash && p.ManifestHash == spec.ManifestHash && p.CompileReceiptHash == spec.CompileReceiptHash && p.CompileSourceHash == spec.SourceHash && p.CompileToolHash == spec.ToolHash && p.CompileBinaryHash == spec.BinaryHash && p.CompileManifestHash == spec.ManifestHash && p.SourceFiles > 0 && p.SourceFiles <= MaxRecoverySourceFiles && p.SourceFiles == p.ManifestFiles && p.SourceFiles == p.CompileFiles
}

func validRecoveryFiles(spec RecoverySpec, files [8]RecoveryFileEvidence) RecoveryProblem {
	post, final := spec.NewPageHash, spec.OldPageHash
	if spec.Case == RecoveryPagePrefix64 {
		post = spec.PartialPageHash
	}
	if spec.Case == RecoveryControl {
		final = spec.NewPageHash
	}
	for i, file := range files {
		if file.Role != RecoveryFileRole(i+1) {
			return RecoveryProblemFiles
		}
		journal := file.Role == RecoveryTrialJournal || file.Role == RecoveryRetainedJournal || file.Role == RecoveryReplayJournal
		if journal && spec.Case == RecoveryControl {
			if file.Before != (RecoveryFileSnapshot{}) || file.After != (RecoveryFileSnapshot{}) {
				return RecoveryProblemFiles
			}
			continue
		}
		size := int64(4096)
		if journal {
			size = 8200
		}
		if !validRecoverySnapshot(file.Before, size) {
			return RecoveryProblemFiles
		}
		if file.Role == RecoveryReplayJournal {
			if file.After != (RecoveryFileSnapshot{}) {
				return RecoveryProblemFiles
			}
		} else if !validRecoverySnapshot(file.After, size) {
			return RecoveryProblemFiles
		}
		if file.Role != RecoveryReplayDatabase && file.Role != RecoveryReplayJournal && file.Before != file.After {
			return RecoveryProblemFiles
		}
		want := post
		switch file.Role {
		case RecoveryBaseline:
			want = spec.OldPageHash
		case RecoveryExpectedNew:
			want = spec.NewPageHash
		case RecoveryTrialJournal, RecoveryRetainedJournal, RecoveryReplayJournal:
			want = files[3].Before.Hash
		}
		if file.Before.Hash != want {
			return RecoveryProblemFiles
		}
		if file.Role == RecoveryReplayDatabase && (file.After.Hash != final || file.After.Device != file.Before.Device || file.After.Inode != file.Before.Inode) {
			return RecoveryProblemFiles
		}
		for j := 0; j < i; j++ {
			other := files[j].Before
			if other.Present && other.Device == file.Before.Device && other.Inode == file.Before.Inode {
				return RecoveryProblemAliases
			}
		}
	}
	return RecoveryProblemNone
}

func validRecoverySnapshot(snapshot RecoveryFileSnapshot, size int64) bool {
	return snapshot.Present && snapshot.Device != 0 && snapshot.Inode != 0 && snapshot.Size == size && snapshot.Hash != ([32]byte{})
}

func validRecoveryCall(selected RecoveryCase, evidence RecoveryCallEvidence) bool {
	if evidence.RegistryChecks != 2 || evidence.RegistryBefore != (RecoveryRegistrySnapshot{}) || evidence.RegistryAfter != (RecoveryRegistrySnapshot{}) {
		return false
	}
	evidence.RegistryChecks = 0
	if selected == RecoveryControl {
		return evidence == (RecoveryCallEvidence{})
	}
	return evidence.Calls == 1 && evidence.Error == None && evidence.ErrorToken == 0 && evidence.Recovered && evidence.RestoredPages == 1 && evidence.OriginalPages == 1 && evidence.MaxJournalBytes == 8200 && evidence.MaxDatabaseBytes == 4096 && evidence.MaxRecords == 1
}

func validRecoveryRead(spec RecoverySpec, evidence RecoveryReadEvidence) bool {
	hash, counter, cookie := spec.OldPageHash, uint32(7), uint32(9)
	if spec.Case == RecoveryControl {
		hash, counter, cookie = spec.NewPageHash, 8, 10
	}
	header := RecoveryPageHeader{PageSize: 4096, DatabasePages: 1, ChangeCounter: counter, SchemaCookie: cookie, VersionValidFor: counter, Encoding: 1, ReadVersion: 1, WriteVersion: 1}
	return evidence.Owner != 0 && evidence.ReadCalls == 1 && evidence.CloseCalls == 1 && evidence.Error == None && evidence.ErrorToken == 0 && evidence.CloseError == None && evidence.CloseErrorToken == 0 && evidence.PageNumber == 1 && evidence.BytesRead == 4096 && evidence.ComparedBytes == 4096 && evidence.MatchingBytes == 4096 && evidence.ExtentBefore == 4096 && evidence.ExtentAfter == 4096 && evidence.PageHash == hash && evidence.Header == header
}

func validRecoveryOwners(evidence RecoveryEvidence) bool {
	if evidence.OwnerCount < 2 || int(evidence.OwnerCount) > MaxRecoveryOwners {
		return false
	}
	readers := 0
	for i := 0; i < int(evidence.OwnerCount); i++ {
		owner := evidence.Owners[i]
		if owner.ID == 0 || owner.Kind < RecoveryFileOwner || owner.Kind > RecoveryReaderOwner || owner.OpenCalls != 1 || owner.CloseCalls != 1 || owner.CloseError != None || owner.CloseErrorToken != 0 || owner.State != OwnerRetired {
			return false
		}
		if owner.Kind == RecoveryReaderOwner {
			readers++
			if owner.ID != evidence.Reader.Owner {
				return false
			}
		}
		for j := 0; j < i; j++ {
			if evidence.Owners[j].ID == owner.ID {
				return false
			}
		}
	}
	for i := int(evidence.OwnerCount); i < MaxRecoveryOwners; i++ {
		if evidence.Owners[i] != (RecoveryOwnerEvidence{}) {
			return false
		}
	}
	return readers == 1
}

func recoveryHashLabel(hash [32]byte) string {
	const digits = "0123456789abcdef"
	var text [64]byte
	for i, value := range hash {
		text[2*i], text[2*i+1] = digits[value>>4], digits[value&15]
	}
	return string(text[:])
}

// RecoveryAdapterHash uses a domain-separated, length-delimited SHA-256 chain.
// It hashes the full plan and every field of each retained event, including
// normally-zero payloads. No formatting, native-width layout, or JSON encoding
// is part of this protocol. It checks bounded identities before hashing them;
// structural/semantic adapter validation is still done separately above.
func RecoveryAdapterHash(plan PlanSpec, events [MaxEvents]Event, count uint8) ([32]byte, RecoveryProblem) {
	if _, problem := NewPlan(plan); problem != NoProblem || count == 0 || int(count) > MaxEvents {
		return [32]byte{}, RecoveryProblemAdapter
	}
	for i := 0; i < int(count); i++ {
		if !validIdentity(events[i].Identity) {
			return [32]byte{}, RecoveryProblemIdentity
		}
	}
	for i := int(count); i < MaxEvents; i++ {
		if events[i] != (Event{}) {
			return [32]byte{}, RecoveryProblemAdapter
		}
	}
	var digest recoveryHashState
	digest.id("rollback-recovery-adapter-v1")
	digest.identity(plan.Identity)
	digest.call(plan.Target)
	digest.number(uint64(plan.Action))
	digest.number(uint64(plan.Prefix))
	digest.boolean(plan.PrefixError)
	digest.number(plan.MainOwner)
	digest.number(plan.JournalOwner)
	digest.number(plan.DirectoryOwner)
	digest.number(plan.CleanupOwner)
	digest.number(uint64(plan.OriginalPages))
	digest.number(uint64(plan.DirtyPages))
	digest.number(uint64(plan.PageSize))
	digest.number(uint64(plan.SectorSize))
	digest.number(uint64(plan.CallCount))
	digest.number(uint64(plan.ExpectedCommit))
	for _, call := range plan.Calls {
		digest.call(call)
	}
	digest.number(uint64(count))
	for i := 0; i < int(count); i++ {
		event := events[i]
		digest.identity(event.Identity)
		digest.number(uint64(event.Ordinal))
		digest.number(uint64(event.Kind))
		digest.call(event.Call)
		digest.number(uint64(event.Action))
		digest.boolean(event.Result.Executed)
		digest.number(uint64(event.Result.NativeN))
		digest.number(uint64(event.Result.NativeError))
		digest.number(event.Result.NativeErrorToken)
		digest.number(uint64(event.Result.ReturnedN))
		digest.number(uint64(event.Result.ReturnedError))
		digest.number(event.Result.ReturnedErrorToken)
		digest.number(uint64(event.Result.NormalizedError))
		digest.number(event.Result.NormalizedErrorToken)
		digest.number(uint64(event.Result.Effect))
		digest.number(uint64(event.Commit.Class))
		digest.number(uint64(event.Commit.Error))
		digest.number(event.Commit.ErrorToken)
		digest.number(uint64(event.Commit.TargetError))
		digest.number(event.Commit.TargetErrorToken)
		digest.number(uint64(event.Commit.Logical))
		digest.number(event.Owners.MainOwner)
		digest.number(event.Owners.JournalOwner)
		digest.number(event.Owners.CleanupOwner)
		digest.number(uint64(event.Owners.Main))
		digest.number(uint64(event.Owners.Journal))
		digest.number(uint64(event.Owners.Cleanup))
		digest.number(uint64(event.Validation))
		digest.number(uint64(event.Rejection))
		digest.number(uint64(event.RejectedAt))
	}
	return digest.sum, RecoveryProblemNone
}

type recoveryHashState struct{ sum [32]byte }

func (digest *recoveryHashState) number(value uint64) {
	var input [41]byte
	copy(input[:32], digest.sum[:])
	input[32] = 1
	for i := 0; i < 8; i++ {
		input[40-i] = byte(value)
		value >>= 8
	}
	digest.sum = sha256.Sum256(input[:])
}

func (digest *recoveryHashState) boolean(value bool) {
	if value {
		digest.number(1)
	} else {
		digest.number(0)
	}
}

func (digest *recoveryHashState) id(value string) {
	var input [35 + MaxIDLength]byte
	copy(input[:32], digest.sum[:])
	input[32], input[33], input[34] = 2, byte(len(value)>>8), byte(len(value))
	copy(input[35:], value)
	digest.sum = sha256.Sum256(input[:35+len(value)])
}

func (digest *recoveryHashState) identity(value Identity) {
	digest.id(value.Suite)
	digest.id(value.Source)
	digest.id(value.Tool)
	digest.id(value.Binary)
	digest.id(value.Manifest)
	digest.id(value.Trial)
	digest.id(value.Scope)
	digest.number(uint64(value.Profile))
}

func (digest *recoveryHashState) call(value Call) {
	digest.number(uint64(value.Point))
	digest.number(uint64(value.Occurrence))
	digest.number(uint64(value.Role))
	digest.number(uint64(value.Offset))
	digest.number(uint64(value.Length))
	digest.number(value.Owner)
}
