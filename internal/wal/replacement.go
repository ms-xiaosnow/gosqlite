package wal

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// PrefixReplacement is a bounded byte-construction recipe, not a recovery or
// publication capability. It can only be planned from ScanPrefix's exact
// wholly-valid uncommitted-tail result. The candidate is the exact committed
// prefix followed by complete duplicate commit frames through its last storage
// sector. Old and new WALs therefore describe the same committed database.
// Native locking, separate-inode identity, Sync, directory-anchored replace,
// directory Sync and retained-resource retirement MUST be supplied by a pager.
// The explicitly opted-in exclusive pager supplies that separate capability;
// its source implementation is not evidence of native crash qualification.
type PrefixReplacement struct {
	prefix CommittedPrefix
	end    int64
	extra  uint32
}

// PlanPrefixReplacement performs no I/O. Aligned commits and a zero-commit
// prefix still need a replacement: merely returning or truncating the old WAL
// would leave pending bytes reachable or overwrite an authoritative sector.
func PlanPrefixReplacement(prefix CommittedPrefix, scanError error, sector uint32) (PrefixReplacement, error) {
	if !prefix.Allows(scanError, TailUncommitted) || scanError == nil {
		return PrefixReplacement{}, errors.Join(ErrUnsupported, scanError)
	}
	if sector < 512 || sector > 65536 || sector&(sector-1) != 0 {
		return PrefixReplacement{}, fmt.Errorf("%w: replacement sector size", ErrUnsupported)
	}
	s := prefix.proof.snapshot
	frameSize := int64(FrameHeaderSize) + int64(s.Header.PageSize)
	boundary := s.EndOffset
	if s.FrameCount != 0 {
		boundary = (s.EndOffset + int64(sector) - 1) / int64(sector) * int64(sector)
	}
	extra := uint64((boundary - s.EndOffset + frameSize - 1) / frameSize)
	end := s.EndOffset + int64(extra)*frameSize
	// Match the native pager's default reopen budget, even when a caller used
	// looser Scan limits. Do this before any source read or candidate write.
	if uint64(s.FrameCount)+extra > 1<<20 || end > 1<<30 ||
		uint64(len(s.Pages))+1 > 1<<20 || s.IndexWork > 16<<20 || 2*extra > 16<<20-s.IndexWork {
		return PrefixReplacement{}, fmt.Errorf("%w: committed-prefix replacement", ErrLimit)
	}
	return PrefixReplacement{prefix: prefix, end: end, extra: uint32(extra)}, nil
}

// Size is the exact candidate length, or zero for an invalid recipe.
func (p PrefixReplacement) Size() int64 { return p.end }

// WriteCandidate writes a fresh, empty, separate candidate supplied by the
// caller. Source must remain immutable and must NOT alias destination. This
// utility neither opens files nor truncates, syncs, replaces, closes or exposes
// a live database. The exclusive pager supplies those retained-directory
// preconditions separately. Success proves byte construction only, never
// durable recovery. Caller retains both owners on any
// error; the complete read/write error graph is returned unchanged or joined.
func (p PrefixReplacement) WriteCandidate(ctx context.Context, source io.ReaderAt, destination io.WriterAt) (Snapshot, error) {
	if p.prefix.proof == nil || p.end < HeaderSize {
		return Snapshot{}, ErrUnsupported
	}
	s := p.prefix.Snapshot()
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	var last []byte
	var pageNo uint32
	if p.extra != 0 {
		last = make([]byte, FrameHeaderSize+int(s.Header.PageSize))
		if err := readAt(source, last, s.EndOffset-int64(len(last))); err != nil {
			return Snapshot{}, err
		}
		pageNo = binary.BigEndian.Uint32(last[:4])
		if binary.BigEndian.Uint32(last[4:8]) != s.DBSize || s.DBSize == 0 ||
			binary.BigEndian.Uint32(last[16:20]) != s.Checksum[0] || binary.BigEndian.Uint32(last[20:24]) != s.Checksum[1] {
			return Snapshot{}, fmt.Errorf("%w: committed-prefix source changed", ErrFormat)
		}
	}
	buf := make([]byte, 64<<10)
	for offset := int64(0); offset < s.EndOffset; {
		if err := ctx.Err(); err != nil {
			return Snapshot{}, err
		}
		n := min(int64(len(buf)), s.EndOffset-offset)
		if err := readAt(source, buf[:n], offset); err != nil {
			return Snapshot{}, err
		}
		if err := writeCandidateAt(destination, buf[:n], offset); err != nil {
			return Snapshot{}, err
		}
		offset += n
	}
	for i := uint32(0); i < p.extra; i++ {
		if err := ctx.Err(); err != nil {
			return Snapshot{}, err
		}
		frame, sum, err := EncodeFrame(s.Header, pageNo, s.DBSize, last[FrameHeaderSize:], s.Checksum)
		if err != nil {
			return Snapshot{}, err
		}
		if err := writeCandidateAt(destination, frame, s.EndOffset); err != nil {
			return Snapshot{}, err
		}
		s.FrameCount++
		if pageNo <= s.DBSize {
			s.Pages[pageNo] = PageRef{Frame: s.FrameCount, Offset: s.EndOffset + FrameHeaderSize}
		}
		s.EndOffset += int64(len(frame))
		s.Checksum = sum
		s.IndexWork += 2
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	s.ValidFrameCount, s.ValidEndOffset = s.FrameCount, s.EndOffset
	s.Tail, s.TailOffset, s.TailCause, s.Complete = TailClean, 0, nil, true
	return s, nil
}

func writeCandidateAt(w io.WriterAt, b []byte, offset int64) error {
	n, err := w.WriteAt(b, offset)
	if n != len(b) {
		return errors.Join(io.ErrShortWrite, err)
	}
	return err
}
