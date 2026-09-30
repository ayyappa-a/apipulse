package runner

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/apipulse/apipulse/internal/probes/timing"
	"github.com/apipulse/apipulse/internal/telemetry/model"
)

// ProbeMode distinguishes warm keep-alive probing from cold diagnostic probing.
type ProbeMode string

const (
	ProbeModeWarm           ProbeMode = "WARM"
	ProbeModeColdDiagnostic ProbeMode = "COLD_DIAGNOSTIC"
)

// ValidationConfig defines assertion rules for the response.
type ValidationConfig struct {
	// ExpectedStatusCodes is a slice of acceptable HTTP status codes (e.g. 200, 204).
	// If empty, defaults to 200-299.
	ExpectedStatusCodes []int `json:"expected_status_codes,omitempty"`

	// ExpectedContentType is a substring that must match the Content-Type header (e.g. "application/json").
	ExpectedContentType string `json:"expected_content_type,omitempty"`

	// MaxSizeBytes is the maximum allowed response body size (default 64KB, max 1MB).
	MaxSizeBytes int64 `json:"max_size_bytes,omitempty"`
}

// ProbeConfig contains the full specification for a synthetic probe.
type ProbeConfig struct {
	// ID is the unique identifier for this probe configuration.
	ID uuid.UUID `json:"id"`

	// DependencyID identifies the external vendor dependency being probed.
	DependencyID uuid.UUID `json:"dependency_id"`

	// Name is a human-readable label for this probe (e.g. "Stripe API Health").
	Name string `json:"name"`

	// Endpoint is the target URL. Must be http or https.
	Endpoint string `json:"endpoint"`

	// Method must be safe (GET, HEAD). POST is only allowed if AllowPOST=true.
	Method string `json:"method"`

	// Region is the probe origin region (e.g. "us-east-1", "eu-west-1", "ap-south-1").
	Region string `json:"region"`

	// Interval is the duration between scheduled executions.
	Interval time.Duration `json:"interval"`

	// Timeout is the hard timeout for each individual probe execution.
	Timeout time.Duration `json:"timeout"`

	// Enabled controls whether the probe is actively scheduled.
	Enabled bool `json:"enabled"`

	// Mode controls connection reuse (WARM vs COLD_DIAGNOSTIC).
	Mode ProbeMode `json:"mode"`

	// Validation specifies expectations on the response.
	Validation ValidationConfig `json:"validation"`

	// AllowPOST must be explicitly set to true if Method is POST.
	AllowPOST bool `json:"allow_post"`
}

// Validate checks the probe configuration for safety and required fields.
func (c *ProbeConfig) Validate() error {
	if c.ID == (uuid.UUID{}) {
		return fmt.Errorf("probe id is required")
	}
	if c.DependencyID == (uuid.UUID{}) {
		return fmt.Errorf("dependency_id is required")
	}
	if strings.TrimSpace(c.Endpoint) == "" {
		return fmt.Errorf("endpoint is required")
	}

	u, err := url.Parse(c.Endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("invalid endpoint URL (must be http or https): %s", c.Endpoint)
	}

	method := strings.ToUpper(strings.TrimSpace(c.Method))
	if method == "" {
		method = "GET"
		c.Method = method
	}
	switch method {
	case "GET", "HEAD":
		// Safe methods
	case "POST":
		if !c.AllowPOST {
			return fmt.Errorf("POST probe rejected: AllowPOST must be explicitly enabled for idempotent test endpoints")
		}
	default:
		return fmt.Errorf("unsafe HTTP method %s: only GET, HEAD, or explicitly authorized POST are permitted", method)
	}

	if c.Region == "" {
		c.Region = "local"
	}
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	if c.Timeout > 60*time.Second {
		return fmt.Errorf("probe timeout %v exceeds maximum allowable limit of 60s", c.Timeout)
	}
	if c.Mode == "" {
		c.Mode = ProbeModeWarm
	}
	if c.Validation.MaxSizeBytes <= 0 {
		c.Validation.MaxSizeBytes = 64 * 1024 // 64 KB default
	}

	return nil
}

// ExecutionResult holds the internal output of a single probe run before normalization.
type ExecutionResult struct {
	ProbeID                  uuid.UUID
	DependencyID             uuid.UUID
	Region                   string
	Timestamp                time.Time
	Success                  bool
	HTTPStatusCode           int
	Timing                   timing.Result
	ErrorType                model.ErrorType
	NetworkError             string
	Timeout                  bool
	ResponseValidationResult string
	BytesRead                int64
}
