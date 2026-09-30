package redis

import (
	"context"
	"testing"
	"time"

	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/apipulse/apipulse/pkg/metrics"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
)

func TestRedisCache_GracefulFallback_WhenUnavailable(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	// Direct instantiation with nil client or unreachable client to verify fallback
	cache := &RedisCache{
		client:              nil,
		metrics:             m,
		fallbackHealth:      make(map[string]model.AssessmentStatus),
		fallbackAssessment:  make(map[string]*model.HealthAssessment),
		fallbackCounters:    make(map[string]int64),
		fallbackDepStatuses: make(map[string]string),
	}

	ctx := context.Background()

	// IsActive must return false when Redis client is nil or down
	if cache.IsActive(ctx) {
		t.Error("expected IsActive=false when client is nil")
	}

	// Set and Get dependency status via fallback
	err := cache.SetDependencyStatus(ctx, "stripe-dep", "healthy")
	if err != nil {
		t.Logf("SetDependencyStatus error (expected when client nil, but fallback stored): %v", err)
	}

	status, _ := cache.GetDependencyStatus(ctx, "stripe-dep")
	if status != "healthy" {
		t.Errorf("expected fallback status='healthy', got %q", status)
	}

	// Hysteresis counter fallback
	cnt, _ := cache.IncrHealthyWindowCount(ctx, "stripe-like", "ap-south-1", time.Minute)
	if cnt != 1 {
		t.Errorf("expected count=1, got %d", cnt)
	}

	cnt2, _ := cache.GetHealthyWindowCount(ctx, "stripe-like", "ap-south-1")
	if cnt2 != 1 {
		t.Errorf("expected count=1, got %d", cnt2)
	}

	_ = cache.ResetHealthyWindowCount(ctx, "stripe-like", "ap-south-1")
	cnt3, _ := cache.GetHealthyWindowCount(ctx, "stripe-like", "ap-south-1")
	if cnt3 != 0 {
		t.Errorf("expected count=0 after reset, got %d", cnt3)
	}

	// Assessment fallback
	assessment := &model.HealthAssessment{
		ID:             uuid.New(),
		DependencyName: "stripe-like",
		Region:         "ap-south-1",
		Status:         model.AssessmentGreyFailure,
		Timestamp:      time.Now().UTC(),
	}

	_ = cache.SetLatestAssessment(ctx, "stripe-like", "ap-south-1", assessment, time.Minute)
	cachedAss, _ := cache.GetLatestAssessment(ctx, "stripe-like", "ap-south-1")
	if cachedAss == nil || cachedAss.Status != model.AssessmentGreyFailure {
		t.Errorf("expected cached assessment status=GREY_FAILURE, got %v", cachedAss)
	}
}
