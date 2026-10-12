// Package rekeyinterruption contains only bounded values for the six-case
// layout-1 rekey source facility. It has no descriptors or executable effects.
// Consistency of supplied values is never evidence that an operation occurred.
package rekeyinterruption

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
)

const (
	ProfileID        = "linux-layout1-rawkey256-4096"
	SchemaVersion    = 1
	MaxFrames        = 64
	MaxFrameBytes    = 4096
	FrameBodyBytes   = 256
	ExpectedExitCode = 86
	AbortExitCode    = 87
	MaxFamilyEntries = 64
	MaxFamilyBytes   = 1 << 20
	MaxFileBytes     = 64 << 10
	RootBytes        = 680
	BaseBytes        = 13112
	CheckScope       = "gosqlite-supported-profile-v1"
)

type Hash = [32]byte
type Profile uint8

const LinuxSnapshotRaw4096 Profile = 1

type Case uint8

const (
	CaseUnknown Case = iota
	Control
	BeforeRootRename
	AfterRootRenameBeforeDirSync
	AfterRootDirSyncBeforeAdoption
	RekeyReturnedNoAck
	AcknowledgedExit
)

func Cases() [6]Case {
	return [6]Case{Control, BeforeRootRename, AfterRootRenameBeforeDirSync, AfterRootDirSyncBeforeAdoption, RekeyReturnedNoAck, AcknowledgedExit}
}

func CaseName(c Case) string {
	switch c {
	case Control:
		return "control"
	case BeforeRootRename:
		return "before-root-rename"
	case AfterRootRenameBeforeDirSync:
		return "after-root-rename-before-dirsync"
	case AfterRootDirSyncBeforeAdoption:
		return "after-root-dirsync-before-adoption"
	case RekeyReturnedNoAck:
		return "rekey-returned-no-ack"
	case AcknowledgedExit:
		return "acknowledged-exit"
	default:
		return ""
	}
}

func ParseCase(s string) (Case, bool) {
	for _, c := range Cases() {
		if s == CaseName(c) {
			return c, true
		}
	}
	return CaseUnknown, false
}

type Point uint8

const (
	PointUnknown Point = iota
	BeforeRenameAttempt
	AfterSuccessfulRename
	AfterSuccessfulPostRenameDirSync
)

type Role uint8

const (
	RoleUnknown Role = iota
	Controller
	Writer
	OldProbe
	NewProbe
	ArchiveCheck
)

type Problem string

const (
	NoProblem            Problem = ""
	ProblemBinding       Problem = "binding"
	ProblemFrame         Problem = "frame"
	ProblemTrace         Problem = "trace"
	ProblemCredential    Problem = "credential-purpose"
	ProblemGeneration    Problem = "selected-generation"
	ProblemSQL           Problem = "sql-index-ledger"
	ProblemACK           Problem = "ack-causality"
	ProblemProcess       Problem = "process-ownership"
	ProblemOwner         Problem = "owner-retirement"
	ProblemArtifact      Problem = "artifact-preservation"
	ProblemNonce         Problem = "public-nonce-ledger"
	ProblemArchive       Problem = "archive-companion"
	ProblemPhysicalClaim Problem = "physical-claim-forbidden"
)

type FileIdentity struct{ Device, Inode uint64 }
type Binding struct {
	Profile                                                 Profile
	Case                                                    Case
	Trial, SourceManifest, Compiler, CompileReceipt, Binary Hash
}
type RoleBinding struct {
	Role       Role
	Generation uint64
	Challenge  Hash
}

func validBinding(b Binding) bool {
	return b.Profile == LinuxSnapshotRaw4096 && CaseName(b.Case) != "" &&
		b.Trial != (Hash{}) && b.SourceManifest != (Hash{}) && b.Compiler != (Hash{}) &&
		b.CompileReceipt != (Hash{}) && b.Binary != (Hash{})
}

type Event uint8

const (
	EventUnknown Event = iota
	ParentAdmitted
	BaselineCheckedAndClosed
	WriterStarted
	WriterReady
	RekeyGranted
	ObservedBeforeRename
	ObservedAfterRename
	ObservedAfterDirSync
	PublicRekeyNil
	WriterClosed
	WriterFinished
	AcknowledgementDurable
	AcknowledgementReceived
	ExitGranted
	WriterWaited
	OriginalsFrozen
	OldProbeStarted
	OldProbeReady
	OldProbeGranted
	OldProbeResult
	OldProbeWaited
	NewProbeStarted
	NewProbeReady
	NewProbeGranted
	NewProbeResult
	NewProbeWaited
	ArchiveStarted
	ArchiveReady
	ArchiveGranted
	ArchiveResult
	ArchiveWaited
	ArtifactsRechecked
	EvidenceComplete
)

func eventRole(event Event) Role {
	switch event {
	case WriterReady, ObservedBeforeRename, ObservedAfterRename, ObservedAfterDirSync,
		PublicRekeyNil, WriterClosed, WriterFinished, AcknowledgementReceived:
		return Writer
	case OldProbeReady, OldProbeResult:
		return OldProbe
	case NewProbeReady, NewProbeResult:
		return NewProbe
	case ArchiveReady, ArchiveResult:
		return ArchiveCheck
	default:
		if event > EventUnknown && event <= EvidenceComplete {
			return Controller
		}
		return RoleUnknown
	}
}

// ExpectedTrace specifies each owned barrier, not an interchangeable Sync
// occurrence. It is a newly allocated value and cannot alter a stored plan.
func ExpectedTrace(c Case) []Event { return ExpectedTraceWithArchive(c, c == Control) }

// Archive is an optional companion of Control, never a seventh case.
func ExpectedTraceWithArchive(c Case, archive bool) []Event {
	if CaseName(c) == "" || archive && c != Control {
		return nil
	}
	r := []Event{ParentAdmitted, BaselineCheckedAndClosed, WriterStarted, WriterReady, RekeyGranted, ObservedBeforeRename}
	if c != BeforeRootRename {
		r = append(r, ObservedAfterRename)
	}
	if c != BeforeRootRename && c != AfterRootRenameBeforeDirSync {
		r = append(r, ObservedAfterDirSync)
	}
	if c == Control || c == RekeyReturnedNoAck || c == AcknowledgedExit {
		r = append(r, PublicRekeyNil)
	}
	if c == Control {
		r = append(r, WriterClosed, WriterFinished)
	} else {
		if c == AcknowledgedExit {
			r = append(r, AcknowledgementDurable, AcknowledgementReceived)
		}
		r = append(r, ExitGranted)
	}
	r = append(r, WriterWaited, OriginalsFrozen, OldProbeStarted, OldProbeReady, OldProbeGranted, OldProbeResult, OldProbeWaited,
		NewProbeStarted, NewProbeReady, NewProbeGranted, NewProbeResult, NewProbeWaited)
	if archive {
		r = append(r, ArchiveStarted, ArchiveReady, ArchiveGranted, ArchiveResult, ArchiveWaited)
	}
	return append(r, ArtifactsRechecked, EvidenceComplete)
}

type Frame struct {
	Binding    Binding
	Role       Role
	Generation uint64
	Challenge  Hash
	Sequence   uint16
	Event      Event
	Payload    Hash
}

// The aggregate source-model transcript uses a fixed positional value encoding.
// It is not the manual bridge's per-role pipe encoding. Prefix, reserved bytes
// and exact extent are checked before any field use; it has no extensible fields.
func EncodeFrame(f Frame) ([]byte, Problem) {
	if !validBinding(f.Binding) || f.Role != eventRole(f.Event) || f.Role == RoleUnknown ||
		f.Generation == 0 || f.Challenge == (Hash{}) || f.Sequence == 0 || f.Sequence > MaxFrames || f.Payload == (Hash{}) {
		return nil, ProblemFrame
	}
	b := make([]byte, 4+FrameBodyBytes)
	binary.BigEndian.PutUint32(b[:4], FrameBodyBytes)
	p := b[4:]
	copy(p[:4], "RKI1")
	p[4], p[5], p[6], p[7], p[8] = SchemaVersion, byte(f.Binding.Profile), byte(f.Binding.Case), byte(f.Role), byte(f.Event)
	binary.BigEndian.PutUint16(p[10:12], f.Sequence)
	binary.BigEndian.PutUint64(p[12:20], f.Generation)
	copy(p[20:52], f.Binding.Trial[:])
	copy(p[52:84], f.Binding.SourceManifest[:])
	copy(p[84:116], f.Binding.Compiler[:])
	copy(p[116:148], f.Binding.CompileReceipt[:])
	copy(p[148:180], f.Binding.Binary[:])
	copy(p[180:212], f.Challenge[:])
	copy(p[212:244], f.Payload[:])
	return b, NoProblem
}

func DecodeFrame(b []byte) (Frame, Problem) {
	var f Frame
	if len(b) != 4+FrameBodyBytes || binary.BigEndian.Uint32(b[:4]) != FrameBodyBytes {
		return f, ProblemFrame
	}
	p := b[4:]
	if string(p[:4]) != "RKI1" || p[4] != SchemaVersion || p[9] != 0 {
		return f, ProblemFrame
	}
	for _, x := range p[244:] {
		if x != 0 {
			return Frame{}, ProblemFrame
		}
	}
	f.Binding.Profile, f.Binding.Case, f.Role, f.Event = Profile(p[5]), Case(p[6]), Role(p[7]), Event(p[8])
	f.Sequence, f.Generation = binary.BigEndian.Uint16(p[10:12]), binary.BigEndian.Uint64(p[12:20])
	copy(f.Binding.Trial[:], p[20:52])
	copy(f.Binding.SourceManifest[:], p[52:84])
	copy(f.Binding.Compiler[:], p[84:116])
	copy(f.Binding.CompileReceipt[:], p[116:148])
	copy(f.Binding.Binary[:], p[148:180])
	copy(f.Challenge[:], p[180:212])
	copy(f.Payload[:], p[212:244])
	if _, problem := EncodeFrame(f); problem != NoProblem {
		return Frame{}, problem
	}
	return f, NoProblem
}

func FrameHash(f Frame) (Hash, Problem) {
	b, p := EncodeFrame(f)
	if p != NoProblem {
		return Hash{}, p
	}
	return sha256.Sum256(b), NoProblem
}

// Transport values mirror ManualRekeyFrame's native JSON shape, including
// declaration/field order, without importing encryption or encstore. This
// per-role transport is separate from the aggregate parent evidence transcript.
// Encoding validates canonical extent only; it never grants a protocol action.
type TransportBinding struct {
	Trial, Source, Binary, Profile, Case, Role, Generation, Challenge string
}
type TransportKDF struct {
	MemoryKiB, Passes uint32
	Parallelism       uint8
}
type TransportEpoch struct {
	DatabaseID        [16]byte
	EpochID           Hash
	Kind              uint8
	KDF               TransportKDF
	KDFSalt, RootSalt Hash
}
type TransportDescriptor struct {
	Role                          uint8
	PageSize, PageCount           uint32
	RecordCount, CommitSequence   uint64
	TransactionID, ParentCommitID Hash
}
type TransportHeader struct {
	Major, Minor, Suite uint16
	Flags               uint32
	Epoch               TransportEpoch
	FileID              Hash
	NoncePrefix         [4]byte
	Descriptor          TransportDescriptor
}
type TransportBorrow struct {
	Acquires, Releases uint32
	Invalid            bool
}
type TransportObservation struct {
	OldHash, CandidateHash Hash
	Old, Candidate         TransportHeader
	OldBorrow, NewBorrow   TransportBorrow
}
type TransportFrame struct {
	Binding     TransportBinding
	Sequence    uint8
	Kind        string
	Point       uint8
	Observation TransportObservation
	PayloadHash Hash
}

func EncodeTransportFrame(frame TransportFrame) ([]byte, Problem) {
	body, err := json.Marshal(frame)
	if err != nil || len(body) == 0 || len(body) > MaxFrameBytes || frame.Sequence == 0 || frame.Sequence > MaxFrames {
		return nil, ProblemFrame
	}
	raw := make([]byte, len(body)+4)
	binary.BigEndian.PutUint32(raw[:4], uint32(len(body)))
	copy(raw[4:], body)
	return raw, NoProblem
}

func DecodeTransportFrame(raw []byte) (TransportFrame, Problem) {
	var frame TransportFrame
	if len(raw) < 5 || len(raw) > MaxFrameBytes+4 || int(binary.BigEndian.Uint32(raw[:4])) != len(raw)-4 {
		return frame, ProblemFrame
	}
	decoder := json.NewDecoder(bytes.NewReader(raw[4:]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&frame); err != nil {
		return TransportFrame{}, ProblemFrame
	}
	canonical, p := EncodeTransportFrame(frame)
	if p != NoProblem || !bytes.Equal(canonical, raw) {
		return TransportFrame{}, ProblemFrame
	}
	return frame, NoProblem
}

type Label uint8

const (
	LabelUnknown Label = iota
	OLD
	NEW
)

type Operation uint8

const (
	OperationUnknown Operation = iota
	OperationOpen
	OperationCreate
	OperationRekeyTarget
	OperationBackupTarget
	OperationRestoreTarget
)

type Purpose uint8

const (
	PurposeUnknown Purpose = iota
	OldSeed
	OldWriter
	NewRekeyTarget
	OldProbeOpen
	NewProbeOpen
	OldArchiveWriter
	OldArchiveSource
	NewRestoreTarget
	NewRestoreOpen
)

type Paths struct{ Original, Trial, ProbeOld, ProbeNew, Archive, Restore string }
type CredentialSpec struct {
	Purpose   Purpose
	Label     Label
	Role      Role
	Operation Operation
	Path      string
}
type CredentialUse struct {
	Spec                                                         CredentialSpec
	Provider                                                     uint64
	Acquires, Releases                                           uint8
	Cloned, Cleared, ImmutablePurpose, IndependentlyAdmittedPath bool
	DuplicateRelease, PurposeFromUntrustedHeader                 bool
}

func CredentialFor(paths Paths, purpose Purpose) (CredentialSpec, Problem) {
	s := CredentialSpec{Purpose: purpose}
	switch purpose {
	case OldSeed:
		s.Label, s.Role, s.Operation, s.Path = OLD, Controller, OperationCreate, paths.Original
	case OldWriter:
		s.Label, s.Role, s.Operation, s.Path = OLD, Writer, OperationOpen, paths.Trial
	case NewRekeyTarget:
		s.Label, s.Role, s.Operation, s.Path = NEW, Writer, OperationRekeyTarget, paths.Trial
	case OldProbeOpen:
		s.Label, s.Role, s.Operation, s.Path = OLD, OldProbe, OperationOpen, paths.ProbeOld
	case NewProbeOpen:
		s.Label, s.Role, s.Operation, s.Path = NEW, NewProbe, OperationOpen, paths.ProbeNew
	case OldArchiveWriter:
		s.Label, s.Role, s.Operation, s.Path = OLD, Controller, OperationBackupTarget, paths.Archive
	case OldArchiveSource:
		s.Label, s.Role, s.Operation, s.Path = OLD, ArchiveCheck, OperationOpen, paths.Archive
	case NewRestoreTarget:
		s.Label, s.Role, s.Operation, s.Path = NEW, ArchiveCheck, OperationRestoreTarget, paths.Restore
	case NewRestoreOpen:
		s.Label, s.Role, s.Operation, s.Path = NEW, ArchiveCheck, OperationOpen, paths.Restore
	default:
		return CredentialSpec{}, ProblemCredential
	}
	if !validPath(s.Path) {
		return CredentialSpec{}, ProblemCredential
	}
	return s, NoProblem
}

func validPath(path string) bool {
	if len(path) < 2 || len(path) > 512 || path[0] != '/' || path[len(path)-1] == '/' {
		return false
	}
	start := 1
	for i := 1; i <= len(path); i++ {
		if i < len(path) && (path[i] < 32 || path[i] == 127 || path[i] == '\\') {
			return false
		}
		if i == len(path) || path[i] == '/' {
			part := path[start:i]
			if part == "" || part == "." || part == ".." {
				return false
			}
			start = i + 1
		}
	}
	return true
}

func ValidatePaths(paths Paths) Problem {
	const suffix = "/inputs/original/fixture.gse"
	if !validPath(paths.Original) || len(paths.Original) <= len(suffix) || paths.Original[len(paths.Original)-len(suffix):] != suffix {
		return ProblemCredential
	}
	root := paths.Original[:len(paths.Original)-len(suffix)]
	if paths.Trial != root+"/trial/fixture.gse" || paths.ProbeOld != root+"/probe-old/fixture.gse" || paths.ProbeNew != root+"/probe-new/fixture.gse" ||
		paths.Archive != root+"/inputs/old-archive.gse" || paths.Restore != root+"/restore-new/fixture.gse" {
		return ProblemCredential
	}
	return NoProblem
}

func ValidateCredentialUse(paths Paths, u CredentialUse) Problem {
	want, p := CredentialFor(paths, u.Spec.Purpose)
	if p != NoProblem || u.Spec != want || u.Provider == 0 || u.Acquires != 1 || u.Releases != 1 ||
		!u.Cloned || !u.Cleared || !u.ImmutablePurpose || !u.IndependentlyAdmittedPath || u.DuplicateRelease || u.PurposeFromUntrustedHeader {
		return ProblemCredential
	}
	return NoProblem
}

// LogicalPages and PageOneCounters are synthetic value relations only. The
// native caller observes semantic SQL/Check and encrypted identities, not these
// plaintext byte hashes or counters; no native byte-preservation proof is made.
type Generation struct {
	Authenticated                                bool
	Label                                        Label
	Mode, Kind                                   string
	Layout, PageSize, PageCount                  uint32
	DatabaseID                                   [16]byte
	EpochID, RootSalt, RootFileID, BaseFileID    Hash
	RootHash, BaseHash, HeadHash, ParentCommitID Hash
	Sequence                                     uint64
	KDFMemory, KDFPasses, KDFParallelism         uint32
	KDFSalt                                      Hash
	LogicalPages                                 [3]Hash
	PageOneCounters                              [6]uint32
}

func validGeneration(g Generation) bool {
	if !g.Authenticated || (g.Label != OLD && g.Label != NEW) || g.Mode != "snapshot" || g.Kind != "RawKey256" ||
		g.Layout != 1 || g.PageSize != 4096 || g.PageCount != 3 || g.DatabaseID == ([16]byte{}) || g.EpochID == (Hash{}) ||
		g.RootSalt == (Hash{}) || g.RootFileID == (Hash{}) || g.BaseFileID == (Hash{}) || g.RootFileID == g.BaseFileID ||
		g.RootHash == (Hash{}) || g.BaseHash == (Hash{}) || g.RootHash == g.BaseHash || g.HeadHash != g.RootHash || g.Sequence == 0 ||
		g.KDFMemory != 0 || g.KDFPasses != 0 || g.KDFParallelism != 0 || g.KDFSalt != (Hash{}) {
		return false
	}
	for _, h := range g.LogicalPages {
		if h == (Hash{}) {
			return false
		}
	}
	return true
}

func ValidateGeneration(old, candidate Generation) Problem {
	if !validGeneration(old) || !validGeneration(candidate) || old.Label != OLD || candidate.Label != NEW || old.Sequence != 4 ||
		old.ParentCommitID == (Hash{}) || candidate.Sequence != 5 || old.DatabaseID != candidate.DatabaseID || old.EpochID == candidate.EpochID || old.RootSalt == candidate.RootSalt ||
		candidate.ParentCommitID != old.RootHash || candidate.LogicalPages != old.LogicalPages || candidate.PageOneCounters != old.PageOneCounters ||
		candidate.RootHash == old.RootHash || candidate.BaseHash == old.BaseHash {
		return ProblemGeneration
	}
	ids := [4]Hash{old.RootFileID, old.BaseFileID, candidate.RootFileID, candidate.BaseFileID}
	for i, id := range ids {
		for j := 0; j < i; j++ {
			if id == ids[j] {
				return ProblemGeneration
			}
		}
	}
	return NoProblem
}

func ObservationPayload(old, candidate Generation) (Hash, Problem) {
	if ValidateGeneration(old, candidate) != NoProblem {
		return Hash{}, ProblemGeneration
	}
	value := [2]Generation{old, candidate}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > 2*MaxFrameBytes {
		return Hash{}, ProblemGeneration
	}
	return sha256.Sum256(raw), NoProblem
}

type SQLType uint8

const (
	SQLTypeUnknown SQLType = iota
	SQLInteger
	SQLText
)

type SQLRow struct {
	IDType    SQLType
	ID        int64
	KeyType   SQLType
	Key       string
	ValueType SQLType
	Value     string
}
type SQLQuery uint8

const (
	SQLAllRows SQLQuery = iota
	SQLBlueIndex
	SQLBlueScan
	SQLRedIndex
	SQLRedScan
	SQLGreenIndex
	SQLGreenScan
	SQLQueryCount
)

type RekeyQueryPlan struct {
	Access, Table, Method, Index, Field, Reason string
	IndexCount, FieldCount                      uint8
	Recheck                                     bool
}
type SQLWork struct{ IndexKeysRead, IndexPageReads, RowIDSeeks, Candidates, BaseRowFetches, ReturnedRows int64 }
type SQLQueryEvidence struct {
	Query                       SQLQuery
	SQL                         string
	Owner, Iterator             uint64
	Rows                        [3]SQLRow
	RowCount                    uint8
	Plan                        RekeyQueryPlan
	Work                        SQLWork
	EOF, ErrCalled              bool
	ErrorToken, CloseErrorToken uint64
	CloseCalls                  uint8
}
type CheckEvidence struct {
	Scope                                                                                                                          string
	Calls                                                                                                                          uint8
	MaxPages, MaxPageReads, Pages, FreePages, ReservedPages, Tables, Indexes, ExtensionIndexes, TableRows, IndexEntries, PageReads uint64
	ErrorToken                                                                                                                     uint64
}
type SQLLedger struct {
	Role       Role
	Generation uint64
	Owner      uint64
	Queries    [7]SQLQueryEvidence
	Check      CheckEvidence
}

func SQLRows() [3]SQLRow {
	return [3]SQLRow{
		{SQLInteger, 1, SQLText, "red", SQLText, "one"},
		{SQLInteger, 2, SQLText, "blue", SQLText, "two"},
		{SQLInteger, 3, SQLText, "red", SQLText, "three"},
	}
}

func RekeyQueryText(q SQLQuery) string {
	switch q {
	case SQLAllRows:
		return "SELECT id,k,v FROM t NOT INDEXED ORDER BY id"
	case SQLBlueIndex:
		return "SELECT id,k,v FROM t WHERE k='blue' ORDER BY id"
	case SQLBlueScan:
		return "SELECT id,k,v FROM t NOT INDEXED WHERE k='blue' ORDER BY id"
	case SQLRedIndex:
		return "SELECT id,k,v FROM t WHERE k='red' ORDER BY id"
	case SQLRedScan:
		return "SELECT id,k,v FROM t NOT INDEXED WHERE k='red' ORDER BY id"
	case SQLGreenIndex:
		return "SELECT id,k,v FROM t WHERE k='green' ORDER BY id"
	case SQLGreenScan:
		return "SELECT id,k,v FROM t NOT INDEXED WHERE k='green' ORDER BY id"
	default:
		return ""
	}
}

// ExpectedSQLLedger exposes literal rows only, not fabricated execution stats.
func ExpectedSQLLedger(q SQLQuery) ([3]SQLRow, uint8) {
	all := SQLRows()
	switch q {
	case SQLAllRows:
		return all, 3
	case SQLBlueIndex, SQLBlueScan:
		return [3]SQLRow{all[1]}, 1
	case SQLRedIndex, SQLRedScan:
		return [3]SQLRow{all[0], all[2]}, 2
	case SQLGreenIndex, SQLGreenScan:
		return [3]SQLRow{}, 0
	default:
		return [3]SQLRow{}, 0
	}
}

func ValidateSQLLedger(l SQLLedger) Problem {
	if l.Role < Controller || l.Role > ArchiveCheck || l.Generation == 0 || l.Owner == 0 {
		return ProblemSQL
	}
	c := l.Check
	if c.Scope != CheckScope || c.Calls != 1 || c.MaxPages != 3 || c.MaxPageReads != 512 || c.Pages != 3 || c.FreePages != 0 ||
		c.ReservedPages != 0 || c.Tables != 1 || c.Indexes != 1 || c.ExtensionIndexes != 0 || c.TableRows != 3 || c.IndexEntries != 3 ||
		c.PageReads == 0 || c.PageReads > 512 || c.ErrorToken != 0 {
		return ProblemSQL
	}
	for i, q := range l.Queries {
		rows, n := ExpectedSQLLedger(SQLQuery(i))
		if q.Query != SQLQuery(i) || q.SQL != RekeyQueryText(q.Query) || q.Owner != l.Owner || q.Iterator == 0 || q.Iterator == l.Owner ||
			q.Rows != rows || q.RowCount != n || !q.EOF || !q.ErrCalled || q.ErrorToken != 0 || q.CloseErrorToken != 0 || q.CloseCalls != 1 {
			return ProblemSQL
		}
		for j := 0; j < i; j++ {
			if q.Iterator == l.Queries[j].Iterator {
				return ProblemSQL
			}
		}
		p, w := q.Plan, q.Work
		if p.Table != "t" || w.ReturnedRows != int64(n) {
			return ProblemSQL
		}
		if q.Query == SQLBlueIndex || q.Query == SQLRedIndex || q.Query == SQLGreenIndex {
			if p.Access != "SEARCH" || p.Method != "btree" || p.Index != "by_k" || p.Field != "k" || p.IndexCount != 1 || p.FieldCount != 1 ||
				!p.Recheck || len(p.Reason) > 1024 || w.IndexPageReads <= 0 || w.IndexPageReads > 32 || w.IndexKeysRead != int64(n) ||
				w.Candidates != int64(n) || w.RowIDSeeks != int64(n) || w.BaseRowFetches != int64(n) {
				return ProblemSQL
			}
		} else if p.Access != "SCAN" || p.Method != "" || p.Index != "" || p.Field != "" || p.IndexCount != 0 || p.FieldCount != 0 || p.Recheck ||
			p.Reason != "NOT INDEXED requested" || w.IndexKeysRead != 0 || w.IndexPageReads != 0 || w.Candidates != 0 || w.RowIDSeeks != 0 || w.BaseRowFetches != 3 {
			return ProblemSQL
		}
	}
	return NoProblem
}

type RekeyReturn uint8

const (
	ReturnUnknown RekeyReturn = iota
	NotReturned
	ReturnedNil
)

type ReturnEvidence struct {
	Calls                               uint8
	Value                               RekeyReturn
	CapturedAdjacent, CapturedUnchanged bool
	ErrorToken                          uint64
}
type ACK struct {
	Present, ParentOwned, Exclusive, ReadbackVerified                                           bool
	ReturnSequence, ACKSequence                                                                 uint16
	OriginalReturnFrame, WrittenHash, ReadbackHash                                              Hash
	File, Directory                                                                             FileIdentity
	Bytes, Written                                                                              uint32
	WriteStep, FileSyncStep, FileCloseStep, DirectorySyncStep, DirectoryCloseStep, ReadbackStep uint16
	FileCloseCalls, DirectoryCloseCalls                                                         uint8
	ErrorToken                                                                                  uint64
}

func ValidateACK(c Case, frames []Frame, a ACK) Problem {
	if c != AcknowledgedExit {
		if a != (ACK{}) {
			return ProblemACK
		}
		return NoProblem
	}
	var returned, acknowledged Frame
	for _, f := range frames {
		if f.Event == PublicRekeyNil {
			if returned.Sequence != 0 {
				return ProblemACK
			}
			returned = f
		}
		if f.Event == AcknowledgementDurable {
			if acknowledged.Sequence != 0 {
				return ProblemACK
			}
			acknowledged = f
		}
	}
	h, p := FrameHash(returned)
	if p != NoProblem || !a.Present || !a.ParentOwned || !a.Exclusive || !a.ReadbackVerified ||
		a.ReturnSequence != returned.Sequence || a.ACKSequence != acknowledged.Sequence || acknowledged.Role != Controller ||
		a.ACKSequence <= a.ReturnSequence || a.OriginalReturnFrame != h || a.WrittenHash != h || a.ReadbackHash != h || acknowledged.Payload != h ||
		!validFileID(a.File) || !validFileID(a.Directory) || a.File == a.Directory || a.Bytes != 4+FrameBodyBytes || a.Written != a.Bytes ||
		a.WriteStep != 1 || a.FileSyncStep != 2 || a.FileCloseStep != 3 || a.DirectorySyncStep != 4 || a.DirectoryCloseStep != 5 || a.ReadbackStep != 6 ||
		a.FileCloseCalls != 1 || a.DirectoryCloseCalls != 1 || a.ErrorToken != 0 {
		return ProblemACK
	}
	return NoProblem
}

type ProcessState uint8

const (
	ProcessUnknown ProcessState = iota
	ProcessExited
	ChildLive
	WaitUnknown
	ProcessSignaled
)

type ProcessReceipt struct {
	Role                                                   Role
	Generation                                             uint64
	Challenge                                              Hash
	ParentHandle                                           uint64
	Starts, Waits                                          uint8
	ParentOwned, ActualSelfImage, NaturalExit, TerminalEOF bool
	Image                                                  FileIdentity
	ImageHash                                              Hash
	State                                                  ProcessState
	ExitCode, SignalNumber                                 int
	TimedOut, Panic, OOM, SpawnedChildren                  bool
}
type RegistryCounts struct{ Groups, Leases, PendingTokens uint64 }
type OwnerReceipt struct {
	Role                                                                Role
	Generation                                                          uint64
	Owner                                                               uint64
	Before, After                                                       RegistryCounts
	AfterKnown                                                          bool
	DBCloseCalls                                                        uint8
	CloseErrorToken                                                     uint64
	UnknownClose, RepeatedRetirement, DeferredCleanup, SecureEraseClaim bool
}

type WrongKeyEvidence struct {
	OriginalError, SQLiteError, Authentication bool
	Operation                                  string
	PublicCode                                 int
	// Any prohibited joined cause, I/O/EOF diagnostic, cleanup, provider error,
	// cancellation, poison or format/limit classification makes this nonzero.
	OtherCauses uint64
}
type ProbeEvidence struct {
	Role                                            Role
	Generation                                      uint64
	Purpose                                         Purpose
	Path                                            string
	OpenCalls                                       uint8
	ReadOnly, NoCreate, NilDB                       bool
	Authenticated                                   bool
	VerifyCalls                                     uint8
	VerifyErrorToken                                uint64
	WrongKey                                        WrongKeyEvidence
	Selected                                        Generation
	Ledger                                          SQLLedger
	AlternateKey, RetainedFallback, Recovery, Retry bool
}

func validateProbe(p ProbeEvidence, paths Paths, expected Generation) Problem {
	purpose, path, label := OldProbeOpen, paths.ProbeOld, OLD
	if p.Role == NewProbe {
		purpose, path, label = NewProbeOpen, paths.ProbeNew, NEW
	} else if p.Role != OldProbe {
		return ProblemGeneration
	}
	if p.Purpose != purpose || p.Path != path || p.OpenCalls != 1 || !p.ReadOnly || !p.NoCreate || p.Generation == 0 ||
		p.AlternateKey || p.RetainedFallback || p.Recovery || p.Retry {
		return ProblemGeneration
	}
	if label == expected.Label {
		if !p.Authenticated || p.NilDB || p.VerifyCalls != 1 || p.VerifyErrorToken != 0 || p.WrongKey != (WrongKeyEvidence{}) || p.Selected != expected ||
			p.Ledger.Role != p.Role || p.Ledger.Generation != p.Generation || ValidateSQLLedger(p.Ledger) != NoProblem {
			return ProblemGeneration
		}
	} else if p.Authenticated || !p.NilDB || p.VerifyCalls != 0 || p.VerifyErrorToken != 0 || p.Selected != (Generation{}) || p.Ledger != (SQLLedger{}) ||
		p.WrongKey != (WrongKeyEvidence{OriginalError: true, SQLiteError: true, Authentication: true, Operation: "open", PublicCode: 26}) {
		return ProblemGeneration
	}
	return NoProblem
}

type Family uint8

const (
	FamilyUnknown Family = iota
	OriginalFamily
	TrialFamily
	PostExitFamily
	OldProbeFamily
	NewProbeFamily
	ArchiveFamily
	RestoreFamily
)

type ArtifactStamp struct {
	Identity            FileIdentity
	Bytes               uint64
	Hash                Hash
	Regular, SingleLink bool
}
type Artifact struct {
	Name          string
	Before, After ArtifactStamp
}
type ArtifactManifest struct {
	Family                                                         Family
	Count                                                          uint8
	Entries                                                        [MaxFamilyEntries]Artifact
	Complete, ParentOwned, ReadbackVerified, FrozenAfterWriterWait bool
	DestinationInitiallyAbsent, ExclusiveCopies, DestinationSynced bool
	Source                                                         Family
	RootName                                                       string
}
type SeedCopyEvidence struct {
	Complete, Exclusive, ReadbackVerified, DestinationAbsent, DestinationSynced bool
	Count                                                                       uint8
	Entries                                                                     [MaxFamilyEntries]Artifact
}

func validFileID(id FileIdentity) bool { return id.Device != 0 && id.Inode != 0 }

func validArtifactName(name, root string, archive bool) bool {
	if archive {
		return name == "old-archive.gse"
	}
	if name == root || name == root+".gse-lock" {
		return true
	}
	prefix := root + ".gse-"
	if len(name) <= len(prefix)+64 || name[:len(prefix)] != prefix {
		return false
	}
	for _, ch := range name[len(prefix) : len(prefix)+64] {
		if !(ch >= '0' && ch <= '9') && !(ch >= 'a' && ch <= 'f') {
			return false
		}
	}
	suffix := name[len(prefix)+64:]
	return suffix == ".base" || suffix == ".retained" || suffix == ".pending"
}

func validateArtifacts(e Evidence, selected Generation) Problem {
	all, ack := e.Artifacts, e.ACK
	want := 5
	if e.ArchiveControl {
		want = 7
	}
	if len(all) != want {
		return ProblemArtifact
	}
	for i, m := range all {
		if m.Family != Family(i+1) || m.Count == 0 || m.Count > MaxFamilyEntries || !m.Complete || !m.ParentOwned || !m.ReadbackVerified ||
			m.RootName != "fixture.gse" || (i > 0 && !m.FrozenAfterWriterWait) {
			return ProblemArtifact
		}
		if m.Family == PostExitFamily || m.Family == OldProbeFamily || m.Family == NewProbeFamily {
			source := TrialFamily
			if m.Family != PostExitFamily {
				source = PostExitFamily
			}
			if m.Source != source || !m.DestinationInitiallyAbsent || !m.ExclusiveCopies || !m.DestinationSynced {
				return ProblemArtifact
			}
			from := all[int(source)-1]
			if m.Count != from.Count {
				return ProblemArtifact
			}
			for j := 0; j < int(m.Count); j++ {
				a, b := m.Entries[j], from.Entries[j]
				if a.Name != b.Name || a.Before.Hash != b.Before.Hash || a.Before.Bytes != b.Before.Bytes {
					return ProblemArtifact
				}
			}
		}
		if m.Family == RestoreFamily && !m.DestinationInitiallyAbsent {
			return ProblemArtifact
		}
		var total uint64
		rootFound := false
		for j, a := range m.Entries {
			if j >= int(m.Count) {
				if a != (Artifact{}) {
					return ProblemArtifact
				}
				continue
			}
			if !validArtifactName(a.Name, m.RootName, m.Family == ArchiveFamily) || (j > 0 && m.Entries[j-1].Name >= a.Name) || a.Before != a.After ||
				!validFileID(a.Before.Identity) || !a.Before.Regular || !a.Before.SingleLink || a.Before.Bytes > MaxFileBytes || a.Before.Hash == (Hash{}) {
				return ProblemArtifact
			}
			if ack.Present && (a.Before.Identity == ack.File || a.Before.Identity == ack.Directory) {
				return ProblemArtifact
			}
			total += a.Before.Bytes
			if a.Name == m.RootName {
				rootFound = true
				root := selected
				if m.Family == OriginalFamily {
					root = e.Old
				}
				if m.Family == RestoreFamily {
					root = e.Archive.Restored
				}
				if a.Before.Hash != root.RootHash || a.Before.Bytes != RootBytes {
					return ProblemArtifact
				}
			}
			if m.Family == ArchiveFamily && (a.Before.Hash != e.Archive.ArchiveHash || a.Before.Bytes != BaseBytes) {
				return ProblemArtifact
			}
			for x := 0; x <= i; x++ {
				for y := 0; y < int(all[x].Count); y++ {
					if x == i && y >= j {
						break
					}
					if a.Before.Identity == all[x].Entries[y].Before.Identity {
						return ProblemArtifact
					}
				}
			}
		}
		if total > MaxFamilyBytes || (m.Family != ArchiveFamily && !rootFound) || (m.Family == ArchiveFamily && m.Count != 1) {
			return ProblemArtifact
		}
	}
	s := e.SeedCopy
	if !s.Complete || !s.Exclusive || !s.ReadbackVerified || !s.DestinationAbsent || !s.DestinationSynced || s.Count != all[0].Count {
		return ProblemArtifact
	}
	for i, a := range s.Entries {
		if i >= int(s.Count) {
			if a != (Artifact{}) {
				return ProblemArtifact
			}
			continue
		}
		source := all[0].Entries[i]
		if a.Name != source.Name || a.Before != a.After || !validFileID(a.Before.Identity) || !a.Before.Regular || !a.Before.SingleLink ||
			a.Before.Hash != source.Before.Hash || a.Before.Bytes != source.Before.Bytes {
			return ProblemArtifact
		}
		for j := 0; j < int(all[0].Count); j++ {
			if a.Before.Identity == all[0].Entries[j].Before.Identity {
				return ProblemArtifact
			}
		}
		for j := 0; j < i; j++ {
			if a.Before.Identity == s.Entries[j].Before.Identity {
				return ProblemArtifact
			}
		}
		if ack.Present && (a.Before.Identity == ack.File || a.Before.Identity == ack.Directory) {
			return ProblemArtifact
		}
	}
	return NoProblem
}

type FileRole uint8

const (
	FileRoleUnknown FileRole = iota
	FileRoot
	FileBase
	FileBackup
)

type RecordRole uint8

const (
	RecordRoleUnknown RecordRole = iota
	RecordMetadata
	RecordData
	RecordFooter
)

type NonceRecord struct {
	Role                     RecordRole
	Offset, Extent, Sequence uint64
	PlaintextBytes           uint32
	Nonce                    [12]byte
}
type NonceObject struct {
	Label                                 Label
	DatabaseID                            [16]byte
	EpochID, RootSalt, FileID, ObjectHash Hash
	Role                                  FileRole
	NoncePrefix                           [4]byte
	Extent                                uint64
	Count                                 uint8
	Records                               [5]NonceRecord
	BoundedParse, WholeObjectHashVerified bool
}

func Nonce(prefix [4]byte, sequence uint64) [12]byte {
	var n [12]byte
	copy(n[:4], prefix[:])
	binary.BigEndian.PutUint64(n[4:], sequence)
	return n
}

// This value ledger is the selected OLD/candidate root/base subset (four
// objects), plus the control's archive/restored root/base (seven). The native
// family's broader historical-object parser is a separately guarded source
// obligation; this function does not purport to validate its older geometry.
// Aliases are repeated object hashes in manifests, not fresh encryption events.
func ValidateNonceLedger(objects []NonceObject) Problem {
	if len(objects) != 4 && len(objects) != 7 {
		return ProblemNonce
	}
	for i, o := range objects {
		if (o.Label != OLD && o.Label != NEW) || o.DatabaseID == ([16]byte{}) || o.EpochID == (Hash{}) || o.RootSalt == (Hash{}) ||
			o.FileID == (Hash{}) || o.ObjectHash == (Hash{}) || !o.BoundedParse || !o.WholeObjectHashVerified {
			return ProblemNonce
		}
		if o.Role == FileRoot {
			if o.Count != 1 || o.Extent != RootBytes {
				return ProblemNonce
			}
		} else if o.Role != FileBase && o.Role != FileBackup {
			return ProblemNonce
		} else if o.Count != 5 || o.Extent != BaseBytes {
			return ProblemNonce
		}
		offset := uint64(512)
		for j, r := range o.Records {
			if j >= int(o.Count) {
				if r != (NonceRecord{}) {
					return ProblemNonce
				}
				continue
			}
			role, plain := RecordMetadata, uint32(64)
			if o.Role == FileRoot {
				plain = 128
			} else if j == 4 {
				role, plain = RecordFooter, 48
			} else if j > 0 {
				role, plain = RecordData, 4096
			}
			if r.Role != role || r.PlaintextBytes != plain || r.Offset != offset || r.Extent != uint64(plain)+40 || r.Sequence != uint64(j) || r.Nonce != Nonce(o.NoncePrefix, r.Sequence) {
				return ProblemNonce
			}
			offset += r.Extent
		}
		if offset != o.Extent {
			return ProblemNonce
		}
		for j := 0; j < i; j++ {
			other := objects[j]
			if o.ObjectHash == other.ObjectHash || o.FileID == other.FileID {
				return ProblemNonce
			}
			if o.Label == other.Label && o.DatabaseID == other.DatabaseID && o.EpochID == other.EpochID && o.RootSalt == other.RootSalt && o.Role == other.Role && o.FileID == other.FileID {
				for a := 0; a < int(o.Count); a++ {
					for b := 0; b < int(other.Count); b++ {
						if o.Records[a].Role == other.Records[b].Role && o.Records[a].Nonce == other.Records[b].Nonce {
							return ProblemNonce
						}
					}
				}
			}
		}
	}
	return NoProblem
}

type ArchiveEvidence struct {
	Present, MadeBeforeRekey, RestoreDestinationAbsent, SourceUnchanged bool
	ArchiveDatabaseID                                                   [16]byte
	ArchiveEpochID, ArchiveRootSalt, ArchiveFileID, ArchiveHash         Hash
	Restored                                                            Generation
	Ledger                                                              SQLLedger
	BackupCalls, RestoreCalls, ReopenCalls, VerifyCalls, CloseCalls     uint8
	ReadOnly, NoCreate                                                  bool
	ErrorToken                                                          uint64
}

type Evidence struct {
	Binding                                            Binding
	Roles                                              [5]RoleBinding
	Paths                                              Paths
	Frames                                             []Frame
	Credentials                                        []CredentialUse
	Old, Candidate                                     Generation
	Baseline                                           SQLLedger
	Return                                             ReturnEvidence
	ACK                                                ACK
	Processes                                          []ProcessReceipt
	Owners                                             []OwnerReceipt
	Probes                                             [2]ProbeEvidence
	Artifacts                                          []ArtifactManifest
	SeedCopy                                           SeedCopyEvidence
	Nonces                                             []NonceObject
	Archive                                            ArchiveEvidence
	ArchiveControl                                     bool
	Filesystem                                         string
	LogicalPageSize                                    uint32
	HardwareSectorClaim, PowerLossClaim, PhysicalClaim bool
}
type Result struct {
	EvidenceConsistent bool
	Problem            Problem
}

func (Result) PhysicalValidated() bool { return false }
func (Result) Pass() bool              { return false }

func Validate(e Evidence) Result {
	if e.PhysicalClaim || e.HardwareSectorClaim || e.PowerLossClaim {
		return Result{Problem: ProblemPhysicalClaim}
	}
	if !validBinding(e.Binding) || len(e.Filesystem) == 0 || len(e.Filesystem) > 128 || e.LogicalPageSize != 4096 {
		return Result{Problem: ProblemBinding}
	}
	c := e.Binding.Case
	if e.ArchiveControl && c != Control {
		return Result{Problem: ProblemArchive}
	}
	roleCount := 4
	if e.ArchiveControl {
		roleCount = 5
	}
	for i, r := range e.Roles {
		if i >= roleCount {
			if r != (RoleBinding{}) {
				return Result{Problem: ProblemBinding}
			}
			continue
		}
		if r.Role != Role(i+1) || r.Generation == 0 || r.Challenge == (Hash{}) {
			return Result{Problem: ProblemBinding}
		}
		for j := 0; j < i; j++ {
			if r.Generation == e.Roles[j].Generation || r.Challenge == e.Roles[j].Challenge {
				return Result{Problem: ProblemBinding}
			}
		}
	}
	trace := ExpectedTraceWithArchive(c, e.ArchiveControl)
	if len(e.Frames) != len(trace) || len(e.Frames) > MaxFrames {
		return Result{Problem: ProblemTrace}
	}
	observation, p := ObservationPayload(e.Old, e.Candidate)
	if p != NoProblem {
		return Result{Problem: p}
	}
	for i, f := range e.Frames {
		if _, p := EncodeFrame(f); p != NoProblem {
			return Result{Problem: p}
		}
		if f.Binding != e.Binding || f.Sequence != uint16(i+1) || f.Event != trace[i] {
			return Result{Problem: ProblemTrace}
		}
		r := e.Roles[int(f.Role)-1]
		if f.Generation != r.Generation || f.Challenge != r.Challenge {
			return Result{Problem: ProblemBinding}
		}
		if (f.Event == ObservedBeforeRename || f.Event == ObservedAfterRename || f.Event == ObservedAfterDirSync || f.Event == PublicRekeyNil) && f.Payload != observation {
			return Result{Problem: ProblemGeneration}
		}
	}
	if p := ValidatePaths(e.Paths); p != NoProblem {
		return Result{Problem: p}
	}
	paths := [6]string{e.Paths.Original, e.Paths.Trial, e.Paths.ProbeOld, e.Paths.ProbeNew, e.Paths.Archive, e.Paths.Restore}
	for i, p := range paths {
		if !validPath(p) {
			return Result{Problem: ProblemCredential}
		}
		for j := 0; j < i; j++ {
			if p == paths[j] {
				return Result{Problem: ProblemCredential}
			}
		}
	}
	credentialCount := 5
	if e.ArchiveControl {
		credentialCount = 9
	}
	if len(e.Credentials) != credentialCount {
		return Result{Problem: ProblemCredential}
	}
	for i, u := range e.Credentials {
		if u.Spec.Purpose != Purpose(i+1) || ValidateCredentialUse(e.Paths, u) != NoProblem {
			return Result{Problem: ProblemCredential}
		}
		for j := 0; j < i; j++ {
			if u.Provider == e.Credentials[j].Provider {
				return Result{Problem: ProblemCredential}
			}
		}
	}
	if p := ValidateGeneration(e.Old, e.Candidate); p != NoProblem {
		return Result{Problem: p}
	}
	if e.Baseline.Role != Controller || e.Baseline.Generation != e.Roles[0].Generation || ValidateSQLLedger(e.Baseline) != NoProblem {
		return Result{Problem: ProblemSQL}
	}
	wantReturn := ReturnEvidence{Calls: 1, Value: NotReturned}
	if c == Control || c == RekeyReturnedNoAck || c == AcknowledgedExit {
		wantReturn = ReturnEvidence{Calls: 1, Value: ReturnedNil, CapturedAdjacent: true, CapturedUnchanged: true}
	}
	if e.Return != wantReturn {
		return Result{Problem: ProblemTrace}
	}
	if p := ValidateACK(c, e.Frames, e.ACK); p != NoProblem {
		return Result{Problem: p}
	}
	if p := validateProcessesAndOwners(e, roleCount); p != NoProblem {
		return Result{Problem: p}
	}
	selected := e.Candidate
	if c == BeforeRootRename {
		selected = e.Old
	}
	for i, p := range e.Probes {
		if p.Role != Role(int(OldProbe)+i) || p.Generation != e.Roles[int(p.Role)-1].Generation || validateProbe(p, e.Paths, selected) != NoProblem {
			return Result{Problem: ProblemGeneration}
		}
	}
	if p := validateArtifacts(e, selected); p != NoProblem {
		return Result{Problem: p}
	}
	if p := validateArchive(e); p != NoProblem {
		return Result{Problem: p}
	}
	if p := validateBoundNonces(e); p != NoProblem {
		return Result{Problem: p}
	}
	return Result{EvidenceConsistent: true}
}

func validateProcessesAndOwners(e Evidence, roleCount int) Problem {
	if len(e.Processes) != roleCount-1 || len(e.Owners) != roleCount {
		return ProblemProcess
	}
	for i, p := range e.Processes {
		r := e.Roles[i+1]
		exit, natural := 0, true
		if p.Role == Writer && e.Binding.Case != Control {
			exit, natural = ExpectedExitCode, false
		}
		if p.Role != Role(i+2) || p.Generation != r.Generation || p.Challenge != r.Challenge || p.ParentHandle == 0 || p.Starts != 1 || p.Waits != 1 ||
			!p.ParentOwned || !p.ActualSelfImage || !validFileID(p.Image) || p.ImageHash != e.Binding.Binary || p.State != ProcessExited || p.ExitCode != exit ||
			p.NaturalExit != natural || !p.TerminalEOF || p.SignalNumber != 0 || p.TimedOut || p.Panic || p.OOM || p.SpawnedChildren {
			return ProblemProcess
		}
		for j := 0; j < i; j++ {
			if p.ParentHandle == e.Processes[j].ParentHandle {
				return ProblemProcess
			}
		}
	}
	for i, o := range e.Owners {
		r := e.Roles[i]
		if o.Role != r.Role || o.Generation != r.Generation || o.Owner == 0 || o.Before != (RegistryCounts{}) || o.CloseErrorToken != 0 ||
			o.UnknownClose || o.RepeatedRetirement || o.SecureEraseClaim {
			return ProblemOwner
		}
		for j := 0; j < i; j++ {
			if o.Owner == e.Owners[j].Owner {
				return ProblemOwner
			}
		}
		if o.Role == Writer && e.Binding.Case != Control {
			if o.AfterKnown || o.After != (RegistryCounts{}) || o.DBCloseCalls != 0 || o.DeferredCleanup {
				return ProblemOwner
			}
		} else {
			closes := uint8(1)
			if (o.Role == OldProbe && e.Binding.Case != BeforeRootRename) || (o.Role == NewProbe && e.Binding.Case == BeforeRootRename) {
				closes = 0
			}
			if !o.AfterKnown || o.After != (RegistryCounts{}) || o.DBCloseCalls != closes || o.DeferredCleanup {
				return ProblemOwner
			}
		}
	}
	ledgers := [4]SQLLedger{e.Baseline, e.Probes[0].Ledger, e.Probes[1].Ledger, e.Archive.Ledger}
	for i, l := range ledgers {
		if l.Owner == 0 {
			continue
		}
		if l.Role < Controller || int(l.Role) > len(e.Owners) {
			return ProblemOwner
		}
		if l.Owner != e.Owners[int(l.Role)-1].Owner {
			return ProblemOwner
		}
		for _, q := range l.Queries {
			for _, o := range e.Owners {
				if q.Iterator == o.Owner {
					return ProblemOwner
				}
			}
		}
		for j := 0; j < i; j++ {
			if ledgers[j].Owner == 0 {
				continue
			}
			if l.Owner == ledgers[j].Owner {
				return ProblemOwner
			}
			for _, a := range l.Queries {
				for _, b := range ledgers[j].Queries {
					if a.Iterator == b.Iterator {
						return ProblemOwner
					}
				}
			}
		}
	}
	return NoProblem
}

func validateArchive(e Evidence) Problem {
	a := e.Archive
	if !e.ArchiveControl {
		if a != (ArchiveEvidence{}) {
			return ProblemArchive
		}
		return NoProblem
	}
	if !a.Present || !a.MadeBeforeRekey || !a.RestoreDestinationAbsent || !a.SourceUnchanged || a.ArchiveDatabaseID == ([16]byte{}) ||
		a.ArchiveDatabaseID == e.Old.DatabaseID || a.ArchiveEpochID == (Hash{}) || a.ArchiveEpochID == e.Old.EpochID || a.ArchiveEpochID == e.Candidate.EpochID ||
		a.ArchiveRootSalt == (Hash{}) || a.ArchiveRootSalt == e.Old.RootSalt || a.ArchiveRootSalt == e.Candidate.RootSalt || a.ArchiveFileID == (Hash{}) || a.ArchiveHash == (Hash{}) ||
		!validGeneration(a.Restored) || a.Restored.Label != NEW || a.Restored.DatabaseID == e.Old.DatabaseID || a.Restored.DatabaseID == a.ArchiveDatabaseID ||
		a.Restored.EpochID == e.Old.EpochID || a.Restored.EpochID == e.Candidate.EpochID || a.Restored.EpochID == a.ArchiveEpochID ||
		a.Restored.RootSalt == e.Old.RootSalt || a.Restored.RootSalt == e.Candidate.RootSalt || a.Restored.RootSalt == a.ArchiveRootSalt ||
		a.Restored.Sequence != 1 || a.Restored.ParentCommitID != (Hash{}) || a.Restored.LogicalPages != e.Old.LogicalPages || a.Restored.PageOneCounters != e.Old.PageOneCounters ||
		a.Ledger.Role != ArchiveCheck || a.Ledger.Generation != e.Roles[4].Generation || ValidateSQLLedger(a.Ledger) != NoProblem ||
		a.BackupCalls != 1 || a.RestoreCalls != 1 || a.ReopenCalls != 1 || a.VerifyCalls != 1 || a.CloseCalls != 1 || !a.ReadOnly || !a.NoCreate || a.ErrorToken != 0 {
		return ProblemArchive
	}
	return NoProblem
}

func validateBoundNonces(e Evidence) Problem {
	want := 4
	if e.ArchiveControl {
		want = 7
	}
	if len(e.Nonces) != want || ValidateNonceLedger(e.Nonces) != NoProblem {
		return ProblemNonce
	}
	for i, o := range e.Nonces {
		g, role := e.Old, FileRoot
		switch i {
		case 1:
			role = FileBase
		case 2:
			g = e.Candidate
		case 3:
			g, role = e.Candidate, FileBase
		case 4:
			a := e.Archive
			if o.Label != OLD || o.DatabaseID != a.ArchiveDatabaseID || o.EpochID != a.ArchiveEpochID || o.RootSalt != a.ArchiveRootSalt ||
				o.FileID != a.ArchiveFileID || o.ObjectHash != a.ArchiveHash || o.Role != FileBackup {
				return ProblemNonce
			}
			continue
		case 5:
			g = e.Archive.Restored
		case 6:
			g, role = e.Archive.Restored, FileBase
		}
		id, hash := g.RootFileID, g.RootHash
		if role == FileBase {
			id, hash = g.BaseFileID, g.BaseHash
		}
		if o.Label != g.Label || o.DatabaseID != g.DatabaseID || o.EpochID != g.EpochID || o.RootSalt != g.RootSalt || o.FileID != id || o.ObjectHash != hash || o.Role != role {
			return ProblemNonce
		}
	}
	for _, o := range e.Nonces {
		found := false
		for _, m := range e.Artifacts {
			for j := 0; j < int(m.Count); j++ {
				if m.Entries[j].Before.Hash == o.ObjectHash && m.Entries[j].Before.Bytes == o.Extent {
					found = true
				}
			}
		}
		if !found {
			return ProblemNonce
		}
	}
	return NoProblem
}
