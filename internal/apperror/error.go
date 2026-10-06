// Package apperror defines errors exposed by the KubeSQL application.
package apperror

import (
	"errors"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	CodeParse             = "E_PARSE"
	CodeUsage             = "E_USAGE"
	CodeSemantic          = "E_SEMANTIC"
	CodeEval              = "E_EVAL"
	CodeWhereRequired     = "E_WHERE_REQUIRED"
	CodeNamespaceRequired = "E_NAMESPACE_REQUIRED"
	CodeKubernetes        = "E_KUBE"
	CodeAlreadyExists     = "E_ALREADY_EXISTS"
	CodeNotFound          = "E_NOT_FOUND"
	CodeForbidden         = "E_FORBIDDEN"
	CodeConflict          = "E_CONFLICT"
	CodeOutput            = "E_OUTPUT"
	CodeInput             = "E_INPUT"
	CodeDiscovery         = "E_DISCOVERY"
	CodeUnsupportedVerb   = "E_UNSUPPORTED_VERB"
)

// Error is the stable application error returned to CLI callers.
// Cause is kept for errors.Is/errors.As and is never serialized by output.
type Error struct {
	Code      string
	Message   string
	Line      int
	Column    int
	Resource  string
	Namespace string
	Name      string
	Cause     error
}

func (e *Error) Error() string {
	if e.Line > 0 && e.Column > 0 {
		return fmt.Sprintf("%s at %d:%d: %s", e.Code, e.Line, e.Column, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }

// New creates a structured application error.
func New(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Wrap creates a structured application error while retaining its cause.
func Wrap(code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

type positioned interface {
	CodeValue() string
	Position() (int, int)
}

// From converts an arbitrary error into a stable application error.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var structured *Error
	if errors.As(err, &structured) {
		return structured
	}
	if apierrors.IsAlreadyExists(err) {
		return Wrap(CodeAlreadyExists, "Kubernetes resource already exists", err)
	}
	if apierrors.IsNotFound(err) {
		return Wrap(CodeNotFound, "Kubernetes resource was not found", err)
	}
	if apierrors.IsForbidden(err) {
		return Wrap(CodeForbidden, "Kubernetes request was forbidden", err)
	}
	if apierrors.IsConflict(err) {
		return Wrap(CodeConflict, "Kubernetes resource conflict", err)
	}
	var located positioned
	if errors.As(err, &located) {
		line, column := located.Position()
		return &Error{Code: located.CodeValue(), Line: line, Column: column, Message: messageWithoutCode(err.Error()), Cause: err}
	}
	if code, message, ok := prefixedCode(err.Error()); ok {
		return &Error{Code: code, Message: message, Cause: err}
	}
	return Wrap(CodeKubernetes, conciseMessage(err), err)
}

func prefixedCode(message string) (string, string, bool) {
	parts := strings.SplitN(message, ": ", 2)
	if len(parts) == 2 && strings.HasPrefix(parts[0], "E_") {
		return parts[0], parts[1], true
	}
	return "", "", false
}

func messageWithoutCode(message string) string {
	if _, text, ok := prefixedCode(message); ok {
		return text
	}
	return message
}

func conciseMessage(err error) string {
	if _, _, ok := prefixedCode(err.Error()); ok {
		return messageWithoutCode(err.Error())
	}
	return "Kubernetes operation failed"
}

// CodeOf returns the stable code for an error.
func CodeOf(err error) string {
	if err == nil {
		return ""
	}
	return From(err).Code
}

// ReasonOf returns a stable per-object reason for write results.
func ReasonOf(err error) string {
	return CodeOf(err)
}

// ExitCode maps application errors to CLI exit statuses.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	switch CodeOf(err) {
	case CodeParse, CodeUsage, CodeSemantic, CodeEval, CodeWhereRequired, CodeNamespaceRequired:
		return 2
	default:
		return 1
	}
}
