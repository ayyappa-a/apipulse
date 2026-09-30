# APIPulse — Third-Party SaaS Uptime & Response-Time Intelligence Platform

> **SRE Command Center & Grey-Failure Defense System for Modern Cloud Stacks**

[![TypeScript](https://img.shields.io/badge/TypeScript-5.x-blue.svg)](https://www.typescriptlang.org/)
[![React](https://img.shields.io/badge/React-18.x-61dafb.svg)](https://react.dev/)
[![Vite](https://img.shields.io/badge/Vite-8.x-646cff.svg)](https://vitejs.dev/)
[![OpenTelemetry](https://img.shields.io/badge/OpenTelemetry-Ready-f5a800.svg)](https://opentelemetry.io/)

---

## Quick Start — Self-Contained Local Environment

> **You do NOT need to install or configure PostgreSQL manually.**
> Docker Compose provisions and configures everything automatically.

### Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (running)
- [Node.js 18+](https://nodejs.org/) (for the frontend dev server)

### 1. Configure environment (one-time)

```bash
cd NEC
cp .env.example .env
```

The default `.env` values are ready for local development — no editing required.

### 2. Start all backend services

```bash
docker compose up --build
```

This single command:
- Starts **TimescaleDB** (PostgreSQL 16 + TimescaleDB extension) on host port **5433**
- Starts **Redis** on port **6379**
- Starts the **Vendor Mock** (Stripe simulator) on port **9001**
- Starts the **APIPulse API** on port **8080** (runs DB migrations automatically)
- Starts the **CodeBegun Demo** customer service on port **8081**

> **Port note:** TimescaleDB uses host port `5433` (not `5432`) to avoid conflicts
> with any existing PostgreSQL installation on your machine.
> All containers communicate internally on the standard port `5432`.

Wait until you see:
```
api | database migrations verified successfully
api | APIPulse API server listening on :8080
```

### 3. Verify health

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

Expected `/ready` response:
```json
{"ready": true, "details": {"postgres": "ok", "redis": "ok", "health_engine": "ok"}}
```

### 4. Start the frontend

In a separate terminal:
```bash
cd frontend
npm install
npm run dev
```

Open [http://localhost:5173](http://localhost:5173) — the SRE dashboard connects to the live backend.

---

## Service Map

| Service | Host Port | Purpose |
|---|---|---|
| `timescaledb` | `5433` | PostgreSQL 16 + TimescaleDB (primary store) |
| `redis` | `6379` | Hot state cache |
| `vendor-mock` | `9001` | Stripe-like vendor simulator |
| `api` | `8080` | APIPulse REST API + OTLP receiver |
| `api` | `9090` | Prometheus metrics scrape |
| `codebegun-demo` | `8081` | Demo customer service (OTel → APIPulse) |

---

## Optional Services

```bash
# Start synthetic probe worker
docker compose --profile probe up

# Start Prometheus scraper (UI at http://localhost:9091)
docker compose --profile metrics up
```

---

## Data Persistence

Database data is stored in the named Docker volume `timescaledb_data`.

```bash
docker compose down          # Stops containers, DATA PRESERVED
docker compose restart       # Restarts containers, DATA PRESERVED
docker compose down -v       # ⚠ DESTROYS database volume — development reset only
```

---

## Direct Local API Execution (Optional)

If you want to run the Go API directly (without Docker) while using the Compose database:

```bash
# Start only infrastructure
docker compose up timescaledb redis vendor-mock -d

# Run API locally (note: uses localhost:5433, not the container default)
DATABASE_URL="postgres://apipulse:apipulse_local_dev@localhost:5433/apipulse?sslmode=disable" \
REDIS_URL="redis://localhost:6379" \
VENDOR_MOCK_URL="http://localhost:9001" \
MIGRATIONS_DIR="./backend/migrations" \
  go run ./backend/cmd/api
```

> The Compose `DATABASE_URL` uses the service hostname `timescaledb:5432`.
> For direct local execution, use `localhost:5433` (the mapped host port).

---

## The Core Problem

Modern cloud platforms rely heavily on third-party SaaS APIs (Stripe, SendGrid, Twilio, Auth0, OpenAI, AWS S3).

A critical blind spot exists in current observability tools:
1. **The "Green Status Page" Lie:** A vendor status page can report **"All Systems Operational"** while a specific geographic edge region (e.g. `ap-south-1`) suffers severe packet loss or 20× latency inflation.
2. **Cascading Pool Exhaustion:** A dependency latency spike from 200ms to 8,000ms does not immediately show up as HTTP 500 errors. Instead, backend threads and database connection pools hang waiting for responses, causing catastrophic cascaded outages in your own infrastructure.
3. **P50 Camouflage:** High median health (p50 of 180ms) hides catastrophic p95 and p99 tail degradations that affect high-value customer transactions.

---

## The APIPulse Solution

APIPulse combines four distinct telemetry streams into a real-time SRE intelligence engine:

```
[Client Application]              [APIPulse Edge Fleet]             [Vendor Status Pages]
(Customer OTel Spans)             (8 Regional Probes)               (Continuous Polling)
        │                                  │                                  │
        └─────────────────┬────────────────┴──────────────────────────────────┘
                          │
                          ▼
        ┌────────────────────────────────────────────────────────┐
        │            APIPulse Health Intelligence Engine         │
        │                                                        │
        │  • p50 / p95 / p99 Percentile Tail Latency Tracking    │
        │  • DNS / TCP / TLS / TTFB Protocol Timing Waterfall    │
        │  • Mathematical Grey-Failure Divergence Detection      │
        │  • Autonomous Circuit Breaker Arming & Trip Signals    │
        └─────────────────────────┬──────────────────────────────┘
                                  │
                                  ▼
        ┌────────────────────────────────────────────────────────┐
        │     SRE Command Center & Real-Time Alert Dispatch      │
        │                                                        │
        │  • Live Edge Fleet Telemetry Matrix                    │
        │  • Prometheus /metrics Scraping & OpenTelemetry Export │
        └────────────────────────────────────────────────────────┘
```

---

## Repository Structure

```
NEC/
├── frontend/             # High-density React/TypeScript SRE Command Center
│   ├── src/
│   │   ├── app/          # Application shell, routing, and provider wrappers
│   │   ├── components/   # Shared layout and UI design-system primitives
│   │   ├── features/     # Feature-scoped components (probes, timeline, waterfalls)
│   │   ├── services/     # Asynchronous data contracts
│   │   ├── hooks/        # Data-fetching and streaming hooks
│   │   ├── types/        # Comprehensive domain definitions
│   │   └── data/mock/    # Deterministic mock datasets
│   └── package.json
│
├── backend/              # Go-based Telemetry Gateway & Probe Workers
│   ├── cmd/api/          # Ingestion API & Gateway daemon
│   ├── cmd/probe/        # Distributed synthetic edge probe worker
│   ├── internal/         # Health intelligence, storage, alerts, and vendor poller
│   ├── migrations/       # SQL schema migrations (001→003, TimescaleDB)
│   └── go.mod
│
├── demo/                 # CodeBegun demo service & Vendor Mock
│   ├── codebegun/        # Demo customer service (OTel → APIPulse)
│   └── vendor-mock/      # Stripe-like vendor simulator
│
├── infrastructure/       # Containerization and telemetry configs
│   ├── docker/           # Dockerfiles for all services
│   ├── prometheus/       # Prometheus scrape configurations
│   └── otel/             # OpenTelemetry Collector pipeline
│
├── docs/                 # Architectural specifications
├── docker-compose.yml    # Self-contained local development environment
├── .env.example          # Environment variable template (copy → .env)
└── .env                  # Local credentials (gitignored, never committed)
```
