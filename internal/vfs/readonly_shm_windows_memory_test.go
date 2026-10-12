package vfs

import (
	"errors"
	"strconv"
	"testing"
)

// This exercises the production decision directly, with every value of its
// uint8 observation domain. No File, native handle or I/O callback is created.
func TestWindowsReadOnlyDMSDecisionMemory(t *testing.T) {
	for _, writable := range []bool{false, true} {
		for value := 0; value < 256; value++ {
			observation := windowsDMSObservation(value)
			name := "readonly/"
			if writable {
				name = "writable/"
			}
			t.Run(name+strconv.Itoa(value), func(t *testing.T) {
				initialize, err := readonlyWindowsDMSDecision(writable, observation)
				wantInitialize := writable && observation == windowsDMSProbeAcquired
				wantRefusal := observation != windowsDMSLocalAttached && observation != windowsDMSProbeContended && !wantInitialize
				if initialize != wantInitialize {
					t.Fatalf("initialize = %v, want %v", initialize, wantInitialize)
				}
				if wantRefusal {
					if !errors.Is(err, ErrUnsupported) {
						t.Fatalf("refusal = %v, want ErrUnsupported", err)
					}
				} else if err != nil {
					t.Fatalf("authorized next step: %v", err)
				}
			})
		}
	}
	// Keep the default observation separate from either positive observation.
	if windowsDMSUnknown != 0 || windowsDMSLocalAttached != 1 || windowsDMSProbeAcquired != 2 || windowsDMSProbeContended != 3 {
		t.Fatal("Windows DMS observations lost their distinct meanings")
	}
}
