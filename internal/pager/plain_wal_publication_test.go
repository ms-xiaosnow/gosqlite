package pager

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// Inert File pointers are identities only. Transfer models never call their
// methods; retirement models use synthetic plainResources throughout.
type plainWALTransferModel struct {
	main, old, next *vfs.File
	calls int
	done bool
}
func (m *plainWALTransferModel) TransferWAL(main, old *vfs.File) (*vfs.File, error) {
	if main != m.main || old != m.old || m.calls != 0 { return nil, ErrPoisoned }
	m.calls++
	return m.next, nil
}
func (m *plainWALTransferModel) Close() error { m.done = true; return nil }
func (m *plainWALTransferModel) RetirementComplete() bool { return m.done }
func TestPlainWALPublicationMemoryExactTransfer(t *testing.T) {
	r := &plainRetirementRegistry{limit: 1}
	g, err := r.reserve()
	if err != nil { t.Fatal(err) }
	main, old, next := &vfs.File{}, &vfs.File{}, &vfs.File{}
	g.main, g.wal = plainFile{main}, plainFile{old}
	m := &plainWALTransferModel{main: main, old: old, next: next}
	if err = g.installWALPublication(m); !errors.Is(err, ErrPoisoned) { t.Fatal("install without reservation") }
	if err = g.reserveTemporary(); err != nil { t.Fatal(err) }
	if err = g.installWALPublication(m); err != nil { t.Fatal(err) }
	if g.temporary != m || g.temporaryReserved { t.Fatal("preowned slot missing") }
	if _, err = g.transferWALPublication(m, &vfs.File{}); !errors.Is(err, ErrPoisoned) || m.calls != 0 { t.Fatal("wrong original consumed transfer") }
	if got, err := g.transferWALPublication(m, old); err != nil || got != next { t.Fatal(err) }
	if g.wal.(plainFile).file != next || g.temporary != m || g.main.(plainFile).file != main || m.calls != 1 { t.Fatal("ownership gap") }
	if _, err = g.transferWALPublication(m, old); !errors.Is(err, ErrPoisoned) || m.calls != 1 { t.Fatal("transfer repeated") }
	// Local inert registry is deliberately not retired through File methods.
}

type plainWALMemoryResource struct {
	name string
	events *[]string
	remaining, calls int
	done bool
	cause error
}
func (m *plainWALMemoryResource) Close() error {
	m.calls++
	*m.events = append(*m.events, m.name)
	if m.remaining > 0 { m.remaining--; return m.cause }
	m.done = true
	return m.cause
}
func (m *plainWALMemoryResource) RetirementComplete() bool { return m.done }
func TestPlainWALPublicationMemoryNoDuplicateUnwind(t *testing.T) {
	for _, diagnostic := range []bool{false, true} {
		r := &plainRetirementRegistry{limit: 1}
		g, err := r.reserve()
		if err != nil { t.Fatal(err) }
		events := []string{}
		main := &plainWALMemoryResource{name: "main", events: &events}
		current := &plainWALMemoryResource{name: "current-wal", events: &events}
		child := &plainWALMemoryResource{name: "composite", events: &events, remaining: 1}
		cause := errors.New("inert retained child diagnostic")
		if diagnostic { child.cause = cause }
		g.main, g.wal, g.temporary = main, current, child
		operation := errors.New("inert publication failure")
		pending := g.closeTemporary(operation)
		if !errors.Is(pending, ErrPlaintextCleanupPending) || !errors.Is(pending, operation) || child.calls != 1 { t.Fatal(pending) }
		unwind := g.beginClass(pending, nil, !g.childClosePending())
		if child.calls != 1 || current.calls != 0 || main.calls != 0 { t.Fatal("same-unwind repeated close", events) }
		if diagnostic && !errors.Is(unwind, cause) { t.Fatal("child diagnostic lost") }
		err = RetryPlaintextCleanup(unwind)
		if errors.Is(err, ErrPlaintextCleanupPending) || !g.done || !reflect.DeepEqual(events, []string{"composite", "composite", "current-wal", "main"}) { t.Fatal(err, events) }
	}
}
func TestPlainWALPublicationMemoryConsumedDiagnosticFailsOpen(t *testing.T) {
	r := &plainRetirementRegistry{limit: 1}
	g, err := r.reserve()
	if err != nil { t.Fatal(err) }
	events := []string{}
	cause := errors.New("inert consumed close diagnostic")
	child := &plainWALMemoryResource{name: "composite", events: &events, cause: cause}
	main := &plainWALMemoryResource{name: "main", events: &events}
	current := &plainWALMemoryResource{name: "current-wal", events: &events}
	g.main, g.wal, g.temporary = main, current, child
	err = g.closeTemporary(nil)
	if !errors.Is(err, cause) || errors.Is(err, ErrPlaintextCleanupPending) || current.calls != 0 || main.calls != 0 { t.Fatal(err) }
	err = g.beginClass(err, nil, !g.childClosePending())
	if !errors.Is(err, cause) || !g.done || !reflect.DeepEqual(events, []string{"composite", "current-wal", "main"}) { t.Fatal(err, events) }
}
