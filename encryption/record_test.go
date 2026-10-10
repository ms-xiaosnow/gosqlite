package encryption

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

type memoryFixture struct {
	master  *MasterKey
	header  Header
	encoded []byte
	records [][]byte
	ids     []RecordIdentity
	plain   [][]byte
}

func descriptor(role FileRole) FileDescriptor {
	d := FileDescriptor{Role: role, PageSize: 512, PageCount: 2, RecordCount: 4, CommitSequence: 7}
	if role == FileRoot {
		d.RecordCount = 1
	}
	d.TransactionID[0], d.ParentCommitID[0] = 7, 6
	return d
}

func fixture(t testing.TB, master *MasterKey, role FileRole) memoryFixture {
	t.Helper()
	w, err := NewFileWriter(master, descriptor(role), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	h, _ := w.Header()
	hb, _ := EncodeHeader(h, Limits{})
	f := memoryFixture{master: master, header: h, encoded: hb}
	f.ids = append(f.ids, RecordIdentity{Role: RecordMetadata})
	f.plain = append(f.plain, []byte("opaque authenticated metadata"))
	if role != FileRoot {
		for page := uint32(1); page <= 2; page++ {
			id := RecordIdentity{Role: RecordData, Sequence: uint64(page), PageNumber: page}
			if role == FileWAL {
				id.FrameOrdinal, id.PageNumber = uint64(page), 3-page
			}
			f.ids = append(f.ids, id)
			f.plain = append(f.plain, bytes.Repeat([]byte{byte(page + 40)}, 512))
		}
		f.ids = append(f.ids, RecordIdentity{Role: RecordFooter, Sequence: 3})
		f.plain = append(f.plain, []byte("opaque authenticated footer"))
	}
	for i, id := range f.ids {
		sealed, err := w.Seal(id.Role, id.PageNumber, id.FrameOrdinal, f.plain[i])
		if err != nil {
			t.Fatal(err)
		}
		f.records = append(f.records, sealed)
	}
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}
	return f
}

func openFixture(t testing.TB, f memoryFixture) *OpenedFile {
	t.Helper()
	r, metadata, err := OpenFile(f.master, f.header.Identity(), f.encoded, f.records[0], Limits{})
	if err != nil || !bytes.Equal(metadata, f.plain[0]) {
		t.Fatalf("metadata authentication failed: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func TestMemoryRecordsAllRoles(t *testing.T) {
	m := testMaster(t, fixedEpoch(), disposableKey(t))
	for _, role := range []FileRole{FileRoot, FileBase, FileWAL, FileBackup} {
		t.Run(fmt.Sprint(role), func(t *testing.T) {
			f := fixture(t, m, role)
			r := openFixture(t, f)
			for i, id := range f.ids {
				got, err := r.Open(id, f.records[i])
				if err != nil || !bytes.Equal(got, f.plain[i]) {
					t.Fatal("record plaintext mismatch")
				}
				if len(got) > 0 {
					got[0] ^= 1
				}
				again, err := r.Open(id, f.records[i])
				if err != nil || !bytes.Equal(again, f.plain[i]) {
					t.Fatal("reader retained caller-owned plaintext alias")
				}
				fromReader, err := ReadRecord(bytes.NewReader(f.records[i]), Limits{})
				if err != nil || !bytes.Equal(fromReader, f.records[i]) {
					t.Fatal("bounded record reader mismatch")
				}
			}
			h, err := r.Header()
			if err != nil || h != f.header {
				t.Fatal("reader lost authenticated header")
			}
			copyOfReader := *r
			_ = copyOfReader.Close()
			if got, err := r.Open(f.ids[0], f.records[0]); !errors.Is(err, ErrClosed) || got != nil {
				t.Fatal("reader copy did not share Close")
			}
		})
	}
	if _, exists := reflect.TypeOf(&OpenedFile{}).MethodByName("Seal"); exists {
		t.Fatal("opened reader unexpectedly has a write entry point")
	}
}

func TestTamperEveryHeaderAndRecordByteInMemory(t *testing.T) {
	f := fixture(t, testMaster(t, fixedEpoch(), disposableKey(t)), FileBase)
	r := openFixture(t, f)
	for offset := range f.encoded {
		mutated := bytes.Clone(f.encoded)
		mutated[offset] ^= 1
		reader, plaintext, err := OpenFile(f.master, f.header.Identity(), mutated, f.records[0], Limits{})
		if err == nil || reader != nil || plaintext != nil {
			t.Fatalf("modified header byte authenticated at %d", offset)
		}
	}
	for index, record := range f.records {
		for offset := range record {
			mutated := bytes.Clone(record)
			mutated[offset] ^= 1
			if got, err := r.Open(f.ids[index], mutated); err == nil || got != nil {
				t.Fatalf("modified record %d byte %d authenticated", index, offset)
			}
		}
		for n := range len(record) {
			if got, err := r.Open(f.ids[index], record[:n]); err == nil || got != nil {
				t.Fatalf("truncated record %d length %d authenticated", index, n)
			}
		}
		if got, err := r.Open(f.ids[index], append(bytes.Clone(record), 0)); err == nil || got != nil {
			t.Fatal("trailing bytes authenticated")
		}
	}
}

func TestWrongKeyAndAuthenticatedSubstitution(t *testing.T) {
	key := disposableKey(t)
	m := testMaster(t, fixedEpoch(), key)
	f := fixture(t, m, FileBase)
	r := openFixture(t, f)
	wrong := testMaster(t, fixedEpoch(), disposableKey(t))
	if opened, got, err := OpenFile(wrong, f.header.Identity(), f.encoded, f.records[0], Limits{}); !errors.Is(err, ErrAuthentication) || opened != nil || got != nil {
		t.Fatal("wrong key exposed plaintext/reader")
	}
	for _, mutate := range []func(*FileIdentity){
		func(i *FileIdentity) { i.DatabaseID[0] ^= 1 }, func(i *FileIdentity) { i.EpochID[0] ^= 1 },
		func(i *FileIdentity) { i.FileID[0] ^= 1 }, func(i *FileIdentity) { i.Role = FileBackup },
	} {
		id := f.header.Identity()
		mutate(&id)
		if reader, got, err := OpenFile(m, id, f.encoded, f.records[0], Limits{}); !errors.Is(err, ErrAuthentication) || got != nil || reader != nil {
			t.Fatal("unexpected file identity accepted")
		}
	}
	for _, mutate := range []func(*RecordIdentity){
		func(i *RecordIdentity) { i.Role = RecordMetadata }, func(i *RecordIdentity) { i.Role = RecordFooter },
		func(i *RecordIdentity) { i.Role = 255 }, func(i *RecordIdentity) { i.Sequence++ },
		func(i *RecordIdentity) { i.PageNumber++ }, func(i *RecordIdentity) { i.FrameOrdinal++ },
	} {
		id := f.ids[1]
		mutate(&id)
		if got, err := r.Open(id, f.records[1]); !errors.Is(err, ErrAuthentication) || got != nil {
			t.Fatal("record location substitution authenticated")
		}
	}
	// Changing a public sequence to the expected slot cannot relocate a page.
	moved := bytes.Clone(f.records[2])
	binary.BigEndian.PutUint64(moved[8:16], f.ids[1].Sequence)
	if got, err := r.Open(f.ids[1], moved); !errors.Is(err, ErrAuthentication) || got != nil {
		t.Fatal("cross-page ciphertext accepted after forged sequence")
	}
	second := fixture(t, m, FileBase)
	if got, err := r.Open(f.ids[1], second.records[1]); !errors.Is(err, ErrAuthentication) || got != nil {
		t.Fatal("cross-file ciphertext authenticated")
	}
	// Re-derive under altered public bootstrap values, as a root opener would.
	// This verifies salt/DBID/epoch are cryptographically bound, not just compared.
	for _, mutate := range []func(*Epoch){
		func(e *Epoch) { e.DatabaseID[0] ^= 1 }, func(e *Epoch) { e.EpochID[0] ^= 1 },
		func(e *Epoch) { e.RootSalt[0] ^= 1 },
	} {
		e := f.header.Epoch
		mutate(&e)
		changedMaster := testMaster(t, e, key)
		h := f.header
		h.Epoch = e
		hb, _ := EncodeHeader(h, Limits{})
		if reader, got, err := OpenFile(changedMaster, h.Identity(), hb, f.records[0], Limits{}); !errors.Is(err, ErrAuthentication) || reader != nil || got != nil {
			t.Fatal("bootstrap derivation-input substitution authenticated")
		}
	}
	wf := fixture(t, m, FileWAL)
	wr := openFixture(t, wf)
	id := wf.ids[1]
	id.PageNumber = 1 // valid page, correct frame, wrong authenticated routing
	if got, err := wr.Open(id, wf.records[1]); !errors.Is(err, ErrAuthentication) || got != nil {
		t.Fatal("WAL page substitution authenticated")
	}
}

func TestRoleSubkeysIndependentlySeparate(t *testing.T) {
	m := testMaster(t, fixedEpoch(), disposableKey(t))
	h := fixedHeader()
	base, err := m.fileCiphers(h, false)
	if err != nil {
		t.Fatal(err)
	}
	nonce := recordNonce([4]byte{1, 2, 3, 4}, 7)
	// Intentionally equal nonce and AAD isolates KEY domain separation here.
	sealed := base[RecordData].Seal(nil, nonce[:], []byte("role-separation-test"), nil)
	for _, recordRole := range []RecordRole{RecordMetadata, RecordFooter} {
		if got, err := base[recordRole].Open(nil, nonce[:], sealed, nil); err == nil || got != nil {
			t.Fatal("record roles share the same encryption key")
		}
	}
	for _, fileRole := range []FileRole{FileRoot, FileWAL, FileBackup} {
		h.Descriptor = descriptor(fileRole)
		other, err := m.fileCiphers(h, false)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := other[RecordData].Open(nil, nonce[:], sealed, nil); err == nil || got != nil {
			t.Fatal("file roles share the same encryption key")
		}
	}
}

func TestWriterFreshIdentityAndNoResume(t *testing.T) {
	m := testMaster(t, fixedEpoch(), disposableKey(t))
	first := fixture(t, m, FileBase)
	second := fixture(t, m, FileBase)
	if first.header.FileID == second.header.FileID || bytes.Equal(first.records[1], second.records[1]) {
		t.Fatal("repeat write reused file identity/ciphertext")
	}
	// A symbolic key identifier contains only public derivation-domain inputs,
	// never actual keys. Each writer/record pair has a unique domain+nonce.
	seen := make(map[string]bool)
	for _, f := range []memoryFixture{first, second} {
		for _, id := range f.ids {
			nonce := recordNonce(f.header.NoncePrefix, id.Sequence)
			pair := append(bytes.Clone(f.header.FileID[:]), byte(f.header.Descriptor.Role), byte(id.Role))
			pair = append(pair, nonce[:]...)
			if seen[string(pair)] {
				t.Fatal("duplicate symbolic key/nonce pair")
			}
			seen[string(pair)] = true
		}
	}
	if got := recordNonce([4]byte{1, 2, 3, 4}, 0x05060708090a0b0c); got != [12]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12} {
		t.Fatal("nonce is not prefix plus big-endian sequence")
	}
	// Simulate entropy collision in private test seam: copied masters must
	// share the issued-ID registry and cannot revive a closed writer identity.
	entropy := append(bytes.Clone(first.header.FileID[:]), first.header.NoncePrefix[:]...)
	copyOfMaster := *m
	if _, err := newFileWriter(&copyOfMaster, descriptor(FileBase), Limits{}, bytes.NewReader(entropy)); !errors.Is(err, ErrRandomness) {
		t.Fatal("previously issued FileID became writable again")
	}
	if _, err := newFileWriter(m, descriptor(FileBase), Limits{}, failingEntropy{}); !errors.Is(err, ErrRandomness) {
		t.Fatal("entropy failure did not abort writer creation")
	}
	if _, err := newFileWriter(m, descriptor(FileBase), Limits{}, bytes.NewReader(make([]byte, 36))); !errors.Is(err, ErrRandomness) {
		t.Fatal("all-zero FileID accepted")
	}
}

func TestWriterErrorsPoisonCopies(t *testing.T) {
	m := testMaster(t, fixedEpoch(), disposableKey(t))
	for _, action := range []func(*FileWriter) error{
		func(w *FileWriter) error { _, e := w.Seal(RecordData, 1, 0, make([]byte, 512)); return e },
		func(w *FileWriter) error { _, e := w.Seal(RecordMetadata, 1, 0, nil); return e },
		func(w *FileWriter) error { _, e := w.Seal(RecordMetadata, 0, 1, nil); return e },
		func(w *FileWriter) error { _, e := w.Seal(255, 0, 0, nil); return e },
		func(w *FileWriter) error {
			_, e := w.Seal(RecordMetadata, 0, 0, make([]byte, MaxRecordBytes+1))
			return e
		},
		func(w *FileWriter) error { return w.Finish() },
		func(w *FileWriter) error {
			w.state.next = math.MaxUint64 // unreachable through the public API
			_, e := w.Seal(RecordMetadata, 0, 0, nil)
			return e
		},
	} {
		w, err := NewFileWriter(m, descriptor(FileBase), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		copyOfWriter := *w
		if err := action(w); err == nil {
			t.Fatal("invalid writer operation succeeded")
		}
		if got, err := copyOfWriter.Seal(RecordMetadata, 0, 0, nil); !errors.Is(err, ErrClosed) || got != nil {
			t.Fatal("writer copy resumed after failure")
		}
		if w.state.ciphers[RecordMetadata] != nil {
			t.Fatal("poisoned writer retained cipher handle")
		}
	}
	w, _ := NewFileWriter(m, descriptor(FileWAL), Limits{})
	_, _ = w.Seal(RecordMetadata, 0, 0, nil)
	_, _ = w.Seal(RecordData, 1, 1, make([]byte, 512))
	if got, err := w.Seal(RecordData, 1, 2, make([]byte, 512)); !errors.Is(err, ErrIdentity) || got != nil {
		t.Fatal("duplicate WAL page accepted")
	}
	root, _ := NewFileWriter(m, descriptor(FileRoot), Limits{})
	_, _ = root.Seal(RecordMetadata, 0, 0, nil)
	if got, err := root.Seal(RecordMetadata, 0, 0, nil); !errors.Is(err, ErrWriterState) || got != nil {
		t.Fatal("second Seal reused completed root namespace")
	}
}

func TestConcurrentHandleCopies(t *testing.T) {
	m := testMaster(t, fixedEpoch(), disposableKey(t))
	for round := 0; round < 32; round++ {
		w, err := NewFileWriter(m, descriptor(FileRoot), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		var sealed atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 16; i++ {
			alias := *w
			wg.Go(func() {
				got, err := alias.Seal(RecordMetadata, 0, 0, []byte("one-shot"))
				if err == nil {
					sealed.Add(1)
				} else if got != nil {
					t.Error("failed concurrent Seal returned bytes")
				}
			})
			if i%4 == 0 && round%2 == 1 {
				wg.Go(func() { _ = alias.Close() })
			}
		}
		wg.Wait()
		if sealed.Load() > 1 || (round%2 == 0 && sealed.Load() != 1) {
			t.Fatal("concurrent copied writer reused its root nonce")
		}
		_ = w.Close()
	}
	f := fixture(t, m, FileBase)
	r := openFixture(t, f)
	var wg sync.WaitGroup
	for range 32 {
		alias := *r
		wg.Go(func() {
			got, err := alias.Open(f.ids[1], f.records[1])
			if err != nil && !errors.Is(err, ErrClosed) {
				t.Error(err)
			}
			if err == nil && !bytes.Equal(got, f.plain[1]) {
				t.Error("concurrent read plaintext mismatch")
			}
		})
		wg.Go(func() { _ = alias.Close() })
	}
	wg.Wait()
	copyOfMaster := *m
	for range 16 {
		wg.Go(func() {
			w, err := NewFileWriter(&copyOfMaster, descriptor(FileRoot), Limits{})
			if err != nil && !errors.Is(err, ErrClosed) {
				t.Error(err)
			}
			if w != nil {
				_ = w.Close()
			}
		})
		wg.Go(func() { _ = m.Close() })
	}
	wg.Wait()
}

func FuzzAuthenticatedRecord(f *testing.F) {
	fixture := fixture(f, testMaster(f, fixedEpoch(), disposableKey(f)), FileBase)
	r := openFixture(f, fixture)
	// Fuzz mutation instructions, not process-specific random ciphertext.
	// Every worker applies the same corpus instruction to its own valid local
	// fixture, so the empty seed always tests successful authentication and
	// mutations are reproducible despite independently generated test keys.
	f.Add([]byte{})
	f.Add([]byte{1})
	f.Add(bytes.Repeat([]byte{255}, RecordHeaderSize))
	f.Fuzz(func(t *testing.T, mask []byte) {
		b := bytes.Clone(fixture.records[1])
		for i := 0; i < len(mask) && i < len(b); i++ {
			b[i] ^= mask[i]
		}
		if len(mask) > len(b) {
			b = append(b, 0) // bounded trailing-byte mutation
		}
		got, err := r.Open(fixture.ids[1], b)
		if err != nil && got != nil {
			t.Fatal("authentication failure exposed plaintext")
		}
		if err == nil && (!bytes.Equal(got, fixture.plain[1]) || !bytes.Equal(b, fixture.records[1])) {
			t.Fatal("modified authenticated record accepted")
		}
	})
}
