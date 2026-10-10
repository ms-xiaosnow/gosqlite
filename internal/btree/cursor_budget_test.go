package btree

import (
	"encoding/binary"
	"errors"
	"testing"
	"unsafe"
)

// Valid page layouts live only in memory. This exercises quota refusal before
// parser allocations, never physical corruption or native I/O failure.
func TestRelationalCursorParseAllocationPreflight(t *testing.T) {
	page := make([]byte, 512)
	page[0] = 13
	count := 70
	binary.BigEndian.PutUint16(page[3:5], uint16(count))
	binary.BigEndian.PutUint16(page[5:7], uint16(len(page)-4*count))
	for i := 0; i < count; i++ {
		off := len(page) - 4*(i+1)
		binary.BigEndian.PutUint16(page[8+2*i:10+2*i], uint16(off))
		copy(page[off:], []byte{2, byte(i + 1), 2, 0})
	}
	sentinel := errors.New("pure memory allocation quota")
	var requested int64
	frame, err := parsePageBudget(page, 2, len(page), bounds{}, func(n int64) error { requested = n; return sentinel })
	want := int64(count)*int64(unsafe.Sizeof(cell{})+unsafe.Sizeof(span{})) + 512
	if !errors.Is(err, sentinel) || requested != want || frame.cells != nil || requested <= int64(len(page))*4+512 {
		t.Fatal(requested, want, frame.cells, err)
	}
	parsed, err := parsePageBudget(page, 2, len(page), bounds{}, func(n int64) error {
		if n != want {
			t.Fatal(n, want)
		}
		return nil
	})
	if err != nil || len(parsed.cells) != count {
		t.Fatal(len(parsed.cells), err)
	}
	// The freeblock span is included in exact capacity before make as well.
	page = make([]byte, 512)
	page[0] = 13
	binary.BigEndian.PutUint16(page[1:3], 504)
	binary.BigEndian.PutUint16(page[3:5], 1)
	binary.BigEndian.PutUint16(page[5:7], 504)
	binary.BigEndian.PutUint16(page[8:10], 508)
	binary.BigEndian.PutUint16(page[506:508], 4)
	copy(page[508:], []byte{2, 1, 2, 0})
	want = int64(unsafe.Sizeof(cell{})) + 2*int64(unsafe.Sizeof(span{})) + 512
	_, err = parsePageBudget(page, 2, len(page), bounds{}, func(n int64) error {
		if n != want {
			t.Fatal(n, want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
