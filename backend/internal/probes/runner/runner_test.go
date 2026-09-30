package runner_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/apipulse/apipulse/internal/probes/mocktarget"
	"github.com/apipulse/apipulse/internal/probes/quota"
	"github.com/apipulse/apipulse/internal/probes/runner"
	"github.com/apipulse/apipulse/internal/storage/postgres"
	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/apipulse/apipulse/pkg/logger"
	"github.com/apipulse/apipulse/pkg/metrics"
)

func setupTestRunner(t *testing.T, allowLocal bool) (*runner.Runner, *metrics.Metrics) {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)
	log := logger.New("test-runner", "error")

	r := runner.New(runner.Config{
		ClientPool:    runner.NewClientPool(runner.DefaultClientConfig()),
		QuotaManager:  quota.NewManager(quota.DefaultConfig()),
		SSRFValidator: runner.NewSSRFValidator(allowLocal),
		Store:         &postgres.NoopTelemetryStore{},
		Metrics:       m,
		Logger:        log,
		DefaultRegion: "us-east-1",
	})

	return r, m
}

func TestRunner_SuccessfulProbe(t *testing.T) {
	target := mocktarget.New(mocktarget.WithNormalDelay(5 * time.Millisecond))
	ts := httptest.NewServer(target)
	defer ts.Close()

	r, _ := setupTestRunner(t, true)

	probeID := uuid.New()
	depID := uuid.New()

	obs, err := r.Execute(context.Background(), runner.ProbeConfig{
		ID:           probeID,
		DependencyID: depID,
		Name:         "Normal Stripe Test",
		Endpoint:     ts.URL + "/scenario/normal",
		Method:       "GET",
		Region:       "us-east-1",
		Interval:     10 * time.Second,
		Timeout:      2 * time.Second,
		Enabled:      true,
		Mode:         runner.ProbeModeWarm,
		Validation: runner.ValidationConfig{
			ExpectedStatusCodes: []int{200},
			ExpectedContentType: "application/json",
		},
	})

	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if !obs.Success {
		t.Errorf("expected success=true, got error_type=%s, err=%s", obs.ErrorType, obs.NetworkError)
	}
	if obs.HTTPStatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", obs.HTTPStatusCode)
	}
	if obs.TotalDurationMS <= 0 {
		t.Errorf("expected positive duration, got %f", obs.TotalDurationMS)
	}
	if obs.ErrorType != model.ErrorTypeNone {
		t.Errorf("expected no error type, got %s", obs.ErrorType)
	}
}

func TestRunner_Timeout(t *testing.T) {
	target := mocktarget.New()
	ts := httptest.NewServer(target)
	defer ts.Close()

	r, _ := setupTestRunner(t, true)

	obs, err := r.Execute(context.Background(), runner.ProbeConfig{
		ID:           uuid.New(),
		DependencyID: uuid.New(),
		Name:         "Timeout Probe",
		Endpoint:     ts.URL + "/scenario/timeout",
		Method:       "GET",
		Region:       "eu-west-1",
		Interval:     10 * time.Second,
		Timeout:      50 * time.Millisecond, // very short timeout
		Enabled:      true,
		Mode:         runner.ProbeModeWarm,
	})

	if err != nil {
		t.Fatalf("expected nil error (failure captured in observation), got %v", err)
	}

	if obs.Success {
		t.Error("expected success=false for timeout")
	}
	if obs.ErrorType != model.ErrorTypeTimeout {
		t.Errorf("expected ErrorTypeTimeout, got %s", obs.ErrorType)
	}
	if !obs.Timeout {
		t.Error("expected obs.Timeout=true")
	}
}

func TestRunner_HTTP500_and_503(t *testing.T) {
	target := mocktarget.New()
	ts := httptest.NewServer(target)
	defer ts.Close()

	r, _ := setupTestRunner(t, true)

	// Test 500
	obs500, _ := r.Execute(context.Background(), runner.ProbeConfig{
		ID:           uuid.New(),
		DependencyID: uuid.New(),
		Name:         "500 Probe",
		Endpoint:     ts.URL + "/scenario/500",
		Method:       "GET",
		Enabled:      true,
	})
	if obs500.Success {
		t.Error("expected failure for 500")
	}
	if obs500.ErrorType != model.ErrorTypeHTTP5xx {
		t.Errorf("expected ErrorTypeHTTP5xx, got %s", obs500.ErrorType)
	}

	// Test 503
	obs503, _ := r.Execute(context.Background(), runner.ProbeConfig{
		ID:           uuid.New(),
		DependencyID: uuid.New(),
		Name:         "503 Probe",
		Endpoint:     ts.URL + "/scenario/503",
		Method:       "GET",
		Enabled:      true,
	})
	if obs503.Success {
		t.Error("expected failure for 503")
	}
	if obs503.ErrorType != model.ErrorTypeHTTP5xx {
		t.Errorf("expected ErrorTypeHTTP5xx, got %s", obs503.ErrorType)
	}
}

func TestRunner_HTTP429_RateLimited(t *testing.T) {
	target := mocktarget.New()
	ts := httptest.NewServer(target)
	defer ts.Close()

	r, _ := setupTestRunner(t, true)

	obs429, _ := r.Execute(context.Background(), runner.ProbeConfig{
		ID:           uuid.New(),
		DependencyID: uuid.New(),
		Name:         "429 Probe",
		Endpoint:     ts.URL + "/scenario/429",
		Method:       "GET",
		Enabled:      true,
	})

	if obs429.Success {
		t.Error("expected failure for 429")
	}
	if obs429.ErrorType != model.ErrorTypeRateLimited {
		t.Errorf("expected ErrorTypeRateLimited for 429, got %s", obs429.ErrorType)
	}
}

func TestRunner_ValidationFailure(t *testing.T) {
	target := mocktarget.New()
	ts := httptest.NewServer(target)
	defer ts.Close()

	r, _ := setupTestRunner(t, true)

	// Expect text/xml but mock returns application/json
	obs, _ := r.Execute(context.Background(), runner.ProbeConfig{
		ID:           uuid.New(),
		DependencyID: uuid.New(),
		Name:         "Validation Failure Probe",
		Endpoint:     ts.URL + "/scenario/normal",
		Method:       "GET",
		Enabled:      true,
		Validation: runner.ValidationConfig{
			ExpectedStatusCodes: []int{200},
			ExpectedContentType: "text/xml",
		},
	})

	if obs.Success {
		t.Error("expected failure due to mismatching Content-Type")
	}
	if obs.ErrorType != model.ErrorTypeValidationFailure {
		t.Errorf("expected ErrorTypeValidationFailure, got %s", obs.ErrorType)
	}
}

func TestRunner_ConnectionReuse(t *testing.T) {
	target := mocktarget.New(mocktarget.WithNormalDelay(1 * time.Millisecond))
	ts := httptest.NewServer(target)
	defer ts.Close()

	r, _ := setupTestRunner(t, true)
	cfg := runner.ProbeConfig{
		ID:           uuid.New(),
		DependencyID: uuid.New(),
		Name:         "Warm Reuse Test",
		Endpoint:     ts.URL + "/scenario/normal",
		Method:       "GET",
		Region:       "us-east-1",
		Enabled:      true,
		Mode:         runner.ProbeModeWarm,
	}

	// Warm probe 1
	obs1, err := r.Execute(context.Background(), cfg)
	if err != nil {
		t.Fatalf("probe 1 failed: %v", err)
	}

	// Warm probe 2
	obs2, err := r.Execute(context.Background(), cfg)
	if err != nil {
		t.Fatalf("probe 2 failed: %v", err)
	}

	if !obs1.Success || !obs2.Success {
		t.Error("expected both probes to succeed")
	}

	// Cold probe should force fresh connection
	cfgCold := cfg
	cfgCold.Mode = runner.ProbeModeColdDiagnostic
	obsCold, err := r.Execute(context.Background(), cfgCold)
	if err != nil {
		t.Fatalf("cold probe failed: %v", err)
	}
	if obsCold.ReusedConnection {
		t.Error("cold probe should not report reused connection")
	}
}

func TestRunner_ContextCancellation(t *testing.T) {
	target := mocktarget.New(mocktarget.WithNormalDelay(100 * time.Millisecond))
	ts := httptest.NewServer(target)
	defer ts.Close()

	r, _ := setupTestRunner(t, true)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	obs, _ := r.Execute(ctx, runner.ProbeConfig{
		ID:           uuid.New(),
		DependencyID: uuid.New(),
		Name:         "Cancelled Probe",
		Endpoint:     ts.URL + "/scenario/normal",
		Method:       "GET",
		Enabled:      true,
	})

	if obs.Success {
		t.Error("expected cancelled probe to fail")
	}
}

func TestRunner_ProbeDisabled(t *testing.T) {
	r, _ := setupTestRunner(t, true)

	_, err := r.Execute(context.Background(), runner.ProbeConfig{
		ID:           uuid.New(),
		DependencyID: uuid.New(),
		Endpoint:     "http://example.com",
		Enabled:      false,
	})

	if err == nil {
		t.Fatal("expected error for disabled probe")
	}
	if !strings.Contains(err.Error(), "disabled") {
		t.Errorf("expected disabled error, got: %v", err)
	}
}

func TestRunner_InvalidConfig(t *testing.T) {
	r, _ := setupTestRunner(t, true)

	// Missing ID
	_, err := r.Execute(context.Background(), runner.ProbeConfig{
		Endpoint: "http://example.com",
		Enabled:  true,
	})
	if err == nil {
		t.Fatal("expected error for missing ID")
	}

	// Unsafe method POST without AllowPOST
	_, err = r.Execute(context.Background(), runner.ProbeConfig{
		ID:           uuid.New(),
		DependencyID: uuid.New(),
		Endpoint:     "http://example.com",
		Method:       "POST",
		AllowPOST:    false,
		Enabled:      true,
	})
	if err == nil {
		t.Fatal("expected error for unauthorized POST method")
	}

	// Unsafe method DELETE
	_, err = r.Execute(context.Background(), runner.ProbeConfig{
		ID:           uuid.New(),
		DependencyID: uuid.New(),
		Endpoint:     "http://example.com",
		Method:       "DELETE",
		Enabled:      true,
	})
	if err == nil {
		t.Fatal("expected error for DELETE method")
	}
}

func TestRunner_SSRF_Protection(t *testing.T) {
	// Setup with AllowLocalTargets = false (Production mode)
	r, _ := setupTestRunner(t, false)

	// Attempt probe against metadata service
	obsMeta, _ := r.Execute(context.Background(), runner.ProbeConfig{
		ID:           uuid.New(),
		DependencyID: uuid.New(),
		Name:         "Metadata SSRF Test",
		Endpoint:     "http://169.254.169.254/latest/meta-data",
		Method:       "GET",
		Enabled:      true,
	})
	if obsMeta.Success {
		t.Error("expected metadata probe to be blocked by SSRF validator")
	}
	if obsMeta.ErrorType != model.ErrorTypeValidationFailure {
		t.Errorf("expected ErrorTypeValidationFailure for SSRF, got %s", obsMeta.ErrorType)
	}

	// Attempt probe against localhost loopback
	obsLocal, _ := r.Execute(context.Background(), runner.ProbeConfig{
		ID:           uuid.New(),
		DependencyID: uuid.New(),
		Name:         "Loopback SSRF Test",
		Endpoint:     "http://127.0.0.1:8080/internal",
		Method:       "GET",
		Enabled:      true,
	})
	if obsLocal.Success {
		t.Error("expected loopback probe to be blocked in production mode")
	}
}

func TestRunner_GreyFailure_Demonstration(t *testing.T) {
	// Simulates the exact APIPulse core problem:
	// The vendor returns HTTP 200 and says "status: operational",
	// but latency is dangerously high (~50ms in this test, 8-10s in production demo).
	target := mocktarget.New(mocktarget.WithGreyFailureDelay(50 * time.Millisecond))
	ts := httptest.NewServer(target)
	defer ts.Close()

	r, _ := setupTestRunner(t, true)

	obs, err := r.Execute(context.Background(), runner.ProbeConfig{
		ID:           uuid.New(),
		DependencyID: uuid.New(),
		Name:         "Grey Failure Probe",
		Endpoint:     ts.URL + "/scenario/grey-failure",
		Method:       "GET",
		Region:       "us-east-1",
		Timeout:      2 * time.Second,
		Enabled:      true,
	})

	if err != nil {
		t.Fatalf("grey failure probe failed: %v", err)
	}

	// In grey failure, the raw HTTP request technically returns 200 OK!
	if obs.HTTPStatusCode != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", obs.HTTPStatusCode)
	}
	if obs.TotalDurationMS < 40 {
		t.Errorf("expected elevated latency >= 40ms, got %f", obs.TotalDurationMS)
	}
}
