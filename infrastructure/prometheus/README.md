# Prometheus Monitoring Configs

Includes:
- `prometheus.yml`: Scrapes APIPulse `/metrics` endpoint every 15s.
- `alerts.yml`: Out-of-band alert rules (e.g. `ThirdPartyGreyFailureDetected`, `CircuitBreakerTripped`).
