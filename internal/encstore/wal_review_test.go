package encstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"strconv"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

// All malformed/authentication fixtures in this file are memory-only. No test
// opens a disk database with wrong credentials or changes a selected disk file.
func walReviewRef(e encryption.Epoch, role encryption.FileRole, count uint32, frames, seq uint64, parent [32]byte, id byte) objectRef {
	d := encryption.FileDescriptor{Role: role, PageSize: 512, PageCount: count, RecordCount: uint64(count) + 2, CommitSequence: seq, TransactionID: [32]byte{id, 1}, ParentCommitID: parent}
	if role == encryption.FileWAL {
		d.RecordCount = frames + 2
	}
	r := objectRef{Descriptor: d, FileID: [32]byte{id, 2}, Hash: [32]byte{id, 3}, PageOneHash: [32]byte{id, 4}, Bytes: objectSize(Geometry{512, count})}
	if role == encryption.FileWAL {
		r.Bytes = segmentSize(512, frames)
	}
	r.CommitID = r.commitID(e)
	return r
}
func walReviewManifest(t *testing.T, master *encryption.MasterKey) (walManifest, encryption.Header) {
	t.Helper()
	e, err := master.Epoch()
	if err != nil {
		t.Fatal(err)
	}
	b := walReviewRef(e, encryption.FileBase, 4, 0, 7, [32]byte{91}, 1)
	a := walReviewRef(e, encryption.FileWAL, 6, 3, 8, b.CommitID, 2)
	z := walReviewRef(e, encryption.FileWAL, 6, 1, 9, a.CommitID, 3)
	m := walManifest{Base: b, Segments: []objectRef{a, z}, Frames: 4, Bytes: a.Bytes + z.Bytes, Head: z.CommitID, PageOneHash: z.PageOneHash, PreviousRootHash: [32]byte{92}}
	d := z.Descriptor
	d.Role = encryption.FileRoot
	d.RecordCount = 1
	return m, encryption.Header{Epoch: e, Descriptor: d}
}
func walReviewSealRoot(t *testing.T, master *encryption.MasterKey, d encryption.FileDescriptor, payload []byte) []byte {
	t.Helper()
	w, err := encryption.NewFileWriter(master, d, encryption.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	h, err := w.Header()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := encryption.EncodeHeader(h, encryption.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := w.Seal(encryption.RecordMetadata, 0, 0, payload)
	if err != nil {
		t.Fatal(err)
	}
	if err = w.Finish(); err != nil {
		t.Fatal(err)
	}
	return append(raw, b...)
}
func TestReviewWALMemoryRootAuthenticationAndExactGraph(t *testing.T) {
	master := reviewMemoryMaster(t)
	m, h := walReviewManifest(t, master)
	o := Options{Mode: WAL}
	raw := walReviewSealRoot(t, master, h.Descriptor, m.encode())
	if err := verifyRootBytes(raw, master, o); err != nil {
		t.Fatal("valid explicit base/head graph", err)
	}
	if m.Base.Descriptor.PageCount == h.Descriptor.PageCount || m.Base.Descriptor.CommitSequence == h.Descriptor.CommitSequence {
		t.Fatal("fixture failed to distinguish base from head")
	}
	for i := range raw {
		b := bytes.Clone(raw)
		b[i] ^= 1
		if err := verifyRootBytes(b, master, o); err == nil {
			t.Fatalf("changed root byte %d accepted", i)
		}
	}
	for _, n := range []int{0, 511, 512, 679, 680, len(raw) - 1} {
		if err := verifyRootBytes(raw[:n], master, o); err == nil {
			t.Fatalf("root length %d accepted", n)
		}
	}
	if err := verifyRootBytes(append(bytes.Clone(raw), 0), master, o); err == nil {
		t.Fatal("trailing root byte accepted")
	}
	cases := map[string]func(*walManifest, *encryption.Header){
		"prefix": func(m *walManifest, h *encryption.Header) {
			m.Segments = m.Segments[:1]
			m.Frames = 3
			m.Bytes = m.Segments[0].Bytes
		},
		"reordered": func(m *walManifest, h *encryption.Header) {
			m.Segments[0], m.Segments[1] = m.Segments[1], m.Segments[0]
		},
		"duplicate file": func(m *walManifest, h *encryption.Header) {
			m.Segments[0].FileID = m.Base.FileID
			m.Segments[0].CommitID = m.Segments[0].commitID(h.Epoch)
		},
		"wrong parent": func(m *walManifest, h *encryption.Header) {
			m.Segments[0].Descriptor.ParentCommitID[0] ^= 1
			m.Segments[0].CommitID = m.Segments[0].commitID(h.Epoch)
		},
		"sequence gap": func(m *walManifest, h *encryption.Header) {
			m.Segments[0].Descriptor.CommitSequence++
			m.Segments[0].CommitID = m.Segments[0].commitID(h.Epoch)
		},
		"base from head": func(m *walManifest, h *encryption.Header) {
			m.Base.Descriptor.PageCount = h.Descriptor.PageCount
			m.Base.CommitID = m.Base.commitID(h.Epoch)
		},
		"wrong total frames":    func(m *walManifest, h *encryption.Header) { m.Frames++ },
		"wrong total bytes":     func(m *walManifest, h *encryption.Header) { m.Bytes++ },
		"wrong head":            func(m *walManifest, h *encryption.Header) { m.Head[0] ^= 1 },
		"wrong head page one":   func(m *walManifest, h *encryption.Header) { m.PageOneHash[0] ^= 1 },
		"wrong head descriptor": func(m *walManifest, h *encryption.Header) { h.Descriptor.TransactionID[0] ^= 1 },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			m, h := walReviewManifest(t, master)
			change(&m, &h)
			raw := walReviewSealRoot(t, master, h.Descriptor, m.encode())
			if err := verifyRootBytes(raw, master, o); err == nil {
				t.Fatal("authenticated invalid graph accepted")
			}
		})
	}
	for _, off := range []int{8, 10, 12, 16, 40, 144, 176, 256 + 1, 256 + 2, 256 + 12, 256 + 232} {
		t.Run("canonical-byte-"+strconv.Itoa(off), func(t *testing.T) {
			b := m.encode()
			b[off] ^= 1
			raw := walReviewSealRoot(t, master, h.Descriptor, b)
			if err := verifyRootBytes(raw, master, o); err == nil {
				t.Fatalf("authenticated noncanonical metadata byte %d accepted", off)
			}
		})
	}
}
func TestReviewWALMemoryManifestCreationAncestry(t *testing.T) {
	master := reviewMemoryMaster(t)
	e, _ := master.Epoch()
	o := Options{Mode: WAL}
	for _, seq := range []uint64{1, 7} {
		parent := [32]byte{}
		previous := [32]byte{}
		if seq > 1 {
			parent[0] = 8
			previous[0] = 9
		}
		b := walReviewRef(e, encryption.FileBase, 4, 0, seq, parent, 1)
		m := walManifest{Base: b, Head: b.CommitID, PageOneHash: b.PageOneHash, PreviousRootHash: previous}
		h := encryption.Header{Epoch: e, Descriptor: b.Descriptor}
		h.Descriptor.Role = encryption.FileRoot
		h.Descriptor.RecordCount = 1
		if _, err := decodeWALManifest(m.encode(), h, o); err != nil {
			t.Fatal("valid base ancestry", seq, err)
		}
		wrong := m
		wrong.PreviousRootHash = [32]byte{}
		if seq == 1 {
			wrong.PreviousRootHash[0] = 1
		}
		if _, err := decodeWALManifest(wrong.encode(), h, o); err == nil {
			t.Errorf("sequence %d accepted noncanonical previous-root receipt", seq)
		}
		wrong = m
		wrong.Base.Descriptor.ParentCommitID = [32]byte{}
		if seq == 1 {
			wrong.Base.Descriptor.ParentCommitID[0] = 1
		}
		wrong.Base.CommitID = wrong.Base.commitID(e)
		wrong.Head = wrong.Base.CommitID
		hh := h
		hh.Descriptor.ParentCommitID = wrong.Base.Descriptor.ParentCommitID
		if _, err := decodeWALManifest(wrong.encode(), hh, o); err == nil {
			t.Errorf("sequence %d accepted noncanonical base ancestry", seq)
		}
	}
}

func walReviewSegmentBytes(t *testing.T, master *encryption.MasterKey, mutateMeta, mutateFooter func([]byte)) ([]byte, objectRef, encryption.Epoch) {
	t.Helper()
	e, err := master.Epoch()
	if err != nil {
		t.Fatal(err)
	}
	d := encryption.FileDescriptor{Role: encryption.FileWAL, PageSize: 512, PageCount: 6, RecordCount: 5, CommitSequence: 8, TransactionID: [32]byte{3}, ParentCommitID: [32]byte{4}}
	pages := []uint32{1, 5, 6}
	p1 := sha256.Sum256(bytes.Repeat([]byte{1}, 512))
	w, err := encryption.NewFileWriter(master, d, encryption.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	h, _ := w.Header()
	raw, err := encryption.EncodeHeader(h, encryption.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	r := objectRef{Descriptor: d, FileID: h.FileID, Bytes: segmentSize(512, 3), PageOneHash: p1}
	meta := segmentMetadata(d, 4, p1, pages)
	if mutateMeta != nil {
		mutateMeta(meta)
	}
	mh := sha256.Sum256(meta)
	b, err := w.Seal(encryption.RecordMetadata, 0, 0, meta)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, b...)
	for i, n := range pages {
		b, err = w.Seal(encryption.RecordData, n, uint64(i+1), bytes.Repeat([]byte{byte(n)}, 512))
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, b...)
	}
	f := segmentFooter(r, 4, sha256.Sum256(raw), mh)
	if mutateFooter != nil {
		mutateFooter(f)
	}
	b, err = w.Seal(encryption.RecordFooter, 0, 0, f)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, b...)
	if err = w.Finish(); err != nil {
		t.Fatal(err)
	}
	r.Hash = sha256.Sum256(raw)
	r.CommitID = r.commitID(e)
	return raw, r, e
}

type walReviewReader struct {
	io.ReaderAt
	reads int
}

func (r *walReviewReader) ReadAt(b []byte, off int64) (int, error) {
	r.reads++
	return r.ReaderAt.ReadAt(b, off)
}
func TestReviewWALMemorySegmentAuthenticationAndCompleteness(t *testing.T) {
	master := reviewMemoryMaster(t)
	raw, r, epoch := walReviewSegmentBytes(t, master, nil, nil)
	o := Options{Mode: WAL}
	ctx := context.Background()
	check := func(b []byte, extent uint64, ref objectRef, previous uint32) error {
		s, err := verifySegment(ctx, bytes.NewReader(b), extent, master, epoch, o, ref, previous)
		if s != nil {
			s.close()
		}
		return err
	}
	if err := check(raw, uint64(len(raw)), r, 4); err != nil {
		t.Fatal("valid memory segment", err)
	}
	for i := range raw {
		b := bytes.Clone(raw)
		b[i] ^= 1
		if err := check(b, uint64(len(b)), r, 4); err == nil {
			t.Fatalf("changed segment byte %d accepted", i)
		}
	}
	for _, n := range []int{0, 511, 512, 679, 684, len(raw) - 249, len(raw) - 1} {
		if err := check(raw[:n], uint64(n), r, 4); err == nil {
			t.Fatalf("truncated extent %d accepted", n)
		}
		if err := check(raw[:n], uint64(len(raw)), r, 4); err == nil {
			t.Fatalf("incomplete ReaderAt %d accepted", n)
		}
	}
	if err := check(append(bytes.Clone(raw), 0), uint64(len(raw)+1), r, 4); err == nil {
		t.Fatal("extra segment extent accepted")
	}
	if err := check(raw, uint64(len(raw)), r, 3); err == nil {
		t.Fatal("wrong predecessor extent accepted")
	}
	swapped := bytes.Clone(raw)
	a := int(frameOffset(512, 3, 2))
	b := int(frameOffset(512, 3, 3))
	size := 512 + recordOverhead
	copy(swapped[a:a+size], raw[b:b+size])
	copy(swapped[b:b+size], raw[a:a+size])
	if err := check(swapped, uint64(len(raw)), r, 4); err == nil {
		t.Fatal("reordered frames accepted")
	}
	changed := bytes.Clone(raw)
	changed[encryption.HeaderSize+recordOverhead+128+12-1] ^= 1
	reader := &walReviewReader{ReaderAt: bytes.NewReader(changed)}
	s, err := verifySegment(ctx, reader, uint64(len(raw)), master, epoch, o, r, 4)
	if s != nil || !errors.Is(err, encryption.ErrAuthentication) || reader.reads != 2 {
		t.Fatalf("unauthenticated metadata advanced into frame work: segment=%v error=%v reads=%d", s != nil, err, reader.reads)
	}
	for _, off := range []int{0, 8, 12, 16, 20, 24, 32, 64, 96, 128, 132, 136} {
		raw, r, epoch := walReviewSegmentBytes(t, master, func(b []byte) { b[off] ^= 1 }, nil)
		s, err := verifySegment(ctx, bytes.NewReader(raw), uint64(len(raw)), master, epoch, o, r, 4)
		if s != nil {
			s.close()
		}
		if err == nil {
			t.Errorf("authenticated invalid metadata byte %d accepted", off)
		}
	}
	for _, off := range []int{0, 8, 40, 72, 104, 112, 120, 124, 128, 136, 168, 200, 207} {
		raw, r, epoch := walReviewSegmentBytes(t, master, nil, func(b []byte) { b[off] ^= 1 })
		s, err := verifySegment(ctx, bytes.NewReader(raw), uint64(len(raw)), master, epoch, o, r, 4)
		if s != nil {
			s.close()
		}
		if err == nil {
			t.Errorf("authenticated invalid footer byte %d accepted", off)
		}
	}
}
func TestReviewWALMemoryExtentAndStalePageModel(t *testing.T) {
	for _, tc := range []struct {
		pages      []uint32
		old, final uint32
		valid      bool
	}{
		{[]uint32{1, 5, 6}, 4, 6, true}, {[]uint32{1, 6}, 4, 6, false}, {[]uint32{1, 5, 5}, 4, 6, false}, {[]uint32{1, 6, 5}, 4, 6, false}, {[]uint32{1}, 6, 3, true}, {[]uint32{2}, 6, 6, false}, {[]uint32{1, 7}, 6, 6, false},
	} {
		err := validateDirectory(tc.pages, tc.old, tc.final)
		if (err == nil) != tc.valid {
			t.Fatalf("directory %v extent %d -> %d: %v", tc.pages, tc.old, tc.final, err)
		}
	}
	master := reviewMemoryMaster(t)
	e, _ := master.Epoch()
	m := walManifest{Base: walReviewRef(e, encryption.FileBase, 6, 0, 7, [32]byte{1}, 1)}
	var segs []*segment
	for i, tc := range []struct {
		pages []uint32
		final uint32
	}{{[]uint32{1, 5}, 6}, {[]uint32{1}, 3}, {[]uint32{1, 4, 5}, 5}, {[]uint32{1}, 2}, {[]uint32{1, 3, 4}, 4}} {
		ref := walReviewRef(e, encryption.FileWAL, tc.final, uint64(len(tc.pages)), uint64(i+8), [32]byte{2}, byte(i+2))
		m.Segments = append(m.Segments, ref)
		segs = append(segs, &segment{ref: ref, pages: tc.pages})
	}
	loc, ceiling, err := buildOverlay(m, segs)
	if err != nil {
		t.Fatal(err)
	}
	if ceiling != 2 || loc[2] != (location{}) || loc[3] != (location{5, 2}) || loc[4] != (location{5, 3}) || loc[5] != (location{}) || loc[6] != (location{}) {
		t.Fatalf("stale locations resurrected: ceiling=%d locations=%v", ceiling, loc)
	}
	for ps := uint32(512); ps <= encryption.MaxPageSize; ps *= 2 {
		for _, frames := range []uint64{1, 3, 65536} {
			end := uint64(frameOffset(ps, frames, frames)) + uint64(ps) + recordOverhead
			if end+walFooterSize+recordOverhead != segmentSize(ps, frames) {
				t.Fatal("segment geometry overflow or gap", ps, frames)
			}
		}
	}
	// Arithmetic decoding never allocates according to this unauthenticated count.
	b := m.encode()
	binary.BigEndian.PutUint32(b[20:], ^uint32(0))
	h := encryption.Header{Epoch: e}
	if _, err := decodeWALManifest(b, h, Options{Mode: WAL}); !errors.Is(err, encryption.ErrLimit) {
		t.Fatal("oversized count did not fail before allocation", err)
	}
}

func TestReviewWALMemoryResourceGraphBounds(t *testing.T) {
	master := reviewMemoryMaster(t)
	m, h := walReviewManifest(t, master)
	for name, limits := range map[string]WALLimits{
		"segments": {MaxSegments: 1}, "frames": {MaxFrames: 3}, "one segment frames": {MaxSegmentFrames: 2}, "one segment bytes": {MaxSegmentBytes: m.Segments[0].Bytes - 1}, "selected bytes": {MaxBytes: m.Bytes - 1}, "root": {MaxRootBytes: uint32(walRootSize(2) - 1)}, "overlay": {MaxOverlayPages: 5}, "metadata memory": {MaxMemoryBytes: 2048}, "descriptors": {MaxOpenFiles: 10}, "scan": {MaxScanBytes: m.Base.Bytes + m.Bytes - 1}, "operation work": {MaxOperationBytes: 1}, "replay": {MaxReplaySteps: 1},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeWALManifest(m.encode(), h, Options{Mode: WAL, WALLimits: limits}); !errors.Is(err, encryption.ErrLimit) {
				t.Fatalf("lowered graph cap ignored: %v", err)
			}
		})
	}
	for _, id := range [][32]byte{m.Base.FileID, m.Segments[0].FileID, m.Segments[1].FileID} {
		same := h
		same.FileID = id
		if _, err := decodeWALManifest(m.encode(), same, Options{Mode: WAL}); err == nil {
			t.Fatal("root reused selected object FileID")
		}
	}
	e, _ := master.Epoch()
	base := walReviewRef(e, encryption.FileBase, 1, 0, 1, [32]byte{}, 1)
	grown := walReviewRef(e, encryption.FileWAL, 1000, 1000, 2, base.CommitID, 2)
	growth := walManifest{Base: base, Segments: []objectRef{grown}, Frames: 1000, Bytes: grown.Bytes, Head: grown.CommitID, PageOneHash: grown.PageOneHash, PreviousRootHash: [32]byte{1}}
	if err := graphBudget(growth, Options{WALLimits: WALLimits{MaxReplaySteps: 1500}}, 1); !errors.Is(err, encryption.ErrLimit) {
		t.Fatal("growth hid final 1000-page coverage pass behind one-page base", err)
	}
	if err := rootExtent(1064, Options{WALLimits: WALLimits{MaxMemoryBytes: 1064*4 - 1}}); !errors.Is(err, encryption.ErrLimit) {
		t.Fatal("root cap not charged before allocation", err)
	}
}

func TestReviewWALMemoryMintedIdentityLedgerBudget(t *testing.T) {
	master := reviewMemoryMaster(t)
	m, _ := walReviewManifest(t, master)
	// Find the model's exact metadata boundary, then prove retained nonce-ID
	// reservations are additive, including simultaneous old and target masters.
	opts := Options{Mode: WAL, WALLimits: WALLimits{MaxSegmentFrames: 3}}
	low, high := uint64(1), uint64(1<<20)
	for low < high {
		mid := low + (high-low)/2
		opts.WALLimits.MaxMemoryBytes = mid
		if graphBudget(m, opts, 1) == nil {
			high = mid
		} else {
			low = mid + 1
		}
	}
	baseline := low
	opts.WALLimits.MaxMemoryBytes = baseline
	if err := graphBudget(m, opts, 1); err != nil {
		t.Fatal("metadata boundary", err)
	}
	const oldIDs, targetIDs = uint64(400), uint64(2)
	opts.WALLimits.MaxMemoryBytes = baseline + (oldIDs+targetIDs)*128
	if err := graphBudget(m, opts, 1, oldIDs*128, targetIDs*128); err != nil {
		t.Fatal("exact two-master ledger budget", err)
	}
	opts.WALLimits.MaxMemoryBytes--
	if err := graphBudget(m, opts, 1, oldIDs*128, targetIDs*128); !errors.Is(err, encryption.ErrLimit) {
		t.Fatal("two-master ledger memory not cumulative", err)
	}
	opts.WALLimits.MaxMemoryBytes = baseline + oldIDs*128 - 1
	store := Store{selection: selection{wal: &m}, opts: opts, writerIDs: oldIDs}
	if err := store.CopyBudget(); !errors.Is(err, encryption.ErrLimit) {
		t.Fatal("copy omitted live nonce-ID registry memory", err)
	}
	if err := graphBudget(m, opts, 1, ^uint64(0)); !errors.Is(err, encryption.ErrLimit) {
		t.Fatal("additional metadata memory overflow accepted", err)
	}
}
