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
			if err := validateExpression(table, statement.Where); err != nil {
				return Table{}, nil, err
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
	if err := validateExpression(table, statement.Where); err != nil {
		return Table{}, nil, err
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

func validateExpression(table Table, expression ast.Expression) error {
	if expression == nil {
		return nil
	}
	switch expression := expression.(type) {
	case *ast.ColumnReference:
		if !hasColumn(table, strings.ToLower(expression.Name)) {
			return fmt.Errorf("E_SEMANTIC: unknown column %q for table %q", expression.Name, table.Name)
		}
	case *ast.BinaryExpression:
		if err := validateExpression(table, expression.Left); err != nil {
			return err
		}
		return validateExpression(table, expression.Right)
	case *ast.UnaryExpression:
		return validateExpression(table, expression.Expression)
	case *ast.IsNullExpression:
		return validateExpression(table, expression.Expression)
	case *ast.Literal:
		return nil
	default:
		return fmt.Errorf("E_SEMANTIC: unsupported WHERE expression %T", expression)
	}
	return nil
}
