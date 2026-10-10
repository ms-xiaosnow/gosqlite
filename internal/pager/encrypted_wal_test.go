package pager

import (
	"bytes"
	"context"
	"errors"
	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
	"os"
	"path/filepath"
	"testing"
)

// Only normal correct-key/intact-file operations. No recovery, fault, tamper,
// missing-selected-file or wrong-credential workflow belongs in this suite.
func TestEncryptedPagerWALFinalDirtySegments(t *testing.T) {
	ctx := context.Background()
	cfg := encryptedTestOptions(t)
	cfg.Mode = encstore.WAL
	path := filepath.Join(t.TempDir(), "wal.gse")
	s, e := OpenEncryptedSession(ctx, path, Options{Create: true, PageSize: 512, KeepWALOnClose: true}, cfg, false)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	tx, e := s.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 80; i++ {
		n, e := tx.AllocatePage(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if e = tx.WritePage(ctx, n, bytes.Repeat([]byte{byte(i + 10)}, 512)); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if e = s.Checkpoint(ctx); e != nil {
		t.Fatal(e)
	}
	bases, e := filepath.Glob(path + ".gse-*.base")
	if e != nil {
		t.Fatal(e)
	}
	saved := map[string][]byte{}
	for _, p := range bases {
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		saved[p] = b
	}
	before := s.EncryptionInfo()
	tx, e = s.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range []byte{101, 102, 103} {
		if e = tx.WritePage(ctx, 40, bytes.Repeat([]byte{v}, 512)); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	after := s.EncryptionInfo()
	if after.WALSegments != 1 || after.WALFrames != 2 || after.WALBytes != 928+2*(512+44) || after.HeadCommitID == before.HeadCommitID {
		t.Fatalf("not a final-dirty segment: %+v", after)
	}
	nextBases, e := filepath.Glob(path + ".gse-*.base")
	if e != nil || len(nextBases) != len(bases) {
		t.Fatal("WAL commit created a base", e)
	}
	for p, want := range saved {
		got, e := os.ReadFile(p)
		if e != nil || !bytes.Equal(want, got) {
			t.Fatal("base changed", e)
		}
	}
	root, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	tx, e = s.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.WritePage(ctx, 40, bytes.Repeat([]byte{104}, 512)); e != nil {
		t.Fatal(e)
	}
	if e = tx.Rollback(); e != nil {
		t.Fatal(e)
	}
	tx, e = s.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	gotRoot, e := os.ReadFile(path)
	if e != nil || !bytes.Equal(root, gotRoot) {
		t.Fatal("rollback/no-op publication", e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e = OpenEncryptedSession(ctx, path, Options{KeepWALOnClose: true}, cfg, false)
	if e != nil {
		t.Fatal(e)
	}
	if s.EncryptionInfo().WALSegments != 1 {
		t.Fatal("normal close lost selection")
	}
	r, e := s.Reader(ctx)
	if e != nil {
		t.Fatal(e)
	}
	p, e := r.ReadPage(ctx, 40)
	if e != nil || !bytes.Equal(p, bytes.Repeat([]byte{103}, 512)) {
		t.Fatal("head mismatch", e)
	}
	r.Close()
	if e = s.Verify(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = OpenEncryptedSession(ctx, path, Options{}, cfg, true); !errors.Is(e, ErrBusy) {
		t.Fatal("reader coexisted with exclusive writer", e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e = OpenEncryptedSession(ctx, path, Options{}, cfg, false)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e = OpenEncryptedSession(ctx, path, Options{}, cfg, true)
	if e != nil {
		t.Fatal(e)
	}
	if s.EncryptionInfo().WALSegments != 0 {
		t.Fatal("default close did not checkpoint")
	}
	s.Close()
}

func TestEncryptedPagerWALQuotaBeforePublication(t *testing.T) {
	ctx := context.Background()
	cfg := encryptedTestOptions(t)
	cfg.Mode = encstore.WAL
	cfg.WALLimits.MaxSegments = 1
	path := filepath.Join(t.TempDir(), "quota.gse")
	s, e := OpenEncryptedSession(ctx, path, Options{Create: true, PageSize: 512, KeepWALOnClose: true}, cfg, false)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	tx, e := s.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	n, e := tx.AllocatePage(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.WritePage(ctx, n, make([]byte, 512)); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	root, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	files, e := os.ReadDir(filepath.Dir(path))
	if e != nil {
		t.Fatal(e)
	}
	tx, e = s.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.WritePage(ctx, n, bytes.Repeat([]byte{1}, 512)); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); !errors.Is(e, ErrLimit) {
		t.Fatal("missing active-segment cap", e)
	}
	if e = tx.Rollback(); e != nil {
		t.Fatal(e)
	}
	now, e := os.ReadFile(path)
	if e != nil || !bytes.Equal(root, now) {
		t.Fatal("quota changed root", e)
	}
	next, e := os.ReadDir(filepath.Dir(path))
	if e != nil || len(next) != len(files) {
		t.Fatal("quota created file", e)
	}
	if e = s.Checkpoint(ctx); e != nil {
		t.Fatal(e)
	}
}
