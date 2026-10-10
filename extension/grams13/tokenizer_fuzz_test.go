package grams13

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// referenceNormalize uses the nonstreaming library interfaces, independently
// checking insertion at each NFC stage. Production uses bounded streaming APIs.
func referenceNormalize(c Config, s string) (string, bool) {
	if c.Namespace == LikeRawV1 {
		if p := strings.IndexByte(s, 0); p >= 0 {
			s = s[:p]
		}
		return strings.Map(func(r rune) rune {
			if r >= 'A' && r <= 'Z' {
				return r + 32
			}
			return r
		}, s), true
	}
	if c.Punctuation == PunctuationRemove {
		s = strings.Map(func(r rune) rune {
			if unicode.IsPunct(r) {
				return -1
			}
			return r
		}, s)
	}
	if c.Whitespace == WhitespaceCollapse && s != "" {
		first, _ := utf8.DecodeRuneInString(s)
		last, _ := utf8.DecodeLastRuneInString(s)
		words := strings.Fields(s)
		if len(words) == 0 {
			s = " "
		} else {
			s = strings.Join(words, " ")
			if unicode.IsSpace(first) {
				s = " " + s
			}
			if unicode.IsSpace(last) {
				s += " "
			}
		}
	}
	n := norm.NFC.String(s)
	if strings.Count(n, norm.GraphemeJoiner) != strings.Count(s, norm.GraphemeJoiner) {
		return "", false
	}
	if c.CaseFold == CaseFoldUnicode {
		s = cases.Fold().String(n)
		n = norm.NFC.String(s)
		if strings.Count(n, norm.GraphemeJoiner) != strings.Count(s, norm.GraphemeJoiner) {
			return "", false
		}
	}
	return n, true
}

func FuzzTokenizerReference(f *testing.F) {
	for _, s := range []string{"", "数据库", "か\u3099", "\u1100\u1161\u11a8", "Straße İ Σς ﬃ", "👩🏽‍💻🇯🇵", "a\x00b", "a\x00\xff", strings.Repeat("\u0301", 31), "á" + strings.Repeat("\u0301", 30), "\tA，\u00a0!\nB—C\u2003", strings.Repeat("\u0301", 30) + "\u034f" + strings.Repeat("\u0344", 15)} {
		for mode := uint8(0); mode < 9; mode++ {
			f.Add(s, mode)
		}
	}
	f.Fuzz(func(t *testing.T, s string, mode uint8) {
		if len(s) > 16384 {
			return
		}
		c := DefaultMatchConfig()
		if mode%9 == 0 {
			c = DefaultLikeConfig()
		} else {
			if mode&1 != 0 {
				c.CaseFold = CaseFoldUnicode
			}
			if mode&2 != 0 {
				c.Whitespace = WhitespaceCollapse
			}
			if mode&4 != 0 {
				c.Punctuation = PunctuationRemove
			}
		}
		c.Limits = Limits{16384, 65536, 16384, 65536, 65536, 1 << 20}
		x := mustTokenizer(t, c)
		got, err := x.Tokenize(context.Background(), s)
		if !utf8.ValidString(s) {
			if !errors.Is(err, ErrInvalidUTF8) || got != nil {
				t.Fatalf("invalid input accepted: %q %v %v", s, got, err)
			}
			return
		}
		want, ok := referenceNormalize(c, s)
		if !ok {
			if !errors.Is(err, ErrStreamSafe) || got != nil {
				t.Fatalf("CGJ insertion accepted: %q %v %v", s, got, err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, referenceTokens(want)) {
			t.Fatalf("tokens differ: input=%q normalized=%q got=%#v want=%#v", s, want, got, referenceTokens(want))
		}
		normalized, err := x.Normalize(context.Background(), s)
		if err != nil || normalized != want {
			t.Fatalf("Normalize(%q)=%q want=%q: %v", s, normalized, want, err)
		}
		if c.Namespace == MatchConfigV1 && !norm.NFC.IsNormalString(normalized) {
			t.Fatal("output is not NFC")
		}
		for _, tok := range got {
			if !utf8.ValidString(tok.Text) || utf8.RuneCountInString(tok.Text) != int(tok.Length) || !strings.Contains(normalized, tok.Text) {
				t.Fatalf("malformed token: %#v", tok)
			}
		}
	})
}

func FuzzLikeLiteralNoFalseNegatives(f *testing.F) {
	for _, s := range []string{"e\u0301", "AéΣ🙂", "a\x00b", "\x00\xff", "か\u3099", "👩‍💻", "A! B"} {
		f.Add(s, uint32(1), uint32(3))
	}
	f.Fuzz(func(t *testing.T, s string, a, b uint32) {
		if len(s) > 4096 || !utf8.ValidString(s) {
			return
		}
		x := mustTokenizer(t, DefaultLikeConfig())
		normalized, _ := referenceNormalize(DefaultLikeConfig(), s)
		r := []rune(normalized)
		start, end := int(a%uint32(len(r)+1)), int(b%uint32(len(r)+1))
		if start > end {
			start, end = end, start
		}
		literal := string(r[start:end])
		docGrams, err := x.Tokenize(context.Background(), s)
		if err != nil {
			t.Fatal(err)
		}
		needleGrams, err := x.Tokenize(context.Background(), literal)
		if err != nil {
			t.Fatal(err)
		}
		set := map[Token]bool{}
		for _, gram := range docGrams {
			set[gram] = true
		}
		for _, gram := range needleGrams {
			if !set[gram] {
				t.Fatalf("false negative doc=%q needle=%q gram=%#v", s, literal, gram)
			}
		}
	})
}

func FuzzParseConfig(f *testing.F) {
	for _, c := range []Config{DefaultLikeConfig(), DefaultMatchConfig()} {
		b, _ := c.Canonical()
		f.Add(b)
	}
	for _, s := range []string{"", "null", "{}", `{"version":1,"version":2}`, strings.Repeat("{", 5000)} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		c, err := ParseConfig(b)
		if err != nil {
			if !errors.Is(err, ErrConfig) {
				t.Fatal(err)
			}
			return
		}
		canonical, err := c.Canonical()
		if err != nil || string(canonical) != string(b) {
			t.Fatalf("accepted noncanonical input: %q", b)
		}
		if err := c.Validate(); err != nil {
			t.Fatal(err)
		}
	})
}
