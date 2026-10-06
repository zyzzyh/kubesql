// Package write executes UPDATE and DELETE statements against Kubernetes.
package write

import (
	"context"
	"fmt"

	"github.com/zyzzyh/kubesql/internal/ast"
	"github.com/zyzzyh/kubesql/internal/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// Executor performs medium-stage UPDATE and DELETE operations.
type Executor struct {
	client    kubernetes.Interface
	dynamic   dynamic.Interface
	resolver  *discovery.Resolver
	namespace string
}

// NewExecutor creates a write executor for one explicit namespace.
func NewExecutor(client kubernetes.Interface, namespace string) *Executor {
	return &Executor{client: client, namespace: namespace}
}

// NewDynamicExecutor keeps typed writes for known tables and enables dynamic
// DELETE for resources resolved through Kubernetes Discovery.
func NewDynamicExecutor(client kubernetes.Interface, dynamicClient dynamic.Interface, resolver *discovery.Resolver, namespace string) *Executor {
	return &Executor{client: client, dynamic: dynamicClient, resolver: resolver, namespace: namespace}
}

// Execute validates and executes one UPDATE or DELETE statement.
func (e *Executor) Execute(ctx context.Context, statement ast.Statement) (Result, error) {
	switch statement := statement.(type) {
	case *ast.UpdateStatement:
		if e.dynamic != nil && e.resolver != nil && !knownWriteTable(statement.Table) {
			return e.updateDynamic(ctx, statement)
		}
		return e.update(ctx, statement)
	case *ast.DeleteStatement:
		if e.dynamic != nil && e.resolver != nil && !knownWriteTable(statement.Table) {
			return e.deleteDynamic(ctx, statement)
		}
		return e.delete(ctx, statement)
	case *ast.InsertStatement:
		if e.dynamic != nil && e.resolver != nil && !knownWriteTable(statement.Table) {
			return e.insertDynamic(ctx, statement)
		}
		return e.insert(ctx, statement)
	default:
		return Result{}, fmt.Errorf("E_SEMANTIC: unsupported write statement %T", statement)
	}
}
