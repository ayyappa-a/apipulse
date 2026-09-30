// detector.go — Grey-Failure Detector and Assessment Engine
//
// This file implements the multi-signal health assessment that is the core of
// APIPulse's grey-failure detection capability.
//
// Detection principle:
//   A vendor can return HTTP 200 and report itself as Operational while causing
//   severe latency degradation for customers. APIPulse detects and explains this.
//
// The engine correlates three independent signals:
//   1. CUSTOMER_OTEL   — What did the customer's application actually experience?
//   2. SYNTHETIC_PROBE — Can APIPulse independently reproduce the degradation?
//   3. VENDOR_STATUS   — What does the vendor publicly declare?
//
// HTTP 200 is NOT health. Latency is the signal.
package health

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/apipulse/apipulse/internal/storage/postgres"
	rediscache "github.com/apipulse/apipulse/internal/storage/redis"
	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/apipulse/apipulse/internal/vendors"
	"github.com/apipulse/apipulse/pkg/metrics"
)

// ─────────────────────────────────────────────────────────────────────────────
// EngineConfig
// ─────────────────────────────────────────────────────────────────────────────

// EngineConfig controls all Health Engine parameters.
// All values are configurable — no magic numbers.
type EngineConfig struct {
	Baseline BaselineConfig

	// BaselineWindowDuration is the time window used to compute the rolling baseline.
	// Example: 15 minutes.
	BaselineWindowDuration time.Duration

	// EvaluationWindowDuration is the recent observation window for signal evaluation.
	// Example: 2 minutes.
	EvaluationWindowDuration time.Duration

	// RecoveryWindowCount is the number of consecutive healthy evaluation windows
	// required before transitioning from RECOVERING to HEALTHY.
	// Example: 3 windows.
	RecoveryWindowCount int64

	// AssessmentCacheTTL is how long to cache a health assessment in Redis.
	AssessmentCacheTTL time.Duration
}

// DefaultEngineConfig returns production-reasonable defaults.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		Baseline:                 DefaultBaselineConfig(),
		BaselineWindowDuration:   15 * time.Minute,
		EvaluationWindowDuration: 2 * time.Minute,
		RecoveryWindowCount:      3,
		AssessmentCacheTTL:       5 * time.Minute,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Engine
// ─────────────────────────────────────────────────────────────────────────────

// Engine is the APIPulse Health Intelligence Engine.
// It evaluates multi-signal observations and produces deterministic assessments.
//
// The Engine must NOT execute synchronously during OTLP ingestion.
// Call Evaluate() from a background goroutine or scheduled worker.
type Engine struct {
	store          postgres.TelemetryStore
	cache          rediscache.StateCache
	vendorProvider vendors.VendorStatusProvider
	metrics        *metrics.Metrics
	cfg            EngineConfig
}

// NewEngine creates a new Health Intelligence Engine.
func NewEngine(
	store postgres.TelemetryStore,
	cache rediscache.StateCache,
	vendorProvider vendors.VendorStatusProvider,
	m *metrics.Metrics,
	cfg EngineConfig,
) *Engine {
	return &Engine{
		store:          store,
		cache:          cache,
		vendorProvider: vendorProvider,
		metrics:        m,
		cfg:            cfg,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Evaluate — the core assessment loop for one dependency+region
// ─────────────────────────────────────────────────────────────────────────────

// EvaluationTarget identifies a dependency+region pair for assessment.
type EvaluationTarget struct {
	DependencyName string
	DependencyID   string
	VendorName     string
	Region         string
}

// Evaluate runs a single health assessment cycle for the given target.
// It fetches observations, computes baselines, evaluates signals,
// determines the assessment status, and persists the result.
//
// This method is safe to call concurrently for different targets.
// It must NOT be called from the OTLP ingestion hot path.
func (e *Engine) Evaluate(ctx context.Context, target EvaluationTarget) (*model.HealthAssessment, error) {
	now := time.Now().UTC()
	baselineStart := now.Add(-e.cfg.BaselineWindowDuration)
	evalStart := now.Add(-e.cfg.EvaluationWindowDuration)

	// ── Step 1: Fetch observations ────────────────────────────────────────────

	depObs, err := e.store.GetRecentDependencyObservations(ctx, target.DependencyName, target.Region, baselineStart)
	if err != nil {
		return nil, fmt.Errorf("fetch customer obs: %w", err)
	}

	probeObs, err := e.store.GetRecentProbeObservations(ctx, target.DependencyName, target.Region, baselineStart)
	if err != nil {
		return nil, fmt.Errorf("fetch probe obs: %w", err)
	}

	// ── Step 2: Split baseline vs evaluation window ───────────────────────────

	// Baseline: full window (up to 15 minutes)
	// Evaluation: recent window (last 2 minutes)
	var baselineDepObs, evalDepObs []model.DependencyObservation
	for _, o := range depObs {
		if o.Timestamp.After(evalStart) {
			evalDepObs = append(evalDepObs, o)
		} else {
			baselineDepObs = append(baselineDepObs, o)
		}
	}

	var baselineProbeObs, evalProbeObs []model.ProbeObservation
	for _, o := range probeObs {
		if o.Timestamp.After(evalStart) {
			evalProbeObs = append(evalProbeObs, o)
		} else {
			baselineProbeObs = append(baselineProbeObs, o)
		}
	}

	// ── Step 3: Compute baselines ─────────────────────────────────────────────

	customerBaseline := ComputeDepObsBaseline(baselineDepObs, e.cfg.Baseline, target.DependencyName, target.Region)
	syntheticBaseline := ComputeProbeBaseline(baselineProbeObs, e.cfg.Baseline, target.DependencyName, target.Region)

	// ── Step 4: Compute evaluation-window P95 ────────────────────────────────

	customerEvalP95, customerHasData := P95FromDepObs(evalDepObs)
	syntheticEvalP95, syntheticHasData := P95FromProbeObs(evalProbeObs)

	// ── Step 5: Evaluate each signal independently ───────────────────────────

	var customerSignal model.SignalState
	var customerMultiplier float64
	if !customerHasData {
		customerSignal = model.SignalInsufficientData
	} else {
		customerSignal, customerMultiplier = EvaluateSignal(customerEvalP95, customerBaseline, e.cfg.Baseline)
	}

	var syntheticSignal model.SignalState
	var syntheticMultiplier float64
	if !syntheticHasData {
		syntheticSignal = model.SignalInsufficientData
	} else {
		syntheticSignal, syntheticMultiplier = EvaluateSignal(syntheticEvalP95, syntheticBaseline, e.cfg.Baseline)
	}

	// ── Step 6: Get vendor status ─────────────────────────────────────────────

	var vendorStatus model.VendorStatus = model.VendorStatusUnknown
	if e.vendorProvider != nil {
		vsRec, vsErr := e.vendorProvider.GetStatus(ctx, target.VendorName, target.DependencyName, target.Region)
		if vsErr == nil {
			vendorStatus = vsRec.Status
			// Persist vendor status record.
			_ = e.store.InsertVendorStatus(ctx, vsRec)
		}
	}

	// ── Step 7: Determine assessment status ──────────────────────────────────

	maxMultiplier := customerMultiplier
	if syntheticMultiplier > maxMultiplier {
		maxMultiplier = syntheticMultiplier
	}

	status, reasonCodes, explanation, action := e.determineAssessmentStatus(
		customerSignal, syntheticSignal, vendorStatus,
		customerEvalP95, syntheticEvalP95,
		customerBaseline, syntheticBaseline,
	)

	// ── Step 8: Apply hysteresis / recovery ──────────────────────────────────

	status, reasonCodes = e.applyHysteresis(ctx, target, status, reasonCodes)

	// ── Step 9: Determine confidence ─────────────────────────────────────────

	confidence := e.computeConfidence(customerSignal, syntheticSignal, vendorStatus)

	// ── Step 10: Build assessment ─────────────────────────────────────────────

	assessment := model.HealthAssessment{
		ID:                     uuid.New(),
		DependencyID:           target.DependencyID,
		DependencyName:         target.DependencyName,
		Region:                 target.Region,
		Timestamp:              now,
		Status:                 status,
		CustomerSignal:         customerSignal,
		SyntheticSignal:        syntheticSignal,
		VendorStatus:           vendorStatus,
		CustomerLatencyMS:      customerEvalP95,
		SyntheticLatencyMS:     syntheticEvalP95,
		CustomerBaselineP95MS:  customerBaseline.P95MS,
		SyntheticBaselineP95MS: syntheticBaseline.P95MS,
		LatencyMultiplier:      maxMultiplier,
		Confidence:             confidence,
		ReasonCodes:            reasonCodes,
		ReasonCodesStr:         encodeReasonCodes(reasonCodes),
		Explanation:            explanation,
		RecommendedAction:      action,
		Source:                 model.SourceAPIPulseAssessment,
		CustomerSampleCount:    len(evalDepObs),
		SyntheticSampleCount:   len(evalProbeObs),
		CreatedAt:              now,
	}

	// ── Step 11: Persist assessment ───────────────────────────────────────────

	if err := e.store.InsertAssessment(ctx, assessment); err != nil {
		// Log but don't fail — assessment is still valid for in-memory usage.
		_ = err
	}

	// ── Step 12: Update Redis hot state ──────────────────────────────────────

	_ = e.cache.SetDependencyHealth(ctx, target.DependencyName, target.Region, status, e.cfg.AssessmentCacheTTL)
	_ = e.cache.SetLatestAssessment(ctx, target.DependencyName, target.Region, &assessment, e.cfg.AssessmentCacheTTL)

	// ── Step 13: Prometheus metrics ───────────────────────────────────────────

	if e.metrics != nil {
		e.metrics.HealthEvaluationsTotal.WithLabelValues(target.DependencyName, target.Region).Inc()
		e.metrics.HealthAssessmentsTotal.WithLabelValues(target.DependencyName, target.Region, string(status)).Inc()
		if status == model.AssessmentGreyFailure {
			e.metrics.GreyFailuresDetectedTotal.WithLabelValues(target.DependencyName, target.Region).Inc()
		}
		if maxMultiplier > 0 {
			e.metrics.DependencyLatencyRatio.WithLabelValues(target.DependencyName, target.Region).Set(maxMultiplier)
		}
	}

	// ── Step 14: Manage incident ──────────────────────────────────────────────

	e.manageIncident(ctx, target, &assessment)

	return &assessment, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Assessment status determination
// ─────────────────────────────────────────────────────────────────────────────

// determineAssessmentStatus applies the rules-based detector to produce an
// AssessmentStatus, reason codes, explanation, and recommended action.
//
// Rules (in priority order):
//
//  1. If both signals are INSUFFICIENT_DATA → INSUFFICIENT_DATA
//  2. If vendor is OUTAGE and at least one signal is degraded → OUTAGE
//  3. If both customer and synthetic are DEGRADED
//     AND vendor is OPERATIONAL → GREY_FAILURE  ← core detection
//  4. If either customer or synthetic is DEGRADED → DEGRADED
//  5. If vendor is DEGRADED/PARTIAL_OUTAGE but signals are OK → UNKNOWN
//  6. Otherwise → HEALTHY
func (e *Engine) determineAssessmentStatus(
	customerSignal, syntheticSignal model.SignalState,
	vendorStatus model.VendorStatus,
	customerP95, syntheticP95 float64,
	customerBaseline, syntheticBaseline model.BaselineResult,
) (model.AssessmentStatus, []model.ReasonCode, string, model.RecommendedAction) {

	var codes []model.ReasonCode

	// Rule 1: Both insufficient data
	if customerSignal == model.SignalInsufficientData &&
		syntheticSignal == model.SignalInsufficientData {
		codes = append(codes, model.ReasonInsufficientData)
		return model.AssessmentInsufficientData, codes,
			"Insufficient observations to compute a reliable health assessment.",
			model.ActionMonitor
	}

	customerDegraded := customerSignal == model.SignalDegraded
	syntheticDegraded := syntheticSignal == model.SignalDegraded

	// Collect relevant reason codes.
	if customerDegraded {
		codes = append(codes, model.ReasonCustomerLatencyElevated)
	}
	if syntheticDegraded {
		codes = append(codes, model.ReasonSyntheticLatencyElevated)
	}
	if customerDegraded && syntheticDegraded {
		codes = append(codes, model.ReasonCustomerAndSyntheticCorrelated)
	}

	// Rule 2: Vendor reports OUTAGE
	if vendorStatus == model.VendorStatusOutage {
		codes = append(codes, model.ReasonVendorStatusDegraded)
		return model.AssessmentOutage, codes,
			"Vendor has declared a major outage. Both independent signals confirm unavailability.",
			model.ActionEscalate
	}

	// Rule 3: Grey Failure — THE CORE DETECTION
	// Both signals degraded AND vendor declares OPERATIONAL AND HTTP returns 200
	if customerDegraded && syntheticDegraded && vendorStatus == model.VendorStatusOperational {
		codes = append(codes,
			model.ReasonVendorStatusOperational,
			model.ReasonHTTPSuccessButHighLatency,
		)
		explanation := fmt.Sprintf(
			"Vendor reports Operational and returns HTTP 200, but both independent "+
				"customer telemetry (P95=%.0fms) and synthetic probes (P95=%.0fms) "+
				"show severe latency degradation far exceeding baseline "+
				"(customer baseline P95=%.0fms, synthetic baseline P95=%.0fms).",
			customerP95, syntheticP95,
			customerBaseline.P95MS, syntheticBaseline.P95MS,
		)
		return model.AssessmentGreyFailure, codes, explanation, model.ActionArmFallbackOrCircuit
	}

	// Rule 3b: One-sided grey failure (only customer signal available and degraded)
	if customerDegraded && syntheticSignal == model.SignalInsufficientData &&
		vendorStatus == model.VendorStatusOperational {
		codes = append(codes,
			model.ReasonVendorStatusOperational,
			model.ReasonHTTPSuccessButHighLatency,
			model.ReasonInsufficientData,
		)
		explanation := fmt.Sprintf(
			"Customer telemetry shows severe latency (P95=%.0fms, baseline P95=%.0fms). "+
				"Vendor reports Operational. Synthetic probe has insufficient data to confirm.",
			customerP95, customerBaseline.P95MS,
		)
		return model.AssessmentDegraded, codes, explanation, model.ActionArmFallbackOrCircuit
	}

	// Rule 4: Degraded (regardless of vendor status)
	if customerDegraded || syntheticDegraded {
		if vendorStatus == model.VendorStatusDegraded || vendorStatus == model.VendorStatusPartialOutage {
			codes = append(codes, model.ReasonVendorStatusDegraded)
			return model.AssessmentDegraded, codes,
				"Latency degradation confirmed by telemetry signals. Vendor also reports degradation.",
				model.ActionInvestigate
		}
		return model.AssessmentDegraded, codes,
			"One or more telemetry signals show elevated latency exceeding configured thresholds.",
			model.ActionInvestigate
	}

	// Rule 5: Vendor degraded but signals healthy
	if vendorStatus == model.VendorStatusDegraded || vendorStatus == model.VendorStatusPartialOutage {
		codes = append(codes, model.ReasonVendorStatusDegraded)
		return model.AssessmentUnknown, codes,
			"Vendor reports degradation but observed latency is within normal range. Monitoring.",
			model.ActionMonitor
	}

	// Rule 6: Healthy
	codes = append(codes, model.ReasonWithinNormalRange)
	if vendorStatus == model.VendorStatusOperational {
		codes = append(codes, model.ReasonVendorStatusOperational)
	}
	return model.AssessmentHealthy, codes,
		"All signals are within expected range.",
		model.ActionNone
}

// ─────────────────────────────────────────────────────────────────────────────
// Hysteresis / Recovery
// ─────────────────────────────────────────────────────────────────────────────

// applyHysteresis prevents rapid HEALTHY↔DEGRADED flapping by requiring
// cfg.RecoveryWindowCount consecutive healthy evaluations before transitioning
// a previously-degraded dependency to HEALTHY.
//
// State machine:
//   DEGRADED/GREY_FAILURE → (healthy evaluation) → RECOVERING → ... →
//   (RecoveryWindowCount healthy evaluations) → HEALTHY
func (e *Engine) applyHysteresis(
	ctx context.Context,
	target EvaluationTarget,
	newStatus model.AssessmentStatus,
	codes []model.ReasonCode,
) (model.AssessmentStatus, []model.ReasonCode) {
	previousStatus, _ := e.cache.GetDependencyHealth(ctx, target.DependencyName, target.Region)

	isDegraded := newStatus == model.AssessmentDegraded ||
		newStatus == model.AssessmentGreyFailure ||
		newStatus == model.AssessmentOutage

	wasRecovering := previousStatus == model.AssessmentRecovering

	if isDegraded {
		// Reset recovery counter when degradation is observed.
		_ = e.cache.ResetHealthyWindowCount(ctx, target.DependencyName, target.Region)
		return newStatus, codes
	}

	if newStatus == model.AssessmentHealthy &&
		(previousStatus == model.AssessmentDegraded ||
			previousStatus == model.AssessmentGreyFailure ||
			previousStatus == model.AssessmentOutage ||
			wasRecovering) {
		// Increment healthy window counter.
		count, _ := e.cache.IncrHealthyWindowCount(ctx, target.DependencyName, target.Region, e.cfg.AssessmentCacheTTL)
		if count < e.cfg.RecoveryWindowCount {
			// Not enough healthy windows yet — still recovering.
			codes = append(codes, model.ReasonRecoveryInProgress)
			return model.AssessmentRecovering, codes
		}
		// Recovery complete.
		_ = e.cache.ResetHealthyWindowCount(ctx, target.DependencyName, target.Region)
		return model.AssessmentHealthy, codes
	}

	// Reset counter when staying healthy.
	if newStatus == model.AssessmentHealthy {
		_ = e.cache.ResetHealthyWindowCount(ctx, target.DependencyName, target.Region)
	}

	return newStatus, codes
}

// ─────────────────────────────────────────────────────────────────────────────
// Confidence computation
// ─────────────────────────────────────────────────────────────────────────────

// computeConfidence returns a categorical confidence level for the assessment.
// We do NOT claim mathematical precision — the underlying evidence doesn't justify it.
//
//   HIGH:    Both customer and synthetic signals available and agree.
//   MEDIUM:  Only one signal available, or signals disagree.
//   LOW:     Neither signal has sufficient data, or vendor status is unknown.
func (e *Engine) computeConfidence(
	customerSignal, syntheticSignal model.SignalState,
	vendorStatus model.VendorStatus,
) model.ConfidenceLevel {
	customerAvail := customerSignal != model.SignalInsufficientData
	syntheticAvail := syntheticSignal != model.SignalInsufficientData

	if customerAvail && syntheticAvail {
		return model.ConfidenceHigh
	}
	if customerAvail || syntheticAvail {
		return model.ConfidenceMedium
	}
	return model.ConfidenceLow
}

// ─────────────────────────────────────────────────────────────────────────────
// Incident management
// ─────────────────────────────────────────────────────────────────────────────

// manageIncident creates or updates an incident based on the assessment.
// Deduplication: only one active incident per dependency+region+type.
func (e *Engine) manageIncident(ctx context.Context, target EvaluationTarget, a *model.HealthAssessment) {
	isDegraded := a.Status == model.AssessmentDegraded ||
		a.Status == model.AssessmentGreyFailure ||
		a.Status == model.AssessmentOutage

	// Deduplication key: dependency + region + incident type.
	// For degradation and grey failure, a unified key is used.
	dedupKey := fmt.Sprintf("%s::%s::degradation", target.DependencyName, target.Region)
	if a.Status == model.AssessmentOutage {
		dedupKey = fmt.Sprintf("%s::%s::%s", target.DependencyName, target.Region, a.Status)
	}

	activeIncident, _ := e.store.GetActiveIncident(ctx, dedupKey)

	// If recovering or healthy and no degradation incident was found, check for an open outage incident.
	if activeIncident == nil && (a.Status == model.AssessmentHealthy || a.Status == model.AssessmentRecovering) {
		outageKey := fmt.Sprintf("%s::%s::%s", target.DependencyName, target.Region, model.AssessmentOutage)
		if inc, err := e.store.GetActiveIncident(ctx, outageKey); err == nil && inc != nil {
			activeIncident = inc
		}
	}

	if isDegraded {
		if activeIncident == nil {
			// Open a new incident.
			severity := "medium"
			if a.Status == model.AssessmentGreyFailure || a.Status == model.AssessmentOutage {
				severity = "high"
			}
			incident := model.Incident{
				ID:               uuid.New(),
				DependencyName:   target.DependencyName,
				Region:           target.Region,
				DeduplicationKey: dedupKey,
				IncidentType:     a.Status,
				Status:           model.IncidentOpen,
				Title:            incidentTitle(a),
				Severity:         severity,
				AssessmentID:     a.ID,
				StartedAt:        a.Timestamp,
				CreatedAt:        a.Timestamp,
				UpdatedAt:        a.Timestamp,
			}
			_ = e.store.UpsertIncident(ctx, incident)
			_ = e.store.InsertIncidentEvent(ctx, model.IncidentEvent{
				ID:             uuid.New(),
				IncidentID:     incident.ID,
				EventType:      model.IncidentEventDetected,
				Reason:         a.Explanation,
				AssessmentID:   a.ID,
				DependencyName: target.DependencyName,
				Region:         target.Region,
				CreatedAt:      a.Timestamp,
			})

			if e.metrics != nil {
				e.metrics.ActiveIncidents.WithLabelValues(target.DependencyName, target.Region).Inc()
			}
		} else if a.Status == model.AssessmentGreyFailure && activeIncident.IncidentType != model.AssessmentGreyFailure {
			// Escalate active degradation incident to Grey Failure
			activeIncident.IncidentType = a.Status
			activeIncident.Title = incidentTitle(a)
			activeIncident.Severity = "high"
			activeIncident.UpdatedAt = a.Timestamp
			activeIncident.AssessmentID = a.ID
			_ = e.store.UpsertIncident(ctx, *activeIncident)
			_ = e.store.InsertIncidentEvent(ctx, model.IncidentEvent{
				ID:             uuid.New(),
				IncidentID:     activeIncident.ID,
				EventType:      "escalated",
				Reason:         a.Explanation,
				AssessmentID:   a.ID,
				DependencyName: target.DependencyName,
				Region:         target.Region,
				CreatedAt:      a.Timestamp,
			})
		}
		return
	}

	// Recovery or healthy: resolve open incident if exists.
	if activeIncident != nil && a.Status == model.AssessmentHealthy {
		resolvedAt := a.Timestamp
		activeIncident.Status = model.IncidentResolved
		activeIncident.ResolvedAt = &resolvedAt
		activeIncident.UpdatedAt = a.Timestamp
		_ = e.store.UpsertIncident(ctx, *activeIncident)
		_ = e.store.InsertIncidentEvent(ctx, model.IncidentEvent{
			ID:             uuid.New(),
			IncidentID:     activeIncident.ID,
			EventType:      model.IncidentEventResolved,
			Reason:         "Dependency returned to healthy state after recovery period.",
			AssessmentID:   a.ID,
			DependencyName: target.DependencyName,
			Region:         target.Region,
			CreatedAt:      a.Timestamp,
		})

		if e.metrics != nil {
			e.metrics.ActiveIncidents.WithLabelValues(target.DependencyName, target.Region).Dec()
		}
	}

	if activeIncident != nil && a.Status == model.AssessmentRecovering {
		activeIncident.Status = model.IncidentRecovering
		activeIncident.UpdatedAt = a.Timestamp
		_ = e.store.UpsertIncident(ctx, *activeIncident)
		_ = e.store.InsertIncidentEvent(ctx, model.IncidentEvent{
			ID:             uuid.New(),
			IncidentID:     activeIncident.ID,
			EventType:      model.IncidentEventRecovering,
			Reason:         "Dependency is recovering — waiting for hysteresis window to complete.",
			AssessmentID:   a.ID,
			DependencyName: target.DependencyName,
			Region:         target.Region,
			CreatedAt:      a.Timestamp,
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func incidentTitle(a *model.HealthAssessment) string {
	switch a.Status {
	case model.AssessmentGreyFailure:
		return fmt.Sprintf("Grey Failure: %s (%s) — Operational but severely degraded",
			a.DependencyName, a.Region)
	case model.AssessmentOutage:
		return fmt.Sprintf("Outage: %s (%s) — Unavailable", a.DependencyName, a.Region)
	default:
		return fmt.Sprintf("Degradation: %s (%s) — Latency elevated", a.DependencyName, a.Region)
	}
}

func encodeReasonCodes(codes []model.ReasonCode) string {
	s := make([]string, len(codes))
	for i, c := range codes {
		s[i] = string(c)
	}
	return strings.Join(s, ",")
}
