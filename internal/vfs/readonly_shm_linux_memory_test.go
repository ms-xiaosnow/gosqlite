//go:build linux

package vfs

import (
	"errors"
	"syscall"
	"testing"
)

func TestReadOnlySHMDMSDecisionMemory(t *testing.T) {
	cases := []struct {
		name       string
		writable   bool
		peer       int16
		initialize bool
		want       error
	}{
		{"readonly unheld", false, syscall.F_UNLCK, false, ErrUnsupported},
		{"readonly live shared", false, syscall.F_RDLCK, false, nil},
		{"readonly exclusive peer", false, syscall.F_WRLCK, false, ErrBusy},
		{"readonly unknown", false, -1, false, ErrUnsupported},
		{"writer unheld", true, syscall.F_UNLCK, true, nil},
		{"writer live shared", true, syscall.F_RDLCK, false, nil},
		{"writer exclusive peer", true, syscall.F_WRLCK, false, ErrBusy},
		{"writer unknown", true, -1, false, ErrUnsupported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initialize, err := readonlySHMDMSDecision(tc.writable, tc.peer)
			if initialize != tc.initialize || !errors.Is(err, tc.want) {
				t.Fatalf("initialize=%v err=%v, want %v %v", initialize, err, tc.initialize, tc.want)
			}
			if !tc.writable && initialize {
				t.Fatal("reader gained initialization or truncate authority")
			}
		})
	}
}

func TestReadOnlySHMMappingProtectionMemory(t *testing.T) {
	if got := readonlySHMProtection(false); got != syscall.PROT_READ {
		t.Fatalf("readonly protection: %d", got)
	}
	if got := readonlySHMProtection(true); got != syscall.PROT_READ|syscall.PROT_WRITE {
		t.Fatalf("writer protection: %d", got)
	}
}
