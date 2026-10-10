package similarity_test

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	sim "github.com/ms-xiaosnow/gosqlite/extension/similarity"
)

func engine(t testing.TB, c sim.Config) *sim.Engine {
	t.Helper()
	e, err := sim.New(c)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func defaultEngine(t testing.TB) *sim.Engine { return engine(t, sim.DefaultConfig()) }

// Independent reference: rune windows in maps, rather than tokenizer token
// slices, merge intersections, or production score/bound helpers. Unicode
// normalization has separate golden cases; reuse only that explicitly shared
// versioned transformation for randomized mathematical tests.
func referenceSet(s string, lo, hi int) map[string]bool {
	r := []rune(s)
	out := map[string]bool{}
	for n := lo; n <= hi; n++ {
		for j := 0; j+n <= len(r); j++ {
			out[string(r[j:j+n])] = true
		}
	}
	return out
}
func referenceScore(a, b string, lo, hi int, m sim.Metric) sim.Score {
	x, y := referenceSet(a, lo, hi), referenceSet(b, lo, hi)
	common := 0
	for term := range x {
		if y[term] {
			common++
		}
	}
	result := sim.Score{LeftTokens: len(x), RightTokens: len(y), Intersection: common}
	if len(x)+len(y) == 0 {
		result.Value = 1
	} else if m == sim.Dice {
		result.Value = float64(common*2) / float64(len(x)+len(y))
	} else {
		result.Value = float64(common) / float64(len(x)+len(y)-common)
	}
	return result
}

func TestSimilarityGolden(t *testing.T) {
	cases := []struct {
		a, b                   string
		n                      int
		metric                 sim.Metric
		want                   float64
		aCount, bCount, common int
	}{
		{"", "", 2, sim.Dice, 1, 0, 0, 0}, {"", "x", 2, sim.Jaccard, 1, 0, 0, 0},
		{"a", "b", 2, sim.Dice, 1, 0, 0, 0}, {"a", "bc", 2, sim.Dice, 0, 0, 1, 0},
		{"aaaaa", "aa", 2, sim.Dice, 1, 1, 1, 1},
		{"abcd", "abce", 2, sim.Dice, 2.0 / 3, 3, 3, 2},
		{"abcd", "abce", 2, sim.Jaccard, .5, 3, 3, 2},
		{"数据库查询", "数据检索", 2, sim.Dice, 2.0 / 7, 4, 3, 1},
		{"数据库查询", "数据检索", 3, sim.Jaccard, 0, 3, 2, 0},
		{"カタカナ", "カタログ", 2, sim.Dice, 1.0 / 3, 3, 3, 1},
		{"🙂🙃🙂", "🙂🙃", 2, sim.Jaccard, .5, 2, 1, 1},
		{"e\u0301", "é", 1, sim.Dice, 1, 1, 1, 1},
		{"a\x00b", "a\x00c", 1, sim.Jaccard, .5, 3, 3, 2},
		{"A", "a", 1, sim.Dice, 0, 1, 1, 0},
		{"👩‍💻", "👩‍🚀", 1, sim.Dice, 2.0 / 3, 3, 3, 2},
		{"가", "가", 1, sim.Jaccard, 1, 1, 1, 1},
	}
	for _, tt := range cases {
		c := sim.DefaultConfig()
		c.MinGram, c.MaxGram = tt.n, tt.n
		e := engine(t, c)
		got, err := e.Compare(context.Background(), tt.a, tt.b, tt.metric)
		want := sim.Score{tt.want, tt.aCount, tt.bCount, tt.common}
		if err != nil || got != want {
			t.Errorf("%q,%q,%d,%s: got %+v/%v want %+v", tt.a, tt.b, tt.n, tt.metric, got, err, want)
		}
	}
}

func TestSimilarityNormalizationIdentity(t *testing.T) {
	c := sim.DefaultConfig()
	c.MinGram, c.MaxGram = 1, 3
	c.Tokenizer.CaseFold = grams13.CaseFoldUnicode
	c.Tokenizer.Punctuation = grams13.PunctuationRemove
	c.Tokenizer.Whitespace = grams13.WhitespaceCollapse
	e := engine(t, c)
	score, err := e.Compare(context.Background(), "Straße!\t Σς\u00a0", "STRASSE σσ ", sim.Dice)
	if err != nil || score.Value != 1 {
		t.Fatalf("folded policies: %+v %v", score, err)
	}
	a, err := e.Prepare(context.Background(), "éStraße")
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.Prepare(context.Background(), "e\u0301STRASSE")
	if err != nil {
		t.Fatal(err)
	}
	score, err = e.ComparePrepared(context.Background(), a, b, sim.Dice)
	if err != nil || score.Value != 1 {
		t.Fatal(score, err)
	}
	hash, _ := c.Hash()
	if a.ConfigHash() != hash {
		t.Fatal("configuration identity not retained")
	}
	tokens := a.Tokens()
	tokens[0].Text = "mutated"
	if a.Tokens()[0].Text == "mutated" {
		t.Fatal("prepared set is mutable")
	}
	changed := c
	changed.Limits.MaxDPCells++
	other := engine(t, changed)
	if _, err := other.ComparePrepared(context.Background(), a, b, sim.Dice); !errors.Is(err, sim.ErrIdentity) {
		t.Fatal(err)
	}
	changed = e.Config()
	changed.MinGram = 2
	if e.Config().MinGram != 1 {
		t.Fatal("configuration is mutable")
	}
	like := sim.DefaultConfig()
	like.MinGram, like.MaxGram = 1, 1
	like.Tokenizer = grams13.DefaultLikeConfig()
	score, err = engine(t, like).Compare(context.Background(), "A\x00x", "a\x00y", sim.Dice)
	if err != nil || score.Value != 1 {
		t.Fatal(score, err)
	}
}

func randomText(r *rand.Rand, maxLen int) string {
	alphabet := []rune("abcAé界カナ가🙂🙃👩\u200d\u0301\u034f \t,\x00ßΣ")
	out := make([]rune, r.Intn(maxLen+1))
	for i := range out {
		out[i] = alphabet[r.Intn(len(alphabet))]
	}
	return string(out)
}

func TestSimilarityReferenceProperty(t *testing.T) {
	r := rand.New(rand.NewSource(51984))
	ctx := context.Background()
	configs := []sim.Config{}
	for lo := 1; lo <= 3; lo++ {
		for hi := lo; hi <= 3; hi++ {
			c := sim.DefaultConfig()
			c.MinGram, c.MaxGram = lo, hi
			configs = append(configs, c)
		}
	}
	for round := 0; round < 12000; round++ {
		c := configs[round%len(configs)]
		if round%2 == 0 {
			c.Tokenizer.CaseFold = grams13.CaseFoldUnicode
		}
		if round%3 == 0 {
			c.Tokenizer.Punctuation = grams13.PunctuationRemove
		}
		if round%5 == 0 {
			c.Tokenizer.Whitespace = grams13.WhitespaceCollapse
		}
		e := engine(t, c)
		tokenizer, _ := grams13.NewTokenizer(c.Tokenizer)
		a, b := randomText(r, 18), randomText(r, 18)
		na, err := tokenizer.Normalize(ctx, a)
		if err != nil {
			t.Fatal(err)
		}
		nb, err := tokenizer.Normalize(ctx, b)
		if err != nil {
			t.Fatal(err)
		}
		m := sim.Dice
		if round%2 != 0 {
			m = sim.Jaccard
		}
		want := referenceScore(na, nb, c.MinGram, c.MaxGram, m)
		got, err := e.Compare(ctx, a, b, m)
		if err != nil || got != want {
			t.Fatalf("round %d: %+v/%v want %+v", round, got, err, want)
		}
		reverse, err := e.Compare(ctx, b, a, m)
		if err != nil || reverse.Value != got.Value || reverse.Intersection != got.Intersection {
			t.Fatal("symmetry", round, got, reverse, err)
		}
		self, err := e.Compare(ctx, a, a, m)
		if err != nil || self.Value != 1 {
			t.Fatal("identity", round, self, err)
		}
		if got.Value < 0 || got.Value > 1 || math.IsNaN(got.Value) {
			t.Fatal("score range", got)
		}
	}
}

func TestSimilarityErrorsAndLimits(t *testing.T) {
	c := sim.DefaultConfig()
	mutations := []func(*sim.Config){
		func(c *sim.Config) { c.Version = 0 }, func(c *sim.Config) { c.MinGram = 0 }, func(c *sim.Config) { c.MaxGram = 4 }, func(c *sim.Config) { c.MinGram = 3; c.MaxGram = 2 },
		func(c *sim.Config) { c.Limits.MaxDPCells = 0 }, func(c *sim.Config) { c.Limits.MaxDPCells = 1<<30 + 1 },
		func(c *sim.Config) { c.Limits.MaxDistanceRunes = 0 }, func(c *sim.Config) { c.Limits.MaxDistanceRunes = 1<<20 + 1 },
		func(c *sim.Config) { c.Limits.MaxCandidates = 0 }, func(c *sim.Config) { c.Limits.MaxResults = 0 },
		func(c *sim.Config) { c.Limits.MaxTotalInputBytes = 0 }, func(c *sim.Config) { c.Limits.MaxFusionLists = 129 },
		func(c *sim.Config) { c.Tokenizer.DependencyVersion = "unknown" },
	}
	for _, mutate := range mutations {
		bad := c
		mutate(&bad)
		if _, err := sim.New(bad); err == nil {
			t.Fatal("accepted config", bad)
		}
	}
	e := engine(t, c)
	ctx := context.Background()
	for _, m := range []sim.Metric{"", "DICE", "bigm"} {
		if _, err := e.Compare(ctx, "a", "a", m); !errors.Is(err, sim.ErrValue) {
			t.Fatal(err)
		}
	}
	if _, err := e.Compare(nil, "a", "a", sim.Dice); !errors.Is(err, sim.ErrContext) {
		t.Fatal(err)
	}
	var zero sim.Engine
	if _, err := zero.Compare(ctx, "a", "a", sim.Dice); !errors.Is(err, sim.ErrConfig) {
		t.Fatal(err)
	}
	var nilEngine *sim.Engine
	if _, err := nilEngine.Prepare(ctx, ""); !errors.Is(err, sim.ErrConfig) {
		t.Fatal(err)
	}
	var nilSet *sim.Set
	if nilSet.Len() != 0 || nilSet.Tokens() != nil || nilSet.ConfigHash() != [32]byte{} {
		t.Fatal("nil accessors")
	}
	if _, err := e.ComparePrepared(ctx, nil, nil, sim.Dice); !errors.Is(err, sim.ErrIdentity) {
		t.Fatal(err)
	}
	if _, err := e.ComparePrepared(ctx, &sim.Set{}, &sim.Set{}, sim.Dice); !errors.Is(err, sim.ErrIdentity) {
		t.Fatal(err)
	}
	for _, input := range []string{"\xff", "ok\x00\xff"} {
		if _, err := e.Prepare(ctx, input); !errors.Is(err, grams13.ErrInvalidUTF8) {
			t.Fatal(err)
		}
	}
	if _, err := e.Prepare(ctx, "a"+strings.Repeat("\u0301", 31)); !errors.Is(err, grams13.ErrStreamSafe) {
		t.Fatal(err)
	}
	tiny := c
	tiny.Tokenizer.Limits.MaxInputBytes = 3
	if _, err := engine(t, tiny).Prepare(ctx, "abcd"); !errors.Is(err, grams13.ErrLimit) {
		t.Fatal(err)
	}
	tiny = c
	tiny.Tokenizer.Limits.MaxDistinctTokens = 1
	if _, err := engine(t, tiny).Prepare(ctx, "ab"); !errors.Is(err, grams13.ErrLimit) {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := e.Prepare(canceled, ""); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	a, _ := c.Canonical()
	b, _ := c.Canonical()
	if !reflect.DeepEqual(a, b) {
		t.Fatal("unstable canonical config")
	}
}

func TestSimilarityConcurrentImmutable(t *testing.T) {
	e := defaultEngine(t)
	ctx := context.Background()
	a, _ := e.Prepare(ctx, "数据库🙂")
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 60; i++ {
				s, err := e.ComparePrepared(ctx, a, a, sim.Jaccard)
				if err != nil || s.Value != 1 {
					t.Error(s, err)
					return
				}
				k := 2
				d, err := e.Levenshtein(ctx, "数据库🙂", "数据仓🙃", &k)
				if err != nil || d != (sim.Distance{Value: 2, Exact: true}) {
					t.Error(d, err)
					return
				}
				_, err = e.Fuse(ctx, []sim.RankedList{{Weight: 1, RowIDs: []int64{2, 1}}, {Weight: 2, RowIDs: []int64{1, 3}}}, 60, 2)
				if err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
