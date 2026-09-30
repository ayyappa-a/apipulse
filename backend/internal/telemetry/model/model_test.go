package model

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func validDepObs() *DependencyObservation {
	return &DependencyObservation{
		ID:             uuid.New(),
		Timestamp:      time.Now().UTC(),
		ServiceName:    "codebegun-checkout",
		DependencyName: "stripe",
		Operation:      "POST /v1/charges",
		Region:         "us-east-1",
		Source:         SourceCustomerOTEL,
		DurationMS:     245.5,
		Status:         StatusOK,
		StatusCode:     200,
		ErrorType:      ErrorTypeNone,
		Environment:    "production",
		IngestedAt:     time.Now().UTC(),
	}
}

func validProbeObs() *ProbeObservation {
	return &ProbeObservation{
		ID:               uuid.New(),
		ProbeID:          uuid.New(),
		DependencyID:     uuid.New(),
		Region:           "eu-west-1",
		Timestamp:        time.Now().UTC(),
		Source:           SourceSyntheticProbe,
		Success:          true,
		HTTPStatusCode:   200,
		DNSDurationMS:    12.3,
		TCPDurationMS:    22.1,
		TLSDurationMS:    55.4,
		TTFBDurationMS:   110.0,
		TransferDurationMS: 8.5,
		TotalDurationMS:  208.3,
		ErrorType:        ErrorTypeNone,
		IngestedAt:       time.Now().UTC(),
	}
}

// --- DependencyObservation tests ---

func TestDependencyObservation_ValidObs(t *testing.T) {
	if err := validDepObs().Validate(); err != nil {
		t.Fatalf("expected valid observation, got error: %v", err)
	}
}

func TestDependencyObservation_MissingServiceName(t *testing.T) {
	o := validDepObs()
	o.ServiceName = ""
	if err := o.Validate(); err == nil {
		t.Fatal("expected validation error for missing service_name")
	}
}

func TestDependencyObservation_MissingDependencyName(t *testing.T) {
	o := validDepObs()
	o.DependencyName = ""
	if err := o.Validate(); err == nil {
		t.Fatal("expected validation error for missing dependency_name")
	}
}

func TestDependencyObservation_ZeroTimestamp(t *testing.T) {
	o := validDepObs()
	o.Timestamp = time.Time{}
	if err := o.Validate(); err == nil {
		t.Fatal("expected validation error for zero timestamp")
	}
}

func TestDependencyObservation_NegativeDuration(t *testing.T) {
	o := validDepObs()
	o.DurationMS = -1.0
	if err := o.Validate(); err == nil {
		t.Fatal("expected validation error for negative duration_ms")
	}
}

func TestDependencyObservation_WrongSource(t *testing.T) {
	o := validDepObs()
	o.Source = SourceSyntheticProbe
	if err := o.Validate(); err == nil {
		t.Fatal("expected validation error for wrong source type")
	}
}

func TestDependencyObservation_InvalidStatus(t *testing.T) {
	o := validDepObs()
	o.Status = "flying"
	if err := o.Validate(); err == nil {
		t.Fatal("expected validation error for invalid status")
	}
}

func TestDependencyObservation_InvalidStatusCode(t *testing.T) {
	o := validDepObs()
	o.StatusCode = 700
	if err := o.Validate(); err == nil {
		t.Fatal("expected validation error for out-of-range status code")
	}
}

func TestDependencyObservation_MissingEnvironment(t *testing.T) {
	o := validDepObs()
	o.Environment = ""
	if err := o.Validate(); err == nil {
		t.Fatal("expected validation error for missing environment")
	}
}

// --- ProbeObservation tests ---

func TestProbeObservation_Valid(t *testing.T) {
	if err := validProbeObs().Validate(); err != nil {
		t.Fatalf("expected valid probe observation, got error: %v", err)
	}
}

func TestProbeObservation_MissingProbeID(t *testing.T) {
	o := validProbeObs()
	o.ProbeID = uuid.UUID{}
	if err := o.Validate(); err == nil {
		t.Fatal("expected validation error for zero probe_id")
	}
}

func TestProbeObservation_WrongSource(t *testing.T) {
	o := validProbeObs()
	o.Source = SourceCustomerOTEL
	if err := o.Validate(); err == nil {
		t.Fatal("expected validation error for wrong source type")
	}
}

func TestProbeObservation_FailureWithNoErrorType(t *testing.T) {
	o := validProbeObs()
	o.Success = false
	o.ErrorType = ErrorTypeNone
	if err := o.Validate(); err == nil {
		t.Fatal("expected validation error for success=false with no error_type")
	}
}

func TestProbeObservation_FailureWithErrorType(t *testing.T) {
	o := validProbeObs()
	o.Success = false
	o.ErrorType = ErrorTypeTimeout
	o.Timeout = true
	if err := o.Validate(); err != nil {
		t.Fatalf("expected valid failing probe observation, got error: %v", err)
	}
}

func TestProbeObservation_NegativeDuration(t *testing.T) {
	o := validProbeObs()
	o.TotalDurationMS = -10.0
	if err := o.Validate(); err == nil {
		t.Fatal("expected validation error for negative total_duration_ms")
	}
}

// --- TelemetrySource constant tests ---

func TestTelemetrySources_AreDistinct(t *testing.T) {
	sources := []TelemetrySource{
		SourceCustomerOTEL,
		SourceSyntheticProbe,
		SourceVendorStatus,
		SourceAPIPulseAssessment,
	}
	seen := map[TelemetrySource]bool{}
	for _, s := range sources {
		if seen[s] {
			t.Errorf("duplicate TelemetrySource value: %s", s)
		}
		seen[s] = true
	}
}
