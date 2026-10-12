//go:build linux && manual_checkpoint_restart

package sqlite_test

// SOURCE_ONLY / REVIEW_HOLD. All process exit, fixture, checkpoint and reopen
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
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/checkpointrestart"
	"github.com/ms-xiaosnow/gosqlite/internal/wal"
	"github.com/ms-xiaosnow/gosqlite/internal/walindex"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

const manualCheckpointWaitLimit = 30 * time.Second
const manualCheckpointLimit = 65536
const manualCheckpointFileLimit = 32 << 20

type manualCheckpointAdmission struct {
	Schema, Trial, Case, Root, Filesystem, Declaration string
	SourceRoot, SourceManifestPath, SourceManifest     string
	CompilerPath, Compiler, BinaryPath, Binary         string
	CompileReceiptPath, CompileReceipt                 string
}
type manualCheckpointArtifact struct {
	Name          string
	Device, Inode uint64
	Bytes         int64
	Hash          [32]byte
}
type manualCheckpointFamily []manualCheckpointArtifact
type manualCheckpointOwner struct {
	ID       string
	Number   uint64
	File     *os.File
	DB       *sqlite.DB
	Rows     *sqlite.Rows
	Tx       *sqlite.Tx
	Parent   string
	Attempts int
	State    string
}
type manualCheckpointOwnerResult struct {
	ID, Parent, State string
	Attempts          int
}
type manualCheckpointProviderResult struct {
	Purpose, Path      string
	Operation          encryption.Operation
	Acquires, Releases int
	Duplicate          bool
}
type manualCheckpointRoleResult struct {
	Role, Generation, Outcome string
	OpenCalls, CloseCalls     int
	NilDB, Authenticated      bool
	Header                    encryption.Header
	RootHash                  [32]byte
	Info                      sqlite.EncryptionInfo
	SQL                       checkpointrestart.Ledger
	Providers                 []manualCheckpointProviderResult
	Owners                    []manualCheckpointOwnerResult
	Registry                  [3]int
	PhysicalValidated         bool
}
type manualCheckpointRun struct {
	t                                 *testing.T
	admission                         manualCheckpointAdmission
	bindingPath, bindingHash, profile string
	role, generation, challenge       string
	owners                            []*manualCheckpointOwner
	causes                            []error
	providers                         []*manualCheckpointProvider
	channel                           *checkpointrestart.ManualChannel
	children                          []*manualCheckpointChild
	result                            manualCheckpointRoleResult
	observation                       checkpointrestart.ManualObservation
	modelObservations                 []checkpointrestart.Observation
	baseline, postExit                manualCheckpointFamily
	selfInfo                          os.FileInfo
	ownerPending                      bool
}

var manualCheckpointRetained []*manualCheckpointRun

func manualCheckpointHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func manualCheckpointDecode(raw []byte, value any) error {
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
func (r *manualCheckpointRun) capture(file *os.File, db *sqlite.DB, rows *sqlite.Rows, parent string) *manualCheckpointOwner {
	o := &manualCheckpointOwner{ID: fmt.Sprintf("%s/%s/%d", r.role, r.generation, len(r.owners)+1), Number: manualCheckpointOwnerNumber(r.role, len(r.owners)+1), File: file, DB: db, Rows: rows, Parent: parent, State: "live"}
	if file == nil && db == nil && rows == nil {
		o.State = "absent"
	}
	r.owners = append(r.owners, o)
	if len(r.owners) > 16384 {
		r.require(false, "owner capacity", nil)
	}
	return o
}
func (r *manualCheckpointRun) pending(err error) {
	if err != nil {
		r.causes = append(r.causes, err)
	}
	pending := append(append(sqlite.PendingPlaintextCleanups(), sqlite.PendingCloseCleanups()...), sqlite.PendingEncryptedCleanups()...)
	r.causes = append(r.causes, pending...)
	if len(pending) != 0 || errors.Is(err, sqlite.ErrEncryptedCleanupPending) {
		r.ownerPending = true
	}
}
func (r *manualCheckpointRun) close(o *manualCheckpointOwner) error {
	if o == nil || o.State == "absent" {
		return nil
	}
	if o.Attempts != 0 || o.State != "live" {
		return errors.New("owner close repeated or unknown")
	}
	r.pending(nil)
	if (o.DB != nil || o.Rows != nil || o.Tx != nil) && r.ownerPending {
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
	} else if o.Tx != nil {
		err = o.Tx.Rollback()
	}
	r.pending(err)
	o.State = "closed"
	if err != nil {
		o.State = "unknown"
		r.ownerPending = true
	}
	return err
}
func (r *manualCheckpointRun) require(ok bool, label string, err error) {
	r.pending(err)
	if !ok || err != nil || r.ownerPending {
		r.t.Fatal(label + "; original causes and owners retained")
	}
}
func (r *manualCheckpointRun) finish() {
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
				manualCheckpointHoldUnknown()
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
func manualCheckpointHoldUnknown() {
	for {
		time.Sleep(time.Hour)
	}
}
func manualCheckpointRegular(info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() {
		return false
	}
	s, ok := info.Sys().(*syscall.Stat_t)
	return ok && s.Nlink == 1 && (s.Uid == uint32(os.Geteuid()) || s.Uid == 0 && info.Mode().Perm()&0022 == 0) && s.Dev != 0 && s.Ino != 0
}
func manualCheckpointAncestors(path string) error {
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
			return errors.Join(err, errors.New("unverified ancestor"))
		}
	}
	return nil
}
func (r *manualCheckpointRun) read(path string, limit int64) ([]byte, os.FileInfo, error) {
	if err := manualCheckpointAncestors(path); err != nil {
		return nil, nil, err
	}
	before, err := os.Lstat(path)
	if err != nil || !manualCheckpointRegular(before) || before.Size() < 0 || before.Size() > limit {
		return nil, nil, errors.Join(err, errors.New("bounded single-link file required"))
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
		return nil, nil, errors.Join(err, errors.New("read identity changed"))
	}
	return raw, before, nil
}
func (r *manualCheckpointRun) syncDirectory(path string) error {
	if err := manualCheckpointAncestors(filepath.Join(path, "sentinel")); err != nil {
		return err
	}
	file, err := os.Open(path)
	owner := r.capture(file, nil, nil, "")
	if err != nil {
		return errors.Join(err, r.close(owner))
	}
	info, statErr := file.Stat()
	if statErr != nil || !info.IsDir() {
		return errors.Join(statErr, errors.New("directory owner required"), r.close(owner))
	}
	syncErr := file.Sync()
	return errors.Join(syncErr, r.close(owner))
}
func (r *manualCheckpointRun) writeExclusive(path string, raw []byte, syncDirectory bool) (manualCheckpointArtifact, error) {
	var artifact manualCheckpointArtifact
	if len(raw) > manualCheckpointFileLimit {
		return artifact, errors.New("artifact bound")
	}
	if err := manualCheckpointAncestors(path); err != nil {
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
		return artifact, errors.Join(err, errors.New("exclusive write readback mismatch"))
	}
	s := info.Sys().(*syscall.Stat_t)
	return manualCheckpointArtifact{Name: filepath.Base(path), Device: uint64(s.Dev), Inode: s.Ino, Bytes: info.Size(), Hash: sha256.Sum256(got)}, nil
}
func (r *manualCheckpointRun) mkdir(path string) error {
	if err := manualCheckpointAncestors(path); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return errors.Join(err, errors.New("private retained namespace required"))
	}
	return r.syncDirectory(filepath.Dir(path))
}
func manualCheckpointOwnerNumber(role string, n int) uint64 {
	base := uint64(100000)
	if role == "writer" {
		base = 200000
	} else if role == "probe" {
		base = 300000
	}
	return base + uint64(n)
}
func manualCheckpointRegistry() [3]int {
	return [3]int{len(sqlite.PendingPlaintextCleanups()), len(sqlite.PendingCloseCleanups()), len(sqlite.PendingEncryptedCleanups())}
}
func manualCheckpointGeneration(role string) string {
	switch role {
	case "controller":
		return "1"
	case "writer":
		return "2"
	case "probe":
		return "3"
	}
	return ""
}
func manualCheckpointRoleRoot(role string) string {
	switch role {
	case "controller":
		return "TestManualCheckpointRestartController"
	case "writer":
		return "TestManualCheckpointRestartWriter"
	case "probe":
		return "TestManualCheckpointRestartProbe"
	}
	return ""
}
func manualCheckpointFlags(role string) []string {
	return []string{"-test.run=^" + manualCheckpointRoleRoot(role) + "$", "-test.count=1", "-test.parallel=1", "-test.shuffle=off", "-test.timeout=0"}
}
func (r *manualCheckpointRun) encrypted() bool {
	return r.profile == "linux-encrypted-layout2-rawkey256-4096"
}
func (r *manualCheckpointRun) shared() bool { return r.profile == "linux-plain-shared-4096" }
func (r *manualCheckpointRun) rootName() string {
	if r.encrypted() {
		return "fixture.gse"
	}
	return "fixture.db"
}
func (r *manualCheckpointRun) path(family string) string {
	return filepath.Join(r.admission.Root, family, r.rootName())
}
func manualCheckpointAdmit(t *testing.T, role string) *manualCheckpointRun {
	if os.Getenv("GOSQLITE_MANUAL_CHECKPOINT_RESTART") != "independently_authorized" {
		t.Skip("manual checkpoint source only; not authorized")
	}
	r := &manualCheckpointRun{t: t, role: role, generation: manualCheckpointGeneration(role), result: manualCheckpointRoleResult{Role: role, Generation: manualCheckpointGeneration(role), PhysicalValidated: false}}
	manualCheckpointRetained = append(manualCheckpointRetained, r)
	r.require(runtime.GOOS == "linux" && os.Geteuid() > 0 && os.Getenv("GOFLAGS") == "" && os.Getenv("GOSQLITE_CHECKPOINT_ROLE") == role && r.generation != "", "fixed unprivileged Linux role", nil)
	r.require(reflect.DeepEqual(os.Args[1:], manualCheckpointFlags(role)), "exact role root and flags", nil)
	r.bindingPath, r.bindingHash = os.Getenv("GOSQLITE_CHECKPOINT_BINDING"), os.Getenv("GOSQLITE_CHECKPOINT_BINDING_SHA256")
	raw, _, err := r.read(r.bindingPath, manualCheckpointLimit)
	r.require(err == nil && manualCheckpointHash(raw) == r.bindingHash, "admitted binding", err)
	r.require(true, "binding schema", manualCheckpointDecode(raw, &r.admission))
	a := r.admission
	profile, _, _, _, ok := checkpointrestart.ManualCase(a.Case)
	r.profile = profile
	r.require(ok && a.Schema == "checkpoint-restart-binding-v1" && a.Trial != "" && len(a.Trial) <= 64 && !strings.ContainsAny(a.Trial, " /\\\t\r\n") && a.Declaration == "linux-process-exit-no-powerloss-retain-all", "closed ten-case admission", nil)
	r.require(true, "new root path", manualCheckpointAncestors(a.Root))
	parent, err := os.Lstat(filepath.Dir(a.Root))
	r.require(err == nil && parent.IsDir() && parent.Mode().Perm() == 0700 && parent.Sys().(*syscall.Stat_t).Uid == uint32(os.Geteuid()), "private root parent", err)
	var filesystem syscall.Statfs_t
	r.require(true, "filesystem observation", syscall.Statfs(filepath.Dir(a.Root), &filesystem))
	r.require(a.Filesystem == fmt.Sprintf("linux-fstype-%x", uint64(filesystem.Type)), "exact filesystem identity", nil)
	r.require(true, "source-to-binary binding", r.checkSourceImage())
	r.require(manualCheckpointRegistry() == [3]int{}, "empty pre-role registries", nil)
	if role != "controller" {
		r.challenge = os.Getenv("GOSQLITE_CHECKPOINT_CHALLENGE")
		challenge, e := hex.DecodeString(r.challenge)
		r.require(e == nil && len(challenge) == 32, "parent challenge", e)
		r.channel, err = checkpointrestart.ManualChildChannel(r.binding())
		r.require(true, "fixed child transport", err)
		grant, e := r.channel.Exchange("role-ready", 0, checkpointrestart.ManualObservation{}, [32]byte{})
		r.require(e == nil && grant == "continue", "parent role-start grant", e)
	}
	return r
}
func (r *manualCheckpointRun) binding() checkpointrestart.ManualBinding {
	a := r.admission
	return checkpointrestart.ManualBinding{Schema: "checkpoint-restart-frame-v1", Trial: a.Trial, Source: a.SourceManifest, Compiler: a.Compiler, CompileReceipt: a.CompileReceipt, Binary: a.Binary, Fixture: checkpointrestart.FixtureID, Profile: r.profile, Case: a.Case, Role: r.role, Generation: r.generation, Challenge: r.challenge}
}
func (r *manualCheckpointRun) checkSourceImage() error {
	a := r.admission
	manifest, _, err := r.read(a.SourceManifestPath, 4<<20)
	if err != nil || manualCheckpointHash(manifest) != a.SourceManifest {
		return errors.New("source manifest mismatch")
	}
	var inventory struct {
		Files map[string]struct {
			Bytes  int64
			SHA256 string
		}
	}
	if err := manualCheckpointDecode(manifest, &inventory); err != nil {
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
		if e != nil || info.Size() != item.Bytes || manualCheckpointHash(data) != item.SHA256 {
			return errors.New("source bytes changed")
		}
		seen[rel] = true
		return nil
	})
	if err != nil || len(seen) != len(inventory.Files) {
		return errors.New("incomplete source closure")
	}
	compiled, _, err := r.read(a.CompileReceiptPath, manualCheckpointLimit)
	if err != nil || manualCheckpointHash(compiled) != a.CompileReceipt {
		return errors.New("compile receipt mismatch")
	}
	var receipt struct {
		Schema, SourceManifest, Compiler, Binary, Package, Tags, GoVersion string
		CompileOnly                                                        bool
		Flags                                                              []string
	}
	if err := manualCheckpointDecode(compiled, &receipt); err != nil {
		return err
	}
	if receipt.Schema != "checkpoint-restart-compile-v1" || receipt.SourceManifest != a.SourceManifest || receipt.Compiler != a.Compiler || receipt.Binary != a.Binary || receipt.Package != "./sqlite" || receipt.Tags != "manual_checkpoint_restart" || receipt.GoVersion != runtime.Version() || !receipt.CompileOnly || !reflect.DeepEqual(receipt.Flags, []string{"test", "-c", "-tags=manual_checkpoint_restart", "-o", a.BinaryPath, "./sqlite"}) {
		return errors.New("compile-only source binding refused")
	}
	tool, _, err := r.read(a.CompilerPath, 128<<20)
	if err != nil || manualCheckpointHash(tool) != a.Compiler {
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
	if errors.Join(statErr, readErr, afterErr, closeErr) != nil || before.Size() <= 0 || before.Size() > 128<<20 || !os.SameFile(before, after) || before.Size() != int64(len(raw)) || before.Size() != after.Size() || manualCheckpointHash(raw) != a.Binary {
		return errors.New("running self image mismatch")
	}
	pinned, info, err := r.read(a.BinaryPath, 128<<20)
	if err != nil || !os.SameFile(before, info) || manualCheckpointHash(pinned) != a.Binary {
		return errors.New("spawn target not self image")
	}
	r.selfInfo = before
	return nil
}

// Disposable synthetic credential. Purpose and target are fixed before Open.
type manualCheckpointProvider struct {
	mu                 sync.Mutex
	purpose, path      string
	operation          encryption.Operation
	acquires, releases int
	duplicate          bool
}
type manualCheckpointMaterial struct {
	provider *manualCheckpointProvider
	key      []byte
	released bool
}

func (r *manualCheckpointRun) provider(purpose string) *manualCheckpointProvider {
	p := &manualCheckpointProvider{purpose: purpose}
	switch purpose {
	case "seed-create":
		p.path, p.operation = r.path("original"), encryption.OperationCreate
	case "writer-open":
		p.path, p.operation = r.path("trial"), encryption.OperationOpen
	case "probe-open":
		p.path, p.operation = r.path("probe"), encryption.OperationOpen
	default:
		r.require(false, "closed credential purpose", nil)
	}
	r.providers = append(r.providers, p)
	return p
}
func (p *manualCheckpointProvider) Acquire(ctx context.Context, request encryption.KeyRequest) (encryption.KeyMaterial, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ctx.Err() != nil || p.acquires != 0 || request.Path != p.path || request.Operation != p.operation || request.Kind != encryption.RawKey256 {
		p.duplicate = true
		return nil, encryption.ErrKeyProvider
	}
	p.acquires++
	key := sha256.Sum256([]byte("public-disposable-checkpoint-restart-SAME-key-v1"))
	return &manualCheckpointMaterial{provider: p, key: append([]byte(nil), key[:]...)}, nil
}
func (m *manualCheckpointMaterial) Kind() encryption.CredentialKind { return encryption.RawKey256 }
func (m *manualCheckpointMaterial) Bytes() []byte                   { return m.key }
func (m *manualCheckpointMaterial) Release() {
	m.provider.mu.Lock()
	defer m.provider.mu.Unlock()
	if m.released {
		m.provider.duplicate = true
		return
	}
	m.released = true
	clear(m.key)
	m.provider.releases++
}
func (r *manualCheckpointRun) providerResults() []manualCheckpointProviderResult {
	var results []manualCheckpointProviderResult
	for _, p := range r.providers {
		p.mu.Lock()
		result := manualCheckpointProviderResult{p.purpose, p.path, p.operation, p.acquires, p.releases, p.duplicate}
		p.mu.Unlock()
		r.require(result.Acquires == 1 && result.Releases == 1 && !result.Duplicate, "same-key single borrow release", nil)
		results = append(results, result)
	}
	return results
}
func (r *manualCheckpointRun) options(purpose string) sqlite.Options {
	o := sqlite.Options{PageSize: 4096, KeepWALOnClose: true, MaxTransactionPages: 4096, MaxTransactionUndoBytes: 64 << 20, MaxSortBytes: 64 << 20, NoCreate: purpose != "seed-create"}
	if r.encrypted() {
		o.Encryption = &sqlite.EncryptionOptions{KeyProvider: r.provider(purpose), CredentialKind: encryption.RawKey256, Mode: sqlite.EncryptionWAL, MaxGenerations: 16, MaxRetainedBytes: 64 << 20, MaxDirectoryEntries: 64, WALLimits: sqlite.EncryptionWALLimits{MaxSegments: 4, MaxFrames: 4096, MaxSegmentFrames: 4096, MaxSegmentBytes: 32 << 20, MaxBytes: 64 << 20, MaxRootBytes: 64 << 10, MaxOverlayPages: 4096}}
		o.ReadOnly = purpose == "probe-open"
	} else {
		o.SectorSize = 4096
		o.WALShared = r.shared()
		o.WALExclusive = !r.shared()
	}
	return o
}

type manualCheckpointDiagnostic struct {
	mu       sync.Mutex
	data     []byte
	overflow bool
}

func (d *manualCheckpointDiagnostic) Write(raw []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.data)+len(raw) > manualCheckpointLimit {
		d.overflow = true
		return 0, io.ErrShortWrite
	}
	d.data = append(d.data, raw...)
	return len(raw), nil
}
func (d *manualCheckpointDiagnostic) snapshot() ([]byte, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]byte(nil), d.data...), d.overflow
}

type manualCheckpointWait struct {
	State *os.ProcessState
	Err   error
}
type manualCheckpointChild struct {
	command                                                       *exec.Cmd
	binding                                                       checkpointrestart.ManualBinding
	control, events                                               *os.File
	controlOwner, eventOwner                                      *manualCheckpointOwner
	stdout, stderr                                                manualCheckpointDiagnostic
	wait                                                          chan manualCheckpointWait
	actual                                                        manualCheckpointWait
	started, waited, live, unknown, receiptAttempted, terminalEOF bool
	sequence                                                      uint8
	frames                                                        [][]byte
	last                                                          checkpointrestart.ManualFrame
}
type manualCheckpointProcessReceipt struct {
	Role, Generation, Challenge, Outcome                  string
	Starts, Waits                                         int
	ExitCode                                              int
	Exited, Signaled, ChildLive, WaitUnknown, TerminalEOF bool
	Frames                                                [][32]byte
	Stdout, Stderr                                        [32]byte
	PhysicalValidated                                     bool
}

func (r *manualCheckpointRun) start(role string) *manualCheckpointChild {
	r.require(r.role == "controller" && len(r.children) < 2 && role != "controller" && manualCheckpointRoleRoot(role) != "", "parent-only closed role spawn", nil)
	for _, previous := range r.children {
		r.require(previous.binding.Role != role && previous.waited, "no duplicate or overlapping live child", nil)
	}
	raw, info, err := r.read(r.admission.BinaryPath, 128<<20)
	r.require(err == nil && os.SameFile(info, r.selfInfo) && manualCheckpointHash(raw) == r.admission.Binary, "fixed self-image before Start", err)
	var nonce [32]byte
	_, err = rand.Read(nonce[:])
	r.require(nonce != [32]byte{}, "parent challenge", err)
	binding := r.binding()
	binding.Role, binding.Generation, binding.Challenge = role, manualCheckpointGeneration(role), hex.EncodeToString(nonce[:])
	controlRead, controlWrite, err := os.Pipe()
	cr := r.capture(controlRead, nil, nil, "")
	cw := r.capture(controlWrite, nil, nil, "")
	r.require(true, "owned control pipe", err)
	eventRead, eventWrite, err := os.Pipe()
	er := r.capture(eventRead, nil, nil, "")
	ew := r.capture(eventWrite, nil, nil, "")
	r.require(true, "owned event pipe", err)
	child := &manualCheckpointChild{binding: binding, control: controlWrite, events: eventRead, controlOwner: cw, eventOwner: er, wait: make(chan manualCheckpointWait, 1)}
	command := exec.Command(r.admission.BinaryPath, manualCheckpointFlags(role)...)
	command.Env = []string{"GOFLAGS=", "GOSQLITE_MANUAL_CHECKPOINT_RESTART=independently_authorized", "GOSQLITE_CHECKPOINT_ROLE=" + role, "GOSQLITE_CHECKPOINT_BINDING=" + r.bindingPath, "GOSQLITE_CHECKPOINT_BINDING_SHA256=" + r.bindingHash, "GOSQLITE_CHECKPOINT_CHALLENGE=" + binding.Challenge}
	command.Dir = filepath.Join(r.admission.Root, "control")
	command.ExtraFiles = []*os.File{controlRead, eventWrite}
	command.Stdout, command.Stderr = &child.stdout, &child.stderr
	child.command = command
	r.children = append(r.children, child)
	err = command.Start()
	if command.Process != nil {
		child.started = true
		go func() {
			err := command.Wait()
			child.wait <- manualCheckpointWait{State: command.ProcessState, Err: err}
		}()
	}
	if err != nil {
		child.unknown = true
		r.require(false, "Start failed; concrete owner retained", err)
	}
	r.require(child.started, "successful Start owns exact process", nil)
	r.require(true, "parent duplicate control-reader Close", r.close(cr))
	r.require(true, "parent duplicate event-writer Close", r.close(ew))
	ready, frame := r.receive(child)
	r.require(ready.Kind == "role-ready" && ready.Point == 0 && ready.Observation == (checkpointrestart.ManualObservation{}) && ready.PayloadHash == [32]byte{}, "verified child ready", nil)
	r.grant(child, ready, frame, "continue")
	return child
}
func (r *manualCheckpointRun) receive(child *manualCheckpointChild) (checkpointrestart.ManualFrame, []byte) {
	type received struct {
		frame checkpointrestart.ManualFrame
		raw   []byte
		err   error
	}
	result := make(chan received, 1)
	go func() {
		frame, raw, err := checkpointrestart.ManualReadFrame(child.events)
		result <- received{frame, raw, err}
	}()
	select {
	case event := <-result:
		r.require(event.err == nil, "bounded event frame", event.err)
		child.sequence++
		r.require(event.frame.Binding == child.binding && event.frame.Sequence == child.sequence && child.sequence <= checkpointrestart.ManualFrameCount, "exact fresh causal event", nil)
		child.frames = append(child.frames, append([]byte(nil), event.raw...))
		child.last = event.frame
		return event.frame, event.raw
	case <-time.After(manualCheckpointWaitLimit):
		child.live, child.unknown = true, true
		r.saveProcess(child, "CHILD_LIVE/WAIT_UNKNOWN")
		r.require(false, "child barrier unresolved; no kill or dependent probe", nil)
	}
	return checkpointrestart.ManualFrame{}, nil
}
func (r *manualCheckpointRun) grant(child *manualCheckpointChild, event checkpointrestart.ManualFrame, raw []byte, kind string) {
	frame := event
	frame.Kind, frame.PayloadHash = kind, sha256.Sum256(raw)
	_, err := checkpointrestart.ManualWriteFrame(child.control, frame)
	r.require(true, "parent causal grant", err)
}
func (r *manualCheckpointRun) waitChild(child *manualCheckpointChild, expected int) {
	r.require(child.started && !child.waited && !child.live && !child.unknown, "one real Wait required", nil)
	select {
	case result := <-child.wait:
		child.actual = result
		if result.State == nil {
			child.unknown = true
			r.saveProcess(child, "WAIT_UNKNOWN")
			// Wait returned without an actual process state. No second Wait
			// can repair this receipt; retain the controller for disposition.
			manualCheckpointHoldUnknown()
		}
		child.waited = true
	case <-time.After(manualCheckpointWaitLimit):
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
	r.require(valid && child.terminalEOF, "exact actual exit, diagnostics and terminal protocol EOF", manualCheckpointWaitUnexpected(child.actual.Err, expected))
	r.require(true, "parent control writer retired once", r.close(child.controlOwner))
	r.require(true, "parent event reader retired once", r.close(child.eventOwner))
}
func manualCheckpointWaitUnexpected(err error, expected int) error {
	if expected == checkpointrestart.ManualExitCode {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil
		}
	}
	return err
}
func (r *manualCheckpointRun) saveProcess(child *manualCheckpointChild, outcome string) {
	if child.receiptAttempted {
		return
	}
	child.receiptAttempted = true
	receipt := manualCheckpointProcessReceipt{Role: child.binding.Role, Generation: child.binding.Generation, Challenge: child.binding.Challenge, Outcome: outcome, ExitCode: -1, ChildLive: child.live, WaitUnknown: child.unknown, TerminalEOF: child.terminalEOF, PhysicalValidated: false}
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

// Validate bounded public framing only. Retained history is not promoted or
// authenticated by this inventory helper; VerifyEncrypted authenticates the
// selected graph separately. No opaque ciphertext is decrypted here.
func manualCheckpointCiphertextExtent(raw []byte, h encryption.Header) error {
	d := h.Descriptor
	if d.PageSize != 4096 || d.PageCount == 0 || d.PageCount > 4096 || d.CommitSequence == 0 || d.CommitSequence > 16 || len(raw) < encryption.HeaderSize+encryption.RecordHeaderSize {
		return errors.New("bounded fixture ciphertext descriptor")
	}
	first, err := encryption.DecodeRecordHeader(raw[encryption.HeaderSize:encryption.HeaderSize+encryption.RecordHeaderSize], encryption.Limits{})
	if err != nil {
		return err
	}
	var metadata, footer, dataRecords uint64
	switch d.Role {
	case encryption.FileRoot:
		metadata = uint64(first.PlaintextSize)
		if d.RecordCount != 1 || metadata < 512 || metadata > 512+4*256 || (metadata-512)%256 != 0 {
			return errors.New("exact layout2 root payload extent")
		}
	case encryption.FileBase:
		if d.RecordCount != uint64(d.PageCount)+2 {
			return errors.New("base page record extent")
		}
		metadata, footer, dataRecords = 64, 48, uint64(d.PageCount)
	case encryption.FileWAL:
		if d.RecordCount < 3 || d.RecordCount > uint64(d.PageCount)+2 || d.RecordCount > 4098 {
			return errors.New("WAL frame record extent")
		}
		dataRecords = d.RecordCount - 2
		metadata, footer = 128+4*dataRecords, 208
	default:
		return errors.New("closed encrypted family role")
	}
	expected := uint64(encryption.HeaderSize) + metadata + uint64(encryption.RecordHeaderSize+encryption.TagSize)
	if d.Role != encryption.FileRoot {
		expected += dataRecords*uint64(d.PageSize+encryption.RecordHeaderSize+encryption.TagSize) + footer + uint64(encryption.RecordHeaderSize+encryption.TagSize)
	}
	if expected != uint64(len(raw)) || expected > manualCheckpointFileLimit {
		return errors.New("exact ciphertext file extent")
	}
	offset := uint64(encryption.HeaderSize)
	for ordinal := uint64(0); ordinal < d.RecordCount; ordinal++ {
		if offset+encryption.RecordHeaderSize > uint64(len(raw)) {
			return errors.New("record header outside ciphertext")
		}
		rh, err := encryption.DecodeRecordHeader(raw[offset:offset+encryption.RecordHeaderSize], encryption.Limits{})
		if err != nil {
			return err
		}
		role, size := encryption.RecordData, uint64(d.PageSize)
		if ordinal == 0 {
			role, size = encryption.RecordMetadata, metadata
		} else if ordinal == d.RecordCount-1 {
			role, size = encryption.RecordFooter, footer
		}
		if rh.Role != role || rh.Sequence != ordinal || uint64(rh.PlaintextSize) != size {
			return errors.New("exact public record role/sequence/extent")
		}
		offset += uint64(encryption.RecordHeaderSize+encryption.TagSize) + size
	}
	if offset != uint64(len(raw)) {
		return errors.New("ciphertext trailing or missing bytes")
	}
	return nil
}
func (r *manualCheckpointRun) family(directory string) (manualCheckpointFamily, error) {
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) == 0 || len(entries) > 64 {
		return nil, errors.Join(err, errors.New("bounded complete family inventory"))
	}
	var manifest manualCheckpointFamily
	var total int64
	selected := false
	rootName := r.rootName()
	var fixtureEpoch encryption.Epoch
	if r.encrypted() {
		selectedRoot, _, err := r.read(filepath.Join(directory, rootName), manualCheckpointFileLimit)
		if err != nil {
			return nil, err
		}
		if len(selectedRoot) < encryption.HeaderSize {
			return nil, errors.New("selected root public header absent")
		}
		header, err := encryption.DecodeHeader(selectedRoot[:encryption.HeaderSize], encryption.Limits{MaxPages: 4096})
		if err != nil {
			return nil, err
		}
		if header.Descriptor.Role != encryption.FileRoot || header.Epoch.Kind != encryption.RawKey256 {
			return nil, errors.New("selected fixture root role/kind")
		}
		fixtureEpoch = header.Epoch
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return nil, errors.New("family links or directories refused")
		}
		raw, info, err := r.read(filepath.Join(directory, name), manualCheckpointFileLimit)
		if err != nil {
			return nil, err
		}
		stat := info.Sys().(*syscall.Stat_t)
		if info.Mode().Perm() != 0600 || stat.Uid != uint32(os.Geteuid()) {
			return nil, errors.New("family exact owner/mode required")
		}
		total += int64(len(raw))
		if total > 64<<20 {
			return nil, errors.New("family byte bound")
		}
		if name == rootName {
			selected = true
		}
		if r.encrypted() {
			if name == rootName+".gse-lock" {
				if len(raw) != 0 {
					return nil, errors.New("lock extent")
				}
			} else {
				if len(raw) < encryption.HeaderSize || string(raw[:8]) != encryption.Magic {
					return nil, errors.New("plaintext or malformed encrypted family")
				}
				header, e := encryption.DecodeHeader(raw[:encryption.HeaderSize], encryption.Limits{MaxPages: 4096})
				if e != nil {
					return nil, e
				}
				if header.Epoch != fixtureEpoch {
					return nil, errors.New("foreign database or epoch in complete fixture family")
				}
				if e := manualCheckpointCiphertextExtent(raw, header); e != nil {
					return nil, e
				}
				if name == rootName {
					if header.Descriptor.Role != encryption.FileRoot {
						return nil, errors.New("selected root role")
					}
				} else {
					suffix := filepath.Ext(name)
					wantRole := encryption.FileRoot
					switch suffix {
					case ".base":
						wantRole = encryption.FileBase
					case ".wal":
						wantRole = encryption.FileWAL
					case ".pending", ".retained":
					default:
						return nil, errors.New("closed encrypted object suffix")
					}
					if name != rootName+".gse-"+hex.EncodeToString(header.FileID[:])+suffix || header.Descriptor.Role != wantRole {
						return nil, errors.New("public role/FileID filename binding")
					}
				}
			}
		} else {
			if name != rootName && name != rootName+"-wal" && name != rootName+"-shm" {
				return nil, errors.New("closed plaintext family")
			}
			if name == rootName && (len(raw) == 0 || len(raw) > 4096*4096 || len(raw)%4096 != 0) {
				return nil, errors.New("main extent")
			}
			if name == rootName+"-shm" && (len(raw) < walindex.FirstHeaderSize || len(raw)%walindex.BlockSize != 0) {
				return nil, errors.New("SHM extent")
			}
		}
		artifact := manualCheckpointArtifact{name, uint64(stat.Dev), stat.Ino, info.Size(), sha256.Sum256(raw)}
		for _, previous := range manifest {
			if previous.Device == artifact.Device && previous.Inode == artifact.Inode {
				return nil, errors.New("family inode alias")
			}
		}
		manifest = append(manifest, artifact)
	}
	if !selected {
		return nil, errors.New("selected authority absent")
	}
	sort.Slice(manifest, func(i, j int) bool { return manifest[i].Name < manifest[j].Name })
	return manifest, nil
}
func manualCheckpointSameBytes(a, b manualCheckpointFamily, distinct bool) bool {
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
func (r *manualCheckpointRun) copyFamily(source, destination string) (manualCheckpointFamily, error) {
	before, err := r.family(source)
	if err != nil {
		return nil, err
	}
	if err := r.mkdir(destination); err != nil {
		return nil, err
	}
	for _, artifact := range before {
		raw, _, err := r.read(filepath.Join(source, artifact.Name), manualCheckpointFileLimit)
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
		return nil, errors.Join(err, errors.New("source family mutated"))
	}
	copied, err := r.family(destination)
	if err != nil || !manualCheckpointSameBytes(before, copied, true) {
		return nil, errors.Join(err, errors.New("complete independent copy failed"))
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
func manualCheckpointFind(family manualCheckpointFamily, name string) (manualCheckpointArtifact, bool) {
	for _, a := range family {
		if a.Name == name {
			return a, true
		}
	}
	return manualCheckpointArtifact{}, false
}
func manualCheckpointDistinct(a, b manualCheckpointFamily) bool {
	for _, x := range a {
		for _, y := range b {
			if x.Device == y.Device && x.Inode == y.Inode {
				return false
			}
		}
	}
	return true
}

type manualCheckpointHead struct {
	Manual      checkpointrestart.ManualHead
	Plain       checkpointrestart.PlainHead
	Info        sqlite.EncryptionInfo
	LogicalHash [32]byte
	BaseFileID  [32]byte
}

func (r *manualCheckpointRun) baseFileID(path string, selected encryption.Header, sequence uint64) [32]byte {
	family, err := r.family(filepath.Dir(path))
	r.require(true, "bounded complete authenticated-head family", err)
	var id [32]byte
	count := 0
	for _, a := range family {
		if !strings.HasSuffix(a.Name, ".base") {
			continue
		}
		raw, _, err := r.read(filepath.Join(filepath.Dir(path), a.Name), manualCheckpointFileLimit)
		r.require(err == nil && len(raw) >= encryption.HeaderSize, "bounded base identity", err)
		h, err := encryption.DecodeHeader(raw[:encryption.HeaderSize], encryption.Limits{MaxPages: 4096})
		r.require(true, "public base header", err)
		if h.Epoch == selected.Epoch && h.Descriptor.CommitSequence == sequence {
			r.require(h.Descriptor.Role == encryption.FileBase && h.Descriptor.PageSize == 4096 && h.FileID != selected.FileID, "selected generation base identity", nil)
			id = h.FileID
			count++
		}
	}
	r.require(count == 1 && id != [32]byte{}, "one matching base in verified selected graph", nil)
	return id
}
func manualCheckpointModelEncrypted(h manualCheckpointHead) checkpointrestart.EncryptedHead {
	return checkpointrestart.EncryptedHead{Authenticated: true, RootHash: h.Manual.OldRootHash, RootBytes: 1320, Header: checkpointrestart.ManualModelHeader(h.Manual.OldRoot), Layout: uint32(h.Info.StorageLayout), HeadCommitID: h.Info.HeadCommitID, BaseFileID: h.BaseFileID, WALSegments: h.Info.WALSegments, WALFrames: h.Info.WALFrames, WALBytes: h.Info.WALBytes}
}
func (r *manualCheckpointRun) head(db *sqlite.DB, path string, digest [32]byte, requireWAL bool) manualCheckpointHead {
	head := manualCheckpointHead{Manual: checkpointrestart.ManualHead{Ledger: digest}}
	if r.encrypted() {
		info, err := db.VerifyEncrypted(context.Background())
		r.require(true, "authenticate selected head", err)
		raw, _, err := r.read(path, 64<<10)
		r.require(err == nil && len(raw) >= encryption.HeaderSize, "bounded layout2 root", err)
		h, err := encryption.DecodeHeader(raw[:encryption.HeaderSize], encryption.Limits{MaxPages: 4096})
		r.require(true, "public root identity", err)
		r.require(info.Mode == sqlite.EncryptionWAL && info.StorageLayout == 2 && info.PageSize == 4096 && info.PageCount > 0 && info.PageCount <= 4096 && info.DatabaseID == h.Epoch.DatabaseID && info.CommitSequence == h.Descriptor.CommitSequence && h.Epoch.Kind == encryption.RawKey256 && info.HeadCommitID != [32]byte{}, "authenticated layout2 geometry", nil)
		if requireWAL {
			r.require(info.CommitSequence == 4 && info.WALSegments == 1 && info.WALFrames > 0 && info.WALFrames <= 4096 && info.WALBytes > 0 && info.WALBytes <= 32<<20 && len(raw) == 1320, "exact committed sequence4/one segment", nil)
		}
		head.Info = info
		baseSequence := uint64(3)
		if info.CommitSequence == 5 {
			baseSequence = 5
		}
		head.BaseFileID = r.baseFileID(path, h, baseSequence)
		head.Manual.OldRoot, head.Manual.OldRootHash, head.Manual.AuthenticatedHeadCommitID = h, sha256.Sum256(raw), info.HeadCommitID
		return head
	}
	main, _, err := r.read(path, 4096*4096)
	r.require(true, "bounded main image", err)
	var snap wal.Snapshot
	var walRaw []byte
	_, statErr := os.Lstat(path + "-wal")
	if statErr == nil {
		walRaw, _, err = r.read(path+"-wal", manualCheckpointFileLimit)
		r.require(true, "owned immutable WAL bytes", err)
		snap, err = wal.Scan(context.Background(), bytes.NewReader(walRaw), int64(len(walRaw)), wal.Limits{MaxWALBytes: 32 << 20, MaxFrames: 4096, MaxIndexEntries: 8192, MaxDBPages: 4096, MaxIndexWork: 1 << 20})
		r.require(err == nil && snap.Complete && snap.Tail == wal.TailClean && snap.Header.PageSize == 4096 && snap.DBSize > 0 && snap.FrameCount > 0 && snap.EndOffset == int64(len(walRaw)), "complete final committed WAL without tail", err)
	} else {
		r.require(!requireWAL && errors.Is(statErr, os.ErrNotExist) && len(main)%4096 == 0, "expected WAL authority", manualCheckpointStatErrUnlessAbsent(statErr))
		snap.DBSize = uint32(len(main) / 4096)
	}
	plain := checkpointrestart.PlainHead{PageSize: 4096, DBSize: snap.DBSize, FrameCount: snap.FrameCount, EndOffset: uint64(snap.EndOffset), Salt: snap.Header.Salt, Checksum: [2]uint32(snap.Checksum), MainHash: sha256.Sum256(main), Complete: true, CleanTail: true}
	if len(walRaw) != 0 {
		plain.WALHash, plain.WALBytes = sha256.Sum256(walRaw), uint64(len(walRaw))
	}
	var pages []uint32
	for page := range snap.Pages {
		pages = append(pages, page)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i] < pages[j] })
	for _, page := range pages {
		ref := snap.Pages[page]
		r.require(ref.Offset >= 0 && ref.Offset+4096 <= int64(len(walRaw)), "selected extent", nil)
		plain.Selected = append(plain.Selected, checkpointrestart.SelectedPage{Page: page, Frame: ref.Frame, Hash: sha256.Sum256(walRaw[ref.Offset : ref.Offset+4096])})
	}
	logical := sha256.New()
	for page := uint32(1); page <= snap.DBSize; page++ {
		var raw []byte
		if ref, ok := snap.Pages[page]; ok {
			raw = walRaw[ref.Offset : ref.Offset+4096]
		} else {
			off := int64(page-1) * 4096
			r.require(off+4096 <= int64(len(main)), "complete logical page source", nil)
			raw = main[off : off+4096]
		}
		logical.Write(raw)
	}
	copy(head.LogicalHash[:], logical.Sum(nil))
	p := checkpointrestart.ManualPage{DBSize: snap.DBSize, Frames: snap.FrameCount, Salt: snap.Header.Salt, Checksum: [2]uint32(snap.Checksum)}
	if requireWAL && r.shared() {
		shm, _, err := r.read(path+"-shm", manualCheckpointFileLimit)
		r.require(err == nil && len(shm) >= walindex.FirstHeaderSize, "current SHM scalar snapshot", err)
		h, err := walindex.ParseHeader(shm[:walindex.HeaderSize])
		r.require(true, "SHM header checksum", err)
		other, err := walindex.ParseHeader(shm[walindex.HeaderSize : 2*walindex.HeaderSize])
		r.require(err == nil && h == other && h.PageSize == 4096 && h.DBSize == snap.DBSize && h.MaxFrame == snap.FrameCount && h.FrameChecksum == [2]uint32(snap.Checksum) && h.Salt == [8]byte(walRaw[16:24]), "SHM/WAL same committed generation", err)
		p.Safe, p.Backfill = h.MaxFrame, binary.NativeEndian.Uint32(shm[96:100])
		r.require(p.Backfill < p.Safe, "unbackfilled C required", nil)
	}
	for _, page := range plain.Selected {
		if page.Frame > p.Backfill {
			p.Selected++
		}
	}
	if requireWAL {
		r.require(p.Selected >= 2 && p.Selected <= 4096, "genuinely partial first copy", nil)
	}
	head.Plain, head.Manual.Page = plain, p
	return head
}
func manualCheckpointStatErrUnlessAbsent(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
func manualCheckpointValue(value any) (checkpointrestart.Value, error) {
	switch v := value.(type) {
	case nil:
		return checkpointrestart.Null(), nil
	case int64:
		return checkpointrestart.Integer(v), nil
	case float64:
		return checkpointrestart.Real(v), nil
	case string:
		if len(v) <= 64<<10 {
			return checkpointrestart.Text(v), nil
		}
	case []byte:
		if len(v) <= 64<<10 {
			return checkpointrestart.Blob(v), nil
		}
	}
	return checkpointrestart.Value{}, errors.New("bounded SQLite storage class required")
}
func (r *manualCheckpointRun) ledger(db *sqlite.DB, dbOwner *manualCheckpointOwner) checkpointrestart.Ledger {
	role := checkpointrestart.Controller
	if r.role == "writer" {
		role = checkpointrestart.Writer
	} else if r.role == "probe" {
		role = checkpointrestart.Probe
	}
	ledger := checkpointrestart.Ledger{Schema: 1, Fixture: checkpointrestart.FixtureID, Role: role, Generation: uint64(role), Owner: dbOwner.Number}
	total := 0
	for i, sql := range checkpointrestart.StreamSQL() {
		rows, err := db.Query(context.Background(), sql)
		owner := r.capture(nil, nil, rows, dbOwner.ID)
		r.require(err == nil && rows != nil, "complete stream acquisition", err)
		stream := checkpointrestart.Stream{ID: uint8(i + 1), SQL: sql, Args: []checkpointrestart.Value{}, Columns: rows.Columns(), Rows: [][]checkpointrestart.Value{}, Receipt: checkpointrestart.RowsReceipt{Owner: owner.Number}}
		for rows.Next() {
			total++
			r.require(total <= 4096 && (i != 3 || len(stream.Rows) < 64), "complete stream row bounds", nil)
			values := rows.Values()
			r.require(len(values) == len(stream.Columns), "exact columns", nil)
			row := make([]checkpointrestart.Value, len(values))
			for j, value := range values {
				row[j], err = manualCheckpointValue(value)
				r.require(true, "copy bounded typed cell before Next", err)
			}
			stream.Rows = append(stream.Rows, row)
		}
		err = rows.Err()
		stream.Receipt.CompleteEOF, stream.Receipt.ErrNil = err == nil, err == nil
		if err != nil {
			owner.State = "unknown"
			r.ownerPending = true
		}
		r.require(true, "actual complete EOF/Rows.Err", err)
		err = r.close(owner)
		stream.Receipt.CloseCalls, stream.Receipt.CloseNil = uint8(owner.Attempts), err == nil
		r.require(true, "one public Rows.Close", err)
		ledger.Streams = append(ledger.Streams, stream)
	}
	for i, id := range checkpointrestart.QueryIDs() {
		var result any
		var err error
		switch i {
		case 0, 1:
			result, err = db.SimilaritySearch(context.Background(), "docs_grams", "body", "abcd", sqlite.SimilarityOptions{Threshold: 0.6, Limit: 2, Where: "docs.active=1", ForceScan: i == 1})
		case 2:
			result, err = db.VectorSearch(context.Background(), "hitems", "embedding", "[20,6,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: "hitems_hnsw", Limit: 5})
		case 3:
			result, err = db.VectorSearch(context.Background(), "hitems", "embedding", "[20,6,1]", sqlite.VectorSearchOptions{Limit: 5})
		case 4:
			result, err = db.VectorSearch(context.Background(), "iitems", "embedding", "[0,0,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: "iitems_ivf", Limit: 5})
		case 5:
			result, err = db.VectorSearch(context.Background(), "iitems", "embedding", "[0,0,1]", sqlite.VectorSearchOptions{Limit: 5})
		}
		r.require(true, "real complete search API", err)
		raw, err := json.Marshal(result)
		r.require(err == nil && len(raw) <= 64<<10, "complete query plan/stats/hits artifact", err)
		ledger.Queries = append(ledger.Queries, checkpointrestart.QueryEvidence{ID: id, Result: raw, Executed: true})
	}
	for _, name := range checkpointrestart.IndexNames() {
		err := db.CheckExtensionIndex(context.Background(), name)
		r.require(true, "complete per-index integrity", err)
		ledger.Checks = append(ledger.Checks, checkpointrestart.IndexCheck{Name: name, Calls: 1, ReturnedNil: true})
	}
	checked, err := db.Check(context.Background(), sqlite.CheckOptions{MaxPages: 4096, MaxPageReads: 1 << 20})
	r.require(true, "bounded full DB.Check", err)
	ledger.Check, ledger.CheckCalls, ledger.CheckReturnedNil, ledger.SchemaCookie = checkpointrestart.CheckResult(checked), 1, true, db.Info().SchemaCookie
	r.require(checkpointrestart.ValidateLedger(ledger) == checkpointrestart.NoProblem, "independent complete C ledger", nil)
	return ledger
}

type manualCheckpointCommitReceipt struct {
	BindingHash, Source, Binary, Fixture, Case, Profile string
	CommitReturnedNil                                   bool
	LedgerDigest                                        [32]byte
	Head                                                manualCheckpointHead
}
type manualCheckpointSeedRecord struct {
	BindingHash    string
	SQL            checkpointrestart.Ledger
	Digest         [32]byte
	Head           manualCheckpointHead
	Commit         manualCheckpointCommitReceipt
	CommitArtifact manualCheckpointArtifact
	Family         manualCheckpointFamily
}
type manualCheckpointReady struct {
	BindingHash  string
	Head         manualCheckpointHead
	Family       manualCheckpointFamily
	LedgerDigest [32]byte
	SQL          checkpointrestart.Ledger
	Providers    []manualCheckpointProviderResult
}

func (r *manualCheckpointRun) begin(db *sqlite.DB, owner *manualCheckpointOwner) (*sqlite.Tx, *manualCheckpointOwner) {
	tx, err := db.Begin(context.Background())
	txOwner := r.capture(nil, nil, nil, owner.ID)
	txOwner.Tx = tx
	if tx != nil {
		txOwner.State = "live"
	}
	r.require(err == nil && tx != nil, "one owned data transaction", err)
	return tx, txOwner
}
func (r *manualCheckpointRun) markCommit(owner *manualCheckpointOwner, err error) {
	r.require(owner.Tx != nil && owner.State == "live" && owner.Attempts == 0, "one original Commit", nil)
	owner.Attempts++
	owner.State = "closed"
	if err != nil {
		owner.State = "unknown"
		r.ownerPending = true
	}
	r.require(true, "captured original Commit result", err)
}
func (r *manualCheckpointRun) insertFixture(tx *sqlite.Tx, final bool) {
	for _, row := range []struct {
		id     int64
		body   any
		active int64
	}{{-9, "abcd", 0}, {1, "abce", 1}, {2, "abcf", 1}, {3, nil, 1}, {4, "disjoint", 1}, {5, "", 1}, {6, "a", 1}} {
		if (row.id == 2) != final {
			continue
		}
		_, err := tx.Exec(context.Background(), "INSERT INTO docs VALUES(?,?,?)", row.id, row.body, row.active)
		r.require(true, "fixed docs row", err)
	}
	for i := 1; i <= 180; i++ {
		if (i == 180) != final {
			continue
		}
		active := int64(0)
		if i%19 == 0 {
			active = 1
		}
		_, err := tx.Exec(context.Background(), "INSERT INTO hitems VALUES(?,?,?,?)", int64(i), fmt.Sprintf("[%d,%d,1]", i, i%7), active, fmt.Sprintf("item %d", i))
		r.require(true, "fixed HNSW source row", err)
	}
	for cluster := 0; cluster < 4; cluster++ {
		for offset := 0; offset < 32; offset++ {
			id := cluster*32 + offset + 1
			if (id == 128) != final {
				continue
			}
			active := int64(0)
			if cluster == 3 {
				active = 1
			}
			value := fmt.Sprintf("[%f,%f,1]", float64(cluster*1000)+float64(offset)/100, float64(cluster*500)+float64(offset)/200)
			_, err := tx.Exec(context.Background(), "INSERT INTO iitems VALUES(?,?,?,?)", int64(id), value, active, fmt.Sprintf("item %d", id))
			r.require(true, "fixed IVF source row", err)
		}
	}
}
func (r *manualCheckpointRun) seed() manualCheckpointSeedRecord {
	for _, name := range []string{"original", "control", "evidence"} {
		r.require(true, "new private seed namespace", r.mkdir(filepath.Join(r.admission.Root, name)))
	}
	db, err := sqlite.OpenContext(context.Background(), r.path("original"), r.options("seed-create"))
	owner := r.capture(nil, db, nil, "")
	r.require(err == nil && db != nil, "one seed create", err)
	baselineTx, baselineOwner := r.begin(db, owner)
	for _, sql := range checkpointrestart.CreateStatements() {
		_, err := baselineTx.Exec(context.Background(), sql)
		r.require(true, "literal source schema", err)
	}
	r.insertFixture(baselineTx, false)
	baselineCommitErr := baselineTx.Commit(context.Background())
	capturedBaselineCommitErr := baselineCommitErr
	r.markCommit(baselineOwner, capturedBaselineCommitErr)
	// B is exactly 312 source rows and no extension; this context is unarmed.
	preparatoryErr := db.Checkpoint(context.Background())
	r.require(true, "one preparatory B checkpoint", preparatoryErr)
	tx, txOwner := r.begin(db, owner)
	r.insertFixture(tx, true)
	r.require(true, "grams creation inside C transaction", tx.CreateGramsIndex(context.Background(), "docs_grams", "docs", []string{"body"}))
	r.require(true, "HNSW creation inside C transaction", tx.CreateHNSWIndex(context.Background(), "hitems_hnsw", "hitems", "embedding"))
	r.require(true, "IVF creation inside C transaction", tx.CreateIVFFlatIndex(context.Background(), "iitems_ivf", "iitems", "embedding", sqlite.IVFFlatIndexOptions{Lists: 4}))
	commitErr := tx.Commit(context.Background())
	capturedCommitErr := commitErr
	commitReturnedNil := capturedCommitErr == nil
	r.markCommit(txOwner, capturedCommitErr)
	ledger := r.ledger(db, owner)
	digest, problem := checkpointrestart.LedgerDigest(ledger)
	r.require(problem == checkpointrestart.NoProblem, "complete retained ledger digest", nil)
	head := r.head(db, r.path("original"), digest, true)
	commit := manualCheckpointCommitReceipt{BindingHash: r.bindingHash, Source: r.admission.SourceManifest, Binary: r.admission.Binary, Fixture: checkpointrestart.FixtureID, Case: r.admission.Case, Profile: r.profile, CommitReturnedNil: commitReturnedNil, LedgerDigest: digest, Head: head}
	raw, err := json.Marshal(commit)
	r.require(true, "committed-head receipt encoding", err)
	artifact, err := r.writeExclusive(filepath.Join(r.admission.Root, "control/committed-head.json"), raw, true)
	r.require(commitReturnedNil, "receipt originates in exact public Commit nil", err)
	// The exclusive receipt's write, fsync, Close, directory fsync/Close and
	// identity/hash readback all precede the seed Close and writer Start.
	r.require(true, "seed DB.Close once with KeepWALOnClose", r.close(owner))
	r.require(manualCheckpointRegistry() == [3]int{}, "seed registries empty", nil)
	r.providerResults()
	family, err := r.family(filepath.Join(r.admission.Root, "original"))
	r.require(true, "frozen complete original family", err)
	seed := manualCheckpointSeedRecord{BindingHash: r.bindingHash, SQL: ledger, Digest: digest, Head: head, Commit: commit, CommitArtifact: artifact, Family: family}
	r.baseline = family
	raw, err = json.Marshal(seed)
	r.require(err == nil && len(raw) <= 4<<20, "bounded seed record", err)
	_, err = r.writeExclusive(filepath.Join(r.admission.Root, "control/seed.json"), raw, true)
	r.require(true, "retained complete seed", err)
	return seed
}
func (r *manualCheckpointRun) loadSeed() manualCheckpointSeedRecord {
	var seed manualCheckpointSeedRecord
	raw, _, err := r.read(filepath.Join(r.admission.Root, "control/seed.json"), 4<<20)
	r.require(true, "retained seed record", err)
	r.require(true, "canonical seed", manualCheckpointDecode(raw, &seed))
	digest, problem := checkpointrestart.LedgerDigest(seed.SQL)
	r.require(seed.BindingHash == r.bindingHash && problem == checkpointrestart.NoProblem && digest == seed.Digest && seed.Head.Manual.Ledger == digest && seed.Commit.CommitReturnedNil && reflect.DeepEqual(seed.Commit.Head, seed.Head), "exact committed C origin", nil)
	commitRaw, commitInfo, err := r.read(filepath.Join(r.admission.Root, "control/committed-head.json"), 4<<20)
	r.require(true, "durable commit receipt readback", err)
	var commit manualCheckpointCommitReceipt
	r.require(true, "canonical committed head", manualCheckpointDecode(commitRaw, &commit))
	stat := commitInfo.Sys().(*syscall.Stat_t)
	r.require(reflect.DeepEqual(commit, seed.Commit) && seed.CommitArtifact == (manualCheckpointArtifact{"committed-head.json", uint64(stat.Dev), stat.Ino, commitInfo.Size(), sha256.Sum256(commitRaw)}) && commit.BindingHash == r.bindingHash && commit.Source == r.admission.SourceManifest && commit.Binary == r.admission.Binary && commit.Profile == r.profile && commit.Case == r.admission.Case && commit.Fixture == checkpointrestart.FixtureID && commit.LedgerDigest == digest, "immutable external committed-head receipt", nil)
	family, err := r.family(filepath.Join(r.admission.Root, "original"))
	r.require(err == nil && reflect.DeepEqual(family, seed.Family), "original complete family preserved", err)
	return seed
}

func (r *manualCheckpointRun) saveRole() {
	r.result.Providers = r.providerResults()
	for _, o := range r.owners {
		r.result.Owners = append(r.result.Owners, manualCheckpointOwnerResult{o.ID, o.Parent, o.State, o.Attempts})
	}
	r.result.Registry = manualCheckpointRegistry()
	r.result.PhysicalValidated = false
	r.require(r.result.Registry == [3]int{} && !r.ownerPending, "post-role registries empty", nil)
	raw, err := json.Marshal(r.result)
	r.require(err == nil && len(raw) <= 4<<20, "bounded complete role result", err)
	artifact, err := r.writeExclusive(filepath.Join(r.admission.Root, "control", r.role+".result.json"), raw, true)
	r.require(true, "role result retained", err)
	grant, err := r.channel.Exchange("role-result", 0, checkpointrestart.ManualObservation{}, artifact.Hash)
	r.require(err == nil && grant == "done", "role completion grant", err)
	r.require(true, "child fixed pipes closed once", r.channel.Close())
}
func (r *manualCheckpointRun) receiveRole(child *manualCheckpointChild) manualCheckpointRoleResult {
	frame, raw := r.receive(child)
	r.require(frame.Kind == "role-result" && frame.Point == 0 && frame.Observation == (checkpointrestart.ManualObservation{}) && frame.PayloadHash != [32]byte{}, "complete role result event", nil)
	r.grant(child, frame, raw, "done")
	r.waitChild(child, 0)
	data, _, err := r.read(filepath.Join(r.admission.Root, "control", child.binding.Role+".result.json"), 4<<20)
	r.require(err == nil && sha256.Sum256(data) == frame.PayloadHash, "waited result hash", err)
	var result manualCheckpointRoleResult
	r.require(true, "canonical role result", manualCheckpointDecode(data, &result))
	r.require(result.Role == child.binding.Role && result.Generation == child.binding.Generation && !result.PhysicalValidated && result.Registry == [3]int{} && result.OpenCalls == 1 && result.CloseCalls == 1 && !result.NilDB, "independent public role lifecycle", nil)
	wantProviders := 0
	if r.encrypted() {
		wantProviders = 1
	}
	r.require(len(result.Providers) == wantProviders, "fixed one-key provider inventory", nil)
	if wantProviders == 1 {
		p := result.Providers[0]
		purpose, path := "writer-open", r.path("trial")
		if child.binding.Role == "probe" {
			purpose, path = "probe-open", r.path("probe")
		}
		r.require(p.Purpose == purpose && p.Path == path && p.Operation == encryption.OperationOpen && p.Acquires == 1 && p.Releases == 1 && !p.Duplicate, "immutable credential purpose and retirement", nil)
	}
	for _, o := range result.Owners {
		r.require(o.State == "absent" && o.Attempts == 0 || o.State == "closed" && o.Attempts == 1, "role one-time owner retirement", nil)
	}
	return result
}
func TestManualCheckpointRestartWriter(t *testing.T) {
	r := manualCheckpointAdmit(t, "writer")
	defer r.finish()
	seed := r.loadSeed()
	db, openErr := sqlite.OpenContext(context.Background(), r.path("trial"), r.options("writer-open"))
	capturedOpenErr := openErr
	owner := r.capture(nil, db, nil, "")
	r.require(capturedOpenErr == nil && db != nil, "one writer public Open", capturedOpenErr)
	ledger := r.ledger(db, owner)
	r.require(checkpointrestart.CompareLedgers(seed.SQL, ledger) == checkpointrestart.NoProblem, "writer all committed C streams and indexes", nil)
	head := r.head(db, r.path("trial"), seed.Digest, true)
	r.require(reflect.DeepEqual(head.Plain, seed.Head.Plain) && head.Info == seed.Head.Info && head.BaseFileID == seed.Head.BaseFileID && head.LogicalHash == seed.Head.LogicalHash && head.Manual.OldRoot == seed.Head.Manual.OldRoot && head.Manual.OldRootHash == seed.Head.Manual.OldRootHash && head.Manual.AuthenticatedHeadCommitID == seed.Head.Manual.AuthenticatedHeadCommitID, "writer original committed authority", nil)
	family, err := r.family(filepath.Join(r.admission.Root, "trial"))
	r.require(true, "writer admission family", err)
	// An abrupt writer never reaches saveRole. Retain and check the one
	// released/cleared same-key borrow before readiness and before arming.
	providers := r.providerResults()
	ready := manualCheckpointReady{BindingHash: r.bindingHash, Head: head, Family: family, LedgerDigest: seed.Digest, SQL: ledger, Providers: providers}
	raw, err := json.Marshal(ready)
	r.require(true, "checkpoint-ready receipt", err)
	artifact, err := r.writeExclusive(filepath.Join(r.admission.Root, "control/writer.ready.json"), raw, true)
	r.require(true, "retained checkpoint-ready receipt", err)
	grant, err := r.channel.Exchange("checkpoint-ready", 0, checkpointrestart.ManualObservation{}, artifact.Hash)
	r.require(err == nil && grant == "continue", "parent verified committed-head grant", err)
	ctx, err := checkpointrestart.ManualContext(context.Background(), r.path("trial"), head.Manual, r.channel)
	r.require(true, "single checkpoint purpose", err)
	checkpointErr := db.Checkpoint(ctx)
	capturedCheckpointErr := checkpointErr
	r.causes = append(r.causes, capturedCheckpointErr)
	r.require(capturedCheckpointErr == nil, "original public Checkpoint nil", capturedCheckpointErr)
	r.require(true, "complete control nil-return barrier", checkpointrestart.ManualReturnedNil(ctx))
	_, _, _, target, _ := checkpointrestart.ManualCase(r.admission.Case)
	r.require(target == 0, "only ordinary control returns", nil)
	r.require(true, "control DB.Close once with KeepWALOnClose", r.close(owner))
	r.result.OpenCalls, r.result.CloseCalls, r.result.Outcome = 1, 1, "CONTROL_CLOSED"
	r.saveRole()
}
func (r *manualCheckpointRun) probePreserved(before, after manualCheckpointFamily) bool {
	if r.encrypted() {
		return reflect.DeepEqual(before, after)
	}
	for _, a := range before {
		if a.Name == r.rootName()+"-shm" {
			continue
		}
		b, ok := manualCheckpointFind(after, a.Name)
		if !ok || a != b {
			return false
		}
	}
	for _, b := range after {
		if b.Name != r.rootName()+"-shm" {
			if _, ok := manualCheckpointFind(before, b.Name); !ok {
				return false
			}
		}
	}
	return true
}
func TestManualCheckpointRestartProbe(t *testing.T) {
	r := manualCheckpointAdmit(t, "probe")
	defer r.finish()
	seed := r.loadSeed()
	var selected checkpointrestart.ManualObservation
	raw, _, err := r.read(filepath.Join(r.admission.Root, "control/selected.json"), manualCheckpointLimit)
	r.require(true, "selected checkpoint observation", err)
	r.require(true, "canonical selected observation", manualCheckpointDecode(raw, &selected))
	before, err := r.family(filepath.Join(r.admission.Root, "probe"))
	r.require(true, "private complete probe copy", err)
	db, openErr := sqlite.OpenContext(context.Background(), r.path("probe"), r.options("probe-open"))
	capturedOpenErr := openErr
	owner := r.capture(nil, db, nil, "")
	r.causes = append(r.causes, capturedOpenErr)
	r.result.OpenCalls, r.result.NilDB = 1, db == nil
	r.require(capturedOpenErr == nil && db != nil, "single independent normal public reopen", capturedOpenErr)
	ledger := r.ledger(db, owner)
	r.require(checkpointrestart.CompareLedgers(seed.SQL, ledger) == checkpointrestart.NoProblem, "probe full committed C ledger unchanged", nil)
	head := r.head(db, r.path("probe"), seed.Digest, false)
	if r.encrypted() {
		r.require(checkpointrestart.ManualRootValid(selected.Root, seed.Head.Manual), "source-authenticated candidate receipt", nil)
		wantHeader, wantHash := selected.Root.Candidate, selected.Root.CandidateHash
		if r.admission.Case == "encrypted-before-root-rename" {
			wantHeader, wantHash = seed.Head.Manual.OldRoot, seed.Head.Manual.OldRootHash
			r.require(head.Info == seed.Head.Info, "complete original authenticated head", nil)
		} else {
			r.require(head.Info.CommitSequence == 5 && head.Info.WALSegments == 0 && head.Info.WALFrames == 0 && head.Info.WALBytes == 0 && head.Info.HeadCommitID != seed.Head.Info.HeadCommitID, "authenticated new base head without segments", nil)
		}
		r.require(head.Manual.OldRoot == wantHeader && head.Manual.OldRootHash == wantHash && head.Info.DatabaseID == seed.Head.Info.DatabaseID && head.Info.PageCount == seed.Head.Info.PageCount && head.Manual.OldRoot.Epoch == seed.Head.Manual.OldRoot.Epoch, "exact selected root with same complete Epoch", nil)
		r.result.Authenticated, r.result.Header, r.result.RootHash, r.result.Info = true, wantHeader, wantHash, head.Info
	} else {
		r.require(head.LogicalHash == seed.Head.LogicalHash && head.Plain.DBSize == seed.SQL.Check.Pages, "exact complete logical plaintext head C", nil)
	}
	r.result.SQL = ledger
	r.require(true, "probe DB.Close once", r.close(owner))
	r.result.CloseCalls, r.result.Outcome = 1, "COMMITTED_C_RECOVERED"
	after, err := r.family(filepath.Join(r.admission.Root, "probe"))
	r.require(err == nil && r.probePreserved(before, after), "probe main/WAL or ciphertext preserved; transient SHM only", err)
	r.saveRole()
}
func (r *manualCheckpointRun) postWaitAuthority(seed manualCheckpointSeedRecord, ready manualCheckpointReady, family manualCheckpointFamily) {
	_, first, _, target, _ := checkpointrestart.ManualCase(r.admission.Case)
	if r.encrypted() {
		root, ok := manualCheckpointFind(family, r.rootName())
		r.require(ok, "selected encrypted root present", nil)
		want := r.observation.Root.CandidateHash
		if target == 5 {
			want = seed.Head.Manual.OldRootHash
		}
		r.require(root.Hash == want, "exact same-kernel selected root after actual Wait", nil)
		return
	}
	walAfter, hasWAL := manualCheckpointFind(family, r.rootName()+"-wal")
	if first == 1 && target != 1 {
		main, ok := manualCheckpointFind(family, r.rootName())
		r.require(!hasWAL && ok && main.Bytes == int64(seed.SQL.Check.Pages)*4096, "exclusive new main authority after WAL retirement", nil)
	} else {
		walBefore, ok := manualCheckpointFind(ready.Family, r.rootName()+"-wal")
		r.require(ok && hasWAL && walBefore == walAfter && walAfter.Hash == seed.Head.Plain.WALHash, "original committed WAL unchanged", nil)
	}
	if first == 3 {
		for _, suffix := range []string{"", "-wal", "-shm"} {
			a, x := manualCheckpointFind(ready.Family, r.rootName()+suffix)
			b, y := manualCheckpointFind(family, r.rootName()+suffix)
			r.require(x && y && a.Device == b.Device && a.Inode == b.Inode, "shared native inode retained", nil)
		}
		if target != 0 {
			shm, _, err := r.read(r.path("trial")+"-shm", manualCheckpointFileLimit)
			r.require(err == nil && len(shm) >= 132, "post-exit shared publication receipt", err)
			backfill := ready.Head.Manual.Page.Backfill
			if target == 4 {
				backfill = ready.Head.Manual.Page.Safe
			}
			r.require(binary.NativeEndian.Uint32(shm[128:132]) == ready.Head.Manual.Page.Safe && binary.NativeEndian.Uint32(shm[96:100]) == backfill, "owning-session attempted/backfill at cut", nil)
		}
	}
}
func TestManualCheckpointRestartController(t *testing.T) {
	r := manualCheckpointAdmit(t, "controller")
	defer r.finish()
	r.require(true, "fresh retained run directory", r.mkdir(r.admission.Root))
	seed := r.seed()
	trialInitial, err := r.copyFamily(filepath.Join(r.admission.Root, "original"), filepath.Join(r.admission.Root, "trial"))
	r.require(true, "independent complete writer family", err)
	writer := r.start("writer")
	frame, raw := r.receive(writer)
	r.require(frame.Kind == "checkpoint-ready" && frame.Point == 0 && frame.Observation == (checkpointrestart.ManualObservation{}) && frame.PayloadHash != [32]byte{}, "writer ready after full C verification", nil)
	data, _, err := r.read(filepath.Join(r.admission.Root, "control/writer.ready.json"), 4<<20)
	r.require(err == nil && sha256.Sum256(data) == frame.PayloadHash, "causal ready artifact", err)
	var ready manualCheckpointReady
	r.require(true, "canonical ready artifact", manualCheckpointDecode(data, &ready))
	r.require(ready.BindingHash == r.bindingHash && checkpointrestart.CompareLedgers(seed.SQL, ready.SQL) == checkpointrestart.NoProblem && ready.LedgerDigest == seed.Digest && ready.Head.Manual.Ledger == seed.Digest && reflect.DeepEqual(ready.Head.Plain, seed.Head.Plain) && ready.Head.Info == seed.Head.Info && ready.Head.BaseFileID == seed.Head.BaseFileID && ready.Head.Manual.OldRoot == seed.Head.Manual.OldRoot && ready.Head.Manual.OldRootHash == seed.Head.Manual.OldRootHash && ready.Head.Manual.AuthenticatedHeadCommitID == seed.Head.Manual.AuthenticatedHeadCommitID && ready.Head.LogicalHash == seed.Head.LogicalHash && manualCheckpointDistinct(seed.Family, ready.Family), "parent verifies original authenticated committed C", nil)
	wantProviders := 0
	if r.encrypted() {
		wantProviders = 1
	}
	r.require(len(ready.Providers) == wantProviders, "pre-arm writer credential inventory", nil)
	if wantProviders == 1 {
		p := ready.Providers[0]
		r.require(p.Purpose == "writer-open" && p.Path == r.path("trial") && p.Operation == encryption.OperationOpen && p.Acquires == 1 && p.Releases == 1 && !p.Duplicate, "pre-arm exact writer purpose and released borrow", nil)
	}
	for _, initial := range trialInitial {
		if initial.Name != r.rootName()+"-shm" {
			got, ok := manualCheckpointFind(ready.Family, initial.Name)
			r.require(ok && got == initial, "writer admission preserved source bytes/inodes", nil)
		}
	}
	r.grant(writer, frame, raw, "continue")
	_, first, last, target, _ := checkpointrestart.ManualCase(r.admission.Case)
	for point := first; point <= last; point++ {
		frame, raw = r.receive(writer)
		r.require(frame.Kind == "checkpoint-observation" && frame.Point == point && frame.PayloadHash == seed.Digest, "ordered causal checkpoint cut", nil)
		if first < 5 {
			page := frame.Observation.Page
			base := page
			base.Page, base.Copied = 0, 0
			r.require(frame.Observation.Root == (checkpointrestart.ManualRoot{}) && base == ready.Head.Manual.Page && page.Selected >= 2, "exact scalar WAL generation", nil)
			if point == first {
				r.require(page.Page > 0 && page.Page <= page.DBSize && page.Copied == 1, "first successful selected page only", nil)
			} else {
				r.require(page.Page == 0 && page.Copied == page.Selected, "all selected copies before terminal publication", nil)
			}
		} else {
			r.require(frame.Observation.Page == (checkpointrestart.ManualPage{}) && checkpointrestart.ManualRootValid(frame.Observation.Root, ready.Head.Manual), "authenticated same-Epoch candidate and original head parent", nil)
			if point != first {
				r.require(frame.Observation == r.observation, "same exact candidate across root cuts", nil)
			}
		}
		r.observation = frame.Observation
		r.modelObservations = append(r.modelObservations, checkpointrestart.ManualModelObservation(point, frame.Observation))
		if point == target {
			r.grant(writer, frame, raw, "exit")
			break
		}
		r.grant(writer, frame, raw, "continue")
	}
	publicReturned := false
	if target == 0 {
		frame, raw = r.receive(writer)
		r.require(frame.Kind == "public-checkpoint-nil" && frame.Point == 0 && frame.Observation == r.observation && frame.PayloadHash == seed.Digest, "actual public checkpoint nil after all cuts", nil)
		publicReturned = true
		r.grant(writer, frame, raw, "continue")
		control := r.receiveRole(writer)
		r.require(control.Outcome == "CONTROL_CLOSED", "complete ordinary control", nil)
	} else {
		r.waitChild(writer, checkpointrestart.ManualExitCode)
	}
	// Only actual owned Wait plus terminal event EOF permits any evidence copy.
	r.require(writer.waited && writer.terminalEOF && !writer.unknown, "writer definitely exited before copy", nil)
	trial, err := r.family(filepath.Join(r.admission.Root, "trial"))
	r.require(true, "frozen post-Wait trial", err)
	r.postWaitAuthority(seed, ready, trial)
	caseID, caseOK := checkpointrestart.ParseCase(r.admission.Case)
	r.require(caseOK, "closed source-model case", nil)
	var old checkpointrestart.EncryptedHead
	var candidate checkpointrestart.RootCandidate
	if r.encrypted() {
		old = manualCheckpointModelEncrypted(seed.Head)
		candidate = checkpointrestart.RootCandidate{RootHash: r.observation.Root.CandidateHash, RootBytes: 1064, Header: checkpointrestart.ManualModelHeader(r.observation.Root.Candidate)}
	}
	r.require(checkpointrestart.ValidateAuthority(caseID, ready.Head.Plain, old, candidate, r.modelObservations) == checkpointrestart.NoProblem, "source-model/native authority scalar parity", nil)
	r.postExit, err = r.copyFamily(filepath.Join(r.admission.Root, "trial"), filepath.Join(r.admission.Root, "evidence/post-exit"))
	r.require(true, "frozen evidence family", err)
	probeFamily, err := r.copyFamily(filepath.Join(r.admission.Root, "evidence/post-exit"), filepath.Join(r.admission.Root, "probe"))
	r.require(true, "one independent private probe", err)
	for _, pair := range [][2]manualCheckpointFamily{{seed.Family, trial}, {seed.Family, r.postExit}, {seed.Family, probeFamily}, {trial, r.postExit}, {trial, probeFamily}, {r.postExit, probeFamily}} {
		r.require(manualCheckpointDistinct(pair[0], pair[1]), "all four family inode sets disjoint", nil)
	}
	selected, err := json.Marshal(r.observation)
	r.require(true, "selected observation artifact", err)
	_, err = r.writeExclusive(filepath.Join(r.admission.Root, "control/selected.json"), selected, true)
	r.require(true, "selected artifact durable", err)
	probe := r.start("probe")
	recovered := r.receiveRole(probe)
	r.require(recovered.Outcome == "COMMITTED_C_RECOVERED" && checkpointrestart.CompareLedgers(seed.SQL, recovered.SQL) == checkpointrestart.NoProblem && recovered.Authenticated == r.encrypted(), "independent full committed C recovered", nil)
	if r.encrypted() {
		wantHeader, wantHash := r.observation.Root.Candidate, r.observation.Root.CandidateHash
		if target == 5 {
			wantHeader, wantHash = seed.Head.Manual.OldRoot, seed.Head.Manual.OldRootHash
			r.require(recovered.Info == seed.Head.Info, "parent exact old authenticated Info", nil)
		} else {
			r.require(recovered.Info.CommitSequence == 5 && recovered.Info.WALSegments == 0 && recovered.Info.WALFrames == 0 && recovered.Info.WALBytes == 0 && recovered.Info.HeadCommitID != seed.Head.Info.HeadCommitID, "parent selected new authenticated base", nil)
		}
		r.require(recovered.Header == wantHeader && recovered.RootHash == wantHash && recovered.Info.StorageLayout == 2 && recovered.Info.Mode == sqlite.EncryptionWAL && recovered.Info.DatabaseID == seed.Head.Info.DatabaseID && recovered.Header.Epoch == seed.Head.Manual.OldRoot.Epoch, "parent full same-key selected identity", nil)
	}
	for _, frozen := range []struct {
		name     string
		manifest manualCheckpointFamily
	}{{"original", seed.Family}, {"trial", trial}, {"evidence/post-exit", r.postExit}} {
		after, err := r.family(filepath.Join(r.admission.Root, frozen.name))
		r.require(err == nil && reflect.DeepEqual(after, frozen.manifest), "full frozen family final recheck", err)
	}
	probeAfter, err := r.family(filepath.Join(r.admission.Root, "probe"))
	r.require(err == nil && r.probePreserved(probeFamily, probeAfter), "private probe allowed changes only", err)
	// Only actual scalar/ledger model predicates were evaluated here. Do not
	// synthesize the pure model's conceptual all-events Evidence transcript.
	receipt := struct {
		Status, Case, Scope                                                                                                                                                                         string
		SourceModelConsistent, FullModelEvidenceEvaluated, AuthorityModelConsistent, LedgerModelConsistent, PublicCheckpointReturned, ActualWaitObserved, CommittedHeadRecovered, PhysicalValidated bool
		Seed                                                                                                                                                                                        manualCheckpointSeedRecord
		Observation                                                                                                                                                                                 checkpointrestart.ManualObservation
		Probe                                                                                                                                                                                       manualCheckpointRoleResult
		Trial, PostExit, ProbeBefore, ProbeAfter                                                                                                                                                    manualCheckpointFamily
	}{Status: "OBSERVATIONS_RECORDED", Case: r.admission.Case, Scope: "same-kernel-controlled-process-exit; no-power-loss-qualification", SourceModelConsistent: false, FullModelEvidenceEvaluated: false, AuthorityModelConsistent: true, LedgerModelConsistent: true, PublicCheckpointReturned: publicReturned, ActualWaitObserved: writer.waited && probe.waited && writer.terminalEOF && probe.terminalEOF, CommittedHeadRecovered: true, PhysicalValidated: false, Seed: seed, Observation: r.observation, Probe: recovered, Trial: trial, PostExit: r.postExit, ProbeBefore: probeFamily, ProbeAfter: probeAfter}
	raw, err = json.Marshal(receipt)
	r.require(err == nil && len(raw) <= 4<<20, "bounded distinct-claim observation receipt", err)
	_, err = r.writeExclusive(filepath.Join(r.admission.Root, "control/observations.json"), raw, true)
	r.require(true, "retained final observations", err)
}
