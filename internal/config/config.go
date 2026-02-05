// Package config handles application configuration loading from environment variables.
package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	// Server configuration
	Host string
	Port int

	// Database
	DatabaseURL string

	// Security
	SecretKeyBase string

	// OAuth
	ExternalBaseURL   string
	OAuthSigningKey   string
	OAuthLoopbackMode bool

	// Jetstream
	JetstreamDisableCursor bool

	// Backfill
	BackfillPDSConcurrency    int
	BackfillMaxPDSWorkers     int
	BackfillMaxHTTPConcurrent int
	BackfillRepoTimeoutMS     int

	// Security
	TrustProxyHeaders bool   // Trust X-User-DID header from reverse proxy (default: false, DANGEROUS if true without proxy)
	AllowedOrigins    string // Comma-separated allowed WebSocket/CORS origins (empty = same-origin only, "*" = allow all)
}

// Load reads configuration from environment variables.
// It loads .env file if present and applies defaults.
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()

	cfg := &Config{
		Host:                      getEnv("HOST", "127.0.0.1"),
		Port:                      getEnvInt("PORT", 8080),
		DatabaseURL:               getEnv("DATABASE_URL", "sqlite:data/hypergoat.db"),
		SecretKeyBase:             getEnv("SECRET_KEY_BASE", ""),
		ExternalBaseURL:           getEnv("EXTERNAL_BASE_URL", ""),
		OAuthSigningKey:           getEnv("OAUTH_SIGNING_KEY", ""),
		OAuthLoopbackMode:         getEnvBool("OAUTH_LOOPBACK_MODE", false),
		JetstreamDisableCursor:    getEnvBool("JETSTREAM_DISABLE_CURSOR", false),
		BackfillPDSConcurrency:    getEnvInt("BACKFILL_PDS_CONCURRENCY", 4),
		BackfillMaxPDSWorkers:     getEnvInt("BACKFILL_MAX_PDS_WORKERS", 10),
		BackfillMaxHTTPConcurrent: getEnvInt("BACKFILL_MAX_HTTP_CONCURRENT", 50),
		BackfillRepoTimeoutMS:     getEnvInt("BACKFILL_REPO_TIMEOUT", 60000),
		TrustProxyHeaders:         getEnvBool("TRUST_PROXY_HEADERS", false),
		AllowedOrigins:            getEnv("ALLOWED_ORIGINS", ""),
	}

	// Generate SecretKeyBase if not provided
	if cfg.SecretKeyBase == "" {
		slog.Warn("SECRET_KEY_BASE not set, generating random key",
			"warning", "Sessions will be invalidated on server restart")
		key, err := generateRandomKey(64)
		if err != nil {
			return nil, fmt.Errorf("failed to generate secret key: %w", err)
		}
		cfg.SecretKeyBase = key
	}

	// Set default external base URL if not provided
	if cfg.ExternalBaseURL == "" {
		cfg.ExternalBaseURL = fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
	}

	return cfg, nil
}

// Validate checks that all required configuration is present and valid.
func (c *Config) Validate() error {
	if len(c.SecretKeyBase) < 64 {
		return fmt.Errorf("SECRET_KEY_BASE must be at least 64 characters")
	}

	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("PORT must be between 1 and 65535")
	}

	return nil
}

// LogConfig logs the configuration (with sensitive values redacted).
func (c *Config) LogConfig() {
	slog.Info("Configuration loaded",
		"host", c.Host,
		"port", c.Port,
		"database_url", redactURL(c.DatabaseURL),
		"external_base_url", c.ExternalBaseURL,
		"oauth_loopback_mode", c.OAuthLoopbackMode,
		"oauth_signing_key_set", c.OAuthSigningKey != "",
		"jetstream_disable_cursor", c.JetstreamDisableCursor,
		"trust_proxy_headers", c.TrustProxyHeaders,
		"allowed_origins", c.AllowedOrigins,
	)

	if c.TrustProxyHeaders {
		slog.Warn("TRUST_PROXY_HEADERS is enabled: X-User-DID header will be trusted for authentication. " +
			"Only enable this when running behind a trusted reverse proxy that sets this header.")
	}
}

// Address returns the server address in host:port format.
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		lower := strings.ToLower(value)
		return lower == "true" || lower == "1" || lower == "yes"
	}
	return defaultValue
}

func generateRandomKey(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

func redactURL(url string) string {
	// Redact password in database URLs.
	// Example: postgres://user:pass@host becomes postgres://user:***@host
	if strings.Contains(url, "@") {
		parts := strings.SplitN(url, "@", 2)
		if len(parts) == 2 {
			prefix := parts[0]
			if idx := strings.LastIndex(prefix, ":"); idx > 0 {
				// Find the protocol separator
				if protoIdx := strings.Index(prefix, "://"); protoIdx > 0 && idx > protoIdx {
					return prefix[:idx+1] + "***@" + parts[1]
				}
			}
		}
	}
	return url
}
