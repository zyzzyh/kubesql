// Package parser converts SQL tokens into an abstract syntax tree.
package parser

import (
	"strconv"

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

// Parse parses one supported statement, an optional semicolon, and EOF.
func (p *Parser) Parse() (ast.Statement, error) {
	var statement ast.Statement
	var err error
	switch p.current.Type {
	case token.Select:
		statement, err = p.parseSelect()
	case token.Insert:
		statement, err = p.parseInsert()
	case token.Update:
		statement, err = p.parseUpdate()
	case token.Delete:
		statement, err = p.parseDelete()
	default:
		return nil, expected("SELECT", p.current)
	}
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

func (p *Parser) parseInsert() (*ast.InsertStatement, error) {
	p.advance()
	if err := p.expect(token.Into, "INTO"); err != nil {
		return nil, err
	}
	if p.current.Type != token.Identifier {
		return nil, expected("table name", p.current)
	}
	statement := &ast.InsertStatement{Type: "insert", Table: p.current.Literal}
	p.advance()
	if err := p.expect(token.LeftParen, "("); err != nil {
		return nil, err
	}
	columns, err := p.parseInsertColumns()
	if err != nil {
		return nil, err
	}
	statement.Columns = columns
	if err := p.expect(token.RightParen, ")"); err != nil {
		return nil, err
	}
	if err := p.expect(token.Values, "VALUES"); err != nil {
		return nil, err
	}
	if err := p.expect(token.LeftParen, "("); err != nil {
		return nil, err
	}
	value, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	statement.Values = []ast.Expression{value}
	if err := p.expect(token.RightParen, ")"); err != nil {
		return nil, err
	}
	return statement, nil
}

func (p *Parser) parseInsertColumns() ([]string, error) {
	columns := make([]string, 0, 1)
	for {
		if p.current.Type != token.Identifier {
			return nil, expected("column name", p.current)
		}
		columns = append(columns, p.current.Literal)
		p.advance()
		if p.current.Type != token.Comma {
			return columns, nil
		}
		p.advance()
	}
}

func (p *Parser) parseUpdate() (*ast.UpdateStatement, error) {
	p.advance()
	if p.current.Type != token.Identifier {
		return nil, expected("table name", p.current)
	}
	statement := &ast.UpdateStatement{Type: "update", Table: p.current.Literal}
	p.advance()
	if err := p.expect(token.Set, "SET"); err != nil {
		return nil, err
	}
	assignments, err := p.parseAssignments()
	if err != nil {
		return nil, err
	}
	statement.Assignments = assignments
	if p.current.Type == token.Where {
		p.advance()
		statement.Where, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}
	return statement, nil
}

func (p *Parser) parseAssignments() ([]ast.Assignment, error) {
	assignments := make([]ast.Assignment, 0, 1)
	for {
		if p.current.Type != token.Identifier {
			return nil, expected("column name", p.current)
		}
		column := p.current.Literal
		p.advance()
		if err := p.expect(token.Equal, "="); err != nil {
			return nil, err
		}
		value, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, ast.Assignment{Column: column, Value: value})
		if p.current.Type != token.Comma {
			return assignments, nil
		}
		p.advance()
	}
}

func (p *Parser) parseDelete() (*ast.DeleteStatement, error) {
	p.advance()
	if err := p.expect(token.From, "FROM"); err != nil {
		return nil, err
	}
	if p.current.Type != token.Identifier {
		return nil, expected("table name", p.current)
	}
	statement := &ast.DeleteStatement{Type: "delete", Table: p.current.Literal}
	p.advance()
	var where ast.Expression
	if p.current.Type == token.Where {
		p.advance()
		parsed, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		where = parsed
	}
	statement.Where = where
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

	var where ast.Expression
	if p.current.Type == token.Where {
		p.advance()
		where, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}
	return &ast.SelectStatement{Type: "select", Columns: columns, Table: table, Where: where}, nil
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

// parseExpression applies SQL boolean precedence: NOT, then AND, then OR.
func (p *Parser) parseExpression() (ast.Expression, error) {
	return p.parseOr()
}

func (p *Parser) parseOr() (ast.Expression, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.current.Type == token.Or {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpression{Type: "binary", Operator: "OR", Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseAnd() (ast.Expression, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.current.Type == token.And {
		p.advance()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpression{Type: "binary", Operator: "AND", Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseNot() (ast.Expression, error) {
	if p.current.Type == token.Not {
		p.advance()
		expression, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpression{Type: "unary", Operator: "NOT", Expression: expression}, nil
	}
	return p.parseComparison()
}

func (p *Parser) parseComparison() (ast.Expression, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	if p.current.Type == token.Is {
		p.advance()
		not := false
		if p.current.Type == token.Not {
			not = true
			p.advance()
		}
		if err := p.expect(token.Null, "NULL"); err != nil {
			return nil, err
		}
		return &ast.IsNullExpression{Type: "is_null", Expression: left, Not: not}, nil
	}

	if isComparisonOperator(p.current.Type) {
		operator := p.current.Literal
		p.advance()
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &ast.BinaryExpression{Type: "binary", Operator: operator, Left: left, Right: right}, nil
	}
	return left, nil
}

func (p *Parser) parsePrimary() (ast.Expression, error) {
	current := p.current
	switch current.Type {
	case token.Identifier:
		p.advance()
		return &ast.ColumnReference{Type: "column_reference", Name: current.Literal}, nil
	case token.String:
		p.advance()
		return &ast.Literal{Type: "literal", Kind: "string", Value: current.Literal}, nil
	case token.Integer:
		p.advance()
		value, err := strconv.ParseInt(current.Literal, 10, 64)
		if err != nil {
			return nil, expected("integer literal", current)
		}
		return &ast.Literal{Type: "literal", Kind: "integer", Value: value}, nil
	case token.Decimal:
		p.advance()
		value, err := strconv.ParseFloat(current.Literal, 64)
		if err != nil {
			return nil, expected("decimal literal", current)
		}
		return &ast.Literal{Type: "literal", Kind: "decimal", Value: value}, nil
	case token.True, token.False:
		p.advance()
		return &ast.Literal{Type: "literal", Kind: "boolean", Value: current.Type == token.True}, nil
	case token.Null:
		p.advance()
		return &ast.Literal{Type: "literal", Kind: "null", Value: nil}, nil
	case token.LeftParen:
		p.advance()
		expression, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if err := p.expect(token.RightParen, ")"); err != nil {
			return nil, err
		}
		return expression, nil
	default:
		return nil, expected("expression", current)
	}
}

func isComparisonOperator(kind token.Type) bool {
	switch kind {
	case token.Equal, token.NotEqual, token.Greater, token.GreaterEqual, token.Less, token.LessEqual:
		return true
	default:
		return false
	}
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
