// Package eval evaluates WHERE expressions using SQL three-valued logic.
package eval

import (
	"fmt"
	"strings"

	"github.com/zyzzyh/kubesql/internal/ast"
)

// Truth is the result of a SQL condition. UNKNOWN is produced by NULL.
type Truth uint8

const (
	Unknown Truth = iota
	False
	True
)

// Evaluate evaluates expression against one resource row.
func Evaluate(expression ast.Expression, row map[string]any) (Truth, error) {
	return evaluateBoolean(expression, row)
}

func evaluateBoolean(expression ast.Expression, row map[string]any) (Truth, error) {
	switch expression := expression.(type) {
	case *ast.BinaryExpression:
		switch strings.ToUpper(expression.Operator) {
		case "AND", "OR":
			left, err := evaluateBoolean(expression.Left, row)
			if err != nil {
				return Unknown, err
			}
			right, err := evaluateBoolean(expression.Right, row)
			if err != nil {
				return Unknown, err
			}
			if strings.EqualFold(expression.Operator, "AND") {
				return and(left, right), nil
			}
			return or(left, right), nil
		default:
			left, err := evaluateValue(expression.Left, row)
			if err != nil {
				return Unknown, err
			}
			right, err := evaluateValue(expression.Right, row)
			if err != nil {
				return Unknown, err
			}
			return compare(expression.Operator, left, right)

		}
	case *ast.UnaryExpression:
		if !strings.EqualFold(expression.Operator, "NOT") {
			return Unknown, fmt.Errorf("E_EVAL: unsupported unary operator %q", expression.Operator)
		}
		result, err := evaluateBoolean(expression.Expression, row)
		if err != nil {
			return Unknown, err
		}
		return negate(result), nil
	case *ast.IsNullExpression:
		value, err := evaluateValue(expression.Expression, row)
		if err != nil {
			return Unknown, err
		}
		isNull := value.kind == nullValue
		if expression.Not {
			isNull = !isNull
		}
		return truth(isNull), nil
	default:
		value, err := evaluateValue(expression, row)
		if err != nil {
			return Unknown, err
		}
		if value.kind == nullValue {
			return Unknown, nil
		}
		if value.kind != booleanValue {
			return Unknown, fmt.Errorf("E_EVAL: WHERE expression must be boolean")
		}
		return truth(value.data.(bool)), nil
	}
}
