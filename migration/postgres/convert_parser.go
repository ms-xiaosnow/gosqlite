package postgres

import "strconv"

type pgStatement struct {
	Kind        string
	Span        SQLSpan
	Select      *pgSelect
	Insert      *pgInsert
	Update      *pgUpdate
	CreateTable *pgCreateTable
	CreateIndex *pgCreateIndex
}
type pgSelect struct {
	Columns []pgResultColumn
	From    []pgToken
	Alias   pgToken
	Where   *pgExpr
	OrderBy []pgOrderTerm
	Limit   *pgExpr
	Span    SQLSpan
}
type pgResultColumn struct {
	Expr  *pgExpr
	Alias pgToken
	Span  SQLSpan
}
type pgOrderTerm struct {
	Expr                    *pgExpr
	Desc, DirectionExplicit bool
	Nulls                   string
	Span, NullsSpan         SQLSpan
}
type pgInsert struct {
	Table   []pgToken
	Columns []pgToken
	Rows    [][]*pgExpr
	Span    SQLSpan
}
type pgUpdate struct {
	Table       []pgToken
	Alias       pgToken
	Assignments []pgAssignment
	Where       *pgExpr
	Span        SQLSpan
}
type pgAssignment struct {
	Column pgToken
	Expr   *pgExpr
	Span   SQLSpan
}
type pgCreateTable struct {
	Name        []pgToken
	IfNotExists bool
	Columns     []pgColumnDef
	Span        SQLSpan
}
type pgColumnDef struct {
	Name                pgToken
	Type                *pgType
	Default             *pgExpr
	NotNull, PrimaryKey bool
	Span                SQLSpan
}
type pgCreateIndex struct {
	Name, Table    []pgToken
	Method, Column pgToken
	OpClass        []pgToken
	Options        []pgIndexOption
	Span, BodySpan SQLSpan
}
type pgIndexOption struct {
	Name, Value pgToken
	Span        SQLSpan
}
type pgType struct {
	Name      []pgToken
	Args      []pgToken
	ArrayDims int
	Span      SQLSpan
}
type pgExpr struct {
	Kind        string
	Span        SQLSpan
	Token       pgToken
	Name        []pgToken
	Op          pgToken
	Left, Right *pgExpr
	Args        []*pgExpr
	Type        *pgType
	height      int
}

type pgParser struct {
	source     string
	tokens     []pgToken
	pos, depth int
	limits     SQLLimits
	err        error
}

// pgParseStatement consumes exactly one scanner-delimited statement. Any
// unsupported suffix invalidates the entire AST, never just the suffix.
func pgParseStatement(source string, tokens []pgToken, statement pgStatementTokens, limits SQLLimits) (*pgStatement, error) {
	l, err := limits.normalized()
	if err != nil {
		return nil, err
	}
	if len(source) > l.MaxBytes || len(tokens) > l.MaxTokens {
		return nil, &SQLParseError{Code: "resource_limit", Span: statement.Span}
	}
	if statement.First < 0 || statement.Last < statement.First || statement.Last > len(tokens) || statement.Span.Start < 0 || statement.Span.End < statement.Span.Start || statement.Span.End > len(source) {
		return nil, &SQLParseError{Code: "invalid_statement_range"}
	}
	p := &pgParser{source: source, limits: l}
	for _, t := range tokens[statement.First:statement.Last] {
		if !pgTrivia(t) && t.Kind != pgEOF {
			p.tokens = append(p.tokens, t)
		}
	}
	for len(p.tokens) > 0 && p.tokens[0].Kind == pgSymbol && p.tokens[0].Text == ";" {
		p.tokens = p.tokens[1:]
	}
	for len(p.tokens) > 0 && p.tokens[len(p.tokens)-1].Kind == pgSymbol && p.tokens[len(p.tokens)-1].Text == ";" {
		p.tokens = p.tokens[:len(p.tokens)-1]
	}
	body := SQLSpan{statement.Span.End, statement.Span.End}
	if len(p.tokens) > 0 {
		body = SQLSpan{p.tokens[0].Start, p.tokens[len(p.tokens)-1].End}
	}
	p.tokens = append(p.tokens, pgToken{Kind: pgEOF, Start: body.End, End: body.End})
	s := &pgStatement{Span: statement.Span}
	switch {
	case p.take("select"):
		s.Kind = "select"
		s.Select = p.selectStatement()
		if s.Select != nil {
			s.Select.Span = body
		}
	case p.take("insert"):
		s.Kind = "insert"
		s.Insert = p.insertStatement()
		if s.Insert != nil {
			s.Insert.Span = body
		}
	case p.take("update"):
		s.Kind = "update"
		s.Update = p.updateStatement()
		if s.Update != nil {
			s.Update.Span = body
		}
	case p.take("create"):
		if p.take("table") {
			s.Kind = "create_table"
			s.CreateTable = p.createTable()
			if s.CreateTable != nil {
				s.CreateTable.Span = body
			}
		} else if p.take("index") {
			s.Kind = "create_index"
			s.CreateIndex = p.createIndex()
			if s.CreateIndex != nil {
				s.CreateIndex.Span = body
				s.CreateIndex.BodySpan = body
			}
		} else {
			p.fail("unsupported_statement")
		}
	default:
		p.fail("unsupported_statement")
	}
	if p.err == nil && p.peek().Kind != pgEOF {
		p.fail("unsupported_syntax")
	}
	if p.err != nil {
		return nil, p.err
	}
	return s, nil
}
func (p *pgParser) peek() pgToken { return p.tokens[p.pos] }
func (p *pgParser) previous() pgToken {
	if p.pos == 0 {
		return p.peek()
	}
	return p.tokens[p.pos-1]
}
func (p *pgParser) next() pgToken {
	t := p.peek()
	if p.err == nil && t.Kind != pgEOF {
		p.pos++
	}
	return t
}
func (p *pgParser) is(text string) bool {
	t := p.peek()
	return !t.Quoted && (t.Kind == pgIdentifier || t.Kind == pgSymbol) && t.Text == text
}
func (p *pgParser) take(text string) bool {
	if p.err == nil && p.is(text) {
		p.next()
		return true
	}
	return false
}
func (p *pgParser) need(text string) pgToken {
	t := p.peek()
	if !p.take(text) {
		p.fail("expected_syntax")
	}
	return t
}
func (p *pgParser) fail(code string) {
	if p.err == nil {
		t := p.peek()
		p.err = &SQLParseError{Code: code, Span: SQLSpan{t.Start, t.End}}
	}
}
func pgReserved(s string) bool {
	switch s {
	case "all", "and", "array", "as", "asc", "between", "by", "case", "cast", "check", "collate", "concurrently", "conflict", "constraint", "create", "cross", "default", "delete", "desc", "distinct", "do", "drop", "else", "end", "except", "exists", "false", "fetch", "filter", "for", "foreign", "from", "full", "group", "having", "if", "ilike", "in", "index", "inner", "insert", "intersect", "into", "is", "isnull", "join", "key", "lateral", "left", "like", "limit", "natural", "not", "notnull", "null", "nulls", "offset", "on", "only", "or", "order", "outer", "over", "primary", "references", "returning", "right", "select", "set", "similar", "table", "then", "true", "union", "unique", "update", "using", "values", "when", "where", "window", "with":
		return true
	}
	return false
}
func (p *pgParser) name() pgToken {
	t := p.peek()
	if t.Kind != pgIdentifier || (!t.Quoted && pgReserved(t.Text)) {
		p.fail("expected_identifier")
		return pgToken{}
	}
	p.next()
	return t
}
func (p *pgParser) names(max int) []pgToken {
	out := []pgToken{p.name()}
	for p.err == nil && p.take(".") {
		if len(out) >= max {
			p.fail("unsupported_name")
			break
		}
		out = append(out, p.name())
	}
	return out
}
func (p *pgParser) alias() pgToken {
	if p.take("as") {
		return p.name()
	}
	t := p.peek()
	if t.Kind == pgIdentifier && (t.Quoted || !pgReserved(t.Text)) {
		return p.name()
	}
	return pgToken{}
}
func (p *pgParser) selectStatement() *pgSelect {
	s := &pgSelect{}
	for p.err == nil {
		start := p.peek().Start
		e := p.expression(0)
		r := pgResultColumn{Expr: e}
		if p.take("as") {
			r.Alias = p.name()
		}
		r.Span = SQLSpan{start, p.previous().End}
		s.Columns = append(s.Columns, r)
		if !p.take(",") {
			break
		}
	}
	p.need("from")
	s.From = p.names(2)
	s.Alias = p.alias()
	if p.take("where") {
		s.Where = p.expression(0)
	}
	if p.take("order") {
		p.need("by")
		for p.err == nil {
			start := p.peek().Start
			o := pgOrderTerm{Expr: p.expression(0)}
			if p.take("asc") {
				o.DirectionExplicit = true
			} else if p.take("desc") {
				o.DirectionExplicit = true
				o.Desc = true
			}
			if p.take("nulls") {
				ns := p.previous().Start
				if p.take("first") {
					o.Nulls = "first"
				} else if p.take("last") {
					o.Nulls = "last"
				} else {
					p.fail("expected_null_order")
				}
				o.NullsSpan = SQLSpan{ns, p.previous().End}
			}
			o.Span = SQLSpan{start, p.previous().End}
			s.OrderBy = append(s.OrderBy, o)
			if !p.take(",") {
				break
			}
		}
	}
	if p.take("limit") {
		s.Limit = p.expression(0)
	}
	return s
}
func (p *pgParser) insertStatement() *pgInsert {
	s := &pgInsert{}
	p.need("into")
	s.Table = p.names(2)
	p.need("(")
	for p.err == nil {
		s.Columns = append(s.Columns, p.name())
		if !p.take(",") {
			break
		}
	}
	p.need(")")
	p.need("values")
	for p.err == nil {
		p.need("(")
		row := []*pgExpr{}
		for p.err == nil {
			row = append(row, p.expression(0))
			if !p.take(",") {
				break
			}
		}
		p.need(")")
		if len(row) != len(s.Columns) {
			p.fail("values_arity")
		}
		s.Rows = append(s.Rows, row)
		if !p.take(",") {
			break
		}
	}
	return s
}
func (p *pgParser) updateStatement() *pgUpdate {
	s := &pgUpdate{Table: p.names(2)}
	s.Alias = p.alias()
	p.need("set")
	for p.err == nil {
		a := pgAssignment{Column: p.name()}
		p.need("=")
		a.Expr = p.expression(0)
		a.Span = SQLSpan{a.Column.Start, p.previous().End}
		s.Assignments = append(s.Assignments, a)
		if !p.take(",") {
			break
		}
	}
	if p.take("where") {
		s.Where = p.expression(0)
	}
	return s
}
func (p *pgParser) createTable() *pgCreateTable {
	s := &pgCreateTable{}
	if p.take("if") {
		p.need("not")
		p.need("exists")
		s.IfNotExists = true
	}
	s.Name = p.names(2)
	p.need("(")
	for p.err == nil {
		c := pgColumnDef{Name: p.name()}
		c.Type = p.typeName()
		seenDefault, seenNull, seenPK := false, false, false
		for p.err == nil {
			if p.take("default") {
				if seenDefault {
					p.fail("duplicate_constraint")
				}
				seenDefault = true
				c.Default = p.expression(0)
			} else if p.take("not") {
				p.need("null")
				if seenNull {
					p.fail("duplicate_constraint")
				}
				seenNull = true
				c.NotNull = true
			} else if p.take("null") {
				if seenNull {
					p.fail("duplicate_constraint")
				}
				seenNull = true
			} else if p.take("primary") {
				p.need("key")
				if seenPK {
					p.fail("duplicate_constraint")
				}
				seenPK = true
				c.PrimaryKey = true
			} else {
				break
			}
		}
		c.Span = SQLSpan{c.Name.Start, p.previous().End}
		s.Columns = append(s.Columns, c)
		if !p.take(",") {
			break
		}
	}
	p.need(")")
	return s
}
func pgIntegerToken(t pgToken) bool {
	if t.Kind != pgNumber || len(t.Text) == 0 {
		return false
	}
	for _, c := range []byte(t.Text) {
		if !pgDigit(c) {
			return false
		}
	}
	return true
}
func (p *pgParser) createIndex() *pgCreateIndex {
	s := &pgCreateIndex{Name: p.names(1)}
	p.need("on")
	s.Table = p.names(2)
	p.need("using")
	s.Method = p.name()
	p.need("(")
	s.Column = p.name()
	if !p.is(")") && p.err == nil {
		s.OpClass = p.names(2)
	}
	p.need(")")
	if p.take("with") {
		p.need("(")
		seen := map[string]bool{}
		for p.err == nil {
			o := pgIndexOption{Name: p.name()}
			p.need("=")
			sign := pgToken{}
			if p.is("+") || p.is("-") {
				sign = p.next()
			}
			o.Value = p.next()
			if !pgIntegerToken(o.Value) {
				p.fail("expected_integer_option")
			}
			if sign.Kind != pgEOF {
				o.Value.Start = sign.Start
				o.Value.Text = sign.Text + o.Value.Text
			}
			if seen[o.Name.Text] {
				p.fail("duplicate_index_option")
			}
			seen[o.Name.Text] = true
			o.Span = SQLSpan{o.Name.Start, o.Value.End}
			s.Options = append(s.Options, o)
			if !p.take(",") {
				break
			}
		}
		p.need(")")
	}
	return s
}
func (p *pgParser) typeName() *pgType {
	start := p.peek().Start
	t := &pgType{Name: p.names(2)}
	if len(t.Name) == 1 && !t.Name[0].Quoted && t.Name[0].Text == "double" {
		t.Name = append(t.Name, p.need("precision"))
	}
	if p.take("(") {
		for p.err == nil {
			n := p.next()
			if !pgIntegerToken(n) {
				p.fail("expected_type_dimension")
			}
			t.Args = append(t.Args, n)
			if !p.take(",") {
				break
			}
		}
		p.need(")")
	}
	for p.take("[") {
		p.need("]")
		t.ArrayDims++
		if t.ArrayDims > p.limits.MaxDepth {
			p.fail("depth_limit")
		}
	}
	t.Span = SQLSpan{start, p.previous().End}
	return t
}

// These binding powers are PostgreSQL 15's precedence, not SQLite's. Generic
// native/user-defined operators (including bitwise/vector operators) share one
// level. Comparisons and IS tests are nonassociative.
func pgPrecedence(t pgToken) int {
	if t.Quoted {
		return -1
	}
	if t.Kind == pgIdentifier {
		switch t.Text {
		case "or":
			return 10
		case "and":
			return 20
		case "is", "is not":
			return 35
		case "like", "ilike":
			return 45
		}
		return -1
	}
	if t.Kind != pgSymbol {
		return -1
	}
	switch t.Text {
	case "=", "<", ">", "<=", ">=", "<>", "!=":
		return 40
	case "+", "-":
		return 60
	case "*", "/", "%":
		return 70
	case "^":
		return 75
	}
	if len(t.Text) > 0 && pgOp(t.Text[0]) {
		return 50
	}
	return -1
}
func (p *pgParser) node(e *pgExpr) *pgExpr {
	e.height = 1
	for _, child := range []*pgExpr{e.Left, e.Right} {
		if child != nil && child.height+1 > e.height {
			e.height = child.height + 1
		}
	}
	for _, child := range e.Args {
		if child != nil && child.height+1 > e.height {
			e.height = child.height + 1
		}
	}
	if e.height > p.limits.MaxDepth {
		p.fail("depth_limit")
		return nil
	}
	return e
}
func (p *pgParser) expression(min int) *pgExpr {
	if p.err != nil {
		return nil
	}
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > p.limits.MaxDepth {
		p.fail("depth_limit")
		return nil
	}
	e := p.prefix()
	for p.err == nil && e != nil {
		if p.is("::") {
			op := p.next()
			typ := p.typeName()
			e = p.node(&pgExpr{Kind: "cast", Span: SQLSpan{e.Span.Start, typ.Span.End}, Op: op, Left: e, Type: typ})
			continue
		}
		op := p.peek()
		prec := pgPrecedence(op)
		if prec < min || prec < 0 {
			break
		}
		if (prec == 35 || prec == 40) && e.Kind == "binary" && pgPrecedence(e.Op) == prec {
			p.fail("nonassociative_operator")
			return nil
		}
		p.next()
		if op.Text == "is" && p.take("not") {
			op.Text = "is not"
			op.End = p.previous().End
		}
		var right *pgExpr
		if prec == 35 {
			if !(p.is("null") || p.is("true") || p.is("false")) {
				p.fail("unsupported_is_test")
				return nil
			}
			right = p.prefix()
		} else {
			right = p.expression(prec + 1)
		}
		if p.err != nil || right == nil {
			return nil
		}
		e = p.node(&pgExpr{Kind: "binary", Span: SQLSpan{e.Span.Start, right.Span.End}, Op: op, Left: e, Right: right})
	}
	return e
}
func (p *pgParser) prefix() *pgExpr {
	t := p.peek()
	e := &pgExpr{Token: t, Span: SQLSpan{t.Start, t.End}}
	switch t.Kind {
	case pgString:
		e.Kind = "string"
		p.next()
	case pgEscapeString, pgDollarString, pgBitString, pgOpaque:
		e.Kind = "opaque"
		p.next()
	case pgNumber:
		e.Kind = "number"
		p.next()
	case pgParameter:
		e.Kind = "parameter"
		p.next()
		n, err := strconv.ParseUint(t.Text[1:], 10, 31)
		if err != nil || n == 0 {
			p.fail("invalid_parameter")
		}
	case pgIdentifier:
		switch {
		case p.take("null"):
			e.Kind = "null"
		case p.take("true") || p.take("false"):
			e.Kind = "boolean"
		case p.take("not"):
			e.Kind = "unary"
			e.Op = t
			e.Left = p.expression(30)
			if e.Left != nil {
				e.Span.End = e.Left.Span.End
			}
		case p.take("array"):
			e.Kind = "array"
			p.need("[")
			if !p.is("]") {
				for p.err == nil {
					if p.is("[") || p.is("array") {
						p.fail("multidimensional_array")
						break
					}
					arg := p.expression(0)
					e.Args = append(e.Args, arg)
					if !p.take(",") {
						break
					}
				}
			}
			p.need("]")
			e.Span.End = p.previous().End
		case p.take("cast"):
			e.Kind = "cast"
			e.Op = t
			p.need("(")
			e.Left = p.expression(0)
			p.need("as")
			e.Type = p.typeName()
			p.need(")")
			e.Span.End = p.previous().End
		default:
			e.Name = []pgToken{p.name()}
			for p.err == nil && p.take(".") {
				if p.take("*") {
					e.Kind = "star"
					break
				}
				if len(e.Name) >= 3 {
					p.fail("unsupported_name")
					break
				}
				e.Name = append(e.Name, p.name())
			}
			if e.Kind != "star" {
				e.Kind = "column"
				if p.take("(") {
					e.Kind = "call"
					if len(e.Name) > 2 {
						p.fail("unsupported_name")
					}
					if !p.is(")") {
						for p.err == nil {
							e.Args = append(e.Args, p.expression(0))
							if !p.take(",") {
								break
							}
						}
					}
					p.need(")")
				}
			}
			e.Span.End = p.previous().End
		}
	case pgSymbol:
		switch {
		case p.take("("):
			e.Kind = "paren"
			e.Left = p.expression(0)
			p.need(")")
			e.Span.End = p.previous().End
		case p.take("*"):
			e.Kind = "star"
		case t.Text == "+" || t.Text == "-" || t.Text == "~":
			p.next()
			e.Kind = "unary"
			e.Op = t
			prec := 80
			if t.Text == "~" {
				prec = 51
			}
			e.Left = p.expression(prec)
			if e.Left != nil {
				e.Span.End = e.Left.Span.End
			}
		default:
			p.fail("unsupported_expression")
		}
	default:
		p.fail("expected_expression")
	}
	if p.err != nil {
		return nil
	}
	return p.node(e)
}
