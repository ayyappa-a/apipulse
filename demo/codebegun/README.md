# CodeBegun Demo Customer Service

## Overview

The `codebegun-demo` service represents a customer application (e.g. e-commerce checkout) that relies on third-party payment gateways.

> **IMPORTANT:**
> CodeBegun is a self-hosted customer demonstration service used to generate genuine OpenTelemetry spans. It is **NOT** a real production system.

---

## Out-of-Band Architecture

```text
Client Request
      │
      ▼
POST /api/payment
      │
      ├──[Direct Outbound Call]──► Stripe-like Vendor Simulator (:9001/v1/charges)
      │
      └──[Asynchronous Spans]───► APIPulse OTLP Receiver (:8080/v1/traces)
```

APIPulse is **never** an inline proxy. If APIPulse is down or unreachable, customer payment transactions continue to execute without interruption.

---

## Endpoints

- `GET /health`: Health check and configured region reporting.
- `POST /api/payment`: Executes a checkout payment against the vendor simulator. Creates an OpenTelemetry client span and exports it to APIPulse.
- `GET /api/demo/scenario`: Inspects active scenario on the vendor simulator.
- `POST /api/demo/scenario`: Sets the scenario on the vendor simulator.

---

## Telemetry & Privacy Contract

The service uses the official OpenTelemetry Go SDK (`go.opentelemetry.io/otel`).

### Emitted Span Attributes:
- `service.name`: `"codebegun-demo"`
- `deployment.environment.name`: `"demo"`
- `dependency.name`: `"stripe-like"`
- `dependency.type`: `"payment"`
- `http.request.method`: `"POST"`
- `http.response.status_code`: `200 | 500 | 503 | ...`
- `server.address`: Hostname of the vendor
- `url.path`: `"/v1/charges"`
- `cloud.region`: Logical region identity (default `"ap-south-1"`)

### Data Privacy Contract:
- **No credit card numbers**
- **No authorization tokens / API keys**
- **No cookies**
- **No request or response bodies**
- **No PII**

---

## Running Locally

```bash
# Terminal 1: Start Vendor Mock
go run ./demo/vendor-mock/main.go

# Terminal 2: Start CodeBegun Demo
go run ./demo/codebegun/main.go
```
