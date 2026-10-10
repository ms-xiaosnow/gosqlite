package main

import (
	"errors"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"testing"
)

// Pure-memory public diagnostic shapes only; no native owner or file is created.
func TestIndependentPendingRetirementDiagnosticsLatch(t *testing.T) {
	for _, tt := range []struct {
		name   string
		marker error
	}{
		{"plaintext", sqlite.ErrPlaintextCleanupPending}, {"shared_open", sqlite.ErrOpenCleanupPending},
		{"shared_close", sqlite.ErrCloseCleanupPending}, {"encrypted", sqlite.ErrEncryptedCleanupPending},
	} {
		t.Run(tt.name, func(t *testing.T) {
			diagnostic := &sqlite.Error{Code: sqlite.CodeIOErr, Op: "memory", Err: errors.Join(tt.marker, sqlite.ErrPoisoned)}
			rep := Report{Status: "failed", UnverifiedRetirement: cleanupDiagnostic(diagnostic), retainedDiagnostics: []error{diagnostic}}
			ledger := newCloseLedger()
			ledger.acquire(&memoryCloseHandle{}, "otherwise clean writer")
			finalizeClosures(&rep, ledger)
			if !rep.UnverifiedRetirement || rep.Shutdown == "workers_joined_normal_close" {
				t.Fatalf("pending diagnostic not latched: unverified=%v shutdown=%s", rep.UnverifiedRetirement, rep.Shutdown)
			}
		})
	}
}
