package ann

import (
	"context"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// ValueKind is a closed ANN representation identity. Empty preserves dense v1.
type ValueKind string

const (
	DenseKind            ValueKind = ""
	HalfKind             ValueKind = "halfvec"
	BitKind              ValueKind = "bit"
	SparseKind           ValueKind = "sparsevec"
	TypedMethodVersion             = 2
	MaxHalfANNDimensions           = 4000
	MaxSparseANNNonzeros           = 1000
)

// Value is an immutable closed union. Constructors do not coerce representations.
// The zero value is invalid; the underlying vector values own immutable storage.
type Value struct {
	kind   ValueKind
	dense  vector.Vector
	half   vector.HalfVector
	bit    vector.BitVector
	sparse vector.SparseVector
}

func DenseValue(v vector.Vector) Value        { return Value{dense: v} }
func HalfValue(v vector.HalfVector) Value     { return Value{kind: HalfKind, half: v} }
func BitValue(v vector.BitVector) Value       { return Value{kind: BitKind, bit: v} }
func SparseValue(v vector.SparseVector) Value { return Value{kind: SparseKind, sparse: v} }
func (v Value) Kind() ValueKind               { return v.kind }
func (v Value) Dense() vector.Vector          { return v.dense }
func (v Value) Half() vector.HalfVector       { return v.half }
func (v Value) Bit() vector.BitVector         { return v.bit }
func (v Value) Sparse() vector.SparseVector   { return v.sparse }
func (v Value) Valid() bool {
	switch v.kind {
	case DenseKind:
		return v.dense.Valid()
	case HalfKind:
		return v.half.Valid()
	case BitKind:
		return v.bit.Valid()
	case SparseKind:
		return v.sparse.Valid()
	}
	return false
}
func (v Value) Dims() int {
	switch v.kind {
	case DenseKind:
		return v.dense.Dims()
	case HalfKind:
		return v.half.Dims()
	case BitKind:
		return v.bit.Dims()
	case SparseKind:
		return v.sparse.Dims()
	}
	return 0
}

// MemoryBytes counts component backing storage, excluding fixed union overhead.
func (v Value) MemoryBytes() int64 {
	switch v.kind {
	case DenseKind:
		return 4 * int64(v.Dims())
	case HalfKind:
		return 2 * int64(v.Dims())
	case BitKind:
		return int64((v.Dims() + 7) / 8)
	case SparseKind:
		return 8 * int64(v.sparse.NNZ())
	}
	return 0
}

// WorkUnits counts dimensions, packed bytes, or nonzero entries for one value.
func (v Value) WorkUnits() uint64 {
	switch v.kind {
	case BitKind:
		return uint64((v.Dims() + 7) / 8)
	case SparseKind:
		return uint64(v.sparse.NNZ())
	default:
		return uint64(v.Dims())
	}
}

// DistanceWork is the bounded actual metric support, not declared sparse width.
func DistanceWork(a, b Value) uint64 {
	if a.kind == SparseKind {
		return a.WorkUnits() + b.WorkUnits()
	}
	return a.WorkUnits()
}
func (v Value) MarshalBinary() ([]byte, error) {
	switch v.kind {
	case DenseKind:
		return v.dense.MarshalBinary()
	case HalfKind:
		return v.half.MarshalBinary()
	case BitKind:
		return v.bit.MarshalBinary()
	case SparseKind:
		return v.sparse.MarshalBinary()
	}
	return nil, vector.ErrInvalid
}
func valueNorm(ctx context.Context, v Value) (float64, error) {
	switch v.kind {
	case DenseKind:
		return vector.Norm(v.dense)
	case HalfKind:
		return vector.HalfNormContext(ctx, v.half)
	case SparseKind:
		return vector.SparseNormContext(ctx, v.sparse)
	}
	return 0, vector.ErrMetric
}
func ValidateValue(v Value, c TypedConfig) error {
	return ValidateValueContext(context.Background(), v, c)
}
func ValidateValueContext(ctx context.Context, v Value, c TypedConfig) error {
	if ctx == nil {
		return vector.ErrContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := c.Validate(); err != nil {
		return err
	}
	if v.kind != c.Kind {
		return fmt.Errorf("%w: value kind", ErrConfig)
	}
	if !v.Valid() {
		return vector.ErrInvalid
	}
	if v.Dims() != c.Dimensions {
		return vector.ErrDimensions
	}
	if v.kind == SparseKind && v.sparse.NNZ() > MaxSparseANNNonzeros {
		return fmt.Errorf("%w: sparse ANN nonzeros", vector.ErrDimensions)
	}
	if c.Metric == vector.Cosine {
		n, err := valueNorm(ctx, v)
		if err != nil {
			return err
		}
		if n == 0 {
			return vector.ErrZeroNorm
		}
	}
	return ctx.Err()
}

// EncodeValue keeps dense GSV v1 byte-for-byte and emits the distinct canonical
// GSH/GSB/GSS v1 codecs for typed-v2 indexes. TypedConfig pins kind and codec identity.
func EncodeValue(v Value, c TypedConfig) ([]byte, error) {
	if err := ValidateValue(v, c); err != nil {
		return nil, err
	}
	return v.MarshalBinary()
}
func DecodeValue(b []byte, c TypedConfig) (Value, error) {
	if err := c.Validate(); err != nil {
		return Value{}, err
	}
	// Bound allocations by the indexed tuple before invoking broader value decoders.
	maxBytes := 12 + int(typedValueStorageBytes(c))
	if c.Kind == SparseKind {
		maxBytes = vector.SparseBinaryHeaderSize + 8*MaxSparseANNNonzeros
	}
	if len(b) > maxBytes {
		return Value{}, vector.ErrEncoding
	}
	var v Value
	var err error
	switch c.Kind {
	case DenseKind:
		var x vector.Vector
		x, err = vector.Decode(b)
		v = DenseValue(x)
	case HalfKind:
		var x vector.HalfVector
		x, err = vector.DecodeHalf(b)
		v = HalfValue(x)
	case BitKind:
		var x vector.BitVector
		x, err = vector.DecodeBit(b)
		v = BitValue(x)
	case SparseKind:
		var x vector.SparseVector
		x, err = vector.DecodeSparse(b)
		v = SparseValue(x)
	default:
		return Value{}, ErrConfig
	}
	if err != nil {
		return Value{}, err
	}
	if err = ValidateValue(v, c); err != nil {
		return Value{}, err
	}
	return v, nil
}
func Distance(c TypedConfig, a, b Value) (float64, error) {
	return DistanceContext(context.Background(), c, a, b)
}

// DistanceContext dispatches by the persisted closed kind/metric tuple. It never
// materializes sparse dimensions or silently converts a metric/representation.
func DistanceContext(ctx context.Context, c TypedConfig, a, b Value) (float64, error) {
	if ctx == nil {
		return 0, vector.ErrContext
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := c.Validate(); err != nil {
		return 0, err
	}
	if a.kind != c.Kind || b.kind != c.Kind {
		return 0, ErrConfig
	}
	if a.Dims() != c.Dimensions || b.Dims() != c.Dimensions {
		return 0, vector.ErrDimensions
	}
	if c.Kind == SparseKind && (a.sparse.NNZ() > MaxSparseANNNonzeros || b.sparse.NNZ() > MaxSparseANNNonzeros) {
		return 0, vector.ErrDimensions
	}
	switch c.Kind {
	case DenseKind:
		d, err := c.Metric.Distance(a.dense, b.dense)
		if err != nil {
			return 0, err
		}
		if err = ctx.Err(); err != nil {
			return 0, err
		}
		return d, nil
	case HalfKind:
		return c.Metric.DistanceHalfContext(ctx, a.half, b.half)
	case BitKind:
		return vector.BitMetricKind(c.Metric).DistanceContext(ctx, a.bit, b.bit)
	case SparseKind:
		return c.Metric.DistanceSparseContext(ctx, a.sparse, b.sparse)
	}
	return 0, ErrConfig
}
