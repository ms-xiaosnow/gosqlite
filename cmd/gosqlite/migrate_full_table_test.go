package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This is only normal JSON CLI output and exclusive creation of a review text
// artifact. No native DB, SQL execution, fault or namespace workflow is invoked.
func TestMigrateFullTableReviewOnly(t *testing.T) {
	root := "../../examples/migration-full-table"
	args := []string{"pgembedding", "transition", "--catalog", filepath.Join(root, "catalog.json"), "--options", filepath.Join(root, "options.json"), "--full-table-evidence", filepath.Join(root, "evidence.json")}
	var out bytes.Buffer
	if err := runMigrate(context.Background(), args, &out); err != nil {
		t.Fatal(err)
	}
	var bundle struct {
		Artifacts []struct{ Name, Kind, Content string }
	}
	if err := json.Unmarshal(out.Bytes(), &bundle); err != nil {
		t.Fatal(err)
	}
	if len(bundle.Artifacts) != 14 {
		t.Fatal(len(bundle.Artifacts))
	}
	found := false
	for _, a := range bundle.Artifacts {
		gold, err := os.ReadFile(filepath.Join(root, "golden", a.Name))
		if err != nil {
			t.Fatal(err)
		}
		if string(gold) != a.Content {
			t.Error("golden mismatch", a.Name)
		}
		if a.Name == "40-swap-and-rebind.sql" {
			found = strings.Contains(a.Content, "RENAME TO")
		}
		if a.Kind == "review_only_sql" {
			for _, line := range strings.Split(a.Content, "\n") {
				if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "--") {
					t.Fatal("active SQL")
				}
			}
		}
	}
	if !found {
		t.Fatal("missing table swap")
	}
	path := filepath.Join(t.TempDir(), "review.json")
	var status bytes.Buffer
	if err := runMigrate(context.Background(), append(append([]string{}, args...), "--report", path), &status); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(written, out.Bytes()) {
		t.Fatal("report bytes", err)
	}
	for _, extra := range [][]string{{"--apply"}, {"--resume"}, {"--full-table-evidence", filepath.Join(root, "evidence.json")}, {"unexpected"}} {
		if err := runMigrate(context.Background(), append(append([]string{}, args...), extra...), &status); err == nil {
			t.Fatal("unsafe/duplicate option accepted", extra)
		}
	}
	// Both formats are explicit: neither input schema is silently auto-adopted.
	if err := runMigrate(context.Background(), args[:len(args)-2], &status); err == nil {
		t.Fatal("full options accepted without evidence")
	}
	old := []string{"pgembedding", "transition", "--catalog", filepath.Join(root, "catalog.json"), "--options", "../../examples/migration-transition/options.json", "--full-table-evidence", filepath.Join(root, "evidence.json")}
	if err := runMigrate(context.Background(), old, &status); err == nil {
		t.Fatal("old options adopted as whole-table choices")
	}
}
