package sql

// SQLite also accepts string-quoted savepoint names. Keep this exception local
// to transaction control rather than broadening ordinary identifier parsing.
func (p *parser) savepointName() (string, error) {
	token := p.peek()
	if token.Kind == String {
		return p.next().Text, nil
	}
	if token.Kind != Identifier {
		return "", p.syntax("expected savepoint name")
	}
	if !token.Quoted {
		// SQLite 3.54 parse.y nm/idj and %fallback ID. JOIN_KW and
		// contextual WINDOW/OVER/FILTER tokens are valid names here.
		switch upperName(token.Text) {
		case "ADD", "ALL", "ALTER", "AND", "AS", "AUTOINCREMENT", "BETWEEN", "CASE", "CHECK", "COLLATE", "COMMIT", "CONSTRAINT", "CREATE", "DEFAULT", "DEFERRABLE", "DELETE", "DISTINCT", "DROP", "ELSE", "ESCAPE", "EXCEPT", "EXISTS", "FOREIGN", "FROM", "GROUP", "HAVING", "IN", "INDEX", "INSERT", "INTERSECT", "INTO", "IS", "ISNULL", "JOIN", "LIMIT", "NOT", "NOTHING", "NOTNULL", "NULL", "ON", "OR", "ORDER", "PRIMARY", "REFERENCES", "RETURNING", "SELECT", "SET", "TABLE", "THEN", "TO", "TRANSACTION", "UNION", "UNIQUE", "UPDATE", "USING", "VALUES", "WHEN", "WHERE":
			return "", p.syntax("expected savepoint name")
		}
	}
	return p.next().Text, nil
}
