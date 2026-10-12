//go:build oracle && linux && manual_backup_publication

package sqlite_test

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/buildinfo"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/backupmanual"
	"github.com/ms-xiaosnow/gosqlite/internal/testoracle"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Test-only official CLI; only intact database creation and read-only integrity
// queries. No fault, interrupted process, recovery or exceptional native action.
func plaintextBackupOracle(t *testing.T, scope *backupmanual.Scope, path, sql string, readOnly bool) string {
	t.Helper()
	scope.Before("oracle")
	exe := scope.OracleExecutable()
	args := []string{"-batch", "-bail", "-noheader", "-list"}
	if readOnly {
		args = append(args, "-readonly")
	}
	cmd := testoracle.ForTest(t, exe, append(args, path)...)
	cmd.Stdin = strings.NewReader(sql + "\n")
	output := scope.OracleOutput()
	cmd.Stdout = output
	cmd.Stderr = output
	scope.HoldOracle(cmd, output)
	err := cmd.Run()
	out, outErr := output.Result()
	scope.RetainErrors(err, outErr)
	// Keep both errors before either observation can freeze this scope. The
	// bounded writer continues consuming after its limit; it never kills or
	// interrupts the child and preserves total count and full-stream digest.
	scope.Record("oracle run error", err)
	scope.Record("oracle output error", outErr)
	scope.Observe("oracle output", string(out), outErr)
	scope.Observe("oracle run", map[string]any{"path": path, "arguments": append(args, path), "input": backupmanual.SQL(sql), "output": string(out)}, err)
	if err != nil {
		t.Fatal(err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func backupOraclePlan(size uint32, path, destination, q string) any {
	return []any{
		backupPlanCall("oracle create", path, map[string]any{"arguments": []string{"-noinit", "-batch", "-bail", "-noheader", "-list", path}, "sql": backupmanual.SQL(q)}, "error == nil; strings.TrimSpace(output) == ok"),
		backupPlanCall("ReadFile", path, nil, "error == nil; retain every before byte"),
		backupPlanCall("Open", path, sqlite.Options{ReadOnly: true, NoCreate: true}, "error == nil"),
		backupPlanCall("DB.Backup", destination, map[string]any{"source": path, "page_size": size}, "error == nil"),
		backupPlanCall("CloseDB", path, nil, "error == nil; one explicit end attempt"),
		backupPlanCall("ReadFile", path, nil, "error == nil; bytes.Equal(before, source bytes)"),
		backupPlanCall("ReadFile", destination, nil, "error == nil; bytes.Equal(before, destination bytes)"),
		backupPlanCall("oracle readonly check", destination, map[string]any{"arguments": []string{"-noinit", "-batch", "-bail", "-noheader", "-list", "-readonly", destination}, "sql": backupmanual.SQL("PRAGMA integrity_check; PRAGMA user_version; PRAGMA application_id; SELECT id,body,hex(payload) FROM t ORDER BY id;")}, "error == nil; strings.TrimSpace(output) == ok\n37\n12345\n1|数据库 😀|0001FF\n2|second|"),
		backupPlanCall("Open", destination, sqlite.Options{ReadOnly: true, NoCreate: true}, "error == nil"),
		backupPlanCall("Check", destination, nil, "error == nil"),
		backupPlanCall("CloseDB", destination, nil, "error == nil; one explicit end attempt"),
	}
}

func TestPlaintextBackupOracleOrdinaryInteroperability(t *testing.T) {
	run := backupmanual.ForTest(t)
	identity := run.Case(t, "oracle-identity")
	identity.SetPlan([]any{
		backupPlanCall("oracle identity", ":memory:", map[string]any{"arguments": []string{"-noinit", "-batch", "-bail", "-noheader", "-list", ":memory:"}, "sql": backupmanual.SQL("SELECT sqlite_version(),sqlite_source_id();")}, buildinfo.SQLiteBaseline+"|"+buildinfo.SQLiteSourceID),
	})
	if got := plaintextBackupOracle(t, identity, ":memory:", "SELECT sqlite_version(),sqlite_source_id();", false); got != buildinfo.SQLiteBaseline+"|"+buildinfo.SQLiteSourceID {
		t.Fatal("oracle identity mismatch", got)
	}
	identity.FinishSuccess()
	for _, size := range []uint32{512, 4096, 65536} {
		if !t.Run(fmt.Sprint(size), func(t *testing.T) {
			scope := run.Case(t, fmt.Sprint(size))
			dir := scope.FixtureDir("data")
			path, destination := filepath.Join(dir, "official.db"), filepath.Join(dir, "native-backup.db")
			q := fmt.Sprintf("PRAGMA page_size=%d; PRAGMA user_version=37; PRAGMA application_id=12345; CREATE TABLE t(id INTEGER PRIMARY KEY,body TEXT,payload BLOB); CREATE INDEX by_body ON t(body); INSERT INTO t VALUES(1,'数据库 😀',X'0001ff'),(2,'second',NULL); PRAGMA integrity_check;", size)
			scope.SetPlan(backupOraclePlan(size, path, destination, q))
			if got := plaintextBackupOracle(t, scope, path, q, false); got != "ok" {
				t.Fatal(got)
			}
			before := backupBytes(t, scope, path)
			db := backupOpen(t, scope, path, sqlite.Options{ReadOnly: true, NoCreate: true})
			scope.Before("oracle source backup")
			info, err := db.Backup(context.Background(), destination)
			scope.Observe("oracle source backup", info, err)
			if err != nil {
				t.Fatal(err)
			}
			if err := scope.CloseDB(db); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, backupBytes(t, scope, path)) || !bytes.Equal(before, backupBytes(t, scope, destination)) {
				t.Fatal("source/metadata bytes changed")
			}
			got := plaintextBackupOracle(t, scope, destination, "PRAGMA integrity_check; PRAGMA user_version; PRAGMA application_id; SELECT id,body,hex(payload) FROM t ORDER BY id;", true)
			if got != "ok\n37\n12345\n1|数据库 😀|0001FF\n2|second|" {
				t.Fatal(got)
			}
			copy := backupOpen(t, scope, destination, sqlite.Options{ReadOnly: true, NoCreate: true})
			scope.Before("oracle copy check")
			check, err := copy.Check(context.Background())
			scope.Observe("oracle copy check", check, err)
			if err != nil {
				t.Fatal(err)
			}
			if err := scope.CloseDB(copy); err != nil {
				t.Fatal(err)
			}
			scope.FinishSuccess()
		}) {
			return
		}
	}
	run.FinishSuccess()
}
