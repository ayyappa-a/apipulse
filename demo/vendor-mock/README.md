# Stripe-like Payment Vendor Simulator

## Overview

A deterministic local HTTP service simulating an external payment processor (such as Stripe).

> **IMPORTANT:**
> This service is strictly a demonstration harness. It does **NOT** connect to real payment networks, process transactions, or use real Stripe credentials.

---

## Simulated Scenarios

The simulator supports 7 deterministic operational scenarios:

| Scenario | Latency | HTTP Status | Response Payload | Meaning |
|:---|:---|:---|:---|:---|
| `NORMAL` | ~200ms | `200 OK` | `{"status":"succeeded"}` | Normal operating condition |
| `DEGRADED` | ~2,000ms | `200 OK` | `{"status":"succeeded"}` | Moderate vendor latency spike |
| `GREY_FAILURE` | ~8,000ms | `200 OK` | `{"status":"succeeded"}` | Severe latency degradation without error codes |
| `TIMEOUT` | 30s | None | Connection hangs | Vendor hangs beyond client deadline |
| `HTTP_500` | Fast (<5ms) | `500 Error` | `{"error":{"type":"api_error"}}` | Unhandled upstream crash |
| `HTTP_503` | Fast (<5ms) | `503 Service Unavailable` | `{"error":{"type":"capacity_error"}}` | Vendor capacity overload |
| `HTTP_429` | Fast (<5ms) | `429 Too Many Requests` | `{"error":{"type":"rate_limit_error"}}` | Upstream rate limit exceeded |

---

## Endpoints

- `POST /v1/charges`: Payment charging endpoint.
- `GET /health`: Health and active scenario check.
- `GET /status`: Simulates the public vendor status page (`All Systems Operational`).
- `GET /admin/scenario`: Returns active scenario.
- `POST /admin/scenario`: Changes active scenario. Example payload:
  ```json
  {"scenario": "GREY_FAILURE"}
  ```

---

## Running Locally

```bash
go run ./demo/vendor-mock/main.go
```
