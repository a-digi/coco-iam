package db

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
)

// SQLDelimiter is the delimiter used to split SQL statements in migration files.
const SQLDelimiter = "/***Statement***/"

// ExecuteSQLFile reads a .sql file, splits it into statements using SQLDelimiter, and executes them using the provided *sql.DB.
func ExecuteSQLFile(db *sql.DB, filePath string) error {
	content, err := os.ReadFile(filePath)

	if err != nil {
		return fmt.Errorf("failed to read SQL file: %w", err)
	}

	// Split by custom delimiter
	queries := strings.Split(string(content), SQLDelimiter)
	for _, query := range queries {
		stmt := strings.TrimSpace(query)
		if stmt == "" {
			continue
		}
		_, err := db.Exec(stmt)
		if err != nil {
			return fmt.Errorf("failed to execute statement: %w\nSQL: %s", err, stmt)
		}
	}

	return nil
}
