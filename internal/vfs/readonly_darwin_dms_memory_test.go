package vfs

import (
	"errors"
	"testing"
)

// Exercise the actual production policy over its complete uint8 input domain.
// This test opens no storage descriptor and never calls a native backend.
func TestDarwinReadOnlyDMSDecisionMemory(t *testing.T) {
	if darwinDMSUnknown != 0 {
		t.Fatal("zero peer must remain unknown")
	}
	for encoded := 0; encoded < 256; encoded++ {
		for _, writable := range []bool{false, true} {
			peer := darwinDMSPeer(encoded)
			beforePeer, beforeWritable := peer, writable
			initialize, err := readonlyDarwinDMSDecision(writable, peer)
			wantInitialize, wantErr := false, ErrUnsupported
			switch peer {
			case darwinDMSUnheld:
				if writable {
					wantInitialize, wantErr = true, nil
				}
			case darwinDMSShared:
				wantErr = nil
			case darwinDMSExclusive:
				wantErr = ErrBusy
			}
			if initialize != wantInitialize || !errors.Is(err, wantErr) {
				t.Fatalf("writable=%t peer=%d: initialize=%t err=%v; want %t %v", writable, peer, initialize, err, wantInitialize, wantErr)
			}
			if initialize && (!writable || peer != darwinDMSUnheld || err != nil) {
				t.Fatal("initialization escaped its sole writable/unheld authority")
			}
			if peer != beforePeer || writable != beforeWritable {
				t.Fatal("policy changed its inputs")
			}
		}
	}
}
