package postgres

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

const MaxArrayTextBytes = 2 << 20

// ValueError identifies a non-NULL source value failure without echoing any
// embedding contents. Element is one-based, or zero for an array-level error.
type ValueError struct {
	Code                        string
	Element, Expected, Observed int
}

func (e *ValueError) Error() string {
	return fmt.Sprintf("real array: %s (element=%d, expected=%d, observed=%d)", e.Code, e.Element, e.Expected, e.Observed)
}
func arrayError(code string, element, expected, observed int) error {
	return &ValueError{code, element, expected, observed}
}
func space(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}

// ParseRealArray accepts only finite, one-dimensional PostgreSQL real[] text.
// SQL NULL is handled by the caller and is never an empty vector. Explicit
// bounds must be [1:n]; signed zeros and subnormals preserve binary32 bits.
// Parsing rounds decimal directly to binary32 via the existing dense parser.
func ParseRealArray(text string, dims int) (vector.Vector, error) {
	empty := vector.Vector{}
	if dims < 1 || dims > vector.MaxDimensions {
		return empty, arrayError("dimension_required", 0, dims, 0)
	}
	if len(text) > MaxArrayTextBytes {
		return empty, arrayError("resource_limit", 0, dims, 0)
	}
	if !utf8.ValidString(text) || strings.IndexByte(text, 0) >= 0 {
		return empty, arrayError("invalid_encoding", 0, dims, 0)
	}
	i := 0
	skip := func() {
		for i < len(text) && space(text[i]) {
			i++
		}
	}
	skip()
	bound := -1
	if i < len(text) && text[i] == '[' {
		i++
		skip()
		readInt := func() (int, error) {
			start := i
			if i < len(text) && (text[i] == '-' || text[i] == '+') {
				i++
			}
			digits := i
			for i < len(text) && text[i] >= '0' && text[i] <= '9' {
				i++
				if i-start > 11 {
					return 0, arrayError("invalid_bounds", 0, dims, 0)
				}
			}
			if i == digits {
				return 0, arrayError("invalid_bounds", 0, dims, 0)
			}
			n, e := strconv.ParseInt(text[start:i], 10, 32)
			if e != nil {
				return 0, arrayError("invalid_bounds", 0, dims, 0)
			}
			return int(n), nil
		}
		lo, e := readInt()
		if e != nil {
			return empty, e
		}
		skip()
		if i >= len(text) || text[i] != ':' {
			return empty, arrayError("invalid_bounds", 0, dims, 0)
		}
		i++
		skip()
		hi, e := readInt()
		if e != nil {
			return empty, e
		}
		skip()
		if i >= len(text) || text[i] != ']' {
			return empty, arrayError("invalid_bounds", 0, dims, 0)
		}
		i++
		skip()
		if i < len(text) && text[i] == '[' {
			return empty, arrayError("multidimensional_array", 0, dims, 0)
		}
		if lo != 1 {
			return empty, arrayError("nondefault_array_bounds", 0, dims, 0)
		}
		if hi < lo || hi > vector.MaxDimensions {
			return empty, arrayError("invalid_bounds", 0, dims, 0)
		}
		bound = hi
		if i >= len(text) || text[i] != '=' {
			return empty, arrayError("invalid_bounds", 0, dims, 0)
		}
		i++
		skip()
	}
	if i >= len(text) || text[i] != '{' {
		return empty, arrayError("array_syntax", 0, dims, 0)
	}
	i++
	skip()
	if i < len(text) && text[i] == '}' {
		return empty, arrayError("empty_array", 0, dims, 0)
	}
	values := make([]float32, 0, min(dims, 64))
	for {
		if len(values) >= vector.MaxDimensions {
			return empty, arrayError("resource_limit", 0, dims, len(values)+1)
		}
		skip()
		if i >= len(text) {
			return empty, arrayError("array_syntax", len(values)+1, dims, 0)
		}
		if text[i] == '{' {
			return empty, arrayError("multidimensional_array", len(values)+1, dims, 0)
		}
		quoted := text[i] == '"'
		var token string
		if quoted {
			i++
			var b strings.Builder
			closed := false
			for i < len(text) {
				c := text[i]
				i++
				if c == '"' {
					closed = true
					break
				}
				if c == '\\' {
					if i == len(text) {
						return empty, arrayError("array_syntax", len(values)+1, dims, 0)
					}
					c = text[i]
					i++
				}
				b.WriteByte(c)
				if b.Len() > vector.MaxElementTextBytes {
					return empty, arrayError("resource_limit", len(values)+1, dims, 0)
				}
			}
			if !closed {
				return empty, arrayError("array_syntax", len(values)+1, dims, 0)
			}
			token = b.String()
			skip()
		} else {
			start := i
			for i < len(text) && text[i] != ',' && text[i] != '}' {
				if text[i] == '{' || text[i] == '}' {
					return empty, arrayError("multidimensional_array", len(values)+1, dims, 0)
				}
				if text[i] == '"' || text[i] == '\\' {
					return empty, arrayError("array_syntax", len(values)+1, dims, 0)
				}
				i++
				if i-start > vector.MaxElementTextBytes {
					return empty, arrayError("resource_limit", len(values)+1, dims, 0)
				}
			}
			token = strings.TrimFunc(text[start:i], func(r rune) bool { return r < 128 && space(byte(r)) })
		}
		if !quoted && strings.EqualFold(token, "NULL") {
			return empty, arrayError("null_element", len(values)+1, dims, 0)
		}
		if len(token) > vector.MaxElementTextBytes {
			return empty, arrayError("resource_limit", len(values)+1, dims, 0)
		}
		scalar, e := vector.Parse("[" + token + "]")
		if e != nil {
			code := "invalid_component"
			if errors.Is(e, vector.ErrNonFinite) {
				code = "nonfinite_component"
			}
			return empty, arrayError(code, len(values)+1, dims, 0)
		}
		if scalar.Dims() != 1 {
			return empty, arrayError("invalid_component", len(values)+1, dims, 0)
		}
		values = append(values, scalar.Values()[0])
		if i >= len(text) {
			return empty, arrayError("array_syntax", len(values), dims, 0)
		}
		switch text[i] {
		case ',':
			i++
		case '}':
			i++
			skip()
			if i != len(text) {
				return empty, arrayError("array_syntax", 0, dims, len(values))
			}
			if bound >= 0 && bound != len(values) {
				return empty, arrayError("bounds_length_mismatch", 0, bound, len(values))
			}
			if len(values) != dims {
				return empty, arrayError("dimension_mismatch", 0, dims, len(values))
			}
			return vector.New(values)
		default:
			return empty, arrayError("array_syntax", len(values), dims, 0)
		}
	}
}
