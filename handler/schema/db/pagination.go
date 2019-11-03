package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

// PaginationRequestMetadata represents the metadata for a query.
type PaginationRequestMetadata interface {
	IsPaginationRequestMetadata()
}

// PaginationRequestForwardMetadata represents the metadata for forward references.
type PaginationRequestForwardMetadata struct {
	Table  string
	Column string
}

// IsPaginationRequestMetadata is used for interface constraining.
func (PaginationRequestForwardMetadata) IsPaginationRequestMetadata() {}

// PaginationRequestBackwardMetadata represents the metadata for forward references.
// Example: SELECT {ForeignReturnColumn} FROM {ForeignTable} WHERE {ForeignReferenceColumn} = {OwnReferenceColumn}
type PaginationRequestBackwardMetadata struct {
	ForeignTable           string
	ForeignReferenceColumn string
	ForeignReturnColumn    string
	OwnReferenceColumn     interface{}
}

// IsPaginationRequestMetadata is used for interface constraining.
func (PaginationRequestBackwardMetadata) IsPaginationRequestMetadata() {}

// PaginationRequestJoinedMetadata represents the metadata for joined references.
// Example: SELECT {ForeignColumn} FROM {JoinTable} WHERE {OwnColumn} = {OwnValue}
type PaginationRequestJoinedMetadata struct {
	JoinTable     string
	ForeignColumn string
	OwnColumn     string
	OwnValue      interface{}
}

// IsPaginationRequestMetadata is used for interface constraining.
func (PaginationRequestJoinedMetadata) IsPaginationRequestMetadata() {}

// PaginationRequest describes the query.
type PaginationRequest struct {
	Ctx context.Context
	DB  *sql.DB

	Metadata PaginationRequestMetadata

	Before *uint
	After  *uint
	First  *uint
	Last   *uint
}

// PaginationResult represents the response from a database pagination query.
type PaginationResult struct {
	Err error

	IDs             []uint
	HasPreviousPage bool
	HasNextPage     bool
}

func max(a, b uint) uint {
	if a < b {
		return b
	}

	return a
}

func ternaryMax(a, b, c uint) uint {
	return max(max(a, b), c)
}

func min(a, b uint) uint {
	if a > b {
		return b
	}

	return a
}

// PaginationQuery queries the database and returns a page of result ids.
func PaginationQuery(r PaginationRequest) PaginationResult {
	if r.First != nil && r.Last != nil {
		// reset last
		r.Last = nil
	}

	var query string
	var args []interface{}
	switch metadata := r.Metadata.(type) {
	case PaginationRequestForwardMetadata:
		query = fmt.Sprintf("SELECT %s FROM %s", metadata.Column, metadata.Table)
	case PaginationRequestBackwardMetadata:
		query = fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", metadata.ForeignReturnColumn, metadata.ForeignTable, metadata.ForeignReferenceColumn)
		args = []interface{}{metadata.OwnReferenceColumn}
	case PaginationRequestJoinedMetadata:
		query = fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", metadata.ForeignColumn, metadata.JoinTable, metadata.OwnColumn)
		args = []interface{}{metadata.OwnValue}
	default:
		return PaginationResult{Err: errors.Errorf("unknown metadata type %T", metadata)}
	}

	// count rows in table
	var count uint
	if err := r.DB.QueryRowContext(r.Ctx, fmt.Sprintf("SELECT count(*) FROM (%s)", query), args...).Scan(&count); err != nil {
		return PaginationResult{Err: errors.Wrap(err, "database error (count)")}
	}

	// get before and after positions (one-based)
	var (
		positionBefore uint
		positionAfter  uint
	)
	if r.Before != nil || r.After != nil {
		var (
			whereExprs  []string
			whereValues []interface{}
		)
		if r.Before != nil {
			whereExprs = append(whereExprs, "id = ?")
			whereValues = append(whereValues, *r.Before)
		}
		if r.After != nil {
			whereExprs = append(whereExprs, "id = ?")
			whereValues = append(whereValues, *r.After)
		}
		positionRows, err := r.DB.QueryContext(r.Ctx, fmt.Sprintf(
			"SELECT * FROM (SELECT id, row_number() OVER () AS row_id FROM (%s)) WHERE %s",
			query, strings.Join(whereExprs, " OR "),
		), append(args, whereValues...)...)
		if err != nil {
			return PaginationResult{Err: errors.Wrap(err, "database error (positions with where)")}
		}
		defer positionRows.Close()

		var (
			id    uint
			rowID uint
		)
		for positionRows.Next() {
			err := positionRows.Scan(&id, &rowID)
			if err != nil {
				return PaginationResult{Err: errors.Wrap(err, "database error (positions scan)")}
			}

			if r.Before != nil && id == *r.Before {
				positionBefore = rowID
			}
			if r.After != nil && id == *r.After {
				positionAfter = rowID
			}
		}

		if err := positionRows.Err(); err != nil {
			return PaginationResult{Err: errors.Wrap(err, "database error (positions error)")}
		}
	}

	// calculate range (one-based)
	var (
		begin uint = 1         // inclusive
		end   uint = count + 1 // exclusive
	)
	if r.Before != nil {
		end = positionBefore
	}
	if r.After != nil {
		begin = positionAfter + 1
	}
	if r.First != nil {
		end = min(begin+*r.First, count+1)
	}
	if r.Last != nil {
		begin = ternaryMax(1, begin, end-*r.Last)
	}

	// query range
	rows, err := r.DB.QueryContext(r.Ctx, fmt.Sprintf(
		`SELECT * FROM (
		SELECT
			*, row_number() OVER () AS __row_id
		FROM (%s)
	) WHERE __row_id >= ? AND __row_id < ?`, query,
	), append(args, &begin, &end)...)
	if err != nil {
		return PaginationResult{Err: errors.Wrap(err, "database error (rows)")}
	}
	defer rows.Close()

	var (
		value  uint
		rowID  uint
		result PaginationResult
	)
	for rows.Next() {
		if err := rows.Scan(&value, &rowID); err != nil {
			return PaginationResult{Err: errors.Wrap(err, "database error (scan)")}
		}

		result.IDs = append(result.IDs, value)
	}

	if err := rows.Err(); err != nil {
		return PaginationResult{Err: errors.Wrap(err, "database error (err)")}
	}

	result.HasPreviousPage = begin > 1
	result.HasNextPage = end < count+1

	return result
}
