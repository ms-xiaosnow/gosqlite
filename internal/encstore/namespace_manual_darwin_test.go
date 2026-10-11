//go:build manual_namespace

package encstore

// Restricted direct namespace-publication primitive test. Retained source only;
// do not execute or relocate it into ordinary CI. Previous evidence is compile
// only, not native runtime. The OS/architecture filename also restricts selection.

import (
	"bytes"
	"context"
	"errors"
	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/darwinfile"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDarwinEncryptedRelativePublicationAndParentLifetime(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_NAMESPACE") != "independently_authorized" {
		t.Skip("restricted raw namespace publication test: excluded from ordinary CI")
	}
	path := filepath.Join(t.TempDir(), "ordinary.gse")
	ns, e := newNamespace(path)
	if e != nil {
		t.Fatal(e)
	}
	defer ns.dir.Close()
	if _, e = inventory(context.Background(), path, 1<<20, 64, ns); e != nil {
		t.Fatal(e)
	}
	pending := path + ".gse-" + strings.Repeat("a", 64) + ".pending"
	if e = ns.write(pending, []byte("first")); e != nil {
		t.Fatal(e)
	}
	if e = ns.rename(pending, path, true); e != nil {
		t.Fatal(e)
	}
	root, info, e := ns.checked(path, os.O_RDONLY)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	if e = ns.stable(root, path, info); e != nil {
		t.Fatal(e)
	}
	next := path + ".gse-" + strings.Repeat("b", 64) + ".pending"
	if e = ns.write(next, []byte("after")); e != nil {
		t.Fatal(e)
	}
	if e = ns.rename(next, path, false); e != nil {
		t.Fatal(e)
	}
	old, e := readExact(root, 5, 0)
	if e != nil || !bytes.Equal(old, []byte("first")) {
		t.Fatal("retained root", e)
	}
	current, ci, e := ns.checked(path, os.O_RDONLY)
	if e != nil {
		t.Fatal(e)
	}
	defer current.Close()
	raw, e := readExact(current, 5, 0)
	if e != nil || !bytes.Equal(raw, []byte("after")) {
		t.Fatal("published root", e)
	}
	if e = ns.stable(current, path, ci); e != nil {
		t.Fatal(e)
	}
	if e = ns.dir.Close(); e != nil {
		t.Fatal(e)
	}
	if e = platformDirectoryCheck(ns.dir, ns.path, ns.info); !errors.Is(e, encryption.ErrClosed) {
		t.Fatal("public directory resurrected", e)
	}
	if e = ns.stable(current, path, ci); e != nil {
		t.Fatal("retained child lost parent", e)
	}
	if e = root.Close(); e != nil {
		t.Fatal(e)
	}
	if e = current.Close(); e != nil {
		t.Fatal(e)
	}
	darwinfile.Process.Lock()
	left := ns.dir.children
	rawDir := ns.dir.descriptor.raw
	darwinfile.Process.Unlock()
	if left != 0 || rawDir != nil {
		t.Fatal("directory retained after last physical child")
	}
}
