//go:build manual_readonly_wal && linux

package sqlite_test

// SOURCE ONLY. A tag or manifest is not execution authorization. This consumes
// already-prepared media and a status-only keeper pipe; it never prepares them.
import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"golang.org/x/sys/unix"
)

type manualReadOnlyWALNativeOwner struct {
	FD                 int
	DB                 *sqlite.DB
	Tx                 *sqlite.Tx
	Rows               *sqlite.Rows
	Original, Terminal error
	Label              string
}
type manualReadOnlyWALNativeRun struct {
	Ledger                                                        manualReadOnlyWALLedger
	Owners                                                        []manualReadOnlyWALNativeOwner
	Errors                                                        []error
	Manifest                                                      manualReadOnlyWALManifest
	RawManifest                                                   []byte
	Phase                                                         manualReadOnlyWALPhase
	Source, Evidence, Child, Events, Input, Control, ManifestFile int
	Family                                                        [3]int
	Candidate                                                     string
	ControlPath, ManifestName                                     string
	ControlStat, ManifestStat                                     manualReadOnlyWALNativeStat
	APICtx                                                        context.Context
	Locked, Complete, OutputFailed, Started, Preexisting          bool
	Failure                                                       string
	LastErrno                                                     uint64
	Baseline                                                      *manualReadOnlyWALNativeSnapshot
}
type manualReadOnlyWALNativeStat struct {
	Identity             manualReadOnlyWALIdentity
	MountID              uint64
	Mode                 uint16
	UID, GID, Links      uint32
	Size                 uint64
	MtimeSec, CtimeSec   int64
	MtimeNsec, CtimeNsec uint32
}
type manualReadOnlyWALNativeSnapshot struct {
	Facts   manualReadOnlyWALFacts
	Control manualReadOnlyWALObject
	Stats   [4]manualReadOnlyWALNativeStat
	Hashes  [3]string
	Groups  []uint32
}
type manualReadOnlyWALNativeEvent struct {
	Kind        string                           `json:"kind"`
	Status      string                           `json:"status,omitempty"`
	Phase       string                           `json:"phase,omitempty"`
	Challenge   string                           `json:"challenge,omitempty"`
	Candidate   string                           `json:"candidate,omitempty"`
	Owner       int                              `json:"owner"`
	Action      string                           `json:"action,omitempty"`
	HadError    bool                             `json:"had_error,omitempty"`
	Failure     string                           `json:"failure,omitempty"`
	Errno       uint64                           `json:"errno,omitempty"`
	Manifest    *manualReadOnlyWALManifest       `json:"manifest,omitempty"`
	Observation *manualReadOnlyWALNativeSnapshot `json:"observation,omitempty"`
	Receipt     *manualReadOnlyWALReceipt        `json:"receipt,omitempty"`
	States      []manualReadOnlyWALOwnerState    `json:"owners,omitempty"`
	Schema      []sqlite.SchemaEntry             `json:"schema,omitempty"`
	Header      *sqlite.FileInfo                 `json:"header,omitempty"`
	Check       *sqlite.CheckResult              `json:"check,omitempty"`
	Build       []string                         `json:"build,omitempty"`
}

// The sole inert package variable retains every owner and original error until
// natural process exit, including failed-open tokens that snapshots omit.
var manualReadOnlyWALNativeRetained *manualReadOnlyWALNativeRun

func TestManualReadOnlyWALPreparedMedia(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_READONLY_WAL") != "independently_authorized" {
		t.Fatal("NOT_RUN: independent authorization gate absent")
	}
	if err := manualReadOnlyWALNativeArgv(os.Args); err != nil {
		t.Fatal("NOT_RUN: exact standalone argv required")
	}
	r := &manualReadOnlyWALNativeRun{Source: -1, Evidence: -1, Child: -1, Events: -1, Input: -1, Control: -1, ManifestFile: -1, Family: [3]int{-1, -1, -1}}
	manualReadOnlyWALNativeRetained = r
	defer manualReadOnlyWALNativeFinish(t, r)
	if len(sqlite.PendingCloseCleanups()) != 0 || len(sqlite.PendingPlaintextCleanups()) != 0 {
		r.Preexisting = true
		r.Failure = "preexisting pending cleanup untouched"
		return
	}
	// Empty public snapshots exclude ordinary live/opening groups and some
	// failed-open registries. A separately reviewed sealed process is required.
	runtime.LockOSThread()
	r.Locked = true
	if err := manualReadOnlyWALNativeRunPrepared(r); err != nil {
		r.Errors = append(r.Errors, err)
		if r.Failure == "" {
			r.Failure = "prepared qualification failed"
		}
		return
	}
	r.Complete = true
}
func manualReadOnlyWALNativeArgv(args []string) error {
	want := []string{"-test.run=^TestManualReadOnlyWALPreparedMedia$", "-test.v=true", "-test.count=1", "-test.failfast=true", "-test.parallel=1", "-test.shuffle=off", "-test.timeout=0"}
	if len(args) != len(want)+1 || args[0] == "" {
		return errors.New("argv shape")
	}
	for i, arg := range want {
		if args[i+1] != arg {
			return errors.New("argv value/order")
		}
	}
	return nil
}
func manualReadOnlyWALNativeFail(r *manualReadOnlyWALNativeRun, stage string, err error) error {
	if err != nil {
		r.Errors = append(r.Errors, err)
		if errno, ok := err.(unix.Errno); ok {
			r.LastErrno = uint64(errno)
		}
	}
	if r.Failure == "" {
		r.Failure = stage
	}
	return errors.New(stage)
}
func manualReadOnlyWALNativeReserve(r *manualReadOnlyWALNativeRun, parent int, kind, label string) (int, error) {
	id, err := manualReadOnlyWALReserve(&r.Ledger, parent, kind)
	if err != nil {
		return -1, manualReadOnlyWALNativeFail(r, "owner reservation", err)
	}
	r.Owners = append(r.Owners, manualReadOnlyWALNativeOwner{FD: -1, Label: label})
	return id, nil
}

// Capture is deliberately the first operation after every acquisition return.
// It is in-memory only, preserves the original error object, and never logs.
func manualReadOnlyWALNativeCaptureFD(r *manualReadOnlyWALNativeRun, id, fd int, original error) error {
	r.Owners[id].FD, r.Owners[id].Original = fd, original
	return manualReadOnlyWALAdopt(&r.Ledger, id, fd >= 0, original != nil, false)
}
func manualReadOnlyWALNativeDirectory(r *manualReadOnlyWALNativeRun, path, label string) (int, error) {
	id, err := manualReadOnlyWALNativeReserve(r, -1, "fd", label)
	if err != nil {
		return -1, err
	}
	fd, original := unix.Openat2(unix.AT_FDCWD, path, &unix.OpenHow{Flags: unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC, Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS})
	capture := manualReadOnlyWALNativeCaptureFD(r, id, fd, original)
	if original != nil || capture != nil {
		return id, manualReadOnlyWALNativeFail(r, "directory acquisition", original)
	}
	return id, nil
}
func manualReadOnlyWALNativeRelative(r *manualReadOnlyWALNativeRun, parent int, name, label string, directory bool) (int, error) {
	id, err := manualReadOnlyWALNativeReserve(r, parent, "fd", label)
	if err != nil {
		return -1, err
	}
	flags := uint64(unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NONBLOCK | unix.O_NOFOLLOW)
	if directory {
		flags |= unix.O_DIRECTORY
	}
	fd, original := unix.Openat2(r.Owners[parent].FD, name, &unix.OpenHow{Flags: flags, Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_XDEV})
	capture := manualReadOnlyWALNativeCaptureFD(r, id, fd, original)
	if original != nil || capture != nil {
		return id, manualReadOnlyWALNativeFail(r, "relative readonly acquisition", original)
	}
	return id, nil
}
func manualReadOnlyWALNativeProc(r *manualReadOnlyWALNativeRun, path string, limit int) ([]byte, error) {
	if path != "/proc/thread-self/status" && path != "/proc/self/mountinfo" {
		return nil, manualReadOnlyWALNativeFail(r, "unapproved self metadata", nil)
	}
	id, err := manualReadOnlyWALNativeReserve(r, -1, "fd", "bounded-self-metadata")
	if err != nil {
		return nil, err
	}
	fd, original := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	capture := manualReadOnlyWALNativeCaptureFD(r, id, fd, original)
	if original != nil || capture != nil {
		return nil, manualReadOnlyWALNativeFail(r, "self metadata acquisition", original)
	}
	var fs unix.Statfs_t
	if err := unix.Fstatfs(r.Owners[id].FD, &fs); err != nil {
		return nil, manualReadOnlyWALNativeFail(r, "self metadata filesystem provenance", err)
	}
	if fs.Type != unix.PROC_SUPER_MAGIC {
		return nil, manualReadOnlyWALNativeFail(r, "self metadata is not procfs", nil)
	}
	raw, err := manualReadOnlyWALNativeRead(r, id, limit)
	if err != nil {
		return nil, err
	}
	if err := manualReadOnlyWALNativeEnd(r, id, "close"); err != nil {
		return nil, err
	}
	return raw, nil
}
func manualReadOnlyWALNativeRead(r *manualReadOnlyWALNativeRun, id, limit int) ([]byte, error) {
	var raw []byte
	buffer := make([]byte, 4096)
	for len(raw) <= limit {
		remaining := limit + 1 - len(raw)
		if remaining < len(buffer) {
			buffer = buffer[:remaining]
		}
		n, err := unix.Read(r.Owners[id].FD, buffer)
		if err != nil {
			return nil, manualReadOnlyWALNativeFail(r, "bounded input read", err)
		}
		if n == 0 {
			return raw, nil
		}
		raw = append(raw, buffer[:n]...)
	}
	return nil, manualReadOnlyWALNativeFail(r, "input byte ceiling", nil)
}
func manualReadOnlyWALNativeStatFD(r *manualReadOnlyWALNativeRun, id int) (manualReadOnlyWALNativeStat, error) {
	const mask = unix.STATX_TYPE | unix.STATX_MODE | unix.STATX_UID | unix.STATX_GID | unix.STATX_NLINK | unix.STATX_INO | unix.STATX_SIZE | unix.STATX_MTIME | unix.STATX_CTIME | unix.STATX_MNT_ID
	var s unix.Statx_t
	if err := unix.Statx(r.Owners[id].FD, "", unix.AT_EMPTY_PATH|unix.AT_SYMLINK_NOFOLLOW, mask, &s); err != nil {
		return manualReadOnlyWALNativeStat{}, manualReadOnlyWALNativeFail(r, "descriptor statx", err)
	}
	if s.Mask&mask != mask {
		return manualReadOnlyWALNativeStat{}, manualReadOnlyWALNativeFail(r, "statx returned mask incomplete", nil)
	}
	return manualReadOnlyWALNativeStatValue(s), nil
}
func manualReadOnlyWALNativeStatValue(s unix.Statx_t) manualReadOnlyWALNativeStat {
	return manualReadOnlyWALNativeStat{manualReadOnlyWALIdentity{s.Dev_major, s.Dev_minor, s.Ino}, s.Mnt_id, s.Mode, s.Uid, s.Gid, s.Nlink, s.Size, s.Mtime.Sec, s.Ctime.Sec, s.Mtime.Nsec, s.Ctime.Nsec}
}
func manualReadOnlyWALNativePathMatches(r *manualReadOnlyWALNativeRun, dir int, path string, expected manualReadOnlyWALNativeStat) error {
	const mask = unix.STATX_TYPE | unix.STATX_MODE | unix.STATX_UID | unix.STATX_GID | unix.STATX_NLINK | unix.STATX_INO | unix.STATX_SIZE | unix.STATX_MTIME | unix.STATX_CTIME | unix.STATX_MNT_ID
	var s unix.Statx_t
	if err := unix.Statx(dir, path, unix.AT_SYMLINK_NOFOLLOW, mask, &s); err != nil {
		return manualReadOnlyWALNativeFail(r, "retained path revalidation", err)
	}
	if s.Mask&mask != mask || manualReadOnlyWALNativeStatValue(s) != expected {
		return manualReadOnlyWALNativeFail(r, "retained path/descriptor identity mismatch", nil)
	}
	return nil
}

// Both retained objects and both pathname views must still match the pre-read
// metadata. The digest does not replace this independently checked binding.
func manualReadOnlyWALNativeControlMatches(r *manualReadOnlyWALNativeRun) error {
	control, err := manualReadOnlyWALNativeStatFD(r, r.Control)
	if err != nil {
		return err
	}
	manifest, err := manualReadOnlyWALNativeStatFD(r, r.ManifestFile)
	if err != nil {
		return err
	}
	if control != r.ControlStat || manifest != r.ManifestStat {
		return manualReadOnlyWALNativeFail(r, "control metadata changed", nil)
	}
	if err := manualReadOnlyWALNativePathMatches(r, unix.AT_FDCWD, r.ControlPath, control); err != nil {
		return err
	}
	if err := manualReadOnlyWALNativePathMatches(r, r.Owners[r.Control].FD, r.ManifestName, manifest); err != nil {
		return err
	}
	return nil
}
func manualReadOnlyWALNativeBuild(r *manualReadOnlyWALNativeRun) ([]string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.GoVersion != "go1.27.2" || info.Main.Path != "github.com/ms-xiaosnow/gosqlite" || info.Main.Replace != nil {
		return nil, manualReadOnlyWALNativeFail(r, "sealed build metadata", nil)
	}
	result := []string{"GoVersion=" + info.GoVersion, "module=" + info.Main.Path, "version=" + info.Main.Version, "sum=" + info.Main.Sum}
	want := map[string]string{"golang.org/x/crypto": "v0.58.0 h1:COkYLr4k7nDI4r6QtMML2AfzeIswWHAJthLS6K/M/54=", "golang.org/x/sys": "v0.49.0 h1:XbzkgYJHdqh/8m2Uu0W/dQv8nktxx4BFHp1M0gROTXA=", "golang.org/x/text": "v0.43.0 h1:1QivrlhwAsnMOcqgOdl9my7s9OyZl8O5/UGUTJVXoWc="}
	for _, dependency := range info.Deps {
		if dependency.Replace != nil || want[dependency.Path] != dependency.Version+" "+dependency.Sum {
			return nil, manualReadOnlyWALNativeFail(r, "unreviewed module build metadata", nil)
		}
		result = append(result, dependency.Path+" "+dependency.Version+" "+dependency.Sum)
	}
	seen := make(map[string]bool)
	for _, setting := range info.Settings {
		if seen[setting.Key] {
			return nil, manualReadOnlyWALNativeFail(r, "duplicate build metadata", nil)
		}
		seen[setting.Key] = true
		allowed := false
		switch setting.Key {
		case "-buildmode":
			allowed = setting.Value == "exe"
		case "-compiler":
			allowed = setting.Value == "gc"
		case "-tags":
			allowed = setting.Value == "manual_readonly_wal"
		case "-trimpath":
			allowed = setting.Value == "true"
		case "CGO_ENABLED":
			allowed = setting.Value == "0"
		case "GOOS":
			allowed = setting.Value == "linux"
		case "GOARCH":
			allowed = setting.Value == "amd64" || setting.Value == "arm64"
		case "GOAMD64":
			allowed = setting.Value == "v1"
		case "GOARM64":
			allowed = setting.Value == "v8.0"
		case "vcs":
			allowed = setting.Value == "git"
		case "vcs.revision":
			allowed = manualReadOnlyWALHex(setting.Value, 40)
		case "vcs.time":
			_, err := time.Parse(time.RFC3339, setting.Value)
			allowed = err == nil
		case "vcs.modified":
			allowed = setting.Value == "false"
		}
		if !allowed {
			return nil, manualReadOnlyWALNativeFail(r, "unreviewed build setting", nil)
		}
		result = append(result, setting.Key+"="+setting.Value)
	}
	for _, key := range []string{"-buildmode", "-compiler", "-tags", "-trimpath", "CGO_ENABLED", "GOOS", "GOARCH"} {
		if !seen[key] {
			return nil, manualReadOnlyWALNativeFail(r, "missing build setting", nil)
		}
	}
	return result, nil
}
func manualReadOnlyWALNativeCredentials(r *manualReadOnlyWALNativeRun) (manualReadOnlyWALCredentials, []uint32, error) {
	var c manualReadOnlyWALCredentials
	ru, eu, su := unix.Getresuid()
	rg, eg, sg := unix.Getresgid()
	groups, err := unix.Getgroups()
	if err != nil {
		return c, nil, manualReadOnlyWALNativeFail(r, "getgroups", err)
	}
	var capData [2]unix.CapUserData
	capHeader := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3, Pid: 0}
	if err := unix.Capget(&capHeader, &capData[0]); err != nil {
		return c, nil, manualReadOnlyWALNativeFail(r, "capget", err)
	}
	if capHeader.Version != unix.LINUX_CAPABILITY_VERSION_3 {
		return c, nil, manualReadOnlyWALNativeFail(r, "capability version", nil)
	}
	raw, err := manualReadOnlyWALNativeProc(r, "/proc/thread-self/status", 65536)
	if err != nil {
		return c, nil, err
	}
	fields := make(map[string][]string)
	for _, line := range strings.Split(string(raw), "\n") {
		if len(line) > 8192 {
			return c, nil, manualReadOnlyWALNativeFail(r, "status line bound", nil)
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch key {
		case "Uid", "Gid", "Groups", "CapEff", "CapPrm", "CapInh", "CapAmb":
			if _, exists := fields[key]; exists {
				return c, nil, manualReadOnlyWALNativeFail(r, "duplicate status field", nil)
			}
			fields[key] = strings.Fields(value)
		}
	}
	if len(fields) != 7 || len(fields["Uid"]) != 4 || len(fields["Gid"]) != 4 {
		return c, nil, manualReadOnlyWALNativeFail(r, "self credential fields", nil)
	}
	for i := 0; i < 4; i++ {
		u, e := strconv.ParseUint(fields["Uid"][i], 10, 32)
		if e != nil {
			return c, nil, manualReadOnlyWALNativeFail(r, "UID parse", e)
		}
		c.UID[i] = uint32(u)
		g, e := strconv.ParseUint(fields["Gid"][i], 10, 32)
		if e != nil {
			return c, nil, manualReadOnlyWALNativeFail(r, "GID parse", e)
		}
		c.GID[i] = uint32(g)
	}
	observedGroups := make([]uint32, 0, len(groups))
	for _, group := range groups {
		if group < 0 {
			return c, nil, manualReadOnlyWALNativeFail(r, "negative group", nil)
		}
		observedGroups = append(observedGroups, uint32(group))
	}
	var procGroups []uint32
	for _, word := range fields["Groups"] {
		group, e := strconv.ParseUint(word, 10, 32)
		if e != nil {
			return c, nil, manualReadOnlyWALNativeFail(r, "group parse", e)
		}
		procGroups = append(procGroups, uint32(group))
	}
	slices.Sort(observedGroups)
	slices.Sort(procGroups)
	c.GroupsMatch = slices.Equal(observedGroups, procGroups)
	c.Effective = [2]uint32{capData[0].Effective, capData[1].Effective}
	c.Permitted = [2]uint32{capData[0].Permitted, capData[1].Permitted}
	c.Inheritable = [2]uint32{capData[0].Inheritable, capData[1].Inheritable}
	sets := make(map[string]uint64)
	for _, key := range []string{"CapEff", "CapPrm", "CapInh", "CapAmb"} {
		if len(fields[key]) != 1 || len(fields[key][0]) != 16 {
			return c, nil, manualReadOnlyWALNativeFail(r, "capability status width", nil)
		}
		value, e := strconv.ParseUint(fields[key][0], 16, 64)
		if e != nil {
			return c, nil, manualReadOnlyWALNativeFail(r, "capability status parse", e)
		}
		sets[key] = value
	}
	c.Ambient = sets["CapAmb"]
	c.ProcMatches = c.UID[0] == uint32(ru) && c.UID[1] == uint32(eu) && c.UID[2] == uint32(su) && c.GID[0] == uint32(rg) && c.GID[1] == uint32(eg) && c.GID[2] == uint32(sg) && sets["CapEff"] == uint64(c.Effective[0])|uint64(c.Effective[1])<<32 && sets["CapPrm"] == uint64(c.Permitted[0])|uint64(c.Permitted[1])<<32 && sets["CapInh"] == uint64(c.Inheritable[0])|uint64(c.Inheritable[1])<<32
	c.Observed = true
	return c, observedGroups, nil
}
func manualReadOnlyWALNativeUnescape(s string) (string, error) {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			result.WriteByte(s[i])
			continue
		}
		if i+3 >= len(s) {
			return "", errors.New("mount escape")
		}
		part := s[i+1 : i+4]
		switch part {
		case "040":
			result.WriteByte(' ')
		case "011":
			result.WriteByte('\t')
		case "012":
			result.WriteByte('\n')
		case "134":
			result.WriteByte('\\')
		default:
			return "", errors.New("mount escape")
		}
		i += 3
	}
	return result.String(), nil
}
func manualReadOnlyWALNativeMounts(r *manualReadOnlyWALNativeRun) (map[uint64][]manualReadOnlyWALMount, error) {
	raw, err := manualReadOnlyWALNativeProc(r, "/proc/self/mountinfo", 4<<20)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(raw), "\n")
	if len(lines) > 8193 {
		return nil, manualReadOnlyWALNativeFail(r, "mountinfo line count", nil)
	}
	mounts := make(map[uint64][]manualReadOnlyWALMount)
	for _, line := range lines {
		if line == "" {
			continue
		}
		if len(line) > 8192 {
			return nil, manualReadOnlyWALNativeFail(r, "mountinfo line size", nil)
		}
		left, right, ok := strings.Cut(line, " - ")
		if !ok {
			return nil, manualReadOnlyWALNativeFail(r, "mountinfo separator", nil)
		}
		a, b := strings.Fields(left), strings.Fields(right)
		if len(a) < 6 || len(b) != 3 {
			return nil, manualReadOnlyWALNativeFail(r, "mountinfo fields", nil)
		}
		id, e := strconv.ParseUint(a[0], 10, 64)
		if e != nil || id == 0 {
			return nil, manualReadOnlyWALNativeFail(r, "mountinfo ID", e)
		}
		major, minor, ok := strings.Cut(a[2], ":")
		if !ok {
			return nil, manualReadOnlyWALNativeFail(r, "mountinfo device", nil)
		}
		ma, e := strconv.ParseUint(major, 10, 32)
		if e != nil {
			return nil, manualReadOnlyWALNativeFail(r, "mountinfo major", e)
		}
		mi, e := strconv.ParseUint(minor, 10, 32)
		if e != nil {
			return nil, manualReadOnlyWALNativeFail(r, "mountinfo minor", e)
		}
		root, e := manualReadOnlyWALNativeUnescape(a[3])
		if e != nil {
			return nil, manualReadOnlyWALNativeFail(r, "mount root escape", e)
		}
		point, e := manualReadOnlyWALNativeUnescape(a[4])
		if e != nil {
			return nil, manualReadOnlyWALNativeFail(r, "mount point escape", e)
		}
		if !strings.HasPrefix(root, "/") || !strings.HasPrefix(point, "/") {
			return nil, manualReadOnlyWALNativeFail(r, "mount absolute paths", nil)
		}
		mounts[id] = append(mounts[id], manualReadOnlyWALMount{ID: id, Major: uint32(ma), Minor: uint32(mi), Root: root, Point: point, Options: a[5], SuperOptions: b[2], Type: b[0]})
	}
	return mounts, nil
}
func manualReadOnlyWALNativeObserveObject(r *manualReadOnlyWALNativeRun, id int, directory bool, mounts map[uint64][]manualReadOnlyWALMount) (manualReadOnlyWALObject, manualReadOnlyWALNativeStat, error) {
	var o manualReadOnlyWALObject
	s, err := manualReadOnlyWALNativeStatFD(r, id)
	if err != nil {
		return o, s, err
	}
	var fs unix.Statfs_t
	if err := unix.Fstatfs(r.Owners[id].FD, &fs); err != nil {
		return o, s, manualReadOnlyWALNativeFail(r, "fstatfs", err)
	}
	matches := mounts[s.MountID]
	if len(matches) != 1 {
		return o, s, manualReadOnlyWALNativeFail(r, "ordinary mount ID match", nil)
	}
	mount := matches[0]
	mount.Observed = true
	mount.Matches = len(matches)
	mount.Magic = int64(fs.Type)
	mount.ReadOnly = fs.Flags&unix.ST_RDONLY != 0
	mode := uint32(unix.R_OK)
	if directory {
		mode |= unix.X_OK
	}
	if id == r.Evidence {
		mode |= unix.W_OK
	}
	if err := unix.Faccessat2(r.Owners[id].FD, "", mode, unix.AT_EMPTY_PATH|unix.AT_EACCESS); err != nil {
		return o, s, manualReadOnlyWALNativeFail(r, "effective read/search access", err)
	}
	write := unix.Faccessat2(r.Owners[id].FD, "", unix.W_OK, unix.AT_EMPTY_PATH|unix.AT_EACCESS)
	result := ""
	switch write {
	case nil:
		result = "ALLOW"
	case unix.EACCES:
		result = "EACCES"
	case unix.EROFS:
		result = "EROFS"
	default:
		return o, s, manualReadOnlyWALNativeFail(r, "effective access unsupported/unknown", write)
	}
	o = manualReadOnlyWALObject{Observed: true, Identity: s.Identity, UID: s.UID, Mode: uint32(s.Mode), Links: s.Links, Mount: mount, ReadAllowed: true, AccessObserved: true, WriteResult: result, AccessCall: "Faccessat2", AccessFlags: unix.AT_EMPTY_PATH | unix.AT_EACCESS, ReadMask: mode, WriteMask: unix.W_OK}
	return o, s, nil
}
func manualReadOnlyWALNativeNames(r *manualReadOnlyWALNativeRun) error {
	if _, err := unix.Seek(r.Owners[r.Source].FD, 0, 0); err != nil {
		return manualReadOnlyWALNativeFail(r, "source directory rewind", err)
	}
	var names []string
	buffer := make([]byte, 4096)
	for reads := 0; reads < 4; reads++ {
		n, err := unix.ReadDirent(r.Owners[r.Source].FD, buffer)
		if err != nil {
			return manualReadOnlyWALNativeFail(r, "source names", err)
		}
		if n == 0 {
			slices.Sort(names)
			if !slices.Equal(names, []string{"readonly.db", "readonly.db-shm", "readonly.db-wal"}) {
				return manualReadOnlyWALNativeFail(r, "exact three source names", nil)
			}
			return nil
		}
		consumed, _, next := unix.ParseDirent(buffer[:n], 4, names)
		names = next
		if consumed != n || len(names) > 3 {
			return manualReadOnlyWALNativeFail(r, "source directory entry bound", nil)
		}
	}
	return manualReadOnlyWALNativeFail(r, "source directory EOF bound", nil)
}
func manualReadOnlyWALNativeHash(r *manualReadOnlyWALNativeRun, id int, size uint64) (string, error) {
	if size > 1048576 {
		return "", manualReadOnlyWALNativeFail(r, "fixture hash budget", nil)
	}
	h := sha256.New()
	buffer := make([]byte, 4096)
	for offset := uint64(0); offset < size; {
		length := uint64(len(buffer))
		if size-offset < length {
			length = size - offset
		}
		n, err := unix.Pread(r.Owners[id].FD, buffer[:length], int64(offset))
		if err != nil || n <= 0 {
			return "", manualReadOnlyWALNativeFail(r, "bounded fixture read", err)
		}
		written, err := h.Write(buffer[:n])
		if err != nil || written != n {
			return "", manualReadOnlyWALNativeFail(r, "hash write", err)
		}
		offset += uint64(n)
	}
	var extra [1]byte
	n, err := unix.Pread(r.Owners[id].FD, extra[:], int64(size))
	if err != nil || n != 0 {
		return "", manualReadOnlyWALNativeFail(r, "fixture exact byte count", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Metadata-only admission: no fixture content is read before a fresh receipt.
func manualReadOnlyWALNativeMetadataNow(r *manualReadOnlyWALNativeRun) (manualReadOnlyWALNativeSnapshot, error) {
	var snapshot manualReadOnlyWALNativeSnapshot
	if r.Ledger.Blocked {
		return snapshot, manualReadOnlyWALNativeFail(r, "observation blocked by ownership uncertainty", nil)
	}
	credentials, groups, err := manualReadOnlyWALNativeCredentials(r)
	if err != nil {
		return snapshot, err
	}
	snapshot.Facts.Credentials, snapshot.Groups = credentials, groups
	mounts, err := manualReadOnlyWALNativeMounts(r)
	if err != nil {
		return snapshot, err
	}
	ids := [4]int{r.Source, r.Family[0], r.Family[1], r.Family[2]}
	for i, id := range ids {
		object, stat, err := manualReadOnlyWALNativeObserveObject(r, id, i == 0, mounts)
		if err != nil {
			return snapshot, err
		}
		snapshot.Facts.Source[i], snapshot.Stats[i] = object, stat
	}
	evidence, _, err := manualReadOnlyWALNativeObserveObject(r, r.Evidence, true, mounts)
	if err != nil {
		return snapshot, err
	}
	snapshot.Facts.Evidence = evidence
	if err := manualReadOnlyWALNativeControlMatches(r); err != nil {
		return snapshot, err
	}
	control, controlStat, err := manualReadOnlyWALNativeObserveObject(r, r.Control, true, mounts)
	if err != nil {
		return snapshot, err
	}
	snapshot.Control = control
	if controlStat != r.ControlStat || control.Mode&unix.S_IFMT != unix.S_IFDIR || !manualReadOnlyWALMountValid(control.Mount, control.Identity, control.Mount.ReadOnly) {
		return snapshot, manualReadOnlyWALNativeFail(r, "control directory mount provenance", nil)
	}
	source := snapshot.Facts.Source[0]
	// Namespace-local identity and mount-root provenance checks are finite;
	// separately reviewed operator provenance covers undiscoverable aliases.
	sourcePlace, err := manualReadOnlyWALNativeBackingPlace(r.Manifest.SourceRoot, source.Mount)
	if err != nil {
		return snapshot, manualReadOnlyWALNativeFail(r, "source mount-root provenance", err)
	}
	evidencePlace, err := manualReadOnlyWALNativeBackingPlace(r.Manifest.EvidenceParent, evidence.Mount)
	if err != nil {
		return snapshot, manualReadOnlyWALNativeFail(r, "evidence mount-root provenance", err)
	}
	controlPlace, err := manualReadOnlyWALNativeBackingPlace(r.ControlPath, control.Mount)
	if err != nil {
		return snapshot, manualReadOnlyWALNativeFail(r, "control mount-root provenance", err)
	}
	separateBacking := source.Identity.Major != evidence.Identity.Major || source.Identity.Minor != evidence.Identity.Minor || sourcePlace != "/" && evidencePlace != "/" && manualReadOnlyWALSeparate(sourcePlace, evidencePlace)
	separateControlSource := control.Identity.Major != source.Identity.Major || control.Identity.Minor != source.Identity.Minor || controlPlace != "/" && sourcePlace != "/" && manualReadOnlyWALSeparate(controlPlace, sourcePlace)
	separateControlEvidence := control.Identity.Major != evidence.Identity.Major || control.Identity.Minor != evidence.Identity.Minor || controlPlace != "/" && evidencePlace != "/" && manualReadOnlyWALSeparate(controlPlace, evidencePlace)
	snapshot.Facts.DistinctLocations = source.Identity != evidence.Identity && control.Identity != source.Identity && control.Identity != evidence.Identity && separateBacking && separateControlSource && separateControlEvidence
	if err := manualReadOnlyWALValidateProfile(snapshot.Facts, r.Manifest); err != nil {
		return snapshot, manualReadOnlyWALNativeFail(r, "actual Linux profile refused", err)
	}
	if err := manualReadOnlyWALNativeNames(r); err != nil {
		return snapshot, err
	}
	if err := manualReadOnlyWALNativePathMatches(r, unix.AT_FDCWD, r.Manifest.SourceRoot, snapshot.Stats[0]); err != nil {
		return snapshot, err
	}
	for i, member := range r.Manifest.Family {
		if snapshot.Stats[i+1].Size != member.Size {
			return snapshot, manualReadOnlyWALNativeFail(r, "manifest fixture size", nil)
		}
		if err := manualReadOnlyWALNativePathMatches(r, r.Owners[r.Source].FD, member.Name, snapshot.Stats[i+1]); err != nil {
			return snapshot, err
		}
	}
	return snapshot, nil
}
func manualReadOnlyWALNativeSnapshotNow(r *manualReadOnlyWALNativeRun) (manualReadOnlyWALNativeSnapshot, error) {
	var snapshot manualReadOnlyWALNativeSnapshot
	if r.Phase.Next == 0 {
		return snapshot, manualReadOnlyWALNativeFail(r, "fixture content requires a fresh keeper receipt", nil)
	}
	snapshot, err := manualReadOnlyWALNativeMetadataNow(r)
	if err != nil {
		return snapshot, err
	}
	for i, member := range r.Manifest.Family {
		digest, err := manualReadOnlyWALNativeHash(r, r.Family[i], member.Size)
		if err != nil {
			return snapshot, err
		}
		snapshot.Hashes[i] = digest
		if digest != member.SHA256 {
			return snapshot, manualReadOnlyWALNativeFail(r, "manifest fixture digest", nil)
		}
		after, err := manualReadOnlyWALNativeStatFD(r, r.Family[i])
		if err != nil {
			return snapshot, err
		}
		if after != snapshot.Stats[i+1] {
			return snapshot, manualReadOnlyWALNativeFail(r, "fixture changed while hashing", nil)
		}
	}
	if err := manualReadOnlyWALNativeHeader(r); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}
func manualReadOnlyWALNativeBoundPaths(r *manualReadOnlyWALNativeRun) error {
	if r.Ledger.Blocked || r.Baseline == nil {
		return manualReadOnlyWALNativeFail(r, "API path observation blocked", nil)
	}
	for i, id := range [4]int{r.Source, r.Family[0], r.Family[1], r.Family[2]} {
		stat, err := manualReadOnlyWALNativeStatFD(r, id)
		if err != nil {
			return err
		}
		if stat != r.Baseline.Stats[i] {
			return manualReadOnlyWALNativeFail(r, "API descriptor identity changed", nil)
		}
		dir, path := unix.AT_FDCWD, r.Manifest.SourceRoot
		if i > 0 {
			dir, path = r.Owners[r.Source].FD, r.Manifest.Family[i-1].Name
		}
		if err := manualReadOnlyWALNativePathMatches(r, dir, path, stat); err != nil {
			return err
		}
	}
	return nil
}
func manualReadOnlyWALNativeBackingPlace(path string, mount manualReadOnlyWALMount) (string, error) {
	if path != mount.Point && !strings.HasPrefix(path, strings.TrimSuffix(mount.Point, "/")+"/") {
		return "", errors.New("descriptor path outside matching mount point")
	}
	relative := strings.TrimPrefix(path, mount.Point)
	return filepath.Clean(mount.Root + "/" + strings.TrimPrefix(relative, "/")), nil
}
func manualReadOnlyWALNativeHeader(r *manualReadOnlyWALNativeRun) error {
	var header [100]byte
	n, err := unix.Pread(r.Owners[r.Family[0]].FD, header[:], 0)
	if err != nil || n != len(header) || string(header[:16]) != "SQLite format 3\x00" || binary.BigEndian.Uint16(header[16:18]) != 4096 || header[18] != 2 || header[19] != 2 {
		return manualReadOnlyWALNativeFail(r, "prepared main header", err)
	}
	return nil
}
func manualReadOnlyWALNativeSameSnapshot(a, b manualReadOnlyWALNativeSnapshot) bool {
	return a.Stats == b.Stats && a.Hashes == b.Hashes && a.Facts == b.Facts && a.Control == b.Control && slices.Equal(a.Groups, b.Groups)
}
func manualReadOnlyWALNativeWrite(r *manualReadOnlyWALNativeRun, id int, raw []byte) error {
	if id != r.Events && id != r.Input || id < 0 || r.Ledger.Owners[id].State != manualReadOnlyWALLive || r.Owners[id].Label != "events.ndjson" && r.Owners[id].Label != "input-manifest.json" {
		return manualReadOnlyWALNativeFail(r, "evidence-only write owner", nil)
	}
	n, err := unix.Write(r.Owners[id].FD, raw)
	if err != nil || n != len(raw) {
		r.OutputFailed = true
		return manualReadOnlyWALNativeFail(r, "evidence short/failed write", err)
	}
	if err := unix.Fsync(r.Owners[id].FD); err != nil {
		r.OutputFailed = true
		return manualReadOnlyWALNativeFail(r, "evidence sync", err)
	}
	return nil
}
func manualReadOnlyWALNativeEventWrite(r *manualReadOnlyWALNativeRun, event manualReadOnlyWALNativeEvent) error {
	raw, err := json.Marshal(event)
	if err != nil {
		r.OutputFailed = true
		return manualReadOnlyWALNativeFail(r, "event encoding", err)
	}
	if len(raw) > 65536 {
		r.OutputFailed = true
		return manualReadOnlyWALNativeFail(r, "event byte ceiling", nil)
	}
	raw = append(raw, '\n')
	if r.Events >= 0 && r.Ledger.Owners[r.Events].State == manualReadOnlyWALLive && !r.OutputFailed {
		if err := manualReadOnlyWALNativeWrite(r, r.Events, raw); err != nil {
			return err
		}
	}
	n, err := os.Stdout.Write(raw)
	if err != nil || n != len(raw) {
		r.OutputFailed = true
		return manualReadOnlyWALNativeFail(r, "stdout short/failed write", err)
	}
	return nil
}
func manualReadOnlyWALNativeEvidence(r *manualReadOnlyWALNativeRun) error {
	r.Candidate = filepath.Join(r.Manifest.EvidenceParent, "readonly-wal-"+r.Manifest.RunID)
	id, err := manualReadOnlyWALNativeReserve(r, r.Evidence, "fd", "evidence-child")
	if err != nil {
		return err
	}
	// The candidate is retained before the one exclusive namespace operation.
	if err := manualReadOnlyWALNativeEventWrite(r, manualReadOnlyWALNativeEvent{Kind: "candidate-reserved", Candidate: r.Candidate, Owner: id}); err != nil {
		return err
	}
	original := unix.Mkdirat(r.Owners[r.Evidence].FD, "readonly-wal-"+r.Manifest.RunID, 0700)
	r.Owners[id].Original = original
	if original != nil {
		if err := manualReadOnlyWALAdopt(&r.Ledger, id, false, true, false); err != nil {
			return err
		}
		return manualReadOnlyWALNativeFail(r, "exclusive evidence mkdir", original)
	}
	fd, original := unix.Openat2(r.Owners[r.Evidence].FD, "readonly-wal-"+r.Manifest.RunID, &unix.OpenHow{Flags: unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW, Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_XDEV})
	capture := manualReadOnlyWALNativeCaptureFD(r, id, fd, original)
	r.Child = id
	if original != nil || capture != nil {
		return manualReadOnlyWALNativeFail(r, "evidence child acquisition", original)
	}
	stat, err := manualReadOnlyWALNativeStatFD(r, id)
	if err != nil {
		return err
	}
	parent, err := manualReadOnlyWALNativeStatFD(r, r.Evidence)
	if err != nil {
		return err
	}
	if stat.Mode != 0040700 || stat.UID != r.Manifest.ReaderUID || stat.MountID != parent.MountID || stat.Identity == parent.Identity {
		return manualReadOnlyWALNativeFail(r, "private evidence child", nil)
	}
	for _, name := range []string{"events.ndjson", "input-manifest.json"} {
		fileID, err := manualReadOnlyWALNativeReserve(r, r.Child, "fd", name)
		if err != nil {
			return err
		}
		fd, original := unix.Openat(r.Owners[r.Child].FD, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
		capture := manualReadOnlyWALNativeCaptureFD(r, fileID, fd, original)
		if name == "events.ndjson" {
			r.Events = fileID
		} else {
			r.Input = fileID
		}
		if original != nil || capture != nil {
			return manualReadOnlyWALNativeFail(r, "exclusive evidence file", original)
		}
		fileStat, err := manualReadOnlyWALNativeStatFD(r, fileID)
		if err != nil {
			return err
		}
		if fileStat.Mode != 0100600 || fileStat.UID != r.Manifest.ReaderUID || fileStat.Links != 1 || fileStat.MountID != stat.MountID || fileStat.Size != 0 {
			return manualReadOnlyWALNativeFail(r, "private evidence file", nil)
		}
	}
	return manualReadOnlyWALNativeWrite(r, r.Input, r.RawManifest)
}
func manualReadOnlyWALNativeReceipt(r *manualReadOnlyWALNativeRun) error {
	phases := [4]string{"keep-false-before", "keep-false-after", "keep-true-before", "keep-true-after"}
	if r.Phase.Next < 0 || r.Phase.Next >= 4 || r.Ledger.Blocked {
		return manualReadOnlyWALNativeFail(r, "receipt state", nil)
	}
	var challenge [16]byte
	if n, err := rand.Read(challenge[:]); err != nil || n != len(challenge) {
		return manualReadOnlyWALNativeFail(r, "fresh challenge", err)
	}
	r.Phase.Challenge = hex.EncodeToString(challenge[:])
	if err := manualReadOnlyWALNativeEventWrite(r, manualReadOnlyWALNativeEvent{Kind: "challenge", Phase: phases[r.Phase.Next], Challenge: r.Phase.Challenge, Owner: -1}); err != nil {
		return err
	}
	deadline := time.Now().Add(30 * time.Second)
	var line []byte
	for len(line) < 2048 {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return manualReadOnlyWALNativeFail(r, "receipt deadline", nil)
		}
		poll := []unix.PollFd{{Fd: 0, Events: unix.POLLIN}}
		ready, err := unix.Poll(poll, int((remaining+time.Millisecond-1)/time.Millisecond))
		if err == unix.EINTR {
			continue
		}
		if err != nil || ready != 1 || poll[0].Revents&unix.POLLIN == 0 || poll[0].Revents&(unix.POLLERR|unix.POLLNVAL) != 0 {
			return manualReadOnlyWALNativeFail(r, "receipt pipe readiness", err)
		}
		var b [1]byte
		n, err := unix.Read(0, b[:])
		if err == unix.EINTR {
			continue
		}
		if err != nil || n != 1 {
			return manualReadOnlyWALNativeFail(r, "receipt pipe EOF/read", err)
		}
		line = append(line, b[0])
		if b[0] == '\n' {
			poll[0].Revents = 0
			ready, err := unix.Poll(poll, 0)
			if err != nil || ready != 0 {
				return manualReadOnlyWALNativeFail(r, "receipt trailing input/closed producer", err)
			}
			receipt, err := manualReadOnlyWALDecodeReceipt(line)
			if err != nil {
				return manualReadOnlyWALNativeFail(r, "receipt exact decoder", err)
			}
			next, err := manualReadOnlyWALAdvancePhase(r.Phase, receipt)
			if err != nil {
				return manualReadOnlyWALNativeFail(r, "receipt transition", err)
			}
			r.Phase = next
			return manualReadOnlyWALNativeEventWrite(r, manualReadOnlyWALNativeEvent{Kind: "keeper-receipt", Status: "cooperatively quiescent live generation", Receipt: &receipt, Owner: -1})
		}
	}
	return manualReadOnlyWALNativeFail(r, "receipt line bound", nil)
}
func manualReadOnlyWALNativeEnd(r *manualReadOnlyWALNativeRun, id int, action string) error {
	if err := manualReadOnlyWALBeginEnd(&r.Ledger, id, action); err != nil {
		return manualReadOnlyWALNativeFail(r, "checked-once dependency barrier", err)
	}
	var original error
	switch r.Ledger.Owners[id].Kind {
	case "fd":
		original = unix.Close(r.Owners[id].FD)
	case "rows":
		original = r.Owners[id].Rows.Close()
	case "tx":
		if action == "commit" {
			original = r.Owners[id].Tx.Commit(r.APICtx)
		} else {
			original = r.Owners[id].Tx.Rollback()
		}
	case "db":
		original = r.Owners[id].DB.Close()
	default:
		r.Ledger.Blocked = true
		return manualReadOnlyWALNativeFail(r, "unknown terminal dispatcher", nil)
	}
	r.Owners[id].Terminal = original
	decision := manualReadOnlyWALFinishEnd(&r.Ledger, id, original != nil)
	logErr := manualReadOnlyWALNativeEventWrite(r, manualReadOnlyWALNativeEvent{Kind: "terminal-attempt", Owner: id, Action: action, HadError: original != nil})
	if original != nil || decision != nil {
		return manualReadOnlyWALNativeFail(r, "FAIL_OWNERSHIP_UNKNOWN", original)
	}
	return logErr
}
func manualReadOnlyWALNativeNext(r *manualReadOnlyWALNativeRun, id int) (bool, error) {
	if err := manualReadOnlyWALBeginEnd(&r.Ledger, id, "next"); err != nil {
		return false, manualReadOnlyWALNativeFail(r, "Next begin", err)
	}
	next := r.Owners[id].Rows.Next()
	var original error
	if !next {
		original = r.Owners[id].Rows.Err()
	}
	r.Owners[id].Terminal = original
	decision := manualReadOnlyWALFinishNext(&r.Ledger, id, next, original != nil)
	if original != nil || decision != nil {
		return false, manualReadOnlyWALNativeFail(r, "FAIL_OWNERSHIP_UNKNOWN", original)
	}
	if !next {
		if err := manualReadOnlyWALNativeEventWrite(r, manualReadOnlyWALNativeEvent{Kind: "terminal-attempt", Owner: id, Action: "Next-false-implicit-close"}); err != nil {
			return false, err
		}
	}
	return next, nil
}
func manualReadOnlyWALNativeRows(r *manualReadOnlyWALNativeRun, parent int, early bool) error {
	if err := manualReadOnlyWALNativeBoundPaths(r); err != nil {
		return err
	}
	id, err := manualReadOnlyWALNativeReserve(r, parent, "rows", "fixed-select")
	if err != nil {
		return err
	}
	var rows *sqlite.Rows
	var original error
	if r.Ledger.Owners[parent].Kind == "db" {
		rows, original = r.Owners[parent].DB.Query(r.APICtx, manualReadOnlyWALQuery)
	} else {
		rows, original = r.Owners[parent].Tx.Query(r.APICtx, manualReadOnlyWALQuery)
	}
	r.Owners[id].Rows, r.Owners[id].Original = rows, original
	capture := manualReadOnlyWALAdopt(&r.Ledger, id, rows != nil, original != nil, original != nil)
	if original != nil || capture != nil {
		return manualReadOnlyWALNativeFail(r, "query acquisition ownership", original)
	}
	if err := manualReadOnlyWALNativeBoundPaths(r); err != nil {
		return err
	}
	if !slices.Equal(rows.Columns(), []string{"id", "text"}) {
		return manualReadOnlyWALNativeFail(r, "exact query columns", nil)
	}
	limit := 2
	if early {
		limit = 1
	}
	for i := 0; i < limit; i++ {
		next, err := manualReadOnlyWALNativeNext(r, id)
		if err != nil {
			return err
		}
		if !next {
			return manualReadOnlyWALNativeFail(r, "early EOF assertion", nil)
		}
		values := rows.Values()
		if len(values) != 2 {
			return manualReadOnlyWALNativeFail(r, "row width", nil)
		}
		key, keyOK := values[0].(int64)
		value, valueOK := values[1].(string)
		want := "current"
		if i == 1 {
			want = "stable"
		}
		if !keyOK || !valueOK || key != int64(i+1) || value != want {
			return manualReadOnlyWALNativeFail(r, "fixed row assertion", nil)
		}
	}
	if early {
		return manualReadOnlyWALNativeEnd(r, id, "close")
	}
	next, err := manualReadOnlyWALNativeNext(r, id)
	if err != nil {
		return err
	}
	if next {
		return manualReadOnlyWALNativeFail(r, "unexpected third row", nil)
	}
	// EOF already consumed cleanup. No Close follows this Next(false).
	return nil
}
func manualReadOnlyWALNativeSchemaOK(schema []sqlite.SchemaEntry) bool {
	if len(schema) != 2 {
		return false
	}
	table, index := false, false
	for _, entry := range schema {
		if entry == (sqlite.SchemaEntry{Type: "table", Name: "t", TableName: "t", SQL: "CREATE TABLE t(id INTEGER PRIMARY KEY,text TEXT)", RootPage: 2}) {
			if table {
				return false
			}
			table = true
			continue
		}
		if entry == (sqlite.SchemaEntry{Type: "index", Name: "ix", TableName: "t", SQL: "CREATE INDEX ix ON t(text)", RootPage: 3}) {
			if index {
				return false
			}
			index = true
			continue
		}
		return false
	}
	return table && index
}
func manualReadOnlyWALNativeReadBegin(r *manualReadOnlyWALNativeRun, parent int, label string) (int, error) {
	if err := manualReadOnlyWALNativeBoundPaths(r); err != nil {
		return -1, err
	}
	id, err := manualReadOnlyWALNativeReserve(r, parent, "call", label)
	if err != nil {
		return -1, err
	}
	if err := manualReadOnlyWALAdopt(&r.Ledger, id, true, false, false); err != nil {
		return -1, err
	}
	if err := manualReadOnlyWALBeginEnd(&r.Ledger, id, "read"); err != nil {
		return -1, err
	}
	return id, nil
}
func manualReadOnlyWALNativeReadFinish(r *manualReadOnlyWALNativeRun, id int, original error) error {
	r.Owners[id].Original, r.Owners[id].Terminal = original, original
	decision := manualReadOnlyWALFinishEnd(&r.Ledger, id, original != nil)
	if original != nil || decision != nil {
		return manualReadOnlyWALNativeFail(r, "read API internal cleanup ownership unknown", original)
	}
	if err := manualReadOnlyWALNativeBoundPaths(r); err != nil {
		return err
	}
	return manualReadOnlyWALNativeEventWrite(r, manualReadOnlyWALNativeEvent{Kind: "terminal-attempt", Owner: id, Action: "read-api-hidden-owner"})
}
func manualReadOnlyWALNativeMetadata(r *manualReadOnlyWALNativeRun, dbID int) error {
	id, err := manualReadOnlyWALNativeReadBegin(r, dbID, "schema")
	if err != nil {
		return err
	}
	schema, original := r.Owners[dbID].DB.Schema(r.APICtx)
	r.Owners[id].Original = original
	if err := manualReadOnlyWALNativeReadFinish(r, id, original); err != nil {
		return err
	}
	if !manualReadOnlyWALNativeSchemaOK(schema) {
		return manualReadOnlyWALNativeFail(r, "exact schema", nil)
	}
	id, err = manualReadOnlyWALNativeReadBegin(r, dbID, "inspect")
	if err != nil {
		return err
	}
	inspection, original := r.Owners[dbID].DB.Inspect(r.APICtx, sqlite.InspectOptions{IncludeExtensions: true})
	r.Owners[id].Original = original
	if err := manualReadOnlyWALNativeReadFinish(r, id, original); err != nil {
		return err
	}
	h := inspection.Header
	if !manualReadOnlyWALNativeSchemaOK(inspection.Schema) || inspection.ExtensionIndexes == nil || len(inspection.ExtensionIndexes) != 0 || h.PageSize != 4096 || h.UsableSize != 4096 || h.PageCount != 3 || h.ReadVersion != 2 || h.WriteVersion != 2 || h.TextEncoding != 1 || h.SchemaFormat != 4 {
		return manualReadOnlyWALNativeFail(r, "exact inspect profile", nil)
	}
	id, err = manualReadOnlyWALNativeReadBegin(r, dbID, "check")
	if err != nil {
		return err
	}
	check, original := r.Owners[dbID].DB.Check(r.APICtx, sqlite.CheckOptions{MaxPages: 16, MaxPageReads: 256})
	r.Owners[id].Original = original
	if err := manualReadOnlyWALNativeReadFinish(r, id, original); err != nil {
		return err
	}
	if check.Scope != sqlite.CheckScope || check.Pages != 3 || check.Tables != 1 || check.Indexes != 1 || check.ExtensionIndexes != 0 || check.TableRows != 2 || check.IndexEntries != 2 || check.FreePages != 0 || check.ReservedPages != 0 || check.PageReads == 0 || check.PageReads > 256 {
		return manualReadOnlyWALNativeFail(r, "exact integrity profile", nil)
	}
	return manualReadOnlyWALNativeEventWrite(r, manualReadOnlyWALNativeEvent{Kind: "read-assertions", Owner: dbID, Status: "fixed SELECT rows 1=current,2=stable; schema/inspect/check exact", Schema: schema, Header: &h, Check: &check})
}
func manualReadOnlyWALNativeTransaction(r *manualReadOnlyWALNativeRun, dbID int, commit bool) error {
	id, err := manualReadOnlyWALNativeReserve(r, dbID, "tx", "deferred-read-transaction")
	if err != nil {
		return err
	}
	tx, original := r.Owners[dbID].DB.Begin(r.APICtx, sqlite.TxOptions{})
	r.Owners[id].Tx, r.Owners[id].Original = tx, original
	capture := manualReadOnlyWALAdopt(&r.Ledger, id, tx != nil, original != nil, original != nil)
	if original != nil || capture != nil {
		return manualReadOnlyWALNativeFail(r, "transaction acquisition ownership", original)
	}
	if err := manualReadOnlyWALNativeRows(r, id, !commit); err != nil {
		return err
	}
	if commit {
		return manualReadOnlyWALNativeEnd(r, id, "commit")
	}
	return manualReadOnlyWALNativeEnd(r, id, "rollback")
}
func manualReadOnlyWALNativeCase(r *manualReadOnlyWALNativeRun, keep bool) error {
	if err := manualReadOnlyWALNativeReceipt(r); err != nil {
		return err
	}
	caseID := "keep-false"
	if keep {
		caseID = "keep-true"
	}
	before, err := manualReadOnlyWALNativeSnapshotNow(r)
	if err != nil {
		return err
	}
	if r.Baseline != nil && !manualReadOnlyWALNativeSameSnapshot(*r.Baseline, before) {
		return manualReadOnlyWALNativeFail(r, "between-case source preservation", nil)
	}
	if err := manualReadOnlyWALNativeEventWrite(r, manualReadOnlyWALNativeEvent{Kind: "before-case", Phase: caseID, Owner: -1, Observation: &before}); err != nil {
		return err
	}
	r.Baseline = &before
	r.Started = true
	apiCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r.APICtx = apiCtx
	id, err := manualReadOnlyWALNativeReserve(r, r.Source, "db", "readonly-shared-WAL")
	if err != nil {
		return err
	}
	db, original := sqlite.OpenContext(r.APICtx, filepath.Join(r.Manifest.SourceRoot, "readonly.db"), sqlite.Options{ReadOnly: true, NoCreate: true, WALShared: true, KeepWALOnClose: keep, MaxSortBytes: 1 << 20})
	r.Owners[id].DB, r.Owners[id].Original = db, original
	capture := manualReadOnlyWALAdopt(&r.Ledger, id, db != nil, original != nil, original != nil)
	if original != nil || capture != nil {
		return manualReadOnlyWALNativeFail(r, "readonly Open ownership", original)
	}
	// Revalidate managed-open path binding without raw lock/DMS probes.
	if err := manualReadOnlyWALNativePathMatches(r, unix.AT_FDCWD, r.Manifest.SourceRoot, before.Stats[0]); err != nil {
		return err
	}
	for i, member := range r.Manifest.Family {
		if err := manualReadOnlyWALNativePathMatches(r, r.Owners[r.Source].FD, member.Name, before.Stats[i+1]); err != nil {
			return err
		}
	}
	if err := manualReadOnlyWALNativeRows(r, id, false); err != nil {
		return err
	}
	if err := manualReadOnlyWALNativeMetadata(r, id); err != nil {
		return err
	}
	if err := manualReadOnlyWALNativeRows(r, id, true); err != nil {
		return err
	}
	if err := manualReadOnlyWALNativeTransaction(r, id, true); err != nil {
		return err
	}
	if err := manualReadOnlyWALNativeTransaction(r, id, false); err != nil {
		return err
	}
	if err := manualReadOnlyWALNativeEnd(r, id, "close"); err != nil {
		return err
	}
	if len(sqlite.PendingCloseCleanups()) != 0 || len(sqlite.PendingPlaintextCleanups()) != 0 {
		r.Ledger.Blocked = true
		return manualReadOnlyWALNativeFail(r, "pending cleanup after DB close", nil)
	}
	after, err := manualReadOnlyWALNativeSnapshotNow(r)
	if err != nil {
		return err
	}
	if !manualReadOnlyWALNativeSameSnapshot(before, after) {
		return manualReadOnlyWALNativeFail(r, "source byte/name/identity preservation", nil)
	}
	if err := manualReadOnlyWALNativeReceipt(r); err != nil {
		return err
	}
	if err := manualReadOnlyWALNativeEventWrite(r, manualReadOnlyWALNativeEvent{Kind: "after-case", Phase: caseID, Owner: id, Observation: &after, Status: "bounded read case complete"}); err != nil {
		return err
	}
	r.Baseline = &after
	return nil
}
func manualReadOnlyWALNativeRunPrepared(r *manualReadOnlyWALNativeRun) error {
	manifestPath := os.Getenv("GOSQLITE_MANUAL_READONLY_WAL_MANIFEST")
	manifestDigest := os.Getenv("GOSQLITE_MANUAL_READONLY_WAL_MANIFEST_SHA256")
	if !manualReadOnlyWALPath(manifestPath) || !manualReadOnlyWALHex(manifestDigest, 64) {
		return manualReadOnlyWALNativeFail(r, "explicit manifest controls", nil)
	}
	r.ControlPath, r.ManifestName = filepath.Dir(manifestPath), filepath.Base(manifestPath)
	if !manualReadOnlyWALPath(r.ControlPath) {
		return manualReadOnlyWALNativeFail(r, "nonroot control directory required", nil)
	}
	build, err := manualReadOnlyWALNativeBuild(r)
	if err != nil {
		return err
	}
	control, err := manualReadOnlyWALNativeDirectory(r, r.ControlPath, "control-directory")
	r.Control = control
	if err != nil {
		return err
	}
	manifestFile, err := manualReadOnlyWALNativeRelative(r, control, r.ManifestName, "control-manifest", false)
	r.ManifestFile = manifestFile
	if err != nil {
		return err
	}
	stat, err := manualReadOnlyWALNativeStatFD(r, manifestFile)
	if err != nil {
		return err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Links != 1 || stat.Size > 32768 {
		return manualReadOnlyWALNativeFail(r, "manifest regular bounded file", nil)
	}
	controlStat, err := manualReadOnlyWALNativeStatFD(r, control)
	if err != nil {
		return err
	}
	if controlStat.Mode&unix.S_IFMT != unix.S_IFDIR {
		return manualReadOnlyWALNativeFail(r, "control directory type", nil)
	}
	r.ControlStat, r.ManifestStat = controlStat, stat
	if err := manualReadOnlyWALNativeControlMatches(r); err != nil {
		return err
	}
	raw, err := manualReadOnlyWALNativeRead(r, manifestFile, 32768)
	if err != nil {
		return err
	}
	if err := manualReadOnlyWALNativeControlMatches(r); err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != manifestDigest {
		return manualReadOnlyWALNativeFail(r, "reviewed manifest digest", nil)
	}
	m, err := manualReadOnlyWALDecodeManifest(raw)
	if err != nil {
		return manualReadOnlyWALNativeFail(r, "manifest exact decoder", err)
	}
	r.Manifest, r.RawManifest = m, raw
	if !manualReadOnlyWALSeparate(m.SourceRoot, r.ControlPath) || !manualReadOnlyWALSeparate(m.EvidenceParent, r.ControlPath) || stat.UID != m.ReaderUID {
		return manualReadOnlyWALNativeFail(r, "separate owned control input", nil)
	}
	var pipe unix.Stat_t
	if err := unix.Fstat(0, &pipe); err != nil {
		return manualReadOnlyWALNativeFail(r, "borrowed stdin observation", err)
	}
	if pipe.Mode&unix.S_IFMT != unix.S_IFIFO {
		return manualReadOnlyWALNativeFail(r, "prepared status-only stdin pipe required", nil)
	}
	r.Phase = manualReadOnlyWALPhase{RunID: m.RunID, Session: m.Keeper.Session, Generation: m.Keeper.Generation}
	source, err := manualReadOnlyWALNativeDirectory(r, m.SourceRoot, "source-directory")
	r.Source = source
	if err != nil {
		return err
	}
	for i, member := range m.Family {
		id, err := manualReadOnlyWALNativeRelative(r, source, member.Name, "source-observation", false)
		r.Family[i] = id
		if err != nil {
			return err
		}
	}
	evidence, err := manualReadOnlyWALNativeDirectory(r, m.EvidenceParent, "evidence-parent")
	r.Evidence = evidence
	if err != nil {
		return err
	}
	admission, err := manualReadOnlyWALNativeMetadataNow(r)
	if err != nil {
		return err
	}
	if err := manualReadOnlyWALNativeEvidence(r); err != nil {
		return err
	}
	if err := manualReadOnlyWALNativeEventWrite(r, manualReadOnlyWALNativeEvent{Kind: "admission", Owner: -1, Manifest: &r.Manifest, Observation: &admission, Build: build, Status: "external source/review/binary binding required; public cleanup snapshots are not zero-live-owner proof"}); err != nil {
		return err
	}
	if err := manualReadOnlyWALNativeCase(r, false); err != nil {
		return err
	}
	if err := manualReadOnlyWALNativeCase(r, true); err != nil {
		return err
	}
	if r.Phase.Next != 4 {
		return manualReadOnlyWALNativeFail(r, "four receipts required", nil)
	}
	final, err := manualReadOnlyWALNativeSnapshotNow(r)
	if err != nil {
		return err
	}
	if r.Baseline == nil || !manualReadOnlyWALNativeSameSnapshot(*r.Baseline, final) {
		return manualReadOnlyWALNativeFail(r, "final preservation/credentials", nil)
	}
	return manualReadOnlyWALNativeEventWrite(r, manualReadOnlyWALNativeEvent{Kind: "pre-final", Owner: -1, Observation: &final, Status: "all assertions complete; explicit retirement and natural exit pending"})
}
func manualReadOnlyWALNativeFinish(t *testing.T, r *manualReadOnlyWALNativeRun) {
	// Panic/incomplete acquisitions or in-flight Next are retained, never ended
	// again. A dependency barrier prevents cascading parent cleanup.
	for i := range r.Ledger.Owners {
		state := &r.Ledger.Owners[i]
		if state.State == manualReadOnlyWALReserved || state.State == manualReadOnlyWALConsuming {
			state.State = manualReadOnlyWALUnknown
			r.Ledger.Blocked = true
		}
	}
	// Independent known-live owners may retire once even after another owner
	// became unknown. No reserve/snapshot/DB operation starts on this path.
	for id := len(r.Owners) - 1; id >= 0; id-- {
		if r.Ledger.Owners[id].State != manualReadOnlyWALLive {
			continue
		}
		blocked := false
		for _, child := range r.Ledger.Owners {
			if child.Parent == id && child.State != manualReadOnlyWALEnded {
				blocked = true
			}
		}
		if blocked {
			continue
		}
		action := "close"
		if r.Ledger.Owners[id].Kind == "tx" {
			action = "rollback"
		}
		if err := manualReadOnlyWALNativeEnd(r, id, action); err != nil {
			r.Errors = append(r.Errors, err)
		}
	}
	pending := len(sqlite.PendingCloseCleanups()) != 0 || len(sqlite.PendingPlaintextCleanups()) != 0
	status := "PRECONDITION_UNSUPPORTED_NOT_RUN"
	if r.Started {
		status = "FAIL"
	}
	if r.Ledger.Blocked || pending && !r.Preexisting || !manualReadOnlyWALRetired(r.Ledger) {
		status = "FAIL_OWNERSHIP_UNKNOWN"
	}
	eligible := r.Complete && r.Failure == "" && !r.OutputFailed && !pending && manualReadOnlyWALRetired(r.Ledger)
	if eligible {
		status = "PROFILE_CANDIDATE_PASS_NATURAL_EXIT_REQUIRED"
	}
	if err := manualReadOnlyWALNativeEventWrite(r, manualReadOnlyWALNativeEvent{Kind: "terminal", Owner: -1, Status: status, Failure: r.Failure, Errno: r.LastErrno, Candidate: r.Candidate, States: r.Ledger.Owners}); err != nil {
		eligible = false
	}
	if r.Locked {
		runtime.UnlockOSThread()
		r.Locked = false
	}
	if !eligible {
		t.Error("prepared readonly WAL did not qualify; retain all input/evidence and original errors; no retry or keeper release")
	}
}
