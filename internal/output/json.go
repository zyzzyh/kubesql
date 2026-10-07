// Package output writes the public JSON responses produced by KubeSQL.
package output

import (
	"encoding/json"
	"io"

	"github.com/zyzzyh/kubesql/internal/write"
)

// WriteRows writes query rows as a JSON array. Nil rows are normalized to [].
func WriteRows(w io.Writer, rows []map[string]any) error {
	// WriteRows emits SELECT results as one JSON array.
	if rows == nil {
		rows = make([]map[string]any, 0)
	}
	return encode(w, rows)
}

// WriteResult writes the stable write-operation response.
func WriteResult(w io.Writer, result write.Result) error {
	// WriteResult emits affected and failed write counts in JSON form.
	return encode(w, result)
}

func encode(w io.Writer, value any) error {
	// encode centralizes JSON encoder settings so all CLI outputs end with one
	// newline and preserve native numeric and null values.
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return err
	}
	return nil
}
