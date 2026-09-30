// Package redis implements Redis-backed hot operational state for APIPulse.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/apipulse/apipulse/pkg/metrics"
)

// DefaultRedisOpTimeout is the bounded timeout for hot-cache operations.
// Redis is non-authoritative operational cache. Hot-cache operations must
// never block or exhaust the parent evaluation or readiness context.
const DefaultRedisOpTimeout = 300 * time.Millisecond

// RedisCache implements StateCache using go-redis/v9 with graceful degradation.
type RedisCache struct {
	client  *redis.Client
	metrics *metrics.Metrics

	// In-memory fallback for hot state when Redis is temporarily unreachable
	fallbackMu          sync.RWMutex
	fallbackHealth      map[string]model.AssessmentStatus
	fallbackAssessment  map[string]*model.HealthAssessment
	fallbackCounters    map[string]int64
	fallbackDepStatuses map[string]string
}

// NewRedisCache connects to Redis with the given URL.
func NewRedisCache(ctx context.Context, redisURL string, m *metrics.Metrics) (*RedisCache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	opts.DialTimeout = DefaultRedisOpTimeout
	opts.ReadTimeout = DefaultRedisOpTimeout
	opts.WriteTimeout = DefaultRedisOpTimeout
	opts.MaxRetries = 1

	client := redis.NewClient(opts)

	// Test connection
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return &RedisCache{
		client:              client,
		metrics:             m,
		fallbackHealth:      make(map[string]model.AssessmentStatus),
		fallbackAssessment:  make(map[string]*model.HealthAssessment),
		fallbackCounters:    make(map[string]int64),
		fallbackDepStatuses: make(map[string]string),
	}, nil
}

// NewTestRedisCache creates a RedisCache with the specified client for testing.
func NewTestRedisCache(client *redis.Client, m *metrics.Metrics) *RedisCache {
	return &RedisCache{
		client:              client,
		metrics:             m,
		fallbackHealth:      make(map[string]model.AssessmentStatus),
		fallbackAssessment:  make(map[string]*model.HealthAssessment),
		fallbackCounters:    make(map[string]int64),
		fallbackDepStatuses: make(map[string]string),
	}
}

// opContext derives an isolated child context with a short bounded timeout
// so that cache operations cannot poison or exhaust the parent context.
func (c *RedisCache) opContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		return context.WithTimeout(context.Background(), DefaultRedisOpTimeout)
	}
	return context.WithTimeout(parent, DefaultRedisOpTimeout)
}

// IsActive returns true if Redis responds to PING within the bounded timeout.
func (c *RedisCache) IsActive(ctx context.Context) bool {
	if c.client == nil {
		return false
	}
	opCtx, cancel := c.opContext(ctx)
	defer cancel()
	return c.client.Ping(opCtx).Err() == nil
}

// Close closes the Redis connection.
func (c *RedisCache) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// ── Dependency health state ──────────────────────────────────────────────────

func (c *RedisCache) SetDependencyHealth(
	ctx context.Context,
	dependencyName, region string,
	status model.AssessmentStatus,
	ttl time.Duration,
) error {
	if c.metrics != nil {
		c.metrics.RedisOperationsTotal.WithLabelValues("set_dependency_health").Inc()
	}

	key := fmt.Sprintf("apipulse:health:%s:%s", dependencyName, region)

	// Keep in-memory fallback warm for graceful degradation
	c.fallbackMu.Lock()
	c.fallbackHealth[key] = status
	c.fallbackMu.Unlock()

	if c.client == nil {
		return nil
	}

	opCtx, cancel := c.opContext(ctx)
	defer cancel()

	err := c.client.Set(opCtx, key, string(status), ttl).Err()
	if err != nil {
		if c.metrics != nil {
			c.metrics.RedisErrorsTotal.WithLabelValues("set_dependency_health").Inc()
		}
		return err
	}
	return nil
}

func (c *RedisCache) GetDependencyHealth(
	ctx context.Context,
	dependencyName, region string,
) (model.AssessmentStatus, error) {
	if c.metrics != nil {
		c.metrics.RedisOperationsTotal.WithLabelValues("get_dependency_health").Inc()
	}

	key := fmt.Sprintf("apipulse:health:%s:%s", dependencyName, region)
	if c.client == nil {
		c.fallbackMu.RLock()
		fb := c.fallbackHealth[key]
		c.fallbackMu.RUnlock()
		return fb, nil
	}

	opCtx, cancel := c.opContext(ctx)
	defer cancel()

	val, err := c.client.Get(opCtx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		if c.metrics != nil {
			c.metrics.RedisErrorsTotal.WithLabelValues("get_dependency_health").Inc()
		}
		c.fallbackMu.RLock()
		fb, ok := c.fallbackHealth[key]
		c.fallbackMu.RUnlock()
		if ok {
			return fb, nil
		}
		return "", err
	}
	return model.AssessmentStatus(val), nil
}

func (c *RedisCache) SetLatestAssessment(
	ctx context.Context,
	dependencyName, region string,
	assessment *model.HealthAssessment,
	ttl time.Duration,
) error {
	if c.metrics != nil {
		c.metrics.RedisOperationsTotal.WithLabelValues("set_latest_assessment").Inc()
	}

	key := fmt.Sprintf("apipulse:assessment:%s:%s", dependencyName, region)

	// Keep in-memory fallback warm for graceful degradation
	c.fallbackMu.Lock()
	c.fallbackAssessment[key] = assessment
	c.fallbackMu.Unlock()

	if c.client == nil {
		return nil
	}

	data, err := json.Marshal(assessment)
	if err != nil {
		return fmt.Errorf("marshal assessment: %w", err)
	}

	opCtx, cancel := c.opContext(ctx)
	defer cancel()

	err = c.client.Set(opCtx, key, data, ttl).Err()
	if err != nil {
		if c.metrics != nil {
			c.metrics.RedisErrorsTotal.WithLabelValues("set_latest_assessment").Inc()
		}
		return err
	}
	return nil
}

func (c *RedisCache) GetLatestAssessment(
	ctx context.Context,
	dependencyName, region string,
) (*model.HealthAssessment, error) {
	if c.metrics != nil {
		c.metrics.RedisOperationsTotal.WithLabelValues("get_latest_assessment").Inc()
	}

	key := fmt.Sprintf("apipulse:assessment:%s:%s", dependencyName, region)
	if c.client == nil {
		c.fallbackMu.RLock()
		fb := c.fallbackAssessment[key]
		c.fallbackMu.RUnlock()
		return fb, nil
	}

	opCtx, cancel := c.opContext(ctx)
	defer cancel()

	val, err := c.client.Get(opCtx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		if c.metrics != nil {
			c.metrics.RedisErrorsTotal.WithLabelValues("get_latest_assessment").Inc()
		}
		c.fallbackMu.RLock()
		fb, ok := c.fallbackAssessment[key]
		c.fallbackMu.RUnlock()
		if ok {
			return fb, nil
		}
		return nil, err
	}

	var a model.HealthAssessment
	if err := json.Unmarshal(val, &a); err != nil {
		return nil, fmt.Errorf("unmarshal cached assessment: %w", err)
	}
	return &a, nil
}

// ── Hysteresis / recovery counters ──────────────────────────────────────────

func (c *RedisCache) IncrHealthyWindowCount(
	ctx context.Context,
	dependencyName, region string,
	ttl time.Duration,
) (int64, error) {
	if c.metrics != nil {
		c.metrics.RedisOperationsTotal.WithLabelValues("incr_healthy_window").Inc()
	}

	key := fmt.Sprintf("apipulse:recovery:%s:%s", dependencyName, region)
	if c.client == nil {
		c.fallbackMu.Lock()
		c.fallbackCounters[key]++
		cnt := c.fallbackCounters[key]
		c.fallbackMu.Unlock()
		return cnt, nil
	}

	opCtx, cancel := c.opContext(ctx)
	defer cancel()

	count, err := c.client.Incr(opCtx, key).Result()
	if err != nil {
		if c.metrics != nil {
			c.metrics.RedisErrorsTotal.WithLabelValues("incr_healthy_window").Inc()
		}
		c.fallbackMu.Lock()
		c.fallbackCounters[key]++
		cnt := c.fallbackCounters[key]
		c.fallbackMu.Unlock()
		return cnt, nil
	}

	c.fallbackMu.Lock()
	c.fallbackCounters[key] = count
	c.fallbackMu.Unlock()

	if ttl > 0 {
		expCtx, expCancel := c.opContext(ctx)
		_ = c.client.Expire(expCtx, key, ttl).Err()
		expCancel()
	}
	return count, nil
}

func (c *RedisCache) ResetHealthyWindowCount(
	ctx context.Context,
	dependencyName, region string,
) error {
	if c.metrics != nil {
		c.metrics.RedisOperationsTotal.WithLabelValues("reset_healthy_window").Inc()
	}

	key := fmt.Sprintf("apipulse:recovery:%s:%s", dependencyName, region)
	c.fallbackMu.Lock()
	delete(c.fallbackCounters, key)
	c.fallbackMu.Unlock()

	if c.client == nil {
		return nil
	}

	opCtx, cancel := c.opContext(ctx)
	defer cancel()

	err := c.client.Del(opCtx, key).Err()
	if err != nil && c.metrics != nil {
		c.metrics.RedisErrorsTotal.WithLabelValues("reset_healthy_window").Inc()
	}
	return nil
}

func (c *RedisCache) GetHealthyWindowCount(
	ctx context.Context,
	dependencyName, region string,
) (int64, error) {
	if c.metrics != nil {
		c.metrics.RedisOperationsTotal.WithLabelValues("get_healthy_window").Inc()
	}

	key := fmt.Sprintf("apipulse:recovery:%s:%s", dependencyName, region)
	if c.client == nil {
		c.fallbackMu.RLock()
		cnt := c.fallbackCounters[key]
		c.fallbackMu.RUnlock()
		return cnt, nil
	}

	opCtx, cancel := c.opContext(ctx)
	defer cancel()

	count, err := c.client.Get(opCtx, key).Int64()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		if c.metrics != nil {
			c.metrics.RedisErrorsTotal.WithLabelValues("get_healthy_window").Inc()
		}
		c.fallbackMu.RLock()
		cnt := c.fallbackCounters[key]
		c.fallbackMu.RUnlock()
		return cnt, nil
	}
	return count, nil
}

// ── Backward-compatible status ──────────────────────────────────────────────

func (c *RedisCache) SetDependencyStatus(ctx context.Context, dependencyID, status string) error {
	if c.metrics != nil {
		c.metrics.RedisOperationsTotal.WithLabelValues("set_dependency_status").Inc()
	}

	key := fmt.Sprintf("apipulse:dep:%s:status", dependencyID)
	c.fallbackMu.Lock()
	c.fallbackDepStatuses[key] = status
	c.fallbackMu.Unlock()

	if c.client == nil {
		return nil
	}

	opCtx, cancel := c.opContext(ctx)
	defer cancel()

	err := c.client.Set(opCtx, key, status, 24*time.Hour).Err()
	if err != nil {
		if c.metrics != nil {
			c.metrics.RedisErrorsTotal.WithLabelValues("set_dependency_status").Inc()
		}
		return err
	}
	return nil
}

func (c *RedisCache) GetDependencyStatus(ctx context.Context, dependencyID string) (string, error) {
	if c.metrics != nil {
		c.metrics.RedisOperationsTotal.WithLabelValues("get_dependency_status").Inc()
	}

	key := fmt.Sprintf("apipulse:dep:%s:status", dependencyID)
	if c.client == nil {
		c.fallbackMu.RLock()
		s := c.fallbackDepStatuses[key]
		c.fallbackMu.RUnlock()
		return s, nil
	}

	opCtx, cancel := c.opContext(ctx)
	defer cancel()

	val, err := c.client.Get(opCtx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		if c.metrics != nil {
			c.metrics.RedisErrorsTotal.WithLabelValues("get_dependency_status").Inc()
		}
		c.fallbackMu.RLock()
		s := c.fallbackDepStatuses[key]
		c.fallbackMu.RUnlock()
		return s, nil
	}
	return val, nil
}
