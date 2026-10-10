// Package encstore implements experimental immutable encrypted snapshots and
// authenticated WAL segments. It never writes native SQLite sidecars.
package encstore

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

var (
	ErrBusySnapshot = fmt.Errorf("%w: encrypted read snapshot is no longer current", ErrBusy)
	ErrPoisoned     = errors.New("encrypted storage ownership is poisoned")
	ErrBusy         = errors.New("encrypted database is busy")
	ErrChanged      = errors.New("encrypted database identity changed")
	ErrUnsupported  = errors.New("unsupported encrypted storage configuration")
	ErrUncertain    = errors.New("encrypted root publication durability is uncertain")
	ErrReadOnly     = errors.New("encrypted database is read-only")
)

// Options is copied by the public connection. Provider and Budget remain
// explicitly shared application-owned objects, never serialized to disk.
type Options struct {
	Mode                Mode
	WALLimits           WALLimits
	KeyProvider         encryption.KeyProvider
	CredentialKind      encryption.CredentialKind
	CreateKDF           encryption.Argon2Params
	KDFPolicy           encryption.KDFPolicy
	DerivationBudget    *encryption.DerivationBudget
	Limits              encryption.Limits
	MaxGenerations      uint64
	MaxRetainedBytes    uint64
	MaxDirectoryEntries uint32
}

func (Options) Format(s fmt.State, verb rune) { _, _ = s.Write([]byte("encstore.Options{redacted}")) }
func (o Options) Validate() error {
	if o.Mode != Snapshot && !o.Mode.IsWAL() {
		return ErrUnsupported
	}
	if _, err := o.WALLimits.normalized(); err != nil {
		return err
	}
	if _, _, _, err := o.retentionLimits(); err != nil {
		return err
	}
	if err := (encryption.Epoch{DatabaseID: [16]byte{1}, EpochID: [32]byte{1}, RootSalt: [32]byte{1}, Kind: encryption.RawKey256}).Validate(o.KDFPolicy); err != nil {
		return err
	}
	if o.KeyProvider == nil {
		return encryption.ErrKeyProvider
	}
	d := encryption.FileDescriptor{Role: encryption.FileBase, PageSize: 512, PageCount: 1, RecordCount: 3}
	if err := d.Validate(o.Limits); err != nil {
		return err
	}
	switch o.CredentialKind {
	case encryption.RawKey256:
		if o.CreateKDF != (encryption.Argon2Params{}) {
			return encryption.ErrKDFPolicy
		}
	case encryption.PasswordArgon2id:
		p := o.CreateKDF
		if p == (encryption.Argon2Params{}) {
			p = encryption.DefaultArgon2Params()
		}
		if err := p.Validate(o.KDFPolicy); err != nil {
			return err
		}
	default:
		return encryption.ErrCredential
	}
	return nil
}

type Geometry struct{ PageSize, PageCount uint32 }
type PageSource interface {
	ReadPage(context.Context, uint32) ([]byte, error)
}
type SourceFunc func(context.Context, uint32) ([]byte, error)

func (f SourceFunc) ReadPage(c context.Context, n uint32) ([]byte, error) { return f(c, n) }

type Prepared struct {
	writerIDs uint64
	Master    *encryption.MasterKey
	Options   Options
}

func Prepare(ctx context.Context, path string, o Options, op encryption.Operation, dbid [16]byte) (*Prepared, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	if err := platformOperation(op, o.Mode); err != nil {
		return nil, err
	}
	var pathErr error
	path, pathErr = canonical(path)
	if pathErr != nil {
		return nil, pathErr
	}
	if err := platformPreflight(path); err != nil {
		return nil, err
	}
	var epoch encryption.Epoch
	var err error
	if dbid == ([16]byte{}) {
		epoch, err = encryption.NewEpoch(o.CredentialKind, o.CreateKDF)
	} else {
		epoch, err = encryption.NewRekeyEpoch(dbid, o.CredentialKind, o.CreateKDF)
	}
	if err != nil {
		return nil, err
	}
	m, err := encryption.DeriveMaster(ctx, o.KeyProvider, encryption.KeyRequest{Path: path, Operation: op}, epoch, o.KDFPolicy, o.DerivationBudget)
	if err != nil {
		return nil, err
	}
	return &Prepared{Master: m, Options: o}, nil
}
func (p *Prepared) Close() error {
	if p == nil {
		return nil
	}
	return p.Master.Close()
}

const metaSize = 64
const footerSize = 48
const manifestSize = 128
const recordOverhead = encryption.RecordHeaderSize + encryption.TagSize
const rootSize = encryption.HeaderSize + recordOverhead + manifestSize

type manifest struct {
	BaseID                [32]byte
	BaseBytes             uint64
	BaseHash, PageOneHash [32]byte
}

func (m manifest) encode() []byte {
	b := make([]byte, manifestSize)
	copy(b, "GSEMNF01")
	copy(b[8:40], m.BaseID[:])
	binary.BigEndian.PutUint64(b[40:48], m.BaseBytes)
	copy(b[48:80], m.BaseHash[:])
	copy(b[80:112], m.PageOneHash[:])
	return b
}
func decodeManifest(b []byte) (manifest, error) {
	var m manifest
	if len(b) != manifestSize || string(b[:8]) != "GSEMNF01" || !zero(b[112:]) {
		return m, encryption.ErrFormat
	}
	copy(m.BaseID[:], b[8:40])
	m.BaseBytes = binary.BigEndian.Uint64(b[40:48])
	copy(m.BaseHash[:], b[48:80])
	copy(m.PageOneHash[:], b[80:112])
	if m.BaseID == ([32]byte{}) {
		return m, encryption.ErrFormat
	}
	return m, nil
}
func metadata(role encryption.FileRole, p1 [32]byte) []byte {
	b := make([]byte, metaSize)
	copy(b, "GSEBASE1")
	if role == encryption.FileBackup {
		copy(b, "GSEBAK01")
	}
	copy(b[8:40], p1[:])
	return b
}
func parseMetadata(b []byte, role encryption.FileRole) ([32]byte, error) {
	var h [32]byte
	magic := "GSEBASE1"
	if role == encryption.FileBackup {
		magic = "GSEBAK01"
	}
	if len(b) != metaSize || string(b[:8]) != magic || !zero(b[40:]) {
		return h, encryption.ErrFormat
	}
	copy(h[:], b[8:40])
	return h, nil
}
func footer(hash [32]byte, size uint64) []byte {
	b := make([]byte, footerSize)
	copy(b, "GSEEND01")
	copy(b[8:40], hash[:])
	binary.BigEndian.PutUint64(b[40:48], size)
	return b
}
func zero(b []byte) bool { return bytes.Count(b, []byte{0}) == len(b) }
func objectSize(g Geometry) uint64 {
	return encryption.HeaderSize + recordOverhead + metaSize + uint64(g.PageCount)*(uint64(g.PageSize)+recordOverhead) + recordOverhead + footerSize
}
func pageOffset(g Geometry, n uint32) int64 {
	return encryption.HeaderSize + recordOverhead + metaSize + int64(n-1)*(int64(g.PageSize)+recordOverhead)
}
func lockPage(size uint32) uint32 { return 0x40000000/size + 1 }
