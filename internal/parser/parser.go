// Package parser converts SQL tokens into an abstract syntax tree.
package parser

import (
	"github.com/zyzzyh/kubesql/internal/ast"
	"github.com/zyzzyh/kubesql/internal/lexer"
	"github.com/zyzzyh/kubesql/internal/token"
)

// Parser keeps the current token while descending through grammar rules.
type Parser struct {
	lexer   *lexer.Lexer
	current token.Token
}

// New prepares a parser for one SQL statement.
func New(input string) *Parser {
	l := lexer.New(input)
	return &Parser{lexer: l, current: l.NextToken()}
}

// Parse parses one SELECT statement, an optional semicolon, and EOF.
func (p *Parser) Parse() (ast.Statement, error) {
	if p.current.Type != token.Select {
		return nil, expected("SELECT", p.current)
	}

	statement, err := p.parseSelect()
	if err != nil {
		return nil, err
	}
	if p.current.Type == token.Semicolon {
		p.advance()
	}
	if err := p.expect(token.EOF, "end of input"); err != nil {
		return nil, err
	}
	return statement, nil
}

func (p *Parser) parseSelect() (*ast.SelectStatement, error) {
	p.advance() // SELECT was checked by Parse.

	columns, err := p.parseSelectList()
	if err != nil {
		return nil, err
	}
	if err := p.expect(token.From, "FROM"); err != nil {
		return nil, err
	}
	if p.current.Type != token.Identifier {
		return nil, expected("table name", p.current)
	}

	table := p.current.Literal
	p.advance()
	return &ast.SelectStatement{Type: "select", Columns: columns, Table: table}, nil
}

func (p *Parser) parseSelectList() ([]ast.SelectItem, error) {
	if p.current.Type == token.Asterisk {
		p.advance()
		return []ast.SelectItem{&ast.Star{Type: "star"}}, nil
	}

	first, err := p.parseColumn()
	if err != nil {
		return nil, err
	}
	columns := []ast.SelectItem{first}
	for p.current.Type == token.Comma {
		p.advance()
		column, err := p.parseColumn()
		if err != nil {
			return nil, err
		}
		columns = append(columns, column)
	}
	return columns, nil
}

func (p *Parser) parseColumn() (*ast.Column, error) {
	if p.current.Type != token.Identifier {
		return nil, expected("column name", p.current)
	}
	column := &ast.Column{Type: "column", Name: p.current.Literal}
	p.advance()
	return column, nil
}

func (p *Parser) expect(kind token.Type, name string) error {
	if p.current.Type != kind {
		return expected(name, p.current)
	}
	p.advance()
	return nil
}

func (p *Parser) advance() {
	p.current = p.lexer.NextToken()
}
