package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// MutationRequest describes the query.
type MutationRequest struct {
	Ctx context.Context
	DB  *sql.DB

	Table        string
	ColumnValues map[string]interface{}
}

// MutationCreateQuery creates a row in the database and returns the created id.
func MutationCreateQuery(r MutationRequest) (uint, error) {
	var columnNames []string
	var columnValueStrings []string
	var columnValues []interface{}
	for name, value := range r.ColumnValues {
		columnNames = append(columnNames, name)
		columnValueStrings = append(columnValueStrings, "?")
		columnValues = append(columnValues, value)
	}

	result, err := r.DB.ExecContext(
		r.Ctx,
		fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", r.Table, strings.Join(columnNames, ", "), strings.Join(columnValueStrings, ", ")),
		columnValues...)
	if err != nil {
		return 0, err
	}

	insertedID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return uint(insertedID), nil
}
