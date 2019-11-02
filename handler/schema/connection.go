package schema

import "github.com/graphql-go/graphql"

type connection struct {
	// PageInfo
	hasNextPage     bool
	hasPreviousPage bool
	startCursor     cursor
	endCursor       cursor

	// Edges
	edges []cursor
}

var connectionArgs = graphql.FieldConfigArgument{
	"before": &graphql.ArgumentConfig{
		Type: graphql.ID, // TODO: or change back to String?
	},
	"after": &graphql.ArgumentConfig{
		Type: graphql.ID, // TODO: or change back to String?
	},
	"first": &graphql.ArgumentConfig{
		Type: graphql.Int,
	},
	"last": &graphql.ArgumentConfig{
		Type: graphql.Int,
	},
}
