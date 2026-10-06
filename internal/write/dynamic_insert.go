package write

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zyzzyh/kubesql/internal/ast"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (e *Executor) insertDynamic(ctx context.Context, statement *ast.InsertStatement) (Result, error) {
	if len(statement.Columns) != 1 || strings.ToLower(statement.Columns[0]) != "manifest" {
		return Result{}, fmt.Errorf("E_SEMANTIC: INSERT requires exactly one manifest column")
	}
	if len(statement.Values) != 1 {
		return Result{}, fmt.Errorf("E_SEMANTIC: INSERT requires exactly one VALUES tuple")
	}
	literal, ok := statement.Values[0].(*ast.Literal)
	if !ok || literal.Kind != "string" {
		return Result{}, fmt.Errorf("E_SEMANTIC: manifest must be a SQL string")
	}
	text, ok := literal.Value.(string)
	if !ok {
		return Result{}, fmt.Errorf("E_SEMANTIC: manifest must be a SQL string")
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(text), &object); err != nil || object == nil {
		return Result{}, fmt.Errorf("E_SEMANTIC: invalid manifest JSON")
	}
	if err := validateDynamicManifest(object); err != nil {
		return Result{}, err
	}

	exact := statement.TableQuoted && strings.Contains(statement.Table, "/")
	resource, resolveErr := e.resolver.Resolve(statement.Table, exact)
	if resolveErr != nil && resource.GVR.Resource == "" {
		return Result{}, resolveErr
	}
	if !resource.Verbs["create"] {
		return Result{}, fmt.Errorf("E_UNSUPPORTED_VERB: resource %q does not support INSERT/create", statement.Table)
	}
	apiVersion, _ := object["apiVersion"].(string)
	if apiVersion != resource.GVR.GroupVersion().String() {
		return Result{}, fmt.Errorf("E_SEMANTIC: manifest apiVersion %q does not match %q", apiVersion, resource.GVR.GroupVersion().String())
	}
	kind, _ := object["kind"].(string)
	if resource.Kind != "" && kind != resource.Kind {
		return Result{}, fmt.Errorf("E_SEMANTIC: manifest kind %q does not match %q", kind, resource.Kind)
	}
	metadata, ok := object["metadata"].(map[string]any)
	if !ok {
		return Result{}, fmt.Errorf("E_SEMANTIC: manifest metadata must be an object")
	}
	name, ok := metadata["name"].(string)
	if !ok || name == "" {
		return Result{}, fmt.Errorf("E_SEMANTIC: manifest metadata.name must be a non-empty string")
	}
	if err := prepareDynamicNamespace(metadata, resource.Namespaced, e.namespace); err != nil {
		return Result{}, err
	}

	created := &unstructured.Unstructured{Object: object}
	var err error
	if resource.Namespaced {
		created, err = e.dynamic.Resource(resource.GVR).Namespace(e.namespace).Create(ctx, created, metav1.CreateOptions{})
	} else {
		created, err = e.dynamic.Resource(resource.GVR).Create(ctx, created, metav1.CreateOptions{})
	}
	result := Result{Errors: make([]ObjectError, 0)}
	if err != nil {
		addError(&result, resource.Name, e.namespace, name, err)
		result.FailedRows = 1
		return result, nil
	}
	_ = created
	result.AffectedRows = 1
	return result, nil
}

func validateDynamicManifest(object map[string]any) error {
	for _, field := range []string{"apiVersion", "kind"} {
		value, ok := object[field].(string)
		if !ok || value == "" {
			return fmt.Errorf("E_SEMANTIC: manifest requires %s", field)
		}
	}
	if _, ok := object["status"]; ok {
		return fmt.Errorf("E_SEMANTIC: status is server-managed and cannot be inserted")
	}
	metadata, ok := object["metadata"].(map[string]any)
	if !ok {
		return fmt.Errorf("E_SEMANTIC: manifest metadata must be an object")
	}
	for _, field := range []string{"uid", "resourceVersion", "managedFields", "creationTimestamp", "generation", "selfLink", "deletionTimestamp", "deletionGracePeriodSeconds"} {
		if _, ok := metadata[field]; ok {
			return fmt.Errorf("E_SEMANTIC: metadata.%s is server-managed and cannot be inserted", field)
		}
	}
	return nil
}

func prepareDynamicNamespace(metadata map[string]any, namespaced bool, namespace string) error {
	value, exists := metadata["namespace"]
	manifestNamespace, _ := value.(string)
	if namespaced {
		if namespace == "" {
			return fmt.Errorf("E_NAMESPACE_REQUIRED: namespaced INSERT requires a namespace")
		}
		if exists && manifestNamespace != "" && manifestNamespace != namespace {
			return fmt.Errorf("E_SEMANTIC: manifest namespace %q differs from CLI namespace %q", manifestNamespace, namespace)
		}
		metadata["namespace"] = namespace
		return nil
	}
	if exists && manifestNamespace != "" {
		return fmt.Errorf("E_SEMANTIC: cluster-scoped resource cannot have namespace %q", manifestNamespace)
	}
	return nil
}
