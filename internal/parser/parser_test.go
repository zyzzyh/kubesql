package parser

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/zyzzyh/kubesql/internal/ast"
)

func TestParseSelect(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "named columns",
			input: "select name, replicas FROM deployments;",
			want:  `{"type":"select","columns":[{"type":"column","name":"name"},{"type":"column","name":"replicas"}],"table":"deployments"}`,
		},
		{
			name:  "star without semicolon",
			input: "SELECT * FROM ingresses",
			want:  `{"type":"select","columns":[{"type":"star"}],"table":"ingresses"}`,
		},
		{
			name:  "unknown names remain syntax-valid",
			input: "SELECT unlisted FROM unknown_table",
			want:  `{"type":"select","columns":[{"type":"column","name":"unlisted"}],"table":"unknown_table"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			statement, err := New(test.input).Parse()
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(statement)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != test.want {
				t.Fatalf("AST JSON: got %s, want %s", got, test.want)
			}
		})
	}
}

func TestParseStarIsDistinctNode(t *testing.T) {
	statement, err := New("SELECT * FROM deployments").Parse()
	if err != nil {
		t.Fatal(err)
	}
	selection, ok := statement.(*ast.SelectStatement)
	if !ok {
		t.Fatalf("statement type: got %T", statement)
	}
	if len(selection.Columns) != 1 {
		t.Fatalf("column count: got %d, want 1", len(selection.Columns))
	}
	if _, ok := selection.Columns[0].(*ast.Star); !ok {
		t.Fatalf("selection type: got %T, want *ast.Star", selection.Columns[0])
	}
}

func TestParseWherePrecedence(t *testing.T) {
	statement, err := New("SELECT name FROM deployments WHERE name = 'web' OR name = 'worker' AND replicas >= 3").Parse()
	if err != nil {
		t.Fatal(err)
	}
	selectStatement := statement.(*ast.SelectStatement)
	got, err := json.Marshal(selectStatement.Where)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"binary","operator":"OR","left":{"type":"binary","operator":"=","left":{"type":"column_reference","name":"name"},"right":{"type":"literal","kind":"string","value":"web"}},"right":{"type":"binary","operator":"AND","left":{"type":"binary","operator":"=","left":{"type":"column_reference","name":"name"},"right":{"type":"literal","kind":"string","value":"worker"}},"right":{"type":"binary","operator":"\u003e=","left":{"type":"column_reference","name":"replicas"},"right":{"type":"literal","kind":"integer","value":3}}}}`
	if string(got) != want {
		t.Fatalf("WHERE AST: got %s, want %s", got, want)
	}
}

func TestParseWhereParenthesesNotAndNull(t *testing.T) {
	tests := []string{
		"SELECT name FROM deployments WHERE NOT (name = 'web')",
		"SELECT name FROM ingresses WHERE default_backend_service IS NULL",
		"SELECT name FROM ingresses WHERE default_backend_service IS NOT NULL",
		"SELECT name FROM deployments WHERE replicas < 2.5 AND replicas <> NULL",
	}
	for _, input := range tests {
		if _, err := New(input).Parse(); err != nil {
			t.Errorf("%q: %v", input, err)
		}
	}
}

func TestParseUpdateAndDelete(t *testing.T) {
	statement, err := New("UPDATE deployments SET replicas = 3, labels = '{''app'': ''web''}' WHERE name = 'web'").Parse()
	if err != nil {
		t.Fatal(err)
	}
	update, ok := statement.(*ast.UpdateStatement)
	if !ok || len(update.Assignments) != 2 || update.Table != "deployments" {
		t.Fatalf("unexpected UPDATE AST: %#v", statement)
	}
	statement, err = New("DELETE FROM deployments WHERE replicas = 0;").Parse()
	if err != nil {
		t.Fatal(err)
	}
	deleteStatement, ok := statement.(*ast.DeleteStatement)
	if !ok || deleteStatement.Table != "deployments" || deleteStatement.Where == nil {
		t.Fatalf("unexpected DELETE AST: %#v", statement)
	}
}

func TestParseInsert(t *testing.T) {
	statement, err := New("INSERT INTO deployments (manifest) VALUES ('{''kind'': ''Deployment''}')").Parse()
	if err != nil {
		t.Fatal(err)
	}
	insert, ok := statement.(*ast.InsertStatement)
	if !ok {
		t.Fatalf("statement type: got %T", statement)
	}
	if insert.Table != "deployments" || len(insert.Columns) != 1 || insert.Columns[0] != "manifest" || len(insert.Values) != 1 {
		t.Fatalf("unexpected INSERT AST: %#v", insert)
	}
	literal, ok := insert.Values[0].(*ast.Literal)
	if !ok || literal.Kind != "string" || literal.Value != "{'kind': 'Deployment'}" {
		t.Fatalf("unexpected INSERT value: %#v", insert.Values[0])
	}
}

func TestParseWhereErrors(t *testing.T) {
	tests := []string{
		"SELECT name FROM deployments WHERE name =",
		"SELECT name FROM deployments WHERE name IS",
		"SELECT name FROM deployments WHERE (name = 'web'",
	}
	for _, input := range tests {
		if _, err := New(input).Parse(); err == nil {
			t.Errorf("%q: expected parse error", input)
		}
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Error
	}{
		{"missing select", "name FROM deployments", Error{Code: "E_PARSE", Line: 1, Column: 1, Message: `expected SELECT, got "name"`}},
		{"missing column", "SELECT name, FROM deployments;", Error{Code: "E_PARSE", Line: 1, Column: 14, Message: `expected column name, got "FROM"`}},
		{"missing from", "SELECT name deployments", Error{Code: "E_PARSE", Line: 1, Column: 13, Message: `expected FROM, got "deployments"`}},
		{"missing table", "SELECT name FROM", Error{Code: "E_PARSE", Line: 1, Column: 17, Message: `expected table name, got "end of input"`}},
		{"extra statement", "SELECT name FROM deployments; SELECT * FROM ingresses", Error{Code: "E_PARSE", Line: 1, Column: 31, Message: `expected end of input, got "SELECT"`}},
		{"invalid character", "SELECT @ FROM deployments", Error{Code: "E_PARSE", Line: 1, Column: 8, Message: `expected column name, got "@"`}},
		{"multiline position", "SELECT name,\nFROM deployments", Error{Code: "E_PARSE", Line: 2, Column: 1, Message: `expected column name, got "FROM"`}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(test.input).Parse()
			var parseErr *Error
			if !errors.As(err, &parseErr) {
				t.Fatalf("error type: got %T (%v), want *parser.Error", err, err)
			}
			if !reflect.DeepEqual(*parseErr, test.want) {
				t.Fatalf("parse error: got %#v, want %#v", *parseErr, test.want)
			}
		})
	}
}
