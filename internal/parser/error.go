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

// CodeValue and Position let shared error output preserve parser locations
// without making parser depend on the application error package.
func (e *Error) CodeValue() string { return e.Code }

func (e *Error) Position() (int, int) { return e.Line, e.Column }

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
