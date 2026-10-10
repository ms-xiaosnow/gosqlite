//go:build !darwin && !(windows && amd64)

package encstore

import (
	"errors"
	"testing"
)

// Descriptor-free state checks. No native failure is installed or executed.
func TestEncryptedManagedStatusMemoryConsumedDiagnostic(t *testing.T) {
	cause := errors.New("memory consumed close diagnostic")
	f := &managedFile{closed: true, consumed: true, closeErr: cause}
	if !f.RetirementComplete() {
		t.Fatal("consumed os.File lease retained because Close reported an error")
	}
	if err := f.closeNative(); !errors.Is(err, cause) {
		t.Fatal("consumed close lost its historical diagnostic", err)
	}
	if !f.RetirementComplete() {
		t.Fatal("repeated close resurrected consumed ownership")
	}
}

func TestEncryptedManagedStatusMemoryDirectoryPins(t *testing.T) {
	f := &managedFile{directory: true, closed: true, children: 1}
	if f.RetirementComplete() {
		t.Fatal("retained directory pin reported complete")
	}
	if err := f.closeNative(); err != nil {
		t.Fatal(err)
	}
	if f.RetirementComplete() {
		t.Fatal("nil deferred close result inferred completion")
	}
	f.children = 0
	f.consumed = true
	if !f.RetirementComplete() {
		t.Fatal("explicitly consumed directory not complete")
	}
	f.parent = &managedFile{}
	if f.RetirementComplete() {
		t.Fatal("retained parent route discarded")
	}
}
