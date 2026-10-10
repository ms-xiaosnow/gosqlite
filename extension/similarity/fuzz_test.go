package similarity_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	sim "github.com/ms-xiaosnow/gosqlite/extension/similarity"
)

// This fuzz target performs only bounded in-memory mathematical operations. It
// opens no database/files and invokes no crash, recovery, fault, or process test.
func FuzzSimilarityMath(f *testing.F) {
	for _, seed := range []struct{ a, b string }{
		{"", ""}, {"a", "b"}, {"数据库查询", "数据检索"}, {"e\u0301", "é"}, {"👩‍💻", "👩‍🚀"}, {"Straße", "STRASSE"}, {"\xff", "ok"}, {"a\x00x", "a\x00y"},
	} {
		f.Add(seed.a, seed.b, uint8(1), uint8(3), .5)
	}
	f.Fuzz(func(t *testing.T, a, b string, n, kByte uint8, threshold float64) {
		// Size filters bound the reference matrix and tokenizer on arbitrary
		// fuzzer input; invalid UTF-8 inside this bound must be rejected normally.
		if len(a) > 192 || len(b) > 192 {
			return
		}
		c := sim.DefaultConfig()
		c.MinGram = 1 + int(n)%3
		c.MaxGram = c.MinGram + int(kByte)%(4-c.MinGram)
		c.Tokenizer.Limits.MaxInputBytes = 192
		c.Tokenizer.Limits.MaxInputRunes = 192
		c.Tokenizer.Limits.MaxOutputBytes = 768
		c.Tokenizer.Limits.MaxOutputRunes = 768
		c.Tokenizer.Limits.MaxDistinctTokens = 2304
		c.Tokenizer.Limits.MaxTokenBytes = 12288
		c.Limits.MaxDistanceRunes = 768
		c.Limits.MaxDPCells = 1 << 20
		if n&4 != 0 {
			c.Tokenizer.CaseFold = grams13.CaseFoldUnicode
		}
		e := engine(t, c)
		ctx := context.Background()
		tokenizer, _ := grams13.NewTokenizer(c.Tokenizer)
		na, errA := tokenizer.Normalize(ctx, a)
		nb, errB := tokenizer.Normalize(ctx, b)
		m := sim.Dice
		if n&8 != 0 {
			m = sim.Jaccard
		}
		got, err := e.Compare(ctx, a, b, m)
		if errA != nil || errB != nil {
			expected := errA
			if expected == nil {
				expected = errB
			}
			if err == nil {
				t.Fatal("accepted rejected normalization", expected)
			}
			for _, category := range []error{grams13.ErrInvalidUTF8, grams13.ErrLimit, grams13.ErrStreamSafe} {
				if errors.Is(expected, category) && !errors.Is(err, category) {
					t.Fatal(err, expected)
				}
			}
			if _, err := e.Levenshtein(ctx, a, b, nil); err == nil {
				t.Fatal("distance accepted rejected normalization")
			}
			return
		}
		want := referenceScore(na, nb, c.MinGram, c.MaxGram, m)
		if err != nil || got != want {
			t.Fatal(got, err, want)
		}
		distance := referenceDistance(na, nb)
		exact, err := e.Levenshtein(ctx, a, b, nil)
		if err != nil || exact != (sim.Distance{Value: distance, Exact: true}) {
			t.Fatal(exact, err, distance)
		}
		k := int(kByte) % 32
		bounded, err := e.Levenshtein(ctx, a, b, &k)
		expected := sim.Distance{Value: distance, Exact: true}
		if distance > k {
			expected = sim.Distance{Value: k + 1, Exact: false}
		}
		if err != nil || bounded != expected {
			t.Fatal(bounded, err, expected)
		}
		minimum, possible, err := sim.MinimumIntersection(got.LeftTokens, got.RightTokens, threshold, m)
		invalid := math.IsNaN(threshold) || math.IsInf(threshold, 0) || threshold < 0 || threshold > 1
		if invalid {
			if err == nil {
				t.Fatal("accepted invalid threshold")
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if got.Value >= threshold && (!possible || got.Intersection < minimum) {
			t.Fatal("bound lost exact match", got, minimum, possible, threshold)
		}
		plan, err := sim.PlanCandidates(got.LeftTokens, threshold)
		if err != nil {
			t.Fatal(err)
		}
		if got.Value >= threshold && plan.Mode == sim.UnionPostings && got.Intersection == 0 {
			t.Fatal("posting union lost exact match", got, threshold)
		}
	})
}
