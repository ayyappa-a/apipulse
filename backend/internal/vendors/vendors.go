// Package vendors implements the VendorStatusProvider interface for APIPulse.
//
// IMPORTANT: This package does NOT build an unrestricted web scraper.
//
// For Phase 3C, the vendor status is provided by a DeterministicStatusAdapter
// that maps from the vendor simulator's scenario to a vendor-declared status.
//
// This is intentional: the grey-failure scenario must demonstrate that a vendor
// can report OPERATIONAL while causing severe latency degradation.
// The vendor status source represents the vendor's own declared state,
// not APIPulse's computed assessment.
//
// Status mapping:
//   NORMAL        → OPERATIONAL
//   DEGRADED      → OPERATIONAL
//   GREY_FAILURE  → OPERATIONAL  ← core demonstration case
//   HTTP_500      → DEGRADED
//   HTTP_503      → OUTAGE
//   HTTP_429      → OPERATIONAL
package vendors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/apipulse/apipulse/internal/telemetry/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// VendorStatusProvider interface
// ─────────────────────────────────────────────────────────────────────────────

// VendorStatusProvider is the abstraction for obtaining vendor-declared status.
// Implementations must not attempt to infer APIPulse's own assessment —
// they represent only what the vendor publicly declares.
type VendorStatusProvider interface {
	// GetStatus fetches the vendor's current declared status for a specific
	// dependency/component. Returns a VendorStatusRecord with Source=VENDOR_STATUS.
	GetStatus(ctx context.Context, vendorName, dependencyName, region string) (model.VendorStatusRecord, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// DeterministicStatusAdapter — vendor simulator adapter
// ─────────────────────────────────────────────────────────────────────────────

// DeterministicStatusAdapter queries the vendor-mock simulator's /status endpoint
// and maps its scenario to the canonical VendorStatus enumeration.
//
// The vendor simulator (demo/vendor-mock) deliberately reports OPERATIONAL
// for grey-failure scenarios — this is the core demonstration:
//   Vendor: OPERATIONAL, APIPulse observed: DEGRADED/GREY_FAILURE
type DeterministicStatusAdapter struct {
	// VendorStatusURL is the base URL of the vendor mock simulator.
	// Example: "http://localhost:8082"
	VendorStatusURL string
	client          *http.Client
}

// NewDeterministicAdapter creates a new status adapter for the vendor simulator.
func NewDeterministicAdapter(vendorStatusURL string) *DeterministicStatusAdapter {
	return &DeterministicStatusAdapter{
		VendorStatusURL: vendorStatusURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// vendorSimulatorStatus is the JSON shape returned by /status on the vendor mock.
type vendorSimulatorStatus struct {
	Scenario   string `json:"scenario"`
	Status     string `json:"status"`
	StatusText string `json:"status_text"`
}

// GetStatus queries the vendor simulator's /status endpoint and returns a
// canonical VendorStatusRecord.
func (a *DeterministicStatusAdapter) GetStatus(
	ctx context.Context,
	vendorName, dependencyName, region string,
) (model.VendorStatusRecord, error) {
	url := a.VendorStatusURL + "/status"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return failedVendorStatus(vendorName, dependencyName, region,
			fmt.Sprintf("failed to build request: %v", err))
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return failedVendorStatus(vendorName, dependencyName, region,
			fmt.Sprintf("status endpoint unreachable: %v", err))
	}
	defer resp.Body.Close()

	var simStatus vendorSimulatorStatus
	if err := json.NewDecoder(resp.Body).Decode(&simStatus); err != nil {
		return failedVendorStatus(vendorName, dependencyName, region,
			fmt.Sprintf("failed to decode vendor status: %v", err))
	}

	status := mapScenarioToVendorStatus(simStatus.Scenario, simStatus.Status)
	statusText := simStatus.StatusText
	if statusText == "" {
		statusText = simStatus.Status
	}

	return model.VendorStatusRecord{
		ID:             uuid.New(),
		VendorName:     vendorName,
		DependencyName: dependencyName,
		Region:         region,
		Timestamp:      time.Now().UTC(),
		Source:         model.SourceVendorStatus,
		Status:         status,
		StatusText:     statusText,
		IngestedAt:     time.Now().UTC(),
	}, nil
}

// mapScenarioToVendorStatus maps the vendor simulator's scenario/status to
// the canonical VendorStatus enumeration.
//
// The mapping is intentionally designed so that GREY_FAILURE scenarios
// still report OPERATIONAL — this is the core demonstration case.
func mapScenarioToVendorStatus(scenario, status string) model.VendorStatus {
	// Primary: use scenario if known.
	switch scenario {
	case "normal", "degraded", "grey_failure", "rate_limited":
		// These scenarios represent cases where the vendor's status page
		// shows OPERATIONAL despite actual degradation (especially grey_failure).
		return model.VendorStatusOperational
	case "error", "http_500":
		return model.VendorStatusDegraded
	case "http_503", "outage":
		return model.VendorStatusOutage
	}

	// Fallback: use status field.
	switch status {
	case "operational":
		return model.VendorStatusOperational
	case "degraded_performance":
		return model.VendorStatusDegraded
	case "partial_outage":
		return model.VendorStatusPartialOutage
	case "major_outage":
		return model.VendorStatusOutage
	default:
		return model.VendorStatusUnknown
	}
}

func failedVendorStatus(vendor, dep, region, msg string) (model.VendorStatusRecord, error) {
	return model.VendorStatusRecord{
		ID:             uuid.New(),
		VendorName:     vendor,
		DependencyName: dep,
		Region:         region,
		Timestamp:      time.Now().UTC(),
		Source:         model.SourceVendorStatus,
		Status:         model.VendorStatusUnknown,
		StatusText:     msg,
		IngestedAt:     time.Now().UTC(),
	}, fmt.Errorf("vendor status unavailable: %s", msg)
}

// ─────────────────────────────────────────────────────────────────────────────
// StaticStatusAdapter — deterministic test/demo adapter
// ─────────────────────────────────────────────────────────────────────────────

// StaticStatusAdapter returns a fixed vendor status regardless of any external call.
// Used in unit tests where the vendor simulator is not running.
type StaticStatusAdapter struct {
	Status     model.VendorStatus
	StatusText string
}

// GetStatus always returns the configured static status.
func (a *StaticStatusAdapter) GetStatus(
	_ context.Context,
	vendorName, dependencyName, region string,
) (model.VendorStatusRecord, error) {
	return model.VendorStatusRecord{
		ID:             uuid.New(),
		VendorName:     vendorName,
		DependencyName: dependencyName,
		Region:         region,
		Timestamp:      time.Now().UTC(),
		Source:         model.SourceVendorStatus,
		Status:         a.Status,
		StatusText:     a.StatusText,
		IngestedAt:     time.Now().UTC(),
	}, nil
}
