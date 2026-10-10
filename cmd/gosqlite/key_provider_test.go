package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

func disposableCLIKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, encryption.KeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clearCredential(key) })
	return key
}

func TestCLIProviderMemorySourcesAndRelease(t *testing.T) {
	ctx := context.Background()
	key := disposableCLIKey(t)
	for _, ending := range []string{"", "\n", "\r\n"} {
		p, err := newCLIKeyProvider("stdin-hex", credentialInput{stdin: func() (io.Reader, error) {
			return strings.NewReader(hex.EncodeToString(key) + ending), nil
		}})
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			m, err := p.Acquire(ctx, encryption.KeyRequest{Kind: encryption.RawKey256})
			if err != nil || !bytes.Equal(m.Bytes(), key) {
				t.Fatal("stdin credential mismatch")
			}
			borrow := m.Bytes()
			m.Release()
			if !bytes.Equal(borrow, make([]byte, len(borrow))) {
				t.Fatal("borrow was not cleared")
			}
		}
		owned := p.key
		p.close()
		if !bytes.Equal(owned, make([]byte, len(owned))) {
			t.Fatal("owned credential was not cleared")
		}
	}
	for _, descriptor := range []string{"env-password:LOCAL_PASSWORD", "stdin-password"} {
		password := "exact password\x00 bytes \r\n"
		p, err := newCLIKeyProvider(descriptor, credentialInput{
			lookupEnv: func(name string) (string, bool) { return password, true },
			stdin:     func() (io.Reader, error) { return strings.NewReader(password), nil },
		})
		if err != nil {
			t.Fatal(err)
		}
		m, err := p.Acquire(ctx, encryption.KeyRequest{Kind: encryption.PasswordArgon2id})
		if err != nil || string(m.Bytes()) != password {
			t.Fatal("password bytes were modified")
		}
		m.Release()
		p.close()
	}
}

func TestCLIProviderMemoryBoundsAndRedaction(t *testing.T) {
	const marker = "private-input-marker-do-not-print"
	for _, descriptor := range []string{marker, "raw:" + marker, "env-hex:", "env-hex:9BAD", "env-password:BAD-NAME", "env-hex:" + strings.Repeat("A", 129)} {
		_, err := newCLIKeyProvider(descriptor, credentialInput{})
		if err == nil || strings.Contains(err.Error(), marker) {
			t.Fatal("invalid provider descriptor was accepted or disclosed")
		}
	}
	for _, tc := range []struct {
		descriptor, value string
	}{
		{"env-hex:KEY", ""}, {"env-hex:KEY", marker},
		{"env-hex:KEY", strings.Repeat("z", 64)},
		{"env-hex:KEY", strings.Repeat("a", 65)},
		{"stdin-hex", strings.Repeat("a", 64) + "\n\n"},
		{"stdin-hex", strings.Repeat("a", 67)},
		{"stdin-password", ""},
		{"env-password:KEY", strings.Repeat("p", encryption.MaxPasswordBytes+1)},
		{"stdin-password", strings.Repeat("p", encryption.MaxPasswordBytes+1)},
	} {
		p, err := newCLIKeyProvider(tc.descriptor, credentialInput{
			lookupEnv: func(string) (string, bool) { return tc.value, true },
			stdin:     func() (io.Reader, error) { return strings.NewReader(tc.value), nil },
		})
		if err != nil {
			t.Fatal(err)
		}
		m, err := p.Acquire(context.Background(), encryption.KeyRequest{Kind: p.kind})
		if err == nil || m != nil || strings.Contains(err.Error(), marker) {
			t.Fatal("invalid memory credential was accepted or disclosed")
		}
		p.close()
	}
	p, err := newCLIKeyProvider("env-password:SECRET_NAME", credentialInput{lookupEnv: func(string) (string, bool) { return marker, true }})
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()
	m, err := p.Acquire(context.Background(), encryption.KeyRequest{Kind: p.kind})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Release()
	for _, value := range []any{p, m} {
		for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
			if rendered := fmt.Sprintf(verb, value); strings.Contains(rendered, marker) || strings.Contains(rendered, "SECRET_NAME") || !strings.Contains(rendered, "redacted") {
				t.Fatal("provider/material formatting was not redacted")
			}
		}
	}
}

func TestCLIProviderMemoryConcurrencyAndCancellation(t *testing.T) {
	key := disposableCLIKey(t)
	reads := 0
	p, err := newCLIKeyProvider("env-hex:KEY", credentialInput{lookupEnv: func(string) (string, bool) {
		reads++
		return hex.EncodeToString(key), true
	}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Acquire(ctx, encryption.KeyRequest{Kind: p.kind}); !errors.Is(err, context.Canceled) || reads != 0 {
		t.Fatal("canceled acquisition consumed input")
	}
	if _, err := p.Acquire(context.Background(), encryption.KeyRequest{Kind: encryption.PasswordArgon2id}); !errors.Is(err, encryption.ErrCredential) || reads != 0 {
		t.Fatal("mismatched kind consumed input")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			m, err := p.Acquire(context.Background(), encryption.KeyRequest{Kind: p.kind})
			if err != nil {
				t.Error(err)
				return
			}
			if !bytes.Equal(m.Bytes(), key) {
				t.Error("concurrent borrow mismatch")
			}
			m.Release()
		})
	}
	wg.Wait()
	if reads != 1 {
		t.Fatal("provider did not cache a single bounded acquisition")
	}
	p.close()
	if _, err := p.Acquire(context.Background(), encryption.KeyRequest{Kind: p.kind}); !errors.Is(err, encryption.ErrCredential) {
		t.Fatal("closed provider accepted acquisition")
	}
}

func TestCLIProviderMemoryInputFailureRedaction(t *testing.T) {
	// This is a synthetic input-provider error only, not a filesystem fault.
	const marker = "input-provider-private-message"
	p, err := newCLIKeyProvider("stdin-password", credentialInput{stdin: func() (io.Reader, error) {
		return nil, errors.New(marker)
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()
	_, err = p.Acquire(context.Background(), encryption.KeyRequest{Kind: p.kind})
	if !errors.Is(err, encryption.ErrKeyProvider) || strings.Contains(err.Error(), marker) {
		t.Fatal("input provider error escaped")
	}
}
