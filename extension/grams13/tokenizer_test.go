package grams13

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

func mustTokenizer(t testing.TB, c Config) *Tokenizer {
	t.Helper()
	x, err := NewTokenizer(c)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func TestNormalizeUnicodeGolden(t *testing.T) {
	fold := DefaultMatchConfig()
	fold.CaseFold = CaseFoldUnicode
	policies := fold
	policies.Whitespace = WhitespaceCollapse
	policies.Punctuation = PunctuationRemove
	for _, tt := range []struct {
		name        string
		config      Config
		input, want string
	}{
		{"raw", DefaultLikeConfig(), "AÉß Σς İKK e\u0301\x00Z", "aÉß Σς İKk e\u0301"},
		{"nfc", DefaultMatchConfig(), "Cafe\u0301 A\u030a e\u0301\u0327", "Café Å ȩ\u0301"},
		{"fold", fold, "Straße Σςσ İIı K K ﬃ ẞ A\u030a", "strasse σσσ i\u0307iı k k ffi ss å"},
		{"renfc_after_fold", fold, "ΐ ǰ", "ΐ ǰ"},
		{"nfc_not_nfkc", DefaultMatchConfig(), "① Ａ ﬁ", "① Ａ ﬁ"},
		{"han", DefaultMatchConfig(), "数据库漢字", "数据库漢字"},
		{"kana", DefaultMatchConfig(), "か\u3099 カ\u3099", "が ガ"},
		{"hangul", DefaultMatchConfig(), "\u1100\u1161\u11a8", "각"},
		{"hangul_raw", DefaultLikeConfig(), "\u1100\u1161\u11a8", "\u1100\u1161\u11a8"},
		{"emoji", fold, "👩🏽‍💻👨‍👩‍👧‍👦🇯🇵❤️", "👩🏽‍💻👨‍👩‍👧‍👦🇯🇵❤️"},
		{"policies", policies, "\tA，\u00a0!\nB—C\u2003\u0085D🙂$\u200b\r", " a bc d🙂$\u200b "},
		{"preserve_policies", DefaultMatchConfig(), "\tA，\u00a0!\nB—C\u2003", "\tA，\u00a0!\nB—C\u2003"},
		{"nul_raw", DefaultLikeConfig(), "A\x00B", "a"},
		{"nul_match", fold, "A\x00B", "a\x00b"},
		{"replacement_character", fold, "\ufffd", "\ufffd"},
		{"noncharacters", fold, "\ufdd0\ufffe\U0010ffff", "\ufdd0\ufffe\U0010ffff"},
		{"empty", fold, "", ""},
		{"explicit_cgj", fold, "A\u034f\u0301", "a\u034f\u0301"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			x := mustTokenizer(t, tt.config)
			got, err := x.Normalize(context.Background(), tt.input)
			if err != nil || got != tt.want {
				t.Fatalf("got %q, want %q: %v", got, tt.want, err)
			}
			again, err := x.Normalize(context.Background(), got)
			if err != nil || again != got {
				t.Fatalf("not idempotent: %q %v", again, err)
			}
		})
	}
}

func TestTokenGolden(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  []Token
	}{
		{"", []Token{}},
		{"a", []Token{{1, "a"}}},
		{"ab", []Token{{1, "a"}, {1, "b"}, {2, "ab"}}},
		{"aaaa", []Token{{1, "a"}, {2, "aa"}, {3, "aaa"}}},
		{"数据库", []Token{{1, "库"}, {1, "据"}, {1, "数"}, {2, "据库"}, {2, "数据"}, {3, "数据库"}}},
		{"👩‍💻", []Token{{1, "\u200d"}, {1, "👩"}, {1, "💻"}, {2, "\u200d💻"}, {2, "👩\u200d"}, {3, "👩‍💻"}}},
		{"か\u3099", []Token{{1, "か"}, {1, "\u3099"}, {2, "か\u3099"}}},
		{"A !", []Token{{1, " "}, {1, "!"}, {1, "a"}, {2, " !"}, {2, "a "}, {3, "a !"}}},
	} {
		x := mustTokenizer(t, DefaultLikeConfig())
		got, err := x.Tokenize(context.Background(), tt.input)
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("input %q got %#v want %#v: %v", tt.input, got, tt.want, err)
		}
	}
}

func TestInvalidUTF8IsNeverHiddenByNUL(t *testing.T) {
	for _, config := range []Config{DefaultLikeConfig(), DefaultMatchConfig()} {
		x := mustTokenizer(t, config)
		for _, s := range []string{"\xff", "ok\x00\xff", "\xc0\x80", "\xed\xa0\x80", "\xf4\x90\x80\x80", "\xe2\x82", "\x80"} {
			if got, err := x.Normalize(context.Background(), s); got != "" || !errors.Is(err, ErrInvalidUTF8) {
				t.Fatalf("Normalize(%q)=%q %v", s, got, err)
			}
			if got, err := x.Tokenize(context.Background(), s); got != nil || !errors.Is(err, ErrInvalidUTF8) {
				t.Fatalf("Tokenize(%q)=%v %v", s, got, err)
			}
			if got, err := x.Contains(context.Background(), s, ""); got || !errors.Is(err, ErrInvalidUTF8) {
				t.Fatalf("Contains(%q)=%v %v", s, got, err)
			}
		}
	}
}

func TestStreamSafeBoundariesAndExplicitCGJ(t *testing.T) {
	x := mustTokenizer(t, DefaultMatchConfig())
	for _, s := range []string{
		strings.Repeat("\u0301", 30), strings.Repeat("\u0301", 31),
		"a" + strings.Repeat("\u0301", 30), "a" + strings.Repeat("\u0301", 31),
		"á" + strings.Repeat("\u0301", 29), "á" + strings.Repeat("\u0301", 30),
		strings.Repeat("\u0344", 15), strings.Repeat("\u0344", 16),
		strings.Repeat("\u1161", 30), strings.Repeat("\u1161", 31),
		strings.Repeat("\u11a8", 30), strings.Repeat("\u11a8", 31),
		strings.Repeat("\u0301", 30) + "\u034f" + strings.Repeat("\u0301", 30),
		strings.Repeat("\u0301", 31) + "\u034f" + strings.Repeat("\u0301", 30),
		strings.Repeat("\u034f", 100),
		"\u1100\u1161" + strings.Repeat("\u0301", 31),
	} {
		want := norm.NFC.String(s)
		inserted := strings.Count(want, norm.GraphemeJoiner) > strings.Count(s, norm.GraphemeJoiner)
		got, err := x.Normalize(context.Background(), s)
		if inserted {
			if got != "" || !errors.Is(err, ErrStreamSafe) {
				t.Fatalf("not rejected %q: %q %v", s, got, err)
			}
		} else if got != want || err != nil {
			t.Fatalf("changed accepted %q: %q want %q %v", s, got, want, err)
		}
		// LIKE preserves exactly these sequences and must never reject for NFC.
		if got, err := mustTokenizer(t, DefaultLikeConfig()).Normalize(context.Background(), s); got != s || err != nil {
			t.Fatalf("raw changed: %q %v", got, err)
		}
	}
	// Removing punctuation joins runs and must not sneak CGJ into the index.
	c := DefaultMatchConfig()
	c.Punctuation = PunctuationRemove
	if _, err := mustTokenizer(t, c).Normalize(context.Background(), strings.Repeat("\u0301", 20)+"!"+strings.Repeat("\u0301", 20)); !errors.Is(err, ErrStreamSafe) {
		t.Fatalf("policy join: %v", err)
	}
}

// This exhaustive, in-memory scan establishes the installed data property used
// by CGJ-count detection. No scalar canonically decomposes or folds into CGJ,
// and CGJ has inert boundaries and no canonical decomposition of its own.
func TestUnicodeTablesCannotOriginateOrRemoveCGJ(t *testing.T) {
	p := norm.NFC.PropertiesString(norm.GraphemeJoiner)
	if p.CCC() != 0 || p.Decomposition() != nil || !p.BoundaryBefore() || !p.BoundaryAfter() {
		t.Fatal("CGJ is not inert under installed tables")
	}
	fold := cases.Fold()
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if r >= 0xd800 && r <= 0xdfff {
			continue
		}
		s := string(r)
		want := 0
		if r == '\u034f' {
			want = 1
		}
		if got := strings.Count(norm.NFD.String(s), norm.GraphemeJoiner); got != want {
			t.Fatalf("NFD U+%04X changes CGJ count: %d", r, got)
		}
		if got := strings.Count(norm.NFC.String(s), norm.GraphemeJoiner); got != want {
			t.Fatalf("NFC U+%04X changes CGJ count: %d", r, got)
		}
		if got := strings.Count(fold.String(s), norm.GraphemeJoiner); got != want {
			t.Fatalf("Fold U+%04X changes CGJ count: %d", r, got)
		}
	}
}

func TestLimitsHaveExactBoundariesAndNoPartialResults(t *testing.T) {
	for _, tt := range []struct {
		name       string
		config     func() Config
		input      string
		mutate     func(*Limits)
		normalized bool
	}{
		{"input_bytes", DefaultLikeConfig, "éé", func(l *Limits) { l.MaxInputBytes = 3 }, true},
		{"input_runes", DefaultLikeConfig, "abc", func(l *Limits) { l.MaxInputRunes = 2 }, true},
		{"input_runes_after_nul", DefaultLikeConfig, "a\x00bc", func(l *Limits) { l.MaxInputRunes = 3 }, true},
		{"output_bytes", DefaultLikeConfig, "éé", func(l *Limits) { l.MaxOutputBytes = 3 }, true},
		{"output_runes", DefaultLikeConfig, "abc", func(l *Limits) { l.MaxOutputRunes = 2 }, true},
		{"distinct_tokens", DefaultLikeConfig, "abc", func(l *Limits) { l.MaxDistinctTokens = 5 }, false},
		{"token_bytes", DefaultLikeConfig, "abc", func(l *Limits) { l.MaxTokenBytes = 9 }, false},
		{"nfc_expansion", DefaultMatchConfig, "\u0344", func(l *Limits) { l.MaxOutputBytes = 3 }, true},
		{"nfc_rune_expansion", DefaultMatchConfig, "\u0344", func(l *Limits) { l.MaxOutputRunes = 1 }, true},
		{"fold_expansion", func() Config { c := DefaultMatchConfig(); c.CaseFold = CaseFoldUnicode; return c }, "İ", func(l *Limits) { l.MaxOutputBytes = 2 }, true},
		{"fold_rune_expansion", func() Config { c := DefaultMatchConfig(); c.CaseFold = CaseFoldUnicode; return c }, "ß", func(l *Limits) { l.MaxOutputRunes = 1 }, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.config()
			tt.mutate(&c.Limits)
			x := mustTokenizer(t, c)
			got, err := x.Tokenize(context.Background(), tt.input)
			if got != nil || !errors.Is(err, ErrLimit) {
				t.Fatalf("partial result or wrong error: %v %v", got, err)
			}
			if tt.normalized {
				if got, err := x.Normalize(context.Background(), tt.input); got != "" || !errors.Is(err, ErrLimit) {
					t.Fatalf("partial normalization %q %v", got, err)
				}
			}
		})
	}
	c := DefaultLikeConfig()
	c.Limits = Limits{3, 3, 3, 3, 6, 10}
	if got, err := mustTokenizer(t, c).Tokenize(context.Background(), "abc"); len(got) != 6 || err != nil {
		t.Fatalf("exact boundaries: %v %v", got, err)
	}
	c.Limits.MaxDistinctTokens = 3
	c.Limits.MaxTokenBytes = 6
	c.Limits.MaxInputBytes = 100000
	c.Limits.MaxInputRunes = 100000
	c.Limits.MaxOutputBytes = 100000
	c.Limits.MaxOutputRunes = 100000
	if got, err := mustTokenizer(t, c).Tokenize(context.Background(), strings.Repeat("a", 100000)); len(got) != 3 || err != nil {
		t.Fatalf("repetition should count distinct grams only: %v %v", got, err)
	}
}

func TestContainsNormalizedSubstringSemantics(t *testing.T) {
	fold := DefaultMatchConfig()
	fold.CaseFold = CaseFoldUnicode
	for _, tt := range []struct {
		c           Config
		hay, needle string
		want        bool
	}{
		{fold, "Straße café", "STRASSE CAFE\u0301", true},
		{DefaultMatchConfig(), "Straße café", "STRASSE", false},
		{fold, "a\x00bc", "bc", true},
		{DefaultLikeConfig(), "a\x00bc", "bc", false},
		{fold, "anything", "", true},
		{fold, "", "", true},
		{fold, "abc", "%", false},
		{fold, "a OR b", "OR", true},
		{fold, "a b", "a AND b", false},
		{DefaultLikeConfig(), "e\u0301", "e", true},
		{DefaultLikeConfig(), "e\u0301", "\u0301", true},
		{DefaultMatchConfig(), "e\u0301", "e", false},
		{DefaultMatchConfig(), "e\u0301", "é", true},
	} {
		got, err := mustTokenizer(t, tt.c).Contains(context.Background(), tt.hay, tt.needle)
		if err != nil || got != tt.want {
			t.Fatalf("contains(%q,%q)=%v want %v: %v", tt.hay, tt.needle, got, tt.want, err)
		}
	}
}

func referenceTokens(s string) []Token {
	r := []rune(s)
	set := map[Token]bool{}
	for n := 1; n <= 3; n++ {
		for i := 0; i+n <= len(r); i++ {
			set[Token{uint8(n), string(r[i : i+n])}] = true
		}
	}
	out := make([]Token, 0, len(set))
	for token := range set {
		out = append(out, token)
	}
	slices.SortFunc(out, func(a, b Token) int {
		if a.Length < b.Length {
			return -1
		}
		if a.Length > b.Length {
			return 1
		}
		return strings.Compare(a.Text, b.Text)
	})
	return out
}

func TestAllLiteralSubstringsRetainCandidateGrams(t *testing.T) {
	x := mustTokenizer(t, DefaultLikeConfig())
	// Every four-rune word over an alphabet covering decomposition, ASCII fold,
	// non-ASCII case, emoji, and punctuation. This tests the LIKE filter proof:
	// raw ASCII fold preserves substring rune boundaries; NFC would not.
	alphabet := []rune{'A', 'a', 'e', '\u0301', 'é', 'Σ', '🙂', '!'}
	for code := 0; code < 4096; code++ {
		n := code
		word := make([]rune, 4)
		for i := range word {
			word[i] = alphabet[n%len(alphabet)]
			n /= len(alphabet)
		}
		doc := string(word)
		got, err := x.Tokenize(context.Background(), doc)
		if err != nil {
			t.Fatal(err)
		}
		set := map[Token]bool{}
		for _, tok := range got {
			set[tok] = true
		}
		for start := 0; start <= len(word); start++ {
			for end := start; end <= len(word); end++ {
				literal := string(word[start:end])
				grams, err := x.Tokenize(context.Background(), literal)
				if err != nil {
					t.Fatal(err)
				}
				for _, gram := range grams {
					if !set[gram] {
						t.Fatalf("false negative doc=%q literal=%q gram=%#v", doc, literal, gram)
					}
				}
			}
		}
	}
}

func TestGramCardinalityAndIndependentReference(t *testing.T) {
	x := mustTokenizer(t, DefaultLikeConfig())
	for n := 0; n <= 100; n++ {
		r := make([]rune, n)
		for i := range r {
			r[i] = rune(0x4e00 + i)
		}
		got, err := x.Tokenize(context.Background(), string(r))
		if err != nil {
			t.Fatal(err)
		}
		want := max(n, 0) + max(n-1, 0) + max(n-2, 0)
		if len(got) != want || !reflect.DeepEqual(got, referenceTokens(string(r))) {
			t.Fatalf("n=%d count=%d want=%d", n, len(got), want)
		}
	}
	rng := rand.New(rand.NewPCG(12, 34))
	alphabet := []rune("aAéé字が가🙂‍!? \x00")
	for iter := 0; iter < 1000; iter++ {
		r := make([]rune, rng.IntN(60))
		for i := range r {
			r[i] = alphabet[rng.IntN(len(alphabet))]
		}
		s := string(r)
		normalized, err := x.Normalize(context.Background(), s)
		if err != nil {
			t.Fatal(err)
		}
		got, err := x.Tokenize(context.Background(), s)
		if err != nil || !reflect.DeepEqual(got, referenceTokens(normalized)) {
			t.Fatalf("%q: %v %v", s, got, err)
		}
	}
}

// countedContext cancels deterministically during internal loops, avoiding
// timing-based cancellation tests that may never exercise in-flight work.
type countedContext struct {
	context.Context
	remaining atomic.Int64
}

func newCountedContext(n int64) *countedContext {
	c := &countedContext{Context: context.Background()}
	c.remaining.Store(n)
	return c
}
func (c *countedContext) Err() error {
	if c.remaining.Add(-1) < 0 {
		return context.Canceled
	}
	return nil
}

func TestCancellationAndInvalidReceivers(t *testing.T) {
	x := mustTokenizer(t, DefaultMatchConfig())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := x.Normalize(ctx, "abc"); got != "" || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
	if got, err := x.Tokenize(ctx, "abc"); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
	if got, err := x.Contains(ctx, "abc", ""); got || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
	if _, err := x.Tokenize(nil, "abc"); !errors.Is(err, ErrContext) {
		t.Fatal(err)
	}
	for _, invalid := range []*Tokenizer{nil, {}} {
		if _, err := invalid.Tokenize(context.Background(), "abc"); !errors.Is(err, ErrConfig) {
			t.Fatal(err)
		}
	}
	if got, err := x.Tokenize(newCountedContext(5), strings.Repeat("é", 10000)); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
	if got, err := normalizeNFC(newCountedContext(20), strings.Repeat("A\u030a", 100), DefaultLimits()); got != "" || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
	if got, err := foldUnicode(newCountedContext(1), strings.Repeat("Straße", 3000), DefaultLimits()); got != "" || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
	tokens := make([]Token, 10000)
	for i := range tokens {
		tokens[i] = Token{1, fmt.Sprintf("%05d", len(tokens)-i)}
	}
	if err := sortTokens(newCountedContext(5), tokens); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	copy := x.Config()
	copy.CaseFold = CaseFoldUnicode
	if x.Config().CaseFold != CaseFoldNone {
		t.Fatal("configuration mutated")
	}
}

func TestConcurrentTokenizerIsImmutable(t *testing.T) {
	c := DefaultMatchConfig()
	c.CaseFold = CaseFoldUnicode
	x := mustTokenizer(t, c)
	const input = "Straße 数据库 か\u3099 \u1100\u1161 👩🏽‍💻 e\u0301"
	want, err := x.Tokenize(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for worker := 0; worker < 32; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				got, err := x.Tokenize(context.Background(), input)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Errorf("concurrent result mismatch: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
