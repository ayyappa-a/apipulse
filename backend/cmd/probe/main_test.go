package main

import (
	"context"
	"testing"

	"github.com/apipulse/apipulse/internal/config"
	"github.com/apipulse/apipulse/internal/probes/runner"
	"github.com/apipulse/apipulse/internal/storage/postgres"
)

func TestDiscoverTargets_HealthTargetsAndStore(t *testing.T) {
	cfg := &config.Config{
		HealthTargets: []string{"stripe-like:stripe:ap-south-1", "stripe-like:stripe:us-east-1"},
	}
	store := postgres.NewInMemoryStore()
	vendorMockURL := "http://vendor-mock:9001"
	defaultRegion := "ap-south-1"

	targets := discoverTargets(context.Background(), cfg, store, vendorMockURL, defaultRegion)
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	foundAPSouth := false
	for _, target := range targets {
		if target.DependencyName != "stripe-like" {
			t.Errorf("expected DependencyName 'stripe-like', got %s", target.DependencyName)
		}
		if target.Endpoint != "http://vendor-mock:9001/v1/charges" {
			t.Errorf("expected Endpoint 'http://vendor-mock:9001/v1/charges', got %s", target.Endpoint)
		}
		if target.Method != "POST" || !target.AllowPOST {
			t.Errorf("expected Method POST with AllowPOST true, got %s, %t", target.Method, target.AllowPOST)
		}

		pCfg := runner.ProbeConfig{
			ID:           target.DependencyID,
			DependencyID: target.DependencyID,
			Name:         target.DependencyName,
			Endpoint:     target.Endpoint,
			Method:       target.Method,
			Region:       target.Region,
			AllowPOST:    target.AllowPOST,
		}
		if err := pCfg.Validate(); err != nil {
			t.Errorf("probe config validation failed: %v", err)
		}

		if target.Region == "ap-south-1" {
			foundAPSouth = true
		}
	}

	if !foundAPSouth {
		t.Errorf("expected to find target in region ap-south-1")
	}
}

func TestDiscoverTargets_Fallback(t *testing.T) {
	cfg := &config.Config{
		HealthTargets: []string{},
	}
	store := postgres.NewInMemoryStore()
	targets := discoverTargets(context.Background(), cfg, store, "http://vendor-mock:9001", "ap-south-1")
	if len(targets) != 1 {
		t.Fatalf("expected 1 fallback target, got %d", len(targets))
	}
	if targets[0].Region != "ap-south-1" {
		t.Errorf("expected fallback region ap-south-1, got %s", targets[0].Region)
	}
}
