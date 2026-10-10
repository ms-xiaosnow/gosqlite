package encstore

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

// Invalid authentication, layout and authority inputs below are memory-only.
// Physical histories use intact files, correct keys and ordinary API operations.
func TestReviewSharedMemoryAuthenticatedExactAuthority(t *testing.T) {
	master := reviewMemoryMaster(t)
	m, h := walReviewManifest(t, master)
	opts := Options{Mode: WALShared}
	legacy := m.encode()
	raw := walReviewSealRoot(t, master, h.Descriptor, m.encodeMode(WALShared))
	if err := verifyRootBytes(raw, master, opts); err != nil {
		t.Fatal(err)
	}
	a := Store{selection: selection{rootBytes: raw}}
	another := walReviewSealRoot(t, master, h.Descriptor, m.encodeMode(WALShared))
	b := Store{selection: selection{rootBytes: another}}
	if err := verifyRootBytes(another, master, opts); err != nil {
		t.Fatal(err)
	}
	if a.Authority() == b.Authority() {
		t.Fatal("different authenticated root identities shared an authority token")
	}
	if !bytes.Equal(legacy, m.encode()) {
		t.Fatal("shared encoding modified legacy manifest")
	}
	for _, mode := range []Mode{Snapshot, WAL} {
		if err := verifyRootBytes(raw, master, Options{Mode: mode}); err == nil {
			t.Fatal("wrong mode accepted", mode)
		}
	}
	otherMaster := reviewMemoryMaster(t)
	if err := verifyRootBytes(raw, otherMaster, opts); err == nil {
		t.Fatal("different epoch accepted in memory")
	}
	for _, off := range []int{10, 12, 16, 40, 144, 176, 177, 255, 256 + 1, 256 + 2, 256 + 12, 256 + 232} {
		payload := m.encodeMode(WALShared)
		payload[off] ^= 1
		invalid := walReviewSealRoot(t, master, h.Descriptor, payload)
		if err := verifyRootBytes(invalid, master, opts); err == nil {
			t.Fatal("authenticated noncanonical field accepted", off)
		}
	}
	for _, change := range []func(*walManifest, *encryption.Header){
		func(m *walManifest, h *encryption.Header) { m.Head[0] ^= 1 },
		func(m *walManifest, h *encryption.Header) { m.PageOneHash[0] ^= 1 },
		func(m *walManifest, h *encryption.Header) { m.Frames++ },
		func(m *walManifest, h *encryption.Header) { m.Bytes++ },
		func(m *walManifest, h *encryption.Header) {
			m.Segments[0], m.Segments[1] = m.Segments[1], m.Segments[0]
		},
		func(m *walManifest, h *encryption.Header) { h.Descriptor.TransactionID[0] ^= 1 },
	} {
		mm, hh := walReviewManifest(t, master)
		change(&mm, &hh)
		invalid := walReviewSealRoot(t, master, hh.Descriptor, mm.encodeMode(WALShared))
		if err := verifyRootBytes(invalid, master, opts); err == nil {
			t.Fatal("authenticated inconsistent graph accepted")
		}
	}
	// Previous-root receipt is authenticated but is not an external freshness oracle.
	m.PreviousRootHash[1] ^= 1
	receipt := walReviewSealRoot(t, master, h.Descriptor, m.encodeMode(WALShared))
	if err := verifyRootBytes(receipt, master, opts); err != nil {
		t.Fatal("receipt incorrectly treated as external authority", err)
	}
}

func TestReviewSharedMemoryCombinedAccountingBoundaries(t *testing.T) {
	master := reviewMemoryMaster(t)
	old, _ := walReviewManifest(t, master)
	next, _ := walReviewManifest(t, master)
	// Resource-only disjoint graph model, with different maximum overlays.
	old.Base.Descriptor.PageCount = 9000
	next.Base.Descriptor.PageCount = 7000
	opts := Options{Mode: WALShared, WALLimits: WALLimits{MaxSegmentFrames: 3}}
	const ids = uint64(47)
	low, high := uint64(1), uint64(16<<20)
	for low < high {
		mid := low + (high-low)/2
		opts.WALLimits.MaxMemoryBytes = mid
		if combinedGraphBudget(&old, next, opts, ids) == nil {
			high = mid
		} else {
			low = mid + 1
		}
	}
	opts.WALLimits.MaxMemoryBytes = low
	if err := combinedGraphBudget(&old, next, opts, ids); err != nil {
		t.Fatal("exact combined boundary", err)
	}
	opts.WALLimits.MaxMemoryBytes--
	if err := combinedGraphBudget(&old, next, opts, ids); !errors.Is(err, encryption.ErrLimit) {
		t.Fatal("combined boundary not enforced", err)
	}
	if err := graphBudget(next, opts, 1, ids*128); err != nil {
		t.Fatal("candidate alone should fit", err)
	}
	opts.WALLimits.MaxMemoryBytes = 0
	opts.WALLimits.MaxOpenFiles = uint32(len(old.Segments) + len(next.Segments) + 10)
	if err := combinedGraphBudget(&old, next, opts, ids); err != nil {
		t.Fatal("exact descriptor boundary", err)
	}
	opts.WALLimits.MaxOpenFiles--
	if err := combinedGraphBudget(&old, next, opts, ids); !errors.Is(err, encryption.ErrLimit) {
		t.Fatal("old graph descriptors omitted", err)
	}
	opts.WALLimits.MaxOpenFiles = 0
	opts.WALLimits.MaxScanBytes = next.Base.Bytes + next.Bytes - 1
	if err := combinedGraphBudget(&old, next, opts, ids); !errors.Is(err, encryption.ErrLimit) {
		t.Fatal("candidate scan admission omitted", err)
	}
}

func TestReviewSharedIntactPinnedSelectionAndLedger(t *testing.T) {
	ctx := context.Background()
	opts := Options{Mode: WALShared, CredentialKind: encryption.RawKey256, KeyProvider: reviewMemoryProvider(bytes.Repeat([]byte{37}, 32))}
	path := filepath.Join(t.TempDir(), "pin.gse")
	initial := bytes.Repeat([]byte{1}, 512)
	a, err := Open(ctx, path, opts, false, true, initial)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Open(ctx, path, opts, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if a.master == b.master {
		t.Fatal("connections share master ownership")
	}
	if a.root != nil || b.root != nil {
		t.Fatal("shared selections retain current-root descriptors")
	}
	original := a.Authority()
	originalBase := a.base
	originalIDs := a.writerIDs
	next := bytes.Repeat([]byte{2}, 512)
	if err = b.AcquireWriter(ctx, nil); err != nil {
		t.Fatal(err)
	}
	// A fresh read is admitted while another connection owns the writer byte.
	if err = a.Refresh(ctx); err != nil {
		t.Fatal("reader while writer reserved", err)
	}
	if a.Authority() != original {
		t.Fatal("unpublished reservation changed authority")
	}
	originalBase = a.base
	if err = b.PublishWAL(ctx, b.HeadCommitID(), Geometry{512, 1}, []uint32{1}, SourceFunc(func(context.Context, uint32) ([]byte, error) { return bytes.Clone(next), nil })); err != nil {
		t.Fatal(err)
	}
	if err = b.ReleaseWriter(true); err != nil {
		t.Fatal(err)
	}
	if a.Authority() != original || a.base != originalBase {
		t.Fatal("external publication replaced pinned selection")
	}
	got, err := a.ReadPage(ctx, 1)
	if err != nil || !bytes.Equal(got, initial) {
		t.Fatal("old pinned page after commit", err)
	}
	if err = b.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	got, err = a.ReadPage(ctx, 1)
	if err != nil || !bytes.Equal(got, initial) {
		t.Fatal("old pinned page after checkpoint", err)
	}
	if err = a.AcquireWriter(ctx, &original); !errors.Is(err, ErrBusySnapshot) || !errors.Is(err, ErrBusy) {
		t.Fatal("stale promotion", err)
	}
	if a.writerOwned || a.gateOwned || a.Authority() != original || a.base != originalBase {
		t.Fatal("stale promotion changed ownership or selection")
	}
	if err = a.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if a.writerIDs != originalIDs {
		t.Fatal("refresh changed minted-ID ledger")
	}
	got, err = a.ReadPage(ctx, 1)
	if err != nil || !bytes.Equal(got, next) {
		t.Fatal("fresh page after checkpoint", err)
	}
	if a.Info().WALSegments != 0 {
		t.Fatal("fresh selection missed checkpoint")
	}
	if err = b.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = a.ReadPage(ctx, 1); err != nil {
		t.Fatal("other connection closed our master", err)
	}
}

func TestReviewSharedIntactQuotaFailurePreservesSelection(t *testing.T) {
	ctx := context.Background()
	cfg := Options{Mode: WALShared, CredentialKind: encryption.RawKey256, KeyProvider: reviewMemoryProvider(bytes.Repeat([]byte{59}, 32))}
	path := filepath.Join(t.TempDir(), "quota.gse")
	before := bytes.Repeat([]byte{1}, 512)
	b, err := Open(ctx, path, cfg, false, true, before)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	limited := cfg
	limited.WALLimits.MaxOpenFiles = 10
	a, err := Open(ctx, path, limited, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	token, base, master := a.Authority(), a.base, a.master
	if err = b.AcquireWriter(ctx, nil); err != nil {
		t.Fatal(err)
	}
	after := bytes.Repeat([]byte{2}, 512)
	if err = b.PublishWAL(ctx, b.HeadCommitID(), Geometry{512, 1}, []uint32{1}, SourceFunc(func(context.Context, uint32) ([]byte, error) { return bytes.Clone(after), nil })); err != nil {
		t.Fatal(err)
	}
	if err = b.ReleaseWriter(true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err = a.Refresh(ctx); !errors.Is(err, encryption.ErrLimit) {
			t.Fatal("combined descriptor admission", err)
		}
		if a.Authority() != token || a.base != base || a.master != master || a.writerOwned || a.gateOwned || a.poisoned != nil {
			t.Fatal("failed refresh altered owned snapshot")
		}
		p, err := a.ReadPage(ctx, 1)
		if err != nil || !bytes.Equal(p, before) {
			t.Fatal("old selection not usable after quota failure", err)
		}
	}
	if err = a.AcquireWriter(ctx, &token); !errors.Is(err, encryption.ErrLimit) {
		t.Fatal("promotion quota failure", err)
	}
	if a.writerOwned || a.gateOwned {
		t.Fatal("promotion quota failure leaked locks")
	}
	if err = b.Checkpoint(ctx); err != nil {
		t.Fatal("failed promotion retained writer", err)
	}
	if err = a.Refresh(ctx); err != nil {
		t.Fatal("retry after smaller valid graph", err)
	}
	p, err := a.ReadPage(ctx, 1)
	if err != nil || !bytes.Equal(p, after) {
		t.Fatal("retry did not adopt latest head", err)
	}
}

func TestReviewSharedMemoryOwnershipPoisonPriority(t *testing.T) {
	// Error composition is memory-only; no native I/O or lock failure is induced.
	s := Store{}
	err := s.poisonOwnership(ErrBusy, true)
	if !errors.Is(err, ErrUncertain) || !errors.Is(err, ErrPoisoned) || !errors.Is(err, ErrBusy) {
		t.Fatal("lost composed uncertainty", err)
	}
	if checked := s.Check(); !errors.Is(checked, ErrUncertain) || !errors.Is(checked, ErrPoisoned) {
		t.Fatal("poison did not precede closed/ownership checks", checked)
	}
}

func TestReviewSharedIntactConcurrentReaderAndPublisher(t *testing.T) {
	ctx := context.Background()
	cfg := Options{Mode: WALShared, CredentialKind: encryption.RawKey256, KeyProvider: reviewMemoryProvider(bytes.Repeat([]byte{73}, 32))}
	path := filepath.Join(t.TempDir(), "concurrent.gse")
	initial := bytes.Repeat([]byte{1}, 512)
	a, err := Open(ctx, path, cfg, false, true, initial)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Open(ctx, path, cfg, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		<-ready
		for i := byte(2); i <= 13; i++ {
			if err := b.AcquireWriter(ctx, nil); err != nil {
				done <- err
				return
			}
			value := bytes.Repeat([]byte{i}, 512)
			err := b.PublishWAL(ctx, b.HeadCommitID(), Geometry{512, 1}, []uint32{1}, SourceFunc(func(context.Context, uint32) ([]byte, error) { return bytes.Clone(value), nil }))
			err = errors.Join(err, b.ReleaseWriter(err == nil))
			if err != nil {
				done <- err
				return
			}
			if i%3 == 0 {
				if err = b.Checkpoint(ctx); err != nil {
					done <- err
					return
				}
			}
		}
		done <- nil
	}()
	close(ready)
	var readErr error
	for i := 0; i < 64; i++ {
		p, err := a.ReadPage(ctx, 1)
		if err != nil {
			readErr = err
			break
		}
		if !bytes.Equal(p, initial) {
			readErr = errors.New("pinned reader changed during cooperative publication")
			break
		}
	}
	if err := <-done; err != nil {
		t.Fatal("publisher", err)
	}
	if readErr != nil {
		t.Fatal(readErr)
	}
	p, err := a.ReadPage(ctx, 1)
	if err != nil || !bytes.Equal(p, initial) {
		t.Fatal("old reader after publisher completed", err)
	}
	if err = a.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	p, err = a.ReadPage(ctx, 1)
	if err != nil || !bytes.Equal(p, bytes.Repeat([]byte{13}, 512)) {
		t.Fatal("fresh reader after publisher completed", err)
	}
}
