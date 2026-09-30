// Package ingest implements the production OpenTelemetry Protocol (OTLP) receiver.
//
// Status: PHASE 3 STUB — Not yet implemented.
//
// When implemented, this package will:
//   - Implement OTLP/HTTP protobuf ingestion at POST /api/v1/telemetry/otlp
//   - Parse ExportTraceServiceRequest protobuf messages from OpenTelemetry SDKs
//   - Extract HTTP client span attributes from ResourceSpans
//   - Map OTel attributes to DependencyObservation fields:
//       http.request.duration → duration_ms
//       http.response.status_code → status_code
//       net.peer.name / url.full (sanitized) → dependency_name + operation
//       net.peer.port, server.address → endpoint
//   - Apply normalization pipeline (normalize.NormalizeDependencyObservation)
//   - Persist to TimescaleDB via TelemetryStore
//
// The development JSON endpoint (POST /api/v1/telemetry/ingest) is a temporary
// stand-in that uses the same normalization and validation pipeline.
// Replacing it with this OTLP receiver in Phase 3 requires only adding the
// protobuf wire format layer on top.
//
// Reference: https://opentelemetry.io/docs/specs/otel/protocol/exporter/
package ingest
