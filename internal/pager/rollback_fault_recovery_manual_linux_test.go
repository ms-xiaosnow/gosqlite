//go:build linux && manual_fault_rollback

package pager

// SOURCE ONLY / MANUAL PHYSICAL NOT_RUN. One process admits one fixed case.
// A private, stable namespace and independent execution/provenance approval are
// prerequisites. Lstat/SameFile checks detect changes; they do not defend against
// an adversary replacing names concurrently. No input is removed by this file.
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
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"
)

// Every descriptor returned to this layer, even with a simultaneous error,
// enters this ledger before the error is examined. Closed entries are retained.
// A close error is terminal/unknown: it is recorded and never retried.
type manualRollbackFaultOwner struct {
	file     *os.File
	reader   *Reader
	path     string
	closes   uint8
	closeErr error
}

type manualRollbackFaultLedger struct {
	owners [96]manualRollbackFaultOwner
	count  int
}

func (l *manualRollbackFaultLedger) reserve() (int, error) {
	if l.count == len(l.owners) {
		return 0, fmt.Errorf("owner ledger full")
	}
	i := l.count
	l.count++
	return i, nil
}

func (l *manualRollbackFaultLedger) close(i int) error {
	o := &l.owners[i]
	if o.closes != 0 {
		return fmt.Errorf("duplicate close refused: %s", o.path)
	}
	if o.file == nil && o.reader == nil {
		return nil
	}
	o.closes++
	if o.file != nil {
		o.closeErr = o.file.Close()
	} else {
		o.closeErr = o.reader.Close()
	}
	return o.closeErr
}

func (l *manualRollbackFaultLedger) retired() bool {
	for i := 0; i < l.count; i++ {
		o := l.owners[i]
		if (o.file != nil || o.reader != nil) && (o.closes != 1 || o.closeErr != nil) {
			return false
		}
	}
	return true
}

func (l *manualRollbackFaultLedger) finish(t *testing.T) {
	for i := 0; i < l.count; i++ {
		o := &l.owners[i]
		if o.closes == 0 && (o.file != nil || o.reader != nil) {
			if err := l.close(i); err != nil {
				t.Errorf("retained owner close: %s: %v", o.path, err)
			}
		}
		if o.closeErr != nil {
			t.Errorf("owner retirement unknown; no retry: %s: %v", o.path, o.closeErr)
		}
	}
}

type manualRollbackFaultBinding struct {
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
type manualRollbackFaultCompile struct {
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

type manualRollbackFaultConfig struct {
	root          string
	rootInfo      os.FileInfo
	caseID        rollbackfault.RecoveryCase
	binding       manualRollbackFaultBinding
	bindingBytes  []byte
	sourceFiles   uint16
	ledger        manualRollbackFaultLedger
	retained      [12]manualRollbackFaultSaved
	retainedCount int
}

type manualRollbackFaultSaved struct {
	name string
	info os.FileInfo
	hash [32]byte
	size int64
}

func manualRollbackFaultHash(value string) ([32]byte, error) {
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

func manualRollbackFaultNamespace(root string) (os.FileInfo, error) {
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

func (c *manualRollbackFaultConfig) stable() error {
	info, err := manualRollbackFaultNamespace(c.root)
	if err != nil {
		return err
	}
	if !os.SameFile(info, c.rootInfo) {
		return fmt.Errorf("root namespace changed; retain everything")
	}
	return nil
}

func manualRollbackFaultRead(l *manualRollbackFaultLedger, path string, limit int64) ([]byte, os.FileInfo, error) {
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
	l.owners[i] = manualRollbackFaultOwner{file: f, path: path}
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

func manualRollbackFaultDecode(raw []byte, value any) error {
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

func manualRollbackFaultAdmit(c *manualRollbackFaultConfig) error {
	// Exact flags are intentional. No regex union, repeated test, subtest,
	// random order, competing test, timeout kill, or wrapper is admitted.
	want := map[string]string{
		"-test.run": "^TestManualRollbackFaultRecovery$", "-test.count": "1",
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
	if os.Getenv("GOSQLITE_ROLLBACK_FAULT_PRIVATE_ROOT_CONFIRMED") != "private_stable_namespace_retained" {
		return fmt.Errorf("private stable namespace and retention must be explicitly confirmed")
	}
	c.root = os.Getenv("GOSQLITE_ROLLBACK_FAULT_ROOT")
	info, err := manualRollbackFaultNamespace(c.root)
	if err != nil {
		return err
	}
	c.rootInfo = info
	i, err := c.ledger.reserve()
	if err != nil {
		return err
	}
	dir, openErr := os.Open(c.root)
	c.ledger.owners[i] = manualRollbackFaultOwner{file: dir, path: c.root}
	if openErr != nil {
		return errors.Join(openErr, c.ledger.close(i))
	}
	entries, readErr := dir.ReadDir(1)
	closeErr := c.ledger.close(i)
	if readErr != io.EOF || len(entries) != 0 || closeErr != nil {
		return errors.Join(fmt.Errorf("root must be empty before admission"), readErr, closeErr)
	}
	bindingPath := os.Getenv("GOSQLITE_ROLLBACK_FAULT_BINDING")
	bindingHash, err := manualRollbackFaultHash(os.Getenv("GOSQLITE_ROLLBACK_FAULT_BINDING_SHA256"))
	if err != nil {
		return err
	}
	raw, _, err := manualRollbackFaultRead(&c.ledger, bindingPath, 65536)
	if err != nil {
		return err
	}
	if sha256.Sum256(raw) != bindingHash {
		return fmt.Errorf("independently reviewed binding hash mismatch")
	}
	if err := manualRollbackFaultDecode(raw, &c.binding); err != nil {
		return err
	}
	c.bindingBytes = raw
	switch c.binding.Case {
	case "control":
		c.caseID = rollbackfault.RecoveryControl
	case "page-prefix-64":
		c.caseID = rollbackfault.RecoveryPagePrefix64
	case "db-sync-before":
		c.caseID = rollbackfault.RecoveryDBSyncBefore
	default:
		return fmt.Errorf("exactly one of three fixed cases required")
	}
	manifest, _, err := manualRollbackFaultRead(&c.ledger, c.binding.SourceManifestPath, 4<<20)
	if err != nil {
		return err
	}
	manifestHash, err := manualRollbackFaultHash(c.binding.Manifest)
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
	if err := manualRollbackFaultDecode(manifest, &inventory); err != nil {
		return err
	}
	if len(inventory.Files) == 0 || len(inventory.Files) > 4096 || len(inventory.Tree) != 40 {
		return fmt.Errorf("finite reviewed source inventory required")
	}
	for path, entry := range inventory.Files {
		if filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || strings.HasPrefix(path, "../") || entry.Bytes < 0 || (entry.GitMode != "100644" && entry.GitMode != "100755") || len(entry.GitBlob) != 40 {
			return fmt.Errorf("invalid reviewed source inventory entry")
		}
		if _, err := manualRollbackFaultHash(entry.SHA256); err != nil {
			return err
		}
	}
	c.sourceFiles = uint16(len(inventory.Files))
	compileRaw, _, err := manualRollbackFaultRead(&c.ledger, c.binding.CompileReceiptPath, 65536)
	if err != nil {
		return err
	}
	compileHash, err := manualRollbackFaultHash(c.binding.CompileReceipt)
	if err != nil || sha256.Sum256(compileRaw) != compileHash {
		return fmt.Errorf("independently reviewed compile receipt digest mismatch: %v", err)
	}
	var receipt manualRollbackFaultCompile
	if err := manualRollbackFaultDecode(compileRaw, &receipt); err != nil {
		return err
	}
	if receipt.Schema != "rollback-fault-compile-v1" || receipt.Source != c.binding.Source || receipt.Tool != c.binding.Tool || receipt.Binary != c.binding.Binary || receipt.Manifest != c.binding.Manifest || receipt.Package != "./internal/pager" || receipt.Tags != "manual_fault_rollback" || !receipt.CompileOnly || receipt.Files != c.sourceFiles || receipt.SourceTree != inventory.Tree {
		return fmt.Errorf("source-to-binary compile receipt does not bind this trial")
	}
	tool, _, err := manualRollbackFaultRead(&c.ledger, c.binding.ToolPath, 128<<20)
	if err != nil {
		return err
	}
	toolHash, err := manualRollbackFaultHash(c.binding.Tool)
	if err != nil || sha256.Sum256(tool) != toolHash {
		return fmt.Errorf("tool digest mismatch: %v", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	image, _, err := manualRollbackFaultRead(&c.ledger, executable, 128<<20)
	if err != nil {
		return err
	}
	binaryHash, err := manualRollbackFaultHash(c.binding.Binary)
	if err != nil || sha256.Sum256(image) != binaryHash {
		return fmt.Errorf("actual executable digest mismatch: %v", err)
	}
	if _, err := manualRollbackFaultHash(c.binding.Source); err != nil {
		return err
	}
	return c.stable()
}

func manualRollbackFaultImages() (old, next, partial [4096]byte) {
	copy(old[:], "SQLite format 3\x00")
	binary.BigEndian.PutUint16(old[16:], 4096)
	old[18], old[19], old[21], old[22], old[23] = 1, 1, 64, 32, 32
	binary.BigEndian.PutUint32(old[24:], 7)
	binary.BigEndian.PutUint32(old[28:], 1)
	binary.BigEndian.PutUint32(old[40:], 9)
	binary.BigEndian.PutUint32(old[44:], 4)
	binary.BigEndian.PutUint32(old[56:], 1)
	binary.BigEndian.PutUint32(old[92:], 7)
	old[100] = 13
	binary.BigEndian.PutUint16(old[105:], 4096)
	for i := 108; i < len(old); i++ {
		old[i] = byte((i*17 + 23) % 251)
	}
	next = old
	binary.BigEndian.PutUint32(next[24:], 8)
	binary.BigEndian.PutUint32(next[40:], 10)
	binary.BigEndian.PutUint32(next[92:], 8)
	partial = old
	copy(partial[:64], next[:64])
	return old, next, partial
}

func (c *manualRollbackFaultConfig) create(name string, data []byte, retain bool) (os.FileInfo, error) {
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
	c.ledger.owners[i] = manualRollbackFaultOwner{file: f, path: path}
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
	actual, actualInfo, err := manualRollbackFaultRead(&c.ledger, path, int64(len(data)))
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
		c.retained[c.retainedCount] = manualRollbackFaultSaved{name: name, info: actualInfo, hash: sha256.Sum256(actual), size: int64(len(actual))}
		c.retainedCount++
	}
	return actualInfo, c.stable()
}

func (c *manualRollbackFaultConfig) retainedUnchanged() error {
	if err := c.stable(); err != nil {
		return err
	}
	for i := 0; i < c.retainedCount; i++ {
		saved := c.retained[i]
		raw, info, err := manualRollbackFaultRead(&c.ledger, filepath.Join(c.root, saved.name), saved.size)
		if err != nil {
			return err
		}
		if !os.SameFile(info, saved.info) || info.Size() != saved.size || sha256.Sum256(raw) != saved.hash {
			return fmt.Errorf("retained input changed: %s", saved.name)
		}
	}
	return nil
}

func manualRollbackFaultPlan(c *manualRollbackFaultConfig) (rollbackfault.Plan, error) {
	b := c.binding
	spec := rollbackfault.PlanSpec{
		Identity:  rollbackfault.Identity{Suite: "rollback-fault-recovery-v1", Source: b.Source, Tool: b.Tool, Binary: b.Binary, Manifest: b.Manifest, Trial: b.Trial, Scope: b.Scope, Profile: rollbackfault.PlainRollback4096},
		MainOwner: 11, JournalOwner: 12, DirectoryOwner: 13, CleanupOwner: 14,
		OriginalPages: 1, DirtyPages: 1, PageSize: 4096, SectorSize: 4096,
		Action: rollbackfault.Pass, ExpectedCommit: rollbackfault.CommitSucceeded,
		Calls: [rollbackfault.MaxCalls]rollbackfault.Call{
			{Point: rollbackfault.JournalCreate, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},
			{Point: rollbackfault.JournalInvalid, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12, Length: 4096},
			{Point: rollbackfault.DBOriginalRead, Occurrence: 1, Role: rollbackfault.Database, Owner: 11, Length: 4096},
			{Point: rollbackfault.JournalRecord, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12, Offset: 4096, Length: 4104},
			{Point: rollbackfault.JournalBodySync, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},
			{Point: rollbackfault.JournalValidHeader, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12, Length: 12},
			{Point: rollbackfault.JournalHeaderSync, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},
			{Point: rollbackfault.PublishDirectorySync, Occurrence: 1, Role: rollbackfault.Directory, Owner: 13},
			{Point: rollbackfault.DBPage, Occurrence: 1, Role: rollbackfault.Database, Owner: 11, Length: 4096},
			{Point: rollbackfault.DBSync, Occurrence: 1, Role: rollbackfault.Database, Owner: 11},
			{Point: rollbackfault.JournalUnlink, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},
			{Point: rollbackfault.FinalDirectorySync, Occurrence: 1, Role: rollbackfault.Directory, Owner: 13},
		},
		CallCount: 12,
	}
	spec.Target = spec.Calls[8]
	if c.caseID == rollbackfault.RecoveryPagePrefix64 {
		spec.Action, spec.Prefix, spec.CallCount, spec.ExpectedCommit = rollbackfault.PrefixShortWrite, 64, 9, rollbackfault.CommitRecoveryRequired
	}
	if c.caseID == rollbackfault.RecoveryDBSyncBefore {
		spec.Target, spec.Action, spec.CallCount, spec.ExpectedCommit = spec.Calls[9], rollbackfault.FailBefore, 10, rollbackfault.CommitRecoveryRequired
	}
	for i := int(spec.CallCount); i < len(spec.Calls); i++ {
		spec.Calls[i] = rollbackfault.Call{}
	}
	plan, problem := rollbackfault.NewPlan(spec)
	if problem != rollbackfault.NoProblem {
		return plan, fmt.Errorf("plan refused: %v", problem)
	}
	return plan, nil
}

func manualRollbackFaultRegistriesEmpty() bool {
	plainRetirements.mu.Lock()
	empty := len(plainRetirements.groups) == 0
	plainRetirements.mu.Unlock()
	return empty && len(PendingPlaintextCleanups()) == 0
}

// Before adapter ownership transfer, close only an untouched live preparation
// transaction. A started/pending/unknown retirement is preserved, never retried.
func manualRollbackFaultAbandon(tx *Tx, cause error) error {
	if tx == nil {
		return cause
	}
	tx.mu.Lock()
	g := tx.cleanup
	live := tx.file != nil && g != nil
	if g != nil {
		g.mu.Lock()
		live = live && !g.started && !g.done && g.cause == nil && g.pending.Load() == nil && len(g.nested) == 0
		g.mu.Unlock()
	}
	tx.mu.Unlock()
	if !live {
		return cause
	}
	return errors.Join(cause, tx.Rollback())
}

func manualRollbackFaultCommit(ctx context.Context, path string, plan rollbackfault.Plan) (rollbackFaultReport, error) {
	if !manualRollbackFaultRegistriesEmpty() {
		return rollbackFaultReport{}, fmt.Errorf("preexisting plain owner obligations")
	}
	tx, openErr := OpenTx(ctx, path, Options{PageSize: 4096, SectorSize: 4096, MaxDirtyPages: 1, MaxFreelistPages: 1, MaxUndoBytes: 16384})
	if openErr != nil {
		// A constructor-carried owner belongs to the production registry. Do
		// not retry its cleanup or infer retirement from a nil Tx.
		return rollbackFaultReport{}, manualRollbackFaultAbandon(tx, openErr)
	}
	if err := tx.BumpSchemaCookie(ctx); err != nil {
		return rollbackFaultReport{}, manualRollbackFaultAbandon(tx, err)
	}
	scope, armErr := armRollbackFault(tx, plan)
	if armErr != nil {
		return rollbackFaultReport{}, manualRollbackFaultAbandon(tx, armErr)
	}
	commitErr := tx.Commit(ctx)
	report := finishRollbackFault(scope, commitErr)
	if report.CommitError != commitErr || report.Problem != nil || report.Observation.Outcome != rollbackfault.AdapterObserved || report.Observation.TargetHits != 1 || report.Observation.Pass() || report.Observation.PhysicalValidated() {
		return report, fmt.Errorf("adapter incomplete; original error retained: %v; problem: %v", commitErr, report.Problem)
	}
	if plan.Spec().Action == rollbackfault.Pass {
		if commitErr != nil {
			return report, fmt.Errorf("control Commit: %w", commitErr)
		}
	} else {
		if !errors.Is(commitErr, ErrRecoveryRequired) || errors.Is(commitErr, ErrPlaintextCleanupPending) || errors.Is(commitErr, ErrCommitUncertain) {
			return report, fmt.Errorf("unqualified Commit failure: %v", commitErr)
		}
		if plan.Spec().Action == rollbackfault.PrefixShortWrite && !errors.Is(commitErr, io.ErrShortWrite) {
			return report, fmt.Errorf("original short-write cause missing")
		}
		if plan.Spec().Action == rollbackfault.FailBefore && !errors.Is(commitErr, errRollbackFaultInjected) {
			return report, fmt.Errorf("original injected cause missing")
		}
	}
	if err := disarmRollbackFault(scope); err != nil {
		return report, err
	}
	if !manualRollbackFaultRegistriesEmpty() {
		return report, fmt.Errorf("Commit owner retirement incomplete")
	}
	return report, nil
}

func manualRollbackFaultRecoverCopy(ctx context.Context, replayPath string) (RecoveryResult, error) {
	if filepath.Base(replayPath) != "replay.db" || !manualRollbackFaultRegistriesEmpty() {
		return RecoveryResult{}, fmt.Errorf("fresh replay path and empty registry required")
	}
	result, recoveryErr := RecoverRollback(ctx, replayPath, RecoveryOptions{Limits: journal.ReplayLimits{MaxJournalBytes: 8200, MaxRecords: 1, MaxDatabaseBytes: 4096}})
	if recoveryErr != nil {
		return result, recoveryErr
	}
	if !manualRollbackFaultRegistriesEmpty() {
		return result, fmt.Errorf("recovery retirement incomplete; no fabricated owner token")
	}
	if !result.Recovered || result.RestoredPages != 1 || result.OriginalPages != 1 {
		return result, fmt.Errorf("exact one-page actual recovery required")
	}
	return result, nil
}

func manualRollbackFaultReadback(c *manualRollbackFaultConfig, ctx context.Context, replayPath string, expected [4096]byte) ([]byte, Header, error) {
	before, err := os.Lstat(replayPath)
	if err != nil || before.Size() != 4096 {
		return nil, Header{}, fmt.Errorf("replay extent before Reader: %v", err)
	}
	i, err := c.ledger.reserve()
	if err != nil {
		return nil, Header{}, err
	}
	reader, openErr := Open(replayPath)
	c.ledger.owners[i] = manualRollbackFaultOwner{reader: reader, path: replayPath}
	if openErr != nil {
		return nil, Header{}, errors.Join(openErr, c.ledger.close(i))
	}
	page, readErr := reader.ReadPage(ctx, 1)
	header := reader.Header()
	closeErr := c.ledger.close(i)
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, header, err
	}
	after, err := os.Lstat(replayPath)
	if err != nil {
		return nil, header, err
	}
	if !os.SameFile(before, after) || after.Size() != 4096 || !bytes.Equal(page, expected[:]) {
		return nil, header, fmt.Errorf("full-page Reader bytes/extent mismatch")
	}
	wanted, err := ParseHeader(expected[:], 4096)
	if err != nil || header != wanted {
		return nil, header, fmt.Errorf("Reader header mismatch: %v", err)
	}
	if !manualRollbackFaultRegistriesEmpty() || !c.ledger.retired() {
		return nil, header, fmt.Errorf("Reader or preparation owner unresolved")
	}
	return page, header, nil
}

func manualRollbackFaultSnapshot(c *manualRollbackFaultConfig, name string, limit int64) (rollbackfault.RecoveryFileSnapshot, error) {
	var snapshot rollbackfault.RecoveryFileSnapshot
	if err := c.stable(); err != nil {
		return snapshot, err
	}
	path := filepath.Join(c.root, name)
	_, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return snapshot, nil
	}
	if err != nil {
		return snapshot, err
	}
	raw, info, err := manualRollbackFaultRead(&c.ledger, path, limit)
	if err != nil {
		return snapshot, err
	}
	metadata, ok := info.Sys().(*syscall.Stat_t)
	if !ok || metadata.Nlink != 1 || metadata.Ino == 0 {
		return snapshot, fmt.Errorf("single-link concrete input identity required")
	}
	snapshot = rollbackfault.RecoveryFileSnapshot{Present: true, Device: uint64(metadata.Dev), Inode: metadata.Ino, Size: info.Size(), Hash: sha256.Sum256(raw)}
	return snapshot, nil
}

func manualRollbackFaultCapture(c *manualRollbackFaultConfig, evidence *rollbackfault.RecoveryEvidence, after bool) error {
	names := [8]string{"baseline.input.db", "expected-new.input.db", "trial.db", "trial.db-journal", "post-commit.input.db", "post-commit-journal.input.bin", "replay.db", "replay.db-journal"}
	for i, name := range names {
		limit := int64(4096)
		if i == 3 || i == 5 || i == 7 {
			limit = 8200
		}
		snapshot, err := manualRollbackFaultSnapshot(c, name, limit)
		if err != nil {
			return err
		}
		evidence.Files[i].Role = rollbackfault.RecoveryFileRole(i + 1)
		if after {
			evidence.Files[i].After = snapshot
		} else {
			evidence.Files[i].Before = snapshot
		}
	}
	return nil
}

func manualRollbackFaultOwners(c *manualRollbackFaultConfig, evidence *rollbackfault.RecoveryEvidence) error {
	if !c.ledger.retired() {
		return fmt.Errorf("ordinary owner ledger incomplete")
	}
	evidence.OwnerCount = 0
	for i := 0; i < c.ledger.count; i++ {
		o := c.ledger.owners[i]
		if o.file == nil && o.reader == nil {
			continue
		}
		kind := rollbackfault.RecoveryFileOwner
		if o.reader != nil {
			kind = rollbackfault.RecoveryReaderOwner
		}
		evidence.Owners[evidence.OwnerCount] = rollbackfault.RecoveryOwnerEvidence{ID: uint64(i + 1000), Kind: kind, OpenCalls: 1, CloseCalls: o.closes, State: rollbackfault.OwnerRetired}
		evidence.OwnerCount++
	}
	return nil
}

func manualRollbackFaultStage(evidence *rollbackfault.RecoveryEvidence, stage rollbackfault.RecoveryStage) {
	evidence.Stages[evidence.StageCount] = stage
	evidence.StageCount++
}

func TestManualRollbackFaultRecovery(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" {
		t.Fatal("NOT_RUN: separately authorized execution, reviewed provenance and private retained root are required")
	}
	var c manualRollbackFaultConfig
	defer c.ledger.finish(t)
	if err := manualRollbackFaultAdmit(&c); err != nil {
		t.Fatalf("BLOCKED before fixture: %v", err)
	}
	old, next, partial := manualRollbackFaultImages()
	plan, err := manualRollbackFaultPlan(&c)
	if err != nil {
		t.Fatalf("BLOCKED before fixture: %v", err)
	}
	var spec rollbackfault.RecoverySpec
	spec.Case, spec.Plan = c.caseID, plan.Spec()
	spec.SourceHash, err = manualRollbackFaultHash(c.binding.Source)
	if err != nil {
		t.Fatal(err)
	}
	spec.ToolHash, err = manualRollbackFaultHash(c.binding.Tool)
	if err != nil {
		t.Fatal(err)
	}
	spec.BinaryHash, err = manualRollbackFaultHash(c.binding.Binary)
	if err != nil {
		t.Fatal(err)
	}
	spec.ManifestHash, err = manualRollbackFaultHash(c.binding.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	spec.CompileReceiptHash, err = manualRollbackFaultHash(c.binding.CompileReceipt)
	if err != nil {
		t.Fatal(err)
	}
	spec.OldPageHash, spec.NewPageHash, spec.PartialPageHash = sha256.Sum256(old[:]), sha256.Sum256(next[:]), sha256.Sum256(partial[:])
	evidence := rollbackfault.RecoveryEvidence{Identity: spec.Plan.Identity, Case: c.caseID, SelectedCases: 1}
	evidence.Provenance = rollbackfault.RecoveryProvenance{
		SourceHash: spec.SourceHash, ToolHash: spec.ToolHash, BinaryHash: spec.BinaryHash, ManifestHash: spec.ManifestHash, CompileReceiptHash: spec.CompileReceiptHash,
		CompileSourceHash: spec.SourceHash, CompileToolHash: spec.ToolHash, CompileBinaryHash: spec.BinaryHash, CompileManifestHash: spec.ManifestHash,
		SourceFiles: c.sourceFiles, ManifestFiles: c.sourceFiles, CompileFiles: c.sourceFiles,
	}
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryAdmitted)
	if _, err := c.create("baseline.input.db", old[:], true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.create("expected-new.input.db", next[:], true); err != nil {
		t.Fatal(err)
	}
	trialInfo, err := c.create("trial.db", old[:], false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.create("binding.input.json", c.bindingBytes, true); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	report, commitCheckErr := manualRollbackFaultCommit(ctx, filepath.Join(c.root, "trial.db"), plan)
	// No recovery, original Close, error substitution or cleanup retry is
	// allowed to turn an incomplete adapter result into a successful trial.
	if commitCheckErr != nil {
		t.Logf("original Commit error: %v; adapter: %+v", report.CommitError, report.Observation)
		for i, diagnostic := range report.Diagnostics {
			if diagnostic != nil {
				t.Logf("adapter diagnostic %d: %v", i, diagnostic)
			}
		}
		t.Fatalf("INCOMPLETE: adapter/owner qualification: %v; all inputs retained", commitCheckErr)
	}
	evidence.ArmCalls, evidence.CommitCalls, evidence.FinishCalls, evidence.DisarmCalls = 1, 1, 1, 1
	evidence.AdapterPlan, evidence.AdapterEvents, evidence.AdapterEventCount, evidence.AdapterObservation = plan.Spec(), report.Events, report.EventCount, report.Observation
	var adapterHashProblem rollbackfault.RecoveryProblem
	evidence.AdapterHash, adapterHashProblem = rollbackfault.RecoveryAdapterHash(evidence.AdapterPlan, evidence.AdapterEvents, evidence.AdapterEventCount)
	if adapterHashProblem != rollbackfault.RecoveryProblemNone {
		t.Fatalf("adapter transcript hash: %v", adapterHashProblem)
	}
	for i := uint8(0); i < report.EventCount; i++ {
		if report.Events[i].Kind == rollbackfault.CommitReturned {
			evidence.CommitReturn = report.Events[i].Commit
		}
	}
	if report.CommitError != nil {
		evidence.CommitDiagnosticHash = sha256.Sum256([]byte(report.CommitError.Error()))
	}
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryCommitReturned)
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryAdapterSealed)
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryAdapterDisarmed)
	if err := c.stable(); err != nil {
		t.Fatal(err)
	}
	post, postInfo, err := manualRollbackFaultRead(&c.ledger, filepath.Join(c.root, "trial.db"), 4096)
	if err != nil {
		t.Fatal(err)
	}
	wantPost := next
	if c.caseID == rollbackfault.RecoveryPagePrefix64 {
		wantPost = partial
	}
	if !os.SameFile(trialInfo, postInfo) || !bytes.Equal(post, wantPost[:]) {
		t.Fatal("FAILED: exact post-Commit page/identity mismatch")
	}
	if _, err := c.create("post-commit.input.db", post, true); err != nil {
		t.Fatal(err)
	}
	var journalBytes []byte
	var journalInfo os.FileInfo
	if c.caseID == rollbackfault.RecoveryControl {
		if _, err := os.Lstat(filepath.Join(c.root, "trial.db-journal")); !os.IsNotExist(err) {
			t.Fatalf("control journal must be absent: %v", err)
		}
	} else {
		journalBytes, journalInfo, err = manualRollbackFaultRead(&c.ledger, filepath.Join(c.root, "trial.db-journal"), 8200)
		if err != nil || len(journalBytes) != 8200 {
			t.Fatalf("FAILED: actual journal bounds: %v", err)
		}
		if _, err := c.create("post-commit-journal.input.bin", journalBytes, true); err != nil {
			t.Fatal(err)
		}
	}
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryInputsRetained)
	replayInfo, err := c.create("replay.db", post, false)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(replayInfo, trialInfo) {
		t.Fatal("replay aliases original trial")
	}
	if c.caseID != rollbackfault.RecoveryControl {
		if _, err := c.create("replay.db-journal", journalBytes, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := manualRollbackFaultCapture(&c, &evidence, false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(evidence.Files); i++ {
		for j := 0; j < i; j++ {
			a, b := evidence.Files[i].Before, evidence.Files[j].Before
			if a.Present && b.Present && a.Device == b.Device && a.Inode == b.Inode {
				t.Fatal("pre-recovery file alias refused")
			}
		}
	}
	// Reject any already-observed namespace/content change before replay, not
	// only in the final receipt validator. The stable private namespace is
	// still an assumption, not a claim of race-proof path security.
	if err := c.retainedUnchanged(); err != nil {
		t.Fatal(err)
	}
	postHash, journalHash := sha256.Sum256(post), sha256.Sum256(journalBytes)
	wantHashes := [8][32]byte{spec.OldPageHash, spec.NewPageHash, postHash, journalHash, postHash, journalHash, postHash, journalHash}
	for i, file := range evidence.Files {
		isJournal := i == 3 || i == 5 || i == 7
		if isJournal && c.caseID == rollbackfault.RecoveryControl {
			if file.Before != (rollbackfault.RecoveryFileSnapshot{}) {
				t.Fatal("unexpected control journal before replay")
			}
			continue
		}
		if !file.Before.Present || file.Before.Hash != wantHashes[i] {
			t.Fatal("pre-recovery retained/copy bytes changed")
		}
	}
	for index, info := range map[int]os.FileInfo{2: trialInfo, 6: replayInfo, 3: journalInfo} {
		if info == nil {
			continue
		}
		metadata, ok := info.Sys().(*syscall.Stat_t)
		actual := evidence.Files[index].Before
		if !ok || actual.Device != uint64(metadata.Dev) || actual.Inode != metadata.Ino {
			t.Fatal("pre-recovery original/replay identity changed")
		}
	}
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryReplayCopied)
	if !manualRollbackFaultRegistriesEmpty() {
		t.Fatal("pre-recovery registry nonempty")
	}
	evidence.Recovery.RegistryChecks++
	if c.caseID != rollbackfault.RecoveryControl {
		result, recoveryErr := manualRollbackFaultRecoverCopy(ctx, filepath.Join(c.root, "replay.db"))
		evidence.Recovery.Calls++
		if recoveryErr != nil {
			t.Fatalf("INCOMPLETE: actual recovery: %v; all original inputs retained", recoveryErr)
		}
		evidence.Recovery.Recovered, evidence.Recovery.RestoredPages, evidence.Recovery.OriginalPages = result.Recovered, result.RestoredPages, result.OriginalPages
		evidence.Recovery.MaxJournalBytes, evidence.Recovery.MaxDatabaseBytes, evidence.Recovery.MaxRecords = 8200, 4096, 1
	}
	if !manualRollbackFaultRegistriesEmpty() {
		t.Fatal("post-recovery registry nonempty")
	}
	evidence.Recovery.RegistryChecks++
	if _, err := os.Lstat(filepath.Join(c.root, "replay.db-journal")); !os.IsNotExist(err) {
		t.Fatalf("replay journal must be absent after recovery/control: %v", err)
	}
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryChecked)
	wantRead := old
	if c.caseID == rollbackfault.RecoveryControl {
		wantRead = next
	}
	readerOwner := uint64(c.ledger.count + 1000)
	page, header, err := manualRollbackFaultReadback(&c, ctx, filepath.Join(c.root, "replay.db"), wantRead)
	if err != nil {
		t.Fatalf("FAILED: new Reader full-page observation: %v", err)
	}
	evidence.Reader = rollbackfault.RecoveryReadEvidence{
		Owner: readerOwner, ReadCalls: 1, CloseCalls: 1, PageNumber: 1, BytesRead: uint32(len(page)), ComparedBytes: 4096, MatchingBytes: 4096,
		ExtentBefore: 4096, ExtentAfter: 4096, PageHash: sha256.Sum256(page),
		Header: rollbackfault.RecoveryPageHeader{PageSize: header.PageSize, DatabasePages: header.PageCount, ChangeCounter: header.ChangeCounter, SchemaCookie: header.SchemaCookie, VersionValidFor: header.VersionValidFor, Encoding: uint8(header.TextEncoding), ReadVersion: header.ReadVersion, WriteVersion: header.WriteVersion},
	}
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryReaderClosed)
	if err := manualRollbackFaultCapture(&c, &evidence, true); err != nil {
		t.Fatal(err)
	}
	if err := c.retainedUnchanged(); err != nil {
		t.Fatal(err)
	}
	if !manualRollbackFaultRegistriesEmpty() {
		t.Fatal("final plain registry nonempty")
	}
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryInputsRechecked)
	if err := manualRollbackFaultOwners(&c, &evidence); err != nil {
		t.Fatal(err)
	}
	var diagnostics [rollbackfault.MaxEvents]string
	for i, diagnostic := range report.Diagnostics {
		if diagnostic != nil {
			diagnostics[i] = diagnostic.Error()
		}
	}
	commitDiagnostic := ""
	if report.CommitError != nil {
		commitDiagnostic = report.CommitError.Error()
	}
	preterminal, err := json.Marshal(struct {
		Status           string
		Spec             rollbackfault.RecoverySpec
		Evidence         rollbackfault.RecoveryEvidence
		CommitDiagnostic string
		Diagnostics      [rollbackfault.MaxEvents]string
	}{"PRETERMINAL_NOT_ACCEPTANCE", spec, evidence, commitDiagnostic, diagnostics})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.create("preterminal.json", preterminal, false); err != nil {
		t.Fatalf("INCOMPLETE: receipt output: %v", err)
	}
	// The receipt descriptor and its readback descriptor are now retired.
	// Final terminal is emitted only to the caller-owned test log, preventing a
	// circular claim that a file contains proof of its own successful Close.
	if err := manualRollbackFaultOwners(&c, &evidence); err != nil {
		t.Fatal(err)
	}
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryTerminal)
	observation := rollbackfault.ValidateRecoveryEvidence(spec, evidence)
	if observation.Outcome != rollbackfault.ScenarioObserved || observation.Pass() || observation.PhysicalValidated() {
		t.Fatalf("FAILED: recovery evidence consistency: %+v", observation)
	}
	terminal, err := json.Marshal(struct {
		Status          string
		PreterminalHash [32]byte
		Spec            rollbackfault.RecoverySpec
		Evidence        rollbackfault.RecoveryEvidence
		Observation     rollbackfault.RecoveryObservation
	}{"ScenarioObserved_INPUT_CONSISTENCY_ONLY", sha256.Sum256(preterminal), spec, evidence, observation})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ROLLBACK_RECOVERY_TERMINAL %s", terminal)
}
