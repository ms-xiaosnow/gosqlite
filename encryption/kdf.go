package encryption

import (
	"context"
	"crypto/hkdf"
	"crypto/sha256"
	"runtime"

	"golang.org/x/crypto/argon2"
)

const (
	KeySize               = 32
	MaxPasswordBytes      = 4096
	MinArgonMemoryKiB     = 64 * 1024
	MaxArgonMemoryKiB     = 256 * 1024
	MinArgonPasses        = 3
	MaxArgonPasses        = 6
	MaxArgonParallelism   = 8
	MaxArgonWorkKiBPasses = MaxArgonMemoryKiB * MaxArgonPasses
	ArgonVersion          = argon2.Version
)

// CredentialKind never auto-detects a raw key from a password's length.
type CredentialKind uint8

const (
	RawKey256        CredentialKind = 1
	PasswordArgon2id CredentialKind = 2
)

// Argon2Params is the exact declared Argon2id cost. Memory must be a multiple
// of four times Parallelism, so the underlying implementation cannot round it.
type Argon2Params struct {
	MemoryKiB   uint32
	Passes      uint32
	Parallelism uint8
}

func DefaultArgon2Params() Argon2Params {
	return Argon2Params{MemoryKiB: MinArgonMemoryKiB, Passes: MinArgonPasses, Parallelism: 4}
}

// KDFPolicy can only lower the format's maximum resource costs. Zero fields
// select format maxima; they never lower the production-strength minimum.
type KDFPolicy struct {
	MaxMemoryKiB     uint32
	MaxPasses        uint32
	MaxParallelism   uint8
	MaxWorkKiBPasses uint64
}

func (p KDFPolicy) normalized() (KDFPolicy, error) {
	if p.MaxMemoryKiB == 0 {
		p.MaxMemoryKiB = MaxArgonMemoryKiB
	}
	if p.MaxPasses == 0 {
		p.MaxPasses = MaxArgonPasses
	}
	if p.MaxParallelism == 0 {
		p.MaxParallelism = MaxArgonParallelism
	}
	if p.MaxWorkKiBPasses == 0 {
		p.MaxWorkKiBPasses = MaxArgonWorkKiBPasses
	}
	if p.MaxMemoryKiB < MinArgonMemoryKiB || p.MaxMemoryKiB > MaxArgonMemoryKiB ||
		p.MaxPasses < MinArgonPasses || p.MaxPasses > MaxArgonPasses ||
		p.MaxParallelism > MaxArgonParallelism ||
		p.MaxWorkKiBPasses < MinArgonMemoryKiB*MinArgonPasses || p.MaxWorkKiBPasses > MaxArgonWorkKiBPasses {
		return KDFPolicy{}, ErrKDFPolicy
	}
	return p, nil
}

// Validate checks both immutable format bounds and optional stricter local
// policy. It performs no expensive derivation and allocates no KDF workspace.
func (p Argon2Params) Validate(policy KDFPolicy) error {
	policy, err := policy.normalized()
	if err != nil {
		return err
	}
	if p.MemoryKiB < MinArgonMemoryKiB || p.MemoryKiB > policy.MaxMemoryKiB ||
		p.Passes < MinArgonPasses || p.Passes > policy.MaxPasses ||
		p.Parallelism == 0 || p.Parallelism > policy.MaxParallelism ||
		p.MemoryKiB%(4*uint32(p.Parallelism)) != 0 ||
		uint64(p.MemoryKiB)*uint64(p.Passes) > policy.MaxWorkKiBPasses {
		return ErrKDFPolicy
	}
	return nil
}

// DerivationBudget bounds concurrent non-interruptible Argon2 operations.
// Copies share the same semaphore. A nil or zero-value budget uses the shared
// process-wide single-derivation budget, not a fresh per-call allowance.
// Applications using explicit budgets must share them across their connectors.
type DerivationBudget struct {
	slots chan struct{}
}

var defaultBudget = DerivationBudget{slots: make(chan struct{}, 1)}

// NewDerivationBudget permits an explicit, bounded concurrency increase.
// The worst-case live KDF workspace is concurrency * MaxArgonMemoryKiB KiB.
// It does not bound runtime/GC retention after a derivation has completed.
func NewDerivationBudget(concurrency uint8) (*DerivationBudget, error) {
	if concurrency == 0 || concurrency > 8 {
		return nil, ErrLimit
	}
	return &DerivationBudget{slots: make(chan struct{}, concurrency)}, nil
}

func (b *DerivationBudget) acquire(ctx context.Context) (func(), error) {
	if ctx == nil {
		return nil, ErrCredential
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b == nil || b.slots == nil {
		b = &defaultBudget
	}
	select {
	case b.slots <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-b.slots
			return nil, err
		}
		return func() { <-b.slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func deriveInput(ctx context.Context, kind CredentialKind, owned []byte, epoch Epoch, policy KDFPolicy, budget *DerivationBudget) ([]byte, error) {
	if err := validCredential(kind, len(owned)); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if kind == RawKey256 {
		return append([]byte(nil), owned...), nil
	}
	if err := epoch.KDF.Validate(policy); err != nil {
		return nil, err
	}
	release, err := budget.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	// IDKey is synchronous. Cancellation is observed before and after it;
	// running it in a detached goroutine would evade the resource budget.
	key := argon2.IDKey(owned, epoch.KDFSalt[:], epoch.KDF.Passes, epoch.KDF.MemoryKiB, epoch.KDF.Parallelism, KeySize)
	if err := ctx.Err(); err != nil {
		wipe(key)
		return nil, err
	}
	return key, nil
}

func validCredential(kind CredentialKind, size int) error {
	switch kind {
	case RawKey256:
		if size != KeySize {
			return ErrCredential
		}
	case PasswordArgon2id:
		if size == 0 || size > MaxPasswordBytes {
			return ErrCredential
		}
	default:
		return ErrCredential
	}
	return nil
}

func hkdfKey(input, salt, info []byte) ([]byte, error) {
	key, err := hkdf.Key(sha256.New, input, salt, string(info), KeySize)
	if err != nil {
		return nil, ErrUnsupportedConfiguration
	}
	return key, nil
}

func wipe(b []byte) {
	clear(b)
	runtime.KeepAlive(b)
}
