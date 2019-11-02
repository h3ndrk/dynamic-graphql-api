package schema

type connection struct {
	// PageInfo
	hasNextPage     bool
	hasPreviousPage bool
	startCursor     string
	endCursor       string

	// Edges
	edges []cursor
}
