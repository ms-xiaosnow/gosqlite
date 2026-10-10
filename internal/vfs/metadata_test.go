package vfs

import (
	"os"
	"path/filepath"
	"testing"
)

// This is ordinary file creation, metadata observation and explicit close.
// No namespace changes, missing selected files or physical failures are used.
func TestMetadataSnapshotsAndResolvedPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ordinary.db")
	f, err := Open(path, OpenOptions{Role: Database, Flags: os.O_RDWR | os.O_CREATE | os.O_EXCL})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err = f.WriteAt([]byte("ordinary payload"), 0); err != nil {
		t.Fatal(err)
	}
	opened, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	named, err := Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !SameFile(opened, named) || !opened.Mode().IsRegular() || opened.Size() != named.Size() || !opened.ModTime().Equal(named.ModTime()) {
		t.Fatal("named and handle snapshots disagree")
	}
	resolved, err := ResolvePath(path)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := Lstat(resolved)
	if err != nil || !SameFile(opened, canonical) {
		t.Fatal("canonical pathname changed identity", err)
	}
	if err = f.ValidatePath(); err != nil {
		t.Fatal(err)
	}
	other, err := Open(filepath.Join(filepath.Dir(path), "peer.db"), OpenOptions{Role: Database, Flags: os.O_RDWR | os.O_CREATE | os.O_EXCL})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := other.Close(); err != nil {
			t.Error(err)
		}
	})
	peer, err := other.Stat()
	if err != nil || SameFile(opened, peer) || SameFile(opened, nil) || SameFile(nil, named) {
		t.Fatal("unrelated or absent snapshots compared equal", err)
	}
}
