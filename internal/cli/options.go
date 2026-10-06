package cli

import (
	"flag"
	"io"

	"github.com/zyzzyh/kubesql/internal/apperror"
)

type options struct {
	kubeconfig    string
	contextName   string
	namespace     string
	allNamespaces bool
	output        string
}

func parseOptions(args []string) (options, error) {
	var opts options
	flags := flag.NewFlagSet("ksql", flag.ContinueOnError)
	// CLI errors are rendered as JSON by Run; suppress flag's plain-text output.
	flags.SetOutput(io.Discard)
	flags.StringVar(&opts.kubeconfig, "kubeconfig", "", "path to kubeconfig")
	flags.StringVar(&opts.contextName, "context", "", "kubeconfig context")
	flags.StringVar(&opts.namespace, "namespace", "", "Kubernetes namespace")
	flags.BoolVar(&opts.allNamespaces, "all-namespaces", false, "query all namespaces")
	flags.StringVar(&opts.output, "output", "json", "output format")
	if err := flags.Parse(args); err != nil {
		return options{}, apperror.Wrap(apperror.CodeUsage, "invalid command-line arguments", err)
	}
	if opts.output != "json" {
		return options{}, apperror.New(apperror.CodeUsage, "unsupported output format "+opts.output)
	}
	return opts, nil
}
