# [Epic] Port Quickslice to Go - Hypergoat Implementation

## Overview

This epic tracks the implementation of **Hypergoat**, a Go port of [Quickslice](https://github.com/quickslice/quickslice) - an AT Protocol AppView server that indexes Lexicon-defined records and exposes them via a dynamically-generated GraphQL API.

**Goal:** Feature parity with Quickslice while leveraging Go's performance characteristics and ecosystem.

**Estimated Timeline:** 8-10 weeks

---

## Background

Quickslice is written in Gleam (targeting Erlang/OTP) and provides:
- Dynamic GraphQL schema generation from AT Protocol Lexicons
- Real-time sync via Jetstream WebSocket
- OAuth 2.0 server with DPoP and AT Protocol bridge
- Multi-database support (SQLite/PostgreSQL)
- GraphQL subscriptions via WebSocket
- Batch backfill from AT Protocol relays

Hypergoat will maintain API compatibility so existing quickslice-client-js clients work unchanged.

---

## Technology Decisions

| Component | Library | Rationale |
|-----------|---------|-----------|
| HTTP | chi | Lightweight, idiomatic, good middleware |
| Database | pgx + modernc/sqlite | Native drivers, no CGO for SQLite |
| GraphQL | graphql-go/graphql | Runtime schema generation |
| WebSocket | nhooyr/websocket | Modern, context-aware |
| JWT/JOSE | golang-jwt + go-jose | Industry standard |
| Config | caarlos0/env | Struct tags, validation |

---

## Implementation Phases

### Phase 1: Foundation (Week 1-2)
- [ ] Project setup (go.mod, Makefile, CI)
- [ ] Configuration management
- [ ] Database executor abstraction (SQLite + PostgreSQL)
- [ ] Core repositories (records, actors, lexicons, config)
- [ ] Database migrations

### Phase 2: Lexicon & GraphQL Core (Week 2-3)
- [ ] Lexicon JSON parser
- [ ] Lexicon registry for cross-references
- [ ] NSID utilities
- [ ] GraphQL type mapping
- [ ] WHERE input types
- [ ] Connection types (Relay spec)
- [ ] Multi-pass schema builder

### Phase 3: GraphQL API (Week 3-4)
- [ ] Record fetcher interface
- [ ] Query resolvers with pagination
- [ ] DataLoader for N+1 prevention
- [ ] Join field resolvers (forward, reverse, DID)
- [ ] Mutation resolvers
- [ ] Aggregation support
- [ ] GraphQL HTTP handler

### Phase 4: Real-time Features (Week 4-5)
- [ ] PubSub system
- [ ] Jetstream WebSocket consumer
- [ ] Manager/supervisor with heartbeat
- [ ] Cursor tracking with batched persistence
- [ ] Event handler (validate, store, publish)
- [ ] GraphQL subscriptions (graphql-ws protocol)

### Phase 5: OAuth & Authentication (Week 5-6)
- [ ] OAuth repositories (10 tables)
- [ ] OAuth 2.0 server endpoints
- [ ] PKCE implementation
- [ ] DPoP implementation
- [ ] DID resolver with caching
- [ ] AT Protocol bridge (initiate, callback, refresh)
- [ ] Auth middleware

### Phase 6: Backfill & Admin (Week 6-7)
- [ ] Backfill worker with semaphore
- [ ] CAR file parser (CBOR + MST)
- [ ] Backfill state tracking
- [ ] Admin GraphQL schema
- [ ] Admin queries (stats, settings, activity)
- [ ] Admin mutations (config, backfill, lexicons)
- [ ] Activity cleanup worker

### Phase 7: Polish & Integration (Week 7-8)
- [ ] Full HTTP router setup
- [ ] Graceful shutdown
- [ ] Health check endpoint
- [ ] Static file serving (Quickslice client)
- [ ] Docker + docker-compose
- [ ] API compatibility testing
- [ ] Documentation

---

## Acceptance Criteria

- [ ] All Quickslice GraphQL queries work identically
- [ ] OAuth flows work with Bluesky PDS
- [ ] Jetstream sync maintains cursor across restarts
- [ ] Backfill completes successfully with real data
- [ ] Admin UI functions correctly
- [ ] Both SQLite and PostgreSQL work
- [ ] Docker deployment works
- [ ] All tests pass (unit + integration)

---

## Key Files to Port

### From `quickslice/server/src/`
| Source | Destination |
|--------|-------------|
| `server.gleam` | `cmd/hypergoat/main.go` |
| `database/executor.gleam` | `internal/database/executor.go` |
| `database/repositories/*.gleam` | `internal/database/repositories/*.go` |
| `jetstream_consumer.gleam` | `internal/jetstream/consumer.go` |
| `event_handler.gleam` | `internal/jetstream/handler.go` |
| `backfill.gleam` | `internal/backfill/worker.go` |
| `pubsub.gleam` | `internal/pubsub/pubsub.go` |
| `handlers/*.gleam` | `internal/handlers/*.go` |
| `lib/oauth/*.gleam` | `internal/oauth/*.go` |
| `graphql/**/*.gleam` | `internal/graphql/**/*.go` |

### From `quickslice/lexicon_graphql/src/`
| Source | Destination |
|--------|-------------|
| `types.gleam` | `internal/lexicon/types.go` |
| `internal/lexicon/parser.gleam` | `internal/lexicon/parser.go` |
| `internal/lexicon/registry.gleam` | `internal/lexicon/registry.go` |
| `schema/builder.gleam` | `internal/graphql/schema/builder.go` |
| `schema/database.gleam` | `internal/graphql/schema/database.go` |
| `input/where.gleam` | `internal/graphql/where/types.go` |
| `mutation/builder.gleam` | `internal/graphql/resolvers/mutation.go` |
| `query/dataloader.gleam` | `internal/graphql/dataloader/loader.go` |

### From `quickslice/atproto_car/`
| Source | Destination |
|--------|-------------|
| CAR parsing | `internal/backfill/car/parser.go` |

---

## Related Issues

- #2 Phase 1: Foundation
- #3 Phase 2: Lexicon & GraphQL Core
- #4 Phase 3: GraphQL API
- #5 Phase 4: Real-time Features
- #6 Phase 5: OAuth & Authentication
- #7 Phase 6: Backfill & Admin
- #8 Phase 7: Polish & Integration

---

## Resources

- [Quickslice Repository](https://github.com/quickslice/quickslice)
- [AT Protocol Documentation](https://atproto.com/docs)
- [Lexicon Specification](https://atproto.com/specs/lexicon)
- [Jetstream Documentation](https://docs.bsky.app/docs/advanced-guides/firehose)
- [graphql-go Documentation](https://github.com/graphql-go/graphql)

---

## Labels

`epic`, `go`, `port`, `atproto`, `graphql`
