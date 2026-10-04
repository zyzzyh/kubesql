package eval

import (
	"fmt"
	"strings"

	"github.com/zyzzyh/kubesql/internal/ast"
)

type valueKind uint8

const (
	nullValue valueKind = iota
	stringValue
	integerValue
	decimalValue
	booleanValue
)

type value struct {
	kind valueKind
	data any
}

func evaluateValue(expression ast.Expression, row map[string]any) (value, error) {
	switch expression := expression.(type) {
	case *ast.ColumnReference:
		key := strings.ToLower(expression.Name)
		data, ok := row[key]
		if !ok {
			return value{}, fmt.Errorf("E_EVAL: unknown column %q", expression.Name)
		}
		return valueFromAny(data)
	case *ast.Literal:
		switch expression.Kind {
		case "null":
			return value{kind: nullValue}, nil
		case "string":
			return value{kind: stringValue, data: expression.Value.(string)}, nil
		case "integer":
			return value{kind: integerValue, data: expression.Value.(int64)}, nil
		case "decimal":
			return value{kind: decimalValue, data: expression.Value.(float64)}, nil
		case "boolean":
			return value{kind: booleanValue, data: expression.Value.(bool)}, nil
		default:
			return value{}, fmt.Errorf("E_EVAL: unsupported literal kind %q", expression.Kind)
		}
	default:
		return value{}, fmt.Errorf("E_EVAL: expression cannot be used as a value")
	}
}

func valueFromAny(data any) (value, error) {
	if data == nil {
		return value{kind: nullValue}, nil
	}
	switch data := data.(type) {
	case string:
		return value{kind: stringValue, data: data}, nil
	case bool:
		return value{kind: booleanValue, data: data}, nil
	case int:
		return value{kind: integerValue, data: int64(data)}, nil
	case int8:
		return value{kind: integerValue, data: int64(data)}, nil
	case int16:
		return value{kind: integerValue, data: int64(data)}, nil
	case int32:
		return value{kind: integerValue, data: int64(data)}, nil
	case int64:
		return value{kind: integerValue, data: data}, nil
	case uint:
		return value{kind: integerValue, data: uint64(data)}, nil
	case uint8:
		return value{kind: integerValue, data: uint64(data)}, nil
	case uint16:
		return value{kind: integerValue, data: uint64(data)}, nil
	case uint32:
		return value{kind: integerValue, data: uint64(data)}, nil
	case uint64:
		return value{kind: integerValue, data: data}, nil
	case float32:
		return value{kind: decimalValue, data: float64(data)}, nil
	case float64:
		return value{kind: decimalValue, data: data}, nil
	default:
		return value{}, fmt.Errorf("E_EVAL: unsupported value type %T", data)
	}
}
