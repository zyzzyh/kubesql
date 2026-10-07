package integration

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDiscoveredCRDSupportsNamespacedAndClusterResources(t *testing.T) {
	requireIntegration(t)
	buildBinary(t)
	namespace := newFixture(t)
	group := fmt.Sprintf("it-%d.lab.example.com", time.Now().UnixNano())
	crdName := "gadgets." + group
	t.Cleanup(func() {
		_ = runCommand(t, repositoryRoot, "kubectl", "delete", "gadgets", "--all", "-n", namespace, "--ignore-not-found", "--wait=true")
		_ = runCommand(t, repositoryRoot, "kubectl", "delete", "crd", crdName, "--ignore-not-found", "--wait=true")
	})

	crd := map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1", "kind": "CustomResourceDefinition",
		"metadata": map[string]any{"name": crdName},
		"spec": map[string]any{
			"group": group, "scope": "Namespaced",
			"names": map[string]any{"plural": "gadgets", "singular": "gadget", "kind": "Gadget"},
			"versions": []any{map[string]any{
				"name": "v1", "served": true, "storage": true,
				"schema": map[string]any{"openAPIV3Schema": map[string]any{
					"type": "object", "properties": map[string]any{"spec": map[string]any{"type": "object", "properties": map[string]any{
						"size": map[string]any{"type": "integer", "minimum": 1}, "message": map[string]any{"type": "string"},
					}}},
				}},
			}},
		},
	}
	if result := runSQL(t, namespace, insertManifestSQL("apiextensions.k8s.io/v1/customresourcedefinitions", crd)); result.err != nil {
		t.Fatalf("create CRD: %v\nstderr:\n%s", result.err, result.stderr)
	}
	crdRows := decodeRows(t, runSQL(t, namespace, "SELECT name FROM \"apiextensions.k8s.io/v1/customresourcedefinitions\" WHERE name = '"+crdName+"';"))
	if len(crdRows) != 1 {
		t.Fatalf("CRD was not queryable after registration: %#v", crdRows)
	}

	resourceName := group + "/v1/gadgets"
	resourceTable := "\"" + resourceName + "\""
	sample := map[string]any{"apiVersion": group + "/v1", "kind": "Gadget", "metadata": map[string]any{"name": "sample"}, "spec": map[string]any{"size": 2, "message": "hello"}}
	if result := runSQL(t, namespace, insertManifestSQL(resourceName, sample)); result.err != nil {
		t.Fatalf("create Custom Resource: %v\nstderr:\n%s", result.err, result.stderr)
	}
	rows := decodeRows(t, runSQL(t, namespace, "SELECT name, \"/spec/size\" AS size FROM "+resourceTable+" WHERE \"/spec/size\" >= 2;"))
	if len(rows) != 1 || rows[0]["name"] != "sample" || rows[0]["size"] != float64(2) {
		t.Fatalf("initial Custom Resource query: %#v", rows)
	}
	updated := runSQL(t, namespace, "UPDATE "+resourceTable+" SET \"/spec/size\" = 3 WHERE name = 'sample';")
	if updated.err != nil || decodeResult(t, updated)["affected_rows"] != float64(1) {
		t.Fatalf("update Custom Resource: %v\n%s", updated.err, updated.stdout)
	}
	updatedRows := decodeRows(t, runSQL(t, namespace, "SELECT \"/spec/message\" AS message FROM "+resourceTable+" WHERE name = 'sample';"))
	if len(updatedRows) != 1 || updatedRows[0]["message"] != "hello" {
		t.Fatalf("UPDATE changed an unrelated field: %#v", updatedRows)
	}
	extra := map[string]any{"apiVersion": group + "/v1", "kind": "Gadget", "metadata": map[string]any{"name": "extra", "namespace": namespace}, "spec": map[string]any{"size": 5}}
	if result := runSQL(t, namespace, insertManifestSQL(resourceName, extra)); result.err != nil {
		t.Fatalf("insert second Custom Resource: %v\nstderr:\n%s", result.err, result.stderr)
	}
	deleted := runSQL(t, namespace, "DELETE FROM "+resourceTable+" WHERE name = 'extra';")
	if deleted.err != nil || decodeResult(t, deleted)["affected_rows"] != float64(1) {
		t.Fatalf("delete Custom Resource: %v\n%s", deleted.err, deleted.stdout)
	}
	remainingRows := decodeRows(t, runSQL(t, namespace, "SELECT name FROM "+resourceTable+" WHERE name = 'extra';"))
	if len(remainingRows) != 0 {
		t.Fatalf("DELETE left the Custom Resource behind: %#v", remainingRows)
	}

	clusterCRDName := "clusternotes." + group
	t.Cleanup(func() {
		_ = runCommand(t, repositoryRoot, "kubectl", "delete", "clusternotes", "--all", "--ignore-not-found", "--wait=true")
		_ = runCommand(t, repositoryRoot, "kubectl", "delete", "crd", clusterCRDName, "--ignore-not-found", "--wait=true")
	})
	clusterCRD := map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1", "kind": "CustomResourceDefinition",
		"metadata": map[string]any{"name": clusterCRDName},
		"spec": map[string]any{
			"group": group, "scope": "Cluster",
			"names": map[string]any{"plural": "clusternotes", "singular": "clusternote", "kind": "ClusterNote"},
			"versions": []any{map[string]any{
				"name": "v1", "served": true, "storage": true,
				"schema": map[string]any{"openAPIV3Schema": map[string]any{
					"type": "object", "properties": map[string]any{"spec": map[string]any{"type": "object", "properties": map[string]any{
						"owner": map[string]any{"type": "string"},
					}}},
				}},
			}},
		},
	}
	if result := runSQL(t, namespace, insertManifestSQL("apiextensions.k8s.io/v1/customresourcedefinitions", clusterCRD)); result.err != nil {
		t.Fatalf("create cluster-scoped CRD: %v\nstderr:\n%s", result.err, result.stderr)
	}
	clusterResourceName := group + "/v1/clusternotes"
	clusterTable := "\"" + clusterResourceName + "\""
	clusterNote := map[string]any{"apiVersion": group + "/v1", "kind": "ClusterNote", "metadata": map[string]any{"name": "note"}, "spec": map[string]any{"owner": "platform"}}
	if result := runSQL(t, namespace, insertManifestSQL(clusterResourceName, clusterNote)); result.err != nil {
		t.Fatalf("create cluster-scoped Custom Resource: %v\nstderr:\n%s", result.err, result.stderr)
	}
	clusterRows := decodeRows(t, runSQL(t, namespace, "SELECT name, namespace, \"/spec/owner\" AS owner FROM "+clusterTable+" WHERE name = 'note';"))
	if len(clusterRows) != 1 || clusterRows[0]["name"] != "note" || clusterRows[0]["namespace"] != nil || clusterRows[0]["owner"] != "platform" {
		t.Fatalf("cluster-scoped query should ignore CLI namespace: %#v", clusterRows)
	}
}

func TestPodMetricsQueryAgainstMetricsServer(t *testing.T) {
	requireIntegration(t)
	buildBinary(t)
	probe := runCommand(t, repositoryRoot, "kubectl", "get", "--raw", "/apis/metrics.k8s.io/v1beta1")
	if probe.err != nil {
		t.Skip("Metrics API is unavailable; install or enable metrics-server to run this test")
	}
	namespace := fmt.Sprintf("sql-metrics-it-%d", time.Now().UnixNano())
	if result := runCommand(t, repositoryRoot, "kubectl", "create", "namespace", namespace); result.err != nil {
		t.Fatalf("create namespace: %v\n%s", result.err, result.stderr)
	}
	t.Cleanup(func() { cleanupNamespace(t, namespace) })
	applyManifest(t, fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: measure
  namespace: %s
spec:
  containers:
    - name: app
      image: busybox:1.36
      command: ["sh", "-c", "sleep 3600"]
      resources:
        requests: {cpu: 10m, memory: 16Mi}
    - name: sidecar
      image: busybox:1.36
      command: ["sh", "-c", "sleep 3600"]
      resources:
        requests: {cpu: 5m, memory: 8Mi}
`, namespace))
	if result := runCommand(t, repositoryRoot, "kubectl", "wait", "--for=condition=Ready", "pod/measure", "-n", namespace, "--timeout=180s"); result.err != nil {
		t.Fatalf("wait for metrics pod: %v\n%s", result.err, result.stderr)
	}

	deadline := time.Now().Add(180 * time.Second)
	for time.Now().Before(deadline) {
		result := runSQL(t, namespace, "SELECT name, cpu_millicores, memory_bytes FROM pod_metrics WHERE name = 'measure';")
		if result.err == nil {
			var rows []map[string]any
			if err := json.Unmarshal([]byte(result.stdout), &rows); err != nil {
				t.Fatalf("decode Metrics result: %v", err)
			}
			if len(rows) == 1 {
				cpu, cpuOK := rows[0]["cpu_millicores"].(float64)
				memory, memoryOK := rows[0]["memory_bytes"].(float64)
				if rows[0]["name"] != "measure" || !cpuOK || !memoryOK || cpu < 0 || memory < 0 {
					t.Fatalf("invalid Metrics row: %#v", rows[0])
				}
				return
			}
		} else if strings.Contains(result.stderr, "E_METRICS_UNAVAILABLE") {
			t.Fatalf("Metrics API became unavailable: %s", result.stderr)
		}
		time.Sleep(3 * time.Second)
	}
	t.Fatal("Pod Metrics sample did not appear within 180 seconds")
}

func insertManifestSQL(table string, manifest map[string]any) string {
	data, _ := json.Marshal(manifest)
	quoted := strings.ReplaceAll(string(data), "'", "''")
	return "INSERT INTO \"" + table + "\" (manifest) VALUES ('" + quoted + "');"
}
