package discovery

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
)

var crdGVR = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}

// WaitForCRD waits for a created CRD to become Established and for each served
// version to appear in API Discovery. Creating the CRD is not considered a
// successful registration until both conditions hold.
func (r *Resolver) WaitForCRD(ctx context.Context, client dynamic.Interface, object *unstructured.Unstructured, timeout time.Duration) error {
	if object == nil || object.GetKind() != "CustomResourceDefinition" || object.GetAPIVersion() != "apiextensions.k8s.io/v1" {
		return fmt.Errorf("E_SEMANTIC: WaitForCRD requires an apiextensions.k8s.io/v1 CustomResourceDefinition")
	}
	name := object.GetName()
	group, found, err := unstructured.NestedString(object.Object, "spec", "group")
	if err != nil || !found || group == "" {
		return fmt.Errorf("E_SEMANTIC: CRD spec.group is required")
	}
	plural, found, err := unstructured.NestedString(object.Object, "spec", "names", "plural")
	if err != nil || !found || plural == "" {
		return fmt.Errorf("E_SEMANTIC: CRD spec.names.plural is required")
	}
	kind, found, err := unstructured.NestedString(object.Object, "spec", "names", "kind")
	if err != nil || !found || kind == "" {
		return fmt.Errorf("E_SEMANTIC: CRD spec.names.kind is required")
	}
	scope, found, err := unstructured.NestedString(object.Object, "spec", "scope")
	if err != nil || !found || (scope != "Namespaced" && scope != "Cluster") {
		return fmt.Errorf("E_SEMANTIC: CRD spec.scope must be Namespaced or Cluster")
	}
	versions, found, err := unstructured.NestedSlice(object.Object, "spec", "versions")
	if err != nil || !found || len(versions) == 0 {
		return fmt.Errorf("E_SEMANTIC: CRD spec.versions is required")
	}
	expected := make([]schema.GroupVersionResource, 0, len(versions))
	for _, value := range versions {
		version, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("E_SEMANTIC: CRD version entry must be an object")
		}
		versionName, _ := version["name"].(string)
		served, _ := version["served"].(bool)
		if served && versionName != "" {
			expected = append(expected, schema.GroupVersionResource{Group: group, Version: versionName, Resource: plural})
		}
	}
	if len(expected) == 0 {
		return fmt.Errorf("E_SEMANTIC: CRD must serve at least one version")
	}

	err = wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, timeout, true, func(ctx context.Context) (bool, error) {
		current, err := client.Resource(crdGVR).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("E_DISCOVERY: read CRD %q while waiting for registration: %w", name, err)
		}
		if !conditionTrue(current.Object, "Established") {
			return false, nil
		}
		for _, gvr := range expected {
			reference := gvr.Group + "/" + gvr.Version + "/" + gvr.Resource
			resource, err := r.Resolve(reference, true)
			if err != nil {
				return false, nil
			}
			if resource.GVR != gvr || resource.Kind != kind || resource.Namespaced != (scope == "Namespaced") {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("E_DISCOVERY: CRD %q did not become Established and discoverable: %w", name, err)
	}
	return nil
}

func conditionTrue(object map[string]any, conditionType string) bool {
	conditions, found, err := unstructured.NestedSlice(object, "status", "conditions")
	if err != nil || !found {
		return false
	}
	for _, raw := range conditions {
		condition, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if condition["type"] == conditionType && condition["status"] == "True" {
			return true
		}
	}
	return false
}
