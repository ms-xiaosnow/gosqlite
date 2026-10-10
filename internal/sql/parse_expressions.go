package sql

func (p *parser) caseExpr() (Expr, error) {
	x := &Case{}
	var err error
	if !p.is("WHEN") {
		x.Base, err = p.expression(0)
		if err != nil {
			return nil, err
		}
	}
	if !p.is("WHEN") {
		return nil, p.syntax("expected WHEN")
	}
	for p.take("WHEN") {
		when, err := p.expression(0)
		if err != nil {
			return nil, err
		}
		if err = p.need("THEN"); err != nil {
			return nil, err
		}
		then, err := p.expression(0)
		if err != nil {
			return nil, err
		}
		x.Arms = append(x.Arms, CaseArm{When: when, Then: then})
	}
	if p.take("ELSE") {
		x.Else, err = p.expression(0)
		if err != nil {
			return nil, err
		}
	}
	if err = p.need("END"); err != nil {
		return nil, err
	}
	return x, nil
}

func (p *parser) castExpr() (Expr, error) {
	if err := p.need("("); err != nil {
		return nil, err
	}
	e, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	if err = p.need("AS"); err != nil {
		return nil, err
	}
	// A CAST type-name is ordinary SQLite affinity text, even if spelled
	// VECTOR(n). It must never route to private extension constructors.
	first, start := p.peek(), p.pos
	for p.peek().Kind == String || p.peek().Kind == Identifier && (p.peek().Quoted || castTypeName(p.peek().Text)) {
		p.next()
	}
	if p.pos > start && p.take("(") {
		for i := 0; ; i++ {
			if i > 1 {
				return nil, p.syntax("at most two type size arguments")
			}
			if p.is("+") || p.is("-") {
				p.next()
			}
			if p.peek().Kind != Number {
				return nil, p.syntax("expected type size")
			}
			p.next()
			if !p.take(",") {
				break
			}
		}
		if err = p.need(")"); err != nil {
			return nil, err
		}
	}
	typ := ""
	if p.pos > start {
		// SQLite preserves the complete typename span, including intervening
		// comments, then dequotes only its initial quoted token. Do not
		// reconstruct it from words: that can change the derived affinity.
		typ = p.text[first.Start:p.tokens[p.pos-1].End]
		if first.Quoted || first.Kind == String {
			typ = first.Text
		}
	}
	if err = p.need(")"); err != nil {
		return nil, err
	}
	return &Cast{Expr: e, Type: typ}, nil
}

func castTypeName(name string) bool {
	// Pinned parse.y's typename uses ID/STRING and ID fallbacks, rather than
	// the broader query parser's reserved-word set. These keywords have no
	// ID fallback in the normal (compound-SELECT-enabled) SQLite build.
	// Unknown names, TRUE/FALSE/STRICT and contextual WINDOW/OVER/FILTER
	// remain identifiers here. Quoted names bypass this check entirely.
	switch upperName(name) {
	case "ADD", "ALL", "ALTER", "AND", "AS", "AUTOINCREMENT", "BETWEEN", "CASE",
		"CHECK", "COLLATE", "COMMIT", "CONSTRAINT", "CREATE", "CROSS", "DEFAULT", "DEFERRABLE",
		"DELETE", "DISTINCT", "DROP", "ELSE", "ESCAPE", "EXCEPT", "EXISTS", "FOREIGN",
		"FROM", "FULL", "GROUP", "HAVING", "IN", "INDEX", "INDEXED", "INNER",
		"INSERT", "INTERSECT", "INTO", "IS", "ISNULL", "JOIN", "LEFT", "LIMIT",
		"NATURAL", "NOT", "NOTHING", "NOTNULL", "NULL", "ON", "OR", "ORDER",
		"OUTER", "PRIMARY", "REFERENCES", "RETURNING", "RIGHT", "SELECT", "SET", "TABLE",
		"THEN", "TO", "TRANSACTION", "UNION", "UNIQUE", "UPDATE", "USING", "VALUES",
		"WHEN", "WHERE":
		return false
	}
	return true
}

// SQLite retains a function-bearing empty-IN operand for aggregate/function
// validation. The original operand is already subject to parser depth limits.
func containsCall(e Expr) bool {
	found := false
	_ = Walk(e, func(e Expr) error {
		switch e.(type) {
		case *Call, *Like: // LIKE is a function call in SQLite's grammar.
			found = true
		}
		return nil
	})
	return found
}
