package pager

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func fixture(t *testing.T, size int) (string, []byte) {
	t.Helper()
	p := filepath.Join("..", "..", "testdata", "readonly", fmt.Sprintf("page%d.db", size))
	data, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return p, data
}

func TestHeaderOraclePageSizes(t *testing.T) {
	for _, size := range []int{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			_, p := fixture(t, size)
			h, e := ParseHeader(p[:100], int64(len(p)))
			if e != nil {
				t.Fatal(e)
			}
			if h.PageSize != uint32(size) || h.UsableSize != uint32(size) || h.PageCount != uint32(len(p)/size) || h.SchemaFormat != 4 || h.TextEncoding != 1 || h.UserVersion != 42 || h.ApplicationID != 1196642636 {
				t.Fatalf("%+v", h)
			}
		})
	}
}

func TestHeaderMalformedAndUnsupported(t *testing.T) {
	_, data := fixture(t, 4096)
	cases := []struct {
		name   string
		mutate func([]byte)
		kind   error
	}{
		{"magic", func(p []byte) { p[0] = 0 }, ErrCorrupt},
		{"size", func(p []byte) { p[16] = 1; p[17] = 1 }, ErrCorrupt},
		{"WAL", func(p []byte) { p[18] = 2 }, ErrUnsupported},
		{"encoding", func(p []byte) { p[59] = 2 }, ErrUnsupported},
		{"auto_vacuum", func(p []byte) { p[55] = 1 }, ErrUnsupported},
		{"incremental_vacuum", func(p []byte) { p[67] = 1 }, ErrUnsupported},
		{"schema", func(p []byte) { p[47] = 5 }, ErrUnsupported},
		{"fraction", func(p []byte) { p[21] = 63 }, ErrCorrupt},
		{"pages", func(p []byte) { binary.BigEndian.PutUint32(p[28:], 0xffffffff) }, ErrCorrupt},
		{"free_count", func(p []byte) { binary.BigEndian.PutUint32(p[36:], 0xffffffff) }, ErrCorrupt},
		{"reserved", func(p []byte) { p[72] = 1 }, ErrUnsupported},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			p := append([]byte(nil), data[:100]...)
			tt.mutate(p)
			if _, e := ParseHeader(p, int64(len(data))); !errors.Is(e, tt.kind) {
				t.Fatalf("%v", e)
			}
		})
	}
	for _, size := range []int64{0, 99, int64(len(data) - 1)} {
		if _, e := ParseHeader(data[:100], size); !errors.Is(e, ErrCorrupt) {
			t.Fatal(e)
		}
	}
	for i := 0; i < 100; i++ {
		if _, e := ParseHeader(data[:i], int64(len(data))); !errors.Is(e, ErrCorrupt) {
			t.Fatalf("short %d %v", i, e)
		}
	}
	// Old SQLite headers use physical size if the in-header count is invalid.
	p := append([]byte(nil), data[:100]...)
	p[95] ^= 1
	binary.BigEndian.PutUint32(p[28:], 0xffffffff)
	h, e := ParseHeader(p, int64(len(data)))
	if e != nil || h.PageCount != uint32(len(data)/4096) {
		t.Fatalf("legacy size %+v %v", h, e)
	}
}

func TestReadOnlyPageAccess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native SQLite-compatible locks currently Linux-only")
	}
	for _, size := range []int{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path, data := fixture(t, size)
			before := sha256.Sum256(data)
			r, e := Open(path)
			if e != nil {
				t.Fatal(e)
			}
			defer r.Close()
			p, e := r.ReadPage(context.Background(), 1)
			if e != nil || len(p) != size {
				t.Fatalf("%d %v", len(p), e)
			}
			p[0] = 0
			q, e := r.ReadPage(context.Background(), 1)
			if e != nil || q[0] != 'S' {
				t.Fatal("page buffer aliases storage", e)
			}
			for _, n := range []uint32{0, r.Header().PageCount + 1} {
				if _, e := r.ReadPage(context.Background(), n); !errors.Is(e, ErrCorrupt) {
					t.Fatalf("page %d: %v", n, e)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, e := r.ReadPage(ctx, 1); !errors.Is(e, context.Canceled) {
				t.Fatal(e)
			}
			if e = r.Close(); e != nil {
				t.Fatal(e)
			}
			if e = r.Close(); e != nil {
				t.Fatal(e)
			}
			if _, e := r.ReadPage(context.Background(), 1); !errors.Is(e, ErrClosed) {
				t.Fatal(e)
			}
			after, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			if sha256.Sum256(after) != before {
				t.Fatal("fixture modified")
			}
		})
	}
}
