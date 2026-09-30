# APIPulse Backend

Go-based backend for the APIPulse Telemetry Gateway, Synthetic Probe Service, and REST API.

## Architecture

```
backend/
├── cmd/
│   ├── api/          # APIPulse Telemetry Gateway & REST API (main entry point)
│   └── probe/        # Synthetic Probe Worker (Phase 3)
│
├── internal/
│   ├── api/
│   │   ├── handlers/ # HTTP request handlers (health, telemetry, probes)
│   │   ├── middleware/ # Request ID, logging, recovery, Prometheus
│   │   └── router/   # Route registration and middleware chain
│   │
│   ├── telemetry/
│   │   ├── model/    # Canonical telemetry data model + validation
│   │   ├── ingest/   # (Phase 3) OTLP receiver
│   │   └── normalize/ # Normalization pipeline (sanitize, stamp source)
│   │
│   ├── storage/
│   │   ├── postgres/ # TelemetryStore interface + NoopStore
│   │   └── redis/    # StateCache interface + NoopCache
│   │
│   ├── probes/       # (Phase 3) Probe runner, timing, quota
│   ├── health/       # (Phase 3) Grey-failure intelligence engine
│   ├── incidents/    # (Phase 3) Incident lifecycle management
│   ├── alerts/       # (Phase 4) Alert policy evaluation
│   ├── vendors/      # (Phase 3) Vendor status scraping
│   └── config/       # Typed configuration from environment variables
│
├── migrations/       # PostgreSQL + TimescaleDB SQL migrations
├── pkg/
│   ├── logger/       # Structured JSON logger
│   └── metrics/      # Prometheus metrics registry
│
├── go.mod
└── README.md
```

## API Endpoints (Phase 2)

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/health` | Liveness probe |
| `GET` | `/ready` | Readiness probe |
| `GET` | `/metrics` | Prometheus scrape endpoint |
| `GET` | `/api/v1` | API version info |
| `POST` | `/api/v1/telemetry/ingest` | ⚠️ Dev: Customer OTEL observations (JSON) |
| `POST` | `/api/v1/probes/results` | ⚠️ Dev: Synthetic probe results (JSON) |

> ⚠️ The `telemetry/ingest` endpoint is a **development contract only**.
> The production path will be `POST /api/v1/telemetry/otlp` (OTLP/HTTP protobuf).

## Running Locally

### Prerequisites
- Go 1.22+
- Docker & Docker Compose (for TimescaleDB + Redis)

### Start infrastructure only
```bash
docker compose up timescaledb redis prometheus
```

### Run the API server
```bash
# From the backend/ directory:
DATABASE_URL="postgres://apipulse:apipulse@localhost:5432/apipulse?sslmode=disable" \
go run ./cmd/api
```

### Run tests
```bash
go test ./...
```

## Configuration

All configuration is via environment variables. See `.env.example` in the repo root.

| Variable | Default | Description |
|----------|---------|-------------|
| `APIPULSE_ENV` | `development` | Environment: `development\|staging\|production` |
| `APIPULSE_HTTP_ADDR` | `:8080` | API server listen address |
| `PROMETHEUS_ADDR` | `:9090` | Prometheus metrics listen address |
| `DATABASE_URL` | (local) | PostgreSQL/TimescaleDB connection string |
| `REDIS_URL` | `redis://localhost:6379` | Redis connection URL |
| `LOG_LEVEL` | `info` | `debug\|info\|warn\|error` |
| `CORS_ALLOWED_ORIGINS` | localhost:5173 | Comma-separated allowed CORS origins |

## Telemetry Sources

See [`docs/architecture/telemetry.md`](../docs/architecture/telemetry.md) for the full telemetry architecture.

APIPulse distinguishes four immutable source types:

| Source | Meaning |
|--------|---------|
| `CUSTOMER_OTEL` | Async OpenTelemetry spans from customer apps |
| `SYNTHETIC_PROBE` | APIPulse-controlled outbound probes |
| `VENDOR_STATUS` | Scraped vendor status page declarations |
| `APIPULSE_ASSESSMENT` | Health Intelligence Engine computed assessments |

## Privacy Contract

APIPulse **never** ingests or stores:
- Request/response bodies
- Auth tokens or API keys
- Customer PII
- Payment data
- Arbitrary query parameters

## Database Migrations

Migrations are in `migrations/`. Run them in order using `psql` or `golang-migrate`:

```bash
# With golang-migrate:
migrate -path migrations/ -database "$DATABASE_URL" up
```

The `probe_observations` and `dependency_observations` tables are converted to
TimescaleDB hypertables. The TimescaleDB extension **must** be available.

## What Is NOT Implemented (Phase 2)

- Real database persistence (NoopStore is used)
- Real Redis connectivity (NoopCache is used)
- Production OTLP/protobuf receiver
- Probe runner (real outbound probes)
- Grey-failure detection engine
- Vendor status scraping
- Authentication
- Slack/PagerDuty integration
