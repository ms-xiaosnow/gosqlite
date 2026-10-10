package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateInspectAndDryRun(t *testing.T) {
	catalog := "../../testdata/pgembedding-0.3.6/catalog.json"
	for _, args := range [][]string{{"migrate", "pgembedding", "inspect", "--catalog", catalog}, {"migrate", "pgembedding", "--catalog", catalog, "--dry-run"}} {
		var b bytes.Buffer
		if e := runContext(context.Background(), args, &b); e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(b.String(), `"legacy_vector"`) {
			t.Fatal(b.String())
		}
	}
	dir := t.TempDir()
	db := filepath.Join(dir, "must-not-exist.db")
	var b bytes.Buffer
	if e := run([]string{"migrate", "pgembedding", "--catalog", catalog, "--output", db, "--dry-run"}, &b); e == nil {
		t.Fatal("unsupported output accepted")
	}
	if _, e := os.Stat(db); !os.IsNotExist(e) {
		t.Fatal("dry-run touched DB")
	}
	for _, extra := range [][]string{{"--apply"}, {"--dry-run=false"}, {"--catalog", catalog}, {"--pg-dsn-env", "SECRET"}, {"--format", "xml"}} {
		args := append([]string{"migrate", "pgembedding", "--catalog", catalog}, extra...)
		if e := run(args, &b); e == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
func TestMigrateReportDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")
	args := []string{"migrate", "pgembedding", "inspect", "--catalog", "../../testdata/pgembedding-0.3.6/catalog.json", "--format", "markdown", "--report", path}
	var b bytes.Buffer
	if e := run(args, &b); e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = run(args, &b); e == nil {
		t.Fatal("overwrote existing report")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("changed existing report")
	}
}
