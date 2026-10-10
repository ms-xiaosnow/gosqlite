package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestEncryptedSharedExampleOrdinary(t *testing.T) {
	var output bytes.Buffer
	if err := run(&output); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"unpublished writes", "stale promotion fails", "Checkpoint retains old snapshots", "Credentials stayed in memory"} {
		if !strings.Contains(output.String(), want) {
			t.Fatal(output.String())
		}
	}
}
