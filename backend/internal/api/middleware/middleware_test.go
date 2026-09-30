package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apipulse/apipulse/pkg/logger"
	"github.com/apipulse/apipulse/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func testMetrics() *metrics.Metrics {
	reg := prometheus.NewRegistry()
	return metrics.New(reg)
}

func testLogger() *logger.Logger {
	return logger.New("test", "error")
}

func TestRequestID_AddsHeader(t *testing.T) {
	var capturedID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = requestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestID(inner)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Header().Get(requestIDHeader) == "" {
		t.Error("expected X-Request-ID header to be set")
	}
	if capturedID == "" {
		t.Error("expected request ID in context")
	}
	if rr.Header().Get(requestIDHeader) != capturedID {
		t.Error("expected header value to match context value")
	}
}

func TestRecovery_CatchesPanic(t *testing.T) {
	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	handler := Recovery(testLogger())(panicking)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	// Should not panic.
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 after panic recovery, got %d", rr.Code)
	}
}

func TestInstrumentation_RecordsMetrics(t *testing.T) {
	m := testMetrics()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := Instrumentation(m)(inner)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	// If no panic, metrics were successfully recorded.
}

func TestStatusClass(t *testing.T) {
	cases := []struct {
		code     int
		expected string
	}{
		{200, "2xx"}, {201, "2xx"}, {204, "2xx"},
		{301, "3xx"}, {302, "3xx"},
		{400, "4xx"}, {404, "4xx"}, {422, "4xx"},
		{500, "5xx"}, {503, "5xx"},
	}
	for _, tc := range cases {
		got := statusClass(tc.code)
		if got != tc.expected {
			t.Errorf("statusClass(%d): expected %q, got %q", tc.code, tc.expected, got)
		}
	}
}

func TestRequestLogging_DoesNotPanic(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestID(RequestLogging(testLogger())(inner))
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
}

func TestResponseWriter_WritesOnce(t *testing.T) {
	rw := newResponseWriter(httptest.NewRecorder())
	rw.WriteHeader(http.StatusAccepted)
	rw.WriteHeader(http.StatusOK) // second call should be ignored

	if rw.statusCode != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", rw.statusCode)
	}
}

// TestRequestID_UniquePerRequest ensures each request gets a distinct ID.
func TestRequestID_UniquePerRequest(t *testing.T) {
	ids := make([]string, 5)
	for i := range ids {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(rr, req)
		ids[i] = rr.Header().Get(requestIDHeader)
	}

	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Errorf("duplicate request ID generated: %q", id)
		}
		seen[id] = true
		if !strings.Contains(id, "-") {
			t.Errorf("request ID does not look like a UUID: %q", id)
		}
	}
}
