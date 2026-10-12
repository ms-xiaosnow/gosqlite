//go:build manual_wal_publication && linux

package sqlite_test

// SOURCE ONLY / NOT_RUN. This finite native harness is deliberately outside
// ordinary/oracle/CI selection. A build tag is not execution authorization.
// Every file survives PASS and FAIL; no crash, fault injection or child process.
// See docs/testing/wal-tail-publication-manual.md before independent manual use.

import (
	"bytes"
	"context"
	"crypto/sha256"
	sqldriver "database/sql/driver"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	gosqldriver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
	"github.com/ms-xiaosnow/gosqlite/internal/wal"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"golang.org/x/sys/unix"
)

const manualWALPageSize = 4096
const manualWALStatMask = unix.STATX_TYPE | unix.STATX_MODE | unix.STATX_UID | unix.STATX_NLINK | unix.STATX_MTIME | unix.STATX_CTIME | unix.STATX_INO | unix.STATX_SIZE | unix.STATX_MNT_ID_UNIQUE
const manualWALResolve = unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_XDEV

// This inert root keeps references alive after an unknown close. It does not
// open resources at package initialization. OS process exit still ends owners;
// this harness does not claim to preserve in-process locks after process exit.
var manualWALRetained []*manualWALState

type manualWALOwner struct {
	name string
	priority int // rows, database, observation files, retained directories
	value any
	close func() error
	attempted bool
}

type manualWALState struct {
	owners []*manualWALOwner
	blocked bool
	root string
	rootFile *os.File
	rootFact unix.Statx_t
	profile manualWALProfile
}

type manualWALProfile struct {
	Version int `json:"version"`
	Root string `json:"root"`
	KernelRelease string `json:"kernel_release"`
	Architecture string `json:"architecture"`
	UID uint32 `json:"uid"`
	MountInfoLine string `json:"mountinfo_line"`
	SectorBytes uint32 `json:"sector_bytes"`
	StorageEvidence string `json:"storage_evidence"`
	Qualification string `json:"qualification"`
}

type manualWALFixture struct {
	path string
	main, tail, published []byte
	mainFile, oldWAL *os.File
	mainFact, oldFact unix.Statx_t
	inputs map[string][]byte
}

func TestManualPublicWALTailPublication(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_WAL_PUBLICATION") != "independently_authorized" {
		t.Fatal("NOT_RUN: explicit independent manual WAL publication authorization is required")
	}
	if flag.Lookup("test.run").Value.String() != "^TestManualPublicWALTailPublication$" ||
		flag.Lookup("test.count").Value.String() != "1" ||
		flag.Lookup("test.timeout").Value.String() != "0s" ||
		flag.Lookup("test.cpu").Value.String() != "" ||
		flag.Lookup("test.bench").Value.String() != "" ||
		flag.Lookup("test.shuffle").Value.String() != "off" {
		t.Fatal("NOT_RUN: require exact top-level selector, count=1 and timeout=0; no narrowed subtests or timeout kill")
	}
	s := &manualWALState{}
	manualWALRetained = append(manualWALRetained, s)
	t.Cleanup(func() {
		s.checkPending(t)
		s.retire(t, 3)
		if s.blocked { t.Errorf("UNKNOWN ownership: retained references and all artifacts; no cleanup retry") }
	})
	s.checkPending(t)
	if s.blocked { t.Fatal("NOT_RUN: existing production owner") }
	s.preflight(t)
	t.Logf("retained root=%s; profile=%+v; all physical scenarios were NOT_RUN before this explicit invocation", s.root, s.profile)
	cases := []struct { name, entrance string }{
		{"nil-default-preserves-input", ""},
		{"open-publishes-writes-reopens", "Open"},
		{"open-context-publishes-writes-reopens", "OpenContext"},
		{"connector-publishes-writes-reopens", "Connector"},
	}
	for _, scenario := range cases {
		if !t.Run(scenario.name, func(t *testing.T) {
			s.checkPending(t)
			s.recheckRoot(t)
			f := s.fixture(t, scenario.name)
			if scenario.entrance == "" {
				for _, entrance := range []string{"Open", "OpenContext", "Connector"} {
					s.refuse(t, f, entrance)
				}
			} else {
				s.publish(t, f, scenario.entrance)
			}
			s.retire(t, 2)
			if s.blocked { t.Fatal("resource retirement failed; stop remaining scenarios") }
			s.checkPending(t)
			for name, want := range f.inputs {
				got := s.readPath(t, filepath.Join(filepath.Dir(f.path), name), 1<<20)
				if !bytes.Equal(got, want) { t.Fatalf("retained input changed: %s", name) }
			}
			s.recheckRoot(t)
			t.Log("ASSERT all input hashes, retirement and pending-owner checks complete")
		}) { return }
	}
}

func (s *manualWALState) own(name string, priority int, value any, close func() error) *manualWALOwner {
	o := &manualWALOwner{name: name, priority: priority, value: value, close: close}
	s.owners = append(s.owners, o)
	return o
}

func (s *manualWALState) closeOnce(t *testing.T, o *manualWALOwner) {
	t.Helper()
	if s.blocked || o.attempted { return }
	o.attempted = true
	if err := o.close(); err != nil {
		s.blocked = true
		t.Errorf("UNKNOWN close %s: %v; keep owner references, parents and artifacts", o.name, err)
		return
	}
	t.Logf("RETIRE once: %s", o.name)
}

func (s *manualWALState) retire(t *testing.T, through int) {
	t.Helper()
	for priority := 0; priority <= through && !s.blocked; priority++ {
		for i := len(s.owners)-1; i >= 0 && !s.blocked; i-- {
			if s.owners[i].priority == priority { s.closeOnce(t, s.owners[i]) }
		}
	}
}

func (s *manualWALState) checkPending(t *testing.T) {
	t.Helper()
	if pending := sqlite.PendingPlaintextCleanups(); len(pending) != 0 {
		s.blocked = true
		t.Errorf("retained production owners: %v; do not retry cleanup", pending)
	}
	if s.blocked && !t.Failed() { t.Error("retained ownership is uncertain") }
}

func manualWALASCIIPath(path string) bool {
	if path == "/" || !filepath.IsAbs(path) || filepath.Clean(path) != path { return false }
	for i := range path {
		c := path[i]
		if !(c == '/' || c == '.' || c == '_' || c == '-' || c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z') { return false }
	}
	return true
}

func manualWALNoSymlink(t *testing.T, path string) {
	t.Helper()
	if !manualWALASCIIPath(path) { t.Fatalf("NOT_RUN: require clean absolute ASCII path: %q", path) }
	current := "/"
	for _, component := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 { t.Fatalf("NOT_RUN: path component %s: %v", current, err) }
	}
}

func (s *manualWALState) openFile(t *testing.T, path string, flags int, priority int) (*os.File, *manualWALOwner) {
	t.Helper()
	if s.blocked { t.Fatal("ownership blocked; no new resources") }
	f, err := os.OpenFile(path, flags|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0600)
	if err != nil { t.Fatalf("open %s: %v", path, err) }
	return f, s.own(path, priority, f, f.Close)
}

func (s *manualWALState) readPath(t *testing.T, path string, limit int64) []byte {
	t.Helper()
	f, owner := s.openFile(t, path, os.O_RDONLY, 2)
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	s.closeOnce(t, owner)
	if err != nil || int64(len(data)) > limit || s.blocked { t.Fatalf("bounded read %s: bytes=%d error=%v", path, len(data), err) }
	return data
}

func (s *manualWALState) fact(t *testing.T, f *os.File) unix.Statx_t {
	t.Helper()
	var fact unix.Statx_t
	if err := unix.Statx(int(f.Fd()), "", unix.AT_EMPTY_PATH|unix.AT_SYMLINK_NOFOLLOW, manualWALStatMask, &fact); err != nil {
		t.Fatalf("NOT_RUN/FAIL statx unique mount identity: %v", err)
	}
	if fact.Mask&manualWALStatMask != manualWALStatMask || fact.Ino == 0 || fact.Mnt_id == 0 {
		t.Fatalf("NOT_RUN/FAIL missing actual STATX_MNT_ID_UNIQUE fields: %+v", fact)
	}
	return fact
}

func manualWALSame(a, b unix.Statx_t) bool {
	return a.Ino == b.Ino && a.Dev_major == b.Dev_major && a.Dev_minor == b.Dev_minor && a.Mnt_id == b.Mnt_id
}

func (s *manualWALState) preflight(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" || runtime.Version() != "go1.27.2" { t.Fatal("NOT_RUN: pinned Linux / Go 1.27.2 required") }
	s.root = os.Getenv("GOSQLITE_MANUAL_WAL_PUBLICATION_DIR")
	manualWALNoSymlink(t, s.root)
	s.rootFile, _ = s.openFile(t, s.root, os.O_RDONLY|unix.O_DIRECTORY, 3)
	s.rootFact = s.fact(t, s.rootFile)
	if s.rootFact.Mode&unix.S_IFMT != unix.S_IFDIR || s.rootFact.Mode&07777 != 0700 || s.rootFact.Uid != uint32(os.Geteuid()) {
		t.Fatal("NOT_RUN: fixture root must be user-owned private mode 0700 directory")
	}
	entries, err := s.rootFile.ReadDir(-1)
	if err != nil || len(entries) != 0 { t.Fatalf("NOT_RUN: fixture root must be empty: %v entries=%d", err, len(entries)) }
	profilePath := os.Getenv("GOSQLITE_MANUAL_WAL_PUBLICATION_PROFILE")
	manualWALNoSymlink(t, profilePath)
	if filepath.Dir(profilePath) != filepath.Dir(s.root) { t.Fatal("NOT_RUN: qualification JSON must be a sibling of fixture root") }
	profileFile, owner := s.openFile(t, profilePath, os.O_RDONLY, 2)
	profileFact := s.fact(t, profileFile)
	if profileFact.Mode&unix.S_IFMT != unix.S_IFREG || profileFact.Mode&07777 != 0600 || profileFact.Uid != uint32(os.Geteuid()) || profileFact.Nlink != 1 || profileFact.Size > 65536 {
		t.Fatal("NOT_RUN: private, single-link, bounded user-owned qualification JSON required")
	}
	profileRaw, err := io.ReadAll(io.LimitReader(profileFile, 65537))
	s.closeOnce(t, owner)
	if err != nil || s.blocked || len(profileRaw) > 65536 { t.Fatalf("qualification read: %v", err) }
	decoder := json.NewDecoder(bytes.NewReader(profileRaw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&s.profile); err != nil { t.Fatalf("NOT_RUN: qualification JSON: %v", err) }
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF { t.Fatal("NOT_RUN: trailing qualification content") }
	var u unix.Utsname
	if err := unix.Uname(&u); err != nil { t.Fatal(err) }
	release := strings.TrimRight(string(u.Release[:]), "\x00")
	p := s.profile
	if p.Version != 1 || p.Root != s.root || p.KernelRelease != release || p.Architecture != runtime.GOARCH || p.UID != uint32(os.Geteuid()) ||
		p.SectorBytes != manualWALPageSize || strings.TrimSpace(p.StorageEvidence) == "" ||
		p.Qualification != "independently_verified_durable_private_ext_storage" {
		t.Fatal("NOT_RUN: explicit kernel/architecture/uid/root/4096-sector/storage qualification pins do not match")
	}
	var fs unix.Statfs_t
	if err := unix.Fstatfs(int(s.rootFile.Fd()), &fs); err != nil { t.Fatal(err) }
	if fs.Type != unix.EXT4_SUPER_MAGIC || fs.Flags&unix.ST_RDONLY != 0 { t.Fatal("NOT_RUN: writable ext-family filesystem required; no overlay/tmpfs fallback") }
	mounts := s.readPath(t, "/proc/self/mountinfo", 4<<20)
	best := ""
	bestLength := -1
	for _, line := range strings.Split(string(mounts), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 { continue }
		mount := fields[4]
		if (s.root == mount || strings.HasPrefix(s.root, strings.TrimSuffix(mount, "/")+"/")) && len(mount) > bestLength {
			best, bestLength = line, len(mount)
		}
	}
	parts := strings.Split(best, " - ")
	if best == "" || best != p.MountInfoLine || len(parts) != 2 { t.Fatal("NOT_RUN: exact current mountinfo qualification mismatch") }
	left, right := strings.Fields(parts[0]), strings.Fields(parts[1])
	if len(left) < 6 || len(right) < 3 || (right[0] != "ext2" && right[0] != "ext3" && right[0] != "ext4") ||
		left[2] != fmt.Sprintf("%d:%d", s.rootFact.Dev_major, s.rootFact.Dev_minor) || !strings.Contains(","+left[5]+",", ",rw,") ||
		strings.Contains(best, "volatile") || strings.Contains(best, "nobarrier") || strings.Contains(best, "barrier=0") {
		t.Fatal("NOT_RUN: unsupported/unqualified mount properties")
	}
	if fs.Bsize <= 0 || fs.Bavail < (uint64(2)<<30)/uint64(fs.Bsize) || fs.Ffree < 10000 { t.Fatal("NOT_RUN: fixture filesystem requires >=2 GiB available and >=10000 free inodes") }
	how := unix.OpenHow{Flags: unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, Resolve: manualWALResolve}
	fd, err := unix.Openat2(int(s.rootFile.Fd()), ".", &how)
	if err != nil { t.Fatalf("NOT_RUN: exact openat2 resolve constraints unavailable: %v", err) }
	probe := os.NewFile(uintptr(fd), "openat2-root-probe")
	if probe == nil { s.blocked = true; t.Fatalf("UNKNOWN raw fd=%d ownership; retain root", fd) }
	probeOwner := s.own("openat2-root-probe", 2, probe, probe.Close)
	if !manualWALSame(s.rootFact, s.fact(t, probe)) { t.Fatal("NOT_RUN: openat2 root identity mismatch") }
	s.closeOnce(t, probeOwner)
	if s.blocked { t.Fatal("NOT_RUN: qualification close failed") }
	s.checkPending(t)
	if t.Failed() { t.Fatal("NOT_RUN: existing retained owners") }
	s.write(t, filepath.Join(s.root, "qualification.input.json"), profileRaw)
	t.Logf("QUALIFIED admission only: kernel=%s go=%s arch=%s uid=%d dev=%d:%d inode=%d unique_mount=%d mask=%#x mountinfo=%s profileSHA256=%x", release, runtime.Version(), runtime.GOARCH, os.Geteuid(), s.rootFact.Dev_major, s.rootFact.Dev_minor, s.rootFact.Ino, s.rootFact.Mnt_id, s.rootFact.Mask, best, sha256.Sum256(profileRaw))
}

func (s *manualWALState) recheckRoot(t *testing.T) {
	t.Helper()
	if s.blocked { t.Fatal("uncertain owner; stop") }
	var named unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, s.root, unix.AT_SYMLINK_NOFOLLOW, manualWALStatMask, &named); err != nil { t.Fatal(err) }
	if named.Mask&manualWALStatMask != manualWALStatMask || !manualWALSame(named, s.rootFact) || !manualWALSame(s.fact(t, s.rootFile), s.rootFact) || named.Mode&07777 != 0700 || named.Uid != uint32(os.Geteuid()) {
		t.Fatal("root identity, mount, ownership or permissions changed; retain all artifacts")
	}
}

func (s *manualWALState) write(t *testing.T, path string, contents []byte) {
	t.Helper()
	f, owner := s.openFile(t, path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 2)
	n, writeErr := f.Write(contents)
	if n != len(contents) && writeErr == nil { writeErr = io.ErrShortWrite }
	syncErr := f.Sync()
	s.closeOnce(t, owner)
	if err := errors.Join(writeErr, syncErr); err != nil || s.blocked { t.Fatalf("fixture write/sync %s: %v", path, err) }
}

// Construct complete canonical table-leaf pages without an oracle process or
// the publisher's recipe. Physical main has row 1='base'; committed WAL has
// row 1='committed'; the whole checksum-valid pending page adds row 2. Expected
// rows are an independent fixed ledger, not inferred from Query's own output.
func manualWALLeaf(t *testing.T, header []byte, rows [][]any, rowids []uint64) []byte {
	t.Helper()
	page := make([]byte, manualWALPageSize)
	copy(page, header)
	start := len(header)
	page[start] = 13
	binary.BigEndian.PutUint16(page[start+3:], uint16(len(rows)))
	end := len(page)
	for i, values := range rows {
		payload, err := record.Encode(values)
		if err != nil { t.Fatal(err) }
		cell := record.AppendVarint(nil, uint64(len(payload)))
		cell = record.AppendVarint(cell, rowids[i])
		cell = append(cell, payload...)
		end -= len(cell)
		if end < start+8+2*len(rows) { t.Fatal("fixture leaf overflow") }
		copy(page[end:], cell)
		binary.BigEndian.PutUint16(page[start+8+2*i:], uint16(end))
	}
	binary.BigEndian.PutUint16(page[start+5:], uint16(end))
	return page
}

func (s *manualWALState) fixture(t *testing.T, name string) manualWALFixture {
	t.Helper()
	dir := filepath.Join(s.root, name)
	if err := os.Mkdir(dir, 0700); err != nil { t.Fatal(err) }
	dirFile, _ := s.openFile(t, dir, os.O_RDONLY|unix.O_DIRECTORY, 3)
	dirFact := s.fact(t, dirFile)
	if dirFact.Mnt_id != s.rootFact.Mnt_id || dirFact.Uid != s.rootFact.Uid || dirFact.Mode&07777 != 0700 { t.Fatal("fixture directory profile mismatch") }
	h := make([]byte, 100)
	copy(h, "SQLite format 3\x00")
	binary.BigEndian.PutUint16(h[16:], manualWALPageSize)
	h[18], h[19], h[21], h[22], h[23] = 2, 2, 64, 32, 32
	for offset, value := range map[int]uint32{24:1, 28:2, 40:1, 44:4, 56:1, 92:1, 96:3054000} { binary.BigEndian.PutUint32(h[offset:], value) }
	ddl := "CREATE TABLE t(id INTEGER PRIMARY KEY, value TEXT NOT NULL)"
	p1 := manualWALLeaf(t, h, [][]any{{"table", "t", "t", int64(2), ddl}}, []uint64{1})
	base := manualWALLeaf(t, nil, [][]any{{nil, "base"}}, []uint64{1})
	committed := manualWALLeaf(t, nil, [][]any{{nil, "committed"}}, []uint64{1})
	pending := manualWALLeaf(t, nil, [][]any{{nil, "committed"}, {nil, "uncommitted"}}, []uint64{1, 2})
	f := manualWALFixture{path:filepath.Join(dir, "publication.db"), main:append(append([]byte(nil), p1...), base...)}
	p1 = append([]byte(nil), p1...)
	binary.BigEndian.PutUint32(p1[24:], 2)
	binary.BigEndian.PutUint32(p1[92:], 2)
	encoded, err := wal.EncodeHeader(wal.Header{Magic:wal.MagicLittleEndian, Version:wal.Version, PageSize:manualWALPageSize, Salt:[2]uint32{0x12345678,0x56789abc}})
	if err != nil { t.Fatal(err) }
	wh, err := wal.ParseHeader(encoded)
	if err != nil { t.Fatal(err) }
	sum := wh.Checksum
	for i, page := range [][]byte{p1, committed} {
		dbSize := uint32(0)
		if i == 1 { dbSize = 2 }
		frame, next, err := wal.EncodeFrame(wh, uint32(i+1), dbSize, page, sum)
		if err != nil { t.Fatal(err) }
		encoded, sum = append(encoded, frame...), next
	}
	f.published = append([]byte(nil), encoded...)
	boundary := (len(encoded)+manualWALPageSize-1)/manualWALPageSize*manualWALPageSize
	padSum := sum
	for len(f.published) < boundary {
		frame, next, err := wal.EncodeFrame(wh, 2, 2, committed, padSum)
		if err != nil { t.Fatal(err) }
		f.published, padSum = append(f.published, frame...), next
	}
	tailFrame, _, err := wal.EncodeFrame(wh, 2, 0, pending, sum)
	if err != nil { t.Fatal(err) }
	f.tail = append(encoded, tailFrame...)
	snapshot, err := wal.Scan(context.Background(), bytes.NewReader(f.tail), int64(len(f.tail)), wal.Limits{})
	if !errors.Is(err, wal.ErrTail) || snapshot.Tail != wal.TailUncommitted || snapshot.FrameCount != 2 || snapshot.ValidFrameCount != 3 || snapshot.ValidEndOffset != int64(len(f.tail)) || snapshot.DBSize != 2 { t.Fatalf("fixture not a complete valid uncommitted tail: %+v %v", snapshot, err) }
	clean, err := wal.Scan(context.Background(), bytes.NewReader(f.published), int64(len(f.published)), wal.Limits{})
	if err != nil || !clean.Complete || clean.DBSize != 2 { t.Fatalf("expected complete publication bytes: %+v %v", clean, err) }
	ledger := []byte("before_publication: main row 1=base; WAL committed row 1=committed\nafter_publication: exactly [(1,committed)]; row 2=uncommitted must be absent\nafter_normal_insert_and_reopen: exactly [(1,committed),(3,after)]\n")
	f.inputs = map[string][]byte{"main.input.db":f.main, "wal.input.bin":f.tail, "published.expected.bin":f.published, "ledger.input.txt":ledger}
	hashes := map[string]string{}
	for name, data := range f.inputs { hashes[name] = fmt.Sprintf("%x", sha256.Sum256(data)) }
	manifest, err := json.MarshalIndent(hashes, "", "  ")
	if err != nil { t.Fatal(err) }
	f.inputs["input-sha256.json"] = append(manifest, '\n')
	for name, data := range f.inputs { s.write(t, filepath.Join(dir, name), data) }
	s.write(t, f.path, f.main)
	s.write(t, f.path+"-wal", f.tail)
	if err := errors.Join(dirFile.Sync(), s.rootFile.Sync()); err != nil { t.Fatalf("fixture directory sync: %v", err) }
	// Observation descriptors stay open until after DB/Conn retirement. In
	// particular, closing another main fd while POSIX locks are held can drop
	// process-associated locks, so no read-and-close convenience helper is used.
	f.mainFile, _ = s.openFile(t, f.path, os.O_RDONLY, 2)
	f.oldWAL, _ = s.openFile(t, f.path+"-wal", os.O_RDONLY, 2)
	f.mainFact, f.oldFact = s.fact(t, f.mainFile), s.fact(t, f.oldWAL)
	for _, fact := range []unix.Statx_t{f.mainFact, f.oldFact} {
		if fact.Mode&unix.S_IFMT != unix.S_IFREG || fact.Nlink != 1 || fact.Uid != s.rootFact.Uid || fact.Mnt_id != s.rootFact.Mnt_id || fact.Dev_major != s.rootFact.Dev_major || fact.Dev_minor != s.rootFact.Dev_minor { t.Fatal("fixture file profile mismatch") }
	}
	if manualWALSame(f.mainFact, f.oldFact) { t.Fatal("main and WAL unexpectedly alias") }
	t.Logf("FIXTURE %s main_inode=%d old_wal_inode=%d mainSHA256=%x WALSHA256=%x expectedSHA256=%x", dir, f.mainFact.Ino, f.oldFact.Ino, sha256.Sum256(f.main), sha256.Sum256(f.tail), sha256.Sum256(f.published))
	return f
}

func (s *manualWALState) equalHeld(t *testing.T, file *os.File, want []byte) {
	t.Helper()
	info, err := file.Stat()
	if err != nil || info.Size() != int64(len(want)) { t.Fatalf("held extent %s: %v", file.Name(), err) }
	got := make([]byte, len(want))
	n, err := file.ReadAt(got, 0)
	if n != len(got) || (err != nil && err != io.EOF) || !bytes.Equal(got, want) { t.Fatalf("held bytes %s: n=%d err=%v SHA256=%x want=%x", file.Name(), n, err, sha256.Sum256(got), sha256.Sum256(want)) }
}

func (s *manualWALState) namedFact(t *testing.T, path string) unix.Statx_t {
	t.Helper()
	var fact unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, manualWALStatMask, &fact); err != nil { t.Fatal(err) }
	if fact.Mask&manualWALStatMask != manualWALStatMask { t.Fatal("named unique mount fields missing") }
	return fact
}

func (s *manualWALState) assertSource(t *testing.T, f manualWALFixture, oldNamed bool) {
	t.Helper()
	s.equalHeld(t, f.mainFile, f.main)
	s.equalHeld(t, f.oldWAL, f.tail)
	if !manualWALSame(s.namedFact(t, f.path), f.mainFact) { t.Fatal("main inode changed") }
	if oldNamed && !manualWALSame(s.namedFact(t, f.path+"-wal"), f.oldFact) { t.Fatal("refused WAL inode changed") }
	for _, suffix := range []string{"-journal", "-shm"} {
		if _, err := os.Lstat(f.path+suffix); !errors.Is(err, os.ErrNotExist) { t.Fatalf("unexpected sidecar %s: %v", suffix, err) }
	}
	directory, dirOwner := s.openFile(t, filepath.Dir(f.path), os.O_RDONLY|unix.O_DIRECTORY, 2)
	entries, err := directory.ReadDir(-1)
	s.closeOnce(t, dirOwner)
	if err != nil || s.blocked { t.Fatalf("fixture inventory: %v", err) }
	expected := map[string]bool{"publication.db":true, "publication.db-wal":true}
	for name := range f.inputs { expected[name] = true }
	if len(entries) != len(expected) { t.Fatalf("unexpected fixture namespace entries=%d expected=%d", len(entries), len(expected)) }
	for _, entry := range entries { if !expected[entry.Name()] { t.Fatalf("unexpected retained candidate/sidecar: %s", entry.Name()) } }
	s.checkPending(t)
	if t.Failed() { t.Fatal("source or owner check failed; stop") }
}

func manualWALOptions(enabled bool) sqlite.Options {
	opts := sqlite.Options{NoCreate:true, WALExclusive:true, KeepWALOnClose:true, SectorSize:manualWALPageSize}
	if enabled { opts.WALTailPublication = &sqlite.WALTailPublicationOptions{} }
	return opts
}

func (s *manualWALState) refuse(t *testing.T, f manualWALFixture, entrance string) {
	t.Helper()
	var err error
	var unexpected *manualWALOwner
	switch entrance {
	case "Open", "OpenContext":
		var db *sqlite.DB
		if entrance == "Open" { db, err = sqlite.Open(f.path, manualWALOptions(false)) } else { db, err = sqlite.OpenContext(context.Background(), f.path, manualWALOptions(false)) }
		if db != nil { unexpected = s.own("unexpected refusal DB", 1, db, db.Close) }
	case "Connector":
		connector, buildErr := gosqldriver.NewConnector(f.path, manualWALOptions(false))
		if buildErr != nil { t.Fatal(buildErr) }
		conn, connectErr := connector.Connect(context.Background())
		err = connectErr
		if conn != nil { unexpected = s.own("unexpected refusal Conn", 1, conn, conn.Close) }
	default: t.Fatal("unknown entrance")
	}
	s.checkPending(t)
	if unexpected != nil { s.closeOnce(t, unexpected); t.Fatalf("nil-default %s returned a handle: %v", entrance, err) }
	var diagnostic *sqlite.Error
	if !errors.Is(err, sqlite.ErrUnsupported) || !errors.Is(err, wal.ErrTail) || !errors.As(err, &diagnostic) || diagnostic.Code != sqlite.CodeError { t.Fatalf("nil-default %s diagnostic: %v", entrance, err) }
	s.assertSource(t, f, true)
	t.Logf("ASSERT nil-default %s refused unchanged: %v", entrance, err)
}

func (s *manualWALState) publish(t *testing.T, f manualWALFixture, entrance string) {
	t.Helper()
	var db *sqlite.DB
	var conn sqldriver.Conn
	var owner *manualWALOwner
	var err error
	switch entrance {
	case "Open": db, err = sqlite.Open(f.path, manualWALOptions(true))
	case "OpenContext": db, err = sqlite.OpenContext(context.Background(), f.path, manualWALOptions(true))
	case "Connector":
		connector, buildErr := gosqldriver.NewConnector(f.path, manualWALOptions(true))
		if buildErr != nil { t.Fatal(buildErr) }
		conn, err = connector.Connect(context.Background())
	default: t.Fatal("unknown entrance")
	}
	if db != nil { owner = s.own(entrance+" DB", 1, db, db.Close) }
	if conn != nil { owner = s.own("Connector Conn", 1, conn, conn.Close) }
	s.checkPending(t)
	if err != nil || owner == nil || s.blocked { t.Fatalf("public %s publication: %v; retain complete fixture", entrance, err) }
	s.assertSource(t, f, false)
	published, _ := s.openFile(t, f.path+"-wal", os.O_RDONLY, 2)
	publishedFact := s.fact(t, published)
	if manualWALSame(publishedFact, f.oldFact) || manualWALSame(publishedFact, f.mainFact) || publishedFact.Mnt_id != f.oldFact.Mnt_id || publishedFact.Nlink != 1 { t.Fatal("publication did not produce a distinct same-mount single-link WAL inode") }
	s.equalHeld(t, published, f.published)
	clean, scanErr := wal.Scan(context.Background(), published, int64(len(f.published)), wal.Limits{})
	if scanErr != nil || !clean.Complete || clean.Tail != wal.TailClean || clean.DBSize != 2 { t.Fatalf("published committed prefix invalid: %+v %v", clean, scanErr) }
	t.Logf("ASSERT publication main_inode=%d old_wal_inode=%d current_wal_inode=%d currentSHA256=%x complete_frames=%d", f.mainFact.Ino, f.oldFact.Ino, publishedFact.Ino, sha256.Sum256(f.published), clean.FrameCount)
	s.query(t, db, conn, [][]any{{int64(1), "committed"}})
	if db != nil { _, err = db.Exec(context.Background(), "INSERT INTO t VALUES(3,'after')") } else {
		execer, ok := conn.(sqldriver.ExecerContext)
		if !ok { t.Fatal("driver lacks ExecerContext") }
		_, err = execer.ExecContext(context.Background(), "INSERT INTO t VALUES(3,'after')", nil)
	}
	s.checkPending(t)
	if err != nil || s.blocked { t.Fatalf("ordinary committed write: %v", err) }
	s.query(t, db, conn, [][]any{{int64(1), "committed"}, {int64(3), "after"}})
	s.assertSource(t, f, false)
	s.closeOnce(t, owner)
	if s.blocked { t.Fatal("close failed; do not reopen") }
	s.checkPending(t)
	if s.blocked { t.Fatal("pending owner; do not reopen") }
	// Nil policy on the now-clean WAL must reopen through the ordinary API.
	reopened, err := sqlite.OpenContext(context.Background(), f.path, manualWALOptions(false))
	var reopenOwner *manualWALOwner
	if reopened != nil { reopenOwner = s.own("ordinary reopen DB", 1, reopened, reopened.Close) }
	s.checkPending(t)
	if err != nil || reopened == nil || s.blocked { t.Fatalf("ordinary nil-policy reopen: %v", err) }
	s.query(t, reopened, nil, [][]any{{int64(1), "committed"}, {int64(3), "after"}})
	s.closeOnce(t, reopenOwner)
	if s.blocked { t.Fatal("reopen close failed") }
	s.assertSource(t, f, false)
	t.Log("ASSERT ordinary write and nil-policy reopen expose exactly the committed ledger")
}

func (s *manualWALState) query(t *testing.T, db *sqlite.DB, conn sqldriver.Conn, want [][]any) {
	t.Helper()
	var got [][]any
	if db != nil {
		rows, err := db.Query(context.Background(), "SELECT id,value FROM t ORDER BY id")
		if err != nil { s.checkPending(t); t.Fatal(err) }
		owner := s.own("sqlite Rows", 0, rows, rows.Close)
		for rows.Next() {
			got = append(got, append([]any(nil), rows.Values()...))
			if len(got) > len(want) { t.Fatal("query exceeded finite expected row count") }
		}
		err = rows.Err()
		s.closeOnce(t, owner)
		if err != nil || s.blocked { t.Fatalf("sqlite rows: %v", err) }
	} else {
		queryer, ok := conn.(sqldriver.QueryerContext)
		if !ok { t.Fatal("driver lacks QueryerContext") }
		rows, err := queryer.QueryContext(context.Background(), "SELECT id,value FROM t ORDER BY id", nil)
		if err != nil { s.checkPending(t); t.Fatal(err) }
		owner := s.own("driver Rows", 0, rows, rows.Close)
		if !reflect.DeepEqual(rows.Columns(), []string{"id", "value"}) { t.Fatal("driver result columns") }
		for {
			values := make([]sqldriver.Value, 2)
			err = rows.Next(values)
			if err == io.EOF { break }
			if err != nil { t.Fatalf("driver Next: %v", err) }
			got = append(got, []any{values[0], values[1]})
			if len(got) > len(want) { t.Fatal("driver query exceeded finite expected row count") }
		}
		s.closeOnce(t, owner)
		if s.blocked { t.Fatal("driver Rows close failed") }
	}
	s.checkPending(t)
	if s.blocked || !reflect.DeepEqual(got, want) { t.Fatalf("ledger mismatch got=%#v want=%#v", got, want) }
	t.Logf("ASSERT query ledger exactly %#v; no uncommitted row", got)
}
