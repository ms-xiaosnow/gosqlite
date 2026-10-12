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
	Relation      Relation // Non-nil for multi-source FROM; legacy leaf fields are preserved.
	Where         Expr
	GroupBy       []Expr
	Having        Expr
	OrderBy       []OrderTerm
	Limit, Offset Expr
	// Explain requests plan rows; NotIndexed explicitly disables index filtering.
	Explain, NotIndexed bool
}

func (*Select) statement() {}

type ResultColumn struct {
	Expr  Expr
	Alias string
	Text  string // Exact source expression, excluding any alias.
}

// NullOrder is independent of sort direction. Default uses SQLite's
// NULL-smallest order (first for ASC, last for DESC).
type NullOrder uint8

const (
	NullsDefault NullOrder = iota
	NullsFirst
	NullsLast
)

type OrderTerm struct {
	Expr  Expr
	Desc  bool
	Nulls NullOrder
}

type CreateTable struct {
	Base
	Name        string
	IfNotExists bool
	Columns     []ColumnDef
	// UniqueConstraints retains declaration order, including duplicate clauses.
	UniqueConstraints []UniqueConstraint
	// CheckConstraints preserves every declaration in source order.
	CheckConstraints []CheckConstraint
}

func (*CreateTable) statement() {}

// UniqueConstraint is a named or anonymous BINARY ASC ordinary constraint.
// Names label constraints only; implicit index names are derived separately.
type UniqueConstraint struct {
	Name    string
	Columns []string
}

// CheckConstraint retains the unfurled source tree for admission. Execution
// compiles a separate immutable tree only after every source reference is bound.
// Text is a substring of Base.SQL, not a separately copied schema string.
type CheckConstraint struct {
	Named      bool
	Name       string
	Text       string
	Start, End int
	Expr       Expr
}

type ColumnDef struct {
	Name, Type          string
	PrimaryKey, NotNull bool
	Default             Expr
}
type CreateIndex struct {
	Base
	Name, Table string
	IfNotExists bool
	Unique      bool
	Columns     []string
	Desc        []bool
	// Method and Options are project extension grammar, never stored as SQL schema.
	Method  string
	Options map[string]any
}

func (*CreateIndex) statement() {}

type Reindex struct {
	Base
	Name string
}

func (*Reindex) statement() {}

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
	Action, Mode, Name string
}

func (*Transaction) statement() {}

// Expr is a scalar expression. Star is valid only in SELECT result lists.
type Expr interface{ expr() }
type Literal struct {
	Value        any
	Boolean      bool
	hexLiteral   bool
	truthLiteral bool // Pinned SQLite syntactic integer truth annotation.
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

// Case preserves lazy selection and the single evaluation of a simple CASE
// operand. A nil Base denotes searched CASE; a nil Else yields SQL NULL.
type Case struct {
	Base Expr
	Arms []CaseArm
	Else Expr
}
type CaseArm struct{ When, Then Expr }

func (*Case) expr() {}

// Cast uses ordinary SQLite declared-type affinity, never a vector converter.
type Cast struct {
	Expr Expr
	Type string
}

func (*Cast) expr() {}

type Between struct {
	Expr, Lower, Upper Expr
	Not                bool
}

func (*Between) expr() {}

type InList struct {
	Expr   Expr
	Values []Expr
	Not    bool
}

func (*InList) expr() {}

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
	case *Case:
		return "CASE ... END"
	case *Cast:
		return "CAST(" + ExprName(x.Expr) + " AS " + x.Type + ")"
	case *Between:
		return ExprName(x.Expr) + " BETWEEN " + ExprName(x.Lower) + " AND " + ExprName(x.Upper)
	case *InList:
		return ExprName(x.Expr) + " IN (...)"
	case *Call:
		return x.Name + "(...)"
	default:
		return "expression"
	}
}
