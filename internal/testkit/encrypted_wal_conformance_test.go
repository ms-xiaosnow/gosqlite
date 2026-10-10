//go:build oracle

package testkit_test

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Correct-key, intact-file, successful SQL/copy/checkpoint/rekey only. Official
// SQLite inspects explicit plaintext exports, never the encrypted container.
func TestOracleEncryptedWALCompleteHead(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "head.gse")
			key := ordinaryEncryptionOptions(t)
			key.Mode = sqlite.EncryptionWAL
			opts := sqlite.Options{Encryption: &key, PageSize: size, KeepWALOnClose: true}
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
			writeExec(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,title TEXT,bucket INTEGER,embedding VECTOR(3) NOT NULL,payload BLOB)")
			writeExec(t, db, "CREATE INDEX items_bucket ON items(bucket)")
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for i := 1; i <= 32; i++ {
				writeTxExec(t, tx, "INSERT INTO items VALUES(?,?,?,?,?)", i, fmt.Sprintf("数据库 東京 😀 %d", i), i%3, fmt.Sprintf("[%d,1,2]", i), []byte{0, 255, byte(i)})
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err = db.CreateGramsIndex(ctx, "items_grams", "items", []string{"title"}); err != nil {
				t.Fatal(err)
			}
			if err = db.CreateHNSWIndex(ctx, "items_hnsw", "items", "embedding"); err != nil {
				t.Fatal(err)
			}
			if err = db.CreateIVFFlatIndex(ctx, "items_ivf", "items", "embedding", sqlite.IVFFlatIndexOptions{Lists: 2}); err != nil {
				t.Fatal(err)
			}
			if err = db.Checkpoint(ctx); err != nil {
				t.Fatal(err)
			}
			base, err := db.VerifyEncrypted(ctx)
			if err != nil || base.WALSegments != 0 || base.Mode != sqlite.EncryptionWAL || base.StorageLayout != 2 {
				t.Fatal(base, err)
			}
			writeExec(t, db, "UPDATE items SET title='committed WAL 数据库 😀',embedding='[20,1,2]',payload=X'FF0001' WHERE id=7")
			tx, err = db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			writeTxExec(t, tx, "DELETE FROM items WHERE id=7")
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			head, err := db.VerifyEncrypted(ctx)
			if err != nil || head.WALSegments != 1 || head.WALFrames == 0 || head.WALBytes == 0 || head.HeadCommitID == base.HeadCommitID || head.DatabaseID != base.DatabaseID {
				t.Fatal(head, err)
			}
			t.Logf("page size=%d logical pages=%d committed segments=%d frames=%d bytes=%d", size, head.PageCount, head.WALSegments, head.WALFrames, head.WALBytes)
			check := func(d *sqlite.DB) {
				t.Helper()
				r, e := d.Query(ctx, "SELECT title,hex(payload),vector_dims(embedding) FROM items WHERE id=7")
				if e != nil {
					t.Fatal(e)
				}
				if !r.Next() {
					t.Fatal("missing committed WAL row", r.Err())
				}
				v := append([]any(nil), r.Values()...)
				if e = r.Close(); e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(v, []any{"committed WAL 数据库 😀", "FF0001", int64(3)}) {
					t.Fatal(v)
				}
				for _, name := range []string{"items_grams", "items_hnsw", "items_ivf"} {
					if e = d.CheckExtensionIndex(ctx, name); e != nil {
						t.Fatal(e)
					}
				}
			}
			check(db)
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			db = nil
			db, err = sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			reopened, err := db.VerifyEncrypted(ctx)
			if err != nil || reopened != head {
				t.Fatal("normal retained-head reopen changed descriptor", reopened, head, err)
			}
			check(db)
			plain := filepath.Join(dir, "export.db")
			if _, err = db.ExportPlaintext(ctx, plain); err != nil {
				t.Fatal(err)
			}
			writeSame(t, db, plain, "SELECT id,title,bucket,hex(embedding),hex(payload) FROM items ORDER BY id")
			writeSame(t, db, plain, "SELECT id,title FROM items WHERE bucket=1 ORDER BY id")
			writeIntegrity(t, plain)
			archiveKey := ordinaryEncryptionOptions(t) // Archive format remains snapshot mode.
			archive := filepath.Join(dir, "portable.gsb")
			if _, err = db.BackupEncrypted(ctx, archive, archiveKey); err != nil {
				t.Fatal(err)
			}
			for _, restore := range []bool{true, false} {
				target := ordinaryEncryptionOptions(t)
				target.Mode = sqlite.EncryptionWAL
				destination := filepath.Join(dir, fmt.Sprintf("copy-%t.gse", restore))
				var copied sqlite.EncryptionInfo
				if restore {
					copied, err = sqlite.RestoreEncrypted(ctx, archive, destination, archiveKey, target)
				} else {
					copied, err = sqlite.ImportEncrypted(ctx, plain, destination, sqlite.Options{ReadOnly: true}, target)
				}
				if err != nil || copied.DatabaseID == head.DatabaseID || copied.Mode != sqlite.EncryptionWAL || copied.WALSegments != 0 {
					t.Fatal(copied, err)
				}
				copyDB, e := sqlite.Open(destination, sqlite.Options{Encryption: &target})
				if e != nil {
					t.Fatal(e)
				}
				check(copyDB)
				if e = copyDB.Close(); e != nil {
					t.Fatal(e)
				}
			}
			target := ordinaryEncryptionOptions(t)
			target.Mode = sqlite.EncryptionWAL
			if err = db.Rekey(ctx, target); err != nil {
				t.Fatal(err)
			}
			rekeyed, err := db.VerifyEncrypted(ctx)
			if err != nil || rekeyed.DatabaseID != head.DatabaseID || rekeyed.WALSegments != 0 || rekeyed.Mode != sqlite.EncryptionWAL || rekeyed.CommitSequence <= head.CommitSequence {
				t.Fatal(rekeyed, err)
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
		})
	}
}
