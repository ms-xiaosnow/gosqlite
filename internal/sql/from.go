package sql

// Relation is a FROM tree. Named leaves retain their own alias/index hint.
// This slice deliberately excludes derived tables, USING and NATURAL joins.
type Relation interface{ relation() }
type TableRelation struct {
	Name, Alias string
	NotIndexed  bool
}

func (*TableRelation) relation() {}

type JoinRelation struct {
	Left, Right Relation
	Kind        string // INNER, CROSS or LEFT; comma joins are CROSS.
	On          Expr
}

func (*JoinRelation) relation() {}

const MaxRelations = 64

func (p *parser) tableRelation() (*TableRelation, error) {
	if p.is("(") {
		return nil, p.unsupported("parenthesized or derived FROM relation")
	}
	name, err := p.name()
	if err != nil {
		return nil, err
	}
	if p.is(".") {
		return nil, p.unsupported("schema-qualified FROM relation")
	}
	alias := ""
	// NATURAL is a join modifier only in this FROM context. Do not newly
	// reserve the word in existing scalar/result-alias grammar.
	if !p.is("NATURAL") {
		alias, err = p.optionalAlias()
		if err != nil {
			return nil, err
		}
	}
	t := &TableRelation{Name: name, Alias: alias}
	if p.take("NOT") {
		if err := p.need("INDEXED"); err != nil {
			return nil, err
		}
		t.NotIndexed = true
	}
	return t, nil
}
func (p *parser) fromRelation() (Relation, error) {
	first, err := p.tableRelation()
	if err != nil {
		return nil, err
	}
	var left Relation = first
	for n := 1; ; n++ {
		if p.is("NATURAL") || p.is("USING") || p.is("RIGHT") || p.is("FULL") {
			return nil, p.unsupported("join " + p.peek().Text)
		}
		kind := ""
		switch {
		case p.take(","):
			kind = "CROSS"
		case p.take("JOIN"):
			kind = "INNER"
		case p.take("INNER"):
			kind = "INNER"
			err = p.need("JOIN")
		case p.take("CROSS"):
			kind = "CROSS"
			err = p.need("JOIN")
		case p.take("LEFT"):
			kind = "LEFT"
			p.take("OUTER")
			err = p.need("JOIN")
		default:
			return left, nil
		}
		if err != nil {
			return nil, err
		}
		if n >= MaxRelations {
			return nil, p.unsupported("more than 64 FROM relations")
		}
		right, err := p.tableRelation()
		if err != nil {
			return nil, err
		}
		join := &JoinRelation{Left: left, Right: right, Kind: kind}
		if p.take("ON") {
			join.On, err = p.expression(0)
			if err != nil {
				return nil, err
			}
		}
		left = join
	}
}
