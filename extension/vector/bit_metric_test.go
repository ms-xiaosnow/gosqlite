package vector_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// bitReferenceDistances deliberately compares one textual bit at a time. It
// does not use packed bytes, bitwise operations, or production popcount logic.
func bitReferenceDistances(a, b string) (hamming, jaccard float64) {
	intersection, union := 0, 0
	for i := range a {
		if a[i] != b[i] {
			hamming++
		}
		if a[i] == '1' && b[i] == '1' {
			intersection++
		}
		if a[i] == '1' || b[i] == '1' {
			union++
		}
	}
	if intersection == 0 {
		return hamming, 1
	}
	return hamming, 1 - float64(intersection)/float64(union)
}

func TestBitMetricsAllSmallPairs(t *testing.T) {
	for width := 0; width <= 8; width++ {
		values := make([]vector.BitVector, 1<<uint(width))
		texts := make([]string, len(values))
		for n := range values {
			text := make([]byte, width)
			remaining := n
			for i := width - 1; i >= 0; i-- {
				text[i] = byte('0' + remaining%2)
				remaining /= 2
			}
			texts[n] = string(text)
			values[n] = mustBit(t, texts[n])
		}
		for i, a := range values {
			for j, b := range values {
				h, jc := bitReferenceDistances(texts[i], texts[j])
				for _, tc := range []struct {
					metric vector.BitMetricKind
					want   float64
				}{{vector.Hamming, h}, {vector.Jaccard, jc}} {
					got, err := tc.metric.Distance(a, b)
					if err != nil || got != tc.want {
						t.Fatalf("width=%d pair=%d,%d metric=%s: got %g want %g err=%v", width, i, j, tc.metric, got, tc.want, err)
					}
				}
			}
		}
	}
}

func TestBitMetricsWidthsAndPublishedSemantics(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		h, j float64
	}{
		{"", "", 0, 1},
		{"000", "000", 0, 1},
		{"111", "111", 0, 0},
		// These values reflect the two binary64 operations in 1 - a/b,
		// rather than rounding the exact rational difference only once.
		{"111", "110", 1, 0.33333333333333337},
		{"111", "100", 2, 0.6666666666666667},
		{"111", "000", 3, 1},
		{"1111", "1110", 1, 0.25},
		{"1111", "1100", 2, 0.5},
		{"1111", "1000", 3, 0.75},
		{"1100", "1000", 1, 0.5},
		{strings.Repeat("10", 10), strings.Repeat("01", 10), 20, 1},
		{strings.Repeat("10", 256) + "1", strings.Repeat("01", 256) + "0", 513, 1},
		{"11" + strings.Repeat("0", 509) + "11", "10" + strings.Repeat("0", 509) + "01", 2, 0.5},
	} {
		a, b := mustBit(t, tc.a), mustBit(t, tc.b)
		if got, err := vector.HammingDistance(a, b); err != nil || got != tc.h {
			t.Fatal("published Hamming", len(tc.a), got, tc.h, err)
		}
		if got, err := vector.JaccardDistance(a, b); err != nil || got != tc.j {
			t.Fatal("published Jaccard", len(tc.a), got, tc.j, err)
		}
	}
	for _, width := range []int{7, 8, 9, 63, 64, 65, 511, 512, 513, 2047, 2048, 2049, vector.MaxBitDimensions - 1, vector.MaxBitDimensions} {
		aText, bText := make([]byte, width), make([]byte, width)
		for i := range aText {
			aText[i], bText[i] = '0', '0'
			if i%3 == 0 || i == width-1 {
				aText[i] = '1'
			}
			if i%5 == 0 || i == width-1 {
				bText[i] = '1'
			}
		}
		a, b := mustBit(t, string(aText)), mustBit(t, string(bText))
		h, j := bitReferenceDistances(string(aText), string(bText))
		for _, tc := range []struct {
			metric vector.BitMetricKind
			want   float64
		}{{vector.Hamming, h}, {vector.Jaccard, j}} {
			for _, pair := range [][2]vector.BitVector{{a, b}, {b, a}} {
				got, err := tc.metric.DistanceContext(context.Background(), pair[0], pair[1])
				if err != nil || got != tc.want {
					t.Fatal("width/symmetry", width, tc.metric, got, tc.want, err)
				}
			}
		}
	}
	zero := mustBit(t, strings.Repeat("0", vector.MaxBitDimensions))
	ones := mustBit(t, strings.Repeat("1", vector.MaxBitDimensions))
	if got, err := vector.HammingDistance(zero, ones); err != nil || got != vector.MaxBitDimensions {
		t.Fatal("maximum Hamming", got, err)
	}
	if got, err := vector.JaccardDistance(zero, zero); err != nil || got != 1 {
		t.Fatal("maximum empty intersection", got, err)
	}
}

func TestBitMetricDomainsAndContexts(t *testing.T) {
	a, b, empty := mustBit(t, "1"), mustBit(t, "11"), mustBit(t, "")
	for _, metric := range []vector.BitMetricKind{vector.Hamming, vector.Jaccard} {
		if metric.Name() != string(metric) {
			t.Fatal("metric name", metric)
		}
		for _, pair := range [][2]vector.BitVector{{a, b}, {b, a}, {empty, a}, {a, empty}} {
			if got, err := metric.Distance(pair[0], pair[1]); !errors.Is(err, vector.ErrDimensions) || got != 0 {
				t.Fatal("mismatched dimensions", metric, got, err)
			}
		}
		for _, pair := range [][2]vector.BitVector{{{}, a}, {a, {}}, {{}, {}}, {{}, empty}, {empty, {}}} {
			if got, err := metric.Distance(pair[0], pair[1]); !errors.Is(err, vector.ErrInvalid) || got != 0 {
				t.Fatal("invalid value", metric, got, err)
			}
		}
		if got, err := metric.DistanceContext(nil, a, a); !errors.Is(err, vector.ErrContext) || got != 0 {
			t.Fatal("nil context", metric, got, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if got, err := metric.DistanceContext(ctx, empty, empty); !errors.Is(err, context.Canceled) || got != 0 {
			t.Fatal("canceled empty pair", metric, got, err)
		}
		ctx, cancel = context.WithDeadline(context.Background(), time.Unix(0, 0))
		if got, err := metric.DistanceContext(ctx, a, a); !errors.Is(err, context.DeadlineExceeded) || got != 0 {
			t.Fatal("expired context", metric, got, err)
		}
		cancel()
		// Typed signatures prevent a bit metric from entering dense search.
		if _, ok := any(metric).(vector.Metric); ok {
			t.Fatal("bit metric implements dense Metric")
		}
	}
	for _, metric := range []vector.BitMetricKind{"", "l1", "l2", "cosine", "negative_inner_product", "Hamming"} {
		if got, err := metric.Distance(a, a); !errors.Is(err, vector.ErrMetric) || got != 0 {
			t.Fatal("invalid metric", metric, got, err)
		}
	}
	for _, fn := range []func(vector.BitVector, vector.BitVector) (float64, error){vector.HammingDistance, vector.JaccardDistance} {
		if got, err := fn(a, b); !errors.Is(err, vector.ErrDimensions) || got != 0 {
			t.Fatal("wrapper dimensions", got, err)
		}
		if got, err := fn(vector.BitVector{}, a); !errors.Is(err, vector.ErrInvalid) || got != 0 {
			t.Fatal("wrapper invalid", got, err)
		}
	}
}

// bitCancelOnCheck uses real context cancellation at a deterministic Err call.
// It exercises checks reached after actual packed-byte arithmetic, without
// timing-dependent goroutines or an invalid context implementation.
type bitCancelOnCheck struct {
	context.Context
	cancel   context.CancelFunc
	checks   int
	cancelAt int
}

func (c *bitCancelOnCheck) Err() error {
	c.checks++
	if c.checks == c.cancelAt {
		c.cancel()
	}
	return c.Context.Err()
}

func TestBitMetricsCancelDuringBlocks(t *testing.T) {
	a := mustBit(t, strings.Repeat("1", vector.MaxBitDimensions))
	b := mustBit(t, strings.Repeat("01", vector.MaxBitDimensions/2))
	for _, metric := range []vector.BitMetricKind{vector.Hamming, vector.Jaccard} {
		// 32 blocks plus an initial and a final check. Cancellation on check
		// 3 or 8 happens inside the bounded loop, after completed arithmetic;
		// check 34 catches cancellation before publishing the final result.
		for _, cancelAt := range []int{1, 2, 3, 8, 34} {
			base, cancel := context.WithCancel(context.Background())
			ctx := &bitCancelOnCheck{Context: base, cancel: cancel, cancelAt: cancelAt}
			got, err := metric.DistanceContext(ctx, a, b)
			cancel()
			if !errors.Is(err, context.Canceled) || got != 0 || ctx.checks != cancelAt {
				t.Fatal("bounded cancellation", metric, cancelAt, ctx.checks, got, err)
			}
		}
		base, cancel := context.WithCancel(context.Background())
		ctx := &bitCancelOnCheck{Context: base, cancel: cancel, cancelAt: 35}
		_, err := metric.DistanceContext(ctx, a, b)
		cancel()
		if err != nil || ctx.checks != 34 {
			t.Fatal("bounded check count", metric, ctx.checks, err)
		}
	}
}
