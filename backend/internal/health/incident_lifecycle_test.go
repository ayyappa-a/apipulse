package health

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/apipulse/apipulse/internal/storage/postgres"
	rediscache "github.com/apipulse/apipulse/internal/storage/redis"
	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/apipulse/apipulse/internal/vendors"
)

// setupTestEngine creates an Engine backed by InMemoryStore and InMemoryCache.
func setupTestEngine() (*Engine, postgres.TelemetryStore) {
	store := postgres.NewInMemoryStore()
	cache := rediscache.NewInMemoryCache()
	vendorAdapter := &vendors.StaticStatusAdapter{Status: model.VendorStatusOperational}
	engine := NewEngine(store, cache, vendorAdapter, nil, DefaultEngineConfig())
	return engine, store
}

// Test 1 — Grey failure opens degradation incident
func TestIncidentLifecycle_Test1_GreyFailureOpensDegradationIncident(t *testing.T) {
	ctx := context.Background()
	engine, store := setupTestEngine()

	target := EvaluationTarget{
		DependencyName: "stripe-like",
		Region:         "ap-south-1",
		VendorName:     "stripe-like",
	}
	now := time.Now().UTC()

	greyFailureAssessment := &model.HealthAssessment{
		ID:             uuid.New(),
		DependencyName: target.DependencyName,
		Region:         target.Region,
		Status:         model.AssessmentGreyFailure,
		Timestamp:      now,
		Explanation:    "Elevated latency observed across customer OTel and synthetic probes",
	}

	engine.manageIncident(ctx, target, greyFailureAssessment)

	expectedDedupKey := "stripe-like::ap-south-1::degradation"
	active, err := store.GetActiveIncident(ctx, expectedDedupKey)
	if err != nil {
		t.Fatalf("GetActiveIncident failed: %v", err)
	}
	if active == nil {
		t.Fatalf("expected active incident for deduplication key %q, but got nil", expectedDedupKey)
	}

	if active.Status != model.IncidentOpen {
		t.Errorf("expected incident status %s, got %s", model.IncidentOpen, active.Status)
	}
	if active.DeduplicationKey != expectedDedupKey {
		t.Errorf("expected deduplication key %q, got %q", expectedDedupKey, active.DeduplicationKey)
	}
	if active.IncidentType != model.AssessmentGreyFailure {
		t.Errorf("expected incident type %s, got %s", model.AssessmentGreyFailure, active.IncidentType)
	}
	if active.AssessmentID != greyFailureAssessment.ID {
		t.Errorf("expected assessment ID %s, got %s", greyFailureAssessment.ID, active.AssessmentID)
	}

	// Verify detection event was created
	inc, err := store.GetIncident(ctx, active.ID.String())
	if err != nil {
		t.Fatalf("GetIncident failed: %v", err)
	}
	if len(inc.Events) == 0 {
		t.Fatal("expected at least one incident event, got 0")
	}
	if inc.Events[0].EventType != model.IncidentEventDetected {
		t.Errorf("expected first event to be %s, got %s", model.IncidentEventDetected, inc.Events[0].EventType)
	}
}

// Test 2 — Recovering finds existing degradation incident
func TestIncidentLifecycle_Test2_RecoveringFindsExistingDegradationIncident(t *testing.T) {
	ctx := context.Background()
	engine, store := setupTestEngine()

	target := EvaluationTarget{
		DependencyName: "stripe-like",
		Region:         "ap-south-1",
		VendorName:     "stripe-like",
	}
	now := time.Now().UTC()

	// 1. Open an existing degradation incident via GREY_FAILURE
	greyFailureAssessment := &model.HealthAssessment{
		ID:             uuid.New(),
		DependencyName: target.DependencyName,
		Region:         target.Region,
		Status:         model.AssessmentGreyFailure,
		Timestamp:      now,
		Explanation:    "Grey failure detected",
	}
	engine.manageIncident(ctx, target, greyFailureAssessment)

	dedupKey := "stripe-like::ap-south-1::degradation"
	originalActive, err := store.GetActiveIncident(ctx, dedupKey)
	if err != nil || originalActive == nil {
		t.Fatalf("setup failed: expected active degradation incident, got %v", originalActive)
	}
	originalID := originalActive.ID

	// 2. Assess as RECOVERING
	recoveringAssessment := &model.HealthAssessment{
		ID:             uuid.New(),
		DependencyName: target.DependencyName,
		Region:         target.Region,
		Status:         model.AssessmentRecovering,
		Timestamp:      now.Add(30 * time.Second),
		Explanation:    "Latency recovering",
	}
	engine.manageIncident(ctx, target, recoveringAssessment)

	// Verify that NO duplicate incident with key 'stripe-like::ap-south-1::RECOVERING' was created
	recoveringKey := "stripe-like::ap-south-1::RECOVERING"
	bogusIncident, _ := store.GetActiveIncident(ctx, recoveringKey)
	if bogusIncident != nil {
		t.Fatalf("bug detected: created duplicate incident with deduplication key %q", recoveringKey)
	}

	// Verify the original degradation incident was found and updated to RECOVERING
	updatedIncident, err := store.GetActiveIncident(ctx, dedupKey)
	if err != nil {
		t.Fatalf("GetActiveIncident failed: %v", err)
	}
	if updatedIncident == nil {
		t.Fatalf("expected existing incident with key %q to be found, got nil", dedupKey)
	}
	if updatedIncident.ID != originalID {
		t.Errorf("expected same incident ID %s, got %s", originalID, updatedIncident.ID)
	}
	if updatedIncident.Status != model.IncidentRecovering {
		t.Errorf("expected status %s, got %s", model.IncidentRecovering, updatedIncident.Status)
	}

	// Verify recovering event was recorded
	inc, err := store.GetIncident(ctx, originalID.String())
	if err != nil {
		t.Fatalf("GetIncident failed: %v", err)
	}
	hasRecoveringEvent := false
	for _, evt := range inc.Events {
		if evt.EventType == model.IncidentEventRecovering {
			hasRecoveringEvent = true
			break
		}
	}
	if !hasRecoveringEvent {
		t.Errorf("expected incident events to include %s", model.IncidentEventRecovering)
	}
}

// Test 3 — Healthy resolves existing degradation incident
func TestIncidentLifecycle_Test3_HealthyResolvesExistingDegradationIncident(t *testing.T) {
	ctx := context.Background()
	engine, store := setupTestEngine()

	target := EvaluationTarget{
		DependencyName: "stripe-like",
		Region:         "ap-south-1",
		VendorName:     "stripe-like",
	}
	now := time.Now().UTC()

	// 1. Open degradation incident via GREY_FAILURE
	greyFailureAssessment := &model.HealthAssessment{
		ID:             uuid.New(),
		DependencyName: target.DependencyName,
		Region:         target.Region,
		Status:         model.AssessmentGreyFailure,
		Timestamp:      now,
		Explanation:    "Grey failure detected",
	}
	engine.manageIncident(ctx, target, greyFailureAssessment)

	dedupKey := "stripe-like::ap-south-1::degradation"
	active, err := store.GetActiveIncident(ctx, dedupKey)
	if err != nil || active == nil {
		t.Fatalf("setup failed: expected active degradation incident, got %v", active)
	}
	originalID := active.ID

	// 2. Assess as RECOVERING
	recoveringAssessment := &model.HealthAssessment{
		ID:             uuid.New(),
		DependencyName: target.DependencyName,
		Region:         target.Region,
		Status:         model.AssessmentRecovering,
		Timestamp:      now.Add(30 * time.Second),
		Explanation:    "Latency recovering",
	}
	engine.manageIncident(ctx, target, recoveringAssessment)

	// 3. Assess as HEALTHY
	healthyTime := now.Add(60 * time.Second)
	healthyAssessment := &model.HealthAssessment{
		ID:             uuid.New(),
		DependencyName: target.DependencyName,
		Region:         target.Region,
		Status:         model.AssessmentHealthy,
		Timestamp:      healthyTime,
		Explanation:    "Service returned to healthy",
	}
	engine.manageIncident(ctx, target, healthyAssessment)

	// Verify no bogus healthy incident was created
	healthyKey := "stripe-like::ap-south-1::HEALTHY"
	bogusHealthy, _ := store.GetActiveIncident(ctx, healthyKey)
	if bogusHealthy != nil {
		t.Fatalf("bug detected: created duplicate incident with deduplication key %q", healthyKey)
	}

	// Verify active incident lookup returns nil (incident is resolved)
	activeAfterResolution, _ := store.GetActiveIncident(ctx, dedupKey)
	if activeAfterResolution != nil {
		t.Errorf("expected no active incident after resolution, but found status=%s", activeAfterResolution.Status)
	}

	// Verify exactly ONE incident exists in store
	allIncidents, err := store.ListIncidents(ctx, 10)
	if err != nil {
		t.Fatalf("ListIncidents failed: %v", err)
	}
	if len(allIncidents) != 1 {
		t.Fatalf("expected exactly 1 incident in store, found %d", len(allIncidents))
	}

	// Verify original incident transitioned to RESOLVED
	resolvedInc, err := store.GetIncident(ctx, originalID.String())
	if err != nil {
		t.Fatalf("GetIncident failed: %v", err)
	}
	if resolvedInc.ID != originalID {
		t.Errorf("expected same incident ID %s, got %s", originalID, resolvedInc.ID)
	}
	if resolvedInc.Status != model.IncidentResolved {
		t.Errorf("expected status %s, got %s", model.IncidentResolved, resolvedInc.Status)
	}
	if resolvedInc.ResolvedAt == nil {
		t.Fatal("expected resolved_at timestamp to be populated, got nil")
	}
	if !resolvedInc.ResolvedAt.Equal(healthyTime) {
		t.Errorf("expected resolved_at to equal %v, got %v", healthyTime, *resolvedInc.ResolvedAt)
	}

	// Verify resolution event was created
	hasResolvedEvent := false
	for _, evt := range resolvedInc.Events {
		if evt.EventType == model.IncidentEventResolved {
			hasResolvedEvent = true
			if evt.AssessmentID != healthyAssessment.ID {
				t.Errorf("expected resolution event assessment ID %s, got %s", healthyAssessment.ID, evt.AssessmentID)
			}
			break
		}
	}
	if !hasResolvedEvent {
		t.Errorf("expected incident events to include %s", model.IncidentEventResolved)
	}
}

// Test 4 — Outage behavior remains intact
func TestIncidentLifecycle_Test4_OutageBehaviorIntact(t *testing.T) {
	ctx := context.Background()
	engine, store := setupTestEngine()

	target := EvaluationTarget{
		DependencyName: "stripe-like",
		Region:         "ap-south-1",
		VendorName:     "stripe-like",
	}
	now := time.Now().UTC()

	// 1. OUTAGE assessment should create deduplication key dependency::region::OUTAGE
	outageAssessment := &model.HealthAssessment{
		ID:             uuid.New(),
		DependencyName: target.DependencyName,
		Region:         target.Region,
		Status:         model.AssessmentOutage,
		Timestamp:      now,
		Explanation:    "Complete vendor outage",
	}
	engine.manageIncident(ctx, target, outageAssessment)

	outageDedupKey := "stripe-like::ap-south-1::OUTAGE"
	activeOutage, err := store.GetActiveIncident(ctx, outageDedupKey)
	if err != nil {
		t.Fatalf("GetActiveIncident failed: %v", err)
	}
	if activeOutage == nil {
		t.Fatalf("expected active outage incident for %q, got nil", outageDedupKey)
	}
	if activeOutage.Status != model.IncidentOpen {
		t.Errorf("expected status %s, got %s", model.IncidentOpen, activeOutage.Status)
	}
	if activeOutage.DeduplicationKey != outageDedupKey {
		t.Errorf("expected deduplication key %q, got %q", outageDedupKey, activeOutage.DeduplicationKey)
	}
	if activeOutage.IncidentType != model.AssessmentOutage {
		t.Errorf("expected incident type %s, got %s", model.AssessmentOutage, activeOutage.IncidentType)
	}
	originalOutageID := activeOutage.ID

	// 2. HEALTHY assessment resolves the active outage incident
	healthyTime := now.Add(45 * time.Second)
	healthyAssessment := &model.HealthAssessment{
		ID:             uuid.New(),
		DependencyName: target.DependencyName,
		Region:         target.Region,
		Status:         model.AssessmentHealthy,
		Timestamp:      healthyTime,
		Explanation:    "Outage resolved",
	}
	engine.manageIncident(ctx, target, healthyAssessment)

	activeOutageAfter, _ := store.GetActiveIncident(ctx, outageDedupKey)
	if activeOutageAfter != nil {
		t.Errorf("expected outage incident to be resolved, got active status=%s", activeOutageAfter.Status)
	}

	resolvedOutage, err := store.GetIncident(ctx, originalOutageID.String())
	if err != nil {
		t.Fatalf("GetIncident failed: %v", err)
	}
	if resolvedOutage.Status != model.IncidentResolved {
		t.Errorf("expected outage status %s, got %s", model.IncidentResolved, resolvedOutage.Status)
	}
	if resolvedOutage.ResolvedAt == nil {
		t.Fatal("expected resolved_at to be populated for resolved outage, got nil")
	}

	hasResolvedEvent := false
	for _, evt := range resolvedOutage.Events {
		if evt.EventType == model.IncidentEventResolved {
			hasResolvedEvent = true
			break
		}
	}
	if !hasResolvedEvent {
		t.Errorf("expected resolution event on outage incident")
	}
}
