//go:build oracle

package testkit_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/testoracle"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

const connectionOracleSelect = "SELECT changes(),total_changes(),last_insert_rowid(),typeof(changes()),typeof(total_changes()),typeof(last_insert_rowid())"

type connectionStep struct {
	query    string
	read     bool
	code     sqlite.Code
	category string
}

// The complete history runs on one official connection and one native
// connection. Normal EOF is used, even after expected logical SQL errors.
// Quote-mode output compares every label, value, and INTEGER/REAL/TEXT class;
// error steps check primary native codes and matching official error categories.
func TestConnectionCountersOracleHistories(t *testing.T) {
	steps := []connectionStep{
		{query: connectionOracleSelect, read: true},
		{query: "CREATE TABLE t(id INTEGER PRIMARY KEY,v NOT NULL,c,tc,li)"},
		{query: "CREATE INDEX ix ON t(v)"},
		{query: "INSERT INTO t VALUES(4,'a',changes(),total_changes(),last_insert_rowid()),(7,'b',changes(),total_changes(),last_insert_rowid())"},
		{query: "SELECT * FROM t ORDER BY id", read: true},
		{query: "SELECT changes()+1,total_changes()*2,CAST(last_insert_rowid() AS TEXT),last_insert_rowid() IN (1,7)", read: true},
		{query: "SELECT count(*),sum(changes()),total_changes(),last_insert_rowid() FROM t", read: true},
		{query: "SELECT t.id,e.id,changes(),total_changes(),last_insert_rowid() FROM t LEFT JOIN t e ON e.id=t.id AND changes()>0 ORDER BY t.id", read: true},
		{query: "SELECT id FROM t WHERE changes()=2 ORDER BY total_changes(),id LIMIT changes() OFFSET 0", read: true},
		{query: "UPDATE t SET v=v,c=changes(),tc=total_changes(),li=last_insert_rowid()"},
		{query: "SELECT * FROM t ORDER BY id", read: true},
		{query: "UPDATE t SET id=id+20 WHERE id=4"},
		{query: "CREATE TABLE extra(x)"},
		{query: "CREATE TABLE IF NOT EXISTS extra(x)"},
		{query: "CREATE INDEX extra_x ON extra(x)"},
		{query: "UPDATE t SET v=v WHERE id=-1"},
		{query: "DELETE FROM t WHERE id=-1"},
		{query: "BEGIN"},
		{query: "INSERT INTO t VALUES(30,'undo',0,0,0)"},
		{query: "SAVEPOINT s"},
		{query: "INSERT INTO t VALUES(40,'undo',0,0,0),(50,'undo',0,0,0)"},
		{query: "ROLLBACK TO s"},
		{query: "RELEASE s"},
		{query: "ROLLBACK"},
		{query: "INSERT INTO t VALUES(60,'first',0,0,0),(70,NULL,0,0,0)", code: sqlite.CodeConstraint, category: "NOT NULL constraint failed"},
		{query: "SELECT * FROM t ORDER BY id", read: true},
		{query: "BEGIN"},
		{query: "INSERT INTO t VALUES(80,'first',0,0,0),(90,NULL,0,0,0)", code: sqlite.CodeConstraint, category: "NOT NULL constraint failed"},
		{query: "SELECT * FROM t ORDER BY id", read: true},
		{query: "ROLLBACK"},
		{query: "UPDATE t SET v=v"},
		{query: "UPDATE t SET missing=1", code: sqlite.CodeError, category: "no such column"},
		{query: "DELETE FROM absent", code: sqlite.CodeError, category: "no such table"},
		{query: "INSERT INTO t(id) VALUES(1,2)", code: sqlite.CodeError, category: "2 values for 1 columns"},
		{query: "UPDATE t SET v=changes(1)", code: sqlite.CodeError, category: "wrong number of arguments"},
		{query: "UPDATE t SET v=abs(-9223372036854775808)", code: sqlite.CodeError, category: "integer overflow"},
		{query: "INSERT INTO t VALUES(7,'dupe',0,0,0)", code: sqlite.CodeConstraint, category: "UNIQUE constraint failed"},
		{query: "INSERT INTO t VALUES(0,'zero',0,0,0),(-7,'negative',0,0,0)"},
		{query: "DELETE FROM t WHERE id=-7"},
		{query: "SAVEPOINT outermost"},
		{query: "INSERT INTO t VALUES(99,'release',0,0,0)"},
		{query: "RELEASE outermost"},
		{query: "DELETE FROM t"},
		{query: "UPDATE t SET v=1"},
		{query: "INSERT INTO t(id,v) VALUES(NULL,'generated')"},
		{query: "SELECT * FROM t ORDER BY id", read: true},
	}
	for _, size := range []uint32{512, 4096, 65536} {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("page%d/shared%t", size, shared), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "native.db")
				db, err := sqlite.Open(path, sqlite.Options{PageSize: size, WALShared: shared})
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				var script strings.Builder
				fmt.Fprintf(&script, "PRAGMA page_size=%d;\n", size)
				// Suppress journal_mode's setup row, then compare only task statements.
				if shared {
					script.WriteString(".output /dev/null\nPRAGMA journal_mode=WAL;\n.output stdout\n")
				}
				var got [][]any
				appendQuery := func(q string) {
					r := confNative(t, db, q)
					if len(r.rows) > 0 {
						var labels []any
						for _, label := range r.columns {
							labels = append(labels, label)
						}
						got = append(got, labels)
						got = append(got, r.rows...)
					}
					script.WriteString(q + ";\n")
				}
				var categories []string
				for _, step := range steps {
					if step.read {
						appendQuery(step.query)
						continue
					}
					result, err := db.Exec(context.Background(), step.query)
					if step.code == 0 {
						if err != nil {
							t.Fatal(step.query, err)
						}
					} else {
						var detail *sqlite.Error
						if !errors.As(err, &detail) || detail.Code != step.code {
							t.Fatalf("%s want code %v: %v", step.query, step.code, err)
						}
						if n, _ := result.RowsAffected(); n != 0 {
							t.Fatal("failed result rows", n)
						}
						if n, _ := result.LastInsertId(); n != 0 {
							t.Fatal("failed result id", n)
						}
						categories = append(categories, step.category)
					}
					script.WriteString(step.query + ";\n")
					appendQuery(connectionOracleSelect)
				}
				cmd := testoracle.ForTest(t, confOracle(t), "-batch", "-quote", "-header", filepath.Join(t.TempDir(), "official.db"))
				cmd.Stdin = strings.NewReader(script.String())
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				out, oracleErr := cmd.Output()
				if oracleErr == nil {
					t.Fatal("official history should report expected logical errors")
				}
				matches := regexp.MustCompile(`(?m)^(?:Parse error|Error) near line \d+: (.*)$`).FindAllStringSubmatch(stderr.String(), -1)
				if len(matches) != len(categories) {
					t.Fatalf("official error count %d want %d: %s", len(matches), len(categories), stderr.String())
				}
				for i, want := range categories {
					if !strings.Contains(matches[i][1], want) {
						t.Fatalf("official error %d want %q: %s", i, want, matches[i][1])
					}
				}
				want, err := confQuoted(out)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					for i := 0; i < len(got) && i < len(want); i++ {
						if !reflect.DeepEqual(got[i], want[i]) {
							t.Fatalf("complete history row %d native=%#v official=%#v", i, got[i], want[i])
						}
					}
					t.Fatalf("history rows native=%d official=%d", len(got), len(want))
				}
				if err = db.Close(); err != nil {
					t.Fatal(err)
				}
				writeIntegrity(t, path)
			})
		}
	}
}

func TestConnectionCountersOracleFunctionErrors(t *testing.T) {
	for _, q := range []string{"SELECT changes(1)", "SELECT total_changes(NULL)", "SELECT last_insert_rowid(1,2)", "SELECT changes(1) LIMIT 0", "SELECT CASE WHEN 0 THEN total_changes(1) ELSE 1 END"} {
		t.Run(q, func(t *testing.T) {
			db, err := sqlite.Open(filepath.Join(t.TempDir(), "native.db"), sqlite.Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			out, oracleErr := testoracle.ForTest(t, confOracle(t), "-batch", "-bail", ":memory:", q).CombinedOutput()
			if oracleErr == nil || !strings.Contains(string(out), "wrong number of arguments") {
				t.Fatal(string(out), oracleErr)
			}
			rows, err := db.Query(context.Background(), q)
			if err == nil {
				rows.Close()
				t.Fatal("accepted invalid counter arity")
			}
			var detail *sqlite.Error
			if !errors.As(err, &detail) || detail.Code != sqlite.CodeError || !strings.Contains(err.Error(), "wrong number of arguments") {
				t.Fatal(err)
			}
		})
	}
}

// Merging the child's stdout and stderr before reading provides deterministic
// error/marker ordering without polling, goroutines or termination hooks.
type counterLiveOracle struct {
	t               *testing.T
	cmd             *exec.Cmd
	in              io.WriteCloser
	out             *bufio.Reader
	errored, closed bool
}

func startCounterLiveOracle(t *testing.T, path string) *counterLiveOracle {
	t.Helper()
	p := &counterLiveOracle{t: t, cmd: testoracle.ForTest(t, confOracle(t), "-batch", "-header", "-quote", path)}
	var err error
	p.in, err = p.cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	p.cmd.Stderr = p.cmd.Stdout
	p.out = bufio.NewReader(out)
	if err = p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if p.closed {
			return
		}
		p.closed = true
		_, writeErr := io.WriteString(p.in, ".quit\n")
		closeErr := p.in.Close()
		waitErr := p.cmd.Wait()
		if writeErr != nil || closeErr != nil || (waitErr != nil) != p.errored {
			t.Errorf("normal oracle completion: write=%v close=%v wait=%v expected logical error=%v", writeErr, closeErr, waitErr, p.errored)
		}
	})
	// Finish shell startup before another process requests EXCLUSIVE. The
	// dot-command handshake does not prepare SQL or initialize schema metadata.
	if _, err := io.WriteString(p.in, ".print __counter_ready__\n"); err != nil {
		t.Fatal(err)
	}
	line, err := p.out.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "__counter_ready__" {
		t.Fatal("oracle startup handshake", line, err)
	}
	return p
}
func (p *counterLiveOracle) run(q string, wantBusy bool) []byte {
	p.t.Helper()
	if _, err := io.WriteString(p.in, q+";\n.print __counter_done__\n"); err != nil {
		p.t.Fatal(err)
	}
	var out bytes.Buffer
	for {
		line, err := p.out.ReadString('\n')
		if err != nil {
			p.t.Fatal("unexpected oracle EOF", err)
		}
		if strings.TrimSpace(line) == "__counter_done__" {
			break
		}
		out.WriteString(line)
	}
	if wantBusy {
		if !strings.Contains(out.String(), "database is locked") {
			p.t.Fatal("wanted official BUSY", out.String())
		}
		p.errored = true
		return nil
	}
	if strings.Contains(out.String(), "Error") || strings.Contains(out.String(), "Parse error") {
		p.t.Fatal("unexpected official error", out.String())
	}
	return out.Bytes()
}
func TestConnectionCountersOracleBusyOutcomes(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "native.db")
	a, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	reference := filepath.Join(t.TempDir(), "official.db")
	x, y := startCounterLiveOracle(t, reference), startCounterLiveOracle(t, reference)
	execBoth := func(db *sqlite.DB, p *counterLiveOracle, q string) {
		t.Helper()
		if _, err := db.Exec(ctx, q); err != nil {
			t.Fatal(q, err)
		}
		p.run(q, false)
	}
	busyBoth := func(db *sqlite.DB, p *counterLiveOracle, q string) {
		t.Helper()
		if _, err := db.Exec(ctx, q); !errors.Is(err, sqlite.ErrBusy) {
			t.Fatal("native want BUSY", q, err)
		}
		p.run(q, true)
	}
	compare := func(q string) {
		t.Helper()
		native := confNative(t, a, q)
		raw := x.run(q, false)
		official, err := confQuoted(raw)
		if err != nil {
			t.Fatal(err)
		}
		var got [][]any
		var labels []any
		for _, v := range native.columns {
			labels = append(labels, v)
		}
		if len(native.rows) > 0 {
			got = append(got, labels)
			got = append(got, native.rows...)
		}
		if !reflect.DeepEqual(got, official) {
			t.Fatalf("%s native=%#v official=%#v", q, got, official)
		}
	}
	execBoth(a, x, "CREATE TABLE t(id INTEGER PRIMARY KEY)")
	execBoth(a, x, "INSERT INTO t VALUES(1),(2)")
	execBoth(b, y, "BEGIN IMMEDIATE")
	busyBoth(a, x, "UPDATE t SET id=id")
	compare(connectionOracleSelect)
	execBoth(b, y, "ROLLBACK")
	execBoth(a, x, "UPDATE t SET id=id")
	execBoth(b, y, "BEGIN")
	peerRows, err := b.Query(ctx, "SELECT * FROM t")
	if err != nil {
		t.Fatal(err)
	}
	if !peerRows.Next() {
		t.Fatal(peerRows.Err())
	}
	y.run("SELECT * FROM t", false)
	busyBoth(a, x, "INSERT INTO t VALUES(3)")
	compare(connectionOracleSelect)
	if err = peerRows.Close(); err != nil {
		t.Fatal(err)
	}
	execBoth(b, y, "ROLLBACK")
	compare("SELECT * FROM t")
	execBoth(a, x, "UPDATE t SET id=id")
	execBoth(b, y, "BEGIN EXCLUSIVE")
	busyBoth(a, x, "DELETE FROM t")
	execBoth(b, y, "ROLLBACK")
	// The ordinary cached schema now reaches the same executed-BUSY boundary.
	compare(connectionOracleSelect)
	execBoth(a, x, "UPDATE t SET id=id")
	compare(connectionOracleSelect)
	execBoth(a, x, "BEGIN")
	execBoth(a, x, "INSERT INTO t VALUES(4)")
	execBoth(b, y, "BEGIN")
	peerRows, err = b.Query(ctx, "SELECT * FROM t")
	if err != nil {
		t.Fatal(err)
	}
	if !peerRows.Next() {
		t.Fatal(peerRows.Err())
	}
	y.run("SELECT * FROM t", false)
	busyBoth(a, x, "COMMIT")
	compare(connectionOracleSelect)
	if err = peerRows.Close(); err != nil {
		t.Fatal(err)
	}
	execBoth(b, y, "ROLLBACK")
	execBoth(a, x, "ROLLBACK")
	compare(connectionOracleSelect)
	compare("SELECT * FROM t")
}
