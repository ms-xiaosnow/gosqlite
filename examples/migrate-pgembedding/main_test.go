package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestConverterExample(t *testing.T) {
	var out bytes.Buffer
	if err := runExample(&out); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"id=3 distance=0", "id=1 distance=5", "id=2 distance=6", "id=4 distance=<nil>", "normal reopen", "EFSearch=64"} {
		if !strings.Contains(out.String(), line) {
			t.Fatalf("missing %q: %s", line, out.String())
		}
	}
}
