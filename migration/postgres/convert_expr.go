package postgres

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func (b *statementBinder) expression(e *pgExpr, expected boundType) (string, boundType, error) {
	text, typ, err := b.renderExpression(e, expected)
	if err != nil {
		return "", boundType{}, err
	}
	if expected.kind != "" && expected.kind != "unknown" {
		if typ.kind == "null" {
			if !expected.nullable {
				return "", typ, b.failure("null_not_allowed", e.Span, "A static NULL is incompatible with the required non-NULL value.")
			}
		} else if typ.kind != expected.kind && !(expected.kind == "real" && typ.kind == "integer") || typ.kind == "legacy" && typ.dims != expected.dims || typ.kind == "bit" && expected.dims != 0 && typ.dims != expected.dims {
			return "", typ, b.failure("type_mismatch", e.Span, "Expression types do not match the bound context.")
		}
	}
	if expected.oid == 21 || expected.oid == 23 || expected.oid == 20 {
		if scalar, ok := constantArrayNumber(e); ok && !strings.ContainsAny(scalar, ".eE") {
			n, err := strconv.ParseInt(scalar, 10, 64)
			if err != nil || expected.oid == 21 && (n < -32768 || n > 32767) || expected.oid == 23 && (n < -2147483648 || n > 2147483647) {
				return "", typ, b.failure("source_integer_range", e.Span, "Static assignment is outside the bound PostgreSQL integer width.")
			}
		}
	}
	if b.cv.opts.Target == TargetLocal && (expected.oid == 700 || expected.oid == 701) && (typ.kind == "integer" || expected.oid == 700 && typ.oid == 701) {
		return "", typ, b.failure("numeric_coercion_unsupported", e.Span, "Dynamic scalar coercion to a different source numeric width requires review.")
	}
	return text, typ, nil
}
func (b *statementBinder) renderExpression(e *pgExpr, expected boundType) (string, boundType, error) {
	if e == nil {
		return "", boundType{}, fmt.Errorf("convert: missing expression")
	}
	raw := b.raw(e.Span)
	switch e.Kind {
	case "column":
		return b.renderColumn(e)
	case "null":
		return raw, boundType{kind: "null", nullable: true}, nil
	case "boolean":
		if b.cv.opts.Target == TargetLocal {
			value := "(1=0)"
			if e.Token.Text == "true" {
				value = "(1=1)"
			}
			return value, boundType{kind: "bool"}, nil
		}
		return raw, boundType{kind: "bool"}, nil
	case "number":
		if len(e.Token.Text) > 128 || !finiteNumber(e.Token.Text) {
			return "", boundType{}, b.failure("numeric_literal_unsupported", e.Span, "Scalar numeric literal exceeds the finite bounded subset.")
		}
		typ := boundType{kind: "integer"}
		if strings.ContainsAny(e.Token.Text, ".eE") || expected.oid == 700 || expected.oid == 701 {
			if expected.oid != 700 && expected.oid != 701 {
				return "", typ, b.failure("numeric_semantics_unsupported", e.Span, "Noninteger decimal literals require a verified source float4/float8 context; arbitrary PostgreSQL numeric is not emulated.")
			}
			bits := 64
			if expected.oid == 700 {
				bits = 32
			}
			value, err := strconv.ParseFloat(e.Token.Text, bits)
			if err != nil || value == 0 && nonzeroDecimalSignificand(e.Token.Text) {
				return "", typ, b.failure("source_float_range", e.Span, "Source floating literal overflows or underflows its PostgreSQL width.")
			}
			if b.cv.opts.Target == TargetLocal {
				raw = strconv.FormatFloat(value, 'g', -1, 64)
			}
			typ.kind, typ.oid = "real", expected.oid
		} else {
			if _, err := strconv.ParseInt(e.Token.Text, 10, 64); err != nil {
				return "", typ, b.failure("integer_range", e.Span, "Scalar integer literal is outside signed 64-bit range.")
			}
		}
		return raw, typ, nil
	case "string":
		if expected.kind == "legacy" {
			v, err := parseConverterRealArray(e.Token.Text, expected.dims)
			if err != nil {
				return "", boundType{}, b.failure("invalid_vector_literal", e.Span, "Source real-array literal is invalid for the selected dimensions.")
			}
			return b.vectorLiteral(v), boundType{kind: "legacy", dims: expected.dims}, nil
		}
		if expected.kind == "bit" {
			v, err := vector.ParseBit(e.Token.Text)
			if err != nil || v.Dims() != expected.dims {
				return "", boundType{}, b.failure("invalid_bit_literal", e.Span, "Bit literal must have the exact selected width.")
			}
			if b.cv.opts.Target == TargetLocal {
				return "bit_vector(" + raw + ")", boundType{kind: "bit", dims: expected.dims}, nil
			}
			return raw + fmt.Sprintf("::bit(%d)", expected.dims), boundType{kind: "bit", dims: expected.dims}, nil
		}
		return raw, boundType{kind: "text"}, nil
	case "opaque":
		// Dollar bodies and E/Unicode strings stay opaque evidence. No SQL inside
		// them is visited, and an unsupported statement never yields converted SQL.
		return "", boundType{}, b.failure("opaque_literal_requires_review", e.Span, "Dollar, escape, bit-prefixed and Unicode literal forms require manual review in this converter subset.")
	case "parameter":
		n, err := strconv.Atoi(strings.TrimPrefix(e.Token.Text, "$"))
		if err != nil || n < 1 || n > 32766 {
			return "", boundType{}, b.failure("parameter_range", e.Span, "Parameter index must be in 1..32766.")
		}
		if expected.kind == "" || expected.kind == "unknown" {
			return "", boundType{}, b.failure("parameter_type_required", e.Span, "Scalar parameters need a proven source type context.")
		}
		if expected.kind == "bit" {
			return "", boundType{}, b.failure("bit_parameter_unsupported", e.Span, "Fixed-width bit parameters require a separate canonical bind adapter not implemented by this checkpoint.")
		}
		p := BindTransform{Parameter: n, SourceKind: expected.kind, Target: b.cv.opts.Target, Action: "identity"}
		p.ScalarOID = expected.oid
		if expected.kind == "real" && expected.oid != 700 && expected.oid != 701 {
			return "", boundType{}, b.failure("parameter_type_required", e.Span, "Real parameters require a verified float4 or float8 source context.")
		}
		if expected.kind != "" && expected.kind != "unknown" {
			p.RejectNull = !expected.nullable
		}
		p.NonNegative = expected.nonNegative
		switch expected.oid {
		case 21:
			p.IntegerBits = 16
		case 23:
			p.IntegerBits = 32
		case 20:
			p.IntegerBits = 64
		}

		typ := expected
		if typ.kind == "" || typ.kind == "unknown" {
			typ = boundType{kind: "unknown", nullable: true}
			p.SourceKind = "scalar"
		}
		typ.nullable = true
		if expected.kind == "legacy" {
			p.SourceKind = "real[]"
			p.Dimensions = expected.dims
			p.Action = "real_array_to_vector"
		}
		if expected.kind == "array" && b.cv.opts.Target == TargetLocal {
			return "", typ, b.failure("array_parameter_unsupported", e.Span, "General array parameters have no local mapping.")
		}
		if previous, ok := b.parameters[n]; ok && (previous.SourceKind != p.SourceKind || previous.Dimensions != p.Dimensions || previous.Action != p.Action || p.SourceKind == "real" && previous.ScalarOID != p.ScalarOID) {
			return "", typ, b.failure("incompatible_parameter_reuse", e.Span, "A parameter is reused with incompatible scalar/array/vector or dimension requirements.")
		}
		if previous, ok := b.parameters[n]; ok {
			p.RejectNull = p.RejectNull || previous.RejectNull
			p.NonNegative = p.NonNegative || previous.NonNegative
			if previous.IntegerBits != 0 && (p.IntegerBits == 0 || previous.IntegerBits < p.IntegerBits) {
				p.IntegerBits = previous.IntegerBits
			}
		}
		b.parameters[n] = p
		if b.cv.opts.Target == TargetLocal {
			return fmt.Sprintf("?%d", n), typ, nil
		}
		if expected.kind == "legacy" {
			return raw + "::" + b.vectorType(expected.dims), typ, nil
		}
		return raw, typ, nil
	case "array":
		if expected.kind != "legacy" {
			return "", boundType{}, b.failure("unbound_array", e.Span, "ARRAY conversion requires a positively bound selected vector context.")
		}
		var components []string
		for _, arg := range e.Args {
			n, ok := constantArrayNumber(arg)
			if !ok {
				return "", boundType{}, b.failure("nonconstant_array", arg.Span, "Only one-dimensional constant decimal arrays are supported.")
			}
			components = append(components, n)
		}
		v, err := parseConverterRealArray("{"+strings.Join(components, ",")+"}", expected.dims)
		if err != nil {
			return "", boundType{}, b.failure("invalid_vector_literal", e.Span, "ARRAY is invalid for the selected finite binary32 dimensions.")
		}
		return b.withTrivia(e.Span, b.vectorLiteral(v)), boundType{kind: "legacy", dims: expected.dims}, nil
	case "paren":
		text, t, err := b.expression(e.Left, expected)
		if err != nil {
			return "", t, err
		}
		out, err := b.compose(e.Span, SQLEdit{e.Left.Span, text})
		return out, t, err
	case "cast":
		declared, err := b.declaredType(e.Type)
		if err != nil {
			return "", boundType{}, err
		}
		if declared.kind == "array" {
			if expected.kind != "legacy" {
				return "", boundType{}, b.failure("unbound_array_cast", e.Span, "A real[] cast alone does not establish selected vector dimensions.")
			}
			text, t, err := b.expression(e.Left, expected)
			if err != nil {
				return "", t, err
			}
			suffix := SQLSpan{e.Left.Span.End, e.Span.End}
			edits := []SQLEdit{{e.Left.Span, text}, {suffix, b.withTrivia(suffix, "")}}
			if e.Op.Text == "cast" {
				prefix := SQLSpan{e.Span.Start, e.Left.Span.Start}
				edits = append(edits, SQLEdit{prefix, b.withTrivia(prefix, "")})
			}
			out, err := b.compose(e.Span, edits...)
			return out, t, err
		}
		// General casts can change numeric width/overflow and string semantics.
		return "", boundType{}, b.failure("cast_semantics_unsupported", e.Span, "Only bound real[] vector casts are converted; other casts require review.")
	case "unary":
		op := e.Op.Text
		if b.cv.opts.Target == TargetLocal && (op == "+" || op == "-") && e.Left.Kind != "number" {
			return "", boundType{}, b.failure("scalar_arithmetic_unsupported", e.Span, "Nonliteral numeric unary operations require PostgreSQL overflow semantics not implemented locally.")
		}
		want := boundType{}
		switch op {
		case "+", "-":
			want.kind = "real"
			want.oid = expected.oid
		case "~":
			want.kind = "integer"
		case "not":
			want.kind = "bool"
		default:
			return "", boundType{}, b.failure("unknown_operator", tokenSpan(e.Op), "Unary operator is outside the supported subset.")
		}
		want.nullable = true
		// Handle signed int64 minimum without first parsing its positive magnitude.
		if op == "-" && e.Left.Kind == "number" && e.Left.Token.Text == "9223372036854775808" {
			return raw, boundType{kind: "integer"}, nil
		}
		text, t, err := b.expression(e.Left, want)
		if err != nil {
			return "", t, err
		}
		out, err := b.compose(e.Span, SQLEdit{e.Left.Span, text})
		if err != nil {
			return "", t, err
		}
		if b.cv.opts.Target == TargetLocal {
			out = "(" + out + ")"
		}
		return out, t, nil
	case "binary":
		return b.binaryExpression(e)
	case "call":
		return b.callExpression(e)
	}
	return "", boundType{}, b.failure("expression_unsupported", e.Span, "Expression is outside the supported typed subset.")
}
func (b *statementBinder) renderColumn(e *pgExpr) (string, boundType, error) {
	t, s, err := b.columnExpr(e, false)
	return s, t, err
}
func constantArrayNumber(e *pgExpr) (string, bool) {
	if e.Kind == "number" && len(e.Token.Text) <= 128 {
		return e.Token.Text, true
	}
	if e.Kind == "unary" && (e.Op.Text == "-" || e.Op.Text == "+") && e.Left.Kind == "number" && len(e.Left.Token.Text) < 128 {
		return e.Op.Text + e.Left.Token.Text, true
	}
	return "", false
}
func (b *statementBinder) distanceOperands(left, right *pgExpr, span SQLSpan) (string, string, boundType, error) {
	lt, err := b.hint(left)
	if err != nil {
		return "", "", lt, err
	}
	rt, err := b.hint(right)
	if err != nil {
		return "", "", rt, err
	}
	want := boundType{nullable: true}
	if lt.kind == "legacy" {
		want.kind, want.dims = "legacy", lt.dims
	} else if rt.kind == "legacy" {
		want.kind, want.dims = "legacy", rt.dims
	} else if lt.kind == "bit" && rt.kind == "bit" {
		want.kind, want.dims = "bit", lt.dims
	} else {
		return "", "", want, b.failure("unbound_distance_operator", span, "Distance operands require a selected legacy vector or two fixed-width bit columns.")
	}
	l, lt, err := b.expression(left, want)
	if err != nil {
		return "", "", want, err
	}
	r, rt, err := b.expression(right, want)
	if err != nil {
		return "", "", want, err
	}
	want.nullable = lt.nullable || rt.nullable
	return l, r, want, nil
}
func (b *statementBinder) binaryExpression(e *pgExpr) (string, boundType, error) {
	op := e.Op.Text
	if op == "<~>" || op == "<->" || op == "<=>" || op == "<%>" {
		l, r, t, err := b.distanceOperands(e.Left, e.Right, e.Span)
		if err != nil {
			return "", t, err
		}
		targetOp := op
		if t.kind == "legacy" {
			if op == "<%>" || !b.verifiedOperator(op) {
				return "", t, b.failure("operator_identity_mismatch", tokenSpan(e.Op), "Operator ownership and real[] signature do not identify the pinned legacy extension.")
			}
			if op == "<~>" {
				targetOp = "<+>"
			}
			if b.cv.opts.Target == TargetPostgres {
				targetOp = "OPERATOR(" + quoteIdent(b.cv.opts.VectorSchema) + "." + targetOp + ")"
			}
			b.warning("numeric_semantics_difference", e.Span, "Target distance arithmetic differs from legacy float32 accumulation; local calculations use compensated float64.")
			if op == "<=>" {
				b.warning("cosine_zero_norm_policy", e.Span, "Local cosine of a zero vector errors; legacy/pgvector behavior differs. Source/application values require zero-norm review.")
			}
		} else if !b.verifiedBitOperator(op) {
			return "", t, b.failure("bit_operator_identity_mismatch", tokenSpan(e.Op), "Bit operator ownership and signature do not identify the pinned pgvector profile.")
		} else if b.cv.opts.Target == TargetPostgres {
			targetOp = "OPERATOR(" + quoteIdent(b.cv.opts.VectorSchema) + "." + op + ")"
		} else if op != "<~>" && op != "<%>" {
			return "", t, b.failure("bit_metric_unsupported", tokenSpan(e.Op), "Only Hamming/Jaccard operators apply to bit operands.")
		}
		out, err := b.compose(e.Span, SQLEdit{e.Left.Span, l}, SQLEdit{tokenSpan(e.Op), targetOp}, SQLEdit{e.Right.Span, r})
		if err != nil {
			return "", t, err
		}
		return "(" + out + ")", boundType{kind: "real", nullable: t.nullable}, nil
	}
	if op == "is" || op == "is not" {
		if e.Right.Kind != "null" {
			return "", boundType{}, b.failure("is_semantics_unsupported", e.Span, "Only IS NULL and IS NOT NULL are supported.")
		}
		l, _, err := b.expression(e.Left, boundType{})
		if err != nil {
			return "", boundType{}, err
		}
		out, err := b.compose(e.Span, SQLEdit{e.Left.Span, l})
		return "(" + out + ")", boundType{kind: "bool"}, err
	}
	want := boundType{nullable: true}
	output := ""
	switch op {
	case "and", "or":
		want.kind = "bool"
		output = "bool"
	case "&", "|", "<<", ">>":
		want.kind = "integer"
		output = "integer"
		leftHint, e1 := b.hint(e.Left)
		rightHint, e2 := b.hint(e.Right)
		if e1 != nil {
			return "", want, e1
		}
		if e2 != nil {
			return "", want, e2
		}
		if leftHint.oid == 20 || rightHint.oid == 20 {
			want.oid = 20
		} else if leftHint.oid == 23 || rightHint.oid == 23 {
			want.oid = 23
		} else if leftHint.oid == 21 || rightHint.oid == 21 {
			want.oid = 21
		}
	case "+", "-", "*", "/", "%":
		if b.cv.opts.Target == TargetLocal {
			return "", want, b.failure("scalar_arithmetic_unsupported", e.Span, "Scalar arithmetic requires PostgreSQL overflow/division/error semantics not implemented by this local converter.")
		}
		want.kind = "real"
		output = "real"
	case "=", "<>", "!=", "<", ">", "<=", ">=":
		l, err := b.hint(e.Left)
		if err != nil {
			return "", want, err
		}
		r, err := b.hint(e.Right)
		if err != nil {
			return "", want, err
		}
		if b.cv.opts.Target == TargetLocal && (l.kind == "real" || r.kind == "real") && l.kind != "unknown" && r.kind != "unknown" && l.kind != "null" && r.kind != "null" && (l.kind != r.kind || l.oid == 0 || l.oid != r.oid) {
			return "", want, b.failure("numeric_comparison_coercion_unsupported", e.Span, "Mixed numeric comparisons require verified PostgreSQL operator coercion beyond this local subset.")
		}
		if l.kind != "unknown" && l.kind != "null" {
			want.kind = l.kind
			want.oid = l.oid
		} else if r.kind != "unknown" && r.kind != "null" {
			want.kind = r.kind
			want.oid = r.oid
		}
		if want.kind == "legacy" || want.kind == "array" || want.kind == "bit" || want.kind == "" {
			return "", want, b.failure("comparison_type_unsupported", e.Span, "Only positively bound scalar comparisons are supported.")
		}
		if want.kind == "text" && b.cv.opts.Target == TargetLocal {
			return "", want, b.failure("text_collation_unverified", e.Span, "Local text comparison requires a verified source collation contract absent from this catalog profile.")
		}
		output = "bool"
	default:
		return "", want, b.failure("unknown_operator", tokenSpan(e.Op), "Operator is outside the supported migration subset.")
	}
	l, lt, err := b.expression(e.Left, want)
	if err != nil {
		return "", lt, err
	}
	r, rt, err := b.expression(e.Right, want)
	if err != nil {
		return "", rt, err
	}
	if output == "real" && lt.kind == "integer" && rt.kind == "integer" {
		output = "integer"
	}
	// PostgreSQL shifts mask counts and narrower integers can overflow unlike
	// SQLite. Preserve only a constant in-range count, and require signed int8
	// columns when a shift's operand is not a small constant.
	if op == "<<" || op == ">>" {
		n, ok := constantArrayNumber(e.Right)
		v, err := strconv.ParseInt(n, 10, 64)
		if !ok || err != nil || v < 0 || v > 31 || lt.oid != 20 && e.Left.Kind != "number" {
			return "", lt, b.failure("shift_semantics_review", e.Span, "Shift conversion requires an int8 left operand or a constant, and a constant count in 0..31.")
		}
		if e.Left.Kind == "number" && op == "<<" {
			left, err := strconv.ParseInt(e.Left.Token.Text, 10, 64)
			if err != nil || left > int64(2147483647)>>uint(v) {
				return "", lt, b.failure("shift_semantics_review", e.Span, "Constant shift could overflow the PostgreSQL integer width.")
			}
		}
	}
	out, err := b.compose(e.Span, SQLEdit{e.Left.Span, l}, SQLEdit{e.Right.Span, r})
	return "(" + out + ")", boundType{kind: output, nullable: lt.nullable || rt.nullable}, err
}
func (b *statementBinder) callExpression(e *pgExpr) (string, boundType, error) {
	if len(e.Name) < 1 || len(e.Name) > 2 || len(e.Args) != 2 {
		return "", boundType{}, b.failure("function_unsupported", e.Span, "Only verified two-argument legacy distance functions are supported.")
	}
	name := e.Name[len(e.Name)-1].Text
	targetName := ""
	switch name {
	case "l2_distance", "cosine_distance":
		targetName = name
	case "manhattan_distance":
		targetName = "l1_distance"
	default:
		return "", boundType{}, b.failure("function_unsupported", pgNameSpan(e.Name), "Function is outside the supported verified signature subset.")
	}
	matches := 0
	verified := false
	for _, f := range b.cv.catalog.Functions {
		if f.Name == name && (len(e.Name) == 1 || f.Schema == e.Name[0].Text) {
			matches++
			verified = b.verifiedFunction(f, name)
		}
	}
	if matches != 1 || !verified {
		return "", boundType{}, b.failure("function_identity_mismatch", pgNameSpan(e.Name), "Function name/signature/owner is ambiguous or does not identify the pinned extension.")
	}
	l, r, t, err := b.distanceOperands(e.Args[0], e.Args[1], e.Span)
	if err != nil {
		return "", t, err
	}
	if t.kind != "legacy" {
		return "", t, b.failure("function_type_mismatch", e.Span, "Legacy distance function requires selected real[] vector operands.")
	}
	if b.cv.opts.Target == TargetPostgres {
		targetName = quoteIdent(b.cv.opts.VectorSchema) + "." + quoteIdent(targetName)
	}
	out, err := b.compose(e.Span, SQLEdit{pgNameSpan(e.Name), b.withTrivia(pgNameSpan(e.Name), targetName)}, SQLEdit{e.Args[0].Span, l}, SQLEdit{e.Args[1].Span, r})
	b.warning("numeric_semantics_difference", e.Span, "Legacy float32 function results become target vector distances with different accumulation/return precision.")
	if name == "cosine_distance" {
		b.warning("cosine_zero_norm_policy", e.Span, "Local cosine of a zero vector errors; application values require review.")
	}
	return out, boundType{kind: "real", nullable: t.nullable}, err
}

func (b *statementBinder) verifiedBitOperator(name string) bool {
	resultType := b.cv.types[701]
	if resultType.Schema != "pg_catalog" || resultType.Name != "float8" || resultType.Kind != "b" || resultType.BaseOID != 0 || resultType.ElementOID != 0 {
		return false
	}
	want := ""
	switch name {
	case "<~>":
		want = "hamming_distance"
	case "<%>":
		want = "jaccard_distance"
	default:
		return false
	}
	var ext Extension
	for _, e := range b.cv.catalog.Extensions {
		if e.Name == "vector" && e.Version == "0.8.7" {
			ext = e
		}
	}
	if ext.OID == 0 {
		return false
	}
	matches := 0
	valid := false
	for _, op := range b.cv.catalog.Operators {
		if op.Name != name || op.LeftOID != 1560 || op.RightOID != 1560 {
			continue
		}
		matches++
		for _, f := range b.cv.catalog.Functions {
			if f.OID == op.FunctionOID && f.Name == want && f.Schema == ext.Schema && f.ExtensionOID == ext.OID && op.Schema == ext.Schema && op.ExtensionOID == ext.OID && len(f.ArgumentOIDs) == 2 && f.ArgumentOIDs[0] == 1560 && f.ArgumentOIDs[1] == 1560 && f.ReturnOID == 701 {
				valid = true
			}
		}
	}
	return matches == 1 && valid
}
