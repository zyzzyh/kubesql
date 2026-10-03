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
