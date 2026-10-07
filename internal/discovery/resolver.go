// Package discovery resolves SQL table names through Kubernetes API discovery.
package discovery

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sdiscovery "k8s.io/client-go/discovery"
)

// Resource describes a discoverable Kubernetes resource.
type Resource struct {
	Name       string
	Kind       string
	GVR        schema.GroupVersionResource
	Namespaced bool
	Verbs      map[string]bool
}

// DiscoveryError retains a discovery failure while allowing callers to use
// resources returned by the server before the failure occurred.
type DiscoveryError struct {
	Cause error
}

func (e *DiscoveryError) Error() string {
	return fmt.Sprintf("E_DISCOVERY: Kubernetes API discovery failed: %v", e.Cause)
}

func (e *DiscoveryError) Unwrap() error { return e.Cause }

// Resolver maps SQL resource references to Kubernetes GroupVersionResources.
type Resolver struct {
	client k8sdiscovery.DiscoveryInterface
}

func NewResolver(client k8sdiscovery.DiscoveryInterface) *Resolver {
	return &Resolver{client: client}
}

// Resolve resolves either a plain resource name or an exact versioned
// reference. Plain names select the preferred version within one API group.
func (r *Resolver) Resolve(reference string, exact bool) (Resource, error) {
	// Resolve maps a SQL resource name to GVR, scope, kind, and supported verbs.
	if r == nil || r.client == nil {
		return Resource{}, fmt.Errorf("E_DISCOVERY: discovery client is nil")
	}

	var requested Reference
	var err error
	if exact {
		requested, err = ParseReference(reference)
		if err != nil {
			return Resource{}, err
		}
	}

	groups, lists, discoveryErr := r.client.ServerGroupsAndResources()
	preferred := preferredVersions(groups)
	candidates := collectCandidates(lists, requested, exact)
	if len(candidates) == 0 {
		if discoveryErr != nil {
			return Resource{}, &DiscoveryError{Cause: discoveryErr}
		}
		if exact {
			return Resource{}, fmt.Errorf("E_DISCOVERY: resource %q was not found", reference)
		}
		return Resource{}, fmt.Errorf("E_DISCOVERY: resource %q was not found", reference)
	}

	selected, err := selectCandidate(candidates, preferred, exact, reference)
	if err != nil {
		return Resource{}, err
	}
	if discoveryErr != nil {
		return selected, &DiscoveryError{Cause: discoveryErr}
	}
	return selected, nil
}

type candidate struct {
	resource  Resource
	group     string
	version   string
	preferred bool
}

func preferredVersions(groups []*metav1.APIGroup) map[string]string {
	preferred := map[string]string{"": "v1"}
	for _, group := range groups {
		if group != nil && group.PreferredVersion.Version != "" {
			preferred[group.Name] = group.PreferredVersion.Version
		}
	}
	return preferred
}

func collectCandidates(lists []*metav1.APIResourceList, requested Reference, exact bool) []candidate {
	var candidates []candidate
	for _, list := range lists {
		if list == nil {
			continue
		}
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		if exact && (gv.Group != requested.Group || gv.Version != requested.Version) {
			continue
		}
		for _, item := range list.APIResources {
			if strings.Contains(item.Name, "/") || item.Name == "" {
				continue
			}
			if exact && item.Name != requested.Resource {
				continue
			}
			verbs := make(map[string]bool, len(item.Verbs))
			for _, verb := range item.Verbs {
				verbs[strings.ToLower(verb)] = true
			}
			candidates = append(candidates, candidate{
				resource: Resource{
					Name:       item.Name,
					Kind:       item.Kind,
					GVR:        gv.WithResource(item.Name),
					Namespaced: item.Namespaced,
					Verbs:      verbs,
				},
				group:   gv.Group,
				version: gv.Version,
			})
		}
	}
	return candidates
}

func selectCandidate(candidates []candidate, preferred map[string]string, exact bool, reference string) (Resource, error) {
	if exact {
		return candidates[0].resource, nil
	}
	groups := map[string][]candidate{}
	for _, item := range candidates {
		groups[item.group] = append(groups[item.group], item)
	}
	if len(groups) > 1 {
		return Resource{}, fmt.Errorf("E_DISCOVERY: resource %q is ambiguous across API groups", reference)
	}
	group := candidates[0].group
	version := preferred[group]
	for _, item := range groups[group] {
		if item.version == version {
			return item.resource, nil
		}
	}
	return groups[group][0].resource, nil
}
