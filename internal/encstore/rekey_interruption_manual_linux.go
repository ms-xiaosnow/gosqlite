//go:build linux && manual_rekey_interruption

package encstore

// SOURCE_ONLY. No native scenario has been executed or authorized by this file.
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
	"sync/atomic"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

const ManualRekeyFrameLimit = 4096
const ManualRekeyFrameCount = 64
const ManualRekeyExitCode = 86
const manualRekeyAbortCode = 87

// Binding contains public admission data only. A PID is never a capability.
type ManualRekeyBinding struct {
	Trial, Source, Binary, Profile, Case, Role, Generation, Challenge string
}
type ManualRekeyObservation struct {
	OldHash, CandidateHash [32]byte
	Old, Candidate         encryption.Header
	OldBorrow, NewBorrow   ManualRekeyBorrowSnapshot
}
type ManualRekeyFrame struct {
	Binding     ManualRekeyBinding
	Sequence    uint8
	Kind        string
	Point       uint8
	Observation ManualRekeyObservation
	PayloadHash [32]byte
}

// The child transport can only own the controller-created fixed fd3/fd4 pipes.
// It cannot execute programs, accept arbitrary descriptors or address a PID.
type ManualRekeyChannel struct {
	binding         ManualRekeyBinding
	control, events *os.File
	sequence        uint8
	closed          bool
}

func ManualRekeyChildChannel(binding ManualRekeyBinding) (*ManualRekeyChannel, error) {
	if binding.Profile != "linux-layout1-rawkey256-4096" || binding.Trial == "" || binding.Source == "" || binding.Binary == "" || binding.Challenge == "" {
		return nil, errors.New("rekey binding refused")
	}
	control := os.NewFile(3, "rekey-control")
	events := os.NewFile(4, "rekey-events")
	if control == nil || events == nil {
		return nil, errors.New("missing fixed pipes")
	}
	ci, ce := control.Stat()
	ei, ee := events.Stat()
	if ce != nil || ee != nil || ci.Mode()&os.ModeNamedPipe == 0 || ei.Mode()&os.ModeNamedPipe == 0 || os.SameFile(ci, ei) {
		// A failed constructor retains its concrete owners, with no ambiguous retry.
		manualRekeyFailedChannels = append(manualRekeyFailedChannels, &ManualRekeyChannel{control: control, events: events})
		return nil, errors.New("distinct fixed pipes required")
	}
	return &ManualRekeyChannel{binding: binding, control: control, events: events}, nil
}

var manualRekeyFailedChannels []*ManualRekeyChannel

func ManualRekeyEncodeFrame(frame ManualRekeyFrame) ([]byte, error) {
	body, err := json.Marshal(frame)
	if err != nil || len(body) == 0 || len(body) > ManualRekeyFrameLimit || frame.Sequence == 0 || frame.Sequence > ManualRekeyFrameCount {
		return nil, errors.New("bounded frame refused")
	}
	raw := make([]byte, len(body)+4)
	binary.BigEndian.PutUint32(raw[:4], uint32(len(body)))
	copy(raw[4:], body)
	return raw, nil
}
func ManualRekeyDecodeFrame(raw []byte) (ManualRekeyFrame, error) {
	var frame ManualRekeyFrame
	if len(raw) < 5 || len(raw) > ManualRekeyFrameLimit+4 || int(binary.BigEndian.Uint32(raw[:4])) != len(raw)-4 {
		return frame, errors.New("frame extent refused")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw[4:]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&frame); err != nil {
		return frame, err
	}
	canonical, err := ManualRekeyEncodeFrame(frame)
	if err != nil || !bytes.Equal(canonical, raw) {
		return frame, errors.New("noncanonical frame refused")
	}
	return frame, nil
}
func ManualRekeyReadFrame(reader *os.File) (ManualRekeyFrame, []byte, error) {
	var prefix [4]byte
	if _, err := io.ReadFull(reader, prefix[:]); err != nil {
		return ManualRekeyFrame{}, nil, err
	}
	n := binary.BigEndian.Uint32(prefix[:])
	if n == 0 || n > ManualRekeyFrameLimit {
		return ManualRekeyFrame{}, nil, errors.New("frame bound")
	}
	raw := make([]byte, int(n)+4)
	copy(raw, prefix[:])
	if _, err := io.ReadFull(reader, raw[4:]); err != nil {
		return ManualRekeyFrame{}, nil, err
	}
	frame, err := ManualRekeyDecodeFrame(raw)
	return frame, raw, err
}
func ManualRekeyWriteFrame(writer *os.File, frame ManualRekeyFrame) ([]byte, error) {
	raw, err := ManualRekeyEncodeFrame(frame)
	if err != nil {
		return nil, err
	}
	n, err := writer.Write(raw)
	if err != nil || n != len(raw) {
		return nil, errors.Join(err, io.ErrShortWrite)
	}
	return raw, nil
}
func (c *ManualRekeyChannel) Exchange(kind string, point uint8, observation ManualRekeyObservation, payload [32]byte) (string, error) {
	if c == nil || c.closed || c.sequence >= ManualRekeyFrameCount {
		return "", errors.New("channel exhausted")
	}
	c.sequence++
	frame := ManualRekeyFrame{Binding: c.binding, Sequence: c.sequence, Kind: kind, Point: point, Observation: observation, PayloadHash: payload}
	raw, err := ManualRekeyWriteFrame(c.events, frame)
	if err != nil {
		return "", err
	}
	grant, _, err := ManualRekeyReadFrame(c.control)
	if err != nil || grant.Binding != frame.Binding || grant.Sequence != frame.Sequence || grant.Point != frame.Point || grant.Observation != frame.Observation || grant.PayloadHash != sha256.Sum256(raw) {
		return "", errors.New("causal grant refused")
	}
	if grant.Kind != "continue" && grant.Kind != "exit" && grant.Kind != "ack-exit" && grant.Kind != "done" {
		return "", errors.New("grant kind refused")
	}
	return grant.Kind, nil
}
func (c *ManualRekeyChannel) Close() error {
	if c == nil || c.closed {
		return errors.New("channel close repeated")
	}
	c.closed = true
	return errors.Join(c.control.Close(), c.events.Close())
}

// Concrete public-counter receipt contains no credentials or callbacks.
type ManualRekeyBorrowSnapshot struct {
	Acquires, Releases uint32
	Invalid            bool
}
type ManualRekeyBorrow struct {
	acquires, releases atomic.Uint32
	invalid            atomic.Bool
}

func (b *ManualRekeyBorrow) Acquired() {
	if b.acquires.Add(1) != 1 {
		b.invalid.Store(true)
	}
}
func (b *ManualRekeyBorrow) Released() {
	if b.releases.Add(1) != 1 {
		b.invalid.Store(true)
	}
}
func (b *ManualRekeyBorrow) Invalid() { b.invalid.Store(true) }
func (b *ManualRekeyBorrow) snapshot() ManualRekeyBorrowSnapshot {
	return ManualRekeyBorrowSnapshot{b.acquires.Load(), b.releases.Load(), b.invalid.Load()}
}

type manualRekeyContextKey struct{}
type manualRekeyScope struct {
	mu                   sync.Mutex
	path                 string
	oldHash              [32]byte
	oldHeader            encryption.Header
	channel              *ManualRekeyChannel
	caseName             string
	oldBorrow, newBorrow *ManualRekeyBorrow
	seen                 uint8
	candidate            ManualRekeyObservation
}

func ManualRekeyContext(ctx context.Context, path string, oldHash [32]byte, oldHeader encryption.Header, channel *ManualRekeyChannel, oldBorrow, newBorrow *ManualRekeyBorrow) (context.Context, error) {
	if oldBorrow == nil || newBorrow == nil || oldBorrow == newBorrow || ctx == nil || ctx.Value(manualRekeyContextKey{}) != nil || channel == nil || channel.binding.Role != "writer" || channel.sequence != 1 || oldHash == [32]byte{} || oldHeader.Descriptor.CommitSequence != 4 {
		return nil, errors.New("single writer scope refused")
	}
	switch channel.binding.Case {
	case "control", "before-root-rename", "after-root-rename-before-dirsync", "after-root-dirsync-before-adoption", "rekey-returned-no-ack", "acknowledged-exit":
	default:
		return nil, errors.New("case refused")
	}
	return context.WithValue(ctx, manualRekeyContextKey{}, &manualRekeyScope{path: path, oldHash: oldHash, oldHeader: oldHeader, channel: channel, caseName: channel.binding.Case, oldBorrow: oldBorrow, newBorrow: newBorrow}), nil
}
func manualRekeyTarget(caseName string) uint8 {
	switch caseName {
	case "before-root-rename":
		return 1
	case "after-root-rename-before-dirsync":
		return 2
	case "after-root-dirsync-before-adoption":
		return 3
	default:
		return 0
	}
}
func rekeyPublicationObserve(ctx context.Context, point rekeyPublicationPoint, path string, oldRoot, candidateRoot []byte) {
	scope, ok := ctx.Value(manualRekeyContextKey{}).(*manualRekeyScope)
	if !ok {
		return
	}
	// Only bounded authenticated ciphertext/public fields cross this boundary.
	// The caller already authenticated both roots. No slice is retained.
	if len(oldRoot) != rootSize || len(candidateRoot) != rootSize {
		manualRekeyAbort()
	}
	oldHeader, oldErr := encryption.DecodeHeader(oldRoot[:encryption.HeaderSize], encryption.Limits{})
	candidateHeader, candidateErr := encryption.DecodeHeader(candidateRoot[:encryption.HeaderSize], encryption.Limits{})
	observation := ManualRekeyObservation{OldHash: sha256.Sum256(oldRoot), CandidateHash: sha256.Sum256(candidateRoot), Old: oldHeader, Candidate: candidateHeader, OldBorrow: scope.oldBorrow.snapshot(), NewBorrow: scope.newBorrow.snapshot()}
	scope.mu.Lock()
	valid := observation.OldBorrow == (ManualRekeyBorrowSnapshot{Acquires: 1, Releases: 1}) && observation.NewBorrow == (ManualRekeyBorrowSnapshot{Acquires: 1, Releases: 1}) && oldErr == nil && candidateErr == nil && scope.path == path && scope.oldHash == observation.OldHash && scope.oldHeader == oldHeader && uint8(point) == scope.seen+1 && point <= rekeyAfterRootDirSync
	valid = valid && oldHeader.Descriptor.PageSize == 4096 && oldHeader.Descriptor.PageCount == 3 && oldHeader.Descriptor.Role == encryption.FileRoot && candidateHeader.Descriptor.Role == encryption.FileRoot
	valid = valid && candidateHeader.Descriptor.PageSize == 4096 && candidateHeader.Descriptor.PageCount == 3 && candidateHeader.Descriptor.CommitSequence == oldHeader.Descriptor.CommitSequence+1 && candidateHeader.Descriptor.ParentCommitID == observation.OldHash
	valid = valid && candidateHeader.Epoch.Kind == encryption.RawKey256 && candidateHeader.Epoch.DatabaseID == oldHeader.Epoch.DatabaseID && candidateHeader.Epoch.EpochID != oldHeader.Epoch.EpochID && candidateHeader.Epoch.RootSalt != oldHeader.Epoch.RootSalt && candidateHeader.FileID != oldHeader.FileID
	if scope.seen != 0 {
		valid = valid && scope.candidate == observation
	}
	if valid {
		scope.seen++
		scope.candidate = observation
	}
	scope.mu.Unlock()
	if !valid {
		manualRekeyAbort()
	}
	// No observer or engine/native lock is acquired during the concrete barrier.
	grant, err := scope.channel.Exchange("publication", uint8(point), observation, [32]byte{})
	if err != nil {
		manualRekeyAbort()
	}
	if manualRekeyTarget(scope.caseName) == uint8(point) {
		if grant != "exit" {
			manualRekeyAbort()
		}
		os.Exit(ManualRekeyExitCode)
	}
	if grant != "continue" {
		manualRekeyAbort()
	}
}
func manualRekeyAbort() { os.Exit(manualRekeyAbortCode) }

// Exactly the manual caller invokes this after adjacent public-result capture.
// A public error never travels through this nil-only barrier.
func ManualRekeyReturnedNil(ctx context.Context) error {
	scope, ok := ctx.Value(manualRekeyContextKey{}).(*manualRekeyScope)
	if !ok || scope.seen != 3 {
		return errors.New("public return without exact cuts")
	}
	grant, err := scope.channel.Exchange("public-nil", 0, scope.candidate, [32]byte{})
	if err != nil {
		return err
	}
	switch scope.caseName {
	case "rekey-returned-no-ack":
		if grant != "exit" {
			return errors.New("unexpected acknowledgement")
		}
		os.Exit(ManualRekeyExitCode)
	case "acknowledged-exit":
		if grant != "ack-exit" {
			return errors.New("missing parent acknowledgement")
		}
		os.Exit(ManualRekeyExitCode)
	case "control":
		if grant != "continue" {
			return errors.New("control grant refused")
		}
	default:
		return errors.New("interruption target missed")
	}
	return nil
}

// Snapshot only registry scalars and atomic pending pointers. Do not acquire a
// group mutex while holding registry.mu: normal close takes the reverse order.
type ManualRekeyOwnerSnapshot struct{ Groups, Leases, Pending int }

func ManualRekeyOwners() ManualRekeyOwnerSnapshot {
	encryptedCleanups.mu.Lock()
	defer encryptedCleanups.mu.Unlock()
	result := ManualRekeyOwnerSnapshot{Groups: len(encryptedCleanups.groups), Leases: encryptedCleanups.leases}
	for group := range encryptedCleanups.groups {
		if group.pending.Load() != nil {
			result.Pending++
		}
	}
	return result
}
