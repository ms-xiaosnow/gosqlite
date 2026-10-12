// Package formatupgrade is a descriptor-free, bounded receipt contract.
// It does not authorize execution and cannot open, close, restore or upgrade anything.
package formatupgrade

import (
	"encoding/hex"
	"fmt"
	"path"
	"reflect"
	"strings"
)

const Profile = "encrypted-layout1-to2-fresh-snapshot-restore-v1"
const Entry = "TestManualEncryptedSnapshotToWALUpgrade"
const MaxEntries = 256
const MaxBytes int64 = 64 << 20
const MaxOwners = 4096

func Hash(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && s != strings.Repeat("0", 64)
}

type File struct {
	Name          string
	Size          int64
	SHA256        string
	Device, Inode uint64
	Mode          uint32
}
type Manifest []File

type Directory struct {
	Path          string
	Device, Inode uint64
	UID, Mode     uint32
}

func (d Directory) Validate(private bool) error {
	if !path.IsAbs(d.Path) || path.Clean(d.Path) != d.Path || d.Inode == 0 || d.Mode == 0 || (private && d.Mode != 0700) {
		return fmt.Errorf("directory identity")
	}
	return nil
}
func (d Directory) ChildOf(parent Directory, name string) error {
	if err := d.Validate(true); err != nil {
		return err
	}
	if d.Path != path.Join(parent.Path, name) || d.Device != parent.Device || d.UID != parent.UID || d.Inode == parent.Inode {
		return fmt.Errorf("owned direct child identity")
	}
	return nil
}

type Snapshot struct {
	Directory Directory
	Phase     string
	Files     Manifest
}

func (s Snapshot) Validate() error {
	if err := s.Directory.Validate(true); err != nil {
		return err
	}
	if s.Phase == "" {
		return fmt.Errorf("manifest phase")
	}
	if err := s.Files.Validate(); err != nil {
		return err
	}
	for _, f := range s.Files {
		if f.Device != s.Directory.Device {
			return fmt.Errorf("manifest file/directory device binding")
		}
	}
	return nil
}

func (m Manifest) Validate() error {
	if len(m) == 0 || len(m) > MaxEntries {
		return fmt.Errorf("manifest cardinality")
	}
	seen := map[string]bool{}
	var total int64
	for _, f := range m {
		if f.Name == "" || f.Name == "." || f.Name == ".." || strings.ContainsAny(f.Name, "/\\\x00") || seen[f.Name] {
			return fmt.Errorf("manifest name")
		}
		seen[f.Name] = true
		if f.Size < 0 || f.Size > MaxBytes || total > MaxBytes-f.Size || !Hash(f.SHA256) || f.Inode == 0 || f.Mode != 0600 {
			return fmt.Errorf("manifest extent/identity")
		}
		total += f.Size
	}
	return nil
}

// Preserve permits only the explicitly named mutable root to change. Its exact
// old ciphertext must appear as a retained root, while every other old inode,
// extent, mode and hash remains unchanged. It never labels the main immutable.
func Preserve(before, after Snapshot, mutableRoot string) error {
	if before.Directory != after.Directory {
		return fmt.Errorf("manifest directory changed")
	}
	if err := before.Validate(); err != nil {
		return err
	}
	if err := after.Validate(); err != nil {
		return err
	}
	next := map[string]File{}
	for _, f := range after.Files {
		next[f.Name] = f
	}
	rootSeen := false
	for _, f := range before.Files {
		g, ok := next[f.Name]
		if !ok {
			return fmt.Errorf("lost original %s", f.Name)
		}
		if f.Name != mutableRoot {
			if f != g {
				return fmt.Errorf("changed original %s", f.Name)
			}
			continue
		}
		rootSeen = true
		if f.SHA256 == g.SHA256 || f.Inode == g.Inode {
			return fmt.Errorf("upgrade root was not replaced")
		}
		retained := false
		for _, candidate := range after.Files {
			if strings.HasPrefix(candidate.Name, mutableRoot+".gse-") && strings.HasSuffix(candidate.Name, ".retained") && Hash(strings.TrimSuffix(strings.TrimPrefix(candidate.Name, mutableRoot+".gse-"), ".retained")) && candidate.Size == f.Size && candidate.SHA256 == f.SHA256 {
				retained = true
			}
		}
		if !retained {
			return fmt.Errorf("old root ciphertext not retained")
		}
	}
	if !rootSeen {
		return fmt.Errorf("missing mutable root")
	}
	return nil
}

func Equal(before, after Snapshot) error {
	if err := before.Validate(); err != nil {
		return err
	}
	if err := after.Validate(); err != nil {
		return err
	}
	if before.Directory != after.Directory || !reflect.DeepEqual(before.Files, after.Files) {
		return fmt.Errorf("manifest changed")
	}
	return nil
}

type Owner struct {
	ID                                      int
	Page                                    uint32
	Kind, Label                             string
	Parent                                  int
	Present                                 bool
	AcquireError                            string
	Attempts                                int
	Terminal, RetireError, Blocked, Refusal string
}

// Retire records a single known retirement or the single EOF-driven Rows close.
// A nonnil diagnostic stays unknown, even if the underlying API may have consumed
// its resource. No subsequent call is licensed by this model.
func (o *Owner) Retire(route, diagnostic string) error {
	if !o.Present || o.Attempts != 0 || o.Terminal != "live" || (route != "close" && route != "eof") {
		return fmt.Errorf("invalid/duplicate retirement")
	}
	o.Attempts = 1
	o.RetireError = diagnostic
	o.Terminal = "retired"
	if diagnostic != "" {
		o.Terminal = "unknown"
	}
	return nil
}

func Owners(owners []Owner) error {
	if len(owners) == 0 || len(owners) > MaxOwners {
		return fmt.Errorf("owner cardinality")
	}
	for i, o := range owners {
		if o.ID != i+1 || o.Label == "" || (o.Kind != "db" && o.Kind != "rows" && o.Kind != "file") || o.Parent < 0 || o.Parent >= o.ID {
			return fmt.Errorf("owner identity")
		}
		if o.Present {
			if o.Blocked != "" || o.Refusal != "" || o.AcquireError != "" || o.Attempts != 1 || o.Terminal != "retired" || o.RetireError != "" {
				return fmt.Errorf("owner retirement %d", o.ID)
			}
		} else if o.Kind != "db" || o.Label != "old-mode-rejection" || o.Parent != 0 || o.Refusal != "unsupported-old-mode" || o.Attempts != 0 || o.Terminal != "absent" || o.AcquireError == "" {
			return fmt.Errorf("invalid absent owner")
		}
		if o.Kind == "rows" && (o.Parent == 0 || owners[o.Parent-1].Kind != "db" || owners[o.Parent-1].Page != o.Page) {
			return fmt.Errorf("Rows parent identity")
		}
		if o.Kind != "rows" && o.Parent != 0 {
			return fmt.Errorf("unexpected owner parent")
		}
		if o.Kind == "file" && !strings.HasPrefix(o.Label, "file:") {
			return fmt.Errorf("file acquisition label")
		}
	}
	return nil
}

// CanRetire is the actual native orchestration precondition. An unbound public
// pending token conservatively blocks every live DB and Rows owner: closing a
// parent could drive an unreturned internal temporary's cleanup group again.
func CanRetire(o Owner, owners []Owner, internalPending bool) error {
	if !o.Present || o.Attempts != 0 || o.Terminal != "live" || o.Blocked != "" {
		return fmt.Errorf("not eligible for first close")
	}
	if internalPending && (o.Kind == "db" || o.Kind == "rows") {
		return fmt.Errorf("unbound internal encrypted pending ownership")
	}
	for _, child := range owners {
		if child.Parent == o.ID && child.Present && child.Terminal != "retired" {
			return fmt.Errorf("unretired child blocks parent")
		}
	}
	return nil
}

type Acquisition struct {
	ID          int
	Page        uint32
	Kind, Label string
	Parent      int
	Present     bool
	Error       string
}

func AcquisitionOf(o Owner) Acquisition {
	return Acquisition{ID: o.ID, Page: o.Page, Kind: o.Kind, Label: o.Label, Parent: o.Parent, Present: o.Present, Error: o.AcquireError}
}
func OwnerInventory(owners []Owner, inventory []Acquisition, cases []Case) error {
	if err := Owners(owners); err != nil {
		return err
	}
	if len(owners) != len(inventory) {
		return fmt.Errorf("truncated acquisition/retirement ledger")
	}
	for i, o := range owners {
		if AcquisitionOf(o) != inventory[i] {
			return fmt.Errorf("acquisition identity changed")
		}
	}
	for _, c := range cases {
		ids := []int{}
		byLabel := map[string]Owner{}
		for _, o := range owners {
			if o.Page != c.PageSize || o.Kind == "file" {
				continue
			}
			ids = append(ids, o.ID)
			if _, duplicate := byLabel[o.Label]; duplicate {
				return fmt.Errorf("duplicate case owner role")
			}
			byLabel[o.Label] = o
		}
		if !reflect.DeepEqual(ids, c.OwnerIDs) || len(ids) != 24 {
			return fmt.Errorf("incomplete per-case DB/Rows inventory")
		}
		for _, role := range []string{"source", "old-mode-rejection", "wal-reopen", "restored-snapshot"} {
			o, ok := byLabel[role]
			if !ok || o.Kind != "db" || (role == "old-mode-rejection") == o.Present {
				return fmt.Errorf("case DB/refusal role")
			}
		}
		for _, phase := range []string{"snapshot", "after-unique-refusal", "wal-reopened", "restored-snapshot"} {
			parent := byLabel["source"].ID
			if phase == "wal-reopened" {
				parent = byLabel["wal-reopen"].ID
			}
			if phase == "restored-snapshot" {
				parent = byLabel["restored-snapshot"].ID
			}
			for _, query := range []string{"rows", "unique", "grams", "scan", "vector"} {
				o, ok := byLabel["rows:"+phase+"-"+query]
				if !ok || o.Kind != "rows" || !o.Present || o.Parent != parent {
					return fmt.Errorf("case Rows role/parent")
				}
			}
		}
	}
	for _, o := range owners {
		if o.Page != 0 && o.Page != 512 && o.Page != 4096 && o.Page != 65536 {
			return fmt.Errorf("unknown owner case")
		}
		if o.Page == 0 && o.Kind != "file" {
			return fmt.Errorf("DB/Rows outside case")
		}
	}
	return nil
}

type Provider struct {
	Page               uint32
	Label              string
	Acquires, Releases int64
	Duplicate          bool
}

type Head struct {
	Mode                          uint8
	Layout                        uint16
	PageSize, PageCount, Segments uint32
	DBID                          string
	Sequence                      uint64
}

func (h Head) valid(page uint32, layout uint16, mode uint8) bool {
	b, err := hex.DecodeString(h.DBID)
	return err == nil && len(b) == 16 && h.DBID != strings.Repeat("0", 32) && h.PageSize == page && h.PageCount > 1 && h.Layout == layout && h.Mode == mode && h.Segments == 0 && h.Sequence > 0
}

type Case struct {
	PageSize                                                  uint32
	Root, SourceDirectory, ArchiveDirectory, RestoreDirectory Directory
	OwnerIDs                                                  []int
	Before, Archive, Upgraded, Reopened, Restored             Head
	SourceBefore, SourceAfter, SourceFinal                    Snapshot
	ArchiveBefore, ArchiveAfter, RestoredFiles                Snapshot
	OldModeRejected, OldModeNil, OldModeUnchanged             bool
	Checks                                                    []string
}

func RequiredChecks() []string {
	return []string{"snapshot-ledger-index", "unique-rejected-unchanged", "archive-created", "upgrade-identity", "originals-retained", "old-mode-rejected-unchanged", "wal-ledger-index", "fresh-restore", "restored-ledger-index", "source-and-archive-unchanged"}
}

func (c Case) Validate() error {
	p := c.PageSize
	if err := c.Root.Validate(true); err != nil {
		return err
	}
	for name, d := range map[string]Directory{"source": c.SourceDirectory, "archive": c.ArchiveDirectory, "restored": c.RestoreDirectory} {
		if err := d.ChildOf(c.Root, name); err != nil {
			return err
		}
	}
	if c.SourceDirectory.Inode == c.ArchiveDirectory.Inode || c.SourceDirectory.Inode == c.RestoreDirectory.Inode || c.ArchiveDirectory.Inode == c.RestoreDirectory.Inode {
		return fmt.Errorf("case family alias")
	}
	for _, binding := range []struct {
		snapshot    Snapshot
		directory   Directory
		phase, main string
	}{
		{c.SourceBefore, c.SourceDirectory, "pre-upgrade-source", "source.gse"}, {c.SourceAfter, c.SourceDirectory, "post-upgrade-source", "source.gse"}, {c.SourceFinal, c.SourceDirectory, "final-source", "source.gse"},
		{c.ArchiveBefore, c.ArchiveDirectory, "pre-upgrade-archive", "before-upgrade.archive"}, {c.ArchiveAfter, c.ArchiveDirectory, "final-archive", "before-upgrade.archive"}, {c.RestoredFiles, c.RestoreDirectory, "restored-snapshot", "restored.gse"},
	} {
		if binding.snapshot.Directory != binding.directory || binding.snapshot.Phase != binding.phase {
			return fmt.Errorf("manifest path/phase binding")
		}
		if err := binding.snapshot.Validate(); err != nil {
			return err
		}
		found := false
		for _, f := range binding.snapshot.Files {
			if f.Name == binding.main {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("manifest required main missing")
		}
	}
	if p != 512 && p != 4096 && p != 65536 {
		return fmt.Errorf("page size")
	}
	if !c.Before.valid(p, 1, 0) || !c.Archive.valid(p, 0, 0) || !c.Upgraded.valid(p, 2, 1) || !c.Reopened.valid(p, 2, 1) || !c.Restored.valid(p, 1, 0) {
		return fmt.Errorf("head geometry/layout")
	}
	if c.Upgraded.DBID != c.Before.DBID || c.Before.Sequence == ^uint64(0) || c.Upgraded.Sequence != c.Before.Sequence+1 || c.Reopened != c.Upgraded {
		return fmt.Errorf("upgrade identity/sequence")
	}
	if c.Archive.DBID == c.Before.DBID || c.Restored.DBID == c.Before.DBID || c.Restored.DBID == c.Archive.DBID {
		return fmt.Errorf("fresh archive/restore identity")
	}
	for _, h := range []Head{c.Archive, c.Upgraded, c.Reopened, c.Restored} {
		if h.PageCount != c.Before.PageCount {
			return fmt.Errorf("page count conservation")
		}
	}
	if !c.OldModeRejected || !c.OldModeNil || !c.OldModeUnchanged {
		return fmt.Errorf("old mode not explicitly refused")
	}
	if !reflect.DeepEqual(c.Checks, RequiredChecks()) {
		return fmt.Errorf("missing/duplicate/out-of-order assertions")
	}
	if err := Preserve(c.SourceBefore, c.SourceAfter, "source.gse"); err != nil {
		return err
	}
	if err := Equal(c.SourceAfter, c.SourceFinal); err != nil {
		return err
	}
	return Equal(c.ArchiveBefore, c.ArchiveAfter)
}

type Receipt struct {
	Profile, Approval, SourceSHA256, BinarySHA256, Toolchain, Platform, RunDirectory string
	SourceRoot, KnownRoot, RunRoot                                                   Directory
	Status                                                                           string
	Cases                                                                            []Case
	Owners                                                                           []Owner
	Acquisitions                                                                     []Acquisition
	Providers                                                                        []Provider
	InternalPendingObserved                                                          bool
	Pending                                                                          int
	ProviderAcquires, ProviderReleases                                               int64
	ProviderDuplicateRelease                                                         bool
}

func (r Receipt) Validate() error {
	if r.Profile != Profile || r.Approval == "" || !Hash(r.SourceSHA256) || !Hash(r.BinarySHA256) || r.Toolchain != "go1.27.2" || (r.Platform != "linux/amd64" && r.Platform != "linux/arm64") || r.RunDirectory == "" || r.Status != "complete" {
		return fmt.Errorf("run identity/status")
	}
	if err := r.SourceRoot.Validate(false); err != nil {
		return err
	}
	if err := r.KnownRoot.Validate(true); err != nil {
		return err
	}
	if r.RunDirectory != r.RunRoot.Path || !strings.HasPrefix(path.Base(r.RunRoot.Path), "format-upgrade-") {
		return fmt.Errorf("run directory binding")
	}
	if err := r.RunRoot.ChildOf(r.KnownRoot, path.Base(r.RunRoot.Path)); err != nil {
		return err
	}
	if r.SourceRoot.Path == r.KnownRoot.Path || strings.HasPrefix(r.SourceRoot.Path, r.RunRoot.Path+"/") || strings.HasPrefix(r.RunRoot.Path, r.SourceRoot.Path+"/") {
		return fmt.Errorf("source/fixture directory separation")
	}
	if len(r.Cases) != 3 {
		return fmt.Errorf("missing/duplicate cases")
	}
	seenDirectories := map[string]bool{}
	allDirectories := []Directory{r.SourceRoot, r.KnownRoot, r.RunRoot}
	for _, c := range r.Cases {
		allDirectories = append(allDirectories, c.Root, c.SourceDirectory, c.ArchiveDirectory, c.RestoreDirectory)
	}
	for _, d := range allDirectories {
		identity := fmt.Sprintf("%d/%d", d.Device, d.Inode)
		if seenDirectories[identity] {
			return fmt.Errorf("directory identity alias")
		}
		seenDirectories[identity] = true
	}
	for i, page := range []uint32{512, 4096, 65536} {
		if r.Cases[i].PageSize != page {
			return fmt.Errorf("case order")
		}
		if err := r.Cases[i].Root.ChildOf(r.RunRoot, fmt.Sprintf("page%d", page)); err != nil {
			return err
		}
		if err := r.Cases[i].Validate(); err != nil {
			return err
		}
	}
	if r.InternalPendingObserved || r.Pending != 0 || r.ProviderAcquires <= 0 || r.ProviderAcquires != r.ProviderReleases || r.ProviderDuplicateRelease {
		return fmt.Errorf("pending provider/native ownership")
	}
	if len(r.Providers) != 9 {
		return fmt.Errorf("provider inventory")
	}
	seen := map[string]bool{}
	var acquires, releases int64
	for _, p := range r.Providers {
		if (p.Page != 512 && p.Page != 4096 && p.Page != 65536) || (p.Label != "fixture-source" && p.Label != "fixture-archive" && p.Label != "fixture-restored") || p.Acquires <= 0 || p.Acquires > 4096 || p.Releases != p.Acquires || p.Duplicate {
			return fmt.Errorf("provider case/accounting")
		}
		key := fmt.Sprintf("%d/%s", p.Page, p.Label)
		if seen[key] {
			return fmt.Errorf("duplicate provider role")
		}
		seen[key] = true
		acquires += p.Acquires
		releases += p.Releases
	}
	if acquires != r.ProviderAcquires || releases != r.ProviderReleases {
		return fmt.Errorf("provider totals")
	}
	return OwnerInventory(r.Owners, r.Acquisitions, r.Cases)
}
