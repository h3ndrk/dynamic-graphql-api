package schema

import (
	"dynamic-graphql-api/handler/schema/db"
	"dynamic-graphql-api/handler/schema/graph"
	"fmt"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/pkg/errors"
)

var (
	graphqlObjects     = map[string]*graphql.Object{}
	graphqlEdges       = map[string]*graphql.Object{}
	graphqlConnections = map[string]*graphql.Object{}
)

func createObjects(g *graph.Graph) {
	g.Nodes().FilterObjects().ForEach(func(obj *graph.Node) bool {
		objName := obj.GetAttrValueDefault("name", "")

		fmt.Printf("Adding object %s ...\n", objName)

		graphqlObjects[objName] = graphql.NewObject(graphql.ObjectConfig{
			Name:   objName,
			Fields: graphql.Fields{},
			Interfaces: []*graphql.Interface{
				node,
			},
		})

		graphqlEdges[objName] = graphql.NewObject(graphql.ObjectConfig{
			Name:        objName + "Edge",
			Description: "An edge in a connection.",
			Fields: graphql.Fields{
				"node": &graphql.Field{
					Type:        graphql.NewNonNull(graphqlObjects[objName]),
					Description: "The item at the end of the edge.",
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return p.Source, nil
					},
				},
				"cursor": &graphql.Field{
					Type:        graphql.NewNonNull(graphql.ID),
					Description: "A cursor for use in pagination.",
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						c, ok := p.Source.(cursor)
						if !ok {
							return nil, errors.New("malformed source")
						}

						return c.OpaqueString(), nil
					},
				},
			},
		})

		graphqlConnections[objName] = graphql.NewObject(graphql.ObjectConfig{
			Name:        objName + "Connection",
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
					Type:        graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphqlEdges[objName]))),
					Description: "The edges to the objects.",
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						connection, ok := p.Source.(connection)
						if !ok {
							return nil, errors.New("malformed source")
						}

						return connection.edges, nil
					},
				},
			},
		})

		return true
	})
}

func getGraphQLTypeFromField(g *graph.Graph, field *graph.Node) (graphql.Output, graphql.FieldConfigArgument, error) {
	if field.HasAttrKey("valueType") {
		valueType := field.GetAttrValueDefault("valueType", "")
		valueTypeWithoutNonNull := strings.TrimSuffix(valueType, "!")
		isNonNull := valueType != valueTypeWithoutNonNull

		var graphqlType graphql.Output
		switch valueTypeWithoutNonNull {
		case "Int":
			graphqlType = graphql.Int
		case "Float":
			graphqlType = graphql.Float
		case "String":
			graphqlType = graphql.String
		case "Boolean":
			graphqlType = graphql.Boolean
		case "ID":
			graphqlType = graphql.ID
		case "DateTime":
			graphqlType = graphql.DateTime
		default:
			return nil, nil, errors.Errorf("unsupported type %s", valueType)
		}

		if isNonNull {
			graphqlType = graphql.NewNonNull(graphqlType)
		}

		return graphqlType, graphql.FieldConfigArgument{}, nil
	} else if field.HasAttrKey("referenceType") {
		referencedObject := g.Edges().FilterSource(field).FilterEdgeType("fieldReferencesObject").Targets().First()
		if referencedObject == nil {
			return nil, nil, errors.Errorf("field %+v does not reference object", field.Attrs)
		}
		referencedObjectName := referencedObject.GetAttrValueDefault("name", "")

		var graphqlType graphql.Output
		graphqlArgs := graphql.FieldConfigArgument{}
		switch field.GetAttrValueDefault("referenceType", "") {
		case "forward":
			graphqlType = graphqlObjects[referencedObjectName]
		case "backward", "joined":
			graphqlType = graphql.NewNonNull(graphqlConnections[referencedObjectName])
			graphqlArgs = connectionArgs
		default:
			return nil, nil, errors.Errorf("unsupported reference type of field %+v", field.Attrs)
		}

		if field.GetAttrValueDefault("isNonNull", "false") == "true" {
			graphqlType = graphql.NewNonNull(graphqlType)
		}

		return graphqlType, graphqlArgs, nil
	}

	return nil, nil, errors.Errorf("unknown type of field %+v", field.Attrs)
}

func addFields(g *graph.Graph) error {
	var err error
	g.Nodes().FilterObjects().ForEach(func(obj *graph.Node) bool {
		objName := obj.GetAttrValueDefault("name", "")

		g.Edges().FilterSource(obj).FilterEdgeType("objectHasField").Targets().ForEach(func(field *graph.Node) bool {
			fieldName := field.GetAttrValueDefault("name", "")

			fieldType, fieldArgs, errTemp := getGraphQLTypeFromField(g, field)
			if errTemp != nil {
				err = errTemp
				return false
			}

			referencedTable := g.Edges().FilterSource(field).FilterEdgeType("fieldHasTable").Targets().First()
			if field.HasAttrKey("valueType") && referencedTable == nil {
				err = errors.Errorf("referenced table not found while resolving field %s.%s:%s", objName, fieldName, fieldType.Name())
				return false
			}
			referencedColumn := g.Edges().FilterSource(field).FilterEdgeType("fieldHasColumn").Targets().First()
			if field.HasAttrKey("valueType") && referencedColumn == nil {
				err = errors.Errorf("referenced column not found while resolving field %s.%s:%s", objName, fieldName, fieldType.Name())
				return false
			}
			foreignTable := g.Edges().FilterSource(field).FilterEdgeType("fieldReferencesTable").Targets().First()
			if field.GetAttrValueDefault("referenceType", "") == "backward" && foreignTable == nil {
				err = errors.Errorf("referenced foreign table not found while resolving field %s.%s:%s", objName, fieldName, fieldType.Name())
				return false
			}
			foreignColumn := g.Edges().FilterSource(field).FilterEdgeType("fieldReferencesColumn").Targets().First()
			if field.GetAttrValueDefault("referenceType", "") == "backward" && foreignColumn == nil {
				err = errors.Errorf("referenced foreign column not found while resolving field %s.%s:%s", objName, fieldName, fieldType.Name())
				return false
			}
			joinTable := g.Edges().FilterSource(field).FilterEdgeType("fieldReferencesJoinTable").Targets().First()
			if field.GetAttrValueDefault("referenceType", "") == "joined" && joinTable == nil {
				err = errors.Errorf("referenced join table not found while resolving field %s.%s:%s", objName, fieldName, fieldType.Name())
				return false
			}
			joinOwnColumn := g.Edges().FilterSource(field).FilterEdgeType("fieldReferencesOwnJoinColumn").Targets().First()
			if field.GetAttrValueDefault("referenceType", "") == "joined" && joinOwnColumn == nil {
				err = errors.Errorf("referenced own join column not found while resolving field %s.%s:%s", objName, fieldName, fieldType.Name())
				return false
			}
			joinForeignColumn := g.Edges().FilterSource(field).FilterEdgeType("fieldReferencesForeignJoinColumn").Targets().First()
			if field.GetAttrValueDefault("referenceType", "") == "joined" && joinForeignColumn == nil {
				err = errors.Errorf("referenced foreign join column not found while resolving field %s.%s:%s", objName, fieldName, fieldType.Name())
				return false
			}

			fmt.Printf("Adding field %s.%s:%s ...\n", objName, fieldName, fieldType.Name())

			graphqlObjects[objName].AddFieldConfig(fieldName, &graphql.Field{
				Type: fieldType,
				Args: fieldArgs,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					c, ok := p.Source.(cursor)
					if !ok {
						return nil, errors.New("Malformed source")
					}

					dbFromContext, err := getDBFromContext(p.Context)
					if err != nil {
						return nil, err
					}

					switch fieldType.Name() {
					case "Int", "Int!":
						return db.ScalarIntQuery(db.ScalarRequest{
							Ctx: p.Context,
							DB:  dbFromContext,

							Table:  referencedTable.GetAttrValueDefault("name", ""),
							Column: referencedColumn.GetAttrValueDefault("name", ""),

							ID: c.id,
						})
					case "Float", "Float!":
						return db.ScalarFloatQuery(db.ScalarRequest{
							Ctx: p.Context,
							DB:  dbFromContext,

							Table:  referencedTable.GetAttrValueDefault("name", ""),
							Column: referencedColumn.GetAttrValueDefault("name", ""),

							ID: c.id,
						})
					case "String", "String!":
						return db.ScalarStringQuery(db.ScalarRequest{
							Ctx: p.Context,
							DB:  dbFromContext,

							Table:  referencedTable.GetAttrValueDefault("name", ""),
							Column: referencedColumn.GetAttrValueDefault("name", ""),

							ID: c.id,
						})
					case "Boolean", "Boolean!":
						return db.ScalarBooleanQuery(db.ScalarRequest{
							Ctx: p.Context,
							DB:  dbFromContext,

							Table:  referencedTable.GetAttrValueDefault("name", ""),
							Column: referencedColumn.GetAttrValueDefault("name", ""),

							ID: c.id,
						})
					case "ID", "ID!":
						return c.OpaqueString(), nil
					case "DateTime", "DateTime!":
						return db.ScalarDateTimeQuery(db.ScalarRequest{
							Ctx: p.Context,
							DB:  dbFromContext,

							Table:  referencedTable.GetAttrValueDefault("name", ""),
							Column: referencedColumn.GetAttrValueDefault("name", ""),

							ID: c.id,
						})
					}

					if field.GetAttrValueDefault("referenceType", "") == "forward" {
						id, err := db.ScalarIntQuery(db.ScalarRequest{
							Ctx: p.Context,
							DB:  dbFromContext,

							Table:  referencedTable.GetAttrValueDefault("name", ""),
							Column: referencedColumn.GetAttrValueDefault("name", ""),

							ID: c.id,
						})
						if err != nil {
							return nil, err
						}
						if id, ok := id.(int64); ok {
							return cursor{object: objName, id: uint(id)}, nil
						}

						return nil, nil
					}

					if field.GetAttrValueDefault("referenceType", "") == "backward" {
						result := db.PaginationQuery(db.PaginationRequest{
							Ctx: p.Context,
							DB:  dbFromContext,

							Metadata: db.PaginationRequestBackwardMetadata{
								ForeignTable:           foreignTable.GetAttrValueDefault("name", ""),
								ForeignReferenceColumn: foreignColumn.GetAttrValueDefault("name", ""),
								ForeignReturnColumn:    "id",
								OwnReferenceColumn:     c.id,
							},

							// TODO: arguments
						})
						if result.Err != nil {
							return nil, result.Err
						}

						var conn connection
						for _, id := range result.IDs {
							conn.edges = append(conn.edges, cursor{object: objName, id: id})
						}

						if len(conn.edges) > 0 {
							conn.startCursor = conn.edges[0]
						}

						if len(conn.edges) > 0 {
							conn.endCursor = conn.edges[len(conn.edges)-1]
						}

						conn.hasPreviousPage = result.HasPreviousPage
						conn.hasNextPage = result.HasNextPage

						return conn, nil
					}

					if field.GetAttrValueDefault("referenceType", "") == "joined" {
						result := db.PaginationQuery(db.PaginationRequest{
							Ctx: p.Context,
							DB:  dbFromContext,

							Metadata: db.PaginationRequestJoinedMetadata{
								JoinTable:     joinTable.GetAttrValueDefault("name", ""),
								ForeignColumn: joinForeignColumn.GetAttrValueDefault("name", ""),
								OwnColumn:     joinOwnColumn.GetAttrValueDefault("name", ""),
								OwnValue:      c.id,
							},

							// TODO: arguments
						})
						if result.Err != nil {
							return nil, result.Err
						}

						var conn connection
						for _, id := range result.IDs {
							conn.edges = append(conn.edges, cursor{object: objName, id: id})
						}

						if len(conn.edges) > 0 {
							conn.startCursor = conn.edges[0]
						}

						if len(conn.edges) > 0 {
							conn.endCursor = conn.edges[len(conn.edges)-1]
						}

						conn.hasPreviousPage = result.HasPreviousPage
						conn.hasNextPage = result.HasNextPage

						return conn, nil
					}

					return nil, nil
				},
			})

			return true
		})
		if err != nil {
			return false
		}

		return true
	})

	return err
}

func initObjects(g *graph.Graph) error {
	// create objects, edges and connections first
	createObjects(g)

	// add fields last to break circular dependencies
	return addFields(g)
}
