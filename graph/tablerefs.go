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
