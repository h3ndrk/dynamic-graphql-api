package handler

import (
	"context"
	"database/sql"
	"dynamic-graphql-api/handler/schema"
	"dynamic-graphql-api/handler/schema/db"
	"net/http"

	"github.com/graphql-go/handler"
	"github.com/pkg/errors"
)

// Handler implements the http.Handler interface and stores a database connection.
type Handler struct {
	db *sql.DB
	h  *handler.Handler
}

// ServeHTTP provides an entrypoint into executing graphQL queries.
func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.h.ContextHandler(context.WithValue(r.Context(), schema.KeyDB, h.db), w, r)
}

// NewHandler creates a new GraphQL handler with a database connection.
func NewHandler(driverName string, dataSourceName string) (*Handler, error) {
	db, sqls, err := db.NewDB(driverName, dataSourceName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create database")
	}

	s, err := schema.NewSchema(sqls)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create schema")
	}

	return &Handler{
		db: db,
		h: handler.New(&handler.Config{
			Schema:     s,
			Pretty:     true,
			Playground: true,
		}),
	}, nil
}

// Close closes the database connection.
func (h *Handler) Close() error {
	return h.db.Close()
}
