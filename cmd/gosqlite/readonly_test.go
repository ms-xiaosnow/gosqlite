package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestQueryReadonlyFixture(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("M1 native read locks are Linux-only; this is not cross-platform acceptance")
	}
	p := filepath.Join("..", "..", "testdata", "readonly", "page4096.db")
	var b bytes.Buffer
	if err := run([]string{"query", p, "SELECT id,label FROM samples WHERE id=29"}, &b); err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(&b)
	var columns struct {
		Columns []string `json:"columns"`
	}
	if err := dec.Decode(&columns); err != nil {
		t.Fatal(err)
	}
	if len(columns.Columns) != 2 || columns.Columns[0] != "id" || columns.Columns[1] != "label" {
		t.Fatal(columns)
	}
	var row struct {
		Row []any `json:"row"`
	}
	if err := dec.Decode(&row); err != nil {
		t.Fatal(err)
	}
	if len(row.Row) != 2 || row.Row[0] != float64(29) || row.Row[1] != "数据库🙂é한글-29" {
		t.Fatal(row)
	}
}
func TestInspectReadonlyFixture(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("M1 native read locks are Linux-only")
	}
	var b bytes.Buffer
	if err := run([]string{"inspect", filepath.Join("..", "..", "testdata", "readonly", "page512.db")}, &b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "SQLite format 3") || !strings.Contains(b.String(), "samples") {
		t.Fatal(b.String())
	}
}
func TestQueryCanceled(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("M1 native read locks are Linux-only")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var b bytes.Buffer
	err := runContext(ctx, []string{"query", filepath.Join("..", "..", "testdata", "readonly", "page4096.db"), "SELECT id FROM samples"}, &b)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
