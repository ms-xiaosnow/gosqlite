package sqlite_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/fileview"
)

// This is a symbolic key domain for the same-credential healthy fixtures below.
// Epoch contains the public master-KDF inputs; FileID and both roles are the
// file-KDF inputs. Geometry, commit identity, page/frame routing and ciphertext
// are AAD or payload, not key domains, so they must not mask nonce reuse here.
// No credential, master key or derived key is obtained or logged by this probe.
type encryptedNoncePair struct {
	epoch      encryption.Epoch
	fileID     [32]byte
	fileRole   encryption.FileRole
	recordRole encryption.RecordRole
	nonce      [encryption.NonceSize]byte
}

func encryptedPublicNoncePair(h encryption.Header, r encryption.RecordHeader) encryptedNoncePair {
	p := encryptedNoncePair{epoch: h.Epoch, fileID: h.FileID, fileRole: h.Descriptor.Role, recordRole: r.Role}
	copy(p.nonce[:4], h.NoncePrefix[:])
	binary.BigEndian.PutUint64(p.nonce[4:], r.Sequence)
	return p
}

type encryptedNonceObject struct {
	header encryption.Header
	digest [32]byte
}

type encryptedNonceSnapshot struct {
	root    encryption.Header
	objects map[encryption.FileIdentity]encryptedNonceObject
	pairs   map[encryptedNoncePair]struct{}
}

// Read only intact files produced by these small ordinary tests. Public record
// roles/sequences suffice for nonce accounting; WAL page routing is encrypted
// metadata and is deliberately not invented here. This is not authentication,
// recovery, a parser-fuzz entry, or a replacement for VerifyEncrypted.
func captureEncryptedNonces(t *testing.T, path string) encryptedNonceSnapshot {
	t.Helper()
	s := encryptedNonceSnapshot{objects: make(map[encryption.FileIdentity]encryptedNonceObject), pairs: make(map[encryptedNoncePair]struct{})}
	dir, root := filepath.Dir(path), filepath.Base(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 || len(entries) > 64 {
		t.Fatal("nonce fixture inventory outside fixed file bound")
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == root+".gse-lock" {
			continue
		}
		role := encryption.FileRoot
		if name != root {
			if !strings.HasPrefix(name, root+".gse-") {
				t.Fatal("unexpected nonce fixture member")
			}
			switch {
			case strings.HasSuffix(name, ".base"):
				role = encryption.FileBase
			case strings.HasSuffix(name, ".wal"):
				role = encryption.FileWAL
			case strings.HasSuffix(name, ".retained"):
			default:
				t.Fatal("unexpected nonce fixture object kind")
			}
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() < encryption.HeaderSize || info.Size() > 4<<20 {
			t.Fatal("nonce fixture object outside fixed byte bound", err)
		}
		raw, err := fileview.ReadFile(filepath.Join(dir, name))
		if err != nil || int64(len(raw)) != info.Size() {
			t.Fatal("nonce fixture read incomplete", err)
		}
		in := bytes.NewReader(raw)
		h, err := encryption.ReadHeader(in, encryption.Limits{MaxRecords: 256, MaxRecordBytes: 1 << 16})
		if err != nil || h.Descriptor.Role != role {
			t.Fatal("nonce fixture public header invalid", err)
		}
		if _, exists := s.objects[h.Identity()]; exists {
			t.Fatal("duplicate object identity in nonce fixture")
		}
		s.objects[h.Identity()] = encryptedNonceObject{header: h, digest: sha256.Sum256(raw)}
		if name == root {
			s.root = h
		}
		for sequence := uint64(0); sequence < h.Descriptor.RecordCount; sequence++ {
			record, err := encryption.ReadRecord(in, encryption.Limits{MaxRecords: 256, MaxRecordBytes: 1 << 16})
			if err != nil {
				t.Fatal("nonce fixture record incomplete", err)
			}
			r, err := encryption.DecodeRecordHeader(record[:encryption.RecordHeaderSize], encryption.Limits{MaxRecords: 256, MaxRecordBytes: 1 << 16})
			wantRole := encryption.RecordData
			if sequence == 0 {
				wantRole = encryption.RecordMetadata
			} else if sequence == h.Descriptor.RecordCount-1 {
				wantRole = encryption.RecordFooter
			}
			if err != nil || r.Sequence != sequence || r.Role != wantRole {
				t.Fatal("nonce fixture record identity invalid", err)
			}
			pair := encryptedPublicNoncePair(h, r)
			if _, exists := s.pairs[pair]; exists {
				t.Fatal("duplicate symbolic key-domain/nonce in retained fixture")
			}
			s.pairs[pair] = struct{}{}
			if len(s.pairs) > 4096 {
				t.Fatal("nonce fixture inventory outside fixed record bound")
			}
		}
		if in.Len() != 0 {
			t.Fatal("nonce fixture has trailing bytes")
		}
	}
	if s.root.FileID == ([32]byte{}) || len(s.pairs) == 0 {
		t.Fatal("nonce fixture root or records missing")
	}
	return s
}

// A successful bounded publication creates one object and one root, retaining
// every previously observed object byte-for-byte. Rescanning retained ciphertext
// is not a new Seal: count it once, then check new pairs against that same set.
func assertEncryptedNonceAdvance(t *testing.T, before, after encryptedNonceSnapshot, objectRole encryption.FileRole) {
	t.Helper()
	if len(after.objects) != len(before.objects)+2 || len(after.pairs) <= len(before.pairs) || after.root.FileID == before.root.FileID {
		t.Fatal("publication did not add exactly two fresh encrypted objects")
	}
	if after.root.Descriptor.CommitSequence != before.root.Descriptor.CommitSequence+1 {
		t.Fatal("nonce publication is not the immediate successor")
	}
	for id, old := range before.objects {
		if current, exists := after.objects[id]; !exists || current != old {
			t.Fatal("previous nonce-bearing object was not retained unchanged")
		}
	}
	for pair := range before.pairs {
		if _, exists := after.pairs[pair]; !exists {
			t.Fatal("previous symbolic key-domain/nonce missing after publication")
		}
	}
	newRoots, newObjects := 0, 0
	for id, object := range after.objects {
		if _, exists := before.objects[id]; exists {
			continue
		}
		if object.header.Epoch != after.root.Epoch {
			t.Fatal("new object does not use the published epoch")
		}
		switch object.header.Descriptor.Role {
		case encryption.FileRoot:
			newRoots++
		case objectRole:
			newObjects++
			// Root repeats its selected object's descriptor except for role
			// and record count. WAL ParentCommitID is a logical object-head
			// ID, not the preceding root's byte digest.
			descriptor := object.header.Descriptor
			descriptor.Role, descriptor.RecordCount = encryption.FileRoot, 1
			if descriptor != after.root.Descriptor {
				t.Fatal("new nonce-bearing object does not match the published root")
			}
		default:
			t.Fatal("unexpected new object in nonce publication")
		}
	}
	if newRoots != 1 || newObjects != 1 {
		t.Fatal("nonce publication lacks its new root/object pair")
	}
}

func TestEncryptionPublicNonceDomainIdentity(t *testing.T) {
	h := encryption.Header{Epoch: encryption.Epoch{DatabaseID: [16]byte{1}, EpochID: [32]byte{2}, RootSalt: [32]byte{3}}, FileID: [32]byte{4}, NoncePrefix: [4]byte{1, 2, 3, 4}, Descriptor: encryption.FileDescriptor{Role: encryption.FileBase}}
	r := encryption.RecordHeader{Role: encryption.RecordData, Sequence: 0x05060708090a0b0c}
	pair := encryptedPublicNoncePair(h, r)
	if pair.nonce != ([12]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}) {
		t.Fatal("public nonce is not prefix plus big-endian sequence")
	}
	// Equal nonces in distinct derivation domains are permitted. No keys are
	// derived and no files are written by these pure-value controls.
	for _, change := range []func(*encryption.Header){
		func(h *encryption.Header) { h.Epoch.DatabaseID[0]++ },
		func(h *encryption.Header) { h.Epoch.EpochID[0]++ },
		func(h *encryption.Header) { h.Epoch.RootSalt[0]++ },
		func(h *encryption.Header) { h.Epoch.KDFSalt[0]++ },
		func(h *encryption.Header) { h.Epoch.KDF.Passes++ },
		func(h *encryption.Header) { h.Epoch.Kind++ },
		func(h *encryption.Header) { h.FileID[0]++ },
		func(h *encryption.Header) { h.Descriptor.Role = encryption.FileWAL },
	} {
		next := h
		change(&next)
		if encryptedPublicNoncePair(next, r) == pair {
			t.Fatal("distinct public derivation domains collapsed")
		}
	}
	nextRecord := r
	nextRecord.Role = encryption.RecordFooter
	if encryptedPublicNoncePair(h, nextRecord) == pair {
		t.Fatal("record-role key domain collapsed")
	}
	// AAD/payload changes cannot disguise reuse of a key-domain/nonce pair.
	next := h
	next.Descriptor.PageSize, next.Descriptor.PageCount, next.Descriptor.RecordCount = 512, 3, 5
	next.Descriptor.CommitSequence++
	next.Descriptor.TransactionID[0]++
	next.Descriptor.ParentCommitID[0]++
	nextRecord = r
	nextRecord.PlaintextSize = 512
	if encryptedPublicNoncePair(next, nextRecord) != pair {
		t.Fatal("AAD or payload length masked a repeated key-domain/nonce")
	}
}
