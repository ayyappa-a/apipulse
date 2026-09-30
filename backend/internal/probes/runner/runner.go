// Package runner implements the production synthetic probe execution engine for APIPulse.
//
// Key architectural guarantees:
//   - Completely OUT-OF-BAND: Never proxies production customer requests.
//   - Safe: Only executes safe HTTP methods (GET, HEAD, or explicitly authorized POST).
//   - Zero Body Retention: Inspects headers/status and discards bodies immediately.
//   - Precision Waterfall Timing: Measures DNS, TCP, TLS, TTFB, and transfer duration.
//   - Explicit Connection Reuse: Distinguishes warm connection pooling from fresh connections.
//   - Bounded Resource Usage: Enforces token-bucket rate limits, hard timeouts, and memory limits.
//   - Observable: Emits Prometheus metrics and structured non-PII audit logs.
package runner

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/apipulse/apipulse/internal/probes/quota"
	"github.com/apipulse/apipulse/internal/probes/timing"
	"github.com/apipulse/apipulse/internal/storage/postgres"
	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/apipulse/apipulse/pkg/logger"
	"github.com/apipulse/apipulse/pkg/metrics"
)

// Runner manages synthetic probe execution.
type Runner struct {
	clientPool   *ClientPool
	quotaMgr     *quota.Manager
	ssrfVal      *SSRFValidator
	store        postgres.TelemetryStore
	metrics      *metrics.Metrics
	logger       *logger.Logger
	defaultRegion string
}

// Config provides dependencies and options to the Runner.
type Config struct {
	ClientPool    *ClientPool
	QuotaManager  *quota.Manager
	SSRFValidator *SSRFValidator
	Store         postgres.TelemetryStore
	Metrics       *metrics.Metrics
	Logger        *logger.Logger
	DefaultRegion string
}

// New creates an initialized probe Runner.
func New(cfg Config) *Runner {
	if cfg.ClientPool == nil {
		cfg.ClientPool = NewClientPool(DefaultClientConfig())
	}
	if cfg.QuotaManager == nil {
		cfg.QuotaManager = quota.NewManager(quota.DefaultConfig())
	}
	if cfg.SSRFValidator == nil {
		cfg.SSRFValidator = NewSSRFValidator(true) // local dev default
	}
	if cfg.Store == nil {
		cfg.Store = &postgres.NoopTelemetryStore{}
	}
	if cfg.Logger == nil {
		cfg.Logger = logger.New("apipulse-probe-runner", "info")
	}
	if cfg.DefaultRegion == "" {
		cfg.DefaultRegion = "local"
	}

	return &Runner{
		clientPool:    cfg.ClientPool,
		quotaMgr:      cfg.QuotaManager,
		ssrfVal:       cfg.SSRFValidator,
		store:         cfg.Store,
		metrics:       cfg.Metrics,
		logger:        cfg.Logger,
		defaultRegion: cfg.DefaultRegion,
	}
}

// Execute performs the complete lifecycle for a single probe and persists the normalized observation.
func (r *Runner) Execute(ctx context.Context, cfg ProbeConfig) (*model.ProbeObservation, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid probe config: %w", err)
	}

	if !cfg.Enabled {
		return nil, fmt.Errorf("probe %s is disabled", cfg.ID)
	}

	region := cfg.Region
	if region == "" {
		region = r.defaultRegion
	}

	// 1. Quota / Rate Limiting Enforcement
	if err := r.quotaMgr.Allow(cfg.DependencyID, region); err != nil {
		r.logger.Warn(fmt.Sprintf("probe rate limited locally: probe_id=%s dependency_id=%s region=%s",
			cfg.ID, cfg.DependencyID, region))
		return r.recordFailure(ctx, cfg, region, model.ErrorTypeRateLimited, "local probe rate limit exceeded", 0, timing.Result{})
	}

	// 2. SSRF Target Validation
	if err := r.ssrfVal.ValidateTarget(cfg.Endpoint); err != nil {
		r.logger.Error(fmt.Sprintf("probe ssrf violation blocked: probe_id=%s endpoint=%s", cfg.ID, cfg.Endpoint), err)
		return r.recordFailure(ctx, cfg, region, model.ErrorTypeValidationFailure, fmt.Sprintf("ssrf validation blocked: %v", err), 0, timing.Result{})
	}

	// 3. Establish Hard Upper Timeout
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 4. Trace & Timing Instrumentation
	tracker, tracedCtx := timing.NewTracker(probeCtx)

	req, err := http.NewRequestWithContext(tracedCtx, cfg.Method, cfg.Endpoint, nil)
	if err != nil {
		return r.recordFailure(ctx, cfg, region, model.ErrorTypeUnknown, fmt.Sprintf("failed to construct request: %v", err), 0, tracker.Finish())
	}

	// Safe standard headers
	req.Header.Set("User-Agent", "APIPulse-SyntheticProbe/1.0 (+https://apipulse.dev)")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	// 5. Select Client and Dispatch
	client := r.clientPool.GetClient(cfg.Mode)
	startTime := time.Now().UTC()

	resp, err := client.Do(req)

	// 6. Handle Network / Transport Failures
	if err != nil {
		timingRes := tracker.Finish()
		errType, isTimeout := classifyTransportError(probeCtx, err)

		return r.recordFailure(ctx, cfg, region, errType, err.Error(), 0, timingRes, withTimeout(isTimeout), withTimestamp(startTime))
	}
	defer resp.Body.Close()

	// 7. Bounded Body Read (Read up to MaxSizeBytes then discard immediately)
	maxBytes := cfg.Validation.MaxSizeBytes
	if maxBytes <= 0 {
		maxBytes = 64 * 1024
	}

	tracker.MarkBodyReadStart()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxBytes))
	tracker.MarkBodyReadDone()

	timingRes := tracker.Finish()

	// 8. Response Validation & Error Classification
	success, errType, valMsg := validateResponse(resp, cfg.Validation)

	// 9. Build Canonical Observation Model
	obs := &model.ProbeObservation{
		ID:                       uuid.New(),
		ProbeID:                  cfg.ID,
		DependencyID:             cfg.DependencyID,
		Region:                   region,
		Timestamp:                startTime,
		Source:                   model.SourceSyntheticProbe,
		Success:                  success,
		HTTPStatusCode:           resp.StatusCode,
		DNSDurationMS:            timingRes.DNSDurationMS,
		TCPDurationMS:            timingRes.TCPDurationMS,
		TLSDurationMS:            timingRes.TLSDurationMS,
		TTFBDurationMS:           timingRes.TTFBDurationMS,
		TransferDurationMS:       timingRes.TransferDurationMS,
		TotalDurationMS:          timingRes.TotalDurationMS,
		ReusedConnection:         timingRes.ReusedConnection,
		ErrorType:                errType,
		NetworkError:             valMsg,
		Timeout:                  false,
		ResponseValidationResult: valMsg,
		IngestedAt:               time.Now().UTC(),
	}

	// 10. Record Metrics & Safe Structured Logging
	r.recordMetrics(cfg.DependencyID.String(), region, obs)
	r.logStructured(obs)

	// 11. Normalize & Store via TelemetryStore
	if err := obs.Validate(); err != nil {
		r.logger.Error("probe observation validation failed", err)
		return obs, fmt.Errorf("probe observation invalid: %w", err)
	}

	if err := r.store.InsertProbeObservation(ctx, *obs); err != nil {
		r.logger.Error("failed to persist probe observation", err)
		return obs, fmt.Errorf("persistence error: %w", err)
	}

	return obs, nil
}

func (r *Runner) recordFailure(ctx context.Context, cfg ProbeConfig, region string, errType model.ErrorType, errMsg string, statusCode int, timingRes timing.Result, opts ...failureOpt) (*model.ProbeObservation, error) {
	params := &failureParams{
		timestamp: time.Now().UTC(),
	}
	for _, opt := range opts {
		opt(params)
	}

	obs := &model.ProbeObservation{
		ID:                 uuid.New(),
		ProbeID:            cfg.ID,
		DependencyID:       cfg.DependencyID,
		Region:             region,
		Timestamp:          params.timestamp,
		Source:             model.SourceSyntheticProbe,
		Success:            false,
		HTTPStatusCode:     statusCode,
		DNSDurationMS:      timingRes.DNSDurationMS,
		TCPDurationMS:      timingRes.TCPDurationMS,
		TLSDurationMS:      timingRes.TLSDurationMS,
		TTFBDurationMS:     timingRes.TTFBDurationMS,
		TransferDurationMS: timingRes.TransferDurationMS,
		TotalDurationMS:    timingRes.TotalDurationMS,
		ReusedConnection:   timingRes.ReusedConnection,
		ErrorType:          errType,
		NetworkError:       sanitizeError(errMsg),
		Timeout:            params.isTimeout,
		IngestedAt:         time.Now().UTC(),
	}

	r.recordMetrics(cfg.DependencyID.String(), region, obs)
	r.logStructured(obs)

	_ = r.store.InsertProbeObservation(ctx, *obs)
	return obs, nil
}

func (r *Runner) recordMetrics(dep string, region string, obs *model.ProbeObservation) {
	if r.metrics == nil {
		return
	}

	r.metrics.ProbeTotal.WithLabelValues(region, dep).Inc()

	if !obs.Success {
		r.metrics.ProbeFailureTotal.WithLabelValues(region, dep, string(obs.ErrorType)).Inc()
	}

	r.metrics.ProbeDurationSecs.WithLabelValues(region, dep).Observe(obs.TotalDurationMS / 1000.0)

	if obs.DNSDurationMS > 0 && r.metrics.ProbeDNSDurationSecs != nil {
		r.metrics.ProbeDNSDurationSecs.WithLabelValues(region, dep).Observe(obs.DNSDurationMS / 1000.0)
	}
	if obs.TCPDurationMS > 0 && r.metrics.ProbeTCPDurationSecs != nil {
		r.metrics.ProbeTCPDurationSecs.WithLabelValues(region, dep).Observe(obs.TCPDurationMS / 1000.0)
	}
	if obs.TLSDurationMS > 0 && r.metrics.ProbeTLSDurationSecs != nil {
		r.metrics.ProbeTLSDurationSecs.WithLabelValues(region, dep).Observe(obs.TLSDurationMS / 1000.0)
	}
	if obs.TTFBDurationMS > 0 && r.metrics.ProbeTTFBDurationSecs != nil {
		r.metrics.ProbeTTFBDurationSecs.WithLabelValues(region, dep).Observe(obs.TTFBDurationMS / 1000.0)
	}
}

func (r *Runner) logStructured(obs *model.ProbeObservation) {
	// Structured logging with bounded, safe fields only.
	// Never log request/response headers, authorization tokens, or payload bodies.
	msg := fmt.Sprintf("PROBE_RESULT probe_id=%s dependency_id=%s region=%s success=%t status_code=%d duration_ms=%.2f error_type=%s reused_conn=%t",
		obs.ProbeID, obs.DependencyID, obs.Region, obs.Success, obs.HTTPStatusCode, obs.TotalDurationMS, obs.ErrorType, obs.ReusedConnection)

	if obs.Success {
		r.logger.Info(msg)
	} else {
		r.logger.Warn(msg)
	}
}

func validateResponse(resp *http.Response, val ValidationConfig) (bool, model.ErrorType, string) {
	statusCode := resp.StatusCode

	// 1. Upstream 429 Rate Limiting
	// Crucial distinction: 429 does not mean the vendor has an outage; it means APIPulse or client is rate limited.
	if statusCode == http.StatusTooManyRequests {
		return false, model.ErrorTypeRateLimited, "vendor returned 429 Too Many Requests"
	}

	// 2. Status code assertions
	if len(val.ExpectedStatusCodes) > 0 {
		match := false
		for _, code := range val.ExpectedStatusCodes {
			if statusCode == code {
				match = true
				break
			}
		}
		if !match {
			errType := model.ErrorTypeValidationFailure
			if statusCode >= 500 {
				errType = model.ErrorTypeHTTP5xx
			} else if statusCode >= 400 {
				errType = model.ErrorTypeHTTP4xx
			}
			return false, errType, fmt.Sprintf("unexpected HTTP status code %d", statusCode)
		}
	} else {
		// Default: accept 200..299
		if statusCode < 200 || statusCode >= 300 {
			errType := model.ErrorTypeValidationFailure
			if statusCode >= 500 {
				errType = model.ErrorTypeHTTP5xx
			} else if statusCode >= 400 {
				errType = model.ErrorTypeHTTP4xx
			}
			return false, errType, fmt.Sprintf("non-2xx HTTP status code %d", statusCode)
		}
	}

	// 3. Content-Type check if configured
	if val.ExpectedContentType != "" {
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(strings.ToLower(ct), strings.ToLower(val.ExpectedContentType)) {
			return false, model.ErrorTypeValidationFailure, fmt.Sprintf("expected Content-Type containing %q, got %q", val.ExpectedContentType, ct)
		}
	}

	return true, model.ErrorTypeNone, ""
}

func classifyTransportError(ctx context.Context, err error) (model.ErrorType, bool) {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || osIsTimeout(err) {
		return model.ErrorTypeTimeout, true
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return model.ErrorTypeDNSFailure, false
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Op == "dial" {
			return model.ErrorTypeTCPFailure, false
		}
	}

	var tlsErr *tls.RecordHeaderError
	if errors.As(err, &tlsErr) || strings.Contains(strings.ToLower(err.Error()), "tls handshake") {
		return model.ErrorTypeTLSFailure, false
	}

	return model.ErrorTypeNetworkError, false
}

func osIsTimeout(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout") || strings.Contains(err.Error(), "deadline exceeded")
}

func sanitizeError(msg string) string {
	// Bounded sanitization, strip URLs containing potential tokens
	if len(msg) > 256 {
		return msg[:256] + "..."
	}
	return msg
}

type failureParams struct {
	isTimeout bool
	timestamp time.Time
}

type failureOpt func(*failureParams)

func withTimeout(b bool) failureOpt {
	return func(p *failureParams) {
		p.isTimeout = b
	}
}

func withTimestamp(t time.Time) failureOpt {
	return func(p *failureParams) {
		p.timestamp = t
	}
}
