//go:build darwin

package encstore

import (
	"errors"
	"os"
	"testing"
)

// Compile-only on Linux. These are descriptor-free status laws; the zero
// os.File is a pointer sentinel and no method or native callback is invoked.
func TestDarwinEncryptedManagedStatusMemory(t *testing.T) {
	d := &encryptedDarwinDescriptor{raw: &os.File{}, idle: true}
	f := &managedFile{closed: true, descriptor: d}
	d.lease = f
	if !f.RetirementComplete() {
		t.Fatal("safe idle regular lease not transferred to process registry")
	}
	f.closeErr = errors.New("memory terminal close uncertainty")
	if f.RetirementComplete() {
		t.Fatal("terminal close diagnostic treated as complete")
	}
	f.closeErr = nil
	f.locks.held[0] = 1
	if f.RetirementComplete() {
		t.Fatal("unreleased exact hold treated as complete")
	}
	f.locks.held[0] = 0
	d.directory = true
	f.children = 1
	if f.RetirementComplete() {
		t.Fatal("dependent directory treated as complete")
	}
	f.children = 0
	if f.RetirementComplete() {
		t.Fatal("idle directory treated as physical retirement")
	}
	d.raw = nil
	d.parent = &managedFile{}
	if f.RetirementComplete() {
		t.Fatal("retained parent pin treated as complete")
	}
	d.parent = nil
	if !f.RetirementComplete() {
		t.Fatal("retired descriptor and pins retained")
	}
}

func TestDarwinEncryptedManagedStatusMemoryDeferredDirectoryClose(t *testing.T) {
	// The directory's child pin exits closeHeld before lock or raw-file work.
	// raw is only an address sentinel, never an acquired native descriptor.
	d := &encryptedDarwinDescriptor{raw: &os.File{}, directory: true}
	f := &managedFile{descriptor: d, children: 1}
	d.lease = f
	if err := f.closeNative(); err != nil {
		t.Fatal("ordinary logical parent close", err)
	}
	if !f.closed || f.RetirementComplete() || d.raw == nil {
		t.Fatal("nil deferred close confused logical closure with retirement")
	}
}
