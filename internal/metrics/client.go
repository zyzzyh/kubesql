// Package metrics reads the Kubernetes Metrics API without depending on the
// optional metrics client module. The request still uses client-go REST,
// kubeconfig authentication, and Kubernetes API error types.
package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured/unstructuredscheme"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

const (
	group   = "metrics.k8s.io"
	version = "v1beta1"
)

// Client is the subset of Metrics API operations used by KubeSQL.
type Client interface {
	ListPodMetrics(context.Context, string) ([]PodMetric, error)
	ListNodeMetrics(context.Context) ([]NodeMetric, error)
}

// RESTClient accesses the standard metrics.k8s.io/v1beta1 endpoints.
type RESTClient struct {
	client rest.Interface
}

// NewClient creates a Metrics API client from an already authenticated REST
// configuration. It does not contact the server until a list method is called.
func NewClient(config *rest.Config) (Client, error) {
	if config == nil {
		return nil, fmt.Errorf("E_METRICS_UNAVAILABLE: REST config is nil")
	}
	copy := rest.CopyConfig(config)
	copy.APIPath = "/apis"
	copy.ContentConfig.GroupVersion = &schema.GroupVersion{Group: group, Version: version}
	copy.ContentConfig.NegotiatedSerializer = unstructuredscheme.NewUnstructuredNegotiatedSerializer()
	client, err := rest.RESTClientFor(copy)
	if err != nil {
		return nil, fmt.Errorf("E_METRICS_UNAVAILABLE: create Metrics API client: %w", err)
	}
	return &RESTClient{client: client}, nil
}

// ListPodMetrics lists all metrics samples in one namespace. The API omits
// pods without a current sample; callers therefore never synthesize zeroes.
func (c *RESTClient) ListPodMetrics(ctx context.Context, namespace string) ([]PodMetric, error) {
	var list unstructured.UnstructuredList
	request := c.client.Get().NamespaceIfScoped(namespace, namespace != "").Resource("pods")
	err := intoList(request.Do(ctx), &list)
	if err != nil {
		return nil, unavailable(err)
	}
	return parsePodMetrics(list.Items)
}

// ListNodeMetrics lists the cluster-scoped node samples.
func (c *RESTClient) ListNodeMetrics(ctx context.Context) ([]NodeMetric, error) {
	var list unstructured.UnstructuredList
	err := intoList(c.client.Get().Resource("nodes").Do(ctx), &list)
	if err != nil {
		return nil, unavailable(err)
	}
	return parseNodeMetrics(list.Items)
}

func intoList(result rest.Result, list *unstructured.UnstructuredList) error {
	statusCode := 0
	result.StatusCode(&statusCode)
	data, err := result.Raw()
	if err != nil {
		if statusCode >= http.StatusBadRequest {
			var status metav1.Status
			if json.Unmarshal(data, &status) == nil && status.Code != 0 {
				return apierrors.FromObject(&status)
			}
		}
		return err
	}
	return result.Into(list)
}

func unavailable(err error) error {
	// Forbidden and other API StatusErrors are preserved so apperror can expose
	// permission and conflict categories. Transport, missing API, and decode
	// failures are reported as Metrics unavailable.
	if isForbidden(err) {
		return err
	}
	return fmt.Errorf("E_METRICS_UNAVAILABLE: Metrics API request failed: %w", err)
}

func isForbidden(err error) bool {
	return apierrors.IsForbidden(err)
}
