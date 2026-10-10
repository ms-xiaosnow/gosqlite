package grams13

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Token is a distinct contiguous gram; Length counts runes, not bytes. Tokens
// have no positions or frequencies. The config namespace is intentionally not
// implicit in Text; consumers must retain the index/config identity separately.
type Token struct {
	Length uint8
	Text   string
}

// Tokenizer is immutable and safe for concurrent use. Per-call working state is
// local. Construct one with NewTokenizer; its zero value is invalid.
type Tokenizer struct {
	config Config
	valid  bool
}

func NewTokenizer(c Config) (*Tokenizer, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &Tokenizer{config: c, valid: true}, nil
}

// Config returns a value copy. Mutating it cannot change this tokenizer.
func (t *Tokenizer) Config() Config {
	if t == nil {
		return Config{}
	}
	return t.config
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return ErrContext
	}
	return ctx.Err()
}

func limitError(name string) error { return fmt.Errorf("%w: %s", ErrLimit, name) }

func validateInput(ctx context.Context, s string, limits Limits) error {
	if len(s) > limits.MaxInputBytes {
		return limitError("input bytes")
	}
	n := 0
	for p := 0; p < len(s); {
		if n&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		r, size := utf8.DecodeRuneInString(s[p:])
		if r == utf8.RuneError && size == 1 {
			return ErrInvalidUTF8
		}
		p += size
		n++
		if n > limits.MaxInputRunes {
			return limitError("input runes")
		}
	}
	return ctx.Err()
}

// textBuffer enforces limits before appending, including for intermediate
// normalization/casefold expansions. Its input must consist of complete UTF-8
// runes. A failed append leaves no externally visible partial output.
type textBuffer struct {
	b      strings.Builder
	runes  int
	limits Limits
}

func (b *textBuffer) append(s string) error {
	if len(s) > b.limits.MaxOutputBytes-b.b.Len() {
		return limitError("output bytes")
	}
	n := utf8.RuneCountInString(s)
	if n > b.limits.MaxOutputRunes-b.runes {
		return limitError("output runes")
	}
	b.runes += n
	b.b.WriteString(s)
	return nil
}

func (b *textBuffer) appendRune(r rune) error {
	if utf8.RuneLen(r) > b.limits.MaxOutputBytes-b.b.Len() {
		return limitError("output bytes")
	}
	if b.runes == b.limits.MaxOutputRunes {
		return limitError("output runes")
	}
	b.runes++
	b.b.WriteRune(r)
	return nil
}

// Normalize returns the namespace's comparison text, never partial output. For
// LIKE the entire input is validated before NUL truncation. For MATCH, explicit
// CGJ is preserved, but inputs for which x/text NFC would insert CGJ are rejected
// rather than silently changing substring semantics (StreamSafeReject policy).
func (t *Tokenizer) Normalize(ctx context.Context, input string) (string, error) {
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	if t == nil || !t.valid {
		return "", ErrConfig
	}
	c := t.config
	if err := validateInput(ctx, input, c.Limits); err != nil {
		return "", err
	}
	out := textBuffer{limits: c.Limits}
	if c.Namespace == LikeRawV1 {
		n := 0
		for _, r := range input {
			if n&1023 == 0 {
				if err := ctx.Err(); err != nil {
					return "", err
				}
			}
			n++
			if r == 0 {
				break
			}
			if r >= 'A' && r <= 'Z' {
				r += 'a' - 'A'
			}
			if err := out.appendRune(r); err != nil {
				return "", err
			}
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		return out.b.String(), nil
	}
	// Apply removal first, so punctuation cannot keep two otherwise adjacent
	// whitespace runs from collapsing. Other symbols/marks remain significant.
	inSpace := false
	n := 0
	for _, r := range input {
		if n&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return "", err
			}
		}
		n++
		if c.Punctuation == PunctuationRemove && unicode.IsPunct(r) {
			continue
		}
		if c.Whitespace == WhitespaceCollapse {
			if unicode.IsSpace(r) {
				if inSpace {
					continue
				}
				r, inSpace = ' ', true
			} else {
				inSpace = false
			}
		}
		if err := out.appendRune(r); err != nil {
			return "", err
		}
	}
	text, err := normalizeNFC(ctx, out.b.String(), c.Limits)
	if err != nil {
		return "", err
	}
	if c.CaseFold == CaseFoldUnicode {
		text, err = foldUnicode(ctx, text, c.Limits)
		if err != nil {
			return "", err
		}
		text, err = normalizeNFC(ctx, text, c.Limits)
		if err != nil {
			return "", err
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return text, nil
}

// normalizeNFC detects insertion by counting CGJ before and after the actual
// x/text normalization algorithm. This is exact because canonical Unicode
// decomposition/composition neither introduces nor removes U+034F, and CGJ
// itself is an inert canonical starter. These properties are exhaustively
// checked against the pinned Unicode tables in tests.
//
// Do not substitute a CCC!=0 run counter: x/text's private streamSafe.next uses
// decomposition leading/trailing nonstarter counts and treats Hangul Jamo V/T
// specially. norm.Iter executes that exact policy with bounded internal storage.
// References in the pinned dependency: unicode/norm/composition.go streamSafe,
// forminfo.go nLeadingNonStarters/nTrailingNonStarters, and iter.go.
func normalizeNFC(ctx context.Context, s string, limits Limits) (string, error) {
	originalCGJ := 0
	n := 0
	for _, r := range s {
		if n&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return "", err
			}
		}
		n++
		if r == '\u034f' {
			originalCGJ++
		}
	}
	var it norm.Iter
	it.InitString(norm.NFC, s)
	out := textBuffer{limits: limits}
	resultCGJ := 0
	for !it.Done() {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		segment := string(it.Next())
		resultCGJ += strings.Count(segment, norm.GraphemeJoiner)
		if resultCGJ > originalCGJ {
			return "", ErrStreamSafe
		}
		if err := out.append(segment); err != nil {
			return "", err
		}
	}
	if resultCGJ != originalCGJ {
		return "", fmt.Errorf("%w: unexpected CGJ removal", ErrStreamSafe)
	}
	return out.b.String(), nil
}

func foldUnicode(ctx context.Context, s string, limits Limits) (string, error) {
	// Full default folding is locale independent and context independent. Use
	// its streaming API so expansion is checked before allocating an unbounded
	// result. The source itself was already bounded by output limits.
	fold := cases.Fold()
	src := []byte(s)
	var dst [4096]byte
	out := textBuffer{limits: limits}
	for len(src) != 0 {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		nDst, nSrc, err := fold.Transform(dst[:], src, true)
		if nDst != 0 {
			if appendErr := out.append(string(dst[:nDst])); appendErr != nil {
				return "", appendErr
			}
		}
		src = src[nSrc:]
		if err != nil && err != transform.ErrShortDst {
			return "", fmt.Errorf("grams13: Unicode fold: %w", err)
		}
		if nDst == 0 && nSrc == 0 {
			return "", fmt.Errorf("grams13: Unicode fold made no progress")
		}
	}
	return out.b.String(), nil
}

// Tokenize emits all distinct contiguous 1-, 2-, and 3-rune grams, sorted by
// Length then UTF-8 Text bytes. Repeated grams occur once. All limits and
// cancellation failures return nil rather than incomplete indexable data.
func (t *Tokenizer) Tokenize(ctx context.Context, input string) ([]Token, error) {
	s, err := t.Normalize(ctx, input)
	if err != nil {
		return nil, err
	}
	limits := t.config.Limits
	// Rune byte offsets avoid confusing UTF-8 bytes with the public gram unit.
	offsets := make([]int, 0, min(len(s), limits.MaxOutputRunes)+1)
	for p := range s {
		if len(offsets)&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		offsets = append(offsets, p)
	}
	offsets = append(offsets, len(s))
	seen := make(map[Token]struct{})
	tokens := make([]Token, 0)
	bytes := 0
	for length := 1; length <= 3; length++ {
		for start := 0; start+length < len(offsets); start++ {
			if start&1023 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			token := Token{uint8(length), s[offsets[start]:offsets[start+length]]}
			if _, exists := seen[token]; exists {
				continue
			}
			if len(tokens) == limits.MaxDistinctTokens {
				return nil, limitError("distinct tokens")
			}
			if len(token.Text) > limits.MaxTokenBytes-bytes {
				return nil, limitError("token bytes")
			}
			bytes += len(token.Text)
			seen[token] = struct{}{}
			tokens = append(tokens, token)
		}
	}
	if err := sortTokens(ctx, tokens); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return tokens, nil
}

func tokenLess(a, b Token) bool {
	return a.Length < b.Length || (a.Length == b.Length && a.Text < b.Text)
}

// Bottom-up mergesort supports bounded cancellation latency during sorting too,
// without comparator panics or exposing a partly sorted token set.
func sortTokens(ctx context.Context, tokens []Token) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(tokens) < 2 {
		return nil
	}
	scratch := make([]Token, len(tokens))
	src, dst := tokens, scratch
	checks := 0
	for width := 1; width < len(tokens); width *= 2 {
		for start := 0; start < len(tokens); start += 2 * width {
			mid, end := min(start+width, len(tokens)), min(start+2*width, len(tokens))
			a, b := start, mid
			for out := start; out < end; out++ {
				if checks&1023 == 0 {
					if err := ctx.Err(); err != nil {
						return err
					}
				}
				checks++
				if b == end || (a < mid && tokenLess(src[a], src[b])) {
					dst[out] = src[a]
					a++
				} else {
					dst[out] = src[b]
					b++
				}
			}
		}
		src, dst = dst, src
	}
	// A final bounded copy is needed when the last pass ended in scratch.
	if &src[0] != &tokens[0] {
		for start := 0; start < len(tokens); start += 1024 {
			if err := ctx.Err(); err != nil {
				return err
			}
			copy(tokens[start:min(start+1024, len(tokens))], src[start:min(start+1024, len(tokens))])
		}
	}
	return ctx.Err()
}

// Contains compares normalized substring containment, not SQL LIKE and not an
// FTS language. SQL MATCH may use this as an exact recheck after gram candidates.
// Any candidate filter must admit all rows for an empty normalized needle.
func (t *Tokenizer) Contains(ctx context.Context, haystack, needle string) (bool, error) {
	h, err := t.Normalize(ctx, haystack)
	if err != nil {
		return false, err
	}
	n, err := t.Normalize(ctx, needle)
	if err != nil {
		return false, err
	}
	result := strings.Contains(h, n)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return result, nil
}
