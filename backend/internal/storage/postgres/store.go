// Package postgres defines the PostgreSQL/TimescaleDB storage interfaces for APIPulse.
//
// Phase 3C extends the TelemetryStore with full read/write capabilities for:
//   - Dependency observations (CUSTOMER_OTEL)
//   - Probe observations (SYNTHETIC_PROBE)
//   - Vendor status records (VENDOR_STATUS)
//   - Health assessments (APIPULSE_ASSESSMENT)
//   - Incidents and incident events
//
// Concrete implementations:
//   - NoopTelemetryStore: discards all writes, returns empty reads (dev/CI fallback)
//   - InMemoryTelemetryStore: thread-safe in-memory store (tests / demo)
//   - PostgresStore: real pgx-backed implementation (Phase 3C+ production)
package postgres

import (
	"context"
	"time"

	"github.com/apipulse/apipulse/internal/telemetry/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// TelemetryStore — extended interface for Phase 3C
// ─────────────────────────────────────────────────────────────────────────────

// TelemetryStore defines the full persistence contract for APIPulse telemetry.
// Any concrete implementation (postgres, in-memory, noop) must satisfy this interface.
type TelemetryStore interface {
	// ── Write operations ──────────────────────────────────────────────────────

	// InsertDependencyObservation persists a normalized CUSTOMER_OTEL observation.
	InsertDependencyObservation(ctx context.Context, obs model.DependencyObservation) error

	// InsertProbeObservation persists a normalized SYNTHETIC_PROBE result.
	InsertProbeObservation(ctx context.Context, obs model.ProbeObservation) error

	// InsertVendorStatus persists a vendor status record.
	InsertVendorStatus(ctx context.Context, rec model.VendorStatusRecord) error

	// InsertAssessment persists a computed health assessment.
	InsertAssessment(ctx context.Context, a model.HealthAssessment) error

	// UpsertIncident creates a new incident or updates an existing one
	// (deduplication by DeduplicationKey).
	UpsertIncident(ctx context.Context, inc model.Incident) error

	// InsertIncidentEvent appends a timeline event to an incident.
	InsertIncidentEvent(ctx context.Context, evt model.IncidentEvent) error

	// ── Read operations ───────────────────────────────────────────────────────

	// GetRecentDependencyObservations returns CUSTOMER_OTEL observations for a
	// dependency+region pair within the given time window.
	GetRecentDependencyObservations(
		ctx context.Context,
		dependencyName, region string,
		since time.Time,
	) ([]model.DependencyObservation, error)

	// GetRecentProbeObservations returns SYNTHETIC_PROBE observations for a
	// dependency+region pair within the given time window.
	GetRecentProbeObservations(
		ctx context.Context,
		dependencyName, region string,
		since time.Time,
	) ([]model.ProbeObservation, error)

	// GetLatestVendorStatus returns the most recent vendor status record.
	GetLatestVendorStatus(
		ctx context.Context,
		vendorName, dependencyName string,
	) (*model.VendorStatusRecord, error)

	// GetLatestAssessment returns the most recent assessment for a dependency+region.
	GetLatestAssessment(
		ctx context.Context,
		dependencyName, region string,
	) (*model.HealthAssessment, error)

	// ListAssessments returns assessments for a dependency, newest first.
	ListAssessments(
		ctx context.Context,
		dependencyName string,
		limit int,
	) ([]model.HealthAssessment, error)

	// GetActiveIncident returns the currently active (non-resolved) incident
	// for a deduplication key, or nil if none exists.
	GetActiveIncident(
		ctx context.Context,
		deduplicationKey string,
	) (*model.Incident, error)

	// ListIncidents returns incidents, newest first.
	ListIncidents(
		ctx context.Context,
		limit int,
	) ([]model.Incident, error)

	// GetIncident returns a specific incident by ID, including events.
	GetIncident(ctx context.Context, id string) (*model.Incident, error)

	// GetActiveTargets returns distinct (dependency_name, region) tuples discovered
	// from ingested observations or assessments.
	GetActiveTargets(ctx context.Context) ([]Target, error)
}

// Target represents a discovered dependency and region tuple.
type Target struct {
	DependencyName string `json:"dependency_name"`
	Region         string `json:"region"`
}

// ─────────────────────────────────────────────────────────────────────────────
// NoopTelemetryStore — dev/test fallback
// ─────────────────────────────────────────────────────────────────────────────

// NoopTelemetryStore is a no-op implementation of TelemetryStore.
// All writes are accepted and discarded. All reads return empty results.
// Enables the API server to run without a database in development.
type NoopTelemetryStore struct{}

func (n *NoopTelemetryStore) InsertDependencyObservation(_ context.Context, _ model.DependencyObservation) error {
	return nil
}
func (n *NoopTelemetryStore) InsertProbeObservation(_ context.Context, _ model.ProbeObservation) error {
	return nil
}
func (n *NoopTelemetryStore) InsertVendorStatus(_ context.Context, _ model.VendorStatusRecord) error {
	return nil
}
func (n *NoopTelemetryStore) InsertAssessment(_ context.Context, _ model.HealthAssessment) error {
	return nil
}
func (n *NoopTelemetryStore) UpsertIncident(_ context.Context, _ model.Incident) error {
	return nil
}
func (n *NoopTelemetryStore) InsertIncidentEvent(_ context.Context, _ model.IncidentEvent) error {
	return nil
}
func (n *NoopTelemetryStore) GetRecentDependencyObservations(_ context.Context, _, _ string, _ time.Time) ([]model.DependencyObservation, error) {
	return nil, nil
}
func (n *NoopTelemetryStore) GetRecentProbeObservations(_ context.Context, _, _ string, _ time.Time) ([]model.ProbeObservation, error) {
	return nil, nil
}
func (n *NoopTelemetryStore) GetLatestVendorStatus(_ context.Context, _, _ string) (*model.VendorStatusRecord, error) {
	return nil, nil
}
func (n *NoopTelemetryStore) GetLatestAssessment(_ context.Context, _, _ string) (*model.HealthAssessment, error) {
	return nil, nil
}
func (n *NoopTelemetryStore) ListAssessments(_ context.Context, _ string, _ int) ([]model.HealthAssessment, error) {
	return nil, nil
}
func (n *NoopTelemetryStore) GetActiveIncident(_ context.Context, _ string) (*model.Incident, error) {
	return nil, nil
}
func (n *NoopTelemetryStore) ListIncidents(_ context.Context, _ int) ([]model.Incident, error) {
	return nil, nil
}
func (n *NoopTelemetryStore) GetIncident(_ context.Context, _ string) (*model.Incident, error) {
	return nil, nil
}
func (n *NoopTelemetryStore) GetActiveTargets(_ context.Context) ([]Target, error) {
	return nil, nil
}
