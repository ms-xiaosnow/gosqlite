//go:build linux && manual_backup_publication

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/testkit/backupmanual"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestCLIPlainBackupOrdinaryRoundTrip(t *testing.T) {
	manualRun := backupmanual.ForTest(t)
	ctx := context.Background()
	scope := manualRun.Case(t, "cli-ordinary-round-trip")
	dir := scope.Dir("data")
	source, destination := filepath.Join(dir, "source.db"), filepath.Join(dir, "copy.db")
	scope.SetPlan([]any{
		map[string]any{"operation": "Open", "path": source, "options": sqlite.Options{}, "expected": "error == nil"},
		map[string]any{"operation": "Exec", "path": source, "input": backupmanual.SQL("CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT,payload BLOB)"), "expected": "error == nil"},
		map[string]any{"operation": "Exec", "path": source, "input": backupmanual.SQL("CREATE INDEX docs_body ON docs(body)"), "expected": "error == nil"},
		map[string]any{"operation": "Exec", "path": source, "input": backupmanual.SQL("INSERT INTO docs VALUES(1,'数据库 😀',X'0001ff')"), "expected": "error == nil"},
		map[string]any{"operation": "CloseDB", "path": source, "expected": "error == nil; one explicit end attempt"},
		map[string]any{"operation": "runMaintenanceWithInput", "arguments": []string{"backup", source, "--out", destination}, "input": "lookupEnv and stdin must never be called", "expected": "error == nil"},
		map[string]any{"operation": "json.Unmarshal", "expected": "error == nil; Status == ok; Format == sqlite3; Mode == rollback; PageCount != 0; RetainsHistory == false"},
		map[string]any{"operation": "buffer.Reset; run", "arguments": []string{"check", destination}, "expected": "error == nil; bytes.Contains(stdout, \"status\":\"ok\")"},
		map[string]any{"operation": "buffer.Reset; run", "arguments": []string{"query", destination, "SELECT id,body,payload FROM docs"}, "input": backupmanual.SQL("SELECT id,body,payload FROM docs"), "expected": "error == nil; bytes.Contains(stdout, 数据库 😀)"},
	})
	scope.Before("cli fixture open")
	db, err := sqlite.Open(source, sqlite.Options{})
	scope.RetainErrors(err)
	if db != nil {
		scope.OwnDB(db)
	}
	scope.Observe("cli fixture open", db != nil, err)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT,payload BLOB)", "CREATE INDEX docs_body ON docs(body)", "INSERT INTO docs VALUES(1,'数据库 😀',X'0001ff')"} {
		scope.Before("cli fixture exec")
		execResult, execErr := db.Exec(ctx, q)
		err = execErr
		scope.Observe("cli fixture exec", map[string]any{"input": backupmanual.SQL(q), "result": fmt.Sprintf("%#v", execResult)}, err)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = scope.CloseDB(db); err != nil {
		t.Fatal(err)
	}
	input := credentialInput{lookupEnv: func(string) (string, bool) { t.Fatal("plaintext backup read credentials"); return "", false }, stdin: func() (io.Reader, error) { t.Fatal("plaintext backup read stdin"); return nil, nil }}
	var out bytes.Buffer
	scope.Before("cli backup")
	err = runMaintenanceWithInput(ctx, "backup", []string{source, "--out", destination}, &out, input)
	scope.Observe("cli backup", out.String(), err)
	if err != nil {
		t.Fatal(err)
	}
	var result maintenanceResult
	err = json.Unmarshal(out.Bytes(), &result)
	scope.Observe("cli backup json", result, err)
	if err != nil || result.Status != "ok" || result.Format != "sqlite3" || result.Mode != "rollback" || result.PageCount == 0 || result.RetainsHistory {
		t.Fatal(result, err)
	}
	out.Reset()
	scope.Before("cli check")
	err = run([]string{"check", destination}, &out)
	scope.Observe("cli check", out.String(), err)
	if err != nil || !bytes.Contains(out.Bytes(), []byte(`"status":"ok"`)) {
		t.Fatal(out.String(), err)
	}
	out.Reset()
	scope.Before("cli query")
	err = run([]string{"query", destination, "SELECT id,body,payload FROM docs"}, &out)
	scope.Observe("cli query", out.String(), err)
	if err != nil || !bytes.Contains(out.Bytes(), []byte("数据库 😀")) {
		t.Fatal(out.String(), err)
	}
	scope.FinishSuccess()
	manualRun.FinishSuccess()
}
