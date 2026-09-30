package runner

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// ClientPool manages hardened HTTP clients for synthetic probing.
type ClientPool struct {
	warmClient *http.Client
	coldClient *http.Client
}

// ClientConfig specifies HTTP connection bounds.
type ClientConfig struct {
	DialTimeout           time.Duration
	TLSHandshakeTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
	IdleConnTimeout       time.Duration
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	MaxConnsPerHost       int
}

// DefaultClientConfig returns production-hardened connection limits.
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		DialTimeout:           5 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       20,
	}
}

// NewClientPool instantiates hardened warm and cold HTTP clients.
func NewClientPool(cfg ClientConfig) *ClientPool {
	dialer := &net.Dialer{
		Timeout:   cfg.DialTimeout,
		KeepAlive: 30 * time.Second,
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Warm transport enables connection pooling and reuse
	warmTransport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:       cfg.MaxConnsPerHost,
		DisableKeepAlives:     false,
		ForceAttemptHTTP2:     true,
	}

	// Cold transport explicitly disables keep-alives to force fresh DNS/TCP/TLS on each probe
	coldTransport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
		DisableKeepAlives:     true, // Forces fresh connection
		MaxConnsPerHost:       cfg.MaxConnsPerHost,
		ForceAttemptHTTP2:     false,
	}

	return &ClientPool{
		warmClient: &http.Client{
			Transport: warmTransport,
			// Timeout is managed per-request via context.WithTimeout
		},
		coldClient: &http.Client{
			Transport: coldTransport,
		},
	}
}

// GetClient returns the appropriate hardened HTTP client for the given probe mode.
func (p *ClientPool) GetClient(mode ProbeMode) *http.Client {
	if mode == ProbeModeColdDiagnostic {
		return p.coldClient
	}
	return p.warmClient
}
