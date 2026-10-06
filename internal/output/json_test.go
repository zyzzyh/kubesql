package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/zyzzyh/kubesql/internal/write"
)

func TestWriteRowsNormalizesNilToEmptyArray(t *testing.T) {
	var output bytes.Buffer
	if err := WriteRows(&output, nil); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(output.String()); got != "[]" {
		t.Fatalf("output: %q", got)
	}
}

func TestWriteResultPreservesWriteShape(t *testing.T) {
	var output bytes.Buffer
	result := write.Result{AffectedRows: 1, FailedRows: 1, Errors: []write.ObjectError{{Resource: "deployments", Name: "api", Reason: "AlreadyExists"}}}
	if err := WriteResult(&output, result); err != nil {
		t.Fatal(err)
	}
	want := `{"affected_rows":1,"failed_rows":1,"errors":[{"resource":"deployments","namespace":"","name":"api","reason":"AlreadyExists"}]}`
	if got := strings.TrimSpace(output.String()); got != want {
		t.Fatalf("output: %s", got)
	}
}
