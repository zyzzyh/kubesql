package write

import (
	"context"
	"fmt"
	"strings"

	"github.com/zyzzyh/kubesql/internal/ast"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (e *Executor) deleteDynamic(ctx context.Context, statement *ast.DeleteStatement) (Result, error) {
	// deleteDynamic evaluates WHERE over listed objects and reports per-object
	// failures instead of aborting the whole multi-object operation.
	if statement.Where == nil {
		return Result{}, fmt.Errorf("E_WHERE_REQUIRED: DELETE requires WHERE")
	}
	exact := statement.TableQuoted && strings.Contains(statement.Table, "/")
	resource, resolveErr := e.resolver.Resolve(statement.Table, exact)
	if resolveErr != nil && resource.GVR.Resource == "" {
		return Result{}, resolveErr
	}
	if !resource.Verbs["list"] {
		return Result{}, fmt.Errorf("E_UNSUPPORTED_VERB: resource %q does not support DELETE/list", statement.Table)
	}
	if !resource.Verbs["delete"] {
		return Result{}, fmt.Errorf("E_UNSUPPORTED_VERB: resource %q does not support DELETE/delete", statement.Table)
	}

	var items *unstructured.UnstructuredList
	var err error
	if resource.Namespaced {
		if e.namespace == "" {
			return Result{}, fmt.Errorf("E_NAMESPACE_REQUIRED: namespaced DELETE requires a namespace")
		}
		items, err = e.dynamic.Resource(resource.GVR).Namespace(e.namespace).List(ctx, metav1.ListOptions{})
	} else {
		items, err = e.dynamic.Resource(resource.GVR).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return Result{}, fmt.Errorf("list %s: %w", resource.GVR.Resource, err)
	}

	result := Result{Errors: make([]ObjectError, 0)}
	for _, item := range items.Items {
		values := dynamicValues(item)
		name, _, _ := unstructured.NestedString(item.Object, "metadata", "name")
		namespace, _, _ := unstructured.NestedString(item.Object, "metadata", "namespace")
		if !matches(statement.Where, values, &result, resource.Name, namespace, name) {
			continue
		}
		var deleteErr error
		if resource.Namespaced {
			deleteErr = e.dynamic.Resource(resource.GVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		} else {
			deleteErr = e.dynamic.Resource(resource.GVR).Delete(ctx, name, metav1.DeleteOptions{})
		}
		if deleteErr != nil {
			addError(&result, resource.Name, namespace, name, deleteErr)
			continue
		}
		result.AffectedRows++
	}
	result.FailedRows = len(result.Errors)
	return result, nil
}

func dynamicValues(item unstructured.Unstructured) map[string]any {
	values := map[string]any{}
	name, _, _ := unstructured.NestedString(item.Object, "metadata", "name")
	namespace, _, _ := unstructured.NestedString(item.Object, "metadata", "namespace")
	values["name"] = name
	values["namespace"] = namespace
	flattenDynamicValues(values, item.Object, "")
	return values
}

func flattenDynamicValues(values map[string]any, object map[string]any, prefix string) {
	for key, value := range object {
		path := prefix + "/" + key
		values[path] = value
		if child, ok := value.(map[string]any); ok {
			flattenDynamicValues(values, child, path)
		}
	}
}

func knownWriteTable(table string) bool {
	switch strings.ToLower(table) {
	case "namespaces", "deployments", "ingresses":
		return true
	default:
		return false
	}
}
