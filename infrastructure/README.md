# APIPulse Infrastructure & Deployment

Infrastructure-as-Code (IaC), containerization, and observability configurations for local development and cloud edge fleets.

## Directory Structure

- **`docker/`**: Dockerfiles for backend API, edge probe worker, and frontend web client.
- **`kubernetes/`**: Helm charts and manifests for deployment across multi-region edge clusters.
- **`prometheus/`**: Prometheus scrape configs, recording rules, and alert rules.
- **`otel/`**: OpenTelemetry Collector pipeline configurations (`otel-collector-config.yaml`).
- **`grafana/`**: Pre-built dashboards for third-party dependency SLOs and tail latency monitoring.
