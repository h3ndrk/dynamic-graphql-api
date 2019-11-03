package schema

import (
	"github.com/graphql-go/graphql"
)

var node *graphql.Interface

func initNodeBefore() {
	node = graphql.NewInterface(graphql.InterfaceConfig{
		Name: "Node",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.NewNonNull(graphql.ID),
			},
		},
	})
}

func initNodeAfter() {
	node.ResolveType = func(p graphql.ResolveTypeParams) *graphql.Object {
		c, ok := p.Value.(cursor)
		if !ok {
			return nil
		}

		for name := range graphqlObjects {
			if name == c.object {
				return graphqlObjects[name]
			}
		}

		return nil
	}
}
