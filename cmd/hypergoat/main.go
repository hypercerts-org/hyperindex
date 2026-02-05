// Package main is the entry point for the Hypergoat server.
//
// Hypergoat is a Go implementation of Quickslice - an AT Protocol AppView server
// that indexes Lexicon-defined records and exposes them via a dynamically-generated
// GraphQL API.
//
// For more information, see the README.md file.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/GainForest/hypergoat/internal/backfill"
	"github.com/GainForest/hypergoat/internal/config"
	"github.com/GainForest/hypergoat/internal/database/migrations"
	"github.com/GainForest/hypergoat/internal/database/repositories"
	hgraphql "github.com/GainForest/hypergoat/internal/graphql"
	"github.com/GainForest/hypergoat/internal/graphql/admin"
	"github.com/GainForest/hypergoat/internal/graphql/resolver"
	"github.com/GainForest/hypergoat/internal/graphql/subscription"
	"github.com/GainForest/hypergoat/internal/jetstream"
	"github.com/GainForest/hypergoat/internal/lexicon"
	"github.com/GainForest/hypergoat/internal/oauth"
	"github.com/GainForest/hypergoat/internal/server"
	"github.com/GainForest/hypergoat/internal/workers"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

// loadLexiconsFromDir loads all lexicon JSON files from a directory tree.
func loadLexiconsFromDir(dir string, registry *lexicon.Registry) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		lex, parseErr := lexicon.ParseBytes(data)
		if parseErr != nil {
			// Skip non-lexicon JSON files
			return nil //nolint:nilerr // intentionally skip parse errors
		}

		registry.Register(lex)
		return nil
	})
}

func run() error {
	// Set up structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting Hypergoat - AT Protocol AppView Server")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	cfg.LogConfig()

	// Connect to database
	db, err := server.ConnectDatabase(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	slog.Info("Database connected successfully", "dialect", db.Dialect().String())

	// Run database migrations
	slog.Info("Running database migrations...")
	if err := migrations.Run(context.Background(), db); err != nil {
		return err
	}
	slog.Info("Database migrations complete")

	// Initialize repositories
	recordsRepo := repositories.NewRecordsRepository(db)
	actorsRepo := repositories.NewActorsRepository(db)
	lexiconsRepo := repositories.NewLexiconsRepository(db)
	configRepo := repositories.NewConfigRepository(db)

	// Initialize config defaults
	ctx := context.Background()
	if err := configRepo.InitializeDefaults(ctx); err != nil {
		slog.Warn("Failed to initialize config defaults", "error", err)
	}

	// Initialize admin DIDs from environment if not already set in database
	if adminDIDs := os.Getenv("ADMIN_DIDS"); adminDIDs != "" {
		existingAdmins := configRepo.GetAdminDIDs(ctx)
		if len(existingAdmins) == 0 {
			if err := configRepo.Set(ctx, "admin_dids", adminDIDs); err != nil {
				slog.Warn("Failed to set admin_dids from environment", "error", err)
			} else {
				slog.Info("Initialized admin DIDs from environment", "dids", adminDIDs)
			}
		}
	}

	// Auto-populate activity from existing records if activity table is empty
	activityRepo := repositories.NewJetstreamActivityRepository(db)
	go func() {
		recordCount, err := recordsRepo.GetCount(ctx)
		if err != nil {
			slog.Warn("Failed to get record count for activity population", "error", err)
			return
		}
		if recordCount == 0 {
			return // No records, nothing to populate
		}

		activityCount, err := activityRepo.GetCount(ctx)
		if err != nil {
			slog.Warn("Failed to get activity count", "error", err)
			return
		}
		if activityCount > 0 {
			return // Activity already populated
		}

		slog.Info("Populating activity from existing records...", "record_count", recordCount)
		populated, err := populateActivityFromRecords(ctx, recordsRepo, activityRepo)
		if err != nil {
			slog.Error("Failed to populate activity", "error", err)
		} else {
			slog.Info("Activity populated from existing records", "count", populated)
		}
	}()

	// Create router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Stats endpoint
	r.Get("/stats", func(w http.ResponseWriter, req *http.Request) {
		reqCtx := req.Context()

		recordCount, err := recordsRepo.GetCount(reqCtx)
		if err != nil {
			slog.Error("Failed to get record count", "error", err)
			recordCount = -1
		}

		actorCount, err := actorsRepo.GetCount(reqCtx)
		if err != nil {
			slog.Error("Failed to get actor count", "error", err)
			actorCount = -1
		}

		lexiconCount, err := lexiconsRepo.GetCount(reqCtx)
		if err != nil {
			slog.Error("Failed to get lexicon count", "error", err)
			lexiconCount = -1
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"records":  recordCount,
			"actors":   actorCount,
			"lexicons": lexiconCount,
			"time":     time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Root endpoint - server info
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"name":        "Hypergoat",
			"description": "AT Protocol AppView Server",
			"version":     "0.1.0-dev",
			"docs":        cfg.ExternalBaseURL + "/docs",
		})
	})

	// Placeholder for XRPC endpoints (AT Protocol)
	r.Route("/xrpc", func(r chi.Router) {
		r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotImplemented)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "NotImplemented",
				"message": "XRPC endpoints are not yet implemented",
			})
		})
	})

	// OAuth endpoints
	oauthSigningKey, _ := oauth.GenerateDPoPKeyPair() // Generate ephemeral key if not configured
	if cfg.OAuthSigningKey != "" {
		if key, err := oauth.ParseDPoPKeyPair(cfg.OAuthSigningKey); err == nil {
			oauthSigningKey = key
		} else {
			slog.Warn("Failed to parse OAuth signing key, using ephemeral key", "error", err)
		}
	}

	oauthHandlers := server.NewOAuthHandlers(server.OAuthHandlerConfig{
		ExternalBaseURL:             cfg.ExternalBaseURL,
		ClientID:                    cfg.ExternalBaseURL + "/oauth-client-metadata.json",
		CallbackURL:                 cfg.ExternalBaseURL + "/oauth/callback",
		SigningKey:                  oauthSigningKey,
		Issuer:                      cfg.ExternalBaseURL,
		ScopesSupported:             []string{"atproto", "transition:generic", "transition:chat.bsky"},
		AccessTokenExpiration:       3600,    // 1 hour
		RefreshTokenExpiration:      1209600, // 14 days
		AuthorizationCodeExpiration: 600,     // 10 minutes
	}, db)

	// OAuth discovery endpoints (/.well-known/*)
	r.Get("/.well-known/oauth-authorization-server", oauthHandlers.HandleAuthorizationServerMetadata)
	r.Get("/.well-known/oauth-protected-resource", oauthHandlers.HandleProtectedResourceMetadata)

	// OAuth client metadata (this server as an OAuth client)
	r.Get("/oauth-client-metadata.json", server.HandleClientMetadata(server.ClientMetadataConfig{
		ExternalBaseURL: cfg.ExternalBaseURL,
		ClientName:      "Hypergoat",
		Scope:           "atproto transition:generic",
	}))

	// OAuth flow endpoints
	r.Get("/oauth/authorize", oauthHandlers.HandleAuthorize)
	r.Post("/oauth/authorize", oauthHandlers.HandleAuthorize)
	r.Get("/oauth/callback", oauthHandlers.HandleCallback)
	r.Post("/oauth/token", oauthHandlers.HandleToken)
	r.Get("/oauth/jwks", oauthHandlers.HandleJWKS)
	r.Post("/oauth/revoke", oauthHandlers.HandleRevoke)

	slog.Info("OAuth endpoints enabled",
		"authorization_server", cfg.ExternalBaseURL+"/.well-known/oauth-authorization-server",
		"client_metadata", cfg.ExternalBaseURL+"/oauth-client-metadata.json",
	)

	// Additional OAuth endpoints
	registerHandler := server.NewOAuthRegisterHandler(db)
	r.Post("/oauth/register", registerHandler.HandleRegister)

	parHandler := server.NewOAuthPARHandler(db)
	r.Post("/oauth/par", parHandler.HandlePAR)

	r.Get("/oauth/dpop/nonce", server.HandleDPoPNonce)
	r.Post("/oauth/dpop/nonce", server.HandleDPoPNonce)

	// Start OAuth cleanup worker (clean up expired tokens every hour)
	oauthCleanupCtx, oauthCleanupCancel := context.WithCancel(context.Background())
	defer oauthCleanupCancel()
	oauthHandlers.StartCleanupWorker(oauthCleanupCtx, 1*time.Hour)

	// Admin GraphQL endpoint
	adminRepos := &admin.Repositories{
		Records:          recordsRepo,
		Actors:           actorsRepo,
		Lexicons:         lexiconsRepo,
		Config:           configRepo,
		OAuthClients:     repositories.NewOAuthClientsRepository(db),
		Activity:         repositories.NewJetstreamActivityRepository(db),
		Labels:           repositories.NewLabelsRepository(db),
		LabelDefinitions: repositories.NewLabelDefinitionsRepository(db),
		LabelPreferences: repositories.NewLabelPreferencesRepository(db),
		Reports:          repositories.NewReportsRepository(db),
	}

	authMiddleware := oauth.NewAuthMiddleware(
		repositories.NewOAuthAccessTokensRepository(db),
		repositories.NewOAuthDPoPJTIRepository(db),
		cfg.ExternalBaseURL,
	)

	// Get domain DID from config or use a placeholder
	domainDID := os.Getenv("DOMAIN_DID")
	if domainDID == "" {
		domainDID = "did:web:" + cfg.Host // Derive from host
	}

	adminHandler, err := admin.NewHandler(adminRepos, authMiddleware, configRepo, domainDID, cfg.TrustProxyHeaders)
	if err != nil {
		slog.Error("Failed to create admin GraphQL handler", "error", err)
	} else {
		// Wire up backfill callback for single-actor backfill from admin UI
		backfillConfig := backfill.DefaultConfig()
		backfillConfig.Collections = backfill.ParseCollections(os.Getenv("BACKFILL_COLLECTIONS"))
		if backfillConfig.Collections == nil {
			backfillConfig.Collections = backfill.ParseCollections(os.Getenv("JETSTREAM_COLLECTIONS"))
		}
		if relayURL := os.Getenv("BACKFILL_RELAY_URL"); relayURL != "" {
			backfillConfig.RelayURL = relayURL
		}
		if plcURL := os.Getenv("BACKFILL_PLC_URL"); plcURL != "" {
			backfillConfig.PLCURL = plcURL
		}

		backfillActivityRepo := repositories.NewJetstreamActivityRepository(db)
		actorBackfiller := backfill.NewBackfiller(backfillConfig, recordsRepo, actorsRepo, backfillActivityRepo)

		// Single actor backfill callback
		adminHandler.Resolver().SetBackfillCallback(func(ctx context.Context, did string) error {
			_, err := actorBackfiller.BackfillActor(ctx, did)
			return err
		})

		// Full network backfill callback (runs in background)
		adminHandler.Resolver().SetFullBackfillCallback(func(ctx context.Context) error {
			// Get collections from registered lexicons if not configured via env
			collections := backfillConfig.Collections
			if len(collections) == 0 {
				lexicons, err := lexiconsRepo.GetAll(ctx)
				if err != nil {
					slog.Error("[backfill] Failed to get lexicons", "error", err)
					return err
				}
				for _, lex := range lexicons {
					collections = append(collections, lex.ID)
				}
			}

			if len(collections) == 0 {
				slog.Warn("[backfill] No collections configured - register lexicons first or set BACKFILL_COLLECTIONS")
				return nil
			}

			// Create a new backfiller with the discovered collections
			bfConfig := backfillConfig
			bfConfig.Collections = collections
			bf := backfill.NewBackfiller(bfConfig, recordsRepo, actorsRepo, backfillActivityRepo)
			defer bf.Close()

			slog.Info("[backfill] Starting full network backfill", "collections", collections)
			stats, err := bf.Run(ctx)
			if err != nil {
				slog.Error("[backfill] Full backfill failed", "error", err)
				return err
			}
			slog.Info("[backfill] Full backfill completed",
				"repos_discovered", stats.ReposDiscovered,
				"repos_processed", stats.ReposProcessed,
				"records_inserted", stats.RecordsInserted,
				"duration", stats.Duration(),
			)
			return nil
		})

		slog.Info("Backfill callbacks configured for admin UI")

		// Admin endpoint with optional auth (allows introspection without auth)
		r.Handle("/admin/graphql", adminHandler.OptionalAuth())
		r.Handle("/admin/graphql/", adminHandler.OptionalAuth())
		slog.Info("Admin GraphQL endpoint enabled", "path", "/admin/graphql")
	}

	// GraphiQL playgrounds
	r.Get("/graphiql", server.HandleGraphiQL(server.GraphiQLConfig{
		Endpoint:             cfg.ExternalBaseURL + "/graphql",
		SubscriptionEndpoint: strings.Replace(cfg.ExternalBaseURL, "http", "ws", 1) + "/graphql/ws",
		Title:                "Hypergoat GraphQL",
		DefaultQuery: `# Hypergoat GraphQL API
# 
# Explore the AT Protocol data indexed by this AppView.
# Try querying for records from your configured lexicons.
#
# Example:
{
  __schema {
    types {
      name
    }
  }
}
`,
	}))

	r.Get("/graphiql/admin", server.HandleGraphiQL(server.GraphiQLConfig{
		Endpoint: cfg.ExternalBaseURL + "/admin/graphql",
		Title:    "Hypergoat Admin",
		DefaultQuery: `# Hypergoat Admin API
#
# Administrative operations for managing the AppView.
# Note: Some operations require authentication.
#
# Example:
{
  statistics {
    recordCount
    actorCount
    lexiconCount
  }
}
`,
	}))

	slog.Info("GraphiQL playgrounds enabled",
		"public", cfg.ExternalBaseURL+"/graphiql",
		"admin", cfg.ExternalBaseURL+"/graphiql/admin",
	)

	// Start background workers
	activityCleanupWorker := workers.NewActivityCleanupWorker(
		repositories.NewJetstreamActivityRepository(db),
	)
	workersCtx, workersCancel := context.WithCancel(context.Background())
	defer workersCancel()
	activityCleanupWorker.Start(workersCtx)

	// Load lexicons and set up GraphQL endpoint
	registry := lexicon.NewRegistry()
	lexiconDir := os.Getenv("LEXICON_DIR")
	if lexiconDir == "" {
		// Default to testdata/lexicons for development
		lexiconDir = "testdata/lexicons"
	}

	if _, err := os.Stat(lexiconDir); err == nil {
		if err := loadLexiconsFromDir(lexiconDir, registry); err != nil {
			slog.Warn("Failed to load lexicons from directory", "dir", lexiconDir, "error", err)
		} else {
			slog.Info("Loaded lexicons from directory", "count", registry.Count(), "dir", lexiconDir)
		}
	}

	// Also load lexicons from the database (uploaded via admin UI)
	dbLexicons, err := lexiconsRepo.GetAll(ctx)
	if err != nil {
		slog.Warn("Failed to load lexicons from database", "error", err)
	} else if len(dbLexicons) > 0 {
		dbLoaded := 0
		for _, dbLex := range dbLexicons {
			if _, err := registry.ParseAndRegister(dbLex.JSON); err != nil {
				slog.Warn("Failed to parse database lexicon", "id", dbLex.ID, "error", err)
			} else {
				dbLoaded++
			}
		}
		slog.Info("Loaded lexicons from database", "count", dbLoaded, "total", len(dbLexicons))
	}

	slog.Info("Total lexicons registered", "count", registry.Count())

	// Create GraphQL handler with database repositories
	repos := &resolver.Repositories{
		Records:  recordsRepo,
		Actors:   actorsRepo,
		Lexicons: lexiconsRepo,
	}
	graphqlHandler, err := hgraphql.NewHandler(registry, repos)
	if err != nil {
		slog.Error("Failed to create GraphQL handler", "error", err)
	} else {
		r.Handle("/graphql", graphqlHandler)
		r.Handle("/graphql/", graphqlHandler)
		slog.Info("GraphQL endpoint enabled", "path", "/graphql")

		// Add WebSocket subscription endpoint
		var allowedOrigins []string
		if cfg.AllowedOrigins != "" {
			allowedOrigins = strings.Split(cfg.AllowedOrigins, ",")
			for i := range allowedOrigins {
				allowedOrigins[i] = strings.TrimSpace(allowedOrigins[i])
			}
		}
		subscriptionHandler := subscription.NewHandler(graphqlHandler.Schema(), allowedOrigins)
		r.Handle("/graphql/ws", subscriptionHandler)
		slog.Info("GraphQL subscriptions enabled", "path", "/graphql/ws")
	}

	// Start Jetstream consumer if collections are configured
	var jsConsumer *jetstream.Consumer
	jsCollections := os.Getenv("JETSTREAM_COLLECTIONS")

	// If no env var, try to get collections from registered lexicons
	var collections []string
	if jsCollections != "" {
		collections = jetstream.ParseCollections(jsCollections)
	} else {
		// Read from database lexicons
		lexicons, err := lexiconsRepo.GetAll(ctx)
		if err != nil {
			slog.Warn("Failed to get lexicons for Jetstream", "error", err)
		} else {
			for _, lex := range lexicons {
				collections = append(collections, lex.ID)
			}
		}
	}

	if len(collections) > 0 {
		jsURL := os.Getenv("JETSTREAM_URL")
		if jsURL == "" {
			jsURL = jetstream.DefaultJetstreamURL
		}
		disableCursor := os.Getenv("JETSTREAM_DISABLE_CURSOR") == "true"

		activityRepo := repositories.NewJetstreamActivityRepository(db)
		jsConsumer = jetstream.NewConsumer(
			jetstream.ConsumerConfig{
				JetstreamURL:  jsURL,
				Collections:   collections,
				DisableCursor: disableCursor,
			},
			recordsRepo,
			actorsRepo,
			configRepo,
			activityRepo,
		)

		// Start consumer in background
		jsCtx, jsCancel := context.WithCancel(context.Background())
		defer jsCancel()

		go func() {
			slog.Info("Starting Jetstream consumer",
				"url", jsURL,
				"collections", collections,
				"disable_cursor", disableCursor,
			)
			if err := jsConsumer.Start(jsCtx); err != nil {
				slog.Error("Jetstream consumer error", "error", err)
			}
		}()
	} else {
		slog.Info("Jetstream consumer disabled (no collections - register lexicons or set JETSTREAM_COLLECTIONS)")
	}

	// Set up lexicon change callback for dynamic Jetstream updates
	if adminHandler != nil {
		adminHandler.Resolver().SetLexiconChangeCallback(func(collections []string) error {
			if jsConsumer == nil {
				// Create consumer if it doesn't exist yet
				jsURL := os.Getenv("JETSTREAM_URL")
				if jsURL == "" {
					jsURL = jetstream.DefaultJetstreamURL
				}
				disableCursor := os.Getenv("JETSTREAM_DISABLE_CURSOR") == "true"
				activityRepo := repositories.NewJetstreamActivityRepository(db)

				jsConsumer = jetstream.NewConsumer(
					jetstream.ConsumerConfig{
						JetstreamURL:  jsURL,
						Collections:   collections,
						DisableCursor: disableCursor,
					},
					recordsRepo,
					actorsRepo,
					configRepo,
					activityRepo,
				)

				// Start consumer in background
				go func() {
					slog.Info("Starting Jetstream consumer (dynamic)",
						"collections", collections,
					)
					if err := jsConsumer.Start(context.Background()); err != nil {
						slog.Error("Jetstream consumer error", "error", err)
					}
				}()
				return nil
			}
			return jsConsumer.UpdateCollections(collections)
		})
		slog.Info("Lexicon change callback configured for dynamic Jetstream updates")
	}

	// Run backfill if enabled
	if os.Getenv("BACKFILL_ON_START") == "true" {
		backfillCollections := os.Getenv("BACKFILL_COLLECTIONS")
		if backfillCollections == "" {
			backfillCollections = jsCollections // Default to jetstream collections
		}

		if backfillCollections != "" {
			collections := backfill.ParseCollections(backfillCollections)
			relayURL := os.Getenv("BACKFILL_RELAY_URL")
			plcURL := os.Getenv("BACKFILL_PLC_URL")

			bfConfig := backfill.DefaultConfig()
			bfConfig.Collections = collections
			if relayURL != "" {
				bfConfig.RelayURL = relayURL
			}
			if plcURL != "" {
				bfConfig.PLCURL = plcURL
			}

			startupActivityRepo := repositories.NewJetstreamActivityRepository(db)
			backfiller := backfill.NewBackfiller(bfConfig, recordsRepo, actorsRepo, startupActivityRepo)

			// Run backfill in background
			go func() {
				slog.Info("Starting backfill operation",
					"collections", collections,
					"relay", bfConfig.RelayURL,
				)
				stats, err := backfiller.Run(context.Background())
				if err != nil {
					slog.Error("Backfill failed", "error", err)
				} else {
					slog.Info("Backfill completed",
						"repos_discovered", stats.ReposDiscovered,
						"repos_processed", stats.ReposProcessed,
						"records_inserted", stats.RecordsInserted,
						"duration", stats.Duration(),
					)
				}
			}()
		} else {
			slog.Warn("BACKFILL_ON_START=true but no collections specified")
		}
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:        cfg.Address(),
		Handler:     r,
		ReadTimeout: 15 * time.Second,
		// WriteTimeout disabled (set to 0) to support long-lived WebSocket connections.
		// Individual handlers enforce their own write deadlines:
		// - WebSocket: per-message deadline in subscription/handler.go
		// - HTTP: standard response lifecycle
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("Server listening", "address", cfg.Address(), "url", cfg.ExternalBaseURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return err
	case <-quit:
	}

	slog.Info("Shutting down server...")

	// Stop background workers
	workersCancel()

	// Stop Jetstream consumer
	if jsConsumer != nil {
		jsConsumer.Stop()
	}

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	slog.Info("Server stopped gracefully")
	return nil
}

// populateActivityFromRecords creates activity entries from existing records.
func populateActivityFromRecords(
	ctx context.Context,
	recordsRepo *repositories.RecordsRepository,
	activityRepo *repositories.JetstreamActivityRepository,
) (int64, error) {
	var count int64
	_, err := recordsRepo.IterateAll(ctx, 1000, func(rec *repositories.Record) error {
		// Extract createdAt from the record JSON, fall back to IndexedAt
		timestamp := extractCreatedAtFromJSON(rec.JSON, rec.IndexedAt)

		// Log as a successful create operation
		_, err := activityRepo.LogActivityWithStatus(ctx, timestamp, "create", rec.Collection, rec.DID, rec.RKey, rec.JSON, "success")
		if err != nil {
			return nil // Continue on error
		}
		count++
		return nil
	})
	return count, err
}

// extractCreatedAtFromJSON extracts the createdAt timestamp from a record's JSON.
// Falls back to the provided fallback time if not found.
func extractCreatedAtFromJSON(recordJSON string, fallback time.Time) time.Time {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(recordJSON), &data); err != nil {
		return fallback
	}

	// Try common timestamp field names
	for _, field := range []string{"createdAt", "$createdAt", "created_at", "timestamp", "indexedAt"} {
		if val, ok := data[field].(string); ok {
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				return t
			}
			if t, err := time.Parse("2006-01-02T15:04:05", val); err == nil {
				return t
			}
		}
	}

	return fallback
}
