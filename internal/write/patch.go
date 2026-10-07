package write

import (
	"encoding/json"
	"fmt"

	"github.com/zyzzyh/kubesql/internal/ast"
)

type patchBuilder struct {
	operations []map[string]any
}

func newPatchBuilder(resourceVersion string) *patchBuilder {
	builder := &patchBuilder{}
	if resourceVersion != "" {
		// Test the version before changing fields. If another writer updated the
		// object after our List request, the API server rejects this patch instead
		// of allowing a stale read to overwrite newer data.
		builder.operations = append(builder.operations, map[string]any{
			"op": "test", "path": "/metadata/resourceVersion", "value": resourceVersion,
		})
	}
	return builder
}

func (p *patchBuilder) set(path string, value any, exists bool) {
	op := "replace"
	if !exists {
		op = "add"
	}
	operation := map[string]any{"op": op, "path": path, "value": value}
	p.operations = append(p.operations, operation)
}

func (p *patchBuilder) bytes() ([]byte, error) {
	data, err := json.Marshal(p.operations)
	if err != nil {
		return nil, fmt.Errorf("build JSON patch: %w", err)
	}
	return data, nil
}

func buildPatch(resourceVersion string, assignments []ast.Assignment, table string, object map[string]any) ([]byte, error) {
	// buildPatch translates typed assignments into a resourceVersion-guarded
	// JSON Patch without replacing the complete Kubernetes object.
	builder := newPatchBuilder(resourceVersion)
	for _, assignment := range assignments {
		value, err := literalValue(assignment.Value)
		if err != nil {
			return nil, err
		}
		path, normalized, exists, err := patchValue(table, assignment.Column, value, object)
		if err != nil {
			return nil, err
		}
		builder.set(path, normalized, exists)
	}
	return builder.bytes()
}

func patchValue(table, column string, value any, object map[string]any) (string, any, bool, error) {
	switch column {
	case "replicas":
		number, ok := value.(int64)
		if !ok || number < 0 {
			return "", nil, false, fmt.Errorf("E_SEMANTIC: replicas must be a non-negative integer")
		}
		return "/spec/replicas", number, pathExists(object, "spec", "replicas"), nil
	case "labels", "annotations":
		mapping, err := jsonMap(value, column)
		if err != nil {
			return "", nil, false, err
		}
		return "/metadata/" + column, mapping, pathExists(object, "metadata", column), nil
	case "default_backend_service":
		service, ok := value.(string)
		if !ok || service == "" {
			return "", nil, false, fmt.Errorf("E_SEMANTIC: default_backend_service must be a non-empty Service name")
		}
		if !hasIngressServiceBackend(object) {
			return "", nil, false, fmt.Errorf("E_SEMANTIC: ingress has no default Service backend")
		}
		return "/spec/defaultBackend/service/name", service, true, nil
	default:
		return "", nil, false, fmt.Errorf("E_SEMANTIC: column %q cannot be updated in table %q", column, table)
	}
}

func pathExists(object map[string]any, parents ...string) bool {
	var current any = object
	for _, parent := range parents {
		values, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = values[parent]
		if !ok {
			return false
		}
	}
	return true
}

func hasIngressServiceBackend(object map[string]any) bool {
	spec, ok := object["spec"].(map[string]any)
	if !ok {
		return false
	}
	backend, ok := spec["defaultBackend"].(map[string]any)
	if !ok {
		return false
	}
	service, ok := backend["service"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = service["name"].(string)
	return ok
}
