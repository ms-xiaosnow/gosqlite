//go:build windows && amd64

package encstore

import (
	"errors"
	"os"
	"testing"
)

// Compile-only on Linux. No HANDLE, native close, or lock call is used.
func TestWindowsEncryptedManagedStatusMemory(t *testing.T) {
	f := &managedFile{handleLifetime: handleLifetime{closed: true}}
	if f.RetirementComplete() {
		t.Fatal("nil raw field inferred retirement without explicit state")
	}
	f.retired = true
	f.locks.poisoned = errors.New("memory historical diagnostic")
	if !f.RetirementComplete() {
		t.Fatal("historical diagnostic hid explicit consumed ownership")
	}
	f.closeUnknown = true
	if f.RetirementComplete() {
		t.Fatal("unknown close state treated as complete")
	}
	f.closeUnknown = false
	f.children = 1
	if f.RetirementComplete() {
		t.Fatal("directory pin treated as retired")
	}
	f.children = 0
	f.locks.held[1] = 2
	if f.RetirementComplete() {
		t.Fatal("unreleased exact lock treated as retired")
	}
	f.locks.held[1] = 0
	f.parent = &managedFile{}
	if f.RetirementComplete() {
		t.Fatal("retained parent route discarded")
	}
}

func TestWindowsEncryptedManagedStatusMemoryDeferredDirectoryClose(t *testing.T) {
	// handleLifetime exits on the child pin before invoking either callback.
	// The zero os.File is only an address sentinel, never a native HANDLE.
	f := &managedFile{raw: &os.File{}, directory: true, handleLifetime: handleLifetime{children: 1}}
	if err := f.closeNative(); err != nil {
		t.Fatal("ordinary logical parent close", err)
	}
	if !f.closed || f.RetirementComplete() || f.raw == nil {
		t.Fatal("nil deferred close confused logical closure with retirement")
	}
}
