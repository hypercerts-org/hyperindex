// Package server contains the main server initialization and orchestration.
package server

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/GainForest/hypergoat/internal/database"
	"github.com/GainForest/hypergoat/internal/database/postgres"
	"github.com/GainForest/hypergoat/internal/database/sqlite"
)

// ConnectDatabase creates a database executor based on the database URL.
// Supported formats:
//   - sqlite:path/to/file.db
//   - sqlite::memory:
//   - postgres://user:pass@host:port/dbname
//   - postgresql://user:pass@host:port/dbname
func ConnectDatabase(databaseURL string) (database.Executor, error) {
	dialect := database.ParseDialect(databaseURL)

	slog.Info("Connecting to database",
		"dialect", dialect.String(),
		"url", redactPassword(databaseURL),
	)

	switch dialect {
	case database.PostgreSQL:
		return postgres.NewExecutor(databaseURL)
	case database.SQLite:
		return sqlite.NewExecutor(databaseURL)
	default:
		return nil, fmt.Errorf("unsupported database URL: %s", databaseURL)
	}
}

// redactPassword hides the password in a database URL for logging.
func redactPassword(url string) string {
	// For postgres URLs: postgres://user:pass@host -> postgres://user:***@host
	if !strings.Contains(url, "@") {
		return url
	}

	parts := strings.SplitN(url, "@", 2)
	if len(parts) != 2 {
		return url
	}

	prefix := parts[0]
	suffix := parts[1]

	if idx := strings.LastIndex(prefix, ":"); idx > 0 {
		if protoIdx := strings.Index(prefix, "://"); protoIdx > 0 && idx > protoIdx {
			return prefix[:idx+1] + "***@" + suffix
		}
	}

	return url
}
