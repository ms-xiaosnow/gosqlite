package btree

import (
	"context"
	"encoding/binary"
	"errors"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"testing"
)

type ownershipMemory struct {
	h pager.Header
	p map[uint32][]byte
}

func (r ownershipMemory) Header() pager.Header { return r.h }
func (r ownershipMemory) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p, ok := r.p[n]
	if !ok {
		return nil, pager.ErrCorrupt
	}
	return append([]byte(nil), p...), nil
}
func ownershipFixture() ownershipMemory {
	r := ownershipMemory{h: pager.Header{PageSize: 512, UsableSize: 512, PageCount: 3, SchemaFormat: 4, TextEncoding: 1}, p: map[uint32][]byte{}}
	for n := uint32(1); n <= 3; n++ {
		p := make([]byte, 512)
		offset := 0
		if n == 1 {
			offset = 100
		}
		p[offset] = 13
		binary.BigEndian.PutUint16(p[offset+5:], 512)
		r.p[n] = p
	}
	r.p[3][0] = 10
	return r
}
func TestOwnershipMemoryValidation(t *testing.T) {
	r := ownershipFixture()
	roots := []RootSpec{{Page: 2, Kind: TableTree}, {Page: 3, Kind: IndexTree, KeyInfo: KeyInfo{Columns: 1}}}
	if err := CheckOwnership(context.Background(), r, roots, nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		roots []RootSpec
		free  []uint32
	}{
		{"live listed free", roots, []uint32{2}},
		{"duplicate root", append(append([]RootSpec{}, roots...), roots[0]), nil},
		{"unclaimed page", roots[:1], nil},
		{"duplicate free", roots[:1], []uint32{3, 3}},
		{"invalid free", roots, []uint32{4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := CheckOwnership(context.Background(), r, tc.roots, tc.free); !errors.Is(err, pager.ErrCorrupt) {
				t.Fatalf("expected corruption classification, got %v", err)
			}
		})
	}
	if err := CheckOwnership(context.Background(), r, roots[:1], []uint32{3}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := CheckOwnership(ctx, r, roots, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
