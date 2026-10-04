package write

import (
	"context"
	"fmt"
	"strings"

	"github.com/zyzzyh/kubesql/internal/ast"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func (e *Executor) update(ctx context.Context, statement *ast.UpdateStatement) (Result, error) {
	if statement.Where == nil {
		return Result{}, fmt.Errorf("E_WHERE_REQUIRED: UPDATE requires WHERE")
	}
	if err := validateUpdate(statement); err != nil {
		return Result{}, err
	}
	result := Result{Errors: make([]ObjectError, 0)}
	switch strings.ToLower(statement.Table) {
	case "deployments":
		items, err := e.client.AppsV1().Deployments(e.namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return Result{}, fmt.Errorf("list deployments: %w", err)
		}
		for _, item := range items.Items {
			var replicas any
			if item.Spec.Replicas != nil {
				replicas = *item.Spec.Replicas
			}
			values := map[string]any{"name": item.Name, "namespace": item.Namespace, "replicas": replicas, "labels": item.Labels, "annotations": item.Annotations}
			if !matches(statement.Where, values, &result, "deployments", item.Namespace, item.Name) {
				continue
			}
			object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&item)
			if err != nil {
				addError(&result, "deployments", item.Namespace, item.Name, err)
				continue
			}
			data, err := buildPatch(item.ResourceVersion, statement.Assignments, "deployments", object)
			if err != nil {
				addError(&result, "deployments", item.Namespace, item.Name, err)
				continue
			}
			if _, err := e.client.AppsV1().Deployments(item.Namespace).Patch(ctx, item.Name, types.JSONPatchType, data, metav1.PatchOptions{}); err != nil {
				addError(&result, "deployments", item.Namespace, item.Name, err)
				continue
			}
			result.AffectedRows++
		}
	case "namespaces":
		items, err := e.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return Result{}, fmt.Errorf("list namespaces: %w", err)
		}
		for _, item := range items.Items {
			values := map[string]any{"name": item.Name, "labels": item.Labels, "annotations": item.Annotations}
			if !matches(statement.Where, values, &result, "namespaces", "", item.Name) {
				continue
			}
			object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&item)
			if err != nil {
				addError(&result, "namespaces", "", item.Name, err)
				continue
			}
			data, err := buildPatch(item.ResourceVersion, statement.Assignments, "namespaces", object)
			if err != nil {
				addError(&result, "namespaces", "", item.Name, err)
				continue
			}
			if _, err := e.client.CoreV1().Namespaces().Patch(ctx, item.Name, types.JSONPatchType, data, metav1.PatchOptions{}); err != nil {
				addError(&result, "namespaces", "", item.Name, err)
				continue
			}
			result.AffectedRows++
		}
	case "ingresses":
		items, err := e.client.NetworkingV1().Ingresses(e.namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return Result{}, fmt.Errorf("list ingresses: %w", err)
		}
		for _, item := range items.Items {
			service := any(nil)
			if item.Spec.DefaultBackend != nil && item.Spec.DefaultBackend.Service != nil {
				service = item.Spec.DefaultBackend.Service.Name
			}
			values := map[string]any{"name": item.Name, "namespace": item.Namespace, "default_backend_service": service, "labels": item.Labels, "annotations": item.Annotations}
			if !matches(statement.Where, values, &result, "ingresses", item.Namespace, item.Name) {
				continue
			}
			object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&item)
			if err != nil {
				addError(&result, "ingresses", item.Namespace, item.Name, err)
				continue
			}
			data, err := buildPatch(item.ResourceVersion, statement.Assignments, "ingresses", object)
			if err != nil {
				addError(&result, "ingresses", item.Namespace, item.Name, err)
				continue
			}
			if _, err := e.client.NetworkingV1().Ingresses(item.Namespace).Patch(ctx, item.Name, types.JSONPatchType, data, metav1.PatchOptions{}); err != nil {
				addError(&result, "ingresses", item.Namespace, item.Name, err)
				continue
			}
			result.AffectedRows++
		}
	default:
		return Result{}, fmt.Errorf("E_SEMANTIC: unknown table %q", statement.Table)
	}
	result.FailedRows = len(result.Errors)
	return result, nil
}
