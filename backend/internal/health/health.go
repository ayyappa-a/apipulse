// Package health implements the APIPulse Health Intelligence Engine.
//
// Phase 3C delivers:
//   - Deterministic latency baseline calculator (no ML)
//   - Multi-signal grey-failure detector
//   - Regional health assessment with hysteresis
//   - Incident lifecycle management
//
// The engine is asynchronous — it runs in a background goroutine and does NOT
// execute synchronously during customer telemetry ingestion.
//
// Privacy: The engine operates only on aggregated, normalized metrics.
// It never accesses raw request/response payloads, headers, or customer IDs.
//
// TERMINOLOGY (consistently enforced):
//   - "CodeBegun Demo Service" / "customer-demo telemetry" (not "real CodeBegun production")
//   - "vendor simulator" / "deterministic vendor simulator" (not "real Stripe")
//   - "self-hosted customer demonstration service"
package health

import (
	"context"
	"sort"

	"github.com/apipulse/apipulse/internal/telemetry/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// BaselineConfig
// ─────────────────────────────────────────────────────────────────────────────

// BaselineConfig controls the rolling baseline window and evaluation parameters.
// All thresholds are configurable — no magic numbers are hard-coded.
type BaselineConfig struct {
	// MinSamples is the minimum number of observations required to compute a
	// meaningful baseline. If fewer samples are available, status = INSUFFICIENT_DATA.
	MinSamples int

	// LatencyMultiplierThreshold is the ratio of observed P95 to baseline P95
	// required to classify a dependency as degraded.
	// Example: 3.0 means observed P95 must be >= 3× baseline P95.
	LatencyMultiplierThreshold float64

	// AbsoluteLatencyThresholdMS is the minimum absolute observed latency (ms)
	// that must be exceeded before classification as degraded.
	// This prevents false positives on micro-latency fluctuations.
	// Example: 1000ms — a 3× spike from 50ms to 150ms is not degraded.
	AbsoluteLatencyThresholdMS float64
}

// DefaultBaselineConfig returns production-reasonable defaults.
// All values can be overridden in engine configuration.
func DefaultBaselineConfig() BaselineConfig {
	return BaselineConfig{
		MinSamples:                 3,
		LatencyMultiplierThreshold: 3.0,
		AbsoluteLatencyThresholdMS: 1000.0,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Baseline calculation
// ─────────────────────────────────────────────────────────────────────────────

// computePercentile returns the value at the given percentile (0-100) from a
// sorted slice of float64 durations.
//
// Uses nearest-rank method. Returns 0 for empty slices.
func computePercentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}
	// Nearest rank: ceiling((p/100)*N), 1-indexed
	rank := int(p/100*float64(len(sorted))+0.5) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}

// ComputeDepObsBaseline calculates the latency baseline from a slice of
// DependencyObservation records.
//
// The slice does NOT need to be pre-sorted — this function sorts internally.
// Returns a BaselineResult with Status=INSUFFICIENT_DATA if fewer than
// cfg.MinSamples observations are provided.
func ComputeDepObsBaseline(
	obs []model.DependencyObservation,
	cfg BaselineConfig,
	dependencyName, region string,
) model.BaselineResult {
	result := model.BaselineResult{
		DependencyName: dependencyName,
		Region:         region,
		Source:         model.SourceCustomerOTEL,
		SampleCount:    len(obs),
	}

	if len(obs) < cfg.MinSamples {
		result.Status = model.BaselineStatusInsufficientData
		return result
	}

	var durations []float64
	for _, o := range obs {
		// Baselines represent normal operating performance — exclude explicit timeouts and errors
		if o.Status == model.StatusTimeout || o.Status == model.StatusError {
			continue
		}
		durations = append(durations, o.DurationMS)
	}

	if len(durations) < cfg.MinSamples {
		result.Status = model.BaselineStatusInsufficientData
		return result
	}

	// Protect baseline from being poisoned by active degradation / chaos tests:
	// If nominal operating observations exist (<= AbsoluteLatencyThresholdMS), compute baseline from them.
	var nominalDurations []float64
	for _, d := range durations {
		if d <= cfg.AbsoluteLatencyThresholdMS {
			nominalDurations = append(nominalDurations, d)
		}
	}
	if len(nominalDurations) > 0 {
		durations = nominalDurations
	}

	sort.Float64s(durations)

	result.P50MS = computePercentile(durations, 50)
	result.P95MS = computePercentile(durations, 95)
	result.P99MS = computePercentile(durations, 99)
	result.Status = model.BaselineStatusReady
	return result
}

// ComputeProbeBaseline calculates the latency baseline from a slice of
// ProbeObservation records (SYNTHETIC_PROBE signal).
func ComputeProbeBaseline(
	obs []model.ProbeObservation,
	cfg BaselineConfig,
	dependencyName, region string,
) model.BaselineResult {
	result := model.BaselineResult{
		DependencyName: dependencyName,
		Region:         region,
		Source:         model.SourceSyntheticProbe,
		SampleCount:    len(obs),
	}

	if len(obs) < cfg.MinSamples {
		result.Status = model.BaselineStatusInsufficientData
		return result
	}

	var durations []float64
	for _, o := range obs {
		if !o.Success || o.Timeout {
			continue
		}
		durations = append(durations, o.TotalDurationMS)
	}

	if len(durations) < cfg.MinSamples {
		result.Status = model.BaselineStatusInsufficientData
		return result
	}

	var nominalDurations []float64
	for _, d := range durations {
		if d <= cfg.AbsoluteLatencyThresholdMS {
			nominalDurations = append(nominalDurations, d)
		}
	}
	if len(nominalDurations) > 0 {
		durations = nominalDurations
	}

	sort.Float64s(durations)

	result.P50MS = computePercentile(durations, 50)
	result.P95MS = computePercentile(durations, 95)
	result.P99MS = computePercentile(durations, 99)
	result.Status = model.BaselineStatusReady
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// Signal evaluation helpers
// ─────────────────────────────────────────────────────────────────────────────

// EvaluateSignal evaluates a single signal's latency against its baseline.
// Returns (signalState, latencyMultiplier).
//
// Rules:
//   - If baseline is INSUFFICIENT_DATA → SignalInsufficientData
//   - If observedP95 >= cfg.LatencyMultiplierThreshold × baselineP95
//     AND observedP95 >= cfg.AbsoluteLatencyThresholdMS → SignalDegraded
//   - Otherwise → SignalHealthy
//
// IMPORTANT: HTTP 200 does NOT make a signal healthy.
// Latency is evaluated independently from HTTP status codes.
func EvaluateSignal(
	observedP95MS float64,
	baseline model.BaselineResult,
	cfg BaselineConfig,
) (model.SignalState, float64) {
	if baseline.Status == model.BaselineStatusInsufficientData {
		return model.SignalInsufficientData, 0
	}

	effectiveBaseline := baseline.P95MS
	if effectiveBaseline > cfg.AbsoluteLatencyThresholdMS {
		effectiveBaseline = cfg.AbsoluteLatencyThresholdMS
	}

	if effectiveBaseline == 0 {
		// If baseline P95 is zero (very fast, near-instant), use absolute threshold only.
		if observedP95MS >= cfg.AbsoluteLatencyThresholdMS {
			return model.SignalDegraded, 0
		}
		return model.SignalHealthy, 1.0
	}

	multiplier := observedP95MS / effectiveBaseline

	if multiplier >= cfg.LatencyMultiplierThreshold &&
		observedP95MS >= cfg.AbsoluteLatencyThresholdMS {
		return model.SignalDegraded, multiplier
	}

	return model.SignalHealthy, multiplier
}

// P95FromDepObs computes P95 latency from a recent observation window.
// Returns 0 and false if there are no observations.
func P95FromDepObs(obs []model.DependencyObservation) (p95 float64, ok bool) {
	if len(obs) == 0 {
		return 0, false
	}
	durations := make([]float64, len(obs))
	for i, o := range obs {
		durations[i] = o.DurationMS
	}
	sort.Float64s(durations)
	return computePercentile(durations, 95), true
}

// P95FromProbeObs computes P95 total duration from a recent probe observation window.
func P95FromProbeObs(obs []model.ProbeObservation) (p95 float64, ok bool) {
	if len(obs) == 0 {
		return 0, false
	}
	durations := make([]float64, len(obs))
	for i, o := range obs {
		durations[i] = o.TotalDurationMS
	}
	sort.Float64s(durations)
	return computePercentile(durations, 95), true
}

// ─────────────────────────────────────────────────────────────────────────────
// Context-aware no-op placeholder for future async engine
// ─────────────────────────────────────────────────────────────────────────────

// Ensure the context import is used for the engine interface.
var _ = context.Background
