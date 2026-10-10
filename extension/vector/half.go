package vector

import (
	"fmt"
	"math"
	"strconv"
)

const MaxHalfDimensions = MaxDimensions

// HalfVector is an immutable vector of 1..16000 finite IEEE-754 binary16
// components. The zero Go value is invalid. All slice accessors return copies.
// Conversions preserve signed zero and binary16 subnormals.
type HalfVector struct{ bits []uint16 }

// NewHalf rounds finite binary32 values to binary16, nearest with ties to even.
// Overflow is rejected after rounding: values just above 65504 can still round
// to the largest finite half. No partial value is returned on error.
func NewHalf(values []float32) (HalfVector, error) {
	if err := checkDimensions(len(values)); err != nil {
		return HalfVector{}, err
	}
	b := make([]uint16, len(values))
	for i, x := range values {
		h, err := halfFromFloat32(x)
		if err != nil {
			return HalfVector{}, fmt.Errorf("%w at component %d", err, i)
		}
		b[i] = h
	}
	return HalfVector{bits: b}, nil
}

// NewHalfBits validates and copies exact binary16 representations. Every NaN
// and infinity representation is rejected; all finite representations survive.
func NewHalfBits(values []uint16) (HalfVector, error) {
	if err := checkDimensions(len(values)); err != nil {
		return HalfVector{}, err
	}
	for i, x := range values {
		if x&0x7c00 == 0x7c00 {
			return HalfVector{}, fmt.Errorf("%w at component %d", ErrNonFinite, i)
		}
	}
	return HalfVector{bits: append([]uint16(nil), values...)}, nil
}

// halfFromFloat32 uses exact binary scaling in float64 before a single
// ties-to-even integer rounding. A binary32 value widens exactly to float64.
// This independent implementation does not depend on hardware half support.
func halfFromFloat32(x float32) (uint16, error) {
	if !finite32(x) {
		return 0, ErrNonFinite
	}
	sign := uint16(math.Float32bits(x) >> 16 & 0x8000)
	f := math.Abs(float64(x))
	if f < 0x1p-14 {
		// Subnormals are integer multiples of 2^-24. Rounding can carry to
		// the minimum normal (0x0400), or to the appropriately signed zero.
		return sign | uint16(math.RoundToEven(math.Ldexp(f, 24))), nil
	}
	_, exponent := math.Frexp(f)
	e := exponent - 1
	m := int(math.RoundToEven(math.Ldexp(f, 10-e)))
	if m == 2048 {
		e++
		m = 1024
	}
	if e > 15 {
		return 0, ErrNonFinite
	}
	return sign | uint16(e+15)<<10 | uint16(m-1024), nil
}

// halfFloat32 is called only for validated finite half representations.
func halfFloat32(h uint16) float32 {
	e, m := int(h>>10&31), int(h&1023)
	var x float32
	if e == 0 {
		x = float32(math.Ldexp(float64(m), -24))
	} else {
		x = float32(math.Ldexp(float64(1024+m), e-25))
	}
	return math.Float32frombits(math.Float32bits(x) | uint32(h&0x8000)<<16)
}

func (v HalfVector) Valid() bool    { return len(v.bits) >= 1 && len(v.bits) <= MaxHalfDimensions }
func (v HalfVector) Dims() int      { return len(v.bits) }
func (v HalfVector) Bits() []uint16 { return append([]uint16(nil), v.bits...) }

// Values returns an exact widening to binary32 in an independent slice.
func (v HalfVector) Values() []float32 {
	if !v.Valid() {
		return nil
	}
	x := make([]float32, len(v.bits))
	for i, h := range v.bits {
		x[i] = halfFloat32(h)
	}
	return x
}

func (v HalfVector) RequireDims(n int) error {
	if err := checkDimensions(n); err != nil {
		return err
	}
	if !v.Valid() {
		return ErrInvalid
	}
	if n != v.Dims() {
		return ErrDimensions
	}
	return nil
}

func (v HalfVector) String() string {
	if !v.Valid() {
		return "<invalid half vector>"
	}
	b := make([]byte, 0, 2+8*v.Dims())
	b = append(b, '[')
	for i, h := range v.bits {
		if i != 0 {
			b = append(b, ',')
		}
		b = strconv.AppendFloat(b, float64(halfFloat32(h)), 'g', -1, 32)
	}
	return string(append(b, ']'))
}

func (v HalfVector) MarshalText() ([]byte, error) {
	if !v.Valid() {
		return nil, ErrInvalid
	}
	return []byte(v.String()), nil
}

// ParseHalf follows the pinned pgvector decimal -> binary32 -> binary16 route.
// It shares Parse's bounded ASCII decimal grammar, including signed underflow
// to zero. It never rounds a decimal directly to binary16.
func ParseHalf(text string) (HalfVector, error) {
	v, err := Parse(text)
	if err != nil {
		return HalfVector{}, err
	}
	return NewHalf(v.values)
}

func ParseHalfDims(text string, n int) (HalfVector, error) {
	if err := checkDimensions(n); err != nil {
		return HalfVector{}, err
	}
	v, err := ParseHalf(text)
	if err != nil {
		return HalfVector{}, err
	}
	if err := v.RequireDims(n); err != nil {
		return HalfVector{}, err
	}
	return v, nil
}

// HalfConversion reports lossy binary32 -> binary16 conversion. Relative error
// is |rounded-source|/|source| for nonzero source components. Exact signed zeros
// have zero error. Changed counts differing binary32 bits after re-widening.
type HalfConversion struct {
	Changed          int
	MaxAbsoluteError float64
	MaxRelativeError float64
}

// HalfFromVector explicitly quantizes dense data and reports conversion error.
// On any error both outputs are zero; callers should retain original vectors.
func HalfFromVector(v Vector) (HalfVector, HalfConversion, error) {
	if !v.Valid() {
		return HalfVector{}, HalfConversion{}, ErrInvalid
	}
	h, err := NewHalf(v.values)
	if err != nil {
		return HalfVector{}, HalfConversion{}, err
	}
	var report HalfConversion
	for i, x := range v.values {
		y := halfFloat32(h.bits[i])
		if math.Float32bits(x) != math.Float32bits(y) {
			report.Changed++
		}
		d := math.Abs(float64(x) - float64(y))
		report.MaxAbsoluteError = math.Max(report.MaxAbsoluteError, d)
		if x != 0 {
			report.MaxRelativeError = math.Max(report.MaxRelativeError, d/math.Abs(float64(x)))
		}
	}
	return h, report, nil
}

// Dense widens half values exactly. It never exposes or reuses private storage.
func (v HalfVector) Dense() (Vector, error) {
	if !v.Valid() {
		return Vector{}, ErrInvalid
	}
	return Vector{values: v.Values()}, nil
}
