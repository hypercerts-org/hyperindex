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
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/GainForest/hypergoat/internal/config"
	"github.com/GainForest/hypergoat/internal/database/migrations"
	"github.com/GainForest/hypergoat/internal/database/repositories"
	"github.com/GainForest/hypergoat/internal/server"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
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

	// Placeholder for GraphQL endpoint
	r.Route("/graphql", func(r chi.Router) {
		r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotImplemented)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "NotImplemented",
				"message": "GraphQL endpoint is not yet implemented",
			})
		})
	})

	// Create HTTP server
	srv := &http.Server{
		Addr:         cfg.Address(),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
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

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	slog.Info("Server stopped gracefully")
	return nil
}
