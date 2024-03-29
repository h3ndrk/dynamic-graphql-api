package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// MutationCreateRequest describes the query.
type MutationCreateRequest struct {
	Ctx context.Context
	DB  *sql.DB

	Table        string
	ColumnValues map[string]interface{}
}

// MutationCreateQuery creates a row in the database and returns the created id.
func MutationCreateQuery(r MutationCreateRequest) (uint, error) {
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

// MutationUpdateRequest describes the query.
type MutationUpdateRequest struct {
	Ctx context.Context
	DB  *sql.DB

	Table                string
	ColumnValues         map[string]interface{}
	ColumnWithPrimaryKey string
}

// MutationUpdateQuery updates a row in the database.
func MutationUpdateQuery(r MutationUpdateRequest) error {
	var columnExprs []string
	var columnValues []interface{}

	var columnID string
	var columnIDValue interface{}

	for name, value := range r.ColumnValues {
		if name == r.ColumnWithPrimaryKey {
			columnID = name
			columnIDValue = value
		} else {
			columnExprs = append(columnExprs, fmt.Sprintf("%s = ?", name))
			columnValues = append(columnValues, value)
		}
	}

	_, err := r.DB.ExecContext(
		r.Ctx,
		fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?", r.Table, strings.Join(columnExprs, ", "), columnID),
		append(columnValues, columnIDValue)...)
	if err != nil {
		return err
	}

	return nil
}

// MutationDeleteRequest describes the query.
type MutationDeleteRequest struct {
	Ctx context.Context
	DB  *sql.DB

	Table       string
	ColumnName  string
	ColumnValue interface{}
}

// MutationDeleteQuery deletes a row from the database.
func MutationDeleteQuery(r MutationDeleteRequest) error {
	_, err := r.DB.ExecContext(
		r.Ctx,
		fmt.Sprintf("DELETE FROM %s WHERE %s = ?", r.Table, r.ColumnName),
		r.ColumnValue)
	if err != nil {
		return err
	}

	return nil
}

// MutationAssociateRequest describes the query.
type MutationAssociateRequest struct {
	Ctx context.Context
	DB  *sql.DB

	Table        string
	ColumnValues map[string]interface{}
}

// MutationAssociateQuery associates two rows in the database.
func MutationAssociateQuery(r MutationAssociateRequest) error {
	var columnNames []string
	var columnValueStrings []string
	var columnValues []interface{}
	for name, value := range r.ColumnValues {
		columnNames = append(columnNames, name)
		columnValueStrings = append(columnValueStrings, "?")
		columnValues = append(columnValues, value)
	}

	_, err := r.DB.ExecContext(
		r.Ctx,
		fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", r.Table, strings.Join(columnNames, ", "), strings.Join(columnValueStrings, ", ")),
		columnValues...)
	if err != nil {
		return err
	}

	return nil
}

// MutationDisassociateRequest describes the query.
type MutationDisassociateRequest struct {
	Ctx context.Context
	DB  *sql.DB

	Table        string
	ColumnValues map[string]interface{}
}

// MutationDisassociateQuery disassociates two rows in the database.
func MutationDisassociateQuery(r MutationDisassociateRequest) error {
	var columnExprs []string
	var columnValues []interface{}

	for name, value := range r.ColumnValues {
		columnExprs = append(columnExprs, fmt.Sprintf("%s = ?", name))
		columnValues = append(columnValues, value)
	}

	_, err := r.DB.ExecContext(
		r.Ctx,
		fmt.Sprintf("DELETE FROM %s WHERE %s", r.Table, strings.Join(columnExprs, " AND ")),
		columnValues...)
	if err != nil {
		return err
	}

	return nil
}
