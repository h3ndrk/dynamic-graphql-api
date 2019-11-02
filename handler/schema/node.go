package schema

import "github.com/graphql-go/graphql"

var node *graphql.Interface

func initNode() error {
	node = graphql.NewInterface(graphql.InterfaceConfig{
		Name: "Node",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.NewNonNull(graphql.ID),
			},
		},
	})

	return nil
}
