package main

// SOURCE_NOT_RUN. This is an inert lexical/AST prevention boundary, not a type
// checker, execution permission, or evidence of physical durability. Frozen
// bytes supplement the semantic checks; independent review must release HOLD.
import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const rollbackFaultSQLBridge = "internal/pager/rollback_fault_sql_manual_linux.go"
const rollbackFaultSQLNative = "sqlite/rollback_fault_sql_manual_linux_test.go"
const rollbackFaultSQLModel = "internal/testkit/rollbackfault/sql_ledger.go"
const rollbackFaultSQLModelTest = "internal/testkit/rollbackfault/sql_ledger_test.go"
const rollbackFaultSQLRoot = "TestManualRollbackFaultSQLLedger"
const rollbackFaultSQLRecoveryCall = "pager.RecoverRollback(ctx, replayPath, pager.RecoveryOptions{Limits: journal.ReplayLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}})"

// The released SQL whole-main/body pins are retained as exact prior evidence.
// New restart wiring is still HOLD: that reviewed SQL identity cannot authorize
// the changed complete main.go source or any native restart execution.
const rollbackFaultSQLMainReview = "HOLD_PENDING_INDEPENDENT_WHOLE_MAIN_REVIEW"

var rollbackFaultSQLPins = map[string]string{
	rollbackFaultSQLBridge:    "9847b258f401c52c8748b2e380a8b96e33eda64567a38edd1833f16df188431a",
	rollbackFaultSQLNative:    "78464b9d6f948be4d1efc0686f8b57a17f1a0ae530081230edc7c553d996bf09",
	rollbackFaultSQLModel:     "96def5b6a7545f7e7f8e9e2acaf16e540237cfa2bccd92875b7961560e328267",
	rollbackFaultSQLModelTest: "07083e68442522330edae5a562e74b2e6d901a9429c846b196737e02b5d7d469",
}

// Full closed declaration/body snapshots are supplemental. Counterexamples
// repin both this snapshot and the byte digest and must still fail semantics.
var rollbackFaultSQLShapes = map[string]string{
	rollbackFaultSQLBridge:    "//go:build linux && manual_fault_rollback\n\npackage pager\n\n// This bridge exists only in the manual build. It exposes neither native\n// owners nor a retry/reset path. The default shipping selection is unchanged.\nimport (\n\t\"errors\"\n\t\"os\"\n\t\"reflect\"\n\n\t\"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault\"\n)\n\ntype manualRollbackSQLScope struct{ scope *rollbackFaultScope }\n\ntype ManualRollbackSQLReport struct {\n\tObservation             rollbackfault.Observation\n\tEvents                  [rollbackfault.MaxEvents]rollbackfault.Event\n\tEventCount              uint8\n\tDiagnostics             [rollbackfault.MaxEvents]error\n\tCommitError             error\n\tProblem                 error\n\tPreDBOnlyTargetCause    bool\n\tPostDBOnlyTargetCause   bool\n\tMisplacedWrite          rollbackfault.SQLMisplacedWrite\n\tMisplaceOnlyTargetCause bool\n}\n\n// Snapshots are separately locked scalar observations, not an atomic proof of\n// absence of concurrent users. The harness requires an independent process.\n// HealthyTx means only an untouched, independently owned preparation Tx can be\n// retired through its public Rollback. No native owner method is invoked here.\ntype ManualRollbackSQLSnapshot struct {\n\tGroups        int\n\tPending       int\n\tHealthyTx     bool\n\tAdapterActive bool\n\tAdapterUsed   bool\n}\n\nfunc ManualRollbackSQLState(tx *Tx) ManualRollbackSQLSnapshot {\n\tvar out ManualRollbackSQLSnapshot\n\tplainRetirements.mu.Lock()\n\tout.Groups = len(plainRetirements.groups)\n\tfor g := range plainRetirements.groups {\n\t\tif g.pending.Load() != nil {\n\t\t\tout.Pending++\n\t\t}\n\t}\n\tplainRetirements.mu.Unlock()\n\trollbackFaultRegistry.mu.Lock()\n\tout.AdapterActive = rollbackFaultRegistry.active != nil\n\tout.AdapterUsed = rollbackFaultRegistry.used\n\trollbackFaultRegistry.mu.Unlock()\n\tif tx != nil {\n\t\ttx.mu.Lock()\n\t\tg := tx.cleanup\n\t\tif g != nil {\n\t\t\tg.mu.Lock()\n\t\t\tmain, concrete := g.main.(plainFile)\n\t\t\tout.HealthyTx = concrete && main.file == tx.file && tx.file != nil && tx.session == nil && tx.shared == nil && tx.encrypted == nil && !tx.keepFile && tx.borrowedCleanup == nil && !g.started && !g.done && g.cause == nil && g.pending.Load() == nil && g.temporary == nil && !g.temporaryReserved && !g.temporaryRequested && g.wal == nil && !g.walReserved && !g.walRequested && len(g.nested) == 0\n\t\t\tg.mu.Unlock()\n\t\t}\n\t\ttx.mu.Unlock()\n\t}\n\treturn out\n}\n\nfunc ArmManualRollbackSQLFault(tx *Tx, plan rollbackfault.Plan) (*manualRollbackSQLScope, error) {\n\tif os.Getenv(\"GOSQLITE_MANUAL_FAULT_ROLLBACK\") != \"independently_authorized\" {\n\t\treturn nil, errRollbackFaultGate\n\t}\n\tspec := plan.Spec()\n\tvar caseID rollbackfault.SQLCase\n\tswitch spec.Action {\n\tcase rollbackfault.Pass:\n\t\tcaseID = rollbackfault.SQLControl\n\tcase rollbackfault.PrefixShortWrite:\n\t\tcaseID = rollbackfault.SQLIndexPrefix64\n\t\tif spec.PrefixError {\n\t\t\tcaseID = rollbackfault.SQLIndexPrefix64Error\n\t\t}\n\tcase rollbackfault.MisplaceIndexPageMinus64:\n\t\tcaseID = rollbackfault.SQLIndexMisplaceMinus64Error\n\tcase rollbackfault.FailBefore:\n\t\tswitch spec.Target.Point {\n\t\tcase rollbackfault.DBSync:\n\t\t\tcaseID = rollbackfault.SQLDBSyncBefore\n\t\tcase rollbackfault.JournalBodySync:\n\t\t\tcaseID = rollbackfault.SQLJournalBodySyncBefore\n\t\tcase rollbackfault.JournalHeaderSync:\n\t\t\tcaseID = rollbackfault.SQLJournalHeaderSyncBefore\n\t\tcase rollbackfault.PublishDirectorySync:\n\t\t\tcaseID = rollbackfault.SQLPublishDirectorySyncBefore\n\t\tcase rollbackfault.JournalUnlink:\n\t\t\tcaseID = rollbackfault.SQLJournalUnlinkBefore\n\t\tcase rollbackfault.FinalDirectorySync:\n\t\t\tcaseID = rollbackfault.SQLFinalDirectorySyncBefore\n\t\tdefault:\n\t\t\treturn nil, errRollbackFaultScope\n\t\t}\n\tdefault:\n\t\treturn nil, errRollbackFaultScope\n\t}\n\tfrozen, problem := rollbackfault.SQLPlan(spec.Identity, caseID)\n\tif problem != rollbackfault.NoProblem || frozen.Spec() != spec {\n\t\treturn nil, errRollbackFaultScope\n\t}\n\tif tx == nil {\n\t\treturn nil, errRollbackFaultScope\n\t}\n\ttx.mu.Lock()\n\tthreePages := tx.originalPages == 3 && tx.header.PageCount == 3 && len(tx.dirty) == 3 && len(tx.dirty[1]) == 4096 && len(tx.dirty[2]) == 4096 && len(tx.dirty[3]) == 4096\n\ttx.mu.Unlock()\n\tif !threePages {\n\t\treturn nil, errRollbackFaultScope\n\t}\n\tstate := ManualRollbackSQLState(tx)\n\tif state.Groups != 1 || state.Pending != 0 || !state.HealthyTx || state.AdapterActive || state.AdapterUsed {\n\t\treturn nil, errRollbackFaultScope\n\t}\n\tscope, err := armRollbackFault(tx, plan)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn &manualRollbackSQLScope{scope: scope}, nil\n}\n\n// The caller must pass the untouched return from public sqlite.Tx.Commit in\n// its immediately following statement. This bridge never invokes Commit.\nfunc FinishManualRollbackSQLFault(scope *manualRollbackSQLScope, commitErr error) ManualRollbackSQLReport {\n\tif scope == nil {\n\t\treturn ManualRollbackSQLReport{Problem: errRollbackFaultScope}\n\t}\n\treport := finishRollbackFault(scope.scope, commitErr)\n\tscope.scope.mu.Lock()\n\ttarget, point := scope.scope.targetError, scope.scope.plan.Spec().Target.Point\n\taction, misplaced := scope.scope.plan.Spec().Action, scope.scope.misplacedWrite\n\tscope.scope.mu.Unlock()\n\tpreDBOnlyTargetCause := false\n\tif point == rollbackfault.JournalBodySync || point == rollbackfault.JournalHeaderSync || point == rollbackfault.PublishDirectorySync {\n\t\tscope.scope.tx.mu.Lock()\n\t\tpoison := scope.scope.tx.poisoned\n\t\tscope.scope.tx.mu.Unlock()\n\t\tpreDBOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr, poison, target, nil)\n\t}\n\tpostDBOnlyTargetCause := false\n\tif point == rollbackfault.JournalUnlink || point == rollbackfault.FinalDirectorySync {\n\t\tscope.scope.tx.mu.Lock()\n\t\tpoison := scope.scope.tx.poisoned\n\t\tscope.scope.tx.mu.Unlock()\n\t\tpermitted := ErrRecoveryRequired\n\t\tif point == rollbackfault.FinalDirectorySync {\n\t\t\tpermitted = ErrCommitUncertain\n\t\t}\n\t\tpostDBOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr, poison, target, permitted)\n\t}\n\tmisplaceOnlyTargetCause := false\n\tif action == rollbackfault.MisplaceIndexPageMinus64 {\n\t\tscope.scope.tx.mu.Lock()\n\t\tpoison := scope.scope.tx.poisoned\n\t\tscope.scope.tx.mu.Unlock()\n\t\tmisplaceOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr, poison, target, ErrRecoveryRequired)\n\t}\n\treturn ManualRollbackSQLReport{Observation: report.Observation, Events: report.Events, EventCount: report.EventCount, Diagnostics: report.Diagnostics, CommitError: report.CommitError, Problem: report.Problem, PreDBOnlyTargetCause: preDBOnlyTargetCause, PostDBOnlyTargetCause: postDBOnlyTargetCause, MisplacedWrite: misplaced, MisplaceOnlyTargetCause: misplaceOnlyTargetCause}\n}\n\n// This observation is limited to the original public wrapper and the actual\n// retired Tx poison. The caller also requires the top object to be *sqlite.Error.\n// Below that wrapper only errors.Join nodes, exact injected/ErrPoisoned leaves,\n// and the one explicitly permitted post-DB classification sentinel are accepted.\n// Unknown shapes and custom Is/As are refused BEFORE any\n// Unwrap, so a consumed cleanup diagnostic cannot disappear behind traversal.\n// No native operation, reset, callback, or retry is introduced here.\nfunc manualRollbackSQLOnlyTargetCause(commitErr, poison, target, permitted error) bool {\n\tif commitErr == nil || poison == nil || target != errRollbackFaultInjected {\n\t\treturn false\n\t}\n\tif permitted != nil && permitted != ErrRecoveryRequired && permitted != ErrCommitUncertain {\n\t\treturn false\n\t}\n\tif _, custom := commitErr.(interface{ Is(error) bool }); custom {\n\t\treturn false\n\t}\n\tif _, custom := commitErr.(interface{ As(any) bool }); custom {\n\t\treturn false\n\t}\n\tpublic, ok := commitErr.(interface{ Unwrap() error })\n\tif !ok || public.Unwrap() != poison {\n\t\treturn false\n\t}\n\tjoinType := reflect.TypeOf(errors.Join(ErrPoisoned, target))\n\tvar pending [rollbackfault.MaxEvents]error\n\tpending[0] = poison\n\tcount, visited := 1, 0\n\tsawTarget, sawPoisoned := false, false\n\tsawPermitted := permitted == nil\n\tfor count != 0 {\n\t\tif visited == len(pending) {\n\t\t\treturn false\n\t\t}\n\t\tvisited++\n\t\tcount--\n\t\tcurrent := pending[count]\n\t\tpending[count] = nil\n\t\tif _, custom := current.(interface{ Is(error) bool }); custom {\n\t\t\treturn false\n\t\t}\n\t\tif _, custom := current.(interface{ As(any) bool }); custom {\n\t\t\treturn false\n\t\t}\n\t\tif current == target {\n\t\t\tsawTarget = true\n\t\t\tcontinue\n\t\t}\n\t\tif current == ErrPoisoned {\n\t\t\tsawPoisoned = true\n\t\t\tcontinue\n\t\t}\n\t\tif permitted != nil && current == permitted {\n\t\t\tsawPermitted = true\n\t\t\tcontinue\n\t\t}\n\t\tif reflect.TypeOf(current) != joinType {\n\t\t\treturn false\n\t\t}\n\t\tchildren := current.(interface{ Unwrap() []error }).Unwrap()\n\t\tif len(children) == 0 || len(children) > len(pending)-count {\n\t\t\treturn false\n\t\t}\n\t\tfor _, child := range children {\n\t\t\tif child == nil {\n\t\t\t\treturn false\n\t\t\t}\n\t\t\tpending[count] = child\n\t\t\tcount++\n\t\t}\n\t}\n\treturn sawTarget && sawPoisoned && sawPermitted\n}\n\nfunc DisarmManualRollbackSQLFault(scope *manualRollbackSQLScope) error {\n\tif scope == nil {\n\t\treturn errRollbackFaultScope\n\t}\n\tstate := ManualRollbackSQLState(nil)\n\tif state.Groups != 0 || state.Pending != 0 {\n\t\treturn errRollbackFaultScope\n\t}\n\treturn disarmRollbackFault(scope.scope)\n}\n",
	rollbackFaultSQLNative:    "//go:build linux && manual_fault_rollback\n\npackage sqlite\n\n// SOURCE ONLY / REVIEW HOLD / PHYSICAL NOT_RUN. One fresh process, one case.\n// A private stable namespace is required; metadata rechecks are not protection\n// against hostile concurrent name replacement. Every input and output remains.\nimport (\n\t\"bytes\"\n\t\"context\"\n\t\"crypto/sha256\"\n\t\"encoding/binary\"\n\t\"encoding/hex\"\n\t\"encoding/json\"\n\t\"errors\"\n\t\"fmt\"\n\t\"io\"\n\t\"os\"\n\t\"path/filepath\"\n\t\"strings\"\n\t\"syscall\"\n\t\"testing\"\n\n\t\"github.com/ms-xiaosnow/gosqlite/internal/journal\"\n\t\"github.com/ms-xiaosnow/gosqlite/internal/pager\"\n\t\"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault\"\n)\n\ntype manualRollbackSQLOwner struct {\n\tfile     *os.File\n\tdb       *DB\n\trows     *Rows\n\tpath     string\n\tcloses   uint8\n\tcloseErr error\n}\n\ntype manualRollbackSQLLedger struct {\n\towners [256]manualRollbackSQLOwner\n\tcount  int\n}\n\nfunc (l *manualRollbackSQLLedger) reserve() (int, error) {\n\tif l.count == len(l.owners) {\n\t\treturn 0, fmt.Errorf(\"owner ledger full\")\n\t}\n\ti := l.count\n\tl.count++\n\treturn i, nil\n}\n\nfunc (l *manualRollbackSQLLedger) close(i int) error {\n\to := &l.owners[i]\n\tif o.closes != 0 {\n\t\treturn fmt.Errorf(\"duplicate close refused: %s\", o.path)\n\t}\n\tif o.file == nil && o.db == nil && o.rows == nil {\n\t\treturn nil\n\t}\n\t// Never turn a DB close into a hidden rollback/cleanup retry.\n\tif o.db != nil && (o.db.tx != nil || len(o.db.rows) != 0) {\n\t\treturn fmt.Errorf(\"DB has live transaction/Rows; retained without retry\")\n\t}\n\to.closes++\n\tif o.file != nil {\n\t\to.closeErr = o.file.Close()\n\t} else if o.rows != nil {\n\t\to.closeErr = o.rows.Close()\n\t} else {\n\t\to.closeErr = o.db.Close()\n\t}\n\treturn o.closeErr\n}\n\nfunc (l *manualRollbackSQLLedger) retired() bool {\n\tfor i := 0; i < l.count; i++ {\n\t\to := l.owners[i]\n\t\tif (o.file != nil || o.db != nil || o.rows != nil) && (o.closes != 1 || o.closeErr != nil) {\n\t\t\treturn false\n\t\t}\n\t}\n\treturn true\n}\n\nfunc (l *manualRollbackSQLLedger) finish(t *testing.T) {\n\t// Reporting only: every acquired owner is retired explicitly at its one\n\t// intended boundary. A failure must not trigger a deferred cleanup retry.\n\tfor i := 0; i < l.count; i++ {\n\t\to := l.owners[i]\n\t\tif (o.file != nil || o.db != nil || o.rows != nil) && (o.closes != 1 || o.closeErr != nil) {\n\t\t\tt.Errorf(\"INCOMPLETE: retained owner %d %s closes=%d error=%v; no retry\", i, o.path, o.closes, o.closeErr)\n\t\t}\n\t}\n}\n\ntype manualRollbackSQLBinding struct {\n\tSource             string\n\tTool               string\n\tBinary             string\n\tManifest           string\n\tCompileReceipt     string\n\tTrial              string\n\tScope              string\n\tCase               string\n\tToolPath           string\n\tSourceManifestPath string\n\tCompileReceiptPath string\n}\n\n// An external reviewed compile receipt is a bounded evidence input, not an\n// executable command or a self-authenticating approval. Its exact bytes must\n// have been reviewed independently and its hash supplied by that review.\ntype manualRollbackSQLCompile struct {\n\tSchema      string\n\tSource      string\n\tTool        string\n\tBinary      string\n\tManifest    string\n\tPackage     string\n\tTags        string\n\tCompileOnly bool\n\tFiles       uint16\n\tSourceTree  string\n}\n\ntype manualRollbackSQLConfig struct {\n\troot          string\n\trootInfo      os.FileInfo\n\tcaseID        rollbackfault.SQLCase\n\tbinding       manualRollbackSQLBinding\n\tbindingBytes  []byte\n\tsourceFiles   uint16\n\tledger        manualRollbackSQLLedger\n\tretained      [12]manualRollbackSQLSaved\n\tretainedCount int\n}\n\ntype manualRollbackSQLSaved struct {\n\tname string\n\tinfo os.FileInfo\n\thash [32]byte\n\tsize int64\n}\n\nfunc manualRollbackSQLHash(value string) ([32]byte, error) {\n\tvar out [32]byte\n\tif len(value) != 64 {\n\t\treturn out, fmt.Errorf(\"SHA-256 requires 64 lowercase hex bytes\")\n\t}\n\tdata, err := hex.DecodeString(value)\n\tif err != nil || hex.EncodeToString(data) != value {\n\t\treturn out, fmt.Errorf(\"invalid SHA-256\")\n\t}\n\tcopy(out[:], data)\n\tif out == ([32]byte{}) {\n\t\treturn out, fmt.Errorf(\"zero SHA-256\")\n\t}\n\treturn out, nil\n}\n\nfunc manualRollbackSQLNamespace(root string) (os.FileInfo, error) {\n\tif !filepath.IsAbs(root) || filepath.Clean(root) != root || root == \"/\" {\n\t\treturn nil, fmt.Errorf(\"absolute clean private root required\")\n\t}\n\tcurrent := string(filepath.Separator)\n\tfor _, component := range strings.Split(strings.TrimPrefix(root, \"/\"), \"/\") {\n\t\tcurrent = filepath.Join(current, component)\n\t\tinfo, err := os.Lstat(current)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tif info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {\n\t\t\treturn nil, fmt.Errorf(\"symlink/non-directory component: %s\", current)\n\t\t}\n\t}\n\tinfo, err := os.Lstat(root)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tmetadata, ok := info.Sys().(*syscall.Stat_t)\n\tif !ok || metadata.Uid != uint32(os.Getuid()) {\n\t\treturn nil, fmt.Errorf(\"private root must belong to the invoking user\")\n\t}\n\tif info.Mode().Perm() != 0700 {\n\t\treturn nil, fmt.Errorf(\"precreated private root must already have mode 0700; no permission changes are made\")\n\t}\n\treturn info, nil\n}\n\nfunc (c *manualRollbackSQLConfig) stable() error {\n\tinfo, err := manualRollbackSQLNamespace(c.root)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif !os.SameFile(info, c.rootInfo) {\n\t\treturn fmt.Errorf(\"root namespace changed; retain everything\")\n\t}\n\treturn nil\n}\n\nfunc manualRollbackSQLRead(l *manualRollbackSQLLedger, path string, limit int64) ([]byte, os.FileInfo, error) {\n\tbefore, err := os.Lstat(path)\n\tif err != nil {\n\t\treturn nil, nil, err\n\t}\n\tif !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > limit {\n\t\treturn nil, nil, fmt.Errorf(\"bounded regular input required: %s\", path)\n\t}\n\ti, err := l.reserve()\n\tif err != nil {\n\t\treturn nil, nil, err\n\t}\n\tf, openErr := os.Open(path)\n\tl.owners[i] = manualRollbackSQLOwner{file: f, path: path}\n\tif openErr != nil {\n\t\treturn nil, nil, errors.Join(openErr, l.close(i))\n\t}\n\tactual, statErr := f.Stat()\n\tif statErr != nil || !os.SameFile(before, actual) || actual.Size() != before.Size() {\n\t\treturn nil, nil, errors.Join(fmt.Errorf(\"input changed on open: %s\", path), statErr, l.close(i))\n\t}\n\tdata, readErr := io.ReadAll(io.LimitReader(f, limit+1))\n\tafter, afterErr := f.Stat()\n\tcloseErr := l.close(i)\n\tpathAfter, pathErr := os.Lstat(path)\n\tif err := errors.Join(readErr, afterErr, closeErr, pathErr); err != nil {\n\t\treturn nil, nil, err\n\t}\n\tif int64(len(data)) != before.Size() || !os.SameFile(before, after) || !os.SameFile(before, pathAfter) || after.Size() != before.Size() || pathAfter.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || !pathAfter.ModTime().Equal(before.ModTime()) {\n\t\treturn nil, nil, fmt.Errorf(\"input changed during bounded read: %s\", path)\n\t}\n\treturn data, before, nil\n}\n\nfunc manualRollbackSQLDecode(raw []byte, value any) error {\n\tdecoder := json.NewDecoder(bytes.NewReader(raw))\n\tdecoder.DisallowUnknownFields()\n\tif err := decoder.Decode(value); err != nil {\n\t\treturn err\n\t}\n\tvar tail any\n\tif decoder.Decode(&tail) != io.EOF {\n\t\treturn fmt.Errorf(\"trailing JSON value\")\n\t}\n\treturn nil\n}\n\nfunc manualRollbackSQLAdmit(c *manualRollbackSQLConfig) error {\n\t// Exact flags are intentional. No regex union, repeated test, subtest,\n\t// random order, competing test, timeout kill, or wrapper is admitted.\n\twant := map[string]string{\n\t\t\"-test.run\": \"^TestManualRollbackFaultSQLLedger$\", \"-test.count\": \"1\",\n\t\t\"-test.shuffle\": \"off\", \"-test.parallel\": \"1\", \"-test.timeout\": \"0\", \"-test.cpu\": \"1\",\n\t\t\"-test.v\": \"true\", // The unique successful terminal uses t.Logf.\n\t}\n\tseen := map[string]bool{}\n\tfor _, arg := range os.Args[1:] {\n\t\tkey, value, found := strings.Cut(arg, \"=\")\n\t\tif !found || seen[key] {\n\t\t\treturn fmt.Errorf(\"explicit unique equals-form test flags required\")\n\t\t}\n\t\tseen[key] = true\n\t\tif expected, ok := want[key]; !ok || value != expected {\n\t\t\treturn fmt.Errorf(\"unreviewed test flag: %s\", arg)\n\t\t}\n\t}\n\tfor key := range want {\n\t\tif !seen[key] {\n\t\t\treturn fmt.Errorf(\"missing exact flag: %s\", key)\n\t\t}\n\t}\n\tif os.Getenv(\"GOSQLITE_ROLLBACK_SQL_PRIVATE_ROOT_CONFIRMED\") != \"private_stable_namespace_retained\" {\n\t\treturn fmt.Errorf(\"private stable namespace and retention must be explicitly confirmed\")\n\t}\n\tc.root = os.Getenv(\"GOSQLITE_ROLLBACK_SQL_ROOT\")\n\tinfo, err := manualRollbackSQLNamespace(c.root)\n\tif err != nil {\n\t\treturn err\n\t}\n\tc.rootInfo = info\n\ti, err := c.ledger.reserve()\n\tif err != nil {\n\t\treturn err\n\t}\n\tdir, openErr := os.Open(c.root)\n\tc.ledger.owners[i] = manualRollbackSQLOwner{file: dir, path: c.root}\n\tif openErr != nil {\n\t\treturn errors.Join(openErr, c.ledger.close(i))\n\t}\n\tentries, readErr := dir.ReadDir(1)\n\tcloseErr := c.ledger.close(i)\n\tif readErr != io.EOF || len(entries) != 0 || closeErr != nil {\n\t\treturn errors.Join(fmt.Errorf(\"root must be empty before admission\"), readErr, closeErr)\n\t}\n\tbindingPath := os.Getenv(\"GOSQLITE_ROLLBACK_SQL_BINDING\")\n\tbindingHash, err := manualRollbackSQLHash(os.Getenv(\"GOSQLITE_ROLLBACK_SQL_BINDING_SHA256\"))\n\tif err != nil {\n\t\treturn err\n\t}\n\traw, _, err := manualRollbackSQLRead(&c.ledger, bindingPath, 65536)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif sha256.Sum256(raw) != bindingHash {\n\t\treturn fmt.Errorf(\"independently reviewed binding hash mismatch\")\n\t}\n\tif err := manualRollbackSQLDecode(raw, &c.binding); err != nil {\n\t\treturn err\n\t}\n\tc.bindingBytes = raw\n\tswitch c.binding.Case {\n\tcase \"control\":\n\t\tc.caseID = rollbackfault.SQLControl\n\tcase \"index-prefix-64\":\n\t\tc.caseID = rollbackfault.SQLIndexPrefix64\n\tcase \"db-sync-before\":\n\t\tc.caseID = rollbackfault.SQLDBSyncBefore\n\tcase \"index-prefix-64-error\":\n\t\tc.caseID = rollbackfault.SQLIndexPrefix64Error\n\tcase \"jbody-sync-before\":\n\t\tc.caseID = rollbackfault.SQLJournalBodySyncBefore\n\tcase \"jheader-sync-before\":\n\t\tc.caseID = rollbackfault.SQLJournalHeaderSyncBefore\n\tcase \"publish-dirsync-before\":\n\t\tc.caseID = rollbackfault.SQLPublishDirectorySyncBefore\n\tcase \"junlink-before\":\n\t\tc.caseID = rollbackfault.SQLJournalUnlinkBefore\n\tcase \"final-dirsync-before\":\n\t\tc.caseID = rollbackfault.SQLFinalDirectorySyncBefore\n\tcase \"index-misplace-minus64-error\":\n\t\tc.caseID = rollbackfault.SQLIndexMisplaceMinus64Error\n\tdefault:\n\t\treturn fmt.Errorf(\"exactly one of ten fixed cases required\")\n\t}\n\tmanifest, _, err := manualRollbackSQLRead(&c.ledger, c.binding.SourceManifestPath, 4<<20)\n\tif err != nil {\n\t\treturn err\n\t}\n\tmanifestHash, err := manualRollbackSQLHash(c.binding.Manifest)\n\tif err != nil || sha256.Sum256(manifest) != manifestHash || c.binding.Source != c.binding.Manifest {\n\t\treturn fmt.Errorf(\"frozen source inventory digest mismatch: %v\", err)\n\t}\n\tvar inventory struct {\n\t\tFiles map[string]struct {\n\t\t\tBytes   int64  `json:\"bytes\"`\n\t\t\tGitBlob string `json:\"git_blob\"`\n\t\t\tGitMode string `json:\"git_mode\"`\n\t\t\tSHA256  string `json:\"sha256\"`\n\t\t} `json:\"files\"`\n\t\tTree string `json:\"tree\"`\n\t}\n\tif err := manualRollbackSQLDecode(manifest, &inventory); err != nil {\n\t\treturn err\n\t}\n\tif len(inventory.Files) == 0 || len(inventory.Files) > 4096 || len(inventory.Tree) != 40 {\n\t\treturn fmt.Errorf(\"finite reviewed source inventory required\")\n\t}\n\tfor path, entry := range inventory.Files {\n\t\tif filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || strings.HasPrefix(path, \"../\") || entry.Bytes < 0 || (entry.GitMode != \"100644\" && entry.GitMode != \"100755\") || len(entry.GitBlob) != 40 {\n\t\t\treturn fmt.Errorf(\"invalid reviewed source inventory entry\")\n\t\t}\n\t\tif _, err := manualRollbackSQLHash(entry.SHA256); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\tc.sourceFiles = uint16(len(inventory.Files))\n\tcompileRaw, _, err := manualRollbackSQLRead(&c.ledger, c.binding.CompileReceiptPath, 65536)\n\tif err != nil {\n\t\treturn err\n\t}\n\tcompileHash, err := manualRollbackSQLHash(c.binding.CompileReceipt)\n\tif err != nil || sha256.Sum256(compileRaw) != compileHash {\n\t\treturn fmt.Errorf(\"independently reviewed compile receipt digest mismatch: %v\", err)\n\t}\n\tvar receipt manualRollbackSQLCompile\n\tif err := manualRollbackSQLDecode(compileRaw, &receipt); err != nil {\n\t\treturn err\n\t}\n\tif receipt.Schema != \"rollback-fault-sql-compile-v1\" || receipt.Source != c.binding.Source || receipt.Tool != c.binding.Tool || receipt.Binary != c.binding.Binary || receipt.Manifest != c.binding.Manifest || receipt.Package != \"./sqlite\" || receipt.Tags != \"manual_fault_rollback\" || !receipt.CompileOnly || receipt.Files != c.sourceFiles || receipt.SourceTree != inventory.Tree {\n\t\treturn fmt.Errorf(\"source-to-binary compile receipt does not bind this trial\")\n\t}\n\ttool, _, err := manualRollbackSQLRead(&c.ledger, c.binding.ToolPath, 128<<20)\n\tif err != nil {\n\t\treturn err\n\t}\n\ttoolHash, err := manualRollbackSQLHash(c.binding.Tool)\n\tif err != nil || sha256.Sum256(tool) != toolHash {\n\t\treturn fmt.Errorf(\"tool digest mismatch: %v\", err)\n\t}\n\texecutable, err := os.Executable()\n\tif err != nil {\n\t\treturn err\n\t}\n\timage, _, err := manualRollbackSQLRead(&c.ledger, executable, 128<<20)\n\tif err != nil {\n\t\treturn err\n\t}\n\tbinaryHash, err := manualRollbackSQLHash(c.binding.Binary)\n\tif err != nil || sha256.Sum256(image) != binaryHash {\n\t\treturn fmt.Errorf(\"actual executable digest mismatch: %v\", err)\n\t}\n\tif _, err := manualRollbackSQLHash(c.binding.Source); err != nil {\n\t\treturn err\n\t}\n\treturn c.stable()\n}\n\nfunc (c *manualRollbackSQLConfig) create(name string, data []byte, retain bool) (os.FileInfo, error) {\n\tif err := c.stable(); err != nil {\n\t\treturn nil, err\n\t}\n\tif filepath.Base(name) != name || name == \".\" || len(data) == 0 || len(data) > 1<<20 {\n\t\treturn nil, fmt.Errorf(\"fixed bounded basename required\")\n\t}\n\tpath := filepath.Join(c.root, name)\n\ti, err := c.ledger.reserve()\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tf, openErr := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)\n\tc.ledger.owners[i] = manualRollbackSQLOwner{file: f, path: path}\n\tif openErr != nil {\n\t\treturn nil, errors.Join(openErr, c.ledger.close(i))\n\t}\n\tinfo, statErr := f.Stat()\n\tif statErr != nil {\n\t\treturn nil, errors.Join(statErr, c.ledger.close(i))\n\t}\n\tfor n := 0; n < c.retainedCount; n++ {\n\t\tif os.SameFile(info, c.retained[n].info) {\n\t\t\treturn nil, errors.Join(fmt.Errorf(\"retained input alias\"), c.ledger.close(i))\n\t\t}\n\t}\n\tn, writeErr := f.Write(data)\n\tif writeErr == nil && n != len(data) {\n\t\twriteErr = io.ErrShortWrite\n\t}\n\tvar syncErr error\n\tif writeErr == nil {\n\t\tsyncErr = f.Sync()\n\t}\n\tcloseErr := c.ledger.close(i)\n\tif err := errors.Join(writeErr, syncErr, closeErr); err != nil {\n\t\treturn nil, err\n\t}\n\tactual, actualInfo, err := manualRollbackSQLRead(&c.ledger, path, int64(len(data)))\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tif !os.SameFile(info, actualInfo) || !bytes.Equal(actual, data) {\n\t\treturn nil, fmt.Errorf(\"created bytes or identity changed\")\n\t}\n\tif retain {\n\t\tif c.retainedCount == len(c.retained) {\n\t\t\treturn nil, fmt.Errorf(\"retained inventory full\")\n\t\t}\n\t\tc.retained[c.retainedCount] = manualRollbackSQLSaved{name: name, info: actualInfo, hash: sha256.Sum256(actual), size: int64(len(actual))}\n\t\tc.retainedCount++\n\t}\n\treturn actualInfo, c.stable()\n}\n\nfunc (c *manualRollbackSQLConfig) retainedUnchanged() error {\n\tif err := c.stable(); err != nil {\n\t\treturn err\n\t}\n\tfor i := 0; i < c.retainedCount; i++ {\n\t\tsaved := c.retained[i]\n\t\traw, info, err := manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, saved.name), saved.size)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tif !os.SameFile(info, saved.info) || info.Size() != saved.size || sha256.Sum256(raw) != saved.hash {\n\t\t\treturn fmt.Errorf(\"retained input changed: %s\", saved.name)\n\t\t}\n\t}\n\treturn nil\n}\n\n// The baseline is authored directly from the frozen independent old ledger.\n// There is no sqlite.Open(create), bootstrap SQL, initial Commit, fixture reader,\n// oracle, generator process, or inference from the damaged database.\nfunc manualRollbackSQLBaseline() ([12288]byte, error) {\n\tvar image [12288]byte\n\tcopy(image[:], \"SQLite format 3\\x00\")\n\tbinary.BigEndian.PutUint16(image[16:], 4096)\n\timage[18], image[19], image[21], image[22], image[23] = 1, 1, 64, 32, 32\n\tbinary.BigEndian.PutUint32(image[24:], 7)\n\tbinary.BigEndian.PutUint32(image[28:], 3)\n\tbinary.BigEndian.PutUint32(image[40:], 9)\n\tbinary.BigEndian.PutUint32(image[44:], 4)\n\tbinary.BigEndian.PutUint32(image[56:], 1)\n\tbinary.BigEndian.PutUint32(image[92:], 7)\n\tbinary.BigEndian.PutUint32(image[96:], 3045000)\n\tschema := [][]any{\n\t\t{\"table\", \"t\", \"t\", int64(2), \"CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT,v TEXT)\"},\n\t\t{\"index\", \"by_k\", \"t\", int64(3), \"CREATE INDEX by_k ON t(k)\"},\n\t}\n\ttable := [][]any{{nil, \"red\", \"one\"}, {nil, \"blue\", \"two\"}, {nil, \"red\", \"three\"}}\n\tindex := [][]any{{\"blue\", int64(2)}, {\"red\", int64(1)}, {\"red\", int64(3)}}\n\tif err := manualRollbackSQLLeaf(image[:4096], 100, 13, schema, []byte{1, 2}); err != nil {\n\t\treturn image, err\n\t}\n\tif err := manualRollbackSQLLeaf(image[4096:8192], 0, 13, table, []byte{1, 2, 3}); err != nil {\n\t\treturn image, err\n\t}\n\tif err := manualRollbackSQLLeaf(image[8192:], 0, 10, index, nil); err != nil {\n\t\treturn image, err\n\t}\n\treturn image, nil\n}\n\n// This deliberately supports only the bounded scalar record shapes above.\nfunc manualRollbackSQLRecord(values []any) ([]byte, error) {\n\theader := []byte{byte(len(values) + 1)}\n\tvar body []byte\n\tfor _, value := range values {\n\t\tswitch v := value.(type) {\n\t\tcase nil:\n\t\t\theader = append(header, 0)\n\t\tcase int64:\n\t\t\tif v < 1 || v > 4 {\n\t\t\t\treturn nil, fmt.Errorf(\"fixture integer outside frozen range\")\n\t\t\t}\n\t\t\theader = append(header, 1)\n\t\t\tbody = append(body, byte(v))\n\t\tcase string:\n\t\t\tif len(v) > 57 {\n\t\t\t\treturn nil, fmt.Errorf(\"fixture text outside single-byte serial bound\")\n\t\t\t}\n\t\t\theader = append(header, byte(13+2*len(v)))\n\t\t\tbody = append(body, v...)\n\t\tdefault:\n\t\t\treturn nil, fmt.Errorf(\"fixture scalar type outside fixed builder\")\n\t\t}\n\t}\n\trecord := append(header, body...)\n\tif len(record) >= 128 {\n\t\treturn nil, fmt.Errorf(\"fixture payload outside single-byte length bound\")\n\t}\n\treturn record, nil\n}\n\nfunc manualRollbackSQLLeaf(page []byte, header int, kind byte, rows [][]any, ids []byte) error {\n\tif len(page) != 4096 || len(rows) > 3 || (kind == 13 && len(ids) != len(rows)) || (kind != 10 && kind != 13) {\n\t\treturn fmt.Errorf(\"fixed leaf shape required\")\n\t}\n\tpage[header] = kind\n\tbinary.BigEndian.PutUint16(page[header+3:], uint16(len(rows)))\n\tend := len(page)\n\tfor i, values := range rows {\n\t\trecord, err := manualRollbackSQLRecord(values)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tcell := []byte{byte(len(record))}\n\t\tif kind == 13 {\n\t\t\tcell = append(cell, ids[i])\n\t\t}\n\t\tcell = append(cell, record...)\n\t\tend -= len(cell)\n\t\tif end < header+8+2*len(rows) {\n\t\t\treturn fmt.Errorf(\"fixed leaf overflow\")\n\t\t}\n\t\tcopy(page[end:], cell)\n\t\tbinary.BigEndian.PutUint16(page[header+8+2*i:], uint16(end))\n\t}\n\tbinary.BigEndian.PutUint16(page[header+5:], uint16(end))\n\treturn nil\n}\n\nfunc manualRollbackSQLRegistryEmpty() bool {\n\ts := pager.ManualRollbackSQLState(nil)\n\treturn s.Groups == 0 && s.Pending == 0 && !s.AdapterActive\n}\n\nfunc manualRollbackSQLOpen(c *manualRollbackSQLConfig, path string, readonly bool) (*DB, int, error) {\n\ti, err := c.ledger.reserve()\n\tif err != nil {\n\t\treturn nil, 0, err\n\t}\n\tdb, openErr := Open(path, Options{ReadOnly: readonly, NoCreate: true, PageSize: 4096, SectorSize: 4096, MaxTransactionPages: 3, MaxTransactionUndoBytes: 65536, MaxSortBytes: 65536})\n\tc.ledger.owners[i] = manualRollbackSQLOwner{db: db, path: path}\n\tif openErr != nil {\n\t\treturn db, i, errors.Join(openErr, c.ledger.close(i))\n\t}\n\tif db == nil || db.session != nil {\n\t\treturn db, i, fmt.Errorf(\"independent plain DB required\")\n\t}\n\treturn db, i, nil\n}\n\n// Retirement on a preparation error is allowed only before adapter ownership\n// transfer and only for the original healthy independent transaction. An\n// unknown, started or pending cleanup is retained, never retried by DB.Close.\nfunc manualRollbackSQLAbandon(tx *Tx, cause error) error {\n\tif tx == nil || tx.done {\n\t\treturn cause\n\t}\n\tstate := pager.ManualRollbackSQLState(tx.storage)\n\tif state.AdapterActive || state.AdapterUsed || state.Pending != 0 || !state.HealthyTx {\n\t\treturn cause\n\t}\n\treturn errors.Join(cause, tx.Rollback())\n}\n\nfunc manualRollbackSQLCommit(c *manualRollbackSQLConfig, ctx context.Context, path string, plan rollbackfault.Plan) (pager.ManualRollbackSQLReport, error) {\n\tspec := plan.Spec()\n\tpreDB := c.caseID == rollbackfault.SQLJournalBodySyncBefore || c.caseID == rollbackfault.SQLJournalHeaderSyncBefore || c.caseID == rollbackfault.SQLPublishDirectorySyncBefore\n\tpostDB := c.caseID == rollbackfault.SQLJournalUnlinkBefore || c.caseID == rollbackfault.SQLFinalDirectorySyncBefore\n\tfrozen, problem := rollbackfault.SQLPlan(spec.Identity, c.caseID)\n\tif problem != rollbackfault.NoProblem || frozen.Spec() != spec {\n\t\treturn pager.ManualRollbackSQLReport{}, fmt.Errorf(\"public Commit case/plan mismatch\")\n\t}\n\tif !manualRollbackSQLRegistryEmpty() {\n\t\treturn pager.ManualRollbackSQLReport{}, fmt.Errorf(\"preexisting owner obligations\")\n\t}\n\tdb, owner, err := manualRollbackSQLOpen(c, path, false)\n\tif err != nil {\n\t\treturn pager.ManualRollbackSQLReport{}, err\n\t}\n\ttx, beginErr := db.Begin(ctx, TxOptions{Immediate: true})\n\tif beginErr != nil {\n\t\treturn pager.ManualRollbackSQLReport{}, errors.Join(manualRollbackSQLAbandon(tx, beginErr), c.ledger.close(owner))\n\t}\n\tfor _, statement := range []string{\n\t\t\"UPDATE t SET k='green',v='ONE' WHERE id=1\",\n\t\t\"DELETE FROM t WHERE id=2\",\n\t\t\"INSERT INTO t(id,k,v) VALUES(4,'blue','four')\",\n\t} {\n\t\tresult, execErr := tx.Exec(ctx, statement)\n\t\tif execErr != nil {\n\t\t\treturn pager.ManualRollbackSQLReport{}, errors.Join(manualRollbackSQLAbandon(tx, execErr), c.ledger.close(owner))\n\t\t}\n\t\tcount, countErr := result.RowsAffected()\n\t\tif countErr != nil || count != 1 {\n\t\t\treturn pager.ManualRollbackSQLReport{}, errors.Join(manualRollbackSQLAbandon(tx, fmt.Errorf(\"DML affected %d: %v\", count, countErr)), c.ledger.close(owner))\n\t\t}\n\t}\n\tscope, armErr := pager.ArmManualRollbackSQLFault(tx.storage, plan)\n\tif armErr != nil {\n\t\treturn pager.ManualRollbackSQLReport{}, errors.Join(manualRollbackSQLAbandon(tx, armErr), c.ledger.close(owner))\n\t}\n\tcommitErr := tx.Commit(ctx)\n\treport := pager.FinishManualRollbackSQLFault(scope, commitErr)\n\t// Keep the original public error object. No storage.Commit bypass, second\n\t// Commit, Rollback, DB.Close or recovery may repair an incomplete result.\n\tif report.CommitError != commitErr || report.Problem != nil || report.Observation.Outcome != rollbackfault.AdapterObserved || report.Observation.TargetHits != 1 || report.Observation.Pass() || report.Observation.PhysicalValidated() || !tx.done || db.tx != nil || tx.activeRows != 0 {\n\t\treturn report, fmt.Errorf(\"INCOMPLETE: public Commit/adapter closure: %v; %v\", commitErr, report.Problem)\n\t}\n\tif c.caseID == rollbackfault.SQLControl {\n\t\tif commitErr != nil {\n\t\t\treturn report, fmt.Errorf(\"control public Commit: %w\", commitErr)\n\t\t}\n\t} else {\n\t\tvar publicError *Error\n\t\tuncertain := c.caseID == rollbackfault.SQLFinalDirectorySyncBefore\n\t\tif !errors.As(commitErr, &publicError) || publicError.Code != CodeIOErr || !errors.Is(commitErr, pager.ErrPoisoned) || errors.Is(commitErr, pager.ErrPlaintextCleanupPending) || errors.Is(commitErr, pager.ErrCommitUncertain) != uncertain {\n\t\t\treturn report, fmt.Errorf(\"unqualified public Commit error: %v\", commitErr)\n\t\t}\n\t\tif preDB {\n\t\t\t// Exact public wrapper identity plus the bridge's actual Tx poison\n\t\t\t// binding excludes extra consumed close errors and unknown chains.\n\t\t\tif commitErr != publicError || !report.PreDBOnlyTargetCause || errors.Is(commitErr, ErrRecoveryRequired) || errors.Is(commitErr, io.ErrShortWrite) {\n\t\t\t\treturn report, fmt.Errorf(\"INCOMPLETE: pre-DB public cause or cleanup differs: %v\", commitErr)\n\t\t\t}\n\t\t} else if postDB {\n\t\t\tif commitErr != publicError || !report.PostDBOnlyTargetCause || errors.Is(commitErr, ErrRecoveryRequired) != !uncertain || errors.Is(commitErr, io.ErrShortWrite) {\n\t\t\t\treturn report, fmt.Errorf(\"INCOMPLETE: post-DB public cause or cleanup differs: %v\", commitErr)\n\t\t\t}\n\t\t} else if !errors.Is(commitErr, ErrRecoveryRequired) {\n\t\t\treturn report, fmt.Errorf(\"unqualified public recovery cause: %v\", commitErr)\n\t\t}\n\t\tif c.caseID == rollbackfault.SQLIndexPrefix64 && !errors.Is(commitErr, io.ErrShortWrite) {\n\t\t\treturn report, fmt.Errorf(\"original short-write cause lost\")\n\t\t}\n\t\tif c.caseID == rollbackfault.SQLIndexMisplaceMinus64Error && (commitErr != publicError || !report.MisplaceOnlyTargetCause || errors.Is(commitErr, io.ErrShortWrite)) {\n\t\t\treturn report, fmt.Errorf(\"INCOMPLETE: misplaced-write public cause or cleanup differs: %v\", commitErr)\n\t\t}\n\t\t// Finish has checked errors.Is against the actual private target error.\n\t\t// Native (64, nil) becomes adapter (64, injected), never ErrShortWrite.\n\t\tif c.caseID == rollbackfault.SQLIndexPrefix64Error && errors.Is(commitErr, io.ErrShortWrite) {\n\t\t\treturn report, fmt.Errorf(\"injected prefix error confused with nil-error short write\")\n\t\t}\n\t}\n\tif err := pager.DisarmManualRollbackSQLFault(scope); err != nil {\n\t\treturn report, err\n\t}\n\tif err := c.ledger.close(owner); err != nil {\n\t\treturn report, err\n\t}\n\tif !manualRollbackSQLRegistryEmpty() {\n\t\treturn report, fmt.Errorf(\"Commit owner retirement incomplete\")\n\t}\n\treturn report, nil\n}\n\n// Inspect only the already-read bounded buffers. These checks are separate\n// from the successful journal-publication events in the generic receipt.\nfunc manualRollbackSQLMisplacement(report pager.ManualRollbackSQLReport, baseline, post, journalBytes []byte) (rollbackfault.SQLMisplacementEvidence, error) {\n\tevidence := rollbackfault.SQLMisplacementEvidence{Writer: report.MisplacedWrite, OnlyTargetCause: report.MisplaceOnlyTargetCause}\n\tif len(baseline) != 12288 || len(post) != 12288 || len(journalBytes) != 16408 {\n\t\treturn evidence, fmt.Errorf(\"misplaced-write exact retained extents required\")\n\t}\n\tevidence.ShiftedSHA256 = sha256.Sum256(post[8128:12224])\n\tevidence.BaselineRangeSHA256 = sha256.Sum256(baseline[8128:12224])\n\tevidence.BaselineTailSHA256 = sha256.Sum256(baseline[12224:12288])\n\tevidence.UntouchedTailSHA256 = sha256.Sum256(post[12224:12288])\n\twriter := evidence.Writer\n\tif writer.RequestedOffset != 8192 || writer.RequestedBytes != 4096 || writer.NativeOffset != 8128 || writer.NativeBytes != 4096 || writer.NativeCalls != 1 || writer.PayloadSHA256 == ([32]byte{}) || writer.PayloadSHA256 != evidence.ShiftedSHA256 || evidence.ShiftedSHA256 == evidence.BaselineRangeSHA256 || !bytes.Equal(post[12224:12288], baseline[12224:12288]) || !evidence.OnlyTargetCause {\n\t\treturn evidence, fmt.Errorf(\"misplaced-write actual argument/payload/tail/cause proof differs\")\n\t}\n\tif report.EventCount != 35 || report.Events[30].Kind != rollbackfault.Returned || report.Events[30].Action != rollbackfault.MisplaceIndexPageMinus64 || report.Events[30].Call != (rollbackfault.Call{Point: rollbackfault.DBPage, Occurrence: 3, Role: rollbackfault.Database, Offset: 8192, Length: 4096, Owner: 11}) || report.Events[30].Result != (rollbackfault.Result{Executed: true, NativeN: 4096, ReturnedN: 4096, ReturnedError: rollbackfault.Injected, NormalizedError: rollbackfault.Injected, Effect: rollbackfault.EffectPossible}) {\n\t\treturn evidence, fmt.Errorf(\"misplaced-write original requested call and native return differ\")\n\t}\n\tplan, err := journal.ParseReplay(journalBytes, journal.ReplayLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288})\n\tif err != nil {\n\t\treturn evidence, err\n\t}\n\tif plan.CountEvidence != journal.ReplayCountFiniteHeaders || plan.Header.Records != 3 || plan.Header.OriginalPages != 3 || plan.Header.PageSize != 4096 || plan.Header.SectorSize != 4096 || plan.DatabaseBytes != 12288 || len(plan.Pages) != 3 {\n\t\treturn evidence, fmt.Errorf(\"misplaced-write complete finite journal inventory required\")\n\t}\n\tif err := plan.ValidateExtent(12288); err != nil {\n\t\treturn evidence, err\n\t}\n\tfor i, page := range plan.Pages {\n\t\tif page.Number != uint32(i+1) || len(page.Data) != 4096 || !bytes.Equal(page.Data, baseline[i*4096:(i+1)*4096]) {\n\t\t\treturn evidence, fmt.Errorf(\"misplaced-write journal original differs from independent baseline\")\n\t\t}\n\t\tevidence.JournalOriginals[i] = sha256.Sum256(page.Data)\n\t}\n\treturn evidence, nil\n}\n\nfunc manualRollbackSQLRecoverCopy(ctx context.Context, replayPath string) (pager.RecoveryResult, error) {\n\tif filepath.Base(replayPath) != \"replay.db\" || !manualRollbackSQLRegistryEmpty() {\n\t\treturn pager.RecoveryResult{}, fmt.Errorf(\"fresh replay path and empty registries required\")\n\t}\n\tresult, recoveryErr := pager.RecoverRollback(ctx, replayPath, pager.RecoveryOptions{Limits: journal.ReplayLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}})\n\tif recoveryErr != nil {\n\t\treturn result, recoveryErr\n\t}\n\tif !manualRollbackSQLRegistryEmpty() {\n\t\treturn result, fmt.Errorf(\"recovery owner retirement incomplete\")\n\t}\n\tif !result.Recovered || result.RestoredPages != 3 || result.OriginalPages != 3 {\n\t\treturn result, fmt.Errorf(\"exact three-page recovery required\")\n\t}\n\treturn result, nil\n}\nfunc manualRollbackSQLQuery(c *manualRollbackSQLConfig, ctx context.Context, db *DB, owner uint64, identity rollbackfault.Identity, query rollbackfault.SQLQuery) (rollbackfault.SQLQueryEvidence, error) {\n\tevidence := rollbackfault.SQLQueryEvidence{Identity: identity, Query: query, Owner: owner, SQL: rollbackfault.SQLQueryText(query)}\n\ti, err := c.ledger.reserve()\n\tif err != nil {\n\t\treturn evidence, err\n\t}\n\trows, queryErr := db.Query(ctx, evidence.SQL)\n\tc.ledger.owners[i] = manualRollbackSQLOwner{rows: rows, path: evidence.SQL}\n\tevidence.Iterator = uint64(i + 1000)\n\tif queryErr != nil {\n\t\treturn evidence, errors.Join(queryErr, c.ledger.close(i))\n\t}\n\tif rows == nil {\n\t\treturn evidence, fmt.Errorf(\"nil Rows with nil error\")\n\t}\n\tplan := rows.Plan()\n\tevidence.Plan = rollbackfault.SQLQueryPlan{Access: plan.Access, Table: plan.Table, Method: plan.Method, IndexCount: uint8(len(plan.Indexes)), FieldCount: uint8(len(plan.Fields)), Recheck: plan.Recheck, Reason: plan.Reason}\n\tif len(plan.Indexes) == 1 {\n\t\tevidence.Plan.Index = plan.Indexes[0]\n\t}\n\tif len(plan.Fields) == 1 {\n\t\tevidence.Plan.Field = plan.Fields[0]\n\t}\n\tif len(plan.Indexes) > 1 || len(plan.Fields) > 1 {\n\t\treturn evidence, errors.Join(fmt.Errorf(\"plan outside fixed SQL/index scope\"), c.ledger.close(i))\n\t}\n\tfor rows.Next() {\n\t\tif evidence.RowCount == 3 {\n\t\t\treturn evidence, errors.Join(fmt.Errorf(\"query exceeded fixed three-row bound\"), c.ledger.close(i))\n\t\t}\n\t\tvalues := rows.Values()\n\t\tif len(values) != 3 {\n\t\t\treturn evidence, errors.Join(fmt.Errorf(\"three typed columns required\"), c.ledger.close(i))\n\t\t}\n\t\tid, idOK := values[0].(int64)\n\t\tkey, keyOK := values[1].(string)\n\t\tvalue, valueOK := values[2].(string)\n\t\tif !idOK || !keyOK || !valueOK {\n\t\t\treturn evidence, errors.Join(fmt.Errorf(\"SQL row types changed\"), c.ledger.close(i))\n\t\t}\n\t\tevidence.Rows[evidence.RowCount] = rollbackfault.SQLRow{IDType: rollbackfault.SQLInteger, ID: id, KeyType: rollbackfault.SQLText, Key: key, ValueType: rollbackfault.SQLText, Value: value}\n\t\tevidence.RowCount++\n\t}\n\t// A single exhausted Next loop, then Err, then a single public Close. EOF\n\t// already retires the internal source; public Close never retries its owner.\n\trowsErr := rows.Err()\n\tevidence.ErrCalled = true\n\tevidence.EOF = rowsErr == nil\n\tstats := rows.Stats()\n\tevidence.Work = rollbackfault.SQLQueryWork{IndexKeysRead: stats.IndexKeysRead, IndexPageReads: stats.IndexPageReads, RowIDSeeks: stats.RowIDSeeks, Candidates: stats.Candidates, BaseRowFetches: stats.BaseRowFetches, ReturnedRows: stats.ReturnedRows}\n\tcloseErr := c.ledger.close(i)\n\tevidence.CloseCalls = c.ledger.owners[i].closes\n\tif err := errors.Join(rowsErr, closeErr); err != nil {\n\t\treturn evidence, err\n\t}\n\treturn evidence, nil\n}\n\nfunc manualRollbackSQLReadback(c *manualRollbackSQLConfig, ctx context.Context, path string, identity rollbackfault.Identity) (rollbackfault.SQLSnapshotEvidence, error) {\n\tevidence := rollbackfault.SQLSnapshotEvidence{Identity: identity}\n\tif !manualRollbackSQLRegistryEmpty() {\n\t\treturn evidence, fmt.Errorf(\"pre-read owner obligations\")\n\t}\n\tbefore, err := os.Lstat(path)\n\tif err != nil || before.Size() != 12288 {\n\t\treturn evidence, fmt.Errorf(\"three-page readback extent: %v\", err)\n\t}\n\tdb, owner, err := manualRollbackSQLOpen(c, path, true)\n\tif err != nil {\n\t\treturn evidence, err\n\t}\n\tevidence.Owner = uint64(owner + 1000)\n\tfor i := 0; i < rollbackfault.SQLQueryCount; i++ {\n\t\tquery, queryErr := manualRollbackSQLQuery(c, ctx, db, evidence.Owner, identity, rollbackfault.SQLQuery(i))\n\t\tevidence.Queries[i] = query\n\t\tif queryErr != nil {\n\t\t\treturn evidence, errors.Join(queryErr, c.ledger.close(owner))\n\t\t}\n\t}\n\tchecked, checkErr := db.Check(ctx, CheckOptions{MaxPages: 3, MaxPageReads: 512})\n\tevidence.Check = rollbackfault.SQLCheckEvidence{Identity: identity, Owner: evidence.Owner, Calls: 1, Pages: checked.Pages, FreePages: checked.FreePages, ReservedPages: checked.ReservedPages, Tables: checked.Tables, Indexes: checked.Indexes, ExtensionIndexes: checked.ExtensionIndexes, TableRows: checked.TableRows, IndexEntries: checked.IndexEntries}\n\tcloseErr := c.ledger.close(owner)\n\tevidence.CloseCalls = c.ledger.owners[owner].closes\n\tif err := errors.Join(checkErr, closeErr); err != nil {\n\t\treturn evidence, err\n\t}\n\tif checked.Scope != CheckScope || checked.PageReads == 0 || checked.PageReads > 512 || !manualRollbackSQLRegistryEmpty() {\n\t\treturn evidence, fmt.Errorf(\"Check scope or owner retirement incomplete\")\n\t}\n\tafter, err := os.Lstat(path)\n\tif err != nil || !os.SameFile(before, after) || after.Size() != 12288 {\n\t\treturn evidence, fmt.Errorf(\"readback file identity/extent changed: %v\", err)\n\t}\n\tevidence.OwnerState = rollbackfault.OwnerRetired\n\treturn evidence, nil\n}\n\nfunc (c *manualRollbackSQLConfig) retain(name string, expected os.FileInfo, limit int64) error {\n\tif c.retainedCount == len(c.retained) {\n\t\treturn fmt.Errorf(\"retained inventory full\")\n\t}\n\traw, info, err := manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, name), limit)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif expected != nil && !os.SameFile(expected, info) {\n\t\treturn fmt.Errorf(\"original artifact identity changed\")\n\t}\n\tc.retained[c.retainedCount] = manualRollbackSQLSaved{name: name, info: info, hash: sha256.Sum256(raw), size: info.Size()}\n\tc.retainedCount++\n\treturn nil\n}\n\nfunc manualRollbackSQLPageHashes(data []byte) ([3][32]byte, error) {\n\tvar result [3][32]byte\n\tif len(data) != 12288 {\n\t\treturn result, fmt.Errorf(\"exact three-page hash input required\")\n\t}\n\tfor i := range result {\n\t\tresult[i] = sha256.Sum256(data[i*4096 : (i+1)*4096])\n\t}\n\treturn result, nil\n}\nfunc manualRollbackSQLArtifact(c *manualRollbackSQLConfig, role rollbackfault.SQLArtifactRole) (rollbackfault.SQLArtifactStamp, error) {\n\tvar stamp rollbackfault.SQLArtifactStamp\n\tname := rollbackfault.SQLArtifactName(role)\n\tpath := filepath.Join(c.root, name)\n\tif _, err := os.Lstat(path); os.IsNotExist(err) {\n\t\treturn stamp, nil\n\t} else if err != nil {\n\t\treturn stamp, err\n\t}\n\tlimit := int64(12288)\n\tif role == rollbackfault.SQLOriginalJournalArtifact || role == rollbackfault.SQLJournalCopyArtifact || role == rollbackfault.SQLReplayJournalArtifact {\n\t\tlimit = 16408\n\t}\n\traw, info, err := manualRollbackSQLRead(&c.ledger, path, limit)\n\tif err != nil {\n\t\treturn stamp, err\n\t}\n\tstat, ok := info.Sys().(*syscall.Stat_t)\n\tif !ok || stat.Nlink != 1 || stat.Ino == 0 {\n\t\treturn stamp, fmt.Errorf(\"single-link concrete artifact identity required\")\n\t}\n\treturn rollbackfault.SQLArtifactStamp{Identity: name, Device: uint64(stat.Dev), Inode: stat.Ino, Exists: true, Bytes: info.Size(), SHA256: sha256.Sum256(raw)}, nil\n}\n\nfunc manualRollbackSQLCapture(c *manualRollbackSQLConfig, evidence *rollbackfault.SQLLedgerEvidence, after bool) error {\n\tif err := c.stable(); err != nil {\n\t\treturn err\n\t}\n\tfor i := 0; i < int(rollbackfault.SQLArtifactCount); i++ {\n\t\trole := rollbackfault.SQLArtifactRole(i)\n\t\tstamp, err := manualRollbackSQLArtifact(c, role)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tevidence.Artifacts[i].Role = role\n\t\tif after {\n\t\t\tevidence.Artifacts[i].After = stamp\n\t\t} else {\n\t\t\tevidence.Artifacts[i].Before = stamp\n\t\t}\n\t}\n\treturn nil\n}\n\nfunc manualRollbackSQLNamespaceContents(c *manualRollbackSQLConfig) error {\n\ti, err := c.ledger.reserve()\n\tif err != nil {\n\t\treturn err\n\t}\n\tdir, openErr := os.Open(c.root)\n\tc.ledger.owners[i] = manualRollbackSQLOwner{file: dir, path: c.root}\n\tif openErr != nil {\n\t\treturn errors.Join(openErr, c.ledger.close(i))\n\t}\n\tentries, readErr := dir.ReadDir(16)\n\tvar tail []os.DirEntry\n\ttailErr := readErr\n\tif readErr == nil {\n\t\ttail, tailErr = dir.ReadDir(1)\n\t}\n\tcloseErr := c.ledger.close(i)\n\tif (readErr != nil && readErr != io.EOF) || tailErr != io.EOF || len(tail) != 0 || len(entries) > 10 || closeErr != nil {\n\t\treturn errors.Join(fmt.Errorf(\"bounded namespace inventory incomplete\"), readErr, tailErr, closeErr)\n\t}\n\tfor _, entry := range entries {\n\t\tswitch entry.Name() {\n\t\tcase \"baseline.input.db\", \"binding.input.json\", \"trial.db\", \"trial.db-journal\", \"post-commit.input.db\", \"post-commit-journal.input.bin\", \"replay.db\", \"replay.db-journal\", \"commit-observation.json\", \"result.json\":\n\t\tdefault:\n\t\t\treturn fmt.Errorf(\"unexpected retained namespace member: %s\", entry.Name())\n\t\t}\n\t\tif !entry.Type().IsRegular() {\n\t\t\treturn fmt.Errorf(\"non-regular retained namespace member\")\n\t\t}\n\t}\n\treturn c.stable()\n}\n\nfunc TestManualRollbackFaultSQLLedger(t *testing.T) {\n\tif os.Getenv(\"GOSQLITE_MANUAL_FAULT_ROLLBACK\") != \"independently_authorized\" {\n\t\tt.Fatal(\"NOT_RUN: separate execution authorization, reviewed provenance and private retained root required\")\n\t}\n\tvar c manualRollbackSQLConfig\n\tdefer c.ledger.finish(t)\n\tif err := manualRollbackSQLAdmit(&c); err != nil {\n\t\tt.Fatalf(\"BLOCKED before fixture: %v\", err)\n\t}\n\tidentity := rollbackfault.Identity{Suite: \"rollback-fault-sql-v1\", Source: c.binding.Source, Tool: c.binding.Tool, Binary: c.binding.Binary, Manifest: c.binding.Manifest, Trial: c.binding.Trial, Scope: c.binding.Scope, Profile: rollbackfault.PlainRollback4096}\n\tplan, problem := rollbackfault.SQLPlan(identity, c.caseID)\n\tif problem != rollbackfault.NoProblem {\n\t\tt.Fatalf(\"BLOCKED before fixture: invalid fixed plan %v\", problem)\n\t}\n\tpreDB := c.caseID == rollbackfault.SQLJournalBodySyncBefore || c.caseID == rollbackfault.SQLJournalHeaderSyncBefore || c.caseID == rollbackfault.SQLPublishDirectorySyncBefore\n\tpostDB := c.caseID == rollbackfault.SQLJournalUnlinkBefore || c.caseID == rollbackfault.SQLFinalDirectorySyncBefore\n\tneedsRecovery := c.caseID == rollbackfault.SQLIndexPrefix64 || c.caseID == rollbackfault.SQLDBSyncBefore || c.caseID == rollbackfault.SQLIndexPrefix64Error || c.caseID == rollbackfault.SQLJournalUnlinkBefore || c.caseID == rollbackfault.SQLIndexMisplaceMinus64Error\n\tbaseline, err := manualRollbackSQLBaseline()\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tfrozenHashes, err := manualRollbackSQLPageHashes(baseline[:])\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif _, err := c.create(\"baseline.input.db\", baseline[:], true); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif _, err := c.create(\"binding.input.json\", c.bindingBytes, true); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tctx := context.Background()\n\tbefore, err := manualRollbackSQLReadback(&c, ctx, filepath.Join(c.root, \"baseline.input.db\"), identity)\n\tif err != nil {\n\t\tt.Fatalf(\"baseline readback: %v\", err)\n\t}\n\tif problem := rollbackfault.ValidateSQLSnapshot(before, identity, rollbackfault.SQLOld); problem != rollbackfault.SQLProblemNone {\n\t\tt.Fatalf(\"baseline independent ledger rejected before fault: %v\", problem)\n\t}\n\tif err := c.retainedUnchanged(); err != nil {\n\t\tt.Fatal(err)\n\t}\n\ttrialInfo, err := c.create(\"trial.db\", baseline[:], false)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\treport, commitCheckErr := manualRollbackSQLCommit(&c, ctx, filepath.Join(c.root, \"trial.db\"), plan)\n\tmisplaced := c.caseID == rollbackfault.SQLIndexMisplaceMinus64Error\n\tvar misplacement rollbackfault.SQLMisplacementEvidence\n\tvar misplacementErr error\n\tvar post, journalBytes []byte\n\tvar postInfo os.FileInfo\n\tif misplaced {\n\t\tmisplacement.Writer, misplacement.OnlyTargetCause = report.MisplacedWrite, report.MisplaceOnlyTargetCause\n\t\tif commitCheckErr == nil {\n\t\t\tpost, postInfo, err = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, \"trial.db\"), 12288)\n\t\t\tmisplacementErr = err\n\t\t\tif err == nil && (len(post) != 12288 || !os.SameFile(trialInfo, postInfo)) {\n\t\t\t\tmisplacementErr = fmt.Errorf(\"misplaced-write post-Commit extent/identity changed\")\n\t\t\t}\n\t\t\tif misplacementErr == nil {\n\t\t\t\tjournalBytes, _, misplacementErr = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, \"trial.db-journal\"), 16408)\n\t\t\t}\n\t\t\tif misplacementErr == nil {\n\t\t\t\tmisplacement, misplacementErr = manualRollbackSQLMisplacement(report, baseline[:], post, journalBytes)\n\t\t\t}\n\t\t}\n\t}\n\t// Retain the untouched public error text and all original diagnostics even\n\t// when the trial must stop before copying/recovery. JSON is supplementary\n\t// evidence; error identity checks occurred directly on the original object.\n\tvar diagnosticTexts [rollbackfault.MaxEvents]string\n\tfor i, diagnostic := range report.Diagnostics {\n\t\tif diagnostic != nil {\n\t\t\tdiagnosticTexts[i] = diagnostic.Error()\n\t\t}\n\t}\n\tcommitText := \"\"\n\tif report.CommitError != nil {\n\t\tcommitText = report.CommitError.Error()\n\t}\n\tmisplacementText := \"\"\n\tif misplacementErr != nil {\n\t\tmisplacementText = misplacementErr.Error()\n\t}\n\tcommitRecord, marshalErr := json.MarshalIndent(struct {\n\t\tIdentity              rollbackfault.Identity\n\t\tPlan                  rollbackfault.PlanSpec\n\t\tObservation           rollbackfault.Observation\n\t\tEvents                [rollbackfault.MaxEvents]rollbackfault.Event\n\t\tEventCount            uint8\n\t\tCommitError           string\n\t\tDiagnostics           [rollbackfault.MaxEvents]string\n\t\tPreDBOnlyTargetCause  bool\n\t\tPostDBOnlyTargetCause bool\n\t\tMisplacement          rollbackfault.SQLMisplacementEvidence\n\t\tMisplacementError     string\n\t}{identity, plan.Spec(), report.Observation, report.Events, report.EventCount, commitText, diagnosticTexts, report.PreDBOnlyTargetCause, report.PostDBOnlyTargetCause, misplacement, misplacementText}, \"\", \"  \")\n\tif marshalErr != nil {\n\t\tt.Fatal(marshalErr)\n\t}\n\tif _, err := c.create(\"commit-observation.json\", commitRecord, true); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif commitCheckErr != nil {\n\t\tt.Fatalf(\"INCOMPLETE: original Commit=%v; adapter=%+v; problem=%v; all inputs retained\", report.CommitError, report.Observation, commitCheckErr)\n\t}\n\tif misplacementErr != nil {\n\t\tt.Fatalf(\"INCOMPLETE: misplaced-write byte/journal proof: %v; all inputs retained\", misplacementErr)\n\t}\n\treceipt, receiptProblem := rollbackfault.NewReceipt(plan)\n\tif receiptProblem != rollbackfault.NoProblem {\n\t\tt.Fatal(receiptProblem)\n\t}\n\tfor i := uint8(0); i < report.EventCount; i++ {\n\t\tif problem := receipt.Append(report.Events[i]); problem != rollbackfault.NoProblem {\n\t\t\tt.Fatal(problem)\n\t\t}\n\t}\n\tif receipt.Seal() != report.Observation {\n\t\tt.Fatal(\"adapter transcript/observation mismatch\")\n\t}\n\tevidence := rollbackfault.SQLLedgerEvidence{Identity: identity, Case: c.caseID, Before: before, Misplacement: misplacement}\n\tevidence.Lifecycle = rollbackfault.SQLLifecycleEvidence{ArmCalls: 1, CommitCalls: 1, FinishCalls: 1, DisarmCalls: 1, WriterOpens: 1, ReopenCalls: 1, ReopenReadOnly: true, WriterRetiredBeforeRecovery: true, RecoveryFinishedBeforeReopen: true, CommitCapturedUnchanged: true}\n\tvar publicError *Error\n\tevidence.Lifecycle.PublicCodeIOErr = errors.As(report.CommitError, &publicError) && publicError.Code == CodeIOErr\n\tevidence.Lifecycle.PublicRecoveryRequired = errors.Is(report.CommitError, ErrRecoveryRequired)\n\tevidence.Lifecycle.PublicShortWrite = errors.Is(report.CommitError, io.ErrShortWrite)\n\tevidence.Lifecycle.PublicPoisoned = errors.Is(report.CommitError, pager.ErrPoisoned)\n\tevidence.Lifecycle.PublicCommitUncertain = errors.Is(report.CommitError, pager.ErrCommitUncertain)\n\tevidence.Lifecycle.PublicCleanupPending = errors.Is(report.CommitError, pager.ErrPlaintextCleanupPending)\n\tevidence.Lifecycle.PreDBOnlyTargetCause = report.PreDBOnlyTargetCause\n\tevidence.Lifecycle.PostDBOnlyTargetCause = report.PostDBOnlyTargetCause\n\tif !misplaced {\n\t\tpost, postInfo, err = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, \"trial.db\"), 12288)\n\t}\n\tif err != nil || len(post) != 12288 || !os.SameFile(trialInfo, postInfo) {\n\t\tt.Fatalf(\"post-Commit extent/identity: %v\", err)\n\t}\n\tif preDB && !bytes.Equal(post, baseline[:]) {\n\t\tt.Fatal(\"INCOMPLETE: pre-DB failure changed original database bytes\")\n\t}\n\tif (c.caseID == rollbackfault.SQLIndexPrefix64 || c.caseID == rollbackfault.SQLIndexPrefix64Error) && !bytes.Equal(post[8192+64:], baseline[8192+64:]) {\n\t\tt.Fatal(\"index short write changed bytes beyond the frozen 64-byte prefix\")\n\t}\n\tif err := c.retain(\"trial.db\", trialInfo, 12288); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif _, err := c.create(\"post-commit.input.db\", post, true); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif postDB {\n\t\t// Read the retained journal-free byte copy, never the original trial.\n\t\t// All seven NEW queries, Check and retired readers precede any replay.\n\t\tpostCommit, readErr := manualRollbackSQLReadback(&c, ctx, filepath.Join(c.root, \"post-commit.input.db\"), identity)\n\t\tevidence.Lifecycle.ReopenCalls++\n\t\tif readErr != nil {\n\t\t\tt.Fatalf(\"post-DB complete NEW readback: %v\", readErr)\n\t\t}\n\t\tevidence.PostCommit = postCommit\n\t\tif problem := rollbackfault.ValidateSQLSnapshot(postCommit, identity, rollbackfault.SQLNew); problem != rollbackfault.SQLProblemNone {\n\t\t\tt.Fatalf(\"post-DB independent NEW ledger rejected before replay: %v\", problem)\n\t\t}\n\t\tif !c.ledger.retired() || !manualRollbackSQLRegistryEmpty() {\n\t\t\tt.Fatal(\"INCOMPLETE: post-DB NEW snapshot owner retirement before replay\")\n\t\t}\n\t\tif err := c.retainedUnchanged(); err != nil {\n\t\t\tt.Fatal(err)\n\t\t}\n\t}\n\tif !needsRecovery {\n\t\tif _, err := os.Lstat(filepath.Join(c.root, \"trial.db-journal\")); !os.IsNotExist(err) {\n\t\t\tt.Fatalf(\"zero-recovery case journal must be absent after Commit: %v\", err)\n\t\t}\n\t} else {\n\t\tif !misplaced {\n\t\t\tjournalBytes, _, err = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, \"trial.db-journal\"), 16408)\n\t\t}\n\t\tif err != nil || len(journalBytes) != 16408 {\n\t\t\tt.Fatalf(\"exact original three-record journal: %v\", err)\n\t\t}\n\t\tif err := c.retain(\"trial.db-journal\", nil, 16408); err != nil {\n\t\t\tt.Fatal(err)\n\t\t}\n\t\tif _, err := c.create(\"post-commit-journal.input.bin\", journalBytes, true); err != nil {\n\t\t\tt.Fatal(err)\n\t\t}\n\t}\n\treplayInfo, err := c.create(\"replay.db\", post, false)\n\tif err != nil || os.SameFile(trialInfo, replayInfo) {\n\t\tt.Fatalf(\"exclusive fresh replay: %v\", err)\n\t}\n\tif needsRecovery {\n\t\tif _, err := c.create(\"replay.db-journal\", journalBytes, false); err != nil {\n\t\t\tt.Fatal(err)\n\t\t}\n\t}\n\tif err := manualRollbackSQLCapture(&c, &evidence, false); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tfor i, artifact := range evidence.Artifacts {\n\t\tfor j := 0; j < i; j++ {\n\t\t\tother := evidence.Artifacts[j].Before\n\t\t\tif artifact.Before.Exists && other.Exists && artifact.Before.Device == other.Device && artifact.Before.Inode == other.Inode {\n\t\t\t\tt.Fatal(\"replay or retained artifact aliases an original\")\n\t\t\t}\n\t\t}\n\t}\n\tif err := manualRollbackSQLNamespaceContents(&c); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif err := c.retainedUnchanged(); err != nil {\n\t\tt.Fatal(err)\n\t}\n\toldBefore, _, err := manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, \"baseline.input.db\"), 12288)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tevidence.Originals.Frozen = frozenHashes\n\tevidence.Originals.Before, err = manualRollbackSQLPageHashes(oldBefore)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tevidence.Originals.BeforeBytes = int64(len(oldBefore))\n\tevidence.Recovery = rollbackfault.SQLRecoveryEvidence{Identity: identity, OwnersRetired: manualRollbackSQLRegistryEmpty()}\n\tif needsRecovery {\n\t\tresult, recoveryErr := manualRollbackSQLRecoverCopy(ctx, filepath.Join(c.root, \"replay.db\"))\n\t\tevidence.Recovery.Calls++\n\t\tif recoveryErr != nil {\n\t\t\tt.Fatalf(\"INCOMPLETE: recovery failed; all originals retained: %v\", recoveryErr)\n\t\t}\n\t\tevidence.Recovery.Limits = rollbackfault.SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}\n\t\tevidence.Recovery.JournalPresentBefore = evidence.Artifacts[rollbackfault.SQLReplayJournalArtifact].Before.Exists\n\t\tevidence.Recovery.Replayed, evidence.Recovery.ReplayedPages, evidence.Recovery.OriginalPages = result.Recovered, result.RestoredPages, result.OriginalPages\n\t\tevidence.Recovery.OwnersRetired = manualRollbackSQLRegistryEmpty()\n\t}\n\tif _, err := os.Lstat(filepath.Join(c.root, \"replay.db-journal\")); !os.IsNotExist(err) {\n\t\tt.Fatalf(\"replay journal not absent after the fixed recovery decision: %v\", err)\n\t}\n\tif err := c.retainedUnchanged(); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tafter, err := manualRollbackSQLReadback(&c, ctx, filepath.Join(c.root, \"replay.db\"), identity)\n\tif err != nil {\n\t\tt.Fatalf(\"fresh read-only SQL/index readback: %v\", err)\n\t}\n\tevidence.After = after\n\twant := rollbackfault.SQLOld\n\tif c.caseID == rollbackfault.SQLControl || c.caseID == rollbackfault.SQLFinalDirectorySyncBefore {\n\t\twant = rollbackfault.SQLNew\n\t}\n\tif problem := rollbackfault.ValidateSQLSnapshot(after, identity, want); problem != rollbackfault.SQLProblemNone {\n\t\tt.Fatalf(\"independent full SQL/index ledger rejected: %v\", problem)\n\t}\n\tif err := manualRollbackSQLCapture(&c, &evidence, true); err != nil {\n\t\tt.Fatal(err)\n\t}\n\toldAfter, _, err := manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, \"baseline.input.db\"), 12288)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tevidence.Originals.After, err = manualRollbackSQLPageHashes(oldAfter)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tevidence.Originals.AfterBytes = int64(len(oldAfter))\n\tif err := c.retainedUnchanged(); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif err := manualRollbackSQLNamespaceContents(&c); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif !c.ledger.retired() || !manualRollbackSQLRegistryEmpty() {\n\t\tt.Fatal(\"INCOMPLETE: terminal owner retirement\")\n\t}\n\tobservation := rollbackfault.ObserveSQLLedger(plan, receipt, evidence)\n\tif !observation.EvidenceConsistent || !observation.ScenarioObserved || observation.Problem != rollbackfault.SQLProblemNone || observation.PhysicalValidated() || observation.Pass() {\n\t\tt.Fatalf(\"SQL evidence inconsistent: %+v\", observation)\n\t}\n\tterminal, err := json.MarshalIndent(struct {\n\t\tSchema         string\n\t\tEvidence       rollbackfault.SQLLedgerEvidence\n\t\tObservation    rollbackfault.SQLObservation\n\t\tSourceFiles    uint16\n\t\tCompileReceipt string\n\t}{\"rollback-fault-sql-observed-v1\", evidence, observation, c.sourceFiles, c.binding.CompileReceipt}, \"\", \"  \")\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif _, err := c.create(\"result.json\", terminal, false); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif !c.ledger.retired() || !manualRollbackSQLRegistryEmpty() {\n\t\tt.Fatal(\"INCOMPLETE: terminal file owner retirement\")\n\t}\n\tt.Logf(\"ROLLBACK_SQL_SCENARIO_OBSERVED case=%s trial=%s source=%s binary=%s physical_validated=false pass=false\", c.binding.Case, c.binding.Trial, c.binding.Source, c.binding.Binary)\n}\n",
	rollbackFaultSQLModel:     "package rollbackfault\n\n// SOURCE ONLY: this finite SQL ledger contract performs no SQL, recovery, file\n// I/O, native call, descriptor operation, hashing, or physical validation. Its\n// observations compare caller-supplied evidence with independently frozen\n// expectations. Even completely fabricated but consistent evidence cannot\n// establish a physical durability or acceptance result.\n\nconst (\n\tSQLPageSize      = 4096\n\tSQLPageCount     = 3\n\tSQLDatabaseBytes = 12288\n\tSQLJournalBytes  = 16408\n\tSQLQueryCount    = 7\n)\n\ntype SQLCase uint8\n\nconst (\n\tSQLCaseUnknown SQLCase = iota\n\tSQLControl\n\tSQLIndexPrefix64\n\tSQLDBSyncBefore\n\tSQLIndexPrefix64Error\n\tSQLJournalBodySyncBefore\n\tSQLJournalHeaderSyncBefore\n\tSQLPublishDirectorySyncBefore\n\tSQLJournalUnlinkBefore\n\tSQLFinalDirectorySyncBefore\n\tSQLIndexMisplaceMinus64Error\n)\n\ntype SQLState uint8\n\nconst (\n\tSQLStateUnknown SQLState = iota\n\tSQLOld\n\tSQLNew\n)\n\ntype SQLValueType uint8\n\nconst (\n\tSQLTypeUnknown SQLValueType = iota\n\tSQLInteger\n\tSQLText\n)\n\n// Explicit type tags prevent stringified integers, NULL, float64, and byte\n// slices from being counted as the required int64/string/string SQL values.\ntype SQLRow struct {\n\tIDType    SQLValueType\n\tID        int64\n\tKeyType   SQLValueType\n\tKey       string\n\tValueType SQLValueType\n\tValue     string\n}\n\ntype SQLQuery uint8\n\nconst (\n\tSQLAllRows SQLQuery = iota\n\tSQLBlueIndex\n\tSQLBlueScan\n\tSQLRedIndex\n\tSQLRedScan\n\tSQLGreenIndex\n\tSQLGreenScan\n)\n\ntype SQLQueryPlan struct {\n\tAccess     string\n\tTable      string\n\tMethod     string\n\tIndex      string\n\tIndexCount uint8\n\tField      string\n\tFieldCount uint8\n\tRecheck    bool\n\tReason     string\n}\n\ntype SQLQueryWork struct {\n\tIndexKeysRead  int64\n\tIndexPageReads int64\n\tRowIDSeeks     int64\n\tCandidates     int64\n\tBaseRowFetches int64\n\tReturnedRows   int64\n}\n\ntype SQLQueryEvidence struct {\n\tIdentity        Identity\n\tQuery           SQLQuery\n\tOwner           uint64\n\tIterator        uint64\n\tSQL             string\n\tRows            [3]SQLRow\n\tRowCount        uint8\n\tPlan            SQLQueryPlan\n\tWork            SQLQueryWork\n\tEOF             bool\n\tErrCalled       bool\n\tError           ErrorKind\n\tErrorToken      uint64\n\tCloseCalls      uint8\n\tCloseError      ErrorKind\n\tCloseErrorToken uint64\n}\n\ntype SQLCheckEvidence struct {\n\tIdentity         Identity\n\tOwner            uint64\n\tCalls            uint8\n\tError            ErrorKind\n\tErrorToken       uint64\n\tPages            uint32\n\tFreePages        uint32\n\tReservedPages    uint32\n\tTables           int\n\tIndexes          int\n\tExtensionIndexes int\n\tTableRows        uint64\n\tIndexEntries     uint64\n}\n\ntype SQLSnapshotEvidence struct {\n\tIdentity        Identity\n\tOwner           uint64\n\tQueries         [SQLQueryCount]SQLQueryEvidence\n\tCheck           SQLCheckEvidence\n\tCloseCalls      uint8\n\tCloseError      ErrorKind\n\tCloseErrorToken uint64\n\tOwnerState      OwnerState\n}\n\n// Frozen is taken from the independently prepared original artifact before any\n// fault trial. Before/After hash that same original artifact, never the working\n// database or recovered output. Hashes are evidence labels, not verified here.\ntype SQLOriginalEvidence struct {\n\tFrozen      [SQLPageCount][32]byte\n\tBefore      [SQLPageCount][32]byte\n\tAfter       [SQLPageCount][32]byte\n\tBeforeBytes int64\n\tAfterBytes  int64\n}\n\ntype SQLRecoveryLimits struct {\n\tMaxJournalBytes  uint64\n\tMaxRecords       uint32\n\tMaxDatabaseBytes uint64\n}\n\ntype SQLRecoveryEvidence struct {\n\tIdentity             Identity\n\tCalls                uint8\n\tError                ErrorKind\n\tErrorToken           uint64\n\tLimits               SQLRecoveryLimits\n\tJournalPresentBefore bool\n\tJournalPresentAfter  bool\n\tReplayed             bool\n\tReplayedPages        uint32\n\tOriginalPages        uint32\n\tOwnersRetired        bool\n}\n\ntype SQLArtifactRole uint8\n\nconst (\n\tSQLBaselineArtifact SQLArtifactRole = iota\n\tSQLTrialArtifact\n\tSQLOriginalJournalArtifact\n\tSQLPostCopyArtifact\n\tSQLJournalCopyArtifact\n\tSQLReplayArtifact\n\tSQLReplayJournalArtifact\n\tSQLArtifactCount\n)\n\n// Identity is a bounded artifact label. Device/Inode are the stat identity of\n// the named artifact, not a descriptor or native-owner capability. The model\n// neither stats a path nor hashes bytes; it only cross-checks these observations.\ntype SQLArtifactStamp struct {\n\tIdentity string\n\tDevice   uint64\n\tInode    uint64\n\tExists   bool\n\tBytes    int64\n\tSHA256   [32]byte\n}\n\ntype SQLArtifactEvidence struct {\n\tRole   SQLArtifactRole\n\tBefore SQLArtifactStamp\n\tAfter  SQLArtifactStamp\n}\n\n// The writer supplies these real arguments before its one native invocation.\n// The generic receipt still records the requested Call without alteration.\ntype SQLMisplacedWrite struct {\n\tRequestedOffset int64\n\tRequestedBytes  int\n\tNativeOffset    int64\n\tNativeBytes     int\n\tNativeCalls     uint8\n\tPayloadSHA256   [32]byte\n}\n\n// This fixed evidence is zero for every other case. Hashes remain supplied\n// labels here; only the source-gated manual caller derives them from bytes.\ntype SQLMisplacementEvidence struct {\n\tWriter              SQLMisplacedWrite\n\tShiftedSHA256       [32]byte\n\tBaselineRangeSHA256 [32]byte\n\tBaselineTailSHA256  [32]byte\n\tUntouchedTailSHA256 [32]byte\n\tJournalOriginals    [SQLPageCount][32]byte\n\tOnlyTargetCause     bool\n}\n\n// Counts are observations, not instructions to close or retry anything. A\n// failed Commit must be followed by retired writer owners and a fresh read-only\n// open. Five fixed post-write failures call recovery; pre-DB failures and\n// final-directory uncertainty require zero recovery calls. The two post-DB\n// cases also read the retained post-Commit copy as NEW before any replay. No second\n// Commit or Rollback attempt is permitted. The two recovery-boundary flags also\n// bind the no-recovery path: retirement and the zero-call decision precede open.\ntype SQLLifecycleEvidence struct {\n\tArmCalls                     uint8\n\tCommitCalls                  uint8\n\tFinishCalls                  uint8\n\tDisarmCalls                  uint8\n\tRollbackCalls                uint8\n\tWriterOpens                  uint8\n\tReopenCalls                  uint8\n\tReopenReadOnly               bool\n\tWriterRetiredBeforeRecovery  bool\n\tRecoveryFinishedBeforeReopen bool\n\tCommitCapturedUnchanged      bool\n\tPublicCodeIOErr              bool\n\tPublicRecoveryRequired       bool\n\tPublicShortWrite             bool\n\tPublicPoisoned               bool\n\tPublicCommitUncertain        bool\n\tPublicCleanupPending         bool\n\tPreDBOnlyTargetCause         bool\n\tPostDBOnlyTargetCause        bool\n}\n\ntype SQLLedgerEvidence struct {\n\tIdentity      Identity\n\tCase          SQLCase\n\tBefore        SQLSnapshotEvidence\n\tPostCommit    SQLSnapshotEvidence\n\tAfter         SQLSnapshotEvidence\n\tOriginals     SQLOriginalEvidence\n\tArtifacts     [SQLArtifactCount]SQLArtifactEvidence\n\tRecovery      SQLRecoveryEvidence\n\tLifecycle     SQLLifecycleEvidence\n\tMisplacement  SQLMisplacementEvidence\n\tPhysicalClaim bool\n}\n\ntype SQLProblem uint8\n\nconst (\n\tSQLProblemNone SQLProblem = iota\n\tSQLProblemPlan\n\tSQLProblemReceipt\n\tSQLProblemIdentity\n\tSQLProblemRows\n\tSQLProblemQueryPlan\n\tSQLProblemIndexWork\n\tSQLProblemEOF\n\tSQLProblemClose\n\tSQLProblemCheck\n\tSQLProblemOriginalHashes\n\tSQLProblemRecovery\n\tSQLProblemLifecycle\n\tSQLProblemPhysicalClaim\n\tSQLProblemArtifacts\n\tSQLProblemMisplacement\n)\n\ntype SQLObservation struct {\n\tEvidenceConsistent bool\n\tScenarioObserved   bool\n\tProblem            SQLProblem\n\tAdapter            Observation\n}\n\nfunc (SQLObservation) PhysicalValidated() bool { return false }\nfunc (SQLObservation) Pass() bool              { return false }\n\n// SQLLedgerRows contains literal expectations, independent of the adapter,\n// Commit error, database bytes, query results, and recovery output.\nfunc SQLLedgerRows(state SQLState) [3]SQLRow {\n\tswitch state {\n\tcase SQLOld:\n\t\treturn [3]SQLRow{\n\t\t\t{SQLInteger, 1, SQLText, \"red\", SQLText, \"one\"},\n\t\t\t{SQLInteger, 2, SQLText, \"blue\", SQLText, \"two\"},\n\t\t\t{SQLInteger, 3, SQLText, \"red\", SQLText, \"three\"},\n\t\t}\n\tcase SQLNew:\n\t\treturn [3]SQLRow{\n\t\t\t{SQLInteger, 1, SQLText, \"green\", SQLText, \"ONE\"},\n\t\t\t{SQLInteger, 3, SQLText, \"red\", SQLText, \"three\"},\n\t\t\t{SQLInteger, 4, SQLText, \"blue\", SQLText, \"four\"},\n\t\t}\n\tdefault:\n\t\treturn [3]SQLRow{}\n\t}\n}\n\nfunc SQLQueryText(query SQLQuery) string {\n\tswitch query {\n\tcase SQLAllRows:\n\t\treturn \"SELECT id,k,v FROM t NOT INDEXED ORDER BY id\"\n\tcase SQLBlueIndex:\n\t\treturn \"SELECT id,k,v FROM t WHERE k='blue' ORDER BY id\"\n\tcase SQLBlueScan:\n\t\treturn \"SELECT id,k,v FROM t NOT INDEXED WHERE k='blue' ORDER BY id\"\n\tcase SQLRedIndex:\n\t\treturn \"SELECT id,k,v FROM t WHERE k='red' ORDER BY id\"\n\tcase SQLRedScan:\n\t\treturn \"SELECT id,k,v FROM t NOT INDEXED WHERE k='red' ORDER BY id\"\n\tcase SQLGreenIndex:\n\t\treturn \"SELECT id,k,v FROM t WHERE k='green' ORDER BY id\"\n\tcase SQLGreenScan:\n\t\treturn \"SELECT id,k,v FROM t NOT INDEXED WHERE k='green' ORDER BY id\"\n\tdefault:\n\t\treturn \"\"\n\t}\n}\n\n// SQLPlan freezes page roles as schema/table/index by offset 0/4096/8192.\n// This function only returns a value; it cannot arm an adapter or open a file.\nfunc SQLPlan(identity Identity, which SQLCase) (Plan, Problem) {\n\tspec := PlanSpec{\n\t\tIdentity:  identity,\n\t\tMainOwner: 11, JournalOwner: 12, DirectoryOwner: 13, CleanupOwner: 14,\n\t\tOriginalPages: 3, DirtyPages: 3, PageSize: SQLPageSize, SectorSize: SQLPageSize,\n\t\tAction: Pass, ExpectedCommit: CommitSucceeded,\n\t\tCalls: [MaxCalls]Call{\n\t\t\t{Point: JournalCreate, Occurrence: 1, Role: Journal, Owner: 12},\n\t\t\t{Point: JournalInvalid, Occurrence: 1, Role: Journal, Owner: 12, Length: 4096},\n\t\t\t{Point: DBOriginalRead, Occurrence: 1, Role: Database, Owner: 11, Offset: 0, Length: 4096},\n\t\t\t{Point: JournalRecord, Occurrence: 1, Role: Journal, Owner: 12, Offset: 4096, Length: 4104},\n\t\t\t{Point: DBOriginalRead, Occurrence: 2, Role: Database, Owner: 11, Offset: 4096, Length: 4096},\n\t\t\t{Point: JournalRecord, Occurrence: 2, Role: Journal, Owner: 12, Offset: 8200, Length: 4104},\n\t\t\t{Point: DBOriginalRead, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192, Length: 4096},\n\t\t\t{Point: JournalRecord, Occurrence: 3, Role: Journal, Owner: 12, Offset: 12304, Length: 4104},\n\t\t\t{Point: JournalBodySync, Occurrence: 1, Role: Journal, Owner: 12},\n\t\t\t{Point: JournalValidHeader, Occurrence: 1, Role: Journal, Owner: 12, Length: 12},\n\t\t\t{Point: JournalHeaderSync, Occurrence: 1, Role: Journal, Owner: 12},\n\t\t\t{Point: PublishDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},\n\t\t\t{Point: DBPage, Occurrence: 1, Role: Database, Owner: 11, Offset: 0, Length: 4096},\n\t\t\t{Point: DBPage, Occurrence: 2, Role: Database, Owner: 11, Offset: 4096, Length: 4096},\n\t\t\t{Point: DBPage, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192, Length: 4096},\n\t\t\t{Point: DBSync, Occurrence: 1, Role: Database, Owner: 11},\n\t\t\t{Point: JournalUnlink, Occurrence: 1, Role: Journal, Owner: 12},\n\t\t\t{Point: FinalDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},\n\t\t},\n\t\tCallCount: 18,\n\t}\n\tspec.Target = spec.Calls[14]\n\tswitch which {\n\tcase SQLControl:\n\tcase SQLIndexPrefix64:\n\t\tspec.Action, spec.Prefix, spec.PrefixError = PrefixShortWrite, 64, false\n\t\tspec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 15\n\tcase SQLDBSyncBefore:\n\t\tspec.Target, spec.Action = spec.Calls[15], FailBefore\n\t\tspec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 16\n\tcase SQLIndexPrefix64Error:\n\t\tspec.Action, spec.Prefix, spec.PrefixError = PrefixShortWrite, 64, true\n\t\tspec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 15\n\tcase SQLJournalBodySyncBefore:\n\t\tspec.Target, spec.Action = spec.Calls[8], FailBefore\n\t\tspec.ExpectedCommit, spec.CallCount = CommitFailed, 11\n\tcase SQLJournalHeaderSyncBefore:\n\t\tspec.Target, spec.Action = spec.Calls[10], FailBefore\n\t\tspec.ExpectedCommit, spec.CallCount = CommitFailed, 13\n\tcase SQLPublishDirectorySyncBefore:\n\t\tspec.Target, spec.Action = spec.Calls[11], FailBefore\n\t\tspec.ExpectedCommit, spec.CallCount = CommitFailed, 14\n\tcase SQLJournalUnlinkBefore:\n\t\tspec.Target, spec.Action = spec.Calls[16], FailBefore\n\t\tspec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 17\n\tcase SQLFinalDirectorySyncBefore:\n\t\tspec.Target, spec.Action = spec.Calls[17], FailBefore\n\t\tspec.ExpectedCommit, spec.CallCount = CommitUncertain, 18\n\tcase SQLIndexMisplaceMinus64Error:\n\t\tspec.Action = MisplaceIndexPageMinus64\n\t\tspec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 15\n\tdefault:\n\t\treturn Plan{}, ProblemTarget\n\t}\n\tif isSQLPreDBFailure(which) {\n\t\t// The selected Sync/helper is not called. Only the normal two cleanup\n\t\t// operations follow; no database write or recovery is in this trace.\n\t\tspec.Calls[spec.CallCount-2] = Call{Point: CleanupUnlink, Occurrence: 1, Role: Journal, Owner: 12}\n\t\tspec.Calls[spec.CallCount-1] = Call{Point: CleanupDirectorySync, Occurrence: 1, Role: Directory, Owner: 13}\n\t}\n\tfor i := int(spec.CallCount); i < MaxCalls; i++ {\n\t\tspec.Calls[i] = Call{}\n\t}\n\treturn NewPlan(spec)\n}\n\n// ObserveSQLLedger rechecks the actual bounded Receipt value; a caller cannot\n// satisfy it by manufacturing an exported Observation with AdapterObserved.\n// All input is read by value and none is retained in the returned observation.\nfunc ObserveSQLLedger(plan Plan, receipt Receipt, evidence SQLLedgerEvidence) SQLObservation {\n\tresult := SQLObservation{}\n\tif evidence.PhysicalClaim {\n\t\tresult.Problem = SQLProblemPhysicalClaim\n\t\treturn result\n\t}\n\texpectedPlan, problem := SQLPlan(evidence.Identity, evidence.Case)\n\tif problem != ProblemNone || !plan.valid || plan.spec != expectedPlan.spec {\n\t\tresult.Problem = SQLProblemPlan\n\t\treturn result\n\t}\n\tif !receipt.plan.valid || receipt.plan.spec != plan.spec {\n\t\tresult.Problem = SQLProblemReceipt\n\t\treturn result\n\t}\n\tresult.Adapter = receipt.Seal()\n\tif result.Adapter.Outcome != AdapterObserved || result.Adapter.Problem != ProblemNone || result.Adapter.TargetHits != 1 {\n\t\tresult.Problem = SQLProblemReceipt\n\t\treturn result\n\t}\n\tif evidence.Identity != plan.spec.Identity || evidence.Before.Identity != evidence.Identity || evidence.After.Identity != evidence.Identity || evidence.Recovery.Identity != evidence.Identity {\n\t\tresult.Problem = SQLProblemIdentity\n\t\treturn result\n\t}\n\tlifecycle := SQLLifecycleEvidence{ArmCalls: 1, CommitCalls: 1, FinishCalls: 1, DisarmCalls: 1, WriterOpens: 1, ReopenCalls: 1, ReopenReadOnly: true, WriterRetiredBeforeRecovery: true, RecoveryFinishedBeforeReopen: true, CommitCapturedUnchanged: true}\n\tif evidence.Case != SQLControl {\n\t\tlifecycle.PublicCodeIOErr, lifecycle.PublicPoisoned = true, true\n\t\tlifecycle.PublicRecoveryRequired = needsSQLRecovery(evidence.Case)\n\t\tlifecycle.PublicShortWrite = evidence.Case == SQLIndexPrefix64\n\t\tlifecycle.PreDBOnlyTargetCause = isSQLPreDBFailure(evidence.Case)\n\t\tlifecycle.PostDBOnlyTargetCause = isSQLPostDBFailure(evidence.Case)\n\t\tlifecycle.PublicCommitUncertain = evidence.Case == SQLFinalDirectorySyncBefore\n\t}\n\tif isSQLPostDBFailure(evidence.Case) {\n\t\tlifecycle.ReopenCalls = 2\n\t}\n\tif evidence.Lifecycle != lifecycle {\n\t\tresult.Problem = SQLProblemLifecycle\n\t\treturn result\n\t}\n\tif !validSQLReaderOwners(plan.spec, evidence.Before.Owner, evidence.After.Owner) {\n\t\tresult.Problem = SQLProblemLifecycle\n\t\treturn result\n\t}\n\tif result.Problem = validateSQLSnapshot(evidence.Before, evidence.Identity, SQLOld); result.Problem != SQLProblemNone {\n\t\treturn result\n\t}\n\tstate := SQLOld\n\tif evidence.Case == SQLControl || evidence.Case == SQLFinalDirectorySyncBefore {\n\t\tstate = SQLNew\n\t}\n\tif result.Problem = validateSQLSnapshot(evidence.After, evidence.Identity, state); result.Problem != SQLProblemNone {\n\t\treturn result\n\t}\n\tif isSQLPostDBFailure(evidence.Case) {\n\t\tif evidence.PostCommit.Identity != evidence.Identity || !validSQLReaderOwners(plan.spec, evidence.Before.Owner, evidence.PostCommit.Owner) || !validSQLReaderOwners(plan.spec, evidence.After.Owner, evidence.PostCommit.Owner) {\n\t\t\tresult.Problem = SQLProblemLifecycle\n\t\t\treturn result\n\t\t}\n\t\tif result.Problem = validateSQLSnapshot(evidence.PostCommit, evidence.Identity, SQLNew); result.Problem != SQLProblemNone {\n\t\t\treturn result\n\t\t}\n\t\tfor _, post := range evidence.PostCommit.Queries {\n\t\t\tfor i := 0; i < SQLQueryCount; i++ {\n\t\t\t\tif post.Iterator == evidence.Before.Queries[i].Iterator || post.Iterator == evidence.After.Queries[i].Iterator {\n\t\t\t\t\tresult.Problem = SQLProblemLifecycle\n\t\t\t\t\treturn result\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t} else if evidence.PostCommit != (SQLSnapshotEvidence{}) {\n\t\tresult.Problem = SQLProblemLifecycle\n\t\treturn result\n\t}\n\t// Iterator identities must not be re-used across the independent before and\n\t// after opens, even if all seven query texts and rows happen to agree.\n\tfor _, before := range evidence.Before.Queries {\n\t\tfor _, after := range evidence.After.Queries {\n\t\t\tif before.Iterator == after.Iterator {\n\t\t\t\tresult.Problem = SQLProblemLifecycle\n\t\t\t\treturn result\n\t\t\t}\n\t\t}\n\t}\n\toriginal := evidence.Originals\n\tif original.BeforeBytes != SQLDatabaseBytes || original.AfterBytes != SQLDatabaseBytes || original.Frozen != original.Before || original.Frozen != original.After {\n\t\tresult.Problem = SQLProblemOriginalHashes\n\t\treturn result\n\t}\n\tfor _, digest := range original.Frozen {\n\t\tif digest == ([32]byte{}) {\n\t\t\tresult.Problem = SQLProblemOriginalHashes\n\t\t\treturn result\n\t\t}\n\t}\n\tif !validSQLMisplacement(evidence.Case, evidence.Misplacement, original.Frozen) {\n\t\tresult.Problem = SQLProblemMisplacement\n\t\treturn result\n\t}\n\tif !validSQLRecovery(evidence.Case, evidence.Recovery) {\n\t\tresult.Problem = SQLProblemRecovery\n\t\treturn result\n\t}\n\tif !validSQLArtifacts(evidence.Case, evidence.Artifacts) {\n\t\tresult.Problem = SQLProblemArtifacts\n\t\treturn result\n\t}\n\tresult.EvidenceConsistent, result.ScenarioObserved = true, true\n\treturn result\n}\n\nfunc validSQLMisplacement(which SQLCase, evidence SQLMisplacementEvidence, originals [SQLPageCount][32]byte) bool {\n\tif which != SQLIndexMisplaceMinus64Error {\n\t\treturn evidence == (SQLMisplacementEvidence{})\n\t}\n\twriter := evidence.Writer\n\treturn writer.RequestedOffset == 8192 && writer.RequestedBytes == 4096 && writer.NativeOffset == 8128 && writer.NativeBytes == 4096 && writer.NativeCalls == 1 && writer.PayloadSHA256 != ([32]byte{}) && writer.PayloadSHA256 == evidence.ShiftedSHA256 && evidence.BaselineRangeSHA256 != ([32]byte{}) && evidence.ShiftedSHA256 != evidence.BaselineRangeSHA256 && evidence.BaselineTailSHA256 != ([32]byte{}) && evidence.UntouchedTailSHA256 == evidence.BaselineTailSHA256 && evidence.JournalOriginals == originals && evidence.OnlyTargetCause\n}\n\nfunc validSQLReaderOwners(spec PlanSpec, before, after uint64) bool {\n\tif before == 0 || after == 0 || before == after {\n\t\treturn false\n\t}\n\tfor _, owner := range [4]uint64{spec.MainOwner, spec.JournalOwner, spec.DirectoryOwner, spec.CleanupOwner} {\n\t\tif before == owner || after == owner {\n\t\t\treturn false\n\t\t}\n\t}\n\treturn true\n}\n\n// ValidateSQLSnapshot permits a source harness to reject a malformed baseline\n// before any fault Commit. Success still has no physical validation authority.\nfunc ValidateSQLSnapshot(snapshot SQLSnapshotEvidence, identity Identity, state SQLState) SQLProblem {\n\tif !validIdentity(identity) || identity.Profile != PlainRollback4096 || snapshot.Identity != identity || snapshot.Owner == 0 {\n\t\treturn SQLProblemIdentity\n\t}\n\tif state != SQLOld && state != SQLNew {\n\t\treturn SQLProblemRows\n\t}\n\treturn validateSQLSnapshot(snapshot, identity, state)\n}\n\nfunc validateSQLSnapshot(snapshot SQLSnapshotEvidence, identity Identity, state SQLState) SQLProblem {\n\tif snapshot.CloseCalls != 1 || snapshot.CloseError != None || snapshot.CloseErrorToken != 0 || snapshot.OwnerState != OwnerRetired {\n\t\treturn SQLProblemClose\n\t}\n\tcheck := snapshot.Check\n\tif check.Identity != identity || check.Owner != snapshot.Owner {\n\t\treturn SQLProblemIdentity\n\t}\n\tif check.Calls != 1 || check.Error != None || check.ErrorToken != 0 || check.Pages != 3 || check.FreePages != 0 || check.ReservedPages != 0 || check.Tables != 1 || check.Indexes != 1 || check.ExtensionIndexes != 0 || check.TableRows != 3 || check.IndexEntries != 3 {\n\t\treturn SQLProblemCheck\n\t}\n\tfor i, query := range snapshot.Queries {\n\t\tif query.Identity != identity || query.Owner != snapshot.Owner || query.Iterator == 0 {\n\t\t\treturn SQLProblemIdentity\n\t\t}\n\t\tfor j := 0; j < i; j++ {\n\t\t\tif query.Iterator == snapshot.Queries[j].Iterator {\n\t\t\t\treturn SQLProblemLifecycle\n\t\t\t}\n\t\t}\n\t\tif query.Query != SQLQuery(i) || query.SQL != SQLQueryText(SQLQuery(i)) {\n\t\t\treturn SQLProblemQueryPlan\n\t\t}\n\t\tif !query.EOF || !query.ErrCalled || query.Error != None || query.ErrorToken != 0 {\n\t\t\treturn SQLProblemEOF\n\t\t}\n\t\tif query.CloseCalls != 1 || query.CloseError != None || query.CloseErrorToken != 0 {\n\t\t\treturn SQLProblemClose\n\t\t}\n\t\trows, count := expectedSQLQueryRows(state, query.Query)\n\t\tif query.RowCount != count || query.Rows != rows {\n\t\t\treturn SQLProblemRows\n\t\t}\n\t\tif !validSQLQueryPlan(query.Query, query.Plan) {\n\t\t\treturn SQLProblemQueryPlan\n\t\t}\n\t\tif !validSQLQueryWork(query.Query, query.Work, count) {\n\t\t\treturn SQLProblemIndexWork\n\t\t}\n\t}\n\treturn SQLProblemNone\n}\n\nfunc expectedSQLQueryRows(state SQLState, query SQLQuery) ([3]SQLRow, uint8) {\n\tall := SQLLedgerRows(state)\n\tif query == SQLAllRows {\n\t\treturn all, 3\n\t}\n\tkey := \"\"\n\tswitch query {\n\tcase SQLBlueIndex, SQLBlueScan:\n\t\tkey = \"blue\"\n\tcase SQLRedIndex, SQLRedScan:\n\t\tkey = \"red\"\n\tcase SQLGreenIndex, SQLGreenScan:\n\t\tkey = \"green\"\n\t}\n\tvar selected [3]SQLRow\n\tvar count uint8\n\tfor _, row := range all {\n\t\tif row.Key == key {\n\t\t\tselected[count] = row\n\t\t\tcount++\n\t\t}\n\t}\n\treturn selected, count\n}\n\nfunc isSQLIndexQuery(query SQLQuery) bool {\n\treturn query == SQLBlueIndex || query == SQLRedIndex || query == SQLGreenIndex\n}\n\nfunc validSQLQueryPlan(query SQLQuery, plan SQLQueryPlan) bool {\n\tif plan.Table != \"t\" {\n\t\treturn false\n\t}\n\tif isSQLIndexQuery(query) {\n\t\treturn plan.Access == \"SEARCH\" && plan.Method == \"btree\" && plan.Index == \"by_k\" && plan.IndexCount == 1 && plan.Field == \"k\" && plan.FieldCount == 1 && plan.Recheck && len(plan.Reason) <= 1024\n\t}\n\treturn plan.Access == \"SCAN\" && plan.Method == \"\" && plan.Index == \"\" && plan.IndexCount == 0 && plan.Field == \"\" && plan.FieldCount == 0 && !plan.Recheck && plan.Reason == \"NOT INDEXED requested\"\n}\n\nfunc validSQLQueryWork(query SQLQuery, work SQLQueryWork, count uint8) bool {\n\tn := int64(count)\n\tif work.ReturnedRows != n {\n\t\treturn false\n\t}\n\tif isSQLIndexQuery(query) {\n\t\t// An empty green result still needs a real index-page probe. Nonempty\n\t\t// equality keys must each drive one candidate and base-row fetch.\n\t\treturn work.IndexPageReads > 0 && work.IndexPageReads <= 32 && work.IndexKeysRead == n && work.Candidates == n && work.RowIDSeeks == n && work.BaseRowFetches == n\n\t}\n\treturn work.IndexKeysRead == 0 && work.IndexPageReads == 0 && work.RowIDSeeks == 0 && work.Candidates == 0 && work.BaseRowFetches == 3\n}\n\nfunc isSQLPreDBFailure(which SQLCase) bool {\n\treturn which == SQLJournalBodySyncBefore || which == SQLJournalHeaderSyncBefore || which == SQLPublishDirectorySyncBefore\n}\n\nfunc isSQLPostDBFailure(which SQLCase) bool {\n\treturn which == SQLJournalUnlinkBefore || which == SQLFinalDirectorySyncBefore\n}\n\nfunc needsSQLRecovery(which SQLCase) bool {\n\treturn which == SQLIndexPrefix64 || which == SQLDBSyncBefore || which == SQLIndexPrefix64Error || which == SQLJournalUnlinkBefore || which == SQLIndexMisplaceMinus64Error\n}\n\nfunc validSQLRecovery(which SQLCase, recovery SQLRecoveryEvidence) bool {\n\tif which == SQLControl || isSQLPreDBFailure(which) || which == SQLFinalDirectorySyncBefore {\n\t\t// Control, pre-DB failures and final-directory uncertainty must not\n\t\t// call recovery, even as a no-op. NEW readback is not a durable ack.\n\t\treturn recovery == (SQLRecoveryEvidence{Identity: recovery.Identity, OwnersRetired: true})\n\t}\n\tif !needsSQLRecovery(which) || recovery.Calls != 1 || recovery.Error != None || recovery.ErrorToken != 0 || !recovery.OwnersRetired || recovery.JournalPresentAfter || recovery.Limits != (SQLRecoveryLimits{MaxJournalBytes: SQLJournalBytes, MaxRecords: SQLPageCount, MaxDatabaseBytes: SQLDatabaseBytes}) {\n\t\treturn false\n\t}\n\treturn recovery.JournalPresentBefore && recovery.Replayed && recovery.ReplayedPages == SQLPageCount && recovery.OriginalPages == SQLPageCount\n}\n\nfunc validSQLArtifacts(which SQLCase, artifacts [SQLArtifactCount]SQLArtifactEvidence) bool {\n\tfor i, artifact := range artifacts {\n\t\tif artifact.Role != SQLArtifactRole(i) {\n\t\t\treturn false\n\t\t}\n\t\tjournal := artifact.Role == SQLOriginalJournalArtifact || artifact.Role == SQLJournalCopyArtifact || artifact.Role == SQLReplayJournalArtifact\n\t\tif journal && (which == SQLControl || isSQLPreDBFailure(which) || which == SQLFinalDirectorySyncBefore) {\n\t\t\tif artifact.Before != (SQLArtifactStamp{}) || artifact.After != (SQLArtifactStamp{}) {\n\t\t\t\treturn false\n\t\t\t}\n\t\t\tcontinue\n\t\t}\n\t\tbefore, after := artifact.Before, artifact.After\n\t\tbytes := int64(SQLDatabaseBytes)\n\t\tif journal {\n\t\t\tbytes = SQLJournalBytes\n\t\t}\n\t\tif !validSQLArtifactStamp(before, bytes) || before.Identity != SQLArtifactName(artifact.Role) {\n\t\t\treturn false\n\t\t}\n\t\tif artifact.Role == SQLReplayJournalArtifact {\n\t\t\tif after != (SQLArtifactStamp{}) {\n\t\t\t\treturn false\n\t\t\t}\n\t\t} else {\n\t\t\tif !validSQLArtifactStamp(after, bytes) || before.Identity != after.Identity || before.Device != after.Device || before.Inode != after.Inode {\n\t\t\t\treturn false\n\t\t\t}\n\t\t\tif artifact.Role != SQLReplayArtifact && before != after {\n\t\t\t\treturn false\n\t\t\t}\n\t\t}\n\t\tfor j := 0; j < i; j++ {\n\t\t\tother := artifacts[j].Before\n\t\t\tif other.Exists && (before.Identity == other.Identity || (before.Device == other.Device && before.Inode == other.Inode)) {\n\t\t\t\treturn false\n\t\t\t}\n\t\t}\n\t}\n\tbaseline := artifacts[SQLBaselineArtifact].Before.SHA256\n\ttrial := artifacts[SQLTrialArtifact].Before.SHA256\n\tpost := artifacts[SQLPostCopyArtifact].Before.SHA256\n\treplayBefore := artifacts[SQLReplayArtifact].Before.SHA256\n\treplayAfter := artifacts[SQLReplayArtifact].After.SHA256\n\tif trial != post || trial != replayBefore {\n\t\treturn false\n\t}\n\tif isSQLPreDBFailure(which) {\n\t\treturn baseline == trial && replayAfter == baseline\n\t}\n\tif baseline == trial {\n\t\treturn false\n\t}\n\tif which == SQLControl || which == SQLFinalDirectorySyncBefore {\n\t\treturn replayAfter == trial\n\t}\n\tif !needsSQLRecovery(which) {\n\t\treturn false\n\t}\n\tjournal := artifacts[SQLOriginalJournalArtifact].Before.SHA256\n\treturn replayAfter == baseline && journal == artifacts[SQLJournalCopyArtifact].Before.SHA256 && journal == artifacts[SQLReplayJournalArtifact].Before.SHA256\n}\n\nfunc SQLArtifactName(role SQLArtifactRole) string {\n\tswitch role {\n\tcase SQLBaselineArtifact:\n\t\treturn \"baseline.input.db\"\n\tcase SQLTrialArtifact:\n\t\treturn \"trial.db\"\n\tcase SQLOriginalJournalArtifact:\n\t\treturn \"trial.db-journal\"\n\tcase SQLPostCopyArtifact:\n\t\treturn \"post-commit.input.db\"\n\tcase SQLJournalCopyArtifact:\n\t\treturn \"post-commit-journal.input.bin\"\n\tcase SQLReplayArtifact:\n\t\treturn \"replay.db\"\n\tcase SQLReplayJournalArtifact:\n\t\treturn \"replay.db-journal\"\n\tdefault:\n\t\treturn \"\"\n\t}\n}\n\nfunc validSQLArtifactStamp(stamp SQLArtifactStamp, bytes int64) bool {\n\tif !stamp.Exists || stamp.Device == 0 || stamp.Inode == 0 || stamp.Bytes != bytes || stamp.SHA256 == ([32]byte{}) || len(stamp.Identity) == 0 || stamp.Identity == \"0\" || len(stamp.Identity) > MaxIDLength {\n\t\treturn false\n\t}\n\tfor _, ch := range stamp.Identity {\n\t\tif ch < '!' || ch > '~' {\n\t\t\treturn false\n\t\t}\n\t}\n\treturn true\n}\n",
	rollbackFaultSQLModelTest: "package rollbackfault\n\nimport \"testing\"\n\n// SOURCE ONLY. These finite synthetic values do not create fixtures, descriptors,\n// native owners, SQL executions, Commit attempts, or recovery operations. Even\n// successful tests would establish only this evidence contract's consistency.\n\nfunc sqlLiteralRows(state SQLState) [3]SQLRow {\n\tif state == SQLNew {\n\t\treturn [3]SQLRow{\n\t\t\t{IDType: SQLInteger, ID: 1, KeyType: SQLText, Key: \"green\", ValueType: SQLText, Value: \"ONE\"},\n\t\t\t{IDType: SQLInteger, ID: 3, KeyType: SQLText, Key: \"red\", ValueType: SQLText, Value: \"three\"},\n\t\t\t{IDType: SQLInteger, ID: 4, KeyType: SQLText, Key: \"blue\", ValueType: SQLText, Value: \"four\"},\n\t\t}\n\t}\n\treturn [3]SQLRow{\n\t\t{IDType: SQLInteger, ID: 1, KeyType: SQLText, Key: \"red\", ValueType: SQLText, Value: \"one\"},\n\t\t{IDType: SQLInteger, ID: 2, KeyType: SQLText, Key: \"blue\", ValueType: SQLText, Value: \"two\"},\n\t\t{IDType: SQLInteger, ID: 3, KeyType: SQLText, Key: \"red\", ValueType: SQLText, Value: \"three\"},\n\t}\n}\n\nfunc sqlLiteralQueryText() [SQLQueryCount]string {\n\treturn [SQLQueryCount]string{\n\t\t\"SELECT id,k,v FROM t NOT INDEXED ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t WHERE k='blue' ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t NOT INDEXED WHERE k='blue' ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t WHERE k='red' ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t NOT INDEXED WHERE k='red' ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t WHERE k='green' ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t NOT INDEXED WHERE k='green' ORDER BY id\",\n\t}\n}\n\nfunc sqlSnapshot(identity Identity, owner, firstIterator uint64, state SQLState) SQLSnapshotEvidence {\n\tsnapshot := SQLSnapshotEvidence{\n\t\tIdentity: identity, Owner: owner, CloseCalls: 1, OwnerState: OwnerRetired,\n\t\tCheck: SQLCheckEvidence{Identity: identity, Owner: owner, Calls: 1, Pages: 3, Tables: 1, Indexes: 1, TableRows: 3, IndexEntries: 3},\n\t}\n\tall := sqlLiteralRows(state)\n\ttexts := sqlLiteralQueryText()\n\tkeys := [SQLQueryCount]string{\"\", \"blue\", \"blue\", \"red\", \"red\", \"green\", \"green\"}\n\tfor i := 0; i < SQLQueryCount; i++ {\n\t\tquery := SQLQueryEvidence{\n\t\t\tIdentity: identity, Query: SQLQuery(i), Owner: owner, Iterator: firstIterator + uint64(i), SQL: texts[i],\n\t\t\tEOF: true, ErrCalled: true, CloseCalls: 1,\n\t\t\tPlan: SQLQueryPlan{Access: \"SCAN\", Table: \"t\", Reason: \"NOT INDEXED requested\"},\n\t\t\tWork: SQLQueryWork{BaseRowFetches: 3},\n\t\t}\n\t\tfor _, row := range all {\n\t\t\tif i == 0 || row.Key == keys[i] {\n\t\t\t\tquery.Rows[query.RowCount] = row\n\t\t\t\tquery.RowCount++\n\t\t\t}\n\t\t}\n\t\tquery.Work.ReturnedRows = int64(query.RowCount)\n\t\tif i == 1 || i == 3 || i == 5 {\n\t\t\tn := int64(query.RowCount)\n\t\t\tquery.Plan = SQLQueryPlan{Access: \"SEARCH\", Table: \"t\", Method: \"btree\", Index: \"by_k\", IndexCount: 1, Field: \"k\", FieldCount: 1, Recheck: true, Reason: \"BINARY ASC leading equality prefix; rowid-ordered candidates; ANN and grams retain precedence\"}\n\t\t\tquery.Work = SQLQueryWork{IndexKeysRead: n, IndexPageReads: 1, RowIDSeeks: n, Candidates: n, BaseRowFetches: n, ReturnedRows: n}\n\t\t}\n\t\tsnapshot.Queries[i] = query\n\t}\n\treturn snapshot\n}\n\nfunc sqlReceipt(t *testing.T, plan Plan, events []Event) Receipt {\n\tt.Helper()\n\treceipt, problem := NewReceipt(plan)\n\tif problem != NoProblem {\n\t\tt.Fatalf(\"synthetic plan refused: %v\", problem)\n\t}\n\tfor _, event := range events {\n\t\tif receipt.Append(event) != NoProblem {\n\t\t\tbreak\n\t\t}\n\t}\n\treturn receipt\n}\n\nfunc sqlEvidenceFixture(t *testing.T, which SQLCase) (Plan, Receipt, SQLLedgerEvidence) {\n\tt.Helper()\n\tidentity := Identity{Suite: \"sql-suite-v1\", Source: \"source-v1\", Tool: \"tool-v1\", Binary: \"binary-v1\", Manifest: \"manifest-v1\", Trial: \"sql-trial-1\", Scope: \"sql-scope-1\", Profile: PlainRollback4096}\n\tplan, problem := SQLPlan(identity, which)\n\tif problem != NoProblem {\n\t\tt.Fatalf(\"synthetic SQL plan refused: %v\", problem)\n\t}\n\tstate := SQLOld\n\tif which == SQLControl || which == SQLFinalDirectorySyncBefore {\n\t\tstate = SQLNew\n\t}\n\tvar hashes [3][32]byte\n\thashes[0][0], hashes[1][0], hashes[2][0] = 11, 22, 33\n\tevidence := SQLLedgerEvidence{\n\t\tIdentity: identity, Case: which,\n\t\tBefore:    sqlSnapshot(identity, 101, 1, SQLOld),\n\t\tAfter:     sqlSnapshot(identity, 102, 8, state),\n\t\tOriginals: SQLOriginalEvidence{Frozen: hashes, Before: hashes, After: hashes, BeforeBytes: 12288, AfterBytes: 12288},\n\t\tRecovery:  SQLRecoveryEvidence{Identity: identity, OwnersRetired: true},\n\t\tLifecycle: SQLLifecycleEvidence{ArmCalls: 1, CommitCalls: 1, FinishCalls: 1, DisarmCalls: 1, WriterOpens: 1, ReopenCalls: 1, ReopenReadOnly: true, WriterRetiredBeforeRecovery: true, RecoveryFinishedBeforeReopen: true, CommitCapturedUnchanged: true},\n\t}\n\tif which == SQLIndexPrefix64 || which == SQLDBSyncBefore || which == SQLIndexPrefix64Error || which == SQLJournalUnlinkBefore || which == SQLIndexMisplaceMinus64Error {\n\t\tevidence.Recovery.Calls = 1\n\t\tevidence.Recovery.Limits = SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}\n\t\tevidence.Recovery.JournalPresentBefore = true\n\t\tevidence.Recovery.Replayed, evidence.Recovery.ReplayedPages = true, 3\n\t\tevidence.Recovery.OriginalPages = 3\n\t\tevidence.Lifecycle.PublicRecoveryRequired = true\n\t}\n\tpostDB := which == SQLJournalUnlinkBefore || which == SQLFinalDirectorySyncBefore\n\tif postDB {\n\t\tevidence.PostCommit = sqlSnapshot(identity, 103, 15, SQLNew)\n\t\tevidence.Lifecycle.ReopenCalls = 2\n\t}\n\tpreDB := which == SQLJournalBodySyncBefore || which == SQLJournalHeaderSyncBefore || which == SQLPublishDirectorySyncBefore\n\tif which != SQLControl {\n\t\tevidence.Lifecycle.PublicCodeIOErr, evidence.Lifecycle.PublicPoisoned = true, true\n\t\tevidence.Lifecycle.PublicShortWrite = which == SQLIndexPrefix64\n\t\tevidence.Lifecycle.PreDBOnlyTargetCause = preDB\n\t\tevidence.Lifecycle.PostDBOnlyTargetCause = postDB\n\t\tevidence.Lifecycle.PublicCommitUncertain = which == SQLFinalDirectorySyncBefore\n\t}\n\tlabels := [SQLArtifactCount]string{\"baseline.input.db\", \"trial.db\", \"trial.db-journal\", \"post-commit.input.db\", \"post-commit-journal.input.bin\", \"replay.db\", \"replay.db-journal\"}\n\tfor i := range evidence.Artifacts {\n\t\trole := SQLArtifactRole(i)\n\t\tevidence.Artifacts[i].Role = role\n\t\tjournal := role == SQLOriginalJournalArtifact || role == SQLJournalCopyArtifact || role == SQLReplayJournalArtifact\n\t\tif journal && (which == SQLControl || preDB || which == SQLFinalDirectorySyncBefore) {\n\t\t\tcontinue\n\t\t}\n\t\tstamp := SQLArtifactStamp{Identity: labels[i], Device: 1, Inode: uint64(i + 1), Exists: true, Bytes: 12288}\n\t\tstamp.SHA256[0] = 22\n\t\tif role == SQLBaselineArtifact || preDB {\n\t\t\tstamp.SHA256[0] = 11\n\t\t}\n\t\tif journal {\n\t\t\tstamp.Bytes, stamp.SHA256[0] = 16408, 33\n\t\t}\n\t\tevidence.Artifacts[i].Before, evidence.Artifacts[i].After = stamp, stamp\n\t\tif (which == SQLIndexPrefix64 || which == SQLDBSyncBefore || which == SQLIndexPrefix64Error || which == SQLJournalUnlinkBefore || which == SQLIndexMisplaceMinus64Error) && role == SQLReplayArtifact {\n\t\t\tevidence.Artifacts[i].After.SHA256[0] = 11\n\t\t}\n\t\tif role == SQLReplayJournalArtifact {\n\t\t\tevidence.Artifacts[i].After = SQLArtifactStamp{}\n\t\t}\n\t}\n\tif which == SQLIndexMisplaceMinus64Error {\n\t\tevidence.Misplacement = SQLMisplacementEvidence{\n\t\t\tWriter:        SQLMisplacedWrite{RequestedOffset: 8192, RequestedBytes: 4096, NativeOffset: 8128, NativeBytes: 4096, NativeCalls: 1, PayloadSHA256: [32]byte{44}},\n\t\t\tShiftedSHA256: [32]byte{44}, BaselineRangeSHA256: [32]byte{55}, BaselineTailSHA256: [32]byte{66}, UntouchedTailSHA256: [32]byte{66}, JournalOriginals: hashes, OnlyTargetCause: true,\n\t\t}\n\t}\n\treturn plan, sqlReceipt(t, plan, fixtureEvents(plan.Spec())), evidence\n}\n\nfunc sqlMustReject(t *testing.T, observation SQLObservation) {\n\tt.Helper()\n\tif observation.EvidenceConsistent || observation.ScenarioObserved || observation.Problem == SQLProblemNone || observation.PhysicalValidated() || observation.Pass() {\n\t\tt.Fatalf(\"invalid synthetic SQL evidence qualified: %+v\", observation)\n\t}\n}\n\nfunc TestSQLLedgerThreeFrozenScenariosRemainObservationOnly(t *testing.T) {\n\tfor _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore, SQLIndexPrefix64Error, SQLJournalBodySyncBefore, SQLJournalHeaderSyncBefore, SQLPublishDirectorySyncBefore, SQLJournalUnlinkBefore, SQLFinalDirectorySyncBefore} {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, which)\n\t\tobservation := ObserveSQLLedger(plan, receipt, evidence)\n\t\tif !observation.EvidenceConsistent || !observation.ScenarioObserved || observation.Problem != SQLProblemNone || observation.Adapter.Outcome != AdapterObserved {\n\t\t\tt.Fatalf(\"scenario %v: %+v\", which, observation)\n\t\t}\n\t\tif observation.PhysicalValidated() || observation.Pass() {\n\t\t\tt.Fatal(\"finite SQL evidence claimed physical acceptance\")\n\t\t}\n\t}\n\t// The public result itself is not a capability to claim physical acceptance.\n\tforged := SQLObservation{EvidenceConsistent: true, ScenarioObserved: true}\n\tif forged.PhysicalValidated() || forged.Pass() {\n\t\tt.Fatal(\"fabricated result claimed physical acceptance\")\n\t}\n}\n\nfunc TestSQLLedgerLiteralExpectationsAndQueriesAreDetached(t *testing.T) {\n\tfor _, state := range []SQLState{SQLOld, SQLNew} {\n\t\trows := SQLLedgerRows(state)\n\t\tif rows != sqlLiteralRows(state) {\n\t\t\tt.Fatalf(\"state %v differs from independent literal ledger\", state)\n\t\t}\n\t\trows[0].ID, rows[0].Key, rows[0].Value = 99, \"wrong\", \"wrong\"\n\t\tif SQLLedgerRows(state) != sqlLiteralRows(state) {\n\t\t\tt.Fatal(\"returned ledger aliases its frozen expectations\")\n\t\t}\n\t}\n\tif SQLLedgerRows(SQLStateUnknown) != ([3]SQLRow{}) || SQLLedgerRows(SQLState(255)) != ([3]SQLRow{}) {\n\t\tt.Fatal(\"unknown ledger state received valid rows\")\n\t}\n\tfor query, text := range sqlLiteralQueryText() {\n\t\tif SQLQueryText(SQLQuery(query)) != text {\n\t\t\tt.Fatalf(\"query %v lost frozen text\", query)\n\t\t}\n\t}\n\tif SQLQueryText(SQLQuery(255)) != \"\" {\n\t\tt.Fatal(\"unknown query received a valid statement\")\n\t}\n}\n\nfunc TestSQLLedgerPlansFreezeThreePageSelectorsAndLimits(t *testing.T) {\n\tif SQLCaseUnknown != 0 || SQLControl != 1 || SQLIndexPrefix64 != 2 || SQLDBSyncBefore != 3 || SQLIndexPrefix64Error != 4 || SQLJournalBodySyncBefore != 5 || SQLJournalHeaderSyncBefore != 6 || SQLPublishDirectorySyncBefore != 7 || SQLJournalUnlinkBefore != 8 || SQLFinalDirectorySyncBefore != 9 {\n\t\tt.Fatal(\"existing SQL case values changed\")\n\t}\n\tfor _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore, SQLIndexPrefix64Error, SQLJournalBodySyncBefore, SQLJournalHeaderSyncBefore, SQLPublishDirectorySyncBefore, SQLJournalUnlinkBefore, SQLFinalDirectorySyncBefore} {\n\t\tplan, _, _ := sqlEvidenceFixture(t, which)\n\t\tspec := plan.Spec()\n\t\tif spec.OriginalPages != 3 || spec.DirtyPages != 3 || spec.PageSize != 4096 || spec.SectorSize != 4096 || spec.MainOwner != 11 || spec.JournalOwner != 12 || spec.DirectoryOwner != 13 || spec.CleanupOwner != 14 {\n\t\t\tt.Fatalf(\"geometry/ownership changed: %+v\", spec)\n\t\t}\n\t\tfor i := 0; i < 3; i++ {\n\t\t\tread := spec.Calls[2+2*i]\n\t\t\trecord := spec.Calls[3+2*i]\n\t\t\tif read.Point != DBOriginalRead || read.Occurrence != uint8(i+1) || read.Offset != int64(i*4096) || record.Point != JournalRecord || record.Occurrence != uint8(i+1) || record.Offset != int64(4096+i*4104) {\n\t\t\t\tt.Fatalf(\"original page role %v was not independently selected\", i)\n\t\t\t}\n\t\t\tif which == SQLControl || which == SQLIndexPrefix64 || which == SQLDBSyncBefore || which == SQLIndexPrefix64Error || which == SQLJournalUnlinkBefore || which == SQLIndexMisplaceMinus64Error || which == SQLFinalDirectorySyncBefore {\n\t\t\t\twrite := spec.Calls[12+i]\n\t\t\t\tif write.Point != DBPage || write.Occurrence != uint8(i+1) || write.Offset != int64(i*4096) {\n\t\t\t\t\tt.Fatalf(\"dirty page role %v was not independently selected\", i)\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t\tswitch which {\n\t\tcase SQLControl:\n\t\t\tif spec.CallCount != 18 || spec.Action != Pass || spec.ExpectedCommit != CommitSucceeded || spec.Target != spec.Calls[14] {\n\t\t\t\tt.Fatal(\"control selector differs\")\n\t\t\t}\n\t\tcase SQLIndexPrefix64:\n\t\t\tif spec.CallCount != 15 || spec.Action != PrefixShortWrite || spec.Prefix != 64 || spec.PrefixError || spec.ExpectedCommit != CommitRecoveryRequired || spec.Target != spec.Calls[14] {\n\t\t\t\tt.Fatal(\"index prefix selector differs\")\n\t\t\t}\n\t\tcase SQLIndexPrefix64Error:\n\t\t\tif spec.CallCount != 15 || spec.Action != PrefixShortWrite || spec.Prefix != 64 || !spec.PrefixError || spec.ExpectedCommit != CommitRecoveryRequired || spec.Target != spec.Calls[14] {\n\t\t\t\tt.Fatal(\"index prefix error selector differs\")\n\t\t\t}\n\t\tcase SQLDBSyncBefore:\n\t\t\tif spec.CallCount != 16 || spec.Action != FailBefore || spec.ExpectedCommit != CommitRecoveryRequired || spec.Target != spec.Calls[15] {\n\t\t\t\tt.Fatal(\"sync-before selector differs\")\n\t\t\t}\n\t\tcase SQLJournalBodySyncBefore:\n\t\t\tif spec.CallCount != 11 || spec.Action != FailBefore || spec.ExpectedCommit != CommitFailed || spec.Target != spec.Calls[8] {\n\t\t\t\tt.Fatal(\"journal body sync-before selector differs\")\n\t\t\t}\n\t\tcase SQLJournalHeaderSyncBefore:\n\t\t\tif spec.CallCount != 13 || spec.Action != FailBefore || spec.ExpectedCommit != CommitFailed || spec.Target != spec.Calls[10] {\n\t\t\t\tt.Fatal(\"journal header sync-before selector differs\")\n\t\t\t}\n\t\tcase SQLPublishDirectorySyncBefore:\n\t\t\tif spec.CallCount != 14 || spec.Action != FailBefore || spec.ExpectedCommit != CommitFailed || spec.Target != spec.Calls[11] {\n\t\t\t\tt.Fatal(\"publish directory sync-before selector differs\")\n\t\t\t}\n\t\tcase SQLJournalUnlinkBefore:\n\t\t\tif spec.CallCount != 17 || spec.Action != FailBefore || spec.ExpectedCommit != CommitRecoveryRequired || spec.Target != spec.Calls[16] {\n\t\t\t\tt.Fatal(\"journal unlink-before selector differs\")\n\t\t\t}\n\t\tcase SQLFinalDirectorySyncBefore:\n\t\t\tif spec.CallCount != 18 || spec.Action != FailBefore || spec.ExpectedCommit != CommitUncertain || spec.Target != spec.Calls[17] {\n\t\t\t\tt.Fatal(\"final directory sync-before selector differs\")\n\t\t\t}\n\t\t}\n\t\tfor i := int(spec.CallCount); i < MaxCalls; i++ {\n\t\t\tif spec.Calls[i] != (Call{}) {\n\t\t\t\tt.Fatal(\"unused call tail was retained\")\n\t\t\t}\n\t\t}\n\t}\n\tplan, _, _ := sqlEvidenceFixture(t, SQLControl)\n\tif _, problem := SQLPlan(plan.Spec().Identity, SQLCaseUnknown); problem == NoProblem {\n\t\tt.Fatal(\"unknown SQL case admitted\")\n\t}\n\tif _, problem := SQLPlan(Identity{}, SQLControl); problem == NoProblem {\n\t\tt.Fatal(\"missing SQL identity admitted\")\n\t}\n}\n\nfunc TestSQLLedgerRejectsOtherwiseAdmittedNonFrozenFaultPlans(t *testing.T) {\n\tfor _, name := range []string{\"prefix65\", \"prefix-error\", \"earlier-db-page\", \"fourth-page-offset\", \"different-owner\", \"control-sync-target\"} {\n\t\twhich := SQLIndexPrefix64\n\t\tif name == \"control-sync-target\" {\n\t\t\twhich = SQLControl\n\t\t}\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, which)\n\t\tspec := plan.Spec()\n\t\tswitch name {\n\t\tcase \"prefix65\":\n\t\t\tspec.Prefix = 65\n\t\tcase \"prefix-error\":\n\t\t\tspec.PrefixError = true\n\t\tcase \"earlier-db-page\":\n\t\t\tspec.Target, spec.CallCount = spec.Calls[13], 14\n\t\t\tfor i := 14; i < MaxCalls; i++ {\n\t\t\t\tspec.Calls[i] = Call{}\n\t\t\t}\n\t\tcase \"fourth-page-offset\":\n\t\t\tspec.Calls[14].Offset = 12288\n\t\t\tspec.Target = spec.Calls[14]\n\t\tcase \"different-owner\":\n\t\t\tspec.MainOwner = 21\n\t\t\tfor i := 0; i < int(spec.CallCount); i++ {\n\t\t\t\tif spec.Calls[i].Role == Database {\n\t\t\t\t\tspec.Calls[i].Owner = 21\n\t\t\t\t}\n\t\t\t}\n\t\t\tspec.Target = spec.Calls[14]\n\t\tcase \"control-sync-target\":\n\t\t\tspec.Target = spec.Calls[15]\n\t\t}\n\t\tchanged, problem := NewPlan(spec)\n\t\tif problem != NoProblem {\n\t\t\tt.Fatalf(\"%s negative should pass generic admission: %v\", name, problem)\n\t\t}\n\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(changed, receipt, evidence)) })\n\t}\n}\n\nfunc TestSQLLedgerRejectsLogicalMixturesAndEqualCountWrongMembers(t *testing.T) {\n\tfor _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore, SQLIndexPrefix64Error, SQLJournalBodySyncBefore, SQLJournalHeaderSyncBefore, SQLPublishDirectorySyncBefore, SQLJournalUnlinkBefore, SQLFinalDirectorySyncBefore} {\n\t\tfor _, name := range []string{\"wrong-ledger\", \"oldrows-newindex\", \"newrows-oldindex\", \"equal-count-wrong-member\", \"equal-count-wrong-value\", \"index-scan-both-wrong\", \"wrong-id-type\", \"wrong-key-type\", \"wrong-value-type\", \"same-count-duplicate-id\", \"unreported-tail\", \"row-reordered\", \"empty-green-hidden-member\", \"before-not-old\"} {\n\t\t\tplan, receipt, evidence := sqlEvidenceFixture(t, which)\n\t\t\told := sqlSnapshot(evidence.Identity, 102, 8, SQLOld)\n\t\t\tnewState := sqlSnapshot(evidence.Identity, 102, 8, SQLNew)\n\t\t\tswitch name {\n\t\t\tcase \"wrong-ledger\":\n\t\t\t\tif which == SQLControl || which == SQLFinalDirectorySyncBefore {\n\t\t\t\t\tevidence.After = old\n\t\t\t\t} else {\n\t\t\t\t\tevidence.After = newState\n\t\t\t\t}\n\t\t\tcase \"oldrows-newindex\":\n\t\t\t\tevidence.After = old\n\t\t\t\tfor _, q := range []SQLQuery{SQLBlueIndex, SQLRedIndex, SQLGreenIndex} {\n\t\t\t\t\tevidence.After.Queries[q] = newState.Queries[q]\n\t\t\t\t}\n\t\t\tcase \"newrows-oldindex\":\n\t\t\t\tevidence.After = newState\n\t\t\t\tfor _, q := range []SQLQuery{SQLBlueIndex, SQLRedIndex, SQLGreenIndex} {\n\t\t\t\t\tevidence.After.Queries[q] = old.Queries[q]\n\t\t\t\t}\n\t\t\tcase \"equal-count-wrong-member\":\n\t\t\t\tevidence.After.Queries[SQLBlueIndex].Rows[0].ID = 77\n\t\t\tcase \"equal-count-wrong-value\":\n\t\t\t\tevidence.After.Queries[SQLAllRows].Rows[0].Value = \"one!\"\n\t\t\tcase \"index-scan-both-wrong\":\n\t\t\t\tevidence.After.Queries[SQLBlueIndex].Rows[0].ID = 88\n\t\t\t\tevidence.After.Queries[SQLBlueScan].Rows[0].ID = 88\n\t\t\tcase \"wrong-id-type\":\n\t\t\t\tevidence.After.Queries[SQLAllRows].Rows[0].IDType = SQLText\n\t\t\tcase \"wrong-key-type\":\n\t\t\t\tevidence.After.Queries[SQLAllRows].Rows[0].KeyType = SQLInteger\n\t\t\tcase \"wrong-value-type\":\n\t\t\t\tevidence.After.Queries[SQLAllRows].Rows[0].ValueType = SQLTypeUnknown\n\t\t\tcase \"same-count-duplicate-id\":\n\t\t\t\tevidence.After.Queries[SQLAllRows].Rows[1].ID = evidence.After.Queries[SQLAllRows].Rows[0].ID\n\t\t\tcase \"unreported-tail\":\n\t\t\t\tevidence.After.Queries[SQLBlueIndex].Rows[2] = old.Queries[SQLAllRows].Rows[0]\n\t\t\tcase \"row-reordered\":\n\t\t\t\tevidence.After.Queries[SQLAllRows].Rows[0], evidence.After.Queries[SQLAllRows].Rows[1] = evidence.After.Queries[SQLAllRows].Rows[1], evidence.After.Queries[SQLAllRows].Rows[0]\n\t\t\tcase \"empty-green-hidden-member\":\n\t\t\t\tevidence.Before.Queries[SQLGreenIndex].Rows[0] = old.Queries[SQLAllRows].Rows[0]\n\t\t\tcase \"before-not-old\":\n\t\t\t\tevidence.Before = sqlSnapshot(evidence.Identity, 101, 1, SQLNew)\n\t\t\t}\n\t\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t\t}\n\t}\n}\n\nfunc TestSQLLedgerRejectsPlanClaimsWithoutActualIndexWorkAndIndependentScans(t *testing.T) {\n\tfor _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore, SQLIndexPrefix64Error, SQLJournalBodySyncBefore, SQLJournalHeaderSyncBefore, SQLPublishDirectorySyncBefore, SQLJournalUnlinkBefore, SQLFinalDirectorySyncBefore} {\n\t\tfor _, name := range []string{\"scan-disguised-as-search\", \"wrong-method\", \"wrong-table\", \"wrong-index\", \"extra-index\", \"wrong-field\", \"extra-field\", \"no-recheck\", \"no-index-pages\", \"no-index-keys\", \"no-rowid-seek\", \"no-base-fetch\", \"no-candidates\", \"wrong-return-count\", \"negative-work\", \"unbounded-work\", \"empty-green-no-probe\", \"scan-uses-index\", \"scan-fewer-rows\", \"scan-has-candidates\", \"scan-fake-reason\", \"scan-has-method\", \"copied-index-query\", \"changed-sql\", \"reordered-query\", \"duplicate-iterator\"} {\n\t\t\tplan, receipt, evidence := sqlEvidenceFixture(t, which)\n\t\t\tquery := &evidence.After.Queries[SQLBlueIndex]\n\t\t\tswitch name {\n\t\t\tcase \"scan-disguised-as-search\":\n\t\t\t\tquery.Plan.Access = \"SCAN\"\n\t\t\tcase \"wrong-method\":\n\t\t\t\tquery.Plan.Method = \"rowid\"\n\t\t\tcase \"wrong-table\":\n\t\t\t\tquery.Plan.Table = \"other\"\n\t\t\tcase \"wrong-index\":\n\t\t\t\tquery.Plan.Index = \"other\"\n\t\t\tcase \"extra-index\":\n\t\t\t\tquery.Plan.IndexCount = 2\n\t\t\tcase \"wrong-field\":\n\t\t\t\tquery.Plan.Field = \"v\"\n\t\t\tcase \"extra-field\":\n\t\t\t\tquery.Plan.FieldCount = 2\n\t\t\tcase \"no-recheck\":\n\t\t\t\tquery.Plan.Recheck = false\n\t\t\tcase \"no-index-pages\":\n\t\t\t\tquery.Work.IndexPageReads = 0\n\t\t\tcase \"no-index-keys\":\n\t\t\t\tquery.Work.IndexKeysRead = 0\n\t\t\tcase \"no-rowid-seek\":\n\t\t\t\tquery.Work.RowIDSeeks = 0\n\t\t\tcase \"no-base-fetch\":\n\t\t\t\tquery.Work.BaseRowFetches = 0\n\t\t\tcase \"no-candidates\":\n\t\t\t\tquery.Work.Candidates = 0\n\t\t\tcase \"wrong-return-count\":\n\t\t\t\tquery.Work.ReturnedRows = 2\n\t\t\tcase \"negative-work\":\n\t\t\t\tquery.Work.IndexPageReads = -1\n\t\t\tcase \"unbounded-work\":\n\t\t\t\tquery.Work.IndexPageReads = 33\n\t\t\tcase \"empty-green-no-probe\":\n\t\t\t\tevidence.Before.Queries[SQLGreenIndex].Work.IndexPageReads = 0\n\t\t\tcase \"scan-uses-index\":\n\t\t\t\tevidence.After.Queries[SQLBlueScan].Work.IndexKeysRead = 1\n\t\t\tcase \"scan-fewer-rows\":\n\t\t\t\tevidence.After.Queries[SQLBlueScan].Work.BaseRowFetches = 2\n\t\t\tcase \"scan-has-candidates\":\n\t\t\t\tevidence.After.Queries[SQLBlueScan].Work.Candidates = 1\n\t\t\tcase \"scan-fake-reason\":\n\t\t\t\tevidence.After.Queries[SQLBlueScan].Plan.Reason = \"no index\"\n\t\t\tcase \"scan-has-method\":\n\t\t\t\tevidence.After.Queries[SQLBlueScan].Plan.Method = \"btree\"\n\t\t\tcase \"copied-index-query\":\n\t\t\t\tevidence.After.Queries[SQLBlueScan] = evidence.After.Queries[SQLBlueIndex]\n\t\t\tcase \"changed-sql\":\n\t\t\t\tquery.SQL = \"EXPLAIN QUERY PLAN SELECT id,k,v FROM t WHERE k='blue' ORDER BY id\"\n\t\t\tcase \"reordered-query\":\n\t\t\t\tevidence.After.Queries[SQLBlueScan], evidence.After.Queries[SQLRedScan] = evidence.After.Queries[SQLRedScan], evidence.After.Queries[SQLBlueScan]\n\t\t\tcase \"duplicate-iterator\":\n\t\t\t\tevidence.After.Queries[SQLBlueScan].Iterator = query.Iterator\n\t\t\t}\n\t\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t\t}\n\t}\n}\n\nfunc TestSQLLedgerRequiresEOFErrOnceCloseAndIndependentRetiredOwners(t *testing.T) {\n\tfor _, name := range []string{\"not-eof\", \"err-not-called\", \"err-present\", \"err-token\", \"missing-row-close\", \"duplicate-row-close\", \"row-close-error\", \"row-close-token\", \"missing-db-close\", \"duplicate-db-close\", \"db-close-error\", \"db-close-token\", \"db-owner-live\", \"same-owner\", \"writer-owner\", \"zero-owner\", \"zero-iterator\", \"iterator-reused-across-opens\", \"query-owner-mismatch\", \"check-owner-mismatch\", \"second-arm\", \"missing-arm\", \"second-commit\", \"missing-commit\", \"missing-finish\", \"second-finish\", \"missing-disarm\", \"second-disarm\", \"reconstructed-commit-error\", \"missing-public-code\", \"missing-public-recovery\", \"missing-public-shortwrite\", \"rollback-after-commit\", \"second-writer-open\", \"second-reopen\", \"writable-reopen\", \"unretired-before-recovery\", \"reopen-before-recovery\"} {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, SQLIndexPrefix64)\n\t\tquery := &evidence.After.Queries[SQLAllRows]\n\t\tswitch name {\n\t\tcase \"not-eof\":\n\t\t\tquery.EOF = false\n\t\tcase \"err-not-called\":\n\t\t\tquery.ErrCalled = false\n\t\tcase \"err-present\":\n\t\t\tquery.Error, query.ErrorToken = Underlying, 10\n\t\tcase \"err-token\":\n\t\t\tquery.ErrorToken = 10\n\t\tcase \"missing-row-close\":\n\t\t\tquery.CloseCalls = 0\n\t\tcase \"duplicate-row-close\":\n\t\t\tquery.CloseCalls = 2\n\t\tcase \"row-close-error\":\n\t\t\tquery.CloseError, query.CloseErrorToken = Underlying, 10\n\t\tcase \"row-close-token\":\n\t\t\tquery.CloseErrorToken = 10\n\t\tcase \"missing-db-close\":\n\t\t\tevidence.After.CloseCalls = 0\n\t\tcase \"duplicate-db-close\":\n\t\t\tevidence.After.CloseCalls = 2\n\t\tcase \"db-close-error\":\n\t\t\tevidence.After.CloseError, evidence.After.CloseErrorToken = Underlying, 10\n\t\tcase \"db-close-token\":\n\t\t\tevidence.After.CloseErrorToken = 10\n\t\tcase \"db-owner-live\":\n\t\t\tevidence.After.OwnerState = OwnerLive\n\t\tcase \"same-owner\":\n\t\t\tevidence.After.Owner = evidence.Before.Owner\n\t\tcase \"writer-owner\":\n\t\t\tevidence.After.Owner = plan.Spec().MainOwner\n\t\tcase \"zero-owner\":\n\t\t\tevidence.After.Owner = 0\n\t\tcase \"zero-iterator\":\n\t\t\tquery.Iterator = 0\n\t\tcase \"iterator-reused-across-opens\":\n\t\t\tquery.Iterator = evidence.Before.Queries[SQLAllRows].Iterator\n\t\tcase \"query-owner-mismatch\":\n\t\t\tquery.Owner = 99\n\t\tcase \"check-owner-mismatch\":\n\t\t\tevidence.After.Check.Owner = 99\n\t\tcase \"second-arm\":\n\t\t\tevidence.Lifecycle.ArmCalls = 2\n\t\tcase \"missing-arm\":\n\t\t\tevidence.Lifecycle.ArmCalls = 0\n\t\tcase \"second-commit\":\n\t\t\tevidence.Lifecycle.CommitCalls = 2\n\t\tcase \"missing-commit\":\n\t\t\tevidence.Lifecycle.CommitCalls = 0\n\t\tcase \"missing-finish\":\n\t\t\tevidence.Lifecycle.FinishCalls = 0\n\t\tcase \"second-finish\":\n\t\t\tevidence.Lifecycle.FinishCalls = 2\n\t\tcase \"missing-disarm\":\n\t\t\tevidence.Lifecycle.DisarmCalls = 0\n\t\tcase \"second-disarm\":\n\t\t\tevidence.Lifecycle.DisarmCalls = 2\n\t\tcase \"reconstructed-commit-error\":\n\t\t\tevidence.Lifecycle.CommitCapturedUnchanged = false\n\t\tcase \"missing-public-code\":\n\t\t\tevidence.Lifecycle.PublicCodeIOErr = false\n\t\tcase \"missing-public-recovery\":\n\t\t\tevidence.Lifecycle.PublicRecoveryRequired = false\n\t\tcase \"missing-public-shortwrite\":\n\t\t\tevidence.Lifecycle.PublicShortWrite = false\n\t\tcase \"rollback-after-commit\":\n\t\t\tevidence.Lifecycle.RollbackCalls = 1\n\t\tcase \"second-writer-open\":\n\t\t\tevidence.Lifecycle.WriterOpens = 2\n\t\tcase \"second-reopen\":\n\t\t\tevidence.Lifecycle.ReopenCalls = 2\n\t\tcase \"writable-reopen\":\n\t\t\tevidence.Lifecycle.ReopenReadOnly = false\n\t\tcase \"unretired-before-recovery\":\n\t\t\tevidence.Lifecycle.WriterRetiredBeforeRecovery = false\n\t\tcase \"reopen-before-recovery\":\n\t\t\tevidence.Lifecycle.RecoveryFinishedBeforeReopen = false\n\t\t}\n\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t}\n}\n\nfunc TestSQLLedgerRequiresCheckHashesAndExactRecoveryEvidence(t *testing.T) {\n\tfor _, name := range []string{\"missing-check\", \"duplicate-check\", \"check-error\", \"check-token\", \"pages\", \"tables\", \"indexes\", \"rows\", \"entries\", \"extensions\", \"free-pages\", \"reserved-pages\", \"before-hash-changed\", \"after-hash-changed\", \"zero-hash\", \"before-size\", \"after-size\", \"missing-recovery\", \"second-recovery\", \"recovery-error\", \"recovery-token\", \"journal-byte-limit\", \"record-limit\", \"database-limit\", \"missing-hot-journal\", \"journal-survives\", \"missing-replay\", \"too-few-replay-pages\", \"too-many-replay-pages\", \"wrong-original-pages\", \"recovery-owner-live\", \"physical-claim\"} {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, SQLIndexPrefix64)\n\t\tswitch name {\n\t\tcase \"missing-check\":\n\t\t\tevidence.After.Check.Calls = 0\n\t\tcase \"duplicate-check\":\n\t\t\tevidence.After.Check.Calls = 2\n\t\tcase \"check-error\":\n\t\t\tevidence.After.Check.Error, evidence.After.Check.ErrorToken = Underlying, 5\n\t\tcase \"check-token\":\n\t\t\tevidence.After.Check.ErrorToken = 5\n\t\tcase \"pages\":\n\t\t\tevidence.After.Check.Pages = 4\n\t\tcase \"tables\":\n\t\t\tevidence.After.Check.Tables = 2\n\t\tcase \"indexes\":\n\t\t\tevidence.After.Check.Indexes = 0\n\t\tcase \"rows\":\n\t\t\tevidence.After.Check.TableRows = 2\n\t\tcase \"entries\":\n\t\t\tevidence.After.Check.IndexEntries = 2\n\t\tcase \"extensions\":\n\t\t\tevidence.After.Check.ExtensionIndexes = 1\n\t\tcase \"free-pages\":\n\t\t\tevidence.After.Check.FreePages = 1\n\t\tcase \"reserved-pages\":\n\t\t\tevidence.After.Check.ReservedPages = 1\n\t\tcase \"before-hash-changed\":\n\t\t\tevidence.Originals.Before[1][0]++\n\t\tcase \"after-hash-changed\":\n\t\t\tevidence.Originals.After[2][0]++\n\t\tcase \"zero-hash\":\n\t\t\tevidence.Originals.Frozen[1], evidence.Originals.Before[1], evidence.Originals.After[1] = [32]byte{}, [32]byte{}, [32]byte{}\n\t\tcase \"before-size\":\n\t\t\tevidence.Originals.BeforeBytes--\n\t\tcase \"after-size\":\n\t\t\tevidence.Originals.AfterBytes++\n\t\tcase \"missing-recovery\":\n\t\t\tevidence.Recovery.Calls = 0\n\t\tcase \"second-recovery\":\n\t\t\tevidence.Recovery.Calls = 2\n\t\tcase \"recovery-error\":\n\t\t\tevidence.Recovery.Error, evidence.Recovery.ErrorToken = Underlying, 5\n\t\tcase \"recovery-token\":\n\t\t\tevidence.Recovery.ErrorToken = 5\n\t\tcase \"journal-byte-limit\":\n\t\t\tevidence.Recovery.Limits.MaxJournalBytes++\n\t\tcase \"record-limit\":\n\t\t\tevidence.Recovery.Limits.MaxRecords = 4\n\t\tcase \"database-limit\":\n\t\t\tevidence.Recovery.Limits.MaxDatabaseBytes = 16384\n\t\tcase \"missing-hot-journal\":\n\t\t\tevidence.Recovery.JournalPresentBefore = false\n\t\tcase \"journal-survives\":\n\t\t\tevidence.Recovery.JournalPresentAfter = true\n\t\tcase \"missing-replay\":\n\t\t\tevidence.Recovery.Replayed = false\n\t\tcase \"too-few-replay-pages\":\n\t\t\tevidence.Recovery.ReplayedPages = 2\n\t\tcase \"too-many-replay-pages\":\n\t\t\tevidence.Recovery.ReplayedPages = 4\n\t\tcase \"wrong-original-pages\":\n\t\t\tevidence.Recovery.OriginalPages = 2\n\t\tcase \"recovery-owner-live\":\n\t\t\tevidence.Recovery.OwnersRetired = false\n\t\tcase \"physical-claim\":\n\t\t\tevidence.PhysicalClaim = true\n\t\t}\n\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t}\n\tplan, receipt, evidence := sqlEvidenceFixture(t, SQLControl)\n\tevidence.Recovery.Replayed, evidence.Recovery.ReplayedPages = true, 3\n\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))\n}\n\nfunc TestSQLLedgerRetainsOriginalsAndRejectsAliasedOrMixedReplayArtifacts(t *testing.T) {\n\tfor _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore, SQLIndexPrefix64Error} {\n\t\tfor _, name := range []string{\"wrong-role\", \"absent-baseline\", \"absent-trial\", \"absent-replay\", \"changed-baseline\", \"changed-trial\", \"changed-post-copy\", \"replay-aliases-trial\", \"post-aliases-trial\", \"changed-inode\", \"changed-device\", \"zero-device\", \"zero-inode\", \"zero-hash\", \"wrong-size\", \"wrong-label\", \"same-post-and-baseline\", \"wrong-replay-input\", \"wrong-replay-output\", \"same-old-and-new-hash\"} {\n\t\t\tplan, receipt, evidence := sqlEvidenceFixture(t, which)\n\t\t\tswitch name {\n\t\t\tcase \"wrong-role\":\n\t\t\t\tevidence.Artifacts[SQLTrialArtifact].Role = SQLPostCopyArtifact\n\t\t\tcase \"absent-baseline\":\n\t\t\t\tevidence.Artifacts[SQLBaselineArtifact].Before = SQLArtifactStamp{}\n\t\t\tcase \"absent-trial\":\n\t\t\t\tevidence.Artifacts[SQLTrialArtifact].Before = SQLArtifactStamp{}\n\t\t\tcase \"absent-replay\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].Before = SQLArtifactStamp{}\n\t\t\tcase \"changed-baseline\":\n\t\t\t\tevidence.Artifacts[SQLBaselineArtifact].After.SHA256[0]++\n\t\t\tcase \"changed-trial\":\n\t\t\t\tevidence.Artifacts[SQLTrialArtifact].After.SHA256[0]++\n\t\t\tcase \"changed-post-copy\":\n\t\t\t\tevidence.Artifacts[SQLPostCopyArtifact].After.SHA256[0]++\n\t\t\tcase \"replay-aliases-trial\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].Before.Inode, evidence.Artifacts[SQLReplayArtifact].After.Inode = evidence.Artifacts[SQLTrialArtifact].Before.Inode, evidence.Artifacts[SQLTrialArtifact].Before.Inode\n\t\t\tcase \"post-aliases-trial\":\n\t\t\t\tevidence.Artifacts[SQLPostCopyArtifact].Before.Inode, evidence.Artifacts[SQLPostCopyArtifact].After.Inode = evidence.Artifacts[SQLTrialArtifact].Before.Inode, evidence.Artifacts[SQLTrialArtifact].Before.Inode\n\t\t\tcase \"changed-inode\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].After.Inode++\n\t\t\tcase \"changed-device\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].After.Device++\n\t\t\tcase \"zero-device\":\n\t\t\t\tevidence.Artifacts[SQLBaselineArtifact].Before.Device = 0\n\t\t\tcase \"zero-inode\":\n\t\t\t\tevidence.Artifacts[SQLBaselineArtifact].Before.Inode = 0\n\t\t\tcase \"zero-hash\":\n\t\t\t\tevidence.Artifacts[SQLBaselineArtifact].Before.SHA256 = [32]byte{}\n\t\t\tcase \"wrong-size\":\n\t\t\t\tevidence.Artifacts[SQLTrialArtifact].Before.Bytes++\n\t\t\tcase \"wrong-label\":\n\t\t\t\tevidence.Artifacts[SQLTrialArtifact].Before.Identity = \"baseline.input.db\"\n\t\t\tcase \"same-post-and-baseline\":\n\t\t\t\tevidence.Artifacts[SQLPostCopyArtifact].Before.SHA256, evidence.Artifacts[SQLPostCopyArtifact].After.SHA256 = evidence.Artifacts[SQLBaselineArtifact].Before.SHA256, evidence.Artifacts[SQLBaselineArtifact].Before.SHA256\n\t\t\tcase \"wrong-replay-input\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].Before.SHA256[0]++\n\t\t\tcase \"wrong-replay-output\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].After.SHA256[0]++\n\t\t\tcase \"same-old-and-new-hash\":\n\t\t\t\tfor _, role := range []SQLArtifactRole{SQLTrialArtifact, SQLPostCopyArtifact, SQLReplayArtifact} {\n\t\t\t\t\tevidence.Artifacts[role].Before.SHA256 = evidence.Artifacts[SQLBaselineArtifact].Before.SHA256\n\t\t\t\t\tevidence.Artifacts[role].After.SHA256 = evidence.Artifacts[SQLBaselineArtifact].Before.SHA256\n\t\t\t\t}\n\t\t\t}\n\t\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t\t}\n\t}\n\tfor _, name := range []string{\"original-journal-removed\", \"original-journal-changed\", \"journal-copy-removed\", \"journal-copy-changed\", \"replay-journal-missing\", \"replay-journal-survives\", \"replay-journal-aliased\", \"different-journal-copy\", \"different-replay-journal\"} {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, SQLIndexPrefix64)\n\t\tswitch name {\n\t\tcase \"original-journal-removed\":\n\t\t\tevidence.Artifacts[SQLOriginalJournalArtifact].After = SQLArtifactStamp{}\n\t\tcase \"original-journal-changed\":\n\t\t\tevidence.Artifacts[SQLOriginalJournalArtifact].After.SHA256[0]++\n\t\tcase \"journal-copy-removed\":\n\t\t\tevidence.Artifacts[SQLJournalCopyArtifact].After = SQLArtifactStamp{}\n\t\tcase \"journal-copy-changed\":\n\t\t\tevidence.Artifacts[SQLJournalCopyArtifact].After.SHA256[0]++\n\t\tcase \"replay-journal-missing\":\n\t\t\tevidence.Artifacts[SQLReplayJournalArtifact].Before = SQLArtifactStamp{}\n\t\tcase \"replay-journal-survives\":\n\t\t\tevidence.Artifacts[SQLReplayJournalArtifact].After = evidence.Artifacts[SQLReplayJournalArtifact].Before\n\t\tcase \"replay-journal-aliased\":\n\t\t\tevidence.Artifacts[SQLReplayJournalArtifact].Before.Inode = evidence.Artifacts[SQLOriginalJournalArtifact].Before.Inode\n\t\tcase \"different-journal-copy\":\n\t\t\tevidence.Artifacts[SQLJournalCopyArtifact].Before.SHA256[0], evidence.Artifacts[SQLJournalCopyArtifact].After.SHA256[0] = 88, 88\n\t\tcase \"different-replay-journal\":\n\t\t\tevidence.Artifacts[SQLReplayJournalArtifact].Before.SHA256[0] = 88\n\t\t}\n\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t}\n\tplan, receipt, evidence := sqlEvidenceFixture(t, SQLControl)\n\tevidence.Recovery.Calls = 1\n\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))\n\tplan, receipt, evidence = sqlEvidenceFixture(t, SQLControl)\n\tevidence.Artifacts[SQLOriginalJournalArtifact].Before.Exists = true\n\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))\n}\n\nfunc TestSQLLedgerSnapshotAdmissionRejectsBeforeCommitMismatches(t *testing.T) {\n\t_, _, evidence := sqlEvidenceFixture(t, SQLControl)\n\tif problem := ValidateSQLSnapshot(evidence.Before, evidence.Identity, SQLOld); problem != SQLProblemNone {\n\t\tt.Fatalf(\"independent old snapshot refused: %v\", problem)\n\t}\n\tif ValidateSQLSnapshot(evidence.Before, evidence.Identity, SQLStateUnknown) == SQLProblemNone {\n\t\tt.Fatal(\"unknown snapshot state admitted\")\n\t}\n\tif ValidateSQLSnapshot(evidence.Before, evidence.Identity, SQLNew) == SQLProblemNone {\n\t\tt.Fatal(\"old snapshot admitted as new\")\n\t}\n\tchanged := evidence.Before\n\tchanged.Identity.Trial = \"other-trial\"\n\tif ValidateSQLSnapshot(changed, evidence.Identity, SQLOld) == SQLProblemNone {\n\t\tt.Fatal(\"snapshot identity mismatch admitted\")\n\t}\n\tchanged = evidence.Before\n\tchanged.Owner = 0\n\tif ValidateSQLSnapshot(changed, evidence.Identity, SQLOld) == SQLProblemNone {\n\t\tt.Fatal(\"zero snapshot owner admitted\")\n\t}\n\tchanged = evidence.Before\n\tchanged.Queries[SQLRedIndex].Rows[0].ID = 9\n\tif ValidateSQLSnapshot(changed, evidence.Identity, SQLOld) == SQLProblemNone {\n\t\tt.Fatal(\"equal-count wrong index member admitted\")\n\t}\n}\n\nfunc TestSQLLedgerBindsSourceToolBinaryManifestTrialAndScopeEverywhere(t *testing.T) {\n\tfor _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore, SQLIndexPrefix64Error, SQLJournalBodySyncBefore, SQLJournalHeaderSyncBefore, SQLPublishDirectorySyncBefore, SQLJournalUnlinkBefore, SQLFinalDirectorySyncBefore} {\n\t\tfor _, where := range []string{\"top\", \"before\", \"after\", \"query\", \"check\", \"recovery\"} {\n\t\t\tfor _, field := range []string{\"suite\", \"source\", \"tool\", \"binary\", \"manifest\", \"trial\", \"scope\", \"profile\"} {\n\t\t\t\tplan, receipt, evidence := sqlEvidenceFixture(t, which)\n\t\t\t\tidentity := &evidence.Identity\n\t\t\t\tswitch where {\n\t\t\t\tcase \"before\":\n\t\t\t\t\tidentity = &evidence.Before.Identity\n\t\t\t\tcase \"after\":\n\t\t\t\t\tidentity = &evidence.After.Identity\n\t\t\t\tcase \"query\":\n\t\t\t\t\tidentity = &evidence.After.Queries[SQLBlueIndex].Identity\n\t\t\t\tcase \"check\":\n\t\t\t\t\tidentity = &evidence.After.Check.Identity\n\t\t\t\tcase \"recovery\":\n\t\t\t\t\tidentity = &evidence.Recovery.Identity\n\t\t\t\t}\n\t\t\t\tswitch field {\n\t\t\t\tcase \"suite\":\n\t\t\t\t\tidentity.Suite = \"another-suite\"\n\t\t\t\tcase \"source\":\n\t\t\t\t\tidentity.Source = \"another-source\"\n\t\t\t\tcase \"tool\":\n\t\t\t\t\tidentity.Tool = \"another-tool\"\n\t\t\t\tcase \"binary\":\n\t\t\t\t\tidentity.Binary = \"another-binary\"\n\t\t\t\tcase \"manifest\":\n\t\t\t\t\tidentity.Manifest = \"another-manifest\"\n\t\t\t\tcase \"trial\":\n\t\t\t\t\tidentity.Trial = \"another-trial\"\n\t\t\t\tcase \"scope\":\n\t\t\t\t\tidentity.Scope = \"another-scope\"\n\t\t\t\tcase \"profile\":\n\t\t\t\t\tidentity.Profile = ProfileUnknown\n\t\t\t\t}\n\t\t\t\tt.Run(where+\"/\"+field, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t\t\t}\n\t\t}\n\t}\n}\n\nfunc TestSQLLedgerRevalidatesReceiptRatherThanTrustingAnObservation(t *testing.T) {\n\tfor _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore, SQLIndexPrefix64Error, SQLJournalBodySyncBefore, SQLJournalHeaderSyncBefore, SQLPublishDirectorySyncBefore, SQLJournalUnlinkBefore, SQLFinalDirectorySyncBefore} {\n\t\tplan, _, evidence := sqlEvidenceFixture(t, which)\n\t\tevents := fixtureEvents(plan.Spec())\n\t\tfor end := 0; end < len(events); end++ {\n\t\t\tsqlMustReject(t, ObserveSQLLedger(plan, sqlReceipt(t, plan, events[:end]), evidence))\n\t\t}\n\t\tfor _, name := range []string{\"second-commit\", \"missing-owner\", \"owner-live\", \"wrong-commit\", \"missing-target-error\", \"target-normalization\", \"wrong-count\", \"physical-validation\", \"foreign-trial\"} {\n\t\t\tchanged := append([]Event(nil), events...)\n\t\t\tfor i := range changed {\n\t\t\t\tswitch name {\n\t\t\t\tcase \"second-commit\":\n\t\t\t\t\tif changed[i].Kind == CommitReturned {\n\t\t\t\t\t\tchanged = append(changed[:i+1], append([]Event{changed[i]}, changed[i+1:]...)...)\n\t\t\t\t\t\trenumber(changed, plan.Spec().Identity)\n\t\t\t\t\t\tgoto evaluate\n\t\t\t\t\t}\n\t\t\t\tcase \"missing-owner\":\n\t\t\t\t\tif changed[i].Kind == OwnersObserved {\n\t\t\t\t\t\tchanged[i].Owners.Main = OwnerUnknown\n\t\t\t\t\t}\n\t\t\t\tcase \"owner-live\":\n\t\t\t\t\tif changed[i].Kind == OwnersObserved {\n\t\t\t\t\t\tchanged[i].Owners.Main = OwnerLive\n\t\t\t\t\t}\n\t\t\t\tcase \"wrong-commit\":\n\t\t\t\t\tif changed[i].Kind == CommitReturned {\n\t\t\t\t\t\tchanged[i].Commit.Class = CommitUncertain\n\t\t\t\t\t\tif which == SQLFinalDirectorySyncBefore {\n\t\t\t\t\t\t\tchanged[i].Commit.Class = CommitRecoveryRequired\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n\t\t\t\tcase \"missing-target-error\":\n\t\t\t\t\tif changed[i].Kind == CommitReturned {\n\t\t\t\t\t\tchanged[i].Commit.TargetError = None\n\t\t\t\t\t\tif which == SQLControl {\n\t\t\t\t\t\t\tchanged[i].Commit.TargetError = ShortWrite\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n\t\t\t\tcase \"target-normalization\":\n\t\t\t\t\tif changed[i].Kind == Returned && changed[i].Call == plan.Spec().Target {\n\t\t\t\t\t\tchanged[i].Result.NormalizedError = None\n\t\t\t\t\t\tif which == SQLControl {\n\t\t\t\t\t\t\tchanged[i].Result.NormalizedError = ShortWrite\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n\t\t\t\tcase \"wrong-count\":\n\t\t\t\t\tif changed[i].Kind == Returned && changed[i].Call == plan.Spec().Target {\n\t\t\t\t\t\tchanged[i].Result.ReturnedN++\n\t\t\t\t\t}\n\t\t\t\tcase \"physical-validation\":\n\t\t\t\t\tif changed[i].Kind == ValidationObserved {\n\t\t\t\t\t\tchanged[i].Validation = Validation(1)\n\t\t\t\t\t}\n\t\t\t\tcase \"foreign-trial\":\n\t\t\t\t\tchanged[i].Identity.Trial = \"other-trial\"\n\t\t\t\t}\n\t\t\t}\n\t\tevaluate:\n\t\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, sqlReceipt(t, plan, changed), evidence)) })\n\t\t}\n\t\tsqlMustReject(t, ObserveSQLLedger(Plan{}, Receipt{}, evidence))\n\t\totherPlan, _, _ := sqlEvidenceFixture(t, SQLControl)\n\t\tif which == SQLControl {\n\t\t\totherPlan, _, _ = sqlEvidenceFixture(t, SQLDBSyncBefore)\n\t\t}\n\t\tsqlMustReject(t, ObserveSQLLedger(plan, sqlReceipt(t, otherPlan, fixtureEvents(otherPlan.Spec())), evidence))\n\t}\n}\n\n// The legacy Three-named root remains stable; this separate positive freezes\n// the two return modes independently instead of inferring one from the other.\nfunc TestSQLLedgerPrefixErrorFreezesNativeAndAdapterReturns(t *testing.T) {\n\tfor _, which := range []SQLCase{SQLIndexPrefix64, SQLIndexPrefix64Error} {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, which)\n\t\tspec := plan.Spec()\n\t\tif spec.Target != (Call{Point: DBPage, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192, Length: 4096}) || spec.CallCount != 15 || spec.Prefix != 64 || spec.PrefixError != (which == SQLIndexPrefix64Error) {\n\t\t\tt.Fatal(\"prefix cases no longer share the one exact write point\")\n\t\t}\n\t\tevents, count := receipt.Events()\n\t\tif count != 35 {\n\t\t\tt.Fatalf(\"prefix trace count: %d\", count)\n\t\t}\n\t\twant := Result{Executed: true, NativeN: 64, ReturnedN: 64, NormalizedError: ShortWrite, Effect: EffectPrefix}\n\t\tcause := ShortWrite\n\t\tif which == SQLIndexPrefix64Error {\n\t\t\twant.ReturnedError, want.NormalizedError, cause = Injected, Injected, Injected\n\t\t}\n\t\tif events[29].Kind != Intent || events[29].Call != spec.Target || events[30].Kind != Returned || events[30].Call != spec.Target || events[30].Action != PrefixShortWrite || events[30].Result != want {\n\t\t\tt.Fatal(\"native (64,nil), adapter return, or normalization changed\")\n\t\t}\n\t\tcommit := events[31].Commit\n\t\tif events[31].Kind != CommitReturned || commit.Class != CommitRecoveryRequired || commit.Error != Underlying || commit.ErrorToken == 0 || commit.TargetError != cause || commit.TargetErrorToken != 0 || commit.Logical != LogicalClosed {\n\t\t\tt.Fatal(\"public error provenance differs from target return\")\n\t\t}\n\t\tif evidence.Lifecycle.PublicShortWrite != (which == SQLIndexPrefix64) || evidence.After.Queries[SQLAllRows].Rows != sqlLiteralRows(SQLOld) {\n\t\t\tt.Fatal(\"prefix mode confused public cause or complete OLD replay\")\n\t\t}\n\t\tobserved := ObserveSQLLedger(plan, receipt, evidence)\n\t\tif !observed.ScenarioObserved || observed.Problem != SQLProblemNone || observed.PhysicalValidated() || observed.Pass() {\n\t\t\tt.Fatalf(\"prefix mode refused or claimed physical acceptance: %+v\", observed)\n\t\t}\n\t}\n}\n\nfunc TestSQLLedgerPrefixErrorRejectsPlanAndReceiptConfusion(t *testing.T) {\n\tfor _, name := range []string{\"old-plan\", \"prefix-bit-dropped\", \"wrong-prefix\", \"wrong-offset\", \"wrong-length\", \"wrong-occurrence\", \"case-only-relabel\", \"native-injected\", \"native-error-masked\", \"native-error-preserved\", \"shortwrite-normalized\", \"nil-adapter-error\", \"native-count\", \"public-nil\", \"commit-target-shortwrite\", \"commit-target-token\", \"missing-target\", \"duplicate-target\"} {\n\t\tplan, _, evidence := sqlEvidenceFixture(t, SQLIndexPrefix64Error)\n\t\tevents := fixtureEvents(plan.Spec())\n\t\tswitch name {\n\t\tcase \"old-plan\":\n\t\t\tplan, _, _ = sqlEvidenceFixture(t, SQLIndexPrefix64)\n\t\t\tevents = fixtureEvents(plan.Spec())\n\t\tcase \"prefix-bit-dropped\":\n\t\t\tplan.spec.PrefixError = false\n\t\tcase \"wrong-prefix\":\n\t\t\tplan.spec.Prefix = 65\n\t\tcase \"wrong-offset\":\n\t\t\tplan.spec.Target.Offset = 4096\n\t\tcase \"wrong-length\":\n\t\t\tplan.spec.Target.Length = 4095\n\t\tcase \"wrong-occurrence\":\n\t\t\tplan.spec.Target.Occurrence = 2\n\t\tcase \"case-only-relabel\":\n\t\t\toldPlan, _, _ := sqlEvidenceFixture(t, SQLIndexPrefix64)\n\t\t\tevents = fixtureEvents(oldPlan.Spec())\n\t\tcase \"native-injected\":\n\t\t\tevents[30].Result.NativeError = Injected\n\t\tcase \"native-error-masked\":\n\t\t\tevents[30].Result.NativeError, events[30].Result.NativeErrorToken = Underlying, 41\n\t\t\tevents[30].Result.Effect = EffectUnknown\n\t\tcase \"native-error-preserved\":\n\t\t\tevents[30].Result.NativeError, events[30].Result.NativeErrorToken = Underlying, 41\n\t\t\tevents[30].Result.ReturnedError, events[30].Result.ReturnedErrorToken = Underlying, 41\n\t\t\tevents[30].Result.NormalizedError, events[30].Result.NormalizedErrorToken = Underlying, 41\n\t\t\tevents[30].Result.Effect = EffectUnknown\n\t\t\tevents[31].Commit.TargetError, events[31].Commit.TargetErrorToken = Underlying, 41\n\t\tcase \"shortwrite-normalized\":\n\t\t\tevents[30].Result.NormalizedError = ShortWrite\n\t\t\tevents[31].Commit.TargetError = ShortWrite\n\t\tcase \"nil-adapter-error\":\n\t\t\tevents[30].Result.ReturnedError = None\n\t\tcase \"native-count\":\n\t\t\tevents[30].Result.NativeN = 63\n\t\tcase \"public-nil\":\n\t\t\tevents[31].Commit.Error, events[31].Commit.ErrorToken = None, 0\n\t\tcase \"commit-target-shortwrite\":\n\t\t\tevents[31].Commit.TargetError = ShortWrite\n\t\tcase \"commit-target-token\":\n\t\t\tevents[31].Commit.TargetErrorToken = 41\n\t\tcase \"missing-target\":\n\t\t\tevents = append(events[:29], events[31:]...)\n\t\t\trenumber(events, plan.Spec().Identity)\n\t\tcase \"duplicate-target\":\n\t\t\tduplicate := []Event{events[29], events[30]}\n\t\t\tevents = append(events[:31], append(duplicate, events[31:]...)...)\n\t\t\trenumber(events, plan.Spec().Identity)\n\t\t}\n\t\tt.Run(name, func(t *testing.T) {\n\t\t\treceipt := sqlReceipt(t, plan, events)\n\t\t\tif name == \"native-error-preserved\" && receipt.Seal().Problem != ProblemUnderlyingFailure {\n\t\t\t\tt.Fatal(\"preserved real native error must refuse the selected injected plan\")\n\t\t\t}\n\t\t\tif name == \"native-error-masked\" && receipt.Seal().Problem != ProblemErrorMasked {\n\t\t\t\tt.Fatal(\"injection must never overwrite an actual native error\")\n\t\t\t}\n\t\t\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))\n\t\t})\n\t}\n}\n\nfunc TestSQLLedgerPrefixErrorRequiresCompleteOldReplayAndOwners(t *testing.T) {\n\tfor _, name := range []string{\"public-shortwrite\", \"missing-code\", \"missing-recovery-cause\", \"reconstructed-public-error\", \"new-ledger\", \"old-rows-new-index\", \"missing-index\", \"missing-eof\", \"missing-err\", \"missing-check\", \"missing-owner\", \"live-owner\", \"replay-original\", \"wrong-recovery-extent\", \"missing-recovery\", \"original-journal-changed\", \"physical-claim\"} {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, SQLIndexPrefix64Error)\n\t\tswitch name {\n\t\tcase \"public-shortwrite\":\n\t\t\tevidence.Lifecycle.PublicShortWrite = true\n\t\tcase \"missing-code\":\n\t\t\tevidence.Lifecycle.PublicCodeIOErr = false\n\t\tcase \"missing-recovery-cause\":\n\t\t\tevidence.Lifecycle.PublicRecoveryRequired = false\n\t\tcase \"reconstructed-public-error\":\n\t\t\tevidence.Lifecycle.CommitCapturedUnchanged = false\n\t\tcase \"new-ledger\":\n\t\t\tevidence.After = sqlSnapshot(evidence.Identity, 102, 8, SQLNew)\n\t\tcase \"old-rows-new-index\":\n\t\t\tnewState := sqlSnapshot(evidence.Identity, 102, 8, SQLNew)\n\t\t\tevidence.After.Queries[SQLBlueIndex] = newState.Queries[SQLBlueIndex]\n\t\tcase \"missing-index\":\n\t\t\tevidence.After.Queries[SQLBlueIndex] = SQLQueryEvidence{}\n\t\tcase \"missing-eof\":\n\t\t\tevidence.After.Queries[SQLAllRows].EOF = false\n\t\tcase \"missing-err\":\n\t\t\tevidence.After.Queries[SQLAllRows].ErrCalled = false\n\t\tcase \"missing-check\":\n\t\t\tevidence.After.Check.Calls = 0\n\t\tcase \"missing-owner\":\n\t\t\tevidence.After.Owner = 0\n\t\tcase \"live-owner\":\n\t\t\tevidence.After.OwnerState = OwnerLive\n\t\tcase \"replay-original\":\n\t\t\tevidence.Artifacts[SQLReplayArtifact].Before.Inode = evidence.Artifacts[SQLTrialArtifact].Before.Inode\n\t\tcase \"wrong-recovery-extent\":\n\t\t\tevidence.Recovery.Limits.MaxDatabaseBytes = 16384\n\t\tcase \"missing-recovery\":\n\t\t\tevidence.Recovery.Calls = 0\n\t\tcase \"original-journal-changed\":\n\t\t\tevidence.Artifacts[SQLOriginalJournalArtifact].After.SHA256[0]++\n\t\tcase \"physical-claim\":\n\t\t\tevidence.PhysicalClaim = true\n\t\t}\n\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t}\n}\n\n// Each row freezes its own target and counts. These values are not obtained\n// from SQLPlan, a case-range comparison, or the receipt under observation.\nfunc sqlLiteralPreDBCases() [3]struct {\n\twhich       SQLCase\n\ttarget      Call\n\ttargetIndex int\n\tcallCount   uint8\n\teventCount  int\n} {\n\treturn [3]struct {\n\t\twhich       SQLCase\n\t\ttarget      Call\n\t\ttargetIndex int\n\t\tcallCount   uint8\n\t\teventCount  int\n\t}{\n\t\t{SQLJournalBodySyncBefore, Call{Point: JournalBodySync, Occurrence: 1, Role: Journal, Owner: 12}, 8, 11, 27},\n\t\t{SQLJournalHeaderSyncBefore, Call{Point: JournalHeaderSync, Occurrence: 1, Role: Journal, Owner: 12}, 10, 13, 31},\n\t\t{SQLPublishDirectorySyncBefore, Call{Point: PublishDirectorySync, Occurrence: 1, Role: Directory, Owner: 13}, 11, 14, 33},\n\t}\n}\n\nfunc TestSQLLedgerCasePartitionsUseOnlyExactIDs(t *testing.T) {\n\tfor value := 0; value <= 255; value++ {\n\t\twhich := SQLCase(value)\n\t\twantPreDB := value == 5 || value == 6 || value == 7\n\t\twantRecovery := value == 2 || value == 3 || value == 4 || value == 8 || value == 10\n\t\twantPostDB := value == 8 || value == 9\n\t\tif isSQLPreDBFailure(which) != wantPreDB || needsSQLRecovery(which) != wantRecovery || isSQLPostDBFailure(which) != wantPostDB {\n\t\t\tt.Fatalf(\"case %d entered the wrong exact-ID partition\", value)\n\t\t}\n\t}\n}\n\nfunc TestSQLLedgerPreDBFailuresFreezeSkippedTargetAndNormalCleanup(t *testing.T) {\n\tprefix := []Call{\n\t\t{Point: JournalCreate, Occurrence: 1, Role: Journal, Owner: 12},\n\t\t{Point: JournalInvalid, Occurrence: 1, Role: Journal, Owner: 12, Length: 4096},\n\t\t{Point: DBOriginalRead, Occurrence: 1, Role: Database, Owner: 11, Offset: 0, Length: 4096},\n\t\t{Point: JournalRecord, Occurrence: 1, Role: Journal, Owner: 12, Offset: 4096, Length: 4104},\n\t\t{Point: DBOriginalRead, Occurrence: 2, Role: Database, Owner: 11, Offset: 4096, Length: 4096},\n\t\t{Point: JournalRecord, Occurrence: 2, Role: Journal, Owner: 12, Offset: 8200, Length: 4104},\n\t\t{Point: DBOriginalRead, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192, Length: 4096},\n\t\t{Point: JournalRecord, Occurrence: 3, Role: Journal, Owner: 12, Offset: 12304, Length: 4104},\n\t\t{Point: JournalBodySync, Occurrence: 1, Role: Journal, Owner: 12},\n\t\t{Point: JournalValidHeader, Occurrence: 1, Role: Journal, Owner: 12, Length: 12},\n\t\t{Point: JournalHeaderSync, Occurrence: 1, Role: Journal, Owner: 12},\n\t\t{Point: PublishDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},\n\t}\n\tcleanup := [2]Call{\n\t\t{Point: CleanupUnlink, Occurrence: 1, Role: Journal, Owner: 12},\n\t\t{Point: CleanupDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},\n\t}\n\tfor _, fixed := range sqlLiteralPreDBCases() {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, fixed.which)\n\t\tspec := plan.Spec()\n\t\tif spec.Target != fixed.target || spec.Action != FailBefore || spec.Prefix != 0 || spec.PrefixError || spec.ExpectedCommit != CommitFailed || spec.CallCount != fixed.callCount {\n\t\t\tt.Fatalf(\"pre-DB selector %v changed: %+v\", fixed.which, spec)\n\t\t}\n\t\twantCalls := append([]Call(nil), prefix[:fixed.targetIndex+1]...)\n\t\twantCalls = append(wantCalls, cleanup[:]...)\n\t\tevents, count := receipt.Events()\n\t\tif int(count) != fixed.eventCount || events[0].Kind != RunStarted {\n\t\t\tt.Fatal(\"pre-DB trace has the wrong exact event extent\")\n\t\t}\n\t\tfor i, call := range wantCalls {\n\t\t\tif spec.Calls[i] != call || call.Point == DBPage || call.Point == DBSync || call.Point == JournalUnlink || call.Point == FinalDirectorySync {\n\t\t\t\tt.Fatalf(\"pre-DB call %d escaped the fixed prefix and cleanup path\", i)\n\t\t\t}\n\t\t\taction := Pass\n\t\t\twant := Result{Executed: true, NativeN: call.Length, ReturnedN: call.Length, Effect: EffectUnknown}\n\t\t\tswitch call.Point {\n\t\t\tcase DBOriginalRead:\n\t\t\t\twant.Effect = EffectNone\n\t\t\tcase JournalInvalid, JournalRecord, JournalValidHeader:\n\t\t\t\twant.Effect = EffectPossible\n\t\t\t}\n\t\t\tif i == fixed.targetIndex {\n\t\t\t\taction = FailBefore\n\t\t\t\twant = Result{ReturnedError: Injected, NormalizedError: Injected, Effect: EffectNone}\n\t\t\t}\n\t\t\tintent, returned := events[1+2*i], events[2+2*i]\n\t\t\tif intent.Kind != Intent || intent.Call != call || intent.Action != action || returned.Kind != Returned || returned.Call != call || returned.Action != action || returned.Result != want {\n\t\t\t\tt.Fatalf(\"pre-DB call %d lost skipped-target or normal-cleanup evidence\", i)\n\t\t\t}\n\t\t}\n\t\tfor i := int(fixed.callCount); i < MaxCalls; i++ {\n\t\t\tif spec.Calls[i] != (Call{}) {\n\t\t\t\tt.Fatal(\"pre-DB unused call tail is not empty\")\n\t\t\t}\n\t\t}\n\t\tcommitAt := 1 + 2*int(fixed.callCount)\n\t\tcommit := events[commitAt].Commit\n\t\tif events[commitAt].Kind != CommitReturned || commit.Class != CommitFailed || commit.Error != Underlying || commit.ErrorToken == 0 || commit.TargetError != Injected || commit.TargetErrorToken != 0 || commit.Logical != LogicalClosed {\n\t\t\tt.Fatal(\"pre-DB public error lost its exact failure class or target cause\")\n\t\t}\n\t\tif events[commitAt+1].Kind != OwnersObserved || events[commitAt+1].Owners != (OwnersResult{MainOwner: 11, JournalOwner: 12, CleanupOwner: 14, Main: OwnerRetired, Journal: OwnerRetired, Cleanup: OwnerRetired}) || events[commitAt+2].Kind != ValidationObserved || events[commitAt+2].Validation != 0 || events[commitAt+3].Kind != Terminal {\n\t\t\tt.Fatal(\"pre-DB trace did not end with retired owners and observation-only terminal\")\n\t\t}\n\t\tif evidence.Recovery != (SQLRecoveryEvidence{Identity: evidence.Identity, OwnersRetired: true}) || !evidence.Lifecycle.PublicCodeIOErr || !evidence.Lifecycle.PublicPoisoned || !evidence.Lifecycle.PreDBOnlyTargetCause || evidence.Lifecycle.PublicRecoveryRequired || evidence.Lifecycle.PublicShortWrite || evidence.Lifecycle.PublicCommitUncertain || evidence.Lifecycle.PublicCleanupPending || !evidence.Lifecycle.WriterRetiredBeforeRecovery || !evidence.Lifecycle.RecoveryFinishedBeforeReopen {\n\t\t\tt.Fatal(\"pre-DB lifecycle confused zero recovery with a missing decision boundary\")\n\t\t}\n\t\tbaseline := evidence.Artifacts[SQLBaselineArtifact].Before.SHA256\n\t\tfor _, role := range []SQLArtifactRole{SQLBaselineArtifact, SQLTrialArtifact, SQLPostCopyArtifact, SQLReplayArtifact} {\n\t\t\tif evidence.Artifacts[role].Before.SHA256 != baseline || evidence.Artifacts[role].After.SHA256 != baseline {\n\t\t\t\tt.Fatal(\"pre-DB whole-byte artifact evidence differs from baseline\")\n\t\t\t}\n\t\t}\n\t\tfor _, role := range []SQLArtifactRole{SQLOriginalJournalArtifact, SQLJournalCopyArtifact, SQLReplayJournalArtifact} {\n\t\t\tif evidence.Artifacts[role] != (SQLArtifactEvidence{Role: role}) {\n\t\t\t\tt.Fatal(\"pre-DB journal artifact was retained or manufactured\")\n\t\t\t}\n\t\t}\n\t\tif evidence.After.Queries[SQLAllRows].Rows != sqlLiteralRows(SQLOld) {\n\t\t\tt.Fatal(\"pre-DB fresh read-only copy is not the complete OLD ledger\")\n\t\t}\n\t\tobservation := ObserveSQLLedger(plan, receipt, evidence)\n\t\tif !observation.EvidenceConsistent || !observation.ScenarioObserved || observation.Problem != SQLProblemNone || observation.Adapter.Outcome != AdapterObserved || observation.PhysicalValidated() || observation.Pass() {\n\t\t\tt.Fatalf(\"pre-DB finite trace refused or claimed physical validation: %+v\", observation)\n\t\t}\n\t}\n}\n\nfunc TestSQLLedgerPreDBFailuresRejectPlanTargetAndCleanupConfusion(t *testing.T) {\n\tfor _, fixed := range sqlLiteralPreDBCases() {\n\t\tfor _, name := range []string{\"wrong-case\", \"old-case\", \"wrong-target\", \"wrong-target-owner\", \"wrong-action\", \"native-executed\", \"native-error\", \"missing-target-error\", \"target-shortwrite\", \"target-token\", \"missing-target\", \"duplicate-target\", \"missing-cleanup-unlink\", \"missing-cleanup-dirsync\", \"cleanup-not-executed\", \"cleanup-real-error\", \"cleanup-error-masked\", \"cleanup-wrong-owner\", \"public-nil\", \"recovery-required\", \"commit-uncertain\", \"cleanup-pending\", \"logical-open\", \"wrong-commit-target\", \"extra-commit-target-token\", \"pending-owner\", \"unknown-owner\", \"physical-validation\"} {\n\t\t\tplan, _, evidence := sqlEvidenceFixture(t, fixed.which)\n\t\t\tevents := fixtureEvents(plan.Spec())\n\t\t\ttargetIntent, targetReturn := 1+2*fixed.targetIndex, 2+2*fixed.targetIndex\n\t\t\tunlinkIntent, unlinkReturn := targetIntent+2, targetReturn+2\n\t\t\tdirsyncIntent := unlinkIntent + 2\n\t\t\tcommitAt := 1 + 2*int(fixed.callCount)\n\t\t\tswitch name {\n\t\t\tcase \"wrong-case\":\n\t\t\t\tevidence.Case = SQLJournalBodySyncBefore\n\t\t\t\tif fixed.which == SQLJournalBodySyncBefore {\n\t\t\t\t\tevidence.Case = SQLJournalHeaderSyncBefore\n\t\t\t\t}\n\t\t\tcase \"old-case\":\n\t\t\t\tevidence.Case = SQLDBSyncBefore\n\t\t\tcase \"wrong-target\":\n\t\t\t\tplan.spec.Target.Point = DBSync\n\t\t\tcase \"wrong-target-owner\":\n\t\t\t\tplan.spec.Target.Owner = 14\n\t\t\tcase \"wrong-action\":\n\t\t\t\tplan.spec.Action = Pass\n\t\t\tcase \"native-executed\":\n\t\t\t\tevents[targetReturn].Result.Executed = true\n\t\t\tcase \"native-error\":\n\t\t\t\tevents[targetReturn].Result.NativeError, events[targetReturn].Result.NativeErrorToken = Underlying, 71\n\t\t\tcase \"missing-target-error\":\n\t\t\t\tevents[targetReturn].Result.ReturnedError = None\n\t\t\tcase \"target-shortwrite\":\n\t\t\t\tevents[targetReturn].Result.NormalizedError = ShortWrite\n\t\t\tcase \"target-token\":\n\t\t\t\tevents[targetReturn].Result.ReturnedErrorToken = 71\n\t\t\tcase \"missing-target\":\n\t\t\t\tevents = append(events[:targetIntent], events[targetReturn+1:]...)\n\t\t\tcase \"duplicate-target\":\n\t\t\t\tduplicate := []Event{events[targetIntent], events[targetReturn]}\n\t\t\t\tevents = append(events[:targetReturn+1], append(duplicate, events[targetReturn+1:]...)...)\n\t\t\tcase \"missing-cleanup-unlink\":\n\t\t\t\tevents = append(events[:unlinkIntent], events[unlinkReturn+1:]...)\n\t\t\tcase \"missing-cleanup-dirsync\":\n\t\t\t\tevents = append(events[:dirsyncIntent], events[dirsyncIntent+2:]...)\n\t\t\tcase \"cleanup-not-executed\":\n\t\t\t\tevents[unlinkReturn].Result.Executed = false\n\t\t\tcase \"cleanup-real-error\":\n\t\t\t\tevents[unlinkReturn].Result.NativeError, events[unlinkReturn].Result.NativeErrorToken = Underlying, 71\n\t\t\t\tevents[unlinkReturn].Result.ReturnedError, events[unlinkReturn].Result.ReturnedErrorToken = Underlying, 71\n\t\t\t\tevents[unlinkReturn].Result.NormalizedError, events[unlinkReturn].Result.NormalizedErrorToken = Underlying, 71\n\t\t\tcase \"cleanup-error-masked\":\n\t\t\t\tevents[unlinkReturn].Result.NativeError, events[unlinkReturn].Result.NativeErrorToken = Underlying, 71\n\t\t\tcase \"cleanup-wrong-owner\":\n\t\t\t\tevents[unlinkIntent].Call.Owner, events[unlinkReturn].Call.Owner = 14, 14\n\t\t\tcase \"public-nil\":\n\t\t\t\tevents[commitAt].Commit.Error, events[commitAt].Commit.ErrorToken = None, 0\n\t\t\tcase \"recovery-required\":\n\t\t\t\tevents[commitAt].Commit.Class = CommitRecoveryRequired\n\t\t\tcase \"commit-uncertain\":\n\t\t\t\tevents[commitAt].Commit.Class = CommitUncertain\n\t\t\tcase \"cleanup-pending\":\n\t\t\t\tevents[commitAt].Commit.Class = CommitCleanupPending\n\t\t\tcase \"logical-open\":\n\t\t\t\tevents[commitAt].Commit.Logical = LogicalOpen\n\t\t\tcase \"wrong-commit-target\":\n\t\t\t\tevents[commitAt].Commit.TargetError = ShortWrite\n\t\t\tcase \"extra-commit-target-token\":\n\t\t\t\tevents[commitAt].Commit.TargetErrorToken = 71\n\t\t\tcase \"pending-owner\":\n\t\t\t\tevents[commitAt+1].Owners.Cleanup = OwnerPending\n\t\t\tcase \"unknown-owner\":\n\t\t\t\tevents[commitAt+1].Owners.Journal = OwnerUnknown\n\t\t\tcase \"physical-validation\":\n\t\t\t\tevents[commitAt+2].Validation = Validation(1)\n\t\t\t}\n\t\t\trenumber(events, plan.Spec().Identity)\n\t\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, sqlReceipt(t, plan, events), evidence)) })\n\t\t}\n\t}\n}\n\nfunc TestSQLLedgerPreDBFailuresRequireBaselineCopiesAndZeroRecovery(t *testing.T) {\n\tfor _, fixed := range sqlLiteralPreDBCases() {\n\t\tfor _, name := range []string{\"one-recovery\", \"replayed\", \"replayed-pages\", \"original-pages\", \"recovery-limits\", \"recovery-error\", \"recovery-token\", \"journal-before\", \"journal-after\", \"recovery-owner-live\", \"trial-not-baseline\", \"post-not-baseline\", \"replay-input-not-baseline\", \"replay-output-not-baseline\", \"coherent-nonbaseline-copies\", \"missing-baseline\", \"missing-trial\", \"missing-post\", \"missing-replay\", \"replay-aliases-trial\", \"post-aliases-baseline\", \"changed-replay-inode\", \"wrong-size\", \"changed-original-page\", \"new-ledger\", \"mixed-new-index\", \"extra-error-cause\", \"physical-claim\"} {\n\t\t\tplan, receipt, evidence := sqlEvidenceFixture(t, fixed.which)\n\t\t\tswitch name {\n\t\t\tcase \"one-recovery\":\n\t\t\t\tevidence.Recovery.Calls = 1\n\t\t\tcase \"replayed\":\n\t\t\t\tevidence.Recovery.Replayed = true\n\t\t\tcase \"replayed-pages\":\n\t\t\t\tevidence.Recovery.ReplayedPages = 3\n\t\t\tcase \"original-pages\":\n\t\t\t\tevidence.Recovery.OriginalPages = 3\n\t\t\tcase \"recovery-limits\":\n\t\t\t\tevidence.Recovery.Limits = SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}\n\t\t\tcase \"recovery-error\":\n\t\t\t\tevidence.Recovery.Error = Underlying\n\t\t\tcase \"recovery-token\":\n\t\t\t\tevidence.Recovery.ErrorToken = 71\n\t\t\tcase \"journal-before\":\n\t\t\t\tevidence.Recovery.JournalPresentBefore = true\n\t\t\tcase \"journal-after\":\n\t\t\t\tevidence.Recovery.JournalPresentAfter = true\n\t\t\tcase \"recovery-owner-live\":\n\t\t\t\tevidence.Recovery.OwnersRetired = false\n\t\t\tcase \"trial-not-baseline\":\n\t\t\t\tevidence.Artifacts[SQLTrialArtifact].Before.SHA256[0] = 22\n\t\t\t\tevidence.Artifacts[SQLTrialArtifact].After.SHA256[0] = 22\n\t\t\tcase \"post-not-baseline\":\n\t\t\t\tevidence.Artifacts[SQLPostCopyArtifact].Before.SHA256[0] = 22\n\t\t\t\tevidence.Artifacts[SQLPostCopyArtifact].After.SHA256[0] = 22\n\t\t\tcase \"replay-input-not-baseline\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].Before.SHA256[0] = 22\n\t\t\tcase \"replay-output-not-baseline\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].After.SHA256[0] = 22\n\t\t\tcase \"coherent-nonbaseline-copies\":\n\t\t\t\tfor _, role := range []SQLArtifactRole{SQLTrialArtifact, SQLPostCopyArtifact, SQLReplayArtifact} {\n\t\t\t\t\tevidence.Artifacts[role].Before.SHA256[0], evidence.Artifacts[role].After.SHA256[0] = 22, 22\n\t\t\t\t}\n\t\t\tcase \"missing-baseline\":\n\t\t\t\tevidence.Artifacts[SQLBaselineArtifact].Before = SQLArtifactStamp{}\n\t\t\tcase \"missing-trial\":\n\t\t\t\tevidence.Artifacts[SQLTrialArtifact].Before = SQLArtifactStamp{}\n\t\t\tcase \"missing-post\":\n\t\t\t\tevidence.Artifacts[SQLPostCopyArtifact].Before = SQLArtifactStamp{}\n\t\t\tcase \"missing-replay\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].Before = SQLArtifactStamp{}\n\t\t\tcase \"replay-aliases-trial\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].Before.Inode, evidence.Artifacts[SQLReplayArtifact].After.Inode = evidence.Artifacts[SQLTrialArtifact].Before.Inode, evidence.Artifacts[SQLTrialArtifact].Before.Inode\n\t\t\tcase \"post-aliases-baseline\":\n\t\t\t\tevidence.Artifacts[SQLPostCopyArtifact].Before.Inode, evidence.Artifacts[SQLPostCopyArtifact].After.Inode = evidence.Artifacts[SQLBaselineArtifact].Before.Inode, evidence.Artifacts[SQLBaselineArtifact].Before.Inode\n\t\t\tcase \"changed-replay-inode\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].After.Inode++\n\t\t\tcase \"wrong-size\":\n\t\t\t\tevidence.Artifacts[SQLTrialArtifact].Before.Bytes--\n\t\t\tcase \"changed-original-page\":\n\t\t\t\tevidence.Originals.After[2][0]++\n\t\t\tcase \"new-ledger\":\n\t\t\t\tevidence.After = sqlSnapshot(evidence.Identity, 102, 8, SQLNew)\n\t\t\tcase \"mixed-new-index\":\n\t\t\t\tnewState := sqlSnapshot(evidence.Identity, 102, 8, SQLNew)\n\t\t\t\tevidence.After.Queries[SQLBlueIndex] = newState.Queries[SQLBlueIndex]\n\t\t\tcase \"extra-error-cause\":\n\t\t\t\tevidence.Lifecycle.PreDBOnlyTargetCause = false\n\t\t\tcase \"physical-claim\":\n\t\t\t\tevidence.PhysicalClaim = true\n\t\t\t}\n\t\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t\t}\n\t\tfor _, role := range []SQLArtifactRole{SQLOriginalJournalArtifact, SQLJournalCopyArtifact, SQLReplayJournalArtifact} {\n\t\t\tfor _, after := range []bool{false, true} {\n\t\t\t\tplan, receipt, evidence := sqlEvidenceFixture(t, fixed.which)\n\t\t\t\tstamp := SQLArtifactStamp{Identity: SQLArtifactName(role), Device: 1, Inode: uint64(role + 1), Exists: true, Bytes: 16408}\n\t\t\t\tstamp.SHA256[0] = 33\n\t\t\t\tif after {\n\t\t\t\t\tevidence.Artifacts[role].After = stamp\n\t\t\t\t} else {\n\t\t\t\t\tevidence.Artifacts[role].Before = stamp\n\t\t\t\t}\n\t\t\t\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))\n\t\t\t}\n\t\t}\n\t}\n}\n\nfunc TestSQLLedgerAllSevenCasesRequireExactPublicFailureFlags(t *testing.T) {\n\tfor _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore, SQLIndexPrefix64Error, SQLJournalBodySyncBefore, SQLJournalHeaderSyncBefore, SQLPublishDirectorySyncBefore, SQLJournalUnlinkBefore, SQLFinalDirectorySyncBefore} {\n\t\tfor _, name := range []string{\"code\", \"poisoned\", \"recovery\", \"shortwrite\", \"uncertain\", \"cleanup-pending\", \"target-only-cause\", \"post-target-only-cause\", \"retired-boundary\", \"recovery-decision-boundary\", \"captured-error\", \"writable-reopen\"} {\n\t\t\tplan, receipt, evidence := sqlEvidenceFixture(t, which)\n\t\t\tlifecycle := &evidence.Lifecycle\n\t\t\tswitch name {\n\t\t\tcase \"code\":\n\t\t\t\tlifecycle.PublicCodeIOErr = !lifecycle.PublicCodeIOErr\n\t\t\tcase \"poisoned\":\n\t\t\t\tlifecycle.PublicPoisoned = !lifecycle.PublicPoisoned\n\t\t\tcase \"recovery\":\n\t\t\t\tlifecycle.PublicRecoveryRequired = !lifecycle.PublicRecoveryRequired\n\t\t\tcase \"shortwrite\":\n\t\t\t\tlifecycle.PublicShortWrite = !lifecycle.PublicShortWrite\n\t\t\tcase \"uncertain\":\n\t\t\t\tlifecycle.PublicCommitUncertain = !lifecycle.PublicCommitUncertain\n\t\t\tcase \"cleanup-pending\":\n\t\t\t\tlifecycle.PublicCleanupPending = true\n\t\t\tcase \"target-only-cause\":\n\t\t\t\tlifecycle.PreDBOnlyTargetCause = !lifecycle.PreDBOnlyTargetCause\n\t\t\tcase \"post-target-only-cause\":\n\t\t\t\tlifecycle.PostDBOnlyTargetCause = !lifecycle.PostDBOnlyTargetCause\n\t\t\tcase \"retired-boundary\":\n\t\t\t\tlifecycle.WriterRetiredBeforeRecovery = false\n\t\t\tcase \"recovery-decision-boundary\":\n\t\t\t\tlifecycle.RecoveryFinishedBeforeReopen = false\n\t\t\tcase \"captured-error\":\n\t\t\t\tlifecycle.CommitCapturedUnchanged = false\n\t\t\tcase \"writable-reopen\":\n\t\t\t\tlifecycle.ReopenReadOnly = false\n\t\t\t}\n\t\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t\t}\n\t}\n}\n\nfunc TestSQLLedgerAllSevenQueriesRequireEOFCheckAndRetiredOwners(t *testing.T) {\n\tfor _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore, SQLIndexPrefix64Error, SQLJournalBodySyncBefore, SQLJournalHeaderSyncBefore, SQLPublishDirectorySyncBefore, SQLJournalUnlinkBefore, SQLFinalDirectorySyncBefore} {\n\t\tfor _, before := range []bool{false, true} {\n\t\t\tfor q := 0; q < SQLQueryCount; q++ {\n\t\t\t\tfor _, name := range []string{\"eof\", \"err-called\", \"row-error\", \"close\", \"owner\", \"iterator\", \"text\", \"work\", \"row-value\"} {\n\t\t\t\t\tplan, receipt, evidence := sqlEvidenceFixture(t, which)\n\t\t\t\t\tsnapshot := &evidence.After\n\t\t\t\t\tif before {\n\t\t\t\t\t\tsnapshot = &evidence.Before\n\t\t\t\t\t}\n\t\t\t\t\tquery := &snapshot.Queries[q]\n\t\t\t\t\tswitch name {\n\t\t\t\t\tcase \"eof\":\n\t\t\t\t\t\tquery.EOF = false\n\t\t\t\t\tcase \"err-called\":\n\t\t\t\t\t\tquery.ErrCalled = false\n\t\t\t\t\tcase \"row-error\":\n\t\t\t\t\t\tquery.Error, query.ErrorToken = Underlying, 71\n\t\t\t\t\tcase \"close\":\n\t\t\t\t\t\tquery.CloseCalls = 0\n\t\t\t\t\tcase \"owner\":\n\t\t\t\t\t\tquery.Owner = 0\n\t\t\t\t\tcase \"iterator\":\n\t\t\t\t\t\tquery.Iterator = 0\n\t\t\t\t\tcase \"text\":\n\t\t\t\t\t\tquery.SQL = \"SELECT count(*) FROM t\"\n\t\t\t\t\tcase \"work\":\n\t\t\t\t\t\tquery.Work.BaseRowFetches++\n\t\t\t\t\tcase \"row-value\":\n\t\t\t\t\t\tquery.Rows[0].Value = \"not-the-frozen-value\"\n\t\t\t\t\t}\n\t\t\t\t\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))\n\t\t\t\t}\n\t\t\t}\n\t\t\tfor _, name := range []string{\"missing-check\", \"check-owner\", \"check-error\", \"missing-index\", \"owner-live\", \"owner-pending\", \"missing-close\", \"duplicate-close\", \"physical-claim\"} {\n\t\t\t\tplan, receipt, evidence := sqlEvidenceFixture(t, which)\n\t\t\t\tsnapshot := &evidence.After\n\t\t\t\tif before {\n\t\t\t\t\tsnapshot = &evidence.Before\n\t\t\t\t}\n\t\t\t\tswitch name {\n\t\t\t\tcase \"missing-check\":\n\t\t\t\t\tsnapshot.Check.Calls = 0\n\t\t\t\tcase \"check-owner\":\n\t\t\t\t\tsnapshot.Check.Owner = 0\n\t\t\t\tcase \"check-error\":\n\t\t\t\t\tsnapshot.Check.Error, snapshot.Check.ErrorToken = Underlying, 71\n\t\t\t\tcase \"missing-index\":\n\t\t\t\t\tsnapshot.Check.Indexes = 0\n\t\t\t\tcase \"owner-live\":\n\t\t\t\t\tsnapshot.OwnerState = OwnerLive\n\t\t\t\tcase \"owner-pending\":\n\t\t\t\t\tsnapshot.OwnerState = OwnerPending\n\t\t\t\tcase \"missing-close\":\n\t\t\t\t\tsnapshot.CloseCalls = 0\n\t\t\t\tcase \"duplicate-close\":\n\t\t\t\t\tsnapshot.CloseCalls = 2\n\t\t\t\tcase \"physical-claim\":\n\t\t\t\t\tevidence.PhysicalClaim = true\n\t\t\t\t}\n\t\t\t\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))\n\t\t\t}\n\t\t}\n\t}\n}\n\nfunc sqlLiteralPostDBCases() [2]struct {\n\twhich       SQLCase\n\ttarget      Call\n\ttargetIndex int\n\tcallCount   uint8\n\teventCount  int\n\tcommitClass CommitClass\n\tfinalState  SQLState\n} {\n\treturn [2]struct {\n\t\twhich       SQLCase\n\t\ttarget      Call\n\t\ttargetIndex int\n\t\tcallCount   uint8\n\t\teventCount  int\n\t\tcommitClass CommitClass\n\t\tfinalState  SQLState\n\t}{\n\t\t{SQLJournalUnlinkBefore, Call{Point: JournalUnlink, Occurrence: 1, Role: Journal, Owner: 12}, 16, 17, 39, CommitRecoveryRequired, SQLOld},\n\t\t{SQLFinalDirectorySyncBefore, Call{Point: FinalDirectorySync, Occurrence: 1, Role: Directory, Owner: 13}, 17, 18, 41, CommitUncertain, SQLNew},\n\t}\n}\n\nfunc TestSQLLedgerPostDBFailuresFreezeSkippedTargetAndPublicationPrefix(t *testing.T) {\n\tprefix := [18]Call{\n\t\t{Point: JournalCreate, Occurrence: 1, Role: Journal, Owner: 12},\n\t\t{Point: JournalInvalid, Occurrence: 1, Role: Journal, Owner: 12, Length: 4096},\n\t\t{Point: DBOriginalRead, Occurrence: 1, Role: Database, Owner: 11, Offset: 0, Length: 4096},\n\t\t{Point: JournalRecord, Occurrence: 1, Role: Journal, Owner: 12, Offset: 4096, Length: 4104},\n\t\t{Point: DBOriginalRead, Occurrence: 2, Role: Database, Owner: 11, Offset: 4096, Length: 4096},\n\t\t{Point: JournalRecord, Occurrence: 2, Role: Journal, Owner: 12, Offset: 8200, Length: 4104},\n\t\t{Point: DBOriginalRead, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192, Length: 4096},\n\t\t{Point: JournalRecord, Occurrence: 3, Role: Journal, Owner: 12, Offset: 12304, Length: 4104},\n\t\t{Point: JournalBodySync, Occurrence: 1, Role: Journal, Owner: 12},\n\t\t{Point: JournalValidHeader, Occurrence: 1, Role: Journal, Owner: 12, Length: 12},\n\t\t{Point: JournalHeaderSync, Occurrence: 1, Role: Journal, Owner: 12},\n\t\t{Point: PublishDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},\n\t\t{Point: DBPage, Occurrence: 1, Role: Database, Owner: 11, Offset: 0, Length: 4096},\n\t\t{Point: DBPage, Occurrence: 2, Role: Database, Owner: 11, Offset: 4096, Length: 4096},\n\t\t{Point: DBPage, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192, Length: 4096},\n\t\t{Point: DBSync, Occurrence: 1, Role: Database, Owner: 11},\n\t\t{Point: JournalUnlink, Occurrence: 1, Role: Journal, Owner: 12},\n\t\t{Point: FinalDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},\n\t}\n\tfor _, fixed := range sqlLiteralPostDBCases() {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, fixed.which)\n\t\tspec := plan.Spec()\n\t\tif spec.Target != fixed.target || spec.Action != FailBefore || spec.Prefix != 0 || spec.PrefixError || spec.ExpectedCommit != fixed.commitClass || spec.CallCount != fixed.callCount {\n\t\t\tt.Fatalf(\"post-DB selector %v changed: %+v\", fixed.which, spec)\n\t\t}\n\t\tevents, count := receipt.Events()\n\t\tif int(count) != fixed.eventCount || events[0].Kind != RunStarted {\n\t\t\tt.Fatal(\"post-DB trace has the wrong exact event extent\")\n\t\t}\n\t\tfor i, call := range prefix[:fixed.callCount] {\n\t\t\tif spec.Calls[i] != call {\n\t\t\t\tt.Fatalf(\"post-DB publication prefix call %d changed\", i)\n\t\t\t}\n\t\t\taction := Pass\n\t\t\twant := Result{Executed: true, NativeN: call.Length, ReturnedN: call.Length, Effect: EffectUnknown}\n\t\t\tswitch call.Point {\n\t\t\tcase DBOriginalRead:\n\t\t\t\twant.Effect = EffectNone\n\t\t\tcase JournalInvalid, JournalRecord, JournalValidHeader, DBPage:\n\t\t\t\twant.Effect = EffectPossible\n\t\t\t}\n\t\t\tif i == fixed.targetIndex {\n\t\t\t\taction = FailBefore\n\t\t\t\twant = Result{ReturnedError: Injected, NormalizedError: Injected, Effect: EffectNone}\n\t\t\t}\n\t\t\tintent, returned := events[1+2*i], events[2+2*i]\n\t\t\tif intent.Kind != Intent || intent.Call != call || intent.Action != action || returned.Kind != Returned || returned.Call != call || returned.Action != action || returned.Result != want {\n\t\t\t\tt.Fatalf(\"call %d lost actual prefix completion or skipped-target evidence\", i)\n\t\t\t}\n\t\t}\n\t\tcommitAt := 1 + 2*int(fixed.callCount)\n\t\tcommit := events[commitAt].Commit\n\t\tif events[commitAt].Kind != CommitReturned || commit.Class != fixed.commitClass || commit.Error != Underlying || commit.ErrorToken == 0 || commit.TargetError != Injected || commit.TargetErrorToken != 0 || commit.Logical != LogicalClosed {\n\t\t\tt.Fatal(\"post-DB error lost its case-specific class or target cause\")\n\t\t}\n\t\tif events[commitAt+1].Kind != OwnersObserved || events[commitAt+1].Owners != (OwnersResult{MainOwner: 11, JournalOwner: 12, CleanupOwner: 14, Main: OwnerRetired, Journal: OwnerRetired, Cleanup: OwnerRetired}) || events[commitAt+2].Kind != ValidationObserved || events[commitAt+2].Validation != 0 || events[commitAt+3].Kind != Terminal {\n\t\t\tt.Fatal(\"post-DB trace claimed qualification or skipped retirement\")\n\t\t}\n\t\tif evidence.PostCommit.Queries[SQLAllRows].Rows != sqlLiteralRows(SQLNew) || evidence.After.Queries[SQLAllRows].Rows != sqlLiteralRows(fixed.finalState) || evidence.Lifecycle.ReopenCalls != 2 || !evidence.Lifecycle.PostDBOnlyTargetCause || evidence.Lifecycle.PreDBOnlyTargetCause {\n\t\t\tt.Fatal(\"post-DB complete NEW copy and case-specific final readback missing\")\n\t\t}\n\t\tif fixed.which == SQLJournalUnlinkBefore {\n\t\t\tif evidence.Recovery.Calls != 1 || !evidence.Recovery.JournalPresentBefore || !evidence.Recovery.Replayed || evidence.Recovery.ReplayedPages != 3 || evidence.Recovery.OriginalPages != 3 || evidence.Recovery.Limits != (SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}) || !evidence.Lifecycle.PublicRecoveryRequired || evidence.Lifecycle.PublicCommitUncertain {\n\t\t\t\tt.Fatal(\"unlink-before did not require one bounded OLD replay\")\n\t\t\t}\n\t\t} else {\n\t\t\tif evidence.Recovery != (SQLRecoveryEvidence{Identity: evidence.Identity, OwnersRetired: true}) || evidence.Lifecycle.PublicRecoveryRequired || !evidence.Lifecycle.PublicCommitUncertain {\n\t\t\t\tt.Fatal(\"final-directory uncertainty was promoted to recovery or success\")\n\t\t\t}\n\t\t\tfor _, role := range []SQLArtifactRole{SQLOriginalJournalArtifact, SQLJournalCopyArtifact, SQLReplayJournalArtifact} {\n\t\t\t\tif evidence.Artifacts[role] != (SQLArtifactEvidence{Role: role}) {\n\t\t\t\t\tt.Fatal(\"journal artifact exists after the completed unlink\")\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t\tobservation := ObserveSQLLedger(plan, receipt, evidence)\n\t\tif !observation.EvidenceConsistent || !observation.ScenarioObserved || observation.Problem != SQLProblemNone || observation.PhysicalValidated() || observation.Pass() {\n\t\t\tt.Fatalf(\"post-DB observation refused or upgraded to durability: %+v\", observation)\n\t\t}\n\t}\n}\n\nfunc TestSQLLedgerPostDBFailuresRejectTraceAndClassificationConfusion(t *testing.T) {\n\tfor _, fixed := range sqlLiteralPostDBCases() {\n\t\tfor _, name := range []string{\"wrong-case\", \"unknown-case\", \"wrong-target\", \"wrong-owner\", \"wrong-action\", \"native-executed\", \"native-error\", \"masked-injection\", \"target-token\", \"missing-target\", \"duplicate-target\", \"DB-sync-skipped\", \"DB-sync-error\", \"DB-sync-error-masked\", \"prior-write-short\", \"public-nil\", \"swapped-class\", \"false-ack\", \"cleanup-pending\", \"logical-open\", \"wrong-commit-target\", \"commit-target-token\", \"duplicate-commit\", \"pending-owner\", \"unknown-owner\", \"physical-validation\"} {\n\t\t\tplan, _, evidence := sqlEvidenceFixture(t, fixed.which)\n\t\t\tevents := fixtureEvents(plan.Spec())\n\t\t\ttargetIntent, targetReturn := 1+2*fixed.targetIndex, 2+2*fixed.targetIndex\n\t\t\tcommitAt := 1 + 2*int(fixed.callCount)\n\t\t\tswitch name {\n\t\t\tcase \"wrong-case\":\n\t\t\t\tevidence.Case = SQLJournalUnlinkBefore\n\t\t\t\tif fixed.which == SQLJournalUnlinkBefore {\n\t\t\t\t\tevidence.Case = SQLFinalDirectorySyncBefore\n\t\t\t\t}\n\t\t\tcase \"unknown-case\":\n\t\t\t\tevidence.Case = SQLCase(11)\n\t\t\tcase \"wrong-target\":\n\t\t\t\tplan.spec.Target.Point = DBSync\n\t\t\tcase \"wrong-owner\":\n\t\t\t\tplan.spec.Target.Owner = 14\n\t\t\tcase \"wrong-action\":\n\t\t\t\tplan.spec.Action = Pass\n\t\t\tcase \"native-executed\":\n\t\t\t\tevents[targetReturn].Result.Executed = true\n\t\t\tcase \"native-error\":\n\t\t\t\tevents[targetReturn].Result.NativeError, events[targetReturn].Result.NativeErrorToken = Underlying, 71\n\t\t\tcase \"masked-injection\":\n\t\t\t\tevents[targetReturn].Result.NormalizedError = None\n\t\t\tcase \"target-token\":\n\t\t\t\tevents[targetReturn].Result.ReturnedErrorToken = 71\n\t\t\tcase \"missing-target\":\n\t\t\t\tevents = append(events[:targetIntent], events[targetReturn+1:]...)\n\t\t\tcase \"duplicate-target\":\n\t\t\t\tduplicate := []Event{events[targetIntent], events[targetReturn]}\n\t\t\t\tevents = append(events[:targetReturn+1], append(duplicate, events[targetReturn+1:]...)...)\n\t\t\tcase \"DB-sync-skipped\":\n\t\t\t\tevents[32].Result.Executed = false\n\t\t\tcase \"DB-sync-error\":\n\t\t\t\tevents[32].Result.NativeError, events[32].Result.NativeErrorToken = Underlying, 71\n\t\t\t\tevents[32].Result.ReturnedError, events[32].Result.ReturnedErrorToken = Underlying, 71\n\t\t\t\tevents[32].Result.NormalizedError, events[32].Result.NormalizedErrorToken = Underlying, 71\n\t\t\tcase \"DB-sync-error-masked\":\n\t\t\t\tevents[32].Result.NativeError, events[32].Result.NativeErrorToken = Underlying, 71\n\t\t\tcase \"prior-write-short\":\n\t\t\t\tevents[30].Result.NativeN = 64\n\t\t\tcase \"public-nil\":\n\t\t\t\tevents[commitAt].Commit.Error, events[commitAt].Commit.ErrorToken = None, 0\n\t\t\tcase \"swapped-class\":\n\t\t\t\tevents[commitAt].Commit.Class = CommitRecoveryRequired\n\t\t\t\tif fixed.which == SQLJournalUnlinkBefore {\n\t\t\t\t\tevents[commitAt].Commit.Class = CommitUncertain\n\t\t\t\t}\n\t\t\tcase \"false-ack\":\n\t\t\t\tevents[commitAt].Commit.Class = CommitSucceeded\n\t\t\tcase \"cleanup-pending\":\n\t\t\t\tevents[commitAt].Commit.Class = CommitCleanupPending\n\t\t\tcase \"logical-open\":\n\t\t\t\tevents[commitAt].Commit.Logical = LogicalOpen\n\t\t\tcase \"wrong-commit-target\":\n\t\t\t\tevents[commitAt].Commit.TargetError = ShortWrite\n\t\t\tcase \"commit-target-token\":\n\t\t\t\tevents[commitAt].Commit.TargetErrorToken = 71\n\t\t\tcase \"duplicate-commit\":\n\t\t\t\tevents = append(events[:commitAt+1], append([]Event{events[commitAt]}, events[commitAt+1:]...)...)\n\t\t\tcase \"pending-owner\":\n\t\t\t\tevents[commitAt+1].Owners.Cleanup = OwnerPending\n\t\t\tcase \"unknown-owner\":\n\t\t\t\tevents[commitAt+1].Owners.Journal = OwnerUnknown\n\t\t\tcase \"physical-validation\":\n\t\t\t\tevents[commitAt+2].Validation = Validation(1)\n\t\t\t}\n\t\t\trenumber(events, plan.Spec().Identity)\n\t\t\tsqlMustReject(t, ObserveSQLLedger(plan, sqlReceipt(t, plan, events), evidence))\n\t\t}\n\t}\n\tplan, _, evidence := sqlEvidenceFixture(t, SQLFinalDirectorySyncBefore)\n\tfor _, name := range []string{\"unlink-skipped\", \"unlink-error\", \"unlink-error-masked\", \"unlink-missing\"} {\n\t\tevents := fixtureEvents(plan.Spec())\n\t\tswitch name {\n\t\tcase \"unlink-skipped\":\n\t\t\tevents[34].Result.Executed = false\n\t\tcase \"unlink-error\":\n\t\t\tevents[34].Result.NativeError, events[34].Result.NativeErrorToken = Underlying, 71\n\t\t\tevents[34].Result.ReturnedError, events[34].Result.ReturnedErrorToken = Underlying, 71\n\t\t\tevents[34].Result.NormalizedError, events[34].Result.NormalizedErrorToken = Underlying, 71\n\t\tcase \"unlink-error-masked\":\n\t\t\tevents[34].Result.NativeError, events[34].Result.NativeErrorToken = Underlying, 71\n\t\tcase \"unlink-missing\":\n\t\t\tevents = append(events[:33], events[35:]...)\n\t\t}\n\t\trenumber(events, plan.Spec().Identity)\n\t\tsqlMustReject(t, ObserveSQLLedger(plan, sqlReceipt(t, plan, events), evidence))\n\t}\n}\n\nfunc TestSQLLedgerPostDBFailuresRequireNewCopyAndExactRecoveryDecision(t *testing.T) {\n\tfor _, fixed := range sqlLiteralPostDBCases() {\n\t\tfor _, name := range []string{\"missing-new-copy\", \"old-copy\", \"mixed-index\", \"same-count-wrong-member\", \"copy-identity\", \"copy-owner-before\", \"copy-owner-after\", \"copy-iterator-before\", \"copy-iterator-after\", \"copy-owner-live\", \"copy-close-error\", \"copy-check\", \"copy-query-plan\", \"wrong-reopen-count\", \"extra-cause\", \"second-commit\", \"rollback\", \"wrong-final-ledger\", \"wrong-recovery-count\", \"wrong-recovery-class\", \"recovery-owner-live\", \"changed-original\", \"changed-trial\", \"changed-post-copy\", \"coherent-old-copies\", \"replay-alias\", \"changed-replay\", \"physical-claim\"} {\n\t\t\tplan, receipt, evidence := sqlEvidenceFixture(t, fixed.which)\n\t\t\tswitch name {\n\t\t\tcase \"missing-new-copy\":\n\t\t\t\tevidence.PostCommit = SQLSnapshotEvidence{}\n\t\t\tcase \"old-copy\":\n\t\t\t\tevidence.PostCommit = sqlSnapshot(evidence.Identity, 103, 15, SQLOld)\n\t\t\tcase \"mixed-index\":\n\t\t\t\tevidence.PostCommit.Queries[SQLBlueIndex] = sqlSnapshot(evidence.Identity, 103, 15, SQLOld).Queries[SQLBlueIndex]\n\t\t\tcase \"same-count-wrong-member\":\n\t\t\t\tevidence.PostCommit.Queries[SQLBlueIndex].Rows[0].ID = 77\n\t\t\tcase \"copy-identity\":\n\t\t\t\tevidence.PostCommit.Identity.Trial = \"wrong-trial\"\n\t\t\tcase \"copy-owner-before\":\n\t\t\t\tevidence.PostCommit = sqlSnapshot(evidence.Identity, evidence.Before.Owner, 15, SQLNew)\n\t\t\tcase \"copy-owner-after\":\n\t\t\t\tevidence.PostCommit = sqlSnapshot(evidence.Identity, evidence.After.Owner, 15, SQLNew)\n\t\t\tcase \"copy-iterator-before\":\n\t\t\t\tevidence.PostCommit.Queries[0].Iterator = evidence.Before.Queries[0].Iterator\n\t\t\tcase \"copy-iterator-after\":\n\t\t\t\tevidence.PostCommit.Queries[0].Iterator = evidence.After.Queries[0].Iterator\n\t\t\tcase \"copy-owner-live\":\n\t\t\t\tevidence.PostCommit.OwnerState = OwnerLive\n\t\t\tcase \"copy-close-error\":\n\t\t\t\tevidence.PostCommit.CloseError, evidence.PostCommit.CloseErrorToken = Underlying, 71\n\t\t\tcase \"copy-check\":\n\t\t\t\tevidence.PostCommit.Check.Calls = 0\n\t\t\tcase \"copy-query-plan\":\n\t\t\t\tevidence.PostCommit.Queries[SQLBlueIndex].Plan.Method = \"scan\"\n\t\t\tcase \"wrong-reopen-count\":\n\t\t\t\tevidence.Lifecycle.ReopenCalls = 1\n\t\t\tcase \"extra-cause\":\n\t\t\t\tevidence.Lifecycle.PostDBOnlyTargetCause = false\n\t\t\tcase \"second-commit\":\n\t\t\t\tevidence.Lifecycle.CommitCalls = 2\n\t\t\tcase \"rollback\":\n\t\t\t\tevidence.Lifecycle.RollbackCalls = 1\n\t\t\tcase \"wrong-final-ledger\":\n\t\t\t\tstate := SQLNew\n\t\t\t\tif fixed.which == SQLFinalDirectorySyncBefore {\n\t\t\t\t\tstate = SQLOld\n\t\t\t\t}\n\t\t\t\tevidence.After = sqlSnapshot(evidence.Identity, 102, 8, state)\n\t\t\tcase \"wrong-recovery-count\":\n\t\t\t\tevidence.Recovery.Calls++\n\t\t\tcase \"wrong-recovery-class\":\n\t\t\t\tevidence.Lifecycle.PublicRecoveryRequired, evidence.Lifecycle.PublicCommitUncertain = evidence.Lifecycle.PublicCommitUncertain, evidence.Lifecycle.PublicRecoveryRequired\n\t\t\tcase \"recovery-owner-live\":\n\t\t\t\tevidence.Recovery.OwnersRetired = false\n\t\t\tcase \"changed-original\":\n\t\t\t\tevidence.Originals.After[0][0]++\n\t\t\tcase \"changed-trial\":\n\t\t\t\tevidence.Artifacts[SQLTrialArtifact].After.SHA256[0]++\n\t\t\tcase \"changed-post-copy\":\n\t\t\t\tevidence.Artifacts[SQLPostCopyArtifact].After.SHA256[0]++\n\t\t\tcase \"coherent-old-copies\":\n\t\t\t\tfor _, role := range []SQLArtifactRole{SQLTrialArtifact, SQLPostCopyArtifact, SQLReplayArtifact} {\n\t\t\t\t\tevidence.Artifacts[role].Before.SHA256 = evidence.Artifacts[SQLBaselineArtifact].Before.SHA256\n\t\t\t\t\tevidence.Artifacts[role].After.SHA256 = evidence.Artifacts[SQLBaselineArtifact].Before.SHA256\n\t\t\t\t}\n\t\t\tcase \"replay-alias\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].Before.Inode = evidence.Artifacts[SQLTrialArtifact].Before.Inode\n\t\t\tcase \"changed-replay\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].After.SHA256[0]++\n\t\t\tcase \"physical-claim\":\n\t\t\t\tevidence.PhysicalClaim = true\n\t\t\t}\n\t\t\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))\n\t\t}\n\t\tfor q := 0; q < SQLQueryCount; q++ {\n\t\t\tfor _, name := range []string{\"eof\", \"err\", \"close\", \"work\", \"value\"} {\n\t\t\t\tplan, receipt, evidence := sqlEvidenceFixture(t, fixed.which)\n\t\t\t\tquery := &evidence.PostCommit.Queries[q]\n\t\t\t\tswitch name {\n\t\t\t\tcase \"eof\":\n\t\t\t\t\tquery.EOF = false\n\t\t\t\tcase \"err\":\n\t\t\t\t\tquery.ErrCalled = false\n\t\t\t\tcase \"close\":\n\t\t\t\t\tquery.CloseCalls = 0\n\t\t\t\tcase \"work\":\n\t\t\t\t\tquery.Work.BaseRowFetches++\n\t\t\t\tcase \"value\":\n\t\t\t\t\tquery.Rows[0].Value = \"wrong\"\n\t\t\t\t}\n\t\t\t\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))\n\t\t\t}\n\t\t}\n\t\tfor _, role := range []SQLArtifactRole{SQLOriginalJournalArtifact, SQLJournalCopyArtifact, SQLReplayJournalArtifact} {\n\t\t\tplan, receipt, evidence := sqlEvidenceFixture(t, fixed.which)\n\t\t\tif fixed.which == SQLJournalUnlinkBefore {\n\t\t\t\tevidence.Artifacts[role] = SQLArtifactEvidence{Role: role}\n\t\t\t} else {\n\t\t\t\tevidence.Artifacts[role].Before = SQLArtifactStamp{Identity: SQLArtifactName(role), Device: 1, Inode: uint64(role + 1), Exists: true, Bytes: 16408, SHA256: [32]byte{33}}\n\t\t\t}\n\t\t\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))\n\t\t}\n\t}\n\tfor _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore, SQLIndexPrefix64Error, SQLJournalBodySyncBefore, SQLJournalHeaderSyncBefore, SQLPublishDirectorySyncBefore} {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, which)\n\t\tif evidence.PostCommit != (SQLSnapshotEvidence{}) || evidence.Lifecycle.PostDBOnlyTargetCause {\n\t\t\tt.Fatal(\"old case gained post-DB observations\")\n\t\t}\n\t\tevidence.PostCommit = sqlSnapshot(evidence.Identity, 103, 15, SQLNew)\n\t\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))\n\t}\n\tfor _, name := range []string{\"missing-recovery\", \"wrong-limits\", \"missing-journal-before\", \"wrong-pages\", \"journal-after\"} {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, SQLJournalUnlinkBefore)\n\t\tswitch name {\n\t\tcase \"missing-recovery\":\n\t\t\tevidence.Recovery.Calls = 0\n\t\tcase \"wrong-limits\":\n\t\t\tevidence.Recovery.Limits.MaxJournalBytes++\n\t\tcase \"missing-journal-before\":\n\t\t\tevidence.Recovery.JournalPresentBefore = false\n\t\tcase \"wrong-pages\":\n\t\t\tevidence.Recovery.ReplayedPages--\n\t\tcase \"journal-after\":\n\t\t\tevidence.Recovery.JournalPresentAfter = true\n\t\t}\n\t\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))\n\t}\n\tfor _, role := range []SQLArtifactRole{SQLOriginalJournalArtifact, SQLJournalCopyArtifact, SQLReplayJournalArtifact} {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, SQLFinalDirectorySyncBefore)\n\t\tevidence.Artifacts[role].After.Exists = true\n\t\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))\n\t}\n\tplan, _, evidence := sqlEvidenceFixture(t, SQLFinalDirectorySyncBefore)\n\tfor value := 11; value <= 255; value++ {\n\t\twhich := SQLCase(value)\n\t\tif _, problem := SQLPlan(plan.Spec().Identity, which); problem == NoProblem || validSQLRecovery(which, evidence.Recovery) || validSQLArtifacts(which, evidence.Artifacts) {\n\t\t\tt.Fatalf(\"future SQL case %d received a classification\", value)\n\t\t}\n\t}\n}\n\nfunc TestSQLLedgerMisplacementRequiresCompleteOldAndActualEvidence(t *testing.T) {\n\tplan, receipt, evidence := sqlEvidenceFixture(t, SQLIndexMisplaceMinus64Error)\n\tobserved := ObserveSQLLedger(plan, receipt, evidence)\n\tif !observed.EvidenceConsistent || !observed.ScenarioObserved || observed.Problem != SQLProblemNone || observed.Adapter.EventCount != 35 || observed.Adapter.TargetHits != 1 || observed.Pass() || observed.PhysicalValidated() {\n\t\tt.Fatal(\"fixed misplaced-write evidence rejected or became physical\", observed)\n\t}\n\tif int(SQLIndexMisplaceMinus64Error) != 10 || isSQLPreDBFailure(SQLIndexMisplaceMinus64Error) || isSQLPostDBFailure(SQLIndexMisplaceMinus64Error) || !needsSQLRecovery(SQLIndexMisplaceMinus64Error) || evidence.PostCommit != (SQLSnapshotEvidence{}) || evidence.Lifecycle.ReopenCalls != 1 {\n\t\tt.Fatal(\"misplaced-write case partition changed\")\n\t}\n\tfor _, name := range []string{\"missing\", \"requested-offset\", \"requested-length\", \"native-correct-offset\", \"native-plus64\", \"native-minus128\", \"native-zero\", \"native-prefix\", \"native-short\", \"zero-calls\", \"two-calls\", \"zero-payload\", \"wrong-payload\", \"baseline-payload\", \"zero-baseline-range\", \"changed-tail\", \"zero-tails\", \"journal-missing\", \"journal-wrong\", \"journal-reordered\", \"extra-close-cause\", \"NEW\", \"mixed-index\", \"wrong-member\", \"post-copy-NEW\", \"recovery-zero\", \"recovery-two\", \"wrong-limits\", \"journal-before-absent\", \"journal-after-present\", \"owners-live\", \"replay-alias\", \"changed-original\", \"changed-trial\", \"replay-NEW\", \"public-success\", \"public-uncertain\", \"public-short\", \"public-pending\", \"pre-DB-cause\", \"post-DB-cause\", \"two-reopens\", \"physical\"} {\n\t\tbad := evidence\n\t\tm := &bad.Misplacement\n\t\tswitch name {\n\t\tcase \"missing\":\n\t\t\t*m = SQLMisplacementEvidence{}\n\t\tcase \"requested-offset\":\n\t\t\tm.Writer.RequestedOffset = 8128\n\t\tcase \"requested-length\":\n\t\t\tm.Writer.RequestedBytes = 64\n\t\tcase \"native-correct-offset\":\n\t\t\tm.Writer.NativeOffset = 8192\n\t\tcase \"native-plus64\":\n\t\t\tm.Writer.NativeOffset = 8256\n\t\tcase \"native-minus128\":\n\t\t\tm.Writer.NativeOffset = 8064\n\t\tcase \"native-zero\":\n\t\t\tm.Writer.NativeOffset = 0\n\t\tcase \"native-prefix\":\n\t\t\tm.Writer.NativeBytes = 64\n\t\tcase \"native-short\":\n\t\t\tm.Writer.NativeBytes = 4095\n\t\tcase \"zero-calls\":\n\t\t\tm.Writer.NativeCalls = 0\n\t\tcase \"two-calls\":\n\t\t\tm.Writer.NativeCalls = 2\n\t\tcase \"zero-payload\":\n\t\t\tm.Writer.PayloadSHA256, m.ShiftedSHA256 = [32]byte{}, [32]byte{}\n\t\tcase \"wrong-payload\":\n\t\t\tm.ShiftedSHA256[0]++\n\t\tcase \"baseline-payload\":\n\t\t\tm.BaselineRangeSHA256 = m.Writer.PayloadSHA256\n\t\tcase \"zero-baseline-range\":\n\t\t\tm.BaselineRangeSHA256 = [32]byte{}\n\t\tcase \"changed-tail\":\n\t\t\tm.UntouchedTailSHA256[0]++\n\t\tcase \"zero-tails\":\n\t\t\tm.BaselineTailSHA256, m.UntouchedTailSHA256 = [32]byte{}, [32]byte{}\n\t\tcase \"journal-missing\":\n\t\t\tm.JournalOriginals[2] = [32]byte{}\n\t\tcase \"journal-wrong\":\n\t\t\tm.JournalOriginals[1][0]++\n\t\tcase \"journal-reordered\":\n\t\t\tm.JournalOriginals[1], m.JournalOriginals[2] = m.JournalOriginals[2], m.JournalOriginals[1]\n\t\tcase \"extra-close-cause\":\n\t\t\tm.OnlyTargetCause = false\n\t\tcase \"NEW\":\n\t\t\tbad.After = sqlSnapshot(bad.Identity, 102, 8, SQLNew)\n\t\tcase \"mixed-index\":\n\t\t\tbad.After.Queries[SQLBlueIndex] = sqlSnapshot(bad.Identity, 102, 8, SQLNew).Queries[SQLBlueIndex]\n\t\tcase \"wrong-member\":\n\t\t\tbad.After.Queries[SQLBlueIndex].Rows[0].ID = 77\n\t\tcase \"post-copy-NEW\":\n\t\t\tbad.PostCommit = sqlSnapshot(bad.Identity, 103, 15, SQLNew)\n\t\tcase \"recovery-zero\":\n\t\t\tbad.Recovery.Calls = 0\n\t\tcase \"recovery-two\":\n\t\t\tbad.Recovery.Calls = 2\n\t\tcase \"wrong-limits\":\n\t\t\tbad.Recovery.Limits.MaxJournalBytes++\n\t\tcase \"journal-before-absent\":\n\t\t\tbad.Recovery.JournalPresentBefore = false\n\t\tcase \"journal-after-present\":\n\t\t\tbad.Recovery.JournalPresentAfter = true\n\t\tcase \"owners-live\":\n\t\t\tbad.Recovery.OwnersRetired = false\n\t\tcase \"replay-alias\":\n\t\t\tbad.Artifacts[SQLReplayArtifact].Before.Inode = bad.Artifacts[SQLTrialArtifact].Before.Inode\n\t\tcase \"changed-original\":\n\t\t\tbad.Originals.After[1][0]++\n\t\tcase \"changed-trial\":\n\t\t\tbad.Artifacts[SQLTrialArtifact].After.SHA256[0]++\n\t\tcase \"replay-NEW\":\n\t\t\tbad.Artifacts[SQLReplayArtifact].After.SHA256 = bad.Artifacts[SQLTrialArtifact].Before.SHA256\n\t\tcase \"public-success\":\n\t\t\tbad.Lifecycle.PublicCodeIOErr = false\n\t\tcase \"public-uncertain\":\n\t\t\tbad.Lifecycle.PublicCommitUncertain = true\n\t\tcase \"public-short\":\n\t\t\tbad.Lifecycle.PublicShortWrite = true\n\t\tcase \"public-pending\":\n\t\t\tbad.Lifecycle.PublicCleanupPending = true\n\t\tcase \"pre-DB-cause\":\n\t\t\tbad.Lifecycle.PreDBOnlyTargetCause = true\n\t\tcase \"post-DB-cause\":\n\t\t\tbad.Lifecycle.PostDBOnlyTargetCause = true\n\t\tcase \"two-reopens\":\n\t\t\tbad.Lifecycle.ReopenCalls = 2\n\t\tcase \"physical\":\n\t\t\tbad.PhysicalClaim = true\n\t\t}\n\t\tif observed := ObserveSQLLedger(plan, receipt, bad); observed.EvidenceConsistent || observed.ScenarioObserved || observed.Problem == SQLProblemNone || observed.Pass() || observed.PhysicalValidated() {\n\t\t\tt.Fatal(\"misplaced-write counterexample accepted\", name, observed)\n\t\t}\n\t}\n}\n\nfunc TestSQLLedgerOldNineCasesRequireZeroMisplacement(t *testing.T) {\n\tfor _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore, SQLIndexPrefix64Error, SQLJournalBodySyncBefore, SQLJournalHeaderSyncBefore, SQLPublishDirectorySyncBefore, SQLJournalUnlinkBefore, SQLFinalDirectorySyncBefore} {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, which)\n\t\tif evidence.Misplacement != (SQLMisplacementEvidence{}) {\n\t\t\tt.Fatal(\"old case gained misplacement evidence\", which)\n\t\t}\n\t\tfor _, name := range []string{\"requested-offset\", \"requested-bytes\", \"native-offset\", \"native-bytes\", \"native-calls\", \"payload\", \"shifted\", \"baseline-range\", \"baseline-tail\", \"tail\", \"journal\", \"cause\"} {\n\t\t\tbad := evidence\n\t\t\tm := &bad.Misplacement\n\t\t\tswitch name {\n\t\t\tcase \"requested-offset\":\n\t\t\t\tm.Writer.RequestedOffset = 8192\n\t\t\tcase \"requested-bytes\":\n\t\t\t\tm.Writer.RequestedBytes = 4096\n\t\t\tcase \"native-offset\":\n\t\t\t\tm.Writer.NativeOffset = 8128\n\t\t\tcase \"native-bytes\":\n\t\t\t\tm.Writer.NativeBytes = 4096\n\t\t\tcase \"native-calls\":\n\t\t\t\tm.Writer.NativeCalls = 1\n\t\t\tcase \"payload\":\n\t\t\t\tm.Writer.PayloadSHA256[0] = 44\n\t\t\tcase \"shifted\":\n\t\t\t\tm.ShiftedSHA256[0] = 44\n\t\t\tcase \"baseline-range\":\n\t\t\t\tm.BaselineRangeSHA256[0] = 55\n\t\t\tcase \"baseline-tail\":\n\t\t\t\tm.BaselineTailSHA256[0] = 66\n\t\t\tcase \"tail\":\n\t\t\t\tm.UntouchedTailSHA256[0] = 66\n\t\t\tcase \"journal\":\n\t\t\t\tm.JournalOriginals[0][0] = 11\n\t\t\tcase \"cause\":\n\t\t\t\tm.OnlyTargetCause = true\n\t\t\t}\n\t\t\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, bad))\n\t\t}\n\t}\n}\n",
}

// These closed semantic decisions are independent of the supplemental whole-file
// snapshots. Repinning a counterexample cannot alter the allowed control flow.
func rollbackFaultSQLExactFunction(path string, functions map[string]*ast.FuncDecl, name, diagnostic, expected string) []string {
	fn := functions[name]
	if fn != nil {
		shape := *fn
		shape.Doc = nil
		fn = &shape
	}
	// Parse this fixed semantic literal independently of mutable file snapshots.
	// Compact both ASTs so source formatting and trailing commas confer nothing.
	frozen, err := parser.ParseFile(token.NewFileSet(), "closed_sql_semantic.go", "package frozen\n"+expected, parser.ParseComments)
	if err != nil || len(frozen.Decls) != 1 {
		return []string{path + ": rollback SQL contract: invalid closed semantic literal: " + name}
	}
	want, ok := frozen.Decls[0].(*ast.FuncDecl)
	if !ok || fn == nil || readOnlyWALManualCompact(fn) != readOnlyWALManualCompact(want) {
		return []string{path + ": rollback SQL contract: " + diagnostic}
	}
	return nil
}

func rollbackFaultSQLOwned(path string) bool {
	return path == rollbackFaultSQLBridge || path == rollbackFaultSQLNative || path == rollbackFaultSQLModel || path == rollbackFaultSQLModelTest
}
func rollbackFaultSQLManual(path string) bool {
	return path == rollbackFaultSQLBridge || path == rollbackFaultSQLNative
}
func rollbackFaultSQLAPI(name string) bool {
	switch name {
	case "ArmManualRollbackSQLFault", "FinishManualRollbackSQLFault", "DisarmManualRollbackSQLFault", "ManualRollbackSQLState":
		return true
	}
	return false
}

// The older scope guard admits only these three direct bridge calls. There is
// no whole-file exemption for old entries, aliases, or function values.
func rollbackFaultSQLOldEntry(path string, id *ast.Ident, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte) bool {
	if path != rollbackFaultSQLBridge {
		return false
	}
	call := rollbackFaultDirectReference(id, parents)
	fn := oracleEnclosingFunction(id, parents)
	if call == nil || fn == nil || fn.Recv != nil {
		return false
	}
	want := map[string]string{
		"ArmManualRollbackSQLFault":    "armRollbackFault(tx, plan)",
		"FinishManualRollbackSQLFault": "finishRollbackFault(scope.scope, commitErr)",
		"DisarmManualRollbackSQLFault": "disarmRollbackFault(scope.scope)",
	}
	return rollbackFaultText(set, raw, call) == want[fn.Name.Name]
}
func rollbackFaultSQLOldPrivate(path string, id *ast.Ident, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte) bool {
	if path != rollbackFaultSQLBridge {
		return false
	}
	if id.Name == "rollbackFaultScope" {
		pointer, ok := parents[id].(*ast.StarExpr)
		if !ok {
			return false
		}
		field, ok := parents[pointer].(*ast.Field)
		if !ok || rollbackFaultText(set, raw, field) != "scope *rollbackFaultScope" {
			return false
		}
		for n := ast.Node(field); n != nil; n = parents[n] {
			if typ, ok := n.(*ast.TypeSpec); ok {
				return typ.Name.Name == "manualRollbackSQLScope" && !typ.Assign.IsValid()
			}
		}
		return false
	}
	fn := oracleEnclosingFunction(id, parents)
	if fn == nil || fn.Recv != nil {
		return false
	}
	if id.Name == "rollbackFaultRegistry" {
		if fn.Name.Name != "ManualRollbackSQLState" {
			return false
		}
		sel, ok := parents[id].(*ast.SelectorExpr)
		if !ok || sel.X != id {
			return false
		}
		switch rollbackFaultText(set, raw, sel) {
		case "rollbackFaultRegistry.mu", "rollbackFaultRegistry.active", "rollbackFaultRegistry.used":
			return true
		}
		return false
	}
	if id.Name == "errRollbackFaultInjected" {
		comparison, ok := parents[id].(*ast.BinaryExpr)
		return fn.Name.Name == "manualRollbackSQLOnlyTargetCause" && ok && comparison.Y == id && rollbackFaultText(set, raw, comparison) == "target != errRollbackFaultInjected"
	}
	if id.Name == "errRollbackFaultGate" {
		return fn.Name.Name == "ArmManualRollbackSQLFault"
	}
	if id.Name == "errRollbackFaultScope" {
		return fn.Name.Name == "ArmManualRollbackSQLFault" || fn.Name.Name == "FinishManualRollbackSQLFault" || fn.Name.Name == "DisarmManualRollbackSQLFault"
	}
	return false
}

// Recovery remains restricted everywhere except this one direct call in this
// exact helper/path/build selection. Merely copying a file or tag grants none.
func rollbackFaultSQLRecoveryReference(path string, id *ast.Ident, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte) bool {
	if filepath.ToSlash(filepath.Clean(path)) != rollbackFaultSQLNative || id.Name != "RecoverRollback" || strings.SplitN(string(raw), "\n", 2)[0] != rollbackFaultManualTag {
		return false
	}
	call := walPublicationDirectCall(id, parents)
	fn := oracleEnclosingFunction(id, parents)
	if call == nil || fn == nil || fn.Recv != nil || fn.Name.Name != "manualRollbackSQLRecoverCopy" {
		return false
	}
	if fn.Type.Params == nil || fn.Type.Results == nil || rollbackFaultText(set, raw, fn.Type.Params) != "(ctx context.Context, replayPath string)" || rollbackFaultText(set, raw, fn.Type.Results) != "(pager.RecoveryResult, error)" {
		return false
	}
	var file *ast.File
	for node := ast.Node(fn); node != nil; node = parents[node] {
		if found, ok := node.(*ast.File); ok {
			file = found
			break
		}
	}
	if file == nil {
		return false
	}
	pagerBinding, journalBinding := false, false
	for _, imp := range file.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return false
		}
		if value == "github.com/ms-xiaosnow/gosqlite/internal/pager" && imp.Name == nil {
			pagerBinding = true
		}
		if value == "github.com/ms-xiaosnow/gosqlite/internal/journal" && imp.Name == nil {
			journalBinding = true
		}
	}
	if !pagerBinding || !journalBinding {
		return false
	}

	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok || qualifier.Name != "pager" {
		return false
	}
	return rollbackFaultText(set, raw, call) == rollbackFaultSQLRecoveryCall
}

// Only the existing ordinary example fields share the model entry's spelling.
// This is a closed syntactic binding check, not general Go type inference: no
// aliases, shadowed types, computed receivers, methods or function values are
// admitted. The two tests may read only their exact same-package main source.
func rollbackFaultSQLOrdinaryPlanIdentifiers(path string, file *ast.File, parents map[ast.Node]ast.Node, readSource func(string) ([]byte, error)) (map[*ast.Ident]bool, error) {
	var sourcePath, typeName string
	switch path {
	case "examples/ann/main.go", "examples/ann/main_test.go":
		sourcePath, typeName = "examples/ann/main.go", "output"
	case "examples/vector/main.go", "examples/vector/main_test.go":
		sourcePath, typeName = "examples/vector/main.go", "hybridOutput"
	default:
		return nil, nil
	}
	if file.Name.Name != "main" {
		return nil, nil
	}
	typeFile := file
	if path != sourcePath {
		raw, err := readSource(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("%s: ordinary SQLPlan companion %s: %w", path, sourcePath, err)
		}
		typeFile, err = parser.ParseFile(token.NewFileSet(), sourcePath, raw, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		if typeFile.Name.Name != file.Name.Name || file.Scope.Lookup(typeName) != nil {
			return nil, nil
		}
	}
	bindings := 0
	for _, imp := range typeFile.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(value)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if value == rollbackFaultImport {
			return nil, nil
		}
		if name == "sqlite" {
			if imp.Name != nil || value != "github.com/ms-xiaosnow/gosqlite/sqlite" {
				return nil, nil
			}
			bindings++
		}
	}
	if bindings != 1 || typeFile.Scope.Lookup("sqlite") != nil {
		return nil, nil
	}
	var declared *ast.TypeSpec
	for _, declaration := range typeFile.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typ, ok := spec.(*ast.TypeSpec)
			if ok && typ.Name.Name == typeName {
				if declared != nil || typ.Assign.IsValid() || typ.TypeParams != nil {
					return nil, nil
				}
				declared = typ
			}
		}
	}
	if declared == nil {
		return nil, nil
	}
	structure, ok := declared.Type.(*ast.StructType)
	if !ok {
		return nil, nil
	}
	var fieldName *ast.Ident
	for _, field := range structure.Fields.List {
		for _, name := range field.Names {
			if name.Name != "SQLPlan" {
				continue
			}
			typ, ok := field.Type.(*ast.SelectorExpr)
			if fieldName != nil || len(field.Names) != 1 || !ok || typ.Sel.Name != "QueryPlan" {
				return nil, nil
			}
			qualifier, ok := typ.X.(*ast.Ident)
			if !ok || qualifier.Name != "sqlite" || qualifier.Obj != nil {
				return nil, nil
			}
			fieldName = name
		}
	}
	if fieldName == nil {
		return nil, nil
	}
	isType := func(expr ast.Expr) bool {
		id, ok := expr.(*ast.Ident)
		if !ok || id.Name != typeName {
			return false
		}
		if typeFile == file {
			return id.Obj != nil && id.Obj == declared.Name.Obj
		}
		return id.Obj == nil
	}
	allowed := map[*ast.Ident]bool{}
	if typeFile == file {
		allowed[fieldName] = true
	}
	ast.Inspect(file, func(node ast.Node) bool {
		sel, ok := node.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "SQLPlan" {
			return true
		}
		receiver, ok := sel.X.(*ast.Ident)
		if !ok || receiver.Obj == nil || receiver.Obj.Kind != ast.Var {
			return true
		}
		fn := oracleEnclosingFunction(sel, parents)
		bound := false
		switch declaration := receiver.Obj.Decl.(type) {
		case *ast.ValueSpec:
			bound = fn != nil && oracleEnclosingFunction(declaration, parents) == fn && len(declaration.Names) == 1 && len(declaration.Values) == 0 && isType(declaration.Type)
		case *ast.AssignStmt:
			if fn != nil && oracleEnclosingFunction(declaration, parents) == fn && declaration.Tok == token.DEFINE && len(declaration.Lhs) == 1 && len(declaration.Rhs) == 1 {
				literal, ok := declaration.Rhs[0].(*ast.CompositeLit)
				bound = ok && isType(literal.Type)
			}
		}
		if !bound {
			return true
		}
		switch use := parents[sel].(type) {
		case *ast.AssignStmt:
			allowed[sel.Sel] = use.Tok == token.ASSIGN && len(use.Lhs) == 1 && len(use.Rhs) == 1 && use.Lhs[0] == sel
		case *ast.SelectorExpr:
			// Only the existing scalar QueryPlan reads are needed. In
			// particular SQLPlan itself is never a call or retained value.
			if use.X == sel && (use.Sel.Name == "Method" || use.Sel.Name == "VectorExact" || use.Sel.Name == "VectorRanking") {
				if _, ok := parents[use].(*ast.BinaryExpr); ok {
					allowed[sel.Sel] = true
				}
			}
		}
		return true
	})
	return allowed, nil
}

func inspectRollbackFaultSQLSource(path string, raw []byte) ([]string, error) {
	return inspectRollbackFaultSQLSourceWithReader(path, raw, os.ReadFile)
}

func inspectRollbackFaultSQLSourceWithReader(path string, raw []byte, readSource func(string) ([]byte, error)) ([]string, error) {
	path = filepath.ToSlash(filepath.Clean(path))
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, raw, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var findings []string
	report := func(message string) { findings = append(findings, path+": rollback SQL contract: "+message) }
	owned, manual := rollbackFaultSQLOwned(path), rollbackFaultSQLManual(path)
	pure := path == rollbackFaultSQLModel || path == rollbackFaultSQLModelTest
	first := strings.SplitN(string(raw), "\n", 2)[0]
	if strings.HasPrefix(path, "internal/pager/rollback_fault_sql") || strings.HasPrefix(path, "sqlite/rollback_fault_sql") || strings.HasPrefix(path, "internal/testkit/rollbackfault/sql_") {
		if !owned {
			report("unreviewed SQL adapter/model/caller source")
		}
	}
	if owned {
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != rollbackFaultSQLPins[path] {
			report("proposed source snapshot changed or pending")
		}
		if expected, ok := rollbackFaultSQLShapes[path]; ok {
			findings = append(findings, rollbackFaultDeclarationShape(path, raw, set, file, expected)...)
		} else {
			report("supplemental declaration snapshot pending")
		}
		if manual {
			if first != rollbackFaultManualTag || strings.Count(string(raw), "//go:build") != 1 || strings.Contains(string(raw), "// +build") {
				report("exact Linux AND manual SQL gate required")
			}
		} else if strings.Contains(string(raw), "//go:build") || strings.Contains(string(raw), "// +build") {
			report("portable SQL model cannot add build constraints")
		}
		wantPackage := "rollbackfault"
		if path == rollbackFaultSQLBridge {
			wantPackage = "pager"
		}
		if path == rollbackFaultSQLNative {
			wantPackage = "sqlite"
		}
		if file.Name.Name != wantPackage {
			report("exact owned package required")
		}
	}
	imports := map[string]string{}
	for _, imp := range file.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(value)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		imports[name] = value
		if value == rollbackFaultImport && (imp.Name != nil || path != rollbackFaultManual && path != rollbackFaultSQLBridge && path != rollbackFaultSQLNative) && !restartGuardFaultImport(path, imp) && !(imp.Name == nil && rollbackFaultRecoveryNativePinned(path, raw)) {
			report("contract import outside exact unaliased consumers")
		}
		if owned && imp.Name != nil {
			report("aliased, blank or dot import forbidden")
		}
		if pure && (path == rollbackFaultSQLModel || value != "testing") {
			report("SQL model has only scalar memory imports")
		}
		if owned && (value == "unsafe" || value == "reflect" && path != rollbackFaultSQLBridge || value == "os/exec" || strings.HasPrefix(value, "net") || value == "golang.org/x/sys/unix") {
			report("process/network/reflection import forbidden")
		}
		if path == rollbackFaultSQLBridge && value != "os" && value != "errors" && value != "reflect" && value != rollbackFaultImport {
			report("bridge imports outside exact scope")
		}
		if path == rollbackFaultSQLNative {
			allowed := "bytes context crypto/sha256 encoding/binary encoding/hex encoding/json errors fmt io os path/filepath strings syscall testing github.com/ms-xiaosnow/gosqlite/internal/journal github.com/ms-xiaosnow/gosqlite/internal/pager " + rollbackFaultImport
			if !strings.Contains(" "+allowed+" ", " "+value+" ") {
				report("manual caller import outside closed source scope")
			}
		}
	}
	parents := oracleParents(file)
	functions := readOnlyWALManualFunctions(file)
	roots := 0
	for _, declaration := range file.Decls {
		if gen, ok := declaration.(*ast.GenDecl); ok && owned && gen.Tok == token.VAR {
			report("owned SQL source cannot add package state")
		}
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if owned && (fn.Name.Name == "init" || fn.Name.Name == "TestMain") {
			report("initializer or TestMain forbidden")
		}
		entry := strings.HasPrefix(fn.Name.Name, "Test") || strings.HasPrefix(fn.Name.Name, "Benchmark") || strings.HasPrefix(fn.Name.Name, "Fuzz") || strings.HasPrefix(fn.Name.Name, "Example")
		if manual && entry {
			roots++
			if path != rollbackFaultSQLNative || fn.Name.Name != rollbackFaultSQLRoot || fn.Recv != nil {
				report("one exact manual SQL root required")
			}
		}
		if owned && fn.Body == nil {
			report("external/assembly owned declaration forbidden")
		}
	}
	if manual && ((path == rollbackFaultSQLNative && roots != 1) || (path == rollbackFaultSQLBridge && roots != 0)) {
		report("one exact manual SQL root required")
	}
	ordinarySQLPlan, err := rollbackFaultSQLOrdinaryPlanIdentifiers(path, file, parents, readSource)
	if err != nil {
		return nil, err
	}
	commitReferences, recoveryReferences := 0, 0
	apiCalls := map[string]int{}
	ast.Inspect(file, func(node ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok {
			if !owned {
				switch id.Name {
				case "SQLPlan":
					if !ordinarySQLPlan[id] && !restartGuardSQLReference(path, id, parents, set, raw) {
						report("SQL model declaration/reference outside exact source")
					}
				case "ObserveSQLLedger", "ValidateSQLSnapshot", "SQLLedgerRows", "SQLQueryText", "SQLArtifactName", "SQLObservation", "SQLLedgerEvidence":
					if !restartGuardSQLReference(path, id, parents, set, raw) {
						report("SQL model declaration/reference outside exact source")
					}
				}
			}
			if strings.HasPrefix(id.Name, "manualRollbackSQL") || strings.HasPrefix(id.Name, "ManualRollbackSQL") || id.Name == rollbackFaultSQLRoot {
				if !manual && !restartGuardSQLReference(path, id, parents, set, raw) {
					report("SQL manual declaration/reference outside exact source")
				}
			}
			if id.Name == "manualRollbackSQLMisplacement" {
				declaration, declares := parents[id].(*ast.FuncDecl)
				call := rollbackFaultDirectReference(id, parents)
				fn := oracleEnclosingFunction(id, parents)
				if !(declares && declaration.Name == id && path == rollbackFaultSQLNative && declaration.Recv == nil) && !(path == rollbackFaultSQLNative && fn != nil && fn.Name.Name == rollbackFaultSQLRoot && call != nil && rollbackFaultText(set, raw, call) == "manualRollbackSQLMisplacement(report, baseline[:], post, journalBytes)") {
					report("fixed misplaced-write helper only from original captured buffers")
				}
			}
			if id.Name == "manualRollbackSQLOnlyTargetCause" {
				declaration, declares := parents[id].(*ast.FuncDecl)
				call := rollbackFaultDirectReference(id, parents)
				fn := oracleEnclosingFunction(id, parents)
				if !(declares && declaration.Name == id && path == rollbackFaultSQLBridge && declaration.Recv == nil) && !(path == rollbackFaultSQLBridge && fn != nil && fn.Name.Name == "FinishManualRollbackSQLFault" && call != nil && (rollbackFaultText(set, raw, call) == "manualRollbackSQLOnlyTargetCause(commitErr, poison, target, nil)" || rollbackFaultText(set, raw, call) == "manualRollbackSQLOnlyTargetCause(commitErr, poison, target, permitted)" || rollbackFaultText(set, raw, call) == "manualRollbackSQLOnlyTargetCause(commitErr, poison, target, ErrRecoveryRequired)")) {
					report("pre-DB cause observer cannot be copied, aliased or called outside exact Finish")
				}
			}
			if rollbackFaultSQLAPI(id.Name) {
				if fn, ok := parents[id].(*ast.FuncDecl); ok && fn.Name == id && path == rollbackFaultSQLBridge && fn.Recv == nil {
					return true
				}
				call := walPublicationDirectCall(id, parents)
				fn := oracleEnclosingFunction(id, parents)
				allowed := false
				if call != nil && fn != nil {
					text := rollbackFaultText(set, raw, call)
					if path == rollbackFaultSQLNative {
						switch fn.Name.Name {
						case "manualRollbackSQLCommit":
							allowed = text == "pager.ArmManualRollbackSQLFault(tx.storage, plan)" || text == "pager.FinishManualRollbackSQLFault(scope, commitErr)" || text == "pager.DisarmManualRollbackSQLFault(scope)"
						case "manualRollbackSQLRegistryEmpty":
							allowed = text == "pager.ManualRollbackSQLState(nil)"
						case "manualRollbackSQLAbandon":
							allowed = text == "pager.ManualRollbackSQLState(tx.storage)"
						}
					} else if path == rollbackFaultSQLBridge {
						allowed = fn.Name.Name == "ArmManualRollbackSQLFault" && text == "ManualRollbackSQLState(tx)" || fn.Name.Name == "DisarmManualRollbackSQLFault" && text == "ManualRollbackSQLState(nil)"
					}
					if allowed {
						apiCalls[id.Name]++
					}
				}
				if !allowed && !restartGuardSQLReference(path, id, parents, set, raw) {
					report("SQL bridge entry outside exact direct caller; aliases/function values/copies forbidden")
				}
			}
			if owned && id.Name == "RecoverRollback" {
				recoveryReferences++
				if !rollbackFaultSQLRecoveryReference(path, id, parents, set, raw) {
					report("recovery outside exact helper/path/call/limits")
				}
			}
		}
		if !owned {
			return true
		}
		if path == rollbackFaultSQLBridge {
			if call, ok := node.(*ast.CallExpr); ok {
				switch function := call.Fun.(type) {
				case *ast.Ident:
					switch function.Name {
					case "len", "ManualRollbackSQLState", "armRollbackFault", "finishRollbackFault", "disarmRollbackFault", "manualRollbackSQLOnlyTargetCause":
					default:
						report("bridge closed call inventory forbids registry mutation or dynamic execution: " + function.Name)
					}
				case *ast.SelectorExpr:
				default:
					report("bridge closed call inventory forbids dynamic execution")
				}
			}
		}
		switch node.(type) {
		case *ast.GoStmt, *ast.SendStmt:
			report("concurrency or hidden closure execution forbidden")
		case *ast.FuncLit:
			if path != rollbackFaultSQLModelTest {
				report("concurrency or hidden closure execution forbidden")
			}
		}
		if deferred, ok := node.(*ast.DeferStmt); ok {
			if path != rollbackFaultSQLNative || rollbackFaultText(set, raw, deferred.Call) != "c.ledger.finish(t)" {
				report("deferred native cleanup/retry forbidden")
			}
		}
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		call, called := parents[sel].(*ast.CallExpr)
		called = called && call.Fun == sel
		fn := oracleEnclosingFunction(sel, parents)
		if sel.Sel.Name == "Commit" {
			if pure && !called {
				return true
			}
			commitReferences++
			if path != rollbackFaultSQLNative || fn == nil || fn.Name.Name != "manualRollbackSQLCommit" || !called || rollbackFaultText(set, raw, call) != "tx.Commit(ctx)" {
				report("one public SQL Commit only; no storage bypass/alias/retry")
			}
		}
		switch sel.Sel.Name {
		case "Setenv", "Unsetenv", "LookupEnv", "Environ", "RetryPlaintextCleanup", "RetryCleanup", "Reset", "Fd", "SyscallConn", "RawConn", "Kill", "Signal", "Command", "CommandContext", "Exit", "Remove", "RemoveAll", "Rename", "Link", "Symlink", "Chmod", "Truncate", "Mkdir", "MkdirAll", "MkdirTemp", "CreateTemp", "TempDir", "Parallel", "Skip", "Skipf", "SkipNow":
			report("forbidden ownership/process/cleanup/test control reference " + sel.Sel.Name)
		}
		if manual && sel.Sel.Name == "Run" {
			report("subtest or hidden execution forbidden")
		}
		if pure {
			switch sel.Sel.Name {
			case "Open", "OpenFile", "Create", "ReadFile", "WriteFile", "Close", "Sync", "Begin", "RecoverRollback", "Getenv", "Rollback":
				report("non-memory SQL model reference " + sel.Sel.Name)
			}
		}
		if path == rollbackFaultSQLBridge {
			if qualifier, ok := sel.X.(*ast.Ident); ok && (qualifier.Name == "reflect" || qualifier.Name == "errors") {
				admitted := false
				if called && fn != nil && fn.Name.Name == "manualRollbackSQLOnlyTargetCause" {
					switch rollbackFaultText(set, raw, call) {
					case "reflect.TypeOf(errors.Join(ErrPoisoned, target))", "reflect.TypeOf(current)", "errors.Join(ErrPoisoned, target)":
						admitted = true
					}
				}
				if !admitted {
					report("pre-DB reflection/error access outside exact bounded cause observer")
				}
			}
		}
		if path == rollbackFaultSQLBridge && called {
			name := readOnlyWALManualName(call.Fun)
			allowed := map[string]bool{"os.Getenv": true, "plainRetirements.mu.Lock": true, "plainRetirements.mu.Unlock": true, "g.pending.Load": true, "rollbackFaultRegistry.mu.Lock": true, "rollbackFaultRegistry.mu.Unlock": true, "tx.mu.Lock": true, "tx.mu.Unlock": true, "g.mu.Lock": true, "g.mu.Unlock": true, "plan.Spec": true, "rollbackfault.SQLPlan": true, "frozen.Spec": true}
			admitted := allowed[name]
			if fn != nil && fn.Name.Name == "FinishManualRollbackSQLFault" {
				switch rollbackFaultText(set, raw, call) {
				case "scope.scope.mu.Lock()", "scope.scope.mu.Unlock()", "scope.scope.plan.Spec()", "scope.scope.tx.mu.Lock()", "scope.scope.tx.mu.Unlock()":
					admitted = true
				}
			}
			if fn != nil && fn.Name.Name == "manualRollbackSQLOnlyTargetCause" {
				switch rollbackFaultText(set, raw, call) {
				case "public.Unwrap()", "reflect.TypeOf(errors.Join(ErrPoisoned, target))", "errors.Join(ErrPoisoned, target)", "reflect.TypeOf(current)", "current.(interface{ Unwrap() []error }).Unwrap()":
					admitted = true
				}
			}
			if !admitted {
				report("bridge cannot call native owner/file/cleanup method " + name)
			}
		}
		if path == rollbackFaultSQLNative && (sel.Sel.Name == "ParseReplay" || sel.Sel.Name == "ValidateExtent") {
			want := "journal.ParseReplay(journalBytes, journal.ReplayLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288})"
			if sel.Sel.Name == "ValidateExtent" {
				want = "plan.ValidateExtent(12288)"
			}
			if !called || fn == nil || fn.Name.Name != "manualRollbackSQLMisplacement" || rollbackFaultText(set, raw, call) != want {
				report("fixed misplaced-write parser and extent call arguments")
			}
		}
		if path == rollbackFaultSQLNative {
			if qualifier, ok := sel.X.(*ast.Ident); ok && qualifier.Name == "syscall" && sel.Sel.Name != "Stat_t" {
				report("syscall capability outside read-only stat metadata")
			}
			critical := map[string]map[string]string{
				"Begin":    {"manualRollbackSQLCommit": "db.Begin(ctx, TxOptions{Immediate: true})"},
				"Exec":     {"manualRollbackSQLCommit": "tx.Exec(ctx, statement)"},
				"Query":    {"manualRollbackSQLQuery": "db.Query(ctx, evidence.SQL)"},
				"Check":    {"manualRollbackSQLReadback": "db.Check(ctx, CheckOptions{MaxPages: 3, MaxPageReads: 512})"},
				"OpenFile": {"create": "os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)"},
				"Sync":     {"create": "f.Sync()"},
				"Write":    {"create": "f.Write(data)"},
			}
			if locations, restricted := critical[sel.Sel.Name]; restricted {
				checkEvidence := false
				if sel.Sel.Name == "Check" && !called && fn != nil && fn.Name.Name == "manualRollbackSQLReadback" && rollbackFaultText(set, raw, sel) == "evidence.Check" {
					if assignment, ok := parents[sel].(*ast.AssignStmt); ok && len(assignment.Lhs) == 1 && assignment.Lhs[0] == sel && len(assignment.Rhs) == 1 {
						if literal, ok := assignment.Rhs[0].(*ast.CompositeLit); ok {
							checkEvidence = readOnlyWALManualName(literal.Type) == "rollbackfault.SQLCheckEvidence"
						}
					}
				}
				if !checkEvidence && (!called || fn == nil || rollbackFaultText(set, raw, call) != locations[fn.Name.Name]) {
					report("native owner method outside exact acquisition/use boundary: " + sel.Sel.Name)
				}
			}
			if sel.Sel.Name == "Close" {
				text := ""
				if called {
					text = rollbackFaultText(set, raw, call)
				}
				if fn == nil || fn.Name.Name != "close" || (text != "o.file.Close()" && text != "o.rows.Close()" && text != "o.db.Close()") {
					report("owner Close must remain in once-only ledger boundary")
				}
			}
			if sel.Sel.Name == "Getenv" && (!called || fn == nil || fn.Name.Name != "manualRollbackSQLAdmit" && fn.Name.Name != rollbackFaultSQLRoot) {
				report("manual SQL environment read outside explicit admission gates")
			}
		}
		if sel.Sel.Name == "Rollback" && !pure {
			if path != rollbackFaultSQLNative || fn == nil || fn.Name.Name != "manualRollbackSQLAbandon" || !called || rollbackFaultText(set, raw, call) != "tx.Rollback()" {
				report("Rollback restricted to healthy pre-arm abandonment")
			}
		}
		return true
	})
	if path == rollbackFaultSQLNative {
		if readOnlyWALManualCountCall(file, "manualRollbackSQLMisplacement") != 1 || readOnlyWALManualCountCall(file, "journal.ParseReplay") != 1 || readOnlyWALManualCountCall(file, "plan.ValidateExtent") != 1 {
			report("one fixed misplaced-write byte/journal proof required")
		}
	}
	if path == rollbackFaultSQLNative {
		if commitReferences != 1 {
			report("exactly one public SQL Commit reference required")
		}
		if recoveryReferences != 1 {
			report("exactly one explicit recovery reference required")
		}
		for _, api := range []string{"ArmManualRollbackSQLFault", "FinishManualRollbackSQLFault", "DisarmManualRollbackSQLFault"} {
			if apiCalls[api] != 1 {
				report("unique bridge entry call required: " + api)
			}
		}
		findings = append(findings, rollbackFaultSQLNativeSemantics(path, raw, set, file, functions)...)
	}
	if path == rollbackFaultSQLBridge {
		findings = append(findings, rollbackFaultSQLBridgeSemantics(path, raw, set, file, functions)...)
	}
	if path == rollbackFaultSQLModel {
		findings = append(findings, rollbackFaultSQLModelSemantics(path, raw, set, file, functions)...)
	}
	if path == "internal/testscope/main.go" {
		findings = append(findings, inspectRollbackFaultSQLWiring(raw)...)
	}
	return findings, nil
}

func rollbackFaultSQLRequire(path string, functions map[string]*ast.FuncDecl, name, diagnostic string, parts ...string) []string {
	var findings []string
	for _, finding := range readOnlyWALManualRequire(functions, name, diagnostic, parts...) {
		findings = append(findings, path+": rollback SQL contract: "+finding)
	}
	return findings
}
func rollbackFaultSQLBridgeSemantics(path string, raw []byte, set *token.FileSet, file *ast.File, functions map[string]*ast.FuncDecl) []string {
	var findings []string
	report := func(message string) { findings = append(findings, path+": rollback SQL contract: "+message) }
	require := func(name, diagnostic string, parts ...string) {
		findings = append(findings, rollbackFaultSQLRequire(path, functions, name, diagnostic, parts...)...)
	}
	wanted := map[string]bool{"ManualRollbackSQLState": true, "ArmManualRollbackSQLFault": true, "FinishManualRollbackSQLFault": true, "DisarmManualRollbackSQLFault": true, "manualRollbackSQLOnlyTargetCause": true}
	if len(functions) != len(wanted) {
		report("bridge exact five-function inventory required")
	}
	for name := range functions {
		if !wanted[name] {
			report("bridge extra helper/native entry")
		}
	}
	arm := readOnlyWALManualBody(functions, "ArmManualRollbackSQLFault")
	gate := `if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" { return nil,errRollbackFaultGate }`
	if arm == nil || len(arm.List) == 0 || readOnlyWALManualCompact(arm.List[0]) != readOnlyWALManualCompactText(gate) {
		report("bridge first action must be rejecting exact environment gate")
	}
	require("ArmManualRollbackSQLFault", "bridge frozen ten-case plan and healthy ownership", `switch spec.Action { case rollbackfault.Pass: caseID = rollbackfault.SQLControl; case rollbackfault.PrefixShortWrite: caseID = rollbackfault.SQLIndexPrefix64; if spec.PrefixError { caseID = rollbackfault.SQLIndexPrefix64Error }; case rollbackfault.MisplaceIndexPageMinus64: caseID = rollbackfault.SQLIndexMisplaceMinus64Error; case rollbackfault.FailBefore: switch spec.Target.Point { case rollbackfault.DBSync: caseID = rollbackfault.SQLDBSyncBefore; case rollbackfault.JournalBodySync: caseID = rollbackfault.SQLJournalBodySyncBefore; case rollbackfault.JournalHeaderSync: caseID = rollbackfault.SQLJournalHeaderSyncBefore; case rollbackfault.PublishDirectorySync: caseID = rollbackfault.SQLPublishDirectorySyncBefore; case rollbackfault.JournalUnlink: caseID = rollbackfault.SQLJournalUnlinkBefore; case rollbackfault.FinalDirectorySync: caseID = rollbackfault.SQLFinalDirectorySyncBefore; default: return nil,errRollbackFaultScope }; default: return nil,errRollbackFaultScope }`, `frozen, problem := rollbackfault.SQLPlan(spec.Identity,caseID)`, `frozen.Spec() != spec`, `state.Groups != 1 || state.Pending != 0 || !state.HealthyTx || state.AdapterActive || state.AdapterUsed`, `scope, err := armRollbackFault(tx,plan)`, `if err != nil { return nil,err }`, `return &manualRollbackSQLScope{scope:scope},nil`)
	require("ArmManualRollbackSQLFault", "exact actual three-page writer extent before arm", `if tx == nil { return nil,errRollbackFaultScope }`, `tx.originalPages == 3 && tx.header.PageCount == 3 && len(tx.dirty) == 3 && len(tx.dirty[1]) == 4096 && len(tx.dirty[2]) == 4096 && len(tx.dirty[3]) == 4096`, `if !threePages { return nil,errRollbackFaultScope }`)
	require("FinishManualRollbackSQLFault", "bridge exact untouched public error forwarding", `if scope == nil { return ManualRollbackSQLReport{Problem:errRollbackFaultScope} }`, `report := finishRollbackFault(scope.scope,commitErr)`, `return ManualRollbackSQLReport{Observation:report.Observation,Events:report.Events,EventCount:report.EventCount,Diagnostics:report.Diagnostics,CommitError:report.CommitError,Problem:report.Problem,PreDBOnlyTargetCause:preDBOnlyTargetCause,PostDBOnlyTargetCause:postDBOnlyTargetCause,MisplacedWrite:misplaced,MisplaceOnlyTargetCause:misplaceOnlyTargetCause}`)
	require("FinishManualRollbackSQLFault", "pre-DB cause bound to actual frozen target and Tx poison", `scope.scope.mu.Lock()`, `target,point := scope.scope.targetError,scope.scope.plan.Spec().Target.Point`, `scope.scope.mu.Unlock()`, `preDBOnlyTargetCause := false`, `if point == rollbackfault.JournalBodySync || point == rollbackfault.JournalHeaderSync || point == rollbackfault.PublishDirectorySync { scope.scope.tx.mu.Lock(); poison := scope.scope.tx.poisoned; scope.scope.tx.mu.Unlock(); preDBOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr,poison,target,nil) }`)
	require("manualRollbackSQLOnlyTargetCause", "closed bounded pre-DB cause traversal", `if commitErr == nil || poison == nil || target != errRollbackFaultInjected { return false }`, `if _,custom := commitErr.(interface{ Is(error) bool }); custom { return false }`, `if _,custom := commitErr.(interface{ As(any) bool }); custom { return false }`, `public,ok := commitErr.(interface{ Unwrap() error })`, `if !ok || public.Unwrap() != poison { return false }`, `joinType := reflect.TypeOf(errors.Join(ErrPoisoned,target))`, `var pending [rollbackfault.MaxEvents]error`, `pending[0] = poison`, `count,visited := 1,0`, `sawTarget,sawPoisoned := false,false`, `sawPermitted := permitted == nil`, `if permitted != nil && permitted != ErrRecoveryRequired && permitted != ErrCommitUncertain { return false }`, `for count != 0 { if visited == len(pending) { return false }; visited++; count--; current := pending[count]; pending[count] = nil; if _,custom := current.(interface{ Is(error) bool }); custom { return false }; if _,custom := current.(interface{ As(any) bool }); custom { return false }; if current == target { sawTarget = true; continue }; if current == ErrPoisoned { sawPoisoned = true; continue }; if permitted != nil && current == permitted { sawPermitted = true; continue }; if reflect.TypeOf(current) != joinType { return false }; children := current.(interface{ Unwrap() []error }).Unwrap(); if len(children) == 0 || len(children) > len(pending)-count { return false }; for _,child := range children { if child == nil { return false }; pending[count] = child; count++ } }`, `return sawTarget && sawPoisoned && sawPermitted`)
	cause := functions["manualRollbackSQLOnlyTargetCause"]
	if cause != nil {
		// Doc is commentary, not part of the closed signature/body contract.
		// Keep the parsed source AST intact for all other guard checks.
		shape := *cause
		shape.Doc = nil
		cause = &shape
	}
	if cause == nil || readOnlyWALManualCompact(cause) != readOnlyWALManualCompactText(`func manualRollbackSQLOnlyTargetCause(commitErr, poison, target, permitted error) bool {
	if commitErr == nil || poison == nil || target != errRollbackFaultInjected {
		return false
	}
	if permitted != nil && permitted != ErrRecoveryRequired && permitted != ErrCommitUncertain {
		return false
	}
	if _, custom := commitErr.(interface{ Is(error) bool }); custom {
		return false
	}
	if _, custom := commitErr.(interface{ As(any) bool }); custom {
		return false
	}
	public, ok := commitErr.(interface{ Unwrap() error })
	if !ok || public.Unwrap() != poison {
		return false
	}
	joinType := reflect.TypeOf(errors.Join(ErrPoisoned, target))
	var pending [rollbackfault.MaxEvents]error
	pending[0] = poison
	count, visited := 1, 0
	sawTarget, sawPoisoned := false, false
	sawPermitted := permitted == nil
	for count != 0 {
		if visited == len(pending) {
			return false
		}
		visited++
		count--
		current := pending[count]
		pending[count] = nil
		if _, custom := current.(interface{ Is(error) bool }); custom {
			return false
		}
		if _, custom := current.(interface{ As(any) bool }); custom {
			return false
		}
		if current == target {
			sawTarget = true
			continue
		}
		if current == ErrPoisoned {
			sawPoisoned = true
			continue
		}
		if permitted != nil && current == permitted {
			sawPermitted = true
			continue
		}
		if reflect.TypeOf(current) != joinType {
			return false
		}
		children := current.(interface{ Unwrap() []error }).Unwrap()
		if len(children) == 0 || len(children) > len(pending)-count {
			return false
		}
		for _, child := range children {
			if child == nil {
				return false
			}
			pending[count] = child
			count++
		}
	}
	return sawTarget && sawPoisoned && sawPermitted
}`) {
		report("closed bounded pre-DB cause traversal cannot add shapes, effects or change rejection order")
	}
	if readOnlyWALManualCountCall(file, "manualRollbackSQLOnlyTargetCause") != 3 || readOnlyWALManualCountCall(file, "reflect.TypeOf") != 2 || readOnlyWALManualCountCall(file, "errors.Join") != 1 {
		report("pre/post-DB and misplacement cause observer has three exact calls in Finish and closed type observations")
	}
	require("FinishManualRollbackSQLFault", "post-DB actual target poison and case-specific sentinel", `postDBOnlyTargetCause := false`, `if point == rollbackfault.JournalUnlink || point == rollbackfault.FinalDirectorySync { scope.scope.tx.mu.Lock(); poison := scope.scope.tx.poisoned; scope.scope.tx.mu.Unlock(); permitted := ErrRecoveryRequired; if point == rollbackfault.FinalDirectorySync { permitted = ErrCommitUncertain }; postDBOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr,poison,target,permitted) }`)
	findings = append(findings, rollbackFaultSQLExactFunction(path, functions, "FinishManualRollbackSQLFault", "post-DB Finish cannot bypass target, poison or sentinel binding", `func FinishManualRollbackSQLFault(scope *manualRollbackSQLScope, commitErr error) ManualRollbackSQLReport {
	if scope == nil {
		return ManualRollbackSQLReport{Problem: errRollbackFaultScope}
	}
	report := finishRollbackFault(scope.scope, commitErr)
	scope.scope.mu.Lock()
	target, point := scope.scope.targetError, scope.scope.plan.Spec().Target.Point
	action, misplaced := scope.scope.plan.Spec().Action, scope.scope.misplacedWrite
	scope.scope.mu.Unlock()
	preDBOnlyTargetCause := false
	if point == rollbackfault.JournalBodySync || point == rollbackfault.JournalHeaderSync || point == rollbackfault.PublishDirectorySync {
		scope.scope.tx.mu.Lock()
		poison := scope.scope.tx.poisoned
		scope.scope.tx.mu.Unlock()
		preDBOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr, poison, target, nil)
	}
	postDBOnlyTargetCause := false
	if point == rollbackfault.JournalUnlink || point == rollbackfault.FinalDirectorySync {
		scope.scope.tx.mu.Lock()
		poison := scope.scope.tx.poisoned
		scope.scope.tx.mu.Unlock()
		permitted := ErrRecoveryRequired
		if point == rollbackfault.FinalDirectorySync {
			permitted = ErrCommitUncertain
		}
		postDBOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr, poison, target, permitted)
	}
	misplaceOnlyTargetCause := false
	if action == rollbackfault.MisplaceIndexPageMinus64 {
		scope.scope.tx.mu.Lock()
		poison := scope.scope.tx.poisoned
		scope.scope.tx.mu.Unlock()
		misplaceOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr, poison, target, ErrRecoveryRequired)
	}
	return ManualRollbackSQLReport{Observation: report.Observation, Events: report.Events, EventCount: report.EventCount, Diagnostics: report.Diagnostics, CommitError: report.CommitError, Problem: report.Problem, PreDBOnlyTargetCause: preDBOnlyTargetCause, PostDBOnlyTargetCause: postDBOnlyTargetCause, MisplacedWrite: misplaced, MisplaceOnlyTargetCause: misplaceOnlyTargetCause}
}`)...)
	require("DisarmManualRollbackSQLFault", "bridge no pending owner/reset/retry", `if scope == nil { return errRollbackFaultScope }`, `state := ManualRollbackSQLState(nil)`, `if state.Groups != 0 || state.Pending != 0 { return errRollbackFaultScope }`, `return disarmRollbackFault(scope.scope)`)
	require("ManualRollbackSQLState", "read-only scalar ownership snapshot", `plainRetirements.mu.Lock()`, `out.Groups = len(plainRetirements.groups)`, `if g.pending.Load() != nil { out.Pending++ }`, `plainRetirements.mu.Unlock()`, `rollbackFaultRegistry.mu.Lock()`, `out.AdapterActive = rollbackFaultRegistry.active != nil`, `out.AdapterUsed = rollbackFaultRegistry.used`, `rollbackFaultRegistry.mu.Unlock()`, `main, concrete := g.main.(plainFile)`, `concrete && main.file == tx.file && tx.file != nil && tx.session == nil && tx.shared == nil && tx.encrypted == nil && !tx.keepFile && tx.borrowedCleanup == nil && !g.started && !g.done && g.cause == nil && g.pending.Load() == nil && g.temporary == nil && !g.temporaryReserved && !g.temporaryRequested && g.wal == nil && !g.walReserved && !g.walRequested && len(g.nested) == 0`, `return out`)
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			typ, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if typ.Assign.IsValid() {
				report("bridge cannot alias ownership types")
			}
			if typ.Name.Name != "manualRollbackSQLScope" && typ.Name.Name != "ManualRollbackSQLReport" && typ.Name.Name != "ManualRollbackSQLSnapshot" {
				report("bridge cannot add owner/cleanup/file exposure types")
			}
			if typ.Name.Name == "ManualRollbackSQLReport" && readOnlyWALManualCompact(typ) != readOnlyWALManualCompactText(`ManualRollbackSQLReport struct { Observation rollbackfault.Observation; Events [rollbackfault.MaxEvents]rollbackfault.Event; EventCount uint8; Diagnostics [rollbackfault.MaxEvents]error; CommitError error; Problem error; PreDBOnlyTargetCause bool; PostDBOnlyTargetCause bool; MisplacedWrite rollbackfault.SQLMisplacedWrite; MisplaceOnlyTargetCause bool }`) {
				report("bridge report cannot expose native owner/cleanup/file pointers")
			}

			if typ.Name.Name == "ManualRollbackSQLSnapshot" && readOnlyWALManualCompact(typ) != readOnlyWALManualCompactText(`ManualRollbackSQLSnapshot struct { Groups int; Pending int; HealthyTx bool; AdapterActive bool; AdapterUsed bool }`) {
				report("snapshot must expose exactly five read-only scalars")
			}
			if typ.Name.Name == "manualRollbackSQLScope" && readOnlyWALManualCompact(typ) != readOnlyWALManualCompactText(`manualRollbackSQLScope struct { scope *rollbackFaultScope }`) {
				report("scope may retain only existing private scope")
			}
		}
	}
	state := functions["ManualRollbackSQLState"]
	if state != nil && state.Body != nil {
		ast.Inspect(state.Body, func(node ast.Node) bool {
			if assignment, ok := node.(*ast.AssignStmt); ok {
				for _, left := range assignment.Lhs {
					name := readOnlyWALManualName(left)
					if strings.Contains(name, ".") && !strings.HasPrefix(name, "out.") {
						report("snapshot cannot mutate native owner/registry")
					}
				}
			}
			return true
		})
	}
	return findings
}

func rollbackFaultSQLNativeSemantics(path string, raw []byte, set *token.FileSet, file *ast.File, functions map[string]*ast.FuncDecl) []string {
	var findings []string
	report := func(message string) { findings = append(findings, path+": rollback SQL contract: "+message) }
	require := func(name, diagnostic string, parts ...string) {
		findings = append(findings, rollbackFaultSQLRequire(path, functions, name, diagnostic, parts...)...)
	}
	findings = append(findings, rollbackFaultSQLExactFunction(path, functions, "manualRollbackSQLMisplacement", "fixed misplaced-write payload, tail and journal originals", `func manualRollbackSQLMisplacement(report pager.ManualRollbackSQLReport, baseline, post, journalBytes []byte) (rollbackfault.SQLMisplacementEvidence, error) {
	evidence := rollbackfault.SQLMisplacementEvidence{Writer: report.MisplacedWrite, OnlyTargetCause: report.MisplaceOnlyTargetCause}
	if len(baseline) != 12288 || len(post) != 12288 || len(journalBytes) != 16408 {
		return evidence, fmt.Errorf("misplaced-write exact retained extents required")
	}
	evidence.ShiftedSHA256 = sha256.Sum256(post[8128:12224])
	evidence.BaselineRangeSHA256 = sha256.Sum256(baseline[8128:12224])
	evidence.BaselineTailSHA256 = sha256.Sum256(baseline[12224:12288])
	evidence.UntouchedTailSHA256 = sha256.Sum256(post[12224:12288])
	writer := evidence.Writer
	if writer.RequestedOffset != 8192 || writer.RequestedBytes != 4096 || writer.NativeOffset != 8128 || writer.NativeBytes != 4096 || writer.NativeCalls != 1 || writer.PayloadSHA256 == ([32]byte{}) || writer.PayloadSHA256 != evidence.ShiftedSHA256 || evidence.ShiftedSHA256 == evidence.BaselineRangeSHA256 || !bytes.Equal(post[12224:12288], baseline[12224:12288]) || !evidence.OnlyTargetCause {
		return evidence, fmt.Errorf("misplaced-write actual argument/payload/tail/cause proof differs")
	}
	if report.EventCount != 35 || report.Events[30].Kind != rollbackfault.Returned || report.Events[30].Action != rollbackfault.MisplaceIndexPageMinus64 || report.Events[30].Call != (rollbackfault.Call{Point: rollbackfault.DBPage, Occurrence: 3, Role: rollbackfault.Database, Offset: 8192, Length: 4096, Owner: 11}) || report.Events[30].Result != (rollbackfault.Result{Executed: true, NativeN: 4096, ReturnedN: 4096, ReturnedError: rollbackfault.Injected, NormalizedError: rollbackfault.Injected, Effect: rollbackfault.EffectPossible}) {
		return evidence, fmt.Errorf("misplaced-write original requested call and native return differ")
	}
	plan, err := journal.ParseReplay(journalBytes, journal.ReplayLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288})
	if err != nil {
		return evidence, err
	}
	if plan.CountEvidence != journal.ReplayCountFiniteHeaders || plan.Header.Records != 3 || plan.Header.OriginalPages != 3 || plan.Header.PageSize != 4096 || plan.Header.SectorSize != 4096 || plan.DatabaseBytes != 12288 || len(plan.Pages) != 3 {
		return evidence, fmt.Errorf("misplaced-write complete finite journal inventory required")
	}
	if err := plan.ValidateExtent(12288); err != nil {
		return evidence, err
	}
	for i, page := range plan.Pages {
		if page.Number != uint32(i+1) || len(page.Data) != 4096 || !bytes.Equal(page.Data, baseline[i*4096:(i+1)*4096]) {
			return evidence, fmt.Errorf("misplaced-write journal original differs from independent baseline")
		}
		evidence.JournalOriginals[i] = sha256.Sum256(page.Data)
	}
	return evidence, nil
}`)...)
	root := readOnlyWALManualBody(functions, rollbackFaultSQLRoot)
	if root == nil || len(root.List) == 0 || readOnlyWALManualCompact(root.List[0]) != readOnlyWALManualCompactText(`if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" { t.Fatal("NOT_RUN: separate execution authorization, reviewed provenance and private retained root required") }`) {
		report("root first action must be exact rejecting environment gate")
	}
	commit := readOnlyWALManualBody(functions, "manualRollbackSQLCommit")
	if readOnlyWALManualSequence(commit, `commitErr := tx.Commit(ctx)`, `report := pager.FinishManualRollbackSQLFault(scope,commitErr)`) < 0 {
		report("public Commit must be immediately followed by Finish with untouched error")
	}
	require("manualRollbackSQLCommit", "public error identity, owners and classification", `if !manualRollbackSQLRegistryEmpty()`, `db, owner, err := manualRollbackSQLOpen(c,path,false)`, `tx, beginErr := db.Begin(ctx,TxOptions{Immediate:true})`, `"UPDATE t SET k='green',v='ONE' WHERE id=1"`, `"DELETE FROM t WHERE id=2"`, `"INSERT INTO t(id,k,v) VALUES(4,'blue','four')"`, `countErr != nil || count != 1`, `scope, armErr := pager.ArmManualRollbackSQLFault(tx.storage,plan)`, `report.CommitError != commitErr || report.Problem != nil || report.Observation.Outcome != rollbackfault.AdapterObserved || report.Observation.TargetHits != 1 || report.Observation.Pass() || report.Observation.PhysicalValidated() || !tx.done || db.tx != nil || tx.activeRows != 0`, `!errors.As(commitErr,&publicError) || publicError.Code != CodeIOErr || !errors.Is(commitErr,pager.ErrPoisoned) || errors.Is(commitErr,pager.ErrPlaintextCleanupPending) || errors.Is(commitErr,pager.ErrCommitUncertain) != uncertain`, `c.caseID == rollbackfault.SQLIndexPrefix64 && !errors.Is(commitErr,io.ErrShortWrite)`, `c.caseID == rollbackfault.SQLIndexPrefix64Error && errors.Is(commitErr,io.ErrShortWrite)`, `if err := pager.DisarmManualRollbackSQLFault(scope); err != nil { return report,err }`, `if err := c.ledger.close(owner); err != nil { return report,err }`, `if !manualRollbackSQLRegistryEmpty()`)
	require("manualRollbackSQLCommit", "exact case-bound public return modes", `spec := plan.Spec()`, `frozen,problem := rollbackfault.SQLPlan(spec.Identity,c.caseID)`, `if problem != rollbackfault.NoProblem || frozen.Spec() != spec { return pager.ManualRollbackSQLReport{},fmt.Errorf("public Commit case/plan mismatch") }`, `if c.caseID == rollbackfault.SQLControl { if commitErr != nil { return report,fmt.Errorf("control public Commit: %w",commitErr) } } else`, `if c.caseID == rollbackfault.SQLIndexPrefix64 && !errors.Is(commitErr,io.ErrShortWrite) { return report,fmt.Errorf("original short-write cause lost") }`, `if c.caseID == rollbackfault.SQLIndexPrefix64Error && errors.Is(commitErr,io.ErrShortWrite) { return report,fmt.Errorf("injected prefix error confused with nil-error short write") }`)
	require("manualRollbackSQLCommit", "public error identity and pre-DB exact cause", `preDB := c.caseID == rollbackfault.SQLJournalBodySyncBefore || c.caseID == rollbackfault.SQLJournalHeaderSyncBefore || c.caseID == rollbackfault.SQLPublishDirectorySyncBefore`, `if preDB { if commitErr != publicError || !report.PreDBOnlyTargetCause || errors.Is(commitErr,ErrRecoveryRequired) || errors.Is(commitErr,io.ErrShortWrite) { return report,fmt.Errorf("INCOMPLETE: pre-DB public cause or cleanup differs: %v",commitErr) } } else if postDB { if commitErr != publicError || !report.PostDBOnlyTargetCause || errors.Is(commitErr,ErrRecoveryRequired) != !uncertain || errors.Is(commitErr,io.ErrShortWrite) { return report,fmt.Errorf("INCOMPLETE: post-DB public cause or cleanup differs: %v",commitErr) } } else if !errors.Is(commitErr,ErrRecoveryRequired) { return report,fmt.Errorf("unqualified public recovery cause: %v",commitErr) }`)
	findings = append(findings, rollbackFaultSQLExactFunction(path, functions, "manualRollbackSQLCommit", "post-DB public Commit exact classification and retirement cannot be bypassed", `func manualRollbackSQLCommit(c *manualRollbackSQLConfig, ctx context.Context, path string, plan rollbackfault.Plan) (pager.ManualRollbackSQLReport, error) {
	spec := plan.Spec()
	preDB := c.caseID == rollbackfault.SQLJournalBodySyncBefore || c.caseID == rollbackfault.SQLJournalHeaderSyncBefore || c.caseID == rollbackfault.SQLPublishDirectorySyncBefore
	postDB := c.caseID == rollbackfault.SQLJournalUnlinkBefore || c.caseID == rollbackfault.SQLFinalDirectorySyncBefore
	frozen, problem := rollbackfault.SQLPlan(spec.Identity, c.caseID)
	if problem != rollbackfault.NoProblem || frozen.Spec() != spec {
		return pager.ManualRollbackSQLReport{}, fmt.Errorf("public Commit case/plan mismatch")
	}
	if !manualRollbackSQLRegistryEmpty() {
		return pager.ManualRollbackSQLReport{}, fmt.Errorf("preexisting owner obligations")
	}
	db, owner, err := manualRollbackSQLOpen(c, path, false)
	if err != nil {
		return pager.ManualRollbackSQLReport{}, err
	}
	tx, beginErr := db.Begin(ctx, TxOptions{Immediate: true})
	if beginErr != nil {
		return pager.ManualRollbackSQLReport{}, errors.Join(manualRollbackSQLAbandon(tx, beginErr), c.ledger.close(owner))
	}
	for _, statement := range []string{
		"UPDATE t SET k='green',v='ONE' WHERE id=1",
		"DELETE FROM t WHERE id=2",
		"INSERT INTO t(id,k,v) VALUES(4,'blue','four')",
	} {
		result, execErr := tx.Exec(ctx, statement)
		if execErr != nil {
			return pager.ManualRollbackSQLReport{}, errors.Join(manualRollbackSQLAbandon(tx, execErr), c.ledger.close(owner))
		}
		count, countErr := result.RowsAffected()
		if countErr != nil || count != 1 {
			return pager.ManualRollbackSQLReport{}, errors.Join(manualRollbackSQLAbandon(tx, fmt.Errorf("DML affected %d: %v", count, countErr)), c.ledger.close(owner))
		}
	}
	scope, armErr := pager.ArmManualRollbackSQLFault(tx.storage, plan)
	if armErr != nil {
		return pager.ManualRollbackSQLReport{}, errors.Join(manualRollbackSQLAbandon(tx, armErr), c.ledger.close(owner))
	}
	commitErr := tx.Commit(ctx)
	report := pager.FinishManualRollbackSQLFault(scope, commitErr)
	// Keep the original public error object. No storage.Commit bypass, second
	// Commit, Rollback, DB.Close or recovery may repair an incomplete result.
	if report.CommitError != commitErr || report.Problem != nil || report.Observation.Outcome != rollbackfault.AdapterObserved || report.Observation.TargetHits != 1 || report.Observation.Pass() || report.Observation.PhysicalValidated() || !tx.done || db.tx != nil || tx.activeRows != 0 {
		return report, fmt.Errorf("INCOMPLETE: public Commit/adapter closure: %v; %v", commitErr, report.Problem)
	}
	if c.caseID == rollbackfault.SQLControl {
		if commitErr != nil {
			return report, fmt.Errorf("control public Commit: %w", commitErr)
		}
	} else {
		var publicError *Error
		uncertain := c.caseID == rollbackfault.SQLFinalDirectorySyncBefore
		if !errors.As(commitErr, &publicError) || publicError.Code != CodeIOErr || !errors.Is(commitErr, pager.ErrPoisoned) || errors.Is(commitErr, pager.ErrPlaintextCleanupPending) || errors.Is(commitErr, pager.ErrCommitUncertain) != uncertain {
			return report, fmt.Errorf("unqualified public Commit error: %v", commitErr)
		}
		if preDB {
			// Exact public wrapper identity plus the bridge's actual Tx poison
			// binding excludes extra consumed close errors and unknown chains.
			if commitErr != publicError || !report.PreDBOnlyTargetCause || errors.Is(commitErr, ErrRecoveryRequired) || errors.Is(commitErr, io.ErrShortWrite) {
				return report, fmt.Errorf("INCOMPLETE: pre-DB public cause or cleanup differs: %v", commitErr)
			}
		} else if postDB {
			if commitErr != publicError || !report.PostDBOnlyTargetCause || errors.Is(commitErr, ErrRecoveryRequired) != !uncertain || errors.Is(commitErr, io.ErrShortWrite) {
				return report, fmt.Errorf("INCOMPLETE: post-DB public cause or cleanup differs: %v", commitErr)
			}
		} else if !errors.Is(commitErr, ErrRecoveryRequired) {
			return report, fmt.Errorf("unqualified public recovery cause: %v", commitErr)
		}
		if c.caseID == rollbackfault.SQLIndexPrefix64 && !errors.Is(commitErr, io.ErrShortWrite) {
			return report, fmt.Errorf("original short-write cause lost")
		}
		if c.caseID == rollbackfault.SQLIndexMisplaceMinus64Error && (commitErr != publicError || !report.MisplaceOnlyTargetCause || errors.Is(commitErr, io.ErrShortWrite)) {
			return report, fmt.Errorf("INCOMPLETE: misplaced-write public cause or cleanup differs: %v", commitErr)
		}
		// Finish has checked errors.Is against the actual private target error.
		// Native (64, nil) becomes adapter (64, injected), never ErrShortWrite.
		if c.caseID == rollbackfault.SQLIndexPrefix64Error && errors.Is(commitErr, io.ErrShortWrite) {
			return report, fmt.Errorf("injected prefix error confused with nil-error short write")
		}
	}
	if err := pager.DisarmManualRollbackSQLFault(scope); err != nil {
		return report, err
	}
	if err := c.ledger.close(owner); err != nil {
		return report, err
	}
	if !manualRollbackSQLRegistryEmpty() {
		return report, fmt.Errorf("Commit owner retirement incomplete")
	}
	return report, nil
}`)...)
	require("manualRollbackSQLAbandon", "healthy pre-arm abandonment only", `if tx == nil || tx.done { return cause }`, `state := pager.ManualRollbackSQLState(tx.storage)`, `if state.AdapterActive || state.AdapterUsed || state.Pending != 0 || !state.HealthyTx { return cause }`, `return errors.Join(cause,tx.Rollback())`)
	require("close", "owner close-once without hidden rollback", `if o.closes != 0`, `if o.file == nil && o.db == nil && o.rows == nil { return nil }`, `if o.db != nil && (o.db.tx != nil || len(o.db.rows) != 0)`, `o.closes++`, `o.closeErr = o.file.Close()`, `o.closeErr = o.rows.Close()`, `o.closeErr = o.db.Close()`, `return o.closeErr`)
	require("retired", "owner retirement includes every acquired owner", `(o.file != nil || o.db != nil || o.rows != nil) && (o.closes != 1 || o.closeErr != nil)`, `return false`)
	require("finish", "deferred reporting cannot retry owners", `(o.file != nil || o.db != nil || o.rows != nil) && (o.closes != 1 || o.closeErr != nil)`, `t.Errorf`)
	if readOnlyWALManualCountCall(readOnlyWALManualBody(functions, "finish"), "l.close") != 0 {
		report("deferred owner report cannot perform cleanup retry")
	}
	require("manualRollbackSQLRecoverCopy", "fresh replay copy exact bounded recovery", `if filepath.Base(replayPath) != "replay.db" || !manualRollbackSQLRegistryEmpty()`, rollbackFaultSQLRecoveryCall, `if recoveryErr != nil { return result,recoveryErr }`, `if !manualRollbackSQLRegistryEmpty()`, `!result.Recovered || result.RestoredPages != 3 || result.OriginalPages != 3`)
	require("manualRollbackSQLBaseline", "fixed independent old three-page table/index baseline", `var image [12288]byte`, `binary.BigEndian.PutUint16(image[16:],4096)`, `binary.BigEndian.PutUint32(image[28:],3)`, `{"table","t","t",int64(2),"CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT,v TEXT)"}`, `{"index","by_k","t",int64(3),"CREATE INDEX by_k ON t(k)"}`, `table := [][]any{{nil,"red","one"},{nil,"blue","two"},{nil,"red","three"}}`, `index := [][]any{{"blue",int64(2)},{"red",int64(1)},{"red",int64(3)}}`, `manualRollbackSQLLeaf(image[:4096],100,13,schema,[]byte{1,2})`, `manualRollbackSQLLeaf(image[4096:8192],0,13,table,[]byte{1,2,3})`, `manualRollbackSQLLeaf(image[8192:],0,10,index,nil)`)
	require("manualRollbackSQLOpen", "bounded independent no-create plain open", `Open(path,Options{ReadOnly:readonly,NoCreate:true,PageSize:4096,SectorSize:4096,MaxTransactionPages:3,MaxTransactionUndoBytes:65536,MaxSortBytes:65536})`, `c.ledger.owners[i] = manualRollbackSQLOwner{db:db,path:path}`, `db == nil || db.session != nil`)
	require("manualRollbackSQLQuery", "real typed SQL rows and index work", `rows, queryErr := db.Query(ctx,evidence.SQL)`, `c.ledger.owners[i] = manualRollbackSQLOwner{rows:rows,path:evidence.SQL}`, `evidence.Iterator = uint64(i+1000)`, `for rows.Next()`, `values[0].(int64)`, `values[1].(string)`, `values[2].(string)`, `if !idOK || !keyOK || !valueOK`, `evidence.RowCount++`, `rowsErr := rows.Err()`, `evidence.ErrCalled = true`, `evidence.EOF = rowsErr == nil`, `stats := rows.Stats()`, `closeErr := c.ledger.close(i)`, `errors.Join(rowsErr,closeErr)`)
	require("manualRollbackSQLReadback", "fresh read-only checked snapshot", `db, owner, err := manualRollbackSQLOpen(c,path,true)`, `for i := 0; i < rollbackfault.SQLQueryCount; i++`, `manualRollbackSQLQuery(c,ctx,db,evidence.Owner,identity,rollbackfault.SQLQuery(i))`, `db.Check(ctx,CheckOptions{MaxPages:3,MaxPageReads:512})`, `errors.Join(checkErr,closeErr)`, `checked.Scope != CheckScope || checked.PageReads == 0 || checked.PageReads > 512 || !manualRollbackSQLRegistryEmpty()`, `evidence.OwnerState = rollbackfault.OwnerRetired`)
	require("manualRollbackSQLAdmit", "one exact fresh-process selection and reviewed provenance", `"-test.run":"^TestManualRollbackFaultSQLLedger$"`, `"-test.count":"1"`, `"-test.shuffle":"off"`, `"-test.parallel":"1"`, `"-test.timeout":"0"`, `"-test.cpu":"1"`, `"-test.v":"true"`, `if !found || seen[key]`, `if expected, ok := want[key]; !ok || value != expected`, `if !seen[key]`, `case "control":`, `case "index-prefix-64":`, `case "db-sync-before":`, `case "index-prefix-64-error":`, `default:`, `receipt.Schema != "rollback-fault-sql-compile-v1"`, `receipt.Package != "./sqlite" || receipt.Tags != "manual_fault_rollback" || !receipt.CompileOnly`, `sha256.Sum256(image) != binaryHash`)
	require("manualRollbackSQLAdmit", "ten exact names and case mappings", `switch c.binding.Case { case "control": c.caseID = rollbackfault.SQLControl; case "index-prefix-64": c.caseID = rollbackfault.SQLIndexPrefix64; case "db-sync-before": c.caseID = rollbackfault.SQLDBSyncBefore; case "index-prefix-64-error": c.caseID = rollbackfault.SQLIndexPrefix64Error; case "jbody-sync-before": c.caseID = rollbackfault.SQLJournalBodySyncBefore; case "jheader-sync-before": c.caseID = rollbackfault.SQLJournalHeaderSyncBefore; case "publish-dirsync-before": c.caseID = rollbackfault.SQLPublishDirectorySyncBefore; case "junlink-before": c.caseID = rollbackfault.SQLJournalUnlinkBefore; case "final-dirsync-before": c.caseID = rollbackfault.SQLFinalDirectorySyncBefore; case "index-misplace-minus64-error": c.caseID = rollbackfault.SQLIndexMisplaceMinus64Error; default: return fmt.Errorf("exactly one of ten fixed cases required") }`)
	require(rollbackFaultSQLRoot, "root independent baseline and copy-only recovery order", `rollbackfault.ValidateSQLSnapshot(before,identity,rollbackfault.SQLOld)`, `manualRollbackSQLCommit(&c,ctx,filepath.Join(c.root,"trial.db"),plan)`, `receipt.Seal() != report.Observation`, `if (c.caseID == rollbackfault.SQLIndexPrefix64 || c.caseID == rollbackfault.SQLIndexPrefix64Error) && !bytes.Equal(post[8192+64:],baseline[8192+64:])`, `c.retain("trial.db",trialInfo,12288)`, `c.create("post-commit.input.db",post,true)`, `c.retain("trial.db-journal",nil,16408)`, `c.create("post-commit-journal.input.bin",journalBytes,true)`, `c.create("replay.db",post,false)`, `os.SameFile(trialInfo,replayInfo)`, `manualRollbackSQLCapture(&c,&evidence,false)`, `manualRollbackSQLRecoverCopy(ctx,filepath.Join(c.root,"replay.db"))`, `evidence.Recovery.Calls++`, `manualRollbackSQLReadback(&c,ctx,filepath.Join(c.root,"replay.db"),identity)`, `want := rollbackfault.SQLOld`, `if c.caseID == rollbackfault.SQLControl || c.caseID == rollbackfault.SQLFinalDirectorySyncBefore { want = rollbackfault.SQLNew }`, `rollbackfault.ValidateSQLSnapshot(after,identity,want)`, `manualRollbackSQLCapture(&c,&evidence,true)`, `evidence.Originals.Frozen = frozenHashes`, `manualRollbackSQLPageHashes(oldBefore)`, `manualRollbackSQLPageHashes(oldAfter)`, `!c.ledger.retired() || !manualRollbackSQLRegistryEmpty()`, `rollbackfault.ObserveSQLLedger(plan,receipt,evidence)`, `!observation.EvidenceConsistent || !observation.ScenarioObserved || observation.Problem != rollbackfault.SQLProblemNone || observation.PhysicalValidated() || observation.Pass()`)
	require(rollbackFaultSQLRoot, "pre-DB whole OLD bytes and no journal or recovery", `preDB := c.caseID == rollbackfault.SQLJournalBodySyncBefore || c.caseID == rollbackfault.SQLJournalHeaderSyncBefore || c.caseID == rollbackfault.SQLPublishDirectorySyncBefore`, `needsRecovery := c.caseID == rollbackfault.SQLIndexPrefix64 || c.caseID == rollbackfault.SQLDBSyncBefore || c.caseID == rollbackfault.SQLIndexPrefix64Error || c.caseID == rollbackfault.SQLJournalUnlinkBefore || c.caseID == rollbackfault.SQLIndexMisplaceMinus64Error`, `if preDB && !bytes.Equal(post,baseline[:]) { t.Fatal("INCOMPLETE: pre-DB failure changed original database bytes") }`, `if !needsRecovery { if _,err := os.Lstat(filepath.Join(c.root,"trial.db-journal")); !os.IsNotExist(err) { t.Fatalf("zero-recovery case journal must be absent after Commit: %v",err) } } else`, `if needsRecovery { if _,err := c.create("replay.db-journal",journalBytes,false); err != nil { t.Fatal(err) } }`, `evidence.Recovery = rollbackfault.SQLRecoveryEvidence{Identity:identity,OwnersRetired:manualRollbackSQLRegistryEmpty()}`, `if needsRecovery { result,recoveryErr := manualRollbackSQLRecoverCopy(ctx,filepath.Join(c.root,"replay.db")); evidence.Recovery.Calls++`, `if _,err := os.Lstat(filepath.Join(c.root,"replay.db-journal")); !os.IsNotExist(err) { t.Fatalf("replay journal not absent after the fixed recovery decision: %v",err) }`)
	require(rollbackFaultSQLRoot, "actual public lifecycle observations", `evidence.Lifecycle.PublicCodeIOErr = errors.As(report.CommitError,&publicError) && publicError.Code == CodeIOErr`, `evidence.Lifecycle.PublicRecoveryRequired = errors.Is(report.CommitError,ErrRecoveryRequired)`, `evidence.Lifecycle.PublicShortWrite = errors.Is(report.CommitError,io.ErrShortWrite)`, `evidence.Lifecycle.PublicPoisoned = errors.Is(report.CommitError,pager.ErrPoisoned)`, `evidence.Lifecycle.PublicCommitUncertain = errors.Is(report.CommitError,pager.ErrCommitUncertain)`, `evidence.Lifecycle.PublicCleanupPending = errors.Is(report.CommitError,pager.ErrPlaintextCleanupPending)`, `evidence.Lifecycle.PreDBOnlyTargetCause = report.PreDBOnlyTargetCause`, `PreDBOnlyTargetCause bool`, `evidence.Lifecycle.PostDBOnlyTargetCause = report.PostDBOnlyTargetCause`, `PostDBOnlyTargetCause bool`, `report.PreDBOnlyTargetCause,report.PostDBOnlyTargetCause,misplacement,misplacementText},"","  ")`)
	if readOnlyWALManualCountCall(root, "manualRollbackSQLReadback") != 3 {
		report("exact three native readback sites for baseline, retained NEW copy and final ledger required")
	}
	recoveryCallCount := readOnlyWALManualCountCall(root, "manualRollbackSQLRecoverCopy")
	if recoveryCallCount != 1 {
		report("root must have exactly one copy-only recovery call")
	}
	baselineAt := readOnlyWALManualSequence(root, `baseline,err := manualRollbackSQLBaseline()`)
	beforeAt := readOnlyWALManualSequence(root, `before,err := manualRollbackSQLReadback(&c,ctx,filepath.Join(c.root,"baseline.input.db"),identity)`)
	validationAt := readOnlyWALManualSequence(root, `if problem := rollbackfault.ValidateSQLSnapshot(before,identity,rollbackfault.SQLOld); problem != rollbackfault.SQLProblemNone { t.Fatalf("baseline independent ledger rejected before fault: %v",problem) }`)
	commitAt := readOnlyWALManualSequence(root, `report,commitCheckErr := manualRollbackSQLCommit(&c,ctx,filepath.Join(c.root,"trial.db"),plan)`)
	copyAt := readOnlyWALManualSequence(root, `replayInfo,err := c.create("replay.db",post,false)`)
	afterAt := readOnlyWALManualSequence(root, `after,err := manualRollbackSQLReadback(&c,ctx,filepath.Join(c.root,"replay.db"),identity)`)
	if baselineAt < 0 || beforeAt <= baselineAt || validationAt <= beforeAt || commitAt <= validationAt || copyAt <= commitAt || afterAt <= copyAt {
		report("root must build and validate baseline once before one Commit and independent replay readback")
	}
	if readOnlyWALManualCountCall(file, "manualRollbackSQLCommit") != 1 || readOnlyWALManualCountCall(file, "manualRollbackSQLBaseline") != 1 {
		report("one root Commit helper call and one independent baseline build required")
	}
	parents := oracleParents(file)
	ast.Inspect(file, func(node ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok && (id.Name == "manualRollbackSQLCommit" || id.Name == "manualRollbackSQLBaseline") {
			if fn, declaration := parents[id].(*ast.FuncDecl); declaration && fn.Name == id {
				return true
			}
			call := rollbackFaultDirectReference(id, parents)
			if call == nil {
				report("one root Commit/baseline helper cannot be aliased or retained")
				return true
			}
			assignment, ok := parents[call].(*ast.AssignStmt)
			if !ok || parents[assignment] != root {
				report("one root Commit/baseline helper must be a direct top-level assignment")
			}
		}
		call, ok := node.(*ast.CallExpr)
		if !ok || readOnlyWALManualName(call.Fun) != "manualRollbackSQLRecoverCopy" {
			return true
		}
		conditional, found := (*ast.IfStmt)(nil), false
		for parent := parents[call]; parent != nil; parent = parents[parent] {
			if candidate, ok := parent.(*ast.IfStmt); ok {
				conditional, found = candidate, true
				break
			}
		}
		if !found || readOnlyWALManualCompact(conditional.Cond) != "needsRecovery" || conditional.Else != nil || parents[conditional] != root {
			report("control and pre-DB may never execute recovery; one post-write branch required")
		} else {
			recoveryAt := -1
			for i, statement := range root.List {
				if statement == conditional {
					recoveryAt = i
				}
			}
			if recoveryAt <= copyAt || recoveryAt >= afterAt {
				report("recovery must stay between fresh replay copy and independent readback")
			}
		}
		return true
	})
	selectorBindings := map[string]int{}
	ast.Inspect(file, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok {
			if declaration, ok := node.(*ast.ValueSpec); ok {
				for _, name := range declaration.Names {
					if name.Name == "preDB" || name.Name == "postDB" || name.Name == "needsRecovery" {
						report("fixed pre-DB/recovery selectors cannot be shadowed or reassigned")
					}
				}
			}
			return true
		}
		for _, left := range assignment.Lhs {
			id, ok := left.(*ast.Ident)
			if !ok || id.Name != "preDB" && id.Name != "postDB" && id.Name != "needsRecovery" {
				continue
			}
			fn := oracleEnclosingFunction(assignment, parents)
			want := ""
			if fn != nil && id.Name == "preDB" && (fn.Name.Name == rollbackFaultSQLRoot || fn.Name.Name == "manualRollbackSQLCommit") {
				want = `preDB := c.caseID == rollbackfault.SQLJournalBodySyncBefore || c.caseID == rollbackfault.SQLJournalHeaderSyncBefore || c.caseID == rollbackfault.SQLPublishDirectorySyncBefore`
			}
			if fn != nil && id.Name == "postDB" && (fn.Name.Name == rollbackFaultSQLRoot || fn.Name.Name == "manualRollbackSQLCommit") {
				want = `postDB := c.caseID == rollbackfault.SQLJournalUnlinkBefore || c.caseID == rollbackfault.SQLFinalDirectorySyncBefore`
			}
			if fn != nil && id.Name == "needsRecovery" && fn.Name.Name == rollbackFaultSQLRoot {
				want = `needsRecovery := c.caseID == rollbackfault.SQLIndexPrefix64 || c.caseID == rollbackfault.SQLDBSyncBefore || c.caseID == rollbackfault.SQLIndexPrefix64Error || c.caseID == rollbackfault.SQLJournalUnlinkBefore || c.caseID == rollbackfault.SQLIndexMisplaceMinus64Error`
			}
			if want == "" || readOnlyWALManualCompact(assignment) != readOnlyWALManualCompactText(want) || parents[assignment] != fn.Body {
				report("fixed pre-DB/recovery selectors cannot be shadowed or reassigned")
			} else {
				selectorBindings[fn.Name.Name+"/"+id.Name]++
			}
		}
		return true
	})
	if selectorBindings[rollbackFaultSQLRoot+"/preDB"] != 1 || selectorBindings[rollbackFaultSQLRoot+"/needsRecovery"] != 1 || selectorBindings["manualRollbackSQLCommit/preDB"] != 1 || selectorBindings[rollbackFaultSQLRoot+"/postDB"] != 1 || selectorBindings["manualRollbackSQLCommit/postDB"] != 1 {
		report("fixed pre-DB/recovery selector bindings required once")
	}
	require(rollbackFaultSQLRoot, "post-DB retained NEW snapshot before any replay", `postDB := c.caseID == rollbackfault.SQLJournalUnlinkBefore || c.caseID == rollbackfault.SQLFinalDirectorySyncBefore`, `if postDB { postCommit,readErr := manualRollbackSQLReadback(&c,ctx,filepath.Join(c.root,"post-commit.input.db"),identity); evidence.Lifecycle.ReopenCalls++; if readErr != nil { t.Fatalf("post-DB complete NEW readback: %v",readErr) }; evidence.PostCommit = postCommit; if problem := rollbackfault.ValidateSQLSnapshot(postCommit,identity,rollbackfault.SQLNew); problem != rollbackfault.SQLProblemNone { t.Fatalf("post-DB independent NEW ledger rejected before replay: %v",problem) }; if !c.ledger.retired() || !manualRollbackSQLRegistryEmpty() { t.Fatal("INCOMPLETE: post-DB NEW snapshot owner retirement before replay") }; if err := c.retainedUnchanged(); err != nil { t.Fatal(err) } }`)
	findings = append(findings, rollbackFaultSQLExactFunction(path, functions, rollbackFaultSQLRoot, "post-DB root exact one-shot captured observations and recovery decisions", `func TestManualRollbackFaultSQLLedger(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" {
		t.Fatal("NOT_RUN: separate execution authorization, reviewed provenance and private retained root required")
	}
	var c manualRollbackSQLConfig
	defer c.ledger.finish(t)
	if err := manualRollbackSQLAdmit(&c); err != nil {
		t.Fatalf("BLOCKED before fixture: %v", err)
	}
	identity := rollbackfault.Identity{Suite: "rollback-fault-sql-v1", Source: c.binding.Source, Tool: c.binding.Tool, Binary: c.binding.Binary, Manifest: c.binding.Manifest, Trial: c.binding.Trial, Scope: c.binding.Scope, Profile: rollbackfault.PlainRollback4096}
	plan, problem := rollbackfault.SQLPlan(identity, c.caseID)
	if problem != rollbackfault.NoProblem {
		t.Fatalf("BLOCKED before fixture: invalid fixed plan %v", problem)
	}
	preDB := c.caseID == rollbackfault.SQLJournalBodySyncBefore || c.caseID == rollbackfault.SQLJournalHeaderSyncBefore || c.caseID == rollbackfault.SQLPublishDirectorySyncBefore
	postDB := c.caseID == rollbackfault.SQLJournalUnlinkBefore || c.caseID == rollbackfault.SQLFinalDirectorySyncBefore
	needsRecovery := c.caseID == rollbackfault.SQLIndexPrefix64 || c.caseID == rollbackfault.SQLDBSyncBefore || c.caseID == rollbackfault.SQLIndexPrefix64Error || c.caseID == rollbackfault.SQLJournalUnlinkBefore || c.caseID == rollbackfault.SQLIndexMisplaceMinus64Error
	baseline, err := manualRollbackSQLBaseline()
	if err != nil {
		t.Fatal(err)
	}
	frozenHashes, err := manualRollbackSQLPageHashes(baseline[:])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.create("baseline.input.db", baseline[:], true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.create("binding.input.json", c.bindingBytes, true); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	before, err := manualRollbackSQLReadback(&c, ctx, filepath.Join(c.root, "baseline.input.db"), identity)
	if err != nil {
		t.Fatalf("baseline readback: %v", err)
	}
	if problem := rollbackfault.ValidateSQLSnapshot(before, identity, rollbackfault.SQLOld); problem != rollbackfault.SQLProblemNone {
		t.Fatalf("baseline independent ledger rejected before fault: %v", problem)
	}
	if err := c.retainedUnchanged(); err != nil {
		t.Fatal(err)
	}
	trialInfo, err := c.create("trial.db", baseline[:], false)
	if err != nil {
		t.Fatal(err)
	}
	report, commitCheckErr := manualRollbackSQLCommit(&c, ctx, filepath.Join(c.root, "trial.db"), plan)
	misplaced := c.caseID == rollbackfault.SQLIndexMisplaceMinus64Error
	var misplacement rollbackfault.SQLMisplacementEvidence
	var misplacementErr error
	var post, journalBytes []byte
	var postInfo os.FileInfo
	if misplaced {
		misplacement.Writer, misplacement.OnlyTargetCause = report.MisplacedWrite, report.MisplaceOnlyTargetCause
		if commitCheckErr == nil {
			post, postInfo, err = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, "trial.db"), 12288)
			misplacementErr = err
			if err == nil && (len(post) != 12288 || !os.SameFile(trialInfo, postInfo)) {
				misplacementErr = fmt.Errorf("misplaced-write post-Commit extent/identity changed")
			}
			if misplacementErr == nil {
				journalBytes, _, misplacementErr = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, "trial.db-journal"), 16408)
			}
			if misplacementErr == nil {
				misplacement, misplacementErr = manualRollbackSQLMisplacement(report, baseline[:], post, journalBytes)
			}
		}
	}
	// Retain the untouched public error text and all original diagnostics even
	// when the trial must stop before copying/recovery. JSON is supplementary
	// evidence; error identity checks occurred directly on the original object.
	var diagnosticTexts [rollbackfault.MaxEvents]string
	for i, diagnostic := range report.Diagnostics {
		if diagnostic != nil {
			diagnosticTexts[i] = diagnostic.Error()
		}
	}
	commitText := ""
	if report.CommitError != nil {
		commitText = report.CommitError.Error()
	}
	misplacementText := ""
	if misplacementErr != nil {
		misplacementText = misplacementErr.Error()
	}
	commitRecord, marshalErr := json.MarshalIndent(struct {
		Identity              rollbackfault.Identity
		Plan                  rollbackfault.PlanSpec
		Observation           rollbackfault.Observation
		Events                [rollbackfault.MaxEvents]rollbackfault.Event
		EventCount            uint8
		CommitError           string
		Diagnostics           [rollbackfault.MaxEvents]string
		PreDBOnlyTargetCause  bool
		PostDBOnlyTargetCause bool
		Misplacement rollbackfault.SQLMisplacementEvidence
		MisplacementError string
	}{identity, plan.Spec(), report.Observation, report.Events, report.EventCount, commitText, diagnosticTexts, report.PreDBOnlyTargetCause, report.PostDBOnlyTargetCause, misplacement, misplacementText}, "", "  ")
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if _, err := c.create("commit-observation.json", commitRecord, true); err != nil {
		t.Fatal(err)
	}
	if commitCheckErr != nil {
		t.Fatalf("INCOMPLETE: original Commit=%v; adapter=%+v; problem=%v; all inputs retained", report.CommitError, report.Observation, commitCheckErr)
	}
	if misplacementErr != nil {
		t.Fatalf("INCOMPLETE: misplaced-write byte/journal proof: %v; all inputs retained", misplacementErr)
	}
	receipt, receiptProblem := rollbackfault.NewReceipt(plan)
	if receiptProblem != rollbackfault.NoProblem {
		t.Fatal(receiptProblem)
	}
	for i := uint8(0); i < report.EventCount; i++ {
		if problem := receipt.Append(report.Events[i]); problem != rollbackfault.NoProblem {
			t.Fatal(problem)
		}
	}
	if receipt.Seal() != report.Observation {
		t.Fatal("adapter transcript/observation mismatch")
	}
	evidence := rollbackfault.SQLLedgerEvidence{Identity: identity, Case: c.caseID, Before: before, Misplacement: misplacement}
	evidence.Lifecycle = rollbackfault.SQLLifecycleEvidence{ArmCalls: 1, CommitCalls: 1, FinishCalls: 1, DisarmCalls: 1, WriterOpens: 1, ReopenCalls: 1, ReopenReadOnly: true, WriterRetiredBeforeRecovery: true, RecoveryFinishedBeforeReopen: true, CommitCapturedUnchanged: true}
	var publicError *Error
	evidence.Lifecycle.PublicCodeIOErr = errors.As(report.CommitError, &publicError) && publicError.Code == CodeIOErr
	evidence.Lifecycle.PublicRecoveryRequired = errors.Is(report.CommitError, ErrRecoveryRequired)
	evidence.Lifecycle.PublicShortWrite = errors.Is(report.CommitError, io.ErrShortWrite)
	evidence.Lifecycle.PublicPoisoned = errors.Is(report.CommitError, pager.ErrPoisoned)
	evidence.Lifecycle.PublicCommitUncertain = errors.Is(report.CommitError, pager.ErrCommitUncertain)
	evidence.Lifecycle.PublicCleanupPending = errors.Is(report.CommitError, pager.ErrPlaintextCleanupPending)
	evidence.Lifecycle.PreDBOnlyTargetCause = report.PreDBOnlyTargetCause
	evidence.Lifecycle.PostDBOnlyTargetCause = report.PostDBOnlyTargetCause
	if !misplaced {
		post, postInfo, err = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, "trial.db"), 12288)
	}
	if err != nil || len(post) != 12288 || !os.SameFile(trialInfo, postInfo) {
		t.Fatalf("post-Commit extent/identity: %v", err)
	}
	if preDB && !bytes.Equal(post, baseline[:]) {
		t.Fatal("INCOMPLETE: pre-DB failure changed original database bytes")
	}
	if (c.caseID == rollbackfault.SQLIndexPrefix64 || c.caseID == rollbackfault.SQLIndexPrefix64Error) && !bytes.Equal(post[8192+64:], baseline[8192+64:]) {
		t.Fatal("index short write changed bytes beyond the frozen 64-byte prefix")
	}
	if err := c.retain("trial.db", trialInfo, 12288); err != nil {
		t.Fatal(err)
	}
	if _, err := c.create("post-commit.input.db", post, true); err != nil {
		t.Fatal(err)
	}
	if postDB {
		// Read the retained journal-free byte copy, never the original trial.
		// All seven NEW queries, Check and retired readers precede any replay.
		postCommit, readErr := manualRollbackSQLReadback(&c, ctx, filepath.Join(c.root, "post-commit.input.db"), identity)
		evidence.Lifecycle.ReopenCalls++
		if readErr != nil {
			t.Fatalf("post-DB complete NEW readback: %v", readErr)
		}
		evidence.PostCommit = postCommit
		if problem := rollbackfault.ValidateSQLSnapshot(postCommit, identity, rollbackfault.SQLNew); problem != rollbackfault.SQLProblemNone {
			t.Fatalf("post-DB independent NEW ledger rejected before replay: %v", problem)
		}
		if !c.ledger.retired() || !manualRollbackSQLRegistryEmpty() {
			t.Fatal("INCOMPLETE: post-DB NEW snapshot owner retirement before replay")
		}
		if err := c.retainedUnchanged(); err != nil {
			t.Fatal(err)
		}
	}
	if !needsRecovery {
		if _, err := os.Lstat(filepath.Join(c.root, "trial.db-journal")); !os.IsNotExist(err) {
			t.Fatalf("zero-recovery case journal must be absent after Commit: %v", err)
		}
	} else {
		if !misplaced {
			journalBytes, _, err = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, "trial.db-journal"), 16408)
		}
		if err != nil || len(journalBytes) != 16408 {
			t.Fatalf("exact original three-record journal: %v", err)
		}
		if err := c.retain("trial.db-journal", nil, 16408); err != nil {
			t.Fatal(err)
		}
		if _, err := c.create("post-commit-journal.input.bin", journalBytes, true); err != nil {
			t.Fatal(err)
		}
	}
	replayInfo, err := c.create("replay.db", post, false)
	if err != nil || os.SameFile(trialInfo, replayInfo) {
		t.Fatalf("exclusive fresh replay: %v", err)
	}
	if needsRecovery {
		if _, err := c.create("replay.db-journal", journalBytes, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := manualRollbackSQLCapture(&c, &evidence, false); err != nil {
		t.Fatal(err)
	}
	for i, artifact := range evidence.Artifacts {
		for j := 0; j < i; j++ {
			other := evidence.Artifacts[j].Before
			if artifact.Before.Exists && other.Exists && artifact.Before.Device == other.Device && artifact.Before.Inode == other.Inode {
				t.Fatal("replay or retained artifact aliases an original")
			}
		}
	}
	if err := manualRollbackSQLNamespaceContents(&c); err != nil {
		t.Fatal(err)
	}
	if err := c.retainedUnchanged(); err != nil {
		t.Fatal(err)
	}
	oldBefore, _, err := manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, "baseline.input.db"), 12288)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Originals.Frozen = frozenHashes
	evidence.Originals.Before, err = manualRollbackSQLPageHashes(oldBefore)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Originals.BeforeBytes = int64(len(oldBefore))
	evidence.Recovery = rollbackfault.SQLRecoveryEvidence{Identity: identity, OwnersRetired: manualRollbackSQLRegistryEmpty()}
	if needsRecovery {
		result, recoveryErr := manualRollbackSQLRecoverCopy(ctx, filepath.Join(c.root, "replay.db"))
		evidence.Recovery.Calls++
		if recoveryErr != nil {
			t.Fatalf("INCOMPLETE: recovery failed; all originals retained: %v", recoveryErr)
		}
		evidence.Recovery.Limits = rollbackfault.SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}
		evidence.Recovery.JournalPresentBefore = evidence.Artifacts[rollbackfault.SQLReplayJournalArtifact].Before.Exists
		evidence.Recovery.Replayed, evidence.Recovery.ReplayedPages, evidence.Recovery.OriginalPages = result.Recovered, result.RestoredPages, result.OriginalPages
		evidence.Recovery.OwnersRetired = manualRollbackSQLRegistryEmpty()
	}
	if _, err := os.Lstat(filepath.Join(c.root, "replay.db-journal")); !os.IsNotExist(err) {
		t.Fatalf("replay journal not absent after the fixed recovery decision: %v", err)
	}
	if err := c.retainedUnchanged(); err != nil {
		t.Fatal(err)
	}
	after, err := manualRollbackSQLReadback(&c, ctx, filepath.Join(c.root, "replay.db"), identity)
	if err != nil {
		t.Fatalf("fresh read-only SQL/index readback: %v", err)
	}
	evidence.After = after
	want := rollbackfault.SQLOld
	if c.caseID == rollbackfault.SQLControl || c.caseID == rollbackfault.SQLFinalDirectorySyncBefore {
		want = rollbackfault.SQLNew
	}
	if problem := rollbackfault.ValidateSQLSnapshot(after, identity, want); problem != rollbackfault.SQLProblemNone {
		t.Fatalf("independent full SQL/index ledger rejected: %v", problem)
	}
	if err := manualRollbackSQLCapture(&c, &evidence, true); err != nil {
		t.Fatal(err)
	}
	oldAfter, _, err := manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, "baseline.input.db"), 12288)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Originals.After, err = manualRollbackSQLPageHashes(oldAfter)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Originals.AfterBytes = int64(len(oldAfter))
	if err := c.retainedUnchanged(); err != nil {
		t.Fatal(err)
	}
	if err := manualRollbackSQLNamespaceContents(&c); err != nil {
		t.Fatal(err)
	}
	if !c.ledger.retired() || !manualRollbackSQLRegistryEmpty() {
		t.Fatal("INCOMPLETE: terminal owner retirement")
	}
	observation := rollbackfault.ObserveSQLLedger(plan, receipt, evidence)
	if !observation.EvidenceConsistent || !observation.ScenarioObserved || observation.Problem != rollbackfault.SQLProblemNone || observation.PhysicalValidated() || observation.Pass() {
		t.Fatalf("SQL evidence inconsistent: %+v", observation)
	}
	terminal, err := json.MarshalIndent(struct {
		Schema         string
		Evidence       rollbackfault.SQLLedgerEvidence
		Observation    rollbackfault.SQLObservation
		SourceFiles    uint16
		CompileReceipt string
	}{"rollback-fault-sql-observed-v1", evidence, observation, c.sourceFiles, c.binding.CompileReceipt}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.create("result.json", terminal, false); err != nil {
		t.Fatal(err)
	}
	if !c.ledger.retired() || !manualRollbackSQLRegistryEmpty() {
		t.Fatal("INCOMPLETE: terminal file owner retirement")
	}
	t.Logf("ROLLBACK_SQL_SCENARIO_OBSERVED case=%s trial=%s source=%s binary=%s physical_validated=false pass=false", c.binding.Case, c.binding.Trial, c.binding.Source, c.binding.Binary)
}`)...)
	return findings
}

func rollbackFaultSQLModelSemantics(path string, raw []byte, set *token.FileSet, file *ast.File, functions map[string]*ast.FuncDecl) []string {
	var findings []string
	report := func(message string) { findings = append(findings, path+": rollback SQL contract: "+message) }
	require := func(name, diagnostic string, parts ...string) {
		findings = append(findings, rollbackFaultSQLRequire(path, functions, name, diagnostic, parts...)...)
	}
	var scalar func(ast.Expr) bool
	scalar = func(expression ast.Expr) bool {
		switch value := expression.(type) {
		case *ast.Ident:
			return value.Name != "any" && value.Name != "error" && value.Name != "uintptr" && value.Name != "unsafe"
		case *ast.ArrayType:
			return value.Len != nil && scalar(value.Elt)
		case *ast.StructType:
			for _, field := range value.Fields.List {
				if !scalar(field.Type) {
					return false
				}
			}
			return true
		}
		return false
	}
	for _, declaration := range file.Decls {
		if gen, ok := declaration.(*ast.GenDecl); ok {
			for _, spec := range gen.Specs {
				if typ, ok := spec.(*ast.TypeSpec); ok && (typ.Assign.IsValid() || !scalar(typ.Type)) {
					report("SQL model types must remain descriptor-free fixed scalar values")
				}
			}
		}
	}
	findings = append(findings, rollbackFaultSQLExactFunction(path, functions, "validSQLMisplacement", "fixed misplaced-write argument/hash/OLD proof and old-nine zero evidence", `func validSQLMisplacement(which SQLCase, evidence SQLMisplacementEvidence, originals [SQLPageCount][32]byte) bool {
	if which != SQLIndexMisplaceMinus64Error {
		return evidence == (SQLMisplacementEvidence{})
	}
	writer := evidence.Writer
	return writer.RequestedOffset == 8192 && writer.RequestedBytes == 4096 && writer.NativeOffset == 8128 && writer.NativeBytes == 4096 && writer.NativeCalls == 1 && writer.PayloadSHA256 != ([32]byte{}) && writer.PayloadSHA256 == evidence.ShiftedSHA256 && evidence.BaselineRangeSHA256 != ([32]byte{}) && evidence.ShiftedSHA256 != evidence.BaselineRangeSHA256 && evidence.BaselineTailSHA256 != ([32]byte{}) && evidence.UntouchedTailSHA256 == evidence.BaselineTailSHA256 && evidence.JournalOriginals == originals && evidence.OnlyTargetCause
}`)...)
	caseDeclarations, lifecycleDeclarations, ledgerDeclarations := 0, 0, 0
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			if value, ok := spec.(*ast.ValueSpec); ok {
				for _, name := range value.Names {
					if name.Name == "SQLCaseUnknown" {
						caseDeclarations++
						if readOnlyWALManualCompact(gen) != readOnlyWALManualCompactText(`const ( SQLCaseUnknown SQLCase = iota; SQLControl; SQLIndexPrefix64; SQLDBSyncBefore; SQLIndexPrefix64Error; SQLJournalBodySyncBefore; SQLJournalHeaderSyncBefore; SQLPublishDirectorySyncBefore; SQLJournalUnlinkBefore; SQLFinalDirectorySyncBefore; SQLIndexMisplaceMinus64Error )`) {
							report("exact fixed SQL case IDs 0 through 10 required")
						}
					}
				}
			}
			if typ, ok := spec.(*ast.TypeSpec); ok && typ.Name.Name == "SQLMisplacedWrite" {
				if readOnlyWALManualCompact(typ) != readOnlyWALManualCompactText(`SQLMisplacedWrite struct { RequestedOffset int64; RequestedBytes int; NativeOffset int64; NativeBytes int; NativeCalls uint8; PayloadSHA256 [32]byte }`) {
					report("fixed misplaced-write actual writer record")
				}
			}
			if typ, ok := spec.(*ast.TypeSpec); ok && typ.Name.Name == "SQLMisplacementEvidence" {
				if readOnlyWALManualCompact(typ) != readOnlyWALManualCompactText(`SQLMisplacementEvidence struct { Writer SQLMisplacedWrite; ShiftedSHA256 [32]byte; BaselineRangeSHA256 [32]byte; BaselineTailSHA256 [32]byte; UntouchedTailSHA256 [32]byte; JournalOriginals [SQLPageCount][32]byte; OnlyTargetCause bool }`) {
					report("fixed misplaced-write closed SQL evidence")
				}
			}
			if typ, ok := spec.(*ast.TypeSpec); ok && typ.Name.Name == "SQLLedgerEvidence" {
				ledgerDeclarations++
				if readOnlyWALManualCompact(typ) != readOnlyWALManualCompactText(`SQLLedgerEvidence struct { Identity Identity; Case SQLCase; Before SQLSnapshotEvidence; PostCommit SQLSnapshotEvidence; After SQLSnapshotEvidence; Originals SQLOriginalEvidence; Artifacts [SQLArtifactCount]SQLArtifactEvidence; Recovery SQLRecoveryEvidence; Lifecycle SQLLifecycleEvidence; Misplacement SQLMisplacementEvidence; PhysicalClaim bool }`) {
					report("closed SQL ledger includes exactly one full post-Commit snapshot")
				}
			}
			if typ, ok := spec.(*ast.TypeSpec); ok && typ.Name.Name == "SQLLifecycleEvidence" {
				lifecycleDeclarations++
				if readOnlyWALManualCompact(typ) != readOnlyWALManualCompactText(`SQLLifecycleEvidence struct { ArmCalls uint8; CommitCalls uint8; FinishCalls uint8; DisarmCalls uint8; RollbackCalls uint8; WriterOpens uint8; ReopenCalls uint8; ReopenReadOnly bool; WriterRetiredBeforeRecovery bool; RecoveryFinishedBeforeReopen bool; CommitCapturedUnchanged bool; PublicCodeIOErr bool; PublicRecoveryRequired bool; PublicShortWrite bool; PublicPoisoned bool; PublicCommitUncertain bool; PublicCleanupPending bool; PreDBOnlyTargetCause bool; PostDBOnlyTargetCause bool }`) {
					report("closed public lifecycle evidence fields required")
				}
			}
		}
	}
	if caseDeclarations != 1 || lifecycleDeclarations != 1 || ledgerDeclarations != 1 {
		report("exact SQL case and lifecycle declarations required")
	}
	for _, name := range []string{"PhysicalValidated", "Pass"} {
		fn := functions[name]
		if fn == nil || readOnlyWALManualCompact(fn) != readOnlyWALManualCompactText("func (SQLObservation) "+name+"() bool { return false }") {
			report("SQL observation cannot confer physical validation or PASS")
		}
	}
	require("SQLLedgerRows", "independent literal old/new ledger", `case SQLOld: return [3]SQLRow{{SQLInteger,1,SQLText,"red",SQLText,"one"},{SQLInteger,2,SQLText,"blue",SQLText,"two"},{SQLInteger,3,SQLText,"red",SQLText,"three"}}`, `case SQLNew: return [3]SQLRow{{SQLInteger,1,SQLText,"green",SQLText,"ONE"},{SQLInteger,3,SQLText,"red",SQLText,"three"},{SQLInteger,4,SQLText,"blue",SQLText,"four"}}`, `default: return [3]SQLRow{}`)
	require("SQLQueryText", "seven independent scan/index statements", `case SQLAllRows: return "SELECT id,k,v FROM t NOT INDEXED ORDER BY id"`, `case SQLBlueIndex: return "SELECT id,k,v FROM t WHERE k='blue' ORDER BY id"`, `case SQLBlueScan: return "SELECT id,k,v FROM t NOT INDEXED WHERE k='blue' ORDER BY id"`, `case SQLRedIndex: return "SELECT id,k,v FROM t WHERE k='red' ORDER BY id"`, `case SQLRedScan: return "SELECT id,k,v FROM t NOT INDEXED WHERE k='red' ORDER BY id"`, `case SQLGreenIndex: return "SELECT id,k,v FROM t WHERE k='green' ORDER BY id"`, `case SQLGreenScan: return "SELECT id,k,v FROM t NOT INDEXED WHERE k='green' ORDER BY id"`)
	require("SQLPlan", "frozen three-page ten-case selectors", `OriginalPages:3,DirtyPages:3,PageSize:SQLPageSize,SectorSize:SQLPageSize`, `{Point:DBPage,Occurrence:3,Role:Database,Owner:11,Offset:8192,Length:4096}`, `CallCount:18`, `spec.Target = spec.Calls[14]`, `case SQLControl:`, `case SQLIndexPrefix64: spec.Action,spec.Prefix,spec.PrefixError = PrefixShortWrite,64,false; spec.ExpectedCommit,spec.CallCount = CommitRecoveryRequired,15`, `case SQLDBSyncBefore: spec.Target,spec.Action = spec.Calls[15],FailBefore; spec.ExpectedCommit,spec.CallCount = CommitRecoveryRequired,16`, `case SQLIndexPrefix64Error: spec.Action,spec.Prefix,spec.PrefixError = PrefixShortWrite,64,true; spec.ExpectedCommit,spec.CallCount = CommitRecoveryRequired,15`, `default: return Plan{},ProblemTarget`, `for i := int(spec.CallCount); i < MaxCalls; i++ { spec.Calls[i] = Call{} }`, `return NewPlan(spec)`)
	require("SQLPlan", "ten exact fixed plan branches", `switch which { case SQLControl: case SQLIndexPrefix64: spec.Action,spec.Prefix,spec.PrefixError = PrefixShortWrite,64,false; spec.ExpectedCommit,spec.CallCount = CommitRecoveryRequired,15; case SQLDBSyncBefore: spec.Target,spec.Action = spec.Calls[15],FailBefore; spec.ExpectedCommit,spec.CallCount = CommitRecoveryRequired,16; case SQLIndexPrefix64Error: spec.Action,spec.Prefix,spec.PrefixError = PrefixShortWrite,64,true; spec.ExpectedCommit,spec.CallCount = CommitRecoveryRequired,15; case SQLJournalBodySyncBefore: spec.Target,spec.Action = spec.Calls[8],FailBefore; spec.ExpectedCommit,spec.CallCount = CommitFailed,11; case SQLJournalHeaderSyncBefore: spec.Target,spec.Action = spec.Calls[10],FailBefore; spec.ExpectedCommit,spec.CallCount = CommitFailed,13; case SQLPublishDirectorySyncBefore: spec.Target,spec.Action = spec.Calls[11],FailBefore; spec.ExpectedCommit,spec.CallCount = CommitFailed,14; case SQLJournalUnlinkBefore: spec.Target,spec.Action = spec.Calls[16],FailBefore; spec.ExpectedCommit,spec.CallCount = CommitRecoveryRequired,17; case SQLFinalDirectorySyncBefore: spec.Target,spec.Action = spec.Calls[17],FailBefore; spec.ExpectedCommit,spec.CallCount = CommitUncertain,18; case SQLIndexMisplaceMinus64Error: spec.Action = MisplaceIndexPageMinus64; spec.ExpectedCommit,spec.CallCount = CommitRecoveryRequired,15; default: return Plan{},ProblemTarget }`)
	require("SQLPlan", "pre-DB fixed target geometry and normal cleanup tail", `{Point:JournalBodySync,Occurrence:1,Role:Journal,Owner:12}`, `{Point:JournalHeaderSync,Occurrence:1,Role:Journal,Owner:12}`, `{Point:PublishDirectorySync,Occurrence:1,Role:Directory,Owner:13}`, `if isSQLPreDBFailure(which) { spec.Calls[spec.CallCount-2] = Call{Point:CleanupUnlink,Occurrence:1,Role:Journal,Owner:12}; spec.Calls[spec.CallCount-1] = Call{Point:CleanupDirectorySync,Occurrence:1,Role:Directory,Owner:13} }`)
	for name, want := range map[string]string{
		"isSQLPreDBFailure":  `func isSQLPreDBFailure(which SQLCase) bool { return which == SQLJournalBodySyncBefore || which == SQLJournalHeaderSyncBefore || which == SQLPublishDirectorySyncBefore }`,
		"isSQLPostDBFailure": `func isSQLPostDBFailure(which SQLCase) bool { return which == SQLJournalUnlinkBefore || which == SQLFinalDirectorySyncBefore }`,
		"needsSQLRecovery":   `func needsSQLRecovery(which SQLCase) bool { return which == SQLIndexPrefix64 || which == SQLDBSyncBefore || which == SQLIndexPrefix64Error || which == SQLJournalUnlinkBefore || which == SQLIndexMisplaceMinus64Error }`,
	} {
		if fn := functions[name]; fn == nil || readOnlyWALManualCompact(fn) != readOnlyWALManualCompactText(want) {
			report("exact pre-DB, post-DB and five recovery cases require closed predicates")
		}
	}
	require("isSQLPreDBFailure", "exact three pre-DB cases", `return which == SQLJournalBodySyncBefore || which == SQLJournalHeaderSyncBefore || which == SQLPublishDirectorySyncBefore`)
	require("needsSQLRecovery", "exact five post-write recovery cases", `return which == SQLIndexPrefix64 || which == SQLDBSyncBefore || which == SQLIndexPrefix64Error || which == SQLJournalUnlinkBefore || which == SQLIndexMisplaceMinus64Error`)
	require("ObserveSQLLedger", "receipt identity and old/new mixtures rejected", `if evidence.PhysicalClaim`, `problem != ProblemNone || !plan.valid || plan.spec != expectedPlan.spec`, `!receipt.plan.valid || receipt.plan.spec != plan.spec`, `result.Adapter = receipt.Seal()`, `result.Adapter.Outcome != AdapterObserved || result.Adapter.Problem != ProblemNone || result.Adapter.TargetHits != 1`, `evidence.Identity != plan.spec.Identity || evidence.Before.Identity != evidence.Identity || evidence.After.Identity != evidence.Identity || evidence.Recovery.Identity != evidence.Identity`, `if evidence.Lifecycle != lifecycle`, `if !validSQLReaderOwners(plan.spec,evidence.Before.Owner,evidence.After.Owner)`, `validateSQLSnapshot(evidence.Before,evidence.Identity,SQLOld)`, `state := SQLOld`, `if evidence.Case == SQLControl || evidence.Case == SQLFinalDirectorySyncBefore { state = SQLNew }`, `validateSQLSnapshot(evidence.After,evidence.Identity,state)`, `if before.Iterator == after.Iterator`, `original.BeforeBytes != SQLDatabaseBytes || original.AfterBytes != SQLDatabaseBytes || original.Frozen != original.Before || original.Frozen != original.After`, `if digest == ([32]byte{})`, `if !validSQLRecovery(evidence.Case,evidence.Recovery)`, `if !validSQLArtifacts(evidence.Case,evidence.Artifacts)`)
	require("ObserveSQLLedger", "one-shot public error lifecycle", `lifecycle := SQLLifecycleEvidence{ArmCalls:1,CommitCalls:1,FinishCalls:1,DisarmCalls:1,WriterOpens:1,ReopenCalls:1,ReopenReadOnly:true,WriterRetiredBeforeRecovery:true,RecoveryFinishedBeforeReopen:true,CommitCapturedUnchanged:true}`, `if evidence.Case != SQLControl`, `lifecycle.PublicCodeIOErr,lifecycle.PublicPoisoned = true,true`, `if evidence.Case != SQLControl { lifecycle.PublicCodeIOErr,lifecycle.PublicPoisoned = true,true; lifecycle.PublicRecoveryRequired = needsSQLRecovery(evidence.Case); lifecycle.PublicShortWrite = evidence.Case == SQLIndexPrefix64; lifecycle.PreDBOnlyTargetCause = isSQLPreDBFailure(evidence.Case); lifecycle.PostDBOnlyTargetCause = isSQLPostDBFailure(evidence.Case); lifecycle.PublicCommitUncertain = evidence.Case == SQLFinalDirectorySyncBefore }`)
	require("validateSQLSnapshot", "full rows, Check, EOF and exact closure", `snapshot.CloseCalls != 1 || snapshot.CloseError != None || snapshot.CloseErrorToken != 0 || snapshot.OwnerState != OwnerRetired`, `check.Calls != 1 || check.Error != None || check.ErrorToken != 0 || check.Pages != 3 || check.FreePages != 0 || check.ReservedPages != 0 || check.Tables != 1 || check.Indexes != 1 || check.ExtensionIndexes != 0 || check.TableRows != 3 || check.IndexEntries != 3`, `query.Query != SQLQuery(i) || query.SQL != SQLQueryText(SQLQuery(i))`, `!query.EOF || !query.ErrCalled || query.Error != None || query.ErrorToken != 0`, `query.CloseCalls != 1 || query.CloseError != None || query.CloseErrorToken != 0`, `rows,count := expectedSQLQueryRows(state,query.Query)`, `query.RowCount != count || query.Rows != rows`, `!validSQLQueryPlan(query.Query,query.Plan)`, `!validSQLQueryWork(query.Query,query.Work,count)`)
	require("validSQLQueryPlan", "by_k plan and independent NOT INDEXED plan", `plan.Access == "SEARCH" && plan.Method == "btree" && plan.Index == "by_k" && plan.IndexCount == 1 && plan.Field == "k" && plan.FieldCount == 1 && plan.Recheck`, `plan.Access == "SCAN" && plan.Method == "" && plan.Index == "" && plan.IndexCount == 0 && plan.Field == "" && plan.FieldCount == 0 && !plan.Recheck && plan.Reason == "NOT INDEXED requested"`)
	require("validSQLQueryWork", "index claim requires actual bounded work", `if work.ReturnedRows != n`, `work.IndexPageReads > 0 && work.IndexPageReads <= 32 && work.IndexKeysRead == n && work.Candidates == n && work.RowIDSeeks == n && work.BaseRowFetches == n`, `work.IndexKeysRead == 0 && work.IndexPageReads == 0 && work.RowIDSeeks == 0 && work.Candidates == 0 && work.BaseRowFetches == 3`)
	require("validSQLRecovery", "control/pre-DB no recovery; post-write exact one bounded replay", `if which == SQLControl || isSQLPreDBFailure(which) || which == SQLFinalDirectorySyncBefore`, `return recovery == (SQLRecoveryEvidence{Identity:recovery.Identity,OwnersRetired:true})`, `!needsSQLRecovery(which) || recovery.Calls != 1 || recovery.Error != None || recovery.ErrorToken != 0 || !recovery.OwnersRetired || recovery.JournalPresentAfter || recovery.Limits != (SQLRecoveryLimits{MaxJournalBytes:SQLJournalBytes,MaxRecords:SQLPageCount,MaxDatabaseBytes:SQLDatabaseBytes})`, `return recovery.JournalPresentBefore && recovery.Replayed && recovery.ReplayedPages == SQLPageCount && recovery.OriginalPages == SQLPageCount`)
	require("validSQLArtifacts", "immutable originals and unaliased replay artifacts", `before.Identity != SQLArtifactName(artifact.Role)`, `artifact.Role == SQLReplayJournalArtifact`, `after != (SQLArtifactStamp{})`, `before.Device != after.Device || before.Inode != after.Inode`, `artifact.Role != SQLReplayArtifact && before != after`, `before.Identity == other.Identity || (before.Device == other.Device && before.Inode == other.Inode)`, `trial != post || trial != replayBefore`, `if isSQLPreDBFailure(which) { return baseline == trial && replayAfter == baseline }`, `if baseline == trial { return false }`, `if !needsSQLRecovery(which) { return false }`, `return replayAfter == trial`, `return replayAfter == baseline && journal == artifacts[SQLJournalCopyArtifact].Before.SHA256 && journal == artifacts[SQLReplayJournalArtifact].Before.SHA256`)
	require("validSQLArtifacts", "pre-DB journal absence and whole OLD copies", `if journal && (which == SQLControl || isSQLPreDBFailure(which) || which == SQLFinalDirectorySyncBefore) { if artifact.Before != (SQLArtifactStamp{}) || artifact.After != (SQLArtifactStamp{}) { return false }; continue }`, `if isSQLPreDBFailure(which) { return baseline == trial && replayAfter == baseline }`)
	require("SQLPlan", "post-DB exact fail-before target geometry and successful prefix", `{Point:DBSync,Occurrence:1,Role:Database,Owner:11}`, `{Point:JournalUnlink,Occurrence:1,Role:Journal,Owner:12}`, `{Point:FinalDirectorySync,Occurrence:1,Role:Directory,Owner:13}`)
	require("ObserveSQLLedger", "post-DB full NEW snapshot and independent retired readers", `if isSQLPostDBFailure(evidence.Case) { lifecycle.ReopenCalls = 2 }`, `evidence.PostCommit.Identity != evidence.Identity || !validSQLReaderOwners(plan.spec,evidence.Before.Owner,evidence.PostCommit.Owner) || !validSQLReaderOwners(plan.spec,evidence.After.Owner,evidence.PostCommit.Owner)`, `validateSQLSnapshot(evidence.PostCommit,evidence.Identity,SQLNew)`, `post.Iterator == evidence.Before.Queries[i].Iterator || post.Iterator == evidence.After.Queries[i].Iterator`, `else if evidence.PostCommit != (SQLSnapshotEvidence{}) { result.Problem = SQLProblemLifecycle; return result }`)
	findings = append(findings, rollbackFaultSQLExactFunction(path, functions, "SQLPlan", "ten exact fixed plan branches cannot add future IDs or bypass validation", `func SQLPlan(identity Identity, which SQLCase) (Plan, Problem) {
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
	case SQLJournalBodySyncBefore:
		spec.Target, spec.Action = spec.Calls[8], FailBefore
		spec.ExpectedCommit, spec.CallCount = CommitFailed, 11
	case SQLJournalHeaderSyncBefore:
		spec.Target, spec.Action = spec.Calls[10], FailBefore
		spec.ExpectedCommit, spec.CallCount = CommitFailed, 13
	case SQLPublishDirectorySyncBefore:
		spec.Target, spec.Action = spec.Calls[11], FailBefore
		spec.ExpectedCommit, spec.CallCount = CommitFailed, 14
	case SQLJournalUnlinkBefore:
		spec.Target, spec.Action = spec.Calls[16], FailBefore
		spec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 17
	case SQLFinalDirectorySyncBefore:
		spec.Target, spec.Action = spec.Calls[17], FailBefore
		spec.ExpectedCommit, spec.CallCount = CommitUncertain, 18
	case SQLIndexMisplaceMinus64Error:
		spec.Action = MisplaceIndexPageMinus64
		spec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 15
	default:
		return Plan{}, ProblemTarget
	}
	if isSQLPreDBFailure(which) {
		// The selected Sync/helper is not called. Only the normal two cleanup
		// operations follow; no database write or recovery is in this trace.
		spec.Calls[spec.CallCount-2] = Call{Point: CleanupUnlink, Occurrence: 1, Role: Journal, Owner: 12}
		spec.Calls[spec.CallCount-1] = Call{Point: CleanupDirectorySync, Occurrence: 1, Role: Directory, Owner: 13}
	}
	for i := int(spec.CallCount); i < MaxCalls; i++ {
		spec.Calls[i] = Call{}
	}
	return NewPlan(spec)
}`)...)
	findings = append(findings, rollbackFaultSQLExactFunction(path, functions, "validSQLRecovery", "exact no-replay/replay decisions reject future IDs and bypasses", `func validSQLRecovery(which SQLCase, recovery SQLRecoveryEvidence) bool {
	if which == SQLControl || isSQLPreDBFailure(which) || which == SQLFinalDirectorySyncBefore {
		return recovery == (SQLRecoveryEvidence{Identity: recovery.Identity, OwnersRetired: true})
	}
	if !needsSQLRecovery(which) || recovery.Calls != 1 || recovery.Error != None || recovery.ErrorToken != 0 || !recovery.OwnersRetired || recovery.JournalPresentAfter || recovery.Limits != (SQLRecoveryLimits{MaxJournalBytes: SQLJournalBytes, MaxRecords: SQLPageCount, MaxDatabaseBytes: SQLDatabaseBytes}) {
		return false
	}
	return recovery.JournalPresentBefore && recovery.Replayed && recovery.ReplayedPages == SQLPageCount && recovery.OriginalPages == SQLPageCount
}`)...)
	findings = append(findings, rollbackFaultSQLExactFunction(path, functions, "validSQLArtifacts", "exact post-DB artifact decisions reject future IDs and bypasses", `func validSQLArtifacts(which SQLCase, artifacts [SQLArtifactCount]SQLArtifactEvidence) bool {
	for i, artifact := range artifacts {
		if artifact.Role != SQLArtifactRole(i) {
			return false
		}
		journal := artifact.Role == SQLOriginalJournalArtifact || artifact.Role == SQLJournalCopyArtifact || artifact.Role == SQLReplayJournalArtifact
		if journal && (which == SQLControl || isSQLPreDBFailure(which) || which == SQLFinalDirectorySyncBefore) {
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
	if trial != post || trial != replayBefore {
		return false
	}
	if isSQLPreDBFailure(which) {
		return baseline == trial && replayAfter == baseline
	}
	if baseline == trial {
		return false
	}
	if which == SQLControl || which == SQLFinalDirectorySyncBefore {
		return replayAfter == trial
	}
	if !needsSQLRecovery(which) {
		return false
	}
	journal := artifacts[SQLOriginalJournalArtifact].Before.SHA256
	return replayAfter == baseline && journal == artifacts[SQLJournalCopyArtifact].Before.SHA256 && journal == artifacts[SQLReplayJournalArtifact].Before.SHA256
}`)...)
	findings = append(findings, rollbackFaultSQLExactFunction(path, functions, "ObserveSQLLedger", "post-DB snapshot, actual receipt and lifecycle rejection order cannot be bypassed", `func ObserveSQLLedger(plan Plan, receipt Receipt, evidence SQLLedgerEvidence) SQLObservation {
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
		lifecycle.PublicCodeIOErr, lifecycle.PublicPoisoned = true, true
		lifecycle.PublicRecoveryRequired = needsSQLRecovery(evidence.Case)
		lifecycle.PublicShortWrite = evidence.Case == SQLIndexPrefix64
		lifecycle.PreDBOnlyTargetCause = isSQLPreDBFailure(evidence.Case)
		lifecycle.PostDBOnlyTargetCause = isSQLPostDBFailure(evidence.Case)
		lifecycle.PublicCommitUncertain = evidence.Case == SQLFinalDirectorySyncBefore
	}
	if isSQLPostDBFailure(evidence.Case) {
		lifecycle.ReopenCalls = 2
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
	if evidence.Case == SQLControl || evidence.Case == SQLFinalDirectorySyncBefore {
		state = SQLNew
	}
	if result.Problem = validateSQLSnapshot(evidence.After, evidence.Identity, state); result.Problem != SQLProblemNone {
		return result
	}
	if isSQLPostDBFailure(evidence.Case) {
		if evidence.PostCommit.Identity != evidence.Identity || !validSQLReaderOwners(plan.spec, evidence.Before.Owner, evidence.PostCommit.Owner) || !validSQLReaderOwners(plan.spec, evidence.After.Owner, evidence.PostCommit.Owner) {
			result.Problem = SQLProblemLifecycle
			return result
		}
		if result.Problem = validateSQLSnapshot(evidence.PostCommit, evidence.Identity, SQLNew); result.Problem != SQLProblemNone {
			return result
		}
		for _, post := range evidence.PostCommit.Queries {
			for i := 0; i < SQLQueryCount; i++ {
				if post.Iterator == evidence.Before.Queries[i].Iterator || post.Iterator == evidence.After.Queries[i].Iterator {
					result.Problem = SQLProblemLifecycle
					return result
				}
			}
		}
	} else if evidence.PostCommit != (SQLSnapshotEvidence{}) {
		result.Problem = SQLProblemLifecycle
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
	if !validSQLMisplacement(evidence.Case, evidence.Misplacement, original.Frozen) {
		result.Problem = SQLProblemMisplacement
		return result
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
}`)...)
	return findings
}

func inspectRollbackFaultSQLWiring(raw []byte) []string {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "internal/testscope/main.go", raw, 0)
	if err != nil {
		return []string{"rollback SQL scanner wiring parse failure"}
	}
	functions := readOnlyWALManualFunctions(file)
	body := readOnlyWALManualBody(functions, "main")
	var walk *ast.BlockStmt
	if body != nil {
		ast.Inspect(body, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok && readOnlyWALManualName(call.Fun) == "filepath.WalkDir" && len(call.Args) == 2 {
				if fn, ok := call.Args[1].(*ast.FuncLit); ok {
					walk = fn.Body
				}
			}
			return true
		})
	}
	var findings []string
	report := func(message string) { findings = append(findings, "rollback SQL scanner wiring: "+message) }
	index := readOnlyWALManualSequence(walk, `rollbackSQL,rollbackSQLErr := inspectRollbackFaultSQLSource(path,raw)`, `if rollbackSQLErr != nil { return rollbackSQLErr }`, `findings = append(findings,rollbackSQL...)`)
	if index < 0 {
		report("direct all-source hook must propagate errors and append findings")
	}
	if walk != nil {
		for i, statement := range walk.List {
			if conditional, ok := statement.(*ast.IfStmt); ok && (readOnlyWALManualCompact(conditional.Cond) == `strings.HasSuffix(path,"_test.go")` || readOnlyWALManualCompact(conditional.Cond) == "oracleCurrentGoPath(path)") && index >= i {
				report("SQL scanner must precede every existing platform/test filter")
			}
		}
	}
	if readOnlyWALManualSequence(body, `rollbackSQLContracts,rollbackSQLContractErr := inspectRollbackFaultSQLContracts(".")`, `if rollbackSQLContractErr != nil { fmt.Fprintln(os.Stderr,rollbackSQLContractErr); failed = true }`, `for _, finding := range rollbackSQLContracts { fmt.Fprintln(os.Stderr,finding); failed = true }`) < 0 {
		report("root SQL contract errors/findings must reach failure bit")
	}
	for _, name := range []string{"inspectRollbackFaultSQLSource", "inspectRollbackFaultSQLContracts", "inspectSource", "inspectBackupManualSource", "inspectBackupManualContracts", "inspectWALPublicationSource", "inspectWALPublicationContracts", "inspectOracleSource", "inspectOracleContracts", "inspectDarwinInventoryManualContract", "inspectFormatUpgradeSource", "inspectFormatUpgradeContracts", "inspectRollbackFaultSource", "inspectRollbackFaultContracts", "inspectReadOnlyWALManualSource", "inspectReadOnlyWALManualContracts"} {
		if readOnlyWALManualCountCall(body, name) != 1 {
			report("missing/duplicate prior or SQL hook: " + name)
		}
	}
	ordinary := readOnlyWALManualBody(functions, "inspectSource")
	if ordinary == nil || readOnlyWALManualCountCall(ordinary, "rollbackFaultSQLRecoveryReference") != 2 {
		report("ordinary recovery exception must guard exactly identifier and selector reports")
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != walPublicationFrozenSources["internal/testscope/main.go"] {
		report(rollbackFaultSQLMainReview + ": whole main and inspectSource remain deliberately unreleased")
	}
	return findings
}

func inspectRollbackFaultSQLContracts(root string) ([]string, error) {
	var findings []string
	for path := range rollbackFaultSQLPins {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return findings, err
		}
		observed, err := inspectRollbackFaultSQLSource(path, raw)
		if err != nil {
			return findings, err
		}
		findings = append(findings, observed...)
	}
	return findings, nil
}
