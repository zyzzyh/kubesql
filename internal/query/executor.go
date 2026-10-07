// Package query executes the basic SELECT statements against Kubernetes.
package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/zyzzyh/kubesql/internal/ast"
	"github.com/zyzzyh/kubesql/internal/catalog"
	"github.com/zyzzyh/kubesql/internal/discovery"
	"github.com/zyzzyh/kubesql/internal/eval"
	metricsclient "github.com/zyzzyh/kubesql/internal/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// Executor reads supported Kubernetes resources and projects public columns.
type Executor struct {
	client        kubernetes.Interface
	dynamicClient dynamic.Interface
	resolver      *discovery.Resolver
	metricsClient metricsclient.Client
	namespace     string
	allNamespaces bool
}

// NewExecutor creates a query executor with an already configured client.
func NewExecutor(client kubernetes.Interface, namespace string, allNamespaces bool) *Executor {
	return &Executor{client: client, namespace: namespace, allNamespaces: allNamespaces}
}

// NewDynamicExecutor adds runtime-discovered resources while preserving the
// typed path used by the original built-in tables.
func NewDynamicExecutor(client kubernetes.Interface, dynamicClient dynamic.Interface, resolver *discovery.Resolver, namespace string, allNamespaces bool, metricClients ...metricsclient.Client) *Executor {
	var metricClient metricsclient.Client
	if len(metricClients) > 0 {
		metricClient = metricClients[0]
	}
	return &Executor{client: client, dynamicClient: dynamicClient, resolver: resolver, metricsClient: metricClient, namespace: namespace, allNamespaces: allNamespaces}
}

// Execute validates and runs one SELECT statement.
func (e *Executor) Execute(ctx context.Context, statement *ast.SelectStatement) ([]map[string]any, error) {
	// Execute resolves semantics before accessing Kubernetes, then selects the
	// typed adapter or Discovery-backed dynamic adapter.
	table, columns, err := catalog.Resolve(statement)
	if err != nil {
		if e.dynamicClient == nil || e.resolver == nil || catalog.IsKnownTable(statement) {
			return nil, err
		}
		table, columns, err = catalog.ResolveDynamic(statement)
		if err != nil {
			return nil, err
		}
		exact := statement.TableQuoted && strings.Contains(statement.Table, "/")
		resource, resolveErr := e.resolver.Resolve(statement.Table, exact)
		if resolveErr != nil && resource.GVR.Resource == "" {
			return nil, resolveErr
		}
		if !resource.Verbs["list"] {
			return nil, fmt.Errorf("E_UNSUPPORTED_VERB: resource %q does not support SELECT/list", statement.Table)
		}
		table.Namespaced = resource.Namespaced
		if table.Namespaced && e.allNamespaces {
			return e.executeDynamic(ctx, resource, columns, "", statement.Where)
		}
		return e.executeDynamic(ctx, resource, columns, e.namespace, statement.Where)
	}
	if table.Metrics {
		return e.executeMetrics(ctx, table.Name, columns, statement.Where, table.Namespaced && e.allNamespaces)
	}
	if table.Namespaced && e.allNamespaces {
		return e.executeAllNamespaces(ctx, table.Name, columns, statement.Where)
	}
	return e.executeNamespace(ctx, table.Name, columns, e.namespace, statement.Where)
}

func (e *Executor) executeMetrics(ctx context.Context, tableName string, columns []catalog.Projection, where ast.Expression, allNamespaces bool) ([]map[string]any, error) {
	if e.metricsClient == nil {
		return nil, fmt.Errorf("E_METRICS_UNAVAILABLE: Metrics API client is not configured")
	}
	rows := make([]resourceRow, 0)
	switch tableName {
	case "pod_metrics":
		namespace := e.namespace
		if allNamespaces {
			namespace = ""
		}
		items, err := e.metricsClient.ListPodMetrics(ctx, namespace)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			rows = append(rows, resourceRow{metric: map[string]any{
				"name": item.Name, "namespace": item.Namespace,
				"cpu_millicores": item.CPUMillicores, "memory_bytes": item.MemoryBytes,
			}})
		}
	case "node_metrics":
		items, err := e.metricsClient.ListNodeMetrics(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			rows = append(rows, resourceRow{metric: map[string]any{
				"name": item.Name, "cpu_millicores": item.CPUMillicores, "memory_bytes": item.MemoryBytes,
			}})
		}
	default:
		return nil, fmt.Errorf("E_SEMANTIC: unknown Metrics table %q", tableName)
	}
	return filterAndProject(resourceList{rows: rows}, columns, where)
}

func (e *Executor) executeDynamic(ctx context.Context, resource discovery.Resource, columns []catalog.Projection, namespace string, where ast.Expression) ([]map[string]any, error) {
	var items *unstructured.UnstructuredList
	var err error
	if resource.Namespaced {
		items, err = e.dynamicClient.Resource(resource.GVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		items, err = e.dynamicClient.Resource(resource.GVR).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", resource.GVR.Resource, err)
	}
	rows := make([]resourceRow, 0, len(items.Items))
	for _, item := range items.Items {
		rows = append(rows, resourceRow{dynamic: item.Object})
	}
	return filterAndProject(resourceList{rows: rows}, columns, where)
}

func (e *Executor) executeNamespace(ctx context.Context, tableName string, columns []catalog.Projection, namespace string, where ast.Expression) ([]map[string]any, error) {
	resources, err := e.list(ctx, tableName, namespace)
	if err != nil {
		return nil, err
	}
	return filterAndProject(resources, columns, where)
}

func (e *Executor) executeAllNamespaces(ctx context.Context, tableName string, columns []catalog.Projection, where ast.Expression) ([]map[string]any, error) {
	return e.executeNamespace(ctx, tableName, columns, "", where)
}

func filterAndProject(resources resourceList, columns []catalog.Projection, where ast.Expression) ([]map[string]any, error) {
	// filterAndProject applies SQL three-valued filtering and builds aliased rows.
	result := make([]map[string]any, 0, len(resources.rows))
	for _, resource := range resources.rows {
		values := resource.values()
		if where != nil {
			matched, err := eval.Evaluate(where, values)
			if err != nil {
				return nil, err
			}
			if matched != eval.True {
				continue
			}
		}
		row := make(map[string]any, len(columns))
		for _, column := range columns {
			if column.Source == "*" {
				for key, value := range values {
					row[key] = value
				}
				continue
			}
			row[column.Output] = values[column.Source]
		}
		result = append(result, row)
	}
	return result, nil
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
