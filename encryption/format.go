package encryption

import (
	"bytes"
	"encoding/binary"
	"io"
)

const (
	Magic             = "GSQLENC1"
	FormatMajor       = 1
	FormatMinor       = 0
	SuiteAES256GCM    = 1
	HeaderSize        = 512
	RecordHeaderSize  = 24
	TagSize           = 16
	NonceSize         = 12
	MaxRecordBytes    = 1 << 20
	MaxPageSize       = 65536
	MaxPages          = 1 << 20
	MaxRecords        = MaxPages + 2
	MaxLogicalBytes   = uint64(MaxPages) * MaxPageSize
	MaxFilesPerMaster = 1 << 20
)

type FileRole uint8

const (
	FileRoot FileRole = iota + 1
	FileBase
	FileWAL
	FileBackup
)

type RecordRole uint8

const (
	RecordMetadata RecordRole = iota + 1
	RecordData
	RecordFooter
)

// Limits may lower, never raise, the experimental format's hard limits. Zero
// fields use the hard limit. Limits apply before allocation or record parsing.
// Aggregate scan/work/directory quotas are the storage adapter's responsibility.
type Limits struct {
	MaxRecordBytes  uint32
	MaxPages        uint32
	MaxRecords      uint64
	MaxLogicalBytes uint64
}

func (l Limits) normalized() (Limits, error) {
	if l.MaxRecordBytes == 0 {
		l.MaxRecordBytes = MaxRecordBytes
	}
	if l.MaxPages == 0 {
		l.MaxPages = MaxPages
	}
	if l.MaxRecords == 0 {
		l.MaxRecords = MaxRecords
	}
	if l.MaxLogicalBytes == 0 {
		l.MaxLogicalBytes = MaxLogicalBytes
	}
	if l.MaxRecordBytes > MaxRecordBytes || l.MaxPages > MaxPages || l.MaxRecords > MaxRecords || l.MaxLogicalBytes > MaxLogicalBytes {
		return Limits{}, ErrLimit
	}
	return l, nil
}

// FileDescriptor contains immutable public geometry and commit identity, but
// cannot contain a FileID or a caller-selected nonce/counter. Root contains one
// metadata record. Base/backup contain metadata, PageCount page slots, footer.
// WAL contains metadata, RecordCount-2 page frames, footer. For SQLite's reserved
// lock-byte page a future storage adapter must define/verify a canonical slot;
// this package does not interpret page bytes or promise database validity.
type FileDescriptor struct {
	Role           FileRole
	PageSize       uint32
	PageCount      uint32
	RecordCount    uint64
	CommitSequence uint64
	TransactionID  [32]byte
	ParentCommitID [32]byte
}

func (d FileDescriptor) Validate(limits Limits) error {
	l, err := limits.normalized()
	if err != nil {
		return err
	}
	if d.Role < FileRoot || d.Role > FileBackup {
		return ErrUnsupported
	}
	if d.PageSize < 512 || d.PageSize > MaxPageSize || d.PageSize&(d.PageSize-1) != 0 || d.PageCount == 0 || d.RecordCount == 0 {
		return ErrFormat
	}
	if d.PageCount > l.MaxPages || d.RecordCount > l.MaxRecords || uint64(d.PageCount)*uint64(d.PageSize) > l.MaxLogicalBytes {
		return ErrLimit
	}
	switch d.Role {
	case FileRoot:
		if d.RecordCount != 1 {
			return ErrFormat
		}
	case FileBase, FileBackup:
		if d.RecordCount != uint64(d.PageCount)+2 {
			return ErrFormat
		}
	case FileWAL:
		if d.RecordCount < 3 || d.RecordCount > uint64(d.PageCount)+2 || d.TransactionID == [32]byte{} || d.ParentCommitID == [32]byte{} {
			return ErrFormat
		}
	}
	if d.Role != FileRoot && d.PageSize > l.MaxRecordBytes {
		return ErrLimit
	}
	return nil
}

// FileIdentity for a child file must come from authenticated parent metadata,
// not merely be copied from that child's untrusted header. The root itself has
// no external freshness oracle; a complete historical root may authenticate.
type FileIdentity struct {
	DatabaseID [16]byte
	EpochID    [32]byte
	FileID     [32]byte
	Role       FileRole
}

// Header is a strictly encoded public bootstrap. Its decoded values are
// untrusted until OpenFile successfully authenticates its metadata record.
// Version, suite, flags, exact geometry and all salts/IDs are bound into AAD.
type Header struct {
	Major       uint16
	Minor       uint16
	Suite       uint16
	Flags       uint32
	Epoch       Epoch
	FileID      [32]byte
	NoncePrefix [4]byte
	Descriptor  FileDescriptor
}

func (h Header) Identity() FileIdentity {
	return FileIdentity{DatabaseID: h.Epoch.DatabaseID, EpochID: h.Epoch.EpochID, FileID: h.FileID, Role: h.Descriptor.Role}
}

func (h Header) Validate(limits Limits) error {
	if h.Major != FormatMajor || h.Minor != FormatMinor || h.Suite != SuiteAES256GCM || h.Flags != 0 {
		return ErrUnsupported
	}
	if h.FileID == [32]byte{} {
		return ErrFormat
	}
	if err := h.Epoch.Validate(KDFPolicy{}); err != nil {
		return err
	}
	return h.Descriptor.Validate(limits)
}

// EncodeHeader writes explicit big-endian fields and canonical zero padding.
// Offsets (end exclusive): magic 0:8; major 8:10; minor 10:12; suite 12:14;
// size 14:16; flags 16:20; file role 20; credential kind 21; Argon version 22;
// lanes 23; memory 24:28; passes 28:32; DBID 32:48; epoch 48:80; FileID 80:112;
// KDF salt 112:144; root salt 144:176; nonce prefix 176:180; page size 180:184;
// page count 184:188; reserved 188:192; record count 192:200; commit 200:208;
// transaction 208:240; parent 240:272; reserved 272:512.
func EncodeHeader(h Header, limits Limits) ([]byte, error) {
	if err := h.Validate(limits); err != nil {
		return nil, err
	}
	b := make([]byte, HeaderSize)
	copy(b, Magic)
	binary.BigEndian.PutUint16(b[8:10], h.Major)
	binary.BigEndian.PutUint16(b[10:12], h.Minor)
	binary.BigEndian.PutUint16(b[12:14], h.Suite)
	binary.BigEndian.PutUint16(b[14:16], HeaderSize)
	binary.BigEndian.PutUint32(b[16:20], h.Flags)
	b[20], b[21] = byte(h.Descriptor.Role), byte(h.Epoch.Kind)
	if h.Epoch.Kind == PasswordArgon2id {
		b[22] = ArgonVersion
	}
	b[23] = h.Epoch.KDF.Parallelism
	binary.BigEndian.PutUint32(b[24:28], h.Epoch.KDF.MemoryKiB)
	binary.BigEndian.PutUint32(b[28:32], h.Epoch.KDF.Passes)
	copy(b[32:48], h.Epoch.DatabaseID[:])
	copy(b[48:80], h.Epoch.EpochID[:])
	copy(b[80:112], h.FileID[:])
	copy(b[112:144], h.Epoch.KDFSalt[:])
	copy(b[144:176], h.Epoch.RootSalt[:])
	copy(b[176:180], h.NoncePrefix[:])
	binary.BigEndian.PutUint32(b[180:184], h.Descriptor.PageSize)
	binary.BigEndian.PutUint32(b[184:188], h.Descriptor.PageCount)
	binary.BigEndian.PutUint64(b[192:200], h.Descriptor.RecordCount)
	binary.BigEndian.PutUint64(b[200:208], h.Descriptor.CommitSequence)
	copy(b[208:240], h.Descriptor.TransactionID[:])
	copy(b[240:272], h.Descriptor.ParentCommitID[:])
	return b, nil
}

func DecodeHeader(b []byte, limits Limits) (Header, error) {
	if len(b) != HeaderSize || string(b[:8]) != Magic || binary.BigEndian.Uint16(b[14:16]) != HeaderSize {
		return Header{}, ErrFormat
	}
	if !zeroBytes(b[188:192]) || !zeroBytes(b[272:]) {
		return Header{}, ErrFormat
	}
	h := Header{
		Major: binary.BigEndian.Uint16(b[8:10]), Minor: binary.BigEndian.Uint16(b[10:12]),
		Suite: binary.BigEndian.Uint16(b[12:14]), Flags: binary.BigEndian.Uint32(b[16:20]),
	}
	h.Descriptor.Role, h.Epoch.Kind = FileRole(b[20]), CredentialKind(b[21])
	if (h.Epoch.Kind == RawKey256 && b[22] != 0) || (h.Epoch.Kind == PasswordArgon2id && b[22] != ArgonVersion) {
		return Header{}, ErrUnsupported
	}
	h.Epoch.KDF = Argon2Params{MemoryKiB: binary.BigEndian.Uint32(b[24:28]), Passes: binary.BigEndian.Uint32(b[28:32]), Parallelism: b[23]}
	copy(h.Epoch.DatabaseID[:], b[32:48])
	copy(h.Epoch.EpochID[:], b[48:80])
	copy(h.FileID[:], b[80:112])
	copy(h.Epoch.KDFSalt[:], b[112:144])
	copy(h.Epoch.RootSalt[:], b[144:176])
	copy(h.NoncePrefix[:], b[176:180])
	h.Descriptor.PageSize, h.Descriptor.PageCount = binary.BigEndian.Uint32(b[180:184]), binary.BigEndian.Uint32(b[184:188])
	h.Descriptor.RecordCount, h.Descriptor.CommitSequence = binary.BigEndian.Uint64(b[192:200]), binary.BigEndian.Uint64(b[200:208])
	copy(h.Descriptor.TransactionID[:], b[208:240])
	copy(h.Descriptor.ParentCommitID[:], b[240:272])
	if err := h.Validate(limits); err != nil {
		return Header{}, err
	}
	return h, nil
}

// ReadHeader consumes exactly HeaderSize bytes; callers own stream completeness
// checks. It does not open paths or allocate from unvalidated header lengths.
func ReadHeader(r io.Reader, limits Limits) (Header, error) {
	var b [HeaderSize]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return Header{}, ErrFormat
	}
	return DecodeHeader(b[:], limits)
}

func zeroBytes(b []byte) bool { return bytes.Count(b, []byte{0}) == len(b) }

// RecordIdentity is an expected authenticated location, not a routing hint
// decoded from unauthenticated record bytes. PageNumber and FrameOrdinal are
// deliberately absent from the public record header.
type RecordIdentity struct {
	Role         RecordRole
	Sequence     uint64
	PageNumber   uint32
	FrameOrdinal uint64
}

type RecordHeader struct {
	Role          RecordRole
	Sequence      uint64
	PlaintextSize uint32
}

func (h RecordHeader) Validate(limits Limits) error {
	l, err := limits.normalized()
	if err != nil {
		return err
	}
	if h.Role < RecordMetadata || h.Role > RecordFooter {
		return ErrUnsupported
	}
	if h.Sequence >= l.MaxRecords || h.PlaintextSize > l.MaxRecordBytes {
		return ErrLimit
	}
	if h.Role == RecordData && (h.PlaintextSize < 512 || h.PlaintextSize > MaxPageSize || h.PlaintextSize&(h.PlaintextSize-1) != 0) {
		return ErrFormat
	}
	return nil
}

// EncodeRecordHeader emits GSER, role, three zero bytes, sequence (u64),
// plaintext length (u32), and four zero bytes. The GCM tag is not truncated.
func EncodeRecordHeader(h RecordHeader, limits Limits) ([]byte, error) {
	if err := h.Validate(limits); err != nil {
		return nil, err
	}
	b := make([]byte, RecordHeaderSize)
	copy(b, "GSER")
	b[4] = byte(h.Role)
	binary.BigEndian.PutUint64(b[8:16], h.Sequence)
	binary.BigEndian.PutUint32(b[16:20], h.PlaintextSize)
	return b, nil
}

func DecodeRecordHeader(b []byte, limits Limits) (RecordHeader, error) {
	if len(b) != RecordHeaderSize || string(b[:4]) != "GSER" || !zeroBytes(b[5:8]) || !zeroBytes(b[20:24]) {
		return RecordHeader{}, ErrFormat
	}
	h := RecordHeader{Role: RecordRole(b[4]), Sequence: binary.BigEndian.Uint64(b[8:16]), PlaintextSize: binary.BigEndian.Uint32(b[16:20])}
	if err := h.Validate(limits); err != nil {
		return RecordHeader{}, err
	}
	return h, nil
}

// ReadRecord reads one bounded encoded record. Before allocating a body it
// validates the fixed header and length. It never returns partial bytes.
// Extra records or trailing stream data must be rejected by the container's
// independently authenticated expected extent; this function reads just one.
func ReadRecord(r io.Reader, limits Limits) ([]byte, error) {
	var b [RecordHeaderSize]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return nil, ErrFormat
	}
	h, err := DecodeRecordHeader(b[:], limits)
	if err != nil {
		return nil, err
	}
	result := make([]byte, RecordHeaderSize+int(h.PlaintextSize)+TagSize)
	copy(result, b[:])
	if _, err := io.ReadFull(r, result[RecordHeaderSize:]); err != nil {
		return nil, ErrFormat
	}
	return result, nil
}
