package write

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/zyzzyh/kubesql/internal/ast"
	"github.com/zyzzyh/kubesql/internal/parser"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestUpdateDeploymentUsesLocalPatch(t *testing.T) {
	replicas := int32(1)
	client := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "demo", Labels: map[string]string{"app": "web"}},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
	})
	statement := parseWrite(t, "UPDATE deployments SET replicas = 3 WHERE name = 'web';")
	result, err := NewExecutor(client, "demo").Execute(context.Background(), statement)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result, Result{AffectedRows: 1, Errors: []ObjectError{}}) {
		t.Fatalf("result: got %#v", result)
	}
	updated, err := client.AppsV1().Deployments("demo").Get(context.Background(), "web", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Spec.Replicas == nil || *updated.Spec.Replicas != 3 || updated.Labels["app"] != "web" {
		t.Fatalf("unexpected deployment after update: %#v", updated)
	}
	if actions := client.Actions(); len(actions) != 3 {
		t.Fatalf("actions: got %d, want list, patch, get", len(actions))
	}
}

func TestDeleteOnlyMatchingObjects(t *testing.T) {
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "demo"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "demo"}},
	)
	statement := parseWrite(t, "DELETE FROM deployments WHERE name = 'web'")
	result, err := NewExecutor(client, "demo").Execute(context.Background(), statement)
	if err != nil {
		t.Fatal(err)
	}
	if result.AffectedRows != 1 || result.FailedRows != 0 {
		t.Fatalf("result: got %#v", result)
	}
	if _, err := client.AppsV1().Deployments("demo").Get(context.Background(), "web", metav1.GetOptions{}); err == nil {
		t.Fatal("web deployment was not deleted")
	}
	if _, err := client.AppsV1().Deployments("demo").Get(context.Background(), "worker", metav1.GetOptions{}); err != nil {
		t.Fatalf("worker deployment was deleted: %v", err)
	}
}

func TestWriteRequiresWhere(t *testing.T) {
	for _, input := range []string{
		"UPDATE deployments SET replicas = 3",
		"DELETE FROM deployments",
	} {
		statement, err := parser.New(input).Parse()
		if err != nil {
			t.Fatal(err)
		}
		result, executeErr := NewExecutor(fake.NewSimpleClientset(), "demo").Execute(context.Background(), statement)
		if executeErr == nil || result.AffectedRows != 0 {
			t.Fatalf("%q: expected E_WHERE_REQUIRED", input)
		}
	}
}

func TestUpdatePatchContainsResourceVersionTest(t *testing.T) {
	data, err := buildPatch("12", []ast.Assignment{{Column: "replicas", Value: &ast.Literal{Kind: "integer", Value: int64(2)}}}, "deployments", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	var operations []map[string]any
	if err := json.Unmarshal(data, &operations); err != nil {
		t.Fatal(err)
	}
	if operations[0]["op"] != "test" || operations[0]["path"] != "/metadata/resourceVersion" {
		t.Fatalf("missing resourceVersion test: %#v", operations)
	}
}

func parseWrite(t *testing.T, input string) ast.Statement {
	t.Helper()
	statement, err := parser.New(input).Parse()
	if err != nil {
		t.Fatal(err)
	}
	return statement
}
