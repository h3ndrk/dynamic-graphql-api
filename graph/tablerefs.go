package graph

import (
	"github.com/pkg/errors"
)

// FilterNodeType filters nodes by type.
func (n Nodes) FilterNodeType(t string) Nodes {
	return n.Filter(func(n *Node) bool {
		return n.HasAttrValue("type", t)
	})
}

// FilterTables filters nodes whether they are a table.
func (n Nodes) FilterTables() Nodes {
	return n.FilterNodeType("table")
}

// FilterColumns filters nodes whether they are a column.
func (n Nodes) FilterColumns() Nodes {
	return n.FilterNodeType("column")
}

func (n Nodes) filterHaveForeignKeyAttrs() Nodes {
	return n.Filter(func(n *Node) bool {
		return !n.HasAttrValue("foreignKeyTable", "") && !n.HasAttrValue("foreignKeyColumn", "")
	})
}

// FilterName filters nodes whether they have a given name.
func (n Nodes) FilterName(name string) Nodes {
	return n.Filter(func(n *Node) bool {
		return n.HasAttrValue("name", name)
	})
}

// FilterHasForeignKeys filters nodes that have foreign key references to other nodes.
func (n Nodes) FilterHasForeignKeys() Nodes {
	g := n.graph

	return n.Filter(func(n *Node) bool {
		// edge target node must have foreign key edges to other tables and columns
		hasTableEdge := g.Edges().FilterSource(n).Filter(func(e *Edge) bool {
			return e.HasAttrValue("type", "foreignKeyReferenceTable")
		}).Len() == 1
		hasColumnEdge := g.Edges().FilterSource(n).Filter(func(e *Edge) bool {
			return e.HasAttrValue("type", "foreignKeyReferenceColumn")
		}).Len() == 1

		return hasTableEdge && hasColumnEdge
	})
}

// FilterEdgeType filters edges by type.
func (e Edges) FilterEdgeType(t string) Edges {
	return e.Filter(func(e *Edge) bool {
		return e.HasAttrValue("type", t)
	})
}

// FilterColumnsByTable filters nodes that are columns of a given table.
// func (n Nodes) FilterColumnsByTable(table *Node) Nodes {
// 	return n.graph.Edges().FilterSource(table).FilterEdgeType("tableHasColumn").Targets()
// }

// AddForeignKeyReferences adds references from all foreign keys to their corresponding tables and columns.
func (g *Graph) AddForeignKeyReferences() error {
	// search all foreign key columns, for each foreign key column:
	//   search the table and column
	//   create reference edges
	var err error
	g.Nodes().FilterColumns().filterHaveForeignKeyAttrs().
		ForEach(func(column *Node) bool {
			referencedTable := g.Nodes().FilterTables().FilterName(column.GetAttrValueDefault("foreignKeyTable", "")).First()
			if referencedTable == nil {
				err = errors.Errorf("failed to find table referenced by foreign key %s.%s",
					column.GetAttrValueDefault("foreignKeyTable", ""),
					column.GetAttrValueDefault("foreignKeyColumn", ""))
				return false
			}

			referencedColumn := g.Edges().FilterSource(referencedTable).FilterEdgeType("tableHasColumn").Targets().FilterName(column.GetAttrValueDefault("foreignKeyColumn", "")).First()
			if referencedColumn == nil {
				err = errors.Errorf("failed to find column referenced by foreign key %s.%s",
					column.GetAttrValueDefault("foreignKeyTable", ""),
					column.GetAttrValueDefault("foreignKeyColumn", ""))
				return false
			}

			g.addEdge(column, referencedTable, map[string]string{
				"type": "foreignKeyReferenceTable",
			})
			g.addEdge(column, referencedColumn, map[string]string{
				"type": "foreignKeyReferenceColumn",
			})

			return true
		})

	return err
}

// MarkJoinTables adds references from all foreign keys to their corresponding tables and columns.
func (g *Graph) MarkJoinTables() error {
	// get all tables, for each table:
	//   get all outgoing edges
	//   if amount of edges != 2 -> table is not a join table
	//   if both edges are foreign key edges -> table is a join table
	g.Nodes().FilterTables().ForEach(func(table *Node) bool {
		table.Attrs["isJoinTable"] = "false"

		columns := g.Edges().FilterSource(table).FilterEdgeType("tableHasColumn").Targets()
		if columns.Len() == 2 && columns.FilterHasForeignKeys().Len() == 2 {
			table.Attrs["isJoinTable"] = "true"
		}

		return true
	})

	return nil
}
