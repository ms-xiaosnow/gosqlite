package postgres

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func fullTokens(raw string) ([]pgToken, error) {
	ts, segs, err := pgScan(raw, DefaultSQLLimits())
	if err != nil || len(segs) > 1 {
		return nil, fmt.Errorf("unsupported expression")
	}
	out := []pgToken{}
	for _, t := range ts {
		if t.Kind == pgEOF || t.Kind == pgWhitespace {
			continue
		}
		if t.Kind == pgComment || t.Text == ";" {
			return nil, fmt.Errorf("unsupported expression trivia")
		}
		out = append(out, t)
	}
	return fullUnparen(out), nil
}
func fullUnparen(ts []pgToken) []pgToken {
	for len(ts) >= 2 && ts[0].Text == "(" && ts[len(ts)-1].Text == ")" {
		depth := 0
		whole := true
		for i, t := range ts {
			if t.Text == "(" {
				depth++
			}
			if t.Text == ")" {
				depth--
			}
			if depth == 0 && i < len(ts)-1 {
				whole = false
				break
			}
		}
		if !whole {
			break
		}
		ts = ts[1 : len(ts)-1]
	}
	return ts
}
func fullKeyword(t pgToken, name string) bool { return !t.Quoted && fullName(t, name) }
func fullName(t pgToken, name string) bool    { return t.Kind == pgIdentifier && t.Text == name }
func fullQualified(ts []pgToken, schema, name string) bool {
	return len(ts) == 1 && fullName(ts[0], name) || len(ts) == 3 && fullName(ts[0], schema) && ts[1].Text == "." && fullName(ts[2], name)
}
func fullCastMatches(ts []pgToken, oid uint32) bool {
	name := fullScalarTypes[oid]
	if fullQualified(ts, "pg_catalog", name) {
		return true
	}
	// Only parser-reserved aliases with the same built-in identity are accepted.
	aliases := map[uint32]string{16: "boolean", 20: "bigint", 21: "smallint", 23: "integer", 700: "real"}
	if aliases[oid] != "" && len(ts) == 1 && !ts[0].Quoted && fullName(ts[0], aliases[oid]) {
		return true
	}
	return oid == 701 && len(ts) == 2 && !ts[0].Quoted && !ts[1].Quoted && fullName(ts[0], "double") && fullName(ts[1], "precision")
}
func fullLiteral(ts []pgToken, oid uint32) (string, error) {
	ts = fullUnparen(ts)
	fail := func() (string, error) { return "", fmt.Errorf("unsupported typed literal") }
	cast := -1
	for i, t := range ts {
		if t.Text == "::" {
			if cast != -1 {
				return fail()
			}
			cast = i
		}
	}
	if cast >= 0 {
		if !fullCastMatches(ts[cast+1:], oid) {
			return fail()
		}
		ts = fullUnparen(ts[:cast])
	}
	typ := fullScalarTypes[oid]
	if typ == "" {
		return fail()
	}
	suffix := "::" + objectKey("pg_catalog", typ)
	if oid == 1021 {
		suffix = "::pg_catalog.float4[]"
	}
	if len(ts) == 1 && fullName(ts[0], "null") && !ts[0].Quoted {
		return "NULL" + suffix, nil
	}
	if oid == 16 && len(ts) == 1 && !ts[0].Quoted && (fullName(ts[0], "true") || fullName(ts[0], "false")) {
		return strings.ToUpper(ts[0].Text) + suffix, nil
	}
	if oid == 25 && len(ts) == 1 && ts[0].Kind == pgString {
		return transitionLiteral(ts[0].Text) + suffix, nil
	}
	if oid != 20 && oid != 21 && oid != 23 && oid != 700 && oid != 701 {
		return fail()
	}
	sign := ""
	if len(ts) == 2 && (ts[0].Text == "-" || ts[0].Text == "+") {
		sign = ts[0].Text
		ts = ts[1:]
	}
	if len(ts) != 1 || (ts[0].Kind != pgNumber && ts[0].Kind != pgString) || len(ts[0].Text) > 128 {
		return fail()
	}
	value := sign + ts[0].Text
	if oid == 20 || oid == 21 || oid == 23 {
		bits := 64
		if oid == 21 {
			bits = 16
		}
		if oid == 23 {
			bits = 32
		}
		n, e := strconv.ParseInt(value, 10, bits)
		if e != nil {
			return fail()
		}
		// Cast the signed text as one value: PostgreSQL's :: binds more tightly
		// than unary minus, so -32768::int2 would cast the overflowing positive
		// magnitude before negating it.
		return transitionLiteral(strconv.FormatInt(n, 10)) + suffix, nil
	}
	bits := 64
	if oid == 700 {
		bits = 32
	}
	n, e := strconv.ParseFloat(value, bits)
	if e != nil || math.IsInf(n, 0) || math.IsNaN(n) || n == 0 && nonzeroDecimalSignificand(value) {
		return fail()
	}
	return transitionLiteral(strconv.FormatFloat(n, 'g', -1, bits)) + suffix, nil
}
func (f *fullTableModel) defaultSQL(d FullDefault, a Column) (string, error) {
	ts, err := fullTokens(d.Expression)
	if err != nil {
		return "", err
	}
	if d.SequenceOID == 0 {
		return fullLiteral(ts, a.TypeOID)
	}
	seq, ok := f.sequences[d.SequenceOID]
	if !ok || seq.OwnedAttNum != a.AttNum || (a.TypeOID != 20 && a.TypeOID != 23 && a.TypeOID != 21) {
		return "", fmt.Errorf("sequence identity")
	}
	// Only nextval('exact qualified sequence'::regclass), optionally pg_catalog
	// qualified, is recognized. No callable user expression reaches the renderer.
	open := -1
	for i, t := range ts {
		if t.Text == "(" {
			open = i
			break
		}
	}
	if open < 0 || !fullQualified(ts[:open], "pg_catalog", "nextval") || ts[len(ts)-1].Text != ")" {
		return "", fmt.Errorf("sequence expression")
	}
	args := ts[open+1 : len(ts)-1]
	if len(args) < 3 || args[0].Kind != pgString || args[1].Text != "::" || !fullQualified(args[2:], "pg_catalog", "regclass") {
		return "", fmt.Errorf("sequence argument")
	}
	names, err := fullTokens(args[0].Text)
	if err != nil || len(names) != 3 || !fullName(names[0], seq.Schema) || names[1].Text != "." || !fullName(names[2], seq.Name) {
		return "", fmt.Errorf("sequence name binding")
	}
	return "pg_catalog.nextval(" + transitionLiteral(objectKey(seq.Schema, seq.Name)) + "::pg_catalog.regclass)", nil
}
func (f *fullTableModel) checkSQL(c FullConstraint) (string, error) {
	ts, err := fullTokens(c.Expression)
	if err != nil {
		return "", err
	}
	a := f.cols[c.Columns[0]]
	if len(ts) < 3 || !fullName(ts[0], a.Name) {
		return "", fmt.Errorf("check column binding")
	}
	if len(ts) == 3 && fullKeyword(ts[1], "is") && fullKeyword(ts[2], "null") {
		return quoteIdent(a.Name) + " IS NULL", nil
	}
	if len(ts) == 4 && fullKeyword(ts[1], "is") && fullKeyword(ts[2], "not") && fullKeyword(ts[3], "null") {
		return quoteIdent(a.Name) + " IS NOT NULL", nil
	}
	if ts[1].Kind != pgSymbol || !map[string]bool{"=": true, "<>": true, "!=": true, "<": true, ">": true, "<=": true, ">=": true}[ts[1].Text] || a.TypeOID == 17 || a.TypeOID == 1021 {
		return "", fmt.Errorf("check operator")
	}
	lit, err := fullLiteral(ts[2:], a.TypeOID)
	if err != nil {
		return "", err
	}
	return quoteIdent(a.Name) + " OPERATOR(pg_catalog." + ts[1].Text + ") " + lit, nil
}
