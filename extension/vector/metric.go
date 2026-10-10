package vector

import "math"

// Metric is an exact distance used by Scan. Lower values rank first. An
// implementation must return finite values or an error, be deterministic, and
// document any domain restrictions. Callers must not confuse this identity with
// an ANN operator class; this package does not implement ANN indexing.
type Metric interface {
	Name() string
	Distance(a, b Vector) (float64, error)
}

// MetricKind identifies this package's four built-in dense distance semantics.
type MetricKind string

const (
	L2         MetricKind = "l2"
	L1         MetricKind = "l1"
	Cosine     MetricKind = "cosine"
	NegativeIP MetricKind = "negative_inner_product"
)

func (m MetricKind) Name() string { return string(m) }

func (m MetricKind) Distance(a, b Vector) (float64, error) {
	switch m {
	case L2:
		return L2Distance(a, b)
	case L1:
		return L1Distance(a, b)
	case Cosine:
		return CosineDistance(a, b)
	case NegativeIP:
		return NegativeInnerProduct(a, b)
	default:
		return 0, ErrMetric
	}
}

func checkPair(a, b Vector) error {
	if !a.Valid() || !b.Valid() {
		return ErrInvalid
	}
	if a.Dims() != b.Dims() {
		return ErrDimensions
	}
	return nil
}

// accumulator uses Neumaier compensation, including cancellation where a new
// term is larger than the running sum. Products of finite float32 inputs and
// sums of at most 16000 terms cannot overflow or underflow float64.
type accumulator struct{ sum, correction float64 }

func (a *accumulator) add(x float64) {
	t := a.sum + x
	if math.Abs(a.sum) >= math.Abs(x) {
		a.correction += (a.sum - t) + x
	} else {
		a.correction += (x - t) + a.sum
	}
	a.sum = t
}
func (a accumulator) value() float64 { return a.sum + a.correction }

// L2Distance returns Euclidean distance (<->), widening before subtraction.
func L2Distance(a, b Vector) (float64, error) {
	if err := checkPair(a, b); err != nil {
		return 0, err
	}
	var sum accumulator
	for i, x := range a.values {
		d := float64(x) - float64(b.values[i])
		sum.add(d * d)
	}
	return math.Sqrt(sum.value()), nil
}

// L1Distance returns Manhattan distance (<+>); <~> is not a dense L1 operator.
func L1Distance(a, b Vector) (float64, error) {
	if err := checkPair(a, b); err != nil {
		return 0, err
	}
	var sum accumulator
	for i, x := range a.values {
		sum.add(math.Abs(float64(x) - float64(b.values[i])))
	}
	return sum.value(), nil
}

// InnerProduct returns the positive dot product (the inner_product function).
func InnerProduct(a, b Vector) (float64, error) {
	if err := checkPair(a, b); err != nil {
		return 0, err
	}
	var sum accumulator
	for i, x := range a.values {
		sum.add(float64(x) * float64(b.values[i]))
	}
	return sum.value(), nil
}

// NegativeInnerProduct returns the negative dot product (<#>), so ascending
// distance ranks the largest positive dot product first.
func NegativeInnerProduct(a, b Vector) (float64, error) {
	x, err := InnerProduct(a, b)
	if err != nil {
		return 0, err
	}
	return -x, nil
}

// Norm returns the L2 norm, including zero for a valid all-zero vector.
func Norm(a Vector) (float64, error) {
	if !a.Valid() {
		return 0, ErrInvalid
	}
	var sum accumulator
	for _, x := range a.values {
		f := float64(x)
		sum.add(f * f)
	}
	return math.Sqrt(sum.value()), nil
}

// CosineDistance returns 1 - cosine similarity in [0,2]. Any zero norm returns
// ErrZeroNorm. This intentional policy prevents NaN from corrupting ordering.
func CosineDistance(a, b Vector) (float64, error) {
	if err := checkPair(a, b); err != nil {
		return 0, err
	}
	var dot, aa, bb accumulator
	for i, x := range a.values {
		f, g := float64(x), float64(b.values[i])
		dot.add(f * g)
		aa.add(f * f)
		bb.add(g * g)
	}
	an, bn := aa.value(), bb.value()
	if an == 0 || bn == 0 {
		return 0, ErrZeroNorm
	}
	// Float32 range and the dimension bound keep this product within float64.
	similarity := dot.value() / math.Sqrt(an*bn)
	similarity = math.Max(-1, math.Min(1, similarity))
	return 1 - similarity, nil
}
