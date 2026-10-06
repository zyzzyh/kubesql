package query

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type resourceRow struct {
	name                  string
	namespace             string
	replicas              any
	defaultBackendService any
	dynamic               map[string]any
}

type resourceList struct {
	rows []resourceRow
}

func (r resourceRow) values() map[string]any {
	if r.dynamic != nil {
		return flattenDynamic(r.dynamic)
	}
	return map[string]any{
		"name":                    r.name,
		"namespace":               r.namespace,
		"replicas":                r.replicas,
		"default_backend_service": r.defaultBackendService,
	}
}

func flattenDynamic(object map[string]any) map[string]any {
	values := make(map[string]any, len(object)+2)
	values["name"], _, _ = unstructured.NestedString(object, "metadata", "name")
	values["namespace"], _, _ = unstructured.NestedString(object, "metadata", "namespace")
	for key, value := range object {
		values[key] = value
		flattenNested(values, "/"+key, value)
	}
	return values
}

func flattenNested(values map[string]any, prefix string, value any) {
	child, ok := value.(map[string]any)
	if !ok {
		return
	}
	for key, item := range child {
		path := prefix + "/" + key
		values[path] = item
		flattenNested(values, path, item)
	}
}

func fromNamespaces(items *corev1.NamespaceList) resourceList {
	rows := make([]resourceRow, 0, len(items.Items))
	for _, item := range items.Items {
		rows = append(rows, resourceRow{name: item.Name})
	}
	return resourceList{rows: rows}
}

func fromDeployments(items *appsv1.DeploymentList) resourceList {
	rows := make([]resourceRow, 0, len(items.Items))
	for _, item := range items.Items {
		var replicas any
		if item.Spec.Replicas != nil {
			replicas = *item.Spec.Replicas
		}
		rows = append(rows, resourceRow{name: item.Name, namespace: item.Namespace, replicas: replicas})
	}
	return resourceList{rows: rows}
}

func fromIngresses(items *networkingv1.IngressList) resourceList {
	rows := make([]resourceRow, 0, len(items.Items))
	for _, item := range items.Items {
		service := any(nil)
		if item.Spec.DefaultBackend != nil && item.Spec.DefaultBackend.Service != nil {
			service = item.Spec.DefaultBackend.Service.Name
		}
		rows = append(rows, resourceRow{name: item.Name, namespace: item.Namespace, defaultBackendService: service})
	}
	return resourceList{rows: rows}
}
