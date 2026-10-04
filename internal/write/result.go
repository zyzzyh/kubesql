// Package write executes UPDATE and DELETE statements against Kubernetes.
package write

// Result summarizes a write operation. Errors are per-object failures.
type Result struct {
	AffectedRows int           `json:"affected_rows"`
	FailedRows   int           `json:"failed_rows,omitempty"`
	Errors       []ObjectError `json:"errors,omitempty"`
}

// ObjectError identifies a resource that could not be changed.
type ObjectError struct {
	Resource  string `json:"resource"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Reason    string `json:"reason"`
}

// HasFailures reports whether the caller should return a non-zero exit code.
func (r Result) HasFailures() bool { return r.FailedRows > 0 }
