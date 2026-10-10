package encryption

import (
	"bytes"
	"context"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/argon2"
)

type testMaterial struct {
	kind     CredentialKind
	data     []byte
	releases int
}

func (m *testMaterial) Kind() CredentialKind { return m.kind }
func (m *testMaterial) Bytes() []byte        { return m.data }
func (m *testMaterial) Release() {
	m.releases++
	wipe(m.data)
}

type providerFunc func(context.Context, KeyRequest) (KeyMaterial, error)

func (f providerFunc) Acquire(c context.Context, r KeyRequest) (KeyMaterial, error) { return f(c, r) }

func fixedEpoch() Epoch {
	e := Epoch{Kind: RawKey256}
	for i := range e.DatabaseID {
		e.DatabaseID[i] = byte(i + 1)
	}
	for i := range e.EpochID {
		e.EpochID[i], e.RootSalt[i] = byte(i+17), byte(i+49)
	}
	return e
}

func disposableKey(t testing.TB) []byte {
	t.Helper()
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { wipe(key) })
	return key
}

func testMaster(t testing.TB, epoch Epoch, credential []byte) *MasterKey {
	t.Helper()
	m := &testMaterial{kind: epoch.Kind, data: bytes.Clone(credential)}
	p := providerFunc(func(_ context.Context, r KeyRequest) (KeyMaterial, error) {
		if r.Kind != epoch.Kind || r.DatabaseID != epoch.DatabaseID || r.EpochID != epoch.EpochID {
			t.Fatal("provider received mismatched identity")
		}
		return m, nil
	})
	key, err := DeriveMaster(context.Background(), p, KeyRequest{Operation: OperationOpen}, epoch, KDFPolicy{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.releases != 1 || !zeroBytes(m.data) {
		t.Fatal("provider material not released once")
	}
	t.Cleanup(func() { _ = key.Close() })
	return key
}

func unhex(t testing.TB, input string) []byte {
	t.Helper()
	b, err := hex.DecodeString(input)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// NIST, GCM-AES256 Example #4, full 128-bit tag and 96-bit IV:
// https://csrc.nist.gov/CSRC/media/Projects/Cryptographic-Standards-and-Guidelines/documents/examples/AES_GCM.pdf
// All literal key bytes in this test are PUBLIC published test-vector data.
func TestAES256GCMKnownAnswer(t *testing.T) {
	key := unhex(t, "feffe9928665731c6d6a8f9467308308feffe9928665731c6d6a8f9467308308")
	iv := unhex(t, "cafebabefacedbaddecaf888")
	plaintext := unhex(t, "d9313225f88406e5a55909c5aff5269a86a7a9531534f7da2e4c303d8a318a721c3c0c95956809532fcf0e2449a6b525b16aedf5aa0de657ba637b391aafd255")
	aad := unhex(t, "3ad77bb40d7a3660a89ecaf32466ef97f5d3d58503b9699de785895a96fdbaaf43b1cd7f598ece23881b00e3ed0306887b0c785e27e8ad3f8223207104725dd4")
	want := unhex(t, "522dc1f099567d07f47f37a32a84427d643a8cdcbfe5c0c97598a2bd2555d1aa8cb08e48590dbb3da7b08b1056828838c5f61e6393ba7a0abcc9f662898015adc06d76f31930fef37acae23ed465ae62")
	aead, err := newAEAD(key)
	if err != nil {
		t.Fatal(err)
	}
	if aead.NonceSize() != NonceSize || aead.Overhead() != TagSize {
		t.Fatal("nonstandard nonce or shortened tag")
	}
	sealed := aead.Seal(nil, iv, plaintext, aad)
	if !bytes.Equal(sealed, want) {
		t.Fatal("NIST AES-256-GCM known answer mismatch")
	}
	got, err := aead.Open(nil, iv, want, aad)
	if err != nil || !bytes.Equal(got, plaintext) {
		t.Fatal("NIST AES-256-GCM decryption mismatch")
	}
	aad[0] ^= 1
	if got, err := aead.Open(nil, iv, want, aad); err == nil || got != nil {
		t.Fatal("modified AAD authenticated")
	}
	if _, err := newAEAD(key[:31]); !errors.Is(err, ErrCredential) {
		t.Fatal("short AES-256 key accepted")
	}
}

// RFC 5869 appendix A.1 (SHA-256). The package derives the first 32 bytes of
// the same stream; the standard implementation is also checked for all 42.
// https://www.rfc-editor.org/rfc/rfc5869.html#appendix-A.1
func TestHKDFKnownAnswer(t *testing.T) {
	input := bytes.Repeat([]byte{0x0b}, 22)
	salt := unhex(t, "000102030405060708090a0b0c")
	info := unhex(t, "f0f1f2f3f4f5f6f7f8f9")
	want := unhex(t, "3cb25f25faacd57a90434f64d0362f2a2d2d0a90cf1a5a4c5db02d56ecc4c5bf34007208d5b887185865")
	got, err := hkdfKey(input, salt, info)
	if err != nil || !bytes.Equal(got, want[:KeySize]) {
		t.Fatal("RFC 5869 package derivation mismatch")
	}
	full, err := hkdf.Key(sha256.New, input, salt, string(info), len(want))
	if err != nil || !bytes.Equal(full, want) {
		t.Fatal("RFC 5869 full known answer mismatch")
	}
}

func TestArgonProductionCostsAndRawSeparation(t *testing.T) {
	e := fixedEpoch()
	e.Kind, e.KDF = PasswordArgon2id, DefaultArgon2Params()
	for i := range e.KDFSalt {
		e.KDFSalt[i] = byte(i + 81)
	}
	// The minimum production policy is used unchanged even in tests. This is
	// a wrapper-wiring check against the upstream API, not an independent KAT.
	password := disposableKey(t) // 32 bytes is STILL password mode.
	m := testMaster(t, e, password)
	input := argon2.IDKey(password, e.KDFSalt[:], e.KDF.Passes, e.KDF.MemoryKiB, e.KDF.Parallelism, KeySize)
	defer wipe(input)
	info := append([]byte("gosqlite/encryption/v1/master\x00"), e.DatabaseID[:]...)
	info = append(info, e.EpochID[:]...)
	want, err := hkdfKey(input, e.RootSalt[:], info)
	if err != nil || !bytes.Equal(m.state.key, want) {
		t.Fatal("Argon2id production-cost key hierarchy mismatch")
	}
	defer wipe(want)
	raw := e
	raw.Kind, raw.KDF, raw.KDFSalt = RawKey256, Argon2Params{}, [32]byte{}
	rawMaster := testMaster(t, raw, password)
	if bytes.Equal(rawMaster.state.key, m.state.key) {
		t.Fatal("raw and explicit 32-byte password credentials were conflated")
	}
}

func TestPasswordAuthenticationAndSaltBinding(t *testing.T) {
	e := fixedEpoch()
	e.Kind, e.KDF, e.KDFSalt[0] = PasswordArgon2id, DefaultArgon2Params(), 1
	password := disposableKey(t)
	f := fixture(t, testMaster(t, e, password), FileRoot)
	_ = openFixture(t, f)
	wrongPassword := bytes.Clone(password)
	wrongPassword[0] ^= 1
	defer wipe(wrongPassword)
	wrong := testMaster(t, e, wrongPassword)
	if r, plaintext, err := OpenFile(wrong, f.header.Identity(), f.encoded, f.records[0], Limits{}); !errors.Is(err, ErrAuthentication) || r != nil || plaintext != nil {
		t.Fatal("wrong password authenticated")
	}
	changed := f.header
	changed.Epoch.KDFSalt[0] ^= 2
	changedMaster := testMaster(t, changed.Epoch, password)
	hb, _ := EncodeHeader(changed, Limits{})
	if r, plaintext, err := OpenFile(changedMaster, changed.Identity(), hb, f.records[0], Limits{}); !errors.Is(err, ErrAuthentication) || r != nil || plaintext != nil {
		t.Fatal("modified password salt authenticated")
	}
}

// This context deterministically signals cancellation at the post-IDKey check.
// It avoids timing-dependent cancellation tests or a detached KDF goroutine.
type postDerivationContext struct {
	checks int
	done   chan struct{}
}

func (*postDerivationContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *postDerivationContext) Done() <-chan struct{}     { return c.done }
func (*postDerivationContext) Value(any) any               { return nil }
func (c *postDerivationContext) Err() error {
	c.checks++
	if c.checks == 4 {
		close(c.done)
	}
	if c.checks >= 4 {
		return context.Canceled
	}
	return nil
}

func TestPostArgonCancellationReturnsNoKey(t *testing.T) {
	e := fixedEpoch()
	e.Kind, e.KDF, e.KDFSalt[0] = PasswordArgon2id, DefaultArgon2Params(), 1
	budget, _ := NewDerivationBudget(1)
	ctx := &postDerivationContext{done: make(chan struct{})}
	key, err := deriveInput(ctx, e.Kind, disposableKey(t), e, KDFPolicy{}, budget)
	if !errors.Is(err, context.Canceled) || key != nil || len(budget.slots) != 0 || ctx.checks != 4 {
		t.Fatal("post-derivation cancellation leaked key or budget")
	}
}

func TestArgonParametersRejectBeforeProvider(t *testing.T) {
	valid := DefaultArgon2Params()
	for _, bad := range []Argon2Params{
		{}, {MemoryKiB: 1, Passes: 3, Parallelism: 4}, {MemoryKiB: MinArgonMemoryKiB, Passes: 2, Parallelism: 4},
		{MemoryKiB: MaxArgonMemoryKiB + 1, Passes: 3, Parallelism: 4}, {MemoryKiB: MinArgonMemoryKiB, Passes: 7, Parallelism: 4},
		{MemoryKiB: MinArgonMemoryKiB, Passes: 3, Parallelism: 0}, {MemoryKiB: MinArgonMemoryKiB, Passes: 3, Parallelism: 9},
		{MemoryKiB: MinArgonMemoryKiB, Passes: 3, Parallelism: 3}, // would be silently rounded
		{MemoryKiB: math.MaxUint32, Passes: math.MaxUint32, Parallelism: 255},
	} {
		if err := bad.Validate(KDFPolicy{}); !errors.Is(err, ErrKDFPolicy) {
			t.Fatal("invalid Argon cost accepted")
		}
		e := fixedEpoch()
		e.Kind, e.KDF, e.KDFSalt[0] = PasswordArgon2id, bad, 1
		calls := 0
		p := providerFunc(func(context.Context, KeyRequest) (KeyMaterial, error) {
			calls++
			return nil, nil
		})
		if _, err := DeriveMaster(context.Background(), p, KeyRequest{Operation: OperationOpen}, e, KDFPolicy{}, nil); !errors.Is(err, ErrKDFPolicy) || calls != 0 {
			t.Fatal("invalid KDF reached provider")
		}
	}
	for _, policy := range []KDFPolicy{
		{MaxMemoryKiB: MinArgonMemoryKiB - 1}, {MaxMemoryKiB: MaxArgonMemoryKiB + 1},
		{MaxPasses: 2}, {MaxPasses: 7}, {MaxParallelism: 9}, {MaxWorkKiBPasses: 1},
		{MaxWorkKiBPasses: MaxArgonWorkKiBPasses + 1},
	} {
		if err := valid.Validate(policy); !errors.Is(err, ErrKDFPolicy) {
			t.Fatal("invalid local policy accepted")
		}
	}
	upper := Argon2Params{MemoryKiB: MaxArgonMemoryKiB, Passes: MaxArgonPasses, Parallelism: MaxArgonParallelism}
	if err := upper.Validate(KDFPolicy{}); err != nil {
		t.Fatal(err)
	}
	if err := upper.Validate(KDFPolicy{MaxWorkKiBPasses: MaxArgonWorkKiBPasses - 1}); !errors.Is(err, ErrKDFPolicy) {
		t.Fatal("combined memory-work cap not enforced")
	}
	if err := upper.Validate(KDFPolicy{MaxMemoryKiB: MinArgonMemoryKiB}); !errors.Is(err, ErrKDFPolicy) {
		t.Fatal("local memory cap not enforced")
	}
}

type secretError struct{}

func (secretError) Error() string { panic("provider Error method must never be called") }

func TestProviderReleaseAndRedaction(t *testing.T) {
	e := fixedEpoch()
	for _, tc := range []struct {
		name     string
		kind     CredentialKind
		size     int
		err      error
		want     error
		material bool
	}{
		{"failure with material", RawKey256, 32, secretError{}, ErrKeyProvider, true},
		{"failure without material", RawKey256, 32, secretError{}, ErrKeyProvider, false},
		{"nil material", RawKey256, 32, nil, ErrCredential, false},
		{"wrong mode", PasswordArgon2id, 32, nil, ErrCredential, true},
		{"empty", RawKey256, 0, nil, ErrCredential, true},
		{"short", RawKey256, 31, nil, ErrCredential, true},
		{"long", RawKey256, 33, nil, ErrCredential, true},
		{"huge", RawKey256, MaxPasswordBytes + 1, nil, ErrCredential, true},
		{"success", RawKey256, 32, nil, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			material := &testMaterial{kind: tc.kind, data: bytes.Repeat([]byte{0x41}, tc.size)}
			p := providerFunc(func(context.Context, KeyRequest) (KeyMaterial, error) {
				if !tc.material {
					return (*testMaterial)(nil), tc.err // typed nil must not panic
				}
				return material, tc.err
			})
			key, err := DeriveMaster(context.Background(), p, KeyRequest{Operation: OperationOpen}, e, KDFPolicy{}, nil)
			if !errors.Is(err, tc.want) {
				t.Fatalf("safe error classification: %v", err)
			}
			if tc.material && material.releases != 1 {
				t.Fatal("material not released exactly once")
			}
			if key != nil {
				_ = key.Close()
			}
		})
	}
	for _, size := range []int{0, MaxPasswordBytes + 1} {
		if err := validCredential(PasswordArgon2id, size); !errors.Is(err, ErrCredential) {
			t.Fatal("password length bound not enforced")
		}
	}
	if err := validCredential(PasswordArgon2id, MaxPasswordBytes); err != nil {
		t.Fatal("maximum password length rejected")
	}
	if _, err := DeriveMaster(context.Background(), providerFunc(nil), KeyRequest{Operation: OperationOpen}, e, KDFPolicy{}, nil); !errors.Is(err, ErrKeyProvider) {
		t.Fatal("typed nil provider accepted")
	}
	m := testMaster(t, e, disposableKey(t))
	for _, value := range []any{m, *m, KeyRequest{Path: "disposable-sensitive-path"}} {
		for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
			out := fmt.Sprintf(format, value)
			if !strings.Contains(out, "redacted") || strings.Contains(out, "disposable-sensitive-path") {
				t.Fatal("key handle/request formatting leaked state")
			}
		}
	}
	owned := m.state.key
	copyOfMaster := *m
	_ = copyOfMaster.Close()
	if !zeroBytes(owned) {
		t.Fatal("owned master buffer not cleared")
	}
	if _, err := m.Epoch(); !errors.Is(err, ErrClosed) {
		t.Fatal("copy did not share closed lifecycle")
	}
}

func TestBudgetCopiesAndCancellation(t *testing.T) {
	b, err := NewDerivationBudget(1)
	if err != nil {
		t.Fatal(err)
	}
	copyOfBudget := *b
	release, err := b.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	base, cancel := context.WithCancel(context.Background())
	ctx := &budgetWaitContext{Context: base, entered: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		r, err := copyOfBudget.acquire(ctx)
		if r != nil {
			r()
		}
		done <- err
	}()
	<-ctx.entered // acquire reached its select with the shared slot still full
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal("copied budget bypassed active slot/cancellation")
	}
	release()
	if len(b.slots) != 0 {
		t.Fatal("canceled waiter leaked slot")
	}
	for _, n := range []uint8{0, 9, 255} {
		if _, err := NewDerivationBudget(n); !errors.Is(err, ErrLimit) {
			t.Fatal("invalid concurrency accepted")
		}
	}
	var calls atomic.Int32
	p := providerFunc(func(context.Context, KeyRequest) (KeyMaterial, error) {
		calls.Add(1)
		return nil, nil
	})
	if _, err := DeriveMaster(ctx, p, KeyRequest{Operation: OperationOpen}, fixedEpoch(), KDFPolicy{}, nil); !errors.Is(err, context.Canceled) || calls.Load() != 0 {
		t.Fatal("canceled derivation called provider")
	}
	// Nil and zero-value budgets share the same actual process allowance.
	r, err := (*DerivationBudget)(nil).acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var zero DerivationBudget
	zeroBase, zeroCancel := context.WithCancel(context.Background())
	zeroCtx := &budgetWaitContext{Context: zeroBase, entered: make(chan struct{})}
	go func() {
		release, err := zero.acquire(zeroCtx)
		if release != nil {
			release()
		}
		done <- err
	}()
	<-zeroCtx.entered
	zeroCancel()
	if err := <-done; !errors.Is(err, context.Canceled) || len(defaultBudget.slots) != 1 {
		t.Fatal("zero budget did not share the occupied process budget")
	}
	r()
}

// Done is evaluated upon entering acquire's blocking select, after the
// pre-cancellation check. The signal makes blocked-wait tests deterministic.
type budgetWaitContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (c *budgetWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.entered) })
	return c.Context.Done()
}

type failingEntropy struct{}

func (failingEntropy) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestEpochFreshnessAndEntropyFailure(t *testing.T) {
	first, err := NewEpoch(RawKey256, Argon2Params{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewRekeyEpoch(first.DatabaseID, PasswordArgon2id, Argon2Params{})
	if err != nil {
		t.Fatal(err)
	}
	if second.DatabaseID != first.DatabaseID || second.EpochID == first.EpochID || second.RootSalt == first.RootSalt || second.KDF != DefaultArgon2Params() || second.KDFSalt == [32]byte{} {
		t.Fatal("fresh rekey identity/salts/default costs missing")
	}
	for _, entropy := range []io.Reader{failingEntropy{}, bytes.NewReader(make([]byte, 128))} {
		if _, err := newEpoch([16]byte{}, RawKey256, Argon2Params{}, entropy); !errors.Is(err, ErrRandomness) {
			t.Fatal("missing/all-zero identity entropy accepted")
		}
	}
	if _, err := NewEpoch(RawKey256, DefaultArgon2Params()); !errors.Is(err, ErrKDFPolicy) {
		t.Fatal("raw mode accepted password KDF parameters")
	}
}
