package pager

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"
)

// These are error/owner decision laws only. No paths, descriptors or native
// faults are used to distinguish initial absence from a later missing path.
func TestPlainCleanupMemoryInitialAbsence(t *testing.T) {
	for _, cause := range []error{os.ErrNotExist, syscall.ENOENT, &os.PathError{Op: "memory", Path: "memory", Err: syscall.ENOENT}, fmt.Errorf("memory: %w", os.ErrNotExist), errors.Join(os.ErrNotExist)} {
		marker := markInitialReaderAbsence(cause, true)
		for _, wrapped := range []error{marker, fmt.Errorf("memory: %w", marker), errors.Join(marker), errors.Join(nil, marker)} {
			if !ReaderProbeAbsent(wrapped) || !errors.Is(wrapped, os.ErrNotExist) {
				t.Fatal("conclusive initial absence lost", wrapped)
			}
		}
		reg := &plainRetirementRegistry{limit: 1}
		g, err := reg.reserve()
		if err != nil {
			t.Fatal(err)
		}
		// The actual failed Reader constructor returns cleanup.begin's join.
		if got := g.begin(marker); !ReaderProbeAbsent(got) || !g.done || len(reg.groups) != 0 {
			t.Fatal("empty constructor retirement lost initial absence", got)
		}
	}
}

func TestPlainCleanupMemoryAmbiguousAbsence(t *testing.T) {
	consumed := errors.New("memory consumed cleanup diagnostic")
	for _, cause := range []error{nil, ErrBusy, errors.Join(os.ErrNotExist, ErrChanged), errors.Join(os.ErrNotExist, consumed), errors.Join(os.ErrNotExist, ErrPoisoned), fmt.Errorf("%w: database path: %w", ErrChanged, os.ErrNotExist)} {
		if ReaderProbeAbsent(markInitialReaderAbsence(cause, true)) {
			t.Fatal("ambiguous initial error authorized creation", cause)
		}
	}
	marker := markInitialReaderAbsence(os.ErrNotExist, true)
	for _, cause := range []error{os.ErrNotExist, errors.Join(os.ErrNotExist), errors.Join(marker, ErrChanged), errors.Join(marker, consumed), errors.Join(marker, marker), errors.Join(marker, ErrPoisoned)} {
		if ReaderProbeAbsent(cause) {
			t.Fatal("post-acquisition or mixed error authorized creation", cause)
		}
	}
	for _, pending := range []bool{false, true} {
		reg := &plainRetirementRegistry{limit: 1}
		g, err := reg.reserve()
		if err != nil {
			t.Fatal(err)
		}
		owner := &plainMemoryOwner{consumed: true, cause: consumed}
		if pending {
			owner.remaining = 1
		}
		g.main = owner
		got := g.begin(marker)
		if ReaderProbeAbsent(got) || !errors.Is(got, consumed) || errors.Is(got, ErrPlaintextCleanupPending) != pending {
			t.Fatal("cleanup diagnostic authorized creation", got)
		}
		if pending {
			_ = RetryPlaintextCleanup(got)
		}
	}
}

type plainAbsenceCycle struct{}

func (e *plainAbsenceCycle) Error() string { return "memory cycle" }
func (e *plainAbsenceCycle) Unwrap() error { return e }
func TestPlainCleanupMemoryAbsenceBound(t *testing.T) {
	var wrapped error = markInitialReaderAbsence(os.ErrNotExist, true)
	for i := 0; i < maxPlainCleanupErrorNodes; i++ {
		wrapped = errors.Join(wrapped)
	}
	for _, cause := range []error{wrapped, &plainAbsenceCycle{}} {
		if ReaderProbeAbsent(cause) {
			t.Fatal("unbounded error graph authorized creation")
		}
	}
	if ReaderProbeAbsent(markInitialReaderAbsence(&plainAbsenceCycle{}, true)) {
		t.Fatal("unbounded initial graph authorized creation")
	}
}

func TestPlainCleanupMemoryPostAcquisitionAbsence(t *testing.T) {
	for _, cause := range []error{os.ErrNotExist, syscall.ENOENT, errors.Join(os.ErrNotExist)} {
		if ReaderProbeAbsent(markInitialReaderAbsence(cause, false)) {
			t.Fatal("post-acquisition metadata error authorized create", cause)
		}
	}
}
