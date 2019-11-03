package schema

import (
	"dynamic-graphql-api/handler/schema/graph"
	"fmt"
	"strings"
	"time"

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
					Type:        graphql.NewNonNull(graphql.String),
					Description: "A cursor for use in pagination.",
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						cursor, ok := p.Source.(cursor)
						if !ok {
							return nil, errors.New("malformed source")
						}

						return cursor.OpaqueString(), nil
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

			fmt.Printf("Adding field %s.%s:%s ...\n", objName, fieldName, fieldType.Name())

			graphqlObjects[objName].AddFieldConfig(fieldName, &graphql.Field{
				Type: fieldType,
				Args: fieldArgs,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					cursor, ok := p.Source.(cursor)
					if !ok {
						return nil, errors.New("Malformed source")
					}

					switch fieldType.Name() {
					case "Int", "Int!":
						// TODO: get Int at cursor from DB (sql.NullInt64)
						return 42, nil
					case "Float", "Float!":
						// TODO: get Float at cursor from DB (sql.NullFloat64)
						return 42.1337, nil
					case "String", "String!":
						// TODO: get String at cursor from DB (sql.NullString)
						return "Hello World!", nil
					case "Boolean", "Boolean!":
						// TODO: get Boolean at cursor from DB (sql.NullBool)
						return true, nil
					case "ID", "ID!":
						return cursor.OpaqueString(), nil
					case "DateTime", "DateTime!":
						// TODO: get DateTime at cursor from DB (sql.NullDateTime)
						return time.Now(), nil
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
