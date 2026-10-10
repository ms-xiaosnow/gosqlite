package postgres

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"unicode/utf8"
)

// budgetCatalog bounds additional inspection work before maps, reports or
// canonical JSON are allocated. The caller already owns programmatic input
// memory; this pass cannot reclaim it. Every collection element is charged
// before traversal. Bytes are a conservative upper bound on encoding/json's
// canonical output, so custom limits may reject a near-limit compact input.
func budgetCatalog(c CatalogSnapshot, l Limits) error { return budgetValue(c, l) }

func budgetValue(c any, l Limits) error {
	bytesLeft, nodesLeft := int64(l.MaxBytes), int64(l.MaxTokens)
	charge := func(n int64) error {
		if n < 0 || n > bytesLeft {
			return fmt.Errorf("catalog: byte budget exceeded")
		}
		bytesLeft -= n
		return nil
	}
	var visit func(reflect.Value, int) error
	visit = func(v reflect.Value, depth int) error {
		if depth > l.MaxDepth {
			return fmt.Errorf("catalog: depth budget exceeded")
		}
		nodesLeft--
		if nodesLeft < 0 {
			return fmt.Errorf("catalog: work budget exceeded")
		}
		switch v.Kind() {
		case reflect.Struct:
			if err := charge(2); err != nil {
				return err
			}
			for i := 0; i < v.NumField(); i++ {
				f := v.Type().Field(i)
				if f.PkgPath != "" {
					continue
				}
				name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
				if name == "-" {
					continue
				}
				if err := charge(int64(len(name) + 4)); err != nil {
					return err
				}
				if err := visit(v.Field(i), depth+1); err != nil {
					return err
				}
			}
		case reflect.Slice:
			if v.IsNil() {
				return fmt.Errorf("catalog: required array must not be nil")
			}
			if int64(v.Len()) > nodesLeft {
				return fmt.Errorf("catalog: collection work budget exceeded")
			}
			if err := charge(int64(v.Len()) + 2); err != nil {
				return err
			}
			for i := 0; i < v.Len(); i++ {
				if err := visit(v.Index(i), depth+1); err != nil {
					return err
				}
			}
		case reflect.String:
			s := v.String()
			if len(s) > 1<<20 || int64(len(s)) > bytesLeft {
				return fmt.Errorf("catalog: string byte budget exceeded")
			}
			if !utf8.ValidString(s) || strings.IndexByte(s, 0) >= 0 {
				return fmt.Errorf("catalog: invalid string encoding")
			}
			n := int64(2)
			for _, r := range s {
				switch {
				case r == '"' || r == '\\':
					n += 2
				case r < 32 || r == '<' || r == '>' || r == '&' || r == '\u2028' || r == '\u2029':
					n += 6
				default:
					n += int64(utf8.RuneLen(r))
				}
			}
			if err := charge(n); err != nil {
				return err
			}
		case reflect.Pointer:
			if v.IsNil() {
				return charge(4)
			}
			return visit(v.Elem(), depth+1)
		case reflect.Float64:
			if math.IsNaN(v.Float()) || math.IsInf(v.Float(), 0) {
				return fmt.Errorf("catalog: nonfinite numeric profile")
			}
			return charge(24)
		case reflect.Int, reflect.Int64, reflect.Uint32:
			return charge(24)
		case reflect.Bool:
			return charge(5)
		default:
			return fmt.Errorf("catalog: unsupported budget field kind")
		}
		return nil
	}
	return visit(reflect.ValueOf(c), 0)
}

// encoding/json replaces unpaired UTF-16 escapes with U+FFFD. Catalog identity
// must not be silently repaired, so reject them before standard JSON decoding.
func checkJSONUnicodeEscapes(b []byte) error {
	inString := false
	hex4 := func(pos int) (uint16, bool) {
		if pos+4 > len(b) {
			return 0, false
		}
		var n uint16
		for _, c := range b[pos : pos+4] {
			n <<= 4
			switch {
			case c >= '0' && c <= '9':
				n += uint16(c - '0')
			case c >= 'a' && c <= 'f':
				n += uint16(c - 'a' + 10)
			case c >= 'A' && c <= 'F':
				n += uint16(c - 'A' + 10)
			default:
				return 0, false
			}
		}
		return n, true
	}
	for i := 0; i < len(b); i++ {
		if b[i] == '"' {
			inString = !inString
			continue
		}
		if !inString || b[i] != '\\' {
			continue
		}
		i++
		if i >= len(b) {
			return fmt.Errorf("json: truncated escape")
		}
		if b[i] != 'u' {
			continue
		}
		v, ok := hex4(i + 1)
		if !ok {
			return fmt.Errorf("json: invalid Unicode escape")
		}
		i += 4
		if v >= 0xdc00 && v <= 0xdfff {
			return fmt.Errorf("json: unpaired Unicode surrogate")
		}
		if v >= 0xd800 && v <= 0xdbff {
			if i+6 >= len(b) || b[i+1] != '\\' || b[i+2] != 'u' {
				return fmt.Errorf("json: unpaired Unicode surrogate")
			}
			low, ok := hex4(i + 3)
			if !ok || low < 0xdc00 || low > 0xdfff {
				return fmt.Errorf("json: unpaired Unicode surrogate")
			}
			i += 6
		}
	}
	return nil
}
