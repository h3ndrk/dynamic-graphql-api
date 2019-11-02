package schema

type connection struct {
	// PageInfo
	hasNextPage     bool
	hasPreviousPage bool
	startCursor     cursor
	endCursor       cursor

	// Edges
	edges []cursor
}
