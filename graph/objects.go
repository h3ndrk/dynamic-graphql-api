package graph

import (
	"github.com/iancoleman/strcase"
	"github.com/jinzhu/inflection"
)

func (g *Graph) addObjectDirectFields(table *Node, object *Node) error {
	g.Edges().FilterSource(table).FilterEdgeType("tableHasColumn").Targets().ForEach(func(column *Node) bool {
		foreignKeyTables := g.Edges().FilterSource(column).FilterEdgeType("foreignKeyReferenceTable")
		foreignKeyColumns := g.Edges().FilterSource(column).FilterEdgeType("foreignKeyReferenceColumn")
		if foreignKeyTables.Len() == 1 && foreignKeyColumns.Len() == 1 {
			// field that references other object
			foreignKeyTable := foreignKeyTables.Targets().First()
			foreignKeyColumn := foreignKeyColumns.Targets().First()

			field := g.addNode(map[string]string{
				"type":        "field",
				"name":        strcase.ToLowerCamel(inflection.Singular(foreignKeyTable.GetAttrValueDefault("name", ""))),
				"isReference": "true",
			})

			g.addEdge(field, foreignKeyTable, map[string]string{
				"type": "fieldReferencesTable",
			})
			g.addEdge(field, foreignKeyColumn, map[string]string{
				"type": "fieldReferencesColumn",
			})

			g.addEdge(object, field, map[string]string{
				"type": "objectHasField",
			})
			// one-to-x field, now determine the x (one of: one, many)
			// search for back-reference for one-to-one
			// foreignKeyTable := foreignKeyTables.Targets().First()
			// foreignKeyColumns := g.Edges().FilterSource(foreignKeyTable).FilterEdgeType("tableHasColumn").Targets().Filter(func(foreignColumn *Node) bool {
			// 	return g.Edges().FilterSource(foreignColumn).FilterTarget(table).FilterEdgeType("foreignKeyReferenceTable").Len() == 1
			// })
			// if foreignKeyColumns.Len() == 1 {
			// 	// found back-reference -> one-to-one

			// }

		} else {
			// scalar field
			valueType := "String"
			switch strcase.ToScreamingSnake(column.GetAttrValueDefault("valueType", "")) {
			case "INTEGER":
				valueType = "Int"
			case "TEXT", "BLOB":
				valueType = "String"
			case "REAL", "NUMERIC":
				valueType = "Float"
			}
			if column.GetAttrValueDefault("isNonNull", "false") == "true" {
				valueType += "!"
			}
			if column.GetAttrValueDefault("isPrimaryKey", "false") == "true" {
				valueType = "ID!"
			}

			field := g.addNode(map[string]string{
				"type":        "field",
				"name":        strcase.ToLowerCamel(column.GetAttrValueDefault("name", "")),
				"isReference": "false",
				"valueType":   valueType,
			})

			g.addEdge(object, field, map[string]string{
				"type": "objectHasField",
			})
		}

		return true
	})

	return nil
}

// AddObjects to graph.
func (g *Graph) AddObjects() error {
	// get all non-join tables, for each table:
	//   add object and edge to the corresponding table
	var err error
	g.Nodes().FilterTables().Filter(func(n *Node) bool {
		return !n.HasAttrValue("isJoinTable", "true")
	}).ForEach(func(table *Node) bool {
		object := g.addNode(map[string]string{
			"type": "object",
			"name": strcase.ToCamel(inflection.Singular(table.GetAttrValueDefault("name", ""))),
		})

		g.addEdge(object, table, map[string]string{
			"type": "objectHasTable",
		})

		err = g.addObjectDirectFields(table, object)
		if err != nil {
			return false
		}

		return true
	})

	return err
}
