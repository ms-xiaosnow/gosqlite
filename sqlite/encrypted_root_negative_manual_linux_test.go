//go:build linux && manual_encrypted_root_negative

package sqlite_test

// SOURCE_ONLY / REVIEW_HOLD / PHYSICAL_NOT_RUN. This entry is deliberately
// excluded from default builds. Gates are acknowledgements, not authorization.
import (
	"bytes"
	"context"
	"crypto/sha256"
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
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/encryptedroot"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

const rootAuthMarker = "root-negative-marker-6b3d2f"

type rootAuthProvider struct {
	mu                 sync.Mutex
	wrong              bool
	acquires, releases int
	duplicate          bool
}
type rootAuthMaterial struct {
	provider *rootAuthProvider
	key      []byte
	released bool
}

func (p *rootAuthProvider) Acquire(context.Context, encryption.KeyRequest) (encryption.KeyMaterial, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.acquires++
	// Public disposable test material; never accepts a user key or credential.
	key := sha256.Sum256([]byte("public-root-negative-fixture-key-v1"))
	if p.wrong {
		key[0] ^= 1
	}
	return &rootAuthMaterial{provider: p, key: append([]byte(nil), key[:]...)}, nil
}
func (m *rootAuthMaterial) Kind() encryption.CredentialKind { return encryption.RawKey256 }
func (m *rootAuthMaterial) Bytes() []byte                   { return m.key }
func (m *rootAuthMaterial) Release() {
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

type rootAuthOwner struct {
	state encryptedroot.Owner
	db    *sqlite.DB
	rows  *sqlite.Rows
	file  *os.File
}
type rootAuthRun struct {
	t               *testing.T
	receipt         encryptedroot.Receipt
	owners          []*rootAuthOwner
	causes          []error // original objects retained; no Error/String logging
	seed, candidate *rootAuthProvider
}

var rootAuthRetained []*rootAuthRun

func (r *rootAuthRun) pending(err error) {
	r.causes = append(r.causes, err)
	pending := sqlite.PendingEncryptedCleanups()
	r.causes = append(r.causes, pending...)
	if len(pending) != 0 || errors.Is(err, sqlite.ErrEncryptedCleanupPending) {
		r.receipt.PendingCleanup = true
	}
}
func (r *rootAuthRun) check(ok bool, label string, err error) {
	r.pending(err)
	if !ok || err != nil || r.receipt.PendingCleanup {
		r.t.Fatal(label + "; artifacts and error owners retained")
	}
}
func (r *rootAuthRun) capture(kind string, parent int, db *sqlite.DB, rows *sqlite.Rows, file *os.File, err error) *rootAuthOwner {
	o := &rootAuthOwner{state: encryptedroot.Owner{ID: len(r.owners) + 1, Parent: parent, Kind: kind, Terminal: "absent"}, db: db, rows: rows, file: file}
	o.state.Present = db != nil || rows != nil || file != nil
	if o.state.Present {
		o.state.Terminal = "live"
	}
	r.owners = append(r.owners, o)
	r.pending(err)
	if len(r.owners) > encryptedroot.MaxOwners {
		r.t.Fatal("owner bound; owners retained")
	}
	return o
}
func (r *rootAuthRun) closeOwner(o *rootAuthOwner) error {
	r.pending(nil)
	if !o.state.Present || o.state.Attempts != 0 || o.state.Terminal != "live" {
		return errors.New("owner not closeable")
	}
	if o.state.Kind != "file" && r.receipt.PendingCleanup {
		o.state.Terminal = "blocked"
		return errors.New("pending internal owner")
	}
	for _, child := range r.owners {
		if child.state.Parent == o.state.ID && child.state.Present && child.state.Terminal != "closed" && child.state.Terminal != "eof" {
			o.state.Terminal = "blocked"
			return errors.New("child owner not retired")
		}
	}
	var err error
	if o.file != nil {
		err = o.file.Close()
	} else if o.rows != nil {
		err = o.rows.Close()
	} else if o.db != nil {
		err = o.db.Close()
	}
	r.pending(err)
	terminal := "closed"
	if err != nil {
		terminal = "unknown"
	}
	stateErr := o.state.Retire(terminal)
	return errors.Join(err, stateErr)
}
func (r *rootAuthRun) finish() {
	r.pending(nil)
	for i := len(r.owners) - 1; i >= 0; i-- {
		o := r.owners[i]
		if o.state.Present && o.state.Terminal == "live" && o.state.Attempts == 0 {
			if r.closeOwner(o) != nil {
				r.t.Error("owner retirement incomplete; no retry")
			}
		}
	}
	for _, o := range r.owners {
		r.receipt.Owners = append(r.receipt.Owners, o.state)
	}
	if r.seed != nil {
		r.seed.mu.Lock()
		r.receipt.SeedAcquires, r.receipt.SeedReleases = r.seed.acquires, r.seed.releases
		r.receipt.DuplicateRelease = r.seed.duplicate
		r.seed.mu.Unlock()
	}
	if r.candidate != nil {
		r.candidate.mu.Lock()
		r.receipt.CandidateAcquires, r.receipt.CandidateReleases = r.candidate.acquires, r.candidate.releases
		r.receipt.DuplicateRelease = r.receipt.DuplicateRelease || r.candidate.duplicate
		r.candidate.mu.Unlock()
	}
	r.pending(nil)
	if r.t.Failed() {
		r.receipt.Status = "failed"
	}
	if err := r.receipt.Validate(); err != nil {
		r.receipt.Status = "failed"
		r.t.Error("finite receipt refused")
	}
	encoded, err := json.Marshal(r.receipt)
	if err != nil || len(encoded) > 65536 || bytes.Contains(encoded, []byte(rootAuthMarker)) || bytes.Contains(encoded, []byte("SQLite format 3")) {
		r.t.Error("diagnostic encoding refused")
		return
	}
	r.t.Logf("ROOT_AUTH_RECEIPT %s", encoded)
	// No durability, power-loss, nonce-restart or whole-A8 PASS claim.
	if !r.t.Failed() {
		r.t.Log("ROOT_AUTH_OBSERVED one_case=true originals_retained=true")
	}
}

func rootAuthOptions(provider *rootAuthProvider) sqlite.EncryptionOptions {
	return sqlite.EncryptionOptions{KeyProvider: provider, CredentialKind: encryption.RawKey256, Mode: sqlite.EncryptionSnapshot,
		MaxGenerations: 3, MaxRetainedBytes: 65536, MaxDirectoryEntries: 16,
		Limits: encryption.Limits{MaxPages: 2, MaxRecords: 4, MaxRecordBytes: 512, MaxLogicalBytes: 1024}}
}
func (r *rootAuthRun) directory(path string) encryptedroot.Directory {
	info, err := os.Lstat(path)
	r.check(err == nil, "directory stat", err)
	s, ok := info.Sys().(*syscall.Stat_t)
	r.check(ok && info.IsDir() && info.Mode()&os.ModeSymlink == 0 && info.Mode().Perm() == 0700 && s.Uid == uint32(os.Geteuid()), "private owned directory", nil)
	return encryptedroot.Directory{Device: uint64(s.Dev), Inode: s.Ino, UID: s.Uid, Mode: uint32(info.Mode().Perm())}
}
func (r *rootAuthRun) makeDirectory(parent, name string) string {
	path := filepath.Join(parent, name)
	err := os.Mkdir(path, 0700)
	r.check(err == nil, "fresh directory only", err)
	return path
}

// The same single-owner helper reads only bounded regular direct files. Read,
// Sync and Close failures never become an authentication-negative observation.
func (r *rootAuthRun) read(path string, limit int64, private bool) ([]byte, encryptedroot.File) {
	before, err := os.Lstat(path)
	r.check(err == nil, "file stat", err)
	s, ok := before.Sys().(*syscall.Stat_t)
	r.check(ok && before.Mode().IsRegular() && before.Mode()&os.ModeSymlink == 0 && before.Size() >= 0 && before.Size() <= limit && s.Nlink == 1, "bounded direct regular file", nil)
	if private {
		r.check(before.Mode().Perm() == 0600 && s.Uid == uint32(os.Geteuid()), "private owned file", nil)
	}
	file, err := os.Open(path)
	o := r.capture("file", 0, nil, nil, file, err)
	r.check(file != nil && err == nil, "read owner", err)
	held, err := file.Stat()
	r.check(err == nil && os.SameFile(before, held), "held file identity", err)
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	r.check(err == nil && int64(len(data)) == before.Size() && int64(len(data)) <= limit, "bounded file read", err)
	after, err := os.Lstat(path)
	r.check(err == nil && os.SameFile(before, after) && before.Size() == after.Size() && before.ModTime() == after.ModTime(), "stable file read", err)
	err = r.closeOwner(o)
	r.check(err == nil, "read owner retired", err)
	return data, encryptedroot.File{Name: filepath.Base(path), Size: int64(len(data)), SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Device: uint64(s.Dev), Inode: s.Ino, UID: s.Uid, Mode: uint32(before.Mode().Perm()), Links: uint64(s.Nlink)}
}
func (r *rootAuthRun) snapshot(dir string) ([]encryptedroot.File, map[string][]byte) {
	directory := r.directory(dir)
	file, err := os.Open(dir)
	o := r.capture("file", 0, nil, nil, file, err)
	r.check(file != nil && err == nil, "directory read owner", err)
	held, err := file.Stat()
	r.check(err == nil, "directory held stat", err)
	s, ok := held.Sys().(*syscall.Stat_t)
	r.check(ok && uint64(s.Dev) == directory.Device && s.Ino == directory.Inode, "directory held identity", nil)
	entries, err := file.ReadDir(encryptedroot.FamilyFiles + 1)
	r.pending(err)
	r.check((err == nil || err == io.EOF) && len(entries) == encryptedroot.FamilyFiles, "exact family entry count", nil)
	err = r.closeOwner(o)
	r.check(err == nil, "directory owner retired", err)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	out := []encryptedroot.File{}
	data := map[string][]byte{}
	for _, entry := range entries {
		body, fact := r.read(filepath.Join(dir, entry.Name()), encryptedroot.MaxFileBytes, true)
		if bytes.Contains(body, []byte(rootAuthMarker)) || bytes.Contains(body, []byte("SQLite format 3")) {
			r.receipt.PlaintextMarker = true
		}
		out = append(out, fact)
		data[entry.Name()] = body
	}
	r.check(directory == r.directory(dir), "snapshot directory stable", nil)
	return out, data
}
func (r *rootAuthRun) writeCopy(path string, data []byte) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	o := r.capture("file", 0, nil, nil, file, err)
	r.check(file != nil && err == nil, "fresh copy owner", err)
	n, err := file.Write(data)
	r.check(err == nil && n == len(data), "one complete copy write", err)
	err = file.Sync()
	r.check(err == nil, "copy sync", err)
	err = r.closeOwner(o)
	r.check(err == nil, "copy owner retired", err)
}

func (r *rootAuthRun) ledger(ctx context.Context, owner *rootAuthOwner) encryptedroot.Ledger {
	var result encryptedroot.Ledger
	info, err := owner.db.VerifyEncrypted(ctx)
	r.check(err == nil, "verify healthy layout1", err)
	result.Verified, result.Layout, result.PageSize, result.Pages, result.Sequence = true, info.StorageLayout, info.PageSize, info.PageCount, info.CommitSequence
	r.check(info.Mode == sqlite.EncryptionSnapshot && info.StorageLayout == 1 && info.PageSize == 512 && info.PageCount == 2 && info.CommitSequence == 3, "fixed healthy geometry", nil)
	check, err := owner.db.Check(ctx, sqlite.CheckOptions{MaxPages: 2, MaxPageReads: 64})
	r.check(err == nil && check.Scope == sqlite.CheckScope, "healthy check", err)
	result.Checked = true
	rows, err := owner.db.Query(ctx, "SELECT id,body FROM entries ORDER BY id")
	rowOwner := r.capture("rows", owner.state.ID, nil, rows, nil, err)
	r.check(err == nil && rows != nil, "healthy query", err)
	got := [][]any{}
	for n := 0; n < 4; n++ {
		if !rows.Next() {
			err = rows.Err()
			r.pending(err)
			terminal := "eof"
			if err != nil {
				terminal = "unknown"
			}
			stateErr := rowOwner.state.Retire(terminal)
			r.check(err == nil && stateErr == nil, "rows EOF retirement", errors.Join(err, stateErr))
			result.EOF, result.RowsErrNil = true, true
			break
		}
		got = append(got, rows.Values())
	}
	want := [][]any{{int64(-7), "alpha"}, {int64(2), rootAuthMarker}, {int64(9), "omega"}}
	result.Rows, result.ExactRows = len(got), reflect.DeepEqual(got, want)
	r.check(result.ExactRows && result.EOF && result.RowsErrNil, "complete literal row ledger", nil)
	return result
}

// The managed cleanup diagnostic intentionally preserves Is/As without an
// Unwrap method. Inspect the original result with public classification, never
// replace it with a string or count an unrelated non-nil I/O error as success.
func rootAuthClass(err error) (string, int, bool) {
	if err == nil {
		return "ok", 0, true
	}
	detail, ok := err.(*sqlite.Error)
	if !ok || detail.Op != "open" {
		return "other", -1, false
	}
	var pathError *os.PathError
	var syscallError syscall.Errno
	ioFailure := errors.As(err, &pathError) || errors.As(err, &syscallError) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrShortWrite)
	other := errors.Is(err, sqlite.ErrEncryptedCleanupPending) || errors.Is(err, sqlite.ErrPoisoned) || errors.Is(err, sqlite.ErrBusy) || errors.Is(err, sqlite.ErrChanged) || errors.Is(err, sqlite.ErrRecoveryRequired) || errors.Is(err, sqlite.ErrCommitUncertain) || errors.Is(err, sqlite.ErrCorrupt) || errors.Is(err, sqlite.ErrMemoryLimit) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, encryption.ErrFormat) || errors.Is(err, encryption.ErrKeyProvider) || errors.Is(err, encryption.ErrCredential)
	facts := encryptedroot.ErrorFacts{Code: int(detail.Code), Authentication: errors.Is(err, encryption.ErrAuthentication), CryptoUnsupported: errors.Is(err, encryption.ErrUnsupported), PublicUnsupported: errors.Is(err, sqlite.ErrUnsupported), IO: ioFailure, Other: other}
	return encryptedroot.Classify(facts)
}

func TestManualEncryptedRootNegative(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_ENCRYPTED_ROOT_NEGATIVE") != "independently_authorized" {
		t.Fatal("manual encrypted root negative is not independently authorized")
	}
	r := &rootAuthRun{t: t, receipt: encryptedroot.Receipt{Profile: encryptedroot.Profile, Status: "started"}}
	rootAuthRetained = append(rootAuthRetained, r)
	defer r.finish()
	plan, err := encryptedroot.Select(os.Getenv("GOSQLITE_MANUAL_ENCRYPTED_ROOT_CASE"))
	r.check(err == nil, "one admitted case", err)
	r.receipt.Case, r.receipt.ChangedOffset = plan.Case, -1
	r.check(os.Geteuid() > 0 && runtime.GOOS == "linux" && os.Getenv("GOFLAGS") == "", "nonprivileged linux profile", nil)
	r.check(os.Getenv("GOSQLITE_MANUAL_ENCRYPTED_ROOT_NAMESPACE") == "private-stable-no-alias-no-other-process", "isolated retained namespace acknowledgement", nil)
	wanted := map[string]string{"test.run": "^TestManualEncryptedRootNegative$", "test.count": "1", "test.timeout": "0s", "test.parallel": "1", "test.shuffle": "off", "test.v": "true"}
	for name, want := range wanted {
		f := flag.Lookup(name)
		r.check(f != nil && f.Value.String() == want, "exact test flags", nil)
	}
	flag.Visit(func(f *flag.Flag) { _, allowed := wanted[f.Name]; r.check(allowed, "only reviewed flags", nil) })
	build, ok := debug.ReadBuildInfo()
	r.check(ok, "build information", nil)
	settings := map[string]string{}
	for _, setting := range build.Settings {
		settings[setting.Key] = setting.Value
	}
	r.check(settings["CGO_ENABLED"] == "0" && settings["GOOS"] == "linux" && settings["-tags"] == "manual_encrypted_root_negative", "exact manual build profile", nil)
	binary, err := os.Executable()
	r.check(err == nil, "binary identity", err)
	_, binaryFact := r.read(binary, 128<<20, false)
	r.receipt.BinarySHA256, r.receipt.ExpectedBinarySHA256 = binaryFact.SHA256, os.Getenv("GOSQLITE_MANUAL_ENCRYPTED_ROOT_BINARY_SHA256")
	r.check(encryptedroot.Hash(r.receipt.ExpectedBinarySHA256) && r.receipt.BinarySHA256 == r.receipt.ExpectedBinarySHA256, "approved binary hash", nil)
	root := os.Getenv("GOSQLITE_MANUAL_ENCRYPTED_ROOT_ROOT")
	r.check(filepath.IsAbs(root) && filepath.Clean(root) == root && root != "/", "known fixture root", nil)
	resolved, err := filepath.EvalSymlinks(root)
	r.check(err == nil && resolved == root, "root has no symlink components", err)
	r.receipt.UID, r.receipt.Root = uint32(os.Geteuid()), r.directory(root)
	run, err := os.MkdirTemp(root, "root-negative-")
	r.check(err == nil, "fresh retained run directory", err)
	t.Logf("ROOT_AUTH_RETAINED_DIRECTORY %s", run)
	r.receipt.Run = r.directory(run)
	original := r.makeDirectory(run, "original")
	candidate := r.makeDirectory(run, "candidate")
	r.receipt.Original, r.receipt.Candidate = r.directory(original), r.directory(candidate)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r.seed = &rootAuthProvider{}
	seedOptions := rootAuthOptions(r.seed)
	seedDB, err := sqlite.OpenContext(ctx, filepath.Join(original, encryptedroot.RootName), sqlite.Options{Encryption: &seedOptions, PageSize: 512})
	seedOwner := r.capture("db", 0, seedDB, nil, nil, err)
	r.check(err == nil && seedDB != nil, "create healthy isolated seed", err)
	_, err = seedDB.Exec(ctx, "CREATE TABLE entries(id INTEGER PRIMARY KEY,body TEXT)")
	r.check(err == nil, "seed table", err)
	_, err = seedDB.Exec(ctx, "INSERT INTO entries VALUES(-7,'alpha'),(2,?),(9,'omega')", rootAuthMarker)
	r.check(err == nil, "seed three literal rows", err)
	r.receipt.Seed = r.ledger(ctx, seedOwner)
	err = r.closeOwner(seedOwner)
	r.check(err == nil, "seed retired before copy", err)
	var originalBytes map[string][]byte
	r.receipt.OriginalBefore, originalBytes = r.snapshot(original)
	rootBytes := originalBytes[encryptedroot.RootName]
	r.check(len(rootBytes) == encryptedroot.RootBytes, "exact layout1 root extent", nil)
	header, err := encryption.DecodeHeader(rootBytes[:512], seedOptions.Limits)
	r.check(err == nil && header.Major == 1 && header.Descriptor.Role == encryption.FileRoot && header.Epoch.Kind == encryption.RawKey256 && header.Descriptor.PageSize == 512 && header.Descriptor.PageCount == 2 && header.Descriptor.RecordCount == 1 && header.Descriptor.CommitSequence == 3, "healthy root framing", err)
	record, err := encryption.DecodeRecordHeader(rootBytes[512:536], seedOptions.Limits)
	r.check(err == nil && record.Role == encryption.RecordMetadata && record.Sequence == 0 && record.PlaintextSize == 128, "metadata exact framing", err)
	r.receipt.SaltBeforeNonzero = !bytes.Equal(rootBytes[144:176], make([]byte, 32))
	for _, member := range r.receipt.OriginalBefore {
		data := bytes.Clone(originalBytes[member.Name])
		if member.Name == encryptedroot.RootName && plan.Offset >= 0 {
			data[plan.Offset] ^= plan.XOR
		}
		r.writeCopy(filepath.Join(candidate, member.Name), data)
	}
	var candidateBytes map[string][]byte
	r.receipt.CandidateBefore, candidateBytes = r.snapshot(candidate)
	changedRoot := candidateBytes[encryptedroot.RootName]
	r.check(len(changedRoot) == encryptedroot.RootBytes, "candidate root extent", nil)
	r.receipt.SaltAfterNonzero = !bytes.Equal(changedRoot[144:176], make([]byte, 32))
	for _, member := range r.receipt.OriginalBefore {
		before, after := originalBytes[member.Name], candidateBytes[member.Name]
		r.check(len(before) == len(after), "copy exact extent", nil)
		for i := 0; i < len(before); i++ {
			if before[i] != after[i] {
				r.receipt.ChangedBytes++
				r.receipt.ChangedOffset, r.receipt.ChangedXOR = i, before[i]^after[i]
				r.check(member.Name == encryptedroot.RootName && i == plan.Offset && before[i]^after[i] == plan.XOR, "only selected root byte changed", nil)
			}
		}
	}
	wantedChanges := 0
	if plan.Offset >= 0 {
		wantedChanges = 1
	}
	r.check(r.receipt.ChangedBytes == wantedChanges && r.receipt.SaltBeforeNonzero && r.receipt.SaltAfterNonzero, "mutation complete before open", nil)
	r.candidate = &rootAuthProvider{wrong: plan.WrongKey}
	candidateOptions := rootAuthOptions(r.candidate)
	r.receipt.OpenCalls++
	db, openErr := sqlite.OpenContext(ctx, filepath.Join(candidate, encryptedroot.RootName), sqlite.Options{Encryption: &candidateOptions, ReadOnly: true, NoCreate: true})
	candidateOwner := r.capture("db", 0, db, nil, nil, openErr)
	r.receipt.DBReturned = db != nil
	r.receipt.Class, r.receipt.Code, r.receipt.CausePreserved = rootAuthClass(openErr)
	if plan.Case == "control" {
		r.check(openErr == nil && db != nil, "correct key control open", openErr)
		r.receipt.QueryCalls++
		r.receipt.Control = r.ledger(ctx, candidateOwner)
	} else {
		// No query, Verify, repair, alternate key or second Open on negatives.
		r.check(openErr != nil && db == nil && r.receipt.Class == plan.Class && r.receipt.Code == plan.Code && r.receipt.CausePreserved, "exact fail-closed public error", nil)
	}
	if candidateOwner.state.Present {
		err = r.closeOwner(candidateOwner)
		r.check(err == nil, "candidate retired", err)
	}
	r.receipt.OriginalAfter, _ = r.snapshot(original)
	r.receipt.CandidateAfter, _ = r.snapshot(candidate)
	r.check(r.receipt.Root == r.directory(root) && r.receipt.Run == r.directory(run) && r.receipt.Original == r.directory(original) && r.receipt.Candidate == r.directory(candidate), "all retained directories unchanged", nil)
	r.receipt.Status = "observed"
}
