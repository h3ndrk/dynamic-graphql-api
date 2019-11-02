package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/h3ndrk/dynamic-graphql-api/associations"
	"github.com/h3ndrk/dynamic-graphql-api/graph"
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

type nodeCursor struct {
	object string
	id     uint
}

func parseNodeCursor(cursor string) (nodeCursor, error) {
	bytesCursor, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return nodeCursor{}, errors.Errorf("Invalid cursor '%s'", cursor)
	}

	stringsID := strings.SplitN(string(bytesCursor), ":", 2)
	if len(stringsID) != 2 {
		return nodeCursor{}, errors.Errorf("Invalid cursor '%s'", cursor)
	}

	int64ID, err := strconv.ParseInt(stringsID[1], 10, 0)
	if err != nil {
		return nodeCursor{}, errors.Wrapf(err, "Invalid cursor '%s'", cursor)
	}

	return nodeCursor{object: stringsID[0], id: uint(int64ID)}, nil
}

func (n nodeCursor) String() string {
	return fmt.Sprintf("%s:%d", n.object, n.id)
}

func (n nodeCursor) OpaqueString() string {
	return base64.StdEncoding.EncodeToString([]byte(n.String()))
}

type mutationPayload struct {
	clientMutationID string
	cursor           nodeCursor
}

type connection struct {
	// PageInfo
	hasNextPage     bool
	hasPreviousPage bool
	startCursor     string
	endCursor       string

	// Edges
	edges []nodeCursor
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
	if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT count(*) FROM (%s)", query), args...).Scan(&count); err != nil {
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
		), append(args, whereValues...)...)
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

	return rows, begin > 1, end < count+1, nil
}

func extractIDFromCursor(cursor, objType string) (uint, error) {
	bytesCursor, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return 0, errors.Errorf("Invalid cursor '%s'", cursor)
	}

	stringID := strings.TrimPrefix(string(bytesCursor), "SubstanceAllowance:")
	if stringID == cursor {
		return 0, errors.Errorf("Invalid cursor '%s'", cursor)
	}

	uintID, err := strconv.ParseInt(stringID, 10, 0)
	if err != nil {
		return 0, errors.Wrapf(err, "Invalid cursor '%s'", cursor)
	}

	return uint(uintID), nil
}

func getConnectionArgs(p graphql.ResolveParams, objType string) (*uint, *uint, *uint, *uint, error) {
	var (
		before *uint
		after  *uint
		first  *uint
		last   *uint
	)

	if beforeCursor, ok := p.Args["before"]; ok {
		id, err := extractIDFromCursor(fmt.Sprintf("%v", beforeCursor), objType)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		before = &id
	}

	if afterCursor, ok := p.Args["after"]; ok {
		id, err := extractIDFromCursor(fmt.Sprintf("%v", afterCursor), objType)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		after = &id
	}

	if firstValue, ok := p.Args["first"]; ok {
		if firstValue, ok := firstValue.(int); ok {
			firstValueUint := uint(firstValue)
			first = &firstValueUint
		} else {
			return nil, nil, nil, nil, errors.Errorf("Invalid value first '%v'", firstValue)
		}
	}

	if lastValue, ok := p.Args["last"]; ok {
		if lastValue, ok := lastValue.(int); ok {
			lastValueUint := uint(lastValue)
			last = &lastValueUint
		} else {
			return nil, nil, nil, nil, errors.Errorf("Invalid value last '%v'", lastValue)
		}
	}

	return before, after, first, last, nil
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
		sqlString string
		sqls      []string
	)
	for sqlRows.Next() {
		err := sqlRows.Scan(&sqlString)
		if err != nil {
			panic(err)
		}

		sqls = append(sqls, sqlString)
	}

	if err := sqlRows.Err(); err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", sqls)

	objectGraph, err := graph.NewGraph(sqls)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s\n", objectGraph)

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
						cursor, ok := p.Source.(nodeCursor)
						if !ok {
							return nil, errors.New("Malformed source")
						}

						return cursor.OpaqueString(), nil
					},
				},
			},
		})

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
			var objFieldArgs graphql.FieldConfigArgument

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
				fieldName = strcase.ToLowerCamel(currentField.Association)
			case associations.ManyToOne, associations.ManyToMany:
				objFieldType = graphql.NewNonNull(graphqlConnections[currentField.Association])
				fieldName = strcase.ToLowerCamel(currentField.Association) + "s"
				objFieldArgs = graphql.FieldConfigArgument{
					"before": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"after": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"first": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"last": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				}
			default:
				panic("unsupported type")
			}

			if currentField.NonNull {
				objFieldType = graphql.NewNonNull(objFieldType)
			}

			fmt.Printf("Defining field '%s' for object '%s' (type: %+v) ...\n", fieldName, currentObj.Name, objFieldType)

			graphqlObjects[currentObj.Name].AddFieldConfig(fieldName, &graphql.Field{
				Type: objFieldType,
				Args: objFieldArgs,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					cursor, ok := p.Source.(nodeCursor)
					if !ok {
						return nil, errors.New("Malformed source")
					}

					switch currentField.AssociationType {
					case associations.Identification:
						fmt.Printf("%s ( %s ) -> %s (%s)\n", responsePathToString(p.Info.Path), currentField.AssociationType, cursor.OpaqueString(), cursor.String())
						return cursor.OpaqueString(), nil
					case associations.Scalar:
						switch currentField.Association {
						case "INTEGER":
							db, err := getDBFromContext(p.Context)
							if err != nil {
								return nil, err
							}

							var (
								value sql.NullInt64
							)
							if err := db.QueryRow("SELECT "+fieldName+" FROM "+currentObj.Name+" WHERE id = ?", cursor.id).Scan(&value); err != nil {
								return nil, err
							}

							if !value.Valid {
								fmt.Printf("%s ( %s ) -> null\n", responsePathToString(p.Info.Path), currentField.AssociationType)
								return nil, nil
							}

							fmt.Printf("%s ( %s ) -> %d\n", responsePathToString(p.Info.Path), currentField.AssociationType, value.Int64)
							return value.Int64, nil
						case "TEXT", "BLOB":
							db, err := getDBFromContext(p.Context)
							if err != nil {
								return nil, err
							}

							var (
								value sql.NullString
							)
							if err := db.QueryRow("SELECT "+fieldName+" FROM "+currentObj.Name+" WHERE id = ?", cursor.id).Scan(&value); err != nil {
								return nil, err
							}

							if !value.Valid {
								fmt.Printf("%s ( %s ) -> null\n", responsePathToString(p.Info.Path), currentField.AssociationType)
								return nil, nil
							}

							fmt.Printf("%s ( %s ) -> %s\n", responsePathToString(p.Info.Path), currentField.AssociationType, value.String)
							return value.String, nil
						case "REAL", "NUMERIC":
							db, err := getDBFromContext(p.Context)
							if err != nil {
								return nil, err
							}

							var (
								value sql.NullFloat64
							)
							if err := db.QueryRow("SELECT "+fieldName+" FROM "+currentObj.Name+" WHERE id = ?", cursor.id).Scan(&value); err != nil {
								return nil, err
							}

							if !value.Valid {
								fmt.Printf("%s ( %s ) -> null\n", responsePathToString(p.Info.Path), currentField.AssociationType)
								return nil, nil
							}

							fmt.Printf("%s ( %s ) -> %f\n", responsePathToString(p.Info.Path), currentField.AssociationType, value.Float64)
							return value.Float64, nil
						default:
							panic("unsupported type")
						}
					case associations.OneToOne, associations.OneToMany:
						db, err := getDBFromContext(p.Context)
						if err != nil {
							return nil, err
						}

						var (
							association sql.NullInt64
						)
						if err := db.QueryRow("SELECT "+currentField.Name+" FROM "+currentObj.Name+" WHERE id = ?", cursor.id).Scan(&association); err != nil {
							return nil, err
						}

						if !association.Valid {
							fmt.Printf("%s ( %s ) -> null\n", responsePathToString(p.Info.Path), currentField.AssociationType)
							return nil, nil
						}

						fmt.Printf("%s ( %s ) -> %d\n", responsePathToString(p.Info.Path), currentField.AssociationType, association.Int64)
						return nodeCursor{object: currentObj.Name, id: uint(association.Int64)}, nil
					case associations.ManyToOne:
						db, err := getDBFromContext(p.Context)
						if err != nil {
							return nil, err
						}

						before, after, first, last, err := getConnectionArgs(p, currentObj.Name)
						if err != nil {
							return nil, err
						}

						if currentField.ForeignField == nil {
							return nil, errors.New("Malformed association (foreign field)")
						}
						rows, hasPreviousPage, hasNextPage, err := getRowsWithPagination(
							p.Context, db, before, after, first, last,
							"SELECT id FROM "+currentField.Association+" WHERE "+*currentField.ForeignField+" = ?", cursor.id)
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

							conn.edges = append(conn.edges, nodeCursor{object: currentField.Association, id: id})
						}

						if err := rows.Err(); err != nil {
							return nil, err
						}

						if len(conn.edges) > 0 {
							conn.startCursor = conn.edges[0].OpaqueString()
						}

						if len(conn.edges) > 0 {
							conn.endCursor = conn.edges[len(conn.edges)-1].OpaqueString()
						}

						conn.hasPreviousPage = hasPreviousPage
						conn.hasNextPage = hasNextPage

						return conn, nil
					case associations.ManyToMany:
						db, err := getDBFromContext(p.Context)
						if err != nil {
							return nil, err
						}

						before, after, first, last, err := getConnectionArgs(p, currentObj.Name)
						if err != nil {
							return nil, err
						}

						if currentField.JoinTable == nil {
							return nil, errors.New("Malformed association (join table)")
						}
						if currentField.JoinForeignField == nil {
							return nil, errors.New("Malformed association (join foreign field)")
						}
						if currentField.JoinOwnField == nil {
							return nil, errors.New("Malformed association (join own field)")
						}
						rows, hasPreviousPage, hasNextPage, err := getRowsWithPagination(
							p.Context, db, before, after, first, last,
							"SELECT "+*currentField.JoinForeignField+" FROM "+*currentField.JoinTable+" WHERE "+*currentField.JoinTable+"."+*currentField.JoinOwnField+" = ?", cursor.id)
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

							conn.edges = append(conn.edges, nodeCursor{object: currentField.Association, id: id})
						}

						if err := rows.Err(); err != nil {
							return nil, err
						}

						if len(conn.edges) > 0 {
							conn.startCursor = conn.edges[0].OpaqueString()
						}

						if len(conn.edges) > 0 {
							conn.endCursor = conn.edges[len(conn.edges)-1].OpaqueString()
						}

						conn.hasPreviousPage = hasPreviousPage
						conn.hasNextPage = hasNextPage

						return conn, nil
					default:
						fmt.Printf("%s ( %s ) -> null (default)\n", responsePathToString(p.Info.Path), currentField.AssociationType)
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
	for name := range graphqlObjects {
		query.AddFieldConfig(strcase.ToLowerCamel(name)+"s", &graphql.Field{
			Type: graphql.NewNonNull(graphqlConnections[name]),
			Args: graphql.FieldConfigArgument{
				"before": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"after": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"first": &graphql.ArgumentConfig{
					Type: graphql.Int,
				},
				"last": &graphql.ArgumentConfig{
					Type: graphql.Int,
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				db, err := getDBFromContext(p.Context)
				if err != nil {
					return nil, err
				}

				before, after, first, last, err := getConnectionArgs(p, name)
				if err != nil {
					return nil, err
				}

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

					conn.edges = append(conn.edges, nodeCursor{object: name, id: id})
				}

				if err := rows.Err(); err != nil {
					return nil, err
				}

				if len(conn.edges) > 0 {
					conn.startCursor = conn.edges[0].OpaqueString()
				}

				if len(conn.edges) > 0 {
					conn.endCursor = conn.edges[len(conn.edges)-1].OpaqueString()
				}

				conn.hasPreviousPage = hasPreviousPage
				conn.hasNextPage = hasNextPage

				return conn, nil
			},
		})
	}

	node.ResolveType = func(p graphql.ResolveTypeParams) *graphql.Object {
		cursor, ok := p.Value.(nodeCursor)
		if !ok {
			return nil
		}

		for name := range graphqlObjects {
			if name == cursor.object {
				return graphqlObjects[name]
			}
		}

		return nil
	}

	query.AddFieldConfig("node", &graphql.Field{
		Type: node,
		Args: graphql.FieldConfigArgument{
			"id": &graphql.ArgumentConfig{
				Type:        graphql.NewNonNull(graphql.ID),
				Description: "The ID of an object",
			},
		},
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			cursorInterface, ok := p.Args["id"]
			if !ok {
				return nil, errors.New("Missing id cursor")
			}
			cursor, ok := cursorInterface.(string)
			if !ok {
				return nil, errors.New("Malformed id cursor")
			}

			bytesCursor, err := base64.StdEncoding.DecodeString(cursor)
			if err != nil {
				return nil, errors.Errorf("Invalid cursor '%s'", cursor)
			}

			stringsID := strings.SplitN(string(bytesCursor), ":", 2)
			if len(stringsID) != 2 {
				return nil, errors.Errorf("Invalid cursor '%s'", cursor)
			}

			uintID, err := strconv.ParseInt(stringsID[1], 10, 0)
			if err != nil {
				return nil, errors.Wrapf(err, "Invalid cursor '%s'", cursor)
			}

			return nodeCursor{object: stringsID[0], id: uint(uintID)}, nil
		},
	})

	mutation := graphql.NewObject(graphql.ObjectConfig{
		Name:   "Mutation",
		Fields: graphql.Fields{},
	})
	for _, obj := range a.Objects {
		currentObj := obj // fix closure

		inputFields := graphql.InputObjectConfigFieldMap{}
		// inputUpdateFields := graphql.InputObjectConfigFieldMap{}
		for _, field := range currentObj.Fields {
			currentField := field // fix closure

			var fieldType graphql.Output
			fieldName := currentField.Name

			switch currentField.AssociationType {
			case associations.Identification:
				// fieldType = graphql.NewNonNull(graphql.ID)
				continue
			case associations.Scalar:
				switch currentField.Association {
				case "INTEGER":
					fieldType = graphql.Int
				case "TEXT", "BLOB":
					fieldType = graphql.String
				case "REAL", "NUMERIC":
					fieldType = graphql.Float
				default:
					panic("unsupported type")
				}
			case associations.OneToOne, associations.OneToMany:
				fieldType = graphql.ID // graphqlObjects[currentField.Association]
				fieldName = strcase.ToLowerCamel(currentField.Association + "_id")
			case associations.ManyToOne, associations.ManyToMany:
				// fieldType = graphql.NewNonNull(graphql.ID) // graphql.NewNonNull(graphqlConnections[currentField.Association])
				// fieldName = strcase.ToLowerCamel(currentField.Association + "_id") + "s"
				continue
			default:
				panic("unsupported type")
			}

			if currentField.NonNull {
				fieldType = graphql.NewNonNull(fieldType)
			}

			inputFields[fieldName] = &graphql.InputObjectFieldConfig{
				Type: fieldType,
			}
		}

		inputFields["clientMutationId"] = &graphql.InputObjectFieldConfig{
			Type: graphql.NewNonNull(graphql.String),
		}

		input := graphql.NewInputObject(graphql.InputObjectConfig{
			Name:   currentObj.Name + "Input",
			Fields: inputFields,
		})

		payload := graphql.NewObject(graphql.ObjectConfig{
			Name: currentObj.Name + "Payload",
			Fields: graphql.Fields{
				"clientMutationId": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						payload, ok := p.Source.(mutationPayload)
						if !ok {
							return nil, errors.New("Malformed source")
						}

						return payload.clientMutationID, nil
					},
				},
				strcase.ToLowerCamel(currentObj.Name): &graphql.Field{
					Type: graphql.NewNonNull(graphqlObjects[currentObj.Name]),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						payload, ok := p.Source.(mutationPayload)
						if !ok {
							return nil, errors.New("Malformed source")
						}

						return payload.cursor, nil
					},
				},
			},
		})

		mutation.AddFieldConfig(strcase.ToLowerCamel("create_"+currentObj.Name), &graphql.Field{
			Type: graphql.NewNonNull(payload),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(input),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				// db, err := getDBFromContext(p.Context)
				// if err != nil {
				// 	return nil, err
				// }

				inputInterface, ok := p.Args["input"]
				if !ok {
					return nil, errors.New("Missing input")
				}
				input, ok := inputInterface.(map[string]interface{})
				if !ok {
					return nil, errors.New("Malformed input")
				}

				// INSERT INTO ...

				// before, after, first, last, err := getConnectionArgs(p, name)
				// if err != nil {
				// 	return nil, err
				// }

				// rows, hasPreviousPage, hasNextPage, err := getRowsWithPagination(
				// 	p.Context, db, before, after, first, last,
				// 	"SELECT id FROM "+name)
				// if err != nil {
				// 	return nil, err
				// }
				// defer rows.Close()

				// var (
				// 	id    uint
				// 	rowID uint
				// 	conn  connection
				// )
				// for rows.Next() {
				// 	if err := rows.Scan(&id, &rowID); err != nil {
				// 		return nil, err
				// 	}

				// 	conn.edges = append(conn.edges, nodeCursor{object: name, id: id})
				// }

				// if err := rows.Err(); err != nil {
				// 	return nil, err
				// }

				// if len(conn.edges) > 0 {
				// 	conn.startCursor = conn.edges[0].OpaqueString()
				// }

				// if len(conn.edges) > 0 {
				// 	conn.endCursor = conn.edges[len(conn.edges)-1].OpaqueString()
				// }

				// conn.hasPreviousPage = hasPreviousPage
				// conn.hasNextPage = hasNextPage

				var payload mutationPayload
				payload.cursor = nodeCursor{object: currentObj.Name, id: 1}

				if clientMutationID, ok := input["clientMutationId"]; ok {
					if clientMutationID, ok := clientMutationID.(string); ok {
						payload.clientMutationID = clientMutationID
					}
				}

				return payload, nil
			},
		})
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: query, Mutation: mutation})
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
}
