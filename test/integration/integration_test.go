package integration

import "testing"

func TestSelectAndWhereAgainstCluster(t *testing.T) {
	requireIntegration(t)
	buildBinary(t)
	namespace := newFixture(t)
	rows := decodeRows(t, runSQL(t, namespace, "SELECT name, namespace, replicas FROM deployments;"))
	if len(rows) != 2 {
		t.Fatalf("deployment rows: got %d, want 2", len(rows))
	}
	namespaces := decodeRows(t, runSQL(t, namespace, "SELECT name FROM namespaces WHERE name = '"+namespace+"';"))
	if len(namespaces) != 1 || namespaces[0]["name"] != namespace {
		t.Fatalf("namespace rows: got %#v", namespaces)
	}
	ingresses := decodeRows(t, runSQL(t, namespace, "SELECT name, default_backend_service FROM ingresses;"))
	if len(ingresses) != 1 || ingresses[0]["default_backend_service"] != "web" {
		t.Fatalf("ingress rows: got %#v", ingresses)
	}
	filtered := decodeRows(t, runSQL(t, namespace, "SELECT name FROM deployments WHERE name = 'web' OR name = 'worker' AND replicas >= 3;"))
	if len(filtered) != 2 {
		t.Fatalf("filtered rows: got %#v, want web and worker", filtered)
	}
}

func TestUpdatePreservesUnchangedDeploymentFields(t *testing.T) {
	requireIntegration(t)
	buildBinary(t)
	namespace := newFixture(t)
	result := decodeResult(t, runSQL(t, namespace, "UPDATE deployments SET replicas = 2 WHERE name = 'web';"))
	if result["affected_rows"] != float64(1) {
		t.Fatalf("update result: %#v", result)
	}
	object := kubectlJSON(t, "get", "deployment", "web", "-n", namespace)
	spec := object["spec"].(map[string]any)
	if spec["replicas"] != float64(2) {
		t.Fatalf("replicas: got %#v", spec["replicas"])
	}
	selector := spec["selector"].(map[string]any)["matchLabels"].(map[string]any)
	if selector["app"] != "web" {
		t.Fatalf("selector changed: %#v", selector)
	}
	template := spec["template"].(map[string]any)
	containers := template["spec"].(map[string]any)["containers"].([]any)
	image := containers[0].(map[string]any)["image"]
	if image != "nginx:1.27" {
		t.Fatalf("image changed: %#v", image)
	}
}

func TestDeleteRemovesOnlyMatchingDeployment(t *testing.T) {
	requireIntegration(t)
	buildBinary(t)
	namespace := newFixture(t)
	result := decodeResult(t, runSQL(t, namespace, "DELETE FROM deployments WHERE name = 'worker';"))
	if result["affected_rows"] != float64(1) {
		t.Fatalf("delete result: %#v", result)
	}
	waitForDeletion(t, "deployment/worker", "-n", namespace)
	web := kubectlJSON(t, "get", "deployment", "web", "-n", namespace)
	if web["metadata"].(map[string]any)["name"] != "web" {
		t.Fatalf("web deployment missing: %#v", web)
	}
}

func TestInsertCreatesAndRejectsDuplicateDeployment(t *testing.T) {
	requireIntegration(t)
	buildBinary(t)
	namespace := newFixture(t)
	sql := `INSERT INTO deployments (manifest) VALUES ('{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"api"},"spec":{"replicas":2,"selector":{"matchLabels":{"app":"api"}},"template":{"metadata":{"labels":{"app":"api"}},"spec":{"containers":[{"name":"api","image":"nginx:1.27"}]}}}}');`
	first := runSQL(t, namespace, sql)
	if first.err != nil {
		t.Fatalf("insert failed: %v\nstderr:\n%s", first.err, first.stderr)
	}
	if result := decodeResult(t, first); result["affected_rows"] != float64(1) {
		t.Fatalf("insert result: %#v", result)
	}
	object := kubectlJSON(t, "get", "deployment", "api", "-n", namespace)
	if object["metadata"].(map[string]any)["namespace"] != namespace {
		t.Fatalf("namespace injection failed: %#v", object["metadata"])
	}
	duplicate := runSQL(t, namespace, sql)
	if duplicate.err == nil {
		t.Fatal("duplicate INSERT returned success")
	}
	result := decodeResult(t, duplicate)
	errors := result["errors"].([]any)
	if len(errors) != 1 || errors[0].(map[string]any)["reason"] != "AlreadyExists" {
		t.Fatalf("duplicate result: %#v", result)
	}
}
