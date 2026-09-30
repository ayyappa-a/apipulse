// APIPulse API Server — cmd/api/main.go
//
// This is the entry point for the APIPulse Telemetry Gateway & Continuous Health Intelligence Server.
//
// Lifecycle:
//   1. Load configuration from environment
//   2. Initialize structured logging
//   3. Register Prometheus metrics
//   4. Initialize persistent storage (PostgreSQL/TimescaleDB via pgxpool, or in-memory for testing)
//   5. Run database migrations if in postgres mode
//   6. Initialize hot state cache (Redis via go-redis, with graceful in-memory fallback)
//   7. Initialize Health Intelligence Engine
//   8. Start background health evaluation scheduler
//   9. Start HTTP API and Prometheus metrics servers
//  10. Graceful shutdown: stop scheduler -> close Redis -> close PostgreSQL -> stop HTTP servers
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/apipulse/apipulse/internal/api/router"
	"github.com/apipulse/apipulse/internal/config"
	"github.com/apipulse/apipulse/internal/health"
	"github.com/apipulse/apipulse/internal/storage/postgres"
	rediscache "github.com/apipulse/apipulse/internal/storage/redis"
	"github.com/apipulse/apipulse/internal/vendors"
	"github.com/apipulse/apipulse/pkg/logger"
	"github.com/apipulse/apipulse/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// --- 1. Configuration ---
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: configuration error: %v\n", err)
		os.Exit(1)
	}

	// --- 2. Logging ---
	log := logger.New("apipulse-api", cfg.LogLevel)
	log.Info("APIPulse API server starting")
	log.Info(fmt.Sprintf("environment=%s addr=%s storage=%s", cfg.Env, cfg.HTTPAddr, cfg.StorageBackend))

	// --- 3. Metrics ---
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	initCtx, initCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer initCancel()

	// --- 4. PostgreSQL / TelemetryStore ---
	var store postgres.TelemetryStore
	var pgStore *postgres.PostgresStore

	if cfg.StorageBackend == "postgres" {
		log.Info("connecting to PostgreSQL / TimescaleDB")
		ps, err := postgres.NewPostgresStore(initCtx, cfg)
		if err != nil {
			log.Error("failed to connect to PostgreSQL — failing startup per strict persistence policy", err)
			os.Exit(1)
		}
		pgStore = ps
		store = ps
		log.Info("PostgreSQL connection pool initialized")

		// 5. Run migrations
		migrationsDir := os.Getenv("MIGRATIONS_DIR")
		if migrationsDir == "" {
			migrationsDir = filepath.Join("migrations")
			if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
				migrationsDir = filepath.Join("backend", "migrations")
			}
		}

		if _, err := os.Stat(migrationsDir); err == nil {
			log.Info(fmt.Sprintf("running database migrations from %s", migrationsDir))
			if err := pgStore.RunMigrations(initCtx, migrationsDir); err != nil {
				log.Warn(fmt.Sprintf("migration execution notice: %v", err))
			} else {
				log.Info("database migrations verified successfully")
			}
		}
	} else {
		log.Info("running with in-memory telemetry store (development/test mode)")
		store = postgres.NewInMemoryStore()
	}

	// --- 6. Redis hot state cache ---
	var cache rediscache.StateCache
	var redisCache *rediscache.RedisCache

	rc, err := rediscache.NewRedisCache(initCtx, cfg.RedisURL, m)
	if err != nil {
		log.Warn(fmt.Sprintf("Redis connection failed (%v) — operating in degraded mode with memory fallback", err))
		cache = &rediscache.NoopStateCache{}
	} else {
		redisCache = rc
		cache = rc
		log.Info("connected to Redis hot state cache")
	}

	// --- 7. Vendor status provider ---
	vendorMockURL := os.Getenv("VENDOR_MOCK_URL")
	if vendorMockURL == "" {
		vendorMockURL = "http://vendor-mock:9001"
	}
	vendorProvider := vendors.NewDeterministicAdapter(vendorMockURL)

	// --- 8. Health Intelligence Engine ---
	engineCfg := health.DefaultEngineConfig()
	engine := health.NewEngine(store, cache, vendorProvider, m, engineCfg)

	// --- 9. Background Health Evaluation Scheduler ---
	targetMap := make(map[string]health.EvaluationTarget)
	for _, raw := range cfg.HealthTargets {
		parts := strings.Split(raw, ":")
		if len(parts) >= 3 {
			t := health.EvaluationTarget{
				DependencyName: parts[0],
				DependencyID:   parts[0],
				VendorName:     parts[1],
				Region:         parts[2],
			}
			targetMap[t.DependencyName+":"+t.Region] = t
		} else if len(parts) == 2 {
			t := health.EvaluationTarget{
				DependencyName: parts[0],
				DependencyID:   parts[0],
				VendorName:     parts[0],
				Region:         parts[1],
			}
			targetMap[t.DependencyName+":"+t.Region] = t
		}
	}

	// Dynamic target discovery from persistent store
	if discovered, err := store.GetActiveTargets(context.Background()); err == nil {
		for _, dt := range discovered {
			key := dt.DependencyName + ":" + dt.Region
			if _, exists := targetMap[key]; !exists {
				targetMap[key] = health.EvaluationTarget{
					DependencyName: dt.DependencyName,
					DependencyID:   dt.DependencyName,
					VendorName:     dt.DependencyName,
					Region:         dt.Region,
				}
			}
		}
	}

	targets := make([]health.EvaluationTarget, 0, len(targetMap))
	for _, t := range targetMap {
		targets = append(targets, t)
	}

	scheduler := health.NewScheduler(engine, targets, cfg.HealthEvalInterval, m, log)
	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	scheduler.Start(serverCtx)

	// --- 10. Readiness check callback ---
	readinessFunc := func(ctx context.Context) (bool, map[string]string) {
		details := make(map[string]string)
		isReady := true

		// Check database
		if pgStore != nil {
			if err := pgStore.Ping(ctx); err != nil {
				details["postgres"] = "error: " + err.Error()
				isReady = false
			} else {
				details["postgres"] = "ok"
				stats := pgStore.Pool().Stat()
				m.PostgresPoolConnections.WithLabelValues("acquired").Set(float64(stats.AcquiredConns()))
				m.PostgresPoolConnections.WithLabelValues("idle").Set(float64(stats.IdleConns()))
				m.PostgresPoolConnections.WithLabelValues("total").Set(float64(stats.TotalConns()))
			}
		} else {
			details["postgres"] = "memory_mode"
		}

		// Check Redis
		if cache.IsActive(ctx) {
			details["redis"] = "ok"
		} else {
			details["redis"] = "degraded"
		}

		// Check Health Engine / Scheduler
		if scheduler.IsRunning() {
			details["health_engine"] = "ok"
		} else {
			details["health_engine"] = "stopped"
			isReady = false
		}

		return isReady, details
	}

	// --- 11. Router ---
	handler := router.NewWithReadiness(cfg, log, m, store, readinessFunc)

	// --- 12. HTTP Servers ---
	apiServer := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	metricsServer := &http.Server{
		Addr:         cfg.PrometheusAddr,
		Handler:      promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// --- Start servers ---
	errCh := make(chan error, 2)

	go func() {
		log.Info(fmt.Sprintf("API server listening on %s", cfg.HTTPAddr))
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("API server error: %w", err)
		}
	}()

	go func() {
		log.Info(fmt.Sprintf("Prometheus metrics server listening on %s", cfg.PrometheusAddr))
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("metrics server error: %w", err)
		}
	}()

	// --- Shutdown signal handling ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info(fmt.Sprintf("shutdown signal received: %s", sig))
	case err := <-errCh:
		log.Error("server error — initiating shutdown", err)
	}

	// --- 13. Graceful Shutdown ---
	log.Info(fmt.Sprintf("graceful shutdown started (timeout: %s)", cfg.ShutdownTimeout))

	// Stop scheduler first
	scheduler.Stop()

	// Shutdown context
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		log.Error("API server shutdown error", err)
	}
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		log.Error("metrics server shutdown error", err)
	}

	// Close Redis
	if redisCache != nil {
		_ = redisCache.Close()
	}

	// Close PostgreSQL
	if pgStore != nil {
		pgStore.Close()
	}

	log.Info("APIPulse API server stopped cleanly")
}
