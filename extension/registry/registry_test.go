package registry

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type testMethod struct {
	name      string
	version   int
	text      string
	configure atomic.Int64
	keys      atomic.Int64
}

func (m *testMethod) Name() string { return m.name }
func (m *testMethod) Version() int { return m.version }
func (m *testMethod) Configure(canonical []byte) (Index, error) {
	m.configure.Add(1)
	return &testIndex{canonical: bytes.Clone(canonical), text: m.text, calls: &m.keys}, nil
}

type testIndex struct {
	canonical []byte
	text      string
	calls     *atomic.Int64
}

func (i *testIndex) Canonical() []byte { return bytes.Clone(i.canonical) }
func (i *testIndex) Keys(ctx context.Context, input string) ([]Key, error) {
	i.calls.Add(1)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []Key{{Namespace: "test", Length: 1, Text: i.text + input}}, nil
}

func TestRegistryDescriptors(t *testing.T) {
	var typedNil *testMethod
	for _, method := range []IndexMethod{nil, typedNil,
		&testMethod{name: "", version: 1},
		&testMethod{name: "Upper", version: 1},
		&testMethod{name: "a-b", version: 1},
		&testMethod{name: "1a", version: 1},
		&testMethod{name: "é", version: 1},
		&testMethod{name: "a\x00", version: 1},
		&testMethod{name: strings.Repeat("a", 65), version: 1},
		&testMethod{name: "demo", version: 0},
		&testMethod{name: "demo", version: -1},
	} {
		if r, err := New(method); r != nil || !errors.Is(err, ErrMethod) {
			t.Fatalf("accepted invalid method: %T: %v %v", method, r, err)
		}
	}
	a := &testMethod{name: "demo_123", version: 1}
	if r, err := New(a, a); r != nil || !errors.Is(err, ErrDuplicate) {
		t.Fatalf("accepted duplicate descriptor: %v %v", r, err)
	}
	b := &testMethod{name: "demo_123", version: 2}
	long := &testMethod{name: strings.Repeat("a", 64), version: 1}
	r, err := New(a, b, long)
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []*testMethod{a, b, long} {
		got, ok := r.Lookup(method.name, method.version)
		if !ok || got.Name() != method.name || got.Version() != method.version {
			t.Fatalf("missing exact descriptor: %v %v", got, ok)
		}
	}
	for _, id := range []identity{{"DEMO_123", 1}, {"demo_123", 0}, {"demo_123", 3}, {"missing", 1}} {
		if method, ok := r.Lookup(id.name, id.version); method != nil || ok {
			t.Fatalf("lookup substituted a method for %+v", id)
		}
	}
}

func TestRegistrySnapshotAndConnectionIsolation(t *testing.T) {
	a := &testMethod{name: "demo", version: 1, text: "a:"}
	b := &testMethod{name: "demo", version: 1, text: "b:"}
	input := []IndexMethod{a}
	original, err := New(input...)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := original.Snapshot()
	input[0] = b                                  // No caller-owned slice retained.
	delete(original.methods, identity{"demo", 1}) // Verify the defensive map copy.
	other, err := New(b)
	if err != nil {
		t.Fatal(err)
	}
	for n, r := range []*Registry{snapshot, other} {
		method, ok := r.Lookup("demo", 1)
		if !ok {
			t.Fatal("snapshot lost method")
		}
		configured, err := method.Configure([]byte(`{"version":1}`))
		if err != nil {
			t.Fatal(err)
		}
		keys, err := configured.Keys(context.Background(), "value")
		want := []string{"a:value", "b:value"}[n]
		if err != nil || len(keys) != 1 || keys[0].Text != want {
			t.Fatalf("method was not invoked: %+v %v", keys, err)
		}
	}
	if a.configure.Load() != 1 || a.keys.Load() != 1 || b.configure.Load() != 1 || b.keys.Load() != 1 {
		t.Fatal("registry bypassed the registered implementations")
	}
	// Snapshotted descriptors cannot change even if an implementation violates
	// its descriptor immutability requirement after registration.
	a.name, a.version = "changed", 2
	method, ok := snapshot.Lookup("demo", 1)
	if !ok || method.Name() != "demo" || method.Version() != 1 {
		t.Fatal("registered descriptor changed")
	}
	if method, ok := snapshot.Lookup("changed", 2); method != nil || ok {
		t.Fatal("new descriptor leaked into snapshot")
	}
}

func TestRegistryEmptyAndConcurrentSnapshots(t *testing.T) {
	var nilRegistry *Registry
	empty, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []*Registry{nilRegistry, {}, empty, nilRegistry.Snapshot()} {
		if method, ok := r.Lookup("demo", 1); method != nil || ok {
			t.Fatal("empty registry returned a method")
		}
		if copy := r.Snapshot(); copy == nil || len(copy.methods) != 0 {
			t.Fatal("empty snapshot is not usable")
		}
	}
	m := &testMethod{name: "demo", version: 1, text: "registered:"}
	r, err := New(m)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for n := 0; n < 16; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repeat := 0; repeat < 50; repeat++ {
				method, ok := r.Snapshot().Lookup("demo", 1)
				if !ok {
					t.Error("concurrent lookup failed")
					return
				}
				index, err := method.Configure([]byte("canonical"))
				if err != nil {
					t.Error(err)
					return
				}
				keys, err := index.Keys(context.Background(), "value")
				if err != nil || len(keys) != 1 || keys[0].Text != "registered:value" {
					t.Errorf("concurrent method call: %v %v", keys, err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if m.configure.Load() != 800 || m.keys.Load() != 800 {
		t.Fatal("lost actual implementation calls")
	}
}
