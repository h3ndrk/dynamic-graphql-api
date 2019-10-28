package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/h3ndrk/dynamic-graphql-sqlite/associations"
)

func responsePathToString(path *graphql.ResponsePath) string {
	if path.Prev != nil {
		return fmt.Sprintf("%s.%v", responsePathToString(path.Prev), path.Key)
	}
	return fmt.Sprintf("%v", path.Key)
}

func main() {
	a, err := associations.Evaluate([]string{
		"CREATE TABLE as (id INTEGER PRIMARY KEY, b_id INTEGER REFERENCES bs(id));",
		"CREATE TABLE bs (id INTEGER PRIMARY KEY, a_id INTEGER REFERENCES as(id));",
	})
	if err != nil {
		panic(err)
	}

	graphqlObjects := map[string]*graphql.Object{}
	// create objects first
	for _, obj := range a.Objects {
		graphqlObjects[obj.Name] = graphql.NewObject(graphql.ObjectConfig{
			Name:   obj.Name,
			Fields: graphql.Fields{},
		})
	}

	// add fields second to break circular dependencies
	for _, obj := range a.Objects {
		currentObj := obj
		for _, field := range currentObj.Fields {
			currentField := field // fix closure
			var objFieldType graphql.Output
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
			case associations.ManyToOne, associations.ManyToMany:
				objFieldType = graphql.NewList(graphql.NewNonNull(graphqlObjects[currentField.Association]))
			default:
				panic("unsupported type")
			}

			if currentField.NonNull {
				objFieldType = graphql.NewNonNull(objFieldType)
			}

			graphqlObjects[currentObj.Name].AddFieldConfig(currentField.Name, &graphql.Field{
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
	for name, obj := range graphqlObjects {
		query.AddFieldConfig(name, &graphql.Field{
			Type: obj,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return 42, nil
			},
		})
	}

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
