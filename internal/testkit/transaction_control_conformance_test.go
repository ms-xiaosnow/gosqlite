//go:build oracle

package testkit_test

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Only ordinary intact-file SQL histories and normal oracle EOF are used.
func TestOracleTransactionSavepointHistories(t *testing.T) {
	confOracle(t)
	for _, size := range []uint32{512, 4096, 65536} {
		for _, shared := range []bool{false, true} {
			t.Run(fmt.Sprintf("page%d/shared%t", size, shared), func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "native.db")
				reference := filepath.Join(dir, "oracle.db")
				db, err := sqlite.Open(path, sqlite.Options{PageSize: size, WALShared: shared})
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				oracle := startLiveOracle(t, reference)
				oracle.run(fmt.Sprintf("PRAGMA page_size=%d;", size))
				if shared {
					oracle.run("PRAGMA journal_mode=WAL;")
				}
				execBoth := func(q string) { t.Helper(); writeExec(t, db, q); oracle.run(q + ";") }
				compare := func(q string) {
					t.Helper()
					r := confNative(t, db, q)
					want := oracle.run(q + ";")
					var got []string
					for _, row := range r.rows {
						var values []string
						for _, v := range row {
							values = append(values, fmt.Sprint(v))
						}
						got = append(got, strings.Join(values, "|"))
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("%s native=%v official=%v", q, got, want)
					}
				}
				execBoth("CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
				execBoth("CREATE INDEX t_v ON t(v)")
				execBoth("INSERT INTO t VALUES(1,'base')")
				for _, begin := range []string{"SAVEPOINT root", "BEGIN DEFERRED"} {
					execBoth(begin)
					execBoth("SAVEPOINT inner")
					execBoth("UPDATE t SET v='outer' WHERE id=1")
					execBoth("SAVEPOINT repeated")
					execBoth("CREATE TABLE transient(id INTEGER PRIMARY KEY)")
					execBoth("CREATE INDEX transient_ix ON transient(id)")
					execBoth("INSERT INTO transient VALUES(7)")
					execBoth("SAVEPOINT repeated")
					execBoth("UPDATE t SET v='inner' WHERE id=1")
					compare("SELECT id,v FROM t ORDER BY id")
					execBoth("ROLLBACK TRANSACTION TO SAVEPOINT repeated")
					compare("SELECT id,v FROM t ORDER BY id")
					execBoth("INSERT INTO t VALUES(2,'rolled-back')")
					execBoth("ROLLBACK TO repeated")
					execBoth("RELEASE SAVEPOINT repeated")
					compare("SELECT id FROM transient")
					execBoth("ROLLBACK TO repeated")
					compare("SELECT name FROM sqlite_schema WHERE name='transient'")
					execBoth("RELEASE inner")
					execBoth("SAVEPOINT 'quoted name'")
					execBoth("INSERT INTO t VALUES(3,'kept')")
					execBoth("RELEASE 'quoted name'")
					if begin == "SAVEPOINT root" {
						execBoth("RELEASE root")
					} else {
						execBoth("END TRANSACTION")
					}
					if !db.Autocommit() {
						t.Fatal("transaction remained active")
					}
					compare("SELECT id,v FROM t ORDER BY id")
					execBoth("DELETE FROM t WHERE id=3")
				}
				oracle.close()
				if shared {
					if err = db.Checkpoint(context.Background()); err != nil {
						t.Fatal(err)
					}
				}
				writeSame(t, db, reference, "SELECT id,v FROM t ORDER BY id")
				if err = db.Close(); err != nil {
					t.Fatal(err)
				}
				writeIntegrity(t, path)
			})
		}
	}
}

func TestOracleTransactionDeferredLiveCommitTiming(t *testing.T) {
	confOracle(t)
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "timing.db")
			db, err := sqlite.Open(path, sqlite.Options{WALShared: shared})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			writeExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
			writeExec(t, db, "INSERT INTO t VALUES(1,'old')")
			peer := startLiveOracle(t, path)
			for _, begin := range []string{"BEGIN DEFERRED", "SAVEPOINT root"} {
				writeExec(t, db, begin)
				_ = confNative(t, db, "SELECT 1")
				peer.run("UPDATE t SET v='committed-between' WHERE id=1;")
				got := confNative(t, db, "SELECT v FROM t").rows
				if !reflect.DeepEqual(got, [][]any{{"committed-between"}}) {
					t.Fatal("early snapshot", got)
				}
				writeExec(t, db, "ROLLBACK")
			}

			// End the imported official WAL generation through an ordinary
			// checkpoint before native publication, preserving sector-safety gates.
			if shared {
				if err = db.Checkpoint(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			peer.run("BEGIN DEFERRED; SELECT 1;")
			writeExec(t, db, "UPDATE t SET v='native-between' WHERE id=1")
			if got := peer.run("SELECT v FROM t;"); !reflect.DeepEqual(got, []string{"native-between"}) {
				t.Fatal("official first access differed", got)
			}
			if shared {
				writeExec(t, db, "UPDATE t SET v='native-after-read' WHERE id=1")
				if got := peer.run("SELECT v FROM t;"); !reflect.DeepEqual(got, []string{"native-between"}) {
					t.Fatal("official snapshot changed", got)
				}
			}
			peer.run("ROLLBACK;")
			peer.close()
		})
	}
}

func TestOracleTransactionExclusiveReservations(t *testing.T) {
	ctx := context.Background()
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "exclusive.db")
			db, err := sqlite.Open(path, sqlite.Options{WALShared: shared})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			writeExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY)")
			writeExec(t, db, "INSERT INTO t VALUES(1)")
			peer := startLiveOracle(t, path)
			writeExec(t, db, "BEGIN EXCLUSIVE")
			// Expected BUSY from an ordinary cooperating process, with normal return.
			out, err := exec.Command(confOracle(t), "-batch", "-bail", path, "BEGIN IMMEDIATE; ROLLBACK;").CombinedOutput()
			if err == nil || !strings.Contains(string(out), "locked") {
				t.Fatalf("official writer bypassed reservation: %v %s", err, out)
			}
			out, err = exec.Command(confOracle(t), "-batch", "-bail", path, "SELECT id FROM t;").CombinedOutput()
			if shared {
				if err != nil || strings.TrimSpace(string(out)) != "1" {
					t.Fatal(err, string(out))
				}
			} else if err == nil || !strings.Contains(string(out), "locked") {
				t.Fatalf("official read bypassed EXCLUSIVE: %v %s", err, out)
			}
			writeExec(t, db, "ROLLBACK")
			peer.run("BEGIN EXCLUSIVE;")
			if _, err = db.Exec(ctx, "BEGIN IMMEDIATE"); !errors.Is(err, sqlite.ErrBusy) || !db.Autocommit() {
				t.Fatal("native writer bypassed official reservation", err)
			}
			peer.run("ROLLBACK;")
			peer.close()
		})
	}
}
