// Package query executes the basic SELECT statements against Kubernetes.
package query

import (
	"context"
	"fmt"

	"github.com/zyzzyh/kubesql/internal/ast"
	"github.com/zyzzyh/kubesql/internal/catalog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Executor reads supported Kubernetes resources and projects public columns.
type Executor struct {
	client        kubernetes.Interface
	namespace     string
	allNamespaces bool
}

// NewExecutor creates a query executor with an already configured client.
func NewExecutor(client kubernetes.Interface, namespace string, allNamespaces bool) *Executor {
	return &Executor{client: client, namespace: namespace, allNamespaces: allNamespaces}
}

// Execute validates and runs one SELECT statement.
func (e *Executor) Execute(ctx context.Context, statement *ast.SelectStatement) ([]map[string]any, error) {
	table, columns, err := catalog.Resolve(statement)
	if err != nil {
		return nil, err
	}
	if table.Namespaced && e.allNamespaces {
		return e.executeAllNamespaces(ctx, table.Name, columns)
	}
	return e.executeNamespace(ctx, table.Name, columns, e.namespace)
}

func (e *Executor) executeNamespace(ctx context.Context, tableName string, columns []string, namespace string) ([]map[string]any, error) {
	resources, err := e.list(ctx, tableName, namespace)
	if err != nil {
		return nil, err
	}
	return project(resources, tableName, columns), nil
}

func (e *Executor) executeAllNamespaces(ctx context.Context, tableName string, columns []string) ([]map[string]any, error) {
	return e.executeNamespace(ctx, tableName, columns, "")
}

func (e *Executor) list(ctx context.Context, tableName, namespace string) (resourceList, error) {
	options := metav1.ListOptions{}
	switch tableName {
	case "namespaces":
		items, err := e.client.CoreV1().Namespaces().List(ctx, options)
		if err != nil {
			return resourceList{}, fmt.Errorf("list namespaces: %w", err)
		}
		return fromNamespaces(items), nil
	case "deployments":
		items, err := e.client.AppsV1().Deployments(namespace).List(ctx, options)
		if err != nil {
			return resourceList{}, fmt.Errorf("list deployments: %w", err)
		}
		return fromDeployments(items), nil
	case "ingresses":
		items, err := e.client.NetworkingV1().Ingresses(namespace).List(ctx, options)
		if err != nil {
			return resourceList{}, fmt.Errorf("list ingresses: %w", err)
		}
		return fromIngresses(items), nil
	default:
		return resourceList{}, fmt.Errorf("E_SEMANTIC: unknown table %q", tableName)
	}
}
