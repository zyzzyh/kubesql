package parser

import (
	"fmt"

	"github.com/zyzzyh/kubesql/internal/token"
)

// Error describes a syntax error at the token where parsing stopped.
type Error struct {
	Code    string `json:"code"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s at %d:%d: %s", e.Code, e.Line, e.Column, e.Message)
}

func expected(want string, got token.Token) error {
	found := got.Literal
	if got.Type == token.EOF {
		found = "end of input"
	}
	return &Error{
		Code:    "E_PARSE",
		Line:    got.Line,
		Column:  got.Column,
		Message: fmt.Sprintf("expected %s, got %q", want, found),
	}
}
