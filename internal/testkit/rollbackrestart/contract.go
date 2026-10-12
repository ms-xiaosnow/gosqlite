// Package rollbackrestart is a bounded, descriptor-free value contract for five
// process-exit scenarios. Consistent caller-supplied values are not physical
// evidence. This package never opens, closes, syncs, spawns, waits, or recovers.
package rollbackrestart

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"

	"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"
)

const (
	SchemaVersion            uint16 = 1
	Suite                           = "rollback-restart-v1"
	MaxFrames                       = 64
	MaxFrameBytes                   = 4096
	MaxAdapterEvents                = 128
	MaxSnapshotBytes                = 32768
	MaxArtifactManifestBytes        = 16384
	MaxScenarioFrames               = 54
	MaxIDLength                     = 128
	ExpectedExitCode                = 86
	AbortExitCode                   = 87
	CallCount                       = 18
)

type Case uint8

const (
	CaseUnknown Case = iota
	Control
	BeforeJCreate
	AfterTablePage
	AfterJUnlink
	AcknowledgedExit
)

func Cases() [5]Case {
	return [5]Case{Control, BeforeJCreate, AfterTablePage, AfterJUnlink, AcknowledgedExit}
}
func CaseName(c Case) string {
	switch c {
	case Control:
		return "control"
	case BeforeJCreate:
		return "before-jcreate"
	case AfterTablePage:
		return "after-table-page"
	case AfterJUnlink:
		return "after-junlink"
	case AcknowledgedExit:
		return "acknowledged-exit"
	default:
		return ""
	}
}
func ParseCase(s string) (Case, bool) {
	for _, c := range Cases() {
		if CaseName(c) == s {
			return c, true
		}
	}
	return CaseUnknown, false
}

// Digest is exactly 64 lowercase hexadecimal bytes when present. It is a label
// unless a named pure digest helper derives it from a bounded value below.
type Digest string
type FileIdentity struct{ Device, Inode uint64 }
type Role uint8

const (
	RoleUnknown Role = iota
	Controller
	Writer
	Recovery
)

type RoleIdentity struct {
	Role       Role
	Generation uint64
	Challenge  Digest
}
type NativeIdentity struct {
	OS         string
	Filesystem string
	SectorSize uint32
	PageSize   uint32
}
type Binding struct {
	Suite    string
	Profile  rollbackfault.Profile
	Case     Case
	Trial    string
	Source   string
	Manifest Digest
	Tool     Digest
	Compile  Digest
	Binary   Digest
	Native   NativeIdentity
	Root     FileIdentity
	Baseline Digest
}
type PlanSpec struct {
	Binding         Binding
	Controller      RoleIdentity
	Writer          RoleIdentity
	Recovery        RoleIdentity
	AdapterIdentity rollbackfault.Identity
	Ticket          uint64
	Image           FileIdentity
}
type Plan struct {
	spec  PlanSpec
	valid bool
}

func (p Plan) Spec() PlanSpec { return p.spec }

// FrameKind is closed. Parent-only observations cannot be replaced by an
// identically worded child report or by output on stdout/stderr.
type FrameKind uint8

const (
	FrameUnknown FrameKind = iota
	ParentAdmitted
	BaselineCheckedAndClosed
	WriterSpawned
	ChildReadyBound
	ArmGranted
	TransactionArmed
	AdapterObserved
	TargetReached
	PublicCommitReturned
	AcknowledgementDurable
	ExitGranted
	NormalFinishGranted
	WriterFinished
	WriterWaitObserved
	OriginalsFrozen
	RecoverySpawned
	RecoveryReadyBound
	RecoveryReadbackGranted
	ReadbackResult
	RecoveryWaitObserved
	ArtifactsRechecked
	ScenarioEvidenceComplete
	Failure
)

type ProcessState uint8

const (
	ProcessUnknown ProcessState = iota
	ProcessRunning
	ProcessExited
	ProcessSignaled
)

// Handle is a parent's process-object ordinal, never an externally supplied PID
// capability. ActualImage attests a /proc/self/exe observation, not argv[0].
type ProcessEvidence struct {
	PID           int64
	Handle        uint64
	Image         FileIdentity
	ImageHash     Digest
	ActualImage   bool
	SpawnObserved bool
	WaitObserved  bool
	State         ProcessState
	ExitCode      int
	Signal        int
	TimedOut      bool
}
type CommitValue uint8

const (
	CommitUnknown CommitValue = iota
	CommitNil
	CommitError
	CommitUncertain
)

type CommitEvidence struct {
	Calls             uint8
	Value             CommitValue
	CapturedAdjacent  bool
	CapturedUnchanged bool
	PublicDone        bool
	PublicDetached    bool
	ActiveRows        uint32
	Main              rollbackfault.OwnerState
	Journal           rollbackfault.OwnerState
	Cleanup           rollbackfault.OwnerState
}

// Ack is a receipt for one immutable O_EXCL file in the separate control
// directory. Sync and close ordering is frozen by the numbered steps. Numbers
// are sequence labels, not proof that the native operations were performed.
type AckEvidence struct {
	CommitSequence      uint8
	CommitFrameSHA256   Digest
	SHA256              Digest
	File                FileIdentity
	Bytes               int64
	Written             int64
	Exclusive           bool
	WriteStep           uint8
	FileSyncStep        uint8
	FileCloseStep       uint8
	DirectorySyncStep   uint8
	DirectoryCloseStep  uint8
	FileCloseCalls      uint8
	DirectoryCloseCalls uint8
	ErrorToken          uint64
}
type Diagnostic uint8

const (
	DiagnosticNone Diagnostic = iota
	NotRun
	AdmissionBlocked
	ProtocolInvalid
	TargetNotReached
	UnderlyingIOFailed
	CommitUncertainDiagnostic
	AckFailed
	AckUncertain
	ChildExitUnexpected
	ChildLive
	WaitUnknown
	RecoveryFailed
	ReadbackFailed
	OwnerPending
	OwnerUnknown
	ArtifactChanged
	EvidenceComplete
)

// Each encoded Frame contains exactly this schema, with no extension fields or
// arbitrary payload. Sequence is global across the two one-way child pipes and
// the parent's private observations. At most 36 AdapterObserved frames occur.
type Frame struct {
	Schema     uint16
	Binding    Binding
	Sender     RoleIdentity
	Sequence   uint8
	Kind       FrameKind
	Ticket     uint64              `json:",omitzero"`
	Adapter    rollbackfault.Event `json:",omitzero"`
	Process    ProcessEvidence     `json:",omitzero"`
	Commit     CommitEvidence      `json:",omitzero"`
	Ack        AckEvidence         `json:",omitzero"`
	Digest     Digest              `json:",omitzero"`
	Diagnostic Diagnostic          `json:",omitzero"`
}
type Problem uint8

const (
	ProblemNone Problem = iota
	ProblemPlan
	ProblemIdentity
	ProblemSchema
	ProblemEncoding
	ProblemFrameLimit
	ProblemSequence
	ProblemOrder
	ProblemPayload
	ProblemAdapter
	ProblemTarget
	ProblemUnderlyingIO
	ProblemCommit
	ProblemAck
	ProblemProcess
	ProblemIncomplete
	ProblemSealed
	ProblemLifecycle
	ProblemArtifacts
	ProblemRecovery
	ProblemSQL
	ProblemPhysicalClaim
)
const NoProblem = ProblemNone

// Observation is deliberately not accepted as an input by Observe. Forging one
// cannot substitute for replaying the private Receipt's admitted frame values.
type Observation struct {
	EvidenceConsistent bool
	ScenarioObserved   bool
	Status             Diagnostic
	Problem            Problem
	FrameCount         uint8
	AdapterEventCount  uint8
	Commit             CommitValue
	Acknowledged       bool
	WriterWaited       bool
	RecoveryWaited     bool
	SQLState           rollbackfault.SQLState
}

func (Observation) PhysicalValidated() bool { return false }
func (Observation) Pass() bool              { return false }

type OwnerDisposition uint8

const (
	DispositionUnknown OwnerDisposition = iota
	RetiredNormally
	NotAcquired
	AbandonedByExpectedProcessExit
)

type WriterLifecycle struct {
	OpenCalls      uint8
	BeginCalls     uint8
	DMLCalls       uint8
	ArmCalls       uint8
	CommitCalls    uint8
	PublicReturned bool
	FinishCalls    uint8
	DisarmCalls    uint8
	RollbackCalls  uint8
	DBCloseCalls   uint8
	Main           OwnerDisposition
	Journal        OwnerDisposition
	Cleanup        OwnerDisposition
}
type RecoveryEvidence struct {
	Generation           uint64
	Calls                uint8
	Limits               rollbackfault.SQLRecoveryLimits
	Error                rollbackfault.ErrorKind
	ErrorToken           uint64
	JournalPresentBefore bool
	JournalPresentAfter  bool
	Replayed             bool
	ReplayedPages        uint32
	OriginalPages        uint32
	OwnersRetired        bool
	ReadOnly             bool
	NoCreate             bool
	OpenCalls            uint8
	CheckMaxPages        uint32
	CheckMaxPageReads    uint32
	NoRetry              bool
}
type ArtifactRole uint8

const (
	BaselineArtifact ArtifactRole = iota
	TrialArtifact
	TrialJournalArtifact
	PostExitArtifact
	PostExitJournalArtifact
	ReplayArtifact
	ReplayJournalArtifact
	AcknowledgementArtifact
	ArtifactCount
)

type ArtifactStamp struct {
	Path    string
	File    FileIdentity
	Exists  bool
	Regular bool
	Links   uint64
	Bytes   int64
	SHA256  Digest
	Pages   [3]Digest
}
type ArtifactEvidence struct {
	Role                ArtifactRole
	Before              ArtifactStamp
	After               ArtifactStamp
	ExclusiveCopy       bool
	CompleteRead        bool
	CompleteWrite       bool
	FileSynced          bool
	FileClosedOnce      bool
	DirectorySynced     bool
	DirectoryClosedOnce bool
}

// RuntimeEvidence is conjunctive. None of these independent requirements is
// inferred from a successful SQL result, exit code, or missing diagnostics.
type RuntimeEvidence struct {
	EnvFirst                      bool
	ExactRoleSelectors            bool
	FixedAllowlistEnvironment     bool
	CountOne                      bool
	ParallelOne                   bool
	CPUOne                        bool
	ShuffleOff                    bool
	TimeoutZero                   bool
	PrivateRoot0700               bool
	NoSymlinkAncestors            bool
	RootUnchanged                 bool
	TwoDedicatedPipes             bool
	PipeOwnersRecorded            bool
	UnusedPipeEndsClosedOnce      bool
	BoundedDiagnostics            bool
	NoInheritedDatabaseOwners     bool
	ParentOwnersClosedBeforeSpawn bool
	FreshRegistryEachChild        bool
	OnlyParentSpawns              bool
	NoSignalOrKill                bool
	NoRetries                     bool
	OriginalsNeverSyncedAfterExit bool
	ArtifactsRetained             bool
	AdapterSameTransaction        bool
	AdapterSameScope              bool
	AdapterSameOwners             bool
	ObserverOutsidePrivateMutex   bool
	OriginalTxLockHeldAtCut       bool
	NoCallbackNativeReplacement   bool
}
type ScenarioEvidence struct {
	Binding          Binding
	Before           rollbackfault.SQLSnapshotEvidence
	After            rollbackfault.SQLSnapshotEvidence
	BeforeGeneration uint64
	AfterGeneration  uint64
	Lifecycle        WriterLifecycle
	Recovery         RecoveryEvidence
	Artifacts        [ArtifactCount]ArtifactEvidence
	Runtime          RuntimeEvidence
	PhysicalClaim    bool
}

// Native transport writes all returned bytes once with checked short writes.
// It must read the 4-byte length first, reject it before allocating if outside
// 1..MaxFrameBytes, then read exactly that many bytes. This pure decoder takes
// the complete prefix+body value, rejects truncation and trailing bytes, and
// rejects every alternate JSON encoding (including duplicates and unknowns).
func EncodeFrame(f Frame) ([]byte, Problem) {
	if problem := ValidateFrame(f); problem != ProblemNone {
		return nil, problem
	}
	body, err := json.Marshal(f)
	if err != nil || len(body) == 0 || len(body) > MaxFrameBytes {
		return nil, ProblemEncoding
	}
	out := make([]byte, 4+len(body))
	binary.BigEndian.PutUint32(out[:4], uint32(len(body)))
	copy(out[4:], body)
	return out, ProblemNone
}
func DecodeFrame(raw []byte) (Frame, Problem) {
	if len(raw) < 5 || len(raw) > MaxFrameBytes+4 {
		return Frame{}, ProblemEncoding
	}
	n := binary.BigEndian.Uint32(raw[:4])
	if n == 0 || n > MaxFrameBytes || int(n) != len(raw)-4 {
		return Frame{}, ProblemEncoding
	}
	var f Frame
	decoder := json.NewDecoder(bytes.NewReader(raw[4:]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&f); err != nil {
		return Frame{}, ProblemEncoding
	}
	encoded, problem := EncodeFrame(f)
	if problem != ProblemNone || !bytes.Equal(encoded, raw) {
		return Frame{}, ProblemEncoding
	}
	return f, ProblemNone
}
func FrameDigest(f Frame) (Digest, Problem) {
	encoded, problem := EncodeFrame(f)
	if problem != ProblemNone {
		return "", problem
	}
	return digestBytes(encoded), ProblemNone
}
func digestBytes(raw []byte) Digest {
	sum := sha256.Sum256(raw)
	return Digest(hex.EncodeToString(sum[:]))
}

func validDigest(d Digest) bool {
	if len(d) != 64 {
		return false
	}
	nonzero := false
	for _, c := range d {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
		if c != '0' {
			nonzero = true
		}
	}
	return nonzero
}
func validID(s string) bool {
	if len(s) == 0 || len(s) > MaxIDLength || s == "0" {
		return false
	}
	for _, c := range s {
		if c < '!' || c > '~' {
			return false
		}
	}
	return true
}
func validFile(f FileIdentity) bool { return f.Device != 0 && f.Inode != 0 }
func validBinding(b Binding) bool {
	return b.Suite == Suite && b.Profile == rollbackfault.PlainRollback4096 && CaseName(b.Case) != "" && validID(b.Trial) && validID(b.Source) && validDigest(b.Manifest) && validDigest(b.Tool) && validDigest(b.Compile) && validDigest(b.Binary) && validDigest(b.Baseline) && b.Native.OS == "linux" && validID(b.Native.Filesystem) && len(b.Native.Filesystem) <= 32 && b.Native.PageSize == 4096 && b.Native.SectorSize == 4096 && validFile(b.Root)
}
func validRole(r RoleIdentity) bool {
	return r.Role >= Controller && r.Role <= Recovery && r.Generation != 0 && validDigest(r.Challenge)
}
func validAdapterIdentity(id rollbackfault.Identity) bool {
	return validID(id.Suite) && validID(id.Source) && validID(id.Tool) && validID(id.Binary) && validID(id.Manifest) && validID(id.Trial) && validID(id.Scope) && id.Profile == rollbackfault.PlainRollback4096
}
func copyText(s string) string {
	var buf [MaxIDLength]byte
	n := copy(buf[:], s)
	return string(buf[:n])
}
func copyDigest(d Digest) Digest { return Digest(copyText(string(d))) }
func copyBinding(b Binding) Binding {
	b.Suite, b.Trial, b.Source = copyText(b.Suite), copyText(b.Trial), copyText(b.Source)
	b.Manifest, b.Tool, b.Compile, b.Binary, b.Baseline = copyDigest(b.Manifest), copyDigest(b.Tool), copyDigest(b.Compile), copyDigest(b.Binary), copyDigest(b.Baseline)
	b.Native.OS, b.Native.Filesystem = copyText(b.Native.OS), copyText(b.Native.Filesystem)
	return b
}
func NewPlan(s PlanSpec) (Plan, Problem) {
	if !validBinding(s.Binding) || !validRole(s.Controller) || !validRole(s.Writer) || !validRole(s.Recovery) || s.Controller.Role != Controller || s.Writer.Role != Writer || s.Recovery.Role != Recovery || s.Ticket == 0 || !validFile(s.Image) {
		return Plan{}, ProblemPlan
	}
	roles := [3]RoleIdentity{s.Controller, s.Writer, s.Recovery}
	for i, role := range roles {
		for j := 0; j < i; j++ {
			if role.Generation == roles[j].Generation || role.Challenge == roles[j].Challenge {
				return Plan{}, ProblemIdentity
			}
		}
	}
	b, id := s.Binding, s.AdapterIdentity
	if !validAdapterIdentity(id) || id.Suite != b.Suite || id.Source != b.Source || id.Tool != string(b.Tool) || id.Binary != string(b.Binary) || id.Manifest != string(b.Manifest) || id.Trial != b.Trial || id.Profile != b.Profile {
		return Plan{}, ProblemIdentity
	}
	s.Binding = copyBinding(b)
	s.Controller.Challenge, s.Writer.Challenge, s.Recovery.Challenge = copyDigest(s.Controller.Challenge), copyDigest(s.Writer.Challenge), copyDigest(s.Recovery.Challenge)
	id.Suite, id.Source, id.Tool, id.Binary = copyText(id.Suite), copyText(id.Source), copyText(id.Tool), copyText(id.Binary)
	id.Manifest, id.Trial, id.Scope = copyText(id.Manifest), copyText(id.Trial), copyText(id.Scope)
	s.AdapterIdentity = id
	return Plan{spec: s, valid: true}, ProblemNone
}

// Target's call number is one-based in the original 18-call table. It is not
// the adapter ordinal, which also includes RunStarted and every Returned.
func Target(c Case) (uint8, rollbackfault.EventKind) {
	switch c {
	case BeforeJCreate:
		return 1, rollbackfault.Intent
	case AfterTablePage:
		return 14, rollbackfault.Returned
	case AfterJUnlink:
		return 17, rollbackfault.Returned
	default:
		return 0, rollbackfault.EventUnknown
	}
}
func ExpectedCalls() [CallCount]rollbackfault.Call {
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
}
func ExpectedAdapterEvents(c Case) uint8 {
	switch c {
	case BeforeJCreate:
		return 2
	case AfterTablePage:
		return 29
	case AfterJUnlink:
		return 35
	case Control, AcknowledgedExit:
		return 37
	default:
		return 0
	}
}
func ExpectedSQLState(c Case) rollbackfault.SQLState {
	switch c {
	case BeforeJCreate, AfterTablePage:
		return rollbackfault.SQLOld
	case Control, AfterJUnlink, AcknowledgedExit:
		return rollbackfault.SQLNew
	default:
		return rollbackfault.SQLStateUnknown
	}
}

func ValidateFrame(f Frame) Problem {
	if f.Schema != SchemaVersion {
		return ProblemSchema
	}
	if !validBinding(f.Binding) || !validRole(f.Sender) {
		return ProblemIdentity
	}
	if f.Sequence == 0 || int(f.Sequence) > MaxFrames {
		return ProblemSequence
	}
	if f.Kind <= FrameUnknown || f.Kind > Failure {
		return ProblemPayload
	}
	if f.Digest != "" && !validDigest(f.Digest) {
		return ProblemPayload
	}
	if f.Process.ImageHash != "" && !validDigest(f.Process.ImageHash) {
		return ProblemPayload
	}
	if f.Ack.SHA256 != "" && !validDigest(f.Ack.SHA256) {
		return ProblemPayload
	}
	if f.Ack.CommitFrameSHA256 != "" && !validDigest(f.Ack.CommitFrameSHA256) {
		return ProblemPayload
	}
	if f.Adapter != (rollbackfault.Event{}) && !validAdapterIdentity(f.Adapter.Identity) {
		return ProblemIdentity
	}
	if f.Process.State > ProcessSignaled || f.Process.Signal < 0 || f.Process.Signal > 64 || f.Process.ExitCode < 0 || f.Process.ExitCode > 255 || f.Commit.Value > CommitUncertain || f.Diagnostic > EvidenceComplete {
		return ProblemPayload
	}
	if f.Commit.Main > rollbackfault.OwnerNotAcquired || f.Commit.Journal > rollbackfault.OwnerNotAcquired || f.Commit.Cleanup > rollbackfault.OwnerNotAcquired {
		return ProblemPayload
	}
	if f.Adapter != (rollbackfault.Event{}) {
		a := f.Adapter
		if a.Kind != rollbackfault.RunStarted && a.Kind != rollbackfault.Intent && a.Kind != rollbackfault.Returned {
			return ProblemPayload
		}
		if a.Action != rollbackfault.Pass || a.Call.Point > rollbackfault.CleanupDirectorySync || a.Call.Role > rollbackfault.Directory || a.Result.NativeError > rollbackfault.Underlying || a.Result.ReturnedError > rollbackfault.Underlying || a.Result.NormalizedError > rollbackfault.Underlying || a.Result.Effect > rollbackfault.EffectPossible || a.Commit != (rollbackfault.CommitResult{}) || a.Owners != (rollbackfault.OwnersResult{}) || a.Validation != rollbackfault.ValidationAbsent || a.Rejection != rollbackfault.NoProblem || a.RejectedAt != 0 {
			return ProblemPayload
		}
	}
	kind := f.Kind
	f.Schema, f.Binding, f.Sender, f.Sequence, f.Kind = 0, Binding{}, RoleIdentity{}, 0, FrameUnknown
	switch kind {
	case ParentAdmitted, WriterSpawned, ChildReadyBound, WriterWaitObserved, RecoverySpawned, RecoveryReadyBound, RecoveryWaitObserved:
		f.Process = ProcessEvidence{}
	case BaselineCheckedAndClosed, OriginalsFrozen, ReadbackResult, ArtifactsRechecked, NormalFinishGranted:
		f.Digest = ""
	case TransactionArmed, AdapterObserved, TargetReached:
		f.Adapter, f.Ticket = rollbackfault.Event{}, 0
	case PublicCommitReturned:
		f.Commit = CommitEvidence{}
	case AcknowledgementDurable:
		f.Ack = AckEvidence{}
	case ExitGranted:
		f.Ack, f.Digest = AckEvidence{}, ""
	case Failure:
		if f.Diagnostic == DiagnosticNone || f.Diagnostic == EvidenceComplete {
			return ProblemPayload
		}
		f.Diagnostic = DiagnosticNone
	}
	if f != (Frame{}) {
		return ProblemPayload
	}
	return ProblemNone
}

type Receipt struct {
	plan            Plan
	frames          [MaxFrames]Frame
	count           uint8
	next            FrameKind
	problem         Problem
	status          Diagnostic
	adapterCount    uint8
	lastAdapter     rollbackfault.Event
	targetHash      Digest
	commitHash      Digest
	commitBodyHash  Digest
	commitBodyBytes int64
	commitSequence  uint8
	ack             AckEvidence
	parent          ProcessEvidence
	writer          ProcessEvidence
	recovery        ProcessEvidence
	baselineDigest  Digest
	frozenDigest    Digest
	readbackDigest  Digest
	checkedDigest   Digest
	writerWaited    bool
	recoveryWaited  bool
	sealed          bool
}

func NewReceipt(p Plan) (Receipt, Problem) {
	if !p.valid {
		return Receipt{}, ProblemPlan
	}
	return Receipt{plan: p, next: ParentAdmitted}, ProblemNone
}
func (r Receipt) Frames() ([MaxFrames]Frame, uint8) { return r.frames, r.count }
func (r *Receipt) Append(f Frame) Problem {
	if r.sealed {
		return ProblemSealed
	}
	if r.problem != ProblemNone {
		return r.problem
	}
	if !r.plan.valid {
		r.problem = ProblemPlan
		return r.problem
	}
	if int(r.count) >= MaxFrames {
		r.problem = ProblemFrameLimit
		return r.problem
	}
	if problem := ValidateFrame(f); problem != ProblemNone {
		r.problem, r.status = problem, ProtocolInvalid
		return problem
	}
	if f.Binding != r.plan.spec.Binding || f.Sender != r.sender(f.Kind) {
		r.problem, r.status = ProblemIdentity, ProtocolInvalid
		return r.problem
	}
	if f.Sequence != r.count+1 {
		r.problem, r.status = ProblemSequence, ProtocolInvalid
		return r.problem
	}
	// Freeze admitted strings instead of retaining an input substring's backing
	// storage. All remaining variable fields have already been bounded above.
	f.Binding, f.Sender = r.plan.spec.Binding, r.sender(f.Kind)
	if f.Adapter != (rollbackfault.Event{}) {
		if f.Adapter.Identity != r.plan.spec.AdapterIdentity {
			r.problem, r.status = ProblemIdentity, ProtocolInvalid
			return r.problem
		}
		f.Adapter.Identity = r.plan.spec.AdapterIdentity
	}
	f.Digest, f.Process.ImageHash = copyDigest(f.Digest), copyDigest(f.Process.ImageHash)
	f.Ack.SHA256, f.Ack.CommitFrameSHA256 = copyDigest(f.Ack.SHA256), copyDigest(f.Ack.CommitFrameSHA256)
	r.frames[r.count], r.count = f, r.count+1
	r.problem = r.accept(f)
	if r.problem != ProblemNone && r.status == DiagnosticNone {
		r.status = ProtocolInvalid
	}
	return r.problem
}
func (r Receipt) sender(kind FrameKind) RoleIdentity {
	switch kind {
	case ChildReadyBound, TransactionArmed, AdapterObserved, TargetReached, PublicCommitReturned, WriterFinished:
		return r.plan.spec.Writer
	case RecoveryReadyBound, ReadbackResult:
		return r.plan.spec.Recovery
	default:
		return r.plan.spec.Controller
	}
}
func (r *Receipt) accept(f Frame) Problem {
	if f.Kind == Failure {
		if r.next == FrameUnknown {
			return ProblemOrder
		}
		r.status, r.next = f.Diagnostic, FrameUnknown
		return ProblemIncomplete
	}
	if f.Kind != r.next || r.next == FrameUnknown {
		return ProblemOrder
	}
	spec, which := r.plan.spec, r.plan.spec.Binding.Case
	switch f.Kind {
	case ParentAdmitted:
		if !validSelf(f.Process, spec) || f.Process.Handle != 0 {
			return ProblemProcess
		}
		r.parent, r.next = f.Process, BaselineCheckedAndClosed
	case BaselineCheckedAndClosed:
		if !validDigest(f.Digest) {
			return ProblemPayload
		}
		r.baselineDigest, r.next = f.Digest, WriterSpawned
	case WriterSpawned:
		if !validSpawn(f.Process, spec) || f.Process.PID == r.parent.PID {
			return ProblemProcess
		}
		r.writer, r.next = f.Process, ChildReadyBound
	case ChildReadyBound:
		if !validReady(f.Process, r.writer, spec) {
			return ProblemProcess
		}
		r.next = ArmGranted
	case ArmGranted:
		r.next = TransactionArmed
	case TransactionArmed:
		want := rollbackfault.Event{Identity: spec.AdapterIdentity, Ordinal: 1, Kind: rollbackfault.RunStarted}
		if f.Ticket != spec.Ticket || f.Adapter != want {
			return ProblemAdapter
		}
		r.adapterCount, r.next = 1, AdapterObserved
	case AdapterObserved:
		if f.Ticket != spec.Ticket || r.adapterCount < 1 || r.adapterCount >= 37 {
			return ProblemAdapter
		}
		index := (r.adapterCount - 1) / 2
		kind := rollbackfault.Intent
		if r.adapterCount%2 == 0 {
			kind = rollbackfault.Returned
		}
		calls := ExpectedCalls()
		want := rollbackfault.Event{Identity: spec.AdapterIdentity, Ordinal: r.adapterCount + 1, Kind: kind, Call: calls[index]}
		if kind == rollbackfault.Returned {
			want.Result = successfulResult(want.Call)
			if f.Adapter.Result != want.Result {
				r.status = UnderlyingIOFailed
				return ProblemUnderlyingIO
			}
		}
		if f.Adapter != want {
			return ProblemAdapter
		}
		r.adapterCount++
		r.lastAdapter = f.Adapter
		targetCall, targetKind := Target(which)
		if targetCall == index+1 && targetKind == kind {
			r.next = TargetReached
		} else if r.adapterCount == 37 {
			r.next = PublicCommitReturned
		}
	case TargetReached:
		if f.Ticket != spec.Ticket || f.Adapter != r.lastAdapter || r.adapterCount != ExpectedAdapterEvents(which) {
			return ProblemTarget
		}
		var problem Problem
		r.targetHash, problem = FrameDigest(f)
		if problem != ProblemNone {
			return problem
		}
		r.next = ExitGranted
	case PublicCommitReturned:
		want := CommitEvidence{Calls: 1, Value: CommitNil, CapturedAdjacent: true, CapturedUnchanged: true, PublicDone: true, PublicDetached: true, Main: rollbackfault.OwnerRetired, Journal: rollbackfault.OwnerRetired, Cleanup: rollbackfault.OwnerRetired}
		if f.Commit != want || (which != Control && which != AcknowledgedExit) {
			r.status = CommitUncertainDiagnostic
			return ProblemCommit
		}
		var problem Problem
		r.commitHash, problem = FrameDigest(f)
		if problem != ProblemNone {
			return problem
		}
		raw, encodeProblem := EncodeFrame(f)
		if encodeProblem != ProblemNone {
			return encodeProblem
		}
		r.commitBodyHash, r.commitBodyBytes = digestBytes(raw[4:]), int64(len(raw)-4)
		r.commitSequence = f.Sequence
		r.next = NormalFinishGranted
		if which == AcknowledgedExit {
			r.next = AcknowledgementDurable
		}
	case AcknowledgementDurable:
		if !validAck(f.Ack, r.commitSequence, r.commitHash, r.commitBodyHash, r.commitBodyBytes) {
			r.status = AckUncertain
			return ProblemAck
		}
		r.ack, r.next = f.Ack, ExitGranted
	case ExitGranted:
		if which == AcknowledgedExit {
			if f.Ack != r.ack || f.Digest != r.ack.SHA256 || !validDigest(f.Digest) {
				return ProblemAck
			}
		} else if which == Control || f.Ack != (AckEvidence{}) || f.Digest != r.targetHash || !validDigest(f.Digest) {
			return ProblemTarget
		}
		r.next = WriterWaitObserved
	case NormalFinishGranted:
		if which != Control || f.Digest != r.commitHash {
			return ProblemCommit
		}
		r.next = WriterFinished
	case WriterFinished:
		r.next = WriterWaitObserved
	case WriterWaitObserved:
		code := ExpectedExitCode
		if which == Control {
			code = 0
		}
		if !validWait(f.Process, r.writer, code) {
			r.status = waitStatus(f.Process)
			return ProblemProcess
		}
		r.writerWaited, r.next = true, OriginalsFrozen
	case OriginalsFrozen:
		if !r.writerWaited || !validDigest(f.Digest) {
			return ProblemProcess
		}
		r.frozenDigest, r.next = f.Digest, RecoverySpawned
	case RecoverySpawned:
		if !r.writerWaited || !validSpawn(f.Process, spec) || f.Process.Handle == r.writer.Handle || f.Process.PID == r.parent.PID {
			return ProblemProcess
		}
		r.recovery, r.next = f.Process, RecoveryReadyBound
	case RecoveryReadyBound:
		if !validReady(f.Process, r.recovery, spec) {
			return ProblemProcess
		}
		r.next = RecoveryReadbackGranted
	case RecoveryReadbackGranted:
		r.next = ReadbackResult
	case ReadbackResult:
		if !validDigest(f.Digest) {
			return ProblemPayload
		}
		r.readbackDigest, r.next = f.Digest, RecoveryWaitObserved
	case RecoveryWaitObserved:
		if !validWait(f.Process, r.recovery, 0) {
			r.status = waitStatus(f.Process)
			return ProblemProcess
		}
		r.recoveryWaited, r.next = true, ArtifactsRechecked
	case ArtifactsRechecked:
		if !r.recoveryWaited || !validDigest(f.Digest) {
			return ProblemProcess
		}
		r.checkedDigest, r.next = f.Digest, ScenarioEvidenceComplete
	case ScenarioEvidenceComplete:
		if !r.writerWaited || !r.recoveryWaited || r.adapterCount != ExpectedAdapterEvents(which) {
			return ProblemIncomplete
		}
		r.next = FrameUnknown
	default:
		return ProblemOrder
	}
	return ProblemNone
}
func successfulResult(call rollbackfault.Call) rollbackfault.Result {
	effect := rollbackfault.EffectUnknown
	switch call.Point {
	case rollbackfault.DBOriginalRead:
		effect = rollbackfault.EffectNone
	case rollbackfault.JournalInvalid, rollbackfault.JournalRecord, rollbackfault.JournalValidHeader, rollbackfault.DBPage:
		effect = rollbackfault.EffectPossible
	}
	return rollbackfault.Result{Executed: true, NativeN: call.Length, ReturnedN: call.Length, Effect: effect}
}
func validSelf(p ProcessEvidence, spec PlanSpec) bool {
	return p.PID > 0 && p.Image == spec.Image && p.ImageHash == spec.Binding.Binary && p.ActualImage && !p.SpawnObserved && !p.WaitObserved && p.State == ProcessRunning && p.ExitCode == 0 && p.Signal == 0 && !p.TimedOut
}
func validSpawn(p ProcessEvidence, spec PlanSpec) bool {
	return p.PID > 0 && p.Handle != 0 && p.Image == spec.Image && p.ImageHash == spec.Binding.Binary && !p.ActualImage && p.SpawnObserved && !p.WaitObserved && p.State == ProcessRunning && p.ExitCode == 0 && p.Signal == 0 && !p.TimedOut
}
func validReady(p, spawned ProcessEvidence, spec PlanSpec) bool {
	return validSelf(p, spec) && p.PID == spawned.PID && p.Handle == spawned.Handle
}
func validWait(p, spawned ProcessEvidence, code int) bool {
	want := spawned
	want.ActualImage, want.SpawnObserved, want.WaitObserved = false, false, true
	want.State, want.ExitCode = ProcessExited, code
	return p == want
}
func waitStatus(p ProcessEvidence) Diagnostic {
	if p.State == ProcessRunning {
		return ChildLive
	}
	if !p.WaitObserved || p.State == ProcessUnknown {
		return WaitUnknown
	}
	return ChildExitUnexpected
}
func validAck(a AckEvidence, sequence uint8, raw, body Digest, bodyBytes int64) bool {
	return sequence != 0 && a.CommitSequence == sequence && a.CommitFrameSHA256 == raw && validDigest(raw) && a.SHA256 == body && validDigest(body) && validFile(a.File) && a.Bytes == bodyBytes && a.Bytes > 0 && a.Bytes <= MaxFrameBytes && a.Written == a.Bytes && a.Exclusive && a.WriteStep == 1 && a.FileSyncStep == 2 && a.FileCloseStep == 3 && a.DirectorySyncStep == 4 && a.DirectoryCloseStep == 5 && a.FileCloseCalls == 1 && a.DirectoryCloseCalls == 1 && a.ErrorToken == 0
}
func (r *Receipt) Seal() Observation {
	r.sealed = true
	o := Observation{Problem: r.problem, Status: r.status, FrameCount: r.count, AdapterEventCount: r.adapterCount, Acknowledged: r.ack != (AckEvidence{}), WriterWaited: r.writerWaited, RecoveryWaited: r.recoveryWaited}
	if r.commitSequence != 0 {
		o.Commit = CommitNil
	}
	if !r.plan.valid {
		o.Problem = ProblemPlan
		return o
	}
	if r.problem != ProblemNone {
		return o
	}
	if r.next != FrameUnknown {
		o.Problem = ProblemIncomplete
		if r.count == 0 {
			o.Status = NotRun
		}
		return o
	}
	o.EvidenceConsistent = true
	// SQL and artifact qualification deliberately require Observe as well.
	return o
}

// SnapshotDigest hashes only a complete, already qualified literal OLD or NEW
// SQL snapshot. The framing digest is separate from the database-byte digest.
func SnapshotDigest(s rollbackfault.SQLSnapshotEvidence) (Digest, Problem) {
	if rollbackfault.ValidateSQLSnapshot(s, s.Identity, rollbackfault.SQLOld) != rollbackfault.SQLProblemNone && rollbackfault.ValidateSQLSnapshot(s, s.Identity, rollbackfault.SQLNew) != rollbackfault.SQLProblemNone {
		return "", ProblemSQL
	}
	raw, err := json.Marshal(s)
	if err != nil || len(raw) > MaxSnapshotBytes {
		return "", ProblemEncoding
	}
	return digestBytes(raw), ProblemNone
}
func ArtifactName(role ArtifactRole) string {
	switch role {
	case BaselineArtifact:
		return "inputs/baseline.input.db"
	case TrialArtifact:
		return "trial/trial.db"
	case TrialJournalArtifact:
		return "trial/trial.db-journal"
	case PostExitArtifact:
		return "evidence/post-exit.input.db"
	case PostExitJournalArtifact:
		return "evidence/post-exit-journal.input.bin"
	case ReplayArtifact:
		return "replay/replay.db"
	case ReplayJournalArtifact:
		return "replay/replay.db-journal"
	case AcknowledgementArtifact:
		return "control/ack.json"
	default:
		return ""
	}
}
func ArtifactManifestDigest(artifacts [ArtifactCount]ArtifactEvidence, after bool) (Digest, Problem) {
	var stamps [ArtifactCount]ArtifactStamp
	for i, a := range artifacts {
		if a.Role != ArtifactRole(i) {
			return "", ProblemArtifacts
		}
		s := a.Before
		if after {
			s = a.After
		}
		if s != (ArtifactStamp{}) && !validStamp(s, ArtifactRole(i)) {
			return "", ProblemArtifacts
		}
		stamps[i] = s
	}
	raw, err := json.Marshal(stamps)
	if err != nil || len(raw) > MaxArtifactManifestBytes {
		return "", ProblemEncoding
	}
	return digestBytes(raw), ProblemNone
}
func validStamp(s ArtifactStamp, role ArtifactRole) bool {
	if s.Path != ArtifactName(role) || !s.Exists || !s.Regular || s.Links != 1 || !validFile(s.File) || !validDigest(s.SHA256) {
		return false
	}
	if role == AcknowledgementArtifact {
		return s.Bytes > 0 && s.Bytes <= MaxFrameBytes && s.Pages == ([3]Digest{})
	}
	if role == TrialJournalArtifact || role == PostExitJournalArtifact || role == ReplayJournalArtifact {
		return s.Bytes == 16408 && s.Pages == ([3]Digest{})
	}
	if s.Bytes != 12288 {
		return false
	}
	for _, page := range s.Pages {
		if !validDigest(page) {
			return false
		}
	}
	return true
}
func sameContent(a, b ArtifactStamp) bool {
	return a.Exists && b.Exists && a.Bytes == b.Bytes && a.SHA256 == b.SHA256 && a.Pages == b.Pages
}
func copiedArtifact(a ArtifactEvidence) bool {
	return a.ExclusiveCopy && a.CompleteRead && a.CompleteWrite && a.FileSynced && a.FileClosedOnce && a.DirectorySynced && a.DirectoryClosedOnce
}
func hasCopyClaims(a ArtifactEvidence) bool {
	return a.ExclusiveCopy || a.CompleteRead || a.CompleteWrite || a.FileSynced || a.FileClosedOnce || a.DirectorySynced || a.DirectoryClosedOnce
}
func validateArtifacts(p Plan, r Receipt, e ScenarioEvidence) bool {
	which := p.spec.Binding.Case
	artifacts := e.Artifacts
	for i, a := range artifacts {
		role := ArtifactRole(i)
		if a.Role != role {
			return false
		}
		journal := role == TrialJournalArtifact || role == PostExitJournalArtifact || role == ReplayJournalArtifact
		present := !journal || which == AfterTablePage
		if role == AcknowledgementArtifact {
			present = which == AcknowledgedExit
		}
		if !present {
			if a != (ArtifactEvidence{Role: role}) {
				return false
			}
			continue
		}
		if !validStamp(a.Before, role) || a.Before.File == p.spec.Binding.Root || a.Before.File == p.spec.Image {
			return false
		}
		if role == ReplayJournalArtifact {
			if a.After != (ArtifactStamp{}) {
				return false
			}
		} else {
			if !validStamp(a.After, role) || a.Before.File != a.After.File {
				return false
			}
			if role != ReplayArtifact && a.Before != a.After {
				return false
			}
			if role == ReplayArtifact && which != AfterTablePage && a.Before != a.After {
				return false
			}
		}
		copyRole := role == PostExitArtifact || role == PostExitJournalArtifact || role == ReplayArtifact || role == ReplayJournalArtifact
		if copyRole {
			if !copiedArtifact(a) {
				return false
			}
		} else if hasCopyClaims(a) {
			return false
		}
		for j := 0; j < i; j++ {
			other := artifacts[j]
			if other.Before.Exists && other.Before.File == a.Before.File {
				return false
			}
			if other.After.Exists && a.After.Exists && other.After.File == a.After.File {
				return false
			}
		}
	}
	baseline, trial := artifacts[BaselineArtifact].Before, artifacts[TrialArtifact].Before
	if baseline.SHA256 != p.spec.Binding.Baseline || !sameContent(trial, artifacts[PostExitArtifact].Before) || !sameContent(trial, artifacts[ReplayArtifact].Before) {
		return false
	}
	if which == BeforeJCreate {
		if !sameContent(trial, baseline) {
			return false
		}
	} else {
		if trial.SHA256 == baseline.SHA256 || trial.Pages[0] == baseline.Pages[0] || trial.Pages[1] == baseline.Pages[1] {
			return false
		}
		if which == AfterTablePage {
			if trial.Pages[2] != baseline.Pages[2] {
				return false
			}
		} else if trial.Pages[2] == baseline.Pages[2] {
			return false
		}
	}
	if which == AfterTablePage {
		if !sameContent(artifacts[ReplayArtifact].After, baseline) || !sameContent(artifacts[TrialJournalArtifact].Before, artifacts[PostExitJournalArtifact].Before) || !sameContent(artifacts[TrialJournalArtifact].Before, artifacts[ReplayJournalArtifact].Before) {
			return false
		}
	}
	if which == AcknowledgedExit {
		a := artifacts[AcknowledgementArtifact].Before
		if a.File != r.ack.File || a.SHA256 != r.ack.SHA256 || a.Bytes != r.ack.Bytes {
			return false
		}
	}
	before, problem := ArtifactManifestDigest(artifacts, false)
	if problem != ProblemNone || before != r.frozenDigest {
		return false
	}
	after, problem := ArtifactManifestDigest(artifacts, true)
	return problem == ProblemNone && after == r.checkedDigest
}
func validLifecycle(which Case, got WriterLifecycle) bool {
	want := WriterLifecycle{OpenCalls: 1, BeginCalls: 1, DMLCalls: 3, ArmCalls: 1, CommitCalls: 1, Main: AbandonedByExpectedProcessExit, Journal: AbandonedByExpectedProcessExit, Cleanup: AbandonedByExpectedProcessExit}
	switch which {
	case BeforeJCreate:
		want.Journal = NotAcquired
	case AfterJUnlink:
		want.Journal = RetiredNormally
	case Control, AcknowledgedExit:
		want.PublicReturned, want.Main, want.Journal, want.Cleanup = true, RetiredNormally, RetiredNormally, RetiredNormally
		if which == Control {
			want.FinishCalls, want.DisarmCalls, want.DBCloseCalls = 1, 1, 1
		}
	}
	return got == want
}
func validRecovery(p Plan, got RecoveryEvidence) bool {
	want := RecoveryEvidence{Generation: p.spec.Recovery.Generation, OwnersRetired: true, ReadOnly: true, NoCreate: true, OpenCalls: 1, CheckMaxPages: 3, CheckMaxPageReads: 512, NoRetry: true}
	if p.spec.Binding.Case == AfterTablePage {
		want.Calls = 1
		want.Limits = rollbackfault.SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}
		want.JournalPresentBefore, want.Replayed, want.ReplayedPages, want.OriginalPages = true, true, 3, 3
	}
	return got == want
}
func validRuntime(got RuntimeEvidence) bool {
	return got == (RuntimeEvidence{
		EnvFirst: true, ExactRoleSelectors: true, FixedAllowlistEnvironment: true,
		CountOne: true, ParallelOne: true, CPUOne: true, ShuffleOff: true, TimeoutZero: true,
		PrivateRoot0700: true, NoSymlinkAncestors: true, RootUnchanged: true,
		TwoDedicatedPipes: true, PipeOwnersRecorded: true, UnusedPipeEndsClosedOnce: true,
		BoundedDiagnostics: true, NoInheritedDatabaseOwners: true, ParentOwnersClosedBeforeSpawn: true,
		FreshRegistryEachChild: true, OnlyParentSpawns: true, NoSignalOrKill: true,
		NoRetries: true, OriginalsNeverSyncedAfterExit: true, ArtifactsRetained: true,
		AdapterSameTransaction: true, AdapterSameScope: true, AdapterSameOwners: true,
		ObserverOutsidePrivateMutex: true, OriginalTxLockHeldAtCut: true, NoCallbackNativeReplacement: true,
	})
}

// Observe replays the private raw frames, then independently checks generations,
// exact OLD/NEW SQL snapshots, lifecycle, copy identity and retained artifacts.
// The old rollbackfault.ObserveSQLLedger terminal model is intentionally unused.
func Observe(p Plan, supplied Receipt, e ScenarioEvidence) Observation {
	bad := func(problem Problem, status Diagnostic) Observation {
		return Observation{Problem: problem, Status: status}
	}
	if e.PhysicalClaim {
		return bad(ProblemPhysicalClaim, ProtocolInvalid)
	}
	if !p.valid || !supplied.plan.valid || p.spec != supplied.plan.spec || supplied.count > MaxFrames {
		return bad(ProblemPlan, ProtocolInvalid)
	}
	r, problem := NewReceipt(p)
	if problem != ProblemNone {
		return bad(problem, ProtocolInvalid)
	}
	for i := uint8(0); i < supplied.count; i++ {
		if r.Append(supplied.frames[i]) != ProblemNone {
			break
		}
	}
	o := r.Seal()
	// Rejected identity or sequence frames may not be retained; their sticky
	// rejection still must not disappear when accepted-prefix values replay.
	if supplied.problem != ProblemNone {
		o.Problem, o.Status, o.EvidenceConsistent = supplied.problem, supplied.status, false
		return o
	}
	if !o.EvidenceConsistent {
		return o
	}
	if e.Binding != p.spec.Binding || e.BeforeGeneration != p.spec.Controller.Generation || e.AfterGeneration != p.spec.Recovery.Generation {
		o.Problem, o.Status = ProblemIdentity, ProtocolInvalid
		o.EvidenceConsistent = false
		return o
	}
	if !validLifecycle(p.spec.Binding.Case, e.Lifecycle) || !validRuntime(e.Runtime) {
		o.Problem, o.Status = ProblemLifecycle, OwnerUnknown
		o.EvidenceConsistent = false
		return o
	}
	if !validRecovery(p, e.Recovery) {
		o.Problem, o.Status = ProblemRecovery, RecoveryFailed
		o.EvidenceConsistent = false
		return o
	}
	if !validateArtifacts(p, r, e) {
		o.Problem, o.Status = ProblemArtifacts, ArtifactChanged
		o.EvidenceConsistent = false
		return o
	}
	state := ExpectedSQLState(p.spec.Binding.Case)
	if rollbackfault.ValidateSQLSnapshot(e.Before, p.spec.AdapterIdentity, rollbackfault.SQLOld) != rollbackfault.SQLProblemNone || rollbackfault.ValidateSQLSnapshot(e.After, p.spec.AdapterIdentity, state) != rollbackfault.SQLProblemNone {
		o.Problem, o.Status = ProblemSQL, ReadbackFailed
		o.EvidenceConsistent = false
		return o
	}
	before, problem := SnapshotDigest(e.Before)
	if problem != ProblemNone || before != r.baselineDigest {
		o.Problem, o.Status = ProblemSQL, ReadbackFailed
		o.EvidenceConsistent = false
		return o
	}
	after, problem := SnapshotDigest(e.After)
	if problem != ProblemNone || after != r.readbackDigest {
		o.Problem, o.Status = ProblemSQL, ReadbackFailed
		o.EvidenceConsistent = false
		return o
	}
	// Local reader/iterator ordinals may legitimately recur across generations;
	// no global-integer inequality is substituted for the fresh-process proof.
	o.ScenarioObserved, o.SQLState, o.Status = true, state, EvidenceComplete
	return o
}

// ScenarioRun contains private receipts, not a caller-forgeable Observation.
// Each native controller invocation still admits exactly one of the five cases.
// This optional pure aggregator rejects missing/duplicate cases and trial/root
// reuse instead of claiming that one successful case covers the whole suite.
type ScenarioRun struct {
	Plan     Plan
	Receipt  Receipt
	Evidence ScenarioEvidence
}
type SuiteObservation struct {
	EvidenceConsistent bool
	Problem            Problem
	Cases              [5]Observation
}

func (SuiteObservation) PhysicalValidated() bool { return false }
func (SuiteObservation) Pass() bool              { return false }
func ObserveSuite(runs [5]ScenarioRun) SuiteObservation {
	out := SuiteObservation{}
	cases := Cases()
	for i, run := range runs {
		if !run.Plan.valid || run.Plan.spec.Binding.Case != cases[i] {
			out.Problem = ProblemPlan
			return out
		}
		b := run.Plan.spec.Binding
		for j := 0; j < i; j++ {
			prior := runs[j].Plan.spec.Binding
			if b.Trial == prior.Trial || b.Root == prior.Root {
				out.Problem = ProblemIdentity
				return out
			}
			common, expected := b, prior
			common.Trial, common.Root, common.Case = "", FileIdentity{}, CaseUnknown
			expected.Trial, expected.Root, expected.Case = "", FileIdentity{}, CaseUnknown
			if common != expected || run.Plan.spec.Image != runs[j].Plan.spec.Image {
				out.Problem = ProblemIdentity
				return out
			}
			roles := [3]RoleIdentity{run.Plan.spec.Controller, run.Plan.spec.Writer, run.Plan.spec.Recovery}
			previous := [3]RoleIdentity{runs[j].Plan.spec.Controller, runs[j].Plan.spec.Writer, runs[j].Plan.spec.Recovery}
			for _, role := range roles {
				for _, other := range previous {
					if role.Challenge == other.Challenge {
						out.Problem = ProblemIdentity
						return out
					}
				}
			}
			for _, artifact := range run.Evidence.Artifacts {
				if !artifact.Before.Exists {
					continue
				}
				for _, other := range runs[j].Evidence.Artifacts {
					if other.Before.Exists && artifact.Before.File == other.Before.File {
						out.Problem = ProblemArtifacts
						return out
					}
				}
			}
		}
		out.Cases[i] = Observe(run.Plan, run.Receipt, run.Evidence)
		if !out.Cases[i].ScenarioObserved || !out.Cases[i].EvidenceConsistent || out.Cases[i].Problem != ProblemNone {
			out.Problem = out.Cases[i].Problem
			if out.Problem == ProblemNone {
				out.Problem = ProblemIncomplete
			}
			return out
		}
	}
	out.EvidenceConsistent = true
	return out
}
