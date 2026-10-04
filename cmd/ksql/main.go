package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/zyzzyh/kubesql/internal/ast"
	"github.com/zyzzyh/kubesql/internal/kube"
	"github.com/zyzzyh/kubesql/internal/parser"
	"github.com/zyzzyh/kubesql/internal/query"
)

type options struct {
	kubeconfig    string
	contextName   string
	namespace     string
	allNamespaces bool
	output        string
}

func main() {
	var opts options
	flag.StringVar(&opts.kubeconfig, "kubeconfig", "", "path to kubeconfig")
	flag.StringVar(&opts.contextName, "context", "", "kubeconfig context")
	flag.StringVar(&opts.namespace, "namespace", "", "Kubernetes namespace")
	flag.BoolVar(&opts.allNamespaces, "all-namespaces", false, "query all namespaces")
	flag.StringVar(&opts.output, "output", "json", "output format")
	flag.Parse()

	if opts.output != "json" {
		fail(fmt.Errorf("unsupported output format %q", opts.output))
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fail(fmt.Errorf("read SQL: %w", err))
	}
	statement, err := parser.New(string(input)).Parse()
	if err != nil {
		fail(err)
	}
	selectStatement, ok := statement.(*ast.SelectStatement)
	if !ok {
		fail(fmt.Errorf("unsupported statement type %T", statement))
	}

	client, defaultNamespace, err := kube.NewClient(opts.kubeconfig, opts.contextName)
	if err != nil {
		fail(err)
	}
	if opts.namespace == "" {
		opts.namespace = defaultNamespace
	}
	rows, err := query.NewExecutor(client, opts.namespace, opts.allNamespaces).Execute(context.Background(), selectStatement)
	if err != nil {
		fail(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(rows); err != nil {
		fail(fmt.Errorf("write JSON: %w", err))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
