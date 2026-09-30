// Package runner provides SSRF validation and mitigation for outbound synthetic probes.
//
// SSRF SECURITY CONTRACT & PRODUCTION ARCHITECTURE:
//
// In multi-tenant cloud environments, outbound HTTP probes create Server-Side
// Request Forgery (SSRF) vulnerabilities if untrusted endpoints are probed:
//   1. Cloud Metadata Service exploitation (e.g. 169.254.169.254 / AWS IMDSv1)
//   2. Internal VPC network scanning (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
//   3. Localhost loopback service probing (127.0.0.1, ::1)
//   4. DNS rebinding attacks (host resolves to public IP on check, private IP on connect)
//
// In Production, APIPulse requires:
//   - Target domain allowlists (only verified vendor domains)
//   - Dedicated isolated egress gateways with strict iptables/firewall rules
//   - Custom net.Dialer with IP pinning & post-resolution IP validation (preventing DNS rebinding)
//   - Cloud metadata blocking at the host network layer
//
// In Local Development & CI:
//   - Local mock servers (127.0.0.1, localhost) must be explicitly permitted
//     via AllowLocalTargets: true.
package runner

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// SSRFValidator inspects destination URLs before outbound probe connection.
type SSRFValidator struct {
	// AllowLocalTargets permits 127.0.0.1 / localhost for deterministic testing.
	// MUST be false in production environments.
	AllowLocalTargets bool

	// AllowedDomains specifies an optional explicit allowlist of domain suffixes (e.g. "stripe.com").
	AllowedDomains []string
}

// NewSSRFValidator creates an SSRF validator with the specified policy.
func NewSSRFValidator(allowLocal bool, allowedDomains ...string) *SSRFValidator {
	return &SSRFValidator{
		AllowLocalTargets: allowLocal,
		AllowedDomains:    allowedDomains,
	}
}

// ValidateTarget parses and checks the target URL against SSRF policy rules.
func (v *SSRFValidator) ValidateTarget(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("ssrf validation: invalid target URL: %w", err)
	}

	hostname := u.Hostname()
	if hostname == "" {
		return fmt.Errorf("ssrf validation: missing hostname in target URL")
	}

	// 1. Check cloud metadata IP explicitly
	if hostname == "169.254.169.254" || hostname == "metadata.google.internal" {
		return fmt.Errorf("ssrf violation: probe to cloud metadata service blocked")
	}

	// 2. Check localhost / loopback
	isLoopback := hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1"
	if isLoopback {
		if v.AllowLocalTargets {
			return nil // Permitted in local test/dev mode
		}
		return fmt.Errorf("ssrf violation: loopback address %s blocked in production mode", hostname)
	}

	// 3. Resolve IP to check for private / internal CIDRs
	ips, err := net.LookupIP(hostname)
	if err == nil {
		for _, ip := range ips {
			if ip.IsLoopback() {
				if !v.AllowLocalTargets {
					return fmt.Errorf("ssrf violation: hostname %s resolves to loopback IP %s", hostname, ip)
				}
			}
			if ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				if !v.AllowLocalTargets {
					return fmt.Errorf("ssrf violation: hostname %s resolves to private/internal IP %s", hostname, ip)
				}
			}
		}
	}

	// 4. Check domain allowlist if configured
	if len(v.AllowedDomains) > 0 {
		matched := false
		for _, d := range v.AllowedDomains {
			if strings.EqualFold(hostname, d) || strings.HasSuffix(strings.ToLower(hostname), "."+strings.ToLower(d)) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("ssrf violation: domain %s is not in configured allowlist", hostname)
		}
	}

	return nil
}
