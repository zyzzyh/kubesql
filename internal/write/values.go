package write

import (
	"encoding/json"
	"fmt"

	"github.com/zyzzyh/kubesql/internal/ast"
)

func literalValue(expression ast.Expression) (any, error) {
	literal, ok := expression.(*ast.Literal)
	if !ok {
		return nil, fmt.Errorf("E_SEMANTIC: assignment value must be a literal")
	}
	switch literal.Kind {
	case "string", "integer", "decimal", "boolean":
		return literal.Value, nil
	case "null":
		return nil, nil
	default:
		return nil, fmt.Errorf("E_SEMANTIC: unsupported assignment value %q", literal.Kind)
	}
}

func jsonMap(value any, column string) (map[string]string, error) {
	text, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("E_SEMANTIC: %s must be a JSON object string", column)
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(text), &result); err != nil || result == nil {
		return nil, fmt.Errorf("E_SEMANTIC: %s must be a JSON object", column)
	}
	return result, nil
}
