package fixturecleanup

import (
	"errors"
	"strings"
	"testing"
)

// Callbacks only: these tests create no files, handles, or credentials.
func TestFinalizeCallbackPolicy(t *testing.T) {
	operationErr := errors.New("operation sentinel")
	closeErr := errors.New("close sentinel")
	removeErr := errors.New("remove sentinel")
	for _, tc := range []struct {
		name      string
		completed bool
		result    error
		removeErr error
		calls     int
		want      []error
		text      string
	}{
		{name: "completed success", completed: true, calls: 1},
		{name: "operation error", completed: true, result: operationErr, want: []error{operationErr}, text: "retained"},
		{name: "joined result", completed: true, result: errors.Join(operationErr, closeErr), want: []error{operationErr, closeErr}, text: "retained"},
		{name: "incomplete without error", text: "did not complete successfully"},
		{name: "incomplete with error", result: operationErr, want: []error{operationErr}, text: "retained"},
		{name: "removal error", completed: true, removeErr: removeErr, calls: 1, want: []error{removeErr}, text: "without retry (removal may be partial)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const dir = "private-example-directory"
			calls := 0
			err := Finalize(dir, tc.completed, tc.result, func(got string) error {
				calls++
				if got != dir {
					t.Fatalf("remove path = %q, want %q", got, dir)
				}
				return tc.removeErr
			})
			if calls != tc.calls {
				t.Fatalf("remove calls = %d, want %d", calls, tc.calls)
			}
			if tc.text == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.text) || !strings.Contains(err.Error(), dir) {
				t.Fatalf("error = %v, want message %q and directory %q", err, tc.text, dir)
			}
			for _, want := range tc.want {
				if !errors.Is(err, want) {
					t.Fatalf("error %v lost sentinel %v", err, want)
				}
			}
		})
	}
}

func TestFinalizeDelayedResult(t *testing.T) {
	lateErr := errors.New("deferred callback sentinel")
	for _, failLate := range []bool{false, true} {
		calls := 0
		run := func() (err error) {
			completed := false
			defer func() {
				err = Finalize("private-example-directory", completed, err, func(string) error {
					calls++
					return nil
				})
			}()
			defer func() {
				if failLate {
					err = errors.Join(err, lateErr)
				}
			}()
			completed = true
			return nil
		}
		err := run()
		if failLate {
			if calls != 0 || !errors.Is(err, lateErr) {
				t.Fatalf("late error = %v, remove calls = %d", err, calls)
			}
		} else if err != nil || calls != 1 {
			t.Fatalf("completed success error = %v, remove calls = %d", err, calls)
		}
	}
}
