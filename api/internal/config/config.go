// Package config loads server configuration from the environment.
package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully resolved server configuration. It is passed explicitly
// to the components that need it; nothing here is read from a global.
type Config struct {
	// Addr is the TCP address the HTTP server listens on, e.g. ":8080".
	Addr string
	// DatabaseURL is a libpq/pgx connection string.
	DatabaseURL string
	// LogLevel controls the slog handler level.
	LogLevel slog.Level
	// LogFormat is "json" or "text".
	LogFormat string
	// CORSOrigins lists the browser origins allowed to call the API.
	// A single "*" allows any origin (fine for a single-user local app).
	CORSOrigins []string
	// MigrateOnStart applies pending migrations during boot.
	MigrateOnStart bool
	// ShutdownTimeout bounds graceful shutdown.
	ShutdownTimeout time.Duration
	// RequestTimeout bounds a single request's handler.
	RequestTimeout time.Duration
	// TaggerURL is the full URL of the tagging service's endpoint, e.g.
	// https://tagger.example.com/tag. Empty disables automatic tagging, and
	// the UI hides the button rather than offering one that cannot work.
	TaggerURL string
	// TaggerTimeout bounds one call to the tagging service. It is generous by
	// default because the service is noticeably slower on a cold start than
	// when warm, and a suggestion that arrives late still beats a spurious
	// failure.
	TaggerTimeout time.Duration
}

// AutoTaggingEnabled reports whether a tagging service is configured.
func (c Config) AutoTaggingEnabled() bool { return c.TaggerURL != "" }

// Load reads the configuration from the process environment, applying
// defaults suited to local development.
func Load() (Config, error) {
	cfg := Config{
		Addr:            env("IDPAD_ADDR", ":8080"),
		DatabaseURL:     env("DATABASE_URL", ""),
		LogFormat:       env("IDPAD_LOG_FORMAT", "json"),
		CORSOrigins:     splitAndTrim(env("IDPAD_CORS_ORIGINS", "*")),
		ShutdownTimeout: 15 * time.Second,
		RequestTimeout:  30 * time.Second,
		TaggerURL:       env("IDPAD_TAGGER_URL", ""),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.TaggerURL != "" {
		if err := validateTaggerURL(cfg.TaggerURL); err != nil {
			return Config{}, fmt.Errorf("IDPAD_TAGGER_URL: %w", err)
		}
	}

	taggerTimeout, err := time.ParseDuration(env("IDPAD_TAGGER_TIMEOUT", "45s"))
	if err != nil {
		return Config{}, fmt.Errorf("IDPAD_TAGGER_TIMEOUT: invalid duration %q", env("IDPAD_TAGGER_TIMEOUT", ""))
	}
	// The upper bound keeps the budget under the server's write timeout, which
	// is derived from it; the lower bound rejects a value that could never
	// complete a call.
	if taggerTimeout < time.Second || taggerTimeout > 2*time.Minute {
		return Config{}, fmt.Errorf("IDPAD_TAGGER_TIMEOUT must be between 1s and 2m, got %s", taggerTimeout)
	}
	cfg.TaggerTimeout = taggerTimeout

	level, err := parseLevel(env("IDPAD_LOG_LEVEL", "info"))
	if err != nil {
		return Config{}, err
	}
	cfg.LogLevel = level

	migrate, err := parseBool(env("IDPAD_MIGRATE_ON_START", "true"))
	if err != nil {
		return Config{}, fmt.Errorf("IDPAD_MIGRATE_ON_START: %w", err)
	}
	cfg.MigrateOnStart = migrate

	if cfg.LogFormat != "json" && cfg.LogFormat != "text" {
		return Config{}, fmt.Errorf("IDPAD_LOG_FORMAT must be json or text, got %q", cfg.LogFormat)
	}

	return cfg, nil
}

// AllowsOrigin reports whether the given browser origin may call the API.
func (c Config) AllowsOrigin(origin string) bool {
	for _, allowed := range c.CORSOrigins {
		if allowed == "*" || strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}

// AllowsAnyOrigin reports whether the wildcard origin is configured.
func (c Config) AllowsAnyOrigin() bool {
	for _, allowed := range c.CORSOrigins {
		if allowed == "*" {
			return true
		}
	}
	return false
}

// validateTaggerURL rejects a misconfigured tagger endpoint at boot rather
// than on the first request. Only absolute http(s) URLs can be called.
func validateTaggerURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("not a valid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("must start with http:// or https://")
	}
	if parsed.Host == "" {
		return fmt.Errorf("must include a host")
	}
	return nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func parseBool(v string) (bool, error) {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("invalid boolean %q", v)
	}
	return b, nil
}

func parseLevel(v string) (slog.Level, error) {
	switch strings.ToLower(v) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("IDPAD_LOG_LEVEL must be one of debug, info, warn, error; got %q", v)
	}
}

func splitAndTrim(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
