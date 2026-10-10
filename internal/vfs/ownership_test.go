package vfs

import (
	"errors"
	"math"
	"testing"
)

func TestSlotLedgerStrictOwnership(t *testing.T) {
	var s slotLedger
	mode, missing, err := s.prepare(4, 1, false)
	if err != nil || !missing || mode != 1 {
		t.Fatalf("prepare: %d %t %v", mode, missing, err)
	}
	s.assign(4, 1, mode)
	if _, missing, err = s.prepare(4, 1, false); err != nil || missing {
		t.Fatalf("duplicate: %t %v", missing, err)
	}
	before := s
	if _, _, err = s.prepare(1, 7, true); !errors.Is(err, ErrLock) || s != before {
		t.Fatalf("conversion mutated ledger: %v", err)
	}
	s.assign(4, 1, 0)
	mode, missing, err = s.prepare(1, 7, true)
	if err != nil || !missing || mode != 2 {
		t.Fatal(mode, missing, err)
	}
	s.assign(1, 7, mode)
	s.assign(2, 2, 0)
	if s[1] != 2 || s[2] != 0 || s[3] != 0 || s[4] != 2 {
		t.Fatal("partial release changed peers", s)
	}
	for _, r := range [][2]int{{-1, 1}, {8, 1}, {0, 9}, {0, 0}, {7, 2}} {
		if _, _, err = s.prepare(r[0], r[1], true); !errors.Is(err, ErrLock) {
			t.Fatalf("range %v: %v", r, err)
		}
	}
	if _, _, err = s.prepare(0, 2, false); !errors.Is(err, ErrLock) {
		t.Fatal("multi-slot shared acquisition accepted")
	}
}
func TestMappingSpanGranularity(t *testing.T) {
	for _, tt := range []struct {
		off, gran     int64
		base          int64
		delta, length int
	}{
		{0, 65536, 0, 0, 32768}, {32768, 65536, 0, 32768, 65536},
		{65536, 65536, 65536, 0, 32768}, {98304, 65536, 65536, 32768, 65536},
		{32768, 16384, 32768, 0, 32768},
	} {
		base, delta, length, err := mappingSpan(tt.off, 32768, tt.gran)
		if err != nil || base != tt.base || delta != tt.delta || length != tt.length {
			t.Fatalf("%+v: %d %d %d %v", tt, base, delta, length, err)
		}
	}
	for _, tt := range []struct {
		off  int64
		n    int
		gran int64
	}{{-1, 1, 4096}, {0, 0, 4096}, {0, 1, 3}, {math.MaxInt64, 1, 4096}, {1, math.MaxInt, 4096}} {
		if _, _, _, err := mappingSpan(tt.off, tt.n, tt.gran); err == nil {
			t.Fatalf("invalid mapping accepted: %+v", tt)
		}
	}
}
func TestMappingRetirementMemoryModel(t *testing.T) {
	// No native mapping, database, or physical failure is involved in this model.
	f := &File{dms: true, maps: make(map[*Mapping]struct{})}
	m := &Mapping{Bytes: []byte{1}, owner: f}
	f.maps[m] = struct{}{}
	unavailable := errors.New("model view still owned")
	if err := m.retireLocked(func(*nativeMapping) error { return unavailable }); !errors.Is(err, unavailable) {
		t.Fatal(err)
	}
	if len(f.maps) != 1 || m.Bytes == nil || !f.dms {
		t.Fatal("unresolved view lost ownership")
	}
	calls := 0
	if err := m.retireLocked(func(*nativeMapping) error { calls++; return nil }); err != nil {
		t.Fatal(err)
	}
	if len(f.maps) != 0 || m.Bytes != nil || !f.dms {
		t.Fatal("view retirement released DMS early")
	}
	if err := m.retireLocked(func(*nativeMapping) error { calls++; return nil }); err != nil || calls != 1 {
		t.Fatal("retirement is not idempotent")
	}
}

func TestCloseOrderingMemoryModel(t *testing.T) {
	// This abstract backend never opens a file or maps physical storage.
	f := &File{dms: true, maps: make(map[*Mapping]struct{})}
	m := &Mapping{Bytes: []byte{1}, owner: f}
	f.maps[m] = struct{}{}
	closes := 0
	closeFile := func() error { closes++; f.dms = false; return nil }
	blocked := errors.New("model mapping remains live")
	err := f.closeResourcesLocked(func(*nativeMapping) error { return blocked }, closeFile)
	if !errors.Is(err, ErrPoisoned) || closes != 0 || !f.dms || len(f.maps) != 1 {
		t.Fatalf("released unresolved resources: %v", err)
	}
	err = f.closeResourcesLocked(func(*nativeMapping) error {
		if !f.dms || closes != 0 {
			t.Fatal("DMS released before unmap")
		}
		return nil
	}, closeFile)
	if err != nil || closes != 1 || f.dms || len(f.maps) != 0 {
		t.Fatalf("final close: %v", err)
	}
}
