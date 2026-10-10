// Package sql implements the explicitly supported SQL grammar and scalar semantics.
// It does not perform storage or invoke an external SQL engine.
package sql

import "fmt"

// ErrUnsupported identifies syntax or semantics outside the supported subset.
type ErrUnsupported struct{ Feature string }

func (e *ErrUnsupported) Error() string { return "unsupported SQL: " + e.Feature }

// Statement is one complete statement. Parse rejects multiple statements.
type Statement interface {
	statement()
	ParameterCount() int
	Parameters() []Parameter
}
type Base struct {
	SQL        string
	Params     int
	BindParams []Parameter
}

func (b Base) ParameterCount() int     { return b.Params }
func (b Base) Parameters() []Parameter { return append([]Parameter(nil), b.BindParams...) }

type Select struct {
	Base
	Columns       []ResultColumn
	From, Alias   string
	Where         Expr
	OrderBy       []OrderTerm
	Limit, Offset Expr
}

func (*Select) statement() {}

type ResultColumn struct {
	Expr  Expr
	Alias string
	Text  string // Exact source expression, excluding any alias.
}
type OrderTerm struct {
	Expr Expr
	Desc bool
}

type CreateTable struct {
	Base
	Name        string
	IfNotExists bool
	Columns     []ColumnDef
}

func (*CreateTable) statement() {}

type ColumnDef struct {
	Name, Type          string
	PrimaryKey, NotNull bool
	Default             Expr
}
type CreateIndex struct {
	Base
	Name, Table string
	IfNotExists bool
	Columns     []string
	Desc        []bool
}

func (*CreateIndex) statement() {}

type Insert struct {
	Base
	Table         string
	Columns       []string
	Rows          [][]Expr
	DefaultValues bool
}

func (*Insert) statement() {}

type Update struct {
	Base
	Table       string
	Assignments []Assignment
	Where       Expr
}

func (*Update) statement() {}

type Assignment struct {
	Column string
	Expr   Expr
}
type Delete struct {
	Base
	Table string
	Where Expr
}

func (*Delete) statement() {}

type Transaction struct {
	Base
	Action, Mode string
}

func (*Transaction) statement() {}

// Expr is a scalar expression. Star is valid only in SELECT result lists.
type Expr interface{ expr() }
type Literal struct {
	Value      any
	Boolean    bool
	hexLiteral bool
}

func (*Literal) expr() {}

type Column struct{ Table, Name string }

func (*Column) expr() {}

type Star struct{ Table string }

func (*Star) expr() {}

type Parameter struct {
	Index int
	Name  string
}

func (*Parameter) expr() {}

type Unary struct {
	Op   string
	Expr Expr
}

func (*Unary) expr() {}

type Binary struct {
	Op          string
	Left, Right Expr
}

func (*Binary) expr() {}

type Like struct {
	Expr, Pattern, Escape Expr
	Not                   bool
}

func (*Like) expr() {}

type Call struct {
	Name string
	Args []Expr
}

func (*Call) expr() {}

// ExprName is a deterministic fallback result-column label. Callers preserving
// exact original labels may use the source spans in Tokenize instead.
func ExprName(e Expr) string {
	switch x := e.(type) {
	case *Column:
		return x.Name
	case *Star:
		if x.Table != "" {
			return x.Table + ".*"
		}
		return "*"
	case *Parameter:
		if x.Name != "" {
			return x.Name
		}
		return fmt.Sprintf("?%d", x.Index)
	case *Literal:
		if x.Value == nil {
			return "NULL"
		}
		return fmt.Sprint(x.Value)
	case *Unary:
		return x.Op + " " + ExprName(x.Expr)
	case *Binary:
		return ExprName(x.Left) + " " + x.Op + " " + ExprName(x.Right)
	case *Like:
		return ExprName(x.Expr) + " LIKE " + ExprName(x.Pattern)
	case *Call:
		return x.Name + "(...)"
	default:
		return "expression"
	}
}
