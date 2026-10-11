package pager

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/walindex"
)

func TestSharedReadOnlyOpenPolicyMemory(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows", "plan9"} {
		for _, create := range []bool{false, true} {
			for _, psow := range []bool{false, true} {
				err := sharedReadOnlyOpenPolicy(goos, true, Options{Create: create, WALPowersafeOverwrite: psow})
				want := goos == "linux" && !create && !psow
				if (err == nil) != want || !want && !errors.Is(err, ErrUnsupported) {
					t.Fatalf("%s create=%v psow=%v: %v", goos, create, psow, err)
				}
				if err := sharedReadOnlyOpenPolicy(goos, false, Options{Create: create, WALPowersafeOverwrite: psow}); err != nil {
					t.Fatal("writable policy changed", err)
				}
			}
		}
	}
	for _, create := range []bool{false, true} {
		if sharedMainOpenFlags(true, create) != os.O_RDONLY || sharedWALOpenFlags(true) != os.O_RDONLY {
			t.Fatal("readonly main/WAL acquired write/create flags")
		}
	}
	if sharedMainOpenFlags(false, false) != os.O_RDWR || sharedMainOpenFlags(false, true) != os.O_RDWR|os.O_CREATE || sharedWALOpenFlags(false) != os.O_RDWR|os.O_CREATE {
		t.Fatal("writable flags changed")
	}
}

func TestSharedReadOnlyReadmarkMemory(t *testing.T) {
	h := walindex.Header{Version: walindex.Version, PageSize: 4096, MaxFrame: 20, DBSize: 12, Salt: [8]byte{1, 2}}
	chosen, mark := 0, uint32(0)
	for slot, candidate := range []uint32{walindex.UnusedReadMark, 3, 9, 21} {
		if sharedReadMarkPreferred(h.MaxFrame, chosen, mark, candidate) {
			chosen, mark = slot+1, candidate
		}
	}
	if chosen != 3 || mark != 9 {
		t.Fatalf("reusable older mark: slot=%d mark=%d", chosen, mark)
	}
	if !sharedReadSnapshotStable(h, h, mark, mark, 7) {
		t.Fatal("older readmark must retain newer captured MaxFrame and DBSize")
	}
	snap := sharedRead{hdr: h, slot: chosen, minFrame: 8}
	if snap.hdr.MaxFrame != 20 || snap.hdr.DBSize != 12 || snap.minFrame != 8 {
		t.Fatal("snapshot downgraded to its protective readmark")
	}
	changed := h
	changed.MaxFrame++
	if sharedReadSnapshotStable(h, changed, mark, mark, 7) || sharedReadSnapshotStable(h, h, mark, mark+1, 7) || sharedReadSnapshotStable(h, h, mark, mark, 21) {
		t.Fatal("accepted changed header/mark or impossible backfill")
	}
	if sharedReadMarkPreferred(20, 0, 0, walindex.UnusedReadMark) || sharedReadMarkPreferred(20, 0, 0, 21) || !sharedReadMarkPreferred(20, chosen, mark, 20) {
		t.Fatal("invalid/exact readmark selection")
	}
	if err := sharedReadOnlyMarkError(true, 0); !errors.Is(err, ErrUnsupported) { t.Fatal(err) }
	if err := sharedReadOnlyMarkError(true, 1); err != nil { t.Fatal(err) }
	if err := sharedReadOnlyMarkError(false, 0); err != nil { t.Fatal(err) }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (&SharedSession{readOnly: true}).beginReadLocked(ctx, false); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation did not precede index access", err)
	}
}

func TestSharedReadOnlyClosePolicyMemory(t *testing.T) {
	for _, keep := range []bool{false, true} {
		registry := &sharedCloseRetirementRegistry{limit: 1}
		group, owners, _ := memoryCloseGroup(t, registry)
		owners[1].remaining = 1
		s := &SharedSession{readOnly: true, opts: Options{KeepWALOnClose: keep}, closeGroup: group}
		checkpoints := 0
		checkpoint := func() error { checkpoints++; return nil }
		err := s.closeLocked(nil, checkpoint)
		if !errors.Is(err, ErrCloseCleanupPending) || !s.closed || checkpoints != 0 || owners[2].calls != 0 || owners[3].calls != 0 {
			t.Fatal("readonly close lost its owner barrier or checkpointed", err)
		}
		if err := s.closeLocked(nil, checkpoint); err != nil || checkpoints != 0 || !group.done {
			t.Fatal("readonly retry checkpointed or failed to retire", err)
		}
		if err := s.closeLocked(nil, checkpoint); err != nil || checkpoints != 0 {
			t.Fatal(err)
		}
		if sharedCloseMayCheckpoint(true, keep) || sharedCloseMayCheckpoint(false, keep) != !keep {
			t.Fatal("close capability decision")
		}
	}
	s := &SharedSession{readOnly: true}
	if _, err := s.Begin(context.Background()); !errors.Is(err, ErrReadOnly) {
		t.Fatal("Begin reached descriptors", err)
	}
	if err := s.promote(context.Background(), nil); !errors.Is(err, ErrReadOnly) {
		t.Fatal("promotion reached descriptors", err)
	}
	if err := s.Checkpoint(context.Background()); !errors.Is(err, ErrReadOnly) {
		t.Fatal("checkpoint reached descriptors", err)
	}
	if err := s.poisonLocked(ErrChanged); !errors.Is(err, ErrPoisoned) || !errors.Is(err, ErrChanged) {
		t.Fatal("release poison policy", err)
	}
}

func TestSharedReadOnlySourceGuards(t *testing.T) {
	checks := map[string][]string{
		"shared_session.go": {
			"sharedReadOnlyOpenPolicy(runtime.GOOS, readOnly, opts)",
			"flags := sharedMainOpenFlags(readOnly, opts.Create)",
			"openWALFile(path+\"-wal\", sharedWALOpenFlags(readOnly), false)",
			"walindex.OpenReadOnly(path + \"-shm\")",
			"if !s.readOnly && (chosen == 0 || mark < h.MaxFrame)",
			"sharedCloseMayCheckpoint(s.readOnly, s.opts.KeepWALOnClose)",
			"for attempt := 0; attempt < 100; attempt++",
			"sharedRead{hdr: h, slot: chosen, minFrame: backfill + 1}",
			"return s.poisonLocked(sharedIndexError(e))",
		},
		"shared_index.go": {"if s.readOnly {", "s.index.Lock(walindex.WriteLock, 1, false)", "readonly WAL requires an initialized live index"},
		"shared_checkpoint.go": {"if s.readOnly {\n\t\treturn ErrReadOnly\n\t}"},
	}
	for path, needles := range checks {
		b, err := os.ReadFile(path)
		if err != nil { t.Fatal(err) }
		for _, needle := range needles {
			if !strings.Contains(string(b), needle) { t.Fatalf("%s missing %q", path, needle) }
		}
	}
}
