// Package handlers — phase3c_handlers.go
// HTTP handlers for the APIPulse Health Intelligence API (Phase 3C).
//
// Endpoints provided:
//   GET /api/v1/dependencies              — List all known dependencies
//   GET /api/v1/dependencies/{id}/health  — Latest health assessment for a dependency
//   GET /api/v1/dependencies/{id}/assessments — Assessment history
//   GET /api/v1/incidents                 — List incidents
//   GET /api/v1/incidents/{id}            — Single incident with events
//
// Source provenance is always preserved in responses.
// Raw OTLP data is never exposed through these endpoints.
package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/apipulse/apipulse/internal/storage/postgres"
	"github.com/apipulse/apipulse/internal/telemetry/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// DependencyHealthHandler — GET /api/v1/dependencies/{name}/health
// ─────────────────────────────────────────────────────────────────────────────

// DependencyHealthHandler returns the latest health assessment for a dependency.
type DependencyHealthHandler struct {
	store postgres.TelemetryStore
}

// NewDependencyHealthHandler creates a DependencyHealthHandler.
func NewDependencyHealthHandler(store postgres.TelemetryStore) *DependencyHealthHandler {
	return &DependencyHealthHandler{store: store}
}

// ServeHTTP handles GET /api/v1/dependencies/{name}/health.
func (h *DependencyHealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	depName := extractPathSegment(r.URL.Path, "/api/v1/dependencies/", "/health")
	if depName == "" {
		writeError(w, http.StatusBadRequest, "missing_dependency", "dependency name is required")
		return
	}

	region := r.URL.Query().Get("region")
	if region == "" {
		region = "ap-south-1" // default region for demo
	}

	assessment, err := h.store.GetLatestAssessment(r.Context(), depName, region)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", "failed to read health assessment")
		return
	}

	if assessment == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"dependency_name": depName,
			"region":         region,
			"status":         string(model.AssessmentInsufficientData),
			"message":        "No health assessment available yet. Insufficient observations.",
			"source":         string(model.SourceAPIPulseAssessment),
		})
		return
	}

	writeJSON(w, http.StatusOK, assessmentResponse(assessment))
}

// ─────────────────────────────────────────────────────────────────────────────
// AssessmentsHandler — GET /api/v1/dependencies/{name}/assessments
// ─────────────────────────────────────────────────────────────────────────────

// AssessmentsHandler returns assessment history for a dependency.
type AssessmentsHandler struct {
	store postgres.TelemetryStore
}

// NewAssessmentsHandler creates an AssessmentsHandler.
func NewAssessmentsHandler(store postgres.TelemetryStore) *AssessmentsHandler {
	return &AssessmentsHandler{store: store}
}

// ServeHTTP handles GET /api/v1/dependencies/{name}/assessments.
func (h *AssessmentsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	depName := extractPathSegment(r.URL.Path, "/api/v1/dependencies/", "/assessments")
	if depName == "" {
		writeError(w, http.StatusBadRequest, "missing_dependency", "dependency name is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	assessments, err := h.store.ListAssessments(r.Context(), depName, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", "failed to read assessments")
		return
	}

	responses := make([]map[string]any, 0, len(assessments))
	for i := range assessments {
		responses = append(responses, assessmentResponse(&assessments[i]))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"dependency_name": depName,
		"count":          len(responses),
		"assessments":    responses,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// IncidentsHandler — GET /api/v1/incidents
// ─────────────────────────────────────────────────────────────────────────────

// IncidentsHandler returns the incident list.
type IncidentsHandler struct {
	store postgres.TelemetryStore
}

// NewIncidentsHandler creates an IncidentsHandler.
func NewIncidentsHandler(store postgres.TelemetryStore) *IncidentsHandler {
	return &IncidentsHandler{store: store}
}

// ServeHTTP handles GET /api/v1/incidents.
func (h *IncidentsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	incidents, err := h.store.ListIncidents(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", "failed to read incidents")
		return
	}

	responses := make([]map[string]any, 0, len(incidents))
	for _, inc := range incidents {
		responses = append(responses, incidentResponse(&inc))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count":     len(responses),
		"incidents": responses,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// IncidentDetailHandler — GET /api/v1/incidents/{id}
// ─────────────────────────────────────────────────────────────────────────────

// IncidentDetailHandler returns a single incident with its event timeline.
type IncidentDetailHandler struct {
	store postgres.TelemetryStore
}

// NewIncidentDetailHandler creates an IncidentDetailHandler.
func NewIncidentDetailHandler(store postgres.TelemetryStore) *IncidentDetailHandler {
	return &IncidentDetailHandler{store: store}
}

// ServeHTTP handles GET /api/v1/incidents/{id}.
func (h *IncidentDetailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "incident ID is required")
		return
	}

	inc, err := h.store.GetIncident(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "incident not found")
		return
	}

	resp := incidentResponse(inc)
	events := make([]map[string]any, 0, len(inc.Events))
	for _, evt := range inc.Events {
		events = append(events, map[string]any{
			"id":              evt.ID,
			"event_type":      string(evt.EventType),
			"reason":          evt.Reason,
			"dependency_name": evt.DependencyName,
			"region":          evt.Region,
			"created_at":      evt.CreatedAt,
		})
	}
	resp["events"] = events

	writeJSON(w, http.StatusOK, resp)
}

// ─────────────────────────────────────────────────────────────────────────────
// DependenciesListHandler — GET /api/v1/dependencies
// ─────────────────────────────────────────────────────────────────────────────

// DependenciesListHandler returns known dependencies with their current health.
type DependenciesListHandler struct {
	store postgres.TelemetryStore
}

// NewDependenciesListHandler creates a DependenciesListHandler.
func NewDependenciesListHandler(store postgres.TelemetryStore) *DependenciesListHandler {
	return &DependenciesListHandler{store: store}
}

// ServeHTTP handles GET /api/v1/dependencies.
func (h *DependenciesListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Dynamically discover active targets from the store
	discovered, err := h.store.GetActiveTargets(r.Context())
	if err != nil || len(discovered) == 0 {
		// Fallback to default demo targets if none discovered yet
		discovered = []postgres.Target{
			{DependencyName: "stripe-like", Region: "ap-south-1"},
			{DependencyName: "stripe-like", Region: "us-east-1"},
		}
	}

	results := make([]map[string]any, 0, len(discovered))
	for _, t := range discovered {
		a, _ := h.store.GetLatestAssessment(r.Context(), t.DependencyName, t.Region)
		entry := map[string]any{
			"dependency_name": t.DependencyName,
			"region":          t.Region,
			"source":          string(model.SourceAPIPulseAssessment),
		}
		if a != nil {
			entry["status"] = string(a.Status)
			entry["vendor_status"] = string(a.VendorStatus)
			entry["customer_signal"] = string(a.CustomerSignal)
			entry["synthetic_signal"] = string(a.SyntheticSignal)
			entry["confidence"] = string(a.Confidence)
			entry["recommended_action"] = string(a.RecommendedAction)
			entry["timestamp"] = a.Timestamp
		} else {
			entry["status"] = string(model.AssessmentInsufficientData)
		}
		results = append(results, entry)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count":        len(results),
		"dependencies": results,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// DependencyObservationsHandler — GET /api/v1/dependencies/{name}/observations
// ─────────────────────────────────────────────────────────────────────────────

// DependencyObservationsHandler returns recent raw observations with full provenance.
type DependencyObservationsHandler struct {
	store postgres.TelemetryStore
}

// NewDependencyObservationsHandler creates a DependencyObservationsHandler.
func NewDependencyObservationsHandler(store postgres.TelemetryStore) *DependencyObservationsHandler {
	return &DependencyObservationsHandler{store: store}
}

// ServeHTTP handles GET /api/v1/dependencies/{name}/observations.
func (h *DependencyObservationsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	depName := extractPathSegment(r.URL.Path, "/api/v1/dependencies/", "/observations")
	if depName == "" {
		writeError(w, http.StatusBadRequest, "missing_dependency", "dependency name is required")
		return
	}

	region := r.URL.Query().Get("region")
	if region == "" {
		region = "ap-south-1"
	}

	minutesStr := r.URL.Query().Get("minutes")
	minutes := 30
	if minutesStr != "" {
		if n, err := strconv.Atoi(minutesStr); err == nil && n > 0 && n <= 1440 {
			minutes = n
		}
	}
	since := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)

	depObs, err := h.store.GetRecentDependencyObservations(r.Context(), depName, region, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", "failed to read customer observations")
		return
	}

	probeObs, err := h.store.GetRecentProbeObservations(r.Context(), depName, region, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", "failed to read probe observations")
		return
	}

	custResponses := make([]map[string]any, 0, len(depObs))
	for _, o := range depObs {
		custResponses = append(custResponses, map[string]any{
			"id":              o.ID,
			"timestamp":       o.Timestamp,
			"duration_ms":     o.DurationMS,
			"status":          string(o.Status),
			"status_code":     o.StatusCode,
			"source":          string(o.Source),
			"region":          o.Region,
			"dependency_name": o.DependencyName,
		})
	}

	probeResponses := make([]map[string]any, 0, len(probeObs))
	for _, p := range probeObs {
		probeResponses = append(probeResponses, map[string]any{
			"id":                p.ID,
			"timestamp":         p.Timestamp,
			"total_duration_ms": p.TotalDurationMS,
			"dns_duration_ms":   p.DNSDurationMS,
			"tcp_duration_ms":   p.TCPDurationMS,
			"tls_duration_ms":   p.TLSDurationMS,
			"ttfb_duration_ms":  p.TTFBDurationMS,
			"transfer_duration_ms": p.TransferDurationMS,
			"success":           p.Success,
			"http_status_code":  p.HTTPStatusCode,
			"reused_connection": p.ReusedConnection,
			"source":            string(p.Source),
			"region":            p.Region,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"dependency_name":       depName,
		"region":                region,
		"customer_observations": custResponses,
		"probe_observations":    probeResponses,
	})
}


// ─────────────────────────────────────────────────────────────────────────────
// Helper serializers
// ─────────────────────────────────────────────────────────────────────────────

func assessmentResponse(a *model.HealthAssessment) map[string]any {
	reasonStrs := make([]string, len(a.ReasonCodes))
	for i, rc := range a.ReasonCodes {
		reasonStrs[i] = string(rc)
	}

	return map[string]any{
		"id":                       a.ID,
		"dependency_name":          a.DependencyName,
		"region":                   a.Region,
		"timestamp":                a.Timestamp,
		"source":                   string(a.Source),
		"status":                   string(a.Status),
		"customer_signal":          string(a.CustomerSignal),
		"synthetic_signal":         string(a.SyntheticSignal),
		"vendor_status":            string(a.VendorStatus),
		"customer_latency_ms":      a.CustomerLatencyMS,
		"synthetic_latency_ms":     a.SyntheticLatencyMS,
		"customer_baseline_p95_ms": a.CustomerBaselineP95MS,
		"synthetic_baseline_p95_ms": a.SyntheticBaselineP95MS,
		"latency_multiplier":       a.LatencyMultiplier,
		"confidence":               string(a.Confidence),
		"reason_codes":             reasonStrs,
		"explanation":              a.Explanation,
		"recommended_action":       string(a.RecommendedAction),
		"customer_sample_count":    a.CustomerSampleCount,
		"synthetic_sample_count":   a.SyntheticSampleCount,
	}
}

func incidentResponse(inc *model.Incident) map[string]any {
	resp := map[string]any{
		"id":                inc.ID,
		"dependency_name":   inc.DependencyName,
		"region":            inc.Region,
		"incident_type":     string(inc.IncidentType),
		"status":            string(inc.Status),
		"title":             inc.Title,
		"severity":          inc.Severity,
		"assessment_id":     inc.AssessmentID,
		"started_at":        inc.StartedAt,
		"created_at":        inc.CreatedAt,
		"updated_at":        inc.UpdatedAt,
		"deduplication_key": inc.DeduplicationKey,
	}
	if inc.ResolvedAt != nil {
		resp["resolved_at"] = inc.ResolvedAt
	}
	return resp
}

// extractPathSegment extracts the segment between prefix and suffix from a URL path.
func extractPathSegment(path, prefix, suffix string) string {
	trimmed := strings.TrimPrefix(path, prefix)
	if suffix != "" {
		idx := strings.LastIndex(trimmed, suffix)
		if idx >= 0 {
			return trimmed[:idx]
		}
	}
	return strings.Trim(trimmed, "/")
}
