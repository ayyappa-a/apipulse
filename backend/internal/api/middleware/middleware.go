// Package middleware provides HTTP middleware for the APIPulse API server.
package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/apipulse/apipulse/pkg/logger"
	"github.com/apipulse/apipulse/pkg/metrics"
	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-ID"

// RequestID attaches a unique request ID to every inbound request.
// The ID is set on the response header for client-side correlation.
// Request IDs are generated server-side and are never derived from user input.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()
		w.Header().Set(requestIDHeader, id)
		r = r.WithContext(withRequestID(r.Context(), id))
		next.ServeHTTP(w, r)
	})
}

// Recovery catches panics in downstream handlers, logs them, and returns a
// 500 Internal Server Error instead of crashing the process.
func Recovery(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered in HTTP handler",
						fmt.Errorf("panic: %v", rec))
					http.Error(w, `{"error":"internal server error"}`,
						http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

// statusClass converts an HTTP status code to a coarse 3-char class string
// (2xx, 4xx, 5xx) for use as a bounded Prometheus label.
func statusClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return "1xx"
	}
}

// Instrumentation records Prometheus metrics for every HTTP request.
// Route label uses the registered route pattern, never the raw URL path,
// ensuring bounded cardinality.
func Instrumentation(m *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := newResponseWriter(w)

			next.ServeHTTP(rw, r)

			duration := time.Since(start).Seconds()
			route := routePattern(r)
			class := statusClass(rw.statusCode)

			m.HTTPRequestsTotal.WithLabelValues(r.Method, route, class).Inc()
			m.HTTPRequestDurationSecs.WithLabelValues(r.Method, route).Observe(duration)
		})
	}
}

// RequestLogging logs every request at INFO level with structured fields.
// It does NOT log request or response bodies.
func RequestLogging(log *logger.Logger) func(http.Handler) http.Handler {
	cl := log.With("http")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := newResponseWriter(w)
			reqID := requestIDFromContext(r.Context())

			next.ServeHTTP(rw, r)

			cl.WithRequestID(reqID).Info(
				fmt.Sprintf("%s %s %d %dms",
					r.Method, r.URL.Path, rw.statusCode,
					time.Since(start).Milliseconds()),
			)
		})
	}
}

// routePattern returns a stable route label for Prometheus.
// Falls back to "_unknown_" for unmatched routes.
func routePattern(r *http.Request) string {
	if p := r.Pattern; p != "" {
		return p
	}
	return "_unknown_"
}
