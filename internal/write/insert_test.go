package write

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestInsertCreatesDeploymentInCLINamespace(t *testing.T) {
	client := fake.NewSimpleClientset()
	statement := parseWrite(t, `INSERT INTO deployments (manifest) VALUES ('{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"api"},"spec":{"replicas":2,"selector":{"matchLabels":{"app":"api"}},"template":{"metadata":{"labels":{"app":"api"}},"spec":{"containers":[{"name":"api","image":"nginx:1.27"}]}}}}')`)
	result, err := NewExecutor(client, "demo").Execute(context.Background(), statement)
	if err != nil {
		t.Fatal(err)
	}
	if result.AffectedRows != 1 || result.FailedRows != 0 {
		t.Fatalf("result: got %#v", result)
	}
	created, err := client.AppsV1().Deployments("demo").Get(context.Background(), "api", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if created.Namespace != "demo" || created.Spec.Replicas == nil || *created.Spec.Replicas != 2 {
		t.Fatalf("unexpected Deployment: %#v", created)
	}
}

func TestInsertCreatesNamespace(t *testing.T) {
	client := fake.NewSimpleClientset()
	statement := parseWrite(t, `INSERT INTO namespaces (manifest) VALUES ('{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"demo"}}')`)
	result, err := NewExecutor(client, "ignored").Execute(context.Background(), statement)
	if err != nil {
		t.Fatal(err)
	}
	if result.AffectedRows != 1 {
		t.Fatalf("result: got %#v", result)
	}
	if _, err := client.CoreV1().Namespaces().Get(context.Background(), "demo", metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestInsertDuplicateReportsAlreadyExists(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "demo"}})
	statement := parseWrite(t, `INSERT INTO namespaces (manifest) VALUES ('{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"demo"}}')`)
	result, err := NewExecutor(client, "ignored").Execute(context.Background(), statement)
	if err != nil {
		t.Fatal(err)
	}
	if result.AffectedRows != 0 || result.FailedRows != 1 || len(result.Errors) != 1 || result.Errors[0].Reason != "AlreadyExists" {
		t.Fatalf("result: got %#v", result)
	}
}

func TestInsertRejectsInvalidManifestBeforeAPIRequest(t *testing.T) {
	client := fake.NewSimpleClientset()
	inputs := []string{
		`INSERT INTO deployments (manifest) VALUES ('{"apiVersion":"v1","kind":"Pod","metadata":{"name":"api"}}')`,
		`INSERT INTO deployments (manifest) VALUES ('{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"api","namespace":"other"}}')`,
		`INSERT INTO namespaces (manifest) VALUES ('{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"demo","namespace":"other"}}')`,
		`INSERT INTO namespaces (manifest) VALUES ('{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"demo"},"status":{}}')`,
	}
	for _, input := range inputs {
		statement := parseWrite(t, input)
		if _, err := NewExecutor(client, "demo").Execute(context.Background(), statement); err == nil {
			t.Errorf("%q: expected validation error", input)
		}
	}
	if len(client.Actions()) != 0 {
		t.Fatalf("API was called before manifest validation: %#v", client.Actions())
	}
}

func TestInsertPreservesDeploymentFields(t *testing.T) {
	client := fake.NewSimpleClientset()
	statement := parseWrite(t, `INSERT INTO deployments (manifest) VALUES ('{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"web"},"spec":{"selector":{"matchLabels":{"app":"web"}},"template":{"metadata":{"labels":{"app":"web"}},"spec":{"containers":[{"name":"web","image":"nginx:1.27"}]}}}}')`)
	if _, err := NewExecutor(client, "demo").Execute(context.Background(), statement); err != nil {
		t.Fatal(err)
	}
	created, err := client.AppsV1().Deployments("demo").Get(context.Background(), "web", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if created.Spec.Selector.MatchLabels["app"] != "web" || created.Spec.Template.Spec.Containers[0].Image != "nginx:1.27" {
		t.Fatalf("manifest fields changed: %#v", created.Spec)
	}
}
