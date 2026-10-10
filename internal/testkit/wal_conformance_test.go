//go:build oracle

package testkit_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// All connections finish through normal Close/.quit after successful commits.
// no_ckpt_on_close is an official supported normal-close retention option. This
// test does not interrupt a transaction or exercise a crash/recovery workflow.
func TestOracleExclusiveWALNormalInterop(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "wal.db")
			ref := filepath.Join(dir, "reference.db")
			var setup strings.Builder
			fmt.Fprintf(&setup, "PRAGMA page_size=%d;\nCREATE TABLE t(id INTEGER PRIMARY KEY,label TEXT NOT NULL,payload BLOB);\nCREATE INDEX ix ON t(label);\nBEGIN;\n", size)
			for i := 1; i <= 75; i++ {
				fmt.Fprintf(&setup, "INSERT INTO t VALUES(%d,'数据库-%03d',X'00FF');\n", i, i)
			}
			setup.WriteString("COMMIT;\n")
			confCLI(t, ref, setup.String(), false)
			// Establish WAL and deliberately retain normally committed frames after
			// normal shell shutdown, so native reads must consult the WAL overlay.
			script := fmt.Sprintf(".dbconfig no_ckpt_on_close 1\nPRAGMA page_size=%d;\nPRAGMA journal_mode=WAL;\nPRAGMA wal_autocheckpoint=0;\n", size) + setup.String() + ".quit\n"
			confCLI(t, path, script, false)
			w, err := os.Stat(path + "-wal")
			if err != nil || w.Size() <= 32 {
				t.Fatalf("official normal retained WAL: %v %v", w, err)
			}
			db, err := sqlite.Open(path, sqlite.Options{WALExclusive: true, KeepWALOnClose: true, NoCreate: true})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			query := "SELECT id,label,hex(payload) FROM t ORDER BY id"
			writeSame(t, db, ref, query)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, q := range []string{"UPDATE t SET label='native' WHERE id<8", "DELETE FROM t WHERE id>70", "INSERT INTO t VALUES(99,'added',X'01FE')"} {
				writeExec(t, db, q)
				confCLI(t, ref, q, false)
			}
			large := strings.Repeat("长文本🙂", 6000)
			blob := bytes.Repeat([]byte{0, 1, 255}, 5000)
			writeExec(t, db, "INSERT INTO t VALUES(100,?,?)", large, blob)
			confCLI(t, ref, "INSERT INTO t VALUES(100,"+writeQuote(large)+",X'"+fmt.Sprintf("%x", blob)+"');", false)
			writeSame(t, db, ref, query)
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if sha256.Sum256(before) != sha256.Sum256(after) {
				t.Fatal("WAL commits changed main database before checkpoint")
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			writeIntegrity(t, path)
			confCompare(t, confExpected(t, ref, query), confExpected(t, path, query))
			forced := "SELECT id,label,hex(payload) FROM t INDEXED BY ix ORDER BY label,id"
			confCompare(t, confExpected(t, ref, forced), confExpected(t, path, forced))
			// The official engine writes/checkpoints normally, then native reopens it.
			official := "UPDATE t SET label='official' WHERE id=99; INSERT INTO t VALUES(101,'from SQLite',NULL);"
			confCLI(t, path, official+"PRAGMA wal_checkpoint(TRUNCATE);", false)
			confCLI(t, ref, official, false)
			db, err = sqlite.Open(path, sqlite.Options{WALExclusive: true, NoCreate: true})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			writeSame(t, db, ref, query)
			if err = db.Checkpoint(ctx); err != nil {
				t.Fatal(err)
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			writeIntegrity(t, path)
			confCompare(t, confExpected(t, ref, query), confExpected(t, path, query))
		})
	}
}

func TestOracleImportedWALCheckpointWithoutNativeWrite(t *testing.T) {
	confOracle(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "import.db")
	reference := filepath.Join(dir, "reference.db")
	var setup strings.Builder
	setup.WriteString("PRAGMA page_size=512; CREATE TABLE t(id INTEGER PRIMARY KEY,label TEXT); CREATE INDEX ix ON t(label); BEGIN;\n")
	for i := 1; i <= 400; i++ {
		fmt.Fprintf(&setup, "INSERT INTO t VALUES(%d,%s);\n", i, writeQuote(fmt.Sprintf("%04d-", i)+strings.Repeat("unchanged", 25)))
	}
	setup.WriteString("COMMIT;\n")
	confCLI(t, path, setup.String(), false)
	confCLI(t, reference, setup.String(), false)
	change := "UPDATE t SET label='official small update' WHERE id=37 OR id=137 OR id=237 OR id=337;"
	confCLI(t, path, ".dbconfig no_ckpt_on_close 1\nPRAGMA journal_mode=WAL;\nPRAGMA wal_autocheckpoint=0;\n"+change+"\n.quit\n", false)
	confCLI(t, reference, change, false)
	db, err := sqlite.Open(path, sqlite.Options{WALExclusive: true, NoCreate: true, SectorSize: 4096, MaxTransactionPages: 16})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	writeSame(t, db, reference, "SELECT id,label FROM t ORDER BY id")
	// No native DML or DDL occurs before this checkpoint. Imported WAL images
	// must protect any untouched co-resident main-file sector pages themselves.
	if err = db.Checkpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	writeIntegrity(t, path)
	confCompare(t, confExpected(t, reference, "SELECT id,label FROM t ORDER BY id"), confExpected(t, path, "SELECT id,label FROM t ORDER BY id"))
	confCompare(t, confExpected(t, reference, "SELECT id,label FROM t INDEXED BY ix ORDER BY label,id"), confExpected(t, path, "SELECT id,label FROM t INDEXED BY ix ORDER BY label,id"))
}
