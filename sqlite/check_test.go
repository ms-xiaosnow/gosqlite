package sqlite_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/fileview"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func checkExec(t *testing.T, db *sqlite.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(context.Background(), q, args...); err != nil {
		t.Fatal(q, err)
	}
}
func checkOpen(t *testing.T, path string, o sqlite.Options) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(path, o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}
func checkFamily(t *testing.T, dir string) map[string][32]byte {
	t.Helper()
	out := map[string][32]byte{}
	err := filepath.WalkDir(dir, func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		p, err := fileview.ReadFile(path)
		if err != nil {
			return err
		}
		out[path] = sha256.Sum256(p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func checkPass(t *testing.T, db *sqlite.DB) sqlite.CheckResult {
	t.Helper()
	r, err := db.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Scope != sqlite.CheckScope || r.PageReads == 0 {
		t.Fatal(r)
	}
	return r
}

func TestNativeCheckOrdinaryPageSizesAndReadOnly(t *testing.T) {
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "normal.db")
			db := checkOpen(t, path, sqlite.Options{PageSize: size})
			checkExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT NOT NULL,n NUMERIC,r REAL,b BLOB)")
			checkExec(t, db, "CREATE INDEX t_key ON t(k,n)")
			checkExec(t, db, "CREATE INDEX t_id ON t(id,r,b)")
			values := []any{nil, int64(9007199254740993), float64(1.25), "123text", []byte{0, 1, 2}}
			tx, err := db.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 20; i++ {
				if _, err = tx.Exec(context.Background(), "INSERT INTO t VALUES(?,?,?,?,?)", int64(i-10), "数据库\x00"+strings.Repeat("v", 700), values[i%len(values)], float64(i)/2, []byte{byte(i)}); err != nil {
					t.Fatal(err)
				}
			}
			if r, err := tx.Check(context.Background()); err != nil || r.TableRows != 20 || r.IndexEntries != 40 {
				t.Fatal(r, err)
			}
			if err = tx.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
			r := checkPass(t, db)
			if r.Tables != 1 || r.Indexes != 2 || r.TableRows != 20 || r.IndexEntries != 40 {
				t.Fatal(r)
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			before := checkFamily(t, dir)
			ro := checkOpen(t, path, sqlite.Options{ReadOnly: true, NoCreate: true})
			again := checkPass(t, ro)
			if again != r {
				t.Fatal(r, again)
			}
			for _, o := range []sqlite.CheckOptions{{MaxPages: r.Pages - 1}, {MaxPageReads: r.PageReads - 1}} {
				got, err := ro.Check(context.Background(), o)
				if !errors.Is(err, sqlite.ErrMemoryLimit) || got != (sqlite.CheckResult{}) {
					t.Fatal(got, err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if got, err := ro.Check(ctx); !errors.Is(err, context.Canceled) || got != (sqlite.CheckResult{}) {
				t.Fatal(got, err)
			}
			if err = ro.Close(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, checkFamily(t, dir)) {
				t.Fatal("read-only check changed database family")
			}
		})
	}
}
func TestNativeCheckFreelistRollbackAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "free.db")
	db := checkOpen(t, path, sqlite.Options{PageSize: 512})
	checkExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT)")
	checkExec(t, db, "CREATE INDEX t_key ON t(k)")
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 80; i++ {
		if _, err = tx.Exec(context.Background(), "INSERT INTO t VALUES(?,?)", i, strings.Repeat("overflow", 100)); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	checkExec(t, db, "DELETE FROM t WHERE id<60")
	r := checkPass(t, db)
	if r.FreePages == 0 || r.TableRows != 20 || r.IndexEntries != 20 {
		t.Fatal(r)
	}
	tx, err = db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), "UPDATE t SET k='temporary' WHERE id>=60"); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if got := checkPass(t, db); got != r {
		t.Fatal(got, r)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db = checkOpen(t, path, sqlite.Options{ReadOnly: true})
	if got := checkPass(t, db); got != r {
		t.Fatal(got, r)
	}
}
func TestNativeCheckWALPinnedSnapshots(t *testing.T) {
	ctx := context.Background()
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "wal.db")
			opts := sqlite.Options{WALExclusive: !shared, WALShared: shared, KeepWALOnClose: true}
			db := checkOpen(t, path, opts)
			checkExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT)")
			checkExec(t, db, "CREATE INDEX t_key ON t(k)")
			checkExec(t, db, "INSERT INTO t VALUES(1,'one')")
			tx, err := db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
			if err != nil {
				t.Fatal(err)
			}
			a, err := tx.Check(ctx)
			if err != nil || a.TableRows != 1 {
				t.Fatal(a, err)
			}
			if shared {
				peer := checkOpen(t, path, opts)
				checkExec(t, peer, "INSERT INTO t VALUES(2,'two')")
				if err = peer.Close(); err != nil {
					t.Fatal(err)
				}
			}
			b, err := db.Check(ctx)
			if err != nil || a != b {
				t.Fatal(a, b, err)
			}
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			latest := checkPass(t, db)
			if shared && latest.TableRows != 2 {
				t.Fatal(latest)
			}
			if err = db.Checkpoint(ctx); err != nil {
				t.Fatal(err)
			}
			if got := checkPass(t, db); got.TableRows != latest.TableRows || got.IndexEntries != latest.IndexEntries {
				t.Fatal(got, latest)
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			db = checkOpen(t, path, opts)
			if got := checkPass(t, db); got.TableRows != latest.TableRows {
				t.Fatal(got)
			}
		})
	}
}
func TestNativeCheckExtensionsAndTypedValues(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "extensions.db")
	db := checkOpen(t, path, sqlite.Options{})
	checkExec(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT,v VECTOR(2),h HALFVEC(2),b BIT(3),s SPARSEVEC(2))")
	for i := 1; i <= 12; i++ {
		checkExec(t, db, "INSERT INTO docs VALUES(?,?,?,?,?,?)", i, fmt.Sprintf("数据库 %d", i), fmt.Sprintf("[%d,1]", i), fmt.Sprintf("[%d,1]", i), "101", "{1:1,2:2}/2")
	}
	for _, q := range []string{
		"CREATE INDEX text_idx ON docs(body) USING grams13",
		"CREATE INDEX dense_hnsw ON docs(v) USING hnsw WITH(opclass='vector_l2_ops',m=4,ef_construction=16)",
		"CREATE INDEX half_hnsw ON docs(h) USING hnsw WITH(opclass='halfvec_l2_ops',m=4,ef_construction=16)",
		"CREATE INDEX bit_hnsw ON docs(b) USING hnsw WITH(opclass='bit_hamming_ops',m=4,ef_construction=16)",
		"CREATE INDEX sparse_hnsw ON docs(s) USING hnsw WITH(opclass='sparsevec_l2_ops',m=4,ef_construction=16)",
		"CREATE INDEX dense_ivf ON docs(v) USING ivfflat WITH(opclass='vector_l2_ops',lists=2,training_samples=12,training_iterations=2)",
		"CREATE INDEX half_ivf ON docs(h) USING ivfflat WITH(opclass='halfvec_l2_ops',lists=2,training_samples=12,training_iterations=2)",
		"CREATE INDEX bit_ivf ON docs(b) USING ivfflat WITH(opclass='bit_hamming_ops',lists=2,training_samples=12,training_iterations=2)",
	} {
		checkExec(t, db, q)
	}
	r := checkPass(t, db)
	if r.ExtensionIndexes != 8 || r.Tables != 10 || r.Indexes != 9 {
		t.Fatal(r)
	}
	checkExec(t, db, "UPDATE docs SET body='changed 数据库',v='[22,1]' WHERE id=2")
	checkExec(t, db, "DELETE FROM docs WHERE id=3")
	checkPass(t, db)
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO docs VALUES(21,'rollback','[21,1]','[21,1]','010','{1:1}/2')"); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	checkPass(t, db)
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db = checkOpen(t, path, sqlite.Options{ReadOnly: true})
	checkPass(t, db)
}
func TestNativeCheckCorrectKeyReadOnly(t *testing.T) {
	for _, mode := range []sqlite.EncryptionMode{sqlite.EncryptionSnapshot, sqlite.EncryptionWAL, sqlite.EncryptionWALShared} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "normal.gse")
			cfg := encryptionOptions(t, encryption.RawKey256)
			cfg.Mode = mode
			db := checkOpen(t, path, sqlite.Options{Encryption: &cfg, KeepWALOnClose: mode != sqlite.EncryptionSnapshot})
			checkExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT)")
			checkExec(t, db, "CREATE INDEX t_k ON t(k)")
			checkExec(t, db, "INSERT INTO t VALUES(1,'encrypted 数据库')")
			checkPass(t, db)
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			before := checkFamily(t, dir)
			ro := checkOpen(t, path, sqlite.Options{Encryption: &cfg, ReadOnly: true, NoCreate: true, KeepWALOnClose: mode != sqlite.EncryptionSnapshot})
			r := checkPass(t, ro)
			if r.TableRows != 1 || r.IndexEntries != 1 {
				t.Fatal(r)
			}
			if err := ro.Close(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, checkFamily(t, dir)) {
				t.Fatal("read-only encrypted check changed family bytes")
			}
		})
	}
}

// The official executable creates only valid databases via ordinary SQL. It is
// an optional test oracle; no production checking delegates SQL to this process.
func checkOracle(t *testing.T, path, sql string) string {
	t.Helper()
	oracle := os.Getenv("SQLITE_ORACLE")
	if oracle == "" {
		t.Skip("SQLITE_ORACLE is required for official supported-profile fixtures")
	}
	cmd := exec.Command(oracle, path, sql)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(string(out), err)
	}
	return strings.TrimSpace(string(out))
}
func TestNativeCheckOfficialSupportedFixtures(t *testing.T) {
	for _, size := range []int{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "official.db")
			sql := fmt.Sprintf("PRAGMA page_size=%d; CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT NOT NULL,n NUMERIC,r REAL,b BLOB); INSERT INTO t VALUES(-9223372036854775808,'数据库',9007199254740993,3.0,X'0001'),(9223372036854775807,'end',1.25,2.5,X''),(0,'',NULL,NULL,NULL); ALTER TABLE t ADD COLUMN extra TEXT DEFAULT 'literal'; CREATE INDEX t_kn ON t(k,n,extra); CREATE INDEX t_id ON t(id,r,b); PRAGMA integrity_check;", size)
			if got := checkOracle(t, path, sql); got != "ok" {
				t.Fatal(got)
			}
			before := checkFamily(t, dir)
			db := checkOpen(t, path, sqlite.Options{ReadOnly: true})
			r := checkPass(t, db)
			if r.TableRows != 3 || r.IndexEntries != 6 {
				t.Fatal(r)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, checkFamily(t, dir)) {
				t.Fatal("official read-only fixture changed")
			}
		})
	}
}
func TestNativeCheckOfficialUnsupportedCatalogAndDirtyExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unsupported.db")
	checkOracle(t, path, "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT); CREATE INDEX descending ON t(k DESC);")
	db := checkOpen(t, path, sqlite.Options{ReadOnly: true})
	if r, err := db.Check(context.Background()); !errors.Is(err, sqlite.ErrUnsupported) || r != (sqlite.CheckResult{}) {
		t.Fatal(r, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(t.TempDir(), "dirty.db")
	db = checkOpen(t, path, sqlite.Options{})
	checkExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT)")
	checkExec(t, db, "INSERT INTO t VALUES(1,'数据库')")
	checkExec(t, db, "CREATE INDEX grams ON t(k) USING grams13")
	checkPass(t, db)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	checkOracle(t, path, "UPDATE t SET k='official update' WHERE id=1;")
	db = checkOpen(t, path, sqlite.Options{})
	if r, err := db.Check(context.Background()); !errors.Is(err, sqlite.ErrReindexRequired) || r != (sqlite.CheckResult{}) {
		t.Fatal(r, err)
	}
	checkExec(t, db, "REINDEX grams")
	checkPass(t, db)
}

func TestNativeCheckOfficialFragmentedTrees(t *testing.T) {
	for _, size := range []int{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "fragmented.db")
			sql := fmt.Sprintf("PRAGMA page_size=%d; CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT,n NUMERIC); WITH RECURSIVE rows(n) AS (VALUES(1) UNION ALL SELECT n+1 FROM rows WHERE n<900) INSERT INTO t SELECT n,printf('%%0100d',n),n FROM rows; CREATE INDEX by_key ON t(k,n); DELETE FROM t WHERE id%%3=0; UPDATE t SET k=substr(k,1,7)||id WHERE id%%4=0; PRAGMA integrity_check;", size)
			if result := checkOracle(t, path, sql); result != "ok" {
				t.Fatal(result)
			}
			db := checkOpen(t, path, sqlite.Options{ReadOnly: true})
			result := checkPass(t, db)
			if result.TableRows != 600 || result.IndexEntries != 600 {
				t.Fatal(result)
			}
		})
	}
}
