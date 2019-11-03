package db

import "database/sql"

// NewDB creates a new database connection and also returns all CREATE TABLE statements.
func NewDB(driverName string, dataSourceName string) (*sql.DB, []string, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, nil, err
	}

	rows, err := db.Query(
		"SELECT sql FROM sqlite_master WHERE type = 'table'",
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var (
		sqlString string
		sqls      []string
	)
	for rows.Next() {
		err := rows.Scan(&sqlString)
		if err != nil {
			return nil, nil, err
		}

		sqls = append(sqls, sqlString)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return db, sqls, nil
}
