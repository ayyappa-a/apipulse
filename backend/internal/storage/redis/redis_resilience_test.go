package redis_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/apipulse/apipulse/internal/api/handlers"
	"github.com/apipulse/apipulse/internal/health"
	"github.com/apipulse/apipulse/internal/storage/postgres"
	rediscache "github.com/apipulse/apipulse/internal/storage/redis"
	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/apipulse/apipulse/internal/vendors"
	"github.com/apipulse/apipulse/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// newDeadRedisCache creates a RedisCache with an unreachable client
// to simulate a hard network / container outage without waiting on ping during creation.
func newDeadRedisCache() rediscache.StateCache {
	opts := &redis.Options{
		Addr:         "127.0.0.1:59998",
		DialTimeout:  100 * time.Millisecond,
		ReadTimeout:  100 * time.Millisecond,
		WriteTimeout: 100 * time.Millisecond,
		MaxRetries:   0,
	}
	client := redis.NewClient(opts)
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)
	return rediscache.NewTestRedisCache(client, m)
}

// Test 1 — Redis unavailable does not poison evaluation context
func TestRedisResilience_Test1_RedisUnavailableDoesNotPoisonEvaluationContext(t *testing.T) {
	deadCache := newDeadRedisCache()
	store := postgres.NewInMemoryStore()
	vendorAdapter := &vendors.StaticStatusAdapter{Status: model.VendorStatusOperational}
	engine := health.NewEngine(store, deadCache, vendorAdapter, nil, health.DefaultEngineConfig())

	target := health.EvaluationTarget{
		DependencyName: "stripe-like",
		Region:         "ap-south-1",
		VendorName:     "stripe-like",
	}

	// 15-second parent evaluation context (same as scheduler)
	parentCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	start := time.Now()
	assessment, err := engine.Evaluate(parentCtx, target)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("engine.Evaluate failed: %v", err)
	}
	if assessment == nil {
		t.Fatal("expected non-nil assessment")
	}

	// Critical check 1: Parent context must NOT be expired or canceled!
	if parentCtx.Err() != nil {
		t.Fatalf("critical bug: parent evaluation context was poisoned/canceled by Redis: %v", parentCtx.Err())
	}

	// Critical check 2: Evaluation must finish quickly, well under 15 seconds
	if elapsed > 3*time.Second {
		t.Errorf("evaluation took too long (%v), expected < 3s", elapsed)
	}

	// Critical check 3: Assessment MUST be persisted into PostgreSQL store
	persisted, err := store.GetLatestAssessment(context.Background(), target.DependencyName, target.Region)
	if err != nil {
		t.Fatalf("store.GetLatestAssessment failed: %v", err)
	}
	if persisted == nil {
		t.Fatal("critical bug: assessment was NOT persisted into PostgreSQL store while Redis was unavailable")
	}
	if persisted.ID != assessment.ID {
		t.Errorf("persisted assessment ID mismatch: expected %s, got %s", assessment.ID, persisted.ID)
	}
}

// Test 2 — Redis timeout is bounded
func TestRedisResilience_Test2_RedisTimeoutIsBounded(t *testing.T) {
	deadCache := newDeadRedisCache()

	// 15-second parent context
	parentCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	start := time.Now()
	status, err := deadCache.GetDependencyHealth(parentCtx, "stripe-like", "ap-south-1")
	elapsed := time.Since(start)

	// Must return within ~350ms (DefaultRedisOpTimeout = 300ms + small margin)
	if elapsed > 600*time.Millisecond {
		t.Errorf("cache call took %v, expected < 600ms", elapsed)
	}

	// Parent context must remain valid
	if parentCtx.Err() != nil {
		t.Errorf("parent context was expired by cache operation: %v", parentCtx.Err())
	}

	t.Logf("Bounded cache call completed in %v with status=%q err=%v", elapsed, status, err)
}

// Test 3 — Readiness with Redis unavailable
func TestRedisResilience_Test3_ReadinessWithRedisUnavailable(t *testing.T) {
	deadCache := newDeadRedisCache()

	// Mock readiness check matching cmd/api/main.go
	readinessFunc := func(ctx context.Context) (bool, map[string]string) {
		details := make(map[string]string)
		isReady := true

		// Postgres is healthy
		details["postgres"] = "ok"

		// Redis is checked with bounded timeout
		if deadCache.IsActive(ctx) {
			details["redis"] = "ok"
		} else {
			details["redis"] = "degraded"
		}

		details["health_engine"] = "ok"
		return isReady, details
	}

	handler := handlers.NewReadyHandler(readinessFunc)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	handler.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	if rec.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", rec.Code)
	}

	// Must complete quickly
	if elapsed > 1*time.Second {
		t.Errorf("readiness check took %v, expected < 1s", elapsed)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode readiness body: %v", err)
	}

	if body["status"] != "ready" {
		t.Errorf("expected status='ready', got %v", body["status"])
	}
	if body["redis"] != "degraded" {
		t.Errorf("expected redis='degraded', got %v", body["redis"])
	}
	if body["postgres"] != "ok" {
		t.Errorf("expected postgres='ok', got %v", body["postgres"])
	}
}

// Test 4 — PostgreSQL strict readiness remains unchanged
func TestRedisResilience_Test4_PostgresStrictReadiness(t *testing.T) {
	// When PostgreSQL fails, readiness MUST return HTTP 503
	readinessFunc := func(ctx context.Context) (bool, map[string]string) {
		details := make(map[string]string)
		details["postgres"] = "error: connection refused"
		details["redis"] = "ok"
		details["health_engine"] = "ok"
		return false, details
	}

	handler := handlers.NewReadyHandler(readinessFunc)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected HTTP 503 when Postgres is unavailable, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode readiness body: %v", err)
	}

	if body["status"] != "unready" {
		t.Errorf("expected status='unready', got %v", body["status"])
	}
}

// Test 5 — Healthy Redis behavior unchanged
func TestRedisResilience_Test5_HealthyRedisBehaviorUnchanged(t *testing.T) {
	// Use in-memory cache to verify hot state operations work seamlessly
	cache := rediscache.NewInMemoryCache()
	ctx := context.Background()

	// 1. IsActive
	if !cache.IsActive(ctx) {
		t.Error("expected IsActive=true for healthy cache")
	}

	// 2. Health state
	_ = cache.SetDependencyHealth(ctx, "stripe-like", "ap-south-1", model.AssessmentHealthy, time.Minute)
	status, err := cache.GetDependencyHealth(ctx, "stripe-like", "ap-south-1")
	if err != nil || status != model.AssessmentHealthy {
		t.Errorf("expected status=HEALTHY, got %v (err: %v)", status, err)
	}

	// 3. Recovery counter
	cnt, _ := cache.IncrHealthyWindowCount(ctx, "stripe-like", "ap-south-1", time.Minute)
	if cnt != 1 {
		t.Errorf("expected count=1, got %d", cnt)
	}

	// 4. Assessment caching
	assessment := &model.HealthAssessment{
		ID:             [16]byte{1, 2, 3},
		DependencyName: "stripe-like",
		Region:         "ap-south-1",
		Status:         model.AssessmentHealthy,
		Timestamp:      time.Now().UTC(),
	}
	_ = cache.SetLatestAssessment(ctx, "stripe-like", "ap-south-1", assessment, time.Minute)
	cached, _ := cache.GetLatestAssessment(ctx, "stripe-like", "ap-south-1")
	if cached == nil || cached.Status != model.AssessmentHealthy {
		t.Errorf("expected cached assessment status=HEALTHY, got %v", cached)
	}
}
