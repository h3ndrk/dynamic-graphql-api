package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/h3ndrk/dynamic-graphql-api/associations"
	"github.com/iancoleman/strcase"
	_ "github.com/mattn/go-sqlite3"
)

func responsePathToString(path *graphql.ResponsePath) string {
	if path.Prev != nil {
		return fmt.Sprintf("%s.%v", responsePathToString(path.Prev), path.Key)
	}
	return fmt.Sprintf("%v", path.Key)
}

type key int

const (
	KeyDB key = iota
)

func getDBFromContext(ctx context.Context) (*sql.DB, error) {
	db, ok := ctx.Value(KeyDB).(*sql.DB)
	if !ok {
		return nil, errors.New("Missing DB in context")
	}

	return db, nil
}

func httpDBMiddleware(db *sql.DB, h *handler.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ContextHandler(context.WithValue(r.Context(), KeyDB, db), w, r)
	})
}

func main() {
	db, err := sql.Open("sqlite3", "test.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	sqlRows, err := db.Query(
		"SELECT sql FROM sqlite_master WHERE type = 'table'",
	)
	if err != nil {
		panic(err)
	}
	defer sqlRows.Close()

	var (
		sql  string
		sqls []string
	)
	for sqlRows.Next() {
		err := sqlRows.Scan(&sql)
		if err != nil {
			panic(err)
		}

		sqls = append(sqls, sql)
	}

	if err := sqlRows.Err(); err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", sqls)

	a, err := associations.Evaluate(sqls)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", a)

	node := graphql.NewInterface(graphql.InterfaceConfig{
		Name: "Node",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.NewNonNull(graphql.ID),
			},
		},
	})

	pageInfo := graphql.NewObject(graphql.ObjectConfig{
		Name:        "PageInfo",
		Description: "Information about pagination in a connection.",
		Fields: graphql.Fields{
			"hasNextPage": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.Boolean),
				Description: "When paginating forwards, are there more items?",
			},
			"hasPreviousPage": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.Boolean),
				Description: "When paginating backwards, are there more items?",
			},
			"startCursor": &graphql.Field{
				Type:        graphql.String,
				Description: "When paginating backwards, the cursor to continue.",
			},
			"endCursor": &graphql.Field{
				Type:        graphql.String,
				Description: "When paginating forwards, the cursor to continue.",
			},
		},
	})

	graphqlObjects := map[string]*graphql.Object{}
	graphqlEdges := map[string]*graphql.Object{}
	graphqlConnections := map[string]*graphql.Object{}
	// create objects, edges and connections first
	for _, obj := range a.Objects {
		graphqlObjects[obj.Name] = graphql.NewObject(graphql.ObjectConfig{
			Name:   obj.Name,
			Fields: graphql.Fields{},
			Interfaces: []*graphql.Interface{
				node,
			},
		})

		graphqlEdges[obj.Name] = graphql.NewObject(graphql.ObjectConfig{
			Name:        obj.Name + "Edge",
			Description: "An edge in a connection.",
			Fields: graphql.Fields{
				"node": &graphql.Field{
					Type:        graphql.NewNonNull(graphqlObjects[obj.Name]),
					Description: "The item at the end of the edge.",
				},
				"cursor": &graphql.Field{
					Type:        graphql.NewNonNull(graphql.String),
					Description: "A cursor for use in pagination.",
				},
			},
		})

		graphqlConnections[obj.Name] = graphql.NewObject(graphql.ObjectConfig{
			Name:        obj.Name + "Connection",
			Description: "A connection to a list of items.",
			Fields: graphql.Fields{
				"pageInfo": &graphql.Field{
					Type:        graphql.NewNonNull(pageInfo),
					Description: "Information to aid in pagination.",
				},
				"edges": &graphql.Field{
					Type:        graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphqlEdges[obj.Name]))),
					Description: "Information to aid in pagination.",
				},
			},
		})
	}

	// add fields last to break circular dependencies
	for _, obj := range a.Objects {
		currentObj := obj // fix closure
		for _, field := range currentObj.Fields {
			currentField := field // fix closure

			var objFieldType graphql.Output
			fieldName := currentField.Name
			switch currentField.AssociationType {
			case associations.Identification:
				objFieldType = graphql.NewNonNull(graphql.ID)
			case associations.Scalar:
				switch currentField.Association {
				case "INTEGER":
					objFieldType = graphql.Int
				case "TEXT", "BLOB":
					objFieldType = graphql.String
				case "REAL", "NUMERIC":
					objFieldType = graphql.Float
				default:
					panic("unsupported type")
				}
			case associations.OneToOne, associations.OneToMany:
				objFieldType = graphqlObjects[currentField.Association]
				fieldName = strcase.ToSnake(currentField.Association)
			case associations.ManyToOne, associations.ManyToMany:
				objFieldType = graphql.NewNonNull(graphqlConnections[currentField.Association])
				fieldName = strcase.ToSnake(currentField.Association) + "s"
			default:
				panic("unsupported type")
			}

			if currentField.NonNull {
				objFieldType = graphql.NewNonNull(objFieldType)
			}

			graphqlObjects[currentObj.Name].AddFieldConfig(fieldName, &graphql.Field{
				Type: objFieldType,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					fmt.Printf("resolve field: %s\n", responsePathToString(p.Info.Path))
					fmt.Printf("association type: %s\n", currentField.AssociationType)
					switch currentField.AssociationType {
					case associations.Identification:
						fmt.Printf("Returning ID ...\n")
						return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s.%s:%d", currentObj.Name, currentField.Name, 42))), nil
					// case associations.Scalar:
					// 	switch currentField.Association {
					// 	case "INTEGER":
					// 		objFieldType = graphql.Int
					// 	case "TEXT", "BLOB":
					// 		objFieldType = graphql.String
					// 	case "REAL", "NUMERIC":
					// 		objFieldType = graphql.Float
					// 	default:
					// 		panic("unsupported type")
					// 	}
					// case associations.OneToOne, associations.OneToMany:
					// 	objFieldType = graphqlObjects[currentField.Association]
					// case associations.ManyToOne, associations.ManyToMany:
					// 	objFieldType = graphql.NewList(graphql.NewNonNull(graphqlObjects[currentField.Association]))
					default:
						fmt.Printf("Returning null ...\n")
						return nil, nil
					}
				},
			})
		}
	}

	query := graphql.NewObject(graphql.ObjectConfig{
		Name:   "Query",
		Fields: graphql.Fields{},
	})
	for name, _ := range graphqlObjects {
		query.AddFieldConfig(strcase.ToSnake(name)+"s", &graphql.Field{
			Type: graphql.NewNonNull(graphqlConnections[name]),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return 42, nil
			},
		})
	}
	// TODO: query.AddFieldConfig("node", ...) for Node retrieval

	// objFields := graphql.Fields{}
	// for _, column := range getColumnsFromStmt(stmt) {
	// 	var objFieldType graphql.Output
	// 	switch column.Type {
	// 	case "INTEGER":
	// 		objFieldType = graphql.Int
	// 	case "TEXT", "BLOB":
	// 		objFieldType = graphql.String
	// 	case "REAL", "NUMERIC":
	// 		objFieldType = graphql.Float
	// 	default:
	// 		panic("unsupported type")
	// 	}
	// 	if column.notNull {
	// 		objFieldType = graphql.NewNonNull(objFieldType)
	// 	}
	// 	objFields[column.name] = &graphql.Field{
	// 		Type: objFieldType,
	// 		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
	// 			fmt.Println("resolve path:", responsePathToString(p.Info.Path))
	// 			switch column.Type {
	// 			case "INTEGER":
	// 				return 42, nil
	// 			case "TEXT", "BLOB":
	// 				return "42", nil
	// 			case "REAL", "NUMERIC":
	// 				return 42.1337, nil
	// 			}
	// 			return nil, errors.New("unsupported type")
	// 		},
	// 	}
	// }
	// obj := graphql.NewObject(graphql.ObjectConfig{
	// 	Name:   "A",
	// 	Fields: objFields,
	// })

	// query := graphql.NewObject(graphql.ObjectConfig{
	// 	Name: "Query",
	// 	Fields: graphql.Fields{
	// 		// "id": relay.GlobalIDField("SubstanceAllowance", nil),
	// 		// "type": &graphql.Field{
	// 		// 	Type:        graphql.NewNonNull(SubstanceAllowanceTypeGQL),
	// 		// },
	// 		"string": &graphql.Field{
	// 			Type: graphql.NewNonNull(graphql.String),
	// 			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
	// 				return "world", nil
	// 			},
	// 		},
	// 		"obj": &graphql.Field{
	// 			Type: graphql.NewNonNull(obj),
	// 			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
	// 				fmt.Printf("%+v\n", p.Info)
	// 				return "world", nil
	// 			},
	// 		},
	// 		// "name": &graphql.Field{
	// 		// 	Type: graphql.NewNonNull(graphql.String),
	// 		// },
	// 		// "lowerQuantityThreshold": &graphql.Field{
	// 		// 	Type: graphql.NewNonNull(graphql.Float),
	// 		// },
	// 		// "upperQuantityThreshold": &graphql.Field{
	// 		// 	Type: graphql.NewNonNull(graphql.Float),
	// 		// },
	// 	},
	// 	// Interfaces: []*graphql.Interface{
	// 	// 	NodeGQL.NodeInterface,
	// 	// },
	// })
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: query})
	if err != nil {
		panic(err)
	}

	h := handler.New(&handler.Config{
		Schema:     &schema,
		Pretty:     true,
		Playground: true,
	})

	http.Handle("/graphql", httpDBMiddleware(db, h))

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
