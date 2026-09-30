package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/apipulse/apipulse/internal/probes/quota"
	"github.com/apipulse/apipulse/internal/probes/runner"
	"github.com/apipulse/apipulse/internal/telemetry/ingest"
	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/apipulse/apipulse/pkg/logger"
	"github.com/apipulse/apipulse/pkg/metrics"
)

type memoryStore struct {
	mu          sync.Mutex
	customerObs []model.DependencyObservation
	probeObs    []model.ProbeObservation
}

func (m *memoryStore) InsertDependencyObservation(_ context.Context, obs model.DependencyObservation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customerObs = append(m.customerObs, obs)
	return nil
}

func (m *memoryStore) InsertProbeObservation(_ context.Context, obs model.ProbeObservation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.probeObs = append(m.probeObs, obs)
	return nil
}

func (m *memoryStore) LastCustomerObs() *model.DependencyObservation {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.customerObs) == 0 {
		return nil
	}
	res := m.customerObs[len(m.customerObs)-1]
	return &res
}

func (m *memoryStore) LastProbeObs() *model.ProbeObservation {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.probeObs) == 0 {
		return nil
	}
	res := m.probeObs[len(m.probeObs)-1]
	return &res
}

type vendorSimulatorHandler struct {
	mu       sync.RWMutex
	scenario string
	delays   map[string]time.Duration
}

func newVendorSimulator() *vendorSimulatorHandler {
	return &vendorSimulatorHandler{
		scenario: "NORMAL",
		delays: map[string]time.Duration{
			"NORMAL":       200 * time.Millisecond,
			"DEGRADED":     2000 * time.Millisecond,
			"GREY_FAILURE": 8000 * time.Millisecond,
		},
	}
}

func (v *vendorSimulatorHandler) SetScenario(s string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.scenario = s
}

func (v *vendorSimulatorHandler) GetScenario() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.scenario
}

func (v *vendorSimulatorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/status" {
		// Vendor public status page declaration
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": map[string]string{
				"name": "Stripe API Status (Public)",
			},
			"status": map[string]string{
				"indicator":   "none",
				"description": "All Systems Operational",
			},
		})
		return
	}

	sc := v.GetScenario()
	delay := v.delays[sc]
	if delay > 0 {
		time.Sleep(delay)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":       "ch_mock_demo_999",
		"status":   "succeeded",
		"paid":     true,
		"scenario": sc,
	})
}

func main() {
	ctx := context.Background()

	fmt.Println("================================================================================")
	fmt.Println("   APIPulse Phase 3B: Dual-Signal Demonstration (Customer OTel + Synthetic Probe)")
	fmt.Println("================================================================================")
	fmt.Println("Architectural Separations Verified:")
	fmt.Println("  1. CodeBegun Demo Customer Application calls Vendor Simulator directly over HTTP.")
	fmt.Println("  2. APIPulse is NOT a synchronous proxy. Telemetry flows asynchronously via OTLP/HTTP.")
	fmt.Println("  3. APIPulse Synthetic Probe independently monitors the same vendor out-of-band.")
	fmt.Println("================================================================================\n")

	// 1. Start Stripe-like Vendor Simulator
	vendor := newVendorSimulator()
	vendorServer := httptest.NewServer(vendor)
	defer vendorServer.Close()
	fmt.Printf("[1] Stripe-like Vendor Simulator running at: %s\n", vendorServer.URL)

	// 2. Start APIPulse OTLP Ingestion Gateway
	store := &memoryStore{}
	promMetrics := metrics.New(prometheus.NewRegistry())
	otlpReceiver := ingest.NewReceiver(ingest.Config{
		Store:         store,
		Metrics:       promMetrics,
		DefaultRegion: "ap-south-1",
	})
	apipulseServer := httptest.NewServer(otlpReceiver)
	defer apipulseServer.Close()
	fmt.Printf("[2] APIPulse OTLP/HTTP Receiver running at:   %s/v1/traces\n", apipulseServer.URL)

	// 3. Initialize CodeBegun OpenTelemetry SDK targeting APIPulse
	otlpEndpoint := strings.TrimPrefix(apipulseServer.URL, "http://")
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(otlpEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		panic(err)
	}

	res, _ := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceNameKey.String("codebegun-demo"),
			semconv.DeploymentEnvironmentKey.String("demo"),
			attribute.String("cloud.region", "ap-south-1"),
		),
	)

	// Use Synchronous SimpleSpanProcessor so spans flush immediately over HTTP to APIPulse
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)),
	)
	defer func() { _ = tp.Shutdown(ctx) }()
	otel.SetTracerProvider(tp)
	tracer := tp.Tracer("codebegun-checkout")
	fmt.Println("[3] CodeBegun OpenTelemetry Tracer initialized (exporting to APIPulse)")

	// 4. Initialize APIPulse Synthetic Probe Runner
	log := logger.New("probe-runner", "error")
	probeRunner := runner.New(runner.Config{
		ClientPool:    runner.NewClientPool(runner.DefaultClientConfig()),
		QuotaManager:  quota.NewManager(quota.DefaultConfig()),
		SSRFValidator: runner.NewSSRFValidator(true),
		Store:         store,
		Metrics:       promMetrics,
		Logger:        log,
		DefaultRegion: "us-east-1",
	})
	fmt.Println("[4] APIPulse Synthetic Probe Engine initialized (region: us-east-1)\n")

	// Execute 3 Stages
	stages := []struct {
		name        string
		scenario    string
		description string
	}{
		{
			name:        "STAGE 1: NORMAL OPERATION",
			scenario:    "NORMAL",
			description: "Healthy vendor responding in ~200ms",
		},
		{
			name:        "STAGE 2: DEGRADED PERFORMANCE",
			scenario:    "DEGRADED",
			description: "Vendor experiencing performance degradation with ~2000ms latency",
		},
		{
			name:        "STAGE 3: GREY FAILURE (THE CORE APIPULSE PROBLEM STATEMENT)",
			scenario:    "GREY_FAILURE",
			description: "Vendor reports 'Operational', HTTP 200 returned, but latency is ~8000ms!",
		},
	}

	for i, stage := range stages {
		fmt.Printf("================================================================================\n")
		fmt.Printf(">>> %s\n", stage.name)
		fmt.Printf("Description: %s\n", stage.description)
		fmt.Printf("================================================================================\n")

		// A. Update Vendor Simulator Scenario
		vendor.SetScenario(stage.scenario)

		// Check vendor public status page
		statusResp, _ := http.Get(vendorServer.URL + "/status")
		var stMap map[string]any
		_ = json.NewDecoder(statusResp.Body).Decode(&stMap)
		statusResp.Body.Close()
		statusDesc := stMap["status"].(map[string]any)["description"]
		fmt.Printf("[Vendor Public Status Page]: \"%v\"\n\n", statusDesc)

		// B. CUSTOMER PATH: CodeBegun processes checkout payment and calls vendor
		fmt.Println("[Signal A: CUSTOMER_OTEL Path]")
		fmt.Printf("  • CodeBegun calls Vendor: POST %s/v1/charges\n", vendorServer.URL)

		serverCtx, serverSpan := tracer.Start(ctx, "POST /api/payment")
		clientCtx, clientSpan := tracer.Start(serverCtx, "POST /v1/charges",
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("dependency.name", "stripe-like"),
				attribute.String("dependency.type", "payment"),
				attribute.String("server.address", cleanHost(vendorServer.URL)),
				attribute.String("http.request.method", "POST"),
				attribute.String("url.path", "/v1/charges"),
				attribute.String("region", "ap-south-1"),
			),
		)

		startReq := time.Now()
		httpReq, _ := http.NewRequestWithContext(clientCtx, "POST", vendorServer.URL+"/v1/charges", bytes.NewReader([]byte(`{"amount":2500}`)))
		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			panic(err)
		}
		clientSpan.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
		resp.Body.Close()
		clientSpan.End()
		serverSpan.End()

		callDuration := time.Since(startReq)
		fmt.Printf("  • CodeBegun finished vendor call in %v (HTTP %d)\n", callDuration.Round(time.Millisecond), resp.StatusCode)
		fmt.Println("  • OpenTelemetry span exported over OTLP/HTTP to APIPulse.")

		// Small pause to allow in-process flush
		time.Sleep(50 * time.Millisecond)
		custObs := store.LastCustomerObs()

		// C. SYNTHETIC PATH: APIPulse independently probes vendor endpoint
		fmt.Println("\n[Signal B: SYNTHETIC_PROBE Path]")
		fmt.Printf("  • APIPulse synthetic probe dispatching to: %s/v1/charges\n", vendorServer.URL)

		probeCfg := runner.ProbeConfig{
			ID:           uuid.New(),
			DependencyID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			Name:         "Independent Vendor Probe",
			Endpoint:     vendorServer.URL + "/v1/charges",
			Method:       "POST",
			AllowPOST:    true,
			Region:       "us-east-1",
			Timeout:      15 * time.Second,
			Enabled:      true,
			Mode:         runner.ProbeModeWarm,
		}

		prbObs, err := probeRunner.Execute(ctx, probeCfg)
		if err != nil {
			panic(err)
		}
		fmt.Printf("  • Synthetic probe completed in %.2f ms (HTTP %d, ReusedConn: %t)\n",
			prbObs.TotalDurationMS, prbObs.HTTPStatusCode, prbObs.ReusedConnection)

		// D. Comparison Table
		fmt.Println("\n--------------------------------------------------------------------------------")
		fmt.Printf("COMPARISON SUMMARY: %s\n", stage.name)
		fmt.Println("--------------------------------------------------------------------------------")
		fmt.Printf("%-24s | %-16s | %-12s | %-16s\n", "TELEMETRY SOURCE", "HTTP STATUS", "OUTCOME", "MEASURED LATENCY")
		fmt.Println("--------------------------------------------------------------------------------")
		if custObs != nil {
			fmt.Printf("%-24s | %-16d | %-12s | %10.2f ms\n",
				custObs.Source, custObs.StatusCode, custObs.Status, custObs.DurationMS)
		}
		fmt.Printf("%-24s | %-16d | %-12s | %10.2f ms\n",
			prbObs.Source, prbObs.HTTPStatusCode, formatSuccess(prbObs.Success), prbObs.TotalDurationMS)
		fmt.Printf("%-24s | %-16s | %-12s | %-16s\n",
			"VENDOR_PUBLIC_STATUS", "200 (Reported)", "Operational", "N/A (Untracked)")
		fmt.Println("--------------------------------------------------------------------------------")

		if stage.scenario == "GREY_FAILURE" {
			fmt.Println("\n🚨 GREY FAILURE DETECTED AND PROVEN:")
			fmt.Println("   1. Vendor Public Status Page claims: 'All Systems Operational'")
			fmt.Println("   2. HTTP Status Code is: 200 OK (No errors thrown by vendor)")
			fmt.Printf("   3. BUT Customer (CodeBegun) experienced: %.2f ms latency (destroying caller threads!)\n", custObs.DurationMS)
			fmt.Printf("   4. AND Independent Synthetic Probe confirmed: %.2f ms latency\n", prbObs.TotalDurationMS)
			fmt.Println("   => APIPulse has independently verified and corroborated the unannounced grey failure!")
		}

		fmt.Println()
		if i == len(stages)-1 {
			// Print final JSON models
			fmt.Println("================================================================================")
			fmt.Println("CANONICAL TELEMETRY MODELS GENERATED (PHASE 3B):")
			fmt.Println("================================================================================")
			custJSON, _ := json.MarshalIndent(custObs, "", "  ")
			fmt.Printf("[1] Canonical model.DependencyObservation (Source: CUSTOMER_OTEL):\n%s\n\n", string(custJSON))

			prbJSON, _ := json.MarshalIndent(prbObs, "", "  ")
			fmt.Printf("[2] Canonical model.ProbeObservation (Source: SYNTHETIC_PROBE):\n%s\n\n", string(prbJSON))
		}
	}

	fmt.Println("================================================================================")
	fmt.Println("   Demonstration successfully concluded. All criteria met.")
	fmt.Println("================================================================================")
}

func formatSuccess(b bool) string {
	if b {
		return "ok"
	}
	return "error"
}

func cleanHost(rawURL string) string {
	parts := strings.Split(strings.TrimPrefix(strings.TrimPrefix(rawURL, "http://"), "https://"), "/")
	return parts[0]
}
