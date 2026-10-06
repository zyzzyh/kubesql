package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/zyzzyh/kubesql/internal/apperror"
)

func TestWriteErrorUsesStableParserShape(t *testing.T) {
	var output bytes.Buffer
	err := &apperror.Error{Code: apperror.CodeParse, Line: 1, Column: 14, Message: `expected column name, got "FROM"`}
	if writeErr := WriteError(&output, err); writeErr != nil {
		t.Fatal(writeErr)
	}
	var response ErrorResponse
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != apperror.CodeParse || response.Line != 1 || response.Column != 14 || response.Message == "" {
		t.Fatalf("response: %#v", response)
	}
}

func TestWriteErrorDoesNotSerializeCause(t *testing.T) {
	var output bytes.Buffer
	err := apperror.Wrap(apperror.CodeKubernetes, "Kubernetes operation failed", errors.New("private server response"))
	if writeErr := WriteError(&output, err); writeErr != nil {
		t.Fatal(writeErr)
	}
	if bytes.Contains(output.Bytes(), []byte("private server response")) {
		t.Fatal("cause leaked into output")
	}
}
