//go:build linux && manual_checkpoint_restart

package checkpointrestart

// SOURCE_ONLY / NOT_RUN. This one-purpose bridge does not authorize execution.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

const ManualFrameLimit = 4096
const ManualFrameCount = 64
const ManualExitCode = 86
const manualAbortCode = 87
const ManualFixture = FixtureID

type ManualBinding struct {
	Schema, Trial, Source, Compiler, CompileReceipt, Binary, Fixture, Profile, Case, Role, Generation, Challenge string
}
type ManualPage struct {
	Page, Selected, DBSize, Frames uint32
	Salt, Checksum                 [2]uint32
	Safe, Backfill, Copied         uint32
}
type ManualRoot struct {
	OldHash, CandidateHash [32]byte
	Old, Candidate         encryption.Header
}
type ManualObservation struct {
	Page ManualPage
	Root ManualRoot
}
type ManualHead struct {
	Ledger                                 [32]byte
	Page                                   ManualPage
	OldRootHash, AuthenticatedHeadCommitID [32]byte
	OldRoot                                encryption.Header
}
type ManualFrame struct {
	Binding     ManualBinding
	Sequence    uint8
	Kind        string
	Point       uint8
	Observation ManualObservation
	PayloadHash [32]byte
}

func ManualCase(caseName string) (profile string, first, last, target uint8, ok bool) {
	switch caseName {
	case "exclusive-control":
		return "linux-plain-exclusive-4096", 1, 2, 0, true
	case "exclusive-after-first-main-copy":
		return "linux-plain-exclusive-4096", 1, 2, 1, true
	case "exclusive-after-wal-unlink":
		return "linux-plain-exclusive-4096", 1, 2, 2, true
	case "shared-control":
		return "linux-plain-shared-4096", 3, 4, 0, true
	case "shared-after-first-main-copy":
		return "linux-plain-shared-4096", 3, 4, 3, true
	case "shared-after-backfill":
		return "linux-plain-shared-4096", 3, 4, 4, true
	case "encrypted-control":
		return "linux-encrypted-layout2-rawkey256-4096", 5, 7, 0, true
	case "encrypted-before-root-rename":
		return "linux-encrypted-layout2-rawkey256-4096", 5, 7, 5, true
	case "encrypted-after-root-rename":
		return "linux-encrypted-layout2-rawkey256-4096", 5, 7, 6, true
	case "encrypted-after-root-dirsync":
		return "linux-encrypted-layout2-rawkey256-4096", 5, 7, 7, true
	default:
		return "", 0, 0, 0, false
	}
}
func manualBindingValid(b ManualBinding) bool {
	profile, _, _, _, ok := ManualCase(b.Case)
	generation := ""
	switch b.Role {
	case "writer":
		generation = "2"
	case "probe":
		generation = "3"
	}
	return ok && b.Schema == "checkpoint-restart-frame-v1" && b.Profile == profile && b.Fixture == ManualFixture && b.Trial != "" && len(b.Trial) <= 64 && len(b.Source) == 64 && len(b.Compiler) == 64 && len(b.CompileReceipt) == 64 && len(b.Binary) == 64 && len(b.Challenge) == 64 && generation != "" && b.Generation == generation
}

type ManualChannel struct {
	binding         ManualBinding
	control, events *os.File
	sequence        uint8
	closed          bool
}

var manualFailedChannels []*ManualChannel

func ManualChildChannel(binding ManualBinding) (*ManualChannel, error) {
	if !manualBindingValid(binding) {
		return nil, errors.New("checkpoint binding refused")
	}
	control := os.NewFile(3, "checkpoint-control")
	events := os.NewFile(4, "checkpoint-events")
	channel := &ManualChannel{binding: binding, control: control, events: events}
	if control == nil || events == nil {
		manualFailedChannels = append(manualFailedChannels, channel)
		return nil, errors.New("missing fixed pipes")
	}
	ci, ce := control.Stat()
	ei, ee := events.Stat()
	if ce != nil || ee != nil || ci.Mode()&os.ModeNamedPipe == 0 || ei.Mode()&os.ModeNamedPipe == 0 || os.SameFile(ci, ei) {
		manualFailedChannels = append(manualFailedChannels, channel)
		return nil, errors.Join(ce, ee, errors.New("distinct fixed pipes required"))
	}
	return channel, nil
}
func ManualEncodeFrame(frame ManualFrame) ([]byte, error) {
	body, err := json.Marshal(frame)
	if err != nil || !manualBindingValid(frame.Binding) || len(body) == 0 || len(body) > ManualFrameLimit || frame.Sequence == 0 || frame.Sequence > ManualFrameCount {
		return nil, errors.New("bounded frame refused")
	}
	raw := make([]byte, len(body)+4)
	binary.BigEndian.PutUint32(raw[:4], uint32(len(body)))
	copy(raw[4:], body)
	return raw, nil
}
func ManualDecodeFrame(raw []byte) (ManualFrame, error) {
	var frame ManualFrame
	if len(raw) < 5 || len(raw) > ManualFrameLimit+4 || int(binary.BigEndian.Uint32(raw[:4])) != len(raw)-4 {
		return frame, errors.New("frame extent refused")
	}
	d := json.NewDecoder(bytes.NewReader(raw[4:]))
	d.DisallowUnknownFields()
	if err := d.Decode(&frame); err != nil {
		return frame, err
	}
	canonical, err := ManualEncodeFrame(frame)
	if err != nil || !bytes.Equal(canonical, raw) {
		return frame, errors.New("noncanonical complete frame required")
	}
	return frame, nil
}
func ManualReadFrame(reader *os.File) (ManualFrame, []byte, error) {
	var prefix [4]byte
	if _, err := io.ReadFull(reader, prefix[:]); err != nil {
		return ManualFrame{}, nil, err
	}
	n := binary.BigEndian.Uint32(prefix[:])
	if n == 0 || n > ManualFrameLimit {
		return ManualFrame{}, nil, errors.New("frame bound")
	}
	raw := make([]byte, int(n)+4)
	copy(raw, prefix[:])
	if _, err := io.ReadFull(reader, raw[4:]); err != nil {
		return ManualFrame{}, nil, err
	}
	frame, err := ManualDecodeFrame(raw)
	return frame, raw, err
}
func ManualWriteFrame(writer *os.File, frame ManualFrame) ([]byte, error) {
	raw, err := ManualEncodeFrame(frame)
	if err != nil {
		return nil, err
	}
	n, err := writer.Write(raw)
	if err != nil || n != len(raw) {
		return nil, errors.Join(err, io.ErrShortWrite)
	}
	return raw, nil
}
func (c *ManualChannel) Exchange(kind string, point uint8, observation ManualObservation, payload [32]byte) (string, error) {
	if c == nil || c.closed || c.sequence >= ManualFrameCount {
		return "", errors.New("channel exhausted")
	}
	c.sequence++
	frame := ManualFrame{Binding: c.binding, Sequence: c.sequence, Kind: kind, Point: point, Observation: observation, PayloadHash: payload}
	raw, err := ManualWriteFrame(c.events, frame)
	if err != nil {
		return "", err
	}
	grant, _, err := ManualReadFrame(c.control)
	if err != nil || grant.Binding != frame.Binding || grant.Sequence != frame.Sequence || grant.Point != frame.Point || grant.Observation != frame.Observation || grant.PayloadHash != sha256.Sum256(raw) {
		return "", errors.Join(err, errors.New("causal grant refused"))
	}
	if grant.Kind != "continue" && grant.Kind != "exit" && grant.Kind != "done" {
		return "", errors.New("grant kind refused")
	}
	return grant.Kind, nil
}
func (c *ManualChannel) Close() error {
	if c == nil || c.closed {
		return errors.New("channel close repeated")
	}
	c.closed = true
	return errors.Join(c.control.Close(), c.events.Close())
}

type manualContextKey struct{}
type manualScope struct {
	mu                        sync.Mutex
	path                      string
	head                      ManualHead
	channel                   *ManualChannel
	first, last, target, seen uint8
	copies, lastPage          uint32
	observation               ManualObservation
}

func ManualContext(ctx context.Context, path string, head ManualHead, channel *ManualChannel) (context.Context, error) {
	if ctx == nil || ctx.Value(manualContextKey{}) != nil || path == "" || head.Ledger == [32]byte{} || channel == nil || channel.binding.Role != "writer" || channel.sequence != 2 {
		return nil, errors.New("single checkpoint writer purpose refused")
	}
	_, first, last, target, ok := ManualCase(channel.binding.Case)
	if !ok {
		return nil, errors.New("case refused")
	}
	if first < 5 {
		p := head.Page
		if p.Selected < 2 || p.Selected > 4096 || p.DBSize < 1 || p.DBSize > 4096 || p.Frames < 2 || p.Frames > 4096 || p.Page != 0 || p.Copied != 0 || head.OldRoot != (encryption.Header{}) || head.OldRootHash != [32]byte{} || head.AuthenticatedHeadCommitID != [32]byte{} {
			return nil, errors.New("plain committed head refused")
		}
		if first == 1 && (p.Safe != 0 || p.Backfill != 0) || first == 3 && (p.Safe != p.Frames || p.Backfill >= p.Safe) {
			return nil, errors.New("checkpoint authority refused")
		}
	} else if head.Page != (ManualPage{}) || head.OldRootHash == [32]byte{} || head.AuthenticatedHeadCommitID == [32]byte{} || head.OldRoot.Descriptor.CommitSequence != 4 || head.OldRoot.Descriptor.PageSize != 4096 || head.OldRoot.Descriptor.PageCount < 1 || head.OldRoot.Descriptor.PageCount > 4096 || head.OldRoot.Epoch.Kind != encryption.RawKey256 {
		return nil, errors.New("authenticated layout2 head refused")
	}
	return context.WithValue(ctx, manualContextKey{}, &manualScope{path: path, head: head, channel: channel, first: first, last: last, target: target}), nil
}
func ManualArmed(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	_, ok := ctx.Value(manualContextKey{}).(*manualScope)
	return ok
}
func ManualAbort() { os.Exit(manualAbortCode) }
func manualBarrier(scope *manualScope, point uint8, observation ManualObservation) {
	grant, err := scope.channel.Exchange("checkpoint-observation", point, observation, scope.head.Ledger)
	if err != nil {
		ManualAbort()
	}
	if scope.target == point {
		if grant != "exit" {
			ManualAbort()
		}
		os.Exit(ManualExitCode)
	}
	if grant != "continue" {
		ManualAbort()
	}
}
func ManualPageObserve(ctx context.Context, point uint8, path string, page ManualPage) {
	if !ManualArmed(ctx) {
		return
	}
	scope := ctx.Value(manualContextKey{}).(*manualScope)
	scope.mu.Lock()
	base := page
	base.Page, base.Copied = 0, 0
	valid := scope.first < 5 && path == scope.path && base == scope.head.Page
	emit := false
	if point == scope.first {
		valid = valid && scope.seen <= 1 && page.Page > scope.lastPage && page.Page <= page.DBSize && scope.copies < page.Selected && scope.copies < 4096
		if valid {
			scope.copies++
			scope.lastPage = page.Page
			page.Copied = scope.copies
			if scope.copies == 1 {
				scope.seen = 1
				emit = true
			}
		}
	} else {
		valid = valid && point == scope.last && scope.seen == 1 && page.Page == 0 && scope.copies == page.Selected
		if valid {
			scope.seen = 2
			page.Copied = scope.copies
			emit = true
		}
	}
	observation := ManualObservation{Page: page}
	if valid && emit {
		scope.observation = observation
	}
	scope.mu.Unlock()
	if !valid {
		ManualAbort()
	}
	if emit {
		manualBarrier(scope, point, observation)
	}
}
func ManualRootValid(root ManualRoot, head ManualHead) bool {
	a, b := root.Old, root.Candidate
	return root.OldHash == head.OldRootHash && a == head.OldRoot && root.CandidateHash != [32]byte{} && root.CandidateHash != root.OldHash && b.Validate(encryption.Limits{}) == nil && a.Descriptor.Role == encryption.FileRoot && b.Descriptor.Role == encryption.FileRoot && b.Epoch == a.Epoch && b.Epoch.Kind == encryption.RawKey256 && b.FileID != a.FileID && b.Descriptor.PageSize == 4096 && b.Descriptor.PageCount == a.Descriptor.PageCount && b.Descriptor.RecordCount == 1 && b.Descriptor.CommitSequence == 5 && b.Descriptor.ParentCommitID == head.AuthenticatedHeadCommitID
}
func ManualRootObserve(ctx context.Context, point uint8, path string, root ManualRoot) {
	if !ManualArmed(ctx) {
		return
	}
	scope := ctx.Value(manualContextKey{}).(*manualScope)
	scope.mu.Lock()
	valid := scope.first == 5 && path == scope.path && point == scope.first+scope.seen && point <= scope.last && ManualRootValid(root, scope.head)
	if scope.seen != 0 {
		valid = valid && root == scope.observation.Root
	}
	if valid {
		scope.seen++
		scope.observation = ManualObservation{Root: root}
	}
	observation := scope.observation
	scope.mu.Unlock()
	if !valid {
		ManualAbort()
	}
	manualBarrier(scope, point, observation)
}
func ManualReturnedNil(ctx context.Context) error {
	if !ManualArmed(ctx) {
		return errors.New("public checkpoint return without purpose")
	}
	scope := ctx.Value(manualContextKey{}).(*manualScope)
	scope.mu.Lock()
	valid := scope.target == 0 && scope.seen == scope.last-scope.first+1
	observation := scope.observation
	scope.mu.Unlock()
	if !valid {
		return errors.New("public checkpoint return without complete controls")
	}
	grant, err := scope.channel.Exchange("public-checkpoint-nil", 0, observation, scope.head.Ledger)
	if err != nil || grant != "continue" {
		return errors.Join(err, errors.New("nil return grant refused"))
	}
	return nil
}

// Explicit value-only mirror adapters. Source guards freeze every field; no
// JSON round-trip silently drops public fields or changes their storage type.
func ManualModelHeader(h encryption.Header) RootHeader {
	return RootHeader{Major: h.Major, Minor: h.Minor, Suite: h.Suite, Flags: h.Flags,
		Epoch:  Epoch{DatabaseID: h.Epoch.DatabaseID, EpochID: h.Epoch.EpochID, Kind: uint8(h.Epoch.Kind), KDF: KDFParams{MemoryKiB: h.Epoch.KDF.MemoryKiB, Passes: h.Epoch.KDF.Passes, Parallelism: h.Epoch.KDF.Parallelism}, KDFSalt: h.Epoch.KDFSalt, RootSalt: h.Epoch.RootSalt},
		FileID: h.FileID, NoncePrefix: h.NoncePrefix,
		Descriptor: Descriptor{Role: uint8(h.Descriptor.Role), PageSize: h.Descriptor.PageSize, PageCount: h.Descriptor.PageCount, RecordCount: h.Descriptor.RecordCount, CommitSequence: h.Descriptor.CommitSequence, TransactionID: h.Descriptor.TransactionID, ParentCommitID: h.Descriptor.ParentCommitID}}
}
func ManualModelObservation(point uint8, o ManualObservation) Observation {
	if point >= 5 {
		return Observation{Point: Point(point), OldRootHash: o.Root.OldHash, CandidateRootHash: o.Root.CandidateHash, OldHeader: ManualModelHeader(o.Root.Old), CandidateHeader: ManualModelHeader(o.Root.Candidate), CandidateBytes: 1064}
	}
	p := o.Page
	x := Observation{Point: Point(point), Page: p.Page, SelectedCount: p.Selected, CopiedCount: p.Copied, DBSize: p.DBSize, FrameCount: p.Frames, Salt: p.Salt, Checksum: p.Checksum, Safe: p.Safe, OldBackfill: p.Backfill}
	if point == 3 || point == 4 {
		x.Attempted = p.Safe
		x.Backfill = p.Backfill
		if point == 4 {
			x.Backfill = p.Safe
		}
	}
	return x
}
