//go:build linux && manual_format_upgrade

package sqlite_test

// SOURCE_ONLY / REVIEW_HOLD / NATIVE_NOT_RUN. No production changes.
// The environment gate is an acknowledgement, not independent authorization.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/formatupgrade"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type manualFormatOwner struct {
	state formatupgrade.Owner
	db    *sqlite.DB
	rows  *sqlite.Rows
	file  *os.File
}
type manualFormatRun struct {
	t         *testing.T
	receipt   formatupgrade.Receipt
	owners    []*manualFormatOwner
	providers []*manualFormatProvider
	causes    []error
	events    int
	page      uint32
}

// Retain unknown owners and error tokens for the remainder of this test process.
// There is deliberately no finalizer, retry, global sweep or filesystem cleanup.
var manualFormatRetained []*manualFormatRun

type manualFormatProvider struct {
	mu                 sync.Mutex
	label              string
	page               uint32
	acquires, releases int64
	duplicate          bool
}
type manualFormatMaterial struct {
	provider *manualFormatProvider
	key      []byte
	released bool
}

func (p *manualFormatProvider) Acquire(context.Context, encryption.KeyRequest) (encryption.KeyMaterial, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.acquires++
	// Public synthetic fixture material, never suitable for user databases.
	key := sha256.Sum256([]byte("public-format-upgrade-fixture-v1/" + p.label))
	return &manualFormatMaterial{provider: p, key: append([]byte(nil), key[:]...)}, nil
}
func (m *manualFormatMaterial) Kind() encryption.CredentialKind { return encryption.RawKey256 }
func (m *manualFormatMaterial) Bytes() []byte                   { return m.key }
func (m *manualFormatMaterial) Release() {
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

func manualFormatError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
func (r *manualFormatRun) event(label string, value any, err error) {
	r.capturePending(err)
	r.events++
	if r.events > 65536 {
		r.t.Fatal("event bound exceeded; all artifacts retained")
	}
	b, marshalErr := json.Marshal(struct {
		Label string
		Value any
		Error string
	}{label, value, manualFormatError(err)})
	if marshalErr != nil || len(b) > 1<<20 {
		r.t.Fatal("receipt encoding failed; artifacts retained")
	}
	r.t.Logf("FORMAT_UPGRADE_EVENT %s", b)
}
func (r *manualFormatRun) capture(label string, db *sqlite.DB, rows *sqlite.Rows, file *os.File, parent int, err error) *manualFormatOwner {
	o := &manualFormatOwner{db: db, rows: rows, file: file}
	o.state = formatupgrade.Owner{ID: len(r.owners) + 1, Page: r.page, Label: label, Parent: parent, AcquireError: manualFormatError(err), Terminal: "absent"}
	if db != nil {
		o.state.Kind = "db"
		o.state.Present = true
	}
	if rows != nil {
		o.state.Kind = "rows"
		o.state.Present = true
	}
	if file != nil {
		o.state.Kind = "file"
		o.state.Present = true
	}
	if o.state.Kind == "" {
		o.state.Kind = "db"
	}
	if strings.HasPrefix(label, "file:") {
		o.state.Kind = "file"
	}
	if strings.HasPrefix(label, "rows:") {
		o.state.Kind = "rows"
	}
	if o.state.Present {
		o.state.Terminal = "live"
	}
	r.owners = append(r.owners, o)
	r.receipt.Acquisitions = append(r.receipt.Acquisitions, formatupgrade.AcquisitionOf(o.state))
	r.event("acquire", o.state, err)
	if len(r.owners) > formatupgrade.MaxOwners {
		r.t.Fatal("owner bound exceeded")
	}
	return o
}
func (r *manualFormatRun) states() []formatupgrade.Owner {
	states := []formatupgrade.Owner{}
	for _, o := range r.owners {
		states = append(states, o.state)
	}
	return states
}
func (r *manualFormatRun) capturePending(cause error) {
	// Capture the original result and every public token before logging or Fatal.
	// Public tokens do not identify one DB, so preserve all possibly affected
	// live DB/Rows owners. Never call a parent Close to discover who owns a token.
	r.causes = append(r.causes, cause)
	pending := sqlite.PendingEncryptedCleanups()
	r.causes = append(r.causes, pending...)
	if len(pending) == 0 && !errors.Is(cause, sqlite.ErrEncryptedCleanupPending) {
		return
	}
	r.receipt.InternalPendingObserved = true
	for _, o := range r.owners {
		if o.state.Present && o.state.Terminal == "live" && (o.state.Kind == "db" || o.state.Kind == "rows") {
			o.state.Blocked = "internal-encrypted-pending"
		}
	}
}
func (r *manualFormatRun) retire(o *manualFormatOwner) error {
	if err := formatupgrade.CanRetire(o.state, r.states(), r.receipt.InternalPendingObserved); err != nil {
		o.state.Blocked = err.Error()
		return err
	}
	var err error
	if o.db != nil {
		err = o.db.Close()
	} else if o.rows != nil {
		err = o.rows.Close()
	} else if o.file != nil {
		err = o.file.Close()
	} else {
		return fmt.Errorf("missing owner")
	}
	stateErr := o.state.Retire("close", manualFormatError(err))
	r.event("retire", o.state, err)
	return errors.Join(err, stateErr)
}
func (r *manualFormatRun) require(ok bool, label string, err error) {
	r.event(label, ok, err)
	if !ok || err != nil {
		r.t.Fatalf("%s failed; every artifact retained", label)
	}
	if r.receipt.InternalPendingObserved {
		r.t.Fatal("encrypted cleanup pending; parent DB/Rows ownership preserved without retry")
	}
}
func (r *manualFormatRun) finish() {
	r.capturePending(nil)
	// Reverse acquisition order; unknown retirement is never retried. A parent
	// whose child is unknown remains visibly live, rather than hiding a retry.
	for i := len(r.owners) - 1; i >= 0; i-- {
		o := r.owners[i]
		if o.state.Present && o.state.Terminal == "live" && o.state.Attempts == 0 {
			if o.state.Blocked != "" {
				r.t.Errorf("blocked owner %d retained: %s", o.state.ID, o.state.Blocked)
				continue
			}
			if err := r.retire(o); err != nil {
				r.t.Errorf("known-owner retirement: %v", err)
			}
		}
	}
	for _, o := range r.owners {
		r.receipt.Owners = append(r.receipt.Owners, o.state)
	}
	pending := sqlite.PendingEncryptedCleanups()
	r.causes = append(r.causes, pending...)
	r.receipt.Pending = len(pending)
	for _, p := range r.providers {
		p.mu.Lock()
		r.receipt.ProviderAcquires += p.acquires
		r.receipt.ProviderReleases += p.releases
		r.receipt.ProviderDuplicateRelease = r.receipt.ProviderDuplicateRelease || p.duplicate
		r.receipt.Providers = append(r.receipt.Providers, formatupgrade.Provider{Page: p.page, Label: p.label, Acquires: p.acquires, Releases: p.releases, Duplicate: p.duplicate})
		p.mu.Unlock()
	}
	if r.t.Failed() {
		r.receipt.Status = "failed"
	}
	err := r.receipt.Validate()
	if err != nil {
		r.receipt.Status = "failed"
		r.t.Errorf("terminal receipt refused: %v", err)
	}
	r.event("terminal-receipt", r.receipt, err)
	if err == nil && !r.t.Failed() {
		r.t.Log("FORMAT_UPGRADE_COMPLETE cases=3 preserved=true owners=retired")
	}
}
func (r *manualFormatRun) options(label string, mode sqlite.EncryptionMode) sqlite.EncryptionOptions {
	p := &manualFormatProvider{label: label, page: r.page}
	r.providers = append(r.providers, p)
	r.event("provider-label", label, nil)
	return sqlite.EncryptionOptions{KeyProvider: p, CredentialKind: encryption.RawKey256, Mode: mode, MaxGenerations: 64, MaxRetainedBytes: 64 << 20, MaxDirectoryEntries: 256}
}
func manualFormatHead(i sqlite.EncryptionInfo) formatupgrade.Head {
	return formatupgrade.Head{Mode: uint8(i.Mode), Layout: i.StorageLayout, PageSize: i.PageSize, PageCount: i.PageCount, Segments: i.WALSegments, DBID: hex.EncodeToString(i.DatabaseID[:]), Sequence: i.CommitSequence}
}

func TestManualEncryptedSnapshotToWALUpgrade(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_FORMAT_UPGRADE") != "independently_authorized" {
		t.Fatal("manual format upgrade is not independently authorized")
	}
	r := &manualFormatRun{t: t, receipt: formatupgrade.Receipt{Profile: formatupgrade.Profile, Status: "started"}}
	manualFormatRetained = append(manualFormatRetained, r)
	defer r.finish()
	r.preflight()
	for _, page := range []uint32{512, 4096, 65536} {
		// No t.Run: a failure stops the remaining cases. Exact selector cannot
		// select a subset and still emit the complete three-case receipt.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		func() { defer cancel(); r.runCase(ctx, page) }()
	}
	r.require(r.receipt.KnownRoot == r.directory(r.receipt.KnownRoot.Path), "known-root-final-identity", nil)
	r.require(r.receipt.RunRoot == r.directory(r.receipt.RunRoot.Path), "run-root-final-identity", nil)
	r.receipt.Status = "complete"
}

func (r *manualFormatRun) preflight() {
	r.require(os.Geteuid() > 0, "nonprivileged-owner", nil)
	r.require(runtime.GOOS == "linux" && (runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64") && runtime.Version() == "go1.27.2", "linux-toolchain", nil)
	r.require(os.Getenv("GOFLAGS") == "" && os.Getenv("GOSQLITE_MANUAL_FORMAT_UPGRADE_NAMESPACE") == "private-stable-no-alias-no-other-process", "stable-namespace-declaration", nil)
	wanted := map[string]string{"test.run": "^TestManualEncryptedSnapshotToWALUpgrade$", "test.count": "1", "test.timeout": "0s", "test.parallel": "1", "test.shuffle": "off", "test.v": "true"}
	for name, want := range wanted {
		f := flag.Lookup(name)
		r.require(f != nil && f.Value.String() == want, "flag-"+name, nil)
	}
	flag.Visit(func(f *flag.Flag) { _, allowed := wanted[f.Name]; r.require(allowed, "only-reviewed-flags", nil) })
	info, ok := debug.ReadBuildInfo()
	r.require(ok && info.GoVersion == "go1.27.2", "build-info", nil)
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	r.require(settings["CGO_ENABLED"] == "0" && settings["GOOS"] == "linux" && settings["GOARCH"] == runtime.GOARCH && settings["-tags"] == "manual_format_upgrade", "build-profile", nil)
	versions := map[string]string{"golang.org/x/crypto": "v0.58.0", "golang.org/x/sys": "v0.49.0", "golang.org/x/text": "v0.43.0"}
	for _, dep := range info.Deps {
		if want, known := versions[dep.Path]; known {
			r.require(dep.Version == want && dep.Replace == nil, "dependency-"+dep.Path, nil)
			delete(versions, dep.Path)
		}
	}
	r.require(len(versions) == 0, "dependencies-complete", nil)
	r.require(len(sqlite.PendingEncryptedCleanups()) == 0, "initial-no-pending-owners", nil)
	r.receipt.Toolchain = runtime.Version()
	r.receipt.Platform = runtime.GOOS + "/" + runtime.GOARCH
	r.receipt.Approval = os.Getenv("GOSQLITE_MANUAL_FORMAT_UPGRADE_APPROVAL")
	r.require(len(r.receipt.Approval) >= 8 && len(r.receipt.Approval) <= 128, "approval-reference", nil)
	binary, err := os.Executable()
	r.event("executable-path", binary, err)
	r.require(err == nil, "executable-path-result", err)
	_, actualBinary := r.hashFile(binary, 128<<20, false)
	r.receipt.BinarySHA256 = actualBinary
	r.require(formatupgrade.Hash(os.Getenv("GOSQLITE_MANUAL_FORMAT_UPGRADE_BINARY_SHA256")) && actualBinary == os.Getenv("GOSQLITE_MANUAL_FORMAT_UPGRADE_BINARY_SHA256"), "binary-pin", nil)
	source := os.Getenv("GOSQLITE_MANUAL_FORMAT_UPGRADE_SOURCE_ROOT")
	r.require(filepath.IsAbs(source) && filepath.Clean(source) == source, "source-root-path", nil)
	sourceInfo, err := os.Lstat(source)
	r.event("source-root-lstat", source, err)
	r.require(err == nil && sourceInfo.IsDir() && sourceInfo.Mode()&os.ModeSymlink == 0, "source-root-directory", err)
	r.receipt.SourceRoot = manualFormatDirectory(source, sourceInfo)
	r.event("source-root-identity", r.receipt.SourceRoot, nil)
	r.require(r.receipt.SourceRoot.Validate(false) == nil, "source-root-identity-valid", nil)
	r.receipt.SourceSHA256 = r.sourceDigest(source)
	r.require(formatupgrade.Hash(os.Getenv("GOSQLITE_MANUAL_FORMAT_UPGRADE_SOURCE_SHA256")) && r.receipt.SourceSHA256 == os.Getenv("GOSQLITE_MANUAL_FORMAT_UPGRADE_SOURCE_SHA256"), "source-pin", nil)
	root := os.Getenv("GOSQLITE_MANUAL_FORMAT_UPGRADE_ROOT")
	r.require(filepath.IsAbs(root) && filepath.Clean(root) == root && root != "/", "known-root-path", nil)
	resolved, err := filepath.EvalSymlinks(root)
	r.event("root-resolution", resolved, err)
	r.require(err == nil && resolved == root, "root-no-symlink", err)
	original := r.directory(root)
	r.receipt.KnownRoot = original
	var filesystem syscall.Statfs_t
	err = syscall.Statfs(root, &filesystem)
	r.event("root-filesystem-facts", filesystem, err)
	r.require(err == nil && filesystem.Type != 0, "root-filesystem-observed", err)
	var kernel syscall.Utsname
	err = syscall.Uname(&kernel)
	r.event("kernel-facts", kernel, err)
	r.require(err == nil, "kernel-observed", err)
	runDir, err := os.MkdirTemp(root, "format-upgrade-")
	r.receipt.RunDirectory = runDir
	r.event("create-owned-run-directory", runDir, err)
	r.require(err == nil, "create-owned-run-directory-result", err)
	r.receipt.RunRoot = r.directory(runDir)
	current := r.directory(root)
	r.require(original == current, "root-still-same-directory", nil)
	r.event("retained-run-directory", runDir, nil)
}

func manualFormatDirectory(path string, info os.FileInfo) formatupgrade.Directory {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return formatupgrade.Directory{}
	}
	return formatupgrade.Directory{Path: path, Device: uint64(s.Dev), Inode: s.Ino, UID: s.Uid, Mode: uint32(info.Mode().Perm())}
}
func (r *manualFormatRun) directory(path string) formatupgrade.Directory {
	info, err := os.Lstat(path)
	r.event("directory-lstat", path, err)
	r.require(err == nil, "directory-exists", err)
	s, ok := info.Sys().(*syscall.Stat_t)
	r.require(ok && info.IsDir() && info.Mode()&os.ModeSymlink == 0 && info.Mode().Perm() == 0700 && s.Uid == uint32(os.Geteuid()), "private-owned-directory", nil)
	fact := manualFormatDirectory(path, info)
	r.event("checked-directory-identity", fact, nil)
	return fact
}
func (r *manualFormatRun) makeDirectory(parent, child string) string {
	path := filepath.Join(parent, child)
	err := os.Mkdir(path, 0700)
	r.event("mkdir", path, err)
	r.require(err == nil, "new-directory-only", err)
	r.directory(path)
	return path
}
func (r *manualFormatRun) absent(path string) {
	info, err := os.Lstat(path)
	r.event("absent-destination", path, err)
	r.require(info == nil && errors.Is(err, os.ErrNotExist), "fresh-destination", nil)
}

func (r *manualFormatRun) hashFile(path string, limit int64, private bool) (formatupgrade.File, string) {
	before, err := os.Lstat(path)
	r.event("file-lstat", path, err)
	r.require(err == nil, "file-lstat-result", err)
	s, ok := before.Sys().(*syscall.Stat_t)
	r.require(ok && before.Mode().IsRegular() && before.Mode()&os.ModeSymlink == 0 && before.Size() >= 0 && before.Size() <= limit && s.Nlink == 1, "bounded-direct-regular-file", nil)
	if private {
		r.require(before.Mode().Perm() == 0600 && s.Uid == uint32(os.Geteuid()), "private-owned-file", nil)
	}
	f, err := os.Open(path)
	o := r.capture("file:"+path, nil, nil, f, 0, err)
	r.require(err == nil && f != nil, "file-open", err)
	held, err := f.Stat()
	r.event("file-fstat", path, err)
	r.require(err == nil && os.SameFile(before, held), "held-file-identity", err)
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, limit+1))
	r.event("file-hash-read", n, err)
	r.require(err == nil && n == before.Size() && n <= limit, "file-hash-extent", err)
	after, err := os.Lstat(path)
	r.event("file-final-lstat", path, err)
	r.require(err == nil && os.SameFile(before, after) && before.Size() == after.Size() && before.ModTime() == after.ModTime(), "file-stable-during-hash", err)
	err = r.retire(o)
	r.require(err == nil, "hash-file-retired", err)
	digest := hex.EncodeToString(h.Sum(nil))
	return formatupgrade.File{Name: filepath.Base(path), Size: n, SHA256: digest, Device: uint64(s.Dev), Inode: s.Ino, Mode: uint32(before.Mode().Perm())}, digest
}
func (r *manualFormatRun) sourceDigest(root string) string {
	paths := []string{}
	dirs := []string{root}
	for i := 0; i < len(dirs); i++ {
		if len(dirs) > 1024 {
			r.t.Fatal("source directory bound")
		}
		path := dirs[i]
		before, err := os.Lstat(path)
		r.event("source-directory-lstat", path, err)
		r.require(err == nil && before.IsDir() && before.Mode()&os.ModeSymlink == 0, "source-direct-directory", err)
		f, err := os.Open(path)
		o := r.capture("file:"+path, nil, nil, f, 0, err)
		r.require(err == nil && f != nil, "source-directory-open", err)
		held, err := f.Stat()
		r.event("source-directory-fstat", path, err)
		r.require(err == nil && os.SameFile(before, held), "source-directory-identity", err)
		entries, err := f.ReadDir(4097)
		r.event("source-directory-read", len(entries), err)
		r.require((err == nil || errors.Is(err, io.EOF)) && len(entries) <= 4096, "source-directory-bound", nil)
		err = r.retire(o)
		r.require(err == nil, "source-directory-retired", err)
		for _, entry := range entries {
			if entry.Name() == ".git" {
				continue
			}
			r.require(entry.Type()&os.ModeSymlink == 0, "source-no-symlink", nil)
			child := filepath.Join(path, entry.Name())
			if entry.IsDir() {
				dirs = append(dirs, child)
				continue
			}
			rel, err := filepath.Rel(root, child)
			r.require(err == nil, "source-relative-path", err)
			if strings.HasSuffix(rel, ".go") || rel == "go.mod" || rel == "go.sum" {
				paths = append(paths, filepath.ToSlash(rel))
				r.require(len(paths) <= 4096, "source-file-count", nil)
			}
		}
	}
	r.require(len(paths) > 10, "source-inventory-nonempty", nil)
	sort.Strings(paths)
	h := sha256.New()
	var total int64
	for _, path := range paths {
		f, digest := r.hashFile(filepath.Join(root, filepath.FromSlash(path)), 8<<20, false)
		total += f.Size
		r.require(total <= 128<<20, "source-byte-bound", nil)
		_, err := fmt.Fprintf(h, "%s\t%s\n", path, digest)
		r.require(err == nil, "source-digest-write", err)
	}
	return hex.EncodeToString(h.Sum(nil))
}
func (r *manualFormatRun) manifest(dir, phase string) formatupgrade.Snapshot {
	before := r.directory(dir)
	f, err := os.Open(dir)
	o := r.capture("file:"+dir, nil, nil, f, 0, err)
	r.require(err == nil && f != nil, "directory-open", err)
	held, err := f.Stat()
	r.event("directory-fstat", dir, err)
	r.require(err == nil && before == manualFormatDirectory(dir, held), "held-directory-identity", err)
	entries, err := f.ReadDir(formatupgrade.MaxEntries + 1)
	r.event("bounded-directory-read", len(entries), err)
	r.require((err == nil || errors.Is(err, io.EOF)) && len(entries) > 0 && len(entries) <= formatupgrade.MaxEntries, "directory-bound", nil)
	err = r.retire(o)
	r.require(err == nil, "directory-retired", err)
	out := formatupgrade.Manifest{}
	for _, entry := range entries {
		record, _ := r.hashFile(filepath.Join(dir, entry.Name()), formatupgrade.MaxBytes, true)
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	err = out.Validate()
	r.require(err == nil, "manifest-valid", err)
	r.require(before == r.directory(dir), "manifest-directory-stable", nil)
	snapshot := formatupgrade.Snapshot{Directory: before, Phase: phase, Files: out}
	r.event("manifest", snapshot, nil)
	return snapshot
}

func (r *manualFormatRun) query(ctx context.Context, dbOwner *manualFormatOwner, label, query string) ([][]any, sqlite.QueryPlan) {
	rows, err := dbOwner.db.Query(ctx, query)
	o := r.capture("rows:"+label, nil, rows, nil, dbOwner.state.ID, err)
	r.require(err == nil && rows != nil, "query-"+label, err)
	plan := rows.Plan()
	out := [][]any{}
	for n := 0; n <= 16; n++ {
		more := rows.Next()
		if !more {
			err = rows.Err()
			stateErr := o.state.Retire("eof", manualFormatError(err))
			r.event("rows-eof", o.state, err)
			r.require(err == nil && stateErr == nil, "rows-eof-retired", errors.Join(err, stateErr))
			r.event("query-result-"+label, struct {
				Rows [][]any
				Plan sqlite.QueryPlan
			}{out, plan}, nil)
			return out, plan
		}
		out = append(out, rows.Values())
	}
	r.t.Fatal("row bound exceeded; known owner is retained for one close")
	return nil, sqlite.QueryPlan{}
}
func (r *manualFormatRun) verify(ctx context.Context, o *manualFormatOwner, label string) formatupgrade.Head {
	info, err := o.db.VerifyEncrypted(ctx)
	r.event(label, info, err)
	r.require(err == nil, "verify-encrypted", err)
	return manualFormatHead(info)
}
func (r *manualFormatRun) ledger(ctx context.Context, o *manualFormatOwner, label string) {
	schema, err := o.db.Schema(ctx)
	r.event("schema-"+label, schema, err)
	r.require(err == nil, "schema-result", err)
	uniqueSchema := 0
	for _, entry := range schema {
		if entry.Name == "docs_token" && entry.Type == "index" && entry.TableName == "docs" && !entry.SQLNull && entry.SQL == "CREATE UNIQUE INDEX docs_token ON docs(token)" {
			uniqueSchema++
		}
	}
	r.require(uniqueSchema == 1, "explicit-unique-schema-preserved", nil)
	result, err := o.db.Check(ctx, sqlite.CheckOptions{MaxPages: 4096, MaxPageReads: 1 << 20})
	r.event("check-"+label, result, err)
	r.require(err == nil && result.Scope == sqlite.CheckScope && result.Indexes >= 1 && result.ExtensionIndexes == 1, "check-supported-indexes", err)
	got, _ := r.query(ctx, o, label+"-rows", "SELECT id,token,body,weight,payload,nullable FROM docs ORDER BY id")
	want := [][]any{{int64(-7), "alpha", "数据库 alpha", float64(1.5), []byte{0, 1, 254}, nil}, {int64(2), "beta", "plain beta", float64(-2.25), []byte{2, 3}, "kept"}, {int64(9), "gamma", "数据库 gamma", float64(0), []byte{}, nil}}
	r.require(reflect.DeepEqual(got, want), "complete-typed-row-ledger", nil)
	got, plan := r.query(ctx, o, label+"-unique", "SELECT id FROM docs WHERE token='beta'")
	r.require(reflect.DeepEqual(got, [][]any{{int64(2)}}) && plan.Access == "SCAN" && plan.Method == "" && len(plan.Indexes) == 0, "explicit-unique-supported-scan", nil)
	indexed, plan := r.query(ctx, o, label+"-grams", "SELECT id FROM docs WHERE body LIKE '%数据库%' ORDER BY id")
	scan, scanPlan := r.query(ctx, o, label+"-scan", "SELECT id FROM docs NOT INDEXED WHERE body LIKE '%数据库%' ORDER BY id")
	r.require(reflect.DeepEqual(indexed, [][]any{{int64(-7)}, {int64(9)}}) && reflect.DeepEqual(indexed, scan) && plan.Method == "grams13" && plan.Access == "SEARCH" && reflect.DeepEqual(plan.Indexes, []string{"docs_grams"}) && scanPlan.Access == "SCAN" && scanPlan.Reason == "NOT INDEXED requested", "grams-index-and-forced-scan", nil)
	vectors, _ := r.query(ctx, o, label+"-vector", "SELECT id,vector_to_text(embedding),vector_dims(embedding),l2_distance(embedding,'[1,0]') FROM docs ORDER BY id")
	r.require(reflect.DeepEqual(vectors, [][]any{{int64(-7), "[1,0]", int64(2), float64(0)}, {int64(2), "[4,4]", int64(2), float64(5)}, {int64(9), "[0,0]", int64(2), float64(1)}}), "dense-vector-exact-ledger", nil)
}
func (r *manualFormatRun) runCase(ctx context.Context, page uint32) {
	r.page = page
	caseDir := r.makeDirectory(r.receipt.RunDirectory, fmt.Sprintf("page%d", page))
	sourceDir := r.makeDirectory(caseDir, "source")
	archiveDir := r.makeDirectory(caseDir, "archive")
	restoreDir := r.makeDirectory(caseDir, "restored")
	source := filepath.Join(sourceDir, "source.gse")
	archive := filepath.Join(archiveDir, "before-upgrade.archive")
	restored := filepath.Join(restoreDir, "restored.gse")
	r.absent(source)
	r.absent(archive)
	r.absent(restored)
	cfg := r.options("fixture-source", sqlite.EncryptionSnapshot)
	archiveCfg := r.options("fixture-archive", sqlite.EncryptionSnapshot)
	restoreCfg := r.options("fixture-restored", sqlite.EncryptionSnapshot)
	c := formatupgrade.Case{PageSize: page, Root: r.directory(caseDir), SourceDirectory: r.directory(sourceDir), ArchiveDirectory: r.directory(archiveDir), RestoreDirectory: r.directory(restoreDir)}
	db, err := sqlite.OpenContext(ctx, source, sqlite.Options{Encryption: &cfg, PageSize: page, MaxTransactionPages: 4096, MaxSortBytes: 1 << 20})
	o := r.capture("source", db, nil, nil, 0, err)
	r.require(err == nil && db != nil, "source-open", err)
	for _, sql := range []string{
		"CREATE TABLE docs(id INTEGER PRIMARY KEY,token TEXT,body TEXT,weight REAL,payload BLOB,nullable TEXT,embedding VECTOR(2))",
		"CREATE UNIQUE INDEX docs_token ON docs(token)",
		"INSERT INTO docs VALUES(-7,'alpha','数据库 alpha',1.5,x'0001fe',NULL,'[1,0]')",
		"INSERT INTO docs VALUES(2,'beta','plain beta',-2.25,x'0203','kept','[4,4]')",
		"INSERT INTO docs VALUES(9,'gamma','数据库 gamma',0.0,x'',NULL,'[0,0]')",
		"CREATE INDEX docs_grams ON docs(body) USING grams13",
	} {
		result, err := db.Exec(ctx, sql)
		r.event("fixture-exec", result, err)
		r.require(err == nil, "fixture-exec-result", err)
	}
	c.Before = r.verify(ctx, o, "snapshot-head")
	r.require(c.Before.Layout == 1 && c.Before.Mode == 0 && c.Before.PageSize == page, "original-snapshot-layout", nil)
	r.ledger(ctx, o, "snapshot")
	c.Checks = append(c.Checks, "snapshot-ledger-index")
	beforeUnique := r.manifest(sourceDir, "unique-before")
	uniqueResult, uniqueErr := db.Exec(ctx, "INSERT INTO docs VALUES(100,'beta','duplicate',1.0,x'ff',NULL,'[0,1]')")
	r.event("unique-rejection", uniqueResult, uniqueErr)
	r.require(errors.Is(uniqueErr, sqlite.ErrConstraint) && !errors.Is(uniqueErr, sqlite.ErrEncryptedCleanupPending), "unique-constraint-refused", nil)
	afterUnique := r.verify(ctx, o, "unique-refusal-head")
	r.require(afterUnique == c.Before, "unique-refusal-unchanged-head", nil)
	err = formatupgrade.Equal(beforeUnique, r.manifest(sourceDir, "unique-after"))
	r.require(err == nil, "unique-refusal-unchanged-files", err)
	r.ledger(ctx, o, "after-unique-refusal")
	c.Checks = append(c.Checks, "unique-rejected-unchanged")
	archiveInfo, err := db.BackupEncrypted(ctx, archive, archiveCfg)
	r.event("backup-encrypted", archiveInfo, err)
	r.require(err == nil, "backup-encrypted-result", err)
	c.Archive = manualFormatHead(archiveInfo)
	c.ArchiveBefore = r.manifest(archiveDir, "pre-upgrade-archive")
	c.SourceBefore = r.manifest(sourceDir, "pre-upgrade-source")
	c.Checks = append(c.Checks, "archive-created")
	err = db.UpgradeEncryptedWAL(ctx)
	r.event("explicit-upgrade-layout1-to2", nil, err)
	r.require(err == nil, "explicit-upgrade-result", err)
	c.Upgraded = r.verify(ctx, o, "upgraded-head")
	r.require(c.Upgraded.Layout == 2 && c.Upgraded.Mode == 1 && c.Upgraded.DBID == c.Before.DBID && c.Upgraded.Sequence == c.Before.Sequence+1 && c.Upgraded.Segments == 0 && c.Upgraded.PageCount == c.Before.PageCount, "upgrade-identity-and-sequence", nil)
	c.Checks = append(c.Checks, "upgrade-identity")
	err = r.retire(o)
	r.require(err == nil, "source-close-once", err)
	c.SourceAfter = r.manifest(sourceDir, "post-upgrade-source")
	err = formatupgrade.Preserve(c.SourceBefore, c.SourceAfter, "source.gse")
	r.require(err == nil, "old-immutable-ciphertext-and-root-retained", err)
	err = formatupgrade.Equal(c.ArchiveBefore, r.manifest(archiveDir, "archive-after-upgrade"))
	r.require(err == nil, "archive-retained-through-upgrade", err)
	c.Checks = append(c.Checks, "originals-retained")
	old, oldErr := sqlite.OpenContext(ctx, source, sqlite.Options{Encryption: &cfg, ReadOnly: true, NoCreate: true})
	oldOwner := r.capture("old-mode-rejection", old, nil, nil, 0, oldErr)
	var oldDetail *sqlite.Error
	c.OldModeRejected = errors.Is(oldErr, sqlite.ErrUnsupported) && errors.As(oldErr, &oldDetail) && oldDetail.Code == sqlite.CodeError && !errors.Is(oldErr, sqlite.ErrEncryptedCleanupPending)
	c.OldModeNil = !oldOwner.state.Present
	r.require(c.OldModeRejected && c.OldModeNil, "old-mode-explicitly-refused", nil)
	oldOwner.state.Refusal = "unsupported-old-mode"
	err = formatupgrade.Equal(c.SourceAfter, r.manifest(sourceDir, "old-mode-after"))
	c.OldModeUnchanged = err == nil
	r.require(c.OldModeUnchanged, "old-mode-family-unchanged", err)
	c.Checks = append(c.Checks, "old-mode-rejected-unchanged")
	walCfg := cfg
	walCfg.Mode = sqlite.EncryptionWAL
	reopened, err := sqlite.OpenContext(ctx, source, sqlite.Options{Encryption: &walCfg, ReadOnly: true, NoCreate: true})
	walOwner := r.capture("wal-reopen", reopened, nil, nil, 0, err)
	r.require(err == nil && reopened != nil, "wal-reopen-result", err)
	c.Reopened = r.verify(ctx, walOwner, "reopened-wal-head")
	r.require(c.Reopened == c.Upgraded, "wal-reopen-identity", nil)
	r.ledger(ctx, walOwner, "wal-reopened")
	err = r.retire(walOwner)
	r.require(err == nil, "wal-reopen-close-once", err)
	c.Checks = append(c.Checks, "wal-ledger-index")
	r.absent(restored)
	restoreInfo, err := sqlite.RestoreEncrypted(ctx, archive, restored, archiveCfg, restoreCfg)
	r.event("fresh-snapshot-restore", restoreInfo, err)
	r.require(err == nil, "fresh-snapshot-restore-result", err)
	r.require(restoreInfo.StorageLayout == 1 && restoreInfo.Mode == sqlite.EncryptionSnapshot && restoreInfo.DatabaseID != archiveInfo.DatabaseID && hex.EncodeToString(restoreInfo.DatabaseID[:]) != c.Before.DBID, "fresh-restore-identity-and-layout", nil)
	c.Checks = append(c.Checks, "fresh-restore")
	restoredDB, err := sqlite.OpenContext(ctx, restored, sqlite.Options{Encryption: &restoreCfg, ReadOnly: true, NoCreate: true})
	restoredOwner := r.capture("restored-snapshot", restoredDB, nil, nil, 0, err)
	r.require(err == nil && restoredDB != nil, "restored-snapshot-open", err)
	c.Restored = r.verify(ctx, restoredOwner, "restored-snapshot-head")
	r.require(c.Restored == manualFormatHead(restoreInfo), "restored-verified-head-matches-result", nil)
	r.ledger(ctx, restoredOwner, "restored-snapshot")
	err = r.retire(restoredOwner)
	r.require(err == nil, "restored-close-once", err)
	c.RestoredFiles = r.manifest(restoreDir, "restored-snapshot")
	c.Checks = append(c.Checks, "restored-ledger-index")
	c.SourceFinal = r.manifest(sourceDir, "final-source")
	c.ArchiveAfter = r.manifest(archiveDir, "final-archive")
	err = errors.Join(formatupgrade.Equal(c.SourceAfter, c.SourceFinal), formatupgrade.Equal(c.ArchiveBefore, c.ArchiveAfter))
	r.require(err == nil, "source-and-archive-final-conservation", err)
	c.Checks = append(c.Checks, "source-and-archive-unchanged")
	r.require(c.Root == r.directory(caseDir), "case-root-final-identity", nil)
	for _, owner := range r.owners {
		if owner.state.Page == page && owner.state.Kind != "file" {
			c.OwnerIDs = append(c.OwnerIDs, owner.state.ID)
		}
	}
	err = c.Validate()
	r.event("case-receipt", c, err)
	r.require(err == nil, "case-complete", err)
	r.receipt.Cases = append(r.receipt.Cases, c)
}
