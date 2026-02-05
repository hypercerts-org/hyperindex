// Package postgres provides a PostgreSQL implementation of the database Executor interface.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver

	"github.com/GainForest/hypergoat/internal/database"
)

// Executor implements database.Executor for PostgreSQL.
type Executor struct {
	db *sql.DB
}

// NewExecutor creates a new PostgreSQL executor from a database URL.
// URL format: "postgres://user:pass@host:port/dbname?sslmode=disable"
func NewExecutor(databaseURL string) (*Executor, error) {
	// Open the database using pgx driver
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, database.ConnectionError("failed to open PostgreSQL database", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, database.ConnectionError("failed to ping PostgreSQL database", err)
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
		if errors.Is(err, sql.ErrNoRows) {
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
		if strings.Contains(err.Error(), "unique constraint") ||
			strings.Contains(err.Error(), "foreign key constraint") {
			return nil, database.ConstraintError("constraint violation", err)
		}
		return nil, database.QueryError("failed to execute statement", err)
	}
	return result, nil
}

// Dialect returns PostgreSQL.
func (e *Executor) Dialect() database.Dialect {
	return database.PostgreSQL
}

// Placeholder returns "$n" for the given parameter index (1-based).
func (e *Executor) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

// Placeholders returns a comma-separated list of "$n" placeholders.
func (e *Executor) Placeholders(count, startIndex int) string {
	if count <= 0 {
		return ""
	}
	placeholders := make([]string, count)
	for i := 0; i < count; i++ {
		placeholders[i] = fmt.Sprintf("$%d", startIndex+i)
	}
	return strings.Join(placeholders, ", ")
}

// JSONExtract generates PostgreSQL JSON extraction SQL.
func (e *Executor) JSONExtract(column, field string) string {
	return fmt.Sprintf("%s->>'%s'", column, field)
}

// JSONExtractPath generates PostgreSQL JSON path extraction SQL.
func (e *Executor) JSONExtractPath(column string, path []string) string {
	if len(path) == 0 {
		return column
	}
	if len(path) == 1 {
		return fmt.Sprintf("%s->>'%s'", column, path[0])
	}

	// Build the path: column->'a'->'b'->>'c'
	var sb strings.Builder
	sb.WriteString(column)
	for i, p := range path[:len(path)-1] {
		_ = i
		sb.WriteString("->'")
		sb.WriteString(p)
		sb.WriteString("'")
	}
	sb.WriteString("->>'")
	sb.WriteString(path[len(path)-1])
	sb.WriteString("'")
	return sb.String()
}

// Now returns PostgreSQL's current timestamp function.
func (e *Executor) Now() string {
	return "NOW()"
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
			// PostgreSQL uses native booleans
			args[i] = bool(v)
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

// scanRows scans rows into a slice.
// TODO: Implement struct scanning using reflection
func scanRows(rows *sql.Rows, dest any) error {
	// For now, we'll implement a simpler version that works with specific types
	// This will be enhanced later to support generic struct scanning
	columns, err := rows.Columns()
	if err != nil {
		return database.DecodeError("failed to get columns", err)
	}

	_ = columns
	_ = dest
	return nil
}
