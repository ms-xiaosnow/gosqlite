package sqlite_test

// Descriptor-free shared decisions. The Linux opt-in root consumes these exact
// functions; default tests below never call a collector or database API.
import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

const manualReadOnlyWALContract = "already-held-live-dms-readmark-no-mutation-until-operator-release-v1"
const manualReadOnlyWALQuery = "SELECT id,text FROM t ORDER BY id"

type manualReadOnlyWALIdentity struct {
	Major uint32 `json:"dev_major"`
	Minor uint32 `json:"dev_minor"`
	Inode uint64 `json:"inode"`
}
type manualReadOnlyWALMember struct {
	Name   string `json:"name"`
	Size   uint64 `json:"size"`
	SHA256 string `json:"sha256"`
	Major  uint32 `json:"dev_major"`
	Minor  uint32 `json:"dev_minor"`
	Inode  uint64 `json:"inode"`
}
type manualReadOnlyWALManifest struct {
	Version        uint32                     `json:"version"`
	RunID          string                     `json:"run_id"`
	Profile        string                     `json:"profile"`
	FixtureProfile string                     `json:"fixture_profile"`
	SourceRoot     string                     `json:"source_root"`
	EvidenceParent string                     `json:"evidence_parent"`
	ReaderUID      uint32                     `json:"reader_uid"`
	ReaderGID      uint32                     `json:"reader_gid"`
	SourceTree     string                     `json:"source_tree"`
	NativeSHA256   string                     `json:"native_source_sha256"`
	PortableSHA256 string                     `json:"portable_source_sha256"`
	GuardSHA256    string                     `json:"guard_receipt_sha256"`
	BinarySHA256   string                     `json:"binary_sha256"`
	SourceIdentity manualReadOnlyWALIdentity  `json:"source_identity"`
	Family         [3]manualReadOnlyWALMember `json:"family"`
	Keeper         struct {
		Session    string `json:"session"`
		Generation string `json:"generation"`
		Contract   string `json:"contract"`
	} `json:"keeper"`
}
type manualReadOnlyWALReceipt struct {
	Version    uint32 `json:"version"`
	RunID      string `json:"run_id"`
	Phase      string `json:"phase"`
	Challenge  string `json:"challenge"`
	Session    string `json:"session"`
	Generation string `json:"generation"`
	Status     string `json:"status"`
}

func manualReadOnlyWALHex(s string, size int) bool {
	if len(s) != size {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func manualReadOnlyWALPath(s string) bool {
	if len(s) < 2 || len(s) > 1024 || s[0] != '/' || strings.HasSuffix(s, "/") || strings.Contains(s, "//") {
		return false
	}
	for _, c := range s {
		if c < 0x20 || c == 0x7f || c == '\\' {
			return false
		}
	}
	for _, part := range strings.Split(s, "/")[1:] {
		if part == "." || part == ".." || part == "" {
			return false
		}
	}
	return true
}
func manualReadOnlyWALSeparate(a, b string) bool {
	return a != b && !strings.HasPrefix(a, b+"/") && !strings.HasPrefix(b, a+"/")
}
func manualReadOnlyWALSession(s string) bool {
	if len(s) < 1 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// Token parsing rejects duplicates before Go struct decoding can fold keys.
// It never invokes user callbacks; values are only inert standard JSON data.
func manualReadOnlyWALJSONValue(d *json.Decoder, depth int, tokens *int) (any, error) {
	if depth > 8 || *tokens >= 512 {
		return nil, errors.New("JSON budget")
	}
	token, err := d.Token()
	*tokens++
	if err != nil {
		return nil, errors.New("JSON syntax")
	}
	switch x := token.(type) {
	case json.Delim:
		switch x {
		case '{':
			m := make(map[string]any)
			for d.More() {
				if *tokens >= 512 {
					return nil, errors.New("JSON tokens")
				}
				k, err := d.Token()
				*tokens++
				if err != nil {
					return nil, errors.New("JSON key")
				}
				key, ok := k.(string)
				if !ok || len(key) > 1024 {
					return nil, errors.New("JSON key")
				}
				if _, exists := m[key]; exists {
					return nil, errors.New("duplicate JSON key")
				}
				value, err := manualReadOnlyWALJSONValue(d, depth+1, tokens)
				if err != nil {
					return nil, err
				}
				m[key] = value
			}
			end, err := d.Token()
			*tokens++
			if err != nil || end != json.Delim('}') || *tokens > 512 {
				return nil, errors.New("JSON object")
			}
			return m, nil
		case '[':
			var values []any
			for d.More() {
				value, err := manualReadOnlyWALJSONValue(d, depth+1, tokens)
				if err != nil {
					return nil, err
				}
				values = append(values, value)
			}
			end, err := d.Token()
			*tokens++
			if err != nil || end != json.Delim(']') || *tokens > 512 {
				return nil, errors.New("JSON array")
			}
			return values, nil
		}
	case string:
		if len(x) <= 1024 {
			return x, nil
		}
	case json.Number:
		if _, err := strconv.ParseUint(string(x), 10, 64); err == nil && !strings.HasPrefix(string(x), "+") {
			return x, nil
		}
	}
	return nil, errors.New("JSON exact nonnull scalar required")
}
func manualReadOnlyWALDecodeObject(raw []byte, limit int) (map[string]any, error) {
	if len(raw) == 0 || len(raw) > limit || !utf8.Valid(raw) {
		return nil, errors.New("JSON bytes")
	}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.UseNumber()
	tokens := 0
	value, err := manualReadOnlyWALJSONValue(d, 1, &tokens)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(raw[d.InputOffset():])) != "" {
		return nil, errors.New("JSON trailing content")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("JSON object required")
	}
	return object, nil
}
func manualReadOnlyWALKeys(object map[string]any, keys string) bool {
	expected := strings.Fields(keys)
	if len(object) != len(expected) {
		return false
	}
	for _, key := range expected {
		if _, ok := object[key]; !ok {
			return false
		}
	}
	return true
}
func manualReadOnlyWALDecodeManifest(raw []byte) (manualReadOnlyWALManifest, error) {
	var m manualReadOnlyWALManifest
	o, err := manualReadOnlyWALDecodeObject(raw, 32768)
	if err != nil {
		return m, err
	}
	if !manualReadOnlyWALKeys(o, "version run_id profile fixture_profile source_root evidence_parent reader_uid reader_gid source_tree native_source_sha256 portable_source_sha256 guard_receipt_sha256 binary_sha256 source_identity family keeper") {
		return m, errors.New("manifest exact keys")
	}
	id, ok := o["source_identity"].(map[string]any)
	if !ok || !manualReadOnlyWALKeys(id, "dev_major dev_minor inode") {
		return m, errors.New("source identity keys")
	}
	keeper, ok := o["keeper"].(map[string]any)
	if !ok || !manualReadOnlyWALKeys(keeper, "session generation contract") {
		return m, errors.New("keeper keys")
	}
	family, ok := o["family"].([]any)
	if !ok || len(family) != 3 {
		return m, errors.New("family length")
	}
	for _, value := range family {
		member, ok := value.(map[string]any)
		if !ok || !manualReadOnlyWALKeys(member, "name size sha256 dev_major dev_minor inode") {
			return m, errors.New("family keys")
		}
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, errors.New("manifest field types or ranges")
	}
	if m.Version != 1 || !manualReadOnlyWALHex(m.RunID, 32) || m.FixtureProfile != "live-ro-wal-4096-v1" || (m.Profile != "nonprivileged-denied-v1" && m.Profile != "readonly-mount-v1") {
		return m, errors.New("manifest profile")
	}
	if !manualReadOnlyWALPath(m.SourceRoot) || !manualReadOnlyWALPath(m.EvidenceParent) || !manualReadOnlyWALSeparate(m.SourceRoot, m.EvidenceParent) || m.ReaderUID == 0 {
		return m, errors.New("manifest reader/paths")
	}
	if !manualReadOnlyWALHex(m.SourceTree, 40) || m.SourceIdentity.Inode == 0 || m.SourceIdentity.Major == 0 && m.SourceIdentity.Minor == 0 {
		return m, errors.New("manifest source identity")
	}
	for _, digest := range []string{m.NativeSHA256, m.PortableSHA256, m.GuardSHA256, m.BinarySHA256} {
		if !manualReadOnlyWALHex(digest, 64) {
			return m, errors.New("manifest digest")
		}
	}
	if !manualReadOnlyWALSession(m.Keeper.Session) || !manualReadOnlyWALHex(m.Keeper.Generation, 32) || m.Keeper.Contract != manualReadOnlyWALContract {
		return m, errors.New("keeper contract")
	}
	for i, name := range []string{"readonly.db", "readonly.db-wal", "readonly.db-shm"} {
		f := m.Family[i]
		if f.Name != name || !manualReadOnlyWALHex(f.SHA256, 64) || f.Inode == 0 || f.Major != m.SourceIdentity.Major || f.Minor != m.SourceIdentity.Minor {
			return m, errors.New("family identity")
		}
		if i == 0 && f.Size != 4096 || i == 1 && (f.Size < 32+24+4096 || f.Size > 1048576 || (f.Size-32)%(24+4096) != 0) || i == 2 && f.Size != 32768 {
			return m, errors.New("family geometry")
		}
		for j := 0; j < i; j++ {
			if m.Family[j].Inode == f.Inode {
				return m, errors.New("family alias")
			}
		}
		if f.Inode == m.SourceIdentity.Inode {
			return m, errors.New("directory/file alias")
		}
	}
	return m, nil
}
func manualReadOnlyWALDecodeReceipt(raw []byte) (manualReadOnlyWALReceipt, error) {
	var r manualReadOnlyWALReceipt
	o, err := manualReadOnlyWALDecodeObject(raw, 2048)
	if err != nil {
		return r, err
	}
	if !manualReadOnlyWALKeys(o, "version run_id phase challenge session generation status") {
		return r, errors.New("receipt exact keys")
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return r, errors.New("receipt types")
	}
	if r.Version != 1 || !manualReadOnlyWALHex(r.RunID, 32) || !manualReadOnlyWALHex(r.Challenge, 32) || !manualReadOnlyWALHex(r.Generation, 32) || !manualReadOnlyWALSession(r.Session) || r.Status != "still-held-quiescent" {
		return r, errors.New("receipt values")
	}
	if r.Phase != "keep-false-before" && r.Phase != "keep-false-after" && r.Phase != "keep-true-before" && r.Phase != "keep-true-after" {
		return r, errors.New("receipt phase")
	}
	return r, nil
}

type manualReadOnlyWALPhase struct {
	Next                                  int
	RunID, Session, Generation, Challenge string
	Used                                  [4]string
}

func manualReadOnlyWALAdvancePhase(s manualReadOnlyWALPhase, r manualReadOnlyWALReceipt) (manualReadOnlyWALPhase, error) {
	phases := [4]string{"keep-false-before", "keep-false-after", "keep-true-before", "keep-true-after"}
	if s.Next < 0 || s.Next >= len(phases) || !manualReadOnlyWALHex(s.Challenge, 32) || r.Version != 1 || r.Phase != phases[s.Next] || r.RunID != s.RunID || r.Session != s.Session || r.Generation != s.Generation || r.Challenge != s.Challenge || r.Status != "still-held-quiescent" {
		return s, errors.New("keeper phase refusal")
	}
	for i := 0; i < s.Next; i++ {
		if s.Used[i] == r.Challenge {
			return s, errors.New("keeper replay")
		}
	}
	s.Used[s.Next] = r.Challenge
	s.Next++
	s.Challenge = ""
	return s, nil
}

// Explicit observation bits prevent zero values, filemode guesses and missing
// syscall results from being admitted as successful kernel observations.
type manualReadOnlyWALCredentials struct {
	Observed                          bool
	UID, GID                          [4]uint32
	GroupsMatch                       bool
	Effective, Permitted, Inheritable [2]uint32
	Ambient                           uint64
	ProcMatches                       bool
}
type manualReadOnlyWALMount struct {
	Observed                                 bool
	ID                                       uint64
	Matches                                  int
	Major, Minor                             uint32
	Root, Point, Options, SuperOptions, Type string
	Magic                                    int64
	ReadOnly                                 bool
}
type manualReadOnlyWALObject struct {
	Observed                         bool
	Identity                         manualReadOnlyWALIdentity
	UID                              uint32
	Mode                             uint32
	Links                            uint32
	Mount                            manualReadOnlyWALMount
	ReadAllowed                      bool
	AccessObserved                   bool
	WriteResult                      string
	AccessCall                       string
	AccessFlags, ReadMask, WriteMask uint32
}
type manualReadOnlyWALFacts struct {
	Credentials       manualReadOnlyWALCredentials
	Source            [4]manualReadOnlyWALObject
	Evidence          manualReadOnlyWALObject
	DistinctLocations bool
}

func manualReadOnlyWALMountValid(m manualReadOnlyWALMount, id manualReadOnlyWALIdentity, readonly bool) bool {
	if !m.Observed || m.ID == 0 || m.Matches != 1 || m.Major != id.Major || m.Minor != id.Minor || m.ReadOnly != readonly {
		return false
	}
	if !(m.Type == "ext4" && m.Magic == 0xef53 || m.Type == "xfs" && m.Magic == 0x58465342) {
		return false
	}
	ro, rw := false, false
	for _, option := range strings.Split(m.Options, ",") {
		ro = ro || option == "ro"
		rw = rw || option == "rw"
	}
	return ro != rw && ro == readonly && len(m.Root) > 0 && len(m.Point) > 0
}
func manualReadOnlyWALValidateProfile(f manualReadOnlyWALFacts, m manualReadOnlyWALManifest) error {
	c := f.Credentials
	if !c.Observed || !c.GroupsMatch || !c.ProcMatches || m.ReaderUID == 0 || !f.DistinctLocations {
		return errors.New("profile observation missing")
	}
	for i := 0; i < 4; i++ {
		if c.UID[i] != m.ReaderUID || c.GID[i] != m.ReaderGID {
			return errors.New("profile credentials")
		}
	}
	if c.Effective != [2]uint32{} || c.Permitted != [2]uint32{} || c.Inheritable != [2]uint32{} || c.Ambient != 0 {
		return errors.New("profile capabilities")
	}
	readonly := m.Profile == "readonly-mount-v1"
	if !readonly && m.Profile != "nonprivileged-denied-v1" {
		return errors.New("profile unknown")
	}
	for i, object := range f.Source {
		id := m.SourceIdentity
		if i > 0 {
			member := m.Family[i-1]
			id = manualReadOnlyWALIdentity{member.Major, member.Minor, member.Inode}
		}
		if !object.Observed || !object.AccessObserved || !object.ReadAllowed || object.AccessCall != "Faccessat2" || object.AccessFlags != 0x1200 || object.WriteMask != 2 || object.ReadMask != 4 && i > 0 || object.ReadMask != 5 && i == 0 || object.UID != m.ReaderUID || object.Identity != id || !manualReadOnlyWALMountValid(object.Mount, id, readonly) || object.Mount.ID != f.Source[0].Mount.ID {
			return errors.New("profile source observation")
		}
		if i == 0 && object.Mode&0170000 != 0040000 || i > 0 && (object.Mode&0170000 != 0100000 || object.Links != 1) {
			return errors.New("profile source type")
		}
		if object.WriteResult != "EACCES" && !(readonly && object.WriteResult == "EROFS") {
			return errors.New("profile effective write denial")
		}
	}
	e := f.Evidence
	if !e.Observed || !e.AccessObserved || !e.ReadAllowed || e.AccessCall != "Faccessat2" || e.AccessFlags != 0x1200 || e.ReadMask != 7 || e.WriteMask != 2 || e.WriteResult != "ALLOW" || e.UID != m.ReaderUID || e.Mode != 0040700 || !manualReadOnlyWALMountValid(e.Mount, e.Identity, false) || e.Identity == f.Source[0].Identity {
		return errors.New("profile evidence")
	}
	return nil
}

const (
	manualReadOnlyWALReserved  = "RESERVED"
	manualReadOnlyWALLive      = "LIVE"
	manualReadOnlyWALConsuming = "CONSUMING"
	manualReadOnlyWALEnded     = "ENDED_OK"
	manualReadOnlyWALUnknown   = "UNKNOWN/BLOCKED"
)

type manualReadOnlyWALOwnerState struct {
	Parent              int
	Kind, State, Action string
	HadOwner, HadError  bool
}
type manualReadOnlyWALLedger struct {
	Owners  []manualReadOnlyWALOwnerState
	Blocked bool
}

func manualReadOnlyWALReserve(l *manualReadOnlyWALLedger, parent int, kind string) (int, error) {
	if l.Blocked || len(l.Owners) >= 512 || parent < -1 || parent >= len(l.Owners) || (parent >= 0 && l.Owners[parent].State != manualReadOnlyWALLive) {
		return -1, errors.New("reserve blocked")
	}
	if kind != "fd" && kind != "db" && kind != "tx" && kind != "rows" && kind != "call" {
		return -1, errors.New("owner kind")
	}
	l.Owners = append(l.Owners, manualReadOnlyWALOwnerState{Parent: parent, Kind: kind, State: manualReadOnlyWALReserved})
	return len(l.Owners) - 1, nil
}
func manualReadOnlyWALAdopt(l *manualReadOnlyWALLedger, id int, present, failed, unknown bool) error {
	if id < 0 || id >= len(l.Owners) || l.Owners[id].State != manualReadOnlyWALReserved {
		l.Blocked = true
		return errors.New("unreserved adoption")
	}
	o := &l.Owners[id]
	o.HadOwner, o.HadError = present, failed
	if unknown || !present && !failed {
		o.State = manualReadOnlyWALUnknown
		l.Blocked = true
		return errors.New("acquisition ownership unknown")
	}
	if present {
		o.State = manualReadOnlyWALLive
	} else {
		o.State = manualReadOnlyWALEnded
	}
	return nil
}
func manualReadOnlyWALBeginEnd(l *manualReadOnlyWALLedger, id int, action string) error {
	if id < 0 || id >= len(l.Owners) {
		return errors.New("unknown owner")
	}
	o := &l.Owners[id]
	if o.State != manualReadOnlyWALLive {
		return errors.New("terminal action repeated/unknown")
	}
	for _, child := range l.Owners {
		if child.Parent == id && child.State != manualReadOnlyWALEnded {
			return errors.New("child retirement barrier")
		}
	}
	valid := o.Kind == "fd" && action == "close" || o.Kind == "db" && action == "close" || o.Kind == "rows" && (action == "close" || action == "next") || o.Kind == "tx" && (action == "commit" || action == "rollback") || o.Kind == "call" && action == "read"
	if !valid {
		return errors.New("terminal action kind")
	}
	o.State, o.Action = manualReadOnlyWALConsuming, action
	return nil
}
func manualReadOnlyWALFinishEnd(l *manualReadOnlyWALLedger, id int, failed bool) error {
	if id < 0 || id >= len(l.Owners) || l.Owners[id].State != manualReadOnlyWALConsuming || l.Owners[id].Action == "next" {
		l.Blocked = true
		return errors.New("terminal result without begin")
	}
	o := &l.Owners[id]
	o.HadError = o.HadError || failed
	if failed {
		o.State = manualReadOnlyWALUnknown
		l.Blocked = true
		return errors.New("terminal ownership unknown")
	}
	o.State = manualReadOnlyWALEnded
	return nil
}
func manualReadOnlyWALFinishNext(l *manualReadOnlyWALLedger, id int, next, failed bool) error {
	if id < 0 || id >= len(l.Owners) || l.Owners[id].State != manualReadOnlyWALConsuming || l.Owners[id].Action != "next" {
		l.Blocked = true
		return errors.New("Next result without begin")
	}
	o := &l.Owners[id]
	o.HadError = o.HadError || failed
	if failed {
		o.State = manualReadOnlyWALUnknown
		l.Blocked = true
		return errors.New("Next ownership unknown")
	}
	if next {
		o.State, o.Action = manualReadOnlyWALLive, ""
	} else {
		o.State = manualReadOnlyWALEnded
	}
	return nil
}
func manualReadOnlyWALRetired(l manualReadOnlyWALLedger) bool {
	if l.Blocked {
		return false
	}
	for _, o := range l.Owners {
		if o.State != manualReadOnlyWALEnded {
			return false
		}
	}
	return true
}

func manualReadOnlyWALMemoryManifest() string {
	return `{"version":1,"run_id":"11111111111111111111111111111111","profile":"nonprivileged-denied-v1","fixture_profile":"live-ro-wal-4096-v1","source_root":"/prepared/source","evidence_parent":"/prepared/evidence","reader_uid":1000,"reader_gid":1000,"source_tree":"1111111111111111111111111111111111111111","native_source_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","portable_source_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","guard_receipt_sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","binary_sha256":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd","source_identity":{"dev_major":8,"dev_minor":1,"inode":100},"family":[{"name":"readonly.db","size":4096,"sha256":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","dev_major":8,"dev_minor":1,"inode":101},{"name":"readonly.db-wal","size":12392,"sha256":"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff","dev_major":8,"dev_minor":1,"inode":102},{"name":"readonly.db-shm","size":32768,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","dev_major":8,"dev_minor":1,"inode":103}],"keeper":{"session":"prepared-1","generation":"22222222222222222222222222222222","contract":"already-held-live-dms-readmark-no-mutation-until-operator-release-v1"}}`
}
func TestReadOnlyWALManualManifestMemory(t *testing.T) {
	valid := manualReadOnlyWALMemoryManifest()
	if _, err := manualReadOnlyWALDecodeManifest([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	mutations := [][2]string{
		{`"version":1`, `"version":1,"version":1`}, {`"inode":100`, `"inode":100,"inode":100`},
		{`"name":"readonly.db"`, `"name":"readonly.db","name":"readonly.db"`}, {`"session":"prepared-1"`, `"session":"prepared-1","session":"prepared-1"`},
		{`"version":1`, `"Version":1`}, {`"version":1`, `"sql":"SELECT 1","version":1`}, {`"version":1,`, ``},
		{`"version":1`, `"version":null`}, {`"version":1`, `"version":"1"`}, {`"version":1`, `"version":1.0`},
		{`"reader_uid":1000`, `"reader_uid":4294967296`}, {`"inode":100`, `"inode":18446744073709551616`},
		{`"source_root":"/prepared/source"`, `"source_root":"/prepared/../source"`},
		{`"source_root":"/prepared/source"`, `"source_root":"/"`}, {`"source_root":"/prepared/source"`, `"source_root":"relative"`},
		{`"evidence_parent":"/prepared/evidence"`, `"evidence_parent":"/prepared/source/child"`},
		{`"profile":"nonprivileged-denied-v1"`, `"profile":"readonly"`}, {`"reader_uid":1000`, `"reader_uid":0`},
		{`"size":4096`, `"size":8192`}, {`"size":12392`, `"size":1048577`}, {`"size":32768`, `"size":0`},
		{`"name":"readonly.db"`, `"name":"../readonly.db"`}, {`"inode":102`, `"inode":101`},
		{`"generation":"22222222222222222222222222222222"`, `"generation":"ABC"`},
		{`already-held-live-dms-readmark-no-mutation-until-operator-release-v1`, `start-a-keeper`},
		{`"version":1`, `"command":"run","version":1`}, {`"version":1`, `"env":{},"version":1`},
	}
	for i, mutation := range mutations {
		raw := strings.Replace(valid, mutation[0], mutation[1], 1)
		if raw == valid {
			t.Fatalf("unused mutation %d", i)
		}
		if _, err := manualReadOnlyWALDecodeManifest([]byte(raw)); err == nil {
			t.Fatalf("admitted mutation %d", i)
		}
	}
	for _, raw := range []string{valid + " {}", valid + string([]byte{255}), strings.Repeat(" ", 32769), `{"a":` + strings.Repeat("[", 9) + `0` + strings.Repeat("]", 9) + `}`, `{"a":[` + strings.Repeat(`0,`, 512) + `0]}`, `{"a":"` + strings.Repeat("x", 1025) + `"}`} {
		if _, err := manualReadOnlyWALDecodeManifest([]byte(raw)); err == nil {
			t.Fatal("admitted JSON boundary")
		}
	}
	if _, err := manualReadOnlyWALDecodeManifest([]byte(valid + strings.Repeat(" ", 32768-len(valid)))); err != nil {
		t.Fatal("exact byte boundary", err)
	}
}
func manualReadOnlyWALMemoryFacts(m manualReadOnlyWALManifest) manualReadOnlyWALFacts {
	f := manualReadOnlyWALFacts{DistinctLocations: true}
	f.Credentials = manualReadOnlyWALCredentials{Observed: true, UID: [4]uint32{1000, 1000, 1000, 1000}, GID: [4]uint32{1000, 1000, 1000, 1000}, GroupsMatch: true, ProcMatches: true}
	mount := manualReadOnlyWALMount{Observed: true, ID: 7, Matches: 1, Major: 8, Minor: 1, Root: "/", Point: "/prepared", Options: "rw,relatime", Type: "ext4", Magic: 0xef53}
	for i := range f.Source {
		id := m.SourceIdentity
		mode := uint32(0040700)
		if i > 0 {
			member := m.Family[i-1]
			id = manualReadOnlyWALIdentity{member.Major, member.Minor, member.Inode}
			mode = 0100400
		}
		f.Source[i] = manualReadOnlyWALObject{Observed: true, Identity: id, UID: 1000, Mode: mode, Links: 1, Mount: mount, ReadAllowed: true, AccessObserved: true, WriteResult: "EACCES", AccessCall: "Faccessat2", AccessFlags: 0x1200, ReadMask: 4, WriteMask: 2}
	}
	f.Source[0].ReadMask = 5
	f.Evidence = manualReadOnlyWALObject{Observed: true, Identity: manualReadOnlyWALIdentity{8, 1, 200}, UID: 1000, Mode: 0040700, Mount: mount, ReadAllowed: true, AccessObserved: true, WriteResult: "ALLOW", AccessCall: "Faccessat2", AccessFlags: 0x1200, ReadMask: 7, WriteMask: 2}
	return f
}
func TestReadOnlyWALManualProfileMemory(t *testing.T) {
	m, err := manualReadOnlyWALDecodeManifest([]byte(manualReadOnlyWALMemoryManifest()))
	if err != nil {
		t.Fatal(err)
	}
	valid := manualReadOnlyWALMemoryFacts(m)
	if err := manualReadOnlyWALValidateProfile(valid, m); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 27; n++ {
		f := valid
		switch n {
		case 0:
			f.Credentials.Observed = false
		case 1:
			f.Credentials.UID[0] = 0
		case 2:
			f.Credentials.UID[3]++
		case 3:
			f.Credentials.GID[3]++
		case 4:
			f.Credentials.GroupsMatch = false
		case 5:
			f.Credentials.Effective[0] = 1
		case 6:
			f.Credentials.Effective[1] = 1
		case 7:
			f.Credentials.Permitted[1] = 1
		case 8:
			f.Credentials.Inheritable[1] = 1
		case 9:
			f.Credentials.Ambient = 1
		case 10:
			f.Credentials.ProcMatches = false
		case 11:
			f.Source[0].Observed = false
		case 12:
			f.Source[1].AccessObserved = false
		case 13:
			f.Source[2].WriteResult = "ENOSYS"
		case 14:
			f.Source[3].WriteResult = "ALLOW"
		case 15:
			f.Source[0].Mount.ReadOnly = true
		case 16:
			f.Source[1].Mount.ID++
		case 17:
			f.Source[2].Mount.Matches = 2
		case 18:
			f.Source[3].Mount.Type = "overlay"
		case 19:
			f.Evidence.WriteResult = "EACCES"
		case 20:
			f.Evidence.Mode = 0040755
		case 21:
			f.DistinctLocations = false
		case 22:
			f.Source[1].Links = 2
		case 23:
			f.Source[1].Mount.Observed = false
		case 24:
			f.Source[1].Mount.Major++
		case 25:
			f.Source[1].Mount.Options = "ro,rw"
		case 26:
			f.Source[1].ReadAllowed = false
		}
		if err := manualReadOnlyWALValidateProfile(f, m); err == nil {
			t.Fatalf("admitted profile mutation %d", n)
		}
	}
	for objectIndex := 0; objectIndex < 5; objectIndex++ {
		for missing := 0; missing < 8; missing++ {
			f := valid
			object := &f.Evidence
			if objectIndex < 4 {
				object = &f.Source[objectIndex]
			}
			switch missing {
			case 0:
				object.Observed = false
			case 1:
				object.AccessObserved = false
			case 2:
				object.ReadAllowed = false
			case 3:
				object.Mount.Observed = false
			case 4:
				object.Mount.Matches = 0
			case 5:
				object.Mount.ID = 0
			case 6:
				object.AccessFlags = 0
			case 7:
				object.AccessCall = "Faccessat"
			}
			if err := manualReadOnlyWALValidateProfile(f, m); err == nil {
				t.Fatal("missing observed fact admitted", objectIndex, missing)
			}
		}
	}
	for word := 0; word < 2; word++ {
		for set := 0; set < 3; set++ {
			f := valid
			switch set {
			case 0:
				f.Credentials.Effective[word] = 1
			case 1:
				f.Credentials.Permitted[word] = 1
			case 2:
				f.Credentials.Inheritable[word] = 1
			}
			if err := manualReadOnlyWALValidateProfile(f, m); err == nil {
				t.Fatal("nonzero capability word admitted")
			}
		}
	}
	m.Profile = "readonly-mount-v1"
	if err := manualReadOnlyWALValidateProfile(valid, m); err == nil {
		t.Fatal("B admitted rw")
	}
	for i := range valid.Source {
		valid.Source[i].Mount.ReadOnly = true
		valid.Source[i].Mount.Options = "ro"
		valid.Source[i].WriteResult = "EROFS"
	}
	if err := manualReadOnlyWALValidateProfile(valid, m); err != nil {
		t.Fatal(err)
	}
}
func TestReadOnlyWALManualHandshakeMemory(t *testing.T) {
	m, err := manualReadOnlyWALDecodeManifest([]byte(manualReadOnlyWALMemoryManifest()))
	if err != nil {
		t.Fatal(err)
	}
	s := manualReadOnlyWALPhase{RunID: m.RunID, Session: m.Keeper.Session, Generation: m.Keeper.Generation}
	for i, phase := range []string{"keep-false-before", "keep-false-after", "keep-true-before", "keep-true-after"} {
		s.Challenge = strings.Repeat(strconv.Itoa(i+3), 32)
		r := manualReadOnlyWALReceipt{1, s.RunID, phase, s.Challenge, s.Session, s.Generation, "still-held-quiescent"}
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := manualReadOnlyWALDecodeReceipt(raw)
		if err != nil || decoded != r {
			t.Fatal("receipt roundtrip", err)
		}
		for n := 0; n < 6; n++ {
			bad := r
			switch n {
			case 0:
				bad.RunID = "bad"
			case 1:
				bad.Session = "bad"
			case 2:
				bad.Generation = "bad"
			case 3:
				bad.Challenge = "bad"
			case 4:
				bad.Phase = "out-of-order"
			case 5:
				bad.Status = "released"
			}
			if _, err := manualReadOnlyWALAdvancePhase(s, bad); err == nil {
				t.Fatal("admitted handshake mutation")
			}
		}
		if i > 0 {
			replayedState, replayed := s, r
			replayedState.Challenge, replayed.Challenge = s.Used[0], s.Used[0]
			if _, err := manualReadOnlyWALAdvancePhase(replayedState, replayed); err == nil {
				t.Fatal("prior nonce reused for new phase")
			}
		}
		next, err := manualReadOnlyWALAdvancePhase(s, r)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := manualReadOnlyWALAdvancePhase(next, r); err == nil {
			t.Fatal("replay")
		}
		s = next
	}
	if s.Next != 4 {
		t.Fatal("incomplete")
	}
	if _, err := manualReadOnlyWALDecodeReceipt([]byte(`{"version":1,"Version":1}`)); err == nil {
		t.Fatal("receipt exact keys")
	}
	valid := `{"version":1,"run_id":"11111111111111111111111111111111","phase":"keep-false-before","challenge":"33333333333333333333333333333333","session":"prepared-1","generation":"22222222222222222222222222222222","status":"still-held-quiescent"}`
	for _, raw := range []string{strings.Replace(valid, `"version":1`, `"version":1,"version":1`, 1), strings.Replace(valid, `"status":"still-held-quiescent"`, `"status":null`, 1), valid + " {}", valid + string([]byte{255}), strings.Repeat(" ", 2049)} {
		if _, err := manualReadOnlyWALDecodeReceipt([]byte(raw)); err == nil {
			t.Fatal("malformed receipt admitted")
		}
	}

}
func TestReadOnlyWALManualRetirementMemory(t *testing.T) {
	for _, terminal := range []string{"close", "commit", "rollback", "read", "next"} {
		for _, failure := range []bool{false, true} {
			l := manualReadOnlyWALLedger{}
			db, err := manualReadOnlyWALReserve(&l, -1, "db")
			if err != nil {
				t.Fatal(err)
			}
			if err := manualReadOnlyWALAdopt(&l, db, true, false, false); err != nil {
				t.Fatal(err)
			}
			kind := "rows"
			if terminal == "commit" || terminal == "rollback" {
				kind = "tx"
			}
			if terminal == "read" {
				kind = "call"
			}
			id, err := manualReadOnlyWALReserve(&l, db, kind)
			if err != nil {
				t.Fatal(err)
			}
			if err := manualReadOnlyWALAdopt(&l, id, true, false, false); err != nil {
				t.Fatal(err)
			}
			if err := manualReadOnlyWALBeginEnd(&l, db, "close"); err == nil {
				t.Fatal("parent over live child")
			}
			if terminal == "next" {
				if err := manualReadOnlyWALBeginEnd(&l, id, "next"); err != nil {
					t.Fatal(err)
				}
				if err := manualReadOnlyWALFinishNext(&l, id, true, false); err != nil || l.Owners[id].State != manualReadOnlyWALLive {
					t.Fatal("Next true", err)
				}
			}
			if err := manualReadOnlyWALBeginEnd(&l, id, terminal); err != nil {
				t.Fatal(err)
			}
			if terminal == "next" {
				err = manualReadOnlyWALFinishNext(&l, id, false, failure)
			} else {
				err = manualReadOnlyWALFinishEnd(&l, id, failure)
			}
			if (err != nil) != failure {
				t.Fatal("terminal classification")
			}
			if err := manualReadOnlyWALBeginEnd(&l, id, "close"); err == nil {
				t.Fatal("double/EOF close")
			}
			if err := manualReadOnlyWALBeginEnd(&l, id, "rollback"); err == nil {
				t.Fatal("rollback after terminal")
			}
			if failure {
				if _, err := manualReadOnlyWALReserve(&l, -1, "fd"); err == nil {
					t.Fatal("successor after uncertainty")
				}
				if err := manualReadOnlyWALBeginEnd(&l, db, "close"); err == nil {
					t.Fatal("parent over unknown child")
				}
			} else {
				if err := manualReadOnlyWALBeginEnd(&l, db, "close"); err != nil {
					t.Fatal(err)
				}
				if err := manualReadOnlyWALFinishEnd(&l, db, false); err != nil || !manualReadOnlyWALRetired(l) {
					t.Fatal("retirement", err)
				}
			}
		}
	}
	l := manualReadOnlyWALLedger{}
	if err := manualReadOnlyWALAdopt(&l, 0, true, true, true); err == nil || !l.Blocked {
		t.Fatal("adopt before reservation")
	}
	l = manualReadOnlyWALLedger{}
	id, err := manualReadOnlyWALReserve(&l, -1, "db")
	if err != nil {
		t.Fatal(err)
	}
	if err := manualReadOnlyWALAdopt(&l, id, true, true, true); err == nil || !l.Owners[id].HadOwner || !l.Owners[id].HadError {
		t.Fatal("owner and original error classification")
	}
	l = manualReadOnlyWALLedger{}
	id, err = manualReadOnlyWALReserve(&l, -1, "db")
	if err != nil {
		t.Fatal(err)
	}
	if err := manualReadOnlyWALAdopt(&l, id, false, true, true); err == nil || !l.Blocked || !l.Owners[id].HadError || l.Owners[id].HadOwner {
		t.Fatal("nil public owner must retain failed-open classification")
	}
	l = manualReadOnlyWALLedger{}
	id, err = manualReadOnlyWALReserve(&l, -1, "rows")
	if err != nil {
		t.Fatal(err)
	}
	if err := manualReadOnlyWALAdopt(&l, id, true, false, false); err != nil {
		t.Fatal(err)
	}
	if err := manualReadOnlyWALBeginEnd(&l, id, "next"); err != nil {
		t.Fatal(err)
	}
	if err := manualReadOnlyWALFinishNext(&l, id, true, false); err != nil {
		t.Fatal(err)
	}
	if err := manualReadOnlyWALBeginEnd(&l, id, "close"); err != nil {
		t.Fatal("unexpected extra row still live for one close", err)
	}
	if err := manualReadOnlyWALFinishEnd(&l, id, false); err != nil {
		t.Fatal(err)
	}
	if err := manualReadOnlyWALBeginEnd(&l, id, "close"); err == nil {
		t.Fatal("extra-row cleanup repeated")
	}
	l = manualReadOnlyWALLedger{}
	id, err = manualReadOnlyWALReserve(&l, -1, "fd")
	if err != nil {
		t.Fatal(err)
	}
	if err := manualReadOnlyWALAdopt(&l, id, true, false, false); err != nil {
		t.Fatal(err)
	}
	if err := manualReadOnlyWALBeginEnd(&l, id, "close"); err != nil {
		t.Fatal(err)
	}
	if err := manualReadOnlyWALFinishEnd(&l, id, true); err == nil || manualReadOnlyWALRetired(l) {
		t.Fatal("evidence close error passed")
	}
}
