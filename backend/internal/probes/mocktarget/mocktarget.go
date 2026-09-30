// Package mocktarget provides a deterministic local test target for synthetic probes.
//
// It simulates real-world vendor behaviors without contacting external internet services:
//   - NORMAL: healthy response with ~200ms latency
//   - DEGRADED: slow response with ~2s latency
//   - GREY_FAILURE: HTTP 200, status "operational", but 8–10s latency
//   - TIMEOUT: hangs beyond probe deadline
//   - HTTP_500: Internal server error
//   - HTTP_503: Service unavailable
//   - HTTP_429: Upstream rate limit
//
// This is strictly for automated verification, CI, and local demonstrations.
package mocktarget

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Scenario represents the operating condition simulated by the mock target.
type Scenario string

const (
	ScenarioNormal      Scenario = "NORMAL"
	ScenarioDegraded    Scenario = "DEGRADED"
	ScenarioGreyFailure Scenario = "GREY_FAILURE"
	ScenarioTimeout     Scenario = "TIMEOUT"
	ScenarioHTTP500     Scenario = "HTTP_500"
	ScenarioHTTP503     Scenario = "HTTP_503"
	ScenarioHTTP429     Scenario = "HTTP_429"
)

// Target is an HTTP handler simulating vendor endpoints with controllable scenarios.
type Target struct {
	mu              sync.RWMutex
	currentScenario Scenario

	// Configurable delays for scenarios
	normalDelay      time.Duration
	degradedDelay    time.Duration
	greyFailureDelay time.Duration
	timeoutDelay     time.Duration
}

// Option configures a Target instance.
type Option func(*Target)

// WithNormalDelay overrides the default 200ms normal delay (useful for fast unit tests).
func WithNormalDelay(d time.Duration) Option {
	return func(t *Target) {
		t.normalDelay = d
	}
}

// WithDegradedDelay overrides the default 2s degraded delay.
func WithDegradedDelay(d time.Duration) Option {
	return func(t *Target) {
		t.degradedDelay = d
	}
}

// WithGreyFailureDelay overrides the default 8s grey failure delay.
func WithGreyFailureDelay(d time.Duration) Option {
	return func(t *Target) {
		t.greyFailureDelay = d
	}
}

// New creates a new deterministic mock vendor target.
func New(opts ...Option) *Target {
	t := &Target{
		currentScenario:  ScenarioNormal,
		normalDelay:      200 * time.Millisecond,
		degradedDelay:    2000 * time.Millisecond,
		greyFailureDelay: 8000 * time.Millisecond,
		timeoutDelay:     30 * time.Second,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// SetScenario changes the scenario served on default paths.
func (t *Target) SetScenario(s Scenario) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.currentScenario = s
}

// GetScenario returns the active default scenario.
func (t *Target) GetScenario() Scenario {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.currentScenario
}

// ServeHTTP handles requests based on route or current dynamic scenario.
func (t *Target) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	scenario := t.resolveScenario(r)

	switch scenario {
	case ScenarioNormal:
		t.sleep(r, t.normalDelay)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":   "operational",
			"vendor":   "mock-payment-gateway",
			"latency":  "normal",
			"scenario": string(scenario),
		})

	case ScenarioDegraded:
		t.sleep(r, t.degradedDelay)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":   "operational",
			"vendor":   "mock-payment-gateway",
			"latency":  "degraded",
			"scenario": string(scenario),
		})

	case ScenarioGreyFailure:
		// The critical APIPulse scenario: Vendor status is reported operational,
		// HTTP response code is 200 OK, but latency is 8-10s (destroying caller threads).
		t.sleep(r, t.greyFailureDelay)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":   "operational",
			"vendor":   "mock-payment-gateway",
			"latency":  "grey_failure_severe_latency",
			"scenario": string(scenario),
		})

	case ScenarioTimeout:
		t.sleep(r, t.timeoutDelay)
		w.WriteHeader(http.StatusOK)

	case ScenarioHTTP500:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "internal_server_error",
			"type":  "api_error",
		})

	case ScenarioHTTP503:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "service_unavailable",
			"type":  "capacity_exhausted",
		})

	case ScenarioHTTP429:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "rate_limit_exceeded",
			"type":  "rate_limit_error",
		})

	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (t *Target) resolveScenario(r *http.Request) Scenario {
	path := r.URL.Path
	switch path {
	case "/scenario/normal":
		return ScenarioNormal
	case "/scenario/degraded":
		return ScenarioDegraded
	case "/scenario/grey-failure":
		return ScenarioGreyFailure
	case "/scenario/timeout":
		return ScenarioTimeout
	case "/scenario/500":
		return ScenarioHTTP500
	case "/scenario/503":
		return ScenarioHTTP503
	case "/scenario/429":
		return ScenarioHTTP429
	default:
		t.mu.RLock()
		defer t.mu.RUnlock()
		return t.currentScenario
	}
}

func (t *Target) sleep(r *http.Request, d time.Duration) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-r.Context().Done():
	}
}
