//go:build linux && manual_backup_publication

// Package backupmanual is a deliberately narrow, test-only manual qualification
// boundary. It retains all synthetic fixtures on success and failure. It is not
// a filesystem sandbox, fault harness, platform certificate, or cleanup runner.
package backupmanual

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"golang.org/x/sys/unix"
)

const (
	profileName = "linux-ext-family-unique-mount-openat2-noreplace-v1"
	trustedNamespace = "stable-private-disposable-no-replacement-no-remount-compatible-locks-storage"
	statxMask = unix.STATX_TYPE | unix.STATX_NLINK | unix.STATX_INO | unix.STATX_SIZE | unix.STATX_MNT_ID_UNIQUE
	planLimit = 1 << 20
	outputLimit = 1 << 20
	evidenceLimit = 16 << 20
	live = "LIVE"
	endAttempted = "END_ATTEMPTED"
	endedOK = "ENDED_OK"
	retainedUnknown = "RETAINED_UNKNOWN"
)

// No initializer acquires a native resource. Retention lasts until natural
// process exit; OS release then occurs and is not a perpetual lock guarantee.
var process struct {
	sync.Mutex
	frozen bool
	runs []*Run
}

type fact struct {
	Mask uint32 `json:"mask"`
	Mode uint16 `json:"mode"`
	Links uint32 `json:"links"`
	Major uint32 `json:"device_major"`
	Minor uint32 `json:"device_minor"`
	Inode uint64 `json:"inode"`
	Mount uint64 `json:"unique_mount_id"`
	Size uint64 `json:"size"`
}
type fdOwner struct {
	fd int
	path string
	state string
	identity fact
	err error
}
type owner struct {
	id string
	parentID string
	kind string
	value any
	parent *sqlite.DB
	state string
	err error
}
type Run struct {
	t *testing.T
	root string
	input manifest
	fds []*fdOwner
	rootFact fact
	scopes []*Scope
	errors []error
	finished bool
	frozen bool
	firstFailure string
}
type Scope struct {
	run *Run
	t *testing.T
	label string
	fixture string
	evidence string
	dirs map[string]string
	fds []*fdOwner
	log *fdOwner
	owners []*owner
	errors []error
	oracleObjects []any
	planned bool
	finished bool
	frozen bool
	written int
	lastStage string
}

func cleanAbsolute(value string) bool {
	return value != "" && filepath.IsAbs(value) && filepath.Clean(value) == value && !strings.ContainsAny(value, "\\\x00")
}
func lowerHex(value string, n int) bool {
	if len(value) != n || strings.ToLower(value) != value { return false }
	_, err := hex.DecodeString(value)
	return err == nil
}
func fail(t *testing.T, message string) {
	t.Helper()
	t.Fatalf("%s", message)
}
func (r *Run) freeze(err error) {
	r.frozen = true
	for _, fd := range r.fds { if fd.state == live || fd.state == endAttempted { fd.state = retainedUnknown } }
	for _, scope := range r.scopes {
		scope.frozen = true
		for _, o := range scope.owners { if o.state == live || o.state == endAttempted { o.state = retainedUnknown } }
	}
	if err != nil { r.errors = append(r.errors, err) }
	process.Lock()
	process.frozen = true
	process.Unlock()
}
func (r *Run) blocked(stage string, err error) {
	if _, ok := err.(profileFailure); ok { stage = "PROFILE_UNAVAILABLE: " + stage }
	if r.firstFailure == "" { r.firstFailure = stage }
	r.freeze(err)
	// Never call arbitrary error Error/Unwrap methods merely to log evidence.
	t := r.t
	if len(r.scopes) != 0 && !r.scopes[len(r.scopes)-1].finished { t = r.scopes[len(r.scopes)-1].t }
	fail(t, "BLOCKED: " + stage + "; retained error type " + fmt.Sprintf("%T", err))
}
func (r *Run) active() {
	process.Lock()
	frozen := process.frozen
	process.Unlock()
	if frozen || r.frozen || r.finished { r.blocked("process or run frozen/finished; no later native case", nil) }
	pending := sqlite.PendingPlaintextCleanups()
	if len(pending) != 0 {
		r.errors = append(r.errors, pending...)
		r.blocked("historical plaintext cleanup token", nil)
	}
}

// ForTest is the first effect in each of the six approved entry functions.
// An env acknowledgement prevents accidental selection; it grants no authority.
func ForTest(t *testing.T) *Run {
	t.Helper()
	if os.Getenv(ackKey) != "1" { fail(t, "MANUAL_SELECTION_REFUSED: explicit acknowledgement must equal 1") }
	root := os.Getenv(rootKey)
	if !cleanAbsolute(root) || root == "/" || root == "/tmp" { fail(t, "MANUAL_SELECTION_REFUSED: explicit private absolute clean root required") }
	r := &Run{t: t, root: root}
	process.Lock()
	refused := process.frozen || len(process.runs) != 0
	process.runs = append(process.runs, r)
	process.Unlock()
	t.Cleanup(func() {
		if t.Failed() || !r.finished { r.freeze(nil) }
		t.Logf("backup manual retained root=%s; natural process exit and saved evidence are required before user cleanup", r.root)
		t.Logf("final metadata ledger (stdout; persist externally): %s", r.ledgerJSON())
	})
	if refused { r.blocked("one exact top-level entry per process", nil) }
	r.active()
	cwd, err := os.Getwd()
	if err != nil { r.blocked("Getwd", err) }
	if cwd != filepath.Join(root, "work") || os.Getenv("GOTMPDIR") != filepath.Join(root, "tmp") || os.Getenv("TMPDIR") != filepath.Join(root, "tmp") {
		r.blocked("MANUAL_SELECTION_REFUSED: CWD=root/work and both temp variables=root/tmp required", nil)
	}
	// These lexical checks are not an adversarial namespace proof. All path
	// ancestors must remain stable under the externally reviewed contract.
	for path := root; ; path = filepath.Dir(path) {
		info, e := os.Lstat(path)
		if e != nil { r.blocked("root ancestor lstat", e) }
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 { r.blocked("root ancestor is not a real directory", nil) }
		if path == "/" { break }
	}
	rootFD := r.openDirectory(root, nil)
	r.rootFact = rootFD.identity
	for _, role := range []string{"work", "tmp", "fixtures", "evidence"} {
		r.openDirectory(filepath.Join(root, role), &r.rootFact)
	}
	r.input = r.loadManifest()
	r.verifyRuntime(cwd)
	return r
}

type profileFailure struct{}
func (profileFailure) Error() string { return "PROFILE_UNAVAILABLE: required returned unique statx identity fields missing" }
func statFact(fd int, path string, flags int) (fact, error) {
	var st unix.Statx_t
	if err := unix.Statx(fd, path, flags, statxMask, &st); err != nil { return fact{}, err }
	if st.Mask&statxMask != statxMask || st.Ino == 0 || st.Mnt_id == 0 || st.Nlink == 0 { return fact{}, profileFailure{} }
	return fact{st.Mask, st.Mode, st.Nlink, st.Dev_major, st.Dev_minor, st.Ino, st.Mnt_id, st.Size}, nil
}
func sameIdentity(a, b fact) bool {
	return a.Major == b.Major && a.Minor == b.Minor && a.Inode == b.Inode && a.Mount == b.Mount && a.Mode&unix.S_IFMT == b.Mode&unix.S_IFMT
}
func sameMount(a, b fact) bool { return a.Major == b.Major && a.Minor == b.Minor && a.Mount == b.Mount }
func (r *Run) openDirectory(path string, root *fact) *fdOwner {
	o := &fdOwner{fd: -1, path: path, state: live}
	if len(r.fds) >= 256 { r.blocked("metadata descriptor count limit", nil) }
	r.fds = append(r.fds, o) // install owner before acquisition
	var err error
	o.fd, err = unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil { o.err = err; o.state = retainedUnknown; r.blocked("open directory", err) }
	var st unix.Statfs_t
	if err = unix.Fstatfs(o.fd, &st); err != nil { r.blocked("directory Fstatfs", err) }
	if st.Type != unix.EXT4_SUPER_MAGIC { r.blocked("PROFILE_UNAVAILABLE: ext-family magic required; tmpfs/overlay unsupported", nil) }
	if st.Bsize <= 0 || st.Bavail < uint64((256 << 20)/st.Bsize) || st.Ffree < 1000 { r.blocked("PROFILE_UNAVAILABLE: at least 256 MiB and 1000 inodes required", nil) }
	held, err := statFact(o.fd, "", unix.AT_EMPTY_PATH|unix.AT_SYMLINK_NOFOLLOW)
	if err != nil { r.blocked("directory held statx", err) }
	named, err := statFact(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW)
	if err != nil { r.blocked("directory named statx", err) }
	var ownerStat unix.Stat_t
	if err = unix.Fstat(o.fd, &ownerStat); err != nil { r.blocked("directory Fstat", err) }
	if !sameIdentity(held, named) || held.Mode&unix.S_IFMT != unix.S_IFDIR || held.Mode&0777 != 0700 || ownerStat.Uid != uint32(os.Geteuid()) {
		r.blocked("directory identity/private owner/mode mismatch", nil)
	}
	if root != nil && !sameMount(held, *root) { r.blocked("PROFILE_UNAVAILABLE: different device or unique mount", nil) }
	o.identity = held
	return o
}
func (r *Run) closeFD(o *fdOwner) error {
	if r.frozen || o.state != live { return fmt.Errorf("descriptor end refused") }
	o.state = endAttempted
	err := unix.Close(o.fd)
	if err != nil { o.state = retainedUnknown; o.err = err; r.freeze(err); return err }
	o.state = endedOK
	return nil
}
func (r *Run) openRegular(path string, flags int, size int64) *fdOwner {
	if !cleanAbsolute(path) { r.blocked("non-absolute file input", nil) }
	info, err := os.Lstat(path)
	if flags&unix.O_CREAT == 0 && (err != nil || !info.Mode().IsRegular()) { r.blocked("regular named file required", err) }
	o := &fdOwner{fd: -1, path: path, state: live}
	if len(r.fds) >= 256 { r.blocked("metadata descriptor count limit", nil) }
	r.fds = append(r.fds, o)
	o.fd, err = unix.Open(path, flags|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0600)
	if err != nil { o.state = retainedUnknown; o.err = err; r.blocked("file open", err) }
	var st unix.Stat_t
	if err = unix.Fstat(o.fd, &st); err != nil { r.blocked("file Fstat", err) }
	if st.Mode&unix.S_IFMT != unix.S_IFREG || st.Nlink != 1 || st.Size < 0 || (size >= 0 && st.Size != size) { r.blocked("file type/link/size mismatch", nil) }
	// External binary pins may be on another admitted immutable read-only
	// location; root-local sidecars/evidence must share the qualified profile.
	if strings.HasPrefix(path, r.root + "/") {
		held, e := statFact(o.fd, "", unix.AT_EMPTY_PATH|unix.AT_SYMLINK_NOFOLLOW)
		if e != nil { r.blocked("local file statx", e) }
		named, e := statFact(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW)
		if e != nil || !sameIdentity(held, named) || !sameMount(held, r.rootFact) { r.blocked("local file identity/profile", e) }
		o.identity = held
	}
	return o
}
func (r *Run) readExact(path string, limit int64, expected *filePin) []byte {
	if expected != nil && (expected.Path != path || expected.Size < 1 || expected.Size > limit || !lowerHex(expected.SHA256, 64)) { r.blocked("invalid fixed file pin", nil) }
	size := int64(-1)
	if expected != nil { size = expected.Size }
	o := r.openRegular(path, unix.O_RDONLY, size)
	var before unix.Stat_t
	if err := unix.Fstat(o.fd, &before); err != nil { r.blocked("read pre-stat", err) }
	if before.Size > limit { r.blocked("read quota", nil) }
	out := make([]byte, 0, int(before.Size))
	buf := make([]byte, 32 << 10)
	for {
		n, err := unix.Read(o.fd, buf)
		if err != nil { r.blocked("read", err) }
		if n == 0 { break }
		if int64(len(out)+n) > limit { r.blocked("read quota exceeded", nil) }
		out = append(out, buf[:n]...)
	}
	r.verifyFileContinuity(o, before)
	if int64(len(out)) != before.Size { r.blocked("file size changed", nil) }
	if expected != nil && fmt.Sprintf("%x", sha256.Sum256(out)) != expected.SHA256 { r.blocked("file hash mismatch", nil) }
	if err := r.closeFD(o); err != nil { r.blocked("file close", err) }
	return out
}
func (r *Run) verifyFileContinuity(o *fdOwner, before unix.Stat_t) {
	var after, named unix.Stat_t
	if err := unix.Fstat(o.fd, &after); err != nil { r.blocked("file post-stat", err) }
	if err := unix.Lstat(o.path, &named); err != nil { r.blocked("file named post-stat", err) }
	if before.Dev != after.Dev || before.Ino != after.Ino || before.Size != after.Size || before.Mtim != after.Mtim || before.Ctim != after.Ctim || after.Dev != named.Dev || after.Ino != named.Ino || after.Size != named.Size || after.Mode != named.Mode || after.Nlink != 1 {
		r.blocked("file changed while observing", nil)
	}
}
func (r *Run) hashBinary(pin filePin) {
	if !cleanAbsolute(pin.Path) || !lowerHex(pin.SHA256, 64) || pin.Size <= 0 || pin.Size > 512 << 20 { r.blocked("invalid executable pin", nil) }
	o := r.openRegular(pin.Path, unix.O_RDONLY, pin.Size)
	var before unix.Stat_t
	if err := unix.Fstat(o.fd, &before); err != nil { r.blocked("binary pre-stat", err) }
	if before.Mode&0111 == 0 { r.blocked("binary is not executable", nil) }
	h := sha256.New()
	var nread int64
	buf := make([]byte, 32 << 10)
	for {
		n, err := unix.Read(o.fd, buf)
		if err != nil { r.blocked("binary read", err) }
		if n == 0 { break }
		nread += int64(n)
		if nread > pin.Size { r.blocked("binary read quota", nil) }
		_, _ = h.Write(buf[:n])
	}
	r.verifyFileContinuity(o, before)
	if nread != pin.Size || hex.EncodeToString(h.Sum(nil)) != pin.SHA256 { r.blocked("binary hash mismatch", nil) }
	if err := r.closeFD(o); err != nil { r.blocked("binary close", err) }
}

func (r *Run) loadManifest() manifest {
	base := filepath.Join(r.root, "evidence")
	raw := r.readExact(filepath.Join(base, "input-manifest.json"), manifestLimit, nil)
	digest := r.readExact(filepath.Join(base, "input-manifest.sha256"), 65, nil)
	if len(digest) == 65 && digest[64] == '\n' { digest = digest[:64] }
	if !lowerHex(string(digest), 64) || string(digest) != fmt.Sprintf("%x", sha256.Sum256(raw)) { r.blocked("manifest raw digest mismatch", nil) }
	m, err := decodeManifest(raw)
	if err != nil { r.blocked("manifest exact schema", err) }
	if m.Schema != 1 || m.Root != r.root || m.Profile != profileName || m.Namespace != trustedNamespace || !lowerHex(m.SourceTree, 40) || m.GoVersion != "go1.27.2" || m.GOARCH != runtime.GOARCH {
		r.blocked("manifest required provenance fields mismatch", nil)
	}
	for _, item := range []struct{ pin filePin; name string; limit int64 }{
		{m.SourceInventory, "source-inventory.json", 8 << 20},
		{m.ToolchainInventory, "toolchain-inventory.json", 16 << 20},
		{m.ModuleInventory, "module-inventory.json", 32 << 20},
		{m.GoMod, "go.mod", 64 << 10},
		{m.GoSum, "go.sum", 1 << 20},
	} {
		r.readExact(filepath.Join(base, item.name), item.limit, &item.pin)
	}
	if len(m.Modules) == 0 || len(m.Modules) > 16 { r.blocked("manifest module count", nil) }
	seen := map[string]bool{}
	for _, mod := range m.Modules {
		if seen[mod.Path] || !strings.HasPrefix(mod.Sum, "h1:") || len(mod.Sum) != 47 { r.blocked("manifest module checksum/duplicate", nil) }
		sum, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(mod.Sum, "h1:"))
		if err != nil || len(sum) != 32 { r.blocked("manifest invalid h1", err) }
		seen[mod.Path] = true
		want := map[string]string{"golang.org/x/crypto":"v0.58.0", "golang.org/x/sys":"v0.49.0", "golang.org/x/text":"v0.43.0"}
		if want[mod.Path] != mod.Version { r.blocked("manifest unexpected module", nil) }
	}
	if len(seen) != 3 { r.blocked("manifest incomplete declared module pins", nil) }
	return m
}
func (r *Run) verifyRuntime(cwd string) {
	m := r.input
	oracle := r.t.Name() == "TestPlaintextBackupOracleOrdinaryInteroperability"
	tags := "manual_backup_publication"
	if oracle { tags = "oracle,manual_backup_publication" }
	selector := "^" + r.t.Name() + "$"
	if m.Selector != selector || m.BuildTags != tags || runtime.GOOS != "linux" || runtime.Version() != m.GoVersion { r.blocked("runtime selection/version mismatch", nil) }
	for key, want := range map[string]string{"test.run":selector,"test.count":"1","test.failfast":"true","test.parallel":"1","test.shuffle":"off","test.timeout":"0s","test.v":"true"} {
		f := flag.Lookup(key)
		if f == nil || f.Value.String() != want { r.blocked("test flag " + key, nil) }
	}
	expectedArgv := []string{m.Executable.Path,"-test.run",selector,"-test.v","-test.count=1","-test.failfast","-test.parallel=1","-test.shuffle=off","-test.timeout=0"}
	if !reflect.DeepEqual(m.RuntimeArgv, expectedArgv) || !reflect.DeepEqual(os.Args, expectedArgv) { r.blocked("exact runtime argv mismatch", nil) }
	expectedEnv := map[string]string{ackKey:"1",rootKey:r.root,"GOTMPDIR":filepath.Join(r.root,"tmp"),"TMPDIR":filepath.Join(r.root,"tmp"),"LC_ALL":"C","LANG":"C","TZ":"UTC","GOMAXPROCS":"2"}
	if oracle {
		if !strings.HasPrefix(r.root, "/tmp/") || m.Oracle == nil || m.Oracle.Version != "3.54.0" || m.Oracle.SourceID != "2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3" { r.blocked("oracle /tmp profile or provenance missing", nil) }
		expectedEnv["SQLITE_ORACLE"] = m.Oracle.Binary.Path
	} else if m.Oracle != nil || os.Getenv("SQLITE_ORACLE") != "" { r.blocked("unexpected oracle input", nil) }
	if !reflect.DeepEqual(m.RuntimeEnv, expectedEnv) { r.blocked("runtime env declaration is not exact allowlist", nil) }
	for key, want := range expectedEnv { if os.Getenv(key) != want { r.blocked("runtime allowlist value " + key, nil) } }
	if cwd != filepath.Join(m.Root,"work") { r.blocked("runtime CWD", nil) }
	buildPackage := "./sqlite"
	if r.t.Name() == "TestCLIPlainBackupOrdinaryRoundTrip" { buildPackage = "./cmd/gosqlite" }
	expectedBuild := []string{filepath.Join(m.BuildEnv["GOROOT"],"bin","go"),"test","-c","-mod=readonly","-buildvcs=false","-p=1","-tags",tags,"-o",m.Executable.Path,buildPackage}
	if !reflect.DeepEqual(m.BuildArgv, expectedBuild) { r.blocked("exact build argv declaration", nil) }
	buildKeys := map[string]bool{"PATH":true,"GOROOT":true,"GOPATH":true,"GOMODCACHE":true,"GOCACHE":true,"GOTMPDIR":true,"TMPDIR":true,"CGO_ENABLED":true,"GOTOOLCHAIN":true,"GOENV":true,"GOWORK":true,"GOFLAGS":true,"GOPROXY":true,"GOSUMDB":true,"GOOS":true,"GOARCH":true,"GOAMD64":true,"GOMAXPROCS":true}
	for key := range m.BuildEnv { if !buildKeys[key] { r.blocked("unapproved build env key", nil) } }
	for key, value := range map[string]string{"CGO_ENABLED":"0","GOTOOLCHAIN":"local","GOENV":"off","GOWORK":"off","GOFLAGS":"","GOPROXY":"off","GOSUMDB":"off","GOOS":"linux","GOARCH":runtime.GOARCH,"GOMAXPROCS":"2"} {
		actual, ok := m.BuildEnv[key]
		if !ok || actual != value { r.blocked("build env required value", nil) }
	}
	for _, key := range []string{"GOROOT","GOPATH","GOMODCACHE","GOCACHE","GOTMPDIR","TMPDIR"} { if !cleanAbsolute(m.BuildEnv[key]) { r.blocked("build env absolute path", nil) } }
	if m.BuildEnv["PATH"] != filepath.Join(m.BuildEnv["GOROOT"],"bin") || m.BuildEnv["GOTMPDIR"] != m.BuildEnv["TMPDIR"] { r.blocked("build PATH/temp declaration mismatch", nil) }
	if runtime.GOARCH == "amd64" && m.BuildEnv["GOAMD64"] != "v1" { r.blocked("build architecture feature mismatch", nil) }
	bi, ok := debug.ReadBuildInfo()
	if !ok || bi.GoVersion != m.GoVersion { r.blocked("METADATA_UNAVAILABLE: runtime BuildInfo", nil) }
	if bi.Main.Path != "github.com/ms-xiaosnow/gosqlite" || bi.Main.Replace != nil { r.blocked("METADATA_MISMATCH: main module", nil) }
	settings := map[string]string{}
	allowed := map[string]bool{"-compiler":true,"-buildmode":true,"-tags":true,"-race":true,"-trimpath":true,"CGO_ENABLED":true,"GOOS":true,"GOARCH":true,"GOAMD64":true,"GOARM64":true}
	for _, setting := range bi.Settings { if allowed[setting.Key] { settings[setting.Key] = setting.Value } }
	for key, want := range map[string]string{"CGO_ENABLED":"0","GOOS":"linux","GOARCH":m.GOARCH,"-compiler":"gc","-buildmode":"exe"} {
		if settings[key] != want { r.blocked("METADATA_MISMATCH: BuildInfo " + key, nil) }
	}
	if settings["-tags"] != tags || settings["-race"] != "" || settings["-trimpath"] != "" { r.blocked("METADATA_MISMATCH: BuildInfo tags/race/trimpath", nil) }
	if runtime.GOARCH == "amd64" && settings["GOAMD64"] != "v1" { r.blocked("METADATA_MISMATCH: architecture feature level", nil) }
	for _, dep := range bi.Deps {
		matched := false
		for _, pin := range m.Modules { if dep.Path == pin.Path && dep.Version == pin.Version && dep.Sum == pin.Sum && dep.Replace == nil { matched = true } }
		if !matched { r.blocked("METADATA_MISMATCH: linked dependency", nil) }
	}
	executable, err := os.Executable()
	if err != nil || executable != m.Executable.Path { r.blocked("executable path mismatch", err) }
	r.hashBinary(m.Executable)
	if oracle { r.hashBinary(m.Oracle.Binary) }
	// Path hash is a stable-namespace cross-check, not proof of the loaded image.
	// VCS settings may be absent; none is substituted for the external tree pin.
	observed := map[string]any{"declared_input":m,"observed_runtime":map[string]any{"go_version":runtime.Version(),"goos":runtime.GOOS,"goarch":runtime.GOARCH,"cwd":cwd,"allowlist_env":expectedEnv,"settings":settings,"main":bi.Main,"dependencies":bi.Deps,"executable_path":executable,"loaded_image_proven":false,"root_fact":r.rootFact}}
	r.writeNew(filepath.Join(r.root,"evidence","observed-runtime.json"), observed, manifestLimit)
}
func (r *Run) writeNew(path string, value any, limit int) {
	raw, err := json.Marshal(value)
	if err != nil || len(raw)+1 > limit { r.blocked("evidence serialization/limit", err) }
	o := r.openRegular(path, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL, 0)
	raw = append(raw,'\n')
	if n, err := unix.Write(o.fd, raw); err != nil || n != len(raw) { r.blocked("evidence write", err) }
	if err := r.closeFD(o); err != nil { r.blocked("evidence close", err) }
}

func (r *Run) Case(t *testing.T, label string) *Scope {
	t.Helper()
	r.active()
	if label == "" || len(label) > 100 || strings.ContainsAny(label,"/\\\x00") { r.blocked("invalid case label", nil) }
	if len(r.scopes) >= 4 { r.blocked("case count limit", nil) }
	for _, previous := range r.scopes { if !previous.finished { r.blocked("previous case incomplete", nil) } }
	// Exclusive mkdir, no reuse, retry, hidden database, or fallback root.
	name := strconv.Itoa(len(r.scopes)+1) + "-" + label
	s := &Scope{run:r,t:t,label:label,fixture:filepath.Join(r.root,"fixtures",name),evidence:filepath.Join(r.root,"evidence",name),dirs:map[string]string{}}
	r.scopes = append(r.scopes,s)
	t.Cleanup(func() {
		if t.Failed() || !s.finished {
			if r.firstFailure=="" { r.firstFailure="assertion or premature exit after "+s.lastStage }
			s.frozen=true; r.freeze(nil)
		}
		t.Logf("retained fixture=%s evidence=%s",s.fixture,s.evidence)
		t.Logf("final owner ledger (stdout; persist externally): %s",s.ledgerJSON())
	})
	for _, path := range []string{s.fixture,s.evidence} {
		if err := unix.Mkdir(path,0700); err != nil { r.blocked("exclusive case directory",err) }
		s.fds = append(s.fds,r.openDirectory(path,&r.rootFact))
	}
	s.log = r.openRegular(filepath.Join(s.evidence,"events.jsonl"),unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL,0)
	return s
}
func (s *Scope) Dir(role string) string {
	s.run.active()
	if s.planned || role=="" || len(role)>40 || strings.ContainsAny(role,"/\\\x00") || role=="." || role==".." { s.run.blocked("invalid or late fixture role",nil) }
	if path,ok:=s.dirs[role];ok { return path }
	path:=filepath.Join(s.fixture,role)
	if err:=unix.Mkdir(path,0700);err!=nil { s.run.blocked("fixture role directory",err) }
	s.fds=append(s.fds,s.run.openDirectory(path,&s.run.rootFact))
	s.dirs[role]=path
	return path
}
// FixtureDir avoids the launch-field name Dir in the strictly guarded oracle caller.
func (s *Scope) FixtureDir(role string) string { return s.Dir(role) }
func (s *Scope) SetPlan(plan any) {
	s.run.active()
	if s.planned { s.run.blocked("plan may not be replaced",nil) }
	s.run.writeNew(filepath.Join(s.evidence,"input-plan.json"),map[string]any{"case":s.label,"roles":s.dirs,"plan":plan},planLimit)
	s.planned=true
	s.Record("plan_saved",map[string]any{"fixture":s.fixture,"evidence":s.evidence})
}
func (s *Scope) Before(stage string) {
	s.t.Helper()
	s.run.active()
	if !s.planned || s.finished || s.frozen { s.run.blocked("case not active/planned: "+stage,nil) }
	s.lastStage=stage
	s.Record("before",stage)
}
func (s *Scope) Record(stage string, result any) {
	if s.frozen || s.run.frozen || s.finished { s.run.blocked("record after freeze/end",nil) }
	if err,ok:=result.(error);ok && err!=nil {
		s.errors=append(s.errors,err)
		details:=safeError(err)
		if limit,ok:=err.(*outputLimitError);ok { details["total_bytes"]=limit.total;details["full_stream_sha256"]=limit.digest;details["output_complete"]=false }
		result=details
	}
	raw,err:=json.Marshal(map[string]any{"stage":stage,"result":result})
	if err!=nil || len(raw)>planLimit || s.written+len(raw)+1>evidenceLimit { s.run.blocked("event serialization/limit",err) }
	raw=append(raw,'\n')
	if n,err:=unix.Write(s.log.fd,raw);err!=nil || n!=len(raw) { s.run.blocked("event write",err) }
	s.written+=len(raw)
}
// RetainErrors is memory-only so a later evidence failure cannot discard a
// simultaneous native error/cleanup token. Call before any fallible reporting.
func (s *Scope) RetainErrors(errs ...error) {
	for _, err := range errs { if err != nil { s.errors=append(s.errors,err) } }
}
func (s *Scope) Observe(stage string, result any, err error) {
	s.RetainErrors(err)
	s.Record(stage,result)
	if err!=nil { s.Record(stage+" error",err);s.frozen=true;s.run.blocked(stage,err) }
}
func (s *Scope) own(kind string,value any,parent *sqlite.DB) {
	if value==nil { s.run.blocked("nil owner",nil) }
	for _,o:=range s.owners { if o.value==value { s.run.blocked("duplicate owner",nil) } }
	if len(s.owners)>=128 { s.run.blocked("owner count limit",nil) }
	parentID:=""
	if parent!=nil {
		p:=s.find(parent)
		if p.state!=live { s.run.blocked("child of non-live DB",nil) }
		parentID=p.id
	}
	o:=&owner{id:strconv.Itoa(len(s.owners)+1),parentID:parentID,kind:kind,value:value,parent:parent,state:live}
	s.owners=append(s.owners,o)
	s.Record("owner acquired",ownerEvidence(o))
}
func (s *Scope) OwnDB(db *sqlite.DB) { s.own("DB",db,nil) }
func (s *Scope) OwnTx(db *sqlite.DB,tx *sqlite.Tx) { s.own("Tx",tx,db) }
func (s *Scope) OwnRows(db *sqlite.DB,rows *sqlite.Rows) { s.own("Rows",rows,db) }
func (s *Scope) find(value any) *owner {
	for _,o:=range s.owners { if o.value==value { return o } }
	s.run.blocked("unregistered owner",nil)
	return nil
}
func (s *Scope) beginEnd(value any) *owner {
	s.Before("owner end")
	o:=s.find(value)
	if o.state!=live { s.run.blocked("owner already end-attempted",nil) }
	if db,ok:=value.(*sqlite.DB);ok {
		for _,child:=range s.owners { if child.parent==db && child.state!=endedOK { s.run.blocked("parent close with unfinished child",nil) } }
	}
	o.state=endAttempted // must precede the actual end operation, even on error
	s.Record("owner END_ATTEMPTED",ownerEvidence(o))
	return o
}
func (s *Scope) end(o *owner,err error) error {
	if err!=nil { o.err=err;o.state=retainedUnknown;s.errors=append(s.errors,err);s.Record("owner retained unknown",err);s.frozen=true;s.run.blocked("owner end failed",err);return err }
	o.state=endedOK
	s.Record("owner ENDED_OK",ownerEvidence(o))
	return nil
}
func (s *Scope) CloseDB(db *sqlite.DB) error {
	o:=s.beginEnd(db)
	return s.end(o,db.Close())
}
func (s *Scope) CommitTx(ctx context.Context,tx *sqlite.Tx) error {
	o:=s.beginEnd(tx)
	return s.end(o,tx.Commit(ctx))
}
func (s *Scope) CloseRows(rows *sqlite.Rows) error {
	o:=s.find(rows)
	// Next(false) already auto-closes internally. An Err means that ending is
	// uncertain; do not call Close again or subsequently close the parent DB.
	if err:=rows.Err();err!=nil { o.state=retainedUnknown;o.err=err;s.errors=append(s.errors,err);s.frozen=true;s.run.blocked("Rows exhaustion end uncertain",err);return err }
	o=s.beginEnd(rows)
	return s.end(o,rows.Close())
}

// Only the reviewed clean existing-final preflight shape is accepted. The
// bounded graph permits known sqlite/stdlib wrappers, and the exact bounded
// ownership-hiding VFS facade. No arbitrary Unwrap/Is/As implementation runs.
func cleanExisting(input error) bool {
	stack:=[]error{input}
	found:=false
	for count:=0;len(stack)>0;count++ {
		if count>=16 { return false }
		err:=stack[len(stack)-1];stack=stack[:len(stack)-1]
		if err==nil { return false }
		if err==os.ErrExist { found=true;continue }
		if e,ok:=err.(*sqlite.Error);ok {
			if e.Op!="backup" || e.Code!=sqlite.CodeError { return false }
			stack=append(stack,e.Err);continue
		}
		t:=reflect.TypeOf(err)
		if t.Kind()!=reflect.Pointer { return false }
		name:=t.Elem().PkgPath()+"."+t.Elem().Name()
		switch name {
		case "errors.joinError":
			children:=err.(interface{Unwrap() []error}).Unwrap()
			if len(children)==0 || len(children)>16-count-len(stack) { return false }
			stack=append(stack,children...)
		case "github.com/ms-xiaosnow/gosqlite/internal/vfs.backupDiagnostic":
			// The facade is bounded by its producer; it has no exposed cleanup
			// owner. In this fixed source its exact clean leaf is os.ErrExist.
			if !errors.Is(err,os.ErrExist) || errors.Is(err,sqlite.ErrPoisoned) || errors.Is(err,sqlite.ErrPlaintextCleanupPending) || errors.Is(err,sqlite.ErrCommitUncertain) { return false }
			if err.Error()!=os.ErrExist.Error() { return false }
			found=true
		default:
			return false
		}
	}
	return found
}
func (s *Scope) AcceptExistingRefusal(err error) bool {
	s.Record("expected existing-target refusal",err)
	if !cleanExisting(err) || len(sqlite.PendingPlaintextCleanups())!=0 { s.frozen=true;s.run.freeze(err);return false }
	return true
}
func (s *Scope) OracleExecutable() string {
	s.Before("oracle executable already pinned")
	if s.run.input.Oracle==nil { s.run.blocked("oracle is not declared",nil) }
	return s.run.input.Oracle.Binary.Path
}
func (s *Scope) FinishSuccess() {
	s.Before("finish case")
	if s.t.Failed() { s.run.blocked("failed assertion",nil) }
	for _,o:=range s.owners { if o.state!=endedOK { s.run.blocked("unfinished native owner",o.err) } }
	s.Record("original_assertions_complete",map[string]any{"retention":"all fixtures and evidence retained","owner_count":len(s.owners),"owners":s.ownerEvidence()})
	if err:=s.run.closeFD(s.log);err!=nil { s.run.blocked("event log close",err) }
	for i:=len(s.fds)-1;i>=0;i-- { if err:=s.run.closeFD(s.fds[i]);err!=nil { s.run.blocked("case directory close",err) } }
	s.finished=true
	s.t.Logf("case PASS; retained fixture=%s evidence=%s",s.fixture,s.evidence)
}
func (r *Run) FinishSuccess() {
	r.active()
	if r.t.Failed() || len(r.scopes)==0 { r.blocked("run did not complete",nil) }
	for _,s:=range r.scopes { if !s.finished { r.blocked("case did not complete",nil) } }
	for i:=len(r.fds)-1;i>=0;i-- {
		o:=r.fds[i]
		if o.state==endedOK { continue }
		if err:=r.closeFD(o);err!=nil { r.blocked("run metadata close",err) }
	}
	r.finished=true
	r.t.Log("manual native assertions and normal owner ends complete; retained artifacts require user cleanup after natural process exit")
}

// Evidence rendering is bounded, memory-only, and never walks unknown error
// graphs. Raw pointer/error ownership stays strongly reachable in process.runs.
func safeError(err error) map[string]any {
	out:=map[string]any{"error_type":fmt.Sprintf("%T",err),"raw_error_retained":err!=nil}
	if e,ok:=err.(*sqlite.Error);ok && e!=nil { out["sqlite_code"]=int(e.Code);out["sqlite_operation"]=e.Op;out["cause_type"]=fmt.Sprintf("%T",e.Err) }
	if e,ok:=err.(syscall.Errno);ok { out["errno"]=int(e) }
	return out
}
func ownerEvidence(o *owner) map[string]any {
	return map[string]any{"id":o.id,"kind":o.kind,"parent_id":o.parentID,"state":o.state,"error":safeError(o.err)}
}
func (s *Scope) ownerEvidence() []any {
	out:=make([]any,0,len(s.owners))
	for _,o:=range s.owners { out=append(out,ownerEvidence(o)) }
	return out
}
func (s *Scope) ledgerJSON() string {
	errs:=make([]any,0,len(s.errors))
	for i,err:=range s.errors { if i>=256 { break };errs=append(errs,safeError(err)) }
	raw,_:=json.Marshal(map[string]any{"fixture":s.fixture,"case":s.label,"last_stage":s.lastStage,"first_failure":s.run.firstFailure,"finished":s.finished,"frozen":s.frozen,"owners":s.ownerEvidence(),"retained_oracle_objects":len(s.oracleObjects),"errors":errs,"event_evidence_may_be_incomplete":s.frozen})
	return string(raw)
}
func (r *Run) ledgerJSON() string {
	fds:=make([]any,0,len(r.fds))
	for i,o:=range r.fds { fds=append(fds,map[string]any{"id":i+1,"path":o.path,"state":o.state,"identity":o.identity,"error":safeError(o.err)}) }
	errs:=make([]any,0,len(r.errors))
	for i,err:=range r.errors { if i>=256 { break };errs=append(errs,safeError(err)) }
	raw,_:=json.Marshal(map[string]any{"root":r.root,"first_failure":r.firstFailure,"finished":r.finished,"frozen":r.frozen,"fds":fds,"errors":errs,"event_evidence_may_be_incomplete":r.frozen})
	return string(raw)
}

// SQL serializes original constants directly, without parsing/reinterpreting
// SQL. Integer kind and complete BLOB bytes survive JSON representation.
func SQL(query string,args ...any) any {
	bindings:=make([]map[string]any,0,len(args))
	for _,value:=range args {
		binding:=map[string]any{}
		switch v:=value.(type) {
		case nil: binding["type"]="null";binding["value"]=nil
		case int: binding["type"]="int";binding["value"]=strconv.Itoa(v)
		case int64: binding["type"]="int64";binding["value"]=strconv.FormatInt(v,10)
		case string: binding["type"]="string";binding["value"]=v
		case []byte: binding["type"]="blob_base64";binding["value"]=base64.StdEncoding.EncodeToString(v)
		default: panic("backupmanual SQL unsupported fixed binding type")
		}
		bindings=append(bindings,binding)
	}
	return map[string]any{"sql":query+"\n","bindings":bindings}
}

// Output consumes the entire normal combined stdout/stderr stream, retaining
// at most 1 MiB plus its complete stream hash/count. Overflow never returns a
// writer error or kills the child; Result refuses success after natural Wait.
type outputLimitError struct { total int64; digest string }
func (e *outputLimitError) Error() string { return fmt.Sprintf("OUTPUT_LIMIT: bytes=%d sha256=%s; output prefix incomplete",e.total,e.digest) }
type Output struct {
	mu sync.Mutex
	data []byte
	h hash.Hash
	total int64
}
func (s *Scope) OracleOutput() *Output { return &Output{h:sha256.New()} }
// HoldOracle retains caller-owned command/stdio objects without importing a
// process package, launching, inspecting launch fields, or taking end authority.
func (s *Scope) HoldOracle(command any, output *Output) {
	if len(s.oracleObjects)>=4 { s.run.blocked("oracle object limit",nil) }
	s.oracleObjects=append(s.oracleObjects,command,output)
}
func (o *Output) Write(p []byte) (int,error) {
	o.mu.Lock();defer o.mu.Unlock()
	o.total+=int64(len(p));_,_=o.h.Write(p)
	remaining:=outputLimit-len(o.data)
	if remaining>len(p) { remaining=len(p) }
	if remaining>0 { o.data=append(o.data,p[:remaining]...) }
	return len(p),nil
}
func (o *Output) Result() ([]byte,error) {
	o.mu.Lock();defer o.mu.Unlock()
	out:=append([]byte(nil),o.data...)
	if o.total>outputLimit { return out,&outputLimitError{total:o.total,digest:hex.EncodeToString(o.h.Sum(nil))} }
	return out,nil
}
