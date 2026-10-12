package checkpointrestart

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"testing"
)

func modelHash(s string) Hash { return sha256.Sum256([]byte(s)) }
func modelFile(name string, id, extent uint64, hash Hash, header RootHeader, bound bool) FamilyEntry {
	s := FileState{Identity: FileIdentity{7, id}, Hash: hash, Bytes: extent, Mode: 0600, OwnerUID: 7, Regular: true, Links: 1, PathFDMatch: true}
	return FamilyEntry{Name: name, Before: s, After: s, Header: header, HeaderBound: bound}
}
func modelCopy(m FamilyManifest, family Family, delta uint64) FamilyManifest {
	out := m
	out.Family = family
	out.Source = m.Family
	out.Directory.Inode += delta
	out.Entries = append([]FamilyEntry(nil), m.Entries...)
	out.DestinationAbsent = true
	out.ExclusiveCopies = true
	out.DestinationSynced = true
	out.FrozenAfterWriterWait = true
	for i := range out.Entries {
		out.Entries[i].Before.Identity.Inode += delta
		out.Entries[i].After = out.Entries[i].Before
	}
	return out
}
func sortModelFamily(m *FamilyManifest) {
	sort.Slice(m.Entries, func(i, j int) bool { return m.Entries[i].Name < m.Entries[j].Name })
}
func syntheticEvidence(c Case) Evidence {
	e := Evidence{Binding: Binding{Schema: SchemaVersion, Case: c, Profile: CaseProfile(c), Trial: modelHash("trial"), SourceManifest: modelHash("source"), Compiler: modelHash("compiler"), CompileReceipt: modelHash("compile-only"), Binary: modelHash("binary"), Fixture: FixtureHash()}, Filesystem: "synthetic-same-kernel-value-only", Seed: syntheticLedger(Controller), WriterLedger: syntheticLedger(Writer)}
	for i := range e.Roles {
		e.Roles[i] = RoleBinding{Role: Role(i + 1), Generation: uint64(i + 1), Challenge: modelHash(string(rune('a' + i)))}
	}
	root := "fixture.db"
	if e.Binding.Profile == EncryptedLayout2Raw4096 {
		root = "fixture.gse"
	}
	e.Paths = Paths{Run: "/retained/trial1", Original: "/retained/trial1/original/" + root, Trial: "/retained/trial1/trial/" + root, PostExit: "/retained/trial1/evidence/post-exit/" + root, Probe: "/retained/trial1/probe/" + root, Control: "/retained/trial1/control"}
	original := FamilyManifest{Family: OriginalFamily, RootName: root, Directory: FileIdentity{7, 500}, DirectoryMode: 0700, OwnerUID: 7, Complete: true, Frozen: true, ReadbackVerified: true, NoDatabaseReopen: true}
	if e.Binding.Profile != EncryptedLayout2Raw4096 {
		e.Plain = PlainHead{PageSize: 4096, DBSize: 64, FrameCount: 5, EndOffset: 32 + 5*(4096+24), Salt: [2]uint32{21, 22}, Checksum: [2]uint32{31, 32}, MainHash: modelHash("baseline-main"), WALHash: modelHash("committed-wal"), WALBytes: 32 + 5*(4096+24), Complete: true, CleanTail: true, Selected: []SelectedPage{{Page: 1, Frame: 4, Hash: modelHash("page1")}, {Page: 2, Frame: 5, Hash: modelHash("page2")}}}
		original.Entries = []FamilyEntry{modelFile(root, 501, 64*4096, e.Plain.MainHash, RootHeader{}, false), modelFile(root+"-wal", 502, e.Plain.WALBytes, e.Plain.WALHash, RootHeader{}, false)}
		if e.Binding.Profile == PlainShared4096 {
			original.Entries = append(original.Entries, modelFile(root+"-shm", 503, 32768, modelHash("shm"), RootHeader{}, false))
		}
	} else {
		epoch := Epoch{DatabaseID: [16]byte{1}, EpochID: modelHash("epoch"), Kind: 1, RootSalt: modelHash("root-salt")}
		old := RootHeader{Major: 1, Suite: 1, Epoch: epoch, FileID: modelHash("old-root-file-id"), NoncePrefix: [4]byte{1}, Descriptor: Descriptor{Role: 1, PageSize: 4096, PageCount: 64, RecordCount: 1, CommitSequence: 4, TransactionID: modelHash("tx4"), ParentCommitID: modelHash("committed-head3")}}
		next := old
		next.FileID = modelHash("new-root-file-id")
		next.NoncePrefix = [4]byte{2}
		next.Descriptor.CommitSequence = 5
		next.Descriptor.TransactionID = modelHash("tx5")
		next.Descriptor.ParentCommitID = modelHash("authenticated-head4")
		e.Old = EncryptedHead{Authenticated: true, RootHash: modelHash("old-root"), RootBytes: 1320, Header: old, Layout: 2, HeadCommitID: modelHash("authenticated-head4"), BaseFileID: modelHash("base3-id"), WALSegments: 1, WALFrames: 5, WALBytes: 928 + 5*(4096+44)}
		e.Candidate = RootCandidate{RootHash: modelHash("new-root"), RootBytes: 1064, Header: next}
		base := old
		base.FileID = e.Old.BaseFileID
		base.Descriptor.Role = 2
		base.Descriptor.RecordCount = 66
		base.Descriptor.CommitSequence = 3
		base.Descriptor.TransactionID = modelHash("tx3")
		base.Descriptor.ParentCommitID = modelHash("head2")
		wal := old
		wal.FileID = modelHash("wal4-id")
		wal.Descriptor.Role = 3
		wal.Descriptor.RecordCount = 7
		original.Entries = []FamilyEntry{modelFile(root, 501, 1320, e.Old.RootHash, old, true), modelFile(root+".gse-lock", 502, 0, modelHash("lock"), RootHeader{}, false), modelFile(root+".gse-"+hex.EncodeToString(base.FileID[:])+".base", 503, 704+64*(4096+40), modelHash("base3"), base, true), modelFile(root+".gse-"+hex.EncodeToString(wal.FileID[:])+".wal", 504, e.Old.WALBytes, modelHash("wal4"), wal, true)}
		for i, role := range []Role{Controller, Writer, Probe} {
			op := uint8(1)
			if i == 0 {
				op = 2
			}
			e.Credentials = append(e.Credentials, CredentialReceipt{Role: role, Provider: uint64(i + 1), Path: []string{e.Paths.Original, e.Paths.Trial, e.Paths.Probe}[i], Operation: op, Kind: 1, Acquires: 1, Releases: 1, Cleared: true, PurposeChecked: true})
		}
	}
	sortModelFamily(&original)
	e.SeedCopy = modelCopy(original, TrialFamily, 10000)
	trial := modelCopy(original, TrialFamily, 10000)
	if e.Binding.Profile != EncryptedLayout2Raw4096 {
		for i := range trial.Entries {
			if trial.Entries[i].Name == root {
				trial.Entries[i].Before.Hash = modelHash("checkpoint-main")
				trial.Entries[i].After = trial.Entries[i].Before
			}
		}
		if c == ExclusiveControl || c == ExclusiveAfterWALUnlink {
			entries := []FamilyEntry{}
			for _, a := range trial.Entries {
				if a.Name != root+"-wal" {
					entries = append(entries, a)
				}
			}
			trial.Entries = entries
		}
	} else {
		candidateBase := e.Candidate.Header
		candidateBase.FileID = modelHash("base5-id")
		candidateBase.Descriptor.Role = 2
		candidateBase.Descriptor.RecordCount = 66
		trial.Entries = append(trial.Entries, modelFile(root+".gse-"+hex.EncodeToString(candidateBase.FileID[:])+".base", 700, 704+64*(4096+40), modelHash("base5"), candidateBase, true), modelFile(root+".gse-"+hex.EncodeToString(e.Old.Header.FileID[:])+".retained", 701, 1320, e.Old.RootHash, e.Old.Header, true))
		if c == EncryptedBeforeRootRename {
			trial.Entries = append(trial.Entries, modelFile(root+".gse-"+hex.EncodeToString(e.Candidate.Header.FileID[:])+".pending", 702, 1064, e.Candidate.RootHash, e.Candidate.Header, true))
		} else {
			for i := range trial.Entries {
				if trial.Entries[i].Name == root {
					trial.Entries[i] = modelFile(root, 702, 1064, e.Candidate.RootHash, e.Candidate.Header, true)
				}
			}
		}
	}
	sortModelFamily(&trial)
	post := modelCopy(trial, PostExitFamily, 20000)
	probe := modelCopy(post, ProbeFamily, 30000)
	probe.Frozen = false
	probe.NoDatabaseReopen = false
	e.Families = []FamilyManifest{original, trial, post, probe}
	for i, point := range ExpectedPoints(c) {
		o := Observation{Point: point}
		if e.Binding.Profile == EncryptedLayout2Raw4096 {
			o.OldRootHash = e.Old.RootHash
			o.CandidateRootHash = e.Candidate.RootHash
			o.OldHeader = e.Old.Header
			o.CandidateHeader = e.Candidate.Header
			o.CandidateBytes = e.Candidate.RootBytes
		} else {
			o.SelectedCount = 2
			o.CopiedCount = 2
			if i == 0 {
				o.Page = 1
				o.CopiedCount = 1
			}
			o.DBSize = 64
			o.FrameCount = e.Plain.FrameCount
			o.Salt = e.Plain.Salt
			o.Checksum = e.Plain.Checksum
			if e.Binding.Profile == PlainShared4096 {
				o.Safe = 5
				o.Attempted = 5
				if i > 0 {
					o.Backfill = 5
				}
			}
		}
		e.Observations = append(e.Observations, o)
	}
	ledger, _ := LedgerDigest(e.Seed)
	var head Hash
	if e.Binding.Profile == EncryptedLayout2Raw4096 {
		head, _ = canonicalHash(e.Old)
	} else {
		head, _ = canonicalHash(e.Plain)
	}
	e.Commit = CommitReceipt{Binding: e.Binding, Present: true, CommitCalls: 1, CommitReturnedNil: true, CapturedAdjacent: true, ActualCommitOrigin: true, BeforeCheckpoint: true, LedgerDigest: ledger, HeadDigest: head, File: FileIdentity{7, 800}, Directory: FileIdentity{7, 801}, Hash: modelHash("external-durable-commit-receipt"), FullWrite: true, FileSynced: true, FileCloseCalls: 1, DirectorySynced: true, DirectoryCloseCalls: 1, ReadbackVerified: true, ExclusiveCreated: true}
	e.Return = ReturnEvidence{Calls: 1}
	if IsControl(c) {
		e.Return.Returned = true
		e.Return.Nil = true
		e.Return.CapturedAdjacent = true
	}
	for i, role := range []Role{Writer, Probe} {
		r := e.Roles[i+1]
		p := ProcessReceipt{Role: role, Generation: r.Generation, Challenge: r.Challenge, Owner: uint64(100 + i), Starts: 1, Waits: 1, ParentOwned: true, ActualSelfImage: true, Image: FileIdentity{7, 900}, ImageHash: e.Binding.Binary, State: ProcessExited, ActualWait: true, ProcessStatePresent: true, NaturalExit: true, TerminalEOF: true}
		if role == Writer && !IsControl(c) {
			p.ExitCode = 86
			p.NaturalExit = false
			p.OriginalExitError = true
			p.WaitErrorToken = 55
		}
		e.Processes = append(e.Processes, p)
	}
	for i, role := range []Role{Controller, Writer, Probe} {
		o := OwnerReceipt{Role: role, Generation: e.Roles[i].Generation, Owner: uint64(role) * 1000, AfterKnown: true, DBCloseCalls: 1, DependentOwnersRetired: true}
		if role == Writer && !IsControl(c) {
			o.AfterKnown = false
			o.DBCloseCalls = 0
			o.DependentOwnersRetired = false
		}
		e.Owners = append(e.Owners, o)
	}
	for i, kind := range []string{"event-pipe", "control-pipe", "event-pipe", "control-pipe"} {
		e.Resources = append(e.Resources, ResourceReceipt{Owner: uint64(200 + i), Parent: 1000, Kind: kind, CapturedBeforeError: true, CloseCalls: 1, Closed: true})
	}
	e.Probe = ProbeEvidence{OpenCalls: 1, CloseCalls: 1, NoCreate: true, KeepWALOnClose: true, SameProfile: true, Ledger: syntheticLedger(Probe), MainWALUnchanged: true}
	if e.Binding.Profile == EncryptedLayout2Raw4096 {
		e.Probe.ReadOnly = true
		e.Probe.VerifyCalls = 1
		e.Probe.VerifiedSelectedRoot = true
		e.Probe.Encrypted = e.Old
		if c != EncryptedBeforeRootRename {
			e.Probe.Encrypted = EncryptedHead{Authenticated: true, RootHash: e.Candidate.RootHash, RootBytes: 1064, Header: e.Candidate.Header, Layout: 2, HeadCommitID: modelHash("authenticated-new-base-head"), BaseFileID: modelHash("base5-id")}
		}
	}
	observed := 0
	var prior Frame
	for i, event := range ExpectedTrace(c) {
		role := eventRole(event)
		r := e.Roles[int(role)-1]
		f := Frame{Binding: e.Binding, Role: role, Generation: r.Generation, Challenge: r.Challenge, Sequence: uint16(i + 1), Event: event, Payload: ledger}
		switch event {
		case ParentAdmitted:
			f.Payload = e.Binding.SourceManifest
		case SeedCommitNil:
			f.Payload = head
		case CommittedReceiptDurable:
			f.Payload = e.Commit.Hash
		case CutObserved:
			f.Point = e.Observations[observed].Point
			f.Payload, _ = canonicalHash(e.Observations[observed])
			observed++
		case CutContinued, ExitGranted:
			f.Point = prior.Point
			f.Payload = prior.Payload
		}
		switch event {
		case WriterOpenGranted, CheckpointGranted, CutContinued, ExitGranted, WriterDoneGranted, ProbeOpenGranted:
			f.Echo, _ = FrameHash(prior)
		}
		e.Frames = append(e.Frames, f)
		prior = f
	}
	return e
}
func cloneEvidence(e Evidence) Evidence {
	b, _ := json.Marshal(e)
	var out Evidence
	_ = json.Unmarshal(b, &out)
	return out
}
func requireEvidence(t *testing.T, e Evidence) {
	t.Helper()
	r := Validate(e)
	if !r.SourceModelConsistent || r.Problem != NoProblem || r.PhysicalValidated {
		t.Fatalf("synthetic %s inconsistent: %+v", CaseName(e.Binding.Case), r)
	}
}
func rejectEvidence(t *testing.T, e Evidence) {
	t.Helper()
	if r := Validate(e); r.SourceModelConsistent || r.Problem == NoProblem || r.PhysicalValidated {
		t.Fatalf("invalid supplied evidence accepted: %+v", r)
	}
}

func TestCheckpointRestartTenCases(t *testing.T) {
	if len(Cases()) != 10 || len(Profiles()) != 3 {
		t.Fatal("closed extent")
	}
	seen := map[string]bool{}
	for _, c := range Cases() {
		name := CaseName(c)
		parsed, ok := ParseCase(name)
		if !ok || parsed != c || seen[name] {
			t.Fatal("case identity")
		}
		seen[name] = true
		e := syntheticEvidence(c)
		requireEvidence(t, e)
		if p := ValidateAuthority(c, e.Plain, e.Old, e.Candidate, e.Observations); p != NoProblem {
			t.Fatal("public authority mirror", p)
		}
	}
	if _, ok := ParseCase("encrypted-layout3"); ok {
		t.Fatal("unapproved case")
	}
	if CaseName(Case(255)) != "" || CaseProfile(Case(255)) != ProfileUnknown {
		t.Fatal("unknown case")
	}
}
func TestCheckpointRestartAuthorityCuts(t *testing.T) {
	for _, c := range Cases() {
		e := syntheticEvidence(c)
		requireEvidence(t, e)
		for i := range e.Observations {
			x := cloneEvidence(e)
			x.Observations = append(x.Observations[:i], x.Observations[i+1:]...)
			rejectEvidence(t, x)
		}
		if len(e.Observations) > 1 {
			x := cloneEvidence(e)
			x.Observations[0], x.Observations[1] = x.Observations[1], x.Observations[0]
			rejectEvidence(t, x)
		}
		if e.Binding.Profile != EncryptedLayout2Raw4096 {
			x := cloneEvidence(e)
			x.Observations[0].CopiedCount = 2
			rejectEvidence(t, x)
			x = cloneEvidence(e)
			x.Plain.Selected = x.Plain.Selected[:1]
			rejectEvidence(t, x)
		}
	}
	e := syntheticEvidence(SharedAfterBackfill)
	e.Observations[1].Backfill = 0
	rejectEvidence(t, e)
	e = syntheticEvidence(SharedControl)
	e.Plain.Selected = append([]SelectedPage{{Page: 1, Frame: 1, Hash: modelHash("prior-backfill")}}, []SelectedPage{{Page: 2, Frame: 4, Hash: modelHash("page2")}, {Page: 3, Frame: 5, Hash: modelHash("page3")}}...)
	for i := range e.Observations {
		e.Observations[i].OldBackfill = 1
	}
	e.Observations[0].Page = 2
	e.Observations[0].Backfill = 1
	if p := ValidateAuthority(e.Binding.Case, e.Plain, e.Old, e.Candidate, e.Observations); p != NoProblem {
		t.Fatal("complete WAL directory must project already-backfilled pages", p)
	}
	e.Observations[0].Page = 1
	if p := ValidateAuthority(e.Binding.Case, e.Plain, e.Old, e.Candidate, e.Observations); p == NoProblem {
		t.Fatal("accepted already-backfilled first copy")
	}
}
func TestCheckpointRestartCommittedHeadCausality(t *testing.T) {
	e := syntheticEvidence(ExclusiveAfterWALUnlink)
	for _, edit := range []func(*Evidence){func(e *Evidence) { e.Commit.BeforeCheckpoint = false }, func(e *Evidence) { e.Commit.ActualCommitOrigin = false }, func(e *Evidence) { e.Commit.CommitReturnedNil = false }, func(e *Evidence) { e.Commit.CheckpointACK = true }, func(e *Evidence) { e.Commit.FileCloseCalls = 2 }, func(e *Evidence) { e.Return.Returned = true; e.Return.Nil = true }, func(e *Evidence) { e.Commit.LedgerDigest = modelHash("stale baseline B") }, func(e *Evidence) { e.Probe.Ledger.Streams[0].Rows = e.Probe.Ledger.Streams[0].Rows[:6] }, func(e *Evidence) { e.WriterLedger.Streams[4].Rows = e.WriterLedger.Streams[4].Rows[:2] }} {
		x := cloneEvidence(e)
		edit(&x)
		rejectEvidence(t, x)
	}
}
func TestCheckpointRestartTransportBinding(t *testing.T) {
	e := syntheticEvidence(EncryptedControl)
	for _, edit := range []func(*Evidence){func(e *Evidence) { e.Frames[0].Binding.Binary = modelHash("other binary") }, func(e *Evidence) { e.Frames[7].Challenge = modelHash("replay") }, func(e *Evidence) { e.Frames[1].Sequence = 1 }, func(e *Evidence) { e.Frames[8].Echo = modelHash("wrong event hash") }, func(e *Evidence) { e.Frames = append(e.Frames, e.Frames[len(e.Frames)-1]) }, func(e *Evidence) { e.Binding.Profile = PlainExclusive4096 }, func(e *Evidence) { e.Roles[2].Generation = e.Roles[1].Generation }} {
		x := cloneEvidence(e)
		edit(&x)
		rejectEvidence(t, x)
	}
	b, p := EncodeFrame(e.Frames[0])
	if p != NoProblem {
		t.Fatal(p)
	}
	if _, p = DecodeFrame(b); p != NoProblem {
		t.Fatal(p)
	}
	bad := bytes.Replace(b, []byte(`"sequence":1`), []byte(`"sequence":1,"sequence":1`), 1)
	if _, p = DecodeFrame(bad); p == NoProblem {
		t.Fatal("duplicate frame key")
	}
	if _, p = DecodeFrame(append(b, '\n')); p == NoProblem {
		t.Fatal("trailing frame data")
	}
}
func TestCheckpointRestartOwnerWaitAndUnknown(t *testing.T) {
	e := syntheticEvidence(SharedAfterFirstMainCopy)
	for _, edit := range []func(*Evidence){func(e *Evidence) { e.Processes[0].Waits = 0 }, func(e *Evidence) { e.Processes[0].Waits = 2 }, func(e *Evidence) { e.Processes[0].ActualWait = false }, func(e *Evidence) { e.Processes[0].ProcessStatePresent = false }, func(e *Evidence) { e.Processes[0].TerminalEOF = false }, func(e *Evidence) { e.Processes[0].DiagnosticStall = true }, func(e *Evidence) { e.Processes[0].SignalNumber = 9 }, func(e *Evidence) { e.Processes[0].OriginalExitError = false }, func(e *Evidence) { e.Owners[1].DBCloseCalls = 1 }, func(e *Evidence) { e.Owners[2].After.Encrypted = 1 }, func(e *Evidence) { e.Resources[0].CloseCalls = 2 }, func(e *Evidence) { e.Resources[0].ReleasedBeforeDependency = true }} {
		x := cloneEvidence(e)
		edit(&x)
		rejectEvidence(t, x)
	}
	p := e.Processes[0]
	p.ActualWait = false
	p.State = ProcessPending
	if !NeedsUnresolvedHold(p) {
		t.Fatal("abandoned live child")
	}
	p.ActualWait = true
	p.State = ProcessExited
	p.ProcessStatePresent = false
	if !NeedsUnresolvedHold(p) {
		t.Fatal("invented Wait ProcessState")
	}
	p.ProcessStatePresent = true
	if NeedsUnresolvedHold(p) {
		t.Fatal("actual terminal disposition")
	}
}
func TestCheckpointRestartPrivateFamilyCopies(t *testing.T) {
	e := syntheticEvidence(EncryptedAfterRootRename)
	for _, edit := range []func(*Evidence){func(e *Evidence) { e.Families[2].FrozenAfterWriterWait = false }, func(e *Evidence) { e.Families[2].Entries[0].Before.Identity = e.Families[1].Entries[0].Before.Identity }, func(e *Evidence) { e.Families[0].Entries[0].Before.Links = 2 }, func(e *Evidence) { e.Families[1].Entries[0].Name += ".unexpected" }, func(e *Evidence) { e.Families[0].Entries[0].After.Hash = modelHash("mutated original") }, func(e *Evidence) { e.Probe.RepairCalls = 1 }, func(e *Evidence) { e.Probe.OpenCalls = 2 }, func(e *Evidence) { e.Paths.Probe = e.Paths.Original }, func(e *Evidence) { e.Families[0].Entries[0].Before.Bytes = MaxFileBytes + 1 }} {
		x := cloneEvidence(e)
		edit(&x)
		rejectEvidence(t, x)
	}
	x := syntheticEvidence(SharedControl)
	x.Probe.SharedSHMReconstructed = true
	for i := range x.Families[3].Entries {
		if x.Families[3].Entries[i].Name == "fixture.db-shm" {
			x.Families[3].Entries[i].After.Hash = modelHash("legitimate SHM reinitialization")
		}
	}
	requireEvidence(t, x)
	x = syntheticEvidence(ExclusiveAfterFirstMainCopy)
	for i := range x.Families {
		a := modelFile("fixture.db-shm", uint64(90000+i*10000), 32768, modelHash("stale SHM"), RootHeader{}, false)
		if i == 3 {
			a.Removed = true
			a.After = FileState{}
		}
		x.Families[i].Entries = append(x.Families[i].Entries, a)
		sortModelFamily(&x.Families[i])
	}
	x.SeedCopy.Entries = append(x.SeedCopy.Entries, modelFile("fixture.db-shm", 100000, 32768, modelHash("stale SHM"), RootHeader{}, false))
	sortModelFamily(&x.SeedCopy)
	requireEvidence(t, x)
}
func TestCheckpointRestartEncryptedSameEpoch(t *testing.T) {
	e := syntheticEvidence(EncryptedBeforeRootRename)
	for _, edit := range []func(*Evidence){func(e *Evidence) { e.Candidate.Header.Epoch.EpochID = modelHash("rekey forbidden") }, func(e *Evidence) { e.Candidate.Header.Epoch.RootSalt = modelHash("new salt forbidden") }, func(e *Evidence) { e.Candidate.Header.Descriptor.ParentCommitID = e.Old.RootHash }, func(e *Evidence) { e.Candidate.Header.FileID = e.Old.Header.FileID }, func(e *Evidence) { e.Candidate.Header.Descriptor.CommitSequence = 6 }, func(e *Evidence) { e.Old.WALSegments = 0 }, func(e *Evidence) { e.Probe.Encrypted.RootHash = e.Candidate.RootHash }, func(e *Evidence) { e.Credentials[1].Operation = 3 }} {
		x := cloneEvidence(e)
		edit(&x)
		rejectEvidence(t, x)
	}
	e = syntheticEvidence(EncryptedAfterRootRename)
	e.Probe.Encrypted = e.Old
	rejectEvidence(t, e)
}
func TestCheckpointRestartNeverPhysicalPass(t *testing.T) {
	for _, c := range Cases() {
		e := syntheticEvidence(c)
		r := Validate(e)
		if !r.SourceModelConsistent || r.PhysicalValidated || r.PublicCheckpointReturned != IsControl(c) {
			t.Fatalf("claim separation: %+v", r)
		}
		for _, edit := range []func(*Evidence){func(e *Evidence) { e.PhysicalClaim = true }, func(e *Evidence) { e.PowerLossClaim = true }, func(e *Evidence) { e.HardwareSectorClaim = true }} {
			x := cloneEvidence(e)
			edit(&x)
			rejectEvidence(t, x)
		}
	}
}
