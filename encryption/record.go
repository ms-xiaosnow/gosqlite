package encryption

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sync"
)

// FileWriter is a write-once nonce allocator. Only NewFileWriter constructs
// one; no public constructor accepts a FileID, nonce prefix or starting counter.
// Copies share a mutex and counter. A Seal error poisons every copy. Call Close
// to abandon a candidate and NewFileWriter to retry with fresh identity/keys.
type FileWriter struct {
	state *writerState
}

type writerState struct {
	mu      sync.Mutex
	header  Header
	digest  [32]byte
	limits  Limits
	ciphers [4]cipher.AEAD
	next    uint64
	closed  bool
	walSeen map[uint32]struct{}
}

func (FileWriter) Format(s fmt.State, verb rune) {
	_, _ = s.Write([]byte("encryption.FileWriter{redacted}"))
}

func NewFileWriter(master *MasterKey, descriptor FileDescriptor, limits Limits) (*FileWriter, error) {
	return newFileWriter(master, descriptor, limits, rand.Reader)
}

// The entropy seam is package-private and used only in pure-memory tests. The
// production API cannot opt into a predictable RNG or reopen a previous ID.
func newFileWriter(master *MasterKey, descriptor FileDescriptor, limits Limits, entropy io.Reader) (*FileWriter, error) {
	limits, err := limits.normalized()
	if err != nil {
		return nil, err
	}
	if err := descriptor.Validate(limits); err != nil {
		return nil, err
	}
	epoch, err := master.Epoch()
	if err != nil {
		return nil, err
	}
	h := Header{Major: FormatMajor, Minor: FormatMinor, Suite: SuiteAES256GCM, Epoch: epoch, Descriptor: descriptor}
	if _, err := io.ReadFull(entropy, h.FileID[:]); err != nil {
		return nil, ErrRandomness
	}
	if h.FileID == [32]byte{} {
		return nil, ErrRandomness
	}
	if _, err := io.ReadFull(entropy, h.NoncePrefix[:]); err != nil {
		return nil, ErrRandomness
	}
	encoded, err := EncodeHeader(h, limits)
	if err != nil {
		return nil, err
	}
	ciphers, err := master.fileCiphers(h, true)
	if err != nil {
		return nil, err
	}
	s := &writerState{header: h, digest: sha256.Sum256(encoded), limits: limits, ciphers: ciphers}
	if descriptor.Role == FileWAL {
		s.walSeen = make(map[uint32]struct{})
	}
	return &FileWriter{state: s}, nil
}

// Header returns a value copy of public, immutable fields, including after
// Close. EncodeHeader produces the bytes to precede the sealed metadata record.
func (w *FileWriter) Header() (Header, error) {
	if w == nil || w.state == nil {
		return Header{}, ErrClosed
	}
	s := w.state
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.header, nil
}

// Seal allocates the next sequence internally. Metadata must be first and
// footer last; base/backup data must be in page order, and WAL frame ordinals
// must be consecutive with unique page numbers. Page/frame zero is mandatory
// for metadata/footer. The returned bytes own their storage and do not alias
// plaintext. Any error permanently disables this writer, even validation errors.
func (w *FileWriter) Seal(role RecordRole, pageNumber uint32, frameOrdinal uint64, plaintext []byte) ([]byte, error) {
	if w == nil || w.state == nil {
		return nil, ErrClosed
	}
	s := w.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	fail := func(err error) ([]byte, error) {
		s.clear()
		return nil, err
	}
	if s.next == math.MaxUint64 || s.next >= s.header.Descriptor.RecordCount {
		return fail(ErrWriterState)
	}
	if len(plaintext) > int(s.limits.MaxRecordBytes) {
		return fail(ErrLimit)
	}
	id := RecordIdentity{Role: role, Sequence: s.next, PageNumber: pageNumber, FrameOrdinal: frameOrdinal}
	if err := validateLocation(s.header, id, uint32(len(plaintext))); err != nil {
		return fail(err)
	}
	if role == RecordData && s.walSeen != nil {
		if _, exists := s.walSeen[pageNumber]; exists {
			return fail(ErrIdentity)
		}
		s.walSeen[pageNumber] = struct{}{}
	}
	rh := RecordHeader{Role: role, Sequence: s.next, PlaintextSize: uint32(len(plaintext))}
	encoded, err := EncodeRecordHeader(rh, s.limits)
	if err != nil {
		return fail(err)
	}
	nonce := recordNonce(s.header.NoncePrefix, s.next)
	aad := recordAAD(s.digest, encoded, id)
	// Burn the sequence before Seal. There is no exposed reset/retry path.
	s.next++
	result := make([]byte, RecordHeaderSize, RecordHeaderSize+len(plaintext)+TagSize)
	copy(result, encoded)
	result = s.ciphers[role].Seal(result, nonce[:], plaintext, aad[:])
	return result, nil
}

// Finish succeeds only after every declared record was sealed. It closes the
// writer either way. It is not a durable publication or whole-file verification
// operation, and it does not interpret metadata/footer contents.
func (w *FileWriter) Finish() error {
	if w == nil || w.state == nil {
		return ErrClosed
	}
	s := w.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	complete := s.next == s.header.Descriptor.RecordCount
	s.clear()
	if !complete {
		return ErrWriterState
	}
	return nil
}

func (w *FileWriter) Close() error {
	if w == nil || w.state == nil {
		return nil
	}
	s := w.state
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clear()
	return nil
}

func (s *writerState) clear() {
	s.ciphers = [4]cipher.AEAD{}
	s.walSeen = nil
	s.closed = true
}

// OpenedFile is a read-only authenticated header/key context. It has no Seal,
// conversion-to-writer, resumed counter, or implicit credential fallback. Copies
// share Close state. Each data record is authenticated again on every Open.
type OpenedFile struct {
	state *readerState
}

type readerState struct {
	mu      sync.Mutex
	header  Header
	digest  [32]byte
	limits  Limits
	ciphers [4]cipher.AEAD
	closed  bool
}

func (OpenedFile) Format(s fmt.State, verb rune) {
	_, _ = s.Write([]byte("encryption.OpenedFile{redacted}"))
}

// OpenFile validates expected identity and authenticates the metadata record
// before returning either a usable reader or metadata plaintext. Limits and
// KDF format bounds are validated first. This does not scan the remaining
// records or establish footer/root semantics or stream completeness.
func OpenFile(master *MasterKey, expected FileIdentity, encodedHeader, metadataRecord []byte, limits Limits) (*OpenedFile, []byte, error) {
	limits, err := limits.normalized()
	if err != nil {
		return nil, nil, err
	}
	h, err := DecodeHeader(encodedHeader, limits)
	if err != nil {
		return nil, nil, err
	}
	if expected != h.Identity() {
		return nil, nil, ErrAuthentication
	}
	ciphers, err := master.fileCiphers(h, false)
	if err != nil {
		if err == ErrIdentity {
			err = ErrAuthentication
		}
		return nil, nil, err
	}
	s := &readerState{header: h, digest: sha256.Sum256(encodedHeader), limits: limits, ciphers: ciphers}
	r := &OpenedFile{state: s}
	metadata, err := r.Open(RecordIdentity{Role: RecordMetadata}, metadataRecord)
	if err != nil {
		_ = r.Close()
		return nil, nil, err
	}
	return r, metadata, nil
}

func (r *OpenedFile) Header() (Header, error) {
	if r == nil || r.state == nil {
		return Header{}, ErrClosed
	}
	s := r.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Header{}, ErrClosed
	}
	return s.header, nil
}

// Open authenticates exactly one complete record at the caller's expected
// identity. It returns a newly owned plaintext buffer only on success, and
// returns nil on every failure. It never calls a plaintext parser/callback.
func (r *OpenedFile) Open(expected RecordIdentity, record []byte) ([]byte, error) {
	if r == nil || r.state == nil {
		return nil, ErrClosed
	}
	s := r.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	if len(record) < RecordHeaderSize {
		return nil, ErrFormat
	}
	rh, err := DecodeRecordHeader(record[:RecordHeaderSize], s.limits)
	if err != nil {
		return nil, err
	}
	if len(record) != RecordHeaderSize+int(rh.PlaintextSize)+TagSize {
		return nil, ErrFormat
	}
	if rh.Role != expected.Role || rh.Sequence != expected.Sequence {
		return nil, ErrAuthentication
	}
	if err := validateLocation(s.header, expected, rh.PlaintextSize); err != nil {
		return nil, ErrAuthentication
	}
	nonce := recordNonce(s.header.NoncePrefix, expected.Sequence)
	aad := recordAAD(s.digest, record[:RecordHeaderSize], expected)
	// Own the scratch buffer even on authentication failure, when a cipher
	// implementation is allowed to have overwritten it. Never return it then.
	owned := make([]byte, int(rh.PlaintextSize))
	plaintext, err := s.ciphers[expected.Role].Open(owned[:0], nonce[:], record[RecordHeaderSize:], aad[:])
	if err != nil {
		wipe(owned)
		return nil, ErrAuthentication
	}
	return plaintext, nil
}

func (r *OpenedFile) Close() error {
	if r == nil || r.state == nil {
		return nil
	}
	s := r.state
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ciphers = [4]cipher.AEAD{}
	s.closed = true
	return nil
}

func validateLocation(h Header, id RecordIdentity, size uint32) error {
	d := h.Descriptor
	if id.Sequence >= d.RecordCount {
		return ErrIdentity
	}
	role := RecordData
	if id.Sequence == 0 {
		role = RecordMetadata
	} else if id.Sequence == d.RecordCount-1 {
		role = RecordFooter
	}
	if id.Role != role || (d.Role == FileRoot && id.Role != RecordMetadata) {
		return ErrIdentity
	}
	if id.Role != RecordData {
		if id.PageNumber != 0 || id.FrameOrdinal != 0 {
			return ErrIdentity
		}
		return nil
	}
	if size != d.PageSize || id.PageNumber == 0 || id.PageNumber > d.PageCount {
		return ErrIdentity
	}
	if d.Role == FileWAL {
		if id.FrameOrdinal != id.Sequence {
			return ErrIdentity
		}
	} else if id.FrameOrdinal != 0 || uint64(id.PageNumber) != id.Sequence {
		return ErrIdentity
	}
	return nil
}

func recordNonce(prefix [4]byte, sequence uint64) [NonceSize]byte {
	var nonce [NonceSize]byte
	copy(nonce[:4], prefix[:])
	binary.BigEndian.PutUint64(nonce[4:], sequence)
	return nonce
}

// The SHA-256 digest commits to every byte of the canonical 512-byte header.
// Remaining fields are fixed-width, so no ambiguous concatenation is possible.
// AAD: domain[8] || headerDigest[32] || recordHeader[24] || page[4] || frame[8].
func recordAAD(headerDigest [32]byte, recordHeader []byte, id RecordIdentity) [76]byte {
	var aad [76]byte
	copy(aad[:8], "GSQLAAD1")
	copy(aad[8:40], headerDigest[:])
	copy(aad[40:64], recordHeader)
	binary.BigEndian.PutUint32(aad[64:68], id.PageNumber)
	binary.BigEndian.PutUint64(aad[68:76], id.FrameOrdinal)
	return aad
}
