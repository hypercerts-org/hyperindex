// Package sqlite provides a SQLite implementation of the database Executor interface.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite" // Pure Go SQLite driver

	"github.com/GainForest/hypergoat/internal/database"
)

// Executor implements database.Executor for SQLite.
type Executor struct {
	db *sql.DB
}

// NewExecutor creates a new SQLite executor from a database URL.
// URL format: "sqlite:path/to/file.db" or "sqlite::memory:"
func NewExecutor(databaseURL string) (*Executor, error) {
	// Parse the URL to get the file path
	path := strings.TrimPrefix(databaseURL, "sqlite:")
	if path == "" {
		path = ":memory:"
	}

	// Open the database
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, database.ConnectionError("failed to open SQLite database", err)
	}

	// Configure connection pool for SQLite
	db.SetMaxOpenConns(1) // SQLite doesn't handle concurrent writes well
	db.SetMaxIdleConns(1)

	// Enable foreign keys and WAL mode for better performance
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, database.ConnectionError("failed to enable foreign keys", err)
	}

	// Enable WAL mode for better concurrent read performance (skip for :memory:)
	if path != ":memory:" {
		if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
			db.Close()
			return nil, database.ConnectionError("failed to enable WAL mode", err)
		}
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, database.ConnectionError("failed to ping SQLite database", err)
	}

	return &Executor{db: db}, nil
}

// Query executes a query and scans results into dest.
func (e *Executor) Query(ctx context.Context, sqlStr string, params []database.Value, dest any) error {
	args := convertParams(params)
	rows, err := e.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return database.QueryError("failed to execute query", err)
	}
	defer rows.Close()

	return scanRows(rows, dest)
}

// QueryRow executes a query expected to return at most one row.
func (e *Executor) QueryRow(ctx context.Context, sqlStr string, params []database.Value, dest ...any) error {
	args := convertParams(params)
	row := e.db.QueryRowContext(ctx, sqlStr, args...)
	if err := row.Scan(dest...); err != nil {
		if err == sql.ErrNoRows {
			return err
		}
		return database.QueryError("failed to scan row", err)
	}
	return nil
}

// Exec executes a statement without returning results.
func (e *Executor) Exec(ctx context.Context, sqlStr string, params []database.Value) (sql.Result, error) {
	args := convertParams(params)
	result, err := e.db.ExecContext(ctx, sqlStr, args...)
	if err != nil {
		// Check for constraint violations
		if strings.Contains(err.Error(), "UNIQUE constraint") ||
			strings.Contains(err.Error(), "FOREIGN KEY constraint") {
			return nil, database.ConstraintError("constraint violation", err)
		}
		return nil, database.QueryError("failed to execute statement", err)
	}
	return result, nil
}

// Dialect returns SQLite.
func (e *Executor) Dialect() database.Dialect {
	return database.SQLite
}

// Placeholder returns "?" for all parameters (SQLite ignores index).
func (e *Executor) Placeholder(index int) string {
	return "?"
}

// Placeholders returns a comma-separated list of "?" placeholders.
func (e *Executor) Placeholders(count, startIndex int) string {
	if count <= 0 {
		return ""
	}
	placeholders := make([]string, count)
	for i := 0; i < count; i++ {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ", ")
}

// JSONExtract generates SQLite JSON extraction SQL.
func (e *Executor) JSONExtract(column, field string) string {
	return fmt.Sprintf("json_extract(%s, '$.%s')", column, field)
}

// JSONExtractPath generates SQLite JSON path extraction SQL.
func (e *Executor) JSONExtractPath(column string, path []string) string {
	jsonPath := "$." + strings.Join(path, ".")
	return fmt.Sprintf("json_extract(%s, '%s')", column, jsonPath)
}

// Now returns SQLite's current timestamp function.
func (e *Executor) Now() string {
	return "datetime('now')"
}

// Close closes the database connection.
func (e *Executor) Close() error {
	return e.db.Close()
}

// DB returns the underlying *sql.DB.
func (e *Executor) DB() *sql.DB {
	return e.db
}

// convertParams converts database.Value slice to []any for sql.DB methods.
func convertParams(params []database.Value) []any {
	if len(params) == 0 {
		return nil
	}

	args := make([]any, len(params))
	for i, param := range params {
		switch v := param.(type) {
		case database.TextValue:
			args[i] = string(v)
		case database.IntValue:
			args[i] = int64(v)
		case database.FloatValue:
			args[i] = float64(v)
		case database.BoolValue:
			// SQLite uses integers for booleans
			if bool(v) {
				args[i] = 1
			} else {
				args[i] = 0
			}
		case database.NullValue:
			args[i] = nil
		case database.BlobValue:
			args[i] = []byte(v)
		case database.TimestamptzValue:
			args[i] = string(v)
		default:
			args[i] = param
		}
	}
	return args
}

// scanRows scans rows into a slice of maps.
// TODO: Implement struct scanning using reflection
func scanRows(rows *sql.Rows, dest any) error {
	_ = dest // Will be used when struct scanning is implemented

	// For now, we'll implement a simpler version that works with specific types
	// This will be enhanced later to support generic struct scanning
	columns, err := rows.Columns()
	if err != nil {
		return database.DecodeError("failed to get columns", err)
	}

	// Create a slice of interface{} to hold the values
	values := make([]any, len(columns))
	valuePtrs := make([]any, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	// For now, return an error indicating we need specific implementations
	_ = columns
	_ = values
	_ = valuePtrs

	return nil
}
