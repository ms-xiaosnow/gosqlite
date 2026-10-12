package rekeyinterruption

import (
	"encoding/binary"
	"testing"
)

// Every fixture below is invented in memory. No helper opens a database,
// inspects a native object, derives a credential or establishes physical truth.
func syntheticGeneration(label Label, seed byte) Generation {
	g := Generation{
		Authenticated: true, Label: label, Mode: "snapshot", Kind: "RawKey256", Layout: 1, PageSize: 4096, PageCount: 3,
		DatabaseID: [16]byte{1}, EpochID: Hash{seed}, RootSalt: Hash{seed + 1}, RootFileID: Hash{seed + 2}, BaseFileID: Hash{seed + 3},
		RootHash: Hash{seed + 4}, BaseHash: Hash{seed + 5}, HeadHash: Hash{seed + 4}, Sequence: 4,
		LogicalPages: [3]Hash{{101}, {102}, {103}}, PageOneCounters: [6]uint32{4, 3, 2, 0, 0, 4},
	}
	if label == NEW {
		g.Sequence = 5
	}
	if label == OLD {
		g.ParentCommitID = Hash{seed + 6}
	}
	return g
}

func syntheticSQL(role Role, generation, owner uint64) SQLLedger {
	l := SQLLedger{Role: role, Generation: generation, Owner: owner,
		Check: CheckEvidence{Scope: CheckScope, Calls: 1, MaxPages: 3, MaxPageReads: 512, Pages: 3, Tables: 1, Indexes: 1, TableRows: 3, IndexEntries: 3, PageReads: 7}}
	for i := range l.Queries {
		q := SQLQuery(i)
		rows, count := ExpectedSQLLedger(q)
		r := SQLQueryEvidence{Query: q, SQL: RekeyQueryText(q), Owner: owner, Iterator: 2000 + uint64(role)*100 + uint64(i),
			Rows: rows, RowCount: count, EOF: true, ErrCalled: true, CloseCalls: 1,
			Plan: RekeyQueryPlan{Access: "SCAN", Table: "t", Reason: "NOT INDEXED requested"},
			Work: SQLWork{BaseRowFetches: 3, ReturnedRows: int64(count)}}
		if q == SQLBlueIndex || q == SQLRedIndex || q == SQLGreenIndex {
			r.Plan = RekeyQueryPlan{Access: "SEARCH", Table: "t", Method: "btree", Index: "by_k", Field: "k", IndexCount: 1, FieldCount: 1, Recheck: true}
			r.Work = SQLWork{IndexKeysRead: int64(count), IndexPageReads: 1, RowIDSeeks: int64(count), Candidates: int64(count), BaseRowFetches: int64(count), ReturnedRows: int64(count)}
		}
		l.Queries[i] = r
	}
	return l
}

func syntheticNonce(g Generation, role FileRole) NonceObject {
	o := NonceObject{Label: g.Label, DatabaseID: g.DatabaseID, EpochID: g.EpochID, RootSalt: g.RootSalt,
		FileID: g.RootFileID, ObjectHash: g.RootHash, Role: role, NoncePrefix: [4]byte{g.EpochID[0], byte(role)},
		Extent: RootBytes, Count: 1, BoundedParse: true, WholeObjectHashVerified: true}
	if role != FileRoot {
		o.FileID, o.ObjectHash, o.Extent, o.Count = g.BaseFileID, g.BaseHash, BaseBytes, 5
	}
	offset := uint64(512)
	for i := 0; i < int(o.Count); i++ {
		r, plain := RecordMetadata, uint32(64)
		if role == FileRoot {
			plain = 128
		} else if i == 4 {
			r, plain = RecordFooter, 48
		} else if i > 0 {
			r, plain = RecordData, 4096
		}
		o.Records[i] = NonceRecord{Role: r, Offset: offset, Extent: uint64(plain) + 40, Sequence: uint64(i), PlaintextBytes: plain, Nonce: Nonce(o.NoncePrefix, uint64(i))}
		offset += uint64(plain) + 40
	}
	return o
}

func syntheticObjectName(id Hash, suffix string) string {
	const digits = "0123456789abcdef"
	b := make([]byte, 64)
	for i, x := range id {
		b[2*i], b[2*i+1] = digits[x>>4], digits[x&15]
	}
	return "fixture.gse.gse-" + string(b) + suffix
}

func syntheticArtifact(name string, hash Hash, extent uint64) Artifact {
	s := ArtifactStamp{Hash: hash, Bytes: extent, Regular: true, SingleLink: true}
	return Artifact{Name: name, Before: s, After: s}
}

func syntheticManifest(f Family, e Evidence) ArtifactManifest {
	m := ArtifactManifest{Family: f, Complete: true, ParentOwned: true, ReadbackVerified: true, FrozenAfterWriterWait: f != OriginalFamily,
		DestinationInitiallyAbsent: true, ExclusiveCopies: true, DestinationSynced: true, RootName: "fixture.gse"}
	var entries []Artifact
	switch f {
	case OriginalFamily:
		entries = []Artifact{syntheticArtifact("fixture.gse", e.Old.RootHash, RootBytes), syntheticArtifact(syntheticObjectName(e.Old.BaseFileID, ".base"), e.Old.BaseHash, BaseBytes)}
	case ArchiveFamily:
		entries = []Artifact{syntheticArtifact("old-archive.gse", e.Archive.ArchiveHash, BaseBytes)}
	case RestoreFamily:
		g := e.Archive.Restored
		entries = []Artifact{syntheticArtifact("fixture.gse", g.RootHash, RootBytes), syntheticArtifact(syntheticObjectName(g.BaseFileID, ".base"), g.BaseHash, BaseBytes)}
	default:
		selected := e.Candidate
		if e.Binding.Case == BeforeRootRename {
			selected = e.Old
		}
		entries = []Artifact{syntheticArtifact("fixture.gse", selected.RootHash, RootBytes),
			syntheticArtifact(syntheticObjectName(e.Old.RootFileID, ".retained"), e.Old.RootHash, RootBytes),
			syntheticArtifact(syntheticObjectName(e.Old.BaseFileID, ".base"), e.Old.BaseHash, BaseBytes),
			syntheticArtifact(syntheticObjectName(e.Candidate.BaseFileID, ".base"), e.Candidate.BaseHash, BaseBytes)}
		if e.Binding.Case == BeforeRootRename {
			entries = append(entries, syntheticArtifact(syntheticObjectName(e.Candidate.RootFileID, ".pending"), e.Candidate.RootHash, RootBytes))
		}
		if f == PostExitFamily {
			m.Source = TrialFamily
		}
		if f == OldProbeFamily || f == NewProbeFamily {
			m.Source = PostExitFamily
		}
	}
	if f != ArchiveFamily {
		entries = append(entries, syntheticArtifact("fixture.gse.gse-lock", Hash{88}, 0))
	}
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].Name < entries[j-1].Name; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
	m.Count = uint8(len(entries))
	for i, a := range entries {
		a.Before.Identity = FileIdentity{Device: 1, Inode: uint64(f)*1000 + uint64(i) + 1}
		a.After = a.Before
		m.Entries[i] = a
	}
	return m
}

func syntheticEvidence(c Case) Evidence {
	e := Evidence{
		Binding: Binding{Profile: LinuxSnapshotRaw4096, Case: c, Trial: Hash{1}, SourceManifest: Hash{2}, Compiler: Hash{3}, CompileReceipt: Hash{4}, Binary: Hash{5}},
		Paths: Paths{Original: "/case/inputs/original/fixture.gse", Trial: "/case/trial/fixture.gse", ProbeOld: "/case/probe-old/fixture.gse",
			ProbeNew: "/case/probe-new/fixture.gse", Archive: "/case/inputs/old-archive.gse", Restore: "/case/restore-new/fixture.gse"},
		Old: syntheticGeneration(OLD, 10), Candidate: syntheticGeneration(NEW, 20), Filesystem: "synthetic-value-only", LogicalPageSize: 4096,
		Return: ReturnEvidence{Calls: 1, Value: NotReturned}, ArchiveControl: c == Control,
	}
	e.Candidate.ParentCommitID = e.Old.RootHash
	roles, credentials := 4, 5
	if c == Control {
		roles, credentials = 5, 9
	}
	for i := 0; i < roles; i++ {
		r := Role(i + 1)
		e.Roles[i] = RoleBinding{Role: r, Generation: uint64(i + 1), Challenge: Hash{byte(70 + i)}}
		o := OwnerReceipt{Role: r, Generation: uint64(i + 1), Owner: 1000 + uint64(r), AfterKnown: true, DBCloseCalls: 1}
		if r == Writer && c != Control {
			o.AfterKnown, o.DBCloseCalls = false, 0
		}
		if (r == OldProbe && c != BeforeRootRename) || (r == NewProbe && c == BeforeRootRename) {
			o.DBCloseCalls = 0
		}
		e.Owners = append(e.Owners, o)
		if r != Controller {
			p := ProcessReceipt{Role: r, Generation: uint64(i + 1), Challenge: e.Roles[i].Challenge, ParentHandle: uint64(i), Starts: 1, Waits: 1,
				ParentOwned: true, ActualSelfImage: true, NaturalExit: true, TerminalEOF: true, Image: FileIdentity{Device: 1, Inode: 9000}, ImageHash: e.Binding.Binary, State: ProcessExited}
			if r == Writer && c != Control {
				p.ExitCode, p.NaturalExit = ExpectedExitCode, false
			}
			e.Processes = append(e.Processes, p)
		}
	}
	for i := 0; i < credentials; i++ {
		s, _ := CredentialFor(e.Paths, Purpose(i+1))
		e.Credentials = append(e.Credentials, CredentialUse{Spec: s, Provider: uint64(i + 1), Acquires: 1, Releases: 1,
			Cloned: true, Cleared: true, ImmutablePurpose: true, IndependentlyAdmittedPath: true})
	}
	var returned Frame
	observation, _ := ObservationPayload(e.Old, e.Candidate)
	for i, event := range ExpectedTrace(c) {
		role := eventRole(event)
		r := e.Roles[int(role)-1]
		f := Frame{Binding: e.Binding, Role: role, Generation: r.Generation, Challenge: r.Challenge, Sequence: uint16(i + 1), Event: event, Payload: Hash{byte(40 + i)}}
		if event == ObservedBeforeRename || event == ObservedAfterRename || event == ObservedAfterDirSync || event == PublicRekeyNil {
			f.Payload = observation
		}
		if event == PublicRekeyNil {
			returned = f
		}
		if event == AcknowledgementDurable {
			h, _ := FrameHash(returned)
			f.Payload = h
			e.ACK = ACK{Present: true, ParentOwned: true, Exclusive: true, ReadbackVerified: true, ReturnSequence: returned.Sequence, ACKSequence: f.Sequence,
				OriginalReturnFrame: h, WrittenHash: h, ReadbackHash: h, File: FileIdentity{Device: 1, Inode: 9001}, Directory: FileIdentity{Device: 1, Inode: 9002},
				Bytes: 4 + FrameBodyBytes, Written: 4 + FrameBodyBytes, WriteStep: 1, FileSyncStep: 2, FileCloseStep: 3, DirectorySyncStep: 4, DirectoryCloseStep: 5,
				ReadbackStep: 6, FileCloseCalls: 1, DirectoryCloseCalls: 1}
		}
		e.Frames = append(e.Frames, f)
	}
	if c == Control || c == RekeyReturnedNoAck || c == AcknowledgedExit {
		e.Return = ReturnEvidence{Calls: 1, Value: ReturnedNil, CapturedAdjacent: true, CapturedUnchanged: true}
	}
	e.Baseline = syntheticSQL(Controller, e.Roles[0].Generation, e.Owners[0].Owner)
	selected := e.Candidate
	if c == BeforeRootRename {
		selected = e.Old
	}
	for i := range e.Probes {
		role, purpose, path, label := OldProbe, OldProbeOpen, e.Paths.ProbeOld, OLD
		if i == 1 {
			role, purpose, path, label = NewProbe, NewProbeOpen, e.Paths.ProbeNew, NEW
		}
		p := ProbeEvidence{Role: role, Generation: e.Roles[int(role)-1].Generation, Purpose: purpose, Path: path, OpenCalls: 1, ReadOnly: true, NoCreate: true, NilDB: true,
			WrongKey: WrongKeyEvidence{OriginalError: true, SQLiteError: true, Authentication: true, Operation: "open", PublicCode: 26}}
		if label == selected.Label {
			p.Authenticated, p.NilDB, p.VerifyCalls, p.Selected, p.WrongKey = true, false, 1, selected, WrongKeyEvidence{}
			p.Ledger = syntheticSQL(role, p.Generation, e.Owners[int(role)-1].Owner)
		}
		e.Probes[i] = p
	}
	e.Nonces = []NonceObject{syntheticNonce(e.Old, FileRoot), syntheticNonce(e.Old, FileBase), syntheticNonce(e.Candidate, FileRoot), syntheticNonce(e.Candidate, FileBase)}
	if c == Control {
		restored := syntheticGeneration(NEW, 40)
		restored.DatabaseID, restored.Sequence = [16]byte{3}, 1
		e.Archive = ArchiveEvidence{Present: true, MadeBeforeRekey: true, RestoreDestinationAbsent: true, SourceUnchanged: true,
			ArchiveDatabaseID: [16]byte{2}, ArchiveEpochID: Hash{30}, ArchiveRootSalt: Hash{31}, ArchiveFileID: Hash{32}, ArchiveHash: Hash{33},
			Restored: restored, Ledger: syntheticSQL(ArchiveCheck, 5, e.Owners[4].Owner), BackupCalls: 1, RestoreCalls: 1, ReopenCalls: 1, VerifyCalls: 1, CloseCalls: 1, ReadOnly: true, NoCreate: true}
		archive := syntheticGeneration(OLD, 30)
		archive.DatabaseID, archive.BaseFileID, archive.BaseHash = [16]byte{2}, Hash{32}, Hash{33}
		e.Nonces = append(e.Nonces, syntheticNonce(archive, FileBackup), syntheticNonce(restored, FileRoot), syntheticNonce(restored, FileBase))
	}
	families := 5
	if c == Control {
		families = 7
	}
	for i := 0; i < families; i++ {
		e.Artifacts = append(e.Artifacts, syntheticManifest(Family(i+1), e))
	}
	e.SeedCopy = SeedCopyEvidence{Complete: true, Exclusive: true, ReadbackVerified: true, DestinationAbsent: true, DestinationSynced: true, Count: e.Artifacts[0].Count}
	for i := 0; i < int(e.SeedCopy.Count); i++ {
		a := e.Artifacts[0].Entries[i]
		a.Before.Identity = FileIdentity{Device: 1, Inode: 8000 + uint64(i)}
		a.After = a.Before
		e.SeedCopy.Entries[i] = a
	}
	return e
}

func syntheticControlWithoutArchive() Evidence {
	e := syntheticEvidence(Control)
	e.ArchiveControl, e.Archive, e.Roles[4] = false, ArchiveEvidence{}, RoleBinding{}
	e.Credentials, e.Processes, e.Owners = e.Credentials[:5], e.Processes[:3], e.Owners[:4]
	e.Artifacts, e.Nonces = e.Artifacts[:5], e.Nonces[:4]
	var frames []Frame
	for _, frame := range e.Frames {
		if frame.Event >= ArchiveStarted && frame.Event <= ArchiveWaited {
			continue
		}
		frame.Sequence = uint16(len(frames) + 1)
		frames = append(frames, frame)
	}
	e.Frames = frames
	return e
}

func TestRekeyInterruptionSixCases(t *testing.T) {
	cases := Cases()
	if len(cases) != 6 || ProfileID != "linux-layout1-rawkey256-4096" {
		t.Fatal("frozen inventory changed")
	}
	for _, c := range cases {
		if got, ok := ParseCase(CaseName(c)); !ok || got != c {
			t.Fatal("case round trip", c)
		}
		e := syntheticEvidence(c)
		if r := Validate(e); !r.EvidenceConsistent || r.Problem != NoProblem {
			t.Fatal("invented consistent values rejected", c, r)
		}
		if c >= BeforeRootRename && c <= AfterRootDirSyncBeforeAdoption {
			if e.Return.Value != NotReturned {
				t.Fatal("exit invented an error or return")
			}
		} else if e.Return.Value != ReturnedNil {
			t.Fatal("actual nil return omitted")
		}
		if e.ACK.Present != (c == AcknowledgedExit) {
			t.Fatal("ACK distinction lost")
		}
	}
	withoutArchive := syntheticControlWithoutArchive()
	if result := Validate(withoutArchive); !result.EvidenceConsistent {
		t.Fatal("control without optional archive rejected", result)
	}
	withoutArchive.ArchiveControl = true
	if Validate(withoutArchive).EvidenceConsistent {
		t.Fatal("missing archive companion accepted")
	}
	misplacedArchive := syntheticEvidence(BeforeRootRename)
	misplacedArchive.ArchiveControl = true
	if Validate(misplacedArchive).EvidenceConsistent {
		t.Fatal("archive outside control accepted")
	}
	if _, ok := ParseCase("power-loss"); ok {
		t.Fatal("extra case")
	}
	e := syntheticEvidence(Control)
	e.Binding.Profile = Profile(2)
	if Validate(e).EvidenceConsistent {
		t.Fatal("extra profile admitted")
	}
}

func TestRekeyInterruptionCutOrder(t *testing.T) {
	for n := 0; n < 8; n++ {
		e := syntheticEvidence(Control)
		switch n {
		case 0:
			e.Frames = append(e.Frames[:5], e.Frames[6:]...)
		case 1:
			e.Frames[6].Event = ObservedBeforeRename
		case 2:
			e.Frames[5].Event, e.Frames[6].Event = e.Frames[6].Event, e.Frames[5].Event
		case 3:
			e.Frames[7].Event = ObservedAfterRename
		case 4:
			e.Frames[6].Event = EventUnknown
		case 5:
			e.Frames[5].Sequence = 64
		case 6:
			e.Return.CapturedAdjacent = false
		case 7:
			e.Return.Calls = 2
		}
		if Validate(e).EvidenceConsistent {
			t.Fatal("bad cut trace accepted", n)
		}
	}
	e := syntheticEvidence(AfterRootRenameBeforeDirSync)
	e.Return = ReturnEvidence{Calls: 1, Value: ReturnedNil, CapturedAdjacent: true, CapturedUnchanged: true}
	if Validate(e).EvidenceConsistent {
		t.Fatal("rename success substituted for public success")
	}
	for _, f := range syntheticEvidence(Control).Frames {
		b, p := EncodeFrame(f)
		if p != NoProblem {
			t.Fatal(p)
		}
		out, p := DecodeFrame(b)
		if p != NoProblem || out != f {
			t.Fatal("frame round trip")
		}
		for _, size := range []int{0, 1, 3, 4, 127, 259} {
			if _, p := DecodeFrame(b[:size]); p == NoProblem {
				t.Fatal("truncated frame")
			}
		}
		if _, p := DecodeFrame(append(b, 0)); p == NoProblem {
			t.Fatal("trailing field")
		}
		b[4+244] = 1
		if _, p := DecodeFrame(b); p == NoProblem {
			t.Fatal("unknown reserved field")
		}
		b, _ = EncodeFrame(f)
		b[3] = 255
		if _, p := DecodeFrame(b); p == NoProblem {
			t.Fatal("noncanonical length")
		}
	}
	transport := TransportFrame{Binding: TransportBinding{Trial: "public-trial", Source: "source-receipt", Binary: "binary-receipt", Profile: ProfileID,
		Case: "control", Role: "writer", Generation: "writer-1", Challenge: "public-challenge"}, Sequence: 1, Kind: "publication", Point: 1,
		Observation: TransportObservation{OldHash: Hash{1}, CandidateHash: Hash{2}, OldBorrow: TransportBorrow{Acquires: 1, Releases: 1}, NewBorrow: TransportBorrow{Acquires: 1, Releases: 1}}}
	raw, p := EncodeTransportFrame(transport)
	if p != NoProblem {
		t.Fatal("native transport-shaped value", p)
	}
	if got, p := DecodeTransportFrame(raw); p != NoProblem || got != transport {
		t.Fatal("transport canonical round trip")
	}
	body := string(raw[4:])
	for _, bad := range []string{body + " ", body + body, body[:len(body)-1] + ",\"Unknown\":1}", body[:len(body)-1] + ",\"Sequence\":1}", body[1:]} {
		encoded := make([]byte, 4+len(bad))
		binary.BigEndian.PutUint32(encoded[:4], uint32(len(bad)))
		copy(encoded[4:], bad)
		if _, p := DecodeTransportFrame(encoded); p == NoProblem {
			t.Fatal("unknown/duplicate/trailing JSON accepted")
		}
	}
	for _, n := range []int{0, 1, 3, 4, len(raw) - 1} {
		if _, p := DecodeTransportFrame(raw[:n]); p == NoProblem {
			t.Fatal("transport truncated prefix/body")
		}
	}
	transport.Sequence = 65
	if _, p := EncodeTransportFrame(transport); p == NoProblem {
		t.Fatal("transport frame count exceeded")
	}
}

func TestRekeyInterruptionCredentialPurpose(t *testing.T) {
	for n := 0; n < 11; n++ {
		e := syntheticEvidence(Control)
		u := &e.Credentials[2]
		switch n {
		case 0:
			u.Spec.Label = OLD
		case 1:
			u.Spec.Purpose = OldWriter
		case 2:
			u.Spec.Operation = OperationOpen
		case 3:
			u.Spec.Path = e.Paths.ProbeNew
		case 4:
			u.Spec.Role = Controller
		case 5:
			u.PurposeFromUntrustedHeader = true
		case 6:
			u.Releases = 0
		case 7:
			u.DuplicateRelease = true
		case 8:
			u.Cloned = false
		case 9:
			u.Cleared = false
		case 10:
			u.Provider = e.Credentials[1].Provider
		}
		if Validate(e).EvidenceConsistent {
			t.Fatal("purpose substitution accepted", n)
		}
	}
	for _, path := range []string{"relative", "/x/../y", "/x//y", "/x/./y", "/x/", "/x\\y"} {
		paths := syntheticEvidence(Control).Paths
		paths.Trial = path
		if _, p := CredentialFor(paths, NewRekeyTarget); p == NoProblem {
			t.Fatal("inadmissible path", path)
		}
	}
	e := syntheticEvidence(Control)
	e.Old.Authenticated = false
	if Validate(e).EvidenceConsistent {
		t.Fatal("header admission treated as authentication")
	}
}

func TestRekeyInterruptionSelectedGeneration(t *testing.T) {
	for n := 0; n < 18; n++ {
		e := syntheticEvidence(RekeyReturnedNoAck)
		switch n {
		case 0:
			e.Probes[1].Selected = e.Old
		case 1:
			e.Candidate.DatabaseID[0]++
		case 2:
			e.Candidate.ParentCommitID = Hash{}
		case 3:
			e.Candidate.Sequence = e.Old.Sequence
		case 4:
			e.Candidate.EpochID = e.Old.EpochID
		case 5:
			e.Candidate.RootSalt = e.Old.RootSalt
		case 6:
			e.Candidate.RootFileID = e.Old.BaseFileID
		case 7:
			e.Candidate.LogicalPages[1] = Hash{255}
		case 8:
			e.Candidate.PageOneCounters[0]++
		case 9:
			e.Probes[0].Authenticated = true
		case 10:
			e.Probes[1].Authenticated = false
		case 11:
			e.Probes[0].WrongKey.PublicCode = 10
		case 12:
			e.Probes[0].WrongKey.OtherCauses = 1
		case 13:
			e.Probes[0].NilDB = false
		case 14:
			e.Probes[0].RetainedFallback = true
		case 15:
			e.Probes[0].VerifyCalls = 1
		case 16:
			e.Probes[1].OpenCalls = 2
		case 17:
			e.Probes[1].NoCreate = false
		}
		if Validate(e).EvidenceConsistent {
			t.Fatal("bad selected generation accepted", n)
		}
	}
	e := syntheticEvidence(BeforeRootRename)
	e.Probes[0].Selected = e.Candidate
	if Validate(e).EvidenceConsistent {
		t.Fatal("pre-rename candidate used as authority")
	}
}

func TestRekeyInterruptionSQLIndexLedger(t *testing.T) {
	for n := 0; n < 21; n++ {
		l := syntheticSQL(NewProbe, 4, 1004)
		switch n {
		case 0:
			l.Queries[0].Rows[0].IDType = SQLText
		case 1:
			l.Queries[0].Rows[0].Value = "ONE"
		case 2:
			l.Queries[0].RowCount = 2
		case 3:
			l.Queries[5].Rows[0] = SQLRows()[0]
		case 4:
			l.Queries[1].Plan.Index = "stale"
		case 5:
			l.Queries[1].Plan.IndexCount = 0
		case 6:
			l.Queries[1].Plan.Recheck = false
		case 7:
			l.Queries[5].Work.IndexPageReads = 0
		case 8:
			l.Queries[3].Work.Candidates = 1
		case 9:
			l.Queries[2].Work.BaseRowFetches = 1
		case 10:
			l.Queries[0].EOF = false
		case 11:
			l.Queries[0].ErrCalled = false
		case 12:
			l.Queries[0].CloseCalls = 0
		case 13:
			l.Queries[0].CloseCalls = 2
		case 14:
			l.Queries[0].CloseErrorToken = 1
		case 15:
			l.Queries[1].Iterator = l.Queries[0].Iterator
		case 16:
			l.Check.Scope = "full-database"
		case 17:
			l.Check.PageReads = 0
		case 18:
			l.Check.PageReads = 513
		case 19:
			l.Check.IndexEntries = 2
		case 20:
			l.Check.ExtensionIndexes = 1
		}
		if ValidateSQLLedger(l) == NoProblem {
			t.Fatal("weak SQL ledger accepted", n)
		}
	}
	if SQLRows() != ([3]SQLRow{{SQLInteger, 1, SQLText, "red", SQLText, "one"}, {SQLInteger, 2, SQLText, "blue", SQLText, "two"}, {SQLInteger, 3, SQLText, "red", SQLText, "three"}}) {
		t.Fatal("literal ledger changed")
	}
}

func TestRekeyInterruptionAckCausality(t *testing.T) {
	for n := 0; n < 16; n++ {
		e := syntheticEvidence(AcknowledgedExit)
		switch n {
		case 0:
			e.ACK.ACKSequence = e.ACK.ReturnSequence - 1
		case 1:
			e.ACK.OriginalReturnFrame[0] ^= 1
		case 2:
			e.ACK.Written--
		case 3:
			e.ACK.ErrorToken = 1
		case 4:
			e.ACK.FileSyncStep = 0
		case 5:
			e.ACK.FileCloseCalls = 0
		case 6:
			e.ACK.DirectorySyncStep = 0
		case 7:
			e.ACK.DirectoryCloseCalls = 2
		case 8:
			e.ACK.File = e.Artifacts[0].Entries[0].Before.Identity
		case 9:
			e.ACK.ParentOwned = false
		case 10:
			e.ACK.ReadbackHash[0] ^= 1
		case 11:
			e.ACK.Exclusive = false
		case 12:
			e.Frames[e.ACK.ReturnSequence-1].Payload[0] ^= 1
		case 13:
			e.Frames[e.ACK.ACKSequence-1].Role = Writer
		case 14:
			e.ACK.Present = false
		case 15:
			e.ACK.ReadbackStep = 4
		}
		if Validate(e).EvidenceConsistent {
			t.Fatal("noncausal ACK accepted", n)
		}
	}
	e := syntheticEvidence(RekeyReturnedNoAck)
	e.ACK = syntheticEvidence(AcknowledgedExit).ACK
	if Validate(e).EvidenceConsistent {
		t.Fatal("no-ACK row silently acknowledged")
	}
}

func TestRekeyInterruptionOwnersAndArtifacts(t *testing.T) {
	for n := 0; n < 29; n++ {
		e := syntheticEvidence(Control)
		switch n {
		case 0:
			e.Processes[0].Starts = 2
		case 1:
			e.Processes[0].Waits = 0
		case 2:
			e.Processes[0].State = ChildLive
		case 3:
			e.Processes[0].State = WaitUnknown
		case 4:
			e.Processes[0].TimedOut = true
		case 5:
			e.Processes[0].SignalNumber = 9
		case 6:
			e.Processes[0].Challenge[0] ^= 1
		case 7:
			e.Processes[0].ActualSelfImage = false
		case 8:
			e.Owners[0].UnknownClose = true
		case 9:
			e.Owners[0].After.PendingTokens = 1
		case 10:
			e.Owners[0].DBCloseCalls = 2
		case 11:
			e.Artifacts[2].Entries[0].Before.Identity = e.Artifacts[1].Entries[0].Before.Identity
			e.Artifacts[2].Entries[0].After = e.Artifacts[2].Entries[0].Before
		case 12:
			e.Artifacts[0].Entries[0].After.Hash[0] ^= 1
		case 13:
			e.Artifacts[2].Entries[0].Before.SingleLink = false
		case 14:
			e.Artifacts[2].Complete = false
		case 15:
			e.Artifacts[2].FrozenAfterWriterWait = false
		case 16:
			e.Archive.RestoreDestinationAbsent = false
		case 17:
			e.Archive.Restored.DatabaseID = e.Old.DatabaseID
		case 18:
			e.Artifacts[6].DestinationInitiallyAbsent = false
		case 19:
			e.Nonces[2].FileID = e.Nonces[0].FileID
		case 20:
			e.Nonces[3].Records[2].Sequence = 1
		case 21:
			e.Nonces[3].Records[2].Nonce[0] ^= 1
		case 22:
			e.Nonces[3].Records[2].Offset++
		case 23:
			e.Nonces[3].ObjectHash = e.Nonces[1].ObjectHash
		case 24:
			e.SeedCopy.Entries[0].Before.Identity = e.Artifacts[0].Entries[0].Before.Identity
			e.SeedCopy.Entries[0].After = e.SeedCopy.Entries[0].Before
		case 25:
			e.SeedCopy.Complete = false
		case 26:
			e.Probes[1].Ledger.Role = RoleUnknown
		case 27:
			e.SeedCopy.Entries[0].After.Hash[0] ^= 1
		case 28:
			e.Processes[0].TerminalEOF = false
		}
		if Validate(e).EvidenceConsistent {
			t.Fatal("lost ownership or bytes accepted", n)
		}
	}
	for _, c := range []Case{BeforeRootRename, AfterRootRenameBeforeDirSync, AfterRootDirSyncBeforeAdoption, RekeyReturnedNoAck, AcknowledgedExit} {
		e := syntheticEvidence(c)
		e.Processes[0].ExitCode = 0
		if Validate(e).EvidenceConsistent {
			t.Fatal("natural exit substituted for grant", c)
		}
		e = syntheticEvidence(c)
		e.Owners[1].AfterKnown = true
		if Validate(e).EvidenceConsistent {
			t.Fatal("exit asserted graceful retirement", c)
		}
		e = syntheticEvidence(c)
		e.Owners[1].DeferredCleanup = true
		if Validate(e).EvidenceConsistent {
			t.Fatal("interruption ran cleanup", c)
		}
	}
	e := syntheticEvidence(Control)
	e.Frames[3].Generation++
	if Validate(e).EvidenceConsistent {
		t.Fatal("stale role generation")
	}
}

func TestRekeyInterruptionNeverPhysicalPass(t *testing.T) {
	for _, c := range Cases() {
		e := syntheticEvidence(c)
		r := Validate(e)
		if !r.EvidenceConsistent || r.PhysicalValidated() || r.Pass() {
			t.Fatal("value consistency became physical truth", c, r)
		}
		e.PhysicalClaim = true
		if r := Validate(e); r.EvidenceConsistent || r.Problem != ProblemPhysicalClaim {
			t.Fatal("physical claim admitted")
		}
		e.PhysicalClaim, e.PowerLossClaim = false, true
		if Validate(e).EvidenceConsistent {
			t.Fatal("process exit became power-loss evidence")
		}
		e.PowerLossClaim, e.HardwareSectorClaim = false, true
		if Validate(e).EvidenceConsistent {
			t.Fatal("page size became sector size")
		}
	}
	if (Result{EvidenceConsistent: true}).Pass() || (Result{EvidenceConsistent: true}).PhysicalValidated() {
		t.Fatal("exported result fields can manufacture PASS")
	}
}
