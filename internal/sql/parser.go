package sql

import (
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const MaxParameters = 32766
const MaxSQLBytes = 1 << 20
const MaxTokens = 32768
const MaxColumns = 2000
const MaxFunctionArgs = 1000
const maxExpressionDepth = 128

type parser struct {
	text               string
	tokens             []Token
	pos, params, depth int
	named              map[string]int
	bindings           map[int]Parameter
	heights            map[Expr]int
}

func Parse(text string) (Statement, error) {
	tokens, err := Tokenize(text)
	if err != nil {
		return nil, err
	}
	p := &parser{text: text, tokens: tokens, named: map[string]int{}, bindings: map[int]Parameter{}}
	var stmt Statement
	switch {
	case p.take("SELECT"):
		stmt, err = p.selectStmt()
	case p.take("CREATE"):
		stmt, err = p.createStmt()
	case p.take("INSERT"):
		stmt, err = p.insertStmt()
	case p.take("UPDATE"):
		stmt, err = p.updateStmt()
	case p.take("DELETE"):
		stmt, err = p.deleteStmt()
	case p.take("BEGIN"):
		x := &Transaction{Action: "BEGIN"}
		if p.is("DEFERRED") || p.is("IMMEDIATE") || p.is("EXCLUSIVE") {
			x.Mode = upperName(p.next().Text)
		}
		p.take("TRANSACTION")
		stmt = x
	case p.take("COMMIT") || p.take("END"):
		p.take("TRANSACTION")
		stmt = &Transaction{Action: "COMMIT"}
	case p.take("ROLLBACK"):
		p.take("TRANSACTION")
		stmt = &Transaction{Action: "ROLLBACK"}
	default:
		return nil, p.unsupported("statement " + p.peek().Text)
	}
	if err != nil {
		return nil, err
	}
	p.take(";")
	if p.peek().Kind != EOF {
		return nil, p.unsupported("trailing syntax " + p.peek().Text)
	}
	bind := make([]Parameter, 0, len(p.bindings))
	for _, v := range p.bindings {
		bind = append(bind, v)
	}
	sort.Slice(bind, func(i, j int) bool { return bind[i].Index < bind[j].Index })
	b := Base{SQL: text, Params: p.params, BindParams: bind}
	switch x := stmt.(type) {
	case *Select:
		x.Base = b
	case *CreateTable:
		x.Base = b
	case *CreateIndex:
		x.Base = b
	case *Insert:
		x.Base = b
	case *Update:
		x.Base = b
	case *Delete:
		x.Base = b
	case *Transaction:
		x.Base = b
	}
	return stmt, nil
}
func ParseExpr(text string) (Expr, error) {
	tokens, err := Tokenize(text)
	if err != nil {
		return nil, err
	}
	p := &parser{text: text, tokens: tokens, named: map[string]int{}, bindings: map[int]Parameter{}}
	e, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	if p.peek().Kind != EOF {
		return nil, p.unsupported("expression suffix " + p.peek().Text)
	}
	return e, nil
}
func (p *parser) peek() Token { return p.tokens[p.pos] }
func (p *parser) next() Token {
	t := p.peek()
	if t.Kind != EOF {
		p.pos++
	}
	return t
}
func (p *parser) is(s string) bool {
	t := p.peek()
	return (t.Kind == Identifier || t.Kind == Symbol) && !t.Quoted && EqualName(t.Text, s)
}
func (p *parser) take(s string) bool {
	if p.is(s) {
		p.next()
		return true
	}
	return false
}
func (p *parser) need(s string) error {
	if p.take(s) {
		return nil
	}
	return p.syntax("expected " + s)
}
func (p *parser) syntax(s string) error {
	return fmt.Errorf("SQL at byte %d: %s (found %q)", p.peek().Start, s, p.peek().Text)
}
func (p *parser) unsupported(s string) error {
	return &ErrUnsupported{Feature: fmt.Sprintf("%s at byte %d", s, p.peek().Start)}
}
func (p *parser) name() (string, error) {
	t := p.peek()
	if t.Kind != Identifier {
		return "", p.syntax("expected identifier")
	}
	if !t.Quoted && reserved(upperName(t.Text)) {
		return "", p.unsupported("reserved identifier " + t.Text)
	}
	p.next()
	return t.Text, nil
}
func reserved(s string) bool {
	switch s {
	case "SELECT", "FROM", "WHERE", "ORDER", "BY", "LIMIT", "OFFSET", "GROUP", "HAVING", "JOIN", "LEFT", "RIGHT", "FULL", "INNER", "OUTER", "CROSS", "ON", "USING", "UNION", "INTERSECT", "EXCEPT", "AS", "ASC", "DESC", "NULL", "NOT", "AND", "OR", "IS", "LIKE", "ESCAPE", "IN", "BETWEEN", "COLLATE", "WHEN", "THEN", "ELSE", "END", "CASE", "DISTINCT", "ALL", "SET", "VALUES", "RETURNING", "DEFAULT", "PRIMARY", "KEY", "UNIQUE", "CHECK", "FOREIGN", "REFERENCES", "CONSTRAINT", "WITHOUT", "STRICT", "AUTOINCREMENT", "CREATE", "TABLE", "INDEX", "INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "IF", "EXISTS", "INTO", "BEGIN", "COMMIT", "ROLLBACK", "TRANSACTION", "REPLACE", "DO", "CONFLICT", "TRUE", "FALSE", "WINDOW", "OVER", "FILTER", "NULLS":
		return true
	}
	return false
}
func (p *parser) optionalAlias() (string, error) {
	if p.take("AS") {
		return p.name()
	}
	t := p.peek()
	if t.Kind == Identifier && (t.Quoted || !reserved(upperName(t.Text))) {
		return p.name()
	}
	return "", nil
}
func (p *parser) selectStmt() (*Select, error) {
	s := &Select{}
	if p.is("DISTINCT") || p.is("ALL") {
		return nil, p.unsupported("SELECT " + p.peek().Text)
	}
	for {
		start := p.peek().Start
		var e Expr
		var err error
		if p.take("*") {
			e = &Star{}
		} else {
			e, err = p.expression(0)
			if err != nil {
				return nil, err
			}
		}
		end := p.tokens[p.pos-1].End
		alias, err := p.optionalAlias()
		if err != nil {
			return nil, err
		}
		if _, star := e.(*Star); star && alias != "" {
			return nil, p.unsupported("alias on *")
		}
		if len(s.Columns) >= MaxColumns {
			return nil, p.unsupported("result columns exceed 2000")
		}
		s.Columns = append(s.Columns, ResultColumn{Expr: e, Alias: alias, Text: p.text[start:end]})
		if !p.take(",") {
			break
		}
	}
	if p.take("FROM") {
		var err error
		s.From, err = p.name()
		if err != nil {
			return nil, err
		}
		s.Alias, err = p.optionalAlias()
		if err != nil {
			return nil, err
		}
	}
	if p.take("WHERE") {
		var err error
		s.Where, err = p.expression(0)
		if err != nil {
			return nil, err
		}
	}
	if p.take("ORDER") {
		if err := p.need("BY"); err != nil {
			return nil, err
		}
		for {
			e, err := p.expression(0)
			if err != nil {
				return nil, err
			}
			desc := p.take("DESC")
			if !desc {
				p.take("ASC")
			}
			s.OrderBy = append(s.OrderBy, OrderTerm{Expr: e, Desc: desc})
			if !p.take(",") {
				break
			}
		}
	}
	if p.take("LIMIT") {
		var err error
		s.Limit, err = p.expression(0)
		if err != nil {
			return nil, err
		}
		if p.take("OFFSET") {
			s.Offset, err = p.expression(0)
		} else if p.take(",") {
			s.Offset = s.Limit
			s.Limit, err = p.expression(0)
		}
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}
func (p *parser) ifNotExists() (bool, error) {
	if !p.take("IF") {
		return false, nil
	}
	if err := p.need("NOT"); err != nil {
		return false, err
	}
	return true, p.need("EXISTS")
}
func (p *parser) createStmt() (Statement, error) {
	if p.take("TABLE") {
		return p.createTable()
	}
	if p.take("INDEX") {
		return p.createIndex()
	}
	return nil, p.unsupported("CREATE " + p.peek().Text)
}
func (p *parser) createTable() (*CreateTable, error) {
	s := &CreateTable{}
	var err error
	s.IfNotExists, err = p.ifNotExists()
	if err != nil {
		return nil, err
	}
	s.Name, err = p.name()
	if err != nil {
		return nil, err
	}
	if err = p.need("("); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	pk := false
	for {
		c := ColumnDef{}
		c.Name, err = p.name()
		if err != nil {
			return nil, err
		}
		key := FoldName(c.Name)
		if seen[key] {
			return nil, p.syntax("duplicate column " + c.Name)
		}
		seen[key] = true
		t := p.peek()
		if t.Kind == Identifier && !t.Quoted {
			switch upperName(t.Text) {
			case "INTEGER", "INT", "TEXT", "REAL", "BLOB", "NUMERIC":
				c.Type = upperName(p.next().Text)
			}
		}
		if p.peek().Kind == Identifier && !p.is("PRIMARY") && !p.is("NOT") && !p.is("DEFAULT") {
			return nil, p.unsupported("column type or constraint " + p.peek().Text)
		}
		for !p.is(",") && !p.is(")") && p.peek().Kind != EOF {
			switch {
			case p.take("PRIMARY"):
				if err = p.need("KEY"); err != nil {
					return nil, err
				}
				if c.PrimaryKey || pk {
					return nil, p.unsupported("multiple primary keys")
				}
				if p.is("DESC") {
					return nil, p.unsupported("INTEGER PRIMARY KEY DESC")
				}
				p.take("ASC")
				c.PrimaryKey = true
				pk = true
			case p.take("NOT"):
				if err = p.need("NULL"); err != nil {
					return nil, err
				}
				if c.NotNull {
					return nil, p.syntax("duplicate NOT NULL")
				}
				c.NotNull = true
			case p.take("DEFAULT"):
				if c.Default != nil {
					return nil, p.syntax("duplicate DEFAULT")
				}
				c.Default, err = p.defaultExpr()
				if err != nil {
					return nil, err
				}
			default:
				return nil, p.unsupported("column constraint " + p.peek().Text)
			}
		}
		if len(s.Columns) >= MaxColumns {
			return nil, p.unsupported("table columns exceed 2000")
		}
		s.Columns = append(s.Columns, c)
		if !p.take(",") {
			break
		}
	}
	if err = p.need(")"); err != nil {
		return nil, err
	}
	return s, nil
}
func (p *parser) defaultExpr() (Expr, error) {
	var e Expr
	var err error
	if p.take("(") {
		e, err = p.expression(0)
		if err == nil {
			err = p.need(")")
		}
	} else {
		e, err = p.expression(10)
	}
	if err != nil {
		return nil, err
	}
	err = Walk(e, func(e Expr) error {
		switch e.(type) {
		case *Column, *Parameter, *Star, *Call:
			return p.unsupported("nonconstant DEFAULT")
		}
		return nil
	})
	return e, err
}
func (p *parser) createIndex() (*CreateIndex, error) {
	s := &CreateIndex{}
	var err error
	s.IfNotExists, err = p.ifNotExists()
	if err != nil {
		return nil, err
	}
	s.Name, err = p.name()
	if err != nil {
		return nil, err
	}
	if err = p.need("ON"); err != nil {
		return nil, err
	}
	s.Table, err = p.name()
	if err != nil {
		return nil, err
	}
	if err = p.need("("); err != nil {
		return nil, err
	}
	for {
		n, err := p.name()
		if err != nil {
			return nil, err
		}
		if len(s.Columns) >= MaxColumns {
			return nil, p.unsupported("column list exceeds 2000")
		}
		s.Columns = append(s.Columns, n)
		desc := p.take("DESC")
		if !desc {
			p.take("ASC")
		}
		s.Desc = append(s.Desc, desc)
		if !p.take(",") {
			break
		}
	}
	if err = p.need(")"); err != nil {
		return nil, err
	}
	return s, nil
}
func (p *parser) insertStmt() (*Insert, error) {
	s := &Insert{}
	if err := p.need("INTO"); err != nil {
		return nil, err
	}
	var err error
	s.Table, err = p.name()
	if err != nil {
		return nil, err
	}
	if p.take("(") {
		for {
			n, err := p.name()
			if err != nil {
				return nil, err
			}
			for _, prev := range s.Columns {
				if EqualName(prev, n) {
					return nil, p.unsupported("duplicate INSERT column")
				}
			}
			if len(s.Columns) >= MaxColumns {
				return nil, p.unsupported("column list exceeds 2000")
			}
			s.Columns = append(s.Columns, n)
			if !p.take(",") {
				break
			}
		}
		if err = p.need(")"); err != nil {
			return nil, err
		}
	}
	if p.take("DEFAULT") {
		s.DefaultValues = true
		return s, p.need("VALUES")
	}
	if err = p.need("VALUES"); err != nil {
		return nil, err
	}
	for {
		if err = p.need("("); err != nil {
			return nil, err
		}
		var row []Expr
		for {
			e, err := p.expression(0)
			if err != nil {
				return nil, err
			}
			if len(row) >= MaxColumns {
				return nil, p.unsupported("VALUES columns exceed 2000")
			}
			row = append(row, e)
			if !p.take(",") {
				break
			}
		}
		if err = p.need(")"); err != nil {
			return nil, err
		}
		if len(s.Rows) > 0 && len(row) != len(s.Rows[0]) {
			return nil, p.syntax("VALUES rows have different lengths")
		}
		s.Rows = append(s.Rows, row)
		if !p.take(",") {
			break
		}
	}
	return s, nil
}
func (p *parser) updateStmt() (*Update, error) {
	s := &Update{}
	var err error
	s.Table, err = p.name()
	if err != nil {
		return nil, err
	}
	if err = p.need("SET"); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for {
		n, err := p.name()
		if err != nil {
			return nil, err
		}
		if seen[FoldName(n)] {
			return nil, p.unsupported("duplicate UPDATE target")
		}
		seen[FoldName(n)] = true
		if err = p.need("="); err != nil {
			return nil, err
		}
		e, err := p.expression(0)
		if err != nil {
			return nil, err
		}
		s.Assignments = append(s.Assignments, Assignment{Column: n, Expr: e})
		if !p.take(",") {
			break
		}
	}
	if p.take("WHERE") {
		s.Where, err = p.expression(0)
	}
	return s, err
}
func (p *parser) deleteStmt() (*Delete, error) {
	s := &Delete{}
	if err := p.need("FROM"); err != nil {
		return nil, err
	}
	var err error
	s.Table, err = p.name()
	if err != nil {
		return nil, err
	}
	if p.take("WHERE") {
		s.Where, err = p.expression(0)
	}
	return s, err
}
func (p *parser) expression(min int) (Expr, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxExpressionDepth {
		return nil, p.unsupported("expression nesting exceeds 128")
	}
	var left Expr
	var err error
	t := p.next()
	switch {
	case t.Kind == String:
		left = &Literal{Value: t.Text}
	case t.Kind == Blob:
		b, _ := hex.DecodeString(t.Text)
		left = &Literal{Value: b}
	case t.Kind == Number:
		v, err := numberLiteral(t.Text)
		if err != nil {
			return nil, err
		}
		left = &Literal{Value: v, hexLiteral: strings.HasPrefix(FoldName(t.Text), "0x")}
	case t.Kind == Bind:
		n, err := p.parameter(t.Text)
		if err != nil {
			return nil, err
		}
		left = n
	case !t.Quoted && t.Text == "(":
		left, err = p.expression(0)
		if err == nil {
			err = p.need(")")
		}
	case !t.Quoted && (t.Text == "+" || t.Text == "-" || t.Text == "~" || EqualName(t.Text, "NOT")):
		op := upperName(t.Text)
		prec := 10
		if op == "NOT" {
			prec = 3
		}
		if op == "-" && p.peek().Kind == Number && strings.ReplaceAll(p.peek().Text, "_", "") == "9223372036854775808" {
			p.next()
			left = &Literal{Value: int64(math.MinInt64)}
		} else {
			var e Expr
			e, err = p.expression(prec)
			if op == "-" && negativeHexOverflow(e) {
				return nil, p.syntax("hex literal too big")
			}
			left = &Unary{Op: op, Expr: e}
		}
	case t.Kind == Identifier && !t.Quoted && EqualName(t.Text, "NULL"):
		left = &Literal{}
	case t.Kind == Identifier && !t.Quoted && (EqualName(t.Text, "TRUE") || EqualName(t.Text, "FALSE")):
		v := int64(0)
		if EqualName(t.Text, "TRUE") {
			v = 1
		}
		left = &Literal{Value: v, Boolean: true}
	case t.Kind == Identifier:
		if !t.Quoted && reserved(upperName(t.Text)) {
			return nil, p.unsupported("expression " + t.Text)
		}
		if p.take("(") {
			name := FoldName(t.Text)
			switch name {
			case "count", "sum", "avg", "total", "group_concat", "string_agg", "min", "max":
				return nil, p.unsupported("aggregate function " + name)
			}
			call := &Call{Name: name}
			if !p.take(")") {
				for {
					arg, err := p.expression(0)
					if err != nil {
						return nil, err
					}
					if len(call.Args) >= MaxFunctionArgs {
						return nil, p.unsupported("function arguments exceed 1000")
					}
					call.Args = append(call.Args, arg)
					if !p.take(",") {
						break
					}
				}
				err = p.need(")")
			}
			left = call
		} else if p.take(".") {
			if p.take("*") {
				left = &Star{Table: t.Text}
			} else {
				n, e := p.name()
				err = e
				left = &Column{Table: t.Text, Name: n}
			}
		} else {
			left = &Column{Name: t.Text}
		}
	default:
		return nil, p.syntax("expected scalar expression")
	}
	if err != nil {
		return nil, err
	}
	if err = p.checkExpr(left); err != nil {
		return nil, err
	}
	for {
		op := upperName(p.peek().Text)
		if p.peek().Quoted || (p.peek().Kind != Identifier && p.peek().Kind != Symbol) {
			break
		}
		neg := false
		if op == "NOT" && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Kind == Identifier && !p.tokens[p.pos+1].Quoted && EqualName(p.tokens[p.pos+1].Text, "LIKE") {
			op = "LIKE"
			neg = true
		}
		prec := precedence(op)
		if prec == 0 || prec < min {
			break
		}
		p.next()
		if neg {
			p.next()
		}
		if op == "ISNULL" || op == "NOTNULL" {
			name := "IS"
			if op == "NOTNULL" {
				name = "IS NOT"
			}
			left = &Binary{Op: name, Left: left, Right: &Literal{}}
			if err = p.checkExpr(left); err != nil {
				return nil, err
			}
			continue
		}
		if op == "IS" && p.take("NOT") {
			op = "IS NOT"
		}
		right, err := p.expression(prec + 1)
		if err != nil {
			return nil, err
		}
		if op == "LIKE" {
			x := &Like{Expr: left, Pattern: right, Not: neg}
			if p.take("ESCAPE") {
				x.Escape, err = p.expression(6)
				if err != nil {
					return nil, err
				}
			}
			left = x
		} else {
			left = &Binary{Op: op, Left: left, Right: right}
		}
		if err = p.checkExpr(left); err != nil {
			return nil, err
		}
	}
	return left, nil
}
func precedence(op string) int {
	switch op {
	case "OR":
		return 1
	case "AND":
		return 2
	case "=", "==", "!=", "<>", "IS", "LIKE", "ISNULL", "NOTNULL":
		return 4
	case "<", ">", "<=", ">=":
		return 5
	case "&", "|", "<<", ">>":
		return 6
	case "+", "-":
		return 7
	case "*", "/", "%":
		return 8
	case "||":
		return 9
	}
	return 0
}
func (p *parser) parameter(name string) (*Parameter, error) {
	n := 0
	label := name
	if name == "?" {
		n = p.params + 1
		label = ""
	} else if name[0] == '?' {
		v, err := strconv.Atoi(name[1:])
		if err != nil || v < 1 || v > MaxParameters {
			return nil, p.syntax("parameter number must be between 1 and 32766")
		}
		n = v
	} else {
		n = p.named[name]
		if n == 0 {
			n = p.params + 1
			p.named[name] = n
		}
	}
	if n > MaxParameters {
		return nil, p.syntax("too many parameters")
	}
	if n > p.params {
		p.params = n
	}
	param := Parameter{Index: n, Name: label}
	if old, ok := p.bindings[n]; !ok || old.Name == "" {
		p.bindings[n] = param
	}
	return &param, nil
}
func numberLiteral(raw string) (any, error) {
	s := strings.ReplaceAll(raw, "_", "")
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		v, e := strconv.ParseUint(s[2:], 16, 64)
		if e != nil {
			return nil, fmt.Errorf("hex integer literal exceeds 64 bits")
		}
		return int64(v), nil
	}
	if !strings.ContainsAny(s, ".eE") {
		if v, e := strconv.ParseInt(s, 10, 64); e == nil {
			return v, nil
		}
	}
	v, e := strconv.ParseFloat(s, 64)
	if e != nil && !math.IsInf(v, 0) {
		return nil, fmt.Errorf("invalid numeric literal %q", raw)
	}
	return v, nil
}

// Walk visits each expression before its children, stopping at the first error.
// It supports statement-wide name/function validation before reading any rows.
func Walk(e Expr, visit func(Expr) error) error {
	if e == nil {
		return nil
	}
	if err := visit(e); err != nil {
		return err
	}
	var children []Expr
	switch x := e.(type) {
	case *Unary:
		children = []Expr{x.Expr}
	case *Binary:
		children = []Expr{x.Left, x.Right}
	case *Like:
		children = []Expr{x.Expr, x.Pattern, x.Escape}
	case *Call:
		children = x.Args
	}
	for _, child := range children {
		if err := Walk(child, visit); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) checkExpr(e Expr) error {
	if p.heights == nil {
		p.heights = map[Expr]int{}
	}
	d := 1
	child := func(c Expr) {
		if c == nil {
			return
		}
		h := p.heights[c]
		if h == 0 {
			h = 1
		}
		if h+1 > d {
			d = h + 1
		}
	}
	switch x := e.(type) {
	case *Unary:
		child(x.Expr)
	case *Binary:
		child(x.Left)
		child(x.Right)
	case *Like:
		child(x.Expr)
		child(x.Pattern)
		child(x.Escape)
	case *Call:
		for _, a := range x.Args {
			child(a)
		}
	}
	if d > maxExpressionDepth {
		return p.unsupported("expression tree depth exceeds 128")
	}
	p.heights[e] = d
	return nil
}

func negativeHexOverflow(e Expr) bool {
	for {
		if u, ok := e.(*Unary); ok && u.Op == "+" {
			e = u.Expr
			continue
		}
		v, ok := e.(*Literal)
		return ok && v.hexLiteral && v.Value == int64(math.MinInt64)
	}
}
