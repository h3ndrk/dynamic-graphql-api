package schema

import (
	"dynamic-graphql-api/handler/schema/graph"

	"github.com/graphql-go/graphql"
	"github.com/iancoleman/strcase"
	"github.com/jinzhu/inflection"
)

var query *graphql.Object

func initQuery(g *graph.Graph) {
	query = graphql.NewObject(graphql.ObjectConfig{
		Name:   "Query",
		Fields: graphql.Fields{},
	})

	g.Nodes().FilterObjects().ForEach(func(obj *graph.Node) bool {
		objName := obj.GetAttrValueDefault("name", "")
		fieldName := inflection.Plural(strcase.ToLowerCamel(objName))

		query.AddFieldConfig(fieldName, &graphql.Field{
			Type: graphql.NewNonNull(graphqlConnections[objName]),
			Args: connectionArgs,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return connection{}, nil
			},
		})

		return true
	})
}
