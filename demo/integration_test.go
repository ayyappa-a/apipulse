package demo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/apipulse/apipulse/internal/telemetry/ingest"
	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/apipulse/apipulse/pkg/metrics"
)

type capturedStore struct {
	mu  sync.Mutex
	obs []model.DependencyObservation
}

func (s *capturedStore) InsertDependencyObservation(_ context.Context, o model.DependencyObservation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.obs = append(s.obs, o)
	return nil
}

func (s *capturedStore) InsertProbeObservation(_ context.Context, _ model.ProbeObservation) error {
	return nil
}

func (s *capturedStore) getObs() []model.DependencyObservation {
	s.mu.Lock()
	defer s.mu.Unlock()
	dst := make([]model.DependencyObservation, len(s.obs))
	copy(dst, s.obs)
	return dst
}

// TestEndToEnd_RealHTTP_OTLP_Path exercises the live pipeline:
// Client -> CodeBegun (real HTTP) -> Vendor Simulator (real HTTP)
//   └─> OpenTelemetry OTLP/HTTP Exporter -> APIPulse Ingestion (real HTTP)
//         └─> Canonical CUSTOMER_OTEL observation
func TestEndToEnd_RealHTTP_OTLP_Path(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Start Live APIPulse OTLP Receiver on real HTTP test server
	store := &capturedStore{}
	m := metrics.New(prometheus.NewRegistry())
	otlpReceiver := ingest.NewReceiver(ingest.Config{
		Store:         store,
		Metrics:       m,
		DefaultRegion: "ap-south-1",
	})
	apipulseServer := httptest.NewServer(otlpReceiver)
	defer apipulseServer.Close()

	// 2. Start Live Vendor Simulator on real HTTP test server
	vendorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Millisecond) // Simulated latency
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "ch_live_test_123",
			"status": "succeeded",
		})
	}))
	defer vendorServer.Close()

	// 3. Initialize real OpenTelemetry SDK targeting the live APIPulse receiver
	endpoint := strings.TrimPrefix(apipulseServer.URL, "http://")
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		t.Fatalf("failed to create OTLP exporter: %v", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceNameKey.String("codebegun-demo"),
			semconv.DeploymentEnvironmentKey.String("demo"),
			attribute.String("cloud.region", "ap-south-1"),
		),
	)
	if err != nil {
		t.Fatalf("failed to build resource: %v", err)
	}

	// Use Synchronous/Batch span processor with immediate export
	bsp := sdktrace.NewSimpleSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	defer func() { _ = tp.Shutdown(ctx) }()
	otel.SetTracerProvider(tp)

	tracer := tp.Tracer("codebegun-checkout")

	// 4. Simulate a real customer checkout request inside CodeBegun
	// Root server span
	serverCtx, serverSpan := tracer.Start(ctx, "POST /api/payment")
	defer serverSpan.End()

	// Child outbound client span for vendor call
	clientCtx, clientSpan := tracer.Start(serverCtx, "POST /v1/charges",
		trace.WithSpanKind(trace.SpanKindClient),
	)
	clientSpan.SetAttributes(
		attribute.String("dependency.name", "stripe-like"),
		attribute.String("dependency.type", "payment"),
		attribute.String("server.address", strings.TrimPrefix(vendorServer.URL, "http://")),
		attribute.String("http.request.method", "POST"),
		attribute.String("url.path", "/v1/charges"),
		attribute.String("region", "ap-south-1"),
	)

	// Make real HTTP POST to Vendor Simulator
	httpReq, err := http.NewRequestWithContext(clientCtx, "POST", vendorServer.URL+"/v1/charges", bytes.NewReader([]byte(`{"amount":2500}`)))
	if err != nil {
		t.Fatalf("failed to construct request: %v", err)
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("call to vendor failed: %v", err)
	}
	clientSpan.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	resp.Body.Close()
	clientSpan.End() // This immediately triggers OTLP export via SimpleSpanProcessor to APIPulse!

	// 5. Force flush to ensure OTLP transport has completed over the wire
	if err := tp.ForceFlush(ctx); err != nil {
		t.Fatalf("force flush failed: %v", err)
	}

	// 6. Assert APIPulse received and created canonical CUSTOMER_OTEL observation
	obsList := store.getObs()
	if len(obsList) == 0 {
		t.Fatal("expected at least 1 CUSTOMER_OTEL observation in APIPulse store, got 0")
	}

	var found *model.DependencyObservation
	for i := range obsList {
		if obsList[i].DependencyName == "stripe-like" {
			found = &obsList[i]
			break
		}
	}

	if found == nil {
		t.Fatalf("could not find stripe-like dependency observation in stored list: %+v", obsList)
	}

	if found.Source != model.SourceCustomerOTEL {
		t.Errorf("expected Source=CUSTOMER_OTEL, got %s", found.Source)
	}
	if found.ServiceName != "codebegun-demo" {
		t.Errorf("expected ServiceName=codebegun-demo, got %s", found.ServiceName)
	}
	if found.Region != "ap-south-1" {
		t.Errorf("expected Region=ap-south-1, got %s", found.Region)
	}
	if found.StatusCode != http.StatusOK {
		t.Errorf("expected StatusCode=200, got %d", found.StatusCode)
	}
	if found.Status != model.StatusOK {
		t.Errorf("expected Status=ok, got %s", found.Status)
	}
	if found.DurationMS <= 0 {
		t.Errorf("expected positive DurationMS, got %f", found.DurationMS)
	}
}
