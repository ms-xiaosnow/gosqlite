package formatupgrade

import (
	"reflect"
	"strings"
	"testing"
)

func memoryReceipt() Receipt {
	sourceRoot := Directory{Path: "/memory/source", Device: 1, Inode: 1, UID: 1000, Mode: 0755}
	knownRoot := Directory{Path: "/memory/root", Device: 1, Inode: 2, UID: 1000, Mode: 0700}
	runRoot := Directory{Path: "/memory/root/format-upgrade-fixture", Device: 1, Inode: 3, UID: 1000, Mode: 0700}
	r := Receipt{Profile: Profile, Approval: "memory-approval", SourceSHA256: strings.Repeat("a", 64), BinarySHA256: strings.Repeat("b", 64), Toolchain: "go1.27.2", Platform: "linux/amd64", RunDirectory: runRoot.Path, SourceRoot: sourceRoot, KnownRoot: knownRoot, RunRoot: runRoot, Status: "complete", ProviderAcquires: 9, ProviderReleases: 9}
	add := func(page uint32, kind, label string, parent int, present bool) int {
		o := Owner{ID: len(r.Owners) + 1, Page: page, Kind: kind, Label: label, Parent: parent, Present: present, Attempts: 1, Terminal: "retired"}
		if !present {
			o.Attempts = 0
			o.Terminal = "absent"
			o.AcquireError = "unsupported fixture mode"
			o.Refusal = "unsupported-old-mode"
		}
		r.Owners = append(r.Owners, o)
		r.Acquisitions = append(r.Acquisitions, AcquisitionOf(o))
		return o.ID
	}
	add(0, "file", "file:memory-source", 0, true)
	for index, page := range []uint32{512, 4096, 65536} {
		caseRoot := Directory{Path: runRoot.Path + "/" + []string{"page512", "page4096", "page65536"}[index], Device: 1, Inode: uint64(10 + index*10), UID: 1000, Mode: 0700}
		sourceDir := caseRoot
		sourceDir.Path += "/source"
		sourceDir.Inode++
		archiveDir := caseRoot
		archiveDir.Path += "/archive"
		archiveDir.Inode += 2
		restoredDir := caseRoot
		restoredDir.Path += "/restored"
		restoredDir.Inode += 3
		old := Head{Layout: 1, PageSize: page, PageCount: 8, DBID: strings.Repeat("1", 32), Sequence: 7}
		archive := old
		archive.Layout = 0
		archive.DBID = strings.Repeat("2", 32)
		archive.Sequence = 1
		next := old
		next.Layout = 2
		next.Mode = 1
		next.Sequence++
		restored := old
		restored.DBID = strings.Repeat("3", 32)
		restored.Sequence = 1
		root := File{Name: "source.gse", Size: 1024, SHA256: strings.Repeat("4", 64), Device: 1, Inode: 100, Mode: 0600}
		base := File{Name: "source.gse.gse-" + strings.Repeat("a", 64) + ".base", Size: 4096, SHA256: strings.Repeat("5", 64), Device: 1, Inode: 101, Mode: 0600}
		newRoot := root
		newRoot.SHA256 = strings.Repeat("6", 64)
		newRoot.Inode = 102
		retained := root
		retained.Name = "source.gse.gse-" + strings.Repeat("b", 64) + ".retained"
		retained.Inode = 103
		archiveFile := File{Name: "before-upgrade.archive", Size: 4096, SHA256: strings.Repeat("7", 64), Device: 1, Inode: 104, Mode: 0600}
		restoredFile := File{Name: "restored.gse", Size: 1024, SHA256: strings.Repeat("8", 64), Device: 1, Inode: 105, Mode: 0600}
		before := Manifest{root, base}
		after := Manifest{newRoot, base, retained}
		c := Case{PageSize: page, Root: caseRoot, SourceDirectory: sourceDir, ArchiveDirectory: archiveDir, RestoreDirectory: restoredDir, Before: old, Archive: archive, Upgraded: next, Reopened: next, Restored: restored,
			SourceBefore: Snapshot{sourceDir, "pre-upgrade-source", before}, SourceAfter: Snapshot{sourceDir, "post-upgrade-source", after}, SourceFinal: Snapshot{sourceDir, "final-source", append(Manifest(nil), after...)},
			ArchiveBefore: Snapshot{archiveDir, "pre-upgrade-archive", Manifest{archiveFile}}, ArchiveAfter: Snapshot{archiveDir, "final-archive", Manifest{archiveFile}}, RestoredFiles: Snapshot{restoredDir, "restored-snapshot", Manifest{restoredFile}},
			OldModeRejected: true, OldModeNil: true, OldModeUnchanged: true, Checks: RequiredChecks()}
		parents := map[string]int{}
		for _, role := range []string{"source", "old-mode-rejection", "wal-reopen", "restored-snapshot"} {
			id := add(page, "db", role, 0, role != "old-mode-rejection")
			parents[role] = id
			c.OwnerIDs = append(c.OwnerIDs, id)
		}
		for _, phase := range []string{"snapshot", "after-unique-refusal", "wal-reopened", "restored-snapshot"} {
			parent := parents["source"]
			if phase == "wal-reopened" {
				parent = parents["wal-reopen"]
			}
			if phase == "restored-snapshot" {
				parent = parents["restored-snapshot"]
			}
			for _, query := range []string{"rows", "unique", "grams", "scan", "vector"} {
				c.OwnerIDs = append(c.OwnerIDs, add(page, "rows", "rows:"+phase+"-"+query, parent, true))
			}
		}
		add(page, "file", "file:memory-family", 0, true)
		for _, label := range []string{"fixture-source", "fixture-archive", "fixture-restored"} {
			r.Providers = append(r.Providers, Provider{Page: page, Label: label, Acquires: 1, Releases: 1})
		}
		r.Cases = append(r.Cases, c)
	}
	return r
}
func TestFormatUpgradeReceiptMemoryCounterexamples(t *testing.T) {
	if err := memoryReceipt().Validate(); err != nil {
		t.Fatal(err)
	}
	mutants := []struct {
		name   string
		change func(*Receipt)
	}{
		{"zero-cases", func(r *Receipt) { r.Cases = nil }},
		{"missing-case", func(r *Receipt) { r.Cases = r.Cases[:2] }},
		{"duplicate-case", func(r *Receipt) { r.Cases[1] = r.Cases[0] }},
		{"wrong-page", func(r *Receipt) { r.Cases[2].PageSize = 1024 }},
		{"skipped-run", func(r *Receipt) { r.Status = "skipped" }},
		{"truncated-run", func(r *Receipt) { r.Status = "started" }},
		{"old-receipt-profile", func(r *Receipt) { r.Profile = "old" }},
		{"missing-approval", func(r *Receipt) { r.Approval = "" }},
		{"source-pin", func(r *Receipt) { r.SourceSHA256 = "HOLD" }},
		{"binary-pin", func(r *Receipt) { r.BinarySHA256 = strings.Repeat("0", 64) }},
		{"wrong-layout", func(r *Receipt) { r.Cases[0].Upgraded.Layout = 3 }},
		{"archive-is-not-family", func(r *Receipt) { r.Cases[0].Archive.Layout = 1 }},
		{"changed-dbid", func(r *Receipt) { r.Cases[0].Upgraded.DBID = strings.Repeat("8", 32) }},
		{"wrong-sequence", func(r *Receipt) { r.Cases[0].Upgraded.Sequence++ }},
		{"reopen-drift", func(r *Receipt) { r.Cases[0].Reopened.PageCount++ }},
		{"restore-original-id", func(r *Receipt) { r.Cases[0].Restored.DBID = r.Cases[0].Before.DBID }},
		{"restore-archive-id", func(r *Receipt) { r.Cases[0].Restored.DBID = r.Cases[0].Archive.DBID }},
		{"restore-page-loss", func(r *Receipt) { r.Cases[0].Restored.PageCount-- }},
		{"swallowed-old-mode", func(r *Receipt) { r.Cases[0].OldModeRejected = false }},
		{"unexpected-old-owner", func(r *Receipt) { r.Cases[0].OldModeNil = false }},
		{"refusal-mutated", func(r *Receipt) { r.Cases[0].OldModeUnchanged = false }},
		{"missing-assertion", func(r *Receipt) { r.Cases[0].Checks = r.Cases[0].Checks[:9] }},
		{"duplicate-assertion", func(r *Receipt) { r.Cases[0].Checks[1] = r.Cases[0].Checks[0] }},
		{"changed-immutable", func(r *Receipt) { r.Cases[0].SourceAfter.Files[1].SHA256 = strings.Repeat("9", 64) }},
		{"missing-retained-root", func(r *Receipt) { r.Cases[0].SourceAfter.Files = r.Cases[0].SourceAfter.Files[:2] }},
		{"root-treated-immutable", func(r *Receipt) { r.Cases[0].SourceAfter.Files[0] = r.Cases[0].SourceBefore.Files[0] }},
		{"source-final-mutated", func(r *Receipt) { r.Cases[0].SourceFinal.Files[0].Size++ }},
		{"archive-mutated", func(r *Receipt) { r.Cases[0].ArchiveAfter.Files[0].Size++ }},
		{"retained-name-format", func(r *Receipt) { r.Cases[0].SourceAfter.Files[2].Name = "source.gse-b.retained" }},
		{"wrong-restored-main", func(r *Receipt) { r.Cases[0].RestoredFiles.Files[0].Name = "substitute.gse" }},
		{"manifest-wrong-device", func(r *Receipt) { r.Cases[0].RestoredFiles.Files[0].Device = 2 }},
		{"missing-restored-manifest", func(r *Receipt) { r.Cases[0].RestoredFiles = Snapshot{} }},
		{"wrong-manifest-directory", func(r *Receipt) { r.Cases[0].SourceAfter.Directory = r.Cases[0].ArchiveDirectory }},
		{"wrong-manifest-phase", func(r *Receipt) { r.Cases[0].SourceBefore.Phase = "final-source" }},
		{"case-path-escape", func(r *Receipt) { r.Cases[0].SourceDirectory.Path = "/elsewhere/source" }},
		{"case-inode-alias", func(r *Receipt) { r.Cases[0].ArchiveDirectory.Inode = r.Cases[0].SourceDirectory.Inode }},
		{"run-path-binding", func(r *Receipt) { r.RunDirectory = "/elsewhere/run" }},
		{"run-root-reparented", func(r *Receipt) {
			r.RunRoot.Path = "/elsewhere/format-upgrade-fixture"
			r.RunDirectory = r.RunRoot.Path
		}},
		{"source-root-missing", func(r *Receipt) { r.SourceRoot = Directory{} }},
		{"owner-single-substitute", func(r *Receipt) { r.Owners = r.Owners[:1] }},
		{"acquisition-truncated", func(r *Receipt) { r.Acquisitions = r.Acquisitions[:1] }},
		{"acquisition-substituted", func(r *Receipt) { r.Acquisitions[0].Label = "different file" }},
		{"case-owner-truncated", func(r *Receipt) { r.Cases[0].OwnerIDs = r.Cases[0].OwnerIDs[:23] }},
		{"rows-parent-zero", func(r *Receipt) { r.Owners[5].Parent = 0; r.Acquisitions[5] = AcquisitionOf(r.Owners[5]) }},
		{"rows-wrong-db-parent", func(r *Receipt) { r.Owners[5].Parent = 4; r.Acquisitions[5] = AcquisitionOf(r.Owners[5]) }},
		{"absent-file", func(r *Receipt) {
			r.Owners[0].Present = false
			r.Owners[0].Attempts = 0
			r.Owners[0].Terminal = "absent"
			r.Owners[0].AcquireError = "arbitrary"
			r.Acquisitions[0] = AcquisitionOf(r.Owners[0])
		}},
		{"missing-nil-refusal", func(r *Receipt) { r.Owners[2].Refusal = "" }},
		{"negative-providers", func(r *Receipt) { r.ProviderAcquires = -1; r.ProviderReleases = -1 }},
		{"provider-case-missing", func(r *Receipt) { r.Providers = r.Providers[:8] }},
		{"provider-role-duplicate", func(r *Receipt) { r.Providers[1] = r.Providers[0] }},
		{"provider-zero-count", func(r *Receipt) { r.Providers[0].Acquires = 0; r.Providers[0].Releases = 0 }},
		{"internal-pending-parent", func(r *Receipt) { r.InternalPendingObserved = true }},
		{"pending-native", func(r *Receipt) { r.Pending = 1 }},
		{"provider-leak", func(r *Receipt) { r.ProviderReleases-- }},
		{"provider-double-release", func(r *Receipt) { r.ProviderDuplicateRelease = true }},
		{"owner-missing", func(r *Receipt) { r.Owners = nil }},
		{"owner-pending", func(r *Receipt) { r.Owners[0].Terminal = "live" }},
		{"owner-unknown", func(r *Receipt) { r.Owners[0].Terminal = "unknown" }},
		{"owner-double-close", func(r *Receipt) { r.Owners[0].Attempts = 2 }},
		{"owner-no-close", func(r *Receipt) { r.Owners[0].Attempts = 0 }},
		{"owner-close-error", func(r *Receipt) { r.Owners[0].RetireError = "unknown" }},
		{"owner-acquire-error", func(r *Receipt) { r.Owners[0].AcquireError = "failed" }},
	}
	for _, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			r := memoryReceipt()
			m.change(&r)
			if err := r.Validate(); err == nil {
				t.Fatal("false PASS")
			}
		})
	}
}
func TestFormatUpgradeOwnerMemoryOnce(t *testing.T) {
	o := Owner{ID: 1, Kind: "rows", Label: "memory", Present: true, Terminal: "live"}
	if err := o.Retire("eof", ""); err != nil || o.Attempts != 1 || o.Terminal != "retired" {
		t.Fatal(o, err)
	}
	before := o
	if err := o.Retire("close", ""); err == nil || !reflect.DeepEqual(before, o) {
		t.Fatal("double retirement changed receipt")
	}
	u := Owner{ID: 2, Kind: "db", Label: "memory", Present: true, Terminal: "live"}
	if err := u.Retire("close", "close diagnostic"); err != nil || u.Terminal != "unknown" {
		t.Fatal(u, err)
	}
	if err := u.Retire("close", ""); err == nil || u.Attempts != 1 || u.Terminal != "unknown" {
		t.Fatal("unknown was retried")
	}
}
func TestFormatUpgradeManifestMemoryBounds(t *testing.T) {
	for _, mutate := range []func(*File){func(f *File) { f.Name = "../escape" }, func(f *File) { f.Size = MaxBytes + 1 }, func(f *File) { f.SHA256 = "not-a-hash" }, func(f *File) { f.Inode = 0 }, func(f *File) { f.Mode = 0644 }} {
		f := memoryReceipt().Cases[0].SourceBefore.Files[0]
		mutate(&f)
		if err := (Manifest{f}).Validate(); err == nil {
			t.Fatal("unbounded/invalid manifest accepted")
		}
	}
	m := memoryReceipt().Cases[0].SourceBefore.Files
	if err := append(m, m[0]).Validate(); err == nil {
		t.Fatal("duplicate file accepted")
	}
}

func TestFormatUpgradePendingOwnershipMemoryOrchestration(t *testing.T) {
	db := Owner{ID: 1, Page: 512, Kind: "db", Label: "source", Present: true, Terminal: "live"}
	if err := CanRetire(db, []Owner{db}, false); err != nil {
		t.Fatal(err)
	}
	if err := CanRetire(db, []Owner{db}, true); err == nil || db.Attempts != 0 || db.Terminal != "live" {
		t.Fatal("unreturned internal pending child permitted parent Close")
	}
	rows := Owner{ID: 2, Page: 512, Kind: "rows", Label: "rows:snapshot-rows", Parent: 1, Present: true, Terminal: "live"}
	if err := rows.Retire("eof", "unknown cleanup"); err != nil {
		t.Fatal(err)
	}
	if err := CanRetire(db, []Owner{db, rows}, false); err == nil {
		t.Fatal("unknown captured child permitted parent Close")
	}
	if err := CanRetire(rows, []Owner{db, rows}, false); err == nil || rows.Attempts != 1 {
		t.Fatal("unknown retirement retried")
	}
	db.Blocked = "internal-encrypted-pending"
	if err := CanRetire(db, []Owner{db}, false); err == nil {
		t.Fatal("historical blocked owner retried")
	}
}
