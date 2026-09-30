# APIPulse Runtime Architecture

## 1. Process Lifecycle & Execution Model

APIPulse operates as an asynchronous, out-of-band reliability intelligence daemon. It never sits as a synchronous reverse proxy between customer applications and upstream SaaS dependencies.

```text
┌───────────────────────────────┐
│       Customer Service        │
│  (e.g., CodeBegun Demo)       │
└──────────────┬────────────────┘
               │  1. Real HTTP request
               ▼
┌───────────────────────────────┐
│     Upstream SaaS Vendor      │
│  (e.g., Stripe Simulator)     │
└───────────────────────────────┘
               │
               │  2. Asynchronous OTLP/HTTP telemetry (non-blocking)
               ▼
┌───────────────────────────────┐
│     APIPulse Telemetry API    │
│      (Port 8080 /v1/traces)   │
└──────────────┬────────────────┘
               │
               ▼
┌───────────────────────────────┐
│     TimescaleDB / Postgres    │
│  (hypertables, time-series)   │
└──────────────┬────────────────┘
               │
               ▼
┌───────────────────────────────┐
│    Health Engine Scheduler    │
│   (autonomous 5s-60s tick)    │
└──────────────┬────────────────┘
               │
               ▼
┌───────────────────────────────┐
│      Redis Hot State Cache    │
│  (hysteresis, active states)  │
└───────────────────────────────┘
```

## 2. Server Startup Sequence

1. **Config Resolution**: Reads environment variables (`APIPULSE_STORAGE`, `DATABASE_URL`, `REDIS_URL`, `APIPULSE_HEALTH_EVALUATION_INTERVAL`).
2. **Structured Logger**: JSON logger with configurable level (`info`, `debug`, `warn`, `error`).
3. **Prometheus Registry**: Isolated registry avoiding default collector collisions.
4. **Persistent Storage**:
   - `postgres`: Creates `pgxpool.Pool` with connection limits, health checks, and automatic SQL migration verification (`001`, `002`, `003`).
   - `memory`: In-memory thread-safe fallback for unit tests and lightweight mock exploration.
5. **Hot State Cache**: Connects to Redis via `go-redis/v9`. If unavailable, gracefully falls back to memory and increments `apipulse_redis_errors_total`.
6. **Health Intelligence Engine**: Binds storage, Redis cache, and deterministic vendor status adapter.
7. **Background Scheduler**: Launches autonomous goroutine polling dependencies with bounded concurrency.
8. **HTTP & Metrics Listeners**:
   - `:8080`: Telemetry ingestion, SRE REST API, readiness (`/ready`), liveness (`/health`).
   - `:9090`: Isolated Prometheus scrape endpoint.

## 3. Server Shutdown Sequence

1. `SIGINT` or `SIGTERM` received.
2. Background scheduler cleanly stopped; in-flight evaluations flushed.
3. HTTP and metrics listeners shut down gracefully with configurable timeout (`APIPULSE_SHUTDOWN_TIMEOUT_SECONDS`).
4. Redis connections closed.
5. PostgreSQL connection pool closed.
6. Process exits with code 0.
