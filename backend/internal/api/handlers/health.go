// Package handlers contains the HTTP request handlers for the APIPulse API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// HealthHandler handles health probe requests.
// GET /health — Kubernetes liveness probe compatible.
// Returns 200 if the server process is alive. Does not check dependencies.
type HealthHandler struct {
	startTime time.Time
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{startTime: time.Now().UTC()}
}

// ServeHTTP handles GET /health.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime_s":  time.Since(h.startTime).Seconds(),
	})
}

// ReadinessCheckFunc is called by ReadyHandler to inspect real subsystems (DB, Redis, Health Engine).
type ReadinessCheckFunc func(ctx context.Context) (isReady bool, details map[string]string)

// ReadyHandler handles readiness probe requests.
// GET /ready — Kubernetes readiness probe compatible.
// Returns 200 only when the server and all mandatory dependencies are ready.
// Returns 503 if required persistence or subsystem is unavailable.
type ReadyHandler struct {
	checkFunc ReadinessCheckFunc
}

// NewReadyHandler creates a new ReadyHandler with optional readiness check function.
func NewReadyHandler(checkers ...ReadinessCheckFunc) *ReadyHandler {
	var c ReadinessCheckFunc
	if len(checkers) > 0 {
		c = checkers[0]
	}
	return &ReadyHandler{checkFunc: c}
}

// ServeHTTP handles GET /ready.
func (h *ReadyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.checkFunc == nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ready",
		})
		return
	}

	isReady, details := h.checkFunc(r.Context())
	status := "ready"
	httpCode := http.StatusOK
	if !isReady {
		status = "unready"
		httpCode = http.StatusServiceUnavailable
	}

	resp := map[string]interface{}{
		"status": status,
	}
	for k, v := range details {
		resp[k] = v
	}

	writeJSON(w, httpCode, resp)
}

// APIInfoHandler handles the API version/info endpoint.
// GET /api/v1 — returns server version and API contract information.
type APIInfoHandler struct {
	version   string
	buildTime string
}

// NewAPIInfoHandler creates a new APIInfoHandler.
func NewAPIInfoHandler(version, buildTime string) *APIInfoHandler {
	return &APIInfoHandler{version: version, buildTime: buildTime}
}

// ServeHTTP handles GET /api/v1.
func (h *APIInfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":       "APIPulse Telemetry Gateway",
		"version":    h.version,
		"api_prefix": "/api/v1",
		"build_time": h.buildTime,
		"endpoints": []string{
			"GET  /health",
			"GET  /ready",
			"GET  /api/v1",
			"POST /api/v1/telemetry/ingest    [dev] Customer OTEL observations",
			"POST /api/v1/probes/results      [dev] Synthetic probe results",
			"GET  /api/v1/dependencies        [stub]",
			"GET  /metrics                    Prometheus scrape endpoint",
		},
	})
}

// writeJSON writes a JSON response with the given status code.
// It sets Content-Type and never writes raw user input back to the client.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// At this point headers are already written; log only.
		http.Error(w, `{"error":"encoding failure"}`, http.StatusInternalServerError)
	}
}

// writeError writes a JSON error response with a fixed, non-reflective error message.
// User-supplied values are never echoed back in error responses.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"error":   code,
		"message": message,
	})
}
