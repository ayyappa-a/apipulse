package health

import (
	"context"
	"testing"
	"time"

	"github.com/apipulse/apipulse/internal/storage/postgres"
	rediscache "github.com/apipulse/apipulse/internal/storage/redis"
	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/apipulse/apipulse/internal/vendors"
	"github.com/apipulse/apipulse/pkg/logger"
	"github.com/apipulse/apipulse/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func TestScheduler_Lifecycle(t *testing.T) {
	store := postgres.NewInMemoryStore()
	cache := &rediscache.NoopStateCache{}
	vProvider := &vendors.StaticStatusAdapter{Status: model.VendorStatusOperational}
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)
	log := logger.New("test-scheduler", "error")

	engine := NewEngine(store, cache, vProvider, m, DefaultEngineConfig())

	targets := []EvaluationTarget{
		{
			DependencyName: "test-dep",
			DependencyID:   "test-id",
			VendorName:     "test-vendor",
			Region:         "us-east-1",
		},
	}

	scheduler := NewScheduler(engine, targets, 50*time.Millisecond, m, log)

	if scheduler.IsRunning() {
		t.Fatal("expected scheduler to not be running before Start")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler.Start(ctx)
	if !scheduler.IsRunning() {
		t.Fatal("expected scheduler to be running after Start")
	}

	// Allow several evaluation ticks
	time.Sleep(150 * time.Millisecond)

	scheduler.Stop()
	if scheduler.IsRunning() {
		t.Fatal("expected scheduler to be stopped after Stop")
	}
}

func TestScheduler_NoOverlappingRuns(t *testing.T) {
	store := postgres.NewInMemoryStore()
	cache := &rediscache.NoopStateCache{}
	vProvider := &vendors.StaticStatusAdapter{Status: model.VendorStatusOperational}
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)
	log := logger.New("test-scheduler", "error")

	engine := NewEngine(store, cache, vProvider, m, DefaultEngineConfig())

	targets := []EvaluationTarget{
		{
			DependencyName: "slow-dep",
			DependencyID:   "slow-id",
			VendorName:     "slow-vendor",
			Region:         "ap-south-1",
		},
	}

	scheduler := NewScheduler(engine, targets, 10*time.Millisecond, m, log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	scheduler.Stop()
}
