// Package redis defines the Redis interface for APIPulse hot operational state.
//
// IMPORTANT — Redis role in APIPulse:
//
//   Redis is NOT the primary telemetry database.
//   Redis is NOT used to store historical observations.
//
// Redis is reserved exclusively for:
//   • Hot health state cache (current assessment per dependency + region)
//   • Probe job coordination (distributed locking for probe scheduling)
//   • Short-lived aggregation cache (rolling counters, deduplication)
//   • Hysteresis counter state (recovery window tracking)
//
// PostgreSQL/TimescaleDB is the historical source of record.
// Redis is a fast, ephemeral operational layer only.
//
// The system must clearly report whether Redis persistence is active.
// When Redis is unavailable, the Noop fallback ensures the system remains functional.
package redis

import (
	"context"
	"time"

	"github.com/apipulse/apipulse/internal/telemetry/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// StateCache interface
// ─────────────────────────────────────────────────────────────────────────────

// StateCache defines the contract for hot state reads and writes.
type StateCache interface {
	// ── Dependency health state ───────────────────────────────────────────────

	// SetDependencyHealth stores the latest assessment status for a dependency+region.
	SetDependencyHealth(ctx context.Context, dependencyName, region string, status model.AssessmentStatus, ttl time.Duration) error

	// GetDependencyHealth returns the cached assessment status for a dependency+region.
	// Returns ("", nil) if no status is cached.
	GetDependencyHealth(ctx context.Context, dependencyName, region string) (model.AssessmentStatus, error)

	// SetLatestAssessment stores the full latest assessment as JSON.
	SetLatestAssessment(ctx context.Context, dependencyName, region string, assessment *model.HealthAssessment, ttl time.Duration) error

	// GetLatestAssessment returns the cached assessment for a dependency+region, or nil.
	GetLatestAssessment(ctx context.Context, dependencyName, region string) (*model.HealthAssessment, error)

	// ── Hysteresis / recovery counters ───────────────────────────────────────

	// IncrHealthyWindowCount increments the consecutive healthy evaluation window
	// counter for a dependency+region. Returns the new count.
	IncrHealthyWindowCount(ctx context.Context, dependencyName, region string, ttl time.Duration) (int64, error)

	// ResetHealthyWindowCount resets the consecutive healthy window counter to 0.
	ResetHealthyWindowCount(ctx context.Context, dependencyName, region string) error

	// GetHealthyWindowCount returns the current consecutive healthy window count.
	GetHealthyWindowCount(ctx context.Context, dependencyName, region string) (int64, error)

	// ── Deduplication ─────────────────────────────────────────────────────────

	// SetDependencyStatus is kept for backward compatibility with Phase 2.
	SetDependencyStatus(ctx context.Context, dependencyID, status string) error
	GetDependencyStatus(ctx context.Context, dependencyID string) (string, error)

	// ── Diagnostics ───────────────────────────────────────────────────────────

	// IsActive returns true if the Redis connection is live.
	IsActive(ctx context.Context) bool
}

// ─────────────────────────────────────────────────────────────────────────────
// NoopStateCache — dev fallback (no Redis required)
// ─────────────────────────────────────────────────────────────────────────────

// NoopStateCache is a no-op implementation of StateCache.
// All writes are discarded. All reads return zero values.
// Enables the service to run without Redis in development.
//
// The system logs a warning when using the Noop cache so operators
// are aware that Redis state is not persisted.
type NoopStateCache struct{}

func (n *NoopStateCache) SetDependencyHealth(_ context.Context, _, _ string, _ model.AssessmentStatus, _ time.Duration) error {
	return nil
}
func (n *NoopStateCache) GetDependencyHealth(_ context.Context, _, _ string) (model.AssessmentStatus, error) {
	return "", nil
}
func (n *NoopStateCache) SetLatestAssessment(_ context.Context, _, _ string, _ *model.HealthAssessment, _ time.Duration) error {
	return nil
}
func (n *NoopStateCache) GetLatestAssessment(_ context.Context, _, _ string) (*model.HealthAssessment, error) {
	return nil, nil
}
func (n *NoopStateCache) IncrHealthyWindowCount(_ context.Context, _, _ string, _ time.Duration) (int64, error) {
	return 0, nil
}
func (n *NoopStateCache) ResetHealthyWindowCount(_ context.Context, _, _ string) error {
	return nil
}
func (n *NoopStateCache) GetHealthyWindowCount(_ context.Context, _, _ string) (int64, error) {
	return 0, nil
}
func (n *NoopStateCache) SetDependencyStatus(_ context.Context, _, _ string) error { return nil }
func (n *NoopStateCache) GetDependencyStatus(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (n *NoopStateCache) IsActive(_ context.Context) bool { return false }

// ─────────────────────────────────────────────────────────────────────────────
// InMemoryStateCache — deterministic test/demo cache
// ─────────────────────────────────────────────────────────────────────────────

// InMemoryStateCache is a thread-safe in-memory implementation of StateCache.
// Used in tests and the Phase 3C integration demo where Redis is unavailable.
type InMemoryStateCache struct {
	health        map[string]model.AssessmentStatus
	assessments   map[string]*model.HealthAssessment
	healthWindows map[string]int64
	statuses      map[string]string
}

// NewInMemoryCache returns a ready-to-use InMemoryStateCache.
func NewInMemoryCache() *InMemoryStateCache {
	return &InMemoryStateCache{
		health:        make(map[string]model.AssessmentStatus),
		assessments:   make(map[string]*model.HealthAssessment),
		healthWindows: make(map[string]int64),
		statuses:      make(map[string]string),
	}
}

func (c *InMemoryStateCache) key(dep, region string) string {
	return dep + "::" + region
}

func (c *InMemoryStateCache) SetDependencyHealth(_ context.Context, dep, region string, status model.AssessmentStatus, _ time.Duration) error {
	c.health[c.key(dep, region)] = status
	return nil
}
func (c *InMemoryStateCache) GetDependencyHealth(_ context.Context, dep, region string) (model.AssessmentStatus, error) {
	return c.health[c.key(dep, region)], nil
}
func (c *InMemoryStateCache) SetLatestAssessment(_ context.Context, dep, region string, a *model.HealthAssessment, _ time.Duration) error {
	c.assessments[c.key(dep, region)] = a
	return nil
}
func (c *InMemoryStateCache) GetLatestAssessment(_ context.Context, dep, region string) (*model.HealthAssessment, error) {
	return c.assessments[c.key(dep, region)], nil
}
func (c *InMemoryStateCache) IncrHealthyWindowCount(_ context.Context, dep, region string, _ time.Duration) (int64, error) {
	c.healthWindows[c.key(dep, region)]++
	return c.healthWindows[c.key(dep, region)], nil
}
func (c *InMemoryStateCache) ResetHealthyWindowCount(_ context.Context, dep, region string) error {
	c.healthWindows[c.key(dep, region)] = 0
	return nil
}
func (c *InMemoryStateCache) GetHealthyWindowCount(_ context.Context, dep, region string) (int64, error) {
	return c.healthWindows[c.key(dep, region)], nil
}
func (c *InMemoryStateCache) SetDependencyStatus(_ context.Context, id, status string) error {
	c.statuses[id] = status
	return nil
}
func (c *InMemoryStateCache) GetDependencyStatus(_ context.Context, id string) (string, error) {
	return c.statuses[id], nil
}
func (c *InMemoryStateCache) IsActive(_ context.Context) bool { return true }
