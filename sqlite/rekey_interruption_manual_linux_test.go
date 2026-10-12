//go:build linux && manual_rekey_interruption

package sqlite_test

// SOURCE_ONLY / REVIEW_HOLD. All process exit, wrong-key, fixture and archive
// execution is NOT_RUN. These gates are requirements, never authorization.
import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/rekeyinterruption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

const manualRekeyProfile = "linux-layout1-rawkey256-4096"
const manualRekeyRootName = "fixture.gse"
const manualRekeyWaitLimit = 30 * time.Second
const manualRekeyLimit = 65536

type manualRekeyAdmission struct {
	Schema, Trial, Case, Root, Filesystem, Declaration string
	SourceRoot, SourceManifestPath, SourceManifest     string
	CompilerPath, Compiler, BinaryPath, Binary         string
	CompileReceiptPath, CompileReceipt                 string
	ArchiveControl                                     bool
}
type manualRekeyArtifact struct {
	Name          string
	Device, Inode uint64
	Bytes         int64
	Hash          [32]byte
}
type manualRekeyFamily []manualRekeyArtifact
type manualRekeyOwner struct {
	ID       string
	File     *os.File
	DB       *sqlite.DB
	Rows     *sqlite.Rows
	Parent   string
	Attempts int
	State    string
}
type manualRekeyRoleResult struct {
	Role, Generation, Outcome string
	OpenCalls, CloseCalls     int
	NilDB                     bool
	Code                      int
	Authenticated             bool
	Header                    encryption.Header
	RootHash                  [32]byte
	Info                      sqlite.EncryptionInfo
	SQL                       manualRekeySQLLedger
	Providers                 []manualRekeyProviderResult
	Owners                    []manualRekeyOwnerResult
	Registry                  encstore.ManualRekeyOwnerSnapshot
	PhysicalValidated         bool
}
type manualRekeyOwnerResult struct {
	ID, Parent, State string
	Attempts          int
}
type manualRekeyRun struct {
	t                           *testing.T
	admission                   manualRekeyAdmission
	bindingPath, bindingHash    string
	role, generation, challenge string
	owners                      []*manualRekeyOwner
	causes                      []error
	providers                   []*manualRekeyProvider
	channel                     *encstore.ManualRekeyChannel
	children                    []*manualRekeyChild
	result                      manualRekeyRoleResult
	observation                 encstore.ManualRekeyObservation
	baseline                    manualRekeyFamily
	postExit                    manualRekeyFamily
	selfInfo                    os.FileInfo
	ownerPending                bool
}

var manualRekeyRetained []*manualRekeyRun

func manualRekeyHash(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func manualRekeyDecode(raw []byte, value any) error {
	if len(raw) == 0 || len(raw) > 4<<20 {
		return errors.New("bounded JSON required")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(raw, canonical) {
		return errors.New("canonical complete JSON required")
	}
	return nil
}
func (r *manualRekeyRun) capture(file *os.File, db *sqlite.DB, rows *sqlite.Rows, parent string) *manualRekeyOwner {
	o := &manualRekeyOwner{ID: fmt.Sprintf("%s/%s/%d", r.role, r.generation, len(r.owners)+1), File: file, DB: db, Rows: rows, Parent: parent, State: "live"}
	if file == nil && db == nil && rows == nil {
		o.State = "absent"
	}
	r.owners = append(r.owners, o)
	if len(r.owners) > 16384 {
		r.require(false, "owner capacity", nil)
	}
	return o
}
func (r *manualRekeyRun) pending(err error) {
	if err != nil {
		r.causes = append(r.causes, err)
	}
	pending := sqlite.PendingEncryptedCleanups()
	r.causes = append(r.causes, pending...)
	if len(pending) != 0 || errors.Is(err, sqlite.ErrEncryptedCleanupPending) {
		r.ownerPending = true
	}
}
func (r *manualRekeyRun) close(o *manualRekeyOwner) error {
	if o == nil || o.State == "absent" {
		return nil
	}
	if o.Attempts != 0 || o.State != "live" {
		return errors.New("owner close repeated or unknown")
	}
	r.pending(nil)
	if (o.DB != nil || o.Rows != nil) && r.ownerPending {
		o.State = "blocked"
		return errors.New("pending native owner")
	}
	for _, child := range r.owners {
		if child.Parent == o.ID && child.State != "closed" && child.State != "absent" {
			o.State = "blocked"
			return errors.New("child not retired")
		}
	}
	o.Attempts++
	var err error
	if o.File != nil {
		err = o.File.Close()
	} else if o.Rows != nil {
		err = o.Rows.Close()
	} else if o.DB != nil {
		err = o.DB.Close()
	}
	r.pending(err)
	o.State = "closed"
	if err != nil {
		o.State = "unknown"
		r.ownerPending = true
	}
	return err
}
func (r *manualRekeyRun) require(ok bool, label string, err error) {
	r.pending(err)
	if !ok || err != nil || r.ownerPending {
		r.t.Fatal(label + "; original causes and owners retained")
	}
}
func (r *manualRekeyRun) finish() {
	// A live/unknown child owns its pipes and process indefinitely. Never tear
	// those capabilities down or treat elapsed time as successful Wait.
	for _, child := range r.children {
		if child.started && !child.waited {
			child.unknown = true
			r.saveProcess(child, "FAILED_OR_WAIT_UNKNOWN")
			// Explicit unresolved-child hold. Fatal/Goexit cannot return this
			// controller to testing.main while a child capability is live.
			// No timeout, new grant, kill, replacement or dependent probe is
			// performed here. External disposition may be necessary forever.
			result := <-child.wait
			child.actual = result
			if result.State == nil {
				child.unknown = true
				manualRekeyHoldUnknown()
			}
			child.waited = true
			late := struct {
				ActualWait        bool
				ExitCode          int
				PhysicalValidated bool
			}{ActualWait: result.State != nil, ExitCode: -1, PhysicalValidated: false}
			if result.State != nil {
				late.ExitCode = result.State.ExitCode()
			}
			raw, encodeErr := json.Marshal(late)
			_, writeErr := r.writeExclusive(filepath.Join(r.admission.Root, "control", child.binding.Role+".late-wait.json"), raw, true)
			if errors.Join(encodeErr, writeErr) != nil {
				r.t.Error("late Wait record incomplete")
			}
		}
		if r.t.Failed() {
			r.saveProcess(child, "FAILED_OR_WAIT_UNKNOWN")
		}
	}
	for i := len(r.owners) - 1; i >= 0; i-- {
		o := r.owners[i]
		if o.State == "live" && o.Attempts == 0 {
			if r.close(o) != nil {
				r.t.Error("one-time retirement failed")
			}
		}
	}
}

// A timer-backed passive hold avoids runtime deadlock termination when a
// completed Wait supplied no ProcessState. Waking performs no external action.
func manualRekeyHoldUnknown() {
	for {
		time.Sleep(time.Hour)
	}
}
func manualRekeyRegular(info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() {
		return false
	}
	s, ok := info.Sys().(*syscall.Stat_t)
	return ok && s.Nlink == 1 && (s.Uid == uint32(os.Geteuid()) || s.Uid == 0 && info.Mode().Perm()&0022 == 0) && s.Dev != 0 && s.Ino != 0
}
func manualRekeyAncestors(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("absolute canonical path required")
	}
	current := "/"
	for _, component := range strings.Split(strings.TrimPrefix(filepath.Dir(path), "/"), "/") {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("unverified ancestor")
		}
	}
	return nil
}
func (r *manualRekeyRun) read(path string, limit int64) ([]byte, os.FileInfo, error) {
	if err := manualRekeyAncestors(path); err != nil {
		return nil, nil, err
	}
	before, err := os.Lstat(path)
	if err != nil || !manualRekeyRegular(before) || before.Size() < 0 || before.Size() > limit {
		return nil, nil, errors.New("bounded single-link file required")
	}
	file, openErr := os.Open(path)
	owner := r.capture(file, nil, nil, "")
	if openErr != nil {
		return nil, nil, errors.Join(openErr, r.close(owner))
	}
	opened, statErr := file.Stat()
	raw, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	after, afterErr := file.Stat()
	closeErr := r.close(owner)
	if err := errors.Join(statErr, readErr, afterErr, closeErr); err != nil {
		return nil, nil, err
	}
	final, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, opened) || !os.SameFile(before, after) || !os.SameFile(before, final) || before.Size() != int64(len(raw)) || before.Size() != after.Size() || before.Size() != final.Size() {
		return nil, nil, errors.New("read identity changed")
	}
	return raw, before, nil
}
func (r *manualRekeyRun) syncDirectory(path string) error {
	if err := manualRekeyAncestors(filepath.Join(path, "sentinel")); err != nil {
		return err
	}
	file, err := os.Open(path)
	owner := r.capture(file, nil, nil, "")
	if err != nil {
		return errors.Join(err, r.close(owner))
	}
	info, statErr := file.Stat()
	if statErr != nil || !info.IsDir() {
		return errors.Join(errors.New("directory owner required"), r.close(owner))
	}
	syncErr := file.Sync()
	return errors.Join(syncErr, r.close(owner))
}
func (r *manualRekeyRun) writeExclusive(path string, raw []byte, syncDirectory bool) (manualRekeyArtifact, error) {
	var artifact manualRekeyArtifact
	if len(raw) > 4<<20 {
		return artifact, errors.New("artifact bound")
	}
	if err := manualRekeyAncestors(path); err != nil {
		return artifact, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	owner := r.capture(file, nil, nil, "")
	if err != nil {
		return artifact, errors.Join(err, r.close(owner))
	}
	n, writeErr := file.Write(raw)
	if writeErr == nil && n != len(raw) {
		writeErr = io.ErrShortWrite
	}
	var syncErr error
	if writeErr == nil {
		syncErr = file.Sync()
	}
	closeErr := r.close(owner)
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return artifact, err
	}
	if syncDirectory {
		if err := r.syncDirectory(filepath.Dir(path)); err != nil {
			return artifact, err
		}
	}
	got, info, err := r.read(path, int64(len(raw)))
	if err != nil || !bytes.Equal(raw, got) {
		return artifact, errors.New("exclusive write readback mismatch")
	}
	s := info.Sys().(*syscall.Stat_t)
	return manualRekeyArtifact{Name: filepath.Base(path), Device: uint64(s.Dev), Inode: s.Ino, Bytes: info.Size(), Hash: sha256.Sum256(got)}, nil
}
func (r *manualRekeyRun) mkdir(path string) error {
	if err := manualRekeyAncestors(path); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return errors.New("private retained namespace required")
	}
	return r.syncDirectory(filepath.Dir(path))
}
func manualRekeyGeneration(role string) string {
	switch role {
	case "controller":
		return "1"
	case "writer":
		return "2"
	case "old-probe":
		return "3"
	case "new-probe":
		return "4"
	case "archive-check":
		return "5"
	}
	return ""
}
func manualRekeyRoleRoot(role string) string {
	switch role {
	case "controller":
		return "TestManualRekeyInterruptionController"
	case "writer":
		return "TestManualRekeyInterruptionWriter"
	case "old-probe":
		return "TestManualRekeyInterruptionOldProbe"
	case "new-probe":
		return "TestManualRekeyInterruptionNewProbe"
	case "archive-check":
		return "TestManualRekeyInterruptionArchive"
	default:
		return ""
	}
}
func manualRekeyFlags(role string) []string {
	return []string{"-test.run=^" + manualRekeyRoleRoot(role) + "$", "-test.count=1", "-test.timeout=0s", "-test.parallel=1", "-test.shuffle=off", "-test.v=true"}
}
func manualRekeyAdmit(t *testing.T, role string) *manualRekeyRun {
	// This independent gate is the first action of every exact test root.
	if os.Getenv("GOSQLITE_MANUAL_REKEY_INTERRUPTION") != "independently_authorized" {
		t.Fatal("rekey interruption not independently authorized")
	}
	r := &manualRekeyRun{t: t, role: role, generation: manualRekeyGeneration(role), result: manualRekeyRoleResult{Role: role, Generation: manualRekeyGeneration(role), Outcome: "NOT_RUN", PhysicalValidated: false}}
	manualRekeyRetained = append(manualRekeyRetained, r)
	r.require(runtime.GOOS == "linux" && os.Geteuid() > 0 && os.Getenv("GOFLAGS") == "" && os.Getenv("GOSQLITE_REKEY_ROLE") == role, "fixed unprivileged Linux role", nil)
	r.require(reflect.DeepEqual(os.Args[1:], manualRekeyFlags(role)), "exact role root and flags", nil)
	r.bindingPath, r.bindingHash = os.Getenv("GOSQLITE_REKEY_BINDING"), os.Getenv("GOSQLITE_REKEY_BINDING_SHA256")
	raw, _, err := r.read(r.bindingPath, manualRekeyLimit)
	r.require(err == nil && manualRekeyHash(raw) == r.bindingHash, "admitted binding", err)
	r.require(true, "binding schema", manualRekeyDecode(raw, &r.admission))
	a := r.admission
	_, caseOK := rekeyinterruption.ParseCase(a.Case)
	r.require(caseOK && a.Schema == "rekey-interruption-binding-v1" && a.Trial != "" && len(a.Trial) <= 64 && !strings.ContainsAny(a.Trial, " /\\\t\r\n") && a.Declaration == "linux-process-exit-no-powerloss-retain-all" && (!a.ArchiveControl || a.Case == "control"), "closed six-case admission", nil)
	r.require(true, "new root path", manualRekeyAncestors(a.Root))
	parent, err := os.Lstat(filepath.Dir(a.Root))
	r.require(err == nil && parent.IsDir() && parent.Mode().Perm() == 0700 && parent.Sys().(*syscall.Stat_t).Uid == uint32(os.Geteuid()), "private root parent", err)
	var filesystem syscall.Statfs_t
	r.require(true, "filesystem observation", syscall.Statfs(filepath.Dir(a.Root), &filesystem))
	r.require(a.Filesystem == fmt.Sprintf("linux-fstype-%x", uint64(filesystem.Type)), "exact filesystem identity", nil)
	r.require(true, "source-to-binary binding", r.checkSourceImage())
	if role != "controller" {
		r.challenge = os.Getenv("GOSQLITE_REKEY_CHALLENGE")
		challenge, e := hex.DecodeString(r.challenge)
		r.require(e == nil && len(challenge) == 32, "parent challenge", e)
		r.channel, err = encstore.ManualRekeyChildChannel(r.binding())
		r.require(true, "fixed child transport", err)
	}
	r.require(encstore.ManualRekeyOwners() == (encstore.ManualRekeyOwnerSnapshot{}) && len(sqlite.PendingEncryptedCleanups()) == 0, "empty pre-role registry", nil)
	if role != "controller" {
		grant, err := r.channel.Exchange("role-ready", 0, encstore.ManualRekeyObservation{}, [32]byte{})
		r.require(err == nil && grant == "continue", "parent role-start grant", err)
	}
	return r
}
func (r *manualRekeyRun) binding() encstore.ManualRekeyBinding {
	return encstore.ManualRekeyBinding{Trial: r.admission.Trial, Source: r.admission.SourceManifest, Binary: r.admission.Binary, Profile: manualRekeyProfile, Case: r.admission.Case, Role: r.role, Generation: r.generation, Challenge: r.challenge}
}
func (r *manualRekeyRun) checkSourceImage() error {
	a := r.admission
	manifest, _, err := r.read(a.SourceManifestPath, 4<<20)
	if err != nil || manualRekeyHash(manifest) != a.SourceManifest {
		return errors.New("source manifest mismatch")
	}
	var inventory struct {
		Files map[string]struct {
			Bytes  int64
			SHA256 string
		}
	}
	if err := manualRekeyDecode(manifest, &inventory); err != nil {
		return err
	}
	if len(inventory.Files) < 1 || len(inventory.Files) > 4096 {
		return errors.New("source inventory bound")
	}
	seen := map[string]bool{}
	err = filepath.WalkDir(a.SourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, e := filepath.Rel(a.SourceRoot, path)
		if e != nil {
			return e
		}
		rel = filepath.ToSlash(rel)
		if !strings.HasSuffix(rel, ".go") && rel != "go.mod" && rel != "go.sum" {
			return nil
		}
		item, ok := inventory.Files[rel]
		if !ok {
			return errors.New("source inventory omission")
		}
		data, info, e := r.read(path, 4<<20)
		if e != nil || info.Size() != item.Bytes || manualRekeyHash(data) != item.SHA256 {
			return errors.New("source bytes changed")
		}
		seen[rel] = true
		return nil
	})
	if err != nil || len(seen) != len(inventory.Files) {
		return errors.New("incomplete source closure")
	}
	compiled, _, err := r.read(a.CompileReceiptPath, manualRekeyLimit)
	if err != nil || manualRekeyHash(compiled) != a.CompileReceipt {
		return errors.New("compile receipt mismatch")
	}
	var receipt struct {
		Schema, SourceManifest, Compiler, Binary, Package, Tags, GoVersion string
		CompileOnly                                                        bool
		Flags                                                              []string
	}
	if err := manualRekeyDecode(compiled, &receipt); err != nil {
		return err
	}
	if receipt.Schema != "rekey-interruption-compile-v1" || receipt.SourceManifest != a.SourceManifest || receipt.Compiler != a.Compiler || receipt.Binary != a.Binary || receipt.Package != "./sqlite" || receipt.Tags != "manual_rekey_interruption" || receipt.GoVersion != runtime.Version() || !receipt.CompileOnly || !reflect.DeepEqual(receipt.Flags, []string{"test", "-c", "-tags=manual_rekey_interruption", "-o", a.BinaryPath, "./sqlite"}) {
		return errors.New("compile-only source binding refused")
	}
	tool, _, err := r.read(a.CompilerPath, 128<<20)
	if err != nil || manualRekeyHash(tool) != a.Compiler {
		return errors.New("compiler identity mismatch")
	}
	image, err := os.Open("/proc/self/exe")
	owner := r.capture(image, nil, nil, "")
	if err != nil {
		return errors.Join(err, r.close(owner))
	}
	before, statErr := image.Stat()
	raw, readErr := io.ReadAll(io.LimitReader(image, (128<<20)+1))
	after, afterErr := image.Stat()
	closeErr := r.close(owner)
	if errors.Join(statErr, readErr, afterErr, closeErr) != nil || before.Size() <= 0 || before.Size() > 128<<20 || !os.SameFile(before, after) || before.Size() != int64(len(raw)) || before.Size() != after.Size() || manualRekeyHash(raw) != a.Binary {
		return errors.New("running self image mismatch")
	}
	pinned, info, err := r.read(a.BinaryPath, 128<<20)
	if err != nil || !os.SameFile(before, info) || manualRekeyHash(pinned) != a.Binary {
		return errors.New("spawn target not self image")
	}
	r.selfInfo = before
	return nil
}

// Credentials are public disposable synthetic material, never application keys.
// Purpose/path/operation are immutable admission decisions made before Open.
type manualRekeyProvider struct {
	mu                   sync.Mutex
	purpose, label, path string
	operation            encryption.Operation
	acquires, releases   int
	duplicate            bool
	borrow               encstore.ManualRekeyBorrow
}
type manualRekeyMaterial struct {
	provider *manualRekeyProvider
	key      []byte
	released bool
}
type manualRekeyProviderResult struct {
	Purpose, Label, Path string
	Operation            encryption.Operation
	Acquires, Releases   int
	Duplicate            bool
}

func (r *manualRekeyRun) provider(purpose string) *manualRekeyProvider {
	p := &manualRekeyProvider{purpose: purpose}
	switch purpose {
	case "old-seed":
		p.label, p.path, p.operation = "OLD", r.path("inputs/original"), encryption.OperationCreate
	case "old-writer":
		p.label, p.path, p.operation = "OLD", r.path("trial"), encryption.OperationOpen
	case "new-rekey":
		p.label, p.path, p.operation = "NEW", r.path("trial"), encryption.OperationRekeyTarget
	case "old-probe":
		p.label, p.path, p.operation = "OLD", r.path("probe-old"), encryption.OperationOpen
	case "new-probe":
		p.label, p.path, p.operation = "NEW", r.path("probe-new"), encryption.OperationOpen
	case "old-archive-target":
		p.label, p.path, p.operation = "OLD", filepath.Join(r.admission.Root, "inputs/old-archive.gse"), encryption.OperationBackupTarget
	case "old-archive-source":
		p.label, p.path, p.operation = "OLD", filepath.Join(r.admission.Root, "inputs/old-archive.gse"), encryption.OperationOpen
	case "new-restore-target":
		p.label, p.path, p.operation = "NEW", r.path("restore-new"), encryption.OperationRestoreTarget
	case "new-restore-open":
		p.label, p.path, p.operation = "NEW", r.path("restore-new"), encryption.OperationOpen
	default:
		r.require(false, "closed provider purpose", nil)
	}
	r.providers = append(r.providers, p)
	return p
}
func (p *manualRekeyProvider) Acquire(ctx context.Context, request encryption.KeyRequest) (encryption.KeyMaterial, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ctx.Err() != nil || p.acquires != 0 || request.Path != p.path || request.Operation != p.operation || request.Kind != encryption.RawKey256 {
		p.borrow.Invalid()
		return nil, encryption.ErrKeyProvider
	}
	p.acquires++
	p.borrow.Acquired()
	var key [32]byte
	switch p.label {
	case "OLD":
		key = sha256.Sum256([]byte("public-disposable-rekey-interruption-OLD-v1"))
	case "NEW":
		key = sha256.Sum256([]byte("public-disposable-rekey-interruption-NEW-v1"))
	default:
		return nil, encryption.ErrKeyProvider
	}
	return &manualRekeyMaterial{provider: p, key: append([]byte(nil), key[:]...)}, nil
}
func (m *manualRekeyMaterial) Kind() encryption.CredentialKind { return encryption.RawKey256 }
func (m *manualRekeyMaterial) Bytes() []byte                   { return m.key }
func (m *manualRekeyMaterial) Release() {
	m.provider.mu.Lock()
	defer m.provider.mu.Unlock()
	if m.released {
		m.provider.duplicate = true
		m.provider.borrow.Invalid()
		return
	}
	m.released = true
	clear(m.key)
	m.provider.releases++
	m.provider.borrow.Released()
}
func manualRekeyOptions(p *manualRekeyProvider) sqlite.EncryptionOptions {
	return sqlite.EncryptionOptions{KeyProvider: p, CredentialKind: encryption.RawKey256, Mode: sqlite.EncryptionSnapshot}
}
func (r *manualRekeyRun) providerResults() []manualRekeyProviderResult {
	var results []manualRekeyProviderResult
	for _, p := range r.providers {
		p.mu.Lock()
		result := manualRekeyProviderResult{p.purpose, p.label, p.path, p.operation, p.acquires, p.releases, p.duplicate}
		p.mu.Unlock()
		r.require(result.Acquires == 1 && result.Releases == 1 && !result.Duplicate, "single borrow release", nil)
		results = append(results, result)
	}
	return results
}
func (r *manualRekeyRun) path(family string) string {
	return filepath.Join(r.admission.Root, family, manualRekeyRootName)
}

type manualRekeyRow struct {
	IDType                         string
	ID                             int64
	KeyType, Key, ValueType, Value string
}
type manualRekeyQuery struct {
	SQL, Owner     string
	Rows           []manualRekeyRow
	Plan           sqlite.QueryPlan
	Stats          sqlite.QueryStats
	EOF, ErrCalled bool
	CloseCalls     int
}
type manualRekeySQLLedger struct {
	Queries [7]manualRekeyQuery
	Check   sqlite.CheckResult
}

func manualRekeyQueries() [7]string {
	return [7]string{
		"SELECT id,k,v FROM t NOT INDEXED ORDER BY id",
		"SELECT id,k,v FROM t WHERE k='blue' ORDER BY id",
		"SELECT id,k,v FROM t NOT INDEXED WHERE k='blue' ORDER BY id",
		"SELECT id,k,v FROM t WHERE k='red' ORDER BY id",
		"SELECT id,k,v FROM t NOT INDEXED WHERE k='red' ORDER BY id",
		"SELECT id,k,v FROM t WHERE k='green' ORDER BY id",
		"SELECT id,k,v FROM t NOT INDEXED WHERE k='green' ORDER BY id",
	}
}
func manualRekeyExpectedRows(index int) []manualRekeyRow {
	one := manualRekeyRow{"int64", 1, "string", "red", "string", "one"}
	two := manualRekeyRow{"int64", 2, "string", "blue", "string", "two"}
	three := manualRekeyRow{"int64", 3, "string", "red", "string", "three"}
	switch index {
	case 0:
		return []manualRekeyRow{one, two, three}
	case 1, 2:
		return []manualRekeyRow{two}
	case 3, 4:
		return []manualRekeyRow{one, three}
	case 5, 6:
		return []manualRekeyRow{}
	}
	return nil
}
func manualRekeyValidateSQL(ledger manualRekeySQLLedger) bool {
	for i, q := range ledger.Queries {
		if q.SQL != manualRekeyQueries()[i] || q.Owner == "" || !reflect.DeepEqual(q.Rows, manualRekeyExpectedRows(i)) || !q.EOF || !q.ErrCalled || q.CloseCalls != 1 {
			return false
		}
		for j := 0; j < i; j++ {
			if q.Owner == ledger.Queries[j].Owner {
				return false
			}
		}
		p, s := q.Plan, q.Stats
		if p.Table != "t" || s.ReturnedRows != int64(len(q.Rows)) {
			return false
		}
		if i == 1 || i == 3 || i == 5 {
			n := int64(len(q.Rows))
			if p.Access != "SEARCH" || p.Method != "btree" || !reflect.DeepEqual(p.Indexes, []string{"by_k"}) || !reflect.DeepEqual(p.Fields, []string{"k"}) || !p.Recheck || len(p.Reason) > 1024 || s.IndexPageReads <= 0 || s.IndexPageReads > 32 || s.IndexKeysRead != n || s.Candidates != n || s.RowIDSeeks != n || s.BaseRowFetches != n {
				return false
			}
		} else if p.Access != "SCAN" || p.Method != "" || len(p.Indexes) != 0 || len(p.Fields) != 0 || p.Recheck || p.Reason != "NOT INDEXED requested" || s.IndexKeysRead != 0 || s.IndexPageReads != 0 || s.RowIDSeeks != 0 || s.Candidates != 0 || s.BaseRowFetches != 3 {
			return false
		}
	}
	c := ledger.Check
	return c.Scope == sqlite.CheckScope && c.Pages == 3 && c.FreePages == 0 && c.ReservedPages == 0 && c.Tables == 1 && c.Indexes == 1 && c.ExtensionIndexes == 0 && c.TableRows == 3 && c.IndexEntries == 3 && c.PageReads > 0 && c.PageReads <= 512
}
func (r *manualRekeyRun) ledger(db *sqlite.DB, dbOwner *manualRekeyOwner) manualRekeySQLLedger {
	var ledger manualRekeySQLLedger
	for i, text := range manualRekeyQueries() {
		rows, err := db.Query(context.Background(), text)
		owner := r.capture(nil, nil, rows, dbOwner.ID)
		r.require(err == nil && rows != nil, "query acquisition", err)
		q := manualRekeyQuery{SQL: text, Owner: owner.ID, Rows: []manualRekeyRow{}, Plan: rows.Plan()}
		for rows.Next() {
			r.require(len(q.Rows) < 3, "three-row bound", nil)
			values := rows.Values()
			r.require(len(values) == 3, "three columns", nil)
			id, a := values[0].(int64)
			key, b := values[1].(string)
			value, c := values[2].(string)
			r.require(a && b && c, "strict row types", nil)
			q.Rows = append(q.Rows, manualRekeyRow{"int64", id, "string", key, "string", value})
		}
		err = rows.Err()
		q.ErrCalled, q.EOF, q.Stats = true, err == nil, rows.Stats()
		// Failed EOF native retirement is not retried through public Close.
		if err != nil {
			owner.State = "unknown"
			r.ownerPending = true
		}
		r.require(true, "EOF and Rows.Err", err)
		r.require(true, "one public Rows.Close", r.close(owner))
		q.CloseCalls = owner.Attempts
		ledger.Queries[i] = q
	}
	checked, err := db.Check(context.Background(), sqlite.CheckOptions{MaxPages: 3, MaxPageReads: 512})
	r.require(true, "bounded complete Check", err)
	ledger.Check = checked
	r.require(manualRekeyValidateSQL(ledger), "literal seven-query SQL and index ledger", nil)
	return ledger
}
func (r *manualRekeyRun) verify(db *sqlite.DB, path string, expected encryption.Header, expectedHash [32]byte) sqlite.EncryptionInfo {
	info, err := db.VerifyEncrypted(context.Background())
	r.require(true, "authenticated selected head", err)
	raw, _, err := r.read(path, manualRekeyLimit)
	r.require(err == nil && len(raw) == 680, "selected root readback", err)
	header, err := encryption.DecodeHeader(raw[:encryption.HeaderSize], encryption.Limits{})
	r.require(true, "selected header", err)
	r.require(header == expected && sha256.Sum256(raw) == expectedHash && info.Mode == sqlite.EncryptionSnapshot && info.StorageLayout == 1 && info.PageSize == 4096 && info.PageCount == 3 && info.WALSegments == 0 && info.WALFrames == 0 && info.DatabaseID == header.Epoch.DatabaseID && info.CommitSequence == header.Descriptor.CommitSequence && info.HeadCommitID == expectedHash, "exact selected generation", nil)
	return info
}
func (r *manualRekeyRun) family(directory string) (manualRekeyFamily, error) {
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) == 0 || len(entries) > 64 {
		return nil, errors.New("bounded complete family inventory")
	}
	var manifest manualRekeyFamily
	var total int64
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || (name != manualRekeyRootName && name != manualRekeyRootName+".gse-lock" && !(strings.HasPrefix(name, manualRekeyRootName+".gse-") && (strings.HasSuffix(name, ".base") || strings.HasSuffix(name, ".pending") || strings.HasSuffix(name, ".retained")))) {
			return nil, errors.New("unexpected family entry")
		}
		raw, info, err := r.read(filepath.Join(directory, name), manualRekeyLimit)
		if err != nil {
			return nil, err
		}
		total += int64(len(raw))
		if total > 1<<20 {
			return nil, errors.New("family byte bound")
		}
		if name == manualRekeyRootName+".gse-lock" {
			if len(raw) != 0 {
				return nil, errors.New("lock extent")
			}
		} else {
			if len(raw) < encryption.HeaderSize || string(raw[:8]) != encryption.Magic {
				return nil, errors.New("plaintext or malformed family object")
			}
			header, e := encryption.DecodeHeader(raw[:encryption.HeaderSize], encryption.Limits{})
			if e != nil {
				return nil, e
			}
			if name != manualRekeyRootName {
				suffix := filepath.Ext(name)
				if name != manualRekeyRootName+".gse-"+hex.EncodeToString(header.FileID[:])+suffix {
					return nil, errors.New("family name/identity mismatch")
				}
				if suffix == ".base" && header.Descriptor.Role != encryption.FileBase || suffix != ".base" && header.Descriptor.Role != encryption.FileRoot {
					return nil, errors.New("family role/name mismatch")
				}
			} else if header.Descriptor.Role != encryption.FileRoot {
				return nil, errors.New("selected root role")
			}
		}
		s := info.Sys().(*syscall.Stat_t)
		artifact := manualRekeyArtifact{name, uint64(s.Dev), s.Ino, info.Size(), sha256.Sum256(raw)}
		for _, previous := range manifest {
			if previous.Device == artifact.Device && previous.Inode == artifact.Inode {
				return nil, errors.New("family inode alias")
			}
		}
		manifest = append(manifest, artifact)
	}
	sort.Slice(manifest, func(i, j int) bool { return manifest[i].Name < manifest[j].Name })
	return manifest, nil
}
func manualRekeySameBytes(a, b manualRekeyFamily, distinct bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Bytes != b[i].Bytes || a[i].Hash != b[i].Hash {
			return false
		}
		if distinct && a[i].Device == b[i].Device && a[i].Inode == b[i].Inode {
			return false
		}
	}
	return true
}
func (r *manualRekeyRun) copyFamily(source, destination string) (manualRekeyFamily, error) {
	before, err := r.family(source)
	if err != nil {
		return nil, err
	}
	if err := r.mkdir(destination); err != nil {
		return nil, err
	}
	for _, artifact := range before {
		raw, _, err := r.read(filepath.Join(source, artifact.Name), manualRekeyLimit)
		if err != nil {
			return nil, err
		}
		if _, err := r.writeExclusive(filepath.Join(destination, artifact.Name), raw, false); err != nil {
			return nil, err
		}
	}
	if err := r.syncDirectory(destination); err != nil {
		return nil, err
	}
	after, err := r.family(source)
	if err != nil || !reflect.DeepEqual(before, after) {
		return nil, errors.New("source family mutated")
	}
	copied, err := r.family(destination)
	if err != nil || !manualRekeySameBytes(before, copied, true) {
		return nil, errors.New("complete independent copy failed")
	}
	for _, a := range before {
		for _, b := range copied {
			if a.Device == b.Device && a.Inode == b.Inode {
				return nil, errors.New("cross-family inode alias")
			}
		}
	}
	return copied, nil
}

type manualRekeyDiagnostic struct {
	mu       sync.Mutex
	data     []byte
	overflow bool
}

func (d *manualRekeyDiagnostic) Write(raw []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.data)+len(raw) > manualRekeyLimit {
		d.overflow = true
		return 0, io.ErrShortWrite
	}
	d.data = append(d.data, raw...)
	return len(raw), nil
}
func (d *manualRekeyDiagnostic) snapshot() ([]byte, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]byte(nil), d.data...), d.overflow
}

type manualRekeyWait struct {
	State *os.ProcessState
	Err   error
}
type manualRekeyChild struct {
	command                                                       *exec.Cmd
	binding                                                       encstore.ManualRekeyBinding
	control, events                                               *os.File
	controlOwner, eventOwner                                      *manualRekeyOwner
	stdout, stderr                                                manualRekeyDiagnostic
	wait                                                          chan manualRekeyWait
	actual                                                        manualRekeyWait
	started, waited, live, unknown, receiptAttempted, terminalEOF bool
	sequence                                                      uint8
	frames                                                        [][]byte
	last                                                          encstore.ManualRekeyFrame
}
type manualRekeyProcessReceipt struct {
	Role, Generation, Challenge, Outcome                  string
	Starts, Waits                                         int
	ExitCode                                              int
	Exited, Signaled, ChildLive, WaitUnknown, TerminalEOF bool
	Frames                                                [][32]byte
	Stdout, Stderr                                        [32]byte
	PhysicalValidated                                     bool
}

func (r *manualRekeyRun) start(role string) *manualRekeyChild {
	r.require(r.role == "controller" && role != "controller" && manualRekeyRoleRoot(role) != "", "parent-only closed role spawn", nil)
	for _, previous := range r.children {
		r.require(previous.binding.Role != role && previous.waited, "no duplicate or overlapping live child", nil)
	}
	raw, info, err := r.read(r.admission.BinaryPath, 128<<20)
	r.require(err == nil && os.SameFile(info, r.selfInfo) && manualRekeyHash(raw) == r.admission.Binary, "fixed self-image before Start", err)
	var nonce [32]byte
	_, err = rand.Read(nonce[:])
	r.require(nonce != [32]byte{}, "parent challenge", err)
	binding := r.binding()
	binding.Role, binding.Generation, binding.Challenge = role, manualRekeyGeneration(role), hex.EncodeToString(nonce[:])
	controlRead, controlWrite, err := os.Pipe()
	r.require(true, "owned control pipe", err)
	cr := r.capture(controlRead, nil, nil, "")
	cw := r.capture(controlWrite, nil, nil, "")
	eventRead, eventWrite, err := os.Pipe()
	r.require(true, "owned event pipe", err)
	er := r.capture(eventRead, nil, nil, "")
	ew := r.capture(eventWrite, nil, nil, "")
	child := &manualRekeyChild{binding: binding, control: controlWrite, events: eventRead, controlOwner: cw, eventOwner: er, wait: make(chan manualRekeyWait, 1)}
	command := exec.Command(r.admission.BinaryPath, manualRekeyFlags(role)...)
	command.Env = []string{"GOFLAGS=", "GOSQLITE_MANUAL_REKEY_INTERRUPTION=independently_authorized", "GOSQLITE_REKEY_ROLE=" + role, "GOSQLITE_REKEY_BINDING=" + r.bindingPath, "GOSQLITE_REKEY_BINDING_SHA256=" + r.bindingHash, "GOSQLITE_REKEY_CHALLENGE=" + binding.Challenge}
	command.Dir = filepath.Join(r.admission.Root, "control")
	command.ExtraFiles = []*os.File{controlRead, eventWrite}
	command.Stdout, command.Stderr = &child.stdout, &child.stderr
	child.command = command
	r.children = append(r.children, child)
	err = command.Start()
	if err != nil {
		child.unknown = true
		r.require(false, "Start failed; concrete owner retained", err)
	}
	child.started = true
	go func() { err := command.Wait(); child.wait <- manualRekeyWait{State: command.ProcessState, Err: err} }()
	r.require(true, "parent duplicate control-reader Close", r.close(cr))
	r.require(true, "parent duplicate event-writer Close", r.close(ew))
	ready, frame := r.receive(child)
	r.require(ready.Kind == "role-ready" && ready.Point == 0 && ready.Observation == (encstore.ManualRekeyObservation{}) && ready.PayloadHash == [32]byte{}, "verified child ready", nil)
	r.grant(child, ready, frame, "continue")
	return child
}
func (r *manualRekeyRun) receive(child *manualRekeyChild) (encstore.ManualRekeyFrame, []byte) {
	type received struct {
		frame encstore.ManualRekeyFrame
		raw   []byte
		err   error
	}
	result := make(chan received, 1)
	go func() {
		frame, raw, err := encstore.ManualRekeyReadFrame(child.events)
		result <- received{frame, raw, err}
	}()
	select {
	case event := <-result:
		r.require(event.err == nil, "bounded event frame", event.err)
		child.sequence++
		r.require(event.frame.Binding == child.binding && event.frame.Sequence == child.sequence && child.sequence <= encstore.ManualRekeyFrameCount, "exact fresh causal event", nil)
		child.frames = append(child.frames, append([]byte(nil), event.raw...))
		child.last = event.frame
		return event.frame, event.raw
	case <-time.After(manualRekeyWaitLimit):
		child.live, child.unknown = true, true
		r.saveProcess(child, "CHILD_LIVE/WAIT_UNKNOWN")
		r.require(false, "child barrier unresolved; no kill or dependent probe", nil)
	}
	return encstore.ManualRekeyFrame{}, nil
}
func (r *manualRekeyRun) grant(child *manualRekeyChild, event encstore.ManualRekeyFrame, raw []byte, kind string) {
	frame := event
	frame.Kind, frame.PayloadHash = kind, sha256.Sum256(raw)
	_, err := encstore.ManualRekeyWriteFrame(child.control, frame)
	r.require(true, "parent causal grant", err)
}
func (r *manualRekeyRun) waitChild(child *manualRekeyChild, expected int) {
	r.require(child.started && !child.waited && !child.live && !child.unknown, "one real Wait required", nil)
	select {
	case result := <-child.wait:
		child.actual = result
		if result.State == nil {
			child.unknown = true
			r.saveProcess(child, "WAIT_UNKNOWN")
			// Wait returned without an actual process state. No second Wait
			// can repair this receipt; retain the controller for disposition.
			manualRekeyHoldUnknown()
		}
		child.waited = true
	case <-time.After(manualRekeyWaitLimit):
		child.live, child.unknown = true, true
		r.saveProcess(child, "CHILD_LIVE/WAIT_UNKNOWN")
		r.require(false, "actual Wait unresolved; child retained", nil)
	}
	state := child.actual.State
	valid := state != nil && state.Exited() && state.ExitCode() == expected
	if state != nil {
		status, ok := state.Sys().(syscall.WaitStatus)
		valid = valid && ok && !status.Signaled()
	}
	if expected == 0 {
		valid = valid && child.actual.Err == nil
	} else {
		var exit *exec.ExitError
		valid = valid && errors.As(child.actual.Err, &exit) && exit.ProcessState == state
	}
	_, stdoutOverflow := child.stdout.snapshot()
	_, stderrOverflow := child.stderr.snapshot()
	valid = valid && !stdoutOverflow && !stderrOverflow
	if valid {
		// After actual Wait, all inherited event writers must be gone. One
		// byte is enough to reject a duplicate frame, suffix or trailing data.
		var trailing [1]byte
		n, endErr := child.events.Read(trailing[:])
		child.terminalEOF = n == 0 && endErr == io.EOF
	}
	r.saveProcess(child, "WAIT_OBSERVED")
	r.require(valid && child.terminalEOF, "exact actual exit, diagnostics and terminal protocol EOF", manualRekeyWaitUnexpected(child.actual.Err, expected))
	r.require(true, "parent control writer retired once", r.close(child.controlOwner))
	r.require(true, "parent event reader retired once", r.close(child.eventOwner))
}
func manualRekeyWaitUnexpected(err error, expected int) error {
	if expected == encstore.ManualRekeyExitCode {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil
		}
	}
	return err
}
func (r *manualRekeyRun) saveProcess(child *manualRekeyChild, outcome string) {
	if child.receiptAttempted {
		return
	}
	child.receiptAttempted = true
	receipt := manualRekeyProcessReceipt{Role: child.binding.Role, Generation: child.binding.Generation, Challenge: child.binding.Challenge, Outcome: outcome, ExitCode: -1, ChildLive: child.live, WaitUnknown: child.unknown, TerminalEOF: child.terminalEOF, PhysicalValidated: false}
	if child.started {
		receipt.Starts = 1
	}
	if child.waited {
		receipt.Waits = 1
	}
	if child.actual.State != nil {
		receipt.Exited, receipt.ExitCode = child.actual.State.Exited(), child.actual.State.ExitCode()
		if s, ok := child.actual.State.Sys().(syscall.WaitStatus); ok {
			receipt.Signaled = s.Signaled()
		}
	}
	for _, raw := range child.frames {
		receipt.Frames = append(receipt.Frames, sha256.Sum256(raw))
	}
	stdout, _ := child.stdout.snapshot()
	stderr, _ := child.stderr.snapshot()
	receipt.Stdout, receipt.Stderr = sha256.Sum256(stdout), sha256.Sum256(stderr)
	prefix := filepath.Join(r.admission.Root, "control", child.binding.Role)
	_, a := r.writeExclusive(prefix+".stdout", stdout, true)
	_, b := r.writeExclusive(prefix+".stderr", stderr, true)
	var transcript []byte
	for _, frame := range child.frames {
		transcript = append(transcript, frame...)
	}
	_, transcriptErr := r.writeExclusive(prefix+".events.bin", transcript, true)
	raw, c := json.Marshal(receipt)
	_, d := r.writeExclusive(prefix+".process.json", raw, true)
	if errors.Join(a, b, c, d, transcriptErr) != nil {
		r.t.Error("process receipt incomplete; no retry")
	}
}
func (r *manualRekeyRun) acknowledge(child *manualRekeyChild, event encstore.ManualRekeyFrame, raw []byte) {
	r.require(r.role == "controller" && r.admission.Case == "acknowledged-exit" && child.binding.Role == "writer" && event.Kind == "public-nil" && event == child.last && len(child.frames) == 5 && bytes.Equal(raw, child.frames[4]), "parent acknowledgement of exact received nil frame", nil)
	artifact, err := r.writeExclusive(filepath.Join(r.admission.Root, "control/parent-ack.frame"), raw, true)
	r.require(err == nil && artifact.Hash == sha256.Sum256(raw), "ACK full write/file Sync/Close/directory Sync/Close/readback", err)
	// writeExclusive creates a new one-link inode. Check all fixture families
	// rather than trusting a source pathname or a child's claim of an ACK.
	for _, family := range []string{"inputs/original", "trial"} {
		manifest, err := r.family(filepath.Join(r.admission.Root, family))
		r.require(true, "ACK nonalias inventory", err)
		for _, entry := range manifest {
			r.require(entry.Device != artifact.Device || entry.Inode != artifact.Inode, "ACK/source inode separation", nil)
		}
	}
	r.grant(child, event, raw, "ack-exit")
}

type manualRekeySeedRecord struct {
	BindingHash     string
	Header          encryption.Header
	Hash            [32]byte
	Info            sqlite.EncryptionInfo
	SQL             manualRekeySQLLedger
	Family          manualRekeyFamily
	ArchiveHeader   encryption.Header
	ArchiveHash     [32]byte
	ArchiveInfo     sqlite.EncryptionInfo
	ArchiveArtifact manualRekeyArtifact
}

func (r *manualRekeyRun) seed() manualRekeySeedRecord {
	for _, directory := range []string{"inputs", "inputs/original", "evidence", "control"} {
		r.require(true, "new retained directory", r.mkdir(filepath.Join(r.admission.Root, directory)))
	}
	provider := r.provider("old-seed")
	options := manualRekeyOptions(provider)
	db, err := sqlite.OpenContext(context.Background(), r.path("inputs/original"), sqlite.Options{PageSize: 4096, Encryption: &options})
	owner := r.capture(nil, db, nil, "")
	r.require(err == nil && db != nil, "one seed create", err)
	for _, statement := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT,v TEXT)", "CREATE INDEX by_k ON t(k)", "INSERT INTO t VALUES(1,'red','one'),(2,'blue','two'),(3,'red','three')"} {
		_, err := db.Exec(context.Background(), statement)
		r.require(true, "fixed seed statement", err)
	}
	raw, _, err := r.read(r.path("inputs/original"), manualRekeyLimit)
	r.require(err == nil && len(raw) == 680, "layout-1 root extent", err)
	header, err := encryption.DecodeHeader(raw[:encryption.HeaderSize], encryption.Limits{})
	r.require(true, "baseline public header", err)
	record := manualRekeySeedRecord{BindingHash: r.bindingHash, Header: header, Hash: sha256.Sum256(raw)}
	record.Info = r.verify(db, r.path("inputs/original"), header, record.Hash)
	r.require(header.Descriptor.CommitSequence == 4 && header.Descriptor.PageCount == 3 && header.Epoch.Kind == encryption.RawKey256, "fixed sequence and geometry", nil)
	record.SQL = r.ledger(db, owner)
	if r.admission.ArchiveControl {
		archiveProvider := r.provider("old-archive-target")
		info, err := db.BackupEncrypted(context.Background(), archiveProvider.path, manualRekeyOptions(archiveProvider))
		r.require(true, "separate old archive", err)
		archive, archiveStat, err := r.read(archiveProvider.path, manualRekeyLimit)
		r.require(err == nil && len(archive) >= encryption.HeaderSize, "archive extent", err)
		record.ArchiveHeader, err = encryption.DecodeHeader(archive[:encryption.HeaderSize], encryption.Limits{})
		r.require(true, "archive public identity", err)
		record.ArchiveHash, record.ArchiveInfo = sha256.Sum256(archive), info
		archiveIdentity := archiveStat.Sys().(*syscall.Stat_t)
		record.ArchiveArtifact = manualRekeyArtifact{"old-archive.gse", uint64(archiveIdentity.Dev), archiveIdentity.Ino, archiveStat.Size(), record.ArchiveHash}
		r.require(info.DatabaseID != record.Info.DatabaseID && record.ArchiveHeader.Descriptor.Role == encryption.FileBackup && record.ArchiveHeader.Epoch.DatabaseID == info.DatabaseID, "fresh portable archive identity", nil)
	}
	r.require(true, "seed DB closed once", r.close(owner))
	r.require(encstore.ManualRekeyOwners() == (encstore.ManualRekeyOwnerSnapshot{}), "seed ownership retired", nil)
	record.Family, err = r.family(filepath.Join(r.admission.Root, "inputs/original"))
	r.require(true, "closed immutable source family", err)
	r.baseline = record.Family
	raw, err = json.Marshal(record)
	r.require(true, "seed record encoding", err)
	_, err = r.writeExclusive(filepath.Join(r.admission.Root, "control/seed.json"), raw, true)
	r.require(true, "retained seed record", err)
	r.providerResults()
	return record
}
func (r *manualRekeyRun) loadSeed() manualRekeySeedRecord {
	var seed manualRekeySeedRecord
	raw, _, err := r.read(filepath.Join(r.admission.Root, "control/seed.json"), 4<<20)
	r.require(true, "seed record", err)
	r.require(true, "seed record canonical", manualRekeyDecode(raw, &seed))
	r.require(seed.BindingHash == r.bindingHash && seed.Header.Descriptor.CommitSequence == 4 && seed.Info.CommitSequence == 4 && manualRekeyValidateSQL(seed.SQL), "frozen authenticated baseline claim", nil)
	manifest, err := r.family(filepath.Join(r.admission.Root, "inputs/original"))
	r.require(err == nil && reflect.DeepEqual(manifest, seed.Family), "original source unchanged", err)
	return seed
}
func manualRekeyObservationValid(observation encstore.ManualRekeyObservation, seed manualRekeySeedRecord) bool {
	a, b := observation.Old, observation.Candidate
	return observation.OldBorrow == (encstore.ManualRekeyBorrowSnapshot{Acquires: 1, Releases: 1}) && observation.NewBorrow == (encstore.ManualRekeyBorrowSnapshot{Acquires: 1, Releases: 1}) && observation.OldHash == seed.Hash && a == seed.Header && observation.CandidateHash != [32]byte{} && observation.CandidateHash != seed.Hash && b.Validate(encryption.Limits{}) == nil && b.Descriptor.Role == encryption.FileRoot && b.Descriptor.PageSize == 4096 && b.Descriptor.PageCount == 3 && b.Descriptor.RecordCount == 1 && b.Descriptor.CommitSequence == 5 && b.Descriptor.ParentCommitID == seed.Hash && b.Epoch.Kind == encryption.RawKey256 && b.Epoch.DatabaseID == a.Epoch.DatabaseID && b.Epoch.EpochID != a.Epoch.EpochID && b.Epoch.RootSalt != a.Epoch.RootSalt && b.FileID != a.FileID
}
func (r *manualRekeyRun) saveRole() {
	r.result.Providers = r.providerResults()
	for _, o := range r.owners {
		r.result.Owners = append(r.result.Owners, manualRekeyOwnerResult{o.ID, o.Parent, o.State, o.Attempts})
	}
	r.result.Registry = encstore.ManualRekeyOwners()
	r.require(r.result.Registry == (encstore.ManualRekeyOwnerSnapshot{}) && !r.ownerPending, "post-role registry empty", nil)
	r.result.PhysicalValidated = false
	raw, err := json.Marshal(r.result)
	r.require(true, "role receipt encoding", err)
	artifact, err := r.writeExclusive(filepath.Join(r.admission.Root, "control", r.role+".result.json"), raw, true)
	r.require(true, "role receipt retained", err)
	grant, err := r.channel.Exchange("role-result", 0, encstore.ManualRekeyObservation{}, artifact.Hash)
	r.require(err == nil && grant == "done", "role completion grant", err)
	r.require(true, "child fixed pipes closed once", r.channel.Close())
}
func TestManualRekeyInterruptionWriter(t *testing.T) {
	r := manualRekeyAdmit(t, "writer")
	defer r.finish()
	seed := r.loadSeed()
	old := r.provider("old-writer")
	options := manualRekeyOptions(old)
	db, err := sqlite.OpenContext(context.Background(), old.path, sqlite.Options{NoCreate: true, Encryption: &options})
	owner := r.capture(nil, db, nil, "")
	r.require(err == nil && db != nil, "one writer OLD Open", err)
	r.verify(db, old.path, seed.Header, seed.Hash)
	r.ledger(db, owner)
	target := r.provider("new-rekey")
	ctx, err := encstore.ManualRekeyContext(context.Background(), old.path, seed.Hash, seed.Header, r.channel, &old.borrow, &target.borrow)
	r.require(true, "single observation context", err)
	rekeyErr := db.Rekey(ctx, manualRekeyOptions(target))
	capturedRekeyErr := rekeyErr
	r.causes = append(r.causes, capturedRekeyErr)
	r.require(capturedRekeyErr == nil, "original public Rekey returned nil", capturedRekeyErr)
	r.providerResults()
	r.require(true, "nil return barrier", encstore.ManualRekeyReturnedNil(ctx))
	// Only control returns through this point. Interrupted paths deliberately
	// skip native closure and all Go defers, and never synthesize a public error.
	r.require(r.admission.Case == "control", "control alone reaches normal Close", nil)
	r.require(true, "control DB.Close once", r.close(owner))
	r.result.OpenCalls, r.result.CloseCalls, r.result.Outcome = 1, 1, "CONTROL_CLOSED"
	r.saveRole()
}
func manualRekeyWrongKey(err error, db *sqlite.DB) bool {
	if db != nil || err == nil {
		return false
	}
	detail, ok := err.(*sqlite.Error)
	if !ok || detail.Op != "open" || detail.Code != sqlite.CodeNotADB || !errors.Is(err, encryption.ErrAuthentication) {
		return false
	}
	var pathError *os.PathError
	var syscallError syscall.Errno
	if errors.As(err, &pathError) || errors.As(err, &syscallError) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.ErrShortWrite) {
		return false
	}
	for _, forbidden := range []error{sqlite.ErrEncryptedCleanupPending, sqlite.ErrPoisoned, sqlite.ErrBusy, sqlite.ErrChanged, sqlite.ErrRecoveryRequired, sqlite.ErrCommitUncertain, sqlite.ErrCorrupt, sqlite.ErrMemoryLimit, sqlite.ErrUnsupported, context.Canceled, context.DeadlineExceeded, encryption.ErrFormat, encryption.ErrLimit, encryption.ErrKeyProvider, encryption.ErrCredential, encryption.ErrUnsupported, encryption.ErrIdentity, encryption.ErrRandomness} {
		if errors.Is(err, forbidden) {
			return false
		}
	}
	return len(sqlite.PendingEncryptedCleanups()) == 0 && encstore.ManualRekeyOwners() == (encstore.ManualRekeyOwnerSnapshot{})
}
func (r *manualRekeyRun) probe(purpose string) {
	seed := r.loadSeed()
	var selected encstore.ManualRekeyObservation
	raw, _, err := r.read(filepath.Join(r.admission.Root, "control/selected.json"), manualRekeyLimit)
	r.require(true, "selected generation claim", err)
	r.require(true, "selected generation encoding", manualRekeyDecode(raw, &selected))
	r.require(manualRekeyObservationValid(selected, seed), "authenticated writer observation", nil)
	provider := r.provider(purpose)
	options := manualRekeyOptions(provider)
	before, err := r.family(filepath.Dir(provider.path))
	r.require(true, "probe frozen copy", err)
	// Purpose is fixed by this role before any disk header or failed Open.
	db, openErr := sqlite.OpenContext(context.Background(), provider.path, sqlite.Options{ReadOnly: true, NoCreate: true, Encryption: &options})
	capturedOpenErr := openErr
	owner := r.capture(nil, db, nil, "")
	r.causes = append(r.causes, capturedOpenErr)
	r.result.OpenCalls, r.result.NilDB = 1, db == nil
	expectOld := r.admission.Case == "before-root-rename"
	expectSuccess := (purpose == "old-probe") == expectOld
	if !expectSuccess {
		r.require(manualRekeyWrongKey(capturedOpenErr, db), "exact wrong-key Open authentication", manualRekeyWrongKeyUnexpected(capturedOpenErr, db))
		r.result.Outcome, r.result.Code = "EXPECTED_AUTHENTICATION", 26
	} else {
		r.require(capturedOpenErr == nil && db != nil, "correct fixed credential Open", capturedOpenErr)
		header, hash := selected.Candidate, selected.CandidateHash
		if expectOld {
			header, hash = seed.Header, seed.Hash
		}
		r.result.Info = r.verify(db, provider.path, header, hash)
		r.result.Header, r.result.RootHash, r.result.Authenticated = header, hash, true
		r.result.SQL = r.ledger(db, owner)
		r.require(true, "probe DB.Close once", r.close(owner))
		r.result.CloseCalls, r.result.Outcome = 1, "AUTHENTICATED_LEDGER"
	}
	after, err := r.family(filepath.Dir(provider.path))
	r.require(err == nil && reflect.DeepEqual(before, after), "probe copy unchanged", err)
	r.saveRole()
}
func manualRekeyWrongKeyUnexpected(err error, db *sqlite.DB) error {
	if manualRekeyWrongKey(err, db) {
		return nil
	}
	return err
}
func TestManualRekeyInterruptionOldProbe(t *testing.T) {
	r := manualRekeyAdmit(t, "old-probe")
	defer r.finish()
	r.probe("old-probe")
}
func TestManualRekeyInterruptionNewProbe(t *testing.T) {
	r := manualRekeyAdmit(t, "new-probe")
	defer r.finish()
	r.probe("new-probe")
}
func TestManualRekeyInterruptionArchive(t *testing.T) {
	r := manualRekeyAdmit(t, "archive-check")
	defer r.finish()
	r.require(r.admission.Case == "control" && r.admission.ArchiveControl, "optional control archive only", nil)
	seed := r.loadSeed()
	source, target := r.provider("old-archive-source"), r.provider("new-restore-target")
	archive, _, err := r.read(source.path, manualRekeyLimit)
	r.require(err == nil && sha256.Sum256(archive) == seed.ArchiveHash, "unchanged old archive", err)
	_, absentErr := os.Lstat(target.path)
	r.require(errors.Is(absentErr, os.ErrNotExist), "absent restore target", nil)
	info, err := sqlite.RestoreEncrypted(context.Background(), source.path, target.path, manualRekeyOptions(source), manualRekeyOptions(target))
	r.require(true, "old archive into new target", err)
	r.require(info.DatabaseID != seed.Info.DatabaseID && info.DatabaseID != seed.ArchiveInfo.DatabaseID && info.PageSize == 4096 && info.PageCount == 3, "fresh restored DBID", nil)
	openProvider := r.provider("new-restore-open")
	options := manualRekeyOptions(openProvider)
	db, err := sqlite.OpenContext(context.Background(), openProvider.path, sqlite.Options{ReadOnly: true, NoCreate: true, Encryption: &options})
	owner := r.capture(nil, db, nil, "")
	r.require(err == nil && db != nil, "one NEW restored Open", err)
	root, _, err := r.read(target.path, manualRekeyLimit)
	r.require(err == nil && len(root) == 680, "restored root", err)
	header, err := encryption.DecodeHeader(root[:encryption.HeaderSize], encryption.Limits{})
	r.require(true, "restored public header", err)
	r.require(header.Epoch.DatabaseID == info.DatabaseID && header.Epoch.EpochID != seed.Header.Epoch.EpochID && header.Epoch.EpochID != seed.ArchiveHeader.Epoch.EpochID && header.FileID != seed.Header.FileID && header.FileID != seed.ArchiveHeader.FileID, "fresh restore epoch and file identities", nil)
	r.result.Info = r.verify(db, target.path, header, sha256.Sum256(root))
	r.result.Header, r.result.RootHash = header, sha256.Sum256(root)
	r.result.SQL = r.ledger(db, owner)
	r.require(true, "restored DB.Close once", r.close(owner))
	r.result.OpenCalls, r.result.CloseCalls, r.result.Authenticated, r.result.Outcome = 1, 1, true, "ARCHIVE_RESTORED_NEW_TARGET"
	after, afterStat, err := r.read(source.path, manualRekeyLimit)
	r.require(err == nil && bytes.Equal(archive, after), "archive unchanged after restore", err)
	archiveIdentity := afterStat.Sys().(*syscall.Stat_t)
	r.require(seed.ArchiveArtifact == (manualRekeyArtifact{"old-archive.gse", uint64(archiveIdentity.Dev), archiveIdentity.Ino, afterStat.Size(), sha256.Sum256(after)}), "archive identity unchanged", nil)
	r.loadSeed()
	r.saveRole()
}

// Public derivation-input observations only. Whole-object hashes identify
// historical encryption events; copies/retained aliases never mint new events.
type manualRekeyNonce struct {
	Credential                string
	DatabaseID                [16]byte
	EpochID, RootSalt, FileID [32]byte
	FileRole                  encryption.FileRole
	RecordRole                encryption.RecordRole
	Offset, Extent, Sequence  uint64
	Prefix                    [4]byte
	Nonce                     [12]byte
	ObjectHash                [32]byte
}

func manualRekeyNonceSameDomain(a, b manualRekeyNonce) bool {
	return a.Credential == b.Credential && a.DatabaseID == b.DatabaseID && a.EpochID == b.EpochID && a.RootSalt == b.RootSalt && a.FileID == b.FileID && a.FileRole == b.FileRole && a.RecordRole == b.RecordRole
}
func (r *manualRekeyRun) nonceLedger(seed manualRekeySeedRecord, selected encstore.ManualRekeyObservation) []manualRekeyNonce {
	var result []manualRekeyNonce
	objects := map[[32]byte]bool{}
	var paths []string
	for _, family := range []string{"inputs/original", "evidence/post-exit"} {
		manifest, err := r.family(filepath.Join(r.admission.Root, family))
		r.require(true, "nonce family inventory", err)
		for _, artifact := range manifest {
			if artifact.Name != manualRekeyRootName+".gse-lock" {
				paths = append(paths, filepath.Join(r.admission.Root, family, artifact.Name))
			}
		}
	}
	if r.admission.ArchiveControl {
		paths = append(paths, filepath.Join(r.admission.Root, "inputs/old-archive.gse"))
		manifest, err := r.family(filepath.Join(r.admission.Root, "restore-new"))
		r.require(true, "restore nonce family", err)
		for _, artifact := range manifest {
			if artifact.Name != manualRekeyRootName+".gse-lock" {
				paths = append(paths, filepath.Join(r.admission.Root, "restore-new", artifact.Name))
			}
		}
	}
	newBase := 0
	for _, path := range paths {
		raw, _, err := r.read(path, manualRekeyLimit)
		r.require(err == nil && len(raw) >= encryption.HeaderSize, "bounded nonce object", err)
		hash := sha256.Sum256(raw)
		if objects[hash] {
			continue
		}
		objects[hash] = true
		header, err := encryption.DecodeHeader(raw[:encryption.HeaderSize], encryption.Limits{})
		r.require(true, "nonce public header", err)
		d := header.Descriptor
		r.require(header.Epoch.Kind == encryption.RawKey256 && header.Epoch.KDF == (encryption.Argon2Params{}) && header.Epoch.KDFSalt == [32]byte{} && d.PageSize == 4096 && d.PageCount >= 1 && d.PageCount <= 3 && d.RecordCount >= 1 && d.RecordCount <= 5, "bounded raw-key public domain", nil)
		label := "OLD"
		if header.Epoch.EpochID == selected.Candidate.Epoch.EpochID || filepath.Dir(path) == filepath.Join(r.admission.Root, "restore-new") {
			label = "NEW"
		}
		if header.Epoch.EpochID == selected.Candidate.Epoch.EpochID && d.Role == encryption.FileBase {
			newBase++
			r.require(header.Epoch == selected.Candidate.Epoch && d.PageCount == 3 && d.CommitSequence == 5 && d.ParentCommitID == seed.Hash && header.FileID != seed.Header.FileID && header.FileID != selected.Candidate.FileID, "fresh candidate base domain", nil)
		}
		offset := encryption.HeaderSize
		for sequence := uint64(0); sequence < d.RecordCount; sequence++ {
			r.require(offset+encryption.RecordHeaderSize <= len(raw), "record header extent", nil)
			record, err := encryption.DecodeRecordHeader(raw[offset:offset+encryption.RecordHeaderSize], encryption.Limits{})
			r.require(true, "bounded record header", err)
			wantRole, wantBytes := encryption.RecordMetadata, uint32(64)
			if d.Role == encryption.FileRoot {
				wantBytes = 128
			} else if sequence == d.RecordCount-1 {
				wantRole, wantBytes = encryption.RecordFooter, 48
			} else if sequence != 0 {
				wantRole, wantBytes = encryption.RecordData, 4096
			}
			extent := encryption.RecordHeaderSize + int(record.PlaintextSize) + encryption.TagSize
			r.require((d.Role == encryption.FileRoot || d.Role == encryption.FileBase || d.Role == encryption.FileBackup) && record.Role == wantRole && record.Sequence == sequence && record.PlaintextSize == wantBytes && offset+extent <= len(raw), "exact record roles/sequences/extents", nil)
			entry := manualRekeyNonce{Credential: label, DatabaseID: header.Epoch.DatabaseID, EpochID: header.Epoch.EpochID, RootSalt: header.Epoch.RootSalt, FileID: header.FileID, FileRole: d.Role, RecordRole: record.Role, Offset: uint64(offset), Extent: uint64(extent), Sequence: sequence, Prefix: header.NoncePrefix, ObjectHash: hash}
			copy(entry.Nonce[:4], entry.Prefix[:])
			binary.BigEndian.PutUint64(entry.Nonce[4:], sequence)
			for _, prior := range result {
				if manualRekeyNonceSameDomain(prior, entry) {
					r.require(prior.Nonce != entry.Nonce && (prior.ObjectHash == entry.ObjectHash || prior.Sequence != entry.Sequence), "distinct event cannot reuse domain/nonce/sequence", nil)
				}
				if prior.FileID == entry.FileID && prior.ObjectHash != entry.ObjectHash {
					r.require(false, "same file identity with inconsistent ciphertext", nil)
				}
			}
			result = append(result, entry)
			r.require(len(result) <= 320, "nonce record bound", nil)
			offset += extent
		}
		r.require(offset == len(raw), "no trailing ciphertext records", nil)
	}
	r.require(newBase == 1, "exact one fresh target base", nil)
	return result
}
func (r *manualRekeyRun) receiveRole(child *manualRekeyChild) manualRekeyRoleResult {
	frame, raw := r.receive(child)
	r.require(frame.Kind == "role-result" && frame.Point == 0 && frame.Observation == (encstore.ManualRekeyObservation{}) && frame.PayloadHash != [32]byte{}, "exact role result event", nil)
	r.grant(child, frame, raw, "done")
	r.waitChild(child, 0)
	data, _, err := r.read(filepath.Join(r.admission.Root, "control", child.binding.Role+".result.json"), 4<<20)
	r.require(err == nil && sha256.Sum256(data) == frame.PayloadHash, "waited role receipt hash", err)
	var result manualRekeyRoleResult
	r.require(true, "canonical role receipt", manualRekeyDecode(data, &result))
	r.require(result.Role == child.binding.Role && result.Generation == child.binding.Generation && !result.PhysicalValidated && result.Registry == (encstore.ManualRekeyOwnerSnapshot{}), "role identity and bookkeeping", nil)
	wantPurposes := []string{}
	switch child.binding.Role {
	case "writer":
		wantPurposes = []string{"old-writer", "new-rekey"}
	case "old-probe":
		wantPurposes = []string{"old-probe"}
	case "new-probe":
		wantPurposes = []string{"new-probe"}
	case "archive-check":
		wantPurposes = []string{"old-archive-source", "new-restore-target", "new-restore-open"}
	}
	r.require(len(result.Providers) == len(wantPurposes), "exact role provider inventory", nil)
	for i, provider := range result.Providers {
		purpose := wantPurposes[i]
		label, path, operation := "OLD", r.path("trial"), encryption.OperationOpen
		switch purpose {
		case "new-rekey":
			label, operation = "NEW", encryption.OperationRekeyTarget
		case "old-probe":
			path = r.path("probe-old")
		case "new-probe":
			label, path = "NEW", r.path("probe-new")
		case "old-archive-source":
			path = filepath.Join(r.admission.Root, "inputs/old-archive.gse")
		case "new-restore-target":
			label, path, operation = "NEW", r.path("restore-new"), encryption.OperationRestoreTarget
		case "new-restore-open":
			label, path = "NEW", r.path("restore-new")
		}
		r.require(provider.Purpose == purpose && provider.Label == label && provider.Path == path && provider.Operation == operation && provider.Acquires == 1 && provider.Releases == 1 && !provider.Duplicate, "immutable role purpose and borrow retirement", nil)
	}
	for _, owner := range result.Owners {
		r.require(owner.State == "absent" && owner.Attempts == 0 || owner.State == "closed" && owner.Attempts == 1, "role one-time owner retirement", nil)
	}
	return result
}
func TestManualRekeyInterruptionController(t *testing.T) {
	r := manualRekeyAdmit(t, "controller")
	defer r.finish()
	r.require(true, "new run directory must not exist", r.mkdir(r.admission.Root))
	seed := r.seed()
	_, err := r.copyFamily(filepath.Join(r.admission.Root, "inputs/original"), filepath.Join(r.admission.Root, "trial"))
	r.require(true, "independent writer family", err)
	writer := r.start("writer")
	target := uint8(0)
	switch r.admission.Case {
	case "before-root-rename":
		target = 1
	case "after-root-rename-before-dirsync":
		target = 2
	case "after-root-dirsync-before-adoption":
		target = 3
	}
	for point := uint8(1); point <= 3; point++ {
		frame, raw := r.receive(writer)
		r.require(frame.Kind == "publication" && frame.Point == point && frame.PayloadHash == [32]byte{} && manualRekeyObservationValid(frame.Observation, seed), "ordered exact publication observer", nil)
		if point == 1 {
			r.observation = frame.Observation
		} else {
			r.require(frame.Observation == r.observation, "one unchanged candidate", nil)
		}
		if target == point {
			r.grant(writer, frame, raw, "exit")
			break
		}
		r.grant(writer, frame, raw, "continue")
	}
	if target == 0 {
		frame, raw := r.receive(writer)
		r.require(frame.Kind == "public-nil" && frame.Point == 0 && frame.Observation == r.observation && frame.PayloadHash == [32]byte{}, "unchanged actual nil return", nil)
		switch r.admission.Case {
		case "control":
			r.grant(writer, frame, raw, "continue")
		case "rekey-returned-no-ack":
			_, err := os.Lstat(filepath.Join(r.admission.Root, "control/parent-ack.frame"))
			r.require(errors.Is(err, os.ErrNotExist), "no external ACK exists", nil)
			r.grant(writer, frame, raw, "exit")
		case "acknowledged-exit":
			r.acknowledge(writer, frame, raw)
		default:
			r.require(false, "closed return case", nil)
		}
	}
	if r.admission.Case == "control" {
		result := r.receiveRole(writer)
		r.require(result.Outcome == "CONTROL_CLOSED" && result.OpenCalls == 1 && result.CloseCalls == 1, "control natural lifecycle", nil)
	} else {
		r.waitChild(writer, encstore.ManualRekeyExitCode)
	}
	// No original/trial Sync, Open, repair, recovery, rename or promotion here.
	// All downstream work requires the owning parent's actual writer Wait.
	r.require(writer.waited && !writer.live && !writer.unknown, "writer retired before freeze", nil)
	trialBefore, err := r.family(filepath.Join(r.admission.Root, "trial"))
	r.require(true, "post-Wait trial inventory", err)
	r.postExit, err = r.copyFamily(filepath.Join(r.admission.Root, "trial"), filepath.Join(r.admission.Root, "evidence/post-exit"))
	r.require(true, "frozen post-exit copy", err)
	oldCopy, err := r.copyFamily(filepath.Join(r.admission.Root, "evidence/post-exit"), filepath.Join(r.admission.Root, "probe-old"))
	r.require(true, "independent OLD probe copy", err)
	newCopy, err := r.copyFamily(filepath.Join(r.admission.Root, "evidence/post-exit"), filepath.Join(r.admission.Root, "probe-new"))
	r.require(true, "independent NEW probe copy", err)
	for _, a := range oldCopy {
		for _, b := range newCopy {
			r.require(a.Device != b.Device || a.Inode != b.Inode, "probe families have distinct inodes", nil)
		}
	}
	selected, err := json.Marshal(r.observation)
	r.require(true, "selected receipt", err)
	_, err = r.writeExclusive(filepath.Join(r.admission.Root, "control/selected.json"), selected, true)
	r.require(true, "selected receipt retained", err)
	oldResult := r.receiveRole(r.start("old-probe"))
	newResult := r.receiveRole(r.start("new-probe"))
	good, bad := newResult, oldResult
	if r.admission.Case == "before-root-rename" {
		good, bad = oldResult, newResult
	}
	r.require(good.Outcome == "AUTHENTICATED_LEDGER" && good.Authenticated && good.OpenCalls == 1 && good.CloseCalls == 1 && !good.NilDB && manualRekeyValidateSQL(good.SQL) && bad.Outcome == "EXPECTED_AUTHENTICATION" && bad.NilDB && bad.OpenCalls == 1 && bad.CloseCalls == 0 && bad.Code == 26 && !bad.Authenticated, "independent OLD/NEW classification", nil)
	expectedHeader, expectedHash := r.observation.Candidate, r.observation.CandidateHash
	if r.admission.Case == "before-root-rename" {
		expectedHeader, expectedHash = seed.Header, seed.Hash
	}
	r.require(good.Header == expectedHeader && good.RootHash == expectedHash && good.Info.StorageLayout == 1 && good.Info.Mode == sqlite.EncryptionSnapshot && good.Info.PageSize == 4096 && good.Info.PageCount == 3 && good.Info.DatabaseID == expectedHeader.Epoch.DatabaseID && good.Info.CommitSequence == expectedHeader.Descriptor.CommitSequence && good.Info.HeadCommitID == expectedHash, "parent exact selected generation", nil)
	var archive manualRekeyRoleResult
	if r.admission.ArchiveControl {
		r.require(true, "absent independent restore family", r.mkdir(filepath.Join(r.admission.Root, "restore-new")))
		archive = r.receiveRole(r.start("archive-check"))
		r.require(archive.Outcome == "ARCHIVE_RESTORED_NEW_TARGET" && archive.Authenticated && manualRekeyValidateSQL(archive.SQL), "archive companion logical ledger", nil)
	}
	if r.admission.ArchiveControl {
		archiveBytes, archiveStat, err := r.read(filepath.Join(r.admission.Root, "inputs/old-archive.gse"), manualRekeyLimit)
		r.require(true, "retained archive final readback", err)
		stat := archiveStat.Sys().(*syscall.Stat_t)
		r.require(seed.ArchiveArtifact == (manualRekeyArtifact{"old-archive.gse", uint64(stat.Dev), stat.Ino, archiveStat.Size(), sha256.Sum256(archiveBytes)}), "archive full manifest unchanged", nil)
	}
	nonceLedger := r.nonceLedger(seed, r.observation)
	originalAfter, err := r.family(filepath.Join(r.admission.Root, "inputs/original"))
	r.require(err == nil && reflect.DeepEqual(seed.Family, originalAfter), "immutable original family", err)
	trialAfter, err := r.family(filepath.Join(r.admission.Root, "trial"))
	r.require(err == nil && reflect.DeepEqual(trialBefore, trialAfter), "immutable exited trial family", err)
	frozenAfter, err := r.family(filepath.Join(r.admission.Root, "evidence/post-exit"))
	r.require(err == nil && reflect.DeepEqual(r.postExit, frozenAfter), "immutable frozen post-exit family", err)
	oldAfter, err := r.family(filepath.Join(r.admission.Root, "probe-old"))
	r.require(err == nil && reflect.DeepEqual(oldCopy, oldAfter), "OLD probe bytes/identity preserved", err)
	newAfter, err := r.family(filepath.Join(r.admission.Root, "probe-new"))
	r.require(err == nil && reflect.DeepEqual(newCopy, newAfter), "NEW probe bytes/identity preserved", err)
	if r.admission.Case != "acknowledged-exit" {
		_, err := os.Lstat(filepath.Join(r.admission.Root, "control/parent-ack.frame"))
		r.require(errors.Is(err, os.ErrNotExist), "ACK remains absent", nil)
	}
	receipt := struct {
		Status, Case, Scope string
		PhysicalValidated   bool
		Seed                manualRekeySeedRecord
		Observation         encstore.ManualRekeyObservation
		OLD, NEW, Archive   manualRekeyRoleResult
		Trial, PostExit     manualRekeyFamily
		Nonces              []manualRekeyNonce
	}{Status: "OBSERVATIONS_RECORDED", Case: r.admission.Case, Scope: "same-kernel-controlled-process-exit; no-power-loss-claim", PhysicalValidated: false, Seed: seed, Observation: r.observation, OLD: oldResult, NEW: newResult, Archive: archive, Trial: trialBefore, PostExit: r.postExit, Nonces: nonceLedger}
	raw, err := json.Marshal(receipt)
	r.require(true, "bounded final receipt", err)
	_, err = r.writeExclusive(filepath.Join(r.admission.Root, "control/observations.json"), raw, true)
	r.require(true, "retained final observations", err)
}
