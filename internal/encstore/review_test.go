package encstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

// Review authentication-negative fixtures stay exclusively in memory. These
// tests do not open, mutate, truncate, replace or fault any database file.
type reviewMemoryProvider []byte
type reviewMemoryMaterial []byte

func (p reviewMemoryProvider) Acquire(context.Context, encryption.KeyRequest) (encryption.KeyMaterial, error) {
	return reviewMemoryMaterial(bytes.Clone(p)), nil
}
func (reviewMemoryMaterial) Kind() encryption.CredentialKind { return encryption.RawKey256 }
func (p reviewMemoryMaterial) Bytes() []byte                 { return p }
func (p reviewMemoryMaterial) Release()                      { clear(p) }

func reviewMemoryMaster(t *testing.T) *encryption.MasterKey {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	epoch, err := encryption.NewEpoch(encryption.RawKey256, encryption.Argon2Params{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := encryption.DeriveMaster(context.Background(), reviewMemoryProvider(key), encryption.KeyRequest{Operation: encryption.OperationCreate}, epoch, encryption.KDFPolicy{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func reviewMemoryRoot(t *testing.T, m *encryption.MasterKey, payload []byte) []byte {
	t.Helper()
	w, err := encryption.NewFileWriter(m, encryption.FileDescriptor{
		Role: encryption.FileRoot, PageSize: 512, PageCount: 2,
		RecordCount: 1, CommitSequence: 7,
	}, encryption.Limits{})
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
	record, err := w.Seal(encryption.RecordMetadata, 0, 0, payload)
	if err != nil {
		t.Fatal(err)
	}
	if err = w.Finish(); err != nil {
		t.Fatal(err)
	}
	return append(raw, record...)
}

func TestReviewMemoryRootAuthenticationAndExactExtent(t *testing.T) {
	m := reviewMemoryMaster(t)
	want := manifest{BaseID: [32]byte{1}, BaseBytes: objectSize(Geometry{512, 2}), BaseHash: [32]byte{2}, PageOneHash: [32]byte{3}}
	raw := reviewMemoryRoot(t, m, want.encode())
	got, err := openRoot(raw, m, encryption.Limits{})
	if err != nil || got != want {
		t.Fatalf("valid memory root: %v", err)
	}
	for i := range raw {
		changed := bytes.Clone(raw)
		changed[i] ^= 1
		got, err := openRoot(changed, m, encryption.Limits{})
		if err == nil || got != (manifest{}) {
			t.Fatalf("changed memory root byte %d returned a manifest", i)
		}
	}
	for size := 0; size < len(raw); size++ {
		if _, err = openRoot(raw[:size], m, encryption.Limits{}); !errors.Is(err, encryption.ErrFormat) {
			t.Fatalf("memory root length %d: %v", size, err)
		}
	}
	if _, err = openRoot(append(bytes.Clone(raw), 0), m, encryption.Limits{}); !errors.Is(err, encryption.ErrFormat) {
		t.Fatalf("memory root accepted extra bytes: %v", err)
	}
}

func TestReviewMemoryAuthenticatedManifestCanonicalFields(t *testing.T) {
	m := reviewMemoryMaster(t)
	valid := manifest{BaseID: [32]byte{1}, BaseBytes: objectSize(Geometry{512, 2})}.encode()
	for _, mutate := range []func([]byte){
		func(p []byte) { p[0] ^= 1 },
		func(p []byte) { clear(p[8:40]) },
		func(p []byte) { p[112] = 1 },
		func(p []byte) { p[127] = 1 },
	} {
		payload := bytes.Clone(valid)
		mutate(payload)
		raw := reviewMemoryRoot(t, m, payload)
		_, err := openRoot(raw, m, encryption.Limits{})
		if !errors.Is(err, encryption.ErrFormat) {
			t.Fatalf("authenticated noncanonical manifest accepted: %v", err)
		}
	}
}

func TestReviewMemorySnapshotSlotGeometry(t *testing.T) {
	for size := uint32(512); size <= encryption.MaxPageSize; size *= 2 {
		g := Geometry{size, encryption.MaxPages}
		first := uint64(pageOffset(g, 1))
		end := uint64(pageOffset(g, g.PageCount)) + uint64(size) + recordOverhead
		if first != encryption.HeaderSize+recordOverhead+metaSize || end+recordOverhead+footerSize != objectSize(g) {
			t.Fatalf("snapshot slot extent mismatch for page size %d", size)
		}
		if uint64(lockPage(size)-1)*uint64(size) != 0x40000000 {
			t.Fatalf("reserved lock-page geometry mismatch for page size %d", size)
		}
	}
}

func TestReviewMemoryInventoryFramingAndOverflow(t *testing.T) {
	id := strings.Repeat("a", 64)
	for _, name := range []string{"db", "db.gse-lock", "db.gse-" + id + ".base", "db.gse-" + id + ".wal", "db.gse-" + id + ".pending", "db.gse-" + id + ".retained"} {
		member, err := familyEntry(name, "db")
		if !member || err != nil {
			t.Fatalf("canonical family entry %q: %v", name, err)
		}
	}
	for _, tail := range []string{"short.base", strings.Repeat("A", 64) + ".base", id + ".base.extra", "../" + id + ".base"} {
		if _, err := familyEntry("db.gse-"+tail, "db"); err == nil {
			t.Fatalf("noncanonical family entry %q accepted", tail)
		}
	}
	if member, err := familyEntry("other.gse-"+id+".base", "db"); member || err != nil {
		t.Fatal("unrelated family was claimed")
	}
	if total, err := charge(math.MaxUint64-1, 1, math.MaxUint64); err != nil || total != math.MaxUint64 {
		t.Fatalf("exact memory-only byte bound rejected: %v", err)
	}
	if _, err := charge(math.MaxUint64, 1, math.MaxUint64); !errors.Is(err, encryption.ErrLimit) {
		t.Fatalf("byte accounting overflow accepted: %v", err)
	}
	if _, err := charge(8, 3, 10); !errors.Is(err, encryption.ErrLimit) {
		t.Fatalf("byte accounting limit accepted: %v", err)
	}
}
