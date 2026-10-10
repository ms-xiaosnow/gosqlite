package similarity_test

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	sim "github.com/ms-xiaosnow/gosqlite/extension/similarity"
)

// Full unbanded matrix reference deliberately shares no production DP storage,
// band calculation, early stop, or prefix/suffix shortcut.
func referenceDistance(a, b string) int {
	x, y := []rune(a), []rune(b)
	matrix := make([][]int, len(x)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(y)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}
	for i := 1; i <= len(x); i++ {
		for j := 1; j <= len(y); j++ {
			cost := 0
			if x[i-1] != y[j-1] {
				cost = 1
			}
			v := matrix[i-1][j-1] + cost
			if del := matrix[i-1][j] + 1; del < v {
				v = del
			}
			if ins := matrix[i][j-1] + 1; ins < v {
				v = ins
			}
			matrix[i][j] = v
		}
	}
	return matrix[len(x)][len(y)]
}

func TestLevenshteinGolden(t *testing.T) {
	e := defaultEngine(t)
	ctx := context.Background()
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0}, {"", "界🙂", 2}, {"kitten", "sitting", 3}, {"flaw", "lawn", 2},
		{"ab", "ba", 2}, {"é", "e\u0301", 0}, {"🙂", "🙃", 1}, {"👩‍💻", "👩‍🚀", 1},
		{"東京大学", "京都大学", 2}, {"a\x00b", "a\x00c", 1}, {"Straße", "STRASSE", 6},
	}
	for _, tt := range cases {
		d, err := e.Levenshtein(ctx, tt.a, tt.b, nil)
		if err != nil || d != (sim.Distance{Value: tt.want, Exact: true}) {
			t.Fatalf("%q,%q: %+v %v want %d", tt.a, tt.b, d, err, tt.want)
		}
		for maxDistance := 0; maxDistance <= tt.want+1; maxDistance++ {
			d, err = e.Levenshtein(ctx, tt.a, tt.b, &maxDistance)
			want := sim.Distance{Value: tt.want, Exact: true}
			if tt.want > maxDistance {
				want = sim.Distance{Value: maxDistance + 1, Exact: false}
			}
			if err != nil || d != want {
				t.Fatalf("%q,%q max %d: %+v %v want %+v", tt.a, tt.b, maxDistance, d, err, want)
			}
		}
	}
	c := sim.DefaultConfig()
	c.Tokenizer.CaseFold = grams13.CaseFoldUnicode
	d, err := engine(t, c).Levenshtein(ctx, "Straße", "STRASSE", nil)
	if err != nil || d.Value != 0 || !d.Exact {
		t.Fatal(d, err)
	}
}

func TestLevenshteinReferenceProperty(t *testing.T) {
	c := sim.DefaultConfig()
	e := engine(t, c)
	tokenizer, _ := grams13.NewTokenizer(c.Tokenizer)
	ctx := context.Background()
	r := rand.New(rand.NewSource(449941))
	for i := 0; i < 6000; i++ {
		a, b := randomText(r, 24), randomText(r, 24)
		na, err := tokenizer.Normalize(ctx, a)
		if err != nil {
			t.Fatal(err)
		}
		nb, err := tokenizer.Normalize(ctx, b)
		if err != nil {
			t.Fatal(err)
		}
		want := referenceDistance(na, nb)
		got, err := e.Levenshtein(ctx, a, b, nil)
		if err != nil || got != (sim.Distance{Value: want, Exact: true}) {
			t.Fatalf("case %d: %+v %v want %d", i, got, err, want)
		}
		for _, k := range []int{0, 1, 2, 3, max(0, want-1), want, want + 1, 100} {
			expected := sim.Distance{Value: want, Exact: true}
			if want > k {
				expected = sim.Distance{Value: k + 1, Exact: false}
			}
			got, err = e.Levenshtein(ctx, a, b, &k)
			if err != nil || got != expected {
				t.Fatalf("case %d maximum %d: %+v %v want %+v (a=%q,b=%q)", i, k, got, err, expected, a, b)
			}
		}
		reverse, err := e.Levenshtein(ctx, b, a, nil)
		if err != nil || reverse.Value != want {
			t.Fatal("symmetry", got, reverse, err)
		}
	}
}

func TestLevenshteinLimitsAndEarlyStop(t *testing.T) {
	ctx := context.Background()
	c := sim.DefaultConfig()
	c.Limits.MaxDPCells = 1
	e := engine(t, c)
	zero := 0
	d, err := e.Levenshtein(ctx, strings.Repeat("a", 100), strings.Repeat("b", 100), &zero)
	if err != nil || d != (sim.Distance{Value: 1, Exact: false}) {
		t.Fatal("one cell threshold stop", d, err)
	}
	d, err = e.Levenshtein(ctx, strings.Repeat("a", 100), strings.Repeat("b", 100), nil)
	if !errors.Is(err, sim.ErrLimit) || d != (sim.Distance{}) {
		t.Fatal("cell limit", d, err)
	}
	d, err = e.Levenshtein(ctx, "a", "abc", &zero)
	if err != nil || d != (sim.Distance{Value: 1, Exact: false}) {
		t.Fatal("length bound", d, err)
	}
	c.Limits.MaxDistanceRunes = 2
	e = engine(t, c)
	d, err = e.Levenshtein(ctx, "aaa", "aaa", nil)
	if !errors.Is(err, sim.ErrLimit) || d != (sim.Distance{}) {
		t.Fatal("identical input must still enforce runes", d, err)
	}
	for _, maximum := range []int{-1, int(^uint(0) >> 1)} {
		if _, err := e.Levenshtein(ctx, "a", "b", &maximum); !errors.Is(err, sim.ErrValue) {
			t.Fatal(err)
		}
	}
	huge := int(^uint(0)>>1) - 1
	d, err = e.Levenshtein(ctx, "a", "b", &huge)
	if err != nil || d != (sim.Distance{Value: 1, Exact: true}) {
		t.Fatal("huge representable bound", d, err)
	}
	if _, err := e.Levenshtein(nil, "a", "b", nil); !errors.Is(err, sim.ErrContext) {
		t.Fatal(err)
	}
}

type countingContext struct {
	context.Context
	calls    int
	cancelAt int
}

func (c *countingContext) Err() error {
	c.calls++
	if c.calls >= c.cancelAt {
		return context.Canceled
	}
	return nil
}

func TestSimilarityCancellationDuringWork(t *testing.T) {
	e := defaultEngine(t)
	for _, at := range []int{2, 50, 300, 700, 1200} {
		ctx := &countingContext{Context: context.Background(), cancelAt: at}
		d, err := e.Levenshtein(ctx, strings.Repeat("a", 512), strings.Repeat("b", 512), nil)
		if !errors.Is(err, context.Canceled) || d != (sim.Distance{}) {
			t.Fatalf("call %d got %+v %v (checks=%d)", at, d, err, ctx.calls)
		}
	}
	ctx := &countingContext{Context: context.Background(), cancelAt: 40}
	s, err := e.Prepare(ctx, strings.Repeat("ab界", 512))
	if !errors.Is(err, context.Canceled) || s != nil {
		t.Fatal(s, err)
	}
}
