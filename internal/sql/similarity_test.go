package sql

import (
	"context"
	"errors"
	"math"
	"math/big"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/extension/similarity"
)

func TestSimilarityScalarGoldens(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want any
	}{
		{"ngram_similarity('ab','bc',1,'dice')", .5},
		{"ngram_similarity('abc','abd',2,'dice')", .5},
		{"ngram_similarity('abcd','abce',3,'dice')", .5},
		{"ngram_similarity('abc','abd',2,'jaccard')", 1.0 / 3},
		{"ngram_similarity('abc','abd','2.000e0','DICE')", .5},
		{"bigram_similarity('aaaa','aa')", float64(1)},
		{"bigram_similarity('a','b')", float64(1)},
		{"bigram_similarity('','')", float64(1)},
		{"bigram_similarity('ab','x')", float64(0)},
		{"bigram_similarity('AB','ab')", float64(0)},
		{"bigram_similarity('éx','éx')", float64(1)},
		{"bigram_similarity('한글','한글')", float64(1)},
		{"bigram_similarity('👩‍💻x','👩‍💻y')", 2.0 / 3},
		{"bigram_similarity(X'6162','ab')", float64(1)},
		{"bigram_similarity(1.0,'1.0')", float64(1)},
		{"bigram_similarity(123,'123')", float64(1)},
		{"levenshtein('kitten','sitting')", int64(3)},
		{"levenshtein('kitten','sitting',1)", int64(2)},
		{"levenshtein('kitten','sitting',2)", int64(3)},
		{"levenshtein('kitten','sitting',3)", int64(3)},
		{"levenshtein('é','é')", int64(0)},
		{"levenshtein('👩‍💻','👩💻')", int64(1)},
		{"levenshtein('a','b',0)", int64(1)},
		{"levenshtein('','',0)", int64(0)},
		{"levenshtein('a','b','2e0')", int64(1)},
		{"bigram_similarity(NULL,'a')", nil},
		{"bigram_similarity('a',NULL)", nil},
		{"ngram_similarity('a','b',NULL,'dice')", nil},
		{"ngram_similarity('a','b',2,NULL)", nil},
		{"ngram_similarity(NULL,'b','bad','bad')", nil},
		{"levenshtein(NULL,'b')", nil},
		{"levenshtein('a','b',NULL)", nil},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			if got := evalText(t, tt.expr, nil); got != tt.want {
				t.Fatalf("got %T %v, want %T %v", got, got, tt.want, tt.want)
			}
		})
	}
	if got := evalText(t, "bigram_similarity(?,?)", &EvalContext{Params: []any{"ab\x00cd", "ab"}}); got != .4 {
		t.Fatalf("NUL was not preserved: %v", got)
	}
}

func TestSimilarityScalarValidation(t *testing.T) {
	for _, expr := range []string{
		"ngram_similarity('a','b',2)", "bigram_similarity('a')", "levenshtein('a')", "levenshtein('a','b',1,2)",
		"ngram_similarity('a','b',0,'dice')", "ngram_similarity('a','b',4,'dice')", "ngram_similarity('a','b',2.1,'dice')",
		"ngram_similarity('a','b','2junk','dice')", "ngram_similarity('a','b','2.00000000000000000001','dice')",
		"ngram_similarity('a','b','0x2','dice')", "ngram_similarity('a','b',X'32','dice')",
		"ngram_similarity('a','b',2,'cosine')", "ngram_similarity('a','b',2,1)", "ngram_similarity('a','b',2,X'64696365')",
		"levenshtein('a','b',-1)", "levenshtein('a','b',.5)", "levenshtein('a','b','1junk')",
		"levenshtein('a','b','1.00000000000000000001')", "levenshtein('a','b','1e999999')",
		"levenshtein('a','b',9223372036854775807)", "levenshtein('a','b',1e999)",
	} {
		e, err := ParseExpr(expr)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Eval(e, nil); err == nil {
			t.Errorf("accepted %s", expr)
		}
	}
	for _, name := range []string{"ngram_similarity", "bigram_similarity", "levenshtein"} {
		if _, err := BuiltinFunctions[name](nil); err == nil {
			t.Errorf("direct builtin %s accepted invalid arity", name)
		}
	}
	if _, err := BuiltinFunctions["bigram_similarity"]([]any{true, "a"}); !errors.Is(err, similarity.ErrValue) {
		t.Fatalf("unsupported direct text type: %v", err)
	}
	if got := evalText(t, "bigram_similarity('a','b')", &EvalContext{Functions: map[string]Function{
		"bigram_similarity": func([]any) (any, error) { return int64(7), nil },
	}}); got != int64(7) {
		t.Fatal("connection override lost", got)
	}
}

func TestSimilarityIntegerExactDecimal(t *testing.T) {
	ctx := context.Background()
	max := int(^uint(0)>>1) - 1
	for _, tt := range []struct {
		value any
		want  int
	}{
		{int64(2), 2}, {float64(2), 2}, {"  +002.000e0\t", 2}, {".2e1", 2}, {"20e-1", 2},
		{"2.e0", 2}, {"0002", 2}, {"-0", 0}, {"0e999999", 0},
		{strings.Repeat("0", 3000) + "2", 2}, {"2" + strings.Repeat("0", 3000) + "e-3000", 2},
		{strconv.Itoa(max), max},
	} {
		got, err := similarityInteger(ctx, tt.value, max)
		if err != nil || got != tt.want {
			t.Fatalf("integer %.80v: %d, %v; want %d", tt.value, got, err, tt.want)
		}
	}
	for _, bad := range []any{"", ".", "+", "1e", "1e+", "2.2", "2/1", "0x2", "2_0", "0e1x", "1.2.3", "2\x00", "1e-999999", "1.00000000000000000001", "9007199254740992.1", "9223372036854775807", math.Inf(1), math.NaN(), []byte("2"), int64(-1), true} {
		if _, err := similarityInteger(ctx, bad, max); err == nil {
			t.Fatalf("accepted %#v", bad)
		}
	}
}

func TestSimilarityScalarLimitsAndCancellation(t *testing.T) {
	for _, tt := range []struct {
		expr string
		arg  any
		want error
	}{
		{"bigram_similarity(?,'a')", strings.Repeat("a", 1<<20+1), grams13.ErrLimit},
		{"bigram_similarity(?,'a')", make([]byte, 1<<20+1), grams13.ErrLimit},
		{"levenshtein('a',?)", make([]byte, 1<<20+1), grams13.ErrLimit},
		{"levenshtein(?,'a')", strings.Repeat("a", 4097), similarity.ErrLimit},
		{"bigram_similarity(?,'a')", "a" + strings.Repeat("\u0301", 31), grams13.ErrStreamSafe},
		{"ngram_similarity('a','b',?,'dice')", strings.Repeat("0", 1<<20+1), similarity.ErrLimit},
	} {
		e, err := ParseExpr(tt.expr)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Eval(e, &EvalContext{Params: []any{tt.arg}}); !errors.Is(err, tt.want) {
			t.Fatalf("%s: %v, want %v", tt.expr, err, tt.want)
		}
	}
	e, _ := ParseExpr("bigram_similarity(?,'a')")
	if _, err := Eval(e, &EvalContext{Params: []any{[]byte{0xff}}}); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, text := range []string{"bigram_similarity('ab','ab')", "ngram_similarity('ab','ab',2,'dice')", "levenshtein('a','b')"} {
		e, _ := ParseExpr(text)
		if _, err := Eval(e, &EvalContext{Context: ctx}); !errors.Is(err, context.Canceled) {
			t.Fatalf("%s did not receive actual operation context: %v", text, err)
		}
	}
}

func TestSimilarityIntegerExactDecimalReference(t *testing.T) {
	ctx := context.Background()
	maximum := int(^uint(0)>>1) - 1
	rng := rand.New(rand.NewSource(5013))
	for i := 0; i < 5000; i++ {
		digits := make([]byte, 1+rng.Intn(35))
		for j := range digits {
			digits[j] = byte('0' + rng.Intn(10))
		}
		point := rng.Intn(len(digits) + 1)
		text := string(digits[:point]) + "." + string(digits[point:]) + "e" + strconv.Itoa(rng.Intn(101)-50)
		if i%5 == 0 {
			text = "-" + text
		}
		rat, ok := new(big.Rat).SetString(text)
		if !ok {
			t.Fatal("invalid reference input", text)
		}
		valid := rat.IsInt() && rat.Sign() >= 0 && rat.Num().IsInt64() && rat.Num().Int64() <= int64(maximum)
		got, err := similarityInteger(ctx, text, maximum)
		if (err == nil) != valid || valid && int64(got) != rat.Num().Int64() {
			t.Fatalf("%q: got %d %v, rational %v, valid %v", text, got, err, rat, valid)
		}
	}
}

func TestSimilarityScalarOversizedMetric(t *testing.T) {
	// Reject length before ASCII folding, which otherwise copies the entire
	// caller-owned metric despite only two short metric names being supported.
	metric := strings.Repeat("dice", 1<<18+1)
	expr, err := ParseExpr("ngram_similarity('ab','ab',2,?)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Eval(expr, &EvalContext{Context: context.Background(), Params: []any{metric}}); !errors.Is(err, similarity.ErrValue) {
		t.Fatalf("oversized metric: %v; want invalid argument", err)
	}
}
