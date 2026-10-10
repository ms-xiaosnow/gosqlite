package main

import (
	"bytes"
	"os"
	"runtime"
	"testing"
)

// This exercises only intact files, correct credentials and normal close/reopen.
func TestEncryptedWALExample(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("experimental encrypted storage requires Linux")
	}
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	var out bytes.Buffer
	if err := run(&out); err != nil {
		t.Fatal(err)
	}
	want := "Created, changed, queried, and authenticated an encrypted WAL head (3 segments).\n" +
		"Normally closed and reopened with the same key; the committed segments remain selected.\n" +
		"Checkpointed and reopened read-only: zero selected segments, same data, retained ciphertext history.\n" +
		"The key stayed in memory; disposable files will now be removed (not securely erased).\n"
	if out.String() != want {
		t.Fatalf("unexpected output: %q", out.String())
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("disposable example directory remains after normal completion")
	}
}
