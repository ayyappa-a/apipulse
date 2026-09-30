package ingest_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/apipulse/apipulse/internal/storage/postgres"
	"github.com/apipulse/apipulse/internal/telemetry/ingest"
	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/apipulse/apipulse/pkg/metrics"
)

// mockStore captures inserted observations for testing assertions.
type mockStore struct {
	mu       sync.Mutex
	depObs   []model.DependencyObservation
	probeObs []model.ProbeObservation
}

func (m *mockStore) InsertDependencyObservation(_ context.Context, obs model.DependencyObservation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.depObs = append(m.depObs, obs)
	return nil
}

func (m *mockStore) InsertProbeObservation(_ context.Context, obs model.ProbeObservation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.probeObs = append(m.probeObs, obs)
	return nil
}

func (m *mockStore) getDepObs() []model.DependencyObservation {
	m.mu.Lock()
	defer m.mu.Unlock()
	dst := make([]model.DependencyObservation, len(m.depObs))
	copy(dst, m.depObs)
	return dst
}

// ── Noop implementations of extended TelemetryStore interface (Phase 3C) ──

func (m *mockStore) InsertVendorStatus(_ context.Context, _ model.VendorStatusRecord) error {
	return nil
}
func (m *mockStore) InsertAssessment(_ context.Context, _ model.HealthAssessment) error {
	return nil
}
func (m *mockStore) UpsertIncident(_ context.Context, _ model.Incident) error { return nil }
func (m *mockStore) InsertIncidentEvent(_ context.Context, _ model.IncidentEvent) error {
	return nil
}
func (m *mockStore) GetRecentDependencyObservations(_ context.Context, _, _ string, _ time.Time) ([]model.DependencyObservation, error) {
	return nil, nil
}
func (m *mockStore) GetRecentProbeObservations(_ context.Context, _, _ string, _ time.Time) ([]model.ProbeObservation, error) {
	return nil, nil
}
func (m *mockStore) GetLatestVendorStatus(_ context.Context, _, _ string) (*model.VendorStatusRecord, error) {
	return nil, nil
}
func (m *mockStore) GetLatestAssessment(_ context.Context, _, _ string) (*model.HealthAssessment, error) {
	return nil, nil
}
func (m *mockStore) ListAssessments(_ context.Context, _ string, _ int) ([]model.HealthAssessment, error) {
	return nil, nil
}
func (m *mockStore) GetActiveIncident(_ context.Context, _ string) (*model.Incident, error) {
	return nil, nil
}
func (m *mockStore) ListIncidents(_ context.Context, _ int) ([]model.Incident, error) {
	return nil, nil
}
func (m *mockStore) GetIncident(_ context.Context, _ string) (*model.Incident, error) {
	return nil, nil
}
func (m *mockStore) GetActiveTargets(_ context.Context) ([]postgres.Target, error) {
	return nil, nil
}


func buildSampleOTLPRequest(depName string, statusCode int, duration time.Duration) *coltracepb.ExportTraceServiceRequest {
	start := time.Now().Add(-duration)
	end := time.Now()

	return &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "codebegun-demo"}}},
						{Key: "deployment.environment.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "demo"}}},
						{Key: "cloud.region", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "ap-south-1"}}},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
								SpanId:            []byte{1, 2, 3, 4, 5, 6, 7, 8},
								Name:              "POST /v1/charges",
								Kind:              tracepb.Span_SPAN_KIND_CLIENT,
								StartTimeUnixNano: uint64(start.UnixNano()),
								EndTimeUnixNano:   uint64(end.UnixNano()),
								Attributes: []*commonpb.KeyValue{
									{Key: "dependency.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: depName}}},
									{Key: "http.request.method", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "POST"}}},
									{Key: "url.path", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "/v1/charges"}}},
									{Key: "http.response.status_code", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: int64(statusCode)}}},
									// Sensitive attributes that MUST be ignored:
									{Key: "authorization", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Bearer sec_test_12345"}}},
									{Key: "card.number", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "424242424242"}}},
									{Key: "http.request.body", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: `{"amount": 1000}`}}},
								},
								Status: &tracepb.Status{
									Code: tracepb.Status_STATUS_CODE_OK,
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestReceiver_ProtobufOTLP(t *testing.T) {
	store := &mockStore{}
	m := metrics.New(prometheus.NewRegistry())
	rec := ingest.NewReceiver(ingest.Config{
		Store:         store,
		Metrics:       m,
		DefaultRegion: "ap-south-1",
	})

	otlpReq := buildSampleOTLPRequest("stripe-like", 200, 200*time.Millisecond)
	protoBytes, err := proto.Marshal(otlpReq)
	if err != nil {
		t.Fatalf("failed to marshal proto: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(protoBytes))
	req.Header.Set("Content-Type", "application/x-protobuf")
	recResp := httptest.NewRecorder()

	rec.ServeHTTP(recResp, req)

	if recResp.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", recResp.Code, recResp.Body.String())
	}

	obs := store.getDepObs()
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation stored, got %d", len(obs))
	}

	o := obs[0]
	if o.Source != model.SourceCustomerOTEL {
		t.Errorf("expected source=%s, got %s", model.SourceCustomerOTEL, o.Source)
	}
	if o.ServiceName != "codebegun-demo" {
		t.Errorf("expected service_name=codebegun-demo, got %s", o.ServiceName)
	}
	if o.DependencyName != "stripe-like" {
		t.Errorf("expected dependency_name=stripe-like, got %s", o.DependencyName)
	}
	if o.Region != "ap-south-1" {
		t.Errorf("expected region=ap-south-1, got %s", o.Region)
	}
	if o.StatusCode != 200 {
		t.Errorf("expected status_code=200, got %d", o.StatusCode)
	}
	if o.Status != model.StatusOK {
		t.Errorf("expected status=ok, got %s", o.Status)
	}
	if o.DurationMS <= 0 {
		t.Errorf("expected positive duration_ms, got %f", o.DurationMS)
	}
}

func TestReceiver_JSON_OTLP(t *testing.T) {
	store := &mockStore{}
	m := metrics.New(prometheus.NewRegistry())
	rec := ingest.NewReceiver(ingest.Config{
		Store:         store,
		Metrics:       m,
		DefaultRegion: "ap-south-1",
	})

	otlpReq := buildSampleOTLPRequest("stripe-like", 200, 8000*time.Millisecond) // Grey failure latency
	jsonBytes, err := protojson.Marshal(otlpReq)
	if err != nil {
		t.Fatalf("failed to marshal protojson: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry/otlp", bytes.NewReader(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	recResp := httptest.NewRecorder()

	rec.ServeHTTP(recResp, req)

	if recResp.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", recResp.Code, recResp.Body.String())
	}

	obs := store.getDepObs()
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation stored, got %d", len(obs))
	}

	o := obs[0]
	if o.Source != model.SourceCustomerOTEL {
		t.Errorf("expected source CUSTOMER_OTEL, got %s", o.Source)
	}
	// Verify that HTTP 200 with 8s latency is mapped to StatusOK (NOT prematurely flagged as app error)
	if o.Status != model.StatusOK {
		t.Errorf("expected status=ok for grey failure HTTP 200, got %s", o.Status)
	}
	if o.DurationMS < 7000 {
		t.Errorf("expected duration >= 7000ms, got %f", o.DurationMS)
	}
}

func TestReceiver_ErrorMapping(t *testing.T) {
	store := &mockStore{}
	m := metrics.New(prometheus.NewRegistry())
	rec := ingest.NewReceiver(ingest.Config{
		Store: store,
		Metrics: m,
	})

	// 500 Internal Server Error
	req500 := buildSampleOTLPRequest("stripe-like", 500, 100*time.Millisecond)
	protoBytes, _ := proto.Marshal(req500)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(protoBytes))
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	w := httptest.NewRecorder()
	rec.ServeHTTP(w, httpReq)

	obs := store.getDepObs()
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}
	if obs[0].Status != model.StatusError {
		t.Errorf("expected status=error for 500, got %s", obs[0].Status)
	}
	if obs[0].ErrorType != model.ErrorTypeHTTP5xx {
		t.Errorf("expected error_type=http_5xx, got %s", obs[0].ErrorType)
	}
}

func TestReceiver_MalformedPayload(t *testing.T) {
	rec := ingest.NewReceiver(ingest.Config{})

	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader([]byte("not valid proto")))
	req.Header.Set("Content-Type", "application/x-protobuf")
	w := httptest.NewRecorder()

	rec.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", w.Code)
	}
}
