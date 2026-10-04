// Package ast defines the parsed structure of SQL statements.
package ast

// Statement is a parsed SQL statement.
type Statement interface {
	statementNode()
}

// SelectItem is a column name or an explicit star in a SELECT list.
type SelectItem interface {
	selectItemNode()
}

// SelectStatement describes the selected columns and source table.
type SelectStatement struct {
	Type    string       `json:"type"`
	Columns []SelectItem `json:"columns"`
	Table   string       `json:"table"`
	Where   Expression   `json:"where,omitempty"`
}

func (*SelectStatement) statementNode() {}

// Column selects one named column.
type Column struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

func (*Column) selectItemNode() {}

// Star selects all columns; expansion belongs to semantic analysis.
type Star struct {
	Type string `json:"type"`
}

func (*Star) selectItemNode() {}

// Expression is a condition or value used by a WHERE clause.
type Expression interface {
	expressionNode()
}

// ColumnReference reads a value from the current resource row.
type ColumnReference struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

func (*ColumnReference) expressionNode() {}

// Literal is a SQL constant such as a string, number, boolean, or NULL.
type Literal struct {
	Type  string `json:"type"`
	Kind  string `json:"kind"`
	Value any    `json:"value"`
}

func (*Literal) expressionNode() {}

// BinaryExpression combines two expressions with a comparison or logical operator.
type BinaryExpression struct {
	Type     string     `json:"type"`
	Operator string     `json:"operator"`
	Left     Expression `json:"left"`
	Right    Expression `json:"right"`
}

func (*BinaryExpression) expressionNode() {}

// UnaryExpression applies a prefix operator such as NOT.
type UnaryExpression struct {
	Type       string     `json:"type"`
	Operator   string     `json:"operator"`
	Expression Expression `json:"expression"`
}

func (*UnaryExpression) expressionNode() {}

// IsNullExpression tests whether an expression evaluates to SQL NULL.
type IsNullExpression struct {
	Type       string     `json:"type"`
	Expression Expression `json:"expression"`
	Not        bool       `json:"not"`
}

func (*IsNullExpression) expressionNode() {}
