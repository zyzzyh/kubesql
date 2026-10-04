package write

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zyzzyh/kubesql/internal/ast"
)

type manifest struct {
	table      string
	apiVersion string
	kind       string
	name       string
	namespace  string
	namespaced bool
	object     map[string]any
}

func decodeManifest(statement *ast.InsertStatement, namespace string) (manifest, error) {
	table := strings.ToLower(statement.Table)
	if !supportedInsertTable(table) {
		return manifest{}, fmt.Errorf("E_SEMANTIC: unknown table %q", statement.Table)
	}
	if len(statement.Columns) != 1 || strings.ToLower(statement.Columns[0]) != "manifest" {
		return manifest{}, fmt.Errorf("E_SEMANTIC: INSERT requires exactly one manifest column")
	}
	if len(statement.Values) != 1 {
		return manifest{}, fmt.Errorf("E_SEMANTIC: INSERT requires exactly one VALUES tuple")
	}
	literal, ok := statement.Values[0].(*ast.Literal)
	if !ok || literal.Kind != "string" {
		return manifest{}, fmt.Errorf("E_SEMANTIC: manifest must be a SQL string")
	}
	text, ok := literal.Value.(string)
	if !ok {
		return manifest{}, fmt.Errorf("E_SEMANTIC: manifest must be a SQL string")
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(text), &object); err != nil {
		return manifest{}, fmt.Errorf("E_SEMANTIC: invalid manifest JSON: %w", err)
	}
	if object == nil {
		return manifest{}, fmt.Errorf("E_SEMANTIC: manifest must be a JSON object")
	}
	apiVersion, err := requiredString(object, "apiVersion")
	if err != nil {
		return manifest{}, err
	}
	kind, err := requiredString(object, "kind")
	if err != nil {
		return manifest{}, err
	}
	expectedAPI, expectedKind, namespaced := insertResourceKind(table)
	if apiVersion != expectedAPI || kind != expectedKind {
		return manifest{}, fmt.Errorf("E_SEMANTIC: manifest apiVersion/kind %q/%q does not match table %q", apiVersion, kind, table)
	}
	metadata, ok := object["metadata"].(map[string]any)
	if !ok {
		return manifest{}, fmt.Errorf("E_SEMANTIC: manifest metadata must be an object")
	}
	name, err := requiredString(metadata, "name")
	if err != nil {
		return manifest{}, err
	}
	if err := rejectServerFields(object, metadata); err != nil {
		return manifest{}, err
	}
	manifestNamespace, err := metadataNamespace(metadata)
	if err != nil {
		return manifest{}, err
	}
	if namespaced {
		if namespace == "" {
			return manifest{}, fmt.Errorf("E_NAMESPACE_REQUIRED: namespaced INSERT requires a namespace")
		}
		if manifestNamespace != "" && manifestNamespace != namespace {
			return manifest{}, fmt.Errorf("E_SEMANTIC: manifest namespace %q differs from CLI namespace %q", manifestNamespace, namespace)
		}
		metadata["namespace"] = namespace
	} else if manifestNamespace != "" {
		return manifest{}, fmt.Errorf("E_SEMANTIC: cluster-scoped resource cannot have namespace %q", manifestNamespace)
	}
	return manifest{table: table, apiVersion: apiVersion, kind: kind, name: name, namespace: namespace, namespaced: namespaced, object: object}, nil
}

func supportedInsertTable(table string) bool {
	return table == "namespaces" || table == "deployments" || table == "ingresses"
}

func insertResourceKind(table string) (string, string, bool) {
	switch table {
	case "namespaces":
		return "v1", "Namespace", false
	case "deployments":
		return "apps/v1", "Deployment", true
	default:
		return "networking.k8s.io/v1", "Ingress", true
	}
}

func requiredString(object map[string]any, key string) (string, error) {
	value, ok := object[key]
	if !ok {
		return "", fmt.Errorf("E_SEMANTIC: manifest requires %s", key)
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return "", fmt.Errorf("E_SEMANTIC: manifest %s must be a non-empty string", key)
	}
	return text, nil
}

func metadataNamespace(metadata map[string]any) (string, error) {
	value, ok := metadata["namespace"]
	if !ok || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("E_SEMANTIC: metadata.namespace must be a string")
	}
	return text, nil
}

func rejectServerFields(object, metadata map[string]any) error {
	if _, ok := object["status"]; ok {
		return fmt.Errorf("E_SEMANTIC: status is server-managed and cannot be inserted")
	}
	for _, field := range []string{"uid", "resourceVersion", "managedFields", "creationTimestamp", "generation", "selfLink", "deletionTimestamp", "deletionGracePeriodSeconds"} {
		if _, ok := metadata[field]; ok {
			return fmt.Errorf("E_SEMANTIC: metadata.%s is server-managed and cannot be inserted", field)
		}
	}
	return nil
}
