package query

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

type resourceRow struct {
	name                  string
	namespace             string
	replicas              any
	defaultBackendService any
}

type resourceList struct {
	rows []resourceRow
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

func project(resources resourceList, tableName string, columns []string) []map[string]any {
	result := make([]map[string]any, 0, len(resources.rows))
	for _, resource := range resources.rows {
		row := make(map[string]any, len(columns))
		for _, column := range columns {
			switch column {
			case "name":
				row[column] = resource.name
			case "namespace":
				row[column] = resource.namespace
			case "replicas":
				row[column] = resource.replicas
			case "default_backend_service":
				row[column] = resource.defaultBackendService
			}
		}
		result = append(result, row)
	}
	return result
}
