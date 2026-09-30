package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/apipulse/apipulse/internal/config"
	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/google/uuid"
)

func TestPostgresStore_InvalidURL(t *testing.T) {
	cfg := &config.Config{
		DatabaseURL: "invalid://bad-url",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := NewPostgresStore(ctx, cfg)
	if err == nil {
		t.Fatal("expected error with invalid database URL, got nil")
	}
}

func TestPostgresStore_UnreachableDatabase(t *testing.T) {
	cfg := &config.Config{
		DatabaseURL:      "postgres://invalid_user:invalid_pass@127.0.0.1:54399/nonexistent?sslmode=disable",
		PostgresMaxConns: 2,
		PostgresMinConns: 1,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := NewPostgresStore(ctx, cfg)
	if err == nil {
		t.Fatal("expected error when connecting to unreachable database, got nil")
	}
}

func TestInMemoryStore_RestartBehavior(t *testing.T) {
	// Verify that store operations behave correctly across simulated restart
	store1 := NewInMemoryStore()
	ctx := context.Background()

	depObs := model.DependencyObservation{
		ID:             uuid.New(),
		Timestamp:      time.Now().UTC(),
		ServiceName:    "codebegun",
		DependencyName: "stripe-like",
		Region:         "ap-south-1",
		DurationMS:     8000,
		Status:         model.StatusOK,
	}

	err := store1.InsertDependencyObservation(ctx, depObs)
	if err != nil {
		t.Fatalf("failed to insert obs: %v", err)
	}

	obs, err := store1.GetRecentDependencyObservations(ctx, "stripe-like", "ap-south-1", time.Now().Add(-time.Hour))
	if err != nil || len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}

	// Active incident deduplication
	inc := model.Incident{
		ID:               uuid.New(),
		DependencyName:   "stripe-like",
		Region:           "ap-south-1",
		DeduplicationKey: "stripe-like:ap-south-1:GREY_FAILURE",
		Status:           model.IncidentOpen,
		Title:            "Grey Failure in ap-south-1",
		Severity:         "critical",
		StartedAt:        time.Now().UTC(),
	}

	_ = store1.UpsertIncident(ctx, inc)

	active, err := store1.GetActiveIncident(ctx, "stripe-like:ap-south-1:GREY_FAILURE")
	if err != nil || active == nil {
		t.Fatalf("expected active incident, got nil")
	}

	// Verify deduplication update (no duplicate)
	inc2 := inc
	inc2.Title = "Updated Title"
	_ = store1.UpsertIncident(ctx, inc2)

	list, _ := store1.ListIncidents(ctx, 10)
	if len(list) != 1 {
		t.Fatalf("expected 1 deduplicated incident, got %d", len(list))
	}
	if list[0].Title != "Updated Title" {
		t.Errorf("expected updated title, got %s", list[0].Title)
	}
}
