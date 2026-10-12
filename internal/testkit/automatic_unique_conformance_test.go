//go:build oracle

package testkit_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// The existing history/confCLI helpers use testoracle.ForTest's fixed isolated
// invocation. No additional process launcher, startup mode or fault path lives
// in this source. All fixtures are healthy SQL-created ordinary rowid tables.
func TestOracleAutomaticUniqueConstraintHistories(t *testing.T) {
	steps := []uniqueOracleStep{
		{query: "CREATE TABLE t(id INTEGER PRIMARY KEY UNIQUE,k NUMERIC CONSTRAINT uq_key UNIQUE,note TEXT,UNIQUE(k),UNIQUE(k,note))"},
		{query: "CREATE UNIQUE INDEX explicit_k ON t(k)"},
		{query: "CREATE INDEX ordinary_note ON t(note)"},
		{query: "SELECT type,name,tbl_name,sql IS NULL,typeof(sql) FROM sqlite_schema ORDER BY name", read: true},
		{query: "INSERT INTO t VALUES(0,'01','base'),(-7,2,'second')"},
		{query: "SELECT id,k,typeof(k),note FROM t ORDER BY id", read: true},
		{query: "INSERT INTO t VALUES(99,1.0,'first conflict')", constraint: true},
		{query: "INSERT INTO t VALUES(8,8,'partial'),(9,'02','later conflict')", constraint: true},
		{query: "UPDATE t SET k=3-k", constraint: true},
		{query: "SELECT id,k,note FROM t ORDER BY id", read: true},
		{query: "UPDATE t SET k=k WHERE id=0"},
		{query: "UPDATE t SET id=10 WHERE id=0"},
		{query: "UPDATE t SET id=0 WHERE id=10"},
		{query: "UPDATE t SET k=1 WHERE id=-7", constraint: true},
		{query: "BEGIN"},
		{query: "INSERT INTO t VALUES(30,3,'kept')"},
		{query: "SAVEPOINT s"},
		{query: "INSERT INTO t VALUES(40,4,'partial'),(41,1,'conflict')", constraint: true},
		{query: "UPDATE t SET k=CASE WHEN id=-7 THEN 7 ELSE 3 END,note='partial' WHERE id<=0", constraint: true},
		{query: "SELECT id,k,note FROM t ORDER BY id", read: true},
		{query: "CREATE TABLE undone(id INTEGER PRIMARY KEY UNIQUE,k UNIQUE,UNIQUE(id,k))"},
		{query: "INSERT INTO undone VALUES(1,'one')"},
		{query: "INSERT INTO undone VALUES(2,'one')", constraint: true},
		{query: "ROLLBACK TO s"},
		{query: "SELECT name FROM sqlite_schema WHERE tbl_name='undone' ORDER BY name", read: true},
		{query: "CREATE TABLE undone(id INTEGER PRIMARY KEY,k)"},
		{query: "INSERT INTO undone VALUES(1,'same'),(2,'same')"},
		{query: "RELEASE s"},
		{query: "COMMIT"},
		{query: "CREATE TABLE IF NOT EXISTS t(id INTEGER PRIMARY KEY,k)"},
		{query: "CREATE TABLE IF NOT EXISTS undone(id INTEGER PRIMARY KEY,k UNIQUE)"},
		{query: "INSERT INTO undone VALUES(3,'same')"},
		{query: "INSERT INTO t VALUES(50,NULL,'base'),(51,NULL,'base')"},
		{query: "DELETE FROM t WHERE id=0"},
		{query: "INSERT INTO t VALUES(0,1,'reused')"},
		{query: "BEGIN"},
		{query: "CREATE TABLE rolled_back(k UNIQUE)"},
		{query: "UPDATE t SET k=9 WHERE id=0"},
		{query: "ROLLBACK"},
		{query: "INSERT INTO t VALUES(52,1,'conflict after rollback')", constraint: true},
		{query: "SELECT type,name,tbl_name,sql IS NULL,typeof(sql) FROM sqlite_schema ORDER BY name", read: true},
		{query: "SELECT id,k,typeof(k),note FROM t ORDER BY id", read: true},
		{query: "CREATE TABLE composite(id INTEGER PRIMARY KEY,a,b,c,CONSTRAINT abc UNIQUE(a,b,c),UNIQUE(c,b,a),UNIQUE(a,a,b,c))"},
		{query: "INSERT INTO composite VALUES(1,1,2,3),(2,1,2,4),(3,1,3,3),(4,2,2,3)"},
		{query: "INSERT INTO composite VALUES(5,1,2,3)", constraint: true},
		{query: "INSERT INTO composite VALUES(10,NULL,2,3),(11,NULL,2,3),(12,1,NULL,3),(13,1,NULL,3),(14,1,2,NULL),(15,1,2,NULL),(16,NULL,NULL,NULL),(17,NULL,NULL,NULL)"},
		{query: "SELECT id,a,b,c FROM composite ORDER BY id", read: true},
		{query: "CREATE TABLE defaults(id INTEGER PRIMARY KEY,k TEXT DEFAULT 'default' UNIQUE,n NUMERIC DEFAULT '01',UNIQUE(n,k))"},
		{query: "INSERT INTO defaults DEFAULT VALUES"},
		{query: "INSERT INTO defaults DEFAULT VALUES", constraint: true},
		{query: "INSERT INTO defaults(k) VALUES(NULL),(NULL)"},
		{query: "SELECT id,k,n,typeof(n) FROM defaults ORDER BY id", read: true},
	}
	for _, size := range []uint32{512, 4096, 65536} {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("page%d/shared%t", size, shared), func(t *testing.T) {
				uniqueOracleHistory(t, size, shared, steps)
			})
		}
	}
}

func TestOracleAutomaticUniqueExactNumericAndAffinity(t *testing.T) {
	steps := []uniqueOracleStep{
		{query: "CREATE TABLE exact(id INTEGER PRIMARY KEY UNIQUE,k UNIQUE)"},
		{query: "INSERT INTO exact VALUES(1,-9223372036854775808),(2,9223372036854775807),(3,9007199254740991),(4,9007199254740992),(5,9007199254740993),(6,9007199254740994.0),(7,9.223372036854776e18),(8,0),(9,'1'),(10,X'31'),(11,CAST(X'6100E7958C' AS TEXT)),(12,X'6100E7958C')"},
		{query: "INSERT INTO exact VALUES(100,-9.223372036854776e18)", constraint: true},
		{query: "INSERT INTO exact VALUES(100,9007199254740992.0)", constraint: true},
		{query: "INSERT INTO exact VALUES(100,9007199254740994)", constraint: true},
		{query: "INSERT INTO exact VALUES(100,-0.0)", constraint: true},
		{query: "INSERT INTO exact VALUES(100,CAST(X'6100E7958C' AS TEXT))", constraint: true},
		{query: "INSERT INTO exact VALUES(100,X'6100E7958C')", constraint: true},
		{query: "SELECT id,typeof(k),hex(k) FROM exact WHERE id>=11 ORDER BY id", read: true},
		{query: "SELECT id,k,typeof(k) FROM exact WHERE id<=10 ORDER BY id", read: true},
		{query: "CREATE TABLE affinities(id INTEGER PRIMARY KEY,n NUMERIC UNIQUE,r REAL UNIQUE,t TEXT UNIQUE,b BLOB UNIQUE)"},
		{query: "INSERT INTO affinities VALUES(1,'01','01',1,1)"},
		{query: "INSERT INTO affinities(id,n) VALUES(2,1.0)", constraint: true},
		{query: "INSERT INTO affinities(id,r) VALUES(2,'1e0')", constraint: true},
		{query: "INSERT INTO affinities(id,t) VALUES(2,1)", constraint: true},
		{query: "INSERT INTO affinities(id,b) VALUES(2,1.0)", constraint: true},
		{query: "INSERT INTO affinities VALUES(2,X'31',1.25,'01','1'),(3,NULL,NULL,NULL,NULL),(4,NULL,NULL,NULL,NULL)"},
		{query: "SELECT id,n,typeof(n),r,typeof(r),t,typeof(t),b,typeof(b) FROM affinities ORDER BY id", read: true},
		{query: "CREATE TABLE signed(id INTEGER PRIMARY KEY UNIQUE,k TEXT UNIQUE)"},
		{query: "INSERT INTO signed VALUES(-9223372036854775808,'min'),(-7,'negative'),(0,'zero'),(9223372036854775807,'max')"},
		{query: "UPDATE signed SET id=id,k=k"},
		{query: "UPDATE signed SET id=2 WHERE id=0"},
		{query: "UPDATE signed SET id=0 WHERE id=2"},
		{query: "UPDATE signed SET k='zero' WHERE id=-7", constraint: true},
		{query: "SELECT id,k FROM signed ORDER BY id", read: true},
	}
	uniqueOracleHistory(t, 512, false, steps)
}

func automaticOracleConstraint(t *testing.T, db *sqlite.DB, q string, args ...any) {
	t.Helper()
	_, err := db.Exec(context.Background(), q, args...)
	var detail *sqlite.Error
	if !errors.Is(err, sqlite.ErrConstraint) || !errors.As(err, &detail) || detail.Code != sqlite.CodeConstraint {
		t.Fatalf("%s: want primary constraint error, got %v", q, err)
	}
}

func automaticOracleIdentifier(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func automaticOracleMetadata(t *testing.T, db *sqlite.DB, path, table, key string) {
	t.Helper()
	q := "SELECT type,name,tbl_name,sql IS NULL,typeof(sql),rootpage>=2 AS rooted FROM sqlite_schema ORDER BY name"
	writeSame(t, db, path, q)
	schema, err := db.Schema(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	roots := map[uint32]bool{}
	implicit := 0
	for _, entry := range schema {
		if entry.RootPage < 2 || roots[entry.RootPage] {
			t.Fatalf("missing/shared schema root: %+v", entry)
		}
		roots[entry.RootPage] = true
		if entry.Type == "index" && strings.HasPrefix(entry.Name, "sqlite_autoindex_") {
			implicit++
			if !entry.SQLNull || entry.SQL != "" || entry.TableName != table {
				t.Fatal(entry)
			}
		}
	}
	if implicit != 5 {
		t.Fatalf("implicit index count=%d want=5", implicit)
	}
	list := confExpected(t, path, "PRAGMA index_list("+writeQuote(table)+")")
	wantOrigin := map[string]string{"explicit_pair": "c", "ordinary_slug": "c"}
	for i := 1; i <= 5; i++ {
		wantOrigin[fmt.Sprintf("sqlite_autoindex_%s_%d", table, i)] = "u"
	}
	if len(list.rows) != len(wantOrigin) {
		t.Fatalf("PRAGMA index_list=%+v", list)
	}
	for _, row := range list.rows {
		if len(row) != 5 {
			t.Fatalf("index_list row=%#v", row)
		}
		name, ok := row[1].(string)
		origin, present := wantOrigin[name]
		unique := int64(1)
		if name == "ordinary_slug" {
			unique = 0
		}
		if !ok || !present || row[2] != unique || row[3] != origin || row[4] != int64(0) {
			t.Fatalf("index_list row=%#v", row)
		}
		delete(wantOrigin, name)
	}
	if len(wantOrigin) != 0 {
		t.Fatalf("missing index_list names=%v", wantOrigin)
	}
	keys := [][]int64{{0}, {3}, {1, 2}, {2, 1}, {1, 1, 2}}
	columns := []string{"id", key, "b", "slug"}
	for i, terms := range keys {
		name := fmt.Sprintf("sqlite_autoindex_%s_%d", table, i+1)
		xinfo := confExpected(t, path, "PRAGMA index_xinfo("+writeQuote(name)+")")
		want := make([][]any, 0, len(terms)+1)
		for position, column := range terms {
			want = append(want, []any{int64(position), column, columns[column], int64(0), "BINARY", int64(1)})
		}
		want = append(want, []any{int64(len(terms)), int64(-1), nil, int64(0), "BINARY", int64(0)})
		if !reflect.DeepEqual(xinfo.rows, want) {
			t.Fatalf("%s index_xinfo=%#v want=%#v", name, xinfo.rows, want)
		}
		// Force every real implicit root on the official side, including IPK
		// and repeated-term indexes; native execution remains a table scan.
		projection := "id,hex(" + automaticOracleIdentifier(key) + "),typeof(" + automaticOracleIdentifier(key) + "),hex(b),typeof(b),slug"
		plain := "SELECT " + projection + " FROM " + automaticOracleIdentifier(table) + " ORDER BY id"
		forced := "SELECT " + projection + " FROM " + automaticOracleIdentifier(table) + " INDEXED BY " + automaticOracleIdentifier(name) + " ORDER BY id"
		confCompare(t, confExpected(t, path, forced), confNative(t, db, plain))
	}
	if _, err := db.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	writeIntegrity(t, path)
}

func TestOracleAutomaticUniqueTwoWayFilesAndOfficialRewrites(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	seed := []string{
		`CREATE TABLE "Uni界"(id INTEGER PRIMARY KEY CONSTRAINT "id unique" UNIQUE,"Key" TEXT,b BLOB,slug TEXT UNIQUE,CONSTRAINT redundant UNIQUE(slug),UNIQUE("Key" COLLATE BINARY ASC,b),UNIQUE(b,"Key"),UNIQUE("Key","Key",b))`,
		`CREATE UNIQUE INDEX explicit_pair ON "Uni界"("Key",b)`,
		`CREATE INDEX ordinary_slug ON "Uni界"(slug)`,
		`INSERT INTO "Uni界" VALUES(0,'zero',X'00','zero'),(-7,'negative',X'FF','negative'),(1,'same',X'01','one'),(2,'same',X'02','two'),(3,NULL,X'00',NULL),(4,NULL,X'00',NULL),(5,'null',NULL,NULL),(6,'null',NULL,NULL)`,
	}
	for _, size := range []uint32{512, 4096, 65536} {
		for _, shared := range []bool{false, true} {
			for _, origin := range []string{"native", "official"} {
				t.Run(fmt.Sprintf("page%d/shared%t/%s", size, shared, origin), func(t *testing.T) {
					path := filepath.Join(t.TempDir(), "roundtrip.db")
					opts := sqlite.Options{PageSize: size, WALShared: shared}
					if origin == "official" {
						setup := fmt.Sprintf("PRAGMA page_size=%d;", size)
						if shared {
							setup += "PRAGMA journal_mode=WAL;"
						}
						confCLI(t, path, setup+strings.Join(seed, ";")+";", false)
					} else {
						created, err := sqlite.Open(path, opts)
						if err != nil {
							t.Fatal(err)
						}
						for _, q := range seed {
							writeExec(t, created, q)
						}
						if err = created.Close(); err != nil {
							t.Fatal(err)
						}
					}
					// Every direction reopens before native writes; no fixture is
					// counted as interoperable after only initial readback.
					db, err := sqlite.Open(path, opts)
					if err != nil {
						t.Fatal(err)
					}
					defer db.Close()
					automaticOracleMetadata(t, db, path, "Uni界", "Key")
					automaticOracleConstraint(t, db, `INSERT INTO "Uni界" VALUES(100,'zero',X'00','different')`)
					automaticOracleConstraint(t, db, `UPDATE "Uni界" SET "Key"='zero',b=X'00' WHERE id=-7`)
					writeExec(t, db, `UPDATE "Uni界" SET id=7 WHERE id=0`)
					writeExec(t, db, `INSERT INTO "Uni界" VALUES(8,'native',X'AA','native')`)
					// Bounded splits at every page size, and an index key beyond
					// the largest local-payload threshold, use healthy writes only.
					tx, err := db.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					for i := 20; i < 68; i++ {
						value := fmt.Sprintf("%03d-", i) + strings.Repeat("界", 700)
						writeTxExec(t, tx, `INSERT INTO "Uni界" VALUES(?,?,X'CC',NULL)`, i, value)
					}
					huge := "overflow-" + strings.Repeat("界", 14000)
					writeTxExec(t, tx, `INSERT INTO "Uni界" VALUES(100,?,X'DD',NULL)`, huge)
					if err = tx.Commit(ctx); err != nil {
						t.Fatal(err)
					}
					automaticOracleConstraint(t, db, `INSERT INTO "Uni界" VALUES(101,?,X'DD',NULL)`, huge)
					automaticOracleMetadata(t, db, path, "Uni界", "Key")
					if err = db.Close(); err != nil {
						t.Fatal(err)
					}
					// Official rewrites the names and stored table SQL while
					// retaining constraint origin. ADD is ordinary literal-default.
					confCLI(t, path, `BEGIN;
						ALTER TABLE "Uni界" RENAME TO "Renamed界";
						ALTER TABLE "Renamed界" RENAME COLUMN "Key" TO "Renamed key";
						ALTER TABLE "Renamed界" ADD COLUMN note TEXT DEFAULT 'literal';
						UPDATE "Renamed界" SET id=0 WHERE id=7;
						DELETE FROM "Renamed界" WHERE id=-7;
						INSERT INTO "Renamed界"(id,"Renamed key",b,slug) VALUES(9,'official',X'BB','official');
						COMMIT;`, false)
					writeIntegrity(t, path)
					db, err = sqlite.Open(path, opts)
					if err != nil {
						t.Fatal(err)
					}
					defer db.Close()
					automaticOracleMetadata(t, db, path, "Renamed界", "Renamed key")
					writeSame(t, db, path, `SELECT id,note FROM "Renamed界" ORDER BY id`)
					automaticOracleConstraint(t, db, `INSERT INTO "Renamed界"(id,"Renamed key",b,slug) VALUES(102,'official',X'BB','different')`)
					writeExec(t, db, `INSERT INTO "Renamed界"(id,"Renamed key",b,slug) VALUES(-7,'negative',X'FF','negative')`)
					writeExec(t, db, `UPDATE "Renamed界" SET "Renamed key"='after rewrite',note='native' WHERE id=8`)
					writeExec(t, db, `DELETE FROM "Renamed界" WHERE id>=20 AND id<30`)
					automaticOracleMetadata(t, db, path, "Renamed界", "Renamed key")
					writeSame(t, db, path, `SELECT id,note FROM "Renamed界" ORDER BY id`)
					if err = db.Close(); err != nil {
						t.Fatal(err)
					}
					// A final official connection writes after the native reopen.
					confCLI(t, path, `INSERT INTO "Renamed界"(id,"Renamed key",b,slug) VALUES(103,'last official',X'EE','last official');`, false)
					db, err = sqlite.Open(path, opts)
					if err != nil {
						t.Fatal(err)
					}
					defer db.Close()
					automaticOracleConstraint(t, db, `INSERT INTO "Renamed界"(id,"Renamed key",b,slug) VALUES(104,'last official',X'EE','different')`)
					writeExec(t, db, `UPDATE "Renamed界" SET note='final native' WHERE id=103`)
					automaticOracleMetadata(t, db, path, "Renamed界", "Renamed key")
				})
			}
		}
	}
}
