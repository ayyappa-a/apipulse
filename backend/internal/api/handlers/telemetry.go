package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/apipulse/apipulse/internal/telemetry/normalize"
	"github.com/apipulse/apipulse/pkg/logger"
	"github.com/apipulse/apipulse/pkg/metrics"
	"github.com/google/uuid"
)

// maxBodyBytes is the maximum request body size accepted by any telemetry endpoint.
// This prevents memory exhaustion from oversized payloads.
const maxBodyBytes = 1 << 20 // 1 MiB

// TelemetryIngestHandler handles the development telemetry ingestion endpoint.
//
// ⚠️  IMPORTANT ARCHITECTURAL NOTE:
// POST /api/v1/telemetry/ingest is a DEVELOPMENT ingestion contract.
// It accepts a simplified JSON body representing a DependencyObservation.
// This is NOT equivalent to a production OTLP/HTTP receiver.
//
// The production ingestion path will be:
//   POST /api/v1/telemetry/otlp  (OpenTelemetry Protocol over HTTP, protobuf)
//
// The OTLP/HTTP receiver is intentionally not implemented in this phase to avoid
// premature complexity. The current JSON endpoint establishes the normalization,
// validation, and tagging pipeline that the production OTLP receiver will reuse.
type TelemetryIngestHandler struct {
	log     *logger.ContextLogger
	metrics *metrics.Metrics
}

// NewTelemetryIngestHandler creates a new TelemetryIngestHandler.
func NewTelemetryIngestHandler(log *logger.Logger, m *metrics.Metrics) *TelemetryIngestHandler {
	return &TelemetryIngestHandler{
		log:     log.With("telemetry_ingest"),
		metrics: m,
	}
}

// DevIngestRequest is the development JSON contract for CUSTOMER_OTEL telemetry ingestion.
// It maps to a DependencyObservation after normalization.
//
// PRIVACY ENFORCEMENT — this struct deliberately excludes:
//   - request/response bodies
//   - authentication tokens
//   - API keys
//   - customer PII
//   - payment information
//   - arbitrary query parameters
type DevIngestRequest struct {
	ServiceName    string  `json:"service_name"`
	DependencyName string  `json:"dependency_name"`
	Operation      string  `json:"operation"`
	Region         string  `json:"region"`
	DurationMS     float64 `json:"duration_ms"`
	Status         string  `json:"status"`
	StatusCode     int     `json:"status_code"`
	ErrorType      string  `json:"error_type,omitempty"`
	TraceID        string  `json:"trace_id,omitempty"`
	Environment    string  `json:"environment"`
}

// ServeHTTP handles POST /api/v1/telemetry/ingest.
func (h *TelemetryIngestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST")
		return
	}

	// Enforce body size limit.
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.metrics.TelemetryRejectedTotal.WithLabelValues(string(model.SourceCustomerOTEL), "read_error").Inc()
		writeError(w, http.StatusBadRequest, "read_error", "Failed to read request body")
		return
	}

	var req DevIngestRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.metrics.TelemetryRejectedTotal.WithLabelValues(string(model.SourceCustomerOTEL), "parse_error").Inc()
		writeError(w, http.StatusBadRequest, "parse_error", "Invalid JSON")
		return
	}

	// Build raw observation — source is always overridden by normalizer.
	raw := &model.DependencyObservation{
		ID:             uuid.New(),
		Timestamp:      time.Now().UTC(),
		ServiceName:    req.ServiceName,
		DependencyName: req.DependencyName,
		Operation:      req.Operation,
		Region:         req.Region,
		Source:         model.SourceCustomerOTEL,
		DurationMS:     req.DurationMS,
		Status:         model.ObservationStatus(req.Status),
		StatusCode:     req.StatusCode,
		ErrorType:      model.ErrorType(req.ErrorType),
		TraceID:        req.TraceID,
		Environment:    req.Environment,
	}

	// Normalize before validation.
	normalized := normalize.NormalizeDependencyObservation(raw)

	// Validate the normalized observation.
	if err := normalized.Validate(); err != nil {
		h.metrics.TelemetryRejectedTotal.WithLabelValues(string(model.SourceCustomerOTEL), "validation_error").Inc()
		h.log.Warn("telemetry observation rejected: " + err.Error())
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "Observation failed validation")
		return
	}

	// TODO (Phase 3): persist normalized observation to TimescaleDB.
	// For now: record the acceptance metric and return success.
	h.metrics.TelemetryIngestedTotal.WithLabelValues(string(model.SourceCustomerOTEL)).Inc()
	h.log.Info("telemetry observation accepted (dev ingest — not yet persisted)")

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "accepted",
		"id":     normalized.ID.String(),
		"note":   "dev ingestion endpoint — persistence not yet implemented",
	})
}

// ProbeResultHandler handles synthetic probe result ingestion.
//
// ⚠️  DEVELOPMENT ENDPOINT
// POST /api/v1/probes/results accepts a JSON ProbeObservation from an APIPulse probe agent.
// Authentication and probe agent identity verification are NOT yet implemented.
type ProbeResultHandler struct {
	log     *logger.ContextLogger
	metrics *metrics.Metrics
}

// NewProbeResultHandler creates a new ProbeResultHandler.
func NewProbeResultHandler(log *logger.Logger, m *metrics.Metrics) *ProbeResultHandler {
	return &ProbeResultHandler{
		log:     log.With("probe_result"),
		metrics: m,
	}
}

// ServeHTTP handles POST /api/v1/probes/results.
func (h *ProbeResultHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.metrics.TelemetryRejectedTotal.WithLabelValues(string(model.SourceSyntheticProbe), "read_error").Inc()
		writeError(w, http.StatusBadRequest, "read_error", "Failed to read request body")
		return
	}

	var obs model.ProbeObservation
	if err := json.Unmarshal(body, &obs); err != nil {
		h.metrics.TelemetryRejectedTotal.WithLabelValues(string(model.SourceSyntheticProbe), "parse_error").Inc()
		writeError(w, http.StatusBadRequest, "parse_error", "Invalid JSON")
		return
	}

	// Normalize before validation.
	normalized := normalize.NormalizeProbeObservation(&obs)

	// Validate.
	if err := normalized.Validate(); err != nil {
		h.metrics.TelemetryRejectedTotal.WithLabelValues(string(model.SourceSyntheticProbe), "validation_error").Inc()
		h.log.Warn("probe observation rejected: " + err.Error())
		writeError(w, http.StatusUnprocessableEntity, "validation_error", "Probe observation failed validation")
		return
	}

	h.metrics.ProbeTotal.WithLabelValues(normalized.Region, "unknown").Inc()
	if !normalized.Success {
		h.metrics.ProbeFailureTotal.WithLabelValues(
			normalized.Region, "unknown", string(normalized.ErrorType)).Inc()
	}
	h.metrics.ProbeDurationSecs.WithLabelValues(normalized.Region, "unknown").Observe(
		normalized.TotalDurationMS / 1000.0,
	)
	h.metrics.TelemetryIngestedTotal.WithLabelValues(string(model.SourceSyntheticProbe)).Inc()

	h.log.Info("probe result accepted (dev endpoint — not yet persisted)")

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "accepted",
		"id":     normalized.ID.String(),
	})
}
