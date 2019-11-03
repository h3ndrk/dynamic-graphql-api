package db

import (
	"context"
	"database/sql"
	"fmt"
)

// ScalarRequest describes the query.
type ScalarRequest struct {
	Ctx context.Context
	DB  *sql.DB

	Table  string
	Column string

	ID uint
}

// ScalarIntQuery queries the database and returns a integer.
func ScalarIntQuery(r ScalarRequest) (interface{}, error) {
	var (
		value sql.NullInt64
	)
	if err := r.DB.QueryRowContext(r.Ctx, fmt.Sprintf("SELECT %s FROM %s WHERE id = ?", r.Column, r.Table), r.ID).Scan(&value); err != nil {
		return nil, err
	}

	if !value.Valid {
		return nil, nil
	}

	return value.Int64, nil
}

// ScalarFloatQuery queries the database and returns a float.
func ScalarFloatQuery(r ScalarRequest) (interface{}, error) {
	var (
		value sql.NullFloat64
	)
	if err := r.DB.QueryRowContext(r.Ctx, fmt.Sprintf("SELECT %s FROM %s WHERE id = ?", r.Column, r.Table), r.ID).Scan(&value); err != nil {
		return nil, err
	}

	if !value.Valid {
		return nil, nil
	}

	return value.Float64, nil
}

// ScalarStringQuery queries the database and returns a string.
func ScalarStringQuery(r ScalarRequest) (interface{}, error) {
	var (
		value sql.NullString
	)
	if err := r.DB.QueryRowContext(r.Ctx, fmt.Sprintf("SELECT %s FROM %s WHERE id = ?", r.Column, r.Table), r.ID).Scan(&value); err != nil {
		return nil, err
	}

	if !value.Valid {
		return nil, nil
	}

	return value.String, nil
}

// ScalarBooleanQuery queries the database and returns a boolean.
func ScalarBooleanQuery(r ScalarRequest) (interface{}, error) {
	var (
		value sql.NullBool
	)
	if err := r.DB.QueryRowContext(r.Ctx, fmt.Sprintf("SELECT %s FROM %s WHERE id = ?", r.Column, r.Table), r.ID).Scan(&value); err != nil {
		return nil, err
	}

	if !value.Valid {
		return nil, nil
	}

	return value.Bool, nil
}

// ScalarDateTimeQuery queries the database and returns a date-time.
func ScalarDateTimeQuery(r ScalarRequest) (interface{}, error) {
	var (
		value sql.NullTime
	)
	if err := r.DB.QueryRowContext(r.Ctx, fmt.Sprintf("SELECT %s FROM %s WHERE id = ?", r.Column, r.Table), r.ID).Scan(&value); err != nil {
		return nil, err
	}

	if !value.Valid {
		return nil, nil
	}

	return value.Time, nil
}
