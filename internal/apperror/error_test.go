package apperror

import (
	"errors"
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type locatedError struct{}

func (locatedError) Error() string        { return "E_PARSE: invalid token" }
func (locatedError) CodeValue() string    { return CodeParse }
func (locatedError) Position() (int, int) { return 2, 7 }

func TestFromPreservesCodeAndPosition(t *testing.T) {
	converted := From(locatedError{})
	if converted.Code != CodeParse || converted.Line != 2 || converted.Column != 7 {
		t.Fatalf("converted error: %#v", converted)
	}
	if converted.Message != "invalid token" {
		t.Fatalf("message: %q", converted.Message)
	}
}

func TestFromPreservesWrappedPosition(t *testing.T) {
	converted := From(fmt.Errorf("parse wrapper: %w", locatedError{}))
	if converted.Code != CodeParse || converted.Line != 2 || converted.Column != 7 {
		t.Fatalf("converted error: %#v", converted)
	}
}

func TestFromSanitizesKubernetesError(t *testing.T) {
	err := apierrors.NewAlreadyExists(schema.GroupResource{Group: "apps", Resource: "deployments"}, "secret-name")
	converted := From(fmt.Errorf("create deployment: %w", err))
	if converted.Code != CodeAlreadyExists {
		t.Fatalf("code: %q", converted.Code)
	}
	if converted.Message != "Kubernetes resource already exists" {
		t.Fatalf("message: %q", converted.Message)
	}
	if converted.Message == err.Error() || converted.Message == "secret-name" {
		t.Fatal("Kubernetes details leaked into public message")
	}
}

func TestExitCode(t *testing.T) {
	if got := ExitCode(New(CodeSemantic, "bad table")); got != 2 {
		t.Fatalf("semantic exit code: %d", got)
	}
	if got := ExitCode(New(CodeKubernetes, "request failed")); got != 1 {
		t.Fatalf("Kubernetes exit code: %d", got)
	}
	if got := ExitCode(nil); got != 0 {
		t.Fatalf("nil exit code: %d", got)
	}
}

func TestFromKeepsCause(t *testing.T) {
	cause := errors.New("cause")
	converted := Wrap(CodeOutput, "write failed", cause)
	if !errors.Is(converted, cause) {
		t.Fatal("structured error did not retain cause")
	}
}
