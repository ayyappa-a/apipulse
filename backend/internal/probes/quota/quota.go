// Package quota enforces outbound probe rate limits and scheduling quotas
// to protect upstream vendors from excessive synthetic traffic.
package quota

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrQuotaExceeded is returned when a probe is blocked by local rate limits.
	ErrQuotaExceeded = errors.New("probe quota exceeded: rate limit reached")
)

// TokenBucket implements a thread-safe token bucket rate limiter.
type TokenBucket struct {
	mu         sync.Mutex
	capacity   float64
	tokens     float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewTokenBucket creates a new token bucket with given capacity and refill rate (tokens/sec).
func NewTokenBucket(capacity float64, refillRate float64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow attempts to consume 1 token. Returns true if allowed, false if exhausted.
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.lastRefill = now

	// Refill tokens based on elapsed time
	tb.tokens += elapsed * tb.refillRate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}

	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}

	return false
}

// Manager manages multi-level quotas (per-dependency and per-region).
type Manager struct {
	mu sync.RWMutex

	defaultDependencyCapacity float64
	defaultDependencyRate     float64

	defaultRegionCapacity float64
	defaultRegionRate     float64

	dependencyBuckets map[uuid.UUID]*TokenBucket
	regionBuckets     map[string]*TokenBucket
}

// Config holds rate limit settings for the quota manager.
type Config struct {
	// DependencyMaxTokens is the burst capacity per dependency (default 10).
	DependencyMaxTokens float64
	// DependencyRatePerSec is steady probe rate allowed per dependency (default 2.0).
	DependencyRatePerSec float64

	// RegionMaxTokens is burst capacity across a region (default 50).
	RegionMaxTokens float64
	// RegionRatePerSec is steady probe rate allowed per region (default 10.0).
	RegionRatePerSec float64
}

// DefaultConfig provides conservative defaults safe for testing and outbound probing.
func DefaultConfig() Config {
	return Config{
		DependencyMaxTokens:  10.0,
		DependencyRatePerSec: 2.0,
		RegionMaxTokens:      50.0,
		RegionRatePerSec:     10.0,
	}
}

// NewManager creates a Quota Manager with the specified configuration.
func NewManager(cfg Config) *Manager {
	if cfg.DependencyMaxTokens <= 0 {
		cfg.DependencyMaxTokens = 10.0
	}
	if cfg.DependencyRatePerSec <= 0 {
		cfg.DependencyRatePerSec = 2.0
	}
	if cfg.RegionMaxTokens <= 0 {
		cfg.RegionMaxTokens = 50.0
	}
	if cfg.RegionRatePerSec <= 0 {
		cfg.RegionRatePerSec = 10.0
	}

	return &Manager{
		defaultDependencyCapacity: cfg.DependencyMaxTokens,
		defaultDependencyRate:     cfg.DependencyRatePerSec,
		defaultRegionCapacity:     cfg.RegionMaxTokens,
		defaultRegionRate:         cfg.RegionRatePerSec,
		dependencyBuckets:         make(map[uuid.UUID]*TokenBucket),
		regionBuckets:             make(map[string]*TokenBucket),
	}
}

// Allow checks both dependency-level and region-level quotas.
// Returns nil if allowed, or ErrQuotaExceeded with context if blocked.
func (m *Manager) Allow(dependencyID uuid.UUID, region string) error {
	m.mu.Lock()

	// 1. Check or initialize region bucket
	rBucket, exists := m.regionBuckets[region]
	if !exists {
		rBucket = NewTokenBucket(m.defaultRegionCapacity, m.defaultRegionRate)
		m.regionBuckets[region] = rBucket
	}

	// 2. Check or initialize dependency bucket
	dBucket, exists := m.dependencyBuckets[dependencyID]
	if !exists {
		dBucket = NewTokenBucket(m.defaultDependencyCapacity, m.defaultDependencyRate)
		m.dependencyBuckets[dependencyID] = dBucket
	}

	m.mu.Unlock()

	// Check region limit first
	if !rBucket.Allow() {
		return fmt.Errorf("%w: region quota exceeded for %s", ErrQuotaExceeded, region)
	}

	// Check dependency limit
	if !dBucket.Allow() {
		return fmt.Errorf("%w: dependency quota exceeded for %s", ErrQuotaExceeded, dependencyID)
	}

	return nil
}
