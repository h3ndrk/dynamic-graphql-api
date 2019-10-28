package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	parse "github.com/h3ndrk/go-sqlite-createtable-parser"
)

func responsePathToString(path *graphql.ResponsePath) string {
	if path.Prev != nil {
		return fmt.Sprintf("%s.%v", responsePathToString(path.Prev), path.Key)
	}
	return fmt.Sprintf("%v", path.Key)
}

type reference struct {
	sourceColumn string
	targetTable  string
	targetColumn string
}

func getReferencesFromStmt(stmt *parse.Table) []reference {
	var references []reference

	for _, column := range stmt.Columns {
		if column.Name != nil && column.ForeignKey != nil && column.ForeignKey.Table != nil && len(column.ForeignKey.Columns) == 1 {
			references = append(references, reference{
				sourceColumn: *column.Name,
				targetTable:  *column.ForeignKey.Table,
				targetColumn: column.ForeignKey.Columns[0],
			})
		}
	}

	for _, constraint := range stmt.TableConstraints {
		if constraint.Type == parse.TableConstraintTypeForeignKey && len(constraint.ForeignKeyColumns) == 1 && constraint.ForeignKey != nil && constraint.ForeignKey.Table != nil && len(constraint.ForeignKey.Columns) == 1 {
			references = append(references, reference{
				sourceColumn: constraint.ForeignKeyColumns[0],
				targetTable:  *constraint.ForeignKey.Table,
				targetColumn: constraint.ForeignKey.Columns[0],
			})
		}
	}

	return references
}

type Column struct {
	name    string
	Type    string
	notNull bool
}

func getColumnsFromStmt(stmt *parse.Table) []Column {
	var columns []Column

	for _, column := range stmt.Columns {
		if column.Name != nil && column.Type != nil {
			columns = append(columns, Column{
				name:    *column.Name,
				Type:    *column.Type,
				notNull: column.NotNull,
			})
		}
	}

	return columns
}

func getTableNameFromStmt(stmt *parse.Table) (string, error) {
	if stmt.Name != nil {
		return *stmt.Name, nil
	}

	return "", errors.New("Missing table name")
}

func main() {
	stmt, err := parse.FromString(`CREATE TABLE a (
  id INTEGER PRIMARY KEY,
  iN INTEGER,
  sN TEXT,
  fN REAL,
  iNN INTEGER NOT NULL,
  sNN TEXT NOT NULL,
  fNN REAL NOT NULL,
  b_id INTEGER REFERENCES b(id)
);`)
	if err != nil {
		panic(err)
	}

	// fmt.Println(getReferencesFromStmt(stmt))
	// fmt.Println(getColumnsFromStmt(stmt))

	objFields := graphql.Fields{}
	for _, column := range getColumnsFromStmt(stmt) {
		var objFieldType graphql.Output
		switch column.Type {
		case "INTEGER":
			objFieldType = graphql.Int
		case "TEXT", "BLOB":
			objFieldType = graphql.String
		case "REAL", "NUMERIC":
			objFieldType = graphql.Float
		default:
			panic("unsupported type")
		}
		if column.notNull {
			objFieldType = graphql.NewNonNull(objFieldType)
		}
		objFields[column.name] = &graphql.Field{
			Type: objFieldType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				fmt.Println("resolve path:", responsePathToString(p.Info.Path))
				switch column.Type {
				case "INTEGER":
					return 42, nil
				case "TEXT", "BLOB":
					return "42", nil
				case "REAL", "NUMERIC":
					return 42.1337, nil
				}
				return nil, errors.New("unsupported type")
			},
		}
	}
	obj := graphql.NewObject(graphql.ObjectConfig{
		Name:   "A",
		Fields: objFields,
	})

	query := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			// "id": relay.GlobalIDField("SubstanceAllowance", nil),
			// "type": &graphql.Field{
			// 	Type:        graphql.NewNonNull(SubstanceAllowanceTypeGQL),
			// },
			"string": &graphql.Field{
				Type: graphql.NewNonNull(graphql.String),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return "world", nil
				},
			},
			"obj": &graphql.Field{
				Type: graphql.NewNonNull(obj),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					fmt.Printf("%+v\n", p.Info)
					return "world", nil
				},
			},
			// "name": &graphql.Field{
			// 	Type: graphql.NewNonNull(graphql.String),
			// },
			// "lowerQuantityThreshold": &graphql.Field{
			// 	Type: graphql.NewNonNull(graphql.Float),
			// },
			// "upperQuantityThreshold": &graphql.Field{
			// 	Type: graphql.NewNonNull(graphql.Float),
			// },
		},
		// Interfaces: []*graphql.Interface{
		// 	NodeGQL.NodeInterface,
		// },
	})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: query})
	if err != nil {
		panic(err)
	}

	h := handler.New(&handler.Config{
		Schema:     &schema,
		Pretty:     true,
		Playground: true,
	})

	http.Handle("/graphql", h)

	log.Fatal(http.ListenAndServe(":8080", nil))

	// runQuery := func(q string) {
	// 	r := graphql.Do(graphql.Params{Schema: schema, RequestString: q})
	// 	if len(r.Errors) > 0 {
	// 		panic(r.Errors)
	// 	}
	// 	rJSON, _ := json.Marshal(r)
	// 	fmt.Printf("%s \n", rJSON)
	// }

	// runQuery(`
	// 	{
	// 		obj {
	// 			id
	// 			test
	// 			b_id
	// 		}
	// 	}
	// `)

	// 	runQuery(`
	// 		query IntrospectionQuery {
	//   __schema {
	//     queryType {
	//       name
	//     }
	//     mutationType {
	//       name
	//     }
	//     subscriptionType {
	//       name
	//     }
	//     types {
	//       ...FullType
	//     }
	//     directives {
	//       name
	//       description
	//       locations
	//       args {
	//         ...InputValue
	//       }
	//     }
	//   }
	// }

	// fragment FullType on __Type {
	//   kind
	//   name
	//   description
	//   fields(includeDeprecated: true) {
	//     name
	//     description
	//     args {
	//       ...InputValue
	//     }
	//     type {
	//       ...TypeRef
	//     }
	//     isDeprecated
	//     deprecationReason
	//   }
	//   inputFields {
	//     ...InputValue
	//   }
	//   interfaces {
	//     ...TypeRef
	//   }
	//   enumValues(includeDeprecated: true) {
	//     name
	//     description
	//     isDeprecated
	//     deprecationReason
	//   }
	//   possibleTypes {
	//     ...TypeRef
	//   }
	// }

	// fragment InputValue on __InputValue {
	//   name
	//   description
	//   type {
	//     ...TypeRef
	//   }
	//   defaultValue
	// }

	// fragment TypeRef on __Type {
	//   kind
	//   name
	//   ofType {
	//     kind
	//     name
	//     ofType {
	//       kind
	//       name
	//       ofType {
	//         kind
	//         name
	//         ofType {
	//           kind
	//           name
	//           ofType {
	//             kind
	//             name
	//             ofType {
	//               kind
	//               name
	//               ofType {
	//                 kind
	//                 name
	//               }
	//             }
	//           }
	//         }
	//       }
	//     }
	//   }
	// }
	// 	`)
}
