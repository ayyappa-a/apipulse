// Package model — assessment.go
// Canonical models for the APIPulse Health Intelligence Engine (Phase 3C).
//
// These types represent APIPULSE_ASSESSMENT (computed health), vendor status
// observations, incidents, and incident events.
//
// Source provenance must be preserved everywhere:
//   CUSTOMER_OTEL | SYNTHETIC_PROBE | VENDOR_STATUS | APIPULSE_ASSESSMENT
package model

import (
	"time"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// Assessment States
// ─────────────────────────────────────────────────────────────────────────────

// AssessmentStatus represents the overall health state of a dependency in a region.
type AssessmentStatus string

const (
	// AssessmentHealthy — both customer and synthetic signals are within expected range.
	AssessmentHealthy AssessmentStatus = "HEALTHY"

	// AssessmentDegraded — latency/errors exceed configured thresholds.
	AssessmentDegraded AssessmentStatus = "DEGRADED"

	// AssessmentGreyFailure — vendor reports OPERATIONAL and HTTP returns 200,
	// but customer and/or synthetic latency is severely elevated.
	// This is APIPulse's primary detection capability.
	AssessmentGreyFailure AssessmentStatus = "GREY_FAILURE"

	// AssessmentOutage — strong evidence of actual unavailability.
	AssessmentOutage AssessmentStatus = "OUTAGE"

	// AssessmentRecovering — previously degraded; returning toward baseline
	// but hysteresis recovery conditions are not yet fully satisfied.
	AssessmentRecovering AssessmentStatus = "RECOVERING"

	// AssessmentUnknown — insufficient or conflicting signals.
	AssessmentUnknown AssessmentStatus = "UNKNOWN"

	// AssessmentInsufficientData — not enough observations for reliable assessment.
	AssessmentInsufficientData AssessmentStatus = "INSUFFICIENT_DATA"
)

// ─────────────────────────────────────────────────────────────────────────────
// Signal States (per-source evaluation result)
// ─────────────────────────────────────────────────────────────────────────────

// SignalState is the evaluated state of a single telemetry signal.
type SignalState string

const (
	SignalHealthy           SignalState = "HEALTHY"
	SignalDegraded          SignalState = "DEGRADED"
	SignalElevated          SignalState = "ELEVATED"
	SignalUnknown           SignalState = "UNKNOWN"
	SignalInsufficientData  SignalState = "INSUFFICIENT_DATA"
)

// ─────────────────────────────────────────────────────────────────────────────
// Vendor Status
// ─────────────────────────────────────────────────────────────────────────────

// VendorStatus is the vendor's declared operational state.
type VendorStatus string

const (
	VendorStatusOperational  VendorStatus = "OPERATIONAL"
	VendorStatusDegraded     VendorStatus = "DEGRADED"
	VendorStatusPartialOutage VendorStatus = "PARTIAL_OUTAGE"
	VendorStatusOutage       VendorStatus = "OUTAGE"
	VendorStatusUnknown      VendorStatus = "UNKNOWN"
)

// ─────────────────────────────────────────────────────────────────────────────
// Confidence
// ─────────────────────────────────────────────────────────────────────────────

// ConfidenceLevel is a categorical confidence in an assessment.
// We do NOT use precise percentages — the underlying evidence does not justify it.
type ConfidenceLevel string

const (
	ConfidenceLow    ConfidenceLevel = "LOW"
	ConfidenceMedium ConfidenceLevel = "MEDIUM"
	ConfidenceHigh   ConfidenceLevel = "HIGH"
)

// ─────────────────────────────────────────────────────────────────────────────
// Reason Codes
// ─────────────────────────────────────────────────────────────────────────────

// ReasonCode is a structured, machine-readable reason for an assessment.
// The frontend derives human-readable explanations from these codes.
type ReasonCode string

const (
	ReasonLatencyAboveBaseline          ReasonCode = "LATENCY_ABOVE_BASELINE"
	ReasonCustomerLatencyElevated       ReasonCode = "CUSTOMER_LATENCY_ELEVATED"
	ReasonSyntheticLatencyElevated      ReasonCode = "SYNTHETIC_LATENCY_ELEVATED"
	ReasonCustomerAndSyntheticCorrelated ReasonCode = "CUSTOMER_AND_SYNTHETIC_CORRELATED"
	ReasonVendorStatusOperational       ReasonCode = "VENDOR_STATUS_OPERATIONAL"
	ReasonVendorStatusDegraded          ReasonCode = "VENDOR_STATUS_DEGRADED"
	ReasonHTTPSuccessButHighLatency     ReasonCode = "HTTP_SUCCESS_BUT_HIGH_LATENCY"
	ReasonRegionalDegradation           ReasonCode = "REGIONAL_DEGRADATION"
	ReasonInsufficientData              ReasonCode = "INSUFFICIENT_DATA"
	ReasonRecoveryInProgress            ReasonCode = "RECOVERY_IN_PROGRESS"
	ReasonHighErrorRate                 ReasonCode = "HIGH_ERROR_RATE"
	ReasonHTTPErrors                    ReasonCode = "HTTP_ERRORS"
	ReasonConnectionFailures            ReasonCode = "CONNECTION_FAILURES"
	ReasonWithinNormalRange             ReasonCode = "WITHIN_NORMAL_RANGE"
)

// ─────────────────────────────────────────────────────────────────────────────
// Recommended Actions
// ─────────────────────────────────────────────────────────────────────────────

// RecommendedAction is a structured action APIPulse recommends (but does NOT enforce).
// The customer application is responsible for implementing any circuit logic.
type RecommendedAction string

const (
	ActionNone                   RecommendedAction = "NONE"
	ActionMonitor                RecommendedAction = "MONITOR"
	ActionArmFallbackOrCircuit   RecommendedAction = "ARM_FALLBACK_OR_CIRCUIT"
	ActionInvestigate            RecommendedAction = "INVESTIGATE"
	ActionEscalate               RecommendedAction = "ESCALATE"
)

// ─────────────────────────────────────────────────────────────────────────────
// Baseline Result
// ─────────────────────────────────────────────────────────────────────────────

// BaselineResult holds computed latency percentiles for a signal window.
// If insufficient samples exist, Status will be InsufficientData.
type BaselineResult struct {
	DependencyName string    `json:"dependency_name"`
	Region         string    `json:"region"`
	Source         TelemetrySource `json:"source"`
	P50MS          float64   `json:"p50_ms"`
	P95MS          float64   `json:"p95_ms"`
	P99MS          float64   `json:"p99_ms"`
	SampleCount    int       `json:"sample_count"`
	WindowStart    time.Time `json:"window_start"`
	WindowEnd      time.Time `json:"window_end"`
	// Status is INSUFFICIENT_DATA if SampleCount < MinSamples, else READY.
	Status         string    `json:"status"`
}

const BaselineStatusReady            = "READY"
const BaselineStatusInsufficientData = "INSUFFICIENT_DATA"

// ─────────────────────────────────────────────────────────────────────────────
// HealthAssessment
// ─────────────────────────────────────────────────────────────────────────────

// HealthAssessment is the canonical APIPULSE_ASSESSMENT record.
// It represents the Health Intelligence Engine's evaluation of a
// dependency in a specific region at a specific point in time.
//
// It contains enough information to explain WHY the assessment was made.
// The frontend must display source provenance explicitly.
type HealthAssessment struct {
	ID           uuid.UUID        `json:"id" db:"id"`
	DependencyID string           `json:"dependency_id" db:"dependency_id"`
	DependencyName string         `json:"dependency_name" db:"dependency_name"`
	Region       string           `json:"region" db:"region"`
	Timestamp    time.Time        `json:"timestamp" db:"timestamp"`

	// Overall assessed status.
	Status AssessmentStatus `json:"status" db:"status"`

	// Signal evaluations — independently assessed.
	CustomerSignal  SignalState `json:"customer_signal" db:"customer_signal"`
	SyntheticSignal SignalState `json:"synthetic_signal" db:"synthetic_signal"`
	VendorStatus    VendorStatus `json:"vendor_status" db:"vendor_status"`

	// Observed latency (latest evaluation window).
	CustomerLatencyMS  float64 `json:"customer_latency_ms" db:"customer_latency_ms"`
	SyntheticLatencyMS float64 `json:"synthetic_latency_ms" db:"synthetic_latency_ms"`

	// Baseline latency (rolling window).
	CustomerBaselineP95MS  float64 `json:"customer_baseline_p95_ms" db:"customer_baseline_p95_ms"`
	SyntheticBaselineP95MS float64 `json:"synthetic_baseline_p95_ms" db:"synthetic_baseline_p95_ms"`

	// Ratio of observed vs baseline (max of both signals).
	LatencyMultiplier float64 `json:"latency_multiplier" db:"latency_multiplier"`

	// Categorical confidence — honest, not fabricated.
	Confidence ConfidenceLevel `json:"confidence" db:"confidence"`

	// Structured reason codes for this assessment.
	ReasonCodes []ReasonCode `json:"reason_codes" db:"-"`
	// Serialized for DB storage.
	ReasonCodesStr string `json:"-" db:"reason_codes"`

	// Human-readable explanation derived from reason codes.
	Explanation string `json:"explanation" db:"explanation"`

	// Recommended action (APIPulse recommends; customer implements).
	RecommendedAction RecommendedAction `json:"recommended_action" db:"recommended_action"`

	// Source is always APIPULSE_ASSESSMENT.
	Source TelemetrySource `json:"source" db:"source"`

	// Number of customer observations used in evaluation window.
	CustomerSampleCount int `json:"customer_sample_count" db:"customer_sample_count"`
	// Number of synthetic observations used in evaluation window.
	SyntheticSampleCount int `json:"synthetic_sample_count" db:"synthetic_sample_count"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ─────────────────────────────────────────────────────────────────────────────
// VendorStatusRecord (stored)
// ─────────────────────────────────────────────────────────────────────────────

// VendorStatusRecord is a stored vendor status entry from the adapter.
type VendorStatusRecord struct {
	ID               uuid.UUID    `json:"id" db:"id"`
	VendorName       string       `json:"vendor_name" db:"vendor_name"`
	DependencyName   string       `json:"dependency_name" db:"dependency_name"`
	Region           string       `json:"region" db:"region"`
	Timestamp        time.Time    `json:"timestamp" db:"timestamp"`
	Source           TelemetrySource `json:"source" db:"source"`
	Status           VendorStatus `json:"status" db:"status"`
	StatusText       string       `json:"status_text" db:"status_text"`
	IncidentReference string      `json:"incident_reference,omitempty" db:"incident_reference"`
	IngestedAt       time.Time    `json:"ingested_at" db:"ingested_at"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Incident
// ─────────────────────────────────────────────────────────────────────────────

// IncidentStatus is the lifecycle state of an incident.
type IncidentStatus string

const (
	IncidentOpen         IncidentStatus = "OPEN"
	IncidentAcknowledged IncidentStatus = "ACKNOWLEDGED"
	IncidentRecovering   IncidentStatus = "RECOVERING"
	IncidentResolved     IncidentStatus = "RESOLVED"
)

// IncidentEventType classifies an incident timeline entry.
type IncidentEventType string

const (
	IncidentEventDetected   IncidentEventType = "DETECTED"
	IncidentEventEscalated  IncidentEventType = "ESCALATED"
	IncidentEventRecovering IncidentEventType = "RECOVERING"
	IncidentEventResolved   IncidentEventType = "RESOLVED"
	IncidentEventNote       IncidentEventType = "NOTE"
)

// Incident represents a health degradation episode for a dependency/region pair.
// Only one active (non-resolved) incident exists per dependency+region+type.
type Incident struct {
	ID             uuid.UUID      `json:"id" db:"id"`
	DependencyName string         `json:"dependency_name" db:"dependency_name"`
	Region         string         `json:"region" db:"region"`
	// DeduplicationKey ensures exactly one active incident per dep+region+type.
	DeduplicationKey string       `json:"deduplication_key" db:"deduplication_key"`
	IncidentType   AssessmentStatus `json:"incident_type" db:"incident_type"`
	Status         IncidentStatus `json:"status" db:"status"`
	Title          string         `json:"title" db:"title"`
	Severity       string         `json:"severity" db:"severity"` // critical | high | medium | low
	AssessmentID   uuid.UUID      `json:"assessment_id" db:"assessment_id"`
	StartedAt      time.Time      `json:"started_at" db:"started_at"`
	ResolvedAt     *time.Time     `json:"resolved_at,omitempty" db:"resolved_at"`
	CreatedAt      time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at" db:"updated_at"`
	// Events are loaded separately (not stored inline).
	Events         []IncidentEvent `json:"events,omitempty" db:"-"`
}

// IncidentEvent is an append-only timeline entry for an incident.
type IncidentEvent struct {
	ID           uuid.UUID         `json:"id" db:"id"`
	IncidentID   uuid.UUID         `json:"incident_id" db:"incident_id"`
	EventType    IncidentEventType `json:"event_type" db:"event_type"`
	Reason       string            `json:"reason" db:"reason"`
	AssessmentID uuid.UUID         `json:"assessment_id" db:"assessment_id"`
	DependencyName string          `json:"dependency_name" db:"dependency_name"`
	Region       string            `json:"region" db:"region"`
	CreatedAt    time.Time         `json:"created_at" db:"created_at"`
}
