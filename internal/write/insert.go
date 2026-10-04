package write

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zyzzyh/kubesql/internal/ast"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (e *Executor) insert(ctx context.Context, statement *ast.InsertStatement) (Result, error) {
	resource, err := decodeManifest(statement, e.namespace)
	if err != nil {
		return Result{}, err
	}
	data, err := json.Marshal(resource.object)
	if err != nil {
		return Result{}, fmt.Errorf("E_SEMANTIC: encode manifest: %w", err)
	}
	result := Result{Errors: make([]ObjectError, 0)}
	switch resource.table {
	case "namespaces":
		var object corev1.Namespace
		if err := json.Unmarshal(data, &object); err != nil {
			return Result{}, fmt.Errorf("E_SEMANTIC: invalid Namespace manifest: %w", err)
		}
		if _, err := e.client.CoreV1().Namespaces().Create(ctx, &object, metav1.CreateOptions{}); err != nil {
			addError(&result, resource.table, "", resource.name, err)
			result.FailedRows = 1
			return result, nil
		}
	case "deployments":
		var object appsv1.Deployment
		if err := json.Unmarshal(data, &object); err != nil {
			return Result{}, fmt.Errorf("E_SEMANTIC: invalid Deployment manifest: %w", err)
		}
		if _, err := e.client.AppsV1().Deployments(resource.namespace).Create(ctx, &object, metav1.CreateOptions{}); err != nil {
			addError(&result, resource.table, resource.namespace, resource.name, err)
			result.FailedRows = 1
			return result, nil
		}
	case "ingresses":
		var object networkingv1.Ingress
		if err := json.Unmarshal(data, &object); err != nil {
			return Result{}, fmt.Errorf("E_SEMANTIC: invalid Ingress manifest: %w", err)
		}
		if _, err := e.client.NetworkingV1().Ingresses(resource.namespace).Create(ctx, &object, metav1.CreateOptions{}); err != nil {
			addError(&result, resource.table, resource.namespace, resource.name, err)
			result.FailedRows = 1
			return result, nil
		}
	}
	result.AffectedRows = 1
	return result, nil
}
