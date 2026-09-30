package quota_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/apipulse/apipulse/internal/probes/quota"
)

func TestTokenBucket_Basic(t *testing.T) {
	// Capacity 2, refill rate 1 per second
	tb := quota.NewTokenBucket(2.0, 1.0)

	if !tb.Allow() {
		t.Fatal("expected first allow to succeed")
	}
	if !tb.Allow() {
		t.Fatal("expected second allow to succeed")
	}
	if tb.Allow() {
		t.Fatal("expected third allow to fail (exhausted)")
	}

	// Wait 1.1s for refill
	time.Sleep(1100 * time.Millisecond)

	if !tb.Allow() {
		t.Fatal("expected allow after refill to succeed")
	}
}

func TestManager_DependencyLimit(t *testing.T) {
	depID := uuid.New()
	region := "us-east-1"

	mgr := quota.NewManager(quota.Config{
		DependencyMaxTokens:  1.0,
		DependencyRatePerSec: 0.1,
		RegionMaxTokens:      10.0,
		RegionRatePerSec:     10.0,
	})

	if err := mgr.Allow(depID, region); err != nil {
		t.Fatalf("first allow failed: %v", err)
	}

	err := mgr.Allow(depID, region)
	if err == nil {
		t.Fatal("expected error on second immediate call")
	}
	if !errors.Is(err, quota.ErrQuotaExceeded) {
		t.Errorf("expected ErrQuotaExceeded, got: %v", err)
	}
}

func TestManager_RegionLimit(t *testing.T) {
	dep1 := uuid.New()
	dep2 := uuid.New()
	region := "eu-west-1"

	mgr := quota.NewManager(quota.Config{
		DependencyMaxTokens:  10.0,
		DependencyRatePerSec: 10.0,
		RegionMaxTokens:      1.0,
		RegionRatePerSec:     0.1,
	})

	if err := mgr.Allow(dep1, region); err != nil {
		t.Fatalf("dep1 allow failed: %v", err)
	}

	err := mgr.Allow(dep2, region)
	if err == nil {
		t.Fatal("expected region limit error on dep2")
	}
	if !errors.Is(err, quota.ErrQuotaExceeded) {
		t.Errorf("expected ErrQuotaExceeded, got: %v", err)
	}
}
