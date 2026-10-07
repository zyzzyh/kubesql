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

// Parse parses one supported statement, an optional semicolon, and EOF without
// making any Kubernetes request.
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

// parseInsert parses the manifest column and value tuple; JSON is validated
// later by the write executor.
func (p *Parser) parseInsert() (*ast.InsertStatement, error) {
	p.advance()
	if err := p.expect(token.Into, "INTO"); err != nil {
		return nil, err
	}
	table, quoted, err := p.parseIdentifier("table name")
	if err != nil {
		return nil, err
	}
	statement := &ast.InsertStatement{Type: "insert", Table: table, TableQuoted: quoted}
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
		column, _, err := p.parseIdentifier("column name")
		if err != nil {
			return nil, err
		}
		columns = append(columns, column)
		if p.current.Type != token.Comma {
			return columns, nil
		}
		p.advance()
	}
}

// parseUpdate parses assignments followed by an optional WHERE expression.
func (p *Parser) parseUpdate() (*ast.UpdateStatement, error) {
	p.advance()
	table, quoted, err := p.parseIdentifier("table name")
	if err != nil {
		return nil, err
	}
	statement := &ast.UpdateStatement{Type: "update", Table: table, TableQuoted: quoted}
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
		column, quoted, err := p.parseIdentifier("column name")
		if err != nil {
			return nil, err
		}
		if err := p.expect(token.Equal, "="); err != nil {
			return nil, err
		}
		value, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, ast.Assignment{Column: column, ColumnQuoted: quoted, Value: value})
		if p.current.Type != token.Comma {
			return assignments, nil
		}
		p.advance()
	}
}

// parseDelete parses a target table and its filtering expression.
func (p *Parser) parseDelete() (*ast.DeleteStatement, error) {
	p.advance()
	if err := p.expect(token.From, "FROM"); err != nil {
		return nil, err
	}
	table, quoted, err := p.parseIdentifier("table name")
	if err != nil {
		return nil, err
	}
	statement := &ast.DeleteStatement{Type: "delete", Table: table, TableQuoted: quoted}
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

// parseSelect parses projections, a source table, and an optional WHERE.
func (p *Parser) parseSelect() (*ast.SelectStatement, error) {
	p.advance() // SELECT was checked by Parse.

	columns, err := p.parseSelectList()
	if err != nil {
		return nil, err
	}
	if err := p.expect(token.From, "FROM"); err != nil {
		return nil, err
	}
	table, quoted, err := p.parseIdentifier("table name")
	if err != nil {
		return nil, err
	}

	var where ast.Expression
	if p.current.Type == token.Where {
		p.advance()
		where, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}
	return &ast.SelectStatement{Type: "select", Columns: columns, Table: table, TableQuoted: quoted, Where: where}, nil
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
	name, quoted, err := p.parseIdentifier("column name")
	if err != nil {
		return nil, err
	}
	column := &ast.Column{Type: "column", Name: name, Quoted: quoted}
	if p.current.Type == token.As {
		p.advance()
		alias, aliasQuoted, err := p.parseIdentifier("alias")
		if err != nil {
			return nil, err
		}
		column.Alias = alias
		column.AliasQuoted = aliasQuoted
	}
	return column, nil
}

// The call chain below mirrors SQL precedence from low to high:
// OR -> AND -> NOT -> comparison. Each higher-level parser consumes a
// complete lower-precedence expression, so the resulting AST has the same
// grouping as SQL without a separate precedence table.
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
	case token.Identifier, token.QuotedIdentifier:
		p.advance()
		return &ast.ColumnReference{Type: "column_reference", Name: current.Literal, Quoted: current.Type == token.QuotedIdentifier}, nil
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

func (p *Parser) parseIdentifier(name string) (string, bool, error) {
	if p.current.Type != token.Identifier && p.current.Type != token.QuotedIdentifier {
		return "", false, expected(name, p.current)
	}
	quoted := p.current.Type == token.QuotedIdentifier
	literal := p.current.Literal
	p.advance()
	return literal, quoted, nil
}

func (p *Parser) advance() {
	p.current = p.lexer.NextToken()
}
