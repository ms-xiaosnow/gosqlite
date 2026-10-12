package vfs

import (
	"errors"
	"io"
	"os"
	"sync"
)

// walPublicationOps is the sole descriptor-free operation boundary. Production
// binds fixed native owners; models bind only bytes, inert identities and events.
// No operation may close the borrowed main/current WAL, or adopt a child owner
// independently of its directory. All methods run under WALPublication.mu.
type walPublicationOps interface {
	allowedFiles() []*File
	acquire() error
	create() error
	readSource([]byte, int64) (int, error)
	writeCandidate([]byte, int64) (int, error)
	candidateInfo() (os.FileInfo, error)
	recheck() error
	syncCandidate() error
	replace() (bool, error)
	transfer(*File, *File) (*File, error)
	postPublication() (os.FileInfo, error)
	syncDirectory() error
	seal()
	closeChild() (bool, error)
	closeDirectory() (bool, error)
}

// WALPublication owns a candidate-or-superseded WAL before its retained parent.
// It borrows the original continuously EXCLUSIVE main. Allocation is memory-only;
// the pager must install this composite in its bounded temporary slot before
// Acquire. Session serialization -> group (transfer only) -> composite -> files
// (main, original, candidate, directory) is the fixed lock order. Native methods
// never call a group, Session, registry admission, or application callback.
type WALPublication struct {
	mu sync.Mutex
	ops walPublicationOps
	acquireAttempted, acquired bool
	createAttempted, created bool
	prepareAttempted, prepared bool
	replaceAttempted, replaced, transferred bool
	finishAttempted, durable bool
	sealed, terminal, complete, failed bool
	childCloseAttempted, childComplete bool
	directoryCloseAttempted, directoryComplete bool
	expectedSize int64
	unsafe error // first unknown ownership graph only, never exposed or formatted
}

// NewWALPublication performs no I/O and grants no default constructor authority.
func NewWALPublication(main, original *File, mainInfo, originalInfo os.FileInfo) *WALPublication {
	return newWALPublicationWithOps(newNativeWALPublication(main, original, mainInfo, originalInfo))
}
func newWALPublicationWithOps(ops walPublicationOps) *WALPublication {
	return &WALPublication{ops: ops}
}
func (p *WALPublication) failure(err error) error {
	if err == nil { return nil }
	p.failed = true
	diagnostic, terminal := walPublicationError(err, p.ops.allowedFiles())
	if terminal {
		p.terminal = true
		if p.unsafe == nil { p.unsafe = err }
	}
	return diagnostic
}
func (p *WALPublication) usable() bool { return p.ops != nil && !p.sealed && !p.terminal && !p.failed }
func (p *WALPublication) Acquire() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.usable() || p.acquireAttempted { return ErrPoisoned }
	p.acquireAttempted = true
	if err := p.ops.acquire(); err != nil { return p.failure(err) }
	p.acquired = true
	return nil
}
func (p *WALPublication) CreateCandidate(size int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.usable() || !p.acquired || p.createAttempted || size < 32 || size > 1<<30 { return ErrPoisoned }
	p.createAttempted, p.expectedSize = true, size
	if err := p.ops.create(); err != nil { return p.failure(err) }
	info, err := p.ops.candidateInfo()
	if err != nil { return p.failure(err) }
	if info == nil || info.Size() != 0 { return ErrIdentity }
	p.created = true
	return nil
}

type walPublicationSource struct{ publication *WALPublication }
func (p *WALPublication) Source() io.ReaderAt { return walPublicationSource{p} }
func (r walPublicationSource) ReadAt(b []byte, off int64) (int, error) {
	p := r.publication
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.usable() || !p.created || p.prepareAttempted || off < 0 { return 0, ErrPoisoned }
	n, err := p.ops.readSource(b, off)
	if n != len(b) { err = errors.Join(io.ErrUnexpectedEOF, err) }
	if n == len(b) && err == io.EOF { return n, io.EOF }
	return n, p.failure(err)
}
func (p *WALPublication) WriteAt(b []byte, off int64) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.usable() || !p.created || p.prepareAttempted || off < 0 || off > p.expectedSize || int64(len(b)) > p.expectedSize-off { return 0, ErrPoisoned }
	n, err := p.ops.writeCandidate(b, off)
	if n != len(b) { err = errors.Join(io.ErrShortWrite, err) }
	return n, p.failure(err)
}

// Prepare is entered only after the caller's final cancellation check. From
// candidate Sync onward the caller must complete the finite publication/barrier
// sequence without cancellation-driven abandonment. No background retry exists.
func (p *WALPublication) Prepare(size int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.usable() || !p.created || p.prepareAttempted || size != p.expectedSize { return ErrPoisoned }
	p.prepareAttempted = true
	info, err := p.ops.candidateInfo()
	if err != nil { return p.failure(err) }
	if info == nil || info.Size() != size { return ErrIdentity }
	if err = p.ops.recheck(); err != nil { return p.failure(err) }
	if err = p.ops.syncCandidate(); err != nil { return p.failure(err) }
	if err = p.ops.recheck(); err != nil { return p.failure(err) }
	p.prepared = true
	return nil
}

// Replace consumes its permit before its only native attempt. A nil syscall
// result is known completion even if later observations fail. Any other outcome
// stays in the original owner representation and permits retirement only.
func (p *WALPublication) Replace() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.usable() || !p.prepared || p.replaceAttempted { return false, ErrPoisoned }
	p.replaceAttempted = true
	known, err := p.ops.replace()
	p.replaced = known
	if !known && err == nil { err = ErrPoisoned }
	return known, p.failure(err)
}

// TransferWAL is memory-only and is called under the parent group's mutex after
// exact group/temporary/current-owner checks. It never closes or unlocks either
// file. On failure both pre-transfer owners remain represented and protected.
func (p *WALPublication) TransferWAL(main, original *File) (*File, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sealed || p.terminal || !p.replaced || p.transferred || p.finishAttempted { return nil, ErrPoisoned }
	current, err := p.ops.transfer(main, original)
	if err != nil { return nil, p.failure(err) }
	if current == nil || current == main || current == original { p.terminal = true; return nil, ErrPoisoned }
	p.transferred = true
	return current, nil
}

// FinishPublication always attempts one protective directory barrier after a
// Replace attempt, including an uncertain outcome or failed transfer/observation.
// A successful barrier cannot erase any earlier failure or establish success.
func (p *WALPublication) FinishPublication() (os.FileInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sealed || !p.replaceAttempted || p.finishAttempted { return nil, ErrPoisoned }
	p.finishAttempted = true
	var info os.FileInfo
	var observed error
	if p.replaced && p.transferred && !p.terminal {
		info, observed = p.ops.postPublication()
	} else {
		observed = ErrPoisoned
	}
	// The barrier is mandatory even when the observation or anchored SHM step
	// failed. It is not a rename retry or a guessed namespace-success probe.
	barrier := p.ops.syncDirectory()
	err := p.failure(errors.Join(observed, barrier))
	p.durable = err == nil && !p.failed && info != nil && info.Size() == p.expectedSize
	if err == nil && !p.durable { err = ErrIdentity }
	return info, err
}
func (p *WALPublication) Durable() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.durable && !p.terminal
}
func (p *WALPublication) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.complete { return nil }
	if !p.sealed { p.sealed = true; p.ops.seal() }
	if p.terminal { return ErrPoisoned }
	var errs []error
	if !p.childComplete {
		if p.childCloseAttempted { p.terminal = true; return ErrPoisoned }
		p.childCloseAttempted = true
		complete, err := p.ops.closeChild()
		errs = append(errs, p.failure(err))
		p.childComplete = complete
		if !complete || p.terminal {
			p.terminal = true // unknown consumption is an owned, non-retryable tombstone
			return errors.Join(ErrPoisoned, errors.Join(errs...))
		}
	}
	if !p.directoryComplete {
		if p.directoryCloseAttempted { p.terminal = true; return ErrPoisoned }
		p.directoryCloseAttempted = true
		complete, err := p.ops.closeDirectory()
		errs = append(errs, p.failure(err))
		p.directoryComplete = complete
		if !complete || p.terminal { p.terminal = true; return errors.Join(ErrPoisoned, errors.Join(errs...)) }
	}
	p.complete = true
	return errors.Join(errs...)
}
func (p *WALPublication) RetirementComplete() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.complete && !p.terminal
}
