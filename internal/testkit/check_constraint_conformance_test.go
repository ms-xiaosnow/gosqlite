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

type checkOracleStep struct {
	query      string
	read       bool
	constraint bool
}

// One normal official connection continues after expected logical constraints;
// there is no -bail, cancellation/termination hook, or physical fault injection.
// ForTest provides the existing isolated startup and confOracle verifies the
// pinned official version/source identity before either history is compared.
func checkOracleHistory(t *testing.T, size uint32, steps []checkOracleStep) {
	t.Helper()
	opts := sqlite.Options{PageSize: size}
	path := filepath.Join(t.TempDir(), "native.db")
	db, err := sqlite.Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var script strings.Builder
	fmt.Fprintf(&script, ".bail off\nPRAGMA page_size=%d;\n", size)
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
			if !errors.Is(err, sqlite.ErrConstraint) || !errors.As(err, &detail) || detail.Code != sqlite.CodeConstraint || !strings.Contains(err.Error(), "CHECK constraint failed:") {
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
	// shell's Error spelling as well, but reject every non-CHECK diagnostic.
	errorLine := regexp.MustCompile(`^(?:Runtime error|Error) near line \d+: (.*)$`)
	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	if len(lines) != constraints {
		t.Fatalf("official constraint count %d want %d: %s", len(lines), constraints, stderr.String())
	}
	for _, line := range lines {
		match := errorLine.FindStringSubmatch(line)
		if len(match) != 2 || !strings.Contains(match[1], "CHECK constraint failed") {
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

func TestOracleCheckConstraintHistories(t *testing.T) {
	steps := []checkOracleStep{
		{query: "CREATE TABLE t(id INTEGER PRIMARY KEY CHECK(id>=-9),a NUMERIC NOT NULL DEFAULT '02' CONSTRAINT positive CHECK(a>0),b NUMERIC DEFAULT 10,note TEXT UNIQUE,CHECK(a<=b))"},
		{query: "CREATE INDEX by_a ON t(a)"},
		{query: "SELECT type,name,tbl_name,sql IS NULL,typeof(sql) FROM sqlite_schema ORDER BY name", read: true},
		{query: "INSERT INTO t VALUES(-9,'01',2,'base')"},
		{query: "INSERT INTO t DEFAULT VALUES"},
		{query: "BEGIN"},
		{query: "INSERT INTO t VALUES(5,5,9,'kept')"},
		{query: "SAVEPOINT s"},
		{query: "INSERT INTO t VALUES(6,6,9,'partial'),(7,0,9,'bad')", constraint: true},
		{query: "UPDATE t SET a=CASE WHEN id=-9 THEN 2 ELSE 0 END WHERE id<0", constraint: true},
		{query: "SELECT id,a,b,note FROM t ORDER BY id", read: true},
		{query: "UPDATE t SET id=-10 WHERE id=5", constraint: true},
		{query: "UPDATE t SET a=a+1,b=b+1"},
		{query: "ROLLBACK TO s"},
		{query: "RELEASE s"},
		{query: "UPDATE t SET a=a,b=b"},
		{query: "COMMIT"},
		{query: "SELECT id,a,b,note FROM t ORDER BY id", read: true},
		{query: "CREATE TABLE truth(id INTEGER PRIMARY KEY,v CHECK(v))"},
		{query: "INSERT INTO truth VALUES(1,NULL),(2,-0.25),(3,'1tail'),(4,'-0.1tail'),(5,X'31'),(6,0x8000000000000000),(7,9007199254740993),(8,'1e-2tail')"},
		{query: "INSERT INTO truth VALUES(99,0)", constraint: true},
		{query: "INSERT INTO truth VALUES(99,-0.0)", constraint: true},
		{query: "INSERT INTO truth VALUES(99,'word')", constraint: true},
		{query: "INSERT INTO truth VALUES(99,'0x1')", constraint: true},
		{query: "INSERT INTO truth VALUES(99,'0_1')", constraint: true},
		{query: "INSERT INTO truth VALUES(99,X'00')", constraint: true},
		{query: "SELECT id,typeof(v),hex(v) FROM truth ORDER BY id", read: true},
		{query: "CREATE TABLE branches(a CHECK(NULL OR ('x' LIKE 'x' ESCAPE 'xx')),CHECK(NOT(NULL AND ('x' LIKE 'x' ESCAPE 'xx'))))"},
		{query: "INSERT INTO branches VALUES(1)"},
		{query: "CREATE TABLE empty(a CHECK(0))"},
		{query: "INSERT INTO empty VALUES(1)", constraint: true},
		{query: "UPDATE empty SET a=1"},
		{query: "DELETE FROM empty"},
		{query: `CREATE TABLE shadow(rowid TEXT,"true","false",CHECK(rowid='r'),CHECK(true>false),CHECK(oid>0))`},
		{query: "INSERT INTO shadow VALUES('r',2,1)"},
		{query: "UPDATE shadow SET _rowid_=-1", constraint: true},
		{query: "UPDATE shadow SET true=false", constraint: true},
		{query: "SELECT rowid,true,false,oid FROM shadow", read: true},
	}
	for _, size := range []uint32{512, 4096} {
		t.Run(fmt.Sprint(size), func(t *testing.T) { checkOracleHistory(t, size, steps) })
	}
}

func TestOracleCheckBidirectionalHealthyCatalogs(t *testing.T) {
	ctx := context.Background()
	definition := "CREATE TABLE t(id INTEGER PRIMARY KEY,a NUMERIC CONSTRAINT positive CHECK(t.a>0),b NUMERIC,note TEXT UNIQUE,CHECK(a<=b))"
	initial := []string{definition, "CREATE INDEX by_a ON t(a)", "INSERT INTO t VALUES(1,'01',3,'one'),(2,2,3,'two')"}
	for _, official := range []bool{false, true} {
		t.Run(fmt.Sprint(official), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "healthy.db")
			if official {
				confCLI(t, path, "PRAGMA page_size=512;"+strings.Join(initial, ";")+";", false)
			} else {
				native := writeOpen(t, path, 512)
				for _, query := range initial {
					writeExec(t, native, query)
				}
				if err := native.Close(); err != nil {
					t.Fatal(err)
				}
			}
			db := writeOpen(t, path, 512)
			if got := confNative(t, db, "SELECT sql FROM sqlite_schema WHERE name='t'"); !reflect.DeepEqual(got.rows, [][]any{{definition}}) {
				t.Fatal("CHECK SQL was reconstructed", got)
			}
			if schema, err := db.Schema(ctx); err != nil || len(schema) != 3 {
				t.Fatal("CHECK invented an extra schema record", schema, err)
			}
			if _, err := db.Check(ctx); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			// Only ordinary official schema edits. Go admits the resulting
			// CREATE text; this does not add native ALTER syntax support.
			confCLI(t, path, "ALTER TABLE t RENAME TO renamed; ALTER TABLE renamed RENAME COLUMN a TO amount; ALTER TABLE renamed ADD COLUMN extra TEXT DEFAULT 'tail'; INSERT INTO renamed(id,amount,b,note) VALUES(3,3,4,'official');", false)
			db = writeOpen(t, path, 512)
			writeExec(t, db, "INSERT INTO renamed(id,amount,b,note) VALUES(4,4,5,'native')")
			_, err := db.Exec(ctx, "INSERT INTO renamed(id,amount,b,note) VALUES(5,0,5,'bad')")
			if !errors.Is(err, sqlite.ErrConstraint) || !strings.Contains(err.Error(), "positive") {
				t.Fatal("imported CHECK was not enforced", err)
			}
			writeSame(t, db, path, "SELECT id,amount,b,note,extra FROM renamed ORDER BY id")
			if got, err := db.Check(ctx); err != nil || got.TableRows != 4 || got.Indexes != 2 || got.IndexEntries != 8 {
				t.Fatal(got, err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			writeIntegrity(t, path)
		})
	}
}

func TestOracleCheckUnsupportedFunctionPositiveControls(t *testing.T) {
	// Official SQLite supports these ordinary CHECK functions. The Go slice
	// intentionally refuses them; negative coverage must not imply SQLite does.
	path := filepath.Join(t.TempDir(), "official-functions.db")
	confCLI(t, path, "CREATE TABLE abs_check(a CHECK(abs(a)>0)); INSERT INTO abs_check VALUES(-1); CREATE TABLE length_check(a CHECK(length(a)>0)); INSERT INTO length_check VALUES('text');", false)
	writeIntegrity(t, path)
	db := writeOpen(t, filepath.Join(t.TempDir(), "native.db"), 512)
	for _, query := range []string{"CREATE TABLE abs_check(a CHECK(abs(a)>0))", "CREATE TABLE length_check(a CHECK(length(a)>0))"} {
		if _, err := db.Exec(context.Background(), query); !errors.Is(err, sqlite.ErrUnsupported) {
			t.Fatal("partial CHECK profile changed", query, err)
		}
	}
	if schema, err := db.Schema(context.Background()); err != nil || len(schema) != 0 {
		t.Fatal("rejected CHECK published schema", schema, err)
	}
}
