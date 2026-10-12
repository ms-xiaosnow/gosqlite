package vfs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// These models call the actual BackupPublication transition methods with no
// native File, fd, path acquisition, random source or database. Opening is
// modeled from its checked admitted state; acquisition wiring is AST-guarded.
type backupPublicationModel struct {
	events []string
	sealed, pin, locked, complete, visible, competitor, sidecar bool
	dirOwned, mainOwned, finishDir, finishMain, createAtRename bool
	fail string
	dirErr, mainErr error
	size uint64
}
func (m *backupPublicationModel) event(name string) error {
	if m.sealed { panic("I/O after seal: " + name) }
	m.events = append(m.events, name)
	if m.fail == name { return fmt.Errorf("model %s", name) }
	return nil
}
func (m *backupPublicationModel) ownedFiles() []*File { return nil }
func (m *backupPublicationModel) openDirectory(string) (backupFact, error) {
	m.dirOwned = true
	return backupModelDirectory(2, 8), m.event("open-directory")
}
func (m *backupPublicationModel) absentFamily(string) error {
	if err := m.event("absence"); err != nil { return err }
	if m.competitor || m.sidecar { return os.ErrExist }
	return nil
}
func (m *backupPublicationModel) createCandidate(string) error {
	m.mainOwned, m.pin = true, true // descriptor returned into reserved owner
	return m.event("create")
}
func (m *backupPublicationModel) lockCandidate() error {
	if err := m.event("lock"); err != nil { return err }
	m.locked = true
	return nil
}
func (m *backupPublicationModel) validateCandidate() (uint64, error) { return m.size, m.event("validate") }
func (m *backupPublicationModel) writeCandidate(b []byte, off int64) (int, error) {
	if !m.locked { panic("write without protector") }
	if err := m.event("write"); err != nil { return 0, err }
	if m.fail == "short" { return len(b)-1, nil }
	m.size = uint64(off)+uint64(len(b))
	m.complete = m.size == 512
	return len(b), nil
}
func (m *backupPublicationModel) syncCandidate() error {
	if !m.locked || !m.complete { panic("incomplete/unlocked Sync") }
	return m.event("file-sync")
}
func (m *backupPublicationModel) publish(string, string) error {
	if err := m.event("rename"); err != nil { return err }
	if m.createAtRename { m.competitor = true }
	if m.competitor { return os.ErrExist }
	if !m.locked || !m.complete { panic("partial public inode") }
	m.visible = true
	return nil
}
func (m *backupPublicationModel) validatePublished() error { return m.event("published-validation") }
func (m *backupPublicationModel) syncDirectory() error { return m.event("directory-sync") }
func (m *backupPublicationModel) seal() error {
	m.events = append(m.events, "seal")
	m.sealed, m.pin = true, false
	return nil
}
func (m *backupPublicationModel) closeDirectory() error {
	if !m.sealed || m.pin { panic("lifetime pin not sealed") }
	m.events = append(m.events, "close-directory")
	if m.finishDir { m.dirOwned = false }
	return m.dirErr
}
func (m *backupPublicationModel) directoryComplete() bool { return !m.dirOwned }
func (m *backupPublicationModel) closeMain() error {
	if !m.sealed || m.pin || m.dirOwned { panic("early protector close") }
	m.events = append(m.events, "close-main")
	if m.finishMain { m.mainOwned, m.locked = false, false }
	return m.mainErr
}
func (m *backupPublicationModel) mainComplete() bool { return !m.mainOwned }
func backupModelOwner() (*BackupPublication, *backupPublicationModel) {
	m := &backupPublicationModel{dirOwned: true, finishDir: true, finishMain: true}
	p := &BackupPublication{ops: m, opened: true, final: "final.db", candidate: ".private-stage", expected: 512}
	return p, m
}
func backupModelCopy(p *BackupPublication) error {
	if err := p.CreatePrivateCandidate(); err != nil { return err }
	if err := p.WritePage(make([]byte, 512), 0); err != nil { return err }
	return p.SyncData()
}
func TestBackupPublicationModelStateAndFailures(t *testing.T) {
	for _, failure := range []string{"", "create", "lock", "absence", "validate", "write", "short", "file-sync", "rename", "published-validation", "directory-sync"} {
		t.Run(failure, func(t *testing.T) {
			p, m := backupModelOwner(); m.fail = failure
			err := backupModelCopy(p)
			if err == nil { err = p.PublishNoReplace() }
			if err == nil { err = p.SyncPublication() }
			if (err != nil) != (failure != "") { t.Fatal(failure, err) }
			if failure == "short" && !errors.Is(err, io.ErrShortWrite) { t.Fatal(err) }
			before := len(m.events)
			if closeErr := p.Close(); closeErr != nil || !p.RetirementComplete() { t.Fatal(closeErr) }
			if failure == "" && p.SuccessfulPublication() != nil { t.Fatal("success withheld") }
			if failure != "" && p.SuccessfulPublication() == nil { t.Fatal("failure became success") }
			for _, e := range m.events[before:] { if e != "seal" && e != "close-directory" && e != "close-main" { t.Fatal("cleanup replay", e) } }
			if failure == "published-validation" && !p.Status().DirectorySynced { t.Fatal("lost independently successful directory Sync") }
			if err := p.WritePage(nil, 0); err == nil { t.Fatal("post-seal write") }
			if err := p.PublishNoReplace(); err == nil { t.Fatal("post-seal publication") }
			if err := p.SyncData(); err == nil { t.Fatal("post-seal Sync") }
		})
	}
}
func TestBackupPublicationModelSameFinalRace(t *testing.T) {
	for _, ordering := range []string{"competitor-before-check", "competitor-at-rename", "publication-first"} {
		competitorFirst := ordering != "publication-first"
		p, m := backupModelOwner()
		if err := backupModelCopy(p); err != nil { t.Fatal(err) }
		if ordering == "competitor-before-check" { m.competitor = true }
		if ordering == "competitor-at-rename" { m.createAtRename = true }
		err := p.PublishNoReplace()
		if competitorFirst {
			if !errors.Is(err, os.ErrExist) || m.visible { t.Fatal("competitor inode adopted/written", err) }
		} else {
			if err != nil || !m.visible || !m.complete || !m.locked { t.Fatal("public inode incomplete/unlocked", err) }
			// Normal SQLite main-first actor cannot take SHARED/create a sidecar
			// while our already-published inode's EXCLUSIVE protector remains.
			canTakeShared := !m.locked
			if canTakeShared { m.sidecar = true }
			if m.sidecar { t.Fatal("ordinary actor bypassed main lock") }
		}
		if err := p.Close(); err != nil { t.Fatal(err) }
	}
	// Independent sidecar actors are a real counterexample to four-name atomicity.
	p, m := backupModelOwner()
	if err := backupModelCopy(p); err != nil { t.Fatal(err) }
	if err := p.PublishNoReplace(); err != nil { t.Fatal(err) }
	m.sidecar = true // a different main inode's lock does not coordinate this write
	if !m.visible || !m.sidecar || !m.locked { t.Fatal("counterexample erased") }
	if BackupBasename("other-wal") || BackupBasename("other-SHM") { t.Fatal("target-as-sidecar admitted") }
}
func TestBackupPublicationModelSealAndRetirement(t *testing.T) {
	for _, dirDone := range []bool{false, true} {
		for _, mainDone := range []bool{false, true} {
			for _, dirError := range []bool{false, true} {
				for _, mainError := range []bool{false, true} {
					p, m := backupModelOwner()
					if err := backupModelCopy(p); err != nil { t.Fatal(err) }
					m.finishDir, m.finishMain = dirDone, mainDone
					if dirError { m.dirErr = errors.New("consumed-or-owned-dir") }
					if mainError { m.mainErr = errors.New("consumed-or-owned-main") }
					if err := p.SealForRetirement(); err != nil || !m.locked || !m.mainOwned || !m.dirOwned || m.pin { t.Fatal("seal released protector", err) }
					_ = p.Close()
					if !dirDone && (!m.locked || !m.mainOwned || strings.Contains(strings.Join(m.events, ","), "close-main")) { t.Fatal("nil close inferred completion") }
					if p.RetirementComplete() != (dirDone && mainDone) { t.Fatal("wrong completion") }
					before := len(m.events)
					m.finishDir, m.finishMain, m.dirErr, m.mainErr = true, true, nil, nil
					if err := p.Close(); err != nil || !p.RetirementComplete() { t.Fatal(err) }
					for _, e := range m.events[before:] { if e != "close-directory" && e != "close-main" { t.Fatal("non-retirement retry", e) } }
				}
			}
		}
	}
	// Historical unconditional child pin makes dir-before-main impossible.
	pin, directoryOwned, mainOwned := true, true, true
	canCloseDirectory := !pin
	canCloseMain := !directoryOwned
	if canCloseDirectory || canCloseMain || !mainOwned { t.Fatal("historical cycle not exposed") }
}

type backupUnknownOwner struct{}
func (*backupUnknownOwner) Error() string { return "unknown owner" }
func (*backupUnknownOwner) Close() error { return nil }
type backupErrorCycle struct{}
func (e *backupErrorCycle) Error() string { return "cycle" }
func (e *backupErrorCycle) Unwrap() error { return e }
type backupTypedCause struct{}
func (*backupTypedCause) Error() string { return "typed leaf" }
func TestBackupPublicationModelOwnedAcquisitionFailure(t *testing.T) {
	for _, input := range []error{&backupUnknownOwner{}, &backupErrorCycle{}, terminalCleanup(errors.New("unknown consumption"))} {
		p, m := backupModelOwner()
		if err := backupModelCopy(p); err != nil { t.Fatal(err) }
		err := p.fail(input)
		if !errors.Is(err, ErrPoisoned) { t.Fatal(err) }
		before := len(m.events)
		if p.Close() == nil || p.RetirementComplete() || len(m.events) != before || !m.locked { t.Fatal("unknown ownership released protector") }
	}
	// A represented interior File token becomes diagnostics, never an outer
	// CleanupOwner. No native descriptor is created or retired by this test.
	f := &File{}
	cause := &backupTypedCause{}
	err, terminal := backupError(retainedFileCleanup(f, cause), f)
	owners, discovery := CleanupOwnersBounded(err, 128)
	var typed *backupTypedCause
	var escaped CleanupOwner
	if terminal || discovery != nil || len(owners) != 0 || !errors.As(err, &typed) || errors.As(err, &escaped) || !errors.Is(err, cause) { t.Fatal("owner escaped diagnostic facade", err) }
	// Exercise the actual admission/acquisition transition boundary with fixed
	// names. Every owned failure remains in the composite before cleanup starts.
	for _, phase := range []string{"open-directory", "absence", "create", "lock", "validate"} {
		m := &backupPublicationModel{fail: phase, finishDir: true, finishMain: true}
		p := &BackupPublication{ops: m}
		source := BackupSourceIdentity{parent: backupModelDirectory(1, 7), base: "source.db", valid: true}
		err := p.openDestinationLocked("inert-parent", "final.db", source, 512, ".fixed-stage")
		if err == nil { err = p.CreatePrivateCandidate() }
		if err == nil { err = p.WritePage(make([]byte, 512), 0) }
		if err == nil || !m.dirOwned { t.Fatal("acquisition failure lost directory owner", phase, err) }
		if (phase == "create" || phase == "lock" || phase == "validate") && !m.mainOwned { t.Fatal("returned child was not retained", phase) }
		if err := p.Close(); err != nil || !p.RetirementComplete() { t.Fatal("represented retirement failed", phase, err) }
	}
}
