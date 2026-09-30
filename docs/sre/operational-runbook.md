# APIPulse SRE Operational Runbook

## 1. Local Compose Startup

To launch the complete continuous local reliability command center:

```bash
# Start all components: APIPulse, TimescaleDB, Redis, Prometheus, CodeBegun Demo, Vendor Simulator
docker compose --profile demo up --build
```

### Readiness Verification:
```bash
curl -i http://localhost:8080/ready
```
Expected output:
```json
{
  "status": "ready",
  "postgres": "ok",
  "redis": "ok",
  "health_engine": "ok"
}
```

## 2. Interactive Demo Walkthrough

### Scenario A: Normal Baseline (200ms)
```bash
# Set simulator to normal
curl -X POST http://localhost:9001/scenario -d '{"scenario":"NORMAL"}' -H "Content-Type: application/json"

# Trigger customer requests
curl http://localhost:8081/checkout
```
Result: `Assessment: HEALTHY`

### Scenario B: Grey-Failure Injection (8,000ms + HTTP 200 + Vendor Operational)
```bash
# Set simulator to grey-failure
curl -X POST http://localhost:9001/scenario -d '{"scenario":"GREY_FAILURE"}' -H "Content-Type: application/json"

# Trigger customer requests
curl http://localhost:8081/checkout
```
Wait 5 seconds for background scheduler tick.
Result:
- Vendor Status: `OPERATIONAL`
- Customer Latency: `~8,450ms` (`↑ 35×` baseline)
- Synthetic Latency: `~8,380ms`
- Assessment: `GREY_FAILURE`
- Incident: `OPEN` (`ARM_FALLBACK_OR_CIRCUIT`)

### Scenario C: Recovery with Hysteresis
```bash
# Restore simulator to normal
curl -X POST http://localhost:9001/scenario -d '{"scenario":"NORMAL"}' -H "Content-Type: application/json"
```
Wait through 3 evaluation ticks:
- `GREY_FAILURE` -> `RECOVERING` (Tick 1) -> `RECOVERING` (Tick 2) -> `HEALTHY` (Tick 3)
- Incident: `OPEN` -> `MONITORING` -> `RESOLVED`

## 3. Persistence & Restart Verification

1. While an incident is `OPEN`, restart the API server container:
```bash
docker compose restart api
```
2. Verify incident persistence:
```bash
curl http://localhost:8080/api/v1/incidents
```
The active incident remains `OPEN`, maintaining its exact started timestamp and historical events without creating duplicates.
