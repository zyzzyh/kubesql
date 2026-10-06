// Package kube creates Kubernetes clients from kubeconfig settings.
package kube

import (
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// NewClient loads kubeconfig and returns the typed client and its default namespace.
func NewClient(kubeconfigPath, contextName string) (kubernetes.Interface, string, error) {
	clients, err := NewClients(kubeconfigPath, contextName)
	if err != nil {
		return nil, "", err
	}
	return clients.Typed, clients.Namespace, nil
}

// Clients contains all Kubernetes clients built from one kubeconfig.
type Clients struct {
	Typed     kubernetes.Interface
	Dynamic   dynamic.Interface
	Discovery discovery.DiscoveryInterface
	Namespace string
}

func NewClients(kubeconfigPath, contextName string) (Clients, error) {
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
		return Clients{}, fmt.Errorf("load kubeconfig: %w", err)
	}
	namespace, _, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).Namespace()
	if err != nil {
		return Clients{}, fmt.Errorf("load kubeconfig namespace: %w", err)
	}
	if namespace == "" {
		namespace = "default"
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return Clients{}, fmt.Errorf("create Kubernetes client: %w", err)
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return Clients{}, fmt.Errorf("create dynamic Kubernetes client: %w", err)
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return Clients{}, fmt.Errorf("create discovery client: %w", err)
	}
	return Clients{Typed: client, Dynamic: dynamicClient, Discovery: discoveryClient, Namespace: namespace}, nil
}
