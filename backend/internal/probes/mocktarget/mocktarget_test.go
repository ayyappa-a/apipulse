package mocktarget_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apipulse/apipulse/internal/probes/mocktarget"
)

func TestMockTarget_Scenarios(t *testing.T) {
	// Configure with fast delays for unit tests
	target := mocktarget.New(
		mocktarget.WithNormalDelay(5*time.Millisecond),
		mocktarget.WithDegradedDelay(10*time.Millisecond),
		mocktarget.WithGreyFailureDelay(15*time.Millisecond),
	)
	ts := httptest.NewServer(target)
	defer ts.Close()

	client := ts.Client()

	tests := []struct {
		path       string
		wantCode   int
		wantStatus string
	}{
		{"/scenario/normal", http.StatusOK, "operational"},
		{"/scenario/degraded", http.StatusOK, "operational"},
		{"/scenario/grey-failure", http.StatusOK, "operational"},
		{"/scenario/500", http.StatusInternalServerError, ""},
		{"/scenario/503", http.StatusServiceUnavailable, ""},
		{"/scenario/429", http.StatusTooManyRequests, ""},
	}

	for _, tt := range tests {
		resp, err := client.Get(ts.URL + tt.path)
		if err != nil {
			t.Fatalf("request to %s failed: %v", tt.path, err)
		}
		if resp.StatusCode != tt.wantCode {
			t.Errorf("path %s: expected code %d, got %d", tt.path, tt.wantCode, resp.StatusCode)
		}

		if tt.wantStatus != "" {
			var body map[string]any
			_ = json.NewDecoder(resp.Body).Decode(&body)
			if body["status"] != tt.wantStatus {
				t.Errorf("path %s: expected status %s, got %v", tt.path, tt.wantStatus, body["status"])
			}
		}
		resp.Body.Close()
	}
}
