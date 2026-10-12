package rollbackfault

// SOURCE ONLY: this finite SQL ledger contract performs no SQL, recovery, file
// I/O, native call, descriptor operation, hashing, or physical validation. Its
// observations compare caller-supplied evidence with independently frozen
// expectations. Even completely fabricated but consistent evidence cannot
// establish a physical durability or acceptance result.

const (
	SQLPageSize      = 4096
	SQLPageCount     = 3
	SQLDatabaseBytes = 12288
	SQLJournalBytes  = 16408
	SQLQueryCount    = 7
)

type SQLCase uint8

const (
	SQLCaseUnknown SQLCase = iota
	SQLControl
	SQLIndexPrefix64
	SQLDBSyncBefore
	SQLIndexPrefix64Error
)

type SQLState uint8

const (
	SQLStateUnknown SQLState = iota
	SQLOld
	SQLNew
)

type SQLValueType uint8

const (
	SQLTypeUnknown SQLValueType = iota
	SQLInteger
	SQLText
)

// Explicit type tags prevent stringified integers, NULL, float64, and byte
// slices from being counted as the required int64/string/string SQL values.
type SQLRow struct {
	IDType    SQLValueType
	ID        int64
	KeyType   SQLValueType
	Key       string
	ValueType SQLValueType
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
)

type SQLQueryPlan struct {
	Access     string
	Table      string
	Method     string
	Index      string
	IndexCount uint8
	Field      string
	FieldCount uint8
	Recheck    bool
	Reason     string
}

type SQLQueryWork struct {
	IndexKeysRead  int64
	IndexPageReads int64
	RowIDSeeks     int64
	Candidates     int64
	BaseRowFetches int64
	ReturnedRows   int64
}

type SQLQueryEvidence struct {
	Identity        Identity
	Query           SQLQuery
	Owner           uint64
	Iterator        uint64
	SQL             string
	Rows            [3]SQLRow
	RowCount        uint8
	Plan            SQLQueryPlan
	Work            SQLQueryWork
	EOF             bool
	ErrCalled       bool
	Error           ErrorKind
	ErrorToken      uint64
	CloseCalls      uint8
	CloseError      ErrorKind
	CloseErrorToken uint64
}

type SQLCheckEvidence struct {
	Identity         Identity
	Owner            uint64
	Calls            uint8
	Error            ErrorKind
	ErrorToken       uint64
	Pages            uint32
	FreePages        uint32
	ReservedPages    uint32
	Tables           int
	Indexes          int
	ExtensionIndexes int
	TableRows        uint64
	IndexEntries     uint64
}

type SQLSnapshotEvidence struct {
	Identity        Identity
	Owner           uint64
	Queries         [SQLQueryCount]SQLQueryEvidence
	Check           SQLCheckEvidence
	CloseCalls      uint8
	CloseError      ErrorKind
	CloseErrorToken uint64
	OwnerState      OwnerState
}

// Frozen is taken from the independently prepared original artifact before any
// fault trial. Before/After hash that same original artifact, never the working
// database or recovered output. Hashes are evidence labels, not verified here.
type SQLOriginalEvidence struct {
	Frozen      [SQLPageCount][32]byte
	Before      [SQLPageCount][32]byte
	After       [SQLPageCount][32]byte
	BeforeBytes int64
	AfterBytes  int64
}

type SQLRecoveryLimits struct {
	MaxJournalBytes  uint64
	MaxRecords       uint32
	MaxDatabaseBytes uint64
}

type SQLRecoveryEvidence struct {
	Identity             Identity
	Calls                uint8
	Error                ErrorKind
	ErrorToken           uint64
	Limits               SQLRecoveryLimits
	JournalPresentBefore bool
	JournalPresentAfter  bool
	Replayed             bool
	ReplayedPages        uint32
	OriginalPages        uint32
	OwnersRetired        bool
}

type SQLArtifactRole uint8

const (
	SQLBaselineArtifact SQLArtifactRole = iota
	SQLTrialArtifact
	SQLOriginalJournalArtifact
	SQLPostCopyArtifact
	SQLJournalCopyArtifact
	SQLReplayArtifact
	SQLReplayJournalArtifact
	SQLArtifactCount
)

// Identity is a bounded artifact label. Device/Inode are the stat identity of
// the named artifact, not a descriptor or native-owner capability. The model
// neither stats a path nor hashes bytes; it only cross-checks these observations.
type SQLArtifactStamp struct {
	Identity string
	Device   uint64
	Inode    uint64
	Exists   bool
	Bytes    int64
	SHA256   [32]byte
}

type SQLArtifactEvidence struct {
	Role   SQLArtifactRole
	Before SQLArtifactStamp
	After  SQLArtifactStamp
}

// Counts are observations, not instructions to close or retry anything. A
// failed Commit must be followed by retired writer owners, explicit recovery,
// and a fresh read-only open, without a second Commit or Rollback attempt.
type SQLLifecycleEvidence struct {
	ArmCalls                     uint8
	CommitCalls                  uint8
	FinishCalls                  uint8
	DisarmCalls                  uint8
	RollbackCalls                uint8
	WriterOpens                  uint8
	ReopenCalls                  uint8
	ReopenReadOnly               bool
	WriterRetiredBeforeRecovery  bool
	RecoveryFinishedBeforeReopen bool
	CommitCapturedUnchanged      bool
	PublicCodeIOErr              bool
	PublicRecoveryRequired       bool
	PublicShortWrite             bool
}

type SQLLedgerEvidence struct {
	Identity      Identity
	Case          SQLCase
	Before        SQLSnapshotEvidence
	After         SQLSnapshotEvidence
	Originals     SQLOriginalEvidence
	Artifacts     [SQLArtifactCount]SQLArtifactEvidence
	Recovery      SQLRecoveryEvidence
	Lifecycle     SQLLifecycleEvidence
	PhysicalClaim bool
}

type SQLProblem uint8

const (
	SQLProblemNone SQLProblem = iota
	SQLProblemPlan
	SQLProblemReceipt
	SQLProblemIdentity
	SQLProblemRows
	SQLProblemQueryPlan
	SQLProblemIndexWork
	SQLProblemEOF
	SQLProblemClose
	SQLProblemCheck
	SQLProblemOriginalHashes
	SQLProblemRecovery
	SQLProblemLifecycle
	SQLProblemPhysicalClaim
	SQLProblemArtifacts
)

type SQLObservation struct {
	EvidenceConsistent bool
	ScenarioObserved   bool
	Problem            SQLProblem
	Adapter            Observation
}

func (SQLObservation) PhysicalValidated() bool { return false }
func (SQLObservation) Pass() bool              { return false }

// SQLLedgerRows contains literal expectations, independent of the adapter,
// Commit error, database bytes, query results, and recovery output.
func SQLLedgerRows(state SQLState) [3]SQLRow {
	switch state {
	case SQLOld:
		return [3]SQLRow{
			{SQLInteger, 1, SQLText, "red", SQLText, "one"},
			{SQLInteger, 2, SQLText, "blue", SQLText, "two"},
			{SQLInteger, 3, SQLText, "red", SQLText, "three"},
		}
	case SQLNew:
		return [3]SQLRow{
			{SQLInteger, 1, SQLText, "green", SQLText, "ONE"},
			{SQLInteger, 3, SQLText, "red", SQLText, "three"},
			{SQLInteger, 4, SQLText, "blue", SQLText, "four"},
		}
	default:
		return [3]SQLRow{}
	}
}

func SQLQueryText(query SQLQuery) string {
	switch query {
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

// SQLPlan freezes page roles as schema/table/index by offset 0/4096/8192.
// This function only returns a value; it cannot arm an adapter or open a file.
func SQLPlan(identity Identity, which SQLCase) (Plan, Problem) {
	spec := PlanSpec{
		Identity:  identity,
		MainOwner: 11, JournalOwner: 12, DirectoryOwner: 13, CleanupOwner: 14,
		OriginalPages: 3, DirtyPages: 3, PageSize: SQLPageSize, SectorSize: SQLPageSize,
		Action: Pass, ExpectedCommit: CommitSucceeded,
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
			{Point: DBSync, Occurrence: 1, Role: Database, Owner: 11},
			{Point: JournalUnlink, Occurrence: 1, Role: Journal, Owner: 12},
			{Point: FinalDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},
		},
		CallCount: 18,
	}
	spec.Target = spec.Calls[14]
	switch which {
	case SQLControl:
	case SQLIndexPrefix64:
		spec.Action, spec.Prefix, spec.PrefixError = PrefixShortWrite, 64, false
		spec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 15
	case SQLDBSyncBefore:
		spec.Target, spec.Action = spec.Calls[15], FailBefore
		spec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 16
	case SQLIndexPrefix64Error:
		spec.Action, spec.Prefix, spec.PrefixError = PrefixShortWrite, 64, true
		spec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 15
	default:
		return Plan{}, ProblemTarget
	}
	for i := int(spec.CallCount); i < MaxCalls; i++ {
		spec.Calls[i] = Call{}
	}
	return NewPlan(spec)
}

// ObserveSQLLedger rechecks the actual bounded Receipt value; a caller cannot
// satisfy it by manufacturing an exported Observation with AdapterObserved.
// All input is read by value and none is retained in the returned observation.
func ObserveSQLLedger(plan Plan, receipt Receipt, evidence SQLLedgerEvidence) SQLObservation {
	result := SQLObservation{}
	if evidence.PhysicalClaim {
		result.Problem = SQLProblemPhysicalClaim
		return result
	}
	expectedPlan, problem := SQLPlan(evidence.Identity, evidence.Case)
	if problem != ProblemNone || !plan.valid || plan.spec != expectedPlan.spec {
		result.Problem = SQLProblemPlan
		return result
	}
	if !receipt.plan.valid || receipt.plan.spec != plan.spec {
		result.Problem = SQLProblemReceipt
		return result
	}
	result.Adapter = receipt.Seal()
	if result.Adapter.Outcome != AdapterObserved || result.Adapter.Problem != ProblemNone || result.Adapter.TargetHits != 1 {
		result.Problem = SQLProblemReceipt
		return result
	}
	if evidence.Identity != plan.spec.Identity || evidence.Before.Identity != evidence.Identity || evidence.After.Identity != evidence.Identity || evidence.Recovery.Identity != evidence.Identity {
		result.Problem = SQLProblemIdentity
		return result
	}
	lifecycle := SQLLifecycleEvidence{ArmCalls: 1, CommitCalls: 1, FinishCalls: 1, DisarmCalls: 1, WriterOpens: 1, ReopenCalls: 1, ReopenReadOnly: true, WriterRetiredBeforeRecovery: true, RecoveryFinishedBeforeReopen: true, CommitCapturedUnchanged: true}
	if evidence.Case != SQLControl {
		lifecycle.PublicCodeIOErr, lifecycle.PublicRecoveryRequired = true, true
		lifecycle.PublicShortWrite = evidence.Case == SQLIndexPrefix64
	}
	if evidence.Lifecycle != lifecycle {
		result.Problem = SQLProblemLifecycle
		return result
	}
	if !validSQLReaderOwners(plan.spec, evidence.Before.Owner, evidence.After.Owner) {
		result.Problem = SQLProblemLifecycle
		return result
	}
	if result.Problem = validateSQLSnapshot(evidence.Before, evidence.Identity, SQLOld); result.Problem != SQLProblemNone {
		return result
	}
	state := SQLOld
	if evidence.Case == SQLControl {
		state = SQLNew
	}
	if result.Problem = validateSQLSnapshot(evidence.After, evidence.Identity, state); result.Problem != SQLProblemNone {
		return result
	}
	// Iterator identities must not be re-used across the independent before and
	// after opens, even if all seven query texts and rows happen to agree.
	for _, before := range evidence.Before.Queries {
		for _, after := range evidence.After.Queries {
			if before.Iterator == after.Iterator {
				result.Problem = SQLProblemLifecycle
				return result
			}
		}
	}
	original := evidence.Originals
	if original.BeforeBytes != SQLDatabaseBytes || original.AfterBytes != SQLDatabaseBytes || original.Frozen != original.Before || original.Frozen != original.After {
		result.Problem = SQLProblemOriginalHashes
		return result
	}
	for _, digest := range original.Frozen {
		if digest == ([32]byte{}) {
			result.Problem = SQLProblemOriginalHashes
			return result
		}
	}
	if !validSQLRecovery(evidence.Case, evidence.Recovery) {
		result.Problem = SQLProblemRecovery
		return result
	}
	if !validSQLArtifacts(evidence.Case, evidence.Artifacts) {
		result.Problem = SQLProblemArtifacts
		return result
	}
	result.EvidenceConsistent, result.ScenarioObserved = true, true
	return result
}

func validSQLReaderOwners(spec PlanSpec, before, after uint64) bool {
	if before == 0 || after == 0 || before == after {
		return false
	}
	for _, owner := range [4]uint64{spec.MainOwner, spec.JournalOwner, spec.DirectoryOwner, spec.CleanupOwner} {
		if before == owner || after == owner {
			return false
		}
	}
	return true
}

// ValidateSQLSnapshot permits a source harness to reject a malformed baseline
// before any fault Commit. Success still has no physical validation authority.
func ValidateSQLSnapshot(snapshot SQLSnapshotEvidence, identity Identity, state SQLState) SQLProblem {
	if !validIdentity(identity) || identity.Profile != PlainRollback4096 || snapshot.Identity != identity || snapshot.Owner == 0 {
		return SQLProblemIdentity
	}
	if state != SQLOld && state != SQLNew {
		return SQLProblemRows
	}
	return validateSQLSnapshot(snapshot, identity, state)
}

func validateSQLSnapshot(snapshot SQLSnapshotEvidence, identity Identity, state SQLState) SQLProblem {
	if snapshot.CloseCalls != 1 || snapshot.CloseError != None || snapshot.CloseErrorToken != 0 || snapshot.OwnerState != OwnerRetired {
		return SQLProblemClose
	}
	check := snapshot.Check
	if check.Identity != identity || check.Owner != snapshot.Owner {
		return SQLProblemIdentity
	}
	if check.Calls != 1 || check.Error != None || check.ErrorToken != 0 || check.Pages != 3 || check.FreePages != 0 || check.ReservedPages != 0 || check.Tables != 1 || check.Indexes != 1 || check.ExtensionIndexes != 0 || check.TableRows != 3 || check.IndexEntries != 3 {
		return SQLProblemCheck
	}
	for i, query := range snapshot.Queries {
		if query.Identity != identity || query.Owner != snapshot.Owner || query.Iterator == 0 {
			return SQLProblemIdentity
		}
		for j := 0; j < i; j++ {
			if query.Iterator == snapshot.Queries[j].Iterator {
				return SQLProblemLifecycle
			}
		}
		if query.Query != SQLQuery(i) || query.SQL != SQLQueryText(SQLQuery(i)) {
			return SQLProblemQueryPlan
		}
		if !query.EOF || !query.ErrCalled || query.Error != None || query.ErrorToken != 0 {
			return SQLProblemEOF
		}
		if query.CloseCalls != 1 || query.CloseError != None || query.CloseErrorToken != 0 {
			return SQLProblemClose
		}
		rows, count := expectedSQLQueryRows(state, query.Query)
		if query.RowCount != count || query.Rows != rows {
			return SQLProblemRows
		}
		if !validSQLQueryPlan(query.Query, query.Plan) {
			return SQLProblemQueryPlan
		}
		if !validSQLQueryWork(query.Query, query.Work, count) {
			return SQLProblemIndexWork
		}
	}
	return SQLProblemNone
}

func expectedSQLQueryRows(state SQLState, query SQLQuery) ([3]SQLRow, uint8) {
	all := SQLLedgerRows(state)
	if query == SQLAllRows {
		return all, 3
	}
	key := ""
	switch query {
	case SQLBlueIndex, SQLBlueScan:
		key = "blue"
	case SQLRedIndex, SQLRedScan:
		key = "red"
	case SQLGreenIndex, SQLGreenScan:
		key = "green"
	}
	var selected [3]SQLRow
	var count uint8
	for _, row := range all {
		if row.Key == key {
			selected[count] = row
			count++
		}
	}
	return selected, count
}

func isSQLIndexQuery(query SQLQuery) bool {
	return query == SQLBlueIndex || query == SQLRedIndex || query == SQLGreenIndex
}

func validSQLQueryPlan(query SQLQuery, plan SQLQueryPlan) bool {
	if plan.Table != "t" {
		return false
	}
	if isSQLIndexQuery(query) {
		return plan.Access == "SEARCH" && plan.Method == "btree" && plan.Index == "by_k" && plan.IndexCount == 1 && plan.Field == "k" && plan.FieldCount == 1 && plan.Recheck && len(plan.Reason) <= 1024
	}
	return plan.Access == "SCAN" && plan.Method == "" && plan.Index == "" && plan.IndexCount == 0 && plan.Field == "" && plan.FieldCount == 0 && !plan.Recheck && plan.Reason == "NOT INDEXED requested"
}

func validSQLQueryWork(query SQLQuery, work SQLQueryWork, count uint8) bool {
	n := int64(count)
	if work.ReturnedRows != n {
		return false
	}
	if isSQLIndexQuery(query) {
		// An empty green result still needs a real index-page probe. Nonempty
		// equality keys must each drive one candidate and base-row fetch.
		return work.IndexPageReads > 0 && work.IndexPageReads <= 32 && work.IndexKeysRead == n && work.Candidates == n && work.RowIDSeeks == n && work.BaseRowFetches == n
	}
	return work.IndexKeysRead == 0 && work.IndexPageReads == 0 && work.RowIDSeeks == 0 && work.Candidates == 0 && work.BaseRowFetches == 3
}

func validSQLRecovery(which SQLCase, recovery SQLRecoveryEvidence) bool {
	if which == SQLControl {
		// A control must not call recovery, even as a purported no-op. Its
		// retained post-commit copy is opened read-only without journal replay.
		return recovery == (SQLRecoveryEvidence{Identity: recovery.Identity, OwnersRetired: true})
	}
	if recovery.Calls != 1 || recovery.Error != None || recovery.ErrorToken != 0 || !recovery.OwnersRetired || recovery.JournalPresentAfter || recovery.Limits != (SQLRecoveryLimits{MaxJournalBytes: SQLJournalBytes, MaxRecords: SQLPageCount, MaxDatabaseBytes: SQLDatabaseBytes}) {
		return false
	}
	return recovery.JournalPresentBefore && recovery.Replayed && recovery.ReplayedPages == SQLPageCount && recovery.OriginalPages == SQLPageCount
}

func validSQLArtifacts(which SQLCase, artifacts [SQLArtifactCount]SQLArtifactEvidence) bool {
	for i, artifact := range artifacts {
		if artifact.Role != SQLArtifactRole(i) {
			return false
		}
		journal := artifact.Role == SQLOriginalJournalArtifact || artifact.Role == SQLJournalCopyArtifact || artifact.Role == SQLReplayJournalArtifact
		if journal && which == SQLControl {
			if artifact.Before != (SQLArtifactStamp{}) || artifact.After != (SQLArtifactStamp{}) {
				return false
			}
			continue
		}
		before, after := artifact.Before, artifact.After
		bytes := int64(SQLDatabaseBytes)
		if journal {
			bytes = SQLJournalBytes
		}
		if !validSQLArtifactStamp(before, bytes) || before.Identity != SQLArtifactName(artifact.Role) {
			return false
		}
		if artifact.Role == SQLReplayJournalArtifact {
			if after != (SQLArtifactStamp{}) {
				return false
			}
		} else {
			if !validSQLArtifactStamp(after, bytes) || before.Identity != after.Identity || before.Device != after.Device || before.Inode != after.Inode {
				return false
			}
			if artifact.Role != SQLReplayArtifact && before != after {
				return false
			}
		}
		for j := 0; j < i; j++ {
			other := artifacts[j].Before
			if other.Exists && (before.Identity == other.Identity || (before.Device == other.Device && before.Inode == other.Inode)) {
				return false
			}
		}
	}
	baseline := artifacts[SQLBaselineArtifact].Before.SHA256
	trial := artifacts[SQLTrialArtifact].Before.SHA256
	post := artifacts[SQLPostCopyArtifact].Before.SHA256
	replayBefore := artifacts[SQLReplayArtifact].Before.SHA256
	replayAfter := artifacts[SQLReplayArtifact].After.SHA256
	if baseline == trial || trial != post || trial != replayBefore {
		return false
	}
	if which == SQLControl {
		return replayAfter == trial
	}
	journal := artifacts[SQLOriginalJournalArtifact].Before.SHA256
	return replayAfter == baseline && journal == artifacts[SQLJournalCopyArtifact].Before.SHA256 && journal == artifacts[SQLReplayJournalArtifact].Before.SHA256
}

func SQLArtifactName(role SQLArtifactRole) string {
	switch role {
	case SQLBaselineArtifact:
		return "baseline.input.db"
	case SQLTrialArtifact:
		return "trial.db"
	case SQLOriginalJournalArtifact:
		return "trial.db-journal"
	case SQLPostCopyArtifact:
		return "post-commit.input.db"
	case SQLJournalCopyArtifact:
		return "post-commit-journal.input.bin"
	case SQLReplayArtifact:
		return "replay.db"
	case SQLReplayJournalArtifact:
		return "replay.db-journal"
	default:
		return ""
	}
}

func validSQLArtifactStamp(stamp SQLArtifactStamp, bytes int64) bool {
	if !stamp.Exists || stamp.Device == 0 || stamp.Inode == 0 || stamp.Bytes != bytes || stamp.SHA256 == ([32]byte{}) || len(stamp.Identity) == 0 || stamp.Identity == "0" || len(stamp.Identity) > MaxIDLength {
		return false
	}
	for _, ch := range stamp.Identity {
		if ch < '!' || ch > '~' {
			return false
		}
	}
	return true
}
