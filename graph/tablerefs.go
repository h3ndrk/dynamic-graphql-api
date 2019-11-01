package graph

import (
	"github.com/pkg/errors"
)

// AddForeignKeyReferences adds references from all foreign keys to their corresponding tables and columns.
func (g *Graph) AddForeignKeyReferences() error {
	// search all foreign key columns
	foreignKeyColumns := g.NodesByFilter(func(n *Node) bool {
		return n.HasAttrValue("type", "column") && !n.HasAttrValue("foreignKeyTable", "") && !n.HasAttrValue("foreignKeyColumn", "")
	})

	// for each foreign key column:
	//   search the table and column
	//   create reference edges
	for _, foreignforeignKeyColumn := range foreignKeyColumns {
		referencedTable, err := g.NodeByFilter(func(n *Node) bool {
			return n.HasAttrValue("type", "table") && n.HasAttrValue("name", foreignforeignKeyColumn.GetAttrValueDefault("foreignKeyTable", ""))
		})
		if err != nil {
			return errors.Wrapf(err, "failed to find table referenced by foreign key %s.%s",
				foreignforeignKeyColumn.GetAttrValueDefault("foreignKeyTable", ""),
				foreignforeignKeyColumn.GetAttrValueDefault("foreignKeyColumn", ""))
		}
		referencedColumns := g.EdgesByFromWithFilter(referencedTable, func(e *Edge) bool {
			return e.To.HasAttrValue("type", "column") && e.To.HasAttrValue("name", foreignforeignKeyColumn.GetAttrValueDefault("foreignKeyColumn", ""))
		})
		if len(referencedColumns) != 1 {
			return errors.Errorf("failed to find column referenced by foreign key %s.%s",
				foreignforeignKeyColumn.GetAttrValueDefault("foreignKeyTable", ""),
				foreignforeignKeyColumn.GetAttrValueDefault("foreignKeyColumn", ""))
		}

		g.AddEdge(foreignforeignKeyColumn, referencedTable, map[string]string{
			"type": "foreignKeyReferenceTable",
		})
		g.AddEdge(foreignforeignKeyColumn, referencedColumns[0].To, map[string]string{
			"type": "foreignKeyReferenceColumn",
		})
	}

	return nil
}

// MarkJoinTables adds references from all foreign keys to their corresponding tables and columns.
func (g *Graph) MarkJoinTables() error {
	// get all tables
	tables := g.NodesByFilter(func(n *Node) bool {
		return n.HasAttrValue("type", "table")
	})

	// for each table:
	//   get all outgoing edges
	//   if amount of edges != 2 -> table is not a join table
	//   if both edges are foreign key edges -> table is a join table
	for _, table := range tables {
		edges := g.EdgesByFromWithFilter(table, func(e *Edge) bool {
			// only match edges with type "tableHasColumn"
			if !e.HasAttrValue("type", "tableHasColumn") {
				return false
			}

			// edge target node must have foreign key edges to other tables and columns
			foreignKeyTableEdges := g.EdgesByFromWithFilter(e.To, func(e *Edge) bool {
				return e.HasAttrValue("type", "foreignKeyReferenceTable")
			})
			foreignKeyColumnEdges := g.EdgesByFromWithFilter(e.To, func(e *Edge) bool {
				return e.HasAttrValue("type", "foreignKeyReferenceColumn")
			})

			return len(foreignKeyTableEdges) == 1 && len(foreignKeyColumnEdges) == 1
		})
		if len(edges) == 2 {
			table.Attrs["isJoinTable"] = "true"
		} else {
			table.Attrs["isJoinTable"] = "false"
		}
	}

	return nil
}
