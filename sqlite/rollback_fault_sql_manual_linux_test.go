//go:build linux && manual_fault_rollback

package sqlite

// SOURCE ONLY / REVIEW HOLD / PHYSICAL NOT_RUN. One fresh process, one case.
// A private stable namespace is required; metadata rechecks are not protection
// against hostile concurrent name replacement. Every input and output remains.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/journal"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"
)

type manualRollbackSQLOwner struct {
	file     *os.File
	db       *DB
	rows     *Rows
	path     string
	closes   uint8
	closeErr error
}

type manualRollbackSQLLedger struct {
	owners [256]manualRollbackSQLOwner
	count  int
}

func (l *manualRollbackSQLLedger) reserve() (int, error) {
	if l.count == len(l.owners) {
		return 0, fmt.Errorf("owner ledger full")
	}
	i := l.count
	l.count++
	return i, nil
}

func (l *manualRollbackSQLLedger) close(i int) error {
	o := &l.owners[i]
	if o.closes != 0 {
		return fmt.Errorf("duplicate close refused: %s", o.path)
	}
	if o.file == nil && o.db == nil && o.rows == nil {
		return nil
	}
	// Never turn a DB close into a hidden rollback/cleanup retry.
	if o.db != nil && (o.db.tx != nil || len(o.db.rows) != 0) {
		return fmt.Errorf("DB has live transaction/Rows; retained without retry")
	}
	o.closes++
	if o.file != nil {
		o.closeErr = o.file.Close()
	} else if o.rows != nil {
		o.closeErr = o.rows.Close()
	} else {
		o.closeErr = o.db.Close()
	}
	return o.closeErr
}

func (l *manualRollbackSQLLedger) retired() bool {
	for i := 0; i < l.count; i++ {
		o := l.owners[i]
		if (o.file != nil || o.db != nil || o.rows != nil) && (o.closes != 1 || o.closeErr != nil) {
			return false
		}
	}
	return true
}

func (l *manualRollbackSQLLedger) finish(t *testing.T) {
	// Reporting only: every acquired owner is retired explicitly at its one
	// intended boundary. A failure must not trigger a deferred cleanup retry.
	for i := 0; i < l.count; i++ {
		o := l.owners[i]
		if (o.file != nil || o.db != nil || o.rows != nil) && (o.closes != 1 || o.closeErr != nil) {
			t.Errorf("INCOMPLETE: retained owner %d %s closes=%d error=%v; no retry", i, o.path, o.closes, o.closeErr)
		}
	}
}

type manualRollbackSQLBinding struct {
	Source             string
	Tool               string
	Binary             string
	Manifest           string
	CompileReceipt     string
	Trial              string
	Scope              string
	Case               string
	ToolPath           string
	SourceManifestPath string
	CompileReceiptPath string
}

// An external reviewed compile receipt is a bounded evidence input, not an
// executable command or a self-authenticating approval. Its exact bytes must
// have been reviewed independently and its hash supplied by that review.
type manualRollbackSQLCompile struct {
	Schema      string
	Source      string
	Tool        string
	Binary      string
	Manifest    string
	Package     string
	Tags        string
	CompileOnly bool
	Files       uint16
	SourceTree  string
}

type manualRollbackSQLConfig struct {
	root          string
	rootInfo      os.FileInfo
	caseID        rollbackfault.SQLCase
	binding       manualRollbackSQLBinding
	bindingBytes  []byte
	sourceFiles   uint16
	ledger        manualRollbackSQLLedger
	retained      [12]manualRollbackSQLSaved
	retainedCount int
}

type manualRollbackSQLSaved struct {
	name string
	info os.FileInfo
	hash [32]byte
	size int64
}

func manualRollbackSQLHash(value string) ([32]byte, error) {
	var out [32]byte
	if len(value) != 64 {
		return out, fmt.Errorf("SHA-256 requires 64 lowercase hex bytes")
	}
	data, err := hex.DecodeString(value)
	if err != nil || hex.EncodeToString(data) != value {
		return out, fmt.Errorf("invalid SHA-256")
	}
	copy(out[:], data)
	if out == ([32]byte{}) {
		return out, fmt.Errorf("zero SHA-256")
	}
	return out, nil
}

func manualRollbackSQLNamespace(root string) (os.FileInfo, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || root == "/" {
		return nil, fmt.Errorf("absolute clean private root required")
	}
	current := string(filepath.Separator)
	for _, component := range strings.Split(strings.TrimPrefix(root, "/"), "/") {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, fmt.Errorf("symlink/non-directory component: %s", current)
		}
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	metadata, ok := info.Sys().(*syscall.Stat_t)
	if !ok || metadata.Uid != uint32(os.Getuid()) {
		return nil, fmt.Errorf("private root must belong to the invoking user")
	}
	if info.Mode().Perm() != 0700 {
		return nil, fmt.Errorf("precreated private root must already have mode 0700; no permission changes are made")
	}
	return info, nil
}

func (c *manualRollbackSQLConfig) stable() error {
	info, err := manualRollbackSQLNamespace(c.root)
	if err != nil {
		return err
	}
	if !os.SameFile(info, c.rootInfo) {
		return fmt.Errorf("root namespace changed; retain everything")
	}
	return nil
}

func manualRollbackSQLRead(l *manualRollbackSQLLedger, path string, limit int64) ([]byte, os.FileInfo, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > limit {
		return nil, nil, fmt.Errorf("bounded regular input required: %s", path)
	}
	i, err := l.reserve()
	if err != nil {
		return nil, nil, err
	}
	f, openErr := os.Open(path)
	l.owners[i] = manualRollbackSQLOwner{file: f, path: path}
	if openErr != nil {
		return nil, nil, errors.Join(openErr, l.close(i))
	}
	actual, statErr := f.Stat()
	if statErr != nil || !os.SameFile(before, actual) || actual.Size() != before.Size() {
		return nil, nil, errors.Join(fmt.Errorf("input changed on open: %s", path), statErr, l.close(i))
	}
	data, readErr := io.ReadAll(io.LimitReader(f, limit+1))
	after, afterErr := f.Stat()
	closeErr := l.close(i)
	pathAfter, pathErr := os.Lstat(path)
	if err := errors.Join(readErr, afterErr, closeErr, pathErr); err != nil {
		return nil, nil, err
	}
	if int64(len(data)) != before.Size() || !os.SameFile(before, after) || !os.SameFile(before, pathAfter) || after.Size() != before.Size() || pathAfter.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || !pathAfter.ModTime().Equal(before.ModTime()) {
		return nil, nil, fmt.Errorf("input changed during bounded read: %s", path)
	}
	return data, before, nil
}

func manualRollbackSQLDecode(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var tail any
	if decoder.Decode(&tail) != io.EOF {
		return fmt.Errorf("trailing JSON value")
	}
	return nil
}

func manualRollbackSQLAdmit(c *manualRollbackSQLConfig) error {
	// Exact flags are intentional. No regex union, repeated test, subtest,
	// random order, competing test, timeout kill, or wrapper is admitted.
	want := map[string]string{
		"-test.run": "^TestManualRollbackFaultSQLLedger$", "-test.count": "1",
		"-test.shuffle": "off", "-test.parallel": "1", "-test.timeout": "0", "-test.cpu": "1",
		"-test.v": "true", // The unique successful terminal uses t.Logf.
	}
	seen := map[string]bool{}
	for _, arg := range os.Args[1:] {
		key, value, found := strings.Cut(arg, "=")
		if !found || seen[key] {
			return fmt.Errorf("explicit unique equals-form test flags required")
		}
		seen[key] = true
		if expected, ok := want[key]; !ok || value != expected {
			return fmt.Errorf("unreviewed test flag: %s", arg)
		}
	}
	for key := range want {
		if !seen[key] {
			return fmt.Errorf("missing exact flag: %s", key)
		}
	}
	if os.Getenv("GOSQLITE_ROLLBACK_SQL_PRIVATE_ROOT_CONFIRMED") != "private_stable_namespace_retained" {
		return fmt.Errorf("private stable namespace and retention must be explicitly confirmed")
	}
	c.root = os.Getenv("GOSQLITE_ROLLBACK_SQL_ROOT")
	info, err := manualRollbackSQLNamespace(c.root)
	if err != nil {
		return err
	}
	c.rootInfo = info
	i, err := c.ledger.reserve()
	if err != nil {
		return err
	}
	dir, openErr := os.Open(c.root)
	c.ledger.owners[i] = manualRollbackSQLOwner{file: dir, path: c.root}
	if openErr != nil {
		return errors.Join(openErr, c.ledger.close(i))
	}
	entries, readErr := dir.ReadDir(1)
	closeErr := c.ledger.close(i)
	if readErr != io.EOF || len(entries) != 0 || closeErr != nil {
		return errors.Join(fmt.Errorf("root must be empty before admission"), readErr, closeErr)
	}
	bindingPath := os.Getenv("GOSQLITE_ROLLBACK_SQL_BINDING")
	bindingHash, err := manualRollbackSQLHash(os.Getenv("GOSQLITE_ROLLBACK_SQL_BINDING_SHA256"))
	if err != nil {
		return err
	}
	raw, _, err := manualRollbackSQLRead(&c.ledger, bindingPath, 65536)
	if err != nil {
		return err
	}
	if sha256.Sum256(raw) != bindingHash {
		return fmt.Errorf("independently reviewed binding hash mismatch")
	}
	if err := manualRollbackSQLDecode(raw, &c.binding); err != nil {
		return err
	}
	c.bindingBytes = raw
	switch c.binding.Case {
	case "control":
		c.caseID = rollbackfault.SQLControl
	case "index-prefix-64":
		c.caseID = rollbackfault.SQLIndexPrefix64
	case "db-sync-before":
		c.caseID = rollbackfault.SQLDBSyncBefore
	case "index-prefix-64-error":
		c.caseID = rollbackfault.SQLIndexPrefix64Error
	case "jbody-sync-before":
		c.caseID = rollbackfault.SQLJournalBodySyncBefore
	case "jheader-sync-before":
		c.caseID = rollbackfault.SQLJournalHeaderSyncBefore
	case "publish-dirsync-before":
		c.caseID = rollbackfault.SQLPublishDirectorySyncBefore
	case "junlink-before":
		c.caseID = rollbackfault.SQLJournalUnlinkBefore
	case "final-dirsync-before":
		c.caseID = rollbackfault.SQLFinalDirectorySyncBefore
	default:
		return fmt.Errorf("exactly one of nine fixed cases required")
	}
	manifest, _, err := manualRollbackSQLRead(&c.ledger, c.binding.SourceManifestPath, 4<<20)
	if err != nil {
		return err
	}
	manifestHash, err := manualRollbackSQLHash(c.binding.Manifest)
	if err != nil || sha256.Sum256(manifest) != manifestHash || c.binding.Source != c.binding.Manifest {
		return fmt.Errorf("frozen source inventory digest mismatch: %v", err)
	}
	var inventory struct {
		Files map[string]struct {
			Bytes   int64  `json:"bytes"`
			GitBlob string `json:"git_blob"`
			GitMode string `json:"git_mode"`
			SHA256  string `json:"sha256"`
		} `json:"files"`
		Tree string `json:"tree"`
	}
	if err := manualRollbackSQLDecode(manifest, &inventory); err != nil {
		return err
	}
	if len(inventory.Files) == 0 || len(inventory.Files) > 4096 || len(inventory.Tree) != 40 {
		return fmt.Errorf("finite reviewed source inventory required")
	}
	for path, entry := range inventory.Files {
		if filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || strings.HasPrefix(path, "../") || entry.Bytes < 0 || (entry.GitMode != "100644" && entry.GitMode != "100755") || len(entry.GitBlob) != 40 {
			return fmt.Errorf("invalid reviewed source inventory entry")
		}
		if _, err := manualRollbackSQLHash(entry.SHA256); err != nil {
			return err
		}
	}
	c.sourceFiles = uint16(len(inventory.Files))
	compileRaw, _, err := manualRollbackSQLRead(&c.ledger, c.binding.CompileReceiptPath, 65536)
	if err != nil {
		return err
	}
	compileHash, err := manualRollbackSQLHash(c.binding.CompileReceipt)
	if err != nil || sha256.Sum256(compileRaw) != compileHash {
		return fmt.Errorf("independently reviewed compile receipt digest mismatch: %v", err)
	}
	var receipt manualRollbackSQLCompile
	if err := manualRollbackSQLDecode(compileRaw, &receipt); err != nil {
		return err
	}
	if receipt.Schema != "rollback-fault-sql-compile-v1" || receipt.Source != c.binding.Source || receipt.Tool != c.binding.Tool || receipt.Binary != c.binding.Binary || receipt.Manifest != c.binding.Manifest || receipt.Package != "./sqlite" || receipt.Tags != "manual_fault_rollback" || !receipt.CompileOnly || receipt.Files != c.sourceFiles || receipt.SourceTree != inventory.Tree {
		return fmt.Errorf("source-to-binary compile receipt does not bind this trial")
	}
	tool, _, err := manualRollbackSQLRead(&c.ledger, c.binding.ToolPath, 128<<20)
	if err != nil {
		return err
	}
	toolHash, err := manualRollbackSQLHash(c.binding.Tool)
	if err != nil || sha256.Sum256(tool) != toolHash {
		return fmt.Errorf("tool digest mismatch: %v", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	image, _, err := manualRollbackSQLRead(&c.ledger, executable, 128<<20)
	if err != nil {
		return err
	}
	binaryHash, err := manualRollbackSQLHash(c.binding.Binary)
	if err != nil || sha256.Sum256(image) != binaryHash {
		return fmt.Errorf("actual executable digest mismatch: %v", err)
	}
	if _, err := manualRollbackSQLHash(c.binding.Source); err != nil {
		return err
	}
	return c.stable()
}

func (c *manualRollbackSQLConfig) create(name string, data []byte, retain bool) (os.FileInfo, error) {
	if err := c.stable(); err != nil {
		return nil, err
	}
	if filepath.Base(name) != name || name == "." || len(data) == 0 || len(data) > 1<<20 {
		return nil, fmt.Errorf("fixed bounded basename required")
	}
	path := filepath.Join(c.root, name)
	i, err := c.ledger.reserve()
	if err != nil {
		return nil, err
	}
	f, openErr := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	c.ledger.owners[i] = manualRollbackSQLOwner{file: f, path: path}
	if openErr != nil {
		return nil, errors.Join(openErr, c.ledger.close(i))
	}
	info, statErr := f.Stat()
	if statErr != nil {
		return nil, errors.Join(statErr, c.ledger.close(i))
	}
	for n := 0; n < c.retainedCount; n++ {
		if os.SameFile(info, c.retained[n].info) {
			return nil, errors.Join(fmt.Errorf("retained input alias"), c.ledger.close(i))
		}
	}
	n, writeErr := f.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	var syncErr error
	if writeErr == nil {
		syncErr = f.Sync()
	}
	closeErr := c.ledger.close(i)
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return nil, err
	}
	actual, actualInfo, err := manualRollbackSQLRead(&c.ledger, path, int64(len(data)))
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, actualInfo) || !bytes.Equal(actual, data) {
		return nil, fmt.Errorf("created bytes or identity changed")
	}
	if retain {
		if c.retainedCount == len(c.retained) {
			return nil, fmt.Errorf("retained inventory full")
		}
		c.retained[c.retainedCount] = manualRollbackSQLSaved{name: name, info: actualInfo, hash: sha256.Sum256(actual), size: int64(len(actual))}
		c.retainedCount++
	}
	return actualInfo, c.stable()
}

func (c *manualRollbackSQLConfig) retainedUnchanged() error {
	if err := c.stable(); err != nil {
		return err
	}
	for i := 0; i < c.retainedCount; i++ {
		saved := c.retained[i]
		raw, info, err := manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, saved.name), saved.size)
		if err != nil {
			return err
		}
		if !os.SameFile(info, saved.info) || info.Size() != saved.size || sha256.Sum256(raw) != saved.hash {
			return fmt.Errorf("retained input changed: %s", saved.name)
		}
	}
	return nil
}

// The baseline is authored directly from the frozen independent old ledger.
// There is no sqlite.Open(create), bootstrap SQL, initial Commit, fixture reader,
// oracle, generator process, or inference from the damaged database.
func manualRollbackSQLBaseline() ([12288]byte, error) {
	var image [12288]byte
	copy(image[:], "SQLite format 3\x00")
	binary.BigEndian.PutUint16(image[16:], 4096)
	image[18], image[19], image[21], image[22], image[23] = 1, 1, 64, 32, 32
	binary.BigEndian.PutUint32(image[24:], 7)
	binary.BigEndian.PutUint32(image[28:], 3)
	binary.BigEndian.PutUint32(image[40:], 9)
	binary.BigEndian.PutUint32(image[44:], 4)
	binary.BigEndian.PutUint32(image[56:], 1)
	binary.BigEndian.PutUint32(image[92:], 7)
	binary.BigEndian.PutUint32(image[96:], 3045000)
	schema := [][]any{
		{"table", "t", "t", int64(2), "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT,v TEXT)"},
		{"index", "by_k", "t", int64(3), "CREATE INDEX by_k ON t(k)"},
	}
	table := [][]any{{nil, "red", "one"}, {nil, "blue", "two"}, {nil, "red", "three"}}
	index := [][]any{{"blue", int64(2)}, {"red", int64(1)}, {"red", int64(3)}}
	if err := manualRollbackSQLLeaf(image[:4096], 100, 13, schema, []byte{1, 2}); err != nil {
		return image, err
	}
	if err := manualRollbackSQLLeaf(image[4096:8192], 0, 13, table, []byte{1, 2, 3}); err != nil {
		return image, err
	}
	if err := manualRollbackSQLLeaf(image[8192:], 0, 10, index, nil); err != nil {
		return image, err
	}
	return image, nil
}

// This deliberately supports only the bounded scalar record shapes above.
func manualRollbackSQLRecord(values []any) ([]byte, error) {
	header := []byte{byte(len(values) + 1)}
	var body []byte
	for _, value := range values {
		switch v := value.(type) {
		case nil:
			header = append(header, 0)
		case int64:
			if v < 1 || v > 4 {
				return nil, fmt.Errorf("fixture integer outside frozen range")
			}
			header = append(header, 1)
			body = append(body, byte(v))
		case string:
			if len(v) > 57 {
				return nil, fmt.Errorf("fixture text outside single-byte serial bound")
			}
			header = append(header, byte(13+2*len(v)))
			body = append(body, v...)
		default:
			return nil, fmt.Errorf("fixture scalar type outside fixed builder")
		}
	}
	record := append(header, body...)
	if len(record) >= 128 {
		return nil, fmt.Errorf("fixture payload outside single-byte length bound")
	}
	return record, nil
}

func manualRollbackSQLLeaf(page []byte, header int, kind byte, rows [][]any, ids []byte) error {
	if len(page) != 4096 || len(rows) > 3 || (kind == 13 && len(ids) != len(rows)) || (kind != 10 && kind != 13) {
		return fmt.Errorf("fixed leaf shape required")
	}
	page[header] = kind
	binary.BigEndian.PutUint16(page[header+3:], uint16(len(rows)))
	end := len(page)
	for i, values := range rows {
		record, err := manualRollbackSQLRecord(values)
		if err != nil {
			return err
		}
		cell := []byte{byte(len(record))}
		if kind == 13 {
			cell = append(cell, ids[i])
		}
		cell = append(cell, record...)
		end -= len(cell)
		if end < header+8+2*len(rows) {
			return fmt.Errorf("fixed leaf overflow")
		}
		copy(page[end:], cell)
		binary.BigEndian.PutUint16(page[header+8+2*i:], uint16(end))
	}
	binary.BigEndian.PutUint16(page[header+5:], uint16(end))
	return nil
}

func manualRollbackSQLRegistryEmpty() bool {
	s := pager.ManualRollbackSQLState(nil)
	return s.Groups == 0 && s.Pending == 0 && !s.AdapterActive
}

func manualRollbackSQLOpen(c *manualRollbackSQLConfig, path string, readonly bool) (*DB, int, error) {
	i, err := c.ledger.reserve()
	if err != nil {
		return nil, 0, err
	}
	db, openErr := Open(path, Options{ReadOnly: readonly, NoCreate: true, PageSize: 4096, SectorSize: 4096, MaxTransactionPages: 3, MaxTransactionUndoBytes: 65536, MaxSortBytes: 65536})
	c.ledger.owners[i] = manualRollbackSQLOwner{db: db, path: path}
	if openErr != nil {
		return db, i, errors.Join(openErr, c.ledger.close(i))
	}
	if db == nil || db.session != nil {
		return db, i, fmt.Errorf("independent plain DB required")
	}
	return db, i, nil
}

// Retirement on a preparation error is allowed only before adapter ownership
// transfer and only for the original healthy independent transaction. An
// unknown, started or pending cleanup is retained, never retried by DB.Close.
func manualRollbackSQLAbandon(tx *Tx, cause error) error {
	if tx == nil || tx.done {
		return cause
	}
	state := pager.ManualRollbackSQLState(tx.storage)
	if state.AdapterActive || state.AdapterUsed || state.Pending != 0 || !state.HealthyTx {
		return cause
	}
	return errors.Join(cause, tx.Rollback())
}

func manualRollbackSQLCommit(c *manualRollbackSQLConfig, ctx context.Context, path string, plan rollbackfault.Plan) (pager.ManualRollbackSQLReport, error) {
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
}

func manualRollbackSQLRecoverCopy(ctx context.Context, replayPath string) (pager.RecoveryResult, error) {
	if filepath.Base(replayPath) != "replay.db" || !manualRollbackSQLRegistryEmpty() {
		return pager.RecoveryResult{}, fmt.Errorf("fresh replay path and empty registries required")
	}
	result, recoveryErr := pager.RecoverRollback(ctx, replayPath, pager.RecoveryOptions{Limits: journal.ReplayLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}})
	if recoveryErr != nil {
		return result, recoveryErr
	}
	if !manualRollbackSQLRegistryEmpty() {
		return result, fmt.Errorf("recovery owner retirement incomplete")
	}
	if !result.Recovered || result.RestoredPages != 3 || result.OriginalPages != 3 {
		return result, fmt.Errorf("exact three-page recovery required")
	}
	return result, nil
}
func manualRollbackSQLQuery(c *manualRollbackSQLConfig, ctx context.Context, db *DB, owner uint64, identity rollbackfault.Identity, query rollbackfault.SQLQuery) (rollbackfault.SQLQueryEvidence, error) {
	evidence := rollbackfault.SQLQueryEvidence{Identity: identity, Query: query, Owner: owner, SQL: rollbackfault.SQLQueryText(query)}
	i, err := c.ledger.reserve()
	if err != nil {
		return evidence, err
	}
	rows, queryErr := db.Query(ctx, evidence.SQL)
	c.ledger.owners[i] = manualRollbackSQLOwner{rows: rows, path: evidence.SQL}
	evidence.Iterator = uint64(i + 1000)
	if queryErr != nil {
		return evidence, errors.Join(queryErr, c.ledger.close(i))
	}
	if rows == nil {
		return evidence, fmt.Errorf("nil Rows with nil error")
	}
	plan := rows.Plan()
	evidence.Plan = rollbackfault.SQLQueryPlan{Access: plan.Access, Table: plan.Table, Method: plan.Method, IndexCount: uint8(len(plan.Indexes)), FieldCount: uint8(len(plan.Fields)), Recheck: plan.Recheck, Reason: plan.Reason}
	if len(plan.Indexes) == 1 {
		evidence.Plan.Index = plan.Indexes[0]
	}
	if len(plan.Fields) == 1 {
		evidence.Plan.Field = plan.Fields[0]
	}
	if len(plan.Indexes) > 1 || len(plan.Fields) > 1 {
		return evidence, errors.Join(fmt.Errorf("plan outside fixed SQL/index scope"), c.ledger.close(i))
	}
	for rows.Next() {
		if evidence.RowCount == 3 {
			return evidence, errors.Join(fmt.Errorf("query exceeded fixed three-row bound"), c.ledger.close(i))
		}
		values := rows.Values()
		if len(values) != 3 {
			return evidence, errors.Join(fmt.Errorf("three typed columns required"), c.ledger.close(i))
		}
		id, idOK := values[0].(int64)
		key, keyOK := values[1].(string)
		value, valueOK := values[2].(string)
		if !idOK || !keyOK || !valueOK {
			return evidence, errors.Join(fmt.Errorf("SQL row types changed"), c.ledger.close(i))
		}
		evidence.Rows[evidence.RowCount] = rollbackfault.SQLRow{IDType: rollbackfault.SQLInteger, ID: id, KeyType: rollbackfault.SQLText, Key: key, ValueType: rollbackfault.SQLText, Value: value}
		evidence.RowCount++
	}
	// A single exhausted Next loop, then Err, then a single public Close. EOF
	// already retires the internal source; public Close never retries its owner.
	rowsErr := rows.Err()
	evidence.ErrCalled = true
	evidence.EOF = rowsErr == nil
	stats := rows.Stats()
	evidence.Work = rollbackfault.SQLQueryWork{IndexKeysRead: stats.IndexKeysRead, IndexPageReads: stats.IndexPageReads, RowIDSeeks: stats.RowIDSeeks, Candidates: stats.Candidates, BaseRowFetches: stats.BaseRowFetches, ReturnedRows: stats.ReturnedRows}
	closeErr := c.ledger.close(i)
	evidence.CloseCalls = c.ledger.owners[i].closes
	if err := errors.Join(rowsErr, closeErr); err != nil {
		return evidence, err
	}
	return evidence, nil
}

func manualRollbackSQLReadback(c *manualRollbackSQLConfig, ctx context.Context, path string, identity rollbackfault.Identity) (rollbackfault.SQLSnapshotEvidence, error) {
	evidence := rollbackfault.SQLSnapshotEvidence{Identity: identity}
	if !manualRollbackSQLRegistryEmpty() {
		return evidence, fmt.Errorf("pre-read owner obligations")
	}
	before, err := os.Lstat(path)
	if err != nil || before.Size() != 12288 {
		return evidence, fmt.Errorf("three-page readback extent: %v", err)
	}
	db, owner, err := manualRollbackSQLOpen(c, path, true)
	if err != nil {
		return evidence, err
	}
	evidence.Owner = uint64(owner + 1000)
	for i := 0; i < rollbackfault.SQLQueryCount; i++ {
		query, queryErr := manualRollbackSQLQuery(c, ctx, db, evidence.Owner, identity, rollbackfault.SQLQuery(i))
		evidence.Queries[i] = query
		if queryErr != nil {
			return evidence, errors.Join(queryErr, c.ledger.close(owner))
		}
	}
	checked, checkErr := db.Check(ctx, CheckOptions{MaxPages: 3, MaxPageReads: 512})
	evidence.Check = rollbackfault.SQLCheckEvidence{Identity: identity, Owner: evidence.Owner, Calls: 1, Pages: checked.Pages, FreePages: checked.FreePages, ReservedPages: checked.ReservedPages, Tables: checked.Tables, Indexes: checked.Indexes, ExtensionIndexes: checked.ExtensionIndexes, TableRows: checked.TableRows, IndexEntries: checked.IndexEntries}
	closeErr := c.ledger.close(owner)
	evidence.CloseCalls = c.ledger.owners[owner].closes
	if err := errors.Join(checkErr, closeErr); err != nil {
		return evidence, err
	}
	if checked.Scope != CheckScope || checked.PageReads == 0 || checked.PageReads > 512 || !manualRollbackSQLRegistryEmpty() {
		return evidence, fmt.Errorf("Check scope or owner retirement incomplete")
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || after.Size() != 12288 {
		return evidence, fmt.Errorf("readback file identity/extent changed: %v", err)
	}
	evidence.OwnerState = rollbackfault.OwnerRetired
	return evidence, nil
}

func (c *manualRollbackSQLConfig) retain(name string, expected os.FileInfo, limit int64) error {
	if c.retainedCount == len(c.retained) {
		return fmt.Errorf("retained inventory full")
	}
	raw, info, err := manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, name), limit)
	if err != nil {
		return err
	}
	if expected != nil && !os.SameFile(expected, info) {
		return fmt.Errorf("original artifact identity changed")
	}
	c.retained[c.retainedCount] = manualRollbackSQLSaved{name: name, info: info, hash: sha256.Sum256(raw), size: info.Size()}
	c.retainedCount++
	return nil
}

func manualRollbackSQLPageHashes(data []byte) ([3][32]byte, error) {
	var result [3][32]byte
	if len(data) != 12288 {
		return result, fmt.Errorf("exact three-page hash input required")
	}
	for i := range result {
		result[i] = sha256.Sum256(data[i*4096 : (i+1)*4096])
	}
	return result, nil
}
func manualRollbackSQLArtifact(c *manualRollbackSQLConfig, role rollbackfault.SQLArtifactRole) (rollbackfault.SQLArtifactStamp, error) {
	var stamp rollbackfault.SQLArtifactStamp
	name := rollbackfault.SQLArtifactName(role)
	path := filepath.Join(c.root, name)
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return stamp, nil
	} else if err != nil {
		return stamp, err
	}
	limit := int64(12288)
	if role == rollbackfault.SQLOriginalJournalArtifact || role == rollbackfault.SQLJournalCopyArtifact || role == rollbackfault.SQLReplayJournalArtifact {
		limit = 16408
	}
	raw, info, err := manualRollbackSQLRead(&c.ledger, path, limit)
	if err != nil {
		return stamp, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 || stat.Ino == 0 {
		return stamp, fmt.Errorf("single-link concrete artifact identity required")
	}
	return rollbackfault.SQLArtifactStamp{Identity: name, Device: uint64(stat.Dev), Inode: stat.Ino, Exists: true, Bytes: info.Size(), SHA256: sha256.Sum256(raw)}, nil
}

func manualRollbackSQLCapture(c *manualRollbackSQLConfig, evidence *rollbackfault.SQLLedgerEvidence, after bool) error {
	if err := c.stable(); err != nil {
		return err
	}
	for i := 0; i < int(rollbackfault.SQLArtifactCount); i++ {
		role := rollbackfault.SQLArtifactRole(i)
		stamp, err := manualRollbackSQLArtifact(c, role)
		if err != nil {
			return err
		}
		evidence.Artifacts[i].Role = role
		if after {
			evidence.Artifacts[i].After = stamp
		} else {
			evidence.Artifacts[i].Before = stamp
		}
	}
	return nil
}

func manualRollbackSQLNamespaceContents(c *manualRollbackSQLConfig) error {
	i, err := c.ledger.reserve()
	if err != nil {
		return err
	}
	dir, openErr := os.Open(c.root)
	c.ledger.owners[i] = manualRollbackSQLOwner{file: dir, path: c.root}
	if openErr != nil {
		return errors.Join(openErr, c.ledger.close(i))
	}
	entries, readErr := dir.ReadDir(16)
	var tail []os.DirEntry
	tailErr := readErr
	if readErr == nil {
		tail, tailErr = dir.ReadDir(1)
	}
	closeErr := c.ledger.close(i)
	if (readErr != nil && readErr != io.EOF) || tailErr != io.EOF || len(tail) != 0 || len(entries) > 10 || closeErr != nil {
		return errors.Join(fmt.Errorf("bounded namespace inventory incomplete"), readErr, tailErr, closeErr)
	}
	for _, entry := range entries {
		switch entry.Name() {
		case "baseline.input.db", "binding.input.json", "trial.db", "trial.db-journal", "post-commit.input.db", "post-commit-journal.input.bin", "replay.db", "replay.db-journal", "commit-observation.json", "result.json":
		default:
			return fmt.Errorf("unexpected retained namespace member: %s", entry.Name())
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular retained namespace member")
		}
	}
	return c.stable()
}

func TestManualRollbackFaultSQLLedger(t *testing.T) {
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
	needsRecovery := c.caseID == rollbackfault.SQLIndexPrefix64 || c.caseID == rollbackfault.SQLDBSyncBefore || c.caseID == rollbackfault.SQLIndexPrefix64Error || c.caseID == rollbackfault.SQLJournalUnlinkBefore
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
	}{identity, plan.Spec(), report.Observation, report.Events, report.EventCount, commitText, diagnosticTexts, report.PreDBOnlyTargetCause, report.PostDBOnlyTargetCause}, "", "  ")
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if _, err := c.create("commit-observation.json", commitRecord, true); err != nil {
		t.Fatal(err)
	}
	if commitCheckErr != nil {
		t.Fatalf("INCOMPLETE: original Commit=%v; adapter=%+v; problem=%v; all inputs retained", report.CommitError, report.Observation, commitCheckErr)
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
	evidence := rollbackfault.SQLLedgerEvidence{Identity: identity, Case: c.caseID, Before: before}
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
	post, postInfo, err := manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, "trial.db"), 12288)
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
	var journalBytes []byte
	if !needsRecovery {
		if _, err := os.Lstat(filepath.Join(c.root, "trial.db-journal")); !os.IsNotExist(err) {
			t.Fatalf("zero-recovery case journal must be absent after Commit: %v", err)
		}
	} else {
		journalBytes, _, err = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, "trial.db-journal"), 16408)
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
}
