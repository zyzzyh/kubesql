// Package write executes UPDATE and DELETE statements against Kubernetes.
package write

import (
	"context"
	"fmt"

	"github.com/zyzzyh/kubesql/internal/ast"
	"k8s.io/client-go/kubernetes"
)

// Executor performs medium-stage UPDATE and DELETE operations.
type Executor struct {
	client    kubernetes.Interface
	namespace string
}

// NewExecutor creates a write executor for one explicit namespace.
func NewExecutor(client kubernetes.Interface, namespace string) *Executor {
	return &Executor{client: client, namespace: namespace}
}

// Execute validates and executes one UPDATE or DELETE statement.
func (e *Executor) Execute(ctx context.Context, statement ast.Statement) (Result, error) {
	switch statement := statement.(type) {
	case *ast.UpdateStatement:
		return e.update(ctx, statement)
	case *ast.DeleteStatement:
		return e.delete(ctx, statement)
	case *ast.InsertStatement:
		return e.insert(ctx, statement)
	default:
		return Result{}, fmt.Errorf("E_SEMANTIC: unsupported write statement %T", statement)
	}
}
