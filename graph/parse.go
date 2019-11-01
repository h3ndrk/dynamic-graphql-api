package graph

import (
	parse "github.com/h3ndrk/go-sqlite-createtable-parser"
	"github.com/pkg/errors"
)

func (g *Graph) addStmtColumn(nodeTable *Node, column *parse.Column, tableConstraints []parse.TableConstraint) error {
	if column.Name == nil {
		return errors.New("unexpected nil column name")
	}
	if column.ForeignKey != nil && column.ForeignKey.Table == nil {
		return errors.New("unexpected nil foreign key table name")
	}
	if column.ForeignKey != nil && len(column.ForeignKey.Columns) != 1 {
		return errors.Errorf("unexpected foreign key column amount (expected: 1, actual: %d)", len(column.ForeignKey.Columns))
	}

	attrs := map[string]string{
		"type":             "column",
		"name":             *column.Name,
		"isNonNull":        "false",
		"valueType":        "",
		"isPrimaryKey":     "false",
		"foreignKeyTable":  "",
		"foreignKeyColumn": "",
	}

	if column.NotNull {
		attrs["isNonNull"] = "true"
	}

	if column.Type != nil {
		attrs["valueType"] = *column.Type
	}

	if column.PrimaryKey {
		attrs["isPrimaryKey"] = "true"
	}

	if column.ForeignKey != nil {
		attrs["foreignKeyTable"] = *column.ForeignKey.Table
		attrs["foreignKeyColumn"] = column.ForeignKey.Columns[0]
	}

	// check for primery key
	for _, constraint := range tableConstraints {
		if constraint.Type != parse.TableConstraintTypePrimaryKey {
			continue
		}
		if len(constraint.IndexedColumns) != 1 {
			continue
		}
		if constraint.IndexedColumns[0].Name == nil {
			continue
		}
		if *constraint.IndexedColumns[0].Name != *column.Name {
			continue
		}

		attrs["isPrimaryKey"] = "true"
		break
	}

	// check for foreign key
	for _, constraint := range tableConstraints {
		if constraint.Type != parse.TableConstraintTypeForeignKey {
			continue
		}
		if len(constraint.ForeignKeyColumns) != 1 {
			continue
		}
		if constraint.ForeignKeyColumns[0] != *column.Name {
			continue
		}
		if constraint.ForeignKey == nil {
			continue
		}
		if constraint.ForeignKey.Table == nil {
			continue
		}
		if len(constraint.ForeignKey.Columns) != 1 {
			continue
		}

		attrs["foreignKeyTable"] = *constraint.ForeignKey.Table
		attrs["foreignKeyColumn"] = constraint.ForeignKey.Columns[0]
		break
	}

	nodeColumn := g.AddNode(attrs)
	g.AddEdge(nodeTable, nodeColumn, map[string]string{
		"type": "tableHasColumn",
	})
	return nil
}

func (g *Graph) addStmtTable(table *parse.Table) error {
	if table.Name == nil {
		return errors.New("unexpected nil table name")
	}

	nodeTable := g.AddNode(map[string]string{
		"type": "table",
		"name": *table.Name,
	})

	for i, column := range table.Columns {
		if err := g.addStmtColumn(nodeTable, &column, table.TableConstraints); err != nil {
			return errors.Wrapf(err, "failed to add column %d", i)
		}
	}

	return nil
}

// AddStmts adds a slice of statements to a graph.
func (g *Graph) AddStmts(stmts []string) error {
	for _, stmt := range stmts {
		table, err := parse.FromString(stmt)
		if err != nil {
			return errors.Wrapf(err, "failed to parse statement '%s'", stmt)
		}

		if err := g.addStmtTable(table); err != nil {
			return errors.Wrap(err, "failed to add table")
		}
	}

	return nil
}
