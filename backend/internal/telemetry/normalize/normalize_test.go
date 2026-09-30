package normalize

import (
	"testing"
	"time"

	"github.com/apipulse/apipulse/internal/telemetry/model"
	"github.com/google/uuid"
)

func TestSanitizeOperation_UUIDs(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{
			input:    "GET /v1/customers/550e8400-e29b-41d4-a716-446655440000/invoices",
			expected: "GET /v1/customers/:id/invoices",
		},
		{
			input:    "POST /v1/charges/123",
			expected: "POST /v1/charges/:id",
		},
		{
			input:    "GET /v1/health",
			expected: "GET /v1/health",
		},
		{
			input:    "",
			expected: "",
		},
	}
	for _, tc := range cases {
		got := sanitizeOperation(tc.input)
		if got != tc.expected {
			t.Errorf("sanitizeOperation(%q): expected %q, got %q", tc.input, tc.expected, got)
		}
	}
}

func TestSanitizeOperation_Truncation(t *testing.T) {
	long := "GET /" + string(make([]byte, 300))
	result := sanitizeOperation(long)
	if len(result) > 256 {
		t.Errorf("expected operation to be truncated to 256 chars, got %d", len(result))
	}
}

func TestNormalizeDependencyObservation_SetsSource(t *testing.T) {
	raw := &model.DependencyObservation{
		ID:             uuid.New(),
		Timestamp:      time.Now(),
		ServiceName:    "my-service",
		DependencyName: "Stripe",
		Operation:      "POST /v1/charges",
		Region:         "US-EAST-1",
		Source:         model.SourceSyntheticProbe, // wrong — normalizer must override
		DurationMS:     100,
		Status:         model.StatusOK,
		StatusCode:     200,
		Environment:    "Production",
	}

	out := NormalizeDependencyObservation(raw)

	if out.Source != model.SourceCustomerOTEL {
		t.Errorf("expected Source=%s, got %s", model.SourceCustomerOTEL, out.Source)
	}
	if out.DependencyName != "stripe" {
		t.Errorf("expected lowercase dependency_name, got %q", out.DependencyName)
	}
	if out.Region != "us-east-1" {
		t.Errorf("expected lowercase region, got %q", out.Region)
	}
	if out.Environment != "production" {
		t.Errorf("expected lowercase environment, got %q", out.Environment)
	}
	if out.Timestamp.Location() != time.UTC {
		t.Error("expected timestamp to be UTC")
	}
	if out.IngestedAt.IsZero() {
		t.Error("expected IngestedAt to be set by normalizer")
	}
}

func TestNormalizeDependencyObservation_DoesNotMutateInput(t *testing.T) {
	raw := &model.DependencyObservation{
		ID:             uuid.New(),
		Timestamp:      time.Now(),
		ServiceName:    "my-service",
		DependencyName: "STRIPE",
		Operation:      "POST /v1/charges",
		Region:         "us-east-1",
		Source:         model.SourceCustomerOTEL,
		DurationMS:     100,
		Status:         model.StatusOK,
		StatusCode:     200,
		Environment:    "production",
	}

	original := raw.DependencyName
	NormalizeDependencyObservation(raw)

	if raw.DependencyName != original {
		t.Error("NormalizeDependencyObservation mutated the input struct")
	}
}

func TestClassifyErrorType(t *testing.T) {
	cases := []struct {
		statusCode int
		status     model.ObservationStatus
		expected   model.ErrorType
	}{
		{0, model.StatusTimeout, model.ErrorTypeTimeout},
		{404, model.StatusError, model.ErrorTypeHTTP4xx},
		{500, model.StatusError, model.ErrorTypeHTTP5xx},
		{503, model.StatusError, model.ErrorTypeHTTP5xx},
		{0, model.StatusError, model.ErrorTypeUnknown},
		{200, model.StatusOK, model.ErrorTypeNone},
	}
	for _, tc := range cases {
		got := classifyErrorType(tc.statusCode, tc.status)
		if got != tc.expected {
			t.Errorf("classifyErrorType(%d, %s): expected %s, got %s",
				tc.statusCode, tc.status, tc.expected, got)
		}
	}
}

func TestNormalizeProbeObservation_SetsSource(t *testing.T) {
	raw := &model.ProbeObservation{
		ID:              uuid.New(),
		ProbeID:         uuid.New(),
		DependencyID:    uuid.New(),
		Region:          "EU-WEST-1",
		Timestamp:       time.Now(),
		Source:          model.SourceVendorStatus, // wrong — normalizer must override
		Success:         true,
		HTTPStatusCode:  200,
		TotalDurationMS: 150,
	}

	out := NormalizeProbeObservation(raw)

	if out.Source != model.SourceSyntheticProbe {
		t.Errorf("expected Source=%s, got %s", model.SourceSyntheticProbe, out.Source)
	}
	if out.Region != "eu-west-1" {
		t.Errorf("expected lowercase region, got %q", out.Region)
	}
}

func TestNormalizeProbeObservation_TimeoutCoercion(t *testing.T) {
	raw := &model.ProbeObservation{
		ID:              uuid.New(),
		ProbeID:         uuid.New(),
		DependencyID:    uuid.New(),
		Region:          "us-east-1",
		Timestamp:       time.Now(),
		Source:          model.SourceSyntheticProbe,
		Success:         false,
		ErrorType:       model.ErrorTypeTimeout,
		TotalDurationMS: 30000,
	}

	out := NormalizeProbeObservation(raw)
	if !out.Timeout {
		t.Error("expected Timeout=true when ErrorType=timeout")
	}
}
