package btree

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
)

type oracleRow struct {
	ID          int64    `json:"id"`
	Label       *string  `json:"label"`
	PayloadHex  string   `json:"payload_hex"`
	PayloadNull int      `json:"payload_null"`
	Score       *float64 `json:"score"`
	Optional    *string  `json:"optional"`
}

func openFixture(t *testing.T, size int) *pager.Reader {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("native SQLite-compatible locks currently Linux-only")
	}
	p := filepath.Join("..", "..", "testdata", "readonly", fmt.Sprintf("page%d.db", size))
	r, e := pager.Open(p)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	})
	return r
}

func schemaRoot(t *testing.T, r *pager.Reader, name string) uint32 {
	t.Helper()
	schema, e := ReadSchema(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range schema {
		if s.Name == name {
			return s.RootPage
		}
	}
	t.Fatalf("schema object %s missing", name)
	return 0
}

func TestOracleRowsAllPageSizes(t *testing.T) {
	expected, e := os.ReadFile(filepath.Join("..", "..", "testdata", "readonly", "samples.json"))
	if e != nil {
		t.Fatal(e)
	}
	var rows []oracleRow
	if e = json.Unmarshal(expected, &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 1028 {
		t.Fatal("oracle row count", len(rows))
	}
	for _, size := range []int{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			r := openFixture(t, size)
			root := schemaRoot(t, r, "samples")
			path := filepath.Join("..", "..", "testdata", "readonly", fmt.Sprintf("page%d.db", size))
			before, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			digest := sha256.Sum256(before)
			c, e := NewCursor(context.Background(), r, root)
			if e != nil {
				t.Fatal(e)
			}
			defer c.Close()
			for i, want := range rows {
				id, got, e := c.Next()
				if e != nil {
					t.Fatalf("row %d: %v", i, e)
				}
				if id != want.ID || len(got) != 5 || got[0] != nil {
					t.Fatalf("row %d id %d values %#v", i, id, got)
				}
				str := func(p *string) any {
					if p == nil {
						return nil
					}
					return *p
				}
				if got[1] != str(want.Label) || got[4] != str(want.Optional) {
					t.Fatalf("row %d text mismatch", i)
				}
				if want.PayloadNull == 1 {
					if got[2] != nil {
						t.Fatalf("row %d expected NULL BLOB", i)
					}
				} else {
					wantBlob, e := hex.DecodeString(want.PayloadHex)
					if e != nil {
						t.Fatal(e)
					}
					gotBlob, ok := got[2].([]byte)
					if !ok || gotBlob == nil || !reflect.DeepEqual(gotBlob, wantBlob) {
						t.Fatalf("row %d blob mismatch", i)
					}
				}
				if want.Score == nil {
					if got[3] != nil {
						t.Fatalf("row %d expected NULL real", i)
					}
				} else {
					var score float64
					switch v := got[3].(type) {
					case int64:
						score = float64(v)
					case float64:
						score = v
					default:
						t.Fatalf("row %d real type %T", i, v)
					}
					if score != *want.Score {
						t.Fatalf("row %d score %g want %g", i, score, *want.Score)
					}
				}
			}
			for i := 0; i < 2; i++ {
				if _, _, e := c.Next(); e != io.EOF {
					t.Fatal("expected sticky EOF", e)
				}
			}
			after, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			if sha256.Sum256(after) != digest {
				t.Fatal("source file changed")
			}
		})
	}
}

func TestOracleIntegerWidthsAndSchema(t *testing.T) {
	want := []int64{math.MinInt64, -140737488355329, -140737488355328, -2147483649, -2147483648, -8388609, -8388608, -32769, -32768, -129, -128, -1, 0, 1, 127, 128, 32767, 32768, 8388607, 8388608, 2147483647, 2147483648, 140737488355327, 140737488355328, math.MaxInt64}
	for _, size := range []int{512, 4096, 65536} {
		r := openFixture(t, size)
		var got []int64
		e := Scan(context.Background(), r, schemaRoot(t, r, "numbers"), func(id int64, row []any) error {
			if len(row) != 1 {
				t.Fatal(row)
			}
			got = append(got, row[0].(int64))
			if id != int64(len(got)) {
				t.Fatal("rowid", id)
			}
			return nil
		})
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatal(got)
		}
		e = Scan(context.Background(), r, schemaRoot(t, r, "quotes"), func(_ int64, row []any) error {
			if !reflect.DeepEqual(row, []any{"it's \"quoted\"", "日本語 한국어 中文 🙂"}) {
				t.Fatal(row)
			}
			return nil
		})
		if e != nil {
			t.Fatal(e)
		}
		if _, e := NewCursor(context.Background(), r, schemaRoot(t, r, "sample_label")); !errors.Is(e, pager.ErrUnsupported) {
			t.Fatal("index accepted", e)
		}
	}
}

func TestCursorCancellationAndEarlyStop(t *testing.T) {
	r := openFixture(t, 512)
	root := schemaRoot(t, r, "samples")
	ctx, cancel := context.WithCancel(context.Background())
	c, e := NewCursor(ctx, r, root)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	if _, _, e = c.Next(); e != nil {
		t.Fatal(e)
	}
	cancel()
	if _, _, e = c.Next(); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, _, e = c.Next(); !errors.Is(e, context.Canceled) {
		t.Fatal("terminal error not sticky", e)
	}
	stop := errors.New("callback stopped")
	count := 0
	if e = Scan(context.Background(), r, root, func(_ int64, _ []any) error { count++; return stop }); !errors.Is(e, stop) || count != 1 {
		t.Fatalf("%v calls %d", e, count)
	}
	c, e = NewCursor(context.Background(), r, root)
	if e != nil {
		t.Fatal(e)
	}
	c.Close()
	if _, _, e = c.Next(); e != io.EOF {
		t.Fatal(e)
	}
}

func TestMalformedPageParser(t *testing.T) {
	p := make([]byte, 512)
	p[0] = 13
	p[5] = 2 // Empty table leaf, content starts at 512.
	if _, e := parsePage(p, 2, 512, bounds{}); e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		name   string
		change func([]byte)
	}{
		{"type", func(p []byte) { p[0] = 0 }},
		{"array", func(p []byte) { p[3] = 255; p[4] = 255 }},
		{"content", func(p []byte) { p[5] = 0; p[6] = 1 }},
		{"fragments", func(p []byte) { p[7] = 61 }},
		{"cell", func(p []byte) { p[4] = 1; p[8] = 2 }},
		{"free", func(p []byte) { p[1] = 1; p[2] = 255 }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			copyP := append([]byte(nil), p...)
			tt.change(copyP)
			if _, e := parsePage(copyP, 2, 512, bounds{}); !errors.Is(e, pager.ErrCorrupt) {
				t.Fatal(e)
			}
		})
	}
}

func TestOracleInteriorSchemaAndOverflow(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native SQLite-compatible locks currently Linux-only")
	}
	path := filepath.Join("..", "..", "testdata", "readonly", "schema512.db")
	r, e := pager.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	p, e := r.ReadPage(context.Background(), 1)
	if e != nil {
		t.Fatal(e)
	}
	if p[100] != 5 {
		t.Fatal("fixture does not cover interior schema root")
	}
	got, e := ReadSchema(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	expected, e := os.ReadFile(filepath.Join("..", "..", "testdata", "readonly", "schema.json"))
	if e != nil {
		t.Fatal(e)
	}
	var want []struct {
		Type  string `json:"type"`
		Name  string `json:"name"`
		Table string `json:"tbl_name"`
		Root  uint32 `json:"rootpage"`
		SQL   string `json:"sql"`
	}
	if e = json.Unmarshal(expected, &want); e != nil {
		t.Fatal(e)
	}
	if len(got) != 81 || len(got) != len(want) {
		t.Fatalf("schema count %d want %d", len(got), len(want))
	}
	for i, w := range want {
		g := got[i]
		if g.Type != w.Type || g.Name != w.Name || g.TableName != w.Table || g.RootPage != w.Root || g.SQL != w.SQL {
			t.Fatalf("schema row %d mismatch", i)
		}
	}
	if len(got[len(got)-1].SQL) < 6000 {
		t.Fatal("schema overflow not covered")
	}
}
