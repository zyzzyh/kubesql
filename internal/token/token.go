// Package token defines the values shared by the lexer and parser.
package token

// Type identifies the kind of a token.
type Type string

const (
	Illegal      Type = "ILLEGAL"
	EOF          Type = "EOF"
	Select       Type = "SELECT"
	Update       Type = "UPDATE"
	Delete       Type = "DELETE"
	From         Type = "FROM"
	Set          Type = "SET"
	Into         Type = "INTO"
	Values       Type = "VALUES"
	Where        Type = "WHERE"
	And          Type = "AND"
	Or           Type = "OR"
	Not          Type = "NOT"
	Is           Type = "IS"
	Null         Type = "NULL"
	True         Type = "TRUE"
	False        Type = "FALSE"
	Identifier   Type = "IDENTIFIER"
	String       Type = "STRING"
	Integer      Type = "INTEGER"
	Decimal      Type = "DECIMAL"
	Comma        Type = ","
	Asterisk     Type = "*"
	Semicolon    Type = ";"
	Equal        Type = "="
	NotEqual     Type = "<>"
	Greater      Type = ">"
	GreaterEqual Type = ">="
	Less         Type = "<"
	LessEqual    Type = "<="
	LeftParen    Type = "("
	RightParen   Type = ")"
)

// Token stores one SQL token and its starting position in the input.
type Token struct {
	Type    Type
	Literal string
	Line    int
	Column  int
}
