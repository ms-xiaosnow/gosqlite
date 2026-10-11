//go:build manual_namespace

package encstore

// Restricted direct namespace-publication primitive test. Retained source only;
// do not execute or relocate it into ordinary CI. Previous evidence is compile
// only, not native runtime. The OS/architecture filename also restricts selection.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsEncryptedRelativePublicationAndRetainedRoot(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_NAMESPACE") != "independently_authorized" {
		t.Skip("restricted raw namespace publication test: excluded from ordinary CI")
	}
	path := filepath.Join(t.TempDir(), "ordinary Ω family.gse")
	ns, err := newNamespace(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ns.dir.Close()
	lock, _, err := ns.checked(path+".gse-lock", os.O_RDWR|os.O_CREATE|os.O_EXCL)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err = lockFile(lock, false); err != nil {
		t.Fatal(err)
	}
	if _, err = inventory(context.Background(), path, 1<<20, 64, ns); err != nil {
		t.Fatal("relative directory inventory", err)
	}
	pending := path + ".gse-" + strings.Repeat("a", 64) + ".pending"
	if err = ns.write(pending, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err = ns.rename(pending, path, true); err != nil {
		t.Fatal(err)
	}
	root, info, err := ns.checked(path, os.O_RDONLY)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err = ns.stable(root, path, info); err != nil {
		t.Fatal(err)
	}
	next := path + ".gse-" + strings.Repeat("b", 64) + ".pending"
	if err = ns.write(next, []byte("after")); err != nil {
		t.Fatal(err)
	}
	if err = ns.rename(next, path, false); err != nil {
		t.Fatal(err)
	}
	// Replacement is an ordinary authorized cutover. The older open handle
	// retains its original bytes until explicit close, without a copy fallback.
	old, err := readExact(root, 5, 0)
	if err != nil || !bytes.Equal(old, []byte("first")) {
		t.Fatal("old root handle", err)
	}
	current, ci, err := ns.checked(path, os.O_RDONLY)
	if err != nil {
		t.Fatal(err)
	}
	defer current.Close()
	raw, err := readExact(current, 5, 0)
	if err != nil || !bytes.Equal(raw, []byte("after")) {
		t.Fatal("new root handle", err)
	}
	if err = ns.stable(current, path, ci); err != nil {
		t.Fatal(err)
	}
	if err = ns.check(); err != nil {
		t.Fatal(err)
	}
}
