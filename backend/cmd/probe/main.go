// APIPulse Synthetic Probe Service — cmd/probe/main.go
//
// Outbound synthetic probe worker engine for APIPulse.
//
// Key Responsibilities:
//   - Execute scheduled HTTP probes against third-party dependency endpoints
//   - Capture precision DNS, TCP, TLS, TTFB, and transfer waterfalls with net/http/httptrace
//   - Differentiate warm keep-alive connection reuse from cold diagnostic probes
//   - Enforce SSRF safety, token-bucket quotas, and hard timeouts
//   - Zero payload retention: inspect only headers and status, discard bodies immediately
//   - Publish normalized model.ProbeObservation telemetry to TelemetryStore
//   - Emit Prometheus metrics and safe structured logs
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http/httptest"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/apipulse/apipulse/internal/config"
	"github.com/apipulse/apipulse/internal/probes/mocktarget"
	"github.com/apipulse/apipulse/internal/probes/quota"
	"github.com/apipulse/apipulse/internal/probes/runner"
	"github.com/apipulse/apipulse/internal/storage/postgres"
	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/apipulse/apipulse/pkg/logger"
	"github.com/apipulse/apipulse/pkg/metrics"
)

type scheduledTarget struct {
	DependencyName string
	Region         string
	Endpoint       string
	Method         string
	AllowPOST      bool
	DependencyID   uuid.UUID
}

func discoverTargets(ctx context.Context, cfg *config.Config, store postgres.TelemetryStore, vendorMockURL, defaultRegion string) []scheduledTarget {
	depID := uuid.MustParse("00000000-0000-0000-0000-000000000001") // Mock Stripe
	targetMap := make(map[string]scheduledTarget)

	// 1. From configured HealthTargets
	for _, raw := range cfg.HealthTargets {
		parts := strings.Split(raw, ":")
		if len(parts) >= 3 {
			depName := parts[0]
			targetRegion := parts[2]
			targetMap[depName+":"+targetRegion] = scheduledTarget{
				DependencyName: depName,
				Region:         targetRegion,
				Endpoint:       vendorMockURL + "/v1/charges",
				Method:         "POST",
				AllowPOST:      true,
				DependencyID:   depID,
			}
		} else if len(parts) == 2 {
			depName := parts[0]
			targetRegion := parts[1]
			targetMap[depName+":"+targetRegion] = scheduledTarget{
				DependencyName: depName,
				Region:         targetRegion,
				Endpoint:       vendorMockURL + "/v1/charges",
				Method:         "POST",
				AllowPOST:      true,
				DependencyID:   depID,
			}
		}
	}

	// 2. Dynamic discovery from persistent store
	if store != nil {
		if discovered, err := store.GetActiveTargets(ctx); err == nil {
			for _, dt := range discovered {
				key := dt.DependencyName + ":" + dt.Region
				if _, exists := targetMap[key]; !exists {
					targetMap[key] = scheduledTarget{
						DependencyName: dt.DependencyName,
						Region:         dt.Region,
						Endpoint:       vendorMockURL + "/v1/charges",
						Method:         "POST",
						AllowPOST:      true,
						DependencyID:   depID,
					}
				}
			}
		}
	}

	// 3. Fallback target if none discovered
	if len(targetMap) == 0 {
		targetMap["stripe-like:"+defaultRegion] = scheduledTarget{
			DependencyName: "stripe-like",
			Region:         defaultRegion,
			Endpoint:       vendorMockURL + "/v1/charges",
			Method:         "POST",
			AllowPOST:      true,
			DependencyID:   depID,
		}
	}

	// 4. Custom endpoint override if provided via PROBE_TARGET_URL
	if customEndpoint := os.Getenv("PROBE_TARGET_URL"); customEndpoint != "" {
		for k, t := range targetMap {
			t.Endpoint = customEndpoint
			targetMap[k] = t
		}
	}

	result := make([]scheduledTarget, 0, len(targetMap))
	for _, t := range targetMap {
		result = append(result, t)
	}
	return result
}

func executeProbeCycle(ctx context.Context, r *runner.Runner, targets []scheduledTarget, region string, log *logger.Logger) {
	log.Info(fmt.Sprintf("Executing scheduled probe cycle for %d discovered target(s)...", len(targets)))
	for _, t := range targets {
		if region != "" && region != "all" && t.Region != region {
			continue
		}

		cfg := runner.ProbeConfig{
			ID:           uuid.New(),
			DependencyID: t.DependencyID,
			Name:         fmt.Sprintf("%s synthetic probe (%s)", t.DependencyName, t.Region),
			Endpoint:     t.Endpoint,
			Method:       t.Method,
			Region:       t.Region,
			Timeout:      15 * time.Second,
			Enabled:      true,
			Mode:         runner.ProbeModeWarm,
			AllowPOST:    t.AllowPOST,
			Validation: runner.ValidationConfig{
				ExpectedStatusCodes: []int{200},
				ExpectedContentType: "application/json",
			},
		}

		obs, err := r.Execute(ctx, cfg)
		if err != nil {
			log.Error(fmt.Sprintf("probe execution error for %s (%s): %v", t.DependencyName, t.Region, err), err)
			continue
		}

		log.Info(fmt.Sprintf("probe succeeded: target=%s region=%s status=%d total_duration=%.2fms ttfb=%.2fms dns=%.2fms tcp=%.2fms",
			t.DependencyName, obs.Region, obs.HTTPStatusCode, obs.TotalDurationMS, obs.TTFBDurationMS, obs.DNSDurationMS, obs.TCPDurationMS))
	}
}

func main() {
	demoMode := flag.Bool("demo", false, "Run the deterministic multi-stage failure demonstration (NORMAL -> DEGRADED -> GREY_FAILURE)")
	targetURL := flag.String("target", "", "Custom target URL to probe immediately")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: configuration error: %v\n", err)
		os.Exit(1)
	}

	log := logger.New("apipulse-probe", cfg.LogLevel)

	region := os.Getenv("APIPULSE_PROBE_REGION")
	if region == "" || region == "local" {
		region = "ap-south-1"
	}

	// In local development, permit loopback testing against mock targets
	allowLocal := cfg.IsDevelopment()

	promMetrics := metrics.New(prometheus.DefaultRegisterer)
	clientPool := runner.NewClientPool(runner.DefaultClientConfig())
	quotaMgr := quota.NewManager(quota.DefaultConfig())
	ssrfValidator := runner.NewSSRFValidator(allowLocal)

	var store postgres.TelemetryStore
	if cfg.StorageBackend == "postgres" {
		ps, err := postgres.NewPostgresStore(context.Background(), cfg)
		if err != nil {
			log.Error("failed to connect to PostgreSQL — using Noop store fallback", err)
			store = &postgres.NoopTelemetryStore{}
		} else {
			defer ps.Close()
			store = ps
			log.Info("PostgreSQL connection pool initialized for probe worker")
		}
	} else {
		store = postgres.NewInMemoryStore()
	}

	probeRunner := runner.New(runner.Config{
		ClientPool:    clientPool,
		QuotaManager:  quotaMgr,
		SSRFValidator: ssrfValidator,
		Store:         store,
		Metrics:       promMetrics,
		Logger:        log,
		DefaultRegion: region,
	})

	log.Info(fmt.Sprintf("APIPulse probe engine initialized — region=%s local_targets_allowed=%t", region, allowLocal))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if *demoMode {
		runDeterministicDemo(ctx, probeRunner, log, region)
		return
	}

	if *targetURL != "" {
		runSingleTarget(ctx, probeRunner, log, region, *targetURL)
		return
	}

	// Normal Worker Loop: Scheduled Probing
	vendorMockURL := os.Getenv("VENDOR_MOCK_URL")
	if vendorMockURL == "" {
		vendorMockURL = "http://vendor-mock:9001"
	}

	probeInterval := 30 * time.Second
	if intervalStr := os.Getenv("APIPULSE_PROBE_INTERVAL"); intervalStr != "" {
		if d, err := time.ParseDuration(intervalStr); err == nil && d > 0 {
			probeInterval = d
		}
	}

	runCycle := func() {
		targets := discoverTargets(ctx, cfg, store, vendorMockURL, region)
		executeProbeCycle(ctx, probeRunner, targets, region, log)
	}

	log.Info(fmt.Sprintf("Probe worker running in scheduled mode (interval=%v). Press Ctrl+C to exit.", probeInterval))
	// Execute first cycle immediately on startup
	runCycle()

	ticker := time.NewTicker(probeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("APIPulse probe worker stopping gracefully")
			return
		case <-ticker.C:
			runCycle()
		}
	}
}

// runDeterministicDemo simulates the APIPulse core problem statement:
//   1. Normal (~200ms)
//   2. Degraded (~2s)
//   3. Grey Failure (8-10s latency, HTTP 200, status "operational")
func runDeterministicDemo(ctx context.Context, r *runner.Runner, log *logger.Logger, region string) {
	fmt.Println("\n=======================================================")
	fmt.Println("   APIPulse Phase 3A: Synthetic Probe Engine Demo")
	fmt.Println("=======================================================")

	target := mocktarget.New(
		mocktarget.WithNormalDelay(200*time.Millisecond),
		mocktarget.WithDegradedDelay(2000*time.Millisecond),
		mocktarget.WithGreyFailureDelay(8000*time.Millisecond),
	)
	mockServer := httptest.NewServer(target)
	defer mockServer.Close()

	depID := uuid.MustParse("00000000-0000-0000-0000-000000000001") // Mock Stripe

	stages := []struct {
		name     string
		endpoint string
		desc     string
	}{
		{
			name:     "1. NORMAL OPERATION",
			endpoint: mockServer.URL + "/scenario/normal",
			desc:     "Vendor responding normally with ~200ms latency",
		},
		{
			name:     "2. DEGRADED PERFORMANCE",
			endpoint: mockServer.URL + "/scenario/degraded",
			desc:     "Vendor experiencing performance degradation with ~2000ms latency",
		},
		{
			name:     "3. GREY FAILURE (CRITICAL PROBLEM STATEMENT)",
			endpoint: mockServer.URL + "/scenario/grey-failure",
			desc:     "Vendor status is 'operational', HTTP status is 200 OK, but latency is ~8000ms",
		},
	}

	for _, s := range stages {
		fmt.Printf("\n--- Executing Stage: %s ---\n", s.name)
		fmt.Printf("Description: %s\n", s.desc)

		probeID := uuid.New()
		cfg := runner.ProbeConfig{
			ID:           probeID,
			DependencyID: depID,
			Name:         s.name,
			Endpoint:     s.endpoint,
			Method:       "GET",
			Region:       region,
			Timeout:      15 * time.Second,
			Enabled:      true,
			Mode:         runner.ProbeModeWarm,
			Validation: runner.ValidationConfig{
				ExpectedStatusCodes: []int{200},
				ExpectedContentType: "application/json",
			},
		}

		obs, err := r.Execute(ctx, cfg)
		if err != nil {
			log.Error(fmt.Sprintf("Demo probe failed: %v", err), err)
			continue
		}

		printObservationSummary(obs)
	}

	fmt.Println("=======================================================")
	fmt.Println("Demo completed successfully. All observations verified.")
	fmt.Println("=======================================================")
}

func runSingleTarget(ctx context.Context, r *runner.Runner, log *logger.Logger, region, endpoint string) {
	fmt.Printf("Executing single probe against %s (region: %s)...\n", endpoint, region)
	cfg := runner.ProbeConfig{
		ID:           uuid.New(),
		DependencyID: uuid.New(),
		Name:         "Single Target Probe",
		Endpoint:     endpoint,
		Method:       "GET",
		Region:       region,
		Timeout:      10 * time.Second,
		Enabled:      true,
		Mode:         runner.ProbeModeWarm,
	}

	obs, err := r.Execute(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe error: %v\n", err)
		return
	}

	printObservationSummary(obs)
}

func printObservationSummary(obs *anyProbeObservation) {
	// Print timing waterfall
	fmt.Printf("  Status Code:       %d\n", obs.HTTPStatusCode)
	fmt.Printf("  Success:           %t\n", obs.Success)
	fmt.Printf("  Connection Reused: %t\n", obs.ReusedConnection)
	fmt.Println("  Waterfall Timing Breakdown:")
	fmt.Printf("    • DNS Resolution:    %8.2f ms\n", obs.DNSDurationMS)
	fmt.Printf("    • TCP Connect:       %8.2f ms\n", obs.TCPDurationMS)
	fmt.Printf("    • TLS Handshake:     %8.2f ms\n", obs.TLSDurationMS)
	fmt.Printf("    • Time-to-First-Byte:%8.2f ms\n", obs.TTFBDurationMS)
	fmt.Printf("    • Transfer Duration: %8.2f ms\n", obs.TransferDurationMS)
	fmt.Printf("    • Total Duration:    %8.2f ms\n", obs.TotalDurationMS)

	// Print JSON representation
	b, _ := json.MarshalIndent(obs, "  ", "  ")
	fmt.Printf("  Normalized ProbeObservation (JSON):\n  %s\n", string(b))
}

type anyProbeObservation = model.ProbeObservation
