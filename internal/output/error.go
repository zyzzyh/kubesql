package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/zyzzyh/kubesql/internal/apperror"
)

// ErrorResponse is the public, sanitized JSON error shape.
type ErrorResponse struct {
	Code      string `json:"code"`
	Line      int    `json:"line,omitempty"`
	Column    int    `json:"column,omitempty"`
	Message   string `json:"message"`
	Resource  string `json:"resource,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

// WriteError writes a structured error to stderr without serializing its cause.
func WriteError(w io.Writer, err error) error {
	if err == nil {
		return nil
	}
	applicationError := apperror.From(err)
	response := ErrorResponse{
		Code:      applicationError.Code,
		Line:      applicationError.Line,
		Column:    applicationError.Column,
		Message:   applicationError.Message,
		Resource:  applicationError.Resource,
		Namespace: applicationError.Namespace,
		Name:      applicationError.Name,
	}
	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		return fmt.Errorf("%s: %w", apperror.CodeOutput, encodeErr)
	}
	return nil
}
