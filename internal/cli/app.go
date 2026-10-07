// Package cli coordinates command-line input, execution, and output.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/zyzzyh/kubesql/internal/apperror"
	"github.com/zyzzyh/kubesql/internal/ast"
	"github.com/zyzzyh/kubesql/internal/discovery"
	"github.com/zyzzyh/kubesql/internal/kube"
	"github.com/zyzzyh/kubesql/internal/output"
	"github.com/zyzzyh/kubesql/internal/parser"
	"github.com/zyzzyh/kubesql/internal/query"
	"github.com/zyzzyh/kubesql/internal/write"
)

// Run executes one SQL statement and returns the process exit code.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// Run is the application boundary: parse input, build clients, dispatch the
	// statement, and translate errors into the documented process exit code.
	opts, err := parseOptions(args)
	if err != nil {
		return reportError(stderr, err)
	}

	input, err := io.ReadAll(stdin)
	if err != nil {
		return reportError(stderr, apperror.Wrap(apperror.CodeInput, "read SQL input", err))
	}
	statement, err := parser.New(string(input)).Parse()
	if err != nil {
		return reportError(stderr, err)
	}
	clients, err := kube.NewClients(opts.kubeconfig, opts.contextName)
	if err != nil {
		return reportError(stderr, err)
	}
	if opts.namespace == "" {
		opts.namespace = clients.Namespace
	}

	switch statement := statement.(type) {
	case *ast.SelectStatement:
		resolver := discovery.NewResolver(clients.Discovery)
		rows, executeErr := query.NewDynamicExecutor(clients.Typed, clients.Dynamic, resolver, opts.namespace, opts.allNamespaces, clients.Metrics).Execute(ctx, statement)
		if executeErr != nil {
			return reportError(stderr, executeErr)
		}
		if writeErr := output.WriteRows(stdout, rows); writeErr != nil {
			return reportError(stderr, apperror.Wrap(apperror.CodeOutput, "write query output", writeErr))
		}
		return 0
	case *ast.UpdateStatement, *ast.DeleteStatement, *ast.InsertStatement:
		resolver := discovery.NewResolver(clients.Discovery)
		result, executeErr := write.NewDynamicExecutor(clients.Typed, clients.Dynamic, resolver, opts.namespace).Execute(ctx, statement)
		if executeErr != nil {
			return reportError(stderr, executeErr)
		}
		if writeErr := output.WriteResult(stdout, result); writeErr != nil {
			return reportError(stderr, apperror.Wrap(apperror.CodeOutput, "write operation output", writeErr))
		}
		if result.HasFailures() {
			return 1
		}
		return 0
	default:
		return reportError(stderr, apperror.New(apperror.CodeSemantic, fmt.Sprintf("unsupported statement type %T", statement)))
	}
}

func reportError(stderr io.Writer, err error) int {
	// reportError keeps stderr as one structured JSON response and returns its
	// stable application exit status.
	if outputErr := output.WriteError(stderr, err); outputErr != nil {
		return apperror.ExitCode(outputErr)
	}
	return apperror.ExitCode(err)
}
