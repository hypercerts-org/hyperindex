# AGENTS.md - Hypergoat Development Guide

Use 'bd' for task tracking

## Current Status (January 2026)

| Phase | Description | Status |
|-------|-------------|--------|
| Phase 1 | Foundation (config, database, migrations) | ✅ Complete |
| Phase 2 | Lexicon Parsing & GraphQL Core | ✅ Complete |
| Phase 3 | GraphQL API (schema, resolvers) | ✅ Complete |
| Phase 4 | Real-time Features (Jetstream, Subscriptions, Backfill) | ✅ Complete |
| Phase 5 | OAuth & Authentication | ✅ Complete |
| Phase 6 | Admin GraphQL & Management | ✅ Complete |
| **Phase 7** | **Server Integration & Complete OAuth** | 🔄 In Progress |

### Phase 7 Progress

| Task | Description | Status |
|------|-------------|--------|
| 7.1 | OAuth discovery endpoints (.well-known/*) | 🔲 Ready |
| 7.2 | OAuth flow endpoints (/oauth/*) | 🔲 Blocked by 7.1 |
| 7.3 | OAuth client registration (/oauth/register) | 🔲 Blocked by 7.2 |
| 7.4 | OAuth PAR endpoint (/oauth/par) | 🔲 Blocked by 7.2 |
| 7.5 | OAuth DPoP nonce endpoint | 🔲 Blocked by 7.2 |
| 7.6 | OAuth client metadata (dynamic) | 🔲 Ready |
| 7.7 | Admin GraphQL handler wiring | 🔲 Blocked by 7.2 |
| 7.8 | Missing admin mutations | 🔲 Blocked by 7.7 |
| 7.9 | GraphiQL playgrounds (CDN) | 🔲 Blocked by 7.7 |
| 7.10 | Start background workers | 🔲 Blocked by 7.7 |
| 7.11 | Integration tests | 🔲 Blocked by 7.8 |

**Next Steps:** Run `bd ready` to see available work. Phase 7 epic: `adventure-n96`

### What Was Completed

**Phase 5 (OAuth):**
- OAuth database repositories (10 tables)
- DID resolver (did:plc, did:web) with caching
- PKCE and DPoP implementations
- OAuth server core (token generation, scopes, errors)
- AT Protocol bridge for PDS authentication
- Auth middleware with JWT validation
- OAuth HTTP handlers (authorize, callback, token, revoke)

**Phase 6 (Admin):**
- Admin database repositories (jetstream_activity, labels, label_definitions, label_preferences, reports)
- Admin GraphQL types and enums
- Admin query and mutation resolvers
- Admin HTTP handler with OAuth auth
- Background workers (activity cleanup, backfill state)
- Integration tests (9 tests passing)

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

### Core
- `DATABASE_URL` - SQLite or PostgreSQL connection string
- `SECRET_KEY_BASE` - Session encryption (64+ chars)
- `EXTERNAL_BASE_URL` - Public URL for OAuth
- `HOST` / `PORT` - Server binding (default: 127.0.0.1:8080)

### Jetstream (Real-time data ingestion)
- `JETSTREAM_COLLECTIONS` - Comma-separated NSIDs to subscribe to
- `JETSTREAM_URL` - Jetstream endpoint (default: wss://jetstream2.us-west.bsky.network/subscribe)
- `JETSTREAM_DISABLE_CURSOR` - Skip cursor tracking (for dev)

### Backfill (Historical data)
- `BACKFILL_ON_START` - Run backfill when server starts
- `BACKFILL_COLLECTIONS` - Collections to backfill (defaults to JETSTREAM_COLLECTIONS)
- `BACKFILL_RELAY_URL` - AT Protocol relay (default: https://relay1.us-west.bsky.network)

### OAuth (Phase 5)
- `OAUTH_SIGNING_KEY` - JWT signing key for client assertions
- `OAUTH_LOOPBACK_MODE` - Enable for local development
- `PLC_DIRECTORY_URL` - DID resolution (default: https://plc.directory)
- `EXTERNAL_BASE_URL` - Public URL for OAuth callbacks

### Package Structure

```
internal/oauth/                    # OAuth & Authentication
├── types.go                       # OAuth type definitions
├── pkce.go, pkce_test.go          # PKCE (RFC 7636)
├── dpop.go, dpop_test.go          # DPoP proof-of-possession (RFC 9449)
├── did.go, did_test.go            # DID resolver (did:plc, did:web)
├── did_cache.go, did_cache_test.go # DID caching with TTL
├── server.go                      # OAuth server core
├── token_generator.go             # Token generation
├── scopes.go, scopes_test.go      # Scope validation
├── errors.go                      # OAuth error types
├── bridge.go, bridge_test.go      # AT Protocol bridge
└── middleware.go, middleware_test.go # Auth middleware

internal/graphql/admin/            # Admin GraphQL API
├── types.go                       # GraphQL types and enums
├── resolvers.go                   # Query/mutation resolvers
├── schema.go                      # Schema builder
├── handler.go                     # HTTP handler
└── handler_test.go                # Tests (9 tests)

internal/workers/                  # Background Workers
├── activity_cleanup.go            # Hourly cleanup (7-day retention)
└── backfill_state.go              # Backfill progress tracking

internal/database/repositories/    # Data Access Layer
├── oauth_*.go                     # 10 OAuth repositories
├── jetstream_activity.go          # Activity logging
├── labels.go                      # Content labels
├── label_definitions.go           # Label types
├── label_preferences.go           # User preferences
└── reports.go                     # Moderation reports
```

## Reference

- **Implementation Plan:** `docs/IMPLEMENTATION_PLAN.md`
- **Original Quickslice:** `../quickslice/` (see AGENTS.md there)
- **AT Protocol:** https://atproto.com/docs

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
