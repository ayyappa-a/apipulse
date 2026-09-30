package postgres

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apipulse/apipulse/internal/telemetry/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// InMemoryTelemetryStore — thread-safe, deterministic store for tests/demo
// ─────────────────────────────────────────────────────────────────────────────

type inMemoryStore struct {
	mu          sync.RWMutex
	depObs      []model.DependencyObservation
	probeObs    []model.ProbeObservation
	vendorStats []model.VendorStatusRecord
	assessments []model.HealthAssessment
	incidents   []model.Incident
	events      []model.IncidentEvent
}

// NewInMemoryStore returns a thread-safe in-memory TelemetryStore.
// Suitable for Phase 3C integration tests and demo scenarios.
// All state is lost on process restart — not for production.
func NewInMemoryStore() TelemetryStore {
	return &inMemoryStore{}
}

// ── Writes ────────────────────────────────────────────────────────────────────

func (s *inMemoryStore) InsertDependencyObservation(_ context.Context, obs model.DependencyObservation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.depObs = append(s.depObs, obs)
	return nil
}

func (s *inMemoryStore) InsertProbeObservation(_ context.Context, obs model.ProbeObservation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.probeObs = append(s.probeObs, obs)
	return nil
}

func (s *inMemoryStore) InsertVendorStatus(_ context.Context, rec model.VendorStatusRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vendorStats = append(s.vendorStats, rec)
	return nil
}

func (s *inMemoryStore) InsertAssessment(_ context.Context, a model.HealthAssessment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.assessments = append(s.assessments, a)
	return nil
}

func (s *inMemoryStore) UpsertIncident(_ context.Context, inc model.Incident) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.incidents {
		if existing.DeduplicationKey == inc.DeduplicationKey {
			inc.CreatedAt = existing.CreatedAt // preserve creation time
			s.incidents[i] = inc
			return nil
		}
	}
	s.incidents = append(s.incidents, inc)
	return nil
}

func (s *inMemoryStore) InsertIncidentEvent(_ context.Context, evt model.IncidentEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, evt)
	return nil
}

// ── Reads ─────────────────────────────────────────────────────────────────────

func (s *inMemoryStore) GetRecentDependencyObservations(
	_ context.Context,
	dependencyName, region string,
	since time.Time,
) ([]model.DependencyObservation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []model.DependencyObservation
	for _, obs := range s.depObs {
		if strings.EqualFold(obs.DependencyName, dependencyName) &&
			strings.EqualFold(obs.Region, region) &&
			obs.Timestamp.After(since) {
			result = append(result, obs)
		}
	}
	return result, nil
}

func (s *inMemoryStore) GetRecentProbeObservations(
	_ context.Context,
	dependencyName, region string,
	since time.Time,
) ([]model.ProbeObservation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []model.ProbeObservation
	for _, obs := range s.probeObs {
		// ProbeObservation uses DependencyID (UUID) — for the in-memory demo
		// we match on NetworkError field or use a convention from the Region.
		// Since the demo injects dependency name via an extra field, match on region only
		// if dependencyName is empty (for broader queries).
		if strings.EqualFold(obs.Region, region) && obs.Timestamp.After(since) {
			result = append(result, obs)
		}
	}
	return result, nil
}

func (s *inMemoryStore) GetLatestVendorStatus(
	_ context.Context,
	_, dependencyName string,
) (*model.VendorStatusRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest *model.VendorStatusRecord
	for i := range s.vendorStats {
		v := &s.vendorStats[i]
		if strings.EqualFold(v.DependencyName, dependencyName) {
			if latest == nil || v.Timestamp.After(latest.Timestamp) {
				latest = v
			}
		}
	}
	if latest == nil {
		return nil, nil
	}
	cp := *latest
	return &cp, nil
}

func (s *inMemoryStore) GetLatestAssessment(
	_ context.Context,
	dependencyName, region string,
) (*model.HealthAssessment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest *model.HealthAssessment
	for i := range s.assessments {
		a := &s.assessments[i]
		if strings.EqualFold(a.DependencyName, dependencyName) &&
			strings.EqualFold(a.Region, region) {
			if latest == nil || a.Timestamp.After(latest.Timestamp) {
				latest = a
			}
		}
	}
	if latest == nil {
		return nil, nil
	}
	cp := *latest
	return &cp, nil
}

func (s *inMemoryStore) ListAssessments(
	_ context.Context,
	dependencyName string,
	limit int,
) ([]model.HealthAssessment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []model.HealthAssessment
	for _, a := range s.assessments {
		if dependencyName == "" || strings.EqualFold(a.DependencyName, dependencyName) {
			result = append(result, a)
		}
	}
	// Sort newest first.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *inMemoryStore) GetActiveIncident(
	_ context.Context,
	deduplicationKey string,
) (*model.Incident, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.incidents {
		inc := &s.incidents[i]
		if inc.DeduplicationKey == deduplicationKey &&
			inc.Status != model.IncidentResolved {
			cp := *inc
			return &cp, nil
		}
	}
	return nil, nil
}

func (s *inMemoryStore) ListIncidents(
	_ context.Context,
	limit int,
) ([]model.Incident, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.Incident, len(s.incidents))
	copy(result, s.incidents)
	// Sort newest first.
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *inMemoryStore) GetIncident(
	_ context.Context,
	id string,
) (*model.Incident, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.incidents {
		if s.incidents[i].ID.String() == id {
			cp := s.incidents[i]
			// Attach events.
			for _, evt := range s.events {
				if evt.IncidentID == cp.ID {
					cp.Events = append(cp.Events, evt)
				}
			}
			sort.Slice(cp.Events, func(a, b int) bool {
				return cp.Events[a].CreatedAt.Before(cp.Events[b].CreatedAt)
			})
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("incident %s not found", id)
}

func (s *inMemoryStore) GetActiveTargets(_ context.Context) ([]Target, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := make(map[string]bool)
	var targets []Target

	add := func(dep, reg string) {
		if dep == "" || reg == "" {
			return
		}
		key := dep + ":" + reg
		if !seen[key] {
			seen[key] = true
			targets = append(targets, Target{DependencyName: dep, Region: reg})
		}
	}

	for _, o := range s.depObs {
		add(o.DependencyName, o.Region)
	}
	for _, a := range s.assessments {
		add(a.DependencyName, a.Region)
	}

	return targets, nil
}

// ── Diagnostics ───────────────────────────────────────────────────────────────

// Stats returns a summary of current store contents (for testing).
func (s *inMemoryStore) Stats() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]int{
		"dep_observations":   len(s.depObs),
		"probe_observations": len(s.probeObs),
		"vendor_statuses":    len(s.vendorStats),
		"assessments":        len(s.assessments),
		"incidents":          len(s.incidents),
		"incident_events":    len(s.events),
	}
}
