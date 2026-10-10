package postgres

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// SQLSpan is a half-open, zero-based byte range in the original SQL source.
type SQLSpan struct{ Start, End int }

const (
	MaxConvertSQLBytes        = 1 << 20
	MaxConvertSQLTokens       = 32768
	MaxConvertSQLDepth        = 64
	MaxConvertSQLStatements   = 256
	MaxConvertSQLLiteralBytes = 1 << 20
)

// SQLLimits can only lower the bounded default PostgreSQL source profile.
// The profile assumes UTF-8, PostgreSQL 15 lexical syntax and
// standard_conforming_strings=on. It never interprets procedural bodies.
type SQLLimits struct{ MaxBytes, MaxTokens, MaxDepth, MaxStatements, MaxLiteralBytes int }

func DefaultSQLLimits() SQLLimits {
	return SQLLimits{MaxConvertSQLBytes, MaxConvertSQLTokens, MaxConvertSQLDepth, MaxConvertSQLStatements, MaxConvertSQLLiteralBytes}
}
func (l SQLLimits) normalized() (SQLLimits, error) {
	d := DefaultSQLLimits()
	if l == (SQLLimits{}) {
		return d, nil
	}
	if l.MaxBytes < 1 || l.MaxBytes > d.MaxBytes || l.MaxTokens < 1 || l.MaxTokens > d.MaxTokens || l.MaxDepth < 1 || l.MaxDepth > d.MaxDepth || l.MaxStatements < 1 || l.MaxStatements > d.MaxStatements || l.MaxLiteralBytes < 1 || l.MaxLiteralBytes > d.MaxLiteralBytes {
		return l, &SQLParseError{Code: "invalid_limits"}
	}
	return l, nil
}

// SQLParseError deliberately contains no source text, literal or identifier.
type SQLParseError struct {
	Code string
	Span SQLSpan
}

func (e *SQLParseError) Error() string {
	return fmt.Sprintf("PostgreSQL SQL %s at bytes %d:%d", e.Code, e.Span.Start, e.Span.End)
}

type pgTokenKind uint8

const (
	pgEOF pgTokenKind = iota
	pgIdentifier
	pgString
	pgEscapeString
	pgDollarString
	pgBitString
	pgOpaque
	pgNumber
	pgParameter
	pgSymbol
	pgWhitespace
	pgComment
)

type pgToken struct {
	Kind       pgTokenKind
	Text       string
	Start, End int
	Quoted     bool
}
type pgStatementTokens struct {
	First, Last int
	Span        SQLSpan
}

func pgNameSpan(name []pgToken) SQLSpan {
	if len(name) == 0 {
		return SQLSpan{}
	}
	return SQLSpan{name[0].Start, name[len(name)-1].End}
}
func pgTrivia(t pgToken) bool { return t.Kind == pgWhitespace || t.Kind == pgComment }
func pgDigit(c byte) bool     { return c >= '0' && c <= '9' }
func pgIdentStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 128
}
func pgIdentPart(c byte) bool { return pgIdentStart(c) || pgDigit(c) || c == '$' }
func pgSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}
func pgOp(c byte) bool { return strings.IndexByte("+-*/<>=~!@#%^&|`?", c) >= 0 }
func pgFold(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// pgScan retains every source byte in ordered token spans. Statement ranges
// cover the entire source when it contains a nonempty statement, including
// leading empty statements and trailing trivia/empty statements. A trivia-only
// or empty-statement-only source returns no statement ranges.
// Opaque tokens are never recursively scanned. This scanner is independent of
// the target SQLite grammar.
func pgScan(source string, limits SQLLimits) ([]pgToken, []pgStatementTokens, error) {
	l, err := limits.normalized()
	if err != nil {
		return nil, nil, err
	}
	fail := func(code string, a, b int) ([]pgToken, []pgStatementTokens, error) {
		return nil, nil, &SQLParseError{Code: code, Span: SQLSpan{a, b}}
	}
	if len(source) > l.MaxBytes {
		return fail("input_limit", 0, len(source))
	}
	if !utf8.ValidString(source) {
		return fail("invalid_encoding", 0, len(source))
	}
	if n := strings.IndexByte(source, 0); n >= 0 {
		return fail("invalid_encoding", n, n+1)
	}
	tokens := make([]pgToken, 0, 32)
	statements := make([]pgStatementTokens, 0, 4)
	stack := make([]byte, 0, 8)
	first, segmentStart, significant := 0, 0, false
	for i := 0; i < len(source); {
		if len(tokens) >= l.MaxTokens-1 {
			return fail("token_limit", i, i)
		}
		start := i
		c := source[i]
		kind := pgSymbol
		quoted := false
		value := ""
		switch {
		case pgSpace(c):
			kind = pgWhitespace
			for i < len(source) && pgSpace(source[i]) {
				i++
			}
			value = source[start:i]
		case strings.HasPrefix(source[i:], "--"):
			kind = pgComment
			i += 2
			for i < len(source) && source[i] != '\n' && source[i] != '\r' {
				i++
			}
			value = source[start:i]
		case strings.HasPrefix(source[i:], "/*"):
			kind = pgComment
			i += 2
			depth := 1
			for i < len(source) && depth > 0 {
				if strings.HasPrefix(source[i:], "/*") {
					depth++
					if depth > l.MaxDepth {
						return fail("depth_limit", start, i+2)
					}
					i += 2
				} else if strings.HasPrefix(source[i:], "*/") {
					depth--
					i += 2
				} else {
					i++
				}
			}
			if depth != 0 {
				return fail("unterminated_comment", start, len(source))
			}
			value = source[start:i]
		case c == '\'' || c == '"' || ((c == 'e' || c == 'E' || c == 'b' || c == 'B' || c == 'x' || c == 'X') && i+1 < len(source) && source[i+1] == '\'') || ((c == 'u' || c == 'U') && i+2 < len(source) && source[i+1] == '&' && (source[i+2] == '\'' || source[i+2] == '"')):
			kind = pgString
			prefix := 0
			if c == '"' {
				kind = pgIdentifier
				quoted = true
			} else if c != '\'' {
				prefix = 1
				switch c {
				case 'e', 'E':
					kind = pgEscapeString
				case 'b', 'B', 'x', 'X':
					kind = pgBitString
				case 'u', 'U':
					kind = pgOpaque
					prefix = 2
				}
			}
			quote := source[start+prefix]
			if quote == '"' {
				quoted = true
			}
			i = start + prefix + 1
			body := i
			closed := false
			for i < len(source) {
				if i-start >= l.MaxLiteralBytes {
					return fail("literal_limit", start, i)
				}
				if kind == pgEscapeString && source[i] == '\\' {
					if i+1 < len(source) {
						i += 2
						continue
					}
					i++
					break
				}
				if source[i] == quote {
					if i+1 < len(source) && source[i+1] == quote {
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				i++
			}
			if !closed {
				return fail("unterminated_quote", start, len(source))
			}
			if i-start > l.MaxLiteralBytes {
				return fail("literal_limit", start, i)
			}
			value = source[body : i-1]
			if kind == pgString || kind == pgIdentifier {
				value = strings.ReplaceAll(value, string([]byte{quote, quote}), string(quote))
			}
			if kind == pgIdentifier && (len(value) == 0 || len(value) > 63) {
				return fail("identifier_length", start, i)
			}
		case c == '$':
			if i+1 < len(source) && pgDigit(source[i+1]) {
				kind = pgParameter
				i += 2
				for i < len(source) && pgDigit(source[i]) {
					i++
				}
				if i < len(source) && pgIdentPart(source[i]) {
					return fail("invalid_parameter", start, i+1)
				}
				if i-start > l.MaxLiteralBytes {
					return fail("literal_limit", start, i)
				}
				value = source[start:i]
			} else {
				j := i + 1
				if j < len(source) && pgIdentStart(source[j]) {
					j++
					for j < len(source) && (pgIdentStart(source[j]) || pgDigit(source[j])) {
						j++
					}
				}
				if j >= len(source) || source[j] != '$' {
					return fail("invalid_dollar_quote", start, j)
				}
				delimiter := source[start : j+1]
				body := j + 1
				offset := strings.Index(source[body:], delimiter)
				if offset < 0 {
					return fail("unterminated_dollar_quote", start, len(source))
				}
				i = body + offset + len(delimiter)
				if i-start > l.MaxLiteralBytes {
					return fail("literal_limit", start, i)
				}
				kind = pgDollarString
				value = source[body : body+offset]
			}
		case pgDigit(c) || (c == '.' && i+1 < len(source) && pgDigit(source[i+1])):
			kind = pgNumber
			for i < len(source) && pgDigit(source[i]) {
				i++
			}
			if i < len(source) && source[i] == '.' && !(i+1 < len(source) && source[i+1] == '.') {
				i++
				for i < len(source) && pgDigit(source[i]) {
					i++
				}
			}
			if i < len(source) && (source[i] == 'e' || source[i] == 'E') {
				i++
				if i < len(source) && (source[i] == '+' || source[i] == '-') {
					i++
				}
				digits := i
				for i < len(source) && pgDigit(source[i]) {
					i++
				}
				if i == digits {
					return fail("invalid_number", start, i)
				}
			}
			if i < len(source) && pgIdentPart(source[i]) {
				return fail("invalid_number", start, i+1)
			}
			if i-start > l.MaxLiteralBytes {
				return fail("literal_limit", start, i)
			}
			value = source[start:i]
		case pgIdentStart(c):
			kind = pgIdentifier
			i++
			for i < len(source) && pgIdentPart(source[i]) {
				i++
			}
			if i-start > 63 {
				return fail("identifier_length", start, i)
			}
			value = pgFold(source[start:i])
		case c == ':' && i+1 < len(source) && (source[i+1] == ':' || source[i+1] == '='):
			i += 2
			value = source[start:i]
		case strings.IndexByte("(),;[].:", c) >= 0:
			i++
			value = source[start:i]
		case pgOp(c):
			i++
			for i < len(source) && pgOp(source[i]) {
				if strings.HasPrefix(source[i:], "--") || strings.HasPrefix(source[i:], "/*") {
					break
				}
				i++
			}
			// PostgreSQL permits a multi-byte operator to end in + or - only if
			// a non-SQL-standard operator character occurs somewhere in its name.
			if !strings.ContainsAny(source[start:i], "~!@#%^&|`?") {
				for i > start+1 && (source[i-1] == '+' || source[i-1] == '-') {
					i--
				}
			}
			if i-start > 63 {
				return fail("operator_length", start, i)
			}
			value = source[start:i]
		default:
			return fail("invalid_character", start, start+1)
		}
		token := pgToken{Kind: kind, Text: value, Start: start, End: i, Quoted: quoted}
		tokens = append(tokens, token)
		if kind == pgWhitespace || kind == pgComment {
			continue
		}
		if kind == pgSymbol {
			switch value {
			case "(", "[":
				if len(stack) >= l.MaxDepth {
					return fail("depth_limit", start, i)
				}
				stack = append(stack, c)
			case ")", "]":
				if len(stack) == 0 || (c == ')' && stack[len(stack)-1] != '(') || (c == ']' && stack[len(stack)-1] != '[') {
					return fail("unmatched_delimiter", start, i)
				}
				stack = stack[:len(stack)-1]
			case ";":
				if len(stack) != 0 {
					return fail("unmatched_delimiter", start, i)
				}
				if significant {
					if len(statements) >= l.MaxStatements {
						return fail("statement_limit", segmentStart, i)
					}
					statements = append(statements, pgStatementTokens{First: first, Last: len(tokens), Span: SQLSpan{segmentStart, i}})
					first = len(tokens)
					segmentStart = i
				}
				significant = false
				continue
			}
		}
		if !significant && kind == pgIdentifier && !quoted && value == "copy" {
			return fail("copy_unsupported", start, i)
		}
		significant = true
	}
	if len(stack) != 0 {
		return fail("unmatched_delimiter", len(source), len(source))
	}
	if significant {
		if len(statements) >= l.MaxStatements {
			return fail("statement_limit", segmentStart, len(source))
		}
		statements = append(statements, pgStatementTokens{First: first, Last: len(tokens), Span: SQLSpan{segmentStart, len(source)}})
	}
	if len(statements) > 0 {
		statements[len(statements)-1].Span.End = len(source)
		statements[len(statements)-1].Last = len(tokens)
	}
	tokens = append(tokens, pgToken{Kind: pgEOF, Start: len(source), End: len(source)})
	return tokens, statements, nil
}
