package vfs

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"
)

// Memory callbacks use inert integer identifiers, never native resources.
func TestPathAbsenceMemoryInitialOpenOnly(t *testing.T) {
	for _, cause := range []error{os.ErrNotExist, syscall.ENOENT, &os.PathError{Op: "memory", Path: "memory", Err: syscall.ENOENT}, fmt.Errorf("memory: %w", os.ErrNotExist), errors.Join(os.ErrNotExist)} {
		r := metadataOwners{limit: 1}
		o, err := r.open(func() (uintptr, error) { return 0, cause })
		calls := 0
		value, absent, got := observeMetadataForCreate(&r, o, err, func(uintptr) (string, error) { calls++; return "", nil }, func(uintptr) error { calls++; return nil })
		if !absent || got != cause || value != "" || calls != 0 || len(r.live) != 0 {
			t.Fatal("initial no-handle absence lost", absent, got, calls)
		}
	}
}
func TestPathAbsenceMemoryInspectionNeverAuthorizes(t *testing.T) {
	for _, inspectErr := range []error{os.ErrNotExist, syscall.ENOENT, errors.Join(os.ErrNotExist), errors.Join(os.ErrNotExist, ErrIdentity), nil} {
		for _, closeErr := range []error{nil, errors.New("memory retirement diagnostic")} {
			r := metadataOwners{limit: 1}
			o, err := r.open(func() (uintptr, error) { return 19, nil })
			if err != nil {
				t.Fatal(err)
			}
			closes := 0
			value, absent, got := observeMetadataForCreate(&r, o, nil, func(h uintptr) (string, error) {
				if h != 19 {
					t.Fatal(h)
				}
				return "observed", inspectErr
			}, func(h uintptr) error { closes++; return closeErr })
			if absent || closes != 1 || errors.Is(got, os.ErrNotExist) != errors.Is(inspectErr, os.ErrNotExist) {
				t.Fatal("post-acquisition error authorized creation", absent, got, closes)
			}
			if inspectErr != nil || closeErr != nil {
				if value != "" || got == nil {
					t.Fatal("failed observation published", value, got)
				}
			} else if value != "observed" || got != nil {
				t.Fatal(value, got)
			}
		}
	}
}

type pathAbsenceCycle struct{}

func (*pathAbsenceCycle) Error() string   { return "memory cycle" }
func (e *pathAbsenceCycle) Unwrap() error { return e }
func TestPathAbsenceMemoryMixedAndBounded(t *testing.T) {
	var over error = os.ErrNotExist
	for i := 0; i < 4096; i++ {
		over = errors.Join(over)
	}
	for _, cause := range []error{nil, ErrIdentity, errors.Join(os.ErrNotExist, ErrIdentity), errors.Join(os.ErrNotExist, errors.New("consumed diagnostic")), errors.Join(os.ErrNotExist, os.ErrNotExist), over, &pathAbsenceCycle{}} {
		if ordinaryPathAbsence(cause) {
			t.Fatal("ambiguous graph accepted")
		}
	}
}
