// Package metrics defines APIPulse's Prometheus instrumentation.
//
// Label cardinality rules (STRICTLY ENFORCED):
//   - Labels must use bounded, known-cardinality values only.
//   - NEVER add labels from: user IDs, trace IDs, request IDs, customer IDs,
//     arbitrary URLs, or any user-provided free-form strings.
//   - All label values must come from controlled enumerations.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all registered APIPulse Prometheus collectors.
// Use New() to create and register them with a given registry.
type Metrics struct {
	// HTTP layer
	HTTPRequestsTotal       *prometheus.CounterVec
	HTTPRequestDurationSecs *prometheus.HistogramVec

	// Telemetry ingestion
	TelemetryIngestedTotal *prometheus.CounterVec
	TelemetryRejectedTotal *prometheus.CounterVec

	// Synthetic probes
	ProbeTotal            *prometheus.CounterVec
	ProbeFailureTotal     *prometheus.CounterVec
	ProbeDurationSecs     *prometheus.HistogramVec
	ProbeDNSDurationSecs  *prometheus.HistogramVec
	ProbeTCPDurationSecs  *prometheus.HistogramVec
	ProbeTLSDurationSecs  *prometheus.HistogramVec
	ProbeTTFBDurationSecs *prometheus.HistogramVec

	// OpenTelemetry OTLP ingestion
	OTELSpansReceivedTotal *prometheus.CounterVec
	OTELSpansAcceptedTotal *prometheus.CounterVec
	OTELSpansRejectedTotal *prometheus.CounterVec
	OTELIngestDurationSecs *prometheus.HistogramVec

	// Health Intelligence Engine (Phase 3C)
	// Bounded labels: dependency, region
	HealthEvaluationsTotal *prometheus.CounterVec
	// Bounded labels: dependency, region, status
	HealthAssessmentsTotal *prometheus.CounterVec
	// Bounded labels: dependency, region
	GreyFailuresDetectedTotal *prometheus.CounterVec
	// Gauge: ratio of observed vs baseline latency
	DependencyLatencyRatio *prometheus.GaugeVec
	// Gauge: number of currently active (open) incidents
	ActiveIncidents *prometheus.GaugeVec
	// Histogram: Health Engine evaluation duration
	HealthEvaluationDurationSecs *prometheus.HistogramVec

	// --- Phase 3D: Scheduler & Operational Storage Metrics ---
	SchedulerEvaluationsTotal        *prometheus.CounterVec
	SchedulerEvaluationFailuresTotal *prometheus.CounterVec
	SchedulerActive                  prometheus.Gauge
	PostgresPoolConnections          *prometheus.GaugeVec
	RedisOperationsTotal             *prometheus.CounterVec
	RedisErrorsTotal                 *prometheus.CounterVec
}

// New creates and registers all APIPulse metrics with the provided registry.
// Pass prometheus.DefaultRegisterer for the standard global registry.
func New(reg prometheus.Registerer) *Metrics {
	factory := promauto.With(reg)

	return &Metrics{
		// --- HTTP layer ---
		HTTPRequestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "apipulse_http_requests_total",
			Help: "Total HTTP requests processed.",
		}, []string{"method", "route", "status_class"}),

		HTTPRequestDurationSecs: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "apipulse_http_request_duration_seconds",
			Help:    "Distribution of HTTP request durations in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),

		// --- Telemetry ingestion ---
		TelemetryIngestedTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "apipulse_telemetry_ingested_total",
			Help: "Total telemetry observations successfully ingested.",
		}, []string{"source"}),

		TelemetryRejectedTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "apipulse_telemetry_rejected_total",
			Help: "Total telemetry observations rejected during ingestion.",
		}, []string{"source", "reason"}),

		// --- Synthetic probes ---
		ProbeTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "apipulse_probe_total",
			Help: "Total synthetic probes executed.",
		}, []string{"region", "dependency"}),

		ProbeFailureTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "apipulse_probe_failure_total",
			Help: "Total synthetic probe failures.",
		}, []string{"region", "dependency", "error_type"}),

		ProbeDurationSecs: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "apipulse_probe_duration_seconds",
			Help:    "Distribution of synthetic probe total durations in seconds.",
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0},
		}, []string{"region", "dependency"}),

		ProbeDNSDurationSecs: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "apipulse_probe_dns_duration_seconds",
			Help:    "Distribution of synthetic probe DNS resolution durations.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
		}, []string{"region", "dependency"}),

		ProbeTCPDurationSecs: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "apipulse_probe_tcp_duration_seconds",
			Help:    "Distribution of synthetic probe TCP connection durations.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
		}, []string{"region", "dependency"}),

		ProbeTLSDurationSecs: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "apipulse_probe_tls_duration_seconds",
			Help:    "Distribution of synthetic probe TLS handshake durations.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.0},
		}, []string{"region", "dependency"}),

		ProbeTTFBDurationSecs: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "apipulse_probe_ttfb_seconds",
			Help:    "Distribution of synthetic probe TTFB durations.",
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		}, []string{"region", "dependency"}),

		// --- OpenTelemetry OTLP Ingestion ---
		OTELSpansReceivedTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "apipulse_otel_spans_received_total",
			Help: "Total OpenTelemetry spans received by the OTLP receiver.",
		}, []string{"service"}),

		OTELSpansAcceptedTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "apipulse_otel_spans_accepted_total",
			Help: "Total OpenTelemetry dependency spans accepted and normalized into CUSTOMER_OTEL observations.",
		}, []string{"service", "dependency"}),

		OTELSpansRejectedTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "apipulse_otel_spans_rejected_total",
			Help: "Total OpenTelemetry spans rejected or skipped during normalization.",
		}, []string{"service", "reason"}),

		OTELIngestDurationSecs: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "apipulse_otel_ingest_duration_seconds",
			Help:    "Duration of OTLP payload parsing and normalization in seconds.",
			Buckets: []float64{0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.5, 1.0},
		}, []string{"endpoint"}),

		// --- Health Intelligence Engine (Phase 3C) ---
		HealthEvaluationsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "apipulse_health_evaluations_total",
			Help: "Total health evaluation cycles run by the Health Intelligence Engine.",
		}, []string{"dependency", "region"}),

		HealthAssessmentsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "apipulse_health_assessments_total",
			Help: "Total APIPULSE_ASSESSMENT records produced, by status.",
		}, []string{"dependency", "region", "status"}),

		GreyFailuresDetectedTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "apipulse_grey_failures_detected_total",
			Help: "Total GREY_FAILURE assessments produced. Core APIPulse detection metric.",
		}, []string{"dependency", "region"}),

		DependencyLatencyRatio: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "apipulse_dependency_latency_ratio",
			Help: "Ratio of observed latency to baseline latency for a dependency+region.",
		}, []string{"dependency", "region"}),

		ActiveIncidents: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "apipulse_active_incidents",
			Help: "Number of currently open (non-resolved) incidents per dependency+region.",
		}, []string{"dependency", "region"}),

		HealthEvaluationDurationSecs: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "apipulse_health_evaluation_duration_seconds",
			Help:    "Duration of a single health evaluation cycle in seconds.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.0},
		}, []string{"dependency"}),

		// --- Phase 3D: Scheduler & Storage Metrics ---
		SchedulerEvaluationsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "apipulse_scheduler_evaluations_total",
			Help: "Total health evaluations executed by the background scheduler.",
		}, []string{"dependency", "region"}),

		SchedulerEvaluationFailuresTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "apipulse_scheduler_evaluation_failures_total",
			Help: "Total health evaluation failures in the background scheduler.",
		}, []string{"dependency", "region"}),

		SchedulerActive: factory.NewGauge(prometheus.GaugeOpts{
			Name: "apipulse_scheduler_active",
			Help: "1 if the background health evaluation scheduler is running, 0 otherwise.",
		}),

		PostgresPoolConnections: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "apipulse_postgres_pool_connections",
			Help: "Current number of connections in the PostgreSQL connection pool by state (acquired, idle, total).",
		}, []string{"state"}),

		RedisOperationsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "apipulse_redis_operations_total",
			Help: "Total operations performed against the Redis hot state cache.",
		}, []string{"operation"}),

		RedisErrorsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "apipulse_redis_errors_total",
			Help: "Total errors encountered during Redis hot state cache operations.",
		}, []string{"operation"}),
	}
}
