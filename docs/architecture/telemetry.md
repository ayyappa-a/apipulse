# APIPulse Telemetry Architecture & Ingestion Model

## 1. High-Level Telemetry Topology

APIPulse is designed from first principles as an **out-of-band** observability and circuit recommendation platform.

> **CRITICAL DISTINCTION:**
> **APIPulse is NOT an inline proxy for CodeBegun production traffic.**
> **APIPulse receives telemetry asynchronously.**
> **APIPulse independently performs controlled synthetic monitoring.**
> **Real user/customer transactions NEVER synchronously traverse APIPulse.**

```
             ╔══════════════════════════════════════════╗
             ║           CUSTOMER SYSTEM                ║
             ╠══════════════════════════════════════════╣
             ║                                          ║
             ║   CodeBegun                              ║
             ║   │                                      ║
             ║   ├─── real request ──────────────────> Stripe
             ║   │         (production path,           ║
             ║   │          no APIPulse involvement)   ║
             ║   │                                      ║
             ║   └─── OpenTelemetry (async) ──────────────────────────────────┐
             ║                                          ║                      │
             ╚══════════════════════════════════════════╝                      │
                                                                               │
             ╔══════════════════════════════════════════╗                      │
             ║           APIPULSE SYSTEM                ║                      │
             ╠══════════════════════════════════════════╣                      │
             ║                                          ║◄─────────────────────┘
             ║   APIPulse Telemetry Gateway             ║
             ║   │                                      ║
             ║   ├── Health Intelligence Engine         ║
             ║   │   (correlates all 4 sources)         ║
             ║   │                                      ║
             ║   └── APIPulse regional probe ──────────────────────────────> Stripe
             ║         (independent controlled probe,   ║   (separate probe path,
             ║          not in CodeBegun request path)  ║    no customer data)
             ║                                          ║
             ║         Probe result ──────────────────> APIPulse
             ║                                          ║
             ╚══════════════════════════════════════════╝
```

**The full topology:**

```
   ┌─────────────────────────────────────────────────────────────┐
   │                    CodeBegun Application                    │
   │               (Production Customer Workload)                │
   └───────────────┬─────────────────────────────┬───────────────┘
                   │                             │
    [1] Production │              [2] Out-of-band│ OpenTelemetry
        Requests   │                  Telemetry  │ Spans & Metrics
                   ▼                             ▼
       ┌───────────────────────┐   ┌───────────────────────────┐
       │   Third-Party SaaS    │   │     APIPulse Ingestion    │
       │ (Stripe, SendGrid...) │   │     Telemetry Gateway     │
       └───────────┬───────────┘   └─────────────┬─────────────┘
                   │                             │
                   │                             ▼
                   │               ┌───────────────────────────┐
                   │               │    Correlation Engine     │
                   │               │   (p50, p95, p99 Tails)   │
                   │               └─────────────▲─────────────┘
                   │                             │
    [3] Distributed│ Outbound Edge               │ [4] Scraped Status
        Synthetic  │ Probes                      │     Pages
                   │                             │
       ┌───────────┴───────────┐   ┌─────────────┴─────────────┐
       │  APIPulse Edge Fleet  │   │   Vendor Status Scraper   │
       │  (8 Global Regions)   │   │  (Public RSS / StatusAPI) │
       └───────────────────────┘   └───────────────────────────┘
                                                 │
                                                 ▼
                                   ┌───────────────────────────┐
                                   │ Grey-Failure Intelligence │
                                   │ & Circuit Breaker Manager │
                                   └─────────────┬─────────────┘
                                                 │
                                                 ▼
                                   ┌───────────────────────────┐
                                   │  SRE Command Center / UI  │
                                   │   & Slack/PD Escalations  │
                                   └───────────────────────────┘
```

---

## 2. Decoupled Ingestion: Why Out-of-Band Matters

1. **Zero Added Latency:** Because APIPulse does not sit as a reverse proxy or inline API gateway, your mission-critical application transactions (e.g. processing a Stripe payment checkout) communicate directly with the third-party dependency.
2. **Zero Blast Radius:** If an APIPulse node or monitoring collector experiences downtime, your production traffic is completely unaffected.
3. **Telemetry Independence:** Telemetry spans are emitted asynchronously using standard OpenTelemetry batch span processors over non-blocking UDP/gRPC/HTTP channels.

---

## 3. The Four Distinct Telemetry Sources

APIPulse derives its intelligence by strictly distinguishing and correlating four distinct data sources. They are **never** visually or conceptually conflated.

### Source 1: Customer Production Telemetry (`CUSTOMER_OTEL`)
- **Origin:** OpenTelemetry SDKs embedded inside client applications (e.g. CodeBegun backend services).
- **Payload:** Client-observed HTTP client spans, actual request durations, payload sizes, caller region, and internal exceptions.
- **Value:** Represents the *actual user experience* under live production load.
- **Privacy:** No request/response bodies, no tokens, no PII. See Section 5.

### Source 2: Synthetic Probe Telemetry (`SYNTHETIC_PROBE`)
- **Origin:** APIPulse's autonomous edge fleet deployed across 8 global points of presence (`us-east-1`, `us-west-2`, `eu-west-1`, `eu-central-1`, `ap-south-1`, `ap-southeast-1`, `ap-northeast-1`, `sa-east-1`).
- **Payload:** High-frequency, deterministic HTTP/TLS probe waterfalls with microsecond precision:
  - DNS resolution time
  - TCP handshake time (SYN / SYN-ACK)
  - TLS 1.3 handshake negotiation
  - Time-To-First-Byte (TTFB)
  - Payload transfer duration
- **Value:** Baseline ground truth free from client application noise. Detects regional degradation even during zero-traffic hours.

### Source 3: Vendor Public Status (`VENDOR_STATUS`)
- **Origin:** Continuous scraping of vendor status pages (e.g. `status.stripe.com`, `status.sendgrid.com`, Atlassian Statuspage APIs, RSS feeds).
- **Payload:** Declared component status (`operational`, `degraded_performance`, `partial_outage`, `major_outage`, `maintenance`).
- **Value:** Establishes the vendor's self-reported claim.

### Source 4: APIPulse Computed Assessment (`APIPULSE_ASSESSMENT`)
- **Origin:** APIPulse Health & Grey-Failure Intelligence Engine.
- **Computation:** Correlates `CUSTOMER_OTEL` + `SYNTHETIC_PROBE` against `VENDOR_STATUS`.
- **Grey-Failure Detection Trigger:** When observed regional P95 tail latency exceeds baseline by ≥3.0× while the vendor's status page continues to declare "Operational", APIPulse marks the dependency as **Regional Grey Failure** and automatically arms or trips the circuit breaker to prevent cascading connection pool exhaustion.

---

## 4. Telemetry Ingestion Endpoints

### Development Endpoint (Phase 2 — Current)

```
POST /api/v1/telemetry/ingest
```

> ⚠️ **This is a DEVELOPMENT ingestion contract. It is NOT equivalent to production OTLP.**
> It accepts a simplified JSON body for pipeline testing and normalization validation.

### Production OTLP Endpoint (Phase 3 — Planned)

```
POST /api/v1/telemetry/otlp
```

> The production endpoint will implement the **OpenTelemetry Protocol (OTLP) over HTTP** using protobuf encoding, compatible with any OpenTelemetry SDK.

**Architectural intent:** The normalization, validation, and tagging pipeline implemented behind the dev endpoint is designed to be reused by the production OTLP receiver. The two endpoints share the same internal model — only the wire format differs.

### Probe Result Endpoint (Development)

```
POST /api/v1/probes/results
```

Accepts a `ProbeObservation` JSON body from an APIPulse probe agent.

---

## 5. Privacy & Security Boundary

APIPulse is an **observability system, not a payload inspection system**.

**APIPulse NEVER ingests or stores:**
- Request bodies or response bodies
- Authentication tokens, API keys, Bearer tokens
- Customer PII (names, emails, phone numbers, addresses)
- Payment information (card numbers, CVV, bank data)
- Authorization headers
- Arbitrary query parameters from customer requests
- Session cookies or session identifiers

**APIPulse DOES ingest (with bounded cardinality):**
- Timing measurements (duration in milliseconds)
- HTTP status codes (integer, not body)
- Normalized, cardinality-safe operation labels
- Region identifiers (known enum)
- Service names (registered at configuration time)
- Dependency names (registered at configuration time)
- Environment labels (production / staging / development)
- Optional trace IDs (never indexed, never used as a Prometheus label)

---

## 6. Storage Architecture

| Store | Role | Why |
|-------|------|-----|
| **PostgreSQL + TimescaleDB** | Primary telemetry storage | Time-series hypertables for probe_observations and dependency_observations. Efficient range queries for dashboard latency percentiles. |
| **Redis** | Hot state cache | Current health status per dependency. Rate limiting for probe agents. NOT used for historical data. |
| **Prometheus** | Operational metrics | APIPulse server health metrics. Bounded cardinality. Never stores customer-derived labels. |

---

## 7. Data Flow — Phase 2 State

```
[Phase 2 — Current]

Customer OTEL SDK  ─────► POST /api/v1/telemetry/ingest (dev JSON)
                               │
                               ▼
                        normalize.NormalizeDependencyObservation()
                               │
                               ▼
                        model.DependencyObservation.Validate()
                               │
                               ▼
                        metrics.TelemetryIngestedTotal.Inc()
                               │
                               ▼
                        [TODO Phase 3: INSERT INTO dependency_observations]

APIPulse Probe  ──────► POST /api/v1/probes/results (dev JSON)
                               │
                               ▼
                        normalize.NormalizeProbeObservation()
                               │
                               ▼
                        model.ProbeObservation.Validate()
                               │
                               ▼
                        [TODO Phase 3: INSERT INTO probe_observations]
```

---

### Architectural Invariant Notice

> **APIPulse is not a synchronous proxy and does not sit in the customer's production request path.**
> **The CodeBegun service in this repository is a self-hosted customer demonstration service used to generate genuine OpenTelemetry telemetry for the hackathon.**

---

## 8. Implementation Status

| Feature | Phase | Status |
|---------|-------|--------|
| Synthetic Probe Engine (`internal/probes/runner`) | Phase 3A | **COMPLETE** |
| Precision httptrace waterfall timing | Phase 3A | **COMPLETE** |
| Outbound rate limiting & SSRF validator | Phase 3A | **COMPLETE** |
| Production OTLP/HTTP trace receiver (`/v1/traces`) | Phase 3B | **COMPLETE** |
| CodeBegun Demo Customer Service (`demo/codebegun`) | Phase 3B | **COMPLETE** |
| Stripe-like Payment Vendor Simulator (`demo/vendor-mock`) | Phase 3B | **COMPLETE** |
| End-to-end dual-signal verification (`demo/run_demo.go`) | Phase 3B | **COMPLETE** |
| PostgreSQL/TimescaleDB concrete persistence | Phase 3C | Deferred (Noop active) |
| Redis hot state writes | Phase 3C | Planned |
| Vendor status page scraper | Phase 3C | Planned |
| Grey-failure correlation engine | Phase 4 | Planned |
| Slack / PagerDuty integration | Phase 4 | Planned |
| Circuit breaker recommendations | Phase 4 | Planned |
