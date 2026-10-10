//go:build oracle

package testkit_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// These tests exercise only correct credentials, intact files, ordinary SQL,
// successful rekey/copy/normal reopen and official read-only SQLite inspection.
// Authentication-negative cases remain memory-only in package encryption.
// No tampered-on-disk, interrupted publication, wrong-key disk open, injected
// I/O failure, forced process exit or physical recovery workflow is run here.
type ordinaryEncryptionProvider struct{ key []byte }
type ordinaryEncryptionMaterial struct{ key []byte }

func (p ordinaryEncryptionProvider) Acquire(ctx context.Context, _ encryption.KeyRequest) (encryption.KeyMaterial, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &ordinaryEncryptionMaterial{key: append([]byte(nil), p.key...)}, nil
}
func (m *ordinaryEncryptionMaterial) Kind() encryption.CredentialKind { return encryption.RawKey256 }
func (m *ordinaryEncryptionMaterial) Bytes() []byte                   { return m.key }
func (m *ordinaryEncryptionMaterial) Release()                        { clear(m.key); m.key = nil }
func ordinaryEncryptionOptions(t *testing.T) sqlite.EncryptionOptions {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(key) })
	return sqlite.EncryptionOptions{KeyProvider: ordinaryEncryptionProvider{key: key}, CredentialKind: encryption.RawKey256}
}

func TestOracleEncryptedSnapshotExportImport(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	const sentinel = "private-marker-for-native-encrypted-snapshot-你好-東京-😀-abcdefghijk"
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			dir := t.TempDir()
			cipherDir := filepath.Join(dir, "cipher")
			if err := os.Mkdir(cipherDir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(cipherDir, "database.gse")
			key := ordinaryEncryptionOptions(t)
			target := ordinaryEncryptionOptions(t)
			backupKey := ordinaryEncryptionOptions(t)
			restoreKey := ordinaryEncryptionOptions(t)
			importKey := ordinaryEncryptionOptions(t)
			opts := sqlite.Options{PageSize: size, Encryption: &key}
			db, err := sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if db != nil {
					if e := db.Close(); e != nil {
						t.Error(e)
					}
				}
			}()
			writeExec(t, db, "CREATE TABLE documents(id INTEGER PRIMARY KEY,title TEXT,embedding VECTOR(3) NOT NULL)")
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for i := 1; i <= 48; i++ {
				writeTxExec(t, tx, "INSERT INTO documents VALUES(?,?,?)", i, fmt.Sprintf("%s 数据库 %d", sentinel, i), fmt.Sprintf("[%d,1,2]", i))
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err = db.CreateGramsIndex(ctx, "document_grams", "documents", []string{"title"}); err != nil {
				t.Fatal(err)
			}
			if err = db.CreateHNSWIndex(ctx, "document_hnsw", "documents", "embedding"); err != nil {
				t.Fatal(err)
			}
			tx, err = db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			writeTxExec(t, tx, "UPDATE documents SET title='temporary rollback value',embedding='[999,1,2]' WHERE id=1")
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"document_grams", "document_hnsw"} {
				if err = db.CheckExtensionIndex(ctx, name); err != nil {
					t.Fatal(err)
				}
			}
			info, err := db.VerifyEncrypted(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if info.PageSize != size || info.PageCount == 0 {
				t.Fatal(info)
			}
			check := func(d *sqlite.DB) {
				t.Helper()
				rows, e := d.Query(ctx, "SELECT id,title,vector_dims(embedding) FROM documents WHERE id=1")
				if e != nil {
					t.Fatal(e)
				}
				if !rows.Next() {
					t.Fatalf("missing row:%v", rows.Err())
				}
				v := append([]any(nil), rows.Values()...)
				e = rows.Close()
				if e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(v, []any{int64(1), sentinel + " 数据库 1", int64(3)}) {
					t.Fatal(v)
				}
				r, e := d.VectorSearch(ctx, "documents", "embedding", "[20,1,2]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: "document_hnsw", Limit: 3})
				if e != nil {
					t.Fatal(e)
				}
				if r.Plan.Method != "hnsw" || r.Plan.Exact || len(r.Hits) != 3 || r.Hits[0].RowID != 20 {
					t.Fatal(r)
				}
			}
			check(db)
			if err = db.Rekey(ctx, target); err != nil {
				t.Fatal(err)
			}
			check(db)
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			db = nil
			opts.Encryption = &target
			db, err = sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			check(db)
			portable := filepath.Join(dir, "portable.gsb")
			backup, err := db.BackupEncrypted(ctx, portable, backupKey)
			if err != nil {
				t.Fatal(err)
			}
			if backup.PageSize != size || backup.PageCount == 0 {
				t.Fatal(backup)
			}
			plain := filepath.Join(dir, "export.db")
			if _, err = db.ExportPlaintext(ctx, plain); err != nil {
				t.Fatal(err)
			}
			writeSame(t, db, plain, "SELECT id,title,hex(embedding) FROM documents ORDER BY id")
			writeIntegrity(t, plain)
			before, err := os.ReadFile(plain)
			if err != nil {
				t.Fatal(err)
			}
			plainHash := sha256.Sum256(before)
			restore := filepath.Join(dir, "restored.gse")
			restored, err := sqlite.RestoreEncrypted(ctx, portable, restore, backupKey, restoreKey)
			if err != nil {
				t.Fatal(err)
			}
			if restored.DatabaseID == info.DatabaseID {
				t.Fatal("restore reused source DB identity")
			}
			rd, e := sqlite.Open(restore, sqlite.Options{Encryption: &restoreKey})
			if e != nil {
				t.Fatal(e)
			}
			check(rd)
			if e = rd.Close(); e != nil {
				t.Fatal(e)
			}
			imported := filepath.Join(dir, "imported.gse")
			if _, err = sqlite.ImportEncrypted(ctx, plain, imported, sqlite.Options{ReadOnly: true}, importKey); err != nil {
				t.Fatal(err)
			}
			idb, e := sqlite.Open(imported, sqlite.Options{Encryption: &importKey})
			if e != nil {
				t.Fatal(e)
			}
			check(idb)
			if e = idb.Close(); e != nil {
				t.Fatal(e)
			}
			after, err := os.ReadFile(plain)
			if err != nil {
				t.Fatal(err)
			}
			if sha256.Sum256(after) != plainHash {
				t.Fatal("import changed plaintext source")
			}
			// Search only retained ciphertext files in the dedicated directory. This
			// is not proof about transient writes; backend source audit covers routing.
			files := 0
			err = filepath.WalkDir(cipherDir, func(p string, e os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if e.IsDir() {
					return nil
				}
				b, readErr := os.ReadFile(p)
				if readErr != nil {
					return readErr
				}
				files++
				for _, marker := range []string{sentinel, "CREATE TABLE documents", "CREATE TABLE __gosqlite_ext_"} {
					if bytes.Contains(b, []byte(marker)) {
						return fmt.Errorf("retained ciphertext exposed marker in %s", filepath.Base(p))
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if files < 2 {
				t.Fatal("no encrypted object/lock family")
			}
			root, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(root), encryption.Magic) {
				t.Fatal("independent encrypted root magic missing")
			}
		})
	}
}
