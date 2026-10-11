package wal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
)

// Every input here is an immutable byte slice or synthetic ReaderAt. There are
// no native files, recovery calls, induced process failures or physical I/O.
func TestPrefixProvenanceMemoryExactErrorOnly(t *testing.T) {
	l := newMemoryLog(t, MagicLittleEndian, 512)
	l.append(t, 1, 1, 11)
	l.append(t, 1, 0, 22)
	p, err := ScanPrefix(context.Background(), bytes.NewReader(l.bytes), int64(len(l.bytes)), Limits{})
	if !p.Allows(err, TailUncommitted) || p.Allows(err, TailSaltMismatch) || p.Allows(nil, TailUncommitted) {
		t.Fatal("wrong typed tail policy", err)
	}
	other := errors.New("joined I/O or retained ownership failure")
	for _, rejected := range []error{ErrTail, fmt.Errorf("%w", err), errors.Join(err), errors.Join(err, other), errors.Join(ErrTail, ErrUnsupported), &prefixCycle{}, prefixSpoof{}} {
		if p.Allows(rejected, TailUncommitted, TailSaltMismatch) {
			t.Fatal("admitted nonexact error", rejected)
		}
	}
	p2, e2 := ScanPrefix(context.Background(), bytes.NewReader(l.bytes), int64(len(l.bytes)), Limits{})
	if p.Allows(e2, TailUncommitted) || p2.Allows(err, TailUncommitted) {
		t.Fatal("cross-scan error admitted")
	}
	if (CommittedPrefix{}).Allows(nil) || (CommittedPrefix{}).Allows(err, TailUncommitted) {
		t.Fatal("zero provenance admitted")
	}
	s := p.Snapshot()
	s.Pages[1] = PageRef{Frame: 200, Offset: 9000}
	s.Header.PageSize, s.Complete, s.Tail = 1, true, TailClean
	fresh := p.Snapshot()
	if fresh.Pages[1].Frame != 1 || fresh.Header.PageSize != 512 || fresh.Complete || fresh.Tail != TailUncommitted {
		t.Fatal("snapshot mutation forged sealed prefix")
	}
	clean, ce := ScanPrefix(context.Background(), bytes.NewReader(l.bytes[:HeaderSize]), HeaderSize, Limits{})
	if ce != nil || !clean.Allows(nil) || clean.Allows(err, TailUncommitted) {
		t.Fatal("clean proof mismatch", ce)
	}
}

type prefixCycle struct{}

func (e *prefixCycle) Error() string { return "cyclic synthetic graph" }
func (e *prefixCycle) Unwrap() error { return e }

type prefixSpoof struct{}

func (prefixSpoof) Error() string { return "synthetic Is(ErrTail)" }
func (prefixSpoof) Is(error) bool { return true }

type prefixReadError struct {
	bytes []byte
	n     int
	err   error
}

func (r prefixReadError) ReadAt(dst []byte, _ int64) (int, error) {
	copy(dst, r.bytes)
	return r.n, r.err
}

func TestPrefixMemoryReadGraphsNeverAdmit(t *testing.T) {
	l := newMemoryLog(t, MagicBigEndian, 512)
	l.append(t, 1, 1, 1)
	l.append(t, 1, 0, 2)
	_, tail := ScanPrefix(context.Background(), bytes.NewReader(l.bytes), int64(len(l.bytes)), Limits{})
	ioFailure := errors.New("synthetic I/O")
	for _, n := range []int{0, HeaderSize - 1, HeaderSize} {
		for _, err := range []error{errors.Join(io.EOF, ioFailure), errors.Join(io.EOF, tail, ioFailure), tail, errors.Join(ErrTail, ErrUnsupported)} {
			r := prefixReadError{bytes: l.bytes, n: n, err: err}
			p, e := ScanPrefix(context.Background(), r, int64(len(l.bytes)), Limits{})
			if e == nil || p.Allows(e, TailUncommitted, TailSaltMismatch) || p.proof != nil {
				t.Fatal("reader graph gained prefix", n, e)
			}
			if !errors.Is(e, err) {
				t.Fatal("reader cause was erased", e, err)
			}
			if n != HeaderSize && !errors.Is(e, io.ErrUnexpectedEOF) {
				t.Fatal("missing short-read classification", e)
			}
		}
	}
}

func TestPrefixMemoryTaxonomyPreservesCauses(t *testing.T) {
	l := newMemoryLog(t, MagicLittleEndian, 512)
	l.append(t, 1, 1, 1)
	end := len(l.bytes)
	l.append(t, 1, 0, 2)
	invalid := append([]byte(nil), l.bytes...)
	invalid[len(invalid)-1] ^= 1
	salt := append([]byte(nil), l.bytes...)
	salt[end+8] ^= 1
	version := append([]byte(nil), l.bytes...)
	version[4] ^= 1
	for _, tc := range []struct {
		name   string
		b      []byte
		status TailStatus
		cause  error
		proof  bool
	}{
		{"pending", l.bytes, TailUncommitted, ErrTail, true},
		{"partial", l.bytes[:len(l.bytes)-1], TailIncomplete, io.ErrUnexpectedEOF, true},
		{"checksum", invalid, TailInvalid, ErrChecksum, true},
		{"salt", salt, TailSaltMismatch, ErrSalt, true},
		{"version", version, TailClean, ErrUnsupported, false},
		{"header", l.bytes[:HeaderSize-1], TailClean, ErrFormat, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, e := ScanPrefix(context.Background(), bytes.NewReader(tc.b), int64(len(tc.b)), Limits{})
			if !errors.Is(e, tc.cause) || (p.proof != nil) != tc.proof {
				t.Fatal("wrong provenance/cause", e)
			}
			if tc.proof && (!p.Allows(e, tc.status) || p.Snapshot().Tail != tc.status) {
				t.Fatal("typed status lost", e)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		ctx    context.Context
		limits Limits
		cause  error
	}{{ctx, Limits{}, context.Canceled}, {context.Background(), Limits{MaxWALBytes: 32}, ErrLimit}} {
		p, e := ScanPrefix(tc.ctx, bytes.NewReader(l.bytes), int64(len(l.bytes)), tc.limits)
		if p.proof != nil || !errors.Is(e, tc.cause) {
			t.Fatal("non-tail got provenance", e)
		}
	}
}
