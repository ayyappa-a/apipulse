// CodeBegun Demo Customer Service — demo/codebegun/main.go
//
// ⚠️  DEMONSTRATION SYSTEM:
// This is a self-hosted customer demonstration service used to generate genuine
// OpenTelemetry telemetry for APIPulse. CodeBegun is not an external production
// company with production access; this service represents the customer application.
//
// Architectural Invariant:
// This service makes direct outbound HTTP calls to the payment vendor simulator.
// APIPulse is NOT a synchronous proxy and never sits in the customer's production request path.
// Spans are exported asynchronously via standard OTLP/HTTP to APIPulse.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultPort       = "8081"
	defaultVendorURL  = "http://localhost:9001"
	defaultOTLPEndpoint = "api:8080"
	defaultRegion     = "ap-south-1"
	serviceName       = "codebegun-demo"
)

// App manages the CodeBegun demo service and its OpenTelemetry tracer.
type App struct {
	vendorURL  string
	region     string
	tracer     trace.Tracer
	httpClient *http.Client
}

// PaymentRequest represents a checkout payment payload.
type PaymentRequest struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	OrderID  string `json:"order_id"`
}

// PaymentResponse is returned to the caller.
type PaymentResponse struct {
	OrderID         string  `json:"order_id"`
	Status          string  `json:"status"`
	VendorLatencyMS float64 `json:"vendor_latency_ms"`
	TraceID         string  `json:"trace_id"`
	Region          string  `json:"region"`
	Error           string  `json:"error,omitempty"`
}

func initTracer(ctx context.Context, otlpEndpoint string, region string) (*sdktrace.TracerProvider, error) {
	// Configure OTLP/HTTP exporter sending to APIPulse OTLP receiver
	// Note: otlptracehttp defaults to appending /v1/traces which APIPulse natively handles.
	cleanEndpoint := normalizeEndpoint(otlpEndpoint)
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cleanEndpoint),
		otlptracehttp.WithInsecure(), // local development
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceNameKey.String(serviceName),
			semconv.DeploymentEnvironmentKey.String("demo"),
			attribute.String("cloud.region", region),
			attribute.String("telemetry.origin", "codebegun-checkout"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Use BatchSpanProcessor with short export timeout for demo responsiveness
	bsp := sdktrace.NewBatchSpanProcessor(
		exporter,
		sdktrace.WithBatchTimeout(500*time.Millisecond),
		sdktrace.WithExportTimeout(5*time.Second),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(tp)
	return tp, nil
}

func NewApp(vendorURL, region string, tracer trace.Tracer) *App {
	return &App{
		vendorURL: vendorURL,
		region:    region,
		tracer:    tracer,
		httpClient: &http.Client{
			Timeout: 15 * time.Second, // Bounded client timeout
		},
	}
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", a.handleHealth)
	mux.HandleFunc("POST /api/payment", a.handlePayment)
	mux.HandleFunc("GET /api/demo/scenario", a.handleGetScenario)
	mux.HandleFunc("POST /api/demo/scenario", a.handleSetScenario)

	return mux
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":     "healthy",
		"service":    serviceName,
		"region":     a.region,
		"vendor_url": a.vendorURL,
	})
}

func (a *App) handlePayment(w http.ResponseWriter, r *http.Request) {
	// 1. Root Server Span
	ctx, serverSpan := a.tracer.Start(r.Context(), "POST /api/payment",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("http.route", "/api/payment"),
			attribute.String("http.method", "POST"),
		),
	)
	defer serverSpan.End()

	var req PaymentRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.OrderID == "" {
		req.OrderID = fmt.Sprintf("ord_%d", time.Now().UnixNano())
	}
	if req.Amount <= 0 {
		req.Amount = 2500 // $25.00
	}
	if req.Currency == "" {
		req.Currency = "usd"
	}

	// 2. Child Dependency Client Span for Outbound Call to Stripe Simulator
	vendorCallStart := time.Now()

	clientCtx, clientSpan := a.tracer.Start(ctx, "POST /v1/charges",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			// Semantic convention attributes:
			attribute.String("dependency.name", "stripe-like"),
			attribute.String("dependency.type", "payment"),
			attribute.String("peer.service", "stripe-like"),
			attribute.String("server.address", cleanHost(a.vendorURL)),
			attribute.String("http.request.method", "POST"),
			attribute.String("url.path", "/v1/charges"),
			attribute.String("region", a.region),
			// STRICT PRIVACY CONTRACT:
			// NEVER set card numbers, credentials, auth tokens, or request bodies here.
		),
	)

	// Direct outbound call to vendor (APIPulse is NOT in the path)
	vendorReqBody, _ := json.Marshal(map[string]any{
		"amount":   req.Amount,
		"currency": req.Currency,
	})

	httpReq, err := http.NewRequestWithContext(clientCtx, "POST", a.vendorURL+"/v1/charges", bytes.NewReader(vendorReqBody))
	if err != nil {
		clientSpan.RecordError(err)
		clientSpan.SetStatus(codes.Error, err.Error())
		clientSpan.End()

		writeJSON(w, http.StatusInternalServerError, PaymentResponse{
			OrderID: req.OrderID,
			Status:  "failed",
			Error:   err.Error(),
			Region:  a.region,
		})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	vendorResp, err := a.httpClient.Do(httpReq)
	vendorDuration := time.Since(vendorCallStart)

	if err != nil {
		// Network timeout or connection refusal
		clientSpan.RecordError(err)
		clientSpan.SetStatus(codes.Error, fmt.Sprintf("vendor network failure: %v", err))
		clientSpan.SetAttributes(
			attribute.Int("http.response.status_code", 0),
		)
		clientSpan.End()

		writeJSON(w, http.StatusGatewayTimeout, PaymentResponse{
			OrderID:         req.OrderID,
			Status:          "timeout",
			VendorLatencyMS: float64(vendorDuration.Milliseconds()),
			TraceID:         serverSpan.SpanContext().TraceID().String(),
			Region:          a.region,
			Error:           err.Error(),
		})
		return
	}
	defer vendorResp.Body.Close()

	// Read and discard body up to 64KB
	_, _ = io.Copy(io.Discard, io.LimitReader(vendorResp.Body, 64*1024))

	// Record HTTP Status Code on span
	clientSpan.SetAttributes(
		attribute.Int("http.response.status_code", vendorResp.StatusCode),
	)

	// Status & Error semantics:
	// If vendor returns 200 (even with 8s Grey Failure latency), span is OK!
	// It is NOT prematurely marked as an application error.
	if vendorResp.StatusCode >= 500 {
		clientSpan.SetStatus(codes.Error, fmt.Sprintf("HTTP %d error from vendor", vendorResp.StatusCode))
	} else if vendorResp.StatusCode >= 400 {
		clientSpan.SetStatus(codes.Error, fmt.Sprintf("HTTP %d client error from vendor", vendorResp.StatusCode))
	} else {
		clientSpan.SetStatus(codes.Ok, "payment processed")
	}

	clientSpan.End() // End client span -> batched for export to APIPulse

	respStatus := "completed"
	httpCode := http.StatusOK
	if vendorResp.StatusCode != http.StatusOK {
		respStatus = "vendor_degraded"
		httpCode = vendorResp.StatusCode
	}

	writeJSON(w, httpCode, PaymentResponse{
		OrderID:         req.OrderID,
		Status:          respStatus,
		VendorLatencyMS: float64(vendorDuration.Milliseconds()),
		TraceID:         serverSpan.SpanContext().TraceID().String(),
		Region:          a.region,
	})
}

func (a *App) handleGetScenario(w http.ResponseWriter, r *http.Request) {
	resp, err := a.httpClient.Get(a.vendorURL + "/admin/scenario")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.Copy(w, resp.Body)
}

func (a *App) handleSetScenario(w http.ResponseWriter, r *http.Request) {
	resp, err := a.httpClient.Post(a.vendorURL+"/admin/scenario", "application/json", r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.Copy(w, resp.Body)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func cleanHost(rawURL string) string {
	parts := strings.Split(strings.TrimPrefix(strings.TrimPrefix(rawURL, "http://"), "https://"), "/")
	return parts[0]
}

// normalizeEndpoint removes any URL scheme (http:// or https://) from the OTLP endpoint string.
func normalizeEndpoint(endpoint string) string {
	if strings.HasPrefix(endpoint, "http://") {
		return strings.TrimPrefix(endpoint, "http://")
	}
	if strings.HasPrefix(endpoint, "https://") {
		return strings.TrimPrefix(endpoint, "https://")
	}
	return endpoint
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}
	vendorURL := os.Getenv("VENDOR_URL")
	if vendorURL == "" {
		vendorURL = defaultVendorURL
	}
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		otlpEndpoint = defaultOTLPEndpoint
	}
	region := os.Getenv("DEMO_REGION")
	if region == "" {
		region = defaultRegion
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Printf("[codebegun-demo] Initializing OpenTelemetry exporter targeting APIPulse at %s", otlpEndpoint)
	tp, err := initTracer(ctx, otlpEndpoint, region)
	if err != nil {
		log.Fatalf("failed to init OpenTelemetry tracer: %v", err)
	}
	defer func() {
		shutdownCtx, sCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer sCancel()
		_ = tp.Shutdown(shutdownCtx)
	}()

	tracer := tp.Tracer("codebegun-checkout")
	app := NewApp(vendorURL, region, tracer)

	addr := ":" + port
	log.Printf("[codebegun-demo] Running on %s (region: %s, vendor: %s)", addr, region, vendorURL)
	log.Printf("[codebegun-demo] Endpoints: POST /api/payment, GET /health, POST /api/demo/scenario")

	server := &http.Server{
		Addr:    addr,
		Handler: app.Routes(),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, sCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer sCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}
