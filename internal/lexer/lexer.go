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

// NextToken returns the next token, or EOF after all input has been read.
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
	case "FROM":
		return token.From
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
