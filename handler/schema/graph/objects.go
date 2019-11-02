package graph

import (
	"github.com/iancoleman/strcase"
	"github.com/jinzhu/inflection"
	"github.com/pkg/errors"
)

// FilterObjects filters nodes whether they are a object.
func (n Nodes) FilterObjects() Nodes {
	return n.FilterNodeType("object")
}

// FilterFields filters nodes whether they are a field.
func (n Nodes) FilterFields() Nodes {
	return n.FilterNodeType("field")
}

func (g *Graph) addObjectDirectFields(table *Node, object *Node) error {
	var err error
	g.Edges().FilterSource(table).FilterEdgeType("tableHasColumn").Targets().ForEach(func(column *Node) bool {
		foreignKeyTables := g.Edges().FilterSource(column).FilterEdgeType("foreignKeyReferenceTable")
		foreignKeyColumns := g.Edges().FilterSource(column).FilterEdgeType("foreignKeyReferenceColumn")
		if foreignKeyTables.Len() == 1 && foreignKeyColumns.Len() == 1 {
			// field that references other object
			foreignKeyTable := foreignKeyTables.Targets().First()
			foreignKeyColumn := foreignKeyColumns.Targets().First()
			referencedObject := g.Edges().FilterTarget(foreignKeyTable).FilterEdgeType("objectHasTable").Targets().First()
			if referencedObject == nil {
				err = errors.Errorf("missing object for table %+v", foreignKeyTable.Attrs)
				return false
			}

			field := g.addNode(map[string]string{
				"type":          "field",
				"name":          inflection.Singular(strcase.ToLowerCamel(foreignKeyTable.GetAttrValueDefault("name", ""))),
				"referenceType": "forward",
				"isNonNull":     column.GetAttrValueDefault("isNonNull", "false"),
			})

			g.addEdge(field, foreignKeyTable, map[string]string{
				"type": "fieldReferencesTable",
			})
			g.addEdge(field, foreignKeyColumn, map[string]string{
				"type": "fieldReferencesColumn",
			})
			g.addEdge(field, referencedObject, map[string]string{
				"type": "fieldReferencesObject",
			})
			g.addEdge(field, table, map[string]string{
				"type": "fieldHasTable",
			})
			g.addEdge(field, column, map[string]string{
				"type": "fieldHasColumn",
			})

			g.addEdge(object, field, map[string]string{
				"type": "objectHasField",
			})
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
				"type":      "field",
				"name":      strcase.ToLowerCamel(column.GetAttrValueDefault("name", "")),
				"valueType": valueType,
			})

			g.addEdge(field, table, map[string]string{
				"type": "fieldHasTable",
			})
			g.addEdge(field, column, map[string]string{
				"type": "fieldHasColumn",
			})

			g.addEdge(object, field, map[string]string{
				"type": "objectHasField",
			})
		}

		return true
	})

	return err
}

func (g *Graph) addObjectBackReferenceFields() error {
	var err error
	g.Nodes().FilterFields().ForEach(func(field *Node) bool {
		fieldTable := g.Edges().FilterSource(field).FilterEdgeType("fieldHasTable").Targets().First()
		fieldColumn := g.Edges().FilterSource(field).FilterEdgeType("fieldHasColumn").Targets().First()
		fieldObject := g.Edges().FilterTarget(field).FilterEdgeType("objectHasField").Sources().First()
		referencedTable := g.Edges().FilterSource(field).FilterEdgeType("fieldReferencesTable").Targets().First()
		referencedColumn := g.Edges().FilterSource(field).FilterEdgeType("fieldReferencesColumn").Targets().First()
		if fieldTable != nil && fieldColumn != nil && referencedTable != nil && referencedColumn != nil {
			referencedObject := g.Edges().FilterTarget(referencedTable).FilterEdgeType("objectHasTable").Targets().First()
			if referencedObject == nil {
				err = errors.Errorf("missing object for table %+v", referencedTable.Attrs)
				return false
			}

			// check that field does not exist already
			fieldName := inflection.Plural(strcase.ToLowerCamel(field.GetAttrValueDefault("name", "") + "_" + fieldTable.GetAttrValueDefault("name", "")))
			if g.Nodes().FilterFields().FilterName(fieldName).Len() != 0 {
				err = errors.Errorf("field %s (attempting to create back-reference) already exists in object %+v", fieldName, referencedObject.Attrs)
				return false
			}

			field := g.addNode(map[string]string{
				"type":          "field",
				"name":          fieldName,
				"referenceType": "backward",
			})

			g.addEdge(field, fieldTable, map[string]string{
				"type": "fieldReferencesTable",
			})
			g.addEdge(field, fieldColumn, map[string]string{
				"type": "fieldReferencesColumn",
			})
			g.addEdge(field, fieldObject, map[string]string{
				"type": "fieldReferencesObject",
			})
			g.addEdge(field, referencedTable, map[string]string{
				"type": "fieldHasTable",
			})
			g.addEdge(field, referencedColumn, map[string]string{
				"type": "fieldHasColumn",
			})

			g.addEdge(referencedObject, field, map[string]string{
				"type": "objectHasField",
			})
		}

		return true
	})

	return err
}

func (g *Graph) addObjectJoinedReferenceFields() error {
	var err error
	g.Nodes().FilterTables().Filter(func(n *Node) bool {
		return n.HasAttrValue("isJoinTable", "true")
	}).ForEach(func(table *Node) bool {
		columns := g.Edges().FilterSource(table).FilterEdgeType("tableHasColumn").Targets().All()
		if len(columns) != 2 {
			err = errors.Errorf("wrong amount of columns in table %+v", table.Attrs)
			return false
		}

		referencedTables := map[*Node]*Node{
			columns[0]: g.Edges().FilterSource(columns[0]).FilterEdgeType("foreignKeyReferenceTable").Targets().First(),
			columns[1]: g.Edges().FilterSource(columns[1]).FilterEdgeType("foreignKeyReferenceTable").Targets().First(),
		}
		if referencedTables[columns[0]] == nil || referencedTables[columns[1]] == nil {
			err = errors.Errorf("failed to find referenced tables of table %+v", table.Attrs)
			return false
		}

		referencedColumns := map[*Node]*Node{
			columns[0]: g.Edges().FilterSource(columns[0]).FilterEdgeType("foreignKeyReferenceColumn").Targets().First(),
			columns[1]: g.Edges().FilterSource(columns[1]).FilterEdgeType("foreignKeyReferenceColumn").Targets().First(),
		}
		if referencedColumns[columns[0]] == nil || referencedColumns[columns[1]] == nil {
			err = errors.Errorf("failed to find referenced columns of table %+v", table.Attrs)
			return false
		}

		referencedObjects := map[*Node]*Node{
			columns[0]: g.Edges().FilterTarget(referencedTables[columns[0]]).FilterEdgeType("objectHasTable").Targets().First(),
			columns[1]: g.Edges().FilterTarget(referencedTables[columns[1]]).FilterEdgeType("objectHasTable").Targets().First(),
		}
		if referencedObjects[columns[0]] == nil || referencedObjects[columns[1]] == nil {
			err = errors.Errorf("failed to find referenced objects of table %+v", table.Attrs)
			return false
		}

		fields := map[*Node]*Node{
			columns[0]: g.addNode(map[string]string{
				"type":          "field",
				"name":          inflection.Plural(strcase.ToLowerCamel(columns[1].GetAttrValueDefault("name", ""))),
				"referenceType": "joined",
			}),
			columns[1]: g.addNode(map[string]string{
				"type":          "field",
				"name":          inflection.Plural(strcase.ToLowerCamel(columns[0].GetAttrValueDefault("name", ""))),
				"referenceType": "joined",
			}),
		}

		// edges to reference join table and columns
		g.addEdge(fields[columns[0]], table, map[string]string{
			"type": "fieldReferencesJoinTable",
		})
		g.addEdge(fields[columns[1]], table, map[string]string{
			"type": "fieldReferencesJoinTable",
		})
		g.addEdge(fields[columns[0]], columns[0], map[string]string{
			"type": "fieldReferencesOwnJoinColumn",
		})
		g.addEdge(fields[columns[1]], columns[1], map[string]string{
			"type": "fieldReferencesOwnJoinColumn",
		})
		g.addEdge(fields[columns[0]], columns[1], map[string]string{
			"type": "fieldReferencesForeignJoinColumn",
		})
		g.addEdge(fields[columns[1]], columns[0], map[string]string{
			"type": "fieldReferencesForeignJoinColumn",
		})

		// edges to reference own table and columns
		g.addEdge(fields[columns[0]], referencedTables[columns[0]], map[string]string{
			"type": "fieldReferencesOwnTable",
		})
		g.addEdge(fields[columns[1]], referencedTables[columns[1]], map[string]string{
			"type": "fieldReferencesOwnTable",
		})
		g.addEdge(fields[columns[0]], referencedColumns[columns[0]], map[string]string{
			"type": "fieldReferencesOwnColumn",
		})
		g.addEdge(fields[columns[1]], referencedColumns[columns[1]], map[string]string{
			"type": "fieldReferencesOwnColumn",
		})

		// edges to reference foreign table and columns
		g.addEdge(fields[columns[0]], referencedTables[columns[1]], map[string]string{
			"type": "fieldReferencesForeignTable",
		})
		g.addEdge(fields[columns[1]], referencedTables[columns[0]], map[string]string{
			"type": "fieldReferencesForeignTable",
		})
		g.addEdge(fields[columns[0]], referencedColumns[columns[1]], map[string]string{
			"type": "fieldReferencesForeignColumn",
		})
		g.addEdge(fields[columns[1]], referencedColumns[columns[0]], map[string]string{
			"type": "fieldReferencesForeignColumn",
		})

		// edges to reference others objects
		g.addEdge(fields[columns[0]], fields[columns[1]], map[string]string{
			"type": "fieldReferencesObject",
		})
		g.addEdge(fields[columns[1]], fields[columns[0]], map[string]string{
			"type": "fieldReferencesObject",
		})

		g.addEdge(referencedObjects[columns[0]], fields[columns[0]], map[string]string{
			"type": "objectHasField",
		})
		g.addEdge(referencedObjects[columns[1]], fields[columns[1]], map[string]string{
			"type": "objectHasField",
		})

		return true
	})

	return err
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
			"name": inflection.Singular(strcase.ToCamel(table.GetAttrValueDefault("name", ""))),
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
	if err != nil {
		return err
	}

	if err := g.addObjectBackReferenceFields(); err != nil {
		return err
	}

	return g.addObjectJoinedReferenceFields()
}
