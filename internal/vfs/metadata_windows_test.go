//go:build windows && amd64

package vfs

import "testing"

// Pure value comparison, compiled with the Windows backend but NOT_RUN on
// the Linux host. No native call, real handle, or metadata failure is used.
func TestWindowsMetadataUsesEveryIdentityBit(t *testing.T) {
	a := &winFileInfo{id: winIdentity{volume: 19, file: [16]byte{1, 2, 3}}}
	b := *a
	if !SameFile(a, &b) {
		t.Fatal("equal native IDs compare unequal")
	}
	for bit := range 64 {
		b = *a
		b.id.volume ^= uint64(1) << bit
		if SameFile(a, &b) {
			t.Fatalf("volume identity bit %d was ignored", bit)
		}
	}
	for bit := range 128 {
		b = *a
		b.id.file[bit/8] ^= byte(1) << (bit % 8)
		if SameFile(a, &b) {
			t.Fatalf("file identity bit %d was ignored", bit)
		}
	}
	var nilInfo *winFileInfo
	if SameFile(a, nilInfo) || SameFile(nilInfo, a) || SameFile(nilInfo, nilInfo) {
		t.Fatal("nil snapshots compare equal")
	}
}
