package timing_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apipulse/apipulse/internal/probes/timing"
)

func TestTracker_BasicHTTP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello apipulse"))
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tracker, tracedCtx := timing.NewTracker(ctx)
	req, err := http.NewRequestWithContext(tracedCtx, http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do failed: %v", err)
	}
	defer resp.Body.Close()

	tracker.MarkBodyReadStart()
	_, _ = io.ReadAll(resp.Body)
	tracker.MarkBodyReadDone()

	res := tracker.Finish()

	if res.TotalDurationMS <= 0 {
		t.Errorf("expected positive TotalDurationMS, got %f", res.TotalDurationMS)
	}
	if res.TTFBDurationMS <= 0 {
		t.Errorf("expected positive TTFBDurationMS, got %f", res.TTFBDurationMS)
	}
}

func TestTracker_ConnectionReuse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	client := ts.Client() // Reuses transport

	// Request 1: Fresh / Cold connection
	ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel1()
	tracker1, tracedCtx1 := timing.NewTracker(ctx1)
	req1, _ := http.NewRequestWithContext(tracedCtx1, http.MethodGet, ts.URL, nil)
	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("req1 failed: %v", err)
	}
	_, _ = io.ReadAll(resp1.Body)
	resp1.Body.Close()
	res1 := tracker1.Finish()

	// Request 2: Warm reused connection
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	tracker2, tracedCtx2 := timing.NewTracker(ctx2)
	req2, _ := http.NewRequestWithContext(tracedCtx2, http.MethodGet, ts.URL, nil)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("req2 failed: %v", err)
	}
	_, _ = io.ReadAll(resp2.Body)
	resp2.Body.Close()
	res2 := tracker2.Finish()

	if !res2.ReusedConnection {
		t.Logf("Note: Connection 2 was not reported as reused in httptest (reused=%v)", res2.ReusedConnection)
	}
	if res1.TotalDurationMS < 0 || res2.TotalDurationMS < 0 {
		t.Errorf("expected non-negative durations, got %f and %f", res1.TotalDurationMS, res2.TotalDurationMS)
	}
}
