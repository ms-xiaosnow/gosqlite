package pager

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/internal/wal"
)

// Inert bytes only: no constructor, native factory, environment or fixture.
func walTailModelPage(size uint32, pages uint32) []byte {
	b := make([]byte, size)
	copy(b, []byte("SQLite format 3\x00"))
	encoded := uint16(size)
	if size == 65536 { encoded = 1 }
	binary.BigEndian.PutUint16(b[16:18], encoded)
	b[18], b[19], b[21], b[22], b[23] = 2, 2, 64, 32, 32
	binary.BigEndian.PutUint32(b[24:28], 1)
	binary.BigEndian.PutUint32(b[28:32], pages)
	binary.BigEndian.PutUint32(b[44:48], 4)
	binary.BigEndian.PutUint32(b[56:60], 1)
	binary.BigEndian.PutUint32(b[92:96], 1)
	return b
}
func walTailModelInput(t *testing.T, magic, page uint32, commits int) ([100]byte, []byte, *pendingWALTail) {
	t.Helper()
	var main [100]byte
	copy(main[:], walTailModelPage(page, 2))
	h := wal.Header{Magic: magic, Version: wal.Version, PageSize: page, Salt: [2]uint32{31, 47}}
	data, err := wal.EncodeHeader(h)
	if err != nil { t.Fatal(err) }
	h, err = wal.ParseHeader(data)
	if err != nil { t.Fatal(err) }
	sum := h.Checksum
	appendFrame := func(n, size uint32, b []byte) {
		frame, next, err := wal.EncodeFrame(h, n, size, b, sum)
		if err != nil { t.Fatal(err) }
		data, sum = append(data, frame...), next
	}
	if commits != 0 {
		appendFrame(1, 0, walTailModelPage(page, 3))
		appendFrame(3, 3, bytes.Repeat([]byte{3}, int(page)))
	}
	if commits > 1 { appendFrame(1, 1, walTailModelPage(page, 1)) }
	appendFrame(1, 0, bytes.Repeat([]byte{99}, int(page)))
	appendFrame(4, 0, bytes.Repeat([]byte{44}, int(page)))
	prefix, scan := wal.ScanPrefix(context.Background(), bytes.NewReader(data), int64(len(data)), wal.Limits{})
	if !exclusiveWALPrefixAllowed(prefix, scan) { t.Fatal(scan) }
	return main, data, &pendingWALTail{prefix: prefix, scanError: scan}
}

type walTailModelInfo int64
func (i walTailModelInfo) Name() string { return "memory" }
func (i walTailModelInfo) Size() int64 { return int64(i) }
func (walTailModelInfo) Mode() os.FileMode { return 0600 }
func (walTailModelInfo) ModTime() time.Time { return time.Time{} }
func (walTailModelInfo) IsDir() bool { return false }
func (walTailModelInfo) Sys() any { return nil }

type walTailModelPublisher struct {
	source, candidate []byte
	events []string
	fail map[string]error
	known, durable, transferred, short bool
	afterPrepare context.CancelFunc
}
func (m *walTailModelPublisher) event(name string) error { m.events = append(m.events, name); return m.fail[name] }
func (m *walTailModelPublisher) Source() io.ReaderAt { return bytes.NewReader(m.source) }
func (m *walTailModelPublisher) Acquire() error { return m.event("acquire") }
func (m *walTailModelPublisher) CreateCandidate(int64) error { return m.event("create") }
func (m *walTailModelPublisher) WriteAt(b []byte, off int64) (int, error) {
	err := m.event("write")
	n := len(b)
	if m.short { n /= 2 }
	if end := int(off)+n; end > len(m.candidate) { m.candidate = append(m.candidate, make([]byte, end-len(m.candidate))...) }
	copy(m.candidate[int(off):], b[:n])
	return n, err
}
func (m *walTailModelPublisher) Prepare(size int64) error {
	if int64(len(m.candidate)) != size { return ErrCorrupt }
	err := m.event("prepare")
	if m.afterPrepare != nil { m.afterPrepare() }
	return err
}
func (m *walTailModelPublisher) Replace() (bool, error) { return m.known, m.event("replace") }
func (m *walTailModelPublisher) FinishPublication() (os.FileInfo, error) {
	post := m.event("post")
	barrier := m.event("directory-sync")
	m.durable = m.known && m.transferred && post == nil && barrier == nil
	return walTailModelInfo(len(m.candidate)), errors.Join(post, barrier)
}
func (m *walTailModelPublisher) Durable() bool { return m.durable }
func (m *walTailModelPublisher) transfer() error {
	if err := m.event("transfer"); err != nil { return err }
	m.transferred = true
	return nil
}
func TestWALTailPublicationMemoryCommittedViews(t *testing.T) {
	for _, magic := range []uint32{wal.MagicLittleEndian, wal.MagicBigEndian} {
		for _, page := range []uint32{512, 4096, 65536} {
			for _, sector := range []uint32{512, 4096, 65536} {
				for _, commits := range []int{0, 1, 2} {
					t.Run(fmt.Sprintf("%x/%d/%d/%d", magic, page, sector, commits), func(t *testing.T) {
						raw, data, pending := walTailModelInput(t, magic, page, commits)
						original := append([]byte(nil), data...)
						header, plan, err := prepareWALTailView(raw, int64(page)*2, int64(len(data)), bytes.NewReader(data), pending, sector)
						if err != nil { t.Fatal(err) }
						wantPages := uint32(2)
						if commits == 1 { wantPages = 3 }
						if commits == 2 { wantPages = 1 }
						if header.PageCount != wantPages { t.Fatal("pending/shrunk extent exposed", header) }
						m := &walTailModelPublisher{source: data, fail: make(map[string]error), known: true}
						clean, info, err := runWALTailPublication(context.Background(), plan, m, m.transfer)
						if err != nil || info.Size() != plan.Size() || !clean.Complete { t.Fatal(err) }
						scanned, err := wal.Scan(context.Background(), bytes.NewReader(m.candidate), int64(len(m.candidate)), wal.Limits{})
						if err != nil || !reflect.DeepEqual(scanned, clean) || !bytes.Equal(data, original) { t.Fatal("constructed view mismatch", err) }
						before := pending.prefix.Snapshot()
						for n, old := range before.Pages {
							ref := clean.Pages[n]
							if !bytes.Equal(data[old.Offset:old.Offset+int64(page)], m.candidate[ref.Offset:ref.Offset+int64(page)]) { t.Fatal("committed page changed", n) }
						}
						if commits == 0 && plan.Size() != wal.HeaderSize { t.Fatal("zero-commit tail retained") }
						if _, ok := clean.Pages[4]; ok { t.Fatal("pending-only extent resurrected") }
					})
				}
			}
		}
	}
}
func TestWALTailPublicationMemoryAdmissionBeforeAcquisition(t *testing.T) {
	raw, data, pending := walTailModelInput(t, wal.MagicLittleEndian, 512, 1)
	for _, edit := range []func(*[100]byte){
		func(h *[100]byte) { h[0] = 0 }, func(h *[100]byte) { h[18] = 1 },
		func(h *[100]byte) { h[17] = 1 }, func(h *[100]byte) { h[20] = 1 },
		func(h *[100]byte) { h[56+3] = 2 }, func(h *[100]byte) { h[52+3] = 1 },
	} {
		bad := raw
		edit(&bad)
		if _, _, err := prepareWALTailView(bad, 1024, int64(len(data)), bytes.NewReader(data), pending, 4096); err == nil { t.Fatal("bad physical header admitted") }
	}
	for _, size := range []int64{0, 100, 1023, 1025} {
		if _, _, err := prepareWALTailView(raw, size, int64(len(data)), bytes.NewReader(data), pending, 4096); err == nil { t.Fatal("physical extent admitted", size) }
	}
	for _, scan := range []error{nil, wal.ErrTail, errors.Join(pending.scanError), errors.Join(pending.scanError, io.ErrClosedPipe)} {
		bad := &pendingWALTail{prefix: pending.prefix, scanError: scan}
		if _, _, err := prepareWALTailView(raw, 1024, int64(len(data)), bytes.NewReader(data), bad, 4096); !errors.Is(err, ErrUnsupported) { t.Fatal("wrong scan identity", err) }
	}
	if _, _, err := prepareWALTailView(raw, 1024, int64(len(data))-1, bytes.NewReader(data), pending, 4096); !errors.Is(err, ErrChanged) { t.Fatal(err) }
	view := pending.prefix.Snapshot()
	delete(view.Pages, 3)
	if _, err := validateWALView(raw, 1024, view, true, bytes.NewReader(data), true); !errors.Is(err, ErrCorrupt) { t.Fatal("pending page satisfied committed gap", err) }
	view = pending.prefix.Snapshot()
	view.Header.PageSize = 1024
	if _, err := validateWALView(raw, 1024, view, true, bytes.NewReader(data), true); !errors.Is(err, ErrCorrupt) { t.Fatal("WAL page size mismatch", err) }
	view = pending.prefix.Snapshot()
	badPage := append([]byte(nil), data...)
	badPage[view.Pages[1].Offset+18] = 1
	if _, err := validateWALView(raw, 1024, view, true, bytes.NewReader(badPage), true); err == nil { t.Fatal("bad logical page one admitted") }
}
func TestWALTailPublicationMemoryFailuresAndCancellation(t *testing.T) {
	raw, data, pending := walTailModelInput(t, wal.MagicLittleEndian, 512, 1)
	_, plan, err := prepareWALTailView(raw, 1024, int64(len(data)), bytes.NewReader(data), pending, 4096)
	if err != nil { t.Fatal(err) }
	cause := errors.New("inert orchestration failure")
	for _, phase := range []string{"acquire", "create", "write", "prepare", "replace", "transfer", "post", "directory-sync"} {
		m := &walTailModelPublisher{source: data, fail: map[string]error{phase: cause}, known: phase != "replace"}
		clean, info, err := runWALTailPublication(context.Background(), plan, m, m.transfer)
		if !errors.Is(err, cause) || clean.Complete || info != nil { t.Fatal("failure published state", phase, err) }
		late := phase == "replace" || phase == "transfer" || phase == "post" || phase == "directory-sync"
		if late && (!errors.Is(err, ErrCommitUncertain) || m.events[len(m.events)-1] != "directory-sync") { t.Fatal("late failure lost barrier", m.events, err) }
	}
	for _, full := range []bool{false, true} {
		m := &walTailModelPublisher{source: data, fail: map[string]error{"write": cause}, known: true, short: !full}
		_, _, err := runWALTailPublication(context.Background(), plan, m, m.transfer)
		if !errors.Is(err, cause) || !full && !errors.Is(err, io.ErrShortWrite) { t.Fatal(err) }
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := &walTailModelPublisher{source: data, known: true}
	if _, _, err := runWALTailPublication(ctx, plan, m, m.transfer); !errors.Is(err, context.Canceled) || len(m.events) != 0 { t.Fatal(err, m.events) }
	ctx, cancel = context.WithCancel(context.Background())
	m = &walTailModelPublisher{source: data, known: true, afterPrepare: cancel}
	if clean, _, err := runWALTailPublication(ctx, plan, m, m.transfer); err != nil || !clean.Complete || m.events[len(m.events)-1] != "directory-sync" { t.Fatal("late cancel skipped finite barrier", err, m.events) }
}

func TestWALTailPublicationMemoryAlignedCommitStillReplaces(t *testing.T) {
	page := walTailModelPage(512, 2)
	var raw [100]byte
	copy(raw[:], page)
	h := wal.Header{Magic: wal.MagicLittleEndian, Version: wal.Version, PageSize: 512}
	data, err := wal.EncodeHeader(h)
	if err != nil { t.Fatal(err) }
	h, err = wal.ParseHeader(data)
	if err != nil { t.Fatal(err) }
	sum := h.Checksum
	for i := 0; i < 20; i++ {
		frame, next, err := wal.EncodeFrame(h, 1, 2, page, sum)
		if err != nil { t.Fatal(err) }
		data, sum = append(data, frame...), next
	}
	committed := len(data)
	frame, _, err := wal.EncodeFrame(h, 1, 0, bytes.Repeat([]byte{99}, 512), sum)
	if err != nil { t.Fatal(err) }
	data = append(data, frame...)
	prefix, scan := wal.ScanPrefix(context.Background(), bytes.NewReader(data), int64(len(data)), wal.Limits{})
	_, plan, err := prepareWALTailView(raw, 1024, int64(len(data)), bytes.NewReader(data), &pendingWALTail{prefix: prefix, scanError: scan}, 512)
	if err != nil || committed%512 != 0 || plan.Size() != int64(committed) { t.Fatal("aligned model", err) }
	m := &walTailModelPublisher{source: data, known: true}
	clean, _, err := runWALTailPublication(context.Background(), plan, m, m.transfer)
	if err != nil || !clean.Complete || !m.transferred || !m.durable || len(m.candidate) != committed { t.Fatal("aligned tail skipped replacement", err, m.events) }
}
