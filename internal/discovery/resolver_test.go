package discovery

import (
	"errors"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sdiscovery "k8s.io/client-go/discovery"
)

type staticDiscovery struct {
	k8sdiscovery.DiscoveryInterface
	groups []*metav1.APIGroup
	lists  []*metav1.APIResourceList
	err    error
}

func (s *staticDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return s.groups, s.lists, s.err
}

func TestResolveUsesExactServedVersionAndScope(t *testing.T) {
	client := &staticDiscovery{lists: []*metav1.APIResourceList{
		{GroupVersion: "lab.example.com/v1", APIResources: []metav1.APIResource{{Name: "gadgets", Kind: "Widget", Namespaced: true, Verbs: metav1.Verbs{"list", "create"}}}},
		{GroupVersion: "lab.example.com/v2", APIResources: []metav1.APIResource{{Name: "gadgets", Kind: "Widget", Namespaced: true, Verbs: metav1.Verbs{"list"}}}},
	}}
	got, err := NewResolver(client).Resolve("lab.example.com/v1/gadgets", true)
	if err != nil {
		t.Fatal(err)
	}
	want := schema.GroupVersionResource{Group: "lab.example.com", Version: "v1", Resource: "gadgets"}
	if got.GVR != want || !got.Namespaced || !got.Verbs["create"] {
		t.Fatalf("resource: got %#v", got)
	}
}

func TestResolveDoesNotTreatFailedGroupAsMissing(t *testing.T) {
	partial := &k8sdiscovery.ErrGroupDiscoveryFailed{Groups: map[schema.GroupVersion]error{
		{Group: "lab.example.com", Version: "v1"}: errors.New("temporarily unavailable"),
	}}
	client := &staticDiscovery{
		lists: []*metav1.APIResourceList{{GroupVersion: "lab.example.com/v1", APIResources: []metav1.APIResource{{Name: "gadgets", Kind: "Widget"}}}},
		err:   partial,
	}
	_, err := NewResolver(client).Resolve("lab.example.com/v1/gadgets", true)
	if err == nil || !strings.Contains(err.Error(), "E_DISCOVERY") || !strings.Contains(err.Error(), "temporarily unavailable") {
		t.Fatalf("expected group discovery failure, got %v", err)
	}
}

func TestResolveKeepsHealthyGroupUsableDuringPartialDiscovery(t *testing.T) {
	partial := &k8sdiscovery.ErrGroupDiscoveryFailed{Groups: map[schema.GroupVersion]error{
		{Group: "broken.example.com", Version: "v1"}: errors.New("group unavailable"),
	}}
	client := &staticDiscovery{
		lists: []*metav1.APIResourceList{{GroupVersion: "lab.example.com/v1", APIResources: []metav1.APIResource{{Name: "gadgets", Kind: "Widget"}}}},
		err:   partial,
	}
	got, err := NewResolver(client).Resolve("lab.example.com/v1/gadgets", true)
	if err != nil || got.GVR.Group != "lab.example.com" {
		t.Fatalf("healthy group resolution: resource=%#v error=%v", got, err)
	}
}

func TestResolveRejectsSubresourcesExplicitly(t *testing.T) {
	_, err := NewResolver(&staticDiscovery{}).Resolve("apps/v1/deployments/status", true)
	if err == nil || !strings.Contains(err.Error(), "E_UNSUPPORTED_SUBRESOURCE") {
		t.Fatalf("expected unsupported subresource error, got %v", err)
	}
}
