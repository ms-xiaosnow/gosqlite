package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var b bytes.Buffer
	if err := run([]string{"version"}, &b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "SQLite behavior baseline 3.54.0") {
		t.Fatal(b.String())
	}
}
func TestUnsupported(t *testing.T) {
	var b bytes.Buffer
	if run([]string{"query"}, &b) == nil {
		t.Fatal("unimplemented command accepted")
	}
}
