//go:build oracle

package testkit_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/testoracle"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type uniqueOracleStep struct {
	query      string
	read       bool
	constraint bool
}

// One normal official connection continues after expected logical constraints;
// there is no -bail, cancellation/termination hook, or physical fault injection.
// ForTest provides the existing isolated startup and confOracle verifies the
// pinned official version/source identity before either history is compared.
func uniqueOracleHistory(t *testing.T, size uint32, shared bool, steps []uniqueOracleStep) {
	t.Helper()
	opts := sqlite.Options{PageSize: size, WALShared: shared}
	path := filepath.Join(t.TempDir(), "native.db")
	db, err := sqlite.Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var script strings.Builder
	fmt.Fprintf(&script, ".bail off\nPRAGMA page_size=%d;\n", size)
	if shared {
		script.WriteString(".output /dev/null\nPRAGMA journal_mode=WAL;\n.output stdout\n")
	}
	var got [][]any
	appendQuery := func(q string) {
		t.Helper()
		r := confNative(t, db, q)
		if len(r.rows) > 0 {
			labels := make([]any, len(r.columns))
			for i, label := range r.columns {
				labels[i] = label
			}
			got = append(got, labels)
			got = append(got, r.rows...)
		}
		script.WriteString(q + ";\n")
	}
	constraints := 0
	for _, step := range steps {
		if step.read {
			appendQuery(step.query)
			continue
		}
		r, err := db.Exec(context.Background(), step.query)
		if step.constraint {
			var detail *sqlite.Error
			if !errors.Is(err, sqlite.ErrConstraint) || !errors.As(err, &detail) || detail.Code != sqlite.CodeConstraint {
				t.Fatalf("%s: want primary constraint error, got %v", step.query, err)
			}
			if n, err := r.RowsAffected(); err != nil || n != 0 {
				t.Fatal("failed statement result", n, err)
			}
			if n, err := r.LastInsertId(); err != nil || n != 0 {
				t.Fatal("failed statement insert id", n, err)
			}
			constraints++
		} else if err != nil {
			t.Fatal(step.query, err)
		}
		script.WriteString(step.query + ";\n")
		appendQuery(connectionOracleSelect)
	}
	cmd := testoracle.ForTest(t, confOracle(t), "-batch", "-quote", "-header", filepath.Join(t.TempDir(), "official.db"))
	cmd.Stdin = strings.NewReader(script.String())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, oracleErr := cmd.Output()
	var exitErr *exec.ExitError
	if constraints == 0 || !errors.As(oracleErr, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("want normal exit 1 for expected logical constraints: %v; %s", oracleErr, stderr.String())
	}
	// Pinned shell.c.in calls execution failures Runtime error. Accept the
	// shell's Error spelling as well, but reject every non-UNIQUE diagnostic.
	errorLine := regexp.MustCompile(`^(?:Runtime error|Error) near line \d+: (.*)$`)
	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	if len(lines) != constraints {
		t.Fatalf("official constraint count %d want %d: %s", len(lines), constraints, stderr.String())
	}
	for _, line := range lines {
		match := errorLine.FindStringSubmatch(line)
		if len(match) != 2 || !strings.Contains(match[1], "UNIQUE constraint failed") {
			t.Fatalf("unexpected official error: %s", line)
		}
	}
	want, err := confQuoted(out)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		for i := 0; i < len(got) && i < len(want); i++ {
			if !reflect.DeepEqual(got[i], want[i]) {
				t.Fatalf("history row %d native=%#v official=%#v", i, got[i], want[i])
			}
		}
		t.Fatalf("history row count native=%d official=%d", len(got), len(want))
	}
	if _, err = db.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	writeIntegrity(t, path)
}

func TestOracleUniqueIndexConstraintHistories(t *testing.T) {
	steps := []uniqueOracleStep{
		{query: "CREATE TABLE t(id INTEGER PRIMARY KEY,k NUMERIC,note TEXT)"},
		// Ordinary CREATE UNIQUE INDEX is a positive on both engines. Unsupported
		// partial/expression forms belong to distinct admission tests.
		{query: "CREATE UNIQUE INDEX unique_k ON t(k COLLATE BINARY ASC)"},
		{query: "CREATE INDEX ordinary_note ON t(note)"},
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
		{query: "SELECT id,k,note FROM t ORDER BY id", read: true},
		{query: "UPDATE t SET k=CASE WHEN id=-7 THEN 7 ELSE 3 END,note='partial' WHERE id<=0", constraint: true},
		{query: "ROLLBACK TO s"},
		{query: "RELEASE s"},
		{query: "COMMIT"},
		{query: "CREATE UNIQUE INDEX IF NOT EXISTS unique_k ON t(k)"},
		{query: "CREATE UNIQUE INDEX IF NOT EXISTS ordinary_note ON t(note)"},
		{query: "INSERT INTO t VALUES(50,NULL,'base'),(51,NULL,'base')"},
		{query: "DELETE FROM t WHERE id=0"},
		{query: "INSERT INTO t VALUES(0,1,'reused')"},
		{query: "BEGIN"},
		{query: "UPDATE t SET k=9 WHERE id=0"},
		{query: "ROLLBACK"},
		{query: "INSERT INTO t VALUES(52,1,'conflict after rollback')", constraint: true},
		{query: "SELECT id,k,typeof(k),note FROM t ORDER BY id", read: true},
		{query: "CREATE TABLE built(id INTEGER PRIMARY KEY,k)"},
		{query: "INSERT INTO built VALUES(1,7),(2,7)"},
		{query: "CREATE UNIQUE INDEX unique_built ON built(k)", constraint: true},
		{query: "SELECT count(*) FROM sqlite_schema WHERE name='unique_built'", read: true},
		{query: "BEGIN"},
		{query: "DELETE FROM built WHERE id=2"},
		{query: "SAVEPOINT ddl"},
		{query: "CREATE UNIQUE INDEX unique_built ON built(k)"},
		{query: "INSERT INTO built VALUES(3,7)", constraint: true},
		{query: "ROLLBACK TO ddl"},
		{query: "INSERT INTO built VALUES(3,7)"},
		{query: "RELEASE ddl"},
		{query: "ROLLBACK"},
		{query: "DELETE FROM built WHERE id=2"},
		{query: "CREATE UNIQUE INDEX unique_built ON built(k)"},
		{query: "INSERT INTO built VALUES(3,7)", constraint: true},
		{query: "SELECT id,k FROM built ORDER BY id", read: true},
		{query: "CREATE TABLE composite(id INTEGER PRIMARY KEY,a,b,c)"},
		{query: "CREATE UNIQUE INDEX unique_abc ON composite(a,b,c)"},
		{query: "INSERT INTO composite VALUES(1,1,2,3),(2,1,2,4),(3,1,3,3),(4,2,2,3)"},
		{query: "INSERT INTO composite VALUES(5,1,2,3)", constraint: true},
		{query: "INSERT INTO composite VALUES(10,NULL,2,3),(11,NULL,2,3),(12,1,NULL,3),(13,1,NULL,3),(14,1,2,NULL),(15,1,2,NULL),(16,NULL,NULL,NULL),(17,NULL,NULL,NULL)"},
		{query: "SELECT id,a,b,c FROM composite ORDER BY id", read: true},
	}
	for _, size := range []uint32{512, 4096, 65536} {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("page%d/shared%t", size, shared), func(t *testing.T) {
				uniqueOracleHistory(t, size, shared, steps)
			})
		}
	}
}

func TestOracleUniqueIndexExactNumericAndAffinity(t *testing.T) {
	steps := []uniqueOracleStep{
		{query: "CREATE TABLE exact(id INTEGER PRIMARY KEY,k)"},
		{query: "CREATE UNIQUE INDEX unique_exact ON exact(k)"},
		{query: "INSERT INTO exact VALUES(1,-9223372036854775808),(2,9223372036854775807),(3,9007199254740991),(4,9007199254740992),(5,9007199254740993),(6,9007199254740994.0),(7,9.223372036854776e18),(8,0),(9,'1'),(10,X'31'),(11,CAST(X'6100E7958C' AS TEXT)),(12,X'6100E7958C')"},
		{query: "INSERT INTO exact VALUES(100,-9.223372036854776e18)", constraint: true},
		{query: "INSERT INTO exact VALUES(100,9007199254740992.0)", constraint: true},
		{query: "INSERT INTO exact VALUES(100,9007199254740994)", constraint: true},
		{query: "INSERT INTO exact VALUES(100,-0.0)", constraint: true},
		{query: "INSERT INTO exact VALUES(100,CAST(X'6100E7958C' AS TEXT))", constraint: true},
		{query: "INSERT INTO exact VALUES(100,X'6100E7958C')", constraint: true},
		{query: "SELECT id,typeof(k),hex(k) FROM exact WHERE id>=11 ORDER BY id", read: true},
		{query: "SELECT id,k,typeof(k) FROM exact WHERE id<=10 ORDER BY id", read: true},
		{query: "CREATE TABLE affinities(id INTEGER PRIMARY KEY,n NUMERIC,r REAL,t TEXT,b BLOB)"},
		{query: "CREATE UNIQUE INDEX unique_n ON affinities(n)"},
		{query: "CREATE UNIQUE INDEX unique_r ON affinities(r)"},
		{query: "CREATE UNIQUE INDEX unique_t ON affinities(t)"},
		{query: "CREATE UNIQUE INDEX unique_b ON affinities(b)"},
		{query: "INSERT INTO affinities VALUES(1,'01','01',1,1)"},
		{query: "INSERT INTO affinities(id,n) VALUES(2,1.0)", constraint: true},
		{query: "INSERT INTO affinities(id,r) VALUES(2,'1e0')", constraint: true},
		{query: "INSERT INTO affinities(id,t) VALUES(2,1)", constraint: true},
		{query: "INSERT INTO affinities(id,b) VALUES(2,1.0)", constraint: true},
		{query: "INSERT INTO affinities VALUES(2,X'31',1.25,'01','1'),(3,NULL,NULL,NULL,NULL),(4,NULL,NULL,NULL,NULL)"},
		{query: "SELECT id,n,typeof(n),r,typeof(r),t,typeof(t),b,typeof(b) FROM affinities ORDER BY id", read: true},
	}
	uniqueOracleHistory(t, 512, false, steps)
}

func TestOracleUniqueIndexTwoWayFiles(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	seed := []string{
		"CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT,b BLOB)",
		"INSERT INTO t VALUES(0,'zero',X'00'),(-7,'negative',X'FF'),(1,'same',X'01'),(2,'same',X'02'),(3,NULL,X'00'),(4,NULL,X'00'),(5,'null',NULL),(6,'null',NULL)",
		"CREATE UNIQUE INDEX unique_kb ON t(k COLLATE BINARY ASC,b)",
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
					db, err := sqlite.Open(path, opts)
					if err != nil {
						t.Fatal(err)
					}
					defer db.Close()
					check := func() {
						t.Helper()
						q := "SELECT id,hex(k),typeof(k),hex(b),typeof(b) FROM t ORDER BY id"
						writeSame(t, db, path, q)
						forced := "SELECT id,hex(k),typeof(k),hex(b),typeof(b) FROM t INDEXED BY unique_kb ORDER BY id"
						confCompare(t, confExpected(t, path, forced), confNative(t, db, q))
						if _, err := db.Check(ctx); err != nil {
							t.Fatal(err)
						}
						writeIntegrity(t, path)
					}
					constraint := func(q string) {
						t.Helper()
						_, err := db.Exec(ctx, q)
						var detail *sqlite.Error
						if !errors.Is(err, sqlite.ErrConstraint) || !errors.As(err, &detail) || detail.Code != sqlite.CodeConstraint {
							t.Fatal(q, err)
						}
					}
					check()
					constraint("INSERT INTO t VALUES(100,'zero',X'00')")
					constraint("UPDATE t SET k='zero',b=X'00' WHERE id=-7")
					writeExec(t, db, "UPDATE t SET id=7 WHERE id=0")
					writeExec(t, db, "INSERT INTO t VALUES(8,'native',X'AA')")
					check()
					if err = db.Close(); err != nil {
						t.Fatal(err)
					}
					// Official writes an explicit-index file created by either engine.
					confCLI(t, path, "BEGIN;UPDATE t SET id=0 WHERE id=7;DELETE FROM t WHERE id=-7;INSERT INTO t VALUES(9,'official',X'BB');COMMIT;", false)
					db, err = sqlite.Open(path, opts)
					if err != nil {
						t.Fatal(err)
					}
					defer db.Close()
					constraint("INSERT INTO t VALUES(101,'official',X'BB')")
					writeExec(t, db, "INSERT INTO t VALUES(-7,'negative',X'FF')")
					check()
				})
			}
		}
	}
}
