package encstore

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/ms-xiaosnow/gosqlite/encryption"
	"reflect"
	"testing"
)

// These hashes were independently produced from explicit big-endian fields by
// Python struct/hashlib, not by the Go encoder under test. Primitive and v1
// snapshot golden vectors remain unchanged in their existing test files.
func walGoldenFixture() (encryption.Header, walManifest) {
	e := encryption.Epoch{DatabaseID: [16]byte{1}, EpochID: [32]byte{2}, RootSalt: [32]byte{1}, Kind: encryption.RawKey256}
	b := objectRef{Descriptor: encryption.FileDescriptor{Role: encryption.FileBase, PageSize: 512, PageCount: 3, RecordCount: 5, CommitSequence: 1, TransactionID: [32]byte{3}}, FileID: [32]byte{4}, Bytes: objectSize(Geometry{512, 3}), Hash: [32]byte{5}, PageOneHash: [32]byte{6}}
	b.CommitID = b.commitID(e)
	r := objectRef{Descriptor: encryption.FileDescriptor{Role: encryption.FileWAL, PageSize: 512, PageCount: 3, RecordCount: 4, CommitSequence: 2, TransactionID: [32]byte{7}, ParentCommitID: b.CommitID}, FileID: [32]byte{8}, Bytes: 2040, Hash: [32]byte{9}, PageOneHash: [32]byte{10}}
	r.CommitID = r.commitID(e)
	d := r.Descriptor
	d.Role = encryption.FileRoot
	d.RecordCount = 1
	h := encryption.Header{Major: 1, Suite: 1, Epoch: e, FileID: [32]byte{99}, Descriptor: d}
	m := walManifest{Base: b, Segments: []objectRef{r}, Frames: 2, Bytes: 2040, Head: r.CommitID, PageOneHash: r.PageOneHash, PreviousRootHash: [32]byte{11}}
	return h, m
}
func TestWALFormatFrozenMemoryVectors(t *testing.T) {
	h, m := walGoldenFixture()
	meta := segmentMetadata(m.Segments[0].Descriptor, 3, m.PageOneHash, []uint32{1, 3})
	footer := segmentFooter(m.Segments[0], 3, [32]byte{12}, sha256.Sum256(meta))
	for _, tc := range []struct {
		name string
		b    []byte
		want string
	}{
		{"base", m.Base.encode(), "613e309783f8c70dbb346f3f353c2d966ccb689a5c44efffe3503ba0db43e83d"},
		{"segment", m.Segments[0].encode(), "32da95661669d2ee051884b142a4df302303b9e16f80f5d99b7b326ca9b35952"},
		{"manifest", m.encode(), "4a6a467a470385e0bcb0a2690d3076f49e92a777b69298024e8e47878fcb3990"},
		{"directory", meta, "fcca2af1316158d56eb0b96824d73ad70d25070072fdc2a73ad3f73054f316fb"},
		{"footer", footer, "b834a2de44d65c3f27237f9d05bc139db514235f25dd988ef836e31b46429f73"},
	} {
		sum := sha256.Sum256(tc.b)
		if hex.EncodeToString(sum[:]) != tc.want {
			t.Fatal(tc.name, "format drift")
		}
	}
	got, e := decodeWALManifest(m.encode(), h, Options{Mode: WAL})
	if e != nil || !reflect.DeepEqual(got, m) {
		t.Fatal("roundtrip", e)
	}
	pages, e := parseSegmentMetadata(meta, m.Segments[0], 3)
	if e != nil || !reflect.DeepEqual(pages, []uint32{1, 3}) {
		t.Fatal("directory roundtrip", e)
	}
	for ps := uint32(512); ps <= 65536; ps *= 2 {
		for _, frames := range []uint64{1, 2, 65536} {
			end := uint64(frameOffset(ps, frames, frames)) + uint64(ps) + recordOverhead + walFooterSize + recordOverhead
			if end != segmentSize(ps, frames) {
				t.Fatal("offset/extent disagree")
			}
		}
	}
}
func FuzzWALFormatMemory(f *testing.F) {
	h, m := walGoldenFixture()
	f.Add(m.encode())
	f.Add([]byte("GSEMNF02"))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = decodeWALManifest(b, h, Options{Mode: WAL, WALLimits: WALLimits{MaxSegments: 4, MaxFrames: 64, MaxSegmentFrames: 16, MaxOverlayPages: 64, MaxMemoryBytes: 1 << 20}})
		_, _ = decodeRef(b, h.Epoch, encryption.Limits{MaxPages: 64, MaxRecords: 66})
		_, _ = parseSegmentMetadata(b, m.Segments[0], 3)
	})
}
