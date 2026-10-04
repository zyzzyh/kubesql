package eval

import (
	"testing"

	"github.com/zyzzyh/kubesql/internal/ast"
	"github.com/zyzzyh/kubesql/internal/parser"
)

func TestEvaluateComparisonsAndNull(t *testing.T) {
	row := map[string]any{"name": "web", "replicas": int32(3), "missing": nil}
	tests := []struct {
		name string
		sql  string
		want Truth
	}{
		{"string equality", "name = 'web'", True},
		{"string inequality", "name <> 'web'", False},
		{"integer comparison", "replicas >= 3", True},
		{"decimal comparison", "replicas < 3.5", True},
		{"null comparison", "missing = 'value'", Unknown},
		{"is null", "missing IS NULL", True},
		{"is not null", "missing IS NOT NULL", False},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expression := parseExpression(t, test.sql)
			got, err := Evaluate(expression, row)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("truth: got %v, want %v", got, test.want)
			}
		})
	}
}

func TestEvaluateLogicalThreeValuedLogic(t *testing.T) {
	row := map[string]any{}
	tests := []struct {
		sql  string
		want Truth
	}{
		{"NULL AND FALSE", False},
		{"NULL AND TRUE", Unknown},
		{"NULL OR FALSE", Unknown},
		{"NULL OR TRUE", True},
		{"NOT NULL", Unknown},
		{"FALSE OR TRUE AND FALSE", False},
		{"(FALSE OR TRUE) AND FALSE", False},
	}
	for _, test := range tests {
		t.Run(test.sql, func(t *testing.T) {
			got, err := Evaluate(parseExpression(t, test.sql), row)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("truth: got %v, want %v", got, test.want)
			}
		})
	}
}

func TestEvaluateRejectsIncompatibleTypes(t *testing.T) {
	_, err := Evaluate(parseExpression(t, "name = 1"), map[string]any{"name": "web"})
	if err == nil {
		t.Fatal("incompatible string and integer comparison was accepted")
	}
}

func parseExpression(t *testing.T, where string) ast.Expression {
	t.Helper()
	statement, err := parser.New("SELECT name FROM deployments WHERE " + where).Parse()
	if err != nil {
		t.Fatal(err)
	}
	return statement.(*ast.SelectStatement).Where
}
