package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/migration/postgres"
)

func converterArgs() []string {
	return []string{"migrate", "pgembedding", "convert", "--catalog", "../../examples/migrate-pgembedding/converter-catalog.json", "--input", "../../examples/migrate-pgembedding/source.sql", "--target", "local", "--table", "30000=documents"}
}
func TestMigrateConverterCLI(t *testing.T) {
	var out bytes.Buffer
	if err := run(converterArgs(), &out); err != nil {
		t.Fatal(err)
	}
	var r postgres.Conversion
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Status != "converted" || r.Validation != "local_parser_checked" || len(r.Statements) != 5 || r.TargetSHA256 == "" {
		t.Fatal(r)
	}
	out.Reset()
	args := append(converterArgs(), "--format", "sql")
	if err := run(args, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "<+>") || !strings.Contains(out.String(), "IS NULL ASC") {
		t.Fatal(out.String())
	}
}
func TestMigrateConverterUnsupportedNoPartialSQL(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.sql")
	if err := os.WriteFile(source, []byte("SELECT id FROM public.documents; DROP TABLE public.documents;"), 0600); err != nil {
		t.Fatal(err)
	}
	args := converterArgs()
	for i, s := range args {
		if s == "--input" {
			args[i+1] = source
		}
	}
	var out bytes.Buffer
	if err := run(append(args, "--format", "sql"), &out); err == nil || out.Len() != 0 {
		t.Fatal("partial SQL escaped", err, out.String())
	}
	out.Reset()
	if err := run(args, &out); err == nil || !strings.Contains(out.String(), `"status": "unsupported"`) || !strings.Contains(out.String(), `"original_sql"`) {
		t.Fatal("missing unsupported report", err, out.String())
	}
}
func TestMigrateConverterArtifactRefusesOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.json")
	args := append(converterArgs(), "--report", path)
	var out bytes.Buffer
	if err := run(args, &out); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = run(args, &out); err == nil {
		t.Fatal("existing report overwritten")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("existing bytes changed")
	}
}
func TestMigrateConverterRejectsFlagsAndCanceledContext(t *testing.T) {
	for _, extra := range [][]string{{"--apply"}, {"--resume"}, {"--output", "db"}, {"--pg-dsn-env", "SECRET"}, {"--target", "postgres"}, {"--standard-conforming-strings=false"}, {"--format", "other"}} {
		var out bytes.Buffer
		if err := run(append(converterArgs(), extra...), &out); err == nil {
			t.Fatal("invalid option accepted", extra)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	if err := runContext(ctx, converterArgs(), &out); err != context.Canceled {
		t.Fatal(err)
	}
}
func TestMigrateConverterPostgresReviewCLI(t *testing.T) {
	args := converterArgs()
	for i, s := range args {
		if s == "--target" {
			args[i+1] = "postgres"
		}
	}
	args = append(args, "--target-schema", "public", "--vector-schema", "vectors", "--format", "sql")
	var out bytes.Buffer
	if err := run(args, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "-- REVIEW ONLY:") || !strings.Contains(out.String(), `OPERATOR("vectors".<+>)`) || strings.Contains(out.String(), "BEGIN;") {
		t.Fatal(out.String())
	}
}
