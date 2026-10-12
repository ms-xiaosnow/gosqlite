//go:build linux && manual_fault_rollback && manual_restart_rollback

package sqlite

// SOURCE ONLY / REVIEW HOLD. All native actions described here are NOT_RUN.
// Exactly one controller case and two fresh, self-image-bound children. No
// command shell, signal, automatic kill, cleanup retry, or original recovery.
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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackrestart"
)

const manualRestartLimit = 65536
const manualRestartWait = 30 * time.Second

type manualRestartBinding struct {
	Schema             string
	Case               string
	Trial              string
	SourceTree         string
	SourceManifest     string
	SourceManifestPath string
	Tool               string
	ToolPath           string
	Binary             string
	BinaryPath         string
	CompileReceipt     string
	CompileReceiptPath string
	Filesystem         string
	NativeDeclaration  string
}

type manualRestartConfig struct {
	sql              manualRollbackSQLConfig
	binding          manualRestartBinding
	rawBinding       []byte
	root             string
	family           [5]manualRollbackSQLConfig
	plan             rollbackrestart.Plan
	role             rollbackrestart.Role
	selfInfo         os.FileInfo
	baseline         [12288]byte
	retained         [24]manualRestartArtifact
	retainedCount    int
	active           *manualRestartChild
	children         [2]*manualRestartChild
	receipt          *rollbackrestart.Receipt
	evidence         *rollbackrestart.ScenarioEvidence
	durableAck       rollbackrestart.AckEvidence
	failureAttempted bool
}

type manualRestartArtifact struct {
	Path   string
	Device uint64
	Inode  uint64
	Bytes  int64
	Hash   string
	Exists bool
}

// Limited diagnostics remain separate from protocol. Exhaustion makes Wait
// fail, never counts as an expected child exit and never kills the child.
type manualRestartDiagnostic struct {
	mu       sync.Mutex
	data     [manualRestartLimit]byte
	count    int
	overflow bool
}

func (d *manualRestartDiagnostic) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	available := len(d.data) - d.count
	if len(p) > available {
		copy(d.data[d.count:], p[:available])
		d.count += available
		d.overflow = true
		return available, io.ErrShortWrite
	}
	copy(d.data[d.count:], p)
	d.count += len(p)
	return len(p), nil
}
func (d *manualRestartDiagnostic) snapshot() ([]byte, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]byte(nil), d.data[:d.count]...), d.overflow
}

type manualRestartWaitResult struct {
	state *os.ProcessState
	err   error
}
type manualRestartChild struct {
	command      *exec.Cmd
	role         rollbackrestart.Role
	identity     rollbackrestart.RoleIdentity
	pid          int
	controls     *os.File
	events       *os.File
	controlOwner int
	eventOwner   int
	stdout       manualRestartDiagnostic
	stderr       manualRestartDiagnostic
	wait         chan manualRestartWaitResult
	waited       bool
	timedOut     bool
	actual       manualRestartWaitResult
	frames       [rollbackrestart.MaxFrames]rollbackrestart.Frame
	frameCount   uint8
	rawFrames    [rollbackrestart.MaxFrames][]byte
	rawCount     uint8
	lastGrant    rollbackrestart.Frame
	grantBytes   int
	grantWritten int
	grantError   string
	sequence     uint8
}

func manualRestartDigest(raw []byte) rollbackrestart.Digest {
	sum := sha256.Sum256(raw)
	return rollbackrestart.Digest(hex.EncodeToString(sum[:]))
}
func manualRestartID(info os.FileInfo) (rollbackrestart.FileIdentity, error) {
	if info == nil {
		return rollbackrestart.FileIdentity{}, fmt.Errorf("missing actual stat")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Dev == 0 || stat.Ino == 0 {
		return rollbackrestart.FileIdentity{}, fmt.Errorf("native dev/inode unavailable")
	}
	return rollbackrestart.FileIdentity{Device: uint64(stat.Dev), Inode: stat.Ino}, nil
}
func manualRestartRegular(info os.FileInfo) error {
	if info == nil || !info.Mode().IsRegular() {
		return fmt.Errorf("regular file required")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 || stat.Uid != uint32(os.Getuid()) {
		return fmt.Errorf("single-link owner-controlled file required")
	}
	return nil
}
func manualRestartRead(c *manualRestartConfig, path string, bound int64) ([]byte, os.FileInfo, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, nil, fmt.Errorf("absolute clean input path required")
	}
	current := string(filepath.Separator)
	for _, component := range strings.Split(strings.TrimPrefix(filepath.Dir(path), "/"), "/") {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, nil, fmt.Errorf("input ancestor unverified: %s: %v", current, err)
		}
	}
	raw, info, err := manualRollbackSQLRead(&c.sql.ledger, path, bound)
	if err != nil {
		return nil, nil, err
	}
	if err := manualRestartRegular(info); err != nil {
		return nil, nil, err
	}
	return raw, info, nil
}
func manualRestartDecode(raw []byte, value any) error {
	if len(raw) == 0 || len(raw) > 4<<20 {
		return fmt.Errorf("bounded canonical JSON required")
	}
	if err := manualRollbackSQLDecode(raw, value); err != nil {
		return err
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(raw, canonical) {
		return fmt.Errorf("noncanonical, duplicate, omitted, or unknown JSON fields: %v", err)
	}
	return nil
}
func manualRestartFlags(role rollbackrestart.Role) ([]string, error) {
	root := ""
	switch role {
	case rollbackrestart.Controller:
		root = "^TestManualRollbackRestartController$"
	case rollbackrestart.Writer:
		root = "^TestManualRollbackRestartWriter$"
	case rollbackrestart.Recovery:
		root = "^TestManualRollbackRestartRecovery$"
	default:
		return nil, fmt.Errorf("unknown role")
	}
	return []string{"-test.run=" + root, "-test.count=1", "-test.shuffle=off", "-test.parallel=1", "-test.timeout=0", "-test.cpu=1", "-test.v=true"}, nil
}
func manualRestartCheckFlags(role rollbackrestart.Role) error {
	want, err := manualRestartFlags(role)
	if err != nil || len(os.Args) != len(want)+1 {
		return fmt.Errorf("exact test root and seven flags required")
	}
	for i := range want {
		if os.Args[i+1] != want[i] {
			return fmt.Errorf("unexpected flag or flag order")
		}
	}
	return nil
}

// /proc/self/exe is the one explicit procfs exception to the no-symlink input
// policy. Hash the opened running image, then compare fstat before and after;
// argv[0], PATH and a replacement executable with the same name are insufficient.
func manualRestartImage(c *manualRestartConfig) error {
	i, err := c.sql.ledger.reserve()
	if err != nil {
		return err
	}
	image, openErr := os.Open("/proc/self/exe")
	c.sql.ledger.owners[i] = manualRollbackSQLOwner{file: image, path: "/proc/self/exe"}
	if openErr != nil {
		return errors.Join(openErr, c.sql.ledger.close(i))
	}
	before, statErr := image.Stat()
	if statErr != nil || before.Size() < 1 || before.Size() > 128<<20 || !before.Mode().IsRegular() {
		return errors.Join(fmt.Errorf("running image unverified"), statErr, c.sql.ledger.close(i))
	}
	raw, readErr := io.ReadAll(io.LimitReader(image, (128<<20)+1))
	after, afterErr := image.Stat()
	closeErr := c.sql.ledger.close(i)
	if err := errors.Join(readErr, afterErr, closeErr); err != nil {
		return err
	}
	if int64(len(raw)) != before.Size() || !os.SameFile(before, after) || before.Size() != after.Size() || manualRestartDigest(raw) != rollbackrestart.Digest(c.binding.Binary) {
		return fmt.Errorf("actual running image changed or digest mismatch")
	}
	if !filepath.IsAbs(c.binding.BinaryPath) || filepath.Clean(c.binding.BinaryPath) != c.binding.BinaryPath {
		return fmt.Errorf("absolute reviewed binary path required")
	}
	target, targetInfo, err := manualRestartRead(c, c.binding.BinaryPath, 128<<20)
	if err != nil || !os.SameFile(before, targetInfo) || manualRestartDigest(target) != rollbackrestart.Digest(c.binding.Binary) {
		return fmt.Errorf("spawn target differs from running image: %v", err)
	}
	c.selfInfo = before
	return nil
}

func manualRestartAdmit(c *manualRestartConfig, role rollbackrestart.Role) error {
	// Must dominate flags, paths, stat, descriptors, owners, nonce and process work.
	if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" || os.Getenv("GOSQLITE_MANUAL_RESTART_ROLLBACK") != "independently_authorized" {
		return fmt.Errorf("ADMISSION_BLOCKED: independent fault and restart authorization required")
	}
	if err := manualRestartCheckFlags(role); err != nil {
		return err
	}
	if os.Getenv("GOSQLITE_RESTART_PRIVATE_ROOT_CONFIRMED") != "private_stable_namespace_retained" || os.Getenv("GOSQLITE_RESTART_ROLE") != fmt.Sprint(uint8(role)) {
		return fmt.Errorf("role and private retained namespace confirmation required")
	}
	c.role = role
	c.root = os.Getenv("GOSQLITE_RESTART_ROOT")
	rootInfo, err := manualRollbackSQLNamespace(c.root)
	if err != nil {
		return err
	}
	c.sql.root, c.sql.rootInfo = c.root, rootInfo
	var filesystem syscall.Statfs_t
	if err := syscall.Statfs(c.root, &filesystem); err != nil {
		return fmt.Errorf("native filesystem identity unavailable: %w", err)
	}
	raw, _, err := manualRestartRead(c, os.Getenv("GOSQLITE_RESTART_BINDING"), 65536)
	if err != nil {
		return err
	}
	if manualRestartDigest(raw) != rollbackrestart.Digest(os.Getenv("GOSQLITE_RESTART_BINDING_SHA256")) {
		return fmt.Errorf("reviewed binding digest mismatch")
	}
	if err := manualRestartDecode(raw, &c.binding); err != nil {
		return err
	}
	c.rawBinding = raw
	if c.binding.Filesystem != fmt.Sprintf("linux-fstype-%x", uint64(filesystem.Type)) {
		return fmt.Errorf("declared filesystem does not match native root statfs")
	}
	which, ok := rollbackrestart.ParseCase(c.binding.Case)
	if !ok || c.binding.Schema != "rollback-restart-binding-v1" || len(c.binding.Trial) < 1 || len(c.binding.Trial) > 64 || strings.ContainsAny(c.binding.Trial, " /\\\t\r\n") || c.binding.NativeDeclaration != "linux-native-process-exit-no-powerloss-4096-retain-all" || c.binding.Filesystem == "" || len(c.binding.Filesystem) > 64 {
		return fmt.Errorf("fixed case, trial and explicit native declaration required")
	}
	manifest, _, err := manualRestartRead(c, c.binding.SourceManifestPath, 4<<20)
	if err != nil || manualRestartDigest(manifest) != rollbackrestart.Digest(c.binding.SourceManifest) {
		return fmt.Errorf("source manifest mismatch: %v", err)
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
	if inventory.Tree != c.binding.SourceTree || len(inventory.Tree) != 40 || len(inventory.Files) < 1 || len(inventory.Files) > 4096 {
		return fmt.Errorf("complete finite source inventory required")
	}
	for path, entry := range inventory.Files {
		if filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || strings.HasPrefix(path, "../") || entry.Bytes < 0 || (entry.GitMode != "100644" && entry.GitMode != "100755") || len(entry.GitBlob) != 40 {
			return fmt.Errorf("invalid source inventory entry")
		}
		if _, err := manualRollbackSQLHash(entry.SHA256); err != nil {
			return err
		}
	}
	compiled, _, err := manualRestartRead(c, c.binding.CompileReceiptPath, 65536)
	if err != nil || manualRestartDigest(compiled) != rollbackrestart.Digest(c.binding.CompileReceipt) {
		return fmt.Errorf("compile receipt mismatch: %v", err)
	}
	var receipt manualRollbackSQLCompile
	if err := manualRestartDecode(compiled, &receipt); err != nil {
		return err
	}
	if receipt.Schema != "rollback-restart-compile-v1" || receipt.Source != c.binding.SourceManifest || receipt.SourceTree != c.binding.SourceTree || receipt.Tool != c.binding.Tool || receipt.Binary != c.binding.Binary || receipt.Manifest != c.binding.SourceManifest || receipt.Package != "./sqlite" || receipt.Tags != "manual_fault_rollback,manual_restart_rollback" || !receipt.CompileOnly || int(receipt.Files) != len(inventory.Files) {
		return fmt.Errorf("reviewed source-to-binary receipt does not bind this restart image")
	}
	tool, _, err := manualRestartRead(c, c.binding.ToolPath, 128<<20)
	if err != nil || manualRestartDigest(tool) != rollbackrestart.Digest(c.binding.Tool) {
		return fmt.Errorf("tool image digest mismatch: %v", err)
	}
	if err := manualRestartImage(c); err != nil {
		return err
	}
	baseline, err := manualRollbackSQLBaseline()
	if err != nil {
		return err
	}
	c.baseline = baseline
	rootID, err := manualRestartID(rootInfo)
	if err != nil {
		return err
	}
	binding := rollbackrestart.Binding{Suite: "rollback-restart-v1", Profile: rollbackfault.PlainRollback4096, Case: which, Trial: c.binding.Trial, Source: c.binding.SourceTree, Manifest: rollbackrestart.Digest(c.binding.SourceManifest), Tool: rollbackrestart.Digest(c.binding.Tool), Compile: rollbackrestart.Digest(c.binding.CompileReceipt), Binary: rollbackrestart.Digest(c.binding.Binary), Native: rollbackrestart.NativeIdentity{OS: "linux", Filesystem: c.binding.Filesystem, SectorSize: 4096, PageSize: 4096}, Root: rootID, Baseline: manualRestartDigest(baseline[:])}
	if role == rollbackrestart.Controller {
		imageID, err := manualRestartID(c.selfInfo)
		if err != nil {
			return err
		}
		spec := rollbackrestart.PlanSpec{Binding: binding, Ticket: 1, Image: imageID}
		identities := []*rollbackrestart.RoleIdentity{&spec.Controller, &spec.Writer, &spec.Recovery}
		for n, identity := range identities {
			var nonce [32]byte
			if count, err := rand.Read(nonce[:]); err != nil || count != len(nonce) {
				return fmt.Errorf("parent nonce generation failed: %v", err)
			}
			*identity = rollbackrestart.RoleIdentity{Role: rollbackrestart.Role(n + 1), Generation: uint64(n + 1), Challenge: rollbackrestart.Digest(hex.EncodeToString(nonce[:]))}
		}
		spec.AdapterIdentity = manualRestartSQLIdentity(binding, spec.Writer)
		plan, problem := rollbackrestart.NewPlan(spec)
		if problem != rollbackrestart.NoProblem {
			return fmt.Errorf("restart plan admission: %v", problem)
		}
		c.plan = plan
	} else {
		rawPlan, _, err := manualRestartRead(c, filepath.Join(c.root, "control", "plan.json"), 65536)
		if err != nil {
			return err
		}
		var spec rollbackrestart.PlanSpec
		if err := manualRestartDecode(rawPlan, &spec); err != nil {
			return err
		}
		if spec.Binding != binding || manualRestartDigest(rawPlan) != rollbackrestart.Digest(os.Getenv("GOSQLITE_RESTART_PLAN_SHA256")) {
			return fmt.Errorf("child plan/root/source/baseline identity mismatch")
		}
		c.plan, _ = rollbackrestart.NewPlan(spec)
		if c.plan.Spec() != spec {
			return fmt.Errorf("child plan refused")
		}
	}
	return c.sql.stable()
}

func manualRestartSQLIdentity(binding rollbackrestart.Binding, role rollbackrestart.RoleIdentity) rollbackfault.Identity {
	return rollbackfault.Identity{Suite: "rollback-restart-v1", Source: binding.Source, Tool: string(binding.Tool), Binary: string(binding.Binary), Manifest: string(binding.Manifest), Trial: binding.Trial, Scope: fmt.Sprintf("restart-role-%d-generation-%d", role.Role, role.Generation), Profile: rollbackfault.PlainRollback4096}
}

func manualRestartFamilies(c *manualRestartConfig, create bool) error {
	names := [5]string{"inputs", "trial", "evidence", "replay", "control"}
	if create {
		owner, err := c.sql.ledger.reserve()
		if err != nil {
			return err
		}
		directory, openErr := os.Open(c.root)
		c.sql.ledger.owners[owner] = manualRollbackSQLOwner{file: directory, path: c.root}
		if openErr != nil {
			return errors.Join(openErr, c.sql.ledger.close(owner))
		}
		actual, statErr := directory.Stat()
		if statErr != nil || !os.SameFile(actual, c.sql.rootInfo) {
			return errors.Join(fmt.Errorf("root owner identity changed"), statErr, c.sql.ledger.close(owner))
		}
		entries, readErr := directory.ReadDir(1)
		closeErr := c.sql.ledger.close(owner)
		if readErr != io.EOF || len(entries) != 0 || closeErr != nil {
			return errors.Join(fmt.Errorf("controller root must be empty"), readErr, closeErr)
		}
	}
	for i, name := range names {
		path := filepath.Join(c.root, name)
		if create {
			if err := os.Mkdir(path, 0700); err != nil {
				return err
			}
		}
		info, err := manualRollbackSQLNamespace(path)
		if err != nil {
			return err
		}
		c.family[i].root, c.family[i].rootInfo = path, info
	}
	return c.sql.stable()
}
func manualRestartSyncDirectory(c *manualRestartConfig, family int) error {
	if family < 0 || family >= len(c.family) {
		return fmt.Errorf("closed family required")
	}
	f := &c.family[family]
	if err := f.stable(); err != nil {
		return err
	}
	i, err := c.sql.ledger.reserve()
	if err != nil {
		return err
	}
	dir, openErr := os.Open(f.root)
	c.sql.ledger.owners[i] = manualRollbackSQLOwner{file: dir, path: f.root}
	if openErr != nil {
		return errors.Join(openErr, c.sql.ledger.close(i))
	}
	actual, statErr := dir.Stat()
	if statErr != nil || !os.SameFile(actual, f.rootInfo) {
		return errors.Join(fmt.Errorf("directory changed"), statErr, c.sql.ledger.close(i))
	}
	syncErr := dir.Sync()
	closeErr := c.sql.ledger.close(i)
	return errors.Join(syncErr, closeErr, f.stable())
}
func manualRestartCreate(c *manualRestartConfig, family int, name string, raw []byte) error {
	allowed := map[string]bool{
		"0/baseline.input.db": true, "0/binding.input.json": true,
		"1/trial.db": true, "2/post-exit.input.db": true, "2/post-exit-journal.input.bin": true,
		"3/replay.db": true, "3/replay.db-journal": true,
		"4/plan.json": true, "4/ack.json": true, "4/baseline-sql.json": true,
		"4/readback.json": true, "4/result.json": true, "4/final-manifest.json": true, "4/failure.json": true,
		"4/failure-wait.json": true, "4/failure-stdout.json": true, "4/failure-stderr.json": true, "4/failure-protocol.json": true,
		"4/writer-stdout.json": true, "4/writer-stderr.json": true,
		"4/recovery-stdout.json": true, "4/recovery-stderr.json": true,
		"4/writer-protocol.json": true, "4/recovery-protocol.json": true,
	}
	if !allowed[fmt.Sprintf("%d/%s", family, name)] {
		return fmt.Errorf("unreviewed artifact name")
	}
	if len(raw) == 0 {
		return fmt.Errorf("empty artifact body refused; diagnostics use explicit JSON envelopes")
	}
	info, err := c.family[family].create(name, raw, false)
	if err != nil {
		return err
	}
	if err := manualRestartRegular(info); err != nil {
		return err
	}
	if err := manualRestartSyncDirectory(c, family); err != nil {
		return err
	}
	if family == 4 && (name == "writer-stdout.json" || name == "writer-stderr.json" || name == "writer-protocol.json" || name == "recovery-stdout.json" || name == "recovery-stderr.json" || name == "recovery-protocol.json" || name == "readback.json" || name == "result.json" || name == "final-manifest.json") {
		return manualRestartRetain(c, "control/"+name, 1<<20)
	}
	return nil
}
func manualRestartArtifactAt(c *manualRestartConfig, relative string, bound int64) (manualRestartArtifact, []byte, error) {
	out := manualRestartArtifact{Path: relative}
	path := filepath.Join(c.root, relative)
	if filepath.IsAbs(relative) || filepath.Clean(relative) != relative || strings.HasPrefix(relative, "../") {
		return out, nil, fmt.Errorf("fixed relative artifact required")
	}
	raw, info, err := manualRestartRead(c, path, bound)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil, nil
	}
	if err != nil {
		return out, nil, err
	}
	id, err := manualRestartID(info)
	if err != nil {
		return out, nil, err
	}
	out.Device, out.Inode, out.Bytes, out.Hash, out.Exists = id.Device, id.Inode, int64(len(raw)), string(manualRestartDigest(raw)), true
	return out, raw, nil
}
func manualRestartRetain(c *manualRestartConfig, relative string, bound int64) error {
	stamp, _, err := manualRestartArtifactAt(c, relative, bound)
	if err != nil {
		return err
	}
	if c.retainedCount == len(c.retained) {
		return fmt.Errorf("artifact inventory full")
	}
	for i := 0; i < c.retainedCount; i++ {
		previous := c.retained[i]
		if previous.Path == relative || (stamp.Exists && previous.Exists && stamp.Device == previous.Device && stamp.Inode == previous.Inode) {
			return fmt.Errorf("duplicate or aliased original/copy")
		}
	}
	c.retained[c.retainedCount] = stamp
	c.retainedCount++
	return nil
}
func manualRestartUnchanged(c *manualRestartConfig) error {
	if err := c.sql.stable(); err != nil {
		return err
	}
	for i := 0; i < len(c.family); i++ {
		if err := c.family[i].stable(); err != nil {
			return err
		}
	}
	for i := 0; i < c.retainedCount; i++ {
		previous := c.retained[i]
		stamp, _, err := manualRestartArtifactAt(c, previous.Path, 1<<20)
		if err != nil || stamp != previous {
			return fmt.Errorf("ARTIFACT_CHANGED: %s: %v", previous.Path, err)
		}
	}
	return nil
}

func manualRestartAppend(c *manualRestartConfig, receipt *rollbackrestart.Receipt, frame rollbackrestart.Frame) error {
	if problem := receipt.Append(frame); problem != rollbackrestart.ProblemNone {
		return fmt.Errorf("PROTOCOL_INVALID: frame %d kind %d: %v", frame.Sequence, frame.Kind, problem)
	}
	return nil
}
func manualRestartParentFrame(c *manualRestartConfig, receipt *rollbackrestart.Receipt, kind rollbackrestart.FrameKind) rollbackrestart.Frame {
	_, count := receipt.Frames()
	return rollbackrestart.Frame{Schema: rollbackrestart.SchemaVersion, Binding: c.plan.Spec().Binding, Sender: c.plan.Spec().Controller, Sequence: count + 1, Kind: kind}
}
func manualRestartPipe(c *manualRestartConfig) (*os.File, *os.File, int, int, error) {
	readOwner, err := c.sql.ledger.reserve()
	if err != nil {
		return nil, nil, 0, 0, err
	}
	writeOwner, err := c.sql.ledger.reserve()
	if err != nil {
		return nil, nil, 0, 0, err
	}
	read, write, pipeErr := os.Pipe()
	c.sql.ledger.owners[readOwner] = manualRollbackSQLOwner{file: read, path: "owned restart pipe reader"}
	c.sql.ledger.owners[writeOwner] = manualRollbackSQLOwner{file: write, path: "owned restart pipe writer"}
	if pipeErr != nil {
		return nil, nil, readOwner, writeOwner, pipeErr
	}
	return read, write, readOwner, writeOwner, nil
}

func manualRestartSpawn(c *manualRestartConfig, receipt *rollbackrestart.Receipt, role rollbackrestart.Role) (*manualRestartChild, error) {
	if c.role != rollbackrestart.Controller || (role != rollbackrestart.Writer && role != rollbackrestart.Recovery) || !manualRollbackSQLRegistryEmpty() || !c.sql.ledger.retired() {
		return nil, fmt.Errorf("parent role and no live owners required before spawn")
	}
	slot := int(role) - int(rollbackrestart.Writer)
	if c.children[slot] != nil {
		return nil, fmt.Errorf("one actual process per fixed role; no retry")
	}
	if role == rollbackrestart.Recovery && (c.children[0] == nil || !c.children[0].waited || c.children[0].actual.state == nil || !c.children[0].actual.state.Exited()) {
		return nil, fmt.Errorf("recovery cannot spawn before actual writer Wait")
	}
	for i := range c.family {
		if !c.family[i].ledger.retired() {
			return nil, fmt.Errorf("family owner live before spawn")
		}
	}
	if err := manualRestartUnchanged(c); err != nil {
		return nil, err
	}
	if err := manualRestartImage(c); err != nil {
		return nil, err
	}
	args, err := manualRestartFlags(role)
	if err != nil {
		return nil, err
	}
	rawPlan, _, err := manualRestartRead(c, filepath.Join(c.root, "control", "plan.json"), 65536)
	if err != nil {
		return nil, err
	}
	_, count := receipt.Frames()
	child := &manualRestartChild{role: role, wait: make(chan manualRestartWaitResult, 1), sequence: count + 1}
	c.children[slot] = child
	child.identity = c.plan.Spec().Writer
	if role == rollbackrestart.Recovery {
		child.identity = c.plan.Spec().Recovery
	}
	controlRead, controlWrite, controlReadOwner, controlWriteOwner, err := manualRestartPipe(c)
	if err != nil {
		return child, err
	}
	eventRead, eventWrite, eventReadOwner, eventWriteOwner, err := manualRestartPipe(c)
	if err != nil {
		return child, err
	}
	child.controls, child.events = controlWrite, eventRead
	child.controlOwner, child.eventOwner = controlWriteOwner, eventReadOwner
	// The only executable is the exact verified running image. No inherited
	// PATH, GOFLAGS, testing flags, role authorization or environment is used.
	command := exec.Command(c.binding.BinaryPath, args...)
	command.Dir = c.root
	command.Env = []string{
		"GOSQLITE_MANUAL_FAULT_ROLLBACK=independently_authorized",
		"GOSQLITE_MANUAL_RESTART_ROLLBACK=independently_authorized",
		"GOSQLITE_RESTART_PRIVATE_ROOT_CONFIRMED=private_stable_namespace_retained",
		"GOSQLITE_RESTART_ROLE=" + fmt.Sprint(uint8(role)),
		"GOSQLITE_RESTART_ROOT=" + c.root,
		"GOSQLITE_RESTART_BINDING=" + filepath.Join(c.root, "inputs", "binding.input.json"),
		"GOSQLITE_RESTART_BINDING_SHA256=" + string(manualRestartDigest(c.rawBinding)),
		"GOSQLITE_RESTART_PLAN_SHA256=" + string(manualRestartDigest(rawPlan)),
		"GOSQLITE_RESTART_SEQUENCE=" + fmt.Sprint(count+1),
	}
	command.ExtraFiles = []*os.File{controlRead, eventWrite}
	command.Stdout, command.Stderr = &child.stdout, &child.stderr
	child.command = command
	c.active = child
	if err := command.Start(); err != nil {
		return child, fmt.Errorf("spawn did not establish an owned child: %w", err)
	}
	child.pid = command.Process.Pid
	// Start proves ownership of this process object; message PIDs never select
	// a process. The goroutine does exactly one actual Wait and retains state.
	go func() {
		waitErr := command.Wait()
		child.wait <- manualRestartWaitResult{state: command.ProcessState, err: waitErr}
	}()
	closeErr := errors.Join(c.sql.ledger.close(controlReadOwner), c.sql.ledger.close(eventWriteOwner))
	if closeErr != nil {
		return child, closeErr
	}
	if err := child.events.SetReadDeadline(time.Now().Add(manualRestartWait)); err != nil {
		return child, err
	}
	if err := child.controls.SetWriteDeadline(time.Now().Add(manualRestartWait)); err != nil {
		return child, err
	}
	kind := rollbackrestart.WriterSpawned
	if role == rollbackrestart.Recovery {
		kind = rollbackrestart.RecoverySpawned
	}
	frame := manualRestartParentFrame(c, receipt, kind)
	frame.Process = rollbackrestart.ProcessEvidence{PID: int64(child.pid), Handle: uint64(role - 1), Image: c.plan.Spec().Image, ImageHash: c.plan.Spec().Binding.Binary, SpawnObserved: true, State: rollbackrestart.ProcessRunning}
	if err := manualRestartAppend(c, receipt, frame); err != nil {
		return child, err
	}
	return child, nil
}

func manualRestartReadFrame(c *manualRestartConfig, child *manualRestartChild, receipt *rollbackrestart.Receipt) (rollbackrestart.Frame, error) {
	if child == nil || child.pid == 0 || child.waited || child.frameCount >= rollbackrestart.MaxFrames {
		return rollbackrestart.Frame{}, fmt.Errorf("owned live child and bounded protocol required")
	}
	var header [4]byte
	if n, err := io.ReadFull(child.events, header[:]); err != nil {
		if child.rawCount < rollbackrestart.MaxFrames {
			child.rawFrames[child.rawCount] = append([]byte(nil), header[:n]...)
			child.rawCount++
		}
		return rollbackrestart.Frame{}, err
	}
	count := binary.BigEndian.Uint32(header[:])
	if count == 0 || count > rollbackrestart.MaxFrameBytes {
		if child.rawCount < rollbackrestart.MaxFrames {
			child.rawFrames[child.rawCount] = append([]byte(nil), header[:]...)
			child.rawCount++
		}
		return rollbackrestart.Frame{}, fmt.Errorf("oversized/empty protocol frame")
	}
	raw := make([]byte, 4+int(count))
	copy(raw, header[:])
	n, readErr := io.ReadFull(child.events, raw[4:])
	if child.rawCount >= rollbackrestart.MaxFrames {
		return rollbackrestart.Frame{}, fmt.Errorf("raw frame limit")
	}
	child.rawFrames[child.rawCount] = append([]byte(nil), raw[:4+n]...)
	child.rawCount++
	if readErr != nil {
		return rollbackrestart.Frame{}, readErr
	}
	frame, problem := rollbackrestart.DecodeFrame(raw)
	if problem != rollbackrestart.ProblemNone || frame.Sender != child.identity || frame.Binding != c.plan.Spec().Binding || frame.Sequence != child.sequence+1 {
		return frame, fmt.Errorf("wrong, stale, duplicate or unbound child frame: %v", problem)
	}
	// Ready's PID is cross-checked against the actual object returned by Start.
	if (frame.Kind == rollbackrestart.ChildReadyBound || frame.Kind == rollbackrestart.RecoveryReadyBound) && frame.Process.PID != int64(child.command.Process.Pid) {
		return frame, fmt.Errorf("child-specified PID does not match parent process object")
	}
	child.frames[child.frameCount], child.frameCount = frame, child.frameCount+1
	child.sequence = frame.Sequence
	if err := manualRestartAppend(c, receipt, frame); err != nil {
		return frame, err
	}
	return frame, nil
}
func manualRestartSendFrame(c *manualRestartConfig, child *manualRestartChild, receipt *rollbackrestart.Receipt, frame rollbackrestart.Frame) error {
	if child == nil || child.pid == 0 || child.waited || frame.Sequence != child.sequence+1 {
		return fmt.Errorf("owned child grant ordering")
	}
	preview := *receipt
	if err := manualRestartAppend(c, &preview, frame); err != nil {
		return err
	}
	raw, problem := rollbackrestart.EncodeFrame(frame)
	if problem != rollbackrestart.ProblemNone {
		return fmt.Errorf("grant encoding: %v", problem)
	}
	child.lastGrant, child.grantBytes = frame, len(raw)
	count, err := child.controls.Write(raw)
	child.grantWritten = count
	if err == nil && count != len(raw) {
		err = io.ErrShortWrite
	}
	if err != nil {
		child.grantError = err.Error()
		return err
	}
	child.grantError = ""
	*receipt = preview
	child.sequence = frame.Sequence
	return nil
}

// A deadline is diagnostic only. The actual process object, channels and file
// owners remain strongly held here until the actual Wait returns. No signal,
// Kill, cleanup, retry, recovery or invented exit is used to end this wait.
func manualRestartAwait(t *testing.T, c *manualRestartConfig, child *manualRestartChild) manualRestartWaitResult {
	if child.waited {
		return child.actual
	}
	timer := time.NewTimer(manualRestartWait)
	select {
	case actual := <-child.wait:
		timer.Stop()
		child.waited, child.actual = true, actual
		return actual
	case <-timer.C:
		child.timedOut = true
		manualRestartPersistFailure(t, c, fmt.Errorf("CHILD_LIVE/WAIT_UNKNOWN: actual Wait exceeded observation deadline"))
		t.Logf("CHILD_LIVE/WAIT_UNKNOWN: retained process object, generation %d, pid %d; no signal, cleanup or recovery", child.identity.Generation, child.pid)
		actual := <-child.wait
		child.waited, child.actual = true, actual
		return actual
	}
}
func manualRestartWaitChild(t *testing.T, c *manualRestartConfig, child *manualRestartChild, receipt *rollbackrestart.Receipt) error {
	actual := manualRestartAwait(t, c, child)
	if child.timedOut {
		return fmt.Errorf("WAIT_UNKNOWN: observation deadline exceeded; actual eventual Wait retained without successful scenario")
	}
	if actual.state == nil || actual.state.Pid() != child.pid {
		return fmt.Errorf("WAIT_UNKNOWN: no actual matching process state: %v", actual.err)
	}
	code := 0
	if child.role == rollbackrestart.Writer && c.plan.Spec().Binding.Case != rollbackrestart.Control {
		code = rollbackrestart.ExpectedExitCode
	}
	if !actual.state.Exited() || actual.state.ExitCode() != code {
		return fmt.Errorf("CHILD_EXIT_UNEXPECTED: %v: %v", actual.state, actual.err)
	}
	if code == 0 && actual.err != nil {
		return fmt.Errorf("natural child exit lacked complete Wait: %w", actual.err)
	}
	if code == rollbackrestart.ExpectedExitCode {
		var exitError *exec.ExitError
		if !errors.As(actual.err, &exitError) || exitError.ProcessState != actual.state {
			return fmt.Errorf("expected exit lacks actual ExitError provenance")
		}
	}
	// With actual exit established, EOF must follow the exact closed protocol.
	// Any extra byte, duplicate terminal, truncation, or diagnostic overflow fails.
	if err := child.events.SetReadDeadline(time.Now().Add(manualRestartWait)); err != nil {
		return err
	}
	var trailing [1]byte
	n, tailErr := child.events.Read(trailing[:])
	if n != 0 && child.rawCount < rollbackrestart.MaxFrames {
		child.rawFrames[child.rawCount] = append([]byte(nil), trailing[:n]...)
		child.rawCount++
	}
	if n != 0 || tailErr != io.EOF {
		return fmt.Errorf("protocol trailing bytes or missing EOF: %v", tailErr)
	}
	stdout, outOverflow := child.stdout.snapshot()
	stderr, errOverflow := child.stderr.snapshot()
	if outOverflow || errOverflow {
		return fmt.Errorf("bounded diagnostic overflow; no successful terminal")
	}
	name := "writer"
	if child.role == rollbackrestart.Recovery {
		name = "recovery"
	}
	stdoutJSON, err := json.Marshal(struct {
		Raw      []byte
		Overflow bool
	}{stdout, outOverflow})
	if err != nil {
		return err
	}
	stderrJSON, err := json.Marshal(struct {
		Raw      []byte
		Overflow bool
	}{stderr, errOverflow})
	if err != nil {
		return err
	}
	if err := manualRestartCreate(c, 4, name+"-stdout.json", stdoutJSON); err != nil {
		return err
	}
	if err := manualRestartCreate(c, 4, name+"-stderr.json", stderrJSON); err != nil {
		return err
	}
	transcript, err := json.Marshal(child.frames[:child.frameCount])
	if err != nil {
		return err
	}
	if err := manualRestartCreate(c, 4, name+"-protocol.json", transcript); err != nil {
		return err
	}
	if err := errors.Join(c.sql.ledger.close(child.eventOwner), c.sql.ledger.close(child.controlOwner)); err != nil {
		return err
	}
	kind := rollbackrestart.WriterWaitObserved
	if child.role == rollbackrestart.Recovery {
		kind = rollbackrestart.RecoveryWaitObserved
	}
	frame := manualRestartParentFrame(c, receipt, kind)
	frame.Process = rollbackrestart.ProcessEvidence{PID: int64(child.pid), Handle: uint64(child.role - 1), Image: c.plan.Spec().Image, ImageHash: c.plan.Spec().Binding.Binary, WaitObserved: true, State: rollbackrestart.ProcessExited, ExitCode: code}
	return manualRestartAppend(c, receipt, frame)
}

func manualRestartAck(c *manualRestartConfig, commit rollbackrestart.Frame) (rollbackrestart.AckEvidence, error) {
	out := rollbackrestart.AckEvidence{}
	if c.role != rollbackrestart.Controller || c.plan.Spec().Binding.Case != rollbackrestart.AcknowledgedExit || commit.Kind != rollbackrestart.PublicCommitReturned || commit.Commit.Value != rollbackrestart.CommitNil || commit.Sender != c.plan.Spec().Writer {
		return out, fmt.Errorf("ACK_FAILED: exact public nil writer receipt required")
	}
	hash, problem := rollbackrestart.FrameDigest(commit)
	if problem != rollbackrestart.ProblemNone {
		return out, fmt.Errorf("ACK_FAILED: original frame unavailable")
	}
	raw, problem := rollbackrestart.EncodeFrame(commit)
	if problem != rollbackrestart.ProblemNone {
		return out, fmt.Errorf("ACK_FAILED: original frame encoding")
	}
	raw = raw[4:]
	// This is the unaltered canonical COMMIT_OK JSON body including every identity
	// and sequence, written exclusively then file-Sync/Close and dir-Sync/Close.
	if err := manualRestartCreate(c, 4, "ack.json", raw); err != nil {
		return out, fmt.Errorf("ACK_UNCERTAIN: %w", err)
	}
	stamp, ack, err := manualRestartArtifactAt(c, "control/ack.json", rollbackrestart.MaxFrameBytes+4)
	if err != nil || !stamp.Exists || !bytes.Equal(ack, raw) {
		return out, fmt.Errorf("ACK_UNCERTAIN: acknowledgement changed: %v", err)
	}
	out = rollbackrestart.AckEvidence{CommitSequence: commit.Sequence, CommitFrameSHA256: hash, SHA256: manualRestartDigest(raw), File: rollbackrestart.FileIdentity{Device: stamp.Device, Inode: stamp.Inode}, Bytes: stamp.Bytes, Written: int64(len(raw)), Exclusive: true, WriteStep: 1, FileSyncStep: 2, FileCloseStep: 3, DirectorySyncStep: 4, DirectoryCloseStep: 5, FileCloseCalls: 1, DirectoryCloseCalls: 1}
	if err := manualRestartRetain(c, "control/ack.json", rollbackrestart.MaxFrameBytes+4); err != nil {
		return out, err
	}
	c.durableAck = out
	return out, nil
}

func manualRestartFreshRegistry() bool {
	state := pager.ManualRollbackSQLState(nil)
	return state.Groups == 0 && state.Pending == 0 && !state.AdapterActive && !state.AdapterUsed
}
func manualRestartWriter(c *manualRestartConfig, channel *pager.ManualRollbackRestartChannel) error {
	if c.role != rollbackrestart.Writer || !manualRestartFreshRegistry() {
		return fmt.Errorf("fresh writer registry required")
	}
	ctx := context.Background()
	db, owner, err := manualRollbackSQLOpen(&c.sql, filepath.Join(c.root, "trial", "trial.db"), false)
	if err != nil {
		return err
	}
	tx, err := db.Begin(ctx, TxOptions{Immediate: true})
	if err != nil {
		return err
	}
	for _, statement := range []string{"UPDATE t SET k='green',v='ONE' WHERE id=1", "DELETE FROM t WHERE id=2", "INSERT INTO t(id,k,v) VALUES(4,'blue','four')"} {
		result, err := tx.Exec(ctx, statement)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil || count != 1 {
			return fmt.Errorf("DML affected %d: %v", count, err)
		}
	}
	if tx.activeRows != 0 || len(db.rows) != 0 {
		return fmt.Errorf("writer has live Rows")
	}
	scope, err := pager.ArmManualRollbackRestart(tx.storage, c.plan, channel)
	if err != nil {
		return err
	}
	commitErr := tx.Commit(ctx)
	captured := pager.CaptureManualRollbackRestartCommit(scope, commitErr, tx.done, db.tx == nil, tx.activeRows)
	if captured.CommitError != commitErr || captured.Problem != nil || commitErr != nil {
		return fmt.Errorf("COMMIT_UNCERTAIN: original=%v capture=%v", commitErr, captured.Problem)
	}
	if err := channel.CommitReturned(captured); err != nil {
		return err
	}
	// Acknowledged-exit cannot return from CommitReturned successfully. Only
	// natural control is allowed to Finish/Disarm and explicitly close its DB.
	if c.plan.Spec().Binding.Case != rollbackrestart.Control {
		return fmt.Errorf("target was not reached")
	}
	if err := pager.CompleteManualRollbackRestartControl(scope); err != nil {
		return err
	}
	if err := c.sql.ledger.close(owner); err != nil {
		return err
	}
	if !c.sql.ledger.retired() || !manualRollbackSQLRegistryEmpty() {
		return fmt.Errorf("OWNER_PENDING: normal writer control")
	}
	return channel.Finished("")
}

type manualRestartReadbackResult struct {
	Binding    rollbackrestart.Binding
	Generation uint64
	Snapshot   rollbackfault.SQLSnapshotEvidence
	Recovery   rollbackrestart.RecoveryEvidence
}

func manualRestartRecovery(c *manualRestartConfig, channel *pager.ManualRollbackRestartChannel) error {
	if c.role != rollbackrestart.Recovery || !manualRestartFreshRegistry() {
		return fmt.Errorf("fresh recovery registry required")
	}
	ctx := context.Background()
	result := manualRestartReadbackResult{Binding: c.plan.Spec().Binding, Generation: c.plan.Spec().Recovery.Generation}
	result.Recovery = rollbackrestart.RecoveryEvidence{Generation: result.Generation, ReadOnly: true, NoCreate: true, OpenCalls: 1, CheckMaxPages: 3, CheckMaxPageReads: 512, NoRetry: true}
	replayPath := filepath.Join(c.root, "replay", "replay.db")
	journalStamp, _, err := manualRestartArtifactAt(c, "replay/replay.db-journal", 16408)
	if err != nil {
		return err
	}
	if c.plan.Spec().Binding.Case == rollbackrestart.AfterTablePage {
		if !journalStamp.Exists || journalStamp.Bytes != 16408 {
			return fmt.Errorf("exact replay journal required")
		}
		result.Recovery.JournalPresentBefore = true
		result.Recovery.Calls = 1
		result.Recovery.Limits = rollbackfault.SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}
		recovered, recoveryErr := manualRollbackSQLRecoverCopy(ctx, replayPath)
		if recoveryErr != nil {
			return fmt.Errorf("RECOVERY_FAILED: %w", recoveryErr)
		}
		result.Recovery.Replayed, result.Recovery.ReplayedPages, result.Recovery.OriginalPages = recovered.Recovered, recovered.RestoredPages, recovered.OriginalPages
		absent, _, err := manualRestartArtifactAt(c, "replay/replay.db-journal", 16408)
		if err != nil || absent.Exists {
			return fmt.Errorf("replay journal survived recovery: %v", err)
		}
	} else if journalStamp.Exists {
		return fmt.Errorf("unexpected journal; no cleanup/recovery is allowed for this case")
	}
	identity := c.plan.Spec().AdapterIdentity
	snapshot, err := manualRollbackSQLReadback(&c.sql, ctx, replayPath, identity)
	if err != nil {
		return fmt.Errorf("READBACK_FAILED: %w", err)
	}
	if problem := rollbackfault.ValidateSQLSnapshot(snapshot, identity, rollbackrestart.ExpectedSQLState(c.plan.Spec().Binding.Case)); problem != rollbackfault.SQLProblemNone {
		return fmt.Errorf("complete SQL/index result differs from frozen expected state: %v", problem)
	}
	result.Snapshot = snapshot
	result.Recovery.OwnersRetired = c.sql.ledger.retired() && manualRollbackSQLRegistryEmpty()
	if !result.Recovery.OwnersRetired {
		return fmt.Errorf("OWNER_PENDING: recovery readback")
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err := manualRestartCreate(c, 4, "readback.json", raw); err != nil {
		return err
	}
	digest, problem := rollbackrestart.SnapshotDigest(snapshot)
	if problem != rollbackrestart.ProblemNone {
		return fmt.Errorf("snapshot digest refused: %v", problem)
	}
	return channel.Finished(digest)
}

func manualRestartArtifactEvidence(c *manualRestartConfig, evidence *rollbackrestart.ScenarioEvidence, after bool) error {
	for i := 0; i < int(rollbackrestart.ArtifactCount); i++ {
		role := rollbackrestart.ArtifactRole(i)
		artifact, raw, err := manualRestartArtifactAt(c, rollbackrestart.ArtifactName(role), 1<<20)
		if err != nil {
			return err
		}
		stamp := rollbackrestart.ArtifactStamp{}
		if artifact.Exists {
			stamp = rollbackrestart.ArtifactStamp{Path: artifact.Path, File: rollbackrestart.FileIdentity{Device: artifact.Device, Inode: artifact.Inode}, Exists: true, Regular: true, Links: 1, Bytes: artifact.Bytes, SHA256: rollbackrestart.Digest(artifact.Hash)}
			if role == rollbackrestart.BaselineArtifact || role == rollbackrestart.TrialArtifact || role == rollbackrestart.PostExitArtifact || role == rollbackrestart.ReplayArtifact {
				if len(raw) != 12288 {
					return fmt.Errorf("three-page artifact required")
				}
				for page := 0; page < 3; page++ {
					stamp.Pages[page] = manualRestartDigest(raw[page*4096 : (page+1)*4096])
				}
			}
		}
		entry := &evidence.Artifacts[i]
		entry.Role = role
		if after {
			entry.After = stamp
		} else {
			entry.Before = stamp
			if stamp.Exists && (role == rollbackrestart.PostExitArtifact || role == rollbackrestart.PostExitJournalArtifact || role == rollbackrestart.ReplayArtifact || role == rollbackrestart.ReplayJournalArtifact) {
				entry.ExclusiveCopy, entry.CompleteRead, entry.CompleteWrite = true, true, true
				entry.FileSynced, entry.FileClosedOnce, entry.DirectorySynced, entry.DirectoryClosedOnce = true, true, true, true
			}
		}
	}
	return nil
}
func manualRestartFreeze(c *manualRestartConfig, evidence *rollbackrestart.ScenarioEvidence) error {
	if c.active == nil || c.active.role != rollbackrestart.Writer || !c.active.waited || c.active.actual.state == nil || !c.active.actual.state.Exited() {
		return fmt.Errorf("actual writer Wait must precede all original reads/copies")
	}
	trial, raw, err := manualRestartArtifactAt(c, "trial/trial.db", 12288)
	if err != nil || !trial.Exists || len(raw) != 12288 {
		return fmt.Errorf("invalid retained trial: %v", err)
	}
	journal, journalRaw, err := manualRestartArtifactAt(c, "trial/trial.db-journal", 16408)
	if err != nil {
		return err
	}
	which := c.plan.Spec().Binding.Case
	if which == rollbackrestart.BeforeJCreate {
		if journal.Exists || !bytes.Equal(raw, c.baseline[:]) {
			return fmt.Errorf("pre-create exit must retain exact OLD and no journal")
		}
	} else if which == rollbackrestart.AfterTablePage {
		if !journal.Exists || len(journalRaw) != 16408 || bytes.Equal(raw[:4096], c.baseline[:4096]) || bytes.Equal(raw[4096:8192], c.baseline[4096:8192]) || !bytes.Equal(raw[8192:], c.baseline[8192:]) {
			return fmt.Errorf("exact two-written-page prefix and intact journal required")
		}
	} else if journal.Exists || bytes.Equal(raw, c.baseline[:]) {
		return fmt.Errorf("completed database bytes and absent journal required")
	}
	if err := manualRestartRetain(c, "trial/trial.db", 12288); err != nil {
		return err
	}
	if err := manualRestartRetain(c, "trial/trial.db-journal", 16408); err != nil {
		return err
	}
	if err := manualRestartCreate(c, 2, "post-exit.input.db", raw); err != nil {
		return err
	}
	if err := manualRestartCreate(c, 3, "replay.db", raw); err != nil {
		return err
	}
	if err := manualRestartRetain(c, "evidence/post-exit.input.db", 12288); err != nil {
		return err
	}
	if journal.Exists {
		if err := manualRestartCreate(c, 2, "post-exit-journal.input.bin", journalRaw); err != nil {
			return err
		}
		if err := manualRestartCreate(c, 3, "replay.db-journal", journalRaw); err != nil {
			return err
		}
	}
	if err := manualRestartRetain(c, "evidence/post-exit-journal.input.bin", 16408); err != nil {
		return err
	}
	if err := manualRestartArtifactEvidence(c, evidence, false); err != nil {
		return err
	}
	// Distinct inodes are checked before permitting the recovery process,
	// including the mutable replay copies that are not in the frozen ledger.
	for i, a := range evidence.Artifacts {
		if !a.Before.Exists {
			continue
		}
		for j := 0; j < i; j++ {
			if evidence.Artifacts[j].Before.Exists && a.Before.File == evidence.Artifacts[j].Before.File {
				return fmt.Errorf("copy aliased an original or another artifact")
			}
		}
	}
	return manualRestartUnchanged(c)
}

func manualRestartDriveWriter(t *testing.T, c *manualRestartConfig, receipt *rollbackrestart.Receipt) error {
	child, err := manualRestartSpawn(c, receipt, rollbackrestart.Writer)
	if err != nil {
		return err
	}
	ready, err := manualRestartReadFrame(c, child, receipt)
	if err != nil || ready.Kind != rollbackrestart.ChildReadyBound {
		return fmt.Errorf("writer ready failed: %v", err)
	}
	arm := manualRestartParentFrame(c, receipt, rollbackrestart.ArmGranted)
	if err := manualRestartSendFrame(c, child, receipt, arm); err != nil {
		return err
	}
	for {
		frame, err := manualRestartReadFrame(c, child, receipt)
		if err != nil {
			return err
		}
		switch frame.Kind {
		case rollbackrestart.TransactionArmed, rollbackrestart.AdapterObserved:
			continue
		case rollbackrestart.TargetReached:
			target, problem := rollbackrestart.FrameDigest(frame)
			if problem != rollbackrestart.ProblemNone {
				return fmt.Errorf("target digest refused")
			}
			exit := manualRestartParentFrame(c, receipt, rollbackrestart.ExitGranted)
			exit.Digest = target
			if err := manualRestartSendFrame(c, child, receipt, exit); err != nil {
				return err
			}
			return manualRestartWaitChild(t, c, child, receipt)
		case rollbackrestart.PublicCommitReturned:
			if c.plan.Spec().Binding.Case == rollbackrestart.AcknowledgedExit {
				ack, err := manualRestartAck(c, frame)
				if err != nil {
					return err
				}
				durable := manualRestartParentFrame(c, receipt, rollbackrestart.AcknowledgementDurable)
				durable.Ack = ack
				if err := manualRestartSendFrame(c, child, receipt, durable); err != nil {
					return err
				}
				exit := manualRestartParentFrame(c, receipt, rollbackrestart.ExitGranted)
				exit.Ack, exit.Digest = ack, ack.SHA256
				if err := manualRestartSendFrame(c, child, receipt, exit); err != nil {
					return err
				}
				return manualRestartWaitChild(t, c, child, receipt)
			}
			if c.plan.Spec().Binding.Case != rollbackrestart.Control {
				return fmt.Errorf("unacknowledged target was not reached")
			}
			grant := manualRestartParentFrame(c, receipt, rollbackrestart.NormalFinishGranted)
			grant.Digest, _ = rollbackrestart.FrameDigest(frame)
			if err := manualRestartSendFrame(c, child, receipt, grant); err != nil {
				return err
			}
		case rollbackrestart.WriterFinished:
			return manualRestartWaitChild(t, c, child, receipt)
		default:
			return fmt.Errorf("closed writer protocol rejected kind %d", frame.Kind)
		}
	}
}
func manualRestartDriveRecovery(t *testing.T, c *manualRestartConfig, receipt *rollbackrestart.Receipt, evidence *rollbackrestart.ScenarioEvidence) error {
	child, err := manualRestartSpawn(c, receipt, rollbackrestart.Recovery)
	if err != nil {
		return err
	}
	ready, err := manualRestartReadFrame(c, child, receipt)
	if err != nil || ready.Kind != rollbackrestart.RecoveryReadyBound {
		return fmt.Errorf("recovery ready failed: %v", err)
	}
	grant := manualRestartParentFrame(c, receipt, rollbackrestart.RecoveryReadbackGranted)
	if err := manualRestartSendFrame(c, child, receipt, grant); err != nil {
		return err
	}
	result, err := manualRestartReadFrame(c, child, receipt)
	if err != nil || result.Kind != rollbackrestart.ReadbackResult {
		return fmt.Errorf("readback terminal missing: %v", err)
	}
	if err := manualRestartWaitChild(t, c, child, receipt); err != nil {
		return err
	}
	raw, _, err := manualRestartRead(c, filepath.Join(c.root, "control", "readback.json"), 65536)
	if err != nil {
		return err
	}
	var readback manualRestartReadbackResult
	if err := manualRestartDecode(raw, &readback); err != nil {
		return err
	}
	if readback.Binding != c.plan.Spec().Binding || readback.Generation != c.plan.Spec().Recovery.Generation {
		return fmt.Errorf("stale/wrong readback binding")
	}
	digest, problem := rollbackrestart.SnapshotDigest(readback.Snapshot)
	if problem != rollbackrestart.ProblemNone || digest != result.Digest {
		return fmt.Errorf("readback artifact differs from child's exact result")
	}
	evidence.After, evidence.Recovery = readback.Snapshot, readback.Recovery
	if err := manualRestartRetain(c, "control/readback.json", 65536); err != nil {
		return err
	}
	return manualRestartUnchanged(c)
}

func manualRestartController(t *testing.T, c *manualRestartConfig) error {
	if c.role != rollbackrestart.Controller || !manualRollbackSQLRegistryEmpty() || !c.sql.ledger.retired() {
		return fmt.Errorf("parent must have no existing database owners")
	}
	if err := manualRestartFamilies(c, true); err != nil {
		return err
	}
	if err := manualRestartCreate(c, 0, "binding.input.json", c.rawBinding); err != nil {
		return err
	}
	planRaw, err := json.Marshal(c.plan.Spec())
	if err != nil {
		return err
	}
	if err := manualRestartCreate(c, 4, "plan.json", planRaw); err != nil {
		return err
	}
	if err := manualRestartRetain(c, "inputs/binding.input.json", 65536); err != nil {
		return err
	}
	if err := manualRestartRetain(c, "control/plan.json", 65536); err != nil {
		return err
	}
	if err := manualRestartCreate(c, 0, "baseline.input.db", c.baseline[:]); err != nil {
		return err
	}
	if err := manualRestartRetain(c, "inputs/baseline.input.db", 12288); err != nil {
		return err
	}
	before, err := manualRollbackSQLReadback(&c.sql, context.Background(), filepath.Join(c.root, "inputs", "baseline.input.db"), c.plan.Spec().AdapterIdentity)
	if err != nil {
		return err
	}
	if problem := rollbackfault.ValidateSQLSnapshot(before, c.plan.Spec().AdapterIdentity, rollbackfault.SQLOld); problem != rollbackfault.SQLProblemNone {
		return fmt.Errorf("baseline SQL/index not literal OLD: %v", problem)
	}
	beforeRaw, err := json.Marshal(before)
	if err != nil {
		return err
	}
	if err := manualRestartCreate(c, 4, "baseline-sql.json", beforeRaw); err != nil {
		return err
	}
	if err := manualRestartRetain(c, "control/baseline-sql.json", 65536); err != nil {
		return err
	}
	if err := manualRestartCreate(c, 1, "trial.db", c.baseline[:]); err != nil {
		return err
	}
	receipt, problem := rollbackrestart.NewReceipt(c.plan)
	c.receipt = &receipt
	if problem != rollbackrestart.ProblemNone {
		return fmt.Errorf("receipt admission: %v", problem)
	}
	parent := manualRestartParentFrame(c, &receipt, rollbackrestart.ParentAdmitted)
	parent.Process = rollbackrestart.ProcessEvidence{PID: int64(os.Getpid()), Image: c.plan.Spec().Image, ImageHash: c.plan.Spec().Binding.Binary, ActualImage: true, State: rollbackrestart.ProcessRunning}
	if err := manualRestartAppend(c, &receipt, parent); err != nil {
		return err
	}
	baseline := manualRestartParentFrame(c, &receipt, rollbackrestart.BaselineCheckedAndClosed)
	baseline.Digest, problem = rollbackrestart.SnapshotDigest(before)
	if problem != rollbackrestart.ProblemNone {
		return fmt.Errorf("baseline snapshot digest")
	}
	if err := manualRestartAppend(c, &receipt, baseline); err != nil {
		return err
	}
	evidence := rollbackrestart.ScenarioEvidence{Binding: c.plan.Spec().Binding, Before: before, BeforeGeneration: c.plan.Spec().Controller.Generation, AfterGeneration: c.plan.Spec().Recovery.Generation}
	c.evidence = &evidence
	if err := manualRestartDriveWriter(t, c, &receipt); err != nil {
		return err
	}
	if err := manualRestartFreeze(c, &evidence); err != nil {
		return err
	}
	frozen := manualRestartParentFrame(c, &receipt, rollbackrestart.OriginalsFrozen)
	frozen.Digest, problem = rollbackrestart.ArtifactManifestDigest(evidence.Artifacts, false)
	if problem != rollbackrestart.ProblemNone {
		return fmt.Errorf("original artifact manifest refused")
	}
	if err := manualRestartAppend(c, &receipt, frozen); err != nil {
		return err
	}
	if err := manualRestartDriveRecovery(t, c, &receipt, &evidence); err != nil {
		return err
	}
	if err := manualRestartArtifactEvidence(c, &evidence, true); err != nil {
		return err
	}
	checked := manualRestartParentFrame(c, &receipt, rollbackrestart.ArtifactsRechecked)
	checked.Digest, problem = rollbackrestart.ArtifactManifestDigest(evidence.Artifacts, true)
	if problem != rollbackrestart.ProblemNone {
		return fmt.Errorf("post-readback artifact manifest refused")
	}
	if err := manualRestartAppend(c, &receipt, checked); err != nil {
		return err
	}
	complete := manualRestartParentFrame(c, &receipt, rollbackrestart.ScenarioEvidenceComplete)
	if err := manualRestartAppend(c, &receipt, complete); err != nil {
		return err
	}
	evidence.Lifecycle = rollbackrestart.WriterLifecycle{OpenCalls: 1, BeginCalls: 1, DMLCalls: 3, ArmCalls: 1, CommitCalls: 1, Main: rollbackrestart.AbandonedByExpectedProcessExit, Journal: rollbackrestart.AbandonedByExpectedProcessExit, Cleanup: rollbackrestart.AbandonedByExpectedProcessExit}
	switch c.plan.Spec().Binding.Case {
	case rollbackrestart.BeforeJCreate:
		evidence.Lifecycle.Journal = rollbackrestart.NotAcquired
	case rollbackrestart.AfterJUnlink:
		evidence.Lifecycle.Journal = rollbackrestart.RetiredNormally
	case rollbackrestart.Control, rollbackrestart.AcknowledgedExit:
		evidence.Lifecycle.PublicReturned = true
		evidence.Lifecycle.Main, evidence.Lifecycle.Journal, evidence.Lifecycle.Cleanup = rollbackrestart.RetiredNormally, rollbackrestart.RetiredNormally, rollbackrestart.RetiredNormally
		if c.plan.Spec().Binding.Case == rollbackrestart.Control {
			evidence.Lifecycle.FinishCalls, evidence.Lifecycle.DisarmCalls, evidence.Lifecycle.DBCloseCalls = 1, 1, 1
		}
	}
	if !c.sql.ledger.retired() || !manualRollbackSQLRegistryEmpty() {
		return fmt.Errorf("parent owner ledger incomplete")
	}
	for i := range c.family {
		if !c.family[i].ledger.retired() {
			return fmt.Errorf("artifact owner ledger incomplete")
		}
	}
	evidence.Runtime = rollbackrestart.RuntimeEvidence{EnvFirst: true, ExactRoleSelectors: true, FixedAllowlistEnvironment: true, CountOne: true, ParallelOne: true, CPUOne: true, ShuffleOff: true, TimeoutZero: true, PrivateRoot0700: true, NoSymlinkAncestors: true, RootUnchanged: true, TwoDedicatedPipes: true, PipeOwnersRecorded: true, UnusedPipeEndsClosedOnce: true, BoundedDiagnostics: true, NoInheritedDatabaseOwners: true, ParentOwnersClosedBeforeSpawn: true, FreshRegistryEachChild: true, OnlyParentSpawns: true, NoSignalOrKill: true, NoRetries: true, OriginalsNeverSyncedAfterExit: true, ArtifactsRetained: true, AdapterSameTransaction: true, AdapterSameScope: true, AdapterSameOwners: true, ObserverOutsidePrivateMutex: true, OriginalTxLockHeldAtCut: true, NoCallbackNativeReplacement: true}
	observation := rollbackrestart.Observe(c.plan, receipt, evidence)
	if !observation.EvidenceConsistent || !observation.ScenarioObserved || observation.PhysicalValidated() || observation.Pass() {
		return fmt.Errorf("scenario evidence rejected: %+v", observation)
	}
	frames, count := receipt.Frames()
	output := struct {
		Status      string
		Plan        rollbackrestart.PlanSpec
		Frames      []rollbackrestart.Frame
		Evidence    rollbackrestart.ScenarioEvidence
		Observation rollbackrestart.Observation
	}{"SCENARIO_EVIDENCE_COMPLETE_NOT_POWERLOSS_ACCEPTANCE", c.plan.Spec(), frames[:count], evidence, observation}
	raw, err := json.Marshal(output)
	if err != nil {
		return err
	}
	if err := manualRestartCreate(c, 4, "result.json", raw); err != nil {
		return err
	}
	if err := manualRestartUnchanged(c); err != nil {
		return err
	}
	// The final manifest includes every retained input and control artifact,
	// including result.json. Its own separately measured identity is held and
	// rechecked in memory; a manifest cannot recursively contain its own hash.
	finalManifest, err := json.Marshal(c.retained[:c.retainedCount])
	if err != nil {
		return err
	}
	if err := manualRestartCreate(c, 4, "final-manifest.json", finalManifest); err != nil {
		return err
	}
	return manualRestartUnchanged(c)
}

// Failure receipts never overwrite success, erase originals or cause a retry.
// A protocol abort permission is a failure-only frame, not a process signal.
// If it cannot be sent or the child stays in native I/O, retain the actual
// handle and wait; no recovery may begin on this path.
type manualRestartFailureRecord struct {
	Status          string
	Error           string
	Plan            rollbackrestart.PlanSpec
	RecordedFrames  []rollbackrestart.Frame
	ProtocolProblem rollbackrestart.Problem
	LastGrant       rollbackrestart.Frame
	GrantBytes      int
	GrantWritten    int
	GrantError      string
	RawChildPrefix  [][]byte
	Commit          rollbackrestart.CommitValue
	DurableAck      rollbackrestart.AckEvidence
	SQLState        rollbackfault.SQLState
	Generation      uint64
	PID             int
	WaitObserved    bool
	ExitCode        int
}

func manualRestartFailureSnapshot(c *manualRestartConfig, cause error) manualRestartFailureRecord {
	child := c.active
	detail := manualRestartFailureRecord{Status: "INCOMPLETE_RETAINED", Error: fmt.Sprint(cause), Plan: c.plan.Spec(), ExitCode: -1}
	if len(detail.Error) > 4096 {
		detail.Error = detail.Error[:4096]
	}
	if c.receipt != nil {
		frames, count := c.receipt.Frames()
		detail.RecordedFrames = append([]rollbackrestart.Frame(nil), frames[:count]...)
		frozenReceipt := *c.receipt
		observation := frozenReceipt.Seal()
		detail.Commit, detail.ProtocolProblem = observation.Commit, observation.Problem
	}
	detail.DurableAck = c.durableAck
	if c.evidence != nil && c.evidence.After.Owner != 0 && rollbackfault.ValidateSQLSnapshot(c.evidence.After, c.plan.Spec().AdapterIdentity, rollbackrestart.ExpectedSQLState(c.plan.Spec().Binding.Case)) == rollbackfault.SQLProblemNone {
		detail.SQLState = rollbackrestart.ExpectedSQLState(c.plan.Spec().Binding.Case)
	}
	if child != nil && child.pid != 0 {
		detail.Generation, detail.PID, detail.WaitObserved = child.identity.Generation, child.pid, child.waited && child.actual.state != nil
		detail.LastGrant, detail.GrantBytes, detail.GrantWritten, detail.GrantError = child.lastGrant, child.grantBytes, child.grantWritten, child.grantError
		detail.RawChildPrefix = append([][]byte(nil), child.rawFrames[:child.rawCount]...)
		if !child.waited {
			detail.Status = "CHILD_LIVE_WAIT_UNKNOWN_RETAINED"
		} else if child.actual.state != nil {
			detail.ExitCode = child.actual.state.ExitCode()
		}
	}
	return detail
}

// Shared by explicit failure and the normal-Wait timeout path. It never waits,
// retries, signals, closes a DB, or overwrites an earlier failed receipt.
func manualRestartPersistFailure(t *testing.T, c *manualRestartConfig, cause error) {
	if c.failureAttempted {
		return
	}
	c.failureAttempted = true
	detail := manualRestartFailureSnapshot(c, cause)
	if c.family[4].root != "" {
		raw, err := json.Marshal(detail)
		if err == nil {
			err = manualRestartCreate(c, 4, "failure.json", raw)
		}
		if err != nil {
			t.Logf("failure evidence write incomplete: %v; originals retained", err)
		}
	}
}

func manualRestartFailure(t *testing.T, c *manualRestartConfig, cause error) {
	child := c.active
	manualRestartPersistFailure(t, c, cause)
	detail := manualRestartFailureSnapshot(c, cause)
	if child == nil || child.pid == 0 {
		return
	}
	if !child.waited {
		failure := rollbackrestart.Frame{Schema: rollbackrestart.SchemaVersion, Binding: c.plan.Spec().Binding, Sender: c.plan.Spec().Controller, Sequence: child.sequence + 1, Kind: rollbackrestart.Failure, Diagnostic: rollbackrestart.ProtocolInvalid}
		raw, problem := rollbackrestart.EncodeFrame(failure)
		if problem == rollbackrestart.ProblemNone {
			n, err := child.controls.Write(raw)
			if err != nil || n != len(raw) {
				t.Logf("failure-only control send incomplete: %v; keeping actual child", err)
			}
		}
		actual := manualRestartAwait(t, c, child)
		detail.WaitObserved = actual.state != nil
		if actual.state != nil {
			detail.ExitCode = actual.state.ExitCode()
		}
	}
	// The eventual real Wait gets a separate file; it cannot overwrite or
	// promote the original failure to successful scenario evidence.
	detail.Status = "FAILED_ACTUAL_WAIT_RETAINED"
	raw, err := json.Marshal(detail)
	if err == nil {
		err = manualRestartCreate(c, 4, "failure-wait.json", raw)
	}
	if err != nil {
		t.Logf("actual Wait evidence write incomplete: %v", err)
	}
	stdout, outOverflow := child.stdout.snapshot()
	stderr, errOverflow := child.stderr.snapshot()
	stdoutJSON, outErr := json.Marshal(struct {
		Raw      []byte
		Overflow bool
	}{stdout, outOverflow})
	stderrJSON, errErr := json.Marshal(struct {
		Raw      []byte
		Overflow bool
	}{stderr, errOverflow})
	if outErr == nil {
		outErr = manualRestartCreate(c, 4, "failure-stdout.json", stdoutJSON)
	}
	if errErr == nil {
		errErr = manualRestartCreate(c, 4, "failure-stderr.json", stderrJSON)
	}
	if err := errors.Join(outErr, errErr); err != nil {
		t.Logf("diagnostic retention: %v", err)
	}
	raw, err = json.Marshal(child.rawFrames[:child.rawCount])
	if err == nil {
		err = manualRestartCreate(c, 4, "failure-protocol.json", raw)
	}
	if err != nil {
		t.Logf("raw prefix retention: %v", err)
	}
}

func TestManualRollbackRestartController(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" || os.Getenv("GOSQLITE_MANUAL_RESTART_ROLLBACK") != "independently_authorized" {
		t.Fatal("NOT_RUN: separate manual restart authorization required")
	}
	var config manualRestartConfig
	if err := manualRestartAdmit(&config, rollbackrestart.Controller); err != nil {
		t.Fatalf("ADMISSION_BLOCKED: %v", err)
	}
	if err := manualRestartController(t, &config); err != nil {
		manualRestartFailure(t, &config, err)
		t.Fatalf("INCOMPLETE: %v; all artifacts retained", err)
	}
	t.Log("SCENARIO_EVIDENCE_COMPLETE: process-exit scenario only; PhysicalValidated=false; Pass=false; all artifacts retained")
}
func TestManualRollbackRestartWriter(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" || os.Getenv("GOSQLITE_MANUAL_RESTART_ROLLBACK") != "independently_authorized" {
		t.Fatal("NOT_RUN: separate manual restart authorization required")
	}
	var config manualRestartConfig
	if err := manualRestartAdmit(&config, rollbackrestart.Writer); err != nil {
		t.Fatalf("ADMISSION_BLOCKED: %v", err)
	}
	if err := manualRestartFamilies(&config, false); err != nil {
		t.Fatal(err)
	}
	channel, err := pager.NewManualRollbackRestartChannel(config.plan, rollbackrestart.Writer)
	if err != nil {
		t.Fatal(err)
	}
	image, err := manualRestartID(config.selfInfo)
	if err != nil {
		t.Fatal(err)
	}
	if err := channel.Ready(image, config.plan.Spec().Binding.Binary); err != nil {
		t.Fatal(err)
	}
	if err := manualRestartWriter(&config, channel); err != nil {
		t.Fatalf("INCOMPLETE: %v; no cleanup retry", err)
	}
}
func TestManualRollbackRestartRecovery(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" || os.Getenv("GOSQLITE_MANUAL_RESTART_ROLLBACK") != "independently_authorized" {
		t.Fatal("NOT_RUN: separate manual restart authorization required")
	}
	var config manualRestartConfig
	if err := manualRestartAdmit(&config, rollbackrestart.Recovery); err != nil {
		t.Fatalf("ADMISSION_BLOCKED: %v", err)
	}
	if err := manualRestartFamilies(&config, false); err != nil {
		t.Fatal(err)
	}
	channel, err := pager.NewManualRollbackRestartChannel(config.plan, rollbackrestart.Recovery)
	if err != nil {
		t.Fatal(err)
	}
	image, err := manualRestartID(config.selfInfo)
	if err != nil {
		t.Fatal(err)
	}
	if err := channel.Ready(image, config.plan.Spec().Binding.Binary); err != nil {
		t.Fatal(err)
	}
	if err := manualRestartRecovery(&config, channel); err != nil {
		t.Fatalf("INCOMPLETE: %v; no recovery retry", err)
	}
}
