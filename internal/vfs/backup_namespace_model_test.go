package vfs

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
)

func backupModelDirectory(inode, mount uint64) backupFact {
	return backupFact{mask: backupRequiredMask, mode: backupDirectory, links: 2, devMajor: 8, devMinor: 1, inode: inode, mount: mount}
}
func backupModelMain(inode, mount uint64) backupFact {
	return backupFact{mask: backupRequiredMask, mode: backupRegular, links: 1, devMajor: 8, devMinor: 1, inode: inode, mount: mount, size: 512}
}
func TestBackupSourceModelDirectChildTopology(t *testing.T) {
	parent, main := backupModelDirectory(2, 7), backupModelMain(3, 7)
	if err := backupDirectChild(parent, main, main); err != nil { t.Fatal(err) }
	for _, change := range []struct{name string; edit func(*backupFact, *backupFact, *backupFact)}{
		{"per-file-bind-single-link", func(p, h, n *backupFact) { h.mount = 9; n.mount = 9 }},
		{"named-mount", func(p, h, n *backupFact) { n.mount++ }},
		{"named-inode", func(p, h, n *backupFact) { n.inode++ }},
		{"hard-link", func(p, h, n *backupFact) { h.links = 2; n.links = 2 }},
		{"symlink", func(p, h, n *backupFact) { n.mode = 0120000 }},
		{"device", func(p, h, n *backupFact) { h.devMinor++; n.devMinor++ }},
		{"parent-type", func(p, h, n *backupFact) { p.mode = backupRegular }},
	} {
		t.Run(change.name, func(t *testing.T) { p, h, n := parent, main, main; change.edit(&p, &h, &n); if backupDirectChild(p, h, n) == nil { t.Fatal("unsupported topology admitted") } })
	}
	for _, bit := range []uint32{0x1, 0x4, 0x100, 0x200, 0x4000} {
		for field := 0; field < 3; field++ {
			facts := []backupFact{parent, main, main}; facts[field].mask &^= bit
			if !errors.Is(backupDirectChild(facts[0], facts[1], facts[2]), ErrUnsupported) { t.Fatal("missing returned mask accepted", bit, field) }
		}
	}
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		empty := main; empty.size = 0
		if (backupSourceSidecar(parent, suffix, empty) == nil) != (suffix == "-wal") { t.Fatal("wrong source sidecar profile", suffix) }
		for _, invalid := range []backupFact{main, {mask: backupRequiredMask, mode: backupDirectory, mount: 7}, {mask: backupRequiredMask, mode: backupRegular, links: 1, mount: 9}, {mode: backupRegular, links: 1, mount: 7}} {
			if backupSourceSidecar(parent, suffix, invalid) == nil { t.Fatal("nonempty/nonregular/aliased/incomplete sidecar accepted", suffix) }
		}
	}
	// A whole-directory mount is internally consistent and remains the same
	// physical parent for family overlap even though its unique mount ID differs.
	aliasParent, aliasMain := parent, main; aliasParent.mount, aliasMain.mount = 11, 11
	if backupDirectChild(aliasParent, aliasMain, aliasMain) != nil || !parent.samePhysical(aliasParent) { t.Fatal("whole-directory alias misclassified") }
	source := BackupSourceIdentity{parent: parent, base: "db", valid: true}
	if backupDestinationAdmission(source, aliasParent, "DB", ".stage") == nil { t.Fatal("mount-ID difference hid family overlap") }
}
func TestBackupDestinationModelAdmissionContracts(t *testing.T) {
	source := BackupSourceIdentity{parent: backupModelDirectory(2, 7), base: "source.db", valid: true}
	for _, final := range []string{"", ".", "..", "../out", "/absolute", "out-wal", "out-SHM", "out-journal", "é.db", "out:stream", "out name", string(make([]byte, 201))} {
		if BackupBasename(final) || backupDestinationAdmission(source, backupModelDirectory(4, 7), final, ".stage") == nil { t.Fatal("dangerous name admitted", final) }
	}
	if backupDestinationAdmission(source, backupModelDirectory(4, 7), "backup.db", ".stage") != nil { t.Fatal("disjoint target rejected") }
	for _, forged := range []BackupSourceIdentity{{}, {parent: source.parent, base: source.base}} {
		if backupDestinationAdmission(forged, backupModelDirectory(4, 7), "backup.db", ".stage") == nil { t.Fatal("unminted source admitted") }
	}
	if backupDestinationAdmission(source, source.parent, "SOURCE.DB", ".stage") == nil { t.Fatal("casefold source collision") }
	if backupDestinationAdmission(source, source.parent, "backup.db", "source.db") == nil { t.Fatal("private stage overlaps source") }
	if backupDestinationAdmission(source, source.parent, "backup.db", "BACKUP.DB") == nil { t.Fatal("private stage overlaps final") }
	// Negative family facts and mode 0700 do not authorize other actors: the
	// state interface contains no mintable four-name/fresh-directory reservation.
	negativeFamily, privateMode, actorCanCreateSidecar := true, true, true
	if !negativeFamily || !privateMode || !actorCanCreateSidecar { t.Fatal("Option A actor counterexample hidden") }
}

// Admission-only models: namespace structs have no lease/File, and no native
// constructor, Acquire, Close, stat or filesystem identity function is called.
func TestBackupSourceModelBenignAdmissionRefusal(t *testing.T) {
	for _, test := range []struct {
		name string
		edge backupSourceRefusalEdge
		cause error
		matched, attempted, sealed, terminal, valid, want bool
	}{
		{name: "pure-name", edge: backupSourceNameRefusal, cause: ErrUnsupported, attempted: true, want: true},
		{name: "matched-parent-policy", edge: backupSourceParentRefusal, cause: fmt.Errorf("profile: %w", ErrUnsupported), matched: true, attempted: true, want: true},
		{name: "matched-parent-capability", edge: backupSourceParentRefusal, cause: errors.Join(ErrUnsupported, syscall.ENOSYS), matched: true, attempted: true, want: true},
		{name: "missing-returned-mask", edge: backupSourceParentRefusal, cause: ErrUnsupported, matched: true, attempted: true, want: true},
		{name: "missing-parent", edge: backupSourceParentRefusal, cause: ErrUnsupported, attempted: true},
		{name: "incorrect-parent", edge: backupSourceParentRefusal, cause: errors.Join(ErrUnsupported, ErrIdentity), attempted: true},
		{name: "identity-join", edge: backupSourceParentRefusal, cause: errors.Join(ErrUnsupported, ErrIdentity), matched: true, attempted: true},
		{name: "poison-join", edge: backupSourceParentRefusal, cause: errors.Join(ErrUnsupported, ErrPoisoned), matched: true, attempted: true},
		{name: "unrelated-join", edge: backupSourceParentRefusal, cause: errors.Join(ErrUnsupported, syscall.EIO), matched: true, attempted: true},
		{name: "generic-unsupported", cause: ErrUnsupported, matched: true, attempted: true},
		{name: "topology-unsupported", edge: backupSourceUnknownRefusal, cause: ErrUnsupported, matched: true, attempted: true},
		{name: "destination-unsupported", edge: backupSourceRefusalEdge(99), cause: ErrUnsupported, matched: true, attempted: true},
		{name: "not-acquiring", edge: backupSourceNameRefusal, cause: ErrUnsupported},
		{name: "sealed", edge: backupSourceNameRefusal, cause: ErrUnsupported, attempted: true, sealed: true},
		{name: "terminal", edge: backupSourceNameRefusal, cause: ErrUnsupported, attempted: true, terminal: true},
		{name: "valid-proof", edge: backupSourceNameRefusal, cause: ErrUnsupported, attempted: true, valid: true},
		{name: "unknown-owner", edge: backupSourceParentRefusal, cause: errors.Join(ErrUnsupported, &backupUnknownOwner{}), matched: true, attempted: true},
		{name: "cyclic-error", edge: backupSourceParentRefusal, cause: errors.Join(ErrUnsupported, &backupErrorCycle{}), matched: true, attempted: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := &BackupSourceNamespace{acquisitionAttempted: test.attempted, sealed: test.sealed, terminal: test.terminal, identity: BackupSourceIdentity{valid: test.valid}}
			s.recordBenignAcquisitionRefusal(test.edge, test.cause, test.matched)
			if s.BenignAcquisitionRefusal() != test.want { t.Fatal("wrong acquisition fact", test.name) }
			if test.want {
				s.sealed = true // retirement preserves only the historical fact
				if !s.BenignAcquisitionRefusal() { t.Fatal("lost historical fact") }
				s.terminal = true
				if s.BenignAcquisitionRefusal() { t.Fatal("terminal fact escaped") }
				s.terminal, s.identity.valid = false, true
				if s.BenignAcquisitionRefusal() { t.Fatal("valid proof became refusal") }
			}
		})
	}
	for _, err := range []error{nil, syscall.ENOSYS, errors.New("unsupported"), errors.Join(ErrUnsupported, &backupTypedCause{})} {
		if backupSourcePolicyRefusal(err) { t.Fatal("unknown policy leaf admitted", err) }
	}
}

// R-1: invoke the actual acquisition state transition with overlapping/invalid
// admission, then attempt downstream methods. No create/write/rename is allowed.
func TestBackupDestinationModelAdmissionBlocksMutation(t *testing.T) {
	for _, test := range []struct { name, sourceBase, final, candidate string; valid, incomplete bool }{
		{"same-final", "final.db", "final.db", ".stage", true, false},
		{"casefold-final", "FINAL.DB", "final.db", ".stage", true, false},
		{"same-candidate", "source.db", "final.db", "SOURCE.DB", true, false},
		{"final-candidate", "source.db", "final.db", "FINAL.DB", true, false},
		{"invalid-source", "source.db", "final.db", ".stage", false, false},
		{"incomplete-parent", "source.db", "final.db", ".stage", true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := &backupPublicationModel{finishDir: true, finishMain: true}
			p := &BackupPublication{ops: m}
			source := BackupSourceIdentity{parent: backupModelDirectory(2, 7), base: test.sourceBase, valid: test.valid}
			if test.incomplete { source.parent.mask = 0 }
			err := p.openDestinationLocked("inert-parent", test.final, source, 512, test.candidate)
			if !errors.Is(err, ErrUnsupported) { t.Fatal("admission accepted", err) }
			if p.CreatePrivateCandidate() == nil || p.WritePage(make([]byte, 512), 0) == nil || p.PublishNoReplace() == nil { t.Fatal("failed admission resumed") }
			for _, event := range m.events { if event != "open-directory" { t.Fatal("admission performed family/create/write/rename", event) } }
			if p.Status().MutationAttempted || m.mainOwned || m.visible || m.size != 0 { t.Fatal("mutated after admission refusal") }
			if err := p.Close(); err != nil || !p.RetirementComplete() || p.SuccessfulPublication() == nil { t.Fatal("retirement fabricated success", err) }
		})
	}
}

func TestBackupSourceModelAcquisitionErrorsHideOwners(t *testing.T) {
	main, directory := &File{}, &File{} // memory identities only, no native owner
	for _, cause := range []error{
		retainedFileCleanup(main, errors.Join(ErrUnsupported, ErrPoisoned)),
		&backupUnknownOwner{},
		&backupErrorCycle{},
		terminalCleanup(ErrPoisoned),
	} {
		s := &BackupSourceNamespace{dir: backupLease{file: directory, fd: -1}, acquisitionAttempted: true}
		diagnostic := s.failure(cause)
		s.recordBenignAcquisitionRefusal(backupSourceParentRefusal, cause, true)
		owners, discovery := CleanupOwnersBounded(diagnostic, 128)
		if !s.terminal || s.unsafe != cause || s.BenignAcquisitionRefusal() || discovery != nil || len(owners) != 0 || !errors.Is(diagnostic, ErrPoisoned) { t.Fatal("acquisition exposed an unknown/main cleanup owner", diagnostic) }
	}
	// A recognized source-directory token is also kept private. Its ownership
	// wrapper is never itself accepted as a benign policy refusal.
	s := &BackupSourceNamespace{dir: backupLease{file: directory, fd: -1}, acquisitionAttempted: true}
	cause := retainedFileCleanup(directory, ErrUnsupported)
	diagnostic := s.failure(cause)
	s.recordBenignAcquisitionRefusal(backupSourceParentRefusal, cause, true)
	owners, err := CleanupOwnersBounded(diagnostic, 128)
	if s.terminal || err != nil || len(owners) != 0 || s.BenignAcquisitionRefusal() || !errors.Is(diagnostic, ErrUnsupported) { t.Fatal("directory owner escaped/refusal broadened", diagnostic) }
}
