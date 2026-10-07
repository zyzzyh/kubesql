package lexer

import (
	"strings"
	"unicode"

	"github.com/zyzzyh/kubesql/internal/token"
)

// Lexer scans SQL input one Unicode code point at a time.
type Lexer struct {
	input    []rune
	position int
	line     int
	column   int
}

// New creates a lexer whose first input position is line 1, column 1.
func New(input string) *Lexer {
	return &Lexer{
		input:  []rune(input),
		line:   1,
		column: 1,
	}
}

// NextToken scans one token while preserving line and column information for
// parser diagnostics.
func (l *Lexer) NextToken() token.Token {
	l.skipWhitespace()

	line, column := l.line, l.column
	if l.position >= len(l.input) {
		return token.Token{Type: token.EOF, Line: line, Column: column}
	}

	current := l.input[l.position]
	switch current {
	case ',':
		return l.readSingleCharacterToken(token.Comma, line, column)
	case '*':
		return l.readSingleCharacterToken(token.Asterisk, line, column)
	case ';':
		return l.readSingleCharacterToken(token.Semicolon, line, column)
	case '(':
		return l.readSingleCharacterToken(token.LeftParen, line, column)
	case ')':
		return l.readSingleCharacterToken(token.RightParen, line, column)
	case '=':
		return l.readSingleCharacterToken(token.Equal, line, column)
	case '>':
		return l.readOptionalSecondCharacterToken(token.Greater, token.GreaterEqual, '=', line, column)
	case '<':
		if l.peek() == '>' {
			return l.readOptionalSecondCharacterToken(token.Less, token.NotEqual, '>', line, column)
		}
		return l.readOptionalSecondCharacterToken(token.Less, token.LessEqual, '=', line, column)
	}

	if current == '\'' {
		return l.readString(line, column)
	}
	if current == '"' {
		return l.readQuotedIdentifier(line, column)
	}
	if current >= '0' && current <= '9' {
		return l.readNumber(line, column)
	}

	if isIdentifierStart(current) {
		return l.readIdentifier(line, column)
	}

	l.advance()
	return token.Token{
		Type:    token.Illegal,
		Literal: string(current),
		Line:    line,
		Column:  column,
	}
}

func (l *Lexer) skipWhitespace() {
	for l.position < len(l.input) && unicode.IsSpace(l.input[l.position]) {
		l.advance()
	}
}

func (l *Lexer) readSingleCharacterToken(kind token.Type, line, column int) token.Token {
	literal := string(l.input[l.position])
	l.advance()
	return token.Token{Type: kind, Literal: literal, Line: line, Column: column}
}

func (l *Lexer) readOptionalSecondCharacterToken(single, combined token.Type, second rune, line, column int) token.Token {
	literal := string(l.input[l.position])
	l.advance()
	if l.position < len(l.input) && l.input[l.position] == second {
		literal += string(second)
		l.advance()
		return token.Token{Type: combined, Literal: literal, Line: line, Column: column}
	}
	return token.Token{Type: single, Literal: literal, Line: line, Column: column}
}

func (l *Lexer) readString(line, column int) token.Token {
	l.advance()
	var value strings.Builder
	for l.position < len(l.input) {
		current := l.input[l.position]
		if current == '\'' {
			l.advance()
			if l.position < len(l.input) && l.input[l.position] == '\'' {
				value.WriteRune('\'')
				l.advance()
				continue
			}
			return token.Token{Type: token.String, Literal: value.String(), Line: line, Column: column}
		}
		value.WriteRune(current)
		l.advance()
	}
	return token.Token{Type: token.Illegal, Literal: value.String(), Line: line, Column: column}
}

// readQuotedIdentifier reads a SQL delimited identifier. Unlike a string,
// its value is kept as an identifier so keywords and punctuation can appear
// inside names such as "apps/v1/statefulsets" or "/spec/replicas".
// readQuotedIdentifier decodes a double-quoted SQL identifier, including the
// SQL escape sequence "" for an embedded quote.
func (l *Lexer) readQuotedIdentifier(line, column int) token.Token {
	l.advance()
	var value strings.Builder
	for l.position < len(l.input) {
		current := l.input[l.position]
		if current == '"' {
			l.advance()
			if l.position < len(l.input) && l.input[l.position] == '"' {
				value.WriteRune('"')
				l.advance()
				continue
			}
			return token.Token{Type: token.QuotedIdentifier, Literal: value.String(), Line: line, Column: column}
		}
		value.WriteRune(current)
		l.advance()
	}
	return token.Token{Type: token.Illegal, Literal: value.String(), Line: line, Column: column}
}

func (l *Lexer) readNumber(line, column int) token.Token {
	start := l.position
	for l.position < len(l.input) && l.input[l.position] >= '0' && l.input[l.position] <= '9' {
		l.advance()
	}
	kind := token.Integer
	if l.position < len(l.input) && l.input[l.position] == '.' {
		kind = token.Decimal
		l.advance()
		for l.position < len(l.input) && l.input[l.position] >= '0' && l.input[l.position] <= '9' {
			l.advance()
		}
	}
	return token.Token{Type: kind, Literal: string(l.input[start:l.position]), Line: line, Column: column}
}

func (l *Lexer) peek() rune {
	if l.position+1 >= len(l.input) {
		return 0
	}
	return l.input[l.position+1]
}

func (l *Lexer) readIdentifier(line, column int) token.Token {
	start := l.position
	for l.position < len(l.input) && isIdentifierPart(l.input[l.position]) {
		l.advance()
	}

	literal := string(l.input[start:l.position])
	return token.Token{
		Type:    keywordType(literal),
		Literal: literal,
		Line:    line,
		Column:  column,
	}
}

func (l *Lexer) advance() {
	current := l.input[l.position]
	if current == '\r' {
		l.position++
		if l.position < len(l.input) && l.input[l.position] == '\n' {
			l.position++
		}
		l.line++
		l.column = 1
		return
	}

	l.position++
	if current == '\n' {
		l.line++
		l.column = 1
		return
	}
	l.column++
}

func keywordType(literal string) token.Type {
	switch strings.ToUpper(literal) {
	case "SELECT":
		return token.Select
	case "INSERT":
		return token.Insert
	case "UPDATE":
		return token.Update
	case "DELETE":
		return token.Delete
	case "FROM":
		return token.From
	case "SET":
		return token.Set
	case "INTO":
		return token.Into
	case "VALUES":
		return token.Values
	case "WHERE":
		return token.Where
	case "AND":
		return token.And
	case "OR":
		return token.Or
	case "NOT":
		return token.Not
	case "IS":
		return token.Is
	case "NULL":
		return token.Null
	case "TRUE":
		return token.True
	case "FALSE":
		return token.False
	case "AS":
		return token.As
	default:
		return token.Identifier
	}
}

func isIdentifierStart(char rune) bool {
	return char == '_' || isASCIILetter(char)
}

func isIdentifierPart(char rune) bool {
	return isIdentifierStart(char) || (char >= '0' && char <= '9')
}

func isASCIILetter(char rune) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}
