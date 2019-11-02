package schema

import (
	"dynamic-graphql-api/handler/schema/graph"

	"github.com/graphql-go/graphql"
)

// NewSchema creates a new schema based on SQL statements.
func NewSchema(sqls []string) (*graphql.Schema, error) {
	objectGraph, err := graph.NewGraph(sqls)
	if err != nil {
		return nil, err
	}

	initNode()
	initPageInfo()
	if err := initObjects(objectGraph); err != nil {
		return nil, err
	}
	initQuery(objectGraph)

	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: query /*, Mutation: mutation*/})
	if err != nil {
		return nil, err
	}

	return &schema, nil
}
