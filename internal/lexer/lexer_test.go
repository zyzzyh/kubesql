package lexer

import (
	"testing"

	"github.com/zyzzyh/kubesql/internal/token"
)

func TestNextToken(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []token.Token
	}{
		{
			name:  "select list",
			input: "SeLeCt name_2, replicas FROM deployments;",
			want: []token.Token{
				{Type: token.Select, Literal: "SeLeCt", Line: 1, Column: 1},
				{Type: token.Identifier, Literal: "name_2", Line: 1, Column: 8},
				{Type: token.Comma, Literal: ",", Line: 1, Column: 14},
				{Type: token.Identifier, Literal: "replicas", Line: 1, Column: 16},
				{Type: token.From, Literal: "FROM", Line: 1, Column: 25},
				{Type: token.Identifier, Literal: "deployments", Line: 1, Column: 30},
				{Type: token.Semicolon, Literal: ";", Line: 1, Column: 41},
				{Type: token.EOF, Line: 1, Column: 42},
			},
		},
		{
			name:  "star and optional semicolon",
			input: "SELECT * FROM deployments",
			want: []token.Token{
				{Type: token.Select, Literal: "SELECT", Line: 1, Column: 1},
				{Type: token.Asterisk, Literal: "*", Line: 1, Column: 8},
				{Type: token.From, Literal: "FROM", Line: 1, Column: 10},
				{Type: token.Identifier, Literal: "deployments", Line: 1, Column: 15},
				{Type: token.EOF, Line: 1, Column: 26},
			},
		},
		{
			name:  "line endings",
			input: "SELECT\r\nname FROM\rdeployments",
			want: []token.Token{
				{Type: token.Select, Literal: "SELECT", Line: 1, Column: 1},
				{Type: token.Identifier, Literal: "name", Line: 2, Column: 1},
				{Type: token.From, Literal: "FROM", Line: 2, Column: 6},
				{Type: token.Identifier, Literal: "deployments", Line: 3, Column: 1},
				{Type: token.EOF, Line: 3, Column: 12},
			},
		},
		{
			name:  "empty input",
			input: "",
			want:  []token.Token{{Type: token.EOF, Line: 1, Column: 1}},
		},
		{
			name:  "unsupported character",
			input: "@",
			want: []token.Token{
				{Type: token.Illegal, Literal: "@", Line: 1, Column: 1},
				{Type: token.EOF, Line: 1, Column: 2},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lexer := New(test.input)
			for index, want := range test.want {
				got := lexer.NextToken()
				if got != want {
					t.Fatalf("token %d: got %#v, want %#v", index, got, want)
				}
			}
		})
	}
}
