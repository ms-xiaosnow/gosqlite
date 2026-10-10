//go:build manual_recovery && linux

package pager

// SOURCE ONLY. Physical journal/recovery scenarios in this file are NOT_RUN.
// Their execution is restricted in the current environment and must not be
// redirected to another machine as a workaround. These are deterministic
// recovery regression scenarios, NOT a power-loss/crash-atomicity proof.
//
// Compilation-only check (allowed; does not execute this test binary):
//   GOCACHE="$PWD/../gocache" GOPATH="$PWD/../gopath" CGO_ENABLED=0 \
//     ../toolchains/go1.27.2/bin/go test -c -tags manual_recovery \
//     -o ../manual-recovery.test ./internal/pager
//
// Restricted execution command, documented exactly but NOT_RUN here. This
// needs independent authorization outside the current platform restriction:
//   GOSQLITE_MANUAL_RECOVERY=independently_authorized \
//     ../manual-recovery.test -test.run '^TestManualRollbackRecovery$' -test.v
//
// Separately missing acceptance evidence includes genuine process interruption,
// short writes, Sync failure, ENOSPC, torn sectors, and power-loss persistence.
// This harness does not implement or certify those additional scenarios.

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/journal"
)

func manualRecoveryFixture(t *testing.T) (string, []byte, []byte, []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "recovery.db")
	original := append(emptyDatabasePage(512), bytes.Repeat([]byte{0x12}, 512)...)
	binary.BigEndian.PutUint32(original[24:], 7)
	binary.BigEndian.PutUint32(original[28:], 2)
	// Page 2 is a valid, empty freelist trunk; unused bytes are arbitrary.
	binary.BigEndian.PutUint32(original[32:], 2)
	binary.BigEndian.PutUint32(original[36:], 1)
	clear(original[512:520])
	binary.BigEndian.PutUint32(original[92:], 7)
	current := append(append([]byte(nil), original...), bytes.Repeat([]byte{0x34}, 512)...)
	binary.BigEndian.PutUint32(current[24:], 8)
	binary.BigEndian.PutUint32(current[28:], 3)
	binary.BigEndian.PutUint32(current[92:], 8)
	current[700] ^= 0x7f
	h := journal.Header{Records: 2, Nonce: 0x12345678, OriginalPages: 2, SectorSize: 512, PageSize: 512}
	data, err := journal.EncodeHeader(h, true)
	if err != nil {
		t.Fatal(err)
	}
	for n := uint32(1); n <= 2; n++ {
		record, err := journal.EncodeRecord(h, n, original[(n-1)*512:n*512])
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, record...)
	}
	return path, original, current, data
}

func manualWriteRecoveryFiles(t *testing.T, path string, db, j []byte) {
	t.Helper()
	for name, data := range map[string][]byte{path: db, path + "-journal": j} {
		if err := os.WriteFile(name, data, 0600); err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(name, os.O_RDWR, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if err := errors.Join(f.Sync(), f.Close()); err != nil {
			t.Fatal(err)
		}
	}
	if err := syncDirectory(path); err != nil {
		t.Fatal(err)
	}
}

func manualAssertRecoveryPreserved(t *testing.T, path string, db, j []byte) {
	t.Helper()
	for name, want := range map[string][]byte{path: db, path + "-journal": j} {
		got, err := os.ReadFile(name)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("changed %s: %v", name, err)
		}
	}
}

func TestManualRollbackRecovery(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_RECOVERY") != "independently_authorized" {
		t.Fatal("NOT_RUN: physical recovery requires independent authorization; do not enable in this restricted environment")
	}
	t.Run("restore-truncate-and-idempotent-reopen", func(t *testing.T) {
		path, original, current, j := manualRecoveryFixture(t)
		manualWriteRecoveryFiles(t, path, current, j)
		result, err := RecoverRollback(context.Background(), path, RecoveryOptions{})
		if err != nil || !result.Recovered || result.RestoredPages != 2 || result.OriginalPages != 2 {
			t.Fatalf("%+v %v", result, err)
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, original) {
			t.Fatalf("original mismatch: %v", err)
		}
		if _, err := os.Lstat(path + "-journal"); !os.IsNotExist(err) {
			t.Fatalf("journal remains: %v", err)
		}
		result, err = RecoverRollback(context.Background(), path, RecoveryOptions{})
		if err != nil || result.Recovered {
			t.Fatalf("no-journal retry: %+v %v", result, err)
		}
		r, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("original-empty-extent", func(t *testing.T) {
		path, _, current, _ := manualRecoveryFixture(t)
		h := journal.Header{Records: 0, OriginalPages: 0, Nonce: 9, PageSize: 512, SectorSize: 512}
		j, err := journal.EncodeHeader(h, true)
		if err != nil {
			t.Fatal(err)
		}
		manualWriteRecoveryFiles(t, path, current, j)
		result, err := RecoverRollback(context.Background(), path, RecoveryOptions{})
		if err != nil || !result.Recovered || result.OriginalPages != 0 {
			t.Fatalf("%+v %v", result, err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() != 0 {
			t.Fatalf("empty original: %v", err)
		}
	})
	for _, mode := range []string{"checksum", "cold", "empty", "trailing", "multiple-segments", "truncated", "journal-limit", "empty-database", "canceled"} {
		t.Run("preserve-"+mode, func(t *testing.T) {
			path, _, current, j := manualRecoveryFixture(t)
			opts := RecoveryOptions{}
			ctx := context.Background()
			switch mode {
			case "checksum":
				j[512+4+312] ^= 1
			case "cold":
				j[0] = 0
			case "empty":
				j = nil
			case "trailing":
				j = append(j, 0)
			case "multiple-segments":
				padding := (512 - len(j)%512) % 512
				j = append(append(j, make([]byte, padding)...), j...)
			case "truncated":
				j = j[:len(j)-1]
			case "journal-limit":
				opts.Limits.MaxJournalBytes = 1
			case "empty-database":
				current = nil
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			manualWriteRecoveryFiles(t, path, current, j)
			result, err := RecoverRollback(ctx, path, opts)
			if err == nil || result.Recovered {
				t.Fatalf("unexpected recovery: %+v %v", result, err)
			}
			manualAssertRecoveryPreserved(t, path, current, j)
		})
	}
	for _, reserved := range []bool{false, true} {
		name := "shared-reader"
		if reserved {
			name = "reserved-writer"
		}
		t.Run("busy-"+name, func(t *testing.T) {
			path, _, current, j := manualRecoveryFixture(t)
			manualWriteRecoveryFiles(t, path, current, j)
			other, err := openRecoveryFile(path, os.O_RDWR)
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			if err := lockShared(other); err != nil {
				t.Fatal(err)
			}
			if reserved {
				if err := lockReserved(other); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := RecoverRollback(context.Background(), path, RecoveryOptions{}); !errors.Is(err, ErrBusy) {
				t.Fatalf("want BUSY: %v", err)
			}
			manualAssertRecoveryPreserved(t, path, current, j)
		})
	}
}
