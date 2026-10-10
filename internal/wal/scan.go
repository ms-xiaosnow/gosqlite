package wal

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// Limits bounds input, CPU work, and retained index entries. Zero selects the
// corresponding default. MaxIndexEntries counts committed and pending map
// entries together (a page present in both counts twice); page contents are
// never retained. One frame buffer of at most 65560 bytes is also allocated.
// MaxDBPages bounds both page numbers and nonzero commit database sizes.
// MaxIndexWork conservatively bounds frame-index inserts plus entries visited when
// promoting commits or applying shrink markers; repeated shrinks cannot cause
// unbounded quadratic work within the other limits. Map capacity is O(the
// entry limit), but this is not a byte-precise Go heap limit.
type Limits struct {
	MaxWALBytes     int64
	MaxFrames       uint32
	MaxIndexEntries uint32
	MaxDBPages      uint32
	MaxIndexWork    uint64
}

func (l Limits) normalized() (Limits, error) {
	if l.MaxWALBytes < 0 {
		return l, fmt.Errorf("%w: negative byte limit", ErrLimit)
	}
	if l.MaxWALBytes == 0 {
		l.MaxWALBytes = 1 << 30
	}
	if l.MaxFrames == 0 {
		l.MaxFrames = 1 << 20
	}
	if l.MaxIndexEntries == 0 {
		l.MaxIndexEntries = 1 << 20
	}
	if l.MaxDBPages == 0 {
		l.MaxDBPages = MaxPageNumber
	}
	if l.MaxDBPages > MaxPageNumber {
		return l, fmt.Errorf("%w: database page limit", ErrLimit)
	}
	if l.MaxIndexWork == 0 {
		l.MaxIndexWork = 16 << 20
	}
	return l, nil
}

// TailStatus classifies bytes after the last committed frame. A salt mismatch
// can be a normal leftover from a reset OR damage; it is never assumed safe.
type TailStatus uint8

const (
	TailClean TailStatus = iota
	TailUncommitted
	TailIncomplete
	TailInvalid
	TailSaltMismatch
)

func (t TailStatus) String() string {
	switch t {
	case TailClean:
		return "clean"
	case TailUncommitted:
		return "uncommitted"
	case TailIncomplete:
		return "incomplete"
	case TailInvalid:
		return "invalid"
	case TailSaltMismatch:
		return "salt-mismatch"
	default:
		return "unknown"
	}
}

// PageRef identifies a page's newest frame at or before the last commit.
// Frame is 1-based. Offset points to the page bytes, after the frame header.
type PageRef struct {
	Frame  uint32
	Offset int64
}

// Snapshot is the last committed database overlay, not the main database.
// Missing pages must be read from the paired database under appropriate locks.
// DBSize=0 and FrameCount=0 mean no transaction was committed in this WAL.
// EndOffset and Checksum describe the LAST COMMIT, or the header if none.
// IndexWork is the charged work through that commit, so writers can check that
// extending a scanned log will remain reopenable under their chosen limits.
// ValidFrameCount/ValidEndOffset may include an uncommitted suffix.
//
// Any non-nil Scan error forbids append, checkpoint, or exposing this snapshot
// as a successfully opened database. On ErrTail, the fields still identify the
// validated committed prefix for explicit higher-level policy/diagnostics;
// Scan NEVER silently truncates or recovers it. Complete is true only when
// scanning succeeded with no tail. The caller must keep the input immutable
// throughout scanning and use, including preventing WAL reset/truncation.
type Snapshot struct {
	Header          Header
	Pages           map[uint32]PageRef
	DBSize          uint32
	FrameCount      uint32
	Checksum        Checksum
	EndOffset       int64
	IndexWork       uint64
	ValidFrameCount uint32
	ValidEndOffset  int64
	Tail            TailStatus
	TailOffset      int64
	TailCause       error
	Complete        bool
}

// Scan validates the supplied immutable size-byte view of r using bounded
// ReaderAt calls. A header-only WAL is valid; zero bytes or a partial header
// are errors (the pager decides how to handle a separately absent/empty WAL).
//
// This deliberately uses a stricter policy than SQLite's recovery scanner:
// incomplete, invalid, salt-mismatching, and wholly valid uncommitted tails
// all return ErrTail. I/O failures, limits, and cancellation are distinct
// errors; they are never interpreted as a clean end or recoverable tail.
// Cancellation is checked between ReaderAt calls and during index work; a
// blocked ReaderAt cannot itself be interrupted through this interface.
func Scan(ctx context.Context, r io.ReaderAt, size int64, limits Limits) (Snapshot, error) {
	var s Snapshot
	if err := ctx.Err(); err != nil {
		return s, err
	}
	l, err := limits.normalized()
	if err != nil {
		return s, err
	}
	if size < HeaderSize {
		return s, fmt.Errorf("%w: incomplete header (%d bytes)", ErrFormat, size)
	}
	if size > l.MaxWALBytes {
		return s, fmt.Errorf("%w: %d WAL bytes", ErrLimit, size)
	}
	var header [HeaderSize]byte
	if err := readAt(r, header[:], 0); err != nil {
		return s, fmt.Errorf("read WAL header: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return s, err
	}
	h, err := ParseHeader(header[:])
	if err != nil {
		return s, err
	}
	frameSize := int64(FrameHeaderSize) + int64(h.PageSize)
	if (size-HeaderSize)/frameSize > int64(l.MaxFrames) {
		return s, fmt.Errorf("%w: frame count", ErrLimit)
	}
	s = Snapshot{Header: h, Pages: make(map[uint32]PageRef), Checksum: h.Checksum,
		EndOffset: HeaderSize, ValidEndOffset: HeaderSize}
	pending := make(map[uint32]PageRef)
	buf := make([]byte, int(frameSize))
	sum := h.Checksum
	var indexWork uint64
	var maxIndexedPage uint32
	for offset := int64(HeaderSize); offset < size; {
		if err := ctx.Err(); err != nil {
			return s, err
		}
		if size-offset < frameSize {
			return tail(s, TailIncomplete, offset, io.ErrUnexpectedEOF)
		}
		if err := readAt(r, buf, offset); err != nil {
			return s, fmt.Errorf("read WAL frame at %d: %w", offset, err)
		}
		if err := ctx.Err(); err != nil {
			return s, err
		}
		f, err := ParseFrame(h, buf[:FrameHeaderSize], buf[FrameHeaderSize:], sum)
		if err != nil {
			kind := TailInvalid
			if errors.Is(err, ErrSalt) {
				kind = TailSaltMismatch
			}
			return tail(s, kind, offset, err)
		}
		if f.PageNo > l.MaxDBPages || f.DBSize > l.MaxDBPages {
			return s, fmt.Errorf("%w: page number or database size", ErrLimit)
		}
		if _, exists := pending[f.PageNo]; !exists && uint64(len(pending))+uint64(len(s.Pages)) >= uint64(l.MaxIndexEntries) {
			return s, fmt.Errorf("%w: page index entries", ErrLimit)
		}
		if indexWork >= l.MaxIndexWork {
			return s, fmt.Errorf("%w: page index work", ErrLimit)
		}
		indexWork++
		s.ValidFrameCount++
		s.ValidEndOffset = offset + frameSize
		sum = f.Checksum
		pending[f.PageNo] = PageRef{Frame: s.ValidFrameCount, Offset: offset + FrameHeaderSize}
		if f.DBSize != 0 {
			// Reserve the complete commit's index work before mutating the
			// committed map. Index visits, including repeated shrink sweeps,
			// must fit a fixed budget independent of attacker-chosen DBSize.
			work := uint64(len(pending))
			if f.DBSize < maxIndexedPage {
				// Promotion may add new entries before the sweep. Charging
				// all pending entries again is a conservative upper bound.
				work += uint64(len(s.Pages)) + uint64(len(pending))
			}
			if work > l.MaxIndexWork-indexWork {
				return s, fmt.Errorf("%w: page index commit work", ErrLimit)
			}
			indexWork += work
			// Promote only completed transactions. Duplicate pages naturally
			// select the most recent frame, including padding commit frames.
			for pageNo, ref := range pending {
				if err := ctx.Err(); err != nil {
					return Snapshot{}, err
				}
				delete(pending, pageNo)
				if pageNo <= f.DBSize {
					s.Pages[pageNo] = ref
					maxIndexedPage = max(maxIndexedPage, pageNo)
				}
			}
			// Even a shrinking commit can skip the sweep when it does not
			// remove any actual indexed pages.
			if f.DBSize < maxIndexedPage {
				maxIndexedPage = 0
				for pageNo := range s.Pages {
					if err := ctx.Err(); err != nil {
						return Snapshot{}, err
					}
					if pageNo > f.DBSize {
						delete(s.Pages, pageNo)
					} else {
						maxIndexedPage = max(maxIndexedPage, pageNo)
					}
				}
			}
			s.DBSize = f.DBSize
			s.FrameCount = s.ValidFrameCount
			s.Checksum = sum
			s.EndOffset = s.ValidEndOffset
			s.IndexWork = indexWork
		}
		offset += frameSize
	}
	if s.ValidFrameCount != s.FrameCount {
		return tail(s, TailUncommitted, s.EndOffset, nil)
	}
	if err := ctx.Err(); err != nil {
		return s, err
	}
	s.Complete = true
	return s, nil
}

func tail(s Snapshot, status TailStatus, offset int64, cause error) (Snapshot, error) {
	s.Tail, s.TailOffset, s.TailCause = status, offset, cause
	if cause == nil {
		return s, fmt.Errorf("%w: %s at %d", ErrTail, status, offset)
	}
	return s, fmt.Errorf("%w: %s at %d: %w", ErrTail, status, offset, cause)
}

func readAt(r io.ReaderAt, b []byte, offset int64) error {
	n, err := r.ReadAt(b, offset)
	if n != len(b) {
		if err == nil || errors.Is(err, io.EOF) {
			return io.ErrUnexpectedEOF
		}
		return err
	}
	// ReaderAt permits a full final read to return EOF. Other errors are
	// propagated even if the reader supplied all requested bytes.
	if err == io.EOF {
		return nil
	}
	return err
}
