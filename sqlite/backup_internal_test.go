package sqlite

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
)

// Admission and public error-graph tests use no pathname access or native file.
func TestPlaintextBackupMemoryAdmission(t *testing.T) {
	for _, o := range []Options{{Encryption: &EncryptionOptions{}}, {WALExclusive: true}, {WALShared: true}} {
		d := &DB{opts: o}
		if info, err := d.Backup(context.Background(), "must-not-be-accessed"); !errors.Is(err, ErrUnsupported) || info != (FileInfo{}) {
			t.Fatal(info, err)
		}
	}
	for _, d := range []*DB{{closed: true}, {tx: &Tx{}}, {rows: map[*Rows]struct{}{&Rows{}: {}}}} {
		want := ErrBusy
		if runtime.GOOS != "linux" {
			want = ErrUnsupported
		}
		if d.closed {
			want = ErrClosed
		}
		if _, err := d.Backup(context.Background(), "must-not-be-accessed"); !errors.Is(err, want) {
			t.Fatal(err)
		}
	}
	d := &DB{}
	tx := &Tx{db: d, ctx: context.Background()}
	if _, err := tx.Backup(context.Background(), "must-not-be-accessed"); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := d.Backup(ctx, "must-not-be-accessed"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := d.Backup(nil, "must-not-be-accessed"); err == nil {
		t.Fatal("nil context accepted")
	}
}
func TestPlaintextBackupMemoryPublicErrorGraphs(t *testing.T) {
	sourceCause, destinationCause, retirementCause := errors.New("source"), errors.New("destination"), errors.New("retirement")
	operation := wrap("backup", errors.Join(pager.ErrCommitUncertain, sourceCause, destinationCause))
	retirement := errors.Join(pager.ErrPlaintextCleanupPending, pager.ErrPoisoned, retirementCause)
	result := wrapCleanup("backup", operation, retirement)
	for _, cause := range []error{ErrCommitUncertain, ErrPlaintextCleanupPending, ErrPoisoned, sourceCause, destinationCause, retirementCause} {
		if !errors.Is(result, cause) {
			t.Fatal("lost public cause", cause, result)
		}
	}
	var detail *Error
	if !errors.As(result, &detail) || detail.Code != CodeIOErr {
		t.Fatal(detail, result)
	}
}
