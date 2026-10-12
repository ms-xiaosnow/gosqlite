package pager

import (
	"errors"
	"io"
	"testing"
)

// This fake has no descriptors, native owner, filesystem or callbacks.
type rollbackMemoryWriter struct {
	n     int
	err   error
	calls int
	got   []byte
	off   int64
}

func (w *rollbackMemoryWriter) WriteAt(b []byte, off int64) (int, error) {
	w.calls++
	w.got = append([]byte(nil), b...)
	w.off = off
	return w.n, w.err
}

func TestRollbackWriteFullAtMemoryContract(t *testing.T) {
	original := errors.New("memory-only original error")
	for _, tc := range []struct {
		name string
		n    int
		err  error
		want error
	}{
		{"complete", 4, nil, nil},
		{"zero-short", 0, nil, io.ErrShortWrite},
		{"prefix-short", 2, nil, io.ErrShortWrite},
		{"long-invalid-count", 5, nil, io.ErrShortWrite},
		{"short-error-priority", 2, original, original},
		{"full-error-priority", 4, original, original},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := &rollbackMemoryWriter{n: tc.n, err: tc.err}
			if got := writeFullAt(w, []byte("body"), 37); got != tc.want {
				t.Fatalf("got %v, want exact error %v", got, tc.want)
			}
			if w.calls != 1 || string(w.got) != "body" || w.off != 37 {
				t.Fatalf("write retried or arguments changed: %+v", w)
			}
		})
	}
}
