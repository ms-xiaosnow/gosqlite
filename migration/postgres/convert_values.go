package postgres

import (
	"fmt"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// Checkpoint A's value parser deliberately permits decimal underflow to zero.
// SQL conversion has a stricter source-error contract: PostgreSQL float4 input
// rejects a nonzero decimal that rounds all the way to zero. Apply that rule
// without changing A or rejecting an already valid []float32 zero bind.
func parseConverterRealArray(text string, dims int) (vector.Vector, error) {
	value, err := ParseRealArray(text, dims)
	if err != nil {
		return vector.Vector{}, err
	}
	values := value.Values()
	i := strings.IndexByte(text, '{') + 1
	for _, component := range values {
		for i < len(text) && space(text[i]) {
			i++
		}
		token := ""
		if text[i] == '"' {
			i++
			var out strings.Builder
			for i < len(text) {
				c := text[i]
				i++
				if c == '"' {
					break
				}
				if c == '\\' {
					c = text[i]
					i++
				}
				out.WriteByte(c)
			}
			token = out.String()
		} else {
			start := i
			for i < len(text) && text[i] != ',' && text[i] != '}' {
				i++
			}
			token = text[start:i]
		}
		if component == 0 && nonzeroDecimalSignificand(token) {
			return vector.Vector{}, fmt.Errorf("convert real array: PostgreSQL nonzero underflow")
		}
		for i < len(text) && space(text[i]) {
			i++
		}
		if i < len(text) && text[i] == ',' {
			i++
		}
	}
	return value, nil
}
func nonzeroDecimalSignificand(token string) bool {
	for _, c := range token {
		if c == 'e' || c == 'E' {
			break
		}
		if c >= '1' && c <= '9' {
			return true
		}
	}
	return false
}
