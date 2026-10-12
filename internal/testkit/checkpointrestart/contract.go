// Package checkpointrestart models one closed ten-case source contract. It has
// no database, filesystem, descriptor, process, callback, or native capability.
// Successful validation only checks supplied values; it never establishes that
// a checkpoint, Sync, Wait, reopen, or physical persistence experiment occurred.
package checkpointrestart

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

const (
	SchemaVersion    = 1
	FixtureID        = "checkpoint-extensions-315-v1"
	ProposalSHA256   = "74ba6412f77953322b908b49b4e47912b76fa5dfcf265f3001d90bd3d05352c1"
	MaxFrames        = 64
	MaxFrameBytes    = 4096
	MaxCopies        = 4096
	MaxPages         = 4096
	MaxFileBytes     = 32 << 20
	MaxFamilyBytes   = 64 << 20
	MaxFamilyEntries = 64
	ExpectedExitCode = 86
	AbortExitCode    = 87
)

type Hash = [32]byte
type Case uint8

const (
	CaseUnknown Case = iota
	ExclusiveControl
	ExclusiveAfterFirstMainCopy
	ExclusiveAfterWALUnlink
	SharedControl
	SharedAfterFirstMainCopy
	SharedAfterBackfill
	EncryptedControl
	EncryptedBeforeRootRename
	EncryptedAfterRootRename
	EncryptedAfterRootDirSync
)

func Cases() [10]Case {
	return [10]Case{ExclusiveControl, ExclusiveAfterFirstMainCopy, ExclusiveAfterWALUnlink, SharedControl, SharedAfterFirstMainCopy, SharedAfterBackfill, EncryptedControl, EncryptedBeforeRootRename, EncryptedAfterRootRename, EncryptedAfterRootDirSync}
}
func CaseName(c Case) string {
	names := [11]string{"", "exclusive-control", "exclusive-after-first-main-copy", "exclusive-after-wal-unlink", "shared-control", "shared-after-first-main-copy", "shared-after-backfill", "encrypted-control", "encrypted-before-root-rename", "encrypted-after-root-rename", "encrypted-after-root-dirsync"}
	if int(c) >= len(names) {
		return ""
	}
	return names[c]
}
func ParseCase(s string) (Case, bool) {
	for _, c := range Cases() {
		if CaseName(c) == s {
			return c, true
		}
	}
	return CaseUnknown, false
}

type Profile uint8

const (
	ProfileUnknown Profile = iota
	PlainExclusive4096
	PlainShared4096
	EncryptedLayout2Raw4096
)

func Profiles() [3]Profile {
	return [3]Profile{PlainExclusive4096, PlainShared4096, EncryptedLayout2Raw4096}
}
func CaseProfile(c Case) Profile {
	switch c {
	case ExclusiveControl, ExclusiveAfterFirstMainCopy, ExclusiveAfterWALUnlink:
		return PlainExclusive4096
	case SharedControl, SharedAfterFirstMainCopy, SharedAfterBackfill:
		return PlainShared4096
	case EncryptedControl, EncryptedBeforeRootRename, EncryptedAfterRootRename, EncryptedAfterRootDirSync:
		return EncryptedLayout2Raw4096
	}
	return ProfileUnknown
}
func ProfileName(p Profile) string {
	switch p {
	case PlainExclusive4096:
		return "linux-plain-exclusive-4096"
	case PlainShared4096:
		return "linux-plain-shared-4096"
	case EncryptedLayout2Raw4096:
		return "linux-encrypted-layout2-rawkey256-4096"
	}
	return ""
}
func IsControl(c Case) bool {
	return c == ExclusiveControl || c == SharedControl || c == EncryptedControl
}

type Point uint8

const (
	PointUnknown Point = iota
	ExclusiveFirstCopy
	ExclusiveWALUnlinked
	SharedFirstCopy
	SharedBackfillSet
	BeforeRootRename
	AfterRootRename
	AfterRootDirSync
)

func TargetPoint(c Case) Point {
	switch c {
	case ExclusiveAfterFirstMainCopy:
		return ExclusiveFirstCopy
	case ExclusiveAfterWALUnlink:
		return ExclusiveWALUnlinked
	case SharedAfterFirstMainCopy:
		return SharedFirstCopy
	case SharedAfterBackfill:
		return SharedBackfillSet
	case EncryptedBeforeRootRename:
		return BeforeRootRename
	case EncryptedAfterRootRename:
		return AfterRootRename
	case EncryptedAfterRootDirSync:
		return AfterRootDirSync
	}
	return PointUnknown
}
func ExpectedPoints(c Case) []Point {
	var p []Point
	switch CaseProfile(c) {
	case PlainExclusive4096:
		p = []Point{ExclusiveFirstCopy, ExclusiveWALUnlinked}
	case PlainShared4096:
		p = []Point{SharedFirstCopy, SharedBackfillSet}
	case EncryptedLayout2Raw4096:
		p = []Point{BeforeRootRename, AfterRootRename, AfterRootDirSync}
	default:
		return nil
	}
	for i, q := range p {
		if q == TargetPoint(c) {
			return p[:i+1]
		}
	}
	return p
}

type Problem string

const (
	NoProblem            Problem = ""
	ProblemBinding       Problem = "binding"
	ProblemFrame         Problem = "frame"
	ProblemTrace         Problem = "trace"
	ProblemHead          Problem = "committed-head"
	ProblemSQL           Problem = "complete-extension-ledger"
	ProblemReceipt       Problem = "commit-causality"
	ProblemOwner         Problem = "owner-retirement"
	ProblemProcess       Problem = "actual-owned-wait"
	ProblemFamily        Problem = "private-family-preservation"
	ProblemPhysicalClaim Problem = "physical-claim-forbidden"
)

type Binding struct {
	Schema         uint16  `json:"schema"`
	Case           Case    `json:"case"`
	Profile        Profile `json:"profile"`
	Trial          Hash    `json:"trial"`
	SourceManifest Hash    `json:"source_manifest"`
	Compiler       Hash    `json:"compiler"`
	CompileReceipt Hash    `json:"compile_receipt"`
	Binary         Hash    `json:"binary"`
	Fixture        Hash    `json:"fixture"`
}

func FixtureHash() Hash { return sha256.Sum256([]byte(FixtureID)) }
func ValidateBinding(b Binding) Problem {
	if b.Schema != SchemaVersion || CaseName(b.Case) == "" || b.Profile != CaseProfile(b.Case) || b.Trial == (Hash{}) || b.SourceManifest == (Hash{}) || b.Compiler == (Hash{}) || b.CompileReceipt == (Hash{}) || b.Binary == (Hash{}) || b.Fixture != FixtureHash() {
		return ProblemBinding
	}
	return NoProblem
}

type Role uint8

const (
	RoleUnknown Role = iota
	Controller
	Writer
	Probe
)

type RoleBinding struct {
	Role       Role   `json:"role"`
	Generation uint64 `json:"generation"`
	Challenge  Hash   `json:"challenge"`
}
type KDFParams struct {
	MemoryKiB   uint32 `json:"memory_kib"`
	Passes      uint32 `json:"passes"`
	Parallelism uint8  `json:"parallelism"`
}
type Epoch struct {
	DatabaseID [16]byte  `json:"database_id"`
	EpochID    Hash      `json:"epoch_id"`
	Kind       uint8     `json:"kind"`
	KDF        KDFParams `json:"kdf"`
	KDFSalt    Hash      `json:"kdf_salt"`
	RootSalt   Hash      `json:"root_salt"`
}
type Descriptor struct {
	Role           uint8  `json:"role"`
	PageSize       uint32 `json:"page_size"`
	PageCount      uint32 `json:"page_count"`
	RecordCount    uint64 `json:"record_count"`
	CommitSequence uint64 `json:"commit_sequence"`
	TransactionID  Hash   `json:"transaction_id"`
	ParentCommitID Hash   `json:"parent_commit_id"`
}
type RootHeader struct {
	Major       uint16     `json:"major"`
	Minor       uint16     `json:"minor"`
	Suite       uint16     `json:"suite"`
	Flags       uint32     `json:"flags"`
	Epoch       Epoch      `json:"epoch"`
	FileID      Hash       `json:"file_id"`
	NoncePrefix [4]byte    `json:"nonce_prefix"`
	Descriptor  Descriptor `json:"descriptor"`
}
type RootCandidate struct {
	RootHash  Hash       `json:"root_hash"`
	RootBytes uint64     `json:"root_bytes"`
	Header    RootHeader `json:"header"`
}
type EncryptedHead struct {
	Authenticated bool       `json:"authenticated"`
	RootHash      Hash       `json:"root_hash"`
	RootBytes     uint64     `json:"root_bytes"`
	Header        RootHeader `json:"header"`
	Layout        uint32     `json:"layout"`
	HeadCommitID  Hash       `json:"head_commit_id"`
	BaseFileID    Hash       `json:"base_file_id"`
	WALSegments   uint32     `json:"wal_segments"`
	WALFrames     uint64     `json:"wal_frames"`
	WALBytes      uint64     `json:"wal_bytes"`
}
type SelectedPage struct {
	Page  uint32 `json:"page"`
	Frame uint32 `json:"frame"`
	Hash  Hash   `json:"hash"`
}
type PlainHead struct {
	PageSize   uint32         `json:"page_size"`
	DBSize     uint32         `json:"db_size"`
	FrameCount uint32         `json:"frame_count"`
	EndOffset  uint64         `json:"end_offset"`
	Salt       [2]uint32      `json:"salt"`
	Checksum   [2]uint32      `json:"checksum"`
	MainHash   Hash           `json:"main_hash"`
	WALHash    Hash           `json:"wal_hash"`
	WALBytes   uint64         `json:"wal_bytes"`
	Complete   bool           `json:"complete"`
	CleanTail  bool           `json:"clean_tail"`
	Selected   []SelectedPage `json:"selected"`
}
type Observation struct {
	Point             Point      `json:"point"`
	Page              uint32     `json:"page"`
	SelectedCount     uint32     `json:"selected_count"`
	CopiedCount       uint32     `json:"copied_count"`
	DBSize            uint32     `json:"db_size"`
	FrameCount        uint32     `json:"frame_count"`
	Salt              [2]uint32  `json:"salt"`
	Checksum          [2]uint32  `json:"checksum"`
	Safe              uint32     `json:"safe"`
	OldBackfill       uint32     `json:"old_backfill"`
	Attempted         uint32     `json:"attempted"`
	Backfill          uint32     `json:"backfill"`
	OldRootHash       Hash       `json:"old_root_hash"`
	CandidateRootHash Hash       `json:"candidate_root_hash"`
	OldHeader         RootHeader `json:"old_header"`
	CandidateHeader   RootHeader `json:"candidate_header"`
	CandidateBytes    uint64     `json:"candidate_bytes"`
}

func validRoot(h RootHeader) bool {
	e, d := h.Epoch, h.Descriptor
	return h.Major == 1 && h.Minor == 0 && h.Suite == 1 && h.Flags == 0 && e.DatabaseID != ([16]byte{}) && e.EpochID != (Hash{}) && e.Kind == 1 && e.KDF == (KDFParams{}) && e.KDFSalt == (Hash{}) && e.RootSalt != (Hash{}) && h.FileID != (Hash{}) && d.Role == 1 && d.PageSize == 4096 && d.PageCount > 0 && d.PageCount <= MaxPages && d.RecordCount == 1 && d.TransactionID != (Hash{})
}
func validEncryptedHead(h EncryptedHead) bool {
	return h.Authenticated && h.RootHash != (Hash{}) && h.RootBytes >= 680 && h.RootBytes <= MaxFileBytes && validRoot(h.Header) && h.Layout == 2 && h.HeadCommitID != (Hash{}) && h.BaseFileID != (Hash{}) && h.WALSegments <= 16 && h.WALFrames <= 4096 && h.WALBytes <= MaxFileBytes
}
func ValidateEncryptedTransition(old EncryptedHead, candidate RootCandidate) Problem {
	if !validEncryptedHead(old) || old.Header.Descriptor.CommitSequence != 4 || old.WALSegments != 1 || old.WALFrames == 0 || old.WALBytes == 0 || old.RootBytes != 1320 || !validRoot(candidate.Header) || candidate.RootHash == (Hash{}) || candidate.RootBytes != 1064 || candidate.Header.Descriptor.CommitSequence != 5 || old.Header.Epoch != candidate.Header.Epoch || old.Header.Descriptor.PageSize != candidate.Header.Descriptor.PageSize || old.Header.Descriptor.PageCount != candidate.Header.Descriptor.PageCount || candidate.Header.Descriptor.ParentCommitID != old.HeadCommitID || old.RootHash == candidate.RootHash || old.Header.FileID == candidate.Header.FileID {
		return ProblemHead
	}
	return NoProblem
}
func ValidatePlainHead(h PlainHead) Problem {
	if h.PageSize != 4096 || h.DBSize == 0 || h.DBSize > MaxPages || h.FrameCount == 0 || h.FrameCount > 4096 || h.EndOffset != 32+uint64(h.FrameCount)*(4096+24) || h.WALBytes != h.EndOffset || h.WALBytes > MaxFileBytes || h.MainHash == (Hash{}) || h.WALHash == (Hash{}) || !h.Complete || !h.CleanTail || len(h.Selected) < 2 || len(h.Selected) > MaxCopies || len(h.Selected) > int(h.DBSize) {
		return ProblemHead
	}
	for i, p := range h.Selected {
		if p.Page == 0 || p.Page > h.DBSize || p.Frame == 0 || p.Frame > h.FrameCount || p.Hash == (Hash{}) || i > 0 && h.Selected[i-1].Page >= p.Page {
			return ProblemHead
		}
	}
	return NoProblem
}
func canonicalHash(v any) (Hash, Problem) {
	b, err := json.Marshal(v)
	if err != nil || len(b) > MaxLedgerBytes {
		return Hash{}, ProblemFrame
	}
	return sha256.Sum256(b), NoProblem
}

type Event uint8

const (
	EventUnknown Event = iota
	ParentAdmitted
	SeedCommitNil
	SeedLedgerChecked
	CommittedReceiptDurable
	SeedClosed
	TrialCopied
	WriterStarted
	WriterReady
	WriterOpenGranted
	CheckpointReady
	CheckpointGranted
	CutObserved
	CutContinued
	ExitGranted
	PublicCheckpointNil
	WriterClosed
	WriterResult
	WriterDoneGranted
	WriterWaited
	EvidenceFrozen
	ProbeCopied
	ProbeStarted
	ProbeReady
	ProbeOpenGranted
	ProbeResult
	ProbeWaited
	ArtifactsRechecked
	EvidenceComplete
)

func eventRole(e Event) Role {
	switch e {
	case WriterReady, CheckpointReady, CutObserved, PublicCheckpointNil, WriterClosed, WriterResult:
		return Writer
	case ProbeReady, ProbeResult:
		return Probe
	default:
		if e > EventUnknown && e <= EvidenceComplete {
			return Controller
		}
		return RoleUnknown
	}
}
func ExpectedTrace(c Case) []Event {
	if CaseName(c) == "" {
		return nil
	}
	t := []Event{ParentAdmitted, SeedCommitNil, SeedLedgerChecked, CommittedReceiptDurable, SeedClosed, TrialCopied, WriterStarted, WriterReady, WriterOpenGranted, CheckpointReady, CheckpointGranted}
	for _, p := range ExpectedPoints(c) {
		t = append(t, CutObserved)
		if p == TargetPoint(c) {
			t = append(t, ExitGranted)
		} else {
			t = append(t, CutContinued)
		}
	}
	if IsControl(c) {
		t = append(t, PublicCheckpointNil, WriterClosed, WriterResult, WriterDoneGranted)
	}
	return append(t, WriterWaited, EvidenceFrozen, ProbeCopied, ProbeStarted, ProbeReady, ProbeOpenGranted, ProbeResult, ProbeWaited, ArtifactsRechecked, EvidenceComplete)
}

type Frame struct {
	Binding    Binding `json:"binding"`
	Role       Role    `json:"role"`
	Generation uint64  `json:"generation"`
	Challenge  Hash    `json:"challenge"`
	Sequence   uint16  `json:"sequence"`
	Event      Event   `json:"event"`
	Point      Point   `json:"point"`
	Payload    Hash    `json:"payload"`
	Echo       Hash    `json:"echo"`
}

func EncodeFrame(f Frame) ([]byte, Problem) {
	if ValidateBinding(f.Binding) != NoProblem || f.Role != eventRole(f.Event) || f.Generation == 0 || f.Challenge == (Hash{}) || f.Sequence == 0 || f.Sequence > MaxFrames || f.Payload == (Hash{}) {
		return nil, ProblemFrame
	}
	b, err := json.Marshal(f)
	if err != nil || len(b) > MaxFrameBytes {
		return nil, ProblemFrame
	}
	return b, NoProblem
}
func DecodeFrame(b []byte) (Frame, Problem) {
	var f Frame
	if !decodeCanonical(b, &f, MaxFrameBytes) {
		return Frame{}, ProblemFrame
	}
	if _, p := EncodeFrame(f); p != NoProblem {
		return Frame{}, p
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

type Paths struct {
	Run      string `json:"run"`
	Original string `json:"original"`
	Trial    string `json:"trial"`
	PostExit string `json:"post_exit"`
	Probe    string `json:"probe"`
	Control  string `json:"control"`
}

func canonicalPath(s string) bool {
	return len(s) > 1 && len(s) <= 1024 && s[0] == '/' && s[len(s)-1] != '/' && !strings.Contains(s, "\\") && !strings.Contains(s, "\x00") && !strings.Contains(s, "//") && !strings.Contains(s, "/../") && !strings.Contains(s, "/./") && !strings.HasSuffix(s, "/..") && !strings.HasSuffix(s, "/.")
}
func ValidatePaths(p Paths, profile Profile) Problem {
	if !canonicalPath(p.Run) {
		return ProblemBinding
	}
	name := "fixture.db"
	if profile == EncryptedLayout2Raw4096 {
		name = "fixture.gse"
	}
	if p.Original != p.Run+"/original/"+name || p.Trial != p.Run+"/trial/"+name || p.PostExit != p.Run+"/evidence/post-exit/"+name || p.Probe != p.Run+"/probe/"+name || p.Control != p.Run+"/control" {
		return ProblemBinding
	}
	return NoProblem
}

type FileIdentity struct {
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

func validIdentity(i FileIdentity) bool { return i.Device != 0 && i.Inode != 0 }

type CommitReceipt struct {
	Binding             Binding      `json:"binding"`
	Present             bool         `json:"present"`
	CommitCalls         uint8        `json:"commit_calls"`
	CommitReturnedNil   bool         `json:"commit_returned_nil"`
	CapturedAdjacent    bool         `json:"captured_adjacent"`
	ActualCommitOrigin  bool         `json:"actual_commit_origin"`
	BeforeCheckpoint    bool         `json:"before_checkpoint"`
	LedgerDigest        Hash         `json:"ledger_digest"`
	HeadDigest          Hash         `json:"head_digest"`
	File                FileIdentity `json:"file"`
	Directory           FileIdentity `json:"directory"`
	Hash                Hash         `json:"hash"`
	FullWrite           bool         `json:"full_write"`
	FileSynced          bool         `json:"file_synced"`
	FileCloseCalls      uint8        `json:"file_close_calls"`
	DirectorySynced     bool         `json:"directory_synced"`
	DirectoryCloseCalls uint8        `json:"directory_close_calls"`
	ReadbackVerified    bool         `json:"readback_verified"`
	ExclusiveCreated    bool         `json:"exclusive_created"`
	ErrorToken          uint64       `json:"error_token"`
	CheckpointACK       bool         `json:"checkpoint_ack"`
}
type ReturnEvidence struct {
	Calls            uint8  `json:"calls"`
	Returned         bool   `json:"returned"`
	Nil              bool   `json:"nil"`
	CapturedAdjacent bool   `json:"captured_adjacent"`
	ErrorToken       uint64 `json:"error_token"`
}
type ProcessState uint8

const (
	ProcessUnknown ProcessState = iota
	ProcessPending
	ProcessExited
)

type ProcessReceipt struct {
	Role                Role         `json:"role"`
	Generation          uint64       `json:"generation"`
	Challenge           Hash         `json:"challenge"`
	Owner               uint64       `json:"owner"`
	Starts              uint8        `json:"starts"`
	Waits               uint8        `json:"waits"`
	ParentOwned         bool         `json:"parent_owned"`
	ActualSelfImage     bool         `json:"actual_self_image"`
	Image               FileIdentity `json:"image"`
	ImageHash           Hash         `json:"image_hash"`
	State               ProcessState `json:"state"`
	ActualWait          bool         `json:"actual_wait"`
	ProcessStatePresent bool         `json:"process_state_present"`
	ExitCode            int          `json:"exit_code"`
	SignalNumber        int          `json:"signal_number"`
	NaturalExit         bool         `json:"natural_exit"`
	OriginalExitError   bool         `json:"original_exit_error"`
	WaitErrorToken      uint64       `json:"wait_error_token"`
	TerminalEOF         bool         `json:"terminal_eof"`
	DiagnosticStall     bool         `json:"diagnostic_stall"`
	TransportErrorToken uint64       `json:"transport_error_token"`
	SpawnedChildren     bool         `json:"spawned_children"`
	Panic               bool         `json:"panic"`
	OOM                 bool         `json:"oom"`
	ReleasedBeforeWait  bool         `json:"released_before_wait"`
}

// NeedsUnresolvedHold never invents a close, second Wait or signal capability.
// Late actual disposition remains failure when a prior transport/stall failed.
func NeedsUnresolvedHold(p ProcessReceipt) bool {
	return p.Starts == 1 && (!p.ActualWait || !p.ProcessStatePresent || p.State != ProcessExited)
}

type RegistryCounts struct {
	Plaintext uint32 `json:"plaintext"`
	Close     uint32 `json:"close"`
	Encrypted uint32 `json:"encrypted"`
}
type OwnerReceipt struct {
	Role                   Role           `json:"role"`
	Generation             uint64         `json:"generation"`
	Owner                  uint64         `json:"owner"`
	Before                 RegistryCounts `json:"before"`
	After                  RegistryCounts `json:"after"`
	AfterKnown             bool           `json:"after_known"`
	DBCloseCalls           uint8          `json:"db_close_calls"`
	CloseErrorToken        uint64         `json:"close_error_token"`
	UnknownClose           bool           `json:"unknown_close"`
	RepeatedRetirement     bool           `json:"repeated_retirement"`
	DeferredCleanup        bool           `json:"deferred_cleanup"`
	SecureEraseClaim       bool           `json:"secure_erase_claim"`
	DependentOwnersRetired bool           `json:"dependent_owners_retired"`
}
type ResourceReceipt struct {
	Owner                    uint64 `json:"owner"`
	Parent                   uint64 `json:"parent"`
	Kind                     string `json:"kind"`
	CapturedBeforeError      bool   `json:"captured_before_error"`
	CloseCalls               uint8  `json:"close_calls"`
	ErrorToken               uint64 `json:"error_token"`
	Closed                   bool   `json:"closed"`
	Unknown                  bool   `json:"unknown"`
	ChildExited              bool   `json:"child_exited"`
	ReleasedBeforeDependency bool   `json:"released_before_dependency"`
}
type CredentialReceipt struct {
	Role           Role   `json:"role"`
	Provider       uint64 `json:"provider"`
	Path           string `json:"path"`
	Operation      uint8  `json:"operation"`
	Kind           uint8  `json:"kind"`
	Acquires       uint8  `json:"acquires"`
	Releases       uint8  `json:"releases"`
	Cleared        bool   `json:"cleared"`
	PurposeChecked bool   `json:"purpose_checked"`
	ErrorToken     uint64 `json:"error_token"`
}

type FileState struct {
	Identity    FileIdentity `json:"identity"`
	Hash        Hash         `json:"hash"`
	Bytes       uint64       `json:"bytes"`
	Mode        uint32       `json:"mode"`
	OwnerUID    uint32       `json:"owner_uid"`
	Regular     bool         `json:"regular"`
	Links       uint32       `json:"links"`
	PathFDMatch bool         `json:"path_fd_match"`
}
type FamilyEntry struct {
	Name        string     `json:"name"`
	Before      FileState  `json:"before"`
	After       FileState  `json:"after"`
	Header      RootHeader `json:"header"`
	HeaderBound bool       `json:"header_bound"`
	Removed     bool       `json:"removed"`
}
type Family uint8

const (
	FamilyUnknown Family = iota
	OriginalFamily
	TrialFamily
	PostExitFamily
	ProbeFamily
)

type FamilyManifest struct {
	Family                Family        `json:"family"`
	Source                Family        `json:"source"`
	RootName              string        `json:"root_name"`
	Directory             FileIdentity  `json:"directory"`
	DirectoryMode         uint32        `json:"directory_mode"`
	OwnerUID              uint32        `json:"owner_uid"`
	Entries               []FamilyEntry `json:"entries"`
	Complete              bool          `json:"complete"`
	Frozen                bool          `json:"frozen"`
	FrozenAfterWriterWait bool          `json:"frozen_after_writer_wait"`
	DestinationAbsent     bool          `json:"destination_absent"`
	ExclusiveCopies       bool          `json:"exclusive_copies"`
	DestinationSynced     bool          `json:"destination_synced"`
	ReadbackVerified      bool          `json:"readback_verified"`
	NoDatabaseReopen      bool          `json:"no_database_reopen"`
}
type ProbeEvidence struct {
	OpenCalls              uint8         `json:"open_calls"`
	CloseCalls             uint8         `json:"close_calls"`
	NoCreate               bool          `json:"no_create"`
	ReadOnly               bool          `json:"read_only"`
	KeepWALOnClose         bool          `json:"keep_wal_on_close"`
	SameProfile            bool          `json:"same_profile"`
	VerifyCalls            uint8         `json:"verify_calls"`
	VerifiedSelectedRoot   bool          `json:"verified_selected_root"`
	Ledger                 Ledger        `json:"ledger"`
	Encrypted              EncryptedHead `json:"encrypted"`
	ErrorToken             uint64        `json:"error_token"`
	RepairCalls            uint8         `json:"repair_calls"`
	CheckpointCalls        uint8         `json:"checkpoint_calls"`
	CommitCalls            uint8         `json:"commit_calls"`
	MainWALUnchanged       bool          `json:"main_wal_unchanged"`
	SharedSHMReconstructed bool          `json:"shared_shm_reconstructed"`
}
type Evidence struct {
	Binding             Binding             `json:"binding"`
	Roles               [3]RoleBinding      `json:"roles"`
	Paths               Paths               `json:"paths"`
	Frames              []Frame             `json:"frames"`
	Observations        []Observation       `json:"observations"`
	Plain               PlainHead           `json:"plain"`
	Old                 EncryptedHead       `json:"old"`
	Candidate           RootCandidate       `json:"candidate"`
	Seed                Ledger              `json:"seed"`
	WriterLedger        Ledger              `json:"writer_ledger"`
	Commit              CommitReceipt       `json:"commit"`
	Return              ReturnEvidence      `json:"return"`
	Processes           []ProcessReceipt    `json:"processes"`
	Owners              []OwnerReceipt      `json:"owners"`
	Resources           []ResourceReceipt   `json:"resources"`
	Credentials         []CredentialReceipt `json:"credentials"`
	Families            []FamilyManifest    `json:"families"`
	SeedCopy            FamilyManifest      `json:"seed_copy"`
	Probe               ProbeEvidence       `json:"probe"`
	Filesystem          string              `json:"filesystem"`
	HardwareSectorClaim bool                `json:"hardware_sector_claim"`
	PowerLossClaim      bool                `json:"power_loss_claim"`
	PhysicalClaim       bool                `json:"physical_claim"`
}
type Result struct {
	SourceModelConsistent    bool    `json:"source_model_consistent"`
	PublicCheckpointReturned bool    `json:"public_checkpoint_returned"`
	ActualWaitObserved       bool    `json:"actual_wait_observed"`
	CommittedHeadRecovered   bool    `json:"committed_head_recovered"`
	PhysicalValidated        bool    `json:"physical_validated"`
	Problem                  Problem `json:"problem"`
}

func validateObservations(e Evidence) Problem {
	points := ExpectedPoints(e.Binding.Case)
	if len(e.Observations) != len(points) {
		return ProblemTrace
	}
	if e.Binding.Profile == EncryptedLayout2Raw4096 {
		if ValidateEncryptedTransition(e.Old, e.Candidate) != NoProblem || e.Plain.PageSize != 0 || len(e.Plain.Selected) != 0 {
			return ProblemHead
		}
	} else if ValidatePlainHead(e.Plain) != NoProblem || e.Old != (EncryptedHead{}) || e.Candidate != (RootCandidate{}) {
		return ProblemHead
	}
	for i, o := range e.Observations {
		if o.Point != points[i] {
			return ProblemTrace
		}
		if e.Binding.Profile == EncryptedLayout2Raw4096 {
			want := Observation{Point: points[i], OldRootHash: e.Old.RootHash, CandidateRootHash: e.Candidate.RootHash, OldHeader: e.Old.Header, CandidateHeader: e.Candidate.Header, CandidateBytes: e.Candidate.RootBytes}
			if o != want {
				return ProblemHead
			}
		} else {
			selected := e.Plain.Selected
			if e.Binding.Profile == PlainShared4096 {
				selected = nil
				oldBackfill := e.Observations[0].OldBackfill
				for _, page := range e.Plain.Selected {
					if page.Frame > oldBackfill {
						selected = append(selected, page)
					}
				}
				if len(selected) < 2 {
					return ProblemHead
				}
			}
			count := uint32(len(selected))
			copied, page := count, uint32(0)
			if i == 0 {
				copied, page = 1, selected[0].Page
			}
			want := Observation{Point: points[i], Page: page, SelectedCount: count, CopiedCount: copied, DBSize: e.Plain.DBSize, FrameCount: e.Plain.FrameCount, Salt: e.Plain.Salt, Checksum: e.Plain.Checksum}
			if e.Binding.Profile == PlainShared4096 {
				want.Safe = e.Plain.FrameCount
				want.Attempted = want.Safe
				want.OldBackfill = o.OldBackfill
				want.Backfill = o.OldBackfill
				if o.OldBackfill >= want.Safe {
					return ProblemHead
				}
				if i > 0 {
					want.Backfill = want.Safe
					if o.OldBackfill != e.Observations[0].OldBackfill {
						return ProblemHead
					}
				}
			}
			if o != want {
				return ProblemHead
			}
		}
	}
	return NoProblem
}
func validateFrames(e Evidence, ledger, head Hash) Problem {
	trace := ExpectedTrace(e.Binding.Case)
	if len(e.Frames) != len(trace) || len(e.Frames) > MaxFrames {
		return ProblemTrace
	}
	observed := 0
	var prior Frame
	for i, f := range e.Frames {
		if _, p := EncodeFrame(f); p != NoProblem {
			return p
		}
		if f.Binding != e.Binding || f.Sequence != uint16(i+1) || f.Event != trace[i] {
			return ProblemTrace
		}
		r := e.Roles[int(f.Role)-1]
		if f.Generation != r.Generation || f.Challenge != r.Challenge {
			return ProblemBinding
		}
		point, payload := PointUnknown, ledger
		switch f.Event {
		case ParentAdmitted:
			payload = e.Binding.SourceManifest
		case SeedCommitNil:
			payload = head
		case CommittedReceiptDurable:
			payload = e.Commit.Hash
		case CutObserved:
			point = e.Observations[observed].Point
			payload, _ = canonicalHash(e.Observations[observed])
			observed++
		case CutContinued, ExitGranted:
			point = prior.Point
			payload = prior.Payload
		}
		if f.Point != point || f.Payload != payload {
			return ProblemTrace
		}
		grant := f.Event == WriterOpenGranted || f.Event == CheckpointGranted || f.Event == CutContinued || f.Event == ExitGranted || f.Event == WriterDoneGranted || f.Event == ProbeOpenGranted
		if grant {
			h, p := FrameHash(prior)
			if p != NoProblem || f.Echo != h {
				return ProblemFrame
			}
		} else if f.Echo != (Hash{}) {
			return ProblemFrame
		}
		prior = f
	}
	return NoProblem
}
func validateCommit(e Evidence, ledger, head Hash) Problem {
	r := e.Commit
	if r.Binding != e.Binding || !r.Present || r.CommitCalls != 1 || !r.CommitReturnedNil || !r.CapturedAdjacent || !r.ActualCommitOrigin || !r.BeforeCheckpoint || r.LedgerDigest != ledger || r.HeadDigest != head || !validIdentity(r.File) || !validIdentity(r.Directory) || r.File == r.Directory || r.Hash == (Hash{}) || !r.FullWrite || !r.FileSynced || r.FileCloseCalls != 1 || !r.DirectorySynced || r.DirectoryCloseCalls != 1 || !r.ReadbackVerified || !r.ExclusiveCreated || r.ErrorToken != 0 || r.CheckpointACK {
		return ProblemReceipt
	}
	want := ReturnEvidence{Calls: 1}
	if IsControl(e.Binding.Case) {
		want.Returned = true
		want.Nil = true
		want.CapturedAdjacent = true
	}
	if e.Return != want {
		return ProblemReceipt
	}
	return NoProblem
}
func validateProcessesOwners(e Evidence) Problem {
	if len(e.Processes) != 2 || len(e.Owners) != 3 || len(e.Resources) < 4 || len(e.Resources) > 256 {
		return ProblemOwner
	}
	all := map[uint64]bool{}
	for i, p := range e.Processes {
		role := Role(i + 2)
		r := e.Roles[i+1]
		abrupt := role == Writer && !IsControl(e.Binding.Case)
		exit := 0
		if abrupt {
			exit = ExpectedExitCode
		}
		if p.Role != role || p.Generation != r.Generation || p.Challenge != r.Challenge || p.Owner == 0 || all[p.Owner] || p.Starts != 1 || p.Waits != 1 || !p.ParentOwned || !p.ActualSelfImage || !validIdentity(p.Image) || p.ImageHash != e.Binding.Binary || p.State != ProcessExited || !p.ActualWait || !p.ProcessStatePresent || p.ExitCode != exit || p.SignalNumber != 0 || p.NaturalExit == abrupt || p.OriginalExitError != abrupt || (p.WaitErrorToken != 0) != abrupt || !p.TerminalEOF || p.DiagnosticStall || p.TransportErrorToken != 0 || p.SpawnedChildren || p.Panic || p.OOM || p.ReleasedBeforeWait {
			return ProblemProcess
		}
		all[p.Owner] = true
	}
	for i, o := range e.Owners {
		r := e.Roles[i]
		if o.Role != r.Role || o.Generation != r.Generation || o.Owner == 0 || all[o.Owner] || o.Before != (RegistryCounts{}) || o.CloseErrorToken != 0 || o.UnknownClose || o.RepeatedRetirement || o.DeferredCleanup || o.SecureEraseClaim {
			return ProblemOwner
		}
		all[o.Owner] = true
		if o.Role == Writer && !IsControl(e.Binding.Case) {
			if o.AfterKnown || o.After != (RegistryCounts{}) || o.DBCloseCalls != 0 || o.DependentOwnersRetired {
				return ProblemOwner
			}
		} else if !o.AfterKnown || o.After != (RegistryCounts{}) || o.DBCloseCalls != 1 || !o.DependentOwnersRetired {
			return ProblemOwner
		}
	}
	for i, l := range []Ledger{e.Seed, e.WriterLedger, e.Probe.Ledger} {
		if l.Role != e.Roles[i].Role || l.Generation != e.Roles[i].Generation || l.Owner != e.Owners[i].Owner {
			return ProblemOwner
		}
		for _, s := range l.Streams {
			if all[s.Receipt.Owner] {
				return ProblemOwner
			}
			all[s.Receipt.Owner] = true
		}
	}
	resourceIDs := map[uint64]bool{}
	for _, o := range e.Resources {
		if o.Owner == 0 || resourceIDs[o.Owner] || all[o.Owner] || o.Parent == 0 || !o.CapturedBeforeError || o.CloseCalls != 1 || o.ErrorToken != 0 || !o.Closed || o.Unknown || o.ReleasedBeforeDependency {
			return ProblemOwner
		}
		resourceIDs[o.Owner] = true
		switch o.Kind {
		case "file", "directory", "event-pipe", "control-pipe":
		default:
			return ProblemOwner
		}
		if !all[o.Parent] {
			return ProblemOwner
		}
	}
	return NoProblem
}
func validateCredentials(e Evidence) Problem {
	if e.Binding.Profile != EncryptedLayout2Raw4096 {
		if len(e.Credentials) != 0 {
			return ProblemBinding
		}
		return NoProblem
	}
	if len(e.Credentials) != 3 {
		return ProblemBinding
	}
	paths := [3]string{e.Paths.Original, e.Paths.Trial, e.Paths.Probe}
	for i, c := range e.Credentials {
		op := uint8(1)
		if i == 0 {
			op = 2
		}
		if c.Role != Role(i+1) || c.Provider == 0 || c.Path != paths[i] || c.Operation != op || c.Kind != 1 || c.Acquires != 1 || c.Releases != 1 || !c.Cleared || !c.PurposeChecked || c.ErrorToken != 0 {
			return ProblemBinding
		}
		for j := 0; j < i; j++ {
			if c.Provider == e.Credentials[j].Provider {
				return ProblemBinding
			}
		}
	}
	return NoProblem
}
func validEntry(e FamilyEntry, root string, profile Profile) bool {
	if e.Name == "" || len(e.Name) > 256 || strings.Contains(e.Name, "/") || strings.Contains(e.Name, "\\") || strings.Contains(e.Name, "\x00") {
		return false
	}
	if profile != EncryptedLayout2Raw4096 {
		return !e.HeaderBound && e.Header == (RootHeader{}) && (e.Name == root || e.Name == root+"-wal" || e.Name == root+"-shm")
	}
	if e.Name == root+".gse-lock" {
		return !e.HeaderBound && e.Header == (RootHeader{})
	}
	if !e.HeaderBound {
		return false
	}
	h := e.Header
	role := h.Descriptor.Role
	d := h.Descriptor
	check := h
	check.Descriptor.Role = 1
	check.Descriptor.RecordCount = 1
	if !validRoot(check) || d.CommitSequence == 0 || d.CommitSequence > 16 {
		return false
	}
	if e.Name == root {
		return role == 1 && d.RecordCount == 1 && e.Before.Bytes >= 1064 && e.Before.Bytes <= 1064+16*256 && (e.Before.Bytes-1064)%256 == 0
	}
	prefix := root + ".gse-" + hex.EncodeToString(h.FileID[:])
	suffix := strings.TrimPrefix(e.Name, prefix)
	if suffix == e.Name {
		return false
	}
	switch suffix {
	case ".base":
		return role == 2 && d.RecordCount == uint64(d.PageCount)+2 && e.Before.Bytes == 704+uint64(d.PageCount)*(4096+40)
	case ".wal":
		return role == 3 && d.RecordCount >= 3 && d.RecordCount <= uint64(d.PageCount)+2 && d.ParentCommitID != (Hash{}) && e.Before.Bytes == 928+(d.RecordCount-2)*(4096+44)
	case ".pending", ".retained":
		return role == 1 && d.RecordCount == 1 && e.Before.Bytes >= 1064 && e.Before.Bytes <= 1064+16*256 && (e.Before.Bytes-1064)%256 == 0
	}
	return false
}
func entryByName(m FamilyManifest, name string) (FamilyEntry, bool) {
	for _, e := range m.Entries {
		if e.Name == name {
			return e, true
		}
	}
	return FamilyEntry{}, false
}
func copyMatches(from, to FamilyManifest) bool {
	if len(from.Entries) != len(to.Entries) {
		return false
	}
	for i, a := range from.Entries {
		b := to.Entries[i]
		if a.Name != b.Name || a.Before.Hash != b.Before.Hash || a.Before.Bytes != b.Before.Bytes || a.Header != b.Header || a.HeaderBound != b.HeaderBound || a.Before.Identity == b.Before.Identity {
			return false
		}
	}
	return true
}
func validateFamilies(e Evidence) Problem {
	if len(e.Families) != 4 {
		return ProblemFamily
	}
	root := "fixture.db"
	if e.Binding.Profile == EncryptedLayout2Raw4096 {
		root = "fixture.gse"
	}
	used := map[FileIdentity]bool{e.Commit.File: true, e.Commit.Directory: true}
	for i, m := range e.Families {
		if m.Family != Family(i+1) || m.RootName != root || !validIdentity(m.Directory) || used[m.Directory] || m.DirectoryMode != 0700 || !m.Complete || !m.ReadbackVerified || len(m.Entries) == 0 || len(m.Entries) > MaxFamilyEntries {
			return ProblemFamily
		}
		used[m.Directory] = true
		if i < 3 {
			if !m.Frozen || !m.NoDatabaseReopen {
				return ProblemFamily
			}
		} else if m.NoDatabaseReopen {
			return ProblemFamily
		}
		if i > 0 && !m.FrozenAfterWriterWait {
			return ProblemFamily
		}
		if i == 1 && (m.Source != OriginalFamily || !m.DestinationAbsent || !m.ExclusiveCopies || !m.DestinationSynced) {
			return ProblemFamily
		}
		if i >= 2 {
			if m.Source != Family(i) || !m.DestinationAbsent || !m.ExclusiveCopies || !m.DestinationSynced || !copyMatches(e.Families[i-1], m) {
				return ProblemFamily
			}
		}
		var total uint64
		mainFound := false
		for j, a := range m.Entries {
			b := a.Before
			if !validEntry(a, root, e.Binding.Profile) || j > 0 && m.Entries[j-1].Name >= a.Name || !validIdentity(b.Identity) || used[b.Identity] || b.Hash == (Hash{}) || b.Bytes > MaxFileBytes || b.Mode != 0600 || b.OwnerUID != m.OwnerUID || !b.Regular || b.Links != 1 || !b.PathFDMatch {
				return ProblemFamily
			}
			used[b.Identity] = true
			total += b.Bytes
			if a.Removed {
				if i != 3 || e.Binding.Profile != PlainExclusive4096 || a.Name != root+"-shm" || a.After != (FileState{}) {
					return ProblemFamily
				}
			} else if a.Before != a.After {
				if i != 3 || e.Binding.Profile != PlainShared4096 || a.Name != root+"-shm" || !e.Probe.SharedSHMReconstructed || a.After.Identity != b.Identity || !a.After.Regular || a.After.Links != 1 || !a.After.PathFDMatch || a.After.Mode != 0600 || a.After.OwnerUID != b.OwnerUID || a.After.Bytes > MaxFileBytes || a.After.Hash == (Hash{}) {
					return ProblemFamily
				}
			}
			if a.Name == root {
				mainFound = true
				if e.Binding.Profile == EncryptedLayout2Raw4096 {
					hash, header := e.Candidate.RootHash, e.Candidate.Header
					if i == 0 || e.Binding.Case == EncryptedBeforeRootRename {
						hash, header = e.Old.RootHash, e.Old.Header
					}
					if b.Hash != hash || a.Header != header {
						return ProblemFamily
					}
				} else if b.Bytes > 4096*MaxPages || b.Bytes%4096 != 0 {
					return ProblemFamily
				}
			}
		}
		if !mainFound || total > MaxFamilyBytes {
			return ProblemFamily
		}
	}
	original, trial := e.Families[0], e.Families[1]
	seed := e.SeedCopy
	if seed.Family != TrialFamily || seed.Source != OriginalFamily || !seed.Complete || !seed.DestinationAbsent || !seed.ExclusiveCopies || !seed.DestinationSynced || !seed.ReadbackVerified || !copyMatches(original, seed) {
		return ProblemFamily
	}
	seedIDs := map[FileIdentity]bool{}
	for _, a := range seed.Entries {
		if seedIDs[a.Before.Identity] {
			return ProblemFamily
		}
		seedIDs[a.Before.Identity] = true
		for _, m := range []FamilyManifest{original, e.Families[2], e.Families[3]} {
			for _, b := range m.Entries {
				if a.Before.Identity == b.Before.Identity {
					return ProblemFamily
				}
			}
		}
		if a.Before.Identity == e.Commit.File || a.Before.Identity == e.Commit.Directory {
			return ProblemFamily
		}
		if a.Before != a.After || !validIdentity(a.Before.Identity) || a.Before.Mode != 0600 || a.Before.OwnerUID != original.OwnerUID || !a.Before.Regular || a.Before.Links != 1 || !a.Before.PathFDMatch {
			return ProblemFamily
		}
	}
	if e.Binding.Profile == EncryptedLayout2Raw4096 {
		oldWAL := false
		oldBase := false
		for _, a := range original.Entries {
			if a.HeaderBound && a.Header.Epoch == e.Old.Header.Epoch {
				if a.Header.Descriptor.Role == 2 && a.Header.FileID == e.Old.BaseFileID {
					oldBase = true
				}
				if a.Header.Descriptor.Role == 3 && a.Header.Descriptor.CommitSequence == 4 && a.Header.Descriptor.TransactionID == e.Old.Header.Descriptor.TransactionID && a.Header.Descriptor.ParentCommitID == e.Old.Header.Descriptor.ParentCommitID && a.Header.Descriptor.RecordCount == e.Old.WALFrames+2 && a.Before.Bytes == e.Old.WALBytes {
					oldWAL = true
				}
			}
		}
		if !oldBase || !oldWAL {
			return ProblemHead
		}
		newBase := false
		pending := false
		for _, a := range trial.Entries {
			h := a.Header
			if !a.HeaderBound {
				continue
			}
			d := e.Candidate.Header.Descriptor
			d.Role = 2
			d.RecordCount = uint64(d.PageCount) + 2
			if h.Epoch == e.Candidate.Header.Epoch && h.Descriptor == d && h.FileID != e.Old.BaseFileID {
				newBase = true
			}
			if a.Name == root+".gse-"+hex.EncodeToString(e.Candidate.Header.FileID[:])+".pending" && a.Before.Hash == e.Candidate.RootHash && a.Header == e.Candidate.Header {
				pending = true
			}
		}
		if !newBase || pending != (e.Binding.Case == EncryptedBeforeRootRename) {
			return ProblemHead
		}
		return NoProblem
	}
	oldWal, ok := entryByName(original, root+"-wal")
	if !ok || oldWal.Before.Hash != e.Plain.WALHash || oldWal.Before.Bytes != e.Plain.WALBytes {
		return ProblemHead
	}
	postWal, hasWal := entryByName(trial, root+"-wal")
	retired := e.Binding.Case == ExclusiveControl || e.Binding.Case == ExclusiveAfterWALUnlink
	if retired {
		if hasWal {
			return ProblemHead
		}
		main, _ := entryByName(trial, root)
		if main.Before.Bytes != uint64(e.Plain.DBSize)*4096 {
			return ProblemHead
		}
	} else if !hasWal || postWal.Before.Hash != oldWal.Before.Hash || postWal.Before.Bytes != oldWal.Before.Bytes {
		return ProblemHead
	}
	if e.Binding.Profile == PlainShared4096 {
		for _, name := range []string{root, root + "-wal", root + "-shm"} {
			before, a := entryByName(seed, name)
			after, b := entryByName(trial, name)
			if !a || !b || before.Before.Identity != after.Before.Identity {
				return ProblemFamily
			}
		}
	}
	return NoProblem
}
func validateProbe(e Evidence) Problem {
	p := e.Probe
	if p.OpenCalls != 1 || p.CloseCalls != 1 || !p.NoCreate || !p.KeepWALOnClose || !p.SameProfile || p.ErrorToken != 0 || p.RepairCalls != 0 || p.CheckpointCalls != 0 || p.CommitCalls != 0 || CompareLedgers(e.Seed, p.Ledger) != NoProblem {
		return ProblemHead
	}
	if e.Binding.Profile != EncryptedLayout2Raw4096 {
		if p.ReadOnly || p.VerifyCalls != 0 || p.VerifiedSelectedRoot || p.Encrypted != (EncryptedHead{}) || !p.MainWALUnchanged || p.SharedSHMReconstructed && e.Binding.Profile != PlainShared4096 {
			return ProblemHead
		}
		return NoProblem
	}
	if !p.ReadOnly || p.VerifyCalls != 1 || !p.VerifiedSelectedRoot || p.SharedSHMReconstructed || !validEncryptedHead(p.Encrypted) {
		return ProblemHead
	}
	if e.Binding.Case == EncryptedBeforeRootRename {
		if p.Encrypted != e.Old {
			return ProblemHead
		}
	} else {
		h := p.Encrypted
		if h.RootHash != e.Candidate.RootHash || h.RootBytes != e.Candidate.RootBytes || h.Header != e.Candidate.Header || h.WALSegments != 0 || h.WALFrames != 0 || h.WALBytes != 0 || h.HeadCommitID == e.Old.HeadCommitID || h.BaseFileID == e.Old.BaseFileID {
			return ProblemHead
		}
	}
	return NoProblem
}
func Validate(e Evidence) Result {
	fail := func(p Problem) Result { return Result{Problem: p} }
	if e.HardwareSectorClaim || e.PowerLossClaim || e.PhysicalClaim {
		return fail(ProblemPhysicalClaim)
	}
	if ValidateBinding(e.Binding) != NoProblem || ValidatePaths(e.Paths, e.Binding.Profile) != NoProblem || len(e.Filesystem) == 0 || len(e.Filesystem) > 128 {
		return fail(ProblemBinding)
	}
	for i, r := range e.Roles {
		if r.Role != Role(i+1) || r.Generation == 0 || r.Challenge == (Hash{}) {
			return fail(ProblemBinding)
		}
		for j := 0; j < i; j++ {
			if r.Generation == e.Roles[j].Generation || r.Challenge == e.Roles[j].Challenge {
				return fail(ProblemBinding)
			}
		}
	}
	ledger, p := LedgerDigest(e.Seed)
	if p != NoProblem || CompareLedgers(e.Seed, e.WriterLedger) != NoProblem {
		return fail(ProblemSQL)
	}
	if p = validateObservations(e); p != NoProblem {
		return fail(p)
	}
	if e.Binding.Profile == EncryptedLayout2Raw4096 {
		if e.Old.Header.Descriptor.PageCount != e.Seed.Check.Pages {
			return fail(ProblemHead)
		}
	} else if e.Plain.DBSize != e.Seed.Check.Pages {
		return fail(ProblemHead)
	}
	var head Hash
	if e.Binding.Profile == EncryptedLayout2Raw4096 {
		head, _ = canonicalHash(e.Old)
	} else {
		head, _ = canonicalHash(e.Plain)
	}
	if p = validateCommit(e, ledger, head); p != NoProblem {
		return fail(p)
	}
	if p = validateFrames(e, ledger, head); p != NoProblem {
		return fail(p)
	}
	if p = validateProcessesOwners(e); p != NoProblem {
		return fail(p)
	}
	if p = validateCredentials(e); p != NoProblem {
		return fail(p)
	}
	if p = validateFamilies(e); p != NoProblem {
		return fail(p)
	}
	if p = validateProbe(e); p != NoProblem {
		return fail(p)
	}
	// These booleans describe assertions contained in a consistent receipt.
	// They are not observations performed by this descriptor-free package.
	return Result{SourceModelConsistent: true, PublicCheckpointReturned: IsControl(e.Binding.Case), ActualWaitObserved: true, CommittedHeadRecovered: true, PhysicalValidated: false}
}

// ValidateAuthority validates supplied immutable authority/cut values only. It
// does not evaluate process, commit receipt, family, owner or reopen evidence.
// A nil problem is deliberately not a complete SourceModelConsistent verdict.
func ValidateAuthority(caseID Case, plain PlainHead, old EncryptedHead, candidate RootCandidate, observations []Observation) Problem {
	if CaseName(caseID) == "" {
		return ProblemBinding
	}
	return validateObservations(Evidence{Binding: Binding{Case: caseID, Profile: CaseProfile(caseID)}, Plain: plain, Old: old, Candidate: candidate, Observations: observations})
}
