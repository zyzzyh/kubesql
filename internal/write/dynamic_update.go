package write

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zyzzyh/kubesql/internal/ast"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func (e *Executor) updateDynamic(ctx context.Context, statement *ast.UpdateStatement) (Result, error) {
	if statement.Where == nil {
		return Result{}, fmt.Errorf("E_WHERE_REQUIRED: UPDATE requires WHERE")
	}
	if len(statement.Assignments) == 0 {
		return Result{}, fmt.Errorf("E_SEMANTIC: UPDATE requires at least one assignment")
	}
	for _, assignment := range statement.Assignments {
		if !assignment.ColumnQuoted || !strings.HasPrefix(assignment.Column, "/") {
			return Result{}, fmt.Errorf("E_SEMANTIC: dynamic UPDATE columns must be quoted JSON Pointers")
		}
		if err := validateDynamicPath(assignment.Column); err != nil {
			return Result{}, err
		}
		if _, err := literalValue(assignment.Value); err != nil {
			return Result{}, err
		}
	}

	exact := statement.TableQuoted && strings.Contains(statement.Table, "/")
	resource, resolveErr := e.resolver.Resolve(statement.Table, exact)
	if resolveErr != nil && resource.GVR.Resource == "" {
		return Result{}, resolveErr
	}
	if !resource.Verbs["list"] {
		return Result{}, fmt.Errorf("E_UNSUPPORTED_VERB: resource %q does not support UPDATE/list", statement.Table)
	}
	if !resource.Verbs["patch"] {
		return Result{}, fmt.Errorf("E_UNSUPPORTED_VERB: resource %q does not support UPDATE/patch", statement.Table)
	}

	var items *unstructured.UnstructuredList
	var err error
	if resource.Namespaced {
		if e.namespace == "" {
			return Result{}, fmt.Errorf("E_NAMESPACE_REQUIRED: namespaced UPDATE requires a namespace")
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
		name, _, _ := unstructured.NestedString(item.Object, "metadata", "name")
		namespace, _, _ := unstructured.NestedString(item.Object, "metadata", "namespace")
		if !matches(statement.Where, dynamicValues(item), &result, resource.Name, namespace, name) {
			continue
		}
		patch, patchErr := buildDynamicPatch(item, statement.Assignments)
		if patchErr != nil {
			addError(&result, resource.Name, namespace, name, patchErr)
			continue
		}
		var patchResult *unstructured.Unstructured
		if resource.Namespaced {
			patchResult, patchErr = e.dynamic.Resource(resource.GVR).Namespace(namespace).Patch(ctx, name, types.JSONPatchType, patch, metav1.PatchOptions{})
		} else {
			patchResult, patchErr = e.dynamic.Resource(resource.GVR).Patch(ctx, name, types.JSONPatchType, patch, metav1.PatchOptions{})
		}
		_ = patchResult
		if patchErr != nil {
			addError(&result, resource.Name, namespace, name, patchErr)
			continue
		}
		result.AffectedRows++
	}
	result.FailedRows = len(result.Errors)
	return result, nil
}

func validateDynamicPath(path string) error {
	for _, forbidden := range []string{"/metadata/name", "/metadata/namespace", "/metadata/uid", "/metadata/resourceVersion", "/status"} {
		if path == forbidden || strings.HasPrefix(path, forbidden+"/") {
			return fmt.Errorf("E_SEMANTIC: JSON Pointer %q targets a server-managed field", path)
		}
	}
	return nil
}

func buildDynamicPatch(item unstructured.Unstructured, assignments []ast.Assignment) ([]byte, error) {
	operations := []map[string]any{{"op": "test", "path": "/metadata/resourceVersion", "value": item.GetResourceVersion()}}
	for _, assignment := range assignments {
		value, err := literalValue(assignment.Value)
		if err != nil {
			return nil, err
		}
		op := "add"
		if pointerExists(item.Object, assignment.Column) {
			op = "replace"
		}
		operations = append(operations, map[string]any{"op": op, "path": assignment.Column, "value": value})
	}
	return json.Marshal(operations)
}

func pointerExists(object map[string]any, pointer string) bool {
	parts := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	var current any = object
	for _, part := range parts {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		values, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = values[part]
		if !ok {
			return false
		}
	}
	return true
}
