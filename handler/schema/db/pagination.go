package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

type PaginationRequest struct {
	Ctx context.Context
	DB  *sql.DB

	Table  string
	Column string

	Before *uint
	After  *uint
	First  *uint
	Last   *uint
}

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
	// SELECT {{ .Column }} FROM {{ .Table }}
	if r.First != nil && r.Last != nil {
		// reset last
		r.Last = nil
	}

	query := fmt.Sprintf("SELECT %s FROM %s", r.Column, r.Table)
	args := []interface{}{}

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
