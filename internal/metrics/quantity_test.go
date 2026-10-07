package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

func TestParsePodMetricsSumsContainerQuantities(t *testing.T) {
	items := []unstructured.Unstructured{{Object: map[string]any{
		"metadata": map[string]any{"name": "measure", "namespace": "demo"},
		"containers": []any{
			map[string]any{"name": "app", "usage": map[string]any{"cpu": "100m", "memory": "32Mi"}},
			map[string]any{"name": "sidecar", "usage": map[string]any{"cpu": "50m", "memory": "48Mi"}},
		},
	}}}
	got, err := parsePodMetrics(items)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "measure" || got[0].Namespace != "demo" || got[0].CPUMillicores != 150 || got[0].MemoryBytes != 80*1024*1024 {
		t.Fatalf("metrics: got %#v", got)
	}
}

func TestMissingContainerSampleIsOmitted(t *testing.T) {
	items := []unstructured.Unstructured{{Object: map[string]any{
		"metadata":   map[string]any{"name": "waiting", "namespace": "demo"},
		"containers": []any{},
	}}}
	got, err := parsePodMetrics(items)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("missing samples must not become zero values: %#v", got)
	}
}

func TestParseNodeMetricUsesQuantityConversions(t *testing.T) {
	items := []unstructured.Unstructured{{Object: map[string]any{
		"metadata": map[string]any{"name": "worker-1"},
		"usage":    map[string]any{"cpu": "250m", "memory": "512Mi"},
	}}}
	got, err := parseNodeMetrics(items)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "worker-1" || got[0].CPUMillicores != 250 || got[0].MemoryBytes != 512*1024*1024 {
		t.Fatalf("metrics: got %#v", got)
	}
}

func TestListPodMetricsUsesMetricsAPIAndPreservesForbidden(t *testing.T) {
	t.Run("path and quantity conversion", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/apis/metrics.k8s.io/v1beta1/namespaces/demo/pods" {
				t.Errorf("request path: got %q", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"apiVersion":"metrics.k8s.io/v1beta1","kind":"PodMetricsList","items":[{"apiVersion":"metrics.k8s.io/v1beta1","kind":"PodMetrics","metadata":{"name":"measure","namespace":"demo"},"containers":[{"name":"app","usage":{"cpu":"25m","memory":"10Mi"}}]}]}`))
		}))
		defer server.Close()
		client, err := NewClient(&rest.Config{Host: server.URL})
		if err != nil {
			t.Fatal(err)
		}
		got, err := client.ListPodMetrics(context.Background(), "demo")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].CPUMillicores != 25 || got[0].MemoryBytes != 10*1024*1024 {
			t.Fatalf("metrics: got %#v", got)
		}
	})

	t.Run("forbidden stays a permission error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Status","status":"Failure","message":"forbidden","reason":"Forbidden","code":403}`))
		}))
		defer server.Close()
		client, err := NewClient(&rest.Config{Host: server.URL})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.ListPodMetrics(context.Background(), "demo")
		if err == nil || !strings.Contains(err.Error(), "forbidden") {
			t.Fatalf("expected preserved Forbidden error, got %v", err)
		}
	})
}
