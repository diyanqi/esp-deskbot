// Package config loads service configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains all runtime settings for the data collection service.
type Config struct {
	// HTTP
	Addr            string
	TLSCertFile     string
	TLSKeyFile      string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	MaxBodyBytes    int64

	// IoTDA push callback
	PushToken        string
	SignatureMaxAge  time.Duration
	DevAllowUnsigned bool

	// Query API
	APIToken string

	// CORS (comma-separated origins, or "*")
	CORSAllowOrigins []string

	// Optional AI health report
	AIEnabled bool
	AIAPIURL  string
	AIModel   string
	AIAPIKey  string
	AITimeout time.Duration

	// Demo controls (scenario injection)
	DemoEnabled bool

	// PostgreSQL
	DatabaseURL      string
	DBMaxConns       int32
	DBMinConns       int32
	DBMaxConnLife    time.Duration
	DBConnectTimeout time.Duration
	RunMigrations    bool

	// Logging
	LogLevel string
	LogJSON  bool
}

// Load reads configuration from the environment and validates it.
func Load() (*Config, error) {
	cfg := &Config{
		Addr:             envString("LISTEN_ADDR", ":11294"),
		TLSCertFile:      envString("TLS_CERT_FILE", ""),
		TLSKeyFile:       envString("TLS_KEY_FILE", ""),
		ReadTimeout:      envDuration("HTTP_READ_TIMEOUT", 15*time.Second),
		WriteTimeout:     envDuration("HTTP_WRITE_TIMEOUT", 60*time.Second),
		IdleTimeout:      envDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout:  envDuration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
		MaxBodyBytes:     envInt64("HTTP_MAX_BODY_BYTES", 5<<20),
		PushToken:        strings.TrimSpace(os.Getenv("IOTDA_PUSH_TOKEN")),
		SignatureMaxAge:  envDuration("IOTDA_SIGNATURE_MAX_AGE", 10*time.Minute),
		DevAllowUnsigned: envBool("DEV_ALLOW_UNSIGNED_PUSH", false),
		APIToken:         strings.TrimSpace(os.Getenv("API_TOKEN")),
		CORSAllowOrigins: splitList(os.Getenv("CORS_ALLOW_ORIGINS")),
		AIEnabled:        envBool("AI_ENABLED", true),
		AIAPIURL:         envString("AI_API_URL", "https://ch.at/v1/chat/completions"),
		AIModel:          envString("AI_MODEL", "gpt-4o"),
		AIAPIKey:         strings.TrimSpace(os.Getenv("AI_API_KEY")),
		AITimeout:        envDuration("AI_TIMEOUT", 45*time.Second),
		DemoEnabled:      envBool("DEMO_ENABLED", false),
		DatabaseURL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
		DBMaxConns:       int32(envInt("DB_MAX_CONNS", 10)),
		DBMinConns:       int32(envInt("DB_MIN_CONNS", 1)),
		DBMaxConnLife:    envDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute),
		DBConnectTimeout: envDuration("DB_CONNECT_TIMEOUT", 10*time.Second),
		RunMigrations:    envBool("RUN_MIGRATIONS", true),
		LogLevel:         strings.ToLower(envString("LOG_LEVEL", "info")),
		LogJSON:          envBool("LOG_JSON", false),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	if cfg.PushToken == "" && !cfg.DevAllowUnsigned {
		return nil, errors.New("IOTDA_PUSH_TOKEN is required (or set DEV_ALLOW_UNSIGNED_PUSH=1 for local testing)")
	}
	if (cfg.TLSCertFile == "") != (cfg.TLSKeyFile == "") {
		return nil, errors.New("TLS_CERT_FILE and TLS_KEY_FILE must be set together")
	}
	if cfg.MaxBodyBytes <= 0 {
		return nil, errors.New("HTTP_MAX_BODY_BYTES must be positive")
	}
	if cfg.DBMaxConns < 1 {
		cfg.DBMaxConns = 1
	}
	if cfg.DBMinConns < 0 || cfg.DBMinConns > cfg.DBMaxConns {
		cfg.DBMinConns = 0
	}
	return cfg, nil
}

// SlogLevel converts the configured level string to slog.Level.
func (c *Config) SlogLevel() slog.Level {
	switch c.LogLevel {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// TLSEnabled reports whether the server should serve TLS directly.
func (c *Config) TLSEnabled() bool {
	return c.TLSCertFile != "" && c.TLSKeyFile != ""
}

func envString(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func envBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

// Describe returns a redacted human-readable summary for startup logs.
func (c *Config) Describe() string {
	tls := "disabled"
	if c.TLSEnabled() {
		tls = "enabled"
	}
	return fmt.Sprintf("addr=%s tls=%s migrations=%t push_auth=%t api_auth=%t ai=%t demo=%t",
		c.Addr, tls, c.RunMigrations, c.PushToken != "", c.APIToken != "", c.AIEnabled, c.DemoEnabled)
}
