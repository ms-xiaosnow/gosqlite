package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Ordinary read-only synthetic JSON fixture and in-memory output comparison.
func TestFullTableExampleGolden(t *testing.T) {
	var out bytes.Buffer
	if err := run(".", &out); err != nil {
		t.Fatal(err)
	}
	var b struct {
		Artifacts []struct{ Name, Content string }
	}
	if err := json.Unmarshal(out.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if len(b.Artifacts) != 14 {
		t.Fatal(len(b.Artifacts))
	}
	for _, a := range b.Artifacts {
		want, err := os.ReadFile(filepath.Join("golden", a.Name))
		if err != nil {
			t.Fatal(err)
		}
		if a.Content != string(want) {
			t.Error("golden mismatch", a.Name)
		}
	}
}
