# APIPulse Demo Environment

This directory contains demonstration services designed to showcase APIPulse's dual-signal observability and grey-failure detection capabilities.

> **CRITICAL ARCHITECTURAL TRUTH:**
> - `CodeBegun` is **NOT** a real external production company or system with production access.
> - The CodeBegun service in this directory is a **self-hosted customer demonstration service** used to generate genuine OpenTelemetry telemetry for the hackathon.
> - The `vendor-mock` service is a **Stripe-like Payment Vendor Simulator**, not real Stripe.
> - **APIPulse is NOT a synchronous proxy.** Customer requests to the vendor NEVER traverse APIPulse. Telemetry flows asynchronously over OTLP/HTTP.

---

## Architecture

```text
                    CUSTOMER DEMO PATH

CodeBegun Demo Service
        │
        │ [1] Direct Outbound HTTP (POST /v1/charges)
        ▼
Stripe-like Vendor Simulator
        │
        │ response / latency / error
        ▼
CodeBegun Demo Service
        │
        │ [2] Asynchronous OpenTelemetry Spans
        ▼
APIPulse Telemetry Gateway (POST /v1/traces)
        │
        ▼
Canonical CUSTOMER_OTEL Observation (model.DependencyObservation)

────────────────────────────────────────────────────────────────────────

               INDEPENDENT SYNTHETIC PROBE PATH

APIPulse Synthetic Probe Runner
        │
        │ Outbound HTTP Probe (POST /v1/charges)
        ▼
Stripe-like Vendor Simulator
        │
        ▼
Canonical SYNTHETIC_PROBE Observation (model.ProbeObservation)
```

---

## Directory Structure

- **[`codebegun/`](./codebegun/README.md)**: Customer application demo. Instruments checkout payment requests with OpenTelemetry Go SDK (`go.opentelemetry.io/otel`).
- **[`vendor-mock/`](./vendor-mock/README.md)**: Stripe-like Payment Vendor Simulator. Simulates `NORMAL`, `DEGRADED`, `GREY_FAILURE`, `TIMEOUT`, `500`, `503`, and `429` scenarios.
- **`run_demo.go`**: Unified judge-friendly script that spins up the simulator, the CodeBegun client, and APIPulse, executing all 3 stages sequentially.

---

## Running the End-to-End Demo

From the `demo/` directory:

```bash
go run ./run_demo.go
```

This runs:
1. **Stage 1 (NORMAL)**: Customer and Probe both measure ~200ms latency, HTTP 200.
2. **Stage 2 (DEGRADED)**: Customer and Probe both measure ~2000ms latency, HTTP 200.
3. **Stage 3 (GREY FAILURE)**:
   - Vendor Public Status Page claims: `All Systems Operational`
   - Vendor returns: `HTTP 200 OK`
   - Customer (`CUSTOMER_OTEL`) experiences: `~8,000ms` latency
   - APIPulse (`SYNTHETIC_PROBE`) independently verifies: `~8,000ms` latency
   - Demonstrates APIPulse's core value: unannounced vendor degradation detection.
