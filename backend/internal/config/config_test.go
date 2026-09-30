package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear all relevant env vars to test defaults.
	keys := []string{
		"APIPULSE_ENV", "APIPULSE_HTTP_ADDR", "PROMETHEUS_ADDR",
		"DATABASE_URL", "REDIS_URL", "LOG_LEVEL",
	}
	for _, k := range keys {
		os.Unsetenv(k)
	}
	// DATABASE_URL has no default that will pass validation in strict mode,
	// so provide a minimal valid one.
	os.Setenv("DATABASE_URL", "postgres://apipulse:apipulse@localhost:5432/apipulse?sslmode=disable")
	defer os.Unsetenv("DATABASE_URL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.Env != EnvDevelopment {
		t.Errorf("expected env=development, got %q", cfg.Env)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("expected HTTPAddr=:8080, got %q", cfg.HTTPAddr)
	}
	if cfg.ShutdownTimeout.Seconds() != 30 {
		t.Errorf("expected ShutdownTimeout=30s, got %v", cfg.ShutdownTimeout)
	}
}

func TestLoad_InvalidEnv(t *testing.T) {
	os.Setenv("APIPULSE_ENV", "production-cloud")
	os.Setenv("DATABASE_URL", "postgres://x:y@localhost/db?sslmode=disable")
	defer func() {
		os.Unsetenv("APIPULSE_ENV")
		os.Unsetenv("DATABASE_URL")
	}()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid APIPULSE_ENV, got nil")
	}
}

func TestLoad_EmptyDatabaseURL(t *testing.T) {
	os.Setenv("APIPULSE_ENV", "development")
	os.Setenv("DATABASE_URL", "")
	defer func() {
		os.Unsetenv("APIPULSE_ENV")
		os.Unsetenv("DATABASE_URL")
	}()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for empty DATABASE_URL, got nil")
	}
}

func TestLoad_ProductionEnv(t *testing.T) {
	os.Setenv("APIPULSE_ENV", "production")
	os.Setenv("DATABASE_URL", "postgres://prod:secret@db:5432/apipulse?sslmode=require")
	defer func() {
		os.Unsetenv("APIPULSE_ENV")
		os.Unsetenv("DATABASE_URL")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.IsDevelopment() {
		t.Error("expected IsDevelopment()=false for production env")
	}
}

func TestLoad_InvalidShutdownTimeout(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://x:y@localhost/db?sslmode=disable")
	os.Setenv("APIPULSE_SHUTDOWN_TIMEOUT_SECONDS", "notanumber")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("APIPULSE_SHUTDOWN_TIMEOUT_SECONDS")
	}()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid APIPULSE_SHUTDOWN_TIMEOUT_SECONDS, got nil")
	}
}

func TestLoad_Phase3DConfig(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://apipulse:apipulse@localhost:5432/apipulse?sslmode=disable")
	os.Setenv("APIPULSE_STORAGE", "memory")
	os.Setenv("APIPULSE_HEALTH_EVALUATION_INTERVAL", "5s")
	os.Setenv("POSTGRES_MAX_CONNS", "30")
	os.Setenv("POSTGRES_MIN_CONNS", "8")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("APIPULSE_STORAGE")
		os.Unsetenv("APIPULSE_HEALTH_EVALUATION_INTERVAL")
		os.Unsetenv("POSTGRES_MAX_CONNS")
		os.Unsetenv("POSTGRES_MIN_CONNS")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.StorageBackend != "memory" {
		t.Errorf("expected StorageBackend='memory', got %q", cfg.StorageBackend)
	}
	if cfg.HealthEvalInterval.Seconds() != 5 {
		t.Errorf("expected HealthEvalInterval=5s, got %v", cfg.HealthEvalInterval)
	}
	if cfg.PostgresMaxConns != 30 {
		t.Errorf("expected PostgresMaxConns=30, got %d", cfg.PostgresMaxConns)
	}
	if cfg.PostgresMinConns != 8 {
		t.Errorf("expected PostgresMinConns=8, got %d", cfg.PostgresMinConns)
	}
}

func TestLoad_InvalidStorageBackend(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://apipulse:apipulse@localhost:5432/apipulse?sslmode=disable")
	os.Setenv("APIPULSE_STORAGE", "mongodb")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("APIPULSE_STORAGE")
	}()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid APIPULSE_STORAGE, got nil")
	}
}
