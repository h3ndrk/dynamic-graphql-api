package schema

import (
	"fmt"

	"github.com/graphql-go/graphql"
	"github.com/pkg/errors"
)

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
		Type: graphql.ID,
	},
	"after": &graphql.ArgumentConfig{
		Type: graphql.ID,
	},
	"first": &graphql.ArgumentConfig{
		Type: graphql.Int,
	},
	"last": &graphql.ArgumentConfig{
		Type: graphql.Int,
	},
}

func getConnectionArgs(p graphql.ResolveParams, objName string) (*uint, *uint, *uint, *uint, error) {
	var (
		before *uint
		after  *uint
		first  *uint
		last   *uint
	)

	if beforeCursor, ok := p.Args["before"]; ok {
		c, err := parseCursor(fmt.Sprintf("%v", beforeCursor))
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if c.object != objName {
			return nil, nil, nil, nil, errors.Errorf("invalid cursor '%s' (not matching type %s)", c, objName)
		}
		before = &c.id
	}

	if afterCursor, ok := p.Args["after"]; ok {
		c, err := parseCursor(fmt.Sprintf("%v", afterCursor))
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if c.object != objName {
			return nil, nil, nil, nil, errors.Errorf("invalid cursor '%s' (not matching type %s)", c, objName)
		}
		after = &c.id
	}

	if firstValue, ok := p.Args["first"]; ok {
		if firstValue, ok := firstValue.(int); ok {
			firstValueUint := uint(firstValue)
			first = &firstValueUint
		} else {
			return nil, nil, nil, nil, errors.Errorf("Invalid value first '%v'", firstValue)
		}
	}

	if lastValue, ok := p.Args["last"]; ok {
		if lastValue, ok := lastValue.(int); ok {
			lastValueUint := uint(lastValue)
			last = &lastValueUint
		} else {
			return nil, nil, nil, nil, errors.Errorf("Invalid value last '%v'", lastValue)
		}
	}

	return before, after, first, last, nil
}
