package write

import (
	"github.com/zyzzyh/kubesql/internal/apperror"
	"github.com/zyzzyh/kubesql/internal/ast"
	"github.com/zyzzyh/kubesql/internal/eval"
)

func addError(result *Result, resource, namespace, name string, err error) {
	reason := apperror.ReasonOf(err)
	if reason == apperror.CodeAlreadyExists {
		reason = "AlreadyExists"
	} else if reason == apperror.CodeNotFound {
		reason = "NotFound"
	}
	result.Errors = append(result.Errors, ObjectError{Resource: resource, Namespace: namespace, Name: name, Reason: reason})
}

func matches(expression ast.Expression, values map[string]any, result *Result, resource, namespace, name string) bool {
	truth, err := eval.Evaluate(expression, values)
	if err != nil {
		addError(result, resource, namespace, name, err)
		return false
	}
	return truth == eval.True
}
