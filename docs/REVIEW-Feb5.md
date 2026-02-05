# Hypergoat Codebase Review — Comprehensive Analysis & Improvement Plan

**Date:** February 5, 2026
**Reviewer:** Expert Code Review (Claude)
**Codebase:** ~25,500 lines of Go across 90 files

---

## Executive Summary

Hypergoat is a well-structured Go port of an AT Protocol AppView server. The core architecture is sound: clean separation between packages, proper use of interfaces for database abstraction, and the dynamic GraphQL-from-lexicons concept is creative. All tests pass, `go vet` is clean. However, there are several **significant issues** ranging from security concerns to architectural weaknesses that should be addressed.

**Overall Grade: B-** — Solid foundation with meaningful gaps in security, concurrency safety, and production readiness.

---

## 1. CRITICAL: Security Concerns

### 1a. SQL Injection in JSONExtract Methods

The `JSONExtract` and `JSONExtractPath` methods in **both** SQLite and PostgreSQL executors interpolate user-controlled `field` and `path` values directly into SQL strings:

```go
// sqlite/executor.go:126
func (e *Executor) JSONExtract(column, field string) string {
    return fmt.Sprintf("json_extract(%s, '$.%s')", column, field)
}
// postgres/executor.go:107
func (e *Executor) JSONExtract(column, field string) string {
    return fmt.Sprintf("%s->>'%s'", column, field)
}
```

If any caller passes user input here, it's injectable. These should sanitize/validate field names or use parameterized approaches.

### 1b. WebSocket Origin Check Disabled

```go
// subscription/handler.go:57
CheckOrigin: func(r *http.Request) bool {
    return true // Allow all origins for development
}
```

This is **fine for dev, dangerous in production**. Needs to be configurable.

### 1c. `backfillActive` Race Condition

```go
// admin/resolvers.go:299
r.backfillActive = true
go func() {
    defer func() { r.backfillActive = false }()
    ...
}()
```

This is a data race — `backfillActive` is read/written from multiple goroutines without synchronization. Should use `atomic.Bool` or a mutex.

### 1d. UseJTI Has a TOCTOU Race

```go
// oauth/middleware.go:405
func UseJTI(ctx context.Context, store JTIStore, jti string, iat int64) (bool, error) {
    exists, err := store.Exists(ctx, jti)
    // ... check-then-act without atomicity
    err = store.Insert(ctx, &DPoPJTI{...})
}
```

The comment acknowledges this. Two concurrent requests could both pass the `Exists` check. The database UNIQUE constraint is the only real protection here.

### 1e. Admin API Auth is "Optional"

```go
// main.go:396
r.Handle("/admin/graphql", adminHandler.OptionalAuth())
```

The admin GraphQL endpoint allows unauthenticated introspection and some queries. While intentional for dev, mutations like `resetAll`, `updateSettings`, `addAdmin` need strict auth gating.

---

## 2. HIGH: Architectural Issues

### 2a. God Function: `main.go:run()` — 700 lines

The `run()` function is an enormous monolith handling:

- Config loading, DB setup, migrations
- OAuth setup, admin setup, backfill setup
- Router configuration with inline handlers
- Jetstream consumer lifecycle
- Graceful shutdown

This needs decomposition into discrete startup phases.

### 2b. Pervasive `os.Getenv` Calls Outside Config

Despite having a `config` package, the codebase directly reads environment variables in:

- `main.go` (ADMIN_DIDS, LEXICON_DIR, JETSTREAM_*, BACKFILL_*, DOMAIN_DID)
- `backfill/backfill.go` (6+ env vars in `DefaultConfig`)

This defeats centralized configuration and makes testing difficult.

### 2c. `scanRows` is Unimplemented

Both SQLite and PostgreSQL executors have `scanRows` functions that **do nothing**:

```go
// sqlite/executor.go:187
func scanRows(rows *sql.Rows, dest any) error {
    // TODO: Implement struct scanning using reflection
    return nil
}
```

The `Query` method on both executors returns `nil` without actually scanning data. This means the `Executor.Query()` method is effectively **broken** — all database reads go through `DB().QueryContext()` directly, bypassing the abstraction.

### 2d. Massive Code Duplication

- `extractCreatedAt` is implemented **three times**: `main.go:728`, `admin/resolvers.go:956`, `backfill/backfill.go:846`
- `parseTimestamp` / timestamp parsing logic duplicated across repos and main
- `redactURL`/`redactPassword` duplicated between `config.go` and `server/database.go`
- `convertToAny` in `repositories/records.go` duplicates `convertParams` in both executors
- `ParseCollections` duplicated between `jetstream/client.go` and `backfill/backfill.go`
- The collection resolver and generic records resolver in `schema/builder.go` are ~90% identical code (lines 417-530 vs 534-643)

### 2e. Global PubSub Singleton

```go
// subscription/pubsub.go:134
var globalPubSub = NewPubSub()
func Global() *PubSub { return globalPubSub }
```

Global mutable state makes testing unreliable and prevents running multiple instances. Should be injected via dependency injection.

### 2f. PubSub Subscriber ID Uses Rune Conversion

```go
// pubsub.go:60
ps.nextID++
id := string(rune(ps.nextID))
```

This converts an int64 to a rune to a string — producing unprintable/colliding IDs. After 1,114,112 subscribers (max Unicode), it wraps. Use `strconv.FormatInt` instead.

---

## 3. MEDIUM: Reliability & Correctness

### 3a. No Transaction Support in the Executor Interface

The `Executor` interface has no `BeginTx` method. All transaction usage bypasses it via `db.DB().BeginTx()`:

```go
// records.go:134
tx, err := r.db.DB().BeginTx(ctx, nil)
```

This means transactions skip the parameterized value system and constraint detection entirely.

### 3b. Pagination Correctness Issue

The cursor-based pagination re-sorts by `createdAt` after fetching by `indexed_at`:

```go
// Fetch extra records to allow for re-sorting by createdAt
fetchLimit := first * 2
```

Over-fetching by 2x doesn't guarantee correctness — records could span page boundaries unpredictably. If a page requests 20 records, fetching 50 and re-sorting can still miss records or produce duplicates.

### 3c. Unbounded ZIP Upload Processing

```go
// admin/resolvers.go:229
func (r *Resolver) UploadLexicons(ctx context.Context, zipBase64 string) (int, error) {
    zipData, err := base64.StdEncoding.DecodeString(zipBase64)
```

No size limit on the base64 input. A malicious upload could OOM the server. Needs max size validation.

### 3d. Jetstream Event Channel Buffer Drops

```go
// jetstream/client.go:166
select {
case c.events <- event:
default:
    slog.Warn("Event channel full, dropping event")
}
```

Events are silently dropped under load. This means data loss in the indexer, which for an AppView is a correctness violation.

### 3e. `WriteTimeout` Too Short for WebSocket

```go
// main.go:658
WriteTimeout: 15 * time.Second,
```

The HTTP server's 15s write timeout can kill long-running WebSocket subscription connections mid-stream.

---

## 4. MEDIUM: Test Coverage Gaps

**Untested packages (no test files at all):**

| Package | Risk Level |
|---------|-----------|
| `cmd/hypergoat` | Low — integration tests cover some paths |
| `internal/database/migrations` | Medium — schema breakage risk |
| `internal/database/repositories` | **High — core data layer untested** |
| `internal/graphql/query` | Low — simple types |
| `internal/graphql/types` | Medium — type mapping edge cases |
| `internal/jetstream` | **High — real-time data pipeline untested** |
| `internal/server` | **High — OAuth handlers are security-critical** |
| `internal/workers` | Low — simple cleanup logic |

**7,000 lines of tests** but concentrated in OAuth middleware, lexicon parsing, and config. The most complex, business-critical code (repositories, OAuth handlers, jetstream consumer) is untested.

---

## 5. LOW: Code Quality & Style

### 5a. Inconsistent Error Handling

- Some errors silently logged: `slog.Warn("Failed...", "error", err)` then continue
- Some errors swallowed: `return nil //nolint:nilerr`
- `fmt.Printf` used instead of `slog` in one place (resolvers.go:98)

### 5b. `go 1.25` in go.mod

```
go 1.25
```

Go 1.25 doesn't exist yet (as of early 2026). This is either aspirational or incorrect.

### 5c. Magic Numbers

Hardcoded values throughout: batch sizes (100, 900, 1000), buffer sizes (100, 1000), timeouts. These should be constants or configurable.

### 5d. Missing CORS Headers

No CORS middleware configured. The GraphQL and admin endpoints will be unusable from browser-based clients without CORS.

---

## Improvement Plan (Prioritized)

### Phase 1: Security Hardening (1-2 weeks)

1. **Sanitize JSONExtract inputs** — validate field names against `^[a-zA-Z0-9_]+$`
2. **Fix backfillActive race** — use `atomic.Bool`
3. **Add auth enforcement to admin mutations** — require authentication for all write operations
4. **Make WebSocket origin configurable** — default to same-origin in production
5. **Add ZIP upload size limits** — cap at configurable max (e.g., 10MB)
6. **Add CORS middleware** — configurable for production/dev

### Phase 2: Architecture Cleanup (2-3 weeks)

1. **Decompose `main.go:run()`** — extract into `server.Setup()`, `server.StartWorkers()`, etc.
2. **Centralize all env vars in config** — eliminate direct `os.Getenv` calls
3. **De-duplicate shared code** — extract `extractCreatedAt`, `parseTimestamp`, `ParseCollections` to shared utils
4. **Add `BeginTx` to Executor interface** — enable proper transaction support
5. **Implement `scanRows`** — or remove the dead code and make the architecture decision explicit
6. **Replace global PubSub** — inject via constructor/context
7. **Fix PubSub subscriber IDs** — use `strconv.FormatInt`
8. **Extract resolver pagination** — deduplicate the two near-identical resolver functions

### Phase 3: Test Coverage (2-3 weeks)

1. **Repository tests** — table-driven tests for all CRUD operations, edge cases
2. **OAuth handler tests** — test the full authorization flow, token exchange, refresh, revocation
3. **Jetstream consumer tests** — mock the WebSocket, test event processing and cursor tracking
4. **Migration tests** — verify up/down migrations work cleanly
5. **Integration test for the full flow** — lexicon -> GraphQL schema -> query -> response

### Phase 4: Production Readiness (1-2 weeks)

1. **Fix pagination algorithm** — consider keyset pagination using (indexed_at, uri) composite cursor
2. **Add backpressure to event channel** — either block or use a growing buffer with limits
3. **Set appropriate WriteTimeout for WebSocket paths** — use per-route timeout or hijack
4. **Add structured health checks** — verify DB connectivity, Jetstream status
5. **Add Prometheus metrics** — request latency, event processing rate, DB pool utilization
6. **Add rate limiting** — on admin mutations, OAuth endpoints
7. **Add request logging correlation** — tie request IDs through the entire call chain

---

## What's Done Well

- **Clean package structure** — logical separation of concerns
- **Dual database support** — SQLite + PostgreSQL with proper dialect abstraction
- **Embedded migrations** — auto-run on startup, proper versioning
- **Graceful shutdown** — signal handling, context cancellation, deferred cleanup
- **AT Protocol integration** — CAR-based backfill with legacy fallback, cursor tracking, DID resolution caching
- **OAuth implementation** — DPoP, PKCE, proper token lifecycle
- **Dynamic GraphQL** — schema built from lexicon definitions is architecturally interesting
- **Reconnection logic** — exponential backoff in Jetstream consumer
- **`go vet` clean** — no static analysis warnings

---

## Summary of Findings by Severity

| Severity | Count | Key Items |
|----------|-------|-----------|
| Critical | 5 | SQL injection, race conditions, auth gaps |
| High | 6 | God function, env var sprawl, dead code, duplication |
| Medium | 5 | Missing transactions, pagination bugs, buffer drops |
| Low | 4 | Style inconsistencies, magic numbers, missing CORS |

This is a genuinely ambitious project with solid fundamentals. The issues identified are normal for a project at this stage. The improvement plan above should bring it to production quality.
