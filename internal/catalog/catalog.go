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
	Dynamic          bool
}

// Projection maps a resource field to the key returned in one result row.
// Source and Output are separate because SQL aliases rename output only.
type Projection struct {
	Source string
	Output string
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

// IsKnownTable reports whether a statement targets a typed built-in table.
func IsKnownTable(statement *ast.SelectStatement) bool {
	name := statement.Table
	if !statement.TableQuoted {
		name = strings.ToLower(name)
	}
	_, ok := tables[name]
	return ok
}

// ResolveDynamic creates projections for a resource whose schema is discovered
// at runtime. Field validation is deferred until unstructured objects arrive.
func ResolveDynamic(statement *ast.SelectStatement) (Table, []Projection, error) {
	table := Table{Name: statement.Table, Dynamic: true}
	if len(statement.Columns) == 1 {
		if _, ok := statement.Columns[0].(*ast.Star); ok {
			return table, []Projection{{Source: "*", Output: "*"}}, nil
		}
	}
	columns := make([]Projection, 0, len(statement.Columns))
	for _, item := range statement.Columns {
		column, ok := item.(*ast.Column)
		if !ok {
			return Table{}, nil, fmt.Errorf("E_SEMANTIC: star cannot be combined with other columns")
		}
		name := column.Name
		if !column.Quoted {
			name = strings.ToLower(name)
		}
		output := column.Alias
		if output == "" {
			output = name
		}
		columns = append(columns, Projection{Source: name, Output: output})
	}
	return table, columns, nil
}

// Resolve validates a SELECT target before any Kubernetes request is made.
func Resolve(statement *ast.SelectStatement) (Table, []Projection, error) {
	tableName := statement.Table
	if !statement.TableQuoted {
		tableName = strings.ToLower(tableName)
	}
	table, ok := tables[tableName]
	if !ok {
		return Table{}, nil, fmt.Errorf("E_SEMANTIC: unknown table %q", statement.Table)
	}

	if len(statement.Columns) == 1 {
		if _, ok := statement.Columns[0].(*ast.Star); ok {
			columns := make([]Projection, 0, len(table.Columns))
			for _, column := range table.Columns {
				columns = append(columns, Projection{Source: column.Name, Output: column.Name})
			}
			if err := validateExpression(table, statement.Where); err != nil {
				return Table{}, nil, err
			}
			return table, columns, nil
		}
	}

	columns := make([]Projection, 0, len(statement.Columns))
	for _, item := range statement.Columns {
		column, ok := item.(*ast.Column)
		if !ok {
			return Table{}, nil, fmt.Errorf("E_SEMANTIC: star cannot be combined with other columns")
		}
		name := column.Name
		if !column.Quoted {
			name = strings.ToLower(name)
		}
		if !hasColumn(table, name) {
			return Table{}, nil, fmt.Errorf("E_SEMANTIC: unknown column %q for table %q", column.Name, table.Name)
		}
		output := column.Alias
		if output == "" {
			output = name
		}
		columns = append(columns, Projection{Source: name, Output: output})
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
