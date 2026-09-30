// Stripe-like Payment Vendor Simulator — demo/vendor-mock/main.go
//
// ⚠️  DEMO SIMULATOR ONLY:
// This is NOT the real Stripe API. It is a self-hosted deterministic test harness
// for demonstrating APIPulse out-of-band monitoring and grey failure detection.
// No real credit cards or payments are ever processed.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Scenario represents the operating condition of the vendor simulator.
type Scenario string

const (
	ScenarioNormal      Scenario = "NORMAL"
	ScenarioDegraded    Scenario = "DEGRADED"
	ScenarioGreyFailure Scenario = "GREY_FAILURE"
	ScenarioTimeout     Scenario = "TIMEOUT"
	ScenarioHTTP500     Scenario = "HTTP_500"
	ScenarioHTTP503     Scenario = "HTTP_503"
	ScenarioHTTP429     Scenario = "HTTP_429"
)

// Server implements the mock payment gateway HTTP handler.
type Server struct {
	mu              sync.RWMutex
	scenario        Scenario
	normalDelay     time.Duration
	degradedDelay   time.Duration
	greyDelay       time.Duration
	timeoutDelay    time.Duration
}

// NewServer creates a new vendor simulator server.
func NewServer() *Server {
	return &Server{
		scenario:      ScenarioNormal,
		normalDelay:   200 * time.Millisecond,
		degradedDelay: 2000 * time.Millisecond,
		greyDelay:     8000 * time.Millisecond,
		timeoutDelay:  30 * time.Second,
	}
}

// SetScenario dynamically changes the active simulation scenario.
func (s *Server) SetScenario(sc Scenario) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch sc {
	case ScenarioNormal, ScenarioDegraded, ScenarioGreyFailure, ScenarioTimeout,
		ScenarioHTTP500, ScenarioHTTP503, ScenarioHTTP429:
		s.scenario = sc
		return nil
	default:
		return fmt.Errorf("invalid scenario: %s", sc)
	}
}

// GetScenario returns the currently active scenario.
func (s *Server) GetScenario() Scenario {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.scenario
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /v1/charges", s.handleCharges)
	mux.HandleFunc("GET /admin/scenario", s.handleGetScenario)
	mux.HandleFunc("POST /admin/scenario", s.handleSetScenario)
	mux.HandleFunc("GET /status", s.handleStatusPage)

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":   "healthy",
		"service":  "stripe-like-payment-simulator",
		"scenario": s.GetScenario(),
	})
}

func (s *Server) handleCharges(w http.ResponseWriter, r *http.Request) {
	// Consume and discard any request body up to 64KB (never retain card/customer data)
	_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 64*1024))
	_ = r.Body.Close()

	scenario := s.GetScenario()

	switch scenario {
	case ScenarioNormal:
		s.sleep(r, s.normalDelay)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":       fmt.Sprintf("ch_mock_%d", time.Now().UnixNano()),
			"status":   "succeeded",
			"paid":     true,
			"scenario": string(scenario),
		})

	case ScenarioDegraded:
		s.sleep(r, s.degradedDelay)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":       fmt.Sprintf("ch_mock_%d", time.Now().UnixNano()),
			"status":   "succeeded",
			"paid":     true,
			"scenario": string(scenario),
		})

	case ScenarioGreyFailure:
		// The APIPulse Crucial Problem Statement:
		// HTTP 200 returned, payment succeeds, but latency is ~8000ms
		s.sleep(r, s.greyDelay)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":       fmt.Sprintf("ch_mock_%d", time.Now().UnixNano()),
			"status":   "succeeded",
			"paid":     true,
			"scenario": string(scenario),
		})

	case ScenarioTimeout:
		s.sleep(r, s.timeoutDelay)
		w.WriteHeader(http.StatusOK)

	case ScenarioHTTP500:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"type":    "api_error",
				"message": "An internal error occurred with our payment processor",
			},
		})

	case ScenarioHTTP503:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"type":    "service_unavailable",
				"message": "Payment engine is temporarily overloaded",
			},
		})

	case ScenarioHTTP429:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"type":    "rate_limit_error",
				"message": "Too many requests to payment API",
			},
		})
	}
}

func (s *Server) handleGetScenario(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"scenario": string(s.GetScenario()),
	})
}

func (s *Server) handleSetScenario(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Scenario string `json:"scenario"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	sc := Scenario(strings.ToUpper(strings.TrimSpace(body.Scenario)))
	if err := s.SetScenario(sc); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("[vendor-mock] Active scenario set to: %s", sc)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":   "updated",
		"scenario": string(sc),
	})
}

func (s *Server) handleStatusPage(w http.ResponseWriter, r *http.Request) {
	// IMPORTANT: This status endpoint deliberately maps certain scenarios to OPERATIONAL
	// even when severe latency degradation is active. This is the core grey-failure
	// demonstration: the vendor's public status says "All Systems Operational" while
	// APIPulse independently observes ~8000ms latency (GREY_FAILURE scenario).
	scenario := s.GetScenario()

	// Map scenario to vendor-declared status and status_text.
	// These represent what a real vendor status page would report —
	// not what APIPulse's health engine computes.
	var status, statusText string
	switch scenario {
	case ScenarioNormal:
		status = "operational"
		statusText = "All Systems Operational"
	case ScenarioDegraded:
		// Intentionally OPERATIONAL — vendor hasn't noticed or hasn't acknowledged yet.
		status = "operational"
		statusText = "All Systems Operational"
	case ScenarioGreyFailure:
		// Critical demonstration: 8000ms latency, HTTP 200, but vendor says OPERATIONAL.
		status = "operational"
		statusText = "All Systems Operational"
	case ScenarioHTTP429:
		status = "operational"
		statusText = "All Systems Operational"
	case ScenarioHTTP500:
		status = "degraded_performance"
		statusText = "Degraded Performance — Payment Processing"
	case ScenarioHTTP503:
		status = "major_outage"
		statusText = "Major Outage — Payment Processing Unavailable"
	case ScenarioTimeout:
		status = "degraded_performance"
		statusText = "Investigating Connectivity Issues"
	default:
		status = "operational"
		statusText = "All Systems Operational"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"scenario":    strings.ToLower(string(scenario)),
		"status":      status,
		"status_text": statusText,
		"page": map[string]string{
			"id":         "mock-stripe-status",
			"name":       "Stripe API (Simulator)",
			"updated_at": time.Now().UTC().Format(time.RFC3339),
		},
		"components": []map[string]string{
			{
				"name":   "Payment Processing",
				"status": status,
			},
		},
	})
}

func (s *Server) sleep(r *http.Request, d time.Duration) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-r.Context().Done():
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9001"
	}

	server := NewServer()
	addr := ":" + port
	log.Printf("Stripe-like Payment Vendor Simulator running on %s", addr)
	log.Printf("Available endpoints: POST /v1/charges, GET /health, GET /status, POST /admin/scenario")

	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		log.Fatalf("vendor mock server failed: %v", err)
	}
}
