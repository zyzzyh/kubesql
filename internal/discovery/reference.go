package discovery

import (
	"fmt"
	"strings"
)

// Reference is an explicitly versioned Kubernetes resource reference.
// Core resources use the form v1/configmaps; grouped resources use
// apps/v1/statefulsets.
type Reference struct {
	Group    string
	Version  string
	Resource string
}

// ParseReference parses an exact group/version/resource reference.
func ParseReference(value string) (Reference, error) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 && len(parts) != 3 {
		return Reference{}, fmt.Errorf("E_DISCOVERY: resource reference %q must contain version/resource or group/version/resource", value)
	}
	for _, part := range parts {
		if part == "" {
			return Reference{}, fmt.Errorf("E_DISCOVERY: resource reference %q contains an empty segment", value)
		}
	}

	ref := Reference{Resource: parts[len(parts)-1]}
	if len(parts) == 2 {
		ref.Version = parts[0]
	} else {
		ref.Group = parts[0]
		ref.Version = parts[1]
	}
	return ref, nil
}
