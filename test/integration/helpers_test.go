package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	repositoryRoot string
	ksqlBinary     string
)

type commandResult struct {
	stdout string
	stderr string
	err    error
}

func init() {
	_, filename, _, _ := runtime.Caller(0)
	repositoryRoot = filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func integrationEnabled() bool {
	return os.Getenv("KUBESQL_INTEGRATION") == "1"
}

func buildBinary(t *testing.T) string {
	t.Helper()
	if ksqlBinary != "" {
		return ksqlBinary
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	path := filepath.Join(os.TempDir(), "kubesql-integration-"+strconv.Itoa(os.Getpid())+suffix)
	result := runCommand(t, repositoryRoot, "go", "build", "-o", path, "./cmd/ksql")
	if result.err != nil {
		t.Fatalf("build ksql: %v\nstdout:\n%s\nstderr:\n%s", result.err, result.stdout, result.stderr)
	}
	ksqlBinary = path
	return path
}

func requireIntegration(t *testing.T) {
	t.Helper()
	if !integrationEnabled() {
		t.Skip("set KUBESQL_INTEGRATION=1 to run Kubernetes integration tests")
	}
	result := runCommand(t, repositoryRoot, "kubectl", "cluster-info")
	if result.err != nil {
		t.Skipf("Kubernetes cluster is unavailable: %v\n%s", result.err, result.stderr)
	}
}

func runCommand(t *testing.T, directory, name string, args ...string) commandResult {
	t.Helper()
	return runCommandWithTimeout(t, 60*time.Second, directory, name, args...)
}

func runCommandWithTimeout(t *testing.T, timeout time.Duration, directory, name string, args ...string) commandResult {
	t.Helper()
	commandContext, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(commandContext, name, args...)
	command.Dir = directory
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if commandContext.Err() != nil {
		err = fmt.Errorf("%s timed out after %s: %w", name, timeout, commandContext.Err())
	}
	return commandResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func applyFixture(t *testing.T, namespace string) {
	t.Helper()
	fixturePath := filepath.Join(repositoryRoot, "fixtures", "integration-base.yaml")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	manifest := strings.ReplaceAll(string(data), "${NAMESPACE}", namespace)
	applyManifest(t, manifest)
}

func applyManifest(t *testing.T, manifest string) {
	t.Helper()
	commandContext, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(commandContext, "kubectl", "apply", "-f", "-")
	command.Dir = repositoryRoot
	command.Stdin = strings.NewReader(manifest)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("apply fixture: %v\nstderr:\n%s", err, stderr.String())
	}
}

func cleanupNamespace(t *testing.T, namespace string) {
	t.Helper()
	result := runCommandWithTimeout(t, 150*time.Second, repositoryRoot, "kubectl", "delete", "namespace", namespace, "--ignore-not-found", "--wait=true", "--timeout=120s")
	if result.err != nil {
		t.Logf("cleanup namespace %s failed: %v\nstderr:\n%s", namespace, result.err, result.stderr)
	}
}

func newFixture(t *testing.T) string {
	t.Helper()
	namespace := fmt.Sprintf("sql-it-%d", time.Now().UnixNano())
	applyFixture(t, namespace)
	t.Cleanup(func() { cleanupNamespace(t, namespace) })
	return namespace
}

func runSQL(t *testing.T, namespace, sql string) commandResult {
	t.Helper()
	command := exec.Command(ksqlBinary, "--namespace", namespace, "--output", "json")
	command.Dir = repositoryRoot
	command.Stdin = strings.NewReader(sql)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return commandResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func decodeRows(t *testing.T, result commandResult) []map[string]any {
	t.Helper()
	if result.err != nil {
		t.Fatalf("ksql failed: %v\nstderr:\n%s\nstdout:\n%s", result.err, result.stderr, result.stdout)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(result.stdout), &rows); err != nil {
		t.Fatalf("decode query output: %v\noutput:\n%s", err, result.stdout)
	}
	return rows
}

func decodeResult(t *testing.T, result commandResult) map[string]any {
	t.Helper()
	if result.stdout == "" {
		t.Fatalf("expected JSON result, stderr:\n%s", result.stderr)
	}
	var output map[string]any
	if err := json.Unmarshal([]byte(result.stdout), &output); err != nil {
		t.Fatalf("decode write output: %v\noutput:\n%s", err, result.stdout)
	}
	return output
}

func kubectlJSON(t *testing.T, args ...string) map[string]any {
	t.Helper()
	result := runCommand(t, repositoryRoot, "kubectl", append(args, "-o", "json")...)
	if result.err != nil {
		t.Fatalf("kubectl %v: %v\nstderr:\n%s", args, result.err, result.stderr)
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(result.stdout), &object); err != nil {
		t.Fatalf("decode kubectl output: %v\noutput:\n%s", err, result.stdout)
	}
	return object
}

func waitForDeletion(t *testing.T, args ...string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		result := runCommand(t, repositoryRoot, "kubectl", append([]string{"get"}, args...)...)
		if result.err != nil && strings.Contains(strings.ToLower(result.stderr), "not found") {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("resource was not deleted within 60 seconds: kubectl get %v", args)
}
