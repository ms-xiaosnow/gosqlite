package encryption

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"reflect"
	"sync"
)

type Operation uint8

const (
	OperationOpen Operation = iota + 1
	OperationCreate
	OperationRekeyTarget
	OperationBackupTarget
	OperationRestoreTarget
)

// KeyRequest is a code-configured provider request. Path is caller-supplied;
// this package does not resolve or access it. DatabaseID and EpochID are NOT
// authenticated until OpenFile succeeds. Providers must not use them to select
// arbitrary external accounts, endpoints, or credentials. Bind such access in
// application code. Formatting a request deliberately redacts its contents.
type KeyRequest struct {
	Operation  Operation
	Path       string
	DatabaseID [16]byte
	EpochID    [32]byte
	Kind       CredentialKind
}

func (KeyRequest) Format(s fmt.State, verb rune) {
	_, _ = s.Write([]byte("encryption.KeyRequest{redacted}"))
}

// KeyProvider supplies one borrowed credential per Acquire. Shared providers
// are responsible for their own concurrency safety. Acquire is never retried
// implicitly. The package never formats or wraps a provider's returned error.
type KeyProvider interface {
	Acquire(context.Context, KeyRequest) (KeyMaterial, error)
}

// KeyMaterial.Bytes is borrowed until Release and must stay stable throughout
// that interval. The package copies at most MaxPasswordBytes and calls Release
// exactly once whenever a nonnil material is returned, including error cases.
// Implementations must not reuse/release the borrow concurrently. Release must
// not panic. A provider owns and is responsible for clearing its own buffers.
type KeyMaterial interface {
	Kind() CredentialKind
	Bytes() []byte
	Release()
}

// Epoch describes public, immutable key-derivation inputs. A new logical
// database or backup needs NewEpoch; rekey retains DatabaseID using
// NewRekeyEpoch. Raw-key mode requires zero KDF fields and zero KDFSalt.
type Epoch struct {
	DatabaseID [16]byte
	EpochID    [32]byte
	Kind       CredentialKind
	KDF        Argon2Params
	KDFSalt    [32]byte
	RootSalt   [32]byte
}

func (e Epoch) Validate(policy KDFPolicy) error {
	if e.DatabaseID == [16]byte{} || e.EpochID == [32]byte{} || e.RootSalt == [32]byte{} {
		return ErrFormat
	}
	if _, err := policy.normalized(); err != nil {
		return err
	}
	switch e.Kind {
	case RawKey256:
		if e.KDF != (Argon2Params{}) || e.KDFSalt != [32]byte{} {
			return ErrFormat
		}
	case PasswordArgon2id:
		if e.KDFSalt == [32]byte{} {
			return ErrFormat
		}
		return e.KDF.Validate(policy)
	default:
		return ErrUnsupported
	}
	return nil
}

func NewEpoch(kind CredentialKind, params Argon2Params) (Epoch, error) {
	return newEpoch([16]byte{}, kind, params, rand.Reader)
}

// NewRekeyEpoch creates fresh salts and epoch identity for an existing logical
// database. It does not rotate or rewrite any existing encrypted data.
func NewRekeyEpoch(databaseID [16]byte, kind CredentialKind, params Argon2Params) (Epoch, error) {
	if databaseID == [16]byte{} {
		return Epoch{}, ErrIdentity
	}
	return newEpoch(databaseID, kind, params, rand.Reader)
}

func newEpoch(databaseID [16]byte, kind CredentialKind, params Argon2Params, entropy io.Reader) (Epoch, error) {
	if kind == PasswordArgon2id {
		if params == (Argon2Params{}) {
			params = DefaultArgon2Params()
		}
		if err := params.Validate(KDFPolicy{}); err != nil {
			return Epoch{}, err
		}
	} else if kind != RawKey256 {
		return Epoch{}, ErrUnsupported
	} else if params != (Argon2Params{}) {
		return Epoch{}, ErrKDFPolicy
	}
	e := Epoch{DatabaseID: databaseID, Kind: kind, KDF: params}
	if databaseID == [16]byte{} {
		if _, err := io.ReadFull(entropy, e.DatabaseID[:]); err != nil {
			return Epoch{}, ErrRandomness
		}
	}
	if _, err := io.ReadFull(entropy, e.EpochID[:]); err != nil {
		return Epoch{}, ErrRandomness
	}
	if _, err := io.ReadFull(entropy, e.RootSalt[:]); err != nil {
		return Epoch{}, ErrRandomness
	}
	if kind == PasswordArgon2id {
		if _, err := io.ReadFull(entropy, e.KDFSalt[:]); err != nil {
			return Epoch{}, ErrRandomness
		}
	}
	if err := e.Validate(KDFPolicy{}); err != nil {
		return Epoch{}, ErrRandomness
	}
	return e, nil
}

// MasterKey is an opaque owned key schedule. Copies share Close and file-ID
// allocation history. Close prevents future file handles but does not revoke
// keys already derived into existing readers or writers; close those too.
type MasterKey struct {
	state *masterState
}

type masterState struct {
	mu     sync.Mutex
	key    []byte
	epoch  Epoch
	issued map[[32]byte]struct{}
	closed bool
}

func (MasterKey) Format(s fmt.State, verb rune) {
	_, _ = s.Write([]byte("encryption.MasterKey{redacted}"))
}

// DeriveMaster validates all public KDF bounds before invoking a provider or
// allocating expensive workspace. It replaces the request's IDs and Kind with
// epoch values; only Operation and Path come from the caller. Cancellation
// cannot interrupt Argon2id mid-derivation, but is checked before and after it.
func DeriveMaster(ctx context.Context, provider KeyProvider, request KeyRequest, epoch Epoch, policy KDFPolicy, budget *DerivationBudget) (*MasterKey, error) {
	if ctx == nil {
		return nil, ErrCredential
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := epoch.Validate(policy); err != nil {
		return nil, err
	}
	if request.Operation < OperationOpen || request.Operation > OperationRestoreTarget {
		return nil, ErrCredential
	}
	if nilInterface(provider) {
		return nil, ErrKeyProvider
	}
	request.DatabaseID, request.EpochID, request.Kind = epoch.DatabaseID, epoch.EpochID, epoch.Kind
	material, err := provider.Acquire(ctx, request)
	if !nilInterface(material) {
		defer material.Release()
	}
	if err != nil {
		// Do not inspect Error(), errors.Is(), or Unwrap() on a provider error.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrKeyProvider
	}
	if nilInterface(material) || material.Kind() != epoch.Kind {
		return nil, ErrCredential
	}
	borrowed := material.Bytes()
	if err := validCredential(epoch.Kind, len(borrowed)); err != nil {
		return nil, err
	}
	owned := append([]byte(nil), borrowed...)
	defer wipe(owned)
	input, err := deriveInput(ctx, epoch.Kind, owned, epoch, policy, budget)
	if err != nil {
		return nil, err
	}
	defer wipe(input)
	info := append([]byte("gosqlite/encryption/v1/master\x00"), epoch.DatabaseID[:]...)
	info = append(info, epoch.EpochID[:]...)
	key, err := hkdfKey(input, epoch.RootSalt[:], info)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		wipe(key)
		return nil, err
	}
	return &MasterKey{state: &masterState{key: key, epoch: epoch, issued: make(map[[32]byte]struct{})}}, nil
}

func nilInterface(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	}
	return false
}

func (m *MasterKey) Epoch() (Epoch, error) {
	if m == nil || m.state == nil {
		return Epoch{}, ErrClosed
	}
	s := m.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Epoch{}, ErrClosed
	}
	return s.epoch, nil
}

func (m *MasterKey) Close() error {
	if m == nil || m.state == nil {
		return nil
	}
	s := m.state
	s.mu.Lock()
	defer s.mu.Unlock()
	wipe(s.key)
	s.key = nil
	s.issued = nil
	s.closed = true
	return nil
}

func (m *MasterKey) fileCiphers(h Header, register bool) ([4]cipher.AEAD, error) {
	var result [4]cipher.AEAD
	if m == nil || m.state == nil {
		return result, ErrClosed
	}
	s := m.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return result, ErrClosed
	}
	if s.epoch != h.Epoch {
		return result, ErrIdentity
	}
	if register {
		if _, exists := s.issued[h.FileID]; exists {
			return result, ErrRandomness
		}
		if len(s.issued) >= MaxFilesPerMaster {
			return result, ErrLimit
		}
		// Reserve before derivation: even a failed construction burns this ID.
		s.issued[h.FileID] = struct{}{}
	}
	for role := RecordMetadata; role <= RecordFooter; role++ {
		info := append([]byte("gosqlite/encryption/v1/file\x00"), s.epoch.DatabaseID[:]...)
		info = append(info, s.epoch.EpochID[:]...)
		info = append(info, byte(h.Descriptor.Role), byte(role))
		key, err := hkdfKey(s.key, h.FileID[:], info)
		if err != nil {
			return [4]cipher.AEAD{}, err
		}
		aead, err := newAEAD(key)
		wipe(key)
		if err != nil {
			return [4]cipher.AEAD{}, err
		}
		result[role] = aead
	}
	return result, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, ErrCredential
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrUnsupportedConfiguration
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrUnsupportedConfiguration
	}
	return aead, nil
}
