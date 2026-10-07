package write

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zyzzyh/kubesql/internal/ast"
)

var writable = map[string]map[string]bool{
	"namespaces":  {"labels": true, "annotations": true},
	"deployments": {"replicas": true, "labels": true, "annotations": true},
	"ingresses":   {"default_backend_service": true, "labels": true, "annotations": true},
}

func validateUpdate(statement *ast.UpdateStatement) error {
	// validateUpdate enforces the fixed tables' writable columns and literal
	// types before any list or patch request is made.
	table := strings.ToLower(statement.Table)
	columns, ok := writable[table]
	if !ok {
		return fmt.Errorf("E_SEMANTIC: unknown table %q", statement.Table)
	}
	if len(statement.Assignments) == 0 {
		return fmt.Errorf("E_SEMANTIC: UPDATE requires at least one assignment")
	}
	seen := map[string]bool{}
	for _, assignment := range statement.Assignments {
		column := strings.ToLower(assignment.Column)
		if !columns[column] {
			return fmt.Errorf("E_SEMANTIC: column %q cannot be updated in table %q", assignment.Column, table)
		}
		if seen[column] {
			return fmt.Errorf("E_SEMANTIC: column %q is assigned more than once", assignment.Column)
		}
		seen[column] = true
		if column == "replicas" {
			literal, ok := assignment.Value.(*ast.Literal)
			number, numberOK := intLiteralValue(literal)
			if !ok || literal.Kind != "integer" || !numberOK || number < 0 {
				return fmt.Errorf("E_SEMANTIC: replicas must be a non-negative integer")
			}
		}
		if err := validateAssignmentValue(column, assignment.Value); err != nil {
			return err
		}
	}
	return validateWhere(table, statement.Where)
}

func intLiteralValue(literal *ast.Literal) (int64, bool) {
	if literal == nil || literal.Kind != "integer" {
		return 0, false
	}
	number, ok := literal.Value.(int64)
	return number, ok
}

func validateAssignmentValue(column string, expression ast.Expression) error {
	literal, ok := expression.(*ast.Literal)
	if !ok {
		return fmt.Errorf("E_SEMANTIC: assignment value must be a literal")
	}
	switch column {
	case "labels", "annotations":
		text, ok := literal.Value.(string)
		if literal.Kind != "string" || !ok {
			return fmt.Errorf("E_SEMANTIC: %s must be a JSON object string", column)
		}
		var object map[string]string
		if err := json.Unmarshal([]byte(text), &object); err != nil || object == nil {
			return fmt.Errorf("E_SEMANTIC: %s must be a JSON object", column)
		}
	case "default_backend_service":
		text, ok := literal.Value.(string)
		if literal.Kind != "string" || !ok || text == "" {
			return fmt.Errorf("E_SEMANTIC: default_backend_service must be a non-empty Service name")
		}
	}
	return nil
}

func validateDelete(statement *ast.DeleteStatement) error {
	if _, ok := writable[strings.ToLower(statement.Table)]; !ok {
		return fmt.Errorf("E_SEMANTIC: unknown table %q", statement.Table)
	}
	return validateWhere(strings.ToLower(statement.Table), statement.Where)
}

func validateWhere(table string, expression ast.Expression) error {
	// validateWhere restricts fixed-table predicates to fields with known types.
	if expression == nil {
		return fmt.Errorf("E_WHERE_REQUIRED: write statement requires WHERE")
	}
	switch expression := expression.(type) {
	case *ast.ColumnReference:
		name := strings.ToLower(expression.Name)
		allowed := map[string]bool{"name": true}
		if table != "namespaces" {
			allowed["namespace"] = true
		}
		if table == "deployments" {
			allowed["replicas"] = true
		}
		if table == "ingresses" {
			allowed["default_backend_service"] = true
		}
		if !allowed[name] {
			return fmt.Errorf("E_SEMANTIC: unknown column %q for table %q", expression.Name, table)
		}
	case *ast.BinaryExpression:
		if err := validateWhere(table, expression.Left); err != nil {
			return err
		}
		return validateWhere(table, expression.Right)
	case *ast.UnaryExpression:
		return validateWhere(table, expression.Expression)
	case *ast.IsNullExpression:
		return validateWhere(table, expression.Expression)
	case *ast.Literal:
		return nil
	default:
		return fmt.Errorf("E_SEMANTIC: unsupported WHERE expression %T", expression)
	}
	return nil
}
