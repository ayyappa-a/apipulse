// Package ingest implements the production OpenTelemetry Protocol (OTLP/HTTP) receiver.
//
// It ingests trace data exported by OpenTelemetry SDKs and Collectors, extracts
// outbound dependency client spans, sanitizes all fields, normalizes them into
// canonical model.DependencyObservation records, and persists them via TelemetryStore.
//
// Wire format support:
//   - application/x-protobuf (standard OTLP protobuf)
//   - application/json (standard OTLP JSON)
//
// Privacy and Security Contract:
//   - Strict allowlist attribute extraction
//   - Zero payload/body retention (request and response bodies are dropped)
//   - Strips authorization, cookies, API keys, credentials, and PII
//   - Raw OTLP payloads are NEVER retained or logged
//   - Always stamps Source = CUSTOMER_OTEL
package ingest

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/apipulse/apipulse/internal/storage/postgres"
	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/apipulse/apipulse/internal/telemetry/normalize"
	"github.com/apipulse/apipulse/pkg/logger"
	"github.com/apipulse/apipulse/pkg/metrics"
)

// maxBodyBytes defines the upper payload size (4 MiB) to prevent memory exhaustion.
const maxBodyBytes = 4 << 20

// Receiver is the HTTP handler for OTLP/HTTP trace ingestion.
type Receiver struct {
	store         postgres.TelemetryStore
	metrics       *metrics.Metrics
	logger        *logger.ContextLogger
	defaultRegion string
}

// Config configures the OTLP Receiver.
type Config struct {
	Store         postgres.TelemetryStore
	Metrics       *metrics.Metrics
	Logger        *logger.Logger
	DefaultRegion string
}

// NewReceiver creates a new OTLP trace receiver.
func NewReceiver(cfg Config) *Receiver {
	if cfg.Store == nil {
		cfg.Store = &postgres.NoopTelemetryStore{}
	}
	if cfg.DefaultRegion == "" {
		cfg.DefaultRegion = "local"
	}
	var cLog *logger.ContextLogger
	if cfg.Logger != nil {
		cLog = cfg.Logger.With("otlp_receiver")
	}

	return &Receiver{
		store:         cfg.Store,
		metrics:       cfg.Metrics,
		logger:        cLog,
		defaultRegion: cfg.DefaultRegion,
	}
}

// ServeHTTP handles OTLP/HTTP POST requests (both /v1/traces and /api/v1/telemetry/otlp).
func (rec *Receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	endpoint := r.URL.Path

	defer func() {
		if rec.metrics != nil && rec.metrics.OTELIngestDurationSecs != nil {
			rec.metrics.OTELIngestDurationSecs.WithLabelValues(endpoint).Observe(time.Since(start).Seconds())
		}
	}()

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed. Use POST for OTLP ingestion", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body: payload too large or aborted", http.StatusBadRequest)
		return
	}

	// 1. Decode OTLP Request (Protobuf or JSON)
	req := &coltracepb.ExportTraceServiceRequest{}
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		// Use standard protojson unmarshaler for OTLP JSON format
		unmarshaler := protojson.UnmarshalOptions{DiscardUnknown: true}
		if err := unmarshaler.Unmarshal(body, req); err != nil {
			rec.recordReject("unknown", "json_parse_error")
			http.Error(w, fmt.Sprintf("Invalid OTLP JSON: %v", err), http.StatusBadRequest)
			return
		}
	} else {
		// Default to standard OTLP protobuf format
		if err := proto.Unmarshal(body, req); err != nil {
			rec.recordReject("unknown", "protobuf_parse_error")
			http.Error(w, fmt.Sprintf("Invalid OTLP protobuf: %v", err), http.StatusBadRequest)
			return
		}
	}

	// 2. Process Spans and Map to Canonical CUSTOMER_OTEL Observations
	observations := rec.extractObservations(req)

	// 3. Normalize, Validate, and Persist
	acceptedCount := 0
	for _, obs := range observations {
		normalized := normalize.NormalizeDependencyObservation(&obs)

		// Hard invariant: Telemetry from CodeBegun application must always be stamped SourceCustomerOTEL
		normalized.Source = model.SourceCustomerOTEL

		if err := normalized.Validate(); err != nil {
			rec.recordReject(normalized.ServiceName, "validation_error")
			continue
		}

		if err := rec.store.InsertDependencyObservation(r.Context(), normalized); err != nil {
			if rec.logger != nil {
				rec.logger.Error("failed to persist customer telemetry observation", err)
			}
			continue
		}

		acceptedCount++
		if rec.metrics != nil {
			if rec.metrics.OTELSpansAcceptedTotal != nil {
				rec.metrics.OTELSpansAcceptedTotal.WithLabelValues(normalized.ServiceName, normalized.DependencyName).Inc()
			}
			if rec.metrics.TelemetryIngestedTotal != nil {
				rec.metrics.TelemetryIngestedTotal.WithLabelValues(string(model.SourceCustomerOTEL)).Inc()
			}
		}
	}

	// 4. Return standard OTLP Success Response
	resp := &coltracepb.ExportTraceServiceResponse{}
	if strings.Contains(contentType, "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		respBytes, _ := protojson.Marshal(resp)
		_, _ = w.Write(respBytes)
	} else {
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
		respBytes, _ := proto.Marshal(resp)
		_, _ = w.Write(respBytes)
	}
}

// extractObservations extracts external dependency client spans and ignores internal server spans.
func (rec *Receiver) extractObservations(req *coltracepb.ExportTraceServiceRequest) []model.DependencyObservation {
	var observations []model.DependencyObservation

	for _, resourceSpans := range req.GetResourceSpans() {
		// Extract Resource Attributes (service.name, environment, region)
		serviceName := "unknown-service"
		environment := "demo"
		resourceRegion := rec.defaultRegion

		if res := resourceSpans.GetResource(); res != nil {
			for _, attr := range res.GetAttributes() {
				switch attr.GetKey() {
				case "service.name":
					serviceName = getStringValue(attr.GetValue())
				case "deployment.environment.name", "deployment.environment":
					environment = getStringValue(attr.GetValue())
				case "cloud.region", "region":
					resourceRegion = getStringValue(attr.GetValue())
				}
			}
		}

		for _, scopeSpans := range resourceSpans.GetScopeSpans() {
			for _, span := range scopeSpans.GetSpans() {
				if rec.metrics != nil && rec.metrics.OTELSpansReceivedTotal != nil {
					rec.metrics.OTELSpansReceivedTotal.WithLabelValues(serviceName).Inc()
				}

				// Only process Client spans (Kind = 3) or spans explicitly marked with a dependency
				isClientSpan := span.GetKind() == tracepb.Span_SPAN_KIND_CLIENT
				depName, operation, statusCode, spanRegion := extractSpanAttributes(span)

				if depName == "" && !isClientSpan {
					// Internal server span or unannotated span, safely skip
					continue
				}

				if depName == "" {
					// Fallback to span name or peer service if not explicitly named
					depName = inferDependencyName(span)
				}

				if depName == "" {
					rec.recordReject(serviceName, "missing_dependency_name")
					continue
				}

				if spanRegion == "" {
					spanRegion = resourceRegion
				}

				if operation == "" {
					operation = span.GetName()
				}

				// Calculate duration in ms
				startNano := span.GetStartTimeUnixNano()
				endNano := span.GetEndTimeUnixNano()
				durationMS := float64(endNano-startNano) / 1e6
				if durationMS < 0 {
					durationMS = 0
				}

				// Derive Status and ErrorType
				obsStatus, errType := deriveStatusAndError(span, statusCode)

				obs := model.DependencyObservation{
					ID:             uuid.New(),
					Timestamp:      time.Unix(0, int64(startNano)).UTC(),
					ServiceName:    serviceName,
					DependencyName: depName,
					Operation:      operation,
					Region:         spanRegion,
					Source:         model.SourceCustomerOTEL,
					DurationMS:     durationMS,
					Status:         obsStatus,
					StatusCode:     statusCode,
					ErrorType:      errType,
					TraceID:        fmt.Sprintf("%x", span.GetTraceId()),
					Environment:    environment,
					IngestedAt:     time.Now().UTC(),
				}

				observations = append(observations, obs)
			}
		}
	}

	return observations
}

// extractSpanAttributes parses allowlisted attributes only.
// Sensitive attributes (tokens, cookies, card numbers, bodies, etc.) are discarded.
func extractSpanAttributes(span *tracepb.Span) (depName, operation string, statusCode int, region string) {
	for _, attr := range span.GetAttributes() {
		key := attr.GetKey()
		val := attr.GetValue()

		switch key {
		case "dependency.name", "peer.service":
			depName = getStringValue(val)
		case "http.request.method", "http.method":
			if operation == "" {
				operation = getStringValue(val)
			}
		case "http.route", "url.path", "http.target":
			path := getStringValue(val)
			if operation != "" && !strings.Contains(operation, " ") {
				operation = operation + " " + path
			} else if operation == "" {
				operation = path
			}
		case "http.response.status_code", "http.status_code":
			statusCode = int(val.GetIntValue())
		case "region", "cloud.region":
			region = getStringValue(val)
		case "server.address", "net.peer.name":
			if depName == "" {
				depName = cleanAddress(getStringValue(val))
			}
		}
	}
	return depName, operation, statusCode, region
}

func deriveStatusAndError(span *tracepb.Span, statusCode int) (model.ObservationStatus, model.ErrorType) {
	// Status code error mappings
	if statusCode >= 500 {
		return model.StatusError, model.ErrorTypeHTTP5xx
	}
	if statusCode == http.StatusTooManyRequests {
		return model.StatusError, model.ErrorTypeRateLimited
	}
	if statusCode >= 400 {
		return model.StatusError, model.ErrorTypeHTTP4xx
	}

	// Check Span Status
	if status := span.GetStatus(); status != nil {
		if status.GetCode() == tracepb.Status_STATUS_CODE_ERROR {
			msg := strings.ToLower(status.GetMessage())
			if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") {
				return model.StatusTimeout, model.ErrorTypeTimeout
			}
			return model.StatusError, model.ErrorTypeUnknown
		}
	}

	// HTTP 200 with elevated latency (e.g. 8s Grey Failure) is NOT an application error.
	// It is successfully handled by the application, though degraded.
	return model.StatusOK, model.ErrorTypeNone
}

func inferDependencyName(span *tracepb.Span) string {
	name := strings.ToLower(span.GetName())
	if strings.Contains(name, "stripe") {
		return "stripe"
	}
	if strings.Contains(name, "sendgrid") {
		return "sendgrid"
	}
	if strings.Contains(name, "twilio") {
		return "twilio"
	}
	return ""
}

func cleanAddress(addr string) string {
	parts := strings.Split(addr, ":")
	return strings.ToLower(parts[0])
}

func getStringValue(v *commonpb.AnyValue) string {
	if v == nil {
		return ""
	}
	return v.GetStringValue()
}

func (rec *Receiver) recordReject(service, reason string) {
	if rec.metrics != nil && rec.metrics.OTELSpansRejectedTotal != nil {
		rec.metrics.OTELSpansRejectedTotal.WithLabelValues(service, reason).Inc()
	}
}
