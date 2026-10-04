// Package kube creates Kubernetes clients from kubeconfig settings.
package kube

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// NewClient loads kubeconfig and returns the typed client and its default namespace.
func NewClient(kubeconfigPath, contextName string) (kubernetes.Interface, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}

	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("load kubeconfig: %w", err)
	}
	namespace, _, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).Namespace()
	if err != nil {
		return nil, "", fmt.Errorf("load kubeconfig namespace: %w", err)
	}
	if namespace == "" {
		namespace = "default"
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, "", fmt.Errorf("create Kubernetes client: %w", err)
	}
	return client, namespace, nil
}
