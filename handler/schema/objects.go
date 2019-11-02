package schema

import (
	"dynamic-graphql-api/handler/schema/graph"
	"errors"

	"github.com/graphql-go/graphql"
)

var (
	graphqlObjects     = map[string]*graphql.Object{}
	graphqlEdges       = map[string]*graphql.Object{}
	graphqlConnections = map[string]*graphql.Object{}
)

func initObjects(g *graph.Graph) error {
	g.Nodes().FilterObjects().ForEach(func(obj *graph.Node) bool {
		objName := obj.GetAttrValueDefault("name", "")

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

	return nil
}
