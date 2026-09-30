# OpenTelemetry (OTel) Collector

Defines receiver pipelines for:
- OTLP gRPC/HTTP ingress from application clients (e.g. CodeBegun backend)
- Processors: batch, memory_limiter, resource, filter
- Exporters: Prometheus metrics endpoint, TimescaleDB, and APIPulse health engine
