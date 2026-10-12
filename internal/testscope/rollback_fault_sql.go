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
	rollbackFaultSQLBridge:    "76f7a9de35d1823d1108c24e9741c76918fdd0839848e0fc6fa75130a8574f73",
	rollbackFaultSQLNative:    "ec631b99e7e0aa376b09d64f1337b872e1916a2cf0fb23eb3c95c40c93a8ba19",
	rollbackFaultSQLModel:     "5eee321b4f867c2eb5082507091319db9c8d41932f9992655840bd1010ac00c7",
	rollbackFaultSQLModelTest: "21b4dfb8a97139901eda7c282946a5f506da773d8093ab035b68d06110c94a34",
}

// Full closed declaration/body snapshots are supplemental. Counterexamples
// repin both this snapshot and the byte digest and must still fail semantics.
var rollbackFaultSQLShapes = map[string]string{
	rollbackFaultSQLBridge:    "//go:build linux && manual_fault_rollback\n\npackage pager\n\n// This bridge exists only in the manual build. It exposes neither native\n// owners nor a retry/reset path. The default shipping selection is unchanged.\nimport (\n\t\"os\"\n\n\t\"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault\"\n)\n\ntype manualRollbackSQLScope struct{ scope *rollbackFaultScope }\n\ntype ManualRollbackSQLReport struct {\n\tObservation rollbackfault.Observation\n\tEvents      [rollbackfault.MaxEvents]rollbackfault.Event\n\tEventCount  uint8\n\tDiagnostics [rollbackfault.MaxEvents]error\n\tCommitError error\n\tProblem     error\n}\n\n// Snapshots are separately locked scalar observations, not an atomic proof of\n// absence of concurrent users. The harness requires an independent process.\n// HealthyTx means only an untouched, independently owned preparation Tx can be\n// retired through its public Rollback. No native owner method is invoked here.\ntype ManualRollbackSQLSnapshot struct {\n\tGroups        int\n\tPending       int\n\tHealthyTx     bool\n\tAdapterActive bool\n\tAdapterUsed   bool\n}\n\nfunc ManualRollbackSQLState(tx *Tx) ManualRollbackSQLSnapshot {\n\tvar out ManualRollbackSQLSnapshot\n\tplainRetirements.mu.Lock()\n\tout.Groups = len(plainRetirements.groups)\n\tfor g := range plainRetirements.groups {\n\t\tif g.pending.Load() != nil {\n\t\t\tout.Pending++\n\t\t}\n\t}\n\tplainRetirements.mu.Unlock()\n\trollbackFaultRegistry.mu.Lock()\n\tout.AdapterActive = rollbackFaultRegistry.active != nil\n\tout.AdapterUsed = rollbackFaultRegistry.used\n\trollbackFaultRegistry.mu.Unlock()\n\tif tx != nil {\n\t\ttx.mu.Lock()\n\t\tg := tx.cleanup\n\t\tif g != nil {\n\t\t\tg.mu.Lock()\n\t\t\tmain, concrete := g.main.(plainFile)\n\t\t\tout.HealthyTx = concrete && main.file == tx.file && tx.file != nil && tx.session == nil && tx.shared == nil && tx.encrypted == nil && !tx.keepFile && tx.borrowedCleanup == nil && !g.started && !g.done && g.cause == nil && g.pending.Load() == nil && g.temporary == nil && !g.temporaryReserved && !g.temporaryRequested && g.wal == nil && !g.walReserved && !g.walRequested && len(g.nested) == 0\n\t\t\tg.mu.Unlock()\n\t\t}\n\t\ttx.mu.Unlock()\n\t}\n\treturn out\n}\n\nfunc ArmManualRollbackSQLFault(tx *Tx, plan rollbackfault.Plan) (*manualRollbackSQLScope, error) {\n\tif os.Getenv(\"GOSQLITE_MANUAL_FAULT_ROLLBACK\") != \"independently_authorized\" {\n\t\treturn nil, errRollbackFaultGate\n\t}\n\tspec := plan.Spec()\n\tvar caseID rollbackfault.SQLCase\n\tswitch spec.Action {\n\tcase rollbackfault.Pass:\n\t\tcaseID = rollbackfault.SQLControl\n\tcase rollbackfault.PrefixShortWrite:\n\t\tcaseID = rollbackfault.SQLIndexPrefix64\n\tcase rollbackfault.FailBefore:\n\t\tcaseID = rollbackfault.SQLDBSyncBefore\n\tdefault:\n\t\treturn nil, errRollbackFaultScope\n\t}\n\tfrozen, problem := rollbackfault.SQLPlan(spec.Identity, caseID)\n\tif problem != rollbackfault.NoProblem || frozen.Spec() != spec {\n\t\treturn nil, errRollbackFaultScope\n\t}\n\tif tx == nil {\n\t\treturn nil, errRollbackFaultScope\n\t}\n\ttx.mu.Lock()\n\tthreePages := tx.originalPages == 3 && tx.header.PageCount == 3 && len(tx.dirty) == 3 && len(tx.dirty[1]) == 4096 && len(tx.dirty[2]) == 4096 && len(tx.dirty[3]) == 4096\n\ttx.mu.Unlock()\n\tif !threePages {\n\t\treturn nil, errRollbackFaultScope\n\t}\n\tstate := ManualRollbackSQLState(tx)\n\tif state.Groups != 1 || state.Pending != 0 || !state.HealthyTx || state.AdapterActive || state.AdapterUsed {\n\t\treturn nil, errRollbackFaultScope\n\t}\n\tscope, err := armRollbackFault(tx, plan)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn &manualRollbackSQLScope{scope: scope}, nil\n}\n\n// The caller must pass the untouched return from public sqlite.Tx.Commit in\n// its immediately following statement. This bridge never invokes Commit.\nfunc FinishManualRollbackSQLFault(scope *manualRollbackSQLScope, commitErr error) ManualRollbackSQLReport {\n\tif scope == nil {\n\t\treturn ManualRollbackSQLReport{Problem: errRollbackFaultScope}\n\t}\n\treport := finishRollbackFault(scope.scope, commitErr)\n\treturn ManualRollbackSQLReport{Observation: report.Observation, Events: report.Events, EventCount: report.EventCount, Diagnostics: report.Diagnostics, CommitError: report.CommitError, Problem: report.Problem}\n}\n\nfunc DisarmManualRollbackSQLFault(scope *manualRollbackSQLScope) error {\n\tif scope == nil {\n\t\treturn errRollbackFaultScope\n\t}\n\tstate := ManualRollbackSQLState(nil)\n\tif state.Groups != 0 || state.Pending != 0 {\n\t\treturn errRollbackFaultScope\n\t}\n\treturn disarmRollbackFault(scope.scope)\n}\n",
	rollbackFaultSQLNative:    "//go:build linux && manual_fault_rollback\n\npackage sqlite\n\n// SOURCE ONLY / REVIEW HOLD / PHYSICAL NOT_RUN. One fresh process, one case.\n// A private stable namespace is required; metadata rechecks are not protection\n// against hostile concurrent name replacement. Every input and output remains.\nimport (\n\t\"bytes\"\n\t\"context\"\n\t\"crypto/sha256\"\n\t\"encoding/binary\"\n\t\"encoding/hex\"\n\t\"encoding/json\"\n\t\"errors\"\n\t\"fmt\"\n\t\"io\"\n\t\"os\"\n\t\"path/filepath\"\n\t\"strings\"\n\t\"syscall\"\n\t\"testing\"\n\n\t\"github.com/ms-xiaosnow/gosqlite/internal/journal\"\n\t\"github.com/ms-xiaosnow/gosqlite/internal/pager\"\n\t\"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault\"\n)\n\ntype manualRollbackSQLOwner struct {\n\tfile     *os.File\n\tdb       *DB\n\trows     *Rows\n\tpath     string\n\tcloses   uint8\n\tcloseErr error\n}\n\ntype manualRollbackSQLLedger struct {\n\towners [256]manualRollbackSQLOwner\n\tcount  int\n}\n\nfunc (l *manualRollbackSQLLedger) reserve() (int, error) {\n\tif l.count == len(l.owners) {\n\t\treturn 0, fmt.Errorf(\"owner ledger full\")\n\t}\n\ti := l.count\n\tl.count++\n\treturn i, nil\n}\n\nfunc (l *manualRollbackSQLLedger) close(i int) error {\n\to := &l.owners[i]\n\tif o.closes != 0 {\n\t\treturn fmt.Errorf(\"duplicate close refused: %s\", o.path)\n\t}\n\tif o.file == nil && o.db == nil && o.rows == nil {\n\t\treturn nil\n\t}\n\t// Never turn a DB close into a hidden rollback/cleanup retry.\n\tif o.db != nil && (o.db.tx != nil || len(o.db.rows) != 0) {\n\t\treturn fmt.Errorf(\"DB has live transaction/Rows; retained without retry\")\n\t}\n\to.closes++\n\tif o.file != nil {\n\t\to.closeErr = o.file.Close()\n\t} else if o.rows != nil {\n\t\to.closeErr = o.rows.Close()\n\t} else {\n\t\to.closeErr = o.db.Close()\n\t}\n\treturn o.closeErr\n}\n\nfunc (l *manualRollbackSQLLedger) retired() bool {\n\tfor i := 0; i < l.count; i++ {\n\t\to := l.owners[i]\n\t\tif (o.file != nil || o.db != nil || o.rows != nil) && (o.closes != 1 || o.closeErr != nil) {\n\t\t\treturn false\n\t\t}\n\t}\n\treturn true\n}\n\nfunc (l *manualRollbackSQLLedger) finish(t *testing.T) {\n\t// Reporting only: every acquired owner is retired explicitly at its one\n\t// intended boundary. A failure must not trigger a deferred cleanup retry.\n\tfor i := 0; i < l.count; i++ {\n\t\to := l.owners[i]\n\t\tif (o.file != nil || o.db != nil || o.rows != nil) && (o.closes != 1 || o.closeErr != nil) {\n\t\t\tt.Errorf(\"INCOMPLETE: retained owner %d %s closes=%d error=%v; no retry\", i, o.path, o.closes, o.closeErr)\n\t\t}\n\t}\n}\n\ntype manualRollbackSQLBinding struct {\n\tSource             string\n\tTool               string\n\tBinary             string\n\tManifest           string\n\tCompileReceipt     string\n\tTrial              string\n\tScope              string\n\tCase               string\n\tToolPath           string\n\tSourceManifestPath string\n\tCompileReceiptPath string\n}\n\n// An external reviewed compile receipt is a bounded evidence input, not an\n// executable command or a self-authenticating approval. Its exact bytes must\n// have been reviewed independently and its hash supplied by that review.\ntype manualRollbackSQLCompile struct {\n\tSchema      string\n\tSource      string\n\tTool        string\n\tBinary      string\n\tManifest    string\n\tPackage     string\n\tTags        string\n\tCompileOnly bool\n\tFiles       uint16\n\tSourceTree  string\n}\n\ntype manualRollbackSQLConfig struct {\n\troot          string\n\trootInfo      os.FileInfo\n\tcaseID        rollbackfault.SQLCase\n\tbinding       manualRollbackSQLBinding\n\tbindingBytes  []byte\n\tsourceFiles   uint16\n\tledger        manualRollbackSQLLedger\n\tretained      [12]manualRollbackSQLSaved\n\tretainedCount int\n}\n\ntype manualRollbackSQLSaved struct {\n\tname string\n\tinfo os.FileInfo\n\thash [32]byte\n\tsize int64\n}\n\nfunc manualRollbackSQLHash(value string) ([32]byte, error) {\n\tvar out [32]byte\n\tif len(value) != 64 {\n\t\treturn out, fmt.Errorf(\"SHA-256 requires 64 lowercase hex bytes\")\n\t}\n\tdata, err := hex.DecodeString(value)\n\tif err != nil || hex.EncodeToString(data) != value {\n\t\treturn out, fmt.Errorf(\"invalid SHA-256\")\n\t}\n\tcopy(out[:], data)\n\tif out == ([32]byte{}) {\n\t\treturn out, fmt.Errorf(\"zero SHA-256\")\n\t}\n\treturn out, nil\n}\n\nfunc manualRollbackSQLNamespace(root string) (os.FileInfo, error) {\n\tif !filepath.IsAbs(root) || filepath.Clean(root) != root || root == \"/\" {\n\t\treturn nil, fmt.Errorf(\"absolute clean private root required\")\n\t}\n\tcurrent := string(filepath.Separator)\n\tfor _, component := range strings.Split(strings.TrimPrefix(root, \"/\"), \"/\") {\n\t\tcurrent = filepath.Join(current, component)\n\t\tinfo, err := os.Lstat(current)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tif info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {\n\t\t\treturn nil, fmt.Errorf(\"symlink/non-directory component: %s\", current)\n\t\t}\n\t}\n\tinfo, err := os.Lstat(root)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tmetadata, ok := info.Sys().(*syscall.Stat_t)\n\tif !ok || metadata.Uid != uint32(os.Getuid()) {\n\t\treturn nil, fmt.Errorf(\"private root must belong to the invoking user\")\n\t}\n\tif info.Mode().Perm() != 0700 {\n\t\treturn nil, fmt.Errorf(\"precreated private root must already have mode 0700; no permission changes are made\")\n\t}\n\treturn info, nil\n}\n\nfunc (c *manualRollbackSQLConfig) stable() error {\n\tinfo, err := manualRollbackSQLNamespace(c.root)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif !os.SameFile(info, c.rootInfo) {\n\t\treturn fmt.Errorf(\"root namespace changed; retain everything\")\n\t}\n\treturn nil\n}\n\nfunc manualRollbackSQLRead(l *manualRollbackSQLLedger, path string, limit int64) ([]byte, os.FileInfo, error) {\n\tbefore, err := os.Lstat(path)\n\tif err != nil {\n\t\treturn nil, nil, err\n\t}\n\tif !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > limit {\n\t\treturn nil, nil, fmt.Errorf(\"bounded regular input required: %s\", path)\n\t}\n\ti, err := l.reserve()\n\tif err != nil {\n\t\treturn nil, nil, err\n\t}\n\tf, openErr := os.Open(path)\n\tl.owners[i] = manualRollbackSQLOwner{file: f, path: path}\n\tif openErr != nil {\n\t\treturn nil, nil, errors.Join(openErr, l.close(i))\n\t}\n\tactual, statErr := f.Stat()\n\tif statErr != nil || !os.SameFile(before, actual) || actual.Size() != before.Size() {\n\t\treturn nil, nil, errors.Join(fmt.Errorf(\"input changed on open: %s\", path), statErr, l.close(i))\n\t}\n\tdata, readErr := io.ReadAll(io.LimitReader(f, limit+1))\n\tafter, afterErr := f.Stat()\n\tcloseErr := l.close(i)\n\tpathAfter, pathErr := os.Lstat(path)\n\tif err := errors.Join(readErr, afterErr, closeErr, pathErr); err != nil {\n\t\treturn nil, nil, err\n\t}\n\tif int64(len(data)) != before.Size() || !os.SameFile(before, after) || !os.SameFile(before, pathAfter) || after.Size() != before.Size() || pathAfter.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || !pathAfter.ModTime().Equal(before.ModTime()) {\n\t\treturn nil, nil, fmt.Errorf(\"input changed during bounded read: %s\", path)\n\t}\n\treturn data, before, nil\n}\n\nfunc manualRollbackSQLDecode(raw []byte, value any) error {\n\tdecoder := json.NewDecoder(bytes.NewReader(raw))\n\tdecoder.DisallowUnknownFields()\n\tif err := decoder.Decode(value); err != nil {\n\t\treturn err\n\t}\n\tvar tail any\n\tif decoder.Decode(&tail) != io.EOF {\n\t\treturn fmt.Errorf(\"trailing JSON value\")\n\t}\n\treturn nil\n}\n\nfunc manualRollbackSQLAdmit(c *manualRollbackSQLConfig) error {\n\t// Exact flags are intentional. No regex union, repeated test, subtest,\n\t// random order, competing test, timeout kill, or wrapper is admitted.\n\twant := map[string]string{\n\t\t\"-test.run\": \"^TestManualRollbackFaultSQLLedger$\", \"-test.count\": \"1\",\n\t\t\"-test.shuffle\": \"off\", \"-test.parallel\": \"1\", \"-test.timeout\": \"0\", \"-test.cpu\": \"1\",\n\t\t\"-test.v\": \"true\", // The unique successful terminal uses t.Logf.\n\t}\n\tseen := map[string]bool{}\n\tfor _, arg := range os.Args[1:] {\n\t\tkey, value, found := strings.Cut(arg, \"=\")\n\t\tif !found || seen[key] {\n\t\t\treturn fmt.Errorf(\"explicit unique equals-form test flags required\")\n\t\t}\n\t\tseen[key] = true\n\t\tif expected, ok := want[key]; !ok || value != expected {\n\t\t\treturn fmt.Errorf(\"unreviewed test flag: %s\", arg)\n\t\t}\n\t}\n\tfor key := range want {\n\t\tif !seen[key] {\n\t\t\treturn fmt.Errorf(\"missing exact flag: %s\", key)\n\t\t}\n\t}\n\tif os.Getenv(\"GOSQLITE_ROLLBACK_SQL_PRIVATE_ROOT_CONFIRMED\") != \"private_stable_namespace_retained\" {\n\t\treturn fmt.Errorf(\"private stable namespace and retention must be explicitly confirmed\")\n\t}\n\tc.root = os.Getenv(\"GOSQLITE_ROLLBACK_SQL_ROOT\")\n\tinfo, err := manualRollbackSQLNamespace(c.root)\n\tif err != nil {\n\t\treturn err\n\t}\n\tc.rootInfo = info\n\ti, err := c.ledger.reserve()\n\tif err != nil {\n\t\treturn err\n\t}\n\tdir, openErr := os.Open(c.root)\n\tc.ledger.owners[i] = manualRollbackSQLOwner{file: dir, path: c.root}\n\tif openErr != nil {\n\t\treturn errors.Join(openErr, c.ledger.close(i))\n\t}\n\tentries, readErr := dir.ReadDir(1)\n\tcloseErr := c.ledger.close(i)\n\tif readErr != io.EOF || len(entries) != 0 || closeErr != nil {\n\t\treturn errors.Join(fmt.Errorf(\"root must be empty before admission\"), readErr, closeErr)\n\t}\n\tbindingPath := os.Getenv(\"GOSQLITE_ROLLBACK_SQL_BINDING\")\n\tbindingHash, err := manualRollbackSQLHash(os.Getenv(\"GOSQLITE_ROLLBACK_SQL_BINDING_SHA256\"))\n\tif err != nil {\n\t\treturn err\n\t}\n\traw, _, err := manualRollbackSQLRead(&c.ledger, bindingPath, 65536)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif sha256.Sum256(raw) != bindingHash {\n\t\treturn fmt.Errorf(\"independently reviewed binding hash mismatch\")\n\t}\n\tif err := manualRollbackSQLDecode(raw, &c.binding); err != nil {\n\t\treturn err\n\t}\n\tc.bindingBytes = raw\n\tswitch c.binding.Case {\n\tcase \"control\":\n\t\tc.caseID = rollbackfault.SQLControl\n\tcase \"index-prefix-64\":\n\t\tc.caseID = rollbackfault.SQLIndexPrefix64\n\tcase \"db-sync-before\":\n\t\tc.caseID = rollbackfault.SQLDBSyncBefore\n\tdefault:\n\t\treturn fmt.Errorf(\"exactly one of three fixed cases required\")\n\t}\n\tmanifest, _, err := manualRollbackSQLRead(&c.ledger, c.binding.SourceManifestPath, 4<<20)\n\tif err != nil {\n\t\treturn err\n\t}\n\tmanifestHash, err := manualRollbackSQLHash(c.binding.Manifest)\n\tif err != nil || sha256.Sum256(manifest) != manifestHash || c.binding.Source != c.binding.Manifest {\n\t\treturn fmt.Errorf(\"frozen source inventory digest mismatch: %v\", err)\n\t}\n\tvar inventory struct {\n\t\tFiles map[string]struct {\n\t\t\tBytes   int64  `json:\"bytes\"`\n\t\t\tGitBlob string `json:\"git_blob\"`\n\t\t\tGitMode string `json:\"git_mode\"`\n\t\t\tSHA256  string `json:\"sha256\"`\n\t\t} `json:\"files\"`\n\t\tTree string `json:\"tree\"`\n\t}\n\tif err := manualRollbackSQLDecode(manifest, &inventory); err != nil {\n\t\treturn err\n\t}\n\tif len(inventory.Files) == 0 || len(inventory.Files) > 4096 || len(inventory.Tree) != 40 {\n\t\treturn fmt.Errorf(\"finite reviewed source inventory required\")\n\t}\n\tfor path, entry := range inventory.Files {\n\t\tif filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || strings.HasPrefix(path, \"../\") || entry.Bytes < 0 || (entry.GitMode != \"100644\" && entry.GitMode != \"100755\") || len(entry.GitBlob) != 40 {\n\t\t\treturn fmt.Errorf(\"invalid reviewed source inventory entry\")\n\t\t}\n\t\tif _, err := manualRollbackSQLHash(entry.SHA256); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\tc.sourceFiles = uint16(len(inventory.Files))\n\tcompileRaw, _, err := manualRollbackSQLRead(&c.ledger, c.binding.CompileReceiptPath, 65536)\n\tif err != nil {\n\t\treturn err\n\t}\n\tcompileHash, err := manualRollbackSQLHash(c.binding.CompileReceipt)\n\tif err != nil || sha256.Sum256(compileRaw) != compileHash {\n\t\treturn fmt.Errorf(\"independently reviewed compile receipt digest mismatch: %v\", err)\n\t}\n\tvar receipt manualRollbackSQLCompile\n\tif err := manualRollbackSQLDecode(compileRaw, &receipt); err != nil {\n\t\treturn err\n\t}\n\tif receipt.Schema != \"rollback-fault-sql-compile-v1\" || receipt.Source != c.binding.Source || receipt.Tool != c.binding.Tool || receipt.Binary != c.binding.Binary || receipt.Manifest != c.binding.Manifest || receipt.Package != \"./sqlite\" || receipt.Tags != \"manual_fault_rollback\" || !receipt.CompileOnly || receipt.Files != c.sourceFiles || receipt.SourceTree != inventory.Tree {\n\t\treturn fmt.Errorf(\"source-to-binary compile receipt does not bind this trial\")\n\t}\n\ttool, _, err := manualRollbackSQLRead(&c.ledger, c.binding.ToolPath, 128<<20)\n\tif err != nil {\n\t\treturn err\n\t}\n\ttoolHash, err := manualRollbackSQLHash(c.binding.Tool)\n\tif err != nil || sha256.Sum256(tool) != toolHash {\n\t\treturn fmt.Errorf(\"tool digest mismatch: %v\", err)\n\t}\n\texecutable, err := os.Executable()\n\tif err != nil {\n\t\treturn err\n\t}\n\timage, _, err := manualRollbackSQLRead(&c.ledger, executable, 128<<20)\n\tif err != nil {\n\t\treturn err\n\t}\n\tbinaryHash, err := manualRollbackSQLHash(c.binding.Binary)\n\tif err != nil || sha256.Sum256(image) != binaryHash {\n\t\treturn fmt.Errorf(\"actual executable digest mismatch: %v\", err)\n\t}\n\tif _, err := manualRollbackSQLHash(c.binding.Source); err != nil {\n\t\treturn err\n\t}\n\treturn c.stable()\n}\n\nfunc (c *manualRollbackSQLConfig) create(name string, data []byte, retain bool) (os.FileInfo, error) {\n\tif err := c.stable(); err != nil {\n\t\treturn nil, err\n\t}\n\tif filepath.Base(name) != name || name == \".\" || len(data) == 0 || len(data) > 1<<20 {\n\t\treturn nil, fmt.Errorf(\"fixed bounded basename required\")\n\t}\n\tpath := filepath.Join(c.root, name)\n\ti, err := c.ledger.reserve()\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tf, openErr := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)\n\tc.ledger.owners[i] = manualRollbackSQLOwner{file: f, path: path}\n\tif openErr != nil {\n\t\treturn nil, errors.Join(openErr, c.ledger.close(i))\n\t}\n\tinfo, statErr := f.Stat()\n\tif statErr != nil {\n\t\treturn nil, errors.Join(statErr, c.ledger.close(i))\n\t}\n\tfor n := 0; n < c.retainedCount; n++ {\n\t\tif os.SameFile(info, c.retained[n].info) {\n\t\t\treturn nil, errors.Join(fmt.Errorf(\"retained input alias\"), c.ledger.close(i))\n\t\t}\n\t}\n\tn, writeErr := f.Write(data)\n\tif writeErr == nil && n != len(data) {\n\t\twriteErr = io.ErrShortWrite\n\t}\n\tvar syncErr error\n\tif writeErr == nil {\n\t\tsyncErr = f.Sync()\n\t}\n\tcloseErr := c.ledger.close(i)\n\tif err := errors.Join(writeErr, syncErr, closeErr); err != nil {\n\t\treturn nil, err\n\t}\n\tactual, actualInfo, err := manualRollbackSQLRead(&c.ledger, path, int64(len(data)))\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tif !os.SameFile(info, actualInfo) || !bytes.Equal(actual, data) {\n\t\treturn nil, fmt.Errorf(\"created bytes or identity changed\")\n\t}\n\tif retain {\n\t\tif c.retainedCount == len(c.retained) {\n\t\t\treturn nil, fmt.Errorf(\"retained inventory full\")\n\t\t}\n\t\tc.retained[c.retainedCount] = manualRollbackSQLSaved{name: name, info: actualInfo, hash: sha256.Sum256(actual), size: int64(len(actual))}\n\t\tc.retainedCount++\n\t}\n\treturn actualInfo, c.stable()\n}\n\nfunc (c *manualRollbackSQLConfig) retainedUnchanged() error {\n\tif err := c.stable(); err != nil {\n\t\treturn err\n\t}\n\tfor i := 0; i < c.retainedCount; i++ {\n\t\tsaved := c.retained[i]\n\t\traw, info, err := manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, saved.name), saved.size)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tif !os.SameFile(info, saved.info) || info.Size() != saved.size || sha256.Sum256(raw) != saved.hash {\n\t\t\treturn fmt.Errorf(\"retained input changed: %s\", saved.name)\n\t\t}\n\t}\n\treturn nil\n}\n\n// The baseline is authored directly from the frozen independent old ledger.\n// There is no sqlite.Open(create), bootstrap SQL, initial Commit, fixture reader,\n// oracle, generator process, or inference from the damaged database.\nfunc manualRollbackSQLBaseline() ([12288]byte, error) {\n\tvar image [12288]byte\n\tcopy(image[:], \"SQLite format 3\\x00\")\n\tbinary.BigEndian.PutUint16(image[16:], 4096)\n\timage[18], image[19], image[21], image[22], image[23] = 1, 1, 64, 32, 32\n\tbinary.BigEndian.PutUint32(image[24:], 7)\n\tbinary.BigEndian.PutUint32(image[28:], 3)\n\tbinary.BigEndian.PutUint32(image[40:], 9)\n\tbinary.BigEndian.PutUint32(image[44:], 4)\n\tbinary.BigEndian.PutUint32(image[56:], 1)\n\tbinary.BigEndian.PutUint32(image[92:], 7)\n\tbinary.BigEndian.PutUint32(image[96:], 3045000)\n\tschema := [][]any{\n\t\t{\"table\", \"t\", \"t\", int64(2), \"CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT,v TEXT)\"},\n\t\t{\"index\", \"by_k\", \"t\", int64(3), \"CREATE INDEX by_k ON t(k)\"},\n\t}\n\ttable := [][]any{{nil, \"red\", \"one\"}, {nil, \"blue\", \"two\"}, {nil, \"red\", \"three\"}}\n\tindex := [][]any{{\"blue\", int64(2)}, {\"red\", int64(1)}, {\"red\", int64(3)}}\n\tif err := manualRollbackSQLLeaf(image[:4096], 100, 13, schema, []byte{1, 2}); err != nil {\n\t\treturn image, err\n\t}\n\tif err := manualRollbackSQLLeaf(image[4096:8192], 0, 13, table, []byte{1, 2, 3}); err != nil {\n\t\treturn image, err\n\t}\n\tif err := manualRollbackSQLLeaf(image[8192:], 0, 10, index, nil); err != nil {\n\t\treturn image, err\n\t}\n\treturn image, nil\n}\n\n// This deliberately supports only the bounded scalar record shapes above.\nfunc manualRollbackSQLRecord(values []any) ([]byte, error) {\n\theader := []byte{byte(len(values) + 1)}\n\tvar body []byte\n\tfor _, value := range values {\n\t\tswitch v := value.(type) {\n\t\tcase nil:\n\t\t\theader = append(header, 0)\n\t\tcase int64:\n\t\t\tif v < 1 || v > 4 {\n\t\t\t\treturn nil, fmt.Errorf(\"fixture integer outside frozen range\")\n\t\t\t}\n\t\t\theader = append(header, 1)\n\t\t\tbody = append(body, byte(v))\n\t\tcase string:\n\t\t\tif len(v) > 57 {\n\t\t\t\treturn nil, fmt.Errorf(\"fixture text outside single-byte serial bound\")\n\t\t\t}\n\t\t\theader = append(header, byte(13+2*len(v)))\n\t\t\tbody = append(body, v...)\n\t\tdefault:\n\t\t\treturn nil, fmt.Errorf(\"fixture scalar type outside fixed builder\")\n\t\t}\n\t}\n\trecord := append(header, body...)\n\tif len(record) >= 128 {\n\t\treturn nil, fmt.Errorf(\"fixture payload outside single-byte length bound\")\n\t}\n\treturn record, nil\n}\n\nfunc manualRollbackSQLLeaf(page []byte, header int, kind byte, rows [][]any, ids []byte) error {\n\tif len(page) != 4096 || len(rows) > 3 || (kind == 13 && len(ids) != len(rows)) || (kind != 10 && kind != 13) {\n\t\treturn fmt.Errorf(\"fixed leaf shape required\")\n\t}\n\tpage[header] = kind\n\tbinary.BigEndian.PutUint16(page[header+3:], uint16(len(rows)))\n\tend := len(page)\n\tfor i, values := range rows {\n\t\trecord, err := manualRollbackSQLRecord(values)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tcell := []byte{byte(len(record))}\n\t\tif kind == 13 {\n\t\t\tcell = append(cell, ids[i])\n\t\t}\n\t\tcell = append(cell, record...)\n\t\tend -= len(cell)\n\t\tif end < header+8+2*len(rows) {\n\t\t\treturn fmt.Errorf(\"fixed leaf overflow\")\n\t\t}\n\t\tcopy(page[end:], cell)\n\t\tbinary.BigEndian.PutUint16(page[header+8+2*i:], uint16(end))\n\t}\n\tbinary.BigEndian.PutUint16(page[header+5:], uint16(end))\n\treturn nil\n}\n\nfunc manualRollbackSQLRegistryEmpty() bool {\n\ts := pager.ManualRollbackSQLState(nil)\n\treturn s.Groups == 0 && s.Pending == 0 && !s.AdapterActive\n}\n\nfunc manualRollbackSQLOpen(c *manualRollbackSQLConfig, path string, readonly bool) (*DB, int, error) {\n\ti, err := c.ledger.reserve()\n\tif err != nil {\n\t\treturn nil, 0, err\n\t}\n\tdb, openErr := Open(path, Options{ReadOnly: readonly, NoCreate: true, PageSize: 4096, SectorSize: 4096, MaxTransactionPages: 3, MaxTransactionUndoBytes: 65536, MaxSortBytes: 65536})\n\tc.ledger.owners[i] = manualRollbackSQLOwner{db: db, path: path}\n\tif openErr != nil {\n\t\treturn db, i, errors.Join(openErr, c.ledger.close(i))\n\t}\n\tif db == nil || db.session != nil {\n\t\treturn db, i, fmt.Errorf(\"independent plain DB required\")\n\t}\n\treturn db, i, nil\n}\n\n// Retirement on a preparation error is allowed only before adapter ownership\n// transfer and only for the original healthy independent transaction. An\n// unknown, started or pending cleanup is retained, never retried by DB.Close.\nfunc manualRollbackSQLAbandon(tx *Tx, cause error) error {\n\tif tx == nil || tx.done {\n\t\treturn cause\n\t}\n\tstate := pager.ManualRollbackSQLState(tx.storage)\n\tif state.AdapterActive || state.AdapterUsed || state.Pending != 0 || !state.HealthyTx {\n\t\treturn cause\n\t}\n\treturn errors.Join(cause, tx.Rollback())\n}\n\nfunc manualRollbackSQLCommit(c *manualRollbackSQLConfig, ctx context.Context, path string, plan rollbackfault.Plan) (pager.ManualRollbackSQLReport, error) {\n\tif !manualRollbackSQLRegistryEmpty() {\n\t\treturn pager.ManualRollbackSQLReport{}, fmt.Errorf(\"preexisting owner obligations\")\n\t}\n\tdb, owner, err := manualRollbackSQLOpen(c, path, false)\n\tif err != nil {\n\t\treturn pager.ManualRollbackSQLReport{}, err\n\t}\n\ttx, beginErr := db.Begin(ctx, TxOptions{Immediate: true})\n\tif beginErr != nil {\n\t\treturn pager.ManualRollbackSQLReport{}, errors.Join(manualRollbackSQLAbandon(tx, beginErr), c.ledger.close(owner))\n\t}\n\tfor _, statement := range []string{\n\t\t\"UPDATE t SET k='green',v='ONE' WHERE id=1\",\n\t\t\"DELETE FROM t WHERE id=2\",\n\t\t\"INSERT INTO t(id,k,v) VALUES(4,'blue','four')\",\n\t} {\n\t\tresult, execErr := tx.Exec(ctx, statement)\n\t\tif execErr != nil {\n\t\t\treturn pager.ManualRollbackSQLReport{}, errors.Join(manualRollbackSQLAbandon(tx, execErr), c.ledger.close(owner))\n\t\t}\n\t\tcount, countErr := result.RowsAffected()\n\t\tif countErr != nil || count != 1 {\n\t\t\treturn pager.ManualRollbackSQLReport{}, errors.Join(manualRollbackSQLAbandon(tx, fmt.Errorf(\"DML affected %d: %v\", count, countErr)), c.ledger.close(owner))\n\t\t}\n\t}\n\tscope, armErr := pager.ArmManualRollbackSQLFault(tx.storage, plan)\n\tif armErr != nil {\n\t\treturn pager.ManualRollbackSQLReport{}, errors.Join(manualRollbackSQLAbandon(tx, armErr), c.ledger.close(owner))\n\t}\n\tcommitErr := tx.Commit(ctx)\n\treport := pager.FinishManualRollbackSQLFault(scope, commitErr)\n\t// Keep the original public error object. No storage.Commit bypass, second\n\t// Commit, Rollback, DB.Close or recovery may repair an incomplete result.\n\tif report.CommitError != commitErr || report.Problem != nil || report.Observation.Outcome != rollbackfault.AdapterObserved || report.Observation.TargetHits != 1 || report.Observation.Pass() || report.Observation.PhysicalValidated() || !tx.done || db.tx != nil || tx.activeRows != 0 {\n\t\treturn report, fmt.Errorf(\"INCOMPLETE: public Commit/adapter closure: %v; %v\", commitErr, report.Problem)\n\t}\n\tif plan.Spec().Action == rollbackfault.Pass {\n\t\tif commitErr != nil {\n\t\t\treturn report, fmt.Errorf(\"control public Commit: %w\", commitErr)\n\t\t}\n\t} else {\n\t\tvar publicError *Error\n\t\tif !errors.As(commitErr, &publicError) || publicError.Code != CodeIOErr || !errors.Is(commitErr, ErrRecoveryRequired) || errors.Is(commitErr, pager.ErrPlaintextCleanupPending) || errors.Is(commitErr, pager.ErrCommitUncertain) {\n\t\t\treturn report, fmt.Errorf(\"unqualified public Commit error: %v\", commitErr)\n\t\t}\n\t\tif plan.Spec().Action == rollbackfault.PrefixShortWrite && !errors.Is(commitErr, io.ErrShortWrite) {\n\t\t\treturn report, fmt.Errorf(\"original short-write cause lost\")\n\t\t}\n\t}\n\tif err := pager.DisarmManualRollbackSQLFault(scope); err != nil {\n\t\treturn report, err\n\t}\n\tif err := c.ledger.close(owner); err != nil {\n\t\treturn report, err\n\t}\n\tif !manualRollbackSQLRegistryEmpty() {\n\t\treturn report, fmt.Errorf(\"Commit owner retirement incomplete\")\n\t}\n\treturn report, nil\n}\n\nfunc manualRollbackSQLRecoverCopy(ctx context.Context, replayPath string) (pager.RecoveryResult, error) {\n\tif filepath.Base(replayPath) != \"replay.db\" || !manualRollbackSQLRegistryEmpty() {\n\t\treturn pager.RecoveryResult{}, fmt.Errorf(\"fresh replay path and empty registries required\")\n\t}\n\tresult, recoveryErr := pager.RecoverRollback(ctx, replayPath, pager.RecoveryOptions{Limits: journal.ReplayLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}})\n\tif recoveryErr != nil {\n\t\treturn result, recoveryErr\n\t}\n\tif !manualRollbackSQLRegistryEmpty() {\n\t\treturn result, fmt.Errorf(\"recovery owner retirement incomplete\")\n\t}\n\tif !result.Recovered || result.RestoredPages != 3 || result.OriginalPages != 3 {\n\t\treturn result, fmt.Errorf(\"exact three-page recovery required\")\n\t}\n\treturn result, nil\n}\nfunc manualRollbackSQLQuery(c *manualRollbackSQLConfig, ctx context.Context, db *DB, owner uint64, identity rollbackfault.Identity, query rollbackfault.SQLQuery) (rollbackfault.SQLQueryEvidence, error) {\n\tevidence := rollbackfault.SQLQueryEvidence{Identity: identity, Query: query, Owner: owner, SQL: rollbackfault.SQLQueryText(query)}\n\ti, err := c.ledger.reserve()\n\tif err != nil {\n\t\treturn evidence, err\n\t}\n\trows, queryErr := db.Query(ctx, evidence.SQL)\n\tc.ledger.owners[i] = manualRollbackSQLOwner{rows: rows, path: evidence.SQL}\n\tevidence.Iterator = uint64(i + 1000)\n\tif queryErr != nil {\n\t\treturn evidence, errors.Join(queryErr, c.ledger.close(i))\n\t}\n\tif rows == nil {\n\t\treturn evidence, fmt.Errorf(\"nil Rows with nil error\")\n\t}\n\tplan := rows.Plan()\n\tevidence.Plan = rollbackfault.SQLQueryPlan{Access: plan.Access, Table: plan.Table, Method: plan.Method, IndexCount: uint8(len(plan.Indexes)), FieldCount: uint8(len(plan.Fields)), Recheck: plan.Recheck, Reason: plan.Reason}\n\tif len(plan.Indexes) == 1 {\n\t\tevidence.Plan.Index = plan.Indexes[0]\n\t}\n\tif len(plan.Fields) == 1 {\n\t\tevidence.Plan.Field = plan.Fields[0]\n\t}\n\tif len(plan.Indexes) > 1 || len(plan.Fields) > 1 {\n\t\treturn evidence, errors.Join(fmt.Errorf(\"plan outside fixed SQL/index scope\"), c.ledger.close(i))\n\t}\n\tfor rows.Next() {\n\t\tif evidence.RowCount == 3 {\n\t\t\treturn evidence, errors.Join(fmt.Errorf(\"query exceeded fixed three-row bound\"), c.ledger.close(i))\n\t\t}\n\t\tvalues := rows.Values()\n\t\tif len(values) != 3 {\n\t\t\treturn evidence, errors.Join(fmt.Errorf(\"three typed columns required\"), c.ledger.close(i))\n\t\t}\n\t\tid, idOK := values[0].(int64)\n\t\tkey, keyOK := values[1].(string)\n\t\tvalue, valueOK := values[2].(string)\n\t\tif !idOK || !keyOK || !valueOK {\n\t\t\treturn evidence, errors.Join(fmt.Errorf(\"SQL row types changed\"), c.ledger.close(i))\n\t\t}\n\t\tevidence.Rows[evidence.RowCount] = rollbackfault.SQLRow{IDType: rollbackfault.SQLInteger, ID: id, KeyType: rollbackfault.SQLText, Key: key, ValueType: rollbackfault.SQLText, Value: value}\n\t\tevidence.RowCount++\n\t}\n\t// A single exhausted Next loop, then Err, then a single public Close. EOF\n\t// already retires the internal source; public Close never retries its owner.\n\trowsErr := rows.Err()\n\tevidence.ErrCalled = true\n\tevidence.EOF = rowsErr == nil\n\tstats := rows.Stats()\n\tevidence.Work = rollbackfault.SQLQueryWork{IndexKeysRead: stats.IndexKeysRead, IndexPageReads: stats.IndexPageReads, RowIDSeeks: stats.RowIDSeeks, Candidates: stats.Candidates, BaseRowFetches: stats.BaseRowFetches, ReturnedRows: stats.ReturnedRows}\n\tcloseErr := c.ledger.close(i)\n\tevidence.CloseCalls = c.ledger.owners[i].closes\n\tif err := errors.Join(rowsErr, closeErr); err != nil {\n\t\treturn evidence, err\n\t}\n\treturn evidence, nil\n}\n\nfunc manualRollbackSQLReadback(c *manualRollbackSQLConfig, ctx context.Context, path string, identity rollbackfault.Identity) (rollbackfault.SQLSnapshotEvidence, error) {\n\tevidence := rollbackfault.SQLSnapshotEvidence{Identity: identity}\n\tif !manualRollbackSQLRegistryEmpty() {\n\t\treturn evidence, fmt.Errorf(\"pre-read owner obligations\")\n\t}\n\tbefore, err := os.Lstat(path)\n\tif err != nil || before.Size() != 12288 {\n\t\treturn evidence, fmt.Errorf(\"three-page readback extent: %v\", err)\n\t}\n\tdb, owner, err := manualRollbackSQLOpen(c, path, true)\n\tif err != nil {\n\t\treturn evidence, err\n\t}\n\tevidence.Owner = uint64(owner + 1000)\n\tfor i := 0; i < rollbackfault.SQLQueryCount; i++ {\n\t\tquery, queryErr := manualRollbackSQLQuery(c, ctx, db, evidence.Owner, identity, rollbackfault.SQLQuery(i))\n\t\tevidence.Queries[i] = query\n\t\tif queryErr != nil {\n\t\t\treturn evidence, errors.Join(queryErr, c.ledger.close(owner))\n\t\t}\n\t}\n\tchecked, checkErr := db.Check(ctx, CheckOptions{MaxPages: 3, MaxPageReads: 512})\n\tevidence.Check = rollbackfault.SQLCheckEvidence{Identity: identity, Owner: evidence.Owner, Calls: 1, Pages: checked.Pages, FreePages: checked.FreePages, ReservedPages: checked.ReservedPages, Tables: checked.Tables, Indexes: checked.Indexes, ExtensionIndexes: checked.ExtensionIndexes, TableRows: checked.TableRows, IndexEntries: checked.IndexEntries}\n\tcloseErr := c.ledger.close(owner)\n\tevidence.CloseCalls = c.ledger.owners[owner].closes\n\tif err := errors.Join(checkErr, closeErr); err != nil {\n\t\treturn evidence, err\n\t}\n\tif checked.Scope != CheckScope || checked.PageReads == 0 || checked.PageReads > 512 || !manualRollbackSQLRegistryEmpty() {\n\t\treturn evidence, fmt.Errorf(\"Check scope or owner retirement incomplete\")\n\t}\n\tafter, err := os.Lstat(path)\n\tif err != nil || !os.SameFile(before, after) || after.Size() != 12288 {\n\t\treturn evidence, fmt.Errorf(\"readback file identity/extent changed: %v\", err)\n\t}\n\tevidence.OwnerState = rollbackfault.OwnerRetired\n\treturn evidence, nil\n}\n\nfunc (c *manualRollbackSQLConfig) retain(name string, expected os.FileInfo, limit int64) error {\n\tif c.retainedCount == len(c.retained) {\n\t\treturn fmt.Errorf(\"retained inventory full\")\n\t}\n\traw, info, err := manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, name), limit)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif expected != nil && !os.SameFile(expected, info) {\n\t\treturn fmt.Errorf(\"original artifact identity changed\")\n\t}\n\tc.retained[c.retainedCount] = manualRollbackSQLSaved{name: name, info: info, hash: sha256.Sum256(raw), size: info.Size()}\n\tc.retainedCount++\n\treturn nil\n}\n\nfunc manualRollbackSQLPageHashes(data []byte) ([3][32]byte, error) {\n\tvar result [3][32]byte\n\tif len(data) != 12288 {\n\t\treturn result, fmt.Errorf(\"exact three-page hash input required\")\n\t}\n\tfor i := range result {\n\t\tresult[i] = sha256.Sum256(data[i*4096 : (i+1)*4096])\n\t}\n\treturn result, nil\n}\nfunc manualRollbackSQLArtifact(c *manualRollbackSQLConfig, role rollbackfault.SQLArtifactRole) (rollbackfault.SQLArtifactStamp, error) {\n\tvar stamp rollbackfault.SQLArtifactStamp\n\tname := rollbackfault.SQLArtifactName(role)\n\tpath := filepath.Join(c.root, name)\n\tif _, err := os.Lstat(path); os.IsNotExist(err) {\n\t\treturn stamp, nil\n\t} else if err != nil {\n\t\treturn stamp, err\n\t}\n\tlimit := int64(12288)\n\tif role == rollbackfault.SQLOriginalJournalArtifact || role == rollbackfault.SQLJournalCopyArtifact || role == rollbackfault.SQLReplayJournalArtifact {\n\t\tlimit = 16408\n\t}\n\traw, info, err := manualRollbackSQLRead(&c.ledger, path, limit)\n\tif err != nil {\n\t\treturn stamp, err\n\t}\n\tstat, ok := info.Sys().(*syscall.Stat_t)\n\tif !ok || stat.Nlink != 1 || stat.Ino == 0 {\n\t\treturn stamp, fmt.Errorf(\"single-link concrete artifact identity required\")\n\t}\n\treturn rollbackfault.SQLArtifactStamp{Identity: name, Device: uint64(stat.Dev), Inode: stat.Ino, Exists: true, Bytes: info.Size(), SHA256: sha256.Sum256(raw)}, nil\n}\n\nfunc manualRollbackSQLCapture(c *manualRollbackSQLConfig, evidence *rollbackfault.SQLLedgerEvidence, after bool) error {\n\tif err := c.stable(); err != nil {\n\t\treturn err\n\t}\n\tfor i := 0; i < int(rollbackfault.SQLArtifactCount); i++ {\n\t\trole := rollbackfault.SQLArtifactRole(i)\n\t\tstamp, err := manualRollbackSQLArtifact(c, role)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tevidence.Artifacts[i].Role = role\n\t\tif after {\n\t\t\tevidence.Artifacts[i].After = stamp\n\t\t} else {\n\t\t\tevidence.Artifacts[i].Before = stamp\n\t\t}\n\t}\n\treturn nil\n}\n\nfunc manualRollbackSQLNamespaceContents(c *manualRollbackSQLConfig) error {\n\ti, err := c.ledger.reserve()\n\tif err != nil {\n\t\treturn err\n\t}\n\tdir, openErr := os.Open(c.root)\n\tc.ledger.owners[i] = manualRollbackSQLOwner{file: dir, path: c.root}\n\tif openErr != nil {\n\t\treturn errors.Join(openErr, c.ledger.close(i))\n\t}\n\tentries, readErr := dir.ReadDir(16)\n\tvar tail []os.DirEntry\n\ttailErr := readErr\n\tif readErr == nil {\n\t\ttail, tailErr = dir.ReadDir(1)\n\t}\n\tcloseErr := c.ledger.close(i)\n\tif (readErr != nil && readErr != io.EOF) || tailErr != io.EOF || len(tail) != 0 || len(entries) > 10 || closeErr != nil {\n\t\treturn errors.Join(fmt.Errorf(\"bounded namespace inventory incomplete\"), readErr, tailErr, closeErr)\n\t}\n\tfor _, entry := range entries {\n\t\tswitch entry.Name() {\n\t\tcase \"baseline.input.db\", \"binding.input.json\", \"trial.db\", \"trial.db-journal\", \"post-commit.input.db\", \"post-commit-journal.input.bin\", \"replay.db\", \"replay.db-journal\", \"commit-observation.json\", \"result.json\":\n\t\tdefault:\n\t\t\treturn fmt.Errorf(\"unexpected retained namespace member: %s\", entry.Name())\n\t\t}\n\t\tif !entry.Type().IsRegular() {\n\t\t\treturn fmt.Errorf(\"non-regular retained namespace member\")\n\t\t}\n\t}\n\treturn c.stable()\n}\n\nfunc TestManualRollbackFaultSQLLedger(t *testing.T) {\n\tif os.Getenv(\"GOSQLITE_MANUAL_FAULT_ROLLBACK\") != \"independently_authorized\" {\n\t\tt.Fatal(\"NOT_RUN: separate execution authorization, reviewed provenance and private retained root required\")\n\t}\n\tvar c manualRollbackSQLConfig\n\tdefer c.ledger.finish(t)\n\tif err := manualRollbackSQLAdmit(&c); err != nil {\n\t\tt.Fatalf(\"BLOCKED before fixture: %v\", err)\n\t}\n\tidentity := rollbackfault.Identity{Suite: \"rollback-fault-sql-v1\", Source: c.binding.Source, Tool: c.binding.Tool, Binary: c.binding.Binary, Manifest: c.binding.Manifest, Trial: c.binding.Trial, Scope: c.binding.Scope, Profile: rollbackfault.PlainRollback4096}\n\tplan, problem := rollbackfault.SQLPlan(identity, c.caseID)\n\tif problem != rollbackfault.NoProblem {\n\t\tt.Fatalf(\"BLOCKED before fixture: invalid fixed plan %v\", problem)\n\t}\n\tbaseline, err := manualRollbackSQLBaseline()\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tfrozenHashes, err := manualRollbackSQLPageHashes(baseline[:])\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif _, err := c.create(\"baseline.input.db\", baseline[:], true); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif _, err := c.create(\"binding.input.json\", c.bindingBytes, true); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tctx := context.Background()\n\tbefore, err := manualRollbackSQLReadback(&c, ctx, filepath.Join(c.root, \"baseline.input.db\"), identity)\n\tif err != nil {\n\t\tt.Fatalf(\"baseline readback: %v\", err)\n\t}\n\tif problem := rollbackfault.ValidateSQLSnapshot(before, identity, rollbackfault.SQLOld); problem != rollbackfault.SQLProblemNone {\n\t\tt.Fatalf(\"baseline independent ledger rejected before fault: %v\", problem)\n\t}\n\tif err := c.retainedUnchanged(); err != nil {\n\t\tt.Fatal(err)\n\t}\n\ttrialInfo, err := c.create(\"trial.db\", baseline[:], false)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\treport, commitCheckErr := manualRollbackSQLCommit(&c, ctx, filepath.Join(c.root, \"trial.db\"), plan)\n\t// Retain the untouched public error text and all original diagnostics even\n\t// when the trial must stop before copying/recovery. JSON is supplementary\n\t// evidence; error identity checks occurred directly on the original object.\n\tvar diagnosticTexts [rollbackfault.MaxEvents]string\n\tfor i, diagnostic := range report.Diagnostics {\n\t\tif diagnostic != nil {\n\t\t\tdiagnosticTexts[i] = diagnostic.Error()\n\t\t}\n\t}\n\tcommitText := \"\"\n\tif report.CommitError != nil {\n\t\tcommitText = report.CommitError.Error()\n\t}\n\tcommitRecord, marshalErr := json.MarshalIndent(struct {\n\t\tIdentity    rollbackfault.Identity\n\t\tPlan        rollbackfault.PlanSpec\n\t\tObservation rollbackfault.Observation\n\t\tEvents      [rollbackfault.MaxEvents]rollbackfault.Event\n\t\tEventCount  uint8\n\t\tCommitError string\n\t\tDiagnostics [rollbackfault.MaxEvents]string\n\t}{identity, plan.Spec(), report.Observation, report.Events, report.EventCount, commitText, diagnosticTexts}, \"\", \"  \")\n\tif marshalErr != nil {\n\t\tt.Fatal(marshalErr)\n\t}\n\tif _, err := c.create(\"commit-observation.json\", commitRecord, true); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif commitCheckErr != nil {\n\t\tt.Fatalf(\"INCOMPLETE: original Commit=%v; adapter=%+v; problem=%v; all inputs retained\", report.CommitError, report.Observation, commitCheckErr)\n\t}\n\treceipt, receiptProblem := rollbackfault.NewReceipt(plan)\n\tif receiptProblem != rollbackfault.NoProblem {\n\t\tt.Fatal(receiptProblem)\n\t}\n\tfor i := uint8(0); i < report.EventCount; i++ {\n\t\tif problem := receipt.Append(report.Events[i]); problem != rollbackfault.NoProblem {\n\t\t\tt.Fatal(problem)\n\t\t}\n\t}\n\tif receipt.Seal() != report.Observation {\n\t\tt.Fatal(\"adapter transcript/observation mismatch\")\n\t}\n\tevidence := rollbackfault.SQLLedgerEvidence{Identity: identity, Case: c.caseID, Before: before}\n\tevidence.Lifecycle = rollbackfault.SQLLifecycleEvidence{ArmCalls: 1, CommitCalls: 1, FinishCalls: 1, DisarmCalls: 1, WriterOpens: 1, ReopenCalls: 1, ReopenReadOnly: true, WriterRetiredBeforeRecovery: true, RecoveryFinishedBeforeReopen: true, CommitCapturedUnchanged: true}\n\tvar publicError *Error\n\tevidence.Lifecycle.PublicCodeIOErr = errors.As(report.CommitError, &publicError) && publicError.Code == CodeIOErr\n\tevidence.Lifecycle.PublicRecoveryRequired = errors.Is(report.CommitError, ErrRecoveryRequired)\n\tevidence.Lifecycle.PublicShortWrite = errors.Is(report.CommitError, io.ErrShortWrite)\n\tpost, postInfo, err := manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, \"trial.db\"), 12288)\n\tif err != nil || len(post) != 12288 || !os.SameFile(trialInfo, postInfo) {\n\t\tt.Fatalf(\"post-Commit extent/identity: %v\", err)\n\t}\n\tif c.caseID == rollbackfault.SQLIndexPrefix64 && !bytes.Equal(post[8192+64:], baseline[8192+64:]) {\n\t\tt.Fatal(\"index short write changed bytes beyond the frozen 64-byte prefix\")\n\t}\n\tif err := c.retain(\"trial.db\", trialInfo, 12288); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif _, err := c.create(\"post-commit.input.db\", post, true); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tvar journalBytes []byte\n\tif c.caseID == rollbackfault.SQLControl {\n\t\tif _, err := os.Lstat(filepath.Join(c.root, \"trial.db-journal\")); !os.IsNotExist(err) {\n\t\t\tt.Fatalf(\"control journal must be absent: %v\", err)\n\t\t}\n\t} else {\n\t\tjournalBytes, _, err = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, \"trial.db-journal\"), 16408)\n\t\tif err != nil || len(journalBytes) != 16408 {\n\t\t\tt.Fatalf(\"exact original three-record journal: %v\", err)\n\t\t}\n\t\tif err := c.retain(\"trial.db-journal\", nil, 16408); err != nil {\n\t\t\tt.Fatal(err)\n\t\t}\n\t\tif _, err := c.create(\"post-commit-journal.input.bin\", journalBytes, true); err != nil {\n\t\t\tt.Fatal(err)\n\t\t}\n\t}\n\treplayInfo, err := c.create(\"replay.db\", post, false)\n\tif err != nil || os.SameFile(trialInfo, replayInfo) {\n\t\tt.Fatalf(\"exclusive fresh replay: %v\", err)\n\t}\n\tif c.caseID != rollbackfault.SQLControl {\n\t\tif _, err := c.create(\"replay.db-journal\", journalBytes, false); err != nil {\n\t\t\tt.Fatal(err)\n\t\t}\n\t}\n\tif err := manualRollbackSQLCapture(&c, &evidence, false); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tfor i, artifact := range evidence.Artifacts {\n\t\tfor j := 0; j < i; j++ {\n\t\t\tother := evidence.Artifacts[j].Before\n\t\t\tif artifact.Before.Exists && other.Exists && artifact.Before.Device == other.Device && artifact.Before.Inode == other.Inode {\n\t\t\t\tt.Fatal(\"replay or retained artifact aliases an original\")\n\t\t\t}\n\t\t}\n\t}\n\tif err := manualRollbackSQLNamespaceContents(&c); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif err := c.retainedUnchanged(); err != nil {\n\t\tt.Fatal(err)\n\t}\n\toldBefore, _, err := manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, \"baseline.input.db\"), 12288)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tevidence.Originals.Frozen = frozenHashes\n\tevidence.Originals.Before, err = manualRollbackSQLPageHashes(oldBefore)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tevidence.Originals.BeforeBytes = int64(len(oldBefore))\n\tevidence.Recovery = rollbackfault.SQLRecoveryEvidence{Identity: identity, OwnersRetired: manualRollbackSQLRegistryEmpty()}\n\tif c.caseID != rollbackfault.SQLControl {\n\t\tresult, recoveryErr := manualRollbackSQLRecoverCopy(ctx, filepath.Join(c.root, \"replay.db\"))\n\t\tevidence.Recovery.Calls++\n\t\tif recoveryErr != nil {\n\t\t\tt.Fatalf(\"INCOMPLETE: recovery failed; all originals retained: %v\", recoveryErr)\n\t\t}\n\t\tevidence.Recovery.Limits = rollbackfault.SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}\n\t\tevidence.Recovery.JournalPresentBefore = evidence.Artifacts[rollbackfault.SQLReplayJournalArtifact].Before.Exists\n\t\tevidence.Recovery.Replayed, evidence.Recovery.ReplayedPages, evidence.Recovery.OriginalPages = result.Recovered, result.RestoredPages, result.OriginalPages\n\t\tevidence.Recovery.OwnersRetired = manualRollbackSQLRegistryEmpty()\n\t}\n\tif _, err := os.Lstat(filepath.Join(c.root, \"replay.db-journal\")); !os.IsNotExist(err) {\n\t\tt.Fatalf(\"replay journal not absent after control/recovery: %v\", err)\n\t}\n\tif err := c.retainedUnchanged(); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tafter, err := manualRollbackSQLReadback(&c, ctx, filepath.Join(c.root, \"replay.db\"), identity)\n\tif err != nil {\n\t\tt.Fatalf(\"fresh read-only SQL/index readback: %v\", err)\n\t}\n\tevidence.After = after\n\twant := rollbackfault.SQLOld\n\tif c.caseID == rollbackfault.SQLControl {\n\t\twant = rollbackfault.SQLNew\n\t}\n\tif problem := rollbackfault.ValidateSQLSnapshot(after, identity, want); problem != rollbackfault.SQLProblemNone {\n\t\tt.Fatalf(\"independent full SQL/index ledger rejected: %v\", problem)\n\t}\n\tif err := manualRollbackSQLCapture(&c, &evidence, true); err != nil {\n\t\tt.Fatal(err)\n\t}\n\toldAfter, _, err := manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, \"baseline.input.db\"), 12288)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tevidence.Originals.After, err = manualRollbackSQLPageHashes(oldAfter)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tevidence.Originals.AfterBytes = int64(len(oldAfter))\n\tif err := c.retainedUnchanged(); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif err := manualRollbackSQLNamespaceContents(&c); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif !c.ledger.retired() || !manualRollbackSQLRegistryEmpty() {\n\t\tt.Fatal(\"INCOMPLETE: terminal owner retirement\")\n\t}\n\tobservation := rollbackfault.ObserveSQLLedger(plan, receipt, evidence)\n\tif !observation.EvidenceConsistent || !observation.ScenarioObserved || observation.Problem != rollbackfault.SQLProblemNone || observation.PhysicalValidated() || observation.Pass() {\n\t\tt.Fatalf(\"SQL evidence inconsistent: %+v\", observation)\n\t}\n\tterminal, err := json.MarshalIndent(struct {\n\t\tSchema         string\n\t\tEvidence       rollbackfault.SQLLedgerEvidence\n\t\tObservation    rollbackfault.SQLObservation\n\t\tSourceFiles    uint16\n\t\tCompileReceipt string\n\t}{\"rollback-fault-sql-observed-v1\", evidence, observation, c.sourceFiles, c.binding.CompileReceipt}, \"\", \"  \")\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif _, err := c.create(\"result.json\", terminal, false); err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif !c.ledger.retired() || !manualRollbackSQLRegistryEmpty() {\n\t\tt.Fatal(\"INCOMPLETE: terminal file owner retirement\")\n\t}\n\tt.Logf(\"ROLLBACK_SQL_SCENARIO_OBSERVED case=%s trial=%s source=%s binary=%s physical_validated=false pass=false\", c.binding.Case, c.binding.Trial, c.binding.Source, c.binding.Binary)\n}\n",
	rollbackFaultSQLModel:     "package rollbackfault\n\n// SOURCE ONLY: this finite SQL ledger contract performs no SQL, recovery, file\n// I/O, native call, descriptor operation, hashing, or physical validation. Its\n// observations compare caller-supplied evidence with independently frozen\n// expectations. Even completely fabricated but consistent evidence cannot\n// establish a physical durability or acceptance result.\n\nconst (\n\tSQLPageSize      = 4096\n\tSQLPageCount     = 3\n\tSQLDatabaseBytes = 12288\n\tSQLJournalBytes  = 16408\n\tSQLQueryCount    = 7\n)\n\ntype SQLCase uint8\n\nconst (\n\tSQLCaseUnknown SQLCase = iota\n\tSQLControl\n\tSQLIndexPrefix64\n\tSQLDBSyncBefore\n)\n\ntype SQLState uint8\n\nconst (\n\tSQLStateUnknown SQLState = iota\n\tSQLOld\n\tSQLNew\n)\n\ntype SQLValueType uint8\n\nconst (\n\tSQLTypeUnknown SQLValueType = iota\n\tSQLInteger\n\tSQLText\n)\n\n// Explicit type tags prevent stringified integers, NULL, float64, and byte\n// slices from being counted as the required int64/string/string SQL values.\ntype SQLRow struct {\n\tIDType    SQLValueType\n\tID        int64\n\tKeyType   SQLValueType\n\tKey       string\n\tValueType SQLValueType\n\tValue     string\n}\n\ntype SQLQuery uint8\n\nconst (\n\tSQLAllRows SQLQuery = iota\n\tSQLBlueIndex\n\tSQLBlueScan\n\tSQLRedIndex\n\tSQLRedScan\n\tSQLGreenIndex\n\tSQLGreenScan\n)\n\ntype SQLQueryPlan struct {\n\tAccess     string\n\tTable      string\n\tMethod     string\n\tIndex      string\n\tIndexCount uint8\n\tField      string\n\tFieldCount uint8\n\tRecheck    bool\n\tReason     string\n}\n\ntype SQLQueryWork struct {\n\tIndexKeysRead  int64\n\tIndexPageReads int64\n\tRowIDSeeks     int64\n\tCandidates     int64\n\tBaseRowFetches int64\n\tReturnedRows   int64\n}\n\ntype SQLQueryEvidence struct {\n\tIdentity        Identity\n\tQuery           SQLQuery\n\tOwner           uint64\n\tIterator        uint64\n\tSQL             string\n\tRows            [3]SQLRow\n\tRowCount        uint8\n\tPlan            SQLQueryPlan\n\tWork            SQLQueryWork\n\tEOF             bool\n\tErrCalled       bool\n\tError           ErrorKind\n\tErrorToken      uint64\n\tCloseCalls      uint8\n\tCloseError      ErrorKind\n\tCloseErrorToken uint64\n}\n\ntype SQLCheckEvidence struct {\n\tIdentity         Identity\n\tOwner            uint64\n\tCalls            uint8\n\tError            ErrorKind\n\tErrorToken       uint64\n\tPages            uint32\n\tFreePages        uint32\n\tReservedPages    uint32\n\tTables           int\n\tIndexes          int\n\tExtensionIndexes int\n\tTableRows        uint64\n\tIndexEntries     uint64\n}\n\ntype SQLSnapshotEvidence struct {\n\tIdentity        Identity\n\tOwner           uint64\n\tQueries         [SQLQueryCount]SQLQueryEvidence\n\tCheck           SQLCheckEvidence\n\tCloseCalls      uint8\n\tCloseError      ErrorKind\n\tCloseErrorToken uint64\n\tOwnerState      OwnerState\n}\n\n// Frozen is taken from the independently prepared original artifact before any\n// fault trial. Before/After hash that same original artifact, never the working\n// database or recovered output. Hashes are evidence labels, not verified here.\ntype SQLOriginalEvidence struct {\n\tFrozen      [SQLPageCount][32]byte\n\tBefore      [SQLPageCount][32]byte\n\tAfter       [SQLPageCount][32]byte\n\tBeforeBytes int64\n\tAfterBytes  int64\n}\n\ntype SQLRecoveryLimits struct {\n\tMaxJournalBytes  uint64\n\tMaxRecords       uint32\n\tMaxDatabaseBytes uint64\n}\n\ntype SQLRecoveryEvidence struct {\n\tIdentity             Identity\n\tCalls                uint8\n\tError                ErrorKind\n\tErrorToken           uint64\n\tLimits               SQLRecoveryLimits\n\tJournalPresentBefore bool\n\tJournalPresentAfter  bool\n\tReplayed             bool\n\tReplayedPages        uint32\n\tOriginalPages        uint32\n\tOwnersRetired        bool\n}\n\ntype SQLArtifactRole uint8\n\nconst (\n\tSQLBaselineArtifact SQLArtifactRole = iota\n\tSQLTrialArtifact\n\tSQLOriginalJournalArtifact\n\tSQLPostCopyArtifact\n\tSQLJournalCopyArtifact\n\tSQLReplayArtifact\n\tSQLReplayJournalArtifact\n\tSQLArtifactCount\n)\n\n// Identity is a bounded artifact label. Device/Inode are the stat identity of\n// the named artifact, not a descriptor or native-owner capability. The model\n// neither stats a path nor hashes bytes; it only cross-checks these observations.\ntype SQLArtifactStamp struct {\n\tIdentity string\n\tDevice   uint64\n\tInode    uint64\n\tExists   bool\n\tBytes    int64\n\tSHA256   [32]byte\n}\n\ntype SQLArtifactEvidence struct {\n\tRole   SQLArtifactRole\n\tBefore SQLArtifactStamp\n\tAfter  SQLArtifactStamp\n}\n\n// Counts are observations, not instructions to close or retry anything. A\n// failed Commit must be followed by retired writer owners, explicit recovery,\n// and a fresh read-only open, without a second Commit or Rollback attempt.\ntype SQLLifecycleEvidence struct {\n\tArmCalls                     uint8\n\tCommitCalls                  uint8\n\tFinishCalls                  uint8\n\tDisarmCalls                  uint8\n\tRollbackCalls                uint8\n\tWriterOpens                  uint8\n\tReopenCalls                  uint8\n\tReopenReadOnly               bool\n\tWriterRetiredBeforeRecovery  bool\n\tRecoveryFinishedBeforeReopen bool\n\tCommitCapturedUnchanged      bool\n\tPublicCodeIOErr              bool\n\tPublicRecoveryRequired       bool\n\tPublicShortWrite             bool\n}\n\ntype SQLLedgerEvidence struct {\n\tIdentity      Identity\n\tCase          SQLCase\n\tBefore        SQLSnapshotEvidence\n\tAfter         SQLSnapshotEvidence\n\tOriginals     SQLOriginalEvidence\n\tArtifacts     [SQLArtifactCount]SQLArtifactEvidence\n\tRecovery      SQLRecoveryEvidence\n\tLifecycle     SQLLifecycleEvidence\n\tPhysicalClaim bool\n}\n\ntype SQLProblem uint8\n\nconst (\n\tSQLProblemNone SQLProblem = iota\n\tSQLProblemPlan\n\tSQLProblemReceipt\n\tSQLProblemIdentity\n\tSQLProblemRows\n\tSQLProblemQueryPlan\n\tSQLProblemIndexWork\n\tSQLProblemEOF\n\tSQLProblemClose\n\tSQLProblemCheck\n\tSQLProblemOriginalHashes\n\tSQLProblemRecovery\n\tSQLProblemLifecycle\n\tSQLProblemPhysicalClaim\n\tSQLProblemArtifacts\n)\n\ntype SQLObservation struct {\n\tEvidenceConsistent bool\n\tScenarioObserved   bool\n\tProblem            SQLProblem\n\tAdapter            Observation\n}\n\nfunc (SQLObservation) PhysicalValidated() bool { return false }\nfunc (SQLObservation) Pass() bool              { return false }\n\n// SQLLedgerRows contains literal expectations, independent of the adapter,\n// Commit error, database bytes, query results, and recovery output.\nfunc SQLLedgerRows(state SQLState) [3]SQLRow {\n\tswitch state {\n\tcase SQLOld:\n\t\treturn [3]SQLRow{\n\t\t\t{SQLInteger, 1, SQLText, \"red\", SQLText, \"one\"},\n\t\t\t{SQLInteger, 2, SQLText, \"blue\", SQLText, \"two\"},\n\t\t\t{SQLInteger, 3, SQLText, \"red\", SQLText, \"three\"},\n\t\t}\n\tcase SQLNew:\n\t\treturn [3]SQLRow{\n\t\t\t{SQLInteger, 1, SQLText, \"green\", SQLText, \"ONE\"},\n\t\t\t{SQLInteger, 3, SQLText, \"red\", SQLText, \"three\"},\n\t\t\t{SQLInteger, 4, SQLText, \"blue\", SQLText, \"four\"},\n\t\t}\n\tdefault:\n\t\treturn [3]SQLRow{}\n\t}\n}\n\nfunc SQLQueryText(query SQLQuery) string {\n\tswitch query {\n\tcase SQLAllRows:\n\t\treturn \"SELECT id,k,v FROM t NOT INDEXED ORDER BY id\"\n\tcase SQLBlueIndex:\n\t\treturn \"SELECT id,k,v FROM t WHERE k='blue' ORDER BY id\"\n\tcase SQLBlueScan:\n\t\treturn \"SELECT id,k,v FROM t NOT INDEXED WHERE k='blue' ORDER BY id\"\n\tcase SQLRedIndex:\n\t\treturn \"SELECT id,k,v FROM t WHERE k='red' ORDER BY id\"\n\tcase SQLRedScan:\n\t\treturn \"SELECT id,k,v FROM t NOT INDEXED WHERE k='red' ORDER BY id\"\n\tcase SQLGreenIndex:\n\t\treturn \"SELECT id,k,v FROM t WHERE k='green' ORDER BY id\"\n\tcase SQLGreenScan:\n\t\treturn \"SELECT id,k,v FROM t NOT INDEXED WHERE k='green' ORDER BY id\"\n\tdefault:\n\t\treturn \"\"\n\t}\n}\n\n// SQLPlan freezes page roles as schema/table/index by offset 0/4096/8192.\n// This function only returns a value; it cannot arm an adapter or open a file.\nfunc SQLPlan(identity Identity, which SQLCase) (Plan, Problem) {\n\tspec := PlanSpec{\n\t\tIdentity:  identity,\n\t\tMainOwner: 11, JournalOwner: 12, DirectoryOwner: 13, CleanupOwner: 14,\n\t\tOriginalPages: 3, DirtyPages: 3, PageSize: SQLPageSize, SectorSize: SQLPageSize,\n\t\tAction: Pass, ExpectedCommit: CommitSucceeded,\n\t\tCalls: [MaxCalls]Call{\n\t\t\t{Point: JournalCreate, Occurrence: 1, Role: Journal, Owner: 12},\n\t\t\t{Point: JournalInvalid, Occurrence: 1, Role: Journal, Owner: 12, Length: 4096},\n\t\t\t{Point: DBOriginalRead, Occurrence: 1, Role: Database, Owner: 11, Offset: 0, Length: 4096},\n\t\t\t{Point: JournalRecord, Occurrence: 1, Role: Journal, Owner: 12, Offset: 4096, Length: 4104},\n\t\t\t{Point: DBOriginalRead, Occurrence: 2, Role: Database, Owner: 11, Offset: 4096, Length: 4096},\n\t\t\t{Point: JournalRecord, Occurrence: 2, Role: Journal, Owner: 12, Offset: 8200, Length: 4104},\n\t\t\t{Point: DBOriginalRead, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192, Length: 4096},\n\t\t\t{Point: JournalRecord, Occurrence: 3, Role: Journal, Owner: 12, Offset: 12304, Length: 4104},\n\t\t\t{Point: JournalBodySync, Occurrence: 1, Role: Journal, Owner: 12},\n\t\t\t{Point: JournalValidHeader, Occurrence: 1, Role: Journal, Owner: 12, Length: 12},\n\t\t\t{Point: JournalHeaderSync, Occurrence: 1, Role: Journal, Owner: 12},\n\t\t\t{Point: PublishDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},\n\t\t\t{Point: DBPage, Occurrence: 1, Role: Database, Owner: 11, Offset: 0, Length: 4096},\n\t\t\t{Point: DBPage, Occurrence: 2, Role: Database, Owner: 11, Offset: 4096, Length: 4096},\n\t\t\t{Point: DBPage, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192, Length: 4096},\n\t\t\t{Point: DBSync, Occurrence: 1, Role: Database, Owner: 11},\n\t\t\t{Point: JournalUnlink, Occurrence: 1, Role: Journal, Owner: 12},\n\t\t\t{Point: FinalDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},\n\t\t},\n\t\tCallCount: 18,\n\t}\n\tspec.Target = spec.Calls[14]\n\tswitch which {\n\tcase SQLControl:\n\tcase SQLIndexPrefix64:\n\t\tspec.Action, spec.Prefix, spec.PrefixError = PrefixShortWrite, 64, false\n\t\tspec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 15\n\tcase SQLDBSyncBefore:\n\t\tspec.Target, spec.Action = spec.Calls[15], FailBefore\n\t\tspec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 16\n\tdefault:\n\t\treturn Plan{}, ProblemTarget\n\t}\n\tfor i := int(spec.CallCount); i < MaxCalls; i++ {\n\t\tspec.Calls[i] = Call{}\n\t}\n\treturn NewPlan(spec)\n}\n\n// ObserveSQLLedger rechecks the actual bounded Receipt value; a caller cannot\n// satisfy it by manufacturing an exported Observation with AdapterObserved.\n// All input is read by value and none is retained in the returned observation.\nfunc ObserveSQLLedger(plan Plan, receipt Receipt, evidence SQLLedgerEvidence) SQLObservation {\n\tresult := SQLObservation{}\n\tif evidence.PhysicalClaim {\n\t\tresult.Problem = SQLProblemPhysicalClaim\n\t\treturn result\n\t}\n\texpectedPlan, problem := SQLPlan(evidence.Identity, evidence.Case)\n\tif problem != ProblemNone || !plan.valid || plan.spec != expectedPlan.spec {\n\t\tresult.Problem = SQLProblemPlan\n\t\treturn result\n\t}\n\tif !receipt.plan.valid || receipt.plan.spec != plan.spec {\n\t\tresult.Problem = SQLProblemReceipt\n\t\treturn result\n\t}\n\tresult.Adapter = receipt.Seal()\n\tif result.Adapter.Outcome != AdapterObserved || result.Adapter.Problem != ProblemNone || result.Adapter.TargetHits != 1 {\n\t\tresult.Problem = SQLProblemReceipt\n\t\treturn result\n\t}\n\tif evidence.Identity != plan.spec.Identity || evidence.Before.Identity != evidence.Identity || evidence.After.Identity != evidence.Identity || evidence.Recovery.Identity != evidence.Identity {\n\t\tresult.Problem = SQLProblemIdentity\n\t\treturn result\n\t}\n\tlifecycle := SQLLifecycleEvidence{ArmCalls: 1, CommitCalls: 1, FinishCalls: 1, DisarmCalls: 1, WriterOpens: 1, ReopenCalls: 1, ReopenReadOnly: true, WriterRetiredBeforeRecovery: true, RecoveryFinishedBeforeReopen: true, CommitCapturedUnchanged: true}\n\tif evidence.Case != SQLControl {\n\t\tlifecycle.PublicCodeIOErr, lifecycle.PublicRecoveryRequired = true, true\n\t\tlifecycle.PublicShortWrite = evidence.Case == SQLIndexPrefix64\n\t}\n\tif evidence.Lifecycle != lifecycle {\n\t\tresult.Problem = SQLProblemLifecycle\n\t\treturn result\n\t}\n\tif !validSQLReaderOwners(plan.spec, evidence.Before.Owner, evidence.After.Owner) {\n\t\tresult.Problem = SQLProblemLifecycle\n\t\treturn result\n\t}\n\tif result.Problem = validateSQLSnapshot(evidence.Before, evidence.Identity, SQLOld); result.Problem != SQLProblemNone {\n\t\treturn result\n\t}\n\tstate := SQLOld\n\tif evidence.Case == SQLControl {\n\t\tstate = SQLNew\n\t}\n\tif result.Problem = validateSQLSnapshot(evidence.After, evidence.Identity, state); result.Problem != SQLProblemNone {\n\t\treturn result\n\t}\n\t// Iterator identities must not be re-used across the independent before and\n\t// after opens, even if all seven query texts and rows happen to agree.\n\tfor _, before := range evidence.Before.Queries {\n\t\tfor _, after := range evidence.After.Queries {\n\t\t\tif before.Iterator == after.Iterator {\n\t\t\t\tresult.Problem = SQLProblemLifecycle\n\t\t\t\treturn result\n\t\t\t}\n\t\t}\n\t}\n\toriginal := evidence.Originals\n\tif original.BeforeBytes != SQLDatabaseBytes || original.AfterBytes != SQLDatabaseBytes || original.Frozen != original.Before || original.Frozen != original.After {\n\t\tresult.Problem = SQLProblemOriginalHashes\n\t\treturn result\n\t}\n\tfor _, digest := range original.Frozen {\n\t\tif digest == ([32]byte{}) {\n\t\t\tresult.Problem = SQLProblemOriginalHashes\n\t\t\treturn result\n\t\t}\n\t}\n\tif !validSQLRecovery(evidence.Case, evidence.Recovery) {\n\t\tresult.Problem = SQLProblemRecovery\n\t\treturn result\n\t}\n\tif !validSQLArtifacts(evidence.Case, evidence.Artifacts) {\n\t\tresult.Problem = SQLProblemArtifacts\n\t\treturn result\n\t}\n\tresult.EvidenceConsistent, result.ScenarioObserved = true, true\n\treturn result\n}\n\nfunc validSQLReaderOwners(spec PlanSpec, before, after uint64) bool {\n\tif before == 0 || after == 0 || before == after {\n\t\treturn false\n\t}\n\tfor _, owner := range [4]uint64{spec.MainOwner, spec.JournalOwner, spec.DirectoryOwner, spec.CleanupOwner} {\n\t\tif before == owner || after == owner {\n\t\t\treturn false\n\t\t}\n\t}\n\treturn true\n}\n\n// ValidateSQLSnapshot permits a source harness to reject a malformed baseline\n// before any fault Commit. Success still has no physical validation authority.\nfunc ValidateSQLSnapshot(snapshot SQLSnapshotEvidence, identity Identity, state SQLState) SQLProblem {\n\tif !validIdentity(identity) || identity.Profile != PlainRollback4096 || snapshot.Identity != identity || snapshot.Owner == 0 {\n\t\treturn SQLProblemIdentity\n\t}\n\tif state != SQLOld && state != SQLNew {\n\t\treturn SQLProblemRows\n\t}\n\treturn validateSQLSnapshot(snapshot, identity, state)\n}\n\nfunc validateSQLSnapshot(snapshot SQLSnapshotEvidence, identity Identity, state SQLState) SQLProblem {\n\tif snapshot.CloseCalls != 1 || snapshot.CloseError != None || snapshot.CloseErrorToken != 0 || snapshot.OwnerState != OwnerRetired {\n\t\treturn SQLProblemClose\n\t}\n\tcheck := snapshot.Check\n\tif check.Identity != identity || check.Owner != snapshot.Owner {\n\t\treturn SQLProblemIdentity\n\t}\n\tif check.Calls != 1 || check.Error != None || check.ErrorToken != 0 || check.Pages != 3 || check.FreePages != 0 || check.ReservedPages != 0 || check.Tables != 1 || check.Indexes != 1 || check.ExtensionIndexes != 0 || check.TableRows != 3 || check.IndexEntries != 3 {\n\t\treturn SQLProblemCheck\n\t}\n\tfor i, query := range snapshot.Queries {\n\t\tif query.Identity != identity || query.Owner != snapshot.Owner || query.Iterator == 0 {\n\t\t\treturn SQLProblemIdentity\n\t\t}\n\t\tfor j := 0; j < i; j++ {\n\t\t\tif query.Iterator == snapshot.Queries[j].Iterator {\n\t\t\t\treturn SQLProblemLifecycle\n\t\t\t}\n\t\t}\n\t\tif query.Query != SQLQuery(i) || query.SQL != SQLQueryText(SQLQuery(i)) {\n\t\t\treturn SQLProblemQueryPlan\n\t\t}\n\t\tif !query.EOF || !query.ErrCalled || query.Error != None || query.ErrorToken != 0 {\n\t\t\treturn SQLProblemEOF\n\t\t}\n\t\tif query.CloseCalls != 1 || query.CloseError != None || query.CloseErrorToken != 0 {\n\t\t\treturn SQLProblemClose\n\t\t}\n\t\trows, count := expectedSQLQueryRows(state, query.Query)\n\t\tif query.RowCount != count || query.Rows != rows {\n\t\t\treturn SQLProblemRows\n\t\t}\n\t\tif !validSQLQueryPlan(query.Query, query.Plan) {\n\t\t\treturn SQLProblemQueryPlan\n\t\t}\n\t\tif !validSQLQueryWork(query.Query, query.Work, count) {\n\t\t\treturn SQLProblemIndexWork\n\t\t}\n\t}\n\treturn SQLProblemNone\n}\n\nfunc expectedSQLQueryRows(state SQLState, query SQLQuery) ([3]SQLRow, uint8) {\n\tall := SQLLedgerRows(state)\n\tif query == SQLAllRows {\n\t\treturn all, 3\n\t}\n\tkey := \"\"\n\tswitch query {\n\tcase SQLBlueIndex, SQLBlueScan:\n\t\tkey = \"blue\"\n\tcase SQLRedIndex, SQLRedScan:\n\t\tkey = \"red\"\n\tcase SQLGreenIndex, SQLGreenScan:\n\t\tkey = \"green\"\n\t}\n\tvar selected [3]SQLRow\n\tvar count uint8\n\tfor _, row := range all {\n\t\tif row.Key == key {\n\t\t\tselected[count] = row\n\t\t\tcount++\n\t\t}\n\t}\n\treturn selected, count\n}\n\nfunc isSQLIndexQuery(query SQLQuery) bool {\n\treturn query == SQLBlueIndex || query == SQLRedIndex || query == SQLGreenIndex\n}\n\nfunc validSQLQueryPlan(query SQLQuery, plan SQLQueryPlan) bool {\n\tif plan.Table != \"t\" {\n\t\treturn false\n\t}\n\tif isSQLIndexQuery(query) {\n\t\treturn plan.Access == \"SEARCH\" && plan.Method == \"btree\" && plan.Index == \"by_k\" && plan.IndexCount == 1 && plan.Field == \"k\" && plan.FieldCount == 1 && plan.Recheck && len(plan.Reason) <= 1024\n\t}\n\treturn plan.Access == \"SCAN\" && plan.Method == \"\" && plan.Index == \"\" && plan.IndexCount == 0 && plan.Field == \"\" && plan.FieldCount == 0 && !plan.Recheck && plan.Reason == \"NOT INDEXED requested\"\n}\n\nfunc validSQLQueryWork(query SQLQuery, work SQLQueryWork, count uint8) bool {\n\tn := int64(count)\n\tif work.ReturnedRows != n {\n\t\treturn false\n\t}\n\tif isSQLIndexQuery(query) {\n\t\t// An empty green result still needs a real index-page probe. Nonempty\n\t\t// equality keys must each drive one candidate and base-row fetch.\n\t\treturn work.IndexPageReads > 0 && work.IndexPageReads <= 32 && work.IndexKeysRead == n && work.Candidates == n && work.RowIDSeeks == n && work.BaseRowFetches == n\n\t}\n\treturn work.IndexKeysRead == 0 && work.IndexPageReads == 0 && work.RowIDSeeks == 0 && work.Candidates == 0 && work.BaseRowFetches == 3\n}\n\nfunc validSQLRecovery(which SQLCase, recovery SQLRecoveryEvidence) bool {\n\tif which == SQLControl {\n\t\t// A control must not call recovery, even as a purported no-op. Its\n\t\t// retained post-commit copy is opened read-only without journal replay.\n\t\treturn recovery == (SQLRecoveryEvidence{Identity: recovery.Identity, OwnersRetired: true})\n\t}\n\tif recovery.Calls != 1 || recovery.Error != None || recovery.ErrorToken != 0 || !recovery.OwnersRetired || recovery.JournalPresentAfter || recovery.Limits != (SQLRecoveryLimits{MaxJournalBytes: SQLJournalBytes, MaxRecords: SQLPageCount, MaxDatabaseBytes: SQLDatabaseBytes}) {\n\t\treturn false\n\t}\n\treturn recovery.JournalPresentBefore && recovery.Replayed && recovery.ReplayedPages == SQLPageCount && recovery.OriginalPages == SQLPageCount\n}\n\nfunc validSQLArtifacts(which SQLCase, artifacts [SQLArtifactCount]SQLArtifactEvidence) bool {\n\tfor i, artifact := range artifacts {\n\t\tif artifact.Role != SQLArtifactRole(i) {\n\t\t\treturn false\n\t\t}\n\t\tjournal := artifact.Role == SQLOriginalJournalArtifact || artifact.Role == SQLJournalCopyArtifact || artifact.Role == SQLReplayJournalArtifact\n\t\tif journal && which == SQLControl {\n\t\t\tif artifact.Before != (SQLArtifactStamp{}) || artifact.After != (SQLArtifactStamp{}) {\n\t\t\t\treturn false\n\t\t\t}\n\t\t\tcontinue\n\t\t}\n\t\tbefore, after := artifact.Before, artifact.After\n\t\tbytes := int64(SQLDatabaseBytes)\n\t\tif journal {\n\t\t\tbytes = SQLJournalBytes\n\t\t}\n\t\tif !validSQLArtifactStamp(before, bytes) || before.Identity != SQLArtifactName(artifact.Role) {\n\t\t\treturn false\n\t\t}\n\t\tif artifact.Role == SQLReplayJournalArtifact {\n\t\t\tif after != (SQLArtifactStamp{}) {\n\t\t\t\treturn false\n\t\t\t}\n\t\t} else {\n\t\t\tif !validSQLArtifactStamp(after, bytes) || before.Identity != after.Identity || before.Device != after.Device || before.Inode != after.Inode {\n\t\t\t\treturn false\n\t\t\t}\n\t\t\tif artifact.Role != SQLReplayArtifact && before != after {\n\t\t\t\treturn false\n\t\t\t}\n\t\t}\n\t\tfor j := 0; j < i; j++ {\n\t\t\tother := artifacts[j].Before\n\t\t\tif other.Exists && (before.Identity == other.Identity || (before.Device == other.Device && before.Inode == other.Inode)) {\n\t\t\t\treturn false\n\t\t\t}\n\t\t}\n\t}\n\tbaseline := artifacts[SQLBaselineArtifact].Before.SHA256\n\ttrial := artifacts[SQLTrialArtifact].Before.SHA256\n\tpost := artifacts[SQLPostCopyArtifact].Before.SHA256\n\treplayBefore := artifacts[SQLReplayArtifact].Before.SHA256\n\treplayAfter := artifacts[SQLReplayArtifact].After.SHA256\n\tif baseline == trial || trial != post || trial != replayBefore {\n\t\treturn false\n\t}\n\tif which == SQLControl {\n\t\treturn replayAfter == trial\n\t}\n\tjournal := artifacts[SQLOriginalJournalArtifact].Before.SHA256\n\treturn replayAfter == baseline && journal == artifacts[SQLJournalCopyArtifact].Before.SHA256 && journal == artifacts[SQLReplayJournalArtifact].Before.SHA256\n}\n\nfunc SQLArtifactName(role SQLArtifactRole) string {\n\tswitch role {\n\tcase SQLBaselineArtifact:\n\t\treturn \"baseline.input.db\"\n\tcase SQLTrialArtifact:\n\t\treturn \"trial.db\"\n\tcase SQLOriginalJournalArtifact:\n\t\treturn \"trial.db-journal\"\n\tcase SQLPostCopyArtifact:\n\t\treturn \"post-commit.input.db\"\n\tcase SQLJournalCopyArtifact:\n\t\treturn \"post-commit-journal.input.bin\"\n\tcase SQLReplayArtifact:\n\t\treturn \"replay.db\"\n\tcase SQLReplayJournalArtifact:\n\t\treturn \"replay.db-journal\"\n\tdefault:\n\t\treturn \"\"\n\t}\n}\n\nfunc validSQLArtifactStamp(stamp SQLArtifactStamp, bytes int64) bool {\n\tif !stamp.Exists || stamp.Device == 0 || stamp.Inode == 0 || stamp.Bytes != bytes || stamp.SHA256 == ([32]byte{}) || len(stamp.Identity) == 0 || stamp.Identity == \"0\" || len(stamp.Identity) > MaxIDLength {\n\t\treturn false\n\t}\n\tfor _, ch := range stamp.Identity {\n\t\tif ch < '!' || ch > '~' {\n\t\t\treturn false\n\t\t}\n\t}\n\treturn true\n}\n",
	rollbackFaultSQLModelTest: "package rollbackfault\n\nimport \"testing\"\n\n// SOURCE ONLY. These finite synthetic values do not create fixtures, descriptors,\n// native owners, SQL executions, Commit attempts, or recovery operations. Even\n// successful tests would establish only this evidence contract's consistency.\n\nfunc sqlLiteralRows(state SQLState) [3]SQLRow {\n\tif state == SQLNew {\n\t\treturn [3]SQLRow{\n\t\t\t{IDType: SQLInteger, ID: 1, KeyType: SQLText, Key: \"green\", ValueType: SQLText, Value: \"ONE\"},\n\t\t\t{IDType: SQLInteger, ID: 3, KeyType: SQLText, Key: \"red\", ValueType: SQLText, Value: \"three\"},\n\t\t\t{IDType: SQLInteger, ID: 4, KeyType: SQLText, Key: \"blue\", ValueType: SQLText, Value: \"four\"},\n\t\t}\n\t}\n\treturn [3]SQLRow{\n\t\t{IDType: SQLInteger, ID: 1, KeyType: SQLText, Key: \"red\", ValueType: SQLText, Value: \"one\"},\n\t\t{IDType: SQLInteger, ID: 2, KeyType: SQLText, Key: \"blue\", ValueType: SQLText, Value: \"two\"},\n\t\t{IDType: SQLInteger, ID: 3, KeyType: SQLText, Key: \"red\", ValueType: SQLText, Value: \"three\"},\n\t}\n}\n\nfunc sqlLiteralQueryText() [SQLQueryCount]string {\n\treturn [SQLQueryCount]string{\n\t\t\"SELECT id,k,v FROM t NOT INDEXED ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t WHERE k='blue' ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t NOT INDEXED WHERE k='blue' ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t WHERE k='red' ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t NOT INDEXED WHERE k='red' ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t WHERE k='green' ORDER BY id\",\n\t\t\"SELECT id,k,v FROM t NOT INDEXED WHERE k='green' ORDER BY id\",\n\t}\n}\n\nfunc sqlSnapshot(identity Identity, owner, firstIterator uint64, state SQLState) SQLSnapshotEvidence {\n\tsnapshot := SQLSnapshotEvidence{\n\t\tIdentity: identity, Owner: owner, CloseCalls: 1, OwnerState: OwnerRetired,\n\t\tCheck: SQLCheckEvidence{Identity: identity, Owner: owner, Calls: 1, Pages: 3, Tables: 1, Indexes: 1, TableRows: 3, IndexEntries: 3},\n\t}\n\tall := sqlLiteralRows(state)\n\ttexts := sqlLiteralQueryText()\n\tkeys := [SQLQueryCount]string{\"\", \"blue\", \"blue\", \"red\", \"red\", \"green\", \"green\"}\n\tfor i := 0; i < SQLQueryCount; i++ {\n\t\tquery := SQLQueryEvidence{\n\t\t\tIdentity: identity, Query: SQLQuery(i), Owner: owner, Iterator: firstIterator + uint64(i), SQL: texts[i],\n\t\t\tEOF: true, ErrCalled: true, CloseCalls: 1,\n\t\t\tPlan: SQLQueryPlan{Access: \"SCAN\", Table: \"t\", Reason: \"NOT INDEXED requested\"},\n\t\t\tWork: SQLQueryWork{BaseRowFetches: 3},\n\t\t}\n\t\tfor _, row := range all {\n\t\t\tif i == 0 || row.Key == keys[i] {\n\t\t\t\tquery.Rows[query.RowCount] = row\n\t\t\t\tquery.RowCount++\n\t\t\t}\n\t\t}\n\t\tquery.Work.ReturnedRows = int64(query.RowCount)\n\t\tif i == 1 || i == 3 || i == 5 {\n\t\t\tn := int64(query.RowCount)\n\t\t\tquery.Plan = SQLQueryPlan{Access: \"SEARCH\", Table: \"t\", Method: \"btree\", Index: \"by_k\", IndexCount: 1, Field: \"k\", FieldCount: 1, Recheck: true, Reason: \"BINARY ASC leading equality prefix; rowid-ordered candidates; ANN and grams retain precedence\"}\n\t\t\tquery.Work = SQLQueryWork{IndexKeysRead: n, IndexPageReads: 1, RowIDSeeks: n, Candidates: n, BaseRowFetches: n, ReturnedRows: n}\n\t\t}\n\t\tsnapshot.Queries[i] = query\n\t}\n\treturn snapshot\n}\n\nfunc sqlReceipt(t *testing.T, plan Plan, events []Event) Receipt {\n\tt.Helper()\n\treceipt, problem := NewReceipt(plan)\n\tif problem != NoProblem {\n\t\tt.Fatalf(\"synthetic plan refused: %v\", problem)\n\t}\n\tfor _, event := range events {\n\t\tif receipt.Append(event) != NoProblem {\n\t\t\tbreak\n\t\t}\n\t}\n\treturn receipt\n}\n\nfunc sqlEvidenceFixture(t *testing.T, which SQLCase) (Plan, Receipt, SQLLedgerEvidence) {\n\tt.Helper()\n\tidentity := Identity{Suite: \"sql-suite-v1\", Source: \"source-v1\", Tool: \"tool-v1\", Binary: \"binary-v1\", Manifest: \"manifest-v1\", Trial: \"sql-trial-1\", Scope: \"sql-scope-1\", Profile: PlainRollback4096}\n\tplan, problem := SQLPlan(identity, which)\n\tif problem != NoProblem {\n\t\tt.Fatalf(\"synthetic SQL plan refused: %v\", problem)\n\t}\n\tstate := SQLOld\n\tif which == SQLControl {\n\t\tstate = SQLNew\n\t}\n\tvar hashes [3][32]byte\n\thashes[0][0], hashes[1][0], hashes[2][0] = 11, 22, 33\n\tevidence := SQLLedgerEvidence{\n\t\tIdentity: identity, Case: which,\n\t\tBefore:    sqlSnapshot(identity, 101, 1, SQLOld),\n\t\tAfter:     sqlSnapshot(identity, 102, 8, state),\n\t\tOriginals: SQLOriginalEvidence{Frozen: hashes, Before: hashes, After: hashes, BeforeBytes: 12288, AfterBytes: 12288},\n\t\tRecovery:  SQLRecoveryEvidence{Identity: identity, OwnersRetired: true},\n\t\tLifecycle: SQLLifecycleEvidence{ArmCalls: 1, CommitCalls: 1, FinishCalls: 1, DisarmCalls: 1, WriterOpens: 1, ReopenCalls: 1, ReopenReadOnly: true, WriterRetiredBeforeRecovery: true, RecoveryFinishedBeforeReopen: true, CommitCapturedUnchanged: true},\n\t}\n\tif which != SQLControl {\n\t\tevidence.Recovery.Calls = 1\n\t\tevidence.Recovery.Limits = SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}\n\t\tevidence.Recovery.JournalPresentBefore = true\n\t\tevidence.Recovery.Replayed, evidence.Recovery.ReplayedPages = true, 3\n\t\tevidence.Recovery.OriginalPages = 3\n\t\tevidence.Lifecycle.PublicCodeIOErr, evidence.Lifecycle.PublicRecoveryRequired = true, true\n\t\tevidence.Lifecycle.PublicShortWrite = which == SQLIndexPrefix64\n\t}\n\tlabels := [SQLArtifactCount]string{\"baseline.input.db\", \"trial.db\", \"trial.db-journal\", \"post-commit.input.db\", \"post-commit-journal.input.bin\", \"replay.db\", \"replay.db-journal\"}\n\tfor i := range evidence.Artifacts {\n\t\trole := SQLArtifactRole(i)\n\t\tevidence.Artifacts[i].Role = role\n\t\tjournal := role == SQLOriginalJournalArtifact || role == SQLJournalCopyArtifact || role == SQLReplayJournalArtifact\n\t\tif journal && which == SQLControl {\n\t\t\tcontinue\n\t\t}\n\t\tstamp := SQLArtifactStamp{Identity: labels[i], Device: 1, Inode: uint64(i + 1), Exists: true, Bytes: 12288}\n\t\tstamp.SHA256[0] = 22\n\t\tif role == SQLBaselineArtifact {\n\t\t\tstamp.SHA256[0] = 11\n\t\t}\n\t\tif journal {\n\t\t\tstamp.Bytes, stamp.SHA256[0] = 16408, 33\n\t\t}\n\t\tevidence.Artifacts[i].Before, evidence.Artifacts[i].After = stamp, stamp\n\t\tif which != SQLControl && role == SQLReplayArtifact {\n\t\t\tevidence.Artifacts[i].After.SHA256[0] = 11\n\t\t}\n\t\tif role == SQLReplayJournalArtifact {\n\t\t\tevidence.Artifacts[i].After = SQLArtifactStamp{}\n\t\t}\n\t}\n\treturn plan, sqlReceipt(t, plan, fixtureEvents(plan.Spec())), evidence\n}\n\nfunc sqlMustReject(t *testing.T, observation SQLObservation) {\n\tt.Helper()\n\tif observation.EvidenceConsistent || observation.ScenarioObserved || observation.Problem == SQLProblemNone || observation.PhysicalValidated() || observation.Pass() {\n\t\tt.Fatalf(\"invalid synthetic SQL evidence qualified: %+v\", observation)\n\t}\n}\n\nfunc TestSQLLedgerThreeFrozenScenariosRemainObservationOnly(t *testing.T) {\n\tfor _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore} {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, which)\n\t\tobservation := ObserveSQLLedger(plan, receipt, evidence)\n\t\tif !observation.EvidenceConsistent || !observation.ScenarioObserved || observation.Problem != SQLProblemNone || observation.Adapter.Outcome != AdapterObserved {\n\t\t\tt.Fatalf(\"scenario %v: %+v\", which, observation)\n\t\t}\n\t\tif observation.PhysicalValidated() || observation.Pass() {\n\t\t\tt.Fatal(\"finite SQL evidence claimed physical acceptance\")\n\t\t}\n\t}\n\t// The public result itself is not a capability to claim physical acceptance.\n\tforged := SQLObservation{EvidenceConsistent: true, ScenarioObserved: true}\n\tif forged.PhysicalValidated() || forged.Pass() {\n\t\tt.Fatal(\"fabricated result claimed physical acceptance\")\n\t}\n}\n\nfunc TestSQLLedgerLiteralExpectationsAndQueriesAreDetached(t *testing.T) {\n\tfor _, state := range []SQLState{SQLOld, SQLNew} {\n\t\trows := SQLLedgerRows(state)\n\t\tif rows != sqlLiteralRows(state) {\n\t\t\tt.Fatalf(\"state %v differs from independent literal ledger\", state)\n\t\t}\n\t\trows[0].ID, rows[0].Key, rows[0].Value = 99, \"wrong\", \"wrong\"\n\t\tif SQLLedgerRows(state) != sqlLiteralRows(state) {\n\t\t\tt.Fatal(\"returned ledger aliases its frozen expectations\")\n\t\t}\n\t}\n\tif SQLLedgerRows(SQLStateUnknown) != ([3]SQLRow{}) || SQLLedgerRows(SQLState(255)) != ([3]SQLRow{}) {\n\t\tt.Fatal(\"unknown ledger state received valid rows\")\n\t}\n\tfor query, text := range sqlLiteralQueryText() {\n\t\tif SQLQueryText(SQLQuery(query)) != text {\n\t\t\tt.Fatalf(\"query %v lost frozen text\", query)\n\t\t}\n\t}\n\tif SQLQueryText(SQLQuery(255)) != \"\" {\n\t\tt.Fatal(\"unknown query received a valid statement\")\n\t}\n}\n\nfunc TestSQLLedgerPlansFreezeThreePageSelectorsAndLimits(t *testing.T) {\n\tfor _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore} {\n\t\tplan, _, _ := sqlEvidenceFixture(t, which)\n\t\tspec := plan.Spec()\n\t\tif spec.OriginalPages != 3 || spec.DirtyPages != 3 || spec.PageSize != 4096 || spec.SectorSize != 4096 || spec.MainOwner != 11 || spec.JournalOwner != 12 || spec.DirectoryOwner != 13 || spec.CleanupOwner != 14 {\n\t\t\tt.Fatalf(\"geometry/ownership changed: %+v\", spec)\n\t\t}\n\t\tfor i := 0; i < 3; i++ {\n\t\t\tread := spec.Calls[2+2*i]\n\t\t\trecord := spec.Calls[3+2*i]\n\t\t\twrite := spec.Calls[12+i]\n\t\t\tif read.Point != DBOriginalRead || read.Occurrence != uint8(i+1) || read.Offset != int64(i*4096) || record.Point != JournalRecord || record.Occurrence != uint8(i+1) || record.Offset != int64(4096+i*4104) || write.Point != DBPage || write.Occurrence != uint8(i+1) || write.Offset != int64(i*4096) {\n\t\t\t\tt.Fatalf(\"page role %v was not independently selected\", i)\n\t\t\t}\n\t\t}\n\t\tswitch which {\n\t\tcase SQLControl:\n\t\t\tif spec.CallCount != 18 || spec.Action != Pass || spec.ExpectedCommit != CommitSucceeded || spec.Target != spec.Calls[14] {\n\t\t\t\tt.Fatal(\"control selector differs\")\n\t\t\t}\n\t\tcase SQLIndexPrefix64:\n\t\t\tif spec.CallCount != 15 || spec.Action != PrefixShortWrite || spec.Prefix != 64 || spec.PrefixError || spec.ExpectedCommit != CommitRecoveryRequired || spec.Target != spec.Calls[14] {\n\t\t\t\tt.Fatal(\"index prefix selector differs\")\n\t\t\t}\n\t\tcase SQLDBSyncBefore:\n\t\t\tif spec.CallCount != 16 || spec.Action != FailBefore || spec.ExpectedCommit != CommitRecoveryRequired || spec.Target != spec.Calls[15] {\n\t\t\t\tt.Fatal(\"sync-before selector differs\")\n\t\t\t}\n\t\t}\n\t\tfor i := int(spec.CallCount); i < MaxCalls; i++ {\n\t\t\tif spec.Calls[i] != (Call{}) {\n\t\t\t\tt.Fatal(\"unused call tail was retained\")\n\t\t\t}\n\t\t}\n\t}\n\tplan, _, _ := sqlEvidenceFixture(t, SQLControl)\n\tif _, problem := SQLPlan(plan.Spec().Identity, SQLCaseUnknown); problem == NoProblem {\n\t\tt.Fatal(\"unknown SQL case admitted\")\n\t}\n\tif _, problem := SQLPlan(Identity{}, SQLControl); problem == NoProblem {\n\t\tt.Fatal(\"missing SQL identity admitted\")\n\t}\n}\n\nfunc TestSQLLedgerRejectsOtherwiseAdmittedNonFrozenFaultPlans(t *testing.T) {\n\tfor _, name := range []string{\"prefix65\", \"prefix-error\", \"earlier-db-page\", \"fourth-page-offset\", \"different-owner\", \"control-sync-target\"} {\n\t\twhich := SQLIndexPrefix64\n\t\tif name == \"control-sync-target\" {\n\t\t\twhich = SQLControl\n\t\t}\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, which)\n\t\tspec := plan.Spec()\n\t\tswitch name {\n\t\tcase \"prefix65\":\n\t\t\tspec.Prefix = 65\n\t\tcase \"prefix-error\":\n\t\t\tspec.PrefixError = true\n\t\tcase \"earlier-db-page\":\n\t\t\tspec.Target, spec.CallCount = spec.Calls[13], 14\n\t\t\tfor i := 14; i < MaxCalls; i++ {\n\t\t\t\tspec.Calls[i] = Call{}\n\t\t\t}\n\t\tcase \"fourth-page-offset\":\n\t\t\tspec.Calls[14].Offset = 12288\n\t\t\tspec.Target = spec.Calls[14]\n\t\tcase \"different-owner\":\n\t\t\tspec.MainOwner = 21\n\t\t\tfor i := 0; i < int(spec.CallCount); i++ {\n\t\t\t\tif spec.Calls[i].Role == Database {\n\t\t\t\t\tspec.Calls[i].Owner = 21\n\t\t\t\t}\n\t\t\t}\n\t\t\tspec.Target = spec.Calls[14]\n\t\tcase \"control-sync-target\":\n\t\t\tspec.Target = spec.Calls[15]\n\t\t}\n\t\tchanged, problem := NewPlan(spec)\n\t\tif problem != NoProblem {\n\t\t\tt.Fatalf(\"%s negative should pass generic admission: %v\", name, problem)\n\t\t}\n\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(changed, receipt, evidence)) })\n\t}\n}\n\nfunc TestSQLLedgerRejectsLogicalMixturesAndEqualCountWrongMembers(t *testing.T) {\n\tfor _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore} {\n\t\tfor _, name := range []string{\"wrong-ledger\", \"oldrows-newindex\", \"newrows-oldindex\", \"equal-count-wrong-member\", \"equal-count-wrong-value\", \"index-scan-both-wrong\", \"wrong-id-type\", \"wrong-key-type\", \"wrong-value-type\", \"same-count-duplicate-id\", \"unreported-tail\", \"row-reordered\", \"empty-green-hidden-member\", \"before-not-old\"} {\n\t\t\tplan, receipt, evidence := sqlEvidenceFixture(t, which)\n\t\t\told := sqlSnapshot(evidence.Identity, 102, 8, SQLOld)\n\t\t\tnewState := sqlSnapshot(evidence.Identity, 102, 8, SQLNew)\n\t\t\tswitch name {\n\t\t\tcase \"wrong-ledger\":\n\t\t\t\tif which == SQLControl {\n\t\t\t\t\tevidence.After = old\n\t\t\t\t} else {\n\t\t\t\t\tevidence.After = newState\n\t\t\t\t}\n\t\t\tcase \"oldrows-newindex\":\n\t\t\t\tevidence.After = old\n\t\t\t\tfor _, q := range []SQLQuery{SQLBlueIndex, SQLRedIndex, SQLGreenIndex} {\n\t\t\t\t\tevidence.After.Queries[q] = newState.Queries[q]\n\t\t\t\t}\n\t\t\tcase \"newrows-oldindex\":\n\t\t\t\tevidence.After = newState\n\t\t\t\tfor _, q := range []SQLQuery{SQLBlueIndex, SQLRedIndex, SQLGreenIndex} {\n\t\t\t\t\tevidence.After.Queries[q] = old.Queries[q]\n\t\t\t\t}\n\t\t\tcase \"equal-count-wrong-member\":\n\t\t\t\tevidence.After.Queries[SQLBlueIndex].Rows[0].ID = 77\n\t\t\tcase \"equal-count-wrong-value\":\n\t\t\t\tevidence.After.Queries[SQLAllRows].Rows[0].Value = \"one!\"\n\t\t\tcase \"index-scan-both-wrong\":\n\t\t\t\tevidence.After.Queries[SQLBlueIndex].Rows[0].ID = 88\n\t\t\t\tevidence.After.Queries[SQLBlueScan].Rows[0].ID = 88\n\t\t\tcase \"wrong-id-type\":\n\t\t\t\tevidence.After.Queries[SQLAllRows].Rows[0].IDType = SQLText\n\t\t\tcase \"wrong-key-type\":\n\t\t\t\tevidence.After.Queries[SQLAllRows].Rows[0].KeyType = SQLInteger\n\t\t\tcase \"wrong-value-type\":\n\t\t\t\tevidence.After.Queries[SQLAllRows].Rows[0].ValueType = SQLTypeUnknown\n\t\t\tcase \"same-count-duplicate-id\":\n\t\t\t\tevidence.After.Queries[SQLAllRows].Rows[1].ID = evidence.After.Queries[SQLAllRows].Rows[0].ID\n\t\t\tcase \"unreported-tail\":\n\t\t\t\tevidence.After.Queries[SQLBlueIndex].Rows[2] = old.Queries[SQLAllRows].Rows[0]\n\t\t\tcase \"row-reordered\":\n\t\t\t\tevidence.After.Queries[SQLAllRows].Rows[0], evidence.After.Queries[SQLAllRows].Rows[1] = evidence.After.Queries[SQLAllRows].Rows[1], evidence.After.Queries[SQLAllRows].Rows[0]\n\t\t\tcase \"empty-green-hidden-member\":\n\t\t\t\tevidence.Before.Queries[SQLGreenIndex].Rows[0] = old.Queries[SQLAllRows].Rows[0]\n\t\t\tcase \"before-not-old\":\n\t\t\t\tevidence.Before = sqlSnapshot(evidence.Identity, 101, 1, SQLNew)\n\t\t\t}\n\t\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t\t}\n\t}\n}\n\nfunc TestSQLLedgerRejectsPlanClaimsWithoutActualIndexWorkAndIndependentScans(t *testing.T) {\n\tfor _, name := range []string{\"scan-disguised-as-search\", \"wrong-method\", \"wrong-table\", \"wrong-index\", \"extra-index\", \"wrong-field\", \"extra-field\", \"no-recheck\", \"no-index-pages\", \"no-index-keys\", \"no-rowid-seek\", \"no-base-fetch\", \"no-candidates\", \"wrong-return-count\", \"negative-work\", \"unbounded-work\", \"empty-green-no-probe\", \"scan-uses-index\", \"scan-fewer-rows\", \"scan-has-candidates\", \"scan-fake-reason\", \"scan-has-method\", \"copied-index-query\", \"changed-sql\", \"reordered-query\", \"duplicate-iterator\"} {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, SQLControl)\n\t\tquery := &evidence.After.Queries[SQLBlueIndex]\n\t\tswitch name {\n\t\tcase \"scan-disguised-as-search\":\n\t\t\tquery.Plan.Access = \"SCAN\"\n\t\tcase \"wrong-method\":\n\t\t\tquery.Plan.Method = \"rowid\"\n\t\tcase \"wrong-table\":\n\t\t\tquery.Plan.Table = \"other\"\n\t\tcase \"wrong-index\":\n\t\t\tquery.Plan.Index = \"other\"\n\t\tcase \"extra-index\":\n\t\t\tquery.Plan.IndexCount = 2\n\t\tcase \"wrong-field\":\n\t\t\tquery.Plan.Field = \"v\"\n\t\tcase \"extra-field\":\n\t\t\tquery.Plan.FieldCount = 2\n\t\tcase \"no-recheck\":\n\t\t\tquery.Plan.Recheck = false\n\t\tcase \"no-index-pages\":\n\t\t\tquery.Work.IndexPageReads = 0\n\t\tcase \"no-index-keys\":\n\t\t\tquery.Work.IndexKeysRead = 0\n\t\tcase \"no-rowid-seek\":\n\t\t\tquery.Work.RowIDSeeks = 0\n\t\tcase \"no-base-fetch\":\n\t\t\tquery.Work.BaseRowFetches = 0\n\t\tcase \"no-candidates\":\n\t\t\tquery.Work.Candidates = 0\n\t\tcase \"wrong-return-count\":\n\t\t\tquery.Work.ReturnedRows = 2\n\t\tcase \"negative-work\":\n\t\t\tquery.Work.IndexPageReads = -1\n\t\tcase \"unbounded-work\":\n\t\t\tquery.Work.IndexPageReads = 33\n\t\tcase \"empty-green-no-probe\":\n\t\t\tevidence.Before.Queries[SQLGreenIndex].Work.IndexPageReads = 0\n\t\tcase \"scan-uses-index\":\n\t\t\tevidence.After.Queries[SQLBlueScan].Work.IndexKeysRead = 1\n\t\tcase \"scan-fewer-rows\":\n\t\t\tevidence.After.Queries[SQLBlueScan].Work.BaseRowFetches = 2\n\t\tcase \"scan-has-candidates\":\n\t\t\tevidence.After.Queries[SQLBlueScan].Work.Candidates = 1\n\t\tcase \"scan-fake-reason\":\n\t\t\tevidence.After.Queries[SQLBlueScan].Plan.Reason = \"no index\"\n\t\tcase \"scan-has-method\":\n\t\t\tevidence.After.Queries[SQLBlueScan].Plan.Method = \"btree\"\n\t\tcase \"copied-index-query\":\n\t\t\tevidence.After.Queries[SQLBlueScan] = evidence.After.Queries[SQLBlueIndex]\n\t\tcase \"changed-sql\":\n\t\t\tquery.SQL = \"EXPLAIN QUERY PLAN SELECT id,k,v FROM t WHERE k='blue' ORDER BY id\"\n\t\tcase \"reordered-query\":\n\t\t\tevidence.After.Queries[SQLBlueScan], evidence.After.Queries[SQLRedScan] = evidence.After.Queries[SQLRedScan], evidence.After.Queries[SQLBlueScan]\n\t\tcase \"duplicate-iterator\":\n\t\t\tevidence.After.Queries[SQLBlueScan].Iterator = query.Iterator\n\t\t}\n\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t}\n}\n\nfunc TestSQLLedgerRequiresEOFErrOnceCloseAndIndependentRetiredOwners(t *testing.T) {\n\tfor _, name := range []string{\"not-eof\", \"err-not-called\", \"err-present\", \"err-token\", \"missing-row-close\", \"duplicate-row-close\", \"row-close-error\", \"row-close-token\", \"missing-db-close\", \"duplicate-db-close\", \"db-close-error\", \"db-close-token\", \"db-owner-live\", \"same-owner\", \"writer-owner\", \"zero-owner\", \"zero-iterator\", \"iterator-reused-across-opens\", \"query-owner-mismatch\", \"check-owner-mismatch\", \"second-arm\", \"missing-arm\", \"second-commit\", \"missing-commit\", \"missing-finish\", \"second-finish\", \"missing-disarm\", \"second-disarm\", \"reconstructed-commit-error\", \"missing-public-code\", \"missing-public-recovery\", \"missing-public-shortwrite\", \"rollback-after-commit\", \"second-writer-open\", \"second-reopen\", \"writable-reopen\", \"unretired-before-recovery\", \"reopen-before-recovery\"} {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, SQLIndexPrefix64)\n\t\tquery := &evidence.After.Queries[SQLAllRows]\n\t\tswitch name {\n\t\tcase \"not-eof\":\n\t\t\tquery.EOF = false\n\t\tcase \"err-not-called\":\n\t\t\tquery.ErrCalled = false\n\t\tcase \"err-present\":\n\t\t\tquery.Error, query.ErrorToken = Underlying, 10\n\t\tcase \"err-token\":\n\t\t\tquery.ErrorToken = 10\n\t\tcase \"missing-row-close\":\n\t\t\tquery.CloseCalls = 0\n\t\tcase \"duplicate-row-close\":\n\t\t\tquery.CloseCalls = 2\n\t\tcase \"row-close-error\":\n\t\t\tquery.CloseError, query.CloseErrorToken = Underlying, 10\n\t\tcase \"row-close-token\":\n\t\t\tquery.CloseErrorToken = 10\n\t\tcase \"missing-db-close\":\n\t\t\tevidence.After.CloseCalls = 0\n\t\tcase \"duplicate-db-close\":\n\t\t\tevidence.After.CloseCalls = 2\n\t\tcase \"db-close-error\":\n\t\t\tevidence.After.CloseError, evidence.After.CloseErrorToken = Underlying, 10\n\t\tcase \"db-close-token\":\n\t\t\tevidence.After.CloseErrorToken = 10\n\t\tcase \"db-owner-live\":\n\t\t\tevidence.After.OwnerState = OwnerLive\n\t\tcase \"same-owner\":\n\t\t\tevidence.After.Owner = evidence.Before.Owner\n\t\tcase \"writer-owner\":\n\t\t\tevidence.After.Owner = plan.Spec().MainOwner\n\t\tcase \"zero-owner\":\n\t\t\tevidence.After.Owner = 0\n\t\tcase \"zero-iterator\":\n\t\t\tquery.Iterator = 0\n\t\tcase \"iterator-reused-across-opens\":\n\t\t\tquery.Iterator = evidence.Before.Queries[SQLAllRows].Iterator\n\t\tcase \"query-owner-mismatch\":\n\t\t\tquery.Owner = 99\n\t\tcase \"check-owner-mismatch\":\n\t\t\tevidence.After.Check.Owner = 99\n\t\tcase \"second-arm\":\n\t\t\tevidence.Lifecycle.ArmCalls = 2\n\t\tcase \"missing-arm\":\n\t\t\tevidence.Lifecycle.ArmCalls = 0\n\t\tcase \"second-commit\":\n\t\t\tevidence.Lifecycle.CommitCalls = 2\n\t\tcase \"missing-commit\":\n\t\t\tevidence.Lifecycle.CommitCalls = 0\n\t\tcase \"missing-finish\":\n\t\t\tevidence.Lifecycle.FinishCalls = 0\n\t\tcase \"second-finish\":\n\t\t\tevidence.Lifecycle.FinishCalls = 2\n\t\tcase \"missing-disarm\":\n\t\t\tevidence.Lifecycle.DisarmCalls = 0\n\t\tcase \"second-disarm\":\n\t\t\tevidence.Lifecycle.DisarmCalls = 2\n\t\tcase \"reconstructed-commit-error\":\n\t\t\tevidence.Lifecycle.CommitCapturedUnchanged = false\n\t\tcase \"missing-public-code\":\n\t\t\tevidence.Lifecycle.PublicCodeIOErr = false\n\t\tcase \"missing-public-recovery\":\n\t\t\tevidence.Lifecycle.PublicRecoveryRequired = false\n\t\tcase \"missing-public-shortwrite\":\n\t\t\tevidence.Lifecycle.PublicShortWrite = false\n\t\tcase \"rollback-after-commit\":\n\t\t\tevidence.Lifecycle.RollbackCalls = 1\n\t\tcase \"second-writer-open\":\n\t\t\tevidence.Lifecycle.WriterOpens = 2\n\t\tcase \"second-reopen\":\n\t\t\tevidence.Lifecycle.ReopenCalls = 2\n\t\tcase \"writable-reopen\":\n\t\t\tevidence.Lifecycle.ReopenReadOnly = false\n\t\tcase \"unretired-before-recovery\":\n\t\t\tevidence.Lifecycle.WriterRetiredBeforeRecovery = false\n\t\tcase \"reopen-before-recovery\":\n\t\t\tevidence.Lifecycle.RecoveryFinishedBeforeReopen = false\n\t\t}\n\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t}\n}\n\nfunc TestSQLLedgerRequiresCheckHashesAndExactRecoveryEvidence(t *testing.T) {\n\tfor _, name := range []string{\"missing-check\", \"duplicate-check\", \"check-error\", \"check-token\", \"pages\", \"tables\", \"indexes\", \"rows\", \"entries\", \"extensions\", \"free-pages\", \"reserved-pages\", \"before-hash-changed\", \"after-hash-changed\", \"zero-hash\", \"before-size\", \"after-size\", \"missing-recovery\", \"second-recovery\", \"recovery-error\", \"recovery-token\", \"journal-byte-limit\", \"record-limit\", \"database-limit\", \"missing-hot-journal\", \"journal-survives\", \"missing-replay\", \"too-few-replay-pages\", \"too-many-replay-pages\", \"wrong-original-pages\", \"recovery-owner-live\", \"physical-claim\"} {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, SQLIndexPrefix64)\n\t\tswitch name {\n\t\tcase \"missing-check\":\n\t\t\tevidence.After.Check.Calls = 0\n\t\tcase \"duplicate-check\":\n\t\t\tevidence.After.Check.Calls = 2\n\t\tcase \"check-error\":\n\t\t\tevidence.After.Check.Error, evidence.After.Check.ErrorToken = Underlying, 5\n\t\tcase \"check-token\":\n\t\t\tevidence.After.Check.ErrorToken = 5\n\t\tcase \"pages\":\n\t\t\tevidence.After.Check.Pages = 4\n\t\tcase \"tables\":\n\t\t\tevidence.After.Check.Tables = 2\n\t\tcase \"indexes\":\n\t\t\tevidence.After.Check.Indexes = 0\n\t\tcase \"rows\":\n\t\t\tevidence.After.Check.TableRows = 2\n\t\tcase \"entries\":\n\t\t\tevidence.After.Check.IndexEntries = 2\n\t\tcase \"extensions\":\n\t\t\tevidence.After.Check.ExtensionIndexes = 1\n\t\tcase \"free-pages\":\n\t\t\tevidence.After.Check.FreePages = 1\n\t\tcase \"reserved-pages\":\n\t\t\tevidence.After.Check.ReservedPages = 1\n\t\tcase \"before-hash-changed\":\n\t\t\tevidence.Originals.Before[1][0]++\n\t\tcase \"after-hash-changed\":\n\t\t\tevidence.Originals.After[2][0]++\n\t\tcase \"zero-hash\":\n\t\t\tevidence.Originals.Frozen[1], evidence.Originals.Before[1], evidence.Originals.After[1] = [32]byte{}, [32]byte{}, [32]byte{}\n\t\tcase \"before-size\":\n\t\t\tevidence.Originals.BeforeBytes--\n\t\tcase \"after-size\":\n\t\t\tevidence.Originals.AfterBytes++\n\t\tcase \"missing-recovery\":\n\t\t\tevidence.Recovery.Calls = 0\n\t\tcase \"second-recovery\":\n\t\t\tevidence.Recovery.Calls = 2\n\t\tcase \"recovery-error\":\n\t\t\tevidence.Recovery.Error, evidence.Recovery.ErrorToken = Underlying, 5\n\t\tcase \"recovery-token\":\n\t\t\tevidence.Recovery.ErrorToken = 5\n\t\tcase \"journal-byte-limit\":\n\t\t\tevidence.Recovery.Limits.MaxJournalBytes++\n\t\tcase \"record-limit\":\n\t\t\tevidence.Recovery.Limits.MaxRecords = 4\n\t\tcase \"database-limit\":\n\t\t\tevidence.Recovery.Limits.MaxDatabaseBytes = 16384\n\t\tcase \"missing-hot-journal\":\n\t\t\tevidence.Recovery.JournalPresentBefore = false\n\t\tcase \"journal-survives\":\n\t\t\tevidence.Recovery.JournalPresentAfter = true\n\t\tcase \"missing-replay\":\n\t\t\tevidence.Recovery.Replayed = false\n\t\tcase \"too-few-replay-pages\":\n\t\t\tevidence.Recovery.ReplayedPages = 2\n\t\tcase \"too-many-replay-pages\":\n\t\t\tevidence.Recovery.ReplayedPages = 4\n\t\tcase \"wrong-original-pages\":\n\t\t\tevidence.Recovery.OriginalPages = 2\n\t\tcase \"recovery-owner-live\":\n\t\t\tevidence.Recovery.OwnersRetired = false\n\t\tcase \"physical-claim\":\n\t\t\tevidence.PhysicalClaim = true\n\t\t}\n\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t}\n\tplan, receipt, evidence := sqlEvidenceFixture(t, SQLControl)\n\tevidence.Recovery.Replayed, evidence.Recovery.ReplayedPages = true, 3\n\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))\n}\n\nfunc TestSQLLedgerRetainsOriginalsAndRejectsAliasedOrMixedReplayArtifacts(t *testing.T) {\n\tfor _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore} {\n\t\tfor _, name := range []string{\"wrong-role\", \"absent-baseline\", \"absent-trial\", \"absent-replay\", \"changed-baseline\", \"changed-trial\", \"changed-post-copy\", \"replay-aliases-trial\", \"post-aliases-trial\", \"changed-inode\", \"changed-device\", \"zero-device\", \"zero-inode\", \"zero-hash\", \"wrong-size\", \"wrong-label\", \"same-post-and-baseline\", \"wrong-replay-input\", \"wrong-replay-output\", \"same-old-and-new-hash\"} {\n\t\t\tplan, receipt, evidence := sqlEvidenceFixture(t, which)\n\t\t\tswitch name {\n\t\t\tcase \"wrong-role\":\n\t\t\t\tevidence.Artifacts[SQLTrialArtifact].Role = SQLPostCopyArtifact\n\t\t\tcase \"absent-baseline\":\n\t\t\t\tevidence.Artifacts[SQLBaselineArtifact].Before = SQLArtifactStamp{}\n\t\t\tcase \"absent-trial\":\n\t\t\t\tevidence.Artifacts[SQLTrialArtifact].Before = SQLArtifactStamp{}\n\t\t\tcase \"absent-replay\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].Before = SQLArtifactStamp{}\n\t\t\tcase \"changed-baseline\":\n\t\t\t\tevidence.Artifacts[SQLBaselineArtifact].After.SHA256[0]++\n\t\t\tcase \"changed-trial\":\n\t\t\t\tevidence.Artifacts[SQLTrialArtifact].After.SHA256[0]++\n\t\t\tcase \"changed-post-copy\":\n\t\t\t\tevidence.Artifacts[SQLPostCopyArtifact].After.SHA256[0]++\n\t\t\tcase \"replay-aliases-trial\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].Before.Inode, evidence.Artifacts[SQLReplayArtifact].After.Inode = evidence.Artifacts[SQLTrialArtifact].Before.Inode, evidence.Artifacts[SQLTrialArtifact].Before.Inode\n\t\t\tcase \"post-aliases-trial\":\n\t\t\t\tevidence.Artifacts[SQLPostCopyArtifact].Before.Inode, evidence.Artifacts[SQLPostCopyArtifact].After.Inode = evidence.Artifacts[SQLTrialArtifact].Before.Inode, evidence.Artifacts[SQLTrialArtifact].Before.Inode\n\t\t\tcase \"changed-inode\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].After.Inode++\n\t\t\tcase \"changed-device\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].After.Device++\n\t\t\tcase \"zero-device\":\n\t\t\t\tevidence.Artifacts[SQLBaselineArtifact].Before.Device = 0\n\t\t\tcase \"zero-inode\":\n\t\t\t\tevidence.Artifacts[SQLBaselineArtifact].Before.Inode = 0\n\t\t\tcase \"zero-hash\":\n\t\t\t\tevidence.Artifacts[SQLBaselineArtifact].Before.SHA256 = [32]byte{}\n\t\t\tcase \"wrong-size\":\n\t\t\t\tevidence.Artifacts[SQLTrialArtifact].Before.Bytes++\n\t\t\tcase \"wrong-label\":\n\t\t\t\tevidence.Artifacts[SQLTrialArtifact].Before.Identity = \"baseline.input.db\"\n\t\t\tcase \"same-post-and-baseline\":\n\t\t\t\tevidence.Artifacts[SQLPostCopyArtifact].Before.SHA256, evidence.Artifacts[SQLPostCopyArtifact].After.SHA256 = evidence.Artifacts[SQLBaselineArtifact].Before.SHA256, evidence.Artifacts[SQLBaselineArtifact].Before.SHA256\n\t\t\tcase \"wrong-replay-input\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].Before.SHA256[0]++\n\t\t\tcase \"wrong-replay-output\":\n\t\t\t\tevidence.Artifacts[SQLReplayArtifact].After.SHA256[0]++\n\t\t\tcase \"same-old-and-new-hash\":\n\t\t\t\tfor _, role := range []SQLArtifactRole{SQLTrialArtifact, SQLPostCopyArtifact, SQLReplayArtifact} {\n\t\t\t\t\tevidence.Artifacts[role].Before.SHA256 = evidence.Artifacts[SQLBaselineArtifact].Before.SHA256\n\t\t\t\t\tevidence.Artifacts[role].After.SHA256 = evidence.Artifacts[SQLBaselineArtifact].Before.SHA256\n\t\t\t\t}\n\t\t\t}\n\t\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t\t}\n\t}\n\tfor _, name := range []string{\"original-journal-removed\", \"original-journal-changed\", \"journal-copy-removed\", \"journal-copy-changed\", \"replay-journal-missing\", \"replay-journal-survives\", \"replay-journal-aliased\", \"different-journal-copy\", \"different-replay-journal\"} {\n\t\tplan, receipt, evidence := sqlEvidenceFixture(t, SQLIndexPrefix64)\n\t\tswitch name {\n\t\tcase \"original-journal-removed\":\n\t\t\tevidence.Artifacts[SQLOriginalJournalArtifact].After = SQLArtifactStamp{}\n\t\tcase \"original-journal-changed\":\n\t\t\tevidence.Artifacts[SQLOriginalJournalArtifact].After.SHA256[0]++\n\t\tcase \"journal-copy-removed\":\n\t\t\tevidence.Artifacts[SQLJournalCopyArtifact].After = SQLArtifactStamp{}\n\t\tcase \"journal-copy-changed\":\n\t\t\tevidence.Artifacts[SQLJournalCopyArtifact].After.SHA256[0]++\n\t\tcase \"replay-journal-missing\":\n\t\t\tevidence.Artifacts[SQLReplayJournalArtifact].Before = SQLArtifactStamp{}\n\t\tcase \"replay-journal-survives\":\n\t\t\tevidence.Artifacts[SQLReplayJournalArtifact].After = evidence.Artifacts[SQLReplayJournalArtifact].Before\n\t\tcase \"replay-journal-aliased\":\n\t\t\tevidence.Artifacts[SQLReplayJournalArtifact].Before.Inode = evidence.Artifacts[SQLOriginalJournalArtifact].Before.Inode\n\t\tcase \"different-journal-copy\":\n\t\t\tevidence.Artifacts[SQLJournalCopyArtifact].Before.SHA256[0], evidence.Artifacts[SQLJournalCopyArtifact].After.SHA256[0] = 88, 88\n\t\tcase \"different-replay-journal\":\n\t\t\tevidence.Artifacts[SQLReplayJournalArtifact].Before.SHA256[0] = 88\n\t\t}\n\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t}\n\tplan, receipt, evidence := sqlEvidenceFixture(t, SQLControl)\n\tevidence.Recovery.Calls = 1\n\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))\n\tplan, receipt, evidence = sqlEvidenceFixture(t, SQLControl)\n\tevidence.Artifacts[SQLOriginalJournalArtifact].Before.Exists = true\n\tsqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))\n}\n\nfunc TestSQLLedgerSnapshotAdmissionRejectsBeforeCommitMismatches(t *testing.T) {\n\t_, _, evidence := sqlEvidenceFixture(t, SQLControl)\n\tif problem := ValidateSQLSnapshot(evidence.Before, evidence.Identity, SQLOld); problem != SQLProblemNone {\n\t\tt.Fatalf(\"independent old snapshot refused: %v\", problem)\n\t}\n\tif ValidateSQLSnapshot(evidence.Before, evidence.Identity, SQLStateUnknown) == SQLProblemNone {\n\t\tt.Fatal(\"unknown snapshot state admitted\")\n\t}\n\tif ValidateSQLSnapshot(evidence.Before, evidence.Identity, SQLNew) == SQLProblemNone {\n\t\tt.Fatal(\"old snapshot admitted as new\")\n\t}\n\tchanged := evidence.Before\n\tchanged.Identity.Trial = \"other-trial\"\n\tif ValidateSQLSnapshot(changed, evidence.Identity, SQLOld) == SQLProblemNone {\n\t\tt.Fatal(\"snapshot identity mismatch admitted\")\n\t}\n\tchanged = evidence.Before\n\tchanged.Owner = 0\n\tif ValidateSQLSnapshot(changed, evidence.Identity, SQLOld) == SQLProblemNone {\n\t\tt.Fatal(\"zero snapshot owner admitted\")\n\t}\n\tchanged = evidence.Before\n\tchanged.Queries[SQLRedIndex].Rows[0].ID = 9\n\tif ValidateSQLSnapshot(changed, evidence.Identity, SQLOld) == SQLProblemNone {\n\t\tt.Fatal(\"equal-count wrong index member admitted\")\n\t}\n}\n\nfunc TestSQLLedgerBindsSourceToolBinaryManifestTrialAndScopeEverywhere(t *testing.T) {\n\tfor _, where := range []string{\"top\", \"before\", \"after\", \"query\", \"check\", \"recovery\"} {\n\t\tfor _, field := range []string{\"suite\", \"source\", \"tool\", \"binary\", \"manifest\", \"trial\", \"scope\", \"profile\"} {\n\t\t\tplan, receipt, evidence := sqlEvidenceFixture(t, SQLControl)\n\t\t\tidentity := &evidence.Identity\n\t\t\tswitch where {\n\t\t\tcase \"before\":\n\t\t\t\tidentity = &evidence.Before.Identity\n\t\t\tcase \"after\":\n\t\t\t\tidentity = &evidence.After.Identity\n\t\t\tcase \"query\":\n\t\t\t\tidentity = &evidence.After.Queries[SQLBlueIndex].Identity\n\t\t\tcase \"check\":\n\t\t\t\tidentity = &evidence.After.Check.Identity\n\t\t\tcase \"recovery\":\n\t\t\t\tidentity = &evidence.Recovery.Identity\n\t\t\t}\n\t\t\tswitch field {\n\t\t\tcase \"suite\":\n\t\t\t\tidentity.Suite = \"another-suite\"\n\t\t\tcase \"source\":\n\t\t\t\tidentity.Source = \"another-source\"\n\t\t\tcase \"tool\":\n\t\t\t\tidentity.Tool = \"another-tool\"\n\t\t\tcase \"binary\":\n\t\t\t\tidentity.Binary = \"another-binary\"\n\t\t\tcase \"manifest\":\n\t\t\t\tidentity.Manifest = \"another-manifest\"\n\t\t\tcase \"trial\":\n\t\t\t\tidentity.Trial = \"another-trial\"\n\t\t\tcase \"scope\":\n\t\t\t\tidentity.Scope = \"another-scope\"\n\t\t\tcase \"profile\":\n\t\t\t\tidentity.Profile = ProfileUnknown\n\t\t\t}\n\t\t\tt.Run(where+\"/\"+field, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })\n\t\t}\n\t}\n}\n\nfunc TestSQLLedgerRevalidatesReceiptRatherThanTrustingAnObservation(t *testing.T) {\n\tfor _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore} {\n\t\tplan, _, evidence := sqlEvidenceFixture(t, which)\n\t\tevents := fixtureEvents(plan.Spec())\n\t\tfor end := 0; end < len(events); end++ {\n\t\t\tsqlMustReject(t, ObserveSQLLedger(plan, sqlReceipt(t, plan, events[:end]), evidence))\n\t\t}\n\t\tfor _, name := range []string{\"second-commit\", \"missing-owner\", \"owner-live\", \"wrong-commit\", \"missing-target-error\", \"target-normalization\", \"wrong-count\", \"physical-validation\", \"foreign-trial\"} {\n\t\t\tchanged := append([]Event(nil), events...)\n\t\t\tfor i := range changed {\n\t\t\t\tswitch name {\n\t\t\t\tcase \"second-commit\":\n\t\t\t\t\tif changed[i].Kind == CommitReturned {\n\t\t\t\t\t\tchanged = append(changed[:i+1], append([]Event{changed[i]}, changed[i+1:]...)...)\n\t\t\t\t\t\trenumber(changed, plan.Spec().Identity)\n\t\t\t\t\t\tgoto evaluate\n\t\t\t\t\t}\n\t\t\t\tcase \"missing-owner\":\n\t\t\t\t\tif changed[i].Kind == OwnersObserved {\n\t\t\t\t\t\tchanged[i].Owners.Main = OwnerUnknown\n\t\t\t\t\t}\n\t\t\t\tcase \"owner-live\":\n\t\t\t\t\tif changed[i].Kind == OwnersObserved {\n\t\t\t\t\t\tchanged[i].Owners.Main = OwnerLive\n\t\t\t\t\t}\n\t\t\t\tcase \"wrong-commit\":\n\t\t\t\t\tif changed[i].Kind == CommitReturned {\n\t\t\t\t\t\tchanged[i].Commit.Class = CommitUncertain\n\t\t\t\t\t}\n\t\t\t\tcase \"missing-target-error\":\n\t\t\t\t\tif changed[i].Kind == CommitReturned {\n\t\t\t\t\t\tchanged[i].Commit.TargetError = None\n\t\t\t\t\t\tif which == SQLControl {\n\t\t\t\t\t\t\tchanged[i].Commit.TargetError = ShortWrite\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n\t\t\t\tcase \"target-normalization\":\n\t\t\t\t\tif changed[i].Kind == Returned && changed[i].Call == plan.Spec().Target {\n\t\t\t\t\t\tchanged[i].Result.NormalizedError = None\n\t\t\t\t\t\tif which == SQLControl {\n\t\t\t\t\t\t\tchanged[i].Result.NormalizedError = ShortWrite\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n\t\t\t\tcase \"wrong-count\":\n\t\t\t\t\tif changed[i].Kind == Returned && changed[i].Call == plan.Spec().Target {\n\t\t\t\t\t\tchanged[i].Result.ReturnedN++\n\t\t\t\t\t}\n\t\t\t\tcase \"physical-validation\":\n\t\t\t\t\tif changed[i].Kind == ValidationObserved {\n\t\t\t\t\t\tchanged[i].Validation = Validation(1)\n\t\t\t\t\t}\n\t\t\t\tcase \"foreign-trial\":\n\t\t\t\t\tchanged[i].Identity.Trial = \"other-trial\"\n\t\t\t\t}\n\t\t\t}\n\t\tevaluate:\n\t\t\tt.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, sqlReceipt(t, plan, changed), evidence)) })\n\t\t}\n\t\tsqlMustReject(t, ObserveSQLLedger(Plan{}, Receipt{}, evidence))\n\t\totherPlan, _, _ := sqlEvidenceFixture(t, SQLControl)\n\t\tif which == SQLControl {\n\t\t\totherPlan, _, _ = sqlEvidenceFixture(t, SQLDBSyncBefore)\n\t\t}\n\t\tsqlMustReject(t, ObserveSQLLedger(plan, sqlReceipt(t, otherPlan, fixtureEvents(otherPlan.Spec())), evidence))\n\t}\n}\n",
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
		if owned && (value == "unsafe" || value == "reflect" || value == "os/exec" || strings.HasPrefix(value, "net") || value == "golang.org/x/sys/unix") {
			report("process/network/reflection import forbidden")
		}
		if path == rollbackFaultSQLBridge && value != "os" && value != rollbackFaultImport {
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
					case "len", "ManualRollbackSQLState", "armRollbackFault", "finishRollbackFault", "disarmRollbackFault":
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
		if path == rollbackFaultSQLBridge && called {
			name := readOnlyWALManualName(call.Fun)
			allowed := map[string]bool{"os.Getenv": true, "plainRetirements.mu.Lock": true, "plainRetirements.mu.Unlock": true, "g.pending.Load": true, "rollbackFaultRegistry.mu.Lock": true, "rollbackFaultRegistry.mu.Unlock": true, "tx.mu.Lock": true, "tx.mu.Unlock": true, "g.mu.Lock": true, "g.mu.Unlock": true, "plan.Spec": true, "rollbackfault.SQLPlan": true, "frozen.Spec": true}
			if !allowed[name] {
				report("bridge cannot call native owner/file/cleanup method " + name)
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
	wanted := map[string]bool{"ManualRollbackSQLState": true, "ArmManualRollbackSQLFault": true, "FinishManualRollbackSQLFault": true, "DisarmManualRollbackSQLFault": true}
	if len(functions) != len(wanted) {
		report("bridge exact four-function inventory required")
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
	require("ArmManualRollbackSQLFault", "bridge frozen three-case plan and healthy ownership", `switch spec.Action { case rollbackfault.Pass: caseID = rollbackfault.SQLControl; case rollbackfault.PrefixShortWrite: caseID = rollbackfault.SQLIndexPrefix64; case rollbackfault.FailBefore: caseID = rollbackfault.SQLDBSyncBefore; default: return nil,errRollbackFaultScope }`, `frozen, problem := rollbackfault.SQLPlan(spec.Identity,caseID)`, `frozen.Spec() != spec`, `state.Groups != 1 || state.Pending != 0 || !state.HealthyTx || state.AdapterActive || state.AdapterUsed`, `scope, err := armRollbackFault(tx,plan)`, `if err != nil { return nil,err }`, `return &manualRollbackSQLScope{scope:scope},nil`)
	require("ArmManualRollbackSQLFault", "exact actual three-page writer extent before arm", `if tx == nil { return nil,errRollbackFaultScope }`, `tx.originalPages == 3 && tx.header.PageCount == 3 && len(tx.dirty) == 3 && len(tx.dirty[1]) == 4096 && len(tx.dirty[2]) == 4096 && len(tx.dirty[3]) == 4096`, `if !threePages { return nil,errRollbackFaultScope }`)
	require("FinishManualRollbackSQLFault", "bridge exact untouched public error forwarding", `if scope == nil { return ManualRollbackSQLReport{Problem:errRollbackFaultScope} }`, `report := finishRollbackFault(scope.scope,commitErr)`, `return ManualRollbackSQLReport{Observation:report.Observation,Events:report.Events,EventCount:report.EventCount,Diagnostics:report.Diagnostics,CommitError:report.CommitError,Problem:report.Problem}`)
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
			if typ.Name.Name == "ManualRollbackSQLReport" && readOnlyWALManualCompact(typ) != readOnlyWALManualCompactText(`ManualRollbackSQLReport struct { Observation rollbackfault.Observation; Events [rollbackfault.MaxEvents]rollbackfault.Event; EventCount uint8; Diagnostics [rollbackfault.MaxEvents]error; CommitError error; Problem error }`) {
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
	root := readOnlyWALManualBody(functions, rollbackFaultSQLRoot)
	if root == nil || len(root.List) == 0 || readOnlyWALManualCompact(root.List[0]) != readOnlyWALManualCompactText(`if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" { t.Fatal("NOT_RUN: separate execution authorization, reviewed provenance and private retained root required") }`) {
		report("root first action must be exact rejecting environment gate")
	}
	commit := readOnlyWALManualBody(functions, "manualRollbackSQLCommit")
	if readOnlyWALManualSequence(commit, `commitErr := tx.Commit(ctx)`, `report := pager.FinishManualRollbackSQLFault(scope,commitErr)`) < 0 {
		report("public Commit must be immediately followed by Finish with untouched error")
	}
	require("manualRollbackSQLCommit", "public error identity, owners and classification", `if !manualRollbackSQLRegistryEmpty()`, `db, owner, err := manualRollbackSQLOpen(c,path,false)`, `tx, beginErr := db.Begin(ctx,TxOptions{Immediate:true})`, `"UPDATE t SET k='green',v='ONE' WHERE id=1"`, `"DELETE FROM t WHERE id=2"`, `"INSERT INTO t(id,k,v) VALUES(4,'blue','four')"`, `countErr != nil || count != 1`, `scope, armErr := pager.ArmManualRollbackSQLFault(tx.storage,plan)`, `report.CommitError != commitErr || report.Problem != nil || report.Observation.Outcome != rollbackfault.AdapterObserved || report.Observation.TargetHits != 1 || report.Observation.Pass() || report.Observation.PhysicalValidated() || !tx.done || db.tx != nil || tx.activeRows != 0`, `!errors.As(commitErr,&publicError) || publicError.Code != CodeIOErr || !errors.Is(commitErr,ErrRecoveryRequired) || errors.Is(commitErr,pager.ErrPlaintextCleanupPending) || errors.Is(commitErr,pager.ErrCommitUncertain)`, `plan.Spec().Action == rollbackfault.PrefixShortWrite && !errors.Is(commitErr,io.ErrShortWrite)`, `if err := pager.DisarmManualRollbackSQLFault(scope); err != nil { return report,err }`, `if err := c.ledger.close(owner); err != nil { return report,err }`, `if !manualRollbackSQLRegistryEmpty()`)
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
	require("manualRollbackSQLAdmit", "one exact fresh-process selection and reviewed provenance", `"-test.run":"^TestManualRollbackFaultSQLLedger$"`, `"-test.count":"1"`, `"-test.shuffle":"off"`, `"-test.parallel":"1"`, `"-test.timeout":"0"`, `"-test.cpu":"1"`, `"-test.v":"true"`, `if !found || seen[key]`, `if expected, ok := want[key]; !ok || value != expected`, `if !seen[key]`, `case "control":`, `case "index-prefix-64":`, `case "db-sync-before":`, `default:`, `receipt.Schema != "rollback-fault-sql-compile-v1"`, `receipt.Package != "./sqlite" || receipt.Tags != "manual_fault_rollback" || !receipt.CompileOnly`, `sha256.Sum256(image) != binaryHash`)
	require(rollbackFaultSQLRoot, "root independent baseline and copy-only recovery order", `rollbackfault.ValidateSQLSnapshot(before,identity,rollbackfault.SQLOld)`, `manualRollbackSQLCommit(&c,ctx,filepath.Join(c.root,"trial.db"),plan)`, `receipt.Seal() != report.Observation`, `if c.caseID == rollbackfault.SQLIndexPrefix64 && !bytes.Equal(post[8192+64:],baseline[8192+64:])`, `c.retain("trial.db",trialInfo,12288)`, `c.create("post-commit.input.db",post,true)`, `c.retain("trial.db-journal",nil,16408)`, `c.create("post-commit-journal.input.bin",journalBytes,true)`, `c.create("replay.db",post,false)`, `os.SameFile(trialInfo,replayInfo)`, `manualRollbackSQLCapture(&c,&evidence,false)`, `manualRollbackSQLRecoverCopy(ctx,filepath.Join(c.root,"replay.db"))`, `evidence.Recovery.Calls++`, `manualRollbackSQLReadback(&c,ctx,filepath.Join(c.root,"replay.db"),identity)`, `want := rollbackfault.SQLOld`, `if c.caseID == rollbackfault.SQLControl { want = rollbackfault.SQLNew }`, `rollbackfault.ValidateSQLSnapshot(after,identity,want)`, `manualRollbackSQLCapture(&c,&evidence,true)`, `evidence.Originals.Frozen = frozenHashes`, `manualRollbackSQLPageHashes(oldBefore)`, `manualRollbackSQLPageHashes(oldAfter)`, `!c.ledger.retired() || !manualRollbackSQLRegistryEmpty()`, `rollbackfault.ObserveSQLLedger(plan,receipt,evidence)`, `!observation.EvidenceConsistent || !observation.ScenarioObserved || observation.Problem != rollbackfault.SQLProblemNone || observation.PhysicalValidated() || observation.Pass()`)
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
		if !found || readOnlyWALManualCompact(conditional.Cond) != "c.caseID!=rollbackfault.SQLControl" || conditional.Else != nil || parents[conditional] != root {
			report("control may never execute recovery; one fault-case branch required")
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
	for _, name := range []string{"PhysicalValidated", "Pass"} {
		fn := functions[name]
		if fn == nil || readOnlyWALManualCompact(fn) != readOnlyWALManualCompactText("func (SQLObservation) "+name+"() bool { return false }") {
			report("SQL observation cannot confer physical validation or PASS")
		}
	}
	require("SQLLedgerRows", "independent literal old/new ledger", `case SQLOld: return [3]SQLRow{{SQLInteger,1,SQLText,"red",SQLText,"one"},{SQLInteger,2,SQLText,"blue",SQLText,"two"},{SQLInteger,3,SQLText,"red",SQLText,"three"}}`, `case SQLNew: return [3]SQLRow{{SQLInteger,1,SQLText,"green",SQLText,"ONE"},{SQLInteger,3,SQLText,"red",SQLText,"three"},{SQLInteger,4,SQLText,"blue",SQLText,"four"}}`, `default: return [3]SQLRow{}`)
	require("SQLQueryText", "seven independent scan/index statements", `case SQLAllRows: return "SELECT id,k,v FROM t NOT INDEXED ORDER BY id"`, `case SQLBlueIndex: return "SELECT id,k,v FROM t WHERE k='blue' ORDER BY id"`, `case SQLBlueScan: return "SELECT id,k,v FROM t NOT INDEXED WHERE k='blue' ORDER BY id"`, `case SQLRedIndex: return "SELECT id,k,v FROM t WHERE k='red' ORDER BY id"`, `case SQLRedScan: return "SELECT id,k,v FROM t NOT INDEXED WHERE k='red' ORDER BY id"`, `case SQLGreenIndex: return "SELECT id,k,v FROM t WHERE k='green' ORDER BY id"`, `case SQLGreenScan: return "SELECT id,k,v FROM t NOT INDEXED WHERE k='green' ORDER BY id"`)
	require("SQLPlan", "frozen three-page three-case selectors", `OriginalPages:3,DirtyPages:3,PageSize:SQLPageSize,SectorSize:SQLPageSize`, `{Point:DBPage,Occurrence:3,Role:Database,Owner:11,Offset:8192,Length:4096}`, `CallCount:18`, `spec.Target = spec.Calls[14]`, `case SQLControl:`, `case SQLIndexPrefix64: spec.Action,spec.Prefix,spec.PrefixError = PrefixShortWrite,64,false; spec.ExpectedCommit,spec.CallCount = CommitRecoveryRequired,15`, `case SQLDBSyncBefore: spec.Target,spec.Action = spec.Calls[15],FailBefore; spec.ExpectedCommit,spec.CallCount = CommitRecoveryRequired,16`, `default: return Plan{},ProblemTarget`, `for i := int(spec.CallCount); i < MaxCalls; i++ { spec.Calls[i] = Call{} }`, `return NewPlan(spec)`)
	require("ObserveSQLLedger", "receipt identity and old/new mixtures rejected", `if evidence.PhysicalClaim`, `problem != ProblemNone || !plan.valid || plan.spec != expectedPlan.spec`, `!receipt.plan.valid || receipt.plan.spec != plan.spec`, `result.Adapter = receipt.Seal()`, `result.Adapter.Outcome != AdapterObserved || result.Adapter.Problem != ProblemNone || result.Adapter.TargetHits != 1`, `evidence.Identity != plan.spec.Identity || evidence.Before.Identity != evidence.Identity || evidence.After.Identity != evidence.Identity || evidence.Recovery.Identity != evidence.Identity`, `if evidence.Lifecycle != lifecycle`, `if !validSQLReaderOwners(plan.spec,evidence.Before.Owner,evidence.After.Owner)`, `validateSQLSnapshot(evidence.Before,evidence.Identity,SQLOld)`, `state := SQLOld`, `if evidence.Case == SQLControl { state = SQLNew }`, `validateSQLSnapshot(evidence.After,evidence.Identity,state)`, `if before.Iterator == after.Iterator`, `original.BeforeBytes != SQLDatabaseBytes || original.AfterBytes != SQLDatabaseBytes || original.Frozen != original.Before || original.Frozen != original.After`, `if digest == ([32]byte{})`, `if !validSQLRecovery(evidence.Case,evidence.Recovery)`, `if !validSQLArtifacts(evidence.Case,evidence.Artifacts)`)
	require("ObserveSQLLedger", "one-shot public error lifecycle", `lifecycle := SQLLifecycleEvidence{ArmCalls:1,CommitCalls:1,FinishCalls:1,DisarmCalls:1,WriterOpens:1,ReopenCalls:1,ReopenReadOnly:true,WriterRetiredBeforeRecovery:true,RecoveryFinishedBeforeReopen:true,CommitCapturedUnchanged:true}`, `if evidence.Case != SQLControl`, `lifecycle.PublicCodeIOErr,lifecycle.PublicRecoveryRequired = true,true`, `lifecycle.PublicShortWrite = evidence.Case == SQLIndexPrefix64`)
	require("validateSQLSnapshot", "full rows, Check, EOF and exact closure", `snapshot.CloseCalls != 1 || snapshot.CloseError != None || snapshot.CloseErrorToken != 0 || snapshot.OwnerState != OwnerRetired`, `check.Calls != 1 || check.Error != None || check.ErrorToken != 0 || check.Pages != 3 || check.FreePages != 0 || check.ReservedPages != 0 || check.Tables != 1 || check.Indexes != 1 || check.ExtensionIndexes != 0 || check.TableRows != 3 || check.IndexEntries != 3`, `query.Query != SQLQuery(i) || query.SQL != SQLQueryText(SQLQuery(i))`, `!query.EOF || !query.ErrCalled || query.Error != None || query.ErrorToken != 0`, `query.CloseCalls != 1 || query.CloseError != None || query.CloseErrorToken != 0`, `rows,count := expectedSQLQueryRows(state,query.Query)`, `query.RowCount != count || query.Rows != rows`, `!validSQLQueryPlan(query.Query,query.Plan)`, `!validSQLQueryWork(query.Query,query.Work,count)`)
	require("validSQLQueryPlan", "by_k plan and independent NOT INDEXED plan", `plan.Access == "SEARCH" && plan.Method == "btree" && plan.Index == "by_k" && plan.IndexCount == 1 && plan.Field == "k" && plan.FieldCount == 1 && plan.Recheck`, `plan.Access == "SCAN" && plan.Method == "" && plan.Index == "" && plan.IndexCount == 0 && plan.Field == "" && plan.FieldCount == 0 && !plan.Recheck && plan.Reason == "NOT INDEXED requested"`)
	require("validSQLQueryWork", "index claim requires actual bounded work", `if work.ReturnedRows != n`, `work.IndexPageReads > 0 && work.IndexPageReads <= 32 && work.IndexKeysRead == n && work.Candidates == n && work.RowIDSeeks == n && work.BaseRowFetches == n`, `work.IndexKeysRead == 0 && work.IndexPageReads == 0 && work.RowIDSeeks == 0 && work.Candidates == 0 && work.BaseRowFetches == 3`)
	require("validSQLRecovery", "control no recovery; fault exact one bounded replay", `if which == SQLControl`, `return recovery == (SQLRecoveryEvidence{Identity:recovery.Identity,OwnersRetired:true})`, `recovery.Calls != 1 || recovery.Error != None || recovery.ErrorToken != 0 || !recovery.OwnersRetired || recovery.JournalPresentAfter || recovery.Limits != (SQLRecoveryLimits{MaxJournalBytes:SQLJournalBytes,MaxRecords:SQLPageCount,MaxDatabaseBytes:SQLDatabaseBytes})`, `return recovery.JournalPresentBefore && recovery.Replayed && recovery.ReplayedPages == SQLPageCount && recovery.OriginalPages == SQLPageCount`)
	require("validSQLArtifacts", "immutable originals and unaliased replay artifacts", `before.Identity != SQLArtifactName(artifact.Role)`, `artifact.Role == SQLReplayJournalArtifact`, `after != (SQLArtifactStamp{})`, `before.Device != after.Device || before.Inode != after.Inode`, `artifact.Role != SQLReplayArtifact && before != after`, `before.Identity == other.Identity || (before.Device == other.Device && before.Inode == other.Inode)`, `baseline == trial || trial != post || trial != replayBefore`, `return replayAfter == trial`, `return replayAfter == baseline && journal == artifacts[SQLJournalCopyArtifact].Before.SHA256 && journal == artifacts[SQLReplayJournalArtifact].Before.SHA256`)
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
