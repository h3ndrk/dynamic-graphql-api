package schema

import (
	"errors"

	"github.com/graphql-go/graphql"
)

var pageInfo *graphql.Object

func initPageInfo() error {
	pageInfo = graphql.NewObject(graphql.ObjectConfig{
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

	return nil
}
