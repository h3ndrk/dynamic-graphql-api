package schema

import (
	"dynamic-graphql-api/handler/schema/db"
	"dynamic-graphql-api/handler/schema/graph"

	"github.com/graphql-go/graphql"
	"github.com/iancoleman/strcase"
	"github.com/jinzhu/inflection"
	"github.com/pkg/errors"
)

var query *graphql.Object

func initQuery(g *graph.Graph) error {
	query = graphql.NewObject(graphql.ObjectConfig{
		Name:   "Query",
		Fields: graphql.Fields{},
	})

	var err error
	g.Nodes().FilterObjects().ForEach(func(obj *graph.Node) bool {
		objName := obj.GetAttrValueDefault("name", "")
		fieldName := inflection.Plural(strcase.ToLowerCamel(objName))

		referencedTable := g.Edges().FilterSource(obj).FilterEdgeType("objectHasTable").Targets().First()
		if referencedTable == nil {
			err = errors.New("referenced table not found")
			return false
		}

		query.AddFieldConfig(fieldName, &graphql.Field{
			Type: graphql.NewNonNull(graphqlConnections[objName]),
			Args: connectionArgs,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				dbFromContext, err := getDBFromContext(p.Context)
				if err != nil {
					return nil, err
				}

				before, after, first, last, err := getConnectionArgs(p, objName)
				if err != nil {
					return nil, err
				}

				result := db.PaginationQuery(db.PaginationRequest{
					Ctx: p.Context,
					DB:  dbFromContext,

					Metadata: db.PaginationRequestForwardMetadata{
						Table:  referencedTable.GetAttrValueDefault("name", ""),
						Column: "id",
					},

					Before: before,
					After:  after,
					First:  first,
					Last:   last,
				})
				if result.Err != nil {
					return nil, result.Err
				}

				// TODO: abstract result -> connection conversion
				var conn connection
				for _, id := range result.IDs {
					conn.edges = append(conn.edges, cursor{object: objName, id: id})
				}

				if len(conn.edges) > 0 {
					conn.startCursor = conn.edges[0]
				}

				if len(conn.edges) > 0 {
					conn.endCursor = conn.edges[len(conn.edges)-1]
				}

				conn.hasPreviousPage = result.HasPreviousPage
				conn.hasNextPage = result.HasNextPage

				return conn, nil
			},
		})

		return true
	})

	return err
}
