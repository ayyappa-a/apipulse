package model

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// validateDependencyObservation enforces the ingestion contract for customer OTEL data.
func validateDependencyObservation(o *DependencyObservation) error {
	var errs []string

	if o.ServiceName == "" {
		errs = append(errs, "service_name is required")
	}
	if o.DependencyName == "" {
		errs = append(errs, "dependency_name is required")
	}
	if o.Timestamp.IsZero() {
		errs = append(errs, "timestamp is required")
	}
	if o.DurationMS < 0 {
		errs = append(errs, "duration_ms must be >= 0")
	}
	if o.Source != SourceCustomerOTEL {
		errs = append(errs, fmt.Sprintf("source must be %s for DependencyObservation, got %s", SourceCustomerOTEL, o.Source))
	}
	validStatuses := map[ObservationStatus]bool{
		StatusOK: true, StatusError: true, StatusTimeout: true,
		StatusDegraded: true, StatusUnknown: true,
	}
	if !validStatuses[o.Status] {
		errs = append(errs, fmt.Sprintf("invalid status %q", o.Status))
	}
	if o.StatusCode < 0 || o.StatusCode > 599 {
		errs = append(errs, fmt.Sprintf("status_code %d is out of valid HTTP range", o.StatusCode))
	}
	if o.Environment == "" {
		errs = append(errs, "environment is required")
	}

	if len(errs) > 0 {
		return fmt.Errorf("DependencyObservation validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// validateProbeObservation enforces the ingestion contract for synthetic probe data.
func validateProbeObservation(o *ProbeObservation) error {
	var errs []string

	if o.ProbeID == (uuid.UUID{}) {
		errs = append(errs, "probe_id is required")
	}
	if o.DependencyID == (uuid.UUID{}) {
		errs = append(errs, "dependency_id is required")
	}
	if o.Region == "" {
		errs = append(errs, "region is required")
	}
	if o.Timestamp.IsZero() {
		errs = append(errs, "timestamp is required")
	}
	if o.Source != SourceSyntheticProbe {
		errs = append(errs, fmt.Sprintf("source must be %s for ProbeObservation, got %s", SourceSyntheticProbe, o.Source))
	}
	if o.TotalDurationMS < 0 {
		errs = append(errs, "total_duration_ms must be >= 0")
	}
	if !o.Success && o.ErrorType == ErrorTypeNone {
		errs = append(errs, "error_type is required when success=false")
	}

	if len(errs) > 0 {
		return fmt.Errorf("ProbeObservation validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}
