// Package rollbackfault models a finite, descriptor-free plain rollback adapter
// receipt. It performs no I/O and cannot establish physical durability, recovery,
// WAL correctness, encryption correctness, or a physical acceptance PASS.
package rollbackfault

const (
	MaxEvents   = 128
	MaxCalls    = 32
	MaxIDLength = 128
)

type Profile uint8

const (
	ProfileUnknown Profile = iota
	PlainRollback4096
)

// Identity binds every event to one frozen artifact, trial, and transaction scope.
// IDs are labels supplied by a future caller, not hashes verified by this model.
type Identity struct {
	Suite    string
	Source   string
	Tool     string
	Binary   string
	Manifest string
	Trial    string
	Scope    string
	Profile  Profile
}

type Point uint8

const (
	PointUnknown Point = iota
	JournalCreate
	JournalInvalid
	DBOriginalRead
	JournalRecord
	JournalBodySync
	JournalValidHeader
	JournalHeaderSync
	PublishDirectorySync
	DBPage
	DBSync
	JournalUnlink
	FinalDirectorySync
	CleanupUnlink
	CleanupDirectorySync
)

type Role uint8

const (
	RoleUnknown Role = iota
	Database
	Journal
	Directory
)

type Action uint8

const (
	Pass Action = iota
	FailBefore
	PrefixShortWrite
)

type Call struct {
	Point      Point
	Occurrence uint8
	Role       Role
	Offset     int64
	Length     int
	Owner      uint64
}

type CommitClass uint8

const (
	CommitUnknown CommitClass = iota
	CommitSucceeded
	CommitFailed
	CommitRecoveryRequired
	CommitUncertain
	CommitCleanupPending
	CommitUnsupported
	CommitRefused
	CommitSkipped
)

// PlanSpec is copied at admission. OriginalPages counts journaled original page
// images, not the database's original extent. The concrete armer must separately
// check that extent and all actual page offsets against the supplied manifest.
// DirectoryOwner names the composite directory helper scope, not a descriptor.
type PlanSpec struct {
	Identity       Identity
	Target         Call
	Action         Action
	Prefix         int
	PrefixError    bool
	MainOwner      uint64
	JournalOwner   uint64
	DirectoryOwner uint64
	CleanupOwner   uint64
	OriginalPages  uint8
	DirtyPages     uint8
	PageSize       uint32
	SectorSize     uint32
	Calls          [MaxCalls]Call
	CallCount      uint8
	ExpectedCommit CommitClass
}

type Plan struct {
	spec  PlanSpec
	valid bool
}

type Problem uint8

const (
	ProblemNone Problem = iota
	ProblemIdentity
	ProblemProfile
	ProblemOwners
	ProblemGeometry
	ProblemAction
	ProblemTarget
	ProblemCallPath
	ProblemCommitExpectation
	ProblemUnadmittedPlan
	ProblemSealed
	ProblemEventLimit
	ProblemOrdinal
	ProblemEventOrder
	ProblemEventPayload
	ProblemCallMismatch
	ProblemErrorEvidence
	ProblemNativeResult
	ProblemErrorMasked
	ProblemUnexpectedCommit
	ProblemLogicalState
	ProblemOwnerObservation
	ProblemPhysicalClaim
	ProblemIncomplete
	ProblemUnderlyingFailure
	ProblemOwnersUnresolved
	ProblemUnqualifiedCommit
)

const NoProblem = ProblemNone

func NewPlan(spec PlanSpec) (Plan, Problem) {
	if !validIdentity(spec.Identity) {
		return Plan{}, ProblemIdentity
	}
	if spec.Identity.Profile != PlainRollback4096 {
		return Plan{}, ProblemProfile
	}
	owners := [4]uint64{spec.MainOwner, spec.JournalOwner, spec.DirectoryOwner, spec.CleanupOwner}
	for i, owner := range owners {
		if owner == 0 {
			return Plan{}, ProblemOwners
		}
		for j := 0; j < i; j++ {
			if owners[j] == owner {
				return Plan{}, ProblemOwners
			}
		}
	}
	if spec.PageSize != 4096 || spec.SectorSize != 4096 || spec.OriginalPages < 1 || spec.OriginalPages > 4 || spec.DirtyPages < 1 || spec.DirtyPages > 4 || spec.OriginalPages > spec.DirtyPages {
		return Plan{}, ProblemGeometry
	}
	if spec.Action > PrefixShortWrite || (spec.Action != PrefixShortWrite && (spec.Prefix != 0 || spec.PrefixError)) {
		return Plan{}, ProblemAction
	}
	if spec.Target.Point < JournalCreate || spec.Target.Point > FinalDirectorySync || spec.Target.Occurrence == 0 {
		return Plan{}, ProblemTarget
	}
	if spec.Action == PrefixShortWrite && (!writePoint(spec.Target.Point) || spec.Prefix < 0 || spec.Prefix >= spec.Target.Length) {
		return Plan{}, ProblemAction
	}
	if spec.ExpectedCommit != expectedCommit(spec) {
		return Plan{}, ProblemCommitExpectation
	}
	if problem := validCalls(spec); problem != ProblemNone {
		return Plan{}, problem
	}
	// A short caller substring can share a much larger backing allocation.
	// Freeze only these already validated bytes, never the caller's storage.
	spec.Identity = copyIdentity(spec.Identity)
	return Plan{spec: spec, valid: true}, ProblemNone
}

func copyIdentity(id Identity) Identity {
	id.Suite = copyIdentityID(id.Suite)
	id.Source = copyIdentityID(id.Source)
	id.Tool = copyIdentityID(id.Tool)
	id.Binary = copyIdentityID(id.Binary)
	id.Manifest = copyIdentityID(id.Manifest)
	id.Trial = copyIdentityID(id.Trial)
	id.Scope = copyIdentityID(id.Scope)
	return id
}

// Only called after identity admission. The fixed buffer bounds even a possible
// retained conversion backing array; it cannot alias the input's large storage.
func copyIdentityID(value string) string {
	var bounded [MaxIDLength]byte
	n := copy(bounded[:], value)
	return string(bounded[:n])
}

// Spec returns a detached value, including the fixed-size call inventory.
func (p Plan) Spec() PlanSpec { return p.spec }

func validIdentity(id Identity) bool {
	ids := [7]string{id.Suite, id.Source, id.Tool, id.Binary, id.Manifest, id.Trial, id.Scope}
	for _, value := range ids {
		if len(value) == 0 || value == "0" || len(value) > MaxIDLength {
			return false
		}
		for _, ch := range value {
			if ch < '!' || ch > '~' {
				return false
			}
		}
	}
	return true
}

func writePoint(point Point) bool {
	return point == JournalInvalid || point == JournalRecord || point == JournalValidHeader || point == DBPage
}

func expectedCommit(spec PlanSpec) CommitClass {
	if spec.Action == Pass {
		return CommitSucceeded
	}
	switch spec.Target.Point {
	case DBPage, DBSync, JournalUnlink:
		return CommitRecoveryRequired
	case FinalDirectorySync:
		return CommitUncertain
	default:
		return CommitFailed
	}
}

// The complete expected trace is structural, not an arbitrary caller checklist.
// Native errors outside the selected action may depart from it; that is evidence
// of an unfulfilled plan, never grounds to silently shorten the checklist.
func validCalls(spec PlanSpec) Problem {
	if spec.CallCount == 0 || int(spec.CallCount) > MaxCalls {
		return ProblemCallPath
	}
	next := JournalCreate
	var occurrences [15]uint8
	var hits uint8
	lastRead, lastWrite := int64(-1), int64(-1)
	for i := 0; i < int(spec.CallCount); i++ {
		call := spec.Calls[i]
		if next == PointUnknown || call.Point != next {
			return ProblemCallPath
		}
		occurrences[call.Point]++
		if call.Occurrence != occurrences[call.Point] || !validCall(spec, call) {
			return ProblemCallPath
		}
		if call.Point == DBOriginalRead {
			if (call.Occurrence == 1 && call.Offset != 0) || call.Offset <= lastRead {
				return ProblemCallPath
			}
			lastRead = call.Offset
		}
		if call.Point == DBPage {
			if (call.Occurrence == 1 && call.Offset != 0) || call.Offset <= lastWrite {
				return ProblemCallPath
			}
			lastWrite = call.Offset
		}
		selected := call == spec.Target
		if selected {
			hits++
		}
		if selected && spec.Action != Pass {
			next = PointUnknown
			if call.Point > JournalCreate && call.Point < DBPage {
				next = CleanupUnlink
			}
			continue
		}
		switch call.Point {
		case JournalCreate:
			next = JournalInvalid
		case JournalInvalid:
			next = DBOriginalRead
		case DBOriginalRead:
			next = JournalRecord
		case JournalRecord:
			next = JournalBodySync
			if call.Occurrence < spec.OriginalPages {
				next = DBOriginalRead
			}
		case JournalBodySync:
			next = JournalValidHeader
		case JournalValidHeader:
			next = JournalHeaderSync
		case JournalHeaderSync:
			next = PublishDirectorySync
		case PublishDirectorySync:
			next = DBPage
		case DBPage:
			next = DBSync
			if call.Occurrence < spec.DirtyPages {
				next = DBPage
			}
		case DBSync:
			next = JournalUnlink
		case JournalUnlink:
			next = FinalDirectorySync
		case FinalDirectorySync, CleanupDirectorySync:
			next = PointUnknown
		case CleanupUnlink:
			next = CleanupDirectorySync
		default:
			return ProblemCallPath
		}
	}
	if next != PointUnknown || hits != 1 {
		return ProblemTarget
	}
	for i := int(spec.CallCount); i < MaxCalls; i++ {
		if spec.Calls[i] != (Call{}) {
			return ProblemCallPath
		}
	}
	return ProblemNone
}

func validCall(spec PlanSpec, call Call) bool {
	role, owner := Journal, spec.JournalOwner
	length, offset := 0, int64(0)
	switch call.Point {
	case JournalCreate, JournalBodySync, JournalHeaderSync, JournalUnlink, CleanupUnlink:
	case JournalInvalid:
		length = 4096
	case JournalRecord:
		length, offset = 4104, 4096+int64(call.Occurrence-1)*4104
	case JournalValidHeader:
		length = 12
	case DBOriginalRead, DBPage:
		role, owner = Database, spec.MainOwner
		length, offset = 4096, call.Offset
		if offset < 0 || offset%4096 != 0 || offset > 3*4096 {
			return false
		}
	case DBSync:
		role, owner = Database, spec.MainOwner
	case PublishDirectorySync, FinalDirectorySync, CleanupDirectorySync:
		role, owner = Directory, spec.DirectoryOwner
	default:
		return false
	}
	return call.Role == role && call.Owner == owner && call.Length == length && call.Offset == offset
}

type ErrorKind uint8

const (
	None ErrorKind = iota
	Injected
	ShortWrite
	Underlying
)

type SideEffect uint8

const (
	EffectUnknown SideEffect = iota
	EffectNone
	EffectPrefix
	EffectPossible
)

// Result separates the actual managed-owner return, adapter return, and the
// writeFullAt result. Diagnostic tokens refer only to caller-owned evidence;
// this package never stores an error interface, function, or file handle.
type Result struct {
	Executed             bool
	NativeN              int
	NativeError          ErrorKind
	NativeErrorToken     uint64
	ReturnedN            int
	ReturnedError        ErrorKind
	ReturnedErrorToken   uint64
	NormalizedError      ErrorKind
	NormalizedErrorToken uint64
	Effect               SideEffect
}

type LogicalState uint8

const (
	LogicalUnknown LogicalState = iota
	LogicalOpen
	LogicalClosed
)

type CommitResult struct {
	Class      CommitClass
	Error      ErrorKind
	ErrorToken uint64
	// TargetError is independently derived from preservation of the original
	// target error in the returned Commit error; copying the plan is not evidence.
	TargetError      ErrorKind
	TargetErrorToken uint64
	Logical          LogicalState
}

type OwnerState uint8

const (
	OwnerUnknown OwnerState = iota
	OwnerLive
	OwnerPending
	OwnerRetired
	OwnerNotAcquired
)

type OwnersResult struct {
	MainOwner    uint64
	JournalOwner uint64
	CleanupOwner uint64
	Main         OwnerState
	Journal      OwnerState
	Cleanup      OwnerState
}

// This source slice has no native recovery evidence input at all. Ledger,
// ordinary/extension index checks and durable acknowledgement checks remain
// absent; journal disappearance cannot stand in for any of them.
type Validation uint8

const ValidationAbsent Validation = 0

type EventKind uint8

const (
	EventUnknown EventKind = iota
	RunStarted
	Intent
	Returned
	CommitReturned
	OwnersObserved
	ValidationObserved
	Terminal
	// This retention marker is never a legal protocol step. Append produces
	// it only on identity rejection, without retaining any untrusted string.
	EventIdentityRejected
)

type Event struct {
	Identity   Identity
	Ordinal    uint8
	Kind       EventKind
	Call       Call
	Action     Action
	Result     Result
	Commit     CommitResult
	Owners     OwnersResult
	Validation Validation
	// Rejection metadata is zero on every legal event. In a rejection marker,
	// Ordinal is the supplied ordinal and RejectedAt is the retained slot.
	Rejection  Problem
	RejectedAt uint8
}

type Outcome uint8

const (
	ObservationInvalid Outcome = iota
	ObservationIncomplete
	ObservationRefused
	AdapterObserved
)

type Observation struct {
	Outcome    Outcome
	Problem    Problem
	TargetHits uint8
	EventCount uint8
}

// Even a syntactically complete caller-supplied observation has no physical
// authority. A future independently reviewed evidence type must remain separate.
func (Observation) PhysicalValidated() bool { return false }
func (Observation) Pass() bool              { return false }

type Receipt struct {
	plan         Plan
	events       [MaxEvents]Event
	count        uint8
	callIndex    uint8
	targetHits   uint8
	next         EventKind
	problem      Problem
	refusal      Problem
	targetResult Result
	commit       CommitResult
	owners       OwnersResult
	sealed       bool
}

func NewReceipt(plan Plan) (Receipt, Problem) {
	if !plan.valid {
		return Receipt{}, ProblemUnadmittedPlan
	}
	return Receipt{plan: plan, next: RunStarted}, ProblemNone
}

// Events returns a detached bounded snapshot. Identity rejection retains only
// a distinct scalar marker, never the rejected identity or other input payload.
// Other rejected events contain only frozen identity and fixed-size values.
// Append errors are sticky, never reset by a retry.
func (r Receipt) Events() ([MaxEvents]Event, uint8) { return r.events, r.count }

func (r *Receipt) Append(event Event) Problem {
	if r.sealed {
		return ProblemSealed
	}
	if r.problem != ProblemNone {
		return r.problem
	}
	if !r.plan.valid {
		r.problem = ProblemUnadmittedPlan
		return r.problem
	}
	if int(r.count) >= MaxEvents {
		r.problem = ProblemEventLimit
		return r.problem
	}
	// Check all variable-size input before retention. Comparing after storage
	// would keep rejected unbounded strings alive even on a sticky failure.
	if !validIdentity(event.Identity) || event.Identity != r.plan.spec.Identity {
		r.problem = ProblemIdentity
		r.events[r.count] = Event{Ordinal: event.Ordinal, Kind: EventIdentityRejected, Rejection: ProblemIdentity, RejectedAt: r.count + 1}
		r.count++
		return r.problem
	}
	// Equal short strings can still borrow large backing storage. Keep only
	// this plan's bounded copied identity; every other event field is scalar.
	event.Identity = r.plan.spec.Identity
	r.events[r.count] = event
	r.count++
	r.problem = r.accept(event)
	return r.problem
}

func (r *Receipt) accept(event Event) Problem {
	if event.Identity != r.plan.spec.Identity {
		return ProblemIdentity
	}
	if event.Ordinal != r.count {
		return ProblemOrdinal
	}
	if event.Kind != r.next || r.next == EventUnknown {
		return ProblemEventOrder
	}
	if !canonicalEvent(event) {
		return ProblemEventPayload
	}
	spec := r.plan.spec
	switch event.Kind {
	case RunStarted:
		r.next = Intent
	case Intent, Returned:
		if r.callIndex >= spec.CallCount || event.Call != spec.Calls[r.callIndex] {
			return ProblemCallMismatch
		}
		action := Pass
		if event.Call == spec.Target {
			action = spec.Action
		}
		if event.Action != action {
			return ProblemAction
		}
		if event.Kind == Intent {
			if event.Call == spec.Target {
				r.targetHits++
			}
			r.next = Returned
			break
		}
		if problem := validResult(spec, event.Call, action, event.Result); problem != ProblemNone {
			return problem
		}
		if event.Result.NativeError != None || (action == Pass && event.Result.NormalizedError != None) {
			r.refusal = ProblemUnderlyingFailure
		}
		if event.Call == spec.Target {
			r.targetResult = event.Result
		}
		r.callIndex++
		r.next = Intent
		if r.callIndex == spec.CallCount {
			r.next = CommitReturned
		}
	case CommitReturned:
		if !validError(event.Commit.Error, event.Commit.ErrorToken) || !validError(event.Commit.TargetError, event.Commit.TargetErrorToken) || event.Commit.Class > CommitSkipped || event.Commit.Logical > LogicalClosed {
			return ProblemErrorEvidence
		}
		if event.Commit.Class == CommitUnknown || event.Commit.Class >= CommitCleanupPending {
			r.refusal = ProblemUnqualifiedCommit
		} else if event.Commit.Class != spec.ExpectedCommit {
			return ProblemUnexpectedCommit
		}
		if event.Commit.Class == CommitSucceeded {
			if event.Commit.Error != None || event.Commit.Logical != LogicalClosed {
				return ProblemUnexpectedCommit
			}
		} else if event.Commit.Error == None {
			return ProblemUnexpectedCommit
		}
		if event.Commit.TargetError != r.targetResult.NormalizedError || event.Commit.TargetErrorToken != r.targetResult.NormalizedErrorToken {
			return ProblemErrorMasked
		}
		if event.Commit.Logical == LogicalUnknown {
			r.refusal = ProblemLogicalState
		} else if spec.Target.Point != JournalCreate || spec.Action == Pass {
			if event.Commit.Logical != LogicalClosed {
				return ProblemLogicalState
			}
		}
		r.commit, r.next = event.Commit, OwnersObserved
	case OwnersObserved:
		owners := event.Owners
		if owners.MainOwner != spec.MainOwner || owners.JournalOwner != spec.JournalOwner || owners.CleanupOwner != spec.CleanupOwner || owners.Main > OwnerNotAcquired || owners.Journal > OwnerNotAcquired || owners.Cleanup > OwnerNotAcquired || owners.Main == OwnerNotAcquired || owners.Cleanup == OwnerNotAcquired {
			return ProblemOwnerObservation
		}
		unacquired := spec.Target.Point == JournalCreate && spec.Action == FailBefore
		if owners.Journal == OwnerNotAcquired && !unacquired {
			return ProblemOwnerObservation
		}
		if unacquired && owners.Journal == OwnerRetired {
			return ProblemOwnerObservation
		}
		r.owners, r.next = owners, ValidationObserved
	case ValidationObserved:
		if event.Validation != ValidationAbsent {
			return ProblemPhysicalClaim
		}
		r.next = Terminal
	case Terminal:
		if r.targetHits != 1 || r.callIndex != spec.CallCount {
			return ProblemIncomplete
		}
		r.next = EventUnknown
	}
	return ProblemNone
}

func canonicalEvent(event Event) bool {
	kind := event.Kind
	event.Identity, event.Ordinal, event.Kind = Identity{}, 0, EventUnknown
	switch kind {
	case Intent:
		event.Call, event.Action = Call{}, Pass
	case Returned:
		event.Call, event.Action, event.Result = Call{}, Pass, Result{}
	case CommitReturned:
		event.Commit = CommitResult{}
	case OwnersObserved:
		event.Owners = OwnersResult{}
	case ValidationObserved:
		event.Validation = ValidationAbsent
	}
	return event == (Event{})
}

func validError(kind ErrorKind, token uint64) bool {
	return kind <= Underlying && ((kind == Underlying && token != 0) || (kind != Underlying && token == 0))
}

func validResult(spec PlanSpec, call Call, action Action, result Result) Problem {
	if !validError(result.NativeError, result.NativeErrorToken) || !validError(result.ReturnedError, result.ReturnedErrorToken) || !validError(result.NormalizedError, result.NormalizedErrorToken) || result.Effect > EffectPossible {
		return ProblemErrorEvidence
	}
	if result.NativeN < 0 || result.ReturnedN < 0 || result.NativeN > call.Length || result.ReturnedN > call.Length {
		return ProblemNativeResult
	}
	if action == FailBefore {
		if result.Executed || result.NativeN != 0 || result.NativeError != None || result.ReturnedN != 0 || result.ReturnedError != Injected || result.NormalizedError != Injected || result.Effect != EffectNone {
			return ProblemNativeResult
		}
		return ProblemNone
	}
	if !result.Executed || result.NativeError == Injected {
		return ProblemNativeResult
	}
	if result.NativeError != None {
		expectedEffect := EffectUnknown
		if call.Point == DBOriginalRead {
			expectedEffect = EffectNone
		}
		if result.Effect != expectedEffect {
			return ProblemNativeResult
		}
		if result.ReturnedN != result.NativeN || result.ReturnedError != result.NativeError || result.ReturnedErrorToken != result.NativeErrorToken || result.NormalizedError != result.NativeError || result.NormalizedErrorToken != result.NativeErrorToken {
			return ProblemErrorMasked
		}
		if action == PrefixShortWrite && result.NativeN > spec.Prefix {
			return ProblemNativeResult
		}
		return ProblemNone
	}
	if action == PrefixShortWrite {
		exposed, normalized := None, ShortWrite
		if spec.PrefixError {
			exposed, normalized = Injected, Injected
		}
		effect := EffectPrefix
		if spec.Prefix == 0 {
			effect = EffectNone
		}
		if result.NativeN != spec.Prefix || result.ReturnedN != spec.Prefix || result.ReturnedError != exposed || result.NormalizedError != normalized || result.Effect != effect {
			return ProblemNativeResult
		}
		return ProblemNone
	}
	if result.ReturnedN != result.NativeN || result.ReturnedError != None {
		return ProblemErrorMasked
	}
	expectedEffect := EffectUnknown
	if call.Point == DBOriginalRead {
		expectedEffect = EffectNone
	}
	if writePoint(call.Point) {
		expectedEffect = EffectPossible
	}
	if result.Effect != expectedEffect {
		return ProblemNativeResult
	}
	normalized := None
	if writePoint(call.Point) && result.NativeN != call.Length {
		normalized = ShortWrite
	}
	if result.NormalizedError != normalized || (call.Point == DBOriginalRead && result.NativeN != call.Length) {
		return ProblemNativeResult
	}
	return ProblemNone
}

// Seal consumes no descriptor, closes no owner, performs no validation, and
// does not infer retirement from logical closure or the absence of diagnostics.
func (r *Receipt) Seal() Observation {
	r.sealed = true
	observation := Observation{Outcome: ObservationInvalid, Problem: r.problem, TargetHits: r.targetHits, EventCount: r.count}
	if !r.plan.valid {
		observation.Problem = ProblemUnadmittedPlan
		return observation
	}
	if r.problem != ProblemNone {
		return observation
	}
	if r.next != EventUnknown || r.targetHits != 1 {
		observation.Outcome, observation.Problem = ObservationIncomplete, ProblemIncomplete
		return observation
	}
	if r.refusal != ProblemNone {
		observation.Outcome, observation.Problem = ObservationRefused, r.refusal
		return observation
	}
	if r.owners.Main != OwnerRetired || r.owners.Cleanup != OwnerRetired || (r.owners.Journal != OwnerRetired && r.owners.Journal != OwnerNotAcquired) {
		observation.Outcome, observation.Problem = ObservationIncomplete, ProblemOwnersUnresolved
		return observation
	}
	observation.Outcome, observation.Problem = AdapterObserved, ProblemNone
	return observation
}
