// Package catalog defines the tables and columns exposed by the basic query stage.
package catalog

import (
	"fmt"
	"strings"

	"github.com/zyzzyh/kubesql/internal/ast"
)

// Column describes one public KubeSQL column.
type Column struct {
	Name string
}

// Table describes a supported Kubernetes resource table.
type Table struct {
	Name             string
	Namespaced       bool
	Columns          []Column
	SupportsAllNames bool
}

var tables = map[string]Table{
	"namespaces": {
		Name:       "namespaces",
		Namespaced: false,
		Columns:    []Column{{Name: "name"}},
	},
	"deployments": {
		Name:       "deployments",
		Namespaced: true,
		Columns:    []Column{{Name: "name"}, {Name: "namespace"}, {Name: "replicas"}},
	},
	"ingresses": {
		Name:       "ingresses",
		Namespaced: true,
		Columns:    []Column{{Name: "name"}, {Name: "namespace"}, {Name: "default_backend_service"}},
	},
}

// Resolve validates a SELECT target before any Kubernetes request is made.
func Resolve(statement *ast.SelectStatement) (Table, []string, error) {
	tableName := strings.ToLower(statement.Table)
	table, ok := tables[tableName]
	if !ok {
		return Table{}, nil, fmt.Errorf("E_SEMANTIC: unknown table %q", statement.Table)
	}

	if len(statement.Columns) == 1 {
		if _, ok := statement.Columns[0].(*ast.Star); ok {
			columns := make([]string, 0, len(table.Columns))
			for _, column := range table.Columns {
				columns = append(columns, column.Name)
			}
			return table, columns, nil
		}
	}

	columns := make([]string, 0, len(statement.Columns))
	for _, item := range statement.Columns {
		column, ok := item.(*ast.Column)
		if !ok {
			return Table{}, nil, fmt.Errorf("E_SEMANTIC: star cannot be combined with other columns")
		}
		name := strings.ToLower(column.Name)
		if !hasColumn(table, name) {
			return Table{}, nil, fmt.Errorf("E_SEMANTIC: unknown column %q for table %q", column.Name, table.Name)
		}
		columns = append(columns, name)
	}
	return table, columns, nil
}

func hasColumn(table Table, name string) bool {
	for _, column := range table.Columns {
		if column.Name == name {
			return true
		}
	}
	return false
}
