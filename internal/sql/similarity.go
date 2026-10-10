package sql

import (
	"context"
	"fmt"
	"math"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/extension/similarity"
)

func isSimilarityFunction(name string) bool {
	return name == "ngram_similarity" || name == "bigram_similarity" || name == "levenshtein"
}

// similarityFunction applies SQL NULL/coercion rules before entering the bounded
// mathematical engine. Default MATCH text is case-sensitive NFC, preserving
// punctuation, whitespace and NUL. These functions are not pg_bigm-compatible.
func similarityFunction(ctx context.Context, name string, args []any) (any, error) {
	if err := ValidateFunction(name, len(args)); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background() // Legacy pure-expression callers have no operation.
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, arg := range args {
		if arg == nil {
			return nil, nil
		}
	}
	config := similarity.DefaultConfig()
	left, err := similarityText(args[0], config.Tokenizer.Limits.MaxInputBytes)
	if err != nil {
		return nil, err
	}
	right, err := similarityText(args[1], config.Tokenizer.Limits.MaxInputBytes)
	if err != nil {
		return nil, err
	}
	metric := similarity.Dice
	var maximum *int
	switch name {
	case "ngram_similarity":
		n, err := similarityInteger(ctx, args[2], 3)
		if err != nil {
			return nil, fmt.Errorf("ngram_similarity n must be an integer in [1,3]: %w", err)
		}
		if n == 0 {
			return nil, fmt.Errorf("%w: ngram_similarity n must be an integer in [1,3]", similarity.ErrValue)
		}
		config.MinGram, config.MaxGram = n, n
		text, ok := args[3].(string)
		if !ok || len(text) != len("dice") && len(text) != len("jaccard") {
			return nil, fmt.Errorf("%w: ngram_similarity metric must be text dice or jaccard", similarity.ErrValue)
		}
		metric = similarity.Metric(FoldName(text))
		if metric != similarity.Dice && metric != similarity.Jaccard {
			return nil, fmt.Errorf("%w: ngram_similarity metric must be dice or jaccard", similarity.ErrValue)
		}
	case "levenshtein":
		if len(args) == 3 {
			n, err := similarityInteger(ctx, args[2], int(^uint(0)>>1)-1)
			if err != nil {
				return nil, fmt.Errorf("levenshtein maximum must be a nonnegative integer below max int: %w", err)
			}
			maximum = &n
		}
	}
	engine, err := similarity.New(config)
	if err != nil {
		return nil, err
	}
	if name == "levenshtein" {
		distance, err := engine.Levenshtein(ctx, left, right, maximum)
		if err != nil {
			return nil, err
		}
		return int64(distance.Value), nil
	}
	score, err := engine.Compare(ctx, left, right, metric)
	if err != nil {
		return nil, err
	}
	return score.Value, nil
}

// SQL evaluation has already normalized its values. Keep direct builtin calls
// equally strict rather than silently mapping an unsupported Go value to "".
func similarityText(value any, maxBytes int) (string, error) {
	switch v := value.(type) {
	case string:
		if len(v) > maxBytes {
			return "", fmt.Errorf("%w: similarity input bytes", grams13.ErrLimit)
		}
	case []byte:
		// Preflight before LikeText copies the caller's blob into a string.
		if len(v) > maxBytes {
			return "", fmt.Errorf("%w: similarity input bytes", grams13.ErrLimit)
		}
	case int64, float64:
	default:
		return "", fmt.Errorf("%w: similarity text cannot coerce %T", similarity.ErrValue, value)
	}
	return LikeText(value)
}

// similarityInteger accepts whole SQL numbers and full decimal numeric text.
// Unlike arithmetic coercion, it never accepts a prefix or truncates a fraction.
// Text uses exact decimal arithmetic, so fractional digits cannot disappear
// through float64 rounding. Blobs are not numeric control arguments.
func similarityInteger(ctx context.Context, value any, maximum int) (int, error) {
	switch v := value.(type) {
	case int64:
		if v >= 0 && uint64(v) <= uint64(maximum) {
			return int(v), nil
		}
	case float64:
		if !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v < 9223372036854775808.0 && math.Trunc(v) == v && uint64(v) <= uint64(maximum) {
			return int(v), nil
		}
	case string:
		return similarityDecimalInteger(ctx, v, maximum)
	}
	return 0, similarity.ErrValue
}

func similarityDecimalInteger(ctx context.Context, raw string, maximum int) (int, error) {
	// This control argument gets the same input-byte budget as MATCH text.
	if len(raw) > similarity.DefaultConfig().Tokenizer.Limits.MaxInputBytes {
		return 0, fmt.Errorf("%w: integer argument bytes", similarity.ErrLimit)
	}
	s := trimSpace(raw)
	i, negative := 0, false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		negative = s[i] == '-'
		i++
	}
	var prefix uint64
	digits, significant, fraction, trailing := 0, 0, 0, 0
	dot := false
	for i < len(s) && s[i] != 'e' && s[i] != 'E' {
		if i&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
		}
		c := s[i]
		i++
		if c == '.' && !dot {
			dot = true
			continue
		}
		if !digit(c) {
			return 0, similarity.ErrValue
		}
		digits++
		if dot {
			fraction++
		}
		if c != '0' || significant != 0 {
			significant++
			if significant <= 19 {
				prefix = prefix*10 + uint64(c-'0')
			}
		}
		if c == '0' {
			trailing++
		} else {
			trailing = 0
		}
	}
	if digits == 0 {
		return 0, similarity.ErrValue
	}
	exponent := 0
	if i < len(s) {
		i++
		negExp := false
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			negExp = s[i] == '-'
			i++
		}
		start := i
		for ; i < len(s); i++ {
			if i&1023 == 0 {
				if err := ctx.Err(); err != nil {
					return 0, err
				}
			}
			if !digit(s[i]) {
				return 0, similarity.ErrValue
			}
			// Saturate beyond every possible decimal point/64-bit boundary.
			exponent = min(len(s)+64, exponent*10+int(s[i]-'0'))
		}
		if start == i {
			return 0, similarity.ErrValue
		}
		if negExp {
			exponent = -exponent
		}
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if significant == 0 {
		return 0, nil
	}
	places := exponent - fraction
	wholeDigits := significant + places
	if negative || wholeDigits < 1 || wholeDigits > 19 || places < -trailing {
		return 0, similarity.ErrValue
	}
	for count := min(significant, 19); count > wholeDigits; count-- {
		prefix /= 10
	}
	for count := significant; count < wholeDigits; count++ {
		prefix *= 10
	}
	if prefix > uint64(maximum) {
		return 0, similarity.ErrValue
	}
	return int(prefix), nil
}
