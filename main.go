package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/h3ndrk/dynamic-graphql-api/associations"
	"github.com/iancoleman/strcase"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
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

type connection struct {
	// PageInfo
	hasNextPage     bool
	hasPreviousPage bool
	startCursor     string
	endCursor       string

	// Edges
	edges []uint
}

func max(a, b uint) uint {
	if a < b {
		return b
	}

	return a
}

func ternaryMax(a, b, c uint) uint {
	return max(max(a, b), c)
}

func min(a, b uint) uint {
	if a > b {
		return b
	}

	return a
}

// getRowsByArgs queries the given database table with pagination and ordering and returns the resulting rows and page-info data (in this order: has previous page, has next page). 'table' and 'columns' need to be escaped because they are directly inserted into the query string.
func getRowsWithPagination(
	ctx context.Context,
	db *sql.DB,
	before, after, first, last *uint,
	query string, args ...interface{},
) (*sql.Rows, bool, bool, error) {
	if first != nil && last != nil {
		// reset last
		last = nil
	}

	// count rows in table
	var count uint
	if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT count(*) FROM (%s)", query)).Scan(&count); err != nil {
		return nil, false, false, errors.Wrap(err, "database error (count)")
	}

	// get before and after positions (one-based)
	var (
		positionBefore uint
		positionAfter  uint
	)
	if before != nil || after != nil {
		var (
			whereExprs  []string
			whereValues []interface{}
		)
		if before != nil {
			whereExprs = append(whereExprs, "id = ?")
			whereValues = append(whereValues, *before)
		}
		if after != nil {
			whereExprs = append(whereExprs, "id = ?")
			whereValues = append(whereValues, *after)
		}
		positionRows, err := db.QueryContext(ctx, fmt.Sprintf(
			"SELECT * FROM (SELECT id, row_number() OVER () AS row_id FROM (%s)) WHERE %s",
			query, strings.Join(whereExprs, " OR "),
		), whereValues...)
		if err != nil {
			return nil, false, false, errors.Wrap(err, "database error (positions with where)")
		}
		defer positionRows.Close()

		var (
			id    uint
			rowID uint
		)
		for positionRows.Next() {
			err := positionRows.Scan(&id, &rowID)
			if err != nil {
				return nil, false, false, errors.Wrap(err, "database error (positions scan)")
			}

			if before != nil && id == *before {
				positionBefore = rowID
			}
			if after != nil && id == *after {
				positionAfter = rowID
			}
		}

		if err := positionRows.Err(); err != nil {
			return nil, false, false, errors.Wrap(err, "database error (positions error)")
		}
	}

	// calculate range (one-based)
	var (
		begin uint = 1         // inclusive
		end   uint = count + 1 // exclusive
	)
	if before != nil {
		end = positionBefore
	}
	if after != nil {
		begin = positionAfter + 1
	}
	if first != nil {
		end = min(begin+*first, count+1)
	}
	if last != nil {
		begin = ternaryMax(1, begin, end-*last)
	}

	// query range
	rows, err := db.QueryContext(ctx, fmt.Sprintf(
		`SELECT * FROM (
		SELECT
			*, row_number() OVER () AS __row_id
		FROM (%s)
	) WHERE __row_id >= ? AND __row_id < ?`, query,
	), append(args, &begin, &end)...)
	if err != nil {
		return nil, false, false, errors.Wrap(err, "database error (rows)")
	}
	// v := reflect.ValueOf(out)
	// if v.Elem().Kind() == reflect.Struct {
	// 	rows, err := db.QueryContext(ctx, fmt.Sprintf(
	// 		`SELECT * FROM (
	// 		SELECT
	// 			*, row_number() OVER () AS __row_id
	// 		FROM (%s)
	// 	) WHERE __row_id >= ? AND __row_id < ?`, query,
	// 	), &begin, &end)
	// 	if err != nil {
	// 		return false, false, errors.Wrap(err, "database error (rows)")
	// 	}
	// 	if !rows.Next() {
	// 		return false, false, errors.New("database error: empty result")
	// 	}
	// 	rows.Columns()
	// 	columnNameMap := getColumnNameMap(v.Elem())
	// 	for i := 0; i < v.Elem().NumField(); i++ {

	// 		v.Elem().Field(i).Interface(
	// 		)
	// 		v.Elem().
	// 	}
	// 	var fields []reflect.StructField
	// for i := 0; i < ts.NumField(); i++ {
	// 	_, ok := ts.Field(i).Tag.Lookup("db")
	// 	if !ok {
	// 		continue
	// 	}
	// 	fields = append(fields, ts.Field(i))
	// }
	// }

	return rows, begin > 1, end < count+1, nil
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
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					connection, ok := p.Source.(connection)
					if !ok {
						return nil, errors.New("Malformed source")
					}
					return connection.hasNextPage, nil
				},
			},
			"hasPreviousPage": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.Boolean),
				Description: "When paginating backwards, are there more items?",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					connection, ok := p.Source.(connection)
					if !ok {
						return nil, errors.New("Malformed source")
					}
					return connection.hasPreviousPage, nil
				},
			},
			"startCursor": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.String),
				Description: "When paginating backwards, the cursor to continue.",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					connection, ok := p.Source.(connection)
					if !ok {
						return nil, errors.New("Malformed source")
					}
					return connection.startCursor, nil
				},
			},
			"endCursor": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.String),
				Description: "When paginating forwards, the cursor to continue.",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					connection, ok := p.Source.(connection)
					if !ok {
						return nil, errors.New("Malformed source")
					}
					return connection.endCursor, nil
				},
			},
		},
	})

	graphqlObjects := map[string]*graphql.Object{}
	graphqlEdges := map[string]*graphql.Object{}
	graphqlConnections := map[string]*graphql.Object{}
	// create objects, edges and connections first
	for _, obj := range a.Objects {
		currentObj := obj
		graphqlObjects[currentObj.Name] = graphql.NewObject(graphql.ObjectConfig{
			Name:   currentObj.Name,
			Fields: graphql.Fields{},
			Interfaces: []*graphql.Interface{
				node,
			},
		})

		graphqlEdges[currentObj.Name] = graphql.NewObject(graphql.ObjectConfig{
			Name:        currentObj.Name + "Edge",
			Description: "An edge in a connection.",
			Fields: graphql.Fields{
				"node": &graphql.Field{
					Type:        graphql.NewNonNull(graphqlObjects[currentObj.Name]),
					Description: "The item at the end of the edge.",
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return p.Source, nil
					},
				},
				"cursor": &graphql.Field{
					Type:        graphql.NewNonNull(graphql.String),
					Description: "A cursor for use in pagination.",
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						id, ok := p.Source.(uint)
						if !ok {
							return nil, errors.New("Malformed source")
						}

						return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%d", currentObj.Name, id))), nil
					},
				},
			},
		})

		// TODO: introduce connection struct with fields pageInfo and edges (ids)
		// TODO: use that informations in this connection
		// TODO: add pagination query code to query and build connection struct
		// TODO: later add input arguments (before, after, first, last)
		graphqlConnections[currentObj.Name] = graphql.NewObject(graphql.ObjectConfig{
			Name:        currentObj.Name + "Connection",
			Description: "A connection to a list of items.",
			Fields: graphql.Fields{
				"pageInfo": &graphql.Field{
					Type:        graphql.NewNonNull(pageInfo),
					Description: "Information to aid in pagination.",
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return p.Source, nil
					},
				},
				"edges": &graphql.Field{
					Type:        graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphqlEdges[currentObj.Name]))),
					Description: "Information to aid in pagination.",
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						connection, ok := p.Source.(connection)
						if !ok {
							return nil, errors.New("Malformed source")
						}

						return connection.edges, nil
					},
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
					id, ok := p.Source.(uint)
					if !ok {
						return nil, errors.New("Malformed source")
					}
					fmt.Printf("%s.%s.p.Source: %+v\n", currentObj.Name, fieldName, p.Source)
					fmt.Printf("resolve field: %s\n", responsePathToString(p.Info.Path))
					fmt.Printf("association type: %s\n", currentField.AssociationType)
					switch currentField.AssociationType {
					case associations.Identification:
						fmt.Printf("Returning ID ...\n")
						return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%d", currentObj.Name, id))), nil
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
				db, err := getDBFromContext(p.Context)
				if err != nil {
					return nil, err
				}

				var before *uint
				// if args.Before != "" {
				// 	id, err := substanceCursorToID(substanceCursor(args.Before))
				// 	if err != nil {
				// 		return nil, err
				// 	}
				// 	before = &id
				// }
				var after *uint
				// if args.After != "" {
				// 	id, err := substanceCursorToID(substanceCursor(args.After))
				// 	if err != nil {
				// 		return nil, err
				// 	}
				// 	after = &id
				// }
				var first *uint
				// if args.First != -1 {
				// 	firstUint := uint(args.First)
				// 	first = &firstUint
				// }
				var last *uint
				// if args.Last != -1 {
				// 	lastUint := uint(args.Last)
				// 	last = &lastUint
				// }
				rows, hasPreviousPage, hasNextPage, err := getRowsWithPagination(
					p.Context, db, before, after, first, last,
					"SELECT id FROM "+name)
				if err != nil {
					return nil, err
				}
				defer rows.Close()

				var (
					id    uint
					rowID uint
					conn  connection
				)
				for rows.Next() {
					if err := rows.Scan(&id, &rowID); err != nil {
						return nil, err
					}

					conn.edges = append(conn.edges, id)
				}

				if err := rows.Err(); err != nil {
					return nil, err
				}

				if len(conn.edges) > 0 {
					conn.startCursor = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%d", name, conn.edges[0])))
				}

				if len(conn.edges) > 0 {
					conn.endCursor = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%d", name, conn.edges[len(conn.edges)-1])))
				}

				conn.hasPreviousPage = hasPreviousPage
				conn.hasNextPage = hasNextPage

				return conn, nil
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
