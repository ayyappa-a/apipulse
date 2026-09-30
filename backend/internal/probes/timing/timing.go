// Package timing provides HTTP request timing instrumentation for synthetic probes
// using net/http/httptrace.
//
// It captures detailed waterfall metrics:
//   - DNS lookup duration
//   - TCP connection duration
//   - TLS handshake duration
//   - Time-to-First-Byte (TTFB)
//   - Response body transfer duration
//   - Total end-to-end wall-clock duration
//   - Explicit connection reuse tracking (warm vs cold)
package timing

import (
	"context"
	"crypto/tls"
	"net/http/httptrace"
	"net/textproto"
	"sync"
	"time"
)

// Result holds the complete millisecond timing breakdown for a synthetic probe.
type Result struct {
	// DNSDurationMS is the time taken to resolve the hostname.
	// Will be 0.0 if the connection was reused from a pool.
	DNSDurationMS float64 `json:"dns_duration_ms"`

	// TCPDurationMS is the time taken to establish the TCP connection.
	// Will be 0.0 if the connection was reused.
	TCPDurationMS float64 `json:"tcp_duration_ms"`

	// TLSDurationMS is the time taken for the TLS handshake.
	// Will be 0.0 if the connection was reused or if plain HTTP.
	TLSDurationMS float64 `json:"tls_duration_ms"`

	// TTFBDurationMS is the time from request write finish until the first response byte arrives.
	TTFBDurationMS float64 `json:"ttfb_duration_ms"`

	// TransferDurationMS is the time taken to stream and consume the response body.
	TransferDurationMS float64 `json:"transfer_duration_ms"`

	// TotalDurationMS is the total wall-clock duration of the probe execution.
	TotalDurationMS float64 `json:"total_duration_ms"`

	// ReusedConnection is true when an existing keep-alive connection was used.
	// When true, DNS/TCP/TLS durations being 0ms accurately reflects reuse,
	// NOT zero-cost network operations.
	ReusedConnection bool `json:"reused_connection"`

	// WasIdle indicates whether the reused connection had been idle in the pool.
	WasIdle bool `json:"was_idle"`

	// ConnectedAddr is the remote IP:port connected to.
	ConnectedAddr string `json:"connected_addr,omitempty"`
}

// Tracker instruments an HTTP request context with httptrace hooks.
type Tracker struct {
	mu sync.Mutex

	start time.Time

	dnsStart time.Time
	dnsDone  time.Time

	connStart time.Time
	connDone  time.Time

	tlsStart time.Time
	tlsDone  time.Time

	reqWrote time.Time
	gotByte  time.Time

	bodyStart time.Time
	bodyDone  time.Time

	reused        bool
	wasIdle       bool
	connectedAddr string
}

// NewTracker initiates a timing tracker and returns the enriched context.
func NewTracker(ctx context.Context) (*Tracker, context.Context) {
	t := &Tracker{
		start: time.Now(),
	}

	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			t.mu.Lock()
			t.dnsStart = time.Now()
			t.mu.Unlock()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			t.mu.Lock()
			t.dnsDone = time.Now()
			t.mu.Unlock()
		},
		ConnectStart: func(network, addr string) {
			t.mu.Lock()
			t.connStart = time.Now()
			t.connectedAddr = addr
			t.mu.Unlock()
		},
		ConnectDone: func(network, addr string, err error) {
			t.mu.Lock()
			t.connDone = time.Now()
			t.mu.Unlock()
		},
		TLSHandshakeStart: func() {
			t.mu.Lock()
			t.tlsStart = time.Now()
			t.mu.Unlock()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			t.mu.Lock()
			t.tlsDone = time.Now()
			t.mu.Unlock()
		},
		GotConn: func(info httptrace.GotConnInfo) {
			t.mu.Lock()
			t.reused = info.Reused
			t.wasIdle = info.WasIdle
			if info.Conn != nil && info.Conn.RemoteAddr() != nil {
				t.connectedAddr = info.Conn.RemoteAddr().String()
			}
			t.mu.Unlock()
		},
		WroteHeaders: func() {
			t.mu.Lock()
			t.reqWrote = time.Now()
			t.mu.Unlock()
		},
		GotFirstResponseByte: func() {
			t.mu.Lock()
			t.gotByte = time.Now()
			t.mu.Unlock()
		},
	}

	return t, httptrace.WithClientTrace(ctx, trace)
}

// MarkBodyReadStart records the beginning of response body consumption.
func (t *Tracker) MarkBodyReadStart() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bodyStart = time.Now()
}

// MarkBodyReadDone records the end of response body consumption.
func (t *Tracker) MarkBodyReadDone() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bodyDone = time.Now()
}

// Finish computes the timing breakdown in milliseconds.
func (t *Tracker) Finish() Result {
	t.mu.Lock()
	defer t.mu.Unlock()

	total := time.Since(t.start)

	var dnsMS, tcpMS, tlsMS, ttfbMS, transferMS float64

	if !t.dnsStart.IsZero() && !t.dnsDone.IsZero() {
		dnsMS = durationMS(t.dnsDone.Sub(t.dnsStart))
	}

	if !t.connStart.IsZero() && !t.connDone.IsZero() {
		tcpMS = durationMS(t.connDone.Sub(t.connStart))
	}

	if !t.tlsStart.IsZero() && !t.tlsDone.IsZero() {
		tlsMS = durationMS(t.tlsDone.Sub(t.tlsStart))
	}

	// TTFB: from request header write (or start if headers wasn't captured) to first byte
	if !t.gotByte.IsZero() {
		ref := t.reqWrote
		if ref.IsZero() {
			ref = t.start
		}
		if t.gotByte.After(ref) {
			ttfbMS = durationMS(t.gotByte.Sub(ref))
		}
	}

	// Transfer: from bodyStart (or gotByte) to bodyDone
	if !t.bodyDone.IsZero() {
		ref := t.bodyStart
		if ref.IsZero() {
			ref = t.gotByte
		}
		if !ref.IsZero() && t.bodyDone.After(ref) {
			transferMS = durationMS(t.bodyDone.Sub(ref))
		}
	}

	return Result{
		DNSDurationMS:      dnsMS,
		TCPDurationMS:      tcpMS,
		TLSDurationMS:      tlsMS,
		TTFBDurationMS:     ttfbMS,
		TransferDurationMS: transferMS,
		TotalDurationMS:    durationMS(total),
		ReusedConnection:   t.reused,
		WasIdle:            t.wasIdle,
		ConnectedAddr:      t.connectedAddr,
	}
}

// WroteHeaderNoop is for satisfying trace hooks if needed.
func (t *Tracker) WroteHeaderField(key string, values []string) {}
func (t *Tracker) Wait100Continue()                             {}
func (t *Tracker) Got100Continue()                              {}
func (t *Tracker) Got1xxResponse(code int, header textproto.MIMEHeader) error {
	return nil
}

func durationMS(d time.Duration) float64 {
	if d <= 0 {
		return 0.0
	}
	return float64(d.Nanoseconds()) / 1e6
}
