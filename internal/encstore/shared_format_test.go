package encstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

func TestSharedLayoutFrozenMemoryModeAndProtocol(t *testing.T) {
	h, m := walGoldenFixture()
	b := m.encodeMode(WALShared)
	sum := sha256.Sum256(b)
	// Independent Python struct/hashlib serialization of the public fixed fields.
	const want = "2b0fd194ef8dd0c4c9bbe3bfcbe51ccfa6b7df6d827811de4f55a45400b3e822"
	if hex.EncodeToString(sum[:]) != want {
		t.Fatalf("layout-3 vector drift: %x", sum)
	}
	got, e := decodeStorageWAL(b, h, Options{Mode: WALShared})
	if e != nil || !reflect.DeepEqual(got, m) {
		t.Fatal("shared roundtrip", e)
	}
	if _, e = decodeWALManifest(b, h, Options{Mode: WAL}); e == nil {
		t.Fatal("legacy decoder accepted shared")
	}
	if _, e = decodeStorageWAL(m.encode(), h, Options{Mode: WALShared}); e == nil {
		t.Fatal("shared decoder accepted legacy")
	}
	for _, offset := range []int{0, 8, 176, 177, 255} {
		changed := bytes.Clone(b)
		changed[offset] ^= 2
		if _, e = decodeStorageWAL(changed, h, Options{Mode: WALShared}); e == nil {
			t.Fatal("protocol/reserved byte accepted", offset)
		}
	}
	master := reviewMemoryMaster(t)
	m, h = walReviewManifest(t, master)
	raw := walReviewSealRoot(t, master, h.Descriptor, m.encodeMode(WALShared))
	if e = verifyRootBytes(raw, master, Options{Mode: WALShared}); e != nil {
		t.Fatal(e)
	}
	for _, mode := range []Mode{Snapshot, WAL} {
		if e = verifyRootBytes(raw, master, Options{Mode: mode}); e == nil {
			t.Fatal("mode boundary accepted", mode)
		}
	}
	raw[len(raw)-1] ^= 1
	if e = verifyRootBytes(raw, master, Options{Mode: WALShared}); e == nil {
		t.Fatal("memory authentication negative accepted")
	}
}

func TestSharedCombinedMemoryAndDescriptorBudgets(t *testing.T) {
	_, m := walGoldenFixture()
	o := Options{Mode: WALShared}
	if e := combinedGraphBudget(&m, m, o, 7); e != nil {
		t.Fatal(e)
	}
	// Either selection alone fits 10 handles, both full selections need 12.
	o.WALLimits.MaxOpenFiles = 10
	if e := graphBudget(m, o, 1); e != nil {
		t.Fatal("single graph", e)
	}
	if e := combinedGraphBudget(&m, m, o, 0); !errors.Is(e, encryption.ErrLimit) {
		t.Fatal("combined handles", e)
	}
	o.WALLimits.MaxOpenFiles = 0
	l, _ := o.WALLimits.normalized()
	single := selectionMemory(m) + uint64(l.MaxSegmentFrames)*80
	o.WALLimits.MaxMemoryBytes = single
	if e := graphBudget(m, o, 1); e != nil {
		t.Fatal("single memory", e)
	}
	if e := combinedGraphBudget(&m, m, o, 0); !errors.Is(e, encryption.ErrLimit) {
		t.Fatal("combined memory", e)
	}
	huge := m
	huge.Frames = ^uint64(0)
	if e := combinedGraphBudget(&huge, m, o, 0); !errors.Is(e, encryption.ErrLimit) {
		t.Fatal("overflow graph bound", e)
	}
	if e := rootBytesBudget(0, ^uint64(0), 0, o); !errors.Is(e, encryption.ErrLimit) {
		t.Fatal("overflow root multiply", e)
	}
	if e := rootBytesBudget(^uint64(0), 100, 0, o); !errors.Is(e, encryption.ErrLimit) {
		t.Fatal("overflow root bound", e)
	}
	if e := rootBytesBudget(0, 100, encryption.MaxFilesPerMaster*2+1, o); !errors.Is(e, encryption.ErrLimit) {
		t.Fatal("ID ledger", e)
	}
}
