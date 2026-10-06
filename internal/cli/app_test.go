package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/zyzzyh/kubesql/internal/apperror"
	"github.com/zyzzyh/kubesql/internal/output"
)

func TestRunParseErrorReturnsStructuredExitCode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), nil, strings.NewReader("SELECT name, FROM deployments"), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code: %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
	var response output.ErrorResponse
	if err := json.Unmarshal(stderr.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != apperror.CodeParse || response.Line != 1 || response.Column != 14 {
		t.Fatalf("response: %#v", response)
	}
}

func TestRunInvalidFlagDoesNotMixPlainTextWithJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"-unknown"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code: %d", code)
	}
	var response output.ErrorResponse
	if err := json.Unmarshal(stderr.Bytes(), &response); err != nil {
		t.Fatalf("stderr is not one JSON document: %v; output=%q", err, stderr.String())
	}
	if response.Code != apperror.CodeUsage {
		t.Fatalf("response: %#v", response)
	}
}
