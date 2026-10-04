package query

import (
	"context"
	"reflect"
	"testing"

	"github.com/zyzzyh/kubesql/internal/ast"
	"github.com/zyzzyh/kubesql/internal/parser"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestExecuteProjectsSupportedTables(t *testing.T) {
	replicas := int32(2)
	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "sql-easy-select"}},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "sql-easy-select"},
			Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		},
	)

	statement := parseSelect(t, "SELECT name, namespace, replicas FROM deployments;")
	rows, err := NewExecutor(client, "sql-easy-select", false).Execute(context.Background(), statement)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{{"name": "web", "namespace": "sql-easy-select", "replicas": int32(2)}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows: got %#v, want %#v", rows, want)
	}

	statement = parseSelect(t, "SELECT * FROM namespaces")
	rows, err = NewExecutor(client, "sql-easy-select", false).Execute(context.Background(), statement)
	if err != nil {
		t.Fatal(err)
	}
	want = []map[string]any{{"name": "sql-easy-select"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("namespace rows: got %#v, want %#v", rows, want)
	}
}

func TestExecuteAllNamespaces(t *testing.T) {
	replicas := int32(1)
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web-a", Namespace: "a"}, Spec: appsv1.DeploymentSpec{Replicas: &replicas}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web-b", Namespace: "b"}, Spec: appsv1.DeploymentSpec{Replicas: &replicas}},
	)
	statement := parseSelect(t, "SELECT name, namespace FROM deployments")
	rows, err := NewExecutor(client, "a", true).Execute(context.Background(), statement)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("row count: got %d, want 2", len(rows))
	}
}

func TestIngressWithoutDefaultBackendProjectsNull(t *testing.T) {
	client := fake.NewSimpleClientset(&networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "internal", Namespace: "sql-easy-where"},
	})
	statement := parseSelect(t, "SELECT name, default_backend_service FROM ingresses")
	rows, err := NewExecutor(client, "sql-easy-where", false).Execute(context.Background(), statement)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{{"name": "internal", "default_backend_service": nil}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows: got %#v, want %#v", rows, want)
	}
}

func TestSemanticValidationHappensBeforeAPIRequest(t *testing.T) {
	client := fake.NewSimpleClientset()
	statement := parseSelect(t, "SELECT missing FROM deployments")
	if _, err := NewExecutor(client, "default", false).Execute(context.Background(), statement); err == nil {
		t.Fatal("unknown column was accepted")
	}
	if actions := client.Actions(); len(actions) != 0 {
		t.Fatalf("API was called before semantic validation: %#v", actions)
	}
}

func TestExecuteWhereFiltersRows(t *testing.T) {
	one, three := int32(1), int32(3)
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "sql-easy-where"}, Spec: appsv1.DeploymentSpec{Replicas: &one}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "sql-easy-where"}, Spec: appsv1.DeploymentSpec{Replicas: &three}},
	)
	statement := parseSelect(t, "SELECT name FROM deployments WHERE name = 'worker' AND replicas >= 3")
	rows, err := NewExecutor(client, "sql-easy-where", false).Execute(context.Background(), statement)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{{"name": "worker"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows: got %#v, want %#v", rows, want)
	}
}

func TestExecuteWhereHonorsAndBeforeOr(t *testing.T) {
	one, three := int32(1), int32(3)
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "sql-easy-where"}, Spec: appsv1.DeploymentSpec{Replicas: &one}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "sql-easy-where"}, Spec: appsv1.DeploymentSpec{Replicas: &three}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "idle", Namespace: "sql-easy-where"}, Spec: appsv1.DeploymentSpec{Replicas: &one}},
	)
	statement := parseSelect(t, "SELECT name FROM deployments WHERE name = 'web' OR name = 'worker' AND replicas >= 3")
	rows, err := NewExecutor(client, "sql-easy-where", false).Execute(context.Background(), statement)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0]["name"] != "web" || rows[1]["name"] != "worker" {
		t.Fatalf("rows: got %#v, want web and worker", rows)
	}
}

func TestExecuteWhereIsNull(t *testing.T) {
	client := fake.NewSimpleClientset(
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "internal", Namespace: "sql-easy-where"}},
	)
	statement := parseSelect(t, "SELECT name FROM ingresses WHERE default_backend_service IS NULL")
	rows, err := NewExecutor(client, "sql-easy-where", false).Execute(context.Background(), statement)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["name"] != "internal" {
		t.Fatalf("rows: got %#v, want internal", rows)
	}
}

func TestWhereUnknownColumnIsRejectedBeforeAPIRequest(t *testing.T) {
	client := fake.NewSimpleClientset()
	statement := parseSelect(t, "SELECT name FROM deployments WHERE missing = 1")
	if _, err := NewExecutor(client, "default", false).Execute(context.Background(), statement); err == nil {
		t.Fatal("unknown WHERE column was accepted")
	}
	if actions := client.Actions(); len(actions) != 0 {
		t.Fatalf("API was called before WHERE validation: %#v", actions)
	}
}

func TestWhereIncompatibleTypesReturnError(t *testing.T) {
	replicas := int32(1)
	client := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
	})
	statement := parseSelect(t, "SELECT name FROM deployments WHERE replicas = 'one'")
	if _, err := NewExecutor(client, "default", false).Execute(context.Background(), statement); err == nil {
		t.Fatal("incompatible WHERE types were accepted")
	}
}

func parseSelect(t *testing.T, input string) *ast.SelectStatement {
	t.Helper()
	statement, err := parser.New(input).Parse()
	if err != nil {
		t.Fatal(err)
	}
	return statement.(*ast.SelectStatement)
}
