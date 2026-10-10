package sql

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"
)

type TokenKind uint8

const (
	EOF TokenKind = iota
	Identifier
	String
	Number
	Blob
	Bind
	Symbol
)

// Token records decoded text and its original byte span. Quoted distinguishes
// identifiers from keywords; SQL quoting never uses backslash escapes.
type Token struct {
	Kind       TokenKind
	Text       string
	Start, End int
	Quoted     bool
}

func Tokenize(s string) ([]Token, error) {
	if len(s) > MaxSQLBytes {
		return nil, &ErrUnsupported{Feature: "SQL length exceeds 1 MiB"}
	}
	if !utf8.ValidString(s) {
		return nil, fmt.Errorf("SQL is not valid UTF-8")
	}
	out := make([]Token, 0, 32)
	add := func(k TokenKind, t string, a, b int, q bool) { out = append(out, Token{k, t, a, b, q}) }
	i := 0
	if strings.HasPrefix(s, "\ufeff") {
		i = 3
	}
	for i < len(s) {
		if len(out) >= MaxTokens {
			return nil, &ErrUnsupported{Feature: "token count exceeds 32768"}
		}
		c := s[i]
		start := i
		if c == 0 {
			return nil, fmt.Errorf("NUL in SQL at byte %d", i)
		}
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' {
			i++
			continue
		}
		if i+1 < len(s) && s[i:i+2] == "--" {
			i += 2
			for i < len(s) && s[i] != '\n' {
				if s[i] == 0 {
					return nil, fmt.Errorf("NUL in SQL")
				}
				i++
			}
			continue
		}
		if i+1 < len(s) && s[i:i+2] == "/*" {
			j := strings.Index(s[i+2:], "*/")
			if j < 0 {
				return nil, fmt.Errorf("unterminated SQL comment at byte %d", i)
			}
			if strings.IndexByte(s[i+2:i+2+j], 0) >= 0 {
				return nil, fmt.Errorf("NUL in SQL")
			}
			i += j + 4
			continue
		}
		if (c == 'x' || c == 'X') && i+1 < len(s) && s[i+1] == '\'' {
			i += 2
			j := strings.IndexByte(s[i:], '\'')
			if j < 0 {
				return nil, fmt.Errorf("unterminated blob at byte %d", start)
			}
			t := s[i : i+j]
			if _, err := hex.DecodeString(t); err != nil {
				return nil, fmt.Errorf("invalid blob literal at byte %d", start)
			}
			i += j + 1
			add(Blob, t, start, i, false)
			continue
		}
		if c == '\'' || c == '"' || c == '`' || c == '[' {
			end := c
			if c == '[' {
				end = ']'
			}
			i++
			var b strings.Builder
			closed := false
			for i < len(s) {
				if s[i] == 0 {
					return nil, fmt.Errorf("NUL in SQL")
				}
				if s[i] == end {
					i++
					if c != '[' && i < len(s) && s[i] == end {
						b.WriteByte(end)
						i++
						continue
					}
					closed = true
					break
				}
				b.WriteByte(s[i])
				i++
			}
			if !closed {
				return nil, fmt.Errorf("unterminated quote at byte %d", start)
			}
			k := Identifier
			if c == '\'' {
				k = String
			}
			add(k, b.String(), start, i, k == Identifier)
			continue
		}
		if c == '?' || c == ':' || c == '@' || c == '$' {
			i++
			if c == '?' {
				for i < len(s) && digit(s[i]) {
					i++
				}
			} else {
				for i < len(s) && identPart(s[i]) {
					i++
				}
				if i == start+1 {
					return nil, fmt.Errorf("empty parameter name at byte %d", start)
				}
			}
			add(Bind, s[start:i], start, i, false)
			continue
		}
		if digit(c) || (c == '.' && i+1 < len(s) && digit(s[i+1])) {
			if c == '0' && i+2 < len(s) && (s[i+1] == 'x' || s[i+1] == 'X') {
				i += 2
				begin := i
				for i < len(s) && (hexDigit(s[i]) || s[i] == '_') {
					i++
				}
				if i == begin {
					return nil, fmt.Errorf("invalid hex integer at byte %d", start)
				}
			} else {
				for i < len(s) && (digit(s[i]) || s[i] == '_') {
					i++
				}
				if i < len(s) && s[i] == '.' {
					i++
					for i < len(s) && (digit(s[i]) || s[i] == '_') {
						i++
					}
				}
				if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
					i++
					if i < len(s) && (s[i] == '+' || s[i] == '-') {
						i++
					}
					begin := i
					for i < len(s) && (digit(s[i]) || s[i] == '_') {
						i++
					}
					if i == begin {
						return nil, fmt.Errorf("invalid exponent at byte %d", start)
					}
				}
			}
			if i < len(s) && identStart(s[i]) {
				return nil, fmt.Errorf("invalid numeric literal at byte %d", start)
			}
			t := s[start:i]
			for j := 0; j < len(t); j++ {
				if t[j] == '_' && (j == 0 || j+1 == len(t) || !validNumericSeparatorNeighbor(t, t[j-1]) || !validNumericSeparatorNeighbor(t, t[j+1])) {
					return nil, fmt.Errorf("invalid numeric separator at byte %d", start+j)
				}
			}
			add(Number, t, start, i, false)
			continue
		}
		if identStart(c) {
			i++
			for i < len(s) && identPart(s[i]) {
				i++
			}
			add(Identifier, s[start:i], start, i, false)
			continue
		}
		// Vector distance operators are maximal three-byte tokens. Recognize them
		// before SQLite's <= and -> without changing either existing operator.
		if i+2 < len(s) && isVectorOperator(s[i:i+3]) {
			i += 3
			add(Symbol, s[start:i], start, i, false)
			continue
		}
		if i+1 < len(s) {
			op := s[i : i+2]
			if op == "<=" || op == ">=" || op == "!=" || op == "<>" || op == "==" || op == "||" || op == "<<" || op == ">>" || op == "->" {
				i += 2
				if op == "->" && i < len(s) && s[i] == '>' {
					i++
				}
				add(Symbol, s[start:i], start, i, false)
				continue
			}
		}
		if strings.ContainsRune("(),;.*+-/%=<>~&|", rune(c)) {
			i++
			add(Symbol, s[start:i], start, i, false)
			continue
		}
		return nil, fmt.Errorf("unrecognized SQL character %q at byte %d", c, i)
	}
	add(EOF, "", len(s), len(s), false)
	return out, nil
}
func digit(c byte) bool    { return c >= '0' && c <= '9' }
func hexDigit(c byte) bool { return digit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }
func identStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 128
}
func identPart(c byte) bool { return identStart(c) || digit(c) || c == '$' }

func validNumericSeparatorNeighbor(token string, c byte) bool {
	if strings.HasPrefix(token, "0x") || strings.HasPrefix(token, "0X") {
		return hexDigit(c)
	}
	return digit(c)
}
