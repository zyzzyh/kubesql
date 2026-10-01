// Package token defines the values shared by the lexer and parser.
package token

// Type identifies the kind of a token.
type Type string

const (
	Illegal    Type = "ILLEGAL"
	EOF        Type = "EOF"
	Select     Type = "SELECT"
	From       Type = "FROM"
	Identifier Type = "IDENTIFIER"
	Comma      Type = ","
	Asterisk   Type = "*"
	Semicolon  Type = ";"
)

// Token stores one SQL token and its starting position in the input.
type Token struct {
	Type    Type
	Literal string
	Line    int
	Column  int
}
