package eval

import (
	"fmt"
	"reflect"
)

func compare(operator string, left, right value) (Truth, error) {
	if left.kind == nullValue || right.kind == nullValue {
		return Unknown, nil
	}
	if isNumeric(left.kind) && isNumeric(right.kind) {
		return compareNumbers(operator, left, right), nil
	}
	if left.kind != right.kind {
		return Unknown, fmt.Errorf("E_EVAL: incompatible types in comparison")
	}
	switch left.kind {
	case stringValue:
		return compareOrdered(operator, left.data.(string), right.data.(string))
	case booleanValue:
		if operator != "=" && operator != "<>" {
			return Unknown, fmt.Errorf("E_EVAL: operator %s is not supported for booleans", operator)
		}
		return equality(operator, left.data.(bool) == right.data.(bool)), nil
	default:
		return Unknown, fmt.Errorf("E_EVAL: unsupported comparison type")
	}
}

func compareNumbers(operator string, left, right value) Truth {
	leftNumber := numericFloat(left)
	rightNumber := numericFloat(right)
	switch operator {
	case "=":
		return truth(leftNumber == rightNumber)
	case "<>":
		return truth(leftNumber != rightNumber)
	case ">":
		return truth(leftNumber > rightNumber)
	case ">=":
		return truth(leftNumber >= rightNumber)
	case "<":
		return truth(leftNumber < rightNumber)
	case "<=":
		return truth(leftNumber <= rightNumber)
	default:
		return Unknown
	}
}

func compareOrdered(operator string, left, right string) (Truth, error) {
	switch operator {
	case "=":
		return truth(left == right), nil
	case "<>":
		return truth(left != right), nil
	case ">":
		return truth(left > right), nil
	case ">=":
		return truth(left >= right), nil
	case "<":
		return truth(left < right), nil
	case "<=":
		return truth(left <= right), nil
	default:
		return Unknown, fmt.Errorf("E_EVAL: unsupported comparison operator %q", operator)
	}
}

func numericFloat(value value) float64 {
	if value.kind == decimalValue {
		return value.data.(float64)
	}
	return reflect.ValueOf(value.data).Convert(reflect.TypeOf(float64(0))).Float()
}

func isNumeric(kind valueKind) bool {
	return kind == integerValue || kind == decimalValue
}

func equality(operator string, equal bool) Truth {
	if operator == "<>" {
		equal = !equal
	}
	return truth(equal)
}
