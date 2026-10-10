package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

type credentialInput struct {
	lookupEnv func(string) (string, bool)
	stdin     func() (io.Reader, error)
}

// Providers are bound to explicit local input descriptors. Unauthenticated
// KeyRequest IDs/path never select accounts, commands, URLs or credential names.
// Acquisition stays lazy so the native API can enforce platform/KDF preflight.
type cliKeyProvider struct {
	mu        sync.Mutex
	kind      encryption.CredentialKind
	name      string
	fromStdin bool
	input     credentialInput
	key       []byte
	attempted bool
	closed    bool
}

func (*cliKeyProvider) Format(s fmt.State, verb rune) {
	_, _ = io.WriteString(s, "gosqlite.keyProvider{redacted}")
}

func newCLIKeyProvider(descriptor string, input credentialInput) (*cliKeyProvider, error) {
	p := &cliKeyProvider{input: input}
	switch {
	case strings.HasPrefix(descriptor, "env-hex:"):
		p.kind, p.name = encryption.RawKey256, strings.TrimPrefix(descriptor, "env-hex:")
	case strings.HasPrefix(descriptor, "env-password:"):
		p.kind, p.name = encryption.PasswordArgon2id, strings.TrimPrefix(descriptor, "env-password:")
	case descriptor == "stdin-hex":
		p.kind, p.fromStdin = encryption.RawKey256, true
	case descriptor == "stdin-password":
		p.kind, p.fromStdin = encryption.PasswordArgon2id, true
	default:
		return nil, errors.New(providerHelp)
	}
	if !p.fromStdin && !validEnvironmentName(p.name) {
		return nil, errors.New("provider requires an explicit ASCII environment-variable name; " + providerHelp)
	}
	return p, nil
}

func validEnvironmentName(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for i := range len(s) {
		b := s[i]
		if b == '_' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || i > 0 && b >= '0' && b <= '9' {
			continue
		}
		return false
	}
	return true
}

func (p *cliKeyProvider) Acquire(ctx context.Context, request encryption.KeyRequest) (encryption.KeyMaterial, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || request.Kind != p.kind {
		return nil, encryption.ErrCredential
	}
	if !p.attempted {
		p.attempted = true
		key, err := p.readCredential(ctx)
		if err != nil {
			return nil, err
		}
		p.key = key
	}
	if len(p.key) == 0 {
		return nil, encryption.ErrKeyProvider
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &cliKeyMaterial{kind: p.kind, key: append([]byte(nil), p.key...)}, nil
}

func (p *cliKeyProvider) readCredential(ctx context.Context) ([]byte, error) {
	maxBytes := encryption.MaxPasswordBytes
	if p.kind == encryption.RawKey256 {
		maxBytes = encryption.KeySize * 2
	}
	var encoded []byte
	if p.fromStdin {
		if p.input.stdin == nil {
			return nil, encryption.ErrKeyProvider
		}
		in, err := p.input.stdin()
		if err != nil || in == nil {
			return nil, encryption.ErrKeyProvider
		}
		// EOF terminates exactly one bounded credential. Raw hex may have one
		// LF or CRLF. Password bytes, including trailing newlines, are exact.
		limit := maxBytes
		if p.kind == encryption.RawKey256 {
			limit += 2
		}
		// Allocate once: growing io.ReadAll buffers can leave earlier copies
		// of credential bytes in allocations we can no longer clear.
		encoded = make([]byte, limit+1)
		defer clearCredential(encoded)
		n, err := io.ReadFull(in, encoded)
		if (err != io.EOF && err != io.ErrUnexpectedEOF) || n > limit {
			return nil, encryption.ErrKeyProvider
		}
		encoded = encoded[:n]
		if p.kind == encryption.RawKey256 && len(encoded) > 0 && encoded[len(encoded)-1] == '\n' {
			encoded = encoded[:len(encoded)-1]
			if len(encoded) > 0 && encoded[len(encoded)-1] == '\r' {
				encoded = encoded[:len(encoded)-1]
			}
		}
	} else {
		if p.input.lookupEnv == nil {
			return nil, encryption.ErrKeyProvider
		}
		value, ok := p.input.lookupEnv(p.name)
		if !ok || len(value) == 0 || len(value) > maxBytes {
			return nil, encryption.ErrKeyProvider
		}
		encoded = []byte(value)
		defer clearCredential(encoded)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.kind == encryption.PasswordArgon2id {
		if len(encoded) == 0 || len(encoded) > maxBytes {
			return nil, encryption.ErrCredential
		}
		return append([]byte(nil), encoded...), nil
	}
	if len(encoded) != maxBytes {
		return nil, encryption.ErrCredential
	}
	key := make([]byte, encryption.KeySize)
	if _, err := hex.Decode(key, encoded); err != nil {
		clearCredential(key)
		return nil, encryption.ErrCredential
	}
	return key, nil
}

func (p *cliKeyProvider) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	clearCredential(p.key)
	p.key = nil
	p.closed = true
}

type cliKeyMaterial struct {
	kind encryption.CredentialKind
	key  []byte
}

func (*cliKeyMaterial) Format(s fmt.State, verb rune) {
	_, _ = io.WriteString(s, "gosqlite.keyMaterial{redacted}")
}
func (m *cliKeyMaterial) Kind() encryption.CredentialKind { return m.kind }
func (m *cliKeyMaterial) Bytes() []byte                   { return m.key }
func (m *cliKeyMaterial) Release() {
	clearCredential(m.key)
	m.key = nil
}
func clearCredential(b []byte) { clear(b); runtime.KeepAlive(b) }
