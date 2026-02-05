# AGENTS.md - Hypergoat Development Guide

Use 'bd' for task tracking

## Project Overview

Hypergoat is a Go port of [Quickslice](https://github.com/quickslice/quickslice) - an AT Protocol AppView server that indexes Lexicon-defined records and exposes them via a dynamically-generated GraphQL API.

**Original:** Gleam (Erlang/OTP) | **Port:** Go 1.22+

## Build/Test Commands

```bash
# Build
make build                    # Build binary to bin/hypergoat
go build ./...                # Build all packages

# Run
make run                      # Build and run server
make dev                      # Run with hot reload (requires air)

# Test
make test                     # Run all tests
go test ./...                 # Run all tests
go test -v ./internal/...     # Run tests with verbose output
go test -run TestName ./...   # Run specific test by name
go test ./internal/lexicon/...  # Run tests for specific package

# Lint
make lint                     # Run golangci-lint
golangci-lint run ./...       # Run linter directly

# Format
make fmt                      # Format code
go fmt ./...                  # Format with go fmt
gofumpt -l -w .               # Format with gofumpt (stricter)

# Database
make db-migrate               # Run migrations
make db-rollback              # Rollback last migration
make db-status                # Show migration status
```

## Code Style Guidelines

### Package Organization
```
internal/           # Private packages
  config/           # Configuration loading
  database/         # Database layer
    executor.go     # Unified interface
    sqlite/         # SQLite implementation
    postgres/       # PostgreSQL implementation
    repositories/   # Data access layer
  graphql/          # GraphQL implementation
  lexicon/          # Lexicon parsing
  oauth/            # OAuth server
pkg/                # Public packages (if any)
cmd/hypergoat/      # Main entry point
```

### Naming Conventions
- **Packages:** lowercase, single word (`lexicon`, `oauth`, `pubsub`)
- **Files:** lowercase with underscores (`did_resolver.go`, `cursor_tracker.go`)
- **Types:** PascalCase (`Executor`, `RecordFetcher`, `WhereClause`)
- **Functions:** PascalCase for exported, camelCase for private
- **Constants:** PascalCase for exported, camelCase for private
- **Interfaces:** Noun or -er suffix (`Executor`, `Fetcher`, `Resolver`)

### Error Handling
Use typed errors with wrapping:
```go
type DBError struct {
    Code    string
    Message string
    Cause   error
}

func (e *DBError) Error() string { return e.Message }
func (e *DBError) Unwrap() error { return e.Cause }

// Usage
if err != nil {
    return fmt.Errorf("failed to query records: %w", err)
}
```

### Context Usage
Always pass context as first parameter:
```go
func (r *RecordsRepository) GetByURI(ctx context.Context, uri string) (*Record, error)
```

### Interface Design
Define interfaces where they're used, not where implemented:
```go
// In the consumer package
type RecordFetcher interface {
    FetchRecords(ctx context.Context, collection string, params PaginationParams) (*QueryResult, error)
}
```

### Testing
- Table-driven tests with descriptive names
- Test both SQLite and PostgreSQL
- Use `testdata/` for fixtures

```go
func TestParseLexicon(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *Lexicon
        wantErr bool
    }{
        {
            name:  "simple record",
            input: `{"lexicon":1,"id":"xyz.test"}`,
            want:  &Lexicon{ID: "xyz.test"},
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseLexicon(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            // ...
        })
    }
}
```

## Database Abstraction

Port Quickslice's Executor pattern for multi-database support:

```go
type Executor interface {
    Query(ctx context.Context, sql string, params []Value, dest any) error
    Exec(ctx context.Context, sql string, params []Value) error
    Dialect() Dialect
    Placeholder(index int) string   // "?" vs "$1"
    JSONExtract(col, field string) string
    Now() string
}
```

## Key Patterns

### Concurrency
| Gleam Pattern | Go Equivalent |
|--------------|---------------|
| Actor + Subject | Goroutine + channel |
| group_registry | sync.Map + channels |
| ETS cache | sync.Map |
| Supervisor | errgroup |

### GraphQL
Using `graphql-go/graphql` for runtime schema building (like Quickslice):
```go
field := &graphql.Field{
    Type: graphql.String,
    Resolve: func(p graphql.ResolveParams) (any, error) {
        // ...
    },
}
```

## Environment Variables

See `.env.example` for all configuration options. Key variables:
- `DATABASE_URL` - SQLite or PostgreSQL connection string
- `SECRET_KEY_BASE` - Session encryption (64+ chars)
- `EXTERNAL_BASE_URL` - Public URL for OAuth
- `OAUTH_LOOPBACK_MODE` - Enable for local development

## Reference

- **Implementation Plan:** `docs/IMPLEMENTATION_PLAN.md`
- **Original Quickslice:** `../quickslice/` (see AGENTS.md there)
- **AT Protocol:** https://atproto.com/docs
