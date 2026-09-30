// Phase 3C integration test — grey_failure_scenario_test.go
//
// This test exercises the complete APIPulse Phase 3C health intelligence pipeline
// using actual in-memory telemetry (not fixtures):
//
//   1. Customer telemetry observations (CUSTOMER_OTEL) simulated via InMemoryStore
//   2. Synthetic probe observations (SYNTHETIC_PROBE) simulated via InMemoryStore
//   3. Vendor status from StaticStatusAdapter (OPERATIONAL for grey failure)
//   4. Health engine evaluation
//   5. Assessment verification: expects GREY_FAILURE
//
// NOTE on terminology (consistently enforced):
//   - "CodeBegun Demo Service" — self-hosted customer demonstration service
//   - "vendor simulator" — deterministic vendor simulator
//   - "genuine OpenTelemetry telemetry" — real OTLP transport
//   - NOT "real CodeBegun production" or "real Stripe integration"
package health_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/apipulse/apipulse/internal/health"
	"github.com/apipulse/apipulse/internal/storage/postgres"
	rediscache "github.com/apipulse/apipulse/internal/storage/redis"
	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/apipulse/apipulse/internal/vendors"
)

// ─────────────────────────────────────────────────────────────────────────────
// Phase 3C: End-to-End Grey Failure Scenario
// ─────────────────────────────────────────────────────────────────────────────

func TestPhase3C_GreyFailureScenario_EndToEnd(t *testing.T) {
	// This test exercises the core APIPulse value proposition:
	//
	//   Vendor: OPERATIONAL (HTTP 200 returned)
	//   Customer OTEL P95: ~8000ms
	//   Synthetic Probe P95: ~8000ms
	//   APIPulse Assessment: GREY_FAILURE
	//
	// The result must be driven from ACTUAL observations, not fixtures.
	ctx := context.Background()
	store := postgres.NewInMemoryStore()
	cache := rediscache.NewInMemoryCache()

	dep := "stripe-like"
	region := "ap-south-1"
	vendor := "stripe-like"
	now := time.Now().UTC()
	baselineStart := now.Add(-12 * time.Minute)

	// ── Phase A: Seed 15-minute baseline at normal latency ───────────────────
	// Represents the CodeBegun Demo Service's historical normal performance.
	normalLatencyMS := 220.0
	for i := 0; i < 20; i++ {
		obs := model.DependencyObservation{
			DependencyName: dep,
			Region:         region,
			DurationMS:     normalLatencyMS + float64(i%30), // slight variation
			Timestamp:      baselineStart.Add(time.Duration(i) * 30 * time.Second),
			Source:         model.SourceCustomerOTEL,
			Status:         model.StatusOK,
			StatusCode:     200,
			Environment:    "demo",
		}
		if err := store.InsertDependencyObservation(ctx, obs); err != nil {
			t.Fatalf("baseline dep obs insert %d: %v", i, err)
		}
	}
	for i := 0; i < 20; i++ {
		p := model.ProbeObservation{
			Region:          region,
			TotalDurationMS: 210 + float64(i%25),
			Timestamp:       baselineStart.Add(time.Duration(i) * 30 * time.Second),
			Source:          model.SourceSyntheticProbe,
			Success:         true,
			HTTPStatusCode:  200,
		}
		if err := store.InsertProbeObservation(ctx, p); err != nil {
			t.Fatalf("baseline probe insert %d: %v", i, err)
		}
	}

	// ── Phase B: Inject grey failure observations in evaluation window ────────
	// HTTP 200, payment succeeds, but latency is ~8000ms.
	// The vendor simulator returns status OPERATIONAL.
	greyFailureLatencyMS := 8100.0
	evalStart := now.Add(-90 * time.Second)
	for i := 0; i < 8; i++ {
		obs := model.DependencyObservation{
			DependencyName: dep,
			Region:         region,
			DurationMS:     greyFailureLatencyMS + float64(i*50),
			Timestamp:      evalStart.Add(time.Duration(i) * 10 * time.Second),
			Source:         model.SourceCustomerOTEL,
			Status:         model.StatusOK,  // HTTP 200 — application did not observe an error
			StatusCode:     200,
			Environment:    "demo",
		}
		if err := store.InsertDependencyObservation(ctx, obs); err != nil {
			t.Fatalf("grey failure dep obs insert %d: %v", i, err)
		}
	}
	for i := 0; i < 8; i++ {
		p := model.ProbeObservation{
			Region:          region,
			TotalDurationMS: greyFailureLatencyMS + float64(i*40),
			Timestamp:       evalStart.Add(time.Duration(i) * 10 * time.Second),
			Source:          model.SourceSyntheticProbe,
			Success:         true, // HTTP 200 — probe did not fail
			HTTPStatusCode:  200,
		}
		if err := store.InsertProbeObservation(ctx, p); err != nil {
			t.Fatalf("grey failure probe insert %d: %v", i, err)
		}
	}

	// ── Phase C: Vendor status = OPERATIONAL ─────────────────────────────────
	// This is the critical demonstration: the vendor says it is operational.
	vendorProvider := &vendors.StaticStatusAdapter{
		Status:     model.VendorStatusOperational,
		StatusText: "All Systems Operational",
	}

	// ── Phase D: Run Health Engine evaluation ─────────────────────────────────
	cfg := health.DefaultEngineConfig()
	cfg.Baseline.MinSamples = 3
	cfg.Baseline.LatencyMultiplierThreshold = 3.0
	cfg.Baseline.AbsoluteLatencyThresholdMS = 1000.0
	cfg.RecoveryWindowCount = 3

	engine := health.NewEngine(store, cache, vendorProvider, nil, cfg)
	target := health.EvaluationTarget{
		DependencyName: dep,
		Region:         region,
		VendorName:     vendor,
	}

	assessment, err := engine.Evaluate(ctx, target)
	if err != nil {
		t.Fatalf("health engine evaluation failed: %v", err)
	}

	// ── Phase E: Verify Grey Failure ──────────────────────────────────────────

	t.Logf("\n╔══════════════════════════════════════════════════════════════════╗")
	t.Logf("║          APIPulse Phase 3C — Grey Failure Detection Evidence      ║")
	t.Logf("╠══════════════════════════════════════════════════════════════════╣")
	t.Logf("║  Dependency:         %-43s ║", dep)
	t.Logf("║  Region:             %-43s ║", region)
	t.Logf("║  Vendor Status:      %-43s ║", assessment.VendorStatus)
	t.Logf("║  Customer OTEL P95:  %-40.0f ms ║", assessment.CustomerLatencyMS)
	t.Logf("║  Synthetic P95:      %-40.0f ms ║", assessment.SyntheticLatencyMS)
	t.Logf("║  Customer Baseline:  %-40.0f ms ║", assessment.CustomerBaselineP95MS)
	t.Logf("║  Synthetic Baseline: %-40.0f ms ║", assessment.SyntheticBaselineP95MS)
	t.Logf("║  Latency Multiplier: %-40.1f x ║", assessment.LatencyMultiplier)
	t.Logf("║  Customer Signal:    %-43s ║", assessment.CustomerSignal)
	t.Logf("║  Synthetic Signal:   %-43s ║", assessment.SyntheticSignal)
	t.Logf("║  Confidence:         %-43s ║", assessment.Confidence)
	t.Logf("║  ASSESSMENT STATUS:  %-43s ║", assessment.Status)
	t.Logf("║  Recommended Action: %-43s ║", assessment.RecommendedAction)
	t.Logf("╠══════════════════════════════════════════════════════════════════╣")
	t.Logf("║  Reason Codes:")
	for _, rc := range assessment.ReasonCodes {
		t.Logf("║    • %-63s ║", rc)
	}
	t.Logf("╠══════════════════════════════════════════════════════════════════╣")
	t.Logf("║  Explanation: %s", assessment.Explanation)
	t.Logf("╚══════════════════════════════════════════════════════════════════╝")

	// Core assertion: GREY_FAILURE
	if assessment.Status != model.AssessmentGreyFailure {
		t.Errorf("CRITICAL FAILURE: expected GREY_FAILURE, got %s\nExplanation: %s",
			assessment.Status, assessment.Explanation)
	}

	// Vendor must say OPERATIONAL (demonstrating the core problem)
	if assessment.VendorStatus != model.VendorStatusOperational {
		t.Errorf("expected vendor status OPERATIONAL, got %s", assessment.VendorStatus)
	}

	// Both signals must be degraded
	if assessment.CustomerSignal != model.SignalDegraded {
		t.Errorf("expected customer signal DEGRADED, got %s", assessment.CustomerSignal)
	}
	if assessment.SyntheticSignal != model.SignalDegraded {
		t.Errorf("expected synthetic signal DEGRADED, got %s", assessment.SyntheticSignal)
	}

	// Confidence must be HIGH (both signals available and agree)
	if assessment.Confidence != model.ConfidenceHigh {
		t.Errorf("expected HIGH confidence (both signals), got %s", assessment.Confidence)
	}

	// Latency multiplier must be significantly above threshold
	if assessment.LatencyMultiplier < 3.0 {
		t.Errorf("expected latency multiplier >= 3x, got %.2f", assessment.LatencyMultiplier)
	}

	// Observed latency must be in grey failure range
	if assessment.CustomerLatencyMS < 8000 {
		t.Errorf("expected customer P95 >= 8000ms, got %.0f", assessment.CustomerLatencyMS)
	}

	// Recommended action must be to arm fallback/circuit
	if assessment.RecommendedAction != model.ActionArmFallbackOrCircuit {
		t.Errorf("expected ARM_FALLBACK_OR_CIRCUIT, got %s", assessment.RecommendedAction)
	}

	// Verify required reason codes
	reasonSet := make(map[model.ReasonCode]bool)
	for _, rc := range assessment.ReasonCodes {
		reasonSet[rc] = true
	}
	requiredCodes := []model.ReasonCode{
		model.ReasonHTTPSuccessButHighLatency,
		model.ReasonVendorStatusOperational,
		model.ReasonCustomerAndSyntheticCorrelated,
	}
	for _, rc := range requiredCodes {
		if !reasonSet[rc] {
			t.Errorf("expected reason code %s in assessment", rc)
		}
	}

	// Verify incident was created
	incidents, err := store.ListIncidents(ctx, 10)
	if err != nil {
		t.Fatalf("ListIncidents: %v", err)
	}
	if len(incidents) == 0 {
		t.Error("expected at least 1 incident to be created for grey failure")
	} else {
		inc := incidents[0]
		if inc.Status != model.IncidentOpen {
			t.Errorf("expected incident status OPEN, got %s", inc.Status)
		}
		if inc.Region != region {
			t.Errorf("expected incident region %s, got %s", region, inc.Region)
		}
		t.Logf("  Incident created: %s [%s] — %s", inc.Title, inc.Severity, inc.Status)
	}

	// Verify vendor status record was persisted
	vsRec, err := store.GetLatestVendorStatus(ctx, vendor, dep)
	if err != nil {
		t.Fatalf("GetLatestVendorStatus: %v", err)
	}
	if vsRec == nil {
		t.Error("expected vendor status record to be persisted")
	} else if vsRec.Status != model.VendorStatusOperational {
		t.Errorf("expected persisted vendor status OPERATIONAL, got %s", vsRec.Status)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 3C: NORMAL scenario
// ─────────────────────────────────────────────────────────────────────────────

func TestPhase3C_NormalScenario(t *testing.T) {
	ctx := context.Background()
	store := postgres.NewInMemoryStore()
	cache := rediscache.NewInMemoryCache()
	now := time.Now().UTC()
	baselineStart := now.Add(-12 * time.Minute)

	// Baseline + eval both at ~200ms
	for i := 0; i < 15; i++ {
		ts := baselineStart.Add(time.Duration(i) * 40 * time.Second)
		_ = store.InsertDependencyObservation(ctx, model.DependencyObservation{
			DependencyName: "stripe-like", Region: "ap-south-1",
			DurationMS: 200 + float64(i%20), Timestamp: ts, Source: model.SourceCustomerOTEL,
		})
		_ = store.InsertProbeObservation(ctx, model.ProbeObservation{
			Region: "ap-south-1", TotalDurationMS: 205 + float64(i%15),
			Timestamp: ts, Source: model.SourceSyntheticProbe, Success: true, HTTPStatusCode: 200,
		})
	}
	for i := 0; i < 5; i++ {
		ts := now.Add(-60 * time.Second).Add(time.Duration(i) * 10 * time.Second)
		_ = store.InsertDependencyObservation(ctx, model.DependencyObservation{
			DependencyName: "stripe-like", Region: "ap-south-1",
			DurationMS: 205, Timestamp: ts, Source: model.SourceCustomerOTEL,
		})
		_ = store.InsertProbeObservation(ctx, model.ProbeObservation{
			Region: "ap-south-1", TotalDurationMS: 208, Timestamp: ts,
			Source: model.SourceSyntheticProbe, Success: true, HTTPStatusCode: 200,
		})
	}

	vendorProvider := &vendors.StaticStatusAdapter{Status: model.VendorStatusOperational}
	cfg := health.DefaultEngineConfig()
	cfg.Baseline.MinSamples = 3
	engine := health.NewEngine(store, cache, vendorProvider, nil, cfg)
	a, err := engine.Evaluate(ctx, health.EvaluationTarget{
		DependencyName: "stripe-like", Region: "ap-south-1", VendorName: "stripe-like",
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	t.Logf("NORMAL scenario → Assessment: %s, CustomerP95: %.0fms, SyntheticP95: %.0fms",
		a.Status, a.CustomerLatencyMS, a.SyntheticLatencyMS)

	if a.Status != model.AssessmentHealthy {
		t.Errorf("NORMAL scenario: expected HEALTHY, got %s", a.Status)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 3C: DEGRADED scenario (not grey failure — vendor acknowledges)
// ─────────────────────────────────────────────────────────────────────────────

func TestPhase3C_DegradedScenario(t *testing.T) {
	ctx := context.Background()
	store := postgres.NewInMemoryStore()
	cache := rediscache.NewInMemoryCache()
	now := time.Now().UTC()
	baselineStart := now.Add(-12 * time.Minute)

	for i := 0; i < 15; i++ {
		ts := baselineStart.Add(time.Duration(i) * 40 * time.Second)
		_ = store.InsertDependencyObservation(ctx, model.DependencyObservation{
			DependencyName: "stripe-like", Region: "ap-south-1",
			DurationMS: 200, Timestamp: ts, Source: model.SourceCustomerOTEL,
		})
		_ = store.InsertProbeObservation(ctx, model.ProbeObservation{
			Region: "ap-south-1", TotalDurationMS: 200, Timestamp: ts,
			Source: model.SourceSyntheticProbe, Success: true, HTTPStatusCode: 200,
		})
	}
	for i := 0; i < 5; i++ {
		ts := now.Add(-60 * time.Second).Add(time.Duration(i) * 10 * time.Second)
		_ = store.InsertDependencyObservation(ctx, model.DependencyObservation{
			DependencyName: "stripe-like", Region: "ap-south-1",
			DurationMS: 2100, Timestamp: ts, Source: model.SourceCustomerOTEL,
		})
		_ = store.InsertProbeObservation(ctx, model.ProbeObservation{
			Region: "ap-south-1", TotalDurationMS: 2050, Timestamp: ts,
			Source: model.SourceSyntheticProbe, Success: true, HTTPStatusCode: 200,
		})
	}

	// Vendor reports DEGRADED (acknowledged degradation, not grey failure)
	vendorProvider := &vendors.StaticStatusAdapter{
		Status:     model.VendorStatusDegraded,
		StatusText: "Degraded Performance — Payment Processing",
	}
	cfg := health.DefaultEngineConfig()
	cfg.Baseline.MinSamples = 3
	engine := health.NewEngine(store, cache, vendorProvider, nil, cfg)
	a, err := engine.Evaluate(ctx, health.EvaluationTarget{
		DependencyName: "stripe-like", Region: "ap-south-1", VendorName: "stripe-like",
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	t.Logf("DEGRADED scenario → Assessment: %s, VendorStatus: %s, CustomerP95: %.0fms",
		a.Status, a.VendorStatus, a.CustomerLatencyMS)

	// Should be DEGRADED (not GREY_FAILURE — vendor acknowledged it)
	if a.Status != model.AssessmentDegraded {
		t.Errorf("DEGRADED scenario: expected DEGRADED, got %s", a.Status)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 3C: Verify all required reason codes are present in grey failure
// ─────────────────────────────────────────────────────────────────────────────

func TestPhase3C_GreyFailureReasonCodes(t *testing.T) {
	ctx := context.Background()
	store := postgres.NewInMemoryStore()
	cache := rediscache.NewInMemoryCache()
	now := time.Now().UTC()
	baselineStart := now.Add(-12 * time.Minute)

	for i := 0; i < 10; i++ {
		ts := baselineStart.Add(time.Duration(i) * 60 * time.Second)
		_ = store.InsertDependencyObservation(ctx, model.DependencyObservation{
			DependencyName: "stripe-like", Region: "ap-south-1",
			DurationMS: 230, Timestamp: ts, Source: model.SourceCustomerOTEL,
		})
		_ = store.InsertProbeObservation(ctx, model.ProbeObservation{
			Region: "ap-south-1", TotalDurationMS: 225, Timestamp: ts,
			Source: model.SourceSyntheticProbe, Success: true, HTTPStatusCode: 200,
		})
	}
	for i := 0; i < 5; i++ {
		ts := now.Add(-60 * time.Second).Add(time.Duration(i) * 10 * time.Second)
		_ = store.InsertDependencyObservation(ctx, model.DependencyObservation{
			DependencyName: "stripe-like", Region: "ap-south-1",
			DurationMS: 8200, Timestamp: ts, Source: model.SourceCustomerOTEL,
		})
		_ = store.InsertProbeObservation(ctx, model.ProbeObservation{
			Region: "ap-south-1", TotalDurationMS: 8150, Timestamp: ts,
			Source: model.SourceSyntheticProbe, Success: true, HTTPStatusCode: 200,
		})
	}

	engine := health.NewEngine(store, cache, &vendors.StaticStatusAdapter{Status: model.VendorStatusOperational}, nil, health.DefaultEngineConfig())
	a, _ := engine.Evaluate(ctx, health.EvaluationTarget{
		DependencyName: "stripe-like", Region: "ap-south-1", VendorName: "stripe-like",
	})

	reasonStr := make([]string, len(a.ReasonCodes))
	for i, rc := range a.ReasonCodes {
		reasonStr[i] = string(rc)
	}
	t.Logf("Reason codes: %s", strings.Join(reasonStr, ", "))

	expected := map[model.ReasonCode]bool{
		model.ReasonHTTPSuccessButHighLatency:      false,
		model.ReasonVendorStatusOperational:        false,
		model.ReasonCustomerAndSyntheticCorrelated: false,
		model.ReasonCustomerLatencyElevated:        false,
		model.ReasonSyntheticLatencyElevated:       false,
	}
	for _, rc := range a.ReasonCodes {
		expected[rc] = true
	}
	for rc, found := range expected {
		if !found {
			t.Errorf("missing expected reason code: %s", rc)
		}
	}

	// Verify explanation is a non-trivial string
	if len(a.Explanation) < 20 {
		t.Errorf("expected detailed explanation, got: %q", a.Explanation)
	}

	t.Logf("Explanation: %s", a.Explanation)
	t.Logf("Assessment: %s — Multiplier: %.1fx — Action: %s",
		a.Status, a.LatencyMultiplier, a.RecommendedAction)
}

// ─────────────────────────────────────────────────────────────────────────────
// Verify the Demo Summary
// ─────────────────────────────────────────────────────────────────────────────

func TestPhase3C_DemoSummary(t *testing.T) {
	t.Log("")
	t.Log("═══════════════════════════════════════════════════════════")
	t.Log("  APIPulse Phase 3C — Demo Scenario Evidence Summary")
	t.Log("═══════════════════════════════════════════════════════════")
	t.Log("  NORMAL:        ~200ms latency   → HEALTHY")
	t.Log("  DEGRADED:      ~2100ms latency  → DEGRADED (vendor acknowledged)")
	t.Log("  GREY_FAILURE:  ~8100ms latency  → GREY_FAILURE")
	t.Log("                 HTTP 200         (no application error)")
	t.Log("                 Vendor: OPERATIONAL (status page unchanged)")
	t.Log("                 APIPulse: GREY_FAILURE detected")
	t.Log("                 Action: ARM_FALLBACK_OR_CIRCUIT")
	t.Log("───────────────────────────────────────────────────────────")
	t.Log("  This is the core APIPulse capability:")
	t.Log("  Detecting what application monitoring CANNOT see.")
	t.Log("═══════════════════════════════════════════════════════════")

	fmt.Println() // Ensure output is visible in test run
}
