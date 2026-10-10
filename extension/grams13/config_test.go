package grams13

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

func TestConfigCanonicalVersionsAndHash(t *testing.T) {
	for _, c := range []Config{DefaultLikeConfig(), DefaultMatchConfig()} {
		b, err := c.Canonical()
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := ParseConfig(b)
		if err != nil || parsed != c {
			t.Fatalf("roundtrip: %#v %v", parsed, err)
		}
		h, err := c.Hash()
		if err != nil || h != sha256.Sum256(b) {
			t.Fatalf("hash: %x %v", h, err)
		}
		if !bytes.HasPrefix(b, []byte(`{"version":1,"namespace":`)) {
			t.Fatalf("noncanonical prefix: %s", b)
		}
		if c.NormalizationUnicodeVersion != norm.Version || c.CaseFoldUnicodeVersion != cases.UnicodeVersion || c.CategoryUnicodeVersion != unicode.Version || c.DependencyVersion != "golang.org/x/text@v0.43.0" {
			t.Fatalf("wrong actual versions: %#v", c)
		}
		// This gate intentionally makes a Unicode data upgrade require an
		// explicit policy/test/metadata compatibility review.
		if norm.Version != "17.0.0" || cases.UnicodeVersion != "17.0.0" || unicode.Version != "17.0.0" {
			t.Fatalf("unreviewed Unicode tables: norm=%s cases=%s categories=%s", norm.Version, cases.UnicodeVersion, unicode.Version)
		}
	}
	a, _ := DefaultLikeConfig().Hash()
	b, _ := DefaultMatchConfig().Hash()
	if a == b {
		t.Fatal("namespace collision")
	}
	base := DefaultMatchConfig()
	seen := map[[32]byte]bool{b: true}
	for _, alter := range []func(*Config){
		func(c *Config) { c.CaseFold = CaseFoldUnicode },
		func(c *Config) { c.Whitespace = WhitespaceCollapse },
		func(c *Config) { c.Punctuation = PunctuationRemove },
		func(c *Config) { c.Limits.MaxInputBytes-- },
		func(c *Config) { c.Limits.MaxOutputBytes-- },
		func(c *Config) { c.Limits.MaxInputRunes-- },
		func(c *Config) { c.Limits.MaxOutputRunes-- },
		func(c *Config) { c.Limits.MaxDistinctTokens-- },
		func(c *Config) { c.Limits.MaxTokenBytes-- },
	} {
		c := base
		alter(&c)
		h, err := c.Hash()
		if err != nil || seen[h] {
			t.Fatalf("configuration identity collision or error: %v", err)
		}
		seen[h] = true
	}
}

func TestConfigStrictRejection(t *testing.T) {
	for _, base := range []Config{DefaultLikeConfig(), DefaultMatchConfig()} {
		for _, alter := range []func(*Config){
			func(c *Config) { c.Version++ },
			func(c *Config) { c.Namespace = "unknown" },
			func(c *Config) { c.Normalization = "NFKC" },
			func(c *Config) { c.CaseFold = "lowercase" },
			func(c *Config) { c.Whitespace = "unknown" },
			func(c *Config) { c.Punctuation = "unknown" },
			func(c *Config) { c.NUL = "unknown" },
			func(c *Config) { c.StreamSafe = "insert" },
			func(c *Config) { c.NormalizationUnicodeVersion = "15.0.0" },
			func(c *Config) { c.CaseFoldUnicodeVersion = "15.0.0" },
			func(c *Config) { c.CategoryUnicodeVersion = "15.0.0" },
			func(c *Config) { c.DependencyVersion = "unknown" },
			func(c *Config) { c.Limits.MaxInputBytes = 0 },
			func(c *Config) { c.Limits.MaxOutputBytes = -1 },
			func(c *Config) { c.Limits.MaxInputRunes = 1<<20 + 1 },
			func(c *Config) { c.Limits.MaxOutputRunes = 0 },
			func(c *Config) { c.Limits.MaxDistinctTokens = 1<<20 + 1 },
			func(c *Config) { c.Limits.MaxTokenBytes = 12<<20 + 1 },
		} {
			c := base
			alter(&c)
			if err := c.Validate(); !errors.Is(err, ErrConfig) {
				t.Fatalf("accepted invalid config %#v: %v", c, err)
			}
			if _, err := NewTokenizer(c); !errors.Is(err, ErrConfig) {
				t.Fatal(err)
			}
			if _, err := c.Canonical(); !errors.Is(err, ErrConfig) {
				t.Fatal(err)
			}
			if _, err := c.Hash(); !errors.Is(err, ErrConfig) {
				t.Fatal(err)
			}
		}
	}
	if err := (Config{}).Validate(); !errors.Is(err, ErrConfig) {
		t.Fatal(err)
	}
	for _, alter := range []func(*Config){
		func(c *Config) { c.Normalization = NormalizationNFC },
		func(c *Config) { c.CaseFold = CaseFoldUnicode },
		func(c *Config) { c.CaseFold = CaseFoldNone },
		func(c *Config) { c.Whitespace = WhitespaceCollapse },
		func(c *Config) { c.Punctuation = PunctuationRemove },
		func(c *Config) { c.NUL = NULPreserve },
		func(c *Config) { c.StreamSafe = StreamSafeReject },
	} {
		c := DefaultLikeConfig()
		alter(&c)
		if err := c.Validate(); !errors.Is(err, ErrConfig) {
			t.Fatalf("LIKE silently changed: %#v", c)
		}
	}
	c := DefaultMatchConfig()
	c.CaseFold = CaseFoldASCII
	if err := c.Validate(); !errors.Is(err, ErrConfig) {
		t.Fatal("ASCII match policy accepted")
	}
}

func TestParseConfigRejectsAmbiguousMetadata(t *testing.T) {
	canonical, _ := DefaultMatchConfig().Canonical()
	s := string(canonical)
	for _, bad := range []string{
		"", "null", "{}", "[]", "true", "\xff", strings.Repeat(" ", 4097),
		" " + s, s + "\n", s + "{}", s[:len(s)-1],
		strings.Replace(s, `"version":1`, `"version":1,"version":1`, 1),
		strings.Replace(s, `"version":1,`, "", 1),
		strings.Replace(s, `"version":1`, `"version":1,"unknown":0`, 1),
		strings.Replace(s, `"version":1`, `"Version":1`, 1),
		strings.Replace(s, `"version":1`, `"version":1.0`, 1),
		strings.Replace(s, `"version":1`, `"version":null`, 1),
		strings.Replace(s, `"namespace":"MATCH_CONFIG_V1"`, `"namespace":"MATCH_CONFIG_V2"`, 1),
		strings.Replace(s, `"max_input_bytes":1048576`, `"max_input_bytes":1048576,"max_input_bytes":1048576`, 1),
	} {
		if _, err := ParseConfig([]byte(bad)); !errors.Is(err, ErrConfig) {
			t.Fatalf("accepted malformed config %q: %v", bad, err)
		}
	}
}

// This independently SHA-256-checked fixture locks the v1 on-disk identity.
func TestCanonicalConfigGolden(t *testing.T) {
	const want = `{"version":1,"namespace":"LIKE_RAW_V1","normalization":"none","case_fold":"ascii-A-Z","whitespace":"preserve","punctuation":"preserve","nul":"first-NUL","stream_safe":"not-applicable","normalization_unicode_version":"17.0.0","case_fold_unicode_version":"17.0.0","category_unicode_version":"17.0.0","dependency_version":"golang.org/x/text@v0.43.0","limits":{"max_input_bytes":1048576,"max_output_bytes":1048576,"max_input_runes":262144,"max_output_runes":262144,"max_distinct_tokens":262144,"max_token_bytes":2097152}}`
	const wantHash = "e2a0ef9d44a3f70b1fb47ff23735f6d90660c72d69336f6743c135f1b1a147a6"
	c := DefaultLikeConfig()
	got, err := c.Canonical()
	if err != nil || string(got) != want {
		t.Fatalf("canonical metadata changed: %s %v", got, err)
	}
	hash, err := c.Hash()
	if err != nil || fmt.Sprintf("%x", hash) != wantHash {
		t.Fatalf("config hash changed: %x %v", hash, err)
	}
}
