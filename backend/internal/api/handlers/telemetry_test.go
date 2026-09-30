package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/apipulse/apipulse/pkg/logger"
	"github.com/apipulse/apipulse/pkg/metrics"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
)

func testMetrics() *metrics.Metrics {
	reg := prometheus.NewRegistry()
	return metrics.New(reg)
}

func testLogger() *logger.Logger {
	return logger.New("test", "error")
}

// --- TelemetryIngestHandler tests ---

func TestTelemetryIngestHandler_ValidObservation(t *testing.T) {
	h := NewTelemetryIngestHandler(testLogger(), testMetrics())

	body := DevIngestRequest{
		ServiceName:    "codebegun-api",
		DependencyName: "stripe",
		Operation:      "POST /v1/charges",
		Region:         "us-east-1",
		DurationMS:     142.3,
		Status:         "ok",
		StatusCode:     200,
		Environment:    "production",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry/ingest", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("expected 202 Accepted, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestTelemetryIngestHandler_MissingServiceName(t *testing.T) {
	h := NewTelemetryIngestHandler(testLogger(), testMetrics())

	body := DevIngestRequest{
		DependencyName: "stripe",
		Operation:      "POST /v1/charges",
		Region:         "us-east-1",
		DurationMS:     100,
		Status:         "ok",
		StatusCode:     200,
		Environment:    "production",
		// ServiceName intentionally omitted
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry/ingest", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestTelemetryIngestHandler_InvalidJSON(t *testing.T) {
	h := NewTelemetryIngestHandler(testLogger(), testMetrics())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry/ingest",
		bytes.NewReader([]byte(`not json`)))
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestTelemetryIngestHandler_MethodNotAllowed(t *testing.T) {
	h := NewTelemetryIngestHandler(testLogger(), testMetrics())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/telemetry/ingest", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

// --- ProbeResultHandler tests ---

func TestProbeResultHandler_ValidProbe(t *testing.T) {
	h := NewProbeResultHandler(testLogger(), testMetrics())

	obs := model.ProbeObservation{
		ID:               uuid.New(),
		ProbeID:          uuid.New(),
		DependencyID:     uuid.New(),
		Region:           "us-east-1",
		Timestamp:        time.Now().UTC(),
		Source:           model.SourceSyntheticProbe,
		Success:          true,
		HTTPStatusCode:   200,
		DNSDurationMS:    10,
		TCPDurationMS:    20,
		TLSDurationMS:    50,
		TTFBDurationMS:   100,
		TransferDurationMS: 5,
		TotalDurationMS:  185,
		ErrorType:        model.ErrorTypeNone,
	}
	b, _ := json.Marshal(obs)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/probes/results", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestProbeResultHandler_FailedProbeWithNoErrorType(t *testing.T) {
	h := NewProbeResultHandler(testLogger(), testMetrics())

	obs := model.ProbeObservation{
		ID:              uuid.New(),
		ProbeID:         uuid.New(),
		DependencyID:    uuid.New(),
		Region:          "eu-west-1",
		Timestamp:       time.Now().UTC(),
		Source:          model.SourceSyntheticProbe,
		Success:         false,
		TotalDurationMS: 30000,
		// ErrorType intentionally omitted — should fail validation
	}
	b, _ := json.Marshal(obs)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/probes/results", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for invalid probe, got %d", rr.Code)
	}
}
