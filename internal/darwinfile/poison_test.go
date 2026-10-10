package darwinfile

import (
	"errors"
	"fmt"
	"testing"
)

// Pure memory: no descriptor, native lock/close, or failure injection occurs.
func TestRegistryMemoryRepeatedPoisonRetainsBoundedFirstCause(t *testing.T) {
	r := New()
	id := Identity{Device: 1, Inode: 2}
	first := errors.New("first abstract lock retirement")
	initial := r.Poison(id, first)
	for i := 0; i < 10000; i++ {
		current := fmt.Errorf("abstract retry %d", i)
		result := r.Poison(id, current)
		if r.Error(id) != initial {
			t.Fatal("retry changed the retained error graph")
		}
		if !errors.Is(result, ErrPoisoned) || !errors.Is(result, first) || !errors.Is(result, current) {
			t.Fatal("current or original cause lost")
		}
		if errors.Is(initial, current) {
			t.Fatal("later failure retained in original poison")
		}
	}
	if r.Poison(id, nil) != initial || !errors.Is(r.Error(id), first) {
		t.Fatal("nil observation replaced the initial cause")
	}
}
