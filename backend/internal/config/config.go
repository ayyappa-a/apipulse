// Package config provides typed configuration loading from environment variables.
// No secret values are hardcoded here. All production values must come from environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Env represents the deployment environment.
type Env string

const (
	EnvDevelopment Env = "development"
	EnvStaging     Env = "staging"
	EnvProduction  Env = "production"
)

// Config is the fully resolved, typed configuration for the APIPulse API server.
// All fields are populated from environment variables with sane defaults for local dev.
type Config struct {
	// Application environment
	Env Env

	// HTTP server configuration
	HTTPAddr        string
	ShutdownTimeout time.Duration

	// Metrics server (Prometheus scrape endpoint)
	PrometheusAddr string

	// Database — PostgreSQL / TimescaleDB
	DatabaseURL string

	// StorageBackend — "postgres" or "memory"
	StorageBackend string

	// PostgreSQL connection pool limits
	PostgresMaxConns int32
	PostgresMinConns int32

	// Cache — Redis
	RedisURL string

	// Health Engine background evaluation interval
	HealthEvalInterval time.Duration

	// Health evaluation targets (comma-separated "dependency:vendor:region" or "dependency:region")
	HealthTargets []string

	// OpenTelemetry export endpoint (for APIPulse's own internal telemetry)
	OTELEndpoint string

	// CORS — comma-separated list of allowed origins
	CORSAllowedOrigins []string

	// Log level: debug | info | warn | error
	LogLevel string
}

// Load reads configuration from environment variables.
// It returns an error if any required variable is missing or invalid.
func Load() (*Config, error) {
	storageBackend := strings.ToLower(getEnv("APIPULSE_STORAGE", "postgres"))
	if storageBackend != "postgres" && storageBackend != "memory" {
		return nil, fmt.Errorf("invalid APIPULSE_STORAGE %q: must be 'postgres' or 'memory'", storageBackend)
	}

	evalInterval := 60 * time.Second
	if intervalStr := os.Getenv("APIPULSE_HEALTH_EVALUATION_INTERVAL"); intervalStr != "" {
		d, err := time.ParseDuration(intervalStr)
		if err != nil {
			return nil, fmt.Errorf("invalid APIPULSE_HEALTH_EVALUATION_INTERVAL %q: %w", intervalStr, err)
		}
		evalInterval = d
	}

	maxConns := int32(20)
	if s := os.Getenv("POSTGRES_MAX_CONNS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			maxConns = int32(n)
		}
	}

	minConns := int32(4)
	if s := os.Getenv("POSTGRES_MIN_CONNS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			minConns = int32(n)
		}
	}

	cfg := &Config{
		Env:                Env(getEnv("APIPULSE_ENV", "development")),
		HTTPAddr:           getEnv("APIPULSE_HTTP_ADDR", ":8080"),
		PrometheusAddr:     getEnv("PROMETHEUS_ADDR", ":9090"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://apipulse:apipulse@localhost:5432/apipulse?sslmode=disable"),
		StorageBackend:     storageBackend,
		PostgresMaxConns:   maxConns,
		PostgresMinConns:   minConns,
		RedisURL:           getEnv("REDIS_URL", "redis://localhost:6379"),
		HealthEvalInterval: evalInterval,
		HealthTargets:      splitCSV(getEnv("APIPULSE_HEALTH_TARGETS", "stripe-like:stripe:ap-south-1,stripe-like:stripe:us-east-1")),
		OTELEndpoint:       getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		CORSAllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173")),
		ShutdownTimeout:    30 * time.Second,
	}

	if s := os.Getenv("APIPULSE_SHUTDOWN_TIMEOUT_SECONDS"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("invalid APIPULSE_SHUTDOWN_TIMEOUT_SECONDS: %w", err)
		}
		cfg.ShutdownTimeout = time.Duration(n) * time.Second
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// validate performs semantic validation of the resolved configuration.
func (c *Config) validate() error {
	validEnvs := map[Env]bool{EnvDevelopment: true, EnvStaging: true, EnvProduction: true}
	if !validEnvs[c.Env] {
		return fmt.Errorf("invalid APIPULSE_ENV %q: must be one of development|staging|production", c.Env)
	}
	if c.HTTPAddr == "" {
		return fmt.Errorf("APIPULSE_HTTP_ADDR must not be empty")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL must not be empty")
	}
	return nil
}

// IsDevelopment returns true when running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.Env == EnvDevelopment
}

// getEnv returns the env variable value or the fallback default.
func getEnv(key, fallback string) string {
	if v, exists := os.LookupEnv(key); exists {
		return v
	}
	return fallback
}

// splitCSV splits a comma-separated string into a trimmed slice.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
