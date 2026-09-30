package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVendorMock_Scenarios(t *testing.T) {
	s := NewServer()
	// Set fast unit test delays
	s.normalDelay = 5 * time.Millisecond
	s.degradedDelay = 10 * time.Millisecond
	s.greyDelay = 15 * time.Millisecond
	s.timeoutDelay = 30 * time.Millisecond

	ts := httptest.NewServer(s.Routes())
	defer ts.Close()

	client := ts.Client()

	// 1. NORMAL scenario
	if err := s.SetScenario(ScenarioNormal); err != nil {
		t.Fatalf("failed to set scenario: %v", err)
	}
	resp, err := client.Post(ts.URL+"/v1/charges", "application/json", bytes.NewReader([]byte(`{"amount":1000}`)))
	if err != nil {
		t.Fatalf("POST /v1/charges failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var normalBody map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&normalBody)
	resp.Body.Close()
	if normalBody["status"] != "succeeded" {
		t.Errorf("expected status succeeded, got %v", normalBody["status"])
	}

	// 2. DEGRADED scenario
	_ = s.SetScenario(ScenarioDegraded)
	resp, err = client.Post(ts.URL+"/v1/charges", "application/json", nil)
	if err != nil {
		t.Fatalf("failed degraded: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 3. GREY_FAILURE scenario (HTTP 200 + successful charge)
	_ = s.SetScenario(ScenarioGreyFailure)
	resp, err = client.Post(ts.URL+"/v1/charges", "application/json", nil)
	if err != nil {
		t.Fatalf("failed grey failure: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 4. HTTP_500 scenario
	_ = s.SetScenario(ScenarioHTTP500)
	resp, err = client.Post(ts.URL+"/v1/charges", "application/json", nil)
	if err != nil {
		t.Fatalf("failed 500: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 5. HTTP_503 scenario
	_ = s.SetScenario(ScenarioHTTP503)
	resp, err = client.Post(ts.URL+"/v1/charges", "application/json", nil)
	if err != nil {
		t.Fatalf("failed 503: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 6. HTTP_429 scenario
	_ = s.SetScenario(ScenarioHTTP429)
	resp, err = client.Post(ts.URL+"/v1/charges", "application/json", nil)
	if err != nil {
		t.Fatalf("failed 429: %v", err)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 7. Status Page (Public status page declares operational even during failures)
	resp, err = client.Get(ts.URL + "/status")
	if err != nil {
		t.Fatalf("failed status page: %v", err)
	}
	var statusBody map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&statusBody)
	resp.Body.Close()
	stMap, ok := statusBody["status"].(map[string]any)
	if !ok || stMap["description"] != "All Systems Operational" {
		t.Errorf("expected All Systems Operational on status page, got %v", statusBody)
	}

	// 8. Scenario Control Endpoint (POST /admin/scenario)
	adminPayload := []byte(`{"scenario":"GREY_FAILURE"}`)
	resp, err = client.Post(ts.URL+"/admin/scenario", "application/json", bytes.NewReader(adminPayload))
	if err != nil {
		t.Fatalf("failed admin scenario: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from admin scenario, got %d", resp.StatusCode)
	}
	resp.Body.Close()
	if s.GetScenario() != ScenarioGreyFailure {
		t.Errorf("expected scenario GREY_FAILURE, got %s", s.GetScenario())
	}
}
