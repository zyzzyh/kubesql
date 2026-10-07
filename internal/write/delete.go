package write

import (
	"context"
	"fmt"
	"strings"

	"github.com/zyzzyh/kubesql/internal/ast"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (e *Executor) delete(ctx context.Context, statement *ast.DeleteStatement) (Result, error) {
	// delete lists fixed typed resources, evaluates WHERE locally, and deletes
	// each matching object while accumulating per-object failures.
	if statement.Where == nil {
		return Result{}, fmt.Errorf("E_WHERE_REQUIRED: DELETE requires WHERE")
	}
	if err := validateDelete(statement); err != nil {
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
			values := map[string]any{"name": item.Name, "namespace": item.Namespace, "replicas": replicas}
			if !matches(statement.Where, values, &result, "deployments", item.Namespace, item.Name) {
				continue
			}
			if err := e.client.AppsV1().Deployments(item.Namespace).Delete(ctx, item.Name, metav1.DeleteOptions{}); err != nil {
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
			if !matches(statement.Where, map[string]any{"name": item.Name}, &result, "namespaces", "", item.Name) {
				continue
			}
			if err := e.client.CoreV1().Namespaces().Delete(ctx, item.Name, metav1.DeleteOptions{}); err != nil {
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
			values := map[string]any{"name": item.Name, "namespace": item.Namespace, "default_backend_service": service}
			if !matches(statement.Where, values, &result, "ingresses", item.Namespace, item.Name) {
				continue
			}
			if err := e.client.NetworkingV1().Ingresses(item.Namespace).Delete(ctx, item.Name, metav1.DeleteOptions{}); err != nil {
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
