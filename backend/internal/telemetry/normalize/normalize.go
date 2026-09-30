// Package normalize implements the APIPulse telemetry normalization pipeline.
//
// Raw telemetry arriving at the ingestion gateway may contain high-cardinality
// fields, dynamic path segments, and potentially sensitive data. This package
// is responsible for transforming raw input into the canonical internal model
// before any storage or analysis occurs.
//
// Normalization guarantees:
//   - operation strings are sanitized of dynamic path segments
//   - status codes are mapped to classification buckets
//   - timestamps are coerced to UTC
//   - sources are validated and stamped
//   - no PII, secrets, or payload data passes through
package normalize

import (
	"regexp"
	"strings"
	"time"

	"github.com/apipulse/apipulse/internal/telemetry/model"
)

// pathSegmentRe matches UUID-like or numeric-only path segments that should
// be replaced with a placeholder to prevent high-cardinality label explosion.
var pathSegmentRe = regexp.MustCompile(`/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}|/\d+`)

// NormalizeDependencyObservation applies the normalization pipeline to a raw
// DependencyObservation before it is persisted.
//
// This function is idempotent and does not mutate the input.
func NormalizeDependencyObservation(raw *model.DependencyObservation) model.DependencyObservation {
	normalized := *raw // copy

	// Always coerce timestamps to UTC.
	normalized.Timestamp = raw.Timestamp.UTC()
	normalized.IngestedAt = time.Now().UTC()

	// Normalize dependency name to lowercase.
	normalized.DependencyName = strings.ToLower(strings.TrimSpace(raw.DependencyName))

	// Normalize service name.
	normalized.ServiceName = strings.TrimSpace(raw.ServiceName)

	// Sanitize operation string: replace dynamic segments with placeholders.
	normalized.Operation = sanitizeOperation(raw.Operation)

	// Normalize region to lowercase.
	normalized.Region = strings.ToLower(strings.TrimSpace(raw.Region))

	// Normalize environment to lowercase.
	normalized.Environment = strings.ToLower(strings.TrimSpace(raw.Environment))

	// Source is always explicitly set by the ingest layer — not from raw input.
	normalized.Source = model.SourceCustomerOTEL

	// Classify error type from status code if not provided.
	if normalized.ErrorType == model.ErrorTypeNone && normalized.Status != model.StatusOK {
		normalized.ErrorType = classifyErrorType(raw.StatusCode, raw.Status)
	}

	return normalized
}

// NormalizeProbeObservation applies the normalization pipeline to a raw ProbeObservation.
func NormalizeProbeObservation(raw *model.ProbeObservation) model.ProbeObservation {
	normalized := *raw

	normalized.Timestamp = raw.Timestamp.UTC()
	normalized.IngestedAt = time.Now().UTC()
	normalized.Region = strings.ToLower(strings.TrimSpace(raw.Region))

	// Source is always explicitly set — synthetic probes cannot self-declare a different source.
	normalized.Source = model.SourceSyntheticProbe

	// Sanitize the network error field: truncate to prevent unbounded strings.
	if len(normalized.NetworkError) > 512 {
		normalized.NetworkError = normalized.NetworkError[:512]
	}

	// Coerce timeout classification.
	if normalized.ErrorType == model.ErrorTypeTimeout {
		normalized.Timeout = true
	}

	return normalized
}

// sanitizeOperation replaces dynamic path segments (UUIDs, numeric IDs) with
// a cardinality-safe placeholder. This prevents label explosion in Prometheus.
//
// Example: "GET /v1/customers/cus_Aa1Bb2Cc3/invoices/123" -> "GET /v1/customers/:id/invoices/:id"
func sanitizeOperation(operation string) string {
	if operation == "" {
		return ""
	}
	// Replace UUID segments.
	result := pathSegmentRe.ReplaceAllStringFunc(operation, func(s string) string {
		return "/:id"
	})
	// Truncate to prevent unbounded storage.
	if len(result) > 256 {
		result = result[:256]
	}
	return result
}

// classifyErrorType derives an error classification from an HTTP status code
// and observation status when the caller has not provided one explicitly.
func classifyErrorType(statusCode int, status model.ObservationStatus) model.ErrorType {
	switch {
	case status == model.StatusTimeout:
		return model.ErrorTypeTimeout
	case statusCode >= 400 && statusCode < 500:
		return model.ErrorTypeHTTP4xx
	case statusCode >= 500:
		return model.ErrorTypeHTTP5xx
	case status == model.StatusError:
		return model.ErrorTypeUnknown
	default:
		return model.ErrorTypeNone
	}
}
