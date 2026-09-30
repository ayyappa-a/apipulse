// Package router assembles the APIPulse API server's HTTP routing tree.
package router

import (
	"net/http"

	"github.com/apipulse/apipulse/internal/api/handlers"
	"github.com/apipulse/apipulse/internal/api/middleware"
	"github.com/apipulse/apipulse/internal/config"
	"github.com/apipulse/apipulse/internal/storage/postgres"
	"github.com/apipulse/apipulse/internal/telemetry/ingest"
	"github.com/apipulse/apipulse/pkg/logger"
	"github.com/apipulse/apipulse/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
)

// New builds and returns the fully assembled APIPulse HTTP mux.
// All routes are registered here. Middleware is applied in order.
func New(cfg *config.Config, log *logger.Logger, m *metrics.Metrics) http.Handler {
	return NewWithStore(cfg, log, m, &postgres.NoopTelemetryStore{})
}

// NewWithStore builds the router with an explicit TelemetryStore.
// Used in tests and when a real database is connected.
func NewWithStore(cfg *config.Config, log *logger.Logger, m *metrics.Metrics, store postgres.TelemetryStore) http.Handler {
	return NewWithReadiness(cfg, log, m, store, nil)
}

// NewWithReadiness builds the router with TelemetryStore and dynamic readiness check func.
func NewWithReadiness(
	cfg *config.Config,
	log *logger.Logger,
	m *metrics.Metrics,
	store postgres.TelemetryStore,
	readyFunc handlers.ReadinessCheckFunc,
) http.Handler {
	mux := http.NewServeMux()

	// --- Operational endpoints (outside /api/v1 prefix) ---
	mux.Handle("GET /health", handlers.NewHealthHandler())
	if readyFunc != nil {
		mux.Handle("GET /ready", handlers.NewReadyHandler(readyFunc))
	} else {
		mux.Handle("GET /ready", handlers.NewReadyHandler())
	}

	// Prometheus metrics scrape endpoint.
	mux.Handle("GET /metrics", promhttp.Handler())

	// --- API v1 info ---
	mux.Handle("GET /api/v1", handlers.NewAPIInfoHandler("0.4.0-phase3d", "2026-09-30"))

	// --- Telemetry ingestion ---
	// Development JSON endpoint (backward compat / local diagnostics).
	mux.Handle("POST /api/v1/telemetry/ingest",
		handlers.NewTelemetryIngestHandler(log, m))

	// OpenTelemetry OTLP/HTTP trace receivers.
	otlpReceiver := ingest.NewReceiver(ingest.Config{
		Store:         store,
		Metrics:       m,
		Logger:        log,
		DefaultRegion: "local",
	})
	mux.Handle("POST /v1/traces", otlpReceiver)
	mux.Handle("POST /api/v1/telemetry/otlp", otlpReceiver)

	// Synthetic probe results ingestion.
	mux.Handle("POST /api/v1/probes/results",
		handlers.NewProbeResultHandler(log, m))

	// --- Phase 3C: Health Intelligence API ---

	// GET /api/v1/dependencies — list dependencies with current health
	mux.Handle("GET /api/v1/dependencies",
		handlers.NewDependenciesListHandler(store))

	// GET /api/v1/dependencies/{name}/health — latest assessment for a dependency
	mux.Handle("GET /api/v1/dependencies/",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			switch {
			case hasPathSuffix(path, "/health"):
				handlers.NewDependencyHealthHandler(store).ServeHTTP(w, r)
			case hasPathSuffix(path, "/assessments"):
				handlers.NewAssessmentsHandler(store).ServeHTTP(w, r)
			case hasPathSuffix(path, "/observations"):
				handlers.NewDependencyObservationsHandler(store).ServeHTTP(w, r)
			default:
				http.NotFound(w, r)
			}
		}))

	// GET /api/v1/incidents — incident list
	mux.Handle("GET /api/v1/incidents",
		handlers.NewIncidentsHandler(store))

	// GET /api/v1/incidents/{id} — incident detail with events
	mux.Handle("GET /api/v1/incidents/",
		handlers.NewIncidentDetailHandler(store))

	// --- CORS ---
	c := cors.New(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: false,
	})

	// Apply middleware chain: CORS -> Recovery -> RequestID -> Logging -> Instrumentation -> mux
	var handler http.Handler = mux
	handler = middleware.Instrumentation(m)(handler)
	handler = middleware.RequestLogging(log)(handler)
	handler = middleware.RequestID(handler)
	handler = middleware.Recovery(log)(handler)
	handler = c.Handler(handler)

	return handler
}

// hasPathSuffix checks if a URL path ends with the given suffix segment.
func hasPathSuffix(path, suffix string) bool {
	n := len(path)
	s := len(suffix)
	if n < s {
		return false
	}
	return path[n-s:] == suffix
}
