package schema

import (
	"context"
	"database/sql"
	"dynamic-graphql-api/handler/schema/graph"

	"github.com/graphql-go/graphql"
	"github.com/pkg/errors"
)

type key int

const (
	// KeyDB is the context key for the database value.
	KeyDB key = iota
)

func getDBFromContext(ctx context.Context) (*sql.DB, error) {
	db, ok := ctx.Value(KeyDB).(*sql.DB)
	if !ok {
		return nil, errors.New("Missing DB in context")
	}

	return db, nil
}

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
