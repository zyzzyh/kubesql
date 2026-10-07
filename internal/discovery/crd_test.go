package discovery

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestWaitForCRDRequiresEstablishedAndDiscovery(t *testing.T) {
	crd := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata":   map[string]any{"name": "gadgets.lab.example.com"},
		"spec": map[string]any{
			"group": "lab.example.com", "scope": "Namespaced",
			"names":    map[string]any{"plural": "gadgets", "kind": "Widget"},
			"versions": []any{map[string]any{"name": "v1", "served": true, "storage": true}},
		},
		"status": map[string]any{"conditions": []any{map[string]any{"type": "Established", "status": "True"}}},
	}}
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), crd)
	discoveryClient := &staticDiscovery{lists: []*metav1.APIResourceList{{
		GroupVersion: "lab.example.com/v1",
		APIResources: []metav1.APIResource{{Name: "gadgets", Kind: "Widget", Namespaced: true, Verbs: metav1.Verbs{"list", "create", "patch", "delete"}}},
	}}}
	resolver := NewResolver(discoveryClient)
	if err := resolver.WaitForCRD(context.Background(), dynamicClient, crd, time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForCRDTimesOutUntilEstablished(t *testing.T) {
	crd := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1", "kind": "CustomResourceDefinition",
		"metadata": map[string]any{"name": "notes.lab.example.com"},
		"spec": map[string]any{
			"group": "lab.example.com", "scope": "Cluster",
			"names":    map[string]any{"plural": "notes", "kind": "Note"},
			"versions": []any{map[string]any{"name": "v1", "served": true}},
		},
	}}
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), crd)
	resolver := NewResolver(&staticDiscovery{lists: []*metav1.APIResourceList{{
		GroupVersion: "lab.example.com/v1", APIResources: []metav1.APIResource{{Name: "notes", Kind: "Note"}},
	}}})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if err := resolver.WaitForCRD(ctx, dynamicClient, crd, time.Second); err == nil {
		t.Fatal("unestablished CRD was reported ready")
	}
}

func TestWaitForCRDValidatesInputKind(t *testing.T) {
	object := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "Namespace"}}
	if err := NewResolver(&staticDiscovery{}).WaitForCRD(context.Background(), nil, object, time.Second); err == nil {
		t.Fatal("non-CRD object was accepted")
	}
}
