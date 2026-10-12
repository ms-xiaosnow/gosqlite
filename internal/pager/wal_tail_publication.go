package pager

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ms-xiaosnow/gosqlite/internal/wal"
)

type pendingWALTail struct {
	prefix wal.CommittedPrefix
	scanError error // identical scanner error, never wrapped before Plan
}

// validateWALView reads only a prospective committed page-one overlay. The
// strict tail branch validates the complete physical mode-2 header too; legacy
// clean/mode-transition behavior retains its previous logical-header policy.
func validateWALView(raw [100]byte, size int64, snapshot wal.Snapshot, hasWAL bool, source io.ReaderAt, strictPhysical bool) (Header, error) {
	if raw[18] != 2 || raw[19] != 2 { return Header{}, fmt.Errorf("%w: exclusive WAL requires mode-2 database", ErrUnsupported) }
	if strictPhysical {
		physical, err := parseHeader(raw[:], size, true, 0)
		if err != nil { return Header{}, err }
		if err = writableHeader(physical); err != nil { return Header{}, err }
	}
	pageOne := make([]byte, 100)
	copy(pageOne, raw[:])
	if ref, ok := snapshot.Pages[1]; ok {
		if source == nil { return Header{}, ErrCorrupt }
		n, err := readNativeAt(source, pageOne, ref.Offset)
		if n != len(pageOne) { return Header{}, errors.Join(ErrCorrupt, io.ErrUnexpectedEOF, err) }
		if err != nil { return Header{}, err }
	}
	h, err := parseHeader(pageOne, size, true, snapshot.DBSize)
	if err != nil { return Header{}, err }
	if h.WriteVersion != 2 || h.ReadVersion != 2 { return Header{}, fmt.Errorf("%w: WAL page one must remain mode 2", ErrCorrupt) }
	mainPageSize := uint32(binary.BigEndian.Uint16(raw[16:18]))
	if mainPageSize == 1 { mainPageSize = 65536 }
	if !bytes.Equal(raw[:16], []byte("SQLite format 3\x00")) || mainPageSize != h.PageSize { return Header{}, fmt.Errorf("%w: physical and logical database headers disagree", ErrCorrupt) }
	if hasWAL && snapshot.Header.PageSize != h.PageSize { return Header{}, fmt.Errorf("%w: WAL and database page sizes differ", ErrCorrupt) }
	if err = writableHeader(h); err != nil { return Header{}, err }
	physical := uint32(size / int64(h.PageSize))
	if uint64(h.PageCount) > uint64(physical)+uint64(len(snapshot.Pages))+1 { return Header{}, fmt.Errorf("%w: WAL appended-page coverage", ErrCorrupt) }
	for n := uint64(physical)+1; n <= uint64(h.PageCount); n++ {
		if uint32(n) == lockPage(h.PageSize) { continue }
		if _, ok := snapshot.Pages[uint32(n)]; !ok { return Header{}, fmt.Errorf("%w: WAL has no image for appended page %d", ErrCorrupt, n) }
	}
	return h, nil
}
func prepareWALTailView(raw [100]byte, size, walSize int64, source io.ReaderAt, pending *pendingWALTail, sector uint32) (Header, wal.PrefixReplacement, error) {
	if pending == nil || !exclusiveWALPrefixAllowed(pending.prefix, pending.scanError) { return Header{}, wal.PrefixReplacement{}, ErrUnsupported }
	view := pending.prefix.Snapshot()
	if walSize <= view.EndOffset || view.ValidEndOffset != walSize { return Header{}, wal.PrefixReplacement{}, ErrChanged }
	validated, err := validateWALView(raw, size, view, true, source, true)
	if err != nil { return Header{}, wal.PrefixReplacement{}, err }
	plan, err := wal.PlanPrefixReplacement(pending.prefix, pending.scanError, sector)
	if err != nil {
		if errors.Is(err, wal.ErrLimit) { err = errors.Join(ErrLimit, err) }
		return Header{}, wal.PrefixReplacement{}, err
	}
	return validated, plan, nil
}

type walTailPublisher interface {
	io.WriterAt
	Source() io.ReaderAt
	Acquire() error
	CreateCandidate(int64) error
	Prepare(int64) error
	Replace() (bool, error)
	FinishPublication() (os.FileInfo, error)
	Durable() bool
}

// runWALTailPublication is the production orchestration boundary used by inert
// models. The installed owner is supplied by the caller; this function never
// constructs native factories, retains callbacks, or exposes a pending view.
func runWALTailPublication(ctx context.Context, plan wal.PrefixReplacement, owner walTailPublisher, transfer func() error) (wal.Snapshot, os.FileInfo, error) {
	fail := func(err error) (wal.Snapshot, os.FileInfo, error) { return wal.Snapshot{}, nil, err }
	if err := ctx.Err(); err != nil { return fail(err) }
	if err := owner.Acquire(); err != nil { return fail(nativeFileError(err)) }
	if err := owner.CreateCandidate(plan.Size()); err != nil { return fail(nativeFileError(err)) }
	clean, err := plan.WriteCandidate(ctx, owner.Source(), owner)
	if err != nil { return fail(nativeFileError(err)) }
	if !clean.Complete || clean.Tail != wal.TailClean || clean.EndOffset != plan.Size() { return fail(ErrCorrupt) }
	if err = ctx.Err(); err != nil { return fail(err) }
	if err = owner.Prepare(plan.Size()); err != nil { return fail(nativeFileError(err)) }
	known, replacement := owner.Replace()
	var exchange error
	if known { exchange = transfer() }
	info, barrier := owner.FinishPublication()
	if err = errors.Join(replacement, exchange, barrier); err != nil { return fail(errors.Join(ErrCommitUncertain, nativeFileError(err))) }
	if !known || !owner.Durable() || info == nil || info.Size() != plan.Size() { return fail(errors.Join(ErrCommitUncertain, ErrPoisoned)) }
	return clean, info, nil
}

// Constructor-only/private Session serialization. The original main EXCLUSIVE
// lock survives acquisition, rename, transfer, SHM, directory Sync and retirement.
func (s *Session) publishWALTailLocked(ctx context.Context, pending *pendingWALTail, validated Header, plan wal.PrefixReplacement) error {
	owner, err := s.cleanup.reserveWALPublication(s.file, s.walFile, s.info, s.walInfo)
	if err != nil { return err }
	clean, info, operation := runWALTailPublication(ctx, plan, owner, func() error {
		current, err := s.cleanup.transferWALPublication(owner, s.walFile)
		if err == nil { s.walFile = current }
		return err
	})
	// Any diagnostic fails Open, even if all resources were consumed. A pending
	// child makes childClosePending suppress the constructor's duplicate attempt.
	retirement := s.cleanup.closeTemporary(operation)
	if retirement != nil { return errors.Join(ErrPoisoned, retirement, pending.scanError) }
	if operation != nil || !owner.RetirementComplete() { return errors.Join(ErrPoisoned, operation, pending.scanError) }
	// The only publication of a clean logical state is after every barrier and
	// the superseded-child-before-directory retirement completed successfully.
	s.walInfo, s.header, s.snapshot, s.appendSafe = info, validated, clean, true
	return nil
}
